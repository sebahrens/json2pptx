package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"log/slog"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/layoutpreview"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// skillInfo is the top-level JSON output for the skill-info subcommand.
//
// Pagination fields (TotalCount, PageSize, NextCursor) are populated by the
// list_templates MCP handler when iterating across template entries. The CLI
// emits all templates at once, so those fields stay zero/empty there.
type skillInfo struct {
	Tool            skillToolInfo         `json:"tool"`
	Templates       []skillTemplateInfo   `json:"templates"`
	SupportedTypes  skillSupportedTypes   `json:"supported_types"`
	PatternsCompact []skillPatternCompact `json:"patterns_compact,omitempty"`
	PatternsFull    []skillPatternFull    `json:"patterns_full,omitempty"`
	Compose         *skillComposeEntry    `json:"compose,omitempty"`
	InputFormats    []string              `json:"input_formats"`
	OutputFormats   []string              `json:"output_formats"`
	IconPolicy      *skillIconPolicy      `json:"icon_policy,omitempty"`
	Deprecations    []skillDeprecation    `json:"deprecations,omitempty"`
	// TotalCount is the total number of templates discovered, irrespective
	// of the current page slice. Omitted when zero (CLI / non-paginated use).
	TotalCount int `json:"total_count,omitempty"`
	// PageSize is the maximum number of template entries the caller
	// requested. Omitted when zero (CLI / non-paginated use).
	PageSize int `json:"page_size,omitempty"`
	// NextCursor is an opaque continuation token. Present only when more
	// template entries remain after the current page; pass it back as the
	// `cursor` argument to retrieve the next slice.
	NextCursor string `json:"next_cursor,omitempty"`
	// Warnings carries per-call advisory hints (e.g. deprecation notices
	// when an agent calls list_templates without the new `fields` projection
	// parameter). It is not a validation/error channel.
	Warnings []string `json:"warnings,omitempty"`
	// SideEffects documents the disk side effects of this discovery call —
	// specifically whether template analysis writes layout-preview PNG cache
	// files, where, and how to suppress them. Always populated by the CLI
	// skill-info and MCP list_templates surfaces so an agent in read-only
	// planning mode can tell whether the call touched the filesystem.
	SideEffects *skillSideEffects `json:"side_effects,omitempty"`
}

// skillSideEffects documents the disk side effects of a discovery call. The
// only side effect skill-info / list_templates can produce is writing
// layout-preview PNG cache files; this block reports whether the current call
// did (or could) write them and names the opt-out so an agent operating in
// read-only planning mode can gather template context without cache writes.
type skillSideEffects struct {
	// PreviewCacheWrites reports whether this call may write layout-preview PNG
	// cache files to disk. False when read-only / no-preview mode is active.
	// When true, actual writes still require the render toolchain (LibreOffice +
	// ImageMagick); when those are absent, preview generation no-ops silently.
	PreviewCacheWrites bool `json:"preview_cache_writes"`
	// ReadOnly reports whether read-only / no-preview mode was active for this
	// call (no layout-preview artifacts written).
	ReadOnly bool `json:"read_only"`
	// PreviewCacheDir is the base directory layout-preview PNGs are cached under
	// when PreviewCacheWrites is true. Reported even in read-only mode so agents
	// know which location the default mode would touch.
	PreviewCacheDir string `json:"preview_cache_dir,omitempty"`
	// DisableWith names the parameter or flag that suppresses preview cache
	// writes (e.g. "read_only=true" for the MCP tool, "--no-preview" for the
	// CLI subcommand).
	DisableWith string `json:"disable_with"`
}

// skillInfoOptions controls optional behavior of per-template skill-info
// analysis.
type skillInfoOptions struct {
	// NoPreview skips layout-preview PNG generation (and the cache writes it
	// performs), making analysis side-effect-free for read-only discovery.
	NoPreview bool
}

// skillComposeEntry describes the compose envelope feature for agents browsing
// skill-info. It surfaces the capability caps, the supported directions, and
// concrete examples so an agent can author a ComposeInput without reading the
// raw input schema or the recommend_visual schema first.
type skillComposeEntry struct {
	Description     string   `json:"description"`
	Directions      []string `json:"directions"`
	MaxSegments     int      `json:"max_segments"`
	MaxNestingDepth int      `json:"max_nesting_depth"`
	MaxLeafPatterns int      `json:"max_leaf_patterns"`
	SmartCompose    bool     `json:"smart_compose"`
	NestedCompose   bool     `json:"nested_compose"`
	// SupportsBanner advertises that ComposeInput.banner is honored: an
	// envelope-level decoration band rendered above the merged grid that does
	// not consume a segment slot.
	SupportsBanner bool `json:"supports_banner"`
	// SupportsCallout advertises that ComposeInput.callout is honored: an
	// envelope-level decoration band rendered below the merged grid that does
	// not consume a segment slot.
	SupportsCallout bool `json:"supports_callout"`
	// SupportsDiagramSegments advertises that SegmentInput may carry a
	// standalone svggen diagram as a third XOR alternative to pattern /
	// compose, letting a native pattern coexist with a chart/diagram on the
	// same slide without flattening through a single-cell grid.
	SupportsDiagramSegments bool                  `json:"supports_diagram_segments"`
	Examples                []skillComposeExample `json:"examples"`
}

// skillComposeExample is a worked compose envelope an agent can adapt. The
// JSON is intentionally minimal so it fits into a system-prompt-sized hint.
type skillComposeExample struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	JSON        json.RawMessage `json:"json"`
}

// skillIconPolicy advertises the deck-wide icon contract: emoji codepoints are
// rejected by pattern validators, and exactly one of the listed IconInput
// sources must be set per icon. Surfaced so agents reading skill-info pick a
// bundled name (or a loadable source) rather than dropping an emoji glyph into
// a pattern field.
type skillIconPolicy struct {
	NoEmoji         bool     `json:"no_emoji"`         // hard rule: emoji codepoints rejected anywhere in deck JSON
	AcceptedSources []string `json:"accepted_sources"` // ordered: name, path, url, svg_data
	Description     string   `json:"description"`      // agent-facing summary
	BundledCatalog  string   `json:"bundled_catalog"`  // pointer to the bundled icon catalog tool
}

// skillDeprecation describes a deprecated feature and its canonical replacement.
type skillDeprecation struct {
	Feature     string `json:"feature"`
	Replacement string `json:"replacement"`
	Note        string `json:"note"`
}

// skillPatternCompact is a compact pattern entry (≤ 40 tokens) for default mode.
//
// Optional descriptive fields carry `omitempty` so the same struct can serve
// both list_patterns projections: in `fields=compact` they are left unset and
// drop from the wire; in `fields=full` (or the legacy default) they are
// populated as before. Tests targeting full mode see no shape change.
type skillPatternCompact struct {
	Name                     string   `json:"name"`
	Cells                    string   `json:"cells,omitempty"`
	UseWhen                  string   `json:"use_when,omitempty"`
	NotWhen                  string   `json:"not_when,omitempty"`
	Category                 string   `json:"category"`
	NarrativeRole            []string `json:"narrative_role,omitempty"`
	PairsWith                []string `json:"pairs_with,omitempty"`
	ComposesWith             []string `json:"composes_with,omitempty"`
	RoleOnSlide              []string `json:"role_on_slide,omitempty"`
	DensityClass             string   `json:"density_class,omitempty"`
	AccentWeight             string   `json:"accent_weight,omitempty"`
	SupportsCallout          bool     `json:"supports_callout,omitempty"`
	EstimatedPromptSizeBytes int      `json:"estimated_prompt_size_bytes,omitempty"`
}

// skillPatternFull is a full pattern entry including the hand-authored schema.
type skillPatternFull struct {
	Name                  string                        `json:"name"`
	Description           string                        `json:"description"`
	Cells                 string                        `json:"cells"`
	UseWhen               string                        `json:"use_when"`
	NotWhen               string                        `json:"not_when"`
	SupportsCallout       bool                          `json:"supports_callout"`
	Version               int                           `json:"version"`
	Schema                json.RawMessage               `json:"schema"`
	CalloutSchema         json.RawMessage               `json:"callout_schema,omitempty"`
	TextBudgetGuide       *textcapacity.TextBudgetGuide `json:"text_budget_guide,omitempty"`
	ExampleValues         any                           `json:"example_values,omitempty"`
	RenderingCapabilities *renderingCapabilities        `json:"rendering_capabilities,omitempty"`
	ComposesWith          []string                      `json:"composes_with,omitempty"`
	RoleOnSlide           []string                      `json:"role_on_slide,omitempty"`
}

// renderingCapabilities describes how a pattern renders icons and other visual elements.
type renderingCapabilities struct {
	IconSupport string `json:"icon_support"` // "none", "text_only", "svg_only", "svg_and_text"
}

// skillToolInfo identifies the tool and its version.
type skillToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Built   string `json:"built,omitempty"`
}

// skillTableStyle describes a table style declared by the template.
type skillTableStyle struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// skillTemplateInfo describes a single available template.
type skillTemplateInfo struct {
	Name        string `json:"name"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	LayoutCount int    `json:"layout_count,omitempty"`
	Error       string `json:"error,omitempty"`
	// SHA256 is the content hash of the template file (template.Reader.Hash()).
	// Agents use it as a stable identity / cache key to detect when a template
	// has changed under a stable name. Present in compact+full.
	SHA256 string `json:"sha256,omitempty"`
	// MetadataVersion is the embedded template metadata schema version (e.g.
	// "1.0"). Omitted when the template carries no metadata block.
	MetadataVersion  string            `json:"metadata_version,omitempty"`
	ThemeColors      map[string]string `json:"theme_colors,omitempty"`
	ColorRoles       *skillColorRoles  `json:"color_roles,omitempty"`
	TitleFont        string            `json:"title_font,omitempty"`
	BodyFont         string            `json:"body_font,omitempty"`
	AccentUsageGuide map[string]string `json:"accent_usage_guide,omitempty"` // from template metadata; omitted when unset
	// SemanticAccents maps semantic roles (positive/negative/neutral) to theme
	// accent names. Mirrors TemplateMetadata.SemanticAccents; omitted when unset.
	SemanticAccents map[string]string `json:"semantic_accents,omitempty"`
	// SurfaceTints maps surface roles (subtle/paper/elevated/inverse) to theme
	// color names for tinted backgrounds. Mirrors TemplateMetadata.SurfaceTints.
	SurfaceTints map[string]string `json:"surface_tints,omitempty"`
	// DataPalette is the ordered list of scheme color names for chart series.
	// Mirrors TemplateMetadata.DataPalette; omitted when unset.
	DataPalette []string `json:"data_palette,omitempty"`
	// LayoutHints carries per-layout authoring hints from template metadata
	// (preferred_for, max_bullets, max_chars, deprecated). Omitted when unset.
	LayoutHints        map[string]types.LayoutHint `json:"layout_hints,omitempty"`
	CanonicalLayoutIDs map[string]string           `json:"canonical_layout_ids,omitempty"` // canonical name → concrete layout ID
	// CanonicalCoverage reports, per content-bearing canonical family
	// (title-slide, section-divider, one-content, qa-closing), whether the
	// template provides a layout and which layouts cover it. Present in
	// compact+full so agents can vet a template before authoring against it.
	CanonicalCoverage map[string]skillCanonicalCoverage `json:"canonical_coverage,omitempty"`
	// DerivableLayouts reports which higher-level layouts the engine can produce
	// from the template's base layouts (two-content, full-image, grid patterns),
	// and what is missing when it cannot. Present in compact+full.
	DerivableLayouts []skillDerivableLayout `json:"derivable_layouts,omitempty"`
	LayoutNames      []string               `json:"layout_names,omitempty"`
	LayoutSummaries  []skillLayoutSummary   `json:"layout_summaries,omitempty"` // compact+full: id+name+placeholders
	TableStyles      []skillTableStyle      `json:"table_styles"`
	Layouts          []skillLayoutInfo      `json:"layouts,omitempty"` // only in full mode
}

// skillCanonicalCoverage reports whether a canonical layout family is present in
// a template and names the layouts that cover it.
type skillCanonicalCoverage struct {
	Family  string   `json:"family"`
	Present bool     `json:"present"`
	Layouts []string `json:"layouts,omitempty"`
}

// skillDerivableLayout reports whether a higher-level layout can be derived from
// a template's base layouts, and what is missing when it cannot.
type skillDerivableLayout struct {
	Name    string   `json:"name"`
	Ready   bool     `json:"ready"`
	Missing []string `json:"missing,omitempty"`
}

// skillColorRoles maps design intent to scheme color names for a template.
// Agents use this to pick safe color pairings without manual WCAG checks.
type skillColorRoles struct {
	PrimaryFill   string   `json:"primary_fill"`    // dark accent for headers (white text safe)
	SecondaryFill string   `json:"secondary_fill"`  // second accent for headers (white text safe)
	BodyFill      string   `json:"body_fill"`       // light fill for body/card cells
	BodyText      string   `json:"body_text"`       // dark text on light backgrounds
	WhiteTextSafe []string `json:"white_text_safe"` // all accents passing WCAG AA (≥3.0) against white
}

// skillLayoutSummary is a lightweight layout entry included in compact mode
// so agents can address layouts by ID and gauge placeholder capacity without
// escalating to full mode.
type skillLayoutSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// CanonicalType is the layout's canonical role (e.g. "Title Slide",
	// "One Content", "Section Divider"). Lets agents pick a layout by intent
	// without escalating to full mode. Omitted when unclassified.
	CanonicalType  string                    `json:"canonical_type,omitempty"`
	Placeholders   []skillPlaceholderCompact `json:"placeholders,omitempty"`
	PreviewPNGPath string                    `json:"preview_png_path,omitempty"`
}

// skillPlaceholderCompact is the minimal placeholder info surfaced in compact
// mode — just enough for agents to size content and place it by role.
type skillPlaceholderCompact struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// Role is the canonical, intent-level placeholder role (title, eyebrow,
	// section_number, body, image, chart, …) — refines Type for role-aware
	// placement. Omitted when unclassified.
	Role     string `json:"role,omitempty"`
	MaxChars int    `json:"max_chars"`
}

// skillLayoutInfo describes a single layout (only included in full mode).
type skillLayoutInfo struct {
	Name string   `json:"name"`
	ID   string   `json:"id"`
	Tags []string `json:"tags"`
	// CanonicalType / CanonicalFamily / CanonicalConfidence are the layout's
	// canonical classification (the same taxonomy examine_template reports).
	// Empty CanonicalType means the layout maps to no canonical role.
	CanonicalType       string                 `json:"canonical_type,omitempty"`
	CanonicalFamily     string                 `json:"canonical_family,omitempty"`
	CanonicalConfidence float64                `json:"canonical_confidence,omitempty"`
	Placeholders        []skillPlaceholderInfo `json:"placeholders"`
	Capacity            skillCapacity          `json:"capacity"`
	PreviewPNGPath      string                 `json:"preview_png_path,omitempty"`
}

// skillPlaceholderInfo describes a placeholder within a layout.
type skillPlaceholderInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	// Role / RoleConfidence are the canonical, intent-level placeholder role and
	// the classifier's 0.0–1.0 confidence in it.
	Role           string  `json:"role,omitempty"`
	RoleConfidence float64 `json:"role_confidence,omitempty"`
	MaxChars       int     `json:"max_chars"`
	X              int64   `json:"x_emu"`
	Y              int64   `json:"y_emu"`
	Width          int64   `json:"width_emu"`
	Height         int64   `json:"height_emu"`
	FontFamily     string  `json:"font_family,omitempty"`
	FontSize       int     `json:"font_size_hundredths,omitempty"`
	// FontSizePt is FontSize expressed in points (the font-size evidence behind
	// the font-aware MaxChars estimate). Omitted when unknown.
	FontSizePt float64 `json:"font_size_pt,omitempty"`
	FontColor  string  `json:"font_color,omitempty"`
}

// skillCapacity summarizes a layout's content capacity.
type skillCapacity struct {
	MaxBullets   int  `json:"max_bullets"`
	MaxTextLines int  `json:"max_text_lines"`
	HasImageSlot bool `json:"has_image_slot"`
	HasChartSlot bool `json:"has_chart_slot"`
}

// skillSupportedTypes lists all supported slide, chart, diagram, and grid types.
type skillSupportedTypes struct {
	SlideTypes            []string                   `json:"slide_types"`
	ChartTypes            []string                   `json:"chart_types"`
	DiagramTypes          []string                   `json:"diagram_types"`
	ChartCapabilities     []svggen.ChartCapability   `json:"chart_capabilities"`
	DiagramCapabilities   []svggen.DiagramCapability `json:"diagram_capabilities"`
	GridCellTypes         []string                   `json:"grid_cell_types"`
	ShapeGeometries       []string                   `json:"shape_geometries"`
	DataFormatHints       map[string]skillDataFormat `json:"data_format_hints,omitempty"`
	DataFormatHintsDigest string                     `json:"data_format_hints_digest,omitempty"`
}

// skillDataFormat describes the expected data structure for a chart or diagram type.
type skillDataFormat struct {
	RequiredKeys []string `json:"required_keys"`
	OptionalKeys []string `json:"optional_keys,omitempty"`
	Description  string   `json:"description"`
}

// runSkillInfo implements the skill-info subcommand.
func runSkillInfo() error {
	// Suppress non-essential info/warn logging so stdout stays clean JSON.
	// Template analysis (synthesis, placeholder resolution) emits slog.Info
	// that would pollute machine-readable output when stderr is merged.
	prevLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	})))
	defer slog.SetDefault(prevLogger)

	fs := flag.NewFlagSet("skill-info", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Analyze a single template by name (optional)")
	mode := fs.String("mode", "compact", "Output mode: list, compact, or full (full emits patterns_full with JSON schemas; ~39K tokens)")
	// Deprecated: --mode=full now always includes full pattern schemas. The flag
	// is accepted for backward compatibility but has no effect; agents should
	// drop it. Kept here so existing callers don't break with an "unknown flag"
	// error on upgrade.
	includeFullSchemas := fs.Bool("include-full-schemas", false, "(Deprecated; --mode=full already includes full pattern schemas)")
	noPreview := fs.Bool("no-preview", false, "Read-only discovery: skip layout-preview PNG generation so no cache files are written (preview_png_path is then omitted). Use when gathering template context in a read-only planning context.")
	jsonFlag := fs.Bool("json", true, "Output as JSON (default: true)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx skill-info [options]\n\n")
		fmt.Fprintf(os.Stderr, "Show template capabilities for Claude Code skill integration.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Validate mode
	switch *mode {
	case "list", "compact", "full":
		// valid
	default:
		return fmt.Errorf("invalid mode %q: must be list, compact, or full", *mode)
	}

	// Discover templates using the same search path as generate
	var templateNames []string
	if *templateName != "" {
		templateNames = []string{*templateName}
	} else {
		templateNames = listAvailableTemplates(*templatesDir)
		sort.Strings(templateNames)
	}

	// Resolve each template name to a path via the search path
	var templatePaths []string
	for _, name := range templateNames {
		path, cleanup, err := resolveTemplatePath(name, *templatesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not resolve template %q: %v\n", name, err)
			continue
		}
		defer cleanup()
		templatePaths = append(templatePaths, path)
	}

	// Build template cache
	cache := template.NewMemoryCache(24 * time.Hour)

	// Analyze each template
	skillOpts := skillInfoOptions{NoPreview: *noPreview}
	var templates []skillTemplateInfo
	for _, path := range templatePaths {
		info, err := analyzeTemplateForSkillInfoOpts(path, cache, *mode, skillOpts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to analyze %s: %v\n", filepath.Base(path), err)
			continue
		}
		templates = append(templates, info)
	}

	// Build pattern entries. In "full" mode we always emit patterns_full with
	// full JSON schemas — no silent downgrade. The legacy --include-full-schemas
	// flag is a deprecated no-op and is intentionally not consulted here.
	_ = includeFullSchemas
	var patternsCompact []skillPatternCompact
	var patternsFull []skillPatternFull
	if *mode != "list" {
		patternsCompact, patternsFull = buildPatternEntries(*mode)
	}

	// Build output
	output := skillInfo{
		Tool: skillToolInfo{
			Name:    "json2pptx",
			Version: Version,
			Commit:  CommitSHA,
			Built:   BuildTime,
		},
		Templates:       templates,
		SupportedTypes:  buildSupportedTypes(),
		PatternsCompact: patternsCompact,
		PatternsFull:    patternsFull,
		Compose:         buildComposeEntry(),
		InputFormats:    []string{"json"},
		OutputFormats:   []string{"pptx"},
		IconPolicy:      buildIconPolicy(),
		Deprecations:    buildDeprecations(),
		SideEffects:     buildSkillSideEffects(*noPreview, "--no-preview"),
	}

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// Plain text fallback
	printSkillInfoText(output, *mode)
	return nil
}

// analyzeTemplateForSkillInfo analyzes a single template and returns skill
// info. Layout-preview generation (which writes PNG cache files) is enabled;
// for a side-effect-free read-only analysis use analyzeTemplateForSkillInfoOpts
// with skillInfoOptions{NoPreview: true}.
func analyzeTemplateForSkillInfo(templatePath string, cache types.TemplateCache, mode string) (skillTemplateInfo, error) {
	return analyzeTemplateForSkillInfoOpts(templatePath, cache, mode, skillInfoOptions{})
}

// analyzeTemplateForSkillInfoOpts analyzes a single template and returns skill
// info, honoring opts. When opts.NoPreview is set, layout-preview PNG
// generation is skipped entirely so the call writes no files; preview_png_path
// fields are then omitted from the result.
func analyzeTemplateForSkillInfoOpts(templatePath string, cache types.TemplateCache, mode string, opts skillInfoOptions) (skillTemplateInfo, error) {
	analysis, err := getOrAnalyzeTemplate(templatePath, cache)
	if err != nil {
		return skillTemplateInfo{}, err
	}

	name := strings.TrimSuffix(filepath.Base(templatePath), ".pptx")
	info := skillTemplateInfo{
		Name:        name,
		AspectRatio: analysis.AspectRatio,
		LayoutCount: len(analysis.Layouts),
	}

	// Always include table_styles (empty array, never null).
	reader, err := template.OpenTemplate(templatePath)
	if err == nil {
		entries := reader.TableStyles()
		tblStyles := make([]skillTableStyle, len(entries))
		for i, e := range entries {
			tblStyles[i] = skillTableStyle{ID: e.ID, Name: e.Name}
		}
		info.TableStyles = tblStyles
		_ = reader.Close()
	}
	if info.TableStyles == nil {
		info.TableStyles = []skillTableStyle{}
	}

	if mode == "list" {
		return info, nil
	}

	// compact and full: include theme colors and layout names
	info.ThemeColors = make(map[string]string, len(analysis.Theme.Colors))
	for _, c := range analysis.Theme.Colors {
		info.ThemeColors[c.Name] = c.RGB
	}
	info.TitleFont = analysis.Theme.TitleFont
	info.BodyFont = analysis.Theme.BodyFont
	info.ColorRoles = buildColorRoles(analysis.Theme.Colors)

	// Stable content identity for change detection / cache keys.
	info.SHA256 = analysis.Hash

	// Surface semantic palette metadata + per-layout hints from the embedded
	// template metadata when present (all omitempty, so metadata-less templates
	// stay slim).
	if md := analysis.Metadata; md != nil {
		info.MetadataVersion = md.Version
		if len(md.AccentUsageGuide) > 0 {
			info.AccentUsageGuide = md.AccentUsageGuide
		}
		if len(md.SemanticAccents) > 0 {
			info.SemanticAccents = md.SemanticAccents
		}
		if len(md.SurfaceTints) > 0 {
			info.SurfaceTints = md.SurfaceTints
		}
		if len(md.DataPalette) > 0 {
			info.DataPalette = md.DataPalette
		}
		if len(md.LayoutHints) > 0 {
			info.LayoutHints = md.LayoutHints
		}
	}

	// Canonical family coverage + derivable-layout readiness let agents vet a
	// template's planning surface without escalating to examine_template.
	info.CanonicalCoverage = buildSkillCanonicalCoverage(analysis.Layouts)
	info.DerivableLayouts = buildSkillDerivableLayouts(analysis.Layouts)

	// Generate layout preview PNGs (best-effort, non-blocking). Skipped in
	// read-only mode so discovery writes no cache files; downstream code already
	// treats a nil previews result as "no previews available".
	var previews *layoutpreview.Result
	if !opts.NoPreview {
		previews, _ = layoutpreview.Generate(templatePath, analysis, nil)
	}

	layoutNames := make([]string, len(analysis.Layouts))
	layoutSummaries := make([]skillLayoutSummary, len(analysis.Layouts))
	for i, l := range analysis.Layouts {
		layoutNames[i] = l.Name
		summary := skillLayoutSummary{
			ID:            l.ID,
			Name:          l.Name,
			CanonicalType: string(template.EffectiveCanonicalType(&analysis.Layouts[i])),
		}

		// Compact placeholder entries (id + type + role + max_chars only)
		phs := make([]skillPlaceholderCompact, 0, len(l.Placeholders))
		for _, ph := range l.Placeholders {
			if ph.Type == types.PlaceholderOther {
				continue
			}
			phs = append(phs, skillPlaceholderCompact{
				ID:       ph.ID,
				Type:     string(ph.Type),
				Role:     string(ph.Role),
				MaxChars: ph.MaxChars,
			})
		}
		if len(phs) > 0 {
			summary.Placeholders = phs
		}

		if previews != nil {
			if p, ok := previews.Paths[l.ID]; ok {
				summary.PreviewPNGPath = p
			}
		}
		layoutSummaries[i] = summary
	}
	info.CanonicalLayoutIDs = layout.ResolveAllCanonicalLayouts(analysis.Layouts)
	info.LayoutNames = layoutNames
	info.LayoutSummaries = layoutSummaries

	if mode == "full" {
		info.Layouts = buildFullLayoutInfos(analysis.Layouts, previews)
	}

	return info, nil
}

// buildSkillSideEffects describes the disk side effects of a discovery call.
// noPreview reflects whether read-only / no-preview mode was active; disableWith
// names the surface-specific opt-out (e.g. "read_only=true" or "--no-preview").
func buildSkillSideEffects(noPreview bool, disableWith string) *skillSideEffects {
	return &skillSideEffects{
		PreviewCacheWrites: !noPreview,
		ReadOnly:           noPreview,
		PreviewCacheDir:    layoutpreview.DefaultCacheDir(),
		DisableWith:        disableWith,
	}
}

// buildFullLayoutInfos constructs detailed layout info with placeholders and previews.
func buildFullLayoutInfos(layouts []types.LayoutMetadata, previews *layoutpreview.Result) []skillLayoutInfo {
	result := make([]skillLayoutInfo, len(layouts))
	for i, l := range layouts {
		phs := make([]skillPlaceholderInfo, 0, len(l.Placeholders))
		var sectionNumberPH *skillPlaceholderInfo
		for _, ph := range l.Placeholders {
			if ph.Type == types.PlaceholderOther {
				continue
			}
			pi := skillPlaceholderInfo{
				ID:             ph.ID,
				Type:           string(ph.Type),
				Role:           string(ph.Role),
				RoleConfidence: ph.RoleConfidence,
				MaxChars:       ph.MaxChars,
				X:              ph.Bounds.X,
				Y:              ph.Bounds.Y,
				Width:          ph.Bounds.Width,
				Height:         ph.Bounds.Height,
				FontFamily:     ph.FontFamily,
				FontSize:       ph.FontSize,
				FontSizePt:     fontHundredthsToPt(ph.FontSize),
				FontColor:      ph.FontColor,
			}
			phs = append(phs, pi)
			if strings.EqualFold(ph.ID, "Section Number") {
				alias := pi
				alias.ID = "section_number"
				alias.Type = "section_number"
				sectionNumberPH = &alias
			}
		}
		if sectionNumberPH != nil {
			phs = append(phs, *sectionNumberPH)
		}
		tags := l.Tags
		if tags == nil {
			tags = []string{}
		}
		ct := template.EffectiveCanonicalType(&layouts[i])
		li := skillLayoutInfo{
			Name:                l.Name,
			ID:                  l.ID,
			Tags:                tags,
			CanonicalType:       string(ct),
			CanonicalFamily:     string(ct.Family()),
			CanonicalConfidence: l.CanonicalConfidence,
			Placeholders:        phs,
			Capacity: skillCapacity{
				MaxBullets:   l.Capacity.MaxBullets,
				MaxTextLines: l.Capacity.MaxTextLines,
				HasImageSlot: l.Capacity.HasImageSlot,
				HasChartSlot: l.Capacity.HasChartSlot,
			},
		}
		if previews != nil {
			if p, ok := previews.Paths[l.ID]; ok {
				li.PreviewPNGPath = p
			}
		}
		result[i] = li
	}
	return result
}

// buildColorRoles derives color_roles from a template's theme colors.
// It identifies which accents pass WCAG AA large-text contrast (≥3.0) against
// white, then picks the first two as primary/secondary fill.
func buildColorRoles(colors []types.ThemeColor) *skillColorRoles {
	white := svggen.MustParseColor("#FFFFFF")

	// accentOrder is the order we check accents for white-text safety.
	accentOrder := []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"}

	var safe []string
	for _, name := range accentOrder {
		hex := findColorHex(colors, name)
		if hex == "" {
			continue
		}
		c, err := svggen.ParseColor(hex)
		if err != nil {
			continue
		}
		if c.ContrastWith(white) >= svggen.WCAGAALarge {
			safe = append(safe, name)
		}
	}

	roles := &skillColorRoles{
		PrimaryFill:   "accent1",
		SecondaryFill: "accent2",
		BodyFill:      "lt2",
		BodyText:      "dk1",
		WhiteTextSafe: safe,
	}

	// Override primary/secondary with the first two white-text-safe accents.
	if len(safe) >= 1 {
		roles.PrimaryFill = safe[0]
	}
	if len(safe) >= 2 {
		roles.SecondaryFill = safe[1]
	}

	return roles
}

// findColorHex returns the hex value for a named theme color, or "".
func findColorHex(colors []types.ThemeColor, name string) string {
	for _, c := range colors {
		if c.Name == name {
			return c.RGB
		}
	}
	return ""
}

// contentCanonicalFamilies enumerates the four content-bearing canonical layout
// families every template is expected to provide. Utility families (Blank,
// Blank+Title) are intentionally excluded — they are not planning targets.
var contentCanonicalFamilies = []types.CanonicalLayoutFamily{
	types.LayoutFamilyTitleSlide,
	types.LayoutFamilySectionDivider,
	types.LayoutFamilyOneContent,
	types.LayoutFamilyQAClosing,
}

// buildSkillCanonicalCoverage reports, for each content-bearing canonical family,
// whether the template provides a covering layout and which layouts cover it.
// Absent families are still listed with present=false so agents see the full
// coverage matrix (mirrors examine_template's canonical_coverage).
func buildSkillCanonicalCoverage(layouts []types.LayoutMetadata) map[string]skillCanonicalCoverage {
	byFamily := template.CanonicalFamilyCoverage(layouts)
	out := make(map[string]skillCanonicalCoverage, len(contentCanonicalFamilies))
	for _, fam := range contentCanonicalFamilies {
		names := byFamily[fam]
		out[string(fam)] = skillCanonicalCoverage{
			Family:  string(fam),
			Present: len(names) > 0,
			Layouts: names,
		}
	}
	return out
}

// buildSkillDerivableLayouts projects template.DerivableLayouts into the
// skill-info wire shape.
func buildSkillDerivableLayouts(layouts []types.LayoutMetadata) []skillDerivableLayout {
	dls := template.DerivableLayouts(layouts)
	out := make([]skillDerivableLayout, len(dls))
	for i, d := range dls {
		out[i] = skillDerivableLayout{Name: d.Name, Ready: d.Ready, Missing: d.Missing}
	}
	return out
}

// fontHundredthsToPt converts a font size in hundredths of a point (the OOXML /
// PlaceholderInfo.FontSize unit) to points. Returns 0 when unknown.
func fontHundredthsToPt(hundredths int) float64 {
	if hundredths <= 0 {
		return 0
	}
	return float64(hundredths) / 100.0
}

// readyDiagramTypeNames returns the names of diagram types with Status "ready".
func readyDiagramTypeNames() []string {
	caps := svggen.DiagramCapabilitiesReady()
	names := make([]string, len(caps))
	for i, c := range caps {
		names[i] = c.Type
	}
	return names
}

// buildSupportedTypes returns the hardcoded lists of supported types.
func buildSupportedTypes() skillSupportedTypes {
	return skillSupportedTypes{
		SlideTypes: []string{
			"title",
			"content",
			"two-column",
			"image",
			"chart",
			"comparison",
			"blank",
			"section",
			"diagram",
		},
		ChartTypes: []string{
			"bar",
			"line",
			"pie",
			"donut",
			"area",
			"radar",
			"scatter",
			"stacked_bar",
			"bubble",
			"stacked_area",
			"grouped_bar",
			"waterfall",
			"funnel",
			"gauge",
			"treemap",
		},
		DiagramTypes:        readyDiagramTypeNames(),
		ChartCapabilities:   svggen.ChartCapabilities(),
		DiagramCapabilities: svggen.DiagramCapabilitiesReady(),
		GridCellTypes:       []string{"shape", "table", "icon", "image", "diagram", "composite"},
		ShapeGeometries:     buildShapeGeometries(),
		DataFormatHints:     buildDataFormatHints(),
	}
}

// buildComposeEntry returns the compose envelope skill-info descriptor with
// the capability caps drawn from composeCapabilities() and two hand-authored
// examples (vertical and horizontal) so an agent can copy-adapt without
// reading the raw input schema. The caps and the examples are kept in one
// place to make compose discoverable from a single skill-info read.
func buildComposeEntry() *skillComposeEntry {
	caps := composeCapabilities()
	verticalExample := json.RawMessage(`{
  "type": "blank",
  "compose": {
    "direction": "vertical",
    "gap": 12,
    "segments": [
      {"pattern": {"name": "stylish-panels", "values": {"panels": [{"title": "Pillar A"}, {"title": "Pillar B"}, {"title": "Pillar C"}]}}, "size_pct": 65},
      {"pattern": {"name": "pull-quote", "values": {"quote": "Strategy is choice.", "attribution": "PM lead"}}, "size_pct": 35}
    ]
  }
}`)
	horizontalExample := json.RawMessage(`{
  "type": "blank",
  "compose": {
    "direction": "horizontal",
    "gap": 12,
    "smart_compose": true,
    "segments": [
      {"pattern": {"name": "kpi-3up", "values": {"kpis": [{"value": "42%", "label": "Win rate"}, {"value": "$1.2M", "label": "ARR"}, {"value": "12", "label": "Logos"}]}}},
      {"pattern": {"name": "process-flow", "values": {"steps": [{"label": "Discover"}, {"label": "Pilot"}, {"label": "Scale"}]}}}
    ]
  }
}`)
	diagramSegmentExample := json.RawMessage(`{
  "type": "blank",
  "compose": {
    "direction": "horizontal",
    "gap": 12,
    "segments": [
      {"size_pct": 50, "pattern": {"name": "pyramid", "values": {"tiers": ["Strategy", "Tactics", "Operations"]}}},
      {"size_pct": 50, "diagram": {"type": "process_flow", "data": {"steps": ["Plan", "Build", "Ship"]}}}
    ]
  }
}`)
	return &skillComposeEntry{
		Description:             "Compose envelope: stack two or more sibling patterns on one slide. Each segment hosts a leaf pattern, a nested compose, or a standalone svggen diagram. Direction picks vertical stack or horizontal side-by-side; size_pct controls share (defaults to equal). Use recommend_visual to discover high-affinity pattern pairs (Category=='compose'). Optional banner (above) and callout (below) decoration bands attach to the envelope without consuming a segment slot.",
		Directions:              append([]string(nil), caps.Directions...),
		MaxSegments:             caps.MaxSegments,
		MaxNestingDepth:         caps.MaxNestingDepth,
		MaxLeafPatterns:         caps.MaxLeafPatterns,
		SmartCompose:            caps.SupportsSmartCompose,
		NestedCompose:           caps.SupportsNestedCompose,
		SupportsBanner:          true,
		SupportsCallout:         true,
		SupportsDiagramSegments: caps.SupportsDiagramSegments,
		Examples: []skillComposeExample{
			{
				Title:       "Vertical: panels above a pull-quote",
				Description: "Stylish-panels (65% height) sit above a pull-quote (35%). Common for executive recap slides.",
				JSON:        verticalExample,
			},
			{
				Title:       "Horizontal: KPI strip beside a process flow",
				Description: "kpi-3up on the left, process-flow on the right, sized by content density.",
				JSON:        horizontalExample,
			},
			{
				Title:       "Horizontal: native pattern + svggen diagram",
				Description: "Pyramid (native pattern) on the left, svggen process_flow diagram on the right — diagram segment is a third XOR alternative to pattern/compose, so the native pattern is not flattened through a single-cell grid.",
				JSON:        diagramSegmentExample,
			},
		},
	}
}

// buildDeprecations returns the list of deprecated features with their replacements.
// buildIconPolicy returns the deck-wide icon contract surfaced to skill-info
// readers. Mirrors the no-emoji rule documented in
// skills/generate-deck/SKILL.md (Icon Names) and docs/INPUT_FORMAT.md, and
// the validators in internal/patterns/{cardgrid,iconrow,herodetail}.go.
func buildIconPolicy() *skillIconPolicy {
	return &skillIconPolicy{
		NoEmoji:         true,
		AcceptedSources: []string{"name", "path", "url", "svg_data"},
		Description: "Emoji codepoints are rejected anywhere in deck JSON " +
			"(icon fields, pattern values, shape text, titles, bullets). " +
			"Use a bundled SVG icon name (preferred) or supply a user icon " +
			"via path / url / svg_data. Exactly one of " +
			"name|path|url|svg_data must be set per IconInput.",
		BundledCatalog: "list_icons (MCP) or `json2pptx icons list` (CLI)",
	}
}

func buildDeprecations() []skillDeprecation {
	return []skillDeprecation{
		{
			Feature:     "value (untyped content field)",
			Replacement: "Use typed fields: text_value, bullets_value, table_value, chart_value, diagram_value, image_value, body_and_bullets_value, bullet_groups_value",
			Note:        "The legacy \"value\" field (json.RawMessage) is still accepted for backward compatibility but should not be used in new decks. Typed fields provide schema validation and clearer intent.",
		},
		{
			Feature:     "Raw template placeholder names (e.g. \"Title 1\", \"Content Placeholder 2\")",
			Replacement: "Use portable placeholder IDs: title, subtitle, body, body_2",
			Note:        "Raw OOXML placeholder names are template-specific and resolved via semantic fallback. Portable IDs work across all templates and resolve at the exact tier.",
		},
	}
}

// buildShapeGeometries returns the sorted list of all known preset geometry names.
func buildShapeGeometries() []string {
	geoms := pptx.KnownGeometries()
	names := make([]string, len(geoms))
	for i, g := range geoms {
		names[i] = string(g)
	}
	sort.Strings(names)
	return names
}

// buildDataFormatHints returns the expected data format for each chart and diagram type.
func buildDataFormatHints() map[string]skillDataFormat {
	return map[string]skillDataFormat{
		// --- Charts ---
		"bar": {
			RequiredKeys: []string{"categories", "series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "categories: string array; series: [{name, values: number[]}]",
		},
		"line": {
			RequiredKeys: []string{"series"},
			OptionalKeys: []string{"categories", "colors", "x_label", "y_label"},
			Description:  "series: [{name, values: number[]}]; categories required unless series contain time_strings or time_values",
		},
		"pie": {
			RequiredKeys: []string{"values"},
			OptionalKeys: []string{"categories", "colors"},
			Description:  "values: number[]; categories: string[] for slice labels",
		},
		"donut": {
			RequiredKeys: []string{"values"},
			OptionalKeys: []string{"categories", "colors"},
			Description:  "values: number[]; categories: string[] for slice labels",
		},
		"area": {
			RequiredKeys: []string{"categories", "series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "categories: string array; series: [{name, values: number[]}]",
		},
		"radar": {
			RequiredKeys: []string{"categories", "series"},
			OptionalKeys: []string{"colors"},
			Description:  "categories: string[] (min 3 axes); series: [{name, values: number[]}]",
		},
		"scatter": {
			RequiredKeys: []string{"series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "series: [{name, points: [{x, y, label?}]}]",
		},
		"stacked_bar": {
			RequiredKeys: []string{"categories", "series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "categories: string[]; series: [{name, values: number[]}]",
		},
		"bubble": {
			RequiredKeys: []string{"series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "series: [{name, points: [{x, y, size, label?}]}]",
		},
		"stacked_area": {
			RequiredKeys: []string{"categories", "series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "categories: string[]; series: [{name, values: number[]}]",
		},
		"grouped_bar": {
			RequiredKeys: []string{"categories", "series"},
			OptionalKeys: []string{"colors", "x_label", "y_label"},
			Description:  "categories: string[]; series: [{name, values: number[]}] (min 2 series)",
		},
		"waterfall": {
			RequiredKeys: []string{"points"},
			OptionalKeys: []string{"colors", "footnote"},
			Description:  "points: [{label, value, type: \"increase\"|\"decrease\"|\"total\"}]",
		},
		"funnel": {
			RequiredKeys: []string{"values"},
			OptionalKeys: []string{"categories", "neck_width", "gap", "show_percentage"},
			Description:  "values: [{label, value}] or number[] with categories for labels",
		},
		"gauge": {
			RequiredKeys: []string{"value"},
			OptionalKeys: []string{"min", "max", "thresholds", "label", "unit"},
			Description:  "value: number; min/max: number; thresholds: [{value, color, label}]",
		},
		"treemap": {
			RequiredKeys: []string{"nodes"},
			OptionalKeys: []string{"padding", "corner_radius"},
			Description:  "nodes: [{label, value, children?, color?}] (alias: items or values)",
		},
		// --- Diagrams ---
		"timeline": {
			RequiredKeys: []string{"events"},
			OptionalKeys: []string{"milestones", "show_today", "time_unit"},
			Description:  "events: [{label, start_date, end_date}] (alias: activities); milestones: [{label, date}]",
		},
		"process_flow": {
			RequiredKeys: []string{"steps"},
			OptionalKeys: []string{"connections", "direction"},
			Description:  "steps: [{id, label, type?, color?}]; connections: [{from, to, label?}]; direction: \"horizontal\"|\"vertical\"",
		},
		"pyramid": {
			RequiredKeys: []string{"levels"},
			OptionalKeys: []string{"gap", "top_width_ratio"},
			Description:  "levels: [{label, description?, color?}]",
		},
		"venn": {
			RequiredKeys: []string{"circles"},
			OptionalKeys: []string{"circle_opacity", "overlap_ratio"},
			Description:  "circles: [{label, items: string[]}] (min 2; alias: sets)",
		},
		"swot": {
			RequiredKeys: []string{"strengths", "weaknesses", "opportunities", "threats"},
			OptionalKeys: []string{"footnote"},
			Description:  "strengths/weaknesses/opportunities/threats: string[] for each quadrant",
		},
		"org_chart": {
			RequiredKeys: []string{"root"},
			OptionalKeys: []string{"node_width", "node_height"},
			Description:  "root: {name, title, children?: [{name, title, children?}...]}",
		},
		"gantt": {
			RequiredKeys: []string{"tasks"},
			OptionalKeys: []string{"milestones", "time_unit", "show_progress", "footnote"},
			Description:  "tasks: [{id, label, start_date, end_date, progress?, group?}]; milestones: [{id, label, date}]",
		},
		"matrix_2x2": {
			RequiredKeys: []string{},
			OptionalKeys: []string{"points", "quadrants", "x_label", "y_label", "quadrant_labels"},
			Description:  "points: [{label, x, y, size?, color?}] or quadrants: [{position, title, items}]; x_label/y_label for axes",
		},
		"porters_five_forces": {
			RequiredKeys: []string{},
			OptionalKeys: []string{"forces", "industry_name", "rivalry", "new_entrants", "substitutes", "suppliers", "buyers"},
			Description:  "forces: [{type, label, intensity, description?}] or map of force-type keys; industry_name: string",
		},
		"house_diagram": {
			RequiredKeys: []string{},
			OptionalKeys: []string{"roof", "sections", "floors", "foundation", "footnote"},
			Description:  "roof: string or {label, color}; sections: [{label, items?, color?}]; foundation: string or {label, color}",
		},
		"business_model_canvas": {
			RequiredKeys: []string{"key_partners", "key_activities", "key_resources", "value_propositions", "customer_relations", "channels", "customer_segments", "cost_structure", "revenue_streams"},
			OptionalKeys: []string{},
			Description:  "9 BMC sections, each a string[] (e.g., key_partners: [\"Partner A\", \"Partner B\"])",
		},
		"value_chain": {
			RequiredKeys: []string{},
			OptionalKeys: []string{"primary", "support", "margin_label", "show_arrows"},
			Description:  "primary: [{label, description?, items?}]; support: [{label, description?, items?}]",
		},
		"nine_box_talent": {
			RequiredKeys: []string{},
			OptionalKeys: []string{"employees", "cells", "x_label", "y_label"},
			Description:  "employees: [{name, performance: 1-3, potential: 1-3}] or cells: [{position, items}]",
		},
		"kpi_dashboard": {
			RequiredKeys: []string{"metrics"},
			OptionalKeys: []string{"gap", "max_columns"},
			Description:  "metrics: [{label, value, unit?, change?, trend?}] (alias: kpis)",
		},
		"heatmap": {
			RequiredKeys: []string{"values"},
			OptionalKeys: []string{"row_labels", "col_labels", "color_scale"},
			Description:  "values: number[][] (2D array); row_labels/col_labels: string[]",
		},
		"fishbone": {
			RequiredKeys: []string{"effect"},
			OptionalKeys: []string{"categories"},
			Description:  "effect: string (problem label); categories: [{name, causes: string[]}]",
		},
		"pestel": {
			RequiredKeys: []string{},
			OptionalKeys: []string{"segments", "political", "economic", "social", "technological", "environmental", "legal"},
			Description:  "segments: [{name, items: string[]}] or individual keys (political, economic, etc.): string[]",
		},
		"panel_layout": {
			RequiredKeys: []string{"panels"},
			OptionalKeys: []string{"layout", "gap", "icon_size"},
			Description:  "panels: [{title, body, icon?, color?}]; layout: \"columns\"|\"rows\"|\"stat_cards\"|\"stylish_panels\"",
		},
	}
}

// computeDataFormatHintsDigest returns a stable SHA-256 hex digest of the
// canonical JSON encoding of the data format hints map. The digest is stable
// across runs because json.Marshal sorts map keys deterministically.
func computeDataFormatHintsDigest(hints map[string]skillDataFormat) string {
	b, err := json.Marshal(hints)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// buildPatternEntries builds compact (always) and full (mode=full only) pattern
// entries from the default pattern registry.
func buildPatternEntries(mode string) ([]skillPatternCompact, []skillPatternFull) {
	reg := patterns.Default()
	all := reg.List() // sorted by name

	compact := make([]skillPatternCompact, len(all))
	for i, p := range all {
		cells := p.CellsHint()
		var sizeBytes int
		if ex, ok := p.(patterns.Exemplar); ok {
			sizeBytes, _ = patterns.CanonicalSizeBytes(p, ex.ExemplarValues())
		}
		supportsCallout := false
		if cs, ok := p.(patterns.CalloutSupport); ok {
			supportsCallout = cs.SupportsCallout()
		}
		tax := p.Taxonomy()
		compact[i] = skillPatternCompact{
			Name:                     p.Name(),
			Cells:                    cells,
			UseWhen:                  p.UseWhen(),
			NotWhen:                  p.NotWhen(),
			Category:                 tax.Category,
			NarrativeRole:            tax.NarrativeRole,
			PairsWith:                tax.PairsWith,
			ComposesWith:             tax.ComposesWith,
			RoleOnSlide:              tax.RoleOnSlide,
			DensityClass:             tax.DensityClass,
			AccentWeight:             tax.AccentWeight,
			SupportsCallout:          supportsCallout,
			EstimatedPromptSizeBytes: sizeBytes,
		}
	}

	if mode != "full" {
		return compact, nil
	}

	full := make([]skillPatternFull, len(all))
	for i, p := range all {
		schemaJSON := patterns.SchemaJSON(p)
		full[i] = skillPatternFull{
			Name:            p.Name(),
			Description:     p.Description(),
			Cells:           compact[i].Cells,
			UseWhen:         p.UseWhen(),
			NotWhen:         p.NotWhen(),
			SupportsCallout: compact[i].SupportsCallout,
			Version:         p.Version(),
			Schema:          schemaJSON,
			ComposesWith:    compact[i].ComposesWith,
			RoleOnSlide:     compact[i].RoleOnSlide,
		}
		if compact[i].SupportsCallout {
			full[i].CalloutSchema = patternCalloutSchemaJSON()
		}
	}
	return compact, full
}

// printSkillInfoText outputs skill info as human-readable text.
func printSkillInfoText(info skillInfo, mode string) {
	fmt.Printf("Tool: %s %s\n", info.Tool.Name, info.Tool.Version)
	fmt.Printf("Input Formats: %s\n", strings.Join(info.InputFormats, ", "))
	fmt.Printf("Output Formats: %s\n", strings.Join(info.OutputFormats, ", "))
	fmt.Println()

	fmt.Printf("Templates (%d):\n", len(info.Templates))
	for _, t := range info.Templates {
		if mode == "list" {
			fmt.Printf("  - %s\n", t.Name)
		} else {
			fmt.Printf("  - %s (%s, %d layouts)\n", t.Name, t.AspectRatio, t.LayoutCount)
		}
	}
	fmt.Println()

	fmt.Printf("Slide Types: %s\n", strings.Join(info.SupportedTypes.SlideTypes, ", "))
	fmt.Printf("Chart Types: %s\n", strings.Join(info.SupportedTypes.ChartTypes, ", "))
	fmt.Printf("Diagram Types: %s\n", strings.Join(info.SupportedTypes.DiagramTypes, ", "))
	if info.IconPolicy != nil {
		fmt.Println()
		fmt.Println("Icon Policy:")
		fmt.Printf("  No Emoji: %t\n", info.IconPolicy.NoEmoji)
		fmt.Printf("  Accepted Sources: %s\n", strings.Join(info.IconPolicy.AcceptedSources, ", "))
		fmt.Printf("  Bundled Catalog: %s\n", info.IconPolicy.BundledCatalog)
		fmt.Printf("  %s\n", info.IconPolicy.Description)
	}
}
