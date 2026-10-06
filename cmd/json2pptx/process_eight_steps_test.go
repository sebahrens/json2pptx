package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-pfyeg: an eight-step process could not be made clean on one
// slide. At the documented budget (71 characters a box) every box wrapped into
// a column of one- and two-word lines (TEXT_WRAPS_NARROW); cut to five words a
// box it drew OVERTALL_FLOW_LANE; one word a box, SLIDE_UNDERUSED as well.

// eightStepProcessSlides are process slides inside the budget list_slide_kinds
// states for the flow diagram: 3–8 steps, 80 characters a box.
func eightStepProcessSlides(t *testing.T) []any {
	t.Helper()
	// Label and description are joined with " — " in a flow box: each pair
	// below fills the 80 characters a box holds.
	full := []map[string]any{
		{"label": "Baseline", "description": "Tag every workload and agree the cost baseline with the finance team"},
		{"label": "Rightsize", "description": "Resize idle compute and remove the orphaned storage volumes we find"},
		{"label": "Commit", "description": "Buy savings plans for the steady base load of each business unit today"},
		{"label": "Automate", "description": "Schedule the non-production environments off outside office hours"},
		{"label": "Govern", "description": "Set budgets and alerts for every team with a named owner of the costs"},
		{"label": "Negotiate", "description": "Reopen the enterprise agreement with twelve months of usage facts"},
		{"label": "Track", "description": "Report the realised savings every month against the agreed baseline too"},
		{"label": "Embed", "description": "Make cloud cost a release criterion in each of the delivery teams now"},
	}
	steps := make([]any, len(full))
	for i, st := range full {
		if n := utf8.RuneCountInString(st["label"].(string) + " — " + st["description"].(string)); n < 76 || n > 80 {
			t.Fatalf("step %d reads %d characters in a flow box, want it at the budget of 80", i+1, n)
		}
		steps[i] = st
	}
	return []any{
		map[string]any{"kind": "process", "title": "Eight steps deliver the savings in 12 months", "takeaway": "Each step has an owner and a date.", "steps": steps},
		map[string]any{"kind": "process", "title": "Seven of them run before the first renewal date", "takeaway": "The last step runs for good.", "steps": steps[:7]},
		map[string]any{"kind": "process", "title": "A claim passes eight desks before it is paid", "takeaway": "Every desk adds a day.",
			"steps": []any{"Intake", "Triage", "Assess", "Verify", "Approve", "Schedule", "Pay", "Close"}},
		map[string]any{"kind": "process", "title": "Eight five-word steps take the claim to payment", "takeaway": "No step has two owners.",
			"steps": []any{"Tag workloads and agree baseline", "Resize idle compute and storage", "Buy plans for base load", "Switch test systems off nightly",
				"Set budgets and name owners", "Reopen the enterprise agreement now", "Report realised savings each month", "Make cost a release criterion"}},
	}
}

// TestEightStepProcessValidatesCleanOnOneSlide is the acceptance test: process
// slides of seven and eight steps, from one word a step to the full budget,
// draw no finding on any template in templates/ (the local p-style among them
// when it is there).
func TestEightStepProcessValidatesCleanOnOneSlide(t *testing.T) {
	mc := refusalTestConfig(t)
	templates := []string{"modern", "midnight-blue"}
	if !testing.Short() {
		templates = shippedTemplateNames(t)
	}
	slides := append([]any{map[string]any{"kind": "title", "title": "Cloud cost programme"}}, eightStepProcessSlides(t)...)
	spec := map[string]any{"meta": map[string]any{"title": "Cloud cost programme", "source": "Illustrative"}, "slides": slides}
	for _, tpl := range templates {
		t.Run(tpl, func(t *testing.T) {
			env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": tpl}))
			for _, f := range env.Findings {
				for _, p := range pathsOf(f) {
					// Deck-level advice (four process slides in a row, no summary)
					// and the cover are about this test's deck, not about a
					// process slide's layout.
					if !strings.HasPrefix(p, "/slides/") || strings.HasPrefix(p, "/slides/0/") || strings.Contains(f.Code, "MONOTONY") || strings.Contains(f.Code, "QUALITY_GATE") || strings.Contains(f.Code, "NO_EXECUTIVE_SUMMARY") {
						continue
					}
					t.Errorf("%s at %s: %s", f.Code, p, f.Message)
				}
			}
		})
	}
}

// A flow whose boxes do wrap names the steps behind them and says how many
// boxes a row holds.
func TestNarrowWrapNamesItsBoxes(t *testing.T) {
	label := "Tag every workload and agree the cost baseline with the finance team"
	steps := make([]any, 8)
	for i := range steps {
		steps[i] = map[string]any{"label": fmt.Sprintf("%s %d", label, i+1)}
	}
	// One row of eight (overrides.rows 1) is the layout that wraps.
	in := patternSlide(&PatternInput{
		Name:      "process-flow",
		Values:    json.RawMessage(mustJSON(t, map[string]any{"steps": steps})),
		Overrides: json.RawMessage(`{"rows":1}`),
	})
	narrow := findCode(collectGeometryFindings(&in, nil, 0, 0, nil), patterns.ErrCodeTextWrapsNarrow)
	if narrow == nil || narrow.Fix == nil {
		t.Fatal("eight 70-character boxes on one row drew no TEXT_WRAPS_NARROW")
	}
	boxes, _ := narrow.Fix.Params["max_boxes"].(int)
	if boxes < 1 || boxes >= 8 {
		t.Errorf("max_boxes = %v, want a row count below the eight boxes that wrap", narrow.Fix.Params["max_boxes"])
	}
	paths, _ := narrow.Fix.Params["paths"].([]string)
	if len(paths) != 8 {
		t.Fatalf("paths = %v, want one per box", paths)
	}
	for i, p := range paths {
		if want := fmt.Sprintf("/slides/0/pattern/values/steps/%d/label", i); p != want {
			t.Errorf("paths[%d] = %s, want %s", i, p, want)
		}
	}
	// The same steps on the default two rows read as labels.
	in.Slides[0].Pattern.Overrides = nil
	if f := findCode(collectGeometryFindings(&in, nil, 0, 0, nil), patterns.ErrCodeTextWrapsNarrow); f != nil {
		t.Errorf("two rows of four still wrap: %s", f.Message)
	}
}

// On a DeckSpec the boxes are the slide's steps: the finding sits on them.
func TestNarrowWrapOnDeckSpecListsTheSteps(t *testing.T) {
	mc := refusalTestConfig(t)
	described := eightStepProcessSlides(t)[0].(map[string]any)["steps"].([]any)[:6]
	spec := map[string]any{
		"meta": map[string]any{"title": "Cloud cost programme", "source": "Illustrative"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Cloud cost programme"},
			// Six described steps are numbered rows; pinned to the flow diagram
			// they are six narrow boxes.
			map[string]any{"kind": "process", "pattern": "process-flow", "title": "Six steps deliver the savings in 12 months", "takeaway": "Each step has an owner.", "steps": described},
		},
	}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "midnight-blue"}))
	for _, f := range env.Findings {
		if !strings.HasSuffix(f.Code, patterns.ErrCodeTextWrapsNarrow) {
			continue
		}
		// The interlocking arrows of the default look leave a label a little
		// more width than the boxes between connectors did, so not every one
		// of the six wraps narrowly; each path is still one of the steps.
		if len(f.Paths) < 3 || len(f.Paths) > 6 || f.Path == nil || *f.Path != f.Paths[0] {
			t.Errorf("paths = %v (path %v), want the steps that wrap", f.Paths, f.Path)
		}
		for _, p := range f.Paths {
			if n, ok := strings.CutPrefix(p, "/slides/1/steps/"); !ok || len(n) != 1 || n[0] < '0' || n[0] > '5' {
				t.Errorf("path %q is not one of the slide's six steps", p)
			}
		}
		boxes, _ := fixParams(f)["max_boxes"].(float64)
		if boxes < 1 || boxes >= 6 {
			t.Errorf("remediation params = %v, want a max_boxes below six", fixParams(f))
		}
		return
	}
	t.Fatal("six described steps pinned to process-flow drew no TEXT_WRAPS_NARROW")
}
