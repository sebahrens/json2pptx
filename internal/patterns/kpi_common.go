package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// Shared KPI types and helpers used by kpi-3up, kpi-4up, etc.
// ---------------------------------------------------------------------------

// KPICell is a single KPI cell: a big number and a short caption.
type KPICell struct {
	Big   string `json:"big"`
	Small string `json:"small"`
}

// KPIOverrides contains pattern-level overrides common to KPI patterns.
type KPIOverrides struct {
	Accent    string  `json:"accent,omitempty"`
	BigSize   float64 `json:"big_size,omitempty"`
	SmallSize float64 `json:"small_size,omitempty"`
}

// KPICellOverride contains per-cell overrides for KPI patterns (D15 whitelist).
type KPICellOverride struct {
	AccentBar     bool    `json:"accent_bar,omitempty"`
	Emphasis      string  `json:"emphasis,omitempty"`
	Align         string  `json:"align,omitempty"`
	VerticalAlign string  `json:"vertical_align,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	Color         string  `json:"color,omitempty"`
}

// kpiCellOverrideAllowed is the whitelist of per-cell override keys (D15).
var kpiCellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

// resolveKPIAccent returns the accent color, defaulting to accent1.
func resolveKPIAccent(ovr *KPIOverrides) string {
	if ovr != nil && ovr.Accent != "" {
		return ovr.Accent
	}
	return "accent1"
}

// resolveKPIBigSize returns the big-number font size, defaulting to 36pt.
func resolveKPIBigSize(ovr *KPIOverrides) float64 {
	if ovr != nil && ovr.BigSize > 0 {
		return ovr.BigSize
	}
	return 36.0
}

// resolveKPISmallSize returns the caption font size, defaulting to 14pt.
func resolveKPISmallSize(ovr *KPIOverrides) float64 {
	if ovr != nil && ovr.SmallSize > 0 {
		return ovr.SmallSize
	}
	return 14.0
}

// validateKPICells validates a slice of KPI cells for a pattern with the given
// name and expected count. siblingHint is the name of the sibling pattern to
// suggest when the count is off by one (e.g. "kpi-4up" when validating kpi-3up).
func validateKPICells(patternName string, cells []KPICell, expectedCount int, siblingHint string, cellOverrides map[int]any) error {
	var errs []error

	// D4: exact count with sibling hint
	if len(cells) != expectedCount {
		hint := ""
		if siblingHint != "" && (len(cells) == expectedCount+1 || len(cells) == expectedCount-1) {
			hint = fmt.Sprintf(" (hint: use pattern %s for %d KPIs)", siblingHint, len(cells))
		}
		errs = append(errs, fmt.Errorf("%s: values must contain exactly %d cells, got %d%s", patternName, expectedCount, len(cells), hint))
	}

	// Per-cell validation
	for i, cell := range cells {
		if cell.Big == "" {
			errs = append(errs, fmt.Errorf("%s: values[%d].big is required", patternName, i))
		} else if len(cell.Big) > 8 {
			errs = append(errs, fmt.Errorf("%s: values[%d].big exceeds maxLength 8 (%d chars)", patternName, i, len(cell.Big)))
		}
		if cell.Small == "" {
			errs = append(errs, fmt.Errorf("%s: values[%d].small is required", patternName, i))
		} else if len(cell.Small) > 40 {
			errs = append(errs, fmt.Errorf("%s: values[%d].small exceeds maxLength 40 (%d chars)", patternName, i, len(cell.Small)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= expectedCount {
			errs = append(errs, fmt.Errorf("%s: cell_overrides key %d out of range [0,%d]", patternName, idx, expectedCount-1))
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
			if !kpiCellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("%s: cell_overrides[%d] contains unknown key %q", patternName, idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

// kpiCellSchema returns the JSON Schema for a single KPI cell.
func kpiCellSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"big":   StringSchema(8).WithDescription("The big number (e.g. \"$4.2M\")"),
			"small": StringSchema(40).WithDescription("Short caption (e.g. \"ARR\")"),
		},
		[]string{"big", "small"},
	).WithAdditionalProperties(false)
}

// kpiCellOverrideSchema returns the JSON Schema for a KPI cell override.
func kpiCellOverrideSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"accent_bar":     BooleanSchema().WithDescription("Show accent bar decoration"),
			"emphasis":       EnumSchema("bold", "italic", "bold-italic").WithDescription("Text emphasis"),
			"align":          EnumSchema("l", "ctr", "r").WithDescription("Horizontal alignment"),
			"vertical_align": EnumSchema("t", "ctr", "b").WithDescription("Vertical alignment"),
			"font_size":      NumberSchema(6, 120).WithDescription("Font size in points"),
			"color":          StringSchema(0).WithDescription("Text color (scheme ref, e.g. \"dk1\")"),
		},
		nil,
	).WithAdditionalProperties(false)
}

// kpiOverridesSchema returns the JSON Schema for KPI pattern-level overrides.
func kpiOverridesSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"accent":     StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"big_size":   NumberSchema(6, 120).WithDescription("Font size for big number in points"),
			"small_size": NumberSchema(6, 120).WithDescription("Font size for small caption in points"),
		},
		nil,
	).WithAdditionalProperties(false)
}
