package main

import (
	"encoding/json"
	"testing"
)

// go-slide-creator-utej: list-truncating repairs must not silently delete
// authored facts.

// churnListInput is a VALID four-card grid whose last card carries a figure.
// The cells were bare strings and the grid dimensions absent, which card-grid
// rejects outright — so the guard was being asked to propose a split of a deck
// that could not generate in the first place, and the proposal it made was not
// testing anything the engine would accept (go-slide-creator-qtjl).
func churnListInput() PresentationInput {
	return patternSlideInput("card-grid", map[string]any{
		"cells": []any{
			map[string]any{"header": "Retention", "body": "Improved."},
			map[string]any{"header": "Onboarding", "body": "Redesigned."},
			map[string]any{"header": "Pricing", "body": "Simplified."},
			map[string]any{"header": "Churn", "body": "4% churn in SMB"},
		},
		"columns": 2,
		"rows":    2,
	})
}

func assertSplitProposal(t *testing.T, r appliedFix, wantFirst int) {
	t.Helper()
	if r.Applied {
		t.Fatal("expected the truncation to be refused")
	}
	if r.Code != "semantic_review_required" {
		t.Errorf("code = %q, want semantic_review_required", r.Code)
	}
	if r.NextToolCall == nil || r.NextToolCall.Tool != "repair_slide" {
		t.Fatalf("expected a repair_slide next_tool_call, got %+v", r.NextToolCall)
	}
	fixes, _ := r.NextToolCall.ArgsTemplate["fixes"].([]any)
	if len(fixes) != 1 {
		t.Fatalf("expected one proposed fix, got %v", r.NextToolCall.ArgsTemplate["fixes"])
	}
	fix := fixes[0].(map[string]any)
	if fix["kind"] != "split_pattern" {
		t.Errorf("proposed kind = %v, want split_pattern", fix["kind"])
	}
	params := fix["params"].(map[string]any)
	if params["path"] != "cells" || params["first"] != wantFirst {
		t.Errorf("proposed params = %v, want path=cells first=%d", params, wantFirst)
	}
}

func cellsOf(t *testing.T, s SlideInput) []any {
	t.Helper()
	var vals map[string]any
	if err := json.Unmarshal(s.Pattern.Values, &vals); err != nil {
		t.Fatal(err)
	}
	return vals["cells"].([]any)
}

func TestRepairReduceItems_RefusesDroppingProtectedFact(t *testing.T) {
	input := churnListInput()
	r := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_items", Params: map[string]any{"path": "cells", "max_items": float64(3)}})
	assertSplitProposal(t, r, 3)
	if got := len(cellsOf(t, input.Slides[0])); got != 4 {
		t.Errorf("refused repair mutated the list: %d items", got)
	}
}

func TestRepairResizeList_RefusesDroppingProtectedFact(t *testing.T) {
	input := churnListInput()
	r := applyRepairFix(&input, 0, repairFixInput{Kind: "resize_list", Params: map[string]any{"path": "cells", "count": float64(3)}})
	assertSplitProposal(t, r, 3)
}

func TestRepairReduceItems_ObjectItemsAndConfirm(t *testing.T) {
	input := patternSlideInput("card-grid", map[string]any{
		"cells": []any{
			map[string]any{"header": "Keep", "body": "plain prose"},
			map[string]any{"header": "Drop", "body": "not approved"},
		},
	})
	r := applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_items", Params: map[string]any{"path": "cells", "max_items": float64(1)}})
	if r.Applied || r.Code != "semantic_review_required" {
		t.Fatalf("negation inside an object item must be protected, got %+v", r)
	}
	r = applyRepairFix(&input, 0, repairFixInput{Kind: "reduce_items", Params: map[string]any{"path": "cells", "max_items": float64(1), "confirm_semantic_change": true}})
	if !r.Applied {
		t.Fatalf("confirm_semantic_change must bypass the guard: %s", r.Message)
	}
}

func TestRepairSplitPattern_SplitsPatternValues(t *testing.T) {
	input := churnListInput()
	r := applyRepairFix(&input, 0, repairFixInput{Kind: "split_pattern", Params: map[string]any{"path": "cells", "first": float64(3)}})
	if !r.Applied {
		t.Fatalf("split_pattern with path not applied: %s", r.Message)
	}
	if len(input.Slides) != 2 {
		t.Fatalf("slides = %d, want 2", len(input.Slides))
	}
	first, second := cellsOf(t, input.Slides[0]), cellsOf(t, input.Slides[1])
	if len(first) != 3 || len(second) != 1 {
		t.Fatalf("split = %v / %v", first, second)
	}
	// The card carrying the figure is the one that moved.
	moved, _ := second[0].(map[string]any)
	if moved["body"] != "4% churn in SMB" {
		t.Errorf("second slide holds %v, want the card with the figure", second[0])
	}
	if input.Slides[0].Pattern == input.Slides[1].Pattern {
		t.Error("split slides share one Pattern pointer")
	}

	// Nothing to move.
	one := patternSlideInput("card-grid", map[string]any{"cells": []any{"only"}})
	if r := applyRepairFix(&one, 0, repairFixInput{Kind: "split_pattern", Params: map[string]any{"path": "cells"}}); r.Applied {
		t.Error("split of a 1-item list must not apply")
	}
}
