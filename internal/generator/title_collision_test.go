package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestDetectTitleCollision(t *testing.T) {
	// Title chrome occupies the top band: bottom edge at ~1.4 inches.
	const titleBottom int64 = 1280160

	t.Run("intrusion into title band at warn", func(t *testing.T) {
		finding := DetectTitleCollision(TitleCollisionInput{
			SlideIndex:          0,
			Path:                "/slides/0/shape_grid/rows/0/cells/0",
			ShapeX:              457200,
			ShapeY:              400000, // top above title bottom
			ShapeCX:             4000000,
			ShapeCY:             2000000, // bottom = 2400000, well below title bottom
			TitleBottom:         titleBottom,
			LayoutDeclaresTitle: true,
			StrictFit:           "warn",
		})

		if finding == nil {
			t.Fatal("expected finding, got nil")
		}
		if finding.Code != patterns.ErrCodeTitleCollision {
			t.Errorf("Code = %q, want %q", finding.Code, patterns.ErrCodeTitleCollision)
		}
		if finding.Action != "review" {
			t.Errorf("Action = %q, want %q", finding.Action, "review")
		}
		if finding.Fix == nil || finding.Fix.Kind != "reposition_shape" {
			t.Errorf("Fix.Kind = %v, want reposition_shape", finding.Fix)
		}
		// Intrusion = titleBottom - ShapeY = 1280160 - 400000 = 880160.
		wantIntrusion := titleBottom - 400000
		if finding.Allowed == nil {
			t.Fatal("expected Allowed extent")
		}
		// available height below title = shapeBottom - titleBottom.
		if got := finding.Allowed.HeightEMU; got != (400000+2000000)-titleBottom {
			t.Errorf("Allowed.HeightEMU = %d, want %d", got, (400000+2000000)-titleBottom)
		}
		_ = wantIntrusion
	})

	t.Run("strict mode promotes to refuse", func(t *testing.T) {
		finding := DetectTitleCollision(TitleCollisionInput{
			Path:                "/slides/0/shape_grid/rows/0/cells/0",
			ShapeX:              457200,
			ShapeY:              400000,
			ShapeCX:             4000000,
			ShapeCY:             2000000,
			TitleBottom:         titleBottom,
			LayoutDeclaresTitle: true,
			StrictFit:           "strict",
		})
		if finding == nil {
			t.Fatal("expected finding, got nil")
		}
		if finding.Action != "refuse" {
			t.Errorf("Action = %q, want refuse", finding.Action)
		}
	})

	t.Run("off mode suppresses", func(t *testing.T) {
		if f := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: 400000, ShapeCX: 4000000, ShapeCY: 2000000,
			TitleBottom: titleBottom, LayoutDeclaresTitle: true, StrictFit: "off",
		}); f != nil {
			t.Errorf("off mode: expected nil, got %+v", f)
		}
	})

	t.Run("no title zone suppresses", func(t *testing.T) {
		if f := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: 400000, ShapeCX: 4000000, ShapeCY: 2000000,
			TitleBottom: titleBottom, LayoutDeclaresTitle: false, StrictFit: "warn",
		}); f != nil {
			t.Errorf("no title zone: expected nil, got %+v", f)
		}
	})

	t.Run("shape below title does not fire", func(t *testing.T) {
		if f := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: titleBottom, ShapeCX: 4000000, ShapeCY: 2000000,
			TitleBottom: titleBottom, LayoutDeclaresTitle: true, StrictFit: "warn",
		}); f != nil {
			t.Errorf("shape at title bottom: expected nil, got %+v", f)
		}
		if f := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: titleBottom + 100, ShapeCX: 4000000, ShapeCY: 2000000,
			TitleBottom: titleBottom, LayoutDeclaresTitle: true, StrictFit: "warn",
		}); f != nil {
			t.Errorf("shape below title bottom: expected nil, got %+v", f)
		}
	})

	t.Run("decorative shapes skipped", func(t *testing.T) {
		for _, role := range []string{"background", "decor"} {
			if f := DetectTitleCollision(TitleCollisionInput{
				ShapeX: 457200, ShapeY: 400000, ShapeCX: 4000000, ShapeCY: 2000000,
				Role: role, TitleBottom: titleBottom, LayoutDeclaresTitle: true, StrictFit: "warn",
			}); f != nil {
				t.Errorf("role %q: expected nil, got %+v", role, f)
			}
		}
	})

	t.Run("fully-inside-title-band intrusion equals shape height", func(t *testing.T) {
		// Shape entirely within the title band: top 200000, bottom 600000, both
		// above titleBottom. Intrusion should equal ShapeCY (400000).
		finding := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: 200000, ShapeCX: 4000000, ShapeCY: 400000,
			TitleBottom: titleBottom, LayoutDeclaresTitle: true, StrictFit: "warn",
		})
		if finding == nil {
			t.Fatal("expected finding, got nil")
		}
		if finding.Measured == nil || finding.Measured.HeightEMU != 400000 {
			t.Errorf("Measured.HeightEMU = %v, want 400000", finding.Measured)
		}
	})

	t.Run("degenerate inputs suppressed", func(t *testing.T) {
		if f := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: 400000, ShapeCX: 0, ShapeCY: 2000000,
			TitleBottom: titleBottom, LayoutDeclaresTitle: true, StrictFit: "warn",
		}); f != nil {
			t.Errorf("zero width: expected nil, got %+v", f)
		}
		if f := DetectTitleCollision(TitleCollisionInput{
			ShapeX: 457200, ShapeY: 400000, ShapeCX: 4000000, ShapeCY: 2000000,
			TitleBottom: 0, LayoutDeclaresTitle: true, StrictFit: "warn",
		}); f != nil {
			t.Errorf("zero title bottom: expected nil, got %+v", f)
		}
	})
}
