package main

import (
	"context"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/svggen"
)

// capabilitiesResponse is the JSON output for the get_capabilities tool.
type capabilitiesResponse struct {
	SchemaVersion      string                       `json:"schema_version"`
	ToolVersion        string                       `json:"tool_version"`
	MCPToolsAvailable  []string                     `json:"mcp_tools_available"`
	DeprecatedFields   []capabilitiesDeprecatedField `json:"deprecated_fields"`
	Features           capabilitiesFeatures          `json:"features"`
	Vocabularies       capabilitiesVocabularies      `json:"vocabularies"`
	ErrorCodes         []string                     `json:"error_codes"`
}

// capabilitiesVocabularies exposes categorical enums and vocabularies so agents
// can discover valid values programmatically instead of parsing tool descriptions.
type capabilitiesVocabularies struct {
	RepairFixKinds       []string            `json:"repair_fix_kinds"`
	FitFindingCodes      []string            `json:"fit_finding_codes"`
	ContentTypes         []string            `json:"content_types"`
	SlideTransitions     []string            `json:"slide_transitions"`
	TransitionSpeeds     []string            `json:"transition_speeds"`
	BuildAnimations      []string            `json:"build_animations"`
	ChartTypes           []string            `json:"chart_types"`
	DiagramTypes         []string            `json:"diagram_types"`
	PlaceholderAliases   map[string][]string `json:"placeholder_aliases"`
	PatternNames         []string            `json:"pattern_names"`
	PatternAliases       map[string]string   `json:"pattern_aliases"`
}

// capabilitiesDeprecatedField describes a deprecated JSON input field.
type capabilitiesDeprecatedField struct {
	Path        string `json:"path"`
	Replacement string `json:"replacement"`
	RemovedIn   string `json:"removed_in,omitempty"`
}

// capabilitiesFitReport describes fit_report support and per-tool defaults.
type capabilitiesFitReport struct {
	Supported bool            `json:"supported"`
	DefaultIn map[string]bool `json:"default_in"`
}

// capabilitiesFeatures describes feature flags the server supports.
type capabilitiesFeatures struct {
	StrictFit            []string              `json:"strict_fit"`
	CompactResponses     bool                  `json:"compact_responses"`
	FitReport            capabilitiesFitReport `json:"fit_report"`
	StrictUnknownKeys    bool                  `json:"strict_unknown_keys"`
	NamedPatterns        bool                  `json:"named_patterns"`
	TemplateSettings     bool                  `json:"template_settings"`
	SupportsInlineMarkup []string              `json:"supports_inline_markup"`
	SupportsSpeakerNotes bool                  `json:"supports_speaker_notes"`
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
		mcp.WithRawOutputSchema(outputSchemaGetCapabilities),
	)
}

func handleGetCapabilities(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	codes := diagnostics.AllCodes()
	sort.Strings(codes)

	resp := capabilitiesResponse{
		SchemaVersion:     SchemaVersion,
		ToolVersion:       Version,
		MCPToolsAvailable: mcpToolNames(),
		DeprecatedFields:  buildDeprecatedFields(),
		Features: capabilitiesFeatures{
			StrictFit:        []string{"off", "warn", "strict"},
			CompactResponses: true,
			FitReport: capabilitiesFitReport{
				Supported: true,
				DefaultIn: map[string]bool{
					"validate_input":            true,
					"preview_presentation_plan": true,
					"generate_presentation":     false,
				},
			},
			StrictUnknownKeys:    true,
			NamedPatterns:        true,
			TemplateSettings:     true,
			SupportsInlineMarkup: []string{"b", "i", "u"},
			SupportsSpeakerNotes: true,
		},
		Vocabularies: buildVocabularies(),
		ErrorCodes:   codes,
	}

	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", "failed to marshal capabilities response"), nil
	}
	return mcpResult, nil
}

// repairFixKinds returns the sorted list of fix kinds supported by applyRepairFix.
// This is derived from the switch statement in mcp_repair.go to stay in sync.
func repairFixKinds() []string {
	return []string{
		"reduce_text",
		"replace_color",
		"shorten_title",
		"split_at_row",
		"swap_layout",
		"use_one_of",
		"use_semantic_color",
	}
}

// buildVocabularies constructs the vocabularies section from authoritative sources.
func buildVocabularies() capabilitiesVocabularies {
	// Chart types from svggen registry.
	chartTypes := svggen.Types()
	sort.Strings(chartTypes)

	// Diagram types from svggen capabilities.
	diagCaps := svggen.DiagramCapabilities()
	diagramTypes := make([]string, 0, len(diagCaps))
	for _, d := range diagCaps {
		diagramTypes = append(diagramTypes, d.Type)
	}
	sort.Strings(diagramTypes)

	// Pattern names and aliases from the default registry.
	reg := patterns.Default()
	patternList := reg.List()
	patternNames := make([]string, 0, len(patternList))
	for _, p := range patternList {
		patternNames = append(patternNames, p.Name())
	}

	// Placeholder aliases grouped by portable name.
	placeholderAliases := map[string][]string{
		"title":    {"Title 1", "Title"},
		"subtitle": {"Subtitle 2", "Subtitle"},
		"body":     {"Content Placeholder 1", "Text Placeholder 1"},
		"body_2":   {"Content Placeholder 2", "Text Placeholder 2"},
		"body_3":   {"Content Placeholder 3", "Text Placeholder 3"},
	}

	return capabilitiesVocabularies{
		RepairFixKinds:     repairFixKinds(),
		FitFindingCodes:    patterns.AllFitFindingCodes(),
		ContentTypes:       generator.AllContentTypes(),
		SlideTransitions:   generator.ValidTransitionNames(),
		TransitionSpeeds:   []string{"slow", "medium", "fast"},
		BuildAnimations:    []string{"bullets"},
		ChartTypes:         chartTypes,
		DiagramTypes:       diagramTypes,
		PlaceholderAliases: placeholderAliases,
		PatternNames:       patternNames,
		PatternAliases:     reg.Aliases(),
	}
}
