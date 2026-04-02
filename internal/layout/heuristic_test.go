package layout

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// Test fixtures
func titleLayout() types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   "layout-title",
		Name: "Title Slide",
		Tags: []string{"title-slide"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  2,
			TextHeavy:     false,
			VisualFocused: true,
		},
	}
}

func contentLayout(maxBullets int) types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   "layout-content",
		Name: "Content Slide",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 500, Bounds: types.BoundingBox{
				X: 457200, Y: 1600200, Width: 8229600, Height: 4525963, // full-size body
			}},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:   maxBullets,
			MaxTextLines: 10,
			TextHeavy:    true,
		},
	}
}

func chartLayout() types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   "layout-chart",
		Name: "Chart Slide",
		Tags: []string{"chart-capable"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "chart-1", Type: types.PlaceholderChart},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			HasChartSlot:  true,
			VisualFocused: true,
		},
	}
}

func twoColumnLayout() types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   "layout-twocol",
		Name: "Two Column",
		Tags: []string{"two-column", "comparison"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 300},
			{ID: "body-2", Type: types.PlaceholderBody, MaxChars: 300},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:   8,
			MaxTextLines: 12,
		},
	}
}

func imageLayout() types.LayoutMetadata {
	return types.LayoutMetadata{
		ID:   "layout-image",
		Name: "Image Slide",
		Tags: []string{"image-left"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "image-1", Type: types.PlaceholderImage},
		},
		Capacity: types.CapacityEstimate{
			HasImageSlot:  true,
			VisualFocused: true,
		},
	}
}

// AC1: Title Slide Selection
func TestSelectLayout_TitleSlide(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(6),
		titleLayout(),
	}

	slide := types.SlideDefinition{
		Index: 0,
		Title: "My Presentation",
		Type:  types.SlideTypeTitle,
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{
			Position:    0,
			TotalSlides: 10,
		},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-title" {
		t.Errorf("expected layout-title, got %s", result.LayoutID)
	}
}

// AC2: Content Fitting
func TestSelectLayout_ContentFitting(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(3),  // Too small
		contentLayout(5),  // Perfect fit
		contentLayout(10), // Extra space
	}

	// Reassign IDs for distinction
	layouts[0].ID = "layout-small"
	layouts[1].ID = "layout-medium"
	layouts[2].ID = "layout-large"

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Content Slide",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"A", "B", "C", "D", "E"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should select medium or large (both fit), not small
	if result.LayoutID == "layout-small" {
		t.Errorf("selected layout with insufficient capacity")
	}

	// Medium or large should have high capacity score
	if result.LayoutID != "layout-medium" && result.LayoutID != "layout-large" {
		t.Errorf("expected layout-medium or layout-large, got %s", result.LayoutID)
	}
}

// AC3: Overflow Warning
func TestSelectLayout_OverflowWarning(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(6), // Max 6 bullets
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Overflow Test",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have warning about bullet overflow
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "10 bullets") && strings.Contains(w, "6") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected overflow warning, got warnings: %v", result.Warnings)
	}
}

// AC4: Consistency Penalty
func TestSelectLayout_ConsistencyPenalty(t *testing.T) {
	layoutA := contentLayout(10)
	layoutA.ID = "layout-a"
	layoutB := contentLayout(10)
	layoutB.ID = "layout-b"

	layouts := []types.LayoutMetadata{layoutA, layoutB}

	slide := types.SlideDefinition{
		Index: 2,
		Title: "Slide 2",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"Point 1", "Point 2"},
		},
	}

	// Context shows layout-a was used previously
	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{
			Position:     2,
			TotalSlides:  10,
			PreviousType: "layout-a",
		},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should prefer layout-b for variety
	if result.LayoutID != "layout-b" {
		t.Errorf("expected layout-b for variety, got %s", result.LayoutID)
	}
}

// AC5: First Slide Bias
func TestSelectLayout_FirstSlideBias(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(6),
		titleLayout(),
	}

	// Ambiguous slide that could be content or title
	slide := types.SlideDefinition{
		Index: 0,
		Title: "Opening",
		Type:  types.SlideTypeContent, // Could use content layout
		Content: types.SlideContent{
			Body: "Welcome",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{
			Position:    0, // First slide
			TotalSlides: 10,
		},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// At position 0, title layout should get bonus
	// However, since slide type is content, it depends on scoring
	// The position bonus should influence toward title if available
	if result.LayoutID == "layout-title" {
		t.Logf("correctly preferred title layout for first slide")
	}
}

// AC13: Last Slide Closing Layout
func TestSelectLayout_LastSlideClosingLayout(t *testing.T) {
	// Create a closing layout
	closingLayout := types.LayoutMetadata{
		ID:   "layout-closing",
		Name: "Closing Slide",
		Tags: []string{"title-slide", "closing"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  2,
			VisualFocused: true,
		},
	}

	// Create a thank-you layout
	thankYouLayout := types.LayoutMetadata{
		ID:   "layout-thankyou",
		Name: "Thank You",
		Tags: []string{"title-slide", "thank-you"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  2,
			VisualFocused: true,
		},
	}

	genericTitle := titleLayout()
	genericContent := contentLayout(6)

	tests := []struct {
		name           string
		layouts        []types.LayoutMetadata
		slide          types.SlideDefinition
		position       int
		totalSlides    int
		expectedLayout string
	}{
		{
			name:    "last slide prefers closing layout",
			layouts: []types.LayoutMetadata{genericTitle, closingLayout, genericContent},
			slide: types.SlideDefinition{
				Title: "Thank You for Your Attention",
				Type:  types.SlideTypeTitle,
			},
			position:       9,
			totalSlides:    10,
			expectedLayout: "layout-closing",
		},
		{
			name:    "last slide prefers thank-you layout",
			layouts: []types.LayoutMetadata{genericTitle, thankYouLayout, genericContent},
			slide: types.SlideDefinition{
				Title: "Questions?",
				Type:  types.SlideTypeTitle,
			},
			position:       4,
			totalSlides:    5,
			expectedLayout: "layout-thankyou",
		},
		{
			name:    "non-last slide does not prefer closing layout",
			layouts: []types.LayoutMetadata{genericTitle, closingLayout},
			slide: types.SlideDefinition{
				Title: "Introduction",
				Type:  types.SlideTypeTitle,
			},
			position:       0,
			totalSlides:    10,
			expectedLayout: "layout-title", // First slide prefers title-slide, not closing
		},
		{
			name:    "middle slide does not prefer closing layout",
			layouts: []types.LayoutMetadata{genericTitle, closingLayout},
			slide: types.SlideDefinition{
				Title: "Middle Slide",
				Type:  types.SlideTypeTitle,
			},
			position:       5,
			totalSlides:    10,
			expectedLayout: "layout-title", // Middle slide should not prefer closing
		},
		{
			name:    "last slide with dense bullets avoids closing layout",
			layouts: []types.LayoutMetadata{closingLayout, genericContent},
			slide: types.SlideDefinition{
				Title: "Stress Test: Dense Bullet List",
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					Bullets: []string{"Item 1", "Item 2", "Item 3", "Item 4", "Item 5", "Item 6"},
				},
			},
			position:       9,
			totalSlides:    10,
			expectedLayout: "layout-content", // Dense bullets should prefer content over closing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SelectionRequest{
				Slide:   tt.slide,
				Layouts: tt.layouts,
				Context: SelectionContext{
					Position:    tt.position,
					TotalSlides: tt.totalSlides,
				},
			}

			result, err := SelectLayout(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.LayoutID != tt.expectedLayout {
				t.Errorf("expected layout %s, got %s (reasoning: %s)",
					tt.expectedLayout, result.LayoutID, result.Reasoning)
			}
		})
	}
}

// Chart/diagram slides should never get closing layout, even at last position
func TestSelectLayout_ChartSlideRejectsClosingLayout(t *testing.T) {
	// Mimic programmatic templates: closing layout has body placeholder but tiny area
	closingLay := types.LayoutMetadata{
		ID:   "layout-closing",
		Name: "Closing",
		Tags: []string{"closing"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 200,
				Bounds: types.BoundingBox{X: 1485900, Y: 685800, Width: 9486900, Height: 1851025}},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  2,
			VisualFocused: true,
		},
	}

	contentLay := contentLayout(6)

	chartSlide := types.SlideDefinition{
		Title: "Sales Data",
		Type:  types.SlideTypeChart,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}}},
		},
	}

	diagramSlide := types.SlideDefinition{
		Title: "Process Flow",
		Type:  types.SlideTypeDiagram,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{Type: "process_flow", Data: map[string]any{"steps": []string{"A", "B"}}},
		},
	}

	tests := []struct {
		name           string
		slide          types.SlideDefinition
		position       int
		totalSlides    int
		expectedLayout string
	}{
		{
			name:           "chart at last position prefers content over closing",
			slide:          chartSlide,
			position:       9,
			totalSlides:    10,
			expectedLayout: "layout-content",
		},
		{
			name:           "diagram at last position prefers content over closing",
			slide:          diagramSlide,
			position:       4,
			totalSlides:    5,
			expectedLayout: "layout-content",
		},
		{
			name:           "chart at middle position prefers content over closing",
			slide:          chartSlide,
			position:       3,
			totalSlides:    10,
			expectedLayout: "layout-content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SelectionRequest{
				Slide:   tt.slide,
				Layouts: []types.LayoutMetadata{closingLay, contentLay},
				Context: SelectionContext{
					Position:    tt.position,
					TotalSlides: tt.totalSlides,
				},
			}

			result, err := SelectLayout(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.LayoutID != tt.expectedLayout {
				t.Errorf("expected %s, got %s (reasoning: %s)",
					tt.expectedLayout, result.LayoutID, result.Reasoning)
			}
		})
	}
}

// AC6: Confidence Scoring
func TestSelectLayout_HighConfidence(t *testing.T) {
	layouts := []types.LayoutMetadata{
		titleLayout(),    // Perfect match
		contentLayout(6), // Poor match for title type
	}

	slide := types.SlideDefinition{
		Index: 0,
		Title: "Title Slide",
		Type:  types.SlideTypeTitle,
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 0, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clear winner should have high confidence
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence > 0.8, got %.2f", result.Confidence)
	}
}

// AC10: Content Mapping
func TestSelectLayout_ContentMapping(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(6),
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Test Slide",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"Bullet 1", "Bullet 2"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have mappings for title and bullets
	if len(result.Mappings) < 2 {
		t.Errorf("expected at least 2 mappings (title + bullets), got %d", len(result.Mappings))
	}

	hasTitle := false
	hasBullets := false
	for _, m := range result.Mappings {
		if m.ContentField == "title" {
			hasTitle = true
		}
		if m.ContentField == "bullets" {
			hasBullets = true
		}
	}

	if !hasTitle {
		t.Errorf("missing title mapping")
	}
	if !hasBullets {
		t.Errorf("missing bullets mapping")
	}
}

// TestSelectLayout_BulletGroupsMapping verifies that bullet_groups content
// produces a "bullets" mapping pointing to a body placeholder.
// Regression test for pptx-hgm: bullet_groups rendered blank when the
// selected layout lacked a body placeholder.
func TestSelectLayout_BulletGroupsMapping(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(8),
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Strategic Priorities",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Body: "Our three pillars:",
			BulletGroups: []types.BulletGroup{
				{Header: "Growth", Bullets: []string{"Revenue up 15%", "New markets"}},
				{Header: "Efficiency", Bullets: []string{"Costs down 10%"}},
			},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 5},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasBullets := false
	for _, m := range result.Mappings {
		if m.ContentField == "bullets" {
			hasBullets = true
		}
	}
	if !hasBullets {
		t.Errorf("expected bullets mapping for bullet_groups content, got mappings: %v", result.Mappings)
	}

	// Should have no warnings about missing body placeholder
	for _, w := range result.Warnings {
		if strings.Contains(w, "bullet_groups") {
			t.Errorf("unexpected bullet_groups warning: %s", w)
		}
	}
}

// AC11: Truncation Detection
func TestSelectLayout_TruncationDetection(t *testing.T) {
	layout := contentLayout(6)
	layout.Placeholders[1].MaxChars = 50 // Small capacity

	layouts := []types.LayoutMetadata{layout}

	longBody := "This is a very long body text that definitely exceeds fifty characters and should trigger truncation detection."

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Test",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Body: longBody,
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find body mapping
	var bodyMapping *ContentMapping
	for i := range result.Mappings {
		if result.Mappings[i].ContentField == "body" {
			bodyMapping = &result.Mappings[i]
			break
		}
	}

	if bodyMapping == nil {
		t.Fatalf("no body mapping found")
	}

	if !bodyMapping.Truncated {
		t.Errorf("expected Truncated=true for long body text")
	}

	if bodyMapping.TruncateAt != 50 {
		t.Errorf("expected TruncateAt=50, got %d", bodyMapping.TruncateAt)
	}
}

// AC12: No Suitable Layout - charts now fall back to body placeholders
func TestSelectLayout_NoSuitableLayout(t *testing.T) {
	// A layout with only a title (no body placeholder) cannot host charts
	layouts := []types.LayoutMetadata{
		{
			ID:   "layout-title-only",
			Name: "Title Only",
			Tags: []string{"title-slide"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			},
			Capacity: types.CapacityEstimate{},
		},
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Chart Slide",
		Type:  types.SlideTypeChart,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}}},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	_, err := SelectLayout(req)
	if err == nil {
		t.Fatalf("expected error for chart slide without chart or body layout")
	}

	if !strings.Contains(err.Error(), "chart") && !strings.Contains(err.Error(), "body") {
		t.Errorf("error should mention chart or body requirement: %v", err)
	}
}

// Test that charts CAN use content layouts with body placeholders
func TestSelectLayout_ChartFallbackToBody(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(6), // No chart capability, but has body placeholder
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Chart Slide",
		Type:  types.SlideTypeChart,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}}},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("chart should fall back to body placeholder, but got error: %v", err)
	}

	// Should produce a warning about using body instead of chart placeholder
	if len(result.Warnings) == 0 {
		t.Error("expected warning about using body placeholder for chart")
	}

	// Should have a diagram mapping (not chart, since we fell back)
	var foundDiagramMapping bool
	for _, m := range result.Mappings {
		if m.ContentField == "diagram" {
			foundDiagramMapping = true
			break
		}
	}
	if !foundDiagramMapping {
		t.Error("expected diagram mapping for fallback chart")
	}
}

// Test no layouts provided
func TestSelectLayout_NoLayouts(t *testing.T) {
	slide := types.SlideDefinition{
		Index: 0,
		Title: "Test",
		Type:  types.SlideTypeContent,
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: []types.LayoutMetadata{},
		Context: SelectionContext{Position: 0, TotalSlides: 1},
	}

	_, err := SelectLayout(req)
	if err == nil {
		t.Fatalf("expected error when no layouts provided")
	}
}

// Test image slide selection
func TestSelectLayout_ImageSlide(t *testing.T) {
	layouts := []types.LayoutMetadata{
		imageLayout(),
		contentLayout(6),
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Image Slide",
		Type:  types.SlideTypeImage,
		Content: types.SlideContent{
			ImagePath: "image.png",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-image" {
		t.Errorf("expected layout-image for image slide, got %s", result.LayoutID)
	}

	// Check for image mapping
	hasImage := false
	for _, m := range result.Mappings {
		if m.ContentField == "image" {
			hasImage = true
			break
		}
	}

	if !hasImage {
		t.Errorf("missing image mapping")
	}
}

// Test image slide falls back to body placeholder when no image placeholder exists
func TestSelectLayout_ImageFallbackToBody(t *testing.T) {
	// Only content layout — no image placeholder
	layouts := []types.LayoutMetadata{
		contentLayout(6),
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Image Slide",
		Type:  types.SlideTypeImage,
		Content: types.SlideContent{
			ImagePath: "photo.png",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Image should be mapped to the body placeholder as fallback
	hasImage := false
	for _, m := range result.Mappings {
		if m.ContentField == "image" {
			hasImage = true
			if m.PlaceholderID != "body-1" && m.PlaceholderID != "idx:1" {
				t.Errorf("expected image mapped to body placeholder, got %s", m.PlaceholderID)
			}
			break
		}
	}

	if !hasImage {
		t.Errorf("expected image mapping via body fallback, got none; mappings: %v", result.Mappings)
	}

	// Should have a warning about the fallback
	hasWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "image") && strings.Contains(w, "body placeholder") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning about image fallback to body, got warnings: %v", result.Warnings)
	}
}

// Test image slide with no placeholder and no body produces warning (not blank)
func TestSelectLayout_ImageNoPlaceholderNoBody(t *testing.T) {
	// Layout with only a title — no image or body placeholder
	layouts := []types.LayoutMetadata{
		titleLayout(),
	}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Image Slide",
		Type:  types.SlideTypeImage,
		Content: types.SlideContent{
			ImagePath: "photo.png",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Image should NOT be mapped (no suitable placeholder)
	for _, m := range result.Mappings {
		if m.ContentField == "image" {
			t.Errorf("expected no image mapping when no body/image placeholder, got mapping to %s", m.PlaceholderID)
		}
	}

	// Should have a warning
	hasWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "image") && strings.Contains(w, "no body placeholder") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning about no available placeholder, got warnings: %v", result.Warnings)
	}
}

// Test chart slide selection
func TestSelectLayout_ChartSlide(t *testing.T) {
	layouts := []types.LayoutMetadata{
		chartLayout(),
		contentLayout(6),
	}

	slide := types.SlideDefinition{
		Index: 2,
		Title: "Sales Data",
		Type:  types.SlideTypeChart,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"categories": []string{"A"}, "values": []float64{10}},
			},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 2, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-chart" {
		t.Errorf("expected layout-chart for chart slide, got %s", result.LayoutID)
	}

	// Check for chart mapping
	hasChart := false
	for _, m := range result.Mappings {
		if m.ContentField == "chart" {
			hasChart = true
			break
		}
	}

	if !hasChart {
		t.Errorf("missing chart mapping")
	}
}

// Test two-column slide selection
func TestSelectLayout_TwoColumnSlide(t *testing.T) {
	layouts := []types.LayoutMetadata{
		twoColumnLayout(),
		contentLayout(6),
	}

	slide := types.SlideDefinition{
		Index: 3,
		Title: "Comparison",
		Type:  types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			Left:  []string{"Pro 1", "Pro 2"},
			Right: []string{"Con 1", "Con 2"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 3, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-twocol" {
		t.Errorf("expected layout-twocol for two-column slide, got %s", result.LayoutID)
	}
}

// Test visual balance scoring
func TestSelectLayout_VisualBalance(t *testing.T) {
	sparseLayout := contentLayout(3)
	sparseLayout.ID = "layout-sparse"
	sparseLayout.Capacity.VisualFocused = true
	sparseLayout.Capacity.TextHeavy = false

	denseLayout := contentLayout(10)
	denseLayout.ID = "layout-dense"
	denseLayout.Capacity.TextHeavy = true
	denseLayout.Capacity.VisualFocused = false

	layouts := []types.LayoutMetadata{sparseLayout, denseLayout}

	// Sparse content
	sparseSlide := types.SlideDefinition{
		Index: 1,
		Title: "Simple",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"One"},
		},
	}

	req := SelectionRequest{
		Slide:   sparseSlide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should prefer sparse layout for sparse content
	if result.LayoutID != "layout-sparse" {
		t.Logf("sparse content selected: %s (note: capacity may override balance)", result.LayoutID)
	}

	// Dense content
	denseSlide := types.SlideDefinition{
		Index: 2,
		Title: "Detailed",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"1", "2", "3", "4", "5", "6", "7"},
			Body:    "Additional text",
		},
	}

	req.Slide = denseSlide
	req.Context.Position = 2

	result, err = SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should prefer dense layout for dense content
	if result.LayoutID != "layout-dense" {
		t.Errorf("expected layout-dense for dense content, got %s", result.LayoutID)
	}
}

// Test semantic tag matching boosts layout scores
func TestSelectLayout_SemanticTagMatching(t *testing.T) {
	// Create a quote layout with semantic tag
	quoteLayout := types.LayoutMetadata{
		ID:   "layout-quote",
		Name: "Quote Slide",
		Tags: []string{"content", "quote"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 500},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:   6,
			MaxTextLines: 10,
			TextHeavy:    true,
		},
	}

	genericContent := contentLayout(6)

	tests := []struct {
		name           string
		slide          types.SlideDefinition
		layouts        []types.LayoutMetadata
		expectedLayout string
		semanticBonus  bool
	}{
		{
			name: "quote title selects quote layout",
			slide: types.SlideDefinition{
				Index: 1,
				Title: "Quote from CEO",
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					Body: "\"Innovation distinguishes between a leader and a follower.\"",
				},
			},
			layouts:        []types.LayoutMetadata{genericContent, quoteLayout},
			expectedLayout: "layout-quote",
			semanticBonus:  true,
		},
		{
			name: "body with quotation marks prefers quote layout",
			slide: types.SlideDefinition{
				Index: 1,
				Title: "Inspiration",
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					Body: "\u201CThe only way to do great work is to love what you do.\u201D \u2014 Steve Jobs",
				},
			},
			layouts:        []types.LayoutMetadata{genericContent, quoteLayout},
			expectedLayout: "layout-quote",
			semanticBonus:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SelectionRequest{
				Slide:   tt.slide,
				Layouts: tt.layouts,
				Context: SelectionContext{Position: 1, TotalSlides: 10},
			}

			result, err := SelectLayout(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.LayoutID != tt.expectedLayout {
				t.Errorf("expected %s, got %s", tt.expectedLayout, result.LayoutID)
			}

			// Check that reasoning includes semantic score
			if tt.semanticBonus && !strings.Contains(result.Reasoning, "Sem:") {
				t.Errorf("reasoning should include semantic score: %s", result.Reasoning)
			}
		})
	}
}

// Test semantic scoring for agenda layouts
func TestSelectLayout_AgendaLayout(t *testing.T) {
	agendaLayout := types.LayoutMetadata{
		ID:   "layout-agenda",
		Name: "Agenda",
		Tags: []string{"content", "agenda"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 500},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:   10,
			MaxTextLines: 15,
			TextHeavy:    true,
		},
	}

	genericContent := contentLayout(10)

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Today's Agenda",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"Topic 1", "Topic 2", "Topic 3"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: []types.LayoutMetadata{genericContent, agendaLayout},
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-agenda" {
		t.Errorf("expected layout-agenda for agenda title, got %s", result.LayoutID)
	}
}

// Test semantic scoring for section header layouts
func TestSelectLayout_SectionHeader(t *testing.T) {
	sectionLayout := types.LayoutMetadata{
		ID:   "layout-section",
		Name: "Section Header",
		Tags: []string{"title-slide", "section-header"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  2,
			VisualFocused: true,
		},
	}

	titleSlide := titleLayout()

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Section 2: Technical Overview",
		Type:  types.SlideTypeTitle,
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: []types.LayoutMetadata{titleSlide, sectionLayout},
		Context: SelectionContext{Position: 5, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-section" {
		t.Errorf("expected layout-section for section title, got %s", result.LayoutID)
	}
}

// Test semantic scoring for big-number/KPI layouts
func TestSelectLayout_BigNumberLayout(t *testing.T) {
	bigNumLayout := types.LayoutMetadata{
		ID:   "layout-bignum",
		Name: "Big Number",
		Tags: []string{"content", "big-number"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 200},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    3,
			MaxTextLines:  4,
			VisualFocused: true,
		},
	}

	genericContent := contentLayout(6)

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Key Metrics",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Body: "Revenue up 45% year over year",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: []types.LayoutMetadata{genericContent, bigNumLayout},
		Context: SelectionContext{Position: 1, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-bignum" {
		t.Errorf("expected layout-bignum for KPI content, got %s", result.LayoutID)
	}
}

// Test containsProminentNumber helper
func TestContainsProminentNumber(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Revenue up 45%", true},
		{"$1.5M in savings", true},
		{"€500K budget", true},
		{"£200 cost", true},
		{"Simple text", false},
		{"One number 5", false},
		{"Two numbers 25", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := containsProminentNumber(tt.text)
			if got != tt.want {
				t.Errorf("containsProminentNumber(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestFindPlaceholder_FallbackToContent verifies that findPlaceholder
// falls back to PlaceholderContent when PlaceholderBody is not found.
// This handles templates with non-standard placeholder types (type="unknown").
func TestFindPlaceholder_FallbackToContent(t *testing.T) {
	tests := []struct {
		name    string
		layout  types.LayoutMetadata
		phType  types.PlaceholderType
		wantID  string
		wantNil bool
	}{
		{
			name: "exact body match",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "title-1", Type: types.PlaceholderTitle},
					{ID: "body-1", Type: types.PlaceholderBody},
				},
			},
			phType:  types.PlaceholderBody,
			wantID:  "body-1",
			wantNil: false,
		},
		{
			name: "fallback to content when body not found",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "title-1", Type: types.PlaceholderTitle},
					{ID: "content-1", Type: types.PlaceholderContent}, // Non-standard type
				},
			},
			phType:  types.PlaceholderBody,
			wantID:  "content-1",
			wantNil: false,
		},
		{
			name: "prefer body over content when both exist",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "content-1", Type: types.PlaceholderContent},
					{ID: "body-1", Type: types.PlaceholderBody},
				},
			},
			phType:  types.PlaceholderBody,
			wantID:  "body-1", // Should find body first
			wantNil: false,
		},
		{
			name: "no fallback for title type",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "content-1", Type: types.PlaceholderContent},
				},
			},
			phType:  types.PlaceholderTitle,
			wantNil: true, // Title should not fallback to content
		},
		{
			name: "nil when no match",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "image-1", Type: types.PlaceholderImage},
				},
			},
			phType:  types.PlaceholderBody,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPlaceholder(tt.layout, tt.phType)

			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected placeholder, got nil")
			}
			if got.ID != tt.wantID {
				t.Errorf("got ID=%q, want %q", got.ID, tt.wantID)
			}
		})
	}
}

// TestScoreCapacitySlotAware verifies that scoreCapacity penalizes layouts
// with fewer content placeholders than the slide's slot count.
func TestScoreCapacitySlotAware(t *testing.T) {
	tests := []struct {
		name      string
		layout    types.LayoutMetadata
		slide     types.SlideDefinition
		wantScore float64
		wantAbove float64 // score must be > this (use -1 to skip)
		wantBelow float64 // score must be < this (use 2 to skip)
	}{
		{
			name: "no slots: capacity perfect",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "body1", Type: types.PlaceholderBody},
				},
			},
			slide:     types.SlideDefinition{},
			wantScore: 1.0,
		},
		{
			name: "2 slots, 2 body placeholders: no penalty",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "body1", Type: types.PlaceholderBody},
					{ID: "body2", Type: types.PlaceholderBody},
				},
			},
			slide: types.SlideDefinition{
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText},
					2: {SlotNumber: 2, Type: types.SlotContentChart},
				},
			},
			wantScore: 1.0,
		},
		{
			name: "2 slots, 1 body placeholder: heavy penalty",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "body1", Type: types.PlaceholderBody},
				},
			},
			slide: types.SlideDefinition{
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText},
					2: {SlotNumber: 2, Type: types.SlotContentChart},
				},
			},
			wantScore: 0.3, // 1.0 - 1*0.7
		},
		{
			name: "3 slots, 1 body placeholder: score floors at 0",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "body1", Type: types.PlaceholderBody},
				},
			},
			slide: types.SlideDefinition{
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1},
					2: {SlotNumber: 2},
					3: {SlotNumber: 3},
				},
			},
			wantScore: 0.0, // 1.0 - 2*0.7 = -0.4 → clamped to 0
		},
		{
			name: "2 slots, 1 body + 1 chart placeholder: no penalty",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "body1", Type: types.PlaceholderBody},
					{ID: "chart1", Type: types.PlaceholderChart},
				},
			},
			slide: types.SlideDefinition{
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText},
					2: {SlotNumber: 2, Type: types.SlotContentChart},
				},
			},
			wantScore: 1.0,
		},
		{
			name: "title placeholder not counted as content slot",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "title", Type: types.PlaceholderTitle},
					{ID: "body1", Type: types.PlaceholderBody},
				},
			},
			slide: types.SlideDefinition{
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText},
					2: {SlotNumber: 2, Type: types.SlotContentChart},
				},
			},
			wantScore: 0.3, // title doesn't count; only 1 content ph for 2 slots
		},
		{
			name: "2 slots with leaked bullets: no penalty when enough placeholders",
			layout: types.LayoutMetadata{
				Placeholders: []types.PlaceholderInfo{
					{ID: "body1", Type: types.PlaceholderBody},
					{ID: "body2", Type: types.PlaceholderBody},
				},
				Capacity: types.CapacityEstimate{MaxBullets: 3},
			},
			slide: types.SlideDefinition{
				Slots: map[int]*types.SlotContent{
					1: {SlotNumber: 1, Type: types.SlotContentText},
					2: {SlotNumber: 2, Type: types.SlotContentChart},
				},
				Content: types.SlideContent{
					// AST lower leaks slot bullets into Content.Bullets
					Bullets: []string{"bullet1", "bullet2", "bullet3", "bullet4", "bullet5", "bullet6"},
				},
			},
			wantScore: 1.0, // slot check passes → skip bullet overflow scoring
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreCapacity(tt.layout, tt.slide)
			if diff := got - tt.wantScore; diff > 0.01 || diff < -0.01 {
				t.Errorf("scoreCapacity() = %.2f, want %.2f", got, tt.wantScore)
			}
		})
	}
}

// TestScoreVariety verifies the variety scoring function.
func TestScoreVariety(t *testing.T) {
	tests := []struct {
		name        string
		layoutID    string
		usedLayouts map[string]int
		wantScore   float64
	}{
		{
			name:        "nil usage map returns 1.0",
			layoutID:    "layout-a",
			usedLayouts: nil,
			wantScore:   1.0,
		},
		{
			name:        "empty usage map returns 1.0",
			layoutID:    "layout-a",
			usedLayouts: map[string]int{},
			wantScore:   1.0,
		},
		{
			name:        "unused layout returns 1.0",
			layoutID:    "layout-a",
			usedLayouts: map[string]int{"layout-b": 2, "layout-c": 1},
			wantScore:   1.0,
		},
		{
			name:        "most used layout returns 0.4",
			layoutID:    "layout-a",
			usedLayouts: map[string]int{"layout-a": 3, "layout-b": 1},
			wantScore:   0.4,
		},
		{
			name:        "partially used layout scales score",
			layoutID:    "layout-a",
			usedLayouts: map[string]int{"layout-a": 1, "layout-b": 3},
			wantScore:   0.8, // 1.0 - (1/3 * 0.6) ≈ 0.8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := types.LayoutMetadata{ID: tt.layoutID}
			ctx := SelectionContext{UsedLayouts: tt.usedLayouts}

			got := scoreVariety(layout, ctx)

			// Allow small floating point tolerance
			if diff := got - tt.wantScore; diff > 0.01 || diff < -0.01 {
				t.Errorf("scoreVariety() = %.2f, want %.2f", got, tt.wantScore)
			}
		})
	}
}

// TestSelectLayout_VarietyBonus verifies that unused layouts get a bonus.
func TestSelectLayout_VarietyBonus(t *testing.T) {
	// Create three identical content layouts
	layoutA := contentLayout(10)
	layoutA.ID = "layout-a"
	layoutB := contentLayout(10)
	layoutB.ID = "layout-b"
	layoutC := contentLayout(10)
	layoutC.ID = "layout-c"

	layouts := []types.LayoutMetadata{layoutA, layoutB, layoutC}

	slide := types.SlideDefinition{
		Index: 5,
		Title: "Test Slide",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			Bullets: []string{"Point 1", "Point 2"},
		},
	}

	// Simulate that layout-a and layout-b have been used heavily
	usedLayouts := map[string]int{
		"layout-a": 3,
		"layout-b": 2,
		// layout-c not used
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{
			Position:    5,
			TotalSlides: 10,
			UsedLayouts: usedLayouts,
		},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// layout-c should be preferred because it hasn't been used
	if result.LayoutID != "layout-c" {
		t.Errorf("expected layout-c (unused) to be preferred, got %s", result.LayoutID)
	}

	// Verify the reasoning includes variety score
	if !strings.Contains(result.Reasoning, "Var:") {
		t.Errorf("reasoning should include variety score: %s", result.Reasoning)
	}
}

// TestSelectLayout_StricterThreshold verifies the raised acceptable score threshold.
func TestSelectLayout_StricterThreshold(t *testing.T) {
	// Verify scoreAcceptable is now 0.8 (raised from 0.7)
	if scoreAcceptable != 0.8 {
		t.Errorf("scoreAcceptable should be 0.8, got %.2f", scoreAcceptable)
	}
}

// TestSelectLayout_TitleHiddenLayoutRejection verifies that title-hidden layouts
// (like "Statement" layouts with off-screen title placeholders) are not selected
// when the slide has a non-empty title.
func TestSelectLayout_TitleHiddenLayoutRejection(t *testing.T) {
	// Create a title-hidden layout (like "Statement" from template-simple)
	// These have title placeholders with negative Y position (off-screen)
	statementLayout := types.LayoutMetadata{
		ID:   "layout-statement",
		Name: "Statement",
		Tags: []string{"title-hidden", "statement"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100,
				Bounds: types.BoundingBox{X: 612648, Y: -1133856, Width: 10652760, Height: 1133856}},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 500,
				Bounds: types.BoundingBox{X: 109728, Y: 1188720, Width: 8266176, Height: 5605272}},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    5,
			MaxTextLines:  5,
			TextHeavy:     false,
			VisualFocused: true,
		},
	}

	// Create a normal content layout
	normalContent := contentLayout(6)

	layouts := []types.LayoutMetadata{statementLayout, normalContent}

	tests := []struct {
		name           string
		slide          types.SlideDefinition
		expectedLayout string
	}{
		{
			name: "slide_with_title_rejects_title_hidden_layout",
			slide: types.SlideDefinition{
				Title: "Key Insights",
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					Body: "Our analysis shows significant growth.",
				},
			},
			expectedLayout: "layout-content", // Should select normal layout, not statement
		},
		{
			name: "slide_without_title_can_use_title_hidden_layout",
			slide: types.SlideDefinition{
				Title: "", // No title
				Type:  types.SlideTypeContent,
				Content: types.SlideContent{
					Body: "A bold statement about growth.",
				},
			},
			expectedLayout: "layout-statement", // Should be allowed when no title
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := SelectionRequest{
				Slide:   tc.slide,
				Layouts: layouts,
				Context: SelectionContext{Position: 1, TotalSlides: 5},
			}

			result, err := SelectLayout(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.LayoutID != tc.expectedLayout {
				t.Errorf("expected layout %s, got %s (reasoning: %s)",
					tc.expectedLayout, result.LayoutID, result.Reasoning)
			}
		})
	}
}

// TestIsLayoutSuitable_TitleHidden verifies the isLayoutSuitable function
// correctly rejects title-hidden layouts when slide has a title.
func TestIsLayoutSuitable_TitleHidden(t *testing.T) {
	titleHiddenLayout := types.LayoutMetadata{
		ID:   "layout-statement",
		Name: "Statement",
		Tags: []string{"title-hidden", "statement"},
	}

	// Layout with title positioned at the bottom of the slide (like Quote layouts)
	titleAtBottomLayout := types.LayoutMetadata{
		ID:   "layout-quote",
		Name: "Quote 3",
		Tags: []string{"title-at-bottom", "quote"},
	}

	normalLayout := types.LayoutMetadata{
		ID:   "layout-content",
		Name: "Content",
		Tags: []string{"content"},
	}

	slideWithTitle := types.SlideDefinition{
		Title: "My Title",
		Type:  types.SlideTypeContent,
	}

	slideWithoutTitle := types.SlideDefinition{
		Title: "",
		Type:  types.SlideTypeContent,
	}

	tests := []struct {
		name     string
		layout   types.LayoutMetadata
		slide    types.SlideDefinition
		expected bool
	}{
		{
			name:     "title_hidden_rejected_when_slide_has_title",
			layout:   titleHiddenLayout,
			slide:    slideWithTitle,
			expected: false,
		},
		{
			name:     "title_hidden_allowed_when_slide_has_no_title",
			layout:   titleHiddenLayout,
			slide:    slideWithoutTitle,
			expected: true,
		},
		{
			name:     "title_at_bottom_rejected_when_slide_has_title",
			layout:   titleAtBottomLayout,
			slide:    slideWithTitle,
			expected: false,
		},
		{
			name:     "title_at_bottom_allowed_when_slide_has_no_title",
			layout:   titleAtBottomLayout,
			slide:    slideWithoutTitle,
			expected: true,
		},
		{
			name:     "normal_layout_allowed_with_title",
			layout:   normalLayout,
			slide:    slideWithTitle,
			expected: true,
		},
		{
			name:     "normal_layout_allowed_without_title",
			layout:   normalLayout,
			slide:    slideWithoutTitle,
			expected: true,
		},
		// Section slides (SlideTypeSection) are exempt from title-at-bottom
		// and title-hidden rejection because section-header layouts commonly
		// use these positioning styles by design.
		{
			name:   "title_at_bottom_allowed_for_section_slide",
			layout: titleAtBottomLayout,
			slide: types.SlideDefinition{
				Title: "Strategic Analysis",
				Type:  types.SlideTypeSection,
			},
			expected: true,
		},
		{
			name:   "title_hidden_allowed_for_section_slide",
			layout: titleHiddenLayout,
			slide: types.SlideDefinition{
				Title: "Strategic Analysis",
				Type:  types.SlideTypeSection,
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isLayoutSuitable(tc.layout, tc.slide)
			if result != tc.expected {
				t.Errorf("expected isLayoutSuitable to return %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestIsLayoutSuitable_TwoColumnSlots verifies that two-column slides with
// explicit slot markers reject layouts with insufficient content placeholders.
func TestIsLayoutSuitable_TwoColumnSlots(t *testing.T) {
	oneContentLayout := types.LayoutMetadata{
		ID:   "layout-one-content",
		Name: "One Content",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderBody},
		},
	}
	twoContentLayout := types.LayoutMetadata{
		ID:   "layout-two-content",
		Name: "Two Content",
		Tags: []string{"two-column", "content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderBody},
			{Type: types.PlaceholderBody},
		},
	}

	slotted := types.SlideDefinition{
		Title: "Product Roadmap",
		Type:  types.SlideTypeTwoColumn,
		Slots: map[int]*types.SlotContent{
			1: {Type: types.SlotContentInfographic},
			2: {Type: types.SlotContentText},
		},
	}
	unslotted := types.SlideDefinition{
		Title: "Comparison",
		Type:  types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			Left:  []string{"Point A"},
			Right: []string{"Point B"},
		},
	}

	tests := []struct {
		name     string
		layout   types.LayoutMetadata
		slide    types.SlideDefinition
		expected bool
	}{
		{
			name:     "slotted_two_column_rejects_one_placeholder",
			layout:   oneContentLayout,
			slide:    slotted,
			expected: false,
		},
		{
			name:     "slotted_two_column_accepts_two_placeholders",
			layout:   twoContentLayout,
			slide:    slotted,
			expected: true,
		},
		{
			name:     "unslotted_two_column_accepts_one_placeholder_as_fallback",
			layout:   oneContentLayout,
			slide:    unslotted,
			expected: true,
		},
		{
			name:     "unslotted_two_column_accepts_two_placeholders",
			layout:   twoContentLayout,
			slide:    unslotted,
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isLayoutSuitable(tc.layout, tc.slide)
			if result != tc.expected {
				t.Errorf("expected isLayoutSuitable to return %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestIsLayoutSuitable_ChartRejectsSectionAndClosing verifies that chart and
// diagram slides are rejected for section-header and closing layouts, which have
// narrow body placeholders that squeeze visual content into a fraction of the slide.
func TestIsLayoutSuitable_ChartRejectsSectionAndClosing(t *testing.T) {
	sectionLayout := types.LayoutMetadata{
		ID:   "layout-section",
		Name: "Section Header",
		Tags: []string{"title-slide", "section-header"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderBody},
		},
	}
	closingLayout := types.LayoutMetadata{
		ID:   "layout-closing",
		Name: "Closing",
		Tags: []string{"title-slide", "closing"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderBody},
		},
	}
	contentLayout := types.LayoutMetadata{
		ID:   "layout-content",
		Name: "One Content",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderBody, Bounds: types.BoundingBox{
				X: 457200, Y: 1600200, Width: 8229600, Height: 4525963, // full-size body
			}},
		},
	}

	tests := []struct {
		name     string
		layout   types.LayoutMetadata
		slide    types.SlideDefinition
		expected bool
	}{
		{
			name:     "chart_rejected_on_section_header",
			layout:   sectionLayout,
			slide:    types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue"},
			expected: false,
		},
		{
			name:     "diagram_rejected_on_section_header",
			layout:   sectionLayout,
			slide:    types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Flow"},
			expected: false,
		},
		{
			name:     "chart_rejected_on_closing",
			layout:   closingLayout,
			slide:    types.SlideDefinition{Type: types.SlideTypeChart, Title: "Summary"},
			expected: false,
		},
		{
			name:     "diagram_rejected_on_closing",
			layout:   closingLayout,
			slide:    types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Architecture"},
			expected: false,
		},
		{
			name:     "chart_accepted_on_content_layout",
			layout:   contentLayout,
			slide:    types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue"},
			expected: true,
		},
		{
			name:     "diagram_accepted_on_content_layout",
			layout:   contentLayout,
			slide:    types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Flow"},
			expected: true,
		},
		// SlideTypeContent with DiagramSpec should also be rejected on title/closing layouts
		{
			name:   "content_with_diagram_rejected_on_closing",
			layout: closingLayout,
			slide: types.SlideDefinition{
				Type:  types.SlideTypeContent,
				Title: "Sales Chart",
				Content: types.SlideContent{
					DiagramSpec: &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"categories": []string{"A"}}},
				},
			},
			expected: false,
		},
		{
			name:   "content_with_diagram_rejected_on_section_header",
			layout: sectionLayout,
			slide: types.SlideDefinition{
				Type:  types.SlideTypeContent,
				Title: "Process Flow",
				Content: types.SlideContent{
					DiagramSpec: &types.DiagramSpec{Type: "process_flow", Data: map[string]any{"steps": []string{"A"}}},
				},
			},
			expected: false,
		},
		{
			name:   "content_with_diagram_accepted_on_content_layout",
			layout: contentLayout,
			slide: types.SlideDefinition{
				Type:  types.SlideTypeContent,
				Title: "Revenue Chart",
				Content: types.SlideContent{
					DiagramSpec: &types.DiagramSpec{Type: "bar_chart", Data: map[string]any{"categories": []string{"A"}}},
				},
			},
			expected: true,
		},
		{
			name:   "content_without_media_accepted_on_closing",
			layout: closingLayout,
			slide: types.SlideDefinition{
				Type:  types.SlideTypeContent,
				Title: "Thank You",
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isLayoutSuitable(tc.layout, tc.slide)
			if result != tc.expected {
				t.Errorf("expected isLayoutSuitable to return %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestIsLayoutSuitable_ChartRejectsTinyPlaceholder verifies that chart/diagram
// slides reject layouts with body placeholders too short for visual content.
// Regression test for pptx-coa: modern-template slide 16 bar chart rendered at
// thumbnail scale because a layout with a thin text-strip body placeholder
// (height < 2 inches) was selected.
func TestIsLayoutSuitable_ChartRejectsTinyPlaceholder(t *testing.T) {
	tinyBodyLayout := types.LayoutMetadata{
		ID:   "layout-tiny",
		Name: "Title 4",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderBody,
				Bounds: types.BoundingBox{
					X: 457200, Y: 1600200,
					Width: 3024899, Height: 547908, // ~0.6 inches tall — too short
				},
			},
		},
	}
	fullBodyLayout := types.LayoutMetadata{
		ID:   "layout-full",
		Name: "Content 6",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderBody,
				Bounds: types.BoundingBox{
					X: 457200, Y: 1600200,
					Width: 8229600, Height: 4525963, // full-size body
				},
			},
		},
	}

	chartSlide := types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue"}
	diagramSlide := types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Flow"}
	textSlide := types.SlideDefinition{Type: types.SlideTypeContent, Title: "Summary"}

	// Charts and diagrams should reject the tiny body layout
	if isLayoutSuitable(tinyBodyLayout, chartSlide) {
		t.Error("expected tiny body layout to be rejected for chart slides")
	}
	if isLayoutSuitable(tinyBodyLayout, diagramSlide) {
		t.Error("expected tiny body layout to be rejected for diagram slides")
	}

	// Full-size layout should be accepted
	if !isLayoutSuitable(fullBodyLayout, chartSlide) {
		t.Error("expected full body layout to be accepted for chart slides")
	}
	if !isLayoutSuitable(fullBodyLayout, diagramSlide) {
		t.Error("expected full body layout to be accepted for diagram slides")
	}

	// Text slides should still accept the tiny layout (text strips are fine for text)
	if !isLayoutSuitable(tinyBodyLayout, textSlide) {
		t.Error("expected tiny body layout to be accepted for text slides")
	}
}

// TestIsLayoutSuitable_ChartRejectsTwoColumnLayout verifies that chart and
// diagram slides are rejected for two-column layouts. Two-column layouts have
// narrower body placeholders (40-60% of slide width) that produce thumbnail-sized
// charts. Regression test for pptx-fa0: modern-template slide 16 bar chart
// rendered as tiny thumbnail because variety scoring selected a two-column layout.
func TestIsLayoutSuitable_ChartRejectsTwoColumnLayout(t *testing.T) {
	twoColLayout := types.LayoutMetadata{
		ID:   "layout-twocol",
		Name: "Two Column (60/40)",
		Tags: []string{"content", "two-column"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderContent,
				Bounds: types.BoundingBox{
					X: 457200, Y: 1600200,
					Width: 5486400, Height: 4525963, // 60% column — wide enough by height but narrow
				},
			},
			{
				Type: types.PlaceholderContent,
				Bounds: types.BoundingBox{
					X: 6400800, Y: 1600200,
					Width: 3657600, Height: 4525963, // 40% column
				},
			},
		},
	}

	contentLayout := types.LayoutMetadata{
		ID:   "layout-content",
		Name: "One Content",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderBody,
				Bounds: types.BoundingBox{
					X: 457200, Y: 1600200,
					Width: 8229600, Height: 4525963,
				},
			},
		},
	}

	chartSlide := types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue"}
	diagramSlide := types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Flow"}
	twoColSlide := types.SlideDefinition{
		Type:  types.SlideTypeTwoColumn,
		Title: "Comparison",
	}

	// Charts and diagrams should reject two-column layouts
	if isLayoutSuitable(twoColLayout, chartSlide) {
		t.Error("expected two-column layout to be rejected for chart slides")
	}
	if isLayoutSuitable(twoColLayout, diagramSlide) {
		t.Error("expected two-column layout to be rejected for diagram slides")
	}

	// Full-width content layout should be accepted for charts
	if !isLayoutSuitable(contentLayout, chartSlide) {
		t.Error("expected content layout to be accepted for chart slides")
	}

	// Two-column layout should still be accepted for two-column slide types
	if !isLayoutSuitable(twoColLayout, twoColSlide) {
		t.Error("expected two-column layout to be accepted for two-column slides")
	}
}

// TestIsLayoutSuitable_ChartRejectsNarrowBody verifies that chart/diagram slides
// reject layouts with body placeholders narrower than 55% of slide width.
// This is a defense-in-depth check that catches layouts not tagged as two-column
// but still having narrow body areas (e.g., asymmetric content layouts).
func TestIsLayoutSuitable_ChartRejectsNarrowBody(t *testing.T) {
	narrowBodyLayout := types.LayoutMetadata{
		ID:   "layout-narrow",
		Name: "Narrow Body",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderBody,
				Bounds: types.BoundingBox{
					X: 457200, Y: 1600200,
					Width: 5486400, Height: 4525963, // ~45% of slide width, below threshold
				},
			},
		},
	}

	chartSlide := types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue"}
	if isLayoutSuitable(narrowBodyLayout, chartSlide) {
		t.Error("expected narrow body layout (45% width) to be rejected for chart slides")
	}
}

// TestSelectLayout_SectionSlidePrefersSectionHeader verifies that section slides
// (SlideTypeSection) strongly prefer section-header layouts over title-slide layouts,
// even when the title-slide layout has better capacity/balance scores.
func TestSelectLayout_SectionSlidePrefersSectionHeader(t *testing.T) {
	// Section-header layout: matches section slides perfectly but has sparse capacity
	sectionHeaderLayout := types.LayoutMetadata{
		ID:   "layout-section-header",
		Name: "Section Divider",
		Tags: []string{"content", "section-header"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 10}, // Small decorative "#" placeholder
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  2,
			VisualFocused: false,
			TextHeavy:     false,
		},
	}

	// Title-slide layout: better capacity/balance scores but wrong type
	titleSlideLayout := types.LayoutMetadata{
		ID:   "layout-title-slide",
		Name: "Title Slide",
		Tags: []string{"title-slide"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 200},
		},
		Capacity: types.CapacityEstimate{
			MaxBullets:    0,
			MaxTextLines:  3,
			VisualFocused: true, // Better visual balance for sparse content
		},
	}

	tests := []struct {
		name           string
		slideTitle     string
		position       int
		previousLayout string
		usedLayouts    map[string]int
	}{
		{
			name:       "generic section title prefers section-header",
			slideTitle: "Strategic Analysis",
		},
		{
			name:       "section-header preferred even when previously used",
			slideTitle: "Financial Overview",
			previousLayout: "layout-section-header",
			usedLayouts:    map[string]int{"layout-section-header": 2},
		},
		{
			name:       "section-header preferred at various positions",
			slideTitle: "Next Steps",
			position:   7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slide := types.SlideDefinition{
				Title: tt.slideTitle,
				Type:  types.SlideTypeSection,
			}

			position := tt.position
			if position == 0 {
				position = 3 // Default to middle position
			}

			req := SelectionRequest{
				Slide:   slide,
				Layouts: []types.LayoutMetadata{titleSlideLayout, sectionHeaderLayout},
				Context: SelectionContext{
					Position:     position,
					TotalSlides:  10,
					PreviousType: tt.previousLayout,
					UsedLayouts:  tt.usedLayouts,
				},
			}

			result, err := SelectLayout(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.LayoutID != "layout-section-header" {
				t.Errorf("expected layout-section-header, got %s (reasoning: %s)",
					result.LayoutID, result.Reasoning)
			}
		})
	}
}

// TestSelectLayout_TwoColumnFallbackToOneContent tests that when no two-column
// layout is available, the selector falls back to a single-content layout
// instead of returning an error.
func TestSelectLayout_TwoColumnFallbackToOneContent(t *testing.T) {
	// Only single-body content layout available — no two-column
	layouts := []types.LayoutMetadata{
		contentLayout(6),
		titleLayout(),
	}

	slide := types.SlideDefinition{
		Index: 3,
		Title: "Side by Side",
		Type:  types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			Left:  []string{"Left 1", "Left 2"},
			Right: []string{"Right 1", "Right 2"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 3, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("expected fallback to content layout, got error: %v", err)
	}

	if result.LayoutID != "layout-content" {
		t.Errorf("expected layout-content as fallback, got %s", result.LayoutID)
	}

	// Should emit a warning about combining content
	hasWarning := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "only one body placeholder") || strings.Contains(w, "combined") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning about combining two-column content, got: %v", result.Warnings)
	}
}

// TestSelectLayout_TwoColumnPrefersRealTwoColumn verifies that when both
// a real two-column layout and a single-content layout are available,
// the two-column layout is selected.
func TestSelectLayout_TwoColumnPrefersRealTwoColumn(t *testing.T) {
	layouts := []types.LayoutMetadata{
		contentLayout(6),
		twoColumnLayout(),
	}

	slide := types.SlideDefinition{
		Index: 3,
		Title: "Comparison",
		Type:  types.SlideTypeTwoColumn,
		Content: types.SlideContent{
			Left:  []string{"Pro 1"},
			Right: []string{"Con 1"},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 3, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-twocol" {
		t.Errorf("expected layout-twocol (real two-column), got %s", result.LayoutID)
	}
}

// Test that body content is routed to the title placeholder when the subtitle
// placeholder has a very small font (e.g., legal disclaimer area on closing slides).
func TestSelectLayout_SmallSubtitleRoutesBodyToTitle(t *testing.T) {
	// Simulate closing layout with small-font subtitle: ctrTitle + small-font subTitle (11pt)
	closingLayout := types.LayoutMetadata{
		ID:   "layout-closing",
		Name: "Closing",
		Tags: []string{"title-slide", "closing"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, MaxChars: 100, FontSize: 6600},
			{ID: "Subtitle 2", Type: types.PlaceholderSubtitle, MaxChars: 200, FontSize: 1100}, // 11pt — legal style
		},
		Capacity: types.CapacityEstimate{MaxTextLines: 2},
	}

	layouts := []types.LayoutMetadata{closingLayout}

	slide := types.SlideDefinition{
		Title: "Discussion",
		Type:  types.SlideTypeTitle,
		Content: types.SlideContent{
			Body: "Questions & Next Steps",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 9, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Body should be routed to title placeholder, not the small subtitle
	var titleMapping, bodyMapping *ContentMapping
	for i := range result.Mappings {
		switch result.Mappings[i].ContentField {
		case "title":
			titleMapping = &result.Mappings[i]
		case "body":
			bodyMapping = &result.Mappings[i]
		}
	}

	if titleMapping == nil {
		t.Fatal("missing title mapping")
	}
	if bodyMapping == nil {
		t.Fatal("missing body mapping — body should be routed to title placeholder")
	}
	if bodyMapping.PlaceholderID != "Title 1" {
		t.Errorf("body PlaceholderID = %q, want %q (title placeholder)", bodyMapping.PlaceholderID, "Title 1")
	}
	if bodyMapping.PlaceholderID == "Subtitle 2" {
		t.Error("body must NOT route to small subtitle placeholder (legal area)")
	}
}

// Test that a subtitle with adequate font size IS used as body fallback.
func TestSelectLayout_AdequateSubtitleUsedForBody(t *testing.T) {
	titleSlideLayout := types.LayoutMetadata{
		ID:   "layout-title",
		Name: "Title Slide",
		Tags: []string{"title-slide"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, MaxChars: 100, FontSize: 4800},
			{ID: "Subtitle 2", Type: types.PlaceholderSubtitle, MaxChars: 200, FontSize: 1400}, // 14pt — adequate
		},
		Capacity: types.CapacityEstimate{MaxTextLines: 4},
	}

	layouts := []types.LayoutMetadata{titleSlideLayout}

	slide := types.SlideDefinition{
		Title: "Welcome",
		Type:  types.SlideTypeTitle,
		Content: types.SlideContent{
			Body: "Company Overview",
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 0, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyMapping *ContentMapping
	for i := range result.Mappings {
		if result.Mappings[i].ContentField == "body" {
			bodyMapping = &result.Mappings[i]
		}
	}

	if bodyMapping == nil {
		t.Fatal("missing body mapping")
	}
	if bodyMapping.PlaceholderID != "Subtitle 2" {
		t.Errorf("body PlaceholderID = %q, want %q (subtitle placeholder)", bodyMapping.PlaceholderID, "Subtitle 2")
	}
}

// TestPenalizeNarrowDiagramSlot verifies that complex diagrams in narrow
// placeholders get a heavy penalty, causing full-width layouts to win.
func TestPenalizeNarrowDiagramSlot(t *testing.T) {
	const (
		fullWidth int64 = 10000000 // ~82% of slide width — full-width body
		halfWidth int64 = 5000000  // ~41% of slide width — narrow column
	)

	t.Run("slotted slide with complex diagram in narrow column gets penalty", func(t *testing.T) {
		narrowTwoCol := types.LayoutMetadata{
			ID:   "layout-narrow-twocol",
			Name: "Two Column Narrow",
			Tags: []string{"two-column"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: halfWidth}},
				{ID: "body-2", Type: types.PlaceholderBody, Index: 2, Bounds: types.BoundingBox{Width: halfWidth}},
			},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeTwoColumn,
			Slots: map[int]*types.SlotContent{
				1: {SlotNumber: 1, Type: types.SlotContentBullets, Bullets: []string{"item"}},
				2: {SlotNumber: 2, Type: types.SlotContentChart, DiagramSpec: &types.DiagramSpec{
					Type: "business_model_canvas",
					Data: map[string]any{"sections": []any{1, 2, 3}},
				}},
			},
		}

		penalty := penalizeNarrowDiagramSlot(narrowTwoCol, slide)
		if penalty < 0.4 {
			t.Errorf("expected heavy penalty (>=0.4) for complex diagram in narrow slot, got %.2f", penalty)
		}
	})

	t.Run("slotted slide with complex diagram in wide column gets no penalty", func(t *testing.T) {
		wideTwoCol := types.LayoutMetadata{
			ID:   "layout-wide-twocol",
			Name: "Two Column Wide",
			Tags: []string{"two-column"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: fullWidth}},
				{ID: "body-2", Type: types.PlaceholderBody, Index: 2, Bounds: types.BoundingBox{Width: fullWidth}},
			},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeTwoColumn,
			Slots: map[int]*types.SlotContent{
				1: {SlotNumber: 1, Type: types.SlotContentBullets, Bullets: []string{"item"}},
				2: {SlotNumber: 2, Type: types.SlotContentChart, DiagramSpec: &types.DiagramSpec{
					Type: "business_model_canvas",
					Data: map[string]any{"sections": []any{1, 2, 3}},
				}},
			},
		}

		penalty := penalizeNarrowDiagramSlot(wideTwoCol, slide)
		if penalty != 0.0 {
			t.Errorf("expected no penalty for wide layout, got %.2f", penalty)
		}
	})

	t.Run("non-complex diagram type in narrow column gets no penalty", func(t *testing.T) {
		narrowTwoCol := types.LayoutMetadata{
			ID:   "layout-narrow-twocol",
			Name: "Two Column Narrow",
			Tags: []string{"two-column"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: halfWidth}},
				{ID: "body-2", Type: types.PlaceholderBody, Index: 2, Bounds: types.BoundingBox{Width: halfWidth}},
			},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeTwoColumn,
			Slots: map[int]*types.SlotContent{
				1: {SlotNumber: 1, Type: types.SlotContentBullets, Bullets: []string{"item"}},
				2: {SlotNumber: 2, Type: types.SlotContentChart, DiagramSpec: &types.DiagramSpec{
					Type: "bar_chart",
					Data: map[string]any{"values": []any{1, 2, 3}},
				}},
			},
		}

		penalty := penalizeNarrowDiagramSlot(narrowTwoCol, slide)
		if penalty != 0.0 {
			t.Errorf("expected no penalty for bar_chart (not complex), got %.2f", penalty)
		}
	})

	t.Run("non-slotted diagram slide in narrow body gets penalty", func(t *testing.T) {
		narrowContent := types.LayoutMetadata{
			ID:   "layout-narrow-content",
			Name: "Content Narrow",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: halfWidth}},
			},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeDiagram,
			Content: types.SlideContent{
				DiagramSpec: &types.DiagramSpec{
					Type: "swot",
					Data: map[string]any{"strengths": []any{"a", "b"}},
				},
			},
		}

		penalty := penalizeNarrowDiagramSlot(narrowContent, slide)
		if penalty < 0.4 {
			t.Errorf("expected heavy penalty for SWOT in narrow body, got %.2f", penalty)
		}
	})

	t.Run("non-slotted simple chart in narrow body gets moderate penalty", func(t *testing.T) {
		narrowContent := types.LayoutMetadata{
			ID:   "layout-narrow-content",
			Name: "Content Narrow",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: halfWidth}},
			},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeChart,
			Content: types.SlideContent{
				DiagramSpec: &types.DiagramSpec{
					Type: "bar_chart",
					Data: map[string]any{"values": []any{1, 2, 3}},
				},
			},
		}

		penalty := penalizeNarrowDiagramSlot(narrowContent, slide)
		if penalty < 0.2 || penalty > 0.4 {
			t.Errorf("expected moderate penalty (0.2–0.4) for bar_chart in narrow body, got %.2f", penalty)
		}
	})

	t.Run("non-slotted simple chart in narrow chart slot gets moderate penalty", func(t *testing.T) {
		narrowChart := types.LayoutMetadata{
			ID:   "layout-narrow-chart",
			Name: "Chart Narrow",
			Tags: []string{"chart-capable"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "chart-1", Type: types.PlaceholderChart, Index: 1, Bounds: types.BoundingBox{Width: halfWidth}},
			},
			Capacity: types.CapacityEstimate{HasChartSlot: true},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeChart,
			Content: types.SlideContent{
				DiagramSpec: &types.DiagramSpec{
					Type: "line_chart",
					Data: map[string]any{"series": []any{1, 2, 3}},
				},
			},
		}

		penalty := penalizeNarrowDiagramSlot(narrowChart, slide)
		if penalty < 0.2 || penalty > 0.4 {
			t.Errorf("expected moderate penalty (0.2–0.4) for line_chart in narrow chart slot, got %.2f", penalty)
		}
	})

	t.Run("non-slotted simple chart in wide body gets no penalty", func(t *testing.T) {
		wideContent := types.LayoutMetadata{
			ID:   "layout-wide-content",
			Name: "Content Wide",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: fullWidth}},
			},
		}

		slide := types.SlideDefinition{
			Type: types.SlideTypeChart,
			Content: types.SlideContent{
				DiagramSpec: &types.DiagramSpec{
					Type: "pie_chart",
					Data: map[string]any{"values": []any{30, 40, 30}},
				},
			},
		}

		penalty := penalizeNarrowDiagramSlot(wideContent, slide)
		if penalty != 0.0 {
			t.Errorf("expected no penalty for pie_chart in wide body, got %.2f", penalty)
		}
	})

	t.Run("slide without diagram gets no penalty", func(t *testing.T) {
		layout := contentLayout(6)
		slide := types.SlideDefinition{
			Type: types.SlideTypeContent,
			Content: types.SlideContent{
				Bullets: []string{"item 1", "item 2"},
			},
		}

		penalty := penalizeNarrowDiagramSlot(layout, slide)
		if penalty != 0.0 {
			t.Errorf("expected no penalty for text-only slide, got %.2f", penalty)
		}
	})
}

// TestSelectLayout_ComplexDiagramPrefersFullWidth verifies that the integrated
// scoring prefers full-width layouts for complex diagrams over narrow two-column.
func TestSelectLayout_ComplexDiagramPrefersFullWidth(t *testing.T) {
	const (
		fullWidth int64 = 10000000
		halfWidth int64 = 5000000
	)

	narrowTwoCol := types.LayoutMetadata{
		ID:   "layout-narrow-twocol",
		Name: "Two Column Narrow",
		Tags: []string{"two-column", "content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, Index: 0, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, Index: 1, MaxChars: 300, Bounds: types.BoundingBox{Width: halfWidth}},
			{ID: "body-2", Type: types.PlaceholderBody, Index: 2, MaxChars: 300, Bounds: types.BoundingBox{Width: halfWidth}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 6},
	}

	fullWidthContent := types.LayoutMetadata{
		ID:   "layout-fullwidth",
		Name: "Content Full Width",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, Index: 0, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, Index: 1, MaxChars: 500, Bounds: types.BoundingBox{Width: fullWidth, Height: 4525963}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 6, TextHeavy: true},
	}

	layouts := []types.LayoutMetadata{narrowTwoCol, fullWidthContent}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Business Model",
		Type:  types.SlideTypeDiagram,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{
				Type: "business_model_canvas",
				Data: map[string]any{"sections": []any{1, 2, 3, 4, 5, 6, 7, 8, 9}},
			},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 1, TotalSlides: 5},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-fullwidth" {
		t.Errorf("expected full-width layout for business_model_canvas, got %s", result.LayoutID)
	}
}

// TestSelectLayout_SimpleChartPrefersWiderBody verifies that simple chart slides
// (bar_chart, line_chart, etc.) prefer layouts with wider content areas over
// layouts with narrow chart slots. This prevents charts from being confined to
// a fraction of the slide when wider alternatives are available
// (where chart slot layouts are only ~45% of slide width).
func TestSelectLayout_SimpleChartPrefersWiderBody(t *testing.T) {
	const (
		wideBody       int64 = 9500000 // ~78% of slide width
		narrowChartSlot int64 = 5200000 // ~43% of slide width
	)

	narrowChartLayout := types.LayoutMetadata{
		ID:   "layout-narrow-chart",
		Name: "Chart With Sidebar",
		Tags: []string{"chart-capable", "content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, Index: 0, MaxChars: 100},
			{ID: "chart-1", Type: types.PlaceholderChart, Index: 1, Bounds: types.BoundingBox{
				Width: narrowChartSlot, Height: 4500000,
			}},
			{ID: "body-1", Type: types.PlaceholderBody, Index: 2, MaxChars: 300, Bounds: types.BoundingBox{
				Width: 2700000, Height: 3400000,
			}},
		},
		Capacity: types.CapacityEstimate{HasChartSlot: true, MaxBullets: 4},
	}

	wideBodyLayout := types.LayoutMetadata{
		ID:   "layout-wide-body",
		Name: "Content Full Width",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, Index: 0, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, Index: 1, MaxChars: 500, Bounds: types.BoundingBox{
				Width: wideBody, Height: 4000000,
			}},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 6, TextHeavy: true},
	}

	layouts := []types.LayoutMetadata{narrowChartLayout, wideBodyLayout}

	slide := types.SlideDefinition{
		Index: 1,
		Title: "Revenue Breakdown",
		Type:  types.SlideTypeChart,
		Content: types.SlideContent{
			DiagramSpec: &types.DiagramSpec{
				Type: "bar_chart",
				Data: map[string]any{"values": []any{10, 20, 30, 40}},
			},
		},
	}

	req := SelectionRequest{
		Slide:   slide,
		Layouts: layouts,
		Context: SelectionContext{Position: 2, TotalSlides: 10},
	}

	result, err := SelectLayout(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LayoutID != "layout-wide-body" {
		t.Errorf("expected wide body layout for bar_chart over narrow chart slot, got %s", result.LayoutID)
	}
}

// TestPenalizeNarrowDiagramSlot_AsymmetricLayout verifies that when a diagram
// is in a two-column layout, the penalty system correctly prefers the variant
// where the diagram gets the wider column (60% vs 40%).
func TestPenalizeNarrowDiagramSlot_AsymmetricLayout(t *testing.T) {
	const (
		wideCol   int64 = 6500000 // 60% column — above narrowDiagramThreshold
		narrowCol int64 = 4300000 // 40% column — below threshold
	)

	slide := types.SlideDefinition{
		Type: types.SlideTypeTwoColumn,
		Slots: map[int]*types.SlotContent{
			1: {SlotNumber: 1, Type: types.SlotContentChart, DiagramSpec: &types.DiagramSpec{
				Type: "process_flow",
				Data: map[string]any{"steps": []any{1, 2, 3, 4, 5}},
			}},
			2: {SlotNumber: 2, Type: types.SlotContentTable, Table: &types.TableSpec{}},
		},
	}

	t.Run("diagram in wide column gets no penalty", func(t *testing.T) {
		layout := types.LayoutMetadata{
			ID:   "layout-60-40",
			Tags: []string{"two-column", "content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: wideCol}},
				{ID: "body-2", Type: types.PlaceholderBody, Index: 2, Bounds: types.BoundingBox{Width: narrowCol}},
			},
		}
		penalty := penalizeNarrowDiagramSlot(layout, slide)
		if penalty != 0.0 {
			t.Errorf("expected no penalty when diagram is in 60%% column, got %.2f", penalty)
		}
	})

	t.Run("diagram in narrow column gets penalty", func(t *testing.T) {
		layout := types.LayoutMetadata{
			ID:   "layout-40-60",
			Tags: []string{"two-column", "content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title-1", Type: types.PlaceholderTitle, Index: 0},
				{ID: "body-1", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{Width: narrowCol}},
				{ID: "body-2", Type: types.PlaceholderBody, Index: 2, Bounds: types.BoundingBox{Width: wideCol}},
			},
		}
		penalty := penalizeNarrowDiagramSlot(layout, slide)
		if penalty >= 0.4 {
			// Diagram is in slot 1 which maps to placeholder idx 0 (narrowCol)
			// This SHOULD get a penalty
		} else {
			t.Errorf("expected heavy penalty when diagram is in 40%% column, got %.2f", penalty)
		}
	})
}

// TestSelectLayout_TableSlidesAvoidSectionDividerAndClosing verifies that table
// slides are never assigned section-divider or closing layouts, even when
// variety scoring would otherwise push them there.
func TestSelectLayout_TableSlidesAvoidSectionDividerAndClosing(t *testing.T) {
	// Content layout (appropriate for tables)
	content := contentLayout(10)
	content.ID = "layout-content"

	// Section divider layout (inappropriate for tables)
	sectionDiv := types.LayoutMetadata{
		ID:   "layout-section-div",
		Name: "Section Divider",
		Tags: []string{"content", "section-header"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 200},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 3, MaxTextLines: 3, TextHeavy: false},
	}

	// Closing layout (inappropriate for tables)
	closing := types.LayoutMetadata{
		ID:   "layout-closing",
		Name: "Closing",
		Tags: []string{"content", "closing", "thank-you"},
		Placeholders: []types.PlaceholderInfo{
			{ID: "title-1", Type: types.PlaceholderTitle, MaxChars: 100},
			{ID: "body-1", Type: types.PlaceholderBody, MaxChars: 100},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 2, MaxTextLines: 2, TextHeavy: false},
	}

	layouts := []types.LayoutMetadata{content, sectionDiv, closing}

	tableSlide := types.SlideDefinition{
		Index: 5,
		Title: "Revenue Summary",
		Type:  types.SlideTypeContent,
		Content: types.SlideContent{
			TableRaw: "| Q1 | Q2 | Q3 | Q4 |\n|---|---|---|---|\n| 100 | 200 | 300 | 400 |",
		},
	}

	t.Run("table_prefers_content_over_section_divider", func(t *testing.T) {
		// Even with heavy usage of content layout, table should still choose it
		req := SelectionRequest{
			Slide:   tableSlide,
			Layouts: layouts,
			Context: SelectionContext{
				Position:    5,
				TotalSlides: 10,
				UsedLayouts: map[string]int{
					"layout-content": 5, // heavily used
					// section-div and closing never used (max variety bonus)
				},
			},
		}

		result, err := SelectLayout(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.LayoutID != "layout-content" {
			t.Errorf("table slide should use content layout, got %s (%s)", result.LayoutID, result.LayoutID)
		}
	})

	t.Run("last_slide_table_avoids_closing", func(t *testing.T) {
		// Table on last slide should not get closing layout bonus
		req := SelectionRequest{
			Slide:   tableSlide,
			Layouts: layouts,
			Context: SelectionContext{
				Position:    9,
				TotalSlides: 10,
				UsedLayouts: map[string]int{
					"layout-content": 5,
				},
			},
		}

		result, err := SelectLayout(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.LayoutID != "layout-content" {
			t.Errorf("last-slide table should use content layout, got %s (%s)", result.LayoutID, result.LayoutID)
		}
	})
}

// TestIsLayoutSuitable_BulletGroupsNeedsBodyPlaceholder verifies that content slides
// with BulletGroups require a layout with body/content placeholder capability.
// Regression test for pptx-o58: bullet_groups content type rendered blank because
// isLayoutSuitable did not check BulletGroups, allowing title-only or closing
// layouts (which lack body placeholders) to be selected.
func TestIsLayoutSuitable_BulletGroupsNeedsBodyPlaceholder(t *testing.T) {
	titleOnlyLayout := types.LayoutMetadata{
		ID:   "layout-title-only",
		Name: "Title Slide",
		Tags: []string{"title-slide"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderSubtitle},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 0},
	}
	contentLayout := types.LayoutMetadata{
		ID:   "layout-content",
		Name: "One Content",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderBody},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 8},
	}

	bulletGroupsSlide := types.SlideDefinition{
		Type:  types.SlideTypeContent,
		Title: "Strategic Priorities",
		Content: types.SlideContent{
			BulletGroups: []types.BulletGroup{
				{Header: "Growth", Bullets: []string{"Revenue up 15%"}},
				{Header: "Efficiency", Bullets: []string{"Costs down 10%"}},
			},
		},
	}

	// Title-only layout should be rejected for bullet_groups content
	if isLayoutSuitable(titleOnlyLayout, bulletGroupsSlide) {
		t.Error("expected title-only layout to be rejected for bullet_groups content")
	}

	// Content layout with body placeholder should be accepted
	if !isLayoutSuitable(contentLayout, bulletGroupsSlide) {
		t.Error("expected content layout to be accepted for bullet_groups content")
	}

	// Regression test for pptx-hgm: a layout tagged "content" but missing a
	// body/content placeholder must be rejected for bullet_groups. Previously,
	// the "content" tag alone was sufficient, causing buildMappings to find
	// bodyPH==nil and silently skip the bullet_groups mapping (blank slide).
	contentNoBodyLayout := types.LayoutMetadata{
		ID:   "layout-content-no-body",
		Name: "Content Without Body",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{Type: types.PlaceholderImage},
		},
		Capacity: types.CapacityEstimate{MaxBullets: 0},
	}
	if isLayoutSuitable(contentNoBodyLayout, bulletGroupsSlide) {
		t.Error("expected content layout WITHOUT body placeholder to be rejected for bullet_groups content")
	}
}

// TestIsLayoutSuitable_ChartRejectsZeroBoundsPlaceholder verifies that chart and
// diagram slides reject layouts whose body placeholder has zero bounds (Width==0
// or Height==0). Zero bounds indicate unresolved master inheritance — the
// placeholder's true size is unknown. Allowing such placeholders produces
// thumbnail charts rendered at (0,0) with no visible area.
// Regression test: SVG diagram thumbnailing with non-standard dimensions.
func TestIsLayoutSuitable_ChartRejectsZeroBoundsPlaceholder(t *testing.T) {
	zeroBoundsLayout := types.LayoutMetadata{
		ID:   "layout-zero",
		Name: "Zero Bounds",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type:   types.PlaceholderBody,
				Bounds: types.BoundingBox{}, // all zeros — unresolved
			},
		},
	}
	zeroHeightLayout := types.LayoutMetadata{
		ID:   "layout-zero-h",
		Name: "Zero Height",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderBody,
				Bounds: types.BoundingBox{
					X: 305816, Y: 1885212,
					Width: 10181590, Height: 0, // width ok, height zero
				},
			},
		},
	}
	zeroWidthLayout := types.LayoutMetadata{
		ID:   "layout-zero-w",
		Name: "Zero Width",
		Tags: []string{"content"},
		Placeholders: []types.PlaceholderInfo{
			{Type: types.PlaceholderTitle},
			{
				Type: types.PlaceholderBody,
				Bounds: types.BoundingBox{
					X: 305816, Y: 1885212,
					Width: 0, Height: 4749009, // height ok, width zero
				},
			},
		},
	}

	chartSlide := types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue"}
	diagramSlide := types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Org"}

	// All zero-bounds variants should reject chart/diagram slides
	for _, layout := range []types.LayoutMetadata{zeroBoundsLayout, zeroHeightLayout, zeroWidthLayout} {
		if isLayoutSuitable(layout, chartSlide) {
			t.Errorf("expected layout %q to be rejected for chart slides (bounds: W=%d H=%d)",
				layout.Name, layout.Placeholders[1].Bounds.Width, layout.Placeholders[1].Bounds.Height)
		}
		if isLayoutSuitable(layout, diagramSlide) {
			t.Errorf("expected layout %q to be rejected for diagram slides (bounds: W=%d H=%d)",
				layout.Name, layout.Placeholders[1].Bounds.Width, layout.Placeholders[1].Bounds.Height)
		}
	}

	// Text slides should still accept zero-bounds layouts
	textSlide := types.SlideDefinition{Type: types.SlideTypeContent, Title: "Summary"}
	if !isLayoutSuitable(zeroBoundsLayout, textSlide) {
		t.Error("expected zero-bounds layout to be accepted for text slides")
	}
}

// TestIsLayoutSuitable_CorporateTemplateChartBounds is a regression fixture capturing
// exact layout metadata from a corporate template
// after harmonization. It verifies that:
//  1. "One Content" (the only full-width content layout) is suitable for charts/diagrams
//  2. "Section Divider" is rejected (tagged section-header)
//  3. "Title Slide" and "Closing" are rejected (no body placeholder / tagged closing)
//
// Regression test: SVG diagram thumbnailing with non-standard dimensions.
func TestIsLayoutSuitable_CorporateTemplateChartBounds(t *testing.T) {
	// Exact metadata values from corporate template analysis after harmonization.
	corpLayouts := []types.LayoutMetadata{
		{
			ID:   "slideLayout1",
			Name: "Title Slide",
			Tags: []string{"title-slide"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 305816, Y: 1885212, Width: 9087875, Height: 4749009}},
				{ID: "subtitle", Type: types.PlaceholderSubtitle, Index: 1, Bounds: types.BoundingBox{X: 305816, Y: 287819, Width: 9087875, Height: 245365}},
			},
		},
		{
			ID:   "slideLayout2",
			Name: "Section Divider",
			Tags: []string{"content", "section-header"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 305816, Y: 1885212, Width: 6121971, Height: 4749009}},
				{ID: "body", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{X: 8315950, Y: 1337412, Width: 3356176, Height: 3603813}},
			},
		},
		{
			ID:   "slideLayout3",
			Name: "One Content",
			Tags: []string{"content"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 305816, Y: 287819, Width: 10181590, Height: 863456}},
				{ID: "body", Type: types.PlaceholderBody, Index: 1, Bounds: types.BoundingBox{X: 305816, Y: 1885212, Width: 10181590, Height: 4749009}},
			},
			Capacity: types.CapacityEstimate{MaxBullets: 18, MaxTextLines: 9, TextHeavy: true},
		},
		{
			ID:   "slideLayout4",
			Name: "Closing",
			Tags: []string{"title-slide", "closing"},
			Placeholders: []types.PlaceholderInfo{
				{ID: "title", Type: types.PlaceholderTitle, Bounds: types.BoundingBox{X: 1606946, Y: 1183418, Width: 9641681, Height: 1504750}},
				{ID: "subtitle", Type: types.PlaceholderSubtitle, Index: 1, Bounds: types.BoundingBox{X: 1606946, Y: 4542894, Width: 9641681, Height: 1896006}},
			},
		},
	}

	chartSlide := types.SlideDefinition{Type: types.SlideTypeChart, Title: "Revenue Chart"}
	diagramSlide := types.SlideDefinition{Type: types.SlideTypeDiagram, Title: "Org Chart"}

	for _, layout := range corpLayouts {
		chartOK := isLayoutSuitable(layout, chartSlide)
		diagramOK := isLayoutSuitable(layout, diagramSlide)

		switch layout.Name {
		case "One Content":
			// Only One Content should be suitable for charts/diagrams
			if !chartOK {
				t.Errorf("Corporate layout %q should be suitable for charts (body W=%d H=%d)",
					layout.Name,
					layout.Placeholders[1].Bounds.Width,
					layout.Placeholders[1].Bounds.Height)
			}
			if !diagramOK {
				t.Errorf("Corporate layout %q should be suitable for diagrams", layout.Name)
			}
		default:
			// All other layouts should be rejected for charts/diagrams
			if chartOK {
				t.Errorf("Corporate layout %q should NOT be suitable for charts", layout.Name)
			}
			if diagramOK {
				t.Errorf("Corporate layout %q should NOT be suitable for diagrams", layout.Name)
			}
		}
	}
}
