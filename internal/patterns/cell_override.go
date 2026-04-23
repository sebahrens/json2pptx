package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
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

// validateCellOverrideKeys validates cell_overrides keys against the D15
// whitelist, returning structured ValidationErrors. The hint parameter is
// appended to out-of-range error messages (e.g. index-to-name mappings).
func validateCellOverrideKeys(patternName string, cellOverrides map[int]any, totalCells int, hint string) error {
	var errs []error
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= totalCells {
			errs = append(errs, errCellOverrideOutOfRange(patternName, idx, totalCells-1, hint))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: cell_overrides[%d]: %w", patternName, idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("%s: cell_overrides[%d]: %w", patternName, idx, err))
			continue
		}
		for key := range keyMap {
			if !cellOverrideAllowed[key] {
				path := fmt.Sprintf("cell_overrides[%d]", idx)
				errs = append(errs, errUnknownKey(patternName, path, key, CellOverrideAllowedList()))
			}
		}
	}
	return errors.Join(errs...)
}
