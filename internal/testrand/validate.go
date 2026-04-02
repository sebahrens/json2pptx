package testrand

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"path"
	"strings"
)

// ValidationResult holds PPTX structural validation results.
type ValidationResult struct {
	Valid      bool     `json:"valid"`
	SlideCount int      `json:"slide_count"`
	Errors     []string `json:"errors,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// ValidatePPTX performs structural validation on a generated PPTX file.
// It checks: valid ZIP, contains ppt/presentation.xml, slide count, no corruption.
func ValidatePPTX(pptxPath string, expectedSlides int) (*ValidationResult, error) {
	r, err := zip.OpenReader(pptxPath)
	if err != nil {
		return &ValidationResult{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid ZIP: %v", err)},
		}, nil
	}
	defer r.Close()

	result := &ValidationResult{Valid: true}

	// Check for required PPTX files
	requiredFiles := map[string]bool{
		"ppt/presentation.xml": false,
		"[Content_Types].xml":  false,
	}

	slideFiles := 0
	for _, f := range r.File {
		name := f.Name
		if _, ok := requiredFiles[name]; ok {
			requiredFiles[name] = true
		}
		// Count slides: ppt/slides/slide1.xml, slide2.xml, etc.
		if strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") {
			base := path.Base(name)
			if strings.HasPrefix(base, "slide") && !strings.Contains(base, "Layout") {
				slideFiles++
			}
		}
	}

	// Verify required files exist
	for name, found := range requiredFiles {
		if !found {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("missing required file: %s", name))
		}
	}

	result.SlideCount = slideFiles

	// Validate slide count matches expectation (if provided)
	if expectedSlides > 0 && slideFiles != expectedSlides {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("expected %d slides, found %d", expectedSlides, slideFiles))
	}

	// Try to parse presentation.xml to ensure it's valid XML
	for _, f := range r.File {
		if f.Name == "ppt/presentation.xml" {
			rc, openErr := f.Open()
			if openErr != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("cannot open presentation.xml: %v", openErr))
				break
			}
			decoder := xml.NewDecoder(rc)
			for {
				_, tokenErr := decoder.Token()
				if tokenErr != nil {
					if tokenErr.Error() == "EOF" {
						break
					}
					result.Valid = false
					result.Errors = append(result.Errors, fmt.Sprintf("invalid XML in presentation.xml: %v", tokenErr))
					break
				}
			}
			rc.Close()
			break
		}
	}

	return result, nil
}
