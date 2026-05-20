package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// mcpToCLI maps every MCP tool name to its CLI subcommand counterpart.
// The parity test below asserts that this table covers all MCP tools and
// that every listed CLI command is recognized by the binary.
var mcpToCLI = map[string]string{
	// Tools with direct CLI subcommands
	"generate_presentation":      "generate",
	"read_presentation":          "read",
	"validate_input":             "validate",
	"list_templates":             "skill-info",
	"list_patterns":              "patterns list",
	"show_pattern":               "patterns show",
	"validate_pattern":           "patterns validate",
	"expand_pattern":             "patterns expand",
	"expand_patterns":            "patterns expand", // batch is an MCP convenience; CLI users loop with the singular subcommand
	"list_icons":                 "icons",
	"preview_icon":               "preview-icon",
	"table_density_guide":        "tables",
	"get_capabilities":           "capabilities",
	"resolve_theme":              "resolve-theme",
	"recommend_pattern":          "recommend-pattern",
	"preview_presentation_plan":  "preview",
	"preview_slide_wireframe":    "preview-wireframe",
	"repair_slide":               "repair",
	"propose_repairs":            "repair", // [MCP-only] CLI users translate findings to fixes manually and invoke 'json2pptx repair'
	"score_deck":                 "score",
	"score_candidates":           "score-candidates",
	"inspect_slide_images":       "inspect",
	"render_slide_image":           "render-slide",
	"render_slide_image_from_json": "render-slide-from-json",
	"render_deck_thumbnails":       "render-thumbnails",
	"list_template_settings":     "template-settings list",
	"register_template_setting":  "template-settings register",
	"delete_template_setting":    "template-settings delete",
	"get_data_format_hints":      "data-format-hints",
	"get_shape_catalog":          "shape-catalog",
	"get_chart_capabilities":     "capabilities",
	"get_diagram_capabilities":   "capabilities",
	"analyze_deck_rhythm":        "analyze-rhythm",
	"plan_deck":                  "plan-deck",
	"recommend_visual":           "recommend-visual",
	"get_input_schema":               "input-schema",
	"validate_presentation_output":   "validate-output",
	"get_started":                    "get-started",
	"describe_finding":               "describe-finding",
}

// TestEveryMCPToolHasCLI asserts that every tool registered in the MCP server
// has a corresponding entry in mcpToCLI.
func TestEveryMCPToolHasCLI(t *testing.T) {
	tools := mcpToolNames()
	for _, tool := range tools {
		if _, ok := mcpToCLI[tool]; !ok {
			t.Errorf("MCP tool %q has no CLI counterpart in mcpToCLI table", tool)
		}
	}
}

// TestCLISubcommandsExist verifies that each CLI command listed in the parity
// table is recognized by the dispatch function (prints help rather than
// "unknown command").
func TestCLISubcommandsExist(t *testing.T) {
	// Build the binary once.
	binary := buildTestBinary(t)

	seen := make(map[string]bool)
	for _, cliCmd := range mcpToCLI {
		// Extract the top-level subcommand (first word).
		topLevel := strings.Fields(cliCmd)[0]
		if seen[topLevel] {
			continue
		}
		seen[topLevel] = true

		t.Run(topLevel, func(t *testing.T) {
			cmd := exec.Command(binary, topLevel, "-h") //nolint:gosec
			cmd.Env = append(os.Environ(), "HOME=/tmp")
			out, _ := cmd.CombinedOutput()
			// -h should produce usage text, not "unknown command".
			if strings.Contains(string(out), "unknown command") {
				t.Errorf("CLI subcommand %q not recognized: %s", topLevel, string(out))
			}
		})
	}
}

// buildTestBinary compiles the json2pptx binary for testing.
func buildTestBinary(t *testing.T) string {
	t.Helper()

	binary := t.TempDir() + "/json2pptx"
	cmd := exec.Command("go", "build", "-o", binary, ".") //nolint:gosec
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v\n%s", err, out)
	}
	return binary
}

// TestHelpListsMCPOnlyTools verifies that `json2pptx help` advertises an
// "MCP-only" section so agents shelling out to the CLI know which capabilities
// require the MCP server. Currently expand_patterns (batch) is the only true
// MCP-only entry, and recommend-visual has documented CLI parity gaps.
func TestHelpListsMCPOnlyTools(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "help") //nolint:gosec
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	mustContain := []string{
		"MCP-only tools",
		"expand_patterns",
		"[MCP-only]",
		"CLI parity gaps",
		"recommend-visual",
	}
	for _, want := range mustContain {
		if !strings.Contains(outStr, want) {
			t.Errorf("help output missing %q\n--- output ---\n%s", want, outStr)
		}
	}
}

// TestHelpListsAllDispatchableSubcommands ensures every CLI subcommand
// recognized by the dispatcher is also documented in the Commands section of
// `json2pptx help`. Subcommands missing from help are invisible to agents that
// scan help text for available commands.
func TestHelpListsAllDispatchableSubcommands(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "help") //nolint:gosec
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	out, _ := cmd.CombinedOutput()
	outStr := string(out)

	// Top-level CLI commands wired in main.dispatch (excluding aliases for
	// version/help and the implicit-generate flag fallback).
	subcommands := []string{
		"generate", "read", "serve", "mcp", "validate", "validate-template",
		"template-check", "validate-output", "patterns", "icons", "tables",
		"skill-info", "capabilities", "get-started", "input-schema",
		"resolve-theme", "recommend-pattern", "preview", "preview-wireframe",
		"repair", "score", "score-candidates", "inspect", "analyze-rhythm",
		"plan-deck", "recommend-visual", "render-slide", "render-slide-from-json",
		"render-thumbnails", "template-settings", "data-format-hints",
		"preview-patterns", "shape-catalog", "audit-palette", "describe-finding",
		"version",
	}
	for _, sub := range subcommands {
		if !strings.Contains(outStr, sub) {
			t.Errorf("help output omits subcommand %q (must be listed so agents discover it)\n--- output ---\n%s", sub, outStr)
		}
	}
}

// TestResolveThemeCLI_OverrideJSON verifies that `resolve-theme -override`
// accepts inline JSON and forwards it as the resolve_theme MCP tool's
// `theme_override` parameter — the new CLI surface for the same per-deck
// override agents already get over MCP.
func TestResolveThemeCLI_OverrideJSON(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "resolve-theme", //nolint:gosec
		"-template", "midnight-blue",
		"-templates-dir", "../../templates",
		"-override", `{"colors":{"accent1":"#336699"}}`,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve-theme -override failed: %v\n%s", err, out)
	}

	outStr := string(out)
	// The applied override echo confirms the flag reached the handler.
	if !strings.Contains(outStr, `"applied_theme_override"`) {
		t.Errorf("expected applied_theme_override in output, got: %s", outStr[:min(400, len(outStr))])
	}
	// The post-override accent1 must appear in the colors map.
	if !strings.Contains(strings.ToLower(outStr), "#336699") {
		t.Errorf("expected #336699 in output (post-override accent1), got: %s", outStr[:min(400, len(outStr))])
	}
}

// TestResolveThemeCLI_OverrideFile verifies that -override accepts an @path
// reference so agents can keep large theme overrides in a file instead of
// shell-quoting them inline.
func TestResolveThemeCLI_OverrideFile(t *testing.T) {
	binary := buildTestBinary(t)

	dir := t.TempDir()
	overridePath := dir + "/theme.json"
	if err := os.WriteFile(overridePath, []byte(`{"colors":{"accent1":"#aabbcc"}}`), 0o600); err != nil {
		t.Fatalf("failed to write override file: %v", err)
	}

	cmd := exec.Command(binary, "resolve-theme", //nolint:gosec
		"-template", "midnight-blue",
		"-templates-dir", "../../templates",
		"-override", "@"+overridePath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve-theme -override @file failed: %v\n%s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "#aabbcc") {
		t.Errorf("expected #aabbcc in output (post-override accent1), got: %s",
			string(out)[:min(400, len(out))])
	}
}

// TestResolveThemeCLI_VariationUnknown verifies that -variation errors with
// a helpful message when the registry has no matching preset. This guards
// the extension point: as soon as a built-in variation is registered, the
// flag works; until then, unknown names produce a clear error rather than
// silently being ignored.
func TestResolveThemeCLI_VariationUnknown(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "resolve-theme", //nolint:gosec
		"-template", "midnight-blue",
		"-templates-dir", "../../templates",
		"-variation", "no-such-variation",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error for unknown variation, got success: %s", string(out))
	}
	if !strings.Contains(string(out), "unknown variation") {
		t.Errorf("expected 'unknown variation' in error, got: %s", string(out))
	}
}

// TestResolveThemeCLI_OverrideAndVariationMutuallyExclusive verifies the CLI
// rejects passing both flags so users don't get a silent winner.
func TestResolveThemeCLI_OverrideAndVariationMutuallyExclusive(t *testing.T) {
	binary := buildTestBinary(t)

	cmd := exec.Command(binary, "resolve-theme", //nolint:gosec
		"-template", "midnight-blue",
		"-templates-dir", "../../templates",
		"-override", `{"colors":{"accent1":"#336699"}}`,
		"-variation", "dark",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error when both -override and -variation are set, got success: %s", string(out))
	}
	if !strings.Contains(string(out), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %s", string(out))
	}
}

// TestValidateFormatJSON verifies that -format=json produces the same
// dryRunOutput shape as the MCP validate_input tool.
func TestValidateFormatJSON(t *testing.T) {
	binary := buildTestBinary(t)

	// Use a minimal valid example.
	examplePath := "../../examples/basic-deck.json"
	if _, err := os.Stat(examplePath); err != nil {
		t.Skip("examples/basic-deck.json not available")
	}

	cmd := exec.Command(binary, "validate", "-format=json", "-templates-dir", "../../templates", examplePath) //nolint:gosec
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("validate -format=json failed: %v\n%s", err, out)
	}

	// The output should be valid JSON containing dryRunOutput keys (valid, slides).
	// The "diagnostics" key is omitempty so may not appear for a valid deck.
	outStr := strings.TrimSpace(string(out))
	if !strings.Contains(outStr, `"valid"`) {
		t.Errorf("expected valid key in -format=json output, got: %s", outStr[:min(200, len(outStr))])
	}
	if !strings.Contains(outStr, `"slides"`) {
		t.Errorf("expected slides key in -format=json output (dryRunOutput shape), got: %s", outStr[:min(200, len(outStr))])
	}
	if !strings.Contains(outStr, `"slide_count"`) {
		t.Errorf("expected slide_count key in -format=json output, got: %s", outStr[:min(200, len(outStr))])
	}
}
