package svggen

import (
	"strings"
	"testing"
)

// The registry must build the error with its real vocabulary, including
// suggestions drawn from aliases.
func TestRegistryNewUnknownTypeError(t *testing.T) {
	err := DefaultRegistry().NewUnknownTypeError("barchart")
	if err.Type != "barchart" {
		t.Errorf("Type = %q", err.Type)
	}
	if len(err.Allowed) == 0 {
		t.Fatal("Allowed must list the registered types")
	}
	if err.DidYouMean != "bar_chart" {
		t.Errorf("DidYouMean = %q, want bar_chart", err.DidYouMean)
	}
	// Allowed must be sorted so the message is stable.
	for i := 1; i < len(err.Allowed); i++ {
		if err.Allowed[i-1] > err.Allowed[i] {
			t.Errorf("Allowed is not sorted at %d: %q > %q", i, err.Allowed[i-1], err.Allowed[i])
		}
	}
}

// End-to-end: rendering an unregistered type must fail with the structured
// error, and a near-miss must carry the suggestion.
func TestRender_UnknownTypeCarriesVocabulary(t *testing.T) {
	tests := []struct {
		typ            string
		wantSuggestion string
	}{
		{"barchart", "bar_chart"},
		{"pie chart", "pie_chart"},
		{"sankey", ""}, // genuinely unregistered: list, but do not guess
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			_, err := Render(&RequestEnvelope{Type: tt.typ, Data: map[string]any{"a": 1}})
			if err == nil {
				t.Fatalf("type %q should be rejected", tt.typ)
			}
			msg := err.Error()
			if !strings.Contains(msg, "allowed types:") {
				t.Errorf("error must list the allowed types: %s", msg)
			}
			if !strings.Contains(msg, "bar_chart") {
				t.Errorf("allowed list should include bar_chart: %s", msg)
			}
			if tt.wantSuggestion == "" {
				if strings.Contains(msg, "did you mean") {
					t.Errorf("an unregistered type must not draw a guess: %s", msg)
				}
				return
			}
			if !strings.Contains(msg, `did you mean "`+tt.wantSuggestion+`"?`) {
				t.Errorf("error should suggest %q: %s", tt.wantSuggestion, msg)
			}
		})
	}
}
