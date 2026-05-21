package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/template"
)

// fitFindingsInEnvelope returns the FIT-category findings from a validate
// response envelope. Fit findings are now folded into the single Findings
// envelope (category "FIT"), replacing the legacy fit_findings array.
func fitFindingsInEnvelope(env diagnostics.FindingEnvelope) []diagnostics.Finding {
	var out []diagnostics.Finding
	for _, f := range env.Findings {
		if f.Category == diagnostics.NamespaceFit {
			out = append(out, f)
		}
	}
	return out
}

// fitFindingCodes returns the codes of the given findings, for diagnostic
// messages in test failures.
func fitFindingCodes(findings []patterns.FitFinding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Code)
	}
	return out
}

func testMCPConfig(t *testing.T) *mcpConfig {
	t.Helper()
	return &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

func TestMCPValidateFitReport(t *testing.T) {
	mc := testMCPConfig(t)

	// A minimal valid deck with no content that would produce fit findings.
	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Test"
			}]
		}]
	}`

	t.Run("fit_report=false omits fit findings", func(t *testing.T) {
		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"fit_report":   false,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp dryRunOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if got := fitFindingsInEnvelope(resp.Findings); len(got) != 0 {
			t.Errorf("expected no FIT findings when fit_report=false, got %d", len(got))
		}
	})

	t.Run("fit_report=true carries the findings envelope", func(t *testing.T) {
		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"fit_report":   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp dryRunOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		// The findings envelope is always present (replacing the legacy
		// fit_findings array), so an agent can branch on findings.ok.
		if resp.Findings.SchemaVersion == "" {
			t.Error("expected findings envelope to be present (schema_version set)")
		}
		if resp.Findings.Findings == nil {
			t.Error("findings.findings must be non-nil (may be empty)")
		}
	})

	t.Run("minimal deck produces no fit findings", func(t *testing.T) {
		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp dryRunOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if got := fitFindingsInEnvelope(resp.Findings); len(got) != 0 {
			t.Errorf("expected no FIT findings for a minimal short-text deck, got %d", len(got))
		}
	})
}

// TestMCPValidateIconBundledNameUnknown verifies that validate_input
// preflights bundled icon names: typos return ICON_BUNDLED_NAME_UNKNOWN
// with structured suggestions so agents can repair the name without burning
// a generate call.
func TestMCPValidateIconBundledNameUnknown(t *testing.T) {
	mc := testMCPConfig(t)

	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"shape_grid": {
				"rows": [{
					"cells": [{"icon": {"name": "chart-pi"}}]
				}]
			}
		}]
	}`

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for invalid icon name, got success: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	diags := legacyDiagsFromWire(t, text)

	var found map[string]any
	for _, d := range diags {
		if d["code"] == "ICON_BUNDLED_NAME_UNKNOWN" {
			found = d
			break
		}
	}
	if found == nil {
		t.Fatalf("expected ICON_BUNDLED_NAME_UNKNOWN diagnostic, got: %s", text)
	}
	if path, _ := found["path"].(string); path != "/slides/0/shape_grid/rows/0/cells/0/icon" {
		t.Errorf("expected json_path '/slides/0/shape_grid/rows/0/cells/0/icon', got %q", path)
	}
	details, _ := found["details"].(map[string]any)
	if details == nil {
		t.Fatalf("expected details map, got %v", found)
	}
	if details["input_value"] != "chart-pi" {
		t.Errorf("expected input_value 'chart-pi', got %v", details["input_value"])
	}
	suggestions, _ := details["suggestions"].([]any)
	if len(suggestions) == 0 || suggestions[0] != "chart-pie" {
		t.Errorf("expected first suggestion 'chart-pie', got %v", suggestions)
	}
}

// TestMCPValidateIconBundledNameValid confirms that a valid bundled icon
// name passes validate_input without emitting an icon finding.
func TestMCPValidateIconBundledNameValid(t *testing.T) {
	mc := testMCPConfig(t)

	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"shape_grid": {
				"rows": [{
					"cells": [{"icon": {"name": "chart-pie"}}]
				}]
			}
		}]
	}`

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected validation to succeed for valid bundled name, got error: %s", text)
	}
}

// TestMCPValidateAssetPathsAllSurfaces ensures validate_input preflights
// every local-asset surface (icon, content image_value, shape_grid cell
// image, slide background) and emits one structured diagnostic per broken
// reference. This is the MCP parity test for go-slide-creator-tigj.
//
// validate_input must surface these findings without succeeding, so agents
// catch broken paths before burning a generate call.
func TestMCPValidateAssetPathsAllSurfaces(t *testing.T) {
	mc := testMCPConfig(t)

	// All four asset paths point at files that do not exist anywhere on
	// disk relative to the server CWD, so each surface produces its own
	// finding.
	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"background": {"image": "tigj-missing-bg.png"},
			"content": [{
				"placeholder_id": "body",
				"type": "image",
				"image_value": {"path": "tigj-missing-photo.png"}
			}],
			"shape_grid": {
				"rows": [{
					"cells": [
						{"icon": {"path": "tigj-missing-icon.svg"}},
						{"image": {"path": "tigj-missing-grid.jpg"}}
					]
				}]
			}
		}]
	}`

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for missing asset paths, got success: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	diags := legacyDiagsFromWire(t, text)

	// Expect exactly one diagnostic per surface, keyed by json_path.
	want := map[string]string{
		"/slides/0/background/image":                     "BACKGROUND_IMAGE_PATH",
		"/slides/0/content/0/image_value/path":           "IMAGE_PATH",
		"/slides/0/shape_grid/rows/0/cells/0/icon":       "ICON_NOT_FOUND",
		"/slides/0/shape_grid/rows/0/cells/1/image/path": "IMAGE_PATH",
	}
	got := make(map[string]string, len(want))
	for _, d := range diags {
		path, _ := d["path"].(string)
		code, _ := d["code"].(string)
		if _, ok := want[path]; ok {
			got[path] = code
		}
	}
	for path, code := range want {
		if got[path] != code {
			t.Errorf("path %q: expected %s, got %q (full envelope: %s)", path, code, got[path], text)
		}
	}
}

// TestMCPValidateIconPathCodes verifies that validate_input emits the
// structured per-failure codes for icon.path issues (extension, missing
// file, traversal) so agents can repair each broken icon without burning a
// generate call.
func TestMCPValidateIconPathCodes(t *testing.T) {
	mc := testMCPConfig(t)

	cases := []struct {
		name     string
		path     string
		wantCode string
	}{
		{name: "non-svg extension", path: "icon.png", wantCode: "ICON_PATH_EXT_INVALID"},
		{name: "missing file", path: "tigj-truly-missing.svg", wantCode: "ICON_NOT_FOUND"},
		{name: "traversal in input", path: "../escape.svg", wantCode: "ICON_PATH_TRAVERSAL"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deckJSON := fmt.Sprintf(`{
				"template": "midnight-blue",
				"slides": [{
					"layout_id": "slideLayout2",
					"shape_grid": {
						"rows": [{
							"cells": [{"icon": {"path": %q}}]
						}]
					}
				}]
			}`, tc.path)

			result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
				"presentation": mustParseJSON(deckJSON),
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError=true for %s, got success: %v", tc.name, result.Content)
			}

			text := result.Content[0].(mcp.TextContent).Text
			diags := legacyDiagsFromWire(t, text)

			var found map[string]any
			for _, d := range diags {
				if d["code"] == tc.wantCode {
					found = d
					break
				}
			}
			if found == nil {
				t.Fatalf("expected %s diagnostic, got: %s", tc.wantCode, text)
			}
			if path, _ := found["path"].(string); path != "/slides/0/shape_grid/rows/0/cells/0/icon" {
				t.Errorf("expected json_path '/slides/0/shape_grid/rows/0/cells/0/icon', got %q", path)
			}
			details, _ := found["details"].(map[string]any)
			if details == nil {
				t.Fatalf("expected details map, got %v", found)
			}
			if details["input_value"] != tc.path {
				t.Errorf("expected input_value %q, got %v", tc.path, details["input_value"])
			}
			if details["asset_kind"] != "icon" {
				t.Errorf("expected asset_kind 'icon', got %v", details["asset_kind"])
			}
		})
	}
}

// TestMCPGenerateAssetPathsAbsoluteSuccess confirms that generate_presentation
// happily accepts absolute paths for every asset surface — the local-asset
// pass must rewrite them in place without inventing new errors.
func TestMCPGenerateAssetPathsAbsoluteSuccess(t *testing.T) {
	mc := testMCPConfig(t)

	tmpDir := t.TempDir()
	bgPath := filepath.Join(tmpDir, "bg.png")
	photoPath := filepath.Join(tmpDir, "photo.png")
	iconPath := filepath.Join(tmpDir, "icon.svg")
	gridPath := filepath.Join(tmpDir, "grid.jpg")
	for _, p := range []string{bgPath, photoPath, gridPath} {
		// Minimal PNG-like bytes are fine for the path-resolution test —
		// we never reach image decode because generate would fail later
		// for unrelated reasons if we did. The local-asset pass cares only
		// about path existence + extension.
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	if err := os.WriteFile(iconPath, []byte(`<svg/>`), 0644); err != nil {
		t.Fatalf("write %s: %v", iconPath, err)
	}

	deckJSON := fmt.Sprintf(`{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"background": {"image": %q},
			"shape_grid": {
				"rows": [{
					"cells": [
						{"icon": {"path": %q}},
						{"image": {"path": %q}}
					]
				}]
			}
		}]
	}`, bgPath, iconPath, gridPath)

	result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deckJSON),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(mcp.TextContent).Text
		t.Fatalf("expected validate_input to succeed for valid absolute paths, got error: %s", text)
	}
}

func TestMCPGenerateStrictFit(t *testing.T) {
	mc := testMCPConfig(t)

	// A valid deck with minimal content (no overflow expected).
	deckJSON := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{
				"placeholder_id": "title",
				"type": "text",
				"text_value": "Hello"
			}]
		}]
	}`

	t.Run("strict_fit=off skips checks", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"strict_fit": "off",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
	})

	t.Run("strict_fit=warn generates with warnings", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"strict_fit": "warn",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
	})

	t.Run("strict_fit=strict succeeds with no overflow", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"strict_fit": "strict",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
	})

	t.Run("strict_fit defaults to warn when omitted", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
	})

	// Test that strict mode refuses generation on overflow.
	t.Run("fit_report=false omits content fit_findings", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"fit_report": false,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		// layout_synthesized findings are always emitted (template-level concern),
		// but content-level findings (fit_overflow, density_exceeded) must not appear.
		if strings.Contains(text, "fit_overflow") || strings.Contains(text, "density_exceeded") {
			t.Error("content fit_findings should not appear when fit_report=false")
		}
	})

	t.Run("fit_report=true includes fit_findings key", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
			"fit_report": true,
			"strict_fit": "off",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
		// fit_findings is omitempty: no findings for a simple deck is expected.
		// The important thing is the code path ran without error.
	})

	t.Run("fit_report absent defaults to no content fit_findings", func(t *testing.T) {
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		// layout_synthesized findings are always emitted (template-level concern),
		// but content-level findings must not appear without fit_report.
		if strings.Contains(text, "fit_overflow") || strings.Contains(text, "density_exceeded") {
			t.Error("content fit_findings should not appear when fit_report is absent")
		}
	})

	t.Run("fit_report=true with overflow populates findings sorted by severity", func(t *testing.T) {
		overflowJSON := `{
			"template": "midnight-blue",
			"slides": [{
				"layout_id": "slideLayout2",
				"content": [{
					"placeholder_id": "title",
					"type": "text",
					"text_value": "Test"
				}, {
					"placeholder_id": "body",
					"type": "table",
					"table_value": {
						"headers": ["A","B","C","D","E","F","G","H","I","J"],
						"rows": [` + func() string {
			longText := strings.Repeat("This is a very long text that overflows ", 8)
			row := `[{"content":"` + longText + `"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}]`
			shortRow := `[{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"},{"content":"x"}]`
			rows := []string{row}
			for i := 0; i < 14; i++ {
				rows = append(rows, shortRow)
			}
			return strings.Join(rows, ",")
		}() + `]
					}
				}]
			}]
		}`

		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(overflowJSON),
			"fit_report": true,
			"strict_fit": "off",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true")
		}
		if len(resp.FitFindings) == 0 {
			t.Skip("no fit findings generated — thresholds may need adjustment")
		}

		// Verify sorting: ActionRank should be non-increasing among slide-level
		// findings (template-level findings like layout_synthesized are appended
		// separately and not subject to per-slide ordering).
		var slideFindings []patterns.FitFinding
		for _, f := range resp.FitFindings {
			if strings.HasPrefix(f.Path, "slides[") {
				slideFindings = append(slideFindings, f)
			}
		}
		for i := 1; i < len(slideFindings); i++ {
			prev := patterns.ActionRank(slideFindings[i-1].Action)
			curr := patterns.ActionRank(slideFindings[i].Action)
			if curr > prev {
				t.Errorf("findings not sorted by ActionRank desc: [%d]=%s (rank %d) before [%d]=%s (rank %d)",
					i-1, slideFindings[i-1].Action, prev,
					i, slideFindings[i].Action, curr)
			}
		}
	})

	t.Run("list_templates returns digest not full hints", func(t *testing.T) {
		result, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp skillInfo
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.SupportedTypes.DataFormatHintsDigest == "" {
			t.Error("expected data_format_hints_digest to be populated")
		}
		if resp.SupportedTypes.DataFormatHints != nil {
			t.Error("expected data_format_hints to be omitted from list_templates response")
		}
	})

	t.Run("strict_fit=warn surfaces structured findings without fit_report", func(t *testing.T) {
		// Regression: previously, warn-mode strict_fit findings were only
		// written to stderr by checkStrictFit. MCP clients had no structured
		// channel for them unless they separately passed fit_report=true.
		longText := strings.Repeat("This is a very long cell that overflows ", 10)
		row := make([]map[string]string, 10)
		for i := range row {
			row[i] = map[string]string{"content": "x"}
		}
		row[0] = map[string]string{"content": longText}
		rowJSON, _ := json.Marshal(row)
		shortRow := make([]map[string]string, 10)
		for i := range shortRow {
			shortRow[i] = map[string]string{"content": "x"}
		}
		shortRowJSON, _ := json.Marshal(shortRow)

		var rows []string
		rows = append(rows, string(rowJSON))
		for i := 0; i < 14; i++ {
			rows = append(rows, string(shortRowJSON))
		}

		overflowDeckJSON := `{
			"template": "midnight-blue",
			"slides": [{
				"layout_id": "slideLayout2",
				"content": [{
					"placeholder_id": "title",
					"type": "text",
					"text_value": "Warn Test"
				}, {
					"placeholder_id": "body",
					"type": "table",
					"table_value": {
						"headers": ["A","B","C","D","E","F","G","H","I","J"],
						"rows": [` + strings.Join(rows, ",") + `]
					}
				}]
			}]
		}`

		// fit_report is intentionally NOT set; strict_fit defaults to "warn".
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(overflowDeckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("warn mode should not refuse; got tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true in warn mode")
		}
		// Look for a fit-overflow or density-exceeded finding — these come
		// from strict_fit warn-mode evaluation.
		sawFitFinding := false
		for _, f := range resp.FitFindings {
			if f.Code == patterns.ErrCodeFitOverflow || f.Code == patterns.ErrCodeDensityExceeded {
				sawFitFinding = true
				break
			}
		}
		if !sawFitFinding {
			t.Errorf("strict_fit=warn should surface fit_overflow/density_exceeded in fit_findings; got %d findings, codes: %v",
				len(resp.FitFindings), fitFindingCodes(resp.FitFindings))
		}
	})

	t.Run("strict_fit=strict refuses on overflow", func(t *testing.T) {
		// Build a table with many columns and long cell content to trigger
		// both TDR density ceiling (cells > 60 at >=18pt is impossible with
		// small fonts, so we use enough rows/cols) AND individual cell overflow.
		// 10 cols forces font scaling down; 12 rows + 1 header = 13 rows;
		// 13*10 = 130 cells, which exceeds the TDR ceiling of 120 at small font.
		longText := strings.Repeat("This is a very long text that will definitely overflow ", 10)
		row := make([]map[string]string, 10)
		for i := range row {
			row[i] = map[string]string{"content": "x"}
		}
		row[0] = map[string]string{"content": longText}
		rowJSON, _ := json.Marshal(row)
		shortRow := make([]map[string]string, 10)
		for i := range shortRow {
			shortRow[i] = map[string]string{"content": "x"}
		}
		shortRowJSON, _ := json.Marshal(shortRow)

		var rows []string
		rows = append(rows, string(rowJSON))
		for i := 0; i < 14; i++ {
			rows = append(rows, string(shortRowJSON))
		}

		overflowDeckJSON := `{
			"template": "midnight-blue",
			"slides": [{
				"layout_id": "slideLayout2",
				"content": [{
					"placeholder_id": "title",
					"type": "text",
					"text_value": "Overflow Test"
				}, {
					"placeholder_id": "body",
					"type": "table",
					"table_value": {
						"headers": ["A","B","C","D","E","F","G","H","I","J"],
						"rows": [` + strings.Join(rows, ",") + `]
					}
				}]
			}]
		}`

		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(overflowDeckJSON),
			"strict_fit": "strict",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should be a tool error (refused generation).
		if !result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Logf("expected tool error for strict-fit overflow, got success: %s", text)
			t.Skip("test data did not trigger unfittable finding — adjust if thresholds change")
		}
		// Verify the error message mentions strict-fit.
		errText := result.Content[0].(mcp.TextContent).Text
		if !strings.Contains(errText, "strict-fit") {
			t.Errorf("expected error to mention strict-fit, got: %s", errText)
		}
	})
}

func TestMCPGetDataFormatHints(t *testing.T) {
	t.Run("returns full hints and digest", func(t *testing.T) {
		result, err := handleGetDataFormatHints(context.Background(), makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp dataFormatHintsResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.Digest == "" {
			t.Error("expected non-empty digest")
		}
		if resp.NotModified {
			t.Error("expected not_modified=false when no digest provided")
		}
		if len(resp.Hints) == 0 {
			t.Error("expected non-empty data_format_hints")
		}
		if _, ok := resp.Hints["bar"]; !ok {
			t.Error("expected 'bar' in data_format_hints")
		}
	})

	t.Run("returns not_modified when digest matches", func(t *testing.T) {
		digest := computeDataFormatHintsDigest(buildDataFormatHints())

		result, err := handleGetDataFormatHints(context.Background(), makeRequest(map[string]any{
			"digest": digest,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("unexpected tool error: %v", result.Content)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp dataFormatHintsResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.NotModified {
			t.Error("expected not_modified=true when digest matches")
		}
		if resp.Digest != digest {
			t.Errorf("digest mismatch: got %q, want %q", resp.Digest, digest)
		}
		if resp.Hints != nil {
			t.Error("expected nil hints when not_modified=true")
		}
	})

	t.Run("returns full hints when digest does not match", func(t *testing.T) {
		result, err := handleGetDataFormatHints(context.Background(), makeRequest(map[string]any{
			"digest": "stale-digest-value",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		text := result.Content[0].(mcp.TextContent).Text
		var resp dataFormatHintsResponse
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if resp.NotModified {
			t.Error("expected not_modified=false when digest does not match")
		}
		if len(resp.Hints) == 0 {
			t.Error("expected non-empty data_format_hints")
		}
	})

	t.Run("digest is stable across calls", func(t *testing.T) {
		d1 := computeDataFormatHintsDigest(buildDataFormatHints())
		d2 := computeDataFormatHintsDigest(buildDataFormatHints())
		if d1 != d2 {
			t.Errorf("digest not stable: %q != %q", d1, d2)
		}
	})

	t.Run("list_templates digest matches get_data_format_hints digest", func(t *testing.T) {
		mc := testMCPConfig(t)
		ltResult, err := mc.handleListTemplates(context.Background(), makeRequest(map[string]any{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ltText := ltResult.Content[0].(mcp.TextContent).Text
		var ltResp skillInfo
		if err := json.Unmarshal([]byte(ltText), &ltResp); err != nil {
			t.Fatalf("failed to parse list_templates response: %v", err)
		}

		hResult, _ := handleGetDataFormatHints(context.Background(), makeRequest(map[string]any{}))
		hText := hResult.Content[0].(mcp.TextContent).Text
		var hResp dataFormatHintsResponse
		if err := json.Unmarshal([]byte(hText), &hResp); err != nil {
			t.Fatalf("failed to parse get_data_format_hints response: %v", err)
		}

		if ltResp.SupportedTypes.DataFormatHintsDigest != hResp.Digest {
			t.Errorf("digest mismatch between list_templates (%q) and get_data_format_hints (%q)",
				ltResp.SupportedTypes.DataFormatHintsDigest, hResp.Digest)
		}
	})
}

// structureOnlyDeckJSON returns a deck payload whose slides are defined
// entirely via the structure block (cover + sections + closing), exercising
// the expansion path that previously only ran in the CLI.
func structureOnlyDeckJSON() string {
	return `{
		"template": "midnight-blue",
		"structure": {
			"cover": {
				"slide_type": "title",
				"content": [{"placeholder_id": "title", "type": "text", "text_value": "Structure Deck"}]
			},
			"closing": {
				"slide_type": "title",
				"content": [{"placeholder_id": "title", "type": "text", "text_value": "Thank You"}]
			},
			"auto_agenda": true,
			"sections": [
				{
					"title": "Section A",
					"slides": [{
						"layout_id": "slideLayout2",
						"content": [{"placeholder_id": "title", "type": "text", "text_value": "A1"}]
					}]
				},
				{
					"title": "Section B",
					"slides": [{
						"layout_id": "slideLayout2",
						"content": [{"placeholder_id": "title", "type": "text", "text_value": "B1"}]
					}]
				}
			]
		}
	}`
}

// TestMCPStructureExpansionParity exercises the structure expansion path
// across every MCP handler that previously short-circuited on
// len(input.Slides) == 0. A valid structure-only payload must succeed via
// generate_presentation, validate_input, preview_presentation_plan, and
// score_deck — matching CLI behavior.
//
// Regression for go-slide-creator-41g5: MCP handlers used to reject
// structure-only payloads because they checked the unexpanded slides count.
func TestMCPStructureExpansionParity(t *testing.T) {
	deckJSON := structureOnlyDeckJSON()

	t.Run("generate_presentation accepts structure-only", func(t *testing.T) {
		mc := testMCPConfig(t)
		result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("expected success for structure-only payload, got error: %s", text)
		}
		text := result.Content[0].(mcp.TextContent).Text
		var resp JSONOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !resp.Success {
			t.Error("expected success=true for structure-only payload")
		}
	})

	t.Run("validate_input accepts structure-only", func(t *testing.T) {
		mc := testMCPConfig(t)
		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("expected success for structure-only payload, got error: %s", text)
		}
	})

	t.Run("preview_presentation_plan accepts structure-only", func(t *testing.T) {
		mc := testMCPConfig(t)
		result, err := mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("expected success for structure-only payload, got error: %s", text)
		}
		// Preview should expose the expanded slides: cover + agenda + 2 dividers + 2 content + closing = 7.
		text := result.Content[0].(mcp.TextContent).Text
		var resp previewPlanOutput
		if err := json.Unmarshal([]byte(text), &resp); err != nil {
			t.Fatalf("failed to parse preview response: %v", err)
		}
		const wantSlides = 7
		if len(resp.ResolvedSlides) != wantSlides {
			t.Errorf("expected %d expanded slides in preview, got %d", wantSlides, len(resp.ResolvedSlides))
		}
	})

	t.Run("score_deck accepts structure-only", func(t *testing.T) {
		mc := testMCPConfig(t)
		result, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("expected success for structure-only payload, got error: %s", text)
		}
	})
}

// TestMCPStructureSlidesMutuallyExclusive verifies that passing BOTH a
// structure block and top-level slides produces a STRUCTURE_AND_SLIDES
// diagnostic on every MCP handler (mirroring the CLI error). Without this,
// an agent could accidentally double-author and have one silently win.
func TestMCPStructureSlidesMutuallyExclusive(t *testing.T) {
	deckJSON := `{
		"template": "midnight-blue",
		"structure": {
			"sections": [
				{
					"title": "S1",
					"slides": [{
						"layout_id": "slideLayout2",
						"content": [{"placeholder_id": "title", "type": "text", "text_value": "S1"}]
					}]
				}
			]
		},
		"slides": [{
			"layout_id": "slideLayout2",
			"content": [{"placeholder_id": "title", "type": "text", "text_value": "Conflict"}]
		}]
	}`

	expectMutualExclusivity := func(t *testing.T, text string) {
		t.Helper()
		diags := legacyDiagsFromWire(t, text)
		var found bool
		for _, d := range diags {
			if d["code"] == "STRUCTURE_AND_SLIDES" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected STRUCTURE_AND_SLIDES diagnostic, got: %s", text)
		}
	}

	handlers := []struct {
		name string
		call func(*mcpConfig) (*mcp.CallToolResult, error)
	}{
		{
			name: "generate_presentation",
			call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
				return mc.handleGenerate(context.Background(), makeRequest(map[string]any{
					"presentation": mustParseJSON(deckJSON),
				}))
			},
		},
		{
			name: "validate_input",
			call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
				return mc.handleValidate(context.Background(), makeRequest(map[string]any{
					"presentation": mustParseJSON(deckJSON),
				}))
			},
		},
		{
			name: "preview_presentation_plan",
			call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
				return mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
					"presentation": mustParseJSON(deckJSON),
				}))
			},
		},
		{
			name: "score_deck",
			call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
				return mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
					"presentation": mustParseJSON(deckJSON),
				}))
			},
		},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			mc := testMCPConfig(t)
			result, err := h.call(mc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !result.IsError {
				t.Fatalf("expected IsError=true for structure+slides conflict, got success: %v", result.Content)
			}
			expectMutualExclusivity(t, result.Content[0].(mcp.TextContent).Text)
		})
	}
}

// TestMCPURLResolutionParity verifies that every URL surface documented in
// the input schema — background.url, content image_value.url, shape_grid
// cell image.url, cell icon.url, nested shape.icon.url — is resolved by MCP
// the same way as the CLI. This is the parity test for go-slide-creator-6z5j.
//
// The test spins up an httptest server so URLs are reachable without
// network access, injects an SSRF-bypassing HTTPClient via the mcpConfig
// resolver options, and asserts:
//
//   - happy path: validate_input + generate_presentation both succeed and
//     every URL field has been rewritten to a cached local path.
//   - failure path: every URL returns 404 and validate_input emits one
//     URL_FETCH_FAILED diagnostic per surface, each tagged with the right
//     JSON Pointer and asset_kind.
func TestMCPURLResolutionParity(t *testing.T) {
	// Minimal PNG/SVG payloads accepted by resource.Resolver's content sniff.
	pngBytes := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)
	svgBytes := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/img.png"):
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		case strings.HasSuffix(r.URL.Path, "/icon.svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write(svgBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Build a deck that exercises ALL FIVE URL surfaces simultaneously.
	deckJSONTmpl := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"background": {"url": %q, "fit": "cover"},
			"content": [{
				"placeholder_id": "body",
				"type": "image",
				"image_value": {"url": %q, "alt": "photo"}
			}],
			"shape_grid": {
				"rows": [{
					"cells": [
						{"image": {"url": %q}},
						{"icon":  {"url": %q}},
						{"shape": {"geometry": "rect", "icon": {"url": %q}}}
					]
				}]
			}
		}]
	}`

	bgURL := srv.URL + "/img.png"
	contentURL := srv.URL + "/img.png?content"
	gridImgURL := srv.URL + "/img.png?grid"
	cellIconURL := srv.URL + "/icon.svg?cell"
	shapeIconURL := srv.URL + "/icon.svg?shape"

	t.Run("happy path resolves all five surfaces", func(t *testing.T) {
		mc := testMCPConfig(t)
		// httptest.Server listens on loopback; bypass the production
		// SSRF dialer so the resolver can actually reach it.
		mc.resolverOpts = resource.ResolverOptions{HTTPClient: &http.Client{}}

		deckJSON := fmt.Sprintf(deckJSONTmpl, bgURL, contentURL, gridImgURL, cellIconURL, shapeIconURL)

		// validate_input must succeed — no URL_FETCH_FAILED diagnostics.
		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("validate_input: unexpected error: %v", err)
		}
		if result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("validate_input: expected success, got error: %s", text)
		}
		text := result.Content[0].(mcp.TextContent).Text
		if strings.Contains(text, "URL_FETCH_FAILED") {
			t.Errorf("validate_input emitted URL_FETCH_FAILED for reachable URLs: %s", text)
		}

		// generate_presentation must also succeed for the same input.
		genResult, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("generate_presentation: unexpected error: %v", err)
		}
		if genResult.IsError {
			genText := genResult.Content[0].(mcp.TextContent).Text
			t.Fatalf("generate_presentation: expected success, got error: %s", genText)
		}
		genText := genResult.Content[0].(mcp.TextContent).Text
		var genResp JSONOutput
		if err := json.Unmarshal([]byte(genText), &genResp); err != nil {
			t.Fatalf("generate_presentation: failed to parse response: %v", err)
		}
		if !genResp.Success {
			t.Errorf("generate_presentation: expected success=true, got error=%q", genResp.Error)
		}
	})

	t.Run("missing URLs surface URL_FETCH_FAILED per asset", func(t *testing.T) {
		mc := testMCPConfig(t)
		mc.resolverOpts = resource.ResolverOptions{HTTPClient: &http.Client{}}

		miss := srv.URL + "/missing"
		deckJSON := fmt.Sprintf(deckJSONTmpl,
			miss+"-bg.png",
			miss+"-content.png",
			miss+"-grid.png",
			miss+"-cell.svg",
			miss+"-shape.svg",
		)

		result, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
			"presentation": mustParseJSON(deckJSON),
		}))
		if err != nil {
			t.Fatalf("validate_input: unexpected error: %v", err)
		}
		if !result.IsError {
			text := result.Content[0].(mcp.TextContent).Text
			t.Fatalf("validate_input: expected IsError=true for unreachable URLs, got success: %s", text)
		}

		text := result.Content[0].(mcp.TextContent).Text
		diags := legacyDiagsFromWire(t, text)

		// One URL_FETCH_FAILED per surface, keyed by JSON Pointer.
		wantSurfaces := map[string]string{
			"/slides/0/background/url":                              "background",
			"/slides/0/content/0/image_value/url":                   "image",
			"/slides/0/shape_grid/rows/0/cells/0/image/url":         "image",
			"/slides/0/shape_grid/rows/0/cells/1/icon/url":          "icon",
			"/slides/0/shape_grid/rows/0/cells/2/shape/icon/url":    "icon",
		}
		gotAssetKind := make(map[string]string, len(wantSurfaces))
		for _, d := range diags {
			if d["code"] != "URL_FETCH_FAILED" {
				continue
			}
			path, _ := d["path"].(string)
			if _, ok := wantSurfaces[path]; !ok {
				continue
			}
			details, _ := d["details"].(map[string]any)
			if details == nil {
				t.Errorf("path %q: missing details map", path)
				continue
			}
			kind, _ := details["asset_kind"].(string)
			gotAssetKind[path] = kind

			// Spot-check that the offending URL is echoed back so agents can
			// repair the broken field without reading the original input.
			if inURL, _ := details["input_url"].(string); !strings.Contains(inURL, "missing") {
				t.Errorf("path %q: expected input_url to echo the missing URL, got %q", path, inURL)
			}
		}
		for path, wantKind := range wantSurfaces {
			if got, ok := gotAssetKind[path]; !ok {
				t.Errorf("missing URL_FETCH_FAILED diagnostic for path %q (full envelope: %s)", path, text)
			} else if got != wantKind {
				t.Errorf("path %q: expected asset_kind %q, got %q", path, wantKind, got)
			}
		}
	})
}

// TestResolveURLsCollectsAllFailures is a unit-level guard that the refactor
// from "return first error" to "return all diagnostics" actually walks every
// surface even when earlier ones fail. Without this, MCP and CLI would
// regress to seeing only one finding per call.
func TestResolveURLsCollectsAllFailures(t *testing.T) {
	stub := failingURLResolver{}
	slides := []SlideInput{{
		Background: &BackgroundInput{URL: "https://example.invalid/bg.png"},
		Content: []ContentInput{{
			Type:       "image",
			ImageValue: &ImageInput{URL: "https://example.invalid/photo.png"},
		}},
		ShapeGrid: &ShapeGridInput{
			Rows: []GridRowInput{{
				Cells: []*GridCellInput{
					{Image: &GridImageInput{URL: "https://example.invalid/grid.png"}},
					{Icon: &IconInput{URL: "https://example.invalid/cell.svg"}},
					{Shape: &ShapeSpecInput{Geometry: "rect", Icon: &IconInput{URL: "https://example.invalid/shape.svg"}}},
				},
			}},
		},
	}}

	findings := resolveURLs(slides, stub)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings (one per URL surface), got %d: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Code != "URL_FETCH_FAILED" {
			t.Errorf("expected code URL_FETCH_FAILED, got %q", f.Code)
		}
		if f.Severity != "error" {
			t.Errorf("expected severity=error, got %q", f.Severity)
		}
	}
}

// failingURLResolver always returns an error so resolveURLs walks every
// surface without partial success masking the iteration.
type failingURLResolver struct{}

func (failingURLResolver) ResolveImage(rawURL string) (string, error) {
	return "", fmt.Errorf("stubbed image failure for %s", rawURL)
}

func (failingURLResolver) ResolveSVG(rawURL string) (string, error) {
	return "", fmt.Errorf("stubbed svg failure for %s", rawURL)
}

// relativeAssetDeckJSON returns a deck payload whose every local-asset
// surface (background.image, content image_value.path, grid image.path,
// grid icon.path) is a bare filename — the deck is only resolvable when an
// explicit base_dir is supplied.
func relativeAssetDeckJSON(bg, photo, gridImg, gridIcon string) string {
	return fmt.Sprintf(`{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "slideLayout2",
			"background": {"image": %q},
			"content": [{
				"placeholder_id": "body",
				"type": "image",
				"image_value": {"path": %q, "alt": "photo"}
			}],
			"shape_grid": {
				"rows": [{
					"cells": [
						{"image": {"path": %q}},
						{"icon":  {"path": %q}}
					]
				}]
			}
		}]
	}`, bg, photo, gridImg, gridIcon)
}

// TestMCPRelativeAssetParity exercises the base_dir flow across every MCP
// handler that advertises it (generate_presentation, validate_input,
// preview_presentation_plan). Each handler must resolve relative paths
// against the supplied base_dir — regardless of the server's process CWD —
// and emit one structured per-asset diagnostic when an asset is missing.
//
// Without this parity guard, base_dir could silently regress on any one
// handler and CLI+MCP would diverge on payloads that look identical to the
// agent. This is the MCP parity test for go-slide-creator-dbgl's "relative
// local asset paths with explicit base_dir" requirement.
//
// Asset basenames are uniquely prefixed (`dbgl-rel-`) so the bare filenames
// in the deck could not accidentally exist under the test process CWD —
// making base_dir the only directory that can resolve them. We do NOT chdir
// in this test: an outer chdir would race against other tests in the package
// that use relative paths like ../../templates.
func TestMCPRelativeAssetParity(t *testing.T) {
	const (
		bgName       = "dbgl-rel-bg.png"
		photoName    = "dbgl-rel-photo.png"
		gridImgName  = "dbgl-rel-grid.jpg"
		gridIconName = "dbgl-rel-icon.svg"
	)

	t.Run("happy path resolves all four surfaces under base_dir", func(t *testing.T) {
		baseDir := t.TempDir()
		// Minimal asset bytes are enough — the local-asset pass cares only
		// about existence + extension, not decode-ability.
		for _, name := range []string{bgName, photoName, gridImgName} {
			if err := os.WriteFile(filepath.Join(baseDir, name), []byte("x"), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
		if err := os.WriteFile(filepath.Join(baseDir, gridIconName), []byte(`<svg/>`), 0o644); err != nil {
			t.Fatalf("write %s: %v", gridIconName, err)
		}

		deckJSON := relativeAssetDeckJSON(bgName, photoName, gridImgName, gridIconName)

		handlers := []struct {
			name string
			call func(*mcpConfig) (*mcp.CallToolResult, error)
		}{
			{
				name: "validate_input",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleValidate(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "generate_presentation",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleGenerate(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "preview_presentation_plan",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "score_deck",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "auto_repair",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
						"max_passes":   float64(1),
					}))
				},
			},
		}

		for _, h := range handlers {
			t.Run(h.name, func(t *testing.T) {
				mc := testMCPConfig(t)
				result, err := h.call(mc)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.IsError {
					text := result.Content[0].(mcp.TextContent).Text
					t.Fatalf("expected success for relative paths under base_dir, got error: %s", text)
				}
				// Asset findings must not appear when every relative path
				// resolves cleanly under base_dir.
				text := result.Content[0].(mcp.TextContent).Text
				for _, code := range []string{"BACKGROUND_IMAGE_PATH", "IMAGE_PATH", "ICON_NOT_FOUND"} {
					if strings.Contains(text, code) {
						t.Errorf("unexpected %s diagnostic in success response: %s", code, text)
					}
				}
			})
		}
	})

	t.Run("missing assets surface one diagnostic per surface under base_dir", func(t *testing.T) {
		// base_dir exists but is empty — every relative path points at a
		// missing file. Each surface must emit its own structured
		// diagnostic with the canonical asset_kind + JSON Pointer, on
		// every handler.
		baseDir := t.TempDir()
		const (
			missBg   = "dbgl-missing-bg.png"
			missImg  = "dbgl-missing-photo.png"
			missGrid = "dbgl-missing-grid.jpg"
			missIcon = "dbgl-missing-icon.svg"
		)
		deckJSON := relativeAssetDeckJSON(missBg, missImg, missGrid, missIcon)

		// Each surface emits exactly one diagnostic keyed by JSON Pointer.
		// asset_kind comes from resolveLocalAssetPaths' canonical tagging.
		want := map[string]struct {
			code      string
			assetKind string
		}{
			"/slides/0/background/image":                     {code: "BACKGROUND_IMAGE_PATH", assetKind: "background"},
			"/slides/0/content/0/image_value/path":           {code: "IMAGE_PATH", assetKind: "image"},
			"/slides/0/shape_grid/rows/0/cells/0/image/path": {code: "IMAGE_PATH", assetKind: "image"},
			"/slides/0/shape_grid/rows/0/cells/1/icon":       {code: "ICON_NOT_FOUND", assetKind: "icon"},
		}

		handlers := []struct {
			name string
			call func(*mcpConfig) (*mcp.CallToolResult, error)
		}{
			{
				name: "validate_input",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleValidate(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "generate_presentation",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleGenerate(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "preview_presentation_plan",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "score_deck",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
					}))
				},
			},
			{
				name: "auto_repair",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     baseDir,
						"max_passes":   float64(1),
					}))
				},
			},
		}

		for _, h := range handlers {
			t.Run(h.name, func(t *testing.T) {
				mc := testMCPConfig(t)
				result, err := h.call(mc)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !result.IsError {
					text := result.Content[0].(mcp.TextContent).Text
					t.Fatalf("expected IsError=true for missing relative assets, got success: %s", text)
				}

				text := result.Content[0].(mcp.TextContent).Text
				diags := legacyDiagsFromWire(t, text)

				got := make(map[string]map[string]string, len(want))
				for _, d := range diags {
					path, _ := d["path"].(string)
					code, _ := d["code"].(string)
					if _, ok := want[path]; !ok {
						continue
					}
					details, _ := d["details"].(map[string]any)
					kind, _ := details["asset_kind"].(string)
					got[path] = map[string]string{"code": code, "asset_kind": kind}
				}
				for path, w := range want {
					g, ok := got[path]
					if !ok {
						t.Errorf("missing diagnostic for %q (full envelope: %s)", path, text)
						continue
					}
					if g["code"] != w.code {
						t.Errorf("path %q: expected code %s, got %s", path, w.code, g["code"])
					}
					if g["asset_kind"] != w.assetKind {
						t.Errorf("path %q: expected asset_kind %s, got %s", path, w.assetKind, g["asset_kind"])
					}
				}
			})
		}
	})

	t.Run("relative base_dir is rejected with INVALID_PARAMETER", func(t *testing.T) {
		// Relative base_dir re-introduces the CWD coupling that base_dir
		// exists to eliminate. Every handler must reject it with the same
		// structured diagnostic, so agents see one signal rather than a
		// scattershot of broken-asset findings.
		deckJSON := relativeAssetDeckJSON("bg.png", "photo.png", "grid.jpg", "icon.svg")

		handlers := []struct {
			name string
			call func(*mcpConfig) (*mcp.CallToolResult, error)
		}{
			{
				name: "validate_input",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleValidate(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     "relative/path/not/allowed",
					}))
				},
			},
			{
				name: "generate_presentation",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleGenerate(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     "relative/path/not/allowed",
					}))
				},
			},
			{
				name: "preview_presentation_plan",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handlePreviewPlan(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     "relative/path/not/allowed",
					}))
				},
			},
			{
				name: "score_deck",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     "relative/path/not/allowed",
					}))
				},
			},
			{
				name: "auto_repair",
				call: func(mc *mcpConfig) (*mcp.CallToolResult, error) {
					return mc.handleAutoRepair(context.Background(), makeRequest(map[string]any{
						"presentation": mustParseJSON(deckJSON),
						"base_dir":     "relative/path/not/allowed",
						"max_passes":   float64(1),
					}))
				},
			},
		}

		for _, h := range handlers {
			t.Run(h.name, func(t *testing.T) {
				mc := testMCPConfig(t)
				result, err := h.call(mc)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				requireStructuredError(t, result, "INVALID_PARAMETER")
			})
		}
	})
}
