package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
)

func TestModernFullBleedImageLeavesTitleVisible(t *testing.T) {
	tmp := t.TempDir()
	imagePath := filepath.Join(tmp, "solid.png")
	solid := image.NewRGBA(image.Rect(0, 0, 1200, 675))
	draw.Draw(solid, solid.Bounds(), &image.Uniform{C: color.RGBA{R: 19, G: 78, B: 150, A: 255}}, image.Point{}, draw.Src)
	f, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, solid); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	imageItem := ContentItem{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: imagePath, Alt: "solid blue background"}}
	output := filepath.Join(tmp, "image-title.pptx")
	_, err = Generate(context.Background(), GenerationRequest{
		TemplatePath:          "../../templates/modern.pptx",
		OutputPath:            output,
		AllowedImagePaths:     []string{tmp},
		ExcludeTemplateSlides: true,
		Slides: []SlideSpec{
			{LayoutID: "slideLayout1", Content: []ContentItem{imageItem}},
			{LayoutID: "slideLayout1", Content: []ContentItem{{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "VISIBLE COVER"}, imageItem}},
			{LayoutID: "slideLayout6", Content: []ContentItem{imageItem}},
			{LayoutID: "slideLayout6", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "VISIBLE CLOSING"}, {PlaceholderID: "subtitle", Type: ContentText, Value: "Readable subtitle"}, imageItem}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	z, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, slideNum := range []int{2, 4} {
		data := zipSlideXML(t, &z.Reader, slideNum)
		pic := bytes.Index(data, []byte("<p:pic>"))
		title := bytes.Index(data, []byte(`name="title"`))
		if pic < 0 || title < 0 || pic >= title {
			t.Errorf("slide %d picture/title order = %d/%d, want picture below title", slideNum, pic, title)
		}
		if slideNum == 4 {
			subtitle := bytes.Index(data, []byte(`name="subtitle"`))
			if subtitle < 0 || pic >= subtitle {
				t.Errorf("closing slide picture/subtitle order = %d/%d, want picture below subtitle", pic, subtitle)
			}
		}
	}

	if ok, missing := render.DependencyStatus(); !ok {
		t.Logf("pixel check skipped: missing render dependencies %v", missing)
		return
	}
	pngs, cleanup, err := render.DeckPNGs(output, 72, true)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if len(pngs) != 4 {
		t.Fatalf("rendered %d slides, want 4", len(pngs))
	}
	// Paired slides differ only by populated title/subtitle text. A picture
	// painted above them makes the paired pixels identical in these title bands.
	for _, tc := range []struct {
		name          string
		control, text int
		yMin, yMax    float64
	}{
		{"cover", 0, 1, 0.30, 0.70},
		{"closing", 2, 3, 0.05, 0.47},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := decodePNGFile(t, pngs[tc.control])
			after := decodePNGFile(t, pngs[tc.text])
			bounds := before.Bounds()
			if bounds != after.Bounds() {
				t.Fatalf("render dimensions differ: %v vs %v", bounds, after.Bounds())
			}
			diff := 0
			for y := int(float64(bounds.Dy()) * tc.yMin); y < int(float64(bounds.Dy())*tc.yMax); y++ {
				for x := bounds.Dx() / 8; x < bounds.Dx()*7/8; x++ {
					if before.At(x, y) != after.At(x, y) {
						diff++
					}
				}
			}
			if diff < 200 {
				t.Errorf("only %d title-band pixels changed; full-bleed image may cover the text", diff)
			}
		})
	}
}

func zipSlideXML(t *testing.T, z *zip.Reader, slideNum int) []byte {
	t.Helper()
	name := SlidePath(slideNum)
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	t.Fatalf("%s not found in generated deck", name)
	return nil
}

func decodePNGFile(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestNativeSVGImageBelowTextAndGridIconAbove(t *testing.T) {
	ctx := &singlePassContext{SVGContext: SVGContext{nativeSVGInserts: make(map[int][]nativeSVGInsert)}}
	slide := []byte(`<p:sld><p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="2" name="title"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`)
	inserts := []nativeSVGInsert{
		{pngRelID: "rId2", svgRelID: "rId3", extentCX: 100, extentCY: 100, behindText: true},
		{pngRelID: "rId4", svgRelID: "rId5", extentCX: 50, extentCY: 50},
	}
	got, err := ctx.insertNativeSVGPics(1, slide, inserts)
	if err != nil {
		t.Fatal(err)
	}
	back := bytes.Index(got, []byte(`r:embed="rId2"`))
	title := bytes.Index(got, []byte(`name="title"`))
	front := bytes.Index(got, []byte(`r:embed="rId4"`))
	if back < 0 || title < 0 || front < 0 || !(back < title && title < front) {
		t.Errorf("native SVG order back/title/front = %d/%d/%d", back, title, front)
	}
}

func TestModernNativeSVGImageBelowTitle(t *testing.T) {
	tmp := t.TempDir()
	imagePath := filepath.Join(tmp, "solid.svg")
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="675" viewBox="0 0 1200 675"><rect width="1200" height="675" fill="#134E96"/></svg>`)
	if err := os.WriteFile(imagePath, svg, 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(tmp, "native-image-title.pptx")
	_, err := Generate(context.Background(), GenerationRequest{
		TemplatePath:          "../../templates/modern.pptx",
		OutputPath:            output,
		AllowedImagePaths:     []string{tmp},
		ExcludeTemplateSlides: true,
		SVGStrategy:           "native",
		Slides: []SlideSpec{{
			LayoutID: "slideLayout1",
			Content: []ContentItem{
				{PlaceholderID: "title", Type: ContentTitleSlideTitle, Value: "VISIBLE SVG COVER"},
				{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: imagePath, Alt: "solid blue background"}},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data := zipSlideXML(t, &z.Reader, 1)
	pic := bytes.Index(data, []byte("<p:pic>"))
	title := bytes.Index(data, []byte(`name="title"`))
	if pic < 0 || title < 0 || pic >= title {
		t.Errorf("native SVG picture/title order = %d/%d, want picture below title", pic, title)
	}
}
