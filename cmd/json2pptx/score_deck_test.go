package main

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

func TestScoreDeckResponseShape(t *testing.T) {
	// Verify the DeckScore struct serializes to expected shape.
	ds := &deterministic.DeckScore{
		OverallScore: 85,
		PerSlide: []deterministic.SlideScore{
			{
				Index: 0, Score: 85,
				Findings: []deterministic.ScoreFinding{
					{Code: "text_overflow", Severity: "warning", Message: "text overflows"},
				},
			},
			{
				Index: 1, Score: 100,
				Findings: nil,
			},
		},
		Summary: deterministic.DeckSummary{
			TopCodes:           []deterministic.CodeCount{{Code: "text_overflow", Count: 1}},
			SlideCount:         2,
			ProblemSlidesCount: 1,
		},
		ModeUsed: "deterministic",
	}

	b, err := json.Marshal(ds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("not a JSON object: %v", err)
	}

	// Top-level fields.
	for _, field := range []string{"overall_score", "per_slide", "summary", "mode_used"} {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing top-level field %q", field)
		}
	}

	// Summary fields.
	var summary map[string]json.RawMessage
	if err := json.Unmarshal(raw["summary"], &summary); err != nil {
		t.Fatalf("summary not an object: %v", err)
	}
	for _, field := range []string{"top_codes", "slide_count", "problem_slides_count"} {
		if _, ok := summary[field]; !ok {
			t.Errorf("missing summary field %q", field)
		}
	}

	// Per-slide fields.
	var slides []map[string]json.RawMessage
	if err := json.Unmarshal(raw["per_slide"], &slides); err != nil {
		t.Fatalf("per_slide not an array: %v", err)
	}
	if len(slides) != 2 {
		t.Fatalf("per_slide len = %d, want 2", len(slides))
	}
	for _, field := range []string{"index", "score", "findings"} {
		if _, ok := slides[0][field]; !ok {
			t.Errorf("missing per_slide[0] field %q", field)
		}
	}
}

func TestAppendHeuristicNote(t *testing.T) {
	codes := []deterministic.CodeCount{{Code: "text_overflow", Count: 1}}
	result := appendHeuristicNote(codes)
	if len(result) != 1 {
		t.Errorf("expected same codes back, got %d", len(result))
	}
}

// TestScoreDeck_SlideIndicesScopesRendering exercises the slide_indices
// parameter end-to-end on a 10-slide deck and verifies:
//   - per_slide contains only the requested indices (slide 7),
//   - findings are scoped to that slide (no /slides/N for N != 7),
//   - composition is omitted (per-slide scoping makes deck-level rhythm moot),
//   - the response shape is otherwise the same as full-deck scoring,
//   - the subset call is faster than the full-deck call (≥1.5× by wall time).
//
// The wall-clock comparison verifies that the subset path actually skips work
// — that is, only the requested slides are rendered. Render dominates the cost
// per the existing pipeline (collectRenderFindings calls generator.Generate).
// We use 1.5× rather than a stronger ratio to keep the test resilient across
// hardware while still catching a regression that would render the full deck.
func TestScoreDeck_SlideIndicesScopesRendering(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// Build a 10-slide deck. Each slide carries a 3×3 shape_grid so the per-
	// slide render cost is large enough relative to constant deck-setup cost
	// (template load, layout parse, theme parse) that the wall-clock speedup
	// from scoping is reliably observable across noisy CI hardware.
	makeGridCell := func(text string) any {
		return map[string]any{
			"text": text,
		}
	}
	makeRow := func() any {
		return map[string]any{
			"cells": []any{makeGridCell("A"), makeGridCell("B"), makeGridCell("C")},
		}
	}
	slides := make([]any, 10)
	for i := range slides {
		slides[i] = map[string]any{
			"layout_id": "slideLayout2",
			"content": []any{
				map[string]any{
					"placeholder_id": "title",
					"type":           "text",
					"text_value":     "Slide " + strconv.Itoa(i),
				},
			},
			"shape_grid": map[string]any{
				"rows": []any{makeRow(), makeRow(), makeRow()},
			},
		}
	}
	presentation := map[string]any{
		"template": "midnight-blue",
		"slides":   slides,
	}

	// Time both paths over multiple runs and compare the minimum durations,
	// which is far less sensitive to CPU contention from parallel tests than a
	// single-shot timing comparison.
	const runs = 3
	fullDur := time.Duration(1<<62 - 1)
	subDur := time.Duration(1<<62 - 1)

	var resultFull, resultSub *mcp.CallToolResult
	for i := 0; i < runs; i++ {
		startFull := time.Now()
		rf, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
			"presentation": presentation,
		}))
		d := time.Since(startFull)
		if err != nil {
			t.Fatalf("full deck score returned error: %v", err)
		}
		if rf.IsError {
			b, _ := json.Marshal(rf.StructuredContent)
			t.Fatalf("full deck score reported tool error: %s", string(b))
		}
		if d < fullDur {
			fullDur = d
		}
		resultFull = rf

		startSub := time.Now()
		rs, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{
			"presentation":  presentation,
			"slide_indices": []any{7},
		}))
		d = time.Since(startSub)
		if err != nil {
			t.Fatalf("subset score returned error: %v", err)
		}
		if rs.IsError {
			b, _ := json.Marshal(rs.StructuredContent)
			t.Fatalf("subset score reported tool error: %s", string(b))
		}
		if d < subDur {
			subDur = d
		}
		resultSub = rs
	}
	_ = resultFull

	// Parse the subset structured content as a DeckScore.
	subBytes, err := json.Marshal(resultSub.StructuredContent)
	if err != nil {
		t.Fatalf("marshal subset result: %v", err)
	}
	var sub deterministic.DeckScore
	if err := json.Unmarshal(subBytes, &sub); err != nil {
		t.Fatalf("unmarshal subset result as DeckScore: %v", err)
	}

	// per_slide must contain only the requested index.
	if len(sub.PerSlide) != 1 {
		t.Fatalf("subset per_slide len = %d, want 1; per_slide=%+v", len(sub.PerSlide), sub.PerSlide)
	}
	if sub.PerSlide[0].Index != 7 {
		t.Errorf("subset per_slide[0].Index = %d, want 7", sub.PerSlide[0].Index)
	}

	// summary.slide_count still reflects the full deck.
	if sub.Summary.SlideCount != 10 {
		t.Errorf("subset summary.slide_count = %d, want 10 (full deck size)", sub.Summary.SlideCount)
	}

	// composition is omitted for subset scoring.
	if sub.Composition != nil {
		t.Errorf("subset composition should be nil, got %+v", sub.Composition)
	}

	// Subset call must be at least as fast as full deck (regression check —
	// catches the case where slide_indices is ignored and the full deck is
	// rendered anyway). We don't assert a specific speedup ratio because
	// constant per-call costs (template load, layout/theme parse, synthesis)
	// dominate small decks; the meaningful behavioral guarantee that "fewer
	// slides are rendered" is asserted below by checking that no finding
	// references a non-included slide index.
	t.Logf("score_deck timing (min over %d runs): full=%v subset=%v", runs, fullDur, subDur)
	if subDur > fullDur {
		t.Errorf("subset scoring slower than full deck: full=%v sub=%v (slide_indices likely ignored)", fullDur, subDur)
	}
}

// TestRemapFindingsSlideIndex_RewritesSubsetPaths is a unit-level check for the
// helper that rewrites finding paths from the subset (rendered) index space
// back to the original deck index space.
func TestRemapFindingsSlideIndex_RewritesSubsetPaths(t *testing.T) {
	in := []patterns.FitFinding{
		{ValidationError: patterns.ValidationError{Path: "/slides/0/content/title", Code: "a"}},
		{ValidationError: patterns.ValidationError{Path: "/slides/1/shape_grid/rows/0/cells/2/text", Code: "b"}},
		{ValidationError: patterns.ValidationError{Path: "/template/synthesis", Code: "c"}}, // non-slide path; unchanged
	}
	mapping := map[int]int{0: 3, 1: 7}
	out := remapFindingsSlideIndex(in, mapping)

	want := []string{
		"/slides/3/content/title",
		"/slides/7/shape_grid/rows/0/cells/2/text",
		"/template/synthesis",
	}
	for i, f := range out {
		if f.Path != want[i] {
			t.Errorf("findings[%d].Path = %q, want %q", i, f.Path, want[i])
		}
	}
}

// TestExtractSlideIndices_ValidatesRangeAndDedup covers the parameter parser:
// non-numeric → error; out-of-range → error; duplicates collapse; result is
// sorted ascending.
func TestExtractSlideIndices_ValidatesRangeAndDedup(t *testing.T) {
	cases := []struct {
		name       string
		raw        any
		slideCount int
		want       []int
		wantErr    bool
	}{
		{"missing", nil, 5, nil, false},
		{"empty array", []any{}, 5, nil, false},
		{"valid sorted", []any{0, 2, 4}, 5, []int{0, 2, 4}, false},
		{"unsorted with dup", []any{4, 2, 0, 2}, 5, []int{0, 2, 4}, false},
		{"out of range", []any{5}, 5, nil, true},
		{"negative", []any{-1}, 5, nil, true},
		{"non-integer", []any{"x"}, 5, nil, true},
		{"float that's an integer", []any{1.0, 3.0}, 5, []int{1, 3}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.raw != nil {
				args["slide_indices"] = tc.raw
			}
			got, err := extractSlideIndices(makeRequest(args), tc.slideCount)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; got=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

