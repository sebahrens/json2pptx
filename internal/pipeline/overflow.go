package pipeline

import (
	"strings"

	"github.com/ahrens/go-slide-creator/internal/layout"
	"github.com/ahrens/go-slide-creator/internal/textfit"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// estimateBodyOverflow checks whether the body text content of a slide would
// overflow the body placeholder of the selected layout, using the same textfit
// engine that the generator uses. Returns true if overflow is estimated.
//
// This is a pre-flight check: it uses placeholder bounds from the template
// analysis (not the actual slide XML), so it may differ slightly from the
// generator's final text population. It is intentionally conservative.
func estimateBodyOverflow(slide types.SlideDefinition, selection *layout.SelectionResult, layouts []types.LayoutMetadata) bool {
	// Find the body/content placeholder for the selected layout
	bodyPH := findBodyPlaceholder(selection.LayoutID, layouts)
	if bodyPH == nil {
		return false // No body placeholder — can't overflow
	}

	// Collect the text that will go into the body placeholder
	paragraphs := collectBodyParagraphs(slide, selection.Mappings)
	if len(paragraphs) == 0 {
		return false
	}

	// Use placeholder font properties (or defaults)
	fontSizeHPt := bodyPH.FontSize
	if fontSizeHPt <= 0 {
		fontSizeHPt = 2000 // 20pt default
	}
	fontName := bodyPH.FontFamily
	if fontName == "" {
		fontName = "Arial"
	}

	result := textfit.Calculate(textfit.Params{
		WidthEMU:    bodyPH.Bounds.Width,
		HeightEMU:   bodyPH.Bounds.Height,
		FontSizeHPt: fontSizeHPt,
		FontName:    fontName,
		Paragraphs:  paragraphs,
	})

	return result.Overflow
}

// findBodyPlaceholder returns the first body or content placeholder for the given layout.
func findBodyPlaceholder(layoutID string, layouts []types.LayoutMetadata) *types.PlaceholderInfo {
	for i := range layouts {
		if layouts[i].ID != layoutID {
			continue
		}
		for j := range layouts[i].Placeholders {
			ph := &layouts[i].Placeholders[j]
			if ph.Type == types.PlaceholderBody || ph.Type == types.PlaceholderContent {
				return ph
			}
		}
	}
	return nil
}

// collectBodyParagraphs extracts the text paragraphs that would be placed in the
// body placeholder, based on content mappings. This mirrors what BuildContentItems
// does but only extracts text strings for textfit estimation.
func collectBodyParagraphs(slide types.SlideDefinition, mappings []layout.ContentMapping) []string {
	var paragraphs []string

	for _, m := range mappings {
		switch m.ContentField {
		case "body":
			if slide.Content.Body != "" {
				paragraphs = append(paragraphs, splitParagraphs(slide.Content.Body)...)
			}
		case "bullets", "body_and_bullets":
			if slide.Content.Body != "" {
				paragraphs = append(paragraphs, splitParagraphs(slide.Content.Body)...)
			}
			paragraphs = append(paragraphs, slide.Content.Bullets...)
			for _, bg := range slide.Content.BulletGroups {
				if bg.Header != "" {
					paragraphs = append(paragraphs, bg.Header)
				}
				paragraphs = append(paragraphs, bg.Bullets...)
			}
			if slide.Content.BodyAfterBullets != "" {
				paragraphs = append(paragraphs, splitParagraphs(slide.Content.BodyAfterBullets)...)
			}
		}
	}

	return paragraphs
}

// splitParagraphs splits text on newlines into separate paragraphs.
func splitParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	for _, l := range lines {
		result = append(result, strings.TrimSpace(l))
	}
	return result
}
