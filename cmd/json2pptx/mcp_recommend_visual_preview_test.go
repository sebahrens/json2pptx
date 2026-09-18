package main

import (
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestMCPRecommendVisual_TemplatePreviewRefs verifies recommend_visual with a
// bundled template returns shipped layout-preview references instead of
// metadata only (go-slide-creator-aruv): every candidate carries the layout
// it renders on plus that layout's thumbnail, and placeholder candidates use
// the thumbnail as their preview (metadata_only=false).
func TestMCPRecommendVisual_TemplatePreviewRefs(t *testing.T) {
	mc := testMCPConfig(t)
	rec := callRecommendVisual(t, mc, map[string]any{
		"intent":     "section divider between chapters",
		"template":   "midnight-blue",
		"candidates": []any{"section", "content", "kpi-3up", "bar_chart"},
	})
	if len(rec.Candidates) == 0 {
		t.Fatal("expected candidates")
	}
	sawPlaceholder := false
	for _, c := range rec.Candidates {
		ex := c.Example
		if ex == nil {
			t.Fatalf("candidate %q: missing example", c.Name)
		}
		if ex.LayoutID == "" || ex.LayoutPreviewPNGPath == "" {
			t.Errorf("candidate %q (%s): missing layout preview ref: %+v", c.Name, c.Category, ex)
			continue
		}
		if !strings.HasSuffix(ex.LayoutPreviewPNGPath, ex.LayoutID+".png") {
			t.Errorf("candidate %q: preview %q does not match layout %q", c.Name, ex.LayoutPreviewPNGPath, ex.LayoutID)
		}
		if _, err := os.Stat(ex.LayoutPreviewPNGPath); err != nil {
			t.Errorf("candidate %q: preview file missing: %v", c.Name, err)
		}
		if c.Category == patterns.VisualCategoryPlaceholder {
			sawPlaceholder = true
			if ex.MetadataOnly || ex.PreviewPNGPath != ex.LayoutPreviewPNGPath || ex.Renderer != "template-preview" {
				t.Errorf("placeholder candidate %q should use its layout thumbnail as preview: %+v", c.Name, ex)
			}
		}
		if c.Name == "section" && ex.LayoutID != "slideLayout4" {
			t.Errorf("section candidate layout = %q, want the Section Divider (slideLayout4)", ex.LayoutID)
		}
	}
	if !sawPlaceholder {
		t.Error("expected at least one placeholder candidate")
	}
}
