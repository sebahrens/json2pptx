package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Shared KPI types and helpers used by kpi-3up, kpi-4up, etc.
// ---------------------------------------------------------------------------

// KPICell is a single KPI cell: a big number and a short caption.
// Supports string shorthand: "Big | Small" unmarshals to {big:"Big", small:"Small"}.
type KPICell struct {
	Big   string `json:"big"`
	Small string `json:"small"`
	Icon  string `json:"icon,omitempty"` // Bundled icon name (e.g. "trending-up", "filled:users")
}

// UnmarshalJSON supports string shorthand "Big | Small" or object {big, small}.
func (c *KPICell) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("KPICell string must be \"Big | Small\", got %q", s)
		}
		c.Big = parts[0]
		c.Small = parts[1]
		return nil
	}
	type alias KPICell
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("KPICell must be string \"Big | Small\" or {big, small}: %w", err)
	}
	*c = KPICell(a)
	return nil
}

// KPIOverrides contains pattern-level overrides common to KPI patterns.
type KPIOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	BigSize        float64 `json:"big_size,omitempty"`
	SmallSize      float64 `json:"small_size,omitempty"`
}

// KPICellOverride is an alias for the shared CellOverride struct.
type KPICellOverride = CellOverride

// applyKPICellTextOverrides modifies KPI text JSON to apply per-cell overrides
// for emphasis, align, vertical_align, font_size, and color. It re-marshals the
// text object with the overridden values.
func applyKPICellTextOverrides(text json.RawMessage, ovr *KPICellOverride) json.RawMessage {
	if ovr == nil {
		return text
	}

	hasParagraphChange := ovr.Emphasis != "" || ovr.FontSize > 0 || ovr.Color != ""
	hasTopLevelChange := ovr.Align != "" || ovr.VerticalAlign != ""
	if !hasParagraphChange && !hasTopLevelChange {
		return text
	}

	var textObj map[string]json.RawMessage
	if err := json.Unmarshal(text, &textObj); err != nil {
		return text
	}

	if ovr.Align != "" {
		a, _ := json.Marshal(ovr.Align)
		textObj["align"] = a
	}
	if ovr.VerticalAlign != "" {
		va, _ := json.Marshal(ovr.VerticalAlign)
		textObj["vertical_align"] = va
	}

	if hasParagraphChange {
		applyKPIParagraphOverrides(textObj, ovr)
	}

	result, _ := json.Marshal(textObj)
	return result
}

// applyKPIParagraphOverrides applies emphasis, font_size, and color overrides
// to each paragraph in the text object.
func applyKPIParagraphOverrides(textObj map[string]json.RawMessage, ovr *KPICellOverride) {
	raw, ok := textObj["paragraphs"]
	if !ok {
		return
	}
	var paragraphs []map[string]any
	if err := json.Unmarshal(raw, &paragraphs); err != nil {
		return
	}
	for i := range paragraphs {
		applyEmphasis(paragraphs[i], ovr.Emphasis)
		if ovr.FontSize > 0 {
			paragraphs[i]["size"] = ovr.FontSize
		}
		if ovr.Color != "" {
			paragraphs[i]["color"] = ovr.Color
		}
	}
	p, _ := json.Marshal(paragraphs)
	textObj["paragraphs"] = p
}

// applyEmphasis sets bold/italic flags on a paragraph map based on the
// emphasis string ("bold", "italic", "bold-italic").
func applyEmphasis(para map[string]any, emphasis string) {
	switch emphasis {
	case "bold":
		para["bold"] = true
		delete(para, "italic")
	case "italic":
		para["italic"] = true
		delete(para, "bold")
	case "bold-italic":
		para["bold"] = true
		para["italic"] = true
	}
}

// resolveKPIAccent returns the accent color, honoring the deck-level accent strategy.
func resolveKPIAccent(ovr *KPIOverrides, ctx ExpandContext) string {
	if ovr == nil {
		return AccentForStrategy(ctx.AccentStrategy, ctx.SlideIndex, ctx.SectionIndex)
	}
	return ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
}

// resolveKPIBigSize returns the big-number font size, defaulting to 36pt.
func resolveKPIBigSize(ovr *KPIOverrides) float64 {
	if ovr == nil {
		return 36.0
	}
	return ResolveSize(ovr.BigSize, 36.0)
}

// resolveKPISmallSize returns the caption font size, defaulting to 14pt.
func resolveKPISmallSize(ovr *KPIOverrides) float64 {
	if ovr == nil {
		return 14.0
	}
	return ResolveSize(ovr.SmallSize, 14.0)
}

// validateKPICells validates a slice of KPI cells for a pattern with the given
// name and expected count. siblingHint is the name of the sibling pattern to
// suggest when the count is off by one (e.g. "kpi-4up" when validating kpi-3up).
func validateKPICells(patternName string, cells []KPICell, expectedCount int, siblingHint string, cellOverrides map[int]any) error {
	var errs []error

	// D4: exact count with swap suggestion
	if len(cells) != expectedCount {
		errs = append(errs, newValidationError(patternName, "values", ErrCodeCountMismatch,
			fmt.Sprintf("%s: values must contain exactly %d cells, got %d", patternName, expectedCount, len(cells)),
			fmt.Sprintf("provide exactly %d cells in values", expectedCount)))

		// Reverse-recommend: suggest alternative patterns that accept the actual cell count.
		if swaps := SuggestSwap(Default(), patternName, len(cells), true); len(swaps) > 0 {
			errs = append(errs, ErrWrongPatternFor(patternName, len(cells), swaps))
		}
	}

	// Per-cell validation
	for i, cell := range cells {
		bigPath := fmt.Sprintf("values[%d].big", i)
		if cell.Big == "" {
			errs = append(errs, errRequired(patternName, bigPath))
		} else if len(cell.Big) > 8 {
			errs = append(errs, errMaxLength(patternName, bigPath, 8, len(cell.Big)))
		}
		smallPath := fmt.Sprintf("values[%d].small", i)
		if cell.Small == "" {
			errs = append(errs, errRequired(patternName, smallPath))
		} else if len(cell.Small) > 40 {
			errs = append(errs, errMaxLength(patternName, smallPath, 40, len(cell.Small)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(patternName, cellOverrides, expectedCount, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// kpiCellSchema returns the JSON Schema for a single KPI cell.
// Accepts either the object form {big, small, icon?} or the shorthand string "Big | Small".
func kpiCellSchema() *Schema {
	return OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Big | Small\" (e.g. \"$4.2M | ARR\")"),
		ObjectSchema(
			map[string]*Schema{
				"big":   StringSchema(8).WithDescription("The big number (e.g. \"$4.2M\")"),
				"small": StringSchema(40).WithDescription("Short caption (e.g. \"ARR\")"),
				"icon":  StringSchema(60).WithDescription("Bundled icon name (e.g. \"trending-up\", \"filled:users\")"),
			},
			[]string{"big", "small"},
		).WithAdditionalProperties(false),
	).WithDescription("KPI cell: string \"Big | Small\" or {big, small, icon?}")
}


// kpiOverridesSchema returns the JSON Schema for KPI pattern-level overrides.
func kpiOverridesSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"big_size":        NumberSchema(6, 120).WithDescription("Font size for big number in points"),
			"small_size":      NumberSchema(6, 120).WithDescription("Font size for small caption in points"),
		},
		nil,
	).WithAdditionalProperties(false)
}

// buildKPITextContent creates a JSON text object with paragraphs for a KPI cell.
func buildKPITextContent(big string, bigSize float64, small string, smallSize float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: big, Size: bigSize, Bold: true, Color: "lt1", Align: "ctr"},
			{Content: small, Size: smallSize, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
