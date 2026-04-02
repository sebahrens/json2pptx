package pptx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInsertIntoSpTree_AtEnd(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1"/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp><p:nvSpPr><p:cNvPr id="2"/></p:nvSpPr></p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic><p:nvPicPr><p:cNvPr id="3" name="Picture"/></p:nvPicPr></p:pic>`)

	result, err := InsertIntoSpTree(slideXML, picXML, InsertAtEnd)
	if err != nil {
		t.Fatalf("InsertIntoSpTree failed: %v", err)
	}

	resultStr := string(result)

	// Check that the picture is in the result
	if !strings.Contains(resultStr, `<p:pic>`) {
		t.Error("result should contain <p:pic>")
	}

	// Check order: p:sp should come before p:pic
	spIndex := strings.Index(resultStr, "<p:sp>")
	picIndex := strings.Index(resultStr, "<p:pic>")
	if spIndex >= picIndex {
		t.Error("p:pic should come after p:sp when inserting at end")
	}
}

func TestInsertIntoSpTree_AtStart(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1"/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp><p:nvSpPr><p:cNvPr id="2"/></p:nvSpPr></p:sp>
      <p:sp><p:nvSpPr><p:cNvPr id="3"/></p:nvSpPr></p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic><p:nvPicPr><p:cNvPr id="4" name="Picture"/></p:nvPicPr></p:pic>`)

	result, err := InsertIntoSpTree(slideXML, picXML, InsertAtStart)
	if err != nil {
		t.Fatalf("InsertIntoSpTree failed: %v", err)
	}

	resultStr := string(result)

	// Check that p:pic comes before the first p:sp (after group properties)
	grpSpPrIndex := strings.Index(resultStr, "</p:grpSpPr>")
	picIndex := strings.Index(resultStr, "<p:pic>")
	firstSpIndex := strings.Index(resultStr, "<p:sp>")

	if picIndex < grpSpPrIndex {
		t.Error("p:pic should come after </p:grpSpPr>")
	}

	if picIndex > firstSpIndex {
		t.Error("p:pic should come before the first <p:sp> when inserting at start")
	}
}

func TestInsertIntoSpTree_AtPosition(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld>
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1"/></p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp><p:cNvPr id="2" name="First"/></p:sp>
      <p:sp><p:cNvPr id="3" name="Second"/></p:sp>
      <p:sp><p:cNvPr id="4" name="Third"/></p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic><p:cNvPr id="5" name="Picture"/></p:pic>`)

	// Insert at position 2 (after 2 shapes, before "Third")
	result, err := InsertIntoSpTree(slideXML, picXML, InsertPosition(2))
	if err != nil {
		t.Fatalf("InsertIntoSpTree failed: %v", err)
	}

	resultStr := string(result)

	// Verify order: First, Second, Picture, Third
	firstIdx := strings.Index(resultStr, `name="First"`)
	secondIdx := strings.Index(resultStr, `name="Second"`)
	picIdx := strings.Index(resultStr, `name="Picture"`)
	thirdIdx := strings.Index(resultStr, `name="Third"`)

	if !(firstIdx < secondIdx && secondIdx < picIdx && picIdx < thirdIdx) {
		t.Errorf("wrong order: First@%d, Second@%d, Picture@%d, Third@%d",
			firstIdx, secondIdx, picIdx, thirdIdx)
	}
}

func TestInsertIntoSpTree_EmptySpTree(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld>
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr><p:cNvPr id="1"/></p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic><p:cNvPr id="2" name="Picture"/></p:pic>`)

	result, err := InsertIntoSpTree(slideXML, picXML, InsertAtEnd)
	if err != nil {
		t.Fatalf("InsertIntoSpTree failed: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "<p:pic>") {
		t.Error("result should contain <p:pic>")
	}
}

func TestInsertIntoSpTree_MultiLineElement(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld>
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr/>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic>
  <p:nvPicPr>
    <p:cNvPr id="2" name="Picture"/>
  </p:nvPicPr>
  <p:blipFill/>
</p:pic>`)

	result, err := InsertIntoSpTree(slideXML, picXML, InsertAtEnd)
	if err != nil {
		t.Fatalf("InsertIntoSpTree failed: %v", err)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "<p:pic>") {
		t.Error("result should contain <p:pic>")
	}
	if !strings.Contains(resultStr, "<p:blipFill/>") {
		t.Error("result should contain <p:blipFill/>")
	}
}

func TestInsertIntoSpTree_MissingSpTree(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld>
  <p:cSld>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic/>`)

	_, err := InsertIntoSpTree(slideXML, picXML, InsertAtEnd)
	if err == nil {
		t.Error("expected error for missing spTree")
	}
}

func TestCountShapesInSpTree(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		expected int
	}{
		{
			name: "no shapes",
			xml: `<p:sld><p:spTree>
				<p:nvGrpSpPr/>
				<p:grpSpPr/>
			</p:spTree></p:sld>`,
			expected: 0,
		},
		{
			name: "one shape",
			xml: `<p:sld><p:spTree>
				<p:nvGrpSpPr/>
				<p:grpSpPr/>
				<p:sp><p:cNvPr/></p:sp>
			</p:spTree></p:sld>`,
			expected: 1,
		},
		{
			name: "multiple shapes",
			xml: `<p:sld><p:spTree>
				<p:nvGrpSpPr/>
				<p:grpSpPr/>
				<p:sp><p:cNvPr id="2"/></p:sp>
				<p:pic><p:cNvPr id="3"/></p:pic>
				<p:graphicFrame><p:cNvPr id="4"/></p:graphicFrame>
				<p:cxnSp><p:cNvPr id="5"/></p:cxnSp>
			</p:spTree></p:sld>`,
			expected: 4,
		},
		{
			name: "nested group shapes",
			xml: `<p:sld><p:spTree>
				<p:nvGrpSpPr/>
				<p:grpSpPr/>
				<p:grpSp>
					<p:sp><p:cNvPr id="2"/></p:sp>
				</p:grpSp>
			</p:spTree></p:sld>`,
			expected: 2, // grpSp and the nested sp
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := CountShapesInSpTree([]byte(tt.xml))
			if err != nil {
				t.Fatalf("CountShapesInSpTree failed: %v", err)
			}
			if count != tt.expected {
				t.Errorf("count = %d, want %d", count, tt.expected)
			}
		})
	}
}

func TestExtractSpTree(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0"?>
<p:sld>
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr/>
      <p:grpSpPr/>
      <p:sp><p:cNvPr id="2"/></p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	spTree, err := ExtractSpTree(slideXML)
	if err != nil {
		t.Fatalf("ExtractSpTree failed: %v", err)
	}

	spTreeStr := string(spTree)
	if !strings.HasPrefix(spTreeStr, "<p:spTree>") {
		t.Error("extracted spTree should start with <p:spTree>")
	}
	if !strings.HasSuffix(spTreeStr, "</p:spTree>") {
		t.Error("extracted spTree should end with </p:spTree>")
	}
	if !strings.Contains(spTreeStr, "<p:sp>") {
		t.Error("extracted spTree should contain the shape element")
	}
}

func TestValidateSpTree(t *testing.T) {
	tests := []struct {
		name    string
		xml     string
		wantErr bool
	}{
		{
			name: "valid spTree",
			xml: `<p:sld><p:spTree>
				<p:nvGrpSpPr/>
				<p:grpSpPr/>
			</p:spTree></p:sld>`,
			wantErr: false,
		},
		{
			name:    "missing spTree",
			xml:     `<p:sld><p:cSld/></p:sld>`,
			wantErr: true,
		},
		{
			name: "missing nvGrpSpPr",
			xml: `<p:sld><p:spTree>
				<p:grpSpPr/>
			</p:spTree></p:sld>`,
			wantErr: true,
		},
		{
			name: "missing grpSpPr",
			xml: `<p:sld><p:spTree>
				<p:nvGrpSpPr/>
			</p:spTree></p:sld>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpTree([]byte(tt.xml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpTree() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestInsertIntoSpTree_RealTemplates tests against actual PPTX templates.
func TestInsertIntoSpTree_RealTemplates(t *testing.T) {
	templateDir := filepath.Join("..", "..", "templates")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Skip("templates directory not found, skipping real template test")
	}

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		t.Fatalf("failed to read templates dir: %v", err)
	}

	picXML := []byte(`<p:pic xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:nvPicPr>
    <p:cNvPr id="999" name="Test Picture"/>
    <p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr>
    <p:nvPr/>
  </p:nvPicPr>
  <p:blipFill>
    <a:blip r:embed="rId999"/>
    <a:stretch><a:fillRect/></a:stretch>
  </p:blipFill>
  <p:spPr>
    <a:xfrm>
      <a:off x="914400" y="914400"/>
      <a:ext cx="1828800" cy="1828800"/>
    </a:xfrm>
    <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
  </p:spPr>
</p:pic>`)

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pptx" {
			path := filepath.Join(templateDir, entry.Name())
			t.Run(entry.Name(), func(t *testing.T) {
				pkg, closer, err := OpenFile(path)
				if err != nil {
					t.Fatalf("OpenFile failed: %v", err)
				}
				defer func() { _ = closer.Close() }()

				enum, err := NewSlideEnumerator(pkg)
				if err != nil {
					t.Fatalf("NewSlideEnumerator failed: %v", err)
				}

				// Test on first slide
				if enum.Count() == 0 {
					t.Skip("no slides in template")
				}

				slidePath := enum.PartPath(0)
				slideXML, err := pkg.ReadEntry(slidePath)
				if err != nil {
					t.Fatalf("ReadEntry failed: %v", err)
				}

				// Validate original spTree
				if err := ValidateSpTree(slideXML); err != nil {
					t.Fatalf("original ValidateSpTree failed: %v", err)
				}

				// Count original shapes
				origCount, err := CountShapesInSpTree(slideXML)
				if err != nil {
					t.Fatalf("CountShapesInSpTree failed: %v", err)
				}

				// Insert at end
				result, err := InsertIntoSpTree(slideXML, picXML, InsertAtEnd)
				if err != nil {
					t.Fatalf("InsertIntoSpTree failed: %v", err)
				}

				// Validate result
				if err := ValidateSpTree(result); err != nil {
					t.Fatalf("result ValidateSpTree failed: %v", err)
				}

				// Count should be increased by 1
				newCount, err := CountShapesInSpTree(result)
				if err != nil {
					t.Fatalf("result CountShapesInSpTree failed: %v", err)
				}

				if newCount != origCount+1 {
					t.Errorf("shape count = %d, want %d", newCount, origCount+1)
				}

				// Check that the inserted element is present
				if !strings.Contains(string(result), `id="999"`) {
					t.Error("inserted element not found in result")
				}

				t.Logf("Successfully inserted p:pic (shapes: %d -> %d)", origCount, newCount)
			})
		}
	}
}

// TestInsertIntoSpTree_PreservesNamespaces verifies namespace preservation.
func TestInsertIntoSpTree_PreservesNamespaces(t *testing.T) {
	slideXML := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
      </p:nvGrpSpPr>
      <p:grpSpPr/>
      <p:sp>
        <p:nvSpPr>
          <p:cNvPr id="2" name="Title"/>
        </p:nvSpPr>
      </p:sp>
    </p:spTree>
  </p:cSld>
</p:sld>`)

	picXML := []byte(`<p:pic><p:cNvPr id="3"/></p:pic>`)

	result, err := InsertIntoSpTree(slideXML, picXML, InsertAtEnd)
	if err != nil {
		t.Fatalf("InsertIntoSpTree failed: %v", err)
	}

	resultStr := string(result)

	// All namespace declarations should still be present
	namespaces := []string{
		`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`,
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`,
		`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`,
	}

	for _, ns := range namespaces {
		if !strings.Contains(resultStr, ns) {
			t.Errorf("namespace declaration missing: %s", ns)
		}
	}
}
