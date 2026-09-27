package semantic

import (
	"encoding/json"
	"strings"
	"testing"
)

func hasCodeAt(t *testing.T, spec *DeckSpec, code, path string) bool {
	t.Helper()
	for _, d := range Validate(spec, StrictnessWarn) {
		if string(d.Code) == code && d.Path == path {
			return true
		}
	}
	return false
}

// go-slide-creator-csclk.43: numbers in nested text positions render as text.
func TestNestedNumericValuesCompile(t *testing.T) {
	spec, ds := ParseJSON([]byte(`{"meta":{"title":"T"},"slides":[{"kind":"kpi_snapshot","title":"K","takeaway":"Numbers carry the story","kpis":[{"label":"Revenue","value":48},{"label":"Margin","value":118}]}]}`))
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	in, _, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(in.Slides[0])
	if !strings.Contains(string(out), `"big":"48"`) || !strings.Contains(string(out), `"big":"118"`) {
		t.Fatalf("numeric KPI values dropped: %s", out)
	}
}

// go-slide-creator-csclk.44: YAML dates and zero-padded / hex ints keep text.
func TestYAMLScalarSourceTextKept(t *testing.T) {
	spec, ds := ParseYAML([]byte("meta:\n  title: T\nslides:\n  - kind: table\n    title: X\n    columns: [A, B]\n    rows:\n      - [2024-03-01, 007]\n      - [0x1F, 5]\n"))
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	rows := spec.Slides[0].Body["rows"].([]any)
	r0, r1 := rows[0].([]any), rows[1].([]any)
	if r0[0] != "2024-03-01" || r0[1] != "007" || r1[0] != "0x1F" {
		t.Fatalf("scalar text lost: %#v %#v", r0, r1)
	}
	if _, ok := r1[1].(int); !ok {
		t.Fatalf("plain int should stay numeric, got %T", r1[1])
	}
}

// go-slide-creator-csclk.46: invalid meta enums are rejected.
func TestMetaEnumsValidated(t *testing.T) {
	spec, _ := ParseJSON([]byte(`{"meta":{"title":"T","accent_strategy":"rainbow","viewing_mode":"cinema","design_mode":"wild"},"slides":[{"kind":"section","title":"S"}]}`))
	for _, p := range []string{"meta.accent_strategy", "meta.viewing_mode", "meta.design_mode"} {
		if !hasCodeAt(t, spec, "SEMANTIC_REQUIRED", p) {
			t.Errorf("no SEMANTIC_REQUIRED at %s", p)
		}
	}
}

// go-slide-creator-csclk.47: a contradicting bridge total is flagged.
func TestBridgeTotalMismatch(t *testing.T) {
	spec, _ := ParseJSON([]byte(`{"meta":{"title":"T"},"slides":[{"kind":"bridge","title":"B","takeaway":"The walk explains the change","columns":[{"label":"Start","type":"total","value":100},{"label":"D","type":"delta","value":-10},{"label":"End","type":"total","value":999}]}]}`))
	if !hasCodeAt(t, spec, "SEMANTIC_BRIDGE_TOTAL_MISMATCH", "slides[0].columns[2].value") {
		t.Fatal("bridge total mismatch not flagged")
	}
	ok, _ := ParseJSON([]byte(`{"meta":{"title":"T"},"slides":[{"kind":"bridge","title":"B","takeaway":"The walk explains the change","columns":[{"label":"Start","type":"total","value":100},{"label":"D","type":"delta","value":-10},{"label":"End","type":"total","value":90}]}]}`))
	if hasCodeAt(t, ok, "SEMANTIC_BRIDGE_TOTAL_MISMATCH", "slides[0].columns[2].value") {
		t.Fatal("consistent bridge flagged")
	}
}

// go-slide-creator-csclk.45: structure-mode locators use structure paths.
func TestStructureModeLocators(t *testing.T) {
	spec, ds := ParseJSON([]byte(`{"meta":{"title":"T"},"structure":{"cover":{"kind":"title","title":"T"},"auto_agenda":true,"sections":[{"title":"A","slides":[{"kind":"stat","title":"a1","value":"42%","label":"share","takeaway":"Share grew fast"}]},{"title":"B","slides":[{"kind":"stat","title":"b1","value":"7x","label":"growth","takeaway":"Growth was strong"}]}]}}`))
	if ds.HasErrors() {
		t.Fatal(ds)
	}
	_, res, err := Compile(spec, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ir := Normalize(spec)
	for i := range ir.Slides {
		if p := ir.slidePath(i); strings.HasPrefix(p, "slides[") {
			t.Errorf("slide %d locator %q is not a structure path", i, p)
		}
	}
	if got := ir.slideListPath(); got != "structure" {
		t.Errorf("slideListPath = %q, want structure", got)
	}
	mf := MapFinding(res.SourceMap, RawFinding{Code: "SLIDE_UNDERUSED", RawPath: "slides[3]"})
	if mf.SemanticPath != "structure.sections[0].slides[0]" {
		t.Errorf("SLIDE_UNDERUSED at slides[3] mapped to %q", mf.SemanticPath)
	}
}
