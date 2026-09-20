package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// execSummarySlide builds an exec-summary slide with n points. The pattern
// requires 3–5, so a 4-point slide has no legal split at all.
func execSummarySlide(n int) *SlideInput {
	items := make([]map[string]string, n)
	for i := range items {
		items[i] = map[string]string{
			"lead":    fmt.Sprintf("Segment %d grew 12%%", i+1),
			"support": "Detail sentence.",
		}
	}
	vals, _ := json.Marshal(map[string]any{"points": items})
	return &SlideInput{
		SlideType: "content",
		Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Summary")}},
		Pattern:   &PatternInput{Name: "exec-summary", Values: vals},
	}
}

// cardGridSplitSlide builds a card-grid slide with n cells; card-grid accepts a
// wide range, so most split points are legal.
func cardGridSplitSlide(n int) *SlideInput {
	items := make([]map[string]string, n)
	for i := range items {
		items[i] = map[string]string{"header": fmt.Sprintf("Card %d", i+1), "body": fmt.Sprintf("Up %d%%", i+3)}
	}
	vals, _ := json.Marshal(map[string]any{"cells": items, "columns": 4, "rows": 2})
	return &SlideInput{
		SlideType: "content",
		Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Cards")}},
		Pattern:   &PatternInput{Name: "card-grid", Values: vals},
	}
}

// TestGuardProposesOnlyALegalSplit is go-slide-creator-qtjl: the refusal used
// to advertise split_pattern{first: keep} whatever the pattern's own minimum,
// so an agent following the next_tool_call hit a hard validation error.
func TestGuardProposesOnlyALegalSplit(t *testing.T) {
	t.Run("no legal split is not proposed", func(t *testing.T) {
		slide := execSummarySlide(4)
		got, blocked := guardDroppedItems("reduce_items", slide, 0, "points",
			[]any{map[string]any{"lead": "Segment 4 grew 12%"}}, 3, map[string]any{})
		if !blocked {
			t.Fatal("dropping a point carrying a figure must be guarded")
		}
		if got.NextToolCall != nil {
			t.Errorf("proposed a split the pattern rejects: %+v", got.NextToolCall.ArgsTemplate)
		}
		for _, want := range []string{"cannot be split", "minimum item count", "confirm_semantic_change"} {
			if !strings.Contains(got.Message, want) {
				t.Errorf("message %q does not mention %q", got.Message, want)
			}
		}
	})

	t.Run("a legal split is proposed with its point", func(t *testing.T) {
		slide := cardGridSplitSlide(8)
		got, blocked := guardDroppedItems("reduce_items", slide, 0, "cells",
			[]any{map[string]any{"header": "Card 7", "body": "Up 9%"}}, 6, map[string]any{})
		if !blocked || got.NextToolCall == nil {
			t.Fatalf("expected a guarded refusal with a proposal, got blocked=%v next=%v", blocked, got.NextToolCall)
		}
		fixes, _ := got.NextToolCall.ArgsTemplate["fixes"].([]any)
		if len(fixes) != 1 {
			t.Fatalf("expected one proposed fix, got %v", got.NextToolCall.ArgsTemplate)
		}
		params, _ := fixes[0].(map[string]any)["params"].(map[string]any)
		if params["first"] != 6 {
			t.Errorf("first = %v, want the caller's own point when it is legal", params["first"])
		}
	})
}

// TestProposedSplitIsActuallyApplicable closes the loop the bead measured:
// following the guard's own next_tool_call must succeed.
func TestProposedSplitIsActuallyApplicable(t *testing.T) {
	slide := cardGridSplitSlide(8)
	_, blocked := guardDroppedItems("reduce_items", slide, 0, "cells",
		[]any{map[string]any{"header": "Card 7", "body": "Up 9%"}}, 6, map[string]any{})
	if !blocked {
		t.Fatal("expected the guard to block")
	}
	split, ok := legalPatternSplit(slide, 0, "cells", 6)
	if !ok {
		t.Fatal("expected a legal split")
	}
	in := &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{*slide}}
	got := applySplitPattern(in, 0, map[string]any{"path": "cells", "first": split})
	if !got.Applied {
		t.Fatalf("the proposed split did not apply: %s", got.Message)
	}
	if len(in.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(in.Slides))
	}
	for i := range in.Slides {
		if err := validatePatternHalf(&in.Slides[i], i); err != nil {
			t.Errorf("slide %d of the applied split does not generate: %v", i, err)
		}
	}
}

// TestLegalPatternSplitFallsBackToBalanced: an impossible preferred point
// yields the most balanced legal one rather than nothing.
func TestLegalPatternSplitFallsBackToBalanced(t *testing.T) {
	slide := cardGridSplitSlide(8)
	got, ok := legalPatternSplit(slide, 0, "cells", 99)
	if !ok {
		t.Fatal("expected a fallback split")
	}
	if got != 4 {
		t.Errorf("fallback split = %d, want the balanced 4", got)
	}
}

// TestLegalPatternSplitLeavesInvalidValuesAlone: values that were already
// invalid are not the split's doing, and the pattern validator reports them.
func TestLegalPatternSplitLeavesInvalidValuesAlone(t *testing.T) {
	slide := execSummarySlide(9) // exec-summary accepts at most 5
	if _, ok := legalPatternSplit(slide, 0, "points", 4); ok {
		t.Error("an already-invalid slide should not yield a split proposal")
	}
}

// TestSplitPatternStillRefusesAnIllegalPoint keeps the apply-side guard: a
// caller may still ask for an impossible split directly.
func TestSplitPatternStillRefusesAnIllegalPoint(t *testing.T) {
	in := &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{*execSummarySlide(4)}}
	got := applySplitPattern(in, 0, map[string]any{"path": "points", "first": 3})
	if got.Applied {
		t.Fatal("splitting a 4-point exec-summary at 3 leaves a 1-point half the pattern rejects")
	}
	if got.Code != "semantic_review_required" {
		t.Errorf("code = %q, want semantic_review_required", got.Code)
	}
	if len(in.Slides) != 1 {
		t.Errorf("a refused split must not modify the deck (%d slides)", len(in.Slides))
	}
}
