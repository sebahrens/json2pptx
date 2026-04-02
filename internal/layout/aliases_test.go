package layout

import (
	"testing"
)

func TestResolveLayoutAlias(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Two-column aliases
		{"two-column resolves to content-2-50-50", "two-column", "content-2-50-50"},
		{"2-col resolves to content-2-50-50", "2-col", "content-2-50-50"},
		{"2col resolves to content-2-50-50", "2col", "content-2-50-50"},
		{"split resolves to content-2-50-50", "split", "content-2-50-50"},
		{"sidebar-right resolves to content-2-70-30", "sidebar-right", "content-2-70-30"},
		{"sidebar-left resolves to content-2-30-70", "sidebar-left", "content-2-30-70"},
		{"main-sidebar resolves to content-2-70-30", "main-sidebar", "content-2-70-30"},
		{"sidebar-main resolves to content-2-30-70", "sidebar-main", "content-2-30-70"},

		// Multi-column aliases
		{"three-column resolves to content-3", "three-column", "content-3"},
		{"3-col resolves to content-3", "3-col", "content-3"},
		{"3col resolves to content-3", "3col", "content-3"},
		{"four-column resolves to content-4", "four-column", "content-4"},
		{"4-col resolves to content-4", "4-col", "content-4"},
		{"five-column resolves to content-5", "five-column", "content-5"},
		{"5-col resolves to content-5", "5-col", "content-5"},

		// Grid aliases
		{"2x2 resolves to grid-2x2", "2x2", "grid-2x2"},
		{"grid resolves to grid-2x2", "grid", "grid-2x2"},
		{"3x3 resolves to grid-3x3", "3x3", "grid-3x3"},
		{"matrix resolves to grid-2x2", "matrix", "grid-2x2"},
		{"2x3 resolves to grid-2x3", "2x3", "grid-2x3"},
		{"3x2 resolves to grid-3x2", "3x2", "grid-3x2"},
		{"4x2 resolves to grid-4x2", "4x2", "grid-4x2"},
		{"4x3 resolves to grid-4x3", "4x3", "grid-4x3"},

		// Special layouts
		{"agenda resolves to agenda", "agenda", "agenda"},
		{"contents resolves to agenda", "contents", "agenda"},
		{"outline resolves to agenda", "outline", "agenda"},
		{"toc resolves to agenda", "toc", "agenda"},

		// Case insensitivity
		{"uppercase TWO-COLUMN resolves", "TWO-COLUMN", "content-2-50-50"},
		{"mixed case Two-Column resolves", "Two-Column", "content-2-50-50"},

		// Whitespace handling
		{"leading whitespace trimmed", "  two-column", "content-2-50-50"},
		{"trailing whitespace trimmed", "two-column  ", "content-2-50-50"},
		{"both whitespace trimmed", "  two-column  ", "content-2-50-50"},

		// Unknown aliases return input unchanged
		{"unknown alias returns input", "my-custom-layout", "my-custom-layout"},
		{"direct layout ID passes through", "content-2-50-50", "content-2-50-50"},
		{"direct grid ID passes through", "grid-3x3", "grid-3x3"},
		{"empty string returns empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveLayoutAlias(tt.input)
			if result != tt.expected {
				t.Errorf("ResolveLayoutAlias(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestClassifyGeneratedLayout(t *testing.T) {
	tests := []struct {
		name         string
		layoutID     string
		expectedTags []string
	}{
		{
			name:         "content-1 gets single-placeholder tag",
			layoutID:     "content-1",
			expectedTags: []string{"content", "single-placeholder"},
		},
		{
			name:         "content-2-50-50 gets two-column tags",
			layoutID:     "content-2-50-50",
			expectedTags: []string{"content", "two-column", "comparison"},
		},
		{
			name:         "content-2-70-30 gets sidebar-right tag",
			layoutID:     "content-2-70-30",
			expectedTags: []string{"content", "two-column", "comparison", "sidebar-right"},
		},
		{
			name:         "content-2-30-70 gets sidebar-left tag",
			layoutID:     "content-2-30-70",
			expectedTags: []string{"content", "two-column", "comparison", "sidebar-left"},
		},
		{
			name:         "content-3 gets three-column and multi-column tags",
			layoutID:     "content-3",
			expectedTags: []string{"content", "three-column", "multi-column"},
		},
		{
			name:         "content-4 gets four-column and multi-column tags",
			layoutID:     "content-4",
			expectedTags: []string{"content", "four-column", "multi-column"},
		},
		{
			name:         "content-5 gets five-column and multi-column tags",
			layoutID:     "content-5",
			expectedTags: []string{"content", "five-column", "multi-column"},
		},
		{
			name:         "grid-2x2 gets grid tags with dimensions",
			layoutID:     "grid-2x2",
			expectedTags: []string{"content", "grid", "grid-2x2"},
		},
		{
			name:         "grid-3x3 gets grid tags with dimensions",
			layoutID:     "grid-3x3",
			expectedTags: []string{"content", "grid", "grid-3x3"},
		},
		{
			name:         "grid-4x3 gets grid tags with dimensions",
			layoutID:     "grid-4x3",
			expectedTags: []string{"content", "grid", "grid-4x3"},
		},
		{
			name:         "agenda gets agenda tags",
			layoutID:     "agenda",
			expectedTags: []string{"agenda", "outline", "list"},
		},
		{
			name:         "unknown layout gets no tags",
			layoutID:     "my-custom-layout",
			expectedTags: nil,
		},
		{
			name:         "empty string gets no tags",
			layoutID:     "",
			expectedTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyGeneratedLayout(tt.layoutID)

			// Handle nil vs empty slice comparison
			if len(tt.expectedTags) == 0 && len(result) == 0 {
				return
			}

			if len(result) != len(tt.expectedTags) {
				t.Errorf("ClassifyGeneratedLayout(%q) returned %d tags, want %d\ngot: %v\nwant: %v",
					tt.layoutID, len(result), len(tt.expectedTags), result, tt.expectedTags)
				return
			}

			// Verify all expected tags are present
			for i, expected := range tt.expectedTags {
				if result[i] != expected {
					t.Errorf("ClassifyGeneratedLayout(%q) tag[%d] = %q, want %q",
						tt.layoutID, i, result[i], expected)
				}
			}
		})
	}
}

func TestIsGeneratedLayout(t *testing.T) {
	tests := []struct {
		name     string
		layoutID string
		expected bool
	}{
		// Content layouts
		{"content-1 is generated", "content-1", true},
		{"content-2-50-50 is generated", "content-2-50-50", true},
		{"content-2-70-30 is generated", "content-2-70-30", true},
		{"content-2-30-70 is generated", "content-2-30-70", true},
		{"content-3 is generated", "content-3", true},
		{"content-4 is generated", "content-4", true},
		{"content-5 is generated", "content-5", true},

		// Grid layouts
		{"grid-2x2 is generated", "grid-2x2", true},
		{"grid-2x3 is generated", "grid-2x3", true},
		{"grid-3x2 is generated", "grid-3x2", true},
		{"grid-3x3 is generated", "grid-3x3", true},
		{"grid-4x2 is generated", "grid-4x2", true},
		{"grid-4x3 is generated", "grid-4x3", true},

		// Special layouts
		{"agenda is generated", "agenda", true},

		// Non-generated layouts
		{"custom layout is not generated", "my-layout", false},
		{"title-slide is not generated", "title-slide", false},
		{"section-header is not generated", "section-header", false},
		{"empty is not generated", "", false},
		{"content without hyphen is not generated", "content", false},
		{"grid without hyphen is not generated", "grid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGeneratedLayout(tt.layoutID)
			if result != tt.expected {
				t.Errorf("IsGeneratedLayout(%q) = %v, want %v", tt.layoutID, result, tt.expected)
			}
		})
	}
}

func TestGetSlotCount(t *testing.T) {
	tests := []struct {
		name     string
		layoutID string
		expected int
	}{
		// Single content
		{"content-1 has 1 slot", "content-1", 1},

		// Two-column layouts
		{"content-2-50-50 has 2 slots", "content-2-50-50", 2},
		{"content-2-70-30 has 2 slots", "content-2-70-30", 2},
		{"content-2-30-70 has 2 slots", "content-2-30-70", 2},

		// Multi-column layouts
		{"content-3 has 3 slots", "content-3", 3},
		{"content-4 has 4 slots", "content-4", 4},
		{"content-5 has 5 slots", "content-5", 5},

		// Grid layouts
		{"grid-2x2 has 4 slots", "grid-2x2", 4},
		{"grid-2x3 has 6 slots", "grid-2x3", 6},
		{"grid-3x2 has 6 slots", "grid-3x2", 6},
		{"grid-3x3 has 9 slots", "grid-3x3", 9},
		{"grid-4x2 has 8 slots", "grid-4x2", 8},
		{"grid-4x3 has 12 slots", "grid-4x3", 12},

		// Special layouts
		{"agenda has 1 slot", "agenda", 1},

		// Unknown layouts
		{"unknown layout has 0 slots", "my-layout", 0},
		{"empty has 0 slots", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSlotCount(tt.layoutID)
			if result != tt.expected {
				t.Errorf("GetSlotCount(%q) = %d, want %d", tt.layoutID, result, tt.expected)
			}
		})
	}
}

// TestLayoutAliasesCompleteness verifies that all expected aliases are defined.
func TestLayoutAliasesCompleteness(t *testing.T) {
	// Verify key aliases exist
	expectedAliases := []string{
		"two-column", "2-col", "split",
		"three-column", "3-col",
		"four-column", "4-col",
		"five-column", "5-col",
		"sidebar-right", "sidebar-left",
		"2x2", "grid", "3x3", "matrix",
		"agenda", "contents", "outline",
	}

	for _, alias := range expectedAliases {
		if _, ok := LayoutAliases[alias]; !ok {
			t.Errorf("expected alias %q to be defined in LayoutAliases", alias)
		}
	}
}

// TestClassifyGeneratedLayoutHasTags verifies that classified layouts
// can be used for type matching in the heuristic selector.
func TestClassifyGeneratedLayoutHasTags(t *testing.T) {
	// Verify two-column layouts get the "two-column" tag
	// which is used by scoreTwoColumnSlide()
	twoColTags := ClassifyGeneratedLayout("content-2-50-50")
	hasTwoCol := false
	for _, tag := range twoColTags {
		if tag == "two-column" {
			hasTwoCol = true
			break
		}
	}
	if !hasTwoCol {
		t.Errorf("content-2-50-50 should have 'two-column' tag for heuristic matching")
	}

	// Verify content layouts get the "content" tag
	contentTags := ClassifyGeneratedLayout("content-1")
	hasContent := false
	for _, tag := range contentTags {
		if tag == "content" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		t.Errorf("content-1 should have 'content' tag for heuristic matching")
	}

	// Verify comparison tag for two-column layouts
	hasComparison := false
	for _, tag := range twoColTags {
		if tag == "comparison" {
			hasComparison = true
			break
		}
	}
	if !hasComparison {
		t.Errorf("content-2-50-50 should have 'comparison' tag for comparison slides")
	}
}
