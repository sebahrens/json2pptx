package main

import "errors"
import "fmt"
import "github.com/sebahrens/json2pptx/internal/types"

// tableGridErrorField mirrors CheckRowWidths' check order so callers attribute
// a malformed grid to the authored widths or rows without parsing its prose.
func tableGridErrorField(table *types.TableSpec) string {
	if table.ColumnWidths != nil && types.ResolveTableColumnWidths(table.ColumnWidths, len(table.Headers), 1000000) == nil {
		return "column_widths"
	}
	for _, row := range table.Rows {
		if types.RowGridWidth(row) > len(table.Headers) {
			return "rows"
		}
	}
	var ink *types.InvalidTableTextColor
	if errors.As(table.CheckTextColors(), &ink) {
		return fmt.Sprintf("rows/%d/%d/text_color", ink.Row, ink.Col)
	}
	return "rows"
}
