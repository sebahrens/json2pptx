package pptx

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
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

// RepairScope classifies who is responsible for fixing a finding.
type RepairScope string

const (
	// RepairScopeSource means the finding maps to source JSON and can be fixed
	// via repair_slide or by editing the input deck.
	RepairScopeSource RepairScope = "source"
	// RepairScopeTemplate means the issue originates from the template, not the
	// source JSON. The agent should report it rather than attempt source repair.
	RepairScopeTemplate RepairScope = "template"
	// RepairScopeGenerator means the issue is a bug in the generation engine.
	// The agent should report it rather than attempt source repair.
	RepairScopeGenerator RepairScope = "generator"
)

// Finding is a single output-validation diagnostic with a stable machine-readable code.
type Finding struct {
	Code     string   `json:"code"`     // Stable code, e.g. "OPC_MISSING_PART", "OOXML_INVALID_COLOR"
	Severity Severity `json:"severity"` // "blocking" or "warning"
	Path     string   `json:"path"`     // Part path inside the PPTX (empty for package-level)
	Message  string   `json:"message"`  // Human-readable description

	// Provenance fields for agent repair targeting.

	// Phase identifies which validation phase produced this finding: "opc" or "ooxml".
	Phase string `json:"phase"`
	// Validator names the specific validator: "structural" or "ooxml_content".
	Validator string `json:"validator"`
	// SlideIndex is the 0-based source slide index when the finding maps
	// deterministically to a single slide. -1 when unmappable.
	SlideIndex int `json:"slide_index"`
	// SourcePath is a JSON Pointer (RFC 6901) into the source input JSON,
	// suitable for repair_slide targeting. Empty when the finding cannot be
	// mapped to source.
	SourcePath string `json:"source_path,omitempty"`
	// Scope classifies responsibility: "source", "template", or "generator".
	Scope RepairScope `json:"scope"`
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
	ErrCodeDuplicateRelID:     "OPC_DUPLICATE_REL_ID",
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

// slidePartRegex extracts the 1-based slide number from a PPTX part path
// like "ppt/slides/slide3.xml".
var slidePartRegex = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

// slideIndexFromPart extracts the 0-based slide index from a PPTX part path.
// Returns -1 if the path does not match a slide part.
func slideIndexFromPart(partPath string) int {
	m := slidePartRegex.FindStringSubmatch(partPath)
	if m == nil {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return -1
	}
	return n - 1 // convert 1-based part number to 0-based slide index
}

// sourcePathFromSlideIndex returns a JSON Pointer to the source slide,
// e.g. "/slides/2" for slide index 2.
func sourcePathFromSlideIndex(idx int) string {
	if idx < 0 {
		return ""
	}
	return fmt.Sprintf("/slides/%d", idx)
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
		slideIdx := slideIndexFromPart(ve.Path)
		scope := classifyOPCScope(slideIdx)
		report.Findings = append(report.Findings, Finding{
			Code:       code,
			Severity:   SeverityBlocking,
			Path:       ve.Path,
			Message:    ve.Message,
			Phase:      "opc",
			Validator:  "structural",
			SlideIndex: slideIdx,
			SourcePath: sourcePathFromSlideIndex(slideIdx),
			Scope:      scope,
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
		slideIdx := slideIndexFromPart(ve.Path)
		report.Findings = append(report.Findings, Finding{
			Code:       code,
			Severity:   SeverityWarning,
			Path:       ve.Path,
			Message:    ve.Message,
			Phase:      "ooxml",
			Validator:  "ooxml_content",
			SlideIndex: slideIdx,
			SourcePath: sourcePathFromSlideIndex(slideIdx),
			Scope:      RepairScopeSource, // OOXML content issues originate from generated content
		})
	}

	return report
}

// classifyOPCScope determines the repair scope for a structural (OPC) finding.
// Slide-level structural errors (e.g. malformed slide XML) map to source scope
// because they likely result from generator processing of source input.
// Package-level errors (missing Content_Types, broken rels) are classified as
// generator-scoped since they indicate engine bugs, not source problems.
func classifyOPCScope(slideIdx int) RepairScope {
	if slideIdx >= 0 {
		return RepairScopeSource
	}
	return RepairScopeGenerator
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
