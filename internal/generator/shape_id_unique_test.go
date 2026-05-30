package generator

import (
	"testing"
)

// TestLateInjectionShapeIDsUnique is a regression test for duplicate cNvPr ids.
// Late-injected takeaway, source note, and footer shapes previously used
// hard-coded ids (998/999/990/992). When a slide already contained one of
// those ids, the generated slide had duplicate p:cNvPr/@id values, which
// PowerPoint/LibreOffice may "repair" by dropping shapes. The injections now
// allocate ids from findMaxShapeID(slideData)+1, so every shape on the slide
// must have a unique id even when the original content collides with the old
// hard-coded values.
func TestLateInjectionShapeIDsUnique(t *testing.T) {
	// Slide whose existing shapes deliberately occupy the old hard-coded ids.
	slide := []byte(`<?xml version="1.0"?><p:sld xmlns:p="x"><p:cSld><p:spTree>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="990" name="A"/></p:nvSpPr></p:sp>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="992" name="B"/></p:nvSpPr></p:sp>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="998" name="C"/></p:nvSpPr></p:sp>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="999" name="D"/></p:nvSpPr></p:sp>` +
		`</p:spTree></p:cSld></p:sld>`)

	var err error
	slide, err = insertTakeaway(slide, "Headline answer")
	if err != nil {
		t.Fatalf("insertTakeaway: %v", err)
	}
	slide, err = insertSourceNote(slide, "Annual Report 2025")
	if err != nil {
		t.Fatalf("insertSourceNote: %v", err)
	}

	positions := map[string]*transformXML{
		"type:dt": {
			Offset: offsetXML{X: 457200, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
		"type:sldNum": {
			Offset: offsetXML{X: 8610600, Y: 6492875},
			Extent: extentXML{CX: 3200400, CY: 365125},
		},
	}
	slide, err = insertFooters(slide, &FooterConfig{Enabled: true, LeftText: "Confidential"}, positions)
	if err != nil {
		t.Fatalf("insertFooters: %v", err)
	}

	// Collect every cNvPr id on the slide and assert uniqueness.
	matches := shapeIDRegex.FindAllSubmatch(slide, -1)
	if len(matches) == 0 {
		t.Fatal("no shape ids found in generated slide")
	}
	seen := make(map[string]bool, len(matches))
	for _, m := range matches {
		id := string(m[1])
		if seen[id] {
			t.Errorf("duplicate shape id %q in generated slide:\n%s", id, slide)
		}
		seen[id] = true
	}

	// 4 original + takeaway + source note + 2 footer shapes = 8 unique ids.
	if len(seen) != 8 {
		t.Errorf("expected 8 unique shape ids, got %d: %v", len(seen), seen)
	}
}
