package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// FooterCollisionInput describes a JSON-authored shape and the footer reserved
// area on a slide. Only shapes authored in the JSON input (shape_grid cells)
// should be checked — shapes inherited from the layout master are excluded by
// the caller.
type FooterCollisionInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "slides[0].shape_grid.rows[1].cells[0]".
	Path string
	// ShapeX is the shape's horizontal offset from the slide left edge (EMU).
	ShapeX int64
	// ShapeY is the shape's vertical offset from the slide top edge (EMU).
	ShapeY int64
	// ShapeCX is the shape width (EMU).
	ShapeCX int64
	// ShapeCY is the shape height (EMU).
	ShapeCY int64
	// Role is the shape's semantic role tag. Shapes with role "background"
	// or "decor" are skipped.
	Role string
	// FooterY is the top edge of the footer reserved area (EMU from top).
	FooterY int64
	// FooterCY is the height of the footer reserved area (EMU).
	FooterCY int64
	// LayoutDeclaresFooter indicates whether the slide's resolved layout
	// declares a footer placeholder (dt, ftr, or sldNum). When false, no
	// finding is emitted regardless of geometry — this prevents false
	// positives on layouts that use heuristic fallback positioning.
	LayoutDeclaresFooter bool
	// StrictFit controls the action severity: "strict" -> refuse,
	// "warn" -> review, "off" -> skip entirely.
	StrictFit string
}

// DetectFooterCollision checks whether a JSON-authored shape intrudes into
// the footer reserved area on a slide whose layout declares a footer
// placeholder.
//
// The detector only fires when LayoutDeclaresFooter is true. Shapes tagged
// with role "background" or "decor" are skipped — they are decorative and
// intentionally placed at the edges.
//
// Returns nil when there is no collision or when the check is not applicable.
func DetectFooterCollision(input FooterCollisionInput) *patterns.FitFinding {
	// Off mode: skip entirely.
	if input.StrictFit == "off" {
		return nil
	}

	// Only fire when the layout declares a footer placeholder.
	if !input.LayoutDeclaresFooter {
		return nil
	}

	// Skip decorative shapes.
	if input.Role == "background" || input.Role == "decor" {
		return nil
	}

	// Guard against degenerate inputs.
	if input.ShapeCX <= 0 || input.ShapeCY <= 0 || input.FooterCY <= 0 {
		return nil
	}

	// Check axis-aligned rectangle intersection on the Y axis.
	// The footer occupies [FooterY, FooterY+FooterCY).
	// The shape occupies [ShapeY, ShapeY+ShapeCY).
	// They intersect when shape bottom > footer top AND shape top < footer bottom.
	shapeBottom := input.ShapeY + input.ShapeCY
	footerBottom := input.FooterY + input.FooterCY

	if shapeBottom <= input.FooterY || input.ShapeY >= footerBottom {
		return nil // No vertical overlap.
	}

	// Compute the vertical intrusion in EMU.
	overlapTop := input.ShapeY
	if overlapTop < input.FooterY {
		overlapTop = input.FooterY
	}
	overlapBottom := shapeBottom
	if overlapBottom > footerBottom {
		overlapBottom = footerBottom
	}
	intrusionEMU := overlapBottom - overlapTop

	action := "review"
	if input.StrictFit == "strict" {
		action = "refuse"
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "shape_grid",
			Path:    input.Path,
			Code:    patterns.ErrCodeFooterCollision,
			Message: fmt.Sprintf(
				"shape bottom edge (%d EMU) intrudes %d EMU into footer area (top=%d EMU)",
				shapeBottom, intrusionEMU, input.FooterY,
			),
			Fix: &patterns.FixSuggestion{Kind: "reposition_shape"},
		},
		Action: action,
		Measured: &patterns.Extent{
			WidthEMU:  input.ShapeCX,
			HeightEMU: input.ShapeCY,
		},
		Allowed: &patterns.Extent{
			WidthEMU:  input.ShapeCX,
			HeightEMU: input.FooterY - input.ShapeY, // available height above footer
		},
	}
}
