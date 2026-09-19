package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// go-slide-creator-5pta. get_started's first step is get_capabilities, and
// SKILL.md repeats it. One call returned 183,242 wire bytes — 46K tokens — of
// which tool_list alone was 55,475 B, a verbatim duplicate of what tools/list
// already sent. The most useful field for an MCP agent
// (runtime.render_available / output_dir, 326 B) sat behind all of it, and the
// inputSchema was {"type":"object","properties":{}}: there was no way to ask
// for less.

func capabilitiesFor(t *testing.T, args map[string]any) (capabilitiesResponse, int) {
	t.Helper()
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	res, err := mc.handleGetCapabilities(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	text := res.Content[0].(mcpgo.TextContent).Text
	var resp capabilitiesResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return resp, len(text)
}

// TestGetCapabilitiesDefaultBudget is the bead's named assertion.
func TestGetCapabilitiesDefaultBudget(t *testing.T) {
	resp, size := capabilitiesFor(t, map[string]any{})

	if size > 12*1024 {
		t.Errorf("default response is %d B, over the 12 KB budget", size)
	}

	// Drift detection must survive the smallest projection.
	if resp.SchemaVersion == "" || resp.ToolVersion == "" || resp.ChangelogURL == "" {
		t.Error("the default lost a version field — drift detection needs them in every projection")
	}
	if resp.Runtime.SchemaFingerprint == "" {
		t.Error("the default lost runtime.schema_fingerprint")
	}
	if len(resp.Features.StrictFit) == 0 {
		t.Error("the default lost features")
	}
	if len(resp.DeprecatedFields) == 0 {
		t.Error("the default lost deprecated_fields")
	}

	// And the expensive duplicates are gone.
	if len(resp.ToolList) != 0 {
		t.Errorf("the default carries tool_list (%d entries) — tools/list already sent it", len(resp.ToolList))
	}
	if len(resp.MCPToolsAvailable) != 0 {
		t.Error("the default carries mcp_tools_available")
	}
	if len(resp.CLIOnlyCommands) != 0 {
		t.Error("the default carries cli_only_commands — not callable over MCP")
	}

	want := []string{"deprecations", "features", "runtime"}
	if strings.Join(resp.SectionsIncluded, ",") != strings.Join(want, ",") {
		t.Errorf("sections_included = %v, want %v", resp.SectionsIncluded, want)
	}
}

func TestGetCapabilitiesSectionsProjection(t *testing.T) {
	runtimeOnly, size := capabilitiesFor(t, map[string]any{"sections": []any{"runtime"}})
	if size > 2*1024 {
		t.Errorf("sections=[runtime] is %d B, want well under 2 KB", size)
	}
	if runtimeOnly.Runtime.OutputDir == "" {
		t.Error("sections=[runtime] lost the runtime block")
	}
	if len(runtimeOnly.Features.StrictFit) != 0 || len(runtimeOnly.DeprecatedFields) != 0 {
		t.Error("sections=[runtime] carried another section")
	}

	tools, _ := capabilitiesFor(t, map[string]any{"sections": []any{"tools"}})
	if len(tools.ToolList) == 0 {
		t.Error("sections=[tools] did not return the catalogue")
	}
	if len(tools.Features.StrictFit) != 0 {
		t.Error("sections=[tools] carried features")
	}

	all, allSize := capabilitiesFor(t, map[string]any{"sections": []any{"all"}})
	for _, check := range []struct {
		name string
		ok   bool
	}{
		{"tool_list", len(all.ToolList) > 0},
		{"mcp_tools_available", len(all.MCPToolsAvailable) > 0},
		{"vocabularies", len(all.Vocabularies.ChartTypes) > 0},
		{"error_codes", len(all.ErrorCodes) > 0},
		{"cli_only_commands", len(all.CLIOnlyCommands) > 0},
		{"deprecated_fields", len(all.DeprecatedFields) > 0},
		{"features", len(all.Features.StrictFit) > 0},
	} {
		if !check.ok {
			t.Errorf("sections=[all] is missing %s", check.name)
		}
	}
	_, defSize := capabilitiesFor(t, map[string]any{})
	if allSize <= defSize*4 {
		t.Errorf("sections=[all] (%d B) is not meaningfully larger than the default (%d B)", allSize, defSize)
	}

	// A comma-separated string is accepted too.
	str, _ := capabilitiesFor(t, map[string]any{"sections": "runtime,features"})
	if str.Runtime.OutputDir == "" || len(str.Features.StrictFit) == 0 {
		t.Error("the comma-separated form did not select both sections")
	}
	if len(str.DeprecatedFields) != 0 {
		t.Error("the comma-separated form carried an unrequested section")
	}
}

func TestGetCapabilitiesRejectsUnknownSection(t *testing.T) {
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	res, err := mc.handleGetCapabilities(context.Background(), makeRequest(map[string]any{
		"sections": []any{"runtime", "nope"},
	}))
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("an unknown section was accepted")
	}
	b, _ := json.Marshal(res.StructuredContent)
	if !strings.Contains(string(b), "nope") {
		t.Errorf("the error does not name the offending value: %s", b)
	}
}

func TestParseCapabilitySections(t *testing.T) {
	def, err := parseCapabilitySections(makeRequest(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range capSectionsDefault {
		if !def.has(want) {
			t.Errorf("default is missing %s", want)
		}
	}
	if def.has(capSectionTools) {
		t.Error("the default includes tools")
	}

	// An empty array falls back to the default rather than returning nothing.
	empty, err := parseCapabilitySections(makeRequest(map[string]any{"sections": []any{}}))
	if err != nil {
		t.Fatal(err)
	}
	if !empty.has(capSectionRuntime) {
		t.Error("an empty sections array should fall back to the default")
	}

	if _, err := parseCapabilitySections(makeRequest(map[string]any{"sections": []any{1}})); err == nil {
		t.Error("a non-string entry was accepted")
	}
	if _, err := parseCapabilitySections(makeRequest(map[string]any{"sections": 5})); err == nil {
		t.Error("a non-array, non-string sections value was accepted")
	}
}
