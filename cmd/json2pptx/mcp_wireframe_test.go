package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestHandlePreviewSlideWireframe_MissingPresentation verifies that the
// handler returns a structured error when the presentation parameter is
// absent.
func TestHandlePreviewSlideWireframe_MissingPresentation(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handlePreviewSlideWireframe(context.Background(), makeRequest(map[string]any{
		"slide_index": float64(0),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when presentation is missing")
	}
}

// TestHandlePreviewSlideWireframe_MissingSlideIndex verifies the handler
// rejects a request that omits the slide_index parameter.
func TestHandlePreviewSlideWireframe_MissingSlideIndex(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handlePreviewSlideWireframe(context.Background(), makeRequest(map[string]any{
		"presentation": map[string]any{
			"template": "midnight-blue",
			"slides":   []any{map[string]any{"layout_id": "slideLayout1"}},
		},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when slide_index is missing")
	}
}

// TestHandlePreviewSlideWireframe_OutOfRange verifies the handler rejects
// a slide_index past the end of the deck with a structured error.
func TestHandlePreviewSlideWireframe_OutOfRange(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handlePreviewSlideWireframe(context.Background(), makeRequest(map[string]any{
		"presentation": map[string]any{
			"template": "midnight-blue",
			"slides":   []any{map[string]any{"layout_id": "slideLayout1"}},
		},
		"slide_index": float64(7),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError result when slide_index is out of range")
	}
}

// TestHandlePreviewSlideWireframe_Success exercises the full path: a
// real template, a slide with a shape_grid, and the default both-format
// output. Asserts the response carries both an SVG document and a
// decodable base64 PNG.
func TestHandlePreviewSlideWireframe_Success(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handlePreviewSlideWireframe(context.Background(), makeRequest(map[string]any{
		"presentation": map[string]any{
			"template": "midnight-blue",
			"slides": []any{
				map[string]any{
					"layout_id": "slideLayout2",
					"content": []any{
						map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Wireframe Smoke Test"},
					},
					"shape_grid": map[string]any{
						"rows": []any{
							map[string]any{
								"cells": []any{
									map[string]any{"shape": map[string]any{"type": "rect", "fill": "accent1"}},
									map[string]any{"shape": map[string]any{"type": "rect", "fill": "accent2"}},
								},
							},
							map[string]any{
								"cells": []any{
									map[string]any{"shape": map[string]any{"type": "rect", "fill": "accent3"}},
									map[string]any{"shape": map[string]any{"type": "rect", "fill": "accent4"}},
								},
							},
						},
					},
				},
			},
		},
		"slide_index": float64(0),
		"width_px":    float64(800),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("nil result")
	}
	if res.IsError {
		t.Fatalf("expected success, got error result")
	}

	// Extract text content.
	text := extractMCPTextContent(res)
	if text == "" {
		t.Fatalf("result missing text content")
	}
	var out struct {
		Index            int    `json:"index"`
		SVG              string `json:"svg"`
		PNG64            string `json:"png_base64"`
		CellCount        int    `json:"cell_count"`
		PlaceholderCount int    `json:"placeholder_count"`
		Width            int    `json:"width"`
		Height           int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("failed to parse response: %v\nbody: %s", err, text)
	}
	if out.Index != 0 {
		t.Errorf("Index = %d, want 0", out.Index)
	}
	if out.SVG == "" {
		t.Errorf("SVG output is empty")
	}
	if !strings.Contains(out.SVG, "<svg") {
		t.Errorf("SVG output missing root element")
	}
	if out.PNG64 == "" {
		t.Errorf("PNG output is empty")
	}
	if _, err := base64.StdEncoding.DecodeString(out.PNG64); err != nil {
		t.Errorf("PNG base64 decode failed: %v", err)
	}
	if out.CellCount != 4 {
		t.Errorf("CellCount = %d, want 4 (2x2 grid)", out.CellCount)
	}
	if out.Width <= 0 || out.Height <= 0 {
		t.Errorf("width/height = %d×%d, want positive", out.Width, out.Height)
	}
}

// TestHandlePreviewSlideWireframe_SVGOnly verifies that format="svg"
// returns only SVG (no PNG payload).
func TestHandlePreviewSlideWireframe_SVGOnly(t *testing.T) {
	mc := cliMCPConfig("../../templates", "")
	res, err := mc.handlePreviewSlideWireframe(context.Background(), makeRequest(map[string]any{
		"presentation": map[string]any{
			"template": "midnight-blue",
			"slides":   []any{map[string]any{"layout_id": "slideLayout1"}},
		},
		"slide_index": float64(0),
		"format":      "svg",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("expected success, got error result")
	}
	text := extractMCPTextContent(res)
	if !strings.Contains(text, `"svg":`) {
		t.Errorf("svg field missing from response")
	}
	if strings.Contains(text, `"png_base64":`) {
		t.Errorf("png_base64 should be omitted when format=svg, got: %s", text[:min(400, len(text))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
