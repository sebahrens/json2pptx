package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// Type aliases for shape grid and table types now defined in internal/jsonschema.
// These aliases allow existing code in package main to continue using the
// unqualified type names (e.g., ShapeGridInput instead of jsonschema.ShapeGridInput).
// ---------------------------------------------------------------------------

type ShapeGridInput = jsonschema.ShapeGridInput
type GridBoundsInput = jsonschema.GridBoundsInput
type GridRowInput = jsonschema.GridRowInput
type ConnectorSpecInput = jsonschema.ConnectorSpecInput
type GridCellInput = jsonschema.GridCellInput
type AccentBarInput = jsonschema.AccentBarInput
type GridImageInput = jsonschema.GridImageInput
type GridOverlayInput = jsonschema.GridOverlayInput
type GridImageTextInput = jsonschema.GridImageTextInput
type IconInput = jsonschema.IconInput
type ShapeSpecInput = jsonschema.ShapeSpecInput
type ShapeFillInput = jsonschema.ShapeFillInput
type TableInput = jsonschema.TableInput
type TableCellInput = jsonschema.TableCellInput
type TableStyleInput = jsonschema.TableStyleInput

// PresentationInput is the top-level typed JSON input.
// Maps to generator.GenerationRequest.
type PresentationInput struct {
	Template       string          `json:"template"`
	OutputFilename string          `json:"output_filename,omitempty"`
	DesignMode      string          `json:"design_mode,omitempty"`      // "constrained" (default) or "free"
	AccentStrategy  string          `json:"accent_strategy,omitempty"`  // "primary" (default), "rotate", or "section-keyed"
	Footer          *JSONFooter     `json:"footer,omitempty"`
	Chrome         *ChromeInput    `json:"chrome,omitempty"`
	ThemeOverride  *ThemeInput     `json:"theme_override,omitempty"`
	Defaults       *DefaultsInput  `json:"defaults,omitempty"`
	Grid           *GridConfig     `json:"grid,omitempty"`
	Structure      *StructureInput `json:"structure,omitempty"`
	Slides         []SlideInput    `json:"slides"`
}

// ChromeInput configures deck-level persistent chrome (footers, page numbers,
// confidentiality marks) that appear on every content slide but are suppressed
// on title and closing slides.
type ChromeInput struct {
	// Confidentiality is a classification stamp (e.g., "Strictly confidential").
	Confidentiality string `json:"confidentiality,omitempty"`
	// ClientName is the client or company name (e.g., "Acme Corp").
	ClientName string `json:"client_name,omitempty"`
	// ProjectCode is the project identifier (e.g., "Aurora").
	ProjectCode string `json:"project_code,omitempty"`
	// FooterDate is the date string shown in the footer (e.g., "May 2026").
	FooterDate string `json:"footer_date,omitempty"`
	// PageNumbers controls slide numbering.
	PageNumbers *PageNumbersInput `json:"page_numbers,omitempty"`
	// SectionCrumb enables running section title in the footer. Requires
	// the structure block with named sections to be useful.
	SectionCrumb bool `json:"section_crumb,omitempty"`
}

// PageNumbersInput controls slide number display within chrome.
type PageNumbersInput struct {
	// Enabled turns page numbers on or off (default: true when chrome is set).
	Enabled *bool `json:"enabled,omitempty"`
	// Format is the page number format string. Supports {current} and {total}
	// placeholders (e.g., "{current} / {total}"). Default: plain slide number.
	Format string `json:"format,omitempty"`
	// Skip lists slide types that should not show page numbers (e.g., ["title", "closing"]).
	// Default: ["title", "closing"].
	Skip []string `json:"skip,omitempty"`
}

// StructureInput defines deck-level structural grammar. When present, the
// generator expands sections into a flat slide sequence with auto-generated
// section dividers and optional agenda slide. Mutually exclusive with top-level
// slides — if structure is set, slides must be empty.
type StructureInput struct {
	// Cover is the opening title slide (placed first in the deck).
	Cover *SlideInput `json:"cover,omitempty"`
	// Closing is the closing title slide (placed last in the deck).
	Closing *SlideInput `json:"closing,omitempty"`
	// AutoAgenda generates an agenda slide listing all section titles,
	// inserted after the cover slide. Requires at least 2 sections.
	AutoAgenda bool `json:"auto_agenda,omitempty"`
	// Sections defines the content groups. Each section gets an auto-generated
	// section divider slide before its content slides.
	Sections []SectionInput `json:"sections"`
}

// SectionInput defines a single section within the deck structure.
type SectionInput struct {
	// Title is the section name, used for the section divider and agenda.
	Title string `json:"title"`
	// Slides contains the content slides within this section.
	Slides []SlideInput `json:"slides"`
}

// DefaultsInput provides deck-level defaults that are shallow-applied to every
// matching block before struct validation. Swap-only semantics: if a block sets
// a field inline, that field wins; otherwise the defaults value is copied in.
type DefaultsInput struct {
	TableStyle *TableStyleInput            `json:"table_style,omitempty"`
	CellStyle  *jsonschema.ShapeSpecInput  `json:"cell_style,omitempty"`
}

// GridConfig specifies a deck-level layout rhythm grid that normalizes content
// positioning across all slides. When set, the generator snaps shape_grid bounds
// to the grid, ensuring titles align at the same Y and content regions start at
// the same Y across the deck.
type GridConfig struct {
	// Columns is the number of logical columns (default: 12). Currently
	// informational — column snapping is a future enhancement.
	Columns int `json:"columns,omitempty"`
	// GutterEMU is the gutter width between columns in EMU (default: 228600 = 0.25 inch).
	GutterEMU int64 `json:"gutter_emu,omitempty"`
	// TitleBaselinePct is the Y-position of the title baseline as a percentage
	// of slide height. All title placeholders align to this value (default: derived from template).
	TitleBaselinePct float64 `json:"title_baseline_pct,omitempty"`
	// ContentTopPct is the Y-position where content starts as a percentage of
	// slide height. Shape grids and body placeholders align to this value (default: derived from template).
	ContentTopPct float64 `json:"content_top_pct,omitempty"`
	// ContentBottomPct is the Y-position where content ends as a percentage of
	// slide height. Shape grids do not extend below this line (default: 92).
	ContentBottomPct float64 `json:"content_bottom_pct,omitempty"`
	// LeftMarginPct is the left margin as a percentage of slide width (default: derived from template).
	LeftMarginPct float64 `json:"left_margin_pct,omitempty"`
	// RightMarginPct is the right margin as a percentage of slide width (default: mirrors left).
	RightMarginPct float64 `json:"right_margin_pct,omitempty"`
}

// UnmarshalJSON handles both regular slides and split_slide entries.
// A split_slide entry is expanded inline into N regular SlideInput entries.
func (p *PresentationInput) UnmarshalJSON(data []byte) error {
	// Use type alias to avoid infinite recursion.
	type Alias PresentationInput
	aux := &struct {
		Slides []json.RawMessage `json:"slides"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	p.Slides = nil
	for i, raw := range aux.Slides {
		// Probe for the "type" field to detect split_slide entries.
		var probe struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return fmt.Errorf("slide %d: %w", i+1, err)
		}

		if probe.Type == "split_slide" {
			var ss SplitSlideInput
			if err := json.Unmarshal(raw, &ss); err != nil {
				return fmt.Errorf("slide %d: invalid split_slide: %w", i+1, err)
			}
			expanded, err := expandSplitSlide(ss)
			if err != nil {
				return fmt.Errorf("slide %d: %w", i+1, err)
			}
			p.Slides = append(p.Slides, expanded...)
		} else {
			var slide SlideInput
			if err := json.Unmarshal(raw, &slide); err != nil {
				return fmt.Errorf("slide %d: %w", i+1, err)
			}
			p.Slides = append(p.Slides, slide)
		}
	}

	return nil
}

// ThemeInput maps to types.ThemeOverride.
type ThemeInput struct {
	Colors    map[string]string `json:"colors,omitempty"`
	TitleFont string            `json:"title_font,omitempty"`
	BodyFont  string            `json:"body_font,omitempty"`
}

// ToThemeOverride converts ThemeInput to types.ThemeOverride.
func (t *ThemeInput) ToThemeOverride() *types.ThemeOverride {
	if t == nil {
		return nil
	}
	return &types.ThemeOverride{
		Colors:    t.Colors,
		TitleFont: t.TitleFont,
		BodyFont:  t.BodyFont,
	}
}

// SlideInput maps to generator.SlideSpec with full metadata.
type SlideInput struct {
	LayoutID        string           `json:"layout_id,omitempty"`
	SlideType       string           `json:"slide_type,omitempty"` // Optional hint: content, title, section, chart, two-column, diagram, image, comparison, blank
	Eyebrow         string           `json:"eyebrow,omitempty"`    // Small-caps label above title (e.g., "STRATEGY — Market Expansion")
	Background      *BackgroundInput `json:"background,omitempty"`
	Content         []ContentInput   `json:"content"`
	ShapeGrid       *ShapeGridInput  `json:"shape_grid,omitempty"`
	Pattern         *PatternInput    `json:"pattern,omitempty"`
	Compose         *ComposeInput    `json:"compose,omitempty"`
	SpeakerNotes    string           `json:"speaker_notes,omitempty"`
	Source          string           `json:"source,omitempty"`
	Transition      string           `json:"transition,omitempty"`
	TransitionSpeed string           `json:"transition_speed,omitempty"`
	Build           string           `json:"build,omitempty"`
	ContrastCheck   *bool            `json:"contrast_check,omitempty"`
}

// BackgroundInput defines a slide background image.
type BackgroundInput struct {
	Image string `json:"image,omitempty"` // File path to background image
	URL   string `json:"url,omitempty"`   // HTTP/HTTPS URL to download background image from
	Fit   string `json:"fit,omitempty"`   // "cover" (default), "stretch", "tile"
}

// ContentInput is a discriminated union for content items.
// The "type" field determines which typed value field to use.
// For backward compat, "value" (json.RawMessage) is also supported.
type ContentInput struct {
	PlaceholderID string `json:"placeholder_id"`
	Type          string `json:"type"`

	// Legacy field — used when typed fields are not set.
	Value json.RawMessage `json:"value,omitempty"`

	// Typed value fields (use ONE, matching the "type" discriminator):
	TextValue           *string              `json:"text_value,omitempty"`
	BulletsValue        *[]string            `json:"bullets_value,omitempty"`
	BodyAndBulletsValue *BodyAndBulletsInput `json:"body_and_bullets_value,omitempty"`
	BodyAndLeadValue    *BodyAndLeadInput    `json:"body_and_lead_value,omitempty"`
	BulletGroupsValue   *BulletGroupsInput   `json:"bullet_groups_value,omitempty"`
	TableValue          *TableInput          `json:"table_value,omitempty"`
	ChartValue          *types.ChartSpec     `json:"chart_value,omitempty"` //nolint:staticcheck // ChartSpec is deprecated but still used for backward compat
	DiagramValue        *types.DiagramSpec   `json:"diagram_value,omitempty"`
	ImageValue          *ImageInput          `json:"image_value,omitempty"`

	// FontSize overrides the template's default font size for this content item.
	// Value is in points (e.g., 72 for 72pt). Only applies to text-based content types.
	FontSize *float64 `json:"font_size,omitempty"`
}

// UsesLegacyValue reports whether this content item relies on the legacy
// "value" field instead of the canonical typed fields (text_value, etc.).
func (c *ContentInput) UsesLegacyValue() bool {
	if len(c.Value) == 0 {
		return false
	}
	// If any typed field is set, the legacy field is ignored.
	switch c.Type {
	case "text":
		return c.TextValue == nil
	case "bullets":
		return c.BulletsValue == nil
	case "body_and_bullets":
		return c.BodyAndBulletsValue == nil
	case "body_and_lead":
		return c.BodyAndLeadValue == nil
	case "bullet_groups":
		return c.BulletGroupsValue == nil
	case "table":
		return c.TableValue == nil
	case "chart":
		return c.ChartValue == nil
	case "diagram":
		return c.DiagramValue == nil
	case "image":
		return c.ImageValue == nil
	default:
		return false
	}
}

// ResolveValue returns the typed value for this content item.
// Priority: typed field > legacy Value json.RawMessage.
// Returns (value, error). A nil value with nil error signals
// that the caller should use the legacy decode path.
func (c *ContentInput) ResolveValue() (any, error) { //nolint:gocognit,gocyclo
	switch c.Type {
	case "text":
		if c.TextValue != nil {
			return *c.TextValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("text content requires text_value or value")
		}
		var s string
		if err := json.Unmarshal(c.Value, &s); err != nil {
			return nil, fmt.Errorf("invalid text value: %w", err)
		}
		return s, nil

	case "bullets":
		if c.BulletsValue != nil {
			return *c.BulletsValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("bullets content requires bullets_value or value")
		}
		var b []string
		if err := json.Unmarshal(c.Value, &b); err != nil {
			return nil, fmt.Errorf("invalid bullets value: %w", err)
		}
		return b, nil

	case "body_and_bullets":
		if c.BodyAndBulletsValue != nil {
			return c.BodyAndBulletsValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("body_and_bullets content requires body_and_bullets_value or value")
		}
		var v BodyAndBulletsInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid body_and_bullets value: %w", err)
		}
		return &v, nil

	case "body_and_lead":
		if c.BodyAndLeadValue != nil {
			return c.BodyAndLeadValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("body_and_lead content requires body_and_lead_value or value")
		}
		var v BodyAndLeadInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid body_and_lead value: %w", err)
		}
		return &v, nil

	case "bullet_groups":
		if c.BulletGroupsValue != nil {
			return c.BulletGroupsValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("bullet_groups content requires bullet_groups_value or value")
		}
		var v BulletGroupsInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid bullet_groups value: %w", err)
		}
		return &v, nil

	case "table":
		if c.TableValue != nil {
			return c.TableValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("table content requires table_value or value")
		}
		var v TableInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid table value: %w", err)
		}
		return &v, nil

	case "chart":
		if c.ChartValue != nil {
			return c.ChartValue, nil
		}
		// nil signals: use legacy decode path in json_mode.go
		return nil, nil

	case "diagram":
		if c.DiagramValue != nil {
			return c.DiagramValue, nil
		}
		return nil, nil

	case "image":
		if c.ImageValue != nil {
			return c.ImageValue, nil
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown content type: %q", c.Type)
	}
}

// BodyAndBulletsInput maps to generator.BodyAndBulletsContent.
type BodyAndBulletsInput struct {
	Body         string   `json:"body"`
	Bullets      []string `json:"bullets"`
	TrailingBody string   `json:"trailing_body,omitempty"`
}

// BodyAndLeadInput maps to generator.BodyAndLeadContent.
type BodyAndLeadInput struct {
	Lead    string   `json:"lead"`    // Lead-in paragraph (rendered 16pt bold)
	Bullets []string `json:"bullets"` // Supporting bullets (rendered 12pt)
}

// BulletGroupsInput maps to generator.BulletGroupsContent.
type BulletGroupsInput struct {
	Body         string             `json:"body,omitempty"`
	Groups       []BulletGroupInput `json:"groups"`
	TrailingBody string             `json:"trailing_body,omitempty"`
}

// BulletGroupInput maps to generator.BulletGroup.
type BulletGroupInput struct {
	Header     string   `json:"header,omitempty"`
	Body       string   `json:"body,omitempty"`
	Bullets    []string `json:"bullets"`
	GroupLabel string   `json:"group_label,omitempty"` // Small-caps accent label above header
}

// ImageInput maps to generator.ImageContent.
type ImageInput struct {
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"` // HTTP/HTTPS URL to download the image from
	Alt  string `json:"alt,omitempty"`
}
