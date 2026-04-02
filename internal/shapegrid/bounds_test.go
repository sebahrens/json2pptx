package shapegrid

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestBoundsFromPercentages_FullSlide(t *testing.T) {
	b := BoundsFromPercentages(0, 0, 100, 100, 0, 0)
	if b.X != 0 || b.Y != 0 {
		t.Errorf("expected origin at 0,0 got %d,%d", b.X, b.Y)
	}
	if b.CX != DefaultSlideWidthEMU || b.CY != DefaultSlideHeightEMU {
		t.Errorf("expected full slide dimensions, got %dx%d", b.CX, b.CY)
	}
}

func TestBoundsFromPercentages_CustomDimensions(t *testing.T) {
	// 4:3 dimensions: 9144000 x 6858000
	b := BoundsFromPercentages(0, 0, 100, 100, 9144000, 6858000)
	if b.CX != 9144000 || b.CY != 6858000 {
		t.Errorf("expected custom dimensions 9144000x6858000, got %dx%d", b.CX, b.CY)
	}
}

func TestBoundsFromPercentages_Zero(t *testing.T) {
	b := BoundsFromPercentages(0, 0, 0, 0, 0, 0)
	if b.X != 0 || b.Y != 0 || b.CX != 0 || b.CY != 0 {
		t.Errorf("expected all zeros, got %+v", b)
	}
}

func TestBoundsFromTitleAndFooter_Normal(t *testing.T) {
	title := pptx.RectEmu{X: 457200, Y: 274638, CX: 8229600, CY: 461963}
	footer := pptx.RectEmu{X: 457200, Y: 6356350, CX: 2895600, CY: 365125}
	b := BoundsFromTitleAndFooter(title, footer, 9, 0)
	if b.Y <= title.Y+title.CY {
		t.Error("grid top should be below title")
	}
	if b.Y+b.CY >= footer.Y {
		t.Error("grid bottom should be above footer")
	}
}

func TestBoundsFromTitleAndFooter_OverlappingTitleFooter(t *testing.T) {
	title := pptx.RectEmu{X: 0, Y: 0, CX: 12192000, CY: 5000000}
	footer := pptx.RectEmu{X: 0, Y: 4000000, CX: 12192000, CY: 1000000}
	b := BoundsFromTitleAndFooter(title, footer, 9, 0)
	// Height should be clamped to 0
	if b.CY != 0 {
		t.Errorf("expected 0 height for overlapping title/footer, got %d", b.CY)
	}
}

func TestBoundsFromTitleAndFooter_NoGap(t *testing.T) {
	title := pptx.RectEmu{X: 0, Y: 0, CX: 12192000, CY: 1000000}
	footer := pptx.RectEmu{X: 0, Y: 6000000, CX: 12192000, CY: 858000}
	b := BoundsFromTitleAndFooter(title, footer, 0, 0)
	if b.Y != 1000000 {
		t.Errorf("expected Y=1000000, got %d", b.Y)
	}
	if b.CY != 5000000 {
		t.Errorf("expected CY=5000000, got %d", b.CY)
	}
}

func TestBoundsFromTitleAndFooter_LargeGap(t *testing.T) {
	title := pptx.RectEmu{X: 0, Y: 0, CX: 12192000, CY: 1000000}
	footer := pptx.RectEmu{X: 0, Y: 6000000, CX: 12192000, CY: 858000}
	b := BoundsFromTitleAndFooter(title, footer, 36, 0) // 36pt gap
	gapEMU := int64(36 * 12700)
	if b.Y != 1000000+gapEMU {
		t.Errorf("expected Y=%d, got %d", 1000000+gapEMU, b.Y)
	}
}

func TestDefaultBounds_MatchesConstants(t *testing.T) {
	b := DefaultBounds(0, 0)
	expected := BoundsFromPercentages(
		DefaultGridBoundsPct[0],
		DefaultGridBoundsPct[1],
		DefaultGridBoundsPct[2],
		DefaultGridBoundsPct[3],
		0, 0,
	)
	if b != expected {
		t.Errorf("DefaultBounds() != BoundsFromPercentages(defaults): %+v vs %+v", b, expected)
	}
}

func TestBoundsFromTitleAndFooter_NoFooterMargin(t *testing.T) {
	// Simulate the no-footer case: footer.Y = slide bottom minus MinBottomMarginEMU
	title := pptx.RectEmu{X: 457200, Y: 274638, CX: 8229600, CY: 461963}
	footerY := DefaultSlideHeightEMU - MinBottomMarginEMU
	footer := pptx.RectEmu{X: 457200, Y: footerY, CX: 8229600, CY: 0}
	b := BoundsFromTitleAndFooter(title, footer, 9, 0)

	gapEMU := int64(9 * 12700)
	// Grid bottom must be at least MinBottomMarginEMU above slide bottom
	gridBottom := b.Y + b.CY
	margin := DefaultSlideHeightEMU - gridBottom
	if margin < MinBottomMarginEMU {
		t.Errorf("grid bottom too close to slide edge: margin=%d EMU, want >= %d", margin, MinBottomMarginEMU)
	}
	// Grid bottom should be footerY - gap
	expectedBottom := footerY - gapEMU
	if gridBottom != expectedBottom {
		t.Errorf("expected grid bottom at %d, got %d", expectedBottom, gridBottom)
	}
}

func TestBoundsFromTitleAndFooter_NarrowTitle(t *testing.T) {
	// Title is narrow (half slide width), but slide is full 16:9.
	// Grid width should expand to slideWidth - 2*title.X, not title.CX.
	title := pptx.RectEmu{X: 457200, Y: 274638, CX: 4000000, CY: 461963}
	footer := pptx.RectEmu{X: 457200, Y: 6356350, CX: 2895600, CY: 365125}
	b := BoundsFromTitleAndFooter(title, footer, 9, DefaultSlideWidthEMU)
	expectedCX := DefaultSlideWidthEMU - 2*title.X // 12192000 - 914400 = 11277600
	if b.CX != expectedCX {
		t.Errorf("expected grid width %d (slide-2*margin), got %d", expectedCX, b.CX)
	}
}

func TestBoundsFromTitleAndFooter_SlideWidthZero(t *testing.T) {
	// When slideWidth is 0, should fall back to title.CX
	title := pptx.RectEmu{X: 457200, Y: 274638, CX: 4000000, CY: 461963}
	footer := pptx.RectEmu{X: 457200, Y: 6356350, CX: 2895600, CY: 365125}
	b := BoundsFromTitleAndFooter(title, footer, 9, 0)
	if b.CX != title.CX {
		t.Errorf("expected grid width %d (title.CX fallback), got %d", title.CX, b.CX)
	}
}

func TestBoundsFromPlaceholder_PassThrough(t *testing.T) {
	input := pptx.RectEmu{X: 123, Y: 456, CX: 789, CY: 101112}
	b := BoundsFromPlaceholder(input)
	if b != input {
		t.Errorf("expected pass-through, got %+v", b)
	}
}
