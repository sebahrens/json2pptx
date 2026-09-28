package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// rotatedAccentDeckPath is where the collapsed ROTATED_ACCENT_UNREADABLE
// finding points: the deck field that caused it, not any one slide.
const rotatedAccentDeckPath = "/accent_strategy"

// collapseRotatedAccentFindings folds the per-slide ROTATED_ACCENT_UNREADABLE
// findings into one deck-level finding.
//
// The reason is a property of the template's palette, not of a slide: every
// rotating pattern slide reported the same substitution, so an eight-slide
// deck on midnight-blue carried eight identical findings and read as eight
// problems (go-slide-creator-kspkr). The collapsed finding keeps the reason
// once and lists the affected slides. Order of the remaining findings is
// preserved.
func collapseRotatedAccentFindings(findings []patterns.FitFinding) []patterns.FitFinding {
	first := -1
	var slides []string
	seen := map[int]bool{}
	for i, f := range findings {
		if f.Code != patterns.ErrCodeRotatedAccentUnreadable {
			continue
		}
		if first < 0 {
			first = i
		}
		if si := slidepath.SlideIndex(f.Path); si >= 0 && !seen[si] {
			seen[si] = true
			slides = append(slides, strconv.Itoa(si+1))
		}
	}
	if first < 0 || (len(slides) <= 1 && findings[first].Path == rotatedAccentDeckPath) {
		return findings
	}
	merged := findings[first]
	merged.Path = rotatedAccentDeckPath
	merged.Pattern = ""
	merged.NextToolCall = nil
	merged.Message = fmt.Sprintf("accent_strategy \"rotate\" (pattern slides %s): %s",
		strings.Join(slides, ", "), rotatedAccentReason(merged.Message))
	out := make([]patterns.FitFinding, 0, len(findings)-len(slides)+1)
	for i, f := range findings {
		switch {
		case i == first:
			out = append(out, merged)
		case f.Code == patterns.ErrCodeRotatedAccentUnreadable:
			continue
		default:
			out = append(out, f)
		}
	}
	return out
}

// rotatedAccentReason strips the "slide N: <pattern>: " prefix
// patternWarningAsFinding puts on a per-slide message.
func rotatedAccentReason(msg string) string {
	if strings.HasPrefix(msg, "slide ") {
		if parts := strings.SplitN(msg, ": ", 3); len(parts) == 3 {
			return parts[2]
		}
	}
	return msg
}
