package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*tokenBucket
	rate     int           // requests per window
	window   time.Duration // time window
	cleanup  time.Duration // cleanup interval for stale entries
}

type tokenBucket struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*tokenBucket),
		rate:     rate,
		window:   window,
		cleanup:  5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request from the given key should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.limiters[key]

	if !exists {
		rl.limiters[key] = &tokenBucket{
			tokens:    rl.rate - 1,
			lastReset: now,
		}
		return true
	}

	// Reset tokens if window has passed
	if now.Sub(bucket.lastReset) >= rl.window {
		bucket.tokens = rl.rate - 1
		bucket.lastReset = now
		return true
	}

	// Check if tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// cleanupLoop periodically removes stale entries
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-2 * rl.window)
		for key, bucket := range rl.limiters {
			if bucket.lastReset.Before(cutoff) {
				delete(rl.limiters, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Default rate limiters
var (
	// IPRateLimiter limits requests per IP address
	// 100 requests per minute per IP
	IPRateLimiter = NewRateLimiter(100, time.Minute)

	// APIKeyRateLimiter limits requests per API key
	// 1000 requests per minute per API key
	APIKeyRateLimiter = NewRateLimiter(1000, time.Minute)

	// TorznabRateLimiter limits Torznab API requests
	// 60 requests per minute (more restrictive for search operations)
	TorznabRateLimiter = NewRateLimiter(60, time.Minute)

	// RelayRateLimiter limits relay WebSocket messages
	// 30 events per minute per client
	RelayRateLimiter = NewRateLimiter(30, time.Minute)
)

// RateLimitByIP middleware limits requests by IP address
func RateLimitByIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)

		if !IPRateLimiter.Allow(ip) {
			log.Warn().Str("ip", ip).Msg("Rate limit exceeded by IP")
			http.Error(w, "Rate limit exceeded. Try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimitByAPIKey middleware limits requests by API key
func RateLimitByAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("apikey")
		}

		// If no API key, fall back to IP-based limiting
		if apiKey == "" {
			apiKey = "nokey:" + getClientIP(r)
		}

		if !APIKeyRateLimiter.Allow(apiKey) {
			log.Warn().Str("key", maskAPIKey(apiKey)).Msg("Rate limit exceeded by API key")
			http.Error(w, "Rate limit exceeded. Try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimitTorznab middleware for Torznab API (more restrictive)
func RateLimitTorznab(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.URL.Query().Get("apikey")
		if apiKey == "" {
			apiKey = getClientIP(r)
		}

		if !TorznabRateLimiter.Allow(apiKey) {
			log.Warn().Str("key", maskAPIKey(apiKey)).Msg("Torznab rate limit exceeded")
			http.Error(w, "Rate limit exceeded. Try again later.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the real client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Real-IP header first (set by RealIP middleware)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}

	// Fall back to remote address
	return r.RemoteAddr
}

// maskAPIKey masks an API key for logging (shows first 8 chars)
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "***"
}

// CheckRelayRateLimit checks rate limit for relay connections
// Returns true if allowed, false if rate limited
func CheckRelayRateLimit(clientID string) bool {
	return RelayRateLimiter.Allow(clientID)
}
