package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// newTestPatternsHandler creates a PatternsHandler backed by the default registry.
func newTestPatternsHandler() *PatternsHandler {
	return NewPatternsHandler(patterns.Default())
}

func TestListPatterns(t *testing.T) {
	h := newTestPatternsHandler()
	req := httptest.NewRequest("GET", "/api/v1/patterns", nil)
	w := httptest.NewRecorder()

	h.ListHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp patternListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Patterns) == 0 {
		t.Fatal("Expected at least one pattern in list")
	}

	// Check that each pattern has required fields
	for _, p := range resp.Patterns {
		if p.Name == "" {
			t.Error("Pattern name should not be empty")
		}
		if p.Description == "" {
			t.Errorf("Pattern %q: description should not be empty", p.Name)
		}
		if p.Version < 1 {
			t.Errorf("Pattern %q: version should be >= 1, got %d", p.Name, p.Version)
		}
	}
}

func TestShowPattern(t *testing.T) {
	h := newTestPatternsHandler()

	t.Run("known pattern", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/patterns/kpi-3up", nil)
		req.SetPathValue("name", "kpi-3up")
		w := httptest.NewRecorder()

		h.ShowHandler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp patternShowResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.Name != "kpi-3up" {
			t.Errorf("Name = %q, want %q", resp.Name, "kpi-3up")
		}
		if resp.Schema == nil {
			t.Error("Expected schema to be non-nil")
		}
	})

	t.Run("unknown pattern returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/patterns/nonexistent", nil)
		req.SetPathValue("name", "nonexistent")
		w := httptest.NewRecorder()

		h.ShowHandler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("Status = %d, want %d", w.Code, http.StatusNotFound)
		}

		var errResp apierrors.Response
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if errResp.Error.Code != apierrors.CodePatternNotFound {
			t.Errorf("Error code = %q, want %q", errResp.Error.Code, apierrors.CodePatternNotFound)
		}
	})
}

func TestValidatePattern(t *testing.T) {
	h := newTestPatternsHandler()

	t.Run("valid input returns ok", func(t *testing.T) {
		body := `{"values": [{"big":"100","small":"Revenue"},{"big":"200","small":"Users"},{"big":"300","small":"Growth"}]}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/validate", bytes.NewBufferString(body))
		req.SetPathValue("name", "kpi-3up")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ValidateHandler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp patternValidateResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !resp.OK {
			t.Error("Expected ok=true for valid input")
		}
	})

	t.Run("invalid values returns structured validation_errors", func(t *testing.T) {
		// Only 2 cells instead of required 3
		body := `{"values": [{"big":"100","small":"Revenue"},{"big":"200","small":"Users"}]}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/validate", bytes.NewBufferString(body))
		req.SetPathValue("name", "kpi-3up")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ValidateHandler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}

		var errResp apierrors.Response
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if errResp.Error.Code != apierrors.CodePatternValidationFailed {
			t.Errorf("Error code = %q, want %q", errResp.Error.Code, apierrors.CodePatternValidationFailed)
		}

		// Verify structured validation_errors in details
		details := errResp.Error.Details
		if details == nil {
			t.Fatal("Expected details to be non-nil")
		}
		if details["pattern"] != "kpi-3up" {
			t.Errorf("details.pattern = %v, want %q", details["pattern"], "kpi-3up")
		}
		veRaw, ok := details["validation_errors"]
		if !ok {
			t.Fatal("Expected details.validation_errors to be present")
		}
		veSlice, ok := veRaw.([]any)
		if !ok || len(veSlice) == 0 {
			t.Fatalf("Expected validation_errors to be a non-empty array, got %T", veRaw)
		}
		// Each entry should have code and message at minimum
		entry, ok := veSlice[0].(map[string]any)
		if !ok {
			t.Fatalf("Expected validation_errors[0] to be an object, got %T", veSlice[0])
		}
		if _, hasCode := entry["code"]; !hasCode {
			t.Error("Expected validation_errors[0] to have 'code' field")
		}
		if _, hasMsg := entry["message"]; !hasMsg {
			t.Error("Expected validation_errors[0] to have 'message' field")
		}
	})

	t.Run("unknown pattern returns 404", func(t *testing.T) {
		body := `{"values": []}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/nonexistent/validate", bytes.NewBufferString(body))
		req.SetPathValue("name", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ValidateHandler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("Status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("malformed JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/validate", bytes.NewBufferString("{bad json"))
		req.SetPathValue("name", "kpi-3up")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ValidateHandler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var errResp apierrors.Response
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}
		if errResp.Error.Code != apierrors.CodeInvalidJSON {
			t.Errorf("Error code = %q, want %q", errResp.Error.Code, apierrors.CodeInvalidJSON)
		}
	})

	t.Run("missing values returns 400", func(t *testing.T) {
		body := `{}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/validate", bytes.NewBufferString(body))
		req.SetPathValue("name", "kpi-3up")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ValidateHandler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})
}

func TestExpandPattern(t *testing.T) {
	h := newTestPatternsHandler()

	t.Run("valid input returns shape_grid", func(t *testing.T) {
		body := `{"values": [{"big":"100","small":"Revenue"},{"big":"200","small":"Users"},{"big":"300","small":"Growth"}]}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/expand", bytes.NewBufferString(body))
		req.SetPathValue("name", "kpi-3up")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ExpandHandler().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}

		var resp patternExpandResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if resp.ShapeGrid == nil {
			t.Error("Expected shape_grid to be non-nil")
		}
	})

	t.Run("invalid input returns 400", func(t *testing.T) {
		body := `{"values": [{"big":"100","small":"Revenue"}]}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/expand", bytes.NewBufferString(body))
		req.SetPathValue("name", "kpi-3up")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ExpandHandler().ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("Status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
		}
	})

	t.Run("unknown pattern returns 404", func(t *testing.T) {
		body := `{"values": []}`
		req := httptest.NewRequest("POST", "/api/v1/patterns/nonexistent/expand", bytes.NewBufferString(body))
		req.SetPathValue("name", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.ExpandHandler().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("Status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestPatternsRouting(t *testing.T) {
	// Test that routes are properly wired into the server via setupRoutes.
	tempDir := t.TempDir()

	server := NewServer(ServerConfig{
		TemplatesDir: tempDir,
		OutputDir:    tempDir,
	})

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"list patterns", "GET", "/api/v1/patterns", "", http.StatusOK},
		{"show known pattern", "GET", "/api/v1/patterns/kpi-3up", "", http.StatusOK},
		{"show unknown pattern", "GET", "/api/v1/patterns/nonexistent", "", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("Status = %d, want %d; body: %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}
