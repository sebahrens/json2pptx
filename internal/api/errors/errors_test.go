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

func TestWriteInternalError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteInternalError(rec, CodeInternalError, "Something went wrong", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
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
