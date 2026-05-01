package main

import (
	"context"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
)

// capabilitiesResponse is the JSON output for the get_capabilities tool.
type capabilitiesResponse struct {
	SchemaVersion      string                       `json:"schema_version"`
	ToolVersion        string                       `json:"tool_version"`
	MCPToolsAvailable  []string                     `json:"mcp_tools_available"`
	DeprecatedFields   []capabilitiesDeprecatedField `json:"deprecated_fields"`
	Features           capabilitiesFeatures          `json:"features"`
}

// capabilitiesDeprecatedField describes a deprecated JSON input field.
type capabilitiesDeprecatedField struct {
	Path        string `json:"path"`
	Replacement string `json:"replacement"`
	RemovedIn   string `json:"removed_in,omitempty"`
}

// capabilitiesFeatures describes feature flags the server supports.
type capabilitiesFeatures struct {
	StrictFit        []string `json:"strict_fit"`
	CompactResponses bool     `json:"compact_responses"`
	FitReport        bool     `json:"fit_report"`
	StrictUnknownKeys bool   `json:"strict_unknown_keys"`
	NamedPatterns    bool     `json:"named_patterns"`
	TemplateSettings bool     `json:"template_settings"`
}

// mcpToolNames returns the sorted list of all registered MCP tool names.
// Keep this in sync with the s.AddTool calls in runMCP.
func mcpToolNames() []string {
	names := []string{
		"generate_presentation",
		"list_templates",
		"get_data_format_hints",
		"get_chart_capabilities",
		"get_diagram_capabilities",
		"validate_input",
		"recommend_pattern",
		"list_patterns",
		"show_pattern",
		"validate_pattern",
		"expand_pattern",
		"list_icons",
		"table_density_guide",
		"resolve_theme",
		"render_slide_image",
		"render_deck_thumbnails",
		"score_deck",
		"preview_presentation_plan",
		"repair_slide",
		"list_template_settings",
		"register_template_setting",
		"delete_template_setting",
		"get_capabilities",
	}
	sort.Strings(names)
	return names
}

// buildDeprecatedFields returns deprecated JSON input fields with their
// replacements. This is the structured version of buildDeprecations().
func buildDeprecatedFields() []capabilitiesDeprecatedField {
	return []capabilitiesDeprecatedField{
		{
			Path:        "slides[].content[].value",
			Replacement: "Use typed field: text_value, bullets_value, table_value, chart_value, diagram_value, image_value, body_and_bullets_value, or bullet_groups_value",
		},
		{
			Path:        "slides[].content[].placeholder (raw OOXML name)",
			Replacement: "Use portable placeholder_id: title, subtitle, body, body_2",
		},
	}
}

func mcpGetCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_capabilities",
		mcp.WithDescription("Returns schema version, available MCP tools, deprecated fields, and feature flags. Use this to detect contract drift between sessions without re-reading SKILL.md. Compare schema_version across sessions — a major bump means breaking changes."),
	)
}

func handleGetCapabilities(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := capabilitiesResponse{
		SchemaVersion:     SchemaVersion,
		ToolVersion:       Version,
		MCPToolsAvailable: mcpToolNames(),
		DeprecatedFields:  buildDeprecatedFields(),
		Features: capabilitiesFeatures{
			StrictFit:         []string{"off", "warn", "strict"},
			CompactResponses:  true,
			FitReport:         true,
			StrictUnknownKeys: true,
			NamedPatterns:     true,
			TemplateSettings:  true,
		},
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", "failed to marshal capabilities response"), nil
	}
	return mcpResult, nil
}
