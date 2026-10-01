package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestImageSlideUsesFullWidthCanvasOnEveryTemplate pins
// go-slide-creator-f0l85 / 2hkgy / dk5sk across every bundled template (plus
// local p-style): a titled slide_type "image" slide — whether the picture is
// authored to placeholder_id "image" or "body" — lands on a layout with a title
// placeholder and either a picture placeholder or exactly one body (never a
// column of Two Content, never a title-less Statement layout), keeps its
// title, renders the picture, and in a body placeholder keeps the whole
// picture (srcRect crop under 30% per axis).
func TestImageSlideUsesFullWidthCanvasOnEveryTemplate(t *testing.T) {
	tmp := t.TempDir()
	portrait := writeImageSlidePNG(t, tmp, "portrait.png", 600, 900)
	landscape := writeImageSlidePNG(t, tmp, "landscape.png", 1600, 900)
	for _, tpl := range testutil.AllTestTemplateNames() {
		t.Run(tpl, func(t *testing.T) {
			a := loadTemplateAnalysis(t, tpl)
			bullets0 := []string{"Revenue EUR 21M", "Margin flat at 68%"}
			title0, title1, title2 := "Revenue grew 17%", "The Rotterdam hub runs at 92% of capacity", "Portrait photo on an image slide"
			in := &PresentationInput{Template: tpl, OutputFilename: "image-slides.pptx", Slides: []SlideInput{
				{LayoutID: "content", Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &title0},
					{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets0},
				}},
				{SlideType: "image", Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &title1},
					{PlaceholderID: "image", Type: "image", ImageValue: &ImageInput{Path: landscape, Alt: "hub"}},
				}},
				{SlideType: "image", Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", TextValue: &title2},
					{PlaceholderID: "body", Type: "image", ImageValue: &ImageInput{Path: portrait, Alt: "portrait"}},
				}},
			}}
			applyDefaults(in)
			res, cleanup, err := RunPresentation(context.Background(), in, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
				AllowedImagePaths: []string{tmp},
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range res.GenResult.FitFindings {
				if f.Code == patterns.ErrCodeContentDropped {
					t.Errorf("%s at %s: %s", f.Code, f.Path, f.Message)
				}
			}
			byID := map[string]types.LayoutMetadata{}
			for _, l := range a.Layouts {
				byID[l.ID] = l
			}
			for i, title := range map[int]string{1: title1, 2: title2} {
				layout, ok := byID[res.SlideSpecs[i].LayoutID]
				if !ok {
					continue // synthesized layout: no template metadata to judge
				}
				hasTitle, hasPic, bodies := false, false, 0
				for _, ph := range layout.Placeholders {
					switch {
					case ph.Type == types.PlaceholderTitle:
						hasTitle = true
					case ph.Type == types.PlaceholderImage:
						hasPic = true
					case (ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent) && !types.IsDisclosurePlaceholder(ph):
						bodies++
					}
				}
				if !hasTitle || (!hasPic && bodies != 1) {
					t.Errorf("slide %d on %q (title=%v pic=%v bodies=%d): want a titled full-width or picture layout", i+1, layout.Name, hasTitle, hasPic, bodies)
				}
				slide := readDeckSlide(t, res.OutputPath, i+1)
				if !bytes.Contains(slide, []byte("<p:pic>")) {
					t.Errorf("slide %d on %q: picture not rendered", i+1, layout.Name)
				}
				if !bytes.Contains(slide, []byte(title)) {
					t.Errorf("slide %d on %q: title %q dropped", i+1, layout.Name, title)
				}
				if !hasPic {
					if v, h := slideSrcRect(slide); v >= 30000 || h >= 30000 {
						t.Errorf("slide %d on %q: body-placeholder picture cropped t+b=%d l+r=%d", i+1, layout.Name, v, h)
					}
				}
			}
		})
	}
}

func writeImageSlidePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func readDeckSlide(t *testing.T, pptxPath string, n int) []byte {
	t.Helper()
	z, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	want := fmt.Sprintf("ppt/slides/slide%d.xml", n)
	for _, f := range z.File {
		if f.Name != want {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	t.Fatalf("%s not in deck", want)
	return nil
}

var deckSrcRectRE = regexp.MustCompile(`<a:srcRect([^/]*)/>`)
var deckSrcRectAttrRE = regexp.MustCompile(`([ltrb])="(-?\d+)"`)

func slideSrcRect(slide []byte) (vertical, horizontal int) {
	m := deckSrcRectRE.FindSubmatch(slide)
	if m == nil {
		return 0, 0
	}
	for _, a := range deckSrcRectAttrRE.FindAllSubmatch(m[1], -1) {
		v, _ := strconv.Atoi(string(a[2]))
		if a[1][0] == 't' || a[1][0] == 'b' {
			vertical += v
		} else {
			horizontal += v
		}
	}
	return vertical, horizontal
}
