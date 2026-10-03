package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-6p9mm: a rejection names the argument at fault and carries
// the value the retry needs.
func TestSubmitVisualReview_RejectionNamesField(t *testing.T) {
	pptxPath, sha, imgs := renderReviewFixture(t)
	stubRenderCache(t, imgs)

	_, err := submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: "deadbeef", Slides: allSlides(imgs, "approved")})
	var rej *visualReviewRejection
	if !errors.As(err, &rej) || rej.Path != "pptx_revision" || rej.Details["expected_revision"] != sha {
		t.Fatalf("stale revision: got %+v", err)
	}

	_, err = submitVisualReview(submitVisualReviewInput{PPTXPath: pptxPath, PPTXRevision: sha, Slides: allSlides(imgs[:1], "approved")})
	if !errors.As(err, &rej) || rej.Path != "slides" {
		t.Fatalf("partial coverage: got %+v", err)
	}
	if missing, _ := rej.Details["missing_slide_indices"].([]int); len(missing) != 1 || missing[0] != 1 {
		t.Errorf("missing_slide_indices = %v, want [1]", rej.Details["missing_slide_indices"])
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pptx_path": pptxPath, "pptx_revision": "deadbeef", "slides": []any{
		map[string]any{"index": 0, "verdict": "approved", "image_path": imgs[0]},
		map[string]any{"index": 1, "verdict": "approved", "image_path": imgs[1]},
	}}
	res, _ := handleSubmitVisualReview(context.Background(), req)
	var env diagnostics.FindingEnvelope
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &env); err != nil || !res.IsError || len(env.Findings) != 1 {
		t.Fatalf("expected one error finding: %s", raw)
	}
	if got := env.Findings[0].Evidence["path"]; got != "pptx_revision" {
		t.Errorf("finding path = %v, want pptx_revision", got)
	}
	if got := env.Findings[0].Evidence["expected_revision"]; got != sha {
		t.Errorf("expected_revision = %v, want %s", got, sha)
	}
}

// go-slide-creator-mn33v: the raw path reports completion state, records its
// gate for submit_visual_review, and chains to the thumbnails.
func TestGenerateReportsCompletionState(t *testing.T) {
	mc := &mcpConfig{templatesDir: testTemplatesDir, outputDir: t.TempDir()}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"presentation": map[string]any{
		"template": "midnight-blue",
		"slides": []any{map[string]any{"layout_id": "title", "content": []any{
			map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Quarterly review"},
		}}},
	}}
	res, err := mc.handleGenerate(context.Background(), req)
	if err != nil || res.IsError {
		t.Fatalf("generate failed: %v %+v", err, res)
	}
	var out JSONOutput
	raw, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.DeterministicReady == nil || out.Publishable == nil || *out.Publishable {
		t.Fatalf("completion block missing or publishable on a fresh render: %s", raw)
	}
	if len(out.BlockingReasons) == 0 {
		t.Error("a fresh render must name the missing visual verdict in blocking_reasons")
	}
	if *out.DeterministicReady && (out.NextToolCall == nil || out.NextToolCall.Tool != "render_deck_thumbnails" || out.NextToolCall.ArgsTemplate["pptx_path"] != out.OutputPath) {
		t.Errorf("next_tool_call = %+v, want render_deck_thumbnails for %s", out.NextToolCall, out.OutputPath)
	}
	if _, known := lookupDeterministicGate(out.ContentHash); !known {
		t.Error("generate_presentation did not record its deterministic gate for submit_visual_review")
	}
}

func TestRenderNextToolCallChain(t *testing.T) {
	fix := &patterns.ToolCallSuggestion{Tool: "repair_slide", ArgsTemplate: map[string]any{}}
	if got := renderNextToolCall(false, fix, "/o/d.pptx", nil); got != fix {
		t.Errorf("blocked render should chain to the blocking fix, got %+v", got)
	}
	got := renderNextToolCall(true, fix, "/o/d.pptx", []int{2})
	if got.Tool != "render_deck_thumbnails" || got.ArgsTemplate["pptx_path"] != "/o/d.pptx" {
		t.Fatalf("ready render: got %+v", got)
	}
	if idx, _ := got.ArgsTemplate["slide_indices"].([]int); len(idx) != 1 || idx[0] != 2 {
		t.Errorf("slide_indices = %v, want [2]", got.ArgsTemplate["slide_indices"])
	}
	review := nextCallSubmitVisualReview("/o/d.pptx", "abc")
	if review.Tool != "submit_visual_review" || review.ArgsTemplate["pptx_revision"] != "abc" {
		t.Errorf("review call = %+v", review)
	}
}

// go-slide-creator-z3pbp: a likely typo retries repair_slide with the
// corrected kind instead of pointing at the vocabulary.
func TestUnknownRepairKindRetriesDidYouMean(t *testing.T) {
	applied := []appliedFix{unappliedFix("reduce_txt")}
	if applied[0].DidYouMean == "" {
		t.Skip("no close match for the typo")
	}
	retargetDidYouMeanRetries(applied, []repairFixInput{{Kind: "reduce_txt", Params: map[string]any{"max_items": 3}}}, 2, "deck_1")
	call := applied[0].NextToolCall
	if call == nil || call.Tool != "repair_slide" || call.ArgsTemplate["deck_id"] != "deck_1" || call.ArgsTemplate["slide_index"] != 2 {
		t.Fatalf("next_tool_call = %+v", call)
	}
	fix := call.ArgsTemplate["fixes"].([]any)[0].(map[string]any)
	if fix["kind"] != applied[0].DidYouMean || fix["params"] == nil {
		t.Errorf("retry fix = %+v", fix)
	}
}

func TestTemplatePrecedenceWarning(t *testing.T) {
	if w := templatePrecedenceWarning("midnight-blue", "forest-green", ""); !strings.Contains(w, "forest-green") || !strings.Contains(w, "/meta/template") {
		t.Errorf("warning = %q", w)
	}
	if w := templatePrecedenceWarning("midnight-blue", "midnight-blue", ""); w != "" {
		t.Errorf("same template warned: %q", w)
	}
	if w := templatePrecedenceWarning("", "forest-green", ""); w != "" {
		t.Errorf("unpinned spec warned: %q", w)
	}
}

// A message names a slide by its slide_number: the raw finding's own slide,
// renumbered to the deck position the diagnostic reports.
func TestSlideNumberMessage(t *testing.T) {
	if got := slideNumberMessage("slide 3: body text is long", 2, 2); got != "slide 3: body text is long" {
		t.Errorf("got %q", got)
	}
	if got := slideNumberMessage("slide 3: body text is long", 2, 4); got != "slide 5: body text is long" {
		t.Errorf("reported position not used: %q", got)
	}
	if got := slideNumberMessage("slide 5: other slide", 2, 2); got != "slide 5: other slide" {
		t.Errorf("mismatched slide rewritten: %q", got)
	}
}

// go-slide-creator-pi6ea: DeckSpec findings keep the raw fix budgets and emit
// structural patches for lists.
func TestSemanticizeFindingsKeepsFixParams(t *testing.T) {
	data := []byte(`{"meta":{"title":"T"},"slides":[{"kind":"content","title":"A long title","bullets":["a","b","c","d"]}]}`)
	envelope := diagnostics.FindingEnvelope{Findings: []diagnostics.Finding{
		{Code: "FIT.title_wrap", Evidence: map[string]any{"path": "slides[0].title"},
			Remediation: &diagnostics.Remediation{Primary: &diagnostics.RemediationAction{Action: diagnostics.ActionShortenText, Params: map[string]any{"path": "/slides/0/title", "max_chars": 20}}}},
		{Code: "FIT.too_many_items", Evidence: map[string]any{"path": "slides[0].bullets"},
			Remediation: &diagnostics.Remediation{Primary: &diagnostics.RemediationAction{Action: diagnostics.ActionApplyPatch, Params: map[string]any{"kind": "reduce_items", "max_items": 2}}}},
		{Code: "FIT.threshold", Evidence: map[string]any{"path": "slides[0].missing"},
			Remediation: &diagnostics.Remediation{Primary: &diagnostics.RemediationAction{Action: diagnostics.ActionApplyPatch, Params: map[string]any{"threshold_pct": 80}}}},
	}}
	semanticizeFindings(&envelope, data, "deck_1")

	title := envelope.Findings[0]
	if title.Remediation == nil || title.Remediation.Primary.Params["max_chars"] != 20 || title.Remediation.Primary.Params["path"] != "/slides/0/title" {
		t.Errorf("title remediation lost max_chars or kept the raw path: %+v", title.Remediation)
	}
	if v, _ := title.Remediation.Primary.Params["value"].(string); !strings.Contains(v, "20 characters") {
		t.Errorf("rewrite hint does not name the budget: %q", v)
	}

	list := envelope.Findings[1]
	ops, _ := list.NextToolCall.ArgsTemplate["patch"].([]any)
	if len(ops) != 2 || ops[0].(map[string]any)["path"] != "/slides/0/bullets/3" || ops[1].(map[string]any)["op"] != "remove" {
		t.Errorf("list patch = %+v", ops)
	}

	other := envelope.Findings[2]
	if other.Remediation == nil || other.Remediation.Primary.Params["threshold_pct"] != 80 {
		t.Errorf("unpatchable finding dropped its params: %+v", other.Remediation)
	}
}
