package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/svggen"
)

// capabilitiesResponse is the JSON output for the get_capabilities tool.
//
// The response carries both the rich json2pptx-specific envelope (runtime,
// changelog_url, mcp_tools_available, deprecated_fields) and the standardized
// cross-server envelope (tool_list, registry, deprecations, vocabularies,
// features) that mirrors svggen-mcp's get_capabilities. Agents can parse
// either shape; the standardized fields enable drift detection across both
// MCP servers without server-specific code paths.
type capabilitiesResponse struct {
	SchemaVersion      string                        `json:"schema_version"`
	ToolVersion        string                        `json:"tool_version"`
	ChangelogURL       string                        `json:"changelog_url"`
	MCPToolsAvailable  []mcpToolEntry                `json:"mcp_tools_available"`
	// ToolList is the cross-server-aligned tool catalog: each entry carries
	// {name, description}. Mirrors svggen-mcp's tool_list. Populated by
	// calling every registered tool's constructor.
	ToolList           []capabilitiesToolListEntry   `json:"tool_list"`
	// Registry groups the canonical names of every registered chart, diagram,
	// and pattern this server can render. Mirrors svggen-mcp's registry block
	// (which leaves patterns empty since it has no pattern engine).
	Registry           capabilitiesRegistry          `json:"registry"`
	DeprecatedFields   []capabilitiesDeprecatedField `json:"deprecated_fields"`
	// Deprecations is the cross-server-aligned deprecation list. Today it is
	// the same content as DeprecatedFields; the alias exists so agents can
	// read a single key name on both MCP servers.
	Deprecations       []capabilitiesDeprecatedField `json:"deprecations"`
	Features           capabilitiesFeatures          `json:"features"`
	Runtime            capabilitiesRuntime           `json:"runtime"`
	Vocabularies       capabilitiesVocabularies      `json:"vocabularies"`
	ErrorCodes         []string                      `json:"error_codes"`
}

// capabilitiesToolListEntry is the cross-server-aligned tool descriptor used
// by the standardized `tool_list` field. Mirrors svggen-mcp's tool_list entry.
type capabilitiesToolListEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// capabilitiesRegistry groups the registered chart, diagram, and pattern
// names so agents can ask one question to discover what the server can
// render. Mirrors svggen-mcp's registry block.
type capabilitiesRegistry struct {
	Charts   []string `json:"charts"`
	Diagrams []string `json:"diagrams"`
	Patterns []string `json:"patterns"`
}

// capabilitiesRuntime exposes environment-dependent state that complements
// the static feature flags: whether write operations are gated on, whether
// render dependencies are installed, which directories the server uses, and
// a schema fingerprint that changes when the contract surface changes.
type capabilitiesRuntime struct {
	SettingsWriteEnabled bool     `json:"settings_write_enabled"`
	RenderAvailable      bool     `json:"render_available"`
	RenderMissingCmds    []string `json:"render_missing_commands"`
	TemplatesDir         string   `json:"templates_dir"`
	OutputDir            string   `json:"output_dir"`
	SchemaFingerprint    string   `json:"schema_fingerprint"`
}

// mcpToolEntry describes an MCP tool with its version metadata.
type mcpToolEntry struct {
	Name    string `json:"name"`
	AddedIn string `json:"added_in"`
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
	StrictFit            []string                   `json:"strict_fit"`
	CompactResponses     bool                       `json:"compact_responses"`
	FitReport            capabilitiesFitReport      `json:"fit_report"`
	StrictUnknownKeys    bool                       `json:"strict_unknown_keys"`
	NamedPatterns        bool                       `json:"named_patterns"`
	TemplateSettings     bool                       `json:"template_settings"`
	SupportsInlineMarkup []string                   `json:"supports_inline_markup"`
	SupportsSpeakerNotes bool                       `json:"supports_speaker_notes"`
	OutputValidation     []string                   `json:"output_validation"`
	Compose              composeFeatureCapabilities `json:"compose"`
	// ComposeEnvelope is a top-level boolean signaling that recommend_visual
	// can return Category=="compose" candidates and that ComposeInput is a
	// supported slide-input envelope. Mirrors the detailed Compose struct so
	// agents can capability-gate without inspecting nested fields.
	ComposeEnvelope      bool                       `json:"compose_envelope"`
	FeatureVersions      map[string]string          `json:"feature_versions"`
}

// mcpToolCatalog returns the full list of MCP tools with version metadata,
// sorted by name. Keep this in sync with the s.AddTool calls in runMCP.
func mcpToolCatalog() []mcpToolEntry {
	entries := []mcpToolEntry{
		// Tools from 1.0.0
		{Name: "generate_presentation", AddedIn: "1.0.0"},
		{Name: "list_templates", AddedIn: "1.0.0"},
		{Name: "get_data_format_hints", AddedIn: "1.0.0"},
		{Name: "get_chart_capabilities", AddedIn: "1.0.0"},
		{Name: "get_diagram_capabilities", AddedIn: "1.0.0"},
		{Name: "validate_input", AddedIn: "1.0.0"},
		// Tools from 2.0.0
		{Name: "recommend_pattern", AddedIn: "2.0.0"},
		{Name: "list_patterns", AddedIn: "2.0.0"},
		{Name: "show_pattern", AddedIn: "2.0.0"},
		{Name: "validate_pattern", AddedIn: "2.0.0"},
		{Name: "expand_pattern", AddedIn: "2.0.0"},
		{Name: "list_icons", AddedIn: "2.0.0"},
		{Name: "table_density_guide", AddedIn: "2.0.0"},
		{Name: "resolve_theme", AddedIn: "2.0.0"},
		{Name: "render_slide_image", AddedIn: "2.0.0"},
		{Name: "render_deck_thumbnails", AddedIn: "2.0.0"},
		{Name: "score_deck", AddedIn: "2.0.0"},
		{Name: "preview_presentation_plan", AddedIn: "2.0.0"},
		{Name: "repair_slide", AddedIn: "2.0.0"},
		{Name: "list_template_settings", AddedIn: "2.0.0"},
		{Name: "register_template_setting", AddedIn: "2.0.0"},
		{Name: "delete_template_setting", AddedIn: "2.0.0"},
		{Name: "get_capabilities", AddedIn: "2.0.0"},
		// Tools from 2.4.0
		{Name: "get_shape_catalog", AddedIn: "2.4.0"},
		// Tools from 2.8.0
		{Name: "read_presentation", AddedIn: "2.8.0"},
		// Tools from 3.1.0
		{Name: "analyze_deck_rhythm", AddedIn: "3.1.0"},
		{Name: "plan_deck", AddedIn: "3.1.0"},
		{Name: "recommend_visual", AddedIn: "3.1.0"},
		// Tools from 4.2.0
		{Name: "get_input_schema", AddedIn: "4.2.0"},
		// Tools from 4.6.0
		{Name: "validate_presentation_output", AddedIn: "4.6.0"},
		// Tools from 4.7.0
		{Name: "inspect_slide_images", AddedIn: "4.7.0"},
		// Tools from 4.8.0
		{Name: "expand_patterns", AddedIn: "4.8.0"},
		// Tools from 4.9.0
		{Name: "score_candidates", AddedIn: "4.9.0"},
		// Tools from 4.11.0
		{Name: "get_started", AddedIn: "4.11.0"},
		// Tools from 4.19.0
		{Name: "render_slide_image_from_json", AddedIn: "4.19.0"},
		// Tools from 4.20.0
		{Name: "preview_slide_wireframe", AddedIn: "4.20.0"},
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

// mcpToolNames returns the sorted list of all registered MCP tool names.
// Keep this in sync with the s.AddTool calls in runMCP.
func mcpToolNames() []string {
	catalog := mcpToolCatalog()
	names := make([]string, len(catalog))
	for i, e := range catalog {
		names[i] = e.Name
	}
	return names
}

// buildDeprecatedFields returns deprecated JSON input fields with their
// replacements. This is the structured version of buildDeprecations().
func buildDeprecatedFields() []capabilitiesDeprecatedField {
	return []capabilitiesDeprecatedField{
		{
			Path:        "slides[].content[].value",
			Replacement: "Use typed field: text_value, bullets_value, table_value, chart_value, diagram_value, image_value, body_and_bullets_value, or bullet_groups_value",
			RemovedIn:   "3.0.0",
		},
		{
			Path:        "slides[].content[].placeholder (raw OOXML name)",
			Replacement: "Use portable placeholder_id: title, subtitle, body, body_2",
			RemovedIn:   "3.0.0",
		},
		{
			Path:        "MCP parameter: json_input (string form)",
			Replacement: "Removed. Use presentation (object form) on generate_presentation, validate_input, repair_slide, preview_presentation_plan, score_deck",
			RemovedIn:   "2.10.0",
		},
		{
			Path:        "MCP parameter: values/overrides/cell_overrides/callout (string forms) and values_object/overrides_object/cell_overrides_object/callout_object (_object suffix)",
			Replacement: "Removed. Use object parameters: values, overrides, cell_overrides, callout on validate_pattern, expand_pattern",
			RemovedIn:   "2.10.0",
		},
	}
}

func mcpGetCapabilitiesTool() mcp.Tool {
	return mcp.NewTool("get_capabilities",
		mcp.WithDescription("Returns schema version, available MCP tools, deprecated fields, and feature flags. Use this to detect contract drift between sessions without re-reading SKILL.md. Compare schema_version across sessions — a major bump means breaking changes."),
		mcp.WithRawOutputSchema(outputSchemaGetCapabilities),
	)
}

func (mc *mcpConfig) handleGetCapabilities(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return buildCapabilitiesResult(ctx, mc.templatesDir, mc.outputDir)
}

// buildCapabilitiesResult constructs the full capabilities response. It is
// shared by the MCP handler and the CLI subcommand.
func buildCapabilitiesResult(ctx context.Context, templatesDir, outputDir string) (*mcp.CallToolResult, error) {
	codes := diagnostics.AllCodes()
	sort.Strings(codes)

	renderAvail, renderMissing := render.DependencyStatus()

	// Resolve the actual templates directory path for the runtime section.
	resolvedTemplatesDir := templatesDir
	if resolvedTemplatesDir != "" {
		dir, embedded := resolveTemplatesDir(resolvedTemplatesDir)
		if embedded {
			resolvedTemplatesDir = "(embedded)"
		} else {
			resolvedTemplatesDir = dir
		}
	}

	deprecations := buildDeprecatedFields()
	resp := capabilitiesResponse{
		SchemaVersion:     SchemaVersion,
		ToolVersion:       Version,
		ChangelogURL:      "docs/SCHEMA_CHANGELOG.md",
		MCPToolsAvailable: mcpToolCatalog(),
		ToolList:          buildToolList(),
		Registry:          buildRegistry(),
		DeprecatedFields:  deprecations,
		Deprecations:      deprecations,
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
			OutputValidation:     []string{"off", "warn", "strict"},
			Compose:              composeCapabilities(),
			ComposeEnvelope:      true,
			FeatureVersions: map[string]string{
				"strict_fit":             "2.0.0",
				"compact_responses":      "2.0.0",
				"fit_report":             "2.0.0",
				"strict_unknown_keys":    "2.0.0",
				"named_patterns":         "2.0.0",
				"template_settings":      "2.0.0",
				"supports_inline_markup": "2.5.0",
				"supports_speaker_notes": "2.5.0",
				"output_validation":      "4.6.0",
				"compose":                "4.10.0",
				"compose_envelope":       "4.11.0",
			},
		},
		Runtime: capabilitiesRuntime{
			SettingsWriteEnabled: settingsWriteAllowed(),
			RenderAvailable:      renderAvail,
			RenderMissingCmds:    renderMissing,
			TemplatesDir:         resolvedTemplatesDir,
			OutputDir:            outputDir,
			SchemaFingerprint:    schemaFingerprint(),
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
		"add_items",
		"autofix_visual",
		"provide_value",
		"reduce_cell_text",
		"reduce_items",
		"reduce_text",
		"remove_field",
		"remove_key",
		"rename_field",
		"replace_color",
		"replace_value",
		"reshape_grid",
		"reshape_value",
		"resize_list",
		"set_pattern_style",
		"shorten_title",
		"split_at_row",
		"split_pattern",
		"swap_layout",
		"swap_pattern",
		"use_one_of",
		"use_semantic_color",
	}
}

// deprecationWarnings scans a parsed PresentationInput for usage of deprecated
// fields and returns human-readable warning strings. Agents can use these to
// migrate decks before the removal version.
func deprecationWarnings(input *PresentationInput) []string {
	var warnings []string
	for i, slide := range input.Slides {
		for j, ci := range slide.Content {
			if ci.UsesLegacyValue() {
				warnings = append(warnings, fmt.Sprintf(
					"slides[%d].content[%d]: uses deprecated 'value' field (removed in 3.0.0) — use typed field (%s_value) instead",
					i, j, ci.Type,
				))
			}
		}
	}
	return warnings
}

// buildVocabularies constructs the vocabularies section from authoritative sources.
func buildVocabularies() capabilitiesVocabularies {
	// Chart types from svggen registry.
	chartTypes := svggen.Types()
	sort.Strings(chartTypes)

	// Diagram types from svggen capabilities (ready only — stubs excluded).
	diagCaps := svggen.DiagramCapabilitiesReady()
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

// toolConstructors returns every registered MCP tool's constructor keyed by
// the tool name returned by that constructor. Keep this map in lockstep with
// the s.AddTool calls in runMCP and the entries in mcpToolCatalog —
// TestMCPToolList_MatchesCatalog enforces the parity.
func toolConstructors() map[string]func() mcp.Tool {
	ctors := []func() mcp.Tool{
		mcpGenerateTool,
		mcpListTemplatesTool,
		mcpGetDataFormatHintsTool,
		mcpGetChartCapabilitiesTool,
		mcpGetDiagramCapabilitiesTool,
		mcpValidateTool,
		mcpRecommendPatternTool,
		mcpRecommendVisualTool,
		mcpListPatternsTool,
		mcpShowPatternTool,
		mcpValidatePatternTool,
		mcpExpandPatternTool,
		mcpExpandPatternsTool,
		mcpListIconsTool,
		mcpGetShapeCatalogTool,
		mcpTableDensityGuideTool,
		mcpResolveThemeTool,
		mcpRenderSlideImageTool,
		mcpRenderSlideImageFromJSONTool,
		mcpRenderDeckThumbnailsTool,
		mcpScoreDeckTool,
		mcpScoreCandidatesTool,
		mcpInspectSlideImagesTool,
		mcpPreviewPlanTool,
		mcpPreviewSlideWireframeTool,
		mcpRepairSlideTool,
		mcpListTemplateSettingsTool,
		mcpRegisterTemplateSettingTool,
		mcpDeleteTemplateSettingTool,
		mcpAnalyzeDeckRhythmTool,
		mcpPlanDeckTool,
		mcpGetCapabilitiesTool,
		mcpGetStartedTool,
		mcpGetInputSchemaTool,
		mcpReadPresentationTool,
		mcpValidateOutputTool,
	}
	out := make(map[string]func() mcp.Tool, len(ctors))
	for _, c := range ctors {
		t := c()
		out[t.Name] = c
	}
	return out
}

// buildToolList returns the standardized {name, description} catalog used by
// the cross-server-aligned tool_list field. Order mirrors mcpToolCatalog (by
// name) so output is deterministic.
func buildToolList() []capabilitiesToolListEntry {
	ctors := toolConstructors()
	catalog := mcpToolCatalog()
	out := make([]capabilitiesToolListEntry, 0, len(catalog))
	for _, e := range catalog {
		c, ok := ctors[e.Name]
		if !ok {
			// Defensive: a name in the catalog without a constructor is a
			// programmer error. Surface it as an empty description rather
			// than dropping the entry so the missing tool is visible.
			out = append(out, capabilitiesToolListEntry{Name: e.Name})
			continue
		}
		t := c()
		out = append(out, capabilitiesToolListEntry{
			Name:        t.Name,
			Description: t.Description,
		})
	}
	return out
}

// buildRegistry returns the standardized chart/diagram/pattern registry used
// by the cross-server-aligned registry field. Pulls from the same sources
// as buildVocabularies so the two cannot drift.
func buildRegistry() capabilitiesRegistry {
	voc := buildVocabularies()
	return capabilitiesRegistry{
		Charts:   voc.ChartTypes,
		Diagrams: voc.DiagramTypes,
		Patterns: voc.PatternNames,
	}
}
