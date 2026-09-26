package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// TestNewSlideRelIDsUniqueAcrossTemplates is the regression test for
// go-slide-creator-s1uvj.23. The native-SVG rId allocator used to seed its
// counter from the template's example-slide rels (which are always excluded
// from output), while the notes and background allocators counted only the
// rels a new slide actually gets. On templates whose slide1 carried extra
// rels (modern) a chart slide with speaker notes wrote rId4 twice. Every
// shipped template must produce unique rIds for a chart combined with notes,
// an image, and a background image.
func TestNewSlideRelIDsUniqueAcrossTemplates(t *testing.T) {
	if testing.Short() {
		t.Skip("generates three decks per shipped template")
	}
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")

	imgPath := filepath.Join(t.TempDir(), "photo.png")
	writeRelTestPNG(t, imgPath)

	title := map[string]any{"placeholder_id": "title", "type": "text", "text_value": "Revenue by quarter"}
	chart := func(placeholder string) map[string]any {
		return map[string]any{"placeholder_id": placeholder, "type": "chart", "chart_value": map[string]any{
			"type": "bar", "title": "Revenue ($M)", "alt": "Revenue rises every quarter.",
			"data": map[string]any{"Q1": 12.0, "Q2": 14.5, "Q3": 15.2, "Q4": 18.0},
		}}
	}
	cases := map[string]map[string]any{
		"chart+notes": {
			"slide_type":    "chart",
			"speaker_notes": "Talk track for the chart.",
			"content":       []any{title, chart("body")},
		},
		"chart+image": {
			"slide_type":    "two-column",
			"speaker_notes": "Talk track.",
			"content": []any{title, chart("body"), map[string]any{
				"placeholder_id": "body_2", "type": "image",
				"image_value": map[string]any{"path": imgPath, "alt": "Photo"},
			}},
		},
		"chart+background": {
			"slide_type":    "chart",
			"speaker_notes": "Talk track.",
			"background":    map[string]any{"image": imgPath},
			"content":       []any{title, chart("body")},
		},
	}

	for _, tpl := range testutil.AllBuiltinTemplateNames() {
		for name, slide := range cases {
			t.Run(tpl+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				data, err := json.Marshal(map[string]any{
					"template": tpl, "output_filename": "rels.pptx", "slides": []any{slide},
				})
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				inputPath := filepath.Join(dir, "input.json")
				if err := os.WriteFile(inputPath, data, 0o600); err != nil {
					t.Fatalf("write input: %v", err)
				}
				if err := runJSONMode(inputPath, filepath.Join(dir, "result.json"), templatesDir, dir,
					"", false, false, tpl, "off", false, "off", "", false); err != nil {
					t.Fatalf("generate: %v", err)
				}
				assertUniqueSlideRelIDs(t, filepath.Join(dir, "rels.pptx"))
			})
		}
	}
}

func writeRelTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: 90, B: uint8(y * 5), A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func assertUniqueSlideRelIDs(t *testing.T, pptxPath string) {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer func() { _ = zr.Close() }()
	checked := 0
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "ppt/slides/_rels/") || !strings.HasSuffix(f.Name, ".rels") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		raw, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		var rels struct {
			Rel []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal(raw, &rels); err != nil {
			t.Fatalf("parse %s: %v", f.Name, err)
		}
		seen := map[string]string{}
		for _, r := range rels.Rel {
			if prev, dup := seen[r.ID]; dup {
				t.Errorf("%s: rId %s used for both %s and %s", f.Name, r.ID, prev, r.Target)
			}
			seen[r.ID] = r.Target
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no slide rels found in output")
	}
}
