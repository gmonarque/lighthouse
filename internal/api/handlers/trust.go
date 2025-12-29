package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/lighthouse-client/lighthouse/internal/database"
	"github.com/lighthouse-client/lighthouse/internal/nostr"
)

// GetWhitelist returns all whitelisted npubs
func GetWhitelist(w http.ResponseWriter, r *http.Request) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT id, npub, alias, notes, added_at
		FROM trust_whitelist
		ORDER BY added_at DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get whitelist")
		return
	}
	defer rows.Close()

	entries := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var npub, addedAt string
		var alias, notes sql.NullString

		if err := rows.Scan(&id, &npub, &alias, &notes, &addedAt); err != nil {
			continue
		}

		entries = append(entries, map[string]interface{}{
			"id":       id,
			"npub":     npub,
			"alias":    alias.String,
			"notes":    notes.String,
			"added_at": addedAt,
		})
	}

	respondJSON(w, http.StatusOK, entries)
}

// AddToWhitelist adds an npub to the whitelist
func AddToWhitelist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Npub  string `json:"npub"`
		Alias string `json:"alias"`
		Notes string `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Npub == "" {
		respondError(w, http.StatusBadRequest, "npub is required")
		return
	}

	db := database.Get()
	result, err := db.Exec(`
		INSERT INTO trust_whitelist (npub, alias, notes)
		VALUES (?, ?, ?)
		ON CONFLICT(npub) DO UPDATE SET
			alias = excluded.alias,
			notes = excluded.notes
	`, req.Npub, req.Alias, req.Notes)

	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add to whitelist")
		return
	}

	id, _ := result.LastInsertId()
	database.LogActivity("whitelist_add", req.Npub)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":    id,
		"npub":  req.Npub,
		"alias": req.Alias,
	})
}

// RemoveFromWhitelist removes an npub from the whitelist and deletes their torrents
func RemoveFromWhitelist(w http.ResponseWriter, r *http.Request) {
	npub := chi.URLParam(r, "npub")

	db := database.Get()

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Check if npub exists in whitelist
	var exists int
	err = tx.QueryRow("SELECT COUNT(*) FROM trust_whitelist WHERE npub = ?", npub).Scan(&exists)
	if err != nil || exists == 0 {
		respondError(w, http.StatusNotFound, "npub not found in whitelist")
		return
	}

	// Convert npub to hex for torrent_uploads queries (uploads are stored as hex pubkey)
	hexPubkey, err := nostr.NpubToHex(npub)
	if err != nil {
		// If conversion fails, try using as-is (might already be hex)
		hexPubkey = npub
	}

	// Delete torrents that only have this uploader
	_, err = tx.Exec(`
		DELETE FROM torrents
		WHERE id IN (
			SELECT torrent_id FROM torrent_uploads
			WHERE uploader_npub = ?
			GROUP BY torrent_id
			HAVING COUNT(DISTINCT uploader_npub) = 1
		)
	`, hexPubkey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete torrents")
		return
	}

	// Remove upload records for this user
	_, err = tx.Exec("DELETE FROM torrent_uploads WHERE uploader_npub = ?", hexPubkey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete upload records")
		return
	}

	// Update upload counts for remaining torrents
	tx.Exec(`
		UPDATE torrents SET
			upload_count = (SELECT COUNT(*) FROM torrent_uploads WHERE torrent_id = torrents.id)
		WHERE id IN (SELECT DISTINCT torrent_id FROM torrent_uploads)
	`)

	// Remove from whitelist
	_, err = tx.Exec("DELETE FROM trust_whitelist WHERE npub = ?", npub)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove from whitelist")
		return
	}

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	database.LogActivity("whitelist_remove", npub)
	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// GetBlacklist returns all blacklisted npubs
func GetBlacklist(w http.ResponseWriter, r *http.Request) {
	db := database.Get()
	rows, err := db.Query(`
		SELECT id, npub, reason, added_at
		FROM trust_blacklist
		ORDER BY added_at DESC
	`)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get blacklist")
		return
	}
	defer rows.Close()

	entries := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id int64
		var npub, addedAt string
		var reason sql.NullString

		if err := rows.Scan(&id, &npub, &reason, &addedAt); err != nil {
			continue
		}

		entries = append(entries, map[string]interface{}{
			"id":       id,
			"npub":     npub,
			"reason":   reason.String,
			"added_at": addedAt,
		})
	}

	respondJSON(w, http.StatusOK, entries)
}

// AddToBlacklist adds an npub to the blacklist and removes their content
func AddToBlacklist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Npub   string `json:"npub"`
		Reason string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Npub == "" {
		respondError(w, http.StatusBadRequest, "npub is required")
		return
	}

	db := database.Get()

	// Convert npub to hex for torrent_uploads queries (uploads are stored as hex pubkey)
	hexPubkey, err := nostr.NpubToHex(req.Npub)
	if err != nil {
		// If conversion fails, try using as-is (might already be hex)
		hexPubkey = req.Npub
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	// Add to blacklist
	_, err = tx.Exec(`
		INSERT INTO trust_blacklist (npub, reason)
		VALUES (?, ?)
		ON CONFLICT(npub) DO UPDATE SET reason = excluded.reason
	`, req.Npub, req.Reason)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to add to blacklist")
		return
	}

	// Remove from whitelist if present
	tx.Exec("DELETE FROM trust_whitelist WHERE npub = ?", req.Npub)

	// Delete all torrents from this uploader
	// First, get torrent IDs that only have this uploader
	_, err = tx.Exec(`
		DELETE FROM torrents
		WHERE id IN (
			SELECT torrent_id FROM torrent_uploads
			WHERE uploader_npub = ?
			GROUP BY torrent_id
			HAVING COUNT(DISTINCT uploader_npub) = 1
		)
	`, hexPubkey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete torrents")
		return
	}

	// Remove upload records
	_, err = tx.Exec("DELETE FROM torrent_uploads WHERE uploader_npub = ?", hexPubkey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete upload records")
		return
	}

	// Update upload counts for remaining torrents
	_, err = tx.Exec(`
		UPDATE torrents SET
			upload_count = (SELECT COUNT(*) FROM torrent_uploads WHERE torrent_id = torrents.id),
			trust_score = trust_score - 10
		WHERE id IN (SELECT DISTINCT torrent_id FROM torrent_uploads)
	`)

	if err := tx.Commit(); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	database.LogActivity("blacklist_add", req.Npub)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"npub":   req.Npub,
		"reason": req.Reason,
		"status": "blocked",
	})
}

// RemoveFromBlacklist removes an npub from the blacklist
func RemoveFromBlacklist(w http.ResponseWriter, r *http.Request) {
	npub := chi.URLParam(r, "npub")

	db := database.Get()
	result, err := db.Exec("DELETE FROM trust_blacklist WHERE npub = ?", npub)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to remove from blacklist")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		respondError(w, http.StatusNotFound, "npub not found in blacklist")
		return
	}

	database.LogActivity("blacklist_remove", npub)
	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// GetTrustSettings returns trust configuration
func GetTrustSettings(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"depth": cfg.Trust.Depth,
	})
}

// UpdateTrustSettings updates trust configuration
func UpdateTrustSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Depth int `json:"depth"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Depth < 0 || req.Depth > 2 {
		respondError(w, http.StatusBadRequest, "Depth must be 0, 1, or 2")
		return
	}

	if err := config.Update("trust.depth", req.Depth); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"depth": req.Depth,
	})
}
