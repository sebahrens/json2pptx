package semantic

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func TestRhythmMonotonyCountsTableContinuationOnce(t *testing.T) {
	table := func(title string) SlideSpec {
		return SlideSpec{Kind: KindTable, Body: map[string]any{"title": title, "headers": []any{"A", "B"}, "rows": []any{[]any{"One", "Two"}}}}
	}
	split := &DeckSpec{Slides: []SlideSpec{table("Ledger (1/2)"), table("Ledger (2/2)"), table("Margins")}}
	if rhythmCodes(Normalize(split).RhythmWarnings())[string(diagnostics.CodeSemanticRhythmMonotony)] {
		t.Fatal("one continued table plus one table became a monotony run")
	}
	separate := &DeckSpec{Slides: []SlideSpec{table("Ledger"), table("Margins"), table("Owners")}}
	if !rhythmCodes(Normalize(separate).RhythmWarnings())[string(diagnostics.CodeSemanticRhythmMonotony)] {
		t.Fatal("three separate tables stopped contributing to monotony")
	}
}
