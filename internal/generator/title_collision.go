package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TitleCollisionInput describes a JSON-authored shape and the title chrome
// reserved area on a slide. Only shapes authored in the JSON input (shape_grid
// cells) should be checked — shapes inherited from the layout master are
// excluded by the caller.
//
// This is the title-side mirror of FooterCollisionInput: it detects shape_grid
// content that intrudes upward into the title placeholder band. The class of
// overlap it surfaces — a grid whose resolved bounds start above the layout's
// title bottom edge — previously slipped through preflight because the
// validator resolved grid geometry differently than generation
// (go-slide-creator-s1rd).
type TitleCollisionInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "/slides/0/shape_grid/rows/1/cells/0".
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
	// TitleBottom is the Y of the title placeholder's bottom edge (EMU from
	// top), i.e. ContentZone.TitleBottom. Content must not start above this.
	TitleBottom int64
	// LayoutDeclaresTitle indicates whether a real title-derived content zone
	// was resolved for the slide. When false, no finding is emitted regardless
	// of geometry — this prevents false positives on slides whose zone was a
	// generic fallback with no title anchor.
	LayoutDeclaresTitle bool
	// StrictFit controls the action severity: "strict" -> refuse,
	// "warn" -> review, "off" -> skip entirely.
	StrictFit string
}

// DetectTitleCollision checks whether a JSON-authored shape intrudes upward
// into the title reserved area on a slide whose resolved content zone is
// anchored to a title placeholder.
//
// The detector only fires when LayoutDeclaresTitle is true. Shapes tagged
// with role "background" or "decor" are skipped — they are decorative and
// intentionally placed at the edges.
//
// Returns nil when there is no collision or when the check is not applicable.
func DetectTitleCollision(input TitleCollisionInput) *patterns.FitFinding {
	// Off mode: skip entirely.
	if input.StrictFit == "off" {
		return nil
	}

	// Only fire when a title-anchored content zone was resolved.
	if !input.LayoutDeclaresTitle {
		return nil
	}

	// Skip decorative shapes.
	if input.Role == "background" || input.Role == "decor" {
		return nil
	}

	// Guard against degenerate inputs.
	if input.ShapeCX <= 0 || input.ShapeCY <= 0 || input.TitleBottom <= 0 {
		return nil
	}

	// The title band occupies [0, TitleBottom). The shape occupies
	// [ShapeY, ShapeY+ShapeCY). They collide when the shape top starts above
	// the title bottom edge.
	if input.ShapeY >= input.TitleBottom {
		return nil // Shape starts at or below the title bottom — no intrusion.
	}

	// Compute the vertical intrusion in EMU: from the shape top down to the
	// lesser of the shape bottom and the title bottom.
	shapeBottom := input.ShapeY + input.ShapeCY
	overlapBottom := shapeBottom
	if overlapBottom > input.TitleBottom {
		overlapBottom = input.TitleBottom
	}
	intrusionEMU := overlapBottom - input.ShapeY

	action := "review"
	if input.StrictFit == "strict" {
		action = "refuse"
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "shape_grid",
			Path:    input.Path,
			Code:    patterns.ErrCodeTitleCollision,
			Message: fmt.Sprintf(
				"shape top edge (%d EMU) intrudes %d EMU into title area (bottom=%d EMU)",
				input.ShapeY, intrusionEMU, input.TitleBottom,
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
			HeightEMU: shapeBottom - input.TitleBottom, // available height below title
		},
	}
}
