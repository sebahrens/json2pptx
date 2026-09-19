package patterns

import (
	"encoding/json"
	"math"
	"testing"
)

// execSummaryCellText decodes a row cell's text object.
func execSummaryCellText(t *testing.T, raw json.RawMessage) chartInsightsText {
	t.Helper()
	var obj chartInsightsText
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("decode cell text %s: %v", raw, err)
	}
	return obj
}

// predictedBaselinePt is where a cell's first baseline falls: the top inset
// plus the ascent of its first paragraph. It is the quantity the alignment is
// about, so the test asserts on it rather than on the anchor alone.
func predictedBaselinePt(text chartInsightsText) float64 {
	if len(text.Paragraphs) == 0 {
		return text.InsetTop
	}
	return text.InsetTop + text.Paragraphs[0].Size*execSummaryAscentRatio
}

// The lead and its support were both centre-anchored, so whenever a lead
// wrapped to two lines the support floated between them — across five rows the
// right column visibly stair-stepped (go-slide-creator-kol0).
func TestExecSummaryRowsShareAFirstBaseline(t *testing.T) {
	for _, numbered := range []bool{true, false} {
		name := "numbered"
		if !numbered {
			name = "unnumbered"
		}
		t.Run(name, func(t *testing.T) {
			vals := &ExecSummaryValues{Points: []ExecSummaryPoint{
				{Lead: "Margins held", Support: "Gross margin was 71% against a 69% plan."},
				{Lead: "Three options were tested on returns, risk and fit", Support: "Option B wins on all three."},
				{Lead: "The programme is behind plan", Support: "Two of six workstreams are amber."},
			}}
			ovr := &ExecSummaryOverrides{}
			if !numbered {
				no := false
				ovr.Numbered = &no
			}

			es := &execSummary{}
			grid, err := es.Expand(ExpandContext{}, vals, ovr, nil)
			if err != nil {
				t.Fatalf("Expand: %v", err)
			}

			rows := 0
			for _, row := range grid.Rows {
				// Point rows carry one cell per column; the thin rules between
				// them carry a single spanning cell.
				if len(row.Cells) < 2 {
					continue
				}
				rows++
				var baselines []float64
				for ci, cell := range row.Cells {
					if cell == nil || cell.Shape == nil || len(cell.Shape.Text) == 0 {
						continue
					}
					text := execSummaryCellText(t, cell.Shape.Text)
					if text.VerticalAlign != "t" {
						t.Errorf("row %d cell %d is %q-anchored; a centred cell floats when its neighbour wraps",
							rows, ci, text.VerticalAlign)
					}
					baselines = append(baselines, predictedBaselinePt(text))
				}
				for i := 1; i < len(baselines); i++ {
					if math.Abs(baselines[i]-baselines[0]) > 0.01 {
						t.Errorf("row %d: first baselines %v differ; every cell in a row must start on the same line",
							rows, baselines)
						break
					}
				}
			}
			if rows == 0 {
				t.Fatal("no point rows found")
			}
		})
	}
}

// The inset is the difference in ascent, and only the smaller text moves.
func TestBaselineInsetPt(t *testing.T) {
	cases := []struct {
		tallest, pt, want float64
	}{
		{22, 22, 0},   // the tallest text sets the line; it never moves
		{22, 17, 4},   // (22-17) * 0.8
		{22, 14, 6.4}, // (22-14) * 0.8
		{17, 22, 0},   // never negative: a cell is not lifted above the line
		{14, 14, 0},
	}
	for _, c := range cases {
		if got := baselineInsetPt(c.tallest, c.pt); math.Abs(got-c.want) > 0.001 {
			t.Errorf("baselineInsetPt(%g, %g) = %g, want %g", c.tallest, c.pt, got, c.want)
		}
	}
}
