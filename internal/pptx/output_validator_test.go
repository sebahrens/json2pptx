package pptx

import (
	"strings"
	"testing"
)

// validPPTXFiles returns a file map for a structurally valid minimal PPTX.
func validPPTXFiles() map[string]string {
	return map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldIdLst>
    <p:sldId id="256" r:id="rId2" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"/>
  </p:sldIdLst>
</p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:xfrm><a:ext cx="914400" cy="457200"/></a:xfrm>
        <a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>
      </p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`,
	}
}

func TestOutputValidator_ValidPPTX(t *testing.T) {
	t.Parallel()

	data := createValidatorTestZIP(validPPTXFiles())
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if !report.IsValid() {
		t.Errorf("expected valid report, got %d findings: %v", len(report.Findings), report.Findings)
	}
	if len(report.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(report.Findings))
	}
}

func TestOutputValidator_StructuralFailure_IsBlocking(t *testing.T) {
	t.Parallel()

	// Missing presentation.xml — structural failure
	files := validPPTXFiles()
	delete(files, "ppt/presentation.xml")

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if report.IsValid() {
		t.Fatal("expected invalid report for missing presentation.xml")
	}

	blocking := report.Blocking()
	if len(blocking) == 0 {
		t.Fatal("expected at least one blocking finding")
	}

	foundOPC := false
	for _, f := range blocking {
		if strings.HasPrefix(f.Code, "OPC_") {
			foundOPC = true
		}
		if f.Severity != SeverityBlocking {
			t.Errorf("blocking finding has wrong severity: %v", f)
		}
	}
	if !foundOPC {
		t.Errorf("expected OPC_ prefixed code in blocking findings, got: %v", blocking)
	}
}

func TestOutputValidator_OOXMLFailure_IsWarning(t *testing.T) {
	t.Parallel()

	// Invalid hex color in srgbClr — OOXML warning, not blocking
	files := validPPTXFiles()
	files["ppt/slides/slide1.xml"] = `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:srgbClr val="notahex"/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	// OOXML errors are warnings — the package is still structurally valid
	if !report.IsValid() {
		t.Errorf("expected valid (no blocking), got blocking findings: %v", report.Blocking())
	}

	warnings := report.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning finding for invalid color")
	}

	foundOOXML := false
	for _, f := range warnings {
		if f.Code == "OOXML_INVALID_COLOR" {
			foundOOXML = true
		}
		if f.Severity != SeverityWarning {
			t.Errorf("warning finding has wrong severity: %v", f)
		}
	}
	if !foundOOXML {
		t.Errorf("expected OOXML_INVALID_COLOR warning, got: %v", warnings)
	}
}

func TestOutputValidator_BothFamilies(t *testing.T) {
	t.Parallel()

	// Missing _rels/.rels (structural) AND duplicate shape IDs (OOXML)
	files := validPPTXFiles()
	delete(files, "_rels/.rels")
	files["ppt/slides/slide1.xml"] = `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="A"/></p:nvSpPr><p:spPr/></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="B"/></p:nvSpPr><p:spPr/></p:sp>
  </p:spTree></p:cSld>
</p:sld>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if report.IsValid() {
		t.Fatal("expected invalid report (structural failure)")
	}

	// Should have both OPC and OOXML findings
	hasOPC := false
	hasOOXML := false
	for _, f := range report.Findings {
		if strings.HasPrefix(f.Code, "OPC_") {
			hasOPC = true
		}
		if strings.HasPrefix(f.Code, "OOXML_") {
			hasOOXML = true
		}
	}

	if !hasOPC {
		t.Error("expected OPC_ findings for missing rels")
	}
	if !hasOOXML {
		t.Error("expected OOXML_ findings for duplicate shape IDs")
	}
}

func TestOutputValidator_DanglingRel_IsBlocking(t *testing.T) {
	t.Parallel()

	// Relationship pointing to non-existent target
	files := validPPTXFiles()
	files["ppt/_rels/presentation.xml.rels"] = `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide99.xml"/>
</Relationships>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if report.IsValid() {
		t.Fatal("expected invalid report for dangling relationship")
	}

	found := false
	for _, f := range report.Blocking() {
		if f.Code == "OPC_DANGLING_REL" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected OPC_DANGLING_REL blocking finding, got: %v", report.Findings)
	}
}

func TestReport_IsValid_EmptyReport(t *testing.T) {
	t.Parallel()
	r := &Report{}
	if !r.IsValid() {
		t.Error("empty report should be valid")
	}
}

func TestReport_HelperMethods(t *testing.T) {
	t.Parallel()

	r := &Report{
		Findings: []Finding{
			{Code: "OPC_MISSING_PART", Severity: SeverityBlocking, Message: "a"},
			{Code: "OOXML_INVALID_COLOR", Severity: SeverityWarning, Message: "b"},
			{Code: "OOXML_DUPLICATE_ID", Severity: SeverityWarning, Message: "c"},
		},
	}

	if r.IsValid() {
		t.Error("report with blocking finding should not be valid")
	}
	if len(r.Blocking()) != 1 {
		t.Errorf("expected 1 blocking, got %d", len(r.Blocking()))
	}
	if len(r.Warnings()) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(r.Warnings()))
	}
}

func TestFinding_ErrorFormat(t *testing.T) {
	t.Parallel()

	f := Finding{
		Code:     "OPC_MISSING_PART",
		Severity: SeverityBlocking,
		Path:     "ppt/presentation.xml",
		Message:  "required part is missing",
	}
	s := f.Error()
	if !strings.Contains(s, "OPC_MISSING_PART") {
		t.Errorf("expected code in error string: %s", s)
	}
	if !strings.Contains(s, "BLOCKING") {
		t.Errorf("expected BLOCKING in error string: %s", s)
	}
	if !strings.Contains(s, "ppt/presentation.xml") {
		t.Errorf("expected path in error string: %s", s)
	}
}
