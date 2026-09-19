package heuristic

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// solidPNG returns a PNG of dimensions w×h filled with the given colour.
func solidPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// edgeFilledPNG paints a black band of `band` pixels along the named edge
// of an otherwise-white image of dimensions w×h.
func edgeFilledPNG(t *testing.T, w, h, band int, edge string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, white)
		}
	}
	switch edge {
	case "top":
		for y := 0; y < band; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, black)
			}
		}
	case "bottom":
		for y := h - band; y < h; y++ {
			for x := 0; x < w; x++ {
				img.SetRGBA(x, y, black)
			}
		}
	case "left":
		for y := 0; y < h; y++ {
			for x := 0; x < band; x++ {
				img.SetRGBA(x, y, black)
			}
		}
	case "right":
		for y := 0; y < h; y++ {
			for x := w - band; x < w; x++ {
				img.SetRGBA(x, y, black)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestInspect_BlankSlideFlagged: a fully-white 16:9 image should produce a
// blank/missing_content finding and nothing else.
func TestInspect_BlankSlideFlagged(t *testing.T) {
	// 1920x1080 is exact 16:9 — no aspect ratio finding expected.
	data := solidPNG(t, 1920, 1080, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	res := Inspect(data, visualqa.SlideInfo{Index: 0, Type: "content"})
	if res.Error != "" {
		t.Fatalf("Inspect error: %s", res.Error)
	}
	if len(res.Findings) == 0 {
		t.Fatal("expected at least one finding for blank slide")
	}
	gotBlank := false
	for _, f := range res.Findings {
		if f.Category == "missing_content" {
			gotBlank = true
		}
		if f.Source != SourceTag {
			t.Errorf("finding %+v not tagged heuristic", f)
		}
		if f.Severity != visualqa.SeverityP3 {
			t.Errorf("finding %+v severity = %s, want P3", f, f.Severity)
		}
	}
	if !gotBlank {
		t.Errorf("expected missing_content finding, got: %+v", res.Findings)
	}
}

// TestInspect_EdgeOverflowFlagged: a fully-white image with TEXT-LIKE ink along
// the bottom should trigger an overflow finding on the bottom edge.
//
// The fixture used to be a solid black band, which is what a template's own
// footer or accent bar looks like — reporting those inverted the signal across
// the calibration decks (go-slide-creator-3pyf), so the check now requires ink
// that alternates with the background the way glyphs do.
func TestInspect_EdgeOverflowFlagged(t *testing.T) {
	// Band must exceed the 1% edge-band fraction (10% chosen for clarity).
	data := edgeTextPNG(t, 1920, 1080, 100, "bottom")
	res := Inspect(data, visualqa.SlideInfo{Index: 1, Type: "content"})
	if res.Error != "" {
		t.Fatalf("Inspect error: %s", res.Error)
	}
	gotOverflow := false
	for _, f := range res.Findings {
		if f.Category == "text_overflow" && f.Location == "bottom edge" {
			gotOverflow = true
		}
	}
	if !gotOverflow {
		t.Errorf("expected bottom-edge text_overflow finding, got: %+v", res.Findings)
	}
}

// TestInspect_AspectRatioFlagged: a square image should trigger the aspect
// ratio finding.
func TestInspect_AspectRatioFlagged(t *testing.T) {
	data := solidPNG(t, 1000, 1000, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	res := Inspect(data, visualqa.SlideInfo{Index: 2, Type: "content"})
	gotAspect := false
	for _, f := range res.Findings {
		if f.Category == "aspect_ratio" {
			gotAspect = true
		}
	}
	if !gotAspect {
		t.Errorf("expected aspect_ratio finding for 1:1 image, got: %+v", res.Findings)
	}
}

// TestInspect_DecodeError surfaces a clear error for non-image bytes.
func TestInspect_DecodeError(t *testing.T) {
	res := Inspect([]byte("not an image"), visualqa.SlideInfo{Index: 3, Type: "content"})
	if res.Error == "" {
		t.Error("expected decode error for non-image bytes")
	}
	if len(res.Findings) != 0 {
		t.Errorf("expected no findings on decode error, got: %+v", res.Findings)
	}
}

// TestInspectAll_ReportMode: the Report.Mode must be "heuristic".
func TestInspectAll_ReportMode(t *testing.T) {
	data := solidPNG(t, 1920, 1080, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	rep := InspectAll([]visualqa.SlideImage{
		{Info: visualqa.SlideInfo{Index: 0, Type: "content"}, Data: data},
	})
	if rep.Mode != SourceTag {
		t.Errorf("Report.Mode = %q, want %q", rep.Mode, SourceTag)
	}
	if rep.SlideCount != 1 {
		t.Errorf("SlideCount = %d, want 1", rep.SlideCount)
	}
}

// edgeTextPNG paints glyph-like strokes into an edge band: ink that alternates
// with the background many times per scan line, unlike a solid decoration.
func edgeTextPNG(t *testing.T, w, h, band int, edge string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, white)
		}
	}
	stroke := func(x0, y0, x1, y1 int) {
		for y := y0; y < y1 && y < h; y++ {
			for x := x0; x < x1 && x < w; x++ {
				img.SetRGBA(x, y, black)
			}
		}
	}
	switch edge {
	case "bottom":
		for y := h - band; y < h; y++ {
			for x := 0; x < w; x += 8 {
				stroke(x, y, x+4, y+1)
			}
		}
	case "top":
		for y := 0; y < band; y++ {
			for x := 0; x < w; x += 8 {
				stroke(x, y, x+4, y+1)
			}
		}
	case "left":
		for x := 0; x < band; x++ {
			for y := 0; y < h; y += 8 {
				stroke(x, y, x+1, y+4)
			}
		}
	case "right":
		for x := w - band; x < w; x++ {
			for y := 0; y < h; y += 8 {
				stroke(x, y, x+1, y+4)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// A solid band is template decoration, not overflow.
func TestInspect_SolidEdgeBandIsNotOverflow(t *testing.T) {
	data := edgeFilledPNG(t, 1920, 1080, 100, "left")
	res := Inspect(data, visualqa.SlideInfo{Index: 0, Type: "content"})
	for _, f := range res.Findings {
		if f.Category == "text_overflow" {
			t.Errorf("solid edge band reported as overflow: %s", f.Description)
		}
	}
}
