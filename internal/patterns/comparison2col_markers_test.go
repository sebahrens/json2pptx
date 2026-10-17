package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestComparisonTickCrossMarkersPreserveInput(t *testing.T) {
	p, _ := Default().Get("comparison-2col")
	v := &Comparison2colValues{Rows: []Comparison2colRow{{Left: "Supported", Right: "Unsupported"}}}
	ovr := &Comparison2colOverrides{Markers: "tick-cross"}
	if err := p.Validate(v, ovr, nil); err != nil {
		t.Fatal(err)
	}
	grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(grid)
	if !strings.Contains(string(raw), "✓ Supported") || !strings.Contains(string(raw), "✗ Unsupported") {
		t.Fatalf("missing literal markers: %s", raw)
	}
	if v.Rows[0].Left != "Supported" || v.Rows[0].Right != "Unsupported" {
		t.Fatal("caller input mutated")
	}
	ovr.Markers = "unknown"
	if err := p.Validate(v, ovr, nil); err == nil {
		t.Fatal("invalid marker mode accepted")
	}
}
