package layout

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// statementLayout is a title-less layout whose only placeholder is a body —
// modern-yellow's "Statement" layout.
func statementLayout() types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   "layout-statement",
		Name: "Statement",
		Tags: []string{"statement"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 300, Bounds: types.BoundingBox{
				X: 457200, Y: 1600200, Width: 8229600, Height: 4525963,
			}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 3, MaxTextLines: 4},
	}
}

// TestSelectLayout_ImageSlidePrefersFullWidthContent pins the routing of a
// titled image slide on a template with no picture placeholder: the single
// full-width body of One Content wins even when variety favours the unused
// Two Content and Statement layouts. Before go-slide-creator-f0l85 every
// layout scored minimal and the hero image landed in one column of Two
// Content; go-slide-creator-2hkgy added that the title-less Statement layout
// silently dropped the headline.
func TestSelectLayout_ImageSlidePrefersFullWidthContent(t *testing.T) {
	layouts := []types.LayoutMetadata{twoColumnLayout(), statementLayout(), contentLayout(6)}
	for _, used := range []map[string]int{nil, {"layout-content": 3}} {
		req := SelectionRequest{
			Slide: types.SlideDefinition{
				Index: 3, Title: "The Rotterdam hub is already at 92% of capacity",
				Type: types.SlideTypeImage, Content: types.SlideContent{ImagePath: "photo.png"},
			},
			Layouts: layouts,
			Context: SelectionContext{Position: 3, TotalSlides: 10, UsedLayouts: used},
		}
		result, err := SelectLayout(req)
		if err != nil {
			t.Fatalf("SelectLayout: %v", err)
		}
		if result.LayoutID != "layout-content" {
			t.Errorf("used=%v: image slide picked %q, want the full-width content layout", used, result.LayoutID)
		}
	}
}

// TestImageSlideRejectsTitlelessAndColumnLayouts pins the hard filter: a
// titled image slide never lands on a title-less layout (its title would be
// replaced by the picture) or on one column of a two-column layout.
func TestImageSlideRejectsTitlelessAndColumnLayouts(t *testing.T) {
	slide := types.SlideDefinition{Title: "Site photo", Type: types.SlideTypeImage, Content: types.SlideContent{ImagePath: "p.png"}}
	if isLayoutSuitable(statementLayout(), slide) {
		t.Error("title-less statement layout accepted for a titled image slide")
	}
	if isLayoutSuitable(twoColumnLayout(), slide) {
		t.Error("two-column layout accepted for a single-image slide")
	}
	if !isLayoutSuitable(contentLayout(6), slide) {
		t.Error("full-width content layout rejected for an image slide")
	}
	if !isLayoutSuitable(imageLayout(), slide) {
		t.Error("picture layout rejected for an image slide")
	}
}
