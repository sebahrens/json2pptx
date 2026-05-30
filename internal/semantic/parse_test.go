package semantic

import (
	"os"
	"path/filepath"
	"testing"
)

// assertCleanBoardUpdate verifies the shared board_update fixture parsed into
// the expected DeckSpec, regardless of source format.
func assertCleanBoardUpdate(t *testing.T, spec *DeckSpec, ds Diagnostics) {
	t.Helper()
	if ds.HasErrors() {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if spec.Meta.Title != "Q2 Board Update" {
		t.Errorf("meta.title = %q, want %q", spec.Meta.Title, "Q2 Board Update")
	}
	if spec.Meta.Archetype != ArchetypeBoardUpdate {
		t.Errorf("meta.archetype = %q, want %q", spec.Meta.Archetype, ArchetypeBoardUpdate)
	}
	if got, want := len(spec.Slides), 5; got != want {
		t.Fatalf("len(slides) = %d, want %d", got, want)
	}
	wantKinds := []SlideKind{KindTitle, KindExecutiveSummary, KindKPISnapshot, KindChartInsight, KindClosing}
	for i, want := range wantKinds {
		if spec.Slides[i].Kind != want {
			t.Errorf("slides[%d].kind = %q, want %q", i, spec.Slides[i].Kind, want)
		}
	}
	// Body should carry payload fields but not the discriminator.
	if got := spec.Slides[0].String("subtitle"); got != "May 2026" {
		t.Errorf("slides[0].body.subtitle = %q, want %q", got, "May 2026")
	}
	if _, leaked := spec.Slides[0].Body["kind"]; leaked {
		t.Error("slides[0].body must not contain the kind discriminator")
	}
}

func TestParseYAMLFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "board_update.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ds := Parse("board_update.yaml", data)
	assertCleanBoardUpdate(t, spec, ds)
}

func TestParseJSONFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "board_update.json"))
	if err != nil {
		t.Fatal(err)
	}
	spec, ds := Parse("board_update.json", data)
	assertCleanBoardUpdate(t, spec, ds)
}

func TestParseUnknownKindIsDiagnostic(t *testing.T) {
	src := `
slides:
  - kind: title
    title: Hello
  - kind: bogus_kind
    title: Oops
`
	spec, ds := ParseYAML([]byte(src))
	if spec == nil {
		t.Fatal("expected best-effort spec even with diagnostics")
	}
	if !ds.HasErrors() {
		t.Fatal("expected an error diagnostic for the unknown kind")
	}

	var found *Diagnostic
	for i := range ds {
		if ds[i].Code == CodeUnknownKind {
			found = &ds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected a %s diagnostic, got %v", CodeUnknownKind, ds)
	}
	if found.Path != "slides[1].kind" {
		t.Errorf("diagnostic path = %q, want %q", found.Path, "slides[1].kind")
	}
	if found.Severity != SeverityError {
		t.Errorf("severity = %q, want %q", found.Severity, SeverityError)
	}
	// The unknown kind is still recorded on the slide for inspection.
	if spec.Slides[1].Kind != SlideKind("bogus_kind") {
		t.Errorf("slides[1].kind = %q, want %q", spec.Slides[1].Kind, "bogus_kind")
	}
}

func TestParseMissingKindIsDiagnostic(t *testing.T) {
	src := `
slides:
  - title: No kind here
`
	_, ds := ParseYAML([]byte(src))
	if !hasCodeAtPath(ds, CodeMissingKind, "slides[0]") {
		t.Fatalf("expected %s at slides[0], got %v", CodeMissingKind, ds)
	}
}

func TestParseUnknownArchetypeIsDiagnostic(t *testing.T) {
	src := `
meta:
  title: Deck
  archetype: not_a_real_archetype
slides:
  - kind: title
    title: Hello
`
	_, ds := ParseYAML([]byte(src))
	if !hasCodeAtPath(ds, CodeUnknownArchetype, "meta.archetype") {
		t.Fatalf("expected %s at meta.archetype, got %v", CodeUnknownArchetype, ds)
	}
}

func TestParseInvalidYAMLIsParseError(t *testing.T) {
	// Unterminated flow mapping — malformed YAML.
	_, ds := ParseYAML([]byte("slides: [ {kind: title"))
	if !ds.HasErrors() {
		t.Fatal("expected a parse error diagnostic")
	}
	if ds[0].Code != CodeParseError {
		t.Errorf("code = %q, want %q", ds[0].Code, CodeParseError)
	}
}

func TestParseInvalidJSONIsParseError(t *testing.T) {
	_, ds := ParseJSON([]byte("{not json"))
	if !ds.HasErrors() || ds[0].Code != CodeParseError {
		t.Fatalf("expected %s, got %v", CodeParseError, ds)
	}
}

func hasCodeAtPath(ds Diagnostics, code, path string) bool {
	for _, d := range ds {
		if d.Code == code && d.Path == path {
			return true
		}
	}
	return false
}
