// Package templatepreview ships and resolves per-layout template preview
// thumbnails (go-slide-creator-aruv).
//
// Thumbnails are small PNGs (default 320px wide) rendered once per bundled
// template layout by `make template-previews` and committed under
// templates/previews/<template>/<layoutID>.png. Discovery surfaces
// (recommend_visual) point agents at them so a recommendation carries a real
// picture of the layout it will render on instead of metadata only.
package templatepreview

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/image/draw"

	"github.com/sebahrens/json2pptx/internal/layoutpreview"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/templates"
)

// DefaultWidth is the committed thumbnail width in pixels.
const DefaultWidth = 320

// DirName is the previews directory next to the template files.
const DirName = "previews"

// RelPath is the preview's path relative to a templates directory.
func RelPath(templateName, layoutID string) string {
	return filepath.Join(DirName, templateName, layoutID+".png")
}

// Resolve returns an absolute path to the preview PNG of layoutID in the
// template at templatePath, or "" when none ships. It looks next to the
// template file first (templates/previews/<name>/<layout>.png); for embedded
// templates (resolved to a temp file) it materialises the embedded preview
// into the user cache directory.
func Resolve(templatePath, layoutID string) string {
	if templatePath == "" || layoutID == "" {
		return ""
	}
	name := strings.TrimSuffix(filepath.Base(templatePath), ".pptx")
	onDisk := filepath.Join(filepath.Dir(templatePath), RelPath(name, layoutID))
	if info, err := os.Stat(onDisk); err == nil && !info.IsDir() {
		if abs, err := filepath.Abs(onDisk); err == nil {
			return abs
		}
		return onDisk
	}
	return extractEmbedded(name, layoutID)
}

// extractEmbedded writes the embedded preview for (name, layoutID) into the
// user cache dir (once) and returns its path, or "" when none is embedded.
func extractEmbedded(name, layoutID string) string {
	data, err := templates.Embedded.ReadFile(filepath.ToSlash(RelPath(name, layoutID)))
	if err != nil {
		return ""
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	dst := filepath.Join(base, "json2pptx", "template-previews", name, layoutID+".png")
	if existing, err := os.ReadFile(dst); err == nil && bytes.Equal(existing, data) { //nolint:gosec // cache path built from fixed parts
		return dst
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return ""
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil { //nolint:gosec // cache file
		return ""
	}
	return dst
}

// Options configures GenerateAll.
type Options struct {
	Width                 int    // thumbnail width in px (default DefaultWidth)
	DPI                   int    // render density before downscaling (default 60)
	LibreOfficeProfileDir string // private LibreOffice profile (avoids profile collisions)
	CacheDir              string // layoutpreview cache dir (default: temp dir)
}

// GenerateAll renders a thumbnail for every layout of every *.pptx template in
// templatesDir into outDir/<template>/<layoutID>.png, replacing stale files.
// It returns the number of thumbnails written per template. Requires
// LibreOffice + ImageMagick (layoutpreview); returns an error when they are
// missing so the make target fails loudly instead of committing nothing.
func GenerateAll(templatesDir, outDir string, opts Options) (map[string]int, error) {
	if opts.Width <= 0 {
		opts.Width = DefaultWidth
	}
	if opts.DPI <= 0 {
		opts.DPI = 60
	}
	cacheDir := opts.CacheDir
	if cacheDir == "" {
		tmp, err := os.MkdirTemp("", "template-previews-*")
		if err != nil {
			return nil, err
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cacheDir = tmp
	}
	paths, err := filepath.Glob(filepath.Join(templatesDir, "*.pptx"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	counts := make(map[string]int, len(paths))
	for _, tplPath := range paths {
		name := strings.TrimSuffix(filepath.Base(tplPath), ".pptx")
		n, err := generateTemplate(tplPath, filepath.Join(outDir, name), cacheDir, opts)
		if err != nil {
			return counts, fmt.Errorf("%s: %w", name, err)
		}
		counts[name] = n
	}
	return counts, nil
}

func generateTemplate(tplPath, outDir, cacheDir string, opts Options) (int, error) {
	reader, err := template.OpenTemplate(tplPath)
	if err != nil {
		return 0, err
	}
	layouts, err := template.ParseLayouts(reader)
	theme := template.ParseTheme(reader)
	hash := reader.Hash()
	_ = reader.Close()
	if err != nil {
		return 0, err
	}
	analysis := &types.TemplateAnalysis{TemplatePath: tplPath, Hash: hash, Layouts: layouts, Theme: theme}
	res, err := layoutpreview.Generate(tplPath, analysis, &layoutpreview.Options{
		CacheDir: cacheDir, DPI: opts.DPI, LibreOfficeProfileDir: opts.LibreOfficeProfileDir,
	})
	if err != nil {
		return 0, err
	}
	if res == nil {
		return 0, fmt.Errorf("layout previews unavailable (LibreOffice + ImageMagick required)")
	}
	if err := os.RemoveAll(outDir); err != nil {
		return 0, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, err
	}
	n := 0
	for _, l := range layouts {
		src, ok := res.Paths[l.ID]
		if !ok {
			continue
		}
		if err := downscalePNG(src, filepath.Join(outDir, l.ID+".png"), opts.Width); err != nil {
			return n, fmt.Errorf("%s: %w", l.ID, err)
		}
		n++
	}
	return n, nil
}

// downscalePNG writes src resized to width px (aspect preserved) as dst.
func downscalePNG(src, dst string, width int) error {
	f, err := os.Open(src) //nolint:gosec // path from layoutpreview output
	if err != nil {
		return err
	}
	img, _, err := image.Decode(f)
	_ = f.Close()
	if err != nil {
		return err
	}
	b := img.Bounds()
	if b.Dx() <= 0 {
		return fmt.Errorf("empty image")
	}
	height := b.Dy() * width / b.Dx()
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Over, nil)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, out); err != nil {
		return err
	}
	return os.WriteFile(dst, buf.Bytes(), 0o644) //nolint:gosec // committed preview asset
}
