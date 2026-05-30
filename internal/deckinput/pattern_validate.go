package deckinput

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// ValidatePattern runs the raw correctness gate a named pattern applies to its
// inputs — the same gate the renderer enforces inside expandPattern, minus the
// expansion — so a caller can predict whether a PatternInput will survive
// rendering without building a ShapeGridInput or needing any template/theme
// context. It looks p.Name up in reg, unmarshals the typed values, overrides,
// and per-cell overrides, and calls the pattern's own Validate.
//
// It returns nil when the inputs are valid; the pattern's errors.Join of
// *patterns.ValidationError when Validate rejects them; or a single error when
// the pattern name is unknown or a payload cannot be decoded into the pattern's
// typed shape. Callers that want structured diagnostics can split the returned
// error with diagnostics.FromJoinedError.
func ValidatePattern(p *PatternInput, reg *patterns.Registry) error {
	if p == nil {
		return nil
	}
	pat, ok := reg.Get(p.Name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", p.Name)
		if suggestion, ok := reg.Suggest(p.Name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return fmt.Errorf("%s", msg)
	}

	values := pat.NewValues()
	if err := json.Unmarshal(p.Values, values); err != nil {
		return fmt.Errorf("pattern %q: invalid values: %w", p.Name, err)
	}

	var overrides any
	if len(p.Overrides) > 0 {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal(p.Overrides, overrides); err != nil {
				return fmt.Errorf("pattern %q: invalid overrides: %w", p.Name, err)
			}
		}
	}

	var cellOverrides map[int]any
	if len(p.CellOverrides) > 0 {
		cellOverrides = make(map[int]any, len(p.CellOverrides))
		for key, raw := range p.CellOverrides {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return fmt.Errorf("pattern %q: cell_overrides key %q is not an integer", p.Name, key)
			}
			co := pat.NewCellOverride()
			if co == nil {
				return fmt.Errorf("pattern %q: does not support cell_overrides", p.Name)
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return fmt.Errorf("pattern %q: invalid cell_overrides[%d]: %w", p.Name, idx, err)
			}
			cellOverrides[idx] = co
		}
	}

	return pat.Validate(values, overrides, cellOverrides)
}
