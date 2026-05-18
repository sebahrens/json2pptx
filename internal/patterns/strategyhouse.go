package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// strategy-house pattern — objective banner over N pillars over a foundation
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&strategyHouse{})
}

type strategyHouse struct{}

func (sh *strategyHouse) Name() string { return "strategy-house" }
func (sh *strategyHouse) Description() string {
	return "Strategy House diagram with optional roof badges, an objective banner, 3-5 pillar columns, and a foundation row"
}
func (sh *strategyHouse) UseWhen() string {
	return "Strategic framework with a top-level objective, 3-5 pillars supporting it, and a foundation row of enablers; prefer arch-stack when layers stack vertically without a single objective/foundation framing, stylish-panels when pillars stand alone without banner/foundation"
}
func (sh *strategyHouse) NotWhen() string {
	return "Layers stack without a single objective and foundation framing (use arch-stack), pillars stand alone without banner/foundation (use stylish-panels), or content is a hierarchy that narrows (use pyramid)"
}
func (sh *strategyHouse) Version() int        { return 1 }
func (sh *strategyHouse) CellsHint() string   { return "objective + 3-5 pillars + foundation (+0-3 roof badges)" }
func (sh *strategyHouse) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "conclude"},
		PairsWith:     []string{"stylish-panels", "kpi-3up", "process-flow"},
		DensityClass:  "medium",
		AccentWeight:  "strong",
	}
}
func (sh *strategyHouse) SupportsCallout() bool        { return false }
func (sh *strategyHouse) SupportsInlineMarkdown() bool { return true }

func (sh *strategyHouse) BudgetConfigurations() []BudgetConfig {
	return []BudgetConfig{
		{Columns: 3, Rows: 3},
		{Columns: 4, Rows: 3},
		{Columns: 5, Rows: 3},
	}
}

func (sh *strategyHouse) ExemplarValues() any {
	return &StrategyHouseValues{
		Objective: "Become the trusted platform for global commerce",
		Pillars: []StrategyHousePillar{
			{Title: "Customer Trust", Body: []string{"Privacy by default", "Transparent pricing"}},
			{Title: "Operational Excellence", Body: []string{"99.99% uptime", "Automated quality gates"}},
			{Title: "Product Velocity", Body: []string{"Weekly releases", "Continuous experimentation"}},
		},
		Foundation: "People · Technology · Data",
		RoofBadges: []string{"Vision", "Mission"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// StrategyHousePillar is a single pillar column with a title and bullet body.
type StrategyHousePillar struct {
	Title string   `json:"title"`
	Body  []string `json:"body,omitempty"`
}

// StrategyHouseValues holds the four regions of the strategy house: optional
// roof badges, a required objective banner, 3-5 pillar columns, and a
// foundation row of enablers.
type StrategyHouseValues struct {
	Objective  string                `json:"objective"`
	Pillars    []StrategyHousePillar `json:"pillars"`
	Foundation string                `json:"foundation"`
	RoofBadges []string              `json:"roof_badges,omitempty"`
}

// StrategyHouseOverrides is the standard text overrides.
type StrategyHouseOverrides = TextOverrides

// StrategyHouseCellOverride is the shared per-cell override.
type StrategyHouseCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (sh *strategyHouse) NewValues() any       { return &StrategyHouseValues{} }
func (sh *strategyHouse) NewOverrides() any    { return &StrategyHouseOverrides{} }
func (sh *strategyHouse) NewCellOverride() any { return &StrategyHouseCellOverride{} }

func (sh *strategyHouse) Schema() *Schema {
	pillarSchema := ObjectSchema(
		map[string]*Schema{
			"title": StringSchema(60).WithDescription("Pillar title"),
			"body":  ArraySchema(StringSchema(120), 0, 5).WithDescription("Pillar bullet items (0-5)"),
		},
		[]string{"title"},
	).WithAdditionalProperties(false).WithDescription("Pillar column with title and optional bullet body")

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"objective":   StringSchema(140).WithDescription("Strategic objective rendered as the banner above the pillars"),
			"pillars":     ArraySchema(pillarSchema, 3, 5).WithDescription("3-5 pillar columns supporting the objective"),
			"foundation":  StringSchema(140).WithDescription("Foundation row text rendered beneath the pillars (enablers, principles, or capabilities)"),
			"roof_badges": ArraySchema(StringSchema(24), 0, 3).WithDescription("Optional badges rendered above the banner (0-3, e.g. vision/mission tags)"),
		},
		[]string{"objective", "pillars", "foundation"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values":         valuesSchema,
			"overrides":      textOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Strategy House: banner objective above 3-5 pillars above a foundation, with optional roof badges")
}

func (sh *strategyHouse) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*StrategyHouseValues)
	if !ok || vals == nil {
		return fmt.Errorf("strategy-house: values must be *StrategyHouseValues, got %T", values)
	}

	const name = "strategy-house"
	var errs []error

	if overrides != nil {
		if ovr, ok := overrides.(*StrategyHouseOverrides); ok {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if strings.TrimSpace(vals.Objective) == "" {
		errs = append(errs, errRequired(name, "objective"))
	} else if len(vals.Objective) > 140 {
		errs = append(errs, errMaxLength(name, "objective", 140, len(vals.Objective)))
	}

	if strings.TrimSpace(vals.Foundation) == "" {
		errs = append(errs, errRequired(name, "foundation"))
	} else if len(vals.Foundation) > 140 {
		errs = append(errs, errMaxLength(name, "foundation", 140, len(vals.Foundation)))
	}

	if len(vals.Pillars) < 3 {
		errs = append(errs, errMinItems(name, "pillars", 3, len(vals.Pillars), "(hint: use stylish-panels for fewer pillars)"))
	}
	if len(vals.Pillars) > 5 {
		errs = append(errs, errMaxItems(name, "pillars", 5, len(vals.Pillars), ""))
	}

	for i, p := range vals.Pillars {
		titlePath := fmt.Sprintf("pillars[%d].title", i)
		if strings.TrimSpace(p.Title) == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if len(p.Title) > 60 {
			errs = append(errs, errMaxLength(name, titlePath, 60, len(p.Title)))
		}
		if len(p.Body) > 5 {
			errs = append(errs, errMaxItems(name, fmt.Sprintf("pillars[%d].body", i), 5, len(p.Body), ""))
		}
		for j, b := range p.Body {
			bulletPath := fmt.Sprintf("pillars[%d].body[%d]", i, j)
			if b == "" {
				errs = append(errs, errRequired(name, bulletPath))
			} else if len(b) > 120 {
				errs = append(errs, errMaxLength(name, bulletPath, 120, len(b)))
			}
		}
	}

	if len(vals.RoofBadges) > 3 {
		errs = append(errs, errMaxItems(name, "roof_badges", 3, len(vals.RoofBadges), ""))
	}
	for i, badge := range vals.RoofBadges {
		path := fmt.Sprintf("roof_badges[%d]", i)
		if badge == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(badge) > 24 {
			errs = append(errs, errMaxLength(name, path, 24, len(badge)))
		}
	}

	// Total addressable cells for cell_overrides:
	//   0           = objective banner
	//   1..N        = pillar columns
	//   N+1         = foundation
	//   N+2         = roof (when roof_badges present)
	totalCells := 2 + len(vals.Pillars)
	if len(vals.RoofBadges) > 0 {
		totalCells++
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func (sh *strategyHouse) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*StrategyHouseValues)
	if !ok {
		return nil, fmt.Errorf("strategy-house: values must be *StrategyHouseValues, got %T", values)
	}
	ovr := &StrategyHouseOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*StrategyHouseOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("strategy-house: overrides must be *StrategyHouseOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	headerSize := ResolveSize(ovr.HeaderSize, 16.0)
	bodySize := ResolveSize(ovr.BodySize, 12.0)
	cellAccentMode := ovr.CellAccentMode

	numPillars := len(vals.Pillars)
	hasRoof := len(vals.RoofBadges) > 0

	// Cell index map (for cell_overrides):
	//   0 = banner, 1..N = pillars, N+1 = foundation, N+2 = roof (if present)
	bannerIdx := 0
	pillarIdx0 := 1
	foundationIdx := numPillars + 1
	roofIdx := numPillars + 2

	var rows []jsonschema.GridRowInput

	// Optional roof row: badges as a tinted strip
	if hasRoof {
		roofCell := &jsonschema.GridCellInput{
			ColSpan: numPillars,
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt2"`),
				Text:     buildStrategyHouseRoofText(vals.RoofBadges, baseAccent),
			},
		}
		applyStrategyHouseOverride(roofCell, cellOverrides, roofIdx, baseAccent)
		rows = append(rows, jsonschema.GridRowInput{
			Height: 12,
			Cells:  []*jsonschema.GridCellInput{roofCell},
		})
	}

	// Banner row: objective on a strong accent fill
	bannerCell := &jsonschema.GridCellInput{
		ColSpan: numPillars,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent)),
			Text:     buildStrategyHouseBannerText(vals.Objective, headerSize),
		},
	}
	applyStrategyHouseOverride(bannerCell, cellOverrides, bannerIdx, baseAccent)
	bannerHeight := 18.0
	if hasRoof {
		bannerHeight = 16.0
	}
	rows = append(rows, jsonschema.GridRowInput{
		Height: bannerHeight,
		Cells:  []*jsonschema.GridCellInput{bannerCell},
	})

	// Pillar row: N cells, light fills with bold titles and bullet bodies
	pillarCells := make([]*jsonschema.GridCellInput, numPillars)
	for i, p := range vals.Pillars {
		accent := ResolveCellAccent(baseAccent, i, cellAccentMode)
		pillarCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Text:     buildStrategyHousePillarText(p.Title, p.Body, headerSize, bodySize, accent),
			},
			AccentBar: &jsonschema.AccentBarInput{
				Position: "top",
				Color:    accent,
				Width:    4,
			},
		}
		applyStrategyHouseOverride(pillarCells[i], cellOverrides, pillarIdx0+i, accent)
	}
	rows = append(rows, jsonschema.GridRowInput{Cells: pillarCells})

	// Foundation row
	foundationCell := &jsonschema.GridCellInput{
		ColSpan: numPillars,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, baseAccent)),
			Text:     buildStrategyHouseFoundationText(vals.Foundation, headerSize-2),
		},
	}
	applyStrategyHouseOverride(foundationCell, cellOverrides, foundationIdx, baseAccent)
	rows = append(rows, jsonschema.GridRowInput{
		Height: 18,
		Cells:  []*jsonschema.GridCellInput{foundationCell},
	})

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, numPillars)),
		Gap:     8,
		RowGap:  6,
		Rows:    rows,
	}

	return grid, nil
}

// ---------------------------------------------------------------------------
// Text builders
// ---------------------------------------------------------------------------

type strategyHouseParagraph struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold,omitempty"`
	Color   string  `json:"color,omitempty"`
	Align   string  `json:"align,omitempty"`
}

type strategyHouseTextObj struct {
	Paragraphs    []strategyHouseParagraph `json:"paragraphs"`
	Align         string                   `json:"align"`
	VerticalAlign string                   `json:"vertical_align"`
}

func buildStrategyHouseBannerText(objective string, size float64) json.RawMessage {
	textObj := strategyHouseTextObj{
		Paragraphs: []strategyHouseParagraph{
			{Content: pptx.ConvertMarkdownEmphasis(objective), Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildStrategyHouseFoundationText(foundation string, size float64) json.RawMessage {
	textObj := strategyHouseTextObj{
		Paragraphs: []strategyHouseParagraph{
			{Content: pptx.ConvertMarkdownEmphasis(foundation), Size: size, Bold: true, Color: "lt1", Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildStrategyHouseRoofText(badges []string, accent string) json.RawMessage {
	joined := strings.Join(badges, "   ·   ")
	textObj := strategyHouseTextObj{
		Paragraphs: []strategyHouseParagraph{
			{Content: joined, Size: 11, Bold: true, Color: accent, Align: "ctr"},
		},
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

func buildStrategyHousePillarText(title string, body []string, titleSize, bodySize float64, accent string) json.RawMessage {
	paras := make([]strategyHouseParagraph, 0, 1+len(body))
	paras = append(paras, strategyHouseParagraph{
		Content: pptx.ConvertMarkdownEmphasis(title),
		Size:    titleSize,
		Bold:    true,
		Color:   accent,
		Align:   "ctr",
	})
	for _, b := range body {
		paras = append(paras, strategyHouseParagraph{
			Content: "• " + pptx.ConvertMarkdownEmphasis(b),
			Size:    bodySize,
			Color:   "dk1",
			Align:   "l",
		})
	}
	verticalAlign := "ctr"
	bodyAlign := "ctr"
	if len(body) > 0 {
		verticalAlign = "t"
		bodyAlign = "l"
	}
	textObj := strategyHouseTextObj{
		Paragraphs:    paras,
		Align:         bodyAlign,
		VerticalAlign: verticalAlign,
	}
	data, _ := json.Marshal(textObj)
	return data
}

func applyStrategyHouseOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	if cell == nil {
		return
	}
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*StrategyHouseCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    4,
		}
	}
}
