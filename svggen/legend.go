package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Title Component
// =============================================================================

// TitleConfig holds configuration for chart title rendering.
type TitleConfig struct {
	// Text is the title text.
	Text string

	// Subtitle is the optional subtitle text.
	Subtitle string

	// HorizontalAlign controls horizontal alignment: "left", "center", "right".
	HorizontalAlign string

	// VerticalAlign controls vertical alignment: "top", "bottom".
	VerticalAlign string

	// Padding around the title block.
	Padding float64

	// TitleStyle customizes title text appearance.
	TitleStyle *TextStyle

	// SubtitleStyle customizes subtitle text appearance.
	SubtitleStyle *TextStyle

	// Gap between title and subtitle.
	TitleSubtitleGap float64
}

// TextStyle defines text appearance options.
type TextStyle struct {
	// FontSize in points.
	FontSize float64

	// FontWeight (100-900).
	FontWeight int

	// Color is the text color.
	Color *Color
}

// DefaultTitleConfig returns default title configuration.
func DefaultTitleConfig() TitleConfig {
	return TitleConfig{
		HorizontalAlign:  "center",
		VerticalAlign:    "top",
		Padding:          8,
		TitleSubtitleGap: 4,
	}
}

// Title renders chart titles and subtitles.
type Title struct {
	builder *SVGBuilder
	config  TitleConfig
}

// NewTitle creates a new title renderer.
func NewTitle(builder *SVGBuilder, config TitleConfig) *Title {
	return &Title{
		builder: builder,
		config:  config,
	}
}

// Draw draws the title at the specified position within bounds.
func (t *Title) Draw(bounds Rect) *Title {
	if t.config.Text == "" {
		return t
	}

	b := t.builder
	style := b.StyleGuide()

	b.Push()

	// Calculate content area with padding
	contentBounds := bounds.Inset(t.config.Padding, t.config.Padding, t.config.Padding, t.config.Padding)

	// Determine title font settings
	titleFontSize := style.Typography.SizeTitle
	titleFontWeight := style.Typography.WeightBold
	titleColor := style.Palette.TextPrimary

	if t.config.TitleStyle != nil {
		if t.config.TitleStyle.FontSize > 0 {
			titleFontSize = t.config.TitleStyle.FontSize
		}
		if t.config.TitleStyle.FontWeight > 0 {
			titleFontWeight = t.config.TitleStyle.FontWeight
		}
		if t.config.TitleStyle.Color != nil {
			titleColor = *t.config.TitleStyle.Color
		}
	}

	// Determine subtitle font settings
	subtitleFontSize := style.Typography.SizeSubtitle
	subtitleFontWeight := style.Typography.WeightNormal
	subtitleColor := style.Palette.TextSecondary

	if t.config.SubtitleStyle != nil {
		if t.config.SubtitleStyle.FontSize > 0 {
			subtitleFontSize = t.config.SubtitleStyle.FontSize
		}
		if t.config.SubtitleStyle.FontWeight > 0 {
			subtitleFontWeight = t.config.SubtitleStyle.FontWeight
		}
		if t.config.SubtitleStyle.Color != nil {
			subtitleColor = *t.config.SubtitleStyle.Color
		}
	}

	// Calculate positions
	var textX float64
	var align TextAlign

	switch t.config.HorizontalAlign {
	case "left":
		textX = contentBounds.X
		align = TextAlignLeft
	case "right":
		textX = contentBounds.X + contentBounds.W
		align = TextAlignRight
	default: // center
		textX = contentBounds.X + contentBounds.W/2
		align = TextAlignCenter
	}

	// Calculate total height of title block
	totalHeight := titleFontSize
	if t.config.Subtitle != "" {
		totalHeight += t.config.TitleSubtitleGap + subtitleFontSize
	}

	// Calculate starting Y based on vertical alignment
	var textY float64
	switch t.config.VerticalAlign {
	case "bottom":
		textY = contentBounds.Y + contentBounds.H - totalHeight
	default: // top
		textY = contentBounds.Y
	}

	availW := contentBounds.W

	// Draw title
	b.SetTextColor(titleColor)
	b.SetFontWeight(titleFontWeight)
	titleFontSize = b.ClampFontSize(t.config.Text, availW, titleFontSize, 8)
	b.SetFontSize(titleFontSize)
	titleText := b.TruncateToWidth(t.config.Text, availW)
	b.DrawText(titleText, textX, textY+titleFontSize/2, align, TextBaselineMiddle)

	// Draw subtitle
	if t.config.Subtitle != "" {
		subtitleY := textY + titleFontSize + t.config.TitleSubtitleGap
		b.SetTextColor(subtitleColor)
		b.SetFontWeight(subtitleFontWeight)
		subtitleFontSize = b.ClampFontSize(t.config.Subtitle, availW, subtitleFontSize, 6)
		b.SetFontSize(subtitleFontSize)
		subtitleText := b.TruncateToWidth(t.config.Subtitle, availW)
		b.DrawText(subtitleText, textX, subtitleY+subtitleFontSize/2, align, TextBaselineMiddle)
	}

	b.Pop()
	return t
}

// Height returns the height needed for the title block.
func (t *Title) Height() float64 {
	if t.config.Text == "" {
		return 0
	}

	style := t.builder.StyleGuide()

	titleFontSize := style.Typography.SizeTitle
	if t.config.TitleStyle != nil && t.config.TitleStyle.FontSize > 0 {
		titleFontSize = t.config.TitleStyle.FontSize
	}

	height := titleFontSize + t.config.Padding*2

	if t.config.Subtitle != "" {
		subtitleFontSize := style.Typography.SizeSubtitle
		if t.config.SubtitleStyle != nil && t.config.SubtitleStyle.FontSize > 0 {
			subtitleFontSize = t.config.SubtitleStyle.FontSize
		}
		height += t.config.TitleSubtitleGap + subtitleFontSize
	}

	return height
}

// =============================================================================
// Footnote Component
// =============================================================================

// FootnoteConfig holds configuration for footnote rendering.
type FootnoteConfig struct {
	// Text is the footnote text.
	Text string

	// HorizontalAlign controls alignment: "left", "center", "right".
	HorizontalAlign string

	// Padding around the footnote.
	Padding float64

	// Style customizes footnote text appearance.
	Style *TextStyle
}

// DefaultFootnoteConfig returns default footnote configuration.
func DefaultFootnoteConfig() FootnoteConfig {
	return FootnoteConfig{
		HorizontalAlign: "left",
		Padding:         3,
	}
}

// FootnoteReservedHeight returns the vertical space needed for a single-line
// footnote at the current SizeCaption, including top/bottom padding.
// Use this when allocating footer height in chart/diagram layouts.
func FootnoteReservedHeight(style *StyleGuide) float64 {
	fontSize := style.Typography.SizeCaption
	padding := DefaultFootnoteConfig().Padding
	return fontSize + padding*2
}

// Footnote renders chart footnotes.
type Footnote struct {
	builder *SVGBuilder
	config  FootnoteConfig
}

// NewFootnote creates a new footnote renderer.
func NewFootnote(builder *SVGBuilder, config FootnoteConfig) *Footnote {
	return &Footnote{
		builder: builder,
		config:  config,
	}
}

// Draw draws the footnote at the specified position within bounds.
// Long footnotes are wrapped to fit within the available width.
func (f *Footnote) Draw(bounds Rect) *Footnote {
	if f.config.Text == "" {
		return f
	}

	b := f.builder
	style := b.StyleGuide()

	b.Push()

	// Calculate content area
	contentBounds := bounds.Inset(f.config.Padding, f.config.Padding, f.config.Padding, f.config.Padding)

	// Determine font settings
	fontSize := style.Typography.SizeCaption
	fontWeight := style.Typography.WeightNormal
	fontColor := style.Palette.TextMuted

	if f.config.Style != nil {
		if f.config.Style.FontSize > 0 {
			fontSize = f.config.Style.FontSize
		}
		if f.config.Style.FontWeight > 0 {
			fontWeight = f.config.Style.FontWeight
		}
		if f.config.Style.Color != nil {
			fontColor = *f.config.Style.Color
		}
	}

	// Map horizontal alignment to BoxAlign
	var boxAlign BoxAlign
	switch f.config.HorizontalAlign {
	case "center":
		boxAlign = AlignTopCenter
	case "right":
		boxAlign = AlignTopRight
	default: // left
		boxAlign = AlignTopLeft
	}

	// Draw footnote with text wrapping so long text doesn't overflow
	b.SetTextColor(fontColor)
	b.SetFontSize(fontSize).SetFontWeight(fontWeight)
	b.DrawWrappedText(f.config.Text, contentBounds, boxAlign)

	b.Pop()
	return f
}

// Height returns the height needed for the footnote (single-line estimate).
// For accurate height with text wrapping, use HeightForWidth.
func (f *Footnote) Height() float64 {
	if f.config.Text == "" {
		return 0
	}

	style := f.builder.StyleGuide()

	fontSize := style.Typography.SizeCaption
	if f.config.Style != nil && f.config.Style.FontSize > 0 {
		fontSize = f.config.Style.FontSize
	}

	return fontSize + f.config.Padding*2
}

// HeightForWidth returns the height needed for the footnote when the text
// is wrapped to fit within the given width. This accounts for multi-line
// wrapping of long footnotes.
func (f *Footnote) HeightForWidth(width float64) float64 {
	if f.config.Text == "" {
		return 0
	}

	b := f.builder
	style := b.StyleGuide()

	fontSize := style.Typography.SizeCaption
	fontWeight := style.Typography.WeightNormal
	if f.config.Style != nil {
		if f.config.Style.FontSize > 0 {
			fontSize = f.config.Style.FontSize
		}
		if f.config.Style.FontWeight > 0 {
			fontWeight = f.config.Style.FontWeight
		}
	}

	contentWidth := width - f.config.Padding*2
	if contentWidth <= 0 {
		return fontSize + f.config.Padding*2
	}

	// Temporarily set font to measure wrapped text
	b.Push()
	b.SetFontSize(fontSize).SetFontWeight(fontWeight)
	block := b.WrapText(f.config.Text, contentWidth)
	b.Pop()

	if len(block.Lines) == 0 {
		return fontSize + f.config.Padding*2
	}

	return block.TotalHeight + f.config.Padding*2
}

// =============================================================================
// Legend Component
// =============================================================================

// LegendPosition specifies where the legend is placed.
type LegendPosition int

const (
	LegendPositionTop LegendPosition = iota
	LegendPositionBottom
	LegendPositionLeft
	LegendPositionRight
)

// LegendLayout specifies how legend items are arranged.
type LegendLayout int

const (
	LegendLayoutHorizontal LegendLayout = iota
	LegendLayoutVertical
	LegendLayoutGrid
)

// LegendMarkerShape specifies the legend marker shape.
type LegendMarkerShape int

const (
	LegendMarkerRect LegendMarkerShape = iota
	LegendMarkerCircle
	LegendMarkerLine
)

// LegendConfig holds configuration for legend rendering.
type LegendConfig struct {
	// Position determines where the legend is placed.
	Position LegendPosition

	// Layout determines how items are arranged.
	Layout LegendLayout

	// HorizontalAlign within the legend area: "left", "center", "right".
	HorizontalAlign string

	// VerticalAlign within the legend area: "top", "middle", "bottom".
	VerticalAlign string

	// Padding around the legend.
	Padding float64

	// ItemGap is the gap between legend items.
	ItemGap float64

	// RowGap is the gap between rows in grid/wrapped layout.
	RowGap float64

	// MarkerShape is the shape of color markers.
	MarkerShape LegendMarkerShape

	// MarkerSize is the size of color markers.
	MarkerSize float64

	// MarkerLabelGap is the gap between marker and label.
	MarkerLabelGap float64

	// MaxWidth for horizontal layout (enables wrapping).
	MaxWidth float64

	// MaxHeight for vertical layout (enables scrolling indicator).
	MaxHeight float64

	// Style customizes legend text appearance.
	Style *TextStyle

	// ShowBorder draws a border around the legend.
	ShowBorder bool

	// BorderColor is the border color.
	BorderColor Color

	// BackgroundColor is the legend background color.
	BackgroundColor *Color

	// TextWidthFactor is a multiplier applied to measured text widths to account
	// for rendering engines (e.g. LibreOffice) that display text wider than the
	// canvas library's font metrics predict. Default 1.0 (no adjustment).
	// PresentationLegendConfig sets this to 1.15 for robustness.
	TextWidthFactor float64
}

// DefaultLegendConfig returns default legend configuration.
func DefaultLegendConfig() LegendConfig {
	return LegendConfig{
		Position:        LegendPositionBottom,
		Layout:          LegendLayoutHorizontal,
		HorizontalAlign: "center",
		VerticalAlign:   "top",
		Padding:         8,
		ItemGap:         16,
		RowGap:          6,
		MarkerShape:     LegendMarkerRect,
		MarkerSize:      12,
		MarkerLabelGap:  6,
		MaxWidth:        0,
		MaxHeight:       0,
		ShowBorder:      false,
		BorderColor:     MustParseColor("#DEE2E6"),
		// The Go canvas library's MeasureText underestimates text widths by
		// ~2.5x relative to SVG renderers (rsvg-convert, LibreOffice) due to
		// internal font-size scaling in the canvas library. This factor
		// compensates so legend items are spaced correctly in rendered output.
		TextWidthFactor: 3.0,
	}
}

// PresentationLegendConfig returns legend configuration optimized for presentation slides.
// Uses larger fonts (20pt) and markers (16px) for better readability in slide contexts.
func PresentationLegendConfig(style *StyleGuide) LegendConfig {
	config := DefaultLegendConfig()
	config.Style = &TextStyle{
		FontSize:   style.Typography.SizeHeading,
		FontWeight: style.Typography.WeightMedium,
		Color:      &style.Palette.TextPrimary,
	}
	config.MarkerSize = 16
	config.MarkerLabelGap = 8
	config.ItemGap = 24
	return config
}

// PresentationPieLegendConfig returns legend configuration for pie/donut charts.
// Uses Body-sized text and grid layout so legend items are readable at projection
// distance. The grid layout keeps the legend compact so the pie occupies the
// majority of the chart area.
func PresentationPieLegendConfig(style *StyleGuide) LegendConfig {
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutGrid
	config.HorizontalAlign = "center"
	config.Style = &TextStyle{
		FontSize:   style.Typography.SizeBody,
		FontWeight: style.Typography.WeightNormal,
		Color:      &style.Palette.TextPrimary,
	}
	config.MarkerSize = 14
	config.MarkerLabelGap = 6
	config.ItemGap = 20
	config.RowGap = 4
	config.Padding = 4
	return config
}

// LegendItem represents a single legend entry.
type LegendItem struct {
	// Label is the legend item label.
	Label string

	// Color is the color for this item.
	Color Color

	// Inactive dims the item when true.
	Inactive bool

	// Value is an optional value to display.
	Value string
}

// Legend renders chart legends.
type Legend struct {
	builder *SVGBuilder
	config  LegendConfig
	items   []LegendItem
}

// NewLegend creates a new legend renderer.
func NewLegend(builder *SVGBuilder, config LegendConfig) *Legend {
	return &Legend{
		builder: builder,
		config:  config,
		items:   nil,
	}
}

// SetItems sets the legend items.
func (l *Legend) SetItems(items []LegendItem) *Legend {
	l.items = items
	return l
}

// AddItem adds a legend item.
func (l *Legend) AddItem(label string, color Color) *Legend {
	l.items = append(l.items, LegendItem{Label: label, Color: color})
	return l
}

// Draw draws the legend within the specified bounds.
func (l *Legend) Draw(bounds Rect) *Legend {
	if len(l.items) == 0 {
		return l
	}

	b := l.builder
	style := b.StyleGuide()

	b.Push()

	// Calculate content area
	contentBounds := bounds.Inset(l.config.Padding, l.config.Padding, l.config.Padding, l.config.Padding)

	// Draw background if specified
	if l.config.BackgroundColor != nil {
		b.SetFillColor(*l.config.BackgroundColor)
		b.FillRect(bounds)
	}

	// Draw border if enabled
	if l.config.ShowBorder {
		b.SetStrokeColor(l.config.BorderColor)
		b.SetStrokeWidth(style.Strokes.WidthThin)
		b.StrokeRect(bounds)
	}

	// Determine font settings
	fontSize := style.Typography.SizeSmall
	fontWeight := style.Typography.WeightNormal
	fontColor := style.Palette.TextSecondary

	if l.config.Style != nil {
		if l.config.Style.FontSize > 0 {
			fontSize = l.config.Style.FontSize
		}
		if l.config.Style.FontWeight > 0 {
			fontWeight = l.config.Style.FontWeight
		}
		if l.config.Style.Color != nil {
			fontColor = *l.config.Style.Color
		}
	}

	b.SetFontSize(fontSize).SetFontWeight(fontWeight)

	// Calculate item dimensions with optional text width safety factor.
	// The factor accounts for rendering engines (LibreOffice) that display
	// SVG text wider than the canvas library's font metrics predict.
	twf := l.config.TextWidthFactor
	if twf <= 0 {
		twf = 1.0
	}
	itemWidths := make([]float64, len(l.items))
	for i, item := range l.items {
		textWidth, _ := b.MeasureText(item.Label)
		textWidth *= twf
		if item.Value != "" {
			valueWidth, _ := b.MeasureText(item.Value)
			textWidth += style.Spacing.XS + valueWidth*twf
		}
		itemWidths[i] = l.config.MarkerSize + l.config.MarkerLabelGap + textWidth
	}

	// Draw items based on layout
	switch l.config.Layout {
	case LegendLayoutHorizontal:
		l.drawHorizontal(contentBounds, itemWidths, fontSize, fontColor)
	case LegendLayoutVertical:
		l.drawVertical(contentBounds, itemWidths, fontSize, fontColor)
	case LegendLayoutGrid:
		l.drawGrid(contentBounds, itemWidths, fontSize, fontColor)
	}

	b.Pop()
	return l
}

// drawHorizontal draws items in a horizontal line with optional wrapping.
func (l *Legend) drawHorizontal(bounds Rect, itemWidths []float64, fontSize float64, fontColor Color) {
	b := l.builder
	style := b.StyleGuide()

	// Calculate total width
	totalWidth := 0.0
	for _, w := range itemWidths {
		totalWidth += w
	}
	totalWidth += float64(len(l.items)-1) * l.config.ItemGap

	// Determine if wrapping is needed
	maxWidth := bounds.W
	if l.config.MaxWidth > 0 && l.config.MaxWidth < maxWidth {
		maxWidth = l.config.MaxWidth
	}

	needsWrap := totalWidth > maxWidth

	// Calculate starting position based on alignment
	var startX float64
	switch l.config.HorizontalAlign {
	case "left":
		startX = bounds.X
	case "right":
		if needsWrap {
			startX = bounds.X
		} else {
			startX = bounds.X + bounds.W - totalWidth
		}
	default: // center
		if needsWrap {
			startX = bounds.X
		} else {
			startX = bounds.X + (bounds.W-totalWidth)/2
		}
	}

	currentX := startX
	currentY := bounds.Y
	rowHeight := math.Max(l.config.MarkerSize, fontSize)

	for i, item := range l.items {
		// Check if wrapping is needed
		if needsWrap && currentX > startX && currentX+itemWidths[i] > bounds.X+maxWidth {
			currentX = startX
			currentY += rowHeight + l.config.RowGap
		}

		l.drawItem(currentX, currentY, item, fontSize, fontColor)
		currentX += itemWidths[i] + l.config.ItemGap
	}

	// Store calculated dimensions
	_ = style
}

// drawVertical draws items in a vertical column.
func (l *Legend) drawVertical(bounds Rect, itemWidths []float64, fontSize float64, fontColor Color) {
	rowHeight := math.Max(l.config.MarkerSize, fontSize)

	// Calculate total height
	totalHeight := float64(len(l.items))*rowHeight + float64(len(l.items)-1)*l.config.RowGap

	// Calculate starting Y based on vertical alignment
	var startY float64
	switch l.config.VerticalAlign {
	case "bottom":
		startY = bounds.Y + bounds.H - totalHeight
	case "middle":
		startY = bounds.Y + (bounds.H-totalHeight)/2
	default: // top
		startY = bounds.Y
	}

	// Calculate starting X based on horizontal alignment
	maxItemWidth := 0.0
	for _, w := range itemWidths {
		if w > maxItemWidth {
			maxItemWidth = w
		}
	}

	var startX float64
	switch l.config.HorizontalAlign {
	case "right":
		startX = bounds.X + bounds.W - maxItemWidth
	case "center":
		startX = bounds.X + (bounds.W-maxItemWidth)/2
	default: // left
		startX = bounds.X
	}

	maxY := bounds.Y + bounds.H
	currentY := startY
	drawn := 0
	for _, item := range l.items {
		// Skip items that would overflow the bounds.
		if currentY+rowHeight > maxY+0.5 {
			break
		}
		l.drawItem(startX, currentY, item, fontSize, fontColor)
		currentY += rowHeight + l.config.RowGap
		drawn++
	}

	if dropped := len(l.items) - drawn; dropped > 0 {
		l.builder.AddFinding(Finding{
			Code:     FindingLegendOverflowDropped,
			Message:  fmt.Sprintf("legend overflow — %d of %d items dropped (insufficient height)", dropped, len(l.items)),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"total": len(l.items), "drawn": drawn, "dropped": dropped},
			},
		})
	}
}

// drawGrid draws items in a grid layout.
func (l *Legend) drawGrid(bounds Rect, itemWidths []float64, fontSize float64, fontColor Color) {
	if len(l.items) == 0 {
		return
	}

	// Find the widest item
	maxItemWidth := 0.0
	for _, w := range itemWidths {
		if w > maxItemWidth {
			maxItemWidth = w
		}
	}

	// Calculate columns that fit
	availableWidth := bounds.W
	colWidth := maxItemWidth + l.config.ItemGap
	numCols := int(availableWidth / colWidth)
	if numCols < 1 {
		numCols = 1
	}

	rowHeight := math.Max(l.config.MarkerSize, fontSize)
	numRows := (len(l.items) + numCols - 1) / numCols

	// Calculate total dimensions
	totalWidth := float64(numCols)*colWidth - l.config.ItemGap
	totalHeight := float64(numRows)*rowHeight + float64(numRows-1)*l.config.RowGap

	// Calculate starting position based on alignment
	var startX float64
	switch l.config.HorizontalAlign {
	case "right":
		startX = bounds.X + bounds.W - totalWidth
	case "center":
		startX = bounds.X + (bounds.W-totalWidth)/2
	default: // left
		startX = bounds.X
	}

	var startY float64
	switch l.config.VerticalAlign {
	case "bottom":
		startY = bounds.Y + bounds.H - totalHeight
	case "middle":
		startY = bounds.Y + (bounds.H-totalHeight)/2
	default: // top
		startY = bounds.Y
	}

	// Draw items in grid, skipping any that would overflow the bounds.
	// This prevents partial/clipped legend items when the legend height
	// is capped (e.g. for pie/donut charts with many categories).
	maxY := bounds.Y + bounds.H
	drawn := 0
	for i, item := range l.items {
		col := i % numCols
		row := i / numCols

		x := startX + float64(col)*colWidth
		y := startY + float64(row)*(rowHeight+l.config.RowGap)

		// Skip items whose row bottom would extend past the available height.
		if y+rowHeight > maxY+0.5 { // 0.5 tolerance for floating point
			break
		}

		l.drawItem(x, y, item, fontSize, fontColor)
		drawn++
	}

	if dropped := len(l.items) - drawn; dropped > 0 {
		l.builder.AddFinding(Finding{
			Code:     FindingLegendOverflowDropped,
			Message:  fmt.Sprintf("legend overflow — %d of %d items dropped (insufficient height)", dropped, len(l.items)),
			Severity: "warning",
			Fix: &FixSuggestion{
				Kind:   FixKindReduceItems,
				Params: map[string]any{"total": len(l.items), "drawn": drawn, "dropped": dropped},
			},
		})
	}
}

// drawItem draws a single legend item.
func (l *Legend) drawItem(x, y float64, item LegendItem, fontSize float64, fontColor Color) {
	b := l.builder
	style := b.StyleGuide()

	markerY := y + (math.Max(l.config.MarkerSize, fontSize)-l.config.MarkerSize)/2

	// Determine item color
	color := item.Color
	textColor := fontColor
	if item.Inactive {
		color = color.WithAlpha(0.3)
		textColor = textColor.WithAlpha(0.5)
	}

	// Draw marker
	b.SetFillColor(color)
	switch l.config.MarkerShape {
	case LegendMarkerCircle:
		radius := l.config.MarkerSize / 2
		b.DrawCircle(x+radius, markerY+radius, radius)

	case LegendMarkerLine:
		lineY := markerY + l.config.MarkerSize/2
		b.SetStrokeColor(color)
		b.SetStrokeWidth(style.Strokes.WidthMedium)
		b.DrawLine(x, lineY, x+l.config.MarkerSize, lineY)

	default: // LegendMarkerRect
		b.FillRect(Rect{
			X: x,
			Y: markerY,
			W: l.config.MarkerSize,
			H: l.config.MarkerSize,
		})
	}

	// Draw label
	labelX := x + l.config.MarkerSize + l.config.MarkerLabelGap
	labelY := y + math.Max(l.config.MarkerSize, fontSize)/2
	b.SetTextColor(textColor)
	b.DrawText(item.Label, labelX, labelY, TextAlignLeft, TextBaselineMiddle)

	// Draw value if present
	if item.Value != "" {
		textWidth, _ := b.MeasureText(item.Label)
		valueX := labelX + textWidth + style.Spacing.XS
		b.DrawText(item.Value, valueX, labelY, TextAlignLeft, TextBaselineMiddle)
	}
}

// Height returns the height needed for the legend.
func (l *Legend) Height(maxWidth float64) float64 {
	if len(l.items) == 0 {
		return 0
	}

	b := l.builder
	style := b.StyleGuide()

	fontSize := style.Typography.SizeSmall
	if l.config.Style != nil && l.config.Style.FontSize > 0 {
		fontSize = l.config.Style.FontSize
	}

	b.SetFontSize(fontSize)

	twf := l.config.TextWidthFactor
	if twf <= 0 {
		twf = 1.0
	}

	rowHeight := math.Max(l.config.MarkerSize, fontSize)

	switch l.config.Layout {
	case LegendLayoutVertical:
		numRows := len(l.items)
		return float64(numRows)*rowHeight + float64(numRows-1)*l.config.RowGap + l.config.Padding*2

	case LegendLayoutGrid:
		// Calculate columns that fit
		maxItemWidth := 0.0
		for _, item := range l.items {
			textWidth, _ := b.MeasureText(item.Label)
			itemWidth := l.config.MarkerSize + l.config.MarkerLabelGap + textWidth*twf
			if itemWidth > maxItemWidth {
				maxItemWidth = itemWidth
			}
		}

		colWidth := maxItemWidth + l.config.ItemGap
		numCols := int((maxWidth - l.config.Padding*2) / colWidth)
		if numCols < 1 {
			numCols = 1
		}

		numRows := (len(l.items) + numCols - 1) / numCols
		return float64(numRows)*rowHeight + float64(numRows-1)*l.config.RowGap + l.config.Padding*2

	default: // LegendLayoutHorizontal
		// Calculate if wrapping is needed
		totalWidth := 0.0
		for _, item := range l.items {
			textWidth, _ := b.MeasureText(item.Label)
			totalWidth += l.config.MarkerSize + l.config.MarkerLabelGap + textWidth*twf
		}
		totalWidth += float64(len(l.items)-1) * l.config.ItemGap

		availableWidth := maxWidth - l.config.Padding*2
		if totalWidth <= availableWidth {
			return rowHeight + l.config.Padding*2
		}

		// Estimate rows needed
		numRows := int(math.Ceil(totalWidth / availableWidth))
		return float64(numRows)*rowHeight + float64(numRows-1)*l.config.RowGap + l.config.Padding*2
	}
}

// Width returns the width needed for the legend.
func (l *Legend) Width() float64 {
	if len(l.items) == 0 {
		return 0
	}

	b := l.builder
	style := b.StyleGuide()

	fontSize := style.Typography.SizeSmall
	if l.config.Style != nil && l.config.Style.FontSize > 0 {
		fontSize = l.config.Style.FontSize
	}

	b.SetFontSize(fontSize)

	twf := l.config.TextWidthFactor
	if twf <= 0 {
		twf = 1.0
	}

	switch l.config.Layout {
	case LegendLayoutVertical:
		// Find widest item
		maxWidth := 0.0
		for _, item := range l.items {
			textWidth, _ := b.MeasureText(item.Label)
			itemWidth := l.config.MarkerSize + l.config.MarkerLabelGap + textWidth*twf
			if itemWidth > maxWidth {
				maxWidth = itemWidth
			}
		}
		return maxWidth + l.config.Padding*2

	default: // Horizontal or Grid
		// Total width of all items
		totalWidth := 0.0
		for _, item := range l.items {
			textWidth, _ := b.MeasureText(item.Label)
			totalWidth += l.config.MarkerSize + l.config.MarkerLabelGap + textWidth*twf
		}
		totalWidth += float64(len(l.items)-1) * l.config.ItemGap
		return totalWidth + l.config.Padding*2
	}
}

// =============================================================================
// Chart Header Component (combines Title + Legend)
// =============================================================================

// ChartHeaderConfig holds configuration for chart header block.
type ChartHeaderConfig struct {
	// Title configuration.
	Title TitleConfig

	// Legend configuration.
	Legend LegendConfig

	// LegendPosition relative to title: "below", "right".
	LegendPosition string

	// Gap between title and legend.
	TitleLegendGap float64

	// Padding around the entire header.
	Padding float64
}

// DefaultChartHeaderConfig returns default chart header configuration.
func DefaultChartHeaderConfig() ChartHeaderConfig {
	return ChartHeaderConfig{
		Title:          DefaultTitleConfig(),
		Legend:         DefaultLegendConfig(),
		LegendPosition: "below",
		TitleLegendGap: 12,
		Padding:        8,
	}
}

// ChartHeader renders a complete chart header with title and legend.
type ChartHeader struct {
	builder *SVGBuilder
	config  ChartHeaderConfig
	items   []LegendItem
}

// NewChartHeader creates a new chart header renderer.
func NewChartHeader(builder *SVGBuilder, config ChartHeaderConfig) *ChartHeader {
	return &ChartHeader{
		builder: builder,
		config:  config,
	}
}

// SetTitle sets the title text.
func (ch *ChartHeader) SetTitle(text string) *ChartHeader {
	ch.config.Title.Text = text
	return ch
}

// SetSubtitle sets the subtitle text.
func (ch *ChartHeader) SetSubtitle(text string) *ChartHeader {
	ch.config.Title.Subtitle = text
	return ch
}

// SetLegendItems sets the legend items.
func (ch *ChartHeader) SetLegendItems(items []LegendItem) *ChartHeader {
	ch.items = items
	return ch
}

// AddLegendItem adds a legend item.
func (ch *ChartHeader) AddLegendItem(label string, color Color) *ChartHeader {
	ch.items = append(ch.items, LegendItem{Label: label, Color: color})
	return ch
}

// Draw draws the chart header within the specified bounds.
func (ch *ChartHeader) Draw(bounds Rect) *ChartHeader {
	contentBounds := bounds.Inset(ch.config.Padding, ch.config.Padding, ch.config.Padding, ch.config.Padding)

	hasTitle := ch.config.Title.Text != ""
	hasLegend := len(ch.items) > 0

	if !hasTitle && !hasLegend {
		return ch
	}

	if ch.config.LegendPosition == "right" {
		// Side-by-side layout
		if hasTitle {
			title := NewTitle(ch.builder, ch.config.Title)
			titleHeight := title.Height()
			titleBounds := Rect{
				X: contentBounds.X,
				Y: contentBounds.Y,
				W: contentBounds.W * 0.6,
				H: titleHeight,
			}
			title.Draw(titleBounds)

			if hasLegend {
				legend := NewLegend(ch.builder, ch.config.Legend)
				legend.SetItems(ch.items)
				legendBounds := Rect{
					X: contentBounds.X + contentBounds.W*0.6 + ch.config.TitleLegendGap,
					Y: contentBounds.Y,
					W: contentBounds.W*0.4 - ch.config.TitleLegendGap,
					H: titleHeight,
				}
				legend.Draw(legendBounds)
			}
		} else if hasLegend {
			legend := NewLegend(ch.builder, ch.config.Legend)
			legend.SetItems(ch.items)
			legend.Draw(contentBounds)
		}
	} else {
		// Stacked layout (default: "below")
		currentY := contentBounds.Y

		if hasTitle {
			title := NewTitle(ch.builder, ch.config.Title)
			titleHeight := title.Height()
			titleBounds := Rect{
				X: contentBounds.X,
				Y: currentY,
				W: contentBounds.W,
				H: titleHeight,
			}
			title.Draw(titleBounds)
			currentY += titleHeight + ch.config.TitleLegendGap
		}

		if hasLegend {
			legend := NewLegend(ch.builder, ch.config.Legend)
			legend.SetItems(ch.items)
			legendHeight := legend.Height(contentBounds.W)
			legendBounds := Rect{
				X: contentBounds.X,
				Y: currentY,
				W: contentBounds.W,
				H: legendHeight,
			}
			legend.Draw(legendBounds)
		}
	}

	return ch
}

// Height returns the total height needed for the chart header.
func (ch *ChartHeader) Height(maxWidth float64) float64 {
	contentWidth := maxWidth - ch.config.Padding*2

	hasTitle := ch.config.Title.Text != ""
	hasLegend := len(ch.items) > 0

	if !hasTitle && !hasLegend {
		return 0
	}

	var height float64

	if ch.config.LegendPosition == "right" {
		// Side-by-side: height is max of title and legend
		if hasTitle {
			title := NewTitle(ch.builder, ch.config.Title)
			height = title.Height()
		}
		if hasLegend {
			legend := NewLegend(ch.builder, ch.config.Legend)
			legend.SetItems(ch.items)
			legendHeight := legend.Height(contentWidth * 0.4)
			if legendHeight > height {
				height = legendHeight
			}
		}
	} else {
		// Stacked: height is sum
		if hasTitle {
			title := NewTitle(ch.builder, ch.config.Title)
			height += title.Height()
		}
		if hasLegend {
			if hasTitle {
				height += ch.config.TitleLegendGap
			}
			legend := NewLegend(ch.builder, ch.config.Legend)
			legend.SetItems(ch.items)
			height += legend.Height(contentWidth)
		}
	}

	return height + ch.config.Padding*2
}
