package main

import (
	"encoding/json"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Sparse single-row flow guard (J2P-FLOW-002).
//
// process-flow and the single-row "dots" style of timeline-horizontal expand
// to one horizontal row sized to its text (go-slide-creator-xb06p made
// process-flow steps content-sized; they used to stretch over the content
// area, which is what this guard was written against). When that row carries
// only short labels (no descriptions / bodies) and is the slide's only
// content, the slide is a short band of a few words over an empty page. This
// guard flags that case at preflight so an agent can swap to a denser layout
// family (numbered-step-strip with a detail zone, process-grid-2row,
// phase-roadmap) or pair the row with a second zone (go-slide-creator-tm46l).
//
// The check intentionally reads slide.Pattern directly rather than the expanded
// grid: a single slide-level pattern is the only shape this applies to. Compose
// envelopes and nested cell patterns are exempt because a second zone already
// absorbs the slide height (their warnings are discarded at merge time anyway),
// satisfying the "embedded-in-grid second-zone case does NOT fire" contract.
const (
	// sparseFlowMinItems / sparseFlowMaxItems bound the item count the guard
	// fires for. Below 3 a pattern fails its own validation; above 6 a
	// process-flow is on two rows and a timeline's stops fill the width.
	sparseFlowMinItems = 3
	sparseFlowMaxItems = 6

	// sparseFlowMaxAvgChars is the average per-cell text length (label, plus
	// date/body for timelines) below which the row is considered sparse. A flow
	// whose cells average more than this carries enough text to justify the
	// height, so it is left alone.
	sparseFlowMaxAvgChars = 40.0
)

// collectSparseSingleRowFlowFindings emits a SPARSE_SINGLE_ROW_FLOW finding for
// each slide whose top-level pattern is a sparse, height-uncapped single-row
// sequence.
func collectSparseSingleRowFlowFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for si := range input.Slides {
		if f := detectSparseSingleRowFlow(input.Slides[si].Pattern, si); f != nil {
			findings = append(findings, *f)
		}
	}
	return findings
}

// detectSparseSingleRowFlow returns a SPARSE_SINGLE_ROW_FLOW finding for a
// slide-level pattern, or nil when the pattern is not an uncapped sparse
// single-row flow.
func detectSparseSingleRowFlow(p *PatternInput, slideIdx int) *patterns.FitFinding {
	if p == nil {
		return nil
	}
	// A pattern placed with bounds or max_height_pct is the author's own
	// composition — not reported.
	if p.Bounds != nil {
		return nil
	}
	if p.MaxHeightPct > 0 && p.MaxHeightPct < 100 {
		return nil
	}

	itemCount, totalChars, ok := sparseFlowTextStats(p)
	if !ok {
		return nil
	}
	if itemCount < sparseFlowMinItems || itemCount > sparseFlowMaxItems {
		return nil
	}

	avg := float64(totalChars) / float64(itemCount)
	if avg >= sparseFlowMaxAvgChars {
		return nil
	}

	finding := patterns.SparseSingleRowFlow(
		p.Name,
		slidepath.SlideField(slideIdx, "pattern"),
		slideIdx,
		itemCount,
		avg,
	)
	return &finding
}

// sparseFlowTextStats returns the item count and total trimmed text length for
// the single-row flow patterns this guard covers. The third return value is
// false when the pattern is not a single-row flow (including the multi-row
// chevron / gantt timeline styles and a process-flow bent onto two rows), in
// which case the guard does not apply.
func sparseFlowTextStats(p *PatternInput) (itemCount, totalChars int, ok bool) {
	switch p.Name {
	case "process-flow":
		var vals struct {
			Steps []struct {
				Label string `json:"label"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(p.Values, &vals); err != nil {
			return 0, 0, false
		}
		// overrides.rows 2 bends a flow of four or more steps onto two rows.
		var ovr struct {
			Rows int `json:"rows"`
		}
		if len(p.Overrides) > 0 && json.Unmarshal(p.Overrides, &ovr) == nil && ovr.Rows == 2 && len(vals.Steps) >= 4 {
			return 0, 0, false
		}
		for _, s := range vals.Steps {
			totalChars += len(strings.TrimSpace(s.Label))
		}
		return len(vals.Steps), totalChars, true

	case "timeline-horizontal":
		// Only the default "dots" style is a single row. chevron adds a date
		// row and gantt renders one row per stop, so neither is a lone row.
		if timelineHorizontalStyle(p.Overrides) != "dots" {
			return 0, 0, false
		}
		var stops []struct {
			Label string `json:"label"`
			Date  string `json:"date"`
			Body  string `json:"body"`
		}
		if err := json.Unmarshal(p.Values, &stops); err != nil {
			return 0, 0, false
		}
		for _, s := range stops {
			totalChars += len(strings.TrimSpace(s.Label)) +
				len(strings.TrimSpace(s.Date)) +
				len(strings.TrimSpace(s.Body))
		}
		return len(stops), totalChars, true

	default:
		return 0, 0, false
	}
}

// timelineHorizontalStyle reads the "style" override from a timeline-horizontal
// pattern, defaulting to "dots" when overrides are absent or do not set it.
func timelineHorizontalStyle(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "dots"
	}
	var ovr struct {
		Style string `json:"style"`
	}
	if err := json.Unmarshal(raw, &ovr); err != nil || ovr.Style == "" {
		return "dots"
	}
	return ovr.Style
}
