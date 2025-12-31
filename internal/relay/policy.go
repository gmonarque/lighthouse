package relay

import (
	"regexp"
	"strings"
	"sync"

	"github.com/gmonarque/lighthouse/internal/decision"
	"github.com/rs/zerolog/log"
)

// decisionStorage is used for curation checks
var decisionStorage = decision.NewStorage()

// TorrentPolicy defines rules for accepting torrent events
type TorrentPolicy struct {
	mu sync.RWMutex

	// Blocklists
	blockedInfohashes map[string]string // infohash -> reason
	blockedPubkeys    map[string]string // pubkey -> reason
	blockedPatterns   []*regexp.Regexp  // name patterns to block

	// Allowlists
	allowedPubkeys map[string]bool // pubkeys that bypass checks

	// Limits
	maxNameLength    int
	maxContentLength int
	maxFileCount     int
	minSize          int64
	maxSize          int64

	// Requirements
	requireInfohash bool
	requireName     bool
	requireSize     bool

	// Curation
	requireCuration bool
	curatorPubkeys  map[string]bool
}

// NewTorrentPolicy creates a new torrent policy with defaults
func NewTorrentPolicy() *TorrentPolicy {
	return &TorrentPolicy{
		blockedInfohashes: make(map[string]string),
		blockedPubkeys:    make(map[string]string),
		blockedPatterns:   []*regexp.Regexp{},
		allowedPubkeys:    make(map[string]bool),
		curatorPubkeys:    make(map[string]bool),
		maxNameLength:     500,
		maxContentLength:  10000,
		maxFileCount:      5000,
		minSize:           0,
		maxSize:           0, // 0 = no limit
		requireInfohash:   true,
		requireName:       true,
		requireSize:       false,
		requireCuration:   false,
	}
}

// CheckEvent checks if a torrent event is allowed by the policy
func (p *TorrentPolicy) CheckEvent(event *Event) (bool, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check if pubkey is allowed (bypass all checks)
	if p.allowedPubkeys[event.PubKey] {
		return true, ""
	}

	// Check if pubkey is blocked
	if reason, blocked := p.blockedPubkeys[event.PubKey]; blocked {
		log.Debug().Str("pubkey", event.PubKey).Str("reason", reason).Msg("Blocked by pubkey")
		return false, "blocked: " + reason
	}

	// Extract torrent metadata
	infohash := event.GetInfohash()
	name := event.GetTagValue("name")
	if name == "" {
		name = event.GetTagValue("title")
	}

	// Check required fields
	if p.requireInfohash && infohash == "" {
		return false, "missing infohash"
	}
	if p.requireName && name == "" {
		return false, "missing name"
	}

	// Check if infohash is blocked
	if reason, blocked := p.blockedInfohashes[strings.ToLower(infohash)]; blocked {
		log.Debug().Str("infohash", infohash).Str("reason", reason).Msg("Blocked by infohash")
		return false, "blocked: " + reason
	}

	// Check name length
	if p.maxNameLength > 0 && len(name) > p.maxNameLength {
		return false, "name too long"
	}

	// Check content length
	if p.maxContentLength > 0 && len(event.Content) > p.maxContentLength {
		return false, "content too long"
	}

	// Check name against blocked patterns
	for _, pattern := range p.blockedPatterns {
		if pattern.MatchString(name) {
			log.Debug().Str("name", name).Str("pattern", pattern.String()).Msg("Blocked by pattern")
			return false, "blocked by content filter"
		}
	}

	// Check size if required
	if p.requireSize {
		sizeStr := event.GetTagValue("size")
		if sizeStr == "" {
			return false, "missing size"
		}
	}

	// Check curation requirement (spec 5.3: community relay MUST only accept curated content)
	if p.requireCuration {
		// Get the infohash to check for curation decisions
		if infohash != "" {
			curated, reason := p.isCurated(infohash)
			if !curated {
				log.Debug().
					Str("infohash", infohash).
					Str("reason", reason).
					Msg("Event rejected: not curated")
				return false, reason
			}
		}
	}

	return true, ""
}

// isCurated checks if an infohash has been accepted by an approved curator
func (p *TorrentPolicy) isCurated(infohash string) (bool, string) {
	// Get decision summary for this infohash
	summary, err := decisionStorage.GetSummary(infohash)
	if err != nil {
		log.Warn().Err(err).Str("infohash", infohash).Msg("Failed to get decision summary")
		return false, "curation check failed"
	}

	// No decisions at all - not curated
	if summary.TotalDecisions == 0 {
		return false, "no curation decision found"
	}

	// Check if final decision is accept
	if summary.FinalDecision != decision.DecisionAccept {
		return false, "rejected by curators"
	}

	// Additionally check if any of the accepting curators are in our trusted list
	if len(p.curatorPubkeys) > 0 {
		decisions, err := decisionStorage.GetByInfohash(infohash)
		if err != nil {
			return false, "failed to verify curator"
		}

		hasApprovedCurator := false
		for _, d := range decisions {
			if d.Decision == decision.DecisionAccept && p.curatorPubkeys[d.CuratorPubkey] {
				hasApprovedCurator = true
				break
			}
		}

		if !hasApprovedCurator {
			return false, "no accept from trusted curator"
		}
	}

	return true, ""
}

// BlockInfohash adds an infohash to the blocklist
func (p *TorrentPolicy) BlockInfohash(infohash, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blockedInfohashes[strings.ToLower(infohash)] = reason
}

// UnblockInfohash removes an infohash from the blocklist
func (p *TorrentPolicy) UnblockInfohash(infohash string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.blockedInfohashes, strings.ToLower(infohash))
}

// BlockPubkey adds a pubkey to the blocklist
func (p *TorrentPolicy) BlockPubkey(pubkey, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.blockedPubkeys[pubkey] = reason
}

// UnblockPubkey removes a pubkey from the blocklist
func (p *TorrentPolicy) UnblockPubkey(pubkey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.blockedPubkeys, pubkey)
}

// AllowPubkey adds a pubkey to the allowlist
func (p *TorrentPolicy) AllowPubkey(pubkey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.allowedPubkeys[pubkey] = true
}

// DisallowPubkey removes a pubkey from the allowlist
func (p *TorrentPolicy) DisallowPubkey(pubkey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.allowedPubkeys, pubkey)
}

// AddBlockedPattern adds a name pattern to block
func (p *TorrentPolicy) AddBlockedPattern(pattern string) error {
	compiled, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.blockedPatterns = append(p.blockedPatterns, compiled)
	return nil
}

// SetRequireCuration sets whether curation is required
func (p *TorrentPolicy) SetRequireCuration(required bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requireCuration = required
}

// AddCurator adds a curator pubkey
func (p *TorrentPolicy) AddCurator(pubkey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.curatorPubkeys[pubkey] = true
}

// RemoveCurator removes a curator pubkey
func (p *TorrentPolicy) RemoveCurator(pubkey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.curatorPubkeys, pubkey)
}

// IsCurator checks if a pubkey is a curator
func (p *TorrentPolicy) IsCurator(pubkey string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.curatorPubkeys[pubkey]
}

// SetLimits sets size limits for torrents
func (p *TorrentPolicy) SetLimits(minSize, maxSize int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.minSize = minSize
	p.maxSize = maxSize
}

// GetBlockedInfohashes returns all blocked infohashes
func (p *TorrentPolicy) GetBlockedInfohashes() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range p.blockedInfohashes {
		result[k] = v
	}
	return result
}

// GetBlockedPubkeys returns all blocked pubkeys
func (p *TorrentPolicy) GetBlockedPubkeys() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range p.blockedPubkeys {
		result[k] = v
	}
	return result
}

// LoadFromFile loads policy from a file
func (p *TorrentPolicy) LoadFromFile(path string) error {
	// TODO: Implement loading from JSON/YAML file
	return nil
}

// PolicyStats contains policy statistics
type PolicyStats struct {
	BlockedInfohashes int
	BlockedPubkeys    int
	BlockedPatterns   int
	AllowedPubkeys    int
	Curators          int
}

// GetStats returns policy statistics
func (p *TorrentPolicy) GetStats() PolicyStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PolicyStats{
		BlockedInfohashes: len(p.blockedInfohashes),
		BlockedPubkeys:    len(p.blockedPubkeys),
		BlockedPatterns:   len(p.blockedPatterns),
		AllowedPubkeys:    len(p.allowedPubkeys),
		Curators:          len(p.curatorPubkeys),
	}
}
