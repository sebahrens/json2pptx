package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

// Timeline slides (go-slide-creator-wrsb).
//
// Dated milestones on a line — "when this happened" or "when this will" — are
// not the same slide as a phased roadmap, and the DeckSpec only had the latter.
// An author with five dates and no workstreams had to bend them into phases or
// drop to raw_json2pptx. This kind compiles them onto timeline-horizontal;
// roadmap keeps phase-roadmap.

const (
	// timelineMinStops / timelineMaxStops mirror the pattern's stop count.
	timelineMinStops = 3
	timelineMaxStops = 7
	// timelineLabelMax / timelineDateMax / timelineBodyMax mirror its string
	// budgets.
	timelineLabelMax = 60
	timelineDateMax  = 30
	timelineBodyMax  = 200
)

// timelineStop is one resolved milestone. It carries the pattern's own field
// names, so the values array marshals straight from it.
type timelineStop struct {
	Label   string `json:"label"`
	Date    string `json:"date,omitempty"`
	EndDate string `json:"end_date,omitempty"`
	Body    string `json:"body,omitempty"`
}

// timelineOverrides carries the visual style. Only "gantt" is ever set: the
// pattern rejects an end_date in any other style, so a payload with ranges has
// to say so.
type timelineOverrides struct {
	Style string `json:"style,omitempty"`
}

// CompileTimeline compiles a timeline payload onto timeline-horizontal, falling
// back to a dated bullet list when the milestones do not fit the pattern.
func CompileTimeline(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	stops := TimelineStops(in.Body)
	if !timelineFits(stops) {
		return compileTimelineFallback(in, stops)
	}

	encoded, err := json.Marshal(stops)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal timeline-horizontal values: %w", err)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: "timeline-horizontal", Values: encoded}
	if timelineHasRanges(stops) {
		// A milestone with a start and an end is a bar, not a dot, and the
		// pattern refuses an end_date in any other style.
		overrides, oErr := json.Marshal(timelineOverrides{Style: "gantt"})
		if oErr != nil {
			return nil, nil, fmt.Errorf("marshal timeline-horizontal overrides: %w", oErr)
		}
		slide.Pattern.Overrides = overrides
	}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values",
		SemanticPath: in.semSlide() + "." + timelineStopsField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// compileTimelineFallback renders the milestones as "Date — Label: body"
// bullets, so a longer or shorter run of dates keeps every word.
func compileTimelineFallback(in Input, stops []timelineStop) (*deckinput.SlideInput, []SourceLink, error) {
	bullets := make([]string, 0, len(stops))
	for _, st := range stops {
		line := st.Label
		if date := timelineDateText(st); date != "" {
			line = date + " — " + line
		}
		if st.Body != "" {
			line += ": " + st.Body
		}
		bullets = append(bullets, line)
	}
	if len(bullets) == 0 {
		return CompileFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "content"}
	links := titleLink(slide, in)
	idx := appendContent(slide, bulletsContent("body", bullets))
	links = append(links, SourceLink{
		RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + "." + timelineStopsField(in.Body),
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// timelineDateText renders a stop's date, as a range when it has an end.
func timelineDateText(st timelineStop) string {
	switch {
	case st.Date != "" && st.EndDate != "":
		return st.Date + "–" + st.EndDate
	case st.Date != "":
		return st.Date
	default:
		return st.EndDate
	}
}

// TimelineStops resolves a timeline payload's milestones. A stop is a string (a
// bare label), or an object carrying a label and optionally a date, an end date
// and a line of body text. Entries with no usable label are dropped.
func TimelineStops(body map[string]any) []timelineStop {
	raw, ok := firstList(body, "milestones", "stops", "events", "timeline")
	if !ok {
		return nil
	}
	var out []timelineStop
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, timelineStop{Label: s})
			}
		case map[string]any:
			label := firstNonEmpty(strField(t, "label"), strField(t, "title"), strField(t, "name"), strField(t, "milestone"), strField(t, "event"))
			if label == "" {
				continue
			}
			out = append(out, timelineStop{
				Label:   label,
				Date:    firstNonEmpty(strField(t, "date"), strField(t, "date_label"), strField(t, "when"), strField(t, "start"), strField(t, "start_date")),
				EndDate: firstNonEmpty(strField(t, "end_date"), strField(t, "end"), strField(t, "until")),
				Body:    firstNonEmpty(strField(t, "body"), strField(t, "description"), strField(t, "detail"), strField(t, "summary")),
			})
		}
	}
	return out
}

// timelineStopsField names the payload field the milestones came from.
func timelineStopsField(body map[string]any) string {
	for _, key := range []string{"milestones", "stops", "events", "timeline"} {
		if _, ok := body[key]; ok {
			return key
		}
	}
	return "milestones"
}

// timelineHasRanges reports whether any stop spans a period rather than marking
// a point.
func timelineHasRanges(stops []timelineStop) bool {
	for _, st := range stops {
		if st.EndDate != "" {
			return true
		}
	}
	return false
}

// timelineFits reports whether the milestones fit the pattern: 3–7 stops, each
// inside its text budgets.
func timelineFits(stops []timelineStop) bool {
	if len(stops) < timelineMinStops || len(stops) > timelineMaxStops {
		return false
	}
	for _, st := range stops {
		if runeLen(st.Label) > timelineLabelMax ||
			runeLen(st.Date) > timelineDateMax ||
			runeLen(st.EndDate) > timelineDateMax ||
			runeLen(st.Body) > timelineBodyMax {
			return false
		}
	}
	return true
}

// TimelinePattern returns the pattern a timeline payload compiles to, or ""
// when it degrades to bullets.
func TimelinePattern(body map[string]any) string {
	if timelineFits(TimelineStops(body)) {
		return "timeline-horizontal"
	}
	return ""
}

// TimelineOverBudget explains why the milestones cannot take the timeline, for
// the finding that reports the degrade. Returns "" when they fit.
func TimelineOverBudget(body map[string]any) string {
	stops := TimelineStops(body)
	switch {
	case len(stops) == 0:
		return ""
	case len(stops) < timelineMinStops:
		return fmt.Sprintf("has %d milestones; a timeline needs at least %d (two dates are a comparison, not a line)", len(stops), timelineMinStops)
	case len(stops) > timelineMaxStops:
		return fmt.Sprintf("has %d milestones; the line holds at most %d", len(stops), timelineMaxStops)
	}
	for i, st := range stops {
		switch {
		case runeLen(st.Label) > timelineLabelMax:
			return fmt.Sprintf("milestone %d's label is %d characters; a stop holds %d", i+1, runeLen(st.Label), timelineLabelMax)
		case runeLen(st.Date) > timelineDateMax:
			return fmt.Sprintf("milestone %d's date is %d characters; a stop holds %d", i+1, runeLen(st.Date), timelineDateMax)
		case runeLen(st.EndDate) > timelineDateMax:
			return fmt.Sprintf("milestone %d's end date is %d characters; a stop holds %d", i+1, runeLen(st.EndDate), timelineDateMax)
		case runeLen(st.Body) > timelineBodyMax:
			return fmt.Sprintf("milestone %d's body is %d characters; a stop holds %d", i+1, runeLen(st.Body), timelineBodyMax)
		}
	}
	return ""
}

// UsableTimelineStopCount returns how many milestones survive extraction.
func UsableTimelineStopCount(body map[string]any) int { return len(TimelineStops(body)) }
