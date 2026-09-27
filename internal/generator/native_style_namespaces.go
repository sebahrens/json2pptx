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
	// Buffer resolved tokens before choosing names: a descendant can legally
	// reserve a conventional OOXML prefix for an extension QName or literal.
	bindings := []struct{ prefix, uri string }{
		{"a", "http://schemas.openxmlformats.org/drawingml/2006/main"},
		{"p", "http://schemas.openxmlformats.org/presentationml/2006/main"},
		{"r", "http://schemas.openxmlformats.org/officeDocument/2006/relationships"},
	}
	tokens, err := captureNativeStyleTokens(d, start)
	if err != nil {
		return err
	}
	prefixes, conflicts := nativeStylePrefixes(tokens, bindings)
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	depth := 0
	for _, token := range tokens {
		switch element := token.(type) {
		case xml.StartElement:
			element = nativeStyleStartElement(element, bindings, prefixes, conflicts, depth == 0)
			token = element
			depth++
		case xml.EndElement:
			element.Name = nativeStyleName(element.Name, prefixes)
			token = element
			depth--
		}
		if err := encoder.EncodeToken(token); err != nil {
			return fmt.Errorf("encode native style namespace: %w", err)
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

func nativeStyleName(name xml.Name, prefixes map[string]string) xml.Name {
	if prefix, ok := prefixes[name.Space]; ok {
		return xml.Name{Local: prefix + ":" + name.Local}
	}
	switch name.Space {
	case "http://www.w3.org/XML/1998/namespace":
		return xml.Name{Local: "xml:" + name.Local}
	default:
		return name
	}
}

func nativeStyleStartElement(element xml.StartElement, bindings []struct{ prefix, uri string }, prefixes map[string]string, conflicts map[string]bool, root bool) xml.StartElement {
	element.Name = nativeStyleName(element.Name, prefixes)
	attrs := make([]xml.Attr, 0, len(element.Attr)+3)
	for _, attr := range element.Attr {
		if attr.Name.Space == "xmlns" || (attr.Name.Space == "" && attr.Name.Local == "xmlns") {
			// Preserve noncanonical explicit bindings too: extension
			// attributes can carry QName values rather than XML names.
			if attr.Name.Space == "xmlns" && attr.Name.Local != "xml" && (conflicts[attr.Name.Local] || (attr.Name.Local != "a" && attr.Name.Local != "p" && attr.Name.Local != "r")) {
				attr.Name = xml.Name{Local: "xmlns:" + attr.Name.Local}
				attrs = append(attrs, attr)
			}
			continue
		}
		attr.Name = nativeStyleName(attr.Name, prefixes)
		attrs = append(attrs, attr)
	}
	for _, binding := range bindings {
		// Fresh prefixes need local declarations: typed outer properties
		// can discard their root declarations when marshaling innerxml.
		if root || conflicts[binding.prefix] {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "xmlns:" + prefixes[binding.uri]}, Value: binding.uri})
		}
	}
	element.Attr = attrs
	return element
}

func captureNativeStyleTokens(d *xml.Decoder, start xml.StartElement) ([]xml.Token, error) {
	tokens := []xml.Token{}
	depth := 0
	var token xml.Token = start
	for {
		// Decoder character-data storage is reused on subsequent reads.
		tokens = append(tokens, xml.CopyToken(token))
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
		if depth == 0 {
			return tokens, nil
		}
		var err error
		token, err = d.Token()
		if err != nil {
			return nil, fmt.Errorf("decode native style namespace: %w", err)
		}
	}
}

func nativeStylePrefixes(tokens []xml.Token, bindings []struct{ prefix, uri string }) (map[string]string, map[string]bool) {
	used, conflicts := map[string]bool{}, map[string]bool{}
	for _, token := range tokens {
		element, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range element.Attr {
			if attr.Name.Space != "xmlns" {
				continue
			}
			used[attr.Name.Local] = true
			for _, binding := range bindings {
				if attr.Name.Local == binding.prefix && attr.Value != binding.uri {
					conflicts[binding.prefix] = true
				}
			}
		}
	}
	prefixes := map[string]string{}
	for _, binding := range bindings {
		prefix := binding.prefix
		if conflicts[prefix] {
			for index := 1; ; index++ {
				candidate := fmt.Sprintf("native_%s%d", binding.prefix, index)
				if !used[candidate] {
					prefix = candidate
					break
				}
			}
		}
		used[prefix] = true
		prefixes[binding.uri] = prefix
	}
	return prefixes, conflicts
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
