package generator

import (
	"archive/zip"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/utils"
)

// TestSectionTitleFontPreserved verifies that section divider titles use
// ContentSectionTitle (not ContentText), preserving the template's large
// font size instead of capping to 24pt.
func TestSectionTitleFontPreserved(t *testing.T) {
	r, err := zip.OpenReader("../../testdata/templates/template_2.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	idx := utils.BuildZipIndex(&r.Reader)
	layoutData, err := utils.ReadFileFromZipIndex(idx, "ppt/slideLayouts/slideLayout3.xml")
	if err != nil {
		t.Fatal(err)
	}

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
