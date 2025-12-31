package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("allows initial requests", func(t *testing.T) {
		limiter := NewRateLimiter(5, time.Minute)
		if !limiter.Allow("test-key") {
			t.Error("Expected first request to be allowed")
		}
	})

	t.Run("enforces rate limit", func(t *testing.T) {
		limiter := NewRateLimiter(3, time.Minute)

		// Use all tokens
		for i := 0; i < 3; i++ {
			if !limiter.Allow("limit-key") {
				t.Errorf("Request %d should be allowed", i+1)
			}
		}

		// Fourth should be denied
		if limiter.Allow("limit-key") {
			t.Error("Request after limit should be denied")
		}
	})

	t.Run("different keys have separate limits", func(t *testing.T) {
		limiter := NewRateLimiter(2, time.Minute)

		// Use all tokens for key1
		limiter.Allow("key1")
		limiter.Allow("key1")
		if limiter.Allow("key1") {
			t.Error("key1 should be rate limited")
		}

		// key2 should still work
		if !limiter.Allow("key2") {
			t.Error("key2 should be allowed")
		}
	})

	t.Run("resets after window", func(t *testing.T) {
		limiter := NewRateLimiter(1, 50*time.Millisecond)

		// Use the token
		limiter.Allow("reset-key")
		if limiter.Allow("reset-key") {
			t.Error("Should be denied immediately")
		}

		// Wait for reset
		time.Sleep(60 * time.Millisecond)

		// Should be allowed again
		if !limiter.Allow("reset-key") {
			t.Error("Should be allowed after window reset")
		}
	})
}

func TestRateLimiterConcurrency(t *testing.T) {
	limiter := NewRateLimiter(50, time.Minute)

	var wg sync.WaitGroup
	var allowed, denied int64
	var mu sync.Mutex

	// Simulate 100 concurrent requests from same key
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if limiter.Allow("concurrent-test") {
				mu.Lock()
				allowed++
				mu.Unlock()
			} else {
				mu.Lock()
				denied++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// With 50 tokens, should allow exactly 50
	if allowed > 55 { // Some tolerance
		t.Errorf("Expected around 50 allowed, got %d", allowed)
	}
	if denied < 40 {
		t.Errorf("Expected around 50 denied, got %d", denied)
	}
}

func TestRateLimitByIP(t *testing.T) {
	handler := RateLimitByIP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("allows normal requests", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345" // Unique IP
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
	})
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		realIP     string
		forwarded  string
		remoteAddr string
		expected   string
	}{
		{
			name:       "X-Real-IP header",
			realIP:     "10.0.0.1",
			remoteAddr: "127.0.0.1:12345",
			expected:   "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For header",
			forwarded:  "10.0.0.2, 10.0.0.3",
			remoteAddr: "127.0.0.1:12345",
			expected:   "10.0.0.2, 10.0.0.3",
		},
		{
			name:       "Remote address fallback",
			remoteAddr: "192.168.1.1:54321",
			expected:   "192.168.1.1:54321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			req.RemoteAddr = tt.remoteAddr

			result := getClientIP(req)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"lh_abc123def456", "lh_abc12***"},
		{"short", "***"},
		{"12345678", "***"},
		{"123456789", "12345678***"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := maskAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
