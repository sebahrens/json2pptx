package patterns

import (
	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// Two-row process flow (go-slide-creator-pfyeg).
//
// Seven or eight steps on one row leave each box about 90pt wide: a label of
// the documented budget (71–72 characters) wrapped to seven or eight lines of
// one or two words (TEXT_WRAPS_NARROW), and a label cut to five words left
// eight slivers under a third of the slide. No rewrite of the text made the
// slide clean. From processFlowTwoRowMinSteps steps the flow now bends: the
// first half runs left to right, a connector drops from its last step, and
// the second half runs back right to left underneath — each box as wide as
// in a four-step flow, the arrows still saying which step follows which.
// Chevrons and arrows point right by construction, so a row of them wraps
// like a line of text instead (second row left to right, no connectors).

// processFlowTwoRowMinSteps is the step count from which process-flow bends
// onto two rows by default.
const processFlowTwoRowMinSteps = 7

// processFlowTwoRowFloor is the smallest step count overrides.rows 2 applies
// to: below it a second row would hold a single step.
const processFlowTwoRowFloor = 4

// processFlowLayout places the steps of a flow on its grid.
type processFlowLayout struct {
	rows int
	// perRow is the column count: every step on one row, or the larger half.
	perRow int
	// snake runs the second row right to left under the first, joined by a
	// connector that drops from the first row's last step.
	snake bool
}

// processFlowLayoutFor is the layout of a process-flow: overrides.rows when
// set, else two rows from processFlowTwoRowMinSteps steps. A flow that mixes
// pointed and plain steps keeps one row unless rows is set — its chevrons
// cannot point back along a returning row.
func processFlowLayoutFor(steps []ProcessFlowStep, ovr *ProcessFlowOverrides) processFlowLayout {
	n := len(steps)
	one := processFlowLayout{rows: 1, perRow: n}
	rows := 0
	if ovr != nil {
		rows = ovr.Rows
	}
	pointed := anyStepPointed(steps)
	switch {
	case rows == 1, n < processFlowTwoRowFloor:
		return one
	case rows == 0 && (n < processFlowTwoRowMinSteps || (pointed && !allStepsPointed(steps))):
		return one
	}
	return processFlowLayout{rows: 2, perRow: (n + 1) / 2, snake: !pointed}
}

// cell is the grid row and column of step i.
func (l processFlowLayout) cell(i int) (row, col int) {
	if l.rows < 2 || i < l.perRow {
		return 0, i
	}
	j := i - l.perRow
	if l.snake {
		return 1, l.perRow - 1 - j
	}
	return 1, j
}

// anyStepPointed reports whether any step is a chevron or a right arrow.
func anyStepPointed(steps []ProcessFlowStep) bool {
	for _, s := range steps {
		if s.Type == "chevron" || s.Type == "arrow" {
			return true
		}
	}
	return false
}

// processFlowGridRows arranges the step cells (in step order) on the layout's
// rows and returns the links a bent flow needs: the drop from the first row's
// last step and the right-to-left arrows of the returning row, which a row
// connector (always left to right) cannot draw.
func processFlowGridRows(cells []*jsonschema.GridCellInput, lay processFlowLayout, connector *jsonschema.ConnectorSpecInput, rowHeight float64) ([]jsonschema.GridRowInput, []jsonschema.GridLinkInput) {
	if lay.rows < 2 {
		return []jsonschema.GridRowInput{{Cells: cells, Connector: connector, MaxHeight: rowHeight}}, nil
	}
	first := jsonschema.GridRowInput{Cells: cells[:lay.perRow], Connector: connector, MaxHeight: rowHeight}
	slots := make([]*jsonschema.GridCellInput, lay.perRow)
	for c := range slots {
		slots[c] = &jsonschema.GridCellInput{} // spacer: keeps the column
	}
	for i := lay.perRow; i < len(cells); i++ {
		_, c := lay.cell(i)
		slots[c] = cells[i]
	}
	second := jsonschema.GridRowInput{Cells: slots, MaxHeight: rowHeight}
	if !lay.snake {
		second.Connector = connector
		return []jsonschema.GridRowInput{first, second}, nil
	}
	var links []jsonschema.GridLinkInput
	if connector != nil {
		for i := lay.perRow - 1; i+1 < len(cells); i++ {
			fromRow, fromCol := lay.cell(i)
			toRow, toCol := lay.cell(i + 1)
			spec := *connector
			links = append(links, jsonschema.GridLinkInput{
				From:      [2]int{fromRow, fromCol},
				To:        [2]int{toRow, toCol},
				Connector: &spec,
			})
		}
	}
	return []jsonschema.GridRowInput{first, second}, links
}
