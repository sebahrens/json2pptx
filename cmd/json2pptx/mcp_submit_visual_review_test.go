package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
