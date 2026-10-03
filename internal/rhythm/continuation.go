package rhythm

import (
	"regexp"
	"strconv"
	"strings"
)

// Continuation slides (go-slide-creator-xy51l). An exhibit too large for one
// slide is split across several whose titles carry a part marker — "Savings
// by lever (1/2)" and "(2/2)", or a trailing "(cont.)". The parts are one unit
// of the argument: the second half of a table is not a second table slide for
// the monotony checks, and nothing structural belongs between the halves.

var (
	// continuationPartRE matches a trailing "(1/2)", "[2 of 3]" or "– 2/3".
	continuationPartRE = regexp.MustCompile(`(?i)\s*(?:[(\[]\s*(\d{1,2})\s*(?:/|of)\s*(\d{1,2})\s*[)\]]|[-–—]\s*(\d{1,2})\s*/\s*(\d{1,2}))\s*$`)
	// continuationWordRE matches a trailing "(cont.)", "(cont'd)", "(continued)".
	continuationWordRE = regexp.MustCompile(`(?i)\s*(?:[(\[]\s*(?:cont\.?|cont'd|cont’d|continued)\s*[)\]]|[-–—,]\s*(?:cont\.|cont'd|cont’d|continued))\s*$`)
)

// ContinuationPart describes a title's part marker.
type ContinuationPart struct {
	// Base is the title without its marker, lower-cased and whitespace-folded,
	// so the parts of one exhibit compare equal.
	Base string
	// Part and Total are the marker's numbers ("(2/3)" is 2 and 3). A
	// "(cont.)" marker has Part 0 and Total 0: it follows whatever precedes it.
	Part, Total int
}

// ParseContinuation reads the part marker from a slide title. ok is false
// for a title without one.
func ParseContinuation(title string) (ContinuationPart, bool) {
	title = strings.TrimSpace(title)
	if m := continuationPartRE.FindStringSubmatchIndex(title); m != nil {
		sub := continuationPartRE.FindStringSubmatch(title)
		partStr, totalStr := sub[1], sub[2]
		if partStr == "" {
			partStr, totalStr = sub[3], sub[4]
		}
		part, _ := strconv.Atoi(partStr)
		total, _ := strconv.Atoi(totalStr)
		base := continuationBase(title[:m[0]])
		if part >= 1 && total >= 2 && part <= total && base != "" {
			return ContinuationPart{Base: base, Part: part, Total: total}, true
		}
		return ContinuationPart{}, false
	}
	if loc := continuationWordRE.FindStringIndex(title); loc != nil {
		if base := continuationBase(title[:loc[0]]); base != "" {
			return ContinuationPart{Base: base}, true
		}
	}
	return ContinuationPart{}, false
}

func continuationBase(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Continues reports whether a slide titled next is the next part of the
// exhibit a slide titled prev shows: the same base title, and either the
// following part number or a "(cont.)" marker.
func Continues(prev, next string) bool {
	n, ok := ParseContinuation(next)
	if !ok {
		return false
	}
	p, prevMarked := ParseContinuation(prev)
	if !prevMarked {
		// "Savings by lever" followed by "Savings by lever (cont.)" or "(2/2)".
		return continuationBase(prev) == n.Base && (n.Part == 0 || n.Part == 2)
	}
	if p.Base != n.Base {
		return false
	}
	if n.Part == 0 {
		return true
	}
	return p.Part > 0 && n.Part == p.Part+1 && (p.Total == n.Total)
}

// ContinuationUnits assigns every slide the index of the first slide of the
// exhibit it belongs to: a slide that continues its predecessor shares the
// predecessor's unit, every other slide is its own. titles[i] is slide i's
// title.
func ContinuationUnits(titles []string) []int {
	units := make([]int, len(titles))
	for i := range titles {
		units[i] = i
		if i > 0 && Continues(titles[i-1], titles[i]) {
			units[i] = units[i-1]
		}
	}
	return units
}

// ContinuationGap is an exhibit whose parts are separated by other slides.
type ContinuationGap struct {
	// Before and After are the slide indices of the two parts.
	Before, After int
	// Between lists the slide indices sitting between them.
	Between []int
}

// maxContinuationGap is how many slides may sit between two parts for them to
// still be read as one split exhibit.
const maxContinuationGap = 3

// ContinuationGaps finds parts of one exhibit that do not follow each other
// directly: "(1/2)", a section divider, "(2/2)".
func ContinuationGaps(titles []string) []ContinuationGap {
	var gaps []ContinuationGap
	for i := range titles {
		p, ok := ParseContinuation(titles[i])
		if !ok || p.Part == 0 || p.Part >= p.Total {
			continue
		}
		if i+1 < len(titles) && Continues(titles[i], titles[i+1]) {
			continue
		}
		for j := i + 2; j < len(titles) && j <= i+1+maxContinuationGap; j++ {
			if !Continues(titles[i], titles[j]) {
				continue
			}
			gap := ContinuationGap{Before: i, After: j}
			for k := i + 1; k < j; k++ {
				gap.Between = append(gap.Between, k)
			}
			gaps = append(gaps, gap)
			break
		}
	}
	return gaps
}
