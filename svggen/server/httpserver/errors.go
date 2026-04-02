// Package httpserver provides the HTTP server for the SVG generation API.
package httpserver

import (
	"encoding/json"
	"net/http"
)

// errorResponse represents a standardized API error response.
type errorResponse struct {
	Success bool        `json:"success"`
	Error   errorDetail `json:"error"`
}

// errorDetail contains the error information.
type errorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error codes used by the SVG generation API.
const (
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeInvalidRequest  = "INVALID_REQUEST"
	CodeInvalidJSON     = "INVALID_JSON"
	CodeInvalidYAML     = "INVALID_YAML"
	CodeRenderError     = "RENDER_ERROR"
	CodeRateLimited     = "RATE_LIMITED"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeValidationError = "VALIDATION_ERROR"
)

// writeErrorResponse writes a standardized JSON error response.
func writeErrorResponse(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := errorResponse{
		Success: false,
		Error: errorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// writeRateLimited writes a 429 Too Many Requests error.
func writeRateLimited(w http.ResponseWriter, message string, details map[string]any) {
	writeErrorResponse(w, http.StatusTooManyRequests, CodeRateLimited, message, details)
}

// writeUnauthorized writes a 401 Unauthorized error.
func writeUnauthorized(w http.ResponseWriter, message string, details map[string]any) {
	writeErrorResponse(w, http.StatusUnauthorized, CodeUnauthorized, message, details)
}
