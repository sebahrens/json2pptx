package generator

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

// decodeNativeStyle captures namespace-resolved tokens before innerxml loses
// declarations inherited from an ancestor. Known OOXML names use the prefixes
// declared by generated slides. Unknown extension namespaces remain URI-backed
// and encoding/xml emits their declarations on the elements that need them.
func decodeNativeStyle(d *xml.Decoder, start xml.StartElement, target any) error {
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	canonicalName := func(name xml.Name) xml.Name {
		switch name.Space {
		case "http://schemas.openxmlformats.org/drawingml/2006/main":
			return xml.Name{Local: "a:" + name.Local}
		case "http://schemas.openxmlformats.org/presentationml/2006/main":
			return xml.Name{Local: "p:" + name.Local}
		case "http://schemas.openxmlformats.org/officeDocument/2006/relationships":
			return xml.Name{Local: "r:" + name.Local}
		case "http://www.w3.org/XML/1998/namespace":
			return xml.Name{Local: "xml:" + name.Local}
		default:
			return name
		}
	}
	depth := 0
	var token xml.Token = start
	for {
		switch element := token.(type) {
		case xml.StartElement:
			element.Name = canonicalName(element.Name)
			attrs := make([]xml.Attr, 0, len(element.Attr)+3)
			for _, attr := range element.Attr {
				if attr.Name.Space == "xmlns" || (attr.Name.Space == "" && attr.Name.Local == "xmlns") {
					// Preserve noncanonical explicit bindings too: extension
					// attributes can carry QName values rather than XML names.
					if attr.Name.Space == "xmlns" && attr.Name.Local != "a" && attr.Name.Local != "p" && attr.Name.Local != "r" && attr.Name.Local != "xml" {
						attr.Name = xml.Name{Local: "xmlns:" + attr.Name.Local}
						attrs = append(attrs, attr)
					}
					continue
				}
				attr.Name = canonicalName(attr.Name)
				attrs = append(attrs, attr)
			}
			if depth == 0 {
				for _, binding := range []struct{ prefix, uri string }{
					{"a", "http://schemas.openxmlformats.org/drawingml/2006/main"},
					{"p", "http://schemas.openxmlformats.org/presentationml/2006/main"},
					{"r", "http://schemas.openxmlformats.org/officeDocument/2006/relationships"},
				} {
					attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "xmlns:" + binding.prefix}, Value: binding.uri})
				}
			}
			element.Attr = attrs
			token = element
			depth++
		case xml.EndElement:
			element.Name = canonicalName(element.Name)
			token = element
			depth--
		}
		if err := encoder.EncodeToken(token); err != nil {
			return fmt.Errorf("encode native style namespace: %w", err)
		}
		if depth == 0 {
			break
		}
		var err error
		token, err = d.Token()
		if err != nil {
			return fmt.Errorf("decode native style namespace: %w", err)
		}
	}
	if err := encoder.Flush(); err != nil {
		return err
	}
	fragment, err := localizeNativeQNameBindings(buf.Bytes())
	if err != nil {
		return err
	}
	return xml.Unmarshal(fragment, target)
}

func (p *bodyPropertiesXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type plain bodyPropertiesXML
	return decodeNativeStyle(d, start, (*plain)(p))
}

func (p *listStyleXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type plain listStyleXML
	return decodeNativeStyle(d, start, (*plain)(p))
}

func (p *paragraphPropertiesXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type plain paragraphPropertiesXML
	return decodeNativeStyle(d, start, (*plain)(p))
}

func (p *runPropertiesXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type plain runPropertiesXML
	return decodeNativeStyle(d, start, (*plain)(p))
}

func (p *endParaRPrXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type plain endParaRPrXML
	return decodeNativeStyle(d, start, (*plain)(p))
}
