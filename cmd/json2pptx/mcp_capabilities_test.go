package main

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// getCapabilitiesResult is a test helper that calls buildCapabilitiesResult
// with empty dirs (CLI-like invocation).
func getCapabilitiesResult(t *testing.T) capabilitiesResponse {
	t.Helper()
	result, err := buildCapabilitiesResult(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var resp capabilitiesResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return resp
}

func TestMCPGetCapabilities(t *testing.T) {
	t.Run("returns valid capabilities", func(t *testing.T) {
		resp := getCapabilitiesResult(t)

		if resp.SchemaVersion == "" {
			t.Error("expected non-empty schema_version")
		}
		if resp.ToolVersion == "" {
			t.Error("expected non-empty tool_version")
		}
		if len(resp.MCPToolsAvailable) == 0 {
			t.Error("expected non-empty mcp_tools_available")
		}
		if len(resp.DeprecatedFields) == 0 {
			t.Error("expected non-empty deprecated_fields")
		}
	})

	t.Run("schema_version matches constant", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		if resp.SchemaVersion != SchemaVersion {
			t.Errorf("schema_version mismatch: got %q, want %q", resp.SchemaVersion, SchemaVersion)
		}
	})

	t.Run("mcp_tools_available is sorted", func(t *testing.T) {
		names := mcpToolNames()
		for i := 1; i < len(names); i++ {
			if names[i] < names[i-1] {
				t.Errorf("mcp_tools_available not sorted: %q before %q", names[i-1], names[i])
			}
		}
	})

	t.Run("mcp_tools_available includes get_capabilities itself", func(t *testing.T) {
		names := mcpToolNames()
		found := false
		for _, n := range names {
			if n == "get_capabilities" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected get_capabilities in mcp_tools_available")
		}
	})

	t.Run("features has strict_fit ladder", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		if len(resp.Features.StrictFit) != 3 {
			t.Errorf("expected 3 strict_fit levels, got %d", len(resp.Features.StrictFit))
		}
	})

	t.Run("vocabularies are populated", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		v := resp.Vocabularies
		if len(v.RepairFixKinds) == 0 {
			t.Error("expected non-empty repair_fix_kinds")
		}
		if len(v.FitFindingCodes) == 0 {
			t.Error("expected non-empty fit_finding_codes")
		}
		if len(v.ContentTypes) == 0 {
			t.Error("expected non-empty content_types")
		}
		if len(v.SlideTransitions) == 0 {
			t.Error("expected non-empty slide_transitions")
		}
		if len(v.ChartTypes) == 0 {
			t.Error("expected non-empty chart_types")
		}
		if len(v.DiagramTypes) == 0 {
			t.Error("expected non-empty diagram_types")
		}
		if len(v.PlaceholderAliases) == 0 {
			t.Error("expected non-empty placeholder_aliases")
		}
		if len(v.PatternNames) == 0 {
			t.Error("expected non-empty pattern_names")
		}
	})

	t.Run("changelog_url is set", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		if resp.ChangelogURL == "" {
			t.Error("expected non-empty changelog_url")
		}
	})

	t.Run("every tool has added_in", func(t *testing.T) {
		catalog := mcpToolCatalog()
		for _, entry := range catalog {
			if entry.AddedIn == "" {
				t.Errorf("tool %q missing added_in", entry.Name)
			}
		}
	})

	t.Run("every deprecation has removed_in", func(t *testing.T) {
		fields := buildDeprecatedFields()
		for _, f := range fields {
			if f.RemovedIn == "" {
				t.Errorf("deprecated field %q missing removed_in", f.Path)
			}
		}
	})

	t.Run("feature_versions is populated", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		if len(resp.Features.FeatureVersions) == 0 {
			t.Error("expected non-empty feature_versions")
		}
		// Every feature version must be a semver-like string.
		for k, v := range resp.Features.FeatureVersions {
			if v == "" {
				t.Errorf("feature_versions[%q] is empty", k)
			}
		}
	})

	t.Run("features.compose advertises the enforced cap", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		c := resp.Features.Compose
		if c.MaxSegments != composeMaxSegments {
			t.Errorf("compose.max_segments = %d, want %d (same as enforced cap)", c.MaxSegments, composeMaxSegments)
		}
		if len(c.Directions) == 0 {
			t.Error("compose.directions should be non-empty")
		}
		// Directions must match the validator's accepted set.
		seenVert, seenHoriz := false, false
		for _, d := range c.Directions {
			switch d {
			case "vertical":
				seenVert = true
			case "horizontal":
				seenHoriz = true
			}
		}
		if !seenVert || !seenHoriz {
			t.Errorf("compose.directions missing vertical/horizontal: %v", c.Directions)
		}
		if !c.SupportsSmartCompose {
			t.Error("compose.supports_smart_compose should be true")
		}
	})

	t.Run("features.compose_envelope flag advertised", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		if !resp.Features.ComposeEnvelope {
			t.Error("features.compose_envelope should be true so agents can capability-gate compose without inspecting the nested compose struct")
		}
		if _, ok := resp.Features.FeatureVersions["compose_envelope"]; !ok {
			t.Error("feature_versions should include compose_envelope")
		}
	})

	t.Run("runtime section is populated", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		rt := resp.Runtime

		// schema_fingerprint must be a non-empty hex digest prefix.
		if rt.SchemaFingerprint == "" {
			t.Error("expected non-empty schema_fingerprint")
		}
		if len(rt.SchemaFingerprint) != 16 {
			t.Errorf("expected 16-char schema_fingerprint, got %d chars: %q", len(rt.SchemaFingerprint), rt.SchemaFingerprint)
		}

		// render_missing_commands should be nil/empty when render_available is true,
		// or non-empty when render_available is false.
		if rt.RenderAvailable && len(rt.RenderMissingCmds) > 0 {
			t.Errorf("render_available=true but render_missing_commands=%v", rt.RenderMissingCmds)
		}
		if !rt.RenderAvailable && len(rt.RenderMissingCmds) == 0 {
			t.Error("render_available=false but render_missing_commands is empty")
		}
	})

	t.Run("settings_write_enabled reflects env", func(t *testing.T) {
		// Without the env var set, should be false.
		resp := getCapabilitiesResult(t)
		if resp.Runtime.SettingsWriteEnabled {
			t.Log("settings_write_enabled=true (env var is set in test environment)")
		}

		// With the env var set, should be true.
		t.Setenv("JSON2PPTX_ALLOW_SETTINGS_WRITE", "1")
		resp = getCapabilitiesResult(t)
		if !resp.Runtime.SettingsWriteEnabled {
			t.Error("expected settings_write_enabled=true when JSON2PPTX_ALLOW_SETTINGS_WRITE=1")
		}
	})

	t.Run("standardized tool_list matches catalog", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		// tool_list must include every name in mcp_tools_available, with
		// non-empty descriptions, and contain no extras.
		catalogNames := map[string]bool{}
		for _, e := range resp.MCPToolsAvailable {
			catalogNames[e.Name] = true
		}
		toolListNames := map[string]bool{}
		for _, e := range resp.ToolList {
			if e.Name == "" {
				t.Errorf("tool_list entry has empty name: %+v", e)
			}
			if e.Description == "" {
				t.Errorf("tool_list entry %q missing description (constructor must set mcp.WithDescription)", e.Name)
			}
			toolListNames[e.Name] = true
		}
		for name := range catalogNames {
			if !toolListNames[name] {
				t.Errorf("tool_list missing entry for %q (present in mcp_tools_available)", name)
			}
		}
		for name := range toolListNames {
			if !catalogNames[name] {
				t.Errorf("tool_list has %q but it is missing from mcp_tools_available catalog", name)
			}
		}
	})

	t.Run("standardized registry mirrors vocabularies", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		// registry must agree with vocabularies on the canonical name lists.
		if len(resp.Registry.Charts) != len(resp.Vocabularies.ChartTypes) {
			t.Errorf("registry.charts size %d != vocabularies.chart_types size %d",
				len(resp.Registry.Charts), len(resp.Vocabularies.ChartTypes))
		}
		if len(resp.Registry.Diagrams) != len(resp.Vocabularies.DiagramTypes) {
			t.Errorf("registry.diagrams size %d != vocabularies.diagram_types size %d",
				len(resp.Registry.Diagrams), len(resp.Vocabularies.DiagramTypes))
		}
		if len(resp.Registry.Patterns) != len(resp.Vocabularies.PatternNames) {
			t.Errorf("registry.patterns size %d != vocabularies.pattern_names size %d",
				len(resp.Registry.Patterns), len(resp.Vocabularies.PatternNames))
		}
	})

	t.Run("deprecations alias mirrors deprecated_fields", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		if len(resp.Deprecations) != len(resp.DeprecatedFields) {
			t.Errorf("deprecations size %d != deprecated_fields size %d (must be the same list)",
				len(resp.Deprecations), len(resp.DeprecatedFields))
		}
		for i := range resp.Deprecations {
			if i >= len(resp.DeprecatedFields) {
				break
			}
			if resp.Deprecations[i].Path != resp.DeprecatedFields[i].Path {
				t.Errorf("deprecations[%d].path=%q != deprecated_fields[%d].path=%q",
					i, resp.Deprecations[i].Path, i, resp.DeprecatedFields[i].Path)
			}
		}
	})

	t.Run("deck chrome / structure feature flags advertised", func(t *testing.T) {
		resp := getCapabilitiesResult(t)
		flags := map[string]capabilitiesFeatureFlag{
			"deck_chrome":       resp.Features.DeckChrome,
			"page_numbers":      resp.Features.PageNumbers,
			"section_structure": resp.Features.SectionStructure,
			"section_crumb":     resp.Features.SectionCrumb,
		}
		for name, f := range flags {
			if !f.Supported {
				t.Errorf("features.%s.supported should be true", name)
			}
			if f.Version == "" {
				t.Errorf("features.%s.version is empty — agents need a version to capability-gate against schema_version drift", name)
			}
			if f.UsageHint == "" {
				t.Errorf("features.%s.usage_hint is empty — agents need a one-line authoring hint pointing at the JSON field that turns this on", name)
			}
			if _, ok := resp.Features.FeatureVersions[name]; !ok {
				t.Errorf("feature_versions missing %q so the version is only advertised in the nested struct — duplicate it to feature_versions so agents using the flat lookup also see it", name)
			}
		}
		// The usage hints must actually name the relevant JSON field so an
		// agent staying within MCP discovery can author the input without
		// reading SKILL.md.
		if !strings.Contains(resp.Features.DeckChrome.UsageHint, "chrome") {
			t.Errorf("deck_chrome.usage_hint should mention the chrome JSON field; got %q", resp.Features.DeckChrome.UsageHint)
		}
		if !strings.Contains(resp.Features.PageNumbers.UsageHint, "page_numbers") {
			t.Errorf("page_numbers.usage_hint should mention the chrome.page_numbers JSON field; got %q", resp.Features.PageNumbers.UsageHint)
		}
		if !strings.Contains(resp.Features.SectionStructure.UsageHint, "structure") {
			t.Errorf("section_structure.usage_hint should mention the structure JSON field; got %q", resp.Features.SectionStructure.UsageHint)
		}
		if !strings.Contains(resp.Features.SectionCrumb.UsageHint, "section_crumb") {
			t.Errorf("section_crumb.usage_hint should mention chrome.section_crumb; got %q", resp.Features.SectionCrumb.UsageHint)
		}
	})

	t.Run("repair_fix_kinds matches applyRepairFix switch cases", func(t *testing.T) {
		// Parse mcp_repair.go and extract case labels from the applyRepairFix switch.
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "mcp_repair.go", nil, 0)
		if err != nil {
			t.Fatalf("failed to parse mcp_repair.go: %v", err)
		}

		var switchCases []string
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "applyRepairFix" {
				return true
			}
			// Walk the function body to find the switch statement.
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				cc, ok := n.(*ast.CaseClause)
				if !ok || cc.List == nil {
					return true
				}
				for _, expr := range cc.List {
					lit, ok := expr.(*ast.BasicLit)
					if ok && lit.Kind == token.STRING {
						// Strip quotes.
						switchCases = append(switchCases, lit.Value[1:len(lit.Value)-1])
					}
				}
				return true
			})
			return false
		})

		sort.Strings(switchCases)
		advertised := repairFixKinds()

		if len(switchCases) != len(advertised) {
			t.Fatalf("applyRepairFix has %d cases %v but repairFixKinds advertises %d %v",
				len(switchCases), switchCases, len(advertised), advertised)
		}
		for i := range switchCases {
			if switchCases[i] != advertised[i] {
				t.Errorf("mismatch at index %d: switch has %q, advertised has %q", i, switchCases[i], advertised[i])
			}
		}
	})
}

// TestMCPToolCatalog_MatchesRegisteredTools parses the s.AddTool calls in mcp.go
// and verifies every registered tool appears in mcpToolCatalog (and vice versa).
func TestMCPToolCatalog_MatchesRegisteredTools(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "mcp.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse mcp.go: %v", err)
	}

	// Extract tool names from the mcpXxxTool() constructor calls inside runMCP.
	// Each s.AddTool(mcpFooTool(), ...) call has a first arg that is a call to
	// a constructor returning a tool with a specific name. We extract names by
	// looking at the first argument of AddTool calls in the runMCP function.
	registered := make(map[string]bool)

	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "registerMCPTools" {
			return true
		}
		// Walk the function body to find s.AddTool calls.
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "AddTool" {
				return true
			}
			// The first argument should be a call to a tool constructor.
			if len(call.Args) < 1 {
				return true
			}
			// Call the constructor at runtime to get the actual tool name.
			// Instead, we collect the constructor function names and map them.
			return true
		})
		return false
	})

	// Alternative approach: use the tool constructors test table which is
	// maintained and lists every constructor. We just need to verify that
	// mcpToolCatalog covers the same set.
	// Parse the tool names from AddTool first arg constructor calls.
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "registerMCPTools" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "AddTool" {
				return true
			}
			if len(call.Args) < 1 {
				return true
			}
			// First arg is a function call — get its name.
			argCall, ok := call.Args[0].(*ast.CallExpr)
			if !ok {
				return true
			}
			var funcName string
			switch fn := argCall.Fun.(type) {
			case *ast.Ident:
				funcName = fn.Name
			case *ast.SelectorExpr:
				funcName = fn.Sel.Name
			}
			if funcName != "" {
				registered[funcName] = true
			}
			return true
		})
		return false
	})

	// Now map constructor function names to tool names by calling them.
	constructors := map[string]func() mcp.Tool{
		"mcpGenerateTool":              mcpGenerateTool,
		"mcpListTemplatesTool":         mcpListTemplatesTool,
		"mcpGetDataFormatHintsTool":    mcpGetDataFormatHintsTool,
		"mcpGetChartCapabilitiesTool":  mcpGetChartCapabilitiesTool,
		"mcpGetDiagramCapabilitiesTool": mcpGetDiagramCapabilitiesTool,
		"mcpValidateTool":             mcpValidateTool,
		"mcpRecommendPatternTool":     mcpRecommendPatternTool,
		"mcpRecommendVisualTool":      mcpRecommendVisualTool,
		"mcpListPatternsTool":         mcpListPatternsTool,
		"mcpShowPatternTool":          mcpShowPatternTool,
		"mcpValidatePatternTool":      mcpValidatePatternTool,
		"mcpExpandPatternTool":        mcpExpandPatternTool,
		"mcpExpandPatternsTool":       mcpExpandPatternsTool,
		"mcpListIconsTool":            mcpListIconsTool,
		"mcpGetShapeCatalogTool":      mcpGetShapeCatalogTool,
		"mcpTableDensityGuideTool":    mcpTableDensityGuideTool,
		"mcpResolveThemeTool":         mcpResolveThemeTool,
		"mcpRenderSlideImageTool":           mcpRenderSlideImageTool,
		"mcpRenderSlideImageFromJSONTool":   mcpRenderSlideImageFromJSONTool,
		"mcpRenderDeckThumbnailsTool":       mcpRenderDeckThumbnailsTool,
		"mcpScoreDeckTool":            mcpScoreDeckTool,
		"mcpScoreCandidatesTool":      mcpScoreCandidatesTool,
		"mcpInspectSlideImagesTool":   mcpInspectSlideImagesTool,
		"mcpPreviewPlanTool":          mcpPreviewPlanTool,
		"mcpPreviewSlideWireframeTool": mcpPreviewSlideWireframeTool,
		"mcpRepairSlideTool":          mcpRepairSlideTool,
		"mcpRepairSlidesBatchTool":    mcpRepairSlidesBatchTool,
		"mcpProposeRepairsTool":       mcpProposeRepairsTool,
		"mcpListTemplateSettingsTool":     mcpListTemplateSettingsTool,
		"mcpRegisterTemplateSettingTool":  mcpRegisterTemplateSettingTool,
		"mcpDeleteTemplateSettingTool":    mcpDeleteTemplateSettingTool,
		"mcpAnalyzeDeckRhythmTool":       mcpAnalyzeDeckRhythmTool,
		"mcpPlanDeckTool":                mcpPlanDeckTool,
		"mcpGetCapabilitiesTool":         mcpGetCapabilitiesTool,
		"mcpGetStartedTool":              mcpGetStartedTool,
		"mcpGetInputSchemaTool":          mcpGetInputSchemaTool,
		"mcpReadPresentationTool":        mcpReadPresentationTool,
		"mcpValidateOutputTool":          mcpValidateOutputTool,
		"mcpDescribeFindingTool":         mcpDescribeFindingTool,
		"mcpPreviewIconTool":             mcpPreviewIconTool,
	}

	// Build the set of tool names that are actually registered in runMCP.
	registeredToolNames := make(map[string]bool)
	for funcName := range registered {
		ctor, ok := constructors[funcName]
		if !ok {
			t.Errorf("runMCP registers %q but no constructor mapping exists in test", funcName)
			continue
		}
		tool := ctor()
		registeredToolNames[tool.Name] = true
	}

	// Compare against catalog.
	catalogNames := make(map[string]bool)
	for _, name := range mcpToolNames() {
		catalogNames[name] = true
	}

	for name := range registeredToolNames {
		if !catalogNames[name] {
			t.Errorf("tool %q is registered in runMCP but missing from mcpToolCatalog", name)
		}
	}
	for name := range catalogNames {
		if !registeredToolNames[name] {
			t.Errorf("tool %q is in mcpToolCatalog but not registered in runMCP", name)
		}
	}
}

func TestDeprecationWarnings(t *testing.T) {
	t.Run("fires on legacy value field", func(t *testing.T) {
		input := &PresentationInput{
			Slides: []SlideInput{
				{
					Content: []ContentInput{
						{PlaceholderID: "body", Type: "text", Value: json.RawMessage(`"hello"`)},
					},
				},
			},
		}
		warnings := deprecationWarnings(input)
		if len(warnings) == 0 {
			t.Error("expected deprecation warning for legacy value usage")
		}
	})

	t.Run("silent on typed fields", func(t *testing.T) {
		text := "hello"
		input := &PresentationInput{
			Slides: []SlideInput{
				{
					Content: []ContentInput{
						{PlaceholderID: "body", Type: "text", TextValue: &text},
					},
				},
			},
		}
		warnings := deprecationWarnings(input)
		if len(warnings) != 0 {
			t.Errorf("expected no deprecation warnings, got %v", warnings)
		}
	})
}
