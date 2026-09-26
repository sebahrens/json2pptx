package generator

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// NativeImageOverlapsText checks possible image coverage, not image pixels or
// contrast. Contain can leave whitespace inside a frame: a positive result
// requires visual review, not an assertion that the text is unreadable.
func NativeImageOverlapsText(text types.PlaceholderInfo, images []types.BoundingBox) bool {
	if opaquePlaceholderFill(text) {
		return false
	}
	a := text.Bounds
	if a.Width <= 0 || a.Height <= 0 {
		return false
	}
	for _, b := range images {
		if b.Width > 0 && b.Height > 0 && a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height {
			return true
		}
	}
	return false
}

func opaquePlaceholderFill(ph types.PlaceholderInfo) bool {
	if (!ph.FillSolid && !ph.FillGradient) || len(ph.FillStops) == 0 {
		return false
	}
	for _, stop := range ph.FillStops {
		if stop.Ref == "" || (stop.Mods.HasAlpha && stop.Mods.Alpha < 100000) {
			return false
		}
	}
	return true
}

func NativeImageContrastFinding(path, placeholderID string) patterns.FitFinding {
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Path: path, Code: patterns.ErrCodeTextOverImageUnverified,
			Message: fmt.Sprintf("text in placeholder %q overlaps an authored native picture frame without an opaque text fill; image coverage and colors require rendered visual review, so canvas contrast cannot verify or safely recolor this text", placeholderID),
			Fix: &patterns.FixSuggestion{Kind: "review", Params: map[string]any{
				"hint": "Use a layout with separate text and image regions for required diagrams/screenshots. For photos, inspect the actual pixels under the text. Do not dim or cover required source labels with a scrim.",
			}},
		},
		Action: "review",
	}
}

// nativeImageContrastExclusions uses the same resolved shapes and final frames
// as media insertion, including explicit image bounds. Only populated text is
// affected; image-only slides and unfilled picture placeholders are excluded.
func (ctx *singlePassContext) nativeImageContrastExclusions(slide *slideXML, spec SlideSpec, slideIndex int) map[int]bool {
	excluded := map[int]bool{}
	shapes := slide.CommonSlideData.ShapeTree.Shapes
	resolver := newPlaceholderResolver(shapes, spec.LayoutID)
	var images []types.BoundingBox
	for _, item := range spec.Content {
		image, ok := item.Value.(ImageContent)
		if item.Type != ContentImage || !ok || strings.TrimSpace(image.Path) == "" || ValidateImageFit(image.Fit) != nil {
			continue
		}
		index, _, found := resolver.ResolveWithFallback(item.PlaceholderID)
		if found {
			images = append(images, getPlaceholderBounds(&shapes[index], image.Bounds))
		}
	}
	if len(images) == 0 {
		return excluded
	}
	resolver = newPlaceholderResolver(shapes, spec.LayoutID)
	layout := ctx.profile.Layout(spec.LayoutID)
	for ci, item := range spec.Content {
		if item.Type == ContentImage || item.Type == ContentDiagram || item.Type == ContentTable || !contentItemHasText(item) {
			continue
		}
		index, _, found := resolver.ResolveWithFallback(item.PlaceholderID)
		if !found || !shapeHasText(&shapes[index]) {
			continue
		}
		ph := types.PlaceholderInfo{ID: item.PlaceholderID, Bounds: getPlaceholderBounds(&shapes[index], nil)}
		if layout != nil {
			for _, metadata := range layout.Placeholders {
				if metadata.ID == shapes[index].NonVisualProperties.ConnectionNonVisual.Name {
					ph.FillSolid, ph.FillGradient, ph.FillStops = metadata.FillSolid, metadata.FillGradient, metadata.FillStops
					break
				}
			}
		}
		if NativeImageOverlapsText(ph, images) && !excluded[index] {
			excluded[index] = true
			ctx.emitFitFinding(NativeImageContrastFinding(slidepath.ContentIndex(slideIndex, ci), item.PlaceholderID))
		}
	}
	return excluded
}
