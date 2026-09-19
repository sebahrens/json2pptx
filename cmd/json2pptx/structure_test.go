package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandStructure_Basic(t *testing.T) {
	s := &StructureInput{
		Cover: &SlideInput{
			Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("My Deck")}},
		},
		Closing: &SlideInput{
			Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Thank You")}},
		},
		AutoAgenda: true,
		Sections: []SectionInput{
			{
				Title: "Introduction",
				Slides: []SlideInput{
					{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Slide 1")}}},
				},
			},
			{
				Title: "Analysis",
				Slides: []SlideInput{
					{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Slide 2")}}},
					{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Slide 3")}}},
				},
			},
		},
	}

	slides, err := expandStructure(s)
	if err != nil {
		t.Fatalf("expandStructure failed: %v", err)
	}

	// Expected: cover + agenda + divider1 + slide1 + divider2 + slide2 + slide3 + closing = 8
	if len(slides) != 8 {
		t.Fatalf("expected 8 slides, got %d", len(slides))
	}

	// Cover
	if slides[0].SlideType != "title" {
		t.Errorf("slides[0]: expected type 'title', got %q", slides[0].SlideType)
	}

	// Agenda (content slide with pattern)
	if slides[1].SlideType != "content" {
		t.Errorf("slides[1]: expected type 'content' (agenda), got %q", slides[1].SlideType)
	}
	if slides[1].Pattern == nil || slides[1].Pattern.Name != "agenda" {
		t.Errorf("slides[1]: expected agenda pattern")
	}

	// Section divider 1
	if slides[2].SlideType != "section" {
		t.Errorf("slides[2]: expected type 'section', got %q", slides[2].SlideType)
	}

	// Content slide 1
	if *slides[3].Content[0].TextValue != "Slide 1" {
		t.Errorf("slides[3]: expected title 'Slide 1', got %q", *slides[3].Content[0].TextValue)
	}

	// Section divider 2
	if slides[4].SlideType != "section" {
		t.Errorf("slides[4]: expected type 'section', got %q", slides[4].SlideType)
	}

	// Content slides 2 and 3
	if *slides[5].Content[0].TextValue != "Slide 2" {
		t.Errorf("slides[5]: expected title 'Slide 2'")
	}
	if *slides[6].Content[0].TextValue != "Slide 3" {
		t.Errorf("slides[6]: expected title 'Slide 3'")
	}

	// Closing
	if slides[7].SlideType != "title" {
		t.Errorf("slides[7]: expected type 'title' (closing), got %q", slides[7].SlideType)
	}
}

func TestExpandStructure_NoAgendaWithSingleSection(t *testing.T) {
	s := &StructureInput{
		AutoAgenda: true,
		Sections: []SectionInput{
			{
				Title:  "Only Section",
				Slides: []SlideInput{{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Slide")}}}},
			},
		},
	}

	slides, err := expandStructure(s)
	if err != nil {
		t.Fatalf("expandStructure failed: %v", err)
	}

	// No agenda with single section: divider + slide = 2
	if len(slides) != 2 {
		t.Fatalf("expected 2 slides (no agenda for single section), got %d", len(slides))
	}
}

func TestExpandStructure_EmptySections(t *testing.T) {
	s := &StructureInput{
		Sections: nil,
	}
	_, err := expandStructure(s)
	if err == nil {
		t.Error("expected error for empty sections")
	}
}

func TestExpandStructure_EmptyTitle(t *testing.T) {
	s := &StructureInput{
		Sections: []SectionInput{
			{Title: "", Slides: []SlideInput{{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("S")}}}}},
		},
	}
	_, err := expandStructure(s)
	if err == nil {
		t.Error("expected error for empty section title")
	}
}

func TestExpandStructure_SectionWithNoSlides(t *testing.T) {
	s := &StructureInput{
		Sections: []SectionInput{
			{Title: "Empty", Slides: nil},
		},
	}
	_, err := expandStructure(s)
	if err == nil {
		t.Error("expected error for section with no slides")
	}
}

func TestValidateStructure_MissingClosing(t *testing.T) {
	input := &PresentationInput{
		Structure: &StructureInput{
			Cover: &SlideInput{
				Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Cover")}},
			},
			Sections: []SectionInput{
				{Title: "S1", Slides: []SlideInput{{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("S")}}}}},
			},
		},
	}

	warnings := validateStructure(input)
	if len(warnings) == 0 {
		t.Error("expected warning for missing closing slide")
	}
}

func TestValidateStructure_FlatSlides_MissingClosing(t *testing.T) {
	title := "Title Slide"
	input := &PresentationInput{
		Slides: []SlideInput{
			{SlideType: "title", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}},
			{SlideType: "content", Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}}},
		},
	}

	warnings := validateStructure(input)
	if len(warnings) == 0 {
		t.Error("expected warning for flat slides with cover but no closing")
	}
}

func TestValidateStructure_WithClosing_NoWarning(t *testing.T) {
	input := &PresentationInput{
		Structure: &StructureInput{
			Cover: &SlideInput{
				Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Cover")}},
			},
			Closing: &SlideInput{
				Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("End")}},
			},
			Sections: []SectionInput{
				{Title: "S1", Slides: []SlideInput{{Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("S")}}}}},
			},
		},
	}

	warnings := validateStructure(input)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

// strPtr is declared in json_mode_test.go

// A structure block IS the slide list, but the CLI's slide-count guard ran at
// parse time and the expansion ran later, so a documented structure-only deck
// validated clean, rendered over MCP, and failed at the CLI with "at least one
// slide is required" — the same deck, two verdicts (go-slide-creator-m1kg).
func TestGenerateAcceptsAStructureOnlyDeck(t *testing.T) {
	dir := t.TempDir()
	deck := `{
      "template": "midnight-blue",
      "output_filename": "structure.pptx",
      "structure": {
        "cover": {"title": "Programme review", "subtitle": "September 2026"},
        "auto_agenda": true,
        "sections": [
          {"title": "Where we are", "slides": [
            {"slide_type": "content", "content": [
              {"placeholder_id": "title", "type": "text", "text_value": "Two workstreams are amber"}]}]},
          {"title": "What we do next", "slides": [
            {"slide_type": "content", "content": [
              {"placeholder_id": "title", "type": "text", "text_value": "Re-plan the migration wave"}]}]}
        ],
        "closing": {"title": "Questions"}
      }
    }`
	inputPath := writeTestFile(t, dir, "structure.json", deck)

	reportPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(inputPath, reportPath, filepath.Join("..", "..", "templates"), dir,
		"", false, false, "midnight-blue", "off", false, "off", "", false); err != nil {
		t.Fatalf("generate rejected a structure-only deck: %v", err)
	}

	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var result struct {
		Success    bool `json:"success"`
		SlideCount int  `json:"slide_count"`
	}
	if err := json.Unmarshal(report, &result); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if !result.Success {
		t.Errorf("generate reported failure: %s", report)
	}
	// cover + agenda + 2 section dividers + 2 content + closing.
	if result.SlideCount < 6 {
		t.Errorf("slide_count = %d; the structure should have expanded into the full sequence", result.SlideCount)
	}
}

// A deck with neither slides nor structure is still an error, and so is a
// structure that expands to nothing.
func TestGenerateStillRequiresContent(t *testing.T) {
	dir := t.TempDir()
	for name, deck := range map[string]string{
		"neither":          `{"template":"midnight-blue"}`,
		"empty structure":  `{"template":"midnight-blue","structure":{}}`,
		"sections is null": `{"template":"midnight-blue","structure":{"sections":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			inputPath := writeTestFile(t, dir, strings.ReplaceAll(name, " ", "_")+".json", deck)
			err := runJSONMode(inputPath, filepath.Join(dir, "r.json"), filepath.Join("..", "..", "templates"), dir,
				"", false, false, "midnight-blue", "off", false, "off", "", false)
			if err == nil {
				t.Error("expected an error for a deck with no content")
			}
		})
	}
}
