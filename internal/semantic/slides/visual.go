package slides

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/deckinput"
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
}

// comparison2colValues mirrors patterns.Comparison2colValues for emission.
type comparison2colValues struct {
	Headers []string            `json:"headers,omitempty"`
	Rows    []comparison2colRow `json:"rows"`
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
type cardGridValues struct {
	Columns int            `json:"columns"`
	Rows    int            `json:"rows"`
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
//   - anything else → a content slide listing each column's items.
//
// Before go-slide-creator-3bgf only the first case had a visual: "compare us to
// three competitors", the most common QBR slide there is, collapsed into one
// run-on bullet per column — 'A | Parcel automation: EUR 60M capex; Payback 3.5
// yrs; Low risk' — on a page that was 60% empty, while the engine had the right
// patterns all along and only a hand-written raw pattern block could reach them.
func CompileComparison(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	if headers, rows, ok := comparisonRows(in.Body); ok {
		encoded, err := json.Marshal(comparison2colValues{Headers: headers, Rows: rows})
		if err != nil {
			return nil, nil, fmt.Errorf("marshal comparison-2col values: %w", err)
		}
		return comparisonPatternSlide(in, "comparison-2col", encoded, ".pattern.values.rows")
	}
	if panels, ok := comparisonPanels(in.Body); ok {
		encoded, err := json.Marshal(panels)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal stylish-panels values: %w", err)
		}
		return comparisonPatternSlide(in, "stylish-panels", encoded, ".pattern.values")
	}
	if cards, ok := comparisonCards(in.Body); ok {
		encoded, err := json.Marshal(cards)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal card-grid values: %w", err)
		}
		return comparisonPatternSlide(in, "card-grid", encoded, ".pattern.values.cells")
	}
	return compileComparisonFallback(in)
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
	// comparisonMaxCards is card-grid's column cap for a single row.
	comparisonMaxCards = 5
	// comparisonPanelMaxBullets / comparisonPanelBulletMax are stylish-panels'
	// per-panel bullet count and per-bullet length budgets.
	comparisonPanelMaxBullets = 8
	comparisonPanelBulletMax  = 200
	// comparisonHeaderMax is the shared title/header budget of both patterns.
	comparisonHeaderMax = 80
	// comparisonCardBodyMax is card-grid's per-card body budget.
	comparisonCardBodyMax = 300
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

// comparisonCards builds a one-row card-grid from 2–5 columns that each carry a
// header and at least one item, joining the items into the card body. It is the
// fallback visual for a comparison stylish-panels cannot hold — an unbalanced
// pair, or a column whose bullets are too many or too long for panels.
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
		if cardBody == "" || runeLen(cardBody) > comparisonCardBodyMax {
			return nil, false
		}
		cells = append(cells, cardGridCell{Header: header, Body: cardBody})
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
func CompileProcess(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	steps := ProcessStepDetails(in.Body)
	switch ProcessPattern(in.Body) {
	case "numbered-step-strip":
		return compileProcessStrip(in, steps)
	case "process-flow":
		return compileProcessFlow(in, steps)
	default:
		return compileStepBulletsFallback(in)
	}
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
func compileProcessFlow(in Input, steps []processStepDetail) (*deckinput.SlideInput, []SourceLink, error) {
	flow := make([]processFlowStep, 0, len(steps))
	for _, st := range steps {
		flow = append(flow, processFlowStep{Label: st.flowLabel(), Type: st.Type})
	}
	encoded, err := json.Marshal(processFlowValues{Steps: flow})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal process-flow values: %w", err)
	}
	return processPatternSlide(in, "process-flow", encoded)
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
			return fmt.Sprintf("step %d reads %d characters with its description; a flow box holds %d", i+1, runeLen(st.flowLabel()), processFlowLabelMax)
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
}

// CompileRoadmap compiles a roadmap slide. With 3–6 named phases it emits the
// phase-roadmap pattern the planner advertises; otherwise it degrades to a
// content slide listing the phases, so the deck still compiles.
func CompileRoadmap(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	phases := roadmapPhases(in.Body)
	if len(phases) < 3 || len(phases) > 6 {
		return compilePhaseBulletsFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content", LayoutID: "blank-title"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)

	encoded, err := json.Marshal(phaseRoadmapValues{Phases: phases})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal phase-roadmap values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{Name: "phase-roadmap", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.phases",
		SemanticPath: in.semSlide() + ".phases",
	})

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
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
				Description: foldPhaseItems(firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary")), phaseItems(t)),
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

// foldPhaseItems folds a phase's sub-bullet items into its single description
// cell so per-phase detail is not dropped. Items are joined with " · " and,
// when a description is already present, appended after it.
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
				if strings.TrimSpace(line) != "" {
					bullets = append(bullets, line)
				}
			}
		}
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
