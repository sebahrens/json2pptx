package pptx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// Test SVG and PNG data for E2E tests
const testSVGData = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
  <rect x="10" y="10" width="180" height="180" fill="#3498db" rx="10"/>
  <text x="100" y="110" text-anchor="middle" fill="white" font-size="24" font-family="Arial">Test</text>
</svg>`

// testPNGData is a minimal valid PNG (1x1 blue pixel)
var testPNGData = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk start
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1 dimensions
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // RGB, 8-bit
	0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk start
	0x54, 0x08, 0xD7, 0x63, 0x68, 0x98, 0xE0, 0x00, // compressed data
	0x00, 0x00, 0x34, 0x00, 0x19, 0xC8, 0x54, 0x6F, // checksum
	0x2D, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, // IEND chunk
	0x44, 0xAE, 0x42, 0x60, 0x82, // IEND CRC
}

// createE2ETestPPTX creates a complete minimal PPTX fixture for E2E testing.
// This includes all required PPTX parts including package relationships.
func createE2ETestPPTX() []byte {
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
                xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2"/>
  </p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr>
        <a:xfrm>
          <a:off x="0" y="0"/>
          <a:ext cx="0" cy="0"/>
          <a:chOff x="0" y="0"/>
          <a:chExt cx="0" cy="0"/>
        </a:xfrm>
      </p:grpSpPr>
    </p:spTree>
  </p:cSld>
</p:sld>`,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			panic("failed to create zip entry: " + err.Error())
		}
		if _, err := w.Write([]byte(content)); err != nil {
			panic("failed to write zip entry: " + err.Error())
		}
	}

	if err := zw.Close(); err != nil {
		panic("failed to close zip: " + err.Error())
	}

	return buf.Bytes()
}

// TestSVGInsertionE2E tests the complete SVG insertion workflow:
// Open fixture -> InsertSVG -> Save -> Validate
func TestSVGInsertionE2E(t *testing.T) {
	t.Parallel()

	// 1. Create a valid PPTX fixture (using helper from api_test.go)
	pptxData := createE2ETestPPTX()

	// 2. Open the document
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Failed to open fixture: %v", err)
	}

	// 3. Insert SVG with PNG fallback
	opts := InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1.0, 1.5, 5.0, 3.0),
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
		Name:       "E2E Test Chart",
		AltText:    "A test chart for E2E validation",
	}

	err = doc.InsertSVG(opts)
	if err != nil {
		t.Fatalf("InsertSVG failed: %v", err)
	}

	// 4. Save the modified document
	savedData, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	// 5. Validate the saved document
	validator, err := NewValidator(savedData)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// 5a. Run general PPTX validation
	if err := validator.Validate(); err != nil {
		t.Errorf("General validation failed: %v", err)
	}

	// 5b. Validate SVG-specific structure
	if err := validator.ValidateSVGInsertion(0); err != nil {
		t.Errorf("SVG insertion validation failed: %v", err)
	}

	// 5c. Check media file counts
	if count := validator.CountMedia(); count < 2 {
		t.Errorf("Expected at least 2 media files (SVG + PNG), got %d", count)
	}

	// 5d. Verify slide contains expected patterns
	if err := validator.AssertSlideContains(0,
		"<p:pic",         // picture element
		"p:blipFill",     // image fill
		"svgBlip",        // SVG extension
		"E2E Test Chart", // shape name
		"E2E validation", // alt text (partial match)
	); err != nil {
		t.Errorf("Slide content assertion failed: %v", err)
	}

	// 5e. Verify slide relationships include image relationships
	slideRelsPath := "ppt/slides/_rels/slide1.xml.rels"
	if err := validator.AssertRelationshipExists(slideRelsPath, RelTypeImage); err != nil {
		t.Errorf("Image relationship not found: %v", err)
	}

	// Log structure for debugging
	t.Logf("E2E test passed. Package structure:\n%s", validator.DumpStructure())
}

// TestSVGInsertionE2E_MultipleInsertions tests inserting multiple SVGs into the same slide.
// Note: The current API generates source IDs based on SlideIndex, so multiple insertions
// on the same slide will reuse the same media files. This test verifies that both p:pic
// elements are created with different shape names.
func TestSVGInsertionE2E_MultipleInsertions(t *testing.T) {
	t.Parallel()

	pptxData := createE2ETestPPTX()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Failed to open fixture: %v", err)
	}

	// Insert first SVG
	err = doc.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(0.5, 0.5, 3.0, 2.0),
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
		Name:       "Chart 1",
	})
	if err != nil {
		t.Fatalf("InsertSVG (1) failed: %v", err)
	}

	// Insert second SVG
	err = doc.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(4.0, 0.5, 3.0, 2.0),
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
		Name:       "Chart 2",
	})
	if err != nil {
		t.Fatalf("InsertSVG (2) failed: %v", err)
	}

	// Save and validate
	savedData, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	validator, err := NewValidator(savedData)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Should have at least 2 media files (SVG + PNG, may reuse if same source ID)
	if count := validator.CountMedia(); count < 2 {
		t.Errorf("Expected at least 2 media files, got %d", count)
	}

	// Validate general structure
	if err := validator.Validate(); err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// Both chart names should appear in the slide
	if err := validator.AssertSlideContains(0, "Chart 1", "Chart 2"); err != nil {
		t.Errorf("Chart names not found: %v", err)
	}

	// Verify slide has two p:pic elements by counting occurrences
	slideXML, err := validator.SlideXML(0)
	if err != nil {
		t.Fatalf("Failed to get slide XML: %v", err)
	}

	picCount := bytes.Count(slideXML, []byte("<p:pic"))
	if picCount != 2 {
		t.Errorf("Expected 2 p:pic elements, got %d", picCount)
	}
}

// TestSVGInsertionE2E_ContentTypesRegistered verifies content types are properly registered
func TestSVGInsertionE2E_ContentTypesRegistered(t *testing.T) {
	t.Parallel()

	pptxData := createE2ETestPPTX()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Failed to open fixture: %v", err)
	}

	err = doc.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1, 1, 4, 3),
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
	})
	if err != nil {
		t.Fatalf("InsertSVG failed: %v", err)
	}

	savedData, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	// Parse saved package and check content types
	pkg, err := OpenFromBytes(savedData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	ctData, err := pkg.ReadEntry(ContentTypesPath)
	if err != nil {
		t.Fatalf("Failed to read [Content_Types].xml: %v", err)
	}

	ct, err := ParseContentTypes(ctData)
	if err != nil {
		t.Fatalf("ParseContentTypes failed: %v", err)
	}

	// Check SVG content type
	if !ct.HasDefault("svg") {
		t.Error("Expected svg Default content type to be registered")
	}

	// Check PNG content type
	if !ct.HasDefault("png") {
		t.Error("Expected png Default content type to be registered")
	}

	// Verify content type values
	if ct.Default("svg") != "image/svg+xml" {
		t.Errorf("SVG content type = %q, want %q", ct.Default("svg"), "image/svg+xml")
	}
	if ct.Default("png") != "image/png" {
		t.Errorf("PNG content type = %q, want %q", ct.Default("png"), "image/png")
	}
}

// TestSVGInsertionE2E_RoundTrip tests that a saved PPTX can be reopened and modified again
func TestSVGInsertionE2E_RoundTrip(t *testing.T) {
	t.Parallel()

	// First round: Create and insert
	pptxData := createE2ETestPPTX()
	doc1, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Round 1: Failed to open fixture: %v", err)
	}

	err = doc1.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1, 1, 3, 2),
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
		Name:       "First Insert",
	})
	if err != nil {
		t.Fatalf("Round 1: InsertSVG failed: %v", err)
	}

	round1Data, err := doc1.SaveToBytes()
	if err != nil {
		t.Fatalf("Round 1: SaveToBytes failed: %v", err)
	}

	// Second round: Open the saved file and insert another SVG
	doc2, err := OpenDocumentFromBytes(round1Data)
	if err != nil {
		t.Fatalf("Round 2: Failed to open saved document: %v", err)
	}

	err = doc2.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(5, 1, 3, 2),
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
		Name:       "Second Insert",
	})
	if err != nil {
		t.Fatalf("Round 2: InsertSVG failed: %v", err)
	}

	round2Data, err := doc2.SaveToBytes()
	if err != nil {
		t.Fatalf("Round 2: SaveToBytes failed: %v", err)
	}

	// Validate final result
	validator, err := NewValidator(round2Data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Should have at least 2 media files (SVG + PNG)
	// Note: Current API may reuse media files for same slide index
	if count := validator.CountMedia(); count < 2 {
		t.Errorf("Expected at least 2 media files after round trip, got %d", count)
	}

	// Both inserts should be present
	if err := validator.AssertSlideContains(0, "First Insert", "Second Insert"); err != nil {
		t.Errorf("Both inserts should be present: %v", err)
	}

	// General validation should pass
	if err := validator.Validate(); err != nil {
		t.Errorf("Final validation failed: %v", err)
	}
}

// TestSVGInsertionE2E_XMLStructure verifies the exact XML structure of the inserted SVG
func TestSVGInsertionE2E_XMLStructure(t *testing.T) {
	t.Parallel()

	pptxData := createE2ETestPPTX()
	doc, err := OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Failed to open fixture: %v", err)
	}

	err = doc.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectEmu{X: 914400, Y: 1828800, CX: 4572000, CY: 2743200}, // 1", 2", 5"x3"
		SVGData:    []byte(testSVGData),
		PNGData:    testPNGData,
		Name:       "XML Test",
		AltText:    "Testing XML structure",
	})
	if err != nil {
		t.Fatalf("InsertSVG failed: %v", err)
	}

	savedData, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	validator, err := NewValidator(savedData)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Get slide XML for detailed inspection
	slideXML, err := validator.SlideXML(0)
	if err != nil {
		t.Fatalf("Failed to get slide XML: %v", err)
	}

	// Verify key XML structure elements
	requiredPatterns := []string{
		// SVG Blip extension
		SVGBlipExtensionURI,
		// SVG namespace
		SVGBlipNamespace,
		// Shape properties
		`<a:off x="914400" y="1828800"`,    // offset
		`<a:ext cx="4572000" cy="2743200"`, // extent
		// Picture element structure
		`<p:cNvPr`,
		`<p:cNvPicPr`,
		`<p:blipFill`,
		`<a:stretch`,
		`<a:fillRect`,
	}

	for _, pattern := range requiredPatterns {
		if !bytes.Contains(slideXML, []byte(pattern)) {
			t.Errorf("Slide XML missing required pattern: %q", pattern)
		}
	}
}

// TestSVGInsertionE2E_DeterministicOutput verifies that the same input produces the same output
func TestSVGInsertionE2E_DeterministicOutput(t *testing.T) {
	t.Parallel()

	generatePPTX := func() []byte {
		pptxData := createE2ETestPPTX()
		doc, _ := OpenDocumentFromBytes(pptxData)
		_ = doc.InsertSVG(InsertOptions{
			SlideIndex: 0,
			Bounds:     RectFromInches(1, 1, 4, 3),
			SVGData:    []byte(testSVGData),
			PNGData:    testPNGData,
			Name:       "Deterministic Test",
		})
		result, _ := doc.SaveToBytes()
		return result
	}

	// Generate twice
	output1 := generatePPTX()
	output2 := generatePPTX()

	// Outputs should be identical
	if !bytes.Equal(output1, output2) {
		t.Error("Deterministic output test failed: two identical operations produced different results")
		t.Logf("Output 1 size: %d bytes", len(output1))
		t.Logf("Output 2 size: %d bytes", len(output2))
	}
}
