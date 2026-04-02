package core

// Diagram is the interface that all diagram renderers must implement.
type Diagram interface {
	// Type returns the diagram type identifier.
	Type() string

	// Render generates an SVG document from the request envelope.
	Render(req *RequestEnvelope) (*SVGDocument, error)

	// Validate checks that the request data is valid for this diagram type.
	Validate(req *RequestEnvelope) error
}

// MultiFormatRenderer is an optional interface that diagrams can implement
// to support rendering to multiple output formats (PNG, PDF).
// Diagrams that implement this interface can be used with RenderMultiFormat.
type MultiFormatRenderer interface {
	Diagram

	// RenderPNG generates PNG bytes from the request envelope.
	// The scale parameter controls the resolution multiplier (e.g., 2.0 for 2x).
	RenderPNG(req *RequestEnvelope, scale float64) ([]byte, error)

	// RenderPDF generates PDF bytes from the request envelope.
	RenderPDF(req *RequestEnvelope) ([]byte, error)
}

// BaseDiagram provides a shared Type() implementation for diagram types.
// Embed it in diagram structs that have a fixed type identifier:
//
//	type FunnelDiagram struct{ core.BaseDiagram }
//
// Then initialize with NewBaseDiagram("funnel_chart").
type BaseDiagram struct {
	typeID string
}

// NewBaseDiagram creates a BaseDiagram with the given type identifier.
func NewBaseDiagram(typeID string) BaseDiagram {
	return BaseDiagram{typeID: typeID}
}

// Type returns the diagram type identifier.
func (b BaseDiagram) Type() string {
	return b.typeID
}
