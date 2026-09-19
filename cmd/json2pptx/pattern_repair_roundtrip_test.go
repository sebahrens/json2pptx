package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// go-slide-creator-qnrb: the visual-QA bbox hit test produced cell paths for
// pattern slides, and propose_repairs handed back a ready-made repair_slide
// args_template — which then returned {applied:false, "slide has no
// shape_grid"}. The repair loop dead-ended on exactly the slides the cell paths
// were built for.
func TestReduceCellTextResolvesPatternValues(t *testing.T) {
	deck := kpiPatternDeck(t, "Revenue across enterprise")

	result := applyRepairFix(&deck, 0, repairFixInput{Kind: "reduce_cell_text", Params: map[string]any{
		"cell_path": slidepath.GridCell(0, 0, 0),
		"max_chars": 20,
	}})
	if !result.Applied {
		t.Fatalf("not applied: code=%q message=%q", result.Code, result.Message)
	}
	var values []map[string]any
	if err := json.Unmarshal(deck.Slides[0].Pattern.Values, &values); err != nil {
		t.Fatal(err)
	}
	small, _ := values[0]["small"].(string)
	if len([]rune(small)) > 20 {
		t.Errorf("value not shortened: %q", small)
	}
	if !strings.HasSuffix(small, "…") {
		t.Errorf("shortened value should be marked as truncated: %q", small)
	}
	if values[1]["small"] != "Gross margin" {
		t.Errorf("a sibling cell's value was changed: %+v", values[1])
	}
	// The expanded grid is dropped so the next render re-expands the pattern.
	if deck.Slides[0].ShapeGrid != nil {
		t.Error("stale expanded grid left on the slide")
	}
}

// The pattern-rooted spelling of the same cell addresses the same value.
func TestReduceCellTextAcceptsPatternRootedPath(t *testing.T) {
	deck := kpiPatternDeck(t, "Revenue across enterprise")
	result := applyRepairFix(&deck, 0, repairFixInput{Kind: "reduce_cell_text", Params: map[string]any{
		"cell_path": "/slides/0/pattern/rows/0/cells/0",
		"max_chars": 20,
	}})
	if !result.Applied {
		t.Fatalf("not applied: code=%q message=%q", result.Code, result.Message)
	}
}

// Text composed at expansion cannot be traced to one value; the refusal names
// the directive that can edit it rather than failing blankly.
func TestReduceCellTextOnPatternRefusesComposedText(t *testing.T) {
	raw := `{"template":"midnight-blue","slides":[{"layout_id":"slideLayout2","pattern":{"name":"process-flow","values":{"steps":[
	  {"label":"Plan","description":"Scope the work"},
	  {"label":"Build","description":"Ship the thing"},
	  {"label":"Run","description":"Operate it"}]}}}]}`
	var deck PresentationInput
	if err := json.Unmarshal([]byte(raw), &deck); err != nil {
		t.Fatal(err)
	}
	grid := expandSlidePatternGrid(&deck.Slides[0], 0, 12192000, 6858000, nil)
	if grid == nil {
		t.Skip("pattern does not expand in this configuration")
	}
	result := applyRepairFix(&deck, 0, repairFixInput{Kind: "reduce_cell_text", Params: map[string]any{
		"cell_path": slidepath.GridCell(0, 0, 0),
		"max_chars": 4,
	}})
	if result.Applied {
		return // the label was traceable; shortening it is a fine outcome
	}
	if result.Message == "" {
		t.Errorf("a refusal must carry a reason: %+v", result)
	}
	if strings.Contains(result.Message, "no shape_grid") {
		t.Errorf("a pattern slide must never be refused for lacking a shape_grid: %q", result.Message)
	}
}

// The bead's acceptance test: every directive propose_repairs emits for a
// pattern slide must be accepted by repair_slide (applied, or refused with a
// code that names the alternative — never "slide has no shape_grid").
func TestProposeRepairsDirectivesApplyOnPatternSlides(t *testing.T) {
	deck := kpiPatternDeck(t, "Revenue across every enterprise segment")
	findings := []proposeRepairsFinding{{
		Code:    "fit_overflow",
		Path:    "/slides/0/pattern/rows/0/cells/0/shape/text",
		Action:  "refuse",
		Message: "text needs 120 chars; cell allows 40",
		Fix: &patterns.FixSuggestion{Kind: "reduce_cell_text", Params: map[string]any{
			"cell_path": slidepath.GridCell(0, 0, 0),
			"max_chars": 24,
		}},
	}}
	proposed := proposeRepairs(&deck, findings)
	if len(proposed.Slides) == 0 {
		t.Fatalf("no directives proposed: %+v", proposed)
	}
	for _, s := range proposed.Slides {
		for _, d := range s.Directives {
			fresh := kpiPatternDeck(t, "Revenue across every enterprise segment")
			res := applyRepairFix(&fresh, s.SlideIndex, repairFixInput{Kind: d.Kind, Params: d.Params})
			if strings.Contains(res.Message, "no shape_grid") {
				t.Errorf("directive %s dead-ends on a pattern slide: %q", d.Kind, res.Message)
			}
			if !res.Applied && res.Code == "" {
				t.Errorf("directive %s neither applied nor explained: %q", d.Kind, res.Message)
			}
		}
	}
}

func kpiPatternDeck(t *testing.T, label string) PresentationInput {
	t.Helper()
	values := []map[string]string{
		{"big": "$21M", "small": label},
		{"big": "68%", "small": "Gross margin"},
		{"big": "1,250", "small": "Customers"},
	}
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return PresentationInput{
		Template: "midnight-blue",
		Slides: []SlideInput{{
			LayoutID: "slideLayout2",
			Pattern:  &PatternInput{Name: "kpi-3up", Values: raw},
		}},
	}
}
