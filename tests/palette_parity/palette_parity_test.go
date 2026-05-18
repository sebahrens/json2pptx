// Package paletteparity provides a cross-engine palette parity end-to-end test.
//
// The test renders a single deck whose slide contains:
//   - a shape_grid cell filled with the template's accent1 color (native OOXML), and
//   - a shape_grid cell containing a single-bar svggen bar_chart that draws its
//     bar from the same resolved palette.
//
// It then drives the full render path (json2pptx → LibreOffice → pdftoppm) to
// produce a PNG, samples the dominant accent-colored pixels in each cell, and
// asserts that the two regions differ by no more than a small CIE76 ΔE.
//
// This guards the boundary called out in go-slide-creator-wbc7: native shape
// fills and svggen chart fills must come from the same palette, otherwise an
// agent that pairs the two on one slide sees silent color drift.
//
// The test is opt-in: it runs only when the PALETTE_PARITY_TEST environment
// variable is set, and skips gracefully when LibreOffice or pdftoppm are not
// installed. CI enables it in a dedicated job that installs both tools.
package paletteparity_test

import (
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// region describes a rectangular pixel area to sample from the rendered slide.
// All coordinates are fractions of the image's width/height so the region is
// resolution-independent.
type region struct {
	name                   string
	x0, y0, x1, y1 float64
}

// dominantColor returns the average sRGB color of the "colorful" pixels in a
// rectangular region. A pixel is considered colorful when its chroma
// (max channel − min channel) exceeds chromaMin. This filters out the white
// chart background, gray axis lines, and black axis labels while keeping the
// accent1-filled rectangle and the accent1-colored bar.
//
// If no colorful pixels are found the function returns (0,0,0,0).
func dominantColor(img image.Image, r region, chromaMin uint8) (rr, gg, bb uint32, count int) {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	x0 := b.Min.X + int(r.x0*float64(w))
	y0 := b.Min.Y + int(r.y0*float64(h))
	x1 := b.Min.X + int(r.x1*float64(w))
	y1 := b.Min.Y + int(r.y1*float64(h))
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	var sumR, sumG, sumB uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := img.At(x, y)
			ri, gi, bi, _ := c.RGBA()
			// RGBA returns 16-bit values; shift to 8-bit.
			r8 := uint8(ri >> 8)
			g8 := uint8(gi >> 8)
			b8 := uint8(bi >> 8)
			maxc := r8
			if g8 > maxc {
				maxc = g8
			}
			if b8 > maxc {
				maxc = b8
			}
			minc := r8
			if g8 < minc {
				minc = g8
			}
			if b8 < minc {
				minc = b8
			}
			if maxc-minc < chromaMin {
				continue
			}
			sumR += uint64(r8)
			sumG += uint64(g8)
			sumB += uint64(b8)
			count++
		}
	}
	if count == 0 {
		return 0, 0, 0, 0
	}
	return uint32(sumR / uint64(count)), uint32(sumG / uint64(count)), uint32(sumB / uint64(count)), count
}

// srgbToLinear converts an 8-bit sRGB channel to linear light.
func srgbToLinear(v uint32) float64 {
	c := float64(v) / 255.0
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// rgbToLab converts sRGB (0..255 ints) to CIE Lab using D65.
func rgbToLab(r, g, b uint32) (L, a, bb float64) {
	rl := srgbToLinear(r)
	gl := srgbToLinear(g)
	bl := srgbToLinear(b)
	// sRGB → XYZ (D65)
	x := rl*0.4124564 + gl*0.3575761 + bl*0.1804375
	y := rl*0.2126729 + gl*0.7151522 + bl*0.0721750
	z := rl*0.0193339 + gl*0.1191920 + bl*0.9503041
	// Normalize by D65 white point
	const (
		xn = 0.95047
		yn = 1.00000
		zn = 1.08883
	)
	fx := labF(x / xn)
	fy := labF(y / yn)
	fz := labF(z / zn)
	L = 116*fy - 16
	a = 500 * (fx - fy)
	bb = 200 * (fy - fz)
	return
}

func labF(t float64) float64 {
	const epsilon = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0
	if t > epsilon {
		return math.Cbrt(t)
	}
	return (kappa*t + 16) / 116
}

// deltaE76 computes the CIE76 Euclidean ΔE between two Lab colors.
func deltaE76(l1, a1, b1, l2, a2, b2 float64) float64 {
	dl := l1 - l2
	da := a1 - a2
	db := b1 - b2
	return math.Sqrt(dl*dl + da*da + db*db)
}

func TestPaletteParity_CrossEngine(t *testing.T) {
	if os.Getenv("PALETTE_PARITY_TEST") == "" {
		t.Skip("PALETTE_PARITY_TEST not set; this opt-in E2E test runs in CI only")
	}
	for _, bin := range []string{"libreoffice", "pdftoppm"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("required binary %q not on PATH: %v", bin, err)
		}
	}

	// Repo root: tests/palette_parity → ../../
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(thisFile)
	repoRoot := filepath.Clean(filepath.Join(pkgDir, "..", ".."))
	templatesDir := filepath.Join(repoRoot, "templates")
	deckJSON := filepath.Join(pkgDir, "deck.json")

	tmp := t.TempDir()

	// Build json2pptx into the temp dir.
	binPath := filepath.Join(tmp, "json2pptx")
	build := exec.Command("go", "build", "-o", binPath, "./cmd/json2pptx") //nolint:gosec // test code with controlled args
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build json2pptx failed: %v\n%s", err, out)
	}

	// Generate the PPTX.
	gen := exec.Command(binPath, "generate", //nolint:gosec // test code with controlled args
		"-json", deckJSON,
		"-template", "midnight-blue",
		"-templates-dir", templatesDir,
		"-output", tmp,
	)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("json2pptx generate failed: %v\n%s", err, out)
	}

	pptxPath := filepath.Join(tmp, "palette_parity.pptx")
	if _, err := os.Stat(pptxPath); err != nil {
		// Fall back to any *.pptx the generator produced.
		matches, _ := filepath.Glob(filepath.Join(tmp, "*.pptx"))
		if len(matches) == 0 {
			t.Fatalf("no PPTX produced in %s", tmp)
		}
		pptxPath = matches[0]
	}

	// Convert to PDF.
	pdfDir := filepath.Join(tmp, "pdf")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	conv := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", pdfDir, pptxPath) //nolint:gosec // test code with controlled args
	if out, err := conv.CombinedOutput(); err != nil {
		t.Fatalf("libreoffice convert failed: %v\n%s", err, out)
	}
	pdfMatches, err := filepath.Glob(filepath.Join(pdfDir, "*.pdf"))
	if err != nil || len(pdfMatches) == 0 {
		t.Fatalf("no PDF produced in %s: %v", pdfDir, err)
	}
	pdfPath := pdfMatches[0]

	// Convert PDF to PNG at a fixed scale so coordinates are predictable.
	pngPrefix := filepath.Join(tmp, "slide")
	rast := exec.Command("pdftoppm", "-png", "-scale-to", "1920", pdfPath, pngPrefix) //nolint:gosec // test code with controlled args
	if out, err := rast.CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm failed: %v\n%s", err, out)
	}
	pngMatches, err := filepath.Glob(pngPrefix + "*.png")
	if err != nil || len(pngMatches) == 0 {
		t.Fatalf("pdftoppm produced no PNG: %v", err)
	}
	// Use slide-1 (single slide deck).
	slidePNG := pngMatches[0]

	f, err := os.Open(slidePNG)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}

	// Sampling regions are well inside each cell so we never straddle the
	// cell boundary (which is a thin gray line at gap=0 between the rect and
	// the chart's plot frame). Coordinates are fractions of slide dimensions.
	//
	// shape_grid bounds in deck.json: x=10%, y=30%, w=80%, h=55%.
	//   → grid is roughly (0.10, 0.30) to (0.90, 0.85)
	//   → cell A (left rect)   ≈ (0.10, 0.30) to (0.50, 0.85)
	//   → cell B (bar chart)   ≈ (0.50, 0.30) to (0.90, 0.85)
	//
	// We shrink each region by ~5% on every side to avoid cell borders and
	// chart frame chrome.
	rects := []region{
		// Interior of the accent1-filled rectangle.
		{name: "shape_accent1", x0: 0.20, y0: 0.40, x1: 0.45, y1: 0.75},
		// Interior of the bar-chart plot area: the bar of a single-category,
		// single-value bar_chart sits near the horizontal centre of the cell
		// and rises from the baseline to the top of the plot frame.
		{name: "bar_chart_accent1", x0: 0.62, y0: 0.40, x1: 0.78, y1: 0.75},
	}

	const chromaMin uint8 = 25

	type sample struct {
		region region
		r, g, b uint32
		count   int
	}
	samples := make([]sample, len(rects))
	for i, r := range rects {
		sr, sg, sb, n := dominantColor(img, r, chromaMin)
		samples[i] = sample{region: r, r: sr, g: sg, b: sb, count: n}
		if n == 0 {
			t.Fatalf("region %q: no colorful pixels found (chromaMin=%d); the deck may have rendered without an accent fill", r.name, chromaMin)
		}
		t.Logf("region %s: avg sRGB=(%d,%d,%d) over %d pixels", r.name, sr, sg, sb, n)
	}

	l1, a1, b1 := rgbToLab(samples[0].r, samples[0].g, samples[0].b)
	l2, a2, b2 := rgbToLab(samples[1].r, samples[1].g, samples[1].b)
	dE := deltaE76(l1, a1, b1, l2, a2, b2)

	threshold := 2.0
	if v := os.Getenv("PALETTE_PARITY_MAX_DELTAE"); v != "" {
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			threshold = parsed
		}
	}

	t.Logf("ΔE76 between shape_accent1 and bar_chart_accent1 = %.3f (threshold %.3f)", dE, threshold)
	if dE > threshold {
		t.Fatalf("palette parity violated: ΔE=%.3f > %.3f\nshape sRGB=(%d,%d,%d) chart sRGB=(%d,%d,%d)\nartifacts: %s",
			dE, threshold,
			samples[0].r, samples[0].g, samples[0].b,
			samples[1].r, samples[1].g, samples[1].b,
			slidePNG,
		)
	}
}
