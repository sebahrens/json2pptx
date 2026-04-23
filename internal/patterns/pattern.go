// Package patterns defines the Pattern interface and a thread-safe Registry
// for named slide patterns. Patterns expand to shape_grid definitions at
// generation time, replacing ~600 tokens of boilerplate with ~100-token calls.
//
// See docs/PATTERN_LIBRARY_SPEC.md for the full design (decisions D1–D17).
package patterns

import (
	"fmt"
	"sort"
	"sync"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// Pattern interface — spec §3.3
// ---------------------------------------------------------------------------

// Pattern is the contract every named pattern must implement.
type Pattern interface {
	Name() string
	Description() string
	UseWhen() string
	Version() int

	// NewValues returns a zero-value pointer to the pattern's Values struct,
	// ready for JSON unmarshalling.
	NewValues() any

	// NewOverrides returns a zero-value pointer to the pattern's Overrides
	// struct, or nil if the pattern has no overrides.
	NewOverrides() any

	// NewCellOverride returns a zero-value pointer to the pattern's per-cell
	// override struct, or nil if the pattern allows no per-cell overrides.
	NewCellOverride() any

	// Schema returns the hand-authored JSON Schema for agent-facing discovery.
	// Authoritative for external contracts (D13).
	Schema() *Schema

	// Validate checks values, overrides, and cell overrides for correctness.
	// Returns an errors.Join-aggregated error, or nil.
	Validate(values, overrides any, cellOverrides map[int]any) error

	// CellsHint returns a human-readable cell count description for compact
	// discovery output. Examples: "3", "3-7", "rows × cols", "9", "4 + axes".
	CellsHint() string

	// Expand produces a ShapeGridInput from the pattern's typed inputs.
	Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error)
}

// ---------------------------------------------------------------------------
// ExpandContext — inputs available to Expand
// ---------------------------------------------------------------------------

// ExpandContext carries template/slide context needed by Pattern.Expand.
type ExpandContext struct {
	Theme        types.ThemeInfo
	Metadata     *types.TemplateMetadata // optional; nil when template has no metadata
	SlideWidth   int64                   // EMU
	SlideHeight  int64                   // EMU
	LayoutBounds LayoutBounds
}

// LayoutBounds describes the usable content area on a slide in EMU.
type LayoutBounds struct {
	X      int64 // Left offset in EMU
	Y      int64 // Top offset in EMU
	Width  int64 // Width in EMU
	Height int64 // Height in EMU
}

// ---------------------------------------------------------------------------
// CalloutSupport — optional interface for envelope-level callout (D18)
// ---------------------------------------------------------------------------

// CalloutSupport is an optional interface patterns can implement to declare
// support for an envelope-level callout row appended below the pattern grid.
// Non-uniform patterns (e.g. matrix-2x2, bmc-canvas) should not implement
// this interface. Patterns that do implement it return true from SupportsCallout.
type CalloutSupport interface {
	SupportsCallout() bool
}

// PatternCallout is the DTO for an envelope-level callout band rendered
// below the pattern content. It is a plain text row (no nested shape_grid,
// no bullets) per D15 discipline.
type PatternCallout struct {
	Text     string `json:"text"`
	Emphasis string `json:"emphasis,omitempty"` // "bold", "italic", "bold-italic"
	Accent   string `json:"accent,omitempty"`   // scheme ref, e.g. "accent1"
}

// ---------------------------------------------------------------------------
// Schema — see schema.go for full implementation (bead 3)
// ---------------------------------------------------------------------------

// Schema holds a hand-authored JSON Schema (draft 2020-12) for a pattern's
// external contract. See schema.go for constructors and helpers.
type Schema struct {
	raw schemaJSON
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry is a thread-safe collection of named patterns.
type Registry struct {
	mu       sync.RWMutex
	patterns map[string]Pattern
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		patterns: make(map[string]Pattern),
	}
}

// Register adds a pattern to the registry. It panics if a pattern with the
// same name is already registered.
func (r *Registry) Register(p Pattern) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.patterns[name]; exists {
		panic(fmt.Sprintf("patterns: duplicate registration for %q", name))
	}
	r.patterns[name] = p
}

// Get returns the pattern with the given name and true, or (nil, false) if
// no such pattern is registered.
func (r *Registry) Get(name string) (Pattern, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.patterns[name]
	return p, ok
}

// List returns all registered patterns sorted by name.
func (r *Registry) List() []Pattern {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Pattern, 0, len(r.patterns))
	for _, p := range r.patterns {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}

// Suggest returns the closest registered pattern name to the given input,
// using Damerau-Levenshtein distance. It returns ("", false) if no pattern
// is within maxDist edits (default 2).
func (r *Registry) Suggest(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	const maxDist = 2
	best := ""
	bestDist := maxDist + 1

	for registered := range r.patterns {
		d := damerauLevenshtein(name, registered)
		if d < bestDist {
			bestDist = d
			best = registered
		}
	}
	if bestDist <= maxDist {
		return best, true
	}
	return "", false
}

// damerauLevenshtein computes the Damerau-Levenshtein distance between two
// strings (optimal string alignment variant). It counts insertions, deletions,
// substitutions, and transpositions of adjacent characters.
func damerauLevenshtein(a, b string) int {
	la, lb := len(a), len(b)
	// Quick bounds check.
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// d[i][j] = distance between a[:i] and b[:j].
	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
			// Transposition.
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+cost)
			}
		}
	}
	return d[la][lb]
}

// ---------------------------------------------------------------------------
// Package-level default registry
// ---------------------------------------------------------------------------

var defaultRegistry = NewRegistry()

// Default returns the package-level registry used for static pattern
// registration via init() functions.
func Default() *Registry {
	return defaultRegistry
}
