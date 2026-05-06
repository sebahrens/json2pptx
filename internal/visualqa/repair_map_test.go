package visualqa

import "testing"

func TestSuggestedFixesForCategory(t *testing.T) {
	tests := []struct {
		category string
		wantLen  int
		wantFirst string
	}{
		{"text_overflow", 3, "reduce_cell_text"},
		{"contrast", 2, "replace_color"},
		{"alignment", 2, "reshape_grid"},
		{"spacing", 1, "reshape_grid"},
		{"font_size", 2, "reduce_text"},
		{"missing_content", 1, "provide_value"},
		{"image_quality", 0, ""},  // no mapping
		{"aspect_ratio", 0, ""},   // no mapping
		{"unknown_cat", 0, ""},    // unknown
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
