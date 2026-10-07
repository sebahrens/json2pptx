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

// hostedPatternSlides places one pattern object in every spot a slide can
// host a pattern below the slide level, keyed by the JSON pointer of that
// pattern object on slide 0.
func hostedPatternSlides(t *testing.T, pattern string) map[string]SlideInput {
	t.Helper()
	decode := func(doc string) SlideInput {
		var s SlideInput
		if err := json.Unmarshal([]byte(doc), &s); err != nil {
			t.Fatalf("fixture: %v\n%s", err, doc)
		}
		return s
	}
	kpi := `{"pattern":{"name":"kpi-2up","values":[{"big":"1","small":"a"},{"big":"2","small":"b"}]}}`
	return map[string]SlideInput{
		"/slides/0/compose/segments/1/pattern": decode(
			`{"compose":{"direction":"horizontal","segments":[` + kpi + `,{"pattern":` + pattern + `}]}}`),
		"/slides/0/compose/segments/0/compose/segments/1/pattern": decode(
			`{"compose":{"direction":"vertical","segments":[{"compose":{"direction":"horizontal","segments":[` +
				kpi + `,{"pattern":` + pattern + `}]}},` + kpi + `]}}`),
		"/slides/0/shape_grid/rows/0/cells/1/pattern": decode(
			`{"shape_grid":{"columns":2,"rows":[{"cells":[{"shape":{"text":"x"}},{"pattern":` + pattern + `}]}]}}`),
		"/slides/0/shape_grid/rows/0/cells/0/grid/rows/1/cells/0/pattern": decode(
			`{"shape_grid":{"columns":1,"rows":[{"cells":[{"grid":{"columns":1,"rows":[{"cells":[{"shape":{"text":"x"}}]},{"cells":[{"pattern":` +
				pattern + `}]}]}}]}]}}`),
	}
}

// hostedPatternValues returns the values of the pattern at pointer (as built
// by hostedPatternSlides) after asset resolution rewrote them.
func hostedPatternValues(t *testing.T, slide *SlideInput, pointer string) string {
	t.Helper()
	for _, h := range slidePatternHosts(slide, 0) {
		if h.path == pointer {
			return string(h.values)
		}
	}
	t.Fatalf("no pattern hosted at %s", pointer)
	return ""
}

// TestResolvePatternImagePaths_EveryHost: a relative image path in a pattern
// resolves against the deck directory wherever the slide hosts the pattern —
// a compose segment, a nested compose, a grid cell, a nested grid — and a
// missing file is the same IMAGE_PATH error a slide-level pattern gets,
// addressed to the authored field. Before go-slide-creator-6tgq9 only the
// slide-level pattern was looked at: the picture was dropped with a log line
// and validate passed.
func TestResolvePatternImagePaths_EveryHost(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "photo.png"), 30, 20)
	patternsWithImages := []struct{ name, values, field string }{
		{"image-text-split", `{"image":{"path":"%s","alt":"x"},"body":"b"}`, "image"},
		{"pull-quote", `{"quote":"q","attribution":"a","image":{"path":"%s","alt":"x"}}`, "image"},
		{"team-bios", `{"members":[{"name":"A B","role":"r"},{"name":"C D","role":"r","photo":{"path":"%s","alt":"x"}}]}`, "members/1/photo"},
		{"contact-directory", `{"groups":[{"name":"g","people":[{"name":"A B","photo":{"path":"%s","alt":"x"}}]}]}`, "groups/0/people/0/photo"},
	}
	for _, p := range patternsWithImages {
		object := func(file string) string {
			return `{"name":"` + p.name + `","values":` + strings.Replace(p.values, "%s", file, 1) + `}`
		}
		for pointer, slide := range hostedPatternSlides(t, object("photo.png")) {
			slides := []SlideInput{slide}
			if diags := resolveLocalAssetPaths(slides, dir); len(diags) != 0 {
				t.Errorf("%s at %s: unexpected diagnostics: %+v", p.name, pointer, diags)
				continue
			}
			src := authoredImageSources(&slides[0], 0)
			if len(src) != 1 {
				t.Errorf("%s at %s: want one authored image file, got %v", p.name, pointer, src)
			}
			for file, field := range src {
				if !filepath.IsAbs(file) || filepath.Base(file) != "photo.png" {
					t.Errorf("%s at %s: image path not resolved against the deck dir: %q", p.name, pointer, file)
				}
				if field != pointer+"/values/"+p.field {
					t.Errorf("%s: authored source = %s, want %s", p.name, field, pointer+"/values/"+p.field)
				}
			}
		}
		for pointer, slide := range hostedPatternSlides(t, object("nope.png")) {
			diags := resolveLocalAssetPaths([]SlideInput{slide}, dir)
			want := pointer + "/values/" + p.field + "/path"
			if len(diags) != 1 || diags[0].Path != want || string(diags[0].Code) != "IMAGE_PATH" {
				t.Errorf("%s: missing file should be one IMAGE_PATH at %s, got %+v", p.name, want, diags)
			}
		}
	}
}

// TestResolvePatternImagePaths_CellPatternKeepsAuthoredKeys: rewriting a cell
// pattern's values leaves the rest of the authored pattern object alone.
func TestResolvePatternImagePaths_CellPatternKeepsAuthoredKeys(t *testing.T) {
	dir := t.TempDir()
	writeTestPNG(t, filepath.Join(dir, "photo.png"), 30, 20)
	const pointer = "/slides/0/shape_grid/rows/0/cells/1/pattern"
	slide := hostedPatternSlides(t, `{"name":"pull-quote","overrides":{"image_side":"right"},"vertical_align":"top","values":{"quote":"q","attribution":"a","image":{"path":"photo.png"}}}`)[pointer]
	slides := []SlideInput{slide}
	if diags := resolveLocalAssetPaths(slides, dir); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	raw := string(slides[0].ShapeGrid.Rows[0].Cells[1].Pattern)
	for _, want := range []string{`"overrides":{"image_side":"right"}`, `"vertical_align":"top"`, `"name":"pull-quote"`} {
		if !strings.Contains(raw, want) {
			t.Errorf("cell pattern lost %s: %s", want, raw)
		}
	}
}

// TestResolvePatternImageURLs_ComposeSegment: an image url in a compose
// segment's pattern starts the URL resolver and is rewritten to the cached
// file, like a slide-level pattern's.
func TestResolvePatternImageURLs_ComposeSegment(t *testing.T) {
	const pointer = "/slides/0/compose/segments/1/pattern"
	object := `{"name":"image-text-split","values":{"image":{"url":"https://example.com/p.png"},"body":"b"}}`
	slides := []SlideInput{hostedPatternSlides(t, object)[pointer]}
	if !hasURLReferences(slides) {
		t.Fatal("a compose segment's pattern image url should require the URL resolver")
	}
	if diags := resolveURLs(slides, fakeURLResolver{path: "/cache/p.png"}); len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if s := hostedPatternValues(t, &slides[0], pointer); !strings.Contains(s, `"/cache/p.png"`) || strings.Contains(s, "example.com") {
		t.Errorf("url should be replaced by the cached path: %s", s)
	}
	failing := []SlideInput{hostedPatternSlides(t, object)[pointer]}
	diags := resolveURLs(failing, fakeURLResolver{err: errors.New("boom")})
	if len(diags) != 1 || diags[0].Path != pointer+"/values/image/url" {
		t.Errorf("fetch failure should be reported at the segment's image url, got %+v", diags)
	}
}

// TestComposeSegmentImage_GeneratesPicture generates the showcase slide that
// found go-slide-creator-6tgq9 — image-text-split in a horizontal compose,
// the picture given relative to the deck — and checks the picture is on the
// slide; with the file missing, generation refuses with the IMAGE_PATH error
// instead of writing a deck without it.
func TestComposeSegmentImage_GeneratesPicture(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(dir, "assets", "console.png"), 160, 90)
	deck := func(file string) string {
		return `{"template":"midnight-blue","output_filename":"seg.pptx","slides":[{"layout_id":"blank-title",
		"content":[{"placeholder_id":"title","type":"text","text_value":"Intake time down by two thirds"}],
		"compose":{"direction":"horizontal","gap":10,"segments":[
		{"size_pct":62,"pattern":{"name":"image-text-split","values":{"image":{"path":"` + file + `","alt":"Console","fit":"contain"},
		"heading":"Claims intake became a conversation","bullets":["Agent handles 78% of first notices"]}}},
		{"size_pct":38,"pattern":{"name":"kpi-2up","values":[{"big":"$0.90","small":"Agent task"},{"big":"$4.20","small":"Human task"}]}}]}}]}`
	}
	templatesDir, _ := filepath.Abs(filepath.Join("..", "..", "templates"))
	generate := func(file string) error {
		jsonPath := filepath.Join(dir, "deck.json")
		if err := os.WriteFile(jsonPath, []byte(deck(file)), 0o644); err != nil {
			t.Fatal(err)
		}
		return runJSONMode(jsonPath, filepath.Join(dir, "result.json"), templatesDir, dir, "", false, false, "", "off", false, "strict", "", false)
	}

	if err := generate("assets/console.png"); err != nil {
		t.Fatalf("generate: %v", err)
	}
	zr, err := zip.OpenReader(filepath.Join(dir, "seg.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	pictures := 0
	for _, f := range zr.File {
		if f.Name == "ppt/slides/slide1.xml" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			pictures = strings.Count(string(b), "<p:pic>")
		}
	}
	if pictures != 1 {
		t.Errorf("the compose segment's picture should be on the slide, got %d <p:pic>", pictures)
	}

	err = generate("assets/nope.png")
	if err == nil || !strings.Contains(err.Error(), "IMAGE_PATH at /slides/0/compose/segments/0/pattern/values/image/path") {
		t.Errorf("a missing segment image should refuse generation with IMAGE_PATH at its field, got %v", err)
	}
}
