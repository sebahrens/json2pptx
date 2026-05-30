package api

import (
	"net/http"
)

// CapabilitiesResponse describes the machine-readable feature boundary
// of the HTTP API surface.
type CapabilitiesResponse struct {
	Surface              string               `json:"surface"`
	Version              string               `json:"version"`
	ConvertCapabilities  ConvertCapabilities  `json:"convert"`
	SemanticCapabilities SemanticCapabilities `json:"semantic"`
	AvailableEndpoints   []string             `json:"available_endpoints"`
	MCPOnlyFeatures      []string             `json:"mcp_only_features"`
	RecommendedInterface string               `json:"recommended_interface"`
}

// ConvertCapabilities describes what POST /api/v1/convert supports.
type ConvertCapabilities struct {
	SupportedSlideFields   []string `json:"supported_slide_fields"`
	SupportedContentFields []string `json:"supported_content_fields"`
	UnsupportedFeatures    []string `json:"unsupported_features"`
}

// SemanticCapabilities describes what the /api/v1/semantic/* endpoints support.
// Render is deferred until the render orchestration is extracted into internal/,
// so it is advertised as unavailable with its CLI/MCP alternatives.
type SemanticCapabilities struct {
	SupportedOperations []string `json:"supported_operations"`
	DeferredOperations  []string `json:"deferred_operations"`
	RequestBody         string   `json:"request_body"`
	QueryParameters     []string `json:"query_parameters"`
	DiagnosticContract  string   `json:"diagnostic_contract"`
}

// CapabilitiesHandler returns a handler for GET /api/v1/capabilities.
func CapabilitiesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := CapabilitiesResponse{
			Surface: "http",
			Version: "1.0.0",
			ConvertCapabilities: ConvertCapabilities{
				SupportedSlideFields: []string{
					"type", "title", "content", "speaker_notes",
					"source", "transition", "transition_speed", "build",
				},
				SupportedContentFields: []string{
					"body", "bullets",
				},
				UnsupportedFeatures: []string{
					"shape_grid (use MCP or CLI)",
					"pattern / compose (use MCP expand_pattern then CLI)",
					"chart_value (use MCP generate_presentation)",
					"diagram_value (use MCP generate_presentation)",
					"table_value (use MCP generate_presentation)",
					"image_value (use MCP generate_presentation)",
					"bullet_groups_value (use MCP generate_presentation)",
					"body_and_bullets_value (use MCP generate_presentation)",
					"design_mode (MCP/CLI only)",
					"accent_strategy (MCP/CLI only)",
					"footer / chrome (MCP/CLI only)",
					"theme_override (MCP/CLI only)",
					"defaults block (MCP/CLI only)",
					"structure block (MCP/CLI only)",
					"layout_id (MCP/CLI only)",
					"eyebrow (MCP/CLI only)",
					"background (MCP/CLI only)",
					"contrast_check (MCP/CLI only)",
				},
			},
			SemanticCapabilities: SemanticCapabilities{
				SupportedOperations: []string{"schema", "validate", "compile"},
				DeferredOperations: []string{
					"render (use `json2pptx semantic render` CLI or render_deck_spec MCP; HTTP returns 501)",
				},
				RequestBody:     "Raw semantic deck spec document; application/json parsed as JSON, application/x-yaml as YAML.",
				QueryParameters: []string{"strict (off|warn|strict, default warn)", "template", "include_compiled_json (compile only)"},
				DiagnosticContract: "Diagnostic-bearing responses use the shared FindingEnvelope; " +
					"transport/request errors use the simple error envelope.",
			},
			AvailableEndpoints: []string{
				"GET  /api/v1/health",
				"GET  /api/v1/capabilities",
				"GET  /api/v1/templates",
				"GET  /api/v1/templates/{name}",
				"GET  /api/v1/slide-types",
				"GET  /api/v1/semantic/schema",
				"POST /api/v1/semantic/validate",
				"POST /api/v1/semantic/compile",
				"POST /api/v1/semantic/render",
				"POST /api/v1/convert",
				"GET  /api/v1/download/{filename}",
				"GET  /api/v1/patterns",
				"GET  /api/v1/patterns/{name}",
				"POST /api/v1/patterns/{name}/validate",
				"POST /api/v1/patterns/{name}/expand",
			},
			MCPOnlyFeatures: []string{
				"generate_presentation (full PresentationInput with all content types)",
				"validate_input (full schema validation)",
				"expand_pattern (pattern → shape_grid expansion)",
				"show_pattern (pattern schema introspection)",
				"list_patterns (pattern catalog)",
				"analyze_deck_rhythm (composition analysis)",
				"recommend_visual (layout/pattern recommendation)",
				"plan_deck (deck planning from brief)",
				"repair_slide (fit-report-driven repair)",
			},
			RecommendedInterface: "For decks with charts, diagrams, tables, images, shape grids, " +
				"or named patterns, use the MCP interface (json2pptx mcp) or the CLI " +
				"(json2pptx generate). The HTTP API supports basic text and bullet slides only.",
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
