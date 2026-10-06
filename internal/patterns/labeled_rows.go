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
// labeled-rows pattern — 2-6 rows, each a keyword label block on the left and
// 1-4 lines of body text on the right (WHY / WHAT / HOW; Smarter / Faster /
// Leaner; Adopt / Adapt / Assemble).
// ---------------------------------------------------------------------------
//
// Layout (label_style "tinted", the default: a neutral-tint block under a
// thin accent rule with the keyword in accent ink; "filled" is the legacy
// solid accent block):
//
//   ┌──────────────┐
//   │ WHY          │  The industry grew 11% in 2025, but most of the growth
//   │ now is the   │  came from market performance. **Fee compression …**
//   │ right time   │
//   └──────────────┘
//   ────────────────────────────────────────────────────────────────────
//   ┌──────────────┐
//   │ WHAT …       │  …
//
// label_style "text" drops the fill and sets the keyword in accent-coloured
// bold type instead, for a lighter page (a list of benefits beside a diagram).
//
// Rows are content-sized: each row is as tall as its taller side at the
// largest type scale that fits the content area, and short slides distribute
// surplus height into the rows so the block reads as the slide's content.

func init() {
	Default().Register(&labeledRows{})
}

type labeledRows struct{}

// Pattern budgets.
const (
	labeledRowsMinRows     = 2
	labeledRowsMaxRows     = 6
	labeledRowsLabelMax    = 24
	labeledRowsSublabelMax = 60
	labeledRowsBodyMax     = 300

	labeledRowsDefaultLabelPct = 22.0
	labeledRowsMinLabelPct     = 12.0
	labeledRowsMaxLabelPct     = 40.0

	labeledRowsColGapPt    = 14.0
	labeledRowsRowGapPt    = 4.0
	labeledRowsBlockInset  = defaultShapeInsetLRPt // label block text inset: the uniform shape margin
	labeledRowsMinLabelPt  = 12.0
	labeledRowsMinFillFrac = 0.60

	labeledRowsStyleTab    = "tab"
	labeledRowsStyleTinted = "tinted"
	labeledRowsStyleFilled = "filled"
	labeledRowsStyleText   = "text"

	// labeledRowsTabPointPt is the depth of a tab's point: the pentagon
	// points into the row it labels.
	labeledRowsTabPointPt = 12.0
	// labeledRowsTabInsetRight is a tab's right text inset: the uniform
	// margin less the half point the preset's text rectangle already keeps
	// clear, so the keyword has the width a plain block gave it.
	labeledRowsTabInsetRight = defaultShapeInsetLRPt - labeledRowsTabPointPt/2
)

// labeledRowsStyles are the accepted overrides.label_style values.
var labeledRowsStyles = []string{labeledRowsStyleTab, labeledRowsStyleTinted, labeledRowsStyleFilled, labeledRowsStyleText}

// labeledRowsStyle is the effective overrides.label_style.
func labeledRowsStyle(ovr *LabeledRowsOverrides) string {
	if ovr == nil || ovr.LabelStyle == "" {
		return labeledRowsStyleTab
	}
	return ovr.LabelStyle
}

func (l *labeledRows) Name() string { return "labeled-rows" }
func (l *labeledRows) Description() string {
	return "2-6 rows, each a keyword label on the left (a pale accent pentagon tab pointing into its row with a bold keyword + optional sublabel, a neutral-tint block under an accent rule, a solid accent block, or accent-coloured text) beside 1-4 lines of body text, rules between content-sized rows"
}
func (l *labeledRows) UseWhen() string {
	return "2–6 parallel themes each introduced by a short keyword label (WHY / WHAT / HOW, Smarter / Faster / Leaner, Adopt / Adapt / Assemble) followed by 1–4 lines of explanation; prefer exec-summary for 3–5 numbered sentence-length conclusions, metric-list when each row leads with a number, comparison-2col for two options side by side, scqa-summary for a Situation / Complication / Questions / Answer arc"
}
func (l *labeledRows) NotWhen() string {
	return "Each row leads with a number (use metric-list or kpi-Nup), the rows are sentence-length conclusions rather than keyword labels (use exec-summary), the content compares two options (use comparison-2col), the story is an SCQA arc (use scqa-summary), rows are ordered steps (use numbered-step-strip), or there is a single message (use pull-quote or stat-hero)"
}
func (l *labeledRows) Version() int      { return 1 }
func (l *labeledRows) CellsHint() string { return "2-6 (rows)" }
func (l *labeledRows) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"frame", "evidence", "conclude"},
		PairsWith:     []string{"metric-list", "kpi-3up", "chart-insights-split"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}

func (l *labeledRows) SupportsInlineMarkdown() bool { return true }

func (l *labeledRows) ExemplarValues() any {
	return &LabeledRowsValues{
		Rows: []LabeledRow{
			{Label: "WHY", Sublabel: "now is the right time to act", Body: "The industry grew 11% in 2025, but 80% of revenue growth came from market performance while fee compression continues. **Firms that wait will be disrupted.**"},
			{Label: "WHAT", Sublabel: "an AI-first firm looks like", Body: "Rebuilt with AI at its core: 2x to 5x research coverage, 3x client coverage per relationship manager, and 300–500 bps of value across the P&L."},
			{Label: "HOW", Sublabel: "to start the journey", Body: "Three plays — **Deploy**, **Reshape**, **Invent** — starting with two or three workflows reimagined end to end, owned by the business."},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// LabeledRow is one row: a keyword label, an optional sublabel, and the body.
type LabeledRow struct {
	Label    string `json:"label"`
	Sublabel string `json:"sublabel,omitempty"`
	Body     string `json:"body"`
}

// LabeledRowsValues holds the 2-6 rows.
type LabeledRowsValues struct {
	Rows []LabeledRow `json:"rows"`
}

// LabeledRowsOverrides are the pattern-level knobs.
type LabeledRowsOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	LabelStyle     string  `json:"label_style,omitempty"` // tinted (default) | filled | text
	LabelWidthPct  float64 `json:"label_width_pct,omitempty"`
	LabelSize      float64 `json:"label_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
	CellAccentMode string  `json:"cell_accent_mode,omitempty"`
}

// LabeledRowsCellOverride is the shared per-cell override, indexed by row.
type LabeledRowsCellOverride = CellOverride

func (l *labeledRows) NewValues() any       { return &LabeledRowsValues{} }
func (l *labeledRows) NewOverrides() any    { return &LabeledRowsOverrides{} }
func (l *labeledRows) NewCellOverride() any { return &LabeledRowsCellOverride{} }

func (l *labeledRows) Schema() *Schema {
	rowSchema := ObjectSchema(
		map[string]*Schema{
			"label":    StringSchema(labeledRowsLabelMax).WithDescription("Short keyword for the row (≤24 chars): \"WHY\", \"Faster\", \"Adopt\". It shrinks as one shared size until every word fits the label column"),
			"sublabel": StringSchema(labeledRowsSublabelMax).WithDescription("Optional smaller line under the keyword (≤60 chars): \"now is the right time to act\""),
			"body":     StringSchema(labeledRowsBodyMax).WithDescription("1-4 lines of explanation (≤300 chars); **bold** marks a key phrase. 2-3 rows hold the full 300 at default sizes; 4 rows about 211 and 5-6 rows about 101 per row, with no multi-line sublabels"),
		},
		[]string{"label", "body"},
	).WithAdditionalProperties(false)

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"rows": ArraySchema(rowSchema, labeledRowsMinRows, labeledRowsMaxRows).WithDescription("2-6 rows, top to bottom"),
		},
		[]string{"rows"},
	).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(
		map[string]*Schema{
			"accent":           StringSchema(0).WithDescription("Accent scheme color for the label rules, keywords and filled blocks (default: the template's color_roles.primary_fill)").WithDefault("accent1"),
			"semantic_accent":  EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"label_style":      EnumSchema(labeledRowsStyles...).WithDescription("tab (default): pale accent pentagon tab pointing into its row, bold keyword; tinted: neutral-tint block under a thin accent rule, keyword in bold accent type; filled: solid accent block with the keyword in measured-contrast text (legacy look); text: no fill, keyword in accent-coloured bold type").WithDefault(labeledRowsStyleTab),
			"label_width_pct":  NumberSchema(labeledRowsMinLabelPct, labeledRowsMaxLabelPct).WithDescription("Width of the label column as a percentage of the pattern width (default 22)"),
			"label_size":       NumberSchema(12, 40).WithDescription("Keyword font size in points (default 22 stepping down to 16 as content grows)"),
			"body_size":        NumberSchema(12, 28).WithDescription("Body font size in points (default 15 stepping down to 12); the sublabel is 2pt smaller than the body, never below 12"),
			"cell_accent_mode": EnumSchema("uniform", "alternate", "progressive").WithDescription("Per-row accent rotation for the label blocks"),
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
	}).WithDescription("Labeled rows: 2-6 rows of a keyword label block (tinted, filled or text) beside 1-4 lines of body text, rules between content-sized rows")
}

func (l *labeledRows) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*LabeledRowsValues)
	if !ok || vals == nil {
		return fmt.Errorf("labeled-rows: values must be *LabeledRowsValues, got %T", values)
	}
	const name = "labeled-rows"
	var errs []error

	if overrides != nil {
		ovr, ok := overrides.(*LabeledRowsOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("labeled-rows: overrides must be *LabeledRowsOverrides, got %T", overrides))
		} else {
			if err := ValidateCellAccentMode(name, ovr.CellAccentMode); err != nil {
				errs = append(errs, err)
			}
			switch ovr.LabelStyle {
			case "", labeledRowsStyleTab, labeledRowsStyleTinted, labeledRowsStyleFilled, labeledRowsStyleText:
			default:
				errs = append(errs, newValidationError(name, "overrides.label_style", ErrCodeUnknownEnum,
					fmt.Sprintf("labeled-rows: overrides.label_style must be one of %s, got %q", strings.Join(labeledRowsStyles, ", "), ovr.LabelStyle),
					UseOneOfFix("overrides.label_style", labeledRowsStyles)))
			}
			if ovr.LabelWidthPct != 0 && (ovr.LabelWidthPct < labeledRowsMinLabelPct || ovr.LabelWidthPct > labeledRowsMaxLabelPct) {
				errs = append(errs, errOutOfRange(name, "overrides.label_width_pct", int(labeledRowsMinLabelPct), int(labeledRowsMaxLabelPct), int(ovr.LabelWidthPct)))
			}
			if ovr.LabelSize != 0 && (ovr.LabelSize < 12 || ovr.LabelSize > 40) {
				errs = append(errs, errOutOfRange(name, "overrides.label_size", 12, 40, int(ovr.LabelSize)))
			}
			if ovr.BodySize != 0 && (ovr.BodySize < 12 || ovr.BodySize > 28) {
				errs = append(errs, errOutOfRange(name, "overrides.body_size", 12, 28, int(ovr.BodySize)))
			}
		}
	}

	if len(vals.Rows) < labeledRowsMinRows {
		errs = append(errs, errMinItems(name, "rows", labeledRowsMinRows, len(vals.Rows), "(hint: use pull-quote or stat-hero for a single message)"))
	}
	if len(vals.Rows) > labeledRowsMaxRows {
		errs = append(errs, errMaxItems(name, "rows", labeledRowsMaxRows, len(vals.Rows), "(hint: merge rows or split across two slides)"))
	}
	for i, r := range vals.Rows {
		for _, f := range []struct {
			field, text string
			max         int
			required    bool
		}{
			{"label", r.Label, labeledRowsLabelMax, true},
			{"sublabel", r.Sublabel, labeledRowsSublabelMax, false},
			{"body", r.Body, labeledRowsBodyMax, true},
		} {
			path := fmt.Sprintf("rows[%d].%s", i, f.field)
			switch {
			case f.required && strings.TrimSpace(f.text) == "":
				errs = append(errs, errRequired(name, path))
			case runeLen(f.text) > f.max:
				errs = append(errs, errMaxLength(name, path, f.max, runeLen(f.text)))
			}
		}
	}
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Rows), "(index = row)"); coErr != nil {
		errs = append(errs, coErr)
	}
	return errors.Join(errs...)
}

// labeledRowsLayout is the measured geometry at one type scale.
type labeledRowsLayout struct {
	cols                          []float64
	filled                        bool
	labelSize, subSize, bodySize  float64
	rowPt                         []float64
	unfitLabels                   []int // rows whose keyword breaks mid-word even at the floor
	labelTextW, bodyFrameW, areaH float64
	rowGapPt                      float64 // labeledRowsRowGapPt on the template grid
	labelColW                     float64 // label column width
}

func (l labeledRowsLayout) natural() float64 {
	n := len(l.rowPt)
	h := float64(n-1) * metricListRulePt
	for _, r := range l.rowPt {
		h += r
	}
	return h + float64(2*n-2)*l.rowGapPt
}

// labeledRowsScales is the default type scale, largest first: label, body,
// each a scale step (go-slide-creator-vmdfm).
var labeledRowsScales = [][2]float64{{scaleLeadPt, scaleSubheadPt}, {scaleLeadPt, scaleBodyPt}, {scaleSubheadPt, scaleBodyPt}}

func labeledRowsSubSize(body float64) float64 { return math.Max(12, body-2) }

func layoutLabeledRows(ctx ExpandContext, vals *LabeledRowsValues, ovr *LabeledRowsOverrides) labeledRowsLayout {
	labelPct := labeledRowsDefaultLabelPct
	if ovr.LabelWidthPct > 0 {
		labelPct = clampPt(ovr.LabelWidthPct, labeledRowsMinLabelPct, labeledRowsMaxLabelPct)
	}
	cols := []float64{labelPct, 100 - labelPct}
	areaW, areaH := sizingAreaPt(ctx)
	filled := ovr.LabelStyle != labeledRowsStyleText

	// Every row count starts at the top step: the 5–6 row lists that used
	// to skip a measuring step rendered at the same 18/14pt once settled
	// onto the scale (go-slide-creator-vmdfm).
	scales := labeledRowsScales
	if ovr.LabelSize > 0 || ovr.BodySize > 0 {
		scales = [][2]float64{{ResolveSize(ovr.LabelSize, scales[0][0]), ResolveSize(ovr.BodySize, scales[0][1])}}
	}
	var lay labeledRowsLayout
	for _, sc := range scales {
		lay = measureLabeledRows(ctx, vals, cols, filled, sc[0], sc[1], areaW)
		lay.areaH = areaH
		if lay.natural() <= areaH {
			break
		}
	}
	return lay
}

func measureLabeledRows(ctx ExpandContext, vals *LabeledRowsValues, cols []float64, filled bool, labelSize, bodySize, areaW float64) labeledRowsLayout {
	usableW := areaW - ctx.Gap(labeledRowsColGapPt)
	labelColW := usableW * cols[0] / 100
	bodyColW := usableW * cols[1] / 100
	lay := labeledRowsLayout{rowGapPt: ctx.Gap(labeledRowsRowGapPt), cols: cols, filled: filled, bodySize: bodySize, subSize: labeledRowsSubSize(bodySize), rowPt: make([]float64, len(vals.Rows))}

	// The keyword may wrap between words ("More / Resilient") but never inside
	// one: shrink one shared size until every word of every label fits.
	font := ctx.Theme.BodyFont
	inset := sizingInsetLRPt
	if filled {
		inset = labeledRowsBlockInset
	}
	// A tab costs the keyword no width: the pentagon's own text rectangle
	// gives up half the point and the tab's right inset gives it back
	// (labeledRowsTabInsetRight).
	lay.labelTextW = labelColW - 2*inset
	lay.labelColW = labelColW
	size := labelSize
	for _, r := range vals.Rows {
		for _, w := range strings.Fields(r.Label) {
			if s := fitSingleLineSize(w, font, true, size, labeledRowsMinLabelPt, lay.labelTextW); s < size {
				size = s
			}
		}
	}
	lay.labelSize = size
	for i, r := range vals.Rows {
		for _, w := range strings.Fields(r.Label) {
			if measuredLines(w, font, true, size, lay.labelTextW) > 1 {
				lay.unfitLabels = append(lay.unfitLabels, i)
				break
			}
		}
	}

	// sizedBlockHeightPt takes the frame width including the side insets; convert the real text width
	// back into the frame width it expects.
	labelFrameW := lay.labelTextW + 2*sizingInsetLRPt
	lay.bodyFrameW = bodyColW
	for i, r := range vals.Rows {
		paras := []sizedPara{{text: r.Label, sizePt: size, bold: true, spaceAfterPt: 2}}
		if strings.TrimSpace(r.Sublabel) != "" {
			paras = append(paras, sizedPara{text: r.Sublabel, sizePt: lay.subSize})
		}
		label := sizedBlockHeightPt(ctx, paras, labelFrameW)
		if filled {
			// The block's own inset replaces the default top/bottom inset.
			label += 2 * (labeledRowsBlockInset - sizingInsetTBPt)
		}
		body := sizedBlockHeightPt(ctx, []sizedPara{{text: r.Body, sizePt: bodySize}}, bodyColW)
		// Never below the written fit of the cells Expand writes: the
		// theme-font model alone can hand the writer a row it stores with an
		// autofit shrink (go-slide-creator-n1muf).
		written := math.Max(
			writtenFitHeightPt(ctx.themeFonts(), labeledRowsLabelText(r, lay, "lt1", "lt1"), labelColW, 0),
			writtenFitHeightPt(ctx.themeFonts(), labeledRowsBodyText(r, lay.bodySize), bodyColW, 0))
		lay.rowPt[i] = math.Ceil(math.Max(math.Max(label, body), written))
	}
	return lay
}

// labeledRowsLabelText is a row's label block text: the keyword, then the
// sublabel when there is one.
func labeledRowsLabelText(r LabeledRow, lay labeledRowsLayout, ink, subInk string) json.RawMessage {
	paras := []chartInsightsParagraph{{Content: r.Label, Size: lay.labelSize, Bold: true, Color: ink, Align: "l", SpaceAfter: 2}}
	if strings.TrimSpace(r.Sublabel) != "" {
		paras = append(paras, chartInsightsParagraph{Content: r.Sublabel, Size: lay.subSize, Color: subInk, Align: "l"})
	}
	return insetText{Paragraphs: paras, Align: "l", VerticalAlign: "ctr"}.json()
}

// labeledRowsBodyText is a row's body text.
func labeledRowsBodyText(r LabeledRow, bodySize float64) json.RawMessage {
	return insetText{
		Paragraphs: []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(r.Body), Size: bodySize, Color: "dk1", Align: "l"}},
		Align:      "l", VerticalAlign: "ctr",
	}.json()
}

func (l *labeledRows) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*LabeledRowsValues)
	if !ok || vals == nil {
		return nil, fmt.Errorf("labeled-rows: values must be *LabeledRowsValues, got %T", values)
	}
	ovr := &LabeledRowsOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*LabeledRowsOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("labeled-rows: overrides must be *LabeledRowsOverrides, got %T", overrides)
		}
	}

	baseAccent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := layoutLabeledRows(ctx, vals, ovr)
	style := labeledRowsStyle(ovr)
	subInkOnLight := inkOnLight(ctx, "dk2", 4.5)

	var rows []jsonschema.GridRowInput
	for i, r := range vals.Rows {
		if i > 0 {
			rows = append(rows, hairlineRuleRow(2))
		}
		accent := ctx.ResolveCellAccent(baseAccent, i, ovr.CellAccentMode)
		co, _ := cellOverrides[i].(*LabeledRowsCellOverride)

		var labelCell *jsonschema.GridCellInput
		if style == labeledRowsStyleTab {
			labelCell = labeledRowsTabCell(ctx, r, lay, accent, co)
		} else if lay.filled && style != labeledRowsStyleFilled {
			// Tinted by default: a column of solid accent label blocks was a
			// wall of colour (go-slide-creator-fl11f). The block keeps the
			// filled geometry and insets, so the measured layout is shared.
			minContrast := TextContrastThreshold(lay.labelSize, true)
			ink := accentInkOnTone(ctx, accent, neutralTone(NeutralTint4), minContrast)
			if co != nil && co.Color != "" {
				ink = co.Color
			}
			labelCell = &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "rect",
					Fill:     neutralFillJSON(NeutralTint4),
					Line:     json.RawMessage(`"none"`),
					Text:     labeledRowsLabelText(r, lay, ink, subInkOnLight),
				},
				AccentBar: &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: peerRuleWidthPt},
			}
		} else if lay.filled {
			// Ink by measured contrast at the block's smallest text: lt1 when
			// it clears WCAG AA there (3:1 for a large bold keyword alone),
			// else the theme's dark ink (go-slide-creator-8xsj3).
			minContrast := TextContrastThreshold(lay.labelSize, true)
			if strings.TrimSpace(r.Sublabel) != "" {
				minContrast = max(minContrast, TextContrastThreshold(lay.subSize, false))
			}
			fill := json.RawMessage(strconv.Quote(accent))
			var ink string
			if co != nil && co.Color != "" {
				ink = co.Color
			} else {
				var tone fillTone
				tone, ink = accentFillAndInk(ctx, fillTone{Color: accent}, minContrast)
				fill = tone.fillJSON()
			}
			labelCell = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     fill,
				Line:     json.RawMessage(`"none"`),
				Text:     labeledRowsLabelText(r, lay, ink, ink),
			}}
		} else {
			ink := inkOnLight(ctx, accent, 3.0)
			if co != nil && co.Color != "" {
				ink = co.Color
			}
			labelCell = &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"none"`),
				Line:     json.RawMessage(`"none"`),
				Text:     labeledRowsLabelText(r, lay, ink, subInkOnLight),
			}}
		}

		// The row's label block is the cell's primary text (D15 text keys).
		applyCellTextOverride(labelCell, co)

		bodyCell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"none"`),
			Line:     json.RawMessage(`"none"`),
			Text:     labeledRowsBodyText(r, lay.bodySize),
		}}
		if co != nil && co.AccentBar {
			bodyCell.AccentBar = &jsonschema.AccentBarInput{Position: "left", Color: accent, Width: 4}
		}
		rows = append(rows, jsonschema.GridRowInput{MinHeight: lay.rowPt[i], MaxHeight: lay.rowPt[i], Cells: []*jsonschema.GridCellInput{labelCell, bodyCell}})
	}

	colsJSON, _ := json.Marshal(lay.cols)
	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  ctx.Gap(labeledRowsColGapPt),
		RowGap:  ctx.Gap(labeledRowsRowGapPt),
		Rows:    rows,
	}
	fillCappedRows(ctx, grid.Rows, grid.RowGap, labeledRowsMinFillFrac, func(i int) bool { return i%2 == 0 })
	if style == labeledRowsStyleTab {
		labeledRowsPointTabs(grid.Rows, lay.labelColW)
	}
	return grid, nil
}

// labeledRowsTabCell is a row's label as a pale pentagon tab pointing into
// its row: the shape says "this labels that", where the tile under an accent
// rule was a web card beside a second system of row rules
// (go-slide-creator-mot7a). The fill is the accent's content swatch and the
// ink is measured on it.
func labeledRowsTabCell(ctx ExpandContext, r LabeledRow, lay labeledRowsLayout, accent string, co *LabeledRowsCellOverride) *jsonschema.GridCellInput {
	tone := tonalContent(ctx, accent)
	ink := tonalInk(ctx, tone)
	if co != nil && co.Color != "" {
		ink = co.Color
	}
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "homePlate",
		Fill:     tone.fillJSON(),
		Line:     noLine,
		Text:     withTextInsetSides(labeledRowsLabelText(r, lay, ink, ink), labeledRowsTabInsetRight, "inset_right"),
	}}
}

// labeledRowsPointTabs gives every tab (the first cell of each even row) one
// point depth, whatever height its row took.
func labeledRowsPointTabs(rows []jsonschema.GridRowInput, labelColW float64) {
	for i := 0; i < len(rows); i += 2 {
		if tab := rows[i].Cells[0]; tab != nil && tab.Shape != nil {
			tab.Shape.Adjustments = map[string]int64{"adj": swimlanePointAdj(labeledRowsTabPointPt, labelColW, rows[i].MaxHeight)}
		}
	}
}

// PostExpandWarnings reports a keyword that breaks mid-word even at the floor
// and a block too tall for the content area at the smallest type scale.
func (l *labeledRows) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*LabeledRowsValues)
	if !ok || v == nil || len(v.Rows) == 0 {
		return nil
	}
	ovr, _ := overrides.(*LabeledRowsOverrides)
	if ovr == nil {
		ovr = &LabeledRowsOverrides{}
	}
	lay := layoutLabeledRows(ctx, v, ovr)
	var out []string
	for _, i := range lay.unfitLabels {
		out = append(out, fmt.Sprintf(
			"%s: labeled-rows rows[%d].label %q has a word wider than the label column even at %.0fpt — the renderer breaks it mid-word; use a shorter keyword or raise overrides.label_width_pct",
			ErrCodeTextExceedsShape, i, v.Rows[i].Label, lay.labelSize))
	}
	if need := lay.natural(); need > lay.areaH+1 {
		out = append(out, fmt.Sprintf(
			"%s: labeled-rows rows[].body needs %.0fpt at the smallest type scale but the content area holds about %.0fpt — shorten the body text (4 rows hold about 211 characters per row, 5-6 rows about 101, without sublabels) or split the rows across two slides",
			ErrCodeBodyTooLong, need, lay.areaH))
	}
	return out
}
