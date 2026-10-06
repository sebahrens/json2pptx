package generator

import (
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/types"
)

// withoutDuplicateChartTitleItem marks a diagram whose title the slide's own
// title already carries, so the chart does not print it a second time
// (go-slide-creator-9nk6a). A chart is the hero of its slide: a small centred
// copy of the headline above it only takes plot height. Any other item, and a
// diagram whose title adds something, is returned unchanged.
func withoutDuplicateChartTitleItem(item ContentItem, slide SlideSpec) ContentItem {
	ds, ok := item.Value.(*types.DiagramSpec)
	if !ok || ds == nil || ds.TitleOnSlide || !chartTitleRepeatsSlideTitle(ds.Title, slideTitleText(slide)) {
		return item
	}
	cp := *ds
	cp.TitleOnSlide = true
	item.Value = &cp
	return item
}

// chartTitleRepeatsSlideTitle reports whether every word of the chart title
// appears, in order and adjacent, in the slide title: the two are equal, or
// the slide title is the chart title with a lead-in or a conclusion around it
// ("Revenue by region" under "Revenue by region: EMEA leads"). Case,
// punctuation and spacing are ignored. A chart title that says more than the
// slide title (a unit, a period) is not a repeat and stays.
func chartTitleRepeatsSlideTitle(chartTitle, slideTitle string) bool {
	chart := titleWords(chartTitle)
	slide := titleWords(slideTitle)
	if len(chart) == 0 || len(chart) > len(slide) {
		return false
	}
	for start := 0; start+len(chart) <= len(slide); start++ {
		match := true
		for i, w := range chart {
			if slide[start+i] != w {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// titleWords splits a title into lower-case words of letters and digits. A
// currency or percent sign is a word of its own, so a unit the slide title
// leaves out keeps the chart title.
func titleWords(s string) []string {
	var words []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			words = append(words, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			cur = append(cur, r)
		case unicode.Is(unicode.Sc, r) || r == '%':
			flush()
			words = append(words, string(r))
		default:
			flush()
		}
	}
	flush()
	return words
}
