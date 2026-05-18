package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// runAuditPalette implements `json2pptx audit-palette <pptx>`.
//
// The command renders a PPTX to PNG (via libreoffice + pdftoppm) and, for each
// slide, samples the dominant non-background color inside every <p:pic> region
// (typically svggen-rendered chart/diagram bitmaps) and every <p:sp> region
// whose <p:spPr> declares an explicit <a:solidFill>. It then computes the
// CIE76 ΔE between every (pic, shape) pair on the slide.
//
// This is the deterministic palette-diff that the Visual-QA agent cannot do:
// it catches silent drift where a native shape fill diverges from a chart
// embedded next to it, even when both came from the "same" template palette.
//
// Acceptance: AC1 emits ΔE per pair; AC2 — see .github/workflows/ci.yml
// `palette-parity` job, which now also runs this command against a smoke deck.
func runAuditPalette() error {
	fs := flag.NewFlagSet("audit-palette", flag.ContinueOnError)
	format := fs.String("format", "json", "Output format: json|text")
	maxDelta := fs.Float64("max-delta-e", 5.0, "Maximum allowed CIE76 ΔE for a (pic, shape) pair before the command exits non-zero")
	chromaMin := fs.Int("chroma-min", 25, "Minimum chroma (max-min channel) for a pixel to count toward the dominant color; filters white/black/gray chrome")
	density := fs.Int("density", 150, "DPI for pdftoppm rasterization")
	outputPath := fs.String("output", "", "Write JSON/text report to this file in addition to stdout")
	keep := fs.Bool("keep", false, "Keep temporary render artifacts (PDF/PNG) for debugging; default is to remove them on exit")
	tmpDirFlag := fs.String("tmp", "", "Use this directory for render artifacts (default: OS temp dir)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx audit-palette <pptx> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Render a PPTX to PNG and report CIE76 ΔE between every embedded\n")
		fmt.Fprintf(os.Stderr, "chart/picture region and every native solid-filled shape region per slide.\n\n")
		fmt.Fprintf(os.Stderr, "Requires `libreoffice` and `pdftoppm` on PATH.\n\n")
		fmt.Fprintf(os.Stderr, "Exit code is non-zero when any pair exceeds -max-delta-e.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return errors.New("missing required positional argument: <pptx>")
	}
	pptxPath := fs.Arg(0)

	if uint8Overflows(*chromaMin) {
		return fmt.Errorf("chroma-min must be in 0..255 (got %d)", *chromaMin)
	}

	report, err := auditPalettePPTX(pptxPath, auditOptions{
		MaxDeltaE: *maxDelta,
		ChromaMin: uint8(*chromaMin),
		Density:   *density,
		TmpDir:    *tmpDirFlag,
		Keep:      *keep,
	})
	if err != nil {
		return err
	}

	if err := emitAuditReport(report, *format, *outputPath); err != nil {
		return err
	}

	if report.Violations > 0 {
		return fmt.Errorf("palette audit: %d pair(s) exceeded ΔE threshold %.3f", report.Violations, *maxDelta)
	}
	return nil
}

func uint8Overflows(v int) bool { return v < 0 || v > 255 }

// --- Report shape -----------------------------------------------------------

type auditOptions struct {
	MaxDeltaE float64
	ChromaMin uint8
	Density   int
	TmpDir    string
	Keep      bool
}

type auditRegion struct {
	Kind       string  `json:"kind"`              // "pic" or "shape"
	Name       string  `json:"name,omitempty"`    // shape/picture cNvPr name
	BoundsEMU  [4]int64 `json:"bounds_emu"`       // x, y, cx, cy
	BoundsPx   [4]int   `json:"bounds_px"`        // x0, y0, x1, y1
	Hex        string   `json:"dominant_hex"`     // sampled dominant color
	R          uint8    `json:"r"`
	G          uint8    `json:"g"`
	B          uint8    `json:"b"`
	PixelCount int      `json:"pixel_count"`
	// DeclaredHex is the explicit srgbClr / accent indirection text for sp
	// regions, blank for pic regions. Best-effort, lower-cased without '#'.
	DeclaredHex string `json:"declared_hex,omitempty"`
}

type auditPair struct {
	Slide  int         `json:"slide"`         // 1-based
	Pic    auditRegion `json:"pic"`
	Shape  auditRegion `json:"shape"`
	DeltaE float64     `json:"delta_e"`
	Pass   bool        `json:"pass"`
}

type auditSlide struct {
	Index       int          `json:"index"`            // 1-based
	PicCount    int          `json:"pic_count"`
	ShapeCount  int          `json:"shape_count"`
	PairCount   int          `json:"pair_count"`
	MaxDeltaE   float64      `json:"max_delta_e"`
	Pairs       []auditPair  `json:"pairs"`
	RenderImage string       `json:"render_image,omitempty"`
}

type auditReport struct {
	PPTX             string       `json:"pptx"`
	SlideCount       int          `json:"slide_count"`
	MaxDeltaEAllowed float64      `json:"max_delta_e_allowed"`
	ChromaMin        uint8        `json:"chroma_min"`
	Density          int          `json:"density"`
	Slides           []auditSlide `json:"slides"`
	Violations       int          `json:"violations"`
}

// --- Top-level driver -------------------------------------------------------

func auditPalettePPTX(pptxPath string, opts auditOptions) (*auditReport, error) {
	abs, err := filepath.Abs(pptxPath)
	if err != nil {
		return nil, fmt.Errorf("resolve pptx path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("pptx not found: %w", err)
	}

	// Required external tools — fail early with a clear message.
	for _, bin := range []string{"libreoffice", "pdftoppm"} {
		if _, err := exec.LookPath(bin); err != nil {
			return nil, fmt.Errorf("required binary %q not found on PATH (install libreoffice + poppler-utils)", bin)
		}
	}

	// Open the PPTX and pull out slide geometry + region metadata.
	pkg, closer, err := pptx.OpenFile(abs)
	if err != nil {
		return nil, fmt.Errorf("open pptx: %w", err)
	}
	defer closer.Close()

	slideCX, slideCY, err := readSlideDimensionsEMU(pkg)
	if err != nil {
		return nil, fmt.Errorf("slide dimensions: %w", err)
	}

	enum, err := pptx.NewSlideEnumerator(pkg)
	if err != nil {
		return nil, fmt.Errorf("enumerate slides: %w", err)
	}

	type slideRegions struct {
		index  int // 1-based
		pics   []paletteShape
		shapes []paletteShape
	}
	var allSlides []slideRegions
	for _, info := range enum.Slides() {
		data, err := pkg.ReadEntry(info.PartPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", info.PartPath, err)
		}
		pics, shapes, err := extractPaletteRegions(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", info.PartPath, err)
		}
		allSlides = append(allSlides, slideRegions{index: info.Index + 1, pics: pics, shapes: shapes})
	}

	// Render the PPTX to one PNG per slide.
	tmp := opts.TmpDir
	cleanup := func() {}
	if tmp == "" {
		t, err := os.MkdirTemp("", "audit-palette-*")
		if err != nil {
			return nil, fmt.Errorf("mkdtemp: %w", err)
		}
		tmp = t
		if !opts.Keep {
			cleanup = func() { _ = os.RemoveAll(t) }
		}
	} else if err := os.MkdirAll(tmp, 0o755); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}
	defer cleanup()

	pngs, err := renderPPTXToPNGs(abs, tmp, opts.Density)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	if len(pngs) == 0 {
		return nil, fmt.Errorf("no PNGs produced from %s", abs)
	}

	report := &auditReport{
		PPTX:             abs,
		SlideCount:       len(allSlides),
		MaxDeltaEAllowed: opts.MaxDeltaE,
		ChromaMin:        opts.ChromaMin,
		Density:          opts.Density,
	}

	for i, sr := range allSlides {
		if i >= len(pngs) {
			break
		}
		img, err := decodePNG(pngs[i])
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", pngs[i], err)
		}
		bounds := img.Bounds()
		imgW := bounds.Dx()
		imgH := bounds.Dy()

		picRegions := samplePaletteRegions(img, sr.pics, slideCX, slideCY, imgW, imgH, opts.ChromaMin, "pic")
		shapeRegions := samplePaletteRegions(img, sr.shapes, slideCX, slideCY, imgW, imgH, opts.ChromaMin, "shape")

		slide := auditSlide{
			Index:       sr.index,
			PicCount:    len(picRegions),
			ShapeCount:  len(shapeRegions),
			RenderImage: pngs[i],
		}

		for _, p := range picRegions {
			for _, s := range shapeRegions {
				if p.PixelCount == 0 || s.PixelCount == 0 {
					continue
				}
				dE := deltaE76Hex(p.R, p.G, p.B, s.R, s.G, s.B)
				pass := dE <= opts.MaxDeltaE
				slide.Pairs = append(slide.Pairs, auditPair{
					Slide:  sr.index,
					Pic:    p,
					Shape:  s,
					DeltaE: dE,
					Pass:   pass,
				})
				if !pass {
					report.Violations++
				}
				if dE > slide.MaxDeltaE {
					slide.MaxDeltaE = dE
				}
			}
		}
		slide.PairCount = len(slide.Pairs)
		report.Slides = append(report.Slides, slide)
	}

	return report, nil
}

// --- Output emission --------------------------------------------------------

func emitAuditReport(report *auditReport, format, outputPath string) error {
	var payload []byte
	switch strings.ToLower(format) {
	case "", "json":
		buf, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal report: %w", err)
		}
		payload = append(buf, '\n')
	case "text":
		payload = []byte(formatAuditText(report))
	default:
		return fmt.Errorf("unknown format %q (want json|text)", format)
	}
	if _, err := os.Stdout.Write(payload); err != nil {
		return err
	}
	if outputPath != "" {
		if err := os.WriteFile(outputPath, payload, 0o644); err != nil { //nolint:gosec // user-specified path
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
	}
	return nil
}

func formatAuditText(r *auditReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Palette audit: %s\n", r.PPTX)
	fmt.Fprintf(&b, "Slides: %d  threshold ΔE=%.3f  chroma-min=%d  density=%d\n", r.SlideCount, r.MaxDeltaEAllowed, r.ChromaMin, r.Density)
	if r.Violations == 0 {
		fmt.Fprintln(&b, "Result: PASS")
	} else {
		fmt.Fprintf(&b, "Result: FAIL (%d violations)\n", r.Violations)
	}
	for _, s := range r.Slides {
		fmt.Fprintf(&b, "  slide %d: %d pic / %d shape / %d pair", s.Index, s.PicCount, s.ShapeCount, s.PairCount)
		if s.PairCount > 0 {
			fmt.Fprintf(&b, "  maxΔE=%.3f", s.MaxDeltaE)
		}
		fmt.Fprintln(&b)
		for _, p := range s.Pairs {
			mark := "✓"
			if !p.Pass {
				mark = "✗"
			}
			fmt.Fprintf(&b, "    %s ΔE=%.3f  pic[%s]=#%s  shape[%s]=#%s",
				mark, p.DeltaE,
				p.Pic.Name, p.Pic.Hex,
				p.Shape.Name, p.Shape.Hex,
			)
			if p.Shape.DeclaredHex != "" {
				fmt.Fprintf(&b, "  (declared #%s)", p.Shape.DeclaredHex)
			}
			fmt.Fprintln(&b)
		}
	}
	return b.String()
}

// --- PPTX inspection --------------------------------------------------------

// paletteShape captures the minimum information needed to find the pixel
// region for a single shape/picture in the rendered PNG.
type paletteShape struct {
	Name        string
	X, Y        int64 // EMU
	CX, CY      int64
	DeclaredHex string // best-effort: srgbClr val for solid-filled <p:sp>
}

// extractPaletteRegions walks the slide XML and returns:
//   - pics:   <p:pic> elements with explicit <p:spPr><a:xfrm/></p:spPr>
//   - shapes: <p:sp>  elements with explicit <p:spPr><a:xfrm/>...<a:solidFill/>
//
// Only shapes with their own xfrm are kept; placeholders without an inline
// xfrm inherit geometry from their layout/master and are skipped because the
// audit cares about author-placed colored shapes adjacent to charts.
func extractPaletteRegions(slideXML []byte) (pics []paletteShape, shapes []paletteShape, err error) {
	var sld auditSlideXML
	if err := xml.Unmarshal(slideXML, &sld); err != nil {
		return nil, nil, err
	}
	for _, p := range sld.CSld.SpTree.Pics {
		if p.SpPr.Xfrm == nil {
			continue
		}
		pics = append(pics, paletteShape{
			Name: p.NvPicPr.CNvPr.Name,
			X:    p.SpPr.Xfrm.Off.X, Y: p.SpPr.Xfrm.Off.Y,
			CX: p.SpPr.Xfrm.Ext.CX, CY: p.SpPr.Xfrm.Ext.CY,
		})
	}
	for _, s := range sld.CSld.SpTree.Shapes {
		if s.SpPr.Xfrm == nil || s.SpPr.SolidFill == nil {
			continue
		}
		shapes = append(shapes, paletteShape{
			Name: s.NvSpPr.CNvPr.Name,
			X:    s.SpPr.Xfrm.Off.X, Y: s.SpPr.Xfrm.Off.Y,
			CX: s.SpPr.Xfrm.Ext.CX, CY: s.SpPr.Xfrm.Ext.CY,
			DeclaredHex: declaredFillHex(s.SpPr.SolidFill),
		})
	}
	return pics, shapes, nil
}

func declaredFillHex(f *auditSolidFill) string {
	if f == nil {
		return ""
	}
	if f.SRGB != nil && f.SRGB.Val != "" {
		return strings.ToLower(strings.TrimPrefix(f.SRGB.Val, "#"))
	}
	// schemeClr indirections (e.g. "accent1") can't be resolved here without
	// loading the theme; surface the scheme name so the report has a clue.
	if f.Scheme != nil && f.Scheme.Val != "" {
		return strings.ToLower(f.Scheme.Val)
	}
	return ""
}

// readSlideDimensionsEMU pulls cx/cy from ppt/presentation.xml. Falls back to
// the standard 16:9 widescreen size if the file is missing or malformed.
func readSlideDimensionsEMU(pkg *pptx.Package) (int64, int64, error) {
	const (
		fallbackCX = int64(12192000) // 13.33"
		fallbackCY = int64(6858000)  // 7.5"
	)
	data, err := pkg.ReadEntry("ppt/presentation.xml")
	if err != nil {
		return fallbackCX, fallbackCY, nil //nolint:nilerr // fallback is intentional
	}
	var p struct {
		XMLName xml.Name `xml:"presentation"`
		SldSz   *struct {
			CX int64 `xml:"cx,attr"`
			CY int64 `xml:"cy,attr"`
		} `xml:"sldSz"`
	}
	if err := xml.Unmarshal(data, &p); err != nil {
		return fallbackCX, fallbackCY, nil //nolint:nilerr
	}
	if p.SldSz == nil || p.SldSz.CX == 0 || p.SldSz.CY == 0 {
		return fallbackCX, fallbackCY, nil
	}
	return p.SldSz.CX, p.SldSz.CY, nil
}

// auditSlideXML is a local minimal slide schema: we need <p:pic> (which the
// shared pptxread types intentionally omit) and the <p:sp> <a:solidFill>
// indirection (also intentionally omitted there because the public reader is
// text-focused).
type auditSlideXML struct {
	XMLName xml.Name        `xml:"sld"`
	CSld    auditCommonSld  `xml:"cSld"`
}

type auditCommonSld struct {
	SpTree auditSpTree `xml:"spTree"`
}

type auditSpTree struct {
	Shapes []auditSp  `xml:"sp"`
	Pics   []auditPic `xml:"pic"`
}

type auditSp struct {
	NvSpPr auditNvSpPr `xml:"nvSpPr"`
	SpPr   auditSpPr   `xml:"spPr"`
}

type auditNvSpPr struct {
	CNvPr auditCNvPr `xml:"cNvPr"`
}

type auditCNvPr struct {
	Name string `xml:"name,attr"`
}

type auditSpPr struct {
	Xfrm      *auditXfrm      `xml:"xfrm"`
	SolidFill *auditSolidFill `xml:"solidFill"`
}

type auditPic struct {
	NvPicPr auditNvPicPr `xml:"nvPicPr"`
	SpPr    auditSpPr    `xml:"spPr"`
}

type auditNvPicPr struct {
	CNvPr auditCNvPr `xml:"cNvPr"`
}

type auditXfrm struct {
	Off auditOff `xml:"off"`
	Ext auditExt `xml:"ext"`
}

type auditOff struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

type auditExt struct {
	CX int64 `xml:"cx,attr"`
	CY int64 `xml:"cy,attr"`
}

type auditSolidFill struct {
	SRGB   *auditSRGB   `xml:"srgbClr"`
	Scheme *auditScheme `xml:"schemeClr"`
}

type auditSRGB struct {
	Val string `xml:"val,attr"`
}

type auditScheme struct {
	Val string `xml:"val,attr"`
}

// --- Rendering --------------------------------------------------------------

// renderPPTXToPNGs invokes libreoffice → pdf and pdftoppm → png. Returns the
// sorted list of PNG paths (one per slide).
func renderPPTXToPNGs(pptxPath, workDir string, density int) ([]string, error) {
	pdfDir := filepath.Join(workDir, "pdf")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		return nil, err
	}
	conv := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", pdfDir, pptxPath) //nolint:gosec // controlled args
	if out, err := conv.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("libreoffice: %v\n%s", err, out)
	}
	pdfs, _ := filepath.Glob(filepath.Join(pdfDir, "*.pdf"))
	if len(pdfs) == 0 {
		return nil, fmt.Errorf("libreoffice produced no PDF in %s", pdfDir)
	}
	pdfPath := pdfs[0]

	pngPrefix := filepath.Join(workDir, "slide")
	rast := exec.Command("pdftoppm", "-png", "-r", fmt.Sprintf("%d", density), pdfPath, pngPrefix) //nolint:gosec // controlled args
	if out, err := rast.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdftoppm: %v\n%s", err, out)
	}
	pngs, err := filepath.Glob(pngPrefix + "*.png")
	if err != nil {
		return nil, err
	}
	sort.Strings(pngs)
	return pngs, nil
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path) //nolint:gosec // controlled path under tmp dir
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// --- Sampling + color math --------------------------------------------------

func samplePaletteRegions(img image.Image, regions []paletteShape, slideCX, slideCY int64, imgW, imgH int, chromaMin uint8, kind string) []auditRegion {
	out := make([]auditRegion, 0, len(regions))
	for _, r := range regions {
		x0, y0, x1, y1 := emuRectToPx(r.X, r.Y, r.CX, r.CY, slideCX, slideCY, imgW, imgH)
		// Shrink ~5% on each side to avoid borders / chrome.
		x0, y0, x1, y1 = shrinkRect(x0, y0, x1, y1, 0.05)
		rr, gg, bb, n := dominantColorPx(img, x0, y0, x1, y1, chromaMin)
		hex := ""
		if n > 0 {
			hex = fmt.Sprintf("%02x%02x%02x", rr, gg, bb)
		}
		out = append(out, auditRegion{
			Kind:        kind,
			Name:        r.Name,
			BoundsEMU:   [4]int64{r.X, r.Y, r.CX, r.CY},
			BoundsPx:    [4]int{x0, y0, x1, y1},
			Hex:         hex,
			R:           rr,
			G:           gg,
			B:           bb,
			PixelCount:  n,
			DeclaredHex: r.DeclaredHex,
		})
	}
	return out
}

func emuRectToPx(x, y, cx, cy, slideCX, slideCY int64, imgW, imgH int) (int, int, int, int) {
	if slideCX <= 0 || slideCY <= 0 {
		return 0, 0, 0, 0
	}
	x0 := int(float64(x) / float64(slideCX) * float64(imgW))
	y0 := int(float64(y) / float64(slideCY) * float64(imgH))
	x1 := int(float64(x+cx) / float64(slideCX) * float64(imgW))
	y1 := int(float64(y+cy) / float64(slideCY) * float64(imgH))
	return x0, y0, x1, y1
}

func shrinkRect(x0, y0, x1, y1 int, frac float64) (int, int, int, int) {
	w := x1 - x0
	h := y1 - y0
	dx := int(float64(w) * frac)
	dy := int(float64(h) * frac)
	if x1-x0-2*dx <= 0 || y1-y0-2*dy <= 0 {
		return x0, y0, x1, y1
	}
	return x0 + dx, y0 + dy, x1 - dx, y1 - dy
}

// dominantColorPx returns the chroma-weighted average sRGB color across the
// rectangle. Pixels whose (max-min) channel difference is below chromaMin are
// skipped, which filters out white/grey/black chrome (axis lines, labels,
// gridlines) while keeping accent-colored fills and chart bars.
func dominantColorPx(img image.Image, x0, y0, x1, y1 int, chromaMin uint8) (uint8, uint8, uint8, int) {
	b := img.Bounds()
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	var sumR, sumG, sumB uint64
	var count int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := img.At(x, y)
			ri, gi, bi, _ := c.RGBA()
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
	return uint8(sumR / uint64(count)), uint8(sumG / uint64(count)), uint8(sumB / uint64(count)), count
}

// deltaE76Hex is a thin wrapper that does the sRGB→Lab conversion and returns
// the CIE76 ΔE between two 8-bit sRGB colors.
func deltaE76Hex(r1, g1, b1, r2, g2, b2 uint8) float64 {
	l1, a1, bb1 := rgbToLab(uint32(r1), uint32(g1), uint32(b1))
	l2, a2, bb2 := rgbToLab(uint32(r2), uint32(g2), uint32(b2))
	dl := l1 - l2
	da := a1 - a2
	db := bb1 - bb2
	return math.Sqrt(dl*dl + da*da + db*db)
}

func srgbToLinear(v uint32) float64 {
	c := float64(v) / 255.0
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func rgbToLab(r, g, b uint32) (L, a, bb float64) {
	rl := srgbToLinear(r)
	gl := srgbToLinear(g)
	bl := srgbToLinear(b)
	x := rl*0.4124564 + gl*0.3575761 + bl*0.1804375
	y := rl*0.2126729 + gl*0.7151522 + bl*0.0721750
	z := rl*0.0193339 + gl*0.1191920 + bl*0.9503041
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
