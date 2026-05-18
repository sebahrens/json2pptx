package main

import (
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestExtractPaletteRegions_PicAndSolidFillShape covers the slide XML parser:
// it must find <p:pic> with explicit xfrm and <p:sp> with both xfrm + solidFill,
// and ignore shapes that lack either.
func TestExtractPaletteRegions_PicAndSolidFillShape(t *testing.T) {
	xmlBody := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="AccentRect"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="100" y="200"/><a:ext cx="3000000" cy="2000000"/></a:xfrm>
          <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
          <a:solidFill><a:srgbClr val="2E5090"/></a:solidFill>
        </p:spPr>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="3" name="NoFill"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm>
        </p:spPr>
      </p:sp>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="4" name="SchemeFill"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>
        <p:spPr>
          <a:xfrm><a:off x="500" y="600"/><a:ext cx="100" cy="100"/></a:xfrm>
          <a:solidFill><a:schemeClr val="accent1"/></a:solidFill>
        </p:spPr>
      </p:sp>
      <p:pic>
        <p:nvPicPr><p:cNvPr id="5" name="ChartImage"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
        <p:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>
        <p:spPr>
          <a:xfrm><a:off x="6000000" y="200"/><a:ext cx="3000000" cy="2000000"/></a:xfrm>
        </p:spPr>
      </p:pic>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	pics, shapes, err := extractPaletteRegions(xmlBody)
	if err != nil {
		t.Fatalf("extractPaletteRegions: %v", err)
	}
	if len(pics) != 1 || pics[0].Name != "ChartImage" {
		t.Fatalf("pics = %+v, want 1 ChartImage", pics)
	}
	if pics[0].X != 6000000 || pics[0].CX != 3000000 {
		t.Errorf("pic bounds = (%d, %d, %d, %d), want (6000000, 200, 3000000, 2000000)",
			pics[0].X, pics[0].Y, pics[0].CX, pics[0].CY)
	}
	if len(shapes) != 2 {
		t.Fatalf("shapes = %+v, want 2 (AccentRect + SchemeFill)", shapes)
	}
	wantNames := map[string]bool{"AccentRect": false, "SchemeFill": false}
	wantHex := map[string]string{"AccentRect": "2e5090", "SchemeFill": "accent1"}
	for _, s := range shapes {
		if _, ok := wantNames[s.Name]; !ok {
			t.Errorf("unexpected shape %q (NoFill should be skipped)", s.Name)
		}
		wantNames[s.Name] = true
		if got := wantHex[s.Name]; s.DeclaredHex != got {
			t.Errorf("shape %s declared = %q, want %q", s.Name, s.DeclaredHex, got)
		}
	}
	for name, seen := range wantNames {
		if !seen {
			t.Errorf("expected shape %q not present", name)
		}
	}
}

// TestDeltaE76Hex_Identity verifies ΔE=0 for identical colors and >0 for
// distinct colors; this is the parity guard against the wrapper drifting from
// its CIE76 contract.
func TestDeltaE76Hex_Identity(t *testing.T) {
	if d := deltaE76Hex(0x2E, 0x50, 0x90, 0x2E, 0x50, 0x90); d != 0 {
		t.Errorf("ΔE(identical) = %v, want 0", d)
	}
	d := deltaE76Hex(0x2E, 0x50, 0x90, 0xC4, 0x3F, 0x3F)
	if d <= 0 {
		t.Errorf("ΔE(differs) = %v, want > 0", d)
	}
	// Sanity check: ΔE between near-identical sRGB triples is small.
	d2 := deltaE76Hex(0x2E, 0x50, 0x90, 0x2F, 0x51, 0x91)
	if d2 > 2.0 {
		t.Errorf("ΔE(near-identical) = %v, want ≤ 2.0", d2)
	}
}

// TestDominantColorPx_FiltersWhiteBackground builds a tiny image with a
// 6×6 white background and a 2×2 blue square in the centre, then asserts the
// dominant-color sampler returns the blue pixel value (white is filtered by
// chromaMin).
func TestDominantColorPx_FiltersWhiteBackground(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 6, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 6; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	// 2x2 blue square at (2,2)..(3,3).
	for y := 2; y <= 3; y++ {
		for x := 2; x <= 3; x++ {
			img.Set(x, y, color.RGBA{46, 80, 144, 255})
		}
	}
	r, g, b, n := dominantColorPx(img, 0, 0, 6, 6, 25)
	if n == 0 {
		t.Fatalf("no colorful pixels found")
	}
	if r != 46 || g != 80 || b != 144 {
		t.Errorf("dominant = (%d,%d,%d), want (46,80,144)", r, g, b)
	}
	if n != 4 {
		t.Errorf("pixel count = %d, want 4 (the blue square only)", n)
	}
}

// TestEmuRectToPx_HalfwayScales asserts that a centred 50%-wide EMU rect maps
// to the right half of the image bounds.
func TestEmuRectToPx_HalfwayScales(t *testing.T) {
	x0, y0, x1, y1 := emuRectToPx(6096000, 0, 6096000, 6858000, 12192000, 6858000, 1920, 1080)
	if x0 != 960 || y0 != 0 || x1 != 1920 || y1 != 1080 {
		t.Errorf("rect = (%d,%d,%d,%d), want (960,0,1920,1080)", x0, y0, x1, y1)
	}
}

// TestRGBToLabRoundTrip_Black asserts the CIE conversion for pure black.
func TestRGBToLabRoundTrip_Black(t *testing.T) {
	L, a, b := rgbToLab(0, 0, 0)
	if math.Abs(L) > 1e-6 || math.Abs(a) > 1e-6 || math.Abs(b) > 1e-6 {
		t.Errorf("Lab(black) = (%v,%v,%v), want ~(0,0,0)", L, a, b)
	}
}

// TestRunAuditPalette_EndToEnd is opt-in (PALETTE_PARITY_TEST=1) and reuses the
// palette_parity smoke deck. It validates that the CLI runs without error and
// emits a JSON report containing at least one (pic, shape) pair.
func TestRunAuditPalette_EndToEnd(t *testing.T) {
	if os.Getenv("PALETTE_PARITY_TEST") == "" {
		t.Skip("PALETTE_PARITY_TEST not set; this opt-in E2E test runs in CI only")
	}
	for _, bin := range []string{"libreoffice", "pdftoppm"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("required binary %q not on PATH: %v", bin, err)
		}
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	templatesDir := filepath.Join(repoRoot, "templates")
	deckJSON := filepath.Join(repoRoot, "tests", "palette_parity", "deck.json")

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "json2pptx")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/json2pptx") //nolint:gosec // controlled args
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	gen := exec.Command(binPath, "generate", //nolint:gosec // controlled args
		"-json", deckJSON,
		"-template", "midnight-blue",
		"-templates-dir", templatesDir,
		"-output", tmp,
	)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	matches, _ := filepath.Glob(filepath.Join(tmp, "*.pptx"))
	if len(matches) == 0 {
		t.Fatalf("no PPTX produced")
	}

	report, err := auditPalettePPTX(matches[0], auditOptions{
		MaxDeltaE: 5.0,
		ChromaMin: 25,
		Density:   150,
		TmpDir:    filepath.Join(tmp, "render"),
		Keep:      true,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if report.SlideCount == 0 {
		t.Fatalf("report has no slides: %+v", report)
	}
	totalPairs := 0
	for _, s := range report.Slides {
		totalPairs += s.PairCount
	}
	if totalPairs == 0 {
		t.Fatalf("report has no (pic, shape) pairs; smoke deck should have at least one. Report: %+v", report)
	}
	if report.Violations != 0 {
		// Surface the first violation for debugging.
		var first auditPair
		for _, s := range report.Slides {
			for _, p := range s.Pairs {
				if !p.Pass {
					first = p
					break
				}
			}
			if first.DeltaE > 0 {
				break
			}
		}
		t.Fatalf("expected 0 violations against threshold 5.0; got %d. First: slide=%d ΔE=%.3f pic=#%s shape=#%s",
			report.Violations, first.Slide, first.DeltaE, first.Pic.Hex, first.Shape.Hex)
	}
}

// TestEmitAuditReport_TextFormat checks that the text formatter renders a
// recognizable summary so the CI logs are useful when the report is huge.
func TestEmitAuditReport_TextFormat(t *testing.T) {
	r := &auditReport{
		PPTX:             "/tmp/x.pptx",
		SlideCount:       1,
		MaxDeltaEAllowed: 5.0,
		ChromaMin:        25,
		Density:          150,
		Slides: []auditSlide{{
			Index: 1, PicCount: 1, ShapeCount: 1, PairCount: 1, MaxDeltaE: 1.2,
			Pairs: []auditPair{{
				Slide: 1,
				Pic:   auditRegion{Kind: "pic", Name: "Chart", Hex: "2e5090"},
				Shape: auditRegion{Kind: "shape", Name: "Rect", Hex: "2e5091", DeclaredHex: "accent1"},
				DeltaE: 1.2, Pass: true,
			}},
		}},
	}
	out := formatAuditText(r)
	for _, want := range []string{
		"Palette audit:",
		"Result: PASS",
		"slide 1:",
		"pic[Chart]=#2e5090",
		"shape[Rect]=#2e5091",
		"declared #accent1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q. got:\n%s", want, out)
		}
	}
}
