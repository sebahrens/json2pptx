package errors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		code       string
		message    string
		details    map[string]any
		wantStatus int
	}{
		{
			name:       "basic error",
			status:     http.StatusBadRequest,
			code:       CodeInvalidRequest,
			message:    "Invalid request body",
			details:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "error with details",
			status:     http.StatusNotFound,
			code:       CodeFileNotFound,
			message:    "File not found",
			details:    map[string]any{"file": "test.pptx"},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal server error",
			status:     http.StatusInternalServerError,
			code:       CodeInternalError,
			message:    "Internal server error",
			details:    nil,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			Write(rec, tt.status, tt.code, tt.message, tt.details)

			// Check status code
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			// Check content type
			ct := rec.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want %q", ct, "application/json")
			}

			// Parse response
			var resp Response
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			// Check response fields
			if resp.Success {
				t.Error("success = true, want false")
			}
			if resp.Error.Code != tt.code {
				t.Errorf("code = %q, want %q", resp.Error.Code, tt.code)
			}
			if resp.Error.Message != tt.message {
				t.Errorf("message = %q, want %q", resp.Error.Message, tt.message)
			}

			// Check details
			if tt.details == nil && resp.Error.Details != nil {
				t.Error("details should be nil")
			}
			if tt.details != nil {
				for k, v := range tt.details {
					if resp.Error.Details[k] != v {
						t.Errorf("details[%s] = %v, want %v", k, resp.Error.Details[k], v)
					}
				}
			}
		})
	}
}

func TestWriteBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteBadRequest(rec, CodeInvalidJSON, "Invalid JSON", nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWriteNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteNotFound(rec, CodeFileNotFound, "File not found", nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestWriteInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteInternalError(rec, CodeInternalError, "Something went wrong", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestWriteRateLimited(t *testing.T) {
	rec := httptest.NewRecorder()
	details := map[string]any{"retry_after": 60}
	WriteRateLimited(rec, "Rate limit exceeded", details)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeRateLimited {
		t.Errorf("code = %q, want %q", resp.Error.Code, CodeRateLimited)
	}
}

func TestWriteRequestTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteRequestTooLarge(rec, "Request too large", nil)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeRequestTooLarge {
		t.Errorf("code = %q, want %q", resp.Error.Code, CodeRequestTooLarge)
	}
}

func TestWriteGatewayTimeout(t *testing.T) {
	rec := httptest.NewRecorder()
	details := map[string]any{"timeout_seconds": 120}
	WriteGatewayTimeout(rec, "Request processing timed out", details)

	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeRequestTimeout {
		t.Errorf("code = %q, want %q", resp.Error.Code, CodeRequestTimeout)
	}
}

func TestWriteUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteUnauthorized(rec, "Invalid API key", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeUnauthorized {
		t.Errorf("code = %q, want %q", resp.Error.Code, CodeUnauthorized)
	}
}

func TestWriteUnsupportedMediaType(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteUnsupportedMediaType(rec, "Unsupported content type", nil)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != CodeInvalidContentType {
		t.Errorf("code = %q, want %q", resp.Error.Code, CodeInvalidContentType)
	}
}

func TestResponseOmitsEmptyDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, http.StatusBadRequest, CodeInvalidRequest, "test", nil)

	// Decode as raw JSON to check structure
	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := raw["error"].(map[string]any)
	if !ok {
		t.Fatal("error field not found")
	}

	// Details should be omitted when nil
	if _, exists := errObj["details"]; exists {
		t.Error("details field should be omitted when nil")
	}
}
