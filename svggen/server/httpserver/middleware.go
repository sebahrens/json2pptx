// Package httpserver provides the HTTP server for the SVG generation API.
package httpserver

import (
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SecurityConfig holds security-related configuration for the SVG API.
type SecurityConfig struct {
	// Auth holds API key authentication settings.
	// When Auth.Enabled is false, authentication is disabled.
	Auth AuthConfig

	// RateLimit holds rate limiting settings.
	RateLimit RateLimitConfig

	// AllowedOrigins is a list of allowed CORS origins.
	// Empty means no cross-origin requests allowed.
	AllowedOrigins []string

	// TrustedProxies is a list of CIDR ranges for trusted proxies.
	// Empty means no proxies are trusted (use direct connection IP).
	TrustedProxies []string
}

// AuthConfig holds API key authentication configuration.
type AuthConfig struct {
	// Enabled controls whether authentication is enforced.
	Enabled bool

	// APIKeys is a list of valid API keys.
	APIKeys []string

	// SkipPaths lists URL paths that bypass authentication.
	SkipPaths []string
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// Enabled controls whether rate limiting is enforced.
	Enabled bool

	// RequestsPerWindow is the maximum number of requests allowed per window.
	RequestsPerWindow int

	// WindowDuration is the time window for rate limiting.
	WindowDuration time.Duration
}

// DefaultSecurityConfig returns the default security configuration.
// By default, security features are disabled for backward compatibility.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		Auth: AuthConfig{
			Enabled:   false,
			SkipPaths: []string{"/healthz"},
		},
		RateLimit: RateLimitConfig{
			Enabled:           false,
			RequestsPerWindow: 100,
			WindowDuration:    time.Minute,
		},
	}
}

// APIKeyHeader is the HTTP header name for API key authentication.
const APIKeyHeader = "X-API-Key"

// authMiddleware creates an authentication middleware.
// This middleware validates API keys for all protected endpoints.
func authMiddleware(config AuthConfig) func(http.Handler) http.Handler {
	// Build a set of skip paths for O(1) lookup
	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	// Pre-convert API keys to bytes for constant-time comparison
	apiKeyBytes := make([][]byte, len(config.APIKeys))
	for i, key := range config.APIKeys {
		apiKeyBytes[i] = []byte(key)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip authentication if disabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip authentication for OPTIONS requests (CORS preflight)
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Skip authentication for exempted paths
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Get API key from header
			apiKey := r.Header.Get(APIKeyHeader)
			if apiKey == "" {
				writeUnauthorized(w, "API key is required", nil)
				return
			}

			// Validate API key using constant-time comparison
			if !validateAPIKey(apiKey, apiKeyBytes) {
				writeUnauthorized(w, "Invalid API key", nil)
				return
			}

			// Authentication successful
			next.ServeHTTP(w, r)
		})
	}
}

// validateAPIKey checks if the provided key matches any valid key.
// Uses constant-time comparison to prevent timing attacks.
func validateAPIKey(key string, validKeys [][]byte) bool {
	keyBytes := []byte(key)

	for _, validKey := range validKeys {
		if subtle.ConstantTimeCompare(keyBytes, validKey) == 1 {
			return true
		}
	}
	return false
}

// securityHeadersMiddleware adds security headers to responses.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Legacy XSS protection (for older browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Strict Content Security Policy - API only, no scripts/styles needed
		w.Header().Set("Content-Security-Policy", "default-src 'none'")

		// Enforce HTTPS (1 year, include subdomains)
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers to responses.
// Only allows requests from origins in the allowedOrigins map.
func corsMiddleware(next http.Handler, allowedOrigins map[string]bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if the origin is allowed
		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ringBufferSize is the maximum number of requests tracked per visitor.
const ringBufferSize = 128

// visitor tracks requests for a single IP address using a fixed-size ring buffer.
type visitor struct {
	requests [ringBufferSize]time.Time
	head     int
	count    int
	lastSeen time.Time
}

// RateLimiter tracks request rates per IP address.
type RateLimiter struct {
	mu             sync.RWMutex
	visitors       map[string]*visitor
	limit          int
	window         time.Duration
	trustedProxies []*net.IPNet
}

// NewRateLimiter creates a new rate limiter with specified limit and window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}

	go rl.cleanup()
	return rl
}

// NewRateLimiterWithTrustedProxies creates a rate limiter with trusted proxy configuration.
func NewRateLimiterWithTrustedProxies(limit int, window time.Duration, trustedProxyCIDRs []string) (*RateLimiter, error) {
	proxies := make([]*net.IPNet, 0, len(trustedProxyCIDRs))
	for _, cidr := range trustedProxyCIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try parsing as a single IP address
			ip := net.ParseIP(cidr)
			if ip == nil {
				return nil, fmt.Errorf("invalid trusted proxy CIDR or IP: %s", cidr)
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		proxies = append(proxies, ipNet)
	}

	rl := &RateLimiter{
		visitors:       make(map[string]*visitor),
		limit:          limit,
		window:         window,
		trustedProxies: proxies,
	}

	go rl.cleanup()
	return rl, nil
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{lastSeen: now}
		rl.visitors[ip] = v
	}

	v.lastSeen = now

	cutoff := now.Add(-rl.window)
	validCount := 0
	var earliestValid time.Time
	var foundEarliest bool

	for i := 0; i < v.count; i++ {
		idx := (v.head + i) % ringBufferSize
		reqTime := v.requests[idx]
		if reqTime.After(cutoff) {
			validCount++
			if !foundEarliest {
				earliestValid = reqTime
				foundEarliest = true
			}
		}
	}

	// Compact ring buffer
	for v.count > 0 {
		headTime := v.requests[v.head]
		if headTime.After(cutoff) {
			break
		}
		v.head = (v.head + 1) % ringBufferSize
		v.count--
	}

	resetTime := now.Add(rl.window)
	if foundEarliest {
		resetTime = earliestValid.Add(rl.window)
	}

	if validCount >= rl.limit {
		return false, 0, resetTime
	}

	tail := (v.head + v.count) % ringBufferSize
	v.requests[tail] = now
	if v.count < ringBufferSize {
		v.count++
	} else {
		v.head = (v.head + 1) % ringBufferSize
	}

	remaining := rl.limit - validCount - 1
	return true, remaining, resetTime
}

// cleanup removes stale visitors every minute.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		staleThreshold := now.Add(-rl.window * 2)

		for ip, v := range rl.visitors {
			if v.lastSeen.Before(staleThreshold) {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// isTrustedProxy checks if an IP address is in the trusted proxy list.
func (rl *RateLimiter) isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}

	rl.mu.RLock()
	proxies := rl.trustedProxies
	rl.mu.RUnlock()

	for _, ipNet := range proxies {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// getClientIP extracts the client IP address from the request.
func (rl *RateLimiter) getClientIP(r *http.Request) string {
	remoteIP := extractRemoteIP(r.RemoteAddr)

	if len(rl.trustedProxies) == 0 {
		return remoteIP
	}

	parsedRemoteIP := net.ParseIP(remoteIP)
	if parsedRemoteIP == nil {
		return remoteIP
	}

	if !rl.isTrustedProxy(parsedRemoteIP) {
		return remoteIP
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		clientIP := extractClientIPFromXFF(xff, rl)
		if clientIP != "" {
			return clientIP
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		xri = strings.TrimSpace(xri)
		if xri != "" {
			return xri
		}
	}

	return remoteIP
}

// extractRemoteIP extracts just the IP address from a host:port string.
func extractRemoteIP(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// extractClientIPFromXFF extracts the client IP from X-Forwarded-For header.
func extractClientIPFromXFF(xff string, rl *RateLimiter) string {
	ips := strings.Split(xff, ",")

	for i := len(ips) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(ips[i])
		if ip == "" {
			continue
		}

		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			continue
		}

		if !rl.isTrustedProxy(parsedIP) {
			return ip
		}
	}

	if len(ips) > 0 {
		return strings.TrimSpace(ips[0])
	}

	return ""
}

// RateLimitMiddleware returns a middleware that enforces rate limiting.
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := limiter.getClientIP(r)
			allowed, remaining, resetTime := limiter.Allow(ip)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

			if !allowed {
				writeRateLimited(w,
					fmt.Sprintf("Rate limit exceeded. Try again after %s", resetTime.Format(time.RFC3339)),
					map[string]any{"reset_at": resetTime.Unix()})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
