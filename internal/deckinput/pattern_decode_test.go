package deckinput

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// TestDecodePatternFindings pins the one reading of a pattern block
// (go-slide-creator-iqknz): each fault is a finding with its own code at a
// path relative to the block, whichever surface asked.
func TestDecodePatternFindings(t *testing.T) {
	const kpis = `[{"big": "1", "small": "a"}, {"big": "2", "small": "b"}, {"big": "3", "small": "c"}]`
	cases := []struct {
		name  string
		block string
		want  []string // "CODE @ path", sorted; nil = accepted
	}{
		{"accepted", `{"name": "kpi-3up", "values": ` + kpis + `}`, nil},
		{"values of the wrong shape", `{"name": "kpi-3up", "values": "nope"}`, []string{"invalid_shape @ values"}},
		{"a value field of the wrong type", `{"name": "kpi-3up", "values": [{"big": 1, "small": "a"}, {"big": "2", "small": "b"}, {"big": "3", "small": "c"}]}`,
			[]string{"invalid_shape @ values[0].big"}},
		{"unknown pattern", `{"name": "kpi-thirty-up", "values": []}`, []string{"UNKNOWN_PATTERN @ name"}},
		{"too few values", `{"name": "kpi-3up", "values": [{"big": "1", "small": "a"}]}`,
			[]string{"count_mismatch @ values", "wrong_pattern @ name"}},
		{"an override the pattern does not have", `{"name": "kpi-3up", "overrides": {"no_such": 1}, "values": ` + kpis + `}`,
			[]string{"PATTERN_UNKNOWN_FIELD @ overrides.no_such"}},
		{"overrides of the wrong shape", `{"name": "kpi-3up", "overrides": "nope", "values": ` + kpis + `}`,
			[]string{"invalid_shape @ overrides"}},
		{"a cell override of the wrong shape", `{"name": "kpi-3up", "cell_overrides": {"0": "nope"}, "values": ` + kpis + `}`,
			[]string{"invalid_shape @ cell_overrides[0]"}},
		{"a cell_overrides key that is not an index", `{"name": "kpi-3up", "cell_overrides": {"x": {}}, "values": ` + kpis + `}`,
			[]string{"PATTERN_ERROR @ "}},
		{"vertical_align outside its set", `{"name": "kpi-3up", "vertical_align": "sideways", "values": ` + kpis + `}`,
			[]string{"PATTERN_ERROR @ "}},
		{"a callout the pattern does not support", `{"name": "kpi-3up", "callout": {"text": "So what"}, "values": ` + kpis + `}`,
			[]string{"callout_unsupported @ callout"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p PatternInput
			if err := json.Unmarshal([]byte(c.block), &p); err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodePattern(&p, patterns.Default())
			if verr := ValidatePattern(&p, patterns.Default()); (verr == nil) != (err == nil) {
				t.Errorf("ValidatePattern = %v, DecodePattern = %v: they must agree", verr, err)
			}
			if c.want == nil {
				if err != nil || decoded == nil || decoded.Pattern == nil || decoded.Values == nil {
					t.Fatalf("DecodePattern = %+v, %v; want the decoded block", decoded, err)
				}
				return
			}
			var pe *PatternError
			if !errors.As(err, &pe) {
				t.Fatalf("error = %v, want a *PatternError", err)
			}
			got := make([]string, len(pe.Findings))
			for i, f := range pe.Findings {
				got[i] = f.Code + " @ " + f.Path
			}
			sort.Strings(got)
			if strings.Join(got, "; ") != strings.Join(c.want, "; ") {
				t.Errorf("findings = %v, want %v", got, c.want)
			}
			if _, block := pe.IsBlockFault(); block != (c.want[0] == "PATTERN_ERROR @ ") {
				t.Errorf("IsBlockFault = %t for %v", block, got)
			}
		})
	}
}
