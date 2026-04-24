// This file provides type aliases and function wrappers that re-export
// everything from core/ for backward compatibility. Importing svggen gives
// you the full package with all diagram types auto-registered via init().
// Importing svggen/core gives you only the types and registry without any
// diagram implementations linked.
package svggen

import "github.com/sebahrens/json2pptx/svggen/core"

// --- Type aliases (same type, full backward compat) ---

// Registry manages diagram type registrations and dispatches render requests.
type Registry = core.Registry

// RequestEnvelope is the top-level container for diagram generation requests.
type RequestEnvelope = core.RequestEnvelope

// OutputSpec defines the output format and dimensions.
type OutputSpec = core.OutputSpec

// StyleSpec defines theming and appearance options.
type StyleSpec = core.StyleSpec

// PaletteSpec specifies a color palette by name or by custom hex colors.
type PaletteSpec = core.PaletteSpec

// ThemeColorInput carries a single theme color from the PPTX template.
type ThemeColorInput = core.ThemeColorInput

// SVGDocument represents a rendered SVG document.
type SVGDocument = core.SVGDocument

// RenderResult contains the output of a render operation.
type RenderResult = core.RenderResult

// RenderOutput wraps RenderResult with a Findings slice for structured feedback.
type RenderOutput = core.RenderOutput

// Diagram is the interface that all diagram renderers must implement.
type Diagram = core.Diagram

// MultiFormatRenderer is an optional interface for multi-format output.
type MultiFormatRenderer = core.MultiFormatRenderer

// BaseDiagram provides a shared Type() implementation for diagram types.
type BaseDiagram = core.BaseDiagram

// ValidationError represents a structured validation failure.
type ValidationError = core.ValidationError

// ValidationErrors is a collection of validation errors.
type ValidationErrors = core.ValidationErrors

// --- Function wrappers ---

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry { return core.NewRegistry() }

// DefaultRegistry returns the package-level registry.
func DefaultRegistry() *Registry { return core.DefaultRegistry() }

// SetDefaultRegistry sets the package-level registry.
func SetDefaultRegistry(r *Registry) *Registry { return core.SetDefaultRegistry(r) }

// ResetDefaultRegistry resets the package-level registry to a fresh instance.
func ResetDefaultRegistry() { core.ResetDefaultRegistry() }

// Register adds a diagram to the default registry.
func Register(d Diagram) { core.Register(d) }

// Alias registers an alias in the default registry.
func Alias(alias, canonical string) { core.Alias(alias, canonical) }

// Render uses the default registry to render a request.
func Render(req *RequestEnvelope) (*SVGDocument, error) { return core.Render(req) }

// Types returns all types in the default registry.
func Types() []string { return core.Types() }

// ParseRequest parses a JSON request into a RequestEnvelope.
func ParseRequest(data []byte) (*RequestEnvelope, error) { return core.ParseRequest(data) }

// NewBaseDiagram creates a BaseDiagram with the given type identifier.
func NewBaseDiagram(typeID string) BaseDiagram { return core.NewBaseDiagram(typeID) }

// IsValidationError checks if an error is a validation error.
func IsValidationError(err error) bool { return core.IsValidationError(err) }

// GetValidationErrors extracts validation errors from an error.
func GetValidationErrors(err error) []ValidationError { return core.GetValidationErrors(err) }

// --- Constants (re-exported) ---

const (
	ErrCodeRequired       = core.ErrCodeRequired
	ErrCodeInvalidType    = core.ErrCodeInvalidType
	ErrCodeInvalidFormat  = core.ErrCodeInvalidFormat
	ErrCodeInvalidValue   = core.ErrCodeInvalidValue
	ErrCodeUnknownField   = core.ErrCodeUnknownField
	ErrCodeParseFailed    = core.ErrCodeParseFailed
	ErrCodeConstraint     = core.ErrCodeConstraint
	ErrCodeUnknownDiagram = core.ErrCodeUnknownDiagram
	MinSVGScale           = core.MinSVGScale
	MaxSVGScale           = core.MaxSVGScale
)
