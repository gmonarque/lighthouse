package ruleset

import (
	"testing"
)

func TestReasonCode_IsDeterministic(t *testing.T) {
	tests := []struct {
		code     ReasonCode
		expected bool
	}{
		{ReasonLegalDMCA, true},
		{ReasonLegalIllegal, true},
		{ReasonAbuseSpam, true},
		{ReasonSemBadMeta, false},
		{ReasonSemDuplicateExact, true},
		{ReasonSemLowQuality, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			result := tt.code.IsDeterministic()
			if result != tt.expected {
				t.Errorf("IsDeterministic() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReasonCode_IsLegal(t *testing.T) {
	tests := []struct {
		code     ReasonCode
		expected bool
	}{
		{ReasonLegalDMCA, true},
		{ReasonLegalIllegal, true},
		{ReasonAbuseSpam, false},
		{ReasonSemBadMeta, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			result := tt.code.IsLegal()
			if result != tt.expected {
				t.Errorf("IsLegal() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReasonCode_IsAbuse(t *testing.T) {
	tests := []struct {
		code     ReasonCode
		expected bool
	}{
		{ReasonLegalDMCA, false},
		{ReasonAbuseSpam, true},
		{ReasonAbuseMalware, true},
		{ReasonSemBadMeta, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			result := tt.code.IsAbuse()
			if result != tt.expected {
				t.Errorf("IsAbuse() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReasonCode_IsSemantic(t *testing.T) {
	tests := []struct {
		code     ReasonCode
		expected bool
	}{
		{ReasonLegalDMCA, false},
		{ReasonAbuseSpam, false},
		{ReasonSemBadMeta, true},
		{ReasonSemLowQuality, true},
		{ReasonSemDuplicateExact, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			result := tt.code.IsSemantic()
			if result != tt.expected {
				t.Errorf("IsSemantic() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReasonCode_Priority(t *testing.T) {
	// Legal should have highest priority
	if ReasonLegalDMCA.Priority() <= ReasonAbuseSpam.Priority() {
		t.Error("Legal codes should have higher priority than abuse codes")
	}

	// Abuse should have higher priority than semantic
	if ReasonAbuseMalware.Priority() <= ReasonSemBadMeta.Priority() {
		t.Error("Abuse codes should have higher priority than semantic codes")
	}
}

func TestReasonCode_Description(t *testing.T) {
	desc := ReasonLegalDMCA.Description()
	if desc == "" || desc == "Unknown reason" {
		t.Error("Description() should return meaningful description for known codes")
	}
}

func TestRuleset_ComputeHash(t *testing.T) {
	rs := &Ruleset{
		ID:      "test-1",
		Version: "1.0.0",
		Type:    RulesetTypeCensoring,
		Rules: []Rule{
			{
				ID:     "rule-1",
				Code:   ReasonLegalDMCA,
				Type:   "deterministic",
				Action: "reject",
				Condition: Condition{
					Type: ConditionTypeInfohashList,
				},
				Enabled: true,
			},
		},
	}

	hash1 := rs.ComputeHash()
	if hash1 == "" {
		t.Error("ComputeHash() returned empty string")
	}

	// Same content should produce same hash
	hash2 := rs.ComputeHash()
	if hash1 != hash2 {
		t.Errorf("ComputeHash() inconsistent: %s != %s", hash1, hash2)
	}

	// Different content should produce different hash
	rs.Rules[0].ID = "rule-2"
	hash3 := rs.ComputeHash()
	if hash1 == hash3 {
		t.Error("ComputeHash() should produce different hash for different content")
	}
}

func TestRuleset_Validate(t *testing.T) {
	// Valid ruleset
	validRS := &Ruleset{
		ID:      "test-1",
		Version: "1.0.0",
		Type:    RulesetTypeCensoring,
		Rules: []Rule{
			{
				ID:     "rule-1",
				Code:   ReasonLegalDMCA,
				Type:   "deterministic",
				Action: "reject",
				Condition: Condition{
					Type: ConditionTypeInfohashList,
				},
				Enabled: true,
			},
		},
	}

	if err := validRS.Validate(); err != nil {
		t.Errorf("Validate() error for valid ruleset: %v", err)
	}

	// Invalid ruleset - missing ID
	invalidRS := &Ruleset{
		Version: "1.0.0",
		Type:    RulesetTypeCensoring,
		Rules: []Rule{
			{
				ID:     "rule-1",
				Code:   ReasonLegalDMCA,
				Type:   "deterministic",
				Action: "reject",
				Condition: Condition{
					Type: ConditionTypeInfohashList,
				},
			},
		},
	}

	if err := invalidRS.Validate(); err == nil {
		t.Error("Validate() should error for missing ID")
	}
}

func TestTorrentData_HasRequiredFields(t *testing.T) {
	data := &TorrentData{
		Name:     "Test Torrent",
		Size:     1024000,
		Category: 2000,
	}

	// Should pass with present fields
	has, missing := data.HasRequiredFields([]string{"name", "size"})
	if !has {
		t.Errorf("HasRequiredFields() should pass, missing: %v", missing)
	}

	// Should fail with missing fields (name counts as title substitute)
	has2, missing2 := data.HasRequiredFields([]string{"uploader", "info_hash"})
	if has2 {
		t.Error("HasRequiredFields() should fail for missing fields")
	}
	if len(missing2) != 2 {
		t.Errorf("Missing fields count = %d, want 2", len(missing2))
	}
}

func TestTorrentData_MetadataScore(t *testing.T) {
	// Empty data should have low score
	emptyData := &TorrentData{}
	if emptyData.MetadataScore() > 10 {
		t.Errorf("Empty data score = %f, want < 10", emptyData.MetadataScore())
	}

	// Rich data should have high score
	richData := &TorrentData{
		Name:     "Test Movie 2023",
		Size:     5000000000,
		Category: 2000,
		Title:    "Test Movie",
		Year:     2023,
		ImdbID:   "tt1234567",
		Overview: "A great movie",
		Tags:     []string{"movie", "action"},
		Files: []FileEntry{
			{Path: "movie.mkv", Size: 5000000000},
		},
	}
	if richData.MetadataScore() < 80 {
		t.Errorf("Rich data score = %f, want >= 80", richData.MetadataScore())
	}
}

func TestEvaluationResult(t *testing.T) {
	result := &EvaluationResult{
		Passed: true,
		Score:  0.95,
		MatchedRules: []MatchedRule{
			{
				RuleID: "rule-1",
				Code:   ReasonAbuseSpam,
				Action: "reject",
			},
		},
	}

	if !result.Passed {
		t.Error("Passed should be true")
	}
	if result.Score != 0.95 {
		t.Errorf("Score = %f, want 0.95", result.Score)
	}
	if len(result.MatchedRules) != 1 {
		t.Errorf("MatchedRules length = %d, want 1", len(result.MatchedRules))
	}
}
