package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// dataSourcePatterns are the patterns whose content is numbers an audience
// will ask "according to whom?" about: charts, bridges, ranked bars and the
// KPI / stat families. Qualitative data_visual patterns (matrix-2x2,
// table-highlight's Harvey balls, capability-heatmap ratings) are left out —
// a source line under a judgement call is noise (go-slide-creator-cuszt). A
// table-highlight of figures is caught by patternDataKind instead.
var dataSourcePatterns = map[string]bool{
	"chart-insights-split":         true,
	"waterfall-bridge":             true,
	"horizontal-bar-with-callouts": true,
	"kpi-2up":                      true,
	"kpi-3up":                      true,
	"kpi-4up":                      true,
	"kpi-5up":                      true,
	"kpi-6up":                      true,
	"kpi-inline":                   true,
	"stat-hero":                    true,
	"hero-detail":                  true,
	"metric-list":                  true,
}

// collectDataWithoutSourceFindings flags content slides that show data — a
// chart, a chart-shaped diagram, a table with numbers in it, or a chart / KPI /
// stat pattern — and cite no source (DATA_WITHOUT_SOURCE,
// go-slide-creator-cuszt). A board deck with unsourced numbers gets sent back.
//
// It reads the authored slide, before pattern or compose expansion, and counts
// a chart-insights-split / stat-hero values.source as a source, since
// generation lifts it into the slide's source zone, and a chart_value footnote,
// which renders at the chart's bottom. Review-weighted: an unsourced data slide
// is the first thing a senior reviewer sends back, and at info it cost nothing
// and never reached the gate (go-slide-creator-mp2p4).
func collectDataWithoutSourceFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var out []patterns.FitFinding
	for si := range input.Slides {
		slide := input.Slides[si]
		if !slideQualifiesForDuplicateTitleCheck(slide) || slideCitesSource(slide) {
			continue
		}
		what := slideDataKind(slide)
		if what == "" {
			continue
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    slidepath.SlideField(si, "source"),
				Code:    patterns.ErrCodeDataWithoutSource,
				Message: fmt.Sprintf("slide %d: shows %s but cites no source — set the slide's source (origin and base, or \"Illustrative\") so the numbers can be traced", si+1, what),
				Fix: &patterns.FixSuggestion{
					Kind: "provide_value",
					Params: map[string]any{
						"field": "source",
						"data":  what,
						"hint":  "set the slide's source, or a deck-level default once (raw top-level source, DeckSpec meta.source); \"Illustrative\" is accepted for estimates",
					},
				},
			},
			Action: "review",
		})
	}
	return out
}

// slideCitesSource reports whether the slide carries a source, either its own
// or one a pattern will lift into the slide's source zone.
func slideCitesSource(slide SlideInput) bool {
	if strings.TrimSpace(slide.Source) != "" {
		return true
	}
	for _, item := range slide.Content {
		if item.ChartValue != nil && strings.TrimSpace(item.ChartValue.Footnote) != "" {
			return true
		}
	}
	if slide.Pattern != nil && patternCitesSource(slide.Pattern) {
		return true
	}
	return slide.Compose != nil && composeCitesSource(slide.Compose)
}

func patternCitesSource(p *PatternInput) bool {
	_, _, ok := patterns.LiftPatternSource(p.Name, p.Values)
	return ok
}

func composeCitesSource(c *ComposeInput) bool {
	for i := range c.Segments {
		seg := &c.Segments[i]
		if seg.HasPattern() && patternCitesSource(&seg.Pattern) {
			return true
		}
		if seg.Compose != nil && composeCitesSource(seg.Compose) {
			return true
		}
	}
	return false
}

// applyDeckSourceDefault gives every data slide that cites no source of its
// own the deck-level default (raw top-level source, DeckSpec meta.source), so
// a deck built from one data set states it once instead of on every slide
// (go-slide-creator-mp2p4). It runs after pattern sources are lifted, so a
// pattern's own values.source still wins.
func applyDeckSourceDefault(input *PresentationInput) {
	source := strings.TrimSpace(input.Source)
	if source == "" {
		return
	}
	for i := range input.Slides {
		slide := &input.Slides[i]
		if !slideQualifiesForDuplicateTitleCheck(*slide) || slideCitesSource(*slide) || slideDataKind(*slide) == "" {
			continue
		}
		slide.Source = source
	}
}

// slideDataKind names the data a slide shows ("a chart", "a table of
// figures", "the kpi-4up pattern"), or "" when it shows none. It is the one
// rule both the deck-source default and DATA_WITHOUT_SOURCE use, and it looks
// wherever a slide can carry figures: content items, a pattern, a compose
// tree and shape-grid cells — the shape every semantic table, region and
// matrix compiles to (go-slide-creator-q2emv, go-slide-creator-1jtn7).
func slideDataKind(slide SlideInput) string {
	for _, item := range slide.Content {
		switch {
		case item.Type == "chart" || item.ChartValue != nil:
			return "a chart"
		case item.Type == "diagram" && item.DiagramValue != nil && isChartishDiagramType(item.DiagramValue.Type):
			return "a chart"
		case item.Type == "table" || item.TableValue != nil:
			if tableHasNumbers(item) {
				return "a table of figures"
			}
		}
	}
	if slide.Pattern != nil {
		if what := patternDataKind(slide.Pattern.Name, slide.Pattern.Values); what != "" {
			return what
		}
	}
	if slide.Compose != nil {
		if what := composeDataKind(slide.Compose); what != "" {
			return what
		}
	}
	if slide.ShapeGrid != nil {
		return gridDataKind(slide.ShapeGrid)
	}
	return ""
}

// patternDataKind names the data a pattern shows, or "". The data patterns
// always count; a table-highlight counts only when a text-scale cell holds a
// figure ("$1.2m", "25k") — Harvey and RAG judgements stay exempt.
func patternDataKind(name string, values json.RawMessage) string {
	switch {
	case dataSourcePatterns[name]:
		return "the " + name + " pattern"
	case name == "table-highlight" && tableHighlightHasFigures(values):
		return "a matrix of figures"
	}
	return ""
}

// tableHighlightHasFigures reports whether any text-scale cell of a
// table-highlight holds a figure (a digit: "$1.2m", "25k", "Q3"). A matrix of
// Harvey balls or RAG dots is a judgement call; one of figures is data an
// audience will ask to see sourced (go-slide-creator-1jtn7). A column's scale
// falls back to values.scale, then to the pattern default, harvey.
func tableHighlightHasFigures(values json.RawMessage) bool {
	var v patterns.TableHighlightValues
	if len(values) == 0 || json.Unmarshal(values, &v) != nil {
		return false
	}
	for _, o := range v.Options {
		for col, s := range o.Scores {
			scale := v.Scale
			if col < len(v.Criteria) && v.Criteria[col].Scale != "" {
				scale = v.Criteria[col].Scale
			}
			if strings.EqualFold(strings.TrimSpace(scale), "text") && strings.ContainsAny(string(s), "0123456789") {
				return true
			}
		}
	}
	return false
}

func composeDataKind(c *ComposeInput) string {
	for i := range c.Segments {
		seg := &c.Segments[i]
		if seg.HasPattern() {
			if what := patternDataKind(seg.Pattern.Name, seg.Pattern.Values); what != "" {
				return what
			}
		}
		if seg.Compose != nil {
			if what := composeDataKind(seg.Compose); what != "" {
				return what
			}
		}
	}
	return ""
}

// gridDataKind names the data a shape grid's cells show, recursing into
// sub-grids and cell-hosted patterns, or "" when it shows none.
func gridDataKind(g *ShapeGridInput) string {
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			if what := cellDataKind(cell); what != "" {
				return what
			}
		}
	}
	return ""
}

func cellDataKind(cell *GridCellInput) string {
	switch {
	case cell == nil:
		return ""
	case cell.Table != nil && tableInputHasNumbers(cell.Table):
		return "a table of figures"
	case cell.Diagram != nil && isChartishDiagramType(cell.Diagram.Type),
		cell.Composite != nil && cell.Composite.SubDiagram != nil && isChartishDiagramType(cell.Composite.SubDiagram.Type):
		return "a chart"
	case cell.Grid != nil:
		return gridDataKind(cell.Grid)
	case len(cell.Pattern) > 0:
		var p PatternInput
		if json.Unmarshal(cell.Pattern, &p) == nil {
			return patternDataKind(p.Name, p.Values)
		}
	}
	return ""
}

// tableHasNumbers reports whether any body cell of a table content item holds
// a digit. A table of words (a RACI, a glossary) is not data to be sourced.
func tableHasNumbers(item ContentInput) bool {
	resolved, err := item.ResolveValue()
	if err != nil {
		return false
	}
	table, ok := resolved.(*TableInput)
	return ok && tableInputHasNumbers(table)
}

func tableInputHasNumbers(table *TableInput) bool {
	if table == nil {
		return false
	}
	for _, row := range table.Rows {
		for _, cell := range row {
			if strings.ContainsAny(cell.Content, "0123456789") {
				return true
			}
		}
	}
	return false
}
