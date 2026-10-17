package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/types"
)

// minTwoColumnGutterEMU is 0.3 inches. A few bundled templates have a native
// 0.167-inch gap, too tight for asymmetric content to remain clearly separated.
const minTwoColumnGutterEMU = layout.MinTwoColumnGutterEMU

// deriveTwoColumnGeometry changes only the slide's cloned body transforms, so
// the title, chrome, fills, and typography keep the source template's style.
// The native outside edges and any larger native gutter remain unchanged.
func deriveTwoColumnGeometry(slide *slideXML, leftPercent int, stacked ...bool) error {
	if leftPercent <= 0 || leftPercent >= 100 {
		return fmt.Errorf("derived two-column split must be between 1 and 99, got %d", leftPercent)
	}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	indices := buildPlaceholderMap(shapes)
	firstIdx, hasFirst := indices["body"]
	secondIdx, hasSecond := indices["body_2"]
	if !hasFirst || !hasSecond {
		return fmt.Errorf("derived two-column layout requires body and body_2 placeholders")
	}
	first, second := &shapes[firstIdx], &shapes[secondIdx]
	if !sideBySideBodies(first, second) {
		return fmt.Errorf("derived two-column layout requires side-by-side body placeholders with resolved bounds")
	}
	left, right := first, second
	if second.ShapeProperties.Transform.Offset.X < first.ShapeProperties.Transform.Offset.X {
		left, right = second, first
	}
	leftTransform := left.ShapeProperties.Transform
	rightTransform := right.ShapeProperties.Transform
	leftBox := types.BoundingBox{X: leftTransform.Offset.X, Y: leftTransform.Offset.Y, Width: leftTransform.Extent.CX, Height: leftTransform.Extent.CY}
	rightBox := types.BoundingBox{X: rightTransform.Offset.X, Y: rightTransform.Offset.Y, Width: rightTransform.Extent.CX, Height: rightTransform.Extent.CY}
	var err error
	if len(stacked) > 0 && stacked[0] {
		leftBox, rightBox, err = layout.DeriveStackedBounds(leftBox, rightBox, leftPercent)
	} else {
		leftBox, rightBox, err = layout.DeriveColumnBounds(leftBox, rightBox, leftPercent)
	}
	if err != nil {
		return err
	}
	leftTransform.Offset.X, leftTransform.Offset.Y = leftBox.X, leftBox.Y
	leftTransform.Extent.CX, leftTransform.Extent.CY = leftBox.Width, leftBox.Height
	rightTransform.Offset.X, rightTransform.Offset.Y = rightBox.X, rightBox.Y
	rightTransform.Extent.CX, rightTransform.Extent.CY = rightBox.Width, rightBox.Height
	return nil
}
