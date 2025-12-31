package middleware

import (
	"context"
	"net/http"

	"github.com/gmonarque/lighthouse/internal/api/apikeys"
	"github.com/gmonarque/lighthouse/internal/config"
)

// Context keys for storing API key info
type contextKey string

const (
	APIKeyContextKey contextKey = "apikey"
)

// apiKeyStorage is the global API key storage
var apiKeyStorage = apikeys.NewStorage()

// InitAPIKeyStorage initializes the API key storage
func InitAPIKeyStorage() error {
	return apiKeyStorage.Init()
}

// APIKeyAuth middleware validates API key authentication
func APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get()

		// Get API key from header or query param
		plaintextKey := r.Header.Get("X-API-Key")
		if plaintextKey == "" {
			plaintextKey = r.URL.Query().Get("apikey")
		}

		// Skip auth if no API key is configured and no multi-user keys exist (first run)
		if cfg.Server.APIKey == "" {
			hasKeys, _ := apiKeyStorage.HasKeys()
			if !hasKeys {
				next.ServeHTTP(w, r)
				return
			}
		}

		// No key provided
		if plaintextKey == "" {
			http.Error(w, "Unauthorized: API key required", http.StatusUnauthorized)
			return
		}

		// Try legacy single API key first (backwards compatibility)
		if cfg.Server.APIKey != "" && plaintextKey == cfg.Server.APIKey {
			// Legacy key has full admin access
			next.ServeHTTP(w, r)
			return
		}

		// Try multi-user API key validation
		key, err := apiKeyStorage.ValidateKey(plaintextKey)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if key == nil {
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			return
		}

		// Store key in context for permission checks
		ctx := context.WithValue(r.Context(), APIKeyContextKey, key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequirePermission middleware checks for specific permissions
func RequirePermission(permissions ...apikeys.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetAPIKeyFromContext(r.Context())

			// If no key in context, assume legacy auth (full access)
			if key == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check if key has required permission
			if !key.HasAnyPermission(permissions...) {
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetAPIKeyFromContext retrieves the API key from request context
func GetAPIKeyFromContext(ctx context.Context) *apikeys.APIKey {
	key, ok := ctx.Value(APIKeyContextKey).(*apikeys.APIKey)
	if !ok {
		return nil
	}
	return key
}

// OptionalAPIKeyAuth middleware that doesn't require auth but validates if provided
func OptionalAPIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := config.Get()

		// Get API key from header or query param
		plaintextKey := r.Header.Get("X-API-Key")
		if plaintextKey == "" {
			plaintextKey = r.URL.Query().Get("apikey")
		}

		// No key provided, continue without auth
		if plaintextKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Validate legacy key
		if cfg.Server.APIKey != "" && plaintextKey == cfg.Server.APIKey {
			next.ServeHTTP(w, r)
			return
		}

		// Validate multi-user key
		key, err := apiKeyStorage.ValidateKey(plaintextKey)
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if key == nil {
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			return
		}

		// Store key in context
		ctx := context.WithValue(r.Context(), APIKeyContextKey, key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAPIKeyStorage returns the API key storage instance
func GetAPIKeyStorage() *apikeys.Storage {
	return apiKeyStorage
}
