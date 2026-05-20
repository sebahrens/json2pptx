package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/resource"
)

const previewInlineSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24"><rect width="24" height="24" fill="#FF0000"/></svg>`

// callPreviewIcon invokes handlePreviewIcon with the given arguments and
// returns the decoded success envelope. It fails the test fatally if the
// tool returns an MCP error.
func callPreviewIcon(t *testing.T, mc *mcpConfig, args map[string]any) *previewIconResponse {
	t.Helper()
	if mc == nil {
		mc = &mcpConfig{}
	}
	result, err := mc.handlePreviewIcon(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handlePreviewIcon returned error: %v", err)
	}
	if result.IsError {
		text, _ := mcpResultText(result)
		t.Fatalf("handlePreviewIcon returned error result: %s", text)
	}
	text, ok := mcpResultText(result)
	if !ok {
		t.Fatalf("preview_icon: no text content in result")
	}
	var resp previewIconResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("decode preview_icon response: %v\nbody: %s", err, text)
	}
	return &resp
}

// callPreviewIconExpectError invokes handlePreviewIcon and asserts the
// result is an MCP error containing the expected code.
func callPreviewIconExpectError(t *testing.T, mc *mcpConfig, args map[string]any, wantCode string) {
	t.Helper()
	if mc == nil {
		mc = &mcpConfig{}
	}
	result, err := mc.handlePreviewIcon(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("handlePreviewIcon returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected error result for %s, got success", wantCode)
	}
	text, ok := mcpResultText(result)
	if !ok {
		t.Fatalf("preview_icon: error result has no text")
	}
	if !strings.Contains(text, wantCode) {
		t.Errorf("expected error to contain code %q, got: %s", wantCode, text)
	}
}

func mcpResultText(result *mcp.CallToolResult) (string, bool) {
	if result == nil || len(result.Content) == 0 {
		return "", false
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		return "", false
	}
	return tc.Text, true
}

func TestPreviewIcon_BundledWithFill(t *testing.T) {
	resp := callPreviewIcon(t, nil, map[string]any{
		"icon": map[string]any{
			"name": "filled:chart-pie",
			"fill": "#FF0000",
		},
	})

	if resp.SourceKind != "bundled" {
		t.Errorf("source_kind = %q, want bundled", resp.SourceKind)
	}
	if resp.QualifiedName != "filled:chart-pie" {
		t.Errorf("qualified_name = %q, want filled:chart-pie", resp.QualifiedName)
	}
	if resp.SVGData == "" {
		t.Fatal("svg_data is empty")
	}
	if !strings.Contains(resp.SVGData, "<svg") {
		t.Errorf("svg_data does not contain '<svg': %s", truncate(resp.SVGData, 200))
	}
	// Fill must be applied — the filled-icon SVGs use fill="currentColor",
	// which applyIconFill replaces with the supplied color.
	if !strings.Contains(resp.SVGData, "#FF0000") {
		t.Errorf("svg_data does not contain fill color '#FF0000': %s", truncate(resp.SVGData, 200))
	}
	if resp.PNGBase64 == "" {
		t.Error("expected non-empty png_base64 for bundled icon with fill")
	}
	if resp.Width <= 0 || resp.Height <= 0 {
		t.Errorf("expected positive PNG dimensions, got %dx%d", resp.Width, resp.Height)
	}
	if _, err := base64.StdEncoding.DecodeString(resp.PNGBase64); err != nil {
		t.Errorf("png_base64 is not valid base64: %v", err)
	}
	if resp.Alt != "filled:chart-pie" {
		t.Errorf("alt = %q, want fallback to icon name", resp.Alt)
	}
}

func TestPreviewIcon_BundledOutlineQualifiesDefault(t *testing.T) {
	resp := callPreviewIcon(t, nil, map[string]any{
		"icon": map[string]any{
			"name": "chart-pie",
		},
	})
	if resp.QualifiedName != "outline:chart-pie" {
		t.Errorf("qualified_name = %q, want outline:chart-pie", resp.QualifiedName)
	}
}

func TestPreviewIcon_BundledNameUnknown(t *testing.T) {
	callPreviewIconExpectError(t, nil, map[string]any{
		"icon": map[string]any{"name": "definitely-not-an-icon"},
	}, "ICON_BUNDLED_NAME_UNKNOWN")
}

func TestPreviewIcon_PathWithBaseDir(t *testing.T) {
	dir := t.TempDir()
	svgPath := filepath.Join(dir, "logo.svg")
	if err := os.WriteFile(svgPath, []byte(previewInlineSVG), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	resp := callPreviewIcon(t, nil, map[string]any{
		"icon": map[string]any{
			"path": "logo.svg",
			"alt":  "Company Logo",
		},
		"base_dir": dir,
	})

	if resp.SourceKind != "path" {
		t.Errorf("source_kind = %q, want path", resp.SourceKind)
	}
	if resp.QualifiedName != "" {
		t.Errorf("qualified_name should be empty for path source, got %q", resp.QualifiedName)
	}
	if resp.Alt != "Company Logo" {
		t.Errorf("alt = %q, want explicit alt", resp.Alt)
	}
	if !strings.Contains(resp.SVGData, "<svg") {
		t.Errorf("svg_data does not contain '<svg'")
	}
}

func TestPreviewIcon_PathWithFillRecolors(t *testing.T) {
	dir := t.TempDir()
	// Outline-style SVG using stroke="currentColor" so applyIconFill recolors stroke.
	outline := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="10"/></svg>`
	svgPath := filepath.Join(dir, "outline.svg")
	if err := os.WriteFile(svgPath, []byte(outline), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	resp := callPreviewIcon(t, nil, map[string]any{
		"icon": map[string]any{
			"path": "outline.svg",
			"fill": "#00FF00",
		},
		"base_dir": dir,
	})

	if !strings.Contains(resp.SVGData, "#00FF00") {
		t.Errorf("expected fill '#00FF00' in svg_data, got: %s", truncate(resp.SVGData, 200))
	}
}

func TestPreviewIcon_InlineSVGData(t *testing.T) {
	resp := callPreviewIcon(t, nil, map[string]any{
		"icon": map[string]any{
			"svg_data": previewInlineSVG,
		},
	})
	if resp.SourceKind != "inline" {
		t.Errorf("source_kind = %q, want inline", resp.SourceKind)
	}
	if resp.SVGData != previewInlineSVG {
		t.Errorf("svg_data not preserved verbatim")
	}
	if resp.QualifiedName != "" {
		t.Errorf("qualified_name should be empty for inline source")
	}
	if resp.Alt != "icon" {
		t.Errorf("alt = %q, want fallback 'icon'", resp.Alt)
	}
}

func TestPreviewIcon_InlineSVGData_FillIgnoredWithWarning(t *testing.T) {
	resp := callPreviewIcon(t, nil, map[string]any{
		"icon": map[string]any{
			"svg_data": previewInlineSVG,
			"fill":     "#00FFFF",
		},
	})
	// Inline SVG must pass through unchanged.
	if resp.SVGData != previewInlineSVG {
		t.Errorf("svg_data was modified despite inline source")
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected a warning about ignored fill on inline svg_data")
	}
	found := false
	for _, w := range resp.Warnings {
		if strings.Contains(w, "ignored") && strings.Contains(w, "svg_data") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning to mention 'ignored' and 'svg_data', got: %v", resp.Warnings)
	}
}

func TestPreviewIcon_MissingSource(t *testing.T) {
	callPreviewIconExpectError(t, nil, map[string]any{
		"icon": map[string]any{},
	}, "ICON_MISSING")
}

func TestPreviewIcon_AmbiguousSources(t *testing.T) {
	callPreviewIconExpectError(t, nil, map[string]any{
		"icon": map[string]any{
			"name":     "chart-pie",
			"svg_data": previewInlineSVG,
		},
	}, "ICON_AMBIGUOUS")
}

func TestPreviewIcon_MissingIconArg(t *testing.T) {
	callPreviewIconExpectError(t, nil, map[string]any{}, "MISSING_PARAMETER")
}

func TestPreviewIcon_SizeClamp(t *testing.T) {
	// Oversize should clamp without erroring.
	resp := callPreviewIcon(t, nil, map[string]any{
		"icon":    map[string]any{"name": "filled:chart-pie", "fill": "#000000"},
		"size_px": float64(99999),
	})
	if resp.Width == 0 || resp.Height == 0 {
		t.Error("expected non-zero dimensions after size clamp")
	}
}

func TestPreviewIcon_URLFetchSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(previewInlineSVG))
	}))
	t.Cleanup(server.Close)

	mc := &mcpConfig{
		resolverOpts: resource.ResolverOptions{
			HTTPClient: server.Client(),
		},
	}
	resp := callPreviewIcon(t, mc, map[string]any{
		"icon": map[string]any{"url": server.URL + "/icon.svg"},
	})
	if resp.SourceKind != "url" {
		t.Errorf("source_kind = %q, want url", resp.SourceKind)
	}
	if !strings.Contains(resp.SVGData, "<svg") {
		t.Errorf("svg_data missing '<svg'")
	}
	if resp.Alt != server.URL+"/icon.svg" {
		t.Errorf("alt = %q, want url fallback", resp.Alt)
	}
}

func TestPreviewIcon_URLFetchFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	mc := &mcpConfig{
		resolverOpts: resource.ResolverOptions{
			HTTPClient: server.Client(),
		},
	}
	callPreviewIconExpectError(t, mc, map[string]any{
		"icon": map[string]any{"url": server.URL + "/missing.svg"},
	}, "URL_FETCH_FAILED")
}

func TestPreviewIcon_PathExtMustBeSVG(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "wrong.png")
	if err := os.WriteFile(pngPath, []byte("not svg"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	callPreviewIconExpectError(t, nil, map[string]any{
		"icon":     map[string]any{"path": "wrong.png"},
		"base_dir": dir,
	}, "ICON_PATH_EXT_INVALID")
}

func TestPreviewIcon_BaseDirMustBeAbsolute(t *testing.T) {
	callPreviewIconExpectError(t, nil, map[string]any{
		"icon":     map[string]any{"path": "logo.svg"},
		"base_dir": "relative/path",
	}, "INVALID_PARAMETER")
}

func TestQualifiedIconName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"chart-pie", "outline:chart-pie"},
		{"outline:chart-pie", "outline:chart-pie"},
		{"filled:chart-pie", "filled:chart-pie"},
		{"  chart-pie  ", "outline:chart-pie"},
	}
	for _, c := range cases {
		if got := qualifiedIconName(c.in); got != c.want {
			t.Errorf("qualifiedIconName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
