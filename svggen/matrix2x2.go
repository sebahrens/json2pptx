package svggen

import (
	"fmt"
	"math"
	"strings"
)

// =============================================================================
// 2x2 Matrix (Quadrant Chart)
// =============================================================================

// Matrix2x2Config holds configuration for 2x2 matrix charts.
type Matrix2x2Config struct {
	ChartConfig

	// QuadrantLabels are the labels for each quadrant [top-left, top-right, bottom-left, bottom-right].
	QuadrantLabels [4]string

	// QuadrantColors are the background colors for each quadrant.
	QuadrantColors [4]Color

	// QuadrantOpacity is the fill opacity for quadrant backgrounds.
	QuadrantOpacity float64

	// XAxisLabel is the label for the x-axis (bottom).
	XAxisLabel string

	// YAxisLabel is the label for the y-axis (left).
	YAxisLabel string

	// XAxisMin is the minimum x value (left edge).
	XAxisMin float64

	// XAxisMax is the maximum x value (right edge).
	XAxisMax float64

	// YAxisMin is the minimum y value (bottom edge).
	YAxisMin float64

	// YAxisMax is the maximum y value (top edge).
	YAxisMax float64

	// ShowGridLines enables grid lines at the axis midpoints.
	ShowGridLines bool

	// GridLineColor is the color of the grid lines.
	GridLineColor Color

	// GridLineWidth is the width of the grid lines.
	GridLineWidth float64

	// GridLineDash enables dashed grid lines.
	GridLineDash bool

	// PointSize is the default size for data points.
	PointSize float64

	// PointShape is the default shape for data points.
	PointShape MarkerShape

	// ShowPointLabels enables labels next to points.
	ShowPointLabels bool

	// LabelOffset is the distance from point center to label.
	LabelOffset float64
}

// DefaultMatrix2x2Config returns default matrix 2x2 configuration.
func DefaultMatrix2x2Config(width, height float64) Matrix2x2Config {
	return Matrix2x2Config{
		ChartConfig: DefaultChartConfig(width, height),
		QuadrantLabels: [4]string{
			"High Value / Low Effort",  // top-left
			"High Value / High Effort", // top-right
			"Low Value / Low Effort",   // bottom-left
			"Low Value / High Effort",  // bottom-right
		},
		QuadrantColors: [4]Color{
			MustParseColor("#59A14F").WithAlpha(0.30), // Green tint
			MustParseColor("#EDC948").WithAlpha(0.30), // Yellow tint
			MustParseColor("#76B7B2").WithAlpha(0.30), // Teal tint
			MustParseColor("#E15759").WithAlpha(0.30), // Red tint
		},
		QuadrantOpacity: 0.30,
		XAxisLabel:      "Effort",
		YAxisLabel:      "Value",
		XAxisMin:        0,
		XAxisMax:        100,
		YAxisMin:        0,
		YAxisMax:        100,
		ShowGridLines:   true,
		GridLineColor:   MustParseColor("#6C757D"),
		GridLineWidth:   1.5,
		GridLineDash:    false,
		PointSize:       12,
		PointShape:      MarkerCircle,
		ShowPointLabels: true,
		LabelOffset:     16,
	}
}

// Matrix2x2Point represents a data point in the matrix.
type Matrix2x2Point struct {
	// Label is the point label.
	Label string

	// X is the x-coordinate (typically 0-100).
	X float64

	// Y is the y-coordinate (typically 0-100).
	Y float64

	// Size overrides the default point size.
	Size float64

	// Color overrides the default point color.
	Color *Color

	// Shape overrides the default point shape.
	Shape MarkerShape

	// Description is optional additional text.
	Description string
}

// Matrix2x2Data represents the data for a 2x2 matrix chart.
type Matrix2x2Data struct {
	// Title is the chart title.
	Title string

	// Subtitle is the chart subtitle.
	Subtitle string

	// Points are the data points to plot.
	Points []Matrix2x2Point

	// Footnote is an optional footnote.
	Footnote string
}

// Matrix2x2Chart renders 2x2 matrix charts.
type Matrix2x2Chart struct {
	builder *SVGBuilder
	config  Matrix2x2Config
}

// NewMatrix2x2Chart creates a new matrix 2x2 chart renderer.
func NewMatrix2x2Chart(builder *SVGBuilder, config Matrix2x2Config) *Matrix2x2Chart {
	return &Matrix2x2Chart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the matrix 2x2 chart.
func (mc *Matrix2x2Chart) Draw(data Matrix2x2Data) error {
	b := mc.builder
	style := b.StyleGuide()

	// Calculate plot area
	plotArea := mc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if mc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Reserve space for axis name labels only (no "High"/"Low" value labels —
	// those are implied by the quadrant names and removing them prevents overlap
	// caused by LibreOffice SVG text scaling, bug go-slide-ceator-a8ax).
	textScale := 1.3 // line-height factor for layout calculations
	bodyH := style.Typography.SizeBody * textScale

	// Y-axis label is rotated -90°. Reserve bodyH plus padding.
	leftSpace := bodyH + style.Spacing.LG
	bottomSpace := bodyH + style.Spacing.LG // X-axis name (horizontal text)
	rightPad := style.Spacing.LG

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + bottomSpace
	plotArea.X += leftSpace
	plotArea.W -= leftSpace + rightPad

	// Draw quadrant backgrounds
	mc.drawQuadrants(plotArea)

	// Draw grid lines
	if mc.config.ShowGridLines {
		mc.drawGridLines(plotArea)
	}

	// Draw axis labels
	mc.drawAxisLabels(plotArea)

	// Draw quadrant labels
	mc.drawQuadrantLabels(plotArea)

	// Draw data points
	mc.drawPoints(data.Points, plotArea)

	// Draw title
	if mc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: mc.config.Width, H: headerHeight + mc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		fh := FootnoteReservedHeight(style)
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: mc.config.Height - fh,
			W: mc.config.Width,
			H: fh,
		})
	}

	return nil
}

// drawQuadrants draws the four quadrant backgrounds.
func (mc *Matrix2x2Chart) drawQuadrants(plotArea Rect) {
	b := mc.builder

	halfW := plotArea.W / 2
	halfH := plotArea.H / 2

	quadrants := []Rect{
		{X: plotArea.X, Y: plotArea.Y, W: halfW, H: halfH},                 // top-left (high value, low effort)
		{X: plotArea.X + halfW, Y: plotArea.Y, W: halfW, H: halfH},         // top-right (high value, high effort)
		{X: plotArea.X, Y: plotArea.Y + halfH, W: halfW, H: halfH},         // bottom-left (low value, low effort)
		{X: plotArea.X + halfW, Y: plotArea.Y + halfH, W: halfW, H: halfH}, // bottom-right (low value, high effort)
	}

	for i, rect := range quadrants {
		b.Push()
		color := mc.config.QuadrantColors[i]
		if color.A == 0 {
			color = mc.config.QuadrantColors[i].WithAlpha(mc.config.QuadrantOpacity)
		}
		b.SetFillColor(color)
		b.SetStrokeColor(Color{A: 0}) // No stroke
		b.FillRect(rect)
		b.Pop()
	}
}

// drawGridLines draws the grid lines at the midpoints.
func (mc *Matrix2x2Chart) drawGridLines(plotArea Rect) {
	b := mc.builder

	midX := plotArea.X + plotArea.W/2
	midY := plotArea.Y + plotArea.H/2

	b.Push()
	b.SetStrokeColor(mc.config.GridLineColor)
	b.SetStrokeWidth(mc.config.GridLineWidth)

	if mc.config.GridLineDash {
		b.SetDashes(6, 3)
	}

	// Vertical line (x-axis midpoint)
	b.DrawLine(midX, plotArea.Y, midX, plotArea.Y+plotArea.H)

	// Horizontal line (y-axis midpoint)
	b.DrawLine(plotArea.X, midY, plotArea.X+plotArea.W, midY)

	b.Pop()
}

// drawAxisLabels draws the axis name labels outside the plot area.
// Only axis names are drawn — "High"/"Low" value labels are omitted to avoid
// overlap with quadrant labels (LibreOffice SVG text scaling, bug a8ax).
// The quadrant labels themselves convey the High/Low semantics.
// The Y-axis label is rotated -90° (reads bottom-to-top) using RotateAround;
// the SVG postprocessor (fixSVGMatrixRotations) converts the matrix transform
// to translate+rotate form for LibreOffice compatibility.
func (mc *Matrix2x2Chart) drawAxisLabels(plotArea Rect) {
	b := mc.builder
	style := b.StyleGuide()

	b.Push()

	// X-axis name (bottom center)
	xLabelY := plotArea.Y + plotArea.H + style.Spacing.LG
	b.SetFontSize(style.Typography.SizeBody)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.DrawText(mc.config.XAxisLabel, plotArea.X+plotArea.W/2, xLabelY, TextAlignCenter, TextBaselineTop)

	// Y-axis name (rotated -90°, reads bottom-to-top)
	b.SetFontSize(style.Typography.SizeBody)
	b.SetFontWeight(style.Typography.WeightMedium)
	yLabelX := style.Typography.SizeBody / 2
	yLabelY := plotArea.Y + plotArea.H/2
	b.Push()
	b.RotateAround(-90, yLabelX, yLabelY)
	b.DrawText(mc.config.YAxisLabel, yLabelX, yLabelY, TextAlignCenter, TextBaselineMiddle)
	b.Pop()

	b.Pop()
}

// drawQuadrantLabels draws the labels for each quadrant.
func (mc *Matrix2x2Chart) drawQuadrantLabels(plotArea Rect) {
	b := mc.builder
	style := b.StyleGuide()

	halfW := plotArea.W / 2
	halfH := plotArea.H / 2
	pad := style.Spacing.MD
	// Each quadrant label gets a bounded rectangle within its quadrant.
	// Labels are centered in each quadrant for clean consulting-style layout.
	rects := []Rect{
		{X: plotArea.X + pad, Y: plotArea.Y + pad, W: halfW - 2*pad, H: halfH - 2*pad},
		{X: plotArea.X + halfW + pad, Y: plotArea.Y + pad, W: halfW - 2*pad, H: halfH - 2*pad},
		{X: plotArea.X + pad, Y: plotArea.Y + halfH + pad, W: halfW - 2*pad, H: halfH - 2*pad},
		{X: plotArea.X + halfW + pad, Y: plotArea.Y + halfH + pad, W: halfW - 2*pad, H: halfH - 2*pad},
	}

	// Center labels in each quadrant for a clean consulting-style layout.
	aligns := []BoxAlign{
		AlignCenter, // top-left quadrant
		AlignCenter, // top-right quadrant
		AlignCenter, // bottom-left quadrant
		AlignCenter, // bottom-right quadrant
	}

	// Use LabelFitStrategy with wrapping to adapt heading size for narrow canvases.
	// In narrow placeholders (e.g., right column of two-column layouts),
	// quadrant label boxes can be very small. The strategy sizes for
	// multi-line wrapped text within the rect (both width and height),
	// so labels wrap naturally instead of being truncated with ellipsis.
	labelBoxW := halfW - 2*pad
	labelBoxH := halfH - 2*pad
	// Use a generous preferred size and a 10pt absolute floor so quadrant
	// labels remain readable even on small half-width canvases where the
	// style guide's scaled SizeBody can drop to ~7-8pt.
	preferredSize := math.Max(style.Typography.SizeHeading, 14)
	minLabelSize := math.Max(style.Typography.SizeBody, 10)
	quadrantFit := LabelFitStrategy{
		PreferredSize: preferredSize,
		MinSize:       minLabelSize,
		AllowWrap:     true,
		MaxLines:      4,
		MinCharWidth:  5.5,
	}
	// Find the longest quadrant label to determine font scaling.
	longestLabel := ""
	for _, label := range mc.config.QuadrantLabels {
		if len([]rune(label)) > len([]rune(longestLabel)) {
			longestLabel = label
		}
	}
	fontSize := style.Typography.SizeHeading
	if longestLabel != "" {
		result := quadrantFit.Fit(b, longestLabel, labelBoxW, labelBoxH)
		fontSize = result.FontSize
	}

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightBold)

	for i, label := range mc.config.QuadrantLabels {
		if label == "" {
			continue
		}
		b.DrawWrappedText(label, rects[i], aligns[i])
	}

	b.Pop()
}

// placedLabel tracks a rendered label's bounding box for collision avoidance.
type placedLabel struct {
	x, y, w, h float64
}

// overlaps checks whether two label bounding boxes overlap.
func (a placedLabel) overlaps(b placedLabel) bool {
	return a.x < b.x+b.w && a.x+a.w > b.x && a.y < b.y+b.h && a.y+a.h > b.y
}

// maxLabelChars is the maximum character length for a data-point label before
// it is truncated with an ellipsis. This prevents excessively long labels from
// dominating the chart and colliding with everything.
const maxLabelChars = 24

// drawPoints draws the data points.
func (mc *Matrix2x2Chart) drawPoints(points []Matrix2x2Point, plotArea Rect) {
	b := mc.builder
	style := b.StyleGuide()
	palette := style.Palette

	// Create scales
	xScale := NewLinearScale(mc.config.XAxisMin, mc.config.XAxisMax)
	xScale.SetRangeLinear(plotArea.X, plotArea.X+plotArea.W)

	yScale := NewLinearScale(mc.config.YAxisMin, mc.config.YAxisMax)
	yScale.SetRangeLinear(plotArea.Y+plotArea.H, plotArea.Y) // Inverted for screen coords

	// Adaptive font size: reduce label font when there are many points to
	// decrease collision pressure. Below 8 points use the normal SizeSmall;
	// from 8-15 points interpolate down to SizeCaption; above 15 stay at
	// SizeCaption.
	labelFontSize := style.Typography.SizeSmall
	n := len(points)
	if n >= 8 {
		minFont := style.Typography.SizeCaption
		if n >= 15 {
			labelFontSize = minFont
		} else {
			// Linear interpolation: 8 → SizeSmall, 15 → SizeCaption
			t := float64(n-8) / 7.0
			labelFontSize = style.Typography.SizeSmall - t*(style.Typography.SizeSmall-minFont)
		}
	}

	// Adaptive label offset: tighten when font shrinks so labels stay close
	// to their data points.
	labelOffset := mc.config.LabelOffset
	if labelFontSize < style.Typography.SizeSmall {
		labelOffset = labelOffset * (labelFontSize / style.Typography.SizeSmall)
	}

	// Collect placed label bounding boxes for collision avoidance.
	var placed []placedLabel

	for i, point := range points {
		x := xScale.Scale(point.X)
		y := yScale.Scale(point.Y)

		// Determine point properties
		size := point.Size
		if size == 0 {
			size = mc.config.PointSize
		}

		color := palette.AccentColor(i)
		if point.Color != nil {
			color = *point.Color
		}

		shape := mc.config.PointShape
		if point.Shape != MarkerNone {
			shape = point.Shape
		}

		// Draw the point
		mc.drawPoint(x, y, size, color, shape)

		// Draw label with collision avoidance
		if mc.config.ShowPointLabels && point.Label != "" {
			// Truncate long labels with ellipsis
			label := point.Label
			if len([]rune(label)) > maxLabelChars {
				label = string([]rune(label)[:maxLabelChars-1]) + "…"
			}
			lbl := mc.drawPointLabelAvoiding(x, y, size, label, plotArea, placed, labelFontSize, labelOffset)
			placed = append(placed, lbl)
		}
	}
}

// drawPoint draws a single data point.
func (mc *Matrix2x2Chart) drawPoint(x, y, size float64, color Color, shape MarkerShape) {
	b := mc.builder
	style := b.StyleGuide()
	radius := size / 2

	b.Push()
	b.SetFillColor(color)
	b.SetStrokeColor(MustParseColor("#FFFFFF"))
	b.SetStrokeWidth(style.Strokes.WidthNormal)

	switch shape {
	case MarkerCircle:
		b.DrawCircle(x, y, radius)

	case MarkerSquare:
		b.DrawRect(Rect{X: x - radius, Y: y - radius, W: size, H: size})

	case MarkerDiamond:
		pts := []Point{
			{X: x, Y: y - radius},
			{X: x + radius, Y: y},
			{X: x, Y: y + radius},
			{X: x - radius, Y: y},
		}
		b.DrawPolygon(pts)

	case MarkerTriangle:
		h := radius * math.Sqrt(3)
		pts := []Point{
			{X: x, Y: y - radius},
			{X: x + h/2, Y: y + radius/2},
			{X: x - h/2, Y: y + radius/2},
		}
		b.DrawPolygon(pts)

	default:
		b.DrawCircle(x, y, radius)
	}

	b.Pop()
}

// labelDirection describes a placement direction relative to a data point.
type labelDirection struct {
	dx, dy float64    // offset multipliers relative to the label offset distance
	align  TextAlign  // text alignment for this direction
}

// drawPointLabelAvoiding draws a label for a data point with collision avoidance.
// It returns the bounding box of the placed label so subsequent labels can avoid it.
//
// The algorithm tries four placement directions (right, left, below, above) before
// falling back to vertical shifting. This distributes labels around their points
// and dramatically reduces overlap when many points are clustered together.
//
// fontSize and offset are pre-computed by drawPoints based on the total number of
// data points so that dense charts get smaller, tighter labels.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (mc *Matrix2x2Chart) drawPointLabelAvoiding(x, y, pointSize float64, label string, plotArea Rect, placed []placedLabel, fontSize, offset float64) placedLabel {
	b := mc.builder
	style := b.StyleGuide()

	// Measure label dimensions using real font metrics for accurate collision
	// detection and bounds checking. The old heuristic (charCount * fontSize * 0.55)
	// underestimated wide labels, causing edge clipping.
	origSize := b.fontSize
	b.SetFontSize(fontSize)
	measuredW, _ := b.MeasureText(label)
	b.SetFontSize(origSize)
	labelW := measuredW
	labelH := fontSize * 1.3

	// LibreOffice renders SVG text ~15-20% wider than MeasureText predicts
	// (different font metrics / hinting). Use an inflated width for clipping
	// and wrapping decisions so labels near the SVG edge get wrapped instead
	// of being viewport-clipped mid-word (bug go-slide-creator-tfjmo).
	const libreOfficeTextInflation = 1.20
	clipW := labelW * libreOfficeTextInflation

	// Four candidate directions: right, left, below, above.
	// Each direction moves the label anchor by (dx*offset, dy*offset) from the
	// point center, with an appropriate text alignment.
	directions := []labelDirection{
		{dx: 1, dy: 0, align: TextAlignLeft},    // right of point
		{dx: -1, dy: 0, align: TextAlignRight},   // left of point
		{dx: 0, dy: 1, align: TextAlignCenter},    // below point
		{dx: 0, dy: -1, align: TextAlignCenter},   // above point
	}

	// Prefer to place the label away from the plot center so it doesn't
	// obscure the data area. Re-order directions accordingly.
	midX := plotArea.X + plotArea.W/2
	midY := plotArea.Y + plotArea.H/2
	if x > midX {
		// Point is in the right half — try right first (push outward)
		// default order is already right-first
	} else {
		// Point is in the left half — try left first
		directions[0], directions[1] = directions[1], directions[0]
	}
	if y > midY {
		// Point is in the bottom half — try below first, then above
		// default order is already below-first
	} else {
		// Point is in the top half — try above first
		directions[2], directions[3] = directions[3], directions[2]
	}

	// Helper: build a candidate bounding box for a given direction.
	buildCandidate := func(dir labelDirection) (placedLabel, float64, TextAlign) {
		lx := x + dir.dx*offset
		ly := y + dir.dy*offset

		bx := lx
		switch dir.align {
		case TextAlignRight:
			bx = lx - labelW
		case TextAlignCenter:
			bx = lx - labelW/2
		}
		return placedLabel{x: bx, y: ly - labelH/2, w: labelW, h: labelH}, lx, dir.align
	}

	collidesWith := func(c placedLabel) bool {
		for _, p := range placed {
			if c.overlaps(p) {
				return true
			}
		}
		return false
	}

	// Check whether a label at (lx, al) would be clipped by the SVG edge.
	// Uses clipW (inflated for LibreOffice rendering) instead of labelW
	// to prevent viewport clipping of labels near SVG edges.
	svgW := b.Width()
	wouldClip := func(lx float64, al TextAlign) bool {
		var availW float64
		switch al {
		case TextAlignLeft:
			availW = svgW - lx
		case TextAlignRight:
			availW = lx
		case TextAlignCenter:
			availW = math.Min(lx, svgW-lx) * 2
		}
		return availW > 0 && clipW > availW
	}

	// Try each direction without vertical shifting first.
	// Two-pass: first prefer directions that are collision-free AND unclipped,
	// then fall back to collision-free even if clipped (SVG edge truncation
	// is less harmful than label overlap). This prevents labels near the SVG
	// edge from being truncated when an opposite direction has ample space
	// (bug go-slide-creator-gy1j3: narrow two-column matrix labels).
	var bestCandidate placedLabel
	var bestLabelX float64
	var bestAlign TextAlign
	found := false

	// Pass 1: collision-free AND no clipping
	for _, dir := range directions {
		c, lx, al := buildCandidate(dir)
		if !collidesWith(c) && !wouldClip(lx, al) {
			bestCandidate = c
			bestLabelX = lx
			bestAlign = al
			found = true
			break
		}
	}

	// Pass 2: collision-free (even if clipped)
	if !found {
		for _, dir := range directions {
			c, lx, al := buildCandidate(dir)
			if !collidesWith(c) {
				bestCandidate = c
				bestLabelX = lx
				bestAlign = al
				found = true
				break
			}
		}
	}

	// If no clean direction found, fall back to the preferred direction
	// (first in list) and shift vertically until clear.
	if !found {
		c, lx, al := buildCandidate(directions[0])
		bestCandidate = c
		bestLabelX = lx
		bestAlign = al

		baseY := c.y
		step := labelH * 0.8
		for attempt := 0; attempt < 16; attempt++ {
			if !collidesWith(bestCandidate) {
				break
			}
			off := step * float64((attempt/2)+1)
			if attempt%2 == 0 {
				bestCandidate.y = baseY + off
			} else {
				bestCandidate.y = baseY - off
			}
		}
	}

	// Draw at the resolved position
	resolvedY := bestCandidate.y + labelH/2 // center Y of the label

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)

	// Constrain label to SVG bounds to prevent edge clipping.
	var availW float64
	switch bestAlign {
	case TextAlignLeft:
		availW = svgW - bestLabelX
	case TextAlignRight:
		availW = bestLabelX
	case TextAlignCenter:
		availW = math.Min(bestLabelX, svgW-bestLabelX) * 2
	}

	if availW > 0 && clipW > availW {
		// Wrap the label into a multi-line bounding box instead of truncating.
		// This preserves full item names in narrow two-column charts
		// (bug go-slide-creator-zci7z: matrix item labels truncated).
		// Uses clipW (inflated for LibreOffice) so labels near the edge wrap
		// proactively instead of being viewport-clipped (bug go-slide-creator-tfjmo).
		wrapH := labelH * 2 // allow up to 2 lines
		var wrapRect Rect
		var wrapAlign BoxAlign
		switch bestAlign {
		case TextAlignLeft:
			wrapRect = Rect{X: bestLabelX, Y: bestCandidate.y, W: availW, H: wrapH}
			wrapAlign = AlignTopLeft
		case TextAlignRight:
			wrapRect = Rect{X: bestLabelX - availW, Y: bestCandidate.y, W: availW, H: wrapH}
			wrapAlign = AlignTopRight
		default: // TextAlignCenter
			wrapRect = Rect{X: bestLabelX - availW/2, Y: bestCandidate.y, W: availW, H: wrapH}
			wrapAlign = AlignTopCenter
		}
		b.DrawWrappedText(label, wrapRect, wrapAlign)
		// Update collision bounding box to reflect wrapped height.
		bestCandidate.h = wrapH
	} else {
		b.DrawText(label, bestLabelX, resolvedY, bestAlign, TextBaselineMiddle)
	}

	b.Pop()

	return bestCandidate
}

// =============================================================================
// Matrix 2x2 Diagram Type (for Registry)
// =============================================================================

// Matrix2x2Diagram implements the Diagram interface for 2x2 matrix charts.
type Matrix2x2Diagram struct{ BaseDiagram }

// Validate checks that the request data is valid for matrix 2x2 charts.
func (d *Matrix2x2Diagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("matrix_2x2 chart requires data. Expected format: {\"x_axis_label\": \"Impact\", \"y_axis_label\": \"Effort\", \"points\": [{\"label\": \"Task A\", \"x\": 0.8, \"y\": 0.6}]}")
	}

	// Points are optional - can show empty matrix
	return nil
}

// Render generates an SVG document from the request envelope.
func (d *Matrix2x2Diagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
// This allows callers to generate PNG/PDF output from the same builder.
//nolint:gocognit,gocyclo // complex chart rendering logic
func (d *Matrix2x2Diagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseMatrix2x2Data(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultMatrix2x2Config(width, height)
		config.ShowPointLabels = true
		config.ShowGridLines = true

		// Override default hardcoded colors with template theme accent colors.
		style := builder.StyleGuide()
		accents := style.Palette.AccentColors()
		if len(accents) >= 4 {
			for i := 0; i < 4; i++ {
				config.QuadrantColors[i] = accents[i].WithAlpha(config.QuadrantOpacity)
			}
		}

		// Apply custom axis labels (support multiple key formats)
		if xLabel, ok := req.Data["x_axis_label"].(string); ok {
			config.XAxisLabel = xLabel
		} else if xLabel, ok := req.Data["x_label"].(string); ok {
			config.XAxisLabel = xLabel
		} else if xAxis, ok := req.Data["x_axis"].(map[string]any); ok {
			if label, ok := xAxis["label"].(string); ok {
				config.XAxisLabel = label
			}
		}
		if yLabel, ok := req.Data["y_axis_label"].(string); ok {
			config.YAxisLabel = yLabel
		} else if yLabel, ok := req.Data["y_label"].(string); ok {
			config.YAxisLabel = yLabel
		} else if yAxis, ok := req.Data["y_axis"].(map[string]any); ok {
			if label, ok := yAxis["label"].(string); ok {
				config.YAxisLabel = label
			}
		}

		// Apply custom quadrant labels from quadrant_labels array
		if labels, ok := req.Data["quadrant_labels"].([]any); ok {
			for i, l := range labels {
				if i < 4 {
					if label, ok := l.(string); ok {
						config.QuadrantLabels[i] = label
					}
				}
			}
		}

		// Apply quadrant labels from quadrants array (title or label field)
		if quadrants, ok := req.Data["quadrants"].([]any); ok {
			for _, q := range quadrants {
				qMap, ok := q.(map[string]any)
				if !ok {
					continue
				}
				position, _ := qMap["position"].(string)
				// Normalize position: support both hyphens and underscores
				position = strings.ReplaceAll(position, "_", "-")
				idx := quadrantPositionIndex(position)
				if idx < 0 {
					continue
				}
				// Prefer "title", fall back to "label"
				if title, ok := qMap["title"].(string); ok {
					config.QuadrantLabels[idx] = title
				} else if label, ok := qMap["label"].(string); ok {
					config.QuadrantLabels[idx] = label
				}
			}
		}

		// Apply custom quadrant colors
		if colors, ok := req.Data["quadrant_colors"].([]any); ok {
			for i, c := range colors {
				if i < 4 {
					if colorStr, ok := c.(string); ok {
						if color, err := ParseColor(colorStr); err == nil {
							config.QuadrantColors[i] = color.WithAlpha(config.QuadrantOpacity)
						}
					}
				}
			}
		}

		// Apply custom quadrant opacity
		if opacity, ok := req.Data["quadrant_opacity"].(float64); ok {
			config.QuadrantOpacity = opacity
			// Re-apply opacity to existing colors
			for i := range config.QuadrantColors {
				config.QuadrantColors[i] = config.QuadrantColors[i].WithAlpha(opacity)
			}
		}

		// Apply axis ranges
		if xMin, ok := req.Data["x_min"].(float64); ok {
			config.XAxisMin = xMin
		}
		if xMax, ok := req.Data["x_max"].(float64); ok {
			config.XAxisMax = xMax
		}
		if yMin, ok := req.Data["y_min"].(float64); ok {
			config.YAxisMin = yMin
		}
		if yMax, ok := req.Data["y_max"].(float64); ok {
			config.YAxisMax = yMax
		}

		chart := NewMatrix2x2Chart(builder, config)
		if err := chart.Draw(data); err != nil {
			return err
		}
		return nil
	})
}

// parseMatrix2x2Data parses the request data into Matrix2x2Data.
//nolint:gocognit,gocyclo // complex chart rendering logic
func parseMatrix2x2Data(req *RequestEnvelope) (Matrix2x2Data, error) {
	data := Matrix2x2Data{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Parse points
	if pointsRaw, ok := req.Data["points"].([]any); ok {
		data.Points = make([]Matrix2x2Point, 0, len(pointsRaw))

		for _, pRaw := range pointsRaw {
			point := Matrix2x2Point{}

			if p, ok := pRaw.(map[string]any); ok {
				if label, ok := p["label"].(string); ok {
					point.Label = label
				}
				if x, ok := p["x"].(float64); ok {
					point.X = x
				} else if x, ok := p["x"].(int); ok {
					point.X = float64(x)
				}
				if y, ok := p["y"].(float64); ok {
					point.Y = y
				} else if y, ok := p["y"].(int); ok {
					point.Y = float64(y)
				}
				if size, ok := p["size"].(float64); ok {
					point.Size = size
				} else if size, ok := p["size"].(int); ok {
					point.Size = float64(size)
				}
				if colorStr, ok := p["color"].(string); ok {
					if c, err := ParseColor(colorStr); err == nil {
						point.Color = &c
					}
				}
				if desc, ok := p["description"].(string); ok {
					point.Description = desc
				}
			}

			data.Points = append(data.Points, point)
		}
	}

	// Parse quadrants format (alternative to points)
	// This format uses quadrant position with items list
	if quadrants, ok := req.Data["quadrants"].([]any); ok && len(data.Points) == 0 {
		data.Points = parseQuadrantItems(quadrants)
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}

// quadrantPositionIndex returns the index (0-3) for a normalized quadrant position string.
// Returns -1 if the position is not recognized.
// Order: 0=top-left, 1=top-right, 2=bottom-left, 3=bottom-right.
func quadrantPositionIndex(position string) int {
	switch position {
	case "top-left":
		return 0
	case "top-right":
		return 1
	case "bottom-left":
		return 2
	case "bottom-right":
		return 3
	default:
		return -1
	}
}

// parseQuadrantItems converts quadrant-based format to points.
// Quadrant format: position (top-left, top-right, bottom-left, bottom-right), title, items
func parseQuadrantItems(quadrants []any) []Matrix2x2Point {
	points := make([]Matrix2x2Point, 0)

	// Define base positions for each quadrant.
	// Items are centered within their quadrant (center at 25/75 on each axis).
	quadrantPositions := map[string]struct{ x, y float64 }{
		"top-left":     {25, 75}, // High value, low effort
		"top-right":    {75, 75}, // High value, high effort
		"bottom-left":  {25, 25}, // Low value, low effort
		"bottom-right": {75, 25}, // Low value, high effort
	}

	for _, q := range quadrants {
		qMap, ok := q.(map[string]any)
		if !ok {
			continue
		}

		position, _ := qMap["position"].(string)
		// Normalize: support both hyphens and underscores
		position = strings.ReplaceAll(position, "_", "-")
		basePos, validPos := quadrantPositions[position]
		if !validPos {
			continue
		}

		// Get items for this quadrant
		items, ok := qMap["items"].([]any)
		if !ok || len(items) == 0 {
			continue
		}

		// Distribute items within the quadrant.
		// For few items (1-2), keep them close to the quadrant center so
		// the quadrant doesn't look empty. For more items, spread them out
		// to avoid overlap.
		itemCount := len(items)
		for i, item := range items {
			var label string
			switch v := item.(type) {
			case string:
				label = v
			case map[string]any:
				label, _ = v["label"].(string)
			}

			if label == "" {
				continue
			}

			// Vertical distribution: center the group of items in the quadrant.
			// Spacing between items scales with count to keep things compact for
			// sparse quadrants and spread out for dense ones.
			yOffset := 0.0
			if itemCount > 1 {
				// Use a spread range that grows with item count but stays moderate
				spreadRange := math.Min(float64(itemCount)*8.0, 20.0)
				yOffset = -spreadRange/2 + (spreadRange * float64(i) / float64(itemCount-1))
			}

			// Small x variation to avoid perfect vertical stacking
			xOffset := 0.0
			if itemCount > 2 {
				xOffset = float64((i % 3) - 1) * 6
			}

			// Use a slightly larger point size for sparse quadrants so the
			// single dot doesn't look lost.
			size := 0.0
			if itemCount <= 2 {
				size = 16 // larger marker for 1-2 items
			}

			point := Matrix2x2Point{
				Label: label,
				X:     basePos.x + xOffset,
				Y:     basePos.y + yOffset,
				Size:  size,
			}
			points = append(points, point)
		}
	}

	return points
}

// =============================================================================
// Convenience Functions
// =============================================================================

// DrawMatrix2x2FromData creates and draws a matrix 2x2 chart from simple data.
func DrawMatrix2x2FromData(builder *SVGBuilder, title string, points []Matrix2x2Point) error {
	config := DefaultMatrix2x2Config(builder.Width(), builder.Height())
	config.ShowPointLabels = true
	config.ShowGridLines = true

	chart := NewMatrix2x2Chart(builder, config)
	return chart.Draw(Matrix2x2Data{
		Title:  title,
		Points: points,
	})
}

// CreateBCGMatrixConfig returns a config preset for BCG Matrix (Growth-Share Matrix).
func CreateBCGMatrixConfig(width, height float64) Matrix2x2Config {
	config := DefaultMatrix2x2Config(width, height)
	config.XAxisLabel = "Relative Market Share"
	config.YAxisLabel = "Market Growth Rate"
	config.QuadrantLabels = [4]string{
		"Stars",          // high growth, high share
		"Question Marks", // high growth, low share
		"Cash Cows",      // low growth, high share
		"Dogs",           // low growth, low share
	}
	// For BCG, x-axis is reversed (high share on left)
	config.XAxisMin = 100
	config.XAxisMax = 0
	return config
}

// CreateEisenhowerMatrixConfig returns a config preset for Eisenhower Matrix (Urgent-Important).
func CreateEisenhowerMatrixConfig(width, height float64) Matrix2x2Config {
	config := DefaultMatrix2x2Config(width, height)
	config.XAxisLabel = "Urgency"
	config.YAxisLabel = "Importance"
	config.QuadrantLabels = [4]string{
		"Do First",  // important, not urgent
		"Schedule",  // important, urgent
		"Delegate",  // not important, not urgent
		"Eliminate", // not important, urgent
	}
	return config
}

