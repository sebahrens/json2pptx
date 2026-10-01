package patterns

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// exemplarPlaceholderRE matches lorem-grade numbered placeholders such as
// "Card 1", "Description 3" or "Item 2" — copy that teaches agents to write
// terse, unrealistic content and makes plan_deck's fit prediction optimistic.
var exemplarPlaceholderRE = regexp.MustCompile(`(?i)\b(card|description|item|title|label|text|heading|header|body|point|option|column|row|cell|step|phase|feature|benefit|metric|value|stage|lorem)\s*\d+\b`)

// exemplarStrings collects every string leaf of an exemplar.
func exemplarStrings(v any, out *[]string) {
	switch t := v.(type) {
	case string:
		*out = append(*out, t)
	case []any:
		for _, e := range t {
			exemplarStrings(e, out)
		}
	case map[string]any:
		for _, e := range t {
			exemplarStrings(e, out)
		}
	}
}

// ExemplarValues are agent-visible (show_pattern example_values, plan_deck
// fit prediction), so none may carry numbered placeholder copy
// (go-slide-creator-swt4x).
func TestExemplarValuesHaveNoNumberedPlaceholders(t *testing.T) {
	for _, pat := range Default().List() {
		ex, ok := pat.(interface{ ExemplarValues() any })
		if !ok {
			continue
		}
		raw, err := json.Marshal(ex.ExemplarValues())
		if err != nil {
			t.Fatalf("%s: %v", pat.Name(), err)
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("%s: %v", pat.Name(), err)
		}
		var leaves []string
		exemplarStrings(decoded, &leaves)
		for _, s := range leaves {
			if m := exemplarPlaceholderRE.FindString(s); m != "" {
				t.Errorf("%s exemplar carries placeholder copy %q in %q", pat.Name(), m, s)
			}
		}
	}
}

// card-grid and comparison-2col cell bodies are realistic 1-2 line copy
// (at least six words), and every agenda-with-images row carries an
// image_label so the exemplar renders no empty placeholder tile
// (go-slide-creator-swt4x).
func TestExemplarCellBodiesAreRealistic(t *testing.T) {
	words := func(s string) int { return len(strings.Fields(s)) }
	cg := (&cardGrid{}).ExemplarValues().(*CardGridValues)
	for i, c := range cg.Cells {
		if words(c.Body) < 6 {
			t.Errorf("card-grid exemplar cells[%d].body %q has fewer than 6 words", i, c.Body)
		}
	}
	cmp := (&comparison2col{}).ExemplarValues().(*Comparison2colValues)
	for i, r := range cmp.Rows {
		if words(r.Left) < 6 || words(r.Right) < 6 {
			t.Errorf("comparison-2col exemplar rows[%d] = %q / %q, want >= 6 words each", i, r.Left, r.Right)
		}
	}
	awi := (&agendaWithImages{}).ExemplarValues().(*AgendaWithImagesValues)
	for i, it := range awi.Items {
		if strings.TrimSpace(it.ImageLabel) == "" {
			t.Errorf("agenda-with-images exemplar items[%d] has no image_label", i)
		}
	}
}
