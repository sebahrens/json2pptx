package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// Contract tests lock the HTTP response shapes that agents and integrations
// depend on. They assert specific JSON field names and types.
//
// Stable fields (safe for programmatic matching):
//   - error envelope: success (bool), error.code (string), error.message (string)
//   - error.details.validation_errors[].code, .message, .path, .fix
//   - convert success: success (bool), stats.slide_count (int)
//   - pattern validate: ok (bool)
//   - pattern show: name (string), schema (object)
//
// Advisory fields (human-readable, may change wording):
//   - error.message text, error.details free-form entries

// TestHTTPConvertMalformedJSON_ContractShape verifies the HTTP convert endpoint
// returns the stable error envelope for malformed JSON input.
func TestHTTPConvertMalformedJSON_ContractShape(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(`{bad json`))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}

	// Parse into raw map to assert field names.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Top-level: success (bool) and error (object).
	for _, field := range []string{"success", "error"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("error response missing stable field %q", field)
		}
	}

	var success bool
	if err := json.Unmarshal(raw["success"], &success); err != nil {
		t.Errorf("success is not a boolean: %v", err)
	}
	if success {
		t.Error("expected success=false for error response")
	}

	// Error object: code (string) and message (string).
	var errObj map[string]json.RawMessage
	if err := json.Unmarshal(raw["error"], &errObj); err != nil {
		t.Fatalf("error is not a JSON object: %v", err)
	}

	for _, field := range []string{"code", "message"} {
		if _, ok := errObj[field]; !ok {
			t.Errorf("error object missing stable field %q", field)
		}
	}

	var code string
	if err := json.Unmarshal(errObj["code"], &code); err != nil {
		t.Errorf("error.code is not a string: %v", err)
	}
	if code != apierrors.CodeInvalidJSON {
		t.Errorf("error.code = %q, want %q", code, apierrors.CodeInvalidJSON)
	}
}

// TestHTTPConvertSyntaxError_DetailsShape verifies that JSON syntax errors
// include offset in the details, which agents use for error location.
func TestHTTPConvertSyntaxError_DetailsShape(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(`{"template": x}`))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}

	var resp apierrors.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse error response: %v", err)
	}

	if resp.Error.Details == nil {
		t.Fatal("expected details to be non-nil for syntax errors")
	}
	if _, hasOffset := resp.Error.Details["offset"]; !hasOffset {
		t.Error("expected details.offset for JSON syntax errors — agents use this to locate the error")
	}
}

// TestHTTPPatternValidationFailed_ContractShape verifies the HTTP pattern
// validation error response includes structured validation_errors.
func TestHTTPPatternValidationFailed_ContractShape(t *testing.T) {
	h := NewPatternsHandler(patterns.Default())

	body := `{"values": [{"big":"100","small":"Revenue"}]}`
	req := httptest.NewRequest("POST", "/api/v1/patterns/kpi-3up/validate", bytes.NewBufferString(body))
	req.SetPathValue("name", "kpi-3up")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ValidateHandler().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}

	// Parse raw to assert field names.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Top-level error envelope.
	errRaw, ok := raw["error"]
	if !ok {
		t.Fatal("missing 'error' field")
	}

	var errObj map[string]json.RawMessage
	if err := json.Unmarshal(errRaw, &errObj); err != nil {
		t.Fatalf("error is not a JSON object: %v", err)
	}

	// details must contain pattern and validation_errors.
	detailsRaw, ok := errObj["details"]
	if !ok {
		t.Fatal("error missing 'details' field")
	}

	var details map[string]json.RawMessage
	if err := json.Unmarshal(detailsRaw, &details); err != nil {
		t.Fatalf("details is not a JSON object: %v", err)
	}

	if _, ok := details["pattern"]; !ok {
		t.Error("details missing 'pattern' field")
	}

	veRaw, ok := details["validation_errors"]
	if !ok {
		t.Fatal("details missing 'validation_errors' field")
	}

	var validationErrors []map[string]json.RawMessage
	if err := json.Unmarshal(veRaw, &validationErrors); err != nil {
		t.Fatalf("validation_errors is not an array: %v", err)
	}
	if len(validationErrors) == 0 {
		t.Fatal("expected non-empty validation_errors")
	}

	// Each validation error must have code and message.
	for i, ve := range validationErrors {
		for _, field := range []string{"code", "message"} {
			if _, ok := ve[field]; !ok {
				t.Errorf("validation_errors[%d] missing stable field %q", i, field)
			}
		}
	}
}
