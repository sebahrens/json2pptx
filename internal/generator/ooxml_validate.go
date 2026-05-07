package generator

import (
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// validateOutputOOXML reads the generated PPTX and runs the unified output
// validation suite (structural + OOXML). Returns a list of warning strings
// for any findings.
func validateOutputOOXML(outputPath string) []string {
	data, err := os.ReadFile(outputPath) //nolint:gosec // Path from internal generation, not user input
	if err != nil {
		return []string{fmt.Sprintf("output-validate: failed to read output: %v", err)}
	}

	report, err := pptx.ValidateOutputBytes(data)
	if err != nil {
		return []string{fmt.Sprintf("output-validate: failed to open PPTX: %v", err)}
	}

	if len(report.Findings) == 0 {
		return nil
	}

	warnings := make([]string, 0, len(report.Findings))
	for _, f := range report.Findings {
		warnings = append(warnings, fmt.Sprintf("output-validate: %s", f.Error()))
	}
	return warnings
}

// ValidateOutputFile validates a PPTX file using the unified output-validation
// suite and returns the report. This is the public API used by CLI commands and tests.
func ValidateOutputFile(path string) (*pptx.Report, error) {
	return pptx.ValidateOutputFile(path)
}
