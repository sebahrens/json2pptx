package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textfit"
)

// ---------------------------------------------------------------------------
// radial-hub pattern — one central idea with 4-8 spokes to labelled satellites
// ---------------------------------------------------------------------------
//
// "Everything relates to the centre": an operating-model hub, a platform and
// the capabilities around it, a stakeholder map, an ecosystem. The items are
// peers of one another and have no order, so the drawing carries no numbers
// and no arrows: a solid accent hub circle, one thin spoke per item and a
// neutral satellite disc at the end of each spoke. A spoke's optional icon is
// drawn in its disc (a layer icon), in the ink measured on the disc's fill; in
// the legend it is also the item's key, in place of the letter.
//
// The hub, the spokes and the satellites are layers of ONE shape-grid cell
// whose fitted bounds are a square (ring_common.go), so they stay round in a
// compose segment or a nested cell. The labels are lattice cells:
//
//	outside (default)  labels left and right of the ring, each on the row of
//	                   its satellite and at one constant gap from that
//	                   satellite's disc (the ring cell is then the spine
//	                   column, ringSpinePlacement); an item at 12 or 6
//	                   o'clock (odd counts) is labelled above / below the ring
//	inside             no label cells: larger satellites hold a short label
//	legend             the ring on the left and a keyed list (A, B, C ...)
//	                   on the right; the default falls back to it when the
//	                   area is too narrow for two label columns
//
// The satellites sit at the middle of the segments of a ring that starts at
// 12 o'clock, so four items sit on the diagonals and six leave the poles
// free: an even count needs no label above or below the ring.

func init() {
	Default().Register(&radialHub{})
}

type radialHub struct{}

func (p *radialHub) Name() string { return rhName }
func (p *radialHub) Description() string {
	return "Central accent hub circle with 4-8 spokes to labelled satellites, each with an optional icon in its disc (hub-and-spoke: operating-model hub, platform and capabilities, stakeholder map, ecosystem); no sequence, no numbers; labels beside the ring, inside the satellites or as a keyed legend"
}
func (p *radialHub) UseWhen() string {
	return "One central idea that 4-8 peer items all relate to, with no order among them: a platform and the capabilities around it, an operating-model hub, a stakeholder or ecosystem map. Each item takes a short label and an optional one-line description; prefer cycle-ring or cycle-nodes when the items follow one another in a loop, state-shift-hub for today/future pairs around a theme, and icon-row or card-grid when there is no centre"
}
func (p *radialHub) NotWhen() string {
	return "The items form an ordered loop (use cycle-ring or cycle-nodes), today/future pairs (use state-shift-hub), nested scopes (use concentric-rings), a hierarchy that decomposes a metric (use driver-tree), peers without a centre (use icon-row or card-grid), or there are fewer than 4 or more than 8 items"
}
func (p *radialHub) Version() int      { return 1 }
func (p *radialHub) CellsHint() string { return "1 hub + 4-8 satellites" }
func (p *radialHub) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"framework-grid", "arch-stack", "exec-summary"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}
func (p *radialHub) SupportsInlineMarkdown() bool { return true }

func (p *radialHub) ExemplarValues() any {
	return &RadialHubValues{
		Center: RadialHubCenter{Label: "Customer data platform", Sublabel: "One governed source"},
		Spokes: []RadialHubSpoke{
			{Label: "Marketing", Description: "Audiences and campaign attribution", Icon: &IconRef{Name: "speakerphone"}},
			{Label: "Sales", Description: "Account insight and next best action", Icon: &IconRef{Name: "trending-up"}},
			{Label: "Service", Description: "Full history at every contact", Icon: &IconRef{Name: "headset"}},
			{Label: "Finance", Description: "Billing and revenue assurance", Icon: &IconRef{Name: "report-money"}},
			{Label: "Risk and compliance", Description: "Consent, retention and audit trail", Icon: &IconRef{Name: "shield-check"}},
			{Label: "Product", Description: "Usage signals for roadmap choices", Icon: &IconRef{Name: "bulb"}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// RadialHubCenter is the hub: a short label and an optional line under it.
type RadialHubCenter struct {
	Label    string `json:"label"`
	Sublabel string `json:"sublabel,omitempty"`
}

// RadialHubSpoke is one satellite.
type RadialHubSpoke struct {
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Icon        *IconRef `json:"icon,omitempty"` // drawn in the satellite disc
}

// RadialHubValues holds the hub, the 4-8 satellites and the optional
// highlighted satellite (1-based; 0 = none).
type RadialHubValues struct {
	Center    RadialHubCenter  `json:"center"`
	Spokes    []RadialHubSpoke `json:"spokes"`
	Highlight int              `json:"highlight,omitempty"`
}

// RadialHubOverrides are the pattern-level overrides. header_size sizes the
// satellite labels and body_size the descriptions.
type RadialHubOverrides struct {
	TextOverrides
	Labels string `json:"labels,omitempty"` // outside (default) | inside | legend
	Spokes string `json:"spokes,omitempty"` // lines (default) | none
}

func (p *radialHub) NewValues() any       { return &RadialHubValues{} }
func (p *radialHub) NewOverrides() any    { return &RadialHubOverrides{} }
func (p *radialHub) NewCellOverride() any { return nil }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	rhName = "radial-hub"

	rhMinSpokes       = 4
	rhMaxSpokes       = 8
	rhMaxInsideSpokes = 6 // larger satellites: more would touch

	rhHubLabelMax    = 24
	rhHubSublabelMax = 32
	rhLabelMax       = 26
	rhDescriptionMax = 60
	rhInsideLabelMax = 14

	rhLabelsOutside = "outside"
	rhLabelsInside  = "inside"
	rhLabelsLegend  = "legend"
	rhSpokesLines   = "lines"
	rhSpokesNone    = "none"

	// Diameters as fractions of the ring square's side. The satellites touch
	// the square's edge, so their centres sit on a circle of radius
	// 0.5 - diameter/2 and a spoke runs from the hub's edge to the satellite's.
	rhHubDia       = 0.42
	rhSatDia       = 0.19
	rhInsideHubDia = 0.32
	rhInsideSatDia = 0.27
	rhLegendSatDia = 0.18
	// rhNodeSatDia is a satellite of the outside layout when no spoke carries
	// an icon: a node dot at the spoke's end. A full disc with nothing in it
	// reads as a missing icon (go-slide-creator-8hwcw).
	rhNodeSatDia = 0.07
	rhMinSpoke   = 0.07 // shortest spoke a grown hub leaves
	// rhCircleFitFrac is the share of a circle's text square its text is
	// fitted to.
	rhCircleFitFrac = 0.97
	rhWordSlack     = 1.05 // a word must clear its circle's text width by this much
	rhHubStep       = 0.02

	rhLabelPt    = scaleSubheadPt // satellite label
	rhBodyPt     = scaleBodyPt    // description, hub sublabel, text inside a satellite
	rhHubPt      = 16.0           // hub label ceiling; steps down to the floor to fit the circle
	rhSpokePt    = 1.5            // spoke stroke
	rhSpokeHiPt  = 3.0            // the highlighted item's spoke
	rhLabelGapPt = ringLabelGapPt // satellite disc (legend: ring square) to its label
	rhRowGapPt   = 4.0            // least gap between two label rows
	rhPoleGapPt  = 4.0            // ring square to the label above / below it
	rhMinLabelPt = 110.0          // narrowest label column that still reads
	rhKeyColPt   = 22.0           // legend key letter column
	rhLegendFrac = 0.50           // ring's share of the width in legend mode
	rhCircleInPt = 2.0            // text inset inside a circle (its text box is already the inscribed square)
	rhSpokeTone  = 35             // dk1 coverage of the spokes that are not the highlighted one
	rhLabelTitle = 1.0            // space after a label above its description
	// A label cell is open text beside the ring: it keeps no side margin (the
	// column gap separates it from the ring and its outer edge is the content
	// edge) and a small one above and below, so eight rows fit a short body.
	rhLabelInsetTBPt = 3.0
	// rhIconScale is a satellite icon's side as a share of its disc's
	// diameter: inside the circle's inscribed square (0.71) with air around it.
	rhIconScale = 0.55
	// rhKeyIconScale is a legend key icon's side as a share of the key column.
	rhKeyIconScale = 0.7
)

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (p *radialHub) Schema() *Schema {
	spoke := ObjectSchema(map[string]*Schema{
		"label":       StringSchema(rhLabelMax).WithDescription("Satellite label, e.g. \"Risk and compliance\" (bold; one line reads best, at most 14 characters with labels inside)"),
		"description": StringSchema(rhDescriptionMax).WithDescription("Optional one-line description under the label (not shown with labels inside). Readable budget by measurement: about 60 characters with 4-6 spokes and 40 with 7-8; expand_pattern reports BODY_TOO_LONG when a label row outgrows its room"),
		"icon":        IconRefSchema("Optional icon in the satellite disc: bundled name (e.g. \"shield-check\") or {name|path|url|svg_data, fill?, alt?} (not with labels inside; the legend key becomes the icon)"),
	}, []string{"label"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"center": ObjectSchema(map[string]*Schema{
			"label":    StringSchema(rhHubLabelMax).WithDescription("Hub label inside the central circle, e.g. \"Customer data platform\" (short words: a word that cannot fit the circle on one line is reported)"),
			"sublabel": StringSchema(rhHubSublabelMax).WithDescription("Optional line under the hub label"),
		}, []string{"label"}).WithAdditionalProperties(false).WithDescription("The central idea every spoke relates to"),
		"spokes":    ArraySchema(RefSchema("spoke"), rhMinSpokes, rhMaxSpokes).WithDescription("4-8 peer items around the hub, clockwise from 12 o'clock; they have no order (use cycle-ring for a loop)"),
		"highlight": IntegerSchema(0, rhMaxSpokes).WithDescription("1-based index of one spoke to emphasise (accent-tinted satellite, heavier spoke, accent label); 0 = none"),
	}, []string{"center", "spokes"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":           StringSchema(0).WithDescription("Accent scheme color of the hub and spokes (default: the template's primary fill)").WithDefault("accent1"),
		"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"header_size":      NumberSchema(12, 24).WithDescription("Satellite label font size in points (default 14)"),
		"body_size":        NumberSchema(12, 20).WithDescription("Description font size in points (default 12)"),
		"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Satellite fills: uniform (default, accent tint), alternate (accent / next accent), progressive (walks accent1-6)").WithDefault("uniform"),
		"labels":           EnumSchema(rhLabelsOutside, rhLabelsInside, rhLabelsLegend).WithDescription("Where the labels go: outside (default; each label beside its satellite, legend in a narrow area), inside (label in a larger satellite: 4-6 spokes, labels of at most 14 characters in words of about 8, no descriptions), legend (ring on the left, keyed list A, B, C ... on the right)").WithDefault(rhLabelsOutside),
		"spokes":           EnumSchema(rhSpokesLines, rhSpokesNone).WithDescription("lines (default) draws a thin spoke from the hub to each satellite; none leaves them unconnected").WithDefault(rhSpokesLines),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"spoke":        spoke,
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Central hub circle with 4-8 spokes to labelled satellites (hub-and-spoke, no sequence)")
}

func rhOverrides(overrides any) *RadialHubOverrides {
	if o, ok := overrides.(*RadialHubOverrides); ok && o != nil {
		return o
	}
	return &RadialHubOverrides{}
}

func (p *radialHub) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*RadialHubValues)
	if !ok || v == nil {
		return fmt.Errorf("radial-hub: values must be *RadialHubValues, got %T", values)
	}
	var errs []error
	if overrides != nil {
		if _, ok := overrides.(*RadialHubOverrides); !ok {
			errs = append(errs, fmt.Errorf("radial-hub: overrides must be *RadialHubOverrides, got %T", overrides))
		}
	}
	ovr := rhOverrides(overrides)

	if strings.TrimSpace(v.Center.Label) == "" {
		errs = append(errs, errRequired(rhName, "center.label"))
	} else if n := runeLen(v.Center.Label); n > rhHubLabelMax {
		errs = append(errs, errMaxLength(rhName, "center.label", rhHubLabelMax, n))
	}
	if n := runeLen(v.Center.Sublabel); n > rhHubSublabelMax {
		errs = append(errs, errMaxLength(rhName, "center.sublabel", rhHubSublabelMax, n))
	}

	if len(v.Spokes) < rhMinSpokes {
		errs = append(errs, errMinItems(rhName, "spokes", rhMinSpokes, len(v.Spokes), "(hint: use kpi-3up, icon-row or hero-detail for fewer than 4 items around one idea)"))
	}
	if len(v.Spokes) > rhMaxSpokes {
		errs = append(errs, errMaxItems(rhName, "spokes", rhMaxSpokes, len(v.Spokes), "(hint: group related items, or use card-grid for up to 12 peers)"))
	}

	if !map[string]bool{"": true, rhLabelsOutside: true, rhLabelsInside: true, rhLabelsLegend: true}[ovr.Labels] {
		errs = append(errs, newValidationError(rhName, "overrides.labels", "invalid_enum",
			fmt.Sprintf("radial-hub: overrides.labels must be one of outside, inside, legend; got %q", ovr.Labels),
			UseOneOfFix("overrides.labels", []string{rhLabelsOutside, rhLabelsInside, rhLabelsLegend})))
	}
	if !map[string]bool{"": true, rhSpokesLines: true, rhSpokesNone: true}[ovr.Spokes] {
		errs = append(errs, newValidationError(rhName, "overrides.spokes", "invalid_enum",
			fmt.Sprintf("radial-hub: overrides.spokes must be one of lines, none; got %q", ovr.Spokes),
			UseOneOfFix("overrides.spokes", []string{rhSpokesLines, rhSpokesNone})))
	}
	if err := ValidateCellAccentMode(rhName, ovr.CellAccentMode); err != nil {
		errs = append(errs, err)
	}

	errs = append(errs, rhValidateSpokes(v.Spokes, ovr.Labels == rhLabelsInside)...)
	if v.Highlight < 0 || v.Highlight > max(len(v.Spokes), rhMinSpokes) {
		errs = append(errs, errOutOfRange(rhName, "highlight", 0, len(v.Spokes), v.Highlight))
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(rhName, "cell_overrides", ErrCodeUnknownKey,
			"radial-hub: cell_overrides are not supported (use overrides)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// rhValidateSpokes checks every spoke's label, description and icon; inside is
// true when the satellites carry the labels.
func rhValidateSpokes(spokes []RadialHubSpoke, inside bool) []error {
	var errs []error
	if inside && len(spokes) > rhMaxInsideSpokes {
		errs = append(errs, errMaxItems(rhName, "spokes", rhMaxInsideSpokes, len(spokes), "(hint: labels inside the satellites hold at most 6 spokes; use overrides.labels outside for 7-8)"))
	}
	labelMax := rhLabelMax
	if inside {
		labelMax = rhInsideLabelMax
	}
	for i, s := range spokes {
		path := fmt.Sprintf("spokes[%d].label", i)
		switch n := runeLen(s.Label); {
		case strings.TrimSpace(s.Label) == "":
			errs = append(errs, errRequired(rhName, path))
		case n > labelMax:
			errs = append(errs, errMaxLength(rhName, path, labelMax, n))
		}
		path = fmt.Sprintf("spokes[%d].description", i)
		switch n := runeLen(s.Description); {
		case inside && strings.TrimSpace(s.Description) != "":
			errs = append(errs, newValidationError(rhName, path, ErrCodeUnknownKey,
				fmt.Sprintf("radial-hub: %s is not shown with overrides.labels inside (a satellite holds its label only); remove it or use labels outside", path),
				RemoveFieldFix(path)))
		case n > rhDescriptionMax:
			errs = append(errs, errMaxLength(rhName, path, rhDescriptionMax, n))
		}
		if s.Icon == nil {
			continue
		}
		path = fmt.Sprintf("spokes[%d].icon", i)
		if inside && !s.Icon.IsEmpty() {
			errs = append(errs, newValidationError(rhName, path, ErrCodeUnknownKey,
				fmt.Sprintf("radial-hub: %s is not shown with overrides.labels inside (a satellite holds its label only); remove it or use labels outside", path),
				RemoveFieldFix(path)))
			continue
		}
		errs = append(errs, validateIconRef(rhName, path, *s.Icon)...)
	}
	return errs
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// rhRect is a lattice rectangle in points from the block's top-left corner.
type rhRect struct {
	x0, x1, y0, y1 float64
}

func (r rhRect) w() float64 { return r.x1 - r.x0 }
func (r rhRect) h() float64 { return r.y1 - r.y0 }

// rhItem is the resolved geometry of one satellite and its label.
type rhItem struct {
	ring   ringItem
	side   string  // right | left | top | bottom (outside), "" otherwise
	text   rhRect  // the label cell; zero in inside mode
	need   float64 // written height the label needs at text.w()
	inside bool    // false when a label set inside the satellite does not fit it
}

// rhLayout carries every measurement Expand and PostExpandWarnings share.
type rhLayout struct {
	mode          string  // outside | inside | legend (resolved: the default may fall back to legend)
	width, height float64 // the block
	ring          rhRect  // the ring square
	spec          ringSpec
	hubDia        float64 // fractions of the ring square's side
	satDia        float64
	hubSize       float64 // hub label size
	hubFits       bool
	labelSize     float64
	bodySize      float64
	items         []rhItem
}

func (l rhLayout) side() float64 { return l.ring.w() }

// ringPlacement places the ring cell. Outside labels follow the ring into its
// bounding square, so the cell is the spine column there; the other layouts
// keep the fit-contain square.
func (l rhLayout) ringPlacement(layers []jsonschema.LayerInput) ringPlacement {
	cell := &jsonschema.GridCellInput{Fit: "contain", Layers: layers}
	if l.mode == rhLabelsOutside {
		return ringSpinePlacement((l.ring.x0+l.ring.x1)/2, l.ring.y0, l.side(), cell)
	}
	return ringPlacement{X0: l.ring.x0, X1: l.ring.x1, Y0: l.ring.y0, Y1: l.ring.y1, Cell: cell}
}

// gapEdges are the x edges of the outside layout's side labels that face
// their satellites (ringProtectEdges); none in the other layouts.
func (l rhLayout) gapEdges() []float64 {
	if l.mode != rhLabelsOutside {
		return nil
	}
	var edges []float64
	for _, it := range l.items {
		switch it.side {
		case ringSideLeft:
			edges = append(edges, it.text.x1)
		case ringSideRight:
			edges = append(edges, it.text.x0)
		}
	}
	return edges
}

// rhMeasure resolves the layout for the content area of ctx.
func rhMeasure(ctx ExpandContext, v *RadialHubValues, ovr *RadialHubOverrides) (rhLayout, error) {
	w, h := sizingAreaPt(ctx)
	lay := rhLayout{
		mode:      ovr.Labels,
		width:     w,
		height:    h,
		hubDia:    rhHubDia,
		satDia:    rhSatDia,
		labelSize: shapegrid.EffectiveTextSizePt(ResolveSize(ovr.HeaderSize, rhLabelPt)),
		bodySize:  shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, rhBodyPt)),
	}
	if lay.mode == "" {
		lay.mode = rhLabelsOutside
	}
	// The default gives way to the legend when the area is too narrow for two
	// label columns, or leaves a ring whose hub cannot hold its label: the
	// legend's ring takes half the width.
	if lay.mode == rhLabelsOutside {
		out := lay
		if !v.hasIcons() {
			out.satDia = rhNodeSatDia
		}
		if rhPlaceOutside(ctx, v, &out) && out.fitHub(ctx, v) {
			return out, nil
		}
		lay.mode = rhLabelsLegend
	}
	var err error
	if lay.mode == rhLabelsInside {
		lay.hubDia, lay.satDia = rhInsideHubDia, rhInsideSatDia
		err = rhPlaceInside(ctx, v, &lay)
	} else {
		lay.satDia = rhLegendSatDia
		err = rhPlaceLegend(ctx, v, &lay)
	}
	if err != nil {
		return lay, err
	}
	lay.fitHub(ctx, v)
	return lay, nil
}

// hasIcons reports whether any spoke carries an icon.
func (v *RadialHubValues) hasIcons() bool {
	for _, s := range v.Spokes {
		if s.Icon != nil && !s.Icon.IsEmpty() {
			return true
		}
	}
	return false
}

// fitHub sizes the hub label and, when the label does not fit the default
// hub at the readable floor, grows the hub towards the satellites (the spokes
// shorten, down to rhMinSpoke) until it does. It reports whether the label
// fits.
func (l *rhLayout) fitHub(ctx ExpandContext, v *RadialHubValues) bool {
	lines := []rhCircleLine{{text: v.Center.Label, bold: true}, {text: v.Center.Sublabel, size: l.bodySize}}
	maxDia := 2 * (l.spec.Radius - l.satDia/2 - rhMinSpoke)
	for dia := l.hubDia; ; dia += rhHubStep {
		dia = math.Min(dia, math.Max(maxDia, l.hubDia))
		if l.hubSize, l.hubFits = rhFitCircleText(ctx, dia*l.side(), rhHubPt, lines...); l.hubFits || dia >= maxDia {
			if l.hubFits {
				l.hubDia = dia
			}
			return l.hubFits
		}
	}
}

// rhRingItems is the satellites' angles: the middles of the N segments of a
// ring that starts at 12 o'clock, on the circle the satellites of diameter
// satDia touch the square's edge from.
func rhRingItems(n int, satDia float64) (ringSpec, []ringItem, error) {
	spec := newRingSpec(n)
	spec.GapDeg = 0
	spec = spec.withBand(1, satDia)
	items, err := spec.items()
	return spec, items, err
}

// rhSideWordPt is the widest word of the labels set in the two label columns
// (ringLabelWordPt). A column is never narrower than it: the ring gives the
// columns that width, down to its own smallest side, before a word breaks
// across two lines (go-slide-creator-y21pc).
func rhSideWordPt(ctx ExpandContext, v *RadialHubValues, items []ringItem, labelPt float64) float64 {
	wordPt := 0.0
	for i, it := range items {
		if it.Side == ringSideLeft || it.Side == ringSideRight {
			wordPt = math.Max(wordPt, ringLabelWordPt(ctx, v.Spokes[i].Label, labelPt))
		}
	}
	return wordPt
}

// rhPlaceOutside lays the ring square between two label columns. It reports
// false when the area is too narrow for them (or the count is not a ring's).
func rhPlaceOutside(ctx ExpandContext, v *RadialHubValues, lay *rhLayout) bool {
	spec, items, err := rhRingItems(len(v.Spokes), lay.satDia)
	if err != nil {
		return false
	}
	colGap, w, h := ctx.Gap(rhLabelGapPt), lay.width, lay.height
	maxSide := w - 2*(rhMinLabelPt+colGap)
	if math.Min(maxSide, h) < ringMinSidePt {
		return false
	}
	if wordPt := rhSideWordPt(ctx, v, items, lay.labelSize); wordPt > rhMinLabelPt {
		maxSide = math.Max(w-2*(math.Ceil(wordPt)+colGap), ringMinSidePt)
	}
	// An item at a pole is labelled above / below the ring, in a cell as wide
	// as the square: the square gives up the height that label needs, and the
	// label is measured again at the narrower square.
	var top, bottom float64
	side := math.Min(maxSide, h)
	for pass := 0; pass < 3; pass++ {
		top, bottom = 0, 0
		for i, it := range items {
			need := rhLabelNeedPt(ctx, *lay, v.Spokes[i], "ctr", false, side) + ctx.Gap(rhPoleGapPt)
			switch it.Side {
			case ringSideTop:
				top = math.Max(top, need)
			case ringSideBottom:
				bottom = math.Max(bottom, need)
			}
		}
		side = math.Min(maxSide, h-top-bottom)
		if side < ringMinSidePt {
			return false
		}
	}
	x0 := (w - side) / 2
	y0 := top + (h-top-bottom-side)/2
	lay.spec, lay.ring = spec, rhRect{x0, x0 + side, y0, y0 + side}
	labelW := x0 - colGap

	lay.items = make([]rhItem, len(items))
	heights := make([]float64, len(items))
	for i, it := range items {
		lay.items[i] = rhItem{ring: it, side: it.Side, inside: true}
		// A row beside 3 or 9 o'clock is the narrowest: every row holds its
		// text at that width.
		width := labelW
		if it.Side == ringSideTop || it.Side == ringSideBottom {
			width = side
		}
		heights[i] = rhLabelNeedPt(ctx, *lay, v.Spokes[i], rhAlign(it.Side), false, width)
	}
	// Side rows follow the ring into its bounding square, so they keep out of
	// the height a pole label takes above / below it.
	rowSpec := ringRowsSpec{CentreY: y0 + side/2, RadiusPt: spec.Radius * side, RowPt: lay.labelSize * sizingLineSpacing, Heights: heights, GapPt: ctx.Gap(rhRowGapPt), Top: 0, Bottom: h}
	if top > 0 {
		rowSpec.Top = y0 - ctx.Gap(rhPoleGapPt)
	}
	if bottom > 0 {
		rowSpec.Bottom = y0 + side + ctx.Gap(rhPoleGapPt)
	}
	// textAt is a side row's label cell: it starts (right) or ends (left) the
	// column gap from its own satellite's disc and runs to the content edge.
	textAt := func(row ringRow) rhRect {
		r := rhRect{y0: math.Max(row.Y-row.H/2, 0), y1: math.Min(row.Y+row.H/2, h)}
		f := spec.badgeFrame(items[row.Index], lay.satDia)
		edge := ringLabelEdgeX(x0+(f.X+f.W/2)*side, y0+(f.Y+f.H/2)*side, f.W*side/2, r.y0, r.y1, colGap, row.Side)
		if row.Side == ringSideLeft {
			r.x0, r.x1 = 0, edge
		} else {
			r.x0, r.x1 = edge, w
		}
		return r
	}
	rows, need, fits := ringSettleRows(items, rowSpec, func(row ringRow) float64 { return textAt(row).w() },
		func(i int, widthPt float64) float64 {
			return rhLabelNeedPt(ctx, *lay, v.Spokes[i], rhAlign(items[i].Side), false, widthPt)
		})
	if !fits {
		// A side's rows are taller than the room together: every row of a
		// side takes an equal share, and PostExpandWarnings names the items
		// that outgrow theirs.
		count := map[string]int{}
		for _, it := range items {
			count[it.Side]++
		}
		capped := append([]float64(nil), need...)
		room := rowSpec.Bottom - rowSpec.Top
		for i, it := range items {
			if k := float64(count[it.Side]); it.Side == ringSideLeft || it.Side == ringSideRight {
				capped[i] = math.Min(capped[i], math.Floor((room-(k-1)*rowSpec.GapPt)/k))
			}
		}
		rowSpec.Heights = capped
		rows, _ = ringLabelRows(items, rowSpec)
	}
	for i, row := range rows {
		it := &lay.items[i]
		it.need = need[i]
		switch it.side {
		case ringSideLeft, ringSideRight:
			it.text = textAt(row)
		case ringSideTop:
			it.text = rhRect{x0, x0 + side, 0, y0 - ctx.Gap(rhPoleGapPt)}
		case ringSideBottom:
			it.text = rhRect{x0, x0 + side, y0 + side + ctx.Gap(rhPoleGapPt), h}
		}
		it.text.y0, it.text.y1 = math.Max(it.text.y0, 0), math.Min(it.text.y1, h)
	}
	return true
}

// rhPlaceInside centres the ring square in the block; the satellites hold
// their own labels.
func rhPlaceInside(ctx ExpandContext, v *RadialHubValues, lay *rhLayout) error {
	spec, items, err := rhRingItems(len(v.Spokes), lay.satDia)
	if err != nil {
		return fmt.Errorf("radial-hub: %w", err)
	}
	side := math.Min(lay.width, lay.height)
	x0, y0 := (lay.width-side)/2, (lay.height-side)/2
	lay.spec, lay.ring = spec, rhRect{x0, x0 + side, y0, y0 + side}
	lay.items = make([]rhItem, len(items))
	for i, it := range items {
		_, fits := rhFitCircleText(ctx, lay.satDia*side, lay.bodySize, rhCircleLine{text: v.Spokes[i].Label, bold: true})
		lay.items[i] = rhItem{ring: it, inside: fits}
	}
	return nil
}

// rhPlaceLegend puts the ring square on the left and a keyed list on the
// right: one row per item in ring order, the block of rows centred on the
// ring.
func rhPlaceLegend(ctx ExpandContext, v *RadialHubValues, lay *rhLayout) error {
	spec, items, err := rhRingItems(len(v.Spokes), lay.satDia)
	if err != nil {
		return fmt.Errorf("radial-hub: %w", err)
	}
	w, h := lay.width, lay.height
	side := math.Min(h, w*rhLegendFrac)
	y0 := (h - side) / 2
	lay.spec, lay.ring = spec, rhRect{0, side, y0, y0 + side}
	colGap, gap := ctx.Gap(rhLabelGapPt), ctx.Gap(rhRowGapPt)
	lay.items = make([]rhItem, len(items))
	for i, it := range items {
		lay.items[i] = rhItem{ring: it, inside: true}
	}
	// One column of rows; two when one does not hold them and the area is
	// wide enough for two readable columns.
	legendX := side + colGap
	if rhLegendColumn(ctx, v, lay, 0, len(items), legendX, w, gap) > h {
		if colW := (w - legendX - colGap) / 2; colW-rhKeyColPt >= rhMinLabelPt {
			first := (len(items) + 1) / 2
			rhLegendColumn(ctx, v, lay, 0, first, legendX, legendX+colW, gap)
			rhLegendColumn(ctx, v, lay, first, len(items), w-colW, w, gap)
		}
	}
	return nil
}

// rhLegendColumn stacks the legend rows of items [from, to) in the column
// x0..x1 (key letter column included), centred on the block's height, and
// returns the height they need. Rows that do not fit take an equal share.
func rhLegendColumn(ctx ExpandContext, v *RadialHubValues, lay *rhLayout, from, to int, x0, x1, gap float64) float64 {
	textX, n := x0+rhKeyColPt, float64(to-from)
	need := (n - 1) * gap
	for i := from; i < to; i++ {
		lay.items[i].need = rhLabelNeedPt(ctx, *lay, v.Spokes[i], "l", true, x1-textX)
		need += lay.items[i].need
	}
	share, total := math.Inf(1), need
	if need > lay.height {
		share, total = math.Floor((lay.height-(n-1)*gap)/n), lay.height
	}
	y := (lay.height - total) / 2
	for i := from; i < to; i++ {
		rowH := math.Min(lay.items[i].need, share)
		lay.items[i].text = rhRect{textX, x1, y, y + rowH}
		y += rowH + gap
	}
	return need
}

func rhAlign(side string) string {
	switch side {
	case ringSideLeft:
		return "r"
	case ringSideRight:
		return "l"
	}
	return "ctr"
}

// rhLabelNeedPt is the height a label (and its description) is written
// unshrunk at in a frame widthPt wide, by the writer's own measure.
func rhLabelNeedPt(ctx ExpandContext, lay rhLayout, s RadialHubSpoke, align string, top bool, widthPt float64) float64 {
	anchor := "ctr"
	if top {
		anchor = "t"
	}
	return math.Ceil(writtenFitHeightPt(ctx.themeFonts(), rhLabelText(s, align, anchor, lay, "dk1"), widthPt, 0))
}

// rhCircleLine is one paragraph set inside a circle. size 0 = the size being
// fitted.
type rhCircleLine struct {
	text string
	bold bool
	size float64
}

// rhFitCircleText returns the largest size (1pt steps from ceiling down to the
// readable floor) at which the lines fit a circle of diameter dia: no word
// breaks, and the writer stores no autofit shrink for the block in the
// circle's text square. A line with its own size keeps it; the others take
// the fitted size. fits is false when even the floor does not hold.
func rhFitCircleText(ctx ExpandContext, dia, ceiling float64, lines ...rhCircleLine) (float64, bool) {
	// The writer sets the text in the ellipse's inscribed square. The square
	// is measured a little short, so a hub that just fits here still fits the
	// circle the renderer resolves a fraction of a point smaller.
	square := dia * math.Sqrt2 / 2 * rhCircleFitFrac
	inner := square - 2*rhCircleInPt
	floor := shapegrid.MinTextSizePt
	for size := shapegrid.EffectiveTextSizePt(ceiling); size >= floor; size-- {
		var paras []rhPara
		broken := false
		for _, ln := range lines {
			if strings.TrimSpace(ln.text) == "" {
				continue
			}
			s := size
			if ln.size > 0 {
				s = ln.size
			}
			for _, word := range strings.Fields(ln.text) {
				if rhWordWidthPt(ctx, word, ln.bold, s) > inner {
					broken = true
				}
			}
			paras = append(paras, rhPara{Content: pptx.ConvertMarkdownEmphasis(ln.text), Size: s, Bold: ln.bold})
		}
		if broken || len(paras) == 0 {
			continue
		}
		tb, err := shapegrid.ResolveTextInput(rhCircleText(paras...))
		if err != nil || tb == nil {
			continue
		}
		tb.ThemeFonts = ctx.themeFonts()
		box := int64(square * sizingEMUPerPt)
		if pptx.AutofitFitsFor(tb, pptx.RectEmu{CX: box, CY: box}) {
			return size, true
		}
	}
	return floor, false
}

// rhWordWidthPt is the width of one word on a line of its own (a line
// measure wraps at spaces only, so it cannot tell that a single word is wider
// than its frame), with the slack the renderer's own substitute face needs.
func rhWordWidthPt(ctx ExpandContext, word string, bold bool, sizePt float64) float64 {
	w, err := textfit.MeasureStyledLineWidth(word, ctx.Theme.BodyFont, sizePt, bold)
	if err != nil {
		return float64(runeLen(word)) * sizePt * 0.6
	}
	return float64(w) / sizingEMUPerPt * rhWordSlack
}

// PostExpandWarnings reports, by measurement, a label row that outgrows the
// room beside the ring (BODY_TOO_LONG), a label that does not fit its
// satellite (NODE_LABEL_TOO_LONG, whose fix moves the labels outside) and a hub
// label that does not fit the hub circle at the readable floor (BODY_TOO_LONG).
func (p *radialHub) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*RadialHubValues)
	if !ok || v == nil || len(v.Spokes) < rhMinSpokes || len(v.Spokes) > rhMaxSpokes {
		return nil
	}
	lay, err := rhMeasure(ctx, v, rhOverrides(overrides))
	if err != nil {
		return nil
	}
	var warnings []string
	for i, it := range lay.items {
		switch {
		case lay.mode == rhLabelsInside && !it.inside:
			warnings = append(warnings, fmt.Sprintf("%s: radial-hub spokes[%d].label does not fit its %.0fpt satellite at %.0fpt (a word breaks or the label runs past the circle); keep it to one or two words of at most about 8 characters each, or remove overrides.labels so the labels stand \"outside\" the ring",
				ErrCodeNodeLabelTooLong, i, lay.satDia*lay.side(), lay.bodySize))
		case lay.mode != rhLabelsInside && it.need > it.text.h()+0.5:
			field := "description"
			if strings.TrimSpace(v.Spokes[i].Description) == "" {
				field = "label"
			}
			warnings = append(warnings, fmt.Sprintf("%s: radial-hub spokes[%d].%s needs %.0fpt but its label row with %d spokes is %.0fpt tall and %.0fpt wide; keep each description to about %d characters — shorten it, drop the descriptions or use fewer spokes",
				ErrCodeBodyTooLong, i, field, it.need, len(v.Spokes), it.text.h(), it.text.w(), rhDescriptionBudget(lay, it)))
		}
	}
	if strings.TrimSpace(v.Center.Label) != "" && !lay.hubFits {
		warnings = append(warnings, fmt.Sprintf("%s: radial-hub center.label does not fit the %.0fpt hub circle even at %.0fpt (a word breaks or the text runs past the circle); keep the label to about 16 characters of short words and the sublabel to about 20",
			ErrCodeBodyTooLong, lay.hubDia*lay.side(), lay.hubSize))
	}
	return warnings
}

// rhDescriptionBudget is the description length (characters) a label row of
// the item's size holds under a one-line label.
func rhDescriptionBudget(lay rhLayout, it rhItem) int {
	avail := it.text.h() - 2*rhLabelInsetTBPt - lay.labelSize*sizingLineSpacing - rhLabelTitle
	lines := math.Max(math.Floor(avail/(lay.bodySize*sizingLineSpacing)), 1)
	cpl := math.Floor(it.text.w() / (sizingCapacityEm * lay.bodySize))
	return int(math.Max(lines*cpl, 10))
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func (p *radialHub) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*RadialHubValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("radial-hub: values must be *RadialHubValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*RadialHubOverrides); !ok {
			return nil, fmt.Errorf("radial-hub: overrides must be *RadialHubOverrides, got %T", overrides)
		}
	}
	if len(v.Spokes) < rhMinSpokes || len(v.Spokes) > rhMaxSpokes {
		return nil, fmt.Errorf("radial-hub: %d spokes; the hub takes %d-%d", len(v.Spokes), rhMinSpokes, rhMaxSpokes)
	}
	ovr := rhOverrides(overrides)
	lay, err := rhMeasure(ctx, v, ovr)
	if err != nil {
		return nil, err
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	accentInk := accentInkOnLight(ctx, accent, sshInkContrastMin)
	side := lay.side()

	// Layers, bottom to top: spokes, hub, satellites.
	var layers []jsonschema.LayerInput
	if ovr.Spokes != rhSpokesNone {
		for i, it := range lay.items {
			fill, stroke := accentFillJSON(accent), rhSpokePt
			switch {
			case v.Highlight == i+1:
				stroke = rhSpokeHiPt
			case v.Highlight > 0:
				fill = neutralFillJSON(rhSpokeTone)
			}
			frame, rotation := rhSpokeFrame(it.ring.MidDeg, lay.hubDia/2, lay.spec.Radius-lay.satDia/2, stroke/side)
			layers = append(layers, jsonschema.LayerInput{
				Name:  fmt.Sprintf("spoke-%d", i+1),
				Frame: rhLayerFrame(frame),
				Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: fill, Line: noLine, Rotation: rotation},
			})
		}
	}
	hubInk := readableTextOn(ctx, fillTone{Color: accent}, "lt1")
	hubParas := []rhPara{{Content: pptx.ConvertMarkdownEmphasis(v.Center.Label), Size: lay.hubSize, Bold: true, Color: hubInk}}
	if sub := strings.TrimSpace(v.Center.Sublabel); sub != "" {
		hubParas = append(hubParas, rhPara{Content: pptx.ConvertMarkdownEmphasis(sub), Size: lay.bodySize, Color: hubInk})
	}
	hub := ringFrame{X: 0.5 - lay.hubDia/2, Y: 0.5 - lay.hubDia/2, W: lay.hubDia, H: lay.hubDia}
	layers = append(layers, jsonschema.LayerInput{
		Name:  "hub",
		Frame: rhLayerFrame(hub),
		Shape: &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: accentFillJSON(accent), Line: noLine, Text: rhCircleText(hubParas...)},
	})
	for i, it := range lay.items {
		tone := rhSatelliteTone(ctx, accent, i, v.Highlight == i+1, ovr.CellAccentMode)
		shape := &jsonschema.ShapeSpecInput{Geometry: "ellipse", Fill: tone.fillJSON(), Line: noLine}
		ink := readableTextOn(ctx, tone, "dk1")
		icon := rhSpokeIcon(v.Spokes[i], iconFillOn(ctx, shape.Fill, accent), rhIconScale)
		switch {
		case lay.mode == rhLabelsInside:
			shape.Text = rhCircleText(rhPara{Content: pptx.ConvertMarkdownEmphasis(v.Spokes[i].Label), Size: lay.bodySize, Bold: true, Color: ink})
		case icon != nil:
			// The icon fills the disc; in the legend it is the item's key too.
			shape.Icon = icon
		case lay.mode == rhLabelsLegend:
			shape.Text = rhCircleText(rhPara{Content: rhKey(i), Size: lay.bodySize, Bold: true, Color: ink})
		}
		layers = append(layers, jsonschema.LayerInput{
			Name:  fmt.Sprintf("satellite-%d", i+1),
			Frame: rhLayerFrame(lay.spec.badgeFrame(it.ring, lay.satDia)),
			Shape: shape,
		})
	}

	places := []ringPlacement{lay.ringPlacement(layers)}
	for i, it := range lay.items {
		if lay.mode == rhLabelsInside {
			break
		}
		labelInk := "dk1"
		if v.Highlight == i+1 {
			labelInk = accentInk
		}
		align, anchor := rhAlign(it.side), "ctr"
		switch {
		case lay.mode == rhLabelsLegend:
			align, anchor = "l", "t"
			keyCell := rhTextCell(rhKeyText(rhPara{Content: rhKey(i), Size: lay.labelSize, Bold: true, Color: accentInk}))
			if icon := rhSpokeIcon(v.Spokes[i], accentInk, rhKeyIconScale); icon != nil {
				// Same mark as in the disc, on the label's first line.
				icon.Position = "top"
				keyCell = rhTextCell(nil)
				keyCell.Shape.Icon = icon
			}
			places = append(places, ringPlacement{X0: it.text.x0 - rhKeyColPt, X1: it.text.x0, Y0: it.text.y0, Y1: it.text.y1, Cell: keyCell})
		case it.side == ringSideBottom:
			anchor = "t"
		case it.side == ringSideTop:
			anchor = "b"
		}
		places = append(places, ringPlacement{X0: it.text.x0, X1: it.text.x1, Y0: it.text.y0, Y1: it.text.y1,
			Cell: rhTextCell(rhLabelText(v.Spokes[i], align, anchor, lay, labelInk))})
	}
	ringProtectEdges(places, lay.gapEdges())
	grid, err := ringLattice(places, lay.width, lay.height)
	if err != nil {
		return nil, fmt.Errorf("radial-hub: %w", err)
	}
	grid.VerticalAlign = "center"
	return grid, nil
}

// rhSpokeIcon is a spoke's icon as a centred shape overlay in fill, its side
// scale of the shape's shorter side unless the author set one; nil without an
// icon.
func rhSpokeIcon(s RadialHubSpoke, fill string, scale float64) *jsonschema.IconInput {
	if s.Icon == nil {
		return nil
	}
	icon := s.Icon.Resolve(fill, "center")
	if icon != nil && icon.Scale == 0 {
		icon.Scale = scale
	}
	return icon
}

// rhKey is the legend key of item i: A, B, C ...
func rhKey(i int) string {
	return string(rune('A' + i))
}

// rhSatelliteTone is a satellite's fill. Satellites are the accent's content
// swatch (tonalContent) — the hub is the one solid accent block — and the
// highlighted one takes a deeper rung of the same ladder; cell_accent_mode
// alternate / progressive is the author asking for accent-filled satellites.
func rhSatelliteTone(ctx ExpandContext, accent string, i int, highlighted bool, mode string) fillTone {
	switch {
	case mode == CellAccentAlternate || mode == CellAccentProgressive:
		return fillTone{Color: ResolveCellAccent(accent, i, mode)}
	case highlighted:
		return rhHighlightTone(ctx, accent)
	}
	return tonalContent(ctx, accent)
}

// rhHighlightLighter is the highlighted satellite's "Lighter N%" swatch.
const rhHighlightLighter = 40

// rhHighlightTone is the highlighted satellite's fill: a deeper rung of the
// accent ladder, clearly apart from its siblings without being a second
// solid block.
func rhHighlightTone(ctx ExpandContext, accent string) fillTone {
	return tonalRung(ctx, accent, rhHighlightLighter)
}

// rhSpokeFrame is a thin `rect` layer that reads as a line from fromR to toR
// (fractions of the square's side, measured from its centre) along deg: the
// unrotated length × stroke box centred on the spoke's middle, and the
// rotation that turns it onto the angle. The rotated shape stays inside the
// square as long as toR does.
func rhSpokeFrame(deg, fromR, toR, stroke float64) (ringFrame, float64) {
	length := toR - fromR
	cx, cy := pointOnCircle(0.5, 0.5, (fromR+toR)/2, deg)
	return ringFrame{X: cx - length/2, Y: cy - stroke/2, W: length, H: stroke}, normDeg(deg)
}

func rhLayerFrame(f ringFrame) jsonschema.LayerFrameInput {
	return jsonschema.LayerFrameInput{X: rhRound(f.X), Y: rhRound(f.Y), W: rhRound(f.W), H: rhRound(f.H)}
}

// rhRound keeps six decimals of a frame fraction: under a hundredth of a
// point on any slide, and stable in the golden across platforms.
func rhRound(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

type rhPara struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
}

type rhTextObj struct {
	Paragraphs    []rhPara `json:"paragraphs"`
	Align         string   `json:"align"`
	VerticalAlign string   `json:"vertical_align"`
	InsetLeft     *float64 `json:"inset_left,omitempty"`
	InsetRight    *float64 `json:"inset_right,omitempty"`
	InsetTop      *float64 `json:"inset_top,omitempty"`
	InsetBottom   *float64 `json:"inset_bottom,omitempty"`
}

// rhTextJSON is a text object with its own margins: lr on the left and right,
// tb at the top and bottom (points). A negative margin keeps the uniform
// shape margin on those sides.
func rhTextJSON(align, anchor string, lr, tb float64, paras ...rhPara) json.RawMessage {
	for i := range paras {
		paras[i].Align = align
	}
	obj := rhTextObj{Paragraphs: paras, Align: align, VerticalAlign: anchor}
	if lr >= 0 {
		obj.InsetLeft, obj.InsetRight = &lr, &lr
	}
	if tb >= 0 {
		obj.InsetTop, obj.InsetBottom = &tb, &tb
	}
	data, _ := json.Marshal(obj)
	return data
}

// rhCircleText is centred text inside a circle, with the small circle inset.
func rhCircleText(paras ...rhPara) json.RawMessage {
	return rhTextJSON("ctr", "ctr", rhCircleInPt, rhCircleInPt, paras...)
}

// rhKeyText is a legend key letter: top-anchored with the label's own
// margins, so the two share a first line.
func rhKeyText(p rhPara) json.RawMessage {
	return rhTextJSON("ctr", "t", 0, rhLabelInsetTBPt, p)
}

// rhLabelText is a satellite's label and description beside the ring.
func rhLabelText(s RadialHubSpoke, align, anchor string, lay rhLayout, labelInk string) json.RawMessage {
	paras := []rhPara{{Content: pptx.ConvertMarkdownEmphasis(s.Label), Size: lay.labelSize, Bold: true, Color: labelInk}}
	if d := strings.TrimSpace(s.Description); d != "" {
		paras[0].SpaceAfter = rhLabelTitle
		paras = append(paras, rhPara{Content: pptx.ConvertMarkdownEmphasis(d), Size: lay.bodySize, Color: "dk1"})
	}
	return rhTextJSON(align, anchor, 0, rhLabelInsetTBPt, paras...)
}

func rhTextCell(text json.RawMessage) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Line: json.RawMessage(`"none"`), Text: text},
	}
}
