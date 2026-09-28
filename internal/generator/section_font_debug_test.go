package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
)

// TestSectionTitleFontPreserved verifies that section divider titles use
// ContentSectionTitle (not ContentText), preserving the template's large
// font size instead of capping to 24pt.
func TestSectionTitleFontPreserved(t *testing.T) {
	// A body-only section divider whose body placeholder carries 96pt, the
	// shape of the untracked template_2 fixture this test used to read (it
	// always skipped; go-slide-creator-gdi5r). No shipped template has a
	// body-only section divider, so the layout is inline.
	layoutData := []byte(sectionDividerBodyOnlyLayout)

	// Normalize the layout bytes so shapes have canonical names
	normalizedData, _ := template.NormalizeLayoutBytes(layoutData)

	slide, err := createSlideFromLayout(normalizedData, 1, nil)
	if err != nil {
		t.Fatal(err)
	}

	pm := buildPlaceholderMap(slide.CommonSlideData.ShapeTree.Shapes)
	shapeIdx, ok := pm["body"]
	if !ok {
		t.Fatal("Could not find body placeholder")
	}

	shape := &slide.CommonSlideData.ShapeTree.Shapes[shapeIdx]

	// Template layout has sz="9600" (96pt) for the section divider body placeholder
	beforeFont := extractFontSizeFromShape(shape)
	if beforeFont != 9600 {
		t.Fatalf("expected template font 9600, got %d", beforeFont)
	}

	err = populateShapeText(shape, ContentItem{
		PlaceholderID: "body",
		Type:          ContentSectionTitle,
		Value:         "Table Coverage",
	}, -1, "Franklin Gothic Book")
	if err != nil {
		t.Fatal(err)
	}

	afterFont := extractFontSizeFromShape(shape)

	// Font should be word-fit capped (>40pt), not body-text capped (24pt).
	// The height-based boost (minSectionTitleFontForHeight) also ensures
	// the font is at least ~32-54pt for visual prominence.
	if afterFont <= 2400 {
		t.Errorf("section title font capped to body-text size %d hpt; want >2400", afterFont)
	}

	// Verify the text is present
	if shape.TextBody == nil || len(shape.TextBody.Paragraphs) == 0 {
		t.Fatal("no paragraphs after population")
	}
	found := false
	for _, p := range shape.TextBody.Paragraphs {
		for _, r := range p.Runs {
			if strings.Contains(r.Text, "Table Coverage") {
				found = true
			}
		}
	}
	if !found {
		t.Error("section title text not found in shape")
	}
}

// sectionDividerBodyOnlyLayout is a minimal section-divider layout with no
// title placeholder and a single body placeholder styled at 96pt.
const sectionDividerBodyOnlyLayout = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sldLayout xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" type="secHead" preserve="1">
  <p:cSld name="Section Divider">
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
      <p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>
      <p:sp>
        <p:nvSpPr><p:cNvPr id="2" name="Text Placeholder 1"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
        <p:spPr><a:xfrm><a:off x="838200" y="1709738"/><a:ext cx="10515600" cy="2852737"/></a:xfrm></p:spPr>
        <p:txBody><a:bodyPr anchor="b"><a:normAutofit/></a:bodyPr><a:lstStyle><a:lvl1pPr marL="0" indent="0"><a:buNone/><a:defRPr sz="9600"/></a:lvl1pPr></a:lstStyle><a:p><a:r><a:rPr lang="en-US" dirty="0"/><a:t>Section title</a:t></a:r></a:p></p:txBody>
      </p:sp>
    </p:spTree>
  </p:cSld>
  <p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>
</p:sldLayout>`
