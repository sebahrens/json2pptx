package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"path/filepath"
	"testing"
)

func TestGridPicturesUsePhotoFooterProtection(t *testing.T) {
	pngPath := writeTestPNG(t, t.TempDir(), "photo.png", 1600, 900)
	svgPath := writeGridTestSVG(t, `viewBox="0 0 1600 900"`)
	for _, tc := range []struct {
		name, path, strategy string
		y, h                 int64
		suppress             bool
	}{
		{"raster full", pngPath, "native", 0, 6858000, true},
		{"raster contained", pngPath, "native", 1000000, 3000000, false},
		{"SVG native full", svgPath, "native", 0, 6858000, true},
		{"SVG fallback full", svgPath, "png", 0, 6858000, true},
		{"SVG contained", svgPath, "native", 1000000, 3000000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if IsSVGFile(tc.path) && !NewSVGConverter().IsPNGAvailable() {
				t.Skip("SVG raster converter unavailable")
			}
			output := filepath.Join(t.TempDir(), "photo.pptx")
			result, err := Generate(context.Background(), GenerationRequest{
				TemplatePath: "../../templates/midnight-blue.pptx", OutputPath: output, ExcludeTemplateSlides: true,
				AllowedImagePaths: []string{filepath.Dir(tc.path)}, SVGStrategy: tc.strategy,
				Footer: &FooterConfig{Enabled: true, LeftText: "Picture chrome sentinel"},
				Slides: []SlideSpec{{LayoutID: "slideLayout2", ImageInserts: []ImageInsert{{Path: tc.path, Fit: "cover", OffsetY: tc.y, ExtentCX: 12192000, ExtentCY: tc.h}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.OpenReader(output)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			slide := zipSlideXML(t, &z.Reader, 1)
			if present := bytes.Contains(slide, []byte("Picture chrome sentinel")); present == tc.suppress {
				t.Fatalf("footer present=%v suppress=%v", present, tc.suppress)
			}
			if flagged := hasFinding(result.FitFindings, patterns.ErrCodeChromeOverImage, ""); flagged != tc.suppress {
				t.Fatalf("photo footer diagnostic=%v want%v", flagged, tc.suppress)
			}
		})
	}
}
