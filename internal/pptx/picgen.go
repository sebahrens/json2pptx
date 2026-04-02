// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"bytes"
	"fmt"
)

// SVGBlipExtensionURI is the GUID for the SVG blip extension in Office 2016+.
// This extension allows embedding SVG with PNG fallback.
const SVGBlipExtensionURI = "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"

// SVGBlipNamespace is the XML namespace for asvg:svgBlip elements.
const SVGBlipNamespace = "http://schemas.microsoft.com/office/drawing/2016/SVG/main"

// PicOptions configures the generation of a p:pic element.
type PicOptions struct {
	// ID is the unique shape ID (cNvPr/@id). Required.
	ID uint32

	// Name is the shape name (cNvPr/@name). Defaults to "Picture N".
	Name string

	// Description is the alt text (cNvPr/@descr). Optional.
	Description string

	// PNGRelID is the relationship ID for the PNG image (r:embed on a:blip). Required.
	PNGRelID string

	// SVGRelID is the relationship ID for the SVG image. Optional.
	// If provided, an asvg:svgBlip extension is added to the blip.
	SVGRelID string

	// Position and size in EMUs
	OffsetX, OffsetY   int64 // a:off x, y
	ExtentCX, ExtentCY int64 // a:ext cx, cy

	// OmitNamespaces omits xmlns declarations from the p:pic element.
	// Use this when inserting into a document that already declares the namespaces.
	OmitNamespaces bool
}

// GeneratePic generates a complete p:pic XML element for embedding an image.
//
// The generated element includes:
// - nvPicPr: Non-visual properties with ID, name, and optional description
// - blipFill: Image reference with optional SVG extension
// - spPr: Shape properties with position and size
//
// If SVGRelID is provided, the blip will include an extension for SVG fallback:
//
//	<a:blip r:embed="[pngRelID]">
//	  <a:extLst>
//	    <a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">
//	      <asvg:svgBlip xmlns:asvg="..." r:embed="[svgRelID]"/>
//	    </a:ext>
//	  </a:extLst>
//	</a:blip>
func GeneratePic(opts PicOptions) ([]byte, error) {
	if opts.ID == 0 {
		return nil, fmt.Errorf("PicOptions.ID is required")
	}
	if opts.PNGRelID == "" {
		return nil, fmt.Errorf("PicOptions.PNGRelID is required")
	}

	// Default name
	name := opts.Name
	if name == "" {
		name = fmt.Sprintf("Picture %d", opts.ID)
	}

	var buf bytes.Buffer

	// Start p:pic - optionally with namespaces
	if opts.OmitNamespaces {
		buf.WriteString(`<p:pic>`)
	} else {
		buf.WriteString(`<p:pic xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" `)
		buf.WriteString(`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" `)
		buf.WriteString(`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	}
	buf.WriteByte('\n')

	// nvPicPr
	buf.WriteString(`  <p:nvPicPr>`)
	buf.WriteByte('\n')

	// cNvPr
	buf.WriteString(fmt.Sprintf(`    <p:cNvPr id="%d" name="%s"`, opts.ID, escapeXMLAttr(name)))
	if opts.Description != "" {
		buf.WriteString(fmt.Sprintf(` descr="%s"`, escapeXMLAttr(opts.Description)))
	}
	buf.WriteString(`/>`)
	buf.WriteByte('\n')

	// cNvPicPr
	buf.WriteString(`    <p:cNvPicPr>`)
	buf.WriteByte('\n')
	buf.WriteString(`      <a:picLocks noChangeAspect="1"/>`)
	buf.WriteByte('\n')
	buf.WriteString(`    </p:cNvPicPr>`)
	buf.WriteByte('\n')

	// nvPr (required but can be empty)
	buf.WriteString(`    <p:nvPr/>`)
	buf.WriteByte('\n')

	buf.WriteString(`  </p:nvPicPr>`)
	buf.WriteByte('\n')

	// blipFill
	buf.WriteString(`  <p:blipFill>`)
	buf.WriteByte('\n')

	// a:blip with potential SVG extension
	if opts.SVGRelID != "" {
		// Blip with SVG extension
		buf.WriteString(fmt.Sprintf(`    <a:blip r:embed="%s">`, opts.PNGRelID))
		buf.WriteByte('\n')
		buf.WriteString(`      <a:extLst>`)
		buf.WriteByte('\n')
		buf.WriteString(fmt.Sprintf(`        <a:ext uri="%s">`, SVGBlipExtensionURI))
		buf.WriteByte('\n')
		buf.WriteString(fmt.Sprintf(`          <asvg:svgBlip xmlns:asvg="%s" r:embed="%s"/>`,
			SVGBlipNamespace, opts.SVGRelID))
		buf.WriteByte('\n')
		buf.WriteString(`        </a:ext>`)
		buf.WriteByte('\n')
		buf.WriteString(`      </a:extLst>`)
		buf.WriteByte('\n')
		buf.WriteString(`    </a:blip>`)
		buf.WriteByte('\n')
	} else {
		// Simple blip without extensions
		buf.WriteString(fmt.Sprintf(`    <a:blip r:embed="%s"/>`, opts.PNGRelID))
		buf.WriteByte('\n')
	}

	// stretch with fillRect (matching working template pattern)
	buf.WriteString(`    <a:stretch>`)
	buf.WriteByte('\n')
	buf.WriteString(`      <a:fillRect/>`)
	buf.WriteByte('\n')
	buf.WriteString(`    </a:stretch>`)
	buf.WriteByte('\n')

	buf.WriteString(`  </p:blipFill>`)
	buf.WriteByte('\n')

	// spPr
	buf.WriteString(`  <p:spPr>`)
	buf.WriteByte('\n')

	// xfrm (transform) if position/size provided
	if opts.ExtentCX > 0 || opts.ExtentCY > 0 {
		buf.WriteString(`    <a:xfrm>`)
		buf.WriteByte('\n')
		buf.WriteString(fmt.Sprintf(`      <a:off x="%d" y="%d"/>`, opts.OffsetX, opts.OffsetY))
		buf.WriteByte('\n')
		buf.WriteString(fmt.Sprintf(`      <a:ext cx="%d" cy="%d"/>`, opts.ExtentCX, opts.ExtentCY))
		buf.WriteByte('\n')
		buf.WriteString(`    </a:xfrm>`)
		buf.WriteByte('\n')

		// prstGeom for rectangle shape
		buf.WriteString(`    <a:prstGeom prst="rect">`)
		buf.WriteByte('\n')
		buf.WriteString(`      <a:avLst/>`)
		buf.WriteByte('\n')
		buf.WriteString(`    </a:prstGeom>`)
		buf.WriteByte('\n')
	}

	buf.WriteString(`  </p:spPr>`)
	buf.WriteByte('\n')

	buf.WriteString(`</p:pic>`)

	return buf.Bytes(), nil
}

// GeneratePicSimple generates a simple p:pic element with minimal options.
// This is a convenience wrapper for common use cases.
// Includes namespace declarations for standalone use.
func GeneratePicSimple(id uint32, pngRelID string, x, y, cx, cy int64) ([]byte, error) {
	return GeneratePic(PicOptions{
		ID:       id,
		PNGRelID: pngRelID,
		OffsetX:  x,
		OffsetY:  y,
		ExtentCX: cx,
		ExtentCY: cy,
	})
}

// GeneratePicSimpleNoNS generates a simple p:pic element without namespace declarations.
// Use this when inserting into a slide that already has namespaces declared at the root.
func GeneratePicSimpleNoNS(id uint32, pngRelID string, x, y, cx, cy int64) ([]byte, error) {
	return GeneratePic(PicOptions{
		ID:             id,
		PNGRelID:       pngRelID,
		OffsetX:        x,
		OffsetY:        y,
		ExtentCX:       cx,
		ExtentCY:       cy,
		OmitNamespaces: true,
	})
}

// GeneratePicWithSVG generates a p:pic element with SVG fallback.
// This creates the structure needed for Office 2016+ SVG support with PNG fallback
// for older versions.
func GeneratePicWithSVG(id uint32, pngRelID, svgRelID string, x, y, cx, cy int64) ([]byte, error) {
	return GeneratePic(PicOptions{
		ID:       id,
		PNGRelID: pngRelID,
		SVGRelID: svgRelID,
		OffsetX:  x,
		OffsetY:  y,
		ExtentCX: cx,
		ExtentCY: cy,
	})
}

// escapeXMLAttr escapes special characters for XML attribute values.
func escapeXMLAttr(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString("&quot;")
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
