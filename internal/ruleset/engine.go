package ruleset

import (
	"regexp"
	"strings"
	"sync"
)

// Engine evaluates torrents against rulesets
type Engine struct {
	censoring *Ruleset
	semantic  *Ruleset
	mu        sync.RWMutex

	// Cached compiled patterns
	regexCache map[string]*regexp.Regexp
	listCache  map[string]map[string]bool
}

// NewEngine creates a new ruleset engine
func NewEngine() *Engine {
	return &Engine{
		regexCache: make(map[string]*regexp.Regexp),
		listCache:  make(map[string]map[string]bool),
	}
}

// SetCensoringRuleset sets the active censoring ruleset
func (e *Engine) SetCensoringRuleset(r *Ruleset) error {
	if r != nil && r.Type != RulesetTypeCensoring {
		return ErrInvalidRulesetType
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.censoring = r
	e.clearCache()
	return nil
}

// SetSemanticRuleset sets the active semantic ruleset
func (e *Engine) SetSemanticRuleset(r *Ruleset) error {
	if r != nil && r.Type != RulesetTypeSemantic {
		return ErrInvalidRulesetType
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.semantic = r
	e.clearCache()
	return nil
}

// GetCensoringRuleset returns the active censoring ruleset
func (e *Engine) GetCensoringRuleset() *Ruleset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.censoring
}

// GetSemanticRuleset returns the active semantic ruleset
func (e *Engine) GetSemanticRuleset() *Ruleset {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.semantic
}

// clearCache clears all cached data
func (e *Engine) clearCache() {
	e.regexCache = make(map[string]*regexp.Regexp)
	e.listCache = make(map[string]map[string]bool)
}

// EvaluateCensoring evaluates a torrent against the censoring ruleset
// Returns matched rules that would cause rejection (deterministic)
func (e *Engine) EvaluateCensoring(t *TorrentData) *EvaluationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &EvaluationResult{Passed: true}

	if e.censoring == nil {
		return result
	}

	for _, rule := range e.censoring.Rules {
		if !rule.Enabled {
			continue
		}

		if matched, score := e.evaluateRule(&rule, t); matched {
			result.MatchedRules = append(result.MatchedRules, MatchedRule{
				RuleID:      rule.ID,
				Code:        rule.Code,
				Action:      rule.Action,
				Description: rule.Description,
				Score:       score,
			})

			if rule.Action == "reject" {
				result.Passed = false
			}
		}
	}

	return result
}

// EvaluateSemantic evaluates a torrent against the semantic ruleset
// Returns matched rules (may be probabilistic with scores)
func (e *Engine) EvaluateSemantic(t *TorrentData) *EvaluationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := &EvaluationResult{Passed: true}

	if e.semantic == nil {
		return result
	}

	totalScore := 0.0
	matchCount := 0

	for _, rule := range e.semantic.Rules {
		if !rule.Enabled {
			continue
		}

		if matched, score := e.evaluateRule(&rule, t); matched {
			result.MatchedRules = append(result.MatchedRules, MatchedRule{
				RuleID:      rule.ID,
				Code:        rule.Code,
				Action:      rule.Action,
				Description: rule.Description,
				Score:       score,
			})

			totalScore += score
			matchCount++

			// Deterministic semantic rules (like exact duplicate) fail immediately
			if rule.Type == "deterministic" && rule.Action == "reject" {
				result.Passed = false
			}
		}
	}

	if matchCount > 0 {
		result.Score = totalScore / float64(matchCount)
	}

	return result
}

// EvaluateAll evaluates a torrent against both rulesets
func (e *Engine) EvaluateAll(t *TorrentData) (*EvaluationResult, *EvaluationResult) {
	return e.EvaluateCensoring(t), e.EvaluateSemantic(t)
}

// evaluateRule evaluates a single rule against the torrent
func (e *Engine) evaluateRule(rule *Rule, t *TorrentData) (bool, float64) {
	switch rule.Condition.Type {
	case ConditionTypeInfohashList:
		return e.evalInfohashList(rule, t)
	case ConditionTypePubkeyList:
		return e.evalPubkeyList(rule, t)
	case ConditionTypeRegex:
		return e.evalRegex(rule, t)
	case ConditionTypeMetadataScore:
		return e.evalMetadataScore(rule, t)
	case ConditionTypeSizeRange:
		return e.evalSizeRange(rule, t)
	case ConditionTypeCategoryMatch:
		return e.evalCategoryMatch(rule, t)
	case ConditionTypeTagMatch:
		return e.evalTagMatch(rule, t)
	default:
		return false, 0
	}
}

// evalInfohashList checks if the torrent's infohash is in a blocklist
func (e *Engine) evalInfohashList(rule *Rule, t *TorrentData) (bool, float64) {
	list := e.getOrCreateList(rule.ID, rule.Condition.Values)
	if list == nil {
		return false, 0
	}

	normalizedHash := strings.ToLower(t.InfoHash)
	if list[normalizedHash] {
		return true, 1.0
	}
	return false, 0
}

// evalPubkeyList checks if the uploader is in a blocklist
func (e *Engine) evalPubkeyList(rule *Rule, t *TorrentData) (bool, float64) {
	list := e.getOrCreateList(rule.ID, rule.Condition.Values)
	if list == nil {
		return false, 0
	}

	normalizedPubkey := strings.ToLower(t.Uploader)
	if list[normalizedPubkey] {
		return true, 1.0
	}
	return false, 0
}

// evalRegex checks if a field matches a regex pattern
func (e *Engine) evalRegex(rule *Rule, t *TorrentData) (bool, float64) {
	pattern, ok := rule.Condition.Value.(string)
	if !ok {
		return false, 0
	}

	re := e.getOrCreateRegex(rule.ID, pattern)
	if re == nil {
		return false, 0
	}

	field := e.getFieldValue(rule.Condition.Field, t)
	if re.MatchString(field) {
		return true, 1.0
	}
	return false, 0
}

// evalMetadataScore checks if the metadata quality score meets the threshold
func (e *Engine) evalMetadataScore(rule *Rule, t *TorrentData) (bool, float64) {
	// Check required fields
	if len(rule.Condition.MinFields) > 0 {
		hasAll, _ := t.HasRequiredFields(rule.Condition.MinFields)
		if !hasAll {
			return true, 1.0 - (t.MetadataScore() / 100)
		}
	}

	// Check score threshold
	if rule.Condition.Threshold > 0 {
		score := t.MetadataScore()
		if score < rule.Condition.Threshold*100 {
			return true, 1.0 - (score / 100)
		}
	}

	return false, 0
}

// evalSizeRange checks if the torrent size is within a range
func (e *Engine) evalSizeRange(rule *Rule, t *TorrentData) (bool, float64) {
	extra := rule.Condition.Extra
	if extra == nil {
		return false, 0
	}

	minSize, _ := extra["min"].(float64)
	maxSize, _ := extra["max"].(float64)

	if minSize > 0 && t.Size < int64(minSize) {
		return true, 1.0
	}
	if maxSize > 0 && t.Size > int64(maxSize) {
		return true, 1.0
	}

	return false, 0
}

// evalCategoryMatch checks if the torrent category matches
func (e *Engine) evalCategoryMatch(rule *Rule, t *TorrentData) (bool, float64) {
	categories := rule.Condition.Values
	if len(categories) == 0 {
		return false, 0
	}

	for _, cat := range categories {
		catInt, ok := cat.(float64)
		if ok && int(catInt) == t.Category {
			return true, 1.0
		}
	}

	return false, 0
}

// evalTagMatch checks if the torrent has specific tags
func (e *Engine) evalTagMatch(rule *Rule, t *TorrentData) (bool, float64) {
	requiredTags := rule.Condition.Values
	if len(requiredTags) == 0 {
		return false, 0
	}

	torrentTags := make(map[string]bool)
	for _, tag := range t.Tags {
		torrentTags[strings.ToLower(tag)] = true
	}

	matchCount := 0
	for _, tag := range requiredTags {
		tagStr, ok := tag.(string)
		if ok && torrentTags[strings.ToLower(tagStr)] {
			matchCount++
		}
	}

	if matchCount > 0 {
		return true, float64(matchCount) / float64(len(requiredTags))
	}

	return false, 0
}

// getOrCreateList gets or creates a cached list from values
func (e *Engine) getOrCreateList(ruleID string, values []interface{}) map[string]bool {
	if cached, ok := e.listCache[ruleID]; ok {
		return cached
	}

	list := make(map[string]bool)
	for _, v := range values {
		if s, ok := v.(string); ok {
			list[strings.ToLower(s)] = true
		}
	}

	e.listCache[ruleID] = list
	return list
}

// getOrCreateRegex gets or creates a cached compiled regex
func (e *Engine) getOrCreateRegex(ruleID, pattern string) *regexp.Regexp {
	if cached, ok := e.regexCache[ruleID]; ok {
		return cached
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	e.regexCache[ruleID] = re
	return re
}

// getFieldValue gets the value of a field from the torrent data
func (e *Engine) getFieldValue(field string, t *TorrentData) string {
	switch field {
	case "name":
		return t.Name
	case "title":
		return t.Title
	case "info_hash":
		return t.InfoHash
	case "uploader":
		return t.Uploader
	case "overview":
		return t.Overview
	case "imdb_id":
		return t.ImdbID
	default:
		return ""
	}
}

// ShouldReject determines if the torrent should be rejected based on evaluation results
func ShouldReject(censoring, semantic *EvaluationResult, threshold float64) (bool, []ReasonCode) {
	reasons := []ReasonCode{}

	// Censoring rules are always definitive
	if censoring != nil && !censoring.Passed {
		for _, rule := range censoring.MatchedRules {
			if rule.Action == "reject" {
				reasons = append(reasons, rule.Code)
			}
		}
		return true, reasons
	}

	// Semantic rules depend on threshold
	if semantic != nil && !semantic.Passed {
		for _, rule := range semantic.MatchedRules {
			if rule.Action == "reject" {
				reasons = append(reasons, rule.Code)
			}
		}
		return true, reasons
	}

	// Check semantic score threshold
	if semantic != nil && semantic.Score > 0 && semantic.Score >= threshold {
		for _, rule := range semantic.MatchedRules {
			reasons = append(reasons, rule.Code)
		}
		return true, reasons
	}

	return false, reasons
}
