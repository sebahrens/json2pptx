// Package errors provides standardized API error handling across all HTTP services.
package errors

import (
	"encoding/json"
	"net/http"
)

// Response represents a standardized API error response.
// This format is used consistently across all API endpoints.
type Response struct {
	Success bool   `json:"success"`
	Error   Detail `json:"error"`
}

// Detail contains the error information.
type Detail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Common error codes used across APIs.
const (
	// Authentication errors (CRIT-01)
	CodeUnauthorized = "UNAUTHORIZED"

	// Request validation errors
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeInvalidJSON    = "INVALID_JSON"
	CodeInvalidYAML    = "INVALID_YAML"

	// Template errors
	CodeTemplateError    = "TEMPLATE_ERROR"
	CodeInvalidTemplate  = "INVALID_TEMPLATE"
	CodeTemplateNotFound = "TEMPLATE_NOT_FOUND"

	// Resource errors
	CodeFileNotFound = "FILE_NOT_FOUND"
	CodeFileError    = "FILE_ERROR"

	// Processing errors
	CodeInvalidInput       = "INVALID_INPUT"
	CodeInvalidSlideType   = "INVALID_SLIDE_TYPE"
	CodeGenerationError    = "GENERATION_ERROR"
	CodeRenderError        = "RENDER_ERROR"
	CodeRateLimited        = "RATE_LIMITED"
	CodeRequestTooLarge    = "REQUEST_TOO_LARGE"
	CodeRequestTimeout     = "REQUEST_TIMEOUT"
	CodeInvalidContentType = "INVALID_CONTENT_TYPE"

	// Server errors
	CodeInternalError = "INTERNAL_ERROR"
)

// Write writes a standardized JSON error response to the http.ResponseWriter.
// It sets the Content-Type header to application/json and writes the status code.
func Write(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := Response{
		Success: false,
		Error: Detail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	// Encode error - if encoding fails, we can't send another response
	// since headers are already written
	_ = json.NewEncoder(w).Encode(response)
}

// WriteBadRequest writes a 400 Bad Request error with the given code and message.
func WriteBadRequest(w http.ResponseWriter, code, message string, details map[string]any) {
	Write(w, http.StatusBadRequest, code, message, details)
}

// WriteNotFound writes a 404 Not Found error with the given code and message.
func WriteNotFound(w http.ResponseWriter, code, message string, details map[string]any) {
	Write(w, http.StatusNotFound, code, message, details)
}

// WriteInternalError writes a 500 Internal Server Error with the given code and message.
func WriteInternalError(w http.ResponseWriter, code, message string, details map[string]any) {
	Write(w, http.StatusInternalServerError, code, message, details)
}

// WriteRateLimited writes a 429 Too Many Requests error.
func WriteRateLimited(w http.ResponseWriter, message string, details map[string]any) {
	Write(w, http.StatusTooManyRequests, CodeRateLimited, message, details)
}

// WriteRequestTooLarge writes a 413 Request Entity Too Large error.
func WriteRequestTooLarge(w http.ResponseWriter, message string, details map[string]any) {
	Write(w, http.StatusRequestEntityTooLarge, CodeRequestTooLarge, message, details)
}

// WriteUnauthorized writes a 401 Unauthorized error (CRIT-01).
func WriteUnauthorized(w http.ResponseWriter, message string, details map[string]any) {
	Write(w, http.StatusUnauthorized, CodeUnauthorized, message, details)
}

// WriteUnsupportedMediaType writes a 415 Unsupported Media Type error.
func WriteUnsupportedMediaType(w http.ResponseWriter, message string, details map[string]any) {
	Write(w, http.StatusUnsupportedMediaType, CodeInvalidContentType, message, details)
}

// WriteGatewayTimeout writes a 504 Gateway Timeout error.
func WriteGatewayTimeout(w http.ResponseWriter, message string, details map[string]any) {
	Write(w, http.StatusGatewayTimeout, CodeRequestTimeout, message, details)
}
