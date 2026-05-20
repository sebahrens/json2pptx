package tokens

// Executive chart-style defaults.
//
// These constants centralise the "consulting-grade" defaults that distinguish
// charts emitted by json2pptx from out-of-the-box Office charts. They are the
// canonical contract: renderers (svggen) and table generators reference these
// values, and the parity test in chart_style_test.go asserts the rendered
// defaults still match.
//
// Per-slide overrides are honoured through the existing `DiagramStyle` /
// `ChartStyle` / `TableStyle` fields — these tokens only describe what the
// engine reaches for when no override is supplied.
//
// Background: bd issue go-slide-creator-bla5. Depends on the tokens layer
// established in clgi.
const (
	// ChartHideAreaBorder, when true, suppresses the outer chart-area
	// rectangle that Office draws by default. Executive consulting decks
	// prefer the plot to read against the slide background.
	ChartHideAreaBorder = true

	// ChartHideVerticalGridlines, when true, suppresses vertical gridlines
	// for Cartesian charts. Horizontal gridlines remain on so the reader
	// can compare bar heights / line values across the y axis.
	ChartHideVerticalGridlines = true

	// ChartLegendMinSeries is the smallest series count for which a legend
	// is rendered by default. A single-series chart carries its label in
	// the title or in a direct label — the legend is redundant.
	ChartLegendMinSeries = 2

	// ChartDirectLabelMaxSeries is the largest series count for which the
	// renderer should prefer direct (in-plot) series labels over a legend.
	// Above this threshold the legend wins because in-plot labels start to
	// collide.
	ChartDirectLabelMaxSeries = 4

	// ChartTickLabelTabularNums, when true, hints that numeric tick labels
	// should render with tabular (monospaced-digit) figures so that columns
	// of numbers align vertically. Renderers that cannot honour the hint
	// (e.g. font does not ship tabular numerals) should fall back silently.
	ChartTickLabelTabularNums = true

	// TableNumericHeaderAlignRight, when true, causes table column headers
	// to inherit the right-alignment of their numeric/currency/percent/delta
	// data column. Without this, a "Revenue" header sits left-aligned over
	// a column of right-aligned dollar figures.
	TableNumericHeaderAlignRight = true
)

// NumericColumnType reports whether a table column_type string identifies a
// numeric column that should be right-aligned. The set mirrors the existing
// alignment override applied to data cells in internal/generator/table.go so
// header and body stay in sync.
func NumericColumnType(columnType string) bool {
	switch columnType {
	case "number", "currency", "percent", "delta":
		return true
	}
	return false
}
