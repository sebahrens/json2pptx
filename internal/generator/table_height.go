package generator

import "github.com/sebahrens/json2pptx/internal/types"

// MeasureTableHeight uses the renderer's complete row plan without a height
// budget, so wrapped cells and authored column widths cannot hide lost rows.
func MeasureTableHeight(table *types.TableSpec, width int64, fontSize int, fontNames ...string) int64 {
	if table == nil {
		return 0
	}
	if fontSize <= 0 {
		fontSize = TableBaseFontSize(table.Style)
	}
	font := defaultFontFamily
	if len(fontNames) > 0 && fontNames[0] != "" {
		font = fontNames[0]
	}
	config := TableRenderConfig{minimumFontSize: table.MinimumFontSize, Style: table.Style, DefaultSize: fontSize, DefaultFont: font}
	resolveTableAppearance(&config)
	applyDefaultTableStyling(table, &config)
	config.compactTypography = fontSize <= engineDefaultRowFontSize
	scaled, _ := columnScaledFontSize(fontSize, len(table.Headers), config.compactTypography)
	config.DefaultSize = max(scaled, table.MinimumFontSize)
	widths, _ := tableSpecColumnWidths(table, width, config.DefaultSize)
	return planTableRowsAt(table, widths, config, config.compactTypography).TotalHeight
}
