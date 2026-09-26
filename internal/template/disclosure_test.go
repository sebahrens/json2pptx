package template

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDisclosureRemainsDistinctFromSubtitle(t *testing.T) {
	raw := []byte(`<root><sp><nvSpPr><cNvPr name="Legal disclosure"/><nvPr><ph type="subTitle" idx="1"/></nvPr></nvSpPr><txBody><lstStyle><lvl1pPr><defRPr sz="1100"/></lvl1pPr></lstStyle></txBody></sp><sp><nvSpPr><cNvPr name="Main Subtitle"/><nvPr><ph type="subTitle" idx="2"/></nvPr></nvSpPr></sp></root>`)
	var doc struct {
		Shapes []shapeXML `xml:"sp"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	result := NormalizePlaceholderNames(doc.Shapes)
	if len(result.Warnings) != 0 {
		t.Fatalf("ambiguous names: %v", result.Warnings)
	}
	if got := doc.Shapes[0].NonVisualProperties.ConnectionNonVisual.Name; got != "legal_disclosure" {
		t.Fatalf("disclosure routed as %q", got)
	}
	if got := doc.Shapes[1].NonVisualProperties.ConnectionNonVisual.Name; got != "subtitle" {
		t.Fatalf("main subtitle routed as %q", got)
	}
	modified := ApplyNormalizationToBytes(raw, result)
	if !bytes.Contains(modified, []byte(`name="legal_disclosure"`)) || !bytes.Contains(modified, []byte(`sz="1100"`)) || !bytes.Contains(modified, []byte(`type="subTitle" idx="1"`)) {
		t.Fatalf("disclosure style/type not preserved: %s", modified)
	}
	role, confidence := ClassifyPlaceholderRole(types.PlaceholderInfo{ID: "legal_disclosure", Type: types.PlaceholderSubtitle, FontSize: 1100}, nil)
	if string(role) != "disclosure" || confidence != 1 {
		t.Fatalf("wrong disclosure role: %s %v", role, confidence)
	}
	// Size alone is not semantics: an ordinary small subtitle stays a subtitle.
	role, _ = ClassifyPlaceholderRole(types.PlaceholderInfo{ID: "subtitle", Type: types.PlaceholderSubtitle, FontSize: 1100}, nil)
	if role != types.PlaceholderRoleSubtitle {
		t.Fatalf("small subtitle misclassified: %s", role)
	}
}

func TestDisclosureClassificationRequiresExplicitTextRole(t *testing.T) {
	for _, kind := range []types.PlaceholderType{types.PlaceholderSubtitle, types.PlaceholderBody, types.PlaceholderContent, types.PlaceholderTitle, types.PlaceholderImage, types.PlaceholderChart, types.PlaceholderTable, types.PlaceholderOther} {
		for _, id := range []string{"legal_disclosure", " Legal Disclosure ", "legal notes", "subtitle"} {
			role, _ := ClassifyPlaceholderRole(types.PlaceholderInfo{ID: id, Type: kind, FontSize: 1100}, nil)
			wantDisclosure := (id == "legal_disclosure" || id == " Legal Disclosure ") && (kind == types.PlaceholderSubtitle || kind == types.PlaceholderBody || kind == types.PlaceholderContent)
			if (role == types.PlaceholderRoleDisclosure) != wantDisclosure {
				t.Errorf("id=%q type=%s role=%s; explicit disclosure=%v", id, kind, role, wantDisclosure)
			}
		}
	}
}
