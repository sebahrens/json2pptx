// Package pptx provides PPTX file manipulation primitives.
package pptx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// Validator provides structural validation for PPTX files.
// It checks required parts, relationship targets, and XML structure.
//
// Use Validator in tests to verify that generated PPTX files are well-formed
// and contain all required components.
//
// Example:
//
//	v := NewValidator(pptxBytes)
//	if err := v.Validate(); err != nil {
//	    t.Fatalf("PPTX validation failed: %v", err)
//	}
type Validator struct {
	pkg    *Package
	errors []ValidationError
}

// ValidationError represents a single validation failure.
type ValidationError struct {
	Path    string // Part path or empty for package-level errors
	Code    string // Error code (e.g., "MISSING_PART", "DANGLING_REL")
	Message string // Human-readable error description
}

// Error implements the error interface for ValidationError.
func (e ValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ValidationErrors collects multiple validation errors.
type ValidationErrors []ValidationError

// Error implements the error interface for ValidationErrors.
func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return "no validation errors"
	}
	if len(errs) == 1 {
		return errs[0].Error()
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d validation errors:\n", len(errs)))
	for i, err := range errs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("  - ")
		b.WriteString(err.Error())
	}
	return b.String()
}

// Common validation error codes.
const (
	ErrCodeMissingPart        = "MISSING_PART"
	ErrCodeDanglingRel        = "DANGLING_REL"
	ErrCodeMissingElement     = "MISSING_ELEMENT"
	ErrCodeMalformedXML       = "MALFORMED_XML"
	ErrCodeMissingContentType = "MISSING_CONTENT_TYPE"
	ErrCodeInvalidStructure   = "INVALID_STRUCTURE"
)

// NewValidator creates a validator from PPTX bytes.
func NewValidator(data []byte) (*Validator, error) {
	pkg, err := OpenFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX: %w", err)
	}
	return &Validator{pkg: pkg}, nil
}

// NewValidatorFromPackage creates a validator from an existing Package.
func NewValidatorFromPackage(pkg *Package) *Validator {
	return &Validator{pkg: pkg}
}

// Package returns the underlying Package for additional inspection.
func (v *Validator) Package() *Package {
	return v.pkg
}

// Errors returns all accumulated validation errors.
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// HasErrors returns true if any validation errors were found.
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// addError appends a validation error.
func (v *Validator) addError(path, code, message string) {
	v.errors = append(v.errors, ValidationError{
		Path:    path,
		Code:    code,
		Message: message,
	})
}

// Validate runs all validation checks and returns any errors found.
func (v *Validator) Validate() error {
	v.errors = nil // Reset errors

	// Required parts
	v.ValidateContentTypes()
	v.ValidatePackageRels()
	v.ValidatePresentation()

	// Validate all relationship targets
	v.ValidateAllRelationshipTargets()

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// ValidateContentTypes checks that [Content_Types].xml exists and is valid.
func (v *Validator) ValidateContentTypes() {
	if !v.pkg.HasEntry(ContentTypesPath) {
		v.addError(ContentTypesPath, ErrCodeMissingPart, "required [Content_Types].xml is missing")
		return
	}

	data, err := v.pkg.ReadEntry(ContentTypesPath)
	if err != nil {
		v.addError(ContentTypesPath, ErrCodeMalformedXML, fmt.Sprintf("failed to read: %v", err))
		return
	}

	_, err = ParseContentTypes(data)
	if err != nil {
		v.addError(ContentTypesPath, ErrCodeMalformedXML, fmt.Sprintf("failed to parse: %v", err))
	}
}

// ValidatePackageRels checks that _rels/.rels exists and is valid.
func (v *Validator) ValidatePackageRels() {
	relsPath := PackageRels()
	if !v.pkg.HasEntry(relsPath) {
		v.addError(relsPath, ErrCodeMissingPart, "required package relationships file is missing")
		return
	}

	data, err := v.pkg.ReadEntry(relsPath)
	if err != nil {
		v.addError(relsPath, ErrCodeMalformedXML, fmt.Sprintf("failed to read: %v", err))
		return
	}

	_, err = ParseRelationships(data)
	if err != nil {
		v.addError(relsPath, ErrCodeMalformedXML, fmt.Sprintf("failed to parse: %v", err))
	}
}

// ValidatePresentation checks that ppt/presentation.xml exists and is valid.
func (v *Validator) ValidatePresentation() {
	presPath := "ppt/presentation.xml"
	if !v.pkg.HasEntry(presPath) {
		v.addError(presPath, ErrCodeMissingPart, "required presentation.xml is missing")
		return
	}

	data, err := v.pkg.ReadEntry(presPath)
	if err != nil {
		v.addError(presPath, ErrCodeMalformedXML, fmt.Sprintf("failed to read: %v", err))
		return
	}

	// Check for required elements
	v.RequireXMLElement(presPath, data, "p:presentation", "sldIdLst")
}

// ValidateAllRelationshipTargets checks that all relationship targets exist.
func (v *Validator) ValidateAllRelationshipTargets() {
	// Find all .rels files
	for _, entry := range v.pkg.Entries() {
		if strings.HasSuffix(entry, ".rels") {
			v.ValidateRelationshipTargets(entry)
		}
	}
}

// ValidateRelationshipTargets checks that all targets in a .rels file exist.
func (v *Validator) ValidateRelationshipTargets(relsPath string) {
	data, err := v.pkg.ReadEntry(relsPath)
	if err != nil {
		return // Already reported by other validation
	}

	rels, err := ParseRelationships(data)
	if err != nil {
		return // Already reported by other validation
	}

	// Determine base path for resolving relative targets
	basePath := getBasePathForRels(relsPath)

	for _, rel := range rels.All() {
		// Skip external relationships
		if rel.TargetMode == "External" {
			continue
		}

		// Resolve target path
		targetPath := resolveRelativeTarget(basePath, rel.Target)

		// Check if target exists
		if !v.pkg.HasEntry(targetPath) {
			v.addError(relsPath, ErrCodeDanglingRel,
				fmt.Sprintf("relationship %s targets non-existent part: %s", rel.ID, targetPath))
		}
	}
}

// getBasePathForRels determines the base path for resolving relative targets.
// For "ppt/slides/_rels/slide1.xml.rels", the base path is "ppt/slides/".
func getBasePathForRels(relsPath string) string {
	// Remove /_rels/filename.rels to get base directory
	dir := path.Dir(relsPath) // e.g., "ppt/slides/_rels"
	dir = path.Dir(dir)       // e.g., "ppt/slides"
	if dir == "." || dir == "" {
		return ""
	}
	return dir + "/"
}

// resolveRelativeTarget resolves a relative target path.
// Handles ".." path components.
func resolveRelativeTarget(basePath, target string) string {
	if strings.HasPrefix(target, "/") {
		// Absolute path (remove leading /)
		return strings.TrimPrefix(target, "/")
	}

	// Build path by processing each component
	parts := strings.Split(basePath+target, "/")
	var resolved []string

	for _, part := range parts {
		if part == ".." && len(resolved) > 0 {
			resolved = resolved[:len(resolved)-1]
		} else if part != "" && part != "." {
			resolved = append(resolved, part)
		}
	}

	return strings.Join(resolved, "/")
}

// RequirePart checks that a specific part exists.
func (v *Validator) RequirePart(partPath string) bool {
	if !v.pkg.HasEntry(partPath) {
		v.addError(partPath, ErrCodeMissingPart, "required part is missing")
		return false
	}
	return true
}

// RequireXMLElement checks that an XML document contains specific elements.
// Uses simple substring matching for efficiency.
func (v *Validator) RequireXMLElement(partPath string, data []byte, elements ...string) {
	for _, elem := range elements {
		// Check for element as tag opening
		pattern := "<" + elem
		altPattern := ":" + elem // Handle prefixed elements like p:sldIdLst

		if !bytes.Contains(data, []byte(pattern)) && !bytes.Contains(data, []byte(altPattern)) {
			v.addError(partPath, ErrCodeMissingElement,
				fmt.Sprintf("required element <%s> not found", elem))
		}
	}
}

// HasPart returns true if a part exists in the package.
func (v *Validator) HasPart(partPath string) bool {
	return v.pkg.HasEntry(partPath)
}

// Part reads a part from the package.
func (v *Validator) Part(partPath string) ([]byte, error) {
	return v.pkg.ReadEntry(partPath)
}

// CountSlides returns the number of slide parts in the package.
func (v *Validator) CountSlides() int {
	count := 0
	for _, entry := range v.pkg.Entries() {
		if strings.HasPrefix(entry, "ppt/slides/slide") &&
			strings.HasSuffix(entry, ".xml") &&
			!strings.Contains(entry, "_rels") {
			count++
		}
	}
	return count
}

// CountMedia returns the number of media files in the package.
func (v *Validator) CountMedia() int {
	count := 0
	for _, entry := range v.pkg.Entries() {
		if strings.HasPrefix(entry, "ppt/media/") {
			count++
		}
	}
	return count
}

// CountSVG returns the number of SVG files in the package.
func (v *Validator) CountSVG() int {
	count := 0
	for _, entry := range v.pkg.Entries() {
		if strings.HasSuffix(entry, ".svg") {
			count++
		}
	}
	return count
}

// CountPNG returns the number of PNG files in the package.
func (v *Validator) CountPNG() int {
	count := 0
	for _, entry := range v.pkg.Entries() {
		if strings.HasSuffix(entry, ".png") {
			count++
		}
	}
	return count
}

// MediaStats returns counts of media files by type.
type MediaStats struct {
	Total     int
	SVG       int
	PNG       int
	Other     int
	SVGFiles  []string
	PNGFiles  []string
}

// MediaStats returns detailed media file statistics.
func (v *Validator) MediaStats() MediaStats {
	stats := MediaStats{}
	for _, entry := range v.pkg.Entries() {
		if !strings.HasPrefix(entry, "ppt/media/") {
			continue
		}
		stats.Total++
		switch {
		case strings.HasSuffix(entry, ".svg"):
			stats.SVG++
			stats.SVGFiles = append(stats.SVGFiles, entry)
		case strings.HasSuffix(entry, ".png"):
			stats.PNG++
			stats.PNGFiles = append(stats.PNGFiles, entry)
		default:
			stats.Other++
		}
	}
	return stats
}

// HasMediaFile checks if a specific media file exists.
func (v *Validator) HasMediaFile(filename string) bool {
	return v.pkg.HasEntry("ppt/media/" + filename)
}

// ValidateSVGInsertion performs checks specific to SVG insertion.
// It verifies:
// - SVG and PNG media files exist
// - Content types are registered
// - Slide contains p:pic element with svgBlip extension
func (v *Validator) ValidateSVGInsertion(slideIndex int) error {
	v.errors = nil

	// Check for SVG and PNG in media folder
	hasSVG := false
	hasPNG := false
	for _, entry := range v.pkg.Entries() {
		if strings.HasPrefix(entry, "ppt/media/") {
			if strings.HasSuffix(entry, ".svg") {
				hasSVG = true
			}
			if strings.HasSuffix(entry, ".png") {
				hasPNG = true
			}
		}
	}

	if !hasSVG {
		v.addError("ppt/media/", ErrCodeMissingPart, "no SVG media file found")
	}
	if !hasPNG {
		v.addError("ppt/media/", ErrCodeMissingPart, "no PNG fallback media file found")
	}

	// Check content types include SVG and PNG
	ctData, err := v.pkg.ReadEntry(ContentTypesPath)
	if err == nil {
		ct, err := ParseContentTypes(ctData)
		if err == nil {
			if !ct.HasDefault("svg") {
				v.addError(ContentTypesPath, ErrCodeMissingContentType,
					"missing Default for svg extension")
			}
			if !ct.HasDefault("png") {
				v.addError(ContentTypesPath, ErrCodeMissingContentType,
					"missing Default for png extension")
			}
		}
	}

	// Check slide contains p:pic
	slidePath := fmt.Sprintf("ppt/slides/slide%d.xml", slideIndex+1)
	if v.pkg.HasEntry(slidePath) {
		slideData, err := v.pkg.ReadEntry(slidePath)
		if err == nil {
			if !bytes.Contains(slideData, []byte("<p:pic")) {
				v.addError(slidePath, ErrCodeMissingElement, "slide does not contain p:pic element")
			}
			// Check for svgBlip extension
			if !bytes.Contains(slideData, []byte("svgBlip")) {
				v.addError(slidePath, ErrCodeMissingElement,
					"slide p:pic does not contain svgBlip extension")
			}
		}
	} else {
		v.addError(slidePath, ErrCodeMissingPart, "slide not found")
	}

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// ValidateSlide performs validation checks on a specific slide.
func (v *Validator) ValidateSlide(slideIndex int) error {
	v.errors = nil

	slidePath := fmt.Sprintf("ppt/slides/slide%d.xml", slideIndex+1)

	if !v.RequirePart(slidePath) {
		return v.Errors()
	}

	data, err := v.pkg.ReadEntry(slidePath)
	if err != nil {
		v.addError(slidePath, ErrCodeMalformedXML, fmt.Sprintf("failed to read: %v", err))
		return v.Errors()
	}

	// Check required slide elements
	v.RequireXMLElement(slidePath, data, "p:sld", "p:cSld", "p:spTree")

	// Validate slide relationships
	slideRelsPath := GetRelsPath(slidePath)
	if v.pkg.HasEntry(slideRelsPath) {
		v.ValidateRelationshipTargets(slideRelsPath)
	}

	if v.HasErrors() {
		return v.Errors()
	}
	return nil
}

// AssertSlideContains checks that a slide contains specific XML content.
func (v *Validator) AssertSlideContains(slideIndex int, patterns ...string) error {
	slidePath := fmt.Sprintf("ppt/slides/slide%d.xml", slideIndex+1)

	data, err := v.pkg.ReadEntry(slidePath)
	if err != nil {
		return fmt.Errorf("failed to read slide: %w", err)
	}

	var missing []string
	for _, pattern := range patterns {
		if !bytes.Contains(data, []byte(pattern)) {
			missing = append(missing, pattern)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("slide %d missing patterns: %v", slideIndex, missing)
	}
	return nil
}

// AssertRelationshipExists checks that a relationship with the given type exists.
func (v *Validator) AssertRelationshipExists(relsPath, relType string) error {
	data, err := v.pkg.ReadEntry(relsPath)
	if err != nil {
		return fmt.Errorf("failed to read relationships: %w", err)
	}

	rels, err := ParseRelationships(data)
	if err != nil {
		return fmt.Errorf("failed to parse relationships: %w", err)
	}

	found := rels.FindByType(relType)
	if len(found) == 0 {
		return fmt.Errorf("no relationship of type %s found in %s", relType, relsPath)
	}
	return nil
}

// SlideXML reads and returns the raw XML for a slide.
func (v *Validator) SlideXML(slideIndex int) ([]byte, error) {
	slidePath := fmt.Sprintf("ppt/slides/slide%d.xml", slideIndex+1)
	return v.pkg.ReadEntry(slidePath)
}

// ContentTypesXMLData returns the raw [Content_Types].xml content.
func (v *Validator) ContentTypesXMLData() ([]byte, error) {
	return v.pkg.ReadEntry(ContentTypesPath)
}

// RelationshipsXMLData returns the raw relationships XML for a part.
func (v *Validator) RelationshipsXMLData(partPath string) ([]byte, error) {
	relsPath := GetRelsPath(partPath)
	return v.pkg.ReadEntry(relsPath)
}

// DumpStructure returns a summary of the package structure for debugging.
func (v *Validator) DumpStructure() string {
	var b strings.Builder
	b.WriteString("PPTX Structure:\n")

	entries := v.pkg.Entries()
	b.WriteString(fmt.Sprintf("  Total entries: %d\n", len(entries)))
	b.WriteString(fmt.Sprintf("  Slides: %d\n", v.CountSlides()))
	b.WriteString(fmt.Sprintf("  Media files: %d\n", v.CountMedia()))

	b.WriteString("\n  Parts:\n")
	for _, entry := range entries {
		b.WriteString(fmt.Sprintf("    - %s\n", entry))
	}

	return b.String()
}

// UnmarshalSlide parses a slide's XML into a generic map for inspection.
// This is useful for asserting specific attribute values.
func (v *Validator) UnmarshalSlide(slideIndex int) (map[string]interface{}, error) {
	data, err := v.SlideXML(slideIndex)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := xml.Unmarshal(data, &result); err != nil {
		// Fall back to string representation if generic unmarshal fails
		return map[string]interface{}{"_raw": string(data)}, nil
	}
	return result, nil
}
