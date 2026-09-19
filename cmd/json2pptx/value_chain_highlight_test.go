package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
)

// bundledTemplateNames are the four templates shipped with the engine.
var bundledTemplateNames = []string{"forest-green", "midnight-blue", "modern-template", "warm-coral"}

// TestValueChainHighlightOnRealTemplates runs the highlight rule against the
// actual theme files rather than a fixture, so a template whose palette moves
// (or a new bundled template) is caught here rather than shipping a value chain
// whose one semantic signal is invisible (go-slide-creator-ah5s).
func TestValueChainHighlightOnRealTemplates(t *testing.T) {
	reg := patterns.Default()
	pat, ok := reg.Get("value-chain")
	if !ok {
		t.Fatal("value-chain is not registered")
	}

	for _, name := range bundledTemplateNames {
		t.Run(name, func(t *testing.T) {
			reader, err := template.OpenTemplate("../../templates/" + name + ".pptx")
			if err != nil {
				t.Fatalf("open template: %v", err)
			}
			defer func() { _ = reader.Close() }()
			theme := template.ParseTheme(reader)

			values := pat.NewValues()
			decodePatternValues(t, values, `{"steps":[
				{"label":"Extraction","description":"Ore out of the ground."},
				{"label":"Processing","description":"Concentrate and refine."},
				{"label":"Manufacturing","description":"Converting inputs into finished goods.","highlight":true},
				{"label":"Distribution","description":"To the customer."}]}`)

			grid, err := pat.Expand(patterns.ExpandContext{Theme: theme}, values, nil, nil)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}

			base := cellFillColor(t, grid.Rows[0].Cells[0].Shape.Fill)
			highlight := cellFillColor(t, grid.Rows[0].Cells[2].Shape.Fill)
			baseColor, ok := themeHex(base, theme.Colors)
			if !ok {
				t.Fatalf("theme has no colour %q", base)
			}
			highlightColor, ok := themeHex(highlight, theme.Colors)
			if !ok {
				t.Fatalf("theme has no colour %q", highlight)
			}

			ratio := highlightColor.ContrastWith(baseColor)
			if ratio < 3.0 {
				t.Errorf("highlight %s (%s) on %s (%s) reads at %.2f:1; below 3:1 the highlighted step is not distinguishable",
					highlight, highlightColor.Hex(), base, baseColor.Hex(), ratio)
			}
			t.Logf("%s: %s %s on %s %s = %.2f:1", name, highlight, highlightColor.Hex(), base, baseColor.Hex(), ratio)
		})
	}
}

// decodePatternValues fills a pattern's typed values struct from JSON.
func decodePatternValues(t *testing.T, target any, raw string) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), target); err != nil {
		t.Fatalf("decode pattern values: %v", err)
	}
}

// cellFillColor reads the fill colour name out of an expanded grid cell.
func cellFillColor(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("cannot read fill %s: %v", raw, err)
	}
	return obj.Color
}
