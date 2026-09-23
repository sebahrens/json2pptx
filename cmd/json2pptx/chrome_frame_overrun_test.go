package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Content-sized pattern rows are sized from the slide area, and the chrome —
// the footer placeholders and the takeaway / source bands — is drawn
// independently. Nothing tied the two together, so a stacked-box strip ran its
// last card down over the footer band (go-slide-creator-n3a1). The geometry is
// settled now by the shared contract (the content zone stops at the resolved
// footer line, and reserveTakeawayBand pulls it above the band stack); this
// test is what keeps it settled, because the failure is invisible in the XML
// unless someone measures it.
func TestPatternBlockStaysAboveChrome(t *testing.T) {
	if testing.Short() {
		t.Skip("generates a deck per template × style × band combination")
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	templatesDir := filepath.Join(projectRoot, "templates")

	steps := []map[string]string{
		{"label": "Assess", "body": "Baseline the estate and agree the scope with the steering group."},
		{"label": "Design", "body": "Target architecture and the migration waves, wave by wave."},
		{"label": "Build", "body": "Stand up the platform and rehearse the cutover twice."},
		{"label": "Run", "body": "Hypercare for four weeks, then steady-state operations."},
		{"label": "Optimise", "body": "Retire the legacy estate and bank the run-rate saving."},
		{"label": "Review", "body": "Post-implementation review against the business case."},
	}

	for _, tpl := range testutil.CoreTemplateNames() {
		for _, style := range []string{"stacked-box", "chevron", "toc"} {
			for _, bands := range []string{"none", "takeaway", "takeaway+source"} {
				name := fmt.Sprintf("%s/%s/%s", tpl, style, bands)
				t.Run(name, func(t *testing.T) {
					slide := map[string]any{
						"slide_type": "content",
						"layout_id":  "content",
						"content": []any{map[string]any{
							"placeholder_id": "title", "type": "text", "text_value": "How the rollout runs",
						}},
						"pattern": map[string]any{
							"name":   "numbered-step-strip",
							"values": map[string]any{"style": style, "steps": steps},
						},
					}
					if strings.Contains(bands, "takeaway") {
						slide["takeaway"] = "Migration completes in Q3 with no service interruption."
					}
					if strings.Contains(bands, "source") {
						slide["source"] = "Programme office, September 2026"
					}

					pptxPath := generateDeckForChromeTest(t, templatesDir, tpl, slide)
					bottom := lowestNonChromeShape(t, pptxPath)

					layouts := loadLayoutsForChromeTest(t, filepath.Join(templatesDir, tpl+".pptx"))
					frame := template.ResolveChromeFrame(
						layoutWithBody(layouts),
						template.ChromeReferenceLayout(layouts),
						chromeTestSlideW, chromeTestSlideH,
						strings.Contains(bands, "takeaway"),
						strings.Contains(bands, "source"),
					)
					limit := frame.Content.Bottom()
					if bottom > limit {
						t.Errorf("the pattern's lowest shape ends at %d, %d EMU below the content frame (%d); "+
							"it is drawing over the %s chrome", bottom, bottom-limit, limit, bands)
					}
				})
			}
		}
	}
}

const (
	chromeTestSlideW = 12192000
	chromeTestSlideH = 6858000
)

// generateDeckForChromeTest writes a one-slide deck and returns the PPTX path.
func generateDeckForChromeTest(t *testing.T, templatesDir, tpl string, slide map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	input := map[string]any{
		"template":        tpl,
		"output_filename": "chrome.pptx",
		"slides":          []any{slide},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	inputPath := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}
	if err := runJSONMode(inputPath, filepath.Join(dir, "result.json"), templatesDir, dir,
		"", false, false, tpl, "off", false, "off", "", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	return filepath.Join(dir, "chrome.pptx")
}

// chromeShapeName matches the shapes that ARE the chrome, which are allowed to
// sit in the band stack because they are what the band stack is for.
var chromeShapeName = regexp.MustCompile(`(?i)takeaway|source|footer|slide number|page`)

// shapeGeometry captures one shape's offset and extent.
var shapeGeometry = regexp.MustCompile(`<a:off x="-?\d+" y="(-?\d+)"/><a:ext cx="\d+" cy="(\d+)"`)

// shapeElement matches one drawable element of a slide.
var shapeElement = regexp.MustCompile(`(?s)<p:(sp|graphicFrame|pic|grpSp)>.*?</p:(sp|graphicFrame|pic|grpSp)>`)

// shapeNameAttr captures a shape's name.
var shapeNameAttr = regexp.MustCompile(`name="([^"]*)"`)

// lowestNonChromeShape returns the largest y+cy over the slide's content
// shapes, skipping the chrome the band stack is reserved for.
func lowestNonChromeShape(t *testing.T, pptxPath string) int64 {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var slideXML string
	for _, f := range zr.File {
		if f.Name != "ppt/slides/slide1.xml" {
			continue
		}
		rc, oErr := f.Open()
		if oErr != nil {
			t.Fatalf("open slide1: %v", oErr)
		}
		data, rErr := io.ReadAll(rc)
		_ = rc.Close()
		if rErr != nil {
			t.Fatalf("read slide1: %v", rErr)
		}
		slideXML = string(data)
	}
	if slideXML == "" {
		t.Fatal("no slide1.xml in the deck")
	}

	var lowest int64
	for _, m := range shapeElement.FindAllString(slideXML, -1) {
		if n := shapeNameAttr.FindStringSubmatch(m); n != nil && chromeShapeName.MatchString(n[1]) {
			continue
		}
		g := shapeGeometry.FindStringSubmatch(m)
		if g == nil {
			continue
		}
		y, _ := strconv.ParseInt(g[1], 10, 64)
		cy, _ := strconv.ParseInt(g[2], 10, 64)
		if y+cy > lowest {
			lowest = y + cy
		}
	}
	if lowest == 0 {
		t.Fatal("no content shapes found on the slide")
	}
	return lowest
}

func loadLayoutsForChromeTest(t *testing.T, path string) []types.LayoutMetadata {
	t.Helper()
	reader, err := template.OpenTemplate(path)
	if err != nil {
		t.Fatalf("open template: %v", err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatalf("parse layouts: %v", err)
	}
	return layouts
}

// layoutWithBody returns the layout the deck's "content" id resolves to: the
// first with a usable body placeholder. nil lets ResolveChromeFrame fall back
// to the reference layout, which is what generation does too.
func layoutWithBody(layouts []types.LayoutMetadata) *types.LayoutMetadata {
	for i := range layouts {
		for _, ph := range layouts[i].Placeholders {
			if ph.Type == types.PlaceholderBody && ph.Bounds.Width > 0 && ph.Bounds.Height > 0 {
				return &layouts[i]
			}
		}
	}
	return nil
}
