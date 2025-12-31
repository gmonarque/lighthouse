package trust

import (
	"sort"
	"time"

	"github.com/gmonarque/lighthouse/internal/decision"
	"github.com/gmonarque/lighthouse/internal/ruleset"
	"github.com/rs/zerolog/log"
)

// rulesetStorage is used to validate ruleset hashes
var rulesetStorage = ruleset.NewStorage()

// Aggregator aggregates decisions from multiple curators
type Aggregator struct {
	policy        *TrustPolicy
	aggPolicy     *AggregationPolicy
	policyStorage *PolicyStorage
}

// NewAggregator creates a new decision aggregator
func NewAggregator(policyStorage *PolicyStorage, aggPolicy *AggregationPolicy) *Aggregator {
	return &Aggregator{
		policyStorage: policyStorage,
		aggPolicy:     aggPolicy,
	}
}

// SetPolicy sets the current trust policy
func (a *Aggregator) SetPolicy(policy *TrustPolicy) {
	a.policy = policy
}

// SetAggregationPolicy sets the aggregation policy
func (a *Aggregator) SetAggregationPolicy(policy *AggregationPolicy) {
	a.aggPolicy = policy
}

// Aggregate aggregates multiple curator decisions for an infohash
func (a *Aggregator) Aggregate(infohash string, decisions []*decision.VerificationDecision) *decision.AggregatedDecision {
	if len(decisions) == 0 {
		return &decision.AggregatedDecision{
			Infohash:      infohash,
			Decision:      decision.DecisionAccept, // Default to accept if no decisions
			Confidence:    0,
			TotalCurators: 0,
			AggregatedAt:  time.Now().UTC(),
		}
	}

	// Filter to only approved curators
	validDecisions := a.filterApprovedCurators(decisions)
	if len(validDecisions) == 0 {
		log.Warn().
			Str("infohash", infohash).
			Int("total", len(decisions)).
			Msg("No decisions from approved curators")

		return &decision.AggregatedDecision{
			Infohash:      infohash,
			Decision:      decision.DecisionAccept, // Default to accept if no valid decisions
			Confidence:    0,
			TotalCurators: 0,
			AggregatedAt:  time.Now().UTC(),
		}
	}

	// Check for legal rejections (always win)
	for _, d := range validDecisions {
		if d.Decision == decision.DecisionReject && d.HasLegalCode() {
			log.Debug().
				Str("infohash", infohash).
				Str("curator", d.CuratorPubkey).
				Msg("Legal rejection takes precedence")

			return &decision.AggregatedDecision{
				Infohash:          infohash,
				Decision:          decision.DecisionReject,
				Confidence:        1.0,
				TotalCurators:     len(validDecisions),
				RejectingCurators: []string{d.CuratorPubkey},
				PrimaryReason:     d.GetPrimaryReason(),
				AllReasons:        d.ReasonCodes,
				SourceDecisions:   []*decision.VerificationDecision{d},
				AggregatedAt:      time.Now().UTC(),
			}
		}
	}

	// Apply aggregation policy
	var result *decision.AggregatedDecision
	switch a.aggPolicy.Mode {
	case AggregationModeAny:
		result = a.aggregateAny(infohash, validDecisions)
	case AggregationModeAll:
		result = a.aggregateAll(infohash, validDecisions)
	case AggregationModeQuorum:
		result = a.aggregateQuorum(infohash, validDecisions)
	case AggregationModeWeighted:
		result = a.aggregateWeighted(infohash, validDecisions)
	default:
		result = a.aggregateAny(infohash, validDecisions)
	}

	result.SourceDecisions = validDecisions
	return result
}

// filterApprovedCurators filters decisions to only those from approved curators
func (a *Aggregator) filterApprovedCurators(decisions []*decision.VerificationDecision) []*decision.VerificationDecision {
	var valid []*decision.VerificationDecision

	for _, d := range decisions {
		// Check if curator is approved
		approved, err := a.policyStorage.IsCuratorApproved(d.CuratorPubkey)
		if err != nil {
			log.Warn().Err(err).Str("curator", d.CuratorPubkey).Msg("Failed to check curator approval")
			continue
		}
		if !approved {
			log.Debug().Str("curator", d.CuratorPubkey).Msg("Skipping decision from unapproved curator")
			continue
		}

		// Optionally verify signature
		if err := decision.VerifyAndValidate(d); err != nil {
			log.Warn().Err(err).Str("curator", d.CuratorPubkey).Msg("Invalid decision signature")
			continue
		}

		// Validate ruleset hash - reject decisions using unapproved/deprecated rulesets
		if d.RulesetHash != "" {
			isApproved, err := rulesetStorage.IsHashApproved(d.RulesetHash)
			if err != nil {
				log.Warn().Err(err).
					Str("curator", d.CuratorPubkey).
					Str("hash", d.RulesetHash).
					Msg("Failed to validate ruleset hash")
				continue
			}
			if !isApproved {
				log.Warn().
					Str("curator", d.CuratorPubkey).
					Str("hash", d.RulesetHash).
					Msg("Decision uses unapproved or deprecated ruleset, skipping")
				continue
			}
		}

		valid = append(valid, d)
	}

	return valid
}

// aggregateAny accepts if any curator accepts
func (a *Aggregator) aggregateAny(infohash string, decisions []*decision.VerificationDecision) *decision.AggregatedDecision {
	result := &decision.AggregatedDecision{
		Infohash:      infohash,
		TotalCurators: len(decisions),
		AggregatedAt:  time.Now().UTC(),
	}

	for _, d := range decisions {
		if d.Decision == decision.DecisionAccept {
			result.AcceptingCurators = append(result.AcceptingCurators, d.CuratorPubkey)
		} else {
			result.RejectingCurators = append(result.RejectingCurators, d.CuratorPubkey)
			result.AllReasons = append(result.AllReasons, d.ReasonCodes...)
		}
	}

	// Accept if at least one accepts
	if len(result.AcceptingCurators) > 0 {
		result.Decision = decision.DecisionAccept
		result.Confidence = float64(len(result.AcceptingCurators)) / float64(len(decisions))
	} else {
		result.Decision = decision.DecisionReject
		result.Confidence = float64(len(result.RejectingCurators)) / float64(len(decisions))
		result.PrimaryReason = getPrimaryReason(result.AllReasons)
	}

	return result
}

// aggregateAll accepts only if all curators accept
func (a *Aggregator) aggregateAll(infohash string, decisions []*decision.VerificationDecision) *decision.AggregatedDecision {
	result := &decision.AggregatedDecision{
		Infohash:      infohash,
		TotalCurators: len(decisions),
		AggregatedAt:  time.Now().UTC(),
	}

	for _, d := range decisions {
		if d.Decision == decision.DecisionAccept {
			result.AcceptingCurators = append(result.AcceptingCurators, d.CuratorPubkey)
		} else {
			result.RejectingCurators = append(result.RejectingCurators, d.CuratorPubkey)
			result.AllReasons = append(result.AllReasons, d.ReasonCodes...)
		}
	}

	// Accept only if all accept
	if len(result.RejectingCurators) == 0 {
		result.Decision = decision.DecisionAccept
		result.Confidence = 1.0
	} else {
		result.Decision = decision.DecisionReject
		result.Confidence = float64(len(result.RejectingCurators)) / float64(len(decisions))
		result.PrimaryReason = getPrimaryReason(result.AllReasons)
	}

	return result
}

// aggregateQuorum accepts if N of M curators accept
func (a *Aggregator) aggregateQuorum(infohash string, decisions []*decision.VerificationDecision) *decision.AggregatedDecision {
	result := &decision.AggregatedDecision{
		Infohash:      infohash,
		TotalCurators: len(decisions),
		AggregatedAt:  time.Now().UTC(),
	}

	for _, d := range decisions {
		if d.Decision == decision.DecisionAccept {
			result.AcceptingCurators = append(result.AcceptingCurators, d.CuratorPubkey)
		} else {
			result.RejectingCurators = append(result.RejectingCurators, d.CuratorPubkey)
			result.AllReasons = append(result.AllReasons, d.ReasonCodes...)
		}
	}

	quorum := a.aggPolicy.QuorumRequired
	if quorum <= 0 {
		quorum = (len(decisions) / 2) + 1 // Default to majority
	}

	if len(result.AcceptingCurators) >= quorum {
		result.Decision = decision.DecisionAccept
		result.Confidence = float64(len(result.AcceptingCurators)) / float64(len(decisions))
	} else if len(result.RejectingCurators) >= quorum {
		result.Decision = decision.DecisionReject
		result.Confidence = float64(len(result.RejectingCurators)) / float64(len(decisions))
		result.PrimaryReason = getPrimaryReason(result.AllReasons)
	} else {
		// Not enough votes yet - default to pending/accept
		result.Decision = decision.DecisionAccept
		result.Confidence = float64(len(result.AcceptingCurators)) / float64(quorum)
	}

	return result
}

// aggregateWeighted accepts based on weighted votes
func (a *Aggregator) aggregateWeighted(infohash string, decisions []*decision.VerificationDecision) *decision.AggregatedDecision {
	result := &decision.AggregatedDecision{
		Infohash:      infohash,
		TotalCurators: len(decisions),
		AggregatedAt:  time.Now().UTC(),
	}

	acceptWeight := 0
	rejectWeight := 0
	totalWeight := 0

	for _, d := range decisions {
		// Get curator weight
		weight := 1
		if a.policy != nil {
			weight = a.policy.GetCuratorWeight(d.CuratorPubkey)
			if weight == 0 {
				weight = 1 // Default weight
			}
		}

		totalWeight += weight

		if d.Decision == decision.DecisionAccept {
			result.AcceptingCurators = append(result.AcceptingCurators, d.CuratorPubkey)
			acceptWeight += weight
		} else {
			result.RejectingCurators = append(result.RejectingCurators, d.CuratorPubkey)
			result.AllReasons = append(result.AllReasons, d.ReasonCodes...)
			rejectWeight += weight
		}
	}

	threshold := a.aggPolicy.WeightThreshold
	if threshold <= 0 {
		threshold = (totalWeight / 2) + 1 // Default to majority
	}

	if acceptWeight >= threshold {
		result.Decision = decision.DecisionAccept
		result.Confidence = float64(acceptWeight) / float64(totalWeight)
	} else if rejectWeight >= threshold {
		result.Decision = decision.DecisionReject
		result.Confidence = float64(rejectWeight) / float64(totalWeight)
		result.PrimaryReason = getPrimaryReason(result.AllReasons)
	} else {
		// Not enough weight - default to accept
		result.Decision = decision.DecisionAccept
		result.Confidence = float64(acceptWeight) / float64(threshold)
	}

	return result
}

// getPrimaryReason returns the highest priority reason from a list
func getPrimaryReason(reasons []ruleset.ReasonCode) ruleset.ReasonCode {
	if len(reasons) == 0 {
		return ""
	}

	// Sort by priority descending
	sorted := make([]ruleset.ReasonCode, len(reasons))
	copy(sorted, reasons)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})

	return sorted[0]
}

// ShouldAccept is a convenience method to check if content should be accepted
func (a *Aggregator) ShouldAccept(infohash string, decisions []*decision.VerificationDecision) bool {
	result := a.Aggregate(infohash, decisions)
	return result.Decision == decision.DecisionAccept
}
