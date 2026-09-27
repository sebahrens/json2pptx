package layout

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestSectionDividerOnlyAcceptsSectionSlides(t *testing.T) {
	divider := types.LayoutMetadata{
		ID: "divider", Name: "Section Divider", Tags: []string{"section-header", "content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title", Type: types.PlaceholderTitle},
			{ID: "body", Type: types.PlaceholderBody},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 20},
	}
	cases := []struct {
		name  string
		slide types.SlideDefinition
		want  bool
	}{
		{"section", types.SlideDefinition{Type: types.SlideTypeSection, Title: "Strategy"}, true},
		{"body", types.SlideDefinition{Type: types.SlideTypeContent, Title: "Results", Content: types.SlideContent{Body: "Details"}}, false},
		{"bullets", types.SlideDefinition{Type: types.SlideTypeContent, Title: "Results", Content: types.SlideContent{Bullets: []string{"First"}}}, false},
		{"table", types.SlideDefinition{Type: types.SlideTypeContent, Title: "Results", Content: types.SlideContent{TableRaw: "| A | B |"}}, false},
		{"two-column", types.SlideDefinition{Type: types.SlideTypeTwoColumn, Title: "Results", Content: types.SlideContent{Left: []string{"A"}, Right: []string{"B"}}}, false},
		{"image", types.SlideDefinition{Type: types.SlideTypeImage, Title: "Results", Content: types.SlideContent{ImagePath: "image.png"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLayoutSuitable(divider, tc.slide); got != tc.want {
				t.Errorf("isLayoutSuitable = %v, want %v", got, tc.want)
			}
		})
	}
}
