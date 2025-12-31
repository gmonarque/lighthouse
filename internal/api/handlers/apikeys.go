package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gmonarque/lighthouse/internal/api/apikeys"
	"github.com/gmonarque/lighthouse/internal/api/middleware"
)

// GetAPIKeys returns all API keys (without sensitive data)
func GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	storage := middleware.GetAPIKeyStorage()

	keys, err := storage.List()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list API keys")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys":  keys,
		"total": len(keys),
	})
}

// CreateAPIKey creates a new API key
func CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		RateLimit   int      `json:"rate_limit"`
		ExpiresIn   int      `json:"expires_in"` // Days until expiry, 0 = never
		Notes       string   `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "Name is required")
		return
	}

	// Convert string permissions to Permission type
	perms := make([]apikeys.Permission, 0, len(req.Permissions))
	for _, p := range req.Permissions {
		perms = append(perms, apikeys.Permission(p))
	}

	// Default to read permission if none specified
	if len(perms) == 0 {
		perms = []apikeys.Permission{apikeys.PermissionRead}
	}

	// Create the key
	key, plaintextKey := apikeys.NewAPIKey(req.Name, perms, "")

	// Set optional fields
	key.RateLimit = req.RateLimit
	key.Notes = req.Notes

	if req.ExpiresIn > 0 {
		expires := time.Now().AddDate(0, 0, req.ExpiresIn)
		key.ExpiresAt = &expires
	}

	// Save the key
	storage := middleware.GetAPIKeyStorage()
	if err := storage.Save(key); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save API key")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         key.ID,
		"name":       key.Name,
		"key":        plaintextKey, // Only returned on creation
		"key_prefix": key.KeyPrefix,
		"message":    "API key created. Save the key now - it won't be shown again.",
	})
}

// GetAPIKeyByID returns a single API key
func GetAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	storage := middleware.GetAPIKeyStorage()
	key, err := storage.GetByID(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get API key")
		return
	}

	if key == nil {
		respondError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Clear sensitive data
	key.KeyHash = ""

	respondJSON(w, http.StatusOK, key)
}

// UpdateAPIKey updates an API key
func UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		RateLimit   int      `json:"rate_limit"`
		Notes       string   `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	storage := middleware.GetAPIKeyStorage()
	key, err := storage.GetByID(id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get API key")
		return
	}

	if key == nil {
		respondError(w, http.StatusNotFound, "API key not found")
		return
	}

	// Update fields
	if req.Name != "" {
		key.Name = req.Name
	}
	if len(req.Permissions) > 0 {
		perms := make([]apikeys.Permission, 0, len(req.Permissions))
		for _, p := range req.Permissions {
			perms = append(perms, apikeys.Permission(p))
		}
		key.Permissions = perms
	}
	key.RateLimit = req.RateLimit
	key.Notes = req.Notes

	if err := storage.Save(key); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "API key updated",
	})
}

// DeleteAPIKey deletes an API key
func DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	storage := middleware.GetAPIKeyStorage()
	if err := storage.Delete(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "API key deleted",
	})
}

// EnableAPIKey enables an API key
func EnableAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	storage := middleware.GetAPIKeyStorage()
	if err := storage.Enable(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to enable API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "API key enabled",
	})
}

// DisableAPIKey disables an API key
func DisableAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "ID is required")
		return
	}

	storage := middleware.GetAPIKeyStorage()
	if err := storage.Disable(id); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to disable API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "API key disabled",
	})
}

// GetAvailablePermissions returns the list of available permissions
func GetAvailablePermissions(w http.ResponseWriter, r *http.Request) {
	permissions := []map[string]interface{}{
		{"id": "read", "name": "Read", "description": "View torrents and search"},
		{"id": "write", "name": "Write", "description": "Add and modify torrents"},
		{"id": "admin", "name": "Admin", "description": "Full administrative access"},
		{"id": "torznab", "name": "Torznab", "description": "Access Torznab API"},
		{"id": "curator", "name": "Curator", "description": "Make curation decisions"},
		{"id": "moderation", "name": "Moderation", "description": "Handle reports and appeals"},
		{"id": "settings", "name": "Settings", "description": "Modify settings"},
		{"id": "relays", "name": "Relays", "description": "Manage relay connections"},
	}

	respondJSON(w, http.StatusOK, permissions)
}
