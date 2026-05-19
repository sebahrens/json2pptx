package svggen

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

// TestRenderWireframe_EmptySlide verifies the renderer produces a valid
// SVG with the slide frame and header even when no cells/placeholders/
// findings are present.
func TestRenderWireframe_EmptySlide(t *testing.T) {
	req := &WireframeRequest{
		SlideIndex:    2,
		LayoutID:      "slideLayout3",
		LayoutName:    "Title and Content",
		SlideType:     "content",
		TemplateName:  "midnight-blue",
		SlideWidth:    9144000,
		SlideHeight:   6858000,
		OutputWidthPx: 800,
	}

	out, err := RenderWireframe(req, RenderWireframeOptions{IncludeSVG: true})
	if err != nil {
		t.Fatalf("RenderWireframe error: %v", err)
	}
	if len(out.SVG) == 0 {
		t.Fatalf("expected non-empty SVG, got 0 bytes")
	}
	svg := string(out.SVG)
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Fatalf("SVG missing root element: %s", truncForLog(svg, 200))
	}
	// Header text must mention the slide number and layout name.
	if !strings.Contains(svg, "Slide 3") {
		t.Errorf("SVG does not contain expected 'Slide 3' header label")
	}
	if !strings.Contains(svg, "Title and Content") {
		t.Errorf("SVG does not contain expected layout name")
	}
}

// TestRenderWireframe_CellsAndFindings exercises the full path: cells,
// placeholders, and a mix of cell-attached + off-cell findings.
func TestRenderWireframe_CellsAndFindings(t *testing.T) {
	req := &WireframeRequest{
		SlideIndex:   0,
		LayoutID:     "slideLayout2",
		SlideWidth:   9144000,
		SlideHeight:  6858000,
		TemplateName: "midnight-blue",
		Placeholders: []WireframePlaceholder{
			{ID: "title", Rect: WireframeRect{X: 457200, Y: 365125, W: 8229600, H: 1143000}},
			{ID: "body", Rect: WireframeRect{X: 457200, Y: 1600200, W: 8229600, H: 4525963}, Remapped: true},
		},
		Cells: []WireframeCell{
			{Row: 0, Col: 0, Kind: "shape", Rect: WireframeRect{X: 457200, Y: 1600200, W: 4114800, H: 2262981}},
			{Row: 0, Col: 1, Kind: "table", Rect: WireframeRect{X: 4572000, Y: 1600200, W: 4114800, H: 2262981}},
			{Row: 1, Col: 0, Kind: "chart", Rect: WireframeRect{X: 457200, Y: 3863181, W: 8229600, H: 2262982}},
		},
		Findings: []WireframeFinding{
			{Code: "TABLE_OVERFLOW", Action: "shrink_or_split", Message: "row count exceeds capacity", HasCell: true, Row: 0, Col: 1},
			{Code: "PLACEHOLDER_REMAPPED", Action: "info", Message: "body remapped from virtual"},
			{Code: "OCCUPANCY_LOW", Action: "review", Message: "<40% filled — consider denser pattern"},
		},
		Occupancy: &WireframeOccupancy{FilledPct: 50, FilledSlots: 3, TotalSlots: 6},
	}
	out, err := RenderWireframe(req, RenderWireframeOptions{IncludeSVG: true, IncludePNG: true, PNGScale: 1.0})
	if err != nil {
		t.Fatalf("RenderWireframe error: %v", err)
	}
	if len(out.SVG) == 0 {
		t.Fatalf("expected SVG output")
	}
	if len(out.PNG) == 0 {
		t.Fatalf("expected PNG output")
	}
	// PNG must decode as a valid image.
	if _, err := png.Decode(bytes.NewReader(out.PNG)); err != nil {
		t.Fatalf("invalid PNG bytes: %v", err)
	}
	svg := string(out.SVG)
	// Cell tags r0,c0..r1,c0 should appear.
	for _, want := range []string{"r0,c0", "r0,c1", "r1,c0", "shape", "table", "chart"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG missing %q label", want)
		}
	}
	// Off-cell findings render code+message in the footer strip.
	if !strings.Contains(svg, "PLACEHOLDER_REMAPPED") {
		t.Errorf("SVG missing PLACEHOLDER_REMAPPED footer label")
	}
	if !strings.Contains(svg, "OCCUPANCY_LOW") {
		t.Errorf("SVG missing OCCUPANCY_LOW footer label")
	}
	// Severity badge for the cell-attached finding.
	if !strings.Contains(svg, "SHR") {
		t.Errorf("SVG missing SHR badge for shrink_or_split action")
	}
	// Occupancy label.
	if !strings.Contains(svg, "fill: 50%") {
		t.Errorf("SVG missing occupancy label, got: %s", truncForLog(svg, 400))
	}
}

// TestRenderWireframe_InvalidInput verifies error paths.
func TestRenderWireframe_InvalidInput(t *testing.T) {
	if _, err := RenderWireframe(nil, RenderWireframeOptions{IncludeSVG: true}); err == nil {
		t.Fatalf("expected error for nil request")
	}
	if _, err := RenderWireframe(&WireframeRequest{SlideWidth: 0, SlideHeight: 100}, RenderWireframeOptions{IncludeSVG: true}); err == nil {
		t.Fatalf("expected error for zero SlideWidth")
	}
}

// TestRenderWireframe_OverlayOnly verifies the overlay path emits a
// transparent-background SVG with no header strip or footer strip, sized
// to the slide aspect ratio. Cell tags and severity badges still render.
func TestRenderWireframe_OverlayOnly(t *testing.T) {
	req := &WireframeRequest{
		SlideIndex:    0,
		SlideWidth:    9144000,
		SlideHeight:   6858000,
		OutputWidthPx: 800,
		Cells: []WireframeCell{
			{Row: 0, Col: 0, Kind: "shape", Rect: WireframeRect{X: 457200, Y: 457200, W: 4114800, H: 2262981}},
			{Row: 0, Col: 1, Kind: "table", Rect: WireframeRect{X: 4572000, Y: 457200, W: 4114800, H: 2262981}},
		},
		Findings: []WireframeFinding{
			{Code: "TABLE_OVERFLOW", Action: "shrink_or_split", Message: "too dense", HasCell: true, Row: 0, Col: 1},
			{Code: "DECK_ISSUE", Action: "review", Message: "off-cell"},
		},
	}
	out, err := RenderWireframe(req, RenderWireframeOptions{
		IncludeSVG:  true,
		IncludePNG:  true,
		PNGScale:    1.0,
		OverlayOnly: true,
	})
	if err != nil {
		t.Fatalf("RenderWireframe overlay error: %v", err)
	}
	if len(out.SVG) == 0 {
		t.Fatalf("expected overlay SVG bytes")
	}
	if len(out.PNG) == 0 {
		t.Fatalf("expected overlay PNG bytes")
	}
	img, err := png.Decode(bytes.NewReader(out.PNG))
	if err != nil {
		t.Fatalf("invalid overlay PNG: %v", err)
	}
	// Aspect ratio of the overlay PNG must equal slide aspect (no header
	// or footer strips added).
	pw := img.Bounds().Dx()
	ph := img.Bounds().Dy()
	wantAspect := req.SlideWidth / req.SlideHeight
	gotAspect := float64(pw) / float64(ph)
	if rel := (gotAspect - wantAspect) / wantAspect; rel > 0.02 || rel < -0.02 {
		t.Errorf("overlay aspect %.4f differs from slide aspect %.4f", gotAspect, wantAspect)
	}
	svg := string(out.SVG)
	// Header text must NOT appear in overlay mode.
	if strings.Contains(svg, "Slide 1") {
		t.Errorf("overlay SVG unexpectedly contains header label 'Slide 1'")
	}
	// Cell tags and badge present.
	for _, want := range []string{"r0,c0", "r0,c1", "SHR"} {
		if !strings.Contains(svg, want) {
			t.Errorf("overlay SVG missing %q", want)
		}
	}
}

func truncForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
