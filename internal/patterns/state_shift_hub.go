package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// ---------------------------------------------------------------------------
// state-shift-hub pattern — "today vs. future state" around a central hub
// ---------------------------------------------------------------------------
//
// A filled accent circle in the middle of the slide holds a short hub label
// ("Today's state vs. agentic state"). 3-6 numbered stage pairs flank it:
// the left column holds the "today" items (right-aligned title + description
// beside an outlined "01".."06" node), the right column the matching "future"
// items (left-aligned beside a filled node). The nodes sit on a circle
// concentric with the hub, so the pairs read as orbiting the shift they
// describe.
//
// The shape grid cannot free-position shapes, so the arc is realised as a
// lattice: every x edge the layout needs (text box edges, node edges, hub
// edges) becomes a column boundary, the column gap is a hairline, and each
// shape spans the lattice columns between its own edges. Gaps between shapes
// are empty lattice cells. Because each shape occupies its own lattice
// rectangle, nothing can overlap whatever size the grid is finally resolved
// at; the trig only decides where the edges fall.

func init() {
	Default().Register(&stateShiftHub{})
}

type stateShiftHub struct{}

func (s *stateShiftHub) Name() string { return "state-shift-hub" }
func (s *stateShiftHub) Description() string {
	return "Central accent hub circle with 3-6 numbered stage pairs orbiting it: 'today' items on the left, matching 'future' items on the right, optional column headers"
}
func (s *stateShiftHub) UseWhen() string {
	return "A current-state vs target-state (today vs future, as-is vs to-be) shift told as 3-6 numbered stage pairs around one central theme, each side with a short title and 1-3 line description; prefer before-after for a single before/after block with bullets, comparison-2col for non-temporal options or pros/cons, and journey-maturity-model for a staged maturity ladder"
}
func (s *stateShiftHub) NotWhen() string {
	return "Only one before/after contrast with bullet lists (use before-after), the two columns are options or pros/cons rather than today vs future (use comparison-2col), the stages form a maturity progression (use journey-maturity-model), or there are fewer than 3 or more than 6 pairs (use before-after or split the slide)"
}
func (s *stateShiftHub) Version() int      { return 1 }
func (s *stateShiftHub) CellsHint() string { return "1 hub + 3-6 pairs (2 nodes + 2 text blocks each)" }
func (s *stateShiftHub) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame", "compare"},
		PairsWith:     []string{"phase-roadmap", "kpi-3up", "exec-summary"},
		DensityClass:  "medium",
		AccentWeight:  "strong",
	}
}
func (s *stateShiftHub) SupportsInlineMarkdown() bool { return true }

func (s *stateShiftHub) ExemplarValues() any {
	return &StateShiftHubValues{
		HubLabel:    "Today's state vs. agentic state",
		LeftHeader:  "TODAY'S STATE",
		RightHeader: "AGENTIC STATE",
		Pairs: []StateShiftPair{
			{Title: "Intake", Before: "Requests arrive by email and are triaged by hand", After: "Agents classify and route every request on arrival"},
			{Title: "Research", Before: "Analysts spend days assembling context", After: "Agents draft a sourced brief in minutes"},
			{Title: "Decision", Before: "Committees wait for weekly meetings", After: "Owners approve pre-analysed options same day"},
			{Title: "Follow-up", Before: "Status is chased manually", After: "Agents track progress and escalate exceptions"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// StateShiftPair is one numbered stage: the "today" item and its "future"
// counterpart. Title is the shared stage title; BeforeTitle / AfterTitle
// replace it on one side.
type StateShiftPair struct {
	Title       string `json:"title,omitempty"`
	Before      string `json:"before"`
	After       string `json:"after"`
	BeforeTitle string `json:"before_title,omitempty"`
	AfterTitle  string `json:"after_title,omitempty"`
}

// StateShiftHubValues holds the hub label, the optional column headers and
// the 3-6 stage pairs.
type StateShiftHubValues struct {
	HubLabel    string           `json:"hub_label"`
	LeftHeader  string           `json:"left_header,omitempty"`
	RightHeader string           `json:"right_header,omitempty"`
	Pairs       []StateShiftPair `json:"pairs"`
}

// StateShiftHubOverrides are the pattern-level overrides.
type StateShiftHubOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	TitleSize      float64 `json:"title_size,omitempty"` // stage titles (default 14)
	BodySize       float64 `json:"body_size,omitempty"`  // descriptions (default 12)
	HubSize        float64 `json:"hub_size,omitempty"`   // hub label ceiling (default 20; shrinks to fit)
}

func (s *stateShiftHub) NewValues() any       { return &StateShiftHubValues{} }
func (s *stateShiftHub) NewOverrides() any    { return &StateShiftHubOverrides{} }
func (s *stateShiftHub) NewCellOverride() any { return nil }

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	sshMinPairs          = 3
	sshMaxPairs          = 6
	sshHubMax            = 60
	sshHeaderMax         = 48
	sshTitleMax          = 40
	sshBodyMax           = 140
	sshTitlePt           = 14.0
	sshBodyPt            = 12.0
	sshHubPt             = 20.0
	sshHeaderPt          = 12.0
	sshNodeLabelPt       = 12.0
	sshNodeDiaPt         = 34.0 // numbered node circle
	sshHubGapPt          = 16.0 // hub edge to the nearest node edge
	sshTextGapPt         = 8.0  // node edge to its text box
	sshRowGapPt          = 6.0
	sshColGapPt          = 0.01 // lattice columns are geometric: a hairline gap
	sshMaxPitchPt        = 96.0 // a stage row never grows taller than this
	sshHubWidthFrac      = 0.25 // hub diameter as a share of the width, before height caps it
	sshHubWidthDenseFrac = 0.21 // the same with 5-6 pairs
	sshMaxBulgePt        = 44.0 // furthest the middle nodes swing out past the top/bottom ones
	sshHubMinPt          = 110.0
	sshHubMaxPt          = 230.0
	sshHaloWidthPt       = 6.0
	sshTitleSpacePt      = 2.0 // space after a stage title
	sshNeedSlackPt       = 2.0 // rounding slack on a measured text block
	// sshInkContrastMin is the bar accent-coloured text on the slide must clear:
	// the generator's contrast pass swaps anything under 4.5:1, so asking less
	// only moves the choice from the pattern to the fixer.
	sshInkContrastMin = 4.5
	sshEdgeMergePt    = 1.0 // lattice edges closer than this collapse into one
)

// ---------------------------------------------------------------------------
// Schema / Validate
// ---------------------------------------------------------------------------

func (s *stateShiftHub) Schema() *Schema {
	pair := ObjectSchema(map[string]*Schema{
		"title":        StringSchema(sshTitleMax).WithDescription("Stage title shown (bold, accent) above both descriptions, e.g. \"Research\""),
		"before":       StringSchema(sshBodyMax).WithDescription("Today / current-state description (left column, right-aligned). Measured readable budget at default sizes with a one-line title: about 140 characters with 3 pairs, 120 with 4, 80 with 5, 40 with 6; expand_pattern reports BODY_TOO_LONG when the text outgrows its row"),
		"after":        StringSchema(sshBodyMax).WithDescription("Future / target-state description (right column, left-aligned); same budget as before"),
		"before_title": StringSchema(sshTitleMax).WithDescription("Optional left-side title replacing title"),
		"after_title":  StringSchema(sshTitleMax).WithDescription("Optional right-side title replacing title"),
	}, []string{"before", "after"}).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(map[string]*Schema{
		"hub_label":    StringSchema(sshHubMax).WithDescription("Short label inside the central circle, e.g. \"Today's state vs. agentic state\" (about 40 characters reads best)"),
		"left_header":  StringSchema(sshHeaderMax).WithDescription("Optional header above the left (today) column, e.g. \"TODAY'S STATE\""),
		"right_header": StringSchema(sshHeaderMax).WithDescription("Optional header above the right (future) column, e.g. \"AGENTIC STATE (doing more in less time)\""),
		"pairs":        ArraySchema(pair, sshMinPairs, sshMaxPairs).WithDescription("3-6 numbered stage pairs; nodes 01..06 orbit the hub on both sides"),
	}, []string{"hub_label", "pairs"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":          StringSchema(0).WithDescription("Accent scheme color for the hub, nodes and titles (default accent1)").WithDefault("accent1"),
		"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"title_size":      NumberSchema(12, 28).WithDescription("Stage title font size in points (default 14)"),
		"body_size":       NumberSchema(12, 20).WithDescription("Description font size in points (default 12)"),
		"hub_size":        NumberSchema(12, 36).WithDescription("Hub label font size ceiling in points (default 20); the label shrinks until it fits the circle"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Central hub circle with 3-6 numbered today/future stage pairs arranged on an arc around it")
}

func (s *stateShiftHub) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*StateShiftHubValues)
	if !ok || v == nil {
		return fmt.Errorf("state-shift-hub: values must be *StateShiftHubValues, got %T", values)
	}
	const name = "state-shift-hub"
	var errs []error

	if overrides != nil {
		if _, ok := overrides.(*StateShiftHubOverrides); !ok {
			errs = append(errs, fmt.Errorf("state-shift-hub: overrides must be *StateShiftHubOverrides, got %T", overrides))
		}
	}

	if strings.TrimSpace(v.HubLabel) == "" {
		errs = append(errs, errRequired(name, "hub_label"))
	} else if runeLen(v.HubLabel) > sshHubMax {
		errs = append(errs, errMaxLength(name, "hub_label", sshHubMax, runeLen(v.HubLabel)))
	}
	for _, f := range []struct{ path, val string }{{"left_header", v.LeftHeader}, {"right_header", v.RightHeader}} {
		if runeLen(f.val) > sshHeaderMax {
			errs = append(errs, errMaxLength(name, f.path, sshHeaderMax, runeLen(f.val)))
		}
	}

	if len(v.Pairs) < sshMinPairs {
		errs = append(errs, errMinItems(name, "pairs", sshMinPairs, len(v.Pairs), "(hint: use before-after for a single today/future contrast)"))
	}
	if len(v.Pairs) > sshMaxPairs {
		errs = append(errs, errMaxItems(name, "pairs", sshMaxPairs, len(v.Pairs), "(hint: merge related stages or split the shift across two slides)"))
	}
	for i, p := range v.Pairs {
		for _, f := range []struct {
			field, val string
			max        int
			required   bool
		}{
			{"before", p.Before, sshBodyMax, true},
			{"after", p.After, sshBodyMax, true},
			{"title", p.Title, sshTitleMax, false},
			{"before_title", p.BeforeTitle, sshTitleMax, false},
			{"after_title", p.AfterTitle, sshTitleMax, false},
		} {
			path := fmt.Sprintf("pairs[%d].%s", i, f.field)
			switch {
			case f.required && strings.TrimSpace(f.val) == "":
				errs = append(errs, errRequired(name, path))
			case runeLen(f.val) > f.max:
				errs = append(errs, errMaxLength(name, path, f.max, runeLen(f.val)))
			}
		}
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"state-shift-hub: cell_overrides are not supported (use overrides)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// sshRow is the resolved geometry of one stage row (points, grid-relative).
type sshRow struct {
	dx       float64 // node centre's horizontal distance from the hub centre
	textEdge float64 // right edge of the left text box (mirrored on the right)
}

// sshLayout carries every measurement Expand and PostExpandWarnings share.
type sshLayout struct {
	width, cx  float64
	headerH    float64 // 0 = no header row
	pitch      float64 // stage row height
	hubD       float64
	hubSize    float64
	hubFits    bool
	titleSize  float64
	bodySize   float64
	rows       []sshRow
	headerEdge float64 // right edge of the left header (mirrored on the right)
	hubTextW   float64 // usable label width inside the hub circle
	hubTextH   float64
	nodeR      float64
}

func sshOverrides(overrides any) *StateShiftHubOverrides {
	if o, ok := overrides.(*StateShiftHubOverrides); ok && o != nil {
		return o
	}
	return &StateShiftHubOverrides{}
}

func (v *StateShiftHubValues) hasHeaders() bool {
	return strings.TrimSpace(v.LeftHeader) != "" || strings.TrimSpace(v.RightHeader) != ""
}

// sshMeasure resolves the geometry: stage-row pitch from the height left under
// the headers, a hub circle sized to the width (capped by the body height),
// and node positions on a circle concentric with the hub. The ring radius is
// the smallest that keeps the top and bottom nodes clear of the hub: those rows
// sit furthest from the centre vertically, so they sit closest horizontally
// and the middle rows bulge outward — the arc.
func sshMeasure(ctx ExpandContext, v *StateShiftHubValues, ovr *StateShiftHubOverrides) sshLayout {
	w, h := sizingAreaPt(ctx)
	n := len(v.Pairs)
	if n < 1 {
		n = 1
	}
	lay := sshLayout{
		width:     w,
		cx:        w / 2,
		titleSize: shapegrid.EffectiveTextSizePt(ResolveSize(ovr.TitleSize, sshTitlePt)),
		bodySize:  shapegrid.EffectiveTextSizePt(ResolveSize(ovr.BodySize, sshBodyPt)),
		nodeR:     sshNodeDiaPt / 2,
	}

	n64 := float64(n)
	hubFrac := sshHubWidthFrac
	if n >= 5 {
		hubFrac = sshHubWidthDenseFrac // dense rows need the width for text
	}
	hubD := math.Min(w*hubFrac, sshHubMaxPt)
	dxMin := hubD/2 + sshHubGapPt + lay.nodeR
	// The header spans its text column and the nodes beside it.
	lay.headerEdge = lay.cx - dxMin + lay.nodeR

	bodyAvail := h
	if v.hasHeaders() {
		lay.headerH = headerRowPt(ctx.Theme.BodyFont, []string{v.LeftHeader, v.RightHeader}, sshHeaderPt, lay.headerEdge-2*defaultShapeInsetLRPt)
		bodyAvail -= lay.headerH + sshRowGapPt
	}
	lay.pitch = math.Min((bodyAvail-(n64-1)*sshRowGapPt)/n64, sshMaxPitchPt)
	lay.pitch = math.Max(lay.pitch, sshNodeDiaPt+4)
	bodyH := n64*lay.pitch + (n64-1)*sshRowGapPt

	lay.hubD = math.Min(hubD, bodyH-2*sshHaloWidthPt)
	lay.hubD = math.Max(lay.hubD, math.Min(sshHubMinPt, bodyH))

	// Nodes sit on a circle concentric with the hub, flattened into an
	// ellipse when the full circle would bulge the middle rows so far out
	// that their text column starves.
	dyMax := math.Abs(sshRowCentre(0, lay.pitch) - bodyH/2)
	ring := math.Hypot(dxMin, dyMax)
	squash := 1.0
	if bulge := ring - dxMin; bulge > sshMaxBulgePt {
		squash = sshMaxBulgePt / bulge
	}
	lay.rows = make([]sshRow, len(v.Pairs))
	for i := range lay.rows {
		dy := sshRowCentre(i, lay.pitch) - bodyH/2
		dx := dxMin + (math.Sqrt(math.Max(ring*ring-dy*dy, dxMin*dxMin))-dxMin)*squash
		lay.rows[i] = sshRow{dx: dx, textEdge: lay.cx - dx - lay.nodeR - sshTextGapPt}
	}

	// The ellipse's text rectangle is its inscribed square.
	inner := lay.hubD * math.Sqrt2 / 2
	lay.hubTextW = inner - 2*defaultShapeInsetLRPt
	lay.hubTextH = inner - 2*defaultShapeInsetTBPt
	lay.hubSize, lay.hubFits = sshFitHubLabel(ctx, v.HubLabel, ResolveSize(ovr.HubSize, sshHubPt), lay.hubTextW, lay.hubTextH)
	return lay
}

func sshRowCentre(i int, pitch float64) float64 {
	return float64(i)*(pitch+sshRowGapPt) + pitch/2
}

// sshFitHubLabel returns the largest size (1pt steps, floored at the renderer's
// readable floor) at which every word of the hub label stays on one line and
// the whole label fits the circle's text square; fits is false when even the
// floor does not.
func sshFitHubLabel(ctx ExpandContext, label string, size, textW, textH float64) (float64, bool) {
	floor := shapegrid.MinTextSizePt
	size = shapegrid.EffectiveTextSizePt(size)
	words := strings.Fields(label)
	for s := size; s >= floor; s-- {
		broken := false
		for _, word := range words {
			if measuredLines(word, ctx.Theme.BodyFont, true, s, textW) > 1 {
				broken = true
				break
			}
		}
		if broken {
			continue
		}
		lines := measuredLines(label, ctx.Theme.BodyFont, true, s, textW)
		if float64(lines)*s*contentLineHeight <= textH {
			return s, true
		}
	}
	return floor, false
}

// sshTitles returns the left and right titles of pair p.
func sshTitles(p StateShiftPair) (before, after string) {
	before, after = strings.TrimSpace(p.Title), strings.TrimSpace(p.Title)
	if t := strings.TrimSpace(p.BeforeTitle); t != "" {
		before = t
	}
	if t := strings.TrimSpace(p.AfterTitle); t != "" {
		after = t
	}
	return before, after
}

// sshTextNeedPt is the measured height a side's title + description needs in a
// text frame frameW wide. The frames are unfilled and vertically centred, so
// they are measured with the renderer's own 3.6pt top/bottom insets rather
// than the conservative card insets of sizedBlockHeightPt; line counts still
// come from paragraphLines, which also honours the capacity model.
func sshTextNeedPt(ctx ExpandContext, lay sshLayout, title, body string, frameW float64) float64 {
	need := 2*defaultShapeInsetTBPt + sshNeedSlackPt
	if title != "" {
		need += sshTitleBlockPt(ctx, lay, title, frameW)
	}
	return need + float64(paragraphLines(ctx, sizedPara{text: body, sizePt: lay.bodySize}, frameW))*lay.bodySize*sizingLineSpacing
}

// sshTitleBlockPt is the height of a stage title plus its space-after.
func sshTitleBlockPt(ctx ExpandContext, lay sshLayout, title string, frameW float64) float64 {
	lines := paragraphLines(ctx, sizedPara{text: title, sizePt: lay.titleSize, bold: true}, frameW)
	return float64(lines)*lay.titleSize*sizingLineSpacing + sshTitleSpacePt
}

// PostExpandWarnings reports, by measurement, a description that outgrows its
// stage row and a hub label that does not fit the circle at the readable floor.
func (s *stateShiftHub) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*StateShiftHubValues)
	if !ok || v == nil || len(v.Pairs) == 0 {
		return nil
	}
	lay := sshMeasure(ctx, v, sshOverrides(overrides))
	var warnings []string
	for i, p := range v.Pairs {
		bt, at := sshTitles(p)
		frameW := lay.rows[i].textEdge
		for _, side := range []struct{ field, title, body string }{{"before", bt, p.Before}, {"after", at, p.After}} {
			need := sshTextNeedPt(ctx, lay, side.title, pptx.ConvertMarkdownEmphasis(side.body), frameW)
			if need <= lay.pitch {
				continue
			}
			lines := sshBodyLineBudget(ctx, lay, side.title, frameW)
			warnings = append(warnings, fmt.Sprintf("%s: state-shift-hub pairs[%d].%s needs %.0fpt but a stage row with %d pairs is %.0fpt tall; keep each description to about %d lines (%d characters) — shorten it or use fewer pairs",
				ErrCodeBodyTooLong, i, side.field, need, len(v.Pairs), lay.pitch, lines, sshCharBudget(lay, lines, frameW)))
		}
	}
	if strings.TrimSpace(v.HubLabel) != "" && !lay.hubFits {
		warnings = append(warnings, fmt.Sprintf("%s: state-shift-hub hub_label does not fit the %.0fpt hub circle even at %.0fpt (a word breaks or the label runs past the circle); keep it to about 40 characters of short words",
			ErrCodeBodyTooLong, lay.hubD, lay.hubSize))
	}
	return warnings
}

// sshBodyLineBudget is how many description lines fit a stage row under title.
func sshBodyLineBudget(ctx ExpandContext, lay sshLayout, title string, frameW float64) int {
	avail := lay.pitch - 2*defaultShapeInsetTBPt - sshNeedSlackPt
	if title != "" {
		avail -= sshTitleBlockPt(ctx, lay, title, frameW)
	}
	return max(int(avail/(lay.bodySize*sizingLineSpacing)), 1)
}

// sshCharBudget converts a line budget into an approximate character count.
func sshCharBudget(lay sshLayout, lines int, frameW float64) int {
	cpl := math.Floor((frameW - 2*sizingInsetLRPt) / (sizingCapacityEm * lay.bodySize))
	return int(float64(lines) * cpl)
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

// sshPlacement is one shape placed on the lattice.
type sshPlacement struct {
	row, rowSpan int
	x0, x1       float64 // grid-relative points
	cell         *jsonschema.GridCellInput
}

func (s *stateShiftHub) Expand(ctx ExpandContext, values, overrides any, _ map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*StateShiftHubValues)
	if !ok || v == nil {
		return nil, fmt.Errorf("state-shift-hub: values must be *StateShiftHubValues, got %T", values)
	}
	if overrides != nil {
		if _, ok := overrides.(*StateShiftHubOverrides); !ok {
			return nil, fmt.Errorf("state-shift-hub: overrides must be *StateShiftHubOverrides, got %T", overrides)
		}
	}
	if len(v.Pairs) == 0 {
		return nil, fmt.Errorf("state-shift-hub: at least one pair is required")
	}
	ovr := sshOverrides(overrides)
	lay := sshMeasure(ctx, v, ovr)

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	accentInk := sshAccentInk(ctx, accent)
	onAccent := readableTextOn(ctx, fillTone{Color: accent}, "lt1")
	w, cx := lay.width, lay.cx

	var places []sshPlacement
	var rows []jsonschema.GridRowInput
	body := 0
	if v.hasHeaders() {
		rows = append(rows, jsonschema.GridRowInput{MinHeight: lay.headerH, MaxHeight: lay.headerH})
		body = 1
		if t := strings.TrimSpace(v.LeftHeader); t != "" {
			places = append(places, sshPlacement{row: 0, rowSpan: 1, x0: 0, x1: lay.headerEdge, cell: sshHeaderCell(t, "r", "dk2")})
		}
		if t := strings.TrimSpace(v.RightHeader); t != "" {
			places = append(places, sshPlacement{row: 0, rowSpan: 1, x0: w - lay.headerEdge, x1: w, cell: sshHeaderCell(t, "l", accentInk)})
		}
	}

	hubCell := &jsonschema.GridCellInput{
		RowSpan: len(v.Pairs),
		Fit:     "contain",
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "ellipse",
			Fill:     accentFillJSON(accent),
			Line:     sshHaloJSON(accent),
			Text:     sshTextJSON("ctr", sshInsets{}, sshPara{Content: pptx.ConvertMarkdownEmphasis(v.HubLabel), Size: lay.hubSize, Bold: true, Color: onAccent}),
		},
	}
	places = append(places, sshPlacement{row: body, rowSpan: len(v.Pairs), x0: cx - lay.hubD/2, x1: cx + lay.hubD/2, cell: hubCell})

	for i, p := range v.Pairs {
		r := body + i
		row := lay.rows[i]
		bt, at := sshTitles(p)
		label := fmt.Sprintf("%02d", i+1)
		nodeX0 := cx - row.dx - lay.nodeR
		nodeX1 := cx - row.dx + lay.nodeR
		places = append(places,
			sshPlacement{row: r, rowSpan: 1, x0: 0, x1: row.textEdge, cell: sshItemCell(bt, p.Before, "r", lay, accentInk)},
			sshPlacement{row: r, rowSpan: 1, x0: nodeX0, x1: nodeX1, cell: sshNodeCell(label, json.RawMessage(`"lt1"`), sshNodeLineJSON(accent), accentInk)},
			sshPlacement{row: r, rowSpan: 1, x0: w - nodeX1, x1: w - nodeX0, cell: sshNodeCell(label, accentFillJSON(accent), json.RawMessage(`"none"`), onAccent)},
			sshPlacement{row: r, rowSpan: 1, x0: w - row.textEdge, x1: w, cell: sshItemCell(at, p.After, "l", lay, accentInk)},
		)
		rows = append(rows, jsonschema.GridRowInput{MaxHeight: math.Floor(lay.pitch)})
	}

	cols, err := sshLattice(places, rows, w)
	if err != nil {
		return nil, err
	}
	colsJSON, _ := json.Marshal(cols)
	return &jsonschema.ShapeGridInput{
		Columns: colsJSON,
		ColGap:  sshColGapPt,
		RowGap:  sshRowGapPt,
		Rows:    rows,
	}, nil
}

// sshLattice turns the placements into lattice columns (percentages) and fills
// rows[].Cells, emitting one empty cell per free lattice column before each
// shape so the shape-grid column cursor lands on the shape's first column.
func sshLattice(places []sshPlacement, rows []jsonschema.GridRowInput, width float64) ([]float64, error) {
	edges := []float64{0, width}
	for _, p := range places {
		edges = append(edges, p.x0, p.x1)
	}
	sort.Float64s(edges)
	merged := []float64{edges[0]}
	for _, e := range edges[1:] {
		if e-merged[len(merged)-1] >= sshEdgeMergePt {
			merged = append(merged, e)
		}
	}
	// The last edge must be the full width.
	merged[len(merged)-1] = width
	col := func(x float64) int {
		best, bestD := 0, math.Inf(1)
		for i, e := range merged {
			if d := math.Abs(e - x); d < bestD {
				best, bestD = i, d
			}
		}
		return best
	}
	nCols := len(merged) - 1
	cols := make([]float64, nCols)
	for i := 0; i < nCols; i++ {
		cols[i] = math.Round((merged[i+1]-merged[i])/width*100*1000) / 1000
	}

	occupied := make([][]bool, len(rows))
	for r := range occupied {
		occupied[r] = make([]bool, nCols)
	}
	sort.SliceStable(places, func(i, j int) bool {
		if places[i].row != places[j].row {
			return places[i].row < places[j].row
		}
		return places[i].x0 < places[j].x0
	})
	cursor := make([]int, len(rows))
	for _, p := range places {
		c0, c1 := col(p.x0), col(p.x1)
		if c1 <= c0 {
			return nil, fmt.Errorf("state-shift-hub: shape at %.1f-%.1fpt collapsed on the lattice", p.x0, p.x1)
		}
		for c := cursor[p.row]; c < c0; c++ {
			if !occupied[p.row][c] {
				rows[p.row].Cells = append(rows[p.row].Cells, &jsonschema.GridCellInput{})
			}
		}
		for dr := 0; dr < p.rowSpan && p.row+dr < len(rows); dr++ {
			for c := c0; c < c1; c++ {
				if occupied[p.row+dr][c] {
					return nil, fmt.Errorf("state-shift-hub: lattice overlap at row %d column %d", p.row+dr, c)
				}
				occupied[p.row+dr][c] = true
			}
		}
		if span := c1 - c0; span > 1 {
			p.cell.ColSpan = span
		}
		rows[p.row].Cells = append(rows[p.row].Cells, p.cell)
		cursor[p.row] = c1
	}
	return cols, nil
}

// ---------------------------------------------------------------------------
// Cell builders
// ---------------------------------------------------------------------------

type sshPara struct {
	Content    string  `json:"content"`
	Size       float64 `json:"size"`
	Bold       bool    `json:"bold,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"`
}

type sshInsets struct {
	L, T, R, B float64
	set        bool
}

type sshTextObj struct {
	Paragraphs    []sshPara `json:"paragraphs"`
	Align         string    `json:"align"`
	VerticalAlign string    `json:"vertical_align"`
	InsetLeft     float64   `json:"inset_left,omitempty"`
	InsetTop      float64   `json:"inset_top,omitempty"`
	InsetRight    float64   `json:"inset_right,omitempty"`
	InsetBottom   float64   `json:"inset_bottom,omitempty"`
}

func sshTextJSON(align string, in sshInsets, paras ...sshPara) json.RawMessage {
	for i := range paras {
		paras[i].Align = align
	}
	obj := sshTextObj{Paragraphs: paras, Align: align, VerticalAlign: "ctr"}
	if in.set {
		obj.InsetLeft, obj.InsetTop, obj.InsetRight, obj.InsetBottom = in.L, in.T, in.R, in.B
	}
	data, _ := json.Marshal(obj)
	return data
}

func sshHeaderCell(text, align, color string) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     json.RawMessage(`"none"`),
			Text:     sshTextJSON(align, sshInsets{}, sshPara{Content: text, Size: sshHeaderPt, Bold: true, Color: color}),
		},
		AccentBar: &jsonschema.AccentBarInput{Position: "bottom", Color: color, Width: 1.5},
	}
}

func sshItemCell(title, body, align string, lay sshLayout, accentInk string) *jsonschema.GridCellInput {
	var paras []sshPara
	if title != "" {
		paras = append(paras, sshPara{Content: pptx.ConvertMarkdownEmphasis(title), Size: lay.titleSize, Bold: true, Color: accentInk, SpaceAfter: sshTitleSpacePt})
	}
	paras = append(paras, sshPara{Content: pptx.ConvertMarkdownEmphasis(body), Size: lay.bodySize, Color: "dk1"})
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     json.RawMessage(`"none"`),
			Text:     sshTextJSON(align, sshInsets{}, paras...),
		},
	}
}

// sshNodeCell is a numbered circle. Its text insets are near zero: the
// ellipse's own text rectangle (the inscribed square) is already narrow, and
// the default 7.2pt side insets would leave "01" a few points to wrap in.
func sshNodeCell(label string, fill, line json.RawMessage, color string) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{
		Fit: "contain",
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "ellipse",
			Fill:     fill,
			Line:     line,
			Text:     sshTextJSON("ctr", sshInsets{T: 0.5, set: true}, sshPara{Content: label, Size: sshNodeLabelPt, Bold: true, Color: color}),
		},
	}
}

func sshNodeLineJSON(accent string) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"color": accent, "width": 1.75})
	return data
}

// sshHaloJSON is a pale ring around the hub (a light tint of the accent), so
// the hub reads as the centre of the orbit rather than one more filled dot.
func sshHaloJSON(accent string) json.RawMessage {
	if isHexColor(accent) {
		data, _ := json.Marshal(map[string]any{"color": "lt2", "width": sshHaloWidthPt})
		return data
	}
	data, _ := json.Marshal(map[string]any{"color": accent, "width": sshHaloWidthPt, "lumMod": 40000, "lumOff": 60000})
	return data
}

// sshAccentInk is the colour of accent text on the slide: the accent when it
// clears sshInkContrastMin against lt1, else the template's dk2 (the swap the
// generator's contrast fixer would make), else dk1. Without a theme the
// accent is trusted.
func sshAccentInk(ctx ExpandContext, accent string) string {
	for _, candidate := range []string{accent, "dk2"} {
		ratio, ok := schemeContrast(ctx, candidate, "lt1")
		if !ok || ratio >= sshInkContrastMin {
			return candidate
		}
	}
	return "dk1"
}
