package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// newDiagramFontTestSpec returns a fresh bar_chart DiagramSpec. When
// explicitFont is non-empty it is set as style.font_family; otherwise the spec
// carries no font so the render path must inject the template body font. A
// fresh spec is required per render because resolveDiagramWithMetadata mutates
// the spec's Style in place.
func newDiagramFontTestSpec(explicitFont string) *types.DiagramSpec {
	spec := &types.DiagramSpec{
		Type:  "bar_chart",
		Title: "Font Test",
		Data: map[string]any{
			"categories": []any{"Alpha", "Beta"},
			"series":     []any{map[string]any{"name": "S1", "values": []any{1.0, 2.0}}},
		},
	}
	if explicitFont != "" {
		spec.Style = &types.DiagramStyle{FontFamily: explicitFont}
	}
	return spec
}

// renderPlaceholderDiagramSVG renders a placeholder diagram through
// resolveDiagramWithMetadata under a context whose template body font is
// bodyFont, and returns the emitted SVG as a string.
func renderPlaceholderDiagramSVG(t *testing.T, bodyFont, explicitFont string) string {
	t.Helper()
	ctx := newSinglePassContext("", nil, nil, false, nil)
	ctx.svgConverter = NewSVGConverterWithConfig(SVGConfig{
		Strategy: SVGStrategyNative,
		Scale:    DefaultSVGScale,
	})
	ctx.themeFontName = bodyFont

	item := ContentItem{
		PlaceholderID: "body",
		Type:          ContentDiagram,
		Value:         newDiagramFontTestSpec(explicitFont),
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 5_000_000, Height: 3_000_000}
	result, ok := ctx.resolveDiagramWithMetadata(1, item, bounds)
	if !ok {
		t.Fatalf("resolveDiagramWithMetadata returned ok=false")
	}
	if len(result.SVG) == 0 {
		t.Fatalf("resolveDiagramWithMetadata returned empty SVG")
	}
	return string(result.SVG)
}

// TestResolveDiagramWithMetadata_InjectsTemplateBodyFont is the regression test
// for go-slide-creator-fwi3: a placeholder diagram with no explicit
// style.font_family must inherit the active template body font. Two templates
// with distinct body fonts must produce SVGs whose font-family differs, and an
// explicit style.font_family always wins over the template body font.
func TestResolveDiagramWithMetadata_InjectsTemplateBodyFont(t *testing.T) {
	svgRoboto := renderPlaceholderDiagramSVG(t, "Roboto", "")
	if !strings.Contains(svgRoboto, "font-family:Roboto") {
		t.Errorf("expected SVG to use injected body font Roboto; got:\n%s", truncForLog(svgRoboto))
	}

	svgGeorgia := renderPlaceholderDiagramSVG(t, "Georgia", "")
	if !strings.Contains(svgGeorgia, "font-family:Georgia") {
		t.Errorf("expected SVG to use injected body font Georgia; got:\n%s", truncForLog(svgGeorgia))
	}
	// The two distinct body fonts must yield distinct font-family declarations.
	if strings.Contains(svgGeorgia, "font-family:Roboto") {
		t.Errorf("Georgia-template SVG must not contain Roboto font-family")
	}

	// Explicit per-diagram font_family wins over the injected template font.
	svgExplicit := renderPlaceholderDiagramSVG(t, "Roboto", "Verdana")
	if !strings.Contains(svgExplicit, "font-family:Verdana") {
		t.Errorf("explicit style.font_family must win; got:\n%s", truncForLog(svgExplicit))
	}
	if strings.Contains(svgExplicit, "font-family:Roboto") {
		t.Errorf("explicit font_family set; injected template body font Roboto must not appear")
	}
}

func truncForLog(s string) string {
	const n = 500
	if len(s) <= n {
		return s
	}
	return s[:n]
}
