package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-xx9i. A semantic DeckSpec unmarshals into a
// PresentationInput whose slides are all empty, so score_deck graded that
// emptiness and answered "overall_score 99, quality_gate PASS" for a deck it
// had never read. The composition score it reported (35 on a well-built
// eight-slide board update) described its own misreading — which would have
// become a confident WRONG refusal once composition joined the gate.

func TestLooksLikeDeckSpec(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		{"declared spec", `{"_kind":"spec","slides":[{"kind":"title","title":"T"}]}`, true},
		{"undeclared but kind-shaped", `{"slides":[{"kind":"title","title":"T"},{"kind":"closing","title":"Q"}]}`, true},
		{"raw deck with content", `{"slides":[{"content":[{"placeholder_id":"title","type":"text"}]}]}`, false},
		{"raw deck with shape_grid", `{"slides":[{"shape_grid":{"rows":[]}}]}`, false},
		{"raw deck with a pattern", `{"slides":[{"pattern":{"name":"kpi-3up"}}]}`, false},
		{"mixed: a kind slide that also has content is raw", `{"slides":[{"kind":"title","content":[{}]}]}`, false},
		{"no slides", `{"template":"midnight-blue"}`, false},
		{"empty slides", `{"slides":[]}`, false},
		{"not JSON", `not json`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := looksLikeDeckSpec([]byte(tt.doc)); got != tt.want {
				t.Errorf("looksLikeDeckSpec = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestScoreDeckRefusesADeckSpec checks the tool itself: rather than grading an
// empty deck, it says what happened and names the tools that read a spec.
func TestScoreDeckRefusesADeckSpec(t *testing.T) {
	spec := map[string]any{
		"_kind":    "spec",
		"template": "midnight-blue",
		"meta":     map[string]any{"title": "Board update"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Board update"},
			map[string]any{"kind": "closing", "title": "Questions?"},
		},
	}
	mc := semanticTestConfig(t)
	res, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{"presentation": spec}))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if !res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		t.Fatalf("score_deck graded a DeckSpec instead of refusing it: %s", string(b)[:400])
	}
	b, _ := json.Marshal(res.StructuredContent)
	for _, want := range []string{"DeckSpec", "validate_deck_spec", "compile_deck_spec"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("refusal does not mention %q: %s", want, string(b))
		}
	}
}

// TestScoreDeckStillGradesARawDeck is the guard against over-matching.
func TestScoreDeckStillGradesARawDeck(t *testing.T) {
	if testing.Short() {
		t.Skip("renders a deck")
	}
	deck := map[string]any{
		"template": "midnight-blue",
		"slides": []any{
			map[string]any{"slide_type": "title", "content": []any{
				map[string]any{"placeholder_id": "title", "type": "text", "text_value": "A real deck"},
			}},
		},
	}
	mc := semanticTestConfig(t)
	res, err := mc.handleScoreDeck(context.Background(), makeRequest(map[string]any{"presentation": deck}))
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res.IsError {
		b, _ := json.Marshal(res.StructuredContent)
		t.Fatalf("a raw deck was refused: %s", string(b))
	}
}
