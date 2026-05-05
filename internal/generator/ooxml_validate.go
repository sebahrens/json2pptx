package generator

import (
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// validateOutputOOXML reads the generated PPTX and runs OOXML content validation.
// Returns a list of warning strings for any violations found.
func validateOutputOOXML(outputPath string) []string {
	data, err := os.ReadFile(outputPath) //nolint:gosec // Path from internal generation, not user input
	if err != nil {
		return []string{fmt.Sprintf("ooxml-validate: failed to read output: %v", err)}
	}

	v, err := pptx.NewOOXMLValidator(data)
	if err != nil {
		return []string{fmt.Sprintf("ooxml-validate: failed to open PPTX: %v", err)}
	}

	if err := v.Validate(); err != nil {
		if verrs, ok := err.(pptx.ValidationErrors); ok {
			warnings := make([]string, 0, len(verrs))
			for _, verr := range verrs {
				warnings = append(warnings, fmt.Sprintf("ooxml-validate: %s", verr.Error()))
			}
			return warnings
		}
		return []string{fmt.Sprintf("ooxml-validate: %v", err)}
	}
	return nil
}

// ValidateOutputFile validates a PPTX file for OOXML content correctness.
// This is the public API used by CLI commands and tests.
func ValidateOutputFile(path string) ([]pptx.ValidationError, error) {
	data, err := os.ReadFile(path) //nolint:gosec // CLI tool - path from command-line argument
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	v, err := pptx.NewOOXMLValidator(data)
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX: %w", err)
	}

	_ = v.Validate()
	return v.Errors(), nil
}
