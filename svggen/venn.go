package svggen

import (
	"fmt"
	"math"
	"sort"
	"strings"
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

	// FixedOverlap stops a two-circle diagram from widening OverlapRatio (up
	// to vennMaxAdaptiveOverlap) when that is what it takes to fit the
	// intersection caption inside the lens. Set when the author supplied
	// overlap_ratio.
	FixedOverlap bool
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
	// layouts are the circles of the last Draw, for region checks in tests.
	layouts []circleLayout
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
	// A Venn beyond 3 circles has no readable planar form, so the extra
	// circles used to be silently dropped: passing 5 rendered 3 with no notice
	// at all (go-slide-creator-onop). Refuse instead, in the same style as the
	// other data constraints, so the author picks a different diagram.
	if len(data.Circles) > vennMaxCircles {
		return fmt.Errorf(
			"venn diagram accepts at most %d circles, got %d — a 4+ set Venn has no readable planar form; use a matrix_2x2, a card grid, or split the comparison across slides",
			vennMaxCircles, len(data.Circles))
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
		vc.adaptOverlap2(data, plotArea, colors)
		layouts = vc.layout2Circles(plotArea, colors)
	} else {
		vc.adaptOverlap3(data, plotArea, colors)
		layouts = vc.layout3Circles(plotArea, colors)
	}

	vc.layouts = layouts

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
		vc.drawIntersection2(data, layouts, plotArea, colors)
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
	// The pair is 2·(2-o)·r wide for overlap o. The old W/2.8 bound ignored
	// that, so a width-bound frame (a half-width column) drew the outer
	// circles past the canvas edge.
	radius := math.Min(area.W/(2*(2-vc.config.OverlapRatio)), area.H/2.1)
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
	// The triangle's footprint is (2 + sqrt(3)(1-o))·r wide and
	// (2 + 1.5(1-o))·r tall for overlap o. The historical divisors (3.2, 3.0)
	// stay the ceiling, so a wider overlap — adopted to fit intersection
	// captions — grows the circles into the space it frees instead of
	// shrinking the diagram.
	spread := 1.0 - vc.config.OverlapRatio
	radius := math.Min(area.W/math.Min(3.2, 2+math.Sqrt(3)*spread), area.H/math.Min(3.0, 2+1.5*spread))
	offset := radius * spread

	// Equilateral triangle arrangement:
	// Top circle, bottom-left, bottom-right.
	// The footprint runs from offset+r above the centroid to offset/2+r below
	// it, so the centroid sits offset/4 below the plot centre to centre the
	// footprint vertically. With the centroid at the plot centre the top
	// circle overran the plot area while space was left below the diagram,
	// which cost the height-bound diagrams on wide slide frames radius.
	triCenterY := centerY + offset/4

	return []circleLayout{
		{cx: centerX, cy: triCenterY - offset, radius: radius, color: colors[0]},                                                  // top
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
	labelSize = style.Typography.SizeSmall  // bold circle/intersection labels
	itemSize = style.Typography.SizeCaption // item text
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
		vc.drawVennItemsAligned(circle.Items, pos.x, itemsStartY, exclusiveWidth, fittedSize, itemsMaxH, align.textAlign, style, circle.Label)
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
		itemsMaxH := cl.radius * vennItemsHeightFrac
		fittedSize := vc.fitVennItemsFontSize(circle.Items, exclusiveW, itemSize, itemsMaxH, style)
		vc.drawVennItemsBudgeted(circle.Items, labelX, itemsStartY, exclusiveW, fittedSize, itemsMaxH, style, circle.Label)
	}
}

// vennMaxAdaptiveOverlap is the widest overlap a two-circle diagram adopts on
// its own to fit an intersection caption. Beyond it the exclusive crescents get
// too thin for their own labels.
const vennMaxAdaptiveOverlap = 0.40

// vennOverlapStep is the increment of the overlap search.
const vennOverlapStep = 0.05

// vennLens2 is the laid-out "ab" region of a two-circle diagram: the caption
// fitted to the lens and the item list's budget below it.
type vennLens2 struct {
	fit        vennCaptionFit
	ix, iy     float64
	itemsTop   float64
	itemsW     float64
	itemsMaxH  float64
	itemsFont  float64
	itemsAllIn bool // every item fits the budget at itemsFont
}

// layoutIntersection2 fits the "ab" caption inside the lens of two circles and
// budgets its items: as wide as the lens where they sit, down to r/2 below
// the centre line.
func (vc *VennChart) layoutIntersection2(region VennRegion, layouts []circleLayout) vennLens2 {
	style := vc.builder.StyleGuide()
	r := layouts[0].radius
	l := vennLens2{
		ix: (layouts[0].cx + layouts[1].cx) / 2,
		iy: (layouts[0].cy + layouts[1].cy) / 2,
	}
	floor := vc.vennCaptionFloor(r)
	labelSize, itemSize := vennFontSizes(style)
	lens := vennRegionShape{in: layouts[:2], pad: vc.vennCaptionPad(floor)}
	l.fit = vc.fitCaptionInRegion(region.Label, lens, l.ix, l.iy, labelSize, floor, style.Typography.WeightMedium)

	l.itemsTop = l.iy + l.fit.height()/2 + labelSize*0.4
	l.itemsMaxH = l.iy + r*0.50 - l.itemsTop
	itemsLens := vennRegionShape{in: layouts[:2], pad: vc.vennCaptionPad(itemSize)}
	l.itemsW = 2 * itemsLens.halfWidth(l.ix, l.itemsTop, l.itemsTop+math.Max(0, math.Min(l.itemsMaxH, itemSize*3)))
	if l.itemsW <= 0 {
		l.itemsW = r * math.Max(vc.config.OverlapRatio*2.5, 0.55)
	}
	l.itemsFont = vc.fitVennItemsFontSize(region.Items, l.itemsW, itemSize, l.itemsMaxH, style)
	l.itemsAllIn = len(region.Items) == 0 || vc.vennItemsHeight(region.Items, l.itemsW, l.itemsFont) <= l.itemsMaxH
	return l
}

// vennItemsHeight is the stacked height of items wrapped at width, as
// drawVennItemsBudgeted lays them out.
func (vc *VennChart) vennItemsHeight(items []string, width, fontSize float64) float64 {
	b := vc.builder
	lineFactor := 1.2
	if st := b.StyleGuide(); st != nil && st.Typography != nil && st.Typography.LineHeight > 0 {
		lineFactor = st.Typography.LineHeight
	}
	b.Push()
	defer b.Pop()
	b.SetFontSize(fontSize)
	lineSpacing := fontSize * lineFactor
	h := 0.0
	for _, item := range items {
		block := b.WrapText(item, width)
		if len(block.Lines) == 0 {
			continue
		}
		h += float64(len(block.Lines)) * lineSpacing
	}
	return h + float64(max(len(items)-1, 0))*lineSpacing*0.15
}

// adaptOverlap2 widens the overlap of a two-circle diagram, in
// vennOverlapStep increments up to vennMaxAdaptiveOverlap, until the "ab"
// caption fits the lens at (close to) its preferred size with all of its
// items. It keeps the overlap that fits best, and leaves it unchanged when no
// overlap helps or when the author fixed it.
func (vc *VennChart) adaptOverlap2(data VennData, area Rect, colors []Color) {
	if vc.config.FixedOverlap || data.Intersections == nil {
		return
	}
	region, ok := data.Intersections["ab"]
	if !ok || (strings.TrimSpace(region.Label) == "" && len(region.Items) == 0) {
		return
	}
	if ov, found := vc.overlapFittingIntersection(region, area, colors); found {
		vc.config.OverlapRatio = ov
	}
}

// overlapFittingIntersection searches overlaps from the current ratio up to
// vennMaxAdaptiveOverlap for the one that fits the "ab" region best: the first
// whose caption fits at 90% of the preferred size with every item, else the
// best by (caption fits, items fit, caption size). It restores the configured
// ratio before returning; found is false when no overlap fits the caption.
func (vc *VennChart) overlapFittingIntersection(region VennRegion, area Rect, colors []Color) (float64, bool) {
	base := vc.config.OverlapRatio
	defer func() { vc.config.OverlapRatio = base }()
	labelSize, _ := vennFontSizes(vc.builder.StyleGuide())
	type score struct {
		items bool
		font  float64
	}
	better := func(a, b score) bool {
		if a.items != b.items {
			return a.items
		}
		return a.font > b.font+1e-9
	}
	bestOv, found := base, false
	var best score
	for i := 0; ; i++ {
		ov := base + float64(i)*vennOverlapStep
		if i > 0 && ov > vennMaxAdaptiveOverlap+1e-9 {
			break
		}
		vc.config.OverlapRatio = ov
		l := vc.layoutIntersection2(region, vc.layout2Circles(area, colors))
		if !l.fit.ok {
			continue
		}
		if l.itemsAllIn && l.fit.fontSize >= labelSize*0.9-1e-9 {
			return ov, true
		}
		if sc := (score{l.itemsAllIn, l.fit.fontSize}); !found || better(sc, best) {
			bestOv, best, found = ov, sc, true
		}
	}
	return bestOv, found
}

// drawIntersection2 draws the label in the intersection region of 2 circles.
func (vc *VennChart) drawIntersection2(data VennData, layouts []circleLayout, area Rect, colors []Color) {
	if data.Intersections == nil {
		return
	}
	region, ok := data.Intersections["ab"]
	if !ok {
		return
	}
	style := vc.builder.StyleGuide()

	// Fit the caption to the lens itself: every line inside both circles over
	// its full height, at a size no smaller than the builder's floor.
	l := vc.layoutIntersection2(region, layouts)
	blendColor := blendColors(layouts[0].color, layouts[1].color)
	vc.drawCaption(l.fit, l.ix, style.Typography.WeightMedium, blendColor.Darken(0.3))
	if !l.fit.ok {
		suggested := 0.0
		if vc.config.FixedOverlap {
			if ov, found := vc.overlapFittingIntersection(region, area, colors); found && ov > vc.config.OverlapRatio {
				suggested = ov
			}
		}
		vc.reportRegionOverflow("ab", region.Label, l.fit, data, suggested)
	}
	vc.drawVennItemsBudgeted(region.Items, l.ix, l.itemsTop, l.itemsW, l.itemsFont, l.itemsMaxH, style, region.Label)
}

// vennPlacedCaption is an intersection caption fitted to its region.
type vennPlacedCaption struct {
	key    string
	region VennRegion
	fit    vennCaptionFit
	x, y   float64
	shape  vennRegionShape
	color  Color
	weight int
}

// vennPairRegions are the pairwise regions of a 3-circle diagram.
var vennPairRegions = []struct {
	key  string
	i, j int
}{
	{"ab", 0, 1},
	{"ac", 0, 2},
	{"bc", 1, 2},
}

// placeCaptions3 fits every intersection caption of a 3-circle diagram to its
// region: pairwise captions inside both of their circles and outside the
// third, the triple caption inside all three.
func (vc *VennChart) placeCaptions3(data VennData, layouts []circleLayout) []vennPlacedCaption {
	if data.Intersections == nil {
		return nil
	}
	style := vc.builder.StyleGuide()
	r := layouts[0].radius
	labelSize, _ := vennFontSizes(style)
	centerX := (layouts[0].cx + layouts[1].cx + layouts[2].cx) / 3
	centerY := (layouts[0].cy + layouts[1].cy + layouts[2].cy) / 3
	floor := vc.vennCaptionFloor(r)
	pad := vc.vennCaptionPad(floor)
	weight := style.Typography.WeightMedium

	var out []vennPlacedCaption
	for _, pr := range vennPairRegions {
		region, ok := data.Intersections[pr.key]
		if !ok {
			continue
		}
		k := 3 - pr.i - pr.j // the circle this region excludes

		// Search the caption anchor along the ray from the triple centre
		// through the pair midpoint (the region's axis of symmetry) for the
		// position that fits the caption largest, preferring the historical
		// 0.45r push.
		mx0 := (layouts[pr.i].cx + layouts[pr.j].cx) / 2
		my0 := (layouts[pr.i].cy + layouts[pr.j].cy) / 2
		dx := mx0 - centerX
		dy := my0 - centerY
		dist := math.Sqrt(dx*dx + dy*dy)
		ux, uy := 0.0, 0.0
		if dist > 0 {
			ux, uy = dx/dist, dy/dist
		}
		shape := vennRegionShape{in: []circleLayout{layouts[pr.i], layouts[pr.j]}, out: []circleLayout{layouts[k]}, pad: pad}
		fit, mx, my := vc.bestAnchoredCaption(region.Label, shape, mx0, my0, ux, uy, r, labelSize, floor, weight)
		out = append(out, vennPlacedCaption{
			key: pr.key, region: region, fit: fit, x: mx, y: my, shape: shape,
			color: blendColors(layouts[pr.i].color, layouts[pr.j].color).Darken(0.3), weight: weight,
		})
	}

	if region, ok := data.Intersections["abc"]; ok {
		shape := vennRegionShape{in: layouts[:3], pad: pad}
		bold := style.Typography.WeightBold
		fit := vc.fitCaptionInRegion(region.Label, shape, centerX, centerY, labelSize, floor, bold)
		out = append(out, vennPlacedCaption{
			key: "abc", region: region, fit: fit, x: centerX, y: centerY, shape: shape,
			color:  blendColors(blendColors(layouts[0].color, layouts[1].color), layouts[2].color).Darken(0.35),
			weight: bold,
		})
	}
	return out
}

// vennCaptionScore ranks a set of placed captions: more fitting captions
// first, then the larger smallest font.
func vennCaptionScore(placed []vennPlacedCaption) (fitting int, minFont float64) {
	minFont = math.Inf(1)
	for _, p := range placed {
		if strings.TrimSpace(p.region.Label) == "" {
			continue
		}
		if p.fit.ok {
			fitting++
		}
		minFont = math.Min(minFont, p.fit.fontSize)
	}
	return fitting, minFont
}

// adaptOverlap3 widens the overlap of a 3-circle diagram, in vennOverlapStep
// increments up to vennMaxAdaptiveOverlap3, until every intersection caption
// fits its region at (close to) its preferred size, keeping the overlap that
// fits the most captions at the largest size. The default 0.20 overlap leaves
// the pairwise and triple regions too small for even a two-word caption at a
// readable size.
func (vc *VennChart) adaptOverlap3(data VennData, area Rect, colors []Color) {
	if vc.config.FixedOverlap || data.Intersections == nil {
		return
	}
	labelled := 0
	for _, reg := range data.Intersections {
		if strings.TrimSpace(reg.Label) != "" {
			labelled++
		}
	}
	if labelled == 0 {
		return
	}
	labelSize, _ := vennFontSizes(vc.builder.StyleGuide())
	base := vc.config.OverlapRatio
	bestOv, bestFit, bestFont := base, -1, -1.0
	for i := 0; ; i++ {
		ov := base + float64(i)*vennOverlapStep
		if i > 0 && ov > vennMaxAdaptiveOverlap3+1e-9 {
			break
		}
		vc.config.OverlapRatio = ov
		fitting, minFont := vennCaptionScore(vc.placeCaptions3(data, vc.layout3Circles(area, colors)))
		if fitting == labelled && minFont >= labelSize*0.9-1e-9 {
			bestOv = ov
			break
		}
		if fitting > bestFit || (fitting == bestFit && minFont > bestFont+1e-9) {
			bestOv, bestFit, bestFont = ov, fitting, minFont
		}
	}
	vc.config.OverlapRatio = bestOv
}

// vennMaxAdaptiveOverlap3 is the widest overlap a 3-circle diagram adopts on
// its own. Past it the pairwise regions shrink again as the triple region
// grows.
const vennMaxAdaptiveOverlap3 = 0.50

// drawIntersections3 draws labels in the intersection regions of 3 circles.
func (vc *VennChart) drawIntersections3(data VennData, layouts []circleLayout) {
	style := vc.builder.StyleGuide()
	r := layouts[0].radius
	labelSize, itemSize := vennFontSizes(style)

	for _, p := range vc.placeCaptions3(data, layouts) {
		vc.drawCaption(p.fit, p.x, p.weight, p.color)
		if !p.fit.ok {
			vc.reportRegionOverflow(p.key, p.region.Label, p.fit, data, 0)
		}

		width := r * 0.65
		if p.key == "abc" {
			width = r * 0.50
		}
		itemsStartY := p.y + p.fit.height()/2 + labelSize*0.3
		itemsMaxH := r * 0.18
		if w := 2 * p.shape.halfWidth(p.x, itemsStartY, itemsStartY+itemsMaxH); w > 0 && w < width {
			width = w
		}
		fitted := vc.fitVennItemsFontSize(p.region.Items, width, itemSize, itemsMaxH, style)
		vc.drawVennItemsBudgeted(p.region.Items, p.x, itemsStartY, width, fitted, itemsMaxH, style, p.region.Label)
	}
}

// vennAnchor is a candidate caption anchor in a pairwise region, in radius
// units along (t) and across (q) the region's axis.
type vennAnchor struct{ t, q float64 }

// vennPairAnchors lists the anchors bestAnchoredCaption tries, nearest to the
// historical 0.45r push first. The region's axis is diagonal for the upper
// pairs, so horizontal text can gain width a little off the axis.
var vennPairAnchors = func() []vennAnchor {
	const preferredT = 0.45
	var out []vennAnchor
	for i := 0; i <= 14; i++ {
		for _, q := range []float64{0, -0.10, 0.10, -0.20, 0.20} {
			out = append(out, vennAnchor{t: 0.10 + float64(i)*0.05, q: q})
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		da := math.Abs(out[a].t-preferredT) + math.Abs(out[a].q)
		db := math.Abs(out[b].t-preferredT) + math.Abs(out[b].q)
		return da < db
	})
	return out
}()

// bestAnchoredCaption fits label into shape at the anchors of vennPairAnchors
// around (x0, y0) on the axis (ux, uy) and returns the best: a fitting layout
// over a failing one, then the larger font, then the anchor nearest the
// historical position. It stops at the first anchor that fits at the
// preferred size.
func (vc *VennChart) bestAnchoredCaption(label string, shape vennRegionShape, x0, y0, ux, uy, r, preferred, floor float64, weight int) (vennCaptionFit, float64, float64) {
	at := func(a vennAnchor) (float64, float64) {
		return x0 + (ux*a.t-uy*a.q)*r, y0 + (uy*a.t+ux*a.q)*r
	}
	var best vennCaptionFit
	bestX, bestY := at(vennPairAnchors[0])
	have := false
	for _, a := range vennPairAnchors {
		ax, ay := at(a)
		fit := vc.fitCaptionInRegion(label, shape, ax, ay, preferred, floor, weight)
		if !fit.ok {
			continue
		}
		if !have || fit.fontSize > best.fontSize+1e-9 {
			best, bestX, bestY, have = fit, ax, ay, true
		}
		if fit.fontSize >= math.Max(preferred, floor)-1e-9 {
			break
		}
	}
	if !have {
		// Nothing fits: keep the historical anchor for the best-effort layout.
		ax, ay := at(vennPairAnchors[0])
		return vc.fitCaptionInRegion(label, shape, ax, ay, preferred, floor, weight), ax, ay
	}
	return best, bestX, bestY
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
func (vc *VennChart) drawVennItemsAligned(items []string, x, startY, maxWidth, fontSize, maxHeight float64, align TextAlign, style *StyleGuide, circleLabel string) {
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

	drawn := 0
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
		drawn++
	}

	b.Pop()
	vc.reportDroppedItems(len(items), drawn, circleLabel)
}

// drawVennItemsBudgeted draws items within a vertical height budget.
// Items that would overflow maxHeight are omitted.
func (vc *VennChart) drawVennItemsBudgeted(items []string, x, startY, maxWidth, fontSize, maxHeight float64, style *StyleGuide, circleLabel string) {
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

	drawn := 0
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
		drawn++
	}

	b.Pop()
	vc.reportDroppedItems(len(items), drawn, circleLabel)
}

// vennMaxCircles is the largest set count a Venn can render readably.
const vennMaxCircles = 3

// vennItemsHeightFrac is the share of a circle's radius reserved for its
// exclusive item list, below the circle label. At the previous 0.30 a
// three-circle Venn had room for barely one short item, so ordinary 2-4 item
// lists were silently truncated (go-slide-creator-onop).
const vennItemsHeightFrac = 0.55

// reportDroppedItems emits a diagram.items_dropped finding when a circle's item
// list did not fit its budget. The items are gone from the picture by the time
// this runs; the finding is what makes the loss visible, since the rendered
// slide otherwise shows a labelled circle and nothing else.
func (vc *VennChart) reportDroppedItems(total, drawn int, circleLabel string) {
	if drawn >= total {
		return
	}
	dropped := total - drawn
	where := "a circle"
	if circleLabel != "" {
		where = fmt.Sprintf("circle %q", circleLabel)
	}
	vc.builder.AddFinding(Finding{
		Field: "circles",
		Code:  FindingDiagramItemsDropped,
		Message: fmt.Sprintf(
			"venn: %d of %d item(s) in %s did not fit and were not rendered — shorten the items, list fewer of them, or move the detail to a text column beside the diagram",
			dropped, total, where),
		Severity: "refuse",
		Fix: &FixSuggestion{
			Kind: FixKindReduceItems,
			Params: map[string]any{
				"dropped_count": dropped,
				"total_count":   total,
				"rendered":      drawn,
				"circle":        circleLabel,
				"diagram_type":  "venn",
			},
		},
	})
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
		MustParseColor(DefaultThemeAccent1Hex), // Blue
		MustParseColor(DefaultThemeAccent3Hex), // Red
		MustParseColor(DefaultThemeAccent5Hex), // Green
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
	// A 4+ set Venn has no readable planar form. Extra circles used to be
	// silently dropped here — 5 circles rendered 3 with no notice at all — so
	// the limit is now part of the data contract (go-slide-creator-onop).
	if len(circleSlice) > vennMaxCircles {
		return fmt.Errorf(
			"venn diagram accepts at most %d circles, got %d — a 4+ set Venn has no readable planar form; use a matrix_2x2, a card grid, or split the comparison across slides",
			vennMaxCircles, len(circleSlice))
	}

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
			config.FixedOverlap = true
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

	// Parse circles. Validate has already refused more than vennMaxCircles, so
	// nothing is dropped here.
	// Accept "sets" as alias for "circles".
	circlesKey := req.Data["circles"]
	if circlesKey == nil {
		circlesKey = req.Data["sets"]
	}
	circlesRaw, ok := toAnySlice(circlesKey)
	if !ok {
		return data, fmt.Errorf("invalid venn circles format")
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
