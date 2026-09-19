package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// go-slide-creator-wmfo: auto_repair returned repairs_applied == [] on every
// calibration deck and every example deck, with no field saying "proposed N,
// applied 0". An agent calling the recommended one-call revise path got its deck
// back unchanged, a red gate, and no next step.
func TestApplyProposedRepairsReportsFailureReasons(t *testing.T) {
	deck := patternSlideInput("card-grid", map[string]any{"columns": 2, "rows": 1, "cells": []any{"A | a", "B | b"}})
	proposed := proposeRepairsOutput{
		Slides: []proposedSlideRepairs{{
			SlideIndex: 0,
			Directives: []proposedDirective{{
				Kind:   "reduce_cell_text",
				Params: map[string]any{"cell_path": "/slides/0/shape_grid/rows/9/cells/9", "max_chars": 20},
			}},
		}},
	}
	applied, failed := applyProposedRepairs(&deck, proposed)
	if len(applied) != 0 {
		t.Fatalf("expected no repair to land, got %v", applied)
	}
	if len(failed) != 1 {
		t.Fatalf("a refused directive must be reported, got %+v", failed)
	}
	if failed[0].Kind != "reduce_cell_text" || failed[0].Reason == "" {
		t.Errorf("failure entry = %+v, want the kind and the reason it gave", failed[0])
	}
}

// A slide with many independent cell directives must be repaired in ONE pass:
// one repair per slide meant twenty passes against a budget of three.
func TestApplyProposedRepairsRepairsEveryTargetInOnePass(t *testing.T) {
	cells := make([]any, 0, 4)
	for i := 0; i < 4; i++ {
		cells = append(cells, map[string]any{"shape": map[string]any{
			"geometry": "rect",
			"text":     strings.Repeat("a great deal of prose for one small cell to carry ", 3),
		}})
	}
	raw, _ := json.Marshal(map[string]any{
		"template": "midnight-blue",
		"slides": []any{map[string]any{
			"layout_id":  "slideLayout2",
			"shape_grid": map[string]any{"columns": 4, "rows": []any{map[string]any{"cells": cells}}},
		}},
	})
	var deck PresentationInput
	if err := json.Unmarshal(raw, &deck); err != nil {
		t.Fatal(err)
	}

	var directives []proposedDirective
	for c := 0; c < 4; c++ {
		directives = append(directives, proposedDirective{
			Kind: "reduce_cell_text",
			Params: map[string]any{
				"cell_path": slidePathGridCell(0, 0, c),
				"max_chars": 40,
			},
		})
	}
	applied, failed := applyProposedRepairs(&deck, proposeRepairsOutput{
		Slides: []proposedSlideRepairs{{SlideIndex: 0, Directives: directives}},
	})
	if len(applied) != 4 {
		t.Errorf("applied %d of 4 independent cell repairs in one pass: %v (failed: %+v)", len(applied), applied, failed)
	}
}

// Two directives aimed at the same element must not stack in one pass.
func TestApplyProposedRepairsSkipsRepeatedTargets(t *testing.T) {
	deck := patternSlideInput("card-grid", map[string]any{
		"columns": 1, "rows": 1,
		"cells": []any{"Header | " + strings.Repeat("body copy that runs long ", 6)},
	})
	deck.Slides[0].ShapeGrid = nil
	target := map[string]any{"path": "cells", "value": []any{"Header | short"}}
	applied, _ := applyProposedRepairs(&deck, proposeRepairsOutput{
		Slides: []proposedSlideRepairs{{SlideIndex: 0, Directives: []proposedDirective{
			{Kind: "replace_value", Params: target},
			{Kind: "replace_value", Params: target},
		}}},
	})
	if len(applied) != 1 {
		t.Errorf("same target applied %d times, want 1: %v", len(applied), applied)
	}
}

// split_pattern is the directive the fit report proposes for an overfull pattern
// slide, and it refused every time with "slide has no shape_grid to split"
// because findings describe the problem, not the field to split.
func TestSplitPatternInfersTheRepeatedField(t *testing.T) {
	cells := make([]any, 0, 12)
	for i := 0; i < 12; i++ {
		cells = append(cells, "Card | body copy")
	}
	deck := patternSlideInput("card-grid", map[string]any{"columns": 4, "rows": 3, "cells": cells})

	result := applyRepairFix(&deck, 0, repairFixInput{Kind: "split_pattern", Params: map[string]any{"first": 6}})
	if !result.Applied {
		t.Fatalf("split_pattern on a pattern slide not applied: %q", result.Message)
	}
	if len(deck.Slides) != 2 {
		t.Fatalf("expected 2 slides after the split, got %d", len(deck.Slides))
	}
	// Each half must satisfy the pattern's own contract (card-grid demands
	// exactly columns x rows cells), or the deck fails to generate.
	for i := range deck.Slides {
		if err := validatePatternHalf(&deck.Slides[i], i); err != nil {
			t.Errorf("half %d does not expand: %v", i, err)
		}
	}
}

// A split that cannot produce valid halves refuses instead of handing back a
// deck that fails to generate.
func TestSplitPatternRefusesWhenHalvesAreInvalid(t *testing.T) {
	cells := make([]any, 0, 5)
	for i := 0; i < 5; i++ {
		cells = append(cells, map[string]any{"label": "Step", "description": "does a thing"})
	}
	deck := patternSlideInput("numbered-step-strip", map[string]any{"steps": cells})
	if err := validatePatternHalf(&deck.Slides[0], 0); err != nil {
		t.Skipf("fixture does not expand: %v", err)
	}
	// numbered-step-strip requires at least 3 steps, so a 1/4 split cannot work.
	result := applyRepairFix(&deck, 0, repairFixInput{Kind: "split_pattern", Params: map[string]any{"first": 1}})
	if result.Applied {
		t.Fatalf("split into an invalid half was applied: deck now has %d slides", len(deck.Slides))
	}
	if !strings.Contains(result.Message, "rejects") {
		t.Errorf("refusal should explain that the pattern rejects the halves: %q", result.Message)
	}
	if len(deck.Slides) != 1 {
		t.Errorf("a refused split must leave the deck untouched, got %d slides", len(deck.Slides))
	}
}

func TestBalancedGridDims(t *testing.T) {
	for _, tc := range []struct{ n, cols, rows int }{
		{6, 3, 2}, {12, 4, 3}, {4, 2, 2}, {8, 4, 2}, {7, 7, 1}, {1, 1, 1},
	} {
		cols, rows := balancedGridDims(tc.n)
		if cols*rows != tc.n {
			t.Errorf("balancedGridDims(%d) = %dx%d, which is not %d cells", tc.n, cols, rows, tc.n)
		}
		if cols != tc.cols || rows != tc.rows {
			t.Errorf("balancedGridDims(%d) = %dx%d, want %dx%d", tc.n, cols, rows, tc.cols, tc.rows)
		}
	}
}

func TestLongestPatternValuesArrayRefusesTies(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"left":  []any{1, 2, 3},
		"right": []any{4, 5, 6},
	})
	if got := longestPatternValuesArray(&PatternInput{Name: "x", Values: raw}); got != "" {
		t.Errorf("tied arrays must not be split blind, got %q", got)
	}
	raw2, _ := json.Marshal(map[string]any{
		"title": "one",
		"items": []any{1, 2, 3, 4},
		"notes": []any{1, 2},
	})
	if got := longestPatternValuesArray(&PatternInput{Name: "x", Values: raw2}); got != "items" {
		t.Errorf("longest array = %q, want items", got)
	}
}

func slidePathGridCell(slide, row, cell int) string {
	return slidepath.GridCell(slide, row, cell)
}
