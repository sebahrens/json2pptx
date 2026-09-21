package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestPyramidDenseTopTierUnbrokenBudget(t *testing.T) {
	p := &pyramid{}
	v := &PyramidValues{Tiers: []string{"Core", "Middle", "Middle", "Base", "Base"}}
	v.Tiers[0] = strings.Repeat("W", 71)
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "tiers[0]") || !strings.Contains(got[0], "about 70") {
		t.Fatalf("dense top-tier warning: %v", got)
	}
	v.Tiers[0] = strings.Repeat("W", 70)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured five-tier target should fit: %v", got)
	}
	v.Tiers[0] = strings.Repeat("word ", 24)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("word-like schema maximum should fit: %v", got)
	}
	v.Tiers = v.Tiers[:3]
	v.Tiers[0] = strings.Repeat("W", 120)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("three-tier schema maximum should fit: %v", got)
	}
}

// testThemeCtx returns an ExpandContext with a midnight-blue-like theme so
// fill-aware text colour selection can resolve scheme colours.
func testThemeCtx() ExpandContext {
	return ExpandContext{
		SlideWidth:  12192000,
		SlideHeight: 6858000,
		Theme: types.ThemeInfo{Colors: []types.ThemeColor{
			{Name: "dk1", RGB: "#000000"},
			{Name: "lt1", RGB: "#FFFFFF"},
			{Name: "dk2", RGB: "#1F2A44"},
			{Name: "lt2", RGB: "#E7EBF1"},
			{Name: "accent1", RGB: "#2E5090"},
			{Name: "accent2", RGB: "#F5C518"},
		}},
	}
}

func TestPyramid_TierWidthsStrictlyIncrease(t *testing.T) {
	p, _ := Default().Get("pyramid")
	for n := 3; n <= 5; n++ {
		tiers := make([]string, n)
		for i := range tiers {
			tiers[i] = fmt.Sprintf("Tier %d", i)
		}
		grid, err := p.Expand(testThemeCtx(), &PyramidValues{Tiers: tiers}, nil, nil)
		if err != nil {
			t.Fatalf("n=%d Expand: %v", n, err)
		}
		var cols []float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil {
			t.Fatalf("n=%d columns: %v", n, err)
		}
		if len(cols) != 2*n-1 {
			t.Fatalf("n=%d: want %d columns, got %d", n, 2*n-1, len(cols))
		}
		prevWidth, prevStart := -1.0, len(cols)
		for i, row := range grid.Rows {
			start, width := 0, 0.0
			var tier *jsonschema.GridCellInput
			for _, c := range row.Cells {
				if c.Shape == nil {
					start++
					continue
				}
				tier = c
			}
			if tier == nil {
				t.Fatalf("n=%d row %d: no tier shape", n, i)
			}
			for k := start; k < start+tier.ColSpan; k++ {
				width += cols[k]
			}
			if width <= prevWidth {
				t.Errorf("n=%d row %d: width %.2f not greater than previous %.2f", n, i, width, prevWidth)
			}
			if start >= prevStart {
				t.Errorf("n=%d row %d: start column %d must move left of %d", n, i, start, prevStart)
			}
			if 2*start+tier.ColSpan != len(cols) {
				t.Errorf("n=%d row %d: tier not centred (start %d span %d of %d)", n, i, start, tier.ColSpan, len(cols))
			}
			prevWidth, prevStart = width, start
		}
		if prevWidth < 99.9 {
			t.Errorf("n=%d: bottom tier should span the full width, got %.2f%%", n, prevWidth)
		}
	}
}

func TestPyramid_TierTextContrast(t *testing.T) {
	p, _ := Default().Get("pyramid")
	ctx := testThemeCtx()
	grid, err := p.Expand(ctx, &PyramidValues{Tiers: []string{"A", "B", "C", "D", "E"}}, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	for i, row := range grid.Rows {
		shape := row.Cells[len(row.Cells)-1].Shape
		var fill struct {
			Color string  `json:"color"`
			Alpha float64 `json:"alpha"`
		}
		if err := json.Unmarshal(shape.Fill, &fill); err != nil {
			t.Fatalf("row %d fill: %v", i, err)
		}
		var text struct {
			Paragraphs []struct {
				Color string `json:"color"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(shape.Text, &text); err != nil {
			t.Fatalf("row %d text: %v", i, err)
		}
		bg, ok := effectiveFillColor(ctx, fillTone{Color: fill.Color, Alpha: fill.Alpha})
		if !ok {
			t.Fatalf("row %d: fill %q unresolved", i, fill.Color)
		}
		fg, _ := resolveThemeColor(ctx, text.Paragraphs[0].Color)
		ratio := fg.ContrastWith(bg)
		if i == len(grid.Rows)-1 && ratio < 4.5 {
			t.Errorf("bottom tier text %s on %s contrast %.2f < 4.5", text.Paragraphs[0].Color, bg.Hex(), ratio)
		}
		if ratio < 3 {
			t.Errorf("row %d: text contrast %.2f < 3", i, ratio)
		}
	}
}

func TestPyramid_TrapezoidAdjAlignsSlopes(t *testing.T) {
	ctx := testThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: 10000000, Height: 4000000}
	cols := pyramidColumns(5)
	adj := pyramidTrapezoidAdj(ctx, 5, cols[0])
	rowH := float64(4000000-4*int64(pyramidRowGapPt*12700)) / 5
	inset := float64(adj) / 100000 * rowH
	want := 10000000 * cols[0] / 100
	if diff := inset - want; diff > 1000 || diff < -1000 {
		t.Errorf("trapezoid inset %.0f EMU, want side column width %.0f", inset, want)
	}
	if got := pyramidTrapezoidAdj(ExpandContext{}, 5, cols[0]); got != 0 {
		t.Errorf("unknown bounds should keep preset adj, got %d", got)
	}
}

func TestPyramid_ExpandBasic(t *testing.T) {
	p, ok := Default().Get("pyramid")
	if !ok {
		t.Fatal("pyramid pattern not registered")
	}

	vals := &PyramidValues{
		Tiers: []string{"Strategy", "Tactics", "Execution"},
	}

	grid, err := p.Expand(ExpandContext{}, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Errorf("expected 3 rows, got %d", len(grid.Rows))
	}
}

func TestPyramid_ValidateTooFew(t *testing.T) {
	p, ok := Default().Get("pyramid")
	if !ok {
		t.Fatal("pyramid pattern not registered")
	}

	vals := &PyramidValues{Tiers: []string{"A", "B"}}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for < 3 tiers")
	}
}

func TestPyramid_ValidateTooMany(t *testing.T) {
	p, ok := Default().Get("pyramid")
	if !ok {
		t.Fatal("pyramid pattern not registered")
	}

	vals := &PyramidValues{Tiers: []string{"A", "B", "C", "D", "E", "F"}}
	err := p.Validate(vals, nil, nil)
	if err == nil {
		t.Error("expected validation error for > 5 tiers")
	}
}
