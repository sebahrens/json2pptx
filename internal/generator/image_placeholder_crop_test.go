package generator

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
			ctx.processRegularImage(1, path, "photo", frame, nil, 2, "")
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

func TestNativeImageContainRetainsWholeSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
		want types.BoundingBox
	}{
		{"wide", 400, 100, types.BoundingBox{X: 100, Y: 312, Width: 300, Height: 75}},
		{"tall", 100, 400, types.BoundingBox{X: 212, Y: 200, Width: 75, Height: 300}},
		{"square", 200, 200, types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 300}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err = png.Encode(f, image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
			ctx := newSinglePassContext("", nil, nil, false, nil)
			ctx.processRegularImage(1, path, "source diagram", types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 300}, nil, 2, "contain")
			if len(ctx.slideRelUpdates[1]) != 1 || len(ctx.mediaFailures) != 0 {
				t.Fatalf("failed insertion: %v", ctx.mediaFailures)
			}
			rel := ctx.slideRelUpdates[1][0]
			got := types.BoundingBox{X: rel.offsetX, Y: rel.offsetY, Width: rel.extentCX, Height: rel.extentCY}
			if got != tc.want || rel.crop != nil {
				t.Fatalf("source cropped/distorted: bounds=%+v crop=%+v, want %+v", got, rel.crop, tc.want)
			}
		})
	}
}

func TestNativeImageFitFailureIsNotSilent(t *testing.T) {
	for _, tc := range []struct {
		name, fit string
		frame     types.BoundingBox
	}{
		{"unknown fit", "stretch", types.BoundingBox{Width: 100, Height: 100}},
		{"zero frame", "contain", types.BoundingBox{Width: 0, Height: 100}},
		{"missing contain source", "contain", types.BoundingBox{Width: 100, Height: 100}},
		{"missing cover source", "cover", types.BoundingBox{Width: 100, Height: 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := newSinglePassContext("", nil, nil, false, nil)
			ctx.processRegularImage(1, filepath.Join(t.TempDir(), "missing.png"), "source", tc.frame, nil, 2, tc.fit)
			if len(ctx.slideRelUpdates[1]) != 0 || len(ctx.mediaFailures) != 1 || ctx.mediaFailures[0].Fallback != "skipped" {
				t.Fatalf("silent or inserted failed image: %+v", ctx.mediaFailures)
			}
			if ctx.mediaFailures[0].SlideNum != 1 || ctx.mediaFailures[0].ContentType != "image" || ctx.mediaFailures[0].Reason == "" {
				t.Fatalf("failure attribution lost: %+v", ctx.mediaFailures)
			}
			if tc.fit == "stretch" && !strings.Contains(ctx.mediaFailures[0].Reason, "invalid image fit") {
				t.Fatalf("unknown fit masked by source read failure: %+v", ctx.mediaFailures)
			}
		})
	}
}

func TestNativeImageFitSVGParity(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("testdata", "svg_fixtures", "square_500x500.svg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range []SVGConversionStrategy{SVGStrategyNative, SVGStrategyPNG} {
		for _, fit := range []string{"cover", "contain"} {
			t.Run(string(strategy)+"/"+fit, func(t *testing.T) {
				ctx := newSinglePassContext("", nil, nil, false, nil)
				ctx.ctx = context.Background()
				ctx.svgConverter = NewSVGConverterWithConfig(SVGConfig{Strategy: strategy})
				if !ctx.svgConverter.IsPNGAvailable() {
					t.Skip("SVG rasterizer unavailable")
				}
				t.Cleanup(func() {
					for _, cleanup := range ctx.svgCleanupFuncs {
						cleanup()
					}
				})
				ctx.processSVGImage(1, path, "Existing square SVG", types.BoundingBox{X: 100, Y: 200, Width: 300, Height: 100}, nil, 2, fit)
				if len(ctx.mediaFailures) > 0 {
					t.Fatalf("SVG failure: %+v", ctx.mediaFailures)
				}
				var x, y, w, h int64
				cropped := false
				if strategy == SVGStrategyNative {
					if len(ctx.nativeSVGInserts[1]) != 1 {
						t.Fatalf("native SVG missing: %v", ctx.warnings)
					}
					r := ctx.nativeSVGInserts[1][0]
					x, y, w, h = r.offsetX, r.offsetY, r.extentCX, r.extentCY
					cropped = r.crop != nil
				} else {
					if len(ctx.slideRelUpdates[1]) != 1 {
						t.Fatalf("raster SVG missing: %v", ctx.warnings)
					}
					r := ctx.slideRelUpdates[1][0]
					x, y, w, h = r.offsetX, r.offsetY, r.extentCX, r.extentCY
					cropped = r.crop != nil
				}
				if fit == "contain" {
					if x != 200 || y != 200 || w != 100 || h != 100 || cropped {
						t.Fatalf("contained SVG cropped/distorted: %d,%d %dx%d crop=%v", x, y, w, h, cropped)
					}
				} else {
					if x != 100 || y != 200 || w != 300 || h != 100 || !cropped {
						t.Fatalf("default cover changed: %d,%d %dx%d crop=%v", x, y, w, h, cropped)
					}
				}
			})
		}
	}
}
