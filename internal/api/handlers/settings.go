package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/lighthouse-client/lighthouse/internal/config"
	"github.com/lighthouse-client/lighthouse/internal/database"
	"github.com/lighthouse-client/lighthouse/internal/nostr"
)

// GetSettings returns current application settings
func GetSettings(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()

	// Get API key (not masked - needed for Torznab integration)
	apiKey := cfg.Server.APIKey

	nsecMasked := ""
	if cfg.Nostr.Identity.Nsec != "" {
		nsecMasked = "***configured***"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"server": map[string]interface{}{
			"host":    cfg.Server.Host,
			"port":    cfg.Server.Port,
			"api_key": apiKey,
		},
		"nostr": map[string]interface{}{
			"identity": map[string]interface{}{
				"npub": cfg.Nostr.Identity.Npub,
				"nsec": nsecMasked,
			},
			"relays": cfg.Nostr.Relays,
		},
		"trust": map[string]interface{}{
			"depth": cfg.Trust.Depth,
		},
		"enrichment": map[string]interface{}{
			"enabled":      cfg.Enrichment.Enabled,
			"tmdb_api_key": cfg.Enrichment.TMDBAPIKey != "",
			"omdb_api_key": cfg.Enrichment.OMDBAPIKey != "",
		},
		"indexer": map[string]interface{}{
			"tag_filter":         cfg.Indexer.TagFilter,
			"tag_filter_enabled": cfg.Indexer.TagFilterEnabled,
		},
	})
}

// UpdateSettings updates application settings
func UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Update each setting
	for key, value := range req {
		if err := config.Update(key, value); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to update setting: "+key)
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// GenerateIdentity creates a new Nostr identity
func GenerateIdentity(w http.ResponseWriter, r *http.Request) {
	npub, nsec, err := nostr.GenerateIdentity()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate identity")
		return
	}

	// Save to config
	if err := config.Update("nostr.identity.npub", npub); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save npub")
		return
	}
	if err := config.Update("nostr.identity.nsec", nsec); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save nsec")
		return
	}

	// Also save to database
	db := database.Get()
	db.Exec(`
		INSERT INTO identities (npub, nsec, is_own)
		VALUES (?, ?, TRUE)
		ON CONFLICT(npub) DO UPDATE SET nsec = excluded.nsec, is_own = TRUE
	`, npub, nsec)

	database.LogActivity("identity_generated", npub)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"npub": npub,
		"nsec": nsec,
	})
}

// ImportIdentity imports an existing Nostr identity
func ImportIdentity(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nsec string `json:"nsec"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Nsec == "" {
		respondError(w, http.StatusBadRequest, "nsec is required")
		return
	}

	npub, err := nostr.NsecToNpub(req.Nsec)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid nsec")
		return
	}

	// Save to config
	if err := config.Update("nostr.identity.npub", npub); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save npub")
		return
	}
	if err := config.Update("nostr.identity.nsec", req.Nsec); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save nsec")
		return
	}

	// Also save to database
	db := database.Get()
	db.Exec(`
		INSERT INTO identities (npub, nsec, is_own)
		VALUES (?, ?, TRUE)
		ON CONFLICT(npub) DO UPDATE SET nsec = excluded.nsec, is_own = TRUE
	`, npub, req.Nsec)

	database.LogActivity("identity_imported", npub)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"npub": npub,
	})
}

// ExportConfig exports the configuration as JSON
func ExportConfig(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=lighthouse-config.json")

	json.NewEncoder(w).Encode(cfg)
}

// ImportConfig imports a configuration from JSON
func ImportConfig(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config

	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid configuration file")
		return
	}

	// Update each section
	if err := config.Update("server", cfg.Server); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to import server config")
		return
	}
	if err := config.Update("nostr", cfg.Nostr); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to import nostr config")
		return
	}
	if err := config.Update("trust", cfg.Trust); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to import trust config")
		return
	}
	if err := config.Update("enrichment", cfg.Enrichment); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to import enrichment config")
		return
	}
	if err := config.Update("indexer", cfg.Indexer); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to import indexer config")
		return
	}

	database.LogActivity("config_imported", "")

	respondJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

// GetSetupStatus returns setup wizard status
func GetSetupStatus(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()

	setupCompleted, _ := database.GetSetting("setup_completed")

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"completed":         setupCompleted == "true",
		"has_identity":      cfg.Nostr.Identity.Npub != "",
		"has_relays":        len(cfg.Nostr.Relays) > 0,
		"has_tmdb_key":      cfg.Enrichment.TMDBAPIKey != "",
		"enrichment_enabled": cfg.Enrichment.Enabled,
	})
}

// CompleteSetup marks the setup wizard as completed
func CompleteSetup(w http.ResponseWriter, r *http.Request) {
	if err := database.SetSetting("setup_completed", "true"); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to complete setup")
		return
	}

	database.LogActivity("setup_completed", "")

	respondJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
