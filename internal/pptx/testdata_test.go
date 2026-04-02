package pptx

import (
	"bytes"
	"embed"
	"regexp"
	"strings"
	"testing"
)

//go:embed testdata/*
var testdataFS embed.FS

// loadTestSVG loads the sample SVG from testdata.
func loadTestSVG(t *testing.T) []byte {
	t.Helper()
	data, err := testdataFS.ReadFile("testdata/sample.svg")
	if err != nil {
		t.Fatalf("failed to load testdata/sample.svg: %v", err)
	}
	return data
}

// loadTestPNG loads the sample PNG from testdata.
func loadTestPNG(t *testing.T) []byte {
	t.Helper()
	data, err := testdataFS.ReadFile("testdata/sample.png")
	if err != nil {
		t.Fatalf("failed to load testdata/sample.png: %v", err)
	}
	return data
}

// loadBaselinePPTX loads the minimal baseline PPTX fixture for testing.
// The baseline is a single-slide, empty PPTX with all required parts.
func loadBaselinePPTX(t *testing.T) []byte {
	t.Helper()
	data, err := testdataFS.ReadFile("testdata/baseline.pptx")
	if err != nil {
		t.Fatalf("failed to load testdata/baseline.pptx: %v", err)
	}
	return data
}

// loadReferenceSVGBlip loads the reference OOXML fragments documentation.
func loadReferenceSVGBlip(t *testing.T) []byte {
	t.Helper()
	data, err := testdataFS.ReadFile("testdata/reference_svgblip.xml")
	if err != nil {
		t.Fatalf("failed to load testdata/reference_svgblip.xml: %v", err)
	}
	return data
}

// loadGoldenPicgenSVG loads the golden expected output for picgen SVG.
func loadGoldenPicgenSVG(t *testing.T) []byte {
	t.Helper()
	data, err := testdataFS.ReadFile("testdata/golden/picgen_svg.xml")
	if err != nil {
		t.Fatalf("failed to load testdata/golden/picgen_svg.xml: %v", err)
	}
	return data
}

// normalizeXML normalizes XML for comparison by removing insignificant whitespace.
// This allows golden tests to pass regardless of formatting differences.
func normalizeXML(xml []byte) string {
	s := string(xml)

	// Collapse all whitespace runs (spaces, tabs, newlines) to single spaces
	whitespace := regexp.MustCompile(`\s+`)
	s = whitespace.ReplaceAllString(s, " ")

	// Remove space before closing tags
	s = strings.ReplaceAll(s, " />", "/>")
	s = strings.ReplaceAll(s, " >", ">")

	// Remove space after opening tags
	s = strings.ReplaceAll(s, "> <", "><")

	// Trim
	s = strings.TrimSpace(s)

	return s
}

// TestTestdataLoads verifies that the testdata fixtures can be loaded.
func TestTestdataLoads(t *testing.T) {
	t.Parallel()

	t.Run("SVG loads", func(t *testing.T) {
		t.Parallel()
		data := loadTestSVG(t)
		if len(data) == 0 {
			t.Error("sample.svg is empty")
		}
		// Verify it starts with expected content
		if len(data) < 5 || string(data[:5]) != "<?xml" {
			t.Errorf("sample.svg should start with <?xml, got %q", string(data[:min(5, len(data))]))
		}
	})

	t.Run("PNG loads", func(t *testing.T) {
		t.Parallel()
		data := loadTestPNG(t)
		if len(data) == 0 {
			t.Error("sample.png is empty")
		}
		// Verify PNG signature
		if len(data) < 8 || data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
			t.Error("sample.png does not have valid PNG signature")
		}
	})

	t.Run("Baseline PPTX loads", func(t *testing.T) {
		t.Parallel()
		data := loadBaselinePPTX(t)
		if len(data) == 0 {
			t.Error("baseline.pptx is empty")
		}
		// Verify PK (ZIP) signature
		if len(data) < 4 || data[0] != 'P' || data[1] != 'K' {
			t.Error("baseline.pptx does not have valid ZIP signature")
		}
	})
}

// TestBaselinePPTXCanBeOpened verifies the baseline PPTX is a valid PPTX document.
func TestBaselinePPTXCanBeOpened(t *testing.T) {
	t.Parallel()
	data := loadBaselinePPTX(t)

	doc, err := OpenDocumentFromBytes(data)
	if err != nil {
		t.Fatalf("Failed to open baseline.pptx: %v", err)
	}

	count, err := doc.SlideCount()
	if err != nil {
		t.Fatalf("Failed to get slide count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 slide, got %d", count)
	}

	// Verify we can insert SVG into the baseline
	err = doc.InsertSVG(InsertOptions{
		SlideIndex: 0,
		Bounds:     RectFromInches(1, 1, 4, 3),
		SVGData:    loadTestSVG(t),
		PNGData:    loadTestPNG(t),
		Name:       "Baseline Test",
	})
	if err != nil {
		t.Fatalf("InsertSVG into baseline failed: %v", err)
	}

	// Verify we can save
	saved, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	if len(saved) <= len(data) {
		t.Error("Saved document should be larger than original (contains SVG+PNG)")
	}
}

// TestReferenceSVGBlipFragments verifies the reference XML fragments are valid
// and the generated picgen output matches expected patterns.
func TestReferenceSVGBlipFragments(t *testing.T) {
	t.Parallel()

	refData := loadReferenceSVGBlip(t)

	// Verify key patterns exist in reference
	patterns := []string{
		SVGBlipExtensionURI, // The extension GUID
		SVGBlipNamespace,    // The asvg namespace
		`xmlns:asvg=`,       // Namespace declaration
		`asvg:svgBlip`,      // SVG blip element
		`r:embed=`,          // Relationship embedding
		`<p:pic`,            // Picture element
		`<p:nvPicPr>`,       // Non-visual properties
		`<p:blipFill>`,      // Blip fill
		`<p:spPr>`,          // Shape properties
		`<a:xfrm>`,          // Transform
		`image/svg+xml`,     // SVG content type
		`image/png`,         // PNG content type
	}

	for _, pattern := range patterns {
		if !bytes.Contains(refData, []byte(pattern)) {
			t.Errorf("Reference fragments missing expected pattern: %q", pattern)
		}
	}

	// Verify picgen output contains the same key elements
	picXML, err := GeneratePicWithSVG(4, "rId1", "rId2", 914400, 1371600, 4572000, 2743200)
	if err != nil {
		t.Fatalf("GeneratePicWithSVG failed: %v", err)
	}

	// Key elements that must be in generated output
	requiredInOutput := []string{
		SVGBlipExtensionURI,
		SVGBlipNamespace,
		`asvg:svgBlip`,
		`r:embed="rId1"`, // PNG rel
		`r:embed="rId2"`, // SVG rel
		`<a:off x="914400" y="1371600"`,
		`<a:ext cx="4572000" cy="2743200"`,
	}

	for _, pattern := range requiredInOutput {
		if !bytes.Contains(picXML, []byte(pattern)) {
			t.Errorf("Generated picXML missing required pattern: %q", pattern)
		}
	}
}

// TestGoldenPicgenSVG compares generated p:pic XML against golden expected output.
// This test uses XML normalization to ignore whitespace differences.
func TestGoldenPicgenSVG(t *testing.T) {
	t.Parallel()

	// Generate actual output
	actual, err := GeneratePic(PicOptions{
		ID:          4,
		Name:        "Test Chart",
		Description: "A test chart for golden comparison",
		PNGRelID:    "rId1",
		SVGRelID:    "rId2",
		OffsetX:     914400,
		OffsetY:     1371600,
		ExtentCX:    4572000,
		ExtentCY:    2743200,
	})
	if err != nil {
		t.Fatalf("GeneratePic failed: %v", err)
	}

	// Load golden expected output
	expected := loadGoldenPicgenSVG(t)

	// Normalize both for comparison
	normalizedActual := normalizeXML(actual)
	normalizedExpected := normalizeXML(expected)

	if normalizedActual != normalizedExpected {
		t.Errorf("Generated XML does not match golden file.\n\nExpected:\n%s\n\nActual:\n%s",
			normalizedExpected, normalizedActual)
	}
}

// TestGoldenPicgenSVG_Simple tests simple p:pic generation (PNG only, no SVG).
func TestGoldenPicgenSVG_Simple(t *testing.T) {
	t.Parallel()

	// Generate simple output (no SVG extension)
	actual, err := GeneratePicSimple(5, "rId3", 0, 0, 1828800, 1371600)
	if err != nil {
		t.Fatalf("GeneratePicSimple failed: %v", err)
	}

	// Verify it does NOT contain SVG extension elements
	if bytes.Contains(actual, []byte("asvg:svgBlip")) {
		t.Error("Simple pic should not contain asvg:svgBlip element")
	}
	if bytes.Contains(actual, []byte(SVGBlipExtensionURI)) {
		t.Error("Simple pic should not contain SVG extension URI")
	}

	// Verify it contains required elements
	required := []string{
		`id="5"`,
		`r:embed="rId3"`,
		`<a:off x="0" y="0"`,
		`<a:ext cx="1828800" cy="1371600"`,
	}
	for _, pattern := range required {
		if !bytes.Contains(actual, []byte(pattern)) {
			t.Errorf("Simple pic missing required pattern: %q", pattern)
		}
	}
}
