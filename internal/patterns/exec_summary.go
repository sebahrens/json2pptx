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
// supporting sentence, plus an optional bottom-line bar.
// ---------------------------------------------------------------------------
//
// Layout (one row per point, thin rules between rows):
//
//   [ 1 ]  Bold lead-in statement            Supporting evidence sentence …
//   ─────────────────────────────────────────────────────────────────────────
//   [ 2 ]  …                                  …
//   ▌Bottom line: optional tinted conclusion bar spanning the full width
//
// Rows are content-sized: each row's height comes from the measured wrapped
// height of its lead and support text, so short summaries do not stretch into
// full-height blocks. Every row is pinned in points (min_height = max_height),
// so the block keeps its size and the pattern vertical_align default centres
// it; short summaries are padded (never past 1.6× their natural height) so
// the slide does not read as mostly empty.

func init() {
	Default().Register(&execSummary{})
}

type execSummary struct{}

// execSummaryBottomLineLabel is the flag's caption. Set in caps because it is a
// label, not a sentence — the statement next to it carries the words.
const execSummaryBottomLineLabel = "BOTTOM LINE"

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
	// execSummaryFlagColPct is the width of the "BOTTOM LINE" flag, wide enough
	// for the label at the support size without crowding the statement.
	execSummaryFlagColPct = 20.0
	// execSummaryFlagGapPt is the gap between the flag's point and the statement
	// box — small, so the two read as one callout.
	execSummaryFlagGapPt  = 4.0
	execSummaryMinFillPct = 62.0 // pad rows until the grid covers this share of the content height
	execSummaryMaxPad     = 1.6  // … but never beyond this multiple of a row's natural height
)

func (e *execSummary) Name() string { return "exec-summary" }
func (e *execSummary) Description() string {
	return "Executive summary of 3-5 bold lead-in statements, each with a supporting sentence, separated by thin rules; optional bottom-line bar"
}
func (e *execSummary) UseWhen() string {
	return "Executive summary or key-messages slide stating 3–5 conclusions as bold lead-ins with one supporting sentence each (answer-first / pyramid-principle style); prefer scqa-summary when the story must follow Situation / Complication / Questions / Answer, pull-quote for a single takeaway, card-grid when items are parallel features rather than conclusions"
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
			{Lead: "The core business is healthy but growth is plateauing", Support: "Revenue grew 4% in FY25 versus 11% for the market; share loss is concentrated in mid-market accounts."},
			{Lead: "Cost to serve is the main margin drag", Support: "Service costs rose 18% while volumes rose 6%, driven by manual onboarding and fragmented tooling."},
			{Lead: "Digital self-service can close the gap within 18 months", Support: "Peers that automated onboarding cut cost to serve 25–30% and lifted retention by 4 points."},
			{Lead: "We recommend a three-wave transformation starting in Q1", Support: "Wave 1 targets quick wins worth $12M run-rate and funds the platform investment in waves 2–3."},
		},
		BottomLine: "Approve the $8M wave-1 budget to start in Q1 and capture $12M run-rate savings by year end.",
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
}

// ExecSummaryCellOverride is the shared per-cell override, indexed by point.
type ExecSummaryCellOverride = CellOverride

func (e *execSummary) NewValues() any       { return &ExecSummaryValues{} }
func (e *execSummary) NewOverrides() any    { return &ExecSummaryOverrides{} }
func (e *execSummary) NewCellOverride() any { return &ExecSummaryCellOverride{} }

func (e *execSummary) Schema() *Schema {
	pointSchema := ObjectSchema(
		map[string]*Schema{
			"lead":    StringSchema(execSummaryLeadMax).WithDescription("Bold lead-in statement — the conclusion, stated as a full sentence (≤90 chars)"),
			"support": StringSchema(execSummarySupportMax).WithDescription("One supporting sentence with the evidence (≤200 chars)"),
		},
		[]string{"lead"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"points":      ArraySchema(pointSchema, execSummaryMinPoints, execSummaryMaxPoints).WithDescription("3-5 key messages, most important first"),
			"bottom_line": StringSchema(execSummaryBottomLineMax).WithDescription("Optional recommendation / ask rendered as a tinted bar under the points"),
		},
		[]string{"points"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color for numbers and the bottom-line bar (default accent1)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":      NumberSchema(12, 40).WithDescription("Lead-in font size in points (default 17; 16 with 5 points)"),
			"body_size":        NumberSchema(12, 40).WithDescription("Support text font size in points (default 14; 13 with 5 points)"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-point accent rotation for the numbers"),
			"numbered":         BooleanSchema().WithDescription("Show the 1..N number column (default true)"),
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
	}).WithDescription("Executive summary: 3-5 bold lead-in statements with supporting sentences, separated by rules, plus an optional bottom-line bar")
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
		} else if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
			errs = append(errs, err)
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
		} else if len(p.Lead) > execSummaryLeadMax {
			errs = append(errs, errMaxLength(name, leadPath, execSummaryLeadMax, len(p.Lead)))
		}
		if len(p.Support) > execSummarySupportMax {
			errs = append(errs, errMaxLength(name, fmt.Sprintf("points[%d].support", i), execSummarySupportMax, len(p.Support)))
		}
	}
	if len(vals.BottomLine) > execSummaryBottomLineMax {
		errs = append(errs, errMaxLength(name, "bottom_line", execSummaryBottomLineMax, len(vals.BottomLine)))
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
		lay = measureExecSummary(ctx, vals, cols, numbered, st[0], st[1], areaW)
		if lay.natural() <= areaH {
			break
		}
	}
	leadSize, supportSize, numSize := lay.leadSize, lay.supportSize, lay.numSize
	rowPt, bottomPt := lay.rowPt, lay.bottomPt
	rowCount := lay.rowCount()
	fixedPt := lay.fixedPt()

	// Pad point rows (whitespace only — they are unfilled) so a short
	// summary does not read as a thin strip in the middle of the slide.
	if target := areaH * execSummaryMinFillPct / 100; lay.natural() < target {
		pointsPt := lay.natural() - fixedPt
		scale := math.Min((target-fixedPt)/pointsPt, execSummaryMaxPad)
		for i := range rowPt {
			rowPt[i] *= scale
		}
	}
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
		accent := ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		cells := make([]*jsonschema.GridCellInput, 0, len(cols))
		if numbered {
			cells = append(cells, execSummaryTextCell([]chartInsightsParagraph{
				{Content: strconv.Itoa(i + 1), Size: numSize, Bold: true, Color: inkOnLight(ctx, accent, 3.0), Align: "l"},
			}, "ctr"))
		}
		leadCell := execSummaryTextCell([]chartInsightsParagraph{
			{Content: pptx.ConvertMarkdownEmphasis(p.Lead), Size: leadSize, Bold: true, Color: leadInk, Align: "l"},
		}, "ctr")
		if co, ok := cellOverrides[i].(*ExecSummaryCellOverride); ok && co.AccentBar {
			leadCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
		}
		cells = append(cells, leadCell)
		cells = append(cells, execSummaryTextCell([]chartInsightsParagraph{
			{Content: pptx.ConvertMarkdownEmphasis(p.Support), Size: supportSize, Color: "dk1", Align: "l"},
		}, "ctr"))
		rows = append(rows, jsonschema.GridRowInput{MinHeight: rowPt[i], MaxHeight: rowPt[i], Cells: cells})
	}

	if bottomPt > 0 {
		// The ask is a labelled callout, not a fourth pale band. A full-width
		// tinted rectangle with a left stripe read as a near-twin of the
		// generator's takeaway band sitting right below it — two similar pale
		// rectangles stacked, neither one reading as the conclusion. It is now a
		// rule that closes the list, a saturated accent flag whose point aims
		// into the statement, and the statement in its own tinted box.
		rows = append(rows,
			jsonschema.GridRowInput{
				MinHeight: execSummaryRulePt, MaxHeight: execSummaryRulePt,
				Cells: []*jsonschema.GridCellInput{{
					ColSpan: len(cols),
					Shape:   &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: ruleFill},
				}},
			},
			jsonschema.GridRowInput{
				MinHeight: bottomPt, MaxHeight: bottomPt,
				Cells: []*jsonschema.GridCellInput{{
					ColSpan: len(cols),
					Grid:    execSummaryBottomLine(ctx, vals.BottomLine, baseAccent, supportSize+1),
				}},
			},
		)
	}

	colsJSON, _ := json.Marshal(cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  execSummaryColGapPt,
		RowGap:  execSummaryRowGapPt,
		Rows:    rows,
	}
	return grid, nil
}

// execSummaryBottomLine renders the ask as a pointing accent flag followed by
// the statement in a tinted box. The flag is a homePlate — a pentagon whose
// point aims right, into the text it introduces — so the callout reads as one
// object with a direction, rather than as another horizontal band.
func execSummaryBottomLine(ctx ExpandContext, bottomLine, accent string, sizePt float64) *jsonschema.ShapeGridInput {
	flagText, _ := json.Marshal(chartInsightsText{
		Paragraphs: []chartInsightsParagraph{{
			Content: execSummaryBottomLineLabel,
			Size:    sizePt - 2,
			Bold:    true,
			Color:   readableTextOn(ctx, fillTone{Color: accent}, "lt1"),
			Align:   "ctr",
		}},
		Align:         "ctr",
		VerticalAlign: "ctr",
	})

	tone := inactiveTintTone(accent)
	statementText, _ := json.Marshal(chartInsightsText{
		Paragraphs: []chartInsightsParagraph{{
			Content: pptx.ConvertMarkdownEmphasis(bottomLine),
			Size:    sizePt,
			Color:   readableTextOn(ctx, tone, "dk1"),
			Align:   "l",
		}},
		Align:         "l",
		VerticalAlign: "ctr",
	})

	colsJSON, _ := json.Marshal([]float64{execSummaryFlagColPct, 100 - execSummaryFlagColPct})
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  execSummaryFlagGapPt,
		Rows: []jsonschema.GridRowInput{{
			Cells: []*jsonschema.GridCellInput{
				{Shape: &jsonschema.ShapeSpecInput{
					Geometry: "homePlate",
					Fill:     json.RawMessage(strconv.Quote(accent)),
					Text:     flagText,
				}},
				{Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     tone.fillJSON(),
					Text:     statementText,
				}},
			},
		}},
	}
}

// execSummaryTextCell builds an unfilled text cell.
func execSummaryTextCell(paras []chartInsightsParagraph, vAlign string) *jsonschema.GridCellInput {
	textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: "l", VerticalAlign: vAlign})
	return &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Text:     textJSON,
		},
	}
}

// execSummaryLayout is the measured natural geometry at one type scale.
type execSummaryLayout struct {
	leadSize, supportSize, numSize float64
	rowPt                          []float64 // natural height per point row
	bottomPt                       float64   // bottom-line bar height (0 = none)
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
func measureExecSummary(ctx ExpandContext, vals *ExecSummaryValues, cols []float64, numbered bool, leadSize, supportSize, areaW float64) execSummaryLayout {
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
	for i, p := range vals.Points {
		lead := sizedBlockHeightPt(ctx, []sizedPara{{text: p.Lead, sizePt: leadSize, bold: true}}, colW(leadCol))
		support := sizedBlockHeightPt(ctx, []sizedPara{{text: p.Support, sizePt: supportSize}}, colW(supportCol))
		h := math.Max(lead, support)
		if numbered {
			h = math.Max(h, lay.numSize*sizingLineSpacing+2*sizingInsetTBPt)
		}
		lay.rowPt[i] = h
	}
	if strings.TrimSpace(vals.BottomLine) != "" {
		lay.bottomPt = sizedBlockHeightPt(ctx, []sizedPara{{text: "<b>Bottom line:</b> " + vals.BottomLine, sizePt: supportSize + 1, bold: true}}, areaW)
	}
	return lay
}
