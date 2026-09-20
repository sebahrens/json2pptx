package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Pattern / content mismatch (go-slide-creator-h339i).
//
// Every geometric check asks whether the content FITS. None asks whether it
// BELONGS. The calibration corpus's B07_wrong_pattern draws three KPIs as a
// timeline and a four-month implementation plan as a 2x2 quadrant matrix: the
// field counts are right, nothing overflows, no detector sees a thing, and a
// human graded the deck 30/100.
//
// These checks read the authored values and report a pattern whose content is
// the wrong SHAPE for it. They are deliberately narrow — a false mismatch on a
// conforming deck is worse than a miss, and the last attempt at this signal
// (pattern_overcrowded) fired on every conforming timeline and 2x2 alike
// (go-slide-creator-wrsb). Each rule demands that EVERY item match, and matches
// only on forms that cannot be mistaken for the pattern's own content.

// periodWord matches a stop label that names a point in time: month names and
// abbreviations, quarters, halves, years, and week/day/phase ordinals.
var periodWord = regexp.MustCompile(`(?i)^(jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)(uary|ruary|ch|il|e|y|ust|tember|ober|ember)?$|` +
	`^(q[1-4]|h[12]|fy\d{2,4}|cy\d{2,4})$|^(19|20)\d{2}$|^(week|wk|day|month|quarter|year|phase|stage|step)\s*\d+$`)

// isPeriodLabel reports whether text reads as a point in time. Multi-token
// labels count when every token is a period word or a year ("April 2026",
// "Q3 FY26").
func isPeriodLabel(text string) bool {
	trimmed := strings.TrimSpace(strings.Trim(text, ".,;:"))
	// "Week 4" / "Phase 2" are one period written as two tokens.
	if periodWord.MatchString(trimmed) {
		return true
	}
	fields := strings.Fields(trimmed)
	if len(fields) == 0 || len(fields) > 3 {
		return false
	}
	for _, f := range fields {
		if !periodWord.MatchString(strings.Trim(f, ".,;:")) {
			return false
		}
	}
	return true
}

// measureToken matches a token that is unmistakably a measurement: a currency
// amount, or a number carrying a unit or percent suffix. A bare integer is
// deliberately excluded — a timeline stop legitimately reads "2026" or "3", and
// treating those as metrics would fire on conforming decks.
var measureToken = regexp.MustCompile(`(?i)^[+\-−]?(` +
	`[$€£¥]\s?\d[\d,.]*\s?[kmbt]?%?|` +
	`\d[\d,.]*\s?(%|x|pts?|bps|d|h|k|m|bn?|tn?|days?|hrs?|mo|mos|yrs?)` +
	`)$`)

// unitWord matches a unit that follows a bare number as a separate token
// ("14 months", "41 days").
var unitWord = regexp.MustCompile(`(?i)^(%|x|pts?|bps|days?|hrs?|hours?|weeks?|months?|mos?|years?|yrs?)$`)

// bareNumber matches a plain number with no unit.
var bareNumber = regexp.MustCompile(`^[+\-−]?\d[\d,.]*$`)

// isMeasureToken reports whether one token is a measurement.
func isMeasureToken(tok string) bool {
	return measureToken.MatchString(strings.Trim(tok, ".,;:"))
}

// isQuantityLabel reports whether text is a measurement rather than a name:
// one measurement token, or a bare number followed by its unit.
func isQuantityLabel(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	switch len(fields) {
	case 1:
		return isMeasureToken(fields[0])
	case 2:
		if isMeasureToken(fields[0]) && unitWord.MatchString(strings.Trim(fields[1], ".,;:")) {
			return true
		}
		return bareNumber.MatchString(strings.Trim(fields[0], ".,;:")) &&
			unitWord.MatchString(strings.Trim(fields[1], ".,;:"))
	default:
		return false
	}
}

// containsQuantity reports whether a short label carries a measurement, e.g.
// "EMEA $12M". Long labels are exempt: a process step may legitimately mention
// a figure ("Collect the first $1M in deposits") and is still a step.
func containsQuantity(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || len(fields) > 3 {
		return false
	}
	for _, f := range fields {
		if isMeasureToken(f) {
			return true
		}
	}
	return false
}

// minMismatchItems is the smallest item count a mismatch rule will judge. Two
// stops are too few to tell a timeline from a comparison.
const minMismatchItems = 3

// collectPatternMismatchFindings reports slide-level patterns whose authored
// content is the wrong shape for them.
func collectPatternMismatchFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for si := range input.Slides {
		p := input.Slides[si].Pattern
		if p == nil || len(p.Values) == 0 {
			continue
		}
		if f := patternMismatchFor(si, p.Name, p.Values); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// patternMismatchFor dispatches the per-pattern rule.
func patternMismatchFor(slideIdx int, name string, values json.RawMessage) *patterns.FitFinding {
	switch name {
	case "timeline-horizontal":
		return timelineAxesInverted(slideIdx, values)
	case "matrix-2x2":
		return matrixOfPeriods(slideIdx, values)
	case "process-flow", "process-flow-compact", "pyramid":
		return sequenceOfMeasures(slideIdx, name, values)
	}
	return nil
}

// timelineStop is the subset of a timeline-horizontal stop these rules read.
type timelineStop struct {
	Label string `json:"label"`
	Date  string `json:"date"`
}

// timelineAxesInverted fires when every stop's label is a measurement and no
// stop's date is a period — the author has put the metric where the time goes.
func timelineAxesInverted(slideIdx int, values json.RawMessage) *patterns.FitFinding {
	var stops []timelineStop
	if err := json.Unmarshal(values, &stops); err != nil {
		// The object form wraps the list.
		var wrapper struct {
			Stops []timelineStop `json:"stops"`
		}
		if json.Unmarshal(values, &wrapper) != nil {
			return nil
		}
		stops = wrapper.Stops
	}
	if len(stops) < minMismatchItems {
		return nil
	}
	for _, s := range stops {
		if !isQuantityLabel(s.Label) {
			return nil
		}
		if isPeriodLabel(s.Date) {
			return nil
		}
	}
	return mismatchFinding(slideIdx, "timeline-horizontal", kpiPatternFor(len(stops)),
		fmt.Sprintf("slide %d: every stop on this timeline is a measurement (%q) against a label that is not a period (%q) — this is a set of metrics, not a sequence in time",
			slideIdx+1, stops[0].Label, stops[0].Date))
}

// matrixQuadrants is the subset of matrix-2x2 values these rules read.
type matrixQuadrants struct {
	TopLeft     struct{ Header string } `json:"top_left"`
	TopRight    struct{ Header string } `json:"top_right"`
	BottomLeft  struct{ Header string } `json:"bottom_left"`
	BottomRight struct{ Header string } `json:"bottom_right"`
}

// matrixOfPeriods fires when all four quadrant headers name points in time: a
// 2x2 says "these two dimensions cross", and four dates do not cross.
func matrixOfPeriods(slideIdx int, values json.RawMessage) *patterns.FitFinding {
	var q matrixQuadrants
	if json.Unmarshal(values, &q) != nil {
		return nil
	}
	headers := []string{q.TopLeft.Header, q.TopRight.Header, q.BottomLeft.Header, q.BottomRight.Header}
	for _, h := range headers {
		if !isPeriodLabel(h) {
			return nil
		}
	}
	return mismatchFinding(slideIdx, "matrix-2x2", "phase-roadmap",
		fmt.Sprintf("slide %d: all four quadrants are named after points in time (%s) — a 2x2 shows two dimensions crossing, and dates run along one",
			slideIdx+1, strings.Join(headers, ", ")))
}

// sequenceItems reads the step / tier lists of the sequence patterns.
type sequenceItems struct {
	Steps []struct {
		Label string `json:"label"`
	} `json:"steps"`
	Tiers []json.RawMessage `json:"tiers"`
}

// sequenceOfMeasures fires when every step of a flow (or tier of a pyramid) is
// a short label carrying a figure: the slide is a set of numbers drawn as an
// order of operations.
func sequenceOfMeasures(slideIdx int, pattern string, values json.RawMessage) *patterns.FitFinding {
	var v sequenceItems
	if json.Unmarshal(values, &v) != nil {
		return nil
	}
	labels := make([]string, 0, len(v.Steps)+len(v.Tiers))
	for _, s := range v.Steps {
		labels = append(labels, s.Label)
	}
	for _, t := range v.Tiers {
		var s string
		if json.Unmarshal(t, &s) == nil {
			labels = append(labels, s)
			continue
		}
		var obj struct {
			Label string `json:"label"`
			Title string `json:"title"`
		}
		if json.Unmarshal(t, &obj) == nil {
			labels = append(labels, firstNonEmptyStr(obj.Label, obj.Title))
		}
	}
	if len(labels) < minMismatchItems {
		return nil
	}
	for _, l := range labels {
		if !containsQuantity(l) {
			return nil
		}
	}
	return mismatchFinding(slideIdx, pattern, kpiPatternFor(len(labels)),
		fmt.Sprintf("slide %d: every item is a short label carrying a figure (%q) — these are measurements side by side, not stages that follow one another",
			slideIdx+1, labels[0]))
}

// firstNonEmptyStr returns the first non-empty string.
func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// kpiPatternFor names the KPI pattern sized for n figures.
func kpiPatternFor(n int) string {
	switch {
	case n <= 2:
		return "kpi-2up"
	case n >= 6:
		return "kpi-6up"
	default:
		return fmt.Sprintf("kpi-%dup", n)
	}
}

// mismatchFinding builds the review-severity finding with its swap_pattern fix.
func mismatchFinding(slideIdx int, from, to, message string) *patterns.FitFinding {
	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: from,
			Path:    slidepath.SlideField(slideIdx, "pattern"),
			Code:    patterns.ErrCodePatternContentMismatch,
			Message: message,
			Fix: &patterns.FixSuggestion{
				Kind: "swap_pattern",
				Params: map[string]any{
					"from": from,
					"to":   to,
				},
			},
		},
		Action: "review",
	}
}
