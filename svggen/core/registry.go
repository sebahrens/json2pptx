package core

import (
	"fmt"
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
