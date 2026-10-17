package api

import (
	"net/http"
)

// CapabilitiesResponse describes the machine-readable feature boundary
// of the HTTP API surface.
type CapabilitiesResponse struct {
	SchemaVersion        string               `json:"schema_version,omitempty"`
	SchemaFingerprint    string               `json:"schema_fingerprint,omitempty"`
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
// Render runs the same presentation runner as the CLI / MCP render surfaces.
// DeferredOperations is kept (empty) so clients that read it see nothing
// deferred.
type SemanticCapabilities struct {
	ValidationMode      string   `json:"validation_mode"`
	SupportedOperations []string `json:"supported_operations"`
	DeferredOperations  []string `json:"deferred_operations"`
	RequestBody         string   `json:"request_body"`
	RenderRequestBody   string   `json:"render_request_body"`
	QueryParameters     []string `json:"query_parameters"`
	DiagnosticContract  string   `json:"diagnostic_contract"`
	RenderResponse      string   `json:"render_response"`
}

// CapabilitiesHandler returns a handler for GET /api/v1/capabilities.
func CapabilitiesHandler(configs ...ServerConfig) http.HandlerFunc {
	cfg := ServerConfig{Version: "dev"}
	if len(configs) > 0 {
		cfg = configs[0]
		if cfg.Version == "" {
			cfg.Version = "dev"
		}
	}
	validationMode := "schema_only"
	if cfg.SemanticValidator != nil {
		validationMode = "trial_render"
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		resp := CapabilitiesResponse{
			Surface:           "http",
			Version:           cfg.Version,
			SchemaVersion:     cfg.SchemaVersion,
			SchemaFingerprint: cfg.SchemaFingerprint,
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
					"pattern / compose (use HTTP semantic/render, MCP or CLI)",
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
				ValidationMode:      validationMode,
				SupportedOperations: []string{"schema", "kinds", "validate", "compile", "render"},
				DeferredOperations:  []string{},
				RequestBody:         "Raw semantic deck spec document; application/json parsed as JSON, application/x-yaml as YAML.",
				RenderRequestBody: "render also accepts multipart/form-data: a \"spec\" part (file or field), an optional " +
					"\"template\" .pptx part (bring-your-own template, used when the spec pins no meta.template) and " +
					"any number of \"assets\" file parts that relative asset paths in the spec resolve against.",
				QueryParameters: []string{
					"strict (off|warn|strict, default warn)", "template",
					"include_compiled_json (compile only)", "output_validation (render only; off|warn|strict, default strict)",
				},
				DiagnosticContract: "validate/compile responses use the shared FindingEnvelope; render returns the " +
					"`json2pptx semantic render` result object (diagnostics carry semantic_path plus raw_path); " +
					"transport/request errors use the simple error envelope. validate reports validation_scope (schema, compile, template_free, render).",
				RenderResponse: "200 with file_url (GET /api/v1/download/{filename}), thumbnails_url, expires_at, slide_count, content_hash, " +
					"quality, deterministic_ready, publishable, warnings, diagnostics; 422 with ok=false, error and " +
					"source-addressed diagnostics when the spec, template, assets or a quality gate refuses the render.",
			},
			AvailableEndpoints: []string{
				"GET  /api/v1/health",
				"GET  /api/v1/ready",
				"GET  /api/v1/thumbnails/{filename}",
				"GET  /api/v1/capabilities",
				"GET  /api/v1/templates",
				"GET  /api/v1/templates/{name}",
				"GET  /api/v1/slide-types",
				"GET  /api/v1/semantic/schema",
				"GET  /api/v1/semantic/kinds",
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

				"analyze_deck_rhythm (composition analysis)",
				"recommend_visual (layout/pattern recommendation)",
				"plan_deck (deck planning from brief)",
				"repair_slide (fit-report-driven repair)",
			},
			RecommendedInterface: "For decks with charts, diagrams, tables, images, shape grids, " +
				"or named patterns, use the MCP interface (json2pptx mcp) or the CLI " +
				"(json2pptx generate). POST /api/v1/convert supports basic text and bullet slides only; " +
				"POST /api/v1/semantic/render renders full semantic deck specs.",
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
