package main

import "github.com/sebahrens/json2pptx/internal/types"

// tableGridErrorField mirrors CheckRowWidths' check order so callers attribute
// a malformed grid to the authored widths or rows without parsing its prose.
func tableGridErrorField(table *types.TableSpec) string {
	if table.ColumnWidths != nil && types.ResolveTableColumnWidths(table.ColumnWidths, len(table.Headers), 1000000) == nil {
		return "column_widths"
	}
	return "rows"
}
