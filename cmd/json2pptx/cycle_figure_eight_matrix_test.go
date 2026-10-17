package main

import (
	"context"
	"encoding/json"
	"fmt"

	_ "image/png"

	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// cycleFigureEightSplits lists the legal left_count values per phase count
// (each lobe holds 2-4 phases); 0 is the default split, half rounded up.
var cycleFigureEightSplits = map[int][]int{4: {0, 2}, 5: {0, 2, 3}, 6: {0, 2, 3, 4}, 7: {0, 3, 4}, 8: {0, 4}}

// cycleFigureEightDescBudget is the documented description budget: 60
// characters while each lobe holds at most three phases, 40 once a lobe holds
// four (docs/PATTERNS.md, the pattern's schema, internal/patterns
// cfeDescBudget). Labels hold the schema maximum, 26, at every count.
func cycleFigureEightDescBudget(n, left int) int {
	if left == 0 {
		left = (n + 1) / 2
	}
	if max(left, n-left) >= 4 {
		return 40
	}
	return 60
}

const cycleFigureEightLabelBudget = 26

// cycleFigureEightBudgetValues is a figure of n phases, left on the left
// lobe, every label and description exactly at its budget.
func cycleFigureEightBudgetValues(n, left int) *patterns.CycleFigureEightValues {
	v := &patterns.CycleFigureEightValues{LeftCount: left, LeftLabel: "Build", RightLabel: "Run"}
	desc := cycleFigureEightDescBudget(n, left)
	for i := 0; i < n; i++ {
		v.Phases = append(v.Phases, patterns.CycleRingPhase{Label: budgetProbeCopy(cycleFigureEightLabelBudget), Description: budgetProbeCopy(desc)})
	}
	return v
}

func cycleFigureEightDeck(t *testing.T, values any, overrides any) PresentationInput {
	t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	p := &PatternInput{Name: "cycle-figure-eight", Values: raw}
	if overrides != nil {
		if p.Overrides, err = json.Marshal(overrides); err != nil {
			t.Fatal(err)
		}
	}
	title := "Release is where the build loop hands over to the run loop"
	return PresentationInput{Slides: []SlideInput{{
		LayoutID: "content",
		Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		Pattern:  p,
	}}}
}

// Every phase count and every split, with every label and description at its
// documented budget, generates on every template (the local p-style included)
// with no refusal, no stored autofit shrink and no run under 12pt; so do a
// highlighted phase and the thin / thick bands.
func TestCycleFigureEightMatrixAtBudgetAcrossTemplates(t *testing.T) {
	templates := testutil.AllTestTemplateNames()
	counts := []int{4, 5, 6, 7, 8}
	if testing.Short() {
		templates, counts = []string{"abstract", "midnight-blue"}, []int{4, 8}
	}
	for _, tpl := range templates {
		for _, n := range counts {
			for _, left := range cycleFigureEightSplits[n] {
				t.Run(fmt.Sprintf("%s/%d/left%d", tpl, n, left), func(t *testing.T) {
					t.Parallel()
					values := cycleFigureEightBudgetValues(n, left)
					values.Phases[n-1].Highlight = true
					var overrides any
					if thickness := []string{"", "thin", "thick"}[(n+left)%3]; thickness != "" {
						overrides = map[string]string{"thickness": thickness}
					}
					path := generateForSweep(t, tpl, cycleFigureEightDeck(t, values, overrides))
					scales, err := storedFontScales(path)
					if err != nil {
						t.Fatal(err)
					}
					if len(scales) > 0 {
						t.Errorf("%d phases (left %d) at budget on %s store autofit shrink(s) %s", n, left, tpl, strings.Join(scales, ", "))
					}
					smallest := smallestGridRunPt(t, path)
					if smallest < 12 {
						t.Errorf("%d phases (left %d) on %s: smallest pattern run is %.1fpt, want >= 12", n, left, tpl, smallest)
					}
					t.Logf("%s n=%d left=%d: min stored size %.1fpt, no shrink", tpl, n, left, smallest)
				})
			}
		}
	}
}

func cycleFigureEightComposeDeck(tpl, direction string, figurePct float64) map[string]any {
	return map[string]any{"template": tpl, "slides": []any{map[string]any{
		"layout_id": "content",
		"content":   []any{map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Planning and delivery feed each other every quarter"}},
		"source":    "Illustrative",
		"compose": map[string]any{"direction": direction, "gap": 12, "segments": []any{
			map[string]any{"size_pct": figurePct, "pattern": map[string]any{"name": "cycle-figure-eight", "values": map[string]any{
				"left_label": "Plan", "right_label": "Deliver",
				"phases": []any{
					map[string]any{"label": "Set priorities"}, map[string]any{"label": "Fund the bets"},
					map[string]any{"label": "Ship increments"}, map[string]any{"label": "Measure outcomes"},
				},
			}}},
			map[string]any{"size_pct": 100 - figurePct, "pattern": map[string]any{"name": "kpi-inline", "values": []any{
				map[string]any{"big": "11d", "small": "Idea to learning"},
				map[string]any{"big": "38", "small": "Experiments a quarter"},
				map[string]any{"big": "1 in 4", "small": "Scaled to all users"},
			}}},
		}},
	}}}
}

// The figure needs width. A horizontal 50% compose segment is refused by
// validate_input and by generate_presentation with the same located
// fit_overflow finding, whose fix swaps to cycle-ring; a vertical 70% segment
// above a KPI strip validates and renders, on every template.
func TestCycleFigureEightComposeSegmentsAcrossTemplates(t *testing.T) {
	templates := testutil.AllTestTemplateNames()
	if testing.Short() {
		templates = []string{"abstract", "midnight-blue"}
	}
	mc := testMCPConfig(t)
	const wantSentence = "two lobes with a label column either side need a content area at least 580pt wide and 140pt tall"
	for _, tpl := range templates {
		t.Run(tpl+"/horizontal 50 is refused", func(t *testing.T) {
			deck := cycleFigureEightComposeDeck(tpl, "horizontal", 50)
			res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
			if err != nil {
				t.Fatal(err)
			}
			validateText := textContent(res)
			if !res.IsError {
				t.Fatalf("validate_input approved a figure eight in a 50%% segment: %.400s", validateText)
			}
			gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck}))
			if err != nil {
				t.Fatal(err)
			}
			genText := textContent(gen)
			if !gen.IsError {
				t.Fatalf("generate_presentation drew a figure eight in a 50%% segment: %.400s", genText)
			}
			for surface, text := range map[string]string{"validate_input": validateText, "generate_presentation": genText} {
				for _, want := range []string{wantSentence, "fit_overflow", "use cycle-ring", "/slides/0/compose/segments/0/pattern/values", `"swap_pattern"`, `"to":"cycle-ring"`} {
					if !strings.Contains(text, want) {
						t.Errorf("%s refusal does not carry %q: %.600s", surface, want, text)
					}
				}
			}
		})
		t.Run(tpl+"/vertical 70 renders", func(t *testing.T) {
			deck := cycleFigureEightComposeDeck(tpl, "vertical", 70)
			res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck, "fit_report": true}))
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				t.Fatalf("validate_input rejected a full-width figure eight: %.600s", textContent(res))
			}
			var out dryRunOutput
			if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil {
				t.Fatalf("validate_input response: %v", err)
			}
			if bad := blockingFindings(out.Findings); !out.Valid || len(bad) > 0 {
				t.Errorf("validate_input findings on the vertical segment:\n  %s", strings.Join(bad, "\n  "))
			}
			gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck}))
			if err != nil {
				t.Fatal(err)
			}
			var g JSONOutput
			if err := json.Unmarshal([]byte(textContent(gen)), &g); err != nil || gen.IsError || !g.Success {
				t.Fatalf("generate_presentation failed (%v): %.600s", err, textContent(gen))
			}
			if scales, err := storedFontScales(g.OutputPath); err != nil || len(scales) > 0 {
				t.Errorf("stored autofit shrink(s) %v (err %v)", scales, err)
			}
			// The figure's own text (the KPI strip below sets its captions smaller).
			texts := []string{"Set priorities", "Fund the bets", "Ship increments", "Measure outcomes", "Plan", "Deliver", "1", "4"}
			sizes := slideRunSizes(t, g.OutputPath, texts...)
			for i, text := range texts {
				if i >= len(sizes) || sizes[i] < 1200 {
					t.Errorf("%q is not written at 12pt or above (sizes %v)", text, sizes)
				}
			}
		})
	}
}

// Whatever rectangle the lobes' cells finally get, each lobe resolves to a
// square and both to the same size, side by side and never overlapping: here
// the expansion is nested in a grid of the default slide bounds, wider and
// taller than the area the pattern measured. (At the measured size the lobes'
// squares share an edge: internal/patterns
// TestFigureEightLobesMeetAtTheCrossing.)
func TestCycleFigureEightLobesStayRound(t *testing.T) {
	reg := patterns.Default()
	figure := PatternInput{Name: "cycle-figure-eight", Values: json.RawMessage(`{"phases":[{"label":"Plan"},{"label":"Code"},{"label":"Test"},{"label":"Release"},{"label":"Operate"},{"label":"Monitor"}]}`)}
	for _, tc := range []struct {
		name string
		w, h float64
	}{
		{"full slide", 828, 349},
		{"abstract", 687, 294},
		{"short band", 828, 200},
		{"70% segment", 630, 352},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := patterns.ExpandContext{LayoutBounds: patterns.LayoutBounds{Width: int64(tc.w * 12700), Height: int64(tc.h * 12700)}}
			p := figure
			grid, _, err := expandPattern(&p, ctx, reg)
			if err != nil {
				t.Fatal(err)
			}
			nested := &jsonschema.ShapeGridInput{
				Columns: json.RawMessage(`[100]`),
				Rows:    []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Grid: grid}}}},
			}
			res, err := resolveShapeGrid(nested, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
			if err != nil || res == nil {
				t.Fatalf("resolveShapeGrid: %v", err)
			}
			// Each lobe's blockArc segments share the lobe's own square.
			type square struct{ x, side int64 }
			var squares []square
			for _, c := range res.Cells {
				if c.Layer && c.ShapeSpec != nil && c.ShapeSpec.Geometry == "blockArc" {
					if d := c.Bounds.CX - c.Bounds.CY; d < -1 || d > 1 {
						t.Errorf("lobe segment resolves to %d x %d EMU, want a square", c.Bounds.CX, c.Bounds.CY)
					}
					squares = append(squares, square{c.Bounds.X, c.Bounds.CX})
				}
			}
			if len(squares) != 6 {
				t.Fatalf("%d segments resolved, want 6", len(squares))
			}
			left, right := squares[0], squares[len(squares)-1]
			if left.side != right.side {
				t.Errorf("lobes are %d and %d EMU wide, want equal", left.side, right.side)
			}
			if gap := right.x - (left.x + left.side); gap < 0 || gap > left.side/2 {
				t.Errorf("%.2fpt between the lobes, want them side by side without overlap", float64(gap)/12700)
			}
		})
	}
}

// The MCP journey: show_pattern states the budgets the pattern warns at and
// the width it needs, and expand_pattern takes every count and split.
func TestCycleFigureEightMCPShowAndExpand(t *testing.T) {
	result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{"name": "cycle-figure-eight"}))
	if err != nil || result.IsError {
		t.Fatalf("show_pattern failed: %v, %+v", err, result)
	}
	entry, ok := result.StructuredContent.(skillPatternFull)
	if !ok {
		t.Fatalf("show_pattern returned %T", result.StructuredContent)
	}
	schema := string(entry.Schema)
	for _, want := range []string{"60 characters while each lobe holds at most 3 phases, 40 once a lobe holds 4", "at least 580pt wide", `"left_count"`, `"left_label"`, `"right_label"`, `"highlight"`, `"thickness"`} {
		if !strings.Contains(schema, want) {
			t.Errorf("show_pattern schema does not state %q", want)
		}
	}

	mc := &mcpConfig{templatesDir: "../../templates"}
	for n := 4; n <= 8; n++ {
		for _, left := range cycleFigureEightSplits[n] {
			res, err := mc.handleExpandPattern(t.Context(), makeRequest(map[string]any{
				"name":      "cycle-figure-eight",
				"values":    cycleFigureEightBudgetValues(n, left),
				"overrides": map[string]any{"cell_accent_mode": "alternate"},
			}))
			if err != nil {
				t.Fatalf("expand_pattern(%d phases, left %d): %v", n, left, err)
			}
			if res.IsError {
				t.Fatalf("expand_pattern(%d phases, left %d) errored: %s", n, left, resultText(res))
			}
			text := resultText(res)
			if !strings.Contains(text, "blockArc") || !strings.Contains(text, "left-arm-out") || strings.Contains(text, patterns.ErrCodeBodyTooLong) {
				t.Errorf("expand_pattern(%d phases, left %d) = %.300s", n, left, text)
			}
		}
	}
	res, err := mc.handleExpandPattern(t.Context(), makeRequest(map[string]any{"name": "cycle-figure-eight", "values": map[string]any{
		"left_count": 2, "phases": cycleFigureEightBudgetValues(8, 0).Phases,
	}}))
	if err == nil && !res.IsError {
		t.Error("expand_pattern accepted left_count 2 with 8 phases (six on one lobe)")
	}
}
