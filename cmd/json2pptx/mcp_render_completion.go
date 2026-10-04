package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// Completion state shared by every render step (go-slide-creator-mn33v,
// go-slide-creator-z3pbp).
//
// generate_presentation used to answer only "was the file written?": success
// plus an input-heuristic score, with no publishable verdict and no next step,
// so a raw deck could be treated as done — and submit_visual_review applied the
// deterministic gate only to render_deck_spec artifacts, so a raw deck with P0
// content could be self-approved. Both render paths now carry the same
// completion block (deterministic_ready / publishable / blocking_reasons),
// record their gate for submit_visual_review, and chain forward with
// next_tool_call: render → render_deck_thumbnails → submit_visual_review.

// rawCompletionStatus derives the render_deck_spec completion block for a raw
// generate_presentation render. fitChecked reports whether the text-fit checks
// ran (strict_fit != off or fit_report=true); structural output validation
// counts only when it ran.
func rawCompletionStatus(fit []patterns.FitFinding, slideCount int, outputFindings []pptx.Finding, outputValidation, contentHash string, fitChecked bool) facadeStatus {
	diags := make([]semanticDiagnostic, 0, len(fit))
	for _, f := range fit {
		if f.Action == "refuse" {
			diags = append(diags, semanticDiagnostic{Code: f.Code, Action: f.Action, RawPath: f.Path})
		}
	}
	reasons := blockingDiagnosticReasons(diags)
	gatePassed := len(reasons) == 0
	gate := deterministic.EvaluateQualityGate(deterministic.ScoreFromFindings(fit, slideCount), fit, deterministic.DefaultQualityGateCriteria())
	if gate == nil || !gate.Passed {
		gatePassed = false
		if gate == nil || len(gate.Reasons) == 0 {
			reasons = append(reasons, "quality gate: failed")
		}
		if gate != nil {
			for _, r := range gate.Reasons {
				reasons = append(reasons, "quality gate: "+r)
			}
		}
	}
	structural := outputValidation != "off" && !hasBlockingOutputFinding(outputFindings)
	evidenceComplete := structural && fitChecked && contentHash != ""
	return deriveFacadeStatus(gatePassed, evidenceComplete, structural, false, reasons, contentProvenanceAuthorSupplied)
}

// nextCallRenderThumbnails is the step after a render: look at the slides.
// changed narrows a re-render to the slides a patch touched.
func nextCallRenderThumbnails(pptxPath string, changed []int) *patterns.ToolCallSuggestion {
	args := map[string]any{"pptx_path": pptxPath}
	if len(changed) > 0 {
		args["slide_indices"] = changed
	}
	return &patterns.ToolCallSuggestion{Tool: "render_deck_thumbnails", ArgsTemplate: args}
}

// nextCallSubmitVisualReview is the step after inspecting thumbnails: record
// the all-slide verdict against the exact artifact revision. The template
// names the verdict values: the tool is outside the default tools/list, so its
// schema is a separate call an agent otherwise has to make to learn three
// words (go-slide-creator-p8i1g).
func nextCallSubmitVisualReview(pptxPath, revision string) *patterns.ToolCallSuggestion {
	return &patterns.ToolCallSuggestion{Tool: "submit_visual_review", ArgsTemplate: map[string]any{
		"pptx_path":     pptxPath,
		"pptx_revision": revision,
		"slides": "<one {index, verdict, image_sha256} per slide: verdict = " + strings.Join(visualReviewVerdicts, " | ") +
			"; image_sha256 = slides[].content_hash from render_deck_thumbnails>",
	}}
}

// renderNextToolCall picks the step after a successful render: the first
// blocking finding's own fix when the deck is not deterministically ready,
// else the thumbnails that feed the visual review.
func renderNextToolCall(deterministicReady bool, blockingFix *patterns.ToolCallSuggestion, pptxPath string, changed []int) *patterns.ToolCallSuggestion {
	if !deterministicReady && blockingFix != nil {
		return blockingFix
	}
	return nextCallRenderThumbnails(pptxPath, changed)
}

// firstBlockingFitCall returns the next_tool_call of the first refuse-class
// fit finding that carries one.
func firstBlockingFitCall(fit []patterns.FitFinding) *patterns.ToolCallSuggestion {
	for _, f := range fit {
		if f.Action == "refuse" && f.NextToolCall != nil {
			return f.NextToolCall
		}
	}
	return nil
}

// firstBlockingSemanticCall is firstBlockingFitCall for render_deck_spec's
// semantic diagnostics.
func firstBlockingSemanticCall(ds []semanticDiagnostic) *patterns.ToolCallSuggestion {
	for _, d := range ds {
		if (d.Severity == "error" || d.Action == "refuse") && d.NextToolCall != nil {
			return d.NextToolCall
		}
	}
	return nil
}

// oneBasedSlidePrefix matches the "slide N:" / "slide N," lead-in raw findings
// use, where N is 1-based.
var oneBasedSlidePrefix = regexp.MustCompile(`^[Ss]lide (\d+)([:,])`)

// slideNumberMessage rewrites a raw finding's "slide N:" lead-in so N is the
// slide_number the DeckSpec surfaces report for the finding: 1-based, the one
// index meant for a person, and the same number in the message and the field
// (go-slide-creator-6p9mm, go-slide-creator-pilpn). rawIdx is the finding's
// own 0-based raw slide; a prefix naming any other slide is left alone.
// reportIdx is the 0-based deck position the diagnostic reports.
func slideNumberMessage(msg string, rawIdx, reportIdx int) string {
	m := oneBasedSlidePrefix.FindStringSubmatch(msg)
	if m == nil || rawIdx < 0 || reportIdx < 0 {
		return msg
	}
	if n, err := strconv.Atoi(m[1]); err != nil || n != rawIdx+1 {
		return msg
	}
	return fmt.Sprintf("slide %d%s", reportIdx+1, msg[len(m[0])-1:])
}
