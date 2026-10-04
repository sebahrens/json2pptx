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
// passed: decision diamonds with no zone to explain the branch, agenda slides drawn as flowcharts, and rotated
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

// OVERTALL_FLOW_LANE is retired (go-slide-creator-0l7dr). It reported a
// timeline-horizontal lane whose "boxes stretch vertically around a few
// words", estimating the lane at 100% of the content height when uncapped.
// Nothing renders that way: an uncapped timeline is sized to its text in
// every style, and under a max_height_pct / bounds cap the rows spread to
// fill the capped area while the dots, chevrons and labels keep their size
// (the dots since go-slide-creator-r684y). What a sparse timeline leaves on
// the slide is reported as it is drawn by SPARSE_SINGLE_ROW_FLOW,
// SLIDE_UNDERUSED and SLIDE_UNBALANCED.

// detectFlowDiamondNoContent flags a standalone process-flow that carries at
// least one decision diamond (step.type == "decision") — the flow draws one
// path through its steps, on one row or two, and has no zone to explain the
// yes/no branch outcomes the diamond implies.
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
