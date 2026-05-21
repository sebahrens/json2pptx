package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/template"
)

// TestMCPExamineTemplate verifies the examine_template MCP tool returns the full
// examine.Report inline (as structured content) for a bundled template, with the
// canonical coverage, layouts, and findings envelope an agent depends on — and
// without writing any artifact directory (MCP mode is side-effect-free).
func TestMCPExamineTemplate(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	text := result.Content[0].(mcp.TextContent).Text
	var report examine.Report
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("failed to parse report: %v", err)
	}

	if report.Template == "" {
		t.Error("expected non-empty template name in report")
	}
	if report.Slide.WidthEMU == 0 || report.Slide.HeightEMU == 0 {
		t.Errorf("expected non-zero slide dimensions, got %+v", report.Slide)
	}
	if len(report.Theme.Colors) == 0 {
		t.Error("expected non-empty theme colors")
	}
	if len(report.Layouts) == 0 {
		t.Error("expected at least one layout in the report")
	}

	// Canonical coverage must enumerate the four content-bearing families.
	for _, fam := range []string{"title-slide", "section-divider", "one-content", "qa-closing"} {
		if _, ok := report.CanonicalCoverage[fam]; !ok {
			t.Errorf("canonical_coverage missing family %q", fam)
		}
	}

	// The findings envelope is always present and stamped with the
	// examine-template subcommand.
	if report.Findings.SchemaVersion == "" {
		t.Error("expected findings envelope to be present (schema_version set)")
	}
	if report.Findings.Subcommand != examine.Subcommand {
		t.Errorf("findings.subcommand = %q, want %q", report.Findings.Subcommand, examine.Subcommand)
	}
	if report.Findings.Findings == nil {
		t.Error("findings.findings must be non-nil (may be empty)")
	}
}

// TestMCPExamineTemplateParity confirms the MCP handler returns the identical
// report the reusable examine.Examine service produces for the same template —
// the CLI and MCP surfaces share that core, so the inline MCP report must match
// what the CLI would write to report.json.
func TestMCPExamineTemplateParity(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	mcpJSON := result.Content[0].(mcp.TextContent).Text

	// Build the same report directly through the reusable service.
	templatePath, cleanup, err := resolveTemplatePath("midnight-blue", mc.templatesDir)
	if err != nil {
		t.Fatalf("resolve template: %v", err)
	}
	defer cleanup()
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() { _ = reader.Close() }()
	report, err := examine.Examine(reader, examine.Options{TemplatePath: templatePath})
	if err != nil {
		t.Fatalf("examine: %v", err)
	}
	wantJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	// Compare via normalized maps so field ordering does not matter.
	var gotMap, wantMap map[string]any
	if err := json.Unmarshal([]byte(mcpJSON), &gotMap); err != nil {
		t.Fatalf("parse MCP report: %v", err)
	}
	if err := json.Unmarshal(wantJSON, &wantMap); err != nil {
		t.Fatalf("parse service report: %v", err)
	}
	gotNorm, _ := json.Marshal(gotMap)
	wantNorm, _ := json.Marshal(wantMap)
	if string(gotNorm) != string(wantNorm) {
		t.Errorf("MCP report differs from examine.Examine output:\n MCP: %s\n svc: %s", gotNorm, wantNorm)
	}
}

// TestMCPExamineTemplateUnknown verifies an unknown template name produces a
// structured TEMPLATE_NOT_FOUND error rather than a panic or success.
func TestMCPExamineTemplateUnknown(t *testing.T) {
	mc := testMCPConfig(t)

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_name": "no-such-template-xyz",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for unknown template, got success: %v", result.Content)
	}
}
