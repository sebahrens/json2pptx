package pipeline

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestExtractSections(t *testing.T) {
	slides := []types.SlideDefinition{
		{Index: 0, Title: "Welcome", Type: types.SlideTypeTitle},
		{Index: 1, Title: "Introduction", Type: types.SlideTypeSection},
		{Index: 2, Title: "Slide Content", Type: types.SlideTypeContent},
		{Index: 3, Title: "Analysis", Type: types.SlideTypeSection},
		{Index: 4, Title: "More Content", Type: types.SlideTypeContent},
		{Index: 5, Title: "Conclusion", Type: types.SlideTypeSection},
	}

	sections := extractSections(slides)
	expected := []string{"Introduction", "Analysis", "Conclusion"}

	if len(sections) != len(expected) {
		t.Fatalf("got %d sections, want %d", len(sections), len(expected))
	}
	for i, s := range sections {
		if s != expected[i] {
			t.Errorf("section[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestExtractSections_NoSections(t *testing.T) {
	slides := []types.SlideDefinition{
		{Index: 0, Title: "Title", Type: types.SlideTypeTitle},
		{Index: 1, Title: "Content", Type: types.SlideTypeContent},
	}
	sections := extractSections(slides)
	if len(sections) != 0 {
		t.Fatalf("got %d sections, want 0", len(sections))
	}
}

func TestGenerateAgenda_InsertAfterTitle(t *testing.T) {
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle},
			{Index: 1, Title: "Strategy", Type: types.SlideTypeSection},
			{Index: 2, Title: "Details", Type: types.SlideTypeContent},
			{Index: 3, Title: "Financials", Type: types.SlideTypeSection},
			{Index: 4, Title: "Numbers", Type: types.SlideTypeContent},
		},
	}

	GenerateAgenda(pres)

	if len(pres.Slides) != 6 {
		t.Fatalf("got %d slides, want 6", len(pres.Slides))
	}

	// Slide 0: title (unchanged)
	if pres.Slides[0].Title != "My Deck" || pres.Slides[0].Type != types.SlideTypeTitle {
		t.Errorf("slide 0 should be title, got %q %s", pres.Slides[0].Title, pres.Slides[0].Type)
	}

	// Slide 1: agenda (inserted)
	agenda := pres.Slides[1]
	if agenda.Title != "Agenda" {
		t.Errorf("agenda title = %q, want %q", agenda.Title, "Agenda")
	}
	if agenda.Type != types.SlideTypeContent {
		t.Errorf("agenda type = %s, want content", agenda.Type)
	}
	if len(agenda.Content.Bullets) != 2 {
		t.Fatalf("agenda bullets = %d, want 2", len(agenda.Content.Bullets))
	}
	if agenda.Content.Bullets[0] != "Strategy" || agenda.Content.Bullets[1] != "Financials" {
		t.Errorf("agenda bullets = %v, want [Strategy Financials]", agenda.Content.Bullets)
	}

	// Slide 2: original slide 1 (section)
	if pres.Slides[2].Title != "Strategy" {
		t.Errorf("slide 2 = %q, want Strategy", pres.Slides[2].Title)
	}

	// Verify reindexing
	for i, s := range pres.Slides {
		if s.Index != i {
			t.Errorf("slide %d has Index %d", i, s.Index)
		}
	}
}

func TestGenerateAgenda_SkipsWhenFewerThan2Sections(t *testing.T) {
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{Index: 0, Title: "Title", Type: types.SlideTypeTitle},
			{Index: 1, Title: "Only Section", Type: types.SlideTypeSection},
			{Index: 2, Title: "Content", Type: types.SlideTypeContent},
		},
	}

	GenerateAgenda(pres)

	if len(pres.Slides) != 3 {
		t.Fatalf("should not add agenda for single section, got %d slides", len(pres.Slides))
	}
}

func TestGenerateAgenda_NoSlides(t *testing.T) {
	pres := &types.PresentationDefinition{}
	GenerateAgenda(pres) // should not panic
	if len(pres.Slides) != 0 {
		t.Fatal("should remain empty")
	}
}

func TestExtractSections_IncludesClosingTitleSlides(t *testing.T) {
	slides := []types.SlideDefinition{
		{Index: 0, Title: "My Deck", Type: types.SlideTypeTitle},
		{Index: 1, Title: "Overview", Type: types.SlideTypeSection},
		{Index: 2, Title: "Details", Type: types.SlideTypeContent},
		{Index: 3, Title: "Roadmap", Type: types.SlideTypeSection},
		{Index: 4, Title: "Plans", Type: types.SlideTypeContent},
		{Index: 5, Title: "Discussion", Type: types.SlideTypeTitle}, // closing slide
	}

	sections := extractSections(slides)
	expected := []string{"Overview", "Roadmap", "Discussion"}

	if len(sections) != len(expected) {
		t.Fatalf("got %d sections, want %d: %v", len(sections), len(expected), sections)
	}
	for i, s := range sections {
		if s != expected[i] {
			t.Errorf("section[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestExtractSections_SkipsFirstTitleSlide(t *testing.T) {
	slides := []types.SlideDefinition{
		{Index: 0, Title: "Opening Title", Type: types.SlideTypeTitle},
		{Index: 1, Title: "Content", Type: types.SlideTypeContent},
	}

	sections := extractSections(slides)
	if len(sections) != 0 {
		t.Fatalf("first title slide should not be extracted, got %v", sections)
	}
}

func TestGenerateAgenda_WithClosingTitleSlide(t *testing.T) {
	pres := &types.PresentationDefinition{
		Slides: []types.SlideDefinition{
			{Index: 0, Title: "Q4 Review", Type: types.SlideTypeTitle},
			{Index: 1, Title: "Performance", Type: types.SlideTypeSection},
			{Index: 2, Title: "Revenue", Type: types.SlideTypeContent},
			{Index: 3, Title: "Roadmap", Type: types.SlideTypeSection},
			{Index: 4, Title: "Plans", Type: types.SlideTypeContent},
			{Index: 5, Title: "Discussion", Type: types.SlideTypeTitle},
		},
	}

	GenerateAgenda(pres)

	if len(pres.Slides) != 7 {
		t.Fatalf("got %d slides, want 7", len(pres.Slides))
	}

	agenda := pres.Slides[1]
	if agenda.Title != "Agenda" {
		t.Errorf("agenda title = %q, want %q", agenda.Title, "Agenda")
	}
	if len(agenda.Content.Bullets) != 3 {
		t.Fatalf("agenda bullets = %d, want 3: %v", len(agenda.Content.Bullets), agenda.Content.Bullets)
	}
	if agenda.Content.Bullets[0] != "Performance" ||
		agenda.Content.Bullets[1] != "Roadmap" ||
		agenda.Content.Bullets[2] != "Discussion" {
		t.Errorf("agenda bullets = %v, want [Performance Roadmap Discussion]", agenda.Content.Bullets)
	}
}
