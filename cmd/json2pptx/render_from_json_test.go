package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/render"
)

// TestHandleRenderSlideImageFromJSON_MissingSlide verifies the handler returns
// a structured error when the required slide parameter is absent.
func TestHandleRenderSlideImageFromJSON_MissingSlide(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"template": "midnight-blue",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when slide is missing")
	}
}

// TestHandleRenderSlideImageFromJSON_MissingTemplate verifies the handler
// rejects a request that omits the template name.
func TestHandleRenderSlideImageFromJSON_MissingTemplate(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"slide": map[string]any{"layout_id": "slideLayout1"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when template is missing")
	}
}

// TestHandleRenderSlideImageFromJSON_TemplateNotFound verifies the handler
// surfaces TEMPLATE_NOT_FOUND for an unknown template name.
func TestHandleRenderSlideImageFromJSON_TemplateNotFound(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(map[string]any{
		"slide":    map[string]any{"layout_id": "slideLayout1"},
		"template": "definitely-not-a-real-template",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result for unknown template")
	}
}

// TestRenderSlideWithCacheKey_RequiresKey verifies the render helper rejects
// an empty cache key — callers must supply an identity derived from upstream
// inputs.
func TestRenderSlideWithCacheKey_RequiresKey(t *testing.T) {
	_, err := render.RenderSlideWithCacheKey("/tmp/does-not-matter.pptx", 0, 100, false, "")
	if err == nil {
		t.Fatal("expected error for empty cache key")
	}
	if !strings.Contains(err.Error(), "key") {
		t.Errorf("expected error mentioning key, got: %v", err)
	}
}

// TestLookupCachedSlide_EmptyKey returns nil for empty key (defensive check
// — callers should never reach this branch but failing closed is correct).
func TestLookupCachedSlide_EmptyKey(t *testing.T) {
	if got := render.LookupCachedSlide("", 0, 100); got != nil {
		t.Fatalf("expected nil for empty key, got %+v", got)
	}
}

// TestRenderSlideImageFromJSON_OutputSchemaShared verifies that the new tool
// reuses the same output schema as render_slide_image — callers can parse
// either response with the same JSON unmarshal.
func TestRenderSlideImageFromJSON_OutputSchemaShared(t *testing.T) {
	tool := mcpRenderSlideImageFromJSONTool()
	// Output schema is the same constant (pointer/value identity not required;
	// what matters is the JSON structure).
	schema := successSchemaBranch(t, tool.RawOutputSchema)
	props, _ := schema["properties"].(map[string]any)
	for _, want := range []string{"index", "png_base64", "path", "size_error"} {
		if _, ok := props[want]; !ok {
			t.Errorf("expected schema property %q", want)
		}
	}
}

// TestRenderSlideImageFromJSON_OverlayParamAdvertised verifies the new
// overlay parameter is exposed on the tool schema so MCP callers know to
// pass it.
func TestRenderSlideImageFromJSON_OverlayParamAdvertised(t *testing.T) {
	tool := mcpRenderSlideImageFromJSONTool()
	encoded, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal input schema: %v", err)
	}
	if !strings.Contains(string(encoded), `"overlay"`) {
		t.Errorf("expected tool input schema to include 'overlay' property, got: %s", string(encoded))
	}
}

// TestBuildOverlayWireframeRequest_ShapeGridCells verifies the overlay
// wireframe builder resolves shape_grid cells from a slide JSON and
// surfaces them in the wireframe request that gets sent to svggen.
func TestBuildOverlayWireframeRequest_ShapeGridCells(t *testing.T) {
	slideJSON := `{
		"layout_id": "blank",
		"shape_grid": {
			"rows": [
				{"cells": [
					{"shape": {"geometry": "rect", "fill": "accent1"}},
					{"shape": {"geometry": "rect", "fill": "accent2"}}
				]},
				{"cells": [
					{"shape": {"geometry": "rect", "fill": "accent3"}},
					{"shape": {"geometry": "rect", "fill": "accent4"}}
				]}
			]
		}
	}`
	tplPath, cleanup, err := resolveTemplatePath("midnight-blue", "../../templates")
	if err != nil {
		t.Skipf("template not available: %v", err)
	}
	defer cleanup()

	wf, err := buildOverlayWireframeRequest(slideJSON, "midnight-blue", tplPath, "../../templates", 800)
	if err != nil {
		t.Fatalf("buildOverlayWireframeRequest: %v", err)
	}
	if wf == nil {
		t.Fatal("expected wireframe request, got nil — shape_grid cells should produce one")
	}
	if len(wf.Cells) != 4 {
		t.Errorf("expected 4 cells from 2x2 shape_grid, got %d", len(wf.Cells))
	}
	if wf.SlideWidth <= 0 || wf.SlideHeight <= 0 {
		t.Errorf("expected positive slide dimensions, got %vx%v", wf.SlideWidth, wf.SlideHeight)
	}
}

// TestBuildOverlayWireframeRequest_NoCellsNoFindings returns nil when
// the slide has no shape_grid and no findings — callers skip the
// overlay step rather than rendering an empty PNG.
func TestBuildOverlayWireframeRequest_NoCellsNoFindings(t *testing.T) {
	// slide_type blank is empty BY DESIGN, so the substance detectors stay quiet
	// (go-slide-creator-q7ar).
	slideJSON := `{"layout_id": "blank", "slide_type": "blank", "content": []}`
	tplPath, cleanup, err := resolveTemplatePath("midnight-blue", "../../templates")
	if err != nil {
		t.Skipf("template not available: %v", err)
	}
	defer cleanup()

	wf, err := buildOverlayWireframeRequest(slideJSON, "midnight-blue", tplPath, "../../templates", 800)
	if err != nil {
		t.Fatalf("buildOverlayWireframeRequest: %v", err)
	}
	if wf != nil {
		t.Errorf("expected nil request for slide with no cells/findings, got cells=%d findings=%d",
			len(wf.Cells), len(wf.Findings))
	}
}

// _ keeps mcpgo referenced even when no other test in this file uses it
// directly.
var _ mcpgo.Tool

// A repeat render of the same slide is served from the cache in the shape of a
// fresh render — an image block and the PNG's path. It used to answer with the
// bare image and its base64 inline, which `render-slide-from-json` reported as
// INTERNAL ("render result names no image file") until --force.
func TestHandleRenderSlideImageFromJSON_CachedAnswerHasTheFreshShape(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	mc := cliMCPConfig("../../templates", "")
	args := map[string]any{
		"template": "midnight-blue",
		"slide": map[string]any{"layout_id": "title", "content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Cached render " + t.Name()},
		}},
	}
	for _, pass := range []string{"fresh", "cached"} {
		res, err := mc.handleRenderSlideImageFromJSON(context.Background(), makeRequest(args))
		if err != nil || res == nil || res.IsError {
			t.Fatalf("%s render: err=%v result=%+v", pass, err, res)
		}
		deck, err := cliRenderedSlides(res)
		if err != nil || len(deck.Slides) != 1 || deck.Slides[0].Path == "" {
			t.Errorf("%s render names no image file: %v %+v", pass, err, deck.Slides)
		}
		blocks := 0
		for _, c := range res.Content {
			if _, ok := c.(mcpgo.ImageContent); ok {
				blocks++
			}
		}
		if blocks != 1 {
			t.Errorf("%s render carries %d image blocks, want 1", pass, blocks)
		}
	}
}
