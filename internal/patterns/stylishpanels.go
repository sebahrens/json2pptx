package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// stylish-panels pattern — accent-banded panels with ribbon headers
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&stylishPanels{})
}

type stylishPanels struct{}

func (sp *stylishPanels) Name() string { return "stylish-panels" }
func (sp *stylishPanels) Description() string {
	return "Accent-banded panels with ribbon headers for pillars, capabilities, or workstreams"
}
func (sp *stylishPanels) UseWhen() string {
	return "3-5 titled content blocks with bullet lists, each representing a pillar, capability, or workstream; prefer card-grid when items need only header+body without bullets, icon-row when items are icon+caption pairs"
}
func (sp *stylishPanels) NotWhen() string {
	return "Items are icon+caption pairs (use icon-row), items need only header+body text (use card-grid), or content is a single metric (use stat-hero)"
}
func (sp *stylishPanels) Version() int      { return 1 }
func (sp *stylishPanels) CellsHint() string { return "3-5" }
func (sp *stylishPanels) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "evidence"},
		PairsWith:     []string{"kpi-3up", "process-flow", "pull-quote"},
		ComposesWith:  []string{"pull-quote", "kpi-3up", "icon-row", "arch-stack"},
		RoleOnSlide:   []string{"pillars"},
		DensityClass:  "medium",
		AccentWeight:  "strong",
	}
}
func (sp *stylishPanels) SupportsCallout() bool        { return true }
func (sp *stylishPanels) SupportsInlineMarkdown() bool { return true }

func (sp *stylishPanels) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 2},
		{Columns: 4, Rows: 2},
		{Columns: 5, Rows: 2},
	}
}

func (sp *stylishPanels) ExemplarValues() any {
	return &StylishPanelsValues{
		{Title: "Strategy", Body: []string{"Market analysis", "Competitive positioning", "Growth targets"}},
		{Title: "Execution", Body: []string{"Sprint planning", "Resource allocation", "Risk management"}},
		{Title: "Measurement", Body: []string{"KPI tracking", "Quarterly reviews", "Course correction"}},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// StylishPanelsItem is a single panel with a title and bullet list body.
type StylishPanelsItem struct {
	Title string   `json:"title"`
	Body  []string `json:"body"`
}

// UnmarshalJSON supports object {title, body} form.
func (item *StylishPanelsItem) UnmarshalJSON(data []byte) error {
	type alias StylishPanelsItem
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("StylishPanelsItem must be {title, body}: %w", err)
	}
	*item = StylishPanelsItem(a)
	return nil
}

// StylishPanelsValues is the values type: 3–5 panel items.
type StylishPanelsValues = []StylishPanelsItem

// StylishPanelsOverrides contains pattern-level overrides.
type StylishPanelsOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"` // uniform | alternate | progressive
	// Ribbon is "dark" (default: the template's structural dark tone, dk2 or
	// a 60% neutral when dk2 is black) or "accent" (accent-filled ribbons;
	// cell_accent_mode applies).
	Ribbon string `json:"ribbon,omitempty"`
}

// stylishPanelsRibbons are the accepted overrides.ribbon values.
var stylishPanelsRibbons = []string{"dark", "accent"}

// StylishPanelsCellOverride is an alias for the shared CellOverride struct.
type StylishPanelsCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (sp *stylishPanels) NewValues() any       { return &StylishPanelsValues{} }
func (sp *stylishPanels) NewOverrides() any    { return &StylishPanelsOverrides{} }
func (sp *stylishPanels) NewCellOverride() any { return &StylishPanelsCellOverride{} }

// Average readable characters per bullet by panel count, longest ribbon title,
// and bullets in a panel, measured against the written size (no run stored
// below its role floor) on every shipped template at default text sizes, every
// shape keeping the uniform 0.5 cm text margin (go-slide-creator-n1muf).
// Individual bullets can be longer when other
// bullets are short because they share one body cell, up to
// stylishPanelSingleBulletMax.
type stylishPanelBudgetBand struct {
	maxTitle  int
	perBullet [8]int
}

var stylishPanelBodyBudgets = map[int][]stylishPanelBudgetBand{
	3: {
		{25, [8]int{200, 200, 161, 121, 81, 75, 75, 40}},
		{45, [8]int{200, 200, 153, 115, 75, 75, 40, 38}},
		{80, [8]int{200, 200, 121, 81, 75, 40, 38, 38}},
	},
	4: {
		{5, [8]int{200, 200, 115, 83, 53, 52, 52, 38}},
		{25, [8]int{200, 175, 102, 77, 52, 52, 38, 32}},
		{80, [8]int{200, 143, 83, 53, 52, 38, 32, 31}},
	},
	5: {
		{5, [8]int{200, 141, 81, 61, 41, 41, 41, 38}},
		{25, [8]int{200, 121, 81, 61, 41, 41, 38, 25}},
		{45, [8]int{200, 101, 61, 41, 41, 38, 38, 25}},
		{65, [8]int{181, 81, 61, 41, 38, 25, 25, 25}},
		{80, [8]int{141, 61, 41, 38, 25, 25, 25, 25}},
	},
}

// stylishPanelSingleBulletMax is the longest single bullet a panel holds
// beside short neighbours (eight bullets, 80-character titles), by panel
// count; a panel's own average budget, when larger, still applies.
var stylishPanelSingleBulletMax = map[int]int{3: 121, 4: 83, 5: 38}

func stylishPanelBodyBudget(panels, longestTitle, bullets int) int {
	if bullets < 1 || bullets > 8 {
		return 200
	}
	for _, band := range stylishPanelBodyBudgets[panels] {
		if longestTitle <= band.maxTitle {
			return band.perBullet[bullets-1]
		}
	}
	return 200
}

func (sp *stylishPanels) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*StylishPanelsValues)
	if !ok || v == nil {
		return nil
	}
	warnings := stylishPanelsBudgetWarnings(v)
	if len(warnings) > 0 || len(*v) == 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 {
		return warnings
	}
	// The character budgets assume a typical content area; with the
	// template's own area the panels are measured against it at the smallest
	// type they step to (go-slide-creator-n1muf).
	ovr, _ := overrides.(*StylishPanelsOverrides)
	if ovr == nil {
		ovr = &StylishPanelsOverrides{}
	}
	if fit := stylishPanelsFit(ctx, *v, ovr); !fit.fits {
		warnings = append(warnings, fmt.Sprintf("%s: stylish-panels ribbons and bullets need %.0fpt at readable sizes but the content area holds about %.0fpt — shorten the bullets or titles, drop bullets, or use fewer panels", ErrCodeBodyTooLong, fit.totalPt, fit.areaHPt))
	}
	return warnings
}

// stylishPanelsBudgetWarnings reports bullet bodies past the measured
// character budgets for their panel count, title length and bullet count.
func stylishPanelsBudgetWarnings(v *StylishPanelsValues) []string {
	longestTitle := 0
	for _, panel := range *v {
		if n := runeLen(panel.Title); n > longestTitle {
			longestTitle = n
		}
	}
	var warnings []string
	for i, panel := range *v {
		budget := stylishPanelBodyBudget(len(*v), longestTitle, len(panel.Body))
		chars := 0
		for _, bullet := range panel.Body {
			chars += runeLen(bullet)
		}
		if len(panel.Body) > 0 && chars > len(panel.Body)*budget {
			warnings = append(warnings, fmt.Sprintf("%s: stylish-panels values[%d].body has %d characters across %d bullets; %d panels with a longest title of %d characters hold about %d characters per bullet on average — shorten or redistribute bullets, use fewer panels, or shorten titles", ErrCodeBodyTooLong, i, chars, len(panel.Body), len(*v), longestTitle, budget))
			continue
		}
		if single, ok := stylishPanelSingleBulletMax[len(*v)]; ok {
			single = max(single, budget)
			for j, bullet := range panel.Body {
				if n := runeLen(bullet); n > single {
					warnings = append(warnings, fmt.Sprintf("%s: stylish-panels values[%d].body[%d] is %d characters; %d panels hold about %d characters in one bullet — shorten the bullet or use fewer panels", ErrCodeBodyTooLong, i, j, n, len(*v), single))
					break
				}
			}
		}
	}
	return warnings
}

func (sp *stylishPanels) Schema() *Schema {
	itemSchema := ObjectSchema(
		map[string]*Schema{
			"title": StringSchema(80).WithDescription("Panel header title; longer ribbon titles leave less height for every panel's bullets"),
			"body":  ArraySchema(StringSchema(200), 1, 8).WithDescription("Bullets share one panel body; readable characters per bullet on average at short titles, by 1-8 bullets: 3 panels 200/200/161/121/81/75/75/40, 4 panels 200/200/115/83/53/52/52/38, 5 panels 200/141/81/61/41/41/41/38. Long titles reduce dense limits; fit warnings name the measured target. A single bullet beside short neighbors may use 121 characters with 3 panels, 83 with 4, 38 with 5"),
		},
		[]string{"title", "body"},
	).WithAdditionalProperties(false).WithDescription("Panel with titled header and bulleted body")

	return ObjectSchema(
		map[string]*Schema{
			"values": ArraySchema(itemSchema, 3, 5).WithDescription("3–5 panel items"),
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":           StringSchema(0).WithDescription("Accent scheme color (default accent2)").WithDefault("accent2"),
					"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
					"header_size":      NumberSchema(6, 120).WithDescription("Font size for panel headers in points"),
					"body_size":        NumberSchema(6, 120).WithDescription("Font size for body bullet text in points"),
					"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-cell accent variation for ribbon=accent: uniform (default), alternate, progressive").WithDefault("uniform"),
					"ribbon":           EnumSchema(stylishPanelsRibbons...).WithDescription("Ribbon header fill: dark (default: the template's structural dark tone, dk2 or a 60% neutral when dk2 is black, so three to five panels are not a wall of accent) or accent (accent-filled ribbons; cell_accent_mode applies)").WithDefault("dark"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Accent-banded panels with ribbon headers for pillars, capabilities, or workstreams")
}

func (sp *stylishPanels) Validate(values, overrides any, cellOverrides map[int]any) error {
	items, ok := values.(*StylishPanelsValues)
	if !ok || items == nil {
		return fmt.Errorf("stylish-panels: values must be []StylishPanelsItem, got %T", values)
	}

	const name = "stylish-panels"
	var errs []error

	// Validate overrides
	if overrides != nil {
		if ovr, ok := overrides.(*StylishPanelsOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if ovr.Ribbon != "" && !slices.Contains(stylishPanelsRibbons, ovr.Ribbon) {
				errs = append(errs, errInvalidEnum(name, "overrides.ribbon", ovr.Ribbon, stylishPanelsRibbons))
			}
		}
	}

	if len(*items) < 3 {
		errs = append(errs, errMinItems(name, "values", 3, len(*items), "(hint: use card-grid for fewer items)"))
	}
	if len(*items) > 5 {
		errs = append(errs, errMaxItems(name, "values", 5, len(*items), ""))
	}

	for i, item := range *items {
		titlePath := fmt.Sprintf("values[%d].title", i)
		if item.Title == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if runeLen(item.Title) > 80 {
			errs = append(errs, errMaxLength(name, titlePath, 80, runeLen(item.Title)))
		}
		bodyPath := fmt.Sprintf("values[%d].body", i)
		if len(item.Body) == 0 {
			errs = append(errs, errRequired(name, bodyPath))
		}
		if len(item.Body) > 8 {
			errs = append(errs, errMaxItems(name, bodyPath, 8, len(item.Body), ""))
		}
		for j, bullet := range item.Body {
			bulletPath := fmt.Sprintf("values[%d].body[%d]", i, j)
			if bullet == "" {
				errs = append(errs, errRequired(name, bulletPath))
			} else if runeLen(bullet) > 200 {
				errs = append(errs, errMaxLength(name, bulletPath, 200, runeLen(bullet)))
			}
		}
	}

	// Validate cell_overrides keys
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(*items), ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (sp *stylishPanels) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	items, ok := values.(*StylishPanelsValues)
	if !ok {
		return nil, fmt.Errorf("stylish-panels: values must be *StylishPanelsValues, got %T", values)
	}
	ovr := &StylishPanelsOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*StylishPanelsOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("stylish-panels: overrides must be *StylishPanelsOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	if baseAccent == "" {
		baseAccent = "accent2"
	}
	// Sizes step down towards the 12pt floor only when that makes the panels
	// fit their written height in the content area (go-slide-creator-n1muf).
	fit := stylishPanelsFit(ctx, *items, ovr)
	headerSize, bodySize := fit.headerSize, fit.bodySize
	cellAccentMode := ovr.CellAccentMode

	n := len(*items)

	// Row 1: ribbon header cells (short, coloured band). The default ribbon
	// is the structural dark tone: three to five solid accent ribbons were a
	// wall of colour (go-slide-creator-fl11f).
	headerCells := make([]*jsonschema.GridCellInput, n)
	for i, item := range *items {
		ribbon := structuralDarkTone(ctx)
		if ovr.Ribbon == "accent" {
			ribbon = fillTone{Color: ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)}
		}
		headerCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     ribbon.fillJSON(),
				Line:     noLine,
				Text:     buildStylishHeaderText(item.Title, headerSize),
			},
		}
	}

	// Row 2: body cells (neutral tint, with bullet text). An lt1 body
	// vanished on white-paper templates, leaving bullets hanging under the
	// ribbons (go-slide-creator-95tp7).
	bodyFill := surfaceFillJSON(ctx, "subtle", NeutralTint4)
	bodyCells := make([]*jsonschema.GridCellInput, n)
	for i, item := range *items {
		accent := ctx.ResolveCellAccent(baseAccent, i, cellAccentMode)
		bodyCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     bodyFill,
				Line:     noLine,
				Text:     buildStylishBodyText(item.Body, bodySize, accent),
			},
		}

		// Apply cell overrides
		if co, ok := cellOverrides[i]; ok {
			cellOvr, coOk := co.(*StylishPanelsCellOverride)
			if coOk {
				applyCellTextOverride(bodyCells[i], cellOvr)
			}
			if coOk && cellOvr.AccentBar {
				bodyCells[i].AccentBar = &jsonschema.AccentBarInput{
					Position: "top",
					Color:    accent,
					Width:    4,
				}
			}
		}
	}

	// Content-sized rows (go-slide-creator-3i7c): the ribbon header is a
	// band ~1.2x the header line height instead of 20% of the slide, and the
	// body row hugs the longest bullet list; the grid centres the block.
	font := ctx.Theme.BodyFont
	contentW, _ := contentAreaPt(ctx)
	colW := equalColumnWidthPt(contentW, n, ctx.Gap(12)) - 2*defaultShapeInsetLRPt
	titles := make([]string, n)
	bodyH := 0.0
	for i, item := range *items {
		titles[i] = item.Title
		bodyH = math.Max(bodyH, shapeTextHeightPt(font, bodyCells[i].Shape.Text, colW))
	}
	// Neither row is ever handed less than the written fit of its tallest
	// cell: sized by the theme-font model alone, the bodies were written at
	// 44-84% autofit on the short content areas (go-slide-creator-n1muf).
	// The ribbon is its written need: the band padding headerRowPt adds made
	// a one-line title a ~65px slab (go-slide-creator-95tp7).
	headerPt := math.Max(headerRowPt(font, titles, headerSize, colW)-headerBandPadPt, fit.headerPt)
	bodyMax := math.Max(math.Round(bodyH+2*defaultShapeInsetTBPt+cardPadPt), fit.bodyPt)
	bodyRow := jsonschema.GridRowInput{Cells: bodyCells, MaxHeight: bodyMax}
	if fit.fits {
		bodyRow.MinHeight = fit.bodyPt
	}
	grid := &jsonschema.ShapeGridInput{
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, n)),
		Gap:           ctx.Gap(stylishPanelsGapPt),
		VerticalAlign: GridVerticalAlignDefault,
		Rows: []jsonschema.GridRowInput{
			{Cells: headerCells, MinHeight: headerPt, MaxHeight: headerPt},
			bodyRow,
		},
	}

	return grid, nil
}

// stylishPanelsGapPt is the gutter between panels and between the ribbon
// header and the panel body.
const stylishPanelsGapPt = 12.0

// stylishPanelsFitResult is the type size the panels are written at and the
// written-fit heights of the ribbon row and the body row at that size.
type stylishPanelsFitResult struct {
	headerSize, bodySize float64
	headerPt, bodyPt     float64
	totalPt, areaHPt     float64
	fits                 bool
}

// stylishPanelsFit measures the ribbon titles and bullet bodies with the
// writer's own fit at their real column width. The defaults (16pt ribbon,
// 14pt bullets) step towards the 12pt floor — bullets first, then the
// ribbon — only when the panels would not otherwise fit the content area;
// authored sizes are kept.
func stylishPanelsFit(ctx ExpandContext, items []StylishPanelsItem, ovr *StylishPanelsOverrides) stylishPanelsFitResult {
	type step struct{ header, body float64 }
	steps := []step{{ResolveSize(ovr.HeaderSize, sizeHeaderPt), ResolveSize(ovr.BodySize, scaleSubheadPt)}}
	if ovr.BodySize == 0 {
		steps = append(steps, step{steps[0].header, 12})
	}
	if ovr.HeaderSize == 0 && ovr.BodySize == 0 {
		steps = append(steps, step{14, 12})
	}
	areaW, areaH := sizingAreaPt(ctx)
	colW := equalColumnWidthPt(areaW, len(items), ctx.Gap(stylishPanelsGapPt))
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Title
	}
	measure := func(st step) stylishPanelsFitResult {
		r := stylishPanelsFitResult{headerSize: st.header, bodySize: st.body, areaHPt: areaH}
		// The ribbon keeps the band padding the model gives it.
		r.headerPt = headerRowPt(ctx.Theme.BodyFont, titles, st.header, colW-2*defaultShapeInsetLRPt)
		for _, item := range items {
			r.headerPt = math.Max(r.headerPt, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, buildStylishHeaderText(item.Title, st.header), colW))
			r.bodyPt = math.Max(r.bodyPt, writtenNeedOrOverflowPt(ctx.Theme.BodyFont, buildStylishBodyText(item.Body, st.body, "accent2"), colW))
		}
		r.totalPt = r.headerPt + ctx.Gap(stylishPanelsGapPt) + r.bodyPt
		r.fits = r.totalPt <= areaH
		return r
	}
	for _, st := range steps {
		if r := measure(st); r.fits {
			return r
		}
	}
	// Nothing fits: keep the first (default or authored) sizes, so an
	// overflowing payload is not also set smaller before it is shrunk.
	return measure(steps[0])
}

// buildStylishHeaderText creates bold centered white text for the accent header band.
func buildStylishHeaderText(title string, headerSize float64) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: title, Size: headerSize, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// buildStylishBodyText creates bullet-list text on a light background.
func buildStylishBodyText(bullets []string, bodySize float64, accent string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
		Bullet  bool    `json:"bullet,omitempty"`
	}
	paras := make([]paragraph, len(bullets))
	for i, b := range bullets {
		paras[i] = paragraph{
			Bullet:  true,
			Content: pptx.ConvertMarkdownEmphasis(b),
			Size:    bodySize,
			Color:   "dk1",
			Align:   "l",
		}
	}
	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	data, _ := json.Marshal(textObj)
	return data
}
