package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/tokens"
)

func kpiSlide(t *testing.T, small, sub string) SlideInput {
	t.Helper()
	values := []map[string]string{
		{"big": "$52M", "small": small, "sub": sub},
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
	long := strings.Repeat("up from $41M last year, driven by enterprise expansion and new logos in EMEA ", 2)
	in := &PresentationInput{ViewingMode: "present", Slides: []SlideInput{kpiSlide(t, "Annual recurring revenue (all segments)", long)}}
	got := readabilityCodes(collectReadabilityFindings(in, nil, 12192000, 6858000))
	if len(got) == 0 {
		t.Fatal("expected TEXT_BELOW_READABLE_MIN for the long KPI label")
	}
	f := got[0]
	if !strings.Contains(f.Path, "/shape_grid/") || !strings.HasSuffix(f.Path, "shape/text") {
		t.Errorf("path = %q, want a shape_grid cell text path", f.Path)
	}
	if f.Fix == nil || f.Fix.Kind != "reduce_text" {
		t.Fatalf("fix = %+v", f.Fix)
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

	// Short labels are fine.
	short := &PresentationInput{Slides: []SlideInput{kpiSlide(t, "ARR", "+4% vs plan")}}
	if got := readabilityCodes(collectReadabilityFindings(short, nil, 12192000, 6858000)); len(got) != 0 {
		t.Errorf("short KPI labels flagged: %v", got[0].Message)
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
		pt    float64
		bold  bool
		chars int
		want  tokens.TextRole
	}{
		{36, true, 4, tokens.TextRoleKPIValue},
		{14, true, 20, tokens.TextRoleCardTitle},
		{12, false, 12, tokens.TextRoleCaption},
		{12, false, 200, tokens.TextRoleCardBody},
	}
	for _, c := range cases {
		if got := cellTextRole(c.pt, c.bold, c.chars); got != c.want {
			t.Errorf("cellTextRole(%v,%v,%d) = %s, want %s", c.pt, c.bold, c.chars, got, c.want)
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

func TestViewingModeEnumValidated(t *testing.T) {
	errs := checkInputEnumValues(&PresentationInput{ViewingMode: "projector"})
	if len(errs) == 0 {
		t.Fatal("unknown viewing_mode should be rejected")
	}
	if errs := checkInputEnumValues(&PresentationInput{ViewingMode: "read"}); len(errs) != 0 {
		t.Errorf("read should be valid: %v", errs[0])
	}
}
