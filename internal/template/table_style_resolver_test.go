package template

import (
	"archive/zip"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestBorrowOpenZIPCloseLeavesOwnerOpen(t *testing.T) {
	z, err := zip.OpenReader("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	reader := BorrowOpenZIP("../../templates/modern-template.pptx", z)
	if got := reader.ResolveTableStyleID(TemplateDefaultSentinel); got == "" {
		t.Fatal("borrowed reader did not resolve template style")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := z.File[0].Open()
	if err != nil {
		t.Errorf("borrowed reader closed the caller's ZIP: %v", err)
	} else {
		_ = f.Close()
	}
}

func TestResolveTableStyleID_Empty(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	got := reader.ResolveTableStyleID("")
	if got != types.DefaultTableStyleID {
		t.Errorf("empty → %q, want %q", got, types.DefaultTableStyleID)
	}
}

func TestResolveTableStyleID_ExplicitGUID(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	guid := "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	got := reader.ResolveTableStyleID(guid)
	if got != guid {
		t.Errorf("explicit GUID → %q, want %q", got, guid)
	}
}

func TestResolveTableStyleID_UnknownGUID(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	unknown := "{00000000-0000-0000-0000-000000000000}"
	got := reader.ResolveTableStyleID(unknown)
	if got != unknown {
		t.Errorf("unknown GUID → %q, want passthrough %q", got, unknown)
	}
}

func TestResolveTableStyleID_TemplateDefault(t *testing.T) {
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	got := reader.ResolveTableStyleID(TemplateDefaultSentinel)

	// Must resolve to some non-empty GUID
	if got == "" {
		t.Fatal("@template-default resolved to empty string")
	}
	if got == TemplateDefaultSentinel {
		t.Fatal("@template-default was not resolved")
	}
}

func TestResolveTableStyleID_Stable(t *testing.T) {
	// Resolution must be deterministic across 100 calls.
	reader, err := OpenTemplate("../../templates/modern-template.pptx")
	if err != nil {
		t.Fatalf("OpenTemplate: %v", err)
	}
	defer reader.Close()

	first := reader.ResolveTableStyleID(TemplateDefaultSentinel)
	for i := 0; i < 100; i++ {
		got := reader.ResolveTableStyleID(TemplateDefaultSentinel)
		if got != first {
			t.Fatalf("iteration %d: got %q, want %q", i, got, first)
		}
	}
}

func TestHasTableStyleDefinition(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{name: "forest-green", want: false},
		{name: "midnight-blue", want: false},
		{name: "modern-yellow", want: false},
		{name: "warm-coral", want: false},
		{name: "modern-template", want: true},
		{name: "blue-corporate", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader, err := OpenTemplate("../../templates/" + tc.name + ".pptx")
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			guid := reader.ResolveTableStyleID(TemplateDefaultSentinel)
			if got := reader.HasTableStyleDefinition(guid); got != tc.want {
				t.Errorf("default style %s definition = %t, want %t", guid, got, tc.want)
			}
		})
	}
}

func TestHasTableStyleDefinitionIgnoresEmptyStub(t *testing.T) {
	const guid = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "empty stub", body: "", want: false},
		{name: "formatting rule", body: `<a:firstRow><a:tcTxStyle b="1"/></a:firstRow>`, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			styles := `<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="` + guid + `"><a:tblStyle styleId="` + guid + `" styleName="Test">` + tc.body + `</a:tblStyle></a:tblStyleLst>`
			path := createTestPPTXWithContent(t, map[string][]byte{
				"ppt/presentation.xml":              []byte(`<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`),
				"ppt/slideLayouts/slideLayout1.xml": []byte(`<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`),
				"ppt/tableStyles.xml":               []byte(styles),
			})
			reader, err := OpenTemplate(path)
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			if got := reader.HasTableStyleDefinition(guid); got != tc.want {
				t.Errorf("HasTableStyleDefinition = %t, want %t", got, tc.want)
			}
		})
	}
}
