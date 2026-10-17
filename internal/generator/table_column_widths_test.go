package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func glossaryWidthFixture() *jsonschema.TableInput {
	table := &jsonschema.TableInput{Headers: []string{"Term", "Meaning"}, ColumnWidths: []float64{1, 3}}
	meaning := "Named owner reviews, resolves, and records the final decision."
	for i := 0; i < 9; i++ {
		table.Rows = append(table.Rows, []jsonschema.TableCellInput{{Content: fmt.Sprintf("Accountable owner %02d", i)}, {Content: meaning}})
	}
	return table
}

func TestGlossaryColumnWidthsRenderNineRows(t *testing.T) {
	table := glossaryWidthFixture()
	const width int64 = 11000000
	if count := table.LogicalRowCountAtWidth(width); count != 10 {
		t.Fatalf("logical rows = %d, want 10", count)
	}
	if count := table.LogicalRowCountAtWidth(width / 3); count <= 10 {
		t.Fatalf("narrow frame incorrectly fits: %d", count)
	}
	spec := table.ToTableSpec()
	bounds := types.BoundingBox{Width: width, Height: 4500000}
	result, err := GenerateTableXML(spec, TableRenderConfig{Bounds: bounds, DefaultSize: 1200})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.XML, `<a:gridCol w="2750000"`) || !strings.Contains(result.XML, `<a:gridCol w="8250000"`) {
		t.Fatalf("rendered columns lost 1:3 widths")
	}
	for _, f := range result.Findings {
		if f.Code == patterns.ErrCodeTableRowsTruncated {
			t.Fatalf("glossary truncated: %s", f.Message)
		}
	}
	for _, f := range DetectTablePreflight(TablePreflightInput{Headers: spec.Headers, Rows: spec.Rows, ColumnWidths: spec.ColumnWidths, Bounds: bounds, DefaultSize: 1200}) {
		if f.Code == patterns.ErrCodeTableRowsTruncated {
			t.Fatalf("preflight truncates glossary: %s", f.Message)
		}
	}
	if !strings.Contains(result.XML, "Accountable owner 08") {
		t.Fatal("last glossary row missing")
	}
}

func TestTableColumnWidthsRejectInvalidRendererValues(t *testing.T) {
	for _, widths := range [][]float64{{}, {1}, {1, 0}, {1, -2}, {1, 2, 3}} {
		spec := glossaryWidthFixture().ToTableSpec()
		spec.ColumnWidths = widths
		if _, err := GenerateTableXML(spec, TableRenderConfig{Bounds: types.BoundingBox{Width: 11000000, Height: 4500000}}); err == nil {
			t.Fatalf("accepted invalid widths %v", widths)
		}
	}
}
