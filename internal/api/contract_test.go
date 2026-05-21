package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apierrors "github.com/sebahrens/json2pptx/internal/api/errors"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// Contract tests lock the HTTP response shapes that agents and integrations
// depend on. They assert specific JSON field names and types.
//
// Stable fields (safe for programmatic matching):
//   - transport-error envelope (apierrors.Response): success (bool),
//     error.code (string), error.message (string)
//   - diagnostic-bearing FindingEnvelope (pattern validate/expand):
//     schema_version (string), tool (string), subcommand (string), ok (bool),
//     findings[].{id, code, severity, category, message}, findings[].evidence.pattern
//   - convert success: success (bool), stats.slide_count (int)
//   - pattern validate: ok (bool)
//   - pattern show: name (string), schema (object)
//
// Advisory fields (human-readable, may change wording):
//   - error.message text, error.details free-form entries, finding.message text

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

// TestHTTPConvertUnsupportedField_RejectsShapeGrid verifies that sending
// unsupported fields like shape_grid returns UNSUPPORTED_FEATURE, not a
// silent pass.
func TestHTTPConvertUnsupportedField_RejectsShapeGrid(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	// JSON with shape_grid — not in APISlide struct, should be rejected.
	body := `{
		"template": "test",
		"slides": [{
			"type": "content",
			"title": "Test",
			"shape_grid": {"rows": []}
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}

	var resp apierrors.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse error response: %v", err)
	}

	if resp.Error.Code != apierrors.CodeUnsupportedFeature {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, apierrors.CodeUnsupportedFeature)
	}

	if resp.Error.Details == nil {
		t.Fatal("expected details to be non-nil")
	}

	if field, ok := resp.Error.Details["field"]; !ok || field != "shape_grid" {
		t.Errorf("expected details.field = \"shape_grid\", got %v", resp.Error.Details["field"])
	}

	// Should mention MCP/CLI as alternatives.
	if !strings.Contains(resp.Error.Message, "MCP") {
		t.Errorf("error message should mention MCP, got: %s", resp.Error.Message)
	}
}

// TestHTTPConvertUnsupportedField_RejectsPattern verifies pattern field is rejected.
func TestHTTPConvertUnsupportedField_RejectsPattern(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	body := `{
		"template": "test",
		"slides": [{
			"type": "content",
			"title": "Test",
			"pattern": {"name": "kpi-3up"}
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}

	var resp apierrors.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse error response: %v", err)
	}

	if resp.Error.Code != apierrors.CodeUnsupportedFeature {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, apierrors.CodeUnsupportedFeature)
	}
}

// TestHTTPConvertUnsupportedField_RejectsDeckLevelFields verifies deck-level
// MCP/CLI fields are rejected at the top-level ConvertRequest.
func TestHTTPConvertUnsupportedField_RejectsDeckLevelFields(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	body := `{
		"template": "test",
		"slides": [{"type": "content", "title": "Test"}],
		"design_mode": "free"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(body))
	w := httptest.NewRecorder()

	service.ConvertHandler()(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Status = %d, want 400", w.Code)
	}

	var resp apierrors.Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse error response: %v", err)
	}

	if resp.Error.Code != apierrors.CodeUnsupportedFeature {
		t.Errorf("error.code = %q, want %q", resp.Error.Code, apierrors.CodeUnsupportedFeature)
	}
}

// TestHTTPCapabilities_ContractShape verifies GET /api/v1/capabilities returns
// the expected machine-readable feature boundary.
func TestHTTPCapabilities_ContractShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	w := httptest.NewRecorder()

	CapabilitiesHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var resp CapabilitiesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if resp.Surface != "http" {
		t.Errorf("surface = %q, want \"http\"", resp.Surface)
	}

	if len(resp.ConvertCapabilities.SupportedSlideFields) == 0 {
		t.Error("expected non-empty supported_slide_fields")
	}

	if len(resp.ConvertCapabilities.SupportedContentFields) == 0 {
		t.Error("expected non-empty supported_content_fields")
	}

	if len(resp.ConvertCapabilities.UnsupportedFeatures) == 0 {
		t.Error("expected non-empty unsupported_features")
	}

	if len(resp.AvailableEndpoints) == 0 {
		t.Error("expected non-empty available_endpoints")
	}

	if len(resp.MCPOnlyFeatures) == 0 {
		t.Error("expected non-empty mcp_only_features")
	}

	if resp.RecommendedInterface == "" {
		t.Error("expected non-empty recommended_interface")
	}
}

// TestHTTPPatternValidationFailed_ContractShape verifies the HTTP pattern
// validation endpoint emits the shared diagnostics.FindingEnvelope — the same
// agent-facing contract as the CLI/MCP surfaces — rather than the legacy
// apierrors.Response with details.validation_errors.
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

	var env diagnostics.FindingEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("response is not a FindingEnvelope: %v", err)
	}

	// Run-level envelope fields.
	if env.SchemaVersion == "" {
		t.Error("envelope missing stable field schema_version")
	}
	if env.Tool == "" {
		t.Error("envelope missing stable field tool")
	}
	if env.Subcommand != "validate_pattern" {
		t.Errorf("subcommand = %q, want %q", env.Subcommand, "validate_pattern")
	}
	if env.OK {
		t.Error("expected ok=false for a failed validation")
	}
	if len(env.Findings) == 0 {
		t.Fatal("expected non-empty findings")
	}

	// Each finding carries the stable, machine-matchable fields.
	for i, f := range env.Findings {
		if f.ID == "" {
			t.Errorf("findings[%d] missing stable field id", i)
		}
		if f.Code == "" {
			t.Errorf("findings[%d] missing stable field code", i)
		}
		if f.Severity == "" {
			t.Errorf("findings[%d] missing stable field severity", i)
		}
		if f.Category == "" {
			t.Errorf("findings[%d] missing stable field category", i)
		}
		if f.Message == "" {
			t.Errorf("findings[%d] missing stable field message", i)
		}
	}

	// The originating pattern rides on the finding evidence so agents can
	// correlate the failure without a separate top-level field.
	if got := env.Findings[0].Evidence["pattern"]; got != "kpi-3up" {
		t.Errorf("findings[0].evidence.pattern = %v, want %q", got, "kpi-3up")
	}
}
