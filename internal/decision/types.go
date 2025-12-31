// Package decision handles verification decisions from curators
package decision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/gmonarque/lighthouse/internal/ruleset"
)

// Decision represents the outcome of a verification
type Decision string

const (
	DecisionAccept Decision = "accept"
	DecisionReject Decision = "reject"
)

// VerificationDecision represents a curator's decision on a torrent event
type VerificationDecision struct {
	ID                 string               `json:"id,omitempty"`
	DecisionID         string               `json:"decision_id"`
	TargetEventID      string               `json:"target_event_id"`
	TargetInfohash     string               `json:"target_infohash"`
	Decision           Decision             `json:"decision"`
	ReasonCodes        []ruleset.ReasonCode `json:"reason_codes,omitempty"`
	RulesetType        string               `json:"ruleset_type,omitempty"`
	RulesetVersion     string               `json:"ruleset_version,omitempty"`
	RulesetHash        string               `json:"ruleset_hash,omitempty"`
	CuratorPubkey      string               `json:"curator_pubkey"`
	Signature          string               `json:"signature"`
	CreatedAt          time.Time            `json:"created_at"`
	ProcessedAt        *time.Time           `json:"processed_at,omitempty"`
	AggregatedDecision *Decision            `json:"aggregated_decision,omitempty"`
}

// NewAcceptDecision creates a new accept decision
func NewAcceptDecision(targetEventID, targetInfohash, curatorPubkey string) *VerificationDecision {
	d := &VerificationDecision{
		TargetEventID:  targetEventID,
		TargetInfohash: targetInfohash,
		Decision:       DecisionAccept,
		CuratorPubkey:  curatorPubkey,
		CreatedAt:      time.Now().UTC(),
	}
	d.DecisionID = d.ComputeID()
	return d
}

// NewRejectDecision creates a new reject decision with reason codes
func NewRejectDecision(targetEventID, targetInfohash, curatorPubkey string, reasons []ruleset.ReasonCode) *VerificationDecision {
	d := &VerificationDecision{
		TargetEventID:  targetEventID,
		TargetInfohash: targetInfohash,
		Decision:       DecisionReject,
		ReasonCodes:    reasons,
		CuratorPubkey:  curatorPubkey,
		CreatedAt:      time.Now().UTC(),
	}
	d.DecisionID = d.ComputeID()
	return d
}

// ComputeID computes a unique ID for the decision
func (d *VerificationDecision) ComputeID() string {
	data := struct {
		TargetEventID  string    `json:"target_event_id"`
		TargetInfohash string    `json:"target_infohash"`
		Decision       Decision  `json:"decision"`
		CuratorPubkey  string    `json:"curator_pubkey"`
		CreatedAt      time.Time `json:"created_at"`
	}{
		TargetEventID:  d.TargetEventID,
		TargetInfohash: d.TargetInfohash,
		Decision:       d.Decision,
		CuratorPubkey:  d.CuratorPubkey,
		CreatedAt:      d.CreatedAt,
	}

	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter ID
}

// SetRulesetInfo sets the ruleset information used for this decision
func (d *VerificationDecision) SetRulesetInfo(rulesetType, version, hash string) {
	d.RulesetType = rulesetType
	d.RulesetVersion = version
	d.RulesetHash = hash
}

// HasLegalCode returns true if any reason code is legal-related
func (d *VerificationDecision) HasLegalCode() bool {
	for _, code := range d.ReasonCodes {
		if code.IsLegal() {
			return true
		}
	}
	return false
}

// HasDeterministicCode returns true if any reason code is deterministic
func (d *VerificationDecision) HasDeterministicCode() bool {
	for _, code := range d.ReasonCodes {
		if code.IsDeterministic() {
			return true
		}
	}
	return false
}

// GetPrimaryReason returns the highest priority reason code
func (d *VerificationDecision) GetPrimaryReason() ruleset.ReasonCode {
	if len(d.ReasonCodes) == 0 {
		return ""
	}

	primary := d.ReasonCodes[0]
	for _, code := range d.ReasonCodes[1:] {
		if code.Priority() > primary.Priority() {
			primary = code
		}
	}
	return primary
}

// ToJSON serializes the decision to JSON
func (d *VerificationDecision) ToJSON() ([]byte, error) {
	return json.Marshal(d)
}

// FromJSON deserializes a decision from JSON
func FromJSON(data []byte) (*VerificationDecision, error) {
	var d VerificationDecision
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// DecisionSummary provides a summary of decisions for an infohash
type DecisionSummary struct {
	Infohash       string   `json:"infohash"`
	TotalDecisions int      `json:"total_decisions"`
	AcceptCount    int      `json:"accept_count"`
	RejectCount    int      `json:"reject_count"`
	HasLegalReject bool     `json:"has_legal_reject"`
	FinalDecision  Decision `json:"final_decision"`
}

// AggregatedDecision represents the aggregated result from multiple curators
type AggregatedDecision struct {
	Infohash          string                  `json:"infohash"`
	Decision          Decision                `json:"decision"`
	Confidence        float64                 `json:"confidence"`
	TotalCurators     int                     `json:"total_curators"`
	AcceptingCurators []string                `json:"accepting_curators,omitempty"`
	RejectingCurators []string                `json:"rejecting_curators,omitempty"`
	PrimaryReason     ruleset.ReasonCode      `json:"primary_reason,omitempty"`
	AllReasons        []ruleset.ReasonCode    `json:"all_reasons,omitempty"`
	SourceDecisions   []*VerificationDecision `json:"source_decisions,omitempty"`
	AggregatedAt      time.Time               `json:"aggregated_at"`
}

// DecisionFilter for querying decisions
type DecisionFilter struct {
	TargetInfohash string
	CuratorPubkey  string
	Decision       Decision
	RulesetHash    string
	Since          *time.Time
	Until          *time.Time
	Limit          int
	Offset         int
}
