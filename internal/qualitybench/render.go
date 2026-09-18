package qualitybench

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// lookPath is swappable in tests.
var lookPath = exec.LookPath

// officeCandidates mirrors cmd/pptx2jpg: Homebrew casks and the macOS app
// bundle ship only `soffice`.
var officeCandidates = []string{"libreoffice", "soffice", "/Applications/LibreOffice.app/Contents/MacOS/soffice"}

// Renderer turns a deck JSON into a PPTX (via the json2pptx binary), rasterizes
// it (LibreOffice -> PDF -> pdftoppm or ImageMagick) and builds a contact sheet.
type Renderer struct {
	JSON2PPTX       string // path to the json2pptx binary
	TemplatesDir    string
	Density         int // raster DPI (default 60 — thumbnails only)
	office          string
	raster          string
	rasterIsPoppler bool
}

// Resolve locates LibreOffice and a rasterizer. It must be called before
// Rasterize.
func (r *Renderer) Resolve() error {
	for _, c := range officeCandidates {
		if p, err := lookPath(c); err == nil {
			r.office = p
			break
		}
	}
	if r.office == "" {
		return fmt.Errorf("LibreOffice not found (tried %s)", strings.Join(officeCandidates, ", "))
	}
	if p, err := lookPath("pdftoppm"); err == nil {
		r.raster, r.rasterIsPoppler = p, true
		return nil
	}
	if p, err := lookPath("magick"); err == nil {
		r.raster = p
		return nil
	}
	return fmt.Errorf("no PDF rasterizer found: install poppler (pdftoppm) or ImageMagick")
}

// Generate writes deckJSON next to outPPTX and runs json2pptx generate.
func (r *Renderer) Generate(ctx context.Context, deckJSON []byte, outPPTX string) error {
	jsonPath := strings.TrimSuffix(outPPTX, filepath.Ext(outPPTX)) + ".json"
	if err := os.WriteFile(jsonPath, deckJSON, 0o600); err != nil {
		return err
	}
	args := []string{"generate", "-json", jsonPath, "-output", outPPTX, "-output-validation", "warn"}
	if r.TemplatesDir != "" {
		args = append(args, "-templates-dir", r.TemplatesDir)
	}
	// #nosec G204 -- the json2pptx binary path is operator-configured.
	out, err := exec.CommandContext(ctx, r.JSON2PPTX, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("json2pptx generate: %w: %s", err, lastLines(string(out), 5))
	}
	return nil
}

// Rasterize converts pptx to one PNG per slide in outDir and returns them in
// slide order.
func (r *Renderer) Rasterize(ctx context.Context, pptx, outDir string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	profile, err := os.MkdirTemp("", "qualitybench-lo-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(profile) }()
	// #nosec G204 -- resolved LibreOffice binary with fixed arguments.
	if out, err := exec.CommandContext(ctx, r.office, "-env:UserInstallation=file://"+filepath.ToSlash(profile),
		"--headless", "--convert-to", "pdf", "--outdir", outDir, pptx).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("LibreOffice: %w: %s", err, lastLines(string(out), 3))
	}
	base := strings.TrimSuffix(filepath.Base(pptx), filepath.Ext(pptx))
	pdf := filepath.Join(outDir, base+".pdf")
	if _, err := os.Stat(pdf); err != nil {
		return nil, fmt.Errorf("LibreOffice produced no PDF for %s", pptx)
	}
	density := r.Density
	if density <= 0 {
		density = 60
	}
	prefix := filepath.Join(outDir, base+"-slide")
	var cmd *exec.Cmd
	if r.rasterIsPoppler {
		// #nosec G204 -- resolved rasterizer with fixed arguments.
		cmd = exec.CommandContext(ctx, r.raster, "-r", strconv.Itoa(density), "-png", pdf, prefix)
	} else {
		// #nosec G204 -- resolved rasterizer with fixed arguments.
		cmd = exec.CommandContext(ctx, r.raster, "-density", strconv.Itoa(density), pdf, prefix+"-%d.png")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("rasterize: %w: %s", err, lastLines(string(out), 3))
	}
	pngs, _ := filepath.Glob(prefix + "-*.png")
	sort.Slice(pngs, func(i, j int) bool { return slideNum(pngs[i]) < slideNum(pngs[j]) })
	if len(pngs) == 0 {
		return nil, fmt.Errorf("rasterizer produced no images for %s", pptx)
	}
	return pngs, nil
}

func slideNum(p string) int {
	b := strings.TrimSuffix(filepath.Base(p), ".png")
	n, _ := strconv.Atoi(b[strings.LastIndex(b, "-")+1:])
	return n
}

// ContactSheet tiles slide images (in order) into a single PNG with cols
// columns of thumbW-wide thumbnails. It carries no run/template labels so it
// can be rated blind.
func ContactSheet(images []string, out string, cols, thumbW int) error {
	if len(images) == 0 {
		return fmt.Errorf("contact sheet needs at least one image")
	}
	if cols <= 0 {
		cols = 3
	}
	if thumbW <= 0 {
		thumbW = 480
	}
	var thumbs []image.Image
	thumbH := 0
	for _, p := range images {
		f, err := os.Open(p) // #nosec G304 -- paths produced by Rasterize
		if err != nil {
			return err
		}
		img, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("decode %s: %w", p, err)
		}
		b := img.Bounds()
		h := b.Dy() * thumbW / maxInt(b.Dx(), 1)
		dst := image.NewRGBA(image.Rect(0, 0, thumbW, h))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
		thumbs = append(thumbs, dst)
		thumbH = maxInt(thumbH, h)
	}
	const gutter = 12
	rows := (len(thumbs) + cols - 1) / cols
	c := minInt(cols, len(thumbs))
	sheet := image.NewRGBA(image.Rect(0, 0, c*thumbW+(c+1)*gutter, rows*thumbH+(rows+1)*gutter))
	xdraw.Draw(sheet, sheet.Bounds(), &image.Uniform{C: color.RGBA{0x88, 0x88, 0x88, 0xff}}, image.Point{}, xdraw.Src)
	for i, t := range thumbs {
		x := gutter + (i%cols)*(thumbW+gutter)
		y := gutter + (i/cols)*(thumbH+gutter)
		xdraw.Draw(sheet, t.Bounds().Add(image.Pt(x, y)), t, image.Point{}, xdraw.Src)
	}
	f, err := os.Create(out) // #nosec G304 -- operator-chosen output path
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return png.Encode(f, sheet)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
