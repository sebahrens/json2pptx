// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/pptx"
)

// SVG rendering compatibility E2E tests.
//
// These tests verify PPTX/SVG rendering behaviors across different PowerPoint versions
// and compatibility scenarios. Test failures include diagnostic messages that indicate:
// 1. Which viewer/version assumption was violated
// 2. Which OOXML structure is incorrect
// 3. Which fallback scenario failed
//
// Reference: specs/19-pptx-svg-compatibility.md

// Test fixtures for SVG and PNG data
const compatTestSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200" viewBox="0 0 200 200">
  <rect x="10" y="10" width="180" height="180" fill="#3498db" rx="10"/>
  <text x="100" y="110" text-anchor="middle" fill="white" font-size="24" font-family="Arial">Compat</text>
</svg>`

// compatTestPNG is a minimal valid PNG (1x1 blue pixel) for fallback
var compatTestPNG = []byte{
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

// OOXML constants for validation - duplicated here to avoid import cycles
const (
	svgBlipExtensionURI = "{96DAC541-7B7A-43D3-8B79-37D633B846F1}"
	svgBlipNamespace    = "http://schemas.microsoft.com/office/drawing/2016/SVG/main"
)

// createCompatTestPPTXWithVersion creates a minimal PPTX with specified PowerPoint version.
// This allows testing compatibility detection logic.
func createCompatTestPPTXWithVersion(t *testing.T, version string, appName string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

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
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
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
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`,
	}

	// Add app.xml with version info if provided
	if version != "" {
		files["docProps/app.xml"] = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>%s</Application>
  <AppVersion>%s</AppVersion>
</Properties>`, appName, version)
	}

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

// CompatibilityError provides detailed diagnostic information for test failures.
type CompatibilityError struct {
	Viewer         string // Which viewer/version was expected
	Assumption     string // What assumption was violated
	OOXMLStructure string // Which OOXML element is affected
	Details        string // Additional diagnostic info
}

func (e CompatibilityError) Error() string {
	return fmt.Sprintf(
		"SVG compatibility test failed:\n"+
			"  Viewer: %s\n"+
			"  Violated assumption: %s\n"+
			"  OOXML structure: %s\n"+
			"  Details: %s",
		e.Viewer, e.Assumption, e.OOXMLStructure, e.Details,
	)
}

// TestNativeSVGEmbedding_PowerPoint2016Plus tests native SVG embedding with the asvg:svgBlip extension.
// This verifies correct OOXML structure for PowerPoint 2016+ compatibility.
func TestNativeSVGEmbedding_PowerPoint2016Plus(t *testing.T) {
	t.Parallel()

	// Create a template with PowerPoint 2016 version metadata
	pptxData := createCompatTestPPTXWithVersion(t, "16.0000", "Microsoft PowerPoint")

	// Open the document
	doc, err := pptx.OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Failed to open fixture: %v", err)
	}

	// Insert SVG with native strategy (includes PNG fallback)
	opts := pptx.InsertOptions{
		SlideIndex: 0,
		Bounds:     pptx.RectFromInches(1.0, 1.5, 5.0, 3.0),
		SVGData:    []byte(compatTestSVG),
		PNGData:    compatTestPNG,
		Name:       "Native SVG Test",
		AltText:    "Testing native SVG embedding for PowerPoint 2016+",
	}

	if err := doc.InsertSVG(opts); err != nil {
		t.Fatalf("InsertSVG failed: %v", err)
	}

	// Save the document
	savedData, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	// Validate the document structure
	validator, err := pptx.NewValidator(savedData)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Test 1: Verify general PPTX structure
	if err := validator.Validate(); err != nil {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "generated PPTX has valid OOXML structure",
			OOXMLStructure: "[general package structure]",
			Details:        err.Error(),
		})
	}

	// Test 2: Verify SVG-specific structure via ValidateSVGInsertion
	if err := validator.ValidateSVGInsertion(0); err != nil {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "SVG insertion creates valid p:pic with svgBlip extension",
			OOXMLStructure: "p:pic/p:blipFill/a:blip/a:extLst/a:ext[@uri='{96DAC541...}']/asvg:svgBlip",
			Details:        err.Error(),
		})
	}

	// Test 3: Verify both media files are present (SVG + PNG fallback)
	mediaCount := validator.CountMedia()
	if mediaCount < 2 {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "native SVG embedding includes both SVG and PNG fallback files",
			OOXMLStructure: "ppt/media/ directory",
			Details:        fmt.Sprintf("expected at least 2 media files (SVG + PNG), got %d", mediaCount),
		})
	}

	// Test 4: Verify asvg:svgBlip extension URI is present
	slideXML, err := validator.SlideXML(0)
	if err != nil {
		t.Fatalf("Failed to get slide XML: %v", err)
	}

	if !bytes.Contains(slideXML, []byte(svgBlipExtensionURI)) {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "native SVG uses correct extension URI for svgBlip",
			OOXMLStructure: "a:ext uri attribute",
			Details:        fmt.Sprintf("expected extension URI %q not found in slide XML", svgBlipExtensionURI),
		})
	}

	// Test 5: Verify asvg namespace is declared
	if !bytes.Contains(slideXML, []byte(svgBlipNamespace)) {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "native SVG uses correct XML namespace",
			OOXMLStructure: "xmlns:asvg declaration",
			Details:        fmt.Sprintf("expected namespace %q not found in slide XML", svgBlipNamespace),
		})
	}

	// Test 6: Verify content types are registered
	pkg, err := pptx.OpenFromBytes(savedData)
	if err != nil {
		t.Fatalf("OpenFromBytes failed: %v", err)
	}

	ctData, err := pkg.ReadEntry("[Content_Types].xml")
	if err != nil {
		t.Fatalf("Failed to read [Content_Types].xml: %v", err)
	}

	ct, err := pptx.ParseContentTypes(ctData)
	if err != nil {
		t.Fatalf("ParseContentTypes failed: %v", err)
	}

	if !ct.HasDefault("svg") || ct.Default("svg") != "image/svg+xml" {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "SVG content type is registered",
			OOXMLStructure: "[Content_Types].xml Default[@Extension='svg']",
			Details:        fmt.Sprintf("expected svg content type 'image/svg+xml', got %q", ct.Default("svg")),
		})
	}

	if !ct.HasDefault("png") || ct.Default("png") != "image/png" {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2016+ (version 16.0)",
			Assumption:     "PNG fallback content type is registered",
			OOXMLStructure: "[Content_Types].xml Default[@Extension='png']",
			Details:        fmt.Sprintf("expected png content type 'image/png', got %q", ct.Default("png")),
		})
	}

	t.Logf("Native SVG embedding test passed: %d media files, svgBlip extension present", mediaCount)
}

// TestPNGFallback_OlderPowerPoint tests PNG fallback behavior for viewers without native SVG support.
// This verifies that when native strategy is used, PNG fallback data is correctly embedded.
func TestPNGFallback_OlderPowerPoint(t *testing.T) {
	t.Parallel()

	// Create a template with older PowerPoint version (2013)
	pptxData := createCompatTestPPTXWithVersion(t, "15.0000", "Microsoft PowerPoint")

	// First, verify compatibility checker detects the old version
	reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
	if err != nil {
		t.Fatalf("Failed to open zip reader: %v", err)
	}

	checker, err := CheckSVGCompatibilityFromReader(reader)
	if err != nil {
		t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
	}

	// Verify compatibility detection
	if checker.IsNativeSupported {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2013 (version 15.0)",
			Assumption:     "older PowerPoint version correctly detected as not supporting native SVG",
			OOXMLStructure: "docProps/app.xml AppVersion",
			Details:        fmt.Sprintf("MajorVersion=%d, IsNativeSupported=%v (expected false)", checker.MajorVersion, checker.IsNativeSupported),
		})
	}

	if checker.MajorVersion != 15 {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2013 (version 15.0)",
			Assumption:     "version parsing correctly extracts major version",
			OOXMLStructure: "docProps/app.xml AppVersion parsing",
			Details:        fmt.Sprintf("expected MajorVersion=15, got %d", checker.MajorVersion),
		})
	}

	// Even for older templates, native SVG insertion should work (it includes PNG fallback)
	doc, err := pptx.OpenDocumentFromBytes(pptxData)
	if err != nil {
		t.Fatalf("Failed to open document: %v", err)
	}

	opts := pptx.InsertOptions{
		SlideIndex: 0,
		Bounds:     pptx.RectFromInches(1.0, 1.5, 5.0, 3.0),
		SVGData:    []byte(compatTestSVG),
		PNGData:    compatTestPNG,
		Name:       "Fallback Test",
		AltText:    "PNG fallback for older viewers",
	}

	if err := doc.InsertSVG(opts); err != nil {
		t.Fatalf("InsertSVG failed: %v", err)
	}

	savedData, err := doc.SaveToBytes()
	if err != nil {
		t.Fatalf("SaveToBytes failed: %v", err)
	}

	validator, err := pptx.NewValidator(savedData)
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}

	// Verify PNG fallback is present in the package
	// When opened in PowerPoint 2013, it will show the PNG while ignoring the svgBlip extension
	mediaCount := validator.CountMedia()
	if mediaCount < 2 {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2013 (version 15.0)",
			Assumption:     "PNG fallback file is embedded alongside SVG",
			OOXMLStructure: "ppt/media/ directory",
			Details:        fmt.Sprintf("expected at least 2 media files for fallback support, got %d", mediaCount),
		})
	}

	// The blip element should reference the PNG as the primary image (for fallback)
	slideXML, err := validator.SlideXML(0)
	if err != nil {
		t.Fatalf("Failed to get slide XML: %v", err)
	}

	// The a:blip element should exist and reference media (PNG is the fallback)
	if !bytes.Contains(slideXML, []byte("<a:blip")) {
		t.Error(CompatibilityError{
			Viewer:         "PowerPoint 2013 (version 15.0)",
			Assumption:     "blip element exists for image embedding",
			OOXMLStructure: "p:blipFill/a:blip",
			Details:        "expected <a:blip> element not found in slide XML",
		})
	}

	t.Logf("PNG fallback test passed: version %d detected, %d media files", checker.MajorVersion, mediaCount)
}

// TestCompatibilityModes verifies the different compatibility mode behaviors.
func TestCompatibilityModes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		version      string // PowerPoint version
		appName      string
		compatMode   SVGNativeCompatibility
		expectFallback bool // Whether fallback should trigger
		expectError    bool // Whether strict mode should error
	}{
		{
			name:           "warn_mode_unknown_version",
			version:        "", // No version info
			appName:        "",
			compatMode:     SVGCompatWarn,
			expectFallback: false,
			expectError:    false,
		},
		{
			name:           "fallback_mode_unknown_version",
			version:        "", // No version info
			appName:        "",
			compatMode:     SVGCompatFallback,
			expectFallback: true, // Should fallback when unknown
			expectError:    false,
		},
		{
			name:           "strict_mode_unknown_version",
			version:        "", // No version info
			appName:        "",
			compatMode:     SVGCompatStrict,
			expectFallback: false,
			expectError:    true, // Should error when unknown
		},
		{
			name:           "fallback_mode_old_version",
			version:        "15.0000",
			appName:        "Microsoft PowerPoint",
			compatMode:     SVGCompatFallback,
			expectFallback: true,
			expectError:    false,
		},
		{
			name:           "strict_mode_old_version",
			version:        "15.0000",
			appName:        "Microsoft PowerPoint",
			compatMode:     SVGCompatStrict,
			expectFallback: false,
			expectError:    true,
		},
		{
			name:           "ignore_mode_old_version",
			version:        "15.0000",
			appName:        "Microsoft PowerPoint",
			compatMode:     SVGCompatIgnore,
			expectFallback: false,
			expectError:    false,
		},
		{
			name:           "warn_mode_supported_version",
			version:        "16.0000",
			appName:        "Microsoft PowerPoint",
			compatMode:     SVGCompatWarn,
			expectFallback: false,
			expectError:    false,
		},
		{
			name:           "strict_mode_supported_version",
			version:        "16.0000",
			appName:        "Microsoft PowerPoint",
			compatMode:     SVGCompatStrict,
			expectFallback: false,
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pptxData := createCompatTestPPTXWithVersion(t, tc.version, tc.appName)

			reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
			if err != nil {
				t.Fatalf("Failed to open zip reader: %v", err)
			}

			checker, err := CheckSVGCompatibilityFromReader(reader)
			if err != nil {
				t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
			}

			// Test ShouldFallback
			gotFallback := checker.ShouldFallback(SVGStrategyNative, tc.compatMode)
			if gotFallback != tc.expectFallback {
				versionInfo := "unknown"
				if tc.version != "" {
					versionInfo = tc.version
				}
				t.Error(CompatibilityError{
					Viewer:         fmt.Sprintf("PowerPoint (%s)", versionInfo),
					Assumption:     fmt.Sprintf("compatibility mode %q triggers fallback=%v", tc.compatMode, tc.expectFallback),
					OOXMLStructure: "SVGCompatibilityChecker.ShouldFallback()",
					Details:        fmt.Sprintf("got fallback=%v, expected=%v", gotFallback, tc.expectFallback),
				})
			}

			// Test CheckStrict
			err = checker.CheckStrict(SVGStrategyNative, tc.compatMode)
			gotError := err != nil
			if gotError != tc.expectError {
				versionInfo := "unknown"
				if tc.version != "" {
					versionInfo = tc.version
				}
				t.Error(CompatibilityError{
					Viewer:         fmt.Sprintf("PowerPoint (%s)", versionInfo),
					Assumption:     fmt.Sprintf("compatibility mode %q produces error=%v", tc.compatMode, tc.expectError),
					OOXMLStructure: "SVGCompatibilityChecker.CheckStrict()",
					Details:        fmt.Sprintf("got error=%v (%v), expected error=%v", gotError, err, tc.expectError),
				})
			}
		})
	}
}

// TestWarningMessages verifies that warning messages are generated appropriately.
func TestWarningMessages(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		version       string
		strategy      SVGConversionStrategy
		expectWarning bool
		warnContains  []string
	}{
		{
			name:          "native_unknown_version_warns",
			version:       "", // Unknown
			strategy:      SVGStrategyNative,
			expectWarning: true,
			warnContains:  []string{"unknown", "compatibility"},
		},
		{
			name:          "native_old_version_warns",
			version:       "15.0000",
			strategy:      SVGStrategyNative,
			expectWarning: true,
			warnContains:  []string{"15.0000", "2016"},
		},
		{
			name:          "native_supported_version_no_warning",
			version:       "16.0000",
			strategy:      SVGStrategyNative,
			expectWarning: false,
		},
		{
			name:          "png_strategy_no_warning",
			version:       "15.0000", // Old version but PNG strategy
			strategy:      SVGStrategyPNG,
			expectWarning: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pptxData := createCompatTestPPTXWithVersion(t, tc.version, "Microsoft PowerPoint")

			reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
			if err != nil {
				t.Fatalf("Failed to open zip reader: %v", err)
			}

			checker, err := CheckSVGCompatibilityFromReader(reader)
			if err != nil {
				t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
			}

			warning := checker.GenerateWarning(tc.strategy)
			gotWarning := warning != ""

			if gotWarning != tc.expectWarning {
				versionInfo := "unknown"
				if tc.version != "" {
					versionInfo = tc.version
				}
				t.Error(CompatibilityError{
					Viewer:         fmt.Sprintf("PowerPoint (%s)", versionInfo),
					Assumption:     fmt.Sprintf("strategy %q with version %q produces warning=%v", tc.strategy, versionInfo, tc.expectWarning),
					OOXMLStructure: "SVGCompatibilityChecker.GenerateWarning()",
					Details:        fmt.Sprintf("got warning=%v, expected=%v, message=%q", gotWarning, tc.expectWarning, warning),
				})
			}

			if tc.expectWarning && len(tc.warnContains) > 0 {
				for _, substr := range tc.warnContains {
					if !strings.Contains(strings.ToLower(warning), strings.ToLower(substr)) {
						t.Errorf("Warning message should contain %q, got: %s", substr, warning)
					}
				}
			}
		})
	}
}

// TestVersionParsing verifies version detection from various docProps/app.xml formats.
func TestVersionParsing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		version         string
		appName         string
		expectMajor     int
		expectSupported bool
	}{
		{
			name:            "powerpoint_2016",
			version:         "16.0000",
			appName:         "Microsoft PowerPoint",
			expectMajor:     16,
			expectSupported: true,
		},
		{
			name:            "powerpoint_365",
			version:         "16.0.14326.20384",
			appName:         "Microsoft Office PowerPoint",
			expectMajor:     16,
			expectSupported: true,
		},
		{
			name:            "powerpoint_2019",
			version:         "16.0",
			appName:         "Microsoft Macintosh PowerPoint",
			expectMajor:     16,
			expectSupported: true,
		},
		{
			name:            "powerpoint_2013",
			version:         "15.0000",
			appName:         "Microsoft PowerPoint",
			expectMajor:     15,
			expectSupported: false,
		},
		{
			name:            "powerpoint_2010",
			version:         "14.0000",
			appName:         "Microsoft PowerPoint",
			expectMajor:     14,
			expectSupported: false,
		},
		{
			name:            "no_version_info",
			version:         "",
			appName:         "",
			expectMajor:     0,
			expectSupported: true, // Unknown = optimistic default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pptxData := createCompatTestPPTXWithVersion(t, tc.version, tc.appName)

			reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
			if err != nil {
				t.Fatalf("Failed to open zip reader: %v", err)
			}

			checker, err := CheckSVGCompatibilityFromReader(reader)
			if err != nil {
				t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
			}

			if checker.MajorVersion != tc.expectMajor {
				t.Error(CompatibilityError{
					Viewer:         tc.appName,
					Assumption:     fmt.Sprintf("version %q parses to major version %d", tc.version, tc.expectMajor),
					OOXMLStructure: "docProps/app.xml AppVersion parsing",
					Details:        fmt.Sprintf("got MajorVersion=%d, expected=%d", checker.MajorVersion, tc.expectMajor),
				})
			}

			if checker.IsNativeSupported != tc.expectSupported {
				t.Error(CompatibilityError{
					Viewer:         tc.appName,
					Assumption:     fmt.Sprintf("version %d %s native SVG", tc.expectMajor, map[bool]string{true: "supports", false: "does not support"}[tc.expectSupported]),
					OOXMLStructure: "SVGCompatibilityChecker.IsNativeSupported",
					Details:        fmt.Sprintf("got IsNativeSupported=%v, expected=%v (MinNativeSVGVersion=%d)", checker.IsNativeSupported, tc.expectSupported, MinNativeSVGVersion),
				})
			}
		})
	}
}
