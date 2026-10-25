package rhythm_test

import (
	"github.com/sebahrens/json2pptx/internal/rhythm"
	"testing"
)

func TestSecondaryAccentDoesNotPenalizeRestrainedDeck(t *testing.T) {
	slides := make([]rhythm.Slide, 10)
	for i := range slides {
		slides[i] = rhythm.Slide{ContentKinds: []string{"text"}, CellAccents: []string{"accent1"}}
	}
	before := rhythm.Analyze(slides).CompositionScore
	slides[9].CellAccents = []string{"accent2"}
	after := rhythm.Analyze(slides).CompositionScore
	if after != before {
		t.Fatalf("one secondary accent changed composition score: %d -> %d", before, after)
	}
}
