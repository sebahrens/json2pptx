package generator

import (
	"log/slog"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// =============================================================================
// Nine Box Talent Native Shapes — 3x3 Grid of Cells
// =============================================================================
//
// Replaces SVG-rendered nine_box_talent diagrams with native OOXML grouped shapes.
// Each cell is a square-cornered card in its score band's tint, a bold label at the
// top, and item names listed below. All 9 cells plus axis label text boxes are
// wrapped in a single p:grpSp with identity child transform.
//
// Layout (3 columns x 3 rows):
//
//                  Low          Medium         High
//           ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   High    │  Enigma   │       │ Growth   │       │  Star    │
//           │ (neutral) │       │(positive)│       │(positive)│
//           └──────────┘       └──────────┘       └──────────┘
//                 gap                gap                gap
//           ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   Medium  │ Dilemma   │       │  Core    │       │  High    │
//           │(negative) │       │ (neutral) │       │ Performer│
//           └──────────┘       └──────────┘       │(positive)│
//                 gap                gap           └──────────┘
//           ┌──────────┐  gap  ┌──────────┐  gap  ┌──────────┐
//   Low     │  Under   │       │ Average  │       │  Solid   │
//           │Performer │       │Performer │       │Performer │
//           │(negative) │       │(negative) │       │ (neutral) │
//           └──────────┘       └──────────┘       └──────────┘
//
// Color strategy: performance + potential forms five score bands. The bands
// resolve through the template's negative, neutral, and positive semantic
// accents; lightness distinguishes adjacent bands without unrelated hues.

// Nine Box EMU constants.
const (
	// nineBoxGap is the gap between cells in EMU.
	// Same as SWOT/PESTEL gap for visual consistency.
	nineBoxGap int64 = 73152

	// nineBoxLabelFontSize is the cell label font size (hundredths of a point).
	// 1200 = 12pt
	nineBoxLabelFontSize int = 1200

	// nineBoxItemFontSize is the name font size: the 12pt body step, the
	// smallest size a projected slide carries. It was 10pt, in one column
	// (go-slide-creator-35rsq).
	nineBoxItemFontSize int = tokens.TypeScaleBodyHPt

	// nineBoxItemSpaceAfter is the space after each listed name (hundredths
	// of a point).
	nineBoxItemSpaceAfter int = 200

	// nineBoxMaxColumns is the most columns a cell sets its names in.
	nineBoxMaxColumns = 3

	// nineBoxAxisFontSize is the axis tick label font size (hundredths of a
	// point). 1200 = 12pt
	nineBoxAxisFontSize int = tokens.TypeScaleBodyHPt

	// nineBoxAxisTitleFontSize is the axis title font size (hundredths of a
	// point), set bold. 1200 = 12pt
	nineBoxAxisTitleFontSize int = tokens.TypeScaleBodyHPt

	// nineBoxAxisLineOffset is the distance of an axis line from the grid and
	// nineBoxAxisLineClear the room between the line and its labels (EMU).
	nineBoxAxisLineOffset int64 = 5 * 12700
	nineBoxAxisLineClear  int64 = 2 * 12700

	// nineBoxAxisStrip is the thickness of one line of axis text — a title or
	// a row of tick labels (20pt).
	nineBoxAxisStrip int64 = 20 * 12700

	// nineBoxAxisLineWidth is the axis line width (0.75pt).
	nineBoxAxisLineWidth int64 = 9525

	// nineBoxMaxBudgetNames bounds the search for how many names a cell holds.
	nineBoxMaxBudgetNames = 12

	// nineBoxDefaultXTitle / nineBoxDefaultYTitle are the axes a talent grid
	// is drawn on when the author names none: the two scales its employees
	// are placed by (docs/diagrams/nine_box_talent.md).
	nineBoxDefaultXTitle = "Performance"
	nineBoxDefaultYTitle = "Potential"
)

func nineBoxSemanticTints(semanticAccents map[string]string) []taxonomyTint {
	role := func(name, fallback string) string {
		if resolved := strings.TrimSpace(semanticAccents[name]); resolved != "" {
			return resolved
		}
		return fallback
	}
	negative := role("negative", "accent2")
	neutral := role("neutral", "accent3")
	positive := role("positive", "accent1")

	// Score = performance + potential on two 0..2 axes. Each anti-diagonal
	// therefore shares one meaning; only the outer bands need a lightness step
	// to preserve the five-level progression.
	band := [5]taxonomyTint{
		{scheme: negative, lumMod: 35000, lumOff: 65000},
		{scheme: negative, lumMod: 20000, lumOff: 80000},
		{scheme: neutral, lumMod: 25000, lumOff: 75000},
		{scheme: positive, lumMod: 20000, lumOff: 80000},
		{scheme: positive, lumMod: 35000, lumOff: 65000},
	}
	out := make([]taxonomyTint, 0, 9)
	for row := 0; row < 3; row++ {
		potential := 2 - row
		for performance := 0; performance < 3; performance++ {
			out = append(out, band[potential+performance])
		}
	}
	return out
}

// nineBoxDefaultLabels returns the standard 9-box cell labels indexed [row][col].
// Row 0 = high potential, Col 0 = low performance.
var nineBoxDefaultLabels = [3][3]string{
	{"Enigma", "Growth Employee", "Star"},
	{"Dilemma", "Core Employee", "High Performer"},
	{"Under Performer", "Average Performer", "Solid Performer"},
}

// isNineBoxDiagram returns true if the diagram spec is a nine_box_talent diagram type.
func isNineBoxDiagram(spec *types.DiagramSpec) bool {
	return spec.Type == "nine_box_talent"
}

// nineBoxCellData holds parsed data for a single cell in the 3x3 grid.
type nineBoxCellData struct {
	row   int
	col   int
	label string
	items []string // item/person names
}

// nineBoxPanels converts a nine_box_talent spec to panel data — the axis
// metadata first, then the nine cells — and reports how many cells the data
// populated.
func nineBoxPanels(diagramSpec *types.DiagramSpec) ([]nativePanelData, int) {
	// Parse cells and axis info from DiagramSpec.Data.
	cells := parseNineBoxCells(diagramSpec.Data)
	// x_label / y_label are the spellings the svggen renderer took and the
	// shipped example uses. They were accepted and not read, so the grid was
	// drawn with no axes at all (go-slide-creator-35rsq).
	xAxisLabel := nineBoxAxisTitle(diagramSpec.Data, nineBoxDefaultXTitle, "x_axis_label", "x_label")
	yAxisLabel := nineBoxAxisTitle(diagramSpec.Data, nineBoxDefaultYTitle, "y_axis_label", "y_label")
	xAxisLabels := parseNineBoxAxisLabels(diagramSpec.Data, "x_axis_labels")
	yAxisLabels := parseNineBoxAxisLabels(diagramSpec.Data, "y_axis_labels")

	// Convert cells into nativePanelData with axis info stored in first panel's body.
	// We encode axis metadata as a prefix in the panels slice — the generate function
	// will parse it back. This avoids changing the panelShapeInsert struct.
	var panels []nativePanelData

	// Panel 0: axis metadata encoded as title="__nine_box_axes__"
	axisBody := encodeNineBoxAxes(xAxisLabel, yAxisLabel, xAxisLabels, yAxisLabels)
	panels = append(panels, nativePanelData{
		title: "__nine_box_axes__",
		body:  axisBody,
	})

	// Panels 1-9: one per cell (all 9 cells, even empty ones)
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			label := nineBoxDefaultLabels[row][col]
			var itemNames []string

			// Find cell data if provided
			for _, c := range cells {
				if c.row == row && c.col == col {
					if c.label != "" {
						label = c.label
					}
					itemNames = c.items
					break
				}
			}

			body := ""
			if len(itemNames) > 0 {
				bulletLines := make([]string, len(itemNames))
				for i, name := range itemNames {
					bulletLines[i] = "- " + name
				}
				body = strings.Join(bulletLines, "\n")
			}

			panels = append(panels, nativePanelData{
				title: label,
				body:  body,
			})
		}
	}
	return panels, len(cells)
}

// nineBoxAxisTitle returns the axis title stated under the first of keys that
// carries one, or def when none does.
func nineBoxAxisTitle(data map[string]any, def string, keys ...string) string {
	for _, k := range keys {
		if s, ok := data[k].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return def
}

// encodeNineBoxAxes encodes axis labels into a single string for transport in nativePanelData.
// Format: "xTitle\nxL0\nxL1\nxL2\nyTitle\nyL0\nyL1\nyL2"
func encodeNineBoxAxes(xTitle, yTitle string, xLabels, yLabels [3]string) string {
	parts := []string{xTitle, xLabels[0], xLabels[1], xLabels[2], yTitle, yLabels[0], yLabels[1], yLabels[2]}
	return strings.Join(parts, "\n")
}

// decodeNineBoxAxes decodes axis labels from the encoded string.
func decodeNineBoxAxes(encoded string) (xTitle string, yTitle string, xLabels, yLabels [3]string) {
	parts := strings.Split(encoded, "\n")
	if len(parts) >= 8 {
		xTitle = parts[0]
		xLabels = [3]string{parts[1], parts[2], parts[3]}
		yTitle = parts[4]
		yLabels = [3]string{parts[5], parts[6], parts[7]}
	}
	return
}

// Laying the grid out from its text (go-slide-creator-35rsq).
//
// A nine-box is a regular 3×3 grid on two axes, so its rows stay equal; what
// varies is how many names a cell lists. A landscape content area gives a
// cell about three times the width of a name and, on the shortest shipped
// templates, the height of two 12pt lines under its label. Names were written
// at 10pt in one column and shrunk to 8.8pt when a cell held four. They are
// now set at 12pt and the cell gives, in this order:
//
//  1. Width before height. A cell whose names do not fit in one column sets
//     them in two or three, filled column by column.
//  2. Padding before type. A grid that still does not fit steps the top and
//     bottom text margins down nativeVerticalPadSteps.
//  3. Only then does the writer store an autofit scale, which generation
//     reports with the budget nineBoxFitBudget measured.
//
// The axes are drawn as axes — a line with an arrowhead along the bottom and
// up the left, each over its title — so the grid reads as performance against
// potential rather than as nine cards. They took 0.75in of height for a row of
// Low / Medium / High and a title; the arrow says which way a scale rises, and
// the tick labels are drawn only when the author names them.

// nineBoxCell is one cell laid out: its label, and its names split into the
// columns they are set in.
type nineBoxCell struct {
	title   string
	columns [][]string
}

// nineBoxLayout is the grid's geometry and the name columns of each cell.
type nineBoxLayout struct {
	gridX, gridY, gridW, gridH int64
	cellW, cellH, labelCY      int64
	pad                        int64
	cells                      [9]nineBoxCell
	axes                       nineBoxAxes
	fits                       bool
}

// nineBoxAxes are the axis titles and the tick labels the author named, the
// y ticks in row order (top row first).
type nineBoxAxes struct {
	xTitle, yTitle string
	xTicks, yTicks [3]string
}

func (a nineBoxAxes) hasX() bool { return a.xTitle != "" || a.xTicks != [3]string{} }
func (a nineBoxAxes) hasY() bool { return a.yTitle != "" || a.yTicks != [3]string{} }

// band is the thickness of an axis: the line's offset from the grid, then a
// strip for the ticks and one for the title, when present.
func nineBoxAxisBand(title string, ticks [3]string) int64 {
	if title == "" && ticks == [3]string{} {
		return 0
	}
	band := nineBoxAxisLineOffset + nineBoxAxisLineClear
	if ticks != [3]string{} {
		band += nineBoxAxisStrip
	}
	if title != "" {
		band += nineBoxAxisStrip
	}
	return band
}

// nineBoxNames parses a cell body back into its names.
func nineBoxNames(body string) []string {
	var names []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if name := strings.TrimPrefix(line, "- "); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// nineBoxColumnText is the text body of name column j of n. The first column
// is the body shape's own text, held to its column by the right inset.
func nineBoxColumnText(names []string, j, n int, bodyW, pad int64, tint taxonomyTint, env nativeDiagramEnv) pptx.TextBody {
	// A cell lists names, one per line: the 6pt panel bullet spacing left a
	// four-name cell on a short content area at a third of its size
	// (go-slide-creator-zbo58).
	lines := make([]string, len(names))
	for i, name := range names {
		lines[i] = "- " + name
	}
	var paras []pptx.Paragraph
	if len(lines) > 0 {
		paras = pptx.ParseBulletText(strings.Join(lines, "\n"), pptx.BulletTextOptions{
			FontSize:    nineBoxItemFontSize,
			Lang:        "en-US",
			Dirty:       true,
			BulletColor: pptx.SchemeFill(panelBulletSchemeColor),
			SpaceAfter:  nineBoxItemSpaceAfter,
		})
		// The last name needs no space under it.
		paras[len(paras)-1].SpaceAfter = 0
	}
	// Use the same accent color for bullets but with full strength.
	bulletColor := pptx.ResolveColorString(tint.scheme)
	for i := range paras {
		if paras[i].Bullet != nil {
			paras[i].Bullet.Color = bulletColor
		}
	}
	diagramPanelBodyColors(paras, tint.scheme)

	insets := nativeCardBodyInsets()
	insets[3] = pad
	insets = nativeColumnInsets(insets, j, n, bodyW)
	return pptx.TextBody{
		Wrap:       "square",
		Anchor:     "t",
		Insets:     insets,
		AutoFit:    "normAutofit",
		Paragraphs: paras,
		ThemeFonts: pptx.ThemeFonts{Major: env.fontName, Minor: env.fontName},
	}
}

// nineBoxLabelText is the text body of a cell's label.
func nineBoxLabelText(label string, pad int64, tint taxonomyTint, env nativeDiagramEnv) pptx.TextBody {
	insets := nativeCardHeaderInsets()
	insets[1] = pad
	return pptx.TextBody{
		Wrap:    "square",
		Anchor:  "ctr",
		Insets:  insets,
		AutoFit: "normAutofit",
		Paragraphs: []pptx.Paragraph{{
			Align:    nativeHeaderAlign,
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     label,
				Lang:     "en-US",
				FontSize: nineBoxLabelFontSize,
				Bold:     true,
				Dirty:    true,
				Color:    tint.titleFill(),
			}},
		}},
		ThemeFonts: pptx.ThemeFonts{Major: env.fontName, Minor: env.fontName},
	}
}

// nineBoxColumnsFit reports whether names, set in c columns, are written
// without a shrink in a cell body of the given size.
func nineBoxColumnsFit(names []string, c int, body pptx.RectEmu, pad int64, tint taxonomyTint, env nativeDiagramEnv) bool {
	cols := nativeSplitColumns(names, c)
	for j, col := range cols {
		text := nineBoxColumnText(col, j, len(cols), body.CX, pad, tint, env)
		rect := nativeColumnRect(body, j, len(cols))
		if nativeTextNeedAtMarginEMU(text, rect.CX, rect.CY+1) > rect.CY {
			return false
		}
		// Columns are for short names: one that wraps in its column reads
		// worse there than in a single column at a tighter margin.
		if len(cols) > 1 {
			blank := make([]string, len(col))
			for i := range blank {
				blank[i] = "x"
			}
			oneLine := nativeTextNeedAtMarginEMU(nineBoxColumnText(blank, j, len(cols), body.CX, pad, tint, env), rect.CX, rect.CY+1)
			if nativeTextNeedAtMarginEMU(text, rect.CX, rect.CY+1) > oneLine+int64(types.EMUPerPoint) {
				return false
			}
		}
	}
	return true
}

// layoutNineBox lays the nine cells and the axes out in bounds. panels is the
// encoded list: the axis metadata, then the nine cells row by row.
func layoutNineBox(panels []nativePanelData, bounds types.BoundingBox, tints []taxonomyTint, env nativeDiagramEnv) nineBoxLayout {
	var l nineBoxLayout
	xTitle, yTitle, xTicks, yTicks := decodeNineBoxAxes(panels[0].body)
	// Y-axis labels are in ascending order [low, medium, high] by convention,
	// but rows are top-to-bottom [high, medium, low], so reverse them.
	l.axes = nineBoxAxes{xTitle: xTitle, yTitle: yTitle, xTicks: xTicks, yTicks: [3]string{yTicks[2], yTicks[1], yTicks[0]}}

	yBand := nineBoxAxisBand(l.axes.yTitle, l.axes.yTicks)
	xBand := nineBoxAxisBand(l.axes.xTitle, l.axes.xTicks)
	l.gridX, l.gridY = bounds.X+yBand, bounds.Y
	l.gridW, l.gridH = bounds.Width-yBand, bounds.Height-xBand
	l.cellW = (l.gridW - 2*nineBoxGap) / 3
	l.cellH = (l.gridH - 2*nineBoxGap) / 3

	names := [9][]string{}
	for i := range names {
		l.cells[i].title = panels[1+i].title
		names[i] = nineBoxNames(panels[1+i].body)
	}

	for _, pad := range nativeVerticalPadSteps {
		l.pad = pad
		l.labelCY = 0
		for i := range l.cells {
			l.labelCY = max(l.labelCY, nativeHeaderNeedEMU(nineBoxLabelText(l.cells[i].title, pad, tints[i], env), l.cellW, l.cellH))
		}
		body := pptx.RectEmu{CX: l.cellW, CY: l.cellH - l.labelCY}
		l.fits = l.labelCY < l.cellH/2
		for i := range l.cells {
			l.cells[i].columns = nil
			if len(names[i]) == 0 {
				continue
			}
			for c := 1; c <= nineBoxMaxColumns && l.cells[i].columns == nil; c++ {
				if nineBoxColumnsFit(names[i], c, body, pad, tints[i], env) {
					l.cells[i].columns = nativeSplitColumns(names[i], c)
				}
			}
			if l.cells[i].columns == nil {
				l.fits = false
			}
		}
		if l.fits {
			return l
		}
	}
	// Nothing left to give but the type: the writer stores the shrink each
	// over-full cell then needs.
	for i := range l.cells {
		if len(names[i]) > 0 && l.cells[i].columns == nil {
			l.cells[i].columns = nativeSplitColumns(names[i], nineBoxOverfullColumns(names[i], l, tints[i], env))
		}
	}
	return l
}

// nineBoxOverfullColumns is how many columns a cell too full for 12pt sets its
// names in: the most that still keep every name on one line, which is the
// shortest list and so the smallest shrink.
func nineBoxOverfullColumns(names []string, l nineBoxLayout, tint taxonomyTint, env nativeDiagramEnv) int {
	tall := pptx.RectEmu{CX: l.cellW, CY: 100 * int64(types.EMUPerInch)}
	need := func(name string, c int) int64 {
		return nativeTextNeedAtMarginEMU(nineBoxColumnText([]string{name}, 1, c, l.cellW, l.pad, tint, env), nativeColumnRect(tall, 1, c).CX, tall.CY)
	}
	for c := nineBoxMaxColumns; c > 1; c-- {
		oneLine := true
		for _, name := range names {
			if need(name, c) > need("x", c)+int64(types.EMUPerPoint) {
				oneLine = false
				break
			}
		}
		if oneLine {
			return c
		}
	}
	return 1
}

// nineBoxFitBudget measures what a cell holds at the authored size in bounds:
// how many names of the diagram's own longest length every cell can list, and
// how many characters a one-line name takes in the columns that count needs.
func nineBoxFitBudget(panels []nativePanelData, bounds types.BoundingBox, tints []taxonomyTint, env nativeDiagramEnv) nativeFitBudget {
	longest := "Name"
	for _, p := range panels[1:] {
		for _, name := range nineBoxNames(p.body) {
			if len([]rune(name)) > len([]rune(longest)) {
				longest = name
			}
		}
	}
	probe := func(n int, name string) nineBoxLayout {
		filled := append([]nativePanelData{}, panels...)
		lines := make([]string, n)
		for i := range lines {
			lines[i] = "- " + name
		}
		for i := 1; i < len(filled); i++ {
			filled[i].body = strings.Join(lines, "\n")
		}
		return layoutNineBox(filled, bounds, tints, env)
	}
	// A name too long for any column is its own problem; count with one that
	// fits a single column so the count says what the height holds.
	if !probe(1, longest).fits {
		longest = "Name"
	}
	items, columns := 0, 1
	for n := 1; n <= nineBoxMaxBudgetNames; n++ {
		l := probe(n, longest)
		if !l.fits {
			break
		}
		items, columns = n, len(l.cells[0].columns)
	}
	l := probe(1, longest)
	bodyW := l.cellW
	chars := nativeOneLineChars(func(s string) pptx.TextBody {
		return nineBoxColumnText([]string{s}, min(1, columns-1), columns, bodyW, l.pad, tints[0], env)
	}, nativeColumnRect(pptx.RectEmu{CX: bodyW}, min(1, columns-1), columns).CX)
	return nativeFitBudget{
		item: "name", container: "cell", maxItems: items, maxChars: chars,
		sizePt: float64(nineBoxItemFontSize) / 100,
	}
}

// generateNineBoxGroupXML produces the complete <p:grpSp> XML for a 3x3 nine box grid
// with its axes.
func generateNineBoxGroupXML(panels []nativePanelData, bounds types.BoundingBox, shapeIDBase uint32, tints []taxonomyTint, env nativeDiagramEnv) string {
	// panels[0] = axis metadata, panels[1..9] = cells [row*3+col]
	if len(panels) != 10 {
		slog.Warn("generateNineBoxGroupXML: expected 10 panels (1 axis + 9 cells)", "got", len(panels))
		return ""
	}
	if len(tints) != 9 {
		tints = nineBoxSemanticTints(nil)
	}
	l := layoutNineBox(panels, bounds, tints, env)

	var children [][]byte
	nextID := shapeIDBase
	id := func() uint32 { nextID++; return nextID }

	// Generate 9 cells.
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			i := row*3 + col
			cell, tint := l.cells[i], tints[i]
			cellX := l.gridX + int64(col)*(l.cellW+nineBoxGap)
			cellY := l.gridY + int64(row)*(l.cellH+nineBoxGap)

			// Label shape: the score band's fill under a bold left-aligned title
			label := nineBoxLabelText(cell.title, l.pad, tint, env)
			children = append(children, []byte(generateNineBoxShapeXML(
				"NineBox "+cell.title, pptx.RectEmu{X: cellX, Y: cellY, CX: l.cellW, CY: l.labelCY}, id(), &tint, label)))

			// Body shape: the same fill, top-aligned names in its first column
			body := pptx.RectEmu{X: cellX, Y: cellY + l.labelCY, CX: l.cellW, CY: l.cellH - l.labelCY}
			n := len(cell.columns)
			var first []string
			if n > 0 {
				first = cell.columns[0]
			}
			children = append(children, []byte(generateNineBoxShapeXML(
				"NineBox Body", body, id(), &tint, nineBoxColumnText(first, 0, n, body.CX, l.pad, tint, env))))
			for j := 1; j < n; j++ {
				children = append(children, []byte(generateNineBoxShapeXML(
					"NineBox Names", nativeColumnRect(body, j, n), id(), nil,
					nineBoxColumnText(cell.columns[j], j, n, body.CX, l.pad, tint, env))))
			}
		}
	}

	children = append(children, nineBoxAxisShapes(l, bounds, id)...)

	groupBounds := pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: bounds.Width, CY: bounds.Height}
	b, err := pptx.GenerateGroup(pptx.GroupOptions{
		ID:       shapeIDBase,
		Name:     "Nine Box Talent",
		Bounds:   groupBounds,
		Children: children,
	})
	if err != nil {
		slog.Warn("generateNineBoxGroupXML failed", "error", err)
		return ""
	}
	return string(b)
}

// nineBoxAxisShapes draws the axes: below the grid a line with an arrowhead
// pointing right, the tick labels the author named and the title; left of it
// the same, turned to read bottom to top with the arrowhead pointing up.
func nineBoxAxisShapes(l nineBoxLayout, bounds types.BoundingBox, id func() uint32) [][]byte {
	var out [][]byte
	if l.axes.hasX() {
		y := l.gridY + l.gridH + nineBoxAxisLineOffset
		out = append(out, []byte(generateNineBoxAxisLineXML(
			pptx.RectEmu{X: l.gridX, Y: y, CX: l.gridW, CY: 0}, id(), "X-Axis", false)))
		y += nineBoxAxisLineClear
		if l.axes.xTicks != [3]string{} {
			for col, tick := range l.axes.xTicks {
				if tick == "" {
					continue
				}
				x := l.gridX + int64(col)*(l.cellW+nineBoxGap)
				out = append(out, []byte(generateNineBoxAxisLabelXML(
					tick, pptx.RectEmu{X: x, Y: y, CX: l.cellW, CY: nineBoxAxisStrip}, id(), nineBoxAxisFontSize, false, false)))
			}
			y += nineBoxAxisStrip
		}
		if l.axes.xTitle != "" {
			out = append(out, []byte(generateNineBoxAxisLabelXML(
				l.axes.xTitle, pptx.RectEmu{X: l.gridX, Y: y, CX: l.gridW, CY: nineBoxAxisStrip}, id(), nineBoxAxisTitleFontSize, true, false)))
		}
	}
	if l.axes.hasY() {
		x := l.gridX - nineBoxAxisLineOffset
		out = append(out, []byte(generateNineBoxAxisLineXML(
			pptx.RectEmu{X: x, Y: l.gridY, CX: 0, CY: l.gridH}, id(), "Y-Axis", true)))
		x -= nineBoxAxisLineClear
		if l.axes.yTicks != [3]string{} {
			x -= nineBoxAxisStrip
			for row, tick := range l.axes.yTicks {
				if tick == "" {
					continue
				}
				y := l.gridY + int64(row)*(l.cellH+nineBoxGap)
				out = append(out, []byte(generateNineBoxAxisLabelXML(
					tick, pptx.RectEmu{X: x, Y: y, CX: nineBoxAxisStrip, CY: l.cellH}, id(), nineBoxAxisFontSize, false, true)))
			}
		}
		if l.axes.yTitle != "" {
			out = append(out, []byte(generateNineBoxAxisLabelXML(
				l.axes.yTitle, pptx.RectEmu{X: bounds.X, Y: l.gridY, CX: nineBoxAxisStrip, CY: l.gridH}, id(), nineBoxAxisTitleFontSize, true, true)))
		}
	}
	return out
}

// generateNineBoxShapeXML produces one text-bearing shape of a cell: a card in
// the cell's tint, or — with no tint — an unfilled text box over one.
func generateNineBoxShapeXML(name string, rect pptx.RectEmu, shapeID uint32, tint *taxonomyTint, text pptx.TextBody) string {
	opts := pptx.ShapeOptions{
		ID:       shapeID,
		Name:     name,
		Bounds:   rect,
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text:     &text,
	}
	if tint != nil {
		opts.Geometry = nativeSurfaceGeometry
		opts.Fill = tint.fill()
		opts.Line = pptx.Line{Width: panelBorderWidth, Fill: pptx.NoFill()}
		opts.TxBox = false
	}
	b, err := pptx.GenerateShape(opts)
	if err != nil {
		slog.Warn("generateNineBoxShapeXML failed", "name", name, "error", err)
		return ""
	}
	return string(b)
}

// generateNineBoxAxisLineXML produces an axis: a straight line with an
// arrowhead at the end its scale rises toward — the right end of the x-axis,
// the top of the y-axis.
func generateNineBoxAxisLineXML(rect pptx.RectEmu, shapeID uint32, name string, upward bool) string {
	b, err := pptx.GenerateConnector(pptx.ConnectorOptions{
		ID:       shapeID,
		Name:     name,
		Geometry: pptx.GeomStraightConnector1,
		Bounds:   rect,
		Line:     pptx.Line{Width: nineBoxAxisLineWidth, Fill: pptx.SchemeFill("dk1")},
		// A vertical line is drawn top to bottom; flipped, it runs bottom to
		// top and its tail — the arrowhead — is at the top.
		FlipV:   upward,
		TailEnd: &pptx.ArrowHead{Type: "triangle", W: "med", Len: "med"},
	})
	if err != nil {
		slog.Warn("generateNineBoxAxisLineXML failed", "error", err)
		return ""
	}
	return string(b)
}

// generateNineBoxAxisLabelXML produces a centred text box for an axis title or
// tick label; vertical ones read bottom to top.
func generateNineBoxAxisLabelXML(text string, rect pptx.RectEmu, shapeID uint32, fontSize int, bold, vertical bool) string {
	body := pptx.TextBody{
		Wrap:    "square",
		Anchor:  "ctr",
		Insets:  pptx.ShapeTextInsets(),
		AutoFit: "noAutofit",
		Paragraphs: []pptx.Paragraph{{
			Align:    "ctr",
			NoBullet: true,
			Runs: []pptx.Run{{
				Text:     text,
				Lang:     "en-US",
				FontSize: fontSize,
				Bold:     bold,
				Dirty:    true,
				Color:    pptx.SchemeFill("dk1"),
			}},
		}},
	}
	name := "Axis Label"
	if vertical {
		body.Vert = "vert270"
		name = "Y-Axis Label"
	}
	b, err := pptx.GenerateShape(pptx.ShapeOptions{
		ID:       shapeID,
		Name:     name,
		Bounds:   rect,
		Geometry: pptx.GeomRect,
		Fill:     pptx.NoFill(),
		TxBox:    true,
		Text:     &body,
	})
	if err != nil {
		slog.Warn("generateNineBoxAxisLabelXML failed", "error", err)
		return ""
	}
	return string(b)
}

// parseNineBoxCells parses cell data from the nine_box_talent diagram data map.
// Supports two formats:
//  1. "cells" array with position, label, and items
//  2. "employees" array with name, performance, potential
func parseNineBoxCells(data map[string]any) []nineBoxCellData {
	// Try "cells" array first.
	if cellsRaw, ok := data["cells"].([]any); ok {
		return parseNineBoxCellsArray(cellsRaw)
	}

	// Try "employees" format.
	if employees, ok := data["employees"].([]any); ok {
		return parseNineBoxEmployees(employees)
	}

	return nil
}

// parseNineBoxCellsArray parses the "cells" format.
func parseNineBoxCellsArray(raw []any) []nineBoxCellData {
	var cells []nineBoxCellData
	for _, r := range raw {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}

		cell := nineBoxCellData{}

		// Parse position from nested "position" object.
		if posMap, ok := m["position"].(map[string]any); ok {
			if row, ok := posMap["row"].(float64); ok {
				cell.row = int(row)
			}
			if col, ok := posMap["col"].(float64); ok {
				cell.col = int(col)
			}
		}
		// Also support flat row/col.
		if row, ok := m["row"].(float64); ok {
			cell.row = int(row)
		}
		if col, ok := m["col"].(float64); ok {
			cell.col = int(col)
		}

		// Parse label.
		if label, ok := m["label"].(string); ok {
			cell.label = label
		}

		// Parse items (array of strings or objects with "name").
		if items, ok := m["items"].([]any); ok {
			for _, item := range items {
				switch v := item.(type) {
				case string:
					cell.items = append(cell.items, v)
				case map[string]any:
					if name, ok := v["name"].(string); ok {
						cell.items = append(cell.items, name)
					}
				}
			}
		}

		cells = append(cells, cell)
	}
	return cells
}

// parseNineBoxEmployees parses the "employees" format, grouping by performance/potential.
func parseNineBoxEmployees(employees []any) []nineBoxCellData {
	// Group employees by grid position.
	type posKey struct{ row, col int }
	cellMap := make(map[posKey][]string)

	for _, e := range employees {
		emp, ok := e.(map[string]any)
		if !ok {
			continue
		}

		name, _ := emp["name"].(string)
		if name == "" {
			continue
		}

		// Accept both the documented string form (low|medium|high) and a
		// numeric 1-3 scale. Field agents frequently send numbers; without
		// normalization the string assertion failed and every employee fell
		// through to the center "Core Employee" cell (go-slide-creator-pc2a).
		performance := normalizeNineBoxLevel(emp["performance"])
		potential := normalizeNineBoxLevel(emp["potential"])

		row, col := employeeToGridPos(performance, potential)
		key := posKey{row, col}
		cellMap[key] = append(cellMap[key], name)
	}

	var cells []nineBoxCellData
	for pos, names := range cellMap {
		cells = append(cells, nineBoxCellData{
			row:   pos.row,
			col:   pos.col,
			items: names,
		})
	}
	return cells
}

// normalizeNineBoxLevel coerces a performance/potential value into the
// canonical low|medium|high token. It accepts the documented string form
// (case-insensitive, plus common synonyms) as well as a numeric 1-3 scale
// (1=low, 2=medium, 3=high) that agents commonly supply. Unrecognized values
// return "" so employeeToGridPos falls back to its medium default.
func normalizeNineBoxLevel(v any) string {
	switch t := v.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "low", "1":
			return "low"
		case "medium", "med", "mid", "moderate", "2":
			return "medium"
		case "high", "3":
			return "high"
		default:
			return ""
		}
	case float64:
		return numericNineBoxLevel(t)
	case int:
		return numericNineBoxLevel(float64(t))
	case int64:
		return numericNineBoxLevel(float64(t))
	}
	return ""
}

// numericNineBoxLevel maps a numeric 1-3 scale onto low|medium|high using
// thresholds so fractional values (e.g. 1.5) still resolve sensibly.
func numericNineBoxLevel(n float64) string {
	switch {
	case n <= 1:
		return "low"
	case n >= 3:
		return "high"
	default:
		return "medium"
	}
}

// employeeToGridPos converts performance/potential strings to grid position.
// Performance -> column: low=0, medium=1, high=2
// Potential -> row: high=0, medium=1, low=2 (inverted — high at top)
func employeeToGridPos(performance, potential string) (row, col int) {
	switch performance {
	case "low":
		col = 0
	case "high":
		col = 2
	default:
		col = 1
	}

	switch potential {
	case "high":
		row = 0
	case "low":
		row = 2
	default:
		row = 1
	}
	return
}

// parseNineBoxAxisLabels parses axis labels from the data map.
func parseNineBoxAxisLabels(data map[string]any, key string) [3]string {
	var labels [3]string
	if arr, ok := data[key].([]any); ok {
		for i, l := range arr {
			if i < 3 {
				if s, ok := l.(string); ok {
					labels[i] = s
				}
			}
		}
	}
	return labels
}
