package patterns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// strategy-house pattern — gabled roof (objective) over N pillars over a
// foundation of one or more levels
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&strategyHouse{})
}

type strategyHouse struct{}

func (sh *strategyHouse) Name() string { return "strategy-house" }
func (sh *strategyHouse) Description() string {
	return "Strategy House: gabled roof carrying the objective (and optional badges), an optional cross-cutting beam, 3-5 pillar columns, and a foundation of 1-3 levels (each a band or a row of 2-5 named cells) — take the pillar count and the levels from the content"
}
func (sh *strategyHouse) UseWhen() string {
	return "Strategic framework with a top-level objective, 3-5 pillars supporting it, and a foundation of enablers; one pillar per independent theme and one foundation level per kind of enabler (split a level into cells when the enablers are separate things); prefer arch-stack when layers stack vertically without a single objective/foundation framing, stylish-panels when pillars stand alone without banner/foundation"
}
func (sh *strategyHouse) NotWhen() string {
	return "Layers stack without a single objective and foundation framing (use arch-stack), pillars stand alone without banner/foundation (use stylish-panels), or content is a hierarchy that narrows (use pyramid)"
}
func (sh *strategyHouse) Version() int { return 1 }
func (sh *strategyHouse) CellsHint() string {
	return "objective + 3-5 pillars + 1-3 foundation levels of 1-5 cells (+beam, +0-3 roof badges)"
}
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
	// Four uneven pillars over a two-level foundation: the shape follows the
	// content, so the exemplar is not the 3 x 2 silhouette agents copied
	// whatever they had to say (go-slide-creator-qad87).
	return &StrategyHouseValues{
		Objective: "Become the trusted platform for global commerce",
		Pillars: []StrategyHousePillar{
			{Title: "Customer Trust", Body: []string{"Privacy by default", "Transparent pricing", "Regional data residency"}},
			{Title: "Operational Excellence", Body: []string{"99.99% uptime", "Automated quality gates"}},
			{Title: "Product Velocity", Body: []string{"Weekly releases", "Continuous experimentation"}},
			{Title: "Partner Reach", Body: []string{"Marketplace live in 12 markets"}},
		},
		FoundationLayers: []StrategyHouseLayer{
			{"One operating model in every market"},
			{"People", "Technology", "Data"},
		},
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

// StrategyHouseLayer is one foundation level: a single entry is a full-width
// band, several entries are a row of equal cells.
type StrategyHouseLayer []string

// StrategyHouseValues holds the house top to bottom: optional roof badges and
// the objective in the roof, an optional cross-cutting beam, 3-5 pillar
// columns, and a foundation of one or more levels.
//
// The foundation is authored either as a string — one band, held in
// Foundation — or as a list of levels, each a string (band) or a list of
// short strings (a row of cells), held in FoundationLayers.
type StrategyHouseValues struct {
	Objective        string
	Beam             string
	Pillars          []StrategyHousePillar
	Foundation       string
	FoundationLayers []StrategyHouseLayer
	RoofBadges       []string
}

// strategyHouseValuesJSON is the wire shape of StrategyHouseValues.
type strategyHouseValuesJSON struct {
	Objective  string                `json:"objective"`
	Beam       string                `json:"beam,omitempty"`
	Pillars    []StrategyHousePillar `json:"pillars"`
	Foundation json.RawMessage       `json:"foundation"`
	RoofBadges []string              `json:"roof_badges,omitempty"`
}

// MarshalJSON writes foundation in the form it was authored in.
func (v StrategyHouseValues) MarshalJSON() ([]byte, error) {
	out := strategyHouseValuesJSON{Objective: v.Objective, Beam: v.Beam, Pillars: v.Pillars, RoofBadges: v.RoofBadges}
	if len(v.FoundationLayers) > 0 {
		layers := make([]any, len(v.FoundationLayers))
		for i, l := range v.FoundationLayers {
			if len(l) == 1 {
				layers[i] = l[0]
			} else {
				layers[i] = []string(l)
			}
		}
		out.Foundation, _ = json.Marshal(layers)
	} else {
		out.Foundation, _ = json.Marshal(v.Foundation)
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads foundation as a string or as a list of levels.
func (v *StrategyHouseValues) UnmarshalJSON(data []byte) error {
	var in strategyHouseValuesJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	*v = StrategyHouseValues{Objective: in.Objective, Beam: in.Beam, Pillars: in.Pillars, RoofBadges: in.RoofBadges}
	raw := bytes.TrimSpace(in.Foundation)
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if raw[0] != '[' {
		return json.Unmarshal(raw, &v.Foundation)
	}
	var levels []json.RawMessage
	if err := json.Unmarshal(raw, &levels); err != nil {
		return err
	}
	v.FoundationLayers = make([]StrategyHouseLayer, len(levels))
	for i, level := range levels {
		var band string
		if err := json.Unmarshal(level, &band); err == nil {
			v.FoundationLayers[i] = StrategyHouseLayer{band}
			continue
		}
		var cells []string
		if err := json.Unmarshal(level, &cells); err != nil {
			return fmt.Errorf("foundation[%d] must be a string (a full-width band) or a list of strings (a row of cells)", i)
		}
		v.FoundationLayers[i] = StrategyHouseLayer(cells)
	}
	return nil
}

// foundationLayers returns the foundation as levels, whichever form it was
// authored in.
func (v *StrategyHouseValues) foundationLayers() []StrategyHouseLayer {
	if len(v.FoundationLayers) > 0 {
		return v.FoundationLayers
	}
	if strings.TrimSpace(v.Foundation) == "" {
		return nil
	}
	return []StrategyHouseLayer{{v.Foundation}}
}

// foundationPath is the values path of cell j of foundation level i.
func (v *StrategyHouseValues) foundationPath(i, j int) string {
	if len(v.FoundationLayers) == 0 {
		return "foundation"
	}
	if len(v.FoundationLayers[i]) == 1 {
		return fmt.Sprintf("foundation[%d]", i)
	}
	return fmt.Sprintf("foundation[%d][%d]", i, j)
}

// strategyHouseCells maps the cell_overrides indices:
//
//	0        = roof (the objective)
//	1..N     = pillar columns
//	N+1..    = foundation cells in reading order
//	then     = beam (when present)
//	last     = roof badge line (when roof_badges are present)
//
// A house with one foundation band and no beam keeps the indices it always
// had: N+1 foundation, N+2 roof badges.
type strategyHouseCells struct {
	foundation []int // first index of each foundation level
	beam       int   // -1 when absent
	badges     int   // -1 when absent
	total      int
}

func (v *StrategyHouseValues) cells() strategyHouseCells {
	c := strategyHouseCells{beam: -1, badges: -1}
	next := 1 + len(v.Pillars)
	for _, l := range v.foundationLayers() {
		c.foundation = append(c.foundation, next)
		next += len(l)
	}
	if len(v.FoundationLayers) == 0 && len(c.foundation) == 0 {
		next++ // the required foundation band keeps its index while it is invalid
	}
	if strings.TrimSpace(v.Beam) != "" {
		c.beam = next
		next++
	}
	if len(v.RoofBadges) > 0 {
		c.badges = next
		next++
	}
	c.total = next
	return c
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

// Equal-copy probes against the written size (no run stored below its role
// floor) on every shipped template give an average per-bullet target for
// each layout, every shape keeping the uniform 0.5 cm text margin
// (go-slide-creator-n1muf). A single long bullet can still fit
// when the other pillars stay concise, up to strategyHouseSingleBulletMax.
func strategyHouseBulletBudget(pillars, bullets int, roof bool) int {
	if pillars < 3 || pillars > 5 || bullets < 1 || bullets > 5 {
		return 120
	}
	withoutRoof := [3][6]int{{120, 120, 120, 120, 80, 40}, {120, 120, 115, 77, 52, 38}, {120, 120, 82, 61, 41, 38}}
	withRoof := [3][6]int{{120, 120, 120, 80, 40, 38}, {120, 120, 83, 52, 38, 35}, {120, 120, 62, 41, 38, 25}}
	if roof {
		return withRoof[pillars-3][bullets]
	}
	return withoutRoof[pillars-3][bullets]
}

// strategyHouseSingleBulletMax is the longest single bullet five pillars hold
// beside concise neighbours.
const strategyHouseSingleBulletMax = 62

// strategyHouseBandBudget is the readable objective / foundation length.
const strategyHouseBandBudget = 132

// Foundation and beam limits. More levels or cells than these cannot stay
// readable under 3-5 pillars on one slide.
const (
	strategyHouseMaxLayers    = 3
	strategyHouseMaxCells     = HouseMaxLevelCells
	strategyHouseCellMaxChars = 40
	strategyHouseBandMaxChars = 140
)

func (sh *strategyHouse) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*StrategyHouseValues)
	if !ok || v == nil || len(v.Pillars) < 3 {
		return nil
	}
	var warnings []string
	bands := []struct{ name, text string }{{"objective", v.Objective}, {"beam", v.Beam}}
	for i, layer := range v.foundationLayers() {
		if len(layer) == 1 {
			bands = append(bands, struct{ name, text string }{v.foundationPath(i, 0), layer[0]})
		}
	}
	for _, band := range bands {
		if n := runeLen(band.text); n > strategyHouseBandBudget {
			warnings = append(warnings, fmt.Sprintf("%s: strategy-house %s is %d characters; the band holds about %d readable characters — shorten it", ErrCodeBodyTooLong, band.name, n, strategyHouseBandBudget))
		}
	}
	warnings = append(warnings, strategyHouseShapeWarnings(v)...)
	if w := strategyHouseBulletWarning(v); w != "" {
		return append(warnings, w)
	}
	// The table above is the budget of the classic three-level house. Extra
	// levels take their height from the pillars, so a house the table passes
	// is still measured against the height it really has.
	if w := sh.heightWarning(ctx, v, overrides); w != "" {
		warnings = append(warnings, w)
	}
	return warnings
}

func strategyHouseBulletWarning(v *StrategyHouseValues) string {
	totalBullets, totalChars := 0, 0
	for _, pillar := range v.Pillars {
		for _, bullet := range pillar.Body {
			totalBullets++
			totalChars += runeLen(bullet)
		}
	}
	if totalBullets == 0 {
		return ""
	}
	bulletsPerPillar := (totalBullets + len(v.Pillars) - 1) / len(v.Pillars)
	budget := strategyHouseBulletBudget(len(v.Pillars), bulletsPerPillar, len(v.RoofBadges) > 0)
	if totalChars <= budget*totalBullets {
		if len(v.Pillars) == 5 {
			for i, pillar := range v.Pillars {
				for j, bullet := range pillar.Body {
					if n := runeLen(bullet); n > strategyHouseSingleBulletMax {
						return fmt.Sprintf("%s: strategy-house pillars[%d].body[%d] is %d characters; five pillars hold about %d characters in one bullet — shorten the bullet", ErrCodeBodyTooLong, i, j, n, strategyHouseSingleBulletMax)
					}
				}
			}
		}
		return ""
	}
	return fmt.Sprintf("%s: strategy-house pillars.body averages %.0f characters across %d bullets; this %d-pillar layout holds about %d characters per bullet at its current density — shorten bullet copy, use fewer bullets, or split the house", ErrCodeBodyTooLong, float64(totalChars)/float64(totalBullets), totalBullets, len(v.Pillars), budget)
}

// heightWarning reports a house whose levels need more height than its
// region has: the pillar rows are then squeezed and their text is written
// below its size. It is how a house placed in a region too small for it —
// a narrow compose or regions cell — is reported instead of clipped.
func (sh *strategyHouse) heightWarning(ctx ExpandContext, v *StrategyHouseValues, overrides any) string {
	ovr, _ := overrides.(*StrategyHouseOverrides)
	if ovr == nil {
		ovr = &StrategyHouseOverrides{}
	}
	fullW, areaH := contentAreaPt(ctx)
	m := v.model()
	layout, err := BuildHouse(m, v.style(ctx, ovr, nil), fullW, areaH)
	if err != nil || !layout.RoofFlattened {
		return ""
	}
	if !layout.Tight {
		return fmt.Sprintf("%s: strategy-house fills this %.0fpt-high region with its %d levels, so the roof is flattened to a %.0fpt gable and no longer reads as a roof — shorten the pillars, drop a level, or give the house a taller region", ErrCodeBodyTooLong, areaH, len(m.Levels), layout.RoofRisePt)
	}
	return fmt.Sprintf("%s: strategy-house needs about %.0fpt of height for its roof and %d levels but this region has %.0fpt, so the pillar text is written smaller — shorten the pillars, drop a level, or give the house a taller region", ErrCodeBodyTooLong, layout.NeedPt, len(m.Levels), areaH)
}

// ErrCodeHouseShapeForced reports a house whose content was pressed into the
// default one-band, equal-pillar shape (go-slide-creator-qad87).
const ErrCodeHouseShapeForced = "HOUSE_SHAPE_FORCED"

// strategyHouseListSplit separates the items of a joined list.
var strategyHouseListSplit = regexp.MustCompile(`\s*[·•|;,/]\s*|\s+\+\s+|\s+&\s+`)

// strategyHouseJoinedItems returns the short items a band string packs
// together ("People · Technology · Data"), or nil when it reads as one
// statement.
func strategyHouseJoinedItems(band string) []string {
	parts := strategyHouseListSplit.Split(strings.TrimSpace(band), -1)
	if len(parts) < 3 {
		return nil
	}
	for _, p := range parts {
		if p == "" || runeLen(p) > 30 || len(strings.Fields(p)) > 4 {
			return nil
		}
	}
	return parts
}

// strategyHouseShapeWarnings nudges when the content was forced into the
// default silhouette: separate enablers joined into one foundation string,
// or a pillar left empty beside full ones.
func strategyHouseShapeWarnings(v *StrategyHouseValues) []string {
	var out []string
	for i, layer := range v.foundationLayers() {
		if len(layer) != 1 {
			continue
		}
		items := strategyHouseJoinedItems(layer[0])
		if len(items) == 0 || len(items) > strategyHouseMaxCells {
			continue
		}
		quoted, _ := json.Marshal(items)
		out = append(out, fmt.Sprintf("%s: strategy-house %s joins %d separate items into one band; give each its own box by writing the level as a list — \"foundation\": [%s] — or keep the band if they are one idea", ErrCodeHouseShapeForced, v.foundationPath(i, 0), len(items), quoted))
	}
	most := 0
	for _, p := range v.Pillars {
		most = max(most, len(p.Body))
	}
	if most >= 3 {
		for i, p := range v.Pillars {
			if len(p.Body) == 0 {
				out = append(out, fmt.Sprintf("%s: strategy-house pillars[%d] (%q) has no body while other pillars carry up to %d bullets; give it its own points, merge it into a neighbouring pillar, or move it to a beam or foundation level", ErrCodeHouseShapeForced, i, p.Title, most))
			}
		}
	}
	return out
}

func (sh *strategyHouse) Schema() *Schema {
	pillarSchema := ObjectSchema(
		map[string]*Schema{
			"title": StringSchema(60).WithDescription("Pillar title"),
			"body":  ArraySchema(StringSchema(120), 0, 5).WithDescription("Pillar bullet items (0-5); pillars need not match in count — write what each theme has; dense average targets depend on pillar count, bullet count and roof badges: three pillars hold about 80 characters per bullet at four (40 at five; one fewer bullet each with roof badges), four pillars 115/77/52/38 at two/three/four/five (83/52/38/35 with badges), five pillars 82/61/41/38 (62/41/38/25 with badges); one bullet beside concise neighbours in five pillars holds about 62"),
		},
		[]string{"title"},
	).WithAdditionalProperties(false).WithDescription("Pillar column with title and optional bullet body")

	layerSchema := OneOfSchema(
		StringSchema(strategyHouseBandMaxChars).WithDescription("Full-width band; about 132 readable characters"),
		ArraySchema(StringSchema(strategyHouseCellMaxChars), 1, strategyHouseMaxCells).WithDescription("Row of 2-5 equal cells, one short label each (a single entry is a band)"),
	)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"objective": StringSchema(strategyHouseBandMaxChars).WithDescription("Strategic objective, drawn in the gabled roof above the pillars; about 132 readable characters"),
			"beam":      StringSchema(strategyHouseBandMaxChars).WithDescription("Optional cross-cutting band between the roof and the pillars (a principle or constraint every pillar shares); about 132 readable characters"),
			"pillars":   ArraySchema(pillarSchema, 3, 5).WithDescription("3-5 pillar columns supporting the objective: one pillar per independent theme in the content — do not merge or pad themes to reach three"),
			"foundation": OneOfSchema(
				StringSchema(strategyHouseBandMaxChars).WithDescription("One foundation band under the pillars; about 132 readable characters"),
				ArraySchema(layerSchema, 1, strategyHouseMaxLayers).WithDescription("1-3 foundation levels, top to bottom: one level per kind of enabler, each a string (band) or a list of short strings (a row of cells)"),
			).WithDescription("Foundation under the pillars: a string for one band, or a list of 1-3 levels where each level is a string (full-width band) or a list of 2-5 short strings (a row of equal cells, e.g. [\"Shared platform\", [\"People\", \"Data\", \"Controls\"]]); split a level into cells when the enablers are separate things rather than joining them with separators"),
			"roof_badges": ArraySchema(StringSchema(24), 0, 3).WithDescription("Optional badges drawn inside the roof above the objective (0-3, e.g. vision/mission tags)"),
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
	}).WithDescription("Strategy House: gabled roof with the objective above 3-5 pillars above a foundation of 1-3 levels, with an optional beam and roof badges")
}

func (sh *strategyHouse) Validate(values, overrides any, cellOverrides map[int]any) error { //nolint:gocognit,gocyclo // one block per field
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
	} else if runeLen(vals.Objective) > strategyHouseBandMaxChars {
		errs = append(errs, errMaxLength(name, "objective", strategyHouseBandMaxChars, runeLen(vals.Objective)))
	}
	if runeLen(vals.Beam) > strategyHouseBandMaxChars {
		errs = append(errs, errMaxLength(name, "beam", strategyHouseBandMaxChars, runeLen(vals.Beam)))
	}

	layers := vals.foundationLayers()
	if len(layers) == 0 {
		errs = append(errs, errRequired(name, "foundation"))
	}
	if len(layers) > strategyHouseMaxLayers {
		errs = append(errs, errMaxItems(name, "foundation", strategyHouseMaxLayers, len(layers), "(hint: a house stays readable with at most 3 foundation levels; merge levels or move one to the beam)"))
	}
	counts := []int{len(vals.Pillars)}
	for i, layer := range layers {
		levelPath := fmt.Sprintf("foundation[%d]", i)
		if len(vals.FoundationLayers) == 0 {
			levelPath = "foundation"
		}
		if len(layer) == 0 {
			errs = append(errs, errMinItems(name, levelPath, 1, 0, "(hint: a level is a string or a list of 2-5 short strings)"))
			continue
		}
		if len(layer) > strategyHouseMaxCells {
			errs = append(errs, errMaxItems(name, levelPath, strategyHouseMaxCells, len(layer), "(hint: a level split into more than 5 cells cannot keep its labels readable; group the cells or use two levels)"))
			continue
		}
		counts = append(counts, len(layer))
		maxChars := strategyHouseBandMaxChars
		if len(layer) > 1 {
			maxChars = strategyHouseCellMaxChars
		}
		for j, cell := range layer {
			path := vals.foundationPath(i, j)
			if strings.TrimSpace(cell) == "" {
				errs = append(errs, errRequired(name, path))
			} else if runeLen(cell) > maxChars {
				errs = append(errs, errMaxLength(name, path, maxChars, runeLen(cell)))
			}
		}
	}

	if len(vals.Pillars) < 3 {
		errs = append(errs, errMinItems(name, "pillars", 3, len(vals.Pillars), "(hint: use stylish-panels for fewer pillars)"))
	}
	if len(vals.Pillars) > 5 {
		errs = append(errs, errMaxItems(name, "pillars", 5, len(vals.Pillars), ""))
	} else if len(vals.Pillars) >= 3 && !HouseColumnsFit(counts...) {
		errs = append(errs, &ValidationError{
			Pattern: name, Path: "foundation", Code: ErrCodeCountMismatch,
			Message: fmt.Sprintf("%s: %d pillars over foundation levels of %s cells cannot share one column grid; give the split levels the same cell count, or a count that divides evenly with the pillars", name, len(vals.Pillars), strategyHouseCountList(counts[1:])),
		})
	}

	for i, p := range vals.Pillars {
		titlePath := fmt.Sprintf("pillars[%d].title", i)
		if strings.TrimSpace(p.Title) == "" {
			errs = append(errs, errRequired(name, titlePath))
		} else if runeLen(p.Title) > 60 {
			errs = append(errs, errMaxLength(name, titlePath, 60, runeLen(p.Title)))
		}
		if len(p.Body) > 5 {
			errs = append(errs, errMaxItems(name, fmt.Sprintf("pillars[%d].body", i), 5, len(p.Body), ""))
		}
		for j, b := range p.Body {
			bulletPath := fmt.Sprintf("pillars[%d].body[%d]", i, j)
			if b == "" {
				errs = append(errs, errRequired(name, bulletPath))
			} else if runeLen(b) > 120 {
				errs = append(errs, errMaxLength(name, bulletPath, 120, runeLen(b)))
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
		} else if runeLen(badge) > 24 {
			errs = append(errs, errMaxLength(name, path, 24, runeLen(badge)))
		}
	}

	if coErr := validateCellOverrideKeys(name, cellOverrides, vals.cells().total, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

func strategyHouseCountList(counts []int) string {
	parts := make([]string, len(counts))
	for i, n := range counts {
		parts[i] = fmt.Sprintf("%d", n)
	}
	return strings.Join(parts, " and ")
}

// model maps the values onto the shared house model: the roof, then the
// beam, the pillars and the foundation levels, top to bottom.
func (v *StrategyHouseValues) model() HouseModel {
	cells := v.cells()
	m := HouseModel{Roof: v.Objective, Badges: v.RoofBadges, RoofOverrideIndex: 0, BadgeOverrideIndex: cells.badges}
	if cells.beam >= 0 {
		m.Levels = append(m.Levels, HouseLevel{Kind: HouseBand, Cells: []HouseCell{{Title: v.Beam}}, OverrideIndex: cells.beam})
	}
	pillars := HouseLevel{Kind: HousePillars, OverrideIndex: 1}
	for _, p := range v.Pillars {
		pillars.Cells = append(pillars.Cells, HouseCell(p))
	}
	m.Levels = append(m.Levels, pillars)
	for i, layer := range v.foundationLayers() {
		level := HouseLevel{Kind: HouseBand, OverrideIndex: cells.foundation[i]}
		for _, text := range layer {
			level.Cells = append(level.Cells, HouseCell{Title: text})
		}
		m.Levels = append(m.Levels, level)
	}
	return m
}

// style resolves the house's look from the template and the overrides.
func (v *StrategyHouseValues) style(ctx ExpandContext, ovr *StrategyHouseOverrides, cellOverrides map[int]any) HouseStyle {
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	st := HouseStyle{
		Fonts:  ctx.themeFonts(),
		Accent: baseAccent,
		PillarAccent: func(i int) string {
			return ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		},
		// The pillar surface used to be lt1 — white on a white slide — so a
		// column was invisible below its last bullet (go-slide-creator-pr3g).
		// Generic light roles become the neutral 4% step (go-slide-creator-8xsj3).
		PillarSurface: tonalPanel(ctx, baseAccent).fillJSON(),
		HeaderPt:      ResolveSize(ovr.HeaderSize, sizeHeaderPt),
		BodyPt:        ResolveSize(ovr.BodySize, scaleBodyPt),
		// The house picks its own type step while the author picked none.
		Grow:     ovr.HeaderSize == 0 && ovr.BodySize == 0,
		ColGapPt: ctx.Gap(strategyHouseColGapPt),
		RowGapPt: ctx.Gap(houseRowGapPt),
		Override: func(idx int, cell *jsonschema.GridCellInput, accent string) {
			applyStrategyHouseOverride(cell, cellOverrides, idx, accent)
		},
	}
	HouseTonalStyle(ctx, &st)
	return st
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

	// The house is content-sized (go-slide-creator-wntyw): the roof's eaves
	// band and every band level are pinned to their text, the pillar row hugs
	// its tallest column, and the block is middle-anchored in the body zone.
	fullW, areaH := contentAreaPt(ctx)
	layout, err := BuildHouse(vals.model(), vals.style(ctx, ovr, cellOverrides), fullW, areaH)
	if err != nil {
		return nil, fmt.Errorf("strategy-house: %w", err)
	}
	return layout.Grid, nil
}

// strategyHouseColGapPt is the gutter between pillars.
const strategyHouseColGapPt = houseColGapPt

// applyStrategyHouseOverride applies one cell_overrides entry: the text keys
// and the accent bar (on the roof, a thin rule under the eaves).
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
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar && cell.Shape != nil && cell.Shape.Geometry == "rect" {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "top",
			Color:    accent,
			Width:    4,
		}
	}
}
