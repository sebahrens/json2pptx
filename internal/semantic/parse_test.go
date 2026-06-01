package semantic

import (
	"os"
	"path/filepath"
	"strings"
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

func TestParseUnknownTopLevelFieldDeckSuggestsMeta(t *testing.T) {
	// The common stale shape: a top-level "deck" object instead of "meta".
	src := `
deck:
  title: Bad Deck
slides:
  - kind: title
    title: Bad Deck
`
	_, ds := ParseYAML([]byte(src))
	if !hasCodeAtPath(ds, CodeUnknownField, "deck") {
		t.Fatalf("expected %s at deck, got %v", CodeUnknownField, ds)
	}
	var found *Diagnostic
	for i := range ds {
		if ds[i].Code == CodeUnknownField {
			found = &ds[i]
			break
		}
	}
	if found.Severity != SeverityError {
		t.Errorf("severity = %q, want %q", found.Severity, SeverityError)
	}
	if want := `did you mean "meta"?`; !strings.Contains(found.Message, want) {
		t.Errorf("message = %q, want it to contain %q", found.Message, want)
	}
}

func TestParseUnknownTopLevelFieldTypoSuggestsClosest(t *testing.T) {
	// A near-miss typo should be steered to the closest valid key.
	src := `
meta:
  title: Good Deck
slide:
  - kind: title
    title: Hi
`
	_, ds := ParseYAML([]byte(src))
	var found *Diagnostic
	for i := range ds {
		if ds[i].Code == CodeUnknownField && ds[i].Path == "slide" {
			found = &ds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected %s at slide, got %v", CodeUnknownField, ds)
	}
	if want := `did you mean "slides"?`; !strings.Contains(found.Message, want) {
		t.Errorf("message = %q, want it to contain %q", found.Message, want)
	}
}

func TestParseUnknownTopLevelFieldNoCloseMatchListsValidKeys(t *testing.T) {
	// An unrelated key gets a generic diagnostic listing the valid top-level keys.
	src := `
meta:
  title: Good Deck
slides:
  - kind: title
    title: Hi
xyzzy_foobar: 1
`
	_, ds := ParseYAML([]byte(src))
	var found *Diagnostic
	for i := range ds {
		if ds[i].Code == CodeUnknownField && ds[i].Path == "xyzzy_foobar" {
			found = &ds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected %s at xyzzy_foobar, got %v", CodeUnknownField, ds)
	}
	if want := "expected one of meta, slides"; !strings.Contains(found.Message, want) {
		t.Errorf("message = %q, want it to contain %q", found.Message, want)
	}
}

func TestParseKnownTopLevelFieldsAreClean(t *testing.T) {
	// meta + slides only — no unknown-field diagnostic.
	src := `
meta:
  title: Good Deck
slides:
  - kind: title
    title: Hi
`
	_, ds := ParseYAML([]byte(src))
	for _, d := range ds {
		if d.Code == CodeUnknownField {
			t.Fatalf("unexpected %s diagnostic: %v", CodeUnknownField, d)
		}
	}
}

func TestParseUnknownMetaFieldTyposSuggestClosest(t *testing.T) {
	// Per the schema's DeckMeta.additionalProperties:false, near-miss meta typos
	// must each surface a suggestion instead of being silently dropped.
	src := `
meta:
  tilte: Typo Title
  archetpe: qbr
slides:
  - kind: title
    title: Hi
`
	_, ds := ParseYAML([]byte(src))
	for _, tc := range []struct {
		path    string
		suggest string
	}{
		{"meta.tilte", `did you mean "title"?`},
		{"meta.archetpe", `did you mean "archetype"?`},
	} {
		var found *Diagnostic
		for i := range ds {
			if ds[i].Code == CodeUnknownField && ds[i].Path == tc.path {
				found = &ds[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("expected %s at %s, got %v", CodeUnknownField, tc.path, ds)
		}
		if found.Severity != SeverityError {
			t.Errorf("%s severity = %q, want %q", tc.path, found.Severity, SeverityError)
		}
		if !strings.Contains(found.Message, tc.suggest) {
			t.Errorf("%s message = %q, want it to contain %q", tc.path, found.Message, tc.suggest)
		}
	}
	noLeakedInternals(t, ds)
}

func TestParseUnknownMetaFieldMigrationAlias(t *testing.T) {
	// A well-known alias is steered to its migration target before fuzzy matching.
	src := `
meta:
  name: Aliased Title
slides:
  - kind: title
    title: Hi
`
	_, ds := ParseYAML([]byte(src))
	var found *Diagnostic
	for i := range ds {
		if ds[i].Code == CodeUnknownField && ds[i].Path == "meta.name" {
			found = &ds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected %s at meta.name, got %v", CodeUnknownField, ds)
	}
	if want := `did you mean "title"?`; !strings.Contains(found.Message, want) {
		t.Errorf("message = %q, want it to contain %q", found.Message, want)
	}
}

func TestParseUnknownMetaFieldNoCloseMatchListsValidKeys(t *testing.T) {
	// An unrelated meta key gets a generic diagnostic listing the valid meta keys.
	src := `
meta:
  title: Good Deck
  xyzzy_foobar: 1
slides:
  - kind: title
    title: Hi
`
	_, ds := ParseYAML([]byte(src))
	var found *Diagnostic
	for i := range ds {
		if ds[i].Code == CodeUnknownField && ds[i].Path == "meta.xyzzy_foobar" {
			found = &ds[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected %s at meta.xyzzy_foobar, got %v", CodeUnknownField, ds)
	}
	if want := "expected one of title, subtitle, archetype"; !strings.Contains(found.Message, want) {
		t.Errorf("message = %q, want it to contain %q", found.Message, want)
	}
}

func TestParseKnownMetaFieldsAreClean(t *testing.T) {
	// Every recognized meta field present — no unknown-field diagnostic.
	src := `
meta:
  title: Good Deck
  subtitle: A subtitle
  archetype: board_update
  template: midnight-blue
  audience: Board
  author: Jane
  date: 2026-06-01
slides:
  - kind: title
    title: Hi
`
	_, ds := ParseYAML([]byte(src))
	for _, d := range ds {
		if d.Code == CodeUnknownField {
			t.Fatalf("unexpected %s diagnostic: %v", CodeUnknownField, d)
		}
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

// noLeakedInternals asserts the diagnostic messages never expose internal Go or
// YAML decoder type names to the agent.
func noLeakedInternals(t *testing.T, ds Diagnostics) {
	t.Helper()
	leaks := []string{"rawDeck", "DeckMeta", "interface {}", "map[string]", "semantic.", "yaml: unmarshal"}
	for _, d := range ds {
		for _, bad := range leaks {
			if strings.Contains(d.Message, bad) {
				t.Errorf("diagnostic message leaks internal detail %q: %q", bad, d.Message)
			}
		}
	}
}

func TestParseRootArrayIsInvalidRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func([]byte) (*DeckSpec, Diagnostics)
		src  string
	}{
		{"json", ParseJSON, `[{"kind":"title"}]`},
		{"yaml", ParseYAML, "- kind: title\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ds := tc.fn([]byte(tc.src))
			if !hasCodeAtPath(ds, CodeInvalidRoot, "") {
				t.Fatalf("expected %s at root, got %v", CodeInvalidRoot, ds)
			}
			noLeakedInternals(t, ds)
		})
	}
}

func TestParseStringMetaIsInvalidMeta(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func([]byte) (*DeckSpec, Diagnostics)
		src  string
	}{
		{"json", ParseJSON, `{"meta":"hello","slides":[]}`},
		{"yaml", ParseYAML, "meta: hello\nslides: []\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ds := tc.fn([]byte(tc.src))
			if !hasCodeAtPath(ds, CodeInvalidMeta, "meta") {
				t.Fatalf("expected %s at meta, got %v", CodeInvalidMeta, ds)
			}
			noLeakedInternals(t, ds)
		})
	}
}

func TestParseStringSlidesIsInvalidSlides(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func([]byte) (*DeckSpec, Diagnostics)
		src  string
	}{
		{"json", ParseJSON, `{"meta":{},"slides":"nope"}`},
		{"yaml", ParseYAML, "meta: {}\nslides: nope\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, ds := tc.fn([]byte(tc.src))
			if !hasCodeAtPath(ds, CodeInvalidSlides, "slides") {
				t.Fatalf("expected %s at slides, got %v", CodeInvalidSlides, ds)
			}
			noLeakedInternals(t, ds)
		})
	}
}

func TestParseScalarRootIsInvalidRoot(t *testing.T) {
	// A bare scalar document is not a usable deck spec root.
	_, ds := ParseJSON([]byte(`"just a string"`))
	if !hasCodeAtPath(ds, CodeInvalidRoot, "") {
		t.Fatalf("expected %s at root, got %v", CodeInvalidRoot, ds)
	}
	if want := "got a string"; !strings.Contains(ds[0].Message, want) {
		t.Errorf("message = %q, want it to contain %q", ds[0].Message, want)
	}
}

func TestParseValidContainersStillParse(t *testing.T) {
	// The generic container pre-check must not reject a well-formed deck: a
	// valid object root with an object meta and array slides parses cleanly.
	src := `
meta:
  title: Good Deck
slides:
  - kind: title
    title: Hi
`
	spec, ds := ParseYAML([]byte(src))
	if ds.HasErrors() {
		t.Fatalf("unexpected diagnostics for a valid deck: %v", ds)
	}
	if spec == nil || len(spec.Slides) != 1 {
		t.Fatalf("expected a 1-slide spec, got %+v", spec)
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
