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

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// concentric-rings test wall (go-slide-creator-qasxs): every layer count on
// every template including the local p-style, the budget finding in the fit
// report, the compose and nested-cell splits, and the recommend_visual
// intents.

// crMatrixLayers is in-budget copy: labels of at most 22 characters and
// one-line descriptions (at most 37), the 4-5 layer budget.
var crMatrixLayers = []patterns.ConcentricRingsLayer{
	{Label: "Sponsor", Description: "Owns the budget and the outcome"},
	{Label: "Leadership team", Description: "Sets priorities across the functions"},
	{Label: "Function heads", Description: "Commit people and change the process"},
	{Label: "Front-line teams", Description: "Adopt the new ways of working"},
	{Label: "Customers and partners", Description: "Feel the change and judge it"},
}

func crMatrixValues(n int, descriptions bool) json.RawMessage {
	v := patterns.ConcentricRingsValues{}
	for _, l := range crMatrixLayers[:n] {
		if !descriptions {
			l.Description = ""
		}
		v.Layers = append(v.Layers, l)
	}
	raw, _ := json.Marshal(v)
	return raw
}

// crMatrixTemplates is every template (p-style when present); -short keeps
// the shortest content area, one mid-sized template and p-style.
func crMatrixTemplates() []string {
	all := testutil.AllTestTemplateNames()
	if !testing.Short() {
		return all
	}
	var out []string
	for _, name := range all {
		switch name {
		case "abstract", "midnight-blue", "p-style":
			out = append(out, name)
		}
	}
	return out
}

// crGenerate generates input on tpl with strict output validation and
// returns the result; any refuse-level fit finding fails the test.
func crGenerate(t *testing.T, tpl string, input PresentationInput) JSONOutput {
	t.Helper()
	input.Template = tpl
	input.OutputFilename = "deck.pptx"
	input.DesignMode = "free"
	dir := t.TempDir()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	inputPath := filepath.Join(dir, "input.json")
	if err := os.WriteFile(inputPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(dir, "result.json")
	if err := runJSONMode(inputPath, resultPath, testutil.TemplatesDir(), dir, "", false, false, tpl, "off", false, "strict", "free", false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	resBytes, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var out JSONOutput
	if err := json.Unmarshal(resBytes, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Success {
		t.Fatalf("generation failed: %s", out.Error)
	}
	for _, f := range out.FitFindings {
		if matrixBlockingFitFinding(f) {
			t.Errorf("refuse-level fit finding %s: %s", f.Code, f.Message)
		}
	}
	return out
}

func crSlideXML(t *testing.T, pptxPath string) string {
	t.Helper()
	zr, err := zip.OpenReader(pptxPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	rc, err := zr.Open("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var (
	crShapeExtRe = regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)
	// A scheme colour with no modifier child: the template's primary fill
	// (an accent, or dk2 on blue-corporate). The tint ladder carries lumMod.
	crSolidRe = regexp.MustCompile(`<a:solidFill><a:schemeClr val="[a-z0-9]+"/></a:solidFill>`)
)

type crEllipse struct {
	x, y, cx, cy int64
	solidAccent  bool
}

// crEllipses returns the ellipse shapes of the slide in drawing order. Rings
// and leader dots are both ellipses; rings are the ones wider than 0.5in.
func crEllipses(t *testing.T, xml string) (rings, dots []crEllipse) {
	t.Helper()
	for _, sp := range strings.Split(xml, "<p:sp>")[1:] {
		if !strings.Contains(sp, `<a:prstGeom prst="ellipse">`) {
			continue
		}
		m := crShapeExtRe.FindStringSubmatch(sp)
		if m == nil {
			t.Fatalf("ellipse without a frame: %.200s", sp)
		}
		var e crEllipse
		e.x, _ = strconv.ParseInt(m[1], 10, 64)
		e.y, _ = strconv.ParseInt(m[2], 10, 64)
		e.cx, _ = strconv.ParseInt(m[3], 10, 64)
		e.cy, _ = strconv.ParseInt(m[4], 10, 64)
		// The fill is the first solidFill of the shape properties; a plain
		// scheme colour with no modifier child is a solid accent.
		spPr := sp
		if i := strings.Index(sp, "</p:spPr>"); i > 0 {
			spPr = sp[:i]
		}
		e.solidAccent = crSolidRe.MatchString(spPr)
		if e.cx > 457200 {
			rings = append(rings, e)
		} else {
			dots = append(dots, e)
		}
	}
	return rings, dots
}

// crAssertRings checks what a viewer must see on any template and in any
// cell: n circles (not ovals), outermost first with every next one inside
// the one before and on its vertical axis, and one solid accent among them.
func crAssertRings(t *testing.T, xml string, n int) []crEllipse {
	t.Helper()
	rings, dots := crEllipses(t, xml)
	if len(rings) != n || len(dots) != n {
		t.Fatalf("%d rings and %d leader dots, want %d of each", len(rings), len(dots), n)
	}
	solid := 0
	for i, r := range rings {
		if d := r.cx - r.cy; d < -1 || d > 1 {
			t.Errorf("ring %d is not round: %d × %d EMU", i, r.cx, r.cy)
		}
		if r.solidAccent {
			solid++
		}
		if i == 0 {
			continue
		}
		out := rings[i-1]
		if r.cx >= out.cx || r.x < out.x || r.y <= out.y || r.x+r.cx > out.x+out.cx || r.y+r.cy > out.y+out.cy+1 {
			t.Errorf("ring %d (%+v) is not nested inside ring %d (%+v)", i, r, i-1, out)
		}
		if off := (r.x + r.cx/2) - (out.x + out.cx/2); off < -127 || off > 127 { // frames are rounded to 1/100000 of the square
			t.Errorf("ring %d is %d EMU off the axis of ring %d", i, off, i-1)
		}
	}
	if solid != 1 || !rings[n-1].solidAccent {
		t.Errorf("%d solid accent rings (core solid: %v), want the core alone", solid, rings[n-1].solidAccent)
	}
	return rings
}

// crAssertTextAtFloor checks that every given text is on the slide in a run
// of 12pt or more and that no shape stores an autofit shrink.
func crAssertTextAtFloor(t *testing.T, out JSONOutput, texts ...string) {
	t.Helper()
	scales, err := storedFontScales(out.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(scales) > 0 {
		t.Errorf("stored autofit shrink(s): %s", strings.Join(scales, ", "))
	}
	for i, size := range slideRunSizes(t, out.OutputPath, texts...) {
		if size < 1200 {
			t.Errorf("%q written at %.1fpt (0 = not on the slide), want 12pt or more", texts[i], float64(size)/100)
		}
	}
}

func crTexts(n int, descriptions bool) []string {
	var out []string
	for _, l := range crMatrixLayers[:n] {
		out = append(out, l.Label)
		if descriptions {
			out = append(out, l.Description)
		}
	}
	return out
}

// 3-5 layers × every template: no refusal, no shrink, every label and
// description at 12pt or more, round nested rings with one solid accent.
func TestConcentricRingsMatrixAcrossTemplates(t *testing.T) {
	t.Parallel()
	for _, tpl := range crMatrixTemplates() {
		for n := 3; n <= 5; n++ {
			t.Run(fmt.Sprintf("%s/%d", tpl, n), func(t *testing.T) {
				t.Parallel()
				title := "Layers of influence"
				out := crGenerate(t, tpl, PresentationInput{Slides: []SlideInput{{
					LayoutID: "content",
					Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
					Pattern:  &PatternInput{Name: "concentric-rings", Values: crMatrixValues(n, true)},
				}}})
				for _, f := range out.FitFindings {
					if f.Code == patterns.ErrCodeBodyTooLong {
						t.Errorf("in-budget copy reported: %s", f.Message)
					}
				}
				crAssertRings(t, crSlideXML(t, out.OutputPath), n)
				crAssertTextAtFloor(t, out, crTexts(n, true)...)
			})
		}
	}
}

// A description past its row reaches the fit report as BODY_TOO_LONG naming
// the layer, on every template.
func TestConcentricRingsBudgetFindingReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.ConcentricRingsValues{Layers: append([]patterns.ConcentricRingsLayer(nil), crMatrixLayers...)}
	values.Layers[3].Description = strings.TrimSpace(strings.Repeat("word ", 14)) // 69 characters, three lines
	values.Layers[3].Label = "Front-line delivery teams"[:24]
	assertBudgetFindingAcrossTemplates(t, "concentric-rings", values, "layers[3].description", "ladder row with 5 layers",
		&patterns.ConcentricRingsOverrides{HeaderSize: 20, BodySize: 18})
}

// Budget probe: the longest description (under a 24-character label) that
// stays readable on every template, per layer count. Run with
// JSON2PPTX_CONCENTRIC_RINGS_BUDGET_PROBE=1 when the layout changes and
// update patterns' crDescBudgetByCount and the schema description.
func TestConcentricRingsBudgetProbe(t *testing.T) {
	if os.Getenv("JSON2PPTX_CONCENTRIC_RINGS_BUDGET_PROBE") == "" {
		t.Skip("set JSON2PPTX_CONCENTRIC_RINGS_BUDGET_PROBE=1")
	}
	for n := 3; n <= 5; n++ {
		budget := probeReadableBudget(t, "concentric-rings", 70, func(length int) any {
			v := &patterns.ConcentricRingsValues{}
			for i := 0; i < n; i++ {
				v.Layers = append(v.Layers, patterns.ConcentricRingsLayer{Label: budgetProbeCopy(24), Description: budgetProbeCopy(length)})
			}
			return v
		})
		t.Logf("layers=%d description budget=%d", n, budget)
	}
}

// A horizontal 50% compose segment and a nested shape-grid cell: the rings
// stay round and nested, the square gives width to the ladder, and labels
// are written at 12pt or more without shrink.
func TestConcentricRingsSplitLayoutsAcrossTemplates(t *testing.T) {
	t.Parallel()
	bullets := json.RawMessage(`{"rows": [
		{"label": "NOW", "body": "Prove the model with one team"},
		{"label": "NEXT", "body": "Roll out across the function"},
		{"label": "THEN", "body": "Make it the standard"}]}`)
	title := "Change spreads outward"
	for _, tpl := range crMatrixTemplates() {
		for n := 3; n <= 5; n++ {
			t.Run(fmt.Sprintf("compose/%s/%d", tpl, n), func(t *testing.T) {
				t.Parallel()
				out := crGenerate(t, tpl, PresentationInput{Slides: []SlideInput{{
					LayoutID: "content",
					Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
					Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
						{SizePct: 50, Pattern: PatternInput{Name: "concentric-rings", Values: crMatrixValues(n, false)}},
						{SizePct: 50, Pattern: PatternInput{Name: "labeled-rows", Values: bullets}},
					}},
				}}})
				rings := crAssertRings(t, crSlideXML(t, out.OutputPath), n)
				// More than a token: the square keeps at least 120pt.
				if rings[0].cx < 120*12700 {
					t.Errorf("outer ring is %.0fpt across in a half-width segment", float64(rings[0].cx)/12700)
				}
				crAssertTextAtFloor(t, out, crTexts(n, false)...)
			})
		}
		t.Run("nested-cell/"+tpl, func(t *testing.T) {
			t.Parallel()
			grid := fmt.Sprintf(`{"columns": [50, 50], "rows": [{"cells": [
				{"pattern": {"name": "concentric-rings", "values": %s}},
				{"shape": {"geometry": "rect", "fill": "none", "text": {"content": "Three circles of adoption", "size": 14, "align": "l"}}}
			]}]}`, crMatrixValues(3, false))
			var sg ShapeGridInput
			if err := json.Unmarshal([]byte(grid), &sg); err != nil {
				t.Fatal(err)
			}
			out := crGenerate(t, tpl, PresentationInput{Slides: []SlideInput{{
				LayoutID:  "content",
				Content:   []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
				ShapeGrid: &sg,
			}}})
			crAssertRings(t, crSlideXML(t, out.OutputPath), 3)
			crAssertTextAtFloor(t, out, crTexts(3, false)...)
		})
	}
}

// recommend_visual ranks concentric-rings first for the intents an agent
// types for an onion model.
func TestMCPRecommendVisual_ConcentricRingsIntents(t *testing.T) {
	mc := testMCPConfig(t)
	for _, intent := range []string{
		"onion model of our stakeholders",
		"layers of influence around the programme sponsor",
		"core adjacent ecosystem view of the business",
		"nested layers from team to enterprise",
	} {
		rec := callRecommendVisual(t, mc, map[string]any{"intent": intent})
		if len(rec.Candidates) == 0 || rec.Candidates[0].Name != "concentric-rings" {
			var names []string
			for _, c := range rec.Candidates {
				names = append(names, c.Name)
			}
			t.Errorf("intent %q: candidates %v, want concentric-rings first", intent, names)
		}
	}
}

// A horizontal compose copies a fixed row's max height onto its cells. A cell
// that spans rows must not take its first row's cap: the ring cell of a
// lattice was shrunk to one ladder row.
func TestComposeHorizontalKeepsRowSpanningCellsUncapped(t *testing.T) {
	ring := &jsonschema.GridCellInput{RowSpan: 3, Fit: "contain"}
	label := &jsonschema.GridCellInput{}
	g := &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{MinHeight: 50, MaxHeight: 50, Cells: []*jsonschema.GridCellInput{ring, label}}}}
	cells := segmentRowCells(g, 0, 2, 0, nil)
	if len(cells) != 2 {
		t.Fatalf("%d cells", len(cells))
	}
	if cells[0].MaxHeight != 0 {
		t.Errorf("row-spanning cell capped at %vpt", cells[0].MaxHeight)
	}
	if cells[1].MaxHeight != 50 {
		t.Errorf("single-row cell max height = %v, want the row's 50", cells[1].MaxHeight)
	}
}
