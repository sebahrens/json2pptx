package layout

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// MinTwoColumnGutterEMU is 0.3 inches. Derived layouts never narrow a larger
// gutter already supplied by the template.
const MinTwoColumnGutterEMU int64 = 274320

// DeriveColumnBounds applies a left/right percentage to two physical columns
// while preserving their outside edges, vertical bounds, and template gutter.
// The arguments must be in left-to-right order.
func DeriveColumnBounds(left, right types.BoundingBox, leftPercent int) (types.BoundingBox, types.BoundingBox, error) {
	if leftPercent <= 0 || leftPercent >= 100 {
		return left, right, fmt.Errorf("derived two-column split must be between 1 and 99, got %d", leftPercent)
	}
	if left.Width <= 0 || right.Width <= 0 || left.Height <= 0 || right.Height <= 0 || left.X+left.Width > right.X {
		return left, right, fmt.Errorf("derived two-column layout requires side-by-side body placeholders with resolved bounds")
	}
	top := max(left.Y, right.Y)
	bottom := min(left.Y+left.Height, right.Y+right.Height)
	if bottom-top < min(left.Height, right.Height)/2 {
		return left, right, fmt.Errorf("derived two-column layout requires side-by-side body placeholders with resolved bounds")
	}
	gutter := max(MinTwoColumnGutterEMU, right.X-left.X-left.Width)
	usable := right.X + right.Width - left.X - gutter
	if usable < 2 {
		return left, right, fmt.Errorf("derived two-column layout has no usable width after %d EMU gutter", gutter)
	}
	leftWidth := usable * int64(leftPercent) / 100
	if leftWidth <= 0 || leftWidth >= usable {
		return left, right, fmt.Errorf("derived two-column split %d leaves an empty column", leftPercent)
	}
	left.Width = leftWidth
	right.X = left.X + leftWidth + gutter
	right.Width = usable - leftWidth
	return left, right, nil
}

// DeriveStackedBounds reuses the native body frame, with a minimum chrome gutter.
func DeriveStackedBounds(first, second types.BoundingBox, topPercent int) (types.BoundingBox, types.BoundingBox, error) {
	if topPercent <= 0 || topPercent >= 100 {
		return first, second, fmt.Errorf("stacked split must be between 1 and 99")
	}
	x, y := min(first.X, second.X), min(first.Y, second.Y)
	w, h := max(first.X+first.Width, second.X+second.Width)-x, max(first.Y+first.Height, second.Y+second.Height)-y
	if w <= 0 || h <= MinTwoColumnGutterEMU+1 {
		return first, second, fmt.Errorf("stacked layout has no usable body after its gutter")
	}
	usable := h - MinTwoColumnGutterEMU
	topH := usable * int64(topPercent) / 100
	return types.BoundingBox{X: x, Y: y, Width: w, Height: topH}, types.BoundingBox{X: x, Y: y + topH + MinTwoColumnGutterEMU, Width: w, Height: usable - topH}, nil
}
