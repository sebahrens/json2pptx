package pptx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// createValidPPTX creates a minimal valid PPTX for validator testing.
func createValidPPTX() []byte {
	return createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
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
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld>
    <p:spTree>
      <p:nvGrpSpPr>
        <p:cNvPr id="1" name=""/>
        <p:cNvGrpSpPr/>
        <p:nvPr/>
      </p:nvGrpSpPr>
      <p:grpSpPr/>
    </p:spTree>
  </p:cSld>
</p:sld>`,
	})
}

// createValidatorTestZIP creates a test ZIP archive from a map of files.
func createValidatorTestZIP(files map[string]string) []byte {
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

func TestNewValidator(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	if v.Package() == nil {
		t.Error("Package() returned nil")
	}
}

func TestNewValidator_InvalidData(t *testing.T) {
	t.Parallel()

	_, err := NewValidator([]byte("not a zip"))
	if err == nil {
		t.Error("expected error for invalid ZIP data")
	}
}

func TestValidator_Validate_ValidPPTX(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.Validate()
	if err != nil {
		t.Errorf("Validate failed on valid PPTX: %v", err)
	}
}

func TestValidator_MissingContentTypes(t *testing.T) {
	t.Parallel()

	// Create PPTX without [Content_Types].xml
	data := createValidatorTestZIP(map[string]string{
		"_rels/.rels":          `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Error("expected validation error for missing [Content_Types].xml")
	}

	// Check specific error
	errors := v.Errors()
	found := false
	for _, e := range errors {
		if e.Code == ErrCodeMissingPart && e.Path == ContentTypesPath {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MISSING_PART error for [Content_Types].xml")
	}
}

func TestValidator_MissingPackageRels(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":  `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Error("expected validation error for missing _rels/.rels")
	}

	errors := v.Errors()
	found := false
	for _, e := range errors {
		if e.Code == ErrCodeMissingPart && e.Path == "_rels/.rels" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MISSING_PART error for _rels/.rels")
	}
}

func TestValidator_MissingPresentation(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":         `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Error("expected validation error for missing presentation.xml")
	}
}

func TestValidator_DanglingRelationship(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels": `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide999.xml"/>
</Relationships>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Error("expected validation error for dangling relationship")
	}

	errors := v.Errors()
	found := false
	for _, e := range errors {
		if e.Code == ErrCodeDanglingRel {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected DANGLING_REL error")
	}
}

func TestValidator_CountSlides(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	count := v.CountSlides()
	if count != 1 {
		t.Errorf("CountSlides = %d, want 1", count)
	}
}

func TestValidator_CountMedia(t *testing.T) {
	t.Parallel()

	// Create PPTX with media files
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":  `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="png" ContentType="image/png"/></Types>`,
		"_rels/.rels":          `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/media/image1.png": "fake png data",
		"ppt/media/image2.png": "fake png data",
		"ppt/media/chart.svg":  "fake svg data",
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	count := v.CountMedia()
	if count != 3 {
		t.Errorf("CountMedia = %d, want 3", count)
	}
}

func TestValidator_HasMediaFile(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":  `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":          `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/media/image1.png": "fake png data",
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	if !v.HasMediaFile("image1.png") {
		t.Error("HasMediaFile(image1.png) = false, want true")
	}
	if v.HasMediaFile("nonexistent.png") {
		t.Error("HasMediaFile(nonexistent.png) = true, want false")
	}
}

func TestValidator_CountSVG(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="svg" ContentType="image/svg+xml"/></Types>`,
		"_rels/.rels":           `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml":  `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/media/chart1.svg":  "<svg></svg>",
		"ppt/media/chart2.svg":  "<svg></svg>",
		"ppt/media/image1.png":  "fake png data",
		"ppt/media/image2.jpeg": "fake jpeg data",
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	if got := v.CountSVG(); got != 2 {
		t.Errorf("CountSVG = %d, want 2", got)
	}
}

func TestValidator_CountPNG(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":  `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="png" ContentType="image/png"/></Types>`,
		"_rels/.rels":          `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/media/chart1.svg": "<svg></svg>",
		"ppt/media/image1.png": "fake png data",
		"ppt/media/image2.png": "fake png data",
		"ppt/media/image3.png": "fake png data",
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	if got := v.CountPNG(); got != 3 {
		t.Errorf("CountPNG = %d, want 3", got)
	}
}

func TestValidator_GetMediaStats(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":           `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml":  `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/media/chart1.svg":  "<svg></svg>",
		"ppt/media/chart2.svg":  "<svg></svg>",
		"ppt/media/image1.png":  "fake png data",
		"ppt/media/image2.jpeg": "fake jpeg data",
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	stats := v.MediaStats()

	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	if stats.SVG != 2 {
		t.Errorf("SVG = %d, want 2", stats.SVG)
	}
	if stats.PNG != 1 {
		t.Errorf("PNG = %d, want 1", stats.PNG)
	}
	if stats.Other != 1 {
		t.Errorf("Other = %d, want 1", stats.Other)
	}
	if len(stats.SVGFiles) != 2 {
		t.Errorf("SVGFiles count = %d, want 2", len(stats.SVGFiles))
	}
	if len(stats.PNGFiles) != 1 {
		t.Errorf("PNGFiles count = %d, want 1", len(stats.PNGFiles))
	}
}

func TestValidator_RequireXMLElement(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Test with existing elements
	presData, _ := v.Part("ppt/presentation.xml")
	v.RequireXMLElement("ppt/presentation.xml", presData, "p:presentation", "sldIdLst")

	if v.HasErrors() {
		t.Errorf("unexpected errors: %v", v.Errors())
	}

	// Reset and test with missing element
	v.errors = nil
	v.RequireXMLElement("ppt/presentation.xml", presData, "nonexistent")

	if !v.HasErrors() {
		t.Error("expected error for missing element")
	}
}

func TestValidator_ValidateSlide(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.ValidateSlide(0)
	if err != nil {
		t.Errorf("ValidateSlide(0) failed: %v", err)
	}

	// Test invalid slide index
	err = v.ValidateSlide(99)
	if err == nil {
		t.Error("expected error for invalid slide index")
	}
}

func TestValidator_AssertSlideContains(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Test existing content
	err = v.AssertSlideContains(0, "p:sld", "p:cSld", "p:spTree")
	if err != nil {
		t.Errorf("AssertSlideContains failed: %v", err)
	}

	// Test missing content
	err = v.AssertSlideContains(0, "nonexistent-element")
	if err == nil {
		t.Error("expected error for missing pattern")
	}
}

func TestValidator_SlideXML(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	xml, err := v.SlideXML(0)
	if err != nil {
		t.Fatalf("SlideXML failed: %v", err)
	}

	if !bytes.Contains(xml, []byte("p:sld")) {
		t.Error("SlideXML should contain p:sld")
	}
}

func TestValidator_DumpStructure(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	dump := v.DumpStructure()
	if dump == "" {
		t.Error("DumpStructure returned empty string")
	}
	if !bytes.Contains([]byte(dump), []byte("PPTX Structure:")) {
		t.Error("DumpStructure should contain header")
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Parallel()

	// Empty errors
	var empty ValidationErrors
	if empty.Error() != "no validation errors" {
		t.Error("empty errors should say 'no validation errors'")
	}

	// Single error
	single := ValidationErrors{
		{Path: "test.xml", Code: "TEST", Message: "test error"},
	}
	msg := single.Error()
	if msg != "[TEST] test.xml: test error" {
		t.Errorf("single error message = %q", msg)
	}

	// Multiple errors
	multiple := ValidationErrors{
		{Path: "a.xml", Code: "ERR1", Message: "error 1"},
		{Path: "b.xml", Code: "ERR2", Message: "error 2"},
	}
	msg = multiple.Error()
	if !bytes.Contains([]byte(msg), []byte("2 validation errors")) {
		t.Error("multiple errors should say '2 validation errors'")
	}
}

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	// With path
	err := ValidationError{Path: "test.xml", Code: "TEST", Message: "test"}
	if err.Error() != "[TEST] test.xml: test" {
		t.Errorf("error with path = %q", err.Error())
	}

	// Without path
	err = ValidationError{Code: "TEST", Message: "test"}
	if err.Error() != "[TEST] test" {
		t.Errorf("error without path = %q", err.Error())
	}
}

func TestResolveRelativeTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		basePath string
		target   string
		want     string
	}{
		{"ppt/slides/", "../media/image1.png", "ppt/media/image1.png"},
		{"ppt/slides/", "slide2.xml", "ppt/slides/slide2.xml"},
		{"ppt/", "slides/slide1.xml", "ppt/slides/slide1.xml"},
		{"", "ppt/presentation.xml", "ppt/presentation.xml"},
		{"ppt/slides/", "/ppt/media/image.png", "ppt/media/image.png"},
		{"ppt/slides/subfolder/", "../../media/image.png", "ppt/media/image.png"},
	}

	for _, tt := range tests {
		t.Run(tt.basePath+"_"+tt.target, func(t *testing.T) {
			got := resolveRelativeTarget(tt.basePath, tt.target)
			if got != tt.want {
				t.Errorf("resolveRelativeTarget(%q, %q) = %q, want %q",
					tt.basePath, tt.target, got, tt.want)
			}
		})
	}
}

func TestGetBasePathForRels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		relsPath string
		want     string
	}{
		{"ppt/slides/_rels/slide1.xml.rels", "ppt/slides/"},
		{"ppt/_rels/presentation.xml.rels", "ppt/"},
		{"_rels/.rels", ""},
	}

	for _, tt := range tests {
		t.Run(tt.relsPath, func(t *testing.T) {
			got := getBasePathForRels(tt.relsPath)
			if got != tt.want {
				t.Errorf("getBasePathForRels(%q) = %q, want %q",
					tt.relsPath, got, tt.want)
			}
		})
	}
}

func TestValidator_HasPart(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	if !v.HasPart("[Content_Types].xml") {
		t.Error("HasPart([Content_Types].xml) = false, want true")
	}
	if v.HasPart("nonexistent.xml") {
		t.Error("HasPart(nonexistent.xml) = true, want false")
	}
}

func TestValidator_GetPart(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	content, err := v.Part("[Content_Types].xml")
	if err != nil {
		t.Fatalf("GetPart failed: %v", err)
	}
	if !bytes.Contains(content, []byte("Types")) {
		t.Error("content should contain Types")
	}

	_, err = v.Part("nonexistent.xml")
	if err == nil {
		t.Error("expected error for nonexistent part")
	}
}

func TestValidator_ContentTypesXMLData(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	ctData, err := v.ContentTypesXMLData()
	if err != nil {
		t.Fatalf("ContentTypesXMLData failed: %v", err)
	}
	if !bytes.Contains(ctData, []byte("Types")) {
		t.Error("content should contain Types")
	}
}

func TestValidator_RelationshipsXMLData(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	relsData, err := v.RelationshipsXMLData("ppt/presentation.xml")
	if err != nil {
		t.Fatalf("RelationshipsXMLData failed: %v", err)
	}
	if !bytes.Contains(relsData, []byte("Relationships")) {
		t.Error("content should contain Relationships")
	}
}

func TestValidator_AssertRelationshipExists(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Test existing relationship type
	err = v.AssertRelationshipExists("ppt/_rels/presentation.xml.rels", RelTypeSlide)
	if err != nil {
		t.Errorf("AssertRelationshipExists failed for existing type: %v", err)
	}

	// Test non-existing relationship type
	err = v.AssertRelationshipExists("ppt/_rels/presentation.xml.rels", "http://example.com/nonexistent")
	if err == nil {
		t.Error("expected error for non-existing relationship type")
	}
}

func TestValidator_ValidateSVGInsertion(t *testing.T) {
	t.Parallel()

	// Create PPTX with SVG insertion
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="svg" ContentType="image/svg+xml"/>
  <Default Extension="png" ContentType="image/png"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
</Types>`,
		"_rels/.rels":          `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:cSld><p:spTree>
    <p:pic>
      <p:blipFill>
        <a:blip xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
          <a:extLst>
            <a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">
              <asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main" r:embed="rId2"/>
            </a:ext>
          </a:extLst>
        </a:blip>
      </p:blipFill>
    </p:pic>
  </p:spTree></p:cSld>
</p:sld>`,
		"ppt/media/image1.svg": "<svg></svg>",
		"ppt/media/image2.png": "fake png",
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.ValidateSVGInsertion(0)
	if err != nil {
		t.Errorf("ValidateSVGInsertion failed: %v", err)
	}
}

func TestValidator_ValidateSVGInsertion_MissingSVG(t *testing.T) {
	t.Parallel()

	// Create PPTX without SVG file
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="png" ContentType="image/png"/>
</Types>`,
		"_rels/.rels":           `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml":  `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.ValidateSVGInsertion(0)
	if err == nil {
		t.Error("expected error for missing SVG")
	}
}

func TestValidator_ExternalRelationship(t *testing.T) {
	t.Parallel()

	// Create PPTX with external relationship (hyperlink)
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":           `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml":  `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// External relationships should not cause dangling rel errors
	err = v.Validate()
	if err != nil {
		t.Errorf("Validate should not fail for external relationship: %v", err)
	}
}

func TestValidator_MalformedXML(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":  "not valid xml <<<<",
		"_rels/.rels":          `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst/></p:presentation>`,
	})

	v, err := NewValidator(data)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Error("expected error for malformed XML")
	}

	errors := v.Errors()
	found := false
	for _, e := range errors {
		if e.Code == ErrCodeMalformedXML {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MALFORMED_XML error")
	}
}

func TestNewValidatorFromPackage(t *testing.T) {
	t.Parallel()

	data := createValidPPTX()
	pkg, err := OpenFromBytes(data)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	v := NewValidatorFromPackage(pkg)
	if v == nil {
		t.Fatal("NewValidatorFromPackage returned nil")
	}

	err = v.Validate()
	if err != nil {
		t.Errorf("Validate failed: %v", err)
	}
}
