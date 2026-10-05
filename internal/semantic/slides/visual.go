package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// This file holds the per-kind compilers for the first-class visual kinds —
// comparison, process, and roadmap — that the planner (internal/semantic.ir)
// advertises a named pattern for. Each emits the advertised pattern when the
// payload fits the pattern's shape, and degrades to a safe content slide with
// readable bullets (never a Go "map[...]" dump) when it does not. The semantic
// validation layer emits a SEMANTIC_DENSITY advisory for the count mismatches
// that cause this degradation, so a degraded slide is never silent.

// ---------------------------------------------------------------------------
// comparison -> comparison-2col
// ---------------------------------------------------------------------------

// comparison2colRow mirrors patterns.Comparison2colRow for emission (the slides
// package depends only on deckinput, never on the renderer's patterns package).
type comparison2colRow struct {
	Left  string `json:"left"`
	Right string `json:"right"`
	// Highlight tints this row: the one the comparison turns on
	// (go-slide-creator-ptazs).
	Highlight bool `json:"highlight,omitempty"`
}

// comparison2colValues mirrors patterns.Comparison2colValues for emission.
type comparison2colValues struct {
	Headers []string            `json:"headers,omitempty"`
	Rows    []comparison2colRow `json:"rows"`
}

// comparison2colOverrides mirrors the patterns.Comparison2colOverrides the
// comparison kind exposes: per-row connector badges for a "from → to" shift,
// and one emphasised column (go-slide-creator-ptazs). The transformation deck's
// "today vs target, aligned row by row" slide needed both and could reach
// neither without raw_json2pptx.
type comparison2colOverrides struct {
	Connectors      bool   `json:"connectors,omitempty"`
	HighlightColumn string `json:"highlight_column,omitempty"`
}

// ComparisonConnectors reports whether the author asked for the per-row
// connector badges between the columns.
func ComparisonConnectors(body map[string]any) bool {
	b, _ := body["connectors"].(bool)
	return b
}

// ComparisonHighlightColumn resolves highlight_column to the pattern's "left"
// or "right": the side by name, or a column header matched case-insensitively.
// ok is false when the field is set and names neither; side is "" when it is
// unset.
func ComparisonHighlightColumn(body map[string]any) (side string, ok bool) {
	ref := strings.TrimSpace(strField(body, "highlight_column"))
	if ref == "" {
		return "", true
	}
	switch strings.ToLower(ref) {
	case "left":
		return "left", true
	case "right":
		return "right", true
	}
	cols := mapList(body, "columns")
	if len(cols) == 2 {
		for i, col := range cols {
			if strings.EqualFold(strings.TrimSpace(columnHeader(col)), ref) {
				return []string{"left", "right"}[i], true
			}
		}
	}
	return "", false
}

// ComparisonHighlightRow resolves highlight_row to a 0-based row index: the
// index itself, or the text of a cell in either column matched
// case-insensitively. ok is false when the field is set and matches no row;
// idx is -1 when it is unset.
func ComparisonHighlightRow(body map[string]any) (idx int, ok bool) {
	raw, present := body["highlight_row"]
	if !present || raw == nil {
		return -1, true
	}
	_, rows, feasible := comparisonRows(body)
	if n, isNumber := numberField(body, "highlight_row"); isNumber {
		i := int(n)
		if !feasible || float64(i) != n || i < 0 || i >= len(rows) {
			return -1, false
		}
		return i, true
	}
	ref := strings.TrimSpace(strField(body, "highlight_row"))
	if ref == "" || !feasible {
		return -1, false
	}
	for i, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Left), ref) || strings.EqualFold(strings.TrimSpace(row.Right), ref) {
			return i, true
		}
	}
	return -1, false
}

// comparisonOverrides builds the comparison-2col overrides the payload asks
// for, or nil when it asks for none that resolve.
func comparisonOverrides(body map[string]any) *comparison2colOverrides {
	ovr := comparison2colOverrides{Connectors: ComparisonConnectors(body)}
	if side, ok := ComparisonHighlightColumn(body); ok {
		ovr.HighlightColumn = side
	}
	if !ovr.Connectors && ovr.HighlightColumn == "" {
		return nil
	}
	return &ovr
}

// stylishPanelsItem mirrors patterns.StylishPanelsItem for emission: a panel
// title over its own bullet list.
type stylishPanelsItem struct {
	Title string   `json:"title"`
	Body  []string `json:"body"`
}

// cardGridCell mirrors patterns.CardGridCell for emission.
type cardGridCell struct {
	Header string `json:"header"`
	Body   string `json:"body"`
}

// cardGridValues mirrors patterns.CardGridValues for emission.
// Columns and Rows are omitted past one row of cards: the pattern arranges the
// cards from their count and leaves a last row that is not full short.
type cardGridValues struct {
	Columns int            `json:"columns,omitempty"`
	Rows    int            `json:"rows,omitempty"`
	Cells   []cardGridCell `json:"cells"`
}

// CompileComparison compiles a comparison slide to the best visual its columns
// support:
//
//   - exactly two balanced columns (equal, non-empty item counts, within the
//     comparison-2col 1–10 row range) → comparison-2col;
//   - 3–5 columns → stylish-panels, a titled panel with its own bullet list per
//     column;
//   - 2–5 columns stylish-panels cannot hold (an unbalanced pair, an over-long
//     bullet) → card-grid, one titled card per column;
//   - 6–12 columns → card-grid, the cards arranged from their count (7 as
//     4 + 3: the pattern leaves a last row that is not full short);
//   - anything else → a content slide listing each column's items.
//
// Before go-slide-creator-3bgf only the first case had a visual: "compare us to
// three competitors", the most common QBR slide there is, collapsed into one
// run-on bullet per column — 'A | Parcel automation: EUR 60M capex; Payback 3.5
// yrs; Low risk' — on a page that was 60% empty, while the engine had the right
// patterns all along and only a hand-written raw pattern block could reach them.
func CompileComparison(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	switch {
	case in.wantsContent():
		return compileComparisonFallback(in)
	case in.Override.Pattern == "stylish-panels" && comparisonPanelsFeasible(in.Body):
		return compileComparisonPanels(in)
	case in.Override.Pattern == "card-grid" && comparisonCardsFeasible(in.Body):
		return compileComparisonCards(in)
	}
	if headers, rows, ok := comparisonRows(in.Body); ok {
		ovr := comparisonOverrides(in.Body)
		// The pattern draws a row highlight or a column highlight, never
		// both; validation refuses the pair, and the column wins here.
		if idx, ok := ComparisonHighlightRow(in.Body); ok && idx >= 0 && (ovr == nil || ovr.HighlightColumn == "") {
			rows[idx].Highlight = true
		}
		encoded, err := json.Marshal(comparison2colValues{Headers: headers, Rows: rows})
		if err != nil {
			return nil, nil, fmt.Errorf("marshal comparison-2col values: %w", err)
		}
		slide, links, err := comparisonPatternSlide(in, "comparison-2col", encoded, ".pattern.values.rows")
		if err != nil {
			return nil, nil, err
		}
		if ovr != nil {
			overrides, oErr := json.Marshal(ovr)
			if oErr != nil {
				return nil, nil, fmt.Errorf("marshal comparison-2col overrides: %w", oErr)
			}
			slide.Pattern.Overrides = overrides
		}
		return slide, links, nil
	}
	if comparisonPanelsFeasible(in.Body) {
		return compileComparisonPanels(in)
	}
	if comparisonCardsFeasible(in.Body) {
		return compileComparisonCards(in)
	}
	return compileComparisonFallback(in)
}

// compileComparisonPanels emits one stylish panel per column.
func compileComparisonPanels(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	panels, ok := comparisonPanels(in.Body)
	if !ok {
		return compileComparisonFallback(in)
	}
	encoded, err := json.Marshal(panels)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal stylish-panels values: %w", err)
	}
	return comparisonPatternSlide(in, "stylish-panels", encoded, ".pattern.values")
}

// compileComparisonCards emits one titled card per column.
func compileComparisonCards(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	cards, ok := comparisonCards(in.Body)
	if !ok {
		return compileComparisonFallback(in)
	}
	encoded, err := json.Marshal(cards)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal card-grid values: %w", err)
	}
	return comparisonPatternSlide(in, "card-grid", encoded, ".pattern.values.cells")
}

// comparisonPatternSlide assembles the slide around a resolved comparison
// pattern: the title placeholder, the pattern, one source link from the
// pattern's values back to the authored columns, and the takeaway.
func comparisonPatternSlide(in Input, pattern string, values []byte, valuesPath string) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: pattern, Values: values}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + valuesPath,
		SemanticPath: in.semSlide() + ".columns",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// ComparisonPattern names the pattern a comparison payload will compile to, or
// "" when it degrades to a content slide. Validation and the explain planner
// consult it so neither advertises a treatment that disagrees with compile.
func ComparisonPattern(body map[string]any) string {
	switch {
	case ComparisonPatternFeasible(body):
		return "comparison-2col"
	case comparisonPanelsFeasible(body):
		return "stylish-panels"
	case comparisonCardsFeasible(body):
		return "card-grid"
	}
	return ""
}

// Pattern bounds the multi-column comparison compilers work within. They mirror
// the patterns' own Validate rules, so a payload that passes here renders.
const (
	// comparisonMaxPanels is stylish-panels' panel cap; comparisonMinPanels its
	// floor (below it the pattern tells the caller to use card-grid).
	comparisonMinPanels = 3
	comparisonMaxPanels = 5
	// comparisonMaxRowCards is card-grid's column cap for a single row;
	// comparisonMaxCards the most cards it arranges over several rows.
	comparisonMaxRowCards = 5
	comparisonMaxCards    = 12
	// comparisonPanelMaxBullets / comparisonPanelBulletMax are stylish-panels'
	// per-panel bullet count and per-bullet length budgets.
	comparisonPanelMaxBullets = 8
	comparisonPanelBulletMax  = 200
	// comparisonHeaderMax is the shared title/header budget of both patterns.
	comparisonHeaderMax = 80
	// comparisonCardBodyMax is card-grid's per-card body budget;
	// comparisonGridCardBodyMax the budget once the cards take several rows.
	comparisonCardBodyMax     = 300
	comparisonGridCardBodyMax = 160
	// comparisonCardItemJoin separates a column's items inside one card body.
	// card-grid renders the body as a single wrapped paragraph, so the items
	// need a visible separator rather than a newline the run builder would drop.
	comparisonCardItemJoin = " · "
)

// comparisonPanels builds stylish-panels values from 3–5 columns that each
// carry a header and at least one item. It reports ok=false when the shape or
// any budget does not fit, so the caller can try the next visual.
func comparisonPanels(body map[string]any) ([]stylishPanelsItem, bool) {
	cols := mapList(body, "columns")
	if len(cols) < comparisonMinPanels || len(cols) > comparisonMaxPanels {
		return nil, false
	}
	panels := make([]stylishPanelsItem, 0, len(cols))
	for _, col := range cols {
		header := columnHeader(col)
		items := columnItems(col)
		if header == "" || runeLen(header) > comparisonHeaderMax || len(items) == 0 || len(items) > comparisonPanelMaxBullets {
			return nil, false
		}
		for _, item := range items {
			if item == "" || runeLen(item) > comparisonPanelBulletMax {
				return nil, false
			}
		}
		panels = append(panels, stylishPanelsItem{Title: header, Body: items})
	}
	return panels, true
}

// comparisonPanelsFeasible reports whether comparisonPanels would succeed.
func comparisonPanelsFeasible(body map[string]any) bool {
	_, ok := comparisonPanels(body)
	return ok
}

// comparisonCards builds a card-grid from 2–12 columns that each carry a header
// and at least one item, joining the items into the card body: one row for 2–5
// columns, and the pattern's own arrangement (7 as 4 + 3) for 6–12. It is the
// fallback visual for a comparison stylish-panels cannot hold — an unbalanced
// pair, a column whose bullets are too many or too long for panels, or more
// columns than panels take.
func comparisonCards(body map[string]any) (*cardGridValues, bool) {
	cols := mapList(body, "columns")
	if len(cols) < 2 || len(cols) > comparisonMaxCards {
		return nil, false
	}
	cells := make([]cardGridCell, 0, len(cols))
	for _, col := range cols {
		header := columnHeader(col)
		items := columnItems(col)
		if header == "" || runeLen(header) > comparisonHeaderMax || len(items) == 0 {
			return nil, false
		}
		cardBody := strings.Join(items, comparisonCardItemJoin)
		bodyMax := comparisonCardBodyMax
		if len(cols) > comparisonMaxRowCards {
			bodyMax = comparisonGridCardBodyMax
		}
		if cardBody == "" || runeLen(cardBody) > bodyMax {
			return nil, false
		}
		cells = append(cells, cardGridCell{Header: header, Body: cardBody})
	}
	if len(cells) > comparisonMaxRowCards {
		return &cardGridValues{Cells: cells}, true
	}
	return &cardGridValues{Columns: len(cells), Rows: 1, Cells: cells}, true
}

// comparisonCardsFeasible reports whether comparisonCards would succeed.
func comparisonCardsFeasible(body map[string]any) bool {
	_, ok := comparisonCards(body)
	return ok
}

// ComparisonMaxRows is the per-column row cap of the comparison-2col pattern.
// Beyond it the compiler degrades to a bullet content slide, so validation and
// the explain planner share this bound to stay in step with compile.
const ComparisonMaxRows = 10

// ComparisonPatternFeasible reports whether a comparison payload will compile to
// the comparison-2col pattern (rather than degrading to a content fallback): it
// must have exactly two columns whose item lists are non-empty, equal in length,
// and within ComparisonMaxRows. Validation and the explain planner consult this
// so neither flags nor advertises a treatment that disagrees with compile.
func ComparisonPatternFeasible(body map[string]any) bool {
	cols := mapList(body, "columns")
	if len(cols) != 2 {
		return false
	}
	left := columnItems(cols[0])
	right := columnItems(cols[1])
	return len(left) > 0 && len(left) == len(right) && len(left) <= ComparisonMaxRows
}

// comparisonRows builds the comparison-2col headers and rows from a comparison
// payload's two columns. It reports ok=false (degrade) when the payload does not
// have exactly two columns whose item lists are non-empty, equal in length, and
// within the pattern's ComparisonMaxRows cap.
func comparisonRows(body map[string]any) (headers []string, rows []comparison2colRow, ok bool) {
	if !ComparisonPatternFeasible(body) {
		return nil, nil, false
	}
	cols := mapList(body, "columns")
	left := columnItems(cols[0])
	right := columnItems(cols[1])
	rows = make([]comparison2colRow, len(left))
	for i := range left {
		rows[i] = comparison2colRow{Left: left[i], Right: right[i]}
	}
	hl, hr := columnHeader(cols[0]), columnHeader(cols[1])
	if hl != "" || hr != "" {
		headers = []string{hl, hr}
	}
	return headers, rows, true
}

// columnHeader extracts a comparison column's header label.
func columnHeader(col map[string]any) string {
	return firstNonEmpty(strField(col, "header"), strField(col, "title"), strField(col, "label"), strField(col, "name"))
}

// columnItems returns a comparison column's row items. It prefers an explicit
// "items" list; when that is absent it synthesises items from "pros"/"cons"
// arrays so pro/con-shaped columns are not silently dropped (field-test 2.2).
// Each of pros and cons collapses to a single labelled line ("Pros: a; b"),
// which keeps the two columns balanced for the comparison-2col visual and
// preserves the distinction in the readable fallback.
func columnItems(col map[string]any) []string {
	if items, _ := stringList(col, "items"); len(items) > 0 {
		return items
	}
	var out []string
	if pros, _ := stringList(col, "pros"); len(pros) > 0 {
		out = append(out, "Pros: "+strings.Join(pros, "; "))
	}
	if cons, _ := stringList(col, "cons"); len(cons) > 0 {
		out = append(out, "Cons: "+strings.Join(cons, "; "))
	}
	return out
}

// compileComparisonFallback renders the comparison as a content slide, one
// bullet per column ("Header: item; item; …"), so unbalanced or many-column
// comparisons still produce readable, valid output.
func compileComparisonFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	var bullets []string
	for _, col := range mapList(in.Body, "columns") {
		items := columnItems(col)
		line := strings.Join(items, "; ")
		if h := columnHeader(col); h != "" {
			if line != "" {
				line = h + ": " + line
			} else {
				line = h
			}
		}
		if strings.TrimSpace(line) != "" {
			bullets = append(bullets, line)
		}
	}
	return contentFallback(in, "columns", bullets)
}

// ---------------------------------------------------------------------------
// process -> process-flow
// ---------------------------------------------------------------------------

// processFlowStep mirrors patterns.ProcessFlowStep for emission.
type processFlowStep struct {
	Label string `json:"label"`
	Type  string `json:"type,omitempty"`
}

// processFlowValues mirrors patterns.ProcessFlowValues for emission.
type processFlowValues struct {
	Steps []processFlowStep `json:"steps"`
}

// CompileProcess compiles a process slide. Steps that carry a description take
// the numbered step strip, which has a place to put one; bare labels and
// branching processes take process-flow. Outside both it degrades to a content
// slide listing the steps, so the deck still compiles.
//
// A pattern / layout override the steps fit wins over that choice. It used to
// be reported by explain and then ignored here, so layout:content and
// pattern:process-flow both still rendered the strip (go-slide-creator-vj549).
func CompileProcess(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	if in.wantsContent() {
		return compileStepBulletsFallback(in)
	}
	steps := ProcessStepDetails(in.Body)
	pattern := ProcessPattern(in.Body)
	if in.Override.Pattern != "" && ProcessCompositionProblem(in.Body, in.Override.Pattern) == "" {
		pattern = in.Override.Pattern
	}
	switch pattern {
	case "numbered-step-strip":
		return compileProcessStrip(in, steps)
	case "process-flow", processFlowSparsePattern:
		return compileProcessFlow(in, steps, pattern)
	default:
		return compileStepBulletsFallback(in)
	}
}

// ProcessCompositionProblem says why the steps cannot take the named process
// pattern, or "" when they can. Validation reports it against an override the
// compiler has to decline.
func ProcessCompositionProblem(body map[string]any, pattern string) string {
	steps := ProcessStepDetails(body)
	switch pattern {
	case "numbered-step-strip":
		if processBranches(steps) {
			return "a step has a type (a decision or other branch shape) the numbered strip cannot draw"
		}
		if len(steps) < processStripMin || len(steps) > processStripMax {
			return fmt.Sprintf("it has %d usable steps; the numbered strip holds %d–%d", len(steps), processStripMin, processStripMax)
		}
		for i, st := range steps {
			if n := runeLen(st.Label); n > processStripLabelMax {
				return fmt.Sprintf("step %d's label reads %d characters; a strip label holds %d", i+1, n, processStripLabelMax)
			}
			if n := runeLen(st.Description); n > processStripBodyMax {
				return fmt.Sprintf("step %d's description reads %d characters; a strip detail line holds %d", i+1, n, processStripBodyMax)
			}
		}
	case "process-flow", processFlowSparsePattern:
		if len(steps) < processFlowMin || len(steps) > processFlowMax {
			return fmt.Sprintf("it has %d usable steps; the flow holds %d–%d", len(steps), processFlowMin, processFlowMax)
		}
		for i, st := range steps {
			// process-flow has no detail zone: the description rides in the
			// box after the label, so the pair is what has to fit.
			if n := runeLen(st.flowLabel()); n > processFlowLabelMax {
				return fmt.Sprintf("step %d reads %d characters with its description; a flow box holds %d (layout content keeps every word as native bullets)", i+1, n, processFlowLabelMax)
			}
		}
		if pattern == processFlowSparsePattern && !processFlowSparse(flowSteps(steps)) {
			return fmt.Sprintf("the compact band holds %d–%d steps averaging under %d characters", processFlowMin, processFlowSparseMaxSteps, processFlowSparseMaxAvgChars)
		}
	}
	return ""
}

// compileProcessStrip emits the numbered rows, each a bold label over its own
// detail line. The label and the description used to be concatenated into one
// string and centred in a process-flow box at ~9pt reversed out of solid accent
// (go-slide-creator-61up).
func compileProcessStrip(in Input, steps []processStepDetail) (*deckinput.SlideInput, []SourceLink, error) {
	strip := make([]numberedStep, 0, len(steps))
	for _, st := range steps {
		strip = append(strip, numberedStep{Label: st.Label, Body: st.Description})
	}
	// Four stacked boxes read as a list; five or six as a run, which is what the
	// chevron style is for.
	style := "stacked-box"
	if len(strip) >= 5 {
		style = "chevron"
	}
	encoded, err := json.Marshal(numberedStepStripValues{Style: style, Steps: strip})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal numbered-step-strip values: %w", err)
	}
	return processPatternSlide(in, "numbered-step-strip", encoded)
}

// compileProcessFlow emits the flow boxes. process-flow has no detail zone, so
// a step that carries a description keeps it appended to the label rather than
// losing it — that only happens on the branching path, where the diamonds are
// the reason to be here at all.
func compileProcessFlow(in Input, steps []processStepDetail, pattern string) (*deckinput.SlideInput, []SourceLink, error) {
	flow := flowSteps(steps)
	encoded, err := json.Marshal(processFlowValues{Steps: flow})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal process-flow values: %w", err)
	}
	// A short run of bare labels takes the compact flow: a shallow,
	// content-sized band top-anchored under the title. Capping process-flow at
	// half the content height stretched its rows over that whole band — four
	// ~200px boxes around one word each above an empty lower half
	// (go-slide-creator-xb06p) — and the uncapped flow is the
	// SPARSE_SINGLE_ROW_FLOW smell. ProcessPattern makes that call; an explicit
	// process-flow override keeps the full flow.
	return processPatternSlide(in, pattern, encoded)
}

const (
	// processFlowSparse* mirror the render-side SPARSE_SINGLE_ROW_FLOW guard:
	// 3–6 steps averaging under 40 characters would stretch into oversized
	// boxes, so the compiler emits processFlowSparsePattern instead.
	processFlowSparseMaxSteps    = 6
	processFlowSparseMaxAvgChars = 40
	processFlowSparsePattern     = "process-flow-compact"
)

// processFlowSparse reports whether a flow is a short run of short labels.
func processFlowSparse(flow []processFlowStep) bool {
	if len(flow) < processFlowMin || len(flow) > processFlowSparseMaxSteps {
		return false
	}
	total := 0
	for _, st := range flow {
		total += len(strings.TrimSpace(st.Label))
	}
	return total < processFlowSparseMaxAvgChars*len(flow)
}

// processPatternSlide assembles a process pattern slide.
func processPatternSlide(in Input, pattern string, values json.RawMessage) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	links := titleLink(slide, in)
	slide.Pattern = &deckinput.PatternInput{Name: pattern, Values: values}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.steps",
		SemanticPath: in.semSlide() + ".steps",
	})
	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// numberedStep is one numbered-step-strip step.
type numberedStep struct {
	Label string `json:"label"`
	Body  string `json:"body,omitempty"`
}

// numberedStepStripValues is the numbered-step-strip pattern's values object.
type numberedStepStripValues struct {
	Style string         `json:"style,omitempty"`
	Steps []numberedStep `json:"steps"`
}

// processStepDetail is one resolved step: what it is, what it means, and
// whether it branches.
type processStepDetail struct {
	Label       string
	Description string
	Type        string
}

// flowLabel renders the step for a pattern with nowhere to put a description.
func (s processStepDetail) flowLabel() string {
	if s.Description == "" || s.Description == s.Label {
		return s.Label
	}
	return s.Label + " — " + s.Description
}

const (
	// processStripMin / processStripMax mirror numbered-step-strip's bounds,
	// and processStripLabelMax / processStripBodyMax its text budgets.
	processStripMin      = 3
	processStripMax      = 6
	processStripLabelMax = 60
	processStripBodyMax  = 180
	processFlowMin       = 3
	processFlowMax       = 8
	processFlowLabelMax  = 80
)

// ProcessStepDetails resolves a process payload's steps WITHOUT flattening a
// step's description into its label.
func ProcessStepDetails(body map[string]any) []processStepDetail {
	raw, ok := body["steps"].([]any)
	if !ok {
		return nil
	}
	var steps []processStepDetail
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				steps = append(steps, processStepDetail{Label: s})
			}
		case map[string]any:
			label := firstNonEmpty(strField(t, "label"), strField(t, "title"), strField(t, "name"), strField(t, "step"), strField(t, "text"), strField(t, "description"))
			if label == "" {
				continue
			}
			description := firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary"))
			if description == label {
				description = ""
			}
			steps = append(steps, processStepDetail{Label: label, Description: description, Type: strField(t, "type")})
		}
	}
	return steps
}

// processBranches reports whether any step is a decision or another shape only
// the flow diagram draws.
func processBranches(steps []processStepDetail) bool {
	for _, st := range steps {
		if st.Type != "" {
			return true
		}
	}
	return false
}

// processDescribed reports whether any step says more than its own name.
func processDescribed(steps []processStepDetail) bool {
	for _, st := range steps {
		if st.Description != "" {
			return true
		}
	}
	return false
}

// processStripFits reports whether the steps fit the numbered strip.
func processStripFits(steps []processStepDetail) bool {
	if len(steps) < processStripMin || len(steps) > processStripMax {
		return false
	}
	for _, st := range steps {
		if runeLen(st.Label) > processStripLabelMax || runeLen(st.Description) > processStripBodyMax {
			return false
		}
	}
	return true
}

// processFlowFits reports whether the steps fit the flow diagram.
func processFlowFits(steps []processStepDetail) bool {
	if len(steps) < processFlowMin || len(steps) > processFlowMax {
		return false
	}
	for _, st := range steps {
		if runeLen(st.flowLabel()) > processFlowLabelMax {
			return false
		}
	}
	return true
}

// ProcessPattern returns the pattern a process payload compiles to, or "" when
// it degrades to bullets. A branching process belongs to the flow diagram
// whatever else it carries — the diamonds are why it is there. Otherwise a step
// that says more than its own name wants the strip, which has a place to put
// it (go-slide-creator-61up).
func ProcessPattern(body map[string]any) string {
	steps := ProcessStepDetails(body)
	pattern := processPatternFor(steps)
	if pattern == "process-flow" && processFlowSparse(flowSteps(steps)) {
		// A short run of short labels takes the compact flow
		// (go-slide-creator-xb06p).
		return processFlowSparsePattern
	}
	return pattern
}

// flowSteps is the steps as compileProcessFlow writes them.
func flowSteps(steps []processStepDetail) []processFlowStep {
	flow := make([]processFlowStep, 0, len(steps))
	for _, st := range steps {
		flow = append(flow, processFlowStep{Label: st.flowLabel(), Type: st.Type})
	}
	return flow
}

// processPatternFor picks between the flow diagram, the numbered strip and
// the bullet fallback ("").
func processPatternFor(steps []processStepDetail) string {
	switch {
	case processBranches(steps) && processFlowFits(steps):
		return "process-flow"
	case processBranches(steps):
		return ""
	case processDescribed(steps) && processStripFits(steps):
		return "numbered-step-strip"
	case !processDescribed(steps) && processFlowFits(steps):
		return "process-flow"
	case processFlowFits(steps):
		// Described, but too many steps for the strip: the flow diagram still
		// carries every word, appended to the label.
		return "process-flow"
	default:
		return ""
	}
}

// ProcessOverBudget explains why the steps cannot take a visual, or "" when
// they can (or when there are none).
func ProcessOverBudget(body map[string]any) string {
	steps := ProcessStepDetails(body)
	if len(steps) == 0 || ProcessPattern(body) != "" {
		return ""
	}
	// The count is of steps the compiler can render — blank entries are already
	// dropped — so it says "usable" rather than echoing the raw list length.
	if len(steps) < processFlowMin {
		return fmt.Sprintf("has %d usable steps; a process needs at least %d", len(steps), processFlowMin)
	}
	if len(steps) > processFlowMax {
		return fmt.Sprintf("has %d usable steps; the flow holds %d", len(steps), processFlowMax)
	}
	for i, st := range steps {
		if runeLen(st.flowLabel()) > processFlowLabelMax {
			reason := fmt.Sprintf("step %d reads %d characters with its description; a flow box holds %d", i+1, runeLen(st.flowLabel()), processFlowLabelMax)
			// Steps with descriptions were written for the numbered rows: say
			// what a row holds too, so the author is not sent to cut every step
			// to a flow box (go-slide-creator-iubjb).
			if processDescribed(steps) && len(steps) <= processStripMax {
				reason += fmt.Sprintf(", and a numbered row holds a label of %d and a description of %d", processStripLabelMax, processStripBodyMax)
			}
			return reason
		}
	}
	return ""
}

// compileStepBulletsFallback renders process steps as "Label — description"
// bullets on a content slide.
func compileStepBulletsFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	steps := ProcessStepDetails(in.Body)
	bullets := make([]string, len(steps))
	for i := range steps {
		bullets[i] = steps[i].flowLabel()
	}
	return contentFallback(in, "steps", bullets)
}

// ---------------------------------------------------------------------------
// roadmap -> phase-roadmap
// ---------------------------------------------------------------------------

// phaseRoadmapPhase mirrors patterns.PhaseRoadmapPhase for emission.
type phaseRoadmapPhase struct {
	Name        string `json:"name"`
	DateLabel   string `json:"date_label,omitempty"`
	Description string `json:"description,omitempty"`
	Active      bool   `json:"active,omitempty"`
	Milestone   string `json:"milestone,omitempty"`
}

// phaseRoadmapValues mirrors patterns.PhaseRoadmapValues for emission.
type phaseRoadmapValues struct {
	Phases []phaseRoadmapPhase `json:"phases"`
	// ParallelTracks are the workstreams that run alongside every phase, drawn
	// as full-width bars under the phases and labelled ParallelLabel
	// ("In parallel" by default) (go-slide-creator-ptazs).
	ParallelTracks []string `json:"parallel_tracks,omitempty"`
	ParallelLabel  string   `json:"parallel_label,omitempty"`
}

// RoadmapMaxTracks is the most parallel tracks phase-roadmap draws.
const RoadmapMaxTracks = patterns.PhaseRoadmapMaxTracks

const (
	// roadmapMinPhases / roadmapMaxPhases are phase-roadmap's phase range.
	roadmapMinPhases = 3
	roadmapMaxPhases = 6
	// roadmapTrackKeys are the keys the parallel tracks are read by: the
	// pattern's own name first, then the word an author reaches for.
	roadmapTracksField      = "parallel_tracks"
	roadmapTracksAliasField = "workstreams"
	roadmapParallelLabel    = "In parallel"
)

// CompileRoadmap compiles a roadmap slide. With 3–6 named phases within the
// pattern's text budgets it emits the phase-roadmap pattern the planner
// advertises — with the parallel tracks when there are any; otherwise it
// degrades to a content slide listing the phases (and the tracks), so the deck
// still compiles.
func CompileRoadmap(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	phases := roadmapPhases(in.Body)
	if in.wantsContent() || len(phases) < roadmapMinPhases || len(phases) > roadmapMaxPhases || RoadmapOverBudget(in.Body) != "" {
		return compilePhaseBulletsFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)

	values := phaseRoadmapValues{Phases: phases, ParallelTracks: RoadmapParallelTracks(in.Body), ParallelLabel: strField(in.Body, "parallel_label")}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal phase-roadmap values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{Name: "phase-roadmap", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.phases",
		SemanticPath: in.semSlide() + ".phases",
	})
	if len(values.ParallelTracks) > 0 {
		links = append(links, SourceLink{
			RawPath:      in.rawSlide() + ".pattern.values.parallel_tracks",
			SemanticPath: in.semSlide() + "." + roadmapTracksKey(in.Body),
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// roadmapTracksKey is the key the tracks were authored under.
func roadmapTracksKey(body map[string]any) string {
	if _, ok := body[roadmapTracksField].([]any); ok {
		return roadmapTracksField
	}
	if _, ok := body[roadmapTracksAliasField].([]any); ok {
		return roadmapTracksAliasField
	}
	return roadmapTracksField
}

// RoadmapParallelTracks reads the workstreams that run alongside every phase:
// strings, or {label|name|title} objects; blank entries are dropped.
func RoadmapParallelTracks(body map[string]any) []string {
	raw, ok := body[roadmapTracksKey(body)].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				out = append(out, s)
			}
		case map[string]any:
			if s := firstNonEmpty(strField(t, "label"), strField(t, "name"), strField(t, "title")); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// RoadmapBudgetItems lists every roadmap field too long for phase-roadmap:
// a phase's name, date label, description (with its items, which render
// inside it), milestone, a parallel track, or the tracks' label. Each is
// reported at its own path with the pattern's limit, so validation points
// at the field to shorten rather than at the slide.
func RoadmapBudgetItems(body map[string]any) []BudgetItem {
	var out []BudgetItem
	raw, _ := body["phases"].([]any)
	n := 0
	for i, e := range raw {
		if s, isString := e.(string); isString {
			// A string entry is the phase's name alone.
			if s = strings.TrimSpace(s); s == "" {
				continue
			}
			n++
			if l := runeLen(s); l > patterns.PhaseRoadmapNameMax {
				out = append(out, BudgetItem{Field: fmt.Sprintf("phases[%d]", i), What: fmt.Sprintf("phase %d's name", n),
					Measured: l, Allowed: patterns.PhaseRoadmapNameMax, Holds: fmt.Sprintf("a phase name holds %d", patterns.PhaseRoadmapNameMax)})
			}
			continue
		}
		t, ok := e.(map[string]any)
		if !ok {
			continue
		}
		nameKey := authoredKey(t, "name", "title", "label", "phase")
		if nameKey == "" {
			continue
		}
		n++
		check := func(key, what string, measured, allowed int, holds string) {
			if measured > allowed {
				out = append(out, BudgetItem{Field: fmt.Sprintf("phases[%d].%s", i, key), What: fmt.Sprintf("phase %d's %s", n, what),
					Measured: measured, Allowed: allowed, Holds: holds})
			}
		}
		check(nameKey, "name", runeLen(strField(t, nameKey)), patterns.PhaseRoadmapNameMax, fmt.Sprintf("a phase name holds %d", patterns.PhaseRoadmapNameMax))
		if dk := authoredKey(t, "date_label", "dates", "date", "period"); dk != "" {
			check(dk, "date label", runeLen(strField(t, dk)), patterns.PhaseRoadmapDateLabelMax, fmt.Sprintf("a date label holds %d", patterns.PhaseRoadmapDateLabelMax))
		}
		descKey := authoredKey(t, "description", "detail", "summary")
		composed := phaseItemBullets(strField(t, descKey), phaseItems(t))
		if descKey == "" {
			descKey = "items"
			if _, ok := t["items"]; !ok {
				descKey = "bullets"
			}
		}
		check(descKey, "description with its items", runeLen(composed), patterns.PhaseRoadmapDescriptionMax, fmt.Sprintf("a phase holds %d", patterns.PhaseRoadmapDescriptionMax))
		check("milestone", "milestone", runeLen(strField(t, "milestone")), patterns.PhaseRoadmapMilestoneMax, fmt.Sprintf("a milestone holds %d", patterns.PhaseRoadmapMilestoneMax))
	}
	tracksKey := roadmapTracksKey(body)
	for i, track := range RoadmapParallelTracks(body) {
		if l := runeLen(track); l > patterns.PhaseRoadmapTrackMax {
			out = append(out, BudgetItem{Field: fmt.Sprintf("%s[%d]", tracksKey, i), What: fmt.Sprintf("parallel track %d", i+1),
				Measured: l, Allowed: patterns.PhaseRoadmapTrackMax, Holds: fmt.Sprintf("a track bar holds %d", patterns.PhaseRoadmapTrackMax)})
		}
	}
	if l := runeLen(strField(body, "parallel_label")); l > patterns.PhaseRoadmapLabelMax {
		out = append(out, BudgetItem{Field: "parallel_label", What: "the parallel tracks' label",
			Measured: l, Allowed: patterns.PhaseRoadmapLabelMax, Holds: fmt.Sprintf("the label holds %d", patterns.PhaseRoadmapLabelMax)})
	}
	return out
}

// RoadmapTrackCountProblem says why the parallel tracks cannot be drawn by
// their count, or "" when they fit (0–4).
func RoadmapTrackCountProblem(body map[string]any) string {
	if n := len(RoadmapParallelTracks(body)); n > patterns.PhaseRoadmapMaxTracks {
		return fmt.Sprintf("has %d parallel tracks; the roadmap draws %d (merge related workstreams)", n, patterns.PhaseRoadmapMaxTracks)
	}
	return ""
}

// RoadmapOverBudget explains why the phases cannot take phase-roadmap for a
// reason other than their count, or "" when they can.
func RoadmapOverBudget(body map[string]any) string {
	if p := RoadmapTrackCountProblem(body); p != "" {
		return p
	}
	if items := RoadmapBudgetItems(body); len(items) > 0 {
		return items[0].Message()
	}
	return ""
}

// roadmapPhases extracts phase-roadmap phases from a roadmap payload's "phases"
// list, accepting string entries (name only) or
// {name|title|label, date_label|dates|date|period, description|detail, active,
// milestone} objects. Entries with no usable name are dropped.
func roadmapPhases(body map[string]any) []phaseRoadmapPhase {
	raw, ok := body["phases"].([]any)
	if !ok {
		return nil
	}
	var phases []phaseRoadmapPhase
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				phases = append(phases, phaseRoadmapPhase{Name: s})
			}
		case map[string]any:
			name := firstNonEmpty(strField(t, "name"), strField(t, "title"), strField(t, "label"), strField(t, "phase"))
			if name == "" {
				continue
			}
			phase := phaseRoadmapPhase{
				Name:        name,
				DateLabel:   firstNonEmpty(strField(t, "date_label"), strField(t, "dates"), strField(t, "date"), strField(t, "period")),
				Description: phaseItemBullets(firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary")), phaseItems(t)),
				Milestone:   strField(t, "milestone"),
			}
			if active, ok := t["active"].(bool); ok {
				phase.Active = active
			}
			phases = append(phases, phase)
		}
	}
	return phases
}

// phaseItems extracts a phase object's "items" (or "bullets") sub-bullet list,
// keeping only non-empty string entries.
func phaseItems(t map[string]any) []string {
	raw, ok := t["items"].([]any)
	if !ok {
		raw, ok = t["bullets"].([]any)
		if !ok {
			return nil
		}
	}
	var out []string
	for _, e := range raw {
		if s, ok := e.(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// phaseItemBullets builds a phase-roadmap description cell: the description as
// a lead-in line, then one "- " line per item, which the pattern renders as
// native bullets. Joined with " · " the items read as one run-on paragraph
// (go-slide-creator-n83ml).
func phaseItemBullets(desc string, items []string) string {
	if len(items) == 0 {
		return desc
	}
	lines := make([]string, 0, len(items)+1)
	if d := strings.TrimSpace(desc); d != "" {
		lines = append(lines, d)
	}
	for _, it := range items {
		lines = append(lines, "- "+it)
	}
	return strings.Join(lines, "\n")
}

// foldPhaseItems folds a phase's sub-bullet items into one line for the
// content-slide fallback, where each phase is already a single bullet. Items
// are joined with " · " and, when a description is present, appended after it.
func foldPhaseItems(desc string, items []string) string {
	if len(items) == 0 {
		return desc
	}
	joined := strings.Join(items, " · ")
	if strings.TrimSpace(desc) == "" {
		return joined
	}
	return desc + " · " + joined
}

// compilePhaseBulletsFallback renders roadmap phases as readable bullets on a
// content slide.
func compilePhaseBulletsFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	var bullets []string
	if raw, ok := in.Body["phases"].([]any); ok {
		for _, e := range raw {
			switch t := e.(type) {
			case string:
				if s := strings.TrimSpace(t); s != "" {
					bullets = append(bullets, s)
				}
			case map[string]any:
				name := firstNonEmpty(strField(t, "name"), strField(t, "title"), strField(t, "label"), strField(t, "phase"))
				date := firstNonEmpty(strField(t, "date_label"), strField(t, "dates"), strField(t, "date"), strField(t, "period"))
				desc := foldPhaseItems(firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary")), phaseItems(t))
				line := name
				if date != "" {
					line = strings.TrimSpace(line + " (" + date + ")")
				}
				if desc != "" {
					if line != "" {
						line += " — " + desc
					} else {
						line = desc
					}
				}
				if milestone := strField(t, "milestone"); milestone != "" {
					line = strings.TrimSpace(line + " (milestone: " + milestone + ")")
				}
				if strings.TrimSpace(line) != "" {
					bullets = append(bullets, line)
				}
			}
		}
	}
	// The parallel workstreams ride along as one closing bullet: the fallback
	// keeps every word the visual would have drawn.
	if tracks := RoadmapParallelTracks(in.Body); len(tracks) > 0 {
		label := firstNonEmpty(strField(in.Body, "parallel_label"), roadmapParallelLabel)
		bullets = append(bullets, label+": "+strings.Join(tracks, " · "))
	}
	return contentFallback(in, "phases", bullets)
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

// titleLink appends the slide's title content (when present) and returns its
// source link, factored out of the per-kind compilers.
func titleLink(slide *deckinput.SlideInput, in Input) []SourceLink {
	if in.Title == "" {
		return nil
	}
	idx := appendContent(slide, textContent("title", in.Title))
	return []SourceLink{{
		RawPath:      fmt.Sprintf("%s.content[%d].text_value", in.rawSlide(), idx),
		SemanticPath: in.semSlide() + ".title",
	}}
}

// contentFallback builds the safe content-slide degradation shared by the
// visual-kind fallbacks: a title, the supplied readable bullets bound to the
// given semantic field, and the takeaway.
func contentFallback(in Input, sourceField string, bullets []string) (*deckinput.SlideInput, []SourceLink, error) {
	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)

	if len(bullets) > 0 {
		idx := appendContent(slide, bulletsContent("body", bullets))
		links = append(links, SourceLink{
			RawPath:      fmt.Sprintf("%s.content[%d].bullets_value", in.rawSlide(), idx),
			SemanticPath: in.semSlide() + "." + sourceField,
		})
	}

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}
