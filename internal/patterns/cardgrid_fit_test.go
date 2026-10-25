package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestOpenCardAdviceAndAutoShapeUseWrittenFit(t *testing.T) {
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: 800 * 12700, Height: 180 * 12700}
	vals := &CardGridValues{Cells: make([]CardGridCell, 4)}
	for i := range vals.Cells {
		vals.Cells[i] = CardGridCell{Header: "Clear recommendation", Body: strings.Repeat("Complete the rollout with local teams. ", 5)}
	}
	pat := &cardGrid{}
	for _, columns := range []int{0, 4} {
		vals.Columns = columns
		grid, err := pat.Expand(ctx, vals, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		var gotColumns int
		if err := json.Unmarshal(grid.Columns, &gotColumns); err != nil {
			t.Fatal(err)
		}
		if gotColumns != 4 || cardGridHeight(grid) > 180 {
			t.Fatalf("columns=%d -> %d, height %.1f", columns, gotColumns, cardGridHeight(grid))
		}
		if warnings := pat.PostExpandWarnings(ctx, vals, nil); len(warnings) > 0 {
			t.Fatalf("fitting open cards warned: %v", warnings)
		}
		result := resolveGridAt(t, grid, pptx.RectEmu{CX: 800 * 12700, CY: 180 * 12700})
		for _, cell := range result.Cells {
			if cell.ShapeSpec == nil || len(cell.ShapeSpec.Text) == 0 {
				continue
			}
			tb, err := shapegrid.ResolveTextInput(cell.ShapeSpec.Text)
			if err != nil {
				t.Fatal(err)
			}
			if scale := pptx.AutofitScaleFor(tb, cell.Bounds); scale < 0.999 {
				t.Errorf("auto arrangement shrank text: %.3f", scale)
			}
		}
	}
	// Authored dimensions stand even when a different arrangement fits better.
	vals.Columns = 2
	grid, err := pat.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(grid.Columns) != "2" {
		t.Fatalf("explicit columns changed: %s", grid.Columns)
	}
	for i := range vals.Cells {
		vals.Cells[i].Body = strings.Repeat("W", 300)
	}
	if warnings := pat.PostExpandWarnings(ctx, vals, nil); len(warnings) == 0 {
		t.Fatal("true open-card overflow not reported")
	}
}

func TestPairedQuotesUseTwoCompleteEditableColumns(t *testing.T) {
	for _, style := range quoteClusterStyles {
		pat := &quoteCluster{}
		grid, err := pat.Expand(fullThemeCtx(), validQuoteClusterValues(2), &QuoteClusterOverrides{Style: style}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(grid.Columns) != "2" {
			t.Fatalf("%s pair: columns=%s", style, grid.Columns)
		}
		if _, err := json.Marshal(grid); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(grid)
		for _, name := range []string{"J. Lin", "P. Reyes"} {
			if !strings.Contains(string(encoded), name) {
				t.Errorf("%s lost attribution %s", style, name)
			}
		}
	}
}

func TestCardGridFitAcrossAspectFontsAndDecoration(t *testing.T) {
	for _, font := range []string{"Liberation Sans", "Segoe UI"} {
		for _, style := range []string{"open", "filled"} {
			t.Run(font+"/"+style, func(t *testing.T) {
				ctx := fullThemeCtx()
				ctx.Theme.BodyFont = font
				ctx.LayoutBounds = LayoutBounds{Width: 200 * 12700, Height: 800 * 12700}
				vals := &CardGridValues{Cells: make([]CardGridCell, 4)}
				for i := range vals.Cells {
					vals.Cells[i] = CardGridCell{Header: "Clear recommendation for local delivery", Body: "Local owners complete the rollout.", Icon: &IconRef{Name: "rocket"}}
				}
				pat := &cardGrid{}
				ovr := &CardGridOverrides{Style: style}
				grid, err := pat.Expand(ctx, vals, ovr, nil)
				if err != nil {
					t.Fatal(err)
				}
				if (style == "open" && string(grid.Columns) != "1") || cardGridHeight(grid) > 800 {
					t.Fatalf("tall narrow grid: columns=%s height=%.1f", grid.Columns, cardGridHeight(grid))
				}
				if warnings := pat.PostExpandWarnings(ctx, vals, ovr); len(warnings) != 0 {
					t.Fatalf("fitting decorated cards warned: %v", warnings)
				}
			})
		}
	}
}
