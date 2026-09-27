package main

import (
	"encoding/json"
	"testing"
)

// go-slide-creator-csclk.4: control characters in theme_override font names
// cannot be written into theme1.xml, so validation rejects them.
func TestCheckThemeOverrideFonts_RejectsControlChars(t *testing.T) {
	errs := checkThemeOverrideFonts(&ThemeInput{TitleFont: "Evil\u0001Font", BodyFont: "B\u0008F"})
	if len(errs) != 2 {
		t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
	}
	if errs := checkThemeOverrideFonts(&ThemeInput{TitleFont: "Georgia", BodyFont: "Fira Sans & Co"}); len(errs) != 0 {
		t.Fatalf("valid fonts rejected: %v", errs)
	}
}

// go-slide-creator-csclk.5: a grid-cell table row wider than its headers is
// refused by generate, so validate must reject it too, nested grids included.
func TestGridTableRowWidthDiagnostics_Nested(t *testing.T) {
	var grid ShapeGridInput
	raw := `{"rows":[{"cells":[
		{"table":{"headers":["A","B","C"],"rows":[["1"],["1","2","3","4"]]}},
		{"grid":{"rows":[{"cells":[{"table":{"headers":["A"],"rows":[["1","2"]]}}]}]}}
	]}]}`
	if err := json.Unmarshal([]byte(raw), &grid); err != nil {
		t.Fatal(err)
	}
	ds := gridTableRowWidthDiagnostics(&grid, "/slides/0/shape_grid", 0)
	want := map[string]bool{
		"/slides/0/shape_grid/rows/0/cells/0/table.rows":                     true,
		"/slides/0/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/table.rows": true,
	}
	if len(ds) != len(want) {
		t.Fatalf("got %d diagnostics, want %d: %+v", len(ds), len(want), ds)
	}
	for _, d := range ds {
		if !want[d.Path] {
			t.Errorf("unexpected path %q", d.Path)
		}
	}
}

// go-slide-creator-csclk.67: the documented patch envelope is not a deck, so
// its "base"/"operations" keys must not be reported as unknown.
func TestCheckInputUnknownKeys_PatchEnvelope(t *testing.T) {
	raw := `{"base":{"template":"midnight-blue","slides":[{"slide_type":"title"}]},"operations":[{"op":"remove","slide_index":0}]}`
	if ves := checkInputUnknownKeys(json.RawMessage(raw)); len(ves) != 0 {
		t.Fatalf("patch envelope reported unknown keys: %v", ves)
	}
	typo := `{"base":{"template":"midnight-blue","slidez":[]},"operations":[{"op":"remove","slide_index":0}]}`
	if ves := checkInputUnknownKeys(json.RawMessage(typo)); len(ves) == 0 {
		t.Fatal("unknown key inside the patch base was not reported")
	}
}

// go-slide-creator-csclk.94: a project_code that already says "Project" is
// not prefixed a second time.
func TestChromeProjectLabel(t *testing.T) {
	for in, want := range map[string]string{
		"Aurora":             "Project Aurora",
		"Project Lighthouse": "Project Lighthouse",
		"project x":          "project x",
		"Projection":         "Project Projection",
	} {
		if got := chromeProjectLabel(in); got != want {
			t.Errorf("chromeProjectLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
