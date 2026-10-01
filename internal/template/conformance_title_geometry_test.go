package template

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCheckContentTitleGeometry(t *testing.T) {
	box := func(x, y, w, h int64) types.BoundingBox { return types.BoundingBox{X: x, Y: y, Width: w, Height: h} }
	layout := func(id string, ct types.CanonicalLayoutType, title types.BoundingBox, anchor string) types.LayoutMetadata {
		return types.LayoutMetadata{ID: id, Name: string(ct), CanonicalType: ct, Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle, Bounds: title, Anchor: anchor},
		}}
	}
	ref := box(838200, 365125, 10515600, 1325563)
	for _, tc := range []struct {
		name      string
		other     types.BoundingBox
		anchor    string
		wantWarn  bool
		wantWords []string
	}{
		{"identical", ref, "", false, nil},
		{"within tolerance", box(838200+50000, 365125-50000, 10515600, 1325563), "t", false, nil},
		// abstract's One Content title before go-slide-creator-kyk01.
		{"abstract drift", box(1322317, 268360, 8724507, 2121177), "b", true, []string{"x 1.45in vs 0.92in", "anchor b vs t"}},
		{"taller only", box(838200, 365125, 10515600, 1325563+200000), "", true, []string{"height"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkContentTitleGeometry([]types.LayoutMetadata{
				layout("slideLayout2", types.CanonicalLayoutOneContent, ref, ""),
				layout("slideLayout5", types.CanonicalLayoutTwoContent, tc.other, tc.anchor),
				layout("slideLayout7", types.CanonicalLayoutBlankTitle, ref, "t"),
			})
			warned := len(got) > 0 && got[0].Status == ConformanceStatusWarn
			if warned != tc.wantWarn {
				t.Fatalf("checks = %+v, want warn=%v", got, tc.wantWarn)
			}
			for _, w := range tc.wantWords {
				if !strings.Contains(got[0].Detail, w) {
					t.Errorf("detail %q missing %q", got[0].Detail, w)
				}
			}
		})
	}
}
