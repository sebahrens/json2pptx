package pipeline

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestAssignSectionNumbersOnlyForEmptySections(t *testing.T) {
	slides := []types.SlideDefinition{
		{Type: types.SlideTypeSection, Title: "First"},
		{Type: types.SlideTypeSection, Title: "Bullets", Content: types.SlideContent{Bullets: []string{"Authored"}}},
		{Type: types.SlideTypeSection, Title: "Table", Content: types.SlideContent{TableRaw: "| A | B |"}},
		{Type: types.SlideTypeSection, Title: "Parsed Table", Content: types.SlideContent{Table: &types.TableSpec{}}},
		{Type: types.SlideTypeSection, Title: "Diagram", Content: types.SlideContent{DiagramSpec: &types.DiagramSpec{Type: "bar_chart"}}},
		{Type: types.SlideTypeSection, Title: "Slots", Slots: map[int]*types.SlotContent{1: {Text: "Authored"}}},
		{Type: types.SlideTypeSection, Title: "Explicit", Content: types.SlideContent{Body: "Custom"}},
		{Type: types.SlideTypeContent, Title: "Ordinary"},
		{Type: types.SlideTypeSection, Title: "Last"},
	}
	assignSectionNumbers(slides)
	want := []string{"01", "", "", "", "", "", "Custom", "", "08"}
	wantAutomatic := []bool{true, false, false, false, false, false, false, false, true}
	for i, slide := range slides {
		if slide.Content.Body != want[i] {
			t.Errorf("slide %d body = %q, want %q", i, slide.Content.Body, want[i])
		}
		if slide.AutoSectionNumber != wantAutomatic[i] {
			t.Errorf("slide %d AutoSectionNumber = %v, want %v", i, slide.AutoSectionNumber, wantAutomatic[i])
		}
	}
}
