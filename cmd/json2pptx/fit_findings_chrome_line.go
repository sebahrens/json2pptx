package main

import (
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// collectChromeLineFindings predicts CHROME_TRUNCATED: the left footer line
// (chrome fields, footer.left_text, section crumb) measured against the footer
// slot of every slide that carries it, with the same fitter generation runs
// (go-slide-creator-m2tlt). The footer line used to lose its tail to an
// ellipsis on a narrow slot — a client name cut on every slide — while
// validation and the score both called the deck clean.
//
// Returns one deck-level finding per distinct shortfall (usually one: the
// content slides share a footer slot); nil without layouts or a footer.
func collectChromeLineFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64, theme *types.ThemeInfo) []patterns.FitFinding {
	if input == nil || len(layouts) == 0 || len(input.Slides) == 0 {
		return nil
	}
	footer := footerConfigForInput(input, len(input.Slides))
	if footer == nil || !footer.Enabled {
		return nil
	}
	predicted := predictSlideLayouts(input, layouts)
	specs := make([]generator.SlideSpec, len(input.Slides))
	for si, layout := range predicted {
		if layout != nil {
			specs[si].LayoutID = layout.ID
		}
	}
	// The same chrome setup a render applies (deckGenerationRequest), so the
	// line, the skipped slides and the page-number width are generation's.
	if input.Chrome != nil {
		applyChromeSkip(specs, input.Chrome, input.Slides, layouts)
		applyChromeSectionCrumb(footer, specs, input.Chrome, input.Slides, layouts)
	}
	applyAppendixPageLabels(footer, input.Slides, layouts)

	reference := template.ChromeReferenceLayout(layouts)
	in := generator.FooterLineInput{Config: footer, SlideWidth: slideWidth, SlideHeight: slideHeight}
	for si, layout := range predicted {
		if layout == nil || specs[si].SkipFooter {
			continue
		}
		in.Slides = append(in.Slides, generator.FooterLineSlide{
			SlideIndex: si,
			LayoutID:   layout.ID,
			Regions:    layout.FooterRegions,
			Frame:      template.ResolveChromeFrame(layout, reference, slideWidth, slideHeight, false, false),
		})
		if in.TemplatePath == "" {
			in.TemplatePath = layout.TemplatePath
		}
	}
	if theme != nil {
		in.FontName = theme.BodyFont
	}
	return generator.PredictFooterTruncation(in)
}
