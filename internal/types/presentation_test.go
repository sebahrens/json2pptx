package types

import (
	"testing"
)

func TestSupportedSlideTypes(t *testing.T) {
	types := SupportedSlideTypes()

	// Check that we have exactly 9 types
	if len(types) != 9 {
		t.Errorf("SupportedSlideTypes() returned %d types, want 9", len(types))
	}

	// Check all expected types are present
	expectedTypes := map[SlideType]bool{
		SlideTypeTitle:      false,
		SlideTypeContent:    false,
		SlideTypeTwoColumn:  false,
		SlideTypeImage:      false,
		SlideTypeChart:      false,
		SlideTypeComparison: false,
		SlideTypeBlank:      false,
		SlideTypeSection:    false,
		SlideTypeDiagram:    false,
	}

	for _, st := range types {
		if _, ok := expectedTypes[st.Type]; !ok {
			t.Errorf("unexpected slide type: %v", st.Type)
		}
		expectedTypes[st.Type] = true
	}

	for slideType, found := range expectedTypes {
		if !found {
			t.Errorf("missing slide type: %v", slideType)
		}
	}

	// Check all types have descriptions
	for _, st := range types {
		if st.Description == "" {
			t.Errorf("slide type %v has empty description", st.Type)
		}
	}
}

func TestIsValidSlideType(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"title", true},
		{"content", true},
		{"two-column", true},
		{"image", true},
		{"chart", true},
		{"comparison", true},
		{"blank", true},
		{"section", true},
		{"diagram", true},
		{"invalid", false},
		{"unknown", false},
		{"", false},
		{"TITLE", false}, // Case sensitive
		{"Content", false},
		{"slide", false},
		{"two_column", false}, // Wrong separator
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidSlideType(tt.input)
			if got != tt.want {
				t.Errorf("IsValidSlideType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlideTypeInfo_JSON(t *testing.T) {
	// Test that SlideTypeInfo has correct JSON tags
	types := SupportedSlideTypes()
	if len(types) == 0 {
		t.Fatal("SupportedSlideTypes() returned empty slice")
	}

	// Check that first type has valid fields
	first := types[0]
	if first.Type == "" {
		t.Error("SlideTypeInfo.Type is empty")
	}
	if first.Description == "" {
		t.Error("SlideTypeInfo.Description is empty")
	}
}

func TestSlideContent_HasGraphic(t *testing.T) {
	tests := []struct {
		name    string
		content SlideContent
		want    bool
	}{
		{
			name:    "empty content has no graphic",
			content: SlideContent{},
			want:    false,
		},
		{
			name: "text-only content has no graphic",
			content: SlideContent{
				Body:    "Some text",
				Bullets: []string{"Item 1", "Item 2"},
			},
			want: false,
		},
		{
			name: "content with DiagramSpec has graphic",
			content: SlideContent{
				Body: "Chart slide",
				DiagramSpec: &DiagramSpec{
					Type: "bar_chart",
					Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}},
				},
			},
			want: true,
		},
		{
			name: "content with ImagePath has graphic",
			content: SlideContent{
				Body:      "Image slide",
				ImagePath: "/path/to/image.png",
			},
			want: true,
		},
		{
			name: "content with multiple graphics has graphic",
			content: SlideContent{
				DiagramSpec: &DiagramSpec{Type: "pie_chart", Data: map[string]any{"values": []float64{50, 50}}},
				ImagePath:   "image.png",
			},
			want: true,
		},
		{
			name: "two-column content without graphics has no graphic",
			content: SlideContent{
				Left:  []string{"Left item"},
				Right: []string{"Right item"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.content.HasGraphic()
			if got != tt.want {
				t.Errorf("SlideContent.HasGraphic() = %v, want %v", got, tt.want)
			}
		})
	}
}
