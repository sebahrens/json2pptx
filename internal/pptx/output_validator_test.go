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

func TestOutputValidator_Provenance_SourceMappableOOXML(t *testing.T) {
	t.Parallel()

	// Invalid color on slide1 — should map to source slide index 0.
	files := validPPTXFiles()
	files["ppt/slides/slide1.xml"] = `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="1" name="Title"/></p:nvSpPr>
      <p:spPr><a:solidFill><a:srgbClr val="ZZZZZZ"/></a:solidFill></p:spPr>
    </p:sp>
  </p:spTree></p:cSld>
</p:sld>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	warnings := report.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected at least one warning")
	}

	f := warnings[0]
	if f.Phase != "ooxml" {
		t.Errorf("expected phase=ooxml, got %q", f.Phase)
	}
	if f.Validator != "ooxml_content" {
		t.Errorf("expected validator=ooxml_content, got %q", f.Validator)
	}
	if f.SlideIndex != 0 {
		t.Errorf("expected slide_index=0, got %d", f.SlideIndex)
	}
	if f.SourcePath != "/slides/0" {
		t.Errorf("expected source_path=/slides/0, got %q", f.SourcePath)
	}
	if f.Scope != RepairScopeSource {
		t.Errorf("expected scope=source, got %q", f.Scope)
	}
}

func TestOutputValidator_Provenance_UnmappablePackageLevel(t *testing.T) {
	t.Parallel()

	// Missing _rels/.rels — package-level, no slide mapping.
	files := validPPTXFiles()
	delete(files, "_rels/.rels")

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	blocking := report.Blocking()
	if len(blocking) == 0 {
		t.Fatal("expected at least one blocking finding")
	}

	// Find the missing rels finding.
	var found *Finding
	for i := range blocking {
		if blocking[i].Code == "OPC_MISSING_PART" && blocking[i].Path == "_rels/.rels" {
			found = &blocking[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected OPC_MISSING_PART for _rels/.rels, got: %v", blocking)
	}

	if found.Phase != "opc" {
		t.Errorf("expected phase=opc, got %q", found.Phase)
	}
	if found.Validator != "structural" {
		t.Errorf("expected validator=structural, got %q", found.Validator)
	}
	if found.SlideIndex != -1 {
		t.Errorf("expected slide_index=-1 for package-level finding, got %d", found.SlideIndex)
	}
	if found.SourcePath != "" {
		t.Errorf("expected empty source_path for package-level finding, got %q", found.SourcePath)
	}
	if found.Scope != RepairScopeGenerator {
		t.Errorf("expected scope=generator, got %q", found.Scope)
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

func TestOutputValidator_DuplicateRelID_IsBlocking(t *testing.T) {
	t.Parallel()

	// Two relationships sharing Id="rId2" — Office shows the repair prompt.
	files := validPPTXFiles()
	files["ppt/_rels/presentation.xml.rels"] = `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if report.IsValid() {
		t.Fatal("expected invalid report for duplicate rId")
	}

	var found *Finding
	for i := range report.Blocking() {
		f := report.Blocking()[i]
		if f.Code == "OPC_DUPLICATE_REL_ID" && f.Path == "ppt/_rels/presentation.xml.rels" {
			found = &f
			break
		}
	}
	if found == nil {
		t.Fatalf("expected OPC_DUPLICATE_REL_ID for ppt/_rels/presentation.xml.rels, got: %v", report.Findings)
	}
	if !strings.Contains(found.Message, "rId2") {
		t.Errorf("expected message to name the duplicate id 'rId2', got %q", found.Message)
	}
	if found.Severity != SeverityBlocking {
		t.Errorf("expected blocking severity, got %q", found.Severity)
	}
}

func TestOutputValidator_DuplicateRelID_OnePerID(t *testing.T) {
	t.Parallel()

	// rId2 appears 3 times — expect exactly one finding for that id (not three).
	files := validPPTXFiles()
	files["ppt/_rels/presentation.xml.rels"] = `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	dupCount := 0
	for _, f := range report.Blocking() {
		if f.Code == "OPC_DUPLICATE_REL_ID" {
			dupCount++
		}
	}
	if dupCount != 1 {
		t.Errorf("expected exactly 1 OPC_DUPLICATE_REL_ID finding for the repeated id, got %d", dupCount)
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

func TestOutputValidator_IllegalXMLChar_IsBlocking(t *testing.T) {
	t.Parallel()

	files := validPPTXFiles()
	// Embed a 0x01 control char in slide text — illegal in XML 1.0.
	files["ppt/slides/slide1.xml"] = "<?xml version=\"1.0\"?>\n" +
		"<p:sld xmlns:p=\"http://schemas.openxmlformats.org/presentationml/2006/main\"" +
		" xmlns:a=\"http://schemas.openxmlformats.org/drawingml/2006/main\">" +
		"<p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id=\"1\" name=\"S\"/></p:nvSpPr>" +
		"<p:spPr/><p:txBody><a:p><a:r><a:t>bad\x01char</a:t></a:r></a:p></p:txBody>" +
		"</p:sp></p:spTree></p:cSld></p:sld>"

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if report.IsValid() {
		t.Fatal("expected invalid report: illegal XML char should be blocking")
	}

	var found *Finding
	for i := range report.Blocking() {
		f := report.Blocking()[i]
		if f.Code == "OOXML_ILLEGAL_XML_CHAR" {
			found = &f
			break
		}
	}
	if found == nil {
		t.Fatalf("expected OOXML_ILLEGAL_XML_CHAR blocking finding, got: %v", report.Findings)
	}
	if found.Scope != RepairScopeGenerator {
		t.Errorf("expected scope=generator for illegal XML char, got %q", found.Scope)
	}
}

func TestOutputValidator_SlideCountMismatch_IsBlocking(t *testing.T) {
	t.Parallel()

	// presentation.xml registers 1 slide ID, but the package has no slide file.
	files := validPPTXFiles()
	delete(files, "ppt/slides/slide1.xml")
	// Also remove the rels entry pointing to the missing slide so it doesn't
	// produce a separate OPC_DANGLING_REL finding that obscures our check.
	files["ppt/_rels/presentation.xml.rels"] = `<?xml version="1.0"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
</Relationships>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	var found *Finding
	for i := range report.Findings {
		f := report.Findings[i]
		if f.Code == "OOXML_SLIDE_COUNT_MISMATCH" {
			found = &f
			break
		}
	}
	if found == nil {
		t.Fatalf("expected OOXML_SLIDE_COUNT_MISMATCH finding, got: %v", report.Findings)
	}
	if found.Severity != SeverityBlocking {
		t.Errorf("expected blocking severity, got %q", found.Severity)
	}
}

func TestOutputValidator_MissingContentTypeOverride_IsBlocking(t *testing.T) {
	t.Parallel()

	// Replace Content_Types so the "xml" Default is removed and only
	// presentation.xml has an explicit Override. slide1.xml is now uncovered:
	// neither a Default for "xml" nor an Override for "/ppt/slides/slide1.xml".
	files := validPPTXFiles()
	files["[Content_Types].xml"] = `<?xml version="1.0"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
</Types>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	if report.IsValid() {
		t.Fatal("expected invalid report for uncovered XML part")
	}

	var found *Finding
	for i := range report.Blocking() {
		f := report.Blocking()[i]
		if f.Code == "OPC_MISSING_CONTENT_TYPE_OVERRIDE" && f.Path == "ppt/slides/slide1.xml" {
			found = &f
			break
		}
	}
	if found == nil {
		t.Fatalf("expected OPC_MISSING_CONTENT_TYPE_OVERRIDE for ppt/slides/slide1.xml, got: %v", report.Findings)
	}
	if found.Severity != SeverityBlocking {
		t.Errorf("expected blocking severity, got %q", found.Severity)
	}
	if found.Phase != "opc" {
		t.Errorf("expected phase=opc, got %q", found.Phase)
	}
	if found.Validator != "structural" {
		t.Errorf("expected validator=structural, got %q", found.Validator)
	}

	// Presentation.xml has an Override — should NOT be flagged.
	for _, f := range report.Findings {
		if f.Code == "OPC_MISSING_CONTENT_TYPE_OVERRIDE" && f.Path == "ppt/presentation.xml" {
			t.Errorf("did not expect OPC_MISSING_CONTENT_TYPE_OVERRIDE for presentation.xml (has Override): %v", f)
		}
	}
}

func TestOutputValidator_ContentTypeCoverage_DefaultExtensionSatisfies(t *testing.T) {
	t.Parallel()

	// validPPTXFiles() has Default Extension="xml" — every XML part is covered
	// by extension Default. No OPC_MISSING_CONTENT_TYPE_OVERRIDE expected.
	data := createValidatorTestZIP(validPPTXFiles())
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	for _, f := range report.Findings {
		if f.Code == "OPC_MISSING_CONTENT_TYPE_OVERRIDE" {
			t.Errorf("did not expect OPC_MISSING_CONTENT_TYPE_OVERRIDE when Default Extension=xml exists: %v", f)
		}
	}
}

func TestOutputValidator_ContentTypeCoverage_UnknownXMLPart(t *testing.T) {
	t.Parallel()

	// Add a stray XML part that has no Default and no Override — must be flagged.
	files := validPPTXFiles()
	files["[Content_Types].xml"] = `<?xml version="1.0"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`
	files["ppt/customXml/item1.xml"] = `<?xml version="1.0"?><root/>`

	data := createValidatorTestZIP(files)
	report, err := ValidateOutputBytes(data)
	if err != nil {
		t.Fatalf("ValidateOutputBytes: %v", err)
	}

	found := false
	for _, f := range report.Blocking() {
		if f.Code == "OPC_MISSING_CONTENT_TYPE_OVERRIDE" && f.Path == "ppt/customXml/item1.xml" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected OPC_MISSING_CONTENT_TYPE_OVERRIDE for ppt/customXml/item1.xml, got: %v", report.Findings)
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
