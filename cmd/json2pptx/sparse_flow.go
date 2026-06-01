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
// to one horizontal row that fills the slide's content area. When that row
// carries only short labels (no descriptions / bodies) and no height cap, the
// cells stretch vertically into oversized boxes around a few words. This guard
// flags that case at preflight so an agent can swap to a denser layout family
// (numbered-step-strip with a detail zone, process-grid-2row, phase-roadmap) or
// cap the height with max_height_pct.
//
// The check intentionally reads slide.Pattern directly rather than the expanded
// grid: a single slide-level pattern is the only shape this applies to. Compose
// envelopes and nested cell patterns are exempt because a second zone already
// absorbs the slide height (their warnings are discarded at merge time anyway),
// satisfying the "embedded-in-grid second-zone case does NOT fire" contract.
const (
	// sparseFlowMinItems / sparseFlowMaxItems bound the item count the guard
	// fires for. Below 3 a pattern fails its own validation; above 6 the row
	// has enough columns that each cell is narrow and short, so the vertical
	// stretch is no longer the dominant problem.
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
	// An explicit height cap (bounds or the max_height_pct alias) already keeps
	// the row from stretching — nothing to flag.
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
// chevron / gantt timeline styles), in which case the guard does not apply.
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
		for _, s := range vals.Steps {
			totalChars += len(strings.TrimSpace(s.Label))
		}
		return len(vals.Steps), totalChars, true

	case "timeline-horizontal":
		// Only the default "dots" style is a single row. chevron adds a date
		// row and gantt renders one row per stop, so neither stretches a lone
		// row to fill the slide.
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
