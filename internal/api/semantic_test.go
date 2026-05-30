package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// validDeckSpecJSON is a minimal spec that validates and compiles cleanly.
const validDeckSpecJSON = `{
  "meta": {"title": "Q3 Review", "archetype": "strategy_proposal"},
  "slides": [
    {"kind": "title", "title": "Q3 Review"},
    {"kind": "section", "title": "Results"}
  ]
}`

// invalidDeckSpecJSON parses as JSON but fails semantic validation: the second
// slide has an unknown kind, which is an always-error structural finding.
const invalidDeckSpecJSON = `{
  "meta": {"title": "Broken"},
  "slides": [
    {"kind": "title", "title": "Broken"},
    {"kind": "not_a_real_kind", "title": "Oops"}
  ]
}`

func postSemantic(t *testing.T, handler http.HandlerFunc, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestSemanticSchemaHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/semantic/schema", nil)
	w := httptest.NewRecorder()
	SemanticSchemaHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var schema map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &schema); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if schema["title"] != "DeckSpec" {
		t.Errorf("schema title = %v, want DeckSpec", schema["title"])
	}
	if _, ok := schema["$defs"]; !ok {
		t.Errorf("schema missing $defs")
	}
}

func TestSemanticValidateHandler_Success(t *testing.T) {
	w := postSemantic(t, SemanticValidateHandler(), "/api/v1/semantic/validate", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp semanticValidateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !resp.Valid {
		t.Errorf("valid = false, want true; findings=%+v", resp.Findings.Findings)
	}
	if resp.Findings.SchemaVersion != diagnostics.SchemaVersion {
		t.Errorf("schema_version = %q, want %q", resp.Findings.SchemaVersion, diagnostics.SchemaVersion)
	}
	if resp.Findings.Subcommand != "semantic_validate" {
		t.Errorf("subcommand = %q, want semantic_validate", resp.Findings.Subcommand)
	}
}

func TestSemanticValidateHandler_InvalidSpec(t *testing.T) {
	w := postSemantic(t, SemanticValidateHandler(), "/api/v1/semantic/validate", "application/json", invalidDeckSpecJSON)
	// Validation completing is still a 200; the result lives in the envelope.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp semanticValidateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp.Valid {
		t.Errorf("valid = true, want false for unknown slide kind")
	}
	if resp.Findings.OK {
		t.Errorf("findings.ok = true, want false")
	}
	if len(resp.Findings.Findings) == 0 {
		t.Fatalf("expected at least one finding")
	}
	found := false
	for _, f := range resp.Findings.Findings {
		if strings.Contains(f.Code, "SEMANTIC_UNKNOWN_KIND") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a SEMANTIC_UNKNOWN_KIND finding; got %+v", resp.Findings.Findings)
	}
}

func TestSemanticValidateHandler_YAMLBody(t *testing.T) {
	yaml := "meta:\n  title: YAML Deck\nslides:\n  - kind: title\n    title: YAML Deck\n"
	w := postSemantic(t, SemanticValidateHandler(), "/api/v1/semantic/validate", "application/x-yaml", yaml)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp semanticValidateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !resp.Valid {
		t.Errorf("valid = false, want true for clean YAML spec; findings=%+v", resp.Findings.Findings)
	}
}

func TestSemanticCompileHandler_Success(t *testing.T) {
	w := postSemantic(t, SemanticCompileHandler(),
		"/api/v1/semantic/compile?include_compiled_json=true", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp semanticCompileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if !resp.OK {
		t.Fatalf("ok = false, want true; error=%q findings=%+v", resp.Error, resp.Findings.Findings)
	}
	if resp.SlideCount != 2 {
		t.Errorf("slide_count = %d, want 2", resp.SlideCount)
	}
	if len(resp.CompiledJSON) == 0 {
		t.Errorf("compiled_json empty despite include_compiled_json=true")
	}
	if resp.Findings.Subcommand != "semantic_compile" {
		t.Errorf("subcommand = %q, want semantic_compile", resp.Findings.Subcommand)
	}
}

func TestSemanticCompileHandler_OmitsCompiledJSONByDefault(t *testing.T) {
	w := postSemantic(t, SemanticCompileHandler(), "/api/v1/semantic/compile", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp semanticCompileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if len(resp.CompiledJSON) != 0 {
		t.Errorf("compiled_json should be omitted by default")
	}
}

func TestSemanticCompileHandler_InvalidSpec(t *testing.T) {
	w := postSemantic(t, SemanticCompileHandler(), "/api/v1/semantic/compile", "application/json", invalidDeckSpecJSON)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", w.Code, w.Body.String())
	}
	var resp semanticCompileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp.OK {
		t.Errorf("ok = true, want false for invalid spec")
	}
	if resp.Error == "" {
		t.Errorf("expected a blocking error message")
	}
	if len(resp.Findings.Findings) == 0 {
		t.Errorf("expected blocking findings in envelope")
	}
}

func TestSemanticCompileHandler_MalformedJSON(t *testing.T) {
	w := postSemantic(t, SemanticCompileHandler(), "/api/v1/semantic/compile", "application/json", `{"slides": [`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body=%s", w.Code, w.Body.String())
	}
	var resp semanticCompileResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if resp.OK {
		t.Errorf("ok = true, want false for malformed JSON")
	}
}

func TestSemanticRenderHandler_Deferred(t *testing.T) {
	w := postSemantic(t, SemanticRenderHandler(), "/api/v1/semantic/render", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("response missing error object: %v", resp)
	}
	if errObj["code"] != "UNSUPPORTED_FEATURE" {
		t.Errorf("error code = %v, want UNSUPPORTED_FEATURE", errObj["code"])
	}
}

func TestSemanticValidateHandler_EmptyBody(t *testing.T) {
	w := postSemantic(t, SemanticValidateHandler(), "/api/v1/semantic/validate", "application/json", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for empty body", w.Code)
	}
}

func TestSemanticValidateHandler_UnsupportedContentType(t *testing.T) {
	w := postSemantic(t, SemanticValidateHandler(), "/api/v1/semantic/validate", "text/csv", validDeckSpecJSON)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415 for unsupported content type", w.Code)
	}
}

func TestSemanticValidateHandler_InvalidStrict(t *testing.T) {
	w := postSemantic(t, SemanticValidateHandler(),
		"/api/v1/semantic/validate?strict=bogus", "application/json", validDeckSpecJSON)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid strict value", w.Code)
	}
}

func TestSemanticValidateHandler_StrictPromotesAdvisories(t *testing.T) {
	// A spec with no archetype and a single content-bearing slide missing a
	// takeaway carries advisory findings. Under strict they become errors.
	spec := `{
      "meta": {"title": "Advisory Deck"},
      "slides": [
        {"kind": "title", "title": "Advisory Deck"},
        {"kind": "executive_summary", "title": "Summary", "points": ["a", "b", "c"]}
      ]
    }`
	warn := postSemantic(t, SemanticValidateHandler(),
		"/api/v1/semantic/validate?strict=warn", "application/json", spec)
	var warnResp semanticValidateResponse
	if err := json.Unmarshal(warn.Body.Bytes(), &warnResp); err != nil {
		t.Fatalf("warn response not JSON: %v", err)
	}
	if !warnResp.Valid {
		t.Errorf("under warn the advisory should not block validity")
	}

	strict := postSemantic(t, SemanticValidateHandler(),
		"/api/v1/semantic/validate?strict=strict", "application/json", spec)
	var strictResp semanticValidateResponse
	if err := json.Unmarshal(strict.Body.Bytes(), &strictResp); err != nil {
		t.Fatalf("strict response not JSON: %v", err)
	}
	if strictResp.Valid {
		t.Errorf("under strict the advisory should be promoted to a blocking error")
	}
}
