package template

import (
	"slices"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestClassifyLayout(t *testing.T) {
	tests := []struct {
		name     string
		layout   types.LayoutMetadata
		wantTags []string
	}{
		{
			name: "title slide - single title only",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide"},
		},
		{
			name: "title slide - title and subtitle",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 500000}},
				},
			},
			wantTags: []string{"title-slide"},
		},
		{
			name: "title slide - title with other utility placeholders (date/footer/sldNum)",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderOther, Index: 10, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 0, Height: 0}}, // Date
					{Type: types.PlaceholderOther, Index: 11, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 0, Height: 0}}, // Footer
					{Type: types.PlaceholderOther, Index: 12, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 0, Height: 0}}, // Slide Number
				},
			},
			wantTags: []string{"title-slide"},
		},
		{
			name: "content - title and body",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content"},
		},
		{
			name: "two-column - side by side bodies",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "two-column", "comparison"},
		},
		{
			name: "comparison - two equal content areas",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderContent, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
					{Type: types.PlaceholderContent, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "two-column", "comparison"},
		},
		{
			name: "image-left - image on left, content on right",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "image-left"},
		},
		{
			name: "image-right - content on left, image on right",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
					{Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "image-right"},
		},
		{
			name: "full-image - large image with minimal text",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 800000}},
					{Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 0, Y: 1000000, Width: 8000000, Height: 5000000}},
				},
			},
			wantTags: []string{"full-image"},
		},
		{
			name: "chart-capable - contains chart placeholder",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderChart, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"chart-capable"},
		},
		{
			name: "chart-capable with content - chart and body",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
					{Type: types.PlaceholderChart, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "chart-capable"},
		},
		{
			name: "blank - no placeholders",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{},
			},
			wantTags: []string{"blank"},
		},
		{
			name: "title-hidden - statement layout with off-screen title",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					// Title with negative Y = off-screen (hidden)
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: -1133856, Width: 8000000, Height: 1133856}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"title-hidden"},
		},
		{
			name: "content not tagged when title is hidden",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					// Title with negative Y = off-screen (hidden)
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: -500000, Width: 8000000, Height: 500000}},
					{Type: types.PlaceholderContent, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"title-hidden"},
		},
		{
			name: "multiple classifications",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
					{Type: types.PlaceholderChart, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "chart-capable"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := tt.layout
			ClassifyLayout(&layout)

			// Check that all expected tags are present
			for _, want := range tt.wantTags {
				if !slices.Contains(layout.Tags, want) {
					t.Errorf("ClassifyLayout() tags = %v, missing expected %q", layout.Tags, want)
				}
			}

			// Check that no unexpected tags are present
			for _, got := range layout.Tags {
				if !slices.Contains(tt.wantTags, got) {
					t.Errorf("ClassifyLayout() tags = %v, unexpected tag %q", layout.Tags, got)
				}
			}
		})
	}
}

func TestClassifyLayout_NilLayout(t *testing.T) {
	// Should not panic with nil layout
	ClassifyLayout(nil)
}

func TestClassifyByName(t *testing.T) {
	tests := []struct {
		name     string
		layout   types.LayoutMetadata
		wantTags []string
	}{
		{
			name: "quote layout by name",
			layout: types.LayoutMetadata{
				Name: "Quote Slide",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"quote"},
		},
		{
			name: "statement layout by name",
			layout: types.LayoutMetadata{
				Name: "Statement",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"statement"},
		},
		{
			name: "big number layout by name",
			layout: types.LayoutMetadata{
				Name: "Big Number",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"big-number"},
		},
		{
			name: "section header by name",
			layout: types.LayoutMetadata{
				Name: "Section Header",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "section-header"},
		},
		{
			name: "divider layout by name",
			layout: types.LayoutMetadata{
				Name: "Divider",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "section-header"},
		},
		{
			name: "agenda layout by name",
			layout: types.LayoutMetadata{
				Name: "Agenda",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "agenda"},
		},
		{
			name: "timeline layout by name",
			layout: types.LayoutMetadata{
				Name: "Timeline",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "timeline-capable"},
		},
		{
			name: "process layout triggers timeline tag",
			layout: types.LayoutMetadata{
				Name: "Process Flow",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "timeline-capable"},
		},
		{
			name: "icon grid layout by name",
			layout: types.LayoutMetadata{
				Name: "Icon Grid",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "icon-grid"},
		},
		{
			name: "matrix layout triggers icon-grid tag",
			layout: types.LayoutMetadata{
				Name: "Matrix Layout",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "icon-grid"},
		},
		{
			name: "KPI layout triggers big-number tag",
			layout: types.LayoutMetadata{
				Name: "KPI Dashboard",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "big-number"},
		},
		{
			name: "outline layout triggers agenda tag",
			layout: types.LayoutMetadata{
				Name: "Outline",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content", "agenda"},
		},
		{
			name: "no semantic tag for generic name",
			layout: types.LayoutMetadata{
				Name: "Title and Content",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
				},
			},
			wantTags: []string{"content"},
		},
		// AC13: Closing/thank-you layout tag tests
		{
			name: "closing layout by name",
			layout: types.LayoutMetadata{
				Name: "Closing Slide",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "closing"},
		},
		{
			name: "thank you layout by name",
			layout: types.LayoutMetadata{
				Name: "Thank You",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "thank-you"},
		},
		{
			name: "conclusion layout triggers closing tag",
			layout: types.LayoutMetadata{
				Name: "Conclusion",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "closing"},
		},
		{
			name: "q&a layout triggers thank-you tag",
			layout: types.LayoutMetadata{
				Name: "Q&A",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "thank-you"},
		},
		{
			name: "questions layout triggers thank-you tag",
			layout: types.LayoutMetadata{
				Name: "Questions",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "thank-you"},
		},
		{
			name: "final slide layout triggers closing tag",
			layout: types.LayoutMetadata{
				Name: "Final Slide",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "closing"},
		},
		{
			name: "end slide layout triggers closing tag",
			layout: types.LayoutMetadata{
				Name: "End Slide",
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 1000000}},
				},
			},
			wantTags: []string{"title-slide", "closing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := tt.layout
			ClassifyLayout(&layout)

			// Check that all expected tags are present
			for _, want := range tt.wantTags {
				if !slices.Contains(layout.Tags, want) {
					t.Errorf("ClassifyLayout() tags = %v, missing expected %q", layout.Tags, want)
				}
			}

			// Check that no unexpected tags are present
			for _, got := range layout.Tags {
				if !slices.Contains(tt.wantTags, got) {
					t.Errorf("ClassifyLayout() tags = %v, unexpected tag %q", layout.Tags, got)
				}
			}
		})
	}
}

func TestAreSideBySide(t *testing.T) {
	tests := []struct {
		name         string
		placeholders []types.PlaceholderInfo
		type1        types.PlaceholderType
		type2        types.PlaceholderType
		want         bool
	}{
		{
			name: "side by side - similar Y, different X",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
			},
			type1: types.PlaceholderBody,
			type2: types.PlaceholderBody,
			want:  true,
		},
		{
			name: "stacked - similar X, different Y",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 2000000}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 4000000, Width: 8000000, Height: 2000000}},
			},
			type1: types.PlaceholderBody,
			type2: types.PlaceholderBody,
			want:  false,
		},
		{
			name: "too close X - not side by side",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 500000, Y: 1500000, Width: 3800000, Height: 4000000}},
			},
			type1: types.PlaceholderBody,
			type2: types.PlaceholderBody,
			want:  false,
		},
		{
			name: "different types - content and body",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
				{Type: types.PlaceholderContent, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
			},
			type1: types.PlaceholderBody,
			type2: types.PlaceholderContent,
			want:  true,
		},
		{
			name: "only one placeholder",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
			},
			type1: types.PlaceholderBody,
			type2: types.PlaceholderBody,
			want:  false,
		},
		{
			name:         "no matching placeholders",
			placeholders: []types.PlaceholderInfo{},
			type1:        types.PlaceholderBody,
			type2:        types.PlaceholderBody,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := areSideBySide(tt.placeholders, tt.type1, tt.type2)
			if got != tt.want {
				t.Errorf("areSideBySide() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasImageOnLeft(t *testing.T) {
	tests := []struct {
		name         string
		placeholders []types.PlaceholderInfo
		want         bool
	}{
		{
			name: "image on left",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
			},
			want: true,
		},
		{
			name: "image on right",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 3800000, Height: 4000000}},
				{Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 4200000, Y: 1500000, Width: 3800000, Height: 4000000}},
			},
			want: false,
		},
		{
			name: "no image placeholder",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
			},
			want: false,
		},
		{
			name: "no body placeholder",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderImage, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}},
			},
			want: false,
		},
		{
			name:         "empty placeholders",
			placeholders: []types.PlaceholderInfo{},
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasImageOnLeft(tt.placeholders)
			if got != tt.want {
				t.Errorf("hasImageOnLeft() = %v, want %v", got, tt.want)
			}
		})
	}
}
