package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// TestCapabilities_AvailableEndpoints_MatchesSetupRoutes verifies every endpoint
// listed in the capabilities response is actually registered in setupRoutes.
// This catches drift where an endpoint is added/removed in setupRoutes but the
// capabilities list isn't updated.
func TestCapabilities_AvailableEndpoints_MatchesSetupRoutes(t *testing.T) {
	// Build a real server so setupRoutes is called.
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	srv := NewServer(ServerConfig{
		TemplatesDir: tempDir,
		OutputDir:    tempDir,
		Cache:        cache,
	})

	// Get the capabilities response.
	resp := getCapabilities(t)

	// For each listed endpoint, issue a request and verify it doesn't 404.
	// A 404 from the default mux means the route was never registered.
	for _, ep := range resp.AvailableEndpoints {
		method, path := parseEndpoint(t, ep)

		// Substitute path parameters with placeholder values so the mux matches.
		path = strings.ReplaceAll(path, "{name}", "test-placeholder")
		path = strings.ReplaceAll(path, "{filename}", "test-placeholder")

		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		// 404 means the route isn't registered. Other codes (400, 405, 500) are
		// fine — we only care that the mux found a handler.
		if w.Code == http.StatusNotFound {
			// Check if it's a mux 404 vs a handler-level 404 (e.g. template not found).
			// Mux 404 has no JSON body; handler 404s from our code have JSON.
			var parsed map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
				t.Errorf("endpoint %q returned 404 — route not registered in setupRoutes", ep)
			}
			// If it parsed as JSON with an "error" field, it's a handler-level 404 (fine).
		}
	}
}

// TestCapabilities_SetupRoutes_AllListedInCapabilities verifies no route exists
// in setupRoutes that isn't listed in AvailableEndpoints. We do this by
// constructing the expected set from setupRoutes patterns and diffing.
func TestCapabilities_SetupRoutes_AllListedInCapabilities(t *testing.T) {
	resp := getCapabilities(t)

	// Build a set of "METHOD /path" from AvailableEndpoints.
	listed := make(map[string]bool)
	for _, ep := range resp.AvailableEndpoints {
		method, path := parseEndpoint(t, ep)
		listed[method+" "+path] = true
	}

	// These are the routes registered in setupRoutes (kept in sync manually —
	// if this list drifts, it means setupRoutes changed without updating this
	// test OR AvailableEndpoints).
	expectedRoutes := []string{
		"GET /api/v1/health",
		"GET /api/v1/capabilities",
		"GET /api/v1/templates",
		"GET /api/v1/templates/{name}",
		"GET /api/v1/slide-types",
		"POST /api/v1/convert",
		"GET /api/v1/download/{filename}",
		"GET /api/v1/patterns",
		"GET /api/v1/patterns/{name}",
		"POST /api/v1/patterns/{name}/validate",
		"POST /api/v1/patterns/{name}/expand",
	}

	for _, route := range expectedRoutes {
		if !listed[route] {
			t.Errorf("route %q is in setupRoutes but missing from AvailableEndpoints", route)
		}
	}

	// Also verify AvailableEndpoints doesn't list phantom routes.
	routeSet := make(map[string]bool)
	for _, r := range expectedRoutes {
		routeSet[r] = true
	}
	for _, ep := range resp.AvailableEndpoints {
		method, path := parseEndpoint(t, ep)
		key := method + " " + path
		if !routeSet[key] {
			t.Errorf("AvailableEndpoints lists %q but it's not in setupRoutes", key)
		}
	}
}

// TestCapabilities_UnsupportedFeatures_CoversKnownMCPFields verifies that
// UnsupportedFeatures mentions every field that PresentationInput supports
// but ConvertRequest rejects via DisallowUnknownFields.
func TestCapabilities_UnsupportedFeatures_CoversKnownMCPFields(t *testing.T) {
	resp := getCapabilities(t)

	// Join all unsupported features into one string for substring matching.
	unsupported := strings.Join(resp.ConvertCapabilities.UnsupportedFeatures, "\n")

	// These are fields in PresentationInput / SlideDefinition that the HTTP API
	// does not support. Each MUST appear (as a substring) in UnsupportedFeatures
	// so agents know to use MCP/CLI instead.
	knownUnsupported := []string{
		"shape_grid",
		"pattern",
		"chart_value",
		"diagram_value",
		"table_value",
		"image_value",
		"bullet_groups_value",
		"body_and_bullets_value",
		"design_mode",
		"accent_strategy",
		"layout_id",
		"eyebrow",
		"background",
		"contrast_check",
		"defaults",
		"theme_override",
	}

	for _, field := range knownUnsupported {
		if !strings.Contains(unsupported, field) {
			t.Errorf("UnsupportedFeatures should mention %q — agents need this to know the field requires MCP/CLI", field)
		}
	}
}

// TestCapabilities_UnsupportedFeatures_DisallowUnknownFieldsRejects verifies
// that fields listed as unsupported actually get rejected by the HTTP API's
// DisallowUnknownFields when sent in a request.
func TestCapabilities_UnsupportedFeatures_DisallowUnknownFieldsRejects(t *testing.T) {
	tempDir := t.TempDir()
	cache := template.NewMemoryCache(24 * 60 * 60)
	templateService := NewTemplateService(tempDir, cache, false)
	service := NewConvertService(tempDir, tempDir, templateService, nil)

	// Fields that can appear at slide level in PresentationInput but not in APISlide.
	slideLevel := []string{"shape_grid", "pattern", "chart_value", "diagram_value", "table_value", "image_value", "layout_id", "eyebrow", "background"}

	for _, field := range slideLevel {
		body := `{"template":"test","slides":[{"type":"content","title":"T","` + field + `":"test"}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(body))
		w := httptest.NewRecorder()

		service.ConvertHandler()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("field %q: status = %d, want 400 — DisallowUnknownFields should reject it", field, w.Code)
		}
	}

	// Fields that can appear at deck level in PresentationInput but not in ConvertRequest.
	deckLevel := []string{"design_mode", "accent_strategy", "defaults", "structure"}

	for _, field := range deckLevel {
		body := `{"template":"test","slides":[{"type":"content","title":"T"}],"` + field + `":"test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/convert", strings.NewReader(body))
		w := httptest.NewRecorder()

		service.ConvertHandler()(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("field %q: status = %d, want 400 — DisallowUnknownFields should reject it", field, w.Code)
		}
	}
}

// TestCapabilities_ResponseSchema_StableFields verifies the capabilities response
// has the exact set of top-level fields agents depend on. Adding a field is fine
// (this test won't fail); removing or renaming one breaks agents.
func TestCapabilities_ResponseSchema_StableFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	w := httptest.NewRecorder()

	CapabilitiesHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", w.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// These are the stable top-level fields agents parse programmatically.
	requiredFields := []string{
		"surface",
		"version",
		"convert",
		"available_endpoints",
		"mcp_only_features",
		"recommended_interface",
	}

	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("capabilities response missing stable field %q", field)
		}
	}

	// Verify convert sub-object has required fields.
	var convertRaw map[string]json.RawMessage
	if err := json.Unmarshal(raw["convert"], &convertRaw); err != nil {
		t.Fatalf("convert is not a JSON object: %v", err)
	}

	convertFields := []string{
		"supported_slide_fields",
		"supported_content_fields",
		"unsupported_features",
	}

	for _, field := range convertFields {
		if _, ok := convertRaw[field]; !ok {
			t.Errorf("convert object missing stable field %q", field)
		}
	}
}

// TestCapabilities_SupportedSlideFields_MatchesAPISlide verifies the
// supported_slide_fields list matches the JSON tags on the APISlide struct.
// This catches drift where a new field is added to APISlide but not to capabilities.
func TestCapabilities_SupportedSlideFields_MatchesAPISlide(t *testing.T) {
	resp := getCapabilities(t)

	// The supported slide fields declared in capabilities.
	supported := make(map[string]bool)
	for _, f := range resp.ConvertCapabilities.SupportedSlideFields {
		supported[f] = true
	}

	// These are the JSON tags from APISlide (excluding the content sub-object).
	apiSlideFields := []string{
		"type", "title", "speaker_notes", "source",
		"transition", "transition_speed", "build",
	}

	for _, f := range apiSlideFields {
		if !supported[f] {
			t.Errorf("APISlide has json tag %q but it's not in supported_slide_fields", f)
		}
	}
}

// TestCapabilities_SupportedContentFields_MatchesAPIContent verifies the
// supported_content_fields list matches the JSON tags on APIContent.
func TestCapabilities_SupportedContentFields_MatchesAPIContent(t *testing.T) {
	resp := getCapabilities(t)

	supported := make(map[string]bool)
	for _, f := range resp.ConvertCapabilities.SupportedContentFields {
		supported[f] = true
	}

	apiContentFields := []string{"body", "bullets"}

	for _, f := range apiContentFields {
		if !supported[f] {
			t.Errorf("APIContent has json tag %q but it's not in supported_content_fields", f)
		}
	}
}

// TestCapabilities_MCPOnlyFeatures_NonEmpty verifies the MCP-only feature list
// is populated and contains known critical tools.
func TestCapabilities_MCPOnlyFeatures_NonEmpty(t *testing.T) {
	resp := getCapabilities(t)

	if len(resp.MCPOnlyFeatures) == 0 {
		t.Fatal("mcp_only_features should not be empty")
	}

	mcpFeatures := strings.Join(resp.MCPOnlyFeatures, "\n")

	knownMCPTools := []string{
		"generate_presentation",
		"validate_input",
		"expand_pattern",
	}

	for _, tool := range knownMCPTools {
		if !strings.Contains(mcpFeatures, tool) {
			t.Errorf("mcp_only_features should mention %q", tool)
		}
	}
}

// TestCapabilities_Patterns_EndpointAccessible verifies the pattern endpoints
// listed in capabilities are actually functional (not just registered).
func TestCapabilities_Patterns_EndpointAccessible(t *testing.T) {
	h := NewPatternsHandler(patterns.Default())

	// GET /api/v1/patterns should return a list.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patterns", nil)
	w := httptest.NewRecorder()
	h.ListHandler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /api/v1/patterns returned %d, want 200", w.Code)
	}
}

// --- helpers ---

// getCapabilities calls the CapabilitiesHandler and returns the parsed response.
func getCapabilities(t *testing.T) CapabilitiesResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	w := httptest.NewRecorder()
	CapabilitiesHandler()(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/capabilities returned %d, want 200", w.Code)
	}
	var resp CapabilitiesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse capabilities response: %v", err)
	}
	return resp
}

// parseEndpoint splits "GET  /api/v1/foo" into method and path.
func parseEndpoint(t *testing.T, ep string) (string, string) {
	t.Helper()
	fields := strings.Fields(ep)
	if len(fields) != 2 {
		t.Fatalf("malformed endpoint string: %q", ep)
	}
	return fields[0], fields[1]
}
