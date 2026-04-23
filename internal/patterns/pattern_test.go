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

func TestDefaultRegistryEmpty(t *testing.T) {
	// The default registry should be empty since no concrete patterns
	// are registered in this package.
	list := Default().List()
	if len(list) != 0 {
		t.Errorf("Default().List() returned %d patterns, want 0 (no patterns registered yet)", len(list))
	}
}
