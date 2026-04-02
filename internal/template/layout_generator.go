package template

import (
	"fmt"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// Default layout generation constants.
const (
	// DefaultGapBetweenPlaceholders is 0.2 inch in EMUs (914400 EMU/inch * 0.2).
	DefaultGapBetweenPlaceholders = 182880

	// DefaultRatioTolerance is the acceptable variance when matching ratios.
	DefaultRatioTolerance = 0.05
)

// LayoutGeneratorConfig configures layout generation behavior.
type LayoutGeneratorConfig struct {
	// GapBetweenPlaceholders is the space between adjacent placeholders in EMUs.
	// Default: 182880 (0.2 inch)
	GapBetweenPlaceholders int64

	// RatioTolerance is the acceptable variance when matching content to layouts.
	// For example, 0.05 means ±5% tolerance.
	RatioTolerance float64
}

// DefaultLayoutGeneratorConfig returns a config with sensible defaults.
func DefaultLayoutGeneratorConfig() LayoutGeneratorConfig {
	return LayoutGeneratorConfig{
		GapBetweenPlaceholders: DefaultGapBetweenPlaceholders,
		RatioTolerance:         DefaultRatioTolerance,
	}
}

// GeneratedLayout represents a programmatically created layout.
// These layouts are generated from a base content layout's style and dimensions.
type GeneratedLayout struct {
	ID           string                  // Unique layout identifier (e.g., "content-2-50-50")
	Name         string                  // Human-readable name (e.g., "Two Column (50/50)")
	BasedOnIdx   int                     // Index of the source content layout
	Placeholders []GeneratedPlaceholder  // Placeholders in this layout
	TitleBounds  *types.BoundingBox      // Resolved title bounds from base layout (nil = inherit from master)
}

// GeneratedPlaceholder represents a placeholder in a generated layout.
type GeneratedPlaceholder struct {
	ID     string               // Slot identifier (e.g., "slot1", "slot2")
	Index  int                  // Placeholder index (1-based)
	Type   string               // Placeholder type (always "body" for universal content)
	Bounds types.BoundingBox    // Position and size in EMUs
	Style  PlaceholderStyle     // Visual style copied from base layout
}

// HorizontalLayoutSpec defines a horizontal layout configuration.
type HorizontalLayoutSpec struct {
	ID     string    // Layout identifier
	Name   string    // Human-readable name
	Ratios []float64 // Width ratios (must sum to approximately 1.0)
}

// HorizontalLayouts defines the available horizontal layout configurations.
// These range from single content to five-column layouts with various ratio splits.
var HorizontalLayouts = []HorizontalLayoutSpec{
	{ID: "content-1", Name: "Single Content", Ratios: []float64{1.0}},
	{ID: "content-2-50-50", Name: "Two Column (50/50)", Ratios: []float64{0.5, 0.5}},
	{ID: "content-2-70-30", Name: "Two Column (70/30)", Ratios: []float64{0.7, 0.3}},
	{ID: "content-2-30-70", Name: "Two Column (30/70)", Ratios: []float64{0.3, 0.7}},
	{ID: "content-2-60-40", Name: "Two Column (60/40)", Ratios: []float64{0.6, 0.4}},
	{ID: "content-2-40-60", Name: "Two Column (40/60)", Ratios: []float64{0.4, 0.6}},
	{ID: "content-3", Name: "Three Column", Ratios: []float64{0.333, 0.334, 0.333}},
	{ID: "content-3-50-25-25", Name: "Three Column (50/25/25)", Ratios: []float64{0.5, 0.25, 0.25}},
	{ID: "content-3-25-50-25", Name: "Three Column (25/50/25)", Ratios: []float64{0.25, 0.5, 0.25}},
	{ID: "content-3-25-25-50", Name: "Three Column (25/25/50)", Ratios: []float64{0.25, 0.25, 0.5}},
	{ID: "content-4", Name: "Four Column", Ratios: []float64{0.25, 0.25, 0.25, 0.25}},
	{ID: "content-5", Name: "Five Column", Ratios: []float64{0.2, 0.2, 0.2, 0.2, 0.2}},
}

// CalculateHorizontalPositions calculates placeholder positions for a horizontal layout.
// It distributes available width according to the given ratios, with gaps between columns.
func CalculateHorizontalPositions(
	contentArea ContentAreaBounds,
	ratios []float64,
	gap int64,
) []types.BoundingBox {
	n := len(ratios)
	if n == 0 {
		return nil
	}

	// Calculate total gap space needed
	totalGaps := int64(n-1) * gap
	if totalGaps < 0 {
		totalGaps = 0
	}

	// Available width after accounting for gaps
	availableWidth := contentArea.Width - totalGaps
	if availableWidth < 0 {
		// Fallback: ignore gaps if they exceed width
		availableWidth = contentArea.Width
		gap = 0
	}

	positions := make([]types.BoundingBox, n)
	currentX := contentArea.X

	// Calculate width for each column based on ratios
	// Use cumulative calculation to avoid rounding errors
	for i := range n {
		var width int64
		if i == n-1 {
			// Last column gets remaining width to avoid rounding errors
			width = contentArea.X + contentArea.Width - currentX
			if i > 0 {
				width -= gap // Account for the gap we're adding
			}
		} else {
			width = int64(float64(availableWidth) * ratios[i])
		}

		positions[i] = types.BoundingBox{
			X:      currentX,
			Y:      contentArea.Y,
			Width:  width,
			Height: contentArea.Height,
		}

		currentX += width + gap
	}

	return positions
}

// GenerateHorizontalLayouts generates all horizontal layout variations
// based on the content area dimensions and base style.
func GenerateHorizontalLayouts(
	contentArea ContentAreaBounds,
	baseStyle PlaceholderStyle,
	config LayoutGeneratorConfig,
) []GeneratedLayout {
	layouts := make([]GeneratedLayout, 0, len(HorizontalLayouts))

	for _, spec := range HorizontalLayouts {
		layout := generateHorizontalLayout(contentArea, baseStyle, config, spec)
		layouts = append(layouts, layout)
	}

	return layouts
}

// generateHorizontalLayout creates a single horizontal layout from a spec.
func generateHorizontalLayout(
	contentArea ContentAreaBounds,
	baseStyle PlaceholderStyle,
	config LayoutGeneratorConfig,
	spec HorizontalLayoutSpec,
) GeneratedLayout {
	positions := CalculateHorizontalPositions(
		contentArea,
		spec.Ratios,
		config.GapBetweenPlaceholders,
	)

	layout := GeneratedLayout{
		ID:           spec.ID,
		Name:         spec.Name,
		BasedOnIdx:   -1, // Will be set by caller if needed
		Placeholders: make([]GeneratedPlaceholder, len(positions)),
	}

	for i, pos := range positions {
		// Create a copy of the base style for this placeholder
		slotStyle := baseStyle
		slotStyle.Bounds = pos

		layout.Placeholders[i] = GeneratedPlaceholder{
			ID:     fmt.Sprintf("slot%d", i+1),
			Index:  i + 1,
			Type:   "body",
			Bounds: pos,
			Style:  slotStyle,
		}
	}

	return layout
}

// GenerateHorizontalLayoutsFromTemplate generates horizontal layouts
// by extracting style and content area from a template.
func GenerateHorizontalLayoutsFromTemplate(
	reader *Reader,
	contentLayoutIdx int,
	config LayoutGeneratorConfig,
) ([]GeneratedLayout, error) {
	// Extract content area bounds
	contentArea, err := ExtractContentAreaBounds(reader, contentLayoutIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content area: %w", err)
	}

	// Extract placeholder style
	baseStyle, err := ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderBody)
	if err != nil {
		// Try content placeholder if body not found
		baseStyle, err = ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderContent)
		if err != nil {
			return nil, fmt.Errorf("failed to extract placeholder style: %w", err)
		}
	}

	// Generate layouts
	layouts := GenerateHorizontalLayouts(*contentArea, *baseStyle, config)

	// Set the base layout index for all generated layouts
	for i := range layouts {
		layouts[i].BasedOnIdx = contentLayoutIdx
	}

	return layouts, nil
}

// FindLayoutByID returns the layout with the given ID, or nil if not found.
func FindLayoutByID(layouts []GeneratedLayout, id string) *GeneratedLayout {
	for i := range layouts {
		if layouts[i].ID == id {
			return &layouts[i]
		}
	}
	return nil
}

// FindLayoutByColumnCount returns layouts with the specified number of columns.
func FindLayoutByColumnCount(layouts []GeneratedLayout, columns int) []GeneratedLayout {
	var matches []GeneratedLayout
	for _, layout := range layouts {
		if len(layout.Placeholders) == columns {
			matches = append(matches, layout)
		}
	}
	return matches
}

// GetLayoutColumnCount returns the number of columns in a layout.
func GetLayoutColumnCount(layout GeneratedLayout) int {
	return len(layout.Placeholders)
}

// ValidateLayoutRatios checks that ratios sum to approximately 1.0.
func ValidateLayoutRatios(ratios []float64, tolerance float64) bool {
	if len(ratios) == 0 {
		return false
	}

	var sum float64
	for _, r := range ratios {
		if r <= 0 || r > 1 {
			return false
		}
		sum += r
	}

	return sum >= 1.0-tolerance && sum <= 1.0+tolerance
}

// GridLayoutSpec defines a grid layout configuration.
type GridLayoutSpec struct {
	ID      string // Layout identifier
	Name    string // Human-readable name
	Columns int    // Number of columns
	Rows    int    // Number of rows
}

// GridLayouts defines the available grid layout configurations.
// These range from 2x2 to 4x3 grids.
var GridLayouts = []GridLayoutSpec{
	{ID: "grid-2x2", Name: "Grid 2x2", Columns: 2, Rows: 2},
	{ID: "grid-3x2", Name: "Grid 3x2", Columns: 3, Rows: 2},
	{ID: "grid-4x2", Name: "Grid 4x2", Columns: 4, Rows: 2},
	{ID: "grid-2x3", Name: "Grid 2x3", Columns: 2, Rows: 3},
	{ID: "grid-3x3", Name: "Grid 3x3", Columns: 3, Rows: 3},
	{ID: "grid-4x3", Name: "Grid 4x3", Columns: 4, Rows: 3},
}

// CalculateGridPositions calculates placeholder positions for a grid layout.
// It distributes available space into a uniform grid with gaps between cells.
// Placeholders are numbered left-to-right, top-to-bottom (row-major order).
func CalculateGridPositions(
	contentArea ContentAreaBounds,
	columns, rows int,
	hGap, vGap int64,
) []types.BoundingBox {
	if columns <= 0 || rows <= 0 {
		return nil
	}

	// Calculate total gap space
	totalHGaps := int64(columns-1) * hGap
	totalVGaps := int64(rows-1) * vGap

	// Available dimensions after accounting for gaps
	availableWidth := contentArea.Width - totalHGaps
	availableHeight := contentArea.Height - totalVGaps

	if availableWidth <= 0 || availableHeight <= 0 {
		// Fallback: ignore gaps if they exceed dimensions
		availableWidth = contentArea.Width
		availableHeight = contentArea.Height
		hGap = 0
		vGap = 0
	}

	// Calculate cell dimensions
	cellWidth := availableWidth / int64(columns)
	cellHeight := availableHeight / int64(rows)

	positions := make([]types.BoundingBox, columns*rows)

	for row := range rows {
		for col := range columns {
			idx := row*columns + col

			// Calculate position
			x := contentArea.X + int64(col)*(cellWidth+hGap)
			y := contentArea.Y + int64(row)*(cellHeight+vGap)

			// For the last column/row, extend to fill remaining space (avoid rounding gaps)
			width := cellWidth
			if col == columns-1 {
				width = contentArea.X + contentArea.Width - x
			}

			height := cellHeight
			if row == rows-1 {
				height = contentArea.Y + contentArea.Height - y
			}

			positions[idx] = types.BoundingBox{
				X:      x,
				Y:      y,
				Width:  width,
				Height: height,
			}
		}
	}

	return positions
}

// GenerateGridLayouts generates all grid layout variations
// based on the content area dimensions and base style.
func GenerateGridLayouts(
	contentArea ContentAreaBounds,
	baseStyle PlaceholderStyle,
	config LayoutGeneratorConfig,
) []GeneratedLayout {
	layouts := make([]GeneratedLayout, 0, len(GridLayouts))

	for _, spec := range GridLayouts {
		layout := generateGridLayout(contentArea, baseStyle, config, spec)
		layouts = append(layouts, layout)
	}

	return layouts
}

// generateGridLayout creates a single grid layout from a spec.
func generateGridLayout(
	contentArea ContentAreaBounds,
	baseStyle PlaceholderStyle,
	config LayoutGeneratorConfig,
	spec GridLayoutSpec,
) GeneratedLayout {
	// Use the same gap for both horizontal and vertical spacing
	gap := config.GapBetweenPlaceholders

	positions := CalculateGridPositions(
		contentArea,
		spec.Columns,
		spec.Rows,
		gap,
		gap,
	)

	layout := GeneratedLayout{
		ID:           spec.ID,
		Name:         spec.Name,
		BasedOnIdx:   -1, // Will be set by caller if needed
		Placeholders: make([]GeneratedPlaceholder, len(positions)),
	}

	for i, pos := range positions {
		// Create a copy of the base style for this placeholder
		slotStyle := baseStyle
		slotStyle.Bounds = pos

		layout.Placeholders[i] = GeneratedPlaceholder{
			ID:     fmt.Sprintf("slot%d", i+1),
			Index:  i + 1,
			Type:   "body",
			Bounds: pos,
			Style:  slotStyle,
		}
	}

	return layout
}

// GenerateGridLayoutsFromTemplate generates grid layouts
// by extracting style and content area from a template.
func GenerateGridLayoutsFromTemplate(
	reader *Reader,
	contentLayoutIdx int,
	config LayoutGeneratorConfig,
) ([]GeneratedLayout, error) {
	// Extract content area bounds
	contentArea, err := ExtractContentAreaBounds(reader, contentLayoutIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content area: %w", err)
	}

	// Extract placeholder style
	baseStyle, err := ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderBody)
	if err != nil {
		// Try content placeholder if body not found
		baseStyle, err = ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderContent)
		if err != nil {
			return nil, fmt.Errorf("failed to extract placeholder style: %w", err)
		}
	}

	// Generate layouts
	layouts := GenerateGridLayouts(*contentArea, *baseStyle, config)

	// Set the base layout index for all generated layouts
	for i := range layouts {
		layouts[i].BasedOnIdx = contentLayoutIdx
	}

	return layouts, nil
}

// GenerateAllLayouts generates all layout variations (horizontal and grid).
func GenerateAllLayouts(
	contentArea ContentAreaBounds,
	baseStyle PlaceholderStyle,
	config LayoutGeneratorConfig,
) []GeneratedLayout {
	horizontal := GenerateHorizontalLayouts(contentArea, baseStyle, config)
	grids := GenerateGridLayouts(contentArea, baseStyle, config)
	return append(horizontal, grids...)
}

// GenerateAllLayoutsFromTemplate generates all layouts from a template.
func GenerateAllLayoutsFromTemplate(
	reader *Reader,
	contentLayoutIdx int,
	config LayoutGeneratorConfig,
) ([]GeneratedLayout, error) {
	// Extract content area bounds
	contentArea, err := ExtractContentAreaBounds(reader, contentLayoutIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to extract content area: %w", err)
	}

	// Extract placeholder style
	baseStyle, err := ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderBody)
	if err != nil {
		// Try content placeholder if body not found
		baseStyle, err = ExtractPlaceholderStyle(reader, contentLayoutIdx, types.PlaceholderContent)
		if err != nil {
			return nil, fmt.Errorf("failed to extract placeholder style: %w", err)
		}
	}

	// Generate all layouts
	layouts := GenerateAllLayouts(*contentArea, *baseStyle, config)

	// Set the base layout index for all generated layouts
	for i := range layouts {
		layouts[i].BasedOnIdx = contentLayoutIdx
	}

	return layouts, nil
}

// GetGridDimensions returns the column and row count for a grid layout.
// Returns (0, 0) if the layout is not a grid layout.
func GetGridDimensions(layout GeneratedLayout) (columns, rows int) {
	for _, spec := range GridLayouts {
		if spec.ID == layout.ID {
			return spec.Columns, spec.Rows
		}
	}
	return 0, 0
}

// FindLayoutByDimensions returns grid layouts matching the given dimensions.
func FindLayoutByDimensions(layouts []GeneratedLayout, columns, rows int) []GeneratedLayout {
	var matches []GeneratedLayout
	for _, layout := range layouts {
		cols, rws := GetGridDimensions(layout)
		if cols == columns && rws == rows {
			matches = append(matches, layout)
		}
	}
	return matches
}
