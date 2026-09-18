package core

import (
	"errors"
	"strings"
	"testing"
)

// go-slide-creator-rrjj: an unknown chart type produced a bare
// `svggen: unknown diagram type "barchart"` with no allowed list and no
// suggestion, while every comparable error on the surface offers both.
func TestClosestType(t *testing.T) {
	candidates := []string{
		"bar_chart", "line_chart", "pie_chart", "stacked_bar_chart", "org_chart",
		"matrix_2x2", "venn", "waterfall", "bar", "line", "pie", "stacked_bar", "org",
	}

	tests := []struct {
		name   string
		target string
		want   string
	}{
		{"separator dropped", "barchart", "bar_chart"},
		{"space instead of underscore", "pie chart", "pie_chart"},
		{"separator dropped, compound", "stackedbar", "stacked_bar"},
		{"digits joined", "matrix2x2", "matrix_2x2"},
		{"case difference", "Bar_Chart", "bar_chart"},
		// Genuinely unregistered types must NOT draw a misleading suggestion.
		{"unregistered: sankey", "sankey", ""},
		{"unregistered: combo", "combo", ""},
		{"unregistered: choropleth", "choropleth", ""},
		{"nonsense", "zzzzqqq", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := closestType(tt.target, candidates); got != tt.want {
				t.Errorf("closestType(%q) = %q, want %q", tt.target, got, tt.want)
			}
		})
	}
}

// The suggestion must be deterministic across calls.
func TestClosestType_Deterministic(t *testing.T) {
	candidates := []string{"bar_chart", "bar", "barchart_alias", "line_chart"}
	first := closestType("barchar", candidates)
	for i := 0; i < 50; i++ {
		if got := closestType("barchar", candidates); got != first {
			t.Fatalf("call %d returned %q, first call returned %q", i, got, first)
		}
	}
}

// The error must carry the allowed vocabulary and, when there is one, the
// suggestion — and must remain matchable with errors.As.
func TestUnknownTypeError(t *testing.T) {
	e := &UnknownTypeError{
		Type:       "barchart",
		DidYouMean: "bar_chart",
		Allowed:    []string{"bar_chart", "line_chart"},
	}
	msg := e.Error()
	for _, want := range []string{`"barchart"`, `did you mean "bar_chart"?`, "allowed types:", "line_chart"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q: %s", want, msg)
		}
	}

	var target *UnknownTypeError
	if !errors.As(error(e), &target) {
		t.Error("UnknownTypeError should be matchable with errors.As")
	}

	// With no suggestion the message must still list the vocabulary.
	bare := (&UnknownTypeError{Type: "sankey", Allowed: []string{"bar_chart"}}).Error()
	if strings.Contains(bare, "did you mean") {
		t.Errorf("no suggestion should mean no did-you-mean clause: %s", bare)
	}
	if !strings.Contains(bare, "allowed types:") {
		t.Errorf("message must still list the vocabulary: %s", bare)
	}
}
