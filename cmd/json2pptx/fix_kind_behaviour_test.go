package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-ui4c: applying a kind that findings actually emit returned
// {applied:false, code:"kind_not_supported"}, which reads as "you made that up".
// A registered advisory kind now answers with the decision to make and the
// executable kinds that address the same defect.
func TestRepairAdvisoryFixKindAnswersWithGuidance(t *testing.T) {
	input := patternSlideInput("card-grid", map[string]any{"columns": 2, "rows": 1})

	result := applyRepairFix(&input, 0, repairFixInput{Kind: "add_detail_or_resize", Params: map[string]any{
		"slide_fill_pct": 22,
	}})
	if result.Applied {
		t.Fatal("an advisory kind must not claim to have edited the deck")
	}
	if result.Code != "advisory_fix_kind" {
		t.Errorf("code = %q, want advisory_fix_kind (kind_not_supported reads as a caller mistake)", result.Code)
	}
	if !strings.Contains(result.Message, "max_height_pct") {
		t.Errorf("message should carry the actual guidance, got %q", result.Message)
	}
	if len(result.Alternatives) == 0 {
		t.Fatal("an advisory answer must name executable alternatives")
	}
	for _, alt := range result.Alternatives {
		if !patterns.FixKindIsExecutable(alt) {
			t.Errorf("alternative %q is not executable", alt)
		}
	}
	if len(result.SupportedKinds) == 0 {
		t.Error("supported_kinds should still be present so the agent can recover in one hop")
	}
}

// A kind nobody emits is still a caller mistake and keeps the old code.
func TestRepairUnknownFixKindStaysUnsupported(t *testing.T) {
	input := patternSlideInput("card-grid", map[string]any{"columns": 2})
	result := applyRepairFix(&input, 0, repairFixInput{Kind: "make_it_prettier"})
	if result.Code != "kind_not_supported" {
		t.Errorf("code = %q, want kind_not_supported for an invented kind", result.Code)
	}
	if len(result.Alternatives) != 0 {
		t.Errorf("an unknown kind has no alternatives, got %v", result.Alternatives)
	}
}

// set_max_height_pct is the executable half of the underfill findings: the
// remediation steps already said "set max_height_pct to ~35", which named no
// directive an agent could send.
func TestRepairSetMaxHeightPct(t *testing.T) {
	input := patternSlideInput("process-flow", map[string]any{
		"steps": []any{map[string]any{"label": "Plan"}, map[string]any{"label": "Build"}, map[string]any{"label": "Ship"}},
	})
	// A pre-expanded grid must be dropped so the pipeline re-expands under the cap.
	input.Slides[0].ShapeGrid = &ShapeGridInput{}

	result := applyRepairFix(&input, 0, repairFixInput{Kind: "set_max_height_pct", Params: map[string]any{"max_height_pct": 35}})
	if !result.Applied {
		t.Fatalf("expected applied, got %q", result.Message)
	}
	if got := input.Slides[0].Pattern.MaxHeightPct; got != 35 {
		t.Errorf("max_height_pct = %v, want 35", got)
	}
	if input.Slides[0].ShapeGrid != nil {
		t.Error("the pre-expanded grid must be cleared so the cap takes effect")
	}

	// Re-capping reports both values so an agent sees what changed.
	again := applyRepairFix(&input, 0, repairFixInput{Kind: "set_max_height_pct", Params: map[string]any{"max_height_pct": 50}})
	if !again.Applied || !strings.Contains(again.Message, "35") {
		t.Errorf("re-cap message = %q", again.Message)
	}
}

func TestRepairSetMaxHeightPct_Rejections(t *testing.T) {
	cases := map[string]map[string]any{
		"missing": {},
		"zero":    {"max_height_pct": 0},
		"over":    {"max_height_pct": 140},
		"string":  {"max_height_pct": "35"},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			input := patternSlideInput("process-flow", map[string]any{"steps": []any{}})
			result := applyRepairFix(&input, 0, repairFixInput{Kind: "set_max_height_pct", Params: params})
			if result.Applied {
				t.Errorf("%s should not apply", name)
			}
			if result.Message == "" {
				t.Error("a refusal must say what is wrong")
			}
		})
	}
	// A slide without a pattern has no height budget to cap.
	raw := PresentationInput{Template: "midnight-blue", Slides: []SlideInput{{LayoutID: "slideLayout2"}}}
	result := applyRepairFix(&raw, 0, repairFixInput{Kind: "set_max_height_pct", Params: map[string]any{"max_height_pct": 35}})
	if result.Applied || !strings.Contains(result.Message, "no pattern") {
		t.Errorf("non-pattern slide: applied=%v message=%q", result.Applied, result.Message)
	}
}

// propose_repairs used to file advisory findings under unmapped[] as
// "fix_kind_not_repairable:<kind>" — the loop looked empty on exactly the decks
// that were nearly good.
func TestProposeRepairsBucketsAdvisoryFindings(t *testing.T) {
	findings := []proposeRepairsFinding{
		{
			Code:    "cell_underfilled",
			Path:    "/slides/0/shape_grid",
			Message: "the shape grid carries only 22% of its text capacity",
			Action:  "review",
			Fix:     &patterns.FixSuggestion{Kind: "add_detail_or_resize", Params: map[string]any{"slide_fill_pct": 22}},
		},
		{
			Code:    "fit_overflow",
			Path:    "/slides/0/content/0",
			Message: "text overflows its placeholder",
			Action:  "shrink_or_split",
			Fix:     &patterns.FixSuggestion{Kind: "reduce_text", Params: map[string]any{"max_length": 120}},
		},
		{
			Code:    "mystery",
			Path:    "/slides/0",
			Message: "something the registry has never heard of",
			Fix:     &patterns.FixSuggestion{Kind: "make_it_prettier"},
		},
	}
	input := patternSlideInput("card-grid", map[string]any{"columns": 2, "rows": 1})
	out := proposeRepairs(&input, findings)

	if len(out.Advisory) != 1 {
		t.Fatalf("advisory bucket = %+v, want the cell_underfilled finding", out.Advisory)
	}
	adv := out.Advisory[0]
	if adv.Kind != "add_detail_or_resize" || adv.Reason != "advisory_fix_kind:add_detail_or_resize" {
		t.Errorf("advisory entry = %+v", adv)
	}
	if adv.Guidance == "" || len(adv.Alternatives) == 0 {
		t.Errorf("advisory entry must carry guidance and alternatives: %+v", adv)
	}
	if adv.Params["slide_fill_pct"] == nil {
		t.Errorf("the finding's own params must survive: %+v", adv.Params)
	}
	if out.Summary.AdvisoryFindings != 1 {
		t.Errorf("summary.advisory_findings = %d, want 1", out.Summary.AdvisoryFindings)
	}

	// The unknown kind stays unmapped, and the executable one still maps.
	if len(out.Unmapped) != 1 || !strings.HasPrefix(out.Unmapped[0].Reason, "fix_kind_not_repairable:") {
		t.Errorf("unmapped = %+v, want only the unknown kind", out.Unmapped)
	}
	if out.Summary.MappedFindings != 1 || len(out.Slides) != 1 {
		t.Errorf("the executable finding must still produce a directive: mapped=%d slides=%+v", out.Summary.MappedFindings, out.Slides)
	}

	// The response must stay valid JSON for the declared schema.
	if _, err := json.Marshal(out); err != nil {
		t.Fatal(err)
	}
}
