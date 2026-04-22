package shapegrid

import (
	"errors"
	"fmt"
)

// Validate checks a Grid for structural errors. It returns a multi-error
// (via errors.Join) containing all problems found, or nil if the grid is valid.
func Validate(grid *Grid) error { //nolint:gocognit,gocyclo
	if grid == nil {
		return nil
	}

	var errs []error

	if len(grid.Columns) == 0 {
		errs = append(errs, fmt.Errorf("shape_grid: empty columns; set \"columns\" to a number (e.g. 3) or an array of percentages (e.g. [30, 40, 30])"))
	}

	numCols := len(grid.Columns)
	numRows := len(grid.Rows)

	if numRows == 0 {
		errs = append(errs, fmt.Errorf("shape_grid: no rows defined; add at least one entry to the \"rows\" array, each containing a \"cells\" array"))
	}

	// Track occupied cells for overlap detection
	occupied := make([][]bool, numRows)
	for r := range occupied {
		occupied[r] = make([]bool, numCols)
	}

	for r, row := range grid.Rows {
		col := 0
		for ci, cell := range row.Cells {
			// Skip occupied cells
			for col < numCols && occupied[r][col] {
				col++
			}

			if cell.Shape != nil && cell.TableSpec != nil {
				errs = append(errs, fmt.Errorf("row %d col %d: cell has both shape and table (only one allowed); remove either the \"shape\" or \"table\" key from this cell", r, ci))
			}

			if cell.Fit != "" && cell.Fit != FitContain && cell.Fit != FitWidth && cell.Fit != FitHeight {
				errs = append(errs, fmt.Errorf("row %d col %d: invalid fit mode %q; valid values are \"contain\", \"fit-width\", or \"fit-height\" (omit for default stretch behavior)", r, ci, cell.Fit))
			}

			if cell.Shape == nil && cell.TableSpec == nil && cell.DiagramSpec == nil && cell.Icon == nil && cell.Image == nil {
				col++
				continue
			}

			colSpan := cell.ColSpan
			if colSpan < 1 {
				colSpan = 1
			}
			rowSpan := cell.RowSpan
			if rowSpan < 1 {
				rowSpan = 1
			}

			// Check col_span exceeds grid
			if col+colSpan > numCols {
				errs = append(errs, fmt.Errorf("row %d col %d: col_span %d exceeds grid width %d; reduce col_span to at most %d, or add more columns", r, ci, colSpan, numCols, numCols-col))
			}

			// Check row_span exceeds grid
			if r+rowSpan > numRows {
				errs = append(errs, fmt.Errorf("row %d col %d: row_span %d exceeds grid height %d; reduce row_span to at most %d, or add more rows", r, ci, rowSpan, numRows, numRows-r))
			}

			// Mark cells as occupied, detecting overlaps
			for dr := 0; dr < rowSpan && r+dr < numRows; dr++ {
				for dc := 0; dc < colSpan && col+dc < numCols; dc++ {
					if occupied[r+dr][col+dc] {
						errs = append(errs, fmt.Errorf("row %d col %d: cell overlap at row %d col %d; another cell's col_span or row_span already covers this position — reduce spans or rearrange cells", r, ci, r+dr, col+dc))
					}
					occupied[r+dr][col+dc] = true
				}
			}

			col += colSpan
		}
	}

	return errors.Join(errs...)
}
