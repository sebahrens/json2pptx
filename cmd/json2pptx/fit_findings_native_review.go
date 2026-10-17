package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

type nativeLayoutReviewStats struct {
	body, caption, title float64
	visuals              int
	visualPath           string
	visualArea           float64
}

func collectNativeLayoutReviewFindings(input *PresentationInput, layouts []types.LayoutMetadata, w, h int64) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	predicted := predictSlideLayouts(input, layouts)
	var out []patterns.FitFinding
	for si, slide := range input.Slides {
		if !slideCarriesArgument(slide, layouts...) || si >= len(predicted) || predicted[si] == nil {
			continue
		}
		out = append(out, nativeSlideLayoutReview(si, slide, predicted[si], layouts, w, h)...)
	}
	return out
}

func nativeSlideLayoutReview(si int, slide SlideInput, layout *types.LayoutMetadata, layouts []types.LayoutMetadata, w, h int64) []patterns.FitFinding {
	var stats nativeLayoutReviewStats
	var out []patterns.FitFinding
	for ci := range slide.Content {
		item := &slide.Content[ci]
		ph := findPlaceholderByID(item.PlaceholderID, layout.Placeholders)
		if ph == nil {
			continue
		}
		path := slidepath.ContentIndex(si, ci)
		if ph.Type == types.PlaceholderTitle || isTitlePlaceholderID(item.PlaceholderID) {
			size, finding := nativeTitleLayoutReview(item, ph, path)
			stats.title = size
			if finding != nil {
				out = append(out, *finding)
			}
			continue
		}
		stats.record(item, ph, path)
	}
	if stats.title > 0 && stats.body > 0 && stats.title <= stats.body+2 || stats.caption > 0 && stats.body > 0 && stats.caption >= stats.body {
		out = append(out, layoutReviewFinding(patterns.ErrCodeTypeHierarchyFlat, slidepath.Slide(si), "review", "The resolved title/body/caption type hierarchy is flat or inverted; retain a title more than 2pt above body and captions smaller than body.", map[string]any{"title_pt": stats.title, "body_pt": stats.body, "caption_pt": stats.caption, "template_layout": layout.ID}))
	}
	if stats.visuals == 1 && slide.ShapeGrid == nil && slide.Pattern == nil && slide.Compose == nil {
		geom := resolveGridGeometry(slide, layouts, w, h)
		safe := contentRelativeBoundsBase(geom.OverrideBounds, geom.Zone, w, h)
		area := float64(safe.CX) * float64(safe.CY)
		if area > 0 && stats.visualArea/area < 0.35 {
			out = append(out, layoutReviewFinding(patterns.ErrCodeExhibitTooSmall, stats.visualPath, "review", fmt.Sprintf("The evidence visual occupies %.0f%% of the content area; enlarge it or give it a dedicated slide.", 100*stats.visualArea/area), map[string]any{"visual_area_share": stats.visualArea / area, "min_area_share": 0.35}))
		}
	}
	return out
}

func (s *nativeLayoutReviewStats) record(item *ContentInput, ph *types.PlaceholderInfo, path string) {
	switch item.Type {
	case "image", "chart", "diagram":
		s.visuals++
		s.visualPath = path
		s.visualArea = float64(ph.Bounds.Width) * float64(ph.Bounds.Height)
	}
	switch item.Type {
	case "text", "bullets", "body_and_bullets", "bullet_groups":
	default:
		return
	}
	size := float64(ph.FontSize) / 100
	if item.FontSize != nil && *item.FontSize > 0 {
		size = *item.FontSize
	}
	if strings.Contains(item.PlaceholderID, "caption") || strings.Contains(item.PlaceholderID, "sublabel") {
		s.caption = math.Max(s.caption, size)
	} else if strings.HasPrefix(item.PlaceholderID, "body") {
		s.body = math.Max(s.body, size)
	}
}

func nativeTitleLayoutReview(item *ContentInput, ph *types.PlaceholderInfo, path string) (float64, *patterns.FitFinding) {
	text := strings.Join(extractContentParagraphs(item), " ")
	if text == "" {
		return 0, nil
	}
	effective := authoredTitlePlaceholder(ph, item)
	measurement := measureTitleInPlaceholder(text, effective)
	size := float64(measurement.renderedSizeHPt(effective.FontSize)) / 100
	m := newGeomMeasurer(&types.ThemeInfo{BodyFont: ph.FontFamily})
	if effective.TextCaps {
		text = strings.ToUpper(text)
	}
	reason := headingOrphanReason(m, geomParagraph{text: text, sizePt: size, bold: effective.TextBold}, float64(ph.Bounds.Width)/emuPerPt-14.4)
	if reason == "" {
		return size, nil
	}
	f := layoutReviewFinding(patterns.ErrCodeTitleOrphan, path, "info", reason+"; rewrite the title or keep number/unit pairs together with a non-breaking space.", nil)
	return size, &f
}
