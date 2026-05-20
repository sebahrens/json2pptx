package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// collectDuplicateTitleFindings flags content slides that share the same
// title text (case-insensitive, whitespace-normalized). Title slides (cover,
// "Thank You", "Q&A") and section dividers are exempt because their phrasing
// legitimately repeats. The finding is emitted on the second and later
// occurrences of any duplicate title so the first occurrence is treated as
// canonical.
func collectDuplicateTitleFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil || len(input.Slides) < 2 {
		return nil
	}
	type occurrence struct {
		slideIdx int
		phID     string
	}
	groups := make(map[string][]occurrence)
	order := make([]string, 0)
	for si, slide := range input.Slides {
		if !slideQualifiesForDuplicateTitleCheck(slide) {
			continue
		}
		phID, text := extractTitleText(slide)
		if text == "" {
			continue
		}
		norm := normalizeTitleText(text)
		if norm == "" {
			continue
		}
		if _, seen := groups[norm]; !seen {
			order = append(order, norm)
		}
		groups[norm] = append(groups[norm], occurrence{slideIdx: si, phID: phID})
	}
	var findings []patterns.FitFinding
	for _, norm := range order {
		occs := groups[norm]
		if len(occs) < 2 {
			continue
		}
		firstSlide := occs[0].slideIdx
		dupSlideNumbers := make([]int, 0, len(occs))
		for _, o := range occs {
			dupSlideNumbers = append(dupSlideNumbers, o.slideIdx+1)
		}
		sort.Ints(dupSlideNumbers)
		for _, occ := range occs[1:] {
			findings = append(findings, makeDuplicateTitleFinding(occ.slideIdx, occ.phID, firstSlide, dupSlideNumbers, len(occs)))
		}
	}
	return findings
}

// slideQualifiesForDuplicateTitleCheck reports whether a slide should be
// included in the duplicate-title scan. Title and section-divider slides are
// exempt; everything else is a content slide for the purposes of this check.
// Slides carrying a shape_grid, pattern, or compose block are treated as
// content slides regardless of how inferSlideType would classify their
// content[] shape — those visuals can repeat a title even when the slide
// otherwise has only a title text run.
func slideQualifiesForDuplicateTitleCheck(slide SlideInput) bool {
	switch types.SlideType(slide.SlideType) {
	case types.SlideTypeTitle, types.SlideTypeSection:
		return false
	}
	if slide.SlideType != "" {
		return true
	}
	if slide.ShapeGrid != nil || slide.Pattern != nil || slide.Compose != nil {
		return true
	}
	switch inferSlideType(slide) {
	case types.SlideTypeTitle, types.SlideTypeSection:
		return false
	}
	return true
}

// extractTitleText returns the first non-empty title-placeholder text on the
// slide, along with the placeholder ID that carried it. Returns empty strings
// when no title text is present.
func extractTitleText(slide SlideInput) (string, string) {
	for i := range slide.Content {
		item := &slide.Content[i]
		if item.Type != "text" {
			continue
		}
		if !isHeadlinePlaceholderID(item.PlaceholderID) {
			continue
		}
		if item.TextValue != nil {
			if trimmed := strings.TrimSpace(*item.TextValue); trimmed != "" {
				return item.PlaceholderID, trimmed
			}
			continue
		}
		if resolved, err := item.ResolveValue(); err == nil {
			if s, ok := resolved.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					return item.PlaceholderID, trimmed
				}
			}
		}
	}
	return "", ""
}

// normalizeTitleText lower-cases the input, collapses runs of whitespace to a
// single space, and trims leading/trailing whitespace so that "  Next   Steps"
// compares equal to "next steps".
func normalizeTitleText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// makeDuplicateTitleFinding builds the FitFinding for a duplicate title at
// the given slide. firstSlide is the 0-based index of the canonical (earliest)
// occurrence; dupSlideNumbers is the 1-based slide numbers of every slide in
// the duplicate group, including this one; total is the size of the group.
func makeDuplicateTitleFinding(slideIdx int, phID string, firstSlide int, dupSlideNumbers []int, total int) patterns.FitFinding {
	if phID == "" {
		phID = "title"
	}
	path := slidepath.Content(slideIdx, phID)
	dupList := joinSlideNumbers(dupSlideNumbers)
	msg := fmt.Sprintf(
		"slide %d: title duplicates slide %d (%d slides share this title: %s); rename so each headline announces a distinct point",
		slideIdx+1, firstSlide+1, total, dupList,
	)
	dupNumbersAny := make([]any, len(dupSlideNumbers))
	for i, n := range dupSlideNumbers {
		dupNumbersAny[i] = n
	}
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path,
			Code: patterns.ErrCodeDuplicateTitle,
			Message: msg,
			Fix: &patterns.FixSuggestion{
				Kind: "shorten_title",
				Params: map[string]any{
					"duplicate_of_slide":          firstSlide + 1,
					"duplicate_slide_numbers":     dupNumbersAny,
					"duplicate_count":             total,
					"placeholder_id":              phID,
				},
			},
		},
		Action: "review",
	}
}

// joinSlideNumbers formats a slice of 1-based slide numbers as a
// comma-separated list, e.g. "3, 5, 8".
func joinSlideNumbers(nums []int) string {
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, ", ")
}

// duplicateTitleSummary returns the count of duplicate-title occurrences
// across the deck (each occurrence beyond the first counts), plus the count of
// distinct duplicate titles. Used by validateStructure to surface a
// deck-level warning and by compositionAxis to penalize the composition
// score.
func duplicateTitleSummary(slides []SlideInput) (occurrencesBeyondFirst, distinctGroups int, examples []string) {
	if len(slides) < 2 {
		return 0, 0, nil
	}
	groups := make(map[string][]int)
	order := make([]string, 0)
	for si, slide := range slides {
		if !slideQualifiesForDuplicateTitleCheck(slide) {
			continue
		}
		_, text := extractTitleText(slide)
		if text == "" {
			continue
		}
		norm := normalizeTitleText(text)
		if norm == "" {
			continue
		}
		if _, seen := groups[norm]; !seen {
			order = append(order, norm)
		}
		groups[norm] = append(groups[norm], si)
	}
	for _, norm := range order {
		idxs := groups[norm]
		if len(idxs) < 2 {
			continue
		}
		distinctGroups++
		occurrencesBeyondFirst += len(idxs) - 1
		examples = append(examples, norm)
	}
	return occurrencesBeyondFirst, distinctGroups, examples
}
