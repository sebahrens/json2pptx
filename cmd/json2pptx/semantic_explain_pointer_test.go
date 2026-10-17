package main

import (
	"github.com/sebahrens/json2pptx/internal/semantic"
	"testing"
)

func TestExplainRhythmUsesFindingPointers(t *testing.T) {
	for _, structured := range []bool{false, true} {
		heavy := semantic.SlideSpec{Kind: semantic.KindOptionMatrix, Body: map[string]any{"title": "Board"}}
		spec := &semantic.DeckSpec{Meta: semantic.DeckMeta{Title: "Review"}, Slides: []semantic.SlideSpec{heavy, heavy, heavy}}
		if structured {
			spec.Structure = &semantic.DeckStructure{Sections: []semantic.DeckSection{{Title: "Decisions", Slides: spec.Slides}}}
			spec.Slides = nil
		}
		ir := semantic.Normalize(spec)
		internal, explain := ir.RhythmWarnings(), ir.Explain().RhythmWarnings
		if len(explain) == 0 || len(explain) != len(internal) {
			t.Fatalf("missing warnings: %+v", explain)
		}
		runs, insertions := 0, 0
		for i, warning := range explain {
			want := internal[i]
			if warning.Path != specPointer(want.Path) || warning.InsertBefore != specPointer(want.InsertBefore) {
				t.Errorf("pointer mismatch: %+v / %+v", warning, want)
			}
			for j, path := range warning.Run {
				runs++
				if path != specPointer(want.Run[j]) {
					t.Errorf("run pointer %q != finding %q", path, specPointer(want.Run[j]))
				}
			}
			if warning.InsertBefore != "" {
				insertions++
			}
			if len(warning.Run) > 0 && warning.SlideNumber < 1 {
				t.Errorf("missing human slide number: %+v", warning)
			}
		}
		if runs == 0 || insertions == 0 {
			t.Fatalf("fixture needs run and density insertion: %+v", explain)
		}
	}
}
