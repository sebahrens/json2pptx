package main

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/svggen"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

// Contract tests lock the machine-readable response shapes that agents depend
// on. They assert specific JSON field names, types, and nesting — not just
// behavioral correctness. Breaking a contract test means an agent integration
// will break.
//
// Stable fields (safe for programmatic matching):
//   - error envelope: schema_version, tool, subcommand, ok, summary, findings[]
//   - findings[].id, .code (namespaced), .category, .severity, .evidence,
//     .remediation.primary.{action,params}, .next_tool_call, .example_value
//   - success, valid, output_path, slide_count, fit_findings[].code
//
// Advisory fields (human-readable, may change wording):
//   - findings[].message, summary text, warnings[]

// TestMCPErrorEnvelope_ContractShape verifies that MCP error results carry the
// shared FindingEnvelope with the exact field names agents expect.
func TestMCPErrorEnvelope_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Trigger a structured error via invalid JSON.
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": "not-an-object",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	// --- Assert envelope shape via StructuredContent ---
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil — agents depend on this")
	}
	b, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}

	// Parse into raw map to assert field names without relying on Go types.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("error envelope is not a JSON object: %v", err)
	}

	// The legacy {diagnostics, summary} envelope is gone; the wire shape is the
	// shared FindingEnvelope.
	if _, ok := raw["diagnostics"]; ok {
		t.Error("legacy 'diagnostics' field must not be present on the error envelope")
	}

	// Run-level envelope metadata agents branch on.
	for _, key := range []string{"schema_version", "tool", "subcommand", "ok", "summary", "findings"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("error envelope missing required key %q", key)
		}
	}

	// "ok" must be false for an error result.
	var ok2 bool
	if err := json.Unmarshal(raw["ok"], &ok2); err != nil {
		t.Errorf("ok is not a boolean: %v", err)
	}
	if ok2 {
		t.Error("ok = true, want false for an error envelope")
	}

	// "summary" must be a string.
	var summary string
	if err := json.Unmarshal(raw["summary"], &summary); err != nil {
		t.Fatalf("summary is not a string: %v", err)
	}

	// "findings" must be a non-empty array.
	var findings []map[string]json.RawMessage
	if err := json.Unmarshal(raw["findings"], &findings); err != nil {
		t.Fatalf("findings is not an array of objects: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("findings array is empty")
	}

	// --- Assert finding entry shape ---
	f := findings[0]
	for _, field := range []string{"id", "code", "category", "message", "severity"} {
		if _, ok := f[field]; !ok {
			t.Errorf("finding missing required field %q", field)
		}
	}

	// "code" must be a namespaced string.
	var code string
	if err := json.Unmarshal(f["code"], &code); err != nil {
		t.Errorf("finding.code is not a string: %v", err)
	}
	if !strings.Contains(code, ".") {
		t.Errorf("finding.code %q is not namespaced", code)
	}

	// "severity" must be one of the stable values.
	var severity string
	if err := json.Unmarshal(f["severity"], &severity); err != nil {
		t.Errorf("finding.severity is not a string: %v", err)
	}
	switch diagnostics.Severity(severity) {
	case diagnostics.SeverityError, diagnostics.SeverityWarning, diagnostics.SeverityInfo:
		// ok
	default:
		t.Errorf("finding.severity = %q, want one of error/warning/info", severity)
	}

	// --- Assert text fallback is also present ---
	if len(result.Content) == 0 {
		t.Fatal("MCP result has no Content text fallback — older clients depend on this")
	}
}

// TestMCPErrorEnvelope_FixShape verifies that the repair suggestion on a finding
// carries the stable remediation.primary.{action, params} structure.
func TestMCPErrorEnvelope_FixShape(t *testing.T) {
	// Trigger an error that includes a fix (unknown pattern with suggestion).
	result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{
		"name": "kp1-3up", // typo — should suggest "kpi-3up"
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var fe diagnostics.FindingEnvelope
	if err := json.Unmarshal(b, &fe); err != nil {
		t.Fatalf("parse envelope: %v", err)
	}

	// Find a finding with a remediation.
	var fixFinding *diagnostics.Finding
	for i := range fe.Findings {
		if fe.Findings[i].Remediation != nil && fe.Findings[i].Remediation.Primary != nil {
			fixFinding = &fe.Findings[i]
			break
		}
	}
	if fixFinding == nil {
		t.Skip("no finding with remediation — typo suggestion may not have fired")
	}

	// Assert remediation shape: primary.{action: string, params?: object}
	primary := fixFinding.Remediation.Primary
	if primary.Action == "" {
		t.Error("remediation.primary.action is empty — agents use this to decide repair action")
	}

	// Round-trip through JSON to verify serialization.
	primaryJSON, _ := json.Marshal(primary)
	var primaryRaw map[string]json.RawMessage
	if err := json.Unmarshal(primaryJSON, &primaryRaw); err != nil {
		t.Fatalf("remediation.primary is not a JSON object: %v", err)
	}
	if _, ok := primaryRaw["action"]; !ok {
		t.Error("remediation.primary JSON missing 'action' field")
	}
}

// TestMCPGenerateSuccess_ContractShape verifies the generate success response
// has the stable fields agents depend on.
func TestMCPGenerateSuccess_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Contract Test"
			}]
		}]
	}`

	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	// Parse via raw map to assert field names.
	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Stable fields agents depend on.
	for _, field := range []string{"success", "output_path", "slide_count"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("generate success response missing stable field %q", field)
		}
	}

	var success bool
	if err := json.Unmarshal(raw["success"], &success); err != nil {
		t.Errorf("success is not a boolean: %v", err)
	}
	if !success {
		t.Error("expected success=true")
	}

	var slideCount int
	if err := json.Unmarshal(raw["slide_count"], &slideCount); err != nil {
		t.Errorf("slide_count is not a number: %v", err)
	}
	if slideCount < 1 {
		t.Errorf("slide_count = %d, want >= 1", slideCount)
	}
}

// TestMCPValidateSuccess_ContractShape verifies the validate success response
// has the stable fields agents depend on.
func TestMCPValidateSuccess_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Contract Test"
			}]
		}]
	}`

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("unexpected tool error")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// Stable fields agents depend on.
	for _, field := range []string{"valid", "slide_count", "slides"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("validate success response missing stable field %q", field)
		}
	}

	var valid bool
	if err := json.Unmarshal(raw["valid"], &valid); err != nil {
		t.Errorf("valid is not a boolean: %v", err)
	}
	if !valid {
		t.Error("expected valid=true for well-formed input")
	}
}

// TestMCPValidateWithDiagnostics_ContractShape verifies that a successful
// validate folds its warnings/info into the single findings envelope (replacing
// the legacy diagnostics[] array).
func TestMCPValidateWithDiagnostics_ContractShape(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Unknown key is a warning (deck stays valid), so it surfaces in the success
	// response's findings envelope.
	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(`{"template":"midnight-blue","tmplate":"typo","slides":[{"layout_id":"slideLayout2","content":[{"placeholder_id":"title","type":"text","text_value":"Hi"}]}]}`),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("validate should return success with findings, not IsError")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	// The legacy diagnostics[] array is gone; diagnostics live in findings.
	if _, ok := raw["diagnostics"]; ok {
		t.Error("legacy 'diagnostics' field must not be present")
	}

	envRaw, ok := raw["findings"]
	if !ok {
		t.Fatal("validate response missing 'findings' envelope")
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(envRaw, &env); err != nil {
		t.Fatalf("findings is not an object: %v", err)
	}
	for _, key := range []string{"schema_version", "tool", "subcommand", "ok", "summary", "findings"} {
		if _, ok := env[key]; !ok {
			t.Errorf("findings envelope missing required key %q", key)
		}
	}
	var subcommand string
	_ = json.Unmarshal(env["subcommand"], &subcommand)
	if subcommand != "validate_input" {
		t.Errorf("findings.subcommand = %q, want validate_input", subcommand)
	}
	var inputSHA string
	_ = json.Unmarshal(env["input_sha256"], &inputSHA)
	if len(inputSHA) != 64 {
		t.Errorf("findings.input_sha256 = %q, want 64-hex-char digest", inputSHA)
	}

	var findings []map[string]json.RawMessage
	if err := json.Unmarshal(env["findings"], &findings); err != nil {
		t.Fatalf("findings.findings is not an array: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected non-empty findings for unknown key")
	}
	// Each finding carries the namespaced code, message, severity, and category.
	for i, f := range findings {
		for _, field := range []string{"id", "code", "message", "severity", "category"} {
			if _, ok := f[field]; !ok {
				t.Errorf("findings[%d] missing required field %q", i, field)
			}
		}
		var code string
		_ = json.Unmarshal(f["code"], &code)
		if !strings.Contains(code, ".") {
			t.Errorf("findings[%d] code %q is not namespaced", i, code)
		}
	}
}

// TestMCPPatternValidate_ContractShape verifies the pattern validation response
// shape: {ok: bool, errors?: [{code, message, path?, fix?}]}.
func TestMCPPatternValidate_ContractShape(t *testing.T) {
	// Valid input → {ok: true}
	result, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name": "kpi-3up",
		"values": []any{
			map[string]any{"big": "A", "small": "a"},
			map[string]any{"big": "B", "small": "b"},
			map[string]any{"big": "C", "small": "c"},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("unexpected tool error for valid input")
	}

	b, _ := json.Marshal(result.StructuredContent)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}
	if _, ok := raw["ok"]; !ok {
		t.Fatal("pattern validate response missing 'ok' field")
	}
	var ok2 bool
	if err := json.Unmarshal(raw["ok"], &ok2); err != nil {
		t.Errorf("ok is not a boolean: %v", err)
	}
	if !ok2 {
		t.Error("expected ok=true for valid input")
	}

	// Invalid input → {ok: false, errors: [...]}
	result2, err := handleValidatePattern(context.Background(), makeRequest(map[string]any{
		"name":   "kpi-3up",
		"values": []any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.IsError {
		t.Fatal("validation failures should not be IsError")
	}

	b2, _ := json.Marshal(result2.StructuredContent)
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(b2, &raw2); err != nil {
		t.Fatalf("response is not a JSON object: %v", err)
	}

	errorsRaw, hasErrors := raw2["errors"]
	if !hasErrors {
		t.Fatal("expected 'errors' field in invalid pattern validation")
	}

	var validationErrors []map[string]json.RawMessage
	if err := json.Unmarshal(errorsRaw, &validationErrors); err != nil {
		t.Fatalf("errors is not an array: %v", err)
	}
	if len(validationErrors) == 0 {
		t.Fatal("expected non-empty errors array")
	}

	// Each error must have at least code and message.
	for i, ve := range validationErrors {
		for _, field := range []string{"code", "message"} {
			if _, ok := ve[field]; !ok {
				t.Errorf("errors[%d] missing required field %q", i, field)
			}
		}
	}
}

// TestMCPOutputSchemas_ValidJSON verifies that every output schema constant
// is valid JSON and has the expected top-level "type" field.
func TestMCPOutputSchemas_ValidJSON(t *testing.T) {
	schemas := map[string]json.RawMessage{
		"generate_presentation":        outputSchemaGenerate,
		"validate_input":               outputSchemaValidate,
		"list_templates":               outputSchemaListTemplates,
		"get_data_format_hints":        outputSchemaGetDataFormatHints,
		"get_chart_capabilities":       outputSchemaGetChartCapabilities,
		"get_diagram_capabilities":     outputSchemaGetDiagramCapabilities,
		"list_patterns":                outputSchemaListPatterns,
		"show_pattern":                 outputSchemaShowPattern,
		"validate_pattern":             outputSchemaValidatePattern,
		"expand_pattern":               outputSchemaExpandPattern,
		"expand_patterns":              outputSchemaExpandPatterns,
		"recommend_pattern":            outputSchemaRecommendPattern,
		"recommend_visual":             outputSchemaRecommendVisual,
		"list_icons":                   outputSchemaListIcons,
		"preview_icon":                 outputSchemaPreviewIcon,
		"get_shape_catalog":            outputSchemaGetShapeCatalog,
		"render_slide_image":           outputSchemaRenderSlideImage,
		"render_slide_image_from_json": outputSchemaRenderSlideImage,
		"render_deck_thumbnails":       outputSchemaRenderDeckThumbnails,
		"score_deck":                   outputSchemaScoreDeck,
		"inspect_slide_images":         outputSchemaInspectSlideImages,
		"preview_presentation_plan":    outputSchemaPreviewPlan,
		"preview_slide_wireframe":      outputSchemaPreviewSlideWireframe,
		"repair_slide":                 outputSchemaRepairSlide,
		"propose_repairs":              outputSchemaProposeRepairs,
		"table_density_guide":          outputSchemaTableDensityGuide,
		"resolve_theme":                outputSchemaResolveTheme,
		"list_template_settings":       outputSchemaListTemplateSettings,
		"register_template_setting":    outputSchemaRegisterTemplateSetting,
		"delete_template_setting":      outputSchemaDeleteTemplateSetting,
		"get_capabilities":             outputSchemaGetCapabilities,
		"read_presentation":            outputSchemaReadPresentation,
		"analyze_deck_rhythm":          outputSchemaAnalyzeDeckRhythm,
		"plan_deck":                    outputSchemaPlanDeck,
		"describe_finding":             outputSchemaDescribeFinding,
		"audit_palette":                outputSchemaAuditPalette,
		"examine_template":             outputSchemaExamineTemplate,
	}

	for name, schema := range schemas {
		var parsed map[string]any
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Errorf("%s: output schema is not valid JSON: %v", name, err)
			continue
		}
		// Every schema must have a "type" field at top level.
		if _, ok := parsed["type"]; !ok {
			t.Errorf("%s: output schema missing top-level 'type' field", name)
		}
	}
}

// TestMCPOutputSchema_WireframeStructuralContract verifies that the
// preview_slide_wireframe output schema declares the structural-only
// inspection-contract fields and marks them required, so agents can
// machine-detect that a wireframe is not rendered visual-QA evidence.
// Regression guard for go-slide-creator-5lr1.
func TestMCPOutputSchema_WireframeStructuralContract(t *testing.T) {
	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(outputSchemaPreviewSlideWireframe, &parsed); err != nil {
		t.Fatalf("wireframe schema is not valid JSON: %v", err)
	}

	contractFields := []string{"inspection_kind", "contract", "not_text_flow_safe", "limitations"}
	for _, f := range contractFields {
		if _, ok := parsed.Properties[f]; !ok {
			t.Errorf("wireframe schema missing property %q", f)
		}
	}

	requiredSet := make(map[string]bool, len(parsed.Required))
	for _, r := range parsed.Required {
		requiredSet[r] = true
	}
	for _, f := range contractFields {
		if !requiredSet[f] {
			t.Errorf("wireframe schema field %q must be in required[]", f)
		}
	}

	// The structural-only contract values are fixed: assert the enums pin
	// them so a future edit cannot loosen the contract silently.
	for field, want := range map[string]string{"inspection_kind": "wireframe_structural", "contract": "structural_only"} {
		var prop struct {
			Enum []string `json:"enum"`
		}
		if err := json.Unmarshal(parsed.Properties[field], &prop); err != nil {
			t.Errorf("wireframe schema field %q is not an object: %v", field, err)
			continue
		}
		if len(prop.Enum) != 1 || prop.Enum[0] != want {
			t.Errorf("wireframe schema field %q enum = %v, want [%q]", field, prop.Enum, want)
		}
	}
}

// TestMCPOutputSchemas_AllToolsCovered verifies that every registered MCP tool
// has a corresponding output schema defined.
func TestMCPOutputSchemas_AllToolsCovered(t *testing.T) {
	covered := map[string]bool{
		"generate_presentation":        true,
		"validate_input":               true,
		"list_templates":               true,
		"get_data_format_hints":        true,
		"get_chart_capabilities":       true,
		"get_diagram_capabilities":     true,
		"list_patterns":                true,
		"show_pattern":                 true,
		"validate_pattern":             true,
		"expand_pattern":               true,
		"expand_patterns":              true,
		"recommend_pattern":            true,
		"list_icons":                   true,
		"preview_icon":                 true,
		"get_shape_catalog":            true,
		"render_slide_image":           true,
		"render_slide_image_from_json": true,
		"render_deck_thumbnails":       true,
		"score_deck":                   true,
		"score_candidates":             true,
		"inspect_slide_images":         true,
		"preview_presentation_plan":    true,
		"preview_slide_wireframe":      true,
		"repair_slide":                 true,
		"repair_slides_batch":          true,
		"propose_repairs":              true,
		"auto_repair":                  true,
		"make_deck":                    true,
		"table_density_guide":          true,
		"resolve_theme":                true,
		"list_template_settings":       true,
		"register_template_setting":    true,
		"delete_template_setting":      true,
		"get_capabilities":             true,
		"read_presentation":            true,
		"analyze_deck_rhythm":          true,
		"plan_deck":                    true,
		"recommend_visual":             true,
		"get_input_schema":             true,
		"validate_presentation_output": true,
		"get_started":                  true,
		"describe_finding":             true,
		"audit_palette":                true,
		"examine_template":             true,
		"apply_deck_patch":             true,
	}

	for _, name := range mcpToolNames() {
		if !covered[name] {
			t.Errorf("MCP tool %q has no output schema — add one to mcp_output_schemas.go", name)
		}
	}
}

// mcpAllToolDefs returns all MCP tool definitions by calling their constructors.
func mcpAllToolDefs() []struct{ Name, Description string } {
	tools := []struct{ Name, Description string }{
		func() struct{ Name, Description string } {
			t := mcpGenerateTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
		func() struct{ Name, Description string } {
			t := mcpListTemplatesTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
		func() struct{ Name, Description string } {
			t := mcpGetDataFormatHintsTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
		func() struct{ Name, Description string } {
			t := mcpGetChartCapabilitiesTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
		func() struct{ Name, Description string } {
			t := mcpGetDiagramCapabilitiesTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
		func() struct{ Name, Description string } {
			t := mcpValidateTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
		func() struct{ Name, Description string } {
			t := mcpRepairSlideTool()
			return struct{ Name, Description string }{t.Name, t.Description}
		}(),
	}
	return tools
}

// TestMCPDescriptions_NoTBD verifies that no MCP tool description contains
// "TBD" language that would mislead agents into thinking capabilities are
// not yet populated.
func TestMCPDescriptions_NoTBD(t *testing.T) {
	for _, tool := range mcpAllToolDefs() {
		if strings.Contains(strings.ToUpper(tool.Description), "TBD") {
			t.Errorf("tool %q description contains 'TBD': %s", tool.Name, tool.Description)
		}
	}
}

// TestMCPSupportedTypes_ChartTypesMatchCapabilities verifies that the
// chart types advertised by buildSupportedTypes match the runtime
// ChartCapabilities registry.
func TestMCPSupportedTypes_ChartTypesMatchCapabilities(t *testing.T) {
	st := buildSupportedTypes()
	caps := svggen.ChartCapabilities()

	capTypes := make([]string, len(caps))
	for i, c := range caps {
		capTypes[i] = c.Type
	}
	sort.Strings(capTypes)

	advertised := make([]string, len(st.ChartTypes))
	copy(advertised, st.ChartTypes)
	sort.Strings(advertised)

	if len(advertised) != len(capTypes) {
		t.Fatalf("advertised %d chart types but capabilities has %d\nadvertised: %v\ncapabilities: %v",
			len(advertised), len(capTypes), advertised, capTypes)
	}
	for i := range advertised {
		if advertised[i] != capTypes[i] {
			t.Errorf("chart type mismatch at [%d]: advertised %q vs capability %q", i, advertised[i], capTypes[i])
		}
	}
}

// TestMCPSupportedTypes_DiagramTypesMatchCapabilities verifies that the
// diagram types advertised by buildSupportedTypes match the runtime
// DiagramCapabilitiesReady registry (stubs excluded).
func TestMCPSupportedTypes_DiagramTypesMatchCapabilities(t *testing.T) {
	st := buildSupportedTypes()
	caps := svggen.DiagramCapabilitiesReady()

	capTypes := make([]string, len(caps))
	for i, c := range caps {
		capTypes[i] = c.Type
	}
	sort.Strings(capTypes)

	advertised := make([]string, len(st.DiagramTypes))
	copy(advertised, st.DiagramTypes)
	sort.Strings(advertised)

	if len(advertised) != len(capTypes) {
		t.Fatalf("advertised %d diagram types but capabilities has %d\nadvertised: %v\ncapabilities: %v",
			len(advertised), len(capTypes), advertised, capTypes)
	}
	for i := range advertised {
		if advertised[i] != capTypes[i] {
			t.Errorf("diagram type mismatch at [%d]: advertised %q vs capability %q", i, advertised[i], capTypes[i])
		}
	}
}

// TestMCPSupportedTypes_GridCellTypesMatchShapeGrid verifies that the
// grid_cell_types advertised by buildSupportedTypes match the CellKind
// constants defined in the shapegrid package.
func TestMCPSupportedTypes_GridCellTypesMatchShapeGrid(t *testing.T) {
	st := buildSupportedTypes()

	// Canonical cell kinds from shapegrid/types.go.
	canonical := []string{
		string(shapegrid.CellKindShape),
		string(shapegrid.CellKindTable),
		string(shapegrid.CellKindIcon),
		string(shapegrid.CellKindImage),
		string(shapegrid.CellKindDiagram),
		string(shapegrid.CellKindComposite),
	}
	sort.Strings(canonical)

	advertised := make([]string, len(st.GridCellTypes))
	copy(advertised, st.GridCellTypes)
	sort.Strings(advertised)

	if len(advertised) != len(canonical) {
		t.Fatalf("advertised %d grid cell types but shapegrid has %d\nadvertised: %v\ncanonical: %v",
			len(advertised), len(canonical), advertised, canonical)
	}
	for i := range advertised {
		if advertised[i] != canonical[i] {
			t.Errorf("grid cell type mismatch at [%d]: advertised %q vs canonical %q", i, advertised[i], canonical[i])
		}
	}
}

// TestMCPDataFormatHints_BMCMatchesCanonicalSchema verifies that the BMC
// data_format_hints use the same field names and required/optional status
// as the canonical pattern schema in internal/patterns/bmccanvas.go.
func TestMCPDataFormatHints_BMCMatchesCanonicalSchema(t *testing.T) {
	hints := buildDataFormatHints()
	bmc, ok := hints["business_model_canvas"]
	if !ok {
		t.Fatal("business_model_canvas missing from data_format_hints")
	}

	// Canonical required fields from internal/patterns/bmccanvas.go.
	canonicalRequired := []string{
		"key_partners", "key_activities", "key_resources",
		"value_propositions", "customer_relations", "channels",
		"customer_segments", "cost_structure", "revenue_streams",
	}
	sort.Strings(canonicalRequired)

	got := make([]string, len(bmc.RequiredKeys))
	copy(got, bmc.RequiredKeys)
	sort.Strings(got)

	if len(got) != len(canonicalRequired) {
		t.Fatalf("BMC required keys: got %v, want %v", got, canonicalRequired)
	}
	for i := range got {
		if got[i] != canonicalRequired[i] {
			t.Errorf("BMC required key [%d]: got %q, want %q", i, got[i], canonicalRequired[i])
		}
	}

	if len(bmc.OptionalKeys) != 0 {
		t.Errorf("BMC should have no optional keys (all are required), got %v", bmc.OptionalKeys)
	}
}
