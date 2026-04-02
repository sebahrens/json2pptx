package generator

import (
	"strings"
	"testing"
)

func TestGenerateSourceNoteShape(t *testing.T) {
	shape := generateSourceNoteShape("Company Annual Report, FY2025")

	// Check it contains the source text
	if !strings.Contains(shape, "Source: Company Annual Report, FY2025") {
		t.Error("shape XML should contain 'Source: ' prefix and the source text")
	}

	// Check it has proper OOXML structure
	if !strings.Contains(shape, "<p:sp>") {
		t.Error("shape XML should start with <p:sp>")
	}
	if !strings.Contains(shape, `sz="800"`) {
		t.Error("font size should be 800 (8pt)")
	}
	if !strings.Contains(shape, `i="1"`) {
		t.Error("text should be italic")
	}
	if !strings.Contains(shape, `val="888888"`) {
		t.Error("text color should be gray (888888)")
	}
}

func TestGenerateSourceNoteShapeXMLEscaping(t *testing.T) {
	shape := generateSourceNoteShape("Smith & Jones <2025>")

	if !strings.Contains(shape, "Smith &amp; Jones &lt;2025&gt;") {
		t.Error("special XML characters should be escaped")
	}
}

func TestInsertSourceNote(t *testing.T) {
	slideXML := []byte(`<p:sld>
  <p:cSld>
    <p:spTree>
      <p:sp>existing shape</p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	result, err := insertSourceNote(slideXML, "Test Source")
	if err != nil {
		t.Fatal(err)
	}

	resultStr := string(result)

	// Source note shape should be inserted before </p:spTree>
	if !strings.Contains(resultStr, "Source: Test Source") {
		t.Error("result should contain the source note text")
	}

	// Original content should still be present
	if !strings.Contains(resultStr, "existing shape") {
		t.Error("original slide content should be preserved")
	}

	// Source note should appear before closing tag
	sourceIdx := strings.Index(resultStr, "Source: Test Source")
	closeIdx := strings.Index(resultStr, "</p:spTree>")
	if sourceIdx > closeIdx {
		t.Error("source note should appear before </p:spTree>")
	}
}

func TestInsertSourceNoteMissingSpTree(t *testing.T) {
	slideXML := []byte(`<p:sld><p:cSld></p:cSld></p:sld>`)

	_, err := insertSourceNote(slideXML, "Test Source")
	if err == nil {
		t.Error("expected error when </p:spTree> is missing")
	}
}
