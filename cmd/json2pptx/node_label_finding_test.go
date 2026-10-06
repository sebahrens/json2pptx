package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// NODE_LABEL_TOO_LONG (go-slide-creator-ptf78): a label set inside a circle of
// cycle-nodes / radial-hub that does not fit it has its own finding code and
// an executable fix — remove overrides.labels — in place of the generic
// BODY_TOO_LONG.

const nodeLabelLoopSlide = `{"layout_id": "blank-title", "content": [
  {"placeholder_id": "title", "type": "text", "text_value": "The agent repeats five moves until the task is done"}],
  "pattern": {"name": "cycle-nodes", "overrides": {"labels": "inside", "arrows": "curved"},
    "values": {"steps": [{"label": "Sense"}, {"label": "WWWWWWWWWWWWWW"}, {"label": "Act"}, {"label": "Learn"}, {"label": "Share"}]}}}`

const nodeLabelHubSlide = `{"layout_id": "blank-title", "content": [
  {"placeholder_id": "title", "type": "text", "text_value": "Six functions draw on one platform"}],
  "pattern": {"name": "radial-hub", "overrides": {"labels": "inside"},
    "values": {"center": {"label": "Core"}, "spokes": [{"label": "Sales"}, {"label": "Infrastructure"},
      {"label": "Service"}, {"label": "Finance"}, {"label": "Risk"}, {"label": "Product"}]}}}`

func nodeLabelDeck() map[string]any {
	return map[string]any{"template": "abstract", "slides": []any{
		verdictJSON(verdictCover), verdictJSON(nodeLabelLoopSlide), verdictJSON(nodeLabelHubSlide),
	}}
}

// nodeLabelFindings are the NODE_LABEL_TOO_LONG findings of an answer as
// "path | action | repair tool | slide_index | fix kind | fix key", sorted. The
// findings envelope (validate) keeps path and action under evidence; the fit
// report (generate) has them on the finding. Both name the repair call.
func nodeLabelFindings(t *testing.T, surface string, findings []any) []string {
	t.Helper()
	var out []string
	for _, f := range answerFindingsWithCode(t, findings, patterns.ErrCodeNodeLabelTooLong) {
		if msg, _ := f["message"].(string); !strings.Contains(msg, "does not fit its") {
			t.Errorf("%s: message does not say what does not fit: %q", surface, msg)
		}
		path, action := f["path"], f["action"]
		if ev, ok := f["evidence"].(map[string]any); ok {
			path, action = ev["path"], ev["action"]
		}
		call, _ := f["next_tool_call"].(map[string]any)
		args, _ := call["args_template"].(map[string]any)
		fixes, _ := args["fixes"].([]any)
		if len(fixes) != 1 {
			t.Errorf("%s: next_tool_call carries %d fixes, want 1: %v", surface, len(fixes), call)
			continue
		}
		fix, _ := fixes[0].(map[string]any)
		params, _ := fix["params"].(map[string]any)
		out = append(out, fmt.Sprintf("%v | %v | %v | %v | %v | %v", path, action, call["tool"], args["slide_index"], fix["kind"], params["key"]))
	}
	sort.Strings(out)
	return out
}

func decodeAnswer(t *testing.T, surface, text string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("%s: answer is not JSON: %v\n%s", surface, err, text)
	}
	return doc
}

// validate's fit report and generate's report the finding identically: the
// same code at the value's own pointer, with the same executable fix, and no
// BODY_TOO_LONG beside it.
func TestNodeLabelTooLongReachesValidateAndGenerateAlike(t *testing.T) {
	mc := testMCPConfig(t)
	deck := nodeLabelDeck()
	want := []string{
		"/slides/1/pattern/values/steps/1/label | review | repair_slide | 1 | remove_key | labels",
		"/slides/2/pattern/values/spokes/1/label | review | repair_slide | 2 | remove_key | labels",
	}

	res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
	if err != nil {
		t.Fatal(err)
	}
	validated := answerFindings(t, decodeAnswer(t, "validate", textContent(res)))

	res, err = mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": deck, "fit_report": true, "output_dir": t.TempDir(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	generated := answerFindings(t, decodeAnswer(t, "generate_presentation", textContent(res)))

	for surface, findings := range map[string][]any{"validate": validated, "generate_presentation": generated} {
		if got := nodeLabelFindings(t, surface, findings); strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s: NODE_LABEL_TOO_LONG findings =\n  %s\nwant\n  %s", surface, strings.Join(got, "\n  "), strings.Join(want, "\n  "))
		}
		for _, f := range answerFindingsWithCode(t, findings, patterns.ErrCodeBodyTooLong) {
			if msg, _ := f["message"].(string); strings.Contains(msg, "does not fit its") {
				t.Errorf("%s: the inside label is still reported as BODY_TOO_LONG: %s", surface, msg)
			}
		}
	}
}

// expand_pattern's own diagnostics carry the code and the fix as well.
func TestNodeLabelTooLongInExpandPatternDiagnostics(t *testing.T) {
	ds := patternExpansionDiagnostics("cycle-nodes", patternExpansionResult{Warnings: []string{
		patterns.ErrCodeNodeLabelTooLong + `: cycle-nodes steps[1].label does not fit its 80pt node at 12pt on 2 lines`,
		patterns.ErrCodeBodyTooLong + `: cycle-nodes center.label does not fit the 60pt circle`,
	}})
	if len(ds) != 2 {
		t.Fatalf("diagnostics = %+v", ds)
	}
	if d := ds[0]; d.Code != patterns.ErrCodeNodeLabelTooLong || d.Fix == nil || d.Fix.Kind != "remove_key" || d.Fix.Params["key"] != "labels" {
		t.Errorf("NODE_LABEL_TOO_LONG diagnostic = %+v (fix %+v), want the remove_key fix", d, d.Fix)
	}
	if d := ds[1]; d.Fix != nil {
		t.Errorf("BODY_TOO_LONG diagnostic gained a fix: %+v", d.Fix)
	}
}

// describe_finding knows the code and names the fix.
func TestDescribeFindingKnowsNodeLabelTooLong(t *testing.T) {
	meta, ok := patterns.GetFindingMeta(patterns.ErrCodeNodeLabelTooLong)
	if !ok || meta.Severity != "review" || len(meta.RemediationSteps) == 0 {
		t.Fatalf("finding meta = %+v, ok=%t", meta, ok)
	}
	res, err := handleDescribeFinding(context.Background(), makeRequest(map[string]any{"code": patterns.ErrCodeNodeLabelTooLong}))
	if err != nil || res.IsError {
		t.Fatalf("describe_finding: %v %s", err, resultText(res))
	}
	wantAll(t, "describe_finding", structuredText(t, res), "inside", "remove_key", "overrides.labels", "outside the ring")
	if info, ok := patterns.FixKind(patterns.OutsideLabelsFix().Kind); !ok || info.Class != patterns.FixClassExecutable {
		t.Errorf("the fix kind %q is not an executable repair_slide kind: %+v", patterns.OutsideLabelsFix().Kind, info)
	}
}

// repair_slide applies the finding's fix as written: the labels override is
// gone, the labels stand outside the ring, and the finding does not come back.
func TestRepairSlideAppliesTheNodeLabelFix(t *testing.T) {
	mc := repairMC(t)
	deck := nodeLabelDeck()
	fix := patterns.OutsideLabelsFix()
	for slideIdx, name := range map[int]string{1: "cycle-nodes", 2: "radial-hub"} {
		res, err := mc.handleRepairSlide(context.Background(), makeRequest(map[string]any{
			"presentation": deck, "slide_index": float64(slideIdx),
			"fixes": []any{map[string]any{"kind": fix.Kind, "params": map[string]any(fix.Params)}},
		}))
		if err != nil {
			t.Fatal(err)
		}
		var out repairSlideOutput
		if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil {
			t.Fatalf("%s: unmarshal: %v\n%s", name, err, textContent(res))
		}
		if len(out.AppliedFixes) != 1 || !out.AppliedFixes[0].Applied {
			t.Fatalf("%s: fix not applied: %+v", name, out.AppliedFixes)
		}
		var patched PresentationInput
		if err := json.Unmarshal(out.PatchedDeck, &patched); err != nil {
			t.Fatal(err)
		}
		p := patched.Slides[slideIdx].Pattern
		if p == nil || p.Name != name {
			t.Fatalf("%s: patched slide lost its pattern: %+v", name, p)
		}
		if strings.Contains(string(p.Overrides), "labels") {
			t.Errorf("%s: overrides still set labels: %s", name, p.Overrides)
		}
		if name == "cycle-nodes" && !strings.Contains(string(p.Overrides), "arrows") {
			t.Errorf("%s: the fix removed more than the labels override: %s", name, p.Overrides)
		}
		raw, _ := json.Marshal(out.Findings.Findings)
		var after []any
		_ = json.Unmarshal(raw, &after)
		for _, f := range answerFindingsWithCode(t, after, patterns.ErrCodeNodeLabelTooLong) {
			if strings.HasPrefix(fmt.Sprint(f["path"]), fmt.Sprintf("/slides/%d/", slideIdx)) {
				t.Errorf("%s: the finding is still reported after the repair: %v", name, f["message"])
			}
		}
	}
}

// Inside a compose segment the finding keeps its code and names the segment,
// and carries no fix: repair_slide edits the slide's own pattern block.
func TestNodeLabelTooLongInAComposeSegment(t *testing.T) {
	var slide SlideInput
	if err := json.Unmarshal([]byte(`{"slide_type": "content", "layout_id": "blank-title", "compose": {"direction": "horizontal", "segments": [
	  {"size_pct": 50, "pattern": {"name": "cycle-nodes", "overrides": {"labels": "inside"},
	    "values": {"steps": [{"label": "Sense"}, {"label": "WWWWWWWWWWWWWW"}, {"label": "Act"}, {"label": "Learn"}]}}},
	  {"size_pct": 50, "pattern": {"name": "metric-list", "values": {"items": [
	    {"value": "38", "label": "Loops closed"}, {"value": "6 wk", "label": "Loop length"}, {"value": "4.2m", "label": "Savings"}]}}}]}}`), &slide); err != nil {
		t.Fatal(err)
	}
	layouts, width, height := schemaMaximaLayouts(t, "midnight-blue")
	deck := &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{slide}}
	found := false
	for _, f := range collectFitFindings(deck, layouts, width, height, nil) {
		if f.Code != patterns.ErrCodeNodeLabelTooLong {
			continue
		}
		found = true
		if f.Path != "/slides/0/compose" || f.SegmentIndex == nil || *f.SegmentIndex != 0 {
			t.Errorf("finding at %q segment %v, want /slides/0/compose segment 0", f.Path, f.SegmentIndex)
		}
		if f.Fix != nil {
			t.Errorf("a compose segment's finding carries a fix repair_slide cannot apply there: %+v", f.Fix)
		}
		if !strings.Contains(f.Message, "steps[1].label") {
			t.Errorf("message does not name the label: %s", f.Message)
		}
	}
	if !found {
		t.Error("no NODE_LABEL_TOO_LONG for an inside label that does not fit its node in a compose segment")
	}
}

// The finding weighs on the quality gate as the BODY_TOO_LONG it replaces: a
// substantive review defect.
func TestNodeLabelTooLongBlocksTheGateLikeBodyTooLong(t *testing.T) {
	for _, code := range []string{patterns.ErrCodeBodyTooLong, patterns.ErrCodeNodeLabelTooLong} {
		f := patterns.FitFinding{ValidationError: patterns.ValidationError{Code: code}, Action: "review"}
		if !deterministic.IsSubstantiveReview(f) {
			t.Errorf("%s at review is not a substantive review defect", code)
		}
	}
}
