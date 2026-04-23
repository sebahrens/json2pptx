package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TableCellOverflowWarning describes a single table cell whose measured text
// height exceeds the declared row height.
type TableCellOverflowWarning struct {
	SlideIndex  int   // 0-based slide index
	Row         int   // 0-based row index (0 = header)
	Col         int   // 0-based column index
	DeclaredEMU int64 // Row height in EMU
	MeasuredEMU int64 // Required text height in EMU
}

// String returns a human-readable warning message.
func (w TableCellOverflowWarning) String() string {
	return fmt.Sprintf("slide %d, table row %d col %d: measured text height (%d EMU) exceeds row height (%d EMU)",
		w.SlideIndex+1, w.Row, w.Col, w.MeasuredEMU, w.DeclaredEMU)
}

// WarnTableCellOverflow measures each cell in the table and returns warnings
// for cells whose rendered text height exceeds the declared row height by more
// than 10% (slack for font-metric platform variance).
//
// slideIdx is the 0-based slide index used in warning messages. The table is
// assumed to use the default row height and font scaling logic from GenerateTableXML.
func WarnTableCellOverflow(table *types.TableSpec, slideIdx int) []TableCellOverflowWarning {
	if table == nil || len(table.Headers) == 0 {
		return nil
	}

	numCols := len(table.Headers)
	fontSize := defaultFontSize

	// Apply the same column-count font scaling as GenerateTableXML.
	if numCols > 4 {
		scale := 4.0 / float64(numCols)
		scaled := int(float64(fontSize) * scale)
		if scaled < minFontSizeForTable {
			scaled = minFontSizeForTable
		}
		fontSize = scaled
	}
	fontPt := float64(fontSize) / 100.0

	// Use default slide width (~90% for body placeholder) split evenly.
	const slideWidthEMU int64 = 8229600
	tableWidthEMU := int64(float64(slideWidthEMU) * 0.9)
	colWidthEMU := tableWidthEMU / int64(numCols)

	// 10% slack threshold for platform font-metric variance.
	rowHeight := int64(defaultRowHeight)
	slackThresholdEMU := rowHeight + rowHeight/10

	var warnings []TableCellOverflowWarning

	// Measure header cells (row 0).
	for ci, header := range table.Headers {
		if header == "" {
			continue
		}
		m := textfit.MeasureRun(header, defaultFontFamily, fontPt, colWidthEMU, 0)
		if m.RequiredEMU > slackThresholdEMU {
			warnings = append(warnings, TableCellOverflowWarning{
				SlideIndex:  slideIdx,
				Row:         0,
				Col:         ci,
				DeclaredEMU: defaultRowHeight,
				MeasuredEMU: m.RequiredEMU,
			})
		}
	}

	// Measure data cells (rows 1+).
	for ri, row := range table.Rows {
		for ci, cell := range row {
			if cell.Content == "" || cell.IsMerged {
				continue
			}
			m := textfit.MeasureRun(cell.Content, defaultFontFamily, fontPt, colWidthEMU, 0)
			if m.RequiredEMU > slackThresholdEMU {
				warnings = append(warnings, TableCellOverflowWarning{
					SlideIndex:  slideIdx,
					Row:         ri + 1, // +1 because row 0 is header
					Col:         ci,
					DeclaredEMU: defaultRowHeight,
					MeasuredEMU: m.RequiredEMU,
				})
			}
		}
	}

	return warnings
}
