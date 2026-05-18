package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	// Ensure all patterns are registered via init().
	_ "github.com/sebahrens/json2pptx/internal/patterns"
)

func scoreCandidatesMC(t *testing.T) *mcpConfig {
	t.Helper()
	return repairMC(t)
}

// minimalScoreDeck returns a deck with `count` simple title slides at layout
// slideLayout1 so the rhythm fingerprint is "title" for each.
func minimalScoreDeck(count int) map[string]any {
	slides := make([]map[string]any, count)
	for i := 0; i < count; i++ {
		slides[i] = map[string]any{
			"layout_id":  "slideLayout1",
			"slide_type": "title",
			"content": []map[string]any{
				{
					"placeholder_id": "title",
					"type":           "text",
					"text_value":     "Slide " + string(rune('A'+i)),
				},
			},
		}
	}
	return map[string]any{
		"template": "midnight-blue",
		"slides":   slides,
	}
}

func TestScoreCandidates_RanksByScore(t *testing.T) {
	mc := scoreCandidatesMC(t)

	deck := minimalScoreDeck(3)

	// Two candidates, both for slot 1. Candidate A is a content slide; B is a title
	// (matches the existing title runs in slot 0 + slot 2 → rhythm penalty).
	candidates := []any{
		// candidate 0 — content slide (no pattern run → no rhythm penalty)
		map[string]any{
			"layout_id":  "slideLayout2",
			"slide_type": "content",
			"content": []map[string]any{
				{"placeholder_id": "title", "type": "text", "text_value": "Section"},
			},
		},
		// candidate 1 — title slide (same as neighbors → run of 3 → -15)
		map[string]any{
			"layout_id":  "slideLayout1",
			"slide_type": "title",
			"content": []map[string]any{
				{"placeholder_id": "title", "type": "text", "text_value": "Another Title"},
			},
		},
	}

	result, err := mc.handleScoreCandidates(context.Background(), makeRequest(map[string]any{
		"presentation": deck,
		"slide_index":  float64(1),
		"candidates":   candidates,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out CandidateScoresResult
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if out.SlideIndex != 1 {
		t.Errorf("slide_index = %d, want 1", out.SlideIndex)
	}
	if out.ModeUsed != "deterministic" {
		t.Errorf("mode_used = %q, want %q", out.ModeUsed, "deterministic")
	}
	if len(out.Candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(out.Candidates))
	}

	// Ranks must be 1, 2 in order returned.
	if out.Candidates[0].Rank != 1 || out.Candidates[1].Rank != 2 {
		t.Errorf("ranks = [%d, %d], want [1, 2]",
			out.Candidates[0].Rank, out.Candidates[1].Rank)
	}

	// The content candidate (index 0) should rank ahead of the title candidate
	// (index 1) because the title candidate accrues a rhythm penalty.
	if out.Candidates[0].Index != 0 {
		t.Errorf("rank 1 candidate index = %d, want 0 (content slide)", out.Candidates[0].Index)
	}
	if out.Candidates[1].Index != 1 {
		t.Errorf("rank 2 candidate index = %d, want 1 (title slide)", out.Candidates[1].Index)
	}

	// The title candidate must show a rhythm penalty of 15 (run of 3).
	if out.Candidates[1].RhythmPenalty != 15 {
		t.Errorf("title candidate rhythm_penalty = %d, want 15",
			out.Candidates[1].RhythmPenalty)
	}
	if len(out.Candidates[1].Notes) == 0 {
		t.Error("title candidate should have rhythm notes")
	}
	// The content candidate must have no rhythm penalty.
	if out.Candidates[0].RhythmPenalty != 0 {
		t.Errorf("content candidate rhythm_penalty = %d, want 0",
			out.Candidates[0].RhythmPenalty)
	}
}

func TestScoreCandidates_ParseError(t *testing.T) {
	mc := scoreCandidatesMC(t)

	deck := minimalScoreDeck(2)

	// One valid, one structurally invalid candidate (unknown top-level key).
	candidates := []any{
		map[string]any{
			"layout_id":  "slideLayout1",
			"slide_type": "title",
			"content": []map[string]any{
				{"placeholder_id": "title", "type": "text", "text_value": "Hi"},
			},
		},
		map[string]any{
			// layout_id must be a string; the array form forces an unmarshal error.
			"layout_id": []any{"not", "a", "string"},
		},
	}

	result, err := mc.handleScoreCandidates(context.Background(), makeRequest(map[string]any{
		"presentation": deck,
		"slide_index":  float64(0),
		"candidates":   candidates,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", textContent(result))
	}

	var out CandidateScoresResult
	if err := json.Unmarshal([]byte(textContent(result)), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(out.Candidates) != 2 {
		t.Fatalf("got %d candidates, want 2", len(out.Candidates))
	}

	// Find the invalid candidate.
	var parseErr *CandidateScore
	for i := range out.Candidates {
		if out.Candidates[i].ParseError != "" {
			parseErr = &out.Candidates[i]
		}
	}
	if parseErr == nil {
		t.Fatalf("expected one candidate with parse_error, got none")
	}
	if parseErr.Score != 0 {
		t.Errorf("parse-error candidate score = %d, want 0", parseErr.Score)
	}
	if !strings.Contains(parseErr.ParseError, "invalid candidate JSON") {
		t.Errorf("parse_error = %q, want containing 'invalid candidate JSON'", parseErr.ParseError)
	}
}

func TestScoreCandidates_InvalidSlideIndex(t *testing.T) {
	mc := scoreCandidatesMC(t)

	deck := minimalScoreDeck(2)

	result, err := mc.handleScoreCandidates(context.Background(), makeRequest(map[string]any{
		"presentation": deck,
		"slide_index":  float64(5), // out of range
		"candidates":   []any{map[string]any{"layout_id": "slideLayout1"}},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for out-of-range slide_index, got success: %s", textContent(result))
	}
}

func TestScoreCandidates_EmptyCandidates(t *testing.T) {
	mc := scoreCandidatesMC(t)

	deck := minimalScoreDeck(1)

	result, err := mc.handleScoreCandidates(context.Background(), makeRequest(map[string]any{
		"presentation": deck,
		"slide_index":  float64(0),
		"candidates":   []any{},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected tool error for empty candidates, got success: %s", textContent(result))
	}
}

func TestScoreCandidates_RhythmPenaltyRunLengths(t *testing.T) {
	// Direct unit test for the rhythm penalty calculation.
	cases := []struct {
		name       string
		slides     []SlideInput
		idx        int
		wantPen    int
		wantNotes  bool
	}{
		{
			name:    "isolated_no_penalty",
			slides:  []SlideInput{{SlideType: "title"}, {SlideType: "content"}, {SlideType: "title"}},
			idx:     1,
			wantPen: 0,
		},
		{
			name:      "run_of_two",
			slides:    []SlideInput{{SlideType: "title"}, {SlideType: "title"}, {SlideType: "content"}},
			idx:       0,
			wantPen:   5,
			wantNotes: true,
		},
		{
			name:      "run_of_three",
			slides:    []SlideInput{{SlideType: "title"}, {SlideType: "title"}, {SlideType: "title"}},
			idx:       1,
			wantPen:   15,
			wantNotes: true,
		},
		{
			name:      "long_run",
			slides:    []SlideInput{{SlideType: "title"}, {SlideType: "title"}, {SlideType: "title"}, {SlideType: "title"}},
			idx:       2,
			wantPen:   15,
			wantNotes: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pen, notes := rhythmPenaltyAt(tc.slides, tc.idx)
			if pen != tc.wantPen {
				t.Errorf("rhythmPenaltyAt(%s) penalty = %d, want %d", tc.name, pen, tc.wantPen)
			}
			if tc.wantNotes && len(notes) == 0 {
				t.Errorf("rhythmPenaltyAt(%s) expected notes, got none", tc.name)
			}
			if !tc.wantNotes && len(notes) != 0 {
				t.Errorf("rhythmPenaltyAt(%s) expected no notes, got %v", tc.name, notes)
			}
		})
	}
}
