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
	"sync"
	"time"

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
	// visualReviewBlockedStatus is recorded when the visual review approves
	// the deck but the deterministic gate of the render that produced this
	// artifact did not pass (P0 content, gate failures): the deck is not done
	// however it looks (go-slide-creator-csclk.127).
	visualReviewBlockedStatus = "reviewed_deterministic_blockers"
)

// deterministicGates remembers, per artifact sha256, the deterministic
// blocking reasons of the render_deck_spec call that wrote that artifact, so
// submit_visual_review can refuse to call a deck with P0 blockers complete.
//
// It is bounded (go-slide-creator-tcxsq): it used to be a sync.Map that never
// evicted, growing with every render a long-lived MCP server performed.
// Entries expire after deterministicGateTTL (the render cache drops an
// artifact after 24h unused, so a review of it cannot arrive later), and at
// maxDeterministicGates passing gates are evicted before blocking ones: losing
// a passing record costs nothing (an unknown gate is simply not applied),
// while a blocking record is what stops a deck with P0 content from being
// called complete.
var deterministicGates = struct {
	mu      sync.Mutex
	entries map[string]deterministicGateEntry
	now     func() time.Time
}{entries: make(map[string]deterministicGateEntry), now: time.Now}

type deterministicGateEntry struct {
	reasons   []string
	expiresAt time.Time
}

const (
	maxDeterministicGates = 4096
	deterministicGateTTL  = 24 * time.Hour
)

// recordDeterministicGate records the deterministic gate outcome for the PPTX
// at pptxPath. A nil/empty reasons slice records a passing gate.
func recordDeterministicGate(pptxPath string, reasons []string) {
	artifact, err := describeArtifact(pptxPath, "pptx")
	if err != nil {
		return
	}
	storeDeterministicGate(artifact.SHA256, reasons)
}

func storeDeterministicGate(sha string, reasons []string) {
	g := &deterministicGates
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if _, exists := g.entries[sha]; !exists && len(g.entries) >= maxDeterministicGates {
		for k, e := range g.entries {
			if now.After(e.expiresAt) {
				delete(g.entries, k)
			}
		}
		// Still full: evict the soonest-expiring passing gate, and a blocking
		// gate only when no passing gate is left.
		for len(g.entries) >= maxDeterministicGates {
			victim, victimPassing := "", false
			var victimExp time.Time
			for k, e := range g.entries {
				passing := len(e.reasons) == 0
				if victim == "" || (passing && !victimPassing) || (passing == victimPassing && e.expiresAt.Before(victimExp)) {
					victim, victimPassing, victimExp = k, passing, e.expiresAt
				}
			}
			delete(g.entries, victim)
		}
	}
	g.entries[sha] = deterministicGateEntry{reasons: append([]string{}, reasons...), expiresAt: now.Add(deterministicGateTTL)}
}

// lookupDeterministicGate returns the recorded deterministic blocking reasons
// for an artifact and whether the gate outcome is known at all.
func lookupDeterministicGate(sha string) ([]string, bool) {
	g := &deterministicGates
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.entries[sha]
	if !ok || g.now().After(e.expiresAt) {
		return nil, false
	}
	return e.reasons, true
}

func mcpSubmitVisualReviewTool() mcp.Tool {
	return mcp.NewTool("submit_visual_review",
		mcp.WithDescription(`Record a host/manual visual review verdict for a rendered PPTX — the completion path when no vision provider (ANTHROPIC_API_KEY) is configured, or when you or a human inspected the rendered slides.

Inputs: pptx_path; pptx_revision (the sha256 content_hash of the exact PPTX reviewed, as the render response returned it); slides[] with EVERY slide exactly once: {index, verdict: approved|changes_requested|inconclusive, image_path or image_sha256, findings?}; reviewer "host" (default) or "manual"; optional semantic revision. Partial coverage or a stale revision is rejected (INVALID_PARAMETER) and nothing is recorded.

The images are evidence: each must be this server's render of that slide of this exact PPTX — submit the path / content_hash render_deck_thumbnails returned. Another slide's or deck's image is rejected, naming the slide it really is. With no server render to compare, the review is recorded as reviewed_unverified_images, never the completion status.

status is "visually_reviewed_current_revision" only when every slide is approved with no P0/P1 finding, output validation passes, the images verify, and (for a render_deck_spec artifact) the deterministic gate passed; otherwise "reviewed_deterministic_blockers" with blocking_reasons and publishable=false.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaSubmitVisualReview)),
		mcp.WithString("pptx_path", mcp.Required(), mcp.Description("Path to the reviewed PPTX file.")),
		mcp.WithString("pptx_revision", mcp.Required(), mcp.Description("sha256 (content_hash) of the PPTX that was reviewed; must match the current file.")),
		mcp.WithArray("slides", mcp.Required(),
			mcp.Description(`One entry per slide: [{"index":0,"verdict":"approved","image_path":"/tmp/thumbs/slide-1.png","findings":[]}, ...]. Every slide must be covered, and each entry must carry the image you inspected — image_path OR image_sha256, from render_deck_thumbnails. A submission without one is rejected: the image is the evidence. A recycled or foreign image is rejected too.`),
			mcp.Items(visualReviewSlideItemSchema())),
		mcp.WithString("reviewer", mcp.Description(`Who reviewed: "host" (default, the calling agent) or "manual" (a human).`)),
		mcp.WithString("revision", mcp.Description("Optional semantic revision (render_deck_spec revision); when given it must match the deck's authoring manifest.")),
	)
}

// visualReviewSlideItemSchema types one entry of the slides array. The
// parameter used to be an untyped array with a prose example, so an agent that
// read the schema and sent [{index, verdict}] for every slide was rejected with
// "slide 0 is missing role or pixel hash" — naming a field that appears in no
// schema, in SKILL.md or in get_started, and not naming the one it actually
// needed. Two failed round-trips to discover a required field
// (go-slide-creator-g2mc).
func visualReviewSlideItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"index": map[string]any{
				"type":        "integer",
				"minimum":     0,
				"description": "0-based slide index. Every slide of the deck must appear exactly once.",
			},
			"verdict": map[string]any{
				"type":        "string",
				"enum":        []any{"approved", "changes_requested", "inconclusive"},
				"description": "Your verdict for this slide.",
			},
			"image_path": map[string]any{
				"type":        "string",
				"description": "Path to the rendered PNG you inspected (render_deck_thumbnails slides[].path). Required unless image_sha256 is given.",
			},
			"image_sha256": map[string]any{
				"type":        "string",
				"description": "Pixel hash of the rendered PNG you inspected (the content_hash render_deck_thumbnails returns; SHA-256 over decoded pixels, PNG metadata ignored). Required unless image_path is given.",
			},
			"role": map[string]any{
				"type":        "string",
				"description": "Optional label for the slide's role in the deck. Defaults to \"slide\".",
			},
			"findings": map[string]any{
				"type":        "array",
				"description": "Optional per-slide findings: [{severity, category, description, location?, bbox?}].",
				"items":       map[string]any{"type": "object"},
			},
		},
		"required": []any{"index", "verdict"},
		"anyOf": []any{
			map[string]any{"required": []any{"image_path"}},
			map[string]any{"required": []any{"image_sha256"}},
		},
	}
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
	// Publishable is present when this server knows the deterministic gate of
	// the render that wrote the artifact: the gate passed AND the review is
	// complete (status visually_reviewed_current_revision).
	Publishable     *bool    `json:"publishable,omitempty"`
	BlockingReasons []string `json:"blocking_reasons,omitempty"`
	Notes           []string `json:"notes,omitempty"`
}

// errVisualReviewRejected marks a review that failed completion validation
// (coverage, staleness, malformed slide entry) as opposed to an I/O failure.
var errVisualReviewRejected = errors.New("visual review rejected")

// visualReviewRejection is a rejection that names the argument at fault. Every
// rejection used to be reported at path "slides", so a stale pptx_revision
// read as a slides problem and the agent re-sent the same stale hash
// (go-slide-creator-6p9mm). Details carries expected_revision /
// missing_slide_indices so the retry needs no second probe.
type visualReviewRejection struct {
	Path    string
	Message string
	Details map[string]any
}

func (r *visualReviewRejection) Error() string { return r.Message }

func (r *visualReviewRejection) Is(target error) bool { return target == errVisualReviewRejected }

func rejectReview(path string, details map[string]any, format string, args ...any) error {
	return &visualReviewRejection{Path: path, Message: fmt.Sprintf(format, args...), Details: details}
}

// checkReviewBinding reports the argument-specific rejections before the
// generic completion check: a stale pptx_revision / revision, and the slide
// indices the submission is missing, duplicates or has out of range.
func checkReviewBinding(in submitVisualReviewInput, artifactSHA, currentRevision string, total int) error {
	if in.PPTXRevision != artifactSHA {
		return rejectReview("pptx_revision", map[string]any{"expected_revision": artifactSHA},
			"pptx_revision %s is not the current file's sha256 %s: the PPTX changed since it was reviewed (or the hash is not its content_hash); re-render the thumbnails of the current file and resubmit with pptx_revision %s",
			in.PPTXRevision, artifactSHA, artifactSHA)
	}
	if in.Revision != "" && in.Revision != currentRevision {
		return rejectReview("revision", map[string]any{"expected_revision": currentRevision},
			"revision %s is not the deck's current revision %s; omit revision or send %s", in.Revision, currentRevision, currentRevision)
	}
	seen := make(map[int]int, len(in.Slides))
	for i, sl := range in.Slides {
		if sl.Index == nil {
			continue // appendReviewSlides names the missing index field
		}
		idx := *sl.Index
		if idx < 0 || idx >= total {
			return rejectReview(fmt.Sprintf("slides[%d].index", i), map[string]any{"slide_count": total},
				"slides[%d].index %d is outside the deck: valid 0-based indices are 0..%d", i, idx, total-1)
		}
		if prev, dup := seen[idx]; dup {
			return rejectReview(fmt.Sprintf("slides[%d].index", i), map[string]any{"duplicate_of": fmt.Sprintf("slides[%d]", prev)},
				"slides[%d].index %d duplicates slides[%d]: every slide must appear exactly once", i, idx, prev)
		}
		seen[idx] = i
	}
	var missing []int
	for idx := 0; idx < total; idx++ {
		if _, ok := seen[idx]; !ok {
			missing = append(missing, idx)
		}
	}
	if len(missing) > 0 {
		return rejectReview("slides", map[string]any{"missing_slide_indices": missing, "slide_count": total},
			"all-slide coverage required: the deck has %d slides (0-based 0..%d) and the review is missing %v", total, total-1, missing)
	}
	return nil
}

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
// currentReviewRevision returns the authoring manifest bound to this exact
// artifact (nil when it describes an earlier one) and the current revision:
// the manifest's revision when bound, else the artifact hash itself.
func currentReviewRevision(manifestPath, artifactSHA string) (*pipeline.AuthoringManifest, string) {
	manifest, _ := pipeline.ReadAuthoringManifest(manifestPath)
	if manifest == nil || manifest.PPTXSHA256 != artifactSHA {
		return nil, artifactSHA
	}
	return manifest, manifest.Revision
}

// applyDeterministicGate keeps a visual verdict from overriding the
// deterministic gate: a deck the render reported publishable:false for P0
// content is not complete however the slides look (go-slide-creator-csclk.127).
func applyDeterministicGate(out *submitVisualReviewOutput, artifactSHA string) {
	gateReasons, known := lookupDeterministicGate(artifactSHA)
	if !known {
		return
	}
	if len(gateReasons) > 0 {
		if out.Status == visualReviewCompleteStatus {
			out.Status = visualReviewBlockedStatus
		}
		out.BlockingReasons = gateReasons
	}
	publishable := out.Status == visualReviewCompleteStatus
	out.Publishable = &publishable
}

func submitVisualReview(in submitVisualReviewInput) (*submitVisualReviewOutput, error) {
	reviewer := in.Reviewer
	if reviewer == "" {
		reviewer = "host"
	}
	if reviewer != "host" && reviewer != "manual" {
		return nil, rejectReview("reviewer", nil, "reviewer must be \"host\" or \"manual\", got %q", reviewer)
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
	manifest, currentRevision := currentReviewRevision(manifestPath, artifact.SHA256)
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
	if err := checkReviewBinding(in, artifact.SHA256, currentRevision, total); err != nil {
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

	applyDeterministicGate(out, artifact.SHA256)

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
			return rejectReview(fmt.Sprintf("slides[%d].index", i), nil, "slides[%d].index is required (0-based)", i)
		}
		if s.Verdict != "approved" && s.Verdict != "changes_requested" && s.Verdict != "inconclusive" {
			return rejectReview(fmt.Sprintf("slides[%d].verdict", i), nil, "slides[%d].verdict must be approved, changes_requested, or inconclusive; got %q", i, s.Verdict)
		}
		if s.ImagePath == "" && s.ImageSHA256 == "" {
			// The image is the evidence: a verdict with no pixels behind it
			// cannot be verified against the artifact's own render, so it is
			// rejected here rather than three checks later under a message
			// that names a field the caller never had to supply.
			return rejectReview(fmt.Sprintf("slides[%d].image_sha256", i), nil, "slides[%d]: one of image_path or image_sha256 is required — submit the rendered PNG you inspected (render_deck_thumbnails returns both for every slide)", i)
		}
		pixelHash := strings.ToLower(s.ImageSHA256)
		if s.ImagePath != "" {
			data, rerr := os.ReadFile(s.ImagePath) //nolint:gosec // reviewer-supplied rendered image path
			if rerr != nil {
				return rejectReview(fmt.Sprintf("slides[%d].image_path", i), nil, "slides[%d].image_path: %v", i, rerr)
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
		var rejection *visualReviewRejection
		if errors.As(err, &rejection) {
			return visualReviewRejectionResult(rejection), nil
		}
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

// visualReviewRejectionResult reports a rejection at the argument it names,
// with its expected values in details.
func visualReviewRejectionResult(r *visualReviewRejection) *mcp.CallToolResult {
	expected := "array"
	switch {
	case r.Path == "pptx_revision" || r.Path == "revision" || r.Path == "reviewer":
		expected = "string"
	case strings.HasSuffix(r.Path, ".index"):
		expected = "integer"
	case strings.HasPrefix(r.Path, "slides["):
		expected = "string"
	}
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{{
		Code:         diagnostics.CodeInvalidParameter,
		Path:         r.Path,
		Message:      r.Message,
		Severity:     diagnostics.SeverityError,
		ExpectedType: expected,
		Details:      r.Details,
		NextToolCall: nextCallRetry("submit_visual_review", r.Path),
	}})
}
