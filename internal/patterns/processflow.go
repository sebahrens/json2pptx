package patterns

// Process-flow pattern: left-to-right steps connected by arrows.

import (
	"fmt"
	"strings"
)

// init registers the process-flow builder.
func init() {
	registerBuilder("process-flow", buildProcessFlow)
}

// stepFontPt returns a starting font size for step boxes that shrinks as the
// number of steps grows, so labels in narrower boxes have a fighting chance of
// fitting before downstream text-fitting kicks in. Each step is "label" or
// "label|description"; an over-long longest label nudges the size down one more
// notch.
func stepFontPt(n, longestLabel int) float64 {
	var pt float64
	switch {
	case n >= 7:
		pt = 11
	case n == 6:
		pt = 12
	case n == 5:
		pt = 13
	case n == 4:
		pt = 14
	default:
		pt = 16
	}
	// Long labels in already-narrow boxes: drop one step further (floor 10pt).
	if longestLabel > 18 && pt > 10 {
		pt--
	}
	return pt
}

// buildProcessFlow lays out a horizontal flow of steps connected by arrows.
// Steps are evenly distributed across the usable content width with small side
// margins so the flow does not bleed to the slide edges. Each step is "label"
// or "label|description"; the optional description renders below the step box.
func buildProcessFlow(p PatternParams) (*BuildResult, error) {
	steps := p.StringSlice("steps")
	if len(steps) == 0 {
		return nil, &BuildError{Pattern: "process-flow", Msg: "steps is required (array of step labels)"}
	}

	n := len(steps)
	// Reserve small side margins so steps and arrows clear the slide edges.
	const sideMargin = 0.03
	usableW := 1.0 - 2*sideMargin
	stepW := usableW / float64(n)

	// Box occupies most of each slot; the remainder is the gap that holds the
	// connector arrow to the next step.
	const boxFrac = 0.80

	longest := 0
	hasDesc := false
	for _, raw := range steps {
		parts := strings.SplitN(raw, "|", 2)
		if l := len(parts[0]); l > longest {
			longest = l
		}
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			hasDesc = true
		}
	}
	fontPt := stepFontPt(n, longest)

	// Center the band vertically; leave room for descriptions when present.
	boxY, boxH := 0.40, 0.24
	if hasDesc {
		boxY, boxH = 0.30, 0.22
	}
	arrowH := 0.05
	arrowY := boxY + boxH/2 - arrowH/2

	cells := make([]Cell, 0, n*2)
	for i, raw := range steps {
		parts := strings.SplitN(raw, "|", 2)
		label := parts[0]
		desc := ""
		if len(parts) == 2 {
			desc = strings.TrimSpace(parts[1])
		}

		x := sideMargin + stepW*float64(i)

		// Step box.
		cells = append(cells, Cell{
			ID:      fmt.Sprintf("step-%d", i),
			XPct:    x,
			YPct:    boxY,
			WPct:    stepW * boxFrac,
			HPct:    boxH,
			Type:    "rounded_rect",
			Text:    label,
			FontPt:  fontPt,
			Bold:    true,
			Fill:    "accent1",
			TextCol: "background1",
			Align:   "center",
			VAlign:  "middle",
		})

		// Optional description below the step box.
		if desc != "" {
			cells = append(cells, Cell{
				ID:     fmt.Sprintf("desc-%d", i),
				XPct:   x,
				YPct:   boxY + boxH + 0.02,
				WPct:   stepW * boxFrac,
				HPct:   0.20,
				Type:   "text",
				Text:   desc,
				FontPt: fontPt - 2,
				Align:  "center",
				VAlign: "top",
			})
		}

		// Connector arrow in the gap to the next step.
		if i < n-1 {
			cells = append(cells, Cell{
				ID:     fmt.Sprintf("arrow-%d", i),
				XPct:   x + stepW*boxFrac,
				YPct:   arrowY,
				WPct:   stepW * (1 - boxFrac),
				HPct:   arrowH,
				Type:   "right_arrow",
				Fill:   "accent2",
			})
		}
	}

	return &BuildResult{Cells: cells}, nil
}
