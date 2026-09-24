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

func TestSourceMapJSONPointerParentLookup(t *testing.T) {
	m := NewSourceMap()
	m.Add("/slides/0/content/1", "slides[0].points", 0)
	for _, path := range []string{
		"/slides/0/content/1/bullets_value",
		"/slides/0/content/1/data/a~1b/label",
	} {
		semanticPath, slideIndex, ok := m.ResolveSemantic(path)
		if !ok || semanticPath != "slides[0].points" || slideIndex != 0 {
			t.Errorf("ResolveSemantic(%q) = %q, %d, %v; want points on slide 0", path, semanticPath, slideIndex, ok)
		}
	}
}

func TestSourceMapJSONPointerMissRecoversSlideIndex(t *testing.T) {
	m := NewSourceMap()
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/slides/7/content/0", 7},
		{"/slides/0", 0},
		{"/slides/-1/content/0", -1},
		{"/slides/nope/content/0", -1},
		{"/other/7", -1},
		{"slides[4].content[0]", 4},
	} {
		_, slideIndex, mapped := m.ResolveSemantic(tc.path)
		if mapped || slideIndex != tc.want {
			t.Errorf("ResolveSemantic(%q) slide = %d, mapped = %v; want %d, false", tc.path, slideIndex, mapped, tc.want)
		}
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
