package main

// Regression tests for go-slide-creator-sr3xk: a forced thumbnail refresh of an
// unchanged revision rejected an earlier, legitimate review because each
// re-render's PNG carried fresh timestamp metadata and identity was the hash of
// the file bytes. Identity is now a pixel hash shared by the render cache, the
// render responses and the verifier.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// withTextChunk inserts a tEXt chunk after IHDR — the shape of ImageMagick's
// date:create / xmp timestamp metadata — leaving the pixels untouched.
func withTextChunk(t *testing.T, data []byte, key, value string) []byte {
	t.Helper()
	const ihdrEnd = 8 + 4 + 4 + 13 + 4
	if len(data) < ihdrEnd || string(data[12:16]) != "IHDR" {
		t.Fatal("not a PNG with a leading IHDR")
	}
	body := append([]byte("tEXt"), []byte(key+"\x00"+value)...)
	var out bytes.Buffer
	out.Write(data[:ihdrEnd])
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(body)-4)) //nolint:gosec // test data
	out.Write(n[:])
	out.Write(body)
	binary.BigEndian.PutUint32(n[:], crc32.ChecksumIEEE(body))
	out.Write(n[:])
	out.Write(data[ihdrEnd:])
	return out.Bytes()
}

// TestSubmitVisualReview_MetadataOnlyDifferenceVerifies runs the real render
// cache reader and verifier (no stub) without LibreOffice: the cache holds a
// re-render whose PNGs differ from the reviewed images only in timestamp
// metadata. The review must verify; wrong-slide and foreign images must not.
func TestSubmitVisualReview_MetadataOnlyDifferenceVerifies(t *testing.T) {
	pptxPath, sha, _ := renderReviewFixture(t)
	t.Setenv("TMPDIR", t.TempDir()) // isolate the render cache (os.TempDir)

	dir := t.TempDir()
	cacheEntry := filepath.Join(os.TempDir(), "json2pptx-render-cache", sha+"-d100")
	if err := os.MkdirAll(cacheEntry, 0o755); err != nil {
		t.Fatal(err)
	}
	var reviewed []string
	for i := 0; i < 2; i++ {
		base := syntheticSlidePNG(t, 64, 36, uint8(10+i)) //nolint:gosec // test seed
		first := withTextChunk(t, base, "date:create", "2026-10-02T11:44:40+00:00")
		refreshed := withTextChunk(t, base, "date:create", "2026-10-02T11:49:03+00:00")
		path := filepath.Join(dir, fmt.Sprintf("slide-%d.png", i))
		reviewed = append(reviewed, path)
		if err := os.WriteFile(path, first, 0o600); err != nil {
			t.Fatal(err)
		}
		// The forced refresh replaced the cache entry with new file bytes.
		if err := os.WriteFile(filepath.Join(cacheEntry, fmt.Sprintf("slide-%d.png", i)), refreshed, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(reviewed, "approved")})
	if err != nil {
		t.Fatalf("review of unchanged pixels rejected after a refresh: %v", err)
	}
	if out.Status != visualReviewCompleteStatus || !out.ImageVerification.verified() || out.ImageVerification.VerifiedSlides != 2 {
		t.Fatalf("status=%q verification=%+v", out.Status, out.ImageVerification)
	}

	// Provenance is not weakened: slide 0's image submitted as slide 1 ...
	swapped := allSlides(reviewed, "approved")
	swapped[1].ImagePath = reviewed[0]
	if _, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: swapped}); !errors.Is(err, errVisualReviewRejected) {
		t.Errorf("wrong-slide image accepted: %v", err)
	}
	// ... and an image from another deck are both rejected.
	foreign := filepath.Join(dir, "foreign.png")
	if err := os.WriteFile(foreign, syntheticSlidePNG(t, 64, 36, 99), 0o600); err != nil {
		t.Fatal(err)
	}
	other := allSlides(reviewed, "approved")
	other[0].ImagePath = foreign
	if _, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: other}); !errors.Is(err, errVisualReviewRejected) {
		t.Errorf("foreign image accepted: %v", err)
	}
}

// TestSubmitVisualReview_SurvivesSelectedForceRefresh is the end-to-end repro:
// full render, a forced refresh of one slide after the clock advances, then the
// original all-slide review resubmitted with the images and hashes retained
// from the first render.
func TestSubmitVisualReview_SurvivesSelectedForceRefresh(t *testing.T) {
	if testing.Short() {
		t.Skip("render integration skipped in -short mode")
	}
	if ok, _ := render.DependencyStatus(); !ok {
		t.Skip("LibreOffice/ImageMagick not installed")
	}
	pptxPath, sha, _ := renderReviewFixture(t)
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir(), cache: template.NewMemoryCache(24 * time.Hour)}

	first, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{"pptx_path": pptxPath, "density": 100, "force": true}))
	if err != nil {
		t.Fatal(err)
	}
	full := assertImageContentDeck(t, first, 2)

	// Retain what the reviewer inspected: a copy of each PNG (the artifact path
	// could be rewritten later) and its content_hash.
	dir := t.TempDir()
	retained := make([]string, len(full.Slides))
	hashes := make([]string, len(full.Slides))
	for i, s := range full.Slides {
		data, err := os.ReadFile(s.Path)
		if err != nil {
			t.Fatal(err)
		}
		if got := visualqa.PixelHash(data); got != s.ContentHash {
			t.Fatalf("slide %d: content_hash %s is not the pixel hash %s of its own path", i, s.ContentHash, got)
		}
		retained[i] = filepath.Join(dir, fmt.Sprintf("slide-%d.png", i))
		if err := os.WriteFile(retained[i], data, 0o600); err != nil {
			t.Fatal(err)
		}
		hashes[i] = s.ContentHash
	}

	time.Sleep(1100 * time.Millisecond) // metadata timestamps have 1s resolution
	refresh, err := mc.handleRenderDeckThumbnails(context.Background(), makeRequest(map[string]any{
		"pptx_path": pptxPath, "density": 100, "slide_indices": []any{1}, "force": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	sub := assertImageContentDeck(t, refresh, 1)
	if sub.Slides[0].ContentHash != hashes[1] {
		t.Errorf("forced re-render of unchanged slide 1 changed its content_hash: %s -> %s", hashes[1], sub.Slides[0].ContentHash)
	}

	byPath, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(retained, "approved")})
	if err != nil {
		t.Fatalf("original review rejected after a forced selected refresh: %v", err)
	}
	if byPath.Status != visualReviewCompleteStatus || byPath.ImageVerification.VerifiedSlides != 2 {
		t.Fatalf("status=%q verification=%+v", byPath.Status, byPath.ImageVerification)
	}

	byHash := allSlides(retained, "approved")
	for i := range byHash {
		byHash[i].ImagePath, byHash[i].ImageSHA256 = "", hashes[i]
	}
	if _, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: byHash}); err != nil {
		t.Fatalf("retained content_hash review rejected after a refresh: %v", err)
	}

	// Provenance checks still bite on real renders.
	swapped := allSlides(retained, "approved")
	swapped[0].ImagePath, swapped[1].ImagePath = retained[1], retained[0]
	if _, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: swapped}); !errors.Is(err, errVisualReviewRejected) {
		t.Errorf("swapped slide images accepted: %v", err)
	}
	if _, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: hashes[0], Slides: allSlides(retained, "approved")}); err == nil {
		t.Error("review bound to a revision that is not the PPTX's sha256 accepted")
	}
	// Changed pixels in a retained image are a different image.
	img, err := png.Decode(bytes.NewReader(mustRead(t, retained[0])))
	if err != nil {
		t.Fatal(err)
	}
	rgba := image.NewRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}
	rgba.Pix[0] ^= 0xFF
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(dir, "tampered.png")
	if err := os.WriteFile(tampered, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := allSlides(retained, "approved")
	bad[0].ImagePath = tampered
	if _, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: bad}); !errors.Is(err, errVisualReviewRejected) {
		t.Errorf("image with altered pixels accepted: %v", err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	return data
}
