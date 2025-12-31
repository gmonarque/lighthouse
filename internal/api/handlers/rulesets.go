package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gmonarque/lighthouse/internal/ruleset"
	"github.com/go-chi/chi/v5"
)

// RulesetResponse represents a ruleset in API responses
type RulesetResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Version     string `json:"version"`
	Hash        string `json:"hash"`
	Description string `json:"description,omitempty"`
	RuleCount   int    `json:"rule_count,omitempty"`
	IsActive    bool   `json:"is_active"`
	Source      string `json:"source,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// rulesetStorage is the storage instance (set during init)
var rulesetStorage *ruleset.Storage

// SetRulesetStorage sets the ruleset storage instance
func SetRulesetStorage(s *ruleset.Storage) {
	rulesetStorage = s
}

// GetRulesets returns all rulesets
func GetRulesets(w http.ResponseWriter, r *http.Request) {
	if rulesetStorage == nil {
		respondJSON(w, http.StatusOK, []RulesetResponse{})
		return
	}

	descriptors, err := rulesetStorage.List()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list rulesets")
		return
	}

	response := make([]RulesetResponse, 0, len(descriptors))
	for _, d := range descriptors {
		response = append(response, RulesetResponse{
			ID:        d.RulesetID,
			Type:      string(d.Type),
			Version:   d.Version,
			Hash:      d.Hash,
			IsActive:  d.IsActive,
			Source:    d.Source,
			CreatedAt: d.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	respondJSON(w, http.StatusOK, response)
}

// GetRuleset returns a specific ruleset by ID
func GetRuleset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Ruleset ID required")
		return
	}

	if rulesetStorage == nil {
		respondError(w, http.StatusNotFound, "Ruleset not found")
		return
	}

	rs, err := rulesetStorage.GetByID(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get ruleset")
		return
	}
	if rs == nil {
		respondError(w, http.StatusNotFound, "Ruleset not found")
		return
	}

	respondJSON(w, http.StatusOK, rs)
}

// ImportRulesetRequest is the request body for importing a ruleset
type ImportRulesetRequest struct {
	Content string `json:"content"` // JSON ruleset content
	Source  string `json:"source"`  // Optional source URL
}

// ImportRuleset imports a new ruleset
func ImportRuleset(w http.ResponseWriter, r *http.Request) {
	var req ImportRulesetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "Ruleset content required")
		return
	}

	// Parse the ruleset
	var rs ruleset.Ruleset
	if err := json.Unmarshal([]byte(req.Content), &rs); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ruleset JSON: "+err.Error())
		return
	}

	// Validate the ruleset
	if err := rs.Validate(); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid ruleset: "+err.Error())
		return
	}

	// Save to storage
	if rulesetStorage != nil {
		if err := rulesetStorage.Save(&rs); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to save ruleset")
			return
		}
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      rs.ID,
		"version": rs.Version,
		"hash":    rs.ComputeHash(),
		"message": "Ruleset imported successfully",
	})
}

// ActivateRuleset activates a specific ruleset version
func ActivateRuleset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Ruleset ID required")
		return
	}

	if rulesetStorage == nil {
		respondError(w, http.StatusNotFound, "Ruleset not found")
		return
	}

	if err := rulesetStorage.SetActive(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to activate ruleset: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Ruleset activated",
	})
}

// DeactivateRuleset deactivates a ruleset
func DeactivateRuleset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Ruleset ID required")
		return
	}

	if rulesetStorage == nil {
		respondError(w, http.StatusNotFound, "Ruleset not found")
		return
	}

	if err := rulesetStorage.Deprecate(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to deactivate ruleset: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Ruleset deactivated",
	})
}

// DeleteRuleset deletes a ruleset
func DeleteRuleset(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "Ruleset ID required")
		return
	}

	if rulesetStorage == nil {
		respondError(w, http.StatusNotFound, "Ruleset not found")
		return
	}

	if err := rulesetStorage.Delete(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete ruleset: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Ruleset deleted",
	})
}

// GetActiveRulesets returns only active rulesets
func GetActiveRulesets(w http.ResponseWriter, r *http.Request) {
	if rulesetStorage == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"censoring": nil,
			"semantic":  nil,
		})
		return
	}

	censoring, _ := rulesetStorage.GetActive(ruleset.RulesetTypeCensoring)
	semantic, _ := rulesetStorage.GetActive(ruleset.RulesetTypeSemantic)

	response := map[string]interface{}{}

	if censoring != nil {
		response["censoring"] = RulesetResponse{
			ID:        censoring.ID,
			Type:      string(censoring.Type),
			Version:   censoring.Version,
			Hash:      censoring.ComputeHash(),
			RuleCount: len(censoring.Rules),
			IsActive:  true,
		}
	}

	if semantic != nil {
		response["semantic"] = RulesetResponse{
			ID:        semantic.ID,
			Type:      string(semantic.Type),
			Version:   semantic.Version,
			Hash:      semantic.ComputeHash(),
			RuleCount: len(semantic.Rules),
			IsActive:  true,
		}
	}

	respondJSON(w, http.StatusOK, response)
}
