package pptx

import (
	"strings"
	"testing"
)

func TestOOXMLValidator_ValidSlide(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:xfrm><a:ext cx="914400" cy="457200"/></a:xfrm>
        <a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
      </p:spPr>
    </p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="2" name="Body"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err != nil {
		t.Errorf("expected no errors, got: %v", err)
	}
}

func TestOOXMLValidator_InvalidSrgbClr(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:srgbClr val="accent1"/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Fatal("expected validation error for scheme name in srgbClr")
	}

	errs := v.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != ErrCodeInvalidColor {
		t.Errorf("expected code %s, got %s", ErrCodeInvalidColor, errs[0].Code)
	}
	if !strings.Contains(errs[0].Message, "accent1") {
		t.Errorf("expected error to mention 'accent1', got: %s", errs[0].Message)
	}
}

func TestOOXMLValidator_InvalidSchemeClr(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:schemeClr val="notacolor"/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Fatal("expected validation error for invalid scheme color")
	}

	errs := v.Errors()
	if errs[0].Code != ErrCodeInvalidScheme {
		t.Errorf("expected code %s, got %s", ErrCodeInvalidScheme, errs[0].Code)
	}
}

func TestOOXMLValidator_DuplicateShapeID(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr><p:spPr/></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Body"/></p:nvSpPr><p:spPr/></p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Fatal("expected validation error for duplicate shape ID")
	}

	errs := v.Errors()
	found := false
	for _, e := range errs {
		if e.Code == ErrCodeDuplicateID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected DUPLICATE_ID error, got: %v", errs)
	}
}

func TestOOXMLValidator_TableGridMismatch(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Table"/></p:nvSpPr><p:spPr/>
      <a:tbl>
        <a:tblGrid><a:gridCol w="100"/><a:gridCol w="100"/><a:gridCol w="100"/></a:tblGrid>
        <a:tr h="100"><a:tc><a:txBody/></a:tc><a:tc><a:txBody/></a:tc><a:tc><a:txBody/></a:tc></a:tr>
        <a:tr h="100"><a:tc><a:txBody/></a:tc><a:tc><a:txBody/></a:tc></a:tr>
      </a:tbl>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Fatal("expected validation error for table grid mismatch")
	}

	errs := v.Errors()
	found := false
	for _, e := range errs {
		if e.Code == ErrCodeInvalidTable {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INVALID_TABLE error, got: %v", errs)
	}
}

func TestOOXMLValidator_ZeroExtent(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:nvGrpSpPr><p:cNvPr id="1" name=""/></p:nvGrpSpPr>
    <p:grpSpPr><a:xfrm><a:ext cx="0" cy="0"/></a:xfrm></p:grpSpPr>
    <p:sp><p:nvSpPr><p:cNvPr id="2" name="Invisible"/></p:nvSpPr>
      <p:spPr><a:xfrm><a:ext cx="0" cy="0"/></a:xfrm></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Fatal("expected validation error for zero extent")
	}

	errs := v.Errors()
	found := false
	for _, e := range errs {
		if e.Code == ErrCodeZeroExtent {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ZERO_EXTENT error, got: %v", errs)
	}
}

func TestOOXMLValidator_P0_1_Regression_SchemeNameInSrgbClr(t *testing.T) {
	// This is the exact bug that P0-1 caught: a scheme color name like "accent1"
	// was written into an srgbClr element instead of being used in schemeClr.
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr>
        <a:solidFill><a:srgbClr val="dk1"/></a:solidFill>
      </p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	err = v.Validate()
	if err == nil {
		t.Fatal("P0-1 regression: scheme name 'dk1' in srgbClr was not caught")
	}

	errs := v.Errors()
	if len(errs) == 0 {
		t.Fatal("expected at least one error")
	}
	if errs[0].Code != ErrCodeInvalidColor {
		t.Errorf("expected INVALID_COLOR, got %s", errs[0].Code)
	}
}

func TestOOXMLValidator_IllegalXMLChar(t *testing.T) {
	// 0x01 is illegal in XML 1.0; Office shows the repair prompt when it encounters it.
	slideXML := "<?xml version=\"1.0\"?>\n<p:sld xmlns:p=\"http://schemas.openxmlformats.org/presentationml/2006/main\">" +
		"<p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id=\"1\" name=\"S\"/></p:nvSpPr>" +
		"<p:txBody><a:p xmlns:a=\"http://schemas.openxmlformats.org/drawingml/2006/main\"><a:r><a:t>hello\x01world</a:t></a:r></a:p></p:txBody>" +
		"</p:sp></p:spTree></p:cSld></p:sld>"

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": slideXML,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for illegal XML control character")
	}

	found := false
	for _, e := range v.Errors() {
		if e.Code == ErrCodeIllegalXMLChar {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ILLEGAL_XML_CHAR error, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_LegalXMLChars_NoFalsePositive(t *testing.T) {
	// Tab (0x09), newline (0x0A), CR (0x0D) are legal in XML 1.0.
	slideXML := "<?xml version=\"1.0\"?>\n<p:sld xmlns:p=\"http://schemas.openxmlformats.org/presentationml/2006/main\">" +
		"<p:cSld><p:spTree>\t\n\r</p:spTree></p:cSld></p:sld>"

	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": slideXML,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err != nil {
		for _, e := range v.Errors() {
			if e.Code == ErrCodeIllegalXMLChar {
				t.Errorf("false positive: tab/newline/CR flagged as illegal XML char")
			}
		}
	}
}

func TestOOXMLValidator_SlideCountMismatch(t *testing.T) {
	// presentation.xml lists 1 slide but no slide file exists in the package.
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst></p:presentation>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for slide count mismatch")
	}

	found := false
	for _, e := range v.Errors() {
		if e.Code == ErrCodeSlideMismatch {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SLIDE_COUNT_MISMATCH error, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_EmptySchemeClrVal(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:schemeClr val=""/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for empty schemeClr val")
	}

	var found bool
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr {
			found = true
		}
		if e.Code == ErrCodeInvalidScheme {
			t.Errorf("empty val should not also emit INVALID_SCHEME: %v", e)
		}
	}
	if !found {
		t.Errorf("expected EMPTY_REQUIRED_ATTR error, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_EmptySrgbClrVal(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:srgbClr val=""/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for empty srgbClr val")
	}

	var found bool
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr {
			found = true
		}
		if e.Code == ErrCodeInvalidColor {
			t.Errorf("empty val should not also emit INVALID_COLOR: %v", e)
		}
	}
	if !found {
		t.Errorf("expected EMPTY_REQUIRED_ATTR error, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_EmptyBlipEmbed(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:pic><p:nvPicPr><p:cNvPr id="1" name="Image"/></p:nvPicPr>
      <p:blipFill><a:blip r:embed=""/></p:blipFill>
      <p:spPr/>
    </p:pic>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for empty blip r:embed")
	}

	found := false
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr && strings.Contains(e.Message, "r:embed") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected EMPTY_REQUIRED_ATTR for r:embed, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_EmptyCNvPrID(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="" name="Bad"/></p:nvSpPr><p:spPr/></p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for empty cNvPr id")
	}

	found := false
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr && strings.Contains(e.Message, "cNvPr") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected EMPTY_REQUIRED_ATTR for cNvPr id, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_BlipFillSelfClosing_MissingBlip(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:pic><p:nvPicPr><p:cNvPr id="1" name="Image"/></p:nvPicPr>
      <p:blipFill/>
      <p:spPr/>
    </p:pic>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for self-closing blipFill")
	}

	found := false
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr && strings.Contains(e.Message, "blipFill") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected EMPTY_REQUIRED_ATTR for blipFill, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_BlipFillEmptyBody_MissingBlip(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:pic><p:nvPicPr><p:cNvPr id="1" name="Image"/></p:nvPicPr>
      <p:blipFill><a:srcRect/></p:blipFill>
      <p:spPr/>
    </p:pic>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	if err := v.Validate(); err == nil {
		t.Fatal("expected validation error for blipFill without blip child")
	}

	found := false
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr && strings.Contains(e.Message, "missing required child") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected EMPTY_REQUIRED_ATTR for missing <a:blip>, got: %v", v.Errors())
	}
}

func TestOOXMLValidator_BlipFillWithBlip_NoFalsePositive(t *testing.T) {
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:pic><p:nvPicPr><p:cNvPr id="1" name="Image"/></p:nvPicPr>
      <p:blipFill><a:blip r:embed="rId7"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>
      <p:spPr/>
    </p:pic>
  </p:spTree></p:cSld>
</p:sld>`,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	_ = v.Validate()
	for _, e := range v.Errors() {
		if e.Code == ErrCodeEmptyRequiredAttr {
			t.Errorf("false positive: valid blipFill with blip flagged as empty: %v", e)
		}
	}
}

func TestOOXMLValidator_SlideCountMatch_NoError(t *testing.T) {
	slideXML := `<?xml version="1.0"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld></p:sld>`
	data := createValidatorTestZIP(map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"></Types>`,
		"ppt/presentation.xml": `<?xml version="1.0"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
			`<p:sldIdLst><p:sldId id="256" r:id="rId2"/></p:sldIdLst></p:presentation>`,
		"ppt/slides/slide1.xml": slideXML,
	})

	v, err := NewOOXMLValidator(data)
	if err != nil {
		t.Fatalf("NewOOXMLValidator: %v", err)
	}

	_ = v.Validate()
	for _, e := range v.Errors() {
		if e.Code == ErrCodeSlideMismatch {
			t.Errorf("false positive: slide count matched but SLIDE_COUNT_MISMATCH emitted")
		}
	}
}
