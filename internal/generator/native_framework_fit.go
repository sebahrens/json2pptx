package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// fitNativeFramework sizes a framework to its measured text while retaining the
// authored width and hanging the result from the placeholder top. Dense frameworks
// keep their original geometry. A sparse finding describes the remaining lack
// of content, rather than suggesting a shape-grid-only repair for a diagram.
func (ctx *singlePassContext) fitNativeFramework(slideNum, contentIdx int, kind string, bounds types.BoundingBox, panels []nativePanelData, meta houseDiagramMeta) types.BoundingBox {
	if bounds.Width <= 0 || bounds.Height <= 0 || len(panels) == 0 {
		return bounds
	}
	size := nativeFrameworkSize(kind, bounds, panels, meta, ctx.themeFontName)
	if size.box <= 0 {
		return bounds
	}
	if f := DetectSparseLayout(SparseLayoutInput{
		SlideIndex: slideNum - 1, Path: slidepath.ContentIndex(slideNum-1, contentIdx),
		BoundsHeightEMU: bounds.Height, ContentHeightEMU: size.ink, AreaMeasured: size.ink == 0,
	}); f != nil {
		f.Pattern = kind
		f.Message = fmt.Sprintf("%s diagram text occupies %.0f%% of its allocated height; add supporting detail or use a smaller diagram region", kind, 100*float64(size.ink)/float64(bounds.Height))
		f.Fix = &patterns.FixSuggestion{Kind: "add_detail_or_resize", Params: map[string]any{
			"diagram_type": kind, "content_height": size.ink, "bounds_height": bounds.Height,
		}}
		ctx.emitFitFinding(*f)
	}
	return fitBoundsToFramework(bounds, size)
}

// nativeFrameworkSize measures a content-sized framework's height, or returns
// the zero size for a kind that keeps the placeholder geometry.
func nativeFrameworkSize(kind string, bounds types.BoundingBox, panels []nativePanelData, meta houseDiagramMeta, fontName string) nativeFrameworkHeight {
	if bounds.Width <= 0 || bounds.Height <= 0 || len(panels) == 0 {
		return nativeFrameworkHeight{}
	}
	switch kind {
	case "swot":
		return swotGridHeight(panels, bounds)
	case "pestel":
		return taxonomyGridHeight(panels, bounds, 3, pestelGap, pestelHeaderFontSize, pestelBodyFontSize, pestelBodyInset, pestelHeaderHeightRatio, fontName)
	case "kpi_dashboard":
		return kpiContentHeight(panels, bounds, fontName)
	case "house_diagram":
		return houseContentHeight(panels, meta, bounds, fontName)
	}
	return nativeFrameworkHeight{}
}

// fitBoundsToFramework hangs a framework of the measured height from the top
// of its body placeholder — the line native body text starts on — so a SWOT
// or five-forces slide starts its content where every other slide of the
// deck does instead of floating a thin band mid-slide
// (go-slide-creator-e17xy); a framework that needs the whole height (or
// more) keeps it.
func fitBoundsToFramework(bounds types.BoundingBox, size nativeFrameworkHeight) types.BoundingBox {
	if size.box <= 0 || size.box >= bounds.Height {
		return bounds
	}
	bounds.Height = size.box
	return bounds
}

type nativeFrameworkHeight struct{ box, ink int64 }

func taxonomyGridHeight(panels []nativePanelData, bounds types.BoundingBox, cols int, gap int64, titleFont, bodyFont int, inset int64, headerRatio float64, fontName string) nativeFrameworkHeight {
	rows := (len(panels) + cols - 1) / cols
	cellW := (bounds.Width - int64(cols-1)*gap) / int64(cols)
	if cellW <= 2*inset || rows == 0 {
		return nativeFrameworkHeight{}
	}
	var cellNeed, inkSum int64
	for _, p := range panels {
		title := max(taxonomyTitleHeight(p.title, cellW, inset, titleFont, fontName), taxonomyTitleHeight(p.title, cellW, inset, titleFont, ""))
		// Taxonomy bodies use panelBulletsParagraphs, including 6pt after
		// every bullet and a left indentation. Measure that actual contract;
		// omitting it makes a two-bullet card shrink until OOXML autofit
		// reduces 11pt text to roughly 8pt.
		bulletWidth := measureWidthEMU(cellW, inset, inset) - 24*int64(types.EMUPerPoint)
		bodyText := panelTextHeightEMU(panelBodyParagraphTexts(p.body, bodyFont), fontName, bulletWidth, bodyFont, panelBulletSpaceAfter)
		inkSum += title - 2*inset + bodyText
		body := bodyText + 2*inset
		if min := 2*panelLineHeightEMU(bodyFont) + 2*inset; body < min {
			body = min
		}
		h := max(title+body, int64(float64(body)/(1-headerRatio)))
		if h > cellNeed {
			cellNeed = h
		}
	}
	return nativeFrameworkHeight{box: int64(rows)*cellNeed + int64(rows-1)*gap, ink: inkSum / int64(cols)}
}

func taxonomyTitleHeight(title string, cellW, inset int64, fontSize int, fontName string) int64 {
	return panelTextHeightEMU([]string{title}, fontName, measureWidthEMU(cellW, inset, inset), fontSize, 0) + 2*inset
}

func kpiContentHeight(panels []nativePanelData, bounds types.BoundingBox, fontName string) nativeFrameworkHeight {
	cols, rows := kpiGridLayout(len(panels))
	if cols == 0 || rows == 0 {
		return nativeFrameworkHeight{}
	}
	cardW := (bounds.Width - int64(cols-1)*kpiGap) / int64(cols)
	if cardW <= 2*kpiInset {
		return nativeFrameworkHeight{}
	}
	width := measureWidthEMU(cardW, kpiInset, kpiInset)
	var cardNeed, inkSum int64
	for _, p := range panels {
		h := panelTextHeightEMU([]string{p.value}, fontName, width, kpiValueFontSize, kpiSpaceAfterValue)
		h += panelTextHeightEMU([]string{p.title}, fontName, width, kpiLabelFontSize, kpiSpaceAfterLabel)
		h += panelTextHeightEMU([]string{p.body}, fontName, width, kpiDeltaFontSize, 0)
		inkSum += h
		h += 2 * kpiInset
		if min := 72 * int64(types.EMUPerPoint); h < min {
			h = min
		}
		if h > cardNeed {
			cardNeed = h
		}
	}
	return nativeFrameworkHeight{box: int64(rows)*cardNeed + int64(rows-1)*kpiGap, ink: inkSum / int64(cols)}
}

func houseContentHeight(panels []nativePanelData, meta houseDiagramMeta, bounds types.BoundingBox, fontName string) nativeFrameworkHeight {
	if len(panels) < 2 {
		return nativeFrameworkHeight{}
	}
	floors := len(meta.floors)
	if floors == 0 {
		return nativeFrameworkHeight{}
	}
	labelFont, itemFont := houseTextFonts(meta)
	panelIdx := 1
	var floorNeed, inkSum int64
	for _, floor := range meta.floors {
		count := floor.sectionCount
		if count < 1 {
			count = 1
		}
		if count > houseMaxSectionsPerFloor {
			count = houseMaxSectionsPerFloor
		}
		width := (bounds.Width - int64(count-1)*housePillarGapEMU) / int64(count)
		if width <= 2*houseTextInsetEMU {
			return nativeFrameworkHeight{}
		}
		var rowInk int64
		for j := 0; j < count && panelIdx < len(panels)-1; j++ {
			p := panels[panelIdx]
			measureW := measureWidthEMU(width, houseTextInsetEMU, houseTextInsetEMU)
			h := panelTextHeightEMU([]string{p.title}, fontName, measureW, labelFont, 0)
			h += panelTextHeightEMU(panelBodyParagraphTexts(p.body, itemFont), fontName, measureW, itemFont, 0)
			rowInk += h
			h += 2 * houseTextInsetEMU
			if min := 60 * int64(types.EMUPerPoint); h < min {
				h = min
			}
			if h > floorNeed {
				floorNeed = h
			}
			panelIdx++
		}
		inkSum += rowInk / int64(count)
	}
	// The renderer gives roof and foundation their own content-sized bands;
	// a long foundation label must not inflate all of the pillar cards.
	totalGaps := int64(floors+1) * houseGapEMU
	roofNeed, foundationNeed := houseEndBandHeights(panels, bounds.Width, labelFont, fontName)
	need := roofNeed + int64(floors)*floorNeed + foundationNeed + totalGaps
	roofInk := panelTextHeightEMU([]string{panels[0].title}, fontName, measureWidthEMU(bounds.Width/2, houseTextInsetEMU, houseTextInsetEMU), labelFont, 0)
	foundationInk := panelTextHeightEMU([]string{panels[len(panels)-1].title}, fontName, measureWidthEMU(bounds.Width, houseTextInsetEMU, houseTextInsetEMU), labelFont, 0)
	return nativeFrameworkHeight{box: need, ink: roofInk + inkSum + foundationInk}
}

func houseTextFonts(meta houseDiagramMeta) (labelFont, itemFont int) {
	labelFont, itemFont = houseLabelFontSize, houseItemFontSize
	sections := 0
	for _, floor := range meta.floors {
		sections += floor.sectionCount
	}
	if sections >= 6 {
		return houseLabelFontSizeSmall, houseItemFontSizeSmall
	}
	return labelFont, itemFont
}

// Roof label placement. The roof triangle's preset text rectangle is its
// lower half, so a centred label sat on the triangle's base, touching the
// pillar row, with the upper two thirds of the roof empty
// (go-slide-creator-n0zpq). The label is an overlay text box instead,
// centred at houseRoofCentre of the roof height — inside the triangle's
// middle third — and as wide as the triangle is at the box's top edge.
const (
	houseRoofCentre      = 0.62
	houseRoofMinHeight   = 50 * int64(types.EMUPerPoint)
	houseRoofClearance   = 8 * int64(types.EMUPerPoint) // label to roof base (pillar row)
	houseRoofWidthFactor = 0.9                          // of the triangle's width at the label's top
)

// houseRoofTextBox returns the roof label box for a roof of roofH: its top
// offset from the roof's top, its height and its width (centred).
func houseRoofTextBox(title, fontName string, labelFont int, width, roofH int64) (top, height, boxW int64) {
	if roofH <= 0 || width <= 0 {
		return 0, 0, 0
	}
	centre := int64(houseRoofCentre * float64(roofH))
	height = panelTextHeightEMU([]string{"Ag"}, fontName, width, labelFont, 0)
	for range 4 {
		top = max(0, centre-height/2)
		boxW = int64(float64(width) * float64(top) / float64(roofH) * houseRoofWidthFactor)
		need := panelTextHeightEMU([]string{title}, fontName, boxW, labelFont, 0)
		if need <= height {
			break
		}
		height = need
	}
	top = max(0, centre-height/2)
	return top, height, boxW
}

// houseRoofHeight is the shortest roof (at least houseRoofMinHeight) whose
// label box keeps houseRoofClearance above the roof base: a one-line
// objective gets a shallow roof rather than a tall empty wedge.
func houseRoofHeight(title, fontName string, labelFont int, width int64) int64 {
	h := houseRoofMinHeight
	for range 60 {
		top, height, boxW := houseRoofTextBox(title, fontName, labelFont, width, h)
		if boxW > 0 && top+height <= h-houseRoofClearance {
			return h
		}
		h += h / 10
	}
	return h
}

func houseEndBandHeights(panels []nativePanelData, width int64, labelFont int, fontName string) (roof, foundation int64) {
	if len(panels) < 2 {
		return 0, 0
	}
	roof = houseRoofHeight(panels[0].title, fontName, labelFont, width)
	foundation = panelTextHeightEMU([]string{panels[len(panels)-1].title}, fontName, measureWidthEMU(width, houseTextInsetEMU, houseTextInsetEMU), labelFont, 0) + 2*houseTextInsetEMU
	return roof, foundation
}
