package pptx

import (
	"fmt"
	"os"
)

// Severity indicates whether a finding blocks output acceptance or is advisory.
type Severity string

const (
	// SeverityBlocking means the PPTX is structurally corrupt and must not be shipped.
	SeverityBlocking Severity = "blocking"
	// SeverityWarning means the content has an issue that may cause rendering
	// problems but the package itself is structurally intact.
	SeverityWarning Severity = "warning"
)

// Finding is a single output-validation diagnostic with a stable machine-readable code.
type Finding struct {
	Code     string   `json:"code"`     // Stable code, e.g. "OPC_MISSING_PART", "OOXML_INVALID_COLOR"
	Severity Severity `json:"severity"` // "blocking" or "warning"
	Path     string   `json:"path"`     // Part path inside the PPTX (empty for package-level)
	Message  string   `json:"message"`  // Human-readable description
}

// Error implements the error interface.
func (f Finding) Error() string {
	sev := "warning"
	if f.Severity == SeverityBlocking {
		sev = "BLOCKING"
	}
	if f.Path != "" {
		return fmt.Sprintf("[%s] %s (%s): %s", f.Code, f.Path, sev, f.Message)
	}
	return fmt.Sprintf("[%s] (%s): %s", f.Code, sev, f.Message)
}

// Report is the result of running the unified output-validation suite.
type Report struct {
	Findings []Finding `json:"findings"`
}

// IsValid returns true when no blocking findings exist.
func (r *Report) IsValid() bool {
	for i := range r.Findings {
		if r.Findings[i].Severity == SeverityBlocking {
			return false
		}
	}
	return true
}

// Blocking returns only the blocking findings.
func (r *Report) Blocking() []Finding {
	var out []Finding
	for i := range r.Findings {
		if r.Findings[i].Severity == SeverityBlocking {
			out = append(out, r.Findings[i])
		}
	}
	return out
}

// Warnings returns only the warning findings.
func (r *Report) Warnings() []Finding {
	var out []Finding
	for i := range r.Findings {
		if r.Findings[i].Severity == SeverityWarning {
			out = append(out, r.Findings[i])
		}
	}
	return out
}

// opcCodeMap maps structural ValidationError codes to stable OPC_ prefixed codes.
var opcCodeMap = map[string]string{
	ErrCodeMissingPart:        "OPC_MISSING_PART",
	ErrCodeDanglingRel:        "OPC_DANGLING_REL",
	ErrCodeMissingElement:     "OPC_MISSING_ELEMENT",
	ErrCodeMalformedXML:       "OPC_MALFORMED_XML",
	ErrCodeMissingContentType: "OPC_MISSING_CONTENT_TYPE",
}

// ooxmlCodeMap maps OOXML ValidationError codes to stable OOXML_ prefixed codes.
var ooxmlCodeMap = map[string]string{
	ErrCodeInvalidColor:  "OOXML_INVALID_COLOR",
	ErrCodeInvalidScheme: "OOXML_INVALID_SCHEME",
	ErrCodeDuplicateID:   "OOXML_DUPLICATE_ID",
	ErrCodeInvalidTable:  "OOXML_INVALID_TABLE",
	ErrCodeZeroExtent:    "OOXML_ZERO_EXTENT",
}

// OutputValidator runs both structural (OPC) and content-level (OOXML) validation
// on a PPTX package, producing a unified Report.
type OutputValidator struct {
	pkg *Package
}

// NewOutputValidator creates a unified output validator from PPTX bytes.
func NewOutputValidator(data []byte) (*OutputValidator, error) {
	pkg, err := OpenFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("failed to open PPTX: %w", err)
	}
	return &OutputValidator{pkg: pkg}, nil
}

// NewOutputValidatorFromPackage creates a unified output validator from an existing Package.
func NewOutputValidatorFromPackage(pkg *Package) *OutputValidator {
	return &OutputValidator{pkg: pkg}
}

// Validate runs both structural and OOXML checks, returning a combined Report.
func (ov *OutputValidator) Validate() *Report {
	report := &Report{}

	// Phase 1: structural / OPC checks
	sv := NewValidatorFromPackage(ov.pkg)
	_ = sv.Validate()
	for _, ve := range sv.Errors() {
		code := opcCodeMap[ve.Code]
		if code == "" {
			code = "OPC_" + ve.Code // Fallback for unknown codes
		}
		report.Findings = append(report.Findings, Finding{
			Code:     code,
			Severity: SeverityBlocking,
			Path:     ve.Path,
			Message:  ve.Message,
		})
	}

	// Phase 2: OOXML content checks
	xv := NewOOXMLValidatorFromPackage(ov.pkg)
	_ = xv.Validate()
	for _, ve := range xv.Errors() {
		code := ooxmlCodeMap[ve.Code]
		if code == "" {
			code = "OOXML_" + ve.Code // Fallback for unknown codes
		}
		report.Findings = append(report.Findings, Finding{
			Code:     code,
			Severity: SeverityWarning,
			Path:     ve.Path,
			Message:  ve.Message,
		})
	}

	return report
}

// ValidateOutputBytes runs the unified output-validation suite on raw PPTX bytes.
func ValidateOutputBytes(data []byte) (*Report, error) {
	ov, err := NewOutputValidator(data)
	if err != nil {
		return nil, err
	}
	return ov.Validate(), nil
}

// ValidateOutputFile runs the unified output-validation suite on a PPTX file.
func ValidateOutputFile(path string) (*Report, error) {
	data, err := os.ReadFile(path) //nolint:gosec // CLI tool - path from command-line argument
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ValidateOutputBytes(data)
}
