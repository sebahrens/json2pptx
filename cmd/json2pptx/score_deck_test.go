package main

import (
	"encoding/json"
	"testing"

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
