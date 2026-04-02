package generator

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// createTestPPTXWithAppVersion creates a minimal PPTX with specified app version metadata.
func createTestPPTXWithAppVersion(t *testing.T, appVersion, application string) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Required [Content_Types].xml
	contentTypes := `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
</Types>`
	writeZipEntry(t, w, "[Content_Types].xml", contentTypes)

	// Required _rels/.rels
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`
	writeZipEntry(t, w, "_rels/.rels", rels)

	// Minimal presentation.xml
	presentation := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
  <p:sldIdLst/>
</p:presentation>`
	writeZipEntry(t, w, "ppt/presentation.xml", presentation)

	// docProps/app.xml with version info
	if appVersion != "" || application != "" {
		appXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>` + application + `</Application>
  <AppVersion>` + appVersion + `</AppVersion>
</Properties>`
		writeZipEntry(t, w, "docProps/app.xml", appXML)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

// createTestPPTXWithoutAppXML creates a minimal PPTX without docProps/app.xml.
func createTestPPTXWithoutAppXML(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	contentTypes := `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`
	writeZipEntry(t, w, "[Content_Types].xml", contentTypes)

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`
	writeZipEntry(t, w, "_rels/.rels", rels)

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	return buf.Bytes()
}

func writeZipEntry(t *testing.T, w *zip.Writer, name, content string) {
	t.Helper()
	f, err := w.Create(name)
	if err != nil {
		t.Fatalf("failed to create zip entry %s: %v", name, err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write zip entry %s: %v", name, err)
	}
}

func TestCheckSVGCompatibility_PowerPoint2016(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithAppVersion(t, "16.0000", "Microsoft Macintosh PowerPoint")

	reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	checker, err := CheckSVGCompatibilityFromReader(reader)
	if err != nil {
		t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
	}

	if !checker.IsNativeSupported {
		t.Error("expected IsNativeSupported=true for PowerPoint 16.0")
	}
	if checker.MajorVersion != 16 {
		t.Errorf("expected MajorVersion=16, got %d", checker.MajorVersion)
	}
	if checker.AppVersion != "16.0000" {
		t.Errorf("expected AppVersion='16.0000', got %q", checker.AppVersion)
	}
}

func TestCheckSVGCompatibility_PowerPoint2013(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithAppVersion(t, "15.0000", "Microsoft PowerPoint")

	reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	checker, err := CheckSVGCompatibilityFromReader(reader)
	if err != nil {
		t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
	}

	if checker.IsNativeSupported {
		t.Error("expected IsNativeSupported=false for PowerPoint 15.0")
	}
	if checker.MajorVersion != 15 {
		t.Errorf("expected MajorVersion=15, got %d", checker.MajorVersion)
	}
}

func TestCheckSVGCompatibility_NoAppXML(t *testing.T) {
	t.Parallel()

	pptxData := createTestPPTXWithoutAppXML(t)

	reader, err := zip.NewReader(bytes.NewReader(pptxData), int64(len(pptxData)))
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}

	checker, err := CheckSVGCompatibilityFromReader(reader)
	if err != nil {
		t.Fatalf("CheckSVGCompatibilityFromReader failed: %v", err)
	}

	// Unknown version - should assume compatible but with warning
	if checker.MajorVersion != 0 {
		t.Errorf("expected MajorVersion=0 (unknown), got %d", checker.MajorVersion)
	}
	if !checker.IsNativeSupported {
		t.Error("expected IsNativeSupported=true when unknown (optimistic)")
	}
}

func TestGenerateWarning_NativeStrategyUnknownVersion(t *testing.T) {
	t.Parallel()

	checker := &SVGCompatibilityChecker{
		MajorVersion:         0,
		IsNativeSupported:    true,
		CompatibilityMessage: "unknown",
	}

	warning := checker.GenerateWarning(SVGStrategyNative)
	if warning == "" {
		t.Error("expected warning for unknown version with native strategy")
	}
	if !strings.Contains(warning, "unknown") {
		t.Errorf("warning should mention unknown compatibility: %s", warning)
	}
}

func TestGenerateWarning_NativeStrategySupported(t *testing.T) {
	t.Parallel()

	checker := &SVGCompatibilityChecker{
		MajorVersion:         16,
		IsNativeSupported:    true,
		CompatibilityMessage: "supported",
	}

	warning := checker.GenerateWarning(SVGStrategyNative)
	if warning != "" {
		t.Errorf("expected no warning for supported version, got: %s", warning)
	}
}

func TestGenerateWarning_PNGStrategy(t *testing.T) {
	t.Parallel()

	checker := &SVGCompatibilityChecker{
		MajorVersion:         15,
		IsNativeSupported:    false,
		CompatibilityMessage: "not supported",
	}

	warning := checker.GenerateWarning(SVGStrategyPNG)
	if warning != "" {
		t.Errorf("expected no warning for PNG strategy, got: %s", warning)
	}
}

func TestShouldFallback_FallbackMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		majorVersion int
		isSupported  bool
		compatMode   SVGNativeCompatibility
		wantFallback bool
	}{
		{
			name:         "fallback mode, unknown version",
			majorVersion: 0,
			isSupported:  true,
			compatMode:   SVGCompatFallback,
			wantFallback: true,
		},
		{
			name:         "fallback mode, unsupported version",
			majorVersion: 15,
			isSupported:  false,
			compatMode:   SVGCompatFallback,
			wantFallback: true,
		},
		{
			name:         "fallback mode, supported version",
			majorVersion: 16,
			isSupported:  true,
			compatMode:   SVGCompatFallback,
			wantFallback: false,
		},
		{
			name:         "warn mode, unknown version",
			majorVersion: 0,
			isSupported:  true,
			compatMode:   SVGCompatWarn,
			wantFallback: false,
		},
		{
			name:         "ignore mode, unsupported version",
			majorVersion: 15,
			isSupported:  false,
			compatMode:   SVGCompatIgnore,
			wantFallback: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &SVGCompatibilityChecker{
				MajorVersion:      tt.majorVersion,
				IsNativeSupported: tt.isSupported,
			}

			got := checker.ShouldFallback(SVGStrategyNative, tt.compatMode)
			if got != tt.wantFallback {
				t.Errorf("ShouldFallback() = %v, want %v", got, tt.wantFallback)
			}
		})
	}
}

func TestCheckStrict_StrictMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		majorVersion int
		isSupported  bool
		wantErr      bool
	}{
		{
			name:         "strict mode, unknown version",
			majorVersion: 0,
			isSupported:  true,
			wantErr:      true,
		},
		{
			name:         "strict mode, unsupported version",
			majorVersion: 15,
			isSupported:  false,
			wantErr:      true,
		},
		{
			name:         "strict mode, supported version",
			majorVersion: 16,
			isSupported:  true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &SVGCompatibilityChecker{
				MajorVersion:      tt.majorVersion,
				IsNativeSupported: tt.isSupported,
				AppVersion:        "test",
			}

			err := checker.CheckStrict(SVGStrategyNative, SVGCompatStrict)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckStrict() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckStrict_WarnMode(t *testing.T) {
	t.Parallel()

	checker := &SVGCompatibilityChecker{
		MajorVersion:      0,
		IsNativeSupported: true,
	}

	// Warn mode should never return error
	err := checker.CheckStrict(SVGStrategyNative, SVGCompatWarn)
	if err != nil {
		t.Errorf("CheckStrict with warn mode should not error, got: %v", err)
	}
}
