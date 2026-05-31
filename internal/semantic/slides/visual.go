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

// CompileComparison compiles a comparison slide. With exactly two balanced
// columns (equal, non-empty item counts, within the comparison-2col 1–10 row
// range) it emits the comparison-2col pattern the planner advertises; otherwise
// it degrades to a content slide listing each column's items, so the deck still
// compiles.
func CompileComparison(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	headers, rows, ok := comparisonRows(in.Body)
	if !ok {
		return compileComparisonFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)

	encoded, err := json.Marshal(comparison2colValues{Headers: headers, Rows: rows})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal comparison-2col values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{Name: "comparison-2col", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.rows",
		SemanticPath: in.semSlide() + ".columns",
	})

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// comparisonRows builds the comparison-2col headers and rows from a comparison
// payload's two columns. It reports ok=false (degrade) when the payload does not
// have exactly two columns whose item lists are non-empty, equal in length, and
// within the pattern's 10-row cap.
func comparisonRows(body map[string]any) (headers []string, rows []comparison2colRow, ok bool) {
	cols := mapList(body, "columns")
	if len(cols) != 2 {
		return nil, nil, false
	}
	left := columnItems(cols[0])
	right := columnItems(cols[1])
	if len(left) == 0 || len(left) != len(right) || len(left) > 10 {
		return nil, nil, false
	}
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

// CompileProcess compiles a process slide. With 3–8 labelled steps it emits the
// process-flow pattern the planner advertises; otherwise it degrades to a
// content slide listing the steps, so the deck still compiles.
func CompileProcess(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	steps := processSteps(in.Body)
	if len(steps) < 3 || len(steps) > 8 {
		return compileStepBulletsFallback(in)
	}

	slide := &deckinput.SlideInput{SlideType: "content"}
	var links []SourceLink
	links = append(links, titleLink(slide, in)...)

	encoded, err := json.Marshal(processFlowValues{Steps: steps})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal process-flow values: %w", err)
	}
	slide.Pattern = &deckinput.PatternInput{Name: "process-flow", Values: encoded}
	links = append(links, SourceLink{
		RawPath:      in.rawSlide() + ".pattern.values.steps",
		SemanticPath: in.semSlide() + ".steps",
	})

	links = append(links, applyTakeaway(slide, in)...)
	return slide, links, nil
}

// processSteps extracts process-flow steps from a process payload's "steps"
// list, accepting string entries or {title|label|name, description, type}
// objects. Entries with no usable label are dropped.
func processSteps(body map[string]any) []processFlowStep {
	raw, ok := body["steps"].([]any)
	if !ok {
		return nil
	}
	var steps []processFlowStep
	for _, e := range raw {
		switch t := e.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" {
				steps = append(steps, processFlowStep{Label: s})
			}
		case map[string]any:
			label := firstNonEmpty(strField(t, "label"), strField(t, "title"), strField(t, "name"), strField(t, "step"), strField(t, "text"), strField(t, "description"))
			if label == "" {
				continue
			}
			step := processFlowStep{Label: label}
			if typ := strField(t, "type"); typ != "" {
				step.Type = typ
			}
			steps = append(steps, step)
		}
	}
	return steps
}

// compileStepBulletsFallback renders process steps as "Label — description"
// bullets on a content slide.
func compileStepBulletsFallback(in Input) (*deckinput.SlideInput, []SourceLink, error) {
	bullets, _ := stringList(in.Body, "steps")
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

	slide := &deckinput.SlideInput{SlideType: "content"}
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
				Description: firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary")),
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
				desc := firstNonEmpty(strField(t, "description"), strField(t, "detail"), strField(t, "summary"))
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
