package jsonschema

import (
	"maps"
	"reflect"
	"slices"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Clone copies a grid and its mutable payloads without serialization. Internal
// font, geometry, and provenance fields deliberately survive the copy.
func (g *ShapeGridInput) Clone() *ShapeGridInput {
	if g == nil {
		return nil
	}
	out := *g
	out.ColRule = slices.Clone(g.ColRule)
	out.Columns = slices.Clone(g.Columns)
	out.Bounds = clonePointer(g.Bounds)
	out.Rows = slices.Clone(g.Rows)
	for i, row := range g.Rows {
		out.Rows[i].Band = slices.Clone(row.Band)
		out.Rows[i].Connector = clonePointer(row.Connector)
		out.Rows[i].Cells = slices.Clone(row.Cells)
		for j, cell := range row.Cells {
			out.Rows[i].Cells[j] = cloneGridCell(cell)
		}
	}
	out.Links = slices.Clone(g.Links)
	for i := range out.Links {
		out.Links[i].Connector = clonePointer(g.Links[i].Connector)
	}
	return &out
}

func clonePointer[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneGridCell(c *GridCellInput) *GridCellInput {
	if c == nil {
		return nil
	}
	out := *c
	out.Shape = cloneGridShape(c.Shape)
	out.Table = cloneGridTable(c.Table)
	out.Icon = clonePointer(c.Icon)
	out.Image = cloneGridImage(c.Image)
	out.Diagram = cloneGridDiagram(c.Diagram)
	out.Composite = clonePointer(c.Composite)
	if out.Composite != nil {
		out.Composite.Text = cloneGridShape(c.Composite.Text)
		out.Composite.SubDiagram = cloneGridDiagram(c.Composite.SubDiagram)
	}
	out.Pattern = slices.Clone(c.Pattern)
	out.Grid = c.Grid.Clone()
	out.AccentBar = clonePointer(c.AccentBar)
	out.Layers = slices.Clone(c.Layers)
	for i := range out.Layers {
		out.Layers[i].Shape = cloneGridShape(c.Layers[i].Shape)
	}
	return &out
}

func cloneGridShape(s *ShapeSpecInput) *ShapeSpecInput {
	out := clonePointer(s)
	if out == nil {
		return nil
	}
	out.Fill = slices.Clone(s.Fill)
	out.Line = slices.Clone(s.Line)
	out.Text = slices.Clone(s.Text)
	out.Adjustments = maps.Clone(s.Adjustments)
	out.Icon = clonePointer(s.Icon)
	out.Link = clonePointer(s.Link)
	return out
}

func cloneGridImage(s *GridImageInput) *GridImageInput {
	out := clonePointer(s)
	if out != nil {
		out.Overlay = clonePointer(s.Overlay)
		out.Text = clonePointer(s.Text)
	}
	return out
}

func cloneGridTable(t *TableInput) *TableInput {
	out := clonePointer(t)
	if out == nil {
		return nil
	}
	out.Headers = slices.Clone(t.Headers)
	out.ColumnWidths = slices.Clone(t.ColumnWidths)
	out.ColumnAlignments = slices.Clone(t.ColumnAlignments)
	out.Rows = slices.Clone(t.Rows)
	for i, row := range t.Rows {
		out.Rows[i] = slices.Clone(row)
		for j, cell := range row {
			out.Rows[i][j].Conditional = clonePointer(cell.Conditional)
			if cell.Conditional != nil {
				out.Rows[i][j].Conditional.Threshold = slices.Clone(cell.Conditional.Threshold)
			}
		}
	}
	out.Style = clonePointer(t.Style)
	if out.Style != nil {
		out.Style.HeaderBackground = clonePointer(t.Style.HeaderBackground)
		out.Style.Striped = clonePointer(t.Style.Striped)
		out.Style.ColumnTypes = slices.Clone(t.Style.ColumnTypes)
	}
	return out
}

func cloneGridDiagram(d *types.DiagramSpec) *types.DiagramSpec {
	out := clonePointer(d)
	if out == nil {
		return nil
	}
	out.Data = clonePayloadMap(d.Data)
	out.Warnings = slices.Clone(d.Warnings)
	out.ChartDiagnostics = slices.Clone(d.ChartDiagnostics)
	for i := range out.ChartDiagnostics {
		out.ChartDiagnostics[i].Details = clonePayloadMap(d.ChartDiagnostics[i].Details)
	}
	out.ChartStyle = clonePointer(d.ChartStyle)
	if out.ChartStyle != nil {
		out.ChartStyle.ShowVerticalGridlines = clonePointer(d.ChartStyle.ShowVerticalGridlines)
		out.ChartStyle.ShowSingleSeriesLegend = clonePointer(d.ChartStyle.ShowSingleSeriesLegend)
	}
	out.Style = clonePointer(d.Style)
	if out.Style != nil {
		out.Style.Colors = slices.Clone(d.Style.Colors)
		out.Style.ThemeColors = slices.Clone(d.Style.ThemeColors)
		out.Style.SemanticAccents = maps.Clone(d.Style.SemanticAccents)
		out.Style.DataPalette = slices.Clone(d.Style.DataPalette)
		out.Style.ShowValues = clonePointer(d.Style.ShowValues)
		out.Style.ValueFormat = clonePointer(d.Style.ValueFormat)
		if out.Style.ValueFormat != nil {
			out.Style.ValueFormat.Decimals = clonePointer(d.Style.ValueFormat.Decimals)
			out.Style.ValueFormat.ThousandsSep = clonePointer(d.Style.ValueFormat.ThousandsSep)
		}
	}
	return out
}

func clonePayloadMap(m map[string]any) map[string]any {
	out := maps.Clone(m)
	for k, v := range m {
		if v != nil {
			out[k] = clonePayloadValue(reflect.ValueOf(v)).Interface()
		}
	}
	return out
}

// Diagram data is open JSON-shaped content, including typed slices supplied by
// Go callers. Copy its containers without changing numbers or concrete types.
func clonePayloadValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return v
		}
		inner := clonePayloadValue(v.Elem())
		out := reflect.New(v.Type()).Elem()
		if v.Kind() == reflect.Pointer {
			out.Set(reflect.New(v.Type().Elem()))
			out.Elem().Set(inner)
		} else {
			out.Set(inner)
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), clonePayloadValue(iter.Value()))
		}
		return out
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		if v.Kind() == reflect.Slice {
			out = reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		}
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(clonePayloadValue(v.Index(i)))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if out.Field(i).CanSet() && v.Field(i).CanInterface() {
				out.Field(i).Set(clonePayloadValue(v.Field(i)))
			}
		}
		return out
	default:
		return v
	}
}
