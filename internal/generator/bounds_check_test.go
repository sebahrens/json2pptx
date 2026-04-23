package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

const (
	testSlideWidth  int64 = 12192000 // standard 16:9 width
	testSlideHeight int64 = 6858000  // standard 16:9 height
)

func TestDetectSlideBoundsOverflow_CornerOffEdgeCenterInside(t *testing.T) {
	// Shape with corner 1 EMU off-edge but center inside -> no finding.
	// Shape starts at X = -1 (1 EMU left of slide edge), width = 1000000.
	// Center = -1 + 1000000/2 = 499999, which is inside [0, slideWidth].
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           -1,
		Y:           0,
		CX:          1000000,
		CY:          500000,
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding when corner is 1 EMU off-edge but center is inside, got: %s", finding.Message)
	}
}

func TestDetectSlideBoundsOverflow_CenterOutside(t *testing.T) {
	// Shape with center outside slide rect -> finding emitted.
	// Shape at X = testSlideWidth (right edge), width = 2000000.
	// Center = testSlideWidth + 1000000, clearly outside.
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           testSlideWidth,
		Y:           0,
		CX:          2000000,
		CY:          500000,
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding == nil {
		t.Fatal("expected finding when shape center is outside slide bounds")
	}

	if finding.Code != patterns.ErrCodeSlideBoundsOverflow {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeSlideBoundsOverflow, finding.Code)
	}
	if finding.Action != "shrink_or_split" {
		t.Errorf("expected action shrink_or_split, got %q", finding.Action)
	}
}

func TestDetectSlideBoundsOverflow_DecorRoleSkipped(t *testing.T) {
	// role: decor shape fully off-slide -> skipped.
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           -5000000,
		Y:           -5000000,
		CX:          1000000,
		CY:          1000000,
		Role:        "decor",
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding for role=decor, got: %s", finding.Message)
	}
}

func TestDetectSlideBoundsOverflow_BackgroundRoleSkipped(t *testing.T) {
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           -5000000,
		Y:           -5000000,
		CX:          1000000,
		CY:          1000000,
		Role:        "background",
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding for role=background, got: %s", finding.Message)
	}
}

func TestDetectSlideBoundsOverflow_CenterInsideNormal(t *testing.T) {
	// A normally positioned shape entirely within slide bounds.
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           1000000,
		Y:           1000000,
		CX:          2000000,
		CY:          1000000,
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding for shape entirely within bounds, got: %s", finding.Message)
	}
}

func TestDetectSlideBoundsOverflow_CenterOutsideVertically(t *testing.T) {
	// Shape pushed below the slide bottom.
	input := BoundsCheckInput{
		SlideIndex:  1,
		Path:        "slides[1].shape_grid.rows[2].cells[0]",
		X:           1000000,
		Y:           testSlideHeight,
		CX:          2000000,
		CY:          2000000,
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding == nil {
		t.Fatal("expected finding when shape center is below slide bottom")
	}
	if finding.Code != patterns.ErrCodeSlideBoundsOverflow {
		t.Errorf("expected code %q, got %q", patterns.ErrCodeSlideBoundsOverflow, finding.Code)
	}
}

func TestDetectSlideBoundsOverflow_NoRoleTreatedAsContent(t *testing.T) {
	// Empty role (default) = content = checked.
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           -5000000,
		Y:           0,
		CX:          1000000,
		CY:          500000,
		Role:        "", // no role = content
		SlideWidth:  testSlideWidth,
		SlideHeight: testSlideHeight,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding == nil {
		t.Fatal("expected finding for shape with no role and center outside bounds")
	}
}

func TestDetectSlideBoundsOverflow_ZeroSlideDimensions(t *testing.T) {
	// Degenerate: zero slide dimensions should not panic.
	input := BoundsCheckInput{
		SlideIndex:  0,
		Path:        "slides[0].shape_grid.rows[0].cells[0]",
		X:           1000,
		Y:           1000,
		CX:          500,
		CY:          500,
		SlideWidth:  0,
		SlideHeight: 0,
	}

	finding := DetectSlideBoundsOverflow(input)
	if finding != nil {
		t.Errorf("expected no finding for zero slide dimensions (guard), got: %s", finding.Message)
	}
}
