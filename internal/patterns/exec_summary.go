package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// ---------------------------------------------------------------------------
// exec-summary pattern — 3-5 bold lead-in statements, each with a short
// supporting sentence, plus an optional bottom-line takeaway band.
// ---------------------------------------------------------------------------
//
// Layout (one row per point, thin rules between rows):
//
//   [ 1 ]  Bold lead-in statement            Supporting evidence sentence …
//   ─────────────────────────────────────────────────────────────────────────
//   [ 2 ]  …                                  …
//   ▌Bottom line: optional takeaway band (flush accent bar, bold text)
//
// Rows start at their measured text height and short summaries distribute
// surplus content-zone height into the point rows. Dividers and the bottom
// line retain their own heights.

func init() {
	Default().Register(&execSummary{})
}

type execSummary struct{}

// Pattern budgets.
const (
	execSummaryMinPoints     = 3
	execSummaryMaxPoints     = 5
	execSummaryLeadMax       = 90
	execSummarySupportMax    = 200
	execSummaryBottomLineMax = 160

	execSummaryNumColPct  = 5.0
	execSummaryLeadColPct = 36.0
	execSummaryColGapPt   = 12.0
	execSummaryRowGapPt   = 7.0
	execSummaryRulePt     = 0.75
	execSummaryMinFillPct = 62.0
)

func (e *execSummary) Name() string { return "exec-summary" }
func (e *execSummary) Description() string {
	return "Executive summary of 3-5 bold lead-in statements, each with a supporting sentence, separated by thin rules; optional bottom-line takeaway band"
}
func (e *execSummary) UseWhen() string {
	return "Executive summary or key-messages slide stating 3–5 conclusions as bold lead-ins with one supporting sentence each (answer-first / pyramid-principle style); prefer scqa-summary when the story must follow Situation / Complication / Questions / Answer, pull-quote for a single takeaway, card-grid when items are parallel features rather than conclusions, labeled-rows when each row is a keyword label (WHY / WHAT / HOW) with body text, metric-list when each row leads with a number"
}
func (e *execSummary) NotWhen() string {
	return "The narrative is an explicit SCQA arc (use scqa-summary), there is one headline message (use pull-quote or stat-hero), items are a deck section list (use agenda), or there are fewer than 3 / more than 5 messages (split or merge)"
}
func (e *execSummary) Version() int      { return 1 }
func (e *execSummary) CellsHint() string { return "3-5 (rows)" }
func (e *execSummary) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"open", "conclude"},
		PairsWith:     []string{"kpi-3up", "chart-insights-split", "table-highlight"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}

func (e *execSummary) SupportsInlineMarkdown() bool { return true }

func (e *execSummary) ExemplarValues() any {
	return &ExecSummaryValues{
		Points: []ExecSummaryPoint{
			{Lead: "The core is healthy but growth is slowing", Support: "Revenue grew 4% in FY25 versus 11% for the market."},
			{Lead: "Cost to serve is the main margin drag", Support: "Service costs rose 18% while volumes rose 6%."},
			{Lead: "Self-service can close the gap in 18 months", Support: "Peers that automated onboarding cut cost to serve 25–30%."},
			{Lead: "We recommend a three-wave transformation", Support: "Wave 1 targets $12M run-rate and funds waves 2–3."},
		},
		BottomLine: "Approve the $8M wave-1 budget to start in Q1.",
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ExecSummaryPoint is one key message: a bold lead-in plus optional support.
type ExecSummaryPoint struct {
	Lead    string `json:"lead"`
	Support string `json:"support,omitempty"`
}

// ExecSummaryValues holds the 3-5 key messages and the optional bottom line.
type ExecSummaryValues struct {
	Points     []ExecSummaryPoint `json:"points"`
	BottomLine string             `json:"bottom_line,omitempty"`
}

// ExecSummaryOverrides embeds the shared text overrides (accent, sizes,
// cell_accent_mode for the numbers) plus a numbering toggle.
type ExecSummaryOverrides struct {
	TextOverrides
	Numbered *bool `json:"numbered,omitempty"` // default true
	// TakeawayEmphasis styles the bottom-line band: "" (accent bar only),
	// "subtle" (5% neutral tint) or "strong" (solid accent).
	TakeawayEmphasis string `json:"takeaway_emphasis,omitempty"`
}

// ExecSummaryCellOverride is the shared per-cell override, indexed by point.
type ExecSummaryCellOverride = CellOverride

func (e *execSummary) NewValues() any       { return &ExecSummaryValues{} }
func (e *execSummary) NewOverrides() any    { return &ExecSummaryOverrides{} }
func (e *execSummary) NewCellOverride() any { return &ExecSummaryCellOverride{} }

// execSummaryBudgets returns the per-point lead limit and the average support
// budget, measured against the written size (no run stored below its role
// floor) on every shipped template with every point at the same length and
// every shape keeping the uniform 0.5 cm text margin (go-slide-creator-n1muf).
// A bottom line longer than a short ask (about 40 characters) takes room from
// the supports.
func execSummaryBudgets(points int, bottomLine string) (lead, support int) {
	bottom := runeLen(strings.TrimSpace(bottomLine))
	switch {
	case points >= 5, points == 4 && bottom > 0:
		return 42, 75
	case points == 4:
		return 82, 150
	case bottom > 40:
		return 88, 68
	case bottom > 0:
		return 88, 152
	default:
		return execSummaryLeadMax, execSummarySupportMax
	}
}

// execSummaryBottomLineBudget is the readable bottom-line length: the schema
// maximum with three points, about 102 characters with four or five.
func execSummaryBottomLineBudget(points int) int {
	if points >= 4 {
		return 102
	}
	return execSummaryBottomLineMax
}

func (e *execSummary) PostExpandWarnings(_ ExpandContext, values, _ any) []string {
	v, ok := values.(*ExecSummaryValues)
	if !ok || v == nil {
		return nil
	}
	leadBudget, supportBudget := execSummaryBudgets(len(v.Points), v.BottomLine)
	var warnings []string
	total := 0
	for i, point := range v.Points {
		total += runeLen(point.Support)
		if n := runeLen(point.Lead); n > leadBudget {
			warnings = append(warnings, fmt.Sprintf("%s: exec-summary points[%d].lead is %d characters; %d points hold about %d lead characters each with this bottom line — shorten the lead or split the summary", ErrCodeBodyTooLong, i, n, len(v.Points), leadBudget))
		}
	}
	if b := execSummaryBottomLineBudget(len(v.Points)); runeLen(strings.TrimSpace(v.BottomLine)) > b {
		warnings = append(warnings, fmt.Sprintf("%s: exec-summary bottom_line is %d characters; %d points leave room for about %d readable bottom-line characters — shorten the ask or use three points", ErrCodeBodyTooLong, runeLen(strings.TrimSpace(v.BottomLine)), len(v.Points), b))
	}
	if total > supportBudget*len(v.Points) {
		warnings = append(warnings, fmt.Sprintf("%s: exec-summary points.support contains %d characters across %d points; the shared space holds about %d support characters per point with this bottom line — shorten supporting sentences or split the summary", ErrCodeBodyTooLong, total, len(v.Points), supportBudget))
	}
	return warnings
}

func (e *execSummary) Schema() *Schema {
	pointSchema := ObjectSchema(
		map[string]*Schema{
			"lead":    StringSchema(execSummaryLeadMax).WithDescription("Bold lead-in statement — the conclusion, stated as a full sentence (≤90 chars)"),
			"support": StringSchema(execSummarySupportMax).WithDescription("One supporting sentence with the evidence (≤200 chars); average support per point: 3 points with a bottom line about 152 (68 with one over 40 characters); 4 points about 150 (75 with a bottom line); 5 points about 75. Leads: about 88 with 3 points and a bottom line, 82 with 4 points, 42 with 4 points and a bottom line or with 5"),
		},
		[]string{"lead"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"points":      ArraySchema(pointSchema, execSummaryMinPoints, execSummaryMaxPoints).WithDescription("3-5 key messages, most important first"),
			"bottom_line": StringSchema(execSummaryBottomLineMax).WithDescription("Optional recommendation / ask rendered as the takeaway band under the points (flush accent bar, bold dk1 text, no box); about 102 readable characters with 4-5 points"),
		},
		[]string{"points"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":            StringSchema(0).WithDescription("Accent scheme color for numbers and the bottom-line accent bar (default accent1)").WithDefault("accent1"),
			"semantic_accent":   EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":       NumberSchema(12, 40).WithDescription("Lead-in font size in points (default 17; 16 with 5 points)"),
			"body_size":         NumberSchema(12, 40).WithDescription("Support text font size in points (default 14; 13 with 5 points)"),
			"cell_accent_mode":  EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-point accent rotation for the numbers"),
			"numbered":          BooleanSchema().WithDescription("Show the 1..N number column (default true)"),
			"takeaway_emphasis": TakeawayEmphasisSchema(),
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
	}).WithDescription("Executive summary: 3-5 bold lead-in statements with supporting sentences, separated by rules, plus an optional bottom-line takeaway band")
}

func (e *execSummary) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*ExecSummaryValues)
	if !ok || vals == nil {
		return fmt.Errorf("exec-summary: values must be *ExecSummaryValues, got %T", values)
	}
	const name = "exec-summary"
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*ExecSummaryOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("exec-summary: overrides must be *ExecSummaryOverrides, got %T", overrides))
		} else {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			if err := validateTakeawayEmphasis(name, ovr.TakeawayEmphasis); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(vals.Points) < execSummaryMinPoints {
		errs = append(errs, errMinItems(name, "points", execSummaryMinPoints, len(vals.Points), "(hint: use pull-quote or stat-hero for one or two messages)"))
	}
	if len(vals.Points) > execSummaryMaxPoints {
		errs = append(errs, errMaxItems(name, "points", execSummaryMaxPoints, len(vals.Points), "(hint: merge messages or split across two slides)"))
	}
	for i, p := range vals.Points {
		leadPath := fmt.Sprintf("points[%d].lead", i)
		if strings.TrimSpace(p.Lead) == "" {
			errs = append(errs, errRequired(name, leadPath))
		} else if runeLen(p.Lead) > execSummaryLeadMax {
			errs = append(errs, errMaxLength(name, leadPath, execSummaryLeadMax, runeLen(p.Lead)))
		}
		if runeLen(p.Support) > execSummarySupportMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("points[%d].support", i), execSummarySupportMax, runeLen(p.Support)))
		}
	}
	if runeLen(vals.BottomLine) > execSummaryBottomLineMax {
		errs = append(errs, errMaxLength(name, "bottom_line", execSummaryBottomLineMax, runeLen(vals.BottomLine)))
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Points), "(index = point)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

func (e *execSummary) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*ExecSummaryValues)
	if !ok {
		return nil, fmt.Errorf("exec-summary: values must be *ExecSummaryValues, got %T", values)
	}
	ovr := &ExecSummaryOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*ExecSummaryOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("exec-summary: overrides must be *ExecSummaryOverrides, got %T", overrides)
		}
	}

	n := len(vals.Points)
	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	numbered := ovr.Numbered == nil || *ovr.Numbered
	leadInk := inkOnLight(ctx, "dk2", 4.5)

	// Column geometry.
	cols := []float64{execSummaryLeadColPct, 100 - execSummaryLeadColPct}
	if numbered {
		cols = []float64{execSummaryNumColPct, execSummaryLeadColPct, 100 - execSummaryNumColPct - execSummaryLeadColPct}
	}
	areaW, areaH := sizingAreaPt(ctx)

	// Type scale: the largest step whose natural height fits the content
	// area (never below the 12pt readability floor). Explicit header_size /
	// body_size overrides pin the scale.
	steps := [][2]float64{{17, 14}, {16, 13}, {15, 12}, {14, 12}}
	if n >= execSummaryMaxPoints {
		steps = steps[1:]
	}
	if ovr.HeaderSize > 0 || ovr.BodySize > 0 {
		steps = [][2]float64{{ResolveSize(ovr.HeaderSize, steps[0][0]), ResolveSize(ovr.BodySize, steps[0][1])}}
	}
	var lay execSummaryLayout
	for _, st := range steps {
		lay = measureExecSummary(ctx, vals, cols, numbered, st[0], st[1], areaW, ovr.TakeawayEmphasis)
		if lay.natural() <= areaH {
			break
		}
	}
	leadSize, supportSize, numSize := lay.leadSize, lay.supportSize, lay.numSize
	rowPt, bottomPt := lay.rowPt, lay.bottomPt
	rowCount := lay.rowCount()

	ruleFill := fillTone{Color: "dk1", Alpha: 30}.fillJSON()
	rows := make([]jsonschema.GridRowInput, 0, rowCount)
	for i, p := range vals.Points {
		if i > 0 {
			rows = append(rows, jsonschema.GridRowInput{
				MinHeight: execSummaryRulePt, MaxHeight: execSummaryRulePt,
				Cells: []*jsonschema.GridCellInput{{
					ColSpan: len(cols),
					Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ruleFill},
				}},
			})
		}
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		cells := make([]*jsonschema.GridCellInput, 0, len(cols))
		// Every cell in a row is top-anchored so the lead and its support share
		// a first baseline. Centred, the support floated between the two lines
		// of a wrapping lead, and across five rows the right column visibly
		// stair-stepped — a consulting exec summary aligns the first baseline of
		// every pair (go-slide-creator-kol0). The smaller text is nudged down by
		// the difference in ascent so "top-anchored" means "same baseline"
		// rather than "same box edge".
		// The row's tallest text sets the shared baseline: the number when the
		// summary is numbered, the lead otherwise.
		tallest := leadSize
		if numbered {
			tallest = numSize
			cells = append(cells, execSummaryTextCell([]chartInsightsParagraph{
				{Content: strconv.Itoa(i + 1), Size: numSize, Bold: true, Color: inkOnLight(ctx, accent, 3.0), Align: "l"},
			}, "t", baselineInsetPt(tallest, numSize)))
		}
		leadCell := execSummaryTextCell([]chartInsightsParagraph{
			{Content: pptx.ConvertMarkdownEmphasis(p.Lead), Size: leadSize, Bold: true, Color: leadInk, Align: "l"},
		}, "t", baselineInsetPt(tallest, leadSize))
		if co, ok := cellOverrides[i].(*ExecSummaryCellOverride); ok {
			applyCellTextOverride(leadCell, co)
			if co.AccentBar {
				leadCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
			}
		}
		cells = append(cells, leadCell)
		cells = append(cells, execSummaryTextCell([]chartInsightsParagraph{
			{Content: pptx.ConvertMarkdownEmphasis(p.Support), Size: supportSize, Color: "dk1", Align: "l"},
		}, "t", baselineInsetPt(tallest, supportSize)))
		rows = append(rows, jsonschema.GridRowInput{MinHeight: rowPt[i], MaxHeight: rowPt[i], Cells: cells})
	}

	if bottomPt > 0 {
		// The ask is the shared takeaway component: a flush accent bar and
		// bold dk1 text, no box, no outline (go-slide-creator-7b5o6). It
		// replaced a closing rule + chevron "BOTTOM LINE" flag + tinted box.
		rows = append(rows, TakeawayRow(ctx, execSummaryTakeaway(vals.BottomLine, baseAccent, ovr.TakeawayEmphasis), len(cols), areaW, execSummaryRowGapPt))
	}

	colsJSON, _ := json.Marshal(cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  execSummaryColGapPt,
		RowGap:  execSummaryRowGapPt,
		Rows:    rows,
	}
	fillCappedRows(ctx, grid.Rows, grid.RowGap, execSummaryMinFillPct/100, func(i int) bool {
		return i < 2*n-1 && i%2 == 0
	})
	return grid, nil
}

// execSummaryTakeaway is the bottom line as a takeaway band spec.
func execSummaryTakeaway(bottomLine, accent, emphasis string) TakeawaySpec {
	return TakeawaySpec{Text: pptx.ConvertMarkdownEmphasis(bottomLine), Accent: accent, Emphasis: emphasis}
}

// execSummaryTextCell builds one unfilled row cell: paragraphs, a vertical
// anchor, and a baseline nudge in points that puts the first baseline where
// the row wants it. The nudge is ADDED to the uniform top margin — a pattern
// may push text further from an edge to align baselines, never closer.
func execSummaryTextCell(paras []chartInsightsParagraph, vAlign string, nudge float64) *jsonschema.GridCellInput {
	insetTop := 0.0
	if nudge > 0 {
		insetTop = defaultShapeInsetTBPt + nudge
	}
	textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: "l", VerticalAlign: vAlign, InsetTop: insetTop})
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     textJSON,
		},
	}
}

// execSummaryAscentRatio approximates a font's ascent as a fraction of its
// point size. It only has to be consistent across the cells of one row: the
// inset it produces is the DIFFERENCE between two ascents, so a systematic
// error cancels.
const execSummaryAscentRatio = 0.8

// baselineInsetPt returns how far a cell of size pt must be nudged down to put
// its first baseline on the baseline of the row's tallest text.
func baselineInsetPt(tallest, pt float64) float64 {
	if d := (tallest - pt) * execSummaryAscentRatio; d > 0 {
		return d
	}
	return 0
}

// execSummaryLayout is the measured natural geometry at one type scale.
type execSummaryLayout struct {
	leadSize, supportSize, numSize float64
	rowPt                          []float64 // natural height per point row
	bottomPt                       float64   // bottom-line takeaway row height (0 = none)
}

func (l execSummaryLayout) rules() int { return len(l.rowPt) - 1 }

func (l execSummaryLayout) rowCount() int {
	c := len(l.rowPt) + l.rules()
	if l.bottomPt > 0 {
		c++
	}
	return c
}

func (l execSummaryLayout) gapsPt() float64 {
	return execSummaryRowGapPt * float64(l.rowCount()-1)
}

// fixedPt is everything that is not a point row: rules, bottom bar, gaps.
func (l execSummaryLayout) fixedPt() float64 {
	return float64(l.rules())*execSummaryRulePt + l.bottomPt + l.gapsPt()
}

func (l execSummaryLayout) natural() float64 {
	t := l.fixedPt()
	for _, h := range l.rowPt {
		t += h
	}
	return t
}

// measureExecSummary measures every row at the given lead / support sizes.
func measureExecSummary(ctx ExpandContext, vals *ExecSummaryValues, cols []float64, numbered bool, leadSize, supportSize, areaW float64, emphasis string) execSummaryLayout {
	usableW := areaW - execSummaryColGapPt*float64(len(cols)-1)
	colW := func(i int) float64 { return usableW * cols[i] / 100 }
	leadCol, supportCol := 0, 1
	if numbered {
		leadCol, supportCol = 1, 2
	}
	lay := execSummaryLayout{
		leadSize:    leadSize,
		supportSize: supportSize,
		numSize:     math.Max(leadSize+8, 22),
		rowPt:       make([]float64, len(vals.Points)),
	}
	tallest := leadSize
	if numbered {
		tallest = lay.numSize
	}
	for i, p := range vals.Points {
		// A baseline nudge adds to the cell's top margin (execSummaryTextCell).
		lead := sizedBlockHeightPt(ctx, []sizedPara{{text: p.Lead, sizePt: leadSize, bold: true}}, colW(leadCol)) + baselineInsetPt(tallest, leadSize)
		support := sizedBlockHeightPt(ctx, []sizedPara{{text: p.Support, sizePt: supportSize}}, colW(supportCol)) + baselineInsetPt(tallest, supportSize)
		h := math.Max(lead, support)
		if numbered {
			h = math.Max(h, lay.numSize*sizingLineSpacing+2*sizingInsetTBPt)
		}
		lay.rowPt[i] = h
	}
	if strings.TrimSpace(vals.BottomLine) != "" {
		lay.bottomPt = TakeawayRowHeightPt(ctx, execSummaryTakeaway(vals.BottomLine, "", emphasis), areaW, execSummaryRowGapPt)
	}
	return lay
}
