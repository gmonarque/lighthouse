package middleware

import (
	"net/http"

	"github.com/lighthouse-client/lighthouse/internal/config"
)

// APIKeyAuth middleware validates API key authentication
func APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get()

		// Get API key from header or query param
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("apikey")
		}

		// Skip auth if no API key is configured (first run)
		if cfg.Server.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate API key
		if apiKey != cfg.Server.APIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// OptionalAPIKeyAuth middleware that doesn't require auth but validates if provided
func OptionalAPIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get()

		// Get API key from header or query param
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("apikey")
		}

		// If API key is provided, validate it
		if apiKey != "" && cfg.Server.APIKey != "" && apiKey != cfg.Server.APIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
