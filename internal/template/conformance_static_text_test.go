package template

import (
	"strings"
	"testing"
)

func TestStaticTextShapes(t *testing.T) {
	const prefix = `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>`
	const suffix = `</p:spTree></p:cSld></p:sldLayout>`
	tests := []struct {
		name    string
		shapes  string
		want    []staticTextShape
		wantErr bool
	}{
		{
			name:   "ordinary and field text warn",
			shapes: `<p:sp><p:nvSpPr><p:cNvPr name="Brand"/><p:nvPr/></p:nvSpPr><p:txBody><a:p><a:r><a:t>My</a:t></a:r><a:fld><a:t>Company</a:t></a:fld></a:p></p:txBody></p:sp>`,
			want:   []staticTextShape{{name: "Brand", text: "My Company"}},
		},
		{
			name:   "placeholder sample text is ignored",
			shapes: `<p:sp><p:nvSpPr><p:cNvPr name="Body"/><p:nvPr><p:ph type="body"/></p:nvPr></p:nvSpPr><p:txBody><a:p><a:r><a:t>Click to edit</a:t></a:r></a:p></p:txBody></p:sp>`,
		},
		{
			name:   "whitespace-only text is ignored",
			shapes: `<p:sp><p:nvSpPr><p:cNvPr name="Spacer"/><p:nvPr/></p:nvSpPr><p:txBody><a:p><a:r><a:t>  </a:t></a:r></a:p></p:txBody></p:sp>`,
		},
		{
			name:    "malformed XML errors",
			shapes:  `<p:sp><p:nvSpPr>`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := staticTextShapes([]byte(prefix + tc.shapes + suffix))
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("shapes = %#v, want %#v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("shape[%d] = %#v, want %#v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestBusinessTemplateHasNoFixedBranding(t *testing.T) {
	report, err := CheckConformance("../../templates/business-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	if report.FailCount() != 0 || report.WarnCount() != 0 {
		t.Fatalf("conformance: %d FAIL, %d WARN: %#v", report.FailCount(), report.WarnCount(), report.Checks)
	}
	reader, err := OpenTemplate("../../templates/business-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, name := range []string{"ppt/slideLayouts/slideLayout1.xml", "ppt/slideLayouts/slideLayout3.xml"} {
		data, err := reader.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "My Consulting Company") {
			t.Errorf("%s retains sample company name", name)
		}
	}
	if reader.hasFile("ppt/media/image1.emf") {
		t.Error("orphaned sample logo remains in template")
	}
}
