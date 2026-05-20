package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// expandStructure converts a StructureInput into a flat []SlideInput.
// The expansion order is:
//  1. Cover slide (if present)
//  2. Agenda slide (if auto_agenda && len(sections) >= 2)
//  3. For each section: section divider + section content slides
//  4. Closing slide (if present)
//
// Section dividers are auto-generated with slide_type "section" and
// auto-numbered (01, 02, ...) via the existing section numbering logic
// in convertPresentationSlides.
func expandStructure(s *StructureInput) ([]SlideInput, error) {
	if len(s.Sections) == 0 {
		return nil, fmt.Errorf("structure: sections must not be empty")
	}

	// Validate section titles
	for i, sec := range s.Sections {
		if sec.Title == "" {
			return nil, fmt.Errorf("structure: sections[%d].title is required", i)
		}
		if len(sec.Slides) == 0 {
			return nil, fmt.Errorf("structure: sections[%d] (%q) has no slides", i, sec.Title)
		}
	}

	// Estimate capacity: cover + agenda + N*(divider+slides) + closing
	totalSlides := 0
	for _, sec := range s.Sections {
		totalSlides += 1 + len(sec.Slides) // divider + content
	}
	if s.Cover != nil {
		totalSlides++
	}
	if s.AutoAgenda && len(s.Sections) >= 2 {
		totalSlides++
	}
	if s.Closing != nil {
		totalSlides++
	}

	slides := make([]SlideInput, 0, totalSlides)

	// 1. Cover slide
	if s.Cover != nil {
		cover := *s.Cover
		if cover.SlideType == "" {
			cover.SlideType = "title"
		}
		slides = append(slides, cover)
	}

	// 2. Agenda slide (uses the "agenda" pattern)
	if s.AutoAgenda && len(s.Sections) >= 2 {
		agenda, err := buildAgendaSlide(s.Sections)
		if err != nil {
			return nil, fmt.Errorf("structure: auto_agenda: %w", err)
		}
		slides = append(slides, agenda)
	}

	// 3. Sections: divider + content
	for _, sec := range s.Sections {
		divider := buildSectionDivider(sec.Title)
		slides = append(slides, divider)
		slides = append(slides, sec.Slides...)
	}

	// 4. Closing slide
	if s.Closing != nil {
		closing := *s.Closing
		if closing.SlideType == "" {
			closing.SlideType = "title"
		}
		slides = append(slides, closing)
	}

	return slides, nil
}

// buildSectionDivider creates a section divider slide for the given title.
// The body placeholder is left empty so that the existing auto-numbering
// logic in convertPresentationSlides fills it with "01", "02", etc.
func buildSectionDivider(title string) SlideInput {
	titleValue := title
	return SlideInput{
		SlideType: "section",
		Content: []ContentInput{
			{
				PlaceholderID: "title",
				Type:          "text",
				TextValue:     &titleValue,
			},
		},
	}
}

// buildAgendaSlide creates a content slide listing all section titles as bullets.
// This uses a simple content slide with bullets — the agenda pattern can be used
// for richer agenda slides via the pattern field instead.
func buildAgendaSlide(sections []SectionInput) (SlideInput, error) {
	titles := make([]string, len(sections))
	for i, sec := range sections {
		titles[i] = sec.Title
	}

	// Build pattern values for the agenda pattern
	agendaValues := AgendaPatternValues{
		Items: titles,
	}
	valuesJSON, err := json.Marshal(agendaValues)
	if err != nil {
		return SlideInput{}, fmt.Errorf("failed to marshal agenda values: %w", err)
	}

	agendaTitle := "Agenda"
	return SlideInput{
		SlideType: "content",
		Content: []ContentInput{
			{
				PlaceholderID: "title",
				Type:          "text",
				TextValue:     &agendaTitle,
			},
		},
		Pattern: &PatternInput{
			Name:   "agenda",
			Values: valuesJSON,
		},
	}, nil
}

// AgendaPatternValues matches the agenda pattern's values schema.
type AgendaPatternValues struct {
	Items []string `json:"items"`
}

// applyStructureExpansion mirrors the CLI's structure-expansion step for MCP
// handlers. If the input has a structure block it is expanded into a flat
// slide list. When both structure and top-level slides are set, this returns
// a mutual-exclusivity diagnostic without mutating the input. When the
// structure block is malformed, expansion errors are surfaced as structured
// diagnostics so agents can repair the payload.
//
// Returns nil when there is nothing to expand or expansion succeeded.
func applyStructureExpansion(input *PresentationInput) []diagnostics.Diagnostic {
	if input == nil || input.Structure == nil {
		return nil
	}
	if len(input.Slides) > 0 {
		return []diagnostics.Diagnostic{{
			Code:     "STRUCTURE_AND_SLIDES",
			Path:     "structure",
			Message:  "structure and slides are mutually exclusive — use one or the other",
			Severity: diagnostics.SeverityError,
			Fix: &diagnostics.Fix{
				Kind:   "remove_field",
				Params: map[string]any{"field": "slides"},
			},
		}}
	}
	expanded, err := expandStructure(input.Structure)
	if err != nil {
		return []diagnostics.Diagnostic{{
			Code:     "INVALID_STRUCTURE",
			Path:     "structure",
			Message:  fmt.Sprintf("invalid structure: %v", err),
			Severity: diagnostics.SeverityError,
			Fix: &diagnostics.Fix{
				Kind:   "fix_structure",
				Params: map[string]any{"error": err.Error()},
			},
		}}
	}
	input.Slides = expanded
	return nil
}

// validateStructure checks for structural grammar issues and returns warnings.
// Currently checks for missing closing slide when cover is present and flags
// content slides that share a title (case-insensitive). Title and
// section-divider slides are exempt from the duplicate-title check because
// cover/closing/divider slides legitimately repeat phrasing.
func validateStructure(input *PresentationInput) []string {
	var warnings []string

	if input.Structure != nil {
		if input.Structure.Cover != nil && input.Structure.Closing == nil {
			warnings = append(warnings, "structure: cover slide present but no closing slide — consider adding a closing slide for symmetry")
		}
	} else if len(input.Slides) > 0 {
		// For flat slides, detect cover/closing asymmetry.
		hasCover := false
		hasClosing := false

		// First slide with type "title" = cover
		if inferSlideType(input.Slides[0]) == "title" {
			hasCover = true
		}

		// Last slide with type "title" = closing (if different from first)
		if len(input.Slides) > 1 && inferSlideType(input.Slides[len(input.Slides)-1]) == "title" {
			hasClosing = true
		}

		if hasCover && !hasClosing {
			warnings = append(warnings, "deck has a cover slide but no closing slide — consider adding a closing/thank-you slide")
		}
	}

	if dupBeyondFirst, dupGroups, _ := duplicateTitleSummary(input.Slides); dupGroups > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"deck has %d content slide title(s) duplicating an earlier slide title across %d title group(s) — rename so each headline announces a distinct point",
			dupBeyondFirst, dupGroups,
		))
	}

	return warnings
}
