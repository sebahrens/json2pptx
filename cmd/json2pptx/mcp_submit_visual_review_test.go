package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// renderReviewFixture renders a small semantic deck (which also writes the
// .authoring.json manifest) and fake per-slide PNGs, returning the pptx path,
// its sha256, and the image paths.
func renderReviewFixture(t *testing.T) (string, string, []string) {
	t.Helper()
	spec := writeSpec(t, "deck.yaml", validSemanticSpec)
	dir := t.TempDir()
	out := filepath.Join(dir, "deck.pptx")
	if stdout, err := runSemanticArgs(t, "render", "--spec", spec, "--output", out, "--templates-dir", testTemplatesDir); err != nil {
		t.Fatalf("render: %v\n%s", err, stdout)
	}
	art, err := describeArtifact(out, "pptx")
	if err != nil {
		t.Fatal(err)
	}
	n, err := countPPTXSlides(out)
	if err != nil || n != 2 {
		t.Fatalf("countPPTXSlides = %d, %v; want 2", n, err)
	}
	var imgs []string
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, fmt.Sprintf("slide-%d.png", i+1))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("pixels-%d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		imgs = append(imgs, p)
	}
	return out, art.SHA256, imgs
}

// stubRenderCache makes verifyReviewImages see imgs as this artifact's own
// rendered slides: index i renders to the bytes of imgs[i]. Real verification
// reads the render cache, which needs LibreOffice to populate — the seam keeps
// these tests hermetic (go-slide-creator-jltp).
func stubRenderCache(t *testing.T, imgs []string) {
	t.Helper()
	byIndex := map[int][]string{}
	for i, p := range imgs {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		byIndex[i] = []string{visualqa.PixelHash(data)}
	}
	stubRenderCacheHashes(t, byIndex)
}

// stubRenderCacheHashes installs an explicit index→hashes render set.
func stubRenderCacheHashes(t *testing.T, byIndex map[int][]string) {
	t.Helper()
	prev := cachedSlideHashes
	cachedSlideHashes = func(string) map[int][]string { return byIndex }
	t.Cleanup(func() { cachedSlideHashes = prev })
}

func allSlides(imgs []string, verdict string) []visualReviewSlideInput {
	out := make([]visualReviewSlideInput, len(imgs))
	for i := range imgs {
		idx := i
		out[i] = visualReviewSlideInput{Index: &idx, Verdict: verdict, ImagePath: imgs[i]}
	}
	return out
}

func TestSubmitVisualReview_CompleteReviewFlipsEvidence(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)
	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(imgs, "approved")})
	if err != nil {
		t.Fatalf("complete review rejected: %v", err)
	}
	if out.Status != visualReviewCompleteStatus || !out.Evidence.Approved || out.Evidence.NeedsReview {
		t.Errorf("status=%q approved=%v reasons=%v", out.Status, out.Evidence.Approved, out.Evidence.Reasons)
	}
	if out.Evidence.InspectionBackend != "host" || !out.Evidence.VisuallyInspected || len(out.Evidence.ReviewedSlideIDs) != 2 {
		t.Errorf("evidence = %+v", out.Evidence)
	}
	if out.Review.Slides[0].ImageSHA256 != visualqa.PixelHash([]byte("pixels-0")) {
		t.Error("image_path was not hashed into the review record")
	}
	// The authoring manifest for this artifact carries the host verdict.
	if !out.ManifestUpdated {
		t.Fatalf("manifest not updated (notes=%v)", out.Notes)
	}
	m, err := pipeline.ReadAuthoringManifest(out.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if m.VisualEvidence == nil || m.VisualEvidence.Reviewer != "host" || m.VisualEvidence.Verdict != "approved" || m.VisualEvidence.ArtifactSHA256 != sha {
		t.Errorf("manifest visual_evidence = %+v", m.VisualEvidence)
	}
	if out.Revision != m.Revision {
		t.Errorf("review bound to revision %q, manifest revision %q", out.Revision, m.Revision)
	}
}

func TestSubmitVisualReview_ChangesRequestedStaysDraft(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)
	slides := allSlides(imgs, "approved")
	slides[1].Findings = []visualqa.Finding{{Severity: visualqa.SeverityP1, Category: "contrast", Description: "low contrast"}}
	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Reviewer: "manual", Slides: slides})
	if err != nil {
		t.Fatalf("review rejected: %v", err)
	}
	if out.Verdict != "changes_requested" || out.Status != visualReviewDraftStatus || out.Evidence.Approved {
		t.Errorf("verdict=%q status=%q approved=%v", out.Verdict, out.Status, out.Evidence.Approved)
	}
	if len(out.Review.Findings) != 1 || out.Review.Findings[0].SlideIndex != 1 || out.Review.Findings[0].Source != "manual" {
		t.Errorf("findings = %+v", out.Review.Findings)
	}
}

func TestSubmitVisualReview_RejectsPartialAndStale(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)
	cases := map[string]submitVisualReviewInput{
		"partial coverage": {PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(imgs[:1], "approved")},
		"stale revision":   {PPTXPath: pptxPath, PPTXRevision: "deadbeef", Slides: allSlides(imgs, "approved")},
		"stale semantic":   {PPTXPath: pptxPath, PPTXRevision: sha, Revision: "old-revision", Slides: allSlides(imgs, "approved")},
		"bad verdict":      {PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(imgs, "looks-fine")},
		"bad reviewer":     {PPTXPath: pptxPath, PPTXRevision: sha, Reviewer: "vision", Slides: allSlides(imgs, "approved")},
		"missing pixels":   {PPTXPath: pptxPath, PPTXRevision: sha, Slides: func() []visualReviewSlideInput { s := allSlides(imgs, "approved"); s[0].ImagePath = ""; return s }()},
		"duplicated slide": {PPTXPath: pptxPath, PPTXRevision: sha, Slides: func() []visualReviewSlideInput {
			s := allSlides(imgs, "approved")
			zero := 0
			s[1].Index = &zero
			return s
		}()},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := submitVisualReview(in)
			if !errors.Is(err, errVisualReviewRejected) {
				t.Errorf("expected rejection, got %v", err)
			}
		})
	}
	m, err := pipeline.ReadAuthoringManifest(pptxPath + ".authoring.json")
	if err != nil {
		t.Fatal(err)
	}
	if m.VisualEvidence != nil {
		t.Error("a rejected review must not write manifest evidence")
	}
}

func TestHandleSubmitVisualReview_MCP(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)
	slides := []any{}
	for i, p := range imgs {
		slides = append(slides, map[string]any{"index": i, "verdict": "approved", "image_path": p})
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pptx_path": pptxPath, "pptx_revision": sha, "slides": slides}
	res, err := handleSubmitVisualReview(context.Background(), req)
	if err != nil || res.IsError {
		t.Fatalf("handler error: %v %+v", err, res)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out submitVisualReviewOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Status != visualReviewCompleteStatus {
		t.Errorf("status = %q", out.Status)
	}

	req.Params.Arguments = map[string]any{"pptx_path": pptxPath, "pptx_revision": sha, "slides": slides[:1]}
	res, _ = handleSubmitVisualReview(context.Background(), req)
	if !res.IsError {
		t.Error("partial review via MCP must be an error result")
	}
}

// go-slide-creator-jltp: a review of six slides all pointing at slide-01.png,
// and a review of another deck's images, both returned
// "visually_reviewed_current_revision". Only the PPTX sha256 was checked, so the
// completion contract was an honour system.
func TestSubmitVisualReview_RejectsRecycledImage(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)

	slides := allSlides(imgs, "approved")
	slides[1].ImagePath = imgs[0] // every slide "reviewed" from slide 0's pixels
	_, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: slides})
	if !errors.Is(err, errVisualReviewRejected) {
		t.Fatalf("recycled image accepted: err = %v", err)
	}
	// The message must name where the image really came from, or the agent
	// cannot tell a recycled image from a stale render.
	if !strings.Contains(err.Error(), "is slide 0 of this deck") {
		t.Errorf("error should identify the real slide: %v", err)
	}
}

func TestSubmitVisualReview_RejectsForeignDeckImages(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)

	// Images rendered from a different deck: same count, same indices, other pixels.
	other := t.TempDir()
	foreign := make([]string, len(imgs))
	for i := range imgs {
		p := filepath.Join(other, fmt.Sprintf("slide-%d.png", i+1))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("other-deck-pixels-%d", i)), 0o600); err != nil {
			t.Fatal(err)
		}
		foreign[i] = p
	}
	_, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(foreign, "approved")})
	if !errors.Is(err, errVisualReviewRejected) {
		t.Fatalf("foreign images accepted: err = %v", err)
	}
	if !strings.Contains(err.Error(), "matches no render of artifact") {
		t.Errorf("error should say the image is not from this artifact: %v", err)
	}
	m, merr := pipeline.ReadAuthoringManifest(pptxPath + ".authoring.json")
	if merr != nil {
		t.Fatal(merr)
	}
	if m.VisualEvidence != nil {
		t.Error("a rejected review must not write manifest evidence")
	}
}

// With no render of the artifact to compare against, the images can be neither
// confirmed nor refuted: the review is recorded, but never as completion.
func TestSubmitVisualReview_UnverifiableWithoutRender(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCacheHashes(t, map[int][]string{})

	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(imgs, "approved")})
	if err != nil {
		t.Fatalf("unverifiable review must be recorded, not rejected: %v", err)
	}
	if out.Status != visualReviewUnverifiedStatus {
		t.Errorf("status = %q, want %q", out.Status, visualReviewUnverifiedStatus)
	}
	if out.Evidence.Approved || out.Evidence.PixelsRendered || out.Evidence.VisuallyInspected {
		t.Errorf("unverified images must not claim inspected pixels: %+v", out.Evidence)
	}
	if out.ImageVerification == nil || out.ImageVerification.Status != imageVerificationUnverifiable || out.ImageVerification.VerifiedSlides != 0 {
		t.Errorf("image_verification = %+v", out.ImageVerification)
	}
	if out.ImageVerification.HowToVerify == "" {
		t.Error("image_verification must tell the agent how to produce verifiable images")
	}
	if out.ManifestUpdated {
		t.Error("an unverified review must not become durable manifest evidence")
	}
}

// The honest path may submit the content_hash render_deck_thumbnails returned
// instead of a file path.
func TestSubmitVisualReview_AcceptsSubmittedPixelHash(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)

	slides := allSlides(imgs, "approved")
	for i := range slides {
		data, err := os.ReadFile(imgs[i])
		if err != nil {
			t.Fatal(err)
		}
		slides[i].ImagePath = ""
		slides[i].ImageSHA256 = strings.ToUpper(visualqa.PixelHash(data)) // case-insensitive
	}
	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: slides})
	if err != nil {
		t.Fatalf("hash-only review rejected: %v", err)
	}
	if out.Status != visualReviewCompleteStatus || !out.ImageVerification.verified() {
		t.Errorf("status=%q verification=%+v", out.Status, out.ImageVerification)
	}
}

// Two slides that genuinely render to identical pixels (a repeated divider) must
// still verify: the rule is "each image is this slide's render", not "all hashes
// differ".
func TestSubmitVisualReview_IdenticalSlidesVerify(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	data, err := os.ReadFile(imgs[0])
	if err != nil {
		t.Fatal(err)
	}
	h := visualqa.PixelHash(data)
	stubRenderCacheHashes(t, map[int][]string{0: {h}, 1: {h}})

	slides := allSlides(imgs, "approved")
	slides[1].ImagePath = imgs[0]
	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: slides})
	if err != nil {
		t.Fatalf("identical slides rejected: %v", err)
	}
	if out.Status != visualReviewCompleteStatus {
		t.Errorf("status = %q, want completion", out.Status)
	}
}

// A deck rendered at several densities has several valid hashes per slide; any
// of them proves the reviewer looked at this artifact.
func TestSubmitVisualReview_AnyCachedDensityVerifies(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	byIndex := map[int][]string{}
	for i := range imgs {
		data, err := os.ReadFile(imgs[i])
		if err != nil {
			t.Fatal(err)
		}
		// index i: a density the reviewer did not use, plus the one it did.
		byIndex[i] = []string{visualqa.PixelHash([]byte(fmt.Sprintf("d150-%d", i))), visualqa.PixelHash(data)}
	}
	stubRenderCacheHashes(t, byIndex)
	out, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(imgs, "approved")})
	if err != nil {
		t.Fatalf("review rejected: %v", err)
	}
	if out.Status != visualReviewCompleteStatus || out.ImageVerification.VerifiedSlides != 2 {
		t.Errorf("status=%q verification=%+v", out.Status, out.ImageVerification)
	}
}
