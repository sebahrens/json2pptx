package generator

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestRegularImageCoversPlaceholderFrame(t *testing.T) {
	for _, tc := range []struct {
		name       string
		imageW     int
		imageH     int
		wantCropOn string
	}{
		{name: "wide source", imageW: 400, imageH: 100, wantCropOn: "horizontal"},
		{name: "tall source", imageW: 100, imageH: 400, wantCropOn: "vertical"},
		{name: "matching source", imageW: 200, imageH: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "image.png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, tc.imageW, tc.imageH))); err != nil {
				_ = f.Close()
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}

			frame := types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 300}
			ctx := newSinglePassContext("", nil, nil, false, nil)
			ctx.processRegularImage(1, path, "photo", frame, nil, 2)
			rels := ctx.slideRelUpdates[1]
			if len(rels) != 1 {
				t.Fatalf("media relationships = %d, want 1; warnings: %v", len(rels), ctx.warnings)
			}
			rel := rels[0]
			if rel.offsetX != frame.X || rel.offsetY != frame.Y || rel.extentCX != frame.Width || rel.extentCY != frame.Height {
				t.Errorf("picture frame = (%d,%d,%d,%d), want %+v", rel.offsetX, rel.offsetY, rel.extentCX, rel.extentCY, frame)
			}
			switch tc.wantCropOn {
			case "horizontal":
				if rel.crop == nil || rel.crop.L <= 0 || rel.crop.L != rel.crop.R || rel.crop.T != 0 || rel.crop.B != 0 {
					t.Errorf("wide source should crop left/right equally, got %+v", rel.crop)
				}
			case "vertical":
				if rel.crop == nil || rel.crop.T <= 0 || rel.crop.T != rel.crop.B || rel.crop.L != 0 || rel.crop.R != 0 {
					t.Errorf("tall source should crop top/bottom equally, got %+v", rel.crop)
				}
			default:
				if rel.crop != nil {
					t.Errorf("matching source should not crop, got %+v", rel.crop)
				}
			}
		})
	}
}
