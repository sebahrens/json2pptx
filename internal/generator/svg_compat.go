// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// SVGCompatibilityChecker detects native SVG support based on template metadata.
type SVGCompatibilityChecker struct {
	// AppVersion is the PowerPoint version from docProps/app.xml (e.g., "16.0000")
	AppVersion string

	// Application is the application name (e.g., "Microsoft Macintosh PowerPoint")
	Application string

	// MajorVersion is the parsed major version number (e.g., 16 for PowerPoint 2016+)
	MajorVersion int

	// IsNativeSupported indicates if native SVG is likely supported
	IsNativeSupported bool

	// CompatibilityMessage provides human-readable compatibility info
	CompatibilityMessage string
}

// MinNativeSVGVersion is the minimum PowerPoint version that supports native SVG.
// PowerPoint 2016 (version 16.0) introduced svgBlip extension support.
const MinNativeSVGVersion = 16

// appPropertiesXML represents the docProps/app.xml structure.
type appPropertiesXML struct {
	XMLName     xml.Name `xml:"Properties"`
	Application string   `xml:"Application"`
	AppVersion  string   `xml:"AppVersion"`
}

// CheckSVGCompatibilityFromReader checks compatibility from an already-open ZIP reader.
func CheckSVGCompatibilityFromReader(reader *zip.Reader) (*SVGCompatibilityChecker, error) {
	return checkSVGCompatibilityFromReader(reader)
}

func checkSVGCompatibilityFromReader(reader *zip.Reader) (*SVGCompatibilityChecker, error) {
	checker := &SVGCompatibilityChecker{
		IsNativeSupported:    true, // Assume supported unless proven otherwise
		CompatibilityMessage: "native SVG compatibility unknown (no app metadata)",
	}

	// Look for docProps/app.xml
	var appFile *zip.File
	for _, f := range reader.File {
		if f.Name == "docProps/app.xml" {
			appFile = f
			break
		}
	}

	if appFile == nil {
		// No app.xml found - can't determine compatibility
		checker.CompatibilityMessage = "native SVG compatibility unknown (docProps/app.xml not found)"
		return checker, nil
	}

	// Read and parse app.xml
	rc, err := appFile.Open()
	if err != nil {
		return checker, nil // Non-fatal - proceed with unknown compatibility
	}
	defer func() { _ = rc.Close() }()

	var props appPropertiesXML
	if err := xml.NewDecoder(rc).Decode(&props); err != nil {
		return checker, nil // Non-fatal - proceed with unknown compatibility
	}

	checker.Application = props.Application
	checker.AppVersion = props.AppVersion

	// Parse version number (format: "16.0000" or "16.0")
	if props.AppVersion != "" {
		parts := strings.Split(props.AppVersion, ".")
		if len(parts) > 0 {
			if major, err := strconv.Atoi(parts[0]); err == nil {
				checker.MajorVersion = major
				checker.IsNativeSupported = major >= MinNativeSVGVersion

				if checker.IsNativeSupported {
					checker.CompatibilityMessage = fmt.Sprintf(
						"native SVG supported (PowerPoint version %d.x detected)",
						major,
					)
				} else {
					checker.CompatibilityMessage = fmt.Sprintf(
						"native SVG NOT supported (PowerPoint version %d.x < 16.0 required)",
						major,
					)
				}
			}
		}
	}

	return checker, nil
}

// GenerateWarning returns a warning message if native SVG compatibility is uncertain.
// Returns empty string if native SVG is confirmed supported or if strategy is not native.
func (c *SVGCompatibilityChecker) GenerateWarning(strategy SVGConversionStrategy) string {
	if strategy != SVGStrategyNative {
		return ""
	}

	if c.MajorVersion == 0 {
		// Unknown version - always warn
		return fmt.Sprintf(
			"native SVG strategy requested but template compatibility is unknown; "+
				"SVG may not display correctly in PowerPoint versions older than 2016; "+
				"PNG fallback will be used for older viewers (%s)",
			c.CompatibilityMessage,
		)
	}

	if !c.IsNativeSupported {
		return fmt.Sprintf(
			"native SVG strategy requested but template was created with %s (version %s); "+
				"SVG will only display in PowerPoint 2016+; "+
				"older versions will show PNG fallback",
			c.Application, c.AppVersion,
		)
	}

	// Native is supported - no warning needed
	return ""
}

// ShouldFallback returns true if the compatibility check suggests falling back to PNG.
// This considers both the template compatibility and the configured compatibility mode.
func (c *SVGCompatibilityChecker) ShouldFallback(
	strategy SVGConversionStrategy,
	compatMode SVGNativeCompatibility,
) bool {
	if strategy != SVGStrategyNative {
		return false
	}

	if compatMode == SVGCompatIgnore {
		return false
	}

	if compatMode == SVGCompatFallback {
		// Fall back if version is unknown or explicitly unsupported
		return c.MajorVersion == 0 || !c.IsNativeSupported
	}

	// For "warn" and "strict" modes, don't auto-fallback
	// (strict will error instead; warn just warns)
	return false
}

// CheckStrict returns an error if strict mode is enabled and compatibility cannot be confirmed.
func (c *SVGCompatibilityChecker) CheckStrict(
	strategy SVGConversionStrategy,
	compatMode SVGNativeCompatibility,
) error {
	if strategy != SVGStrategyNative {
		return nil
	}

	if compatMode != SVGCompatStrict {
		return nil
	}

	if c.MajorVersion == 0 {
		return fmt.Errorf(
			"native SVG strict mode: cannot confirm PowerPoint compatibility; " +
				"set SVG_NATIVE_COMPATIBILITY=warn or specify a compatible template",
		)
	}

	if !c.IsNativeSupported {
		return fmt.Errorf(
			"native SVG strict mode: template was created with PowerPoint version %s, "+
				"which does not support native SVG (requires 16.0+)",
			c.AppVersion,
		)
	}

	return nil
}
