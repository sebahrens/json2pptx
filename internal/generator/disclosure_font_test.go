package generator

import "testing"

func TestDisclosureTextPreservesNativeFont(t *testing.T) {
	shape := makeShape("legal_disclosure", "subTitle", intPtr(1), 5000000, 4800000, 6000000, 1600000)
	shape.TextBody = &textBodyXML{BodyProperties: &bodyPropertiesXML{Anchor: "b"}, ListStyle: &listStyleXML{Inner: `<a:lvl1pPr algn="l"><a:defRPr sz="1100"/></a:lvl1pPr>`}}
	if err := setTextParagraph(&shape, "legal_disclosure", "Provided legal disclosure", 2800, "Georgia"); err != nil {
		t.Fatal(err)
	}
	if got := extractFontSizeFromShape(&shape); got != 1100 {
		t.Fatalf("native legal font enlarged from11pt to%.2fpt", float64(got)/100)
	}
	if shape.TextBody.BodyProperties.Anchor != "b" {
		t.Fatal("legal anchoring changed")
	}
}

func TestDisclosureFontExceptionDoesNotLowerOrdinaryText(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		phType string
		want   int
	}{
		{"explicit underscore", "legal_disclosure", "subTitle", 1100},
		{"explicit spaces", "Legal Disclosure", "body", 1100},
		{"ordinary subtitle", "subtitle", "subTitle", 1200},
		{"ordinary body", "body", "body", 1200},
		{"incidental legal name", "legal notes", "body", 1200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shape := makeShape(tc.id, tc.phType, intPtr(1), 5000000, 4800000, 6000000, 1600000)
			shape.TextBody = &textBodyXML{BodyProperties: &bodyPropertiesXML{Anchor: "b"}, ListStyle: &listStyleXML{Inner: `<a:lvl1pPr algn="l"><a:defRPr sz="1100"/></a:lvl1pPr>`}}
			if err := setTextParagraph(&shape, tc.id, "Required source", 2800, "Georgia"); err != nil {
				t.Fatal(err)
			}
			if got := extractFontSizeFromShape(&shape); got != tc.want {
				t.Fatalf("font size=%d, want %d", got, tc.want)
			}
		})
	}
}
