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

	// Expand produces a ShapeGridInput from the pattern's typed inputs.
	Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error)
}

// ---------------------------------------------------------------------------
// ExpandContext — inputs available to Expand
// ---------------------------------------------------------------------------

// ExpandContext carries template/slide context needed by Pattern.Expand.
type ExpandContext struct {
	Theme        types.ThemeInfo
	SlideWidth   int64 // EMU
	SlideHeight  int64 // EMU
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

// ---------------------------------------------------------------------------
// Package-level default registry
// ---------------------------------------------------------------------------

var defaultRegistry = NewRegistry()

// Default returns the package-level registry used for static pattern
// registration via init() functions.
func Default() *Registry {
	return defaultRegistry
}
