package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
}

func imageTextSplitSlide(values string) SlideInput {
	return SlideInput{Pattern: &PatternInput{Name: "image-text-split", Values: json.RawMessage(values)}}
}

func TestResolvePatternImagePaths_RelativeToDeckDir(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "photo.png"), 30, 20)
	slides := []SlideInput{imageTextSplitSlide(`{"image":{"path":"photo.png","alt":"x"},"body":"b"}`)}

	if diags := resolveLocalAssetPaths(slides, dir); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	var v struct {
		Image jsonschema.GridImageInput `json:"image"`
	}
	if err := json.Unmarshal(slides[0].Pattern.Values, &v); err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(v.Image.Path) || filepath.Base(v.Image.Path) != "photo.png" {
		t.Errorf("pattern image path should resolve against the deck dir, got %q", v.Image.Path)
	}

	missing := []SlideInput{imageTextSplitSlide(`{"image":{"path":"nope.png"},"body":"b"}`)}
	diags := resolveLocalAssetPaths(missing, dir)
	if len(diags) == 0 || !strings.Contains(diags[0].Path, "/pattern/values/image/path") {
		t.Errorf("missing pattern image should be reported at its values path, got %+v", diags)
	}
}

type fakeURLResolver struct {
	path string
	err  error
}

func (f fakeURLResolver) ResolveImage(string) (string, error) { return f.path, f.err }
func (f fakeURLResolver) ResolveSVG(string) (string, error)   { return f.path, f.err }

func TestResolvePatternImageURLs(t *testing.T) {
	slides := []SlideInput{imageTextSplitSlide(`{"image":{"url":"https://example.com/p.png"},"body":"b"}`)}
	if !hasURLReferences(slides) {
		t.Fatal("pattern image url should require the URL resolver")
	}
	if diags := resolveURLs(slides, fakeURLResolver{path: "/cache/p.png"}); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if s := string(slides[0].Pattern.Values); !strings.Contains(s, `"/cache/p.png"`) || strings.Contains(s, "example.com") {
		t.Errorf("url should be replaced by the cached path: %s", s)
	}

	failing := []SlideInput{imageTextSplitSlide(`{"image":{"url":"https://example.com/p.png"},"body":"b"}`)}
	diags := resolveURLs(failing, fakeURLResolver{err: errors.New("boom")})
	if len(diags) != 1 || !strings.HasSuffix(diags[0].Path, "/pattern/values/image/url") {
		t.Errorf("fetch failure should be reported at the pattern image url, got %+v", diags)
	}
	if hasURLReferences([]SlideInput{imageTextSplitSlide(`{"body":"b"}`)}) {
		t.Error("placeholder-only image-text-split has no URL references")
	}
}

func TestFillDefaultFitImages(t *testing.T) {
	input := []GridRowInput{{Cells: []*GridCellInput{
		{Image: &GridImageInput{Path: "a.png"}},
		{Image: &GridImageInput{Path: "b.png"}, Fit: "contain"},
		{Image: &GridImageInput{Path: "c.svg"}},
	}}}
	rows := convertGridRows(input)
	specs := defaultFitImageSpecs(input, rows)
	if len(specs) != 1 || !specs[rows[0].Cells[0].Image] {
		t.Fatalf("only the default-fit raster image should be widened, got %v", specs)
	}
	grid := &shapegrid.Grid{Bounds: pptx.RectEmu{CX: 3000000, CY: 1000000}, Columns: []float64{33.3, 33.3, 33.4}, Rows: rows}
	res, err := shapegrid.Resolve(grid, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	fillDefaultFitImages(res, specs)
	for _, c := range res.Cells {
		wide := c.Bounds == c.CellBounds
		switch c.ImageSpec.Path {
		case "a.png":
			if !wide {
				t.Errorf("default-fit image should fill its cell: %+v vs %+v", c.Bounds, c.CellBounds)
			}
		default:
			if wide {
				t.Errorf("%s should keep shapegrid's square frame", c.ImageSpec.Path)
			}
		}
	}
}

// TestImageTextSplit_GeneratesCoverCroppedPicture generates a deck whose
// image-text-split slide references a relative 3:1 PNG and checks the picture
// fills its (non-3:1) cell with a centred a:srcRect crop.
func TestImageTextSplit_GeneratesCoverCroppedPicture(t *testing.T) {
	if testing.Short() {
		t.Skip("generation test")
	}
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "wide.png"), 900, 300)
	deck := `{"template":"midnight-blue","output_filename":"its.pptx","slides":[{"layout_id":"content",
		"content":[{"placeholder_id":"title","type":"text","text_value":"Case study"}],
		"pattern":{"name":"image-text-split","values":{"image":{"path":"wide.png","alt":"Wide"},"heading":"Heading","body":"Body copy for the case study."}}}]}`
	jsonPath := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(jsonPath, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}
	templatesDir, _ := filepath.Abs(filepath.Join("..", "..", "templates"))
	resultPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(jsonPath, resultPath, templatesDir, dir, "", false, false, "", "off", false, "strict", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "its.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	var slideXML string
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			slideXML = string(b)
		}
	}
	pic := regexp.MustCompile(`(?s)<p:pic>.*?</p:pic>`).FindString(slideXML)
	if pic == "" {
		t.Fatal("no picture on the image-text-split slide")
	}
	m := regexp.MustCompile(`<a:srcRect l="(\d+)" t="0" r="(\d+)" b="0"/>`).FindStringSubmatch(pic)
	if m == nil || m[1] == "0" || m[1] != m[2] {
		t.Errorf("3:1 picture should be centre-cropped left/right: %s", pic)
	}
	ext := regexp.MustCompile(`<a:ext cx="(\d+)" cy="(\d+)"/>`).FindStringSubmatch(pic)
	if ext == nil || ext[1] == ext[2] {
		t.Errorf("picture should fill its (non-square) cell, got ext %v", ext)
	}
}
