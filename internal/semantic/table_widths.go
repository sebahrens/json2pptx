package semantic

import (
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

func tableColumnWidthsSchema() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "number", "exclusiveMinimum": 0}}
}

func validateTableColumnWidths(path string, body map[string]any, columns int, s *semDiags) {
	value, exists := body["column_widths"]
	if !exists {
		return
	}
	raw, ok := value.([]any)
	if !ok || len(raw) != columns || len(raw) == 0 {
		s.hard(path+".column_widths", diagnostics.CodeSemanticFieldType, fmt.Sprintf("column_widths requires one positive number per header (%d)", columns))
		return
	}
	widths := slides.TableColumnWidths(body)
	for i, w := range widths {
		if w <= 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			s.hard(fmt.Sprintf("%s.column_widths[%d]", path, i), diagnostics.CodeSemanticFieldType, "column width must be a finite positive number")
		}
	}
}
