package generator

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func writeTestPNG(t *testing.T, dir, name string, w, h int) string {
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

func generateImageDeck(t *testing.T, templateName string, footer *FooterConfig, slides []SlideSpec) (*GenerationResult, []byte) {
	t.Helper()
	tmp := t.TempDir()
	out := filepath.Join(tmp, "deck.pptx")
	result, err := Generate(context.Background(), GenerationRequest{
		TemplatePath:          "../../templates/" + templateName + ".pptx",
		OutputPath:            out,
		AllowedImagePaths:     []string{os.TempDir(), tmp},
		ExcludeTemplateSlides: true,
		Footer:                footer,
		Slides:                slides,
	})
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	return result, zipSlideXML(t, &z.Reader, 1)
}

func hasFinding(findings []patterns.FitFinding, code, path string) bool {
	for _, f := range findings {
		if f.Code == code && (path == "" || f.Path == path) {
			return true
		}
	}
	return false
}

var srcRectRE = regexp.MustCompile(`<a:srcRect([^/]*)/>`)
var srcRectAttrRE = regexp.MustCompile(`([ltrb])="(-?\d+)"`)

func srcRectTotals(slideXML []byte) (vertical, horizontal int) {
	m := srcRectRE.FindSubmatch(slideXML)
	if m == nil {
		return 0, 0
	}
	for _, a := range srcRectAttrRE.FindAllSubmatch(m[1], -1) {
		v, _ := strconv.Atoi(string(a[2]))
		switch string(a[1]) {
		case "t", "b":
			vertical += v
		case "l", "r":
			horizontal += v
		}
	}
	return vertical, horizontal
}

// TestPortraitImageInBodyPlaceholderKeepsWholePicture pins
// go-slide-creator-dk5sk: a 600x900 portrait in One Content's wide body with
// no fit used to be cover-cropped (srcRect t+b ≈ 77%). It is now placed whole,
// anchored to the body's left edge.
func TestPortraitImageInBodyPlaceholderKeepsWholePicture(t *testing.T) {
	img := writeTestPNG(t, t.TempDir(), "portrait.png", 600, 900)
	result, slide := generateImageDeck(t, "midnight-blue", nil, []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Portrait photo on an image slide"},
		{PlaceholderID: "body", Type: ContentImage, Value: ImageContent{Path: img, Alt: "portrait"}},
	}}})
	if !bytes.Contains(slide, []byte("<p:pic>")) {
		t.Fatalf("no picture rendered; warnings: %v", result.Warnings)
	}
	if v, h := srcRectTotals(slide); v+h >= 30000 {
		t.Errorf("portrait in a body placeholder cropped by t+b=%d l+r=%d, want < 30000 (whole picture)", v, h)
	}
	if hasFinding(result.FitFindings, patterns.ErrCodeImageHeavyCrop, "") {
		t.Errorf("contain placement must not report IMAGE_HEAVY_CROP: %+v", result.FitFindings)
	}
}

// TestExplicitCoverReportsHeavyCrop pins the advisory for an authored cover
// that throws away more than 30% of an axis.
func TestExplicitCoverReportsHeavyCrop(t *testing.T) {
	img := writeTestPNG(t, t.TempDir(), "portrait.png", 600, 900)
	result, slide := generateImageDeck(t, "midnight-blue", nil, []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Portrait photo"},
		{PlaceholderID: "body", Type: ContentImage, Value: ImageContent{Path: img, Alt: "portrait", Fit: "cover"}},
	}}})
	if v, _ := srcRectTotals(slide); v < 30000 {
		t.Fatalf("explicit cover should crop the portrait vertically, got t+b=%d", v)
	}
	if !hasFinding(result.FitFindings, patterns.ErrCodeImageHeavyCrop, "/slides/0/content/1") {
		t.Errorf("explicit heavy cover crop not reported: %+v", result.FitFindings)
	}
}

// TestImagePlaceholderFallsBackToBody pins go-slide-creator-f0l85: an image
// authored as placeholder_id "image" on a layout with no picture placeholder
// lands in the primary body (with an info remap) instead of CONTENT_DROPPED.
func TestImagePlaceholderFallsBackToBody(t *testing.T) {
	img := writeTestPNG(t, t.TempDir(), "landscape.png", 1600, 900)
	result, slide := generateImageDeck(t, "midnight-blue", nil, []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Hub photo"},
		{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: img, Alt: "hub"}},
	}}})
	if !bytes.Contains(slide, []byte("<p:pic>")) {
		t.Fatalf("image not rendered via body fallback; findings: %+v", result.FitFindings)
	}
	if hasFinding(result.FitFindings, patterns.ErrCodeContentDropped, "") {
		t.Errorf("image fallback still reports CONTENT_DROPPED: %+v", result.FitFindings)
	}
	if !hasFinding(result.FitFindings, patterns.ErrCodePlaceholderRemapped, "/slides/0/content/1/placeholder_id") {
		t.Errorf("image remap not reported: %+v", result.FitFindings)
	}
}

// TestTitleReplacedByImageIsReported pins go-slide-creator-2hkgy: on a
// title-less layout the title falls back to the only body shape, which the
// picture then replaces. That loss must surface as CONTENT_DROPPED on the
// title block, never silently.
func TestTitleReplacedByImageIsReported(t *testing.T) {
	img := writeTestPNG(t, t.TempDir(), "portrait.png", 600, 900)
	result, _ := generateImageDeck(t, "modern-yellow", nil, []SlideSpec{{LayoutID: "slideLayout4", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Portrait photo on an image slide"},
		{PlaceholderID: "body", Type: ContentImage, Value: ImageContent{Path: img, Alt: "portrait"}},
	}}})
	if !hasFinding(result.FitFindings, patterns.ErrCodeContentDropped, "/slides/0/content/0") {
		t.Fatalf("title replaced by the picture was not reported as CONTENT_DROPPED: %+v", result.FitFindings)
	}
}

// TestFullBleedPictureSuppressesFooterChrome pins go-slide-creator-3bph8: on
// modern's full-bleed picture layout the footer text and page number would
// sit on the photo, so they are omitted and CHROME_OVER_IMAGE is reported.
func TestFullBleedPictureSuppressesFooterChrome(t *testing.T) {
	img := writeTestPNG(t, t.TempDir(), "landscape.png", 1600, 900)
	footer := &FooterConfig{Enabled: true, LeftText: "Acme Corp | Strictly Confidential"}
	result, slide := generateImageDeck(t, "modern", footer, []SlideSpec{{LayoutID: "slideLayout1", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Full-bleed photo"},
		{PlaceholderID: "image", Type: ContentImage, Value: ImageContent{Path: img, Alt: "photo"}},
	}}})
	if bytes.Contains(slide, []byte("Strictly Confidential")) {
		t.Error("footer text drawn over a full-bleed picture")
	}
	if !hasFinding(result.FitFindings, patterns.ErrCodeChromeOverImage, "") {
		t.Errorf("CHROME_OVER_IMAGE not reported: %+v", result.FitFindings)
	}

	// A picture inside the content area keeps the footer.
	result, slide = generateImageDeck(t, "midnight-blue", footer, []SlideSpec{{LayoutID: "slideLayout2", Content: []ContentItem{
		{PlaceholderID: "title", Type: ContentText, Value: "Contained photo"},
		{PlaceholderID: "body", Type: ContentImage, Value: ImageContent{Path: img, Alt: "photo"}},
	}}})
	if !bytes.Contains(slide, []byte("Strictly Confidential")) {
		t.Error("footer dropped on a slide whose picture stays above the footer band")
	}
	if hasFinding(result.FitFindings, patterns.ErrCodeChromeOverImage, "") {
		t.Errorf("CHROME_OVER_IMAGE reported for a contained picture: %+v", result.FitFindings)
	}
}
