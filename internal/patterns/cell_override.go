package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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

// hasTextOverride reports whether the override carries any of the D15 text
// keys (emphasis, align, vertical_align, font_size, color).
func (o *CellOverride) hasTextOverride() bool {
	return o != nil && (o.Emphasis != "" || o.Align != "" || o.VerticalAlign != "" || o.FontSize > 0 || o.Color != "")
}

// applyCellTextOverride applies the D15 per-cell text keys (font_size,
// emphasis, align, vertical_align, color) to the primary text of a pattern
// cell: a shape's text, a composite cell's text shape, or an image cell's
// overlay label. Cells without text are left unchanged. Every pattern that
// accepts cell_overrides routes its target cell through this helper so the
// text keys the shared cellOverride schema advertises are honoured instead of
// silently dropped (go-slide-creator-s1uvj.36).
func applyCellTextOverride(cell *jsonschema.GridCellInput, ovr *CellOverride) {
	if cell == nil || !ovr.hasTextOverride() {
		return
	}
	switch {
	case cell.Shape != nil:
		cell.Shape.Text = applyCellTextOverrideToText(cell.Shape.Text, ovr)
	case cell.Composite != nil && cell.Composite.Text != nil:
		cell.Composite.Text.Text = applyCellTextOverrideToText(cell.Composite.Text.Text, ovr)
	case cell.Image != nil && cell.Image.Text != nil:
		applyCellTextOverrideToImageText(cell.Image.Text, ovr)
	}
}

// applyCellTextOverrideToImageText applies the D15 text keys to an image
// cell's overlay label.
func applyCellTextOverrideToImageText(t *jsonschema.GridImageTextInput, ovr *CellOverride) {
	if ovr.FontSize > 0 {
		t.Size = ovr.FontSize
	}
	if ovr.Emphasis != "" {
		// The image label carries no italic flag; bold is the only emphasis
		// it can express, so "italic" clears bold as it does for shapes.
		t.Bold = ovr.Emphasis == "bold" || ovr.Emphasis == "bold-italic"
	}
	if ovr.Color != "" {
		t.Color = ovr.Color
	}
	if ovr.Align != "" {
		t.Align = ovr.Align
	}
	if ovr.VerticalAlign != "" {
		t.VerticalAlign = ovr.VerticalAlign
	}
}

// applyCellTextOverrideToText rewrites a shape_grid text payload (string
// shorthand, {content} object or {paragraphs} object) with the D15 per-cell
// text keys. font_size, emphasis and color apply to every paragraph; align
// sets the text-level default and every paragraph's own align (a paragraph's
// align wins over the default, so setting only the default would be a no-op);
// vertical_align sets the anchor. The payload is returned unchanged when it
// carries no text or the override has no text keys.
func applyCellTextOverrideToText(text json.RawMessage, ovr *CellOverride) json.RawMessage {
	if len(text) == 0 || !ovr.hasTextOverride() {
		return text
	}

	var textObj map[string]json.RawMessage
	var s string
	if err := json.Unmarshal(text, &s); err == nil {
		textObj = map[string]json.RawMessage{"content": marshalRaw(s)}
	} else if err := json.Unmarshal(text, &textObj); err != nil || textObj == nil {
		return text
	}

	if ovr.Align != "" {
		textObj["align"] = marshalRaw(ovr.Align)
	}
	if ovr.VerticalAlign != "" {
		textObj["vertical_align"] = marshalRaw(ovr.VerticalAlign)
	}

	if raw, ok := textObj["paragraphs"]; ok {
		var paragraphs []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &paragraphs); err != nil {
			return text
		}
		for _, para := range paragraphs {
			if para == nil {
				continue
			}
			applyCellRunOverride(para, ovr)
			if ovr.Align != "" {
				para["align"] = marshalRaw(ovr.Align)
			}
		}
		textObj["paragraphs"] = marshalRaw(paragraphs)
	} else {
		applyCellRunOverride(textObj, ovr)
	}

	result, err := json.Marshal(textObj)
	if err != nil {
		return text
	}
	return result
}

// applyCellRunOverride applies font_size, emphasis and color to one run-level
// object (a paragraph, or a {content} text object).
func applyCellRunOverride(obj map[string]json.RawMessage, ovr *CellOverride) {
	if ovr.FontSize > 0 {
		obj["size"] = marshalRaw(ovr.FontSize)
	}
	if ovr.Color != "" {
		obj["color"] = marshalRaw(ovr.Color)
	}
	switch ovr.Emphasis {
	case "bold":
		obj["bold"] = marshalRaw(true)
		delete(obj, "italic")
	case "italic":
		obj["italic"] = marshalRaw(true)
		delete(obj, "bold")
	case "bold-italic":
		obj["bold"] = marshalRaw(true)
		obj["italic"] = marshalRaw(true)
	}
}

// marshalRaw marshals a value that cannot fail to encode (strings, numbers,
// bools, and maps/slices of json.RawMessage decoded from valid JSON).
func marshalRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
