package patterns

import (
	"sort"
	"strings"
)

// CellOverride contains per-cell overrides shared across all patterns (D15 whitelist).
// Every pattern uses the same 6-field struct for cell-level customization.
type CellOverride struct {
	AccentBar     bool    `json:"accent_bar,omitempty"`
	Emphasis      string  `json:"emphasis,omitempty"`
	Align         string  `json:"align,omitempty"`
	VerticalAlign string  `json:"vertical_align,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	Color         string  `json:"color,omitempty"`
}

// cellOverrideAllowed is the shared whitelist of per-cell override keys (D15).
var cellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

// CellOverrideAllowedList returns a sorted, comma-separated string of
// the allowed per-cell override keys for use in error messages.
func CellOverrideAllowedList() string {
	keys := make([]string, 0, len(cellOverrideAllowed))
	for k := range cellOverrideAllowed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
