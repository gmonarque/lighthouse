package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gmonarque/lighthouse/internal/trust"
	"github.com/go-chi/chi/v5"
)

// CuratorResponse represents a curator in API responses
type CuratorResponse struct {
	Pubkey          string   `json:"pubkey"`
	Alias           string   `json:"alias,omitempty"`
	Weight          int      `json:"weight"`
	Status          string   `json:"status"`
	ApprovedAt      string   `json:"approved_at,omitempty"`
	RevokedAt       string   `json:"revoked_at,omitempty"`
	RevokeReason    string   `json:"revoke_reason,omitempty"`
	TrustedRulesets []string `json:"trusted_rulesets,omitempty"`
}

// trustPolicyStorage is the trust policy storage instance
var trustPolicyStorage *trust.PolicyStorage

// SetTrustPolicyStorage sets the trust policy storage instance
func SetTrustPolicyStorage(s *trust.PolicyStorage) {
	trustPolicyStorage = s
}

// GetCurators returns all trusted curators
func GetCurators(w http.ResponseWriter, r *http.Request) {
	if trustPolicyStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"curators": []CuratorResponse{},
			"total":    0,
		})
		return
	}

	curators, err := trustPolicyStorage.ListCurators()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list curators")
		return
	}

	response := make([]CuratorResponse, 0, len(curators))
	for _, c := range curators {
		cr := CuratorResponse{
			Pubkey:          c.Pubkey,
			Alias:           c.Alias,
			Weight:          c.Weight,
			Status:          c.Status,
			TrustedRulesets: c.ApprovedRulesets,
		}
		if c.ApprovedAt != nil && !c.ApprovedAt.IsZero() {
			cr.ApprovedAt = c.ApprovedAt.Format("2006-01-02T15:04:05Z")
		}
		if c.RevokedAt != nil && !c.RevokedAt.IsZero() {
			cr.RevokedAt = c.RevokedAt.Format("2006-01-02T15:04:05Z")
		}
		cr.RevokeReason = c.RevokeReason
		response = append(response, cr)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"curators": response,
		"total":    len(response),
	})
}

// AddCuratorRequest is the request body for adding a curator
type AddCuratorRequest struct {
	Pubkey   string   `json:"pubkey"`
	Alias    string   `json:"alias,omitempty"`
	Weight   int      `json:"weight,omitempty"`
	Rulesets []string `json:"rulesets,omitempty"`
}

// AddCurator adds a new trusted curator
func AddCurator(w http.ResponseWriter, r *http.Request) {
	var req AddCuratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Pubkey == "" {
		respondError(w, http.StatusBadRequest, "Pubkey required")
		return
	}

	if req.Weight == 0 {
		req.Weight = 1
	}

	if trustPolicyStorage == nil {
		respondError(w, http.StatusInternalServerError, "Trust storage not available")
		return
	}

	// Get current policy or create new one
	policy, err := trustPolicyStorage.GetCurrent()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get policy")
		return
	}

	if policy == nil {
		// Create a new policy
		policy = trust.NewTrustPolicy("")
	}

	// Add curator to allowlist
	curator := trust.CuratorEntry{
		Pubkey:  req.Pubkey,
		Alias:   req.Alias,
		Weight:  req.Weight,
		AddedAt: time.Now().UTC(),
	}
	policy.Allowlist = append(policy.Allowlist, curator)

	// Save policy
	if err := trustPolicyStorage.Save(policy); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save policy")
		return
	}

	if err := trustPolicyStorage.SetCurrent(policy.PolicyID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to activate policy")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"message": "Curator added successfully",
		"pubkey":  req.Pubkey,
	})
}

// UpdateCuratorRequest is the request body for updating a curator
type UpdateCuratorRequest struct {
	Alias  string `json:"alias,omitempty"`
	Weight int    `json:"weight,omitempty"`
}

// UpdateCurator updates a curator's info
func UpdateCurator(w http.ResponseWriter, r *http.Request) {
	pubkey := chi.URLParam(r, "pubkey")
	if pubkey == "" {
		respondError(w, http.StatusBadRequest, "Pubkey required")
		return
	}

	var req UpdateCuratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if trustPolicyStorage == nil {
		respondError(w, http.StatusInternalServerError, "Trust storage not available")
		return
	}

	// Get current policy
	policy, err := trustPolicyStorage.GetCurrent()
	if err != nil || policy == nil {
		respondError(w, http.StatusNotFound, "No active policy found")
		return
	}

	// Find and update curator
	found := false
	for i, c := range policy.Allowlist {
		if c.Pubkey == pubkey {
			if req.Alias != "" {
				policy.Allowlist[i].Alias = req.Alias
			}
			if req.Weight > 0 {
				policy.Allowlist[i].Weight = req.Weight
			}
			found = true
			break
		}
	}

	if !found {
		respondError(w, http.StatusNotFound, "Curator not found")
		return
	}

	// Save updated policy
	if err := trustPolicyStorage.Save(policy); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update policy")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Curator updated",
	})
}

// RevokeCuratorRequest is the request body for revoking a curator
type RevokeCuratorRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RevokeCurator revokes a curator's trust
func RevokeCurator(w http.ResponseWriter, r *http.Request) {
	pubkey := chi.URLParam(r, "pubkey")
	if pubkey == "" {
		respondError(w, http.StatusBadRequest, "Pubkey required")
		return
	}

	var req RevokeCuratorRequest
	json.NewDecoder(r.Body).Decode(&req) // Optional body

	if trustPolicyStorage == nil {
		respondError(w, http.StatusInternalServerError, "Trust storage not available")
		return
	}

	// Get current policy
	policy, err := trustPolicyStorage.GetCurrent()
	if err != nil || policy == nil {
		respondError(w, http.StatusNotFound, "No active policy found")
		return
	}

	// Remove from allowlist and add to revoked
	newAllowlist := make([]trust.CuratorEntry, 0)
	var revokedCurator *trust.CuratorEntry
	for _, c := range policy.Allowlist {
		if c.Pubkey == pubkey {
			revokedCurator = &c
		} else {
			newAllowlist = append(newAllowlist, c)
		}
	}

	if revokedCurator == nil {
		respondError(w, http.StatusNotFound, "Curator not found in allowlist")
		return
	}

	policy.Allowlist = newAllowlist
	policy.Revoked = append(policy.Revoked, trust.RevokedKey{
		Pubkey:    pubkey,
		Reason:    req.Reason,
		RevokedAt: time.Now().UTC(),
	})

	// Save updated policy
	if err := trustPolicyStorage.Save(policy); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update policy")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Curator revoked",
	})
}

// GetTrustPolicy returns the current trust policy
func GetTrustPolicy(w http.ResponseWriter, r *http.Request) {
	if trustPolicyStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"policy": nil,
		})
		return
	}

	policy, err := trustPolicyStorage.GetCurrent()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get policy")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"policy": policy,
	})
}

// AggregationPolicyResponse represents aggregation policy in API responses
type AggregationPolicyResponse struct {
	Mode            string `json:"mode"`
	QuorumRequired  int    `json:"quorum_required"`
	WeightThreshold int    `json:"weight_threshold"`
}

// aggregationPolicy holds the current aggregation policy (in-memory for now)
var currentAggregationPolicy = &trust.AggregationPolicy{
	Mode:           trust.AggregationModeQuorum,
	QuorumRequired: 1,
}

// GetAggregationPolicy returns the current aggregation policy
func GetAggregationPolicy(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, AggregationPolicyResponse{
		Mode:            string(currentAggregationPolicy.Mode),
		QuorumRequired:  currentAggregationPolicy.QuorumRequired,
		WeightThreshold: currentAggregationPolicy.WeightThreshold,
	})
}

// UpdateAggregationPolicyRequest is the request body for updating aggregation policy
type UpdateAggregationPolicyRequest struct {
	Mode            string `json:"mode"` // "quorum", "any", "all", "weighted"
	QuorumRequired  int    `json:"quorum_required,omitempty"`
	WeightThreshold int    `json:"weight_threshold,omitempty"`
}

// UpdateAggregationPolicy updates the aggregation policy
func UpdateAggregationPolicy(w http.ResponseWriter, r *http.Request) {
	var req UpdateAggregationPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Mode == "" {
		respondError(w, http.StatusBadRequest, "Mode required")
		return
	}

	// Validate mode
	switch req.Mode {
	case "quorum", "any", "all", "weighted":
		// Valid
	default:
		respondError(w, http.StatusBadRequest, "Invalid mode. Use: quorum, any, all, weighted")
		return
	}

	currentAggregationPolicy = &trust.AggregationPolicy{
		Mode:            trust.AggregationMode(req.Mode),
		QuorumRequired:  req.QuorumRequired,
		WeightThreshold: req.WeightThreshold,
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Aggregation policy updated",
	})
}
