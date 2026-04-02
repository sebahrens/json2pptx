package pptx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

func TestGenerateGroup_Basic(t *testing.T) {
	t.Parallel()

	// Create 3 child shapes
	child1, err := GenerateShape(ShapeOptions{
		ID: 2, Bounds: RectFromInches(1, 1, 1, 1), Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("child1: %v", err)
	}
	child2, err := GenerateShape(ShapeOptions{
		ID: 3, Bounds: RectFromInches(2.5, 1, 1, 1), Geometry: GeomEllipse,
	})
	if err != nil {
		t.Fatalf("child2: %v", err)
	}
	child3, err := GenerateShape(ShapeOptions{
		ID: 4, Bounds: RectFromInches(4, 1, 1, 1), Geometry: GeomTriangle,
	})
	if err != nil {
		t.Fatalf("child3: %v", err)
	}

	grp, err := GenerateGroup(GroupOptions{
		ID:       1,
		Bounds:   RectFromInches(1, 1, 4, 1),
		Children: [][]byte{child1, child2, child3},
	})
	if err != nil {
		t.Fatalf("GenerateGroup failed: %v", err)
	}

	s := string(grp)

	// Must have essential group structure
	for _, tag := range []string{
		"<p:grpSp>", "</p:grpSp>",
		"<p:nvGrpSpPr>", "</p:nvGrpSpPr>",
		"<p:grpSpPr>", "</p:grpSpPr>",
		"<p:cNvGrpSpPr/>",
		"<a:chOff", "<a:chExt",
	} {
		if !strings.Contains(s, tag) {
			t.Errorf("missing %s", tag)
		}
	}

	// Should contain all 3 children
	if strings.Count(s, "<p:sp>") != 3 {
		t.Errorf("expected 3 child shapes, got %d", strings.Count(s, "<p:sp>"))
	}

	// Default name
	if !strings.Contains(s, `name="Group 1"`) {
		t.Error("missing default name")
	}

	// Identity transform: chOff==off, chExt==ext
	bounds := RectFromInches(1, 1, 4, 1)
	if !strings.Contains(s, formatOff(bounds.X, bounds.Y)) {
		t.Error("missing a:off")
	}
	if !strings.Contains(s, formatChOff(bounds.X, bounds.Y)) {
		t.Error("identity transform: chOff should equal off")
	}

	// Well-formed XML
	var v interface{}
	if err := xml.Unmarshal(grp, &v); err != nil {
		t.Errorf("invalid XML: %v\n%s", err, s)
	}
}

func TestGenerateGroup_WithChildSpace(t *testing.T) {
	t.Parallel()

	child, err := GenerateShape(ShapeOptions{
		ID: 2, Bounds: RectEmu{X: 0, Y: 0, CX: 1000, CY: 1000}, Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("child: %v", err)
	}

	slideBounds := RectFromInches(2, 2, 4, 3)
	childSpace := RectEmu{X: 0, Y: 0, CX: 1000, CY: 1000}

	grp, err := GenerateGroupWithChildSpace(GroupOptions{
		ID:       1,
		Bounds:   slideBounds,
		Children: [][]byte{child},
	}, childSpace)
	if err != nil {
		t.Fatalf("GenerateGroupWithChildSpace failed: %v", err)
	}

	s := string(grp)

	// Slide-space offset/extent
	if !strings.Contains(s, formatOff(slideBounds.X, slideBounds.Y)) {
		t.Error("missing slide-space a:off")
	}
	if !strings.Contains(s, formatExt(slideBounds.CX, slideBounds.CY)) {
		t.Error("missing slide-space a:ext")
	}

	// Child coordinate space (different from slide space)
	if !strings.Contains(s, formatChOff(childSpace.X, childSpace.Y)) {
		t.Error("missing child a:chOff")
	}
	if !strings.Contains(s, formatChExt(childSpace.CX, childSpace.CY)) {
		t.Error("missing child a:chExt")
	}

	// Well-formed XML
	var v interface{}
	if err := xml.Unmarshal(grp, &v); err != nil {
		t.Errorf("invalid XML: %v\n%s", err, s)
	}
}

func TestGenerateGroup_Nested(t *testing.T) {
	t.Parallel()

	// Create inner group with a shape
	innerChild, err := GenerateShape(ShapeOptions{
		ID: 3, Bounds: RectFromInches(1, 1, 1, 1), Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("innerChild: %v", err)
	}

	innerGroup, err := GenerateGroup(GroupOptions{
		ID:       2,
		Name:     "Inner Group",
		Bounds:   RectFromInches(1, 1, 2, 2),
		Children: [][]byte{innerChild},
	})
	if err != nil {
		t.Fatalf("inner group: %v", err)
	}

	// Create outer group containing the inner group
	outerGroup, err := GenerateGroup(GroupOptions{
		ID:       1,
		Name:     "Outer Group",
		Bounds:   RectFromInches(0, 0, 5, 5),
		Children: [][]byte{innerGroup},
	})
	if err != nil {
		t.Fatalf("outer group: %v", err)
	}

	s := string(outerGroup)

	// Nested group: should have 2 p:grpSp elements
	if strings.Count(s, "<p:grpSp>") != 2 {
		t.Errorf("expected 2 nested p:grpSp, got %d", strings.Count(s, "<p:grpSp>"))
	}

	if !strings.Contains(s, `name="Outer Group"`) {
		t.Error("missing outer group name")
	}
	if !strings.Contains(s, `name="Inner Group"`) {
		t.Error("missing inner group name")
	}

	// Well-formed XML
	var v interface{}
	if err := xml.Unmarshal(outerGroup, &v); err != nil {
		t.Errorf("invalid XML: %v\n%s", err, s)
	}
}

func TestGenerateGroup_EmptyChildren(t *testing.T) {
	t.Parallel()

	grp, err := GenerateGroup(GroupOptions{
		ID:     1,
		Bounds: RectFromInches(0, 0, 2, 2),
	})
	if err != nil {
		t.Fatalf("GenerateGroup failed: %v", err)
	}

	// Should be valid even with no children
	var v interface{}
	if err := xml.Unmarshal(grp, &v); err != nil {
		t.Errorf("invalid XML: %v\n%s", err, string(grp))
	}

	// No child shapes
	if strings.Contains(string(grp), "<p:sp>") {
		t.Error("should not contain child shapes")
	}
}

func TestGenerateGroup_ValidationErrors(t *testing.T) {
	t.Parallel()

	_, err := GenerateGroup(GroupOptions{
		Bounds: RectFromInches(0, 0, 1, 1),
	})
	if err == nil {
		t.Error("expected error for missing ID")
	}
}

func TestGenerateGroup_CustomNameAndDescription(t *testing.T) {
	t.Parallel()

	grp, err := GenerateGroup(GroupOptions{
		ID:          1,
		Name:        "My Group",
		Description: "A group of shapes",
		Bounds:      RectFromInches(0, 0, 2, 2),
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(grp)
	if !strings.Contains(s, `name="My Group"`) {
		t.Error("missing custom name")
	}
	if !strings.Contains(s, `descr="A group of shapes"`) {
		t.Error("missing description")
	}
}

func TestGenerateGroup_SpecialCharsInName(t *testing.T) {
	t.Parallel()

	grp, err := GenerateGroup(GroupOptions{
		ID:          1,
		Name:        `Group "A" & <B>`,
		Description: `Alt: "test" & <value>`,
		Bounds:      RectFromInches(0, 0, 1, 1),
	})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}

	s := string(grp)
	if !strings.Contains(s, `&quot;`) {
		t.Error("quotes should be escaped")
	}
	if !strings.Contains(s, `&amp;`) {
		t.Error("ampersands should be escaped")
	}

	var v interface{}
	if err := xml.Unmarshal(grp, &v); err != nil {
		t.Errorf("invalid XML: %v", err)
	}
}

func TestInsertGroup_Success(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	child, err := GenerateShape(ShapeOptions{
		ID: 10, Bounds: RectFromInches(1, 1, 1, 1), Geometry: GeomRect,
	})
	if err != nil {
		t.Fatalf("child: %v", err)
	}

	err = doc.InsertGroup(InsertGroupOptions{
		SlideIndex: 0,
		Group: GroupOptions{
			Name:     "Test Group",
			Bounds:   RectFromInches(1, 1, 3, 2),
			Children: [][]byte{child},
		},
	})
	if err != nil {
		t.Fatalf("InsertGroup failed: %v", err)
	}

	// Verify the document can be saved and reloaded
	saved, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	doc2, err := OpenDocumentFromBytes(saved)
	if err != nil {
		t.Fatalf("Failed to open saved document: %v", err)
	}

	slideXML, err := doc2.Package().ReadEntry("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatalf("Failed to read slide: %v", err)
	}

	if !bytes.Contains(slideXML, []byte("<p:grpSp>")) {
		t.Error("p:grpSp element not found in slide")
	}
	if !bytes.Contains(slideXML, []byte(`name="Test Group"`)) {
		t.Error("Group name not found in slide")
	}
}

func TestInsertGroup_ZeroBounds(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	err = doc.InsertGroup(InsertGroupOptions{
		SlideIndex: 0,
		Group:      GroupOptions{Bounds: RectEmu{}},
	})
	if err == nil {
		t.Error("Expected error for zero bounds, got nil")
	}
}

func TestInsertGroup_InvalidSlideIndex(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithSlide()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("OpenDocumentFromBytes failed: %v", err)
	}

	err = doc.InsertGroup(InsertGroupOptions{
		SlideIndex: 99,
		Group:      GroupOptions{Bounds: RectFromInches(0, 0, 1, 1)},
	})
	if err == nil {
		t.Error("Expected error for invalid slide index, got nil")
	}
}

// Helper functions for formatting expected XML snippets.

func formatOff(x, y int64) string {
	return fmt.Sprintf(`<a:off x="%d" y="%d"/>`, x, y)
}

func formatExt(cx, cy int64) string {
	return fmt.Sprintf(`<a:ext cx="%d" cy="%d"/>`, cx, cy)
}

func formatChOff(x, y int64) string {
	return fmt.Sprintf(`<a:chOff x="%d" y="%d"/>`, x, y)
}

func formatChExt(cx, cy int64) string {
	return fmt.Sprintf(`<a:chExt cx="%d" cy="%d"/>`, cx, cy)
}
