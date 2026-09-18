package pptx

import (
	"bytes"
	"fmt"
)

// ConnectorOptions configures the generation of a p:cxnSp (connector) element.
type ConnectorOptions struct {
	// ID is the unique shape ID (cNvPr/@id). Required.
	ID uint32

	// Name is the connector name (cNvPr/@name). Defaults to "Connector N".
	Name string

	// Geometry is the connector preset geometry type. Required.
	// Valid values: straightConnector1, bentConnector2-5, curvedConnector2-5.
	Geometry PresetGeometry

	// Bounds defines position and size in EMU.
	Bounds RectEmu

	// Line is the connector outline. If zero value, a default 1pt black line is used.
	Line Line

	// HeadEnd is an optional arrowhead at the start of the connector.
	HeadEnd *ArrowHead

	// TailEnd is an optional arrowhead at the end of the connector.
	TailEnd *ArrowHead

	// StartConn is an optional connection to a shape at the start.
	StartConn *ConnectionRef

	// EndConn is an optional connection to a shape at the end.
	EndConn *ConnectionRef

	// FlipH mirrors the connector horizontally.
	FlipH bool

	// FlipV mirrors the connector vertically.
	FlipV bool
}

// ConnectionRef identifies a connection site on a target shape.
type ConnectionRef struct {
	ShapeID uint32 // cNvPr id of target shape
	SiteIdx int    // Connection site index; see ConnectionSiteIndex (rect: 0=top, 1=left, 2=bottom, 3=right)
}

// ArrowHead describes an arrowhead on a connector end.
type ArrowHead struct {
	Type string // "triangle", "arrow", "stealth", "diamond", "oval", "none"
	W    string // Width: "sm", "med", "lg"
	Len  string // Length: "sm", "med", "lg"
}

// Connector preset geometries.
const (
	GeomStraightConnector1 PresetGeometry = "straightConnector1"
	GeomBentConnector2     PresetGeometry = "bentConnector2"
	GeomBentConnector3     PresetGeometry = "bentConnector3"
	GeomBentConnector4     PresetGeometry = "bentConnector4"
	GeomBentConnector5     PresetGeometry = "bentConnector5"
	GeomCurvedConnector2   PresetGeometry = "curvedConnector2"
	GeomCurvedConnector3   PresetGeometry = "curvedConnector3"
	GeomCurvedConnector4   PresetGeometry = "curvedConnector4"
	GeomCurvedConnector5   PresetGeometry = "curvedConnector5"
)

// GenerateConnector generates a complete p:cxnSp XML element for a connector shape.
//
// The generated element includes:
//   - p:nvCxnSpPr: Non-visual properties (cNvPr, cNvCxnSpPr with optional stCxn/endCxn, nvPr)
//   - p:spPr: Shape properties (xfrm, prstGeom, line properties)
//   - NO p:txBody (connectors cannot contain text)
func GenerateConnector(opts ConnectorOptions) ([]byte, error) {
	if opts.ID == 0 {
		return nil, fmt.Errorf("ConnectorOptions.ID is required")
	}
	if opts.Geometry == "" {
		return nil, fmt.Errorf("ConnectorOptions.Geometry is required")
	}

	name := opts.Name
	if name == "" {
		name = fmt.Sprintf("Connector %d", opts.ID)
	}

	// Default line: 1pt black solid
	line := opts.Line
	if line.Width == 0 && line.Fill.IsZero() {
		line = SolidLinePoints(1.0, "000000")
	}

	var buf bytes.Buffer

	buf.WriteString(`<p:cxnSp>`)
	buf.WriteByte('\n')

	// --- nvCxnSpPr ---
	buf.WriteString(`  <p:nvCxnSpPr>`)
	buf.WriteByte('\n')
	fmt.Fprintf(&buf, `    <p:cNvPr id="%d" name="%s"/>`, opts.ID, escapeXMLAttr(name))
	buf.WriteByte('\n')

	if opts.StartConn != nil || opts.EndConn != nil {
		buf.WriteString(`    <p:cNvCxnSpPr>`)
		buf.WriteByte('\n')
		if opts.StartConn != nil {
			fmt.Fprintf(&buf, `      <a:stCxn id="%d" idx="%d"/>`,
				opts.StartConn.ShapeID, opts.StartConn.SiteIdx)
			buf.WriteByte('\n')
		}
		if opts.EndConn != nil {
			fmt.Fprintf(&buf, `      <a:endCxn id="%d" idx="%d"/>`,
				opts.EndConn.ShapeID, opts.EndConn.SiteIdx)
			buf.WriteByte('\n')
		}
		buf.WriteString(`    </p:cNvCxnSpPr>`)
	} else {
		buf.WriteString(`    <p:cNvCxnSpPr/>`)
	}
	buf.WriteByte('\n')

	buf.WriteString(`    <p:nvPr/>`)
	buf.WriteByte('\n')
	buf.WriteString(`  </p:nvCxnSpPr>`)
	buf.WriteByte('\n')

	// --- spPr ---
	buf.WriteString(`  <p:spPr>`)
	buf.WriteByte('\n')

	buf.WriteString(`    `)
	WriteTransform(&buf, opts.Bounds, 0, opts.FlipH, opts.FlipV)
	buf.WriteByte('\n')

	fmt.Fprintf(&buf, `    <a:prstGeom prst="%s">`, string(opts.Geometry))
	buf.WriteByte('\n')
	buf.WriteString(`      <a:avLst/>`)
	buf.WriteByte('\n')
	buf.WriteString(`    </a:prstGeom>`)
	buf.WriteByte('\n')

	// Line with optional arrowheads
	buf.WriteString(`    `)
	writeConnectorLine(&buf, line, opts.HeadEnd, opts.TailEnd)
	buf.WriteByte('\n')

	buf.WriteString(`  </p:spPr>`)
	buf.WriteByte('\n')

	buf.WriteString(`</p:cxnSp>`)

	return buf.Bytes(), nil
}

// writeConnectorLine writes an a:ln element with optional arrowhead elements.
func writeConnectorLine(buf *bytes.Buffer, line Line, headEnd, tailEnd *ArrowHead) {
	if line.Width > 0 {
		fmt.Fprintf(buf, `<a:ln w="%d"`, line.Width)
	} else {
		buf.WriteString(`<a:ln`)
	}
	if line.Cap != "" {
		fmt.Fprintf(buf, ` cap="%s"`, line.Cap)
	}
	if line.Compound != "" {
		fmt.Fprintf(buf, ` cmpd="%s"`, line.Compound)
	}
	if line.Align != "" {
		fmt.Fprintf(buf, ` algn="%s"`, line.Align)
	}
	buf.WriteString(`>`)

	// Fill
	line.Fill.WriteTo(buf)

	// Dash
	if line.Dash != "" {
		fmt.Fprintf(buf, `<a:prstDash val="%s"/>`, line.Dash)
	}

	// Join
	switch line.Join {
	case "round":
		buf.WriteString(`<a:round/>`)
	case "bevel":
		buf.WriteString(`<a:bevel/>`)
	case "miter":
		buf.WriteString(`<a:miter/>`)
	}

	// Arrowheads
	if headEnd != nil {
		writeArrowHead(buf, "a:headEnd", headEnd)
	}
	if tailEnd != nil {
		writeArrowHead(buf, "a:tailEnd", tailEnd)
	}

	buf.WriteString(`</a:ln>`)
}

// writeArrowHead writes an arrowhead element (a:headEnd or a:tailEnd).
func writeArrowHead(buf *bytes.Buffer, tag string, ah *ArrowHead) {
	fmt.Fprintf(buf, `<%s type="%s"`, tag, ah.Type)
	if ah.W != "" {
		fmt.Fprintf(buf, ` w="%s"`, ah.W)
	}
	if ah.Len != "" {
		fmt.Fprintf(buf, ` len="%s"`, ah.Len)
	}
	buf.WriteString(`/>`)
}

// geometryTipOffset returns an outward offset (in EMU) to add when routing
// connectors away from a shape edge. Pointed geometries like homePlate and
// chevron have tips that extend to the bounding box edge; connectors starting
// exactly at the edge visually overlap with the tip. The offset pushes the
// connector endpoint past the tip into the gap.
func geometryTipOffset(geom PresetGeometry, shapeWidth int64) int64 {
	switch geom {
	case GeomHomePlate, GeomChevron, GeomRightArrow:
		// ~15% of shape width provides clearance past the pointed tip
		return shapeWidth * 15 / 100
	case GeomLeftArrow:
		// Left arrow has tip on the left side
		return shapeWidth * 15 / 100
	default:
		return 0
	}
}

// geometryHasTipRight returns true if the geometry has a pointed tip on its right side.
func geometryHasTipRight(geom PresetGeometry) bool {
	switch geom {
	case GeomHomePlate, GeomChevron, GeomRightArrow:
		return true
	default:
		return false
	}
}

// geometryHasTipLeft returns true if the geometry has a pointed tip on its left side.
func geometryHasTipLeft(geom PresetGeometry) bool {
	switch geom {
	case GeomLeftArrow, GeomChevron:
		return true
	default:
		return false
	}
}


// ConnectionSide names a side of a shape's bounding box for connector routing.
type ConnectionSide int

// Connection sides in the order they appear in most preset cxnLst entries.
const (
	SideTop ConnectionSide = iota
	SideLeft
	SideBottom
	SideRight
)

// ConnectionSiteIndex returns the index into the preset geometry's
// connection-site list (<a:cxnLst>) for the midpoint of the given side.
//
// OOXML presets list their sites counter-clockwise from the top, so for
// rect-like presets (rect, roundRect, chevron, homePlate, diamond, callouts,
// …) the order is 0 = top, 1 = left, 2 = bottom, 3 = right. A few presets
// carry extra sites (ellipse has 8; triangle has 6) and are mapped
// explicitly. Using the wrong index makes renderers that honour the
// attachment (LibreOffice on import, PowerPoint whenever a shape moves)
// re-route the connector from the far side of each shape, dragging the line
// across both shapes and their text.
func ConnectionSiteIndex(geom PresetGeometry, side ConnectionSide) int {
	switch geom {
	case GeomEllipse:
		return [...]int{0, 2, 4, 6}[side]
	case GeomTriangle:
		return [...]int{0, 1, 3, 5}[side]
	default:
		return int(side)
	}
}

// ConnectorRoute is a resolved connector path between two shapes.
type ConnectorRoute struct {
	Bounds    RectEmu // Normalised connector bounds (positive extents)
	StartSite int     // Connection site index on the source shape
	EndSite   int     // Connection site index on the target shape
	StartX    int64   // Start point (EMU)
	StartY    int64
	EndX      int64 // End point (EMU)
	EndY      int64
	FlipH     bool // End point lies left of the start point
	FlipV     bool // End point lies above the start point
}

// RouteBetween computes connector bounds and connection site indices for routing
// a connector between two shapes. It finds the closest pair of edges and returns
// bounds that span from one connection point to the other.
//
// For pointed geometries (homePlate, chevron, arrows), the connector endpoints
// are offset past the tip to avoid visual overlap.
//
// Site indices come from ConnectionSiteIndex (for rect: 0 = top, 1 = left,
// 2 = bottom, 3 = right). Callers that need the flip flags for a straight or
// bent connector should use Route.
func RouteBetween(source, target ShapeOptions) (bounds RectEmu, startSite, endSite int) {
	r := Route(source, target, false)
	return r.Bounds, r.StartSite, r.EndSite
}

// Route computes the connector path between source and target. When
// horizontal is true the connector always runs from a side edge to the facing
// side edge (used for cells that share a grid row, whose centres may be far
// apart vertically when one of them spans several rows); otherwise the
// dominant axis between the shape centres decides.
func Route(source, target ShapeOptions, horizontal bool) ConnectorRoute {
	srcCX := source.Bounds.X + source.Bounds.CX/2
	srcCY := source.Bounds.Y + source.Bounds.CY/2
	tgtCX := target.Bounds.X + target.Bounds.CX/2
	tgtCY := target.Bounds.Y + target.Bounds.CY/2

	dx := tgtCX - srcCX
	dy := tgtCY - srcCY

	var r ConnectorRoute
	if horizontal || abs64(dx) >= abs64(dy) {
		if dx >= 0 {
			r.StartSite = ConnectionSiteIndex(source.Geometry, SideRight)
			r.EndSite = ConnectionSiteIndex(target.Geometry, SideLeft)
			r.StartX = source.Bounds.X + source.Bounds.CX
			r.EndX = target.Bounds.X
			if geometryHasTipRight(source.Geometry) {
				r.StartX += geometryTipOffset(source.Geometry, source.Bounds.CX)
			}
			if geometryHasTipLeft(target.Geometry) {
				r.EndX -= geometryTipOffset(target.Geometry, target.Bounds.CX)
			}
		} else {
			r.StartSite = ConnectionSiteIndex(source.Geometry, SideLeft)
			r.EndSite = ConnectionSiteIndex(target.Geometry, SideRight)
			r.StartX = source.Bounds.X
			r.EndX = target.Bounds.X + target.Bounds.CX
			if geometryHasTipLeft(source.Geometry) {
				r.StartX -= geometryTipOffset(source.Geometry, source.Bounds.CX)
			}
			if geometryHasTipRight(target.Geometry) {
				r.EndX += geometryTipOffset(target.Geometry, target.Bounds.CX)
			}
		}
		r.StartY = srcCY
		r.EndY = tgtCY
	} else {
		if dy >= 0 {
			r.StartSite = ConnectionSiteIndex(source.Geometry, SideBottom)
			r.EndSite = ConnectionSiteIndex(target.Geometry, SideTop)
			r.StartY = source.Bounds.Y + source.Bounds.CY
			r.EndY = target.Bounds.Y
		} else {
			r.StartSite = ConnectionSiteIndex(source.Geometry, SideTop)
			r.EndSite = ConnectionSiteIndex(target.Geometry, SideBottom)
			r.StartY = source.Bounds.Y
			r.EndY = target.Bounds.Y + target.Bounds.CY
		}
		r.StartX = srcCX
		r.EndX = tgtCX
	}

	w := abs64(r.StartX - r.EndX)
	h := abs64(r.StartY - r.EndY)
	if w == 0 {
		w = 1
	}
	if h == 0 {
		h = 1
	}
	r.Bounds = RectEmu{X: min64(r.StartX, r.EndX), Y: min64(r.StartY, r.EndY), CX: w, CY: h}
	r.FlipH = r.EndX < r.StartX
	r.FlipV = r.EndY < r.StartY
	return r
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
