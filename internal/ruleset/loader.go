package ruleset

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

// Loader handles loading rulesets from various sources
type Loader struct {
	httpClient *http.Client
	cacheDir   string
}

// NewLoader creates a new ruleset loader
func NewLoader(cacheDir string) *Loader {
	return &Loader{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir: cacheDir,
	}
}

// LoadFromFile loads a ruleset from a local file
func (l *Loader) LoadFromFile(path string) (*Ruleset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ruleset file: %w", err)
	}

	return l.parseRuleset(data, "file://"+path)
}

// LoadFromURL loads a ruleset from a URL
func (l *Loader) LoadFromURL(url string) (*Ruleset, error) {
	resp, err := l.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ruleset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch ruleset: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return l.parseRuleset(data, url)
}

// LoadFromJSON loads a ruleset from JSON data
func (l *Loader) LoadFromJSON(data []byte, source string) (*Ruleset, error) {
	return l.parseRuleset(data, source)
}

// parseRuleset parses and validates a ruleset
func (l *Loader) parseRuleset(data []byte, source string) (*Ruleset, error) {
	var ruleset Ruleset
	if err := json.Unmarshal(data, &ruleset); err != nil {
		return nil, fmt.Errorf("failed to parse ruleset JSON: %w", err)
	}

	// Validate the ruleset
	if err := ruleset.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ruleset: %w", err)
	}

	// Compute hash if not present
	if ruleset.Hash == "" {
		ruleset.Hash = ruleset.ComputeHash()
	} else {
		// Verify hash
		computed := ruleset.ComputeHash()
		if computed != ruleset.Hash {
			log.Warn().
				Str("expected", ruleset.Hash).
				Str("computed", computed).
				Msg("Ruleset hash mismatch")
		}
	}

	// Set defaults for rules
	for i := range ruleset.Rules {
		if ruleset.Rules[i].Priority == 0 {
			ruleset.Rules[i].Priority = ruleset.Rules[i].Code.Priority()
		}
	}

	log.Debug().
		Str("id", ruleset.ID).
		Str("type", string(ruleset.Type)).
		Str("version", ruleset.Version).
		Int("rules", len(ruleset.Rules)).
		Str("source", source).
		Msg("Loaded ruleset")

	return &ruleset, nil
}

// CacheRuleset caches a ruleset to disk
func (l *Loader) CacheRuleset(r *Ruleset) error {
	if l.cacheDir == "" {
		return nil
	}

	if err := os.MkdirAll(l.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	filename := fmt.Sprintf("%s_%s_%s.json", r.Type, r.ID, r.Version)
	path := filepath.Join(l.cacheDir, filename)

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ruleset: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Debug().
		Str("path", path).
		Str("ruleset", r.ID).
		Msg("Cached ruleset")

	return nil
}

// LoadFromCache loads a ruleset from cache
func (l *Loader) LoadFromCache(rulesetType RulesetType, id, version string) (*Ruleset, error) {
	if l.cacheDir == "" {
		return nil, fmt.Errorf("cache directory not configured")
	}

	filename := fmt.Sprintf("%s_%s_%s.json", rulesetType, id, version)
	path := filepath.Join(l.cacheDir, filename)

	return l.LoadFromFile(path)
}

// ListCached lists all cached rulesets
func (l *Loader) ListCached() ([]RulesetDescriptor, error) {
	if l.cacheDir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(l.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var descriptors []RulesetDescriptor
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(l.cacheDir, entry.Name())
		r, err := l.LoadFromFile(path)
		if err != nil {
			log.Warn().Err(err).Str("file", entry.Name()).Msg("Failed to load cached ruleset")
			continue
		}

		info, _ := entry.Info()
		descriptors = append(descriptors, RulesetDescriptor{
			RulesetID: r.ID,
			Type:      string(r.Type),
			Version:   r.Version,
			Hash:      r.Hash,
			Source:    "file://" + path,
			UpdatedAt: info.ModTime(),
		})
	}

	return descriptors, nil
}

// CreateDefaultCensoringRuleset creates a default censoring ruleset
func CreateDefaultCensoringRuleset() *Ruleset {
	return &Ruleset{
		ID:          "default-censoring",
		Type:        RulesetTypeCensoring,
		Version:     "1.0.0",
		Description: "Default censoring ruleset for blocking spam and malware",
		Rules: []Rule{
			{
				ID:          "block-spam-patterns",
				Code:        ReasonAbuseSpam,
				Type:        "deterministic",
				Description: "Block torrents with common spam patterns in name",
				Condition: Condition{
					Type:     ConditionTypeRegex,
					Field:    "name",
					Value:    `(?i)(bonus|free|download now|click here|xxx.*bonus)`,
				},
				Action:  "reject",
				Enabled: true,
			},
			{
				ID:          "block-malware-extensions",
				Code:        ReasonAbuseMalware,
				Type:        "deterministic",
				Description: "Block torrents with suspicious executable patterns",
				Condition: Condition{
					Type:     ConditionTypeRegex,
					Field:    "name",
					Value:    `(?i)\.(exe|bat|cmd|scr|pif|com)\.torrent$`,
				},
				Action:  "reject",
				Enabled: true,
			},
		},
		CreatedAt: time.Now(),
	}
}

// CreateDefaultSemanticRuleset creates a default semantic ruleset
func CreateDefaultSemanticRuleset() *Ruleset {
	return &Ruleset{
		ID:          "default-semantic",
		Type:        RulesetTypeSemantic,
		Version:     "1.0.0",
		Description: "Default semantic ruleset for quality control",
		Rules: []Rule{
			{
				ID:          "require-basic-metadata",
				Code:        ReasonSemBadMeta,
				Type:        "probabilistic",
				Description: "Require basic metadata fields",
				Condition: Condition{
					Type:      ConditionTypeMetadataScore,
					MinFields: []string{"name", "size"},
					Threshold: 0.3,
				},
				Action:  "reject",
				Enabled: true,
			},
			{
				ID:          "minimum-size",
				Code:        ReasonSemLowQuality,
				Type:        "probabilistic",
				Description: "Reject very small torrents (likely incomplete or fake)",
				Condition: Condition{
					Type: ConditionTypeSizeRange,
					Extra: map[string]interface{}{
						"min": float64(1024), // 1KB minimum
					},
				},
				Action:  "reject",
				Enabled: true,
			},
		},
		CreatedAt: time.Now(),
	}
}
