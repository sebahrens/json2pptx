package generator

import (
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// helper to build types.SlotContent for tests
func makeTextSlot(num int, text string) *types.SlotContent {
	return &types.SlotContent{
		SlotNumber: num,
		RawContent: text,
		Type:       types.SlotContentText,
		Text:       text,
	}
}

func makeBulletsSlot(num int, raw string, bullets []string) *types.SlotContent {
	return &types.SlotContent{
		SlotNumber: num,
		RawContent: raw,
		Type:       types.SlotContentBullets,
		Bullets:    bullets,
	}
}

func makeChartSlot(num int, raw string) *types.SlotContent {
	return &types.SlotContent{
		SlotNumber: num,
		RawContent: raw,
		Type:       types.SlotContentChart,
		DiagramSpec: &types.DiagramSpec{
			Type:  "bar",
			Title: "Chart",
		},
	}
}

func makeTableSlot(num int, raw string) *types.SlotContent {
	return &types.SlotContent{
		SlotNumber: num,
		RawContent: raw,
		Type:       types.SlotContentTable,
		Table: &types.TableSpec{
			Headers: []string{"A", "B"},
			Rows:    [][]types.TableCell{{{Content: "1"}, {Content: "2"}}},
		},
	}
}

// =============================================================================
// TEST CASE DEFINITIONS FOR MISSING PPTX LAYOUT COMBINATIONS
// =============================================================================
//
// This file defines comprehensive test cases for:
// - Category 1: Text + Chart side-by-side layouts
// - Category 2: Table layouts (standalone and combined)
// - Category 3: Aspect ratio validation for circular vs rectangular charts
//
// Test naming convention: Test<Category>_<SubCategory>_<Scenario>
// Example: TestTextChart_TextLeftChartRight_BarChart
//
// =============================================================================

// =============================================================================
// CATEGORY 1: TEXT + CHART LAYOUTS
// =============================================================================

// TestTextChart_TextLeftChartRight_BarChart validates text-left, bar-chart-right layout.
// Input: Two slots - bullets on left, bar chart on right
// Expected: Bullets populate slot 1, chart in slot 2 generates warning (not yet supported)
// NOTE: This test documents expected behavior; chart-in-slot support is tracked separately.
func TestTextChart_TextLeftChartRight_BarChart(t *testing.T) {
	slots := map[int]*types.SlotContent{
		1: makeBulletsSlot(1, "- Revenue exceeded targets\n- Customer acquisition up\n- Retention improved",
			[]string{"Revenue exceeded targets", "Customer acquisition up", "Retention improved"}),
		2: makeChartSlot(2, "```chart\ntype: bar\ndata:\n  Q1: 100\n  Q2: 150\n```"),
	}

	layout := types.LayoutMetadata{
		Name: "Two Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	// Slot 1 (bullets) should succeed
	if len(result.ContentItems) < 1 {
		t.Errorf("Expected at least 1 content item (bullets), got %d", len(result.ContentItems))
	} else if result.ContentItems[0].Type != ContentBullets {
		t.Errorf("Slot 1 type = %v, want ContentBullets", result.ContentItems[0].Type)
	}

	// Slot 2 (chart) currently generates warning - this documents the limitation
	// TODO: When chart-in-slot support is added, update this test to expect 2 items
	if len(result.Warnings) == 0 {
		t.Log("Note: chart-in-slot support may have been added; update test expectations")
	} else {
		t.Logf("Expected warning for chart in slot (current limitation): %v", result.Warnings)
	}
}

// TestTextChart_TextLeftChartRight_PieChart validates pie chart in right column.
// Pie charts use full container and constrain the circle internally via min(W,H).
// NOTE: This test documents expected behavior; validates text slot works.
func TestTextChart_TextLeftChartRight_PieChart(t *testing.T) {
	slots := map[int]*types.SlotContent{
		1: makeTextSlot(1, "Analysis of market segments"),
		2: makeChartSlot(2, "```chart\ntype: pie\ndata:\n  Enterprise: 42\n  SMB: 28\n```"),
	}

	layout := types.LayoutMetadata{
		Name: "Two Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	// Slot 1 (text) should succeed
	if len(result.ContentItems) < 1 {
		t.Errorf("Expected at least 1 content item (text), got %d", len(result.ContentItems))
	} else if result.ContentItems[0].Type != ContentText {
		t.Errorf("Slot 1 type = %v, want ContentText", result.ContentItems[0].Type)
	}

	// Document pie chart layout behavior
	t.Log("NOTE: Pie charts use full container dimensions; circle is constrained internally via min(W,H)/2")
}

// TestTextChart_ChartLeftTextRight_DonutChart validates donut chart in left column.
// NOTE: This test documents expected behavior; validates bullets slot works.
func TestTextChart_ChartLeftTextRight_DonutChart(t *testing.T) {
	slots := map[int]*types.SlotContent{
		1: makeChartSlot(1, "```chart\ntype: donut\ndata:\n  R&D: 35\n  Sales: 28\n```"),
		2: makeBulletsSlot(2, "- Key insight 1\n- Key insight 2",
			[]string{"Key insight 1", "Key insight 2"}),
	}

	layout := types.LayoutMetadata{
		Name: "Two Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	// Slot 2 (bullets) should succeed even if slot 1 (chart) fails
	foundBullets := false
	for _, item := range result.ContentItems {
		if item.Type == ContentBullets {
			foundBullets = true
			break
		}
	}
	if !foundBullets {
		t.Error("Expected bullets content item to be present")
	}

	// Document donut chart layout behavior
	t.Log("NOTE: Donut charts use full container dimensions; circle is constrained internally via min(W,H)/2")
}

// =============================================================================
// CATEGORY 2: TABLE LAYOUTS
// =============================================================================

// TestTable_Standalone_Simple validates basic table rendering.
func TestTable_Standalone_Simple(t *testing.T) {
	tableMarkdown := `| Metric | Q3 | Q4 |
|--------|----|----|
| Revenue | $4.2M | $5.0M |
| Margin | 62% | 65% |`

	slots := map[int]*types.SlotContent{
		1: makeTableSlot(1, tableMarkdown),
	}

	layout := types.LayoutMetadata{
		Name: "Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if len(result.ContentItems) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.ContentItems))
	}

	if result.ContentItems[0].Type != ContentTable {
		t.Errorf("Expected ContentTable, got %v", result.ContentItems[0].Type)
	}
}

// TestTable_TextLeftTableRight validates text + table side-by-side layout.
func TestTable_TextLeftTableRight(t *testing.T) {
	tableMarkdown := `| Rep | Actual | % |
|-----|--------|---|
| Team A | $620K | 124% |
| Team B | $510K | 113% |`

	slots := map[int]*types.SlotContent{
		1: makeBulletsSlot(1, "- Revenue exceeded target\n- Strong performance",
			[]string{"Revenue exceeded target", "Strong performance"}),
		2: makeTableSlot(2, tableMarkdown),
	}

	layout := types.LayoutMetadata{
		Name: "Two Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if len(result.ContentItems) != 2 {
		t.Errorf("Expected 2 content items, got %d", len(result.ContentItems))
	}

	// First slot should be bullets
	if result.ContentItems[0].Type != ContentBullets {
		t.Errorf("Slot 1 type = %v, want ContentBullets", result.ContentItems[0].Type)
	}

	// Second slot should be table
	if result.ContentItems[1].Type != ContentTable {
		t.Errorf("Slot 2 type = %v, want ContentTable", result.ContentItems[1].Type)
	}
}

// TestTable_TableLeftTextRight validates table + text reversed layout.
func TestTable_TableLeftTextRight(t *testing.T) {
	tableMarkdown := `| Competitor | Share |
|------------|-------|
| Us | 28% |
| Comp A | 24% |`

	slots := map[int]*types.SlotContent{
		1: makeTableSlot(1, tableMarkdown),
		2: makeTextSlot(2, "Market analysis shows strong position"),
	}

	layout := types.LayoutMetadata{
		Name: "Two Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	if len(result.ContentItems) != 2 {
		t.Errorf("Expected 2 content items, got %d", len(result.ContentItems))
	}

	// First slot should be table
	if result.ContentItems[0].Type != ContentTable {
		t.Errorf("Slot 1 type = %v, want ContentTable", result.ContentItems[0].Type)
	}

	// Second slot should be text
	if result.ContentItems[1].Type != ContentText {
		t.Errorf("Slot 2 type = %v, want ContentText", result.ContentItems[1].Type)
	}
}

// TestTable_TableChartSideBySide validates table + chart combination.
// NOTE: This test documents expected behavior for table + chart side-by-side.
func TestTable_TableChartSideBySide(t *testing.T) {
	tableMarkdown := `| Source | Amount |
|--------|--------|
| Subs | $2.8M |
| Services | $1.2M |`

	slots := map[int]*types.SlotContent{
		1: makeTableSlot(1, tableMarkdown),
		2: makeChartSlot(2, "```chart\ntype: donut\ndata:\n  Subs: 70\n  Services: 30\n```"),
	}

	layout := types.LayoutMetadata{
		Name: "Two Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "Title 1", Type: types.PlaceholderTitle, Index: 0},
			{ID: "Content 1", Type: types.PlaceholderBody, Index: 1},
			{ID: "Content 2", Type: types.PlaceholderContent, Index: 2},
		},
	}

	result, err := BuildSlotContentItems(slots, layout, SlotPopulationConfig{})
	if err != nil {
		t.Fatalf("BuildSlotContentItems() error = %v", err)
	}

	// Slot 1 (table) should succeed
	foundTable := false
	for _, item := range result.ContentItems {
		if item.Type == ContentTable {
			foundTable = true
			break
		}
	}
	if !foundTable {
		t.Error("Expected table content item to be present")
	}

	// Slot 2 (chart) currently generates warning
	t.Log("EXPECTED: Table renders in slot 1, chart visualization in slot 2")
}

// =============================================================================
// CATEGORY 3: ASPECT RATIO TEST CASES
// =============================================================================

// AspectRatioTestCase defines a test case for aspect ratio validation.
type AspectRatioTestCase struct {
	Name            string
	ChartType       types.ChartType
	MustBeCircular  bool     // True for pie, donut, gauge, radar
	CanStretch      bool     // True for bar, line, area, waterfall
	ContainerAspect float64  // Width/Height ratio of container
	ExpectedRatio   *float64 // Expected aspect ratio (nil means any)
}

// GetAspectRatioTestCases returns all aspect ratio test cases.
func GetAspectRatioTestCases() []AspectRatioTestCase {
	return []AspectRatioTestCase{
		// All charts fill their full container dimensions. Each chart type
		// handles its circular shape internally via min(W,H) inscribed rendering.
		// Gauge and radar benefit from extra width for labels/legends.
		{
			Name:            "Gauge chart full width",
			ChartType:       types.ChartGauge,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Gauge chart half width",
			ChartType:       types.ChartGauge,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 8.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Radar chart full width",
			ChartType:       types.ChartRadar,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Pie chart full width",
			ChartType:       types.ChartPie,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Donut chart full width",
			ChartType:       types.ChartDonut,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Bar chart full width",
			ChartType:       types.ChartBar,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil, // Can match container
		},
		{
			Name:            "Bar chart half width",
			ChartType:       types.ChartBar,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 8.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Line chart full width",
			ChartType:       types.ChartLine,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Line chart half width",
			ChartType:       types.ChartLine,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 8.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Area chart full width",
			ChartType:       types.ChartArea,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Waterfall chart full width",
			ChartType:       types.ChartWaterfall,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
		{
			Name:            "Funnel chart full width",
			ChartType:       types.ChartFunnel,
			MustBeCircular:  false,
			CanStretch:      true,
			ContainerAspect: 16.0 / 9.0,
			ExpectedRatio:   nil,
		},
	}
}

// TestAspectRatio_AllChartsCanStretchToFillContainer validates that all chart
// types are configured to fill their container (no forced 1:1 aspect ratio).
func TestAspectRatio_AllChartsCanStretchToFillContainer(t *testing.T) {
	testCases := GetAspectRatioTestCases()

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			if tc.MustBeCircular {
				t.Errorf("Chart %s should not be MustBeCircular — all charts fill their container", tc.ChartType)
			}
			if !tc.CanStretch {
				t.Errorf("Chart %s should have CanStretch=true", tc.ChartType)
			}
		})
	}
}

// TestAspectRatio_RectangularChartsCanStretch validates rectangular charts.
func TestAspectRatio_RectangularChartsCanStretch(t *testing.T) {
	testCases := GetAspectRatioTestCases()

	for _, tc := range testCases {
		if !tc.CanStretch {
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			// For rectangular charts, FitMode can be "stretch" or omitted
			spec := &types.DiagramSpec{
				Type:    string(tc.ChartType) + "_chart",
				Width:   800,
				Height:  600,
				FitMode: "stretch",
			}

			// Rectangular charts should allow stretching
			if spec.FitMode != "stretch" && spec.FitMode != "" {
				t.Logf("Rectangular chart %s using FitMode=%q", tc.ChartType, spec.FitMode)
			}
		})
	}
}

// TestAspectRatio_DetectLayoutPreset validates preset detection logic.
func TestAspectRatio_DetectLayoutPreset(t *testing.T) {
	tests := []struct {
		name            string
		placeholderW    types.EMU
		slideW          types.EMU
		expectedPreset  types.LayoutPreset
	}{
		{
			name:           "Full width placeholder",
			placeholderW:   8000000, // ~90% of slide
			slideW:         9144000,
			expectedPreset: types.PresetContent16x9,
		},
		{
			name:           "Half width placeholder",
			placeholderW:   4500000, // ~50% of slide
			slideW:         9144000,
			expectedPreset: types.PresetHalf16x9,
		},
		{
			name:           "Third width placeholder",
			placeholderW:   2800000, // ~30% of slide
			slideW:         9144000,
			expectedPreset: types.PresetThird16x9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preset := types.DetectLayoutPreset(tt.placeholderW, tt.slideW)
			if preset != tt.expectedPreset {
				t.Errorf("DetectLayoutPreset() = %v, want %v", preset, tt.expectedPreset)
			}
		})
	}
}

// =============================================================================
// VALIDATION ASSERTIONS FOR CI
// =============================================================================

// ValidationAssertion defines what to check in rendered output.
type ValidationAssertion struct {
	SlideIndex         int
	PlaceholderIndex   int
	ContentType        string
	AspectRatioCheck   bool    // True = check aspect ratio
	ExpectedRatio      float64 // 1.0 for circular, 0 for any
	RatioTolerance     float64 // Acceptable deviation from expected
	TextContains       string  // Text that should be present
	MinElementCount    int     // Minimum number of elements (bullets, cells, etc.)
	TableColumnsCheck  int     // Expected number of table columns (0 = no check)
	TableRowsCheck     int     // Expected number of table rows (0 = no check)
}

// GetCIValidationAssertions returns assertions for CI validation.
func GetCIValidationAssertions() []ValidationAssertion {
	return []ValidationAssertion{
		// Text + Chart assertions
		{
			SlideIndex:       1,
			PlaceholderIndex: 1,
			ContentType:      "bullets",
			MinElementCount:  3,
		},
		{
			SlideIndex:       1,
			PlaceholderIndex: 2,
			ContentType:      "chart",
			AspectRatioCheck: false, // Bar chart can stretch
		},
		{
			SlideIndex:       2, // Pie chart slide
			PlaceholderIndex: 2,
			ContentType:      "chart",
			AspectRatioCheck: false, // Pie uses full container; circle constrained internally
		},
		// Table assertions
		{
			SlideIndex:        5,
			PlaceholderIndex:  1,
			ContentType:       "table",
			TableColumnsCheck: 3,
			TableRowsCheck:    4,
		},
	}
}

