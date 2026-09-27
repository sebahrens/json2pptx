package generator

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeCompatibilityPrefixListLocalization(t *testing.T) {
	const mc = `xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"`
	for _, tc := range []struct {
		name, source string
		unchanged    bool
		want         string
	}{
		{"ordinary literal", `<root xmlns:q="urn:q"><extra Requires="q" Ignorable="q"/></root>`, true, ""},
		{"other namespace", `<root xmlns:q="urn:q" xmlns:x="urn:x"><x:Choice Requires="q" x:Ignorable="q"/></root>`, true, ""},
		{"qualified Requires is not schema Requires", `<root ` + mc + ` xmlns:q="urn:q" xmlns:x="urn:x"><mc:Choice x:Requires="q"/></root>`, true, ""},
		{"already local", `<root ` + mc + `><mc:Choice xmlns:q="urn:q" Requires="q"/></root>`, true, ""},
		{"nested override", `<root ` + mc + ` xmlns:q="urn:outer"><scope xmlns:q="urn:inner"><mc:Choice Requires="q"/></scope></root>`, false, `<mc:Choice Requires="q" xmlns:q="urn:inner"/>`},
		{"multiple prefixes", `<root ` + mc + ` xmlns:q="urn:q" xmlns:z="urn:z"><mc:Choice Requires="q z"/></root>`, false, `<mc:Choice Requires="q z" xmlns:q="urn:q" xmlns:z="urn:z"/>`},
		{"escaped URI", `<root ` + mc + ` xmlns:q="urn:q&amp;value"><mc:Choice Requires="q"/></root>`, false, `xmlns:q="urn:q&amp;value"/>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := localizeNativeQNameBindings([]byte(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			if tc.unchanged && string(result) != tc.source {
				t.Fatalf("opaque/already-local source rewritten: %s", result)
			}
			if tc.want != "" && !strings.Contains(string(result), tc.want) {
				t.Fatalf("missing original scoped binding: %s", result)
			}
			var tree any
			if err := xml.Unmarshal(result, &tree); err != nil {
				t.Fatalf("invalid emitted XML: %v", err)
			}
		})
	}
}

func TestNativeCompatibilityPrefixListsInGeneratedPPTX(t *testing.T) {
	const mcURI = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	for _, prefix := range []string{"q", "a"} {
		for _, scope := range []string{"run", "shape", "override"} {
			t.Run(prefix+"/"+scope, func(t *testing.T) {
				declaration := fmt.Sprintf(` xmlns:%s="urn:required-feature" xmlns:z="urn:second-feature"`, prefix)
				shapeDecl, runDecl := "", declaration
				if scope == "shape" {
					shapeDecl, runDecl = declaration, ""
				}
				if scope == "override" {
					shapeDecl = fmt.Sprintf(` xmlns:%s="urn:outer-feature" xmlns:z="urn:outer-second"`, prefix)
				}
				required := prefix + " z"
				extra := fmt.Sprintf(`<mc:AlternateContent xmlns:mc="%s" mc:Ignorable="%s" mc:MustUnderstand="%s"><mc:Choice Requires="%s"><x:extra xmlns:x="urn:vendor-extension" x:literal="%s"/></mc:Choice><mc:Fallback><d:latin typeface="Georgia"/></mc:Fallback></mc:AlternateContent>`, mcURI, required, required, required, prefix)
				fixture := `<s:sp xmlns:s="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:d="http://schemas.openxmlformats.org/drawingml/2006/main"` + shapeDecl + `><s:nvSpPr><s:cNvPr id="99" name="legal_disclosure"/><s:cNvSpPr/><s:nvPr><s:ph type="subTitle" idx="7"/></s:nvPr></s:nvSpPr><s:spPr><d:xfrm><d:off x="500000" y="5600000"/><d:ext cx="11000000" cy="500000"/></d:xfrm></s:spPr><s:txBody><d:bodyPr/><d:lstStyle/><d:p><d:r><d:rPr sz="1100"` + runDecl + `>` + extra + `</d:rPr><d:t>Native prompt</d:t></d:r></d:p></s:txBody></s:sp>`
				dir := t.TempDir()
				source, output := filepath.Join(dir, "source.pptx"), filepath.Join(dir, "generated.pptx")
				writeDisclosureAlignmentTemplate(t, source, fixture)
				_, err := Generate(context.Background(), GenerationRequest{TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}, {PlaceholderID: "legal_disclosure", Type: ContentText, Value: "Required disclosure"}}}}})
				if err != nil {
					t.Fatal(err)
				}
				slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
				for _, text := range []string{"Required title", "Required disclosure"} {
					if strings.Count(slide, "<a:t>"+text+"</a:t>") != 1 {
						t.Fatalf("source text missing or duplicated: %s", text)
					}
				}
				decoder := xml.NewDecoder(strings.NewReader(slide))
				scopes := []map[string]string{}
				requires, ignorable, mustUnderstand, fallback, font, literal := 0, 0, 0, 0, 0, 0
				for {
					token, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					switch element := token.(type) {
					case xml.StartElement:
						bindings := map[string]string{}
						if len(scopes) > 0 {
							for key, value := range scopes[len(scopes)-1] {
								bindings[key] = value
							}
						}
						for _, attr := range element.Attr {
							if attr.Name.Space == "xmlns" {
								bindings[attr.Name.Local] = attr.Value
							}
						}
						scopes = append(scopes, bindings)
						if element.Name.Space == mcURI && element.Name.Local == "Fallback" {
							fallback++
						}
						if element.Name.Local == "latin" {
							if element.Name.Space != "http://schemas.openxmlformats.org/drawingml/2006/main" {
								t.Errorf("fallback font namespace changed: %v", element.Name)
							}
							for _, attr := range element.Attr {
								if attr.Name.Local == "typeface" && attr.Value == "Georgia" {
									font++
								}
							}
						}
						for _, attr := range element.Attr {
							prefixList := false
							if element.Name.Space == mcURI && element.Name.Local == "Choice" && attr.Name.Space == "" && attr.Name.Local == "Requires" {
								requires++
								prefixList = true
							}
							if attr.Name.Space == mcURI && attr.Name.Local == "Ignorable" {
								ignorable++
								prefixList = true
							}
							if attr.Name.Space == mcURI && attr.Name.Local == "MustUnderstand" {
								mustUnderstand++
								prefixList = true
							}
							if prefixList && (attr.Value != required || bindings[prefix] != "urn:required-feature" || bindings["z"] != "urn:second-feature") {
								t.Errorf("prefix-list semantics changed: %v, scope %v", attr, bindings)
							}
							if attr.Name.Space == "urn:vendor-extension" && attr.Name.Local == "literal" {
								literal++
								if attr.Value != prefix {
									t.Errorf("opaque literal changed: %q", attr.Value)
								}
							}
						}
					case xml.EndElement:
						scopes = scopes[:len(scopes)-1]
					}
				}
				if requires != 1 || ignorable != 1 || mustUnderstand != 1 || fallback != 1 || font != 1 || literal != 1 {
					t.Fatalf("lost/duplicate evidence: %d/%d/%d/%d/%d/%d", requires, ignorable, mustUnderstand, fallback, font, literal)
				}
			})
		}
	}
}

func TestNativeQNameBindingsInGeneratedPPTX(t *testing.T) {
	for _, scope := range []string{"run", "shape", "override"} {
		t.Run(scope, func(t *testing.T) {
			declaration := ` xmlns:q="urn:required-choice"`
			shapeDecl, runDecl := "", declaration
			if scope == "shape" {
				shapeDecl, runDecl = declaration, ""
			}
			if scope == "override" {
				shapeDecl = ` xmlns:q="urn:outer-choice"`
			}
			fixture := `<p:sp` + shapeDecl + `><p:nvSpPr><p:cNvPr id="99" name="legal_disclosure"/><p:cNvSpPr/><p:nvPr><p:ph type="subTitle" idx="7"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="500000" y="5600000"/><a:ext cx="11000000" cy="500000"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr sz="1100"` + runDecl + `><a:latin typeface="Georgia"/><a:extLst><a:ext uri="urn:test"><x:extra xmlns:x="urn:vendor-extension" x:mode="q:choice"/></a:ext></a:extLst></a:rPr><a:t>Native prompt</a:t></a:r></a:p></p:txBody></p:sp>`
			dir := t.TempDir()
			source, output := filepath.Join(dir, "source.pptx"), filepath.Join(dir, "generated.pptx")
			writeDisclosureAlignmentTemplate(t, source, fixture)
			_, err := Generate(context.Background(), GenerationRequest{TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}, {PlaceholderID: "legal_disclosure", Type: ContentText, Value: "Required disclosure"}}}}})
			if err != nil {
				t.Fatal(err)
			}
			slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
			if !strings.Contains(slide, "Required disclosure") {
				t.Fatal("source missing")
			}
			decoder := xml.NewDecoder(strings.NewReader(slide))
			scopes := []map[string]string{}
			found := false
			for {
				tok, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				switch element := tok.(type) {
				case xml.StartElement:
					bindings := map[string]string{}
					if len(scopes) > 0 {
						for prefix, uri := range scopes[len(scopes)-1] {
							bindings[prefix] = uri
						}
					}
					for _, attr := range element.Attr {
						if attr.Name.Space == "xmlns" {
							bindings[attr.Name.Local] = attr.Value
						}
					}
					scopes = append(scopes, bindings)
					if element.Name.Space == "urn:vendor-extension" && element.Name.Local == "extra" {
						for _, attr := range element.Attr {
							if attr.Name.Space == "urn:vendor-extension" && attr.Name.Local == "mode" && attr.Value == "q:choice" {
								found = true
								if bindings["q"] != "urn:required-choice" {
									t.Errorf("emitted QName value %q lost required binding: %v", attr.Value, bindings)
								}
							}
						}
					}
				case xml.EndElement:
					scopes = scopes[:len(scopes)-1]
				}
			}
			if !found {
				t.Fatal("extension or its QName-valued attribute lost")
			}
		})
	}
}

func TestNativeReservedQNameScopeReproduction(t *testing.T) {
	for _, prefix := range []string{"a", "p", "r"} {
		for _, scope := range []string{"run", "shape", "override"} {
			t.Run(prefix+"/"+scope, func(t *testing.T) {
				declaration := fmt.Sprintf(" xmlns:%s=\"urn:required-choice\"", prefix)
				shapeDecl, runDecl := "", declaration
				if scope == "shape" {
					shapeDecl, runDecl = declaration, ""
				}
				if scope == "override" {
					shapeDecl = fmt.Sprintf(" xmlns:%s=\"urn:outer-choice\"", prefix)
				}
				extra := fmt.Sprintf("<x:extra xmlns:x=\"urn:vendor-extension\" xmlns:native_%s1=\"urn:existing-alias\" x:mode=\"%s:choice\" x:other=\"native_%s1:other\" x:uri=\"https://example.invalid/a:choice\" x:literal=\"%s:choice\"/>", prefix, prefix, prefix, prefix)
				extra = strings.Replace(extra, " x:mode=", ` xmlns:rel="http://schemas.openxmlformats.org/officeDocument/2006/relationships" rel:id="rId1" x:mode=`, 1)
				fixture := `<s:sp xmlns:s="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:d="http://schemas.openxmlformats.org/drawingml/2006/main"` + shapeDecl + `><s:nvSpPr><s:cNvPr id="99" name="legal_disclosure"/><s:cNvSpPr/><s:nvPr><s:ph type="subTitle" idx="7"/></s:nvPr></s:nvSpPr><s:spPr><d:xfrm><d:off x="500000" y="5600000"/><d:ext cx="11000000" cy="500000"/></d:xfrm></s:spPr><s:txBody><d:bodyPr/><d:lstStyle/><d:p><d:r><d:rPr sz="1100"` + runDecl + `><d:latin typeface="Georgia"/><d:extLst><d:ext uri="urn:test">` + extra + `</d:ext></d:extLst></d:rPr><d:t>Native prompt</d:t></d:r></d:p></s:txBody></s:sp>`
				dir := t.TempDir()
				source, output := filepath.Join(dir, "source.pptx"), filepath.Join(dir, "generated.pptx")
				writeDisclosureAlignmentTemplate(t, source, fixture)
				_, err := Generate(context.Background(), GenerationRequest{TemplatePath: source, OutputPath: output, ExcludeTemplateSlides: true, Slides: []SlideSpec{{LayoutID: "slideLayout7", Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Required title"}, {PlaceholderID: "legal_disclosure", Type: ContentText, Value: "Required disclosure"}}}}})
				if err != nil {
					t.Fatal(err)
				}
				slide := readZipFileString(t, output, "ppt/slides/slide1.xml")
				for _, text := range []string{"Required title", "Required disclosure"} {
					if strings.Count(slide, "<a:t>"+text+"</a:t>") != 1 {
						t.Fatalf("source text missing or duplicated: %s", text)
					}
				}
				decoder := xml.NewDecoder(strings.NewReader(slide))
				scopes := []map[string]string{}
				found, choice, other, url, literal, font, relationship := 0, 0, 0, 0, 0, 0, 0
				for {
					tok, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					switch element := tok.(type) {
					case xml.StartElement:
						bindings := map[string]string{}
						if len(scopes) > 0 {
							for key, uri := range scopes[len(scopes)-1] {
								bindings[key] = uri
							}
						}
						for _, attr := range element.Attr {
							if attr.Name.Space == "xmlns" {
								bindings[attr.Name.Local] = attr.Value
							}
						}
						scopes = append(scopes, bindings)
						if element.Name.Local == "latin" {
							if element.Name.Space != "http://schemas.openxmlformats.org/drawingml/2006/main" {
								t.Errorf("font namespace lost: %v", element.Name)
							}
							for _, attr := range element.Attr {
								if attr.Name.Local == "typeface" && attr.Value == "Georgia" {
									font++
								}
							}
						}
						if element.Name.Space == "urn:vendor-extension" && element.Name.Local == "extra" {
							// Relationship IDs must retain their standard namespace
							// even when the conventional r prefix is extension-bound.
							for _, attr := range element.Attr {
								if attr.Name.Local == "id" && attr.Value == "rId1" {
									relationship++
									if attr.Name.Space != "http://schemas.openxmlformats.org/officeDocument/2006/relationships" {
										t.Errorf("relationship namespace lost: %v", attr.Name)
									}
								}
							}
							found++
							for _, attr := range element.Attr {
								if attr.Name.Space != "urn:vendor-extension" {
									continue
								}
								switch attr.Name.Local {
								case "mode":
									choice++
									head, name, ok := strings.Cut(attr.Value, ":")
									if !ok || name != "choice" || bindings[head] != "urn:required-choice" {
										t.Errorf("wrong expanded choice QName %q: %v", attr.Value, bindings)
									}
								case "other":
									other++
									head, name, ok := strings.Cut(attr.Value, ":")
									if !ok || name != "other" || bindings[head] != "urn:existing-alias" {
										t.Errorf("alias collision lost existing QName %q: %v", attr.Value, bindings)
									}
								case "literal":
									literal++
									if attr.Value != prefix+":choice" {
										t.Errorf("opaque literal value rewritten: %q", attr.Value)
									}
								case "uri":
									url++
									if attr.Value != "https://example.invalid/a:choice" {
										t.Errorf("URI value rewritten: %q", attr.Value)
									}
								}
							}
						}
					case xml.EndElement:
						scopes = scopes[:len(scopes)-1]
					}
				}
				if found != 1 || choice != 1 || other != 1 || url != 1 || literal != 1 || font != 1 || relationship != 1 {
					t.Fatalf("missing/duplicate extension evidence: %d/%d/%d/%d/%d/%d/%d", found, choice, other, url, literal, font, relationship)
				}
			})
		}
	}
}
