package decision

import (
	"testing"

	"github.com/gmonarque/lighthouse/internal/ruleset"
)

func TestNewAcceptDecision(t *testing.T) {
	d := NewAcceptDecision("event123", "infohash456", "pubkey789")

	if d.Decision != DecisionAccept {
		t.Errorf("Decision = %s, want %s", d.Decision, DecisionAccept)
	}
	if d.TargetEventID != "event123" {
		t.Errorf("TargetEventID = %s, want event123", d.TargetEventID)
	}
	if d.TargetInfohash != "infohash456" {
		t.Errorf("TargetInfohash = %s, want infohash456", d.TargetInfohash)
	}
	if d.CuratorPubkey != "pubkey789" {
		t.Errorf("CuratorPubkey = %s, want pubkey789", d.CuratorPubkey)
	}
	if d.DecisionID == "" {
		t.Error("DecisionID should not be empty")
	}
	if d.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestNewRejectDecision(t *testing.T) {
	reasons := []ruleset.ReasonCode{ruleset.ReasonLegalDMCA, ruleset.ReasonAbuseSpam}
	d := NewRejectDecision("event123", "infohash456", "pubkey789", reasons)

	if d.Decision != DecisionReject {
		t.Errorf("Decision = %s, want %s", d.Decision, DecisionReject)
	}
	if len(d.ReasonCodes) != 2 {
		t.Errorf("ReasonCodes length = %d, want 2", len(d.ReasonCodes))
	}
}

func TestVerificationDecision_HasLegalCode(t *testing.T) {
	tests := []struct {
		name     string
		codes    []ruleset.ReasonCode
		expected bool
	}{
		{
			name:     "with legal code",
			codes:    []ruleset.ReasonCode{ruleset.ReasonLegalDMCA},
			expected: true,
		},
		{
			name:     "without legal code",
			codes:    []ruleset.ReasonCode{ruleset.ReasonAbuseSpam},
			expected: false,
		},
		{
			name:     "mixed codes",
			codes:    []ruleset.ReasonCode{ruleset.ReasonAbuseSpam, ruleset.ReasonLegalIllegal},
			expected: true,
		},
		{
			name:     "empty codes",
			codes:    []ruleset.ReasonCode{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &VerificationDecision{ReasonCodes: tt.codes}
			result := d.HasLegalCode()
			if result != tt.expected {
				t.Errorf("HasLegalCode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVerificationDecision_HasDeterministicCode(t *testing.T) {
	tests := []struct {
		name     string
		codes    []ruleset.ReasonCode
		expected bool
	}{
		{
			name:     "with deterministic code",
			codes:    []ruleset.ReasonCode{ruleset.ReasonLegalDMCA},
			expected: true,
		},
		{
			name:     "without deterministic code",
			codes:    []ruleset.ReasonCode{ruleset.ReasonSemBadMeta},
			expected: false,
		},
		{
			name:     "mixed codes",
			codes:    []ruleset.ReasonCode{ruleset.ReasonSemBadMeta, ruleset.ReasonAbuseSpam},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &VerificationDecision{ReasonCodes: tt.codes}
			result := d.HasDeterministicCode()
			if result != tt.expected {
				t.Errorf("HasDeterministicCode() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVerificationDecision_GetPrimaryReason(t *testing.T) {
	// Test with legal code (should be primary due to highest priority)
	d1 := &VerificationDecision{
		ReasonCodes: []ruleset.ReasonCode{ruleset.ReasonAbuseSpam, ruleset.ReasonLegalDMCA},
	}
	primary1 := d1.GetPrimaryReason()
	if primary1 != ruleset.ReasonLegalDMCA {
		t.Errorf("GetPrimaryReason() = %s, want %s", primary1, ruleset.ReasonLegalDMCA)
	}

	// Test with no codes
	d2 := &VerificationDecision{}
	primary2 := d2.GetPrimaryReason()
	if primary2 != "" {
		t.Errorf("GetPrimaryReason() for empty = %s, want empty", primary2)
	}
}

func TestVerificationDecision_SetRulesetInfo(t *testing.T) {
	d := &VerificationDecision{}
	d.SetRulesetInfo("censoring", "1.0.0", "hash123")

	if d.RulesetType != "censoring" {
		t.Errorf("RulesetType = %s, want censoring", d.RulesetType)
	}
	if d.RulesetVersion != "1.0.0" {
		t.Errorf("RulesetVersion = %s, want 1.0.0", d.RulesetVersion)
	}
	if d.RulesetHash != "hash123" {
		t.Errorf("RulesetHash = %s, want hash123", d.RulesetHash)
	}
}

func TestVerificationDecision_ComputeID(t *testing.T) {
	d1 := NewAcceptDecision("event1", "hash1", "pubkey1")
	d2 := NewAcceptDecision("event1", "hash1", "pubkey1")
	d3 := NewAcceptDecision("event2", "hash1", "pubkey1")

	// Same inputs at different times should produce different IDs (due to CreatedAt)
	// But same decision should have consistent ID
	id1 := d1.ComputeID()
	if id1 == "" {
		t.Error("ComputeID() should not return empty string")
	}

	// Different inputs should produce different IDs
	if d1.DecisionID == d3.DecisionID {
		t.Error("Different decisions should have different IDs")
	}

	_ = d2 // Used to verify the test setup
}

func TestVerificationDecision_JSON(t *testing.T) {
	original := NewAcceptDecision("event123", "infohash456", "pubkey789")
	original.SetRulesetInfo("censoring", "1.0.0", "hash123")

	// Serialize
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	// Deserialize
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON() error: %v", err)
	}

	// Compare
	if restored.DecisionID != original.DecisionID {
		t.Errorf("DecisionID mismatch: %s != %s", restored.DecisionID, original.DecisionID)
	}
	if restored.Decision != original.Decision {
		t.Errorf("Decision mismatch: %s != %s", restored.Decision, original.Decision)
	}
	if restored.RulesetHash != original.RulesetHash {
		t.Errorf("RulesetHash mismatch: %s != %s", restored.RulesetHash, original.RulesetHash)
	}
}

func TestDecisionSummary(t *testing.T) {
	summary := &DecisionSummary{
		Infohash:       "hash123",
		TotalDecisions: 7,
		AcceptCount:    5,
		RejectCount:    2,
		HasLegalReject: false,
		FinalDecision:  DecisionAccept,
	}

	if summary.AcceptCount != 5 {
		t.Errorf("AcceptCount = %d, want 5", summary.AcceptCount)
	}
	if summary.TotalDecisions != 7 {
		t.Errorf("TotalDecisions = %d, want 7", summary.TotalDecisions)
	}
	if summary.FinalDecision != DecisionAccept {
		t.Errorf("FinalDecision = %s, want %s", summary.FinalDecision, DecisionAccept)
	}
}

func TestAggregatedDecision(t *testing.T) {
	agg := &AggregatedDecision{
		Infohash:          "hash123",
		Decision:          DecisionAccept,
		Confidence:        0.8,
		TotalCurators:     5,
		AcceptingCurators: []string{"curator1", "curator2", "curator3", "curator4"},
		RejectingCurators: []string{"curator5"},
	}

	if agg.Decision != DecisionAccept {
		t.Errorf("Decision = %s, want %s", agg.Decision, DecisionAccept)
	}
	if agg.Confidence != 0.8 {
		t.Errorf("Confidence = %f, want 0.8", agg.Confidence)
	}
	if len(agg.AcceptingCurators) != 4 {
		t.Errorf("AcceptingCurators length = %d, want 4", len(agg.AcceptingCurators))
	}
	if agg.TotalCurators != 5 {
		t.Errorf("TotalCurators = %d, want 5", agg.TotalCurators)
	}
}

func TestDecisionFilter(t *testing.T) {
	filter := &DecisionFilter{
		TargetInfohash: "hash123",
		CuratorPubkey:  "pubkey456",
		Decision:       DecisionReject,
		Limit:          10,
		Offset:         0,
	}

	if filter.TargetInfohash != "hash123" {
		t.Errorf("TargetInfohash = %s, want hash123", filter.TargetInfohash)
	}
	if filter.Decision != DecisionReject {
		t.Errorf("Decision = %s, want %s", filter.Decision, DecisionReject)
	}
}
