// Package ruleset provides versioned rule definitions for content curation
package ruleset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReasonCode represents a standardized rejection reason
type ReasonCode string

// Deterministic reason codes (always result in the same decision)
const (
	// Legal reasons
	ReasonLegalDMCA    ReasonCode = "LEGAL_DMCA"
	ReasonLegalIllegal ReasonCode = "LEGAL_ILLEGAL"

	// Abuse reasons
	ReasonAbuseSpam    ReasonCode = "ABUSE_SPAM"
	ReasonAbuseMalware ReasonCode = "ABUSE_MALWARE"

	// Semantic - deterministic
	ReasonSemDuplicateExact ReasonCode = "SEM_DUPLICATE_EXACT"
)

// Probabilistic reason codes (may vary based on thresholds)
const (
	ReasonSemDuplicateProbable  ReasonCode = "SEM_DUPLICATE_PROBABLE"
	ReasonSemBadMeta            ReasonCode = "SEM_BAD_META"
	ReasonSemLowQuality         ReasonCode = "SEM_LOW_QUALITY"
	ReasonSemCategoryMismatch   ReasonCode = "SEM_CATEGORY_MISMATCH"
)

// IsDeterministic returns true if this reason code always produces the same decision
func (r ReasonCode) IsDeterministic() bool {
	switch r {
	case ReasonLegalDMCA, ReasonLegalIllegal,
		ReasonAbuseSpam, ReasonAbuseMalware,
		ReasonSemDuplicateExact:
		return true
	}
	return false
}

// IsLegal returns true if this is a legal-related reason
func (r ReasonCode) IsLegal() bool {
	return strings.HasPrefix(string(r), "LEGAL_")
}

// IsAbuse returns true if this is an abuse-related reason
func (r ReasonCode) IsAbuse() bool {
	return strings.HasPrefix(string(r), "ABUSE_")
}

// IsSemantic returns true if this is a semantic-related reason
func (r ReasonCode) IsSemantic() bool {
	return strings.HasPrefix(string(r), "SEM_")
}

// Priority returns the priority of this reason (higher = more important)
func (r ReasonCode) Priority() int {
	switch r {
	case ReasonLegalDMCA, ReasonLegalIllegal:
		return 100
	case ReasonAbuseMalware:
		return 90
	case ReasonAbuseSpam:
		return 80
	case ReasonSemDuplicateExact:
		return 70
	case ReasonSemDuplicateProbable:
		return 60
	case ReasonSemBadMeta, ReasonSemLowQuality, ReasonSemCategoryMismatch:
		return 50
	}
	return 0
}

// String returns the string representation
func (r ReasonCode) String() string {
	return string(r)
}

// Description returns a human-readable description
func (r ReasonCode) Description() string {
	switch r {
	case ReasonLegalDMCA:
		return "Content removed due to documented legal report (DMCA)"
	case ReasonLegalIllegal:
		return "Content is manifestly illegal"
	case ReasonAbuseSpam:
		return "Content identified as spam, flooding, or abusive duplication"
	case ReasonAbuseMalware:
		return "Content contains malicious or dangerous material"
	case ReasonSemDuplicateExact:
		return "Exact duplicate (same infohash already exists)"
	case ReasonSemDuplicateProbable:
		return "Probable duplicate (same files detected)"
	case ReasonSemBadMeta:
		return "Metadata is inconsistent or incomplete"
	case ReasonSemLowQuality:
		return "Content quality is insufficient"
	case ReasonSemCategoryMismatch:
		return "Category does not match content"
	}
	return "Unknown reason"
}

// RulesetType represents the type of ruleset
type RulesetType string

const (
	RulesetTypeCensoring RulesetType = "censoring"
	RulesetTypeSemantic  RulesetType = "semantic"
)

// Ruleset represents a versioned collection of rules
type Ruleset struct {
	ID          string        `json:"id"`
	Type        RulesetType   `json:"type"`
	Version     string        `json:"version"`
	Hash        string        `json:"hash,omitempty"`
	Description string        `json:"description,omitempty"`
	Rules       []Rule        `json:"rules"`
	CreatedAt   time.Time     `json:"created_at,omitempty"`
	DeprecatedAt *time.Time   `json:"deprecated_at,omitempty"`
}

// Rule represents a single rule within a ruleset
type Rule struct {
	ID          string      `json:"id"`
	Code        ReasonCode  `json:"code"`
	Type        string      `json:"type"` // "deterministic" or "probabilistic"
	Description string      `json:"description,omitempty"`
	Condition   Condition   `json:"condition"`
	Action      string      `json:"action"` // "accept" or "reject"
	Enabled     bool        `json:"enabled"`
	Priority    int         `json:"priority,omitempty"`
}

// Condition represents a rule condition
type Condition struct {
	Type      string                 `json:"type"`
	Field     string                 `json:"field,omitempty"`
	Operator  string                 `json:"operator,omitempty"`
	Value     interface{}            `json:"value,omitempty"`
	Values    []interface{}          `json:"values,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Threshold float64                `json:"threshold,omitempty"`
	MinFields []string               `json:"min_fields,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// ConditionType constants
const (
	ConditionTypeInfohashList  = "infohash_list"
	ConditionTypePubkeyList    = "pubkey_list"
	ConditionTypeRegex         = "regex"
	ConditionTypeMetadataScore = "metadata_score"
	ConditionTypeSizeRange     = "size_range"
	ConditionTypeCategoryMatch = "category_match"
	ConditionTypeTagMatch      = "tag_match"
	ConditionTypeCustom        = "custom"
)

// RulesetDescriptor contains metadata about a ruleset
type RulesetDescriptor struct {
	RulesetID   string    `json:"ruleset_id"`
	Type        string    `json:"type"`
	Version     string    `json:"version"`
	Hash        string    `json:"hash"`
	Source      string    `json:"source"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsActive    bool      `json:"is_active"`
}

// ComputeHash computes the SHA256 hash of the ruleset content
func (r *Ruleset) ComputeHash() string {
	// Create a copy without the hash field
	copy := *r
	copy.Hash = ""
	copy.CreatedAt = time.Time{}
	copy.DeprecatedAt = nil

	data, err := json.Marshal(copy)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Validate validates the ruleset structure
func (r *Ruleset) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("ruleset ID is required")
	}
	if r.Type != RulesetTypeCensoring && r.Type != RulesetTypeSemantic {
		return fmt.Errorf("invalid ruleset type: %s", r.Type)
	}
	if r.Version == "" {
		return fmt.Errorf("ruleset version is required")
	}
	if len(r.Rules) == 0 {
		return fmt.Errorf("ruleset must contain at least one rule")
	}

	for i, rule := range r.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule %d (%s): %w", i, rule.ID, err)
		}
	}

	return nil
}

// Validate validates a rule
func (r *Rule) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if r.Code == "" {
		return fmt.Errorf("reason code is required")
	}
	if r.Action != "accept" && r.Action != "reject" {
		return fmt.Errorf("action must be 'accept' or 'reject'")
	}
	if r.Type != "deterministic" && r.Type != "probabilistic" {
		return fmt.Errorf("type must be 'deterministic' or 'probabilistic'")
	}
	if r.Condition.Type == "" {
		return fmt.Errorf("condition type is required")
	}
	return nil
}

// EvaluationResult represents the result of evaluating a torrent against rules
type EvaluationResult struct {
	Passed       bool         `json:"passed"`
	MatchedRules []MatchedRule `json:"matched_rules,omitempty"`
	Score        float64      `json:"score,omitempty"`
}

// MatchedRule represents a rule that matched during evaluation
type MatchedRule struct {
	RuleID      string     `json:"rule_id"`
	Code        ReasonCode `json:"code"`
	Action      string     `json:"action"`
	Description string     `json:"description,omitempty"`
	Score       float64    `json:"score,omitempty"`
}

// TorrentData represents the data used for rule evaluation
type TorrentData struct {
	InfoHash    string            `json:"info_hash"`
	Name        string            `json:"name"`
	Size        int64             `json:"size"`
	Category    int               `json:"category"`
	Tags        []string          `json:"tags,omitempty"`
	Files       []FileEntry       `json:"files,omitempty"`
	Uploader    string            `json:"uploader"`
	Title       string            `json:"title,omitempty"`
	Year        int               `json:"year,omitempty"`
	ImdbID      string            `json:"imdb_id,omitempty"`
	TmdbID      int               `json:"tmdb_id,omitempty"`
	Overview    string            `json:"overview,omitempty"`
	EventID     string            `json:"event_id,omitempty"`
	RelayURL    string            `json:"relay_url,omitempty"`
}

// FileEntry represents a file in a torrent
type FileEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Hash string `json:"hash,omitempty"`
}

// HasRequiredFields checks if the torrent has the specified required fields
func (t *TorrentData) HasRequiredFields(fields []string) (bool, []string) {
	missing := []string{}
	for _, field := range fields {
		switch field {
		case "title":
			if t.Title == "" && t.Name == "" {
				missing = append(missing, field)
			}
		case "name":
			if t.Name == "" {
				missing = append(missing, field)
			}
		case "size":
			if t.Size == 0 {
				missing = append(missing, field)
			}
		case "category":
			if t.Category == 0 {
				missing = append(missing, field)
			}
		case "info_hash":
			if t.InfoHash == "" {
				missing = append(missing, field)
			}
		case "uploader":
			if t.Uploader == "" {
				missing = append(missing, field)
			}
		}
	}
	return len(missing) == 0, missing
}

// MetadataScore calculates a quality score for the metadata (0-100)
func (t *TorrentData) MetadataScore() float64 {
	score := 0.0
	maxScore := 0.0

	// Required fields (40 points)
	maxScore += 40
	if t.Name != "" {
		score += 15
	}
	if t.Size > 0 {
		score += 15
	}
	if t.Category > 0 {
		score += 10
	}

	// Enhanced metadata (40 points)
	maxScore += 40
	if t.Title != "" {
		score += 10
	}
	if t.Year > 0 {
		score += 5
	}
	if t.ImdbID != "" || t.TmdbID > 0 {
		score += 10
	}
	if t.Overview != "" {
		score += 10
	}
	if len(t.Tags) > 0 {
		score += 5
	}

	// File information (20 points)
	maxScore += 20
	if len(t.Files) > 0 {
		score += 20
	}

	return (score / maxScore) * 100
}
