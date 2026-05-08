package visualqa

import "testing"

func TestSuggestedFixesForCategory(t *testing.T) {
	tests := []struct {
		category  string
		wantLen   int
		wantFirst string
	}{
		{"text_overflow", 3, "reduce_cell_text"},
		{"contrast", 2, "replace_color"},
		{"alignment", 2, "reshape_grid"},
		{"spacing", 1, "reshape_grid"},
		{"font_size", 2, "reduce_text"},
		{"missing_content", 1, "provide_value"},
		{"image_quality", 0, ""},  // review-only, no mapping
		{"aspect_ratio", 0, ""},   // review-only, no mapping
		{"border_style", 0, ""},   // review-only, no mapping
		{"unknown_cat", 0, ""},    // unknown category
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			fixes := SuggestedFixesForCategory(tt.category)
			if len(fixes) != tt.wantLen {
				t.Errorf("SuggestedFixesForCategory(%q) returned %d fixes, want %d", tt.category, len(fixes), tt.wantLen)
			}
			if tt.wantLen > 0 && fixes[0].Kind != tt.wantFirst {
				t.Errorf("first fix kind = %q, want %q", fixes[0].Kind, tt.wantFirst)
			}
		})
	}
}

func TestAllMappedCategoriesAreValid(t *testing.T) {
	for cat := range categoryFixMap {
		if !ValidCategory(cat) {
			t.Errorf("categoryFixMap contains invalid category %q", cat)
		}
	}
}

func TestIsReviewOnly(t *testing.T) {
	tests := []struct {
		category string
		want     bool
	}{
		// Categories with deterministic fix mappings are NOT review-only.
		{"text_overflow", false},
		{"contrast", false},
		{"reshape_grid", false}, // not a valid category at all
		// Review-only categories: valid but no fix mapping.
		{"image_quality", true},
		{"aspect_ratio", true},
		{"border_style", true},
		// Unknown categories are not review-only (they're invalid).
		{"unknown_cat", false},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			got := IsReviewOnly(tt.category)
			if got != tt.want {
				t.Errorf("IsReviewOnly(%q) = %v, want %v", tt.category, got, tt.want)
			}
		})
	}
}

func TestReviewOnlyCategoriesAreValid(t *testing.T) {
	for _, cat := range ReviewOnlyCategories {
		if !ValidCategory(cat) {
			t.Errorf("ReviewOnlyCategories contains invalid category %q", cat)
		}
	}
}
