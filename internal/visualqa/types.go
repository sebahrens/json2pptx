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

// VocabularyEntry is one allowed finding value with its one-line meaning.
type VocabularyEntry struct {
	Name    string
	Meaning string
}

// severityVocabulary lists the allowed severities, most severe first.
var severityVocabulary = []VocabularyEntry{
	{string(SeverityP0), "unreadable or broken"},
	{string(SeverityP1), "major defect"},
	{string(SeverityP2), "minor, cosmetic"},
	{string(SeverityP3), "nitpick"},
}

// categoryVocabulary lists the allowed finding categories. It is the single
// source for the vision prompt's parser, submit_visual_review's input schema
// and its rejection message (go-slide-creator-lk37o).
var categoryVocabulary = []VocabularyEntry{
	{"text_overflow", "text spills its box"},
	{"text_truncation", "text cut off"},
	{"contrast", "low text contrast"},
	{"alignment", "edges or baselines off"},
	{"spacing", "uneven gaps or margins"},
	{"overlap", "elements collide"},
	{"missing_content", "content absent"},
	{"font_size", "type too small"},
	{"visual_hierarchy", "emphasis unclear"},
	{"chart_readability", "chart hard to read"},
	{"table_readability", "table hard to read"},
	{"image_quality", "picture blurred or badly cropped"},
	{"layout_balance", "lopsided or empty areas"},
	{"color_consistency", "off-palette colour"},
	{"border_style", "stray or uneven outlines"},
	{"footer_clearance", "content crowds the footer"},
	{"aspect_ratio", "distorted proportions"},
}

// SeverityVocabulary returns the allowed severities with their meanings.
func SeverityVocabulary() []VocabularyEntry {
	return append([]VocabularyEntry(nil), severityVocabulary...)
}

// CategoryVocabulary returns the allowed categories with their meanings.
func CategoryVocabulary() []VocabularyEntry {
	return append([]VocabularyEntry(nil), categoryVocabulary...)
}

func inVocabulary(list []VocabularyEntry, name string) bool {
	for _, e := range list {
		if e.Name == name {
			return true
		}
	}
	return false
}

// ValidSeverity reports whether s is an allowed severity value.
func ValidSeverity(s Severity) bool { return inVocabulary(severityVocabulary, string(s)) }

// ValidCategory reports whether cat is an allowed finding category.
func ValidCategory(cat string) bool { return inVocabulary(categoryVocabulary, cat) }

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

// SuggestedFix pairs a repair_slide fix kind with optional parameters.
type SuggestedFix struct {
	Kind   string         `json:"kind"`
	Params map[string]any `json:"params,omitempty"`
}

// Finding represents a single visual defect detected by the QA agent.
//
// Source distinguishes vision-backed findings (Claude vision API) from
// heuristic fallback findings produced when ANTHROPIC_API_KEY is unset, and
// conservative deterministic pixel-geometry findings merged into vision mode.
// Heuristic findings are advisory only (typically SeverityP3); deterministic
// findings are conservative geometry checks (typically SeverityP2).
type Finding struct {
	SlideIndex     int            `json:"slide_index"`
	SlideType      string         `json:"slide_type"`
	Severity       Severity       `json:"severity"`
	Category       string         `json:"category"`                  // e.g. "text_overflow", "contrast", "alignment"
	Description    string         `json:"description"`               // Human-readable description
	Location       string         `json:"location"`                  // Where on the slide (e.g. "bottom-left", "title area")
	Source         string         `json:"source,omitempty"`          // "vision" (default), "deterministic", or "heuristic"
	SuggestedFixes []SuggestedFix `json:"suggested_fixes,omitempty"` // Mapped repair_slide fix kinds
	// BBox is the optional defect region in normalized slide coordinates.
	// propose_repairs hit-tests it against generated element bounds to target
	// a specific shape/cell path instead of the whole slide.
	BBox *BBox `json:"bbox,omitempty"`
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
//
// Mode indicates which inspection backend produced the results:
//   - "vision"     — Claude vision API (requires ANTHROPIC_API_KEY)
//   - "heuristic"  — pure-Go fallback checks (no API key required); findings
//     are advisory only and may have higher false-positive rates.
type Report struct {
	Template    string        `json:"template"`
	Mode        string        `json:"mode,omitempty"` // "vision" or "heuristic"
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
