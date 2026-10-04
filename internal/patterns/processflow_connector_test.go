package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-66ojb: the connector between two steps was as long as the
// 12pt grid gap with a 4pt arrowhead, so a flow read as a row of buttons.

func pfSteps(n int) []ProcessFlowStep {
	labels := []string{"Request", "Review", "Approve", "Build", "Test", "Deploy", "Monitor", "Close"}
	steps := make([]ProcessFlowStep, n)
	for i := range steps {
		steps[i] = ProcessFlowStep{Label: labels[i]}
	}
	return steps
}

func TestProcessFlowConnectorHasItsOwnLength(t *testing.T) {
	wide := ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000,
		Metadata: &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: 16}}}
	for _, name := range []string{"process-flow", "process-flow-compact"} {
		p, _ := Default().Get(name)
		for n := 3; n <= 8; n++ {
			vals := &ProcessFlowValues{Steps: pfSteps(n)}
			grid, err := p.Expand(testThemeCtx(), vals, nil, nil)
			if err != nil {
				t.Fatalf("%s n=%d: %v", name, n, err)
			}
			if grid.Gap < processFlowConnectorMinPt {
				t.Errorf("%s n=%d: step gap %.0fpt is shorter than the %.0fpt connector minimum", name, n, grid.Gap, processFlowConnectorMinPt)
			}
			// The connector's length follows the boxes on a row: process-flow
			// puts seven or eight steps on two rows.
			perRow := n
			if name == "process-flow" && n >= processFlowTwoRowMinSteps {
				perRow = (n + 1) / 2
			}
			if grid.Gap != processFlowConnectorLenPt(perRow) {
				t.Errorf("%s n=%d: step gap %.0fpt, want the %.0fpt connector length", name, n, grid.Gap, processFlowConnectorLenPt(perRow))
			}
			conn := grid.Rows[0].Connector
			if conn == nil || conn.Style != "arrow" || conn.Head != "lg" || conn.Width != processFlowConnectorLinePt {
				t.Errorf("%s n=%d: connector = %+v, want a %.0fpt arrow with the large head", name, n, conn, processFlowConnectorLinePt)
			}
			// The length does not follow the template gutter.
			scaled, err := p.Expand(wide, vals, nil, nil)
			if err != nil {
				t.Fatalf("%s n=%d: %v", name, n, err)
			}
			if scaled.Gap != processFlowConnectorLenPt(perRow) {
				t.Errorf("%s n=%d: a 16pt template gutter changed the connector length to %.0fpt", name, n, scaled.Gap)
			}
		}
	}

	// A row of chevrons draws its own direction: no connector, plain gap.
	p, _ := Default().Get("process-flow")
	steps := pfSteps(5)
	for i := range steps {
		steps[i].Type = "chevron"
	}
	grid, err := p.Expand(testThemeCtx(), &ProcessFlowValues{Steps: steps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Rows[0].Connector != nil || grid.Gap != processFlowGapPt {
		t.Errorf("a chevron row has connector %+v and gap %.0fpt, want none and %.0fpt", grid.Rows[0].Connector, grid.Gap, processFlowGapPt)
	}
}

// A diamond's text rectangle is half its width: a decision whose word needs
// it takes a wider column, and a word that still cannot fit is reported.
func TestProcessFlowDecisionKeepsItsWordWhole(t *testing.T) {
	ctx := testThemeCtx()
	for _, name := range []string{"process-flow", "process-flow-compact"} {
		p, _ := Default().Get(name)
		// One row of eight: process-flow bends eight steps onto two rows unless
		// told otherwise (TestProcessFlowTwoRows covers the bent layout).
		var ovr any
		if name == "process-flow" {
			ovr = &ProcessFlowOverrides{Rows: 1}
		}
		steps := pfSteps(8)
		steps[3] = ProcessFlowStep{Label: "Approved?", Type: "decision"}
		grid, err := p.Expand(ctx, &ProcessFlowValues{Steps: steps}, ovr, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var cols []float64
		if err := json.Unmarshal(grid.Columns, &cols); err != nil || len(cols) != 8 {
			t.Fatalf("%s: columns = %s, want eight widths with a wider decision", name, grid.Columns)
		}
		if cols[3] <= cols[0] {
			t.Errorf("%s: decision column %.2f%% is not wider than a step's %.2f%%", name, cols[3], cols[0])
		}
		if cols[0] < 100.0/8*(1-processFlowDecisionMaxGiveFrac)-3 {
			t.Errorf("%s: steps gave up more than a quarter of their width: %.2f%%", name, cols[0])
		}
		if w := p.(PostExpandWarner).PostExpandWarnings(ctx, &ProcessFlowValues{Steps: steps}, ovr); len(w) != 0 {
			t.Errorf("%s: a decision that fits after widening was reported: %v", name, w)
		}

		// No decision needs widening: the columns stay the plain count.
		plain, err := p.Expand(ctx, &ProcessFlowValues{Steps: pfSteps(8)}, ovr, nil)
		if err != nil {
			t.Fatal(err)
		}
		if string(plain.Columns) != "8" {
			t.Errorf("%s: columns = %s, want 8", name, plain.Columns)
		}

		steps[3].Label = "Counterintuitively?"
		warnings := p.(PostExpandWarner).PostExpandWarnings(ctx, &ProcessFlowValues{Steps: steps}, ovr)
		if len(warnings) != 1 || !strings.HasPrefix(warnings[0], ErrCodeTextExceedsShape+": ") || !strings.Contains(warnings[0], "steps[3].label") {
			t.Errorf("%s: a word no diamond can hold: warnings = %v", name, warnings)
		}
	}
}
