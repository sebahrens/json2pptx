package patterns

import (
	"encoding/json"
	"testing"
)

func TestActionRank(t *testing.T) {
	tests := []struct {
		action string
		want   int
	}{
		{"info", 0},
		{"review", 1},
		{"shrink_or_split", 2},
		{"refuse", 3},
		{"unknown", -1},
		{"", -1},
	}
	for _, tt := range tests {
		got := ActionRank(tt.action)
		if got != tt.want {
			t.Errorf("ActionRank(%q) = %d, want %d", tt.action, got, tt.want)
		}
	}
}

func TestActionRankOrdering(t *testing.T) {
	// Verify refuse > shrink_or_split > review > info.
	ordered := []string{"info", "review", "shrink_or_split", "refuse"}
	for i := 1; i < len(ordered); i++ {
		if ActionRank(ordered[i]) <= ActionRank(ordered[i-1]) {
			t.Errorf("ActionRank(%q) should be > ActionRank(%q)", ordered[i], ordered[i-1])
		}
	}
}

func TestFitFindingJSON(t *testing.T) {
	f := FitFinding{
		ValidationError: ValidationError{
			Pattern: "table",
			Path:    "slides[0].content.rows[3]",
			Code:    ErrCodeFitOverflow,
			Message: "row 3 overflows cell height",
			Fix:     &FixSuggestion{Kind: "split_at_row", Params: map[string]any{"row": 3}},
		},
		Action:        "shrink_or_split",
		Measured:      &Extent{WidthEMU: 914400, HeightEMU: 1828800},
		Allowed:       &Extent{WidthEMU: 914400, HeightEMU: 1371600},
		OverflowRatio: 1.33,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Embedded ValidationError fields must appear at top level (not nested).
	for _, key := range []string{"pattern", "path", "code", "message", "fix"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected top-level key %q from embedded ValidationError", key)
		}
	}

	// FitFinding-specific fields must also be present.
	for _, key := range []string{"action", "measured", "allowed", "overflow_ratio"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected top-level key %q from FitFinding", key)
		}
	}

	// Verify no nested "ValidationError" wrapper.
	if _, ok := m["ValidationError"]; ok {
		t.Error("embedded ValidationError should not appear as a named key")
	}
}

func TestFitFindingJSONOmitsNilExtents(t *testing.T) {
	f := FitFinding{
		ValidationError: ValidationError{
			Pattern: "shape_grid",
			Path:    "slides[1]",
			Code:    ErrCodeDensityExceeded,
			Message: "too dense",
		},
		Action: "review",
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := m["measured"]; ok {
		t.Error("nil Measured should be omitted from JSON")
	}
	if _, ok := m["allowed"]; ok {
		t.Error("nil Allowed should be omitted from JSON")
	}
	if _, ok := m["overflow_ratio"]; ok {
		t.Error("zero OverflowRatio should be omitted from JSON")
	}
}
