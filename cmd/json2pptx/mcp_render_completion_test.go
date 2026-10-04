package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// go-slide-creator-fjuhm: the template argument replaces meta.template on
// compile_deck_spec and explain_deck_spec (and `semantic compile` / `semantic
// explain`) as it does on validate and render, and the response says so.
func TestTemplateArgumentWinsOnCompileAndExplain(t *testing.T) {
	spec := map[string]any{
		"meta":   map[string]any{"title": "Precedence", "template": "midnight-blue"},
		"slides": []any{map[string]any{"kind": "title", "title": "One template rule"}},
	}
	mc := semanticTestConfig(t)
	type answer struct {
		Template string   `json:"template"`
		Warnings []string `json:"warnings"`
	}
	check := func(label string, got answer, want string, warns bool) {
		t.Helper()
		if got.Template != want {
			t.Errorf("%s: template = %q, want %q", label, got.Template, want)
		}
		if warned := len(got.Warnings) == 1 && strings.Contains(got.Warnings[0], "overrides meta.template"); warned != warns {
			t.Errorf("%s: warnings = %v, want an override notice: %v", label, got.Warnings, warns)
		}
	}
	for _, tc := range []struct {
		arg, want string
		warns     bool
	}{{"", "midnight-blue", false}, {"forest-green", "forest-green", true}} {
		args := map[string]any{"spec": spec}
		if tc.arg != "" {
			args["template"] = tc.arg
		}
		var compiled, explained answer
		structuredInto(t, mustCall(t, handleCompileDeckSpec, args).StructuredContent, &compiled)
		check("compile_deck_spec "+tc.arg, compiled, tc.want, tc.warns)
		structuredInto(t, mustCall(t, mc.handleExplainDeckSpec, args).StructuredContent, &explained)
		check("explain_deck_spec "+tc.arg, explained, tc.want, tc.warns)
	}

	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"compile", "explain"} {
		stdout, stderr, code := cliRun(t, nil, "semantic", sub, "--spec", path, "--template", "forest-green")
		var got answer
		if err := json.Unmarshal([]byte(stdout), &got); err != nil || code != 0 {
			t.Fatalf("semantic %s exited %d (%v): %s", sub, code, err, stderr)
		}
		if got.Template != "forest-green" || !strings.Contains(stderr, "overrides meta.template") {
			t.Errorf("semantic %s: template = %q, stderr %q", sub, got.Template, stderr)
		}
	}
}

func TestTemplatePrecedenceWarning(t *testing.T) {
	// A template argument replaces the pin for the call and says how to keep it.
	over := resolveSpecTemplate("midnight-blue", "forest-green", "", specSource{})
	if over.Override != "forest-green" || over.Bind != "midnight-blue" || len(over.Warnings) != 1 ||
		!strings.Contains(over.Warnings[0], "forest-green") || !strings.Contains(over.Warnings[0], "/meta/template") {
		t.Errorf("override = %+v", over)
	}
	if same := resolveSpecTemplate("midnight-blue", "midnight-blue", "", specSource{}); same.Override != "" || len(same.Warnings) != 0 {
		t.Errorf("same template: %+v", same)
	}
	if unpinned := resolveSpecTemplate("", "forest-green", "", specSource{}); unpinned.Override != "" || len(unpinned.Warnings) != 0 {
		t.Errorf("unpinned spec: %+v", unpinned)
	}
	// A template file does not replace the pin.
	if w := templatePrecedenceWarning("midnight-blue", "", "brand.pptx"); !strings.Contains(w, "brand.pptx") || !strings.Contains(w, "was ignored") {
		t.Errorf("warning = %q", w)
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
// structural patches for lists. go-slide-creator-micna: the remediation holds
// the action and its budgets, the patch lives in next_tool_call alone, and a
// measurement that is no budget (threshold_pct) stays in the message.
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
	if title.Remediation == nil || title.Remediation.Primary.Action != diagnostics.ActionShortenText || len(title.Remediation.Primary.Params) != 1 || title.Remediation.Primary.Params["max_chars"] != 20 {
		t.Errorf("title remediation is not shorten_text {max_chars: 20}: %+v", title.Remediation.Primary)
	}
	rewrite, _ := title.NextToolCall.ArgsTemplate["patch"].([]any)
	if len(rewrite) != 1 || rewrite[0].(map[string]any)["path"] != "/slides/0/title" {
		t.Fatalf("title patch = %+v", rewrite)
	}
	if v, _ := rewrite[0].(map[string]any)["value"].(string); !strings.Contains(v, "20 characters") {
		t.Errorf("rewrite hint does not name the budget: %q", v)
	}
	if title.PatchVerified {
		t.Error("a patch the author has to complete is marked verified")
	}

	list := envelope.Findings[1]
	ops, _ := list.NextToolCall.ArgsTemplate["patch"].([]any)
	if len(ops) != 2 || ops[0].(map[string]any)["path"] != "/slides/0/bullets/3" || ops[1].(map[string]any)["op"] != "remove" {
		t.Errorf("list patch = %+v", ops)
	}

	if list.Remediation == nil || list.Remediation.Primary.Action != diagnostics.ActionApplyPatch || list.Remediation.Primary.Params["max_items"] != 2 {
		t.Errorf("list remediation = %+v, want apply_patch {max_items: 2}", list.Remediation)
	}

	other := envelope.Findings[2]
	if other.Remediation != nil || other.NextToolCall != nil {
		t.Errorf("a finding with no budget and no patch carries a remedy: %+v %+v", other.Remediation, other.NextToolCall)
	}
}
