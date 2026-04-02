package template

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ahrens/go-slide-creator/internal/types"
)

// MetadataFilePath is the location of metadata JSON within the PPTX archive.
const MetadataFilePath = "ppt/go-slide-creator-metadata.json"

// MetadataParseError indicates a problem parsing template metadata.
type MetadataParseError struct {
	Path    string // Path within the PPTX archive
	Message string // Human-readable error message
	Cause   error  // Underlying error (if any)
}

func (e *MetadataParseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("metadata parse error in %s: %s: %v", e.Path, e.Message, e.Cause)
	}
	return fmt.Sprintf("metadata parse error in %s: %s", e.Path, e.Message)
}

func (e *MetadataParseError) Unwrap() error {
	return e.Cause
}

// MetadataVersionError indicates an unsupported metadata version.
type MetadataVersionError struct {
	Version    string // The version found
	MinVersion string // Minimum supported version
	MaxVersion string // Maximum (current) supported version
}

func (e *MetadataVersionError) Error() string {
	return fmt.Sprintf("unsupported metadata version %s: supported range is %s to %s",
		e.Version, e.MinVersion, e.MaxVersion)
}

// ParseMetadata reads and parses template metadata from a PPTX reader.
// Returns nil (without error) if no metadata file is present.
// Returns error if metadata is present but malformed or has unsupported version.
func ParseMetadata(reader *Reader) (*types.TemplateMetadata, error) {
	// Check if metadata file exists
	if !reader.hasFile(MetadataFilePath) {
		return nil, nil // No metadata - this is OK
	}

	// Read the metadata file
	data, err := reader.ReadFile(MetadataFilePath)
	if err != nil {
		return nil, &MetadataParseError{
			Path:    MetadataFilePath,
			Message: "failed to read metadata file",
			Cause:   err,
		}
	}

	// Parse JSON
	var metadata types.TemplateMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, &MetadataParseError{
			Path:    MetadataFilePath,
			Message: "invalid JSON",
			Cause:   err,
		}
	}

	// Validate version
	if err := ValidateMetadataVersion(metadata.Version); err != nil {
		return nil, err
	}

	return &metadata, nil
}

// ValidateMetadataVersion checks if a version string is supported.
// Empty version is treated as "1.0" (the minimum version).
func ValidateMetadataVersion(version string) error {
	// Empty version defaults to minimum
	if version == "" {
		return nil
	}

	// Parse version components
	current, err := parseVersion(version)
	if err != nil {
		return &MetadataParseError{
			Path:    MetadataFilePath,
			Message: fmt.Sprintf("invalid version format: %s", version),
			Cause:   err,
		}
	}

	minVersion, _ := parseVersion(types.MetadataVersionMin)
	maxVersion, _ := parseVersion(types.MetadataVersionCurrent)

	// Check if version is in supported range
	if compareVersions(current, minVersion) < 0 || compareVersions(current, maxVersion) > 0 {
		return &MetadataVersionError{
			Version:    version,
			MinVersion: types.MetadataVersionMin,
			MaxVersion: types.MetadataVersionCurrent,
		}
	}

	return nil
}

// versionParts holds parsed version components (major.minor).
type versionParts struct {
	major int
	minor int
}

// parseVersion parses a version string like "1.0" into components.
func parseVersion(version string) (versionParts, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		return versionParts{}, fmt.Errorf("version must be in format major.minor")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return versionParts{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return versionParts{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	return versionParts{major: major, minor: minor}, nil
}

// compareVersions compares two versions.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b versionParts) int {
	if a.major < b.major {
		return -1
	}
	if a.major > b.major {
		return 1
	}
	if a.minor < b.minor {
		return -1
	}
	if a.minor > b.minor {
		return 1
	}
	return 0
}

// NormalizeMetadataVersion returns the effective version for a metadata object.
// If version is empty, returns the minimum supported version.
func NormalizeMetadataVersion(metadata *types.TemplateMetadata) string {
	if metadata == nil || metadata.Version == "" {
		return types.MetadataVersionMin
	}
	return metadata.Version
}

// ApplyMetadataHints merges metadata hints into layout analysis results.
// This updates MaxBullets, MaxChars, and marks deprecated layouts.
func ApplyMetadataHints(layouts []types.LayoutMetadata, metadata *types.TemplateMetadata) {
	if metadata == nil || metadata.LayoutHints == nil {
		return
	}

	for i := range layouts {
		hint, ok := metadata.LayoutHints[layouts[i].ID]
		if !ok {
			// Try by layout name as fallback
			hint, ok = metadata.LayoutHints[layouts[i].Name]
			if !ok {
				continue
			}
		}

		// Apply hint overrides
		if hint.MaxBullets > 0 {
			layouts[i].Capacity.MaxBullets = hint.MaxBullets
		}

		// Apply preferred content tags
		if len(hint.PreferredFor) > 0 {
			// Add preferred tags to layout tags
			for _, pref := range hint.PreferredFor {
				if !sliceContains(layouts[i].Tags, pref) {
					layouts[i].Tags = append(layouts[i].Tags, pref)
				}
			}
		}

		// Mark deprecated layouts
		if hint.Deprecated {
			if !sliceContains(layouts[i].Tags, "deprecated") {
				layouts[i].Tags = append(layouts[i].Tags, "deprecated")
			}
		}
	}
}

// sliceContains checks if a string slice contains a value.
func sliceContains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// ValidationResult contains the outcome of template validation.
type ValidationResult struct {
	// Valid indicates whether the template passed validation.
	// In soft mode, this is true even if there are warnings.
	// In strict mode, this is false if there are any errors or warnings.
	Valid bool

	// Errors are critical issues that always fail validation.
	Errors []string

	// Warnings are issues that fail validation only in strict mode.
	Warnings []string

	// Metadata is the parsed metadata (nil if parsing failed).
	Metadata *types.TemplateMetadata
}

// HasIssues returns true if there are any errors or warnings.
func (vr *ValidationResult) HasIssues() bool {
	return len(vr.Errors) > 0 || len(vr.Warnings) > 0
}

// AllIssues returns a combined list of all errors and warnings.
func (vr *ValidationResult) AllIssues() []string {
	result := make([]string, 0, len(vr.Errors)+len(vr.Warnings))
	result = append(result, vr.Errors...)
	result = append(result, vr.Warnings...)
	return result
}

// ValidateTemplateMetadata validates template metadata with the specified mode.
// In soft mode, metadata errors result in warnings (and nil metadata) but validation passes.
// In strict mode, any metadata errors cause validation to fail.
//
// Parameters:
//   - reader: The template reader to validate
//   - strictMode: If true, warnings become errors
//
// Returns a ValidationResult containing the validation outcome.
func ValidateTemplateMetadata(reader *Reader, strictMode bool) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Attempt to parse metadata
	metadata, err := ParseMetadata(reader)
	if err != nil {
		// Classify the error as warning or error based on type
		switch e := err.(type) {
		case *MetadataParseError:
			// Parse errors are warnings in soft mode, errors in strict mode
			result.Warnings = append(result.Warnings, e.Error())
		case *MetadataVersionError:
			// Version errors are warnings in soft mode, errors in strict mode
			result.Warnings = append(result.Warnings, e.Error())
		default:
			// Unknown errors are always errors
			result.Errors = append(result.Errors, err.Error())
		}
	} else {
		result.Metadata = metadata
	}

	// Validate required fields if metadata exists
	if metadata != nil {
		validateRequiredFields(metadata, result)
	}

	// Determine final validity based on mode
	if len(result.Errors) > 0 {
		result.Valid = false
	} else if strictMode && len(result.Warnings) > 0 {
		result.Valid = false
	}

	return result
}

// validateRequiredFields checks for missing or invalid required fields.
func validateRequiredFields(metadata *types.TemplateMetadata, result *ValidationResult) {
	// Version validation is already done in ParseMetadata
	// Additional field validations can be added here

	// Validate aspect ratio format if provided
	if metadata.AspectRatio != "" {
		if !isValidAspectRatio(metadata.AspectRatio) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("invalid aspect ratio format: %s (expected format like '16:9')", metadata.AspectRatio))
		}
	}

	// Validate layout hints reference valid layouts (this would need layout info)
	// For now, we just ensure the hints map is well-formed if present
	if metadata.LayoutHints != nil {
		for key, hint := range metadata.LayoutHints {
			if key == "" {
				result.Warnings = append(result.Warnings, "layout hint with empty key")
				continue
			}
			if hint.MaxBullets < 0 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("layout %q has invalid max_bullets: %d (must be non-negative)", key, hint.MaxBullets))
			}
			if hint.MaxChars < 0 {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("layout %q has invalid max_chars: %d (must be non-negative)", key, hint.MaxChars))
			}
		}
	}
}

// isValidAspectRatio checks if a string is a valid aspect ratio (e.g., "16:9", "4:3").
func isValidAspectRatio(ratio string) bool {
	parts := strings.Split(ratio, ":")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
