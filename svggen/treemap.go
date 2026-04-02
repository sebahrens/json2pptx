package svggen

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// =============================================================================
// Treemap Chart
// =============================================================================

// TreemapChartConfig holds configuration for treemap charts.
type TreemapChartConfig struct {
	ChartConfig

	// Padding between cells in points.
	Padding float64

	// CornerRadius rounds cell corners.
	CornerRadius float64

	// LabelMinSize is the minimum cell dimension to show labels.
	LabelMinSize float64

	// ShowLabels enables cell labels.
	ShowLabels bool

	// ShowValues enables value labels inside cells.
	ShowValueLabels bool

	// LabelPosition determines where labels are placed.
	LabelPosition TreemapLabelPosition

	// Algorithm is the layout algorithm to use.
	Algorithm TreemapAlgorithm
}

// TreemapLabelPosition determines label placement in treemap cells.
type TreemapLabelPosition string

const (
	// TreemapLabelTop places labels at the top of the cell.
	TreemapLabelTop TreemapLabelPosition = "top"

	// TreemapLabelCenter places labels at the center of the cell.
	TreemapLabelCenter TreemapLabelPosition = "center"
)

// TreemapAlgorithm determines the layout algorithm.
type TreemapAlgorithm string

const (
	// TreemapSquarify uses the squarified algorithm for better aspect ratios.
	TreemapSquarify TreemapAlgorithm = "squarify"

	// TreemapSliceAndDice alternates between horizontal and vertical slicing.
	TreemapSliceAndDice TreemapAlgorithm = "slice-dice"
)

// DefaultTreemapChartConfig returns default treemap chart configuration.
func DefaultTreemapChartConfig(width, height float64) TreemapChartConfig {
	return TreemapChartConfig{
		ChartConfig:     DefaultChartConfig(width, height),
		Padding:         2,
		CornerRadius:    4,
		LabelMinSize:    12,
		ShowLabels:      true,
		ShowValueLabels: true,
		LabelPosition:   TreemapLabelCenter,
		Algorithm:       TreemapSquarify,
	}
}

// TreemapNode represents a node in the treemap hierarchy.
type TreemapNode struct {
	// Label is the node label.
	Label string

	// Value is the node value (size of the area).
	Value float64

	// Children are child nodes for hierarchical data.
	Children []*TreemapNode

	// Color overrides the default color for this node.
	Color *Color

	// computed layout bounds (filled during layout)
	bounds Rect
}

// TotalValue returns the total value of this node (sum of children or own value).
func (n *TreemapNode) TotalValue() float64 {
	if len(n.Children) == 0 {
		return n.Value
	}
	total := 0.0
	for _, child := range n.Children {
		total += child.TotalValue()
	}
	return total
}

// TreemapData represents the data for a treemap chart.
type TreemapData struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Nodes are the top-level treemap nodes.
	Nodes []*TreemapNode

	// Footnote is an optional footnote text.
	Footnote string
}

// TreemapChart renders treemap charts.
type TreemapChart struct {
	builder *SVGBuilder
	config  TreemapChartConfig
}

// NewTreemapChart creates a new treemap chart renderer.
func NewTreemapChart(builder *SVGBuilder, config TreemapChartConfig) *TreemapChart {
	return &TreemapChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the treemap chart.
func (tc *TreemapChart) Draw(data TreemapData) error {
	if len(data.Nodes) == 0 {
		return fmt.Errorf("treemap chart requires at least one node")
	}

	b := tc.builder
	style := b.StyleGuide()
	colors := tc.getColors(style, len(data.Nodes))

	// Calculate plot area
	plotArea := tc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if tc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight

	// Calculate total value
	totalValue := 0.0
	for _, node := range data.Nodes {
		totalValue += node.TotalValue()
	}

	if totalValue == 0 {
		return fmt.Errorf("treemap total value cannot be zero")
	}

	// Apply squarified layout algorithm
	tc.squarifyLayout(data.Nodes, plotArea, totalValue)

	// Draw nodes
	tc.drawNodes(data.Nodes, colors, 0)

	// Draw title
	if tc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: tc.config.Width, H: headerHeight + tc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		fh := FootnoteReservedHeight(style)
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: tc.config.Height - fh,
			W: tc.config.Width,
			H: fh,
		})
	}

	return nil
}

// squarifyLayout applies the squarified treemap algorithm.
// This produces rectangles with aspect ratios closer to 1 (more square-like).
func (tc *TreemapChart) squarifyLayout(nodes []*TreemapNode, bounds Rect, totalValue float64) {
	if len(nodes) == 0 || totalValue == 0 {
		return
	}

	// Sort nodes by value descending for better layout
	sortedNodes := make([]*TreemapNode, len(nodes))
	copy(sortedNodes, nodes)
	slices.SortFunc(sortedNodes, func(a, b *TreemapNode) int {
		return cmp.Compare(b.TotalValue(), a.TotalValue()) // descending
	})

	// Normalize values to fill the available area
	area := bounds.W * bounds.H
	normalizedValues := make([]float64, len(sortedNodes))
	for i, node := range sortedNodes {
		normalizedValues[i] = (node.TotalValue() / totalValue) * area
	}

	tc.squarify(sortedNodes, normalizedValues, []int{}, bounds, tc.shortestSide(bounds))
}

// squarify implements the recursive squarified algorithm.
func (tc *TreemapChart) squarify(nodes []*TreemapNode, values []float64, row []int, bounds Rect, width float64) {
	if len(values) == 0 {
		tc.layoutRow(nodes, values, row, bounds, width)
		return
	}

	// Try adding the next item to the row
	newRow := append(row, len(row))
	rowValues := tc.getRowValues(values, newRow)

	if len(row) == 0 || tc.worstAspect(rowValues, width) <= tc.worstAspect(tc.getRowValues(values, row), width) {
		// Adding improves or maintains aspect ratio - continue
		if len(newRow) < len(values) {
			tc.squarify(nodes, values, newRow, bounds, width)
		} else {
			tc.layoutRow(nodes, values, newRow, bounds, width)
		}
	} else {
		// Adding makes it worse - layout current row and start new one
		remainingBounds := tc.layoutRow(nodes, values, row, bounds, width)
		remainingValues := values[len(row):]
		remainingNodes := nodes[len(row):]
		tc.squarify(remainingNodes, remainingValues, []int{}, remainingBounds, tc.shortestSide(remainingBounds))
	}
}

// getRowValues extracts values for the given row indices.
func (tc *TreemapChart) getRowValues(values []float64, row []int) []float64 {
	result := make([]float64, len(row))
	for i, idx := range row {
		if idx < len(values) {
			result[i] = values[idx]
		}
	}
	return result
}

// worstAspect calculates the worst aspect ratio in a row of values.
func (tc *TreemapChart) worstAspect(values []float64, width float64) float64 {
	if len(values) == 0 || width == 0 {
		return math.Inf(1)
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	if sum == 0 {
		return math.Inf(1)
	}

	worst := 0.0
	for _, v := range values {
		if v <= 0 {
			continue
		}
		// Calculate the rectangle dimensions
		h := v / (sum / width)
		w := sum / width
		aspect := math.Max(h/w, w/h)
		if aspect > worst {
			worst = aspect
		}
	}

	return worst
}

// shortestSide returns the shortest side of a rectangle.
func (tc *TreemapChart) shortestSide(bounds Rect) float64 {
	return math.Min(bounds.W, bounds.H)
}

// layoutRow positions nodes in a row and returns the remaining bounds.
func (tc *TreemapChart) layoutRow(nodes []*TreemapNode, values []float64, row []int, bounds Rect, width float64) Rect {
	if len(row) == 0 {
		return bounds
	}

	rowValues := tc.getRowValues(values, row)
	sum := 0.0
	for _, v := range rowValues {
		sum += v
	}

	if sum == 0 {
		return bounds
	}

	// Determine layout direction based on bounds aspect ratio
	horizontal := bounds.W >= bounds.H

	var rowLength float64
	if horizontal {
		rowLength = sum / bounds.H
	} else {
		rowLength = sum / bounds.W
	}

	// Position each node in the row
	offset := 0.0
	for i, idx := range row {
		if idx >= len(nodes) {
			continue
		}

		node := nodes[idx]
		value := rowValues[i]

		var nodeW, nodeH, nodeX, nodeY float64
		if horizontal {
			nodeW = rowLength
			nodeH = value / rowLength
			nodeX = bounds.X
			nodeY = bounds.Y + offset
			offset += nodeH
		} else {
			nodeW = value / rowLength
			nodeH = rowLength
			nodeX = bounds.X + offset
			nodeY = bounds.Y
			offset += nodeW
		}

		// Apply padding
		node.bounds = Rect{
			X: nodeX + tc.config.Padding/2,
			Y: nodeY + tc.config.Padding/2,
			W: math.Max(0, nodeW-tc.config.Padding),
			H: math.Max(0, nodeH-tc.config.Padding),
		}
	}

	// Return remaining bounds
	if horizontal {
		return Rect{
			X: bounds.X + rowLength,
			Y: bounds.Y,
			W: bounds.W - rowLength,
			H: bounds.H,
		}
	}
	return Rect{
		X: bounds.X,
		Y: bounds.Y + rowLength,
		W: bounds.W,
		H: bounds.H - rowLength,
	}
}

// drawNodes recursively draws treemap nodes.
func (tc *TreemapChart) drawNodes(nodes []*TreemapNode, colors []Color, depth int) {
	b := tc.builder
	style := b.StyleGuide()

	for i, node := range nodes {
		bounds := node.bounds
		if bounds.W <= 0 || bounds.H <= 0 {
			continue
		}

		// Get color for this node
		color := colors[i%len(colors)]
		if node.Color != nil {
			color = *node.Color
		}

		// Draw cell background
		b.Push()
		b.SetFillColor(color)
		b.SetStrokeColor(color.Darken(0.2))
		b.SetStrokeWidth(1)

		if tc.config.CornerRadius > 0 {
			b.DrawRoundedRect(bounds, tc.config.CornerRadius)
		} else {
			b.FillRect(bounds)
			b.StrokeRect(bounds)
		}
		b.Pop()

		// Draw label if cell is large enough to fit readable text.
		// Use a minimum font size check: compute what font would fit, and
		// suppress only when even the smallest readable size won't fit.
		// For cells below LabelMinSize but at least 8px, render an abbreviated
		// label (first 2 characters) so no cell is completely unlabeled.
		const minLabelFontSize = 6.0
		const abbreviatedMinSize = 8.0
		if tc.config.ShowLabels {
			if bounds.W >= tc.config.LabelMinSize && bounds.H >= tc.config.LabelMinSize {
				candidateFontSize := math.Min(style.Typography.SizeSmall, math.Min(bounds.W/5, bounds.H/3))
				if candidateFontSize >= minLabelFontSize {
					tc.drawNodeLabel(node, bounds, style)
				} else {
					tc.drawAbbreviatedLabel(node, bounds, style)
				}
			} else if bounds.W >= abbreviatedMinSize && bounds.H >= abbreviatedMinSize {
				tc.drawAbbreviatedLabel(node, bounds, style)
			}
		}

		// Recursively draw children
		if len(node.Children) > 0 {
			// Calculate inner bounds for children
			innerBounds := Rect{
				X: bounds.X + tc.config.Padding,
				Y: bounds.Y + tc.config.Padding,
				W: bounds.W - 2*tc.config.Padding,
				H: bounds.H - 2*tc.config.Padding,
			}

			// Calculate child total value
			childTotal := 0.0
			for _, child := range node.Children {
				childTotal += child.TotalValue()
			}

			// Layout children
			if childTotal > 0 {
				tc.squarifyLayout(node.Children, innerBounds, childTotal)
				tc.drawNodes(node.Children, colors, depth+1)
			}
		}
	}
}

// drawNodeLabel draws the label for a treemap node.
func (tc *TreemapChart) drawNodeLabel(node *TreemapNode, bounds Rect, style *StyleGuide) {
	b := tc.builder

	// Determine text color based on background luminance
	textColor := MustParseColor("#FFFFFF")

	b.Push()
	b.SetTextColor(textColor)

	// Calculate font size based on cell size, with a 6pt floor for readability.
	fontSize := math.Min(style.Typography.SizeSmall, math.Min(bounds.W/5, bounds.H/3))
	fontSize = math.Max(fontSize, 6.0)
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightMedium)

	var labelX, labelY float64
	switch tc.config.LabelPosition {
	case TreemapLabelTop:
		labelX = bounds.X + bounds.W/2
		labelY = bounds.Y + fontSize + tc.config.Padding
		b.DrawText(node.Label, labelX, labelY, TextAlignCenter, TextBaselineMiddle)

	case TreemapLabelCenter:
		labelX = bounds.X + bounds.W/2
		labelY = bounds.Y + bounds.H/2
		if tc.config.ShowValueLabels {
			// Label above center
			b.DrawText(node.Label, labelX, labelY-fontSize/2, TextAlignCenter, TextBaselineBottom)
			// Value below center
			b.SetFontSize(fontSize * 0.8)
			valueText := formatValue(node.TotalValue(), tc.config.ValueFormat)
			b.DrawText(valueText, labelX, labelY+fontSize/2, TextAlignCenter, TextBaselineTop)
		} else {
			b.DrawText(node.Label, labelX, labelY, TextAlignCenter, TextBaselineMiddle)
		}
	}

	b.Pop()
}

// drawAbbreviatedLabel draws a short abbreviated label (first 2 characters)
// for treemap cells that are too small for a full label.
func (tc *TreemapChart) drawAbbreviatedLabel(node *TreemapNode, bounds Rect, style *StyleGuide) {
	if node.Label == "" {
		return
	}

	b := tc.builder
	textColor := MustParseColor("#FFFFFF")

	// Abbreviate: use first 2 characters (or full label if shorter).
	abbrev := node.Label
	if len(abbrev) > 2 {
		abbrev = abbrev[:2]
	}

	b.Push()
	b.SetTextColor(textColor)
	b.SetFontSize(6.0)
	b.SetFontWeight(style.Typography.WeightMedium)

	labelX := bounds.X + bounds.W/2
	labelY := bounds.Y + bounds.H/2
	b.DrawText(abbrev, labelX, labelY, TextAlignCenter, TextBaselineMiddle)
	b.Pop()
}

// getColors returns colors for the treemap nodes.
func (tc *TreemapChart) getColors(style *StyleGuide, count int) []Color {
	return resolveColors(tc.config.Colors, style, count)
}

// =============================================================================
// Treemap Chart Diagram Type (for Registry)
// =============================================================================

// TreemapDiagram implements the Diagram interface for treemap charts.
type TreemapDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for treemap charts.
func (d *TreemapDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("treemap chart requires data. Expected format: {\"nodes\": [{\"label\": \"Category A\", \"value\": 100}, {\"label\": \"Category B\", \"value\": 60}]}")
	}

	// Accept "items" as alias for "nodes"
	if _, hasItems := req.Data["items"]; hasItems {
		if _, hasNodes := req.Data["nodes"]; !hasNodes {
			req.Data["nodes"] = req.Data["items"]
		}
	}

	// Check for nodes or values array
	_, hasNodes := req.Data["nodes"]
	_, hasValues := req.Data["values"]

	if !hasNodes && !hasValues {
		return fmt.Errorf("treemap chart requires 'nodes', 'items', or 'values' array in data. Expected: {\"nodes\": [{\"label\": \"Category A\", \"value\": 100, \"children\": [...]}]} or {\"categories\": [\"A\", \"B\"], \"values\": [100, 60]}")
	}

	return nil
}

// Render generates an SVG document from the request envelope.
func (d *TreemapDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
func (d *TreemapDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseTreemapData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultTreemapChartConfig(width, height)
		config.ShowLabels = true
		config.ShowValueLabels = req.Style.ShowValues

		// Apply custom options
		if padding, ok := req.Data["padding"].(float64); ok {
			config.Padding = padding
		}
		if cornerRadius, ok := req.Data["corner_radius"].(float64); ok {
			config.CornerRadius = cornerRadius
		}
		if labelPos, ok := req.Data["label_position"].(string); ok {
			config.LabelPosition = TreemapLabelPosition(labelPos)
		}

		chart := NewTreemapChart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// parseTreemapData parses the request data into TreemapData.
func parseTreemapData(req *RequestEnvelope) (TreemapData, error) {
	data := TreemapData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Try parsing nodes array first (hierarchical format)
	if nodesRaw, ok := toAnySlice(req.Data["nodes"]); ok {
		data.Nodes = parseTreemapNodes(nodesRaw)
	} else if valuesRaw, ok := toAnySlice(req.Data["values"]); ok {
		// Try parsing as label-value point objects first (from buildLabelValuePoints)
		if nodes := parseTreemapNodes(valuesRaw); len(nodes) > 0 {
			data.Nodes = nodes
		} else {
			// Fall back to simple float64 values array with categories
			values, _ := toFloat64Slice(valuesRaw)
			categories := []string{}
			if catsRaw, ok := req.Data["categories"]; ok {
				categories, _ = toStringSlice(catsRaw)
			}
			data.Nodes = make([]*TreemapNode, len(values))
			for i, v := range values {
				label := fmt.Sprintf("Item %d", i+1)
				if i < len(categories) {
					label = categories[i]
				}
				data.Nodes[i] = &TreemapNode{Label: label, Value: v}
			}
		}
	} else {
		return data, fmt.Errorf("invalid treemap data format")
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}

// parseTreemapNodes recursively parses treemap nodes from raw data.
func parseTreemapNodes(raw []any) []*TreemapNode {
	nodes := make([]*TreemapNode, 0, len(raw))

	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			node := &TreemapNode{}

			if label, ok := m["label"].(string); ok {
				node.Label = label
			}
			if value, ok := m["value"].(float64); ok {
				node.Value = value
			} else if value, ok := m["value"].(int); ok {
				node.Value = float64(value)
			}
			if colorStr, ok := m["color"].(string); ok {
				if c, err := ParseColor(colorStr); err == nil {
					node.Color = &c
				}
			}

			// Parse children recursively
			if children, ok := m["children"].([]any); ok {
				node.Children = parseTreemapNodes(children)
			}

			nodes = append(nodes, node)
		}
	}

	return nodes
}

