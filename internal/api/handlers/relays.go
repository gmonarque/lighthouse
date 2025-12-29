package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/lighthouse-client/lighthouse/internal/database"
)

// GetRelays returns all configured relays
func GetRelays(w http.ResponseWriter, r *http.Request) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT id, url, name, preset, enabled, status, last_connected_at, created_at
		FROM relays
		ORDER BY created_at ASC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get relays")
		return
	}
	defer rows.Close()

	relays := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var url, status, createdAt string
		var name, preset sql.NullString
		var enabled bool
		var lastConnectedAt sql.NullString

		if err := rows.Scan(&id, &url, &name, &preset, &enabled, &status, &lastConnectedAt, &createdAt); err != nil {
			continue
		}

		relays = append(relays, map[string]interface{}{
			"id":                id,
			"url":               url,
			"name":              name.String,
			"preset":            preset.String,
			"enabled":           enabled,
			"status":            status,
			"last_connected_at": lastConnectedAt.String,
			"created_at":        createdAt,
		})
	}

	respondJSON(w, http.StatusOK, relays)
}

// AddRelay adds a new relay
func AddRelay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Name    string `json:"name"`
		Preset  string `json:"preset"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "URL is required")
		return
	}

	db := database.Get()
	result, err := db.Exec(`
		INSERT INTO relays (url, name, preset, enabled, status)
		VALUES (?, ?, ?, ?, 'disconnected')
	`, req.URL, req.Name, req.Preset, req.Enabled)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add relay")
		return
	}

	id, _ := result.LastInsertId()
	database.LogActivity("relay_added", req.URL)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"url":     req.URL,
		"name":    req.Name,
		"preset":  req.Preset,
		"enabled": req.Enabled,
		"status":  "disconnected",
	})
}

// UpdateRelay updates a relay configuration
func UpdateRelay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid relay ID")
		return
	}

	var req struct {
		URL     string `json:"url"`
		Name    string `json:"name"`
		Preset  string `json:"preset"`
		Enabled *bool  `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db := database.Get()

	// Build update query dynamically
	query := "UPDATE relays SET "
	args := []interface{}{}
	updates := []string{}

	if req.URL != "" {
		updates = append(updates, "url = ?")
		args = append(args, req.URL)
	}
	if req.Name != "" {
		updates = append(updates, "name = ?")
		args = append(args, req.Name)
	}
	if req.Preset != "" {
		updates = append(updates, "preset = ?")
		args = append(args, req.Preset)
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, *req.Enabled)
	}

	if len(updates) == 0 {
		respondError(w, http.StatusBadRequest, "No fields to update")
		return
	}

	for i, u := range updates {
		if i > 0 {
			query += ", "
		}
		query += u
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := db.Exec(query, args...)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update relay")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(w, http.StatusNotFound, "Relay not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DeleteRelay removes a relay
func DeleteRelay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid relay ID")
		return
	}

	db := database.Get()
	result, err := db.Exec("DELETE FROM relays WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete relay")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(w, http.StatusNotFound, "Relay not found")
		return
	}

	database.LogActivity("relay_deleted", idStr)
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ConnectRelay manually connects to a relay
func ConnectRelay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid relay ID")
		return
	}

	// TODO: Implement actual relay connection via relay manager
	db := database.Get()
	_, err = db.Exec("UPDATE relays SET status = 'connecting' WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update relay status")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "connecting"})
}

// DisconnectRelay manually disconnects from a relay
func DisconnectRelay(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid relay ID")
		return
	}

	// TODO: Implement actual relay disconnection via relay manager
	db := database.Get()
	_, err = db.Exec("UPDATE relays SET status = 'disconnected' WHERE id = ?", id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update relay status")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}
