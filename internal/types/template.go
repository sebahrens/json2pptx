package types

import "time"

// MetadataVersion constants define supported template metadata schema versions.
// Version history:
//   - v1.0: Initial version with basic metadata (name, description, author, tags)
const (
	// MetadataVersionCurrent is the current metadata schema version.
	MetadataVersionCurrent = "1.0"

	// MetadataVersionMin is the minimum supported metadata schema version.
	MetadataVersionMin = "1.0"
)

// TemplateMetadata contains versioned metadata for a PowerPoint template.
// This is stored as JSON in ppt/go-slide-creator-metadata.json within the PPTX.
type TemplateMetadata struct {
	// Version is the metadata schema version (e.g., "1.0").
	// Required field - templates without version are considered v1.0.
	Version string `json:"version"`

	// Name is the template display name (e.g., "Corporate Presentation").
	Name string `json:"name,omitempty"`

	// Description provides details about the template's purpose and style.
	Description string `json:"description,omitempty"`

	// Author identifies who created the template.
	Author string `json:"author,omitempty"`

	// Tags are keywords for template categorization and search.
	Tags []string `json:"tags,omitempty"`

	// CreatedAt is when the template was first created.
	CreatedAt *time.Time `json:"created_at,omitempty"`

	// UpdatedAt is when the template was last modified.
	UpdatedAt *time.Time `json:"updated_at,omitempty"`

	// AspectRatio overrides auto-detected ratio (e.g., "16:9", "4:3").
	AspectRatio string `json:"aspect_ratio,omitempty"`

	// LayoutHints provides additional hints for specific layouts.
	LayoutHints map[string]LayoutHint `json:"layout_hints,omitempty"`

	// SemanticAccents maps semantic roles (positive, negative, neutral) to
	// theme accent names (e.g. "accent3"). Patterns that set semantic_accent
	// resolve through this map; templates without it fall back to accent1.
	SemanticAccents map[string]string `json:"semantic_accents,omitempty"`
}

// LayoutHint provides additional metadata hints for a specific layout.
type LayoutHint struct {
	// PreferredFor indicates content types this layout works best with.
	PreferredFor []string `json:"preferred_for,omitempty"`

	// MaxBullets overrides computed bullet capacity.
	MaxBullets int `json:"max_bullets,omitempty"`

	// MaxChars overrides computed character capacity.
	MaxChars int `json:"max_chars,omitempty"`

	// Deprecated marks a layout as deprecated (should not be auto-selected).
	Deprecated bool `json:"deprecated,omitempty"`
}

// SynthesisManifest stores generated layout XML bytes for synthetic layouts.
// These are produced when a template lacks required capabilities (e.g., two-column).
// The generator writes these bytes into the output PPTX alongside native layout files.
type SynthesisManifest struct {
	// SyntheticFiles maps layout paths (e.g., "ppt/slideLayouts/slideLayout99.xml")
	// to their generated XML bytes. Also includes .rels files.
	SyntheticFiles map[string][]byte
}

// TemplateAnalysis contains the complete analysis of a PowerPoint template.
type TemplateAnalysis struct {
	TemplatePath string             // Path to the template file
	Hash         string             // SHA256 hash of file for cache validation
	AspectRatio  string             // "16:9" or "4:3"
	SlideWidth   int64              // Slide width in EMU from presentation.xml <p:sldSz> (0 = unknown, use 16:9 default)
	SlideHeight  int64              // Slide height in EMU from presentation.xml <p:sldSz> (0 = unknown, use 16:9 default)
	Layouts      []LayoutMetadata   // Available slide layouts
	Theme        ThemeInfo          // Theme colors and fonts
	AnalyzedAt   time.Time          // Timestamp of analysis
	Metadata     *TemplateMetadata  // Optional embedded metadata (nil if not present)
	Synthesis    *SynthesisManifest // nil if no synthesis needed
}

// LayoutMetadata describes a single slide layout in a template.
type LayoutMetadata struct {
	ID           string            // Internal layout ID from XML
	Name         string            // Human-readable layout name
	Index        int               // Position in template (zero-based)
	Placeholders []PlaceholderInfo // Placeholders in this layout
	Capacity     CapacityEstimate  // Content capacity estimate
	Tags         []string          // Classification tags
}

// PlaceholderInfo describes a placeholder within a layout.
type PlaceholderInfo struct {
	ID       string          // Placeholder ID
	Type     PlaceholderType // Type of placeholder
	Index    int             // Placeholder index for population
	Bounds   BoundingBox     // Position and size in EMUs
	MaxChars int             // Estimated character capacity

	// Font properties (resolved from placeholder, layout, master, or theme)
	FontFamily string // Font family name (e.g., "Arial", "Calibri")
	FontSize   int    // Font size in hundredths of a point (e.g., 1400 = 14pt)
	FontColor  string // Font color as hex string (e.g., "#000000")
}

// PlaceholderType represents the type of content a placeholder accepts.
type PlaceholderType string

const (
	PlaceholderTitle    PlaceholderType = "title"    // Title placeholder
	PlaceholderSubtitle PlaceholderType = "subtitle" // Subtitle placeholder (on title slides)
	PlaceholderBody     PlaceholderType = "body"     // Body text placeholder
	PlaceholderImage    PlaceholderType = "image"    // Image placeholder
	PlaceholderChart    PlaceholderType = "chart"    // Chart placeholder
	PlaceholderTable    PlaceholderType = "table"    // Table placeholder
	PlaceholderContent  PlaceholderType = "content"  // Generic content placeholder
	PlaceholderOther    PlaceholderType = "other"    // Non-content utility placeholders (date, footer, slide number)
)

// BoundingBox represents a rectangular area in EMUs (English Metric Units).
// 914400 EMUs = 1 inch
type BoundingBox struct {
	X      int64 // EMUs from left edge
	Y      int64 // EMUs from top edge
	Width  int64 // Width in EMUs
	Height int64 // Height in EMUs
}

// CapacityEstimate provides hints about layout content capacity.
type CapacityEstimate struct {
	MaxBullets    int  // Comfortable number of bullet points
	MaxTextLines  int  // Text lines before overflow
	HasImageSlot  bool // Contains image placeholder
	HasChartSlot  bool // Contains chart placeholder
	TextHeavy     bool // Primarily text-focused layout
	VisualFocused bool // Primarily visual-focused layout
}

// ThemeInfo contains theme colors and typography information.
type ThemeInfo struct {
	Name      string       // Theme name
	Colors    []ThemeColor // Theme colors
	TitleFont string       // Font for titles
	BodyFont  string       // Font for body text
}

// ThemeColor represents a single color in the theme.
type ThemeColor struct {
	Name string // Color name (accent1, accent2, dk1, lt1, etc.)
	RGB  string // Hex color value (e.g., "#FF0000")
}

// ApplyOverride merges a ThemeOverride into this ThemeInfo, returning a new copy.
// Only non-empty override values replace template defaults.
func (t ThemeInfo) ApplyOverride(o *ThemeOverride) ThemeInfo {
	if o == nil {
		return t
	}

	result := ThemeInfo{
		Name:      t.Name,
		TitleFont: t.TitleFont,
		BodyFont:  t.BodyFont,
		Colors:    make([]ThemeColor, len(t.Colors)),
	}
	copy(result.Colors, t.Colors)

	// Override fonts
	if o.TitleFont != "" {
		result.TitleFont = o.TitleFont
	}
	if o.BodyFont != "" {
		result.BodyFont = o.BodyFont
	}

	// Override colors by name
	if len(o.Colors) > 0 {
		for i, c := range result.Colors {
			if hex, ok := o.Colors[c.Name]; ok {
				result.Colors[i].RGB = hex
			}
		}
	}

	return result
}

// TemplateCache provides caching for template analysis results.
type TemplateCache interface {
	Get(path string) (*TemplateAnalysis, bool)
	Set(path string, analysis *TemplateAnalysis)
	Invalidate(path string)
	Clear()    // Clear removes all entries from the cache
	Size() int // Size returns the number of entries in the cache
}

// FastValidationCache is an optional interface for caches that support fast modTime-based validation.
// This avoids expensive hash calculation on every request.
// Implementations should embed TemplateCache.
type FastValidationCache interface {
	TemplateCache
	GetWithFastValidation(path string) (*TemplateAnalysis, bool)
	SetWithModTime(path string, analysis *TemplateAnalysis, modTime time.Time)
}
