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

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// skillInfo is the top-level JSON output for the skill-info subcommand.
type skillInfo struct {
	Tool             skillToolInfo          `json:"tool"`
	Templates        []skillTemplateInfo    `json:"templates"`
	SupportedTypes   skillSupportedTypes    `json:"supported_types"`
	PatternsCompact  []skillPatternCompact  `json:"patterns_compact,omitempty"`
	PatternsFull     []skillPatternFull     `json:"patterns_full,omitempty"`
	InputFormats     []string               `json:"input_formats"`
	OutputFormats    []string               `json:"output_formats"`
}

// skillPatternCompact is a compact pattern entry (≤ 40 tokens) for default mode.
type skillPatternCompact struct {
	Name                   string `json:"name"`
	Cells                  string `json:"cells"`
	UseWhen                string `json:"use_when"`
	SupportsCallout        bool   `json:"supports_callout"`
	EstimatedPromptSizeBytes int  `json:"estimated_prompt_size_bytes"`
}

// skillPatternFull is a full pattern entry including the hand-authored schema.
type skillPatternFull struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Cells           string          `json:"cells"`
	UseWhen         string          `json:"use_when"`
	SupportsCallout bool            `json:"supports_callout"`
	Version         int             `json:"version"`
	Schema          json.RawMessage `json:"schema"`
	CalloutSchema   json.RawMessage `json:"callout_schema,omitempty"`
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
	Name         string                   `json:"name"`
	AspectRatio  string                   `json:"aspect_ratio"`
	LayoutCount  int                      `json:"layout_count"`
	ThemeColors  map[string]string        `json:"theme_colors,omitempty"`
	ColorRoles   *skillColorRoles         `json:"color_roles,omitempty"`
	TitleFont    string                   `json:"title_font,omitempty"`
	BodyFont     string                   `json:"body_font,omitempty"`
	LayoutNames  []string                 `json:"layout_names,omitempty"`
	TableStyles  []skillTableStyle        `json:"table_styles"`
	Layouts      []skillLayoutInfo        `json:"layouts,omitempty"` // only in full mode
}

// skillColorRoles maps design intent to scheme color names for a template.
// Agents use this to pick safe color pairings without manual WCAG checks.
type skillColorRoles struct {
	PrimaryFill   string   `json:"primary_fill"`        // dark accent for headers (white text safe)
	SecondaryFill string   `json:"secondary_fill"`      // second accent for headers (white text safe)
	BodyFill      string   `json:"body_fill"`           // light fill for body/card cells
	BodyText      string   `json:"body_text"`           // dark text on light backgrounds
	WhiteTextSafe []string `json:"white_text_safe"`     // all accents passing WCAG AA (≥3.0) against white
}

// skillLayoutInfo describes a single layout (only included in full mode).
type skillLayoutInfo struct {
	Name         string                   `json:"name"`
	ID           string                   `json:"id"`
	Tags         []string                 `json:"tags"`
	Placeholders []skillPlaceholderInfo   `json:"placeholders"`
	Capacity     skillCapacity            `json:"capacity"`
}

// skillPlaceholderInfo describes a placeholder within a layout.
type skillPlaceholderInfo struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	MaxChars int    `json:"max_chars"`
	Width    int64  `json:"width_emu"`
	Height   int64  `json:"height_emu"`
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
	SlideTypes           []string                    `json:"slide_types"`
	ChartTypes           []string                    `json:"chart_types"`
	DiagramTypes         []string                    `json:"diagram_types"`
	GridCellTypes        []string                    `json:"grid_cell_types"`
	ShapeGeometries      []string                    `json:"shape_geometries"`
	DataFormatHints      map[string]skillDataFormat  `json:"data_format_hints,omitempty"`
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
	fs := flag.NewFlagSet("skill-info", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	templateName := fs.String("template", "", "Analyze a single template by name (optional)")
	mode := fs.String("mode", "compact", "Output mode: list, compact, or full")
	includeFullSchemas := fs.Bool("include-full-schemas", false, "Include full JSON schemas for all patterns (large; ~39K tokens)")
	jsonFlag := fs.Bool("json", true, "Output as JSON (default: true)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx skill-info [options]\n\n")
		fmt.Fprintf(os.Stderr, "Show template capabilities for Claude Code skill integration.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
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
	var templates []skillTemplateInfo
	for _, path := range templatePaths {
		info, err := analyzeTemplateForSkillInfo(path, cache, *mode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to analyze %s: %v\n", filepath.Base(path), err)
			continue
		}
		templates = append(templates, info)
	}

	// Build pattern entries (compact for list/compact mode, full only when explicitly requested)
	var patternsCompact []skillPatternCompact
	var patternsFull []skillPatternFull
	if *mode != "list" {
		effectiveMode := *mode
		if effectiveMode == "full" && !*includeFullSchemas {
			effectiveMode = "compact"
		}
		patternsCompact, patternsFull = buildPatternEntries(effectiveMode)
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
		InputFormats:    []string{"json"},
		OutputFormats:   []string{"pptx"},
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

// analyzeTemplateForSkillInfo analyzes a single template and returns skill info.
func analyzeTemplateForSkillInfo(templatePath string, cache types.TemplateCache, mode string) (skillTemplateInfo, error) {
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

	layoutNames := make([]string, len(analysis.Layouts))
	for i, l := range analysis.Layouts {
		layoutNames[i] = l.Name
	}
	info.LayoutNames = layoutNames

	if mode == "full" {
		// Include detailed placeholder information per layout
		layouts := make([]skillLayoutInfo, len(analysis.Layouts))
		for i, l := range analysis.Layouts {
			phs := make([]skillPlaceholderInfo, 0, len(l.Placeholders))
			for _, ph := range l.Placeholders {
				// Skip internal OOXML metadata placeholders (date, footer, slide number)
				// that agents should never target.
				if ph.Type == types.PlaceholderOther {
					continue
				}
				phs = append(phs, skillPlaceholderInfo{
					ID:       ph.ID,
					Type:     string(ph.Type),
					MaxChars: ph.MaxChars,
					Width:    ph.Bounds.Width,
					Height:   ph.Bounds.Height,
				})
			}
			tags := l.Tags
			if tags == nil {
				tags = []string{}
			}
			layouts[i] = skillLayoutInfo{
				Name: l.Name,
				ID:   l.ID,
				Tags: tags,
				Placeholders: phs,
				Capacity: skillCapacity{
					MaxBullets:   l.Capacity.MaxBullets,
					MaxTextLines: l.Capacity.MaxTextLines,
					HasImageSlot: l.Capacity.HasImageSlot,
					HasChartSlot: l.Capacity.HasChartSlot,
				},
			}
		}
		info.Layouts = layouts
	}

	return info, nil
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
		DiagramTypes: []string{
			"timeline",
			"process_flow",
			"pyramid",
			"venn",
			"swot",
			"org_chart",
			"gantt",
			"matrix_2x2",
			"porters_five_forces",
			"house_diagram",
			"business_model_canvas",
			"value_chain",
			"nine_box_talent",
			"kpi_dashboard",
			"heatmap",
			"fishbone",
			"pestel",
			"panel_layout",
			"icon_columns",
			"icon_rows",
			"stat_cards",
		},
		GridCellTypes:   []string{"shape", "table", "icon", "image"},
		ShapeGeometries: buildShapeGeometries(),
		DataFormatHints: buildDataFormatHints(),
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
			RequiredKeys: []string{},
			OptionalKeys: []string{"key_partners", "key_activities", "key_resources", "value_proposition", "customer_relationships", "channels", "customer_segments", "cost_structure", "revenue_streams"},
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
			Description:  "panels: [{title, body, icon?, color?}]; layout: \"columns\"|\"rows\"|\"stat_cards\"",
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
		compact[i] = skillPatternCompact{
			Name:                     p.Name(),
			Cells:                    cells,
			UseWhen:                  p.UseWhen(),
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
			SupportsCallout: compact[i].SupportsCallout,
			Version:         p.Version(),
			Schema:          schemaJSON,
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
}
