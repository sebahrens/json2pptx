package shapegrid

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// TestIconOverlayBounds_LeftCappedByWidth covers go-slide-creator-5lbo: a
// tall card must not hand 60% of its width to a left icon.
func TestIconOverlayBounds_LeftCappedByWidth(t *testing.T) {
	shape := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 1500000}
	l := iconOverlayBounds(&IconSpec{Position: "left"}, shape, true)
	if maxW := int64(float64(shape.CX) * leftIconMaxWidthFrac); l.Bounds.CX > maxW {
		t.Errorf("left icon width %d exceeds cap %d", l.Bounds.CX, maxW)
	}
	if want := defaultTextInsetLREMU + l.Bounds.CX + 2*iconOverlayGapEMU; l.TextInsets[0] != want {
		t.Errorf("left inset = %d, want default+icon+padding %d", l.TextInsets[0], want)
	}
}

// TestIconOverlayBounds_TopCappedOnLandscape: short wide cards keep most of
// their height for text; square cards and explicit scales are unchanged.
func TestIconOverlayBounds_TopCappedOnLandscape(t *testing.T) {
	wide := pptx.RectEmu{CX: 3000000, CY: 1000000}
	top := iconOverlayBounds(&IconSpec{Position: "top"}, wide, true)
	if maxH := int64(float64(wide.CY) * topIconMaxHeightFrac); top.Bounds.CY > maxH {
		t.Errorf("top icon height %d exceeds cap %d on landscape shape", top.Bounds.CY, maxH)
	}
	explicit := iconOverlayBounds(&IconSpec{Position: "top", Scale: 0.6}, wide, true)
	if explicit.Bounds.CY != 600000 {
		t.Errorf("explicit scale must not be capped, got %d", explicit.Bounds.CY)
	}
	square := iconOverlayBounds(&IconSpec{Position: "top"}, pptx.RectEmu{CX: 1000000, CY: 1000000}, true)
	if square.Bounds.CY != 600000 {
		t.Errorf("square shape keeps 0.6 icon, got %d", square.Bounds.CY)
	}
}

// go-slide-creator-jzb1: buildTextBody writes all four insets as soon as one
// is non-zero, so an icon overlay that only set its own axis silently zeroed
// PowerPoint's default padding on the other three sides — card text then sat
// flush against the fill edge. Every overlay position must return complete
// insets: defaults everywhere, plus the icon reservation on its own axis.
func TestIconOverlayBounds_KeepsDefaultInsetsOnNonIconSides(t *testing.T) {
	card := pptx.RectEmu{CX: 3000000, CY: 1800000}

	t.Run("left", func(t *testing.T) {
		l := iconOverlayBounds(&IconSpec{Position: "left"}, card, true)
		if l.TextInsets[0] <= defaultTextInsetLREMU {
			t.Errorf("left inset %d must exceed the default %d (it reserves the icon)", l.TextInsets[0], defaultTextInsetLREMU)
		}
		if l.TextInsets[1] != defaultTextInsetTBEMU {
			t.Errorf("top inset = %d, want default %d", l.TextInsets[1], defaultTextInsetTBEMU)
		}
		if l.TextInsets[2] != defaultTextInsetLREMU {
			t.Errorf("right inset = %d, want default %d", l.TextInsets[2], defaultTextInsetLREMU)
		}
		if l.TextInsets[3] != defaultTextInsetTBEMU {
			t.Errorf("bottom inset = %d, want default %d", l.TextInsets[3], defaultTextInsetTBEMU)
		}
	})

	t.Run("top", func(t *testing.T) {
		l := iconOverlayBounds(&IconSpec{Position: "top"}, card, true)
		if l.TextInsets[1] <= defaultTextInsetTBEMU {
			t.Errorf("top inset %d must exceed the default %d (it reserves the icon)", l.TextInsets[1], defaultTextInsetTBEMU)
		}
		if l.TextInsets[0] != defaultTextInsetLREMU {
			t.Errorf("left inset = %d, want default %d", l.TextInsets[0], defaultTextInsetLREMU)
		}
		if l.TextInsets[2] != defaultTextInsetLREMU {
			t.Errorf("right inset = %d, want default %d", l.TextInsets[2], defaultTextInsetLREMU)
		}
		if l.TextInsets[3] != defaultTextInsetTBEMU {
			t.Errorf("bottom inset = %d, want default %d", l.TextInsets[3], defaultTextInsetTBEMU)
		}
	})

	t.Run("center keeps insets unset so the renderer default applies", func(t *testing.T) {
		l := iconOverlayBounds(&IconSpec{Position: "center"}, card, true)
		if l.TextInsets != [4]int64{} {
			t.Errorf("center overlay must not write insets at all, got %v", l.TextInsets)
		}
	})
}
