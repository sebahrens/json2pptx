package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestMCPToolConstructors verifies that every MCP tool constructor in the
// package returns a Tool with a non-empty Name and Description. These are
// pure functions used at server startup; testing them here both guards
// against accidental regressions and lifts coverage on otherwise-unexercised
// metadata code.
func TestMCPToolConstructors(t *testing.T) {
	cases := []struct {
		wantName string
		fn       func() mcpgo.Tool
	}{
		{"generate_presentation", mcpGenerateTool},
		{"list_templates", mcpListTemplatesTool},
		{"get_data_format_hints", mcpGetDataFormatHintsTool},
		{"validate_input", mcpValidateTool},
		{"get_chart_capabilities", mcpGetChartCapabilitiesTool},
		{"get_diagram_capabilities", mcpGetDiagramCapabilitiesTool},
		{"recommend_pattern", mcpRecommendPatternTool},
		{"list_patterns", mcpListPatternsTool},
		{"show_pattern", mcpShowPatternTool},
		{"validate_pattern", mcpValidatePatternTool},
		{"expand_pattern", mcpExpandPatternTool},
		{"expand_patterns", mcpExpandPatternsTool},
		{"recommend_visual", mcpRecommendVisualTool},
		{"list_icons", mcpListIconsTool},
		{"preview_icon", mcpPreviewIconTool},
		{"render_slide_image", mcpRenderSlideImageTool},
		{"render_slide_image_from_json", mcpRenderSlideImageFromJSONTool},
		{"render_deck_thumbnails", mcpRenderDeckThumbnailsTool},
		{"plan_deck", mcpPlanDeckTool},
		{"read_presentation", mcpReadPresentationTool},
		{"analyze_deck_rhythm", mcpAnalyzeDeckRhythmTool},
		{"get_shape_catalog", mcpGetShapeCatalogTool},
		{"table_density_guide", mcpTableDensityGuideTool},
		{"resolve_theme", mcpResolveThemeTool},
		{"list_template_settings", mcpListTemplateSettingsTool},
		{"register_template_setting", mcpRegisterTemplateSettingTool},
		{"delete_template_setting", mcpDeleteTemplateSettingTool},
		{"repair_slide", mcpRepairSlideTool},
		{"preview_plan", mcpPreviewPlanTool},
		{"score_deck", mcpScoreDeckTool},
		{"inspect_slide_images", mcpInspectSlideImagesTool},
		{"get_capabilities", mcpGetCapabilitiesTool},
	}

	seen := make(map[string]bool, len(cases))
	for _, c := range cases {
		t.Run(c.wantName, func(t *testing.T) {
			tool := c.fn()
			if tool.Name == "" {
				t.Fatal("Tool.Name is empty")
			}
			if tool.Description == "" {
				t.Errorf("%s: Description is empty", tool.Name)
			}
			if seen[tool.Name] {
				t.Errorf("duplicate tool name %q", tool.Name)
			}
			seen[tool.Name] = true
		})
	}
}

// captureStderr redirects os.Stderr for the duration of fn and returns what
// was written, so we can verify CLI usage banners without parsing flags.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.Bytes()
	}()
	fn()
	_ = w.Close()
	os.Stderr = orig
	return string(<-done)
}

func TestPrintUsage(t *testing.T) {
	out := captureStderr(t, func() { printUsage() })
	for _, want := range []string{"json2pptx", "generate", "validate", "mcp", "patterns"} {
		if !strings.Contains(out, want) {
			t.Errorf("printUsage output missing %q: %q", want, out)
		}
	}
}

func TestPrintIconsUsage(t *testing.T) {
	out := captureStderr(t, func() { printIconsUsage() })
	if !strings.Contains(out, "icons") {
		t.Errorf("printIconsUsage missing 'icons': %q", out)
	}
	if !strings.Contains(out, "list") {
		t.Errorf("printIconsUsage missing 'list': %q", out)
	}
}

// withSavedArgs restores os.Args after fn runs so tests do not leak state
// to subsequent tests in the package.
func withSavedArgs(fn func()) {
	saved := os.Args
	defer func() { os.Args = saved }()
	fn()
}

func TestRunIcons_NoArgs_PrintsUsage(t *testing.T) {
	withSavedArgs(func() {
		os.Args = []string{"json2pptx"}
		out := captureStderr(t, func() {
			if err := runIcons(); err != nil {
				t.Fatalf("runIcons: %v", err)
			}
		})
		if !strings.Contains(out, "icons") {
			t.Errorf("expected usage banner, got %q", out)
		}
	})
}

func TestRunIcons_HelpReturnsNil(t *testing.T) {
	withSavedArgs(func() {
		os.Args = []string{"json2pptx", "help"}
		out := captureStderr(t, func() {
			if err := runIcons(); err != nil {
				t.Fatalf("runIcons help: %v", err)
			}
		})
		if !strings.Contains(out, "icons") {
			t.Errorf("expected usage banner from help, got %q", out)
		}
	})
}

func TestRunIcons_UnknownReturnsError(t *testing.T) {
	withSavedArgs(func() {
		os.Args = []string{"json2pptx", "definitely-not-a-subcommand"}
		// captureStderr to suppress the usage banner from test output.
		_ = captureStderr(t, func() {
			if err := runIcons(); err == nil {
				t.Error("expected error for unknown subcommand")
			}
		})
	})
}

func TestRunIconsListTable(t *testing.T) {
	// runIconsListTable writes to os.Stdout; redirect to verify it produces
	// the expected per-set summary lines.
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.Bytes()
	}()
	if err := runIconsListTable([]string{"outline"}); err != nil {
		t.Fatalf("runIconsListTable: %v", err)
	}
	_ = w.Close()
	os.Stdout = orig
	out := string(<-done)
	if !strings.Contains(out, "outline") {
		t.Errorf("expected 'outline' in output, got %q", out)
	}
}

func TestRunIconsListJSON(t *testing.T) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan []byte, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.Bytes()
	}()
	if err := runIconsListJSON([]string{"outline"}); err != nil {
		t.Fatalf("runIconsListJSON: %v", err)
	}
	_ = w.Close()
	os.Stdout = orig
	out := string(<-done)
	if !strings.Contains(out, `"set"`) || !strings.Contains(out, `"outline"`) {
		t.Errorf("expected JSON with set=outline, got %q", out)
	}
}

func TestRunIconsListTable_BadSet(t *testing.T) {
	if err := runIconsListTable([]string{"definitely-not-a-real-set"}); err == nil {
		t.Error("expected error for unknown icon set")
	}
}

func TestPrintPatternsUsage(t *testing.T) {
	out := captureStderr(t, func() { printPatternsUsage() })
	for _, want := range []string{"patterns", "list", "show", "validate", "expand"} {
		if !strings.Contains(out, want) {
			t.Errorf("printPatternsUsage missing %q: %q", want, out)
		}
	}
}

func TestRunPatterns_NoArgs_PrintsUsage(t *testing.T) {
	withSavedArgs(func() {
		os.Args = []string{"json2pptx"}
		out := captureStderr(t, func() {
			if err := runPatterns(); err != nil {
				t.Fatalf("runPatterns: %v", err)
			}
		})
		if !strings.Contains(out, "patterns") {
			t.Errorf("expected usage banner, got %q", out)
		}
	})
}

func TestRunPatterns_HelpReturnsNil(t *testing.T) {
	withSavedArgs(func() {
		os.Args = []string{"json2pptx", "help"}
		out := captureStderr(t, func() {
			if err := runPatterns(); err != nil {
				t.Fatalf("runPatterns help: %v", err)
			}
		})
		if !strings.Contains(out, "patterns") {
			t.Errorf("expected usage banner, got %q", out)
		}
	})
}

func TestRunPatterns_UnknownReturnsError(t *testing.T) {
	withSavedArgs(func() {
		os.Args = []string{"json2pptx", "definitely-not-a-real-subcommand"}
		_ = captureStderr(t, func() {
			if err := runPatterns(); err == nil {
				t.Error("expected error for unknown patterns subcommand")
			}
		})
	})
}

func TestUnknownPatternError(t *testing.T) {
	reg := patterns.Default()
	err := unknownPatternError("definitely-not-a-pattern", reg)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown pattern") {
		t.Errorf("error missing 'unknown pattern' prefix: %q", msg)
	}
	if !strings.Contains(msg, "available") {
		t.Errorf("error missing available list: %q", msg)
	}
	if !strings.Contains(msg, "json2pptx patterns list") {
		t.Errorf("error missing list hint: %q", msg)
	}
}

func TestUnknownPatternError_SuggestsCloseMatch(t *testing.T) {
	reg := patterns.Default()
	// "kpi-3uo" is one transposition from "kpi-3up", a real pattern.
	err := unknownPatternError("kpi-3uo", reg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("expected fuzzy suggestion, got %q", err.Error())
	}
}

func TestContainsStr(t *testing.T) {
	cases := []struct {
		name  string
		slice []string
		needle string
		want  bool
	}{
		{"present", []string{"a", "b", "c"}, "b", true},
		{"missing", []string{"a", "b", "c"}, "z", false},
		{"empty", nil, "x", false},
		{"empty needle in empty slice", nil, "", false},
		{"empty needle present", []string{"", "a"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsStr(tc.slice, tc.needle); got != tc.want {
				t.Errorf("containsStr(%v, %q) = %v, want %v", tc.slice, tc.needle, got, tc.want)
			}
		})
	}
}

func TestBoundingBoxToGeom(t *testing.T) {
	bb := types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 400}
	got := boundingBoxToGeom(bb)
	if got == nil {
		t.Fatal("got nil resolvedGeom")
	}
	if got.X != 100 || got.Y != 200 || got.Width != 300 || got.Height != 400 {
		t.Errorf("got %+v, want X=100 Y=200 W=300 H=400", got)
	}
}

func TestHandleRecommendPattern_MissingIntent(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRecommendPattern(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError=true result for missing intent")
	}
}

func TestHandleRecommendPattern_Success(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRecommendPattern(context.Background(), makeRequest(map[string]any{
		"intent": "show 3 KPI metrics",
		"content_hints": map[string]any{
			"item_count":  3,
			"has_metrics": true,
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.IsError {
		t.Errorf("expected non-error result, got IsError=true")
	}
}

func TestHandleRecommendPattern_VarietyOptions(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	// Exercise the optional recent_patterns / prefer_variety / slide_index branches.
	res, err := mc.handleRecommendPattern(context.Background(), makeRequest(map[string]any{
		"intent":           "compare two options",
		"recent_patterns":  []any{"kpi-3up", "kpi-4up"},
		"prefer_variety":   true,
		"slide_index":      float64(4),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected non-error result, got %+v", res)
	}
}

func TestHandleRecommendVisual_MissingIntent(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRecommendVisual(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError=true result for missing intent")
	}
}

func TestHandleRecommendVisual_Success(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRecommendVisual(context.Background(), makeRequest(map[string]any{
		"intent":          "compare quarterly revenue",
		"prefer_variety":  true,
		"slide_index":     float64(2),
		"recent_patterns": []any{"kpi-3up"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected non-error result, got %+v", res)
	}
}

// TestHandleRecommendPattern_Candidates verifies the MCP handler plumbs the
// candidates array through to the patterns package and returns every name
// (no threshold cutoff applied).
func TestHandleRecommendPattern_Candidates(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRecommendPattern(context.Background(), makeRequest(map[string]any{
		"intent":     "show key metrics",
		"candidates": []any{"kpi-3up", "matrix-2x2", "comparison-2col", "card-grid"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected non-error result, got %+v", res)
	}

	var output struct {
		Candidates []struct {
			PatternName    string  `json:"pattern_name"`
			Score          float64 `json:"score"`
			Rationale      string  `json:"rationale"`
			ConfidenceBand string  `json:"confidence_band"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(textContent(res)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(output.Candidates) != 4 {
		t.Fatalf("expected 4 candidates (one per shortlist name), got %d: %+v",
			len(output.Candidates), output.Candidates)
	}
	for _, c := range output.Candidates {
		if c.Rationale == "" {
			t.Errorf("candidate %q: empty rationale", c.PatternName)
		}
		if c.ConfidenceBand == "" {
			t.Errorf("candidate %q: empty confidence_band", c.PatternName)
		}
	}
}

// TestHandleRecommendVisual_Candidates verifies the unified visual handler
// plumbs candidates through and resolves categories across the catalog.
func TestHandleRecommendVisual_Candidates(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRecommendVisual(context.Background(), makeRequest(map[string]any{
		"intent":     "compare quarterly revenue",
		"candidates": []any{"kpi-3up", "bar", "title", "swot"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected non-error result, got %+v", res)
	}

	var output struct {
		Candidates []struct {
			Category       string  `json:"category"`
			Name           string  `json:"name"`
			Score          float64 `json:"score"`
			Rationale      string  `json:"rationale"`
			ConfidenceBand string  `json:"confidence_band"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(textContent(res)), &output); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(output.Candidates) != 4 {
		t.Fatalf("expected 4 candidates (one per shortlist name), got %d: %+v",
			len(output.Candidates), output.Candidates)
	}
	wantCat := map[string]string{
		"kpi-3up": "named_pattern",
		"bar":     "chart",
		"title":   "placeholder_layout",
		"swot":    "diagram",
	}
	for _, c := range output.Candidates {
		if got := wantCat[c.Name]; got != "" && c.Category != got {
			t.Errorf("candidate %q: got category %q, want %q", c.Name, c.Category, got)
		}
		if c.Rationale == "" {
			t.Errorf("candidate %q: empty rationale", c.Name)
		}
		if c.ConfidenceBand == "" {
			t.Errorf("candidate %q: empty confidence_band", c.Name)
		}
	}
}

// _ retains import usages even if some helpers are removed in the future.
var _ = patterns.Default

func TestHandleRenderSlideImage_MissingPath(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderSlideImage(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for missing pptx_path")
	}
}

func TestHandleRenderSlideImage_FileNotFound(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderSlideImage(context.Background(), makeRequest(map[string]any{
		"pptx_path": "/nonexistent/path/to/missing.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for missing file")
	}
}

func TestHandleRenderSlideImage_OptionParsing(t *testing.T) {
	// Create a placeholder file so the os.Stat check passes; rendering will
	// then fail (LibreOffice unavailable or invalid PPTX), but the density
	// clamp / slide_index / force branches get exercised.
	dir := t.TempDir()
	p := dir + "/fake.pptx"
	if err := os.WriteFile(p, []byte("not really a pptx"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderSlideImage(context.Background(), makeRequest(map[string]any{
		"pptx_path":   p,
		"slide_index": float64(1),
		"density":     float64(10000), // exercise upper-clamp
		"force":       true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	// Render is expected to fail (no LibreOffice / fake PPTX); we just want the
	// branches covered.
	if !res.IsError {
		t.Logf("render unexpectedly succeeded for fake PPTX (LibreOffice present)")
	}
}

func TestHandleRenderSlideImage_DensityLowerClamp(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/fake.pptx"
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mc := cliMCPConfig("./templates", "./out")
	_, err := mc.handleRenderSlideImage(context.Background(), makeRequest(map[string]any{
		"pptx_path": p,
		"density":   float64(10), // lower-clamp
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRenderDeckThumbnails_MissingPath(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for missing pptx_path")
	}
}

func TestHandleRenderDeckThumbnails_FileNotFound(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path": "/nonexistent/missing.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for missing file")
	}
}

func TestHandleRenderDeckThumbnails_OptionParsing(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/fake.pptx"
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	mc := cliMCPConfig("./templates", "./out")
	// Upper-clamp density and provide max_slides.
	if _, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path":  p,
		"density":    float64(9999), // upper clamp
		"max_slides": float64(3),
		"force":      true,
	})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Lower-clamp density.
	if _, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path": p,
		"density":   float64(1), // lower clamp
	})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleRenderSlideImage_PathTraversal(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderSlideImage(context.Background(), makeRequest(map[string]any{
		"pptx_path": "../../etc/passwd.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for path traversal")
	}
}

func TestHandleRenderDeckThumbnails_PathTraversal(t *testing.T) {
	mc := cliMCPConfig("./templates", "./out")
	res, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path": "/tmp/../../../etc/secret.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for path traversal")
	}
}

func TestHandleReadPresentation_PathTraversal(t *testing.T) {
	res, err := handleReadPresentation(context.Background(), makeRequest(map[string]any{
		"pptx_path": "../../../etc/passwd.pptx",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for path traversal")
	}
}

func TestHandleReadPresentation_WrongExtension(t *testing.T) {
	res, err := handleReadPresentation(context.Background(), makeRequest(map[string]any{
		"pptx_path": "/tmp/secrets.txt",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError for wrong extension")
	}
}

func TestHandleAnalyzeDeckRhythm_MissingPresentation(t *testing.T) {
	res, err := handleAnalyzeDeckRhythm(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for missing presentation")
	}
}

func TestHandleAnalyzeDeckRhythm_InvalidJSON(t *testing.T) {
	res, err := handleAnalyzeDeckRhythm(context.Background(), makeRequest(map[string]any{
		"presentation": "not-an-object",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for non-object presentation")
	}
}

func TestHandleAnalyzeDeckRhythm_EmptySlides(t *testing.T) {
	res, err := handleAnalyzeDeckRhythm(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(`{"slides":[]}`),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for empty slides")
	}
}

func TestHandleListIcons_AllSets(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestHandleListIcons_FilterAndSearch(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"set":    "outline",
		"search": "arrow",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestHandleListIcons_BadSet(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"set": "definitely-not-a-set",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for bad set")
	}
}

// TestHandleListIcons_QualifiedName verifies that every set entry exposes a
// parallel icons[] array whose qualified_name field is in canonical
// "<set>:<name>" form — the authoring token agents drop into icon.name.
func TestHandleListIcons_QualifiedName(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"page_size": float64(200),
	}))
	if err != nil || res.IsError {
		t.Fatalf("unexpected failure: err=%v result=%+v", err, res)
	}
	var resp listIconsResponse
	if err := json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Sets) == 0 {
		t.Fatal("expected at least one set in response")
	}
	for _, s := range resp.Sets {
		if len(s.Icons) != len(s.Names) {
			t.Errorf("set %q: icons length %d != names length %d", s.Set, len(s.Icons), len(s.Names))
		}
		for i, entry := range s.Icons {
			want := s.Set + ":" + entry.Name
			if entry.QualifiedName != want {
				t.Errorf("set %q icon %d: qualified_name = %q, want %q", s.Set, i, entry.QualifiedName, want)
			}
			if i < len(s.Names) && entry.Name != s.Names[i] {
				t.Errorf("set %q icon %d: name = %q, want %q (must match parallel names[])", s.Set, i, entry.Name, s.Names[i])
			}
		}
	}
	// Filled icons must use the qualified form — verify there is at least one
	// filled icon and its qualified_name starts with "filled:".
	var filledOK bool
	for _, s := range resp.Sets {
		if s.Set != "filled" {
			continue
		}
		for _, entry := range s.Icons {
			if !strings.HasPrefix(entry.QualifiedName, "filled:") {
				t.Errorf("filled icon %q: qualified_name = %q, want prefix \"filled:\"", entry.Name, entry.QualifiedName)
			}
			filledOK = true
		}
	}
	if !filledOK {
		t.Skip("no filled icons in response — cannot verify filled: prefix")
	}
}

// TestHandleListIcons_Pagination verifies cursor + page_size iteration covers
// the full icon corpus without duplicates and emits next_cursor only when
// more entries remain.
func TestHandleListIcons_Pagination(t *testing.T) {
	// Baseline call: one big page.
	full, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"set":       "outline",
		"page_size": float64(200),
	}))
	if err != nil || full.IsError {
		t.Fatalf("baseline call failed: err=%v result=%+v", err, full)
	}
	var fullResp listIconsResponse
	if err := json.Unmarshal([]byte(full.Content[0].(mcpgo.TextContent).Text), &fullResp); err != nil {
		t.Fatalf("failed to parse baseline: %v", err)
	}
	total := fullResp.TotalCount
	if total < 10 {
		t.Skipf("not enough outline icons to exercise pagination (total=%d)", total)
	}

	// Iterate with a small page size.
	const pageSize = 7
	cursor := ""
	seen := make(map[string]bool, total)
	iterations := 0
	for {
		args := map[string]any{
			"set":       "outline",
			"page_size": float64(pageSize),
		}
		if cursor != "" {
			args["cursor"] = cursor
		}
		res, err := handleListIcons(context.Background(), makeRequest(args))
		if err != nil || res.IsError {
			t.Fatalf("page call failed at cursor %q: err=%v result=%+v", cursor, err, res)
		}
		var resp listIconsResponse
		if err := json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &resp); err != nil {
			t.Fatalf("failed to parse page: %v", err)
		}
		if resp.TotalCount != total {
			t.Errorf("total_count = %d, want %d", resp.TotalCount, total)
		}
		count := 0
		for _, s := range resp.Sets {
			for _, n := range s.Names {
				key := s.Set + ":" + n
				if seen[key] {
					t.Errorf("duplicate icon %q across pages", key)
				}
				seen[key] = true
				count++
			}
		}
		if count > pageSize {
			t.Errorf("page exceeded page_size: got %d, want <= %d", count, pageSize)
		}
		if resp.NextCursor == "" {
			break
		}
		cursor = resp.NextCursor
		iterations++
		if iterations > total {
			t.Fatalf("pagination did not terminate after %d iterations", iterations)
		}
	}
	if len(seen) != total {
		t.Errorf("iterated %d unique icons, want %d", len(seen), total)
	}
}

// TestHandleListIcons_InvalidCursor verifies a malformed cursor produces a
// structured INVALID_PARAMETER error rather than a panic.
func TestHandleListIcons_InvalidCursor(t *testing.T) {
	res, err := handleListIcons(context.Background(), makeRequest(map[string]any{
		"cursor": "nope",
	}))
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError result for invalid cursor, got %+v", res)
	}
}

func TestHandleGetShapeCatalog_All(t *testing.T) {
	res, err := handleGetShapeCatalog(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestHandleGetShapeCatalog_FilterCategory(t *testing.T) {
	res, err := handleGetShapeCatalog(context.Background(), makeRequest(map[string]any{
		"category": "basic",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	// Either matches or is empty; just exercise the branch.
	_ = res.IsError
}

func TestHandleGetShapeCatalog_Search(t *testing.T) {
	res, err := handleGetShapeCatalog(context.Background(), makeRequest(map[string]any{
		"search": "rect",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
}

func TestHandleAnalyzeDeckRhythm_Success(t *testing.T) {
	deck := `{
		"slides": [
			{"layout":"title","placeholders":{"title":"Hello"}},
			{"layout":"content","placeholders":{"title":"Body","body":"Content"}}
		]
	}`
	res, err := handleAnalyzeDeckRhythm(context.Background(), makeRequest(map[string]any{
		"presentation": mustParseJSON(deck),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.IsError {
		t.Errorf("unexpected IsError result: %+v", res)
	}
}
