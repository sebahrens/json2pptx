package patterns

import (
	"encoding/json"
	"testing"
)

// TestFindingMetaExampleAfterValidates checks that every describe_finding
// ExampleAfter snippet that is a named-pattern JSON object decodes and
// validates against the registered pattern, so the remediation an agent
// copies does not itself fail validation (go-slide-creator-csclk.108).
func TestFindingMetaExampleAfterValidates(t *testing.T) {
	for _, code := range AllFindingMetaCodes() {
		meta, _ := GetFindingMeta(code)
		var doc struct {
			Pattern *struct {
				Name      string          `json:"name"`
				Values    json.RawMessage `json:"values"`
				Overrides json.RawMessage `json:"overrides"`
			} `json:"pattern"`
		}
		if json.Unmarshal([]byte(meta.ExampleAfter), &doc) != nil || doc.Pattern == nil || doc.Pattern.Name == "" {
			continue // prose or elided (non-JSON) example
		}
		pat, ok := Default().Get(doc.Pattern.Name)
		if !ok {
			t.Errorf("%s: ExampleAfter names unknown pattern %q", code, doc.Pattern.Name)
			continue
		}
		if errs := InspectPatternInput(pat, doc.Pattern.Values, doc.Pattern.Overrides, nil); len(errs) > 0 {
			t.Errorf("%s: ExampleAfter for %s has input findings: %v", code, doc.Pattern.Name, errs[0])
			continue
		}
		vals := pat.NewValues()
		if err := json.Unmarshal(doc.Pattern.Values, vals); err != nil {
			t.Errorf("%s: ExampleAfter values for %s do not decode: %v", code, doc.Pattern.Name, err)
			continue
		}
		if err := pat.Validate(vals, nil, nil); err != nil {
			t.Errorf("%s: ExampleAfter for %s fails validation: %v", code, doc.Pattern.Name, err)
		}
	}
}
