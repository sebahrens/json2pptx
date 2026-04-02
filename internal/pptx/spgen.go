package pptx

import (
	"bytes"
	"fmt"
)

// ShapeOptions configures the generation of a p:sp (shape) element.
type ShapeOptions struct {
	// ID is the unique shape ID (cNvPr/@id). Required.
	ID uint32

	// Name is the shape name (cNvPr/@name). Defaults to "Shape N".
	Name string

	// Description is the alt text (cNvPr/@descr). Optional.
	Description string

	// Bounds defines position and size in EMU.
	Bounds RectEmu

	// Geometry is the preset geometry type. Required.
	Geometry PresetGeometry

	// Adjustments are optional custom adjustment handle values.
	// If empty, the shape uses its default geometry.
	Adjustments []AdjustValue

	// Fill is the shape fill. If zero value, no fill element is emitted.
	Fill Fill

	// Line is the shape outline. If zero value (Width==0 and Fill.IsZero()),
	// no line element is emitted.
	Line Line

	// Text is the optional text body. If nil, a minimal empty txBody is emitted
	// (required by the OOXML spec for p:sp elements).
	Text *TextBody

	// TxBox marks the shape as a text box (cNvSpPr txBox="1").
	TxBox bool

	// Rotation in 60,000ths of a degree (e.g. 5400000 = 90°).
	Rotation int64

	// FlipH mirrors the shape horizontally.
	FlipH bool

	// FlipV mirrors the shape vertically.
	FlipV bool

	// Effects is an optional list of DrawingML effects (shadow, glow, soft edge).
	// When non-empty, an <a:effectLst> is emitted inside p:spPr.
	Effects []Effect
}

// GenerateShape generates a complete p:sp XML element for a preset geometry shape.
//
// The generated element includes:
//   - p:nvSpPr: Non-visual properties (cNvPr, cNvSpPr, nvPr)
//   - p:spPr: Shape properties (xfrm, prstGeom, fill, line)
//   - p:txBody: Text body (from Text option, or minimal default)
func GenerateShape(opts ShapeOptions) ([]byte, error) {
	if opts.ID == 0 {
		return nil, fmt.Errorf("ShapeOptions.ID is required")
	}
	if opts.Geometry == "" {
		return nil, fmt.Errorf("ShapeOptions.Geometry is required")
	}

	name := opts.Name
	if name == "" {
		name = fmt.Sprintf("Shape %d", opts.ID)
	}

	var buf bytes.Buffer

	// p:sp
	buf.WriteString(`<p:sp>`)
	buf.WriteByte('\n')

	// --- nvSpPr ---
	buf.WriteString(`  <p:nvSpPr>`)
	buf.WriteByte('\n')

	// cNvPr
	buf.WriteString(fmt.Sprintf(`    <p:cNvPr id="%d" name="%s"`, opts.ID, escapeXMLAttr(name)))
	if opts.Description != "" {
		buf.WriteString(fmt.Sprintf(` descr="%s"`, escapeXMLAttr(opts.Description)))
	}
	buf.WriteString(`/>`)
	buf.WriteByte('\n')

	// cNvSpPr
	if opts.TxBox {
		buf.WriteString(`    <p:cNvSpPr txBox="1"/>`)
	} else {
		buf.WriteString(`    <p:cNvSpPr/>`)
	}
	buf.WriteByte('\n')

	// nvPr
	buf.WriteString(`    <p:nvPr/>`)
	buf.WriteByte('\n')

	buf.WriteString(`  </p:nvSpPr>`)
	buf.WriteByte('\n')

	// --- spPr ---
	buf.WriteString(`  <p:spPr>`)
	buf.WriteByte('\n')

	// xfrm
	buf.WriteString(`    `)
	WriteTransform(&buf, opts.Bounds, opts.Rotation, opts.FlipH, opts.FlipV)
	buf.WriteByte('\n')

	// prstGeom + avLst
	buf.WriteString(fmt.Sprintf(`    <a:prstGeom prst="%s">`, string(opts.Geometry)))
	buf.WriteByte('\n')
	if len(opts.Adjustments) > 0 {
		buf.WriteString(`      <a:avLst>`)
		buf.WriteByte('\n')
		for _, adj := range opts.Adjustments {
			buf.WriteString(fmt.Sprintf(`        <a:gd name="%s" fmla="val %d"/>`,
				escapeXMLAttr(adj.Name), adj.Value))
			buf.WriteByte('\n')
		}
		buf.WriteString(`      </a:avLst>`)
		buf.WriteByte('\n')
	} else {
		buf.WriteString(`      <a:avLst/>`)
		buf.WriteByte('\n')
	}
	buf.WriteString(`    </a:prstGeom>`)
	buf.WriteByte('\n')

	// Fill
	if !opts.Fill.IsZero() {
		buf.WriteString(`    `)
		opts.Fill.WriteTo(&buf)
		buf.WriteByte('\n')
	}

	// Line
	if opts.Line.Width > 0 || !opts.Line.Fill.IsZero() {
		buf.WriteString(`    `)
		opts.Line.WriteTo(&buf)
		buf.WriteByte('\n')
	}

	// Effects
	if len(opts.Effects) > 0 {
		buf.WriteString(`    `)
		WriteEffectList(&buf, opts.Effects)
		buf.WriteByte('\n')
	}

	buf.WriteString(`  </p:spPr>`)
	buf.WriteByte('\n')

	// --- txBody ---
	if opts.Text != nil {
		buf.WriteString(`  `)
		opts.Text.WriteTo(&buf)
		buf.WriteByte('\n')
	} else {
		// Minimal text body required by spec
		buf.WriteString(`  <p:txBody>`)
		buf.WriteByte('\n')
		buf.WriteString(`    <a:bodyPr/>`)
		buf.WriteByte('\n')
		buf.WriteString(`    <a:lstStyle/>`)
		buf.WriteByte('\n')
		buf.WriteString(`    <a:p/>`)
		buf.WriteByte('\n')
		buf.WriteString(`  </p:txBody>`)
		buf.WriteByte('\n')
	}

	buf.WriteString(`</p:sp>`)

	return buf.Bytes(), nil
}
