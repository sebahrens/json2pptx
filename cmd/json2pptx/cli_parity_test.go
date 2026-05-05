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
	"list_icons":                 "icons",
	"table_density_guide":        "tables",
	"get_capabilities":           "capabilities",
	"resolve_theme":              "resolve-theme",
	"recommend_pattern":          "recommend-pattern",
	"preview_presentation_plan":  "preview",
	"repair_slide":               "repair",
	"score_deck":                 "score",
	"render_slide_image":         "render-slide",
	"render_deck_thumbnails":     "render-thumbnails",
	"list_template_settings":     "template-settings list",
	"register_template_setting":  "template-settings register",
	"delete_template_setting":    "template-settings delete",
	"get_data_format_hints":      "data-format-hints",
	"get_shape_catalog":          "shape-catalog",
	"get_chart_capabilities":     "capabilities",
	"get_diagram_capabilities":   "capabilities",
	"analyze_deck_rhythm":        "analyze-rhythm",
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
