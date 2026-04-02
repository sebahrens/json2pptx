package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultSecurityConfig(t *testing.T) {
	cfg := DefaultSecurityConfig()

	if cfg.Auth.Enabled {
		t.Error("Auth should be disabled by default")
	}
	if !contains(cfg.Auth.SkipPaths, "/healthz") {
		t.Error("SkipPaths should include /healthz")
	}
	if cfg.RateLimit.Enabled {
		t.Error("RateLimit should be disabled by default")
	}
	if cfg.RateLimit.RequestsPerWindow != 100 {
		t.Errorf("RequestsPerWindow = %d, want 100", cfg.RateLimit.RequestsPerWindow)
	}
	if cfg.RateLimit.WindowDuration != time.Minute {
		t.Errorf("WindowDuration = %v, want 1m", cfg.RateLimit.WindowDuration)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func TestAuthMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	t.Run("disabled authentication passes all requests", func(t *testing.T) {
		cfg := AuthConfig{Enabled: false}
		middleware := authMiddleware(cfg)(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("enabled authentication requires API key", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled: true,
			APIKeys: []string{"test-key-123"},
		}
		middleware := authMiddleware(cfg)(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid API key grants access", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled: true,
			APIKeys: []string{"test-key-123"},
		}
		middleware := authMiddleware(cfg)(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(APIKeyHeader, "test-key-123")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("invalid API key is rejected", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled: true,
			APIKeys: []string{"test-key-123"},
		}
		middleware := authMiddleware(cfg)(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(APIKeyHeader, "wrong-key")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("skip paths bypass authentication", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled:   true,
			APIKeys:   []string{"test-key-123"},
			SkipPaths: []string{"/healthz"},
		}
		middleware := authMiddleware(cfg)(handler)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("OPTIONS requests bypass authentication", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled: true,
			APIKeys: []string{"test-key-123"},
		}
		middleware := authMiddleware(cfg)(handler)

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("multiple API keys supported", func(t *testing.T) {
		cfg := AuthConfig{
			Enabled: true,
			APIKeys: []string{"key-1", "key-2", "key-3"},
		}
		middleware := authMiddleware(cfg)(handler)

		for _, key := range []string{"key-1", "key-2", "key-3"} {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set(APIKeyHeader, key)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("key %q: status = %d, want %d", key, rec.Code, http.StatusOK)
			}
		}
	})
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := securityHeadersMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Content-Security-Policy":   "default-src 'none'",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}

	for header, expected := range expectedHeaders {
		actual := rec.Header().Get(header)
		if actual != expected {
			t.Errorf("%s = %q, want %q", header, actual, expected)
		}
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allowed origin gets CORS headers", func(t *testing.T) {
		allowedOrigins := map[string]bool{"https://example.com": true}
		middleware := corsMiddleware(handler, allowedOrigins)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
			t.Error("expected CORS header for allowed origin")
		}
		if rec.Header().Get("Vary") != "Origin" {
			t.Error("expected Vary: Origin header")
		}
	})

	t.Run("disallowed origin gets no CORS headers", func(t *testing.T) {
		allowedOrigins := map[string]bool{"https://example.com": true}
		middleware := corsMiddleware(handler, allowedOrigins)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://malicious.com")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("expected no CORS header for disallowed origin")
		}
	})

	t.Run("empty allowed origins blocks all", func(t *testing.T) {
		allowedOrigins := map[string]bool{}
		middleware := corsMiddleware(handler, allowedOrigins)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Error("expected no CORS header when no origins allowed")
		}
	})

	t.Run("OPTIONS preflight returns 204", func(t *testing.T) {
		allowedOrigins := map[string]bool{"https://example.com": true}
		middleware := corsMiddleware(handler, allowedOrigins)

		req := httptest.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})
}

func TestRateLimiter(t *testing.T) {
	t.Run("allows requests under limit", func(t *testing.T) {
		limiter := NewRateLimiter(5, time.Minute)

		for i := 0; i < 5; i++ {
			allowed, remaining, _ := limiter.Allow("192.168.1.1")
			if !allowed {
				t.Errorf("request %d should be allowed", i+1)
			}
			expectedRemaining := 4 - i
			if remaining != expectedRemaining {
				t.Errorf("request %d: remaining = %d, want %d", i+1, remaining, expectedRemaining)
			}
		}
	})

	t.Run("blocks requests over limit", func(t *testing.T) {
		limiter := NewRateLimiter(3, time.Minute)

		// Exhaust the limit
		for i := 0; i < 3; i++ {
			limiter.Allow("192.168.1.1")
		}

		// Next request should be blocked
		allowed, remaining, _ := limiter.Allow("192.168.1.1")
		if allowed {
			t.Error("request should be blocked after limit exceeded")
		}
		if remaining != 0 {
			t.Errorf("remaining = %d, want 0", remaining)
		}
	})

	t.Run("different IPs have separate limits", func(t *testing.T) {
		limiter := NewRateLimiter(2, time.Minute)

		// Use up IP1's limit
		limiter.Allow("192.168.1.1")
		limiter.Allow("192.168.1.1")

		// IP2 should still have its full limit
		allowed, remaining, _ := limiter.Allow("192.168.1.2")
		if !allowed {
			t.Error("different IP should have separate limit")
		}
		if remaining != 1 {
			t.Errorf("remaining = %d, want 1", remaining)
		}
	})
}

func TestRateLimiterWithTrustedProxies(t *testing.T) {
	t.Run("valid CIDR notation", func(t *testing.T) {
		_, err := NewRateLimiterWithTrustedProxies(10, time.Minute, []string{"10.0.0.0/8"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("single IP address", func(t *testing.T) {
		_, err := NewRateLimiterWithTrustedProxies(10, time.Minute, []string{"192.168.1.1"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid CIDR returns error", func(t *testing.T) {
		_, err := NewRateLimiterWithTrustedProxies(10, time.Minute, []string{"invalid"})
		if err == nil {
			t.Error("expected error for invalid CIDR")
		}
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("sets rate limit headers", func(t *testing.T) {
		limiter := NewRateLimiter(10, time.Minute)
		middleware := RateLimitMiddleware(limiter)(handler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		if rec.Header().Get("X-RateLimit-Limit") != "10" {
			t.Error("expected X-RateLimit-Limit header")
		}
		if rec.Header().Get("X-RateLimit-Remaining") == "" {
			t.Error("expected X-RateLimit-Remaining header")
		}
		if rec.Header().Get("X-RateLimit-Reset") == "" {
			t.Error("expected X-RateLimit-Reset header")
		}
	})

	t.Run("returns 429 when rate limited", func(t *testing.T) {
		limiter := NewRateLimiter(1, time.Minute)
		middleware := RateLimitMiddleware(limiter)(handler)

		// First request succeeds
		req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req1.RemoteAddr = "192.168.1.1:12345"
		rec1 := httptest.NewRecorder()
		middleware.ServeHTTP(rec1, req1)

		if rec1.Code != http.StatusOK {
			t.Errorf("first request: status = %d, want %d", rec1.Code, http.StatusOK)
		}

		// Second request is rate limited
		req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
		req2.RemoteAddr = "192.168.1.1:12345"
		rec2 := httptest.NewRecorder()
		middleware.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusTooManyRequests {
			t.Errorf("second request: status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
		}
	})
}

func TestExtractRemoteIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"with port", "192.168.1.1:12345", "192.168.1.1"},
		{"ipv6 with port", "[::1]:12345", "::1"},
		{"no port", "192.168.1.1", "192.168.1.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractRemoteIP(tc.remoteAddr)
			if got != tc.want {
				t.Errorf("extractRemoteIP(%q) = %q, want %q", tc.remoteAddr, got, tc.want)
			}
		})
	}
}

func TestServerWithSecurityConfig(t *testing.T) {
	t.Run("server applies authentication when enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Security = &SecurityConfig{
			Auth: AuthConfig{
				Enabled:   true,
				APIKeys:   []string{"test-key"},
				SkipPaths: []string{"/healthz"},
			},
		}

		server := NewServer(cfg, nil)

		// Request without API key should fail
		req := httptest.NewRequest(http.MethodGet, "/types", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated request: status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}

		// Request with API key should succeed
		req2 := httptest.NewRequest(http.MethodGet, "/types", nil)
		req2.Header.Set(APIKeyHeader, "test-key")
		rec2 := httptest.NewRecorder()
		server.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Errorf("authenticated request: status = %d, want %d", rec2.Code, http.StatusOK)
		}
	})

	t.Run("health endpoint bypasses auth", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Security = &SecurityConfig{
			Auth: AuthConfig{
				Enabled:   true,
				APIKeys:   []string{"test-key"},
				SkipPaths: []string{"/healthz"},
			},
		}

		server := NewServer(cfg, nil)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("health endpoint: status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("server applies rate limiting when enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Security = &SecurityConfig{
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerWindow: 2,
				WindowDuration:    time.Minute,
			},
		}

		server := NewServer(cfg, nil)

		// First two requests should succeed
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest(http.MethodGet, "/types", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
			}
		}

		// Third request should be rate limited
		req := httptest.NewRequest(http.MethodGet, "/types", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("rate limited request: status = %d, want %d", rec.Code, http.StatusTooManyRequests)
		}
	})

	t.Run("security headers are always applied", func(t *testing.T) {
		cfg := DefaultConfig()
		server := NewServer(cfg, nil)

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)

		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Error("expected security headers on all responses")
		}
	})
}

func TestValidateAPIKey(t *testing.T) {
	validKeys := [][]byte{
		[]byte("key-1"),
		[]byte("key-2"),
		[]byte("longer-key-with-more-characters"),
	}

	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid key 1", "key-1", true},
		{"valid key 2", "key-2", true},
		{"valid long key", "longer-key-with-more-characters", true},
		{"invalid key", "wrong-key", false},
		{"empty key", "", false},
		{"similar key", "key-3", false},
		{"partial key", "key-", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateAPIKey(tc.key, validKeys)
			if got != tc.valid {
				t.Errorf("validateAPIKey(%q) = %v, want %v", tc.key, got, tc.valid)
			}
		})
	}
}
