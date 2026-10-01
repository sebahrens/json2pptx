package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/types"
)

// deckBackMatter reports, per slide, whether it is appendix back matter: an
// appendix divider (see appendixDividerPrefixes) and the slides after it up to
// the next ordinary divider, or a slide whose running section (structure
// sections[].appendix) names an appendix. Deck-rhythm checks skip these
// slides (go-slide-creator-khzni).
func deckBackMatter(slides []SlideInput, layouts []types.LayoutMetadata) []bool {
	prefixes := appendixDividerPrefixes(slides, layouts)
	probes := make([]types.BackMatterProbe, len(slides))
	for i := range slides {
		if isSectionSlideInput(slides[i], layouts) {
			probes[i] = types.BackMatterProbe{Divider: true, AppendixDivider: prefixes[i] != ""}
			continue
		}
		probes[i] = types.BackMatterProbe{Crumb: slides[i].SectionTitle}
	}
	return types.BackMatterMask(probes)
}

// appendixDividerPrefixes returns, per slide, the page-label prefix of a
// divider that opens appendix back matter ("A", "B", …) and "" for every other
// slide. A divider opens back matter when it is marked appendix: true, is
// titled Appendix / Backup / Annex, or carries a single-letter section_number
// label ("A") after the deck has numbered chapters — a deck that letters all
// its chapters A, B, C is not an appendix. The prefix is the divider's letter
// label, the letter of an "Appendix B: …" title, or "A".
func appendixDividerPrefixes(slides []SlideInput, layouts []types.LayoutMetadata) []string {
	_, ordinals := deckSectionNumbers(slides, layouts)
	out := make([]string, len(slides))
	numberedBefore := false
	for i := range slides {
		if !isSectionSlideInput(slides[i], layouts) {
			continue
		}
		_, title := extractTitleText(slides[i])
		letter := ""
		if o := slides[i].SectionNumber; o != nil && !o.Suppress {
			letter = singleLetterLabel(o.Label)
		}
		switch {
		case slides[i].Appendix || types.IsAppendixSectionTitle(title):
			out[i] = letter
			if out[i] == "" {
				out[i] = appendixTitleLetter(title)
			}
		case letter != "" && numberedBefore:
			out[i] = letter
		}
		if ordinals[i] > 0 {
			numberedBefore = true
		}
	}
	return out
}

// singleLetterLabel returns an upper-case section_number label that is one
// ASCII letter ("A", "b"), or "".
func singleLetterLabel(label string) string {
	label = strings.TrimSpace(label)
	if len(label) != 1 {
		return ""
	}
	c := label[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	if c < 'A' || c > 'Z' {
		return ""
	}
	return string(c)
}

// appendixTitleLetter returns the letter an appendix title names ("Appendix B:
// Methodology" → "B"), or "A".
func appendixTitleLetter(title string) string {
	fields := strings.FieldsFunc(title, func(r rune) bool {
		return r == ' ' || r == ':' || r == '-' || r == '.' || r == '\t' || r == '–' || r == '—'
	})
	if len(fields) >= 2 {
		if l := singleLetterLabel(fields[1]); l != "" {
			return l
		}
	}
	return "A"
}

// deckPageLabels returns the literal page labels of a deck with appendix back
// matter (go-slide-creator-khzni): back-matter slides read A1, A2, … (the
// count restarts per appendix letter), the appendix divider shows no number,
// and a main-deck slide placed after the back matter (a closing slide)
// continues the main deck's count instead of its physical position. labels[i]
// is "" where the physical slide number is already right, so the slide keeps
// PowerPoint's auto field. mainTotal counts the main-deck slides for a
// "{current} / {total}" format. All return values are zero when the deck has
// no back matter.
func deckPageLabels(slides []SlideInput, layouts []types.LayoutMetadata) (labels []string, hide []bool, mainTotal int) {
	mask := deckBackMatter(slides, layouts)
	anyBack := false
	for _, b := range mask {
		anyBack = anyBack || b
	}
	if !anyBack {
		return nil, nil, 0
	}
	prefixes := appendixDividerPrefixes(slides, layouts)
	labels = make([]string, len(slides))
	hide = make([]bool, len(slides))
	counters := map[string]int{}
	current := "A"
	for i := range slides {
		if !mask[i] {
			mainTotal++
			if mainTotal != i+1 {
				labels[i] = strconv.Itoa(mainTotal)
			}
			continue
		}
		if prefixes[i] != "" {
			current = prefixes[i]
			hide[i] = true
			continue
		}
		counters[current]++
		labels[i] = current + strconv.Itoa(counters[current])
	}
	return labels, hide, mainTotal
}

// applyAppendixPageLabels numbers appendix back matter A1, A2, … in the footer
// (go-slide-creator-khzni). slides must be index-aligned with the generated
// slide specs. The main deck's own numbers stay contiguous, and a "{total}"
// format counts only the main deck.
func applyAppendixPageLabels(footer *generator.FooterConfig, slides []SlideInput, layouts []types.LayoutMetadata) {
	if footer == nil || !footer.Enabled {
		return
	}
	labels, hide, mainTotal := deckPageLabels(slides, layouts)
	if labels == nil {
		return
	}
	footer.PageLabelBySlide = labels
	footer.HidePageNumberBySlide = hide
	footer.TotalSlides = mainTotal
}

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
