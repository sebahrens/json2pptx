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
	SiteIdx int    // Connection site index (0=top, 1=right, 2=bottom, 3=left for rect)
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
	buf.WriteString(fmt.Sprintf(`    <p:cNvPr id="%d" name="%s"/>`, opts.ID, escapeXMLAttr(name)))
	buf.WriteByte('\n')

	if opts.StartConn != nil || opts.EndConn != nil {
		buf.WriteString(`    <p:cNvCxnSpPr>`)
		buf.WriteByte('\n')
		if opts.StartConn != nil {
			buf.WriteString(fmt.Sprintf(`      <a:stCxn id="%d" idx="%d"/>`,
				opts.StartConn.ShapeID, opts.StartConn.SiteIdx))
			buf.WriteByte('\n')
		}
		if opts.EndConn != nil {
			buf.WriteString(fmt.Sprintf(`      <a:endCxn id="%d" idx="%d"/>`,
				opts.EndConn.ShapeID, opts.EndConn.SiteIdx))
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

	buf.WriteString(fmt.Sprintf(`    <a:prstGeom prst="%s">`, string(opts.Geometry)))
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

// RouteBetween computes connector bounds and connection site indices for routing
// a connector between two shapes. It finds the closest pair of edges and returns
// bounds that span from one connection point to the other.
//
// For pointed geometries (homePlate, chevron, arrows), the connector endpoints
// are offset past the tip to avoid visual overlap.
//
// Connection site indices (for rectangular shapes):
//
//	0 = top center, 1 = right center, 2 = bottom center, 3 = left center
func RouteBetween(source, target ShapeOptions) (bounds RectEmu, startSite, endSite int) {
	// Compute centers of each shape
	srcCX := source.Bounds.X + source.Bounds.CX/2
	srcCY := source.Bounds.Y + source.Bounds.CY/2
	tgtCX := target.Bounds.X + target.Bounds.CX/2
	tgtCY := target.Bounds.Y + target.Bounds.CY/2

	dx := tgtCX - srcCX
	dy := tgtCY - srcCY

	absDX := dx
	if absDX < 0 {
		absDX = -absDX
	}
	absDY := dy
	if absDY < 0 {
		absDY = -absDY
	}

	var startX, startY, endX, endY int64

	if absDX >= absDY {
		// Horizontal-dominant: connect right/left edges
		if dx >= 0 {
			startSite = 1 // source right
			endSite = 3   // target left
			startX = source.Bounds.X + source.Bounds.CX
			startY = srcCY
			endX = target.Bounds.X
			endY = tgtCY

			// Offset past pointed tips to avoid visual overlap
			if geometryHasTipRight(source.Geometry) {
				startX += geometryTipOffset(source.Geometry, source.Bounds.CX)
			}
			if geometryHasTipLeft(target.Geometry) {
				endX -= geometryTipOffset(target.Geometry, target.Bounds.CX)
			}
		} else {
			startSite = 3 // source left
			endSite = 1   // target right
			startX = source.Bounds.X
			startY = srcCY
			endX = target.Bounds.X + target.Bounds.CX
			endY = tgtCY

			// Offset past pointed tips to avoid visual overlap
			if geometryHasTipLeft(source.Geometry) {
				startX -= geometryTipOffset(source.Geometry, source.Bounds.CX)
			}
			if geometryHasTipRight(target.Geometry) {
				endX += geometryTipOffset(target.Geometry, target.Bounds.CX)
			}
		}
	} else {
		// Vertical-dominant: connect top/bottom edges
		if dy >= 0 {
			startSite = 2 // source bottom
			endSite = 0   // target top
			startX = srcCX
			startY = source.Bounds.Y + source.Bounds.CY
			endX = tgtCX
			endY = target.Bounds.Y
		} else {
			startSite = 0 // source top
			endSite = 2   // target bottom
			startX = srcCX
			startY = source.Bounds.Y
			endX = tgtCX
			endY = target.Bounds.Y + target.Bounds.CY
		}
	}

	// Compute bounds: position at min(x,y), size = abs(delta)
	minX := startX
	if endX < minX {
		minX = endX
	}
	minY := startY
	if endY < minY {
		minY = endY
	}

	w := startX - endX
	if w < 0 {
		w = -w
	}
	h := startY - endY
	if h < 0 {
		h = -h
	}

	if w == 0 {
		w = 1
	}
	if h == 0 {
		h = 1
	}

	bounds = RectEmu{X: minX, Y: minY, CX: w, CY: h}
	return bounds, startSite, endSite
}
