// Package curator provides content curation functionality
package curator

import (
	"fmt"
	"sync"
	"time"

	"github.com/gmonarque/lighthouse/internal/config"
	"github.com/gmonarque/lighthouse/internal/decision"
	"github.com/gmonarque/lighthouse/internal/ruleset"
	"github.com/gmonarque/lighthouse/internal/trust"
	"github.com/nbd-wtf/go-nostr"
	"github.com/rs/zerolog/log"
)

// Curator evaluates content against rulesets and produces verification decisions
type Curator struct {
	mu sync.RWMutex

	// Configuration
	enabled    bool
	mode       string // "local", "remote", "hybrid"
	privateKey string
	publicKey  string

	// Components
	engine          *ruleset.Engine
	signer          *decision.Signer
	decisionStorage *decision.Storage
	rulesetStorage  *ruleset.Storage

	// State
	running bool
	stats   CuratorStats
}

// CuratorStats tracks curator statistics
type CuratorStats struct {
	TotalProcessed int64
	TotalAccepted  int64
	TotalRejected  int64
	LastProcessed  time.Time
}

// Config holds curator configuration
type Config struct {
	Enabled    bool
	Mode       string
	PrivateKey string
}

// NewCurator creates a new curator instance
func NewCurator(cfg Config) (*Curator, error) {
	c := &Curator{
		enabled:         cfg.Enabled,
		mode:            cfg.Mode,
		engine:          ruleset.NewEngine(),
		decisionStorage: decision.NewStorage(),
		rulesetStorage:  ruleset.NewStorage(),
	}

	if cfg.PrivateKey != "" {
		signer, err := decision.NewSigner(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create signer: %w", err)
		}
		c.signer = signer
		c.privateKey = cfg.PrivateKey
		c.publicKey = signer.GetPublicKey()
	}

	return c, nil
}

// Init initializes the curator with rulesets
func (c *Curator) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialize default rulesets if needed
	if err := c.rulesetStorage.InitDefaults(); err != nil {
		log.Warn().Err(err).Msg("Failed to initialize default rulesets")
	}

	// Load active rulesets
	if err := c.loadActiveRulesets(); err != nil {
		return fmt.Errorf("failed to load rulesets: %w", err)
	}

	c.running = true
	log.Info().
		Bool("enabled", c.enabled).
		Str("mode", c.mode).
		Msg("Curator initialized")

	return nil
}

// loadActiveRulesets loads the active rulesets into the engine
func (c *Curator) loadActiveRulesets() error {
	// Load censoring ruleset
	censoring, err := c.rulesetStorage.GetActive(ruleset.RulesetTypeCensoring)
	if err != nil {
		return err
	}
	if censoring != nil {
		c.engine.SetCensoringRuleset(censoring)
		log.Debug().
			Str("id", censoring.ID).
			Str("version", censoring.Version).
			Msg("Loaded censoring ruleset")
	}

	// Load semantic ruleset
	semantic, err := c.rulesetStorage.GetActive(ruleset.RulesetTypeSemantic)
	if err != nil {
		return err
	}
	if semantic != nil {
		c.engine.SetSemanticRuleset(semantic)
		log.Debug().
			Str("id", semantic.ID).
			Str("version", semantic.Version).
			Msg("Loaded semantic ruleset")
	}

	return nil
}

// IsEnabled returns whether the curator is enabled
func (c *Curator) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// GetPublicKey returns the curator's public key
func (c *Curator) GetPublicKey() string {
	return c.publicKey
}

// ProcessEvent evaluates a Nostr torrent event and produces a verification decision
func (c *Curator) ProcessEvent(event *nostr.Event) (*decision.VerificationDecision, error) {
	c.mu.RLock()
	enabled := c.enabled
	c.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("curator is disabled")
	}

	// Parse event to torrent data
	torrentData, err := ParseNostrEvent(event)
	if err != nil {
		log.Debug().Err(err).Str("event_id", event.ID).Msg("Failed to parse event")
		return c.createRejectDecision(event, []ruleset.ReasonCode{ruleset.ReasonSemBadMeta})
	}

	// Evaluate against rulesets
	censoringResult := c.engine.EvaluateCensoring(torrentData)
	semanticResult := c.engine.EvaluateSemantic(torrentData)

	// Determine decision
	shouldReject, reasons := ruleset.ShouldReject(censoringResult, semanticResult, 0.7)

	var d *decision.VerificationDecision
	if shouldReject {
		d = decision.NewRejectDecision(event.ID, torrentData.InfoHash, c.publicKey, reasons)
	} else {
		d = decision.NewAcceptDecision(event.ID, torrentData.InfoHash, c.publicKey)
	}

	// Set ruleset info
	if censoring := c.engine.GetCensoringRuleset(); censoring != nil {
		d.SetRulesetInfo(string(censoring.Type), censoring.Version, censoring.Hash)
	}

	// Sign the decision
	if c.signer != nil {
		if err := c.signer.Sign(d); err != nil {
			log.Error().Err(err).Msg("Failed to sign decision")
		}
	}

	// Update stats
	c.mu.Lock()
	c.stats.TotalProcessed++
	if d.Decision == decision.DecisionAccept {
		c.stats.TotalAccepted++
	} else {
		c.stats.TotalRejected++
	}
	c.stats.LastProcessed = time.Now()
	c.mu.Unlock()

	log.Debug().
		Str("event_id", event.ID).
		Str("infohash", torrentData.InfoHash).
		Str("decision", string(d.Decision)).
		Int("reasons", len(reasons)).
		Msg("Processed event")

	return d, nil
}

// ProcessTorrent evaluates torrent data directly
func (c *Curator) ProcessTorrent(data *ruleset.TorrentData, eventID string) (*decision.VerificationDecision, error) {
	c.mu.RLock()
	enabled := c.enabled
	c.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("curator is disabled")
	}

	// Evaluate against rulesets
	censoringResult := c.engine.EvaluateCensoring(data)
	semanticResult := c.engine.EvaluateSemantic(data)

	// Determine decision
	shouldReject, reasons := ruleset.ShouldReject(censoringResult, semanticResult, 0.7)

	var d *decision.VerificationDecision
	if shouldReject {
		d = decision.NewRejectDecision(eventID, data.InfoHash, c.publicKey, reasons)
	} else {
		d = decision.NewAcceptDecision(eventID, data.InfoHash, c.publicKey)
	}

	// Set ruleset info
	if censoring := c.engine.GetCensoringRuleset(); censoring != nil {
		d.SetRulesetInfo(string(censoring.Type), censoring.Version, censoring.Hash)
	}

	// Sign the decision
	if c.signer != nil {
		if err := c.signer.Sign(d); err != nil {
			log.Error().Err(err).Msg("Failed to sign decision")
		}
	}

	// Update stats
	c.mu.Lock()
	c.stats.TotalProcessed++
	if d.Decision == decision.DecisionAccept {
		c.stats.TotalAccepted++
	} else {
		c.stats.TotalRejected++
	}
	c.stats.LastProcessed = time.Now()
	c.mu.Unlock()

	return d, nil
}

// createRejectDecision creates a reject decision with the given reasons
func (c *Curator) createRejectDecision(event *nostr.Event, reasons []ruleset.ReasonCode) (*decision.VerificationDecision, error) {
	// Extract infohash from event if possible
	infohash := ""
	for _, tag := range event.Tags {
		if len(tag) >= 2 && (tag[0] == "x" || tag[0] == "btih" || tag[0] == "infohash") {
			infohash = tag[1]
			break
		}
	}

	d := decision.NewRejectDecision(event.ID, infohash, c.publicKey, reasons)

	if c.signer != nil {
		if err := c.signer.Sign(d); err != nil {
			log.Error().Err(err).Msg("Failed to sign decision")
		}
	}

	c.mu.Lock()
	c.stats.TotalProcessed++
	c.stats.TotalRejected++
	c.stats.LastProcessed = time.Now()
	c.mu.Unlock()

	return d, nil
}

// SaveDecision saves a decision to storage
func (c *Curator) SaveDecision(d *decision.VerificationDecision) error {
	return c.decisionStorage.Save(d)
}

// GetStats returns curator statistics
func (c *Curator) GetStats() CuratorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// ReloadRulesets reloads rulesets from storage
func (c *Curator) ReloadRulesets() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.loadActiveRulesets()
}

// SetEnabled enables or disables the curator
func (c *Curator) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
}

// GetEngine returns the ruleset engine
func (c *Curator) GetEngine() *ruleset.Engine {
	return c.engine
}

// GetDecisionStorage returns the decision storage
func (c *Curator) GetDecisionStorage() *decision.Storage {
	return c.decisionStorage
}

// GetRulesetStorage returns the ruleset storage
func (c *Curator) GetRulesetStorage() *ruleset.Storage {
	return c.rulesetStorage
}

// Global curator instance
var globalCurator *Curator

// InitGlobal initializes the global curator instance
func InitGlobal() error {
	cfg := config.Get()

	curatorCfg := Config{
		Enabled:    cfg.Curator.Enabled,
		Mode:       cfg.Curator.Mode,
		PrivateKey: cfg.Nostr.Identity.Nsec,
	}

	var err error
	globalCurator, err = NewCurator(curatorCfg)
	if err != nil {
		return err
	}

	return globalCurator.Init()
}

// Get returns the global curator instance
func Get() *Curator {
	return globalCurator
}

// CurationResult contains the full result of a curation evaluation
type CurationResult struct {
	Decision        *decision.VerificationDecision
	CensoringResult *ruleset.EvaluationResult
	SemanticResult  *ruleset.EvaluationResult
	Accepted        bool
	Reasons         []ruleset.ReasonCode
}

// EvaluateWithDetails evaluates and returns detailed results
func (c *Curator) EvaluateWithDetails(data *ruleset.TorrentData, eventID string) (*CurationResult, error) {
	c.mu.RLock()
	enabled := c.enabled
	c.mu.RUnlock()

	if !enabled {
		return nil, fmt.Errorf("curator is disabled")
	}

	// Evaluate against rulesets
	censoringResult := c.engine.EvaluateCensoring(data)
	semanticResult := c.engine.EvaluateSemantic(data)

	// Determine decision
	shouldReject, reasons := ruleset.ShouldReject(censoringResult, semanticResult, 0.7)

	var d *decision.VerificationDecision
	if shouldReject {
		d = decision.NewRejectDecision(eventID, data.InfoHash, c.publicKey, reasons)
	} else {
		d = decision.NewAcceptDecision(eventID, data.InfoHash, c.publicKey)
	}

	// Sign the decision
	if c.signer != nil {
		c.signer.Sign(d)
	}

	return &CurationResult{
		Decision:        d,
		CensoringResult: censoringResult,
		SemanticResult:  semanticResult,
		Accepted:        !shouldReject,
		Reasons:         reasons,
	}, nil
}

// ParseNostrEvent converts a Nostr event to TorrentData
func ParseNostrEvent(event *nostr.Event) (*ruleset.TorrentData, error) {
	if event.Kind != 2003 {
		return nil, fmt.Errorf("unexpected event kind: %d", event.Kind)
	}

	data := &ruleset.TorrentData{
		EventID:  event.ID,
		Uploader: event.PubKey,
	}

	// Track required tag "i" presence (spec 7.0.1)
	hasTagI := false

	// Extract info from tags
	for _, tag := range event.Tags {
		if len(tag) < 2 {
			continue
		}

		switch tag[0] {
		case "x", "btih", "infohash":
			data.InfoHash = tag[1]
		case "title", "name":
			if data.Name == "" {
				data.Name = tag[1]
			}
			if tag[0] == "title" {
				data.Title = tag[1]
			}
		case "size":
			fmt.Sscanf(tag[1], "%d", &data.Size)
		case "category", "cat":
			fmt.Sscanf(tag[1], "%d", &data.Category)
		case "t":
			data.Tags = append(data.Tags, tag[1])
		case "i":
			// External IDs (imdb:tt*, tmdb:*)
			hasTagI = true
			if len(tag[1]) > 5 {
				if tag[1][:5] == "imdb:" {
					data.ImdbID = tag[1][5:]
				} else if tag[1][:5] == "tmdb:" {
					fmt.Sscanf(tag[1][5:], "%d", &data.TmdbID)
				}
			}
		case "file":
			if len(tag) >= 3 {
				var size int64
				fmt.Sscanf(tag[2], "%d", &size)
				data.Files = append(data.Files, ruleset.FileEntry{
					Path: tag[1],
					Size: size,
				})
			}
		}
	}

	// Use content as overview if available
	if event.Content != "" && data.Overview == "" {
		data.Overview = event.Content
	}

	if data.InfoHash == "" {
		return nil, fmt.Errorf("missing infohash")
	}

	// Spec 7.0.1: Event MUST be rejected if tag "i" is absent
	if !hasTagI {
		return nil, fmt.Errorf("missing required tag 'i' (external ID)")
	}

	return data, nil
}

// CuratorManager manages the curator integration with the indexer
type CuratorManager struct {
	curator     *Curator
	aggregator  *trust.Aggregator
	policyStore *trust.PolicyStorage
}

// NewCuratorManager creates a new curator manager
func NewCuratorManager(curator *Curator) *CuratorManager {
	policyStore := trust.NewPolicyStorage()

	aggPolicy := &trust.AggregationPolicy{
		Mode:           trust.AggregationModeAny,
		QuorumRequired: 1,
	}

	return &CuratorManager{
		curator:     curator,
		policyStore: policyStore,
		aggregator:  trust.NewAggregator(policyStore, aggPolicy),
	}
}

// ShouldIndex determines if a torrent should be indexed based on curation
func (m *CuratorManager) ShouldIndex(data *ruleset.TorrentData, eventID string) (bool, error) {
	if m.curator == nil || !m.curator.IsEnabled() {
		return true, nil // Accept all if curator disabled
	}

	result, err := m.curator.EvaluateWithDetails(data, eventID)
	if err != nil {
		return true, err // Accept on error
	}

	return result.Accepted, nil
}
