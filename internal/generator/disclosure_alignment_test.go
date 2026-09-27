package generator

import (
	"strconv"
	"strings"
	"testing"
)

func TestDisclosurePreservesNativeAlignmentAcrossTextTypes(t *testing.T) {
	for _, phType := range []string{"subTitle", "body", "obj", ""} {
		for _, name := range []string{"legal_disclosure", " Legal Disclosure "} {
			for _, id := range []string{"legal_disclosure", "idx:7"} {
				for _, alignment := range []string{"ctr", "r"} {
					for _, explicit := range []bool{false, true} {
						t.Run(strings.Join([]string{phType, name, id, alignment}, "/")+"/explicit="+strconv.FormatBool(explicit), func(t *testing.T) {
							shape := makeShape(name, phType, intPtr(7), 1000000, 4800000, 6000000, 1000000)
							shape.TextBody = &textBodyXML{
								BodyProperties: &bodyPropertiesXML{Anchor: "b"},
								ListStyle:      &listStyleXML{Inner: `<a:lvl1pPr algn="` + alignment + `"><a:defRPr sz="1100"/></a:lvl1pPr>`},
								Paragraphs:     []paragraphXML{{Runs: []runXML{{Text: "Native disclosure prompt"}}}},
							}
							want := ""
							if explicit {
								// The paragraph overrides the opposite list alignment.
								want = map[string]string{"ctr": "r", "r": "ctr"}[alignment]
								shape.TextBody.Paragraphs[0].Properties = &paragraphPropertiesXML{Algn: want}
							}
							original := shape.TextBody.Paragraphs[0].Properties
							if err := populateShapeText(&shape, ContentItem{PlaceholderID: id, Type: ContentText, Value: "Required disclosure first line\nRequired disclosure second line"}, -1, ""); err != nil {
								t.Fatal(err)
							}
							if len(shape.TextBody.Paragraphs) != 2 {
								t.Fatal("required disclosure paragraphs lost")
							}
							for _, paragraph := range shape.TextBody.Paragraphs {
								if paragraph.Properties == nil || paragraph.Properties.Algn != want {
									t.Fatalf("alignment=%+v; want %q (empty inherits native list)", paragraph.Properties, want)
								}
							}
							if original != nil && original.Algn != want {
								t.Fatal("native paragraph properties mutated")
							}
							if extractFontSizeFromShape(&shape) != 1100 || shape.TextBody.BodyProperties.Anchor != "b" {
								t.Fatal("native disclosure font or anchoring changed")
							}
						})
					}
				}
			}
		}
	}
}

func TestDisclosureAlignmentExceptionRequiresExplicitTextRole(t *testing.T) {
	for _, tc := range []struct {
		name, id, phType string
		wantFont         int
	}{
		{"ordinary body", "body", "body", 1800},
		{"legal notes", "legal notes", "body", 1800},
		{"legal_disclosures", "legal_disclosure", "obj", 1800},
		{"legal_disclosure", "legal_disclosure", "pic", 1200},
		{"legal_disclosure", "legal_disclosure", "chart", 1200},
	} {
		t.Run(tc.name+"/"+tc.phType, func(t *testing.T) {
			shape := makeShape(tc.name, tc.phType, intPtr(7), 1000000, 4800000, 6000000, 1000000)
			shape.TextBody = &textBodyXML{
				BodyProperties: &bodyPropertiesXML{},
				ListStyle:      &listStyleXML{Inner: `<a:lvl1pPr algn="r"><a:defRPr sz="1100"/></a:lvl1pPr>`},
				Paragraphs:     []paragraphXML{{Properties: &paragraphPropertiesXML{Algn: "ctr"}, Runs: []runXML{{Text: "Native prompt"}}}},
			}
			if err := populateShapeText(&shape, ContentItem{PlaceholderID: tc.id, Type: ContentText, Value: "Required ordinary source"}, -1, ""); err != nil {
				t.Fatal(err)
			}
			if p := shape.TextBody.Paragraphs[0].Properties; p == nil || p.Algn != "l" {
				t.Fatalf("ordinary left alignment changed: %+v", p)
			}
			if got := extractFontSizeFromShape(&shape); got != tc.wantFont {
				t.Fatalf("ordinary type size=%d; want %d", got, tc.wantFont)
			}
		})
	}
}
