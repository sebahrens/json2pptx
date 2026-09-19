package pptx

import (
	"bytes"
	"fmt"
	"strings"
)

// GroupOptions configures the generation of a p:grpSp (group shape) element.
type GroupOptions struct {
	// ID is the unique shape ID (cNvPr/@id). Required.
	ID uint32

	// Name is the group name (cNvPr/@name). Defaults to "Group N".
	Name string

	// Description is the alt text (cNvPr/@descr). Optional.
	Description string

	// Bounds defines the slide-space position and size in EMU.
	Bounds RectEmu

	// Children contains pre-generated child XML elements (p:sp, p:pic, p:cxnSp, p:grpSp).
	// Each entry must be a well-formed XML fragment.
	Children [][]byte
}

// GenerateGroup generates a complete p:grpSp XML element with identity child transform.
//
// The child coordinate space equals the slide-space bounds (chOff=off, chExt=ext),
// meaning children use the same coordinate system as the group's position on the slide.
func GenerateGroup(opts GroupOptions) ([]byte, error) {
	return GenerateGroupWithChildSpace(opts, opts.Bounds)
}

// GenerateGroupWithChildSpace generates a p:grpSp with an explicit child coordinate space.
//
// The childBounds parameter defines the coordinate space for child elements:
//   - childBounds.X, childBounds.Y → a:chOff (child offset origin)
//   - childBounds.CX, childBounds.CY → a:chExt (child extent)
//
// This allows children to use a different coordinate system than the group's
// slide-space position, enabling scaling and translation of grouped content.
func GenerateGroupWithChildSpace(opts GroupOptions, childBounds RectEmu) ([]byte, error) {
	if opts.ID == 0 {
		return nil, fmt.Errorf("GroupOptions.ID is required")
	}

	name := opts.Name
	if name == "" {
		name = fmt.Sprintf("Group %d", opts.ID)
	}

	var buf bytes.Buffer

	// p:grpSp
	buf.WriteString(`<p:grpSp>`)
	buf.WriteByte('\n')

	// --- nvGrpSpPr ---
	buf.WriteString(`  <p:nvGrpSpPr>`)
	buf.WriteByte('\n')

	// cNvPr
	fmt.Fprintf(&buf, `    <p:cNvPr id="%d" name="%s"`, opts.ID, escapeXMLAttr(name))
	if opts.Description != "" {
		fmt.Fprintf(&buf, ` descr="%s"`, escapeXMLAttr(opts.Description))
	}
	buf.WriteString(`/>`)
	buf.WriteByte('\n')

	// cNvGrpSpPr
	buf.WriteString(`    <p:cNvGrpSpPr/>`)
	buf.WriteByte('\n')

	// nvPr
	buf.WriteString(`    <p:nvPr/>`)
	buf.WriteByte('\n')

	buf.WriteString(`  </p:nvGrpSpPr>`)
	buf.WriteByte('\n')

	// --- grpSpPr ---
	buf.WriteString(`  <p:grpSpPr>`)
	buf.WriteByte('\n')

	// xfrm with child offset/extent
	buf.WriteString(`    <a:xfrm>`)
	fmt.Fprintf(&buf, `<a:off x="%d" y="%d"/>`, opts.Bounds.X, opts.Bounds.Y)
	fmt.Fprintf(&buf, `<a:ext cx="%d" cy="%d"/>`, opts.Bounds.CX, opts.Bounds.CY)
	fmt.Fprintf(&buf, `<a:chOff x="%d" y="%d"/>`, childBounds.X, childBounds.Y)
	fmt.Fprintf(&buf, `<a:chExt cx="%d" cy="%d"/>`, childBounds.CX, childBounds.CY)
	buf.WriteString(`</a:xfrm>`)
	buf.WriteByte('\n')

	buf.WriteString(`  </p:grpSpPr>`)
	buf.WriteByte('\n')

	// --- Children ---
	for _, child := range opts.Children {
		buf.WriteString(`  `)
		buf.Write(child)
		buf.WriteByte('\n')
	}

	buf.WriteString(`</p:grpSp>`)

	return buf.Bytes(), nil
}

// SetGroupDescription writes description into a generated group's own cNvPr as
// the descr attribute (alt text), returning the group XML unchanged when the
// description is empty or a descr is already present.
//
// The alt text is applied here rather than passed through GroupOptions because
// the thirteen native diagram group builders in internal/generator assemble
// their children long before the DiagramSpec's description is resolved, and a
// group whose cNvPr has no descr announces nothing at all to a screen reader
// (go-slide-creator-6e8h).
func SetGroupDescription(groupXML string, description string) string {
	if description == "" || groupXML == "" {
		return groupXML
	}
	const open = `<p:cNvPr `
	start := strings.Index(groupXML, open)
	if start < 0 {
		return groupXML
	}
	end := strings.Index(groupXML[start:], `/>`)
	if end < 0 {
		return groupXML
	}
	end += start
	if strings.Contains(groupXML[start:end], ` descr="`) {
		return groupXML
	}
	return groupXML[:end] + fmt.Sprintf(` descr="%s"`, escapeXMLAttr(description)) + groupXML[end:]
}
