package generator

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestNativeStyleNamespaceExtensionsAndAttributes(t *testing.T) {
	const source = `<d:rPr xmlns:d="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:rel="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:x="urn:vendor-extension" lang="de-DE" sz="1100"><d:latin typeface="Georgia"/><x:extra xmlns:q="urn:qname" x:mode="q:choice" xml:space="preserve"> kept </x:extra><d:hlinkClick rel:id="rId1"/></d:rPr>`
	var style runPropertiesXML
	if err := xml.Unmarshal([]byte(source), &style); err != nil {
		t.Fatal(err)
	}
	if style.Lang != "de-DE" || style.FontSize != "1100" {
		t.Fatal("lost native attributes")
	}
	fragment := `<root xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + style.Inner + `</root>`
	d := xml.NewDecoder(strings.NewReader(fragment))
	foundFont, foundExtension, foundRelationship := false, false, false
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "latin" {
			foundFont = start.Name.Space == "http://schemas.openxmlformats.org/drawingml/2006/main"
		}
		if start.Name.Local == "extra" {
			foundExtension = start.Name.Space == "urn:vendor-extension"
			mode, space, binding := false, false, false
			for _, attr := range start.Attr {
				mode = mode || (attr.Name.Space == "urn:vendor-extension" && attr.Name.Local == "mode" && attr.Value == "q:choice")
				space = space || (attr.Name.Space == "http://www.w3.org/XML/1998/namespace" && attr.Name.Local == "space" && attr.Value == "preserve")
				binding = binding || (attr.Name.Space == "xmlns" && attr.Name.Local == "q" && attr.Value == "urn:qname")
			}
			if !mode || !space || !binding {
				t.Errorf("extension attributes/binding lost: %+v", start.Attr)
			}
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "id" && attr.Value == "rId1" {
				foundRelationship = attr.Name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
			}
		}
	}
	if !foundFont || !foundExtension || !foundRelationship {
		t.Errorf("font=%t extension=%t relationship=%t", foundFont, foundExtension, foundRelationship)
	}
}

func TestNativeStyleNamespaceMalformedInput(t *testing.T) {
	for _, source := range []string{`<rPr><latin></rPr>`, `<rPr><latin`, `<rPr><latin>&invalid;</latin></rPr>`} {
		var style runPropertiesXML
		if err := xml.Unmarshal([]byte(source), &style); err == nil {
			t.Errorf("accepted malformed style %q", source)
		}
	}
}
