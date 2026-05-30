package semantic

import "strings"

// SourceMap records the correspondence between raw json2pptx JSON paths in the
// generated PresentationInput and the semantic source paths they were compiled
// from. It lets a later phase map a raw finding (e.g. a fit-report path like
// "slides[2].shape_grid.cells[0]") back to the semantic authoring path it
// originated from ("slides[2].kpis[0]"), so agent-facing diagnostics point at
// the source the author actually wrote rather than at generated internals.
//
// Lookup resolves an exact raw path first, then walks up parent segments to the
// nearest registered ancestor. This makes a single coarse mapping (e.g. on a
// whole slide or a whole grid) cover every nested raw path beneath it, so the
// compiler need not enumerate every leaf.
type SourceMap struct {
	// entries maps a normalized raw path to its source entry.
	entries map[string]SourceEntry
}

// SourceEntry is one raw↔semantic correspondence.
type SourceEntry struct {
	// RawPath is the path in the generated PresentationInput (normalized).
	RawPath string `json:"raw_path"`
	// SemanticPath is the corresponding path in the semantic DeckSpec.
	SemanticPath string `json:"semantic_path"`
	// SlideIndex is the source slide index this entry belongs to, or -1 when the
	// mapping is deck-level (not tied to a specific slide).
	SlideIndex int `json:"slide_index"`
}

// NewSourceMap returns an empty SourceMap ready for Add.
func NewSourceMap() *SourceMap {
	return &SourceMap{entries: map[string]SourceEntry{}}
}

// Add registers a raw→semantic correspondence for the given slide index. A
// slideIndex of -1 marks a deck-level mapping. Re-adding a raw path overwrites
// the prior entry.
func (m *SourceMap) Add(rawPath, semanticPath string, slideIndex int) {
	if m.entries == nil {
		m.entries = map[string]SourceEntry{}
	}
	p := normalizePath(rawPath)
	m.entries[p] = SourceEntry{
		RawPath:      p,
		SemanticPath: semanticPath,
		SlideIndex:   slideIndex,
	}
}

// Len reports the number of registered entries.
func (m *SourceMap) Len() int {
	if m == nil {
		return 0
	}
	return len(m.entries)
}

// Lookup resolves a raw path to its source entry. It tries an exact match
// first, then falls back to the nearest registered ancestor path. The returned
// bool reports whether any match (exact or parent) was found.
func (m *SourceMap) Lookup(rawPath string) (SourceEntry, bool) {
	if m == nil || len(m.entries) == 0 {
		return SourceEntry{}, false
	}
	p := normalizePath(rawPath)
	for {
		if e, ok := m.entries[p]; ok {
			return e, true
		}
		parent := parentPath(p)
		if parent == p {
			return SourceEntry{}, false
		}
		p = parent
	}
}

// Entries returns the registered entries keyed by normalized raw path. The
// returned map is the live backing store; callers must not mutate it.
func (m *SourceMap) Entries() map[string]SourceEntry {
	if m == nil {
		return nil
	}
	return m.entries
}

// normalizePath canonicalizes a JSON path for stable map keys: it trims
// surrounding whitespace and a single leading "$" or "." prefix (so "$.slides",
// ".slides", and "slides" all collapse to the same key).
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "$")
	p = strings.TrimPrefix(p, ".")
	return p
}

// parentPath returns the path one segment up from p, stripping a trailing
// ".field" or "[index]" segment. It returns "" once the root is reached; a path
// with no further segments returns itself unchanged, which Lookup uses as its
// termination signal.
func parentPath(p string) string {
	if p == "" {
		return ""
	}
	i := strings.LastIndexAny(p, ".[")
	if i < 0 {
		return ""
	}
	return p[:i]
}
