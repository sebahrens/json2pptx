package template

import (
	"fmt"
	"slices"

	"github.com/sebahrens/json2pptx/internal/types"
)

// checkConflictingLayoutTags reports combinations that make the same layout
// eligible for incompatible slide roles. Other tag pairs (for example
// two-column/comparison) intentionally describe the same structure.
func checkConflictingLayoutTags(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck
	for _, layout := range layouts {
		if slices.Contains(layout.Tags, "content") && slices.Contains(layout.Tags, "section-header") {
			checks = append(checks, ConformanceCheck{
				Category: "layout", Check: "No conflicting structural tags", Status: ConformanceStatusWarn,
				Detail: fmt.Sprintf("Layout %q (%s) has both content and section-header tags; reserve section-number layouts for section slides", layout.Name, layout.ID),
			})
		}
	}
	if len(checks) == 0 {
		return []ConformanceCheck{{Category: "layout", Check: "No conflicting structural tags", Status: ConformanceStatusPass}}
	}
	return checks
}

// checkTextPlaceholderOverlap catches intersecting author-addressable text
// slots. Picture/text overlays are checked separately because they are often
// intentional; section-number frames are decorative and may overlap a title.
func checkTextPlaceholderOverlap(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck
	for _, layout := range layouts {
		for i, left := range layout.Placeholders {
			if !conformanceTextSlot(left) {
				continue
			}
			for _, right := range layout.Placeholders[i+1:] {
				if !conformanceTextSlot(right) || !materialPlaceholderOverlap(left.Bounds, right.Bounds) {
					continue
				}
				checks = append(checks, ConformanceCheck{
					Category: "geometry", Check: "Text placeholders do not overlap", Status: ConformanceStatusWarn,
					Detail: fmt.Sprintf("Layout %q (%s): %q overlaps %q", layout.Name, layout.ID, left.ID, right.ID),
				})
			}
		}
	}
	if len(checks) == 0 {
		return []ConformanceCheck{{Category: "geometry", Check: "Text placeholders do not overlap", Status: ConformanceStatusPass}}
	}
	return checks
}

// checkImageTitleLayerOrder checks the source layout's placeholder order.
// A later, overlapping image paints over title text in an ordinary OOXML
// shape tree. The generator also safeguards emitted slide order, but this
// catches a template that is unsafe when used outside that path.
func checkImageTitleLayerOrder(layouts []types.LayoutMetadata) []ConformanceCheck {
	var checks []ConformanceCheck
	for _, layout := range layouts {
		for imageIndex, image := range layout.Placeholders {
			if image.Type != types.PlaceholderImage {
				continue
			}
			for titleIndex, title := range layout.Placeholders {
				if title.Type != types.PlaceholderTitle || titleIndex >= imageIndex ||
					!materialPlaceholderOverlap(image.Bounds, title.Bounds) {
					continue
				}
				checks = append(checks, ConformanceCheck{
					Category: "layering", Check: "Image does not cover title", Status: ConformanceStatusWarn,
					Detail: fmt.Sprintf("Layout %q (%s): image %q follows and overlaps title %q in the layout shape tree", layout.Name, layout.ID, image.ID, title.ID),
				})
			}
		}
	}
	if len(checks) == 0 {
		return []ConformanceCheck{{Category: "layering", Check: "Image does not cover title", Status: ConformanceStatusPass}}
	}
	return checks
}

func conformanceTextSlot(ph types.PlaceholderInfo) bool {
	if ph.Role == types.PlaceholderRoleSectionNumber || ph.Bounds.Width <= 0 || ph.Bounds.Height <= 0 {
		return false
	}
	switch ph.Type {
	case types.PlaceholderTitle, types.PlaceholderSubtitle, types.PlaceholderBody, types.PlaceholderContent:
		return true
	default:
		return false
	}
}

func materialPlaceholderOverlap(a, b types.BoundingBox) bool {
	if a.Width <= 0 || a.Height <= 0 || b.Width <= 0 || b.Height <= 0 {
		return false
	}
	// Use float64 throughout: malformed external templates can carry EMU
	// coordinates whose sum or area would overflow int64.
	w := min(float64(a.X)+float64(a.Width), float64(b.X)+float64(b.Width)) - max(float64(a.X), float64(b.X))
	h := min(float64(a.Y)+float64(a.Height), float64(b.Y)+float64(b.Height)) - max(float64(a.Y), float64(b.Y))
	if w <= 0 || h <= 0 {
		return false
	}
	// Ignore sub-millimetre edge noise and overlaps below 2% of the smaller
	// slot.
	const oneMM = 36000.0
	area := w * h
	smaller := min(float64(a.Width)*float64(a.Height), float64(b.Width)*float64(b.Height))
	return w >= oneMM && h >= oneMM && area >= smaller*0.02
}
