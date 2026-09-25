package patterns

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"testing"
)

// cellOverrideProbeValues lists, per D15 key, the values tried against a
// pattern's exemplar. An enum key passes when at least one of its values
// changes the output: the exemplar's cell may already carry one of them (a
// bold header is unchanged by "bold").
var cellOverrideProbeValues = map[string][]any{
	"accent_bar":     {true, false}, // false: horizontal-bar-with-callouts draws the bar by default
	"emphasis":       {"italic", "bold", "bold-italic"},
	"align":          {"r", "l", "ctr"},
	"vertical_align": {"b", "t", "ctr"},
	"font_size":      {41.0},
	"color":          {"accent6"},
}

// TestCellOverrideKeys_ChangeExpandOrAreRejected is the regression for
// go-slide-creator-s1uvj.36: the shared cellOverride schema advertised
// font_size / emphasis / align / vertical_align / color for every pattern and
// Validate accepted them, but most patterns only read accent_bar, so the text
// keys were silently dropped. For every registered pattern that accepts
// cell_overrides, each key its schema advertises must change the Expand
// output, and each D15 key it does not advertise must be rejected by Validate
// as unknown_key.
func TestCellOverrideKeys_ChangeExpandOrAreRejected(t *testing.T) {
	reg := Default()
	for _, pat := range reg.List() {
		if pat.NewCellOverride() == nil {
			continue
		}
		t.Run(pat.Name(), func(t *testing.T) {
			ex, ok := pat.(Exemplar)
			if !ok {
				t.Fatalf("pattern accepts cell_overrides but has no Exemplar")
			}
			exemplarJSON, err := json.Marshal(ex.ExemplarValues())
			if err != nil {
				t.Fatalf("marshal exemplar: %v", err)
			}
			decodeValues := func() any {
				v := pat.NewValues()
				if err := json.Unmarshal(exemplarJSON, v); err != nil {
					t.Fatalf("decode exemplar: %v", err)
				}
				return v
			}
			expand := func(cellOverrides map[int]any) []byte {
				grid, err := pat.Expand(ExpandContext{}, decodeValues(), nil, cellOverrides)
				if err != nil {
					t.Fatalf("expand: %v", err)
				}
				out, err := json.Marshal(grid)
				if err != nil {
					t.Fatalf("marshal grid: %v", err)
				}
				return out
			}
			baseline := expand(nil)

			advertised := map[string]bool{}
			if sch := cellOverrideSchema(pat.Schema()); sch != nil {
				for _, k := range sch.PropertyNames() {
					advertised[k] = true
				}
			}

			keys := make([]string, 0, len(cellOverrideProbeValues))
			for k := range cellOverrideProbeValues {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, key := range keys {
				changed, rejected := false, false
			probe:
				// Index 1 too: the exemplar's first cell may already carry
				// the effect (metric-list highlights item 0, so its accent
				// bar is drawn with or without accent_bar).
				for idx := 0; idx <= 1; idx++ {
					for _, v := range cellOverrideProbeValues[key] {
						raw, _ := json.Marshal(map[string]any{key: v})
						co := pat.NewCellOverride()
						if err := json.Unmarshal(raw, co); err != nil {
							t.Fatalf("decode cell override %s: %v", raw, err)
						}
						cos := map[int]any{idx: co}
						if err := pat.Validate(decodeValues(), nil, cos); err != nil {
							var ve *ValidationError
							if errors.As(err, &ve) && ve.Code == ErrCodeUnknownKey {
								rejected = true
								break probe
							}
							if idx > 0 && errors.As(err, &ve) && ve.Code == ErrCodeOutOfRange {
								break probe
							}
							t.Fatalf("Validate(cell_overrides[%d]=%s): %v", idx, raw, err)
						}
						if !bytes.Equal(expand(cos), baseline) {
							changed = true
							break probe
						}
					}
				}
				switch {
				case advertised[key] && rejected:
					t.Errorf("schema advertises %q but Validate rejects it", key)
				case advertised[key] && !changed:
					t.Errorf("schema advertises %q but cell_overrides[0].%s leaves the Expand output unchanged", key, key)
				case !advertised[key] && !rejected:
					t.Errorf("schema does not advertise %q but Validate accepts it", key)
				}
			}

			// The value itself must reach the output, not merely perturb it
			// (a later layout pass could otherwise overwrite the override).
			// A value passes when the output carries more "key":value pairs
			// than the baseline does.
			for _, key := range []string{"font_size", "color", "align", "vertical_align"} {
				if !advertised[key] {
					continue
				}
				reached := false
				for _, v := range cellOverrideProbeValues[key] {
					raw, _ := json.Marshal(map[string]any{key: v})
					co := pat.NewCellOverride()
					if err := json.Unmarshal(raw, co); err != nil {
						t.Fatalf("decode cell override %s: %v", raw, err)
					}
					field := key
					if key == "font_size" {
						field = "size"
					}
					marker, _ := json.Marshal(map[string]any{field: v})
					marker = bytes.Trim(marker, "{}")
					if bytes.Count(expand(map[int]any{0: co}), marker) > bytes.Count(baseline, marker) {
						reached = true
						break
					}
				}
				if !reached {
					t.Errorf("no cell_overrides[0].%s value reached the cell text (a later layout pass may overwrite it)", key)
				}
			}
		})
	}
}
