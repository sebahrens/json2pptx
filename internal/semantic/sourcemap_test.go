package semantic

import "testing"

func TestSourceMapExactLookup(t *testing.T) {
	m := NewSourceMap()
	m.Add("slides[2].shape_grid.cells[0]", "slides[2].kpis[0]", 2)

	e, ok := m.Lookup("slides[2].shape_grid.cells[0]")
	if !ok {
		t.Fatal("expected exact match")
	}
	if e.SemanticPath != "slides[2].kpis[0]" {
		t.Errorf("SemanticPath = %q, want slides[2].kpis[0]", e.SemanticPath)
	}
	if e.SlideIndex != 2 {
		t.Errorf("SlideIndex = %d, want 2", e.SlideIndex)
	}
}

func TestSourceMapParentLookup(t *testing.T) {
	m := NewSourceMap()
	// A coarse mapping on a whole grid covers every nested raw path beneath it.
	m.Add("slides[2].shape_grid", "slides[2].kpis", 2)

	// A deeper raw path with no exact entry resolves to the nearest ancestor.
	e, ok := m.Lookup("slides[2].shape_grid.cells[0].text")
	if !ok {
		t.Fatal("expected parent-path fallback match")
	}
	if e.SemanticPath != "slides[2].kpis" {
		t.Errorf("SemanticPath = %q, want slides[2].kpis (parent)", e.SemanticPath)
	}
}

func TestSourceMapExactBeatsParent(t *testing.T) {
	m := NewSourceMap()
	m.Add("slides[2].shape_grid", "slides[2].kpis", 2)
	m.Add("slides[2].shape_grid.cells[0]", "slides[2].kpis[0]", 2)

	e, ok := m.Lookup("slides[2].shape_grid.cells[0]")
	if !ok {
		t.Fatal("expected match")
	}
	if e.SemanticPath != "slides[2].kpis[0]" {
		t.Errorf("exact lookup returned parent: SemanticPath = %q", e.SemanticPath)
	}
}

func TestSourceMapMissLookup(t *testing.T) {
	m := NewSourceMap()
	m.Add("slides[2].shape_grid", "slides[2].kpis", 2)

	if _, ok := m.Lookup("slides[5].title"); ok {
		t.Error("expected no match for an unrelated path")
	}
	if _, ok := m.Lookup(""); ok {
		t.Error("expected no match for the empty path")
	}
}

func TestSourceMapNormalizesPaths(t *testing.T) {
	m := NewSourceMap()
	m.Add("$.slides[0]", "slides[0]", 0)

	// Leading "$"/"." prefixes collapse to the same key.
	for _, q := range []string{"slides[0]", ".slides[0]", "$.slides[0]"} {
		if _, ok := m.Lookup(q); !ok {
			t.Errorf("Lookup(%q) failed; path normalization should collapse prefixes", q)
		}
	}
	if m.Len() != 1 {
		t.Errorf("Len() = %d, want 1", m.Len())
	}
}

func TestSourceMapNilSafe(t *testing.T) {
	var m *SourceMap
	if _, ok := m.Lookup("slides[0]"); ok {
		t.Error("nil SourceMap Lookup should report no match")
	}
	if m.Len() != 0 {
		t.Errorf("nil SourceMap Len() = %d, want 0", m.Len())
	}
}
