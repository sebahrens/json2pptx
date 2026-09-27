package generator

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"strings"
)

// localizeNativeQNameBindings preserves inherited prefix bindings used in
// attribute values. encoding/xml resolves element/attribute names, but leaves
// QName values opaque; native-style innerxml subsequently drops their ancestors.
// Insert declarations at the value's element, without reserializing the source.
func localizeNativeQNameBindings(source []byte) ([]byte, error) {
	d := xml.NewDecoder(bytes.NewReader(source))
	scopes := []map[string]string{}
	var out bytes.Buffer
	copied := 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("resolve native QName scope: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			bindings := map[string]string{}
			if len(scopes) > 0 {
				bindings = maps.Clone(scopes[len(scopes)-1])
			}
			local := map[string]bool{}
			for _, attr := range element.Attr {
				if attr.Name.Space == "xmlns" {
					bindings[attr.Name.Local] = attr.Value
					local[attr.Name.Local] = true
				}
			}
			scopes = append(scopes, bindings)
			declarations, err := nativeQNameDeclarations(element.Attr, bindings, local)
			if err != nil {
				return nil, err
			}
			if len(declarations) > 0 {
				// InputOffset points just after this start tag, including '/>'
				// for empty elements. Keep all original bytes and quote styles.
				at := int(d.InputOffset()) - 1
				if source[at-1] == '/' {
					at--
				}
				out.Write(source[copied:at])
				out.Write(declarations)
				copied = at
			}
		case xml.EndElement:
			scopes = scopes[:len(scopes)-1]
		}
	}
	if copied == 0 {
		return source, nil
	}
	out.Write(source[copied:])
	return out.Bytes(), nil
}

func nativeQNameDeclarations(attrs []xml.Attr, bindings map[string]string, local map[string]bool) ([]byte, error) {
	var declarations bytes.Buffer
	for _, attr := range attrs {
		if attr.Name.Space == "xmlns" || attr.Name.Local == "xmlns" {
			continue
		}
		for _, value := range strings.Fields(attr.Value) {
			prefix, name, ok := strings.Cut(value, ":")
			uri := bindings[prefix]
			if !ok || name == "" || strings.ContainsAny(name, ":/") || uri == "" || local[prefix] || prefix == "xml" {
				continue
			}
			declarations.WriteString(` xmlns:` + prefix + `="`)
			if err := xml.EscapeText(&declarations, []byte(uri)); err != nil {
				return nil, fmt.Errorf("escape native QName binding: %w", err)
			}
			declarations.WriteByte('"')
			local[prefix] = true
		}
	}
	return declarations.Bytes(), nil
}
