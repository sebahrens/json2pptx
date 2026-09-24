package types

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

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

	// SurfaceTints maps surface roles to theme color names for tinted backgrounds.
	// Roles: "subtle" (lightest tint), "paper" (card/panel background),
	// "elevated" (raised surface), "inverse" (dark surface for contrast).
	// Values are scheme color names (e.g. "lt2", "accent1").
	SurfaceTints map[string]string `json:"surface_tints,omitempty"`

	// DataPalette is an ordered list of scheme color names for chart series.
	// svggen uses this to ensure chart colors match the template's visual identity.
	// Example: ["accent1", "accent2", "accent5", "accent3", "accent6", "accent4"]
	DataPalette []string `json:"data_palette,omitempty"`

	// AccentUsageGuide maps accent color names to prose descriptions of their
	// intended visual role (e.g., "accent1": "strong primary headers").
	// Template authors supply this; the engine never synthesises it.
	// When present, list_templates surfaces it so agents can pick colours with intent.
	AccentUsageGuide map[string]string `json:"accent_usage_guide,omitempty"`
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
	TableStyles  []TableStyleInfo   // Table styles declared in ppt/tableStyles.xml
}

// TableStyleInfo describes a table style declared in a template.
type TableStyleInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LayoutMetadata describes a single slide layout in a template.
type LayoutMetadata struct {
	ID           string            // Internal layout ID from XML
	Name         string            // Human-readable layout name
	Index        int               // Position in template (zero-based)
	Placeholders []PlaceholderInfo // Placeholders in this layout
	Capacity     CapacityEstimate  // Content capacity estimate
	Tags         []string          // Classification tags

	// CanonicalType is the canonical layout role assigned by the single
	// authoritative layout classifier (see internal/template.ClassifyLayoutCanonical
	// / ClassifyCanonicalRole). It is the stable wire ID shared by template
	// parsing, generation, and preflight. Empty when the layout does not map to
	// any canonical role.
	CanonicalType CanonicalLayoutType

	// CanonicalConfidence is the 0.0–1.0 confidence of CanonicalType.
	CanonicalConfidence float64

	// MasterPath and ThemePath identify the actual OOXML inheritance chain for
	// this layout. Theme is the effective theme reached through that chain.
	// They are populated by template profiling and left empty by older callers.
	MasterPath string
	ThemePath  string
	Theme      ThemeInfo
	// BackgroundHex is the visible solid background inherited from this
	// layout or its master. Empty means it cannot be resolved to one color.
	BackgroundHex string
	// BackgroundRef preserves its literal hex or mapped theme scheme slot so
	// contrast preflight can re-resolve it after theme_override.
	BackgroundRef string
	// BackgroundMods records OOXML color transforms on the inherited solid
	// background so theme overrides can re-resolve its visible color.
	BackgroundMods BackgroundColorModifiers `json:"-"`
	// Chrome uses the layout's own background, not the master fallback used by
	// placeholder contrast. Keep that source and tx1 color-map override so
	// preflight can predict footer/page-number repairs after theme_override.
	ChromeBackgroundRef  string                   `json:"-"`
	ChromeBackgroundMods BackgroundColorModifiers `json:"-"`
	ChromeTextRef        string                   `json:"-"`

	// FooterRegions are the resolved date / footer / slide-number chrome
	// rectangles a slide on this layout carries: the layout's own dt/ftr/sldNum
	// placeholders when it declares them, otherwise the slide master's. They
	// are populated by ParseLayouts and feed the takeaway/source band and
	// footer geometry (template.ResolveChromeFrame).
	FooterRegions []ChromeRegion
	// DecorRegions are filled, non-placeholder shapes inherited from the layout
	// and its master. They are potential exclusions from authored content.
	DecorRegions []DecorRegion
}

// DecorRegion is the bounding box of visible template artwork.
type DecorRegion struct {
	Source string `json:"source"` // "layout" or "master"
	Name   string `json:"name"`
	X      int64  `json:"x_emu"`
	Y      int64  `json:"y_emu"`
	Width  int64  `json:"w_emu"`
	Height int64  `json:"h_emu"`
}

// BackgroundColorModifiers are OOXML background-fill transforms in 1/100000
// units. Presence flags distinguish an absent transform from an explicit zero.
type BackgroundColorModifiers struct {
	LumMod, LumOff, Tint, Shade, Alpha int
	HasLumMod, HasAlpha                bool
}

// ChromeRegion is one resolved chrome rectangle of a layout (e.g. the "ftr"
// footer placeholder inherited from the slide master).
type ChromeRegion struct {
	Type   string `json:"type"` // OOXML placeholder type: "dt", "ftr", or "sldNum"
	X      int64  `json:"x_emu"`
	Y      int64  `json:"y_emu"`
	Width  int64  `json:"w_emu"`
	Height int64  `json:"h_emu"`
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
	// FontColorMods and InheritedFontColorMods retain text-color transforms so
	// preflight can judge the visible color against an authored background.
	// These are internal analysis metadata, not an author-facing color change.
	FontColorMods          BackgroundColorModifiers `json:"-"`
	InheritedFontColorMods BackgroundColorModifiers `json:"-"`
	// InheritedFontColor is the effective layout/master text-color reference,
	// mapped through the layout's clrMapOvr. FontColor keeps its narrower
	// meaning — "the layout declared this" — because examine_template reports
	// it to agents as the template's own declaration.
	//
	// Without it the contrast preflight simply skipped every placeholder whose
	// colour is inherited, so a layout like midnight-blue's Section Divider got
	// a contrast_autofixed swap at generate time and silence at validate time
	// (go-slide-creator-j4364).
	InheritedFontColor string

	// TextCaps and LineSpacingPct are the inherited text style that changes
	// measured fit (resolved for title placeholders from the layout lstStyle →
	// master titleStyle): all-caps rendering and the lnSpc percentage (0 =
	// single spacing).
	TextCaps       bool
	LineSpacingPct int

	// SpcBefPt is the inherited space-before in points (master bodyStyle /
	// titleStyle level 1, overridden by the layout placeholder's own lstStyle).
	// A body placeholder's autofit has to budget it per paragraph: fourteen
	// bullets at a 10pt spcBef is 140pt of height, which is the difference
	// between a predicted 70% font scale and the 50% the renderer applies
	// (go-slide-creator-nlrg).
	SpcBefPt float64

	// Role is the canonical, agent-facing placeholder role assigned by
	// internal/template.ClassifyPlaceholderRole. It refines Type with
	// intent-level distinctions (eyebrow vs title, section_number vs body,
	// date/footer/page_number vs other) and is the single source of truth for
	// role-aware content placement and diagnostics. Empty until classified.
	Role PlaceholderRole

	// RoleConfidence is the 0.0–1.0 confidence of Role.
	RoleConfidence float64
}

// IsAutoFilledPlaceholder reports template fields populated by the generator,
// not by an authored slide placeholder value.
func IsAutoFilledPlaceholder(id string) bool {
	return strings.EqualFold(id, "Section Number") || strings.EqualFold(id, "section_number")
}

// PlaceholderRole is the canonical, agent-facing role of a placeholder within a
// layout. Unlike PlaceholderType (which mirrors the raw OOXML placeholder type),
// the role captures intent: an eyebrow/kicker is distinguished from a title, a
// decorative section number from a body, and date/footer/page-number chrome from
// generic "other" placeholders. The string values are stable wire IDs.
type PlaceholderRole string

const (
	PlaceholderRoleTitle         PlaceholderRole = "title"
	PlaceholderRoleSubtitle      PlaceholderRole = "subtitle"
	PlaceholderRoleEyebrow       PlaceholderRole = "eyebrow"
	PlaceholderRoleSectionNumber PlaceholderRole = "section_number"
	PlaceholderRoleBody          PlaceholderRole = "body"
	PlaceholderRoleImage         PlaceholderRole = "image"
	PlaceholderRoleChart         PlaceholderRole = "chart"
	PlaceholderRoleFooter        PlaceholderRole = "footer"
	PlaceholderRolePageNumber    PlaceholderRole = "page_number"
	PlaceholderRoleDate          PlaceholderRole = "date"
	PlaceholderRoleOther         PlaceholderRole = "other"
)

// CanonicalLayoutType is the canonical, agent-facing type of a slide layout. Its
// values are the same stable wire IDs used by the layout classifier in
// internal/template (CanonicalRole* constants); there is intentionally one
// layout taxonomy, not two. Empty (CanonicalLayoutUnknown) means the layout does
// not structurally correspond to any canonical role.
type CanonicalLayoutType string

const (
	CanonicalLayoutUnknown        CanonicalLayoutType = ""
	CanonicalLayoutTitleSlide     CanonicalLayoutType = "Title Slide"
	CanonicalLayoutOneContent     CanonicalLayoutType = "One Content"
	CanonicalLayoutTwoContent     CanonicalLayoutType = "Two Content"
	CanonicalLayoutSectionDivider CanonicalLayoutType = "Section Divider"
	CanonicalLayoutBlank          CanonicalLayoutType = "Blank"
	CanonicalLayoutBlankTitle     CanonicalLayoutType = "Blank + Title"
	CanonicalLayoutClosing        CanonicalLayoutType = "Closing"
)

// CanonicalLayoutFamily is the coarse four-way grouping of layout types used by
// the layout taxonomy (title-slide, section-divider, one-content, qa-closing).
// It is a view over CanonicalLayoutType — every template should provide at least
// one layout in each content-bearing family — not an independent taxonomy.
type CanonicalLayoutFamily string

const (
	LayoutFamilyTitleSlide     CanonicalLayoutFamily = "title-slide"
	LayoutFamilySectionDivider CanonicalLayoutFamily = "section-divider"
	LayoutFamilyOneContent     CanonicalLayoutFamily = "one-content"
	LayoutFamilyQAClosing      CanonicalLayoutFamily = "qa-closing"
	LayoutFamilyOther          CanonicalLayoutFamily = "other"
)

// Family maps a canonical layout type to its coarse four-way family. Blank and
// Blank+Title are utility layouts and map to LayoutFamilyOther.
func (t CanonicalLayoutType) Family() CanonicalLayoutFamily {
	switch t {
	case CanonicalLayoutTitleSlide:
		return LayoutFamilyTitleSlide
	case CanonicalLayoutSectionDivider:
		return LayoutFamilySectionDivider
	case CanonicalLayoutOneContent, CanonicalLayoutTwoContent:
		return LayoutFamilyOneContent
	case CanonicalLayoutClosing:
		return LayoutFamilyQAClosing
	default:
		return LayoutFamilyOther
	}
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

// ApplyOverride merges a ThemeOverride into this ThemeInfo, returning a new copy
// and warnings for non-embedded font overrides or unrecognized color keys.
// Only non-empty override values replace template defaults.
func (t ThemeInfo) ApplyOverride(o *ThemeOverride) (ThemeInfo, []string) {
	if o == nil {
		return t, nil
	}

	result := ThemeInfo{
		Name:      t.Name,
		TitleFont: t.TitleFont,
		BodyFont:  t.BodyFont,
		Colors:    make([]ThemeColor, len(t.Colors)),
	}
	copy(result.Colors, t.Colors)

	// Override fonts, warning when the replacement font is not in the template's theme.
	var warnings []string
	if o.TitleFont != "" {
		if o.TitleFont != t.TitleFont {
			warnings = append(warnings, fmt.Sprintf(
				"theme_override.title_font: %q is not embedded in template (template uses %q) and may substitute at render time",
				o.TitleFont, t.TitleFont))
		}
		result.TitleFont = o.TitleFont
	}
	if o.BodyFont != "" {
		if o.BodyFont != t.BodyFont {
			warnings = append(warnings, fmt.Sprintf(
				"theme_override.body_font: %q is not embedded in template (template uses %q) and may substitute at render time",
				o.BodyFont, t.BodyFont))
		}
		result.BodyFont = o.BodyFont
	}

	// Override colors by name, tracking which keys matched
	matched := make(map[string]bool, len(o.Colors))
	if len(o.Colors) > 0 {
		for i, c := range result.Colors {
			if hex, ok := o.Colors[c.Name]; ok {
				result.Colors[i].RGB = hex
				matched[c.Name] = true
			}
		}
	}

	// Warn about unrecognized color keys
	if len(matched) < len(o.Colors) {
		// Collect valid names for the suggestion
		validNames := make([]string, len(t.Colors))
		for i, c := range t.Colors {
			validNames[i] = c.Name
		}
		for key := range o.Colors {
			if !matched[key] {
				warnings = append(warnings, fmt.Sprintf(
					"theme_override.colors.%s: unknown scheme color key (ignored); valid keys: %v",
					key, validNames))
			}
		}
		sort.Strings(warnings)
	}

	return result, warnings
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
