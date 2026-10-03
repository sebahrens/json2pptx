// Package layoutpreview generates PNG preview images for template slide layouts.
// It creates a minimal single-slide PPTX per layout using the generator, converts
// to PNG via LibreOffice + ImageMagick, and caches the results on disk.
package layoutpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Result holds the preview PNG paths keyed by layout ID.
type Result struct {
	Paths         map[string]string // layout ID -> absolute path to PNG file
	CacheIdentity string            // template, recipe, runtime and density identity
}

// Options configures preview generation.
type Options struct {
	// CacheDir is the base directory for cached previews.
	// Defaults to ~/.cache/json2pptx/layout-previews if empty.
	CacheDir string
	// DPI controls the rendering density (default: 96).
	DPI int
	// LibreOfficeProfileDir, when set, runs LibreOffice with a private user
	// profile (-env:UserInstallation) so concurrent renders do not collide on
	// the shared default profile. Empty = LibreOffice's default profile.
	LibreOfficeProfileDir string
}

func (o *Options) loProfileArgs() []string {
	if o == nil || o.LibreOfficeProfileDir == "" {
		return nil
	}
	abs, err := filepath.Abs(o.LibreOfficeProfileDir)
	if err != nil {
		abs = o.LibreOfficeProfileDir
	}
	return []string{"-env:UserInstallation=file://" + filepath.ToSlash(abs)}
}

func (o *Options) cacheDir() string {
	if o != nil && o.CacheDir != "" {
		return o.CacheDir
	}
	return DefaultCacheDir()
}

// DefaultCacheDir returns the base directory layout-preview PNGs are cached
// under when no explicit Options.CacheDir is set
// (~/.cache/json2pptx/layout-previews, falling back to a temp dir when the
// user home directory cannot be resolved). It is exported so discovery
// surfaces (skill-info / list_templates) can report the preview cache location
// to agents alongside the read-only opt-out.
func DefaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "json2pptx-layout-previews")
	}
	return filepath.Join(home, ".cache", "json2pptx", "layout-previews")
}

func (o *Options) dpi() int {
	if o != nil && o.DPI > 0 {
		return o.DPI
	}
	return 96
}

// Deadlines for the external preview subprocesses and for a whole cold
// preview build. They are vars so tests can shrink them to force a timeout
// with a fake hung binary (go-slide-creator-b7qqg.6). The LibreOffice step
// converts one slide per layout in a single run, so it gets more headroom than
// a render-tool conversion; each ImageMagick call rasterizes a single page.
var (
	previewLibreOfficeTimeout = 180 * time.Second
	previewImageMagickTimeout = 60 * time.Second
	previewTotalTimeout       = 5 * time.Minute
)

// previewTimeoutError rewraps a renderer *render.TimeoutError with guidance
// for the discovery surfaces (list_templates, examine_template, skill-info)
// rather than the render tools' force=true retry. errors.As still reaches the
// underlying *render.TimeoutError (tool, code, elapsed).
func previewTimeoutError(step string, err error) error {
	var te *render.TimeoutError
	if !errors.As(err, &te) {
		return err
	}
	return &previewTimeout{
		msg: fmt.Sprintf("layout preview %s: %s. Previews are optional: retry discovery, or pass "+
			"read_only=true / --no-preview to skip them; if it recurs the renderer is likely wedged — "+
			"restart LibreOffice/ImageMagick", step, te.Summary()),
		err: te,
	}
}

// previewTimeout carries discovery guidance in its message while unwrapping to
// the structured *render.TimeoutError, whose own message cites the render
// tools' force=true retry that discovery does not take.
type previewTimeout struct {
	msg string
	err error
}

func (e *previewTimeout) Error() string { return e.msg }
func (e *previewTimeout) Unwrap() error { return e.err }

// Generate produces PNG preview images for each layout in the template.
// Returns nil if LibreOffice or ImageMagick is not available (graceful degradation).
// It runs without a request context; callers on a request path should use
// GenerateContext so cancellation reaches the renderer subprocesses.
func Generate(templatePath string, analysis *types.TemplateAnalysis, opts *Options) (*Result, error) {
	return GenerateContext(context.Background(), templatePath, analysis, opts)
}

// GenerateContext is Generate bound to ctx: cancelling ctx (or exceeding the
// per-step / total preview deadlines) kills the LibreOffice or ImageMagick
// process group it started, removes its own temp files and partially written
// PNGs, and returns an error. Deadline overruns wrap a *render.TimeoutError.
func GenerateContext(ctx context.Context, templatePath string, analysis *types.TemplateAnalysis, opts *Options) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Check tool availability — graceful degradation when missing
	if !hasLibreOffice() || !hasImageMagick() {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cacheDir := opts.cacheDir()
	templateName := strings.TrimSuffix(filepath.Base(templatePath), ".pptx")

	// Reuse only previews from the same template, generator, recipe, renderer
	// and effective resolution. Legacy template-only caches are left untouched.
	hash, err := currentPreviewCacheIdentityContext(ctx, templatePath, analysis, opts)
	if err != nil {
		return nil, fmt.Errorf("identify preview cache: %w", err)
	}

	previewDir := filepath.Join(cacheDir, templateName, hash)

	// Check if cache is valid (marker file exists and PNGs present)
	markerPath := filepath.Join(previewDir, ".done")
	if _, err := os.Stat(markerPath); err == nil {
		if result, _ := collectCachedPreviews(previewDir, analysis); result != nil {
			result.CacheIdentity = hash
			// A hit counts as use: the sweep ages sets by their marker.
			now := time.Now()
			_ = os.Chtimes(markerPath, now, now)
			return result, nil
		}
		// Stale marker with no PNGs — regenerate
		_ = os.Remove(markerPath)
	}

	// Generate previews
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		return nil, fmt.Errorf("create preview dir: %w", err)
	}

	// Generate a single PPTX with all layouts (one slide per layout)
	// then split the resulting PDF pages into individual PNGs.
	buildCtx, cancel := context.WithTimeout(ctx, previewTotalTimeout)
	defer cancel()
	if err := generateAllPreviews(buildCtx, templatePath, analysis, previewDir, opts.dpi(), opts.loProfileArgs()); err != nil {
		return nil, err
	}

	result, err := collectCachedPreviews(previewDir, analysis)
	if err != nil {
		return nil, fmt.Errorf("incomplete layout previews: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("no layout previews generated")
	}
	result.CacheIdentity = hash
	// Only a complete, readable set may be marked reusable.
	if err := os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644); err != nil {
		return nil, fmt.Errorf("mark layout previews complete: %w", err)
	}
	// A new set was just added, so this is the moment the cache can have
	// outgrown its bound; hits above never pay for the walk.
	_, _ = SweepCache(cacheDir, CacheMaxAge, CacheMaxBytes, previewDir)
	return result, nil
}

// generateAllPreviews creates a PPTX with one slide per layout, converts to PDF,
// then splits into per-layout PNG files. Every external step is bounded by ctx
// and its own deadline through internal/render's owned-process runner, and on
// failure the PNGs this call wrote are removed so no partial set lingers in the
// cache directory (other files there are left alone).
func generateAllPreviews(ctx context.Context, templatePath string, analysis *types.TemplateAnalysis, previewDir string, dpi int, loArgs []string) (err error) {
	tmpDir, err := os.MkdirTemp("", "layoutpreview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	var written []string
	defer func() {
		if err != nil {
			for _, p := range written {
				_ = os.Remove(p)
			}
		}
	}()

	// Build slide specs — one slide per layout with sample content so
	// placeholder positions are visible in the rendered PNG.
	slides := make([]generator.SlideSpec, len(analysis.Layouts))
	for i, layout := range analysis.Layouts {
		slides[i] = generator.SlideSpec{
			LayoutID: layout.ID,
			Content:  SampleContent(layout),
		}
	}

	// Generate a PPTX using the real generator
	pptxPath := filepath.Join(tmpDir, "layouts.pptx")
	_, err = generator.Generate(ctx, generator.GenerationRequest{
		TemplatePath:          templatePath,
		OutputPath:            pptxPath,
		Slides:                slides,
		ExcludeTemplateSlides: true,
	})
	if err != nil {
		return fmt.Errorf("generate preview pptx: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("layout preview cancelled: %w", err)
	}

	// Convert to PDF via LibreOffice
	args := append(append([]string(nil), loArgs...), "--headless", "--convert-to", "pdf", "--outdir", tmpDir, pptxPath)
	if stderr, err := render.RunLibreOfficeBounded(ctx, pptxPath, previewLibreOfficeTimeout, libreOfficeBin(), args...); err != nil {
		return previewStepError(ctx, "libreoffice convert", err, stderr)
	}

	pdfPath := filepath.Join(tmpDir, "layouts.pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return fmt.Errorf("pdf not created by libreoffice")
	}

	// Split PDF pages into individual PNGs using ImageMagick
	magickBin := imageMagickBin()
	for i, layout := range analysis.Layouts {
		pngPath := filepath.Join(previewDir, layout.ID+".png")
		pageSpec := fmt.Sprintf("%s[%d]", pdfPath, i)
		written = append(written, pngPath)
		stderr, err := render.RunImageMagickBounded(ctx, pageSpec, previewImageMagickTimeout, magickBin,
			"-density", fmt.Sprintf("%d", dpi), pageSpec, "-quality", "90", pngPath)
		if err != nil {
			return previewStepError(ctx, fmt.Sprintf("rasterize layout %s (page %d)", layout.ID, i+1), err, stderr)
		}
	}

	return nil
}

// previewStepError turns a failed bounded subprocess into an actionable error:
// a renderer deadline keeps its *render.TimeoutError (with discovery-specific
// guidance), an exhausted total preview budget or caller cancellation keeps the
// context error, and other failures carry the captured stderr.
func previewStepError(ctx context.Context, step string, err error, stderr string) error {
	var te *render.TimeoutError
	if errors.As(err, &te) {
		return previewTimeoutError(step, err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return fmt.Errorf("layout preview %s: preview build exceeded its %s budget. Previews are optional: "+
				"retry discovery, or pass read_only=true / --no-preview to skip them: %w", step, previewTotalTimeout, ctxErr)
		}
		return fmt.Errorf("layout preview %s cancelled: %w", step, ctxErr)
	}
	if s := strings.TrimSpace(stderr); s != "" {
		return fmt.Errorf("%s: %w: %s", step, err, s)
	}
	return fmt.Errorf("%s: %w", step, err)
}

func collectCachedPreviews(previewDir string, analysis *types.TemplateAnalysis) (*Result, error) {
	result := &Result{Paths: make(map[string]string)}
	for _, layout := range analysis.Layouts {
		pngPath := filepath.Join(previewDir, layout.ID+".png")
		f, err := os.Open(pngPath) // #nosec G304 -- expected layout ID in configured preview cache
		if err != nil {
			return nil, fmt.Errorf("layout %s preview missing: %w", layout.ID, err)
		}
		_, err = png.Decode(f)
		_ = f.Close()
		if err != nil {
			return nil, fmt.Errorf("layout %s preview unreadable: %w", layout.ID, err)
		}
		result.Paths[layout.ID] = pngPath
	}
	if len(result.Paths) == 0 {
		return nil, nil
	}
	return result, nil
}

func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// The render binaries are resolved by internal/render, the single place that
// knows libreoffice/soffice and magick/convert are the same tool under
// different names (go-slide-creator-rdql).
func hasLibreOffice() bool {
	_, err := render.OfficeCommand()
	return err == nil
}

func libreOfficeBin() string {
	bin, err := render.OfficeCommand()
	if err != nil {
		return "soffice"
	}
	return bin
}

func hasImageMagick() bool {
	_, err := render.ImageMagickCommand()
	return err == nil
}

func imageMagickBin() string {
	path, _ := render.ImageMagickCommand()
	return path
}

// SampleContent builds placeholder content items for a layout so the preview
// renders visible text in each placeholder region rather than a blank slide.
func SampleContent(layout types.LayoutMetadata) []generator.ContentItem {
	var items []generator.ContentItem
	isSectionDivider := layout.CanonicalType == types.CanonicalLayoutSectionDivider
	for _, ph := range layout.Placeholders {
		if types.IsDisclosurePlaceholder(ph) {
			items = append(items, generator.ContentItem{PlaceholderID: ph.ID, Type: generator.ContentText, Value: "Illustrative disclosure for review"})
			continue
		}
		if ph.Role == types.PlaceholderRoleSectionNumber || types.IsAutoFilledPlaceholder(ph.ID) {
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentText,
				Value:         "01",
			})
			continue
		}
		switch ph.Type {
		case types.PlaceholderTitle:
			value := layout.Name
			if isSectionDivider {
				value = "Section"
			}
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentText,
				Value:         value,
			})
		case types.PlaceholderSubtitle:
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentText,
				Value:         "Subtitle placeholder",
			})
		case types.PlaceholderBody, types.PlaceholderContent:
			if isSectionDivider {
				items = append(items, generator.ContentItem{
					PlaceholderID: ph.ID,
					Type:          generator.ContentText,
					Value:         "Overview",
				})
				continue
			}
			items = append(items, generator.ContentItem{
				PlaceholderID: ph.ID,
				Type:          generator.ContentBullets,
				Value:         []string{"First bullet point", "Second bullet point", "Third bullet point"},
			})
			// Skip PlaceholderOther (date, footer, slide number) — not useful for previews.
		}
	}
	return items
}
