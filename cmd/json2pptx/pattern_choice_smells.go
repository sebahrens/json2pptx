package main

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Pattern-choice & rendering-geometry QA heuristics (J2P-VQA-009).
//
// A rendered-deck review surfaced layout-quality issues that static validation
// passed: single-row flow lanes stretched over-tall, decision diamonds with no
// zone to explain the branch, agenda slides drawn as flowcharts, and rotated
// axis bands that intrude into the quadrants after rotation. These detectors
// flag those cases at preflight so an agent can swap to a better-suited
// pattern (or, for the rotated band, fix the geometry).
//
// Each finding is advisory (action "review") and carries a QA class via
// patterns.FindingClass: the flow / agenda smells are "pattern_choice" (the
// engine rendered what it was asked — swap the pattern), while the rotated
// axis band is "rendering" (a geometry artifact to fix). The split lets a QA
// report separate a poor pattern choice from a rendering bug.

const (
	// overtallFlowMinHeightPct is the lane-height fraction (% of the content
	// area) above which a sparse single-row flow reads as over-tall boxes. A
	// cap at or below this (e.g. the recommended max_height_pct ~35) is fine.
	overtallFlowMinHeightPct = 50.0

	// matrixRotationTolDeg is how close (in degrees) a shape's rotation must be
	// to 90° or 270° to count as a rotated axis band.
	matrixRotationTolDeg = 15.0
)

// tocFlowchartPatterns are the sequential / flowchart patterns that imply a
// causal or temporal order an agenda / table-of-contents slide does not have.
var tocFlowchartPatterns = map[string]bool{
	"process-flow":         true,
	"process-flow-compact": true,
	"swimlane":             true,
	"timeline-horizontal":  true,
}

// tocTitleVocab holds the distinctive agenda / table-of-contents title phrases.
// Kept tight (no bare "contents" / "outline" / "roadmap") to avoid flagging a
// legitimate roadmap or content slide that merely contains one of those words.
var tocTitleVocab = []string{
	"agenda",
	"table of contents",
	"what we'll cover",
	"what we will cover",
}

// collectPatternChoiceFindings runs the J2P-VQA-009 visual-quality heuristics
// over every slide and returns the findings. It reads slide.Pattern /
// slide.ShapeGrid directly, so compose envelopes and nested cell patterns are
// exempt from the slide-level flow checks by construction (a second zone there
// already absorbs the slide height / explains the branch).
func collectPatternChoiceFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for si := range input.Slides {
		slide := &input.Slides[si]
		if f := detectOvertallFlowLane(slide.Pattern, si); f != nil {
			findings = append(findings, *f)
		}
		if f := detectFlowDiamondNoContent(slide.Pattern, si); f != nil {
			findings = append(findings, *f)
		}
		if f := detectTocFlowchartVocab(slide, si); f != nil {
			findings = append(findings, *f)
		}
		findings = append(findings, detectMatrixAxisImbalance(slide.ShapeGrid, si)...)
	}
	return findings
}

// detectOvertallFlowLane flags a slide-level process-flow / timeline-horizontal
// whose lane occupies more than half the content height with short per-cell
// labels. It is the complement to detectSparseSingleRowFlow: it covers the
// over-tall cases that guard does not (a height cap that is still too tall, or
// a 7–8 step row), and defers to SPARSE_SINGLE_ROW_FLOW whenever that guard
// owns the case so the two never both fire on one slide.
func detectOvertallFlowLane(p *PatternInput, slideIdx int) *patterns.FitFinding {
	if p == nil {
		return nil
	}
	itemCount, totalChars, ok := sparseFlowTextStats(p)
	if !ok || itemCount < sparseFlowMinItems {
		return nil
	}
	avg := float64(totalChars) / float64(itemCount)
	if avg >= sparseFlowMaxAvgChars {
		return nil
	}
	// Defer to SPARSE_SINGLE_ROW_FLOW (uncapped, 3–6 items) — OVERTALL only
	// covers what that guard misses.
	if detectSparseSingleRowFlow(p, slideIdx) != nil {
		return nil
	}
	laneHeightPct := overtallFlowLaneHeightPct(p)
	if laneHeightPct <= overtallFlowMinHeightPct {
		return nil
	}
	f := patterns.OvertallFlowLane(p.Name, slidepath.SlideField(slideIdx, "pattern"), slideIdx, itemCount, laneHeightPct, avg)
	return &f
}

// overtallFlowLaneHeightPct estimates the lane height as a percentage of the
// content area: an explicit max_height_pct / bounds.height when present, else
// ~100% (an uncapped single-row flow fills the whole content zone).
func overtallFlowLaneHeightPct(p *PatternInput) float64 {
	if p.MaxHeightPct > 0 && p.MaxHeightPct < 100 {
		return p.MaxHeightPct
	}
	if p.Bounds != nil && p.Bounds.Height > 0 {
		return p.Bounds.Height
	}
	return 100.0
}

// detectFlowDiamondNoContent flags a standalone process-flow that carries at
// least one decision diamond (step.type == "decision") — a lone single-row
// flow has no zone to explain the yes/no branch outcomes the diamond implies.
func detectFlowDiamondNoContent(p *PatternInput, slideIdx int) *patterns.FitFinding {
	if p == nil || p.Name != "process-flow" {
		return nil
	}
	var vals struct {
		Steps []struct {
			Type string `json:"type"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(p.Values, &vals); err != nil {
		return nil
	}
	diamonds := 0
	for _, s := range vals.Steps {
		if s.Type == "decision" {
			diamonds++
		}
	}
	if diamonds == 0 {
		return nil
	}
	f := patterns.FlowDiamondNoContent(slidepath.SlideField(slideIdx, "pattern"), slideIdx, diamonds)
	return &f
}

// detectTocFlowchartVocab flags an agenda / table-of-contents slide drawn with
// a sequential flow pattern. The title must match the agenda vocabulary AND the
// slide-level pattern must be one of the flowchart families.
func detectTocFlowchartVocab(slide *SlideInput, slideIdx int) *patterns.FitFinding {
	if slide == nil || slide.Pattern == nil || !tocFlowchartPatterns[slide.Pattern.Name] {
		return nil
	}
	_, title := extractTitleText(*slide)
	if title == "" {
		return nil
	}
	if !matchesTocVocab(strings.ToLower(title)) {
		return nil
	}
	f := patterns.TocFlowchartVocab(slide.Pattern.Name, slidepath.SlideField(slideIdx, "pattern"), slideIdx, title)
	return &f
}

// matchesTocVocab reports whether a lower-cased title contains an agenda /
// table-of-contents vocabulary phrase.
func matchesTocVocab(lowerTitle string) bool {
	for _, kw := range tocTitleVocab {
		if strings.Contains(lowerTitle, kw) {
			return true
		}
	}
	return false
}

// detectMatrixAxisImbalance flags a shape_grid cell whose text-bearing shape is
// rotated ~90°/270° and spans rows or columns (an axis band). Rotating the band
// flips its width/height about its center, so it renders wide-short (or
// tall-narrow) and intrudes into adjacent cells.
func detectMatrixAxisImbalance(grid *ShapeGridInput, slideIdx int) []patterns.FitFinding {
	if grid == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for ri, row := range grid.Rows {
		for ci, cell := range row.Cells {
			if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
				continue
			}
			if !isNearRightAngleRotation(cell.Shape.Rotation) {
				continue
			}
			if cell.RowSpan < 2 && cell.ColSpan < 2 {
				continue
			}
			path := slidepath.GridCellField(slideIdx, ri, ci, "shape")
			findings = append(findings, patterns.MatrixAxisImbalance(path, slideIdx, cell.Shape.Rotation))
		}
	}
	return findings
}

// isNearRightAngleRotation reports whether a rotation (degrees) is within
// matrixRotationTolDeg of 90° or 270° after normalizing to [0,360).
func isNearRightAngleRotation(deg float64) bool {
	r := math.Mod(deg, 360)
	if r < 0 {
		r += 360
	}
	return math.Abs(r-90) <= matrixRotationTolDeg || math.Abs(r-270) <= matrixRotationTolDeg
}
