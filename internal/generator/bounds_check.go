package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// BoundsCheckInput describes a JSON-authored shape to check for slide-bounds
// overflow. Only shapes authored in the JSON input (shape_grid cells) should
// be checked — shapes inherited from the layout master are excluded by the
// caller.
type BoundsCheckInput struct {
	// SlideIndex is the zero-based slide index.
	SlideIndex int
	// Path is the JSON path, e.g. "slides[0].shape_grid.rows[1].cells[0]".
	Path string
	// X is the shape's horizontal offset from the slide left edge (EMU).
	X int64
	// Y is the shape's vertical offset from the slide top edge (EMU).
	Y int64
	// CX is the shape width (EMU).
	CX int64
	// CY is the shape height (EMU).
	CY int64
	// Role is the shape's semantic role tag. Shapes with role "background"
	// or "decor" are skipped. Empty string means content (checked).
	Role string
	// SlideWidth is the slide width (EMU).
	SlideWidth int64
	// SlideHeight is the slide height (EMU).
	SlideHeight int64
}

// DetectSlideBoundsOverflow checks whether a JSON-authored shape's center
// falls outside the slide rectangle. A center-based threshold is used instead
// of corner-based to eliminate 1-EMU rounding false positives: a shape whose
// corner is 1 EMU past the edge but whose center is inside is not flagged.
//
// Shapes tagged with role "background" or "decor" are skipped — they are
// decorative and intentionally placed at the edges or off-slide.
//
// Returns nil when the shape is within bounds or excluded by role.
func DetectSlideBoundsOverflow(input BoundsCheckInput) *patterns.FitFinding {
	// Skip decorative shapes.
	if input.Role == "background" || input.Role == "decor" {
		return nil
	}

	// Guard against degenerate inputs.
	if input.SlideWidth <= 0 || input.SlideHeight <= 0 {
		return nil
	}

	// Compute center of the shape.
	centerX := input.X + input.CX/2
	centerY := input.Y + input.CY/2

	// Check if center is outside the slide rectangle [0, SlideWidth] x [0, SlideHeight].
	outsideX := centerX < 0 || centerX > input.SlideWidth
	outsideY := centerY < 0 || centerY > input.SlideHeight

	if !outsideX && !outsideY {
		return nil
	}

	var direction string
	switch {
	case outsideX && outsideY:
		direction = "horizontally and vertically"
	case outsideX:
		direction = "horizontally"
	default:
		direction = "vertically"
	}

	return &patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Pattern: "shape_grid",
			Path:    input.Path,
			Code:    patterns.ErrCodeSlideBoundsOverflow,
			Message: fmt.Sprintf(
				"shape center (%d, %d) EMU falls outside slide bounds (%d x %d) %s",
				centerX, centerY,
				input.SlideWidth, input.SlideHeight,
				direction,
			),
			Fix: &patterns.FixSuggestion{Kind: "reposition_shape"},
		},
		Action: "shrink_or_split",
		Measured: &patterns.Extent{
			WidthEMU:  input.CX,
			HeightEMU: input.CY,
		},
		Allowed: &patterns.Extent{
			WidthEMU:  input.SlideWidth,
			HeightEMU: input.SlideHeight,
		},
	}
}
