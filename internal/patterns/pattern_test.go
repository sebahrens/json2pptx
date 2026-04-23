package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// stubPattern — minimal Pattern implementation for registry tests
// ---------------------------------------------------------------------------

type stubPattern struct {
	name    string
	desc    string
	useWhen string
	version int
}

func (s *stubPattern) Name() string        { return s.name }
func (s *stubPattern) Description() string { return s.desc }
func (s *stubPattern) UseWhen() string     { return s.useWhen }
func (s *stubPattern) Version() int        { return s.version }
func (s *stubPattern) NewValues() any      { return nil }
func (s *stubPattern) NewOverrides() any   { return nil }
func (s *stubPattern) NewCellOverride() any { return nil }
func (s *stubPattern) Schema() *Schema     { return nil }
func (s *stubPattern) CellsHint() string   { return "" }

func (s *stubPattern) Validate(_, _ any, _ map[int]any) error { return nil }
func (s *stubPattern) Expand(_ ExpandContext, _, _ any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &stubPattern{name: "test-pattern", desc: "A test", useWhen: "testing", version: 1}

	r.Register(p)

	got, ok := r.Get("test-pattern")
	if !ok {
		t.Fatal("expected Get to find registered pattern")
	}
	if got.Name() != "test-pattern" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test-pattern")
	}
}

func TestGetMissing(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("expected Get to return false for unregistered pattern")
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	p := &stubPattern{name: "dup"}

	r.Register(p)

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()

	r.Register(p)
}

func TestListSorted(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "zebra"})
	r.Register(&stubPattern{name: "alpha"})
	r.Register(&stubPattern{name: "middle"})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("List() returned %d patterns, want 3", len(list))
	}

	want := []string{"alpha", "middle", "zebra"}
	for i, p := range list {
		if p.Name() != want[i] {
			t.Errorf("List()[%d].Name() = %q, want %q", i, p.Name(), want[i])
		}
	}
}

func TestListEmpty(t *testing.T) {
	r := NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Fatalf("List() on empty registry returned %d patterns, want 0", len(list))
	}
}

func TestNewValuesOverridesCellOverrideMayReturnNil(t *testing.T) {
	p := &stubPattern{name: "nil-pattern"}

	if p.NewValues() != nil {
		t.Error("NewValues() should return nil for stub")
	}
	if p.NewOverrides() != nil {
		t.Error("NewOverrides() should return nil for stub")
	}
	if p.NewCellOverride() != nil {
		t.Error("NewCellOverride() should return nil for stub")
	}
}

// ---------------------------------------------------------------------------
// CalloutSupport interface tests (D18)
// ---------------------------------------------------------------------------

func TestCardGridSupportsCallout(t *testing.T) {
	p, ok := Default().Get("card-grid")
	if !ok {
		t.Fatal("expected card-grid to be registered")
	}
	cs, ok := p.(CalloutSupport)
	if !ok {
		t.Fatal("card-grid does not implement CalloutSupport")
	}
	if !cs.SupportsCallout() {
		t.Error("card-grid.SupportsCallout() = false, want true")
	}
}

func TestComparison2colSupportsCallout(t *testing.T) {
	p, ok := Default().Get("comparison-2col")
	if !ok {
		t.Fatal("expected comparison-2col to be registered")
	}
	cs, ok := p.(CalloutSupport)
	if !ok {
		t.Fatal("comparison-2col does not implement CalloutSupport")
	}
	if !cs.SupportsCallout() {
		t.Error("comparison-2col.SupportsCallout() = false, want true")
	}
}

func TestMatrix2x2DoesNotSupportCallout(t *testing.T) {
	p, ok := Default().Get("matrix-2x2")
	if !ok {
		t.Fatal("expected matrix-2x2 to be registered")
	}
	if cs, ok := p.(CalloutSupport); ok && cs.SupportsCallout() {
		t.Error("matrix-2x2 should not support callout")
	}
}

func TestOnlyExpectedPatternsOptIntoCallout(t *testing.T) {
	allowed := map[string]bool{
		"card-grid":       true,
		"comparison-2col": true,
	}
	for _, p := range Default().List() {
		cs, ok := p.(CalloutSupport)
		if ok && cs.SupportsCallout() && !allowed[p.Name()] {
			t.Errorf("pattern %q implements CalloutSupport but is not in the allowed set", p.Name())
		}
	}
}

// ---------------------------------------------------------------------------
// Damerau-Levenshtein + Suggest tests
// ---------------------------------------------------------------------------

func TestDamerauLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"abc", "abd", 1},   // substitution
		{"abc", "abcd", 1},  // insertion
		{"abcd", "abc", 1},  // deletion
		{"abc", "bac", 1},   // transposition
		{"card-grid", "card_grid", 1},  // underscore vs hyphen
		{"card-grids", "card-grid", 1}, // trailing s
		{"kpi-3up", "kpi-3pu", 1},      // transposition
		{"card-grid", "bmc-canvas", 10}, // very different
	}
	for _, tt := range tests {
		got := damerauLevenshtein(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("damerauLevenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggestFindsClose(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "card-grid"})
	r.Register(&stubPattern{name: "bmc-canvas"})
	r.Register(&stubPattern{name: "kpi-3up"})

	tests := []struct {
		input      string
		wantName   string
		wantFound  bool
	}{
		{"card_grid", "card-grid", true},   // underscore → hyphen (dist 1)
		{"card-grids", "card-grid", true},  // trailing s (dist 1)
		{"kpi-3pu", "kpi-3up", true},       // transposition (dist 1)
		{"bmc-canva", "bmc-canvas", true},  // missing char (dist 1)
		{"totally-different", "", false},    // too far away
	}
	for _, tt := range tests {
		name, ok := r.Suggest(tt.input)
		if ok != tt.wantFound {
			t.Errorf("Suggest(%q) found=%v, want %v", tt.input, ok, tt.wantFound)
			continue
		}
		if ok && name != tt.wantName {
			t.Errorf("Suggest(%q) = %q, want %q", tt.input, name, tt.wantName)
		}
	}
}

func TestSuggestEmptyRegistry(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Suggest("anything")
	if ok {
		t.Error("Suggest on empty registry should return false")
	}
}

func TestDefaultRegistryContainsKpi3up(t *testing.T) {
	// The default registry should contain kpi-3up after init().
	p, ok := Default().Get("kpi-3up")
	if !ok {
		t.Fatal("expected kpi-3up to be registered in default registry")
	}
	if p.Name() != "kpi-3up" {
		t.Errorf("Name() = %q, want %q", p.Name(), "kpi-3up")
	}
}

// ---------------------------------------------------------------------------
// Alias tests
// ---------------------------------------------------------------------------

func TestRegisterAliasAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "timeline-horizontal"})
	r.RegisterAlias("timeline", "timeline-horizontal")

	// Alias resolves to canonical pattern.
	p, ok := r.Get("timeline")
	if !ok {
		t.Fatal("expected Get to find pattern via alias")
	}
	if p.Name() != "timeline-horizontal" {
		t.Errorf("Name() = %q, want %q", p.Name(), "timeline-horizontal")
	}

	// Canonical name still works.
	p2, ok := r.Get("timeline-horizontal")
	if !ok {
		t.Fatal("expected Get to find pattern via canonical name")
	}
	if p2.Name() != "timeline-horizontal" {
		t.Errorf("Name() = %q, want %q", p2.Name(), "timeline-horizontal")
	}
}

func TestRegisterAliasPanicsOnCollisionWithCanonical(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "card-grid"})

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic when alias collides with canonical name")
		}
	}()
	r.RegisterAlias("card-grid", "card-grid")
}

func TestRegisterAliasPanicsOnDuplicate(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "timeline-horizontal"})
	r.RegisterAlias("timeline", "timeline-horizontal")

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic on duplicate alias")
		}
	}()
	r.RegisterAlias("timeline", "timeline-horizontal")
}

func TestRegisterAliasPanicsOnMissingTarget(t *testing.T) {
	r := NewRegistry()

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic when alias target is not registered")
		}
	}()
	r.RegisterAlias("tl", "nonexistent")
}

func TestResolveAlias(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "bmc-canvas"})
	r.RegisterAlias("bmc", "bmc-canvas")

	if got := r.ResolveAlias("bmc"); got != "bmc-canvas" {
		t.Errorf("ResolveAlias(%q) = %q, want %q", "bmc", got, "bmc-canvas")
	}
	if got := r.ResolveAlias("bmc-canvas"); got != "bmc-canvas" {
		t.Errorf("ResolveAlias(%q) = %q, want %q", "bmc-canvas", got, "bmc-canvas")
	}
	if got := r.ResolveAlias("unknown"); got != "unknown" {
		t.Errorf("ResolveAlias(%q) = %q, want %q", "unknown", got, "unknown")
	}
}

func TestAliasesReturnsMap(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "timeline-horizontal"})
	r.Register(&stubPattern{name: "bmc-canvas"})
	r.RegisterAlias("timeline", "timeline-horizontal")
	r.RegisterAlias("bmc", "bmc-canvas")

	aliases := r.Aliases()
	if len(aliases) != 2 {
		t.Fatalf("Aliases() returned %d entries, want 2", len(aliases))
	}
	if aliases["timeline"] != "timeline-horizontal" {
		t.Errorf("aliases[timeline] = %q, want %q", aliases["timeline"], "timeline-horizontal")
	}
	if aliases["bmc"] != "bmc-canvas" {
		t.Errorf("aliases[bmc] = %q, want %q", aliases["bmc"], "bmc-canvas")
	}
}

func TestListExcludesAliases(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "timeline-horizontal"})
	r.RegisterAlias("timeline", "timeline-horizontal")

	list := r.List()
	if len(list) != 1 {
		t.Fatalf("List() returned %d patterns, want 1 (aliases excluded)", len(list))
	}
	if list[0].Name() != "timeline-horizontal" {
		t.Errorf("List()[0].Name() = %q, want %q", list[0].Name(), "timeline-horizontal")
	}
}

func TestSuggestFindsAlias(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubPattern{name: "timeline-horizontal"})
	r.RegisterAlias("timeline", "timeline-horizontal")

	// "timline" is 1 edit from "timeline" (the alias).
	name, ok := r.Suggest("timline")
	if !ok {
		t.Fatal("expected Suggest to find a match for typo of alias")
	}
	if name != "timeline" {
		t.Errorf("Suggest(%q) = %q, want %q", "timline", name, "timeline")
	}
}

func TestDefaultRegistryAliases(t *testing.T) {
	// Verify the default aliases resolve correctly.
	tests := []struct {
		alias     string
		canonical string
	}{
		{"timeline", "timeline-horizontal"},
		{"bmc", "bmc-canvas"},
		{"matrix", "matrix-2x2"},
		{"comparison", "comparison-2col"},
	}
	for _, tt := range tests {
		p, ok := Default().Get(tt.alias)
		if !ok {
			t.Errorf("Default().Get(%q) failed", tt.alias)
			continue
		}
		if p.Name() != tt.canonical {
			t.Errorf("Default().Get(%q).Name() = %q, want %q", tt.alias, p.Name(), tt.canonical)
		}
	}
}
