package template

import (
	"slices"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TestClassifyLayout_CompactTitle locks the firing rule for the "compact-title"
// planning hint: a visible title placeholder that is BOTH positioned in the
// lower half of the slide (title-at-bottom geometry) AND small enough to hold
// only a short single-line title. This is the modern-yellow Section Divider
// hazard (go-slide-creator-plsd): a 48pt title slot squeezed beneath a 208pt
// decorative "Section Number" frame.
func TestClassifyLayout_CompactTitle(t *testing.T) {
	const belowHalf = emuTitleBottomThreshold + 1 // lower half of the slide
	const aboveHalf = emuTitleBottomThreshold - 1 // upper half of the slide

	titleAt := func(y int64, maxChars int) types.PlaceholderInfo {
		return types.PlaceholderInfo{
			ID:       "Title 4",
			Type:     types.PlaceholderTitle,
			Bounds:   types.BoundingBox{X: 0, Y: y, Width: 11000000, Height: 1500000},
			MaxChars: maxChars,
		}
	}
	decorativeBody := types.PlaceholderInfo{
		ID:       "Section Number",
		Type:     types.PlaceholderBody,
		Bounds:   types.BoundingBox{X: 7000000, Y: 800000, Width: 4000000, Height: 2600000},
		MaxChars: 1,
	}

	tests := []struct {
		name        string
		layout      types.LayoutMetadata
		wantCompact bool
	}{
		{
			name: "small title at bottom is compact",
			layout: types.LayoutMetadata{
				Name:         "Section Divider",
				Placeholders: []types.PlaceholderInfo{decorativeBody, titleAt(belowHalf, 30)},
			},
			wantCompact: true,
		},
		{
			name: "roomy title at bottom is not compact",
			layout: types.LayoutMetadata{
				Name:         "Section Divider",
				Placeholders: []types.PlaceholderInfo{decorativeBody, titleAt(belowHalf, 69)},
			},
			wantCompact: false,
		},
		{
			name: "small title at top is not compact",
			layout: types.LayoutMetadata{
				Name:         "One Content",
				Placeholders: []types.PlaceholderInfo{titleAt(aboveHalf, 29), decorativeBody},
			},
			wantCompact: false,
		},
		{
			name: "title at bottom with unknown font (MaxChars 0) is not compact",
			layout: types.LayoutMetadata{
				Name:         "Section Divider",
				Placeholders: []types.PlaceholderInfo{decorativeBody, titleAt(belowHalf, 0)},
			},
			wantCompact: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			layout := tt.layout
			ClassifyLayout(&layout)
			gotCompact := slices.Contains(layout.Tags, "compact-title")
			if gotCompact != tt.wantCompact {
				t.Errorf("compact-title present = %v, want %v (tags: %v)", gotCompact, tt.wantCompact, layout.Tags)
			}
			// compact-title is a refinement of title-at-bottom: it must never be
			// emitted without the geometry tag that justifies it.
			if gotCompact && !slices.Contains(layout.Tags, "title-at-bottom") {
				t.Errorf("compact-title emitted without title-at-bottom (tags: %v)", layout.Tags)
			}
		})
	}
}
