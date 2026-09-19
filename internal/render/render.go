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
	"sync/atomic"
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

// LibreOffice refuses to do two things at once inside one user profile: the
// second headless process attaches to the first one's session, exits 0, and
// writes no PDF. mu stops that happening inside this process, but it says
// nothing about the other processes on the machine — a second MCP server, a
// parallel agent, or the user's own open LibreOffice. Measured on macOS with
// four concurrent conversions against the shared default profile: 2 of 4
// produced no PDF and logged nothing at all (go-slide-creator-0ixs).
//
// So every conversion runs against a profile only this process uses. It is
// created once and reused: a cold profile costs LibreOffice ~0.55s to bootstrap
// (1.34s vs 0.80s per conversion, measured) and mu already means no two
// conversions here share it in time.
var (
	loProfileOnce sync.Once
	loProfileDir  string
	loProfileErr  error
	// loRetries counts conversions that had to be retried because LibreOffice
	// exited successfully and wrote no PDF. With a private profile this should
	// stay at zero; TestConcurrentConversion asserts it does, because the retry
	// alone is enough to make four concurrent conversions eventually succeed and
	// would otherwise hide a regression in the profile itself.
	loRetries atomic.Int64
)

// LibreOfficeRetryCount reports how many conversions in this process needed a
// retry after LibreOffice produced no PDF.
func LibreOfficeRetryCount() int64 { return loRetries.Load() }

// libreOfficeProfile returns this process's private LibreOffice profile
// directory, creating it on first use.
func libreOfficeProfile() (string, error) {
	loProfileOnce.Do(func() {
		loProfileDir, loProfileErr = os.MkdirTemp("", fmt.Sprintf("json2pptx-lo-%d-", os.Getpid()))
	})
	return loProfileDir, loProfileErr
}

// loProfileArg renders a profile directory as the -env:UserInstallation
// argument LibreOffice expects. An empty dir yields no argument, which means
// the shared default profile — only acceptable when a private one could not be
// created at all.
func loProfileArg(dir string) []string {
	if dir == "" {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	return []string{"-env:UserInstallation=file://" + filepath.ToSlash(abs)}
}

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
	// SlideCount is how many slides the deck has, whatever was returned. A
	// selective render makes that distinction load-bearing: "3 images back"
	// means nothing about the deck without it.
	SlideCount int `json:"slide_count,omitempty"`
	// Selected lists the 0-based indices a selective render returned, ascending.
	// Nil for a full-deck render.
	Selected []int `json:"selected,omitempty"`
}

// checkDep verifies that a command-line tool is available on PATH.
func checkDep(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: install it to use render tools", name)
	}
	return nil
}

// officeCommand returns the LibreOffice-compatible binary available on PATH.
// macOS Homebrew installs the CLI as soffice, while Linux packages commonly
// expose libreoffice.
func officeCommand() (string, error) {
	for _, name := range []string{"libreoffice", "soffice"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("libreoffice/soffice not found on PATH: install LibreOffice to use render tools")
}

// CheckDependencies verifies LibreOffice and ImageMagick are available.
func CheckDependencies() error {
	if _, err := officeCommand(); err != nil {
		return err
	}
	return checkDep("magick")
}

// DependencyStatus checks each render dependency and returns whether rendering
// is available and which commands are missing.
func DependencyStatus() (available bool, missing []string) {
	if _, err := officeCommand(); err != nil {
		missing = append(missing, "libreoffice/soffice")
	}
	if checkDep("magick") != nil {
		missing = append(missing, "magick")
	}
	return len(missing) == 0, missing
}

// pptxToPDF converts a PPTX to PDF via LibreOffice headless.
// Returns the path to the generated PDF. The conversion is bounded by
// libreOfficeTimeout: the LibreOffice mutex is held only for the duration of
// that deadline, never indefinitely, and a timeout returns a *TimeoutError.
//
// It runs against this process's private profile (see libreOfficeProfile) and,
// if LibreOffice still exits successfully without writing a PDF, retries once
// against a throwaway profile before giving up.
func pptxToPDF(ctx context.Context, pptxPath, tmpDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	office, err := officeCommand()
	if err != nil {
		return "", err
	}

	profile, profileErr := libreOfficeProfile()
	pdfPath, stderr, err := convertToPDF(ctx, office, profile, pptxPath, tmpDir)
	if err == nil {
		return pdfPath, nil
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		return "", err // structured timeout; propagate verbatim
	}
	if !errors.Is(err, errNoPDFProduced) {
		return "", err
	}

	// LibreOffice claimed success and produced nothing. That is the signature of
	// a profile it could not own, so the one retry uses a profile nothing else
	// has ever touched (go-slide-creator-0ixs).
	loRetries.Add(1)
	retryProfile, mkErr := os.MkdirTemp("", "json2pptx-lo-retry-")
	if mkErr == nil {
		defer func() { _ = os.RemoveAll(retryProfile) }()
		var retryStderr string
		pdfPath, retryStderr, err = convertToPDF(ctx, office, retryProfile, pptxPath, tmpDir)
		if err == nil {
			return pdfPath, nil
		}
		if errors.As(err, &te) {
			return "", err
		}
		if s := strings.TrimSpace(retryStderr); s != "" {
			stderr = s
		}
	}

	// Both attempts produced no PDF. Name the cause an agent can act on: this is
	// an environment collision, not a defect in the deck.
	msg := fmt.Sprintf("LibreOffice produced no PDF at %s after 2 attempts "+
		"(profile %s). This usually means another LibreOffice instance was running, "+
		"not that the deck is invalid: close any open LibreOffice and retry this call",
		pdfPathFor(pptxPath, tmpDir), profileDescription(profile, profileErr))
	if stderr = strings.TrimSpace(stderr); stderr != "" {
		msg += "; libreoffice said: " + stderr
	}
	return "", errors.New(msg)
}

// errNoPDFProduced marks the case where LibreOffice exited successfully but
// wrote no PDF — the concurrency signature, and the only case worth retrying.
var errNoPDFProduced = errors.New("libreoffice produced no PDF")

// convertToPDF runs one LibreOffice conversion against the given profile
// directory, returning the PDF path on success.
func convertToPDF(ctx context.Context, office, profile, pptxPath, tmpDir string) (pdfPath, stderr string, err error) {
	args := append(loProfileArg(profile),
		"--headless",
		"--convert-to", "pdf",
		"--outdir", tmpDir,
		pptxPath,
	)
	_, stderr, err = runBounded(ctx, toolLibreOffice, pptxPath, libreOfficeTimeout, office, args...)
	if err != nil {
		var te *TimeoutError
		if errors.As(err, &te) {
			return "", stderr, err // structured timeout; propagate verbatim
		}
		if s := strings.TrimSpace(stderr); s != "" {
			return "", stderr, fmt.Errorf("libreoffice conversion failed: %w: %s", err, s)
		}
		return "", stderr, fmt.Errorf("libreoffice conversion failed: %w", err)
	}

	pdfPath = pdfPathFor(pptxPath, tmpDir)
	if _, err := os.Stat(pdfPath); err != nil {
		return "", stderr, errNoPDFProduced
	}
	return pdfPath, stderr, nil
}

// pdfPathFor returns the path LibreOffice writes for a given input deck.
func pdfPathFor(pptxPath, tmpDir string) string {
	base := strings.TrimSuffix(filepath.Base(pptxPath), filepath.Ext(pptxPath))
	return filepath.Join(tmpDir, base+".pdf")
}

// profileDescription renders the profile directory for an error message,
// including the reason when no private profile could be created.
func profileDescription(profile string, err error) string {
	if err != nil {
		return fmt.Sprintf("none: %v", err)
	}
	if profile == "" {
		return "shared default"
	}
	return profile
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
	pngs, hash, cleanup, err := deckPNGs(pptxPath, density, force)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	result := &DeckResult{SlideCount: len(pngs)}
	limit := len(pngs)
	if maxSlides > 0 && maxSlides < limit {
		limit = maxSlides
		result.Truncated = true
	}

	for i := 0; i < limit; i++ {
		result.Slides = append(result.Slides, slideImageOrError(i, pngs[i], hash))
	}

	return result, nil
}

// RenderDeckIndices renders only the named 0-based slides. It exists for the
// repair loop: re-inspecting one changed slide out of fifteen used to mean
// pulling all fifteen thumbnails back, because the only narrowing knob was a
// prefix cap (go-slide-creator-2018). The deck is converted once either way —
// the saving is in what crosses the wire, which is where the cost is.
//
// Indices are de-duplicated and returned in ascending order regardless of the
// order asked for, so the images line up with Selected. An index outside the
// deck is an IndexRangeError rather than a silent omission: an agent that
// asked to look at slide 9 must not be told it looked at slide 9.
func RenderDeckIndices(pptxPath string, density int, indices []int, force bool) (*DeckResult, error) {
	pngs, hash, cleanup, err := deckPNGs(pptxPath, density, force)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	wanted := make([]int, 0, len(indices))
	seen := make(map[int]bool, len(indices))
	var bad []int
	for _, idx := range indices {
		if seen[idx] {
			continue
		}
		seen[idx] = true
		if idx < 0 || idx >= len(pngs) {
			bad = append(bad, idx)
			continue
		}
		wanted = append(wanted, idx)
	}
	if len(bad) > 0 {
		sort.Ints(bad)
		return nil, &IndexRangeError{Indices: bad, SlideCount: len(pngs)}
	}
	sort.Ints(wanted)

	result := &DeckResult{SlideCount: len(pngs), Selected: wanted}
	for _, idx := range wanted {
		result.Slides = append(result.Slides, slideImageOrError(idx, pngs[idx], hash))
	}
	return result, nil
}

// IndexRangeError reports slide indices a deck does not have, together with how
// many it does, so the caller can say both in one message.
type IndexRangeError struct {
	Indices    []int
	SlideCount int
}

func (e *IndexRangeError) Error() string {
	return fmt.Sprintf("slide indices %v are outside the deck: it has %d slides (0-%d)",
		e.Indices, e.SlideCount, e.SlideCount-1)
}

// slideImageOrError builds one slide's image, recording a per-slide error rather
// than failing the whole render: one unreadable thumbnail should not cost the
// caller the other fourteen.
func slideImageOrError(index int, png, hash string) SlideImage {
	img, err := buildSlideImage(index, png, hash)
	if err != nil {
		return SlideImage{Index: index, SizeErr: err.Error()}
	}
	return *img
}

// deckPNGs converts a deck to one PNG per slide, through the cache, and returns
// them with the PPTX content hash they were rendered from plus a cleanup the
// caller must run once it has read the files: on a cache miss the PNGs live in a
// temp directory, and deleting it before the caller reads them would turn every
// slide into "rendered image bytes unavailable".
func deckPNGs(pptxPath string, density int, force bool) (pngs []string, hash string, cleanup func(), err error) {
	cleanup = func() {}
	if err := CheckDependencies(); err != nil {
		return nil, "", cleanup, err
	}

	hash, err = hashFile(pptxPath)
	if err != nil {
		return nil, "", cleanup, fmt.Errorf("hash pptx: %w", err)
	}

	key := cacheKey(hash, density)
	if !force {
		pngs = getCachedPNGs(key)
	}
	if pngs != nil {
		return pngs, hash, cleanup, nil
	}

	tmpDir, err := os.MkdirTemp("", "render-deck-*")
	if err != nil {
		return nil, "", cleanup, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmpDir) }

	ctx := context.Background()
	pdfPath, err := pptxToPDF(ctx, pptxPath, tmpDir)
	if err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	pngs, err = pdfToPNGs(ctx, pdfPath, tmpDir, density)
	if err != nil {
		cleanup()
		return nil, "", func() {}, err
	}
	storeCachePNGs(key, pngs)
	return pngs, hash, cleanup, nil
}

// InvalidateCache removes all cached render results.
func InvalidateCache() error {
	return os.RemoveAll(cacheDir())
}

// CachedSlideHashes returns the content hashes of every rendered slide this
// process has cached for one PPTX, keyed by 0-based slide index. A slide can map
// to several hashes — one per density the deck was rendered at — because the
// cache stores one directory per (source hash, density) pair.
//
// sourceHash is the PPTX file's sha256, the same identity RenderDeck/RenderSlide
// cache under. An unknown deck, a hash that is not 64 hex digits, or an empty
// cache all return an empty map: callers must treat "no cached render" as "not
// verifiable", never as "does not match".
//
// It exists so submit_visual_review can bind a reviewer's submitted image to the
// artifact's own pixels without invoking LibreOffice (go-slide-creator-jltp).
func CachedSlideHashes(sourceHash string) map[int][]string {
	out := map[int][]string{}
	if !isHexHash(sourceHash) {
		return out
	}
	dirs, err := filepath.Glob(filepath.Join(cacheDir(), sourceHash+"-d*"))
	if err != nil {
		return out
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		pngs, gerr := filepath.Glob(filepath.Join(dir, "slide-*.png"))
		if gerr != nil {
			continue
		}
		sortPNGsByIndex(pngs)
		for _, png := range pngs {
			idx := pngIndexFromName(png)
			if idx < 0 {
				continue
			}
			data, rerr := os.ReadFile(png) //nolint:gosec // path comes from our own cache directory
			if rerr != nil {
				continue
			}
			sum := sha256.Sum256(data)
			hash := hex.EncodeToString(sum[:])
			if !containsString(out[idx], hash) {
				out[idx] = append(out[idx], hash)
			}
		}
	}
	return out
}

// isHexHash reports whether s is a 64-character lowercase hex digest. It keeps a
// caller-supplied identity out of the glob pattern used to walk the cache.
func isHexHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
