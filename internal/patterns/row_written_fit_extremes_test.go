package patterns

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// Rows sized from the theme-font model alone can be handed to the writer
// shorter than the written fit of their own text; on the short content areas
// the stored normAutofit shrink then takes 12pt runs below the presentation
// floor (go-slide-creator-n1muf). For every extreme but legal payload below,
// on every short content area, a pattern must either write its text without
// shrinking it below 12pt or report BODY_TOO_LONG / TEXT_EXCEEDS_SHAPE itself
// (the agent then shortens the copy; content is never truncated).
func TestExtremePayloadsWriteAboveFloorOrWarn(t *testing.T) {
	areas := []struct {
		name string
		w, h float64
	}{
		{"abstract-short", 687, 294},
		{"modern", 851, 311},
		{"warm-coral", 828, 349},
		{"midnight-blue", 797, 349},
		{"modern-template", 824, 336},
		{"p-style", 899, 360},
	}
	for _, tc := range rowFitExtremeCases() {
		p, ok := Default().Get(tc.pattern)
		if !ok {
			t.Fatalf("pattern %q not registered", tc.pattern)
		}
		values := p.NewValues()
		if err := json.Unmarshal([]byte(tc.values), values); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err := p.Validate(values, nil, nil); err != nil {
			t.Fatalf("%s: payload is not legal: %v", tc.name, err)
		}
		for _, a := range areas {
			t.Run(tc.name+"/"+a.name, func(t *testing.T) {
				ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}}
				warned := ""
				if w, ok := p.(PostExpandWarner); ok {
					for _, msg := range w.PostExpandWarnings(ctx, values, nil) {
						if strings.HasPrefix(msg, ErrCodeBodyTooLong) || strings.HasPrefix(msg, ErrCodeTextExceedsShape) {
							warned = msg
						}
					}
				}
				grid, err := p.Expand(ctx, values, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				ApplyGridDefaults(grid)
				bounds := pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
				shrunk := writtenBelowFloor(t, grid, bounds)
				if len(shrunk) > 0 && warned == "" {
					t.Errorf("written below the 12pt floor without BODY_TOO_LONG: %s", strings.Join(shrunk, "; "))
				}
				if tc.mustFit && warned != "" {
					t.Errorf("payload the pattern documents as fitting reports %q", warned)
				}
			})
		}
	}
}

type rowFitExtremeCase struct {
	name, pattern, values string
	// mustFit marks payloads inside the pattern's documented budget: they
	// must lay out readably, not merely warn.
	mustFit bool
}

// rowFitWords builds n characters of ordinary worded copy.
func rowFitWords(n, seed int) string {
	words := strings.Fields("operational resilience requires sustained investment across regional distribution centres while customer retention improves through faster onboarding and better service quality measured quarterly by independent reviewers")
	var out []string
	for i := seed; ; i++ {
		next := append(append([]string(nil), out...), words[i%len(words)])
		if len(strings.Join(next, " ")) > n {
			break
		}
		out = next
	}
	if len(out) == 0 {
		return words[seed%len(words)][:n]
	}
	return strings.Join(out, " ")
}

func rowFitJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func rowFitExtremeCases() []rowFitExtremeCase {
	var cases []rowFitExtremeCase
	add := func(name, pattern string, mustFit bool, v any) {
		cases = append(cases, rowFitExtremeCase{name: pattern + "/" + name, pattern: pattern, values: rowFitJSON(v), mustFit: mustFit})
	}

	// icon-row: 3–5 captions of up to 60 characters.
	for _, n := range []int{3, 4, 5} {
		var items []map[string]any
		for i := 0; i < n; i++ {
			items = append(items, map[string]any{"icon": "rocket", "caption": rowFitWords(60, i)})
		}
		add(fmt.Sprintf("%dx60", n), "icon-row", true, items)
	}

	// labeled-rows at the documented per-row budgets.
	lr := func(rows, body, sub int, label string) map[string]any {
		var out []map[string]any
		for i := 0; i < rows; i++ {
			r := map[string]any{"label": label, "body": rowFitWords(body, i+3)}
			if sub > 0 {
				r["sublabel"] = rowFitWords(sub, i)
			}
			out = append(out, r)
		}
		return map[string]any{"rows": out}
	}
	// The per-row budgets assume a typical content area; on the short areas
	// the pattern reports BODY_TOO_LONG for them rather than shrinking.
	add("3x150", "labeled-rows", true, lr(3, 150, 0, "Faster"))
	add("3x300", "labeled-rows", false, lr(3, 300, 60, "Operational Excellence"))
	add("4x211", "labeled-rows", false, lr(4, 211, 0, "Faster"))
	add("6x101", "labeled-rows", false, lr(6, 101, 0, "Faster"))
	add("6x101-sublabels", "labeled-rows", false, lr(6, 101, 30, "Faster"))
	add("6x300", "labeled-rows", false, lr(6, 300, 60, "Operational Excellence"))

	// matrix-2x2: long worded quadrants, with and without icons.
	quad := func(i, header, body int, icon bool) map[string]any {
		q := map[string]any{"header": rowFitWords(header, i), "body": rowFitWords(body, i+5)}
		if icon {
			q["icon"] = "rocket"
		}
		return q
	}
	mx := func(header, body int, icon bool) map[string]any {
		return map[string]any{
			"x_axis_label": "Customer Value", "y_axis_label": rowFitWords(40, 2),
			"top_left": quad(0, header, body, icon), "top_right": quad(1, header, body, icon),
			"bottom_left": quad(2, header, body, icon), "bottom_right": quad(3, header, body, icon),
		}
	}
	add("80+200", "matrix-2x2", false, mx(80, 200, false))
	add("30+160", "matrix-2x2", true, mx(30, 160, false))
	add("30+120-icons", "matrix-2x2", false, mx(30, 120, true))

	// process-grid-2row: 3 and 6 columns at every field's maximum.
	for _, n := range []int{3, 6} {
		v := map[string]any{"row1_label": rowFitWords(40, 0), "row2_label": rowFitWords(40, 1)}
		var r1, r2, hs, os []string
		for i := 0; i < n; i++ {
			r1 = append(r1, rowFitWords(40, i))
			r2 = append(r2, rowFitWords(40, i+2))
			hs = append(hs, rowFitWords(24, i))
			os = append(os, rowFitWords(32, i+1))
		}
		v["row1_phases"], v["row2_phases"] = r1, r2
		add(fmt.Sprintf("%d-bare", n), "process-grid-2row", false, v)
		v["column_headers"], v["outcomes"] = hs, os
		add(fmt.Sprintf("%d-full", n), "process-grid-2row", false, v)
	}

	// process-flow / process-flow-compact at the documented label budgets.
	steps := func(n, label int, kind func(int) string) map[string]any {
		var out []map[string]any
		for i := 0; i < n; i++ {
			out = append(out, map[string]any{"label": rowFitWords(label, i), "type": kind(i)})
		}
		return map[string]any{"steps": out}
	}
	step := func(int) string { return "step" }
	mixed := func(i int) string {
		if i%2 == 1 {
			return "decision"
		}
		return "step"
	}
	chevron := func(int) string { return "chevron" }
	for _, pat := range []string{"process-flow", "process-flow-compact"} {
		for _, c := range []struct{ n, label int }{{3, 80}, {4, 80}, {6, 76}, {7, 52}, {8, 51}} {
			add(fmt.Sprintf("%dx%d", c.n, c.label), pat, false, steps(c.n, c.label, step))
			add(fmt.Sprintf("%dx%d-decisions", c.n, c.label), pat, false, steps(c.n, c.label, mixed))
		}
		for _, c := range []struct{ n, label int }{{4, 80}, {5, 61}, {6, 31}, {7, 12}, {8, 10}} {
			add(fmt.Sprintf("%dx%d-chevrons", c.n, c.label), pat, false, steps(c.n, c.label, chevron))
		}
	}
	return cases
}

// writtenBelowFloor resolves grid at bounds (nested grids too) and returns
// the cells whose written normAutofit shrink takes a run below 12pt.
func writtenBelowFloor(t *testing.T, in *jsonschema.ShapeGridInput, bounds pptx.RectEmu) []string {
	t.Helper()
	if in.Bounds != nil {
		b := in.Bounds
		bounds = pptx.RectEmu{
			X: bounds.X + int64(float64(bounds.CX)*b.X/100), Y: bounds.Y + int64(float64(bounds.CY)*b.Y/100),
			CX: int64(float64(bounds.CX) * b.Width / 100), CY: int64(float64(bounds.CY) * b.Height / 100),
		}
	}
	res := resolveWrittenGrid(t, in, bounds)
	var out []string
	for _, c := range res.Cells {
		if c.Kind == shapegrid.CellKindSubGrid {
			if src := in.Rows[c.RowIdx].Cells[c.ColIdx]; src != nil && src.Grid != nil {
				out = append(out, writtenBelowFloor(t, src.Grid, c.Bounds)...)
			}
			continue
		}
		if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb == nil {
			continue
		}
		smallest := smallestRunPt(tb)
		if smallest == 0 {
			continue
		}
		data, err := shapegrid.GenerateCellShapeXML(c)
		if err != nil {
			t.Fatal(err)
		}
		var shape struct {
			Body struct {
				Props struct {
					Autofit struct {
						Scale int `xml:"fontScale,attr"`
					} `xml:"normAutofit"`
				} `xml:"bodyPr"`
			} `xml:"txBody"`
		}
		if err := xml.Unmarshal(data, &shape); err != nil {
			t.Fatal(err)
		}
		scale := 1.0
		if shape.Body.Props.Autofit.Scale > 0 {
			scale = float64(shape.Body.Props.Autofit.Scale) / 100000
		}
		if smallest >= 12 && smallest*scale < 12-1e-9 {
			out = append(out, fmt.Sprintf("%q %.0fpt×%.0f%%=%.1fpt in %.0f×%.0fpt", firstText(tb), smallest, scale*100, smallest*scale,
				float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700))
		}
	}
	return out
}

// resolveWrittenGrid is resolveGridAt carrying icon overlays, cell height caps
// and shape adjustments, which change the written text box.
func resolveWrittenGrid(t *testing.T, in *jsonschema.ShapeGridInput, bounds pptx.RectEmu) *shapegrid.ResolveResult {
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
			cells[j] = shapegrid.Cell{ColSpan: c.ColSpan, RowSpan: c.RowSpan, MaxHeight: c.MaxHeight, Fit: shapegrid.FitMode(c.Fit), Placeholder: c.Grid != nil}
			if c.Shape != nil {
				cells[j].Shape = &shapegrid.ShapeSpec{Geometry: c.Shape.Geometry, Fill: c.Shape.Fill, Line: c.Shape.Line, Text: c.Shape.Text, Adjustments: c.Shape.Adjustments}
				if ic := c.Shape.Icon; ic != nil {
					cells[j].Icon = &shapegrid.IconSpec{Name: ic.Name, SVGData: ic.SVGData, Scale: ic.Scale, Position: ic.Position}
				}
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
	g := &shapegrid.Grid{Bounds: bounds, Columns: cols, Rows: rows, ColGap: colGap, RowGap: rowGap, VAlign: vAlign}
	if err := shapegrid.Validate(g); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := shapegrid.Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return res
}
