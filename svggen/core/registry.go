package core

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages diagram type registrations and dispatches render requests.
type Registry struct {
	mu       sync.RWMutex
	diagrams map[string]Diagram
	aliases  map[string]string // alias -> canonical type
}

// defaultRegistry is the package-level registry used by Render().
// Access through DefaultRegistry(), SetDefaultRegistry(), or ResetDefaultRegistry().
var defaultRegistry = NewRegistry()

// registryMu protects access to defaultRegistry for thread-safety.
var registryMu sync.RWMutex

// DefaultRegistry returns the package-level registry used by Render().
// For testing, use SetDefaultRegistry() to inject a custom registry.
func DefaultRegistry() *Registry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return defaultRegistry
}

// SetDefaultRegistry sets the package-level registry.
// This is useful for testing to inject a custom registry.
// Returns the previous registry so it can be restored in test cleanup.
func SetDefaultRegistry(r *Registry) *Registry {
	registryMu.Lock()
	defer registryMu.Unlock()
	old := defaultRegistry
	defaultRegistry = r
	return old
}

// ResetDefaultRegistry resets the package-level registry to a fresh instance.
// This is useful for test cleanup to ensure test isolation.
func ResetDefaultRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	defaultRegistry = NewRegistry()
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		diagrams: make(map[string]Diagram),
		aliases:  make(map[string]string),
	}
}

// Register adds a diagram implementation to the registry.
// Panics if a diagram with the same type is already registered.
func (r *Registry) Register(d Diagram) {
	r.mu.Lock()
	defer r.mu.Unlock()

	typ := d.Type()
	if _, exists := r.diagrams[typ]; exists {
		panic(fmt.Sprintf("svggen: diagram type %q already registered", typ))
	}
	r.diagrams[typ] = d
}

// MustRegister is like Register but returns the registry for chaining.
func (r *Registry) MustRegister(d Diagram) *Registry {
	r.Register(d)
	return r
}

// Alias registers an alias that resolves to a canonical diagram type.
// This allows short names like "funnel" to resolve to "funnel_chart".
// Panics if the alias name conflicts with an already-registered diagram type.
func (r *Registry) Alias(alias, canonical string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.diagrams[alias]; exists {
		panic(fmt.Sprintf("svggen: alias %q conflicts with registered diagram type", alias))
	}
	r.aliases[alias] = canonical
}

// Get retrieves a diagram by type, checking aliases as a fallback.
// Returns nil if the type is not registered and no alias matches.
func (r *Registry) Get(typ string) Diagram {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if d, ok := r.diagrams[typ]; ok {
		return d
	}
	// Check aliases
	if canonical, ok := r.aliases[typ]; ok {
		return r.diagrams[canonical]
	}
	return nil
}

// Types returns all registered diagram types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.diagrams))
	for typ := range r.diagrams {
		types = append(types, typ)
	}
	return types
}

// Aliases returns all alternative names (excluding name itself) that resolve to
// the same underlying diagram. The input may be either a canonical registered
// type ID or an alias. Result is sorted lexicographically.
//
// Example: Aliases("bar_chart") -> ["bar"]; Aliases("bar") -> ["bar_chart"].
// Returns an empty slice when the name is unknown or has no equivalents.
func (r *Registry) Aliases(name string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Resolve to the canonical registered type ID.
	canonical := name
	if c, ok := r.aliases[name]; ok {
		canonical = c
	}
	// If canonical isn't actually registered, give up.
	if _, ok := r.diagrams[canonical]; !ok {
		return []string{}
	}

	out := make([]string, 0)
	for alias, target := range r.aliases {
		if target == canonical && alias != name {
			out = append(out, alias)
		}
	}
	// If the caller passed an alias, include the canonical registered name.
	if canonical != name {
		out = append(out, canonical)
	}
	sort.Strings(out)
	return out
}

// Render dispatches a request to the appropriate diagram renderer.
func (r *Registry) Render(req *RequestEnvelope) (*SVGDocument, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Clamp extreme float64 values to prevent NaN/Inf in downstream math.
	if req.Data != nil {
		ClampDataValues(req.Data)
	}

	d := r.Get(req.Type)
	if d == nil {
		return nil, fmt.Errorf("svggen: unknown diagram type %q", req.Type)
	}

	// If the diagram provides a data schema, validate unknown fields first.
	if ds, ok := d.(DiagramWithSchema); ok {
		if err := ValidateUnknownFields(req.Data, ds.DataSchema(), req.Type); err != nil {
			return nil, fmt.Errorf("svggen: validation failed for %q: %w", req.Type, err)
		}
	}

	if err := d.Validate(req); err != nil {
		return nil, fmt.Errorf("svggen: validation failed for %q: %w", req.Type, err)
	}

	return d.Render(req)
}

// Register adds a diagram to the default registry.
func Register(d Diagram) {
	DefaultRegistry().Register(d)
}

// Alias registers an alias in the default registry.
func Alias(alias, canonical string) {
	DefaultRegistry().Alias(alias, canonical)
}

// Render uses the default registry to render a request.
func Render(req *RequestEnvelope) (*SVGDocument, error) {
	return DefaultRegistry().Render(req)
}

// Types returns all types in the default registry.
func Types() []string {
	return DefaultRegistry().Types()
}

// Aliases returns alternative names for the given diagram type in the default
// registry. See (*Registry).Aliases for details.
func Aliases(name string) []string {
	return DefaultRegistry().Aliases(name)
}
