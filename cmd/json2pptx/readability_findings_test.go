package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

func kpiSlide(t *testing.T, small, sub string) SlideInput {
	t.Helper()
	// The pattern caps deltas at 12 characters. These readability tests also
	// exercise a raw shape_grid carrying much longer text, so expand a valid
	// KPI first and then replace only that paragraph in the editable grid.
	patternSub := sub
	if len([]rune(sub)) > 12 {
		patternSub = "+4%"
	}
	values := []map[string]string{
		{"big": "$52M", "small": small, "sub": patternSub},
		{"big": "+21%", "small": "YoY growth"},
		{"big": "117%", "small": "NRR"},
	}
	raw, _ := json.Marshal(values)
	// A KPI banner row (1.5in tall), the typical compose/strip placement where
	// long labels and deltas get shrunk by the renderer.
	ctx := patterns.ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: patterns.LayoutBounds{X: 457200, Y: 1371600, Width: 11277600, Height: 1371600},
	}
	grid, _, err := expandPattern(&PatternInput{Name: "kpi-3up", Values: raw}, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expand kpi-3up: %v", err)
	}
	if patternSub != sub {
		var text struct {
			Paragraphs []map[string]any `json:"paragraphs"`
		}
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		text.Paragraphs[1]["content"] = sub
		encoded, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		grid.Rows[0].Cells[0].Shape.Text = encoded
	}
	grid.Bounds = &GridBoundsInput{X: 5, Y: 20, Width: 90, Height: 20} // banner strip
	return SlideInput{LayoutID: "blank", ShapeGrid: grid}
}

func readabilityCodes(fs []patterns.FitFinding) []patterns.FitFinding {
	var out []patterns.FitFinding
	for _, f := range fs {
		if f.Code == patterns.ErrCodeTextBelowReadableMin {
			out = append(out, f)
		}
	}
	return out
}

// go-slide-creator-vbic: a KPI card whose label is far too long for its cell
// is shrunk by the renderer below the present-mode caption/body floor.
func TestReadability_KPILongLabelPresentMode(t *testing.T) {
	// Far too long for the card: the writer stores a deep autofit shrink.
	long := strings.Repeat("up from $41M last year, driven by enterprise expansion and new logos in EMEA ", 4)
	in := &PresentationInput{ViewingMode: "present", Slides: []SlideInput{kpiSlide(t, "Annual recurring revenue (all segments)", long)}}
	got := readabilityCodes(collectReadabilityFindings(in, nil, 12192000, 6858000))
	if len(got) == 0 {
		t.Fatal("expected TEXT_BELOW_READABLE_MIN for the long KPI label")
	}
	f := got[0]
	if !strings.Contains(f.Path, "/shape_grid/") || !strings.HasSuffix(f.Path, "shape/text") {
		t.Errorf("path = %q, want a shape_grid cell text path", f.Path)
	}
	// go-slide-creator-9zof: the fix must name a directive that can edit grid
	// cell text — reduce_text walks content items and would apply nothing.
	if f.Fix == nil || f.Fix.Kind != "reduce_cell_text" {
		t.Fatalf("fix = %+v", f.Fix)
	}
	if f.Fix.Params["cell_path"] != "/slides/0/shape_grid/rows/0/cells/0" {
		t.Errorf("fix must carry the cell path, got %v", f.Fix.Params["cell_path"])
	}
	if pt, _ := f.Fix.Params["actual_pt"].(float64); pt <= 0 || pt >= 12 {
		t.Errorf("actual_pt = %v, want a positive size below 12pt", f.Fix.Params["actual_pt"])
	}
	if s := f.Fix.Params["strategy"]; s != "shorten" && s != "split" {
		t.Errorf("strategy = %v", s)
	}
	if f.Fix.Params["viewing_mode"] != "present" {
		t.Errorf("viewing_mode = %v", f.Fix.Params["viewing_mode"])
	}
	if f.Fix.Params["measurement_source"] != "predicted" {
		t.Errorf("autofit finding source = %v, want predicted", f.Fix.Params["measurement_source"])
	}

	// Short labels are fine.
	short := &PresentationInput{Slides: []SlideInput{kpiSlide(t, "ARR", "+4% vs plan")}}
	if got := readabilityCodes(collectReadabilityFindings(short, nil, 12192000, 6858000)); len(got) != 0 {
		t.Errorf("short KPI labels flagged: %v", got[0].Message)
	}
}

func TestAuthoredSmallGridTextCarriesExactSource(t *testing.T) {
	f := worstReadability([]cellParagraph{{text: "Small label", sizePt: 8}}, 1,
		tokens.ViewingModePresentation, "/slides/0/shape_grid/rows/0/cells/0/shape/text", "")
	if f == nil || f.Fix == nil || f.Fix.Params["measurement_source"] != "authored" {
		t.Fatalf("8pt authored text must be marked exact: %+v", f)
	}
}

func TestReadabilityExplicitInsetsDoNotSubtractDefaultsAgain(t *testing.T) {
	var input PresentationInput
	if err := json.Unmarshal([]byte(`{
		"viewing_mode":"present",
		"slides":[{"layout_id":"blank","shape_grid":{
			"bounds":{"x":5,"y":20,"width":90,"height":6.67},
			"columns":[100],"rows":[{"cells":[{"shape":{
				"geometry":"rect","text":{"content":"First line\nSecond line",
				"size":13,"bold":true,"inset_left":2,"inset_right":2,
				"inset_top":2,"inset_bottom":2}
			}}]}]
		}}]
	}`), &input); err != nil {
		t.Fatal(err)
	}
	// 36pt frame minus 4pt authored padding leaves 32pt for two 13pt
	// lines at 1.2 spacing. The written shape fits without any font shrink.
	if got := readabilityCodes(collectReadabilityFindings(&input, nil, 12192000, 6858000)); len(got) != 0 {
		t.Fatalf("explicitly padded readable text falsely shrunk: %+v", got)
	}
}

func TestWrittenCellAutofitAccountsForSpacingAndOverlay(t *testing.T) {
	for _, tc := range []struct {
		name    string
		text    string
		height  float64
		top     float64
		shrinks bool
	}{
		{"default padding fits", `{"content":"First line\nSecond line","size":13}`, 62, 0, false}, // two 13pt lines + 2 x 0.5 cm margin
		{"authored padding fits", `{"content":"First line\nSecond line","size":13,"inset_top":2,"inset_bottom":2}`, 36, 0, false},
		{"overlay consumes room", `{"content":"First line\nSecond line","size":13,"inset_top":2,"inset_bottom":2}`, 36, 12, true},
		{"paragraph spacing consumes room", `{"paragraphs":[{"content":"First line","size":13,"space_after":16},{"content":"Second line","size":13}]}`, 40, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cell := shapegrid.ResolvedCell{
				ID: 42, Bounds: pptx.RectEmu{CX: shapegrid.PtToEMU(400), CY: shapegrid.PtToEMU(tc.height)},
				ShapeSpec:  &shapegrid.ShapeSpec{Geometry: "rect", Text: json.RawMessage(tc.text)},
				TextInsets: [4]int64{0, shapegrid.PtToEMU(tc.top), 0, 0},
			}
			before, err := json.Marshal(cell)
			if err != nil {
				t.Fatal(err)
			}
			scale, err := writtenCellAutofitScale(cell)
			if err != nil {
				t.Fatal(err)
			}
			if tc.shrinks {
				if scale <= 0 || scale >= .85 {
					t.Fatalf("expected material shrink, got %v", scale)
				}
			} else if scale != 1 {
				t.Fatalf("readable frame shrank to %v", scale)
			}
			after, err := json.Marshal(cell)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("measurement mutated the source cell")
			}
		})
	}
}

func TestWrittenCellAutofitRejectsInvalidShape(t *testing.T) {
	cell := shapegrid.ResolvedCell{
		Bounds:    pptx.RectEmu{CX: shapegrid.PtToEMU(400), CY: shapegrid.PtToEMU(40)},
		ShapeSpec: &shapegrid.ShapeSpec{Geometry: "rect", Fill: json.RawMessage(`{"alpha":101}`), Text: json.RawMessage(`"Text"`)},
	}
	if scale, err := writtenCellAutofitScale(cell); err == nil || scale != 0 {
		t.Fatalf("invalid shape must not acquire a readability measurement: scale=%v err=%v", scale, err)
	}
}

// read mode has lower floors: a label that is borderline in present mode is
// accepted when the deck is read on screen.
func TestReadability_ReadModeIsMoreLenient(t *testing.T) {
	sub := strings.Repeat("up from $41M last year, driven by expansion ", 2)
	present := &PresentationInput{ViewingMode: "present", Slides: []SlideInput{kpiSlide(t, "ARR", sub)}}
	read := &PresentationInput{ViewingMode: "read", Slides: []SlideInput{kpiSlide(t, "ARR", sub)}}
	np := len(readabilityCodes(collectReadabilityFindings(present, nil, 12192000, 6858000)))
	nr := len(readabilityCodes(collectReadabilityFindings(read, nil, 12192000, 6858000)))
	if nr > np {
		t.Errorf("read mode produced more findings (%d) than present (%d)", nr, np)
	}
}

func TestCellTextRole(t *testing.T) {
	cases := []struct {
		pt        float64
		bold      bool
		text      string
		populated int
		want      tokens.TextRole
	}{
		{36, true, "$52M", 1, tokens.TextRoleKPIValue},
		{36, true, "$52M", 2, tokens.TextRoleKPIValue},
		{40, true, "3.8x", 1, tokens.TextRoleKPIValue}, // metric-list value alone in its cell
		{36, false, "78%", 1, tokens.TextRoleKPIValue},
		{36, true, "12", 2, tokens.TextRoleKPIValue}, // short figure with its caption
		// A lone index marker labels an axis, row or step: a caption, not a metric.
		{24, true, "1", 1, tokens.TextRoleCaption},
		{24, false, "(4)", 1, tokens.TextRoleCaption},
		{28, true, "04.", 1, tokens.TextRoleCaption},
		{12, true, "3", 1, tokens.TextRoleCaption},
		// Display words state no figure.
		{28, true, "Domestic", 1, tokens.TextRoleCardTitle},
		{14, true, strings.Repeat("x", 20), 1, tokens.TextRoleCardTitle},
		{12, false, strings.Repeat("x", 12), 1, tokens.TextRoleCaption},
		{12, false, strings.Repeat("x", 200), 1, tokens.TextRoleCardBody},
	}
	for _, c := range cases {
		if got := cellTextRole(c.pt, c.bold, c.text, c.populated); got != c.want {
			t.Errorf("cellTextRole(%v,%v,%q,%d) = %s, want %s", c.pt, c.bold, c.text, c.populated, got, c.want)
		}
	}
	if ps := parseCellParagraphs(json.RawMessage(`{"content":"a\nb","bold":true,"size":10}`)); len(ps) != 2 || !ps[0].bold || ps[0].sizePt != 12 {
		t.Errorf("parseCellParagraphs object = %+v (size 10 floors to 12)", ps)
	}
	if ps := parseCellParagraphs(json.RawMessage(`{"paragraphs":[{"content":"$52M","size":36,"bold":true},{"content":"ARR","size":14}]}`)); len(ps) != 2 || ps[0].sizePt != 36 || ps[1].bold {
		t.Errorf("parseCellParagraphs paragraphs = %+v", ps)
	}
	if ps := parseCellParagraphs(json.RawMessage(`"x"`)); len(ps) != 1 || ps[0].bold || ps[0].sizePt != renderDefaultCellPt {
		t.Errorf("parseCellParagraphs string = %+v", ps)
	}
}

func TestPullQuoteReadabilityUsesProseFloor(t *testing.T) {
	pat, ok := patterns.Default().Get("pull-quote")
	if !ok {
		t.Fatal("pull-quote pattern not registered")
	}
	for _, templateName := range schemaMaximaTemplateNames(t) {
		t.Run(templateName, func(t *testing.T) {
			if pt, note := measureSchemaMaximumPt(t, pat, templateName); note != "" || pt != 0 {
				t.Fatalf("500-character quote with headshot has below-floor finding at %.1fpt: %s", pt, note)
			}
			values, note := schemaMaximumValues(pat)
			if note != "" {
				t.Fatal(note)
			}
			plain := values.(*patterns.PullQuoteValues)
			plain.Image = nil
			encoded, err := json.Marshal(plain)
			if err != nil {
				t.Fatal(err)
			}
			layouts, width, height := schemaMaximaLayouts(t, templateName)
			input := &PresentationInput{Template: templateName, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", Pattern: &PatternInput{Name: "pull-quote", Values: encoded},
			}}}
			if findings := collectReadabilityFindings(input, layouts, width, height); len(findings) != 0 {
				t.Fatalf("500-character quote without headshot should clear the prose floor: %+v", findings)
			}
			expanded, _ := expandPatternsForFit(input, width, height, nil, layouts...)
			grid := expanded.Slides[0].ShapeGrid
			if grid == nil || grid.Source != "pattern:pull-quote" {
				t.Fatalf("pull-quote expansion lost its source stamp: %+v", grid)
			}
			raw := &PresentationInput{Template: templateName, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title", ShapeGrid: grid,
			}}}
			if findings := collectReadabilityFindings(raw, layouts, width, height); len(findings) != 0 {
				t.Fatalf("source-stamped expanded quote changed role: %+v", findings)
			}
		})
	}

	quote := cellParagraph{text: strings.Repeat("quoted prose ", 12), sizePt: 36}
	if finding := worstReadability([]cellParagraph{quote}, 0.25, tokens.ViewingModePresentation, "/quote", tokens.TextRoleBody); finding == nil {
		t.Fatal("a quote fitted to 9pt must still be reported below the 12pt prose floor")
	}
	shortQuote := cellParagraph{text: "Make it simple.", sizePt: 36}
	if finding := worstReadability([]cellParagraph{shortQuote}, 0.4, tokens.ViewingModePresentation, "/short-quote", tokens.TextRoleBody); finding != nil {
		t.Fatalf("a 14.4pt quote should clear the 12pt prose floor: %+v", finding)
	}
	kpi := cellParagraph{text: "$12.4M", sizePt: 36}
	if finding := worstReadability([]cellParagraph{kpi}, 0.4, tokens.ViewingModePresentation, "/kpi", ""); finding == nil || !strings.Contains(finding.Message, "kpi-value") {
		t.Fatalf("non-italic KPI lost its 18pt floor: %+v", finding)
	}
	longKPI := cellParagraph{text: "Revenue $12.4M in Q4, up 30% year-over-year", sizePt: 36}
	if finding := worstReadability([]cellParagraph{longKPI}, 0.4, tokens.ViewingModePresentation, "/long-kpi", ""); finding == nil || !strings.Contains(finding.Message, "kpi-value") {
		t.Fatalf("long display KPI lost its 18pt floor: %+v", finding)
	}
}

func TestViewingModeEnumValidated(t *testing.T) {
	errs := checkInputEnumValues(&PresentationInput{ViewingMode: "projector"})
	if len(errs) == 0 {
		t.Fatal("unknown viewing_mode should be rejected")
	}
	if errs := checkInputEnumValues(&PresentationInput{ViewingMode: "read"}); len(errs) != 0 {
		t.Errorf("read should be valid: %v", errs[0])
	}
}

// A cell whose text cannot be written smaller — it sits at the grid's 12pt
// floor — keeps a stored autofit scale, which LibreOffice ignores. While the
// result stays readable nothing else reports it, so one advisory per slide
// names the cells and what the renderers do (go-slide-creator-217cd):
// examples/business-model-canvas.json's first canvas, authored at 9pt.
func TestStoredAutofitScaleIsReported(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "business-model-canvas.json"))
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	a := loadTemplateAnalysis(t, "midnight-blue")
	var refit []patterns.FitFinding
	for _, f := range collectReadabilityFindingsWithTheme(&input, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		if f.Code == patterns.ErrCodeTextBelowReadableMin {
			t.Errorf("the canvas text stays readable and must not be refused: %s", f.Message)
		}
		if f.Code == patterns.ErrCodeFitOverflow {
			refit = append(refit, f)
		}
	}
	if len(refit) != 1 {
		t.Fatalf("stored-scale advisories = %d, want one for the first canvas: %+v", len(refit), refit)
	}
	f := refit[0]
	if f.Action != "info" || !strings.HasPrefix(f.Path, "/slides/1/shape_grid/rows/") || !strings.Contains(f.Message, "LibreOffice") {
		t.Errorf("advisory = action %q path %q message %q, want info on a slide-2 cell naming the renderers", f.Action, f.Path, f.Message)
	}
	if f.Fix == nil || f.Fix.Kind != "reduce_cell_text" {
		t.Fatalf("advisory fix = %+v, want reduce_cell_text", f.Fix)
	}
	cells, _ := f.Fix.Params["cells"].([]any)
	scale, _ := f.Fix.Params["font_scale"].(float64)
	if len(cells) < 2 || scale <= 0 || scale >= 1 || f.Fix.Params["cell_path"] == nil {
		t.Errorf("advisory params = %+v, want the cells, the smallest font_scale and the cell_path to cut", f.Fix.Params)
	}

	// Text that fits, or whose shrink is written into its sizes, says nothing.
	var roomy ShapeGridInput
	if err := json.Unmarshal([]byte(`{"columns":2,"rows":[{"cells":[
		{"shape":{"geometry":"rect","fill":"lt2","text":{"content":"Plenty of room","size":14}}},
		{"shape":{"geometry":"rect","fill":"lt2","text":{"content":"Here as well","size":14}}}]}]}`), &roomy); err != nil {
		t.Fatal(err)
	}
	fits := PresentationInput{Slides: []SlideInput{{LayoutID: "content", ShapeGrid: &roomy}}}
	for _, f := range collectReadabilityFindingsWithTheme(&fits, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		if f.Code == patterns.ErrCodeFitOverflow {
			t.Errorf("a grid that stores no scale must not be reported: %s", f.Message)
		}
	}
}
