package generator

import (
	"strings"
	"testing"
)

func TestGenerateFooterShapes(t *testing.T) {
	positions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
		"type:ftr": {
			Offset: offsetXML{X: 4038600, Y: 6492875},
			Extent: extentXML{CX: 4114800, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}

	result := generateFooterShapes(positions, "Acme Corp")

	// Verify left and right footer shapes are present (no center)
	if !strings.Contains(result, "Footer Left") {
		t.Error("expected left footer shape")
	}
	if strings.Contains(result, "Footer Center") {
		t.Error("center footer should not be generated")
	}
	if !strings.Contains(result, "Footer Right") {
		t.Error("expected right footer shape")
	}

	// Verify content
	if !strings.Contains(result, "Acme Corp") {
		t.Error("expected left text 'Acme Corp'")
	}

	// Verify slide number uses field element
	if !strings.Contains(result, `type="slidenum"`) {
		t.Error("expected slidenum field type")
	}

	// Verify scheme color is used (tx1)
	if !strings.Contains(result, `schemeClr`) {
		t.Error("expected scheme color reference")
	}
	if strings.Contains(result, "888888") {
		t.Error("should not use hardcoded #888888 color")
	}

	// Verify alignment attributes
	if !strings.Contains(result, `algn="l"`) {
		t.Error("expected left alignment for left footer")
	}
	if !strings.Contains(result, `algn="r"`) {
		t.Error("expected right alignment for right footer")
	}

	// Verify shape IDs
	if !strings.Contains(result, `id="990"`) {
		t.Error("expected shape ID 990 for left footer")
	}
	if !strings.Contains(result, `id="992"`) {
		t.Error("expected shape ID 992 for right footer")
	}
}

func TestGenerateFooterShapes_EmptyLeftAndCenter(t *testing.T) {
	positions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
		"type:ftr": {
			Offset: offsetXML{X: 4038600, Y: 6492875},
			Extent: extentXML{CX: 4114800, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}

	// No left text — only slide number should appear
	result := generateFooterShapes(positions, "")

	if strings.Contains(result, "Footer Left") {
		t.Error("left footer should not appear when text is empty")
	}
	if strings.Contains(result, "Footer Center") {
		t.Error("center footer should not be generated")
	}
	if !strings.Contains(result, "Footer Right") {
		t.Error("right footer (slide number) should always appear")
	}
	if !strings.Contains(result, `type="slidenum"`) {
		t.Error("expected slidenum field element")
	}
}

func TestInsertFooters(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?><p:sld><p:cSld><p:spTree><p:sp>content</p:sp></p:spTree></p:cSld></p:sld>`)

	config := &FooterConfig{
		Enabled:  true,
		LeftText: "Test Corp",
	}

	positions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}

	result, err := insertFooters(slideXML, config, positions)
	if err != nil {
		t.Fatalf("insertFooters failed: %v", err)
	}

	resultStr := string(result)

	// Verify shapes are inserted before </p:spTree>
	footerIdx := strings.Index(resultStr, "Footer Left")
	closingIdx := strings.Index(resultStr, "</p:spTree>")
	if footerIdx == -1 || closingIdx == -1 || footerIdx > closingIdx {
		t.Error("footer shapes should be inserted before </p:spTree>")
	}

	// Verify content
	if !strings.Contains(resultStr, "Test Corp") {
		t.Error("expected left footer text")
	}
	if !strings.Contains(resultStr, `type="slidenum"`) {
		t.Error("expected slidenum field element")
	}
}

func TestInsertFooters_DisabledConfig(t *testing.T) {
	slideXML := []byte(`<p:sld><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`)

	// Disabled config
	config := &FooterConfig{Enabled: false}
	result, err := insertFooters(slideXML, config, computeDefaultFooterPositions(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(slideXML) {
		t.Error("disabled footer config should not modify slide XML")
	}

	// Nil config
	result, err = insertFooters(slideXML, nil, computeDefaultFooterPositions(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(slideXML) {
		t.Error("nil footer config should not modify slide XML")
	}
}

func TestResolveFooterPositions_FromMaster(t *testing.T) {
	masterPositions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 100, Y: 200},
			Extent: extentXML{CX: 300, CY: 400},
		},
		"type:ftr": {
			Offset: offsetXML{X: 500, Y: 600},
			Extent: extentXML{CX: 700, CY: 800},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 900, Y: 1000},
			Extent: extentXML{CX: 500000, CY: 1200}, // above minSldNumWidth, so no clamping
		},
		"type:title": {
			Offset: offsetXML{X: 0, Y: 0},
			Extent: extentXML{CX: 5000, CY: 2000},
		},
	}

	positions := resolveFooterPositions(masterPositions, 0)

	// Should only have footer keys
	if len(positions) != 3 {
		t.Fatalf("expected 3 footer positions, got %d", len(positions))
	}

	// Verify it uses master values, not defaults
	if positions["type:dt"].Offset.X != 100 {
		t.Errorf("expected master dt offset X=100, got %d", positions["type:dt"].Offset.X)
	}
	if positions["type:ftr"].Offset.X != 500 {
		t.Errorf("expected master ftr offset X=500, got %d", positions["type:ftr"].Offset.X)
	}
	if positions["type:sldNum"].Offset.X != 900 {
		t.Errorf("expected master sldNum offset X=900, got %d", positions["type:sldNum"].Offset.X)
	}
}

func TestResolveFooterPositions_MinSldNumWidth(t *testing.T) {
	// Narrow sldNum placeholder (like PwC template: cx=298209) should be widened
	// to minSldNumWidth, expanding leftward to keep the right edge fixed.
	narrowCX := int64(298209)
	sldNumX := int64(11503818)
	masterPositions := map[string]*transformXML{
		"type:sldNum": {
			Offset: offsetXML{X: sldNumX, Y: 6501384},
			Extent: extentXML{CX: narrowCX, CY: 161583},
		},
	}

	positions := resolveFooterPositions(masterPositions, defaultSlideHeightEMU)

	pos := positions["type:sldNum"]
	if pos.Extent.CX != minSldNumWidth {
		t.Errorf("expected sldNum CX=%d, got %d", minSldNumWidth, pos.Extent.CX)
	}
	// Right edge should be preserved
	expectedRightEdge := sldNumX + narrowCX
	actualRightEdge := pos.Offset.X + pos.Extent.CX
	if actualRightEdge != expectedRightEdge {
		t.Errorf("right edge shifted: expected %d, got %d", expectedRightEdge, actualRightEdge)
	}
}

func TestResolveFooterPositions_DefaultFallback(t *testing.T) {
	// No footer keys in master — should fall back to defaults
	masterPositions := map[string]*transformXML{
		"type:title": {
			Offset: offsetXML{X: 0, Y: 0},
			Extent: extentXML{CX: 5000, CY: 2000},
		},
	}

	positions := resolveFooterPositions(masterPositions, 0)

	if len(positions) != 3 {
		t.Fatalf("expected 3 default footer positions, got %d", len(positions))
	}

	// Verify defaults are used
	if positions["type:dt"].Offset.X != computeDefaultFooterPositions(0)["type:dt"].Offset.X {
		t.Error("expected default dt position")
	}
}

func TestResolveFooterPositions_NilMap(t *testing.T) {
	positions := resolveFooterPositions(nil, 0)

	if len(positions) != 3 {
		t.Fatalf("expected 3 default footer positions, got %d", len(positions))
	}
}

func TestResolveFooterPositions_ClampsOffScreenY(t *testing.T) {
	// Simulate template where dt placeholder is below the slide boundary:
	// y=7056786 EMU (7.717") + cy=365125 EMU = 7421911 EMU (bottom edge)
	// This exceeds the 6858000 EMU (7.50") slide height.
	// After normalization, all positions get the max Y (7056786), then clamping
	// brings them all down to slideHeight - CY.
	masterPositions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 7056786}, // 7.717" — below slide edge
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
		"type:ftr": {
			Offset: offsetXML{X: 4038600, Y: 6492875}, // Normal — within bounds
			Extent: extentXML{CX: 4114800, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6492875}, // Normal — within bounds
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}

	positions := resolveFooterPositions(masterPositions, 0)

	// After normalization all get max Y (7056786), then clamping adjusts to slideHeight - CY
	expectedY := defaultSlideHeightEMU - positions["type:dt"].Extent.CY
	for _, key := range []string{"type:dt", "type:ftr", "type:sldNum"} {
		if positions[key].Offset.Y != expectedY {
			t.Errorf("%s Y = %d, want %d (normalized + clamped)", key, positions[key].Offset.Y, expectedY)
		}
	}
}

func TestNormalizeFooterVerticalPositions(t *testing.T) {
	// All positions are on-screen (Y+CY <= defaultSlideHeightEMU)
	positions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 6400000},
			Extent: extentXML{CX: 3200400, CY: 300000},
		},
		"type:ftr": {
			Offset: offsetXML{X: 4038600, Y: 6492875},
			Extent: extentXML{CX: 4114800, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6450000},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}

	normalizeFooterVerticalPositions(positions, 0)

	// All should share the max visible Y (6492875) and max CY (365125)
	for _, key := range []string{"type:dt", "type:ftr", "type:sldNum"} {
		if positions[key].Offset.Y != 6492875 {
			t.Errorf("%s Y = %d, want 6492875", key, positions[key].Offset.Y)
		}
		if positions[key].Extent.CY != 365125 {
			t.Errorf("%s CY = %d, want 365125", key, positions[key].Extent.CY)
		}
	}

	// X and CX should be unchanged
	if positions["type:dt"].Offset.X != 457200 {
		t.Errorf("dt X changed unexpectedly: %d", positions["type:dt"].Offset.X)
	}
	if positions["type:ftr"].Extent.CX != 4114800 {
		t.Errorf("ftr CX changed unexpectedly: %d", positions["type:ftr"].Extent.CX)
	}
}

func TestNormalizeFooterVerticalPositions_OffScreenDt(t *testing.T) {
	// dt placeholder is off-screen (Y+CY > defaultSlideHeightEMU=6858000),
	// simulating a hidden date field. It should NOT drag visible footers down.
	positions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 7200000}, // 7200000 + 300000 = 7500000 > 6858000
			Extent: extentXML{CX: 3200400, CY: 300000},
		},
		"type:ftr": {
			Offset: offsetXML{X: 4038600, Y: 6492875}, // 6492875 + 365125 = 6858000, on-screen
			Extent: extentXML{CX: 4114800, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6400000}, // 6400000 + 365125 = 6765125, on-screen
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}

	normalizeFooterVerticalPositions(positions, 0)

	// visibleY should be 6492875 (ftr), not 7200000 (off-screen dt)
	wantY := int64(6492875)
	wantCY := int64(365125)
	for _, key := range []string{"type:dt", "type:ftr", "type:sldNum"} {
		if positions[key].Offset.Y != wantY {
			t.Errorf("%s Y = %d, want %d", key, positions[key].Offset.Y, wantY)
		}
		if positions[key].Extent.CY != wantCY {
			t.Errorf("%s CY = %d, want %d", key, positions[key].Extent.CY, wantCY)
		}
	}
}

func TestNormalizeFooterVerticalPositions_Empty(t *testing.T) {
	// Should not panic on empty map
	normalizeFooterVerticalPositions(map[string]*transformXML{}, 0)
	normalizeFooterVerticalPositions(nil, 0)
}

func TestClampFooterPosition(t *testing.T) {
	tests := []struct {
		name         string
		y, cy        int64
		slideHeight  int64
		expectedY    int64
	}{
		{"within bounds", 6000000, 365125, 6858000, 6000000},
		{"exactly at edge", 6492875, 365125, 6858000, 6492875},
		{"below boundary", 7056786, 365125, 6858000, 6492875},
		{"way below", 9000000, 365125, 6858000, 6492875},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := &transformXML{
				Offset: offsetXML{Y: tt.y},
				Extent: extentXML{CY: tt.cy},
			}
			clampFooterPosition(pos, tt.slideHeight)
			if pos.Offset.Y != tt.expectedY {
				t.Errorf("Y = %d, want %d", pos.Offset.Y, tt.expectedY)
			}
		})
	}
}

func TestExtractSlideTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  []ContentItem
		expected string
	}{
		{
			name: "title found",
			content: []ContentItem{
				{PlaceholderID: "Title 1", Type: ContentText, Value: "My Slide Title"},
				{PlaceholderID: "Content Placeholder 2", Type: ContentBullets, Value: []string{"a", "b"}},
			},
			expected: "My Slide Title",
		},
		{
			name: "no title placeholder",
			content: []ContentItem{
				{PlaceholderID: "Content Placeholder 2", Type: ContentText, Value: "Not a title"},
			},
			expected: "",
		},
		{
			name:     "empty content",
			content:  nil,
			expected: "",
		},
		{
			name: "heading placeholder",
			content: []ContentItem{
				{PlaceholderID: "Heading 1", Type: ContentText, Value: "Heading Text"},
			},
			expected: "Heading Text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSlideTitle(tt.content)
			if got != tt.expected {
				t.Errorf("extractSlideTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRemoveFooterPlaceholders(t *testing.T) {
	idx10 := 10
	idx11 := 11
	idx12 := 12
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "title"}},
					}},
					{NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Content Placeholder 2"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
					}},
					{NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Date Placeholder"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "dt", Index: &idx10}},
					}},
					{NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Footer Placeholder"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "ftr", Index: &idx11}},
					}},
					{NonVisualProperties: nonVisualPropertiesXML{
						ConnectionNonVisual: connectionNonVisualXML{Name: "Slide Number Placeholder"},
						NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "sldNum", Index: &idx12}},
					}},
				},
			},
		},
	}

	result := removeFooterPlaceholders(slide)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 2 {
		t.Fatalf("expected 2 shapes after removing footer placeholders, got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}

	// Verify remaining shapes are title and body
	names := []string{
		result.CommonSlideData.ShapeTree.Shapes[0].NonVisualProperties.ConnectionNonVisual.Name,
		result.CommonSlideData.ShapeTree.Shapes[1].NonVisualProperties.ConnectionNonVisual.Name,
	}
	if names[0] != "Title 1" || names[1] != "Content Placeholder 2" { //nolint:gosec // test validates length above
		t.Errorf("expected [Title 1, Content Placeholder 2], got %v", names)
	}
}

func TestRemoveDuplicateFooterBrandMark(t *testing.T) {
	// Simulate template with a layout-inherited brand-mark text box in the footer zone
	// alongside normal content shapes. When LeftText matches the brand mark,
	// the shape should be removed to prevent duplication.
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					// Normal title placeholder — should be kept
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "title"}},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 0, Y: 0},
								Extent: extentXML{CX: 5000000, CY: 1000000},
							},
						},
					},
					// Brand mark text box in footer zone — should be removed
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "TextBox 8"},
							NvPr:                nvPrXML{}, // No placeholder
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 394728, Y: 6501384}, // In footer zone
								Extent: extentXML{CX: 431483, CY: 161583},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Acme Corp"}}},
							},
						},
					},
					// Normal body placeholder — should be kept
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Content 2"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 0, Y: 1200000},
								Extent: extentXML{CX: 5000000, CY: 4000000},
							},
						},
					},
				},
			},
		},
	}

	result := removeDuplicateFooterBrandMark(slide, "Acme Corp", 0)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 2 {
		t.Fatalf("expected 2 shapes after removing brand mark, got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}

	// Verify the brand mark text box was removed and others remain
	names := []string{
		result.CommonSlideData.ShapeTree.Shapes[0].NonVisualProperties.ConnectionNonVisual.Name,
		result.CommonSlideData.ShapeTree.Shapes[1].NonVisualProperties.ConnectionNonVisual.Name,
	}
	if names[0] != "Title 1" || names[1] != "Content 2" { //nolint:gosec // test validates length above
		t.Errorf("expected [Title 1, Content 2], got %v", names)
	}
}

func TestRemoveDuplicateFooterBrandMark_KeepsNonMatchingText(t *testing.T) {
	// A text box in the footer zone with different text should NOT be removed
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "TextBox 1"},
							NvPr:                nvPrXML{},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 0, Y: 6500000},
								Extent: extentXML{CX: 1000000, CY: 200000},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Confidential"}}},
							},
						},
					},
				},
			},
		},
	}

	result := removeDuplicateFooterBrandMark(slide, "Acme Corp", 0)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 1 {
		t.Fatalf("expected 1 shape (non-matching text should be kept), got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}
}

func TestRemoveDuplicateFooterBrandMark_KeepsShapeOutsideFooterZone(t *testing.T) {
	// A brand-mark text box NOT in the footer zone should be kept
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "TextBox 1"},
							NvPr:                nvPrXML{},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 0, Y: 1000000}, // Near top of slide
								Extent: extentXML{CX: 1000000, CY: 200000},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Acme Corp"}}},
							},
						},
					},
				},
			},
		},
	}

	result := removeDuplicateFooterBrandMark(slide, "Acme Corp", 0)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 1 {
		t.Fatalf("expected 1 shape (outside footer zone should be kept), got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}
}

func TestRemoveDuplicateFooterBrandMark_EmptyLeftText(t *testing.T) {
	// When LeftText is empty, no shapes should be removed
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "TextBox 1"},
							NvPr:                nvPrXML{},
						},
						ShapeProperties: shapePropertiesXML{
							Transform: &transformXML{
								Offset: offsetXML{X: 0, Y: 6500000},
								Extent: extentXML{CX: 1000000, CY: 200000},
							},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{
								{Runs: []runXML{{Text: "Acme Corp"}}},
							},
						},
					},
				},
			},
		},
	}

	result := removeDuplicateFooterBrandMark(slide, "", 0)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 1 {
		t.Fatalf("expected 1 shape (empty LeftText means no removal), got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}
}

func TestRemoveEmptyPlaceholders(t *testing.T) {
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					// Populated title — should be kept
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Title 1"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "ctrTitle"}},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{{Runs: []runXML{{Text: "Thank You"}}}},
						},
					},
					// Empty subtitle — should be removed (this is the closing slide bug)
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Subtitle 2"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "subTitle"}},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{{}}, // empty <a:p/>
						},
					},
					// Non-placeholder shape (no ph element) — should be kept
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Logo"},
						},
					},
					// Empty footer placeholder — should be kept (footer types excluded)
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Footer"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "ftr"}},
						},
					},
				},
			},
		},
	}

	result := removeEmptyPlaceholders(slide)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 3 {
		t.Fatalf("expected 3 shapes (title, logo, footer), got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}

	names := make([]string, len(result.CommonSlideData.ShapeTree.Shapes))
	for i, s := range result.CommonSlideData.ShapeTree.Shapes {
		names[i] = s.NonVisualProperties.ConnectionNonVisual.Name
	}
	expected := []string{"Title 1", "Logo", "Footer"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("shape %d: expected %q, got %q", i, expected[i], name)
		}
	}
}

func TestRemoveEmptyPlaceholders_RunsWithEmptyText(t *testing.T) {
	// Regression: placeholders with runs containing only whitespace/empty text
	// should be treated as empty and removed.
	slide := &slideXML{
		CommonSlideData: commonSlideDataXML{
			ShapeTree: shapeTreeXML{
				Shapes: []shapeXML{
					// Populated body — should be kept
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Body 1"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{{Runs: []runXML{{Text: "Real content"}}}},
						},
					},
					// Placeholder with empty-string run — should be removed
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Subtitle 2"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "subTitle"}},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{{Runs: []runXML{{Text: ""}}}},
						},
					},
					// Placeholder with whitespace-only run — should be removed
					{
						NonVisualProperties: nonVisualPropertiesXML{
							ConnectionNonVisual: connectionNonVisualXML{Name: "Body 3"},
							NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
						},
						TextBody: &textBodyXML{
							Paragraphs: []paragraphXML{{Runs: []runXML{{Text: "   "}}}},
						},
					},
				},
			},
		},
	}

	result := removeEmptyPlaceholders(slide)

	if len(result.CommonSlideData.ShapeTree.Shapes) != 1 {
		t.Fatalf("expected 1 shape (Body 1), got %d", len(result.CommonSlideData.ShapeTree.Shapes))
	}
	if result.CommonSlideData.ShapeTree.Shapes[0].NonVisualProperties.ConnectionNonVisual.Name != "Body 1" {
		t.Errorf("expected kept shape to be Body 1, got %q",
			result.CommonSlideData.ShapeTree.Shapes[0].NonVisualProperties.ConnectionNonVisual.Name)
	}
}

func TestGenerateFooterShape_XMLEscaping(t *testing.T) {
	pos := &transformXML{
		Offset: offsetXML{X: 0, Y: 0},
		Extent: extentXML{CX: 1000, CY: 500},
	}

	result := generateFooterShape(990, "Footer Left", pos, "A&B <Corp>", "l")

	if !strings.Contains(result, "A&amp;B &lt;Corp&gt;") {
		t.Error("expected XML-escaped text in footer shape")
	}
}
