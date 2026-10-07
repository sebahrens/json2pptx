package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// assertTakeawayBand checks one band row against the shared takeaway spec
// (go-slide-creator-7b5o6): a flush accent bar cell, then an unfilled,
// unstroked text cell carrying wantText in 14pt-class bold dk1 ink,
// top-anchored and inset 12pt from the bar.
func assertTakeawayBand(t *testing.T, band jsonschema.GridRowInput, wantText string) {
	t.Helper()
	if len(band.Cells) != 2 {
		t.Fatalf("takeaway band cells = %d, want 2 (bar, text)", len(band.Cells))
	}
	bar, text := band.Cells[0], band.Cells[1]
	if bar.Shape == nil || bar.Shape.Text != nil || string(bar.Shape.Line) != `"none"` {
		t.Errorf("bar cell = %+v, want a textless, unstroked rect", bar.Shape)
	}
	if !strings.HasPrefix(string(bar.Shape.Fill), `"accent`) {
		t.Errorf("bar fill = %s, want an accent", bar.Shape.Fill)
	}
	if text.Shape == nil || string(text.Shape.Fill) != `"none"` || string(text.Shape.Line) != `"none"` {
		t.Fatalf("text cell = %+v, want no fill and no stroke", text.Shape)
	}
	var txt takeawayText
	if err := json.Unmarshal(text.Shape.Text, &txt); err != nil {
		t.Fatal(err)
	}
	p := txt.Paragraphs[0]
	if p.Content != wantText || !p.Bold || p.Color != TakeawayInk || p.Size < 12 {
		t.Errorf("takeaway paragraph = %+v, want %q bold %s", p, wantText, TakeawayInk)
	}
	if txt.VerticalAlign != "t" {
		t.Errorf("vertical_align = %q, want top-anchored", txt.VerticalAlign)
	}
	if txt.InsetLeft == nil || *txt.InsetLeft != TakeawayTextInsetPt {
		t.Errorf("inset_left = %v, want %v", txt.InsetLeft, TakeawayTextInsetPt)
	}
}

// assertTakeawayFilledBand holds a cell to the "band" variant
// (go-slide-creator-3a1rm): one shape in the dark structural neutral — never
// an accent — with bold text in the ink measured against it, 12pt in from
// both edges, centred on the band's height, no outline.
func assertTakeawayFilledBand(t *testing.T, ctx ExpandContext, cell *jsonschema.GridCellInput, wantText string) {
	t.Helper()
	if cell == nil || cell.Shape == nil {
		t.Fatalf("takeaway band cell = %+v, want a shape", cell)
	}
	wantFill, wantInk := TakeawayBandTone(ctx)
	if string(cell.Shape.Fill) != string(wantFill) || strings.Contains(string(cell.Shape.Fill), "accent") || string(cell.Shape.Line) != `"none"` {
		t.Errorf("band fill/line = %s/%s, want %s (a neutral, not an accent) and no outline", cell.Shape.Fill, cell.Shape.Line, wantFill)
	}
	var txt takeawayText
	if err := json.Unmarshal(cell.Shape.Text, &txt); err != nil {
		t.Fatal(err)
	}
	p := txt.Paragraphs[0]
	if p.Content != wantText || !p.Bold || p.Color != wantInk || p.Size < 12 {
		t.Errorf("takeaway paragraph = %+v, want %q bold in %s", p, wantText, wantInk)
	}
	if txt.VerticalAlign != "ctr" {
		t.Errorf("vertical_align = %q, want centred on the band", txt.VerticalAlign)
	}
	if txt.InsetLeft == nil || *txt.InsetLeft != TakeawayTextInsetPt || txt.InsetRight == nil || *txt.InsetRight != TakeawayTextInsetPt {
		t.Errorf("side insets = %v / %v, want %v", txt.InsetLeft, txt.InsetRight, TakeawayTextInsetPt)
	}
}

// The "band" variant is a full-width band of the host grid itself: a spacer
// row, then one filled cell spanning the host's columns, flush with the rules
// above it (a nested grid would stand 4pt in on each side). The default is
// the bar.
func TestTakeawayRows_Band(t *testing.T) {
	ctx := fullThemeCtx()
	spec := TakeawaySpec{Text: "Fund the build-out now.", Emphasis: TakeawayEmphasisBand}
	rows := TakeawayRows(ctx, spec, 3, 800, 2)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want spacer + band", len(rows))
	}
	if got, want := rows[0].MinHeight, TakeawayGapAbovePt-2*2; got != want || rows[0].MaxHeight != want {
		t.Errorf("spacer = %v, want %v (16pt of air less the host gap above and below it)", got, want)
	}
	band := rows[1]
	if len(band.Cells) != 1 || band.Cells[0].ColSpan != 3 || band.Cells[0].Grid != nil {
		t.Fatalf("band row = %+v, want one shape cell spanning 3 columns", band)
	}
	assertTakeawayFilledBand(t, ctx, band.Cells[0], spec.Text)
	if !band.AutoHeight || band.MinHeight != TakeawayBandHeightPt(ctx, spec, 800) || band.MaxHeight != 0 {
		t.Errorf("band row = auto %v min %v max %v, want an auto row floored at its measured height", band.AutoHeight, band.MinHeight, band.MaxHeight)
	}
	if got, want := TakeawayRowHeightPt(ctx, spec, 800, 2), rows[0].MinHeight+2+band.MinHeight; got != want {
		t.Errorf("TakeawayRowHeightPt = %v, want %v (spacer + host gap + band)", got, want)
	}
	// A host gap of 8pt or more is the air: no spacer row.
	if rows := TakeawayRows(ctx, spec, 1, 800, 8); len(rows) != 1 {
		t.Errorf("rows at an 8pt host gap = %d, want the band alone", len(rows))
	}
	// The default, which "bar" names, is the nested-grid row with the accent
	// bar: no fill behind the text.
	for _, e := range []string{TakeawayEmphasisNone, TakeawayEmphasisBar} {
		rows := TakeawayRows(ctx, TakeawaySpec{Text: "x", Emphasis: e}, 1, 800, 8)
		if len(rows) != 1 || rows[0].Cells[0].Grid == nil {
			t.Fatalf("emphasis %q rows = %+v, want one nested-grid row", e, rows)
		}
		sub := rows[0].Cells[0].Grid.Rows
		assertTakeawayBand(t, sub[len(sub)-1], "x")
	}
	// On a template whose dk2 is black the band is a charcoal of dk1.
	fill, ink := TakeawayBandTone(ExpandContext{Theme: types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#000000"}, {Name: "lt2", RGB: "#FFFFFF"}, {Name: "accent1", RGB: "#FD5108"},
	}}})
	if !strings.Contains(string(fill), `"dk1"`) || !strings.Contains(string(fill), "85000") || ink != "lt1" {
		t.Errorf("band on a black-dk2 theme = %s with %s ink, want dk1 at 85%% with lt1", fill, ink)
	}
}

func TestTakeawayRow_Bar(t *testing.T) {
	ctx := fullThemeCtx()
	spec := TakeawaySpec{Text: "Fund the build-out now.", Emphasis: TakeawayEmphasisBar}
	row := TakeawayRow(ctx, spec, 3, 800, 8)
	if row.Cells[0].ColSpan != 3 || row.Cells[0].Grid == nil {
		t.Fatalf("row = %+v, want a sub-grid spanning 3 columns", row)
	}
	sub := row.Cells[0].Grid
	if len(sub.Rows) != 2 {
		t.Fatalf("sub-grid rows = %d, want spacer + band", len(sub.Rows))
	}
	if got := sub.Rows[0].MinHeight; got != TakeawayGapAbovePt-8-SubGridInsetPt {
		t.Errorf("spacer = %v, want %v (16pt of air minus the host row gap and the sub-grid inset)", got, TakeawayGapAbovePt-8-SubGridInsetPt)
	}
	assertTakeawayBand(t, sub.Rows[1], spec.Text)
	if want := TakeawayRowHeightPt(ctx, spec, 800, 8); !row.AutoHeight || row.MinHeight != want || row.MaxHeight != 0 {
		t.Errorf("row = auto %v min %v max %v, want an auto row floored at %v (a max would switch the host grid to content-sized rows)", row.AutoHeight, row.MinHeight, row.MaxHeight, want)
	}
	var cols []float64
	if err := json.Unmarshal(sub.Columns, &cols); err != nil {
		t.Fatal(err)
	}
	if barPt := cols[0] / 100 * (800 - 2*SubGridInsetPt); barPt < 2.99 || barPt > 3.01 {
		t.Errorf("bar width = %.2fpt, want 3pt", barPt)
	}
}

func TestTakeawayRow_Emphasis(t *testing.T) {
	ctx := fullThemeCtx()
	subtle := TakeawayGrid(ctx, TakeawaySpec{Text: "x", Emphasis: TakeawayEmphasisSubtle}, 800, 0, 30)
	fill := string(subtle.Rows[0].Cells[1].Shape.Fill)
	if !strings.Contains(fill, `"dk1"`) || !strings.Contains(fill, `"alpha":5`) {
		t.Errorf("subtle fill = %s, want a 5%% dk1 tint", fill)
	}
	strong := TakeawayGrid(ctx, TakeawaySpec{Text: "x", Emphasis: TakeawayEmphasisStrong}, 800, 0, 30)
	cell := strong.Rows[0].Cells[1].Shape
	if string(cell.Fill) != `"accent1"` || string(cell.Line) != `"none"` {
		t.Errorf("strong band = fill %s line %s, want solid accent1, no stroke", cell.Fill, cell.Line)
	}
	var txt takeawayText
	_ = json.Unmarshal(cell.Text, &txt)
	if ink := txt.Paragraphs[0].Color; ink != "lt1" {
		t.Errorf("strong ink on #2E5090 = %q, want lt1 (measured contrast)", ink)
	}
	if !ValidTakeawayEmphasis("") || !ValidTakeawayEmphasis("bar") || !ValidTakeawayEmphasis("band") || !ValidTakeawayEmphasis("strong") || ValidTakeawayEmphasis("bold") {
		t.Error("ValidTakeawayEmphasis accepts only \"\", bar, band, subtle, strong")
	}
}

func TestTakeawayRow_WrapsToTwoLines(t *testing.T) {
	ctx := fullThemeCtx()
	one := TakeawayBandHeightPt(ctx, TakeawaySpec{Text: "Short."}, 800)
	two := TakeawayBandHeightPt(ctx, TakeawaySpec{Text: strings.Repeat("A long recommendation that wraps. ", 5)}, 800)
	if two-one < TakeawaySizePt {
		t.Errorf("two-line band %.1fpt should be at least a line taller than one-line %.1fpt", two, one)
	}
}
