// Package visualqa provides an AI-powered visual quality assurance agent
// that inspects rendered slide images using Claude Haiku's vision capabilities.
// It applies per-slide-type prompts to detect visual defects such as text
// overflow, misalignment, contrast issues, and layout problems.
package visualqa

import "fmt"

// Severity indicates the impact level of a visual defect.
type Severity string

const (
	SeverityP0 Severity = "P0" // Catastrophic: unreadable, broken layout
	SeverityP1 Severity = "P1" // Major: significant visual issue
	SeverityP2 Severity = "P2" // Minor: cosmetic issue
	SeverityP3 Severity = "P3" // Nitpick: suggestion for improvement
)

// validSeverities is the set of allowed severity values.
var validSeverities = map[Severity]bool{
	SeverityP0: true,
	SeverityP1: true,
	SeverityP2: true,
	SeverityP3: true,
}

// ValidSeverity reports whether s is an allowed severity value.
func ValidSeverity(s Severity) bool {
	return validSeverities[s]
}

// allowedCategories is the set of allowed finding category strings.
var allowedCategories = map[string]bool{
	"text_overflow":      true,
	"text_truncation":    true,
	"contrast":           true,
	"alignment":          true,
	"spacing":            true,
	"overlap":            true,
	"missing_content":    true,
	"font_size":          true,
	"visual_hierarchy":   true,
	"chart_readability":  true,
	"table_readability":  true,
	"image_quality":      true,
	"layout_balance":     true,
	"color_consistency":  true,
	"border_style":       true,
	"footer_clearance":   true,
	"aspect_ratio":       true,
}

// ValidCategory reports whether cat is an allowed finding category.
func ValidCategory(cat string) bool {
	return allowedCategories[cat]
}

// SchemaError indicates the model returned structurally valid JSON but with
// values outside the allowed schema (unknown severity or category). Callers
// can type-assert this to distinguish schema violations from transport or
// JSON parse errors.
type SchemaError struct {
	Violations []string // human-readable descriptions of each violation
}

func (e *SchemaError) Error() string {
	return fmt.Sprintf("schema validation failed: %v", e.Violations)
}

// Finding represents a single visual defect detected by the QA agent.
type Finding struct {
	SlideIndex  int      `json:"slide_index"`
	SlideType   string   `json:"slide_type"`
	Severity    Severity `json:"severity"`
	Category    string   `json:"category"`    // e.g. "text_overflow", "contrast", "alignment"
	Description string   `json:"description"` // Human-readable description
	Location    string   `json:"location"`    // Where on the slide (e.g. "bottom-left", "title area")
}

// String returns a human-readable representation of the finding.
func (f Finding) String() string {
	return fmt.Sprintf("[%s] Slide %d (%s) — %s: %s [%s]",
		f.Severity, f.SlideIndex, f.SlideType, f.Category, f.Description, f.Location)
}

// SlideResult holds the QA result for a single slide.
type SlideResult struct {
	SlideIndex int       `json:"slide_index"`
	SlideType  string    `json:"slide_type"`
	Findings   []Finding `json:"findings"`
	RawOutput  string    `json:"raw_output"` // Full model response for debugging
	Error      string    `json:"error,omitempty"`
}

// Report holds the complete QA results for a presentation.
type Report struct {
	Template    string        `json:"template"`
	SlideCount  int           `json:"slide_count"`
	Results     []SlideResult `json:"results"`
	TotalByP0   int           `json:"total_p0"`
	TotalByP1   int           `json:"total_p1"`
	TotalByP2   int           `json:"total_p2"`
	TotalByP3   int           `json:"total_p3"`
	TotalIssues int           `json:"total_issues"`
}

// Summarize computes aggregate counts from results.
func (r *Report) Summarize() {
	r.TotalByP0 = 0
	r.TotalByP1 = 0
	r.TotalByP2 = 0
	r.TotalByP3 = 0
	r.TotalIssues = 0
	for _, sr := range r.Results {
		for _, f := range sr.Findings {
			r.TotalIssues++
			switch f.Severity {
			case SeverityP0:
				r.TotalByP0++
			case SeverityP1:
				r.TotalByP1++
			case SeverityP2:
				r.TotalByP2++
			case SeverityP3:
				r.TotalByP3++
			}
		}
	}
}

// SlideInfo provides context about a slide for the QA agent.
type SlideInfo struct {
	Index int    // Zero-based slide index
	Type  string // Slide type (title, content, chart, etc.)
	Title string // Slide title if available
}
