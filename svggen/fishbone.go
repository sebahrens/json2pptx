package svggen

import (
	"fmt"
	"math"
)

// =============================================================================
// Fishbone (Ishikawa) Diagram
// =============================================================================

// FishboneConfig holds configuration for fishbone diagrams.
type FishboneConfig struct {
	ChartConfig

	// BranchAngle is the angle of branches from the spine in degrees.
	BranchAngle float64

	// SpineWidth is the stroke width of the main spine.
	SpineWidth float64

	// BranchWidth is the stroke width of branches.
	BranchWidth float64

	// CornerRadius is the radius for rounded category boxes.
	CornerRadius float64

	// MaxVisibleCategories is the maximum number of categories rendered
	// before collapsing the rest into an overflow indicator. Zero means
	// no limit; the default is 10.
	MaxVisibleCategories int
}

// fishboneBranchLayout holds precomputed layout information for a single
// category branch, used in the two-pass collision detection algorithm.
type fishboneBranchLayout struct {
	catIndex   int     // index into FishboneData.Categories
	isTop      bool    // true = top branch, false = bottom
	branchX    float64 // X where branch meets the spine
	endX       float64 // branch endpoint X
	endY       float64 // branch endpoint Y
	catName    string  // category name (full text, may be wrapped)
	fontSize   float64 // category label font size
	boxRect    Rect    // bounding box of the category label
	catWrapped bool    // true if label should be drawn with text wrapping
}

// DefaultFishboneConfig returns default fishbone configuration.
func DefaultFishboneConfig(width, height float64) FishboneConfig {
	return FishboneConfig{
		ChartConfig:          DefaultChartConfig(width, height),
		BranchAngle:          30,
		SpineWidth:           4,
		BranchWidth:          3,
		CornerRadius:         6,
		MaxVisibleCategories: 10,
	}
}

// FishboneData represents the data for a fishbone diagram.
type FishboneData struct {
	Title    string
	Subtitle string
	// Effect is the main issue/effect at the head of the fish.
	Effect string
	// Categories are the cause categories branching off the spine.
	Categories []FishboneCategory
}

// FishboneCategory represents a cause category on the fishbone.
type FishboneCategory struct {
	// Name is the category name (e.g., "People", "Process", "Materials").
	Name string
	// Causes are individual causes within this category.
	Causes []string
}

// FishboneChart renders fishbone/Ishikawa diagrams.
type FishboneChart struct {
	builder *SVGBuilder
	config  FishboneConfig
}

// NewFishboneChart creates a new fishbone chart renderer.
func NewFishboneChart(builder *SVGBuilder, config FishboneConfig) *FishboneChart {
	return &FishboneChart{
		builder: builder,
		config:  config,
	}
}

// Draw renders the fishbone diagram.
//nolint:gocognit // complex chart rendering logic
func (fc *FishboneChart) Draw(data FishboneData) error {
	if data.Effect == "" {
		return fmt.Errorf("fishbone diagram requires an 'effect'")
	}

	b := fc.builder
	style := b.StyleGuide()
	accentColors := style.Palette.AccentColors()

	// Width-proportional scaling factor. Reference width is 900 (the default
	// golden test fishbone size). At narrow widths (e.g., 500 or 760), fonts
	// and boxes scale down proportionally so everything fits in the viewBox.
	const refWidth = 900.0
	widthScale := fc.config.Width / refWidth
	if widthScale > 1.0 {
		widthScale = 1.0 // never scale UP beyond reference
	}
	isNarrow := fc.config.Width < 600
	isVeryNarrow := fc.config.Width < 500

	// Density-based font scaling: reduce font sizes when many categories
	// AND many items are present to prevent text overlap along the spine.
	numCats := len(data.Categories)
	totalCausesAll := 0
	maxCausesInCat := 0
	for _, cat := range data.Categories {
		totalCausesAll += len(cat.Causes)
		if len(cat.Causes) > maxCausesInCat {
			maxCausesInCat = len(cat.Causes)
		}
	}

	densityScale := 1.0
	switch {
	case numCats >= 7:
		densityScale = 0.75
	case numCats >= 5:
		densityScale = 0.85
	}

	// Additional scaling when total item count is very high (many categories
	// each with many causes). This prevents cause text from overlapping
	// within and across branches.
	itemDensityScale := 1.0
	switch {
	case totalCausesAll > 30:
		itemDensityScale = 0.70
	case totalCausesAll > 20:
		itemDensityScale = 0.80
	case totalCausesAll > 14 && numCats >= 5:
		itemDensityScale = 0.85
	}
	// When a single category has many causes, the branch gets crowded.
	if maxCausesInCat >= 6 && itemDensityScale > 0.80 {
		itemDensityScale = 0.80
	}
	if itemDensityScale < densityScale {
		densityScale = itemDensityScale
	}

	// Scale font sizes proportionally to width and density
	bodyFontSize := style.Typography.SizeBody * widthScale * densityScale
	smallFontSize := style.Typography.SizeSmall * widthScale * densityScale
	captionFontSize := style.Typography.SizeCaption * widthScale * densityScale

	// Enforce minimum readable sizes. Fishbone uses a 9pt floor for base
	// font sizes to maintain stable layout geometry; cause labels use their
	// own lower floor (7pt) via LabelFitStrategy at draw time.
	// At very narrow widths, allow smaller fonts to prevent severe truncation.
	floorBase := 9.0
	if isVeryNarrow {
		floorBase = 7.0
	}
	floor := math.Max(floorBase, b.MinFontSize())
	if bodyFontSize < floor {
		bodyFontSize = floor
	}
	if smallFontSize < floor {
		smallFontSize = floor
	}
	if captionFontSize < floor {
		captionFontSize = floor
	}

	// Calculate plot area
	plotArea := fc.config.PlotArea()

	// Adjust for title
	headerHeight := 0.0
	if fc.config.ShowTitle && data.Title != "" {
		headerHeight = style.Typography.SizeTitle + style.Spacing.MD
		if data.Subtitle != "" {
			headerHeight += style.Typography.SizeSubtitle + style.Spacing.XS
		}
	}
	plotArea.Y += headerHeight
	plotArea.H -= headerHeight

	// Layout: spine runs horizontally, effect box on right.
	// Effect box is sized dynamically to fit the effect text, constrained
	// within 18%-35% of plot width and 15%-40% of plot height.
	// The width adapts: start at 18%, widen if text would be truncated.
	minEffectW := plotArea.W * 0.18
	maxEffectW := plotArea.W * 0.35
	if minEffectW > maxEffectW {
		minEffectW = maxEffectW
	}
	// At narrow widths, give effect box a minimum size based on text
	if isNarrow {
		minEffectWNarrow := bodyFontSize * 6 // room for ~6 characters
		if minEffectW < minEffectWNarrow {
			minEffectW = minEffectWNarrow
		}
	}

	// Measure effect text to determine needed box dimensions.
	// Adaptively widen the effect box (up to maxEffectW) if the text
	// would overflow the height cap at the initial width.
	effectPadding := style.Spacing.SM * widthScale
	maxEffectH := plotArea.H * 0.40

	effectBoxW := minEffectW
	effectFontSize := bodyFontSize

	b.Push()
	b.SetFontSize(effectFontSize)
	b.SetFontWeight(style.Typography.WeightBold)
	effectBlock := b.WrapText(data.Effect, effectBoxW-effectPadding*2)
	b.Pop()

	effectBoxH := plotArea.H * 0.15 // minimum
	if effectBlock.TotalHeight+effectPadding*2 > effectBoxH {
		effectBoxH = effectBlock.TotalHeight + effectPadding*2
	}

	// If text overflows height cap, widen the box progressively
	if effectBoxH > maxEffectH && effectBoxW < maxEffectW {
		for effectBoxW < maxEffectW {
			effectBoxW += plotArea.W * 0.03
			if effectBoxW > maxEffectW {
				effectBoxW = maxEffectW
			}
			b.Push()
			b.SetFontSize(effectFontSize)
			b.SetFontWeight(style.Typography.WeightBold)
			effectBlock = b.WrapText(data.Effect, effectBoxW-effectPadding*2)
			b.Pop()
			effectBoxH = effectBlock.TotalHeight + effectPadding*2
			if effectBoxH < plotArea.H*0.15 {
				effectBoxH = plotArea.H * 0.15
			}
			if effectBoxH <= maxEffectH {
				break
			}
		}
	}

	// If still overflows after max width, reduce font size
	if effectBoxH > maxEffectH {
		minFont := b.MinFontSize()
		for effectFontSize > minFont && effectBoxH > maxEffectH {
			effectFontSize *= 0.9
			if effectFontSize < minFont {
				effectFontSize = minFont
			}
			b.Push()
			b.SetFontSize(effectFontSize)
			b.SetFontWeight(style.Typography.WeightBold)
			effectBlock = b.WrapText(data.Effect, effectBoxW-effectPadding*2)
			b.Pop()
			effectBoxH = effectBlock.TotalHeight + effectPadding*2
			if effectBoxH < plotArea.H*0.15 {
				effectBoxH = plotArea.H * 0.15
			}
		}
	}

	// Final height cap
	if effectBoxH > maxEffectH {
		effectBoxH = maxEffectH
	}

	spineStartX := plotArea.X + style.Spacing.LG*widthScale
	spineEndX := plotArea.X + plotArea.W - effectBoxW
	spineY := plotArea.Y + plotArea.H/2

	// Scale spine and branch widths for narrow charts
	spineWidth := fc.config.SpineWidth * math.Max(0.6, widthScale)
	branchWidth := fc.config.BranchWidth * math.Max(0.6, widthScale)

	// Draw spine (main horizontal line)
	b.Push()
	b.SetStrokeColor(style.Palette.TextPrimary)
	b.SetStrokeWidth(spineWidth)
	b.DrawLine(spineStartX, spineY, spineEndX, spineY)
	b.Pop()

	// Draw arrowhead at spine end
	arrowSize := 10.0 * math.Max(0.6, widthScale)
	b.Push()
	b.SetFillColor(style.Palette.TextPrimary)
	arrowPoints := []Point{
		{X: spineEndX, Y: spineY},
		{X: spineEndX - arrowSize, Y: spineY - arrowSize/2},
		{X: spineEndX - arrowSize, Y: spineY + arrowSize/2},
	}
	b.DrawPolygon(arrowPoints)
	b.Pop()

	// Draw effect box
	b.Push()
	effectColor := accentColors[0]
	b.SetFillColor(effectColor.WithAlpha(0.15))
	b.SetStrokeColor(effectColor)
	b.SetStrokeWidth(2)
	effectRect := Rect{
		X: spineEndX,
		Y: spineY - effectBoxH/2,
		W: effectBoxW,
		H: effectBoxH,
	}
	b.DrawRoundedRect(effectRect, fc.config.CornerRadius)
	b.Pop()

	// Draw effect text using the pre-computed effectBlock. We avoid
	// DrawWrappedText here because it re-wraps from scratch, and floating-
	// point differences between the sizing pass and the rendering pass can
	// cause the re-wrapped TotalHeight to exceed contentRect.H by a
	// sub-pixel amount, triggering unwanted truncation.
	b.Push()
	b.SetFontSize(effectFontSize)
	b.SetFontWeight(style.Typography.WeightBold)
	contentRect := effectRect.Inset(effectPadding, effectPadding, effectPadding, effectPadding)
	x, y := b.AlignBlockInRect(effectBlock, contentRect, AlignCenter)
	b.DrawTextBlock(effectBlock, x, y, HorizontalAlignCenter)
	b.Pop()

	// Draw categories as branches
	if numCats == 0 {
		// Draw title and return
		if fc.config.ShowTitle && data.Title != "" {
			titleConfig := DefaultTitleConfig()
			titleConfig.Text = data.Title
			titleConfig.Subtitle = data.Subtitle
			title := NewTitle(b, titleConfig)
			title.Draw(Rect{X: 0, Y: 0, W: fc.config.Width, H: headerHeight + fc.config.MarginTop})
		}
		return nil
	}

	// Determine the effective max visible categories. Collapse excess into
	// an overflow indicator drawn near the spine.
	maxVisible := fc.config.MaxVisibleCategories
	if maxVisible <= 0 {
		maxVisible = 10
	}
	visibleCats := data.Categories
	overflowCount := 0
	if numCats > maxVisible {
		overflowCount = numCats - maxVisible
		visibleCats = data.Categories[:maxVisible]
	}
	numVisible := len(visibleCats)

	spineLength := spineEndX - spineStartX - arrowSize
	branchAngle := fc.config.BranchAngle * math.Pi / 180

	// At narrow widths, reduce branch angle to keep branches more vertical,
	// which uses less horizontal space and prevents overlap.
	if isNarrow {
		branchAngle = 20 * math.Pi / 180
	}

	// Distribute categories along the spine, alternating top and bottom
	numTop := (numVisible + 1) / 2
	numBottom := numVisible / 2

	topSpacing := spineLength / float64(numTop+1)
	bottomSpacing := spineLength / float64(numBottom+1)

	// Branch length scales with height but is also constrained by width
	// to prevent branches from extending beyond the viewBox at narrow sizes.
	branchLen := plotArea.H * 0.35
	maxBranchLen := plotArea.W * 0.25 // prevent horizontal overflow
	if branchLen*math.Sin(branchAngle) > maxBranchLen {
		branchLen = maxBranchLen / math.Sin(branchAngle)
	}

	// Density-aware cause capping: count total causes across all categories
	// and reduce the per-category limit when the diagram is too dense.
	totalCauses := 0
	for _, cat := range visibleCats {
		totalCauses += len(cat.Causes)
	}

	maxCausesPerCat := 10 // default generous limit
	switch {
	case isNarrow:
		maxCausesPerCat = 2
	case totalCauses > 24:
		maxCausesPerCat = 2
	case totalCauses > 18:
		maxCausesPerCat = 3
	case numVisible >= 6:
		maxCausesPerCat = 4
	}

	// ─── Pass 1: compute branch layouts and category label bounding boxes ──
	layouts := make([]fishboneBranchLayout, numVisible)
	topIdx := 0
	bottomIdx := 0

	for i, cat := range visibleCats {
		isTop := i%2 == 0
		var branchX float64
		if isTop {
			topIdx++
			branchX = spineStartX + float64(topIdx)*topSpacing
		} else {
			bottomIdx++
			branchX = spineStartX + float64(bottomIdx)*bottomSpacing
		}

		direction := -1.0
		if !isTop {
			direction = 1.0
		}

		branchEndX := branchX - branchLen*math.Sin(branchAngle)
		branchEndY := spineY + direction*branchLen*math.Cos(branchAngle)

		// Clamp branch endpoints to viewBox bounds
		if branchEndX < plotArea.X {
			branchEndX = plotArea.X
		}
		if branchEndY < plotArea.Y {
			branchEndY = plotArea.Y + smallFontSize
		}
		if branchEndY > plotArea.Y+plotArea.H {
			branchEndY = plotArea.Y + plotArea.H - smallFontSize
		}

		// Measure category label bounding box
		catName := cat.Name
		catFontSize := smallFontSize

		// Category labels alternate top/bottom, so the effective horizontal
		// density is per-side count, not total count. Use the larger of the
		// two sides (numTop) as the divisor.
		perSideCount := float64(numTop)
		if perSideCount < 1 {
			perSideCount = 1
		}
		maxCatBoxW := spineLength / perSideCount * 0.90

		// Use LabelFitStrategy for consistent shrink → wrap → truncate cascade.
		innerPad := style.Spacing.MD * 2 * widthScale
		innerW := maxCatBoxW - innerPad
		if innerW < 0 {
			innerW = 0
		}
		catMaxH := catFontSize*3*1.4 + style.Spacing.SM*2*widthScale // room for 3 wrapped lines
		// Use a 9pt floor for category labels to maintain stable box sizes;
		// smaller boxes cascade into unstable branch/cause positioning.
		// At very narrow widths, allow 7pt to prevent severe truncation.
		catFloor := 9.0
		if isVeryNarrow {
			catFloor = 7.0
		}
		catMinFont := math.Max(catFloor, b.MinFontSize())
		catLabelFit := LabelFitStrategy{
			PreferredSize: catFontSize,
			MinSize:       catMinFont,
			AllowWrap:     true,
			MaxLines:      3,
			MinCharWidth:  4.0,
		}
		catResult := catLabelFit.Fit(b, catName, innerW, catMaxH)
		catFontSize = catResult.FontSize
		catWrapped := catResult.Wrapped

		var catBoxW, catBoxH float64
		if catWrapped {
			b.Push()
			b.SetFontSize(catFontSize)
			block := b.WrapText(catName, innerW)
			b.Pop()
			catBoxW = maxCatBoxW
			catBoxH = block.TotalHeight + style.Spacing.SM*2*widthScale
		} else {
			b.Push()
			b.SetFontSize(catFontSize)
			catNameW, _ := b.MeasureText(catResult.DisplayText)
			b.Pop()
			catBoxW = catNameW + innerPad
			catBoxH = catFontSize + style.Spacing.SM*2*widthScale
			if catBoxW > maxCatBoxW {
				catBoxW = maxCatBoxW
			}
		}

		catBoxX := branchEndX - catBoxW/2
		catBoxY := branchEndY - catBoxH/2

		if catBoxX < plotArea.X {
			catBoxX = plotArea.X
		}
		if catBoxX+catBoxW > plotArea.X+plotArea.W {
			catBoxX = plotArea.X + plotArea.W - catBoxW
		}

		layouts[i] = fishboneBranchLayout{
			catIndex:   i,
			isTop:      isTop,
			branchX:    branchX,
			endX:       branchEndX,
			endY:       branchEndY,
			catName:    catName,
			fontSize:   catFontSize,
			boxRect:    Rect{X: catBoxX, Y: catBoxY, W: catBoxW, H: catBoxH},
			catWrapped: catWrapped,
		}
	}

	// ─── Pass 2: collision detection and resolution ─────────────────────────
	fc.resolveCollisions(layouts, b, plotArea, spineStartX, spineEndX, arrowSize, spineY, branchAngle, branchLen, smallFontSize, widthScale, style)

	// ─── Pass 3: draw branches and labels using resolved positions ──────────
	for _, lay := range layouts {
		cat := visibleCats[lay.catIndex]
		color := accentColors[lay.catIndex%len(accentColors)]

		// Draw branch line from spine to resolved endpoint
		b.Push()
		b.SetStrokeColor(color)
		b.SetStrokeWidth(branchWidth)
		b.DrawLine(lay.branchX, spineY, lay.endX, lay.endY)
		b.Pop()

		// Draw category label box
		b.Push()
		b.SetFillColor(color.WithAlpha(0.2))
		b.SetStrokeColor(color)
		b.SetStrokeWidth(1.5)
		b.DrawRoundedRect(lay.boxRect, fc.config.CornerRadius)
		b.Pop()

		b.Push()
		b.SetFontSize(lay.fontSize)
		b.SetFontWeight(style.Typography.WeightBold)
		if lay.catWrapped {
			innerRect := lay.boxRect.Inset(style.Spacing.XS*widthScale, style.Spacing.XS*widthScale, style.Spacing.XS*widthScale, style.Spacing.XS*widthScale)
			b.DrawWrappedText(lay.catName, innerRect, AlignCenter)
		} else {
			b.DrawText(lay.catName, lay.boxRect.X+lay.boxRect.W/2, lay.endY, TextAlignCenter, TextBaselineMiddle)
		}
		b.Pop()

		// At very narrow widths, skip individual cause labels entirely and
		// show a compact "(N causes)" badge below the category box. This
		// prevents unreadable truncated text that makes categories appear
		// missing.
		if isVeryNarrow && len(cat.Causes) > 0 {
			badgeText := fmt.Sprintf("(%d causes)", len(cat.Causes))
			badgeFontSize := math.Max(captionFontSize*0.85, b.MinFontSize())
			badgeY := lay.boxRect.Y + lay.boxRect.H + badgeFontSize*1.2
			if !lay.isTop {
				// Bottom branches: badge goes above the box
				badgeY = lay.boxRect.Y - badgeFontSize*0.5
			}
			b.Push()
			b.SetFontSize(badgeFontSize)
			b.SetFillColor(style.Palette.TextSecondary)
			b.DrawText(badgeText, lay.boxRect.X+lay.boxRect.W/2, badgeY, TextAlignCenter, TextBaselineMiddle)
			b.Pop()
			continue
		}

		// Draw individual causes as horizontal sub-branches off the bone.
		subFontSize := captionFontSize
		subTickLen := math.Max(style.Spacing.MD, branchLen*math.Sin(branchAngle)*0.45*widthScale)

		causes := cat.Causes
		hiddenCount := 0
		if len(causes) > maxCausesPerCat {
			hiddenCount = len(causes) - maxCausesPerCat
			causes = causes[:maxCausesPerCat]
		}

		type causeEntry struct {
			text    string
			isExtra bool
		}
		displayCauses := make([]causeEntry, 0, len(causes)+1)
		for _, c := range causes {
			displayCauses = append(displayCauses, causeEntry{text: c})
		}
		if hiddenCount > 0 {
			displayCauses = append(displayCauses, causeEntry{
				text:    fmt.Sprintf("… +%d more", hiddenCount),
				isExtra: true,
			})
		}

		// Compute minimum vertical gap required between cause labels.
		// Use 2.8x the font size so that labels can wrap to 2 lines when
		// the horizontal space is too narrow for the full text. At the 9pt
		// font floor, 2 wrapped lines at 7pt causeMinFont need ~22.8pt
		// (WrapText line height ≈ 1.565 × fontSize × 2 lines ≈ 21.9pt,
		// plus inter-line spacing). 2.8× gives causeMaxH = 9 × 2.8 × 0.95
		// = 23.94pt, enough for 2 wrapped lines with comfortable margin.
		minVerticalGap := subFontSize * 2.8
		branchVerticalSpan := math.Abs(lay.endY - spineY)

		// Maximum items that can fit without vertical overlap along this branch.
		// We use the branch vertical span (excluding the category box area at the
		// end), divided by the minimum gap per item. Reserve 15% of the branch
		// for the category box region.
		usableBranchSpan := branchVerticalSpan * 0.85
		maxFittable := int(usableBranchSpan / minVerticalGap)
		if maxFittable < 1 {
			maxFittable = 1
		}

		// If we can't fit all display causes, cap to what fits and add overflow.
		if len(displayCauses) > maxFittable {
			// Keep the first (maxFittable - 1) causes and add overflow indicator
			extraHidden := len(displayCauses) - maxFittable
			if maxFittable > 1 {
				displayCauses = displayCauses[:maxFittable-1]
			} else {
				displayCauses = displayCauses[:1]
				extraHidden = len(causes) - 1 + hiddenCount
			}
			if extraHidden > 0 {
				displayCauses = append(displayCauses, causeEntry{
					text:    fmt.Sprintf("… +%d more", extraHidden),
					isExtra: true,
				})
			}
		}

		// Compute the maximum horizontal extent for cause labels. Limit labels
		// so they don't extend into the territory of adjacent branches on the
		// same side. The available width is bounded by the horizontal distance
		// to the nearest adjacent branch's sub-tick endpoint.
		//
		// The inter-branch factor is 0.95 (not lower) because cause labels on
		// adjacent branches sit at different Y positions along angled branches,
		// so slight horizontal overlap in bounding boxes doesn't produce visual
		// collision. The previous 0.80 factor was too conservative and forced
		// truncation of moderate-length labels (e.g., "Complex escalation
		// procedures" at 600–760pt widths with 6 categories).
		maxCauseLabelW := subTickLen*4.5 + style.Spacing.LG*widthScale
		for _, other := range layouts {
			if other.isTop != lay.isTop || other.catIndex == lay.catIndex {
				continue
			}
			hDist := math.Abs(lay.branchX - other.branchX)
			if hDist > 0 && hDist < maxCauseLabelW*1.5 {
				maxCauseLabelW = math.Min(maxCauseLabelW, hDist*0.95)
			}
		}

		for j, entry := range displayCauses {
			t := float64(j+1) / float64(len(displayCauses)+1)
			subX := lay.branchX + t*(lay.endX-lay.branchX)
			subY := spineY + t*(lay.endY-spineY)

			subEndX := subX - subTickLen
			subEndY := subY

			if subEndX < plotArea.X {
				subEndX = plotArea.X
			}

			if !entry.isExtra {
				b.Push()
				b.SetStrokeColor(color.WithAlpha(0.6))
				b.SetStrokeWidth(1.5 * math.Max(0.6, widthScale))
				b.DrawLine(subX, subY, subEndX, subEndY)
				b.Pop()
			}

			// Use the full inter-branch width for text fitting. The label
			// is positioned centered at the sub-tick midpoint, then clamped
			// so it stays within the plot area. This avoids truncation when
			// neither left nor right space alone is enough but the combined
			// space exceeds maxCauseLabelW.
			causeMinFont := math.Min(DefaultMinFontSize, 7.0)
			causeMaxH := minVerticalGap * 0.95
			availableW := maxCauseLabelW

			causeFit := LabelFitStrategy{PreferredSize: subFontSize, MinSize: causeMinFont, MinCharWidth: 5.5, AllowWrap: true, MaxLines: 3}

			origMin := b.MinFontSize()
			b.SetMinFontSize(causeMinFont)
			causeResult := causeFit.Fit(b, entry.text, availableW, causeMaxH)
			b.SetMinFontSize(origMin)

			// Position the label box centered at the sub-tick midpoint,
			// then clamp to plot bounds.
			tickMidX := (subX + subEndX) / 2
			labelBoxX := tickMidX - availableW/2
			if labelBoxX < plotArea.X {
				labelBoxX = plotArea.X
			}
			if labelBoxX+availableW > plotArea.X+plotArea.W {
				labelBoxX = plotArea.X + plotArea.W - availableW
			}

			b.Push()
			b.SetFontSize(causeResult.FontSize)
			if entry.isExtra {
				b.SetFillColor(style.Palette.TextSecondary)
			} else {
				b.SetFillColor(style.Palette.TextPrimary)
			}
			if causeResult.Wrapped {
				wrapRect := Rect{
					X: labelBoxX,
					Y: subEndY - causeMaxH/2,
					W: availableW,
					H: causeMaxH,
				}
				b.DrawWrappedText(causeResult.DisplayText, wrapRect, AlignCenter)
			} else {
				// Center-align at the midpoint of the label box
				b.DrawText(causeResult.DisplayText, labelBoxX+availableW/2, subEndY, TextAlignCenter, TextBaselineMiddle)
			}
			b.Pop()
		}
	}

	// Draw overflow indicator if categories were collapsed
	if overflowCount > 0 {
		overflowText := fmt.Sprintf("+%d more categories", overflowCount)
		overflowFontSize := captionFontSize
		overflowY := spineY + smallFontSize*1.5
		overflowX := spineStartX + spineLength*0.05

		b.Push()
		b.SetFontSize(overflowFontSize)
		b.SetFillColor(style.Palette.TextSecondary)
		b.DrawText(overflowText, overflowX, overflowY, TextAlignLeft, TextBaselineMiddle)
		b.Pop()
	}

	// Draw title
	if fc.config.ShowTitle && data.Title != "" {
		titleConfig := DefaultTitleConfig()
		titleConfig.Text = data.Title
		titleConfig.Subtitle = data.Subtitle
		title := NewTitle(b, titleConfig)
		title.Draw(Rect{X: 0, Y: 0, W: fc.config.Width, H: headerHeight + fc.config.MarginTop})
	}

	return nil
}

// resolveCollisions detects and fixes overlapping category label bounding boxes.
// It operates on same-side (top or bottom) labels independently because labels
// on opposite sides of the spine cannot collide with each other.
//
// Resolution strategy (applied in order):
//  1. Spread X positions further apart along the spine.
//  2. If still overlapping, reduce label font size (down to a floor).
//  3. Re-measure and re-position after each adjustment.
//nolint:gocognit // complex chart rendering logic
func (fc *FishboneChart) resolveCollisions(
	layouts []fishboneBranchLayout,
	b *SVGBuilder,
	plotArea Rect,
	spineStartX, spineEndX, arrowSize, spineY, branchAngle, branchLen, smallFontSize, widthScale float64,
	style *StyleGuide,
) {
	// Separate into top and bottom groups
	var topIdxs, bottomIdxs []int
	for i := range layouts {
		if layouts[i].isTop {
			topIdxs = append(topIdxs, i)
		} else {
			bottomIdxs = append(bottomIdxs, i)
		}
	}

	resolveSide := func(idxs []int) {
		if len(idxs) < 2 {
			return
		}

		const maxPasses = 3
		for pass := 0; pass < maxPasses; pass++ {
			hasOverlap := false
			for a := 0; a < len(idxs)-1; a++ {
				for c := a + 1; c < len(idxs); c++ {
					if layouts[idxs[a]].boxRect.Intersects(layouts[idxs[c]].boxRect) {
						hasOverlap = true
						break
					}
				}
				if hasOverlap {
					break
				}
			}

			if !hasOverlap {
				return
			}

			if pass == 0 {
				// Strategy 1: Spread X positions further apart.
				// Re-distribute using more of the available spine length.
				spineLength := spineEndX - spineStartX - arrowSize
				n := len(idxs)
				newSpacing := spineLength / float64(n+1)

				for rank, idx := range idxs {
					lay := &layouts[idx]
					lay.branchX = spineStartX + float64(rank+1)*newSpacing

					direction := -1.0
					if !lay.isTop {
						direction = 1.0
					}
					lay.endX = lay.branchX - branchLen*math.Sin(branchAngle)
					lay.endY = spineY + direction*branchLen*math.Cos(branchAngle)

					if lay.endX < plotArea.X {
						lay.endX = plotArea.X
					}
					if lay.endY < plotArea.Y {
						lay.endY = plotArea.Y + smallFontSize
					}
					if lay.endY > plotArea.Y+plotArea.H {
						lay.endY = plotArea.Y + plotArea.H - smallFontSize
					}

					// Recompute box position
					lay.boxRect.X = lay.endX - lay.boxRect.W/2
					lay.boxRect.Y = lay.endY - lay.boxRect.H/2

					if lay.boxRect.X < plotArea.X {
						lay.boxRect.X = plotArea.X
					}
					if lay.boxRect.X+lay.boxRect.W > plotArea.X+plotArea.W {
						lay.boxRect.X = plotArea.X + plotArea.W - lay.boxRect.W
					}
				}
			} else {
				// Strategy 2: Reduce font size and re-measure labels.
				for _, idx := range idxs {
					lay := &layouts[idx]
					newSize := lay.fontSize * 0.85
					// At very narrow widths, allow smaller fonts to prevent
					// severe truncation that makes categories appear missing.
					collisionFloor := 9.0
					if plotArea.W < 500 {
						collisionFloor = 7.0
					}
					minSize := math.Max(collisionFloor, b.MinFontSize()) // stable category box sizes
					if newSize < minSize {
						newSize = minSize
					}
					if newSize >= lay.fontSize {
						continue // already at floor
					}
					lay.fontSize = newSize

					// Re-measure text width at new font size
					b.Push()
					b.SetFontSize(lay.fontSize)
					catNameW, _ := b.MeasureText(lay.catName)
					b.Pop()
					catNameW *= 1.3

					catBoxW := catNameW + style.Spacing.MD*2*widthScale
					catBoxH := lay.fontSize + style.Spacing.SM*2*widthScale

					lay.boxRect.W = catBoxW
					lay.boxRect.H = catBoxH
					lay.boxRect.X = lay.endX - catBoxW/2
					lay.boxRect.Y = lay.endY - catBoxH/2

					if lay.boxRect.X < plotArea.X {
						lay.boxRect.X = plotArea.X
					}
					if lay.boxRect.X+lay.boxRect.W > plotArea.X+plotArea.W {
						lay.boxRect.X = plotArea.X + plotArea.W - lay.boxRect.W
					}
				}
			}
		}
	}

	resolveSide(topIdxs)
	resolveSide(bottomIdxs)
}

// =============================================================================
// Fishbone Diagram Interface
// =============================================================================

// FishboneDiagram implements the Diagram interface for fishbone diagrams.
type FishboneDiagram struct{ BaseDiagram }

// Validate checks that the request data is valid for a fishbone diagram.
func (d *FishboneDiagram) Validate(req *RequestEnvelope) error {
	data := req.Data

	// Normalize: accept "problem" as alias for "effect" (common in JSON input)
	if _, hasEffect := data["effect"]; !hasEffect {
		if problem, hasProblem := data["problem"]; hasProblem {
			data["effect"] = problem
		} else {
			return fmt.Errorf("fishbone requires 'effect' (or 'problem') field. Expected format: {\"effect\": \"Low Sales\", \"causes\": [{\"category\": \"People\", \"items\": [\"Understaffed\"]}]}")
		}
	}

	return nil
}

// Render generates an SVG document for the fishbone diagram.
func (d *FishboneDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}

// RenderWithBuilder implements DiagramWithBuilder for multi-format support.
func (d *FishboneDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	return RenderWithHelper(req, func(builder *SVGBuilder, req *RequestEnvelope) error {
		fbData, err := extractFishboneData(req)
		if err != nil {
			return err
		}

		width, height := builder.Width(), builder.Height()
		config := DefaultFishboneConfig(width, height)
		config.ShowTitle = req.Title != ""

		chart := NewFishboneChart(builder, config)
		if err := chart.Draw(fbData); err != nil {
			return fmt.Errorf("fishbone render failed: %w", err)
		}
		return nil
	})
}

// extractFishboneData extracts FishboneData from a request envelope.
func extractFishboneData(req *RequestEnvelope) (FishboneData, error) {
	data := req.Data
	fbData := FishboneData{
		Title:    req.Title,
		Subtitle: req.Subtitle,
	}

	if effect, ok := data["effect"].(string); ok {
		fbData.Effect = effect
	}

	if categories, ok := data["categories"]; ok {
		catSlice, ok := categories.([]any)
		if !ok {
			return fbData, fmt.Errorf("fishbone 'categories' must be an array")
		}

		for _, catItem := range catSlice {
			catMap, ok := catItem.(map[string]any)
			if !ok {
				continue
			}

			cat := FishboneCategory{}
			if name, ok := catMap["name"].(string); ok {
				cat.Name = name
			} else if label, ok := catMap["label"].(string); ok {
				cat.Name = label
			}

			if causes, ok := catMap["causes"]; ok {
				if causeSlice, ok := toStringSlice(causes); ok {
					cat.Causes = causeSlice
				}
			}

			fbData.Categories = append(fbData.Categories, cat)
		}
	}

	return fbData, nil
}

