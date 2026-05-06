package textcapacity

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// emuInch is 914400 EMU (1 inch).
const emuInch = int64(types.EMUPerInch)

func TestComputeBudget_MonotonicallyDecreasesWithCellSize(t *testing.T) {
	// At 12pt body font, budgets should shrink as cells shrink.
	sizes := []struct {
		name string
		w, h int64
	}{
		{"3x2 cell", 3 * emuInch, 2 * emuInch},
		{"2x2 cell", 2 * emuInch, 2 * emuInch},
		{"2x1 cell", 2 * emuInch, 1 * emuInch},
		{"1x1 cell", 1 * emuInch, 1 * emuInch},
	}

	var prevChars int
	for _, sz := range sizes {
		b := computeBudget(sz.w, sz.h, 12.0)
		if b.MaxChars <= 0 {
			t.Errorf("%s: MaxChars=%d, want > 0", sz.name, b.MaxChars)
			continue
		}
		if prevChars > 0 && b.MaxChars >= prevChars {
			t.Errorf("%s: MaxChars=%d >= previous %d, expected monotonic decrease", sz.name, b.MaxChars, prevChars)
		}
		prevChars = b.MaxChars
		t.Logf("%s: MaxChars=%d, MaxLines=%d", sz.name, b.MaxChars, b.MaxLines)
	}
}

func TestComputeBudget_ShrinkWithFontSize(t *testing.T) {
	// Same cell dimensions, increasing font size → fewer characters.
	w, h := 3*emuInch, 2*emuInch
	fontSizes := []float64{10, 12, 16, 24}

	var prevChars int
	for _, pt := range fontSizes {
		b := computeBudget(w, h, pt)
		if b.MaxChars <= 0 {
			t.Errorf("%.0fpt: MaxChars=%d, want > 0", pt, b.MaxChars)
			continue
		}
		if prevChars > 0 && b.MaxChars >= prevChars {
			t.Errorf("%.0fpt: MaxChars=%d >= previous %d, expected decrease as font grows", pt, b.MaxChars, prevChars)
		}
		prevChars = b.MaxChars
		t.Logf("%.0fpt: MaxChars=%d, MaxLines=%d", pt, b.MaxChars, b.MaxLines)
	}
}

func TestComputeBudget_GridSizes(t *testing.T) {
	// Test that budgets for 2x2, 3x2, and 3x3 grids at 12pt body, 16pt header
	// produce monotonically decreasing body budgets as cells shrink.
	gridW := int64(8 * emuInch)
	gridH := int64(4 * emuInch)

	grids := []struct {
		name     string
		cols     int
		rows     int
		bodyFont float64
	}{
		{"2x2 at 12pt", 2, 2, 12},
		{"3x2 at 12pt", 3, 2, 12},
		{"3x3 at 12pt", 3, 3, 12},
	}

	var prevChars int
	for _, g := range grids {
		cellW := gridW / int64(g.cols)
		cellH := gridH / int64(g.rows)
		b := computeBudget(cellW, cellH, g.bodyFont)
		if b.MaxChars <= 0 {
			t.Errorf("%s: MaxChars=%d, want > 0", g.name, b.MaxChars)
			continue
		}
		if prevChars > 0 && b.MaxChars >= prevChars {
			t.Errorf("%s: MaxChars=%d >= previous %d, expected decrease", g.name, b.MaxChars, prevChars)
		}
		prevChars = b.MaxChars
		t.Logf("%s: cell=%dx%d EMU, MaxChars=%d, MaxLines=%d",
			g.name, cellW, cellH, b.MaxChars, b.MaxLines)
	}
}

func TestComputeBudget_ZeroDimensions(t *testing.T) {
	b := computeBudget(0, emuInch, 12)
	if b.MaxChars != 0 {
		t.Errorf("zero width: MaxChars=%d, want 0", b.MaxChars)
	}
	b = computeBudget(emuInch, 0, 12)
	if b.MaxChars != 0 {
		t.Errorf("zero height: MaxChars=%d, want 0", b.MaxChars)
	}
}

func TestComputeBudget_VerySmallCell(t *testing.T) {
	// A cell too small for even one line should return MaxChars >= 0 without panic.
	b := computeBudget(10000, 10000, 12)
	t.Logf("tiny cell: MaxChars=%d, MaxLines=%d", b.MaxChars, b.MaxLines)
}

func TestForPlaceholder(t *testing.T) {
	ph := types.PlaceholderInfo{
		Bounds: types.BoundingBox{
			Width:  7 * int64(emuInch),
			Height: 4 * int64(emuInch),
		},
		FontSize: 1400, // 14pt
	}

	d := ForPlaceholder(ph, "Hello World")
	if d.MaxChars <= 0 {
		t.Errorf("MaxChars=%d, want > 0", d.MaxChars)
	}
	if d.ActualChars != 11 {
		t.Errorf("ActualChars=%d, want 11", d.ActualChars)
	}
	if d.FontPt != 14.0 {
		t.Errorf("FontPt=%.1f, want 14.0", d.FontPt)
	}
	t.Logf("Placeholder: MaxChars=%d, ActualChars=%d, DensityPct=%d, Status=%s",
		d.MaxChars, d.ActualChars, d.DensityPct, d.Status)
}

func TestForPlaceholder_Markdown(t *testing.T) {
	ph := types.PlaceholderInfo{
		Bounds: types.BoundingBox{
			Width:  4 * int64(emuInch),
			Height: 2 * int64(emuInch),
		},
		FontSize: 1100, // 11pt
	}

	// "**bold**" has 4 visual characters ("bold"), not 8.
	d := ForPlaceholder(ph, "**bold**")
	if d.ActualChars != 4 {
		t.Errorf("ActualChars=%d, want 4 (markdown stripped)", d.ActualChars)
	}
}

func TestForPlaceholder_DefaultFont(t *testing.T) {
	ph := types.PlaceholderInfo{
		Bounds: types.BoundingBox{
			Width:  4 * int64(emuInch),
			Height: 2 * int64(emuInch),
		},
		// FontSize: 0 → default 11pt
	}

	d := ForPlaceholder(ph, "test")
	if d.FontPt != 11.0 {
		t.Errorf("FontPt=%.1f, want 11.0 (default)", d.FontPt)
	}
}

func TestForResolvedGrid_NilResult(t *testing.T) {
	d := ForResolvedGrid(nil)
	if d != nil {
		t.Errorf("nil result: got %v, want nil", d)
	}
}

func TestForResolvedGrid_MixedCells(t *testing.T) {
	textJSON := json.RawMessage(`"Hello World"`)
	result := &shapegrid.ResolveResult{
		Cells: []shapegrid.ResolvedCell{
			{
				Kind:       shapegrid.CellKindShape,
				CellBounds: pptx.RectEmu{CX: 3 * emuInch, CY: 2 * emuInch},
				ShapeSpec: &shapegrid.ShapeSpec{
					Text: textJSON,
				},
			},
			{
				Kind:       shapegrid.CellKindImage,
				CellBounds: pptx.RectEmu{CX: 3 * emuInch, CY: 2 * emuInch},
			},
			{
				Kind:       shapegrid.CellKindShape,
				CellBounds: pptx.RectEmu{CX: 2 * emuInch, CY: emuInch},
				ShapeSpec: &shapegrid.ShapeSpec{
					Text: json.RawMessage(`{"content": "Short", "size": 16}`),
				},
			},
		},
	}

	densities := ForResolvedGrid(result)
	if len(densities) != 3 {
		t.Fatalf("got %d densities, want 3", len(densities))
	}

	// Cell 0: shape with text
	if densities[0].MaxChars <= 0 {
		t.Errorf("cell 0: MaxChars=%d, want > 0", densities[0].MaxChars)
	}
	if densities[0].ActualChars != 11 {
		t.Errorf("cell 0: ActualChars=%d, want 11", densities[0].ActualChars)
	}

	// Cell 1: image — zero density, underfilled status
	if densities[1].MaxChars != 0 {
		t.Errorf("cell 1 (image): MaxChars=%d, want 0", densities[1].MaxChars)
	}
	if densities[1].Status != StatusUnderfilled {
		t.Errorf("cell 1 (image): Status=%q, want %q", densities[1].Status, StatusUnderfilled)
	}

	// Cell 2: shape at 16pt — should have fewer MaxChars than cell 0 (smaller cell, bigger font)
	if densities[2].MaxChars <= 0 {
		t.Errorf("cell 2: MaxChars=%d, want > 0", densities[2].MaxChars)
	}
	if densities[2].MaxChars >= densities[0].MaxChars {
		t.Errorf("cell 2 (smaller, bigger font) MaxChars=%d >= cell 0 MaxChars=%d",
			densities[2].MaxChars, densities[0].MaxChars)
	}
	if densities[2].FontPt != 16.0 {
		t.Errorf("cell 2: FontPt=%.1f, want 16.0", densities[2].FontPt)
	}

	t.Logf("cell 0: %+v", densities[0])
	t.Logf("cell 1: %+v", densities[1])
	t.Logf("cell 2: %+v", densities[2])
}

func TestDensityStatus(t *testing.T) {
	tests := []struct {
		actual, max int
		wantStatus  Status
	}{
		{0, 100, StatusUnderfilled},
		{50, 100, StatusUnderfilled},
		{59, 100, StatusUnderfilled},
		{60, 100, StatusOptimal},
		{100, 100, StatusOptimal},
		{110, 100, StatusOptimal},
		{111, 100, StatusOverflow},
		{200, 100, StatusOverflow},
	}
	for _, tt := range tests {
		b := Budget{MaxChars: tt.max}
		d := buildDensity(b, tt.actual)
		if d.Status != tt.wantStatus {
			t.Errorf("actual=%d max=%d: Status=%q, want %q",
				tt.actual, tt.max, d.Status, tt.wantStatus)
		}
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain text", "plain text"},
		{"**bold**", "bold"},
		{"*italic*", "italic"},
		{"***bold-italic***", "bold-italic"},
		{"no asterisks", "no asterisks"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripMarkdown(tt.input)
		if got != tt.want {
			t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBudgetDeterminism(t *testing.T) {
	// Same inputs must always produce the same output (deterministic metrics).
	w, h := 4*emuInch, 2*emuInch
	b1 := computeBudget(w, h, 12.0)
	b2 := computeBudget(w, h, 12.0)
	if b1.MaxChars != b2.MaxChars || b1.MaxLines != b2.MaxLines {
		t.Errorf("non-deterministic: first=%+v, second=%+v", b1, b2)
	}
}
