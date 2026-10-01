package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Patterns that size their own rows must never hand the writer a row shorter
// than the written fit of its text: on the short content areas of the shipped
// templates (abstract 687×294pt, modern 851×311pt) their exemplars were stored
// with normAutofit shrinks that took 12–14pt runs to 9–11.8pt, below the 12pt
// presentation floor, and generation refused them (go-slide-creator-k3eb3).
// The pattern must either lay the content out readably — row gaps, hub width,
// type steps down to the floor — or report BODY_TOO_LONG itself.
func TestShortContentAreaExemplarsStayAboveFloor(t *testing.T) {
	areas := []struct {
		name string
		w, h float64
	}{
		{"abstract", 687, 294},
		{"modern", 851, 311},
		{"warm-coral", 828, 349},
	}
	for _, name := range []string{"state-shift-hub", "agenda-with-images", "exec-summary", "metric-list", "next-steps", "scqa-summary", "bmc-canvas"} {
		p, ok := Default().Get(name)
		if !ok {
			t.Fatalf("pattern %q not registered", name)
		}
		for _, a := range areas {
			t.Run(name+"/"+a.name, func(t *testing.T) {
				ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}}
				values := p.(Exemplar).ExemplarValues()
				if w, ok := p.(PostExpandWarner); ok {
					for _, msg := range w.PostExpandWarnings(ctx, values, nil) {
						t.Errorf("exemplar reports %s", msg)
					}
				}
				grid, err := p.Expand(ctx, values, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				ApplyGridDefaults(grid)
				res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
				for _, c := range res.Cells {
					if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
						continue
					}
					tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
					if err != nil || tb == nil {
						continue
					}
					for j := range tb.Insets {
						tb.Insets[j] += c.TextInsets[j]
					}
					smallest := smallestRunPt(tb)
					if smallest < 12 {
						continue // captions and footnotes have their own floors
					}
					if scale := pptx.AutofitScaleFor(tb, c.Bounds); smallest*scale < 12 {
						t.Errorf("%q is written at %.0fpt × %.0f%% = %.1fpt in a %.0f×%.0fpt shape, below the 12pt floor",
							firstText(tb), smallest, scale*100, smallest*scale, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700)
					}
				}
			})
		}
	}
}

// resolveGridAt resolves an expanded grid at bounds, carrying the row sizing
// fields content-sized patterns use; nested sub-grids reserve their cell.
func resolveGridAt(t *testing.T, in *jsonschema.ShapeGridInput, bounds pptx.RectEmu) *shapegrid.ResolveResult {
	t.Helper()
	var cols []float64
	if err := json.Unmarshal(in.Columns, &cols); err != nil {
		var n int
		if err := json.Unmarshal(in.Columns, &n); err != nil {
			t.Fatalf("columns: %v", err)
		}
		for i := 0; i < n; i++ {
			cols = append(cols, 100/float64(n))
		}
	}
	rows := make([]shapegrid.Row, len(in.Rows))
	for i, r := range in.Rows {
		cells := make([]shapegrid.Cell, len(r.Cells))
		for j, c := range r.Cells {
			if c == nil {
				continue
			}
			cells[j] = shapegrid.Cell{ColSpan: c.ColSpan, RowSpan: c.RowSpan, Fit: shapegrid.FitMode(c.Fit), Placeholder: c.Grid != nil}
			if c.Shape != nil {
				cells[j].Shape = &shapegrid.ShapeSpec{Geometry: c.Shape.Geometry, TypeScale: c.Shape.TypeScale, Fill: c.Shape.Fill, Line: c.Shape.Line, Text: c.Shape.Text}
			}
		}
		rows[i] = shapegrid.Row{Cells: cells, Height: r.Height, AutoHeight: r.AutoHeight, Flex: r.Flex, MinHeight: r.MinHeight, MaxHeight: r.MaxHeight}
	}
	colGap, rowGap := in.ColGap, in.RowGap
	if colGap == 0 {
		colGap = in.Gap
	}
	if rowGap == 0 {
		rowGap = in.Gap
	}
	vAlign, _ := shapegrid.ParseVerticalAlign(in.VerticalAlign)
	g := &shapegrid.Grid{Bounds: bounds, Columns: cols, Rows: rows, ColGap: colGap, RowGap: rowGap, VAlign: vAlign, TypeScale: in.TypeScale}
	if err := shapegrid.Validate(g); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := shapegrid.Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return res
}

func smallestRunPt(tb *pptx.TextBody) float64 {
	smallest := 0
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if strings.TrimSpace(r.Text) != "" && r.FontSize > 0 && (smallest == 0 || r.FontSize < smallest) {
				smallest = r.FontSize
			}
		}
	}
	return float64(smallest) / 100
}

func firstText(tb *pptx.TextBody) string {
	for _, p := range tb.Paragraphs {
		for _, r := range p.Runs {
			if s := strings.TrimSpace(r.Text); s != "" {
				return s
			}
		}
	}
	return ""
}
