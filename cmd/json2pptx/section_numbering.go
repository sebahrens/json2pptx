package main

import (
	"fmt"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/types"
)

// deckSectionNumbers returns, per slide, the label auto-injected into a
// section divider's number slot ("" for non-dividers and unnumbered dividers),
// and the 1-based chapter ordinal the automatic counter assigned (0 when the
// divider was not counted).
//
// Rules (go-slide-creator-7ldh9):
//   - section_number: false — no number, the counter does not advance;
//   - section_number: "<label>" — the label verbatim; a numeric label also
//     resets the counter to its value, so later dividers continue from it;
//   - otherwise a divider titled Appendix / Backup / Annex / Q&A / Questions /
//     Thank you is unnumbered and not counted;
//   - every other divider gets the next "%02d" number.
//
// Every surface that predicts the injected number (rendering, contrast
// checks, SECTION_NUMBER_SEQUENCE_MISMATCH) shares this one function.
func deckSectionNumbers(slides []SlideInput, layouts []types.LayoutMetadata) (labels []string, ordinals []int) {
	labels = make([]string, len(slides))
	ordinals = make([]int, len(slides))
	counter := 0
	for i := range slides {
		if !isSectionSlideInput(slides[i], layouts) {
			continue
		}
		if o := slides[i].SectionNumber; o != nil {
			if o.Suppress {
				continue
			}
			if o.Label != "" {
				labels[i] = o.Label
				if n, err := strconv.Atoi(o.Label); err == nil && n > 0 {
					counter = n
					ordinals[i] = n
				}
				continue
			}
		}
		if _, title := extractTitleText(slides[i]); types.IsUnnumberedSectionTitle(title) {
			continue
		}
		counter++
		labels[i] = fmt.Sprintf("%02d", counter)
		ordinals[i] = counter
	}
	return labels, ordinals
}
