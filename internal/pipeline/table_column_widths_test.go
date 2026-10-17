package pipeline

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func TestTableDensityUsesAuthoredColumnWidthsAndFrame(t *testing.T) {
	table := &jsonschema.TableInput{Headers: []string{"Term", "Meaning"}, ColumnWidths: []float64{1, 3}}
	for i := 0; i < 9; i++ {
		table.Rows = append(table.Rows, []jsonschema.TableCellInput{{Content: "Accountable owner 00"}, {Content: "Named owner reviews, resolves, and records the final decision."}})
	}
	if warnings := DetectTableDensity(table, "table", 11000000); len(warnings) != 0 {
		t.Fatalf("readable glossary refused: %v", warnings)
	}
	if warnings := DetectTableDensity(table, "table", 2500000); len(warnings) == 0 {
		t.Fatal("wrapped glossary bypassed density at narrow frame")
	}
	table.Rows[0][0].Content = "one\ntwo\nthree"
	if warnings := DetectTableDensity(table, "table", 11000000); len(warnings) == 0 {
		t.Fatal("explicit newlines bypassed density")
	}
}
