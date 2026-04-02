package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Venn Diagram
// =============================================================================

// VennConfig holds configuration for Venn diagrams.
type VennConfig struct {
	ChartConfig

	// CircleOpacity is the fill opacity for circle backgrounds.
	CircleOpacity float64

	// StrokeWidth is the line width for circle outlines.
	StrokeWidth float64

	// OverlapRatio controls how much circles overlap (0 = touching, 1 = concentric).
	// Typical values: 0.3-0.5
	OverlapRatio float64
}

// DefaultVennConfig returns default Venn configuration.
func DefaultVennConfig(width, height float64) VennConfig {
	return VennConfig{
		ChartConfig:   DefaultChartConfig(width, height),
		CircleOpacity: 0.25,
		StrokeWidth:   2.0,
		OverlapRatio:  0.20,
	}
}

// VennCircle represents a single circle in the Venn diagram.
type VennCircle struct {
	// Label is the circle name displayed inside it.
	Label string

	// Items are optional items listed exclusively in this circle.
	Items []string
}

// VennRegion represents a labeled intersection region.
type VennRegion struct {
	// Label is the text for this intersection.
	Label string

	// Items are optional items for this region.
	Items []string
}

// VennData represents the data for a Venn diagram.
type VennData struct {
	// Title is the diagram title.
	Title string

	// Subtitle is the diagram subtitle.
	Subtitle string

	// Circles are the 2 or 3 circles.
	Circles []VennCircle

	// Intersections maps region keys to region data.
	// For 2 circles: "ab" = intersection of circle 0 and 1.
	// For 3 circles: "ab", "ac", "bc", "abc".
	Intersections map[string]VennRegion

	// Footnote is an optional footnote text.
	Footnote string
}

// VennChart renders Venn diagrams.
type VennChart struct {
	builder *SVGBuilder
	config  VennConfig
}

// NewVennChart creates a new Venn chart renderer.
func NewVennChart(builder *SVGBuilder, config VennConfig) *VennChart {
	return &VennChart{
		builder: builder,
		config:  config,
	}
}

// circleLayout holds computed position and radius for a circle.
type circleLayout struct {
	cx, cy float64
	radius float64
	color  Color
}

// Draw renders the Venn diagram.
func (vc *VennChart) Draw(data VennData) error {
	// Cap at 3 circles gracefully — use first 3 if more provided.
	if len(data.Circles) > 3 {
		data.Circles = data.Circles[:3]
	}
	numCircles := len(data.Circles)
	if numCircles < 2 {
		return fmt.Errorf("venn diagram requires at least 2 circles, got %d", numCircles)
	}

	b := vc.builder
	style := b.StyleGuide()

	plotArea := vc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if vc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}

	// Adjust for footnote
	footerHeight := 0.0
	if data.Footnote != "" {
		footerHeight = FootnoteReservedHeight(style)
	}

	plotArea.Y += headerHeight
	plotArea.H -= headerHeight + footerHeight

	colors := vc.getColors(style, numCircles)

	// Compute circle layouts
	var layouts []circleLayout
	if numCircles == 2 {
		layouts = vc.layout2Circles(plotArea, colors)
	} else {
		layouts = vc.layout3Circles(plotArea, colors)
	}

	// Draw circles (background fill)
	for _, cl := range layouts {
		vc.drawCircleFill(cl)
	}

	// Draw circle outlines on top
	for _, cl := range layouts {
		vc.drawCircleStroke(cl)
	}

	// Draw circle labels (exclusive region text)
	if numCircles == 2 {
		vc.drawLabels2(data, layouts)
	} else {
		vc.drawLabels3(data, layouts)
	}

	// Draw intersection labels
	if numCircles == 2 {
		vc.drawIntersection2(data, layouts)
	} else {
		vc.drawIntersections3(data, layouts)
	}

	// Draw title
	if vc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: vc.config.Width, H: headerHeight + vc.config.MarginTop})
	}

	// Draw footnote
	if data.Footnote != "" {
		footnoteConfig := DefaultFootnoteConfig()
		footnoteConfig.Text = data.Footnote
		footnote := NewFootnote(b, footnoteConfig)
		footnote.Draw(Rect{
			X: 0,
			Y: vc.config.Height - footerHeight,
			W: vc.config.Width,
			H: footerHeight,
		})
	}

	return nil
}

// layout2Circles computes positions for a 2-circle Venn diagram.
func (vc *VennChart) layout2Circles(area Rect, colors []Color) []circleLayout {
	// Two circles side by side with overlap.
	// Use most of the available area so labels have room.
	centerY := area.Y + area.H/2
	radius := math.Min(area.W/2.8, area.H/2.1)
	offset := radius * (1.0 - vc.config.OverlapRatio)
	centerX := area.X + area.W/2

	return []circleLayout{
		{cx: centerX - offset, cy: centerY, radius: radius, color: colors[0]},
		{cx: centerX + offset, cy: centerY, radius: radius, color: colors[1]},
	}
}

// layout3Circles computes positions for a 3-circle Venn diagram.
func (vc *VennChart) layout3Circles(area Rect, colors []Color) []circleLayout {
	// Three circles in a triangular arrangement
	centerX := area.X + area.W/2
	centerY := area.Y + area.H/2
	radius := math.Min(area.W/3.2, area.H/3.0)
	offset := radius * (1.0 - vc.config.OverlapRatio)

	// Equilateral triangle arrangement:
	// Top circle, bottom-left, bottom-right.
	// Place centroid at the plot area center so the "abc" label sits at the
	// visual center.  The triangle is top-heavy (1 circle above vs 2 below),
	// but the title/header fills the space above the circles.
	triCenterY := centerY

	return []circleLayout{
		{cx: centerX, cy: triCenterY - offset, radius: radius, color: colors[0]},                                           // top
		{cx: centerX - offset*math.Cos(math.Pi/6), cy: triCenterY + offset*math.Sin(math.Pi/6), radius: radius, color: colors[1]}, // bottom-left
		{cx: centerX + offset*math.Cos(math.Pi/6), cy: triCenterY + offset*math.Sin(math.Pi/6), radius: radius, color: colors[2]}, // bottom-right
	}
}

// drawCircleFill draws the filled background of a circle.
func (vc *VennChart) drawCircleFill(cl circleLayout) {
	b := vc.builder
	b.Push()
	b.SetFillColor(cl.color.WithAlpha(vc.config.CircleOpacity))
	b.SetStrokeColor(Color{A: 0}) // no stroke for fill pass
	b.DrawCircle(cl.cx, cl.cy, cl.radius)
	b.Pop()
}

// drawCircleStroke draws the outline of a circle.
func (vc *VennChart) drawCircleStroke(cl circleLayout) {
	b := vc.builder
	b.Push()
	b.SetFillColor(Color{A: 0}) // no fill for stroke pass
	b.SetStrokeColor(cl.color.Darken(0.15))
	b.SetStrokeWidth(vc.config.StrokeWidth)
	b.DrawCircle(cl.cx, cl.cy, cl.radius)
	b.Pop()
}

// vennFontSizes returns style-guide-based font sizes for Venn labels and items.
// Uses SizeSmall for bold labels and SizeCaption for item text.
const vennLoScale = 1.0

func vennFontSizes(style *StyleGuide) (labelSize, itemSize float64) {
	labelSize = style.Typography.SizeSmall   // bold circle/intersection labels
	itemSize = style.Typography.SizeCaption  // item text
	return
}

// drawLabels2 draws labels in the exclusive regions of a 2-circle diagram.
func (vc *VennChart) drawLabels2(data VennData, layouts []circleLayout) {
	b := vc.builder
	style := b.StyleGuide()

	r := layouts[0].radius
	overlap := vc.config.OverlapRatio

	// Inner edge of each circle's exclusive crescent (closest to the overlap).
	// Left circle's inner edge: cx + r*(1 - 2*overlap) — right circle mirrored.
	// Place label at the center of the exclusive crescent, biased outward.
	crescentInnerX0 := layouts[0].cx + r*(1.0-2*overlap)
	crescentOuterX0 := layouts[0].cx - r
	aLabelX := (crescentInnerX0 + crescentOuterX0) / 2

	crescentInnerX1 := layouts[1].cx - r*(1.0-2*overlap)
	crescentOuterX1 := layouts[1].cx + r
	bLabelX := (crescentInnerX1 + crescentOuterX1) / 2

	// Position labels in the upper portion of the circle.
	aLabelY := layouts[0].cy - r*0.40
	bLabelY := layouts[1].cy - r*0.40

	// Width of the exclusive crescent region.
	exclusiveWidth := r * (1.0 - overlap) * 0.95

	// Use style-guide font sizes.
	labelSize, itemSize := vennFontSizes(style)

	// Vertical budget: allow items to extend below center line so all items fit.
	// The exclusive crescent extends vertically; items are aligned outward so
	// they won't collide with intersection labels (which are centered).
	maxItemsBottom := layouts[0].cy + r*0.25

	// Align text outward: left circle uses right-align (text extends left),
	// right circle uses left-align (text extends right). This prevents
	// exclusive text from bleeding into the intersection zone.
	alignments := []struct {
		hAlign    HorizontalAlign
		textAlign TextAlign
	}{
		{HorizontalAlignRight, TextAlignRight},
		{HorizontalAlignLeft, TextAlignLeft},
	}

	positions := []struct {
		x, y  float64
		color Color
	}{
		{aLabelX, aLabelY, layouts[0].color},
		{bLabelX, bLabelY, layouts[1].color},
	}

	for i, pos := range positions {
		if i >= len(data.Circles) {
			break
		}
		circle := data.Circles[i]
		align := alignments[i]

		// Draw circle label (bold), clamped to fit loScaled width, wrapped at full width.
		// Use 6pt floor so long single-word labels (e.g. "Engineering") can shrink
		// enough to fit narrow crescent regions without truncation.
		vennLabelMin := math.Max(6, math.Min(8, exclusiveWidth*0.10))
		labelFit := LabelFitStrategy{PreferredSize: labelSize, MinSize: vennLabelMin, MinCharWidth: 4.0}
		origMin := b.MinFontSize()
		b.SetMinFontSize(vennLabelMin)
		labelResult := labelFit.Fit(b, circle.Label, exclusiveWidth*vennLoScale, 0)
		b.SetMinFontSize(origMin)

		// Shrink font further so every individual word fits within
		// exclusiveWidth, preventing mid-word hyphenation like "Engineeri-ng".
		labelWords := splitIntoWords(circle.Label)
		labelFontSize := labelResult.FontSize
		for _, w := range labelWords {
			if fs := b.ClampFontSize(w, exclusiveWidth, labelFontSize, vennLabelMin); fs < labelFontSize {
				labelFontSize = fs
			}
		}

		b.Push()
		b.SetFontSize(labelFontSize)
		b.SetFontWeight(style.Typography.WeightBold)
		b.SetTextColor(pos.color.Darken(0.3))

		// Widen wrap boundary if widest word still overflows at min font,
		// so WrapText never resorts to character-level breaking.
		wrapWidth := exclusiveWidth
		for _, w := range labelWords {
			if ww, _ := b.MeasureText(w); ww > wrapWidth {
				wrapWidth = ww
			}
		}
		block := b.WrapText(circle.Label, wrapWidth)
		if len(block.Lines) > 0 {
			labelY := pos.y - block.TotalHeight/2
			b.DrawTextBlock(block, pos.x, labelY+block.LineHeight, align.hAlign)
		}
		b.Pop()

		// Draw items below label, auto-shrinking font to fit all items.
		itemsStartY := pos.y + block.TotalHeight/2 + labelSize*0.4
		itemsMaxH := maxItemsBottom - itemsStartY
		fittedSize := vc.fitVennItemsFontSize(circle.Items, exclusiveWidth, itemSize, itemsMaxH, style)
		vc.drawVennItemsAligned(circle.Items, pos.x, itemsStartY, exclusiveWidth, fittedSize, itemsMaxH, align.textAlign, style)
	}
}

// drawLabels3 draws labels in the exclusive regions of a 3-circle diagram.
func (vc *VennChart) drawLabels3(data VennData, layouts []circleLayout) {
	b := vc.builder
	style := b.StyleGuide()

	labelSize, itemSize := vennFontSizes(style)

	// Exclusive region positions: push labels away from center
	centerX := (layouts[0].cx + layouts[1].cx + layouts[2].cx) / 3
	centerY := (layouts[0].cy + layouts[1].cy + layouts[2].cy) / 3

	for i, cl := range layouts {
		if i >= len(data.Circles) {
			break
		}
		circle := data.Circles[i]

		// Push label position outward from center — use 0.60 to keep
		// exclusive labels well clear of intersection text.
		dx := cl.cx - centerX
		dy := cl.cy - centerY
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist == 0 {
			dist = 1
		}
		labelX := cl.cx + dx/dist*cl.radius*0.60
		labelY := cl.cy + dy/dist*cl.radius*0.60

		exclusiveW := cl.radius * 0.80
		// Clamp font to fit within loScaled width, then wrap at full geometric width.
		// Lower min floor for narrow 3-circle exclusive regions.
		vennLabelMin3 := math.Max(6, math.Min(8, exclusiveW*0.10))
		labelFit3 := LabelFitStrategy{PreferredSize: labelSize, MinSize: vennLabelMin3, MinCharWidth: 4.0}
		origMin3 := b.MinFontSize()
		b.SetMinFontSize(vennLabelMin3)
		labelResult3 := labelFit3.Fit(b, circle.Label, exclusiveW*vennLoScale, 0)
		b.SetMinFontSize(origMin3)

		// Shrink font further so every individual word fits within
		// exclusiveW, preventing mid-word hyphenation like "Engineeri-ng".
		labelWords3 := splitIntoWords(circle.Label)
		labelFontSize3 := labelResult3.FontSize
		for _, w := range labelWords3 {
			if fs := b.ClampFontSize(w, exclusiveW, labelFontSize3, vennLabelMin3); fs < labelFontSize3 {
				labelFontSize3 = fs
			}
		}

		b.Push()
		b.SetFontSize(labelFontSize3)
		b.SetFontWeight(style.Typography.WeightBold)
		b.SetTextColor(cl.color.Darken(0.3))

		// Widen wrap boundary if widest word still overflows at min font,
		// so WrapText never resorts to character-level breaking.
		wrapWidth3 := exclusiveW
		for _, w := range labelWords3 {
			if ww, _ := b.MeasureText(w); ww > wrapWidth3 {
				wrapWidth3 = ww
			}
		}
		block := b.WrapText(circle.Label, wrapWidth3)
		if len(block.Lines) > 0 {
			bY := labelY - block.TotalHeight/2
			b.DrawTextBlock(block, labelX, bY+block.LineHeight, HorizontalAlignCenter)
		}
		b.Pop()

		itemsStartY := labelY + block.TotalHeight/2 + labelSize*0.4
		itemsMaxH := cl.radius * 0.30
		fittedSize := vc.fitVennItemsFontSize(circle.Items, exclusiveW, itemSize, itemsMaxH, style)
		vc.drawVennItemsBudgeted(circle.Items, labelX, itemsStartY, exclusiveW, fittedSize, itemsMaxH, style)
	}
}

// drawIntersection2 draws the label in the intersection region of 2 circles.
func (vc *VennChart) drawIntersection2(data VennData, layouts []circleLayout) {
	if data.Intersections == nil {
		return
	}

	b := vc.builder
	style := b.StyleGuide()
	r := layouts[0].radius

	region, ok := data.Intersections["ab"]
	if !ok {
		return
	}

	// Intersection center is midpoint between circle centers.
	ix := (layouts[0].cx + layouts[1].cx) / 2
	iy := (layouts[0].cy + layouts[1].cy) / 2

	// Blend colors for intersection
	blendColor := blendColors(layouts[0].color, layouts[1].color)

	// Calculate available width for the intersection lens.
	// Use a generous width: the lens is geometrically wide enough to hold text
	// at readable sizes. The previous formula (OverlapRatio*1.6) was too narrow.
	intersectWidth := r * math.Max(vc.config.OverlapRatio*2.5, 0.55)

	// Use style-guide font sizes, clamped to fit the intersection region.
	// Don't apply vennLoScale to the clamp budget — it double-shrinks the font.
	// The loScale is already handled by WrapText below.
	labelSize, itemSize := vennFontSizes(style)
	// Scale min floor down for very small canvases (r < 60) to avoid truncation.
	minFloor := math.Max(6, math.Min(11, r*0.15))
	interFit := LabelFitStrategy{PreferredSize: labelSize, MinSize: minFloor, MinCharWidth: 4.0}
	origMinInter := b.MinFontSize()
	b.SetMinFontSize(minFloor)
	interResult := interFit.Fit(b, region.Label, intersectWidth, 0)
	b.SetMinFontSize(origMinInter)
	labelSize = interResult.FontSize

	b.Push()
	b.SetFontSize(labelSize)
	b.SetFontWeight(style.Typography.WeightMedium)
	b.SetTextColor(blendColor.Darken(0.3))

	// Wrap the label using full geometric width (font already clamped for loScale)
	block := b.WrapText(region.Label, intersectWidth)
	if len(block.Lines) > 0 {
		labelY := iy - block.TotalHeight/2
		b.DrawTextBlock(block, ix, labelY+block.LineHeight, HorizontalAlignCenter)
	}
	b.Pop()

	// Budget items to stay within the bottom half of the intersection lens
	labelBottomY := iy + block.TotalHeight/2 + labelSize*0.4
	maxItemsBottom := iy + r*0.50
	itemsMaxH := maxItemsBottom - labelBottomY
	fittedSize := vc.fitVennItemsFontSize(region.Items, intersectWidth, itemSize, itemsMaxH, style)
	vc.drawVennItemsBudgeted(region.Items, ix, labelBottomY, intersectWidth, fittedSize, itemsMaxH, style)
}

// drawIntersections3 draws labels in the intersection regions of 3 circles.
func (vc *VennChart) drawIntersections3(data VennData, layouts []circleLayout) {
	if data.Intersections == nil {
		return
	}

	b := vc.builder
	style := b.StyleGuide()

	r := layouts[0].radius
	labelSize, itemSize := vennFontSizes(style)

	centerX := (layouts[0].cx + layouts[1].cx + layouts[2].cx) / 3
	centerY := (layouts[0].cy + layouts[1].cy + layouts[2].cy) / 3

	// Pairwise intersection positions
	pairRegions := []struct {
		key  string
		i, j int
	}{
		{"ab", 0, 1},
		{"ac", 0, 2},
		{"bc", 1, 2},
	}

	for _, pr := range pairRegions {
		region, ok := data.Intersections[pr.key]
		if !ok {
			continue
		}

		// Midpoint between two circle centers, pushed strongly away from the
		// triple center (0.45 * r) so pairwise labels don't collide with "abc"
		// or with each other.
		mx := (layouts[pr.i].cx + layouts[pr.j].cx) / 2
		my := (layouts[pr.i].cy + layouts[pr.j].cy) / 2
		dx := mx - centerX
		dy := my - centerY
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > 0 {
			pushFactor := r * 0.45
			mx += dx / dist * pushFactor
			my += dy / dist * pushFactor
		}

		blendColor := blendColors(layouts[pr.i].color, layouts[pr.j].color)
		pairWidth := r * 0.65

		// Clamp font to fit within geometric width (no loScale — it double-shrinks)
		// Scale min floor down for very small canvases to avoid truncation.
		pairMinFloor := math.Max(8, math.Min(11, r*0.15))
		pairFit := LabelFitStrategy{PreferredSize: labelSize, MinSize: pairMinFloor, MinCharWidth: 5.5}
		pairResult := pairFit.Fit(b, region.Label, pairWidth, 0)
		pairLabelSize := pairResult.FontSize
		b.Push()
		b.SetFontSize(pairLabelSize)
		b.SetFontWeight(style.Typography.WeightMedium)
		b.SetTextColor(blendColor.Darken(0.3))
		block := b.WrapText(region.Label, pairWidth)
		if len(block.Lines) > 0 {
			labelY := my - block.TotalHeight/2
			b.DrawTextBlock(block, mx, labelY+block.LineHeight, HorizontalAlignCenter)
		}
		b.Pop()

		itemsStartY := my + block.TotalHeight/2 + labelSize*0.3
		pairItemsMaxH := r * 0.18
		fittedPairSize := vc.fitVennItemsFontSize(region.Items, pairWidth, itemSize, pairItemsMaxH, style)
		vc.drawVennItemsBudgeted(region.Items, mx, itemsStartY, pairWidth, fittedPairSize, pairItemsMaxH, style)
	}

	// Triple intersection: "abc"
	if region, ok := data.Intersections["abc"]; ok {
		blendColor := blendColors(blendColors(layouts[0].color, layouts[1].color), layouts[2].color)
		tripleWidth := r * 0.50

		// Clamp font to fit within geometric width (no loScale — it double-shrinks)
		tripleMinFloor := math.Max(8, math.Min(11, r*0.15))
		tripleFit := LabelFitStrategy{PreferredSize: labelSize, MinSize: tripleMinFloor, MinCharWidth: 5.5}
		tripleResult := tripleFit.Fit(b, region.Label, tripleWidth, 0)
		tripleLabelSize := tripleResult.FontSize
		b.Push()
		b.SetFontSize(tripleLabelSize)
		b.SetFontWeight(style.Typography.WeightBold)
		b.SetTextColor(blendColor.Darken(0.35))
		block := b.WrapText(region.Label, tripleWidth)
		if len(block.Lines) > 0 {
			labelY := centerY - block.TotalHeight/2
			b.DrawTextBlock(block, centerX, labelY+block.LineHeight, HorizontalAlignCenter)
		}
		b.Pop()

		itemsStartY := centerY + block.TotalHeight/2 + labelSize*0.3
		tripleItemsMaxH := r * 0.18
		fittedTripleSize := vc.fitVennItemsFontSize(region.Items, tripleWidth, itemSize, tripleItemsMaxH, style)
		vc.drawVennItemsBudgeted(region.Items, centerX, itemsStartY, tripleWidth, fittedTripleSize, tripleItemsMaxH, style)
	}
}

// fitVennItemsFontSize uses binary search to find the largest font size (floor 6pt)
// at which all items fit within the given vertical budget. Returns the original
// fontSize if all items already fit, or a smaller size if shrinking is needed.
func (vc *VennChart) fitVennItemsFontSize(items []string, maxWidth, fontSize, maxHeight float64, style *StyleGuide) float64 {
	if len(items) == 0 || maxHeight <= 0 {
		return fontSize
	}

	b := vc.builder
	lineHeightFactor := 1.2
	if style.Typography != nil {
		lineHeightFactor = style.Typography.LineHeight
	}

	// Measure total height of all items at a given font size.
	measureHeight := func(fs float64) float64 {
		b.Push()
		b.SetFontSize(fs)
		lineSpacing := fs * lineHeightFactor
		h := 0.0
		for _, item := range items {
			block := b.WrapText(item, maxWidth)
			if len(block.Lines) == 0 {
				continue
			}
			h += float64(len(block.Lines)) * lineSpacing
			h += lineSpacing * 0.15 // inter-item gap
		}
		b.Pop()
		return h
	}

	// If items already fit at the requested size, return it.
	if measureHeight(fontSize) <= maxHeight {
		return fontSize
	}

	// Binary search for the largest font that fits.
	minFont := 8.0
	if floor := b.MinFontSize(); minFont < floor {
		minFont = floor
	}
	lo, hi := minFont, fontSize
	for hi-lo > 0.25 {
		mid := (lo + hi) / 2
		if measureHeight(mid) <= maxHeight {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// drawVennItemsAligned draws items with a specific text alignment and vertical budget.
func (vc *VennChart) drawVennItemsAligned(items []string, x, startY, maxWidth, fontSize, maxHeight float64, align TextAlign, style *StyleGuide) {
	if len(items) == 0 || maxHeight <= 0 {
		return
	}

	b := vc.builder
	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextPrimary)

	lineSpacing := fontSize
	if style.Typography != nil {
		lineSpacing *= style.Typography.LineHeight
	}

	y := startY
	for _, item := range items {
		block := b.WrapText(item, maxWidth)
		if len(block.Lines) == 0 {
			continue
		}
		itemHeight := float64(len(block.Lines)) * lineSpacing
		if y+itemHeight-startY > maxHeight {
			break
		}
		for _, line := range block.Lines {
			if line.Text == "" {
				continue
			}
			b.DrawText(line.Text, x, y, align, TextBaselineTop)
			y += lineSpacing
		}
		y += lineSpacing * 0.15
	}

	b.Pop()
}

// drawVennItemsBudgeted draws items within a vertical height budget.
// Items that would overflow maxHeight are omitted.
func (vc *VennChart) drawVennItemsBudgeted(items []string, x, startY, maxWidth, fontSize, maxHeight float64, style *StyleGuide) {
	if len(items) == 0 || maxHeight <= 0 {
		return
	}

	b := vc.builder

	b.Push()
	b.SetFontSize(fontSize)
	b.SetFontWeight(style.Typography.WeightNormal)
	b.SetTextColor(style.Palette.TextPrimary)

	lineSpacing := fontSize
	if style.Typography != nil {
		lineSpacing *= style.Typography.LineHeight
	}

	y := startY
	for _, item := range items {
		block := b.WrapText(item, maxWidth)
		if len(block.Lines) == 0 {
			continue
		}
		// Check if this item would overflow the budget
		itemHeight := float64(len(block.Lines)) * lineSpacing
		if y+itemHeight-startY > maxHeight {
			break
		}
		for _, line := range block.Lines {
			if line.Text == "" {
				continue
			}
			b.DrawText(line.Text, x, y, TextAlignCenter, TextBaselineTop)
			y += lineSpacing
		}
		y += lineSpacing * 0.15
	}

	b.Pop()
}


// blendColors creates a simple average blend of two colors.
func blendColors(a, b Color) Color {
	return Color{
		R: uint8((uint16(a.R) + uint16(b.R)) / 2),
		G: uint8((uint16(a.G) + uint16(b.G)) / 2),
		B: uint8((uint16(a.B) + uint16(b.B)) / 2),
		A: math.Max(a.A, b.A),
	}
}

// getColors returns colors for the Venn circles.
func (vc *VennChart) getColors(style *StyleGuide, count int) []Color {
	if len(vc.config.Colors) >= count {
		return vc.config.Colors[:count]
	}
	accents := style.Palette.AccentColors()
	if len(accents) >= count {
		return accents[:count]
	}
	// Fallback: sensible defaults for Venn diagrams
	return []Color{
		MustParseColor("#4E79A7"), // Blue
		MustParseColor("#E15759"), // Red
		MustParseColor("#59A14F"), // Green
	}
}

// =============================================================================
// Venn Diagram Type (for Registry)
// =============================================================================

// VennDiagram implements the Diagram interface for Venn diagrams.
type VennDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for Venn diagrams.
func (d *VennDiagram) Validate(req *RequestEnvelope) error {
	if req.Data == nil {
		return fmt.Errorf("venn diagram requires data. Expected format: {\"circles\": [{\"label\": \"Set A\", \"items\": [\"x\"]}, {\"label\": \"Set B\", \"items\": [\"y\"]}]}")
	}

	// Accept both "circles" (canonical) and "sets" (common alias from LLM output).
	circles, hasCircles := req.Data["circles"]
	if !hasCircles {
		circles, hasCircles = req.Data["sets"]
	}
	if !hasCircles {
		return fmt.Errorf("venn diagram requires 'circles' (or 'sets') array in data. Expected: {\"circles\": [{\"label\": \"Set A\", \"items\": [\"x\"]}, {\"label\": \"Set B\", \"items\": [\"y\"]}]}")
	}

	circleSlice, ok := toAnySlice(circles)
	if !ok {
		return fmt.Errorf("venn diagram 'circles' must be an array of objects, e.g. [{\"label\": \"Set A\", \"items\": [\"x\"]}, {\"label\": \"Set B\", \"items\": [\"y\"]}]")
	}

	if len(circleSlice) < 2 {
		return fmt.Errorf("venn diagram requires at least 2 circles (got %d). Provide at least: [{\"label\": \"Set A\"}, {\"label\": \"Set B\"}]", len(circleSlice))
	}
	// Allow >3 circles in input — parseVennData will use the first 3.

	return nil
}

// Render generates an SVG document from the request envelope.
func (d *VennDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder renders the diagram and returns both the builder and SVG document.
func (d *VennDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		data, err := parseVennData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultVennConfig(width, height)

		// Apply custom options
		if opacity, ok := req.Data["circle_opacity"].(float64); ok {
			config.CircleOpacity = opacity
		}
		if strokeW, ok := req.Data["stroke_width"].(float64); ok {
			config.StrokeWidth = strokeW
		}
		if overlap, ok := req.Data["overlap_ratio"].(float64); ok {
			config.OverlapRatio = overlap
		}

		chart := NewVennChart(builder, config)
		return chart.Draw(data)
	})
}

// parseVennData parses the request data into VennData.
func parseVennData(req *RequestEnvelope) (VennData, error) {
	data := VennData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	// Parse circles (cap at 3 — use first 3 if more provided).
	// Accept "sets" as alias for "circles".
	circlesKey := req.Data["circles"]
	if circlesKey == nil {
		circlesKey = req.Data["sets"]
	}
	circlesRaw, ok := toAnySlice(circlesKey)
	if !ok {
		return data, fmt.Errorf("invalid venn circles format")
	}
	if len(circlesRaw) > 3 {
		circlesRaw = circlesRaw[:3]
	}

	for _, cRaw := range circlesRaw {
		c, ok := cRaw.(map[string]any)
		if !ok {
			return data, fmt.Errorf("invalid venn circle format")
		}

		circle := VennCircle{}
		if label, ok := c["label"].(string); ok {
			circle.Label = label
		}
		circle.Items = parseStringList(c["items"])
		data.Circles = append(data.Circles, circle)
	}

	// Parse intersections
	if intersRaw, ok := req.Data["intersections"].(map[string]any); ok {
		data.Intersections = make(map[string]VennRegion)
		for key, vRaw := range intersRaw {
			switch v := vRaw.(type) {
			case map[string]any:
				region := VennRegion{}
				if label, ok := v["label"].(string); ok {
					region.Label = label
				}
				region.Items = parseStringList(v["items"])
				data.Intersections[key] = region
			case string:
				data.Intersections[key] = VennRegion{Label: v}
			}
		}
	}

	// Parse footnote
	if footnote, ok := req.Data["footnote"].(string); ok {
		data.Footnote = footnote
	}

	return data, nil
}

