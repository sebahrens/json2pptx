package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
)

// Liveness remains public for orchestration. Every other route, including
// downloads, requires the same optional bearer token.
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if s.apiToken == "" || (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path == "/api/v1/health" {
		return true
	}
	want := sha256.Sum256([]byte("Bearer " + s.apiToken))
	got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
	if subtle.ConstantTimeCompare(want[:], got[:]) == 1 {
		return true
	}
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, apierrors.CodeUnauthorized, "A valid bearer token is required", nil)
	return false
}

// CORS is opt-in. A preflight carries no bearer token: validate only its
// origin, requested method and headers, then authenticate the actual request.
func (s *Server) allowCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if len(s.corsAllowedOrigins) == 0 || origin == "" {
		return true
	}
	w.Header().Add("Vary", "Origin")
	allowed := false
	for _, candidate := range s.corsAllowedOrigins {
		if candidate == origin || candidate == "*" {
			allowed = true
			break
		}
	}
	if !allowed {
		writeError(w, http.StatusForbidden, apierrors.CodeInvalidRequest, "Origin is not allowed", nil)
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
	if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
		return true
	}
	w.Header().Add("Vary", "Access-Control-Request-Method")
	w.Header().Add("Vary", "Access-Control-Request-Headers")
	method := r.Header.Get("Access-Control-Request-Method")
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodPost {
		writeError(w, http.StatusForbidden, apierrors.CodeInvalidRequest, "CORS method is not allowed", nil)
		return false
	}
	for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "", "authorization", "content-type", "x-request-id":
		default:
			writeError(w, http.StatusForbidden, apierrors.CodeInvalidRequest, "CORS header is not allowed", nil)
			return false
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
	w.WriteHeader(http.StatusNoContent)
	return false
}
