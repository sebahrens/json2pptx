package generator

import (
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// fitNativeFramework is fitNativeFrameworkAt for a body placeholder: the
// sparse finding, if any, is emitted against the authored content item.
func (ctx *singlePassContext) fitNativeFramework(slideNum, contentIdx int, kind string, bounds types.BoundingBox, panels []nativePanelData, meta houseDiagramMeta) types.BoundingBox {
	fit, f := fitNativeFrameworkAt(kind, bounds, panels, meta, ctx.themeFontName, nativeDiagramSite{
		slideIndex: slideNum - 1, path: slidepath.ContentIndex(slideNum-1, contentIdx),
	})
	if f != nil {
		ctx.emitFitFinding(*f)
	}
	return fit
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

// houseContentHeight is the height the shared house builder gives the house
// in bounds: its content-sized block, and the text height inside it.
func houseContentHeight(panels []nativePanelData, meta houseDiagramMeta, bounds types.BoundingBox, fontName string) nativeFrameworkHeight {
	if len(panels) < 2 || len(meta.floors) == 0 {
		return nativeFrameworkHeight{}
	}
	layout, err := layoutHouse(panels, meta, bounds, nativeDiagramEnv{fontName: fontName})
	if err != nil {
		return nativeFrameworkHeight{}
	}
	box := int64(layout.HeightPt * float64(types.EMUPerPoint))
	if layout.Tight {
		// A house short of height needs all of it.
		box = bounds.Height
	}
	return nativeFrameworkHeight{box: box, ink: int64(layout.InkPt * float64(types.EMUPerPoint))}
}
