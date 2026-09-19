package heuristic

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// go-slide-creator-3pyf: the edge-band check reported 14 text_overflow findings
// on the best deck in the calibration set and 10 on the worst — an inverted
// signal. Every one was the template's own decoration: "100.0% of pixels in the
// left edge" is midnight-blue's accent rail, "16.7% in the right edge" its
// takeaway band. An agent without a vision key learned to ignore the visual
// channel, the only one that correlates with quality.
func TestEdgeBandIgnoresSolidDecoration(t *testing.T) {
	img := blankImage(400, 300, color.White)
	// A solid accent rail down the left edge, exactly what a template draws.
	fillRect(img, 0, 0, 12, 300, color.RGBA{R: 0x1F, G: 0x3A, B: 0x93, A: 255})

	res := Inspect(encodePNG(t, img), visualqa.SlideInfo{Index: 0})
	for _, f := range res.Findings {
		if f.Category == "text_overflow" {
			t.Errorf("solid decoration reported as overflow: %s", f.Description)
		}
	}
}

// Text running into the edge alternates with the background many times per
// scan line and is still reported.
func TestEdgeBandReportsTextLikeInk(t *testing.T) {
	img := blankImage(400, 300, color.White)
	// Glyph-like vertical strokes crossing the left band.
	for y := 20; y < 280; y += 6 {
		for x := 0; x < 10; x += 3 {
			fillRect(img, x, y, x+1, y+4, color.RGBA{A: 255})
		}
	}

	res := Inspect(encodePNG(t, img), visualqa.SlideInfo{Index: 0})
	found := false
	for _, f := range res.Findings {
		if f.Category == "text_overflow" {
			found = true
		}
	}
	if !found {
		t.Error("text-like ink in the edge band was not reported")
	}
}

// The blank detector — the check that actually carries signal — is untouched.
func TestBlankSlideStillReported(t *testing.T) {
	img := blankImage(400, 300, color.White)
	res := Inspect(encodePNG(t, img), visualqa.SlideInfo{Index: 0})
	found := false
	for _, f := range res.Findings {
		if f.Category == "missing_content" {
			found = true
		}
	}
	if !found {
		t.Errorf("a blank slide produced no missing_content finding: %+v", res.Findings)
	}
}

func blankImage(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fillRect(img, 0, 0, w, h, c)
	return img
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.Set(x, y, c)
		}
	}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
