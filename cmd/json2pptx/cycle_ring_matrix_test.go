package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// cycleRingBudgets are the documented copy budgets per phase count: label and
// description characters (skills/generate-deck/PATTERNS.md, the pattern's
// schema descriptions, internal/patterns cycleRingLabelBudget /
// cycleRingDescBudget).
var cycleRingBudgets = map[int][2]int{4: {28, 90}, 5: {28, 70}, 6: {28, 70}, 7: {24, 50}, 8: {24, 50}}

// cycleRingBudgetValues is a ring of n phases, every label and description
// exactly at its budget.
func cycleRingBudgetValues(n int) *patterns.CycleRingValues {
	b := cycleRingBudgets[n]
	v := &patterns.CycleRingValues{}
	for i := 0; i < n; i++ {
		v.Phases = append(v.Phases, patterns.CycleRingPhase{Label: budgetProbeCopy(b[0]), Description: budgetProbeCopy(b[1])})
	}
	return v
}

func cycleRingDeck(t *testing.T, values any, overrides any) PresentationInput {
	t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	p := &PatternInput{Name: "cycle-ring", Values: raw}
	if overrides != nil {
		if p.Overrides, err = json.Marshal(overrides); err != nil {
			t.Fatal(err)
		}
	}
	title := "The cycle repeats every quarter"
	return PresentationInput{Slides: []SlideInput{{
		LayoutID: "content",
		Content:  []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
		Pattern:  p,
	}}}
}

var cycleRingRunSizeRe = regexp.MustCompile(`<a:rPr[^>]* sz="(\d+)"`)

// smallestSlideRunPt is the smallest run size on the deck's first slide whose
// shape is one of the pattern's (a preset shape written by the grid), in
// points; 0 when none carries a size.
func smallestGridRunPt(t *testing.T, pptxPath string) float64 {
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
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	smallest := 0.0
	for _, sp := range strings.Split(string(data), "<p:sp>")[1:] {
		if !strings.Contains(sp, "<a:prstGeom") || strings.Contains(sp, "<p:ph") {
			continue
		}
		for _, m := range cycleRingRunSizeRe.FindAllStringSubmatch(sp, -1) {
			v, _ := strconv.Atoi(m[1])
			if pt := float64(v) / 100; smallest == 0 || pt < smallest {
				smallest = pt
			}
		}
	}
	return smallest
}

// Every phase count, with every label and description at its documented
// budget, generates on every template (the local p-style included) with no
// refusal, no stored autofit shrink and no run under 12pt; so do the
// `arrows` style and a highlighted phase.
func TestCycleRingMatrixAtBudgetAcrossTemplates(t *testing.T) {
	templates := testutil.AllTestTemplateNames()
	counts := []int{4, 5, 6, 7, 8}
	if testing.Short() {
		templates, counts = []string{"abstract", "midnight-blue"}, []int{4, 8}
	}
	for _, tpl := range templates {
		for _, n := range counts {
			t.Run(fmt.Sprintf("%s/%d", tpl, n), func(t *testing.T) {
				t.Parallel()
				values := cycleRingBudgetValues(n)
				values.Phases[n-1].Highlight = true
				var overrides any
				if n%2 == 1 {
					overrides = map[string]string{"style": "arrows"}
				}
				path := generateForSweep(t, tpl, cycleRingDeck(t, values, overrides))
				scales, err := storedFontScales(path)
				if err != nil {
					t.Fatal(err)
				}
				if len(scales) > 0 {
					t.Errorf("%d phases at budget on %s store autofit shrink(s) %s", n, tpl, strings.Join(scales, ", "))
				}
				smallest := smallestGridRunPt(t, path)
				if smallest < 12 {
					t.Errorf("%d phases on %s: smallest pattern run is %.1fpt, want >= 12", n, tpl, smallest)
				}
				t.Logf("%s n=%d: min stored size %.1fpt, no shrink", tpl, n, smallest)
			})
		}
	}
}

// cycleRingLayerSquare returns the bounds of the first ring layer (a blockArc
// in the band's frame: the ring's own square) and how many label cells the
// resolved grid carries.
func cycleRingResolved(t *testing.T, grid *jsonschema.ShapeGridInput) (w, h int64, texts []string) {
	t.Helper()
	res, err := resolveShapeGrid(grid, newAllocFrom(200), nil, nil, 12192000, 6858000, nil)
	if err != nil || res == nil {
		t.Fatalf("resolveShapeGrid: %v", err)
	}
	for _, c := range res.Cells {
		if c.Layer && c.LayerIdx == 0 && w == 0 {
			w, h = c.Bounds.CX, c.Bounds.CY
		}
	}
	for _, xml := range res.Shapes {
		for _, m := range shapeRunTextRe.FindAllStringSubmatch(string(xml), -1) {
			texts = append(texts, m[1])
		}
	}
	return w, h, texts
}

// In a split layout the ring stays round and the labels move into the legend
// beside it: a 50% and a 60% horizontal compose segment, a vertical segment
// and a nested grid cell, on every template.
func TestCycleRingStaysRoundInSplitLayoutsAcrossTemplates(t *testing.T) {
	ring := PatternInput{Name: "cycle-ring", Values: json.RawMessage(`{"phases":[{"label":"Forecast"},{"label":"Performance review"},{"label":"Decide and commit"},{"label":"Execute"},{"label":"Close the books"}]}`)}
	side := PatternInput{Name: "metric-list", Values: json.RawMessage(`{"items":[{"label":"Actions closed on time","value":"14 of 16"},{"label":"Forecast accuracy","value":"96%"},{"label":"Days to close","value":"4"}]}`)}
	templates := testutil.AllTestTemplateNames()
	if testing.Short() {
		templates = []string{"abstract", "midnight-blue"}
	}
	for _, tpl := range templates {
		geom := loadSchemaMaximaGeometry(t, tpl)
		for _, tc := range []struct {
			name      string
			direction string
			ringPct   float64
		}{
			{"horizontal 50", "horizontal", 50},
			{"horizontal 60", "horizontal", 60},
			{"vertical 65", "vertical", 65},
		} {
			t.Run(tpl+"/"+tc.name, func(t *testing.T) {
				title := "The monthly operating rhythm closed 14 of 16 actions on time"
				deck := &PresentationInput{Template: tpl, Slides: []SlideInput{{
					SlideType: "content", LayoutID: "blank-title",
					Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: &title}},
					Compose: &ComposeInput{Direction: tc.direction, Gap: 12, Segments: []SegmentInput{
						{SizePct: tc.ringPct, Pattern: ring},
						{SizePct: 100 - tc.ringPct, Pattern: side},
					}},
				}}}
				for _, f := range collectFitFindings(deck, geom.layouts, geom.width, geom.height, nil) {
					if matrixBlockingFitFinding(f) {
						t.Errorf("blocking finding %s: %s", f.Code, f.Message)
					}
				}
				path := generateForSweep(t, tpl, *deck)
				if scales, err := storedFontScales(path); err != nil || len(scales) > 0 {
					t.Errorf("stored autofit shrink(s) %v (err %v)", scales, err)
				}
				if smallest := smallestGridRunPt(t, path); smallest < 12 {
					t.Errorf("smallest pattern run is %.1fpt, want >= 12", smallest)
				}
			})
		}
	}

	// The ring cell resolves to a square in every context, and the narrow
	// contexts carry the legend (every label once, numbered 1-5).
	reg := patterns.Default()
	for _, tc := range []struct {
		name   string
		w, h   float64
		legend bool
	}{
		{"full slide", 828, 349, false},
		{"50% segment", 408, 341, true},
		{"60% segment", 490, 341, false},
		{"40% segment", 325, 286, true},
		{"nested cell", 300, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := patterns.ExpandContext{LayoutBounds: patterns.LayoutBounds{Width: int64(tc.w * 12700), Height: int64(tc.h * 12700)}}
			p := ring
			grid, _, err := expandPattern(&p, ctx, reg)
			if err != nil {
				t.Fatal(err)
			}
			cols := inferColumnCount(grid)
			if wantCols := 4; tc.legend && cols != wantCols {
				t.Errorf("legend layout has %d lattice columns, want %d (ring, gap, numeral, label)", cols, wantCols)
			}
			if !tc.legend && cols < 5 {
				t.Errorf("outside layout has %d lattice columns, want a label column either side of the ring", cols)
			}
			nested := &jsonschema.ShapeGridInput{
				Columns: json.RawMessage(`[100]`),
				Rows:    []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Grid: grid}}}},
			}
			w, h, texts := cycleRingResolved(t, nested)
			if w == 0 || math.Abs(float64(w-h)) > 1 {
				t.Errorf("ring square resolves to %d x %d EMU, want a square", w, h)
			}
			joined := strings.Join(texts, "|")
			for _, label := range []string{"Forecast", "Performance review", "Decide and commit", "Execute", "Close the books"} {
				if strings.Count(joined, label) != 1 {
					t.Errorf("label %q appears %d times in %q", label, strings.Count(joined, label), joined)
				}
			}
		})
	}
}

// The MCP journey: show_pattern states the budgets the pattern warns at, and
// expand_pattern takes the exemplar and every phase count.
func TestCycleRingMCPShowAndExpand(t *testing.T) {
	result, err := handleShowPattern(context.Background(), makeRequest(map[string]any{"name": "cycle-ring"}))
	if err != nil || result.IsError {
		t.Fatalf("show_pattern failed: %v, %+v", err, result)
	}
	entry, ok := result.StructuredContent.(skillPatternFull)
	if !ok {
		t.Fatalf("show_pattern returned %T", result.StructuredContent)
	}
	schema := string(entry.Schema)
	for _, want := range []string{"28 characters with 4-6 phases, 24 with 7-8", "4: 90 characters, 5-6: 70, 7-8: 50", `"arrows"`, `"counter_clockwise"`, `"legend"`, `"highlight"`} {
		if !strings.Contains(schema, want) {
			t.Errorf("show_pattern schema does not state %q", want)
		}
	}

	mc := &mcpConfig{templatesDir: "../../templates"}
	for n := 4; n <= 8; n++ {
		res, err := mc.handleExpandPattern(t.Context(), makeRequest(map[string]any{
			"name":      "cycle-ring",
			"values":    cycleRingBudgetValues(n),
			"overrides": map[string]any{"style": "arrows", "cell_accent_mode": "alternate"},
		}))
		if err != nil {
			t.Fatalf("expand_pattern(%d phases): %v", n, err)
		}
		if res.IsError {
			t.Fatalf("expand_pattern(%d phases) errored: %s", n, resultText(res))
		}
		if text := resultText(res); !strings.Contains(text, "circularArrow") || strings.Contains(text, patterns.ErrCodeBodyTooLong) {
			t.Errorf("expand_pattern(%d phases) = %.300s", n, text)
		}
	}
	res, err := mc.handleExpandPattern(t.Context(), makeRequest(map[string]any{"name": "cycle-ring", "values": cycleRingBudgetValues(4).Phases[:3]}))
	if err == nil && !res.IsError {
		t.Error("expand_pattern accepted values that are not the pattern's object")
	}
}
