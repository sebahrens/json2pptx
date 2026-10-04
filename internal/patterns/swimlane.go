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
// swimlane pattern — horizontal bands per actor with steps placed per lane
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&swimlane{})
}

type swimlane struct{}

func (s *swimlane) Name() string        { return "swimlane" }
func (s *swimlane) Description() string { return "Horizontal swimlane diagram with actors and steps" }
func (s *swimlane) UseWhen() string {
	return "Cross-functional process where steps are owned by different actors/roles; prefer process-flow when all steps belong to one actor, roadmap-phased when lanes are workstreams over time"
}
func (s *swimlane) NotWhen() string {
	return "All steps belong to a single actor (use process-flow), lanes represent time-phased workstreams (use roadmap-phased), or responsibilities are a simple list (use card-grid)"
}
func (s *swimlane) Version() int      { return 1 }
func (s *swimlane) CellsHint() string { return "lanes × steps" }
func (s *swimlane) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "structural",
		NarrativeRole: []string{"frame"},
		PairsWith:     []string{"kpi-3up", "arch-stack", "process-flow"},
		DensityClass:  "high",
		AccentWeight:  "normal",
	}
}
func (s *swimlane) SupportsCallout() bool        { return true }
func (s *swimlane) SupportsInlineMarkdown() bool { return true }

func (s *swimlane) ExemplarValues() any {
	return &SwimlaneValues{
		Lanes: []SwimlaneLane{
			{Actor: "Customer", Steps: []string{"Report incident through the portal", "", "", "", "", "Confirm service restored and close"}},
			{Actor: "Service desk", Steps: []string{"", "Log, classify and assign priority", "", "", "Verify fix with the customer", ""}},
			{Actor: "Engineering", Steps: []string{"", "", "Diagnose the root cause", "Deploy and monitor the fix", "", ""}},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SwimlaneLane represents one horizontal band with an actor label and steps.
type SwimlaneLane struct {
	Actor string   `json:"actor"`
	Steps []string `json:"steps"` // Empty string = no shape in that column
}

// SwimlaneValues holds the lanes for the swimlane pattern.
type SwimlaneValues struct {
	Lanes []SwimlaneLane `json:"lanes"`
	// Flow is the order the arrows follow: [lane, step] pairs (both 0-based),
	// one per step in the order the process visits them. Omitted, the arrows
	// follow the columns left to right and, inside a column, the lanes top to
	// bottom — which is only the process when every column holds one step.
	Flow [][2]int `json:"flow,omitempty"`
}

// SwimlaneOverrides is the standard text overrides.
type SwimlaneOverrides = TextOverrides

// SwimlaneCellOverride is the shared per-cell override.
type SwimlaneCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (s *swimlane) NewValues() any       { return &SwimlaneValues{} }
func (s *swimlane) NewOverrides() any    { return &SwimlaneOverrides{} }
func (s *swimlane) NewCellOverride() any { return &SwimlaneCellOverride{} }

// Measured with TestSwimlaneBudgetProbe against the written size (no run
// stored below its role floor) on every shipped template
// (go-slide-creator-n1muf), every shape keeping the uniform 0.5 cm text
// margin. Rows are step columns 2..8; columns are lane counts 2..6.
var swimlaneStepBudgets = [7][5]int{
	{80, 80, 80, 80, 52},
	{80, 80, 80, 62, 32},
	{80, 80, 75, 50, 25},
	{80, 77, 47, 32, 17},
	{80, 75, 45, 30, 15},
	{80, 51, 31, 21, 11},
	{78, 50, 30, 20, 10},
}

// swimlaneActorBudget is the readable actor label length with five lanes
// (swimlaneActorBudgetSix with six).
const (
	swimlaneActorBudget    = 30
	swimlaneActorBudgetSix = 15
)

func swimlaneStepBudget(steps, lanes int) int {
	if steps < 2 || steps > 8 || lanes < 2 || lanes > 6 {
		return 80
	}
	return swimlaneStepBudgets[steps-2][lanes-2]
}

// swimlaneSharedColumn returns the first column (0-based) that holds a step
// in more than one lane, and the number of lanes it holds one in.
func swimlaneSharedColumn(lanes []SwimlaneLane) (col, count int, shared bool) {
	steps := 0
	if len(lanes) > 0 {
		steps = len(lanes[0].Steps)
	}
	for j := 0; j < steps; j++ {
		n := 0
		for _, lane := range lanes {
			if j < len(lane.Steps) && lane.Steps[j] != "" {
				n++
			}
		}
		if n > 1 {
			return j, n, true
		}
	}
	return 0, 0, false
}

func (s *swimlane) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*SwimlaneValues)
	if !ok || v == nil {
		return nil
	}
	steps := 0
	if len(v.Lanes) > 0 {
		steps = len(v.Lanes[0].Steps)
	}
	budget := swimlaneStepBudget(steps, len(v.Lanes))
	var warnings []string
	// The arrows can only be derived when every column holds one step. With
	// two steps in a column and no stated order they run down the column and
	// on to the next one, which is rarely the process (go-slide-creator-v786r).
	if len(v.Flow) == 0 {
		if col, n, shared := swimlaneSharedColumn(v.Lanes); shared {
			warnings = append(warnings, fmt.Sprintf("%s: swimlane step column %d holds a step in %d lanes and values.flow is not set, so the arrows run down each column, top lane first, then on to the next column — which is the process only by accident; give every step its own column (empty strings elsewhere), or state the order in values.flow as [lane, step] pairs, e.g. [[0,0],[1,0],[1,1]]", ErrCodeSwimlaneFlowAmbiguous, col, n))
		}
	}
	for i, lane := range v.Lanes {
		actorBudget := 0
		switch {
		case len(v.Lanes) >= 6:
			actorBudget = swimlaneActorBudgetSix
		case len(v.Lanes) == 5:
			actorBudget = swimlaneActorBudget
		}
		if n := runeLen(lane.Actor); actorBudget > 0 && n > actorBudget {
			warnings = append(warnings, fmt.Sprintf("%s: swimlane lanes[%d].actor is %d characters; %d lanes hold about %d actor characters — shorten the label or use fewer lanes", ErrCodeBodyTooLong, i, n, len(v.Lanes), actorBudget))
		}
		for j, step := range lane.Steps {
			if n := runeLen(step); n > budget {
				warnings = append(warnings, fmt.Sprintf("%s: swimlane lanes[%d].steps[%d] is %d characters; a %d-step x %d-lane grid holds about %d per step before text shrinks below the readable minimum — shorten the step or use fewer columns/lanes", ErrCodeBodyTooLong, i, j, n, steps, len(v.Lanes), budget))
			}
		}
	}
	return warnings
}

func (s *swimlane) Schema() *Schema {
	laneSchema := ObjectSchema(
		map[string]*Schema{
			"actor": StringSchema(40).WithDescription("Lane actor/function label; about 30 characters with five lanes, 15 with six"),
			"steps": ArraySchema(StringSchema(80), 2, 8).WithDescription("Steps in this lane, one entry per step column (empty = no shape). Give every step its own column — empty strings in the other lanes — so the arrows, which connect the columns left to right, follow the process; when two lanes hold a step in the same column, state the order in values.flow. Approximate readable chars per step by step columns x lanes (lanes 2/3/4/5/6): steps 2: 80/80/80/80/52; 3: 80/80/80/62/32; 4: 80/80/75/50/25; 5: 80/77/47/32/17; 6: 80/75/45/30/15; 7: 80/51/31/21/11; 8: 78/50/30/20/10"),
		},
		[]string{"actor", "steps"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"lanes": ArraySchema(laneSchema, 2, 6).WithDescription("Horizontal lanes (2-6 actors), each bounded by full-width hairline rules with its actor label at the left"),
			"flow":  ArraySchema(ArraySchema(IntegerSchema(0, 7), 2, 2), 2, 64).WithDescription("Order of the arrows as [lane, step] pairs (both 0-based): one arrow from each pair to the next. Omit it when every step column holds one step — the arrows then connect the columns left to right. Set it when lanes share a column (SWIMLANE_FLOW_AMBIGUOUS otherwise) or the process loops back. Each pair must name a non-empty step"),
		},
		[]string{"lanes"},
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
	}).WithDescription("Horizontal swimlane diagram with actors and steps")
}

func (s *swimlane) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*SwimlaneValues)
	if !ok || vals == nil {
		return fmt.Errorf("swimlane: values must be *SwimlaneValues, got %T", values)
	}

	const name = "swimlane"
	var errs []error

	if len(vals.Lanes) < 2 {
		errs = append(errs, errMinItems(name, "lanes", 2, len(vals.Lanes), ""))
	}
	if len(vals.Lanes) > 6 {
		errs = append(errs, errMaxItems(name, "lanes", 6, len(vals.Lanes), ""))
	}

	// All lanes must have the same number of steps.
	var stepCount int
	for i, lane := range vals.Lanes {
		if lane.Actor == "" {
			errs = append(errs, errRequired(name, fmt.Sprintf("lanes[%d].actor", i)))
		} else if runeLen(lane.Actor) > 40 {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("lanes[%d].actor", i), 40, runeLen(lane.Actor)))
		}

		if len(lane.Steps) < 2 {
			errs = append(errs, errMinItems(name, fmt.Sprintf("lanes[%d].steps", i), 2, len(lane.Steps), ""))
		}
		if len(lane.Steps) > 8 {
			errs = append(errs, errMaxItems(name, fmt.Sprintf("lanes[%d].steps", i), 8, len(lane.Steps), ""))
		}

		if i == 0 {
			stepCount = len(lane.Steps)
		} else if len(lane.Steps) != stepCount {
			errs = append(errs, newValidationError(name, fmt.Sprintf("lanes[%d].steps", i), ErrCodeCountMismatch,
				fmt.Sprintf("swimlane: all lanes must have the same number of steps (lane 0 has %d, lane %d has %d)", stepCount, i, len(lane.Steps)),
				ResizeListFix(fmt.Sprintf("lanes[%d].steps", i), stepCount)))
		}

		for j, step := range lane.Steps {
			if step != "" && runeLen(step) > 80 {
				errs = append(errs, errMaxLength(name, fmt.Sprintf("lanes[%d].steps[%d]", i, j), 80, runeLen(step)))
			}
		}
	}

	errs = append(errs, validateSwimlaneFlow(name, vals)...)

	// Total cells: lanes * (1 actor label + steps)
	totalCells := 0
	for _, lane := range vals.Lanes {
		totalCells += 1 + len(lane.Steps)
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, totalCells, ""); coErr != nil {
		errs = append(errs, coErr)
	}

	return errors.Join(errs...)
}

// validateSwimlaneFlow checks that values.flow names existing, non-empty
// steps and never links a step to itself.
func validateSwimlaneFlow(name string, vals *SwimlaneValues) []error {
	if len(vals.Flow) == 0 {
		return nil
	}
	var errs []error
	if len(vals.Flow) < 2 {
		errs = append(errs, errMinItems(name, "flow", 2, len(vals.Flow), "(an arrow needs two steps; omit flow to connect the columns left to right)"))
	}
	for k, ref := range vals.Flow {
		path := fmt.Sprintf("flow[%d]", k)
		lane, step := ref[0], ref[1]
		if lane < 0 || lane >= len(vals.Lanes) {
			errs = append(errs, errOutOfRange(name, path+"[0]", 0, len(vals.Lanes)-1, lane))
			continue
		}
		if step < 0 || step >= len(vals.Lanes[lane].Steps) {
			errs = append(errs, errOutOfRange(name, path+"[1]", 0, len(vals.Lanes[lane].Steps)-1, step))
			continue
		}
		if vals.Lanes[lane].Steps[step] == "" {
			errs = append(errs, newValidationError(name, path, ErrCodeEmptyValue,
				fmt.Sprintf("swimlane: %s names lanes[%d].steps[%d], which is empty — flow pairs are [lane, step] (0-based) and must point at a step that has text", path, lane, step),
				ReshapeValueFix(path, "[lane, step] of a non-empty step", "[0, 0]")))
			continue
		}
		if k > 0 && vals.Flow[k-1] == ref {
			errs = append(errs, newValidationError(name, path, ErrCodeOutOfRange,
				fmt.Sprintf("swimlane: %s repeats flow[%d] — an arrow cannot run from a step to itself", path, k-1),
				RemoveFieldFix(path)))
		}
	}
	return errs
}

func (s *swimlane) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*SwimlaneValues)
	if !ok {
		return nil, fmt.Errorf("swimlane: values must be *SwimlaneValues, got %T", values)
	}
	ovr := &SwimlaneOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*SwimlaneOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("swimlane: overrides must be *SwimlaneOverrides, got %T", overrides)
		}
	}

	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	// Determine number of columns: 1 actor label + N steps
	stepCount := 0
	if len(vals.Lanes) > 0 {
		stepCount = len(vals.Lanes[0].Steps)
	}
	numCols := 1 + stepCount

	// Tiles and type follow the lane (go-slide-creator-0e0en): the lane's
	// share is the tile, and where every step and actor holds at the subhead
	// step inside it the lane is set in that step. Authored sizes are kept.
	sizing := swimlaneLaneSizing(ctx, len(vals.Lanes), stepCount)
	headerSize := ResolveSize(ovr.HeaderSize, scaleBodyPt)
	bodySize := ResolveSize(ovr.BodySize, scaleDenseBodyPt)
	if ovr.BodySize == 0 && ovr.HeaderSize == 0 && sizing.holdsAt(ctx, vals, scaleSubheadPt) {
		headerSize, bodySize = scaleSubheadPt, scaleSubheadPt
	}

	// Column widths: actor label gets 15%, steps split the rest
	cols := make([]float64, numCols)
	cols[0] = 15
	stepWidth := 85.0 / float64(stepCount)
	for i := 1; i < numCols; i++ {
		cols[i] = stepWidth
	}
	colsJSON, _ := json.Marshal(cols)

	cellIdx := 0
	// Every lane is bounded by full-width hairline rules — one above the
	// first lane, one between lanes, one below the last — so a step's lane is
	// read from the band it sits in, not inferred from its height on the
	// slide (go-slide-creator-jz5r9). The rules are row rules: they sit in
	// the lane gaps and take no row, so lane i is still grid row i for links
	// and overlay anchors.
	var rows []jsonschema.GridRowInput

	// Step tiles are the only filled shapes: one neutral tint, no outline
	// (go-slide-creator-pgdkp).
	laneFill := string(neutralFillJSON(NeutralTint8))
	laneLine := noLine

	for _, lane := range vals.Lanes {
		cells := make([]*jsonschema.GridCellInput, numCols)

		// The actor label is the lane's heading: bold dk1 text at the left
		// of its band, with no fill of its own. It was a 16% tile, the
		// heaviest shape on the slide (go-slide-creator-jz5r9).
		actorText := buildSwimlaneTextContent(lane.Actor, headerSize, true, "dk1", "l")
		cells[0] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Line:     noLine,
				Text:     actorText,
			},
		}
		applySwimlaneOverride(cells[0], cellOverrides, cellIdx, accent)
		cellIdx++

		// Step cells
		for j, step := range lane.Steps {
			if step == "" {
				// Empty position: an unpainted spacer, so the lane reads as
				// a flow with gaps rather than a table of blank tiles
				// (go-slide-creator-0b3f6). Connectors skip it.
				cells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "rect",
						Fill:     json.RawMessage(`"none"`),
						Line:     laneLine,
					},
				}
			} else {
				stepText := buildSwimlaneTextContent(pptx.ConvertMarkdownEmphasis(step), bodySize, false, "dk1", "ctr")
				cells[j+1] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(laneFill),
						Line:     laneLine,
						Text:     stepText,
					},
				}
			}
			applySwimlaneOverride(cells[j+1], cellOverrides, cellIdx, accent)
			cellIdx++
		}

		rule := "below"
		if len(rows) == 0 {
			rule = "both"
		}
		rows = append(rows, jsonschema.GridRowInput{Cells: cells, Rule: rule})
	}

	// A step tile takes its share of the lane, and never less than the longest
	// step needs, centred in its lane: the lane is the band between its
	// rules, and the air above and below the tiles is where a hand-off arrow
	// to the next lane is drawn. Tiles that filled their lane left that arrow
	// a few points to live in (go-slide-creator-jz5r9). A lane shorter than
	// the tile caps it.
	tileH := swimlaneTileHeightPt(ctx, rows, sizing)
	for _, row := range rows {
		for j, c := range row.Cells {
			if j > 0 && c != nil && c.Shape != nil && len(c.Shape.Text) > 0 {
				c.MaxHeight = tileH
			}
		}
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  swimlaneColGap(stepCount),
		RowGap:  swimlaneRowGap(len(vals.Lanes)),
		Rows:    rows,
		Links:   swimlaneLinks(vals, accent),
	}

	return grid, nil
}

// Gutters wide enough to carry a visible arrow between steps: the column
// gap holds a lane change's elbow turn plus an arrowhead, the row gap a
// same-column hand-off arrow. Dense grids narrow them so the step text keeps
// the room its budget was measured with (the 8-step x 6-lane schema maximum
// is pinned by TestSchemaMaximaStayReadable).
func swimlaneColGap(steps int) float64 {
	switch {
	case steps >= 7:
		return 6
	case steps == 6:
		return 10
	}
	return 14
}

func swimlaneRowGap(lanes int) float64 {
	switch {
	case lanes >= 6:
		return 3
	case lanes == 5:
		return 6
	case lanes == 4:
		return 8
	}
	return 10
}

// swimlaneTileMinPt is the height of a step tile whose text and lane both
// need less.
const swimlaneTileMinPt = 44.0

// A step tile takes swimlaneTileLaneShare of its lane's height, so the flow
// fills a tall lane (two or three lanes, or the larger business-template
// slide) the way it fills a short one, and no more than
// swimlaneTileMaxAspect of its own width, so a tile stays a landscape box.
// The fixed 44pt tile covered a quarter of a three-lane slide and needed a
// SLIDE_UNDERUSED threshold of its own (go-slide-creator-0e0en).
const (
	swimlaneTileLaneShare = 0.75
	swimlaneTileMaxAspect = 0.9
)

// swimlaneSizing is the room a lane gives its step tiles and actor label.
type swimlaneSizing struct {
	// colW and actorW are the step and actor column widths, laneH the lane
	// height and tileH the tile height the lane's share gives, all in points.
	colW, actorW, laneH, tileH float64
}

// swimlaneLaneSizing measures the lanes of a steps x lanes grid in the
// content area. The share is stated in the pattern's design points: the
// resolver grows point heights by the canvas scale on a larger slide
// (shapegrid.CanvasScaleFor), and the lane it is a share of already has the
// slide's size.
func swimlaneLaneSizing(ctx ExpandContext, lanes, steps int) swimlaneSizing {
	if lanes < 1 || steps < 1 {
		return swimlaneSizing{tileH: swimlaneTileMinPt}
	}
	contentW, contentH := contentAreaPt(ctx)
	s := swimlaneSizing{
		colW:   (contentW - float64(steps)*swimlaneColGap(steps)) * 0.85 / float64(steps),
		actorW: (contentW - float64(steps)*swimlaneColGap(steps)) * 0.15,
		laneH:  (contentH - float64(lanes-1)*swimlaneRowGap(lanes)) / float64(lanes),
	}
	canvas := shapegrid.CanvasScaleFor(ctx.SlideWidth, ctx.SlideHeight)
	s.tileH = math.Max(swimlaneTileMinPt, math.Min(s.laneH*swimlaneTileLaneShare/canvas, s.colW*swimlaneTileMaxAspect))
	return s
}

// holdsAt reports whether every step fits the lane's tile and every actor its
// lane at sizePt, as the writer measures them: no shrink stored, no word
// broken.
func (s swimlaneSizing) holdsAt(ctx ExpandContext, vals *SwimlaneValues, sizePt float64) bool {
	if s.colW <= 0 || s.laneH <= 0 {
		return false
	}
	fonts := ctx.themeFonts()
	canvas := shapegrid.CanvasScaleFor(ctx.SlideWidth, ctx.SlideHeight)
	for _, lane := range vals.Lanes {
		actor := buildSwimlaneTextContent(lane.Actor, sizePt, true, "dk1", "l")
		if writtenFitHeightPt(fonts, actor, s.actorW, 0) > s.laneH/canvas {
			return false
		}
		for _, step := range lane.Steps {
			if step == "" {
				continue
			}
			text := buildSwimlaneTextContent(pptx.ConvertMarkdownEmphasis(step), sizePt, false, "dk1", "ctr")
			if writtenFitHeightPt(fonts, text, s.colW, 0) > s.tileH {
				return false
			}
		}
	}
	return true
}

// swimlaneTileMaxFill bounds a tile by its content: no taller than this many
// times the written fit of the longest step, so a lane of one-word steps is
// not drawn as large boxes around a word (and still reads as the thin slide
// it is).
const swimlaneTileMaxFill = 1.3

// swimlaneTileHeightPt is the one height every step tile takes: the lane's
// share (swimlaneSizing.tileH) up to swimlaneTileMaxFill of the longest
// step's written fit at the step column width, and never less than that fit
// or swimlaneTileMinPt.
func swimlaneTileHeightPt(ctx ExpandContext, rows []jsonschema.GridRowInput, lane swimlaneSizing) float64 {
	need := swimlaneTileMinPt
	if lane.colW > 0 {
		fonts := ctx.themeFonts()
		for _, row := range rows {
			for j, c := range row.Cells {
				if j == 0 || c == nil || c.Shape == nil || len(c.Shape.Text) == 0 {
					continue
				}
				need = math.Max(need, writtenFitHeightPt(fonts, c.Shape.Text, lane.colW, 0))
			}
		}
	}
	return math.Max(need, math.Min(lane.tileH, need*swimlaneTileMaxFill))
}

// swimlaneLinks joins consecutive steps with accent arrows. The order is
// values.flow when it is given (go-slide-creator-v786r); otherwise reading
// order — column by column, top lane first within a column — which is the
// process when every column holds one step. A lane change in another column
// turns in the column gutter; a hand-off within one column runs straight
// across the lane rule. Empty positions are skipped (go-slide-creator-0b3f6).
func swimlaneLinks(vals *SwimlaneValues, accent string) []jsonschema.GridLinkInput {
	lanes := vals.Lanes
	steps := 0
	if len(lanes) > 0 {
		steps = len(lanes[0].Steps)
	}
	var order [][2]int // {grid row, grid column}
	if len(vals.Flow) > 0 {
		for _, ref := range vals.Flow {
			lane, step := ref[0], ref[1]
			if lane < 0 || lane >= len(lanes) || step < 0 || step >= len(lanes[lane].Steps) || lanes[lane].Steps[step] == "" {
				continue
			}
			order = append(order, [2]int{lane, step + 1})
		}
	} else {
		for j := 0; j < steps; j++ {
			for i, lane := range lanes {
				if j < len(lane.Steps) && lane.Steps[j] != "" {
					order = append(order, [2]int{i, j + 1})
				}
			}
		}
	}
	var links []jsonschema.GridLinkInput
	for k := 0; k+1 < len(order); k++ {
		links = append(links, jsonschema.GridLinkInput{
			From:      order[k],
			To:        order[k+1],
			Connector: &jsonschema.ConnectorSpecInput{Style: "arrow", Color: accent, Width: 1.5},
		})
	}
	return links
}

func buildSwimlaneTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
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
			{Content: content, Size: size, Bold: bold, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

func applySwimlaneOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*SwimlaneCellOverride)
	if !coOk {
		return
	}
	applyCellTextOverride(cell, cellOvr)
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
