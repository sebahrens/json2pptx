package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// svgBase64Blob matches an embedded data-URI payload — in practice the base64
// OpenType of an @font-face block.
var svgBase64Blob = regexp.MustCompile(`base64,([A-Za-z0-9+/=]+)`)

// maxEmbeddedFontBytes is the per-face budget for an embedded font block, in
// base64 characters. A full unsubsetted Calibri face is ~547,000; a subset of
// the glyphs one chart draws is a few thousand. The budget sits far from both,
// so it catches the renderer falling back to whole-family embedding without
// pinning the subsetter's exact output.
const maxEmbeddedFontBytes = 120_000

// maxChartSVGBytes is the per-chart document budget from go-slide-creator-i0ep's
// VERIFY: each chart SVG under 50 KB.
const maxChartSVGBytes = 50_000

// TestChartSVGEmbedsSubsettedFonts pins the fix for a six-chart deck that
// shipped 6.6 MB of media for 5 KB of drawing: every chart re-embedded the full
// Calibri regular and bold faces as base64 (go-slide-creator-i0ep).
func TestChartSVGEmbedsSubsettedFonts(t *testing.T) {
	req := &RequestEnvelope{
		Type:   "bar_chart",
		Title:  "Quarterly revenue by region",
		Output: OutputSpec{Width: 900, Height: 500},
		Data: map[string]any{
			"categories": []any{"Q1", "Q2", "Q3", "Q4"},
			"series": []any{
				map[string]any{"name": "EMEA", "values": []any{12.0, 18.0, 21.0, 26.0}},
				map[string]any{"name": "AMER", "values": []any{20.0, 22.0, 24.0, 31.0}},
			},
		},
	}
	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	svg := string(doc.Content)

	if len(svg) > maxChartSVGBytes {
		t.Errorf("chart SVG is %d bytes, want under %d — is font subsetting off?", len(svg), maxChartSVGBytes)
	}

	blobs := svgBase64Blob.FindAllStringSubmatch(svg, -1)
	if len(blobs) == 0 {
		t.Skip("no fonts embedded on this platform; nothing to size")
	}
	for i, m := range blobs {
		if n := len(m[1]); n > maxEmbeddedFontBytes {
			t.Errorf("embedded font %d is %d base64 bytes, want under %d — the whole family is being embedded",
				i, n, maxEmbeddedFontBytes)
		}
	}

	// The subset must still be a usable face: the block declares a family the
	// drawing's font-family references, so text does not fall back silently.
	if !strings.Contains(svg, "@font-face") {
		t.Error("expected an @font-face block alongside the embedded font data")
	}
}

// TestSVGRenderOptionsSubsetFonts states the single deliberate difference from
// the canvas renderer's defaults, so flipping it back is a visible edit rather
// than a silent 30x size regression.
func TestSVGRenderOptionsSubsetFonts(t *testing.T) {
	if !svgRenderOptions.SubsetFonts {
		t.Error("SubsetFonts must stay on; see go-slide-creator-i0ep")
	}
	if !svgRenderOptions.EmbedFonts {
		t.Error("EmbedFonts must stay on, or charts lose their typeface in viewers without it")
	}
	if svgRenderOptions.SizeUnits != "mm" {
		t.Errorf("SizeUnits = %q, want mm: the builder renders in millimetres and rescales afterwards",
			svgRenderOptions.SizeUnits)
	}
}
