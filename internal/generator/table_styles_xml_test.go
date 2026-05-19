package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
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
