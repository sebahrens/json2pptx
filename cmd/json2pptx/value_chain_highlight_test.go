package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/svggen"
)

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

	for _, name := range testutil.AllTestTemplateNames() {
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

			base, baseMods := cellFill(t, grid.Rows[0].Cells[0].Shape.Fill)
			highlight, highlightMods := cellFill(t, grid.Rows[0].Cells[2].Shape.Fill)
			baseColor, ok := themeHex(base, theme.Colors)
			if !ok {
				t.Fatalf("theme has no colour %q", base)
			}
			highlightColor, ok := themeHex(highlight, theme.Colors)
			if !ok {
				t.Fatalf("theme has no colour %q", highlight)
			}
			// Steps are neutral tints (dk1 at ~16%) over the white slide; judge
			// what the viewer sees, not the untinted base colour.
			white := svggen.MustParseColor("#FFFFFF")
			baseColor = patterns.EffectiveColorMods(baseColor, baseMods, white)
			highlightColor = patterns.EffectiveColorMods(highlightColor, highlightMods, white)

			// A saturated highlight against a light neutral step needs 2:1 to
			// stand out (LOW_CONTRAST_HIGHLIGHT, go-slide-creator-8xsj3).
			ratio := highlightColor.ContrastWith(baseColor)
			if ratio < 2.0 {
				t.Errorf("highlight %s (%s) on %s (%s) reads at %.2f:1; below 2:1 the highlighted step is not distinguishable",
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

// cellFill reads the fill colour name and its lumMod/lumOff modifiers.
func cellFill(t *testing.T, raw json.RawMessage) (string, patterns.ColorMods) {
	t.Helper()
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, patterns.ColorMods{}
	}
	var obj struct {
		Color  string `json:"color"`
		LumMod int    `json:"lumMod"`
		LumOff int    `json:"lumOff"`
		Tint   int    `json:"tint"`
		Shade  int    `json:"shade"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("cannot read fill %s: %v", raw, err)
	}
	return obj.Color, patterns.ColorMods{LumMod: obj.LumMod, LumOff: obj.LumOff, Tint: obj.Tint, Shade: obj.Shade}
}
