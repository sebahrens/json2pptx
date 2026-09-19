package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/visualqa"
)

// submit_visual_review records a host/manual reviewer's all-slide verdict for
// a generated PPTX. It is the completion path for decks inspected without a
// configured vision provider: the verdict is validated with
// visualqa.ReviewRecord.ValidateCompletion (every slide covered, pixel hashes
// present, bound to the current artifact revision), then promoted into
// pipeline.QualityEvidence (inspection_backend = host|manual) and — when the
// deck has a `.authoring.json` manifest for this exact artifact — persisted as
// the manifest's visual_evidence.

const (
	visualReviewCompleteStatus = "visually_reviewed_current_revision"
	visualReviewDraftStatus    = "draft_needs_visual_review"
	// visualReviewUnverifiedStatus is recorded when the review itself is well
	// formed and positive but its images could not be matched against this
	// artifact's own rendered pixels. It is deliberately NOT the completion
	// status (go-slide-creator-jltp).
	visualReviewUnverifiedStatus = "reviewed_unverified_images"
)

func mcpSubmitVisualReviewTool() mcp.Tool {
	return mcp.NewTool("submit_visual_review",
		mcp.WithDescription(`Record a host/manual visual review verdict for a generated PPTX — the completion path when no vision provider (ANTHROPIC_API_KEY) is configured, or when a human/host agent inspected the rendered slides itself.

Inputs: pptx_path, pptx_revision (the sha256 content_hash of the exact PPTX you reviewed, as returned by generate_presentation / render_deck_spec), and one entry per slide in slides[]: {index (0-based), verdict: approved|changes_requested|inconclusive, image_path or image_sha256 (the rendered PNG you inspected, e.g. from render_deck_thumbnails), role?, findings?: [{severity, category, description, location?, bbox?}]}. Optional reviewer: "host" (default) or "manual"; optional revision (semantic manifest revision, checked when given).

The review is validated by ReviewRecord.ValidateCompletion: EVERY slide must be covered exactly once with a pixel hash, and pptx_revision must equal the current file's sha256 — partial coverage or a stale revision is rejected (INVALID_PARAMETER) and nothing is recorded.

The images are evidence, so they are checked against the artifact's own pixels: each slide's pixel hash must equal this server's render of that slide of this exact PPTX (any density it was rendered at counts). Submitting another slide's image, or another deck's, is rejected with INVALID_PARAMETER naming which slide the image really is. Render with render_deck_thumbnails (or render_slide_image per slide) and submit the returned path / content_hash. When this server has no render of the artifact to compare against, the review is still recorded but image_verification.status is "unverifiable", evidence.pixels_rendered stays false, and the status is "reviewed_unverified_images" — never the completion status, and no manifest evidence is written.

On success the response carries quality evidence with inspection_backend=host|manual; status is "visually_reviewed_current_revision" only when every slide is approved with no P0/P1 finding, the artifact passes structural output validation, AND image_verification.status is "verified". When <pptx_path>.authoring.json exists for this artifact and the images verified, its visual_evidence is updated.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaSubmitVisualReview)),
		mcp.WithString("pptx_path", mcp.Required(), mcp.Description("Path to the reviewed PPTX file.")),
		mcp.WithString("pptx_revision", mcp.Required(), mcp.Description("sha256 (content_hash) of the PPTX that was reviewed; must match the current file.")),
		mcp.WithArray("slides", mcp.Required(), mcp.Description(`One entry per slide: [{"index":0,"verdict":"approved","image_path":"/tmp/thumbs/slide-1.png","findings":[]}, ...]. Every slide must be covered, and each image must be this server's render of that slide of this PPTX (from render_deck_thumbnails / render_slide_image) — a recycled or foreign image is rejected.`)),
		mcp.WithString("reviewer", mcp.Description(`Who reviewed: "host" (default, the calling agent) or "manual" (a human).`)),
		mcp.WithString("revision", mcp.Description("Optional semantic revision (render_deck_spec revision); when given it must match the deck's authoring manifest.")),
	)
}

// visualReviewSlideInput is one slide verdict submitted by the reviewer.
type visualReviewSlideInput struct {
	Index       *int               `json:"index"`
	Verdict     string             `json:"verdict"`
	Role        string             `json:"role,omitempty"`
	ImagePath   string             `json:"image_path,omitempty"`
	ImageSHA256 string             `json:"image_sha256,omitempty"`
	Findings    []visualqa.Finding `json:"findings,omitempty"`
}

type submitVisualReviewInput struct {
	PPTXPath     string
	PPTXRevision string
	Revision     string
	Reviewer     string
	Slides       []visualReviewSlideInput
}

type submitVisualReviewOutput struct {
	OK                bool                      `json:"ok"`
	Status            string                    `json:"status"`
	Verdict           string                    `json:"verdict"`
	Reviewer          string                    `json:"reviewer"`
	PPTXPath          string                    `json:"pptx_path"`
	ArtifactSHA256    string                    `json:"artifact_sha256"`
	Revision          string                    `json:"revision"`
	TotalSlides       int                       `json:"total_slides"`
	Evidence          *pipeline.QualityEvidence `json:"evidence"`
	Review            *visualqa.ReviewRecord    `json:"review"`
	ImageVerification *imageVerification        `json:"image_verification"`
	ManifestPath      string                    `json:"manifest_path,omitempty"`
	ManifestUpdated   bool                      `json:"manifest_updated"`
	Notes             []string                  `json:"notes,omitempty"`
}

// errVisualReviewRejected marks a review that failed completion validation
// (coverage, staleness, malformed slide entry) as opposed to an I/O failure.
var errVisualReviewRejected = errors.New("visual review rejected")

var slidePartRE = regexp.MustCompile(`^ppt/slides/slide[0-9]+\.xml$`)

// countPPTXSlides counts the slide parts in a PPTX package.
func countPPTXSlides(path string) (int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return 0, err
	}
	defer func() { _ = zr.Close() }()
	n := 0
	for _, f := range zr.File {
		if slidePartRE.MatchString(f.Name) {
			n++
		}
	}
	return n, nil
}

func deckVerdict(slides []visualReviewSlideInput) string {
	verdict := "approved"
	for _, s := range slides {
		switch s.Verdict {
		case "changes_requested":
			return "changes_requested"
		case "inconclusive":
			verdict = "inconclusive"
		}
		for _, f := range s.Findings {
			if f.Severity == visualqa.SeverityP0 || f.Severity == visualqa.SeverityP1 {
				return "changes_requested"
			}
		}
	}
	return verdict
}

// submitVisualReview validates a host/manual review against the current
// artifact and returns the resulting quality evidence. Validation failures
// wrap errVisualReviewRejected.
func submitVisualReview(in submitVisualReviewInput) (*submitVisualReviewOutput, error) {
	reviewer := in.Reviewer
	if reviewer == "" {
		reviewer = "host"
	}
	if reviewer != "host" && reviewer != "manual" {
		return nil, fmt.Errorf("%w: reviewer must be \"host\" or \"manual\", got %q", errVisualReviewRejected, reviewer)
	}
	artifact, err := describeArtifact(in.PPTXPath, "pptx")
	if err != nil {
		return nil, err
	}
	total, err := countPPTXSlides(in.PPTXPath)
	if err != nil {
		return nil, fmt.Errorf("read slides from %s: %w", in.PPTXPath, err)
	}

	// The current revision is the authoring manifest's revision when the deck
	// has one for this exact artifact, else the artifact hash itself.
	manifestPath := in.PPTXPath + ".authoring.json"
	manifest, _ := pipeline.ReadAuthoringManifest(manifestPath)
	if manifest != nil && manifest.PPTXSHA256 != artifact.SHA256 {
		manifest = nil // manifest describes an earlier artifact; do not bind to it
	}
	currentRevision := artifact.SHA256
	if manifest != nil {
		currentRevision = manifest.Revision
	}
	claimedRevision := in.Revision
	if claimedRevision == "" {
		claimedRevision = currentRevision
	}

	record := &visualqa.ReviewRecord{
		ArtifactSHA256: in.PPTXRevision,
		Revision:       claimedRevision,
		Backend:        reviewer,
		Verdict:        deckVerdict(in.Slides),
		Findings:       []visualqa.Finding{},
	}
	if err := appendReviewSlides(record, in.Slides, reviewer); err != nil {
		return nil, err
	}
	if verr := record.ValidateCompletion(artifact.SHA256, currentRevision, total); verr != nil {
		return nil, fmt.Errorf("%w: %v (current artifact sha256 %s, revision %s, %d slides)", errVisualReviewRejected, verr, artifact.SHA256, currentRevision, total)
	}

	// Bind the review to the artifact's own pixels: a well-formed review of
	// somebody else's images is exactly how the completion status was forged
	// (go-slide-creator-jltp).
	verification, verr := verifyReviewImages(record.Slides, artifact.SHA256, total)
	if verr != nil {
		return nil, verr
	}

	reviewed := make([]string, 0, total)
	for i := 0; i < total; i++ {
		reviewed = append(reviewed, fmt.Sprintf("slide-%d", i+1))
	}
	structuralValid := false
	if report, rerr := pptx.ValidateOutputFile(in.PPTXPath); rerr == nil {
		structuralValid = report.IsValid()
	}
	// The artifact is a json2pptx output identified by its hash; generation
	// always runs schema and fit checks before writing it.
	evidence := &pipeline.QualityEvidence{
		ArtifactSHA256:  artifact.SHA256,
		Revision:        currentRevision,
		SchemaValid:     true,
		Generated:       true,
		FitChecked:      true,
		StructuralValid: structuralValid,
		// PixelsRendered is a claim about THIS artifact: only a render this
		// server produced for it proves the reviewer saw its pixels.
		PixelsRendered:    verification.verified(),
		InspectionBackend: reviewer,
		ReviewedSlideIDs:  reviewed,
		TotalSlides:       total,
		VisualVerdict:     record.Verdict,
	}
	if !structuralValid {
		evidence.Reasons = append(evidence.Reasons, "artifact failed structural output validation")
	}
	if !verification.verified() {
		evidence.Reasons = append(evidence.Reasons, verification.Reasons...)
	}
	evidence.Finalize()

	out := &submitVisualReviewOutput{
		OK:                true,
		Status:            visualReviewDraftStatus,
		Verdict:           record.Verdict,
		Reviewer:          reviewer,
		PPTXPath:          in.PPTXPath,
		ArtifactSHA256:    artifact.SHA256,
		Revision:          currentRevision,
		TotalSlides:       total,
		Evidence:          evidence,
		Review:            record,
		ImageVerification: verification,
	}
	switch {
	case evidence.Approved:
		out.Status = visualReviewCompleteStatus
	case !verification.verified() && record.Verdict == "approved":
		// The review approves the deck but carries no verified evidence: record
		// it, and say so in the status rather than calling the deck done.
		out.Status = visualReviewUnverifiedStatus
		out.Notes = append(out.Notes, howToVerify)
	}

	// An unverified review never becomes durable evidence in the manifest.
	if manifest != nil && !verification.verified() {
		out.Notes = append(out.Notes, "authoring manifest not updated: the submitted images were not verified against this artifact's render")
		manifest = nil
	}
	if manifest != nil {
		manifest.VisualEvidence = &pipeline.VisualEvidence{
			ArtifactSHA256: artifact.SHA256,
			Reviewer:       reviewer,
			ReviewedSlides: reviewed,
			Verdict:        record.Verdict,
		}
		if werr := pipeline.WriteAuthoringManifest(manifestPath, manifest); werr != nil {
			out.Notes = append(out.Notes, fmt.Sprintf("authoring manifest not updated: %v", werr))
		} else {
			out.ManifestPath = manifestPath
			out.ManifestUpdated = true
		}
	}
	return out, nil
}

// appendReviewSlides converts the submitted slide verdicts into ReviewSlides
// (hashing image_path pixels) and collects their findings onto the record.
func appendReviewSlides(record *visualqa.ReviewRecord, slides []visualReviewSlideInput, reviewer string) error {
	for i, s := range slides {
		if s.Index == nil {
			return fmt.Errorf("%w: slides[%d].index is required", errVisualReviewRejected, i)
		}
		if s.Verdict != "approved" && s.Verdict != "changes_requested" && s.Verdict != "inconclusive" {
			return fmt.Errorf("%w: slides[%d].verdict must be approved, changes_requested, or inconclusive; got %q", errVisualReviewRejected, i, s.Verdict)
		}
		pixelHash := strings.ToLower(s.ImageSHA256)
		if s.ImagePath != "" {
			data, rerr := os.ReadFile(s.ImagePath) //nolint:gosec // reviewer-supplied rendered image path
			if rerr != nil {
				return fmt.Errorf("%w: slides[%d].image_path: %v", errVisualReviewRejected, i, rerr)
			}
			pixelHash = visualqa.PixelHash(data)
		}
		role := s.Role
		if role == "" {
			role = "slide"
		}
		record.Slides = append(record.Slides, visualqa.ReviewSlide{Index: *s.Index, Role: role, ImagePath: s.ImagePath, ImageSHA256: pixelHash})
		for _, f := range s.Findings {
			f.SlideIndex = *s.Index
			f.Source = reviewer
			record.Findings = append(record.Findings, f)
		}
	}
	return nil
}

func handleSubmitVisualReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	const tool = "submit_visual_review"
	path := request.GetString("pptx_path", "")
	if path == "" {
		return argRequired(request, tool, "pptx_path", "string", "/tmp/out/deck.pptx", nil), nil
	}
	if err := api.ValidatePptxPath(path); err != nil {
		return argInvalidValue(tool, diagnostics.CodeInvalidPath, "pptx_path", err.Error(), "string", "/tmp/out/deck.pptx", nil), nil
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return mcpFileNotFoundError(tool, "pptx_path", path), nil
	}
	rev := request.GetString("pptx_revision", "")
	if rev == "" {
		return argRequired(request, tool, "pptx_revision", "string", "<sha256 content_hash of the reviewed pptx>", nil), nil
	}
	rawSlides, ok := request.GetArguments()["slides"]
	if !ok || rawSlides == nil {
		return argRequired(request, tool, "slides", "array", []any{map[string]any{"index": 0, "verdict": "approved", "image_path": "/tmp/thumbs/slide-1.png"}}, nil), nil
	}
	encoded, err := json.Marshal(rawSlides)
	if err != nil {
		return argInvalidJSON("slides", err.Error(), "array", nil, nil), nil
	}
	var slides []visualReviewSlideInput
	if err := json.Unmarshal(encoded, &slides); err != nil {
		return argInvalidJSON("slides", fmt.Sprintf("slides must be an array of {index, verdict, image_path|image_sha256, findings?}: %v", err), "array", nil, nil), nil
	}

	out, err := submitVisualReview(submitVisualReviewInput{
		PPTXPath:     path,
		PPTXRevision: rev,
		Revision:     request.GetString("revision", ""),
		Reviewer:     request.GetString("reviewer", ""),
		Slides:       slides,
	})
	if err != nil {
		if errors.Is(err, errVisualReviewRejected) {
			return argInvalidValue(tool, "INVALID_PARAMETER", "slides", err.Error(), "array", nil, nil), nil
		}
		return api.MCPSimpleError("INTERNAL", err.Error()), nil
	}
	res, merr := api.MCPSuccessResult(ctx, out)
	if merr != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", merr)), nil
	}
	return res, nil
}
