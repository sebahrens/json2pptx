// Package types provides shared data structures for the slide generator.
package types

import "fmt"

// PresentationDefinition represents a parsed markdown presentation.
type PresentationDefinition struct {
	Metadata Metadata          // From frontmatter
	Slides   []SlideDefinition // Parsed slides
	Errors   []ParseError      // Non-fatal warnings
}

// Metadata holds presentation metadata from YAML frontmatter.
type Metadata struct {
	Title        string         `yaml:"title"`        // Required: Presentation title
	Subtitle     string         `yaml:"subtitle"`     // Optional: Subtitle for the title slide
	Author       string         `yaml:"author"`       // Optional: Author name
	Template     string         `yaml:"template"`     // Required: Template file path or name
	Theme        string         `yaml:"theme"`        // Optional: Theme name
	Date         string         `yaml:"date"`         // Optional: Date string
	Autopaginate *bool          `yaml:"autopaginate"` // Optional: Auto-split overflowing slides into continuation slides (default: true)
	Data         map[string]any `yaml:"data"`         // Optional: Variable data for {{ interpolation }}
	AutoAgenda   bool           `yaml:"auto_agenda"`  // Optional: Auto-generate agenda slide from section titles

	// BrandColor generates a complete theme from a single hex color using color theory.
	// When set, it produces a ThemeOverride with all 12 color slots (accent1-6, dk1/dk2, lt1/lt2, hlink, folHlink).
	// If theme_override is also specified, its values take precedence over generated values.
	// Example YAML:
	//   brand_color: "#E31837"
	BrandColor string `yaml:"brand_color"`

	// ThemeOverride allows per-deck color and font overrides in frontmatter.
	// Color keys are standard OOXML theme color names: accent1-accent6, dk1, dk2, lt1, lt2, hlink, folHlink.
	// Font keys: title_font, body_font.
	// Example YAML:
	//   theme_override:
	//     accent1: "#336699"
	//     accent2: "#996633"
	//     title_font: "Helvetica"
	ThemeOverride *ThemeOverride `yaml:"theme_override"`

	// Transition sets the default slide transition for all slides in the deck.
	// Supported values: fade, push, wipe, cover, uncover, cut, dissolve, none.
	// Per-slide @transition("...") directives override this default.
	// Example YAML:
	//   transition: fade
	Transition string `yaml:"transition"`

	// TransitionSpeed sets the default transition speed: "slow", "med" (default), or "fast".
	// Example YAML:
	//   transition_speed: fast
	TransitionSpeed string `yaml:"transition_speed"`
}

// ThemeOverride specifies per-deck color and font overrides.
// Colors is a map of theme color name → hex value (e.g., "accent1" → "#336699").
// Only specified colors are overridden; unspecified ones keep their template values.
type ThemeOverride struct {
	Colors    map[string]string `yaml:"colors"`     // Color overrides: name → hex (e.g., accent1: "#336699")
	TitleFont string            `yaml:"title_font"` // Override title font family
	BodyFont  string            `yaml:"body_font"`  // Override body font family
}

// AutopaginateEnabled returns whether autopagination is enabled.
// Autopaginate defaults to true when not explicitly set in frontmatter.
// Users can disable it by setting autopaginate: false.
func (m Metadata) AutopaginateEnabled() bool {
	if m.Autopaginate == nil {
		return true // default: enabled
	}
	return *m.Autopaginate
}

// SlideDefinition represents a single slide parsed from markdown.
type SlideDefinition struct {
	Index        int                  // Zero-based slide index
	SourceLine   int                  // 1-based line number where this slide starts in the source document
	Title        string               // Slide title
	Type         SlideType            // Slide type
	Content      SlideContent         // Structured content
	RawContent   string               // Original markdown for debugging
	Slots        map[int]*SlotContent // Parsed slot content (nil if no ::slotN:: markers)
	SpeakerNotes    string               // Speaker notes text (from <!-- notes: ... --> or @notes block)
	Source          string               // Source attribution text (from @source("...") or <!-- source: "..." -->)
	Transition      string               // Per-slide transition override (from @transition("fade") or frontmatter default)
	TransitionSpeed string               // Per-slide transition speed override ("slow", "med", "fast")
	Build           string               // Build animation type for bullets: "bullets" for one-by-one reveal
}

// SlotContentType identifies the type of content in a slot.
type SlotContentType string

const (
	SlotContentText            SlotContentType = "text"
	SlotContentBullets         SlotContentType = "bullets"
	SlotContentBodyAndBullets  SlotContentType = "body_and_bullets"
	SlotContentBulletGroups    SlotContentType = "bullet_groups"
	SlotContentTable           SlotContentType = "table"
	SlotContentChart           SlotContentType = "chart"
	SlotContentInfographic     SlotContentType = "infographic"
	SlotContentImage           SlotContentType = "image"
)

// SlotContent represents content for a single slot in a multi-placeholder layout.
type SlotContent struct {
	SlotNumber       int             // The slot number (1-indexed)
	RawContent       string          // Raw markdown content for this slot
	Type             SlotContentType // Detected content type
	Text             string          // Plain text content (for SlotContentText)
	Bullets          []string        // Bullet list items (for SlotContentBullets / SlotContentBodyAndBullets)
	Body             string          // Leading body text before bullets (for SlotContentBodyAndBullets)
	BodyAfterBullets string          // Trailing body text after bullets (for SlotContentBodyAndBullets)
	BulletGroups     []BulletGroup   // Grouped bullets with section headers (for SlotContentBulletGroups)
	Table            *TableSpec      // Pre-parsed table (for SlotContentTable)
	BodyAfterFence   string          // Trailing text after code fence (for SlotContentChart / SlotContentInfographic)
	DiagramSpec      *DiagramSpec    // Pre-parsed diagram (for SlotContentChart / SlotContentInfographic)
	ImagePath        string          // Extracted image path (for SlotContentImage)
}

// HasSlots returns true if the slide has parsed slot content.
func (s *SlideDefinition) HasSlots() bool {
	return len(s.Slots) > 0
}

// SlideType represents the type of slide.
type SlideType string

const (
	SlideTypeTitle      SlideType = "title"      // Opening slide
	SlideTypeContent    SlideType = "content"    // Standard bullet slide
	SlideTypeTwoColumn  SlideType = "two-column" // Side-by-side content
	SlideTypeImage      SlideType = "image"      // Image-focused slide
	SlideTypeChart      SlideType = "chart"      // Data visualization
	SlideTypeComparison SlideType = "comparison" // Comparison layout
	SlideTypeBlank      SlideType = "blank"      // Blank slide
	SlideTypeSection    SlideType = "section"    // Section divider
	SlideTypeDiagram    SlideType = "diagram"    // Diagram slide (pyramid, venn, etc.)
)

// SlideTypeInfo provides information about a supported slide type.
type SlideTypeInfo struct {
	Type        SlideType `json:"type"`
	Description string    `json:"description"`
}

// SupportedSlideTypes returns all supported slide types with descriptions.
func SupportedSlideTypes() []SlideTypeInfo {
	return []SlideTypeInfo{
		{Type: SlideTypeTitle, Description: "Opening slide with title and optional subtitle"},
		{Type: SlideTypeContent, Description: "Standard bullet slide with title and body"},
		{Type: SlideTypeTwoColumn, Description: "Side-by-side content layout"},
		{Type: SlideTypeImage, Description: "Image-focused slide with title"},
		{Type: SlideTypeChart, Description: "Data visualization slide"},
		{Type: SlideTypeComparison, Description: "Comparison layout for side-by-side elements"},
		{Type: SlideTypeBlank, Description: "Empty slide with no placeholders"},
		{Type: SlideTypeSection, Description: "Section divider slide for separating presentation sections"},
		{Type: SlideTypeDiagram, Description: "Diagram slide (pyramid, venn, org chart, etc.)"},
	}
}

// IsValidSlideType checks if the given string is a valid slide type.
func IsValidSlideType(s string) bool {
	switch s {
	case string(SlideTypeTitle),
		string(SlideTypeContent),
		string(SlideTypeTwoColumn),
		string(SlideTypeImage),
		string(SlideTypeChart),
		string(SlideTypeComparison),
		string(SlideTypeBlank),
		string(SlideTypeSection),
		string(SlideTypeDiagram):
		return true
	default:
		return false
	}
}

// SlideContent holds the structured content of a slide.
type SlideContent struct {
	Body             string        // Plain text body (before bullets)
	BodyAfterBullets string        // Body text that appears after bullets (trailing paragraphs)
	Bullets          []string      // Bullet points (flat list)
	BulletGroups     []BulletGroup // Grouped bullets with section headers
	Left             []string      // Two-column left content
	Right            []string      // Two-column right content
	ImagePath        string        // Image reference path or URL
	DiagramSpec      *DiagramSpec  // Unified diagram specification (charts, infographics, etc.)
	TableRaw         string        // Raw markdown table text (for standalone tables outside of slots)
	Table            *TableSpec    // Pre-parsed table (parsed from TableRaw)
}

// BulletGroup represents a section with an optional header and bullet items.
// This is used for hierarchical bullet structures where bold text lines
// serve as section headers followed by indented bullets.
// Example markdown:
//
//	**Phase 1 - Foundation** (Q1)
//	- Core platform stabilization
//	- Performance optimization
type BulletGroup struct {
	Header  string   // Section header (optional, empty for bullets without a header)
	Body    string   // Optional body text after the header but before bullets
	Bullets []string // Bullets under this header
}

// HasGraphic returns true if the slide content includes any graphical elements
// (diagrams or images) that should be visually inspected.
func (c *SlideContent) HasGraphic() bool {
	return c.DiagramSpec != nil || c.ImagePath != ""
}

// DiagramSpec defines a diagram to be rendered via svggen.
// This is the unified type for all visual diagrams (charts, infographics, etc.).
// The Data map is passed directly to svggen, supporting all registered diagram types.
// svggen validates the type and data at render time.
type DiagramSpec struct {
	Type    string         `json:"type" yaml:"type"`                             // Diagram type (bar_chart, timeline, process_flow, etc.)
	Title    string         `json:"title,omitempty" yaml:"title,omitempty"`       // Optional title
	Subtitle string         `json:"subtitle,omitempty" yaml:"subtitle,omitempty"` // Optional subtitle / post-fence caption
	Data     map[string]any `json:"data" yaml:"data"`                             // Diagram-specific data payload
	Width   int            `json:"width,omitempty" yaml:"width,omitempty"`       // Width in pixels (default: 800)
	Height  int            `json:"height,omitempty" yaml:"height,omitempty"`     // Height in pixels (default: 600)
	Scale   float64        `json:"scale,omitempty" yaml:"scale,omitempty"`       // Resolution scale (default: calculated dynamically, min 2.0)
	FitMode  string         `json:"fit_mode,omitempty" yaml:"fit_mode,omitempty"` // Fit mode: "stretch" (default), "contain", or "cover"
	Style    *DiagramStyle  `json:"style,omitempty" yaml:"style,omitempty"`       // Optional styling overrides
	Warnings []string       `json:"warnings,omitempty" yaml:"-"`                  // Non-fatal warnings (e.g., flat-map auto-conversion)
}

// DiagramStyle provides styling options for diagram rendering.
type DiagramStyle struct {
	Colors      []string     `json:"colors,omitempty" yaml:"colors,omitempty"`           // Hex colors for data series
	ThemeColors []ThemeColor `json:"-" yaml:"-"`                                         // Theme colors from template (internal use)
	FontFamily  string       `json:"font_family,omitempty" yaml:"font_family,omitempty"` // Font for labels and text
	ShowLegend  bool         `json:"show_legend,omitempty" yaml:"show_legend,omitempty"` // Display legend
	ShowValues  bool         `json:"show_values,omitempty" yaml:"show_values,omitempty"` // Display values on elements
	Background  string       `json:"background,omitempty" yaml:"background,omitempty"`   // Background color
}

// ParseError represents a non-fatal parsing issue with source position.
type ParseError struct {
	SourceFile string // Source filename (empty if not set)
	Line       int    // 1-based line number in the source document
	Column     int    // 1-based column number (0 means unknown)
	Message    string // Error description
	Field      string // Affected field name
	Level      ErrorLevel
}

// Format returns the error in file:line:col format.
// Examples:
//
//	input.md:42:3: warning: slide has 15 bullet points (>8)
//	input.md:10: error: required field 'title' is missing
//	line 42: warning: slide has 15 bullet points (>8)
func (e ParseError) Format() string {
	var prefix string
	if e.SourceFile != "" {
		prefix = e.SourceFile
	}

	if prefix != "" {
		if e.Column > 0 {
			prefix = fmt.Sprintf("%s:%d:%d", prefix, e.Line, e.Column)
		} else {
			prefix = fmt.Sprintf("%s:%d", prefix, e.Line)
		}
	} else {
		if e.Column > 0 {
			prefix = fmt.Sprintf("line %d:%d", e.Line, e.Column)
		} else {
			prefix = fmt.Sprintf("line %d", e.Line)
		}
	}

	return fmt.Sprintf("%s: %s: %s", prefix, e.Level, e.Message)
}

// ErrorLevel indicates the severity of a parse error.
type ErrorLevel string

const (
	ErrorLevelWarning ErrorLevel = "warning" // Non-fatal warning
	ErrorLevelError   ErrorLevel = "error"   // Fatal error
)
