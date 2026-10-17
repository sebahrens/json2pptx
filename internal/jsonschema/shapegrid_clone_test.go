package jsonschema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestShapeGridClonePreservesProvenanceAndOwnsPayloads(t *testing.T) {
	var source ShapeGridInput
	raw := `{"columns":[40,60],"col_rule":{"color":"accent1"},"bounds":{"x":1,"y":2,"width":90,"height":80},"links":[{"from":[0,0],"to":[0,1],"connector":{"color":"accent1"}}],"rows":[{"band":{"color":"lt2"},"connector":{"color":"accent2"},"cells":[{"shape":{"geometry":"rect","fill":"lt1","line":{"color":"accent1"},"text":"Original","adjustments":{"adj":5000},"icon":{"name":"check"},"link":{"url":"https://example.test"}},"layers":[{"frame":{"w":1,"h":1},"shape":{"geometry":"rect","text":"Layer"}}]},{"grid":{"columns":1,"rows":[{"cells":[{"pattern":{"name":"stat-hero"}}]}]}}]},{"cells":[{"table":{"headers":["Amount"],"rows":[[{"content":"1","conditional":{"rule":"above","threshold":0}}]],"style":{"striped":true,"header_background":"lt2","column_types":["delta"]},"column_widths":[1],"column_alignments":["right"]}},{"image":{"path":"photo.png","overlay":{"color":"dk1"},"text":{"content":"Photo"}}}]}]}`
	if err := json.Unmarshal([]byte(raw), &source); err != nil {
		t.Fatal(err)
	}
	source.BoundsRelativeToContentArea = true
	source.KeepTextSizes = true
	source.VerifiedMatrixPattern = "comparison"
	source.Rows[0].Cells[0].Shape.MeasureFonts = MeasureFonts{Major: "Lora", Minor: "Arial"}
	source.Rows[1].Cells[0].Table.MinimumFontSize = 1000
	source.Rows[1].Cells[0].Table.Style.StyleIDFromDefaults = true
	number := 7
	flag := true
	diagram := &types.DiagramSpec{TitleOnSlide: true, Data: map[string]any{"series": []map[string]any{{"values": []float64{1, 2}}}, "ptr": &number}, Warnings: []string{"original"}, ChartDiagnostics: []types.ChartDiagnostic{{Details: map[string]any{"source": []string{"original"}}}}, Style: &types.DiagramStyle{SemanticAccents: map[string]string{"positive": "accent3"}, Colors: []string{"blue"}, ThemeColors: []types.ThemeColor{{Name: "accent1", RGB: "0000FF"}}, DataPalette: []string{"0000FF"}, ShowValues: &flag, ValueFormat: &types.ValueFormatSpec{Decimals: &number, ThousandsSep: &flag}}, ChartStyle: &types.ChartStyleOverrides{ShowVerticalGridlines: &flag}}
	source.Rows = append(source.Rows, GridRowInput{Cells: []*GridCellInput{{Diagram: diagram}, {Composite: &CompositeInput{Text: &ShapeSpecInput{Text: json.RawMessage(`"KPI"`)}, SubDiagram: diagram}}, nil}})
	clone := source.Clone()
	if !reflect.DeepEqual(&source, clone) {
		t.Fatal("clone lost fields, nil cells, or concrete payload types")
	}
	before, _ := json.Marshal(&source)
	clone.Bounds.X = 99
	clone.Columns[1] = '9'
	clone.ColRule[2] = 'x'
	clone.Links[0].Connector.Color = "changed"
	clone.Rows[0].Band[2] = 'x'
	clone.Rows[0].Connector.Color = "changed"
	shape := clone.Rows[0].Cells[0].Shape
	shape.Text[1] = 'x'
	shape.Fill[1] = 'x'
	shape.Line[2] = 'x'
	shape.Adjustments["adj"] = 9
	shape.Icon.Name = "changed"
	shape.Link.URL = "changed"
	shape.MeasureFonts.Minor = "Changed"
	clone.Rows[0].Cells[0].Layers[0].Shape.Text[1] = 'x'
	clone.Rows[0].Cells[1].Grid.Rows[0].Cells[0].Pattern[2] = 'x'
	table := clone.Rows[1].Cells[0].Table
	table.Headers[0] = "changed"
	table.Rows[0][0].Content = "changed"
	table.Rows[0][0].Conditional.Threshold[0] = '9'
	*table.Style.Striped = false
	*table.Style.HeaderBackground = "changed"
	table.Style.ColumnTypes[0] = "changed"
	table.ColumnWidths[0] = 9
	table.ColumnAlignments[0] = "changed"
	table.MinimumFontSize = 9
	table.Style.StyleIDFromDefaults = false
	clone.Rows[1].Cells[1].Image.Overlay.Color = "changed"
	clone.Rows[1].Cells[1].Image.Text.Content = "changed"
	d := clone.Rows[2].Cells[0].Diagram
	d.Data["series"].([]map[string]any)[0]["values"].([]float64)[0] = 9
	*d.Data["ptr"].(*int) = 9
	d.Warnings[0] = "changed"
	d.ChartDiagnostics[0].Details["source"].([]string)[0] = "changed"
	d.Style.SemanticAccents["positive"] = "changed"
	d.Style.Colors[0] = "changed"
	d.Style.ThemeColors[0].RGB = "changed"
	d.Style.DataPalette[0] = "changed"
	*d.Style.ShowValues = false
	*d.Style.ValueFormat.Decimals = 9
	*d.Style.ValueFormat.ThousandsSep = false
	*d.ChartStyle.ShowVerticalGridlines = false
	d.TitleOnSlide = false
	clone.Rows[2].Cells[1].Composite.Text.Text[1] = 'x'
	clone.Rows[2].Cells[1].Composite.SubDiagram.Data["series"].([]map[string]any)[0]["values"].([]float64)[0] = 8
	after, _ := json.Marshal(&source)
	if string(before) != string(after) || source.Rows[0].Cells[0].Shape.MeasureFonts.Minor != "Arial" || source.Rows[1].Cells[0].Table.MinimumFontSize != 1000 || !source.Rows[1].Cells[0].Table.Style.StyleIDFromDefaults || !diagram.TitleOnSlide || diagram.Style.SemanticAccents["positive"] != "accent3" || diagram.Style.ThemeColors[0].RGB != "0000FF" {
		t.Fatal("mutating cloned output changed caller-owned source")
	}
}

func TestShapeGridCloneKeepsMalformedRawForValidation(t *testing.T) {
	source := &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Pattern: json.RawMessage(`{"name":`)}}}}}
	clone := source.Clone()
	if string(clone.Rows[0].Cells[0].Pattern) != `{"name":` {
		t.Fatal("clone silently erased malformed input")
	}
	if (*ShapeGridInput)(nil).Clone() != nil {
		t.Fatal("nil clone became present")
	}
}
