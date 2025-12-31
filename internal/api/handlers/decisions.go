package handlers

import (
	"net/http"
	"strconv"

	"github.com/gmonarque/lighthouse/internal/decision"
	"github.com/gmonarque/lighthouse/internal/ruleset"
	"github.com/go-chi/chi/v5"
)

// DecisionResponse represents a decision in API responses
type DecisionResponse struct {
	DecisionID     string   `json:"decision_id"`
	Decision       string   `json:"decision"`
	ReasonCodes    []string `json:"reason_codes"`
	TargetEventID  string   `json:"target_event_id"`
	TargetInfohash string   `json:"target_infohash"`
	CuratorPubkey  string   `json:"curator_pubkey"`
	RulesetType    string   `json:"ruleset_type,omitempty"`
	RulesetVersion string   `json:"ruleset_version,omitempty"`
	RulesetHash    string   `json:"ruleset_hash,omitempty"`
	CreatedAt      string   `json:"created_at"`
	Signature      string   `json:"signature,omitempty"`
}

// decisionStorage is the storage instance
var decisionStorage *decision.Storage

// SetDecisionStorage sets the decision storage instance
func SetDecisionStorage(s *decision.Storage) {
	decisionStorage = s
}

// GetDecisions returns all verification decisions with optional filtering
func GetDecisions(w http.ResponseWriter, r *http.Request) {
	if decisionStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"decisions": []DecisionResponse{},
			"total":     0,
		})
		return
	}

	// Parse query params
	filter := decision.DecisionFilter{
		TargetInfohash: r.URL.Query().Get("infohash"),
		CuratorPubkey:  r.URL.Query().Get("curator"),
	}

	if d := r.URL.Query().Get("decision"); d != "" {
		filter.Decision = decision.Decision(d)
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	} else {
		filter.Limit = 50
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	decisions, err := decisionStorage.Query(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to query decisions")
		return
	}

	// Get stats for total count
	stats, _ := decisionStorage.GetStats()
	total := int64(0)
	if stats != nil {
		if t, ok := stats["total_decisions"].(int64); ok {
			total = t
		}
	}

	response := make([]DecisionResponse, 0, len(decisions))
	for _, d := range decisions {
		reasonCodes := make([]string, len(d.ReasonCodes))
		for i, rc := range d.ReasonCodes {
			reasonCodes[i] = string(rc)
		}

		response = append(response, DecisionResponse{
			DecisionID:     d.DecisionID,
			Decision:       string(d.Decision),
			ReasonCodes:    reasonCodes,
			TargetEventID:  d.TargetEventID,
			TargetInfohash: d.TargetInfohash,
			CuratorPubkey:  d.CuratorPubkey,
			RulesetType:    d.RulesetType,
			RulesetVersion: d.RulesetVersion,
			RulesetHash:    d.RulesetHash,
			CreatedAt:      d.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Signature:      d.Signature,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"decisions": response,
		"total":     total,
	})
}

// GetDecisionsByInfohash returns decisions for a specific infohash
func GetDecisionsByInfohash(w http.ResponseWriter, r *http.Request) {
	infohash := chi.URLParam(r, "infohash")
	if infohash == "" {
		respondError(w, http.StatusBadRequest, "Infohash required")
		return
	}

	if decisionStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"decisions": []DecisionResponse{},
			"summary":   nil,
		})
		return
	}

	decisions, err := decisionStorage.GetByInfohash(infohash)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get decisions")
		return
	}

	response := make([]DecisionResponse, 0, len(decisions))
	acceptCount := 0
	rejectCount := 0
	var hasLegalReject bool

	for _, d := range decisions {
		reasonCodes := make([]string, len(d.ReasonCodes))
		for i, rc := range d.ReasonCodes {
			reasonCodes[i] = string(rc)
		}

		response = append(response, DecisionResponse{
			DecisionID:     d.DecisionID,
			Decision:       string(d.Decision),
			ReasonCodes:    reasonCodes,
			TargetEventID:  d.TargetEventID,
			TargetInfohash: d.TargetInfohash,
			CuratorPubkey:  d.CuratorPubkey,
			RulesetType:    d.RulesetType,
			RulesetVersion: d.RulesetVersion,
			CreatedAt:      d.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})

		if d.Decision == decision.DecisionAccept {
			acceptCount++
		} else {
			rejectCount++
			if d.HasLegalCode() {
				hasLegalReject = true
			}
		}
	}

	// Compute final decision based on legal priority
	finalDecision := "pending"
	if hasLegalReject {
		finalDecision = "reject"
	} else if acceptCount > rejectCount {
		finalDecision = "accept"
	} else if rejectCount > 0 {
		finalDecision = "reject"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"decisions": response,
		"summary": map[string]interface{}{
			"total_decisions":  len(decisions),
			"accept_count":     acceptCount,
			"reject_count":     rejectCount,
			"has_legal_reject": hasLegalReject,
			"final_decision":   finalDecision,
		},
	})
}

// GetDecisionStats returns overall decision statistics
func GetDecisionStats(w http.ResponseWriter, r *http.Request) {
	if decisionStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"total_decisions": 0,
			"accept_count":    0,
			"reject_count":    0,
			"unique_curators": 0,
		})
		return
	}

	stats, err := decisionStorage.GetStats()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetReasonCodes returns all available reason codes with descriptions
func GetReasonCodes(w http.ResponseWriter, r *http.Request) {
	codes := []map[string]interface{}{
		{
			"code":          string(ruleset.ReasonLegalDMCA),
			"category":      "legal",
			"deterministic": true,
			"description":   ruleset.ReasonLegalDMCA.Description(),
		},
		{
			"code":          string(ruleset.ReasonLegalIllegal),
			"category":      "legal",
			"deterministic": true,
			"description":   ruleset.ReasonLegalIllegal.Description(),
		},
		{
			"code":          string(ruleset.ReasonAbuseSpam),
			"category":      "abuse",
			"deterministic": true,
			"description":   ruleset.ReasonAbuseSpam.Description(),
		},
		{
			"code":          string(ruleset.ReasonAbuseMalware),
			"category":      "abuse",
			"deterministic": true,
			"description":   ruleset.ReasonAbuseMalware.Description(),
		},
		{
			"code":          string(ruleset.ReasonSemDuplicateExact),
			"category":      "semantic",
			"deterministic": true,
			"description":   ruleset.ReasonSemDuplicateExact.Description(),
		},
		{
			"code":          string(ruleset.ReasonSemDuplicateProbable),
			"category":      "semantic",
			"deterministic": false,
			"description":   ruleset.ReasonSemDuplicateProbable.Description(),
		},
		{
			"code":          string(ruleset.ReasonSemBadMeta),
			"category":      "semantic",
			"deterministic": false,
			"description":   ruleset.ReasonSemBadMeta.Description(),
		},
		{
			"code":          string(ruleset.ReasonSemLowQuality),
			"category":      "semantic",
			"deterministic": false,
			"description":   ruleset.ReasonSemLowQuality.Description(),
		},
		{
			"code":          string(ruleset.ReasonSemCategoryMismatch),
			"category":      "semantic",
			"deterministic": false,
			"description":   ruleset.ReasonSemCategoryMismatch.Description(),
		},
	}

	respondJSON(w, http.StatusOK, codes)
}
