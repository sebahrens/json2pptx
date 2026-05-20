package tokens

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen"
)

// TestChartStyleDefaults_Parity_Gridlines asserts that svggen's chart-grid
// default still matches the executive token contract. If svggen flips
// DefaultGridConfig.ShowVertical back to true, this test catches the drift
// before a deck ever ships with PowerPoint-looking vertical gridlines.
//
// The constant is the canonical source; svggen is required to match.
func TestChartStyleDefaults_Parity_Gridlines(t *testing.T) {
	cfg := svggen.DefaultGridConfig()
	wantVertical := !ChartHideVerticalGridlines
	if cfg.ShowVertical != wantVertical {
		t.Errorf("svggen.DefaultGridConfig().ShowVertical = %t, want %t (per tokens.ChartHideVerticalGridlines = %t)",
			cfg.ShowVertical, wantVertical, ChartHideVerticalGridlines)
	}
	// Horizontal gridlines must stay on — the y-axis carries the comparison.
	if !cfg.ShowHorizontal {
		t.Error("svggen.DefaultGridConfig().ShowHorizontal = false, want true (horizontal gridlines remain on by executive default)")
	}
}

// TestNumericColumnType locks the set of column-type strings that are
// considered numeric for header / cell right-alignment. Drift here would mean
// a "delta" column suddenly stops right-aligning, or a "date" column starts.
func TestNumericColumnType(t *testing.T) {
	numeric := []string{"number", "currency", "percent", "delta"}
	for _, c := range numeric {
		if !NumericColumnType(c) {
			t.Errorf("NumericColumnType(%q) = false, want true", c)
		}
	}
	nonNumeric := []string{"", "text", "date", "string", "Number" /* case-sensitive */}
	for _, c := range nonNumeric {
		if NumericColumnType(c) {
			t.Errorf("NumericColumnType(%q) = true, want false", c)
		}
	}
}

// TestChartStyleDefaults_Parity_TabularNums asserts that svggen emits the
// tabular-nums CSS rule on every chart SVG when ChartTickLabelTabularNums is
// true. Without this, executive decks would render columns of numbers with
// proportional digits and the tens-column drift would be visible to readers.
func TestChartStyleDefaults_Parity_TabularNums(t *testing.T) {
	if !ChartTickLabelTabularNums {
		t.Skip("ChartTickLabelTabularNums is false; renderer is not expected to emit the rule")
	}
	req := &svggen.RequestEnvelope{
		Type:  "bar_chart",
		Title: "parity",
		Data: map[string]any{
			"categories": []string{"A", "B"},
			"series": []map[string]any{
				{"name": "S1", "values": []float64{1, 2}},
			},
		},
		Output: svggen.OutputSpec{Format: "svg", Width: 400, Height: 300},
	}
	out, err := svggen.RenderMultiFormatWithFindings(req, "svg")
	if err != nil {
		t.Fatalf("render svg: %v", err)
	}
	if out == nil || out.SVG == nil {
		t.Fatal("svggen returned no SVG document")
	}
	got := string(out.SVG.Content)
	const want = "font-variant-numeric:tabular-nums"
	if !strings.Contains(got, want) {
		t.Errorf("chart SVG missing %q rule (per tokens.ChartTickLabelTabularNums)", want)
	}
}

// TestChartStyleDefaults_Sanity guards the contract values themselves so a
// future "drop legend when series >= 3" or "direct-label up to 8 series"
// change has to update both the constant and this test (and, by the
// commit-discipline policy, RULES.md if it surfaces these to agents).
func TestChartStyleDefaults_Sanity(t *testing.T) {
	if !ChartHideAreaBorder {
		t.Error("ChartHideAreaBorder = false; executive defaults drop the outer chart-area rectangle")
	}
	if !ChartHideVerticalGridlines {
		t.Error("ChartHideVerticalGridlines = false; executive defaults drop vertical gridlines")
	}
	if ChartLegendMinSeries < 2 {
		t.Errorf("ChartLegendMinSeries = %d; single-series charts must not show a legend by default", ChartLegendMinSeries)
	}
	if ChartDirectLabelMaxSeries < ChartLegendMinSeries {
		t.Errorf("ChartDirectLabelMaxSeries (%d) must be ≥ ChartLegendMinSeries (%d); otherwise the direct-label window is empty",
			ChartDirectLabelMaxSeries, ChartLegendMinSeries)
	}
	if !ChartTickLabelTabularNums {
		t.Error("ChartTickLabelTabularNums = false; numeric tick labels should request tabular figures")
	}
	if !TableNumericHeaderAlignRight {
		t.Error("TableNumericHeaderAlignRight = false; numeric headers should inherit the right-alignment of their data column")
	}
}
