package generator

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/utils"
)

func TestBuildTableStylesXML_AlwaysIncludesDefault(t *testing.T) {
	xml := buildTableStylesXML(nil)

	if !strings.Contains(xml, types.DefaultTableStyleID) {
		t.Errorf("expected engine default GUID %s in output, got: %s",
			types.DefaultTableStyleID, xml)
	}
	if !strings.Contains(xml, `def="`+types.DefaultTableStyleID+`"`) {
		t.Errorf("expected def attribute to reference engine default, got: %s", xml)
	}
	if !strings.Contains(xml, `<a:tblStyle styleId="`+types.DefaultTableStyleID+`" styleName="Medium Style 2 - Accent 1"/>`) {
		t.Errorf("expected stub tblStyle for engine default, got: %s", xml)
	}
}

func TestBuildTableStylesXML_IncludesCustomGUID(t *testing.T) {
	custom := "{ABCDEF01-1234-5678-9ABC-DEF012345678}"
	xml := buildTableStylesXML(map[string]bool{custom: true})

	if !strings.Contains(xml, custom) {
		t.Errorf("expected custom GUID %s in output, got: %s", custom, xml)
	}
	// Unknown GUIDs get a generic name; the important thing is that styleId is declared.
	if !strings.Contains(xml, `styleId="`+custom+`"`) {
		t.Errorf("expected styleId for custom GUID, got: %s", xml)
	}
}

func TestBuildTableStylesXML_DeduplicatesDefault(t *testing.T) {
	// Passing the default explicitly must not produce duplicate <a:tblStyle> entries.
	xml := buildTableStylesXML(map[string]bool{types.DefaultTableStyleID: true})
	count := strings.Count(xml, `styleId="`+types.DefaultTableStyleID+`"`)
	if count != 1 {
		t.Errorf("expected exactly one entry for engine default, got %d: %s", count, xml)
	}
}

func TestInstallTableStylesOverride_NoOpWhenNoTables(t *testing.T) {
	ctx := &singlePassContext{}
	ctx.tableStyleIDsUsed = map[string]bool{}
	ctx.installTableStylesOverride()
	if _, ok := ctx.syntheticFiles[PathTableStyles]; ok {
		t.Error("expected no synthetic tableStyles.xml when no tables were rendered")
	}
}

func TestInstallTableStylesOverride_WritesWhenTablesPresent(t *testing.T) {
	// No template index — falls back to fully-synthetic approach.
	ctx := &singlePassContext{}
	ctx.tableStyleIDsUsed = map[string]bool{types.DefaultTableStyleID: true}
	ctx.installTableStylesOverride()
	data, ok := ctx.syntheticFiles[PathTableStyles]
	if !ok {
		t.Fatal("expected synthetic tableStyles.xml to be written")
	}
	if !strings.Contains(string(data), types.DefaultTableStyleID) {
		t.Errorf("synthetic tableStyles.xml missing engine default: %s", data)
	}
}

// TestParseTemplateStyleIDs verifies GUID extraction from tableStyles.xml bytes.
func TestParseTemplateStyleIDs(t *testing.T) {
	brand := "{B56CA0EE-1E45-4AA6-A82B-0F5650326712}"

	t.Run("self-closing — no children", func(t *testing.T) {
		data := []byte(`<a:tblStyleLst xmlns:a="..." def="` + types.DefaultTableStyleID + `"/>`)
		got := parseTemplateStyleIDs(data)
		if len(got) != 0 {
			t.Errorf("expected empty map for self-closing element, got %v", got)
		}
	})

	t.Run("single child element", func(t *testing.T) {
		data := []byte(`<a:tblStyleLst xmlns:a="..." def="` + brand + `"><a:tblStyle styleId="` + brand + `" styleName="Brand Table 3"/></a:tblStyleLst>`)
		got := parseTemplateStyleIDs(data)
		if len(got) != 1 {
			t.Errorf("expected 1 ID, got %d: %v", len(got), got)
		}
		if !got[brand] {
			t.Errorf("expected brand GUID %q in result %v", brand, got)
		}
	})

	t.Run("multiple children", func(t *testing.T) {
		guid2 := "{AAAAAAAA-0000-0000-0000-000000000002}"
		data := []byte(`<a:tblStyleLst><a:tblStyle styleId="` + brand + `"/><a:tblStyle styleId="` + guid2 + `"/></a:tblStyleLst>`)
		got := parseTemplateStyleIDs(data)
		if !got[brand] || !got[guid2] {
			t.Errorf("expected both GUIDs in result, got %v", got)
		}
	})
}

// TestMergeTemplateWithStubs verifies that existing template elements are
// preserved and stubs are appended only for missing GUIDs.
func TestMergeTemplateWithStubs(t *testing.T) {
	brand := "{B56CA0EE-1E45-4AA6-A82B-0F5650326712}"
	newGUID := "{DEADBEEF-0000-0000-0000-000000000001}"

	t.Run("open-close form preserves existing element body", func(t *testing.T) {
		template := `<a:tblStyleLst xmlns:a="..." def="` + brand + `"><a:tblStyle styleId="` + brand + `" styleName="Brand Table 3"><a:wholeTbl/></a:tblStyle></a:tblStyleLst>`
		merged := mergeTemplateWithStubs([]byte(template), []string{newGUID})
		if merged == nil {
			t.Fatal("expected non-nil result")
		}
		out := string(merged)
		if !strings.Contains(out, `<a:wholeTbl/>`) {
			t.Error("original element children should be preserved")
		}
		if !strings.Contains(out, `styleId="`+newGUID+`"`) {
			t.Error("stub for missing GUID should be appended")
		}
		if !strings.Contains(out, `</a:tblStyleLst>`) {
			t.Error("output should have proper closing tag")
		}
	})

	t.Run("self-closing form gets stubs injected", func(t *testing.T) {
		template := `<a:tblStyleLst xmlns:a="..." def="` + types.DefaultTableStyleID + `"/>`
		merged := mergeTemplateWithStubs([]byte(template), []string{types.DefaultTableStyleID})
		if merged == nil {
			t.Fatal("expected non-nil result")
		}
		out := string(merged)
		if !strings.Contains(out, `styleId="`+types.DefaultTableStyleID+`"`) {
			t.Errorf("stub for GUID should be present in: %s", out)
		}
		if !strings.Contains(out, `</a:tblStyleLst>`) {
			t.Errorf("output should have proper closing tag, got: %s", out)
		}
	})

	t.Run("nil missing IDs returns template unchanged", func(t *testing.T) {
		template := `<a:tblStyleLst><a:tblStyle styleId="{A}"/></a:tblStyleLst>`
		merged := mergeTemplateWithStubs([]byte(template), nil)
		if string(merged) != template {
			t.Errorf("expected unchanged template, got %q", merged)
		}
	})
}

// TestInstallTableStylesOverride_NoOverrideWhenTemplateCoversAllGUIDs verifies
// that no synthetic file is written when the template's tableStyles.xml already
// declares all referenced GUIDs.
func TestInstallTableStylesOverride_NoOverrideWhenTemplateCoversAllGUIDs(t *testing.T) {
	brand := "{B56CA0EE-1E45-4AA6-A82B-0F5650326712}"
	templateXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="` + brand + `">` +
		`<a:tblStyle styleId="` + brand + `" styleName="Brand Table 3"><a:wholeTbl/></a:tblStyle>` +
		`</a:tblStyleLst>`

	idx := buildZipIndex(t, map[string][]byte{PathTableStyles: []byte(templateXML)})

	ctx := &singlePassContext{}
	ctx.templateIndex = idx
	ctx.tableStyleIDsUsed = map[string]bool{brand: true}
	ctx.installTableStylesOverride()

	if _, ok := ctx.syntheticFiles[PathTableStyles]; ok {
		t.Error("no synthetic override expected when template already covers all referenced GUIDs")
	}
}

// TestInstallTableStylesOverride_MergesWhenGUIDMissing verifies that when the
// template declares some styles but not all referenced ones, the merged result
// preserves the template's elements and adds stubs for the missing GUIDs.
func TestInstallTableStylesOverride_MergesWhenGUIDMissing(t *testing.T) {
	brand := "{B56CA0EE-1E45-4AA6-A82B-0F5650326712}"
	newGUID := "{DEADBEEF-0000-0000-0000-000000000001}"
	templateXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="` + brand + `">` +
		`<a:tblStyle styleId="` + brand + `" styleName="Brand Table 3"><a:wholeTbl/></a:tblStyle>` +
		`</a:tblStyleLst>`

	idx := buildZipIndex(t, map[string][]byte{PathTableStyles: []byte(templateXML)})

	ctx := &singlePassContext{}
	ctx.templateIndex = idx
	ctx.tableStyleIDsUsed = map[string]bool{brand: true, newGUID: true}
	ctx.installTableStylesOverride()

	data, ok := ctx.syntheticFiles[PathTableStyles]
	if !ok {
		t.Fatal("expected synthetic override when a referenced GUID is missing from template")
	}
	out := string(data)
	if !strings.Contains(out, `<a:wholeTbl/>`) {
		t.Error("original template element body should be preserved in merged output")
	}
	if !strings.Contains(out, `styleId="`+newGUID+`"`) {
		t.Error("missing GUID should have a stub in the merged output")
	}
}

// TestInstallTableStylesOverride_SelfClosingTemplate verifies that a template
// with a self-closing <a:tblStyleLst/> gets converted to the open form with
// stubs injected for all referenced GUIDs.
func TestInstallTableStylesOverride_SelfClosingTemplate(t *testing.T) {
	templateXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<a:tblStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" def="` + types.DefaultTableStyleID + `"/>`

	idx := buildZipIndex(t, map[string][]byte{PathTableStyles: []byte(templateXML)})

	ctx := &singlePassContext{}
	ctx.templateIndex = idx
	ctx.tableStyleIDsUsed = map[string]bool{types.DefaultTableStyleID: true}
	ctx.installTableStylesOverride()

	data, ok := ctx.syntheticFiles[PathTableStyles]
	if !ok {
		t.Fatal("expected synthetic override for self-closing template with missing GUIDs")
	}
	out := string(data)
	if !strings.Contains(out, `styleId="`+types.DefaultTableStyleID+`"`) {
		t.Errorf("expected default GUID stub in output: %s", out)
	}
	if !strings.Contains(out, `</a:tblStyleLst>`) {
		t.Errorf("expected proper closing tag in merged output: %s", out)
	}
}

// buildZipIndex builds a utils.ZipIndex backed by an in-memory ZIP archive
// containing the provided name→content entries.
func buildZipIndex(t *testing.T, files map[string][]byte) utils.ZipIndex {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create ZIP entry %q: %v", name, err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("failed to write ZIP entry %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close ZIP writer: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("failed to open in-memory ZIP: %v", err)
	}
	return utils.BuildZipIndex(r)
}
