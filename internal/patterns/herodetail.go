package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// hero-detail pattern — big hero stat at top with supporting detail cards below
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&heroDetail{})
}

type heroDetail struct{}

func (hd *heroDetail) Name() string { return "hero-detail" }
func (hd *heroDetail) Description() string {
	return "Big hero statistic with 2-4 supporting detail cards below"
}
func (hd *heroDetail) UseWhen() string {
	return "One dominant metric plus 2-4 supporting detail bullets; prefer stat-hero when no details are needed, kpi-3up when all metrics have equal weight"
}
func (hd *heroDetail) NotWhen() string {
	return "No supporting details (use stat-hero), all items have equal weight (use kpi-3up or kpi-4up), or content is a quote (use pull-quote)"
}
func (hd *heroDetail) Version() int      { return 2 }
func (hd *heroDetail) CellsHint() string { return "1 + 2-4" }
func (hd *heroDetail) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "hero",
		NarrativeRole: []string{"evidence"},
		PairsWith:     []string{"kpi-3up", "chart", "process-flow"},
		DensityClass:  "low",
		AccentWeight:  "strong",
	}
}

func (hd *heroDetail) SupportsCallout() bool        { return true }
func (hd *heroDetail) SupportsInlineMarkdown() bool { return true }

func (hd *heroDetail) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 2, Rows: 2},
		{Columns: 3, Rows: 2},
		{Columns: 4, Rows: 2},
	}
}

func (hd *heroDetail) ExemplarValues() any {
	return &HeroDetailValues{
		Hero: HeroDetailHero{
			Value: "$2.4B",
			Label: "Addressable AI consulting market by FY27",
		},
		Details: []HeroDetailItem{
			{Title: "Market Growth", Body: "42% CAGR driven by enterprise adoption"},
			{Title: "Key Segment", Body: "Financial services leads with 35% share"},
			{Title: "Outlook", Body: "Projected to reach $5.1B by FY30"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// HeroDetailHero holds the hero stat data (top row).
type HeroDetailHero struct {
	Value   string `json:"value"`             // The big number (e.g. "$2.4B", "99.9%")
	Label   string `json:"label"`             // One-line context beneath the number
	Context string `json:"context,omitempty"` // Optional subtext
}

// HeroDetailItem is a single detail card (bottom row).
type HeroDetailItem struct {
	Icon  *IconRef `json:"icon,omitempty"` // Icon: bundled-name string shorthand or {name|path|url|svg_data, fill?, alt?, position?} object. Emoji glyphs are rejected.
	Title string   `json:"title"`          // Card title
	Body  string   `json:"body,omitempty"` // Card body text
}

// HeroDetailValues holds the data for a hero-detail pattern.
type HeroDetailValues struct {
	Hero    HeroDetailHero   `json:"hero"`
	Details []HeroDetailItem `json:"details"`
}

// HeroDetailOverrides contains pattern-level overrides for hero-detail.
type HeroDetailOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeroSize       float64 `json:"hero_size,omitempty"`   // Font size for the big number (default 80)
	LabelSize      float64 `json:"label_size,omitempty"`  // Font size for the label (default 16)
	HeaderSize     float64 `json:"header_size,omitempty"` // Font size for detail titles (default 14)
	DetailSize     float64 `json:"detail_size,omitempty"` // Font size for detail body text (default 11)
	Style          string  `json:"style,omitempty"`       // "minimal" (default) or "cards"
}

// HeroDetailCellOverride is an alias for the shared CellOverride struct.
type HeroDetailCellOverride = CellOverride

// validHeroDetailStyles enumerates the allowed style values.
var validHeroDetailStyles = map[string]bool{
	"":        true, // default = minimal
	"cards":   true,
	"minimal": true,
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (hd *heroDetail) NewValues() any       { return &HeroDetailValues{} }
func (hd *heroDetail) NewOverrides() any    { return &HeroDetailOverrides{} }
func (hd *heroDetail) NewCellOverride() any { return &HeroDetailCellOverride{} }

// Measured against the written size (no run stored below its role floor) on
// every shipped template with every shape keeping the uniform 0.5 cm text
// margin (go-slide-creator-n1muf). Icons occupy vertical room in their own
// card; no-icon cards keep the 200-char maximum except four cards with long
// titles.
func heroDetailBodyBudget(details, titleChars int, icon bool) int {
	switch {
	case !icon && details >= 4 && titleChars > 20:
		return 150
	case !icon:
		return 200
	case details <= 2 && titleChars > 20:
		return 66
	case details <= 2:
		return 193
	case details == 3 && titleChars > 20:
		return 41
	case details == 3:
		return 118
	case titleChars > 27:
		return 0
	case titleChars > 20:
		return 30
	default:
		return 32
	}
}

// heroDetailContextBudget is the readable hero context length.
const heroDetailContextBudget = 40

func (hd *heroDetail) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*HeroDetailValues)
	if !ok || v == nil {
		return nil
	}
	var warnings []string
	if n := runeLen(v.Hero.Context); n > heroDetailContextBudget {
		warnings = append(warnings, fmt.Sprintf("%s: hero-detail hero.context is %d characters; the hero holds about %d readable context characters — shorten the context", ErrCodeBodyTooLong, n, heroDetailContextBudget))
	}
	for i, detail := range v.Details {
		icon := detail.Icon != nil
		budget := heroDetailBodyBudget(len(v.Details), runeLen(detail.Title), icon)
		if n := runeLen(detail.Body); n > 0 && budget == 0 {
			warnings = append(warnings, fmt.Sprintf("%s: hero-detail details[%d].body is %d characters; %d detail cards (icon=%t) with a %d-character title hold no readable body — shorten the title, omit the icon, or use fewer cards", ErrCodeBodyTooLong, i, n, len(v.Details), icon, runeLen(detail.Title)))
		} else if n > budget {
			warnings = append(warnings, fmt.Sprintf("%s: hero-detail details[%d].body is %d characters; %d detail cards (icon=%t) with a %d-character title hold about %d readable body characters — shorten the body or title, omit the icon, or use fewer cards", ErrCodeBodyTooLong, i, n, len(v.Details), icon, runeLen(detail.Title), budget))
		}
	}
	if len(warnings) > 0 || ctx.LayoutBounds.Width <= 0 || ctx.LayoutBounds.Height <= 0 || len(v.Details) == 0 {
		return warnings
	}
	// The budgets assume a typical content area; with the template's own
	// area the rows are measured against it (go-slide-creator-n1muf).
	ovr, _ := overrides.(*HeroDetailOverrides)
	if ovr == nil {
		ovr = &HeroDetailOverrides{}
	}
	if plan := hd.layout(ctx, v, ovr, nil); !plan.fits && heroDetailShrinks(ctx, plan) {
		warnings = append(warnings, fmt.Sprintf("%s: hero-detail hero and detail cards need %s at readable sizes but the content area holds about %.0fpt — shorten the card bodies or titles, the hero label or context, or use fewer cards", ErrCodeBodyTooLong, readableNeedPhrase(plan.need), plan.avail))
	}
	return warnings
}

func (hd *heroDetail) Schema() *Schema {
	heroSchema := ObjectSchema(
		map[string]*Schema{
			"value":   StringSchema(20).WithDescription("The big number (e.g. \"$2.4B\", \"99.9%\")"),
			"label":   StringSchema(80).WithDescription("One-line label beneath the number"),
			"context": StringSchema(120).WithDescription("Optional subtext line; about 40 readable characters"),
		},
		[]string{"value", "label"},
	).WithAdditionalProperties(false)

	detailSchema := ObjectSchema(
		map[string]*Schema{
			"icon":  IconRefSchema("Optional icon: bundled name string or {name|path|url|svg_data, fill?, alt?, position?} object"),
			"title": StringSchema(60).WithDescription("Detail card title"),
			"body":  StringSchema(200).WithDescription("Detail card body. No-icon cards retain 200 readable characters (150 for four cards with titles over 20 characters). With icons, two cards hold about 193 (66 for titles over 20 characters), three cards about 118 for titles up to 20 characters or 41 for longer, four cards about 32 for titles up to 20, 30 up to 27, and no body beyond"),
		},
		[]string{"title"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"hero":    heroSchema.WithDescription("The hero statistic (top row)"),
			"details": ArraySchema(detailSchema, 2, 4).WithDescription("Supporting detail cards (bottom row, 2-4 items)"),
		},
		[]string{"hero", "details"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"hero_size":       NumberSchema(40, 200).WithDescription("Font size for the big number in points (default 80)"),
			"label_size":      NumberSchema(6, 60).WithDescription("Font size for the label in points (default 16)"),
			"header_size":     NumberSchema(6, 60).WithDescription("Font size for detail titles in points (default 14)"),
			"detail_size":     NumberSchema(6, 40).WithDescription("Font size for detail body text in points (default 11)"),
			"style":           EnumSchema("minimal", "cards").WithDescription("Visual style: minimal (default: detail text beside a thin accent rule, no fill, top-aligned titles, so the hero stays the one emphasised element) or cards (accent-filled detail cards)").WithDefault("minimal"),
		},
		nil,
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      overridesSchema,
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Big hero statistic with 2-4 supporting detail cards below")
}

func (hd *heroDetail) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*HeroDetailValues)
	if !ok || v == nil {
		return fmt.Errorf("hero-detail: values must be *HeroDetailValues, got %T", values)
	}

	const name = "hero-detail"
	var errs []error

	// Hero validation
	if v.Hero.Value == "" {
		errs = append(errs, errRequired(name, "hero.value"))
	} else if runeLen(v.Hero.Value) > 20 {
		errs = append(errs, errMaxLength(name, "hero.value", 20, runeLen(v.Hero.Value)))
	}

	if v.Hero.Label == "" {
		errs = append(errs, errRequired(name, "hero.label"))
	} else if runeLen(v.Hero.Label) > 80 {
		errs = append(errs, errMaxLength(name, "hero.label", 80, runeLen(v.Hero.Label)))
	}

	if v.Hero.Context != "" && runeLen(v.Hero.Context) > 120 {
		errs = append(errs, errMaxLength(name, "hero.context", 120, runeLen(v.Hero.Context)))
	}

	// Details validation
	if len(v.Details) < 2 || len(v.Details) > 4 {
		errs = append(errs, errOutOfRange(name, "details count", 2, 4, len(v.Details)))
	}

	for i, d := range v.Details {
		titlePath := fmt.Sprintf("details[%d].title", i)
		if d.Title == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if runeLen(d.Title) > 60 {
			errs = append(errs, errMaxLength(name, titlePath, 60, runeLen(d.Title)))
		}
		if d.Body != "" && runeLen(d.Body) > 200 {
			bodyPath := fmt.Sprintf("details[%d].body", i)
			errs = append(errs, errMaxLength(name, bodyPath, 200, runeLen(d.Body)))
		}
		if d.Icon != nil {
			iconPath := fmt.Sprintf("details[%d].icon", i)
			errs = append(errs, validateIconRef(name, iconPath, *d.Icon)...)
		}
	}

	// Validate style override
	if overrides != nil {
		if ovr, ok := overrides.(*HeroDetailOverrides); ok {
			if ovr.Style != "" && !validHeroDetailStyles[ovr.Style] {
				errs = append(errs, &ValidationError{
					Pattern: name,
					Path:    "overrides.style",
					Code:    "invalid_enum",
					Message: fmt.Sprintf("hero-detail: overrides.style must be one of cards, minimal; got %q", ovr.Style),
				})
			}
		}
	}

	// Validate cell_overrides keys — total cells = 1 (hero) + len(details)
	totalCells := 1 + len(v.Details)
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (hd *heroDetail) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*HeroDetailValues)
	if !ok {
		return nil, fmt.Errorf("hero-detail: values must be *HeroDetailValues, got %T", values)
	}
	ovr := &HeroDetailOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*HeroDetailOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("hero-detail: overrides must be *HeroDetailOverrides, got %T", overrides)
		}
	}

	plan := hd.layout(ctx, v, ovr, cellOverrides)

	// Content-sized rows (go-slide-creator-3i7c): the hero row hugs the
	// stat + label, detail cards hug their title/body (sparse card text is
	// centred), and the grid centres the block vertically. Neither row is
	// below the written fit of its text (go-slide-creator-n1muf).
	grid := &jsonschema.ShapeGridInput{
		VerticalAlign: GridVerticalAlignDefault,
		Columns:       json.RawMessage(fmt.Sprintf(`%d`, len(v.Details))),
		Gap:           ctx.Gap(heroDetailGapPt),
		Rows: []jsonschema.GridRowInput{
			{MinHeight: plan.heroNeed, MaxHeight: plan.heroMax, Cells: []*jsonschema.GridCellInput{plan.hero}},
			{MinHeight: plan.detailMin(), MaxHeight: plan.detailMax, Cells: plan.details},
		},
	}
	if plan.rowGap != ctx.Gap(heroDetailGapPt) {
		grid.RowGap = plan.rowGap
	}

	return grid, nil
}

// heroDetailGapPt is the default gap between the hero and the cards and
// between the cards.
const heroDetailGapPt = 10.0

// heroDetailPlan is one candidate layout: the cells at one set of sizes and
// the heights of the hero row and the card row.
type heroDetailPlan struct {
	hero                *jsonschema.GridCellInput
	details             []*jsonschema.GridCellInput
	heroNeed, heroMax   float64
	detailNeed          float64
	detailMax           float64
	rowGap, need, avail float64
	fits                bool
}

// detailMin floors the card row at its written fit while the block fits;
// past that the cards take what the hero leaves (BODY_TOO_LONG reports it).
func (p heroDetailPlan) detailMin() float64 {
	if p.fits {
		return p.detailNeed
	}
	return 0
}

// heroDetailHeroSteps are the default hero figure sizes, largest first. The
// hero takes the largest that holds its stat, label and context within its
// share of the content area.
var heroDetailHeroSteps = []float64{80, 64, 48}

// heroDetailHeroSharePct is the share of the content area the hero row keeps.
const heroDetailHeroSharePct = 45.0

// layout sizes the hero and card rows to the written fit of their text
// (go-slide-creator-n1muf). The hero figure takes the largest default step
// that fits its share of the area; when the cards still do not fit, the hero
// steps down further, then the row gap gives way, then the label and card
// titles step to the 14pt / 12pt floor. Authored sizes are kept. A step is
// taken only when it makes the block fit.
func (hd *heroDetail) layout(ctx ExpandContext, v *HeroDetailValues, ovr *HeroDetailOverrides, cellOverrides map[int]any) heroDetailPlan {
	type step struct{ hero, label, header, gap float64 }
	label := ResolveSize(ovr.LabelSize, sizeHeaderPt)
	header := ResolveSize(ovr.HeaderSize, scaleSubheadPt)
	heroSizes := []float64{ResolveSize(ovr.HeroSize, heroDetailHeroSteps[0])}
	if ovr.HeroSize == 0 {
		heroSizes = heroDetailHeroSteps
	}
	// The first hero step is the largest whose hero row keeps its share.
	_, areaH := sizingAreaPt(ctx)
	base := len(heroSizes) - 1
	for i, hs := range heroSizes[:base] {
		if p := hd.measure(ctx, v, ovr, cellOverrides, hs, label, header, ctx.Gap(heroDetailGapPt)); p.heroNeed <= areaH*heroDetailHeroSharePct/100 {
			base = i
			break
		}
	}
	var steps []step
	for _, hs := range heroSizes[base:] {
		steps = append(steps, step{hs, label, header, ctx.Gap(heroDetailGapPt)})
	}
	last := steps[len(steps)-1]
	steps = append(steps, step{last.hero, last.label, last.header, ctx.Gap(heroDetailMinGapPt)})
	if ovr.LabelSize == 0 || ovr.HeaderSize == 0 {
		st := step{last.hero, last.label, last.header, ctx.Gap(heroDetailMinGapPt)}
		if ovr.LabelSize == 0 {
			st.label = heroDetailMinLabelPt
		}
		if ovr.HeaderSize == 0 {
			st.header = heroDetailMinHeaderPt
		}
		steps = append(steps, st)
	}
	var first heroDetailPlan
	for i, st := range steps {
		plan := hd.measure(ctx, v, ovr, cellOverrides, st.hero, st.label, st.header, st.gap)
		if plan.fits {
			return plan
		}
		if i == 0 {
			first = plan
		}
	}
	return first
}

// Floors the hero-detail steps down to before it reports BODY_TOO_LONG.
const (
	heroDetailMinGapPt    = 4.0
	heroDetailMinLabelPt  = 14.0
	heroDetailMinHeaderPt = 12.0
)

// measure builds the cells at one set of sizes and measures both rows: the
// theme-font model the pattern always used, floored at the written fit of
// the hero and of every card at its real width.
func (hd *heroDetail) measure(ctx ExpandContext, v *HeroDetailValues, ovr *HeroDetailOverrides, cellOverrides map[int]any, heroSize, labelSize, headerSize, rowGap float64) heroDetailPlan {
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	detailSize := ResolveSize(ovr.DetailSize, scaleDenseBodyPt)
	// minimal is the default: the hero number is the slide's one emphasis,
	// so the detail cards carry no accent fill (go-slide-creator-19pp9).
	style := ovr.Style
	if style == "" {
		style = "minimal"
	}

	// Row 1: Hero stat (single cell spanning all columns)
	heroCell := hd.buildHeroCell(v.Hero, accent, heroSize, labelSize)
	heroCell.ColSpan = len(v.Details)
	// Cell 0 is the hero; Validate accepts it, so honour it like the details.
	applyHeroDetailCellOverride(heroCell, cellOverrides, 0, accent)

	font := ctx.Theme.BodyFont
	contentW, contentH := contentAreaPt(ctx)
	// The fit is judged against the conservative sizing area (the layout's
	// own area when known).
	_, availH := sizingAreaPt(ctx)
	heroH := shapeTextHeightPt(font, heroCell.Shape.Text, contentW-2*defaultShapeInsetLRPt)
	plan := heroDetailPlan{hero: heroCell, rowGap: rowGap, avail: availH}
	plan.heroNeed = rowTextNeedPt(heroCell.Shape.Text, contentW)
	plan.heroMax = math.Max(math.Round(math.Min(heroH+2*defaultShapeInsetTBPt+cardPadPt, contentH*heroDetailHeroSharePct/100)), plan.heroNeed)

	// Row 2: Detail cards (N columns)
	detailCells := make([]*jsonschema.GridCellInput, len(v.Details))
	for i, d := range v.Details {
		switch style {
		case "minimal":
			detailCells[i] = hd.buildMinimalDetailCell(ctx, d, accent, headerSize, detailSize)
		default: // "cards"
			detailCells[i] = hd.buildCardDetailCell(ctx, d, accent, headerSize, detailSize)
		}
		// Apply cell overrides (detail cells are 1-indexed since cell 0 is the hero)
		applyHeroDetailCellOverride(detailCells[i], cellOverrides, i+1, accent)
	}

	cardW := equalColumnWidthPt(contentW, len(v.Details), ctx.Gap(heroDetailGapPt))
	textW := cardW - 2*defaultShapeInsetLRPt
	textHs := make([]float64, len(detailCells))
	cardH := 0.0
	for i, dc := range detailCells {
		textHs[i] = shapeTextHeightPt(font, dc.Shape.Text, textW)
		cardH = math.Max(cardH, contentCardHeightPt(textHs[i], cardW, dc.Shape.Icon != nil))
		need := rowTextNeedPt(dc.Shape.Text, cardW)
		if dc.Shape.Icon != nil {
			need = heroDetailIconCardPt(need, cardW)
		}
		plan.detailNeed = math.Max(plan.detailNeed, need)
	}
	plan.detailMax = math.Max(cardH, plan.detailNeed)
	for i, dc := range detailCells {
		// Minimal cards share one title line: centring a two-line body's
		// card dropped its neighbours' titles out of line beside the equal
		// accent rules (go-slide-creator-ppcfn).
		if dc.Shape.Icon == nil && style != "minimal" {
			dc.Shape.Text = anchorSparseText(dc.Shape.Text, textHs[i], plan.detailMax-2*defaultShapeInsetTBPt)
		}
	}
	plan.details = detailCells
	plan.need = plan.heroNeed + rowGap + plan.detailNeed
	plan.fits = plan.need <= plan.avail
	return plan
}

// heroDetailIconCardPt is the smallest card height whose text area, below
// the writer's top icon zone (grid.go iconOverlayBounds: the default 0.6
// scale of the card's smaller side, capped at 40% of the height on a
// landscape card, plus a 3pt gap above and below), holds textNeedPt.
func heroDetailIconCardPt(textNeedPt, cardW float64) float64 {
	for h := math.Ceil(textNeedPt); h < textNeedPt+rowTextBeyondAreaPt; h++ {
		icon := 0.6 * math.Min(cardW, h)
		if cardW > 1.2*h {
			icon = math.Min(icon, 0.4*h)
		}
		if h-(icon+6) >= textNeedPt {
			return h
		}
	}
	return textNeedPt + rowTextBeyondAreaPt
}

// heroDetailShrinks reports whether the writer would store a shrink for the
// hero or a card of plan: past the fit the hero keeps its row and the cards
// take the rest.
func heroDetailShrinks(ctx ExpandContext, plan heroDetailPlan) bool {
	contentW, _ := contentAreaPt(ctx)
	cardW := equalColumnWidthPt(contentW, len(plan.details), ctx.Gap(heroDetailGapPt))
	fits := func(text json.RawMessage, w, h float64) bool {
		tb, err := shapegrid.ResolveTextInput(text)
		return err != nil || tb == nil || pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: int64(w * sizingEMUPerPt), CY: int64(h * sizingEMUPerPt)})
	}
	if !fits(plan.hero.Shape.Text, contentW, plan.heroNeed) {
		return true
	}
	cardH := math.Min(plan.detailNeed, plan.avail-plan.rowGap-plan.heroNeed)
	for _, dc := range plan.details {
		if dc.Shape.Icon != nil {
			if cardH < plan.detailNeed {
				return true
			}
			continue
		}
		if !fits(dc.Shape.Text, cardW, cardH) {
			return true
		}
	}
	return false
}

// buildHeroCell produces the hero stat cell for the top row.
func (hd *heroDetail) buildHeroCell(hero HeroDetailHero, accent string, heroSize, labelSize float64) *jsonschema.GridCellInput {
	paras := []heroDetailParagraph{
		{Content: hero.Value, Size: heroSize, Bold: true, Color: accent, Align: "ctr"},
		{Content: hero.Label, Size: labelSize, Color: "dk1", Align: "ctr"},
	}
	if hero.Context != "" {
		paras = append(paras, heroDetailParagraph{
			Content: hero.Context, Size: labelSize - 2, Color: "dk1", Align: "ctr",
		})
	}

	textObj := heroDetailText{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	textJSON, _ := json.Marshal(textObj)

	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Text:     textJSON,
		},
	}
}

// buildCardDetailCell produces an accent-filled detail card (default style).
func (hd *heroDetail) buildCardDetailCell(ctx ExpandContext, d HeroDetailItem, accent string, headerSize, detailSize float64) *jsonschema.GridCellInput {
	paras := []heroDetailParagraph{
		{Content: d.Title, Size: headerSize, Bold: true, Color: "lt1", Align: "l"},
	}
	if d.Body != "" {
		paras = append(paras, heroDetailParagraph{
			Content: pptx.ConvertMarkdownEmphasis(d.Body), Size: detailSize, Color: "lt1", Align: "l",
		})
	}

	textObj := heroDetailText{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	textJSON, _ := json.Marshal(textObj)

	gc := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "roundRect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     textJSON,
		},
	}

	// Add icon overlay when provided
	if d.Icon != nil {
		if icon := d.Icon.Resolve(iconFillOn(ctx, gc.Shape.Fill, accent), "top"); icon != nil {
			gc.Shape.Icon = icon
		}
	}

	return gc
}

// buildMinimalDetailCell produces a light-background detail card with accent header text.
func (hd *heroDetail) buildMinimalDetailCell(ctx ExpandContext, d HeroDetailItem, accent string, headerSize, detailSize float64) *jsonschema.GridCellInput {
	paras := []heroDetailParagraph{
		{Content: d.Title, Size: headerSize, Bold: true, Color: accent, Align: "l"},
	}
	if d.Body != "" {
		paras = append(paras, heroDetailParagraph{
			Content: pptx.ConvertMarkdownEmphasis(d.Body), Size: detailSize, Color: "dk1", Align: "l",
		})
	}

	textObj := heroDetailText{
		Paragraphs:    paras,
		Align:         "l",
		VerticalAlign: "t",
	}
	textJSON, _ := json.Marshal(textObj)

	gc := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			// No fill: an lt1 card is invisible on white paper and a white
			// box on a tinted slide background (go-slide-creator-19pp9).
			Fill: json.RawMessage(`"none"`),
			Text: textJSON,
		},
		AccentBar: &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    3,
		},
	}

	if d.Icon != nil {
		if icon := d.Icon.Resolve(iconFillOn(ctx, gc.Shape.Fill, accent), "top"); icon != nil {
			gc.Shape.Icon = icon
		}
	}

	return gc
}

// heroDetailParagraph is a text paragraph for JSON marshalling.
type heroDetailParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

// heroDetailText is the text object for JSON marshalling.
type heroDetailText struct {
	Paragraphs    []heroDetailParagraph `json:"paragraphs"`
	Align         string                `json:"align"`
	VerticalAlign string                `json:"vertical_align"`
}

// applyHeroDetailCellOverride applies cell_overrides[idx] (0 = hero, i+1 =
// detail i): the D15 text keys and a top accent bar.
func applyHeroDetailCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	cellOvr, ok := cellOverrides[idx].(*HeroDetailCellOverride)
	if !ok || cell == nil {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    3,
		}
	}
}
