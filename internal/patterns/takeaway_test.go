package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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

func TestTakeawayRow_Default(t *testing.T) {
	ctx := fullThemeCtx()
	spec := TakeawaySpec{Text: "Fund the build-out now."}
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
	if !ValidTakeawayEmphasis("") || !ValidTakeawayEmphasis("strong") || ValidTakeawayEmphasis("bold") {
		t.Error("ValidTakeawayEmphasis accepts only \"\", subtle, strong")
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
