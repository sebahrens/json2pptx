package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDependencies(t *testing.T) {
	// This test verifies that checkDep returns a sensible error for
	// a command that definitely doesn't exist.
	err := checkDep("definitely-not-a-real-command-12345")
	if err == nil {
		t.Fatal("expected error for missing command")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestRenderSlide_FileNotFound(t *testing.T) {
	_, err := RenderSlide("/tmp/nonexistent-deck.pptx", 0, 100)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestRenderDeck_FileNotFound(t *testing.T) {
	_, err := RenderDeck("/tmp/nonexistent-deck.pptx", 50, 10)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadAsBase64(t *testing.T) {
	// Create a temp file with known content.
	f, err := os.CreateTemp("", "render-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	content := []byte("hello world")
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	b64, size, err := readAsBase64(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if size != len(content) {
		t.Errorf("size = %d, want %d", size, len(content))
	}
	if b64 == "" {
		t.Error("expected non-empty base64")
	}
}

// integrationPPTXPath resolves the PPTX used by the render integration tests.
// CI and local runs can point it at any generated deck via the RENDER_TEST_PPTX
// environment variable; otherwise it falls back to the conventional path that
// the render-integration CI job (see .github/workflows/ci.yml) generates. This
// keeps these tests pointable at a real deck instead of relying on a single
// hard-coded location. The test is skipped only when no deck is available.
//
// See docs/TESTING.md for the test-tier classification and how this fixture is
// provisioned in CI.
func integrationPPTXPath(t *testing.T) string {
	t.Helper()
	pptxPath := os.Getenv("RENDER_TEST_PPTX")
	if pptxPath == "" {
		pptxPath = "/tmp/render-test/basic-deck.pptx"
	}
	if _, err := os.Stat(pptxPath); os.IsNotExist(err) {
		t.Skipf("skipping integration test: PPTX not found at %s "+
			"(set RENDER_TEST_PPTX or run the render-integration CI job)", pptxPath)
	}
	return pptxPath
}

// TestIntegrationRenderSlide tests actual rendering if dependencies are available.
// Skipped in CI if libreoffice/magick are not installed.
func TestIntegrationRenderSlide(t *testing.T) {
	if err := CheckDependencies(); err != nil {
		t.Skipf("skipping integration test: %v", err)
	}

	pptxPath := integrationPPTXPath(t)

	img, err := RenderSlide(pptxPath, 0, 72)
	if err != nil {
		t.Fatalf("RenderSlide failed: %v", err)
	}

	if img.PNG64 == "" && img.Path == "" {
		t.Fatal("expected either PNG64 or Path to be set")
	}

	if img.PNG64 != "" {
		data, err := base64.StdEncoding.DecodeString(img.PNG64)
		if err != nil {
			t.Fatalf("invalid base64: %v", err)
		}
		// PNG magic bytes
		if len(data) < 8 || string(data[:4]) != "\x89PNG" {
			t.Fatal("decoded data is not a valid PNG")
		}
	}
}

func TestCacheKeyDiffersByDensity(t *testing.T) {
	k1 := cacheKey("abc123", 50)
	k2 := cacheKey("abc123", 100)
	if k1 == k2 {
		t.Fatal("cache keys should differ for different densities")
	}
}

func TestCacheKeyDiffersByHash(t *testing.T) {
	k1 := cacheKey("hash1", 50)
	k2 := cacheKey("hash2", 50)
	if k1 == k2 {
		t.Fatal("cache keys should differ for different hashes")
	}
}

func TestHashFile(t *testing.T) {
	f, err := os.CreateTemp("", "render-hash-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if _, err := f.Write([]byte("content A")); err != nil {
		t.Fatal(err)
	}
	f.Close()

	h1, err := hashFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same content => same hash
	h2, err := hashFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatal("same file should produce same hash")
	}

	// Change content => different hash
	if err := os.WriteFile(f.Name(), []byte("content B"), 0644); err != nil {
		t.Fatal(err)
	}
	h3, err := hashFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h3 {
		t.Fatal("different content should produce different hash")
	}
}

// TestSortPNGsByIndex_NumericOrder is a regression test for the lex-sort bug
// that caused render-slide --slide-index N to return the wrong slide for decks
// with >9 slides. Files named "slide-0.png" through "slide-12.png" must sort
// in numeric order (0,1,2,...,12), not lexicographic (0,1,10,11,12,2,3,...).
func TestSortPNGsByIndex_NumericOrder(t *testing.T) {
	// Mimic the shuffled order ImageMagick + Glob returns lexicographically.
	files := []string{
		"/tmp/foo/slide-0.png",
		"/tmp/foo/slide-1.png",
		"/tmp/foo/slide-10.png",
		"/tmp/foo/slide-11.png",
		"/tmp/foo/slide-12.png",
		"/tmp/foo/slide-2.png",
		"/tmp/foo/slide-3.png",
		"/tmp/foo/slide-4.png",
		"/tmp/foo/slide-5.png",
		"/tmp/foo/slide-6.png",
		"/tmp/foo/slide-7.png",
		"/tmp/foo/slide-8.png",
		"/tmp/foo/slide-9.png",
	}

	sortPNGsByIndex(files)

	for i, p := range files {
		if got := pngIndexFromName(p); got != i {
			t.Errorf("position %d: got slide-%d.png, want slide-%d.png (full=%s)", i, got, i, p)
		}
	}
}

// TestStoreCacheAndRetrieve_ManySlides verifies the cache round-trip preserves
// presentation order when there are more than 9 slides — i.e. position N in
// the returned slice corresponds to the Nth source slide, not the Nth lex-sorted
// filename.
func TestStoreCacheAndRetrieve_ManySlides(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cache-test-many-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	const n = 13
	var pngs []string
	for i := 0; i < n; i++ {
		p := filepath.Join(tmpDir, fmt.Sprintf("slide-%d.png", i))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("png-data-%d", i)), 0644); err != nil {
			t.Fatal(err)
		}
		pngs = append(pngs, p)
	}

	key := "test-store-retrieve-many-key"
	defer os.RemoveAll(filepath.Join(cacheDir(), key))

	storeCachePNGs(key, pngs)

	cached := getCachedPNGs(key)
	if len(cached) != n {
		t.Fatalf("expected %d cached PNGs, got %d", n, len(cached))
	}

	for i, path := range cached {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := fmt.Sprintf("png-data-%d", i)
		if string(data) != expected {
			t.Errorf("cached file at position %d: got %q, want %q (path=%s)", i, string(data), expected, path)
		}
	}
}

func TestGetCachedPNGs_Miss(t *testing.T) {
	result := getCachedPNGs("nonexistent-key-12345")
	if result != nil {
		t.Fatal("expected nil for cache miss")
	}
}

func TestStoreCacheAndRetrieve(t *testing.T) {
	// Create temp PNGs
	tmpDir, err := os.MkdirTemp("", "cache-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	var pngs []string
	for i := 0; i < 3; i++ {
		p := filepath.Join(tmpDir, fmt.Sprintf("slide-%d.png", i))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("png-data-%d", i)), 0644); err != nil {
			t.Fatal(err)
		}
		pngs = append(pngs, p)
	}

	key := "test-store-retrieve-key"
	// Clean up after test
	defer os.RemoveAll(filepath.Join(cacheDir(), key))

	storeCachePNGs(key, pngs)

	cached := getCachedPNGs(key)
	if len(cached) != 3 {
		t.Fatalf("expected 3 cached PNGs, got %d", len(cached))
	}

	// Verify content
	for i, path := range cached {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		expected := fmt.Sprintf("png-data-%d", i)
		if string(data) != expected {
			t.Errorf("cached file %d: got %q, want %q", i, string(data), expected)
		}
	}
}

// TestSlideImageFromBytes_PathCollisionFreeAcrossDecks is the core regression
// test for the index-addressed-path bug: two different decks rendering the SAME
// slide index used to write to the same /tmp/json2pptx-slide-N.png and clobber
// each other. Artifacts are now content-addressed, so distinct content yields
// distinct paths and neither overwrites the other.
func TestSlideImageFromBytes_PathCollisionFreeAcrossDecks(t *testing.T) {
	deckA := bytes.Repeat([]byte{0xA1}, maxInlineBytes+1000)
	deckB := bytes.Repeat([]byte{0xB2}, maxInlineBytes+1000)

	const sameIndex = 0
	imgA, err := SlideImageFromBytes(sameIndex, deckA, "deck-A-source")
	if err != nil {
		t.Fatal(err)
	}
	imgB, err := SlideImageFromBytes(sameIndex, deckB, "deck-B-source")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(imgA.Path)
	defer os.Remove(imgB.Path)

	if imgA.Path == "" || imgB.Path == "" {
		t.Fatalf("expected path references for large images, got A=%q B=%q", imgA.Path, imgB.Path)
	}
	if imgA.Path == imgB.Path {
		t.Fatalf("path collision: two decks at slide index %d returned the same path %q", sameIndex, imgA.Path)
	}

	// Each file must still hold its own deck's bytes — neither clobbered the other.
	if got, _ := os.ReadFile(imgA.Path); !bytes.Equal(got, deckA) {
		t.Error("artifact A content was overwritten or corrupted")
	}
	if got, _ := os.ReadFile(imgB.Path); !bytes.Equal(got, deckB) {
		t.Error("artifact B content was overwritten or corrupted")
	}
}

// TestSlideImageFromBytes_IdenticalContentSamePath verifies the only case where
// two renders are allowed to share a path: byte-identical content. Index and
// source identity do not affect the path — the content hash does.
func TestSlideImageFromBytes_IdenticalContentSamePath(t *testing.T) {
	data := bytes.Repeat([]byte{0xC3}, maxInlineBytes+500)

	img1, err := SlideImageFromBytes(0, data, "src-1")
	if err != nil {
		t.Fatal(err)
	}
	img2, err := SlideImageFromBytes(5, data, "src-2") // different index + source, same bytes
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(img1.Path)

	if img1.Path != img2.Path {
		t.Errorf("identical content should map to the same path: %q vs %q", img1.Path, img2.Path)
	}
	if img1.ContentHash != img2.ContentHash {
		t.Errorf("identical content should produce the same content hash: %q vs %q", img1.ContentHash, img2.ContentHash)
	}
}

// TestSlideImageFromBytes_LargeArtifactFields verifies the response carries the
// stable-identity fields (content hash, source identity, slide index) and the
// cleanup semantics required for an on-disk artifact.
func TestSlideImageFromBytes_LargeArtifactFields(t *testing.T) {
	data := bytes.Repeat([]byte{0xD4}, maxInlineBytes+1)

	img, err := SlideImageFromBytes(2, data, "deck-source-hash")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(img.Path)

	if img.PNG64 != "" {
		t.Error("large image should not be inlined as base64")
	}
	if img.Path == "" {
		t.Fatal("large image should produce a path reference")
	}
	if img.Index != 2 {
		t.Errorf("index = %d, want 2", img.Index)
	}
	if img.ContentHash == "" {
		t.Error("content_hash must be populated")
	}
	if img.SourceHash != "deck-source-hash" {
		t.Errorf("source_hash = %q, want %q", img.SourceHash, "deck-source-hash")
	}
	if img.Cleanup != ArtifactCleanupPolicy {
		t.Errorf("cleanup = %q, want ArtifactCleanupPolicy", img.Cleanup)
	}
	if !strings.Contains(filepath.Base(img.Path), img.ContentHash) {
		t.Errorf("path %q does not embed content hash %q", img.Path, img.ContentHash)
	}
	if filepath.Dir(img.Path) != artifactsDir() {
		t.Errorf("artifact dir = %q, want %q", filepath.Dir(img.Path), artifactsDir())
	}
}

// TestSlideImageFromBytes_SmallInline verifies small images are still delivered
// inline as base64, carry a content/source hash, and report no cleanup (there is
// no on-disk artifact to clean up).
func TestSlideImageFromBytes_SmallInline(t *testing.T) {
	data := []byte("small png bytes")

	img, err := SlideImageFromBytes(1, data, "src")
	if err != nil {
		t.Fatal(err)
	}

	if img.Path != "" {
		t.Errorf("small image should be inline, got path %q", img.Path)
	}
	if img.PNG64 == "" {
		t.Error("small image should be inlined as base64")
	}
	if img.Cleanup != "" {
		t.Errorf("inline image should have no cleanup semantics, got %q", img.Cleanup)
	}
	if img.ContentHash == "" {
		t.Error("content_hash must be populated even for inline images")
	}
	if img.SourceHash != "src" {
		t.Errorf("source_hash = %q, want %q", img.SourceHash, "src")
	}

	decoded, err := base64.StdEncoding.DecodeString(img.PNG64)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, data) {
		t.Error("inline base64 did not round-trip to original bytes")
	}
}

func TestIntegrationRenderDeck(t *testing.T) {
	if err := CheckDependencies(); err != nil {
		t.Skipf("skipping integration test: %v", err)
	}

	pptxPath := integrationPPTXPath(t)
	usingDefaultDeck := os.Getenv("RENDER_TEST_PPTX") == ""

	result, err := RenderDeck(pptxPath, 50, 3)
	if err != nil {
		t.Fatalf("RenderDeck failed: %v", err)
	}

	if len(result.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	if len(result.Slides) > 3 {
		t.Errorf("expected at most 3 slides, got %d", len(result.Slides))
	}
	// The default basic-deck has 7 slides, so truncated should be true with
	// max=3. Only assert this for the default deck; an overridden deck
	// (RENDER_TEST_PPTX) may have a different slide count.
	if usingDefaultDeck && !result.Truncated {
		t.Error("expected truncated=true with max_slides=3 on a 7-slide deck")
	}
}
