package generator

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestIsPyramidDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "pyramid type",
			spec:     &types.DiagramSpec{Type: "pyramid"},
			expected: true,
		},
		{
			name:     "heatmap type",
			spec:     &types.DiagramSpec{Type: "heatmap"},
			expected: false,
		},
		{
			name:     "swot type",
			spec:     &types.DiagramSpec{Type: "swot"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPyramidDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isPyramidDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParsePyramidDiagramData(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		wantErr    bool
		wantLevels int
	}{
		{
			name: "basic levels with labels and descriptions",
			data: map[string]any{
				"levels": []any{
					map[string]any{"label": "Apex", "description": "Top level"},
					map[string]any{"label": "Middle"},
					map[string]any{"label": "Base", "description": "Foundation"},
				},
			},
			wantLevels: 3,
		},
		{
			name: "string-only levels",
			data: map[string]any{
				"levels": []any{"Top", "Middle", "Bottom"},
			},
			wantLevels: 3,
		},
		{
			name: "single level",
			data: map[string]any{
				"levels": []any{map[string]any{"label": "Only"}},
			},
			wantLevels: 1,
		},
		{
			name:    "missing levels key",
			data:    map[string]any{"title": "Test"},
			wantErr: true,
		},
		{
			name:    "empty levels",
			data:    map[string]any{"levels": []any{}},
			wantErr: true,
		},
		{
			name: "too many levels",
			data: map[string]any{
				"levels": func() []any {
					levels := make([]any, 21)
					for i := range levels {
						levels[i] = "Level"
					}
					return levels
				}(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels, err := parsePyramidDiagramData(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePyramidDiagramData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(levels) != tt.wantLevels {
				t.Errorf("parsePyramidDiagramData() got %d levels, want %d", len(levels), tt.wantLevels)
			}
		})
	}
}

func TestPyramidTrapezoidAdj(t *testing.T) {
	tests := []struct {
		name       string
		levelIndex int
		numLevels  int
		widthRatio float64
		wantMin    int64
		wantMax    int64
	}{
		{
			name:       "single level is rectangle",
			levelIndex: 0,
			numLevels:  1,
			widthRatio: 1.0,
			wantMin:    0,
			wantMax:    0,
		},
		{
			name:       "bottom level of multi-level",
			levelIndex: 4,
			numLevels:  5,
			widthRatio: 1.0,
			wantMin:    0,     // Has some taper (top aligns with level above)
			wantMax:    15000, // But not extreme
		},
		{
			name:       "top level has large adj",
			levelIndex: 0,
			numLevels:  5,
			widthRatio: 0.15,
			wantMin:    10000, // Should be significantly tapered
			wantMax:    50000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pyramidTrapezoidAdj(tt.levelIndex, tt.numLevels, tt.widthRatio)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("pyramidTrapezoidAdj(%d, %d, %f) = %d, want in range [%d, %d]",
					tt.levelIndex, tt.numLevels, tt.widthRatio, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestPyramidLevelFill(t *testing.T) {
	// Just verify the functions don't panic and return valid fills.
	_ = pyramidLevelFill(0, 5) // apex
	_ = pyramidLevelFill(2, 5) // middle
	_ = pyramidLevelFill(4, 5) // base
	_ = pyramidLevelFill(0, 1) // single level
}

func TestPyramidLevelTextColor(t *testing.T) {
	// Just verify the functions don't panic.
	_ = pyramidLevelTextColor(0, 5) // apex — should be light text
	_ = pyramidLevelTextColor(4, 5) // base — should be dark text
	_ = pyramidLevelTextColor(0, 1) // single level
}

func TestGeneratePyramidGroupXML(t *testing.T) {
	bounds := types.BoundingBox{
		X: 500000, Y: 1000000,
		Width: 8000000, Height: 5000000,
	}

	panels := []nativePanelData{
		{title: "Self-Actualization", body: "Creativity"},
		{title: "Esteem", body: "Achievement"},
		{title: "Love/Belonging"},
		{title: "Safety"},
		{title: "Physiological", body: "Food, water"},
	}

	xml := generatePyramidGroupXML(panels, bounds, 10000, "Arial")
	if xml == "" {
		t.Fatal("generatePyramidGroupXML returned empty string")
	}

	// Should contain group wrapper.
	if !strings.Contains(xml, "p:grpSp") {
		t.Error("output should contain p:grpSp group wrapper")
	}

	// Should contain all level labels.
	for _, p := range panels {
		if !strings.Contains(xml, p.title) {
			t.Errorf("output should contain level label %q", p.title)
		}
	}

	// Should contain descriptions where provided.
	if !strings.Contains(xml, "Creativity") {
		t.Error("output should contain description text")
	}

	// Should contain trapezoid geometry.
	if !strings.Contains(xml, "trapezoid") {
		t.Error("output should contain trapezoid geometry")
	}

	// Should be valid XML.
	if err := isValidXMLFragment(xml); err != nil {
		t.Errorf("output is not valid XML: %v", err)
	}
}

func TestGeneratePyramidGroupXML_SingleLevel(t *testing.T) {
	bounds := types.BoundingBox{
		X: 500000, Y: 1000000,
		Width: 8000000, Height: 5000000,
	}

	panels := []nativePanelData{
		{title: "Only Level"},
	}

	xml := generatePyramidGroupXML(panels, bounds, 10000, "Arial")
	if xml == "" {
		t.Fatal("generatePyramidGroupXML returned empty string for single level")
	}

	if !strings.Contains(xml, "Only Level") {
		t.Error("output should contain the single level label")
	}
}

func TestGeneratePyramidGroupXML_ManyLevels(t *testing.T) {
	bounds := types.BoundingBox{
		X: 500000, Y: 1000000,
		Width: 8000000, Height: 5000000,
	}

	var panels []nativePanelData
	for i := 0; i < 10; i++ {
		panels = append(panels, nativePanelData{
			title: strings.Repeat("L", i+1),
		})
	}

	xml := generatePyramidGroupXML(panels, bounds, 10000, "Arial")
	if xml == "" {
		t.Fatal("generatePyramidGroupXML returned empty string for 10 levels")
	}

	// Should contain all 10 level shapes.
	count := strings.Count(xml, "Pyramid Level")
	if count != 10 {
		t.Errorf("expected 10 Pyramid Level shapes, got %d", count)
	}
}

func TestGeneratePyramidGroupXML_Empty(t *testing.T) {
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}
	result := generatePyramidGroupXML(nil, bounds, 10000, "Arial")
	if result != "" {
		t.Error("expected empty string for nil panels")
	}
}

func TestPyramidAllocatesHeightToNarrowTextHeavyApex(t *testing.T) {
	bounds := types.BoundingBox{X: 500000, Y: 1000000, Width: 8000000, Height: 5000000}
	panels := []nativePanelData{
		{title: "Vision", body: "Be the trusted platform for regulated industries"},
		{title: "Strategy", body: "Expand deliberately"},
		{title: "Capabilities", body: "Build core services"},
		{title: "Initiatives", body: "Quarterly targets"},
	}
	heights, gap := pyramidLevelHeights(panels, bounds, pyramidLabelFontSize, pyramidDescFontSize, "Arial")
	if heights[0] <= heights[len(heights)-1] {
		t.Fatalf("text-heavy narrow apex = %d EMU, base = %d EMU; apex should receive more height", heights[0], heights[len(heights)-1])
	}
	var used int64
	for _, height := range heights {
		if height <= 0 {
			t.Fatalf("non-positive tier height in %v", heights)
		}
		used += height
	}
	if used+int64(len(panels)-1)*gap != bounds.Height {
		t.Errorf("tier heights and gaps consume %d EMU, want placeholder height %d", used+int64(len(panels)-1)*gap, bounds.Height)
	}

	// Assert the public OOXML geometry uses the allocation, not merely the
	// helper's return value. This catches a regression to equal-height shapes.
	group := generatePyramidGroupXML(panels, bounds, 10000, "Arial")
	var output struct {
		Shapes []struct {
			ShapeProperties struct {
				Transform struct {
					Offset struct {
						Y int64 `xml:"y,attr"`
					} `xml:"off"`
					Extent struct {
						Height int64 `xml:"cy,attr"`
					} `xml:"ext"`
				} `xml:"xfrm"`
			} `xml:"spPr"`
		} `xml:"sp"`
	}
	if err := xml.Unmarshal([]byte(group), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Shapes) != len(heights) {
		t.Fatalf("OOXML contains %d tiers, want %d", len(output.Shapes), len(heights))
	}
	wantY := bounds.Y
	for i, shape := range output.Shapes {
		if shape.ShapeProperties.Transform.Offset.Y != wantY {
			t.Errorf("OOXML tier %d begins at %d, want %d", i, shape.ShapeProperties.Transform.Offset.Y, wantY)
		}
		if shape.ShapeProperties.Transform.Extent.Height != heights[i] {
			t.Errorf("OOXML tier %d height = %d, allocated %d", i, shape.ShapeProperties.Transform.Extent.Height, heights[i])
		}
		wantY += heights[i] + gap
	}
	if wantY-gap != bounds.Y+bounds.Height {
		t.Errorf("OOXML pyramid ends at %d, placeholder ends at %d", wantY-gap, bounds.Y+bounds.Height)
	}
}

func TestPyramidTierHeightsStayWithinShortPlaceholder(t *testing.T) {
	bounds := types.BoundingBox{Width: 8000000, Height: 300000}
	panels := []nativePanelData{{title: "A"}, {title: "B"}, {title: "C"}, {title: "D"}}
	heights, gap := pyramidLevelHeights(panels, bounds, pyramidLabelFontSize, pyramidDescFontSize, "Arial")
	if gap != 0 {
		t.Errorf("short placeholder gap = %d, want 0", gap)
	}
	var used int64
	for _, height := range heights {
		if height < 0 {
			t.Fatalf("negative tier height in %v", heights)
		}
		used += height
	}
	if used != bounds.Height {
		t.Errorf("short placeholder uses %d EMU, want %d", used, bounds.Height)
	}
}

func TestPyramidLongApexDoesNotCollapseBase(t *testing.T) {
	bounds := types.BoundingBox{Width: 8000000, Height: 2500000}
	panels := []nativePanelData{
		{title: "Vision", body: strings.Repeat("regulated industries and trusted platforms ", 80)},
		{title: "Middle"},
		{title: "Base"},
	}
	heights, gap := pyramidLevelHeights(panels, bounds, pyramidLabelFontSize, pyramidDescFontSize, "Arial")
	minimum := 2*pyramidTextInset + int64(11*1.2*float64(types.EMUPerPoint))
	if heights[1] < minimum || heights[2] < minimum {
		t.Errorf("long apex starved other tiers below one readable line: %v", heights)
	}
	var used int64
	for _, height := range heights {
		used += height
	}
	if used+int64(len(panels)-1)*gap != bounds.Height {
		t.Errorf("tier allocation uses %d EMU, want %d", used+int64(len(panels)-1)*gap, bounds.Height)
	}
}

func TestPyramidEstimateShapeCount(t *testing.T) {
	panels := make([]nativePanelData, 5)
	got := pyramidEstimateShapeCount(panels)
	if got != 6 { // 1 group + 5 levels
		t.Errorf("pyramidEstimateShapeCount() = %d, want 6", got)
	}
}

// isValidXMLFragment checks if a string is valid XML by wrapping it in a root element.
func isValidXMLFragment(s string) error {
	wrapped := "<root xmlns:p='http://schemas.openxmlformats.org/presentationml/2006/main' xmlns:a='http://schemas.openxmlformats.org/drawingml/2006/main' xmlns:r='http://schemas.openxmlformats.org/officeDocument/2006/relationships'>" + s + "</root>"
	return xml.Unmarshal([]byte(wrapped), new(any))
}
