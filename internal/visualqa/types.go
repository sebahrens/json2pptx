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
