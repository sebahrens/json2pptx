package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestDetectFooterCollision(t *testing.T) {
	// Common slide dimensions: 10" x 7.5" in EMU.
	const (
		slideWidth  int64 = 9144000
		slideHeight int64 = 6858000
		// Footer at bottom 0.4 inches.
		footerY  int64 = 6493875 // slideHeight - footerCY
		footerCY int64 = 364125
	)

	t.Run("intrusion with declared footer at warn", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[2].cells[0]",
			ShapeX:               457200,
			ShapeY:               6000000,
			ShapeCX:              4000000,
			ShapeCY:              800000, // bottom = 6800000, well into footer area
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "warn",
		})

		if finding == nil {
			t.Fatal("expected finding, got nil")
		}
		if finding.Code != patterns.ErrCodeFooterCollision {
			t.Errorf("Code = %q, want %q", finding.Code, patterns.ErrCodeFooterCollision)
		}
		if finding.Action != "review" {
			t.Errorf("Action = %q, want %q", finding.Action, "review")
		}
		if finding.Fix == nil || finding.Fix.Kind != "reposition_shape" {
			t.Errorf("Fix.Kind = %v, want reposition_shape", finding.Fix)
		}
	})

	t.Run("intrusion with declared footer at strict", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[2].cells[0]",
			ShapeX:               457200,
			ShapeY:               6000000,
			ShapeCX:              4000000,
			ShapeCY:              800000,
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "strict",
		})

		if finding == nil {
			t.Fatal("expected finding, got nil")
		}
		if finding.Action != "refuse" {
			t.Errorf("Action = %q, want %q", finding.Action, "refuse")
		}
	})

	t.Run("intrusion without declared footer returns nil", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[2].cells[0]",
			ShapeX:               457200,
			ShapeY:               6000000,
			ShapeCX:              4000000,
			ShapeCY:              800000,
			LayoutDeclaresFooter: false, // no declared footer
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "warn",
		})

		if finding != nil {
			t.Errorf("expected nil finding when layout has no declared footer, got %+v", finding)
		}
	})

	t.Run("decorative role skipped", func(t *testing.T) {
		for _, role := range []string{"background", "decor"} {
			finding := DetectFooterCollision(FooterCollisionInput{
				SlideIndex:           0,
				Path:                 "slides[0].shape_grid.rows[0].cells[0]",
				ShapeX:               0,
				ShapeY:               6000000,
				ShapeCX:              slideWidth,
				ShapeCY:              800000,
				Role:                 role,
				LayoutDeclaresFooter: true,
				FooterY:              footerY,
				FooterCY:             footerCY,
				StrictFit:            "warn",
			})

			if finding != nil {
				t.Errorf("role=%q: expected nil finding for decorative shape, got %+v", role, finding)
			}
		}
	})

	t.Run("no collision when shape above footer", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[0].cells[0]",
			ShapeX:               457200,
			ShapeY:               1000000,
			ShapeCX:              4000000,
			ShapeCY:              2000000, // bottom = 3000000, well above footer
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "warn",
		})

		if finding != nil {
			t.Errorf("expected nil finding when shape is above footer, got %+v", finding)
		}
	})

	t.Run("off mode skips entirely", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[2].cells[0]",
			ShapeX:               457200,
			ShapeY:               6000000,
			ShapeCX:              4000000,
			ShapeCY:              800000,
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "off",
		})

		if finding != nil {
			t.Errorf("expected nil finding when StrictFit=off, got %+v", finding)
		}
	})

	t.Run("shape touching footer edge exactly returns nil", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[1].cells[0]",
			ShapeX:               457200,
			ShapeY:               5493875,
			ShapeCX:              4000000,
			ShapeCY:              1000000, // bottom = 6493875 = footerY exactly
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "warn",
		})

		if finding != nil {
			t.Errorf("expected nil finding when shape bottom == footer top, got %+v", finding)
		}
	})

	t.Run("shape 1 EMU into footer triggers finding", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[1].cells[0]",
			ShapeX:               457200,
			ShapeY:               5493875,
			ShapeCX:              4000000,
			ShapeCY:              1000001, // bottom = 6493876, 1 EMU into footer
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "warn",
		})

		if finding == nil {
			t.Fatal("expected finding for 1-EMU intrusion, got nil")
		}
	})

	t.Run("measured and allowed extents populated", func(t *testing.T) {
		finding := DetectFooterCollision(FooterCollisionInput{
			SlideIndex:           0,
			Path:                 "slides[0].shape_grid.rows[2].cells[0]",
			ShapeX:               457200,
			ShapeY:               6000000,
			ShapeCX:              4000000,
			ShapeCY:              800000,
			LayoutDeclaresFooter: true,
			FooterY:              footerY,
			FooterCY:             footerCY,
			StrictFit:            "warn",
		})

		if finding == nil {
			t.Fatal("expected finding, got nil")
		}
		if finding.Measured == nil {
			t.Fatal("Measured is nil")
		}
		if finding.Measured.WidthEMU != 4000000 || finding.Measured.HeightEMU != 800000 {
			t.Errorf("Measured = %+v, want {4000000, 800000}", finding.Measured)
		}
		if finding.Allowed == nil {
			t.Fatal("Allowed is nil")
		}
		// Allowed height = footerY - shapeY = 6493875 - 6000000 = 493875
		if finding.Allowed.HeightEMU != 493875 {
			t.Errorf("Allowed.HeightEMU = %d, want 493875", finding.Allowed.HeightEMU)
		}
	})

	t.Run("degenerate inputs return nil", func(t *testing.T) {
		cases := []struct {
			name     string
			shapeCX  int64
			shapeCY  int64
			footerCY int64
		}{
			{"zero shape width", 0, 800000, footerCY},
			{"zero shape height", 4000000, 0, footerCY},
			{"zero footer height", 4000000, 800000, 0},
		}
		for _, tc := range cases {
			finding := DetectFooterCollision(FooterCollisionInput{
				SlideIndex:           0,
				Path:                 "slides[0].shape_grid.rows[0].cells[0]",
				ShapeX:               457200,
				ShapeY:               6000000,
				ShapeCX:              tc.shapeCX,
				ShapeCY:              tc.shapeCY,
				LayoutDeclaresFooter: true,
				FooterY:              footerY,
				FooterCY:             tc.footerCY,
				StrictFit:            "warn",
			})
			if finding != nil {
				t.Errorf("%s: expected nil, got %+v", tc.name, finding)
			}
		}
	})
}
