package generator

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// textClaimedShapes maps each shape index that receives a text block to the
// content index of the first such block. It mirrors populateTextInSlide's
// resolution so prepareImages can tell when a picture would replace a shape
// that already carries the author's copy.
func textClaimedShapes(shapes []shapeXML, content []ContentItem, layoutID string) map[int]int {
	claims := make(map[int]int)
	resolver := newPlaceholderResolver(shapes, layoutID)
	for j, item := range content {
		if item.Type == ContentImage || item.Type == ContentDiagram || item.Type == ContentTable || !contentItemHasText(item) {
			continue
		}
		idx, _, found := resolver.ResolveWithFallback(item.PlaceholderID)
		if !found {
			continue
		}
		if _, taken := claims[idx]; !taken {
			claims[idx] = j
		}
	}
	return claims
}

// imageFallbackShape picks a home for an image whose placeholder_id the
// layout does not declare: the layout's picture placeholder first, then its
// largest free body/content placeholder. Shapes already holding text or a
// visual are skipped, so the fallback never paints over the author's copy.
//
// Nine of ten shipped templates have no picture placeholder, so an image
// authored as placeholder_id "image" used to be CONTENT_DROPPED on any of
// them; on modern the reverse ("body" on a picture-only layout) failed the
// same way (go-slide-creator-f0l85).
func imageFallbackShape(shapes []shapeXML, textClaims, visualClaims map[int]int) (int, bool) {
	free := func(i int) bool {
		_, t := textClaims[i]
		_, v := visualClaims[i]
		return !t && !v
	}
	for i := range shapes {
		ph := shapes[i].NonVisualProperties.NvPr.Placeholder
		if ph != nil && ph.Type == "pic" && free(i) {
			return i, true
		}
	}
	best, bestArea := -1, int64(-1)
	for i := range shapes {
		if !isContentPlaceholder(&shapes[i]) || !free(i) {
			continue
		}
		if a := shapeArea(&shapes[i]); a > bestArea {
			best, bestArea = i, a
		}
	}
	return best, best >= 0
}

// resolveImageShape resolves an image content block to a shape, falling back
// to imageFallbackShape when the authored placeholder_id is absent. remapped
// reports that the fallback was used.
func resolveImageShape(resolver *placeholderResolver, shapes []shapeXML, placeholderID string, textClaims, visualClaims map[int]int) (idx int, tier ResolutionTier, found, remapped bool) {
	idx, tier, found = resolver.ResolveWithFallback(placeholderID)
	if found {
		return idx, tier, true, false
	}
	idx, found = imageFallbackShape(shapes, textClaims, visualClaims)
	if !found {
		return 0, 0, false, false
	}
	return idx, TierSemantic, true, true
}

// imageRemappedFinding is the info finding for an image routed to a different
// placeholder than the one authored.
func imageRemappedFinding(slideIndex, contentIdx int, from, to string, available []string) patterns.FitFinding {
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path:    slidepath.ContentField(slideIndex, contentIdx, "placeholder_id"),
			Code:    patterns.ErrCodePlaceholderRemapped,
			Message: fmt.Sprintf("slide %d: image placeholder %q remapped to %q (the layout has no %q placeholder)", slideIndex+1, from, to, from),
			Fix: &patterns.FixSuggestion{
				Kind:   "remap_placeholder",
				Params: map[string]any{"from": from, "to": to, "available_placeholders": available},
			},
		},
		Action: "info",
	}
}

// textReplacedByImageFinding reports a text block whose shape a picture
// replaced. The picture swaps the placeholder for a p:pic, so the text is
// not rendered — most often a title that fell back to the only body shape of
// a title-less layout (go-slide-creator-2hkgy).
func textReplacedByImageFinding(slideIndex, textIdx, imageIdx int, item ContentItem, layoutID string) patterns.FitFinding {
	locator := fmt.Sprintf("content block %d (%s)", textIdx+1, item.Type)
	return patterns.ContentDroppedPlaceholderOccupied(slidepath.ContentIndex(slideIndex, textIdx), locator,
		item.PlaceholderID, layoutID, imageIdx, "picture, which replaces the placeholder and its text")
}

// resolveVisualShape resolves a visual content block for prepareImages. An
// image falls back to imageFallbackShape when its placeholder_id is absent
// (reporting the remap), and a text block the picture would replace is
// reported as CONTENT_DROPPED (go-slide-creator-2hkgy).
func (ctx *singlePassContext) resolveVisualShape(resolver *placeholderResolver, shapes []shapeXML, spec SlideSpec, slideNum, contentIdx int, textClaims, visualClaims map[int]int) (int, ResolutionTier, bool) {
	item := spec.Content[contentIdx]
	if item.Type != ContentImage {
		return resolver.ResolveWithFallback(item.PlaceholderID)
	}
	shapeIdx, tier, found, remapped := resolveImageShape(resolver, shapes, item.PlaceholderID, textClaims, visualClaims)
	if !found {
		return 0, 0, false
	}
	if remapped {
		ctx.emitFitFinding(imageRemappedFinding(slideNum-1, contentIdx, item.PlaceholderID,
			shapes[shapeIdx].NonVisualProperties.ConnectionNonVisual.Name, resolver.Keys()))
	}
	if textIdx, taken := textClaims[shapeIdx]; taken {
		ctx.emitFitFinding(textReplacedByImageFinding(slideNum-1, textIdx, contentIdx, spec.Content[textIdx], spec.LayoutID))
	}
	return shapeIdx, tier, true
}
