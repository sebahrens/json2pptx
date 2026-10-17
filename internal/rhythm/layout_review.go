package rhythm

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	CodeSameSideRun           = "same_side_run"
	CodeLowVisualShareRun     = "low_visual_share_run"
	CodeTakeawayRestatesTitle = "takeaway_restates_title"
	CodeTakeawayHeavyDeck     = "takeaway_heavy_deck"
	CodeLayoutVarietyHigh     = "layout_variety_high"
)

// layoutRecommendations only uses known geometry. A zero area share means
// unknown, rather than an exhibit with no area. Navigation and appendix pages
// do not contribute to content-deck thresholds.
func layoutRecommendations(slides []Slide, info []SlideInfo) []Recommendation {
	var out []Recommendation
	content, takeaways, first := 0, 0, -1
	families := map[string]bool{}
	side, sideStart, sideCount := "", 0, 0
	lowStart, lowCount := 0, 0
	for i, s := range slides {
		if isStructural(s) || s.Appendix {
			side, sideCount, lowCount = "", 0, 0
			continue
		}
		content++
		if first < 0 {
			first = i
		}
		families[visualFamily(info[i].Pattern)] = true
		if s.HasTakeaway {
			takeaways++
			if tokenOverlap(s.Title, s.Takeaway) > 0.6 {
				out = append(out, layoutAdvice(CodeTakeawayRestatesTitle, i, "the takeaway repeats the title; use a separate inference or remove the repeated line"))
			}
		}
		if s.VisualSide != "" && s.VisualSide != "full" && s.VisualAreaShare > 0 {
			if s.VisualSide != side {
				side, sideStart, sideCount = s.VisualSide, i, 0
			}
			sideCount++
			if sideCount == 3 {
				out = append(out, layoutAdvice(CodeSameSideRun, sideStart, fmt.Sprintf("three consecutive exhibits place their visual on the %s; consider changing the side or the split", side)))
			}
		} else {
			side, sideCount = "", 0
		}
		if s.VisualAreaShare > 0 && s.VisualAreaShare < 0.35 {
			if lowCount == 0 {
				lowStart = i
			}
			lowCount++
			if lowCount == 3 {
				out = append(out, layoutAdvice(CodeLowVisualShareRun, lowStart, "three consecutive slides give visuals less than 35% of the content area; enlarge the exhibit or give it a dedicated slide"))
			}
		} else {
			lowCount = 0
		}
	}
	if content > 0 && takeaways*2 > content {
		out = append(out, layoutAdvice(CodeTakeawayHeavyDeck, first, fmt.Sprintf("%d of %d content slides carry takeaway bands; keep a band for a separate inference rather than repeating each title", takeaways, content)))
	}
	if content > 0 && content <= 20 && len(families) > 6 {
		out = append(out, layoutAdvice(CodeLayoutVarietyHigh, first, fmt.Sprintf("%d layout families across %d content slides may feel restless; reuse a small set of layouts where the evidence permits", len(families), content)))
	}
	return out
}

func layoutAdvice(code string, slide int, message string) Recommendation {
	return Recommendation{Code: code, SlideIndex: slide, Message: message, Severity: "info", RecommendedBreak: []string{}}
}

func tokenOverlap(a, b string) float64 {
	tokens := func(s string) map[string]bool {
		out := map[string]bool{}
		for _, word := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
			if len([]rune(word)) > 1 {
				out[word] = true
			}
		}
		return out
	}
	x, y := tokens(a), tokens(b)
	if len(x) == 0 || len(y) == 0 {
		return 0
	}
	shared := 0
	for word := range x {
		if y[word] {
			shared++
		}
	}
	return float64(shared) / float64(max(len(x), len(y)))
}
