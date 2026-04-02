package svggen

import (
	"math"
	"testing"
)

// =============================================================================
// ComputeCartesianLayout Tests
// =============================================================================

// helper to build a default style and chart config for 800x600.
func defaultTestLayoutInputs() (ChartConfig, *StyleGuide) {
	config := DefaultChartConfig(800, 600)
	style := DefaultStyleGuide()
	return config, style
}

func TestComputeCartesianLayout_Basic(t *testing.T) {
	config, style := defaultTestLayoutInputs()

	layout := ComputeCartesianLayout(config, style, "Title", "Subtitle", "Footnote", 3)

	// headerHeight = SizeTitle(18) + Spacing.MD(8) + SizeSubtitle(15) + Spacing.XS(4) = 45
	if math.Abs(layout.HeaderHeight-45) > 0.001 {
		t.Errorf("HeaderHeight = %v, want 45", layout.HeaderHeight)
	}

	// footerHeight = FootnoteReservedHeight = SizeCaption(10) + Padding(3)*2 = 16
	if math.Abs(layout.FooterHeight-16) > 0.001 {
		t.Errorf("FooterHeight = %v, want 16", layout.FooterHeight)
	}

	// legendHeight = SizeSmall(10) + Spacing.LG(12) = 22
	if math.Abs(layout.LegendHeight-22) > 0.001 {
		t.Errorf("LegendHeight = %v, want 22", layout.LegendHeight)
	}

	// PlotArea should be adjusted from the base PlotArea.
	// Base: X=60, Y=40, W=720, H=500
	// Adjusted: Y += 45 => 85, H -= (45+16+22) => 417  (legend bottom)
	if math.Abs(layout.PlotArea.X-60) > 0.001 {
		t.Errorf("PlotArea.X = %v, want 60", layout.PlotArea.X)
	}
	if math.Abs(layout.PlotArea.Y-85) > 0.001 {
		t.Errorf("PlotArea.Y = %v, want 85", layout.PlotArea.Y)
	}
	if math.Abs(layout.PlotArea.W-720) > 0.001 {
		t.Errorf("PlotArea.W = %v, want 720", layout.PlotArea.W)
	}
	if math.Abs(layout.PlotArea.H-417) > 0.001 {
		t.Errorf("PlotArea.H = %v, want 417", layout.PlotArea.H)
	}
}

func TestComputeCartesianLayout_NoTitle(t *testing.T) {
	config, style := defaultTestLayoutInputs()
	config.ShowTitle = false

	layout := ComputeCartesianLayout(config, style, "Title", "Subtitle", "Footnote", 3)

	if layout.HeaderHeight != 0 {
		t.Errorf("HeaderHeight = %v, want 0 when ShowTitle=false", layout.HeaderHeight)
	}
}

func TestComputeCartesianLayout_NoSubtitle(t *testing.T) {
	config, style := defaultTestLayoutInputs()

	layout := ComputeCartesianLayout(config, style, "Title", "", "Footnote", 3)

	// headerHeight = SizeTitle(18) + Spacing.MD(8) = 26
	if math.Abs(layout.HeaderHeight-26) > 0.001 {
		t.Errorf("HeaderHeight = %v, want 26 (title only, no subtitle)", layout.HeaderHeight)
	}
}

func TestComputeCartesianLayout_NoFootnote(t *testing.T) {
	config, style := defaultTestLayoutInputs()

	layout := ComputeCartesianLayout(config, style, "Title", "Subtitle", "", 3)

	if layout.FooterHeight != 0 {
		t.Errorf("FooterHeight = %v, want 0 when footnote is empty", layout.FooterHeight)
	}
}

func TestComputeCartesianLayout_NoLegend(t *testing.T) {
	t.Run("ShowLegend=false", func(t *testing.T) {
		config, style := defaultTestLayoutInputs()
		config.ShowLegend = false

		layout := ComputeCartesianLayout(config, style, "Title", "", "", 3)

		if layout.LegendHeight != 0 {
			t.Errorf("LegendHeight = %v, want 0 when ShowLegend=false", layout.LegendHeight)
		}
	})

	t.Run("seriesCount<=1", func(t *testing.T) {
		config, style := defaultTestLayoutInputs()
		// ShowLegend is true by default

		layout := ComputeCartesianLayout(config, style, "Title", "", "", 1)

		if layout.LegendHeight != 0 {
			t.Errorf("LegendHeight = %v, want 0 when seriesCount=1", layout.LegendHeight)
		}
	})
}

func TestComputeCartesianLayout_LegendBottom(t *testing.T) {
	config, style := defaultTestLayoutInputs()
	config.LegendPosition = LegendPositionBottom

	withLegend := ComputeCartesianLayout(config, style, "Title", "", "", 3)

	config2, _ := defaultTestLayoutInputs()
	config2.ShowLegend = false

	withoutLegend := ComputeCartesianLayout(config2, style, "Title", "", "", 3)

	// When legend is at bottom, plot area height should be reduced by legendHeight.
	diff := withoutLegend.PlotArea.H - withLegend.PlotArea.H
	if math.Abs(diff-withLegend.LegendHeight) > 0.001 {
		t.Errorf("PlotArea.H difference = %v, want %v (legendHeight)", diff, withLegend.LegendHeight)
	}
}

func TestComputeCartesianLayout_CrossChartConsistency(t *testing.T) {
	// The KEY test: all Cartesian chart types (bar, line, scatter, waterfall) use
	// ComputeCartesianLayout with the same ChartConfig, so they must produce
	// identical layout. We call the function 4 times with the same inputs
	// (simulating each chart type) and verify the results are identical.
	config, style := defaultTestLayoutInputs()

	title := "Revenue by Quarter"
	subtitle := "FY2025"
	footnote := "Source: internal data"

	chartTypes := []struct {
		name        string
		seriesCount int
	}{
		{"BarChart", 3},
		{"LineChart", 3},
		{"ScatterChart", 3},
		{"WaterfallChart", 1}, // waterfall always passes seriesCount=1
	}

	// Compute layout for the first type as reference.
	ref := ComputeCartesianLayout(config, style, title, subtitle, footnote, chartTypes[0].seriesCount)

	// All multi-series chart types must match.
	for _, ct := range chartTypes[1:] {
		t.Run(ct.name, func(t *testing.T) {
			got := ComputeCartesianLayout(config, style, title, subtitle, footnote, ct.seriesCount)

			// HeaderHeight must always match (independent of series count).
			if got.HeaderHeight != ref.HeaderHeight {
				t.Errorf("HeaderHeight = %v, want %v (same as %s)", got.HeaderHeight, ref.HeaderHeight, chartTypes[0].name)
			}

			// FooterHeight must always match (independent of series count).
			if got.FooterHeight != ref.FooterHeight {
				t.Errorf("FooterHeight = %v, want %v (same as %s)", got.FooterHeight, ref.FooterHeight, chartTypes[0].name)
			}

			// For matching seriesCount, ALL fields must match.
			if ct.seriesCount == chartTypes[0].seriesCount {
				if got.LegendHeight != ref.LegendHeight {
					t.Errorf("LegendHeight = %v, want %v (same as %s)", got.LegendHeight, ref.LegendHeight, chartTypes[0].name)
				}
				if got.PlotArea != ref.PlotArea {
					t.Errorf("PlotArea = %+v, want %+v (same as %s)", got.PlotArea, ref.PlotArea, chartTypes[0].name)
				}
			}
		})
	}

	// Additionally, verify that two calls with identical args produce bit-identical results.
	t.Run("Deterministic", func(t *testing.T) {
		a := ComputeCartesianLayout(config, style, title, subtitle, footnote, 3)
		b := ComputeCartesianLayout(config, style, title, subtitle, footnote, 3)

		if a != b {
			t.Errorf("Two identical calls produced different results:\n  a=%+v\n  b=%+v", a, b)
		}
	})
}

// =============================================================================
// ComputeLabelStep Tests
// =============================================================================

func TestComputeLabelStep(t *testing.T) {
	tests := []struct {
		numCategories int
		expected      int
	}{
		{5, 1},
		{10, 1},
		{14, 1},
		{15, 2},
		{19, 2},
		{20, 2},  // (20+9)/10 = 2
		{25, 4},  // (25+7)/8 = 4
		{50, 7},  // (50+7)/8 = 7
		{100, 13}, // (100+7)/8 = 13
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := ComputeLabelStep(tt.numCategories)
			if got != tt.expected {
				t.Errorf("ComputeLabelStep(%d) = %d, want %d", tt.numCategories, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// AdaptXLabels Tests
// =============================================================================

func TestAdaptXLabels_WaterfallHalfWidth(t *testing.T) {
	// Regression test for go-slide-creator-lb0pr:
	// A waterfall chart with 6 typical labels in a half-width (two-column)
	// layout should NOT require rotation. Labels like "FY24 Base" and
	// "Downgrades" fit horizontally at the floor font size when the
	// measurement safety factor is reasonable.
	b := NewSVGBuilder(480, 540) // Approximate half_16x9 in points
	categories := []string{"FY24 Base", "New Logos", "Expansion", "Churn", "Downgrades", "FY25 Total"}

	// Approximate plotWidth for this chart size (width minus margins)
	plotWidth := 410.0
	baseFontSize := 9.0 // SizeSmall at floor after scaling
	isNarrow := true     // 480 < 500

	layout := AdaptXLabels(b, categories, plotWidth, baseFontSize, isNarrow)

	if layout.Rotation != 0 {
		t.Errorf("AdaptXLabels() Rotation = %v, want 0 (labels should fit horizontally for 6 waterfall categories)",
			layout.Rotation)
	}
	if layout.LabelStep != 1 {
		t.Errorf("AdaptXLabels() LabelStep = %v, want 1 (no thinning needed for 6 labels)",
			layout.LabelStep)
	}
	if layout.FontSize < 9.0 {
		t.Errorf("AdaptXLabels() FontSize = %v, want >= 9.0", layout.FontSize)
	}
}

func TestAdaptXLabels_DenseLabelsStillRotate(t *testing.T) {
	// Ensure that charts with many or very long labels still rotate as needed.
	b := NewSVGBuilder(480, 540)
	categories := []string{
		"Prior Year NOI", "Same-Store Growth", "Acquisitions",
		"Dispositions", "Vacancy Impact", "Capital Improvements",
		"Development Pipeline", "Management Fees", "Current NOI",
	}

	plotWidth := 410.0
	baseFontSize := 9.0
	isNarrow := true

	layout := AdaptXLabels(b, categories, plotWidth, baseFontSize, isNarrow)

	// 9 labels with long names should trigger rotation
	if layout.Rotation == 0 {
		t.Errorf("AdaptXLabels() Rotation = 0, expected rotation for 9 long labels at half-width")
	}
}

// =============================================================================
// DrawCartesianGrid Tests
// =============================================================================

func TestDrawCartesianGrid_NilScales(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	plotArea := Rect{X: 60, Y: 40, W: 300, H: 200}

	t.Run("both nil", func(t *testing.T) {
		// Should not panic with both scales nil.
		DrawCartesianGrid(b, plotArea, nil, nil)
	})

	t.Run("yScale nil, xScale non-nil", func(t *testing.T) {
		xScale := NewLinearScale(0, 100)
		xScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)
		DrawCartesianGrid(b, plotArea, nil, xScale)
	})

	t.Run("yScale non-nil, xScale nil", func(t *testing.T) {
		yScale := NewLinearScale(0, 100)
		yScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)
		DrawCartesianGrid(b, plotArea, yScale, nil)
	})

	t.Run("both non-nil", func(t *testing.T) {
		yScale := NewLinearScale(0, 100)
		yScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y)
		xScale := NewLinearScale(0, 100)
		xScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)
		DrawCartesianGrid(b, plotArea, yScale, xScale)
	})
}
