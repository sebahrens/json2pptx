package template

import (
	"path/filepath"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestDetectSlideType_TitleSlide(t *testing.T) {
	tests := []struct {
		name       string
		layout     types.LayoutMetadata
		wantType   string
		minConfidence float64
	}{
		{
			name: "classic title slide - title only",
			layout: types.LayoutMetadata{
				Name:  "Title Slide",
				Index: 0,
				Tags:  []string{"title-slide"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeTitle,
			minConfidence: 0.6,
		},
		{
			name: "title slide with subtitle",
			layout: types.LayoutMetadata{
				Name:  "Title Slide",
				Index: 0,
				Tags:  []string{"title-slide"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderSubtitle, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 500000}},
				},
			},
			wantType:      SlideTypeTitle,
			minConfidence: 0.7,
		},
		{
			name: "layout named title without structural tag",
			layout: types.LayoutMetadata{
				Name:  "Title Layout",
				Index: 1,
				Tags:  []string{},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeTitle,
			minConfidence: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := DetectSlideType(tt.layout)
			if detection.DetectedAs != tt.wantType {
				t.Errorf("DetectSlideType() = %v, want %v", detection.DetectedAs, tt.wantType)
			}
			if detection.Confidence < tt.minConfidence {
				t.Errorf("DetectSlideType() confidence = %v, want >= %v", detection.Confidence, tt.minConfidence)
			}
		})
	}
}

func TestDetectSlideType_ContentSlide(t *testing.T) {
	tests := []struct {
		name       string
		layout     types.LayoutMetadata
		wantType   string
		minConfidence float64
	}{
		{
			name: "content slide with title and body",
			layout: types.LayoutMetadata{
				Name:  "Title and Content",
				Index: 1,
				Tags:  []string{"content"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}, MaxChars: 500},
				},
			},
			wantType:      SlideTypeContent,
			minConfidence: 0.7,
		},
		{
			name: "content slide with content placeholder",
			layout: types.LayoutMetadata{
				Name:  "Content Only",
				Index: 2,
				Tags:  []string{"content"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
					{Type: types.PlaceholderContent, Bounds: types.BoundingBox{X: 0, Y: 1500000, Width: 8000000, Height: 4000000}, MaxChars: 300},
				},
			},
			wantType:      SlideTypeContent,
			minConfidence: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := DetectSlideType(tt.layout)
			if detection.DetectedAs != tt.wantType {
				t.Errorf("DetectSlideType() = %v, want %v", detection.DetectedAs, tt.wantType)
			}
			if detection.Confidence < tt.minConfidence {
				t.Errorf("DetectSlideType() confidence = %v, want >= %v", detection.Confidence, tt.minConfidence)
			}
		})
	}
}

func TestDetectSlideType_SectionSlide(t *testing.T) {
	tests := []struct {
		name       string
		layout     types.LayoutMetadata
		wantType   string
		minConfidence float64
	}{
		{
			name: "section header with name",
			layout: types.LayoutMetadata{
				Name:  "Section Header",
				Index: 2,
				Tags:  []string{"section-header"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeSection,
			minConfidence: 0.7,
		},
		{
			name: "section divider",
			layout: types.LayoutMetadata{
				Name:  "Section Divider",
				Index: 3,
				Tags:  []string{},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeSection,
			minConfidence: 0.5,
		},
		{
			name: "section transition layout",
			layout: types.LayoutMetadata{
				Name:  "Transition",
				Index: 4,
				Tags:  []string{"section-header"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeSection,
			minConfidence: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := DetectSlideType(tt.layout)
			if detection.DetectedAs != tt.wantType {
				t.Errorf("DetectSlideType() = %v, want %v", detection.DetectedAs, tt.wantType)
			}
			if detection.Confidence < tt.minConfidence {
				t.Errorf("DetectSlideType() confidence = %v, want >= %v", detection.Confidence, tt.minConfidence)
			}
		})
	}
}

func TestDetectSlideType_ClosingSlide(t *testing.T) {
	tests := []struct {
		name       string
		layout     types.LayoutMetadata
		wantType   string
		minConfidence float64
	}{
		{
			name: "closing slide with tag",
			layout: types.LayoutMetadata{
				Name:  "Closing Slide",
				Index: 10,
				Tags:  []string{"closing"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeClosing,
			minConfidence: 0.7,
		},
		{
			name: "thank you slide",
			layout: types.LayoutMetadata{
				Name:  "Thank You",
				Index: 11,
				Tags:  []string{"thank-you"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeClosing,
			minConfidence: 0.6,
		},
		{
			name: "questions slide",
			layout: types.LayoutMetadata{
				Name:  "Questions?",
				Index: 12,
				Tags:  []string{},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeClosing,
			minConfidence: 0.3,
		},
		{
			name: "end slide",
			layout: types.LayoutMetadata{
				Name:  "End Slide",
				Index: 13,
				Tags:  []string{"closing"},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
				},
			},
			wantType:      SlideTypeClosing,
			minConfidence: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := DetectSlideType(tt.layout)
			if detection.DetectedAs != tt.wantType {
				t.Errorf("DetectSlideType() = %v, want %v", detection.DetectedAs, tt.wantType)
			}
			if detection.Confidence < tt.minConfidence {
				t.Errorf("DetectSlideType() confidence = %v, want >= %v", detection.Confidence, tt.minConfidence)
			}
		})
	}
}

func TestDetectSlideType_Unknown(t *testing.T) {
	tests := []struct {
		name   string
		layout types.LayoutMetadata
	}{
		{
			name: "blank slide with no placeholders",
			layout: types.LayoutMetadata{
				Name:  "Blank",
				Index: 5,
				Tags:  []string{"blank"},
				Placeholders: []types.PlaceholderInfo{},
			},
		},
		{
			name: "ambiguous slide",
			layout: types.LayoutMetadata{
				Name:  "Custom Layout",
				Index: 6,
				Tags:  []string{},
				Placeholders: []types.PlaceholderInfo{
					{Type: types.PlaceholderOther, Bounds: types.BoundingBox{X: 0, Y: 0, Width: 0, Height: 0}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := DetectSlideType(tt.layout)
			if detection.DetectedAs != SlideTypeUnknown {
				t.Errorf("DetectSlideType() = %v, want %v", detection.DetectedAs, SlideTypeUnknown)
			}
		})
	}
}

func TestValidateMinimalTemplate_WithRealTemplate(t *testing.T) {
	// Test with real template from testdata
	templatePath := filepath.Join("testdata", "standard.pptx")

	result, err := ValidateMinimalTemplate(templatePath)
	if err != nil {
		t.Fatalf("ValidateMinimalTemplate() error = %v", err)
	}

	// Standard template should have at least some detected layouts
	// We don't require all 4 for the testdata template since it may not be a "minimal" template
	t.Logf("Validation result: Valid=%v, Title=%d, Content=%d, Section=%d, Closing=%d",
		result.Valid, result.TitleSlideIdx, result.ContentSlideIdx,
		result.SectionSlideIdx, result.ClosingSlideIdx)
	t.Logf("Errors: %v", result.Errors)
	t.Logf("Warnings: %v", result.Warnings)

	// At minimum, it should detect a title and content slide
	if result.TitleSlideIdx == -1 && result.ContentSlideIdx == -1 {
		t.Error("Expected at least a title or content slide to be detected")
	}
}

func TestValidateMinimalTemplate_InvalidPath(t *testing.T) {
	result, err := ValidateMinimalTemplate("nonexistent.pptx")
	if err == nil {
		t.Error("ValidateMinimalTemplate() expected error for nonexistent file")
	}
	if result != nil {
		t.Error("ValidateMinimalTemplate() expected nil result for error case")
	}
}

func TestMinimalTemplateValidation_DefaultValues(t *testing.T) {
	result := &MinimalTemplateValidation{
		Valid:           false,
		TitleSlideIdx:   -1,
		ContentSlideIdx: -1,
		SectionSlideIdx: -1,
		ClosingSlideIdx: -1,
	}

	if result.Valid {
		t.Error("Default Valid should be false")
	}
	if result.TitleSlideIdx != -1 {
		t.Error("Default TitleSlideIdx should be -1")
	}
	if result.ContentSlideIdx != -1 {
		t.Error("Default ContentSlideIdx should be -1")
	}
	if result.SectionSlideIdx != -1 {
		t.Error("Default SectionSlideIdx should be -1")
	}
	if result.ClosingSlideIdx != -1 {
		t.Error("Default ClosingSlideIdx should be -1")
	}
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}

	for _, tt := range tests {
		result := clampScore(tt.input)
		if result != tt.expected {
			t.Errorf("clampScore(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestHasTag(t *testing.T) {
	tags := []string{"title-slide", "content", "closing"}

	if !hasTag(tags, "title-slide") {
		t.Error("hasTag should find 'title-slide'")
	}
	if !hasTag(tags, "content") {
		t.Error("hasTag should find 'content'")
	}
	if hasTag(tags, "nonexistent") {
		t.Error("hasTag should not find 'nonexistent'")
	}
	if hasTag(nil, "any") {
		t.Error("hasTag should return false for nil slice")
	}
}

func TestHasSignificantBodyCapacity(t *testing.T) {
	tests := []struct {
		name         string
		placeholders []types.PlaceholderInfo
		want         bool
	}{
		{
			name: "body with significant capacity",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, MaxChars: 500},
			},
			want: true,
		},
		{
			name: "body with low capacity",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderBody, MaxChars: 50},
			},
			want: false,
		},
		{
			name: "content placeholder with significant capacity",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderContent, MaxChars: 200},
			},
			want: true,
		},
		{
			name: "no body placeholders",
			placeholders: []types.PlaceholderInfo{
				{Type: types.PlaceholderTitle, MaxChars: 500},
			},
			want: false,
		},
		{
			name: "empty placeholders",
			placeholders: []types.PlaceholderInfo{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSignificantBodyCapacity(tt.placeholders)
			if got != tt.want {
				t.Errorf("hasSignificantBodyCapacity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountLayoutPlaceholders(t *testing.T) {
	placeholders := []types.PlaceholderInfo{
		{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{Y: 100000}},
		{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{Y: -100000}}, // hidden
		{Type: types.PlaceholderSubtitle, Bounds: types.BoundingBox{Y: 200000}},
		{Type: types.PlaceholderBody, Bounds: types.BoundingBox{Y: 300000}},
		{Type: types.PlaceholderContent, Bounds: types.BoundingBox{Y: 400000}},
		{Type: types.PlaceholderImage, Bounds: types.BoundingBox{Y: 500000}},
		{Type: types.PlaceholderChart, Bounds: types.BoundingBox{Y: 600000}},
		{Type: types.PlaceholderOther, Bounds: types.BoundingBox{Y: 0}},
	}

	counts := countLayoutPlaceholders(placeholders)

	if counts.title != 2 {
		t.Errorf("title count = %d, want 2", counts.title)
	}
	if counts.visibleTitle != 1 {
		t.Errorf("visibleTitle count = %d, want 1", counts.visibleTitle)
	}
	if counts.subtitle != 1 {
		t.Errorf("subtitle count = %d, want 1", counts.subtitle)
	}
	if counts.body != 2 { // body + content
		t.Errorf("body count = %d, want 2", counts.body)
	}
	if counts.image != 1 {
		t.Errorf("image count = %d, want 1", counts.image)
	}
	if counts.chart != 1 {
		t.Errorf("chart count = %d, want 1", counts.chart)
	}
}

// TestSlideTypeDetection_Reasons verifies that detection reasons are populated.
func TestSlideTypeDetection_Reasons(t *testing.T) {
	layout := types.LayoutMetadata{
		Name:  "Title Slide",
		Index: 0,
		Tags:  []string{"title-slide"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 0, Y: 100000, Width: 8000000, Height: 1000000}},
		},
	}

	detection := DetectSlideType(layout)

	if len(detection.Reasons) == 0 {
		t.Error("Expected detection reasons to be populated")
	}
	if detection.LayoutName != "Title Slide" {
		t.Errorf("LayoutName = %v, want Title Slide", detection.LayoutName)
	}
	if detection.LayoutIndex != 0 {
		t.Errorf("LayoutIndex = %v, want 0", detection.LayoutIndex)
	}
}
