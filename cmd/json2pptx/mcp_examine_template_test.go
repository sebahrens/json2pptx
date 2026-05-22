package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/examine"
	"github.com/sebahrens/json2pptx/internal/template"
)

// copyBundledTemplate resolves a bundled template by name and copies its bytes
// to dst, returning dst. It is the fixture builder for the guarded
// template_path tests: a real .pptx on disk that examine_template can open.
func copyBundledTemplate(t *testing.T, mc *mcpConfig, name, dst string) string {
	t.Helper()
	src, cleanup, err := resolveTemplatePath(name, mc.templatesDir)
	if err != nil {
		t.Fatalf("resolve bundled template %q: %v", name, err)
	}
	defer cleanup()
	data, err := os.ReadFile(src) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("read bundled template %q: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil { //nolint:gosec // test fixture output
		t.Fatalf("write template copy %q: %v", dst, err)
	}
	return dst
}

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

// TestMCPExamineTemplatePathAllowed verifies the guarded template_path form
// inspects a local .pptx that sits inside base_dir — the parity that lets an
// MCP-only agent examine a not-yet-registered template file.
func TestMCPExamineTemplatePathAllowed(t *testing.T) {
	mc := testMCPConfig(t)
	dir := t.TempDir()
	copyBundledTemplate(t, mc, "midnight-blue", filepath.Join(dir, "candidate.pptx"))

	// Relative template_path resolves against base_dir.
	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_path": "candidate.pptx",
		"base_dir":      dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success for an allowed path inside base_dir, got error: %v", result.Content)
	}

	var report examine.Report
	if uerr := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &report); uerr != nil {
		t.Fatalf("failed to parse report: %v", uerr)
	}
	if report.Template != "candidate.pptx" {
		t.Errorf("report.Template = %q, want %q", report.Template, "candidate.pptx")
	}
	if len(report.Layouts) == 0 {
		t.Error("expected at least one layout in the path-form report")
	}
}

// TestMCPExamineTemplatePathForbiddenOutsideBase verifies an absolute path that
// resolves outside base_dir is rejected with a clear INVALID_PATH forbidden-path
// diagnostic, even though the file exists.
func TestMCPExamineTemplatePathForbiddenOutsideBase(t *testing.T) {
	mc := testMCPConfig(t)
	allowed := t.TempDir()
	outside := t.TempDir()
	victim := copyBundledTemplate(t, mc, "midnight-blue", filepath.Join(outside, "victim.pptx"))

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_path": victim, // absolute, exists, but outside `allowed`
		"base_dir":      allowed,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for a path outside base_dir, got success: %v", result.Content)
	}
	env := parseMCPError(t, result)
	if env.Diagnostics[0].Code != diagnostics.CodeInvalidPath {
		t.Errorf("expected code=%q, got %q (msg=%q)", diagnostics.CodeInvalidPath, env.Diagnostics[0].Code, env.Diagnostics[0].Message)
	}
	if env.Diagnostics[0].Path != "template_path" {
		t.Errorf("expected path=template_path, got %q", env.Diagnostics[0].Path)
	}
}

// TestMCPExamineTemplatePathTraversal verifies a raw ".." traversal is rejected
// before it can escape base_dir.
func TestMCPExamineTemplatePathTraversal(t *testing.T) {
	mc := testMCPConfig(t)
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("mkdir child: %v", err)
	}
	// Place a real template in the parent so only the ".." guard — not a
	// missing file — can be the reason for rejection.
	copyBundledTemplate(t, mc, "midnight-blue", filepath.Join(parent, "escape.pptx"))

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_path": "../escape.pptx",
		"base_dir":      child,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for a '..' traversal, got success: %v", result.Content)
	}
	env := parseMCPError(t, result)
	if env.Diagnostics[0].Code != diagnostics.CodeInvalidPath {
		t.Errorf("expected code=%q, got %q (msg=%q)", diagnostics.CodeInvalidPath, env.Diagnostics[0].Code, env.Diagnostics[0].Message)
	}
}

// TestMCPExamineTemplatePathNotFound verifies a missing file inside base_dir
// surfaces FILE_NOT_FOUND (distinct from the forbidden-path case).
func TestMCPExamineTemplatePathNotFound(t *testing.T) {
	mc := testMCPConfig(t)
	dir := t.TempDir()

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_path": "missing.pptx",
		"base_dir":      dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for a missing file, got success: %v", result.Content)
	}
	env := parseMCPError(t, result)
	if env.Diagnostics[0].Code != diagnostics.CodeFileNotFound {
		t.Errorf("expected code=%q, got %q (msg=%q)", diagnostics.CodeFileNotFound, env.Diagnostics[0].Code, env.Diagnostics[0].Message)
	}
}

// TestMCPExamineTemplatePathWrongExt verifies a non-.pptx extension is rejected
// with INVALID_PARAMETER.
func TestMCPExamineTemplatePathWrongExt(t *testing.T) {
	mc := testMCPConfig(t)
	dir := t.TempDir()
	note := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(note, []byte("not a template"), 0o644); err != nil {
		t.Fatalf("write notes.txt: %v", err)
	}

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_path": note,
		"base_dir":      dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true for a non-.pptx path, got success: %v", result.Content)
	}
	env := parseMCPError(t, result)
	if env.Diagnostics[0].Code != diagnostics.CodeInvalidParameter {
		t.Errorf("expected code=%q, got %q (msg=%q)", diagnostics.CodeInvalidParameter, env.Diagnostics[0].Code, env.Diagnostics[0].Message)
	}
}

// TestMCPExamineTemplateAmbiguous verifies supplying both template_name and
// template_path is rejected with AMBIGUOUS_INPUT rather than silently picking one.
func TestMCPExamineTemplateAmbiguous(t *testing.T) {
	mc := testMCPConfig(t)
	dir := t.TempDir()
	candidate := copyBundledTemplate(t, mc, "midnight-blue", filepath.Join(dir, "candidate.pptx"))

	result, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
		"template_path": candidate,
		"base_dir":      dir,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError=true when both forms are supplied, got success: %v", result.Content)
	}
	env := parseMCPError(t, result)
	if env.Diagnostics[0].Code != diagnostics.CodeAmbiguousInput {
		t.Errorf("expected code=%q, got %q (msg=%q)", diagnostics.CodeAmbiguousInput, env.Diagnostics[0].Code, env.Diagnostics[0].Message)
	}
}

// TestMCPExamineTemplatePathParityWithName confirms the guarded path form and
// the registered-name form produce the same report for the same underlying
// template bytes (modulo the display name, which derives from the file path).
func TestMCPExamineTemplatePathParityWithName(t *testing.T) {
	mc := testMCPConfig(t)
	dir := t.TempDir()
	copyBundledTemplate(t, mc, "midnight-blue", filepath.Join(dir, "candidate.pptx"))

	pathResult, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_path": "candidate.pptx",
		"base_dir":      dir,
	}))
	if err != nil || pathResult.IsError {
		t.Fatalf("path form failed: err=%v result=%v", err, pathResult)
	}
	nameResult, err := mc.handleExamineTemplate(context.Background(), makeRequest(map[string]any{
		"template_name": "midnight-blue",
	}))
	if err != nil || nameResult.IsError {
		t.Fatalf("name form failed: err=%v result=%v", err, nameResult)
	}

	var pathReport, nameReport examine.Report
	if uerr := json.Unmarshal([]byte(pathResult.Content[0].(mcp.TextContent).Text), &pathReport); uerr != nil {
		t.Fatalf("parse path report: %v", uerr)
	}
	if uerr := json.Unmarshal([]byte(nameResult.Content[0].(mcp.TextContent).Text), &nameReport); uerr != nil {
		t.Fatalf("parse name report: %v", uerr)
	}

	// SHA-256 and structural facts must match — same bytes, same template.
	if pathReport.SHA256 != nameReport.SHA256 {
		t.Errorf("sha256 differs: path=%q name=%q", pathReport.SHA256, nameReport.SHA256)
	}
	if len(pathReport.Layouts) != len(nameReport.Layouts) {
		t.Errorf("layout count differs: path=%d name=%d", len(pathReport.Layouts), len(nameReport.Layouts))
	}
}
