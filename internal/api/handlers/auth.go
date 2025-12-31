package handlers

import (
	"net/http"
	"strings"

	"github.com/gmonarque/lighthouse/internal/config"
)

// GetAPIKey returns the API key to the frontend for authentication
// This endpoint is only accessible from the same origin (browser requests)
func GetAPIKey(w http.ResponseWriter, r *http.Request) {
	// Security: Only allow requests that appear to be from the browser frontend
	// Check for Referer header or Sec-Fetch-Site header
	secFetchSite := r.Header.Get("Sec-Fetch-Site")
	referer := r.Header.Get("Referer")
	origin := r.Header.Get("Origin")

	// Allow same-origin requests or requests without these headers (for localhost dev)
	isSameOrigin := secFetchSite == "same-origin" ||
		secFetchSite == "" ||
		strings.Contains(referer, r.Host) ||
		strings.Contains(origin, r.Host) ||
		origin == ""

	// Also check if it's a local request
	remoteAddr := r.RemoteAddr
	isLocal := strings.HasPrefix(remoteAddr, "127.0.0.1") ||
		strings.HasPrefix(remoteAddr, "[::1]") ||
		strings.HasPrefix(remoteAddr, "localhost")

	if !isSameOrigin && !isLocal {
		respondError(w, http.StatusForbidden, "API key can only be retrieved from the web interface")
		return
	}

	cfg := config.Get()
	respondJSON(w, http.StatusOK, map[string]string{
		"api_key": cfg.Server.APIKey,
	})
}
