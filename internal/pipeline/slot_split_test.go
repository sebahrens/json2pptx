package pipeline

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// singlePlaceholderLayouts returns layouts where max content capacity is 1.
func singlePlaceholderLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{
			ID:   "title-layout",
			Name: "Title Slide",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
			},
		},
		{
			ID:   "content-layout",
			Name: "Content",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "body", Type: types.PlaceholderBody},
			},
		},
	}
}

// twoPlaceholderLayouts returns layouts where max content capacity is 2.
func twoPlaceholderLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{
			ID:   "content-layout",
			Name: "Content",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "body", Type: types.PlaceholderBody},
			},
		},
		{
			ID:   "two-col-layout",
			Name: "Two Column",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "body1", Type: types.PlaceholderBody},
				{ID: "body2", Type: types.PlaceholderBody},
			},
		},
	}
}

// chartPlaceholderLayouts returns layouts that include a dedicated chart placeholder.
func chartPlaceholderLayouts() []types.LayoutMetadata {
	return []types.LayoutMetadata{
		{
			ID:   "chart-layout",
			Name: "Chart",
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle},
				{ID: "body", Type: types.PlaceholderBody},
				{ID: "chart", Type: types.PlaceholderChart},
			},
		},
	}
}

func TestSplitContentOverflow_NoSplitNeeded(t *testing.T) {
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{Title: "Plain Content", Content: types.SlideContent{Body: "Hello"}},
		},
	}

	warnings := SplitContentOverflow(pres, singlePlaceholderLayouts())

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(pres.Slides) != 1 {
		t.Errorf("expected 1 slide, got %d", len(pres.Slides))
	}
}

func TestSplitContentOverflow_SlotFits(t *testing.T) {
	// 2 slots on a template that has a 2-placeholder layout: no split needed
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Title: "Two Columns",
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText, Text: "Left text"},
					2: {SlotNumber: 2, Type: types.SlotContentText, Text: "Right text"},
				},
			},
		},
	}

	warnings := SplitContentOverflow(pres, twoPlaceholderLayouts())

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(pres.Slides) != 1 {
		t.Errorf("expected 1 slide, got %d", len(pres.Slides))
	}
}

func TestSplitContentOverflow_SlotOverflow(t *testing.T) {
	// 2 slots on a template where max capacity is 1: should split into 2 slides
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Title:        "Revenue Analysis",
				SpeakerNotes: "Key revenue points",
				Source:       "Internal data",
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentBullets, Bullets: []string{"Revenue grew 15%", "Margins improved"}},
					2: {SlotNumber: 2, Type: types.SlotContentChart, DiagramSpec: &types.DiagramSpec{Type: "bar"}},
				},
			},
		},
	}

	warnings := SplitContentOverflow(pres, singlePlaceholderLayouts())

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides after split, got %d", len(pres.Slides))
	}

	// Page 1: text content (slot 1)
	page1 := pres.Slides[0]
	if page1.Title != "Revenue Analysis (1/2)" {
		t.Errorf("page1 title = %q, want %q", page1.Title, "Revenue Analysis (1/2)")
	}
	if len(page1.Slots) != 1 {
		t.Errorf("page1 should have 1 slot, got %d", len(page1.Slots))
	}
	if page1.Slots[1].Type != types.SlotContentBullets {
		t.Errorf("page1 slot1 type = %q, want bullets", page1.Slots[1].Type)
	}
	if page1.SpeakerNotes != "Key revenue points" {
		t.Errorf("page1 should have speaker notes")
	}
	if page1.Source != "Internal data" {
		t.Errorf("page1 should have source")
	}

	// Page 2: chart content (slot 2)
	page2 := pres.Slides[1]
	if page2.Title != "Revenue Analysis (2/2)" {
		t.Errorf("page2 title = %q, want %q", page2.Title, "Revenue Analysis (2/2)")
	}
	if len(page2.Slots) != 1 {
		t.Errorf("page2 should have 1 slot, got %d", len(page2.Slots))
	}
	if page2.Slots[1].Type != types.SlotContentChart {
		t.Errorf("page2 slot1 type = %q, want chart", page2.Slots[1].Type)
	}
	if page2.Slots[1].SlotNumber != 1 {
		t.Errorf("page2 slot should be renumbered to 1, got %d", page2.Slots[1].SlotNumber)
	}
	if page2.SpeakerNotes != "" {
		t.Errorf("page2 should not have speaker notes")
	}
}

func TestSplitContentOverflow_ThreeSlotsOnSinglePH(t *testing.T) {
	// 3 slots on a template where max capacity is 1: should split into 3 slides
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Title: "Triple Content",
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText, Text: "Part A"},
					2: {SlotNumber: 2, Type: types.SlotContentText, Text: "Part B"},
					3: {SlotNumber: 3, Type: types.SlotContentChart, DiagramSpec: &types.DiagramSpec{Type: "pie"}},
				},
			},
		},
	}

	warnings := SplitContentOverflow(pres, singlePlaceholderLayouts())

	if len(pres.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(pres.Slides))
	}
	if pres.Slides[0].Title != "Triple Content (1/3)" {
		t.Errorf("page1 title = %q", pres.Slides[0].Title)
	}
	if pres.Slides[2].Title != "Triple Content (3/3)" {
		t.Errorf("page3 title = %q", pres.Slides[2].Title)
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

func TestSplitContentOverflow_ChartBodyCollision(t *testing.T) {
	// Standard path: body text + chart, no chart placeholder → split
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Title: "Revenue Overview",
				Content: types.SlideContent{
					Body:        "Revenue grew 15% year-over-year.",
					DiagramSpec: &types.DiagramSpec{Type: "bar"},
				},
				SpeakerNotes: "Important chart",
			},
		},
	}

	warnings := SplitContentOverflow(pres, singlePlaceholderLayouts())

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// Page 1: text only
	page1 := pres.Slides[0]
	if page1.Content.Body != "Revenue grew 15% year-over-year." {
		t.Errorf("page1 should have body text")
	}
	if page1.Content.DiagramSpec != nil {
		t.Errorf("page1 should NOT have a diagram")
	}

	// Page 2: chart only
	page2 := pres.Slides[1]
	if page2.Content.DiagramSpec == nil {
		t.Errorf("page2 should have diagram")
	}
	if page2.Content.Body != "" {
		t.Errorf("page2 should NOT have body text")
	}
}

func TestSplitContentOverflow_ChartBodyWithChartPH(t *testing.T) {
	// Body + chart, but template HAS a chart placeholder → no split
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Title: "Revenue Overview",
				Content: types.SlideContent{
					Body:        "Revenue grew 15% year-over-year.",
					DiagramSpec: &types.DiagramSpec{Type: "bar"},
				},
			},
		},
	}

	warnings := SplitContentOverflow(pres, chartPlaceholderLayouts())

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(pres.Slides) != 1 {
		t.Errorf("expected 1 slide, got %d", len(pres.Slides))
	}
}

func TestSplitContentOverflow_SlideIndexReindexed(t *testing.T) {
	// Verify that slide indices are correctly re-numbered after split
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{Index: 0, Title: "Intro", Content: types.SlideContent{Body: "Hello"}},
			{
				Index: 1,
				Title: "Split Me",
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText, Text: "A"},
					2: {SlotNumber: 2, Type: types.SlotContentText, Text: "B"},
				},
			},
			{Index: 2, Title: "Outro", Content: types.SlideContent{Body: "Bye"}},
		},
	}

	SplitContentOverflow(pres, singlePlaceholderLayouts())

	if len(pres.Slides) != 4 {
		t.Fatalf("expected 4 slides, got %d", len(pres.Slides))
	}

	for i, slide := range pres.Slides {
		if slide.Index != i {
			t.Errorf("slide %d has index %d, want %d", i, slide.Index, i)
		}
	}
}

func TestSplitContentOverflow_ChartOnlyNoPanic(t *testing.T) {
	// Chart without body text should NOT be split
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{
				Title: "Chart Only",
				Content: types.SlideContent{
					DiagramSpec: &types.DiagramSpec{Type: "pie"},
				},
			},
		},
	}

	warnings := SplitContentOverflow(pres, singlePlaceholderLayouts())

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(pres.Slides) != 1 {
		t.Errorf("expected 1 slide, got %d", len(pres.Slides))
	}
}

func TestMaxContentPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		layouts []types.LayoutMetadata
		want    int
	}{
		{"empty", nil, 0},
		{"single_body", singlePlaceholderLayouts(), 1},
		{"two_body", twoPlaceholderLayouts(), 2},
		{"chart_layout", chartPlaceholderLayouts(), 2}, // body + chart
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxContentPlaceholders(tt.layouts)
			if got != tt.want {
				t.Errorf("maxContentPlaceholders() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAnyLayoutHasChartPlaceholder(t *testing.T) {
	if anyLayoutHasChartPlaceholder(singlePlaceholderLayouts()) {
		t.Error("singlePlaceholderLayouts should not have chart PH")
	}
	if !anyLayoutHasChartPlaceholder(chartPlaceholderLayouts()) {
		t.Error("chartPlaceholderLayouts should have chart PH")
	}
}
