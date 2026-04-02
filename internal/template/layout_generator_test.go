package template

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestCalculateHorizontalPositions_SingleColumn(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      100000,
		Y:      200000,
		Width:  8000000,
		Height: 4000000,
	}

	positions := CalculateHorizontalPositions(contentArea, []float64{1.0}, 182880)

	if len(positions) != 1 {
		t.Fatalf("Expected 1 position, got %d", len(positions))
	}

	pos := positions[0]
	if pos.X != contentArea.X {
		t.Errorf("X = %d, want %d", pos.X, contentArea.X)
	}
	if pos.Y != contentArea.Y {
		t.Errorf("Y = %d, want %d", pos.Y, contentArea.Y)
	}
	if pos.Width != contentArea.Width {
		t.Errorf("Width = %d, want %d", pos.Width, contentArea.Width)
	}
	if pos.Height != contentArea.Height {
		t.Errorf("Height = %d, want %d", pos.Height, contentArea.Height)
	}
}

func TestCalculateHorizontalPositions_TwoColumns(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      100000,
		Y:      200000,
		Width:  8000000, // 8 million EMUs
		Height: 4000000,
	}
	gap := int64(182880) // ~0.2 inch

	positions := CalculateHorizontalPositions(contentArea, []float64{0.5, 0.5}, gap)

	if len(positions) != 2 {
		t.Fatalf("Expected 2 positions, got %d", len(positions))
	}

	// Available width after gap
	availableWidth := contentArea.Width - gap

	// First column
	expectedWidth1 := int64(float64(availableWidth) * 0.5)
	if positions[0].X != contentArea.X {
		t.Errorf("First column X = %d, want %d", positions[0].X, contentArea.X)
	}
	if positions[0].Width != expectedWidth1 {
		t.Errorf("First column Width = %d, want %d", positions[0].Width, expectedWidth1)
	}

	// Second column should start after first column + gap
	expectedX2 := contentArea.X + expectedWidth1 + gap
	if positions[1].X != expectedX2 {
		t.Errorf("Second column X = %d, want %d", positions[1].X, expectedX2)
	}

	// Both columns should have same Y and Height
	for i, pos := range positions {
		if pos.Y != contentArea.Y {
			t.Errorf("Column %d Y = %d, want %d", i+1, pos.Y, contentArea.Y)
		}
		if pos.Height != contentArea.Height {
			t.Errorf("Column %d Height = %d, want %d", i+1, pos.Height, contentArea.Height)
		}
	}

	// Total width should approximately equal content area width
	totalWidth := positions[0].Width + gap + positions[1].Width
	if totalWidth != contentArea.Width {
		t.Logf("Note: Total width %d differs from content width %d by %d EMUs",
			totalWidth, contentArea.Width, contentArea.Width-totalWidth)
	}
}

func TestCalculateHorizontalPositions_ThreeColumns(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  9000000,
		Height: 5000000,
	}
	gap := int64(100000)

	positions := CalculateHorizontalPositions(contentArea, []float64{0.333, 0.334, 0.333}, gap)

	if len(positions) != 3 {
		t.Fatalf("Expected 3 positions, got %d", len(positions))
	}

	// Verify all columns have the same Y and Height
	for i, pos := range positions {
		if pos.Y != 0 {
			t.Errorf("Column %d Y = %d, want 0", i+1, pos.Y)
		}
		if pos.Height != contentArea.Height {
			t.Errorf("Column %d Height = %d, want %d", i+1, pos.Height, contentArea.Height)
		}
	}

	// Verify columns don't overlap
	for i := 0; i < len(positions)-1; i++ {
		endOfCurrent := positions[i].X + positions[i].Width
		startOfNext := positions[i+1].X
		if endOfCurrent > startOfNext {
			t.Errorf("Column %d overlaps with column %d: end %d > start %d",
				i+1, i+2, endOfCurrent, startOfNext)
		}
	}
}

func TestCalculateHorizontalPositions_EmptyRatios(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 4000000,
	}

	positions := CalculateHorizontalPositions(contentArea, []float64{}, 182880)

	if positions != nil {
		t.Errorf("Expected nil for empty ratios, got %v", positions)
	}
}

func TestCalculateHorizontalPositions_AsymmetricRatios(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  10000000,
		Height: 5000000,
	}
	gap := int64(200000)

	// 70/30 split
	positions := CalculateHorizontalPositions(contentArea, []float64{0.7, 0.3}, gap)

	if len(positions) != 2 {
		t.Fatalf("Expected 2 positions, got %d", len(positions))
	}

	// First column should be roughly 70% of available width
	availableWidth := contentArea.Width - gap
	expectedWidth1 := int64(float64(availableWidth) * 0.7)

	if positions[0].Width != expectedWidth1 {
		t.Errorf("First column width = %d, want approximately %d", positions[0].Width, expectedWidth1)
	}

	// Second column takes remaining space
	if positions[1].Width <= 0 {
		t.Error("Second column should have positive width")
	}
}

func TestGenerateHorizontalLayouts(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      500000,
		Y:      1500000,
		Width:  8000000,
		Height: 4500000,
	}

	baseStyle := PlaceholderStyle{
		FontFamily: "Arial",
		FontSize:   1800,
		FontColor:  "#000000",
	}

	config := DefaultLayoutGeneratorConfig()

	layouts := GenerateHorizontalLayouts(contentArea, baseStyle, config)

	// Should generate all horizontal layouts
	if len(layouts) != len(HorizontalLayouts) {
		t.Errorf("Generated %d layouts, expected %d", len(layouts), len(HorizontalLayouts))
	}

	// Verify each layout
	for _, layout := range layouts {
		if layout.ID == "" {
			t.Error("Layout ID should not be empty")
		}
		if layout.Name == "" {
			t.Error("Layout Name should not be empty")
		}
		if len(layout.Placeholders) == 0 {
			t.Errorf("Layout %s has no placeholders", layout.ID)
		}

		// Verify placeholders
		for i, ph := range layout.Placeholders {
			if ph.Index != i+1 {
				t.Errorf("Placeholder index = %d, want %d", ph.Index, i+1)
			}
			if ph.Type != "body" {
				t.Errorf("Placeholder type = %s, want body", ph.Type)
			}
			if ph.Style.FontFamily != "Arial" {
				t.Errorf("Placeholder style FontFamily = %s, want Arial", ph.Style.FontFamily)
			}
		}
	}
}

func TestFindLayoutByID(t *testing.T) {
	layouts := []GeneratedLayout{
		{ID: "content-1", Name: "Single"},
		{ID: "content-2-50-50", Name: "Two Column"},
		{ID: "content-3", Name: "Three Column"},
	}

	tests := []struct {
		id   string
		want bool
	}{
		{"content-1", true},
		{"content-2-50-50", true},
		{"content-3", true},
		{"nonexistent", false},
		{"", false},
	}

	for _, tt := range tests {
		result := FindLayoutByID(layouts, tt.id)
		found := result != nil
		if found != tt.want {
			t.Errorf("FindLayoutByID(%q) found = %v, want %v", tt.id, found, tt.want)
		}
		if found && result.ID != tt.id {
			t.Errorf("FindLayoutByID(%q) returned ID = %s", tt.id, result.ID)
		}
	}
}

func TestFindLayoutByColumnCount(t *testing.T) {
	layouts := []GeneratedLayout{
		{ID: "single", Placeholders: make([]GeneratedPlaceholder, 1)},
		{ID: "two-a", Placeholders: make([]GeneratedPlaceholder, 2)},
		{ID: "two-b", Placeholders: make([]GeneratedPlaceholder, 2)},
		{ID: "three", Placeholders: make([]GeneratedPlaceholder, 3)},
	}

	tests := []struct {
		columns int
		want    int
	}{
		{1, 1},
		{2, 2},
		{3, 1},
		{4, 0},
		{0, 0},
	}

	for _, tt := range tests {
		results := FindLayoutByColumnCount(layouts, tt.columns)
		if len(results) != tt.want {
			t.Errorf("FindLayoutByColumnCount(%d) returned %d results, want %d",
				tt.columns, len(results), tt.want)
		}
	}
}

func TestValidateLayoutRatios(t *testing.T) {
	tests := []struct {
		name      string
		ratios    []float64
		tolerance float64
		want      bool
	}{
		{"single 1.0", []float64{1.0}, 0.05, true},
		{"two 50/50", []float64{0.5, 0.5}, 0.05, true},
		{"three equal", []float64{0.333, 0.334, 0.333}, 0.05, true},
		{"four equal", []float64{0.25, 0.25, 0.25, 0.25}, 0.05, true},
		{"slightly over", []float64{0.51, 0.51}, 0.05, true},
		{"too high", []float64{0.6, 0.6}, 0.05, false},
		{"too low", []float64{0.3, 0.3}, 0.05, false},
		{"negative", []float64{0.5, -0.5}, 0.05, false},
		{"zero", []float64{0, 0.5}, 0.05, false},
		{"empty", []float64{}, 0.05, false},
		{"over 1", []float64{1.5}, 0.05, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateLayoutRatios(tt.ratios, tt.tolerance)
			if got != tt.want {
				t.Errorf("ValidateLayoutRatios(%v, %v) = %v, want %v",
					tt.ratios, tt.tolerance, got, tt.want)
			}
		})
	}
}

func TestGetLayoutColumnCount(t *testing.T) {
	tests := []struct {
		layout GeneratedLayout
		want   int
	}{
		{GeneratedLayout{Placeholders: make([]GeneratedPlaceholder, 1)}, 1},
		{GeneratedLayout{Placeholders: make([]GeneratedPlaceholder, 2)}, 2},
		{GeneratedLayout{Placeholders: make([]GeneratedPlaceholder, 5)}, 5},
		{GeneratedLayout{Placeholders: nil}, 0},
	}

	for _, tt := range tests {
		got := GetLayoutColumnCount(tt.layout)
		if got != tt.want {
			t.Errorf("GetLayoutColumnCount() = %d, want %d", got, tt.want)
		}
	}
}

func TestDefaultLayoutGeneratorConfig(t *testing.T) {
	config := DefaultLayoutGeneratorConfig()

	if config.GapBetweenPlaceholders != DefaultGapBetweenPlaceholders {
		t.Errorf("Default gap = %d, want %d",
			config.GapBetweenPlaceholders, DefaultGapBetweenPlaceholders)
	}
	if config.RatioTolerance != DefaultRatioTolerance {
		t.Errorf("Default tolerance = %f, want %f",
			config.RatioTolerance, DefaultRatioTolerance)
	}
}

func TestHorizontalLayoutsValidRatios(t *testing.T) {
	for _, spec := range HorizontalLayouts {
		if !ValidateLayoutRatios(spec.Ratios, 0.01) {
			t.Errorf("Layout %s has invalid ratios: %v", spec.ID, spec.Ratios)
		}
	}
}

func TestGenerateHorizontalLayoutsFromTemplate(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	config := DefaultLayoutGeneratorConfig()

	// Try a few layout indices to find one with a content placeholder
	var layouts []GeneratedLayout
	for i := 0; i < 5; i++ {
		layouts, err = GenerateHorizontalLayoutsFromTemplate(reader, i, config)
		if err == nil {
			break
		}
	}

	if layouts == nil {
		t.Fatal("Could not generate layouts from any layout index")
	}

	// Should generate all horizontal layouts
	if len(layouts) != len(HorizontalLayouts) {
		t.Errorf("Generated %d layouts, expected %d", len(layouts), len(HorizontalLayouts))
	}

	// Verify BasedOnIdx is set
	for _, layout := range layouts {
		if layout.BasedOnIdx < 0 {
			t.Errorf("Layout %s has invalid BasedOnIdx: %d", layout.ID, layout.BasedOnIdx)
		}
	}

	t.Logf("Successfully generated %d horizontal layouts", len(layouts))
}

func TestCalculateHorizontalPositions_NoGaps(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  9000000,
		Height: 5000000,
	}

	positions := CalculateHorizontalPositions(contentArea, []float64{0.5, 0.5}, 0)

	if len(positions) != 2 {
		t.Fatalf("Expected 2 positions, got %d", len(positions))
	}

	// With no gap, columns should be adjacent
	expectedWidth := contentArea.Width / 2
	if positions[0].Width != expectedWidth {
		t.Errorf("First column width = %d, want %d", positions[0].Width, expectedWidth)
	}

	// Second column should start exactly where first ends
	if positions[1].X != positions[0].Width {
		t.Errorf("Second column X = %d, want %d (no gap)", positions[1].X, positions[0].Width)
	}
}

func TestGeneratedPlaceholder_SlotNaming(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 4000000,
	}

	config := DefaultLayoutGeneratorConfig()
	layout := generateHorizontalLayout(contentArea, PlaceholderStyle{}, config,
		HorizontalLayoutSpec{ID: "test", Name: "Test", Ratios: []float64{0.25, 0.25, 0.25, 0.25}})

	expectedNames := []string{"slot1", "slot2", "slot3", "slot4"}
	for i, ph := range layout.Placeholders {
		if ph.ID != expectedNames[i] {
			t.Errorf("Placeholder %d ID = %s, want %s", i, ph.ID, expectedNames[i])
		}
	}
}

func TestGeneratedLayout_TypesMatch(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      100000,
		Y:      200000,
		Width:  8000000,
		Height: 4000000,
	}

	baseStyle := PlaceholderStyle{
		Bounds: types.BoundingBox{X: 0, Y: 0, Width: 100, Height: 100},
	}

	config := DefaultLayoutGeneratorConfig()
	layouts := GenerateHorizontalLayouts(contentArea, baseStyle, config)

	for _, layout := range layouts {
		for _, ph := range layout.Placeholders {
			// Bounds should be within content area
			if ph.Bounds.X < contentArea.X {
				t.Errorf("Layout %s: placeholder X %d < content area X %d",
					layout.ID, ph.Bounds.X, contentArea.X)
			}
			if ph.Bounds.Y != contentArea.Y {
				t.Errorf("Layout %s: placeholder Y %d != content area Y %d",
					layout.ID, ph.Bounds.Y, contentArea.Y)
			}
			// Style bounds should match position bounds
			if ph.Style.Bounds != ph.Bounds {
				t.Errorf("Layout %s: style bounds don't match placeholder bounds", layout.ID)
			}
		}
	}
}

// Grid Layout Tests

func TestCalculateGridPositions_2x2(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 6000000,
	}
	gap := int64(100000)

	positions := CalculateGridPositions(contentArea, 2, 2, gap, gap)

	if len(positions) != 4 {
		t.Fatalf("Expected 4 positions for 2x2 grid, got %d", len(positions))
	}

	// Verify positions are in row-major order
	// Expected order: [0,0], [0,1], [1,0], [1,1] (row, col)

	// First row, first column
	if positions[0].X != 0 || positions[0].Y != 0 {
		t.Errorf("Position 0: X=%d, Y=%d, want X=0, Y=0", positions[0].X, positions[0].Y)
	}

	// First row, second column
	if positions[1].Y != 0 {
		t.Errorf("Position 1: Y=%d, want Y=0", positions[1].Y)
	}
	if positions[1].X <= positions[0].X {
		t.Errorf("Position 1 should be to the right of position 0")
	}

	// Second row, first column
	if positions[2].X != 0 {
		t.Errorf("Position 2: X=%d, want X=0", positions[2].X)
	}
	if positions[2].Y <= positions[0].Y {
		t.Errorf("Position 2 should be below position 0")
	}

	// Second row, second column
	if positions[3].X <= positions[2].X {
		t.Errorf("Position 3 should be to the right of position 2")
	}
	if positions[3].Y <= positions[1].Y {
		t.Errorf("Position 3 should be below position 1")
	}
}

func TestCalculateGridPositions_3x2(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      100000,
		Y:      200000,
		Width:  9000000,
		Height: 6000000,
	}
	gap := int64(150000)

	positions := CalculateGridPositions(contentArea, 3, 2, gap, gap)

	if len(positions) != 6 {
		t.Fatalf("Expected 6 positions for 3x2 grid, got %d", len(positions))
	}

	// Verify first row has 3 items
	for i := 0; i < 3; i++ {
		if positions[i].Y != contentArea.Y {
			t.Errorf("Position %d should be in first row (Y=%d), got Y=%d",
				i, contentArea.Y, positions[i].Y)
		}
	}

	// Verify second row has 3 items
	for i := 3; i < 6; i++ {
		if positions[i].Y <= contentArea.Y {
			t.Errorf("Position %d should be in second row, but Y=%d", i, positions[i].Y)
		}
	}

	// Verify columns are in order
	for i := 0; i < 2; i++ {
		if positions[i+1].X <= positions[i].X {
			t.Errorf("Position %d should be to the right of position %d", i+1, i)
		}
	}
}

func TestCalculateGridPositions_4x3(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  10000000,
		Height: 7500000,
	}
	gap := int64(100000)

	positions := CalculateGridPositions(contentArea, 4, 3, gap, gap)

	if len(positions) != 12 {
		t.Fatalf("Expected 12 positions for 4x3 grid, got %d", len(positions))
	}

	// Verify all positions are within content area
	for i, pos := range positions {
		if pos.X < contentArea.X {
			t.Errorf("Position %d X=%d is less than content area X=%d", i, pos.X, contentArea.X)
		}
		if pos.Y < contentArea.Y {
			t.Errorf("Position %d Y=%d is less than content area Y=%d", i, pos.Y, contentArea.Y)
		}
		// Check right edge (with some tolerance for last column)
		if pos.X+pos.Width > contentArea.X+contentArea.Width+1 {
			t.Errorf("Position %d right edge exceeds content area", i)
		}
		// Check bottom edge (with some tolerance for last row)
		if pos.Y+pos.Height > contentArea.Y+contentArea.Height+1 {
			t.Errorf("Position %d bottom edge exceeds content area", i)
		}
	}
}

func TestCalculateGridPositions_InvalidDimensions(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 6000000,
	}

	// Zero columns
	if positions := CalculateGridPositions(contentArea, 0, 2, 0, 0); positions != nil {
		t.Error("Expected nil for 0 columns")
	}

	// Zero rows
	if positions := CalculateGridPositions(contentArea, 2, 0, 0, 0); positions != nil {
		t.Error("Expected nil for 0 rows")
	}

	// Negative columns
	if positions := CalculateGridPositions(contentArea, -1, 2, 0, 0); positions != nil {
		t.Error("Expected nil for negative columns")
	}

	// Negative rows
	if positions := CalculateGridPositions(contentArea, 2, -1, 0, 0); positions != nil {
		t.Error("Expected nil for negative rows")
	}
}

func TestCalculateGridPositions_NoGaps(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 6000000,
	}

	positions := CalculateGridPositions(contentArea, 2, 2, 0, 0)

	if len(positions) != 4 {
		t.Fatalf("Expected 4 positions, got %d", len(positions))
	}

	// With no gaps, cells should be adjacent
	expectedCellWidth := contentArea.Width / 2
	expectedCellHeight := contentArea.Height / 2

	if positions[0].Width != expectedCellWidth {
		t.Errorf("Cell width = %d, want %d", positions[0].Width, expectedCellWidth)
	}
	if positions[0].Height != expectedCellHeight {
		t.Errorf("Cell height = %d, want %d", positions[0].Height, expectedCellHeight)
	}

	// Position [1] should start exactly where [0] ends horizontally
	if positions[1].X != positions[0].Width {
		t.Errorf("Position 1 X = %d, want %d", positions[1].X, positions[0].Width)
	}

	// Position [2] should start exactly where [0] ends vertically
	if positions[2].Y != positions[0].Height {
		t.Errorf("Position 2 Y = %d, want %d", positions[2].Y, positions[0].Height)
	}
}

func TestCalculateGridPositions_NoOverlap(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  9000000,
		Height: 6000000,
	}
	gap := int64(100000)

	positions := CalculateGridPositions(contentArea, 3, 2, gap, gap)

	// Check no horizontal overlaps within rows
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			i := row*3 + col
			j := i + 1
			rightEdge := positions[i].X + positions[i].Width
			if rightEdge > positions[j].X {
				t.Errorf("Position %d overlaps with position %d horizontally: right edge %d > left edge %d",
					i, j, rightEdge, positions[j].X)
			}
		}
	}

	// Check no vertical overlaps within columns
	for col := 0; col < 3; col++ {
		i := col         // First row
		j := col + 3     // Second row
		bottomEdge := positions[i].Y + positions[i].Height
		if bottomEdge > positions[j].Y {
			t.Errorf("Position %d overlaps with position %d vertically: bottom edge %d > top edge %d",
				i, j, bottomEdge, positions[j].Y)
		}
	}
}

func TestGenerateGridLayouts(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      500000,
		Y:      1500000,
		Width:  8000000,
		Height: 4500000,
	}

	baseStyle := PlaceholderStyle{
		FontFamily: "Arial",
		FontSize:   1800,
		FontColor:  "#000000",
	}

	config := DefaultLayoutGeneratorConfig()

	layouts := GenerateGridLayouts(contentArea, baseStyle, config)

	// Should generate all grid layouts
	if len(layouts) != len(GridLayouts) {
		t.Errorf("Generated %d layouts, expected %d", len(layouts), len(GridLayouts))
	}

	// Verify each layout
	for _, layout := range layouts {
		if layout.ID == "" {
			t.Error("Layout ID should not be empty")
		}
		if layout.Name == "" {
			t.Error("Layout Name should not be empty")
		}

		// Get expected dimensions from spec
		cols, rows := GetGridDimensions(layout)
		expectedCount := cols * rows

		if len(layout.Placeholders) != expectedCount {
			t.Errorf("Layout %s has %d placeholders, expected %d",
				layout.ID, len(layout.Placeholders), expectedCount)
		}

		// Verify placeholder naming
		for i, ph := range layout.Placeholders {
			expectedID := fmt.Sprintf("slot%d", i+1)
			if ph.ID != expectedID {
				t.Errorf("Layout %s: placeholder %d ID = %s, want %s",
					layout.ID, i, ph.ID, expectedID)
			}
			if ph.Index != i+1 {
				t.Errorf("Layout %s: placeholder Index = %d, want %d",
					layout.ID, ph.Index, i+1)
			}
			if ph.Type != "body" {
				t.Errorf("Layout %s: placeholder type = %s, want body", layout.ID, ph.Type)
			}
		}
	}
}

func TestGenerateGridLayoutsFromTemplate(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	config := DefaultLayoutGeneratorConfig()

	// Try a few layout indices to find one with a content placeholder
	var layouts []GeneratedLayout
	for i := 0; i < 5; i++ {
		layouts, err = GenerateGridLayoutsFromTemplate(reader, i, config)
		if err == nil {
			break
		}
	}

	if layouts == nil {
		t.Fatal("Could not generate grid layouts from any layout index")
	}

	// Should generate all grid layouts
	if len(layouts) != len(GridLayouts) {
		t.Errorf("Generated %d layouts, expected %d", len(layouts), len(GridLayouts))
	}

	// Verify BasedOnIdx is set
	for _, layout := range layouts {
		if layout.BasedOnIdx < 0 {
			t.Errorf("Layout %s has invalid BasedOnIdx: %d", layout.ID, layout.BasedOnIdx)
		}
	}

	t.Logf("Successfully generated %d grid layouts", len(layouts))
}

func TestGetGridDimensions(t *testing.T) {
	tests := []struct {
		layoutID    string
		wantColumns int
		wantRows    int
	}{
		{"grid-2x2", 2, 2},
		{"grid-3x2", 3, 2},
		{"grid-4x2", 4, 2},
		{"grid-2x3", 2, 3},
		{"grid-3x3", 3, 3},
		{"grid-4x3", 4, 3},
		{"content-1", 0, 0},         // Not a grid layout
		{"nonexistent", 0, 0},       // Unknown layout
	}

	for _, tt := range tests {
		layout := GeneratedLayout{ID: tt.layoutID}
		cols, rows := GetGridDimensions(layout)
		if cols != tt.wantColumns || rows != tt.wantRows {
			t.Errorf("GetGridDimensions(%s) = (%d, %d), want (%d, %d)",
				tt.layoutID, cols, rows, tt.wantColumns, tt.wantRows)
		}
	}
}

func TestFindLayoutByDimensions(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 6000000,
	}
	config := DefaultLayoutGeneratorConfig()

	layouts := GenerateGridLayouts(contentArea, PlaceholderStyle{}, config)

	tests := []struct {
		columns int
		rows    int
		want    int // Expected number of matches
	}{
		{2, 2, 1},
		{3, 2, 1},
		{4, 3, 1},
		{5, 5, 0}, // No such grid
		{0, 0, 0},
	}

	for _, tt := range tests {
		results := FindLayoutByDimensions(layouts, tt.columns, tt.rows)
		if len(results) != tt.want {
			t.Errorf("FindLayoutByDimensions(%d, %d) returned %d results, want %d",
				tt.columns, tt.rows, len(results), tt.want)
		}
	}
}

func TestGenerateAllLayouts(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      0,
		Y:      0,
		Width:  8000000,
		Height: 6000000,
	}
	config := DefaultLayoutGeneratorConfig()

	layouts := GenerateAllLayouts(contentArea, PlaceholderStyle{}, config)

	expectedCount := len(HorizontalLayouts) + len(GridLayouts)
	if len(layouts) != expectedCount {
		t.Errorf("GenerateAllLayouts returned %d layouts, expected %d",
			len(layouts), expectedCount)
	}

	// Verify we have both horizontal and grid layouts
	hasHorizontal := false
	hasGrid := false
	for _, layout := range layouts {
		if layout.ID == "content-1" {
			hasHorizontal = true
		}
		if layout.ID == "grid-2x2" {
			hasGrid = true
		}
	}

	if !hasHorizontal {
		t.Error("GenerateAllLayouts missing horizontal layouts")
	}
	if !hasGrid {
		t.Error("GenerateAllLayouts missing grid layouts")
	}
}

func TestGenerateAllLayoutsFromTemplate(t *testing.T) {
	templatePath := filepath.Join("testdata", "standard.pptx")

	reader, err := OpenTemplate(templatePath)
	if err != nil {
		t.Fatalf("Failed to open template: %v", err)
	}
	defer func() { _ = reader.Close() }()

	config := DefaultLayoutGeneratorConfig()

	// Try a few layout indices to find one with a content placeholder
	var layouts []GeneratedLayout
	for i := 0; i < 5; i++ {
		layouts, err = GenerateAllLayoutsFromTemplate(reader, i, config)
		if err == nil {
			break
		}
	}

	if layouts == nil {
		t.Fatal("Could not generate layouts from any layout index")
	}

	expectedCount := len(HorizontalLayouts) + len(GridLayouts)
	if len(layouts) != expectedCount {
		t.Errorf("Generated %d layouts, expected %d", len(layouts), expectedCount)
	}

	t.Logf("Successfully generated %d total layouts (horizontal + grid)", len(layouts))
}

func TestGridLayoutsValidSpecs(t *testing.T) {
	for _, spec := range GridLayouts {
		if spec.ID == "" {
			t.Error("Grid layout spec has empty ID")
		}
		if spec.Name == "" {
			t.Error("Grid layout spec has empty Name")
		}
		if spec.Columns < 1 || spec.Columns > 4 {
			t.Errorf("Grid layout %s has invalid Columns: %d", spec.ID, spec.Columns)
		}
		if spec.Rows < 1 || spec.Rows > 3 {
			t.Errorf("Grid layout %s has invalid Rows: %d", spec.ID, spec.Rows)
		}
	}
}

func TestGridLayout_BoundsWithinContentArea(t *testing.T) {
	contentArea := ContentAreaBounds{
		X:      100000,
		Y:      200000,
		Width:  8000000,
		Height: 6000000,
	}

	baseStyle := PlaceholderStyle{}
	config := DefaultLayoutGeneratorConfig()
	layouts := GenerateGridLayouts(contentArea, baseStyle, config)

	for _, layout := range layouts {
		for _, ph := range layout.Placeholders {
			// Check X bounds
			if ph.Bounds.X < contentArea.X {
				t.Errorf("Layout %s: placeholder X %d < content area X %d",
					layout.ID, ph.Bounds.X, contentArea.X)
			}

			// Check Y bounds
			if ph.Bounds.Y < contentArea.Y {
				t.Errorf("Layout %s: placeholder Y %d < content area Y %d",
					layout.ID, ph.Bounds.Y, contentArea.Y)
			}

			// Check right edge
			rightEdge := ph.Bounds.X + ph.Bounds.Width
			contentRight := contentArea.X + contentArea.Width
			if rightEdge > contentRight+1 { // +1 for rounding tolerance
				t.Errorf("Layout %s: placeholder right edge %d > content area right %d",
					layout.ID, rightEdge, contentRight)
			}

			// Check bottom edge
			bottomEdge := ph.Bounds.Y + ph.Bounds.Height
			contentBottom := contentArea.Y + contentArea.Height
			if bottomEdge > contentBottom+1 { // +1 for rounding tolerance
				t.Errorf("Layout %s: placeholder bottom edge %d > content area bottom %d",
					layout.ID, bottomEdge, contentBottom)
			}

			// Verify style bounds match position bounds
			if ph.Style.Bounds != ph.Bounds {
				t.Errorf("Layout %s: style bounds don't match placeholder bounds", layout.ID)
			}
		}
	}
}
