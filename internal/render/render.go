// Package render converts PPTX files to PNG images using LibreOffice and ImageMagick.
package render

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// pngIndexFromName extracts the integer N from a filename matching "slide-N.png".
// Returns -1 if the name doesn't conform, which sorts those entries first.
func pngIndexFromName(path string) int {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".png")
	base = strings.TrimPrefix(base, "slide-")
	n, err := strconv.Atoi(base)
	if err != nil {
		return -1
	}
	return n
}

// sortPNGsByIndex sorts files matching "slide-N.png" by the numeric N.
// This avoids lexicographic ordering placing slide-10.png before slide-2.png,
// which would map slide_index 2 to the 11th rendered page for decks >9 slides.
func sortPNGsByIndex(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return pngIndexFromName(files[i]) < pngIndexFromName(files[j])
	})
}

// maxInlineBytes is the base64-decoded size cap per slide image.
// If a rendered PNG exceeds this, the tool returns a path reference instead.
const maxInlineBytes = 200 * 1024 // 200 KB

// ArtifactCleanupPolicy documents the lifetime of on-disk render artifacts
// returned via SlideImage.Path. Artifacts are content-addressed: the filename
// embeds the SHA-256 of the PNG, so a given path always holds the same bytes and
// is never overwritten with different content. They live under the render cache
// directory and are removed by InvalidateCache or by OS temp cleanup.
const ArtifactCleanupPolicy = "content-addressed; path is stable while the file exists and is never overwritten with different content; removed by render-cache invalidation (InvalidateCache) or OS temp cleanup"

// mu serializes LibreOffice invocations (single-threaded per process).
var mu sync.Mutex

// cacheDir returns the directory used for rendered slide caches.
func cacheDir() string {
	return filepath.Join(os.TempDir(), "json2pptx-render-cache")
}

// hashFile computes the SHA-256 hash of a file's contents.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// HashFile is the exported form of hashFile, used by callers that need to
// build composite cache keys (e.g. JSON-input hash + template hash).
func HashFile(path string) (string, error) {
	return hashFile(path)
}

// cacheKey returns a unique directory name for a given file hash and density.
func cacheKey(hash string, density int) string {
	return fmt.Sprintf("%s-d%d", hash, density)
}

// getCachedPNGs returns cached PNG paths if they exist for the given key.
// Returns nil if cache miss.
func getCachedPNGs(key string) []string {
	dir := filepath.Join(cacheDir(), key)
	files, err := filepath.Glob(filepath.Join(dir, "slide-*.png"))
	if err != nil || len(files) == 0 {
		return nil
	}
	sortPNGsByIndex(files)
	return files
}

// storeCachePNGs copies rendered PNGs into the cache directory for future reuse.
func storeCachePNGs(key string, pngs []string) {
	dir := filepath.Join(cacheDir(), key)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return // best-effort caching
	}
	for i, src := range pngs {
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		dst := filepath.Join(dir, fmt.Sprintf("slide-%d.png", i))
		_ = os.WriteFile(dst, data, 0644)
	}
}

// SlideImage holds the rendered output for a single slide.
type SlideImage struct {
	Index   int    `json:"index"`
	PNG64   string `json:"png_base64,omitempty"`
	Path    string `json:"path,omitempty"`
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	SizeErr string `json:"size_error,omitempty"`

	// ContentHash is the SHA-256 (hex) of the rendered PNG bytes. It is the
	// stable identity of this image regardless of delivery (inline or path), and
	// is what makes Path collision-free: two renders share a Path only when their
	// ContentHash is identical.
	ContentHash string `json:"content_hash,omitempty"`
	// SourceHash identifies the upstream artifact this image was rendered from —
	// the PPTX file content hash for deck/slide renders, or the caller-supplied
	// cache key (e.g. slide-JSON + template hash) for keyed renders.
	SourceHash string `json:"source_hash,omitempty"`
	// Cleanup describes the lifetime/cleanup semantics of an on-disk Path
	// artifact. Empty when the image is delivered inline as PNG64.
	Cleanup string `json:"cleanup,omitempty"`
}

// DeckResult holds the result of rendering an entire deck.
type DeckResult struct {
	Slides    []SlideImage `json:"slides"`
	Truncated bool         `json:"truncated"`
}

// checkDep verifies that a command-line tool is available on PATH.
func checkDep(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: install it to use render tools", name)
	}
	return nil
}

// CheckDependencies verifies LibreOffice and ImageMagick are available.
func CheckDependencies() error {
	if err := checkDep("libreoffice"); err != nil {
		return err
	}
	return checkDep("magick")
}

// DependencyStatus checks each render dependency and returns whether rendering
// is available and which commands are missing.
func DependencyStatus() (available bool, missing []string) {
	for _, cmd := range []string{"libreoffice", "magick"} {
		if checkDep(cmd) != nil {
			missing = append(missing, cmd)
		}
	}
	return len(missing) == 0, missing
}

// pptxToPDF converts a PPTX to PDF via LibreOffice headless.
// Returns the path to the generated PDF. The conversion is bounded by
// libreOfficeTimeout: the LibreOffice mutex is held only for the duration of
// that deadline, never indefinitely, and a timeout returns a *TimeoutError.
func pptxToPDF(ctx context.Context, pptxPath, tmpDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	_, stderr, err := runBounded(ctx, toolLibreOffice, pptxPath, libreOfficeTimeout,
		toolLibreOffice,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", tmpDir,
		pptxPath,
	)
	if err != nil {
		var te *TimeoutError
		if errors.As(err, &te) {
			return "", err // structured timeout; propagate verbatim
		}
		if stderr = strings.TrimSpace(stderr); stderr != "" {
			return "", fmt.Errorf("libreoffice conversion failed: %w: %s", err, stderr)
		}
		return "", fmt.Errorf("libreoffice conversion failed: %w", err)
	}

	base := strings.TrimSuffix(filepath.Base(pptxPath), filepath.Ext(pptxPath))
	pdfPath := filepath.Join(tmpDir, base+".pdf")
	if _, err := os.Stat(pdfPath); err != nil {
		return "", fmt.Errorf("PDF not created at %s", pdfPath)
	}
	return pdfPath, nil
}

// pdfToPNGs converts a multi-page PDF to individual PNG files using ImageMagick.
// Returns sorted list of generated PNG paths. The conversion is bounded by
// imageMagickTimeout; a timeout returns a *TimeoutError.
func pdfToPNGs(ctx context.Context, pdfPath, outDir string, density int) ([]string, error) {
	pattern := filepath.Join(outDir, "slide-%d.png")
	_, stderr, err := runBounded(ctx, toolImageMagick, pdfPath, imageMagickTimeout,
		toolImageMagick,
		"-density", fmt.Sprintf("%d", density),
		pdfPath,
		"-quality", "95",
		pattern,
	)
	if err != nil {
		var te *TimeoutError
		if errors.As(err, &te) {
			return nil, err // structured timeout; propagate verbatim
		}
		if stderr = strings.TrimSpace(stderr); stderr != "" {
			return nil, fmt.Errorf("imagemagick conversion failed: %w: %s", err, stderr)
		}
		return nil, fmt.Errorf("imagemagick conversion failed: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(outDir, "slide-*.png"))
	if err != nil {
		return nil, err
	}
	sortPNGsByIndex(files)
	return files, nil
}

// readAsBase64 reads a file and returns its base64-encoded content.
func readAsBase64(path string) (string, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	return base64.StdEncoding.EncodeToString(data), len(data), nil
}

// artifactsDir is where content-addressed render artifacts (large PNGs returned
// as a path reference rather than inline base64) are written. It lives under the
// render cache dir so InvalidateCache clears artifacts and cache entries together.
func artifactsDir() string {
	return filepath.Join(cacheDir(), "artifacts")
}

// WriteArtifact persists PNG bytes to a content-addressed path under the render
// artifacts directory and returns that path, regardless of size. Unlike
// SlideImageFromBytes (which only writes a path when the image exceeds the inline
// cap), this always materializes a stable on-disk path — useful when a caller
// needs a filesystem path for every image, e.g. the visual-QA loop recording
// thumbnail paths in its trace. Identical bytes always map to the same path.
func WriteArtifact(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	return writeArtifact(data, hex.EncodeToString(sum[:]))
}

// writeArtifact writes PNG bytes to a content-addressed path under artifactsDir
// and returns that path. The filename embeds contentHash, so identical content
// always maps to the same path and different content never collides. An existing
// file of the same size already holds identical bytes, so the write is skipped.
func writeArtifact(data []byte, contentHash string) (string, error) {
	if err := os.MkdirAll(artifactsDir(), 0755); err != nil {
		return "", err
	}
	// path is internal: render cache dir + sha256 content hash, not user-tainted.
	path := filepath.Join(artifactsDir(), fmt.Sprintf("slide-%s.png", contentHash))
	if info, err := os.Stat(path); err == nil && info.Size() == int64(len(data)) { //nolint:gosec // path is internal (cache dir + content hash)
		return path, nil
	}
	if err := os.WriteFile(path, data, 0644); err != nil { //nolint:gosec // path is internal (cache dir + content hash)
		return "", err
	}
	return path, nil
}

// SlideImageFromBytes builds a SlideImage from in-memory PNG bytes, applying the
// size-based inline/path fan-out: images at or under maxInlineBytes are returned
// inline as base64; larger images are written to a content-addressed artifact
// path. ContentHash and SourceHash are always populated; Cleanup is set only when
// a Path artifact is produced. sourceHash identifies the upstream deck/input.
func SlideImageFromBytes(index int, data []byte, sourceHash string) (*SlideImage, error) {
	sum := sha256.Sum256(data)
	contentHash := hex.EncodeToString(sum[:])
	img := &SlideImage{
		Index:       index,
		ContentHash: contentHash,
		SourceHash:  sourceHash,
	}
	if len(data) > maxInlineBytes {
		path, err := writeArtifact(data, contentHash)
		if err != nil {
			return nil, fmt.Errorf("write artifact: %w", err)
		}
		img.Path = path
		img.Cleanup = ArtifactCleanupPolicy
	} else {
		img.PNG64 = base64.StdEncoding.EncodeToString(data)
	}
	return img, nil
}

// buildSlideImage reads a rendered PNG file and delegates to SlideImageFromBytes.
func buildSlideImage(index int, pngPath, sourceHash string) (*SlideImage, error) {
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return nil, err
	}
	return SlideImageFromBytes(index, data, sourceHash)
}

// RenderSlide renders a single slide from a PPTX file to a PNG.
// slideIndex is 0-based.
func RenderSlide(pptxPath string, slideIndex, density int) (*SlideImage, error) {
	return RenderSlideOpts(pptxPath, slideIndex, density, false)
}

// RenderSlideOpts renders a single slide with an option to bypass the cache.
func RenderSlideOpts(pptxPath string, slideIndex, density int, force bool) (*SlideImage, error) {
	if err := CheckDependencies(); err != nil {
		return nil, err
	}

	hash, err := hashFile(pptxPath)
	if err != nil {
		return nil, fmt.Errorf("hash pptx: %w", err)
	}

	key := cacheKey(hash, density)
	var pngs []string

	if !force {
		pngs = getCachedPNGs(key)
	}

	if pngs == nil {
		tmpDir, err := os.MkdirTemp("", "render-slide-*")
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		pdfPath, err := pptxToPDF(ctx, pptxPath, tmpDir)
		if err != nil {
			return nil, err
		}

		pngs, err = pdfToPNGs(ctx, pdfPath, tmpDir, density)
		if err != nil {
			return nil, err
		}

		storeCachePNGs(key, pngs)
	}

	if slideIndex < 0 || slideIndex >= len(pngs) {
		return nil, fmt.Errorf("slide_index %d out of range (deck has %d slides)", slideIndex, len(pngs))
	}

	img, err := buildSlideImage(slideIndex, pngs[slideIndex], hash)
	if err != nil {
		return nil, fmt.Errorf("read rendered slide: %w", err)
	}
	return img, nil
}

// RenderSlideWithCacheKey renders a single slide from a PPTX, caching the
// result under the caller-supplied cache key rather than the PPTX file content
// hash. Use this when the PPTX is a transient intermediate (e.g. generated on
// the fly from a single-slide JSON) whose own content hash is not a stable
// identity for the upstream design.
//
// The cache directory layout matches RenderSlideOpts (one subdirectory per
// key+density), so invalidation via InvalidateCache also clears these entries.
func RenderSlideWithCacheKey(pptxPath string, slideIndex, density int, force bool, key string) (*SlideImage, error) {
	if key == "" {
		return nil, fmt.Errorf("cache key is required")
	}
	if err := CheckDependencies(); err != nil {
		return nil, err
	}

	fullKey := cacheKey(key, density)
	var pngs []string

	if !force {
		pngs = getCachedPNGs(fullKey)
	}

	if pngs == nil {
		tmpDir, err := os.MkdirTemp("", "render-slide-keyed-*")
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		pdfPath, err := pptxToPDF(ctx, pptxPath, tmpDir)
		if err != nil {
			return nil, err
		}

		pngs, err = pdfToPNGs(ctx, pdfPath, tmpDir, density)
		if err != nil {
			return nil, err
		}

		storeCachePNGs(fullKey, pngs)
	}

	if slideIndex < 0 || slideIndex >= len(pngs) {
		return nil, fmt.Errorf("slide_index %d out of range (deck has %d slides)", slideIndex, len(pngs))
	}

	img, err := buildSlideImage(slideIndex, pngs[slideIndex], key)
	if err != nil {
		return nil, fmt.Errorf("read rendered slide: %w", err)
	}
	return img, nil
}

// LookupCachedSlide returns the cached PNG for the given key+density+index
// without invoking the renderer. Returns nil if no cached entry exists.
// This is the fast path for callers that want to avoid generating the
// intermediate PPTX when a prior render is still cached.
func LookupCachedSlide(key string, slideIndex, density int) *SlideImage {
	if key == "" {
		return nil
	}
	pngs := getCachedPNGs(cacheKey(key, density))
	if pngs == nil || slideIndex < 0 || slideIndex >= len(pngs) {
		return nil
	}
	img, err := buildSlideImage(slideIndex, pngs[slideIndex], key)
	if err != nil {
		return nil
	}
	return img
}

// RenderDeck renders all slides in a PPTX to PNG thumbnails.
func RenderDeck(pptxPath string, density, maxSlides int) (*DeckResult, error) {
	return RenderDeckOpts(pptxPath, density, maxSlides, false)
}

// RenderDeckOpts renders all slides with an option to bypass the cache.
// When force is true, the conversion is re-executed even if a cached result exists.
func RenderDeckOpts(pptxPath string, density, maxSlides int, force bool) (*DeckResult, error) {
	if err := CheckDependencies(); err != nil {
		return nil, err
	}

	hash, err := hashFile(pptxPath)
	if err != nil {
		return nil, fmt.Errorf("hash pptx: %w", err)
	}

	key := cacheKey(hash, density)
	var pngs []string

	if !force {
		pngs = getCachedPNGs(key)
	}

	if pngs == nil {
		tmpDir, err := os.MkdirTemp("", "render-deck-*")
		if err != nil {
			return nil, fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		ctx := context.Background()
		pdfPath, err := pptxToPDF(ctx, pptxPath, tmpDir)
		if err != nil {
			return nil, err
		}

		pngs, err = pdfToPNGs(ctx, pdfPath, tmpDir, density)
		if err != nil {
			return nil, err
		}

		storeCachePNGs(key, pngs)
	}

	result := &DeckResult{}
	limit := len(pngs)
	if maxSlides > 0 && maxSlides < limit {
		limit = maxSlides
		result.Truncated = true
	}

	for i := 0; i < limit; i++ {
		img, err := buildSlideImage(i, pngs[i], hash)
		if err != nil {
			result.Slides = append(result.Slides, SlideImage{Index: i, SizeErr: err.Error()})
			continue
		}
		result.Slides = append(result.Slides, *img)
	}

	return result, nil
}

// InvalidateCache removes all cached render results.
func InvalidateCache() error {
	return os.RemoveAll(cacheDir())
}
