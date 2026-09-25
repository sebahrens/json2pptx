package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/svggen"
)

// ---------------------------------------------------------------------------
// text-sidebar pattern — narrative introduction page: a main text column
// (optional heading, 1-4 paragraphs, 0-6 bullets) beside a sidebar panel that
// states the one key message in large bold type.
// ---------------------------------------------------------------------------
//
// Layout (sidebar_side = right):
//
//   Introduction                               ┌───────────────────┐
//   Paragraph one …                            │                   │
//   Paragraph two …                            │  In this paper we │
//   • bullet                                   │  show how …       │
//   • bullet                                   │                   │
//   Closing paragraph …                        └───────────────────┘
//
// The main column and the sidebar are two top-level cells, so both stay
// visible to the readability preflight. The row is content-sized in points
// (at least 60% of the content height, so the panel reads as a panel) and the
// pattern vertical_align default centres it. The sidebar's text colour is
// measured against its own fill — a tinted surface or the solid accent.

func init() {
	Default().Register(&textSidebar{})
}

type textSidebar struct{}

// Pattern budgets.
const (
	tsHeadingMax      = 80
	tsParagraphMax    = 450
	tsMinParagraphs   = 1
	tsMaxParagraphs   = 4
	tsBulletMax       = 150
	tsMaxBullets      = 6
	tsSidebarMax      = 200
	tsDefaultSidePct  = 32.0
	tsMinSidePct      = 25.0
	tsMaxSidePct      = 40.0
	tsColGapPt        = 28.0
	tsMinFillPct      = 60.0
	tsSidebarInsetPt  = 18.0
	tsSidebarBarPt    = 4.0
	tsParaSpacePt     = 8.0
	tsBulletSpacePt   = 4.0
	tsHeadingSpacePt  = 10.0
	tsDefaultBodySize = 14.0
	tsSparseBodySize  = 16.0
)

func (t *textSidebar) Name() string { return "text-sidebar" }
func (t *textSidebar) Description() string {
	return "Narrative intro page: main text column (optional heading, 1-4 paragraphs, 0-6 bullets) beside a tinted or accent-filled sidebar panel holding one large bold key message"
}
func (t *textSidebar) UseWhen() string {
	return "Introduction / foreword / context page written as prose — 1–4 paragraphs (plus up to 6 bullets) with ONE key message pulled out into a sidebar panel; prefer pull-quote when the callout is an attributed quotation, image-text-split when a photo sits beside the text, exec-summary when the content is 3–5 parallel conclusions"
}
func (t *textSidebar) NotWhen() string {
	return "The callout is someone's attributed quote (use pull-quote), the visual is a photo or screenshot (use image-text-split), the content is 3–5 bold conclusions with evidence (use exec-summary), or the text is a bullet list with no narrative (use card-grid or a plain content slide)"
}
func (t *textSidebar) Version() int      { return 1 }
func (t *textSidebar) CellsHint() string { return "1 text column + 1 sidebar" }
func (t *textSidebar) Taxonomy() PatternTaxonomy {
	return PatternTaxonomy{
		Category:      "narrative",
		NarrativeRole: []string{"open", "frame"},
		PairsWith:     []string{"exec-summary", "agenda", "kpi-3up"},
		DensityClass:  "medium",
		AccentWeight:  "normal",
	}
}

func (t *textSidebar) SupportsInlineMarkdown() bool { return true }

func (t *textSidebar) ExemplarValues() any {
	return &TextSidebarValues{
		Heading: "Introduction",
		Paragraphs: []string{
			"We meet often with CEOs, CFOs and CIOs to discuss AI — a topic that is both captivating and evolving rapidly. After working with more than 2,500 clients over the past three years, we are **sharing our most recent insights** in a new series for the C-suite.",
			"In this edition we discuss the future of asset management and the role agentic AI will play in separating those who successfully implement it from those who do not. We address the questions executives ask most:",
		},
		Bullets: []string{
			"How do recent advances in agentic AI affect our strategy?",
			"We have run AI pilots for two years and the ROI is underwhelming — what are the best doing differently?",
			"What does an end-to-end AI-first operating model look like?",
		},
		Sidebar: "In this perspective, we outline how AI-first asset managers unlock end-to-end transformation and sustained value creation",
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// TextSidebarValues holds the main column and the sidebar message.
type TextSidebarValues struct {
	Heading    string   `json:"heading,omitempty"`
	Paragraphs []string `json:"paragraphs"`
	Bullets    []string `json:"bullets,omitempty"`
	Sidebar    string   `json:"sidebar"`
}

// TextSidebarOverrides are the pattern-level overrides.
type TextSidebarOverrides struct {
	Accent          string  `json:"accent,omitempty"`
	SemanticAccent  string  `json:"semantic_accent,omitempty"`
	SidebarSide     string  `json:"sidebar_side,omitempty"`      // right (default) | left
	SidebarWidthPct float64 `json:"sidebar_width_pct,omitempty"` // 25-40, default 32
	BodySize        float64 `json:"body_size,omitempty"`         // default 14
	SidebarSize     float64 `json:"sidebar_size,omitempty"`      // default 24, stepped down to fit
	SidebarStyle    string  `json:"sidebar_style,omitempty"`     // tinted (default) | filled
}

func (t *textSidebar) NewValues() any       { return &TextSidebarValues{} }
func (t *textSidebar) NewOverrides() any    { return &TextSidebarOverrides{} }
func (t *textSidebar) NewCellOverride() any { return nil }

func (t *textSidebar) Schema() *Schema {
	valuesSchema := ObjectSchema(map[string]*Schema{
		"heading":    StringSchema(tsHeadingMax).WithDescription("Optional bold heading at the top of the main column, e.g. \"Introduction\""),
		"paragraphs": ArraySchema(StringSchema(tsParagraphMax), tsMinParagraphs, tsMaxParagraphs).WithDescription("1-4 prose paragraphs (≤450 chars each; **bold** emphasis allowed). The column holds roughly 1,100 characters at 14pt and 1,500 at 12pt; longer copy reports BODY_TOO_LONG"),
		"bullets":    ArraySchema(StringSchema(tsBulletMax), 0, tsMaxBullets).WithDescription("0-6 bullets (≤150 chars each), placed after the first paragraph that ends with a colon, else after the last paragraph"),
		"sidebar":    StringSchema(tsSidebarMax).WithDescription("The one key message, set large and bold in the sidebar panel (≤200 chars)"),
	}, []string{"paragraphs", "sidebar"}).WithAdditionalProperties(false)

	overridesSchema := ObjectSchema(map[string]*Schema{
		"accent":            StringSchema(0).WithDescription("Accent scheme color for the sidebar panel (default accent1)").WithDefault("accent1"),
		"semantic_accent":   EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
		"sidebar_side":      EnumSchema("right", "left").WithDescription("Which side the sidebar sits on (default right)").WithDefault("right"),
		"sidebar_width_pct": NumberSchema(tsMinSidePct, tsMaxSidePct).WithDescription("Sidebar width as a percentage of the grid (default 32)").WithDefault(tsDefaultSidePct),
		"body_size":         NumberSchema(12, 20).WithDescription("Main-column body / bullet font size in points (default 14 — 16 for short copy, stepped down to 12 for long copy; the heading is 6pt larger)"),
		"sidebar_size":      NumberSchema(14, 40).WithDescription("Sidebar message font size in points (default 24, stepped down until the message fits)"),
		"sidebar_style":     EnumSchema("tinted", "filled").WithDescription("tinted (default): pale accent surface with an accent top bar and dark text; filled: solid accent panel with measured light text").WithDefault("tinted"),
	}, nil).WithAdditionalProperties(false)

	return ObjectSchema(map[string]*Schema{
		"values":    valuesSchema,
		"overrides": overridesSchema,
	}, []string{"values"}).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Narrative intro page: prose column beside a sidebar panel with one large key message")
}

func (t *textSidebar) Validate(values, overrides any, cellOverrides map[int]any) error {
	v, ok := values.(*TextSidebarValues)
	if !ok || v == nil {
		return fmt.Errorf("text-sidebar: values must be *TextSidebarValues, got %T", values)
	}
	const name = "text-sidebar"
	var errs []error
	if overrides != nil {
		ovr, ok := overrides.(*TextSidebarOverrides)
		if !ok {
			errs = append(errs, fmt.Errorf("text-sidebar: overrides must be *TextSidebarOverrides, got %T", overrides))
		} else {
			errs = append(errs, tsValidateOverrides(ovr)...)
		}
	}
	if runeLen(v.Heading) > tsHeadingMax {
		errs = append(errs, errMaxLength(name, "heading", tsHeadingMax, runeLen(v.Heading)))
	}
	if len(v.Paragraphs) < tsMinParagraphs {
		errs = append(errs, errMinItems(name, "paragraphs", tsMinParagraphs, len(v.Paragraphs), "(hint: the main column needs at least one paragraph of prose)"))
	}
	if len(v.Paragraphs) > tsMaxParagraphs {
		errs = append(errs, errMaxItems(name, "paragraphs", tsMaxParagraphs, len(v.Paragraphs), "(hint: merge paragraphs or continue on a second slide)"))
	}
	for i, p := range v.Paragraphs {
		path := fmt.Sprintf("paragraphs[%d]", i)
		if strings.TrimSpace(p) == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(p) > tsParagraphMax {
			errs = append(errs, errMaxLength(name, path, tsParagraphMax, runeLen(p)))
		}
	}
	if len(v.Bullets) > tsMaxBullets {
		errs = append(errs, errMaxItems(name, "bullets", tsMaxBullets, len(v.Bullets), "(hint: keep the strongest six or move detail to a card-grid slide)"))
	}
	for i, b := range v.Bullets {
		path := fmt.Sprintf("bullets[%d]", i)
		if strings.TrimSpace(b) == "" {
			errs = append(errs, errRequired(name, path))
		} else if runeLen(b) > tsBulletMax {
			errs = append(errs, errMaxLength(name, path, tsBulletMax, runeLen(b)))
		}
	}
	if strings.TrimSpace(v.Sidebar) == "" {
		errs = append(errs, errRequired(name, "sidebar"))
	} else if runeLen(v.Sidebar) > tsSidebarMax {
		errs = append(errs, errMaxLength(name, "sidebar", tsSidebarMax, runeLen(v.Sidebar)))
	}
	if len(cellOverrides) > 0 {
		errs = append(errs, newValidationError(name, "cell_overrides", ErrCodeUnknownKey,
			"text-sidebar: cell_overrides are not supported (use overrides)", RemoveFieldFix("cell_overrides")))
	}
	return errors.Join(errs...)
}

func tsValidateOverrides(ovr *TextSidebarOverrides) []error {
	const name = "text-sidebar"
	var errs []error
	if ovr.SidebarSide != "" && ovr.SidebarSide != "left" && ovr.SidebarSide != "right" {
		errs = append(errs, newValidationError(name, "overrides.sidebar_side", ErrCodeUnknownEnum,
			fmt.Sprintf("text-sidebar: overrides.sidebar_side must be right or left, got %q", ovr.SidebarSide), UseOneOfFix("overrides.sidebar_side", []string{"right", "left"})))
	}
	if ovr.SidebarStyle != "" && ovr.SidebarStyle != "tinted" && ovr.SidebarStyle != "filled" {
		errs = append(errs, newValidationError(name, "overrides.sidebar_style", ErrCodeUnknownEnum,
			fmt.Sprintf("text-sidebar: overrides.sidebar_style must be tinted or filled, got %q", ovr.SidebarStyle), UseOneOfFix("overrides.sidebar_style", []string{"tinted", "filled"})))
	}
	if ovr.SidebarWidthPct != 0 && (ovr.SidebarWidthPct < tsMinSidePct || ovr.SidebarWidthPct > tsMaxSidePct) {
		errs = append(errs, errOutOfRange(name, "overrides.sidebar_width_pct", int(tsMinSidePct), int(tsMaxSidePct), int(ovr.SidebarWidthPct)))
	}
	if ovr.BodySize != 0 && (ovr.BodySize < 12 || ovr.BodySize > 20) {
		errs = append(errs, errOutOfRange(name, "overrides.body_size", 12, 20, int(ovr.BodySize)))
	}
	if ovr.SidebarSize != 0 && (ovr.SidebarSize < 14 || ovr.SidebarSize > 40) {
		errs = append(errs, errOutOfRange(name, "overrides.sidebar_size", 14, 40, int(ovr.SidebarSize)))
	}
	return errs
}

// tsLayout is the measured geometry of one expansion.
type tsLayout struct {
	sidePct, mainPct float64
	mainW, sideW     float64
	headingSize      float64
	bodySize         float64
	sidebarSize      float64
	mainPt           float64 // measured main column height
	sidePt           float64 // measured sidebar height
	heightPt         float64 // row height
	mainFits         bool
	sideFits         bool
}

// tsBulletAnchor is the index of the paragraph the bullets follow: the first
// paragraph that introduces a list (ends in a colon), else the last one.
func tsBulletAnchor(paragraphs []string) int {
	for i, p := range paragraphs {
		if strings.HasSuffix(strings.TrimSpace(p), ":") {
			return i
		}
	}
	return len(paragraphs) - 1
}

// tsMainParas returns the measured paragraphs of the main column, in order:
// heading, paragraphs, with the bullets after the paragraph that introduces
// them (see tsBulletAnchor).
func tsMainParas(v *TextSidebarValues, headingSize, bodySize float64) []sizedPara {
	var paras []sizedPara
	if strings.TrimSpace(v.Heading) != "" {
		paras = append(paras, sizedPara{text: v.Heading, sizePt: headingSize, bold: true, spaceAfterPt: tsHeadingSpacePt})
	}
	anchor := tsBulletAnchor(v.Paragraphs)
	for i, p := range v.Paragraphs {
		paras = append(paras, sizedPara{text: p, sizePt: bodySize, spaceAfterPt: tsParaSpacePt})
		if i == anchor {
			for j, b := range v.Bullets {
				space := tsBulletSpacePt
				if j == len(v.Bullets)-1 {
					space = tsParaSpacePt
				}
				paras = append(paras, sizedPara{text: "• " + b, sizePt: bodySize, spaceAfterPt: space})
			}
		}
	}
	return paras
}

func tsSidebarHeightPt(ctx ExpandContext, text string, size, widthPt float64) float64 {
	// paragraphLines removes the default 7.2pt insets; the panel uses wider ones.
	w := widthPt - 2*tsSidebarInsetPt + 2*sizingInsetLRPt
	lines := paragraphLines(ctx, sizedPara{text: text, sizePt: size, bold: true}, w)
	return float64(lines)*size*sizingLineSpacing + 2*tsSidebarInsetPt + tsSidebarBarPt + sizingSafetyPt
}

func tsMeasure(ctx ExpandContext, v *TextSidebarValues, ovr *TextSidebarOverrides) tsLayout {
	areaW, areaH := sizingAreaPt(ctx)
	lay := tsLayout{sidePct: clampPct(ovr.SidebarWidthPct, tsDefaultSidePct, tsMinSidePct, tsMaxSidePct)}
	lay.mainPct = 100 - lay.sidePct
	lay.mainW = (areaW - tsColGapPt) * lay.mainPct / 100
	lay.sideW = (areaW - tsColGapPt) * lay.sidePct / 100

	// Short copy is promoted to 16pt (it must still leave the column
	// visibly un-crammed); longer copy steps down from 14pt to the floor.
	type bodyStep struct{ size, limitFrac float64 }
	bodySteps := []bodyStep{{tsSparseBodySize, 0.7}, {tsDefaultBodySize, 1}, {13, 1}, {12, 1}}
	if ovr.BodySize > 0 {
		bodySteps = []bodyStep{{ovr.BodySize, 1}}
	}
	for _, st := range bodySteps {
		lay.bodySize, lay.headingSize = st.size, st.size+6
		lay.mainPt = sizedBlockHeightPt(ctx, tsMainParas(v, lay.headingSize, lay.bodySize), lay.mainW)
		if lay.mainPt <= areaH*st.limitFrac {
			break
		}
	}
	lay.mainFits = lay.mainPt <= areaH
	lay.heightPt = math.Min(math.Max(lay.mainPt, areaH*tsMinFillPct/100), areaH)

	sideSteps := []float64{28, 26, 24, 22, 20, 18, 16}
	if ovr.SidebarSize > 0 {
		sideSteps = []float64{ovr.SidebarSize}
	}
	// The sidebar should be a dominant statement, not a caption: take the
	// largest step that fits the row AND still leaves the panel some air.
	for _, s := range sideSteps {
		lay.sidebarSize = s
		lay.sidePt = tsSidebarHeightPt(ctx, v.Sidebar, s, lay.sideW)
		if lay.sidePt <= lay.heightPt*0.85 {
			break
		}
	}
	lay.sideFits = lay.sidePt <= lay.heightPt
	if !lay.sideFits && lay.sidePt <= areaH {
		lay.heightPt = lay.sidePt
		lay.sideFits = true
	}
	lay.heightPt = math.Round(lay.heightPt*10) / 10
	return lay
}

// PostExpandWarnings reports a main column or sidebar message that does not
// fit even at the smallest type step.
func (t *textSidebar) PostExpandWarnings(ctx ExpandContext, values, overrides any) []string {
	v, ok := values.(*TextSidebarValues)
	if !ok || v == nil {
		return nil
	}
	ovr, _ := overrides.(*TextSidebarOverrides)
	if ovr == nil {
		ovr = &TextSidebarOverrides{}
	}
	lay := tsMeasure(ctx, v, ovr)
	var warnings []string
	if !lay.mainFits {
		chars := runeLen(v.Heading)
		for _, p := range v.Paragraphs {
			chars += runeLen(p)
		}
		for _, b := range v.Bullets {
			chars += runeLen(b)
		}
		_, areaH := sizingAreaPt(ctx)
		budget := int(float64(chars) * areaH / lay.mainPt)
		warnings = append(warnings, fmt.Sprintf(
			"%s: text-sidebar paragraphs and bullets hold %d characters; the main column holds about %d at %.0fpt before text shrinks below the readable minimum — cut the copy or continue on a second slide",
			ErrCodeBodyTooLong, chars, budget, lay.bodySize))
	}
	if !lay.sideFits {
		warnings = append(warnings, fmt.Sprintf(
			"%s: text-sidebar sidebar is %d characters and does not fit the panel at %.0fpt — shorten the key message or widen overrides.sidebar_width_pct",
			ErrCodeBodyTooLong, runeLen(v.Sidebar), lay.sidebarSize))
	}
	return warnings
}

func (t *textSidebar) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	v, ok := values.(*TextSidebarValues)
	if !ok {
		return nil, fmt.Errorf("text-sidebar: values must be *TextSidebarValues, got %T", values)
	}
	ovr := &TextSidebarOverrides{}
	if overrides != nil {
		if ovr, ok = overrides.(*TextSidebarOverrides); !ok {
			return nil, fmt.Errorf("text-sidebar: overrides must be *TextSidebarOverrides, got %T", overrides)
		}
	}
	accent := ctx.ResolveAccent(ovr.Accent, ovr.SemanticAccent)
	lay := tsMeasure(ctx, v, ovr)

	mainCell := tsMainCell(ctx, v, lay)
	sideCell := tsSidebarCell(ctx, v.Sidebar, lay, accent, ovr.SidebarStyle)
	cells := []*jsonschema.GridCellInput{mainCell, sideCell}
	cols := []float64{lay.mainPct, lay.sidePct}
	if ovr.SidebarSide == "left" {
		cells = []*jsonschema.GridCellInput{sideCell, mainCell}
		cols = []float64{lay.sidePct, lay.mainPct}
	}
	colsJSON, _ := json.Marshal(cols)
	return &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(colsJSON),
		ColGap:  tsColGapPt,
		Rows:    []jsonschema.GridRowInput{{MinHeight: lay.heightPt, MaxHeight: lay.heightPt, Cells: cells}},
	}, nil
}

func tsMainCell(ctx ExpandContext, v *TextSidebarValues, lay tsLayout) *jsonschema.GridCellInput {
	headingInk := inkOnLight(ctx, "dk2", 4.5)
	var paras []chartInsightsParagraph
	hasHeading := strings.TrimSpace(v.Heading) != ""
	for i, sp := range tsMainParas(v, lay.headingSize, lay.bodySize) {
		para := chartInsightsParagraph{Content: pptx.ConvertMarkdownEmphasis(sp.text), Size: sp.sizePt, Color: "dk1", Align: "l", SpaceAfter: sp.spaceAfterPt}
		if i == 0 && hasHeading {
			para.Bold = true
			para.Color = headingInk
		}
		paras = append(paras, para)
	}
	if n := len(paras); n > 0 {
		paras[n-1].SpaceAfter = 0
	}
	// Copy that fills the column starts level with the panel's top; short
	// copy is centred against the panel instead of hanging from its top edge.
	vAlign := "t"
	if lay.mainPt < lay.heightPt*0.8 {
		vAlign = "ctr"
	}
	textJSON, _ := json.Marshal(chartInsightsText{Paragraphs: paras, Align: "l", VerticalAlign: vAlign})
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect", Fill: json.RawMessage(`"none"`), Text: textJSON}}
}

// tsSidebarCell builds the key-message panel. tinted: a pale accent surface
// with an accent bar on top; filled: the solid accent. Either way the text
// colour is measured against the fill actually painted.
func tsSidebarCell(ctx ExpandContext, message string, lay tsLayout, accent, style string) *jsonschema.GridCellInput {
	tone := paleAccentTone(accent)
	fallback := "dk1"
	if style == "filled" {
		tone = fillTone{Color: accent}
		fallback = "lt1"
	}
	// The message is large bold type, so the 3:1 large-text bar applies —
	// the same bar the generator's contrast pass holds it to.
	ink := readableInkOn(ctx, tone, fallback, svggen.WCAGAALarge)
	text := struct {
		Paragraphs    []chartInsightsParagraph `json:"paragraphs"`
		Align         string                   `json:"align"`
		VerticalAlign string                   `json:"vertical_align"`
		InsetLeft     float64                  `json:"inset_left"`
		InsetRight    float64                  `json:"inset_right"`
		InsetTop      float64                  `json:"inset_top"`
		InsetBottom   float64                  `json:"inset_bottom"`
	}{
		Paragraphs:    []chartInsightsParagraph{{Content: pptx.ConvertMarkdownEmphasis(message), Size: lay.sidebarSize, Bold: true, Color: ink, Align: "l"}},
		Align:         "l",
		VerticalAlign: "ctr",
		InsetLeft:     tsSidebarInsetPt,
		InsetRight:    tsSidebarInsetPt,
		InsetTop:      tsSidebarInsetPt,
		InsetBottom:   tsSidebarInsetPt,
	}
	textJSON, _ := json.Marshal(text)
	cell := &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     tone.fillJSON(),
		Line:     json.RawMessage(`"none"`),
		Text:     textJSON,
	}}
	if style != "filled" {
		cell.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: accent, Width: tsSidebarBarPt}
	}
	return cell
}
