package patterns

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/svggen"
)

// VisualCategory classifies the recommended visual approach.
type VisualCategory string

const (
	VisualCategoryPlaceholder VisualCategory = "placeholder_layout"
	VisualCategoryPattern     VisualCategory = "named_pattern"
	VisualCategoryChart       VisualCategory = "chart"
	VisualCategoryDiagram     VisualCategory = "diagram"
	VisualCategoryShapeGrid   VisualCategory = "raw_shape_grid"
	// VisualCategoryCompose signals that the slide is best authored as a
	// compose envelope combining two or more sibling patterns (e.g.,
	// stylish-panels + pull-quote, kpi-3up + process-flow). Compose candidates
	// always carry placement.composable_with populated from the high-affinity
	// pattern pair that triggered the suggestion.
	VisualCategoryCompose VisualCategory = "compose"
)

// PlacementGuidance describes how to author and place a recommended candidate.
// It helps the agent choose the correct authoring path without guessing.
type PlacementGuidance struct {
	// PreferredPlacement is the recommended placement context: "placeholder", "shape_grid", or "either".
	PreferredPlacement string `json:"preferred_placement"`
	// HostStrategy describes how to host this visual: "placeholder_content", "grid_cell", "pattern_expansion", or "standalone_slide".
	HostStrategy string `json:"host_strategy"`
	// GridEmbeddable indicates whether this candidate can be placed inside a shape_grid cell.
	GridEmbeddable bool `json:"grid_embeddable"`
	// RenderPipeline is the render strategy in the preferred placement: "native_ooxml", "svg", or "template_driven".
	RenderPipeline string `json:"render_pipeline"`
	// ComposableWith lists categories or patterns this candidate composes with on a single slide.
	ComposableWith []string `json:"composable_with,omitempty"`
}

// Template-support status values. They classify how well a candidate fits a
// specific template once template context is supplied.
const (
	// TemplateSupportSupported means the template natively covers the layout /
	// capability the candidate needs and no capacity concern was detected.
	TemplateSupportSupported = "supported"
	// TemplateSupportRisky means the candidate is producible but only via a
	// synthesized/derived layout, or it brushes against a capacity / content-zone
	// constraint. The reasons explain the caveat.
	TemplateSupportRisky = "risky"
	// TemplateSupportUnsupported means the candidate requires a canonical or
	// derivable layout the template cannot provide. The reasons name what is
	// missing.
	TemplateSupportUnsupported = "unsupported"
)

// TemplateSupport reports how well a visual candidate fits a specific template.
// It is populated by the engine (internal/generator) when the recommend_visual
// caller supplies template context; it is nil for template-agnostic results.
type TemplateSupport struct {
	// Status is one of "supported", "risky", or "unsupported".
	Status string `json:"status"`
	// Reasons explain the status: which layouts cover the candidate, what is
	// synthesized, which capacity / content-zone constraint applies, or what is
	// absent. Always carries at least one entry.
	Reasons []string `json:"reasons,omitempty"`
	// RequiredLayout names the canonical layout or derivable capability the
	// candidate needs (e.g. "Title Slide", "Two Content", "full-image",
	// "grid base"). Empty for candidates with no specific layout requirement.
	RequiredLayout string `json:"required_layout,omitempty"`
}

// VisualCandidate is a scored recommendation across all visual types.
type VisualCandidate struct {
	Category       VisualCategory `json:"category"`
	Name           string         `json:"name"`
	Score          float64        `json:"score"`
	Rationale      string         `json:"rationale"`
	ConfidenceBand string         `json:"confidence_band"`
	DiversityBonus bool           `json:"diversity_bonus,omitempty"`
	// Placement provides authoring guidance for the agent. Populated by the MCP layer
	// from capability truth; nil when returned from the pure scoring function.
	Placement *PlacementGuidance `json:"placement,omitempty"`
	// TemplateSupport reports template-specific feasibility. Populated by the
	// engine when the caller supplies template context; nil for
	// template-agnostic results.
	TemplateSupport *TemplateSupport `json:"template_support,omitempty"`
}

// VisualHints extends ContentHints with data-shape information for chart/diagram routing.
type VisualHints struct {
	ContentHints
	DataPoints  int    `json:"data_points,omitempty"`  // number of data points (for chart routing)
	SeriesCount int    `json:"series_count,omitempty"` // number of data series
	Audience    string `json:"audience,omitempty"`     // e.g. "executive", "technical", "investor"
}

// RecommendVisualResult is the output of RecommendVisual.
type RecommendVisualResult struct {
	Candidates              []VisualCandidate `json:"candidates"`
	QueryUnderstood         string            `json:"query_understood_as"`
	DisambiguatingQuestions []string          `json:"disambiguating_questions,omitempty"`

	// ResponseFingerprint is a sha256 hex digest of the canonical JSON of this
	// response with the field zeroed. Agents may use it as a cache key. The
	// MCP handler sets this; callers using RecommendVisual directly may leave
	// it empty.
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`
}

// chartRule maps intent keywords to a chart type with scoring.
type chartRule struct {
	chartType         string
	keywords          []string
	baseScore         float64
	rationale         string
	needsMultiSeries  bool
	needsSingleSeries bool
}

// diagramRule maps intent keywords to a diagram type with scoring.
type diagramRule struct {
	diagramType string
	keywords    []string
	baseScore   float64
	rationale   string
}

// placeholderRule maps intent keywords to a slide type with scoring.
type placeholderRule struct {
	slideType string
	keywords  []string
	baseScore float64
	rationale string
}

var chartRules = []chartRule{
	// Trend / time-series
	{chartType: "line", keywords: []string{"trend", "time series", "over time", "monthly", "quarterly", "yearly", "growth", "decline", "trajectory"}, baseScore: 0.90, rationale: "Line chart for showing trends over time"},
	{chartType: "area", keywords: []string{"area", "cumulative", "volume over time", "stacked area"}, baseScore: 0.85, rationale: "Area chart for cumulative or volume trends"},
	{chartType: "stacked_area", keywords: []string{"stacked area", "composition over time", "mix over time"}, baseScore: 0.88, rationale: "Stacked area for showing composition changes over time"},

	// Comparison
	{chartType: "bar", keywords: []string{"bar", "compare", "comparison", "rank", "ranking", "revenue", "sales", "budget"}, baseScore: 0.85, rationale: "Bar chart for comparing categories"},
	{chartType: "grouped_bar", keywords: []string{"grouped", "side by side", "multi-series comparison"}, baseScore: 0.88, rationale: "Grouped bar chart for multi-series category comparison", needsMultiSeries: true},
	{chartType: "stacked_bar", keywords: []string{"stacked", "composition", "breakdown", "proportion"}, baseScore: 0.85, rationale: "Stacked bar chart for showing part-to-whole within categories"},

	// Distribution / proportion
	{chartType: "pie", keywords: []string{"pie", "share", "distribution", "market share", "allocation", "split"}, baseScore: 0.85, rationale: "Pie chart for showing proportions of a whole", needsSingleSeries: true},
	{chartType: "donut", keywords: []string{"donut", "ring", "share", "distribution"}, baseScore: 0.82, rationale: "Donut chart for proportions with center annotation", needsSingleSeries: true},
	{chartType: "treemap", keywords: []string{"treemap", "hierarchical proportion", "nested proportion"}, baseScore: 0.85, rationale: "Treemap for hierarchical part-to-whole visualization"},

	// Correlation / scatter
	{chartType: "scatter", keywords: []string{"scatter", "correlation", "relationship", "xy", "plot", "regression"}, baseScore: 0.90, rationale: "Scatter plot for showing correlations between two variables"},
	{chartType: "bubble", keywords: []string{"bubble", "three variable", "3d scatter", "size and position"}, baseScore: 0.88, rationale: "Bubble chart for three-variable relationships"},

	// Specialized
	{chartType: "radar", keywords: []string{"radar", "spider", "capability", "competency", "multi-dimension", "assessment"}, baseScore: 0.90, rationale: "Radar/spider chart for multi-dimensional comparison"},
	{chartType: "waterfall", keywords: []string{"waterfall", "bridge", "variance", "change breakdown", "incremental"}, baseScore: 0.92, rationale: "Waterfall chart for showing incremental changes"},
	{chartType: "funnel", keywords: []string{"funnel", "conversion", "pipeline", "stages", "attrition", "drop-off"}, baseScore: 0.92, rationale: "Funnel chart for conversion pipeline visualization"},
	{chartType: "gauge", keywords: []string{"gauge", "meter", "speedometer", "target", "threshold", "progress toward"}, baseScore: 0.90, rationale: "Gauge for single-value progress or target tracking"},
}

var diagramRules = []diagramRule{
	{diagramType: "timeline", keywords: []string{"timeline", "milestone", "chronological", "history", "evolution"}, baseScore: 0.90, rationale: "Timeline diagram for chronological events"},
	{diagramType: "process_flow", keywords: []string{"process", "flow", "workflow", "decision", "flowchart", "procedure", "pipeline"}, baseScore: 0.90, rationale: "Process flow diagram with steps and decisions"},
	{diagramType: "pyramid", keywords: []string{"pyramid", "hierarchy", "maslow", "tiered", "levels"}, baseScore: 0.90, rationale: "Pyramid diagram for hierarchical structures"},
	{diagramType: "venn", keywords: []string{"venn", "overlap", "intersection", "shared", "common ground"}, baseScore: 0.92, rationale: "Venn diagram for showing overlapping concepts"},
	{diagramType: "swot", keywords: []string{"swot", "strengths", "weaknesses", "opportunities", "threats"}, baseScore: 0.95, rationale: "SWOT analysis diagram"},
	{diagramType: "org_chart", keywords: []string{"org chart", "organization", "reporting structure", "hierarchy chart", "team structure"}, baseScore: 0.92, rationale: "Organizational chart for reporting structures"},
	{diagramType: "gantt", keywords: []string{"gantt", "project plan", "schedule", "task timeline", "project timeline"}, baseScore: 0.92, rationale: "Gantt chart for project scheduling"},
	{diagramType: "matrix_2x2", keywords: []string{"matrix", "quadrant", "2x2", "positioning", "priority matrix", "impact effort"}, baseScore: 0.90, rationale: "2x2 matrix for positioning and prioritization"},
	{diagramType: "porters_five_forces", keywords: []string{"porter", "five forces", "competitive", "industry analysis"}, baseScore: 0.95, rationale: "Porter's Five Forces competitive analysis"},
	{diagramType: "house_diagram", keywords: []string{"house", "temple", "pillars", "foundation", "strategic house"}, baseScore: 0.90, rationale: "House/temple diagram for strategic frameworks"},
	{diagramType: "business_model_canvas", keywords: []string{"business model canvas", "bmc", "osterwalder", "lean canvas"}, baseScore: 0.95, rationale: "Business Model Canvas for business design"},
	{diagramType: "value_chain", keywords: []string{"value chain", "porter value", "primary activities", "support activities"}, baseScore: 0.92, rationale: "Value chain analysis diagram"},
	{diagramType: "fishbone", keywords: []string{"fishbone", "ishikawa", "cause and effect", "root cause", "cause effect"}, baseScore: 0.92, rationale: "Fishbone/Ishikawa diagram for root cause analysis"},
	{diagramType: "pestel", keywords: []string{"pestel", "pest", "political", "economic", "social", "technological", "environmental", "legal"}, baseScore: 0.92, rationale: "PESTEL analysis framework"},
	{diagramType: "heatmap", keywords: []string{"heatmap", "heat map", "intensity", "density map"}, baseScore: 0.88, rationale: "Heatmap for visualizing intensity across two dimensions"},
	{diagramType: "nine_box_talent", keywords: []string{"9 box", "nine box", "talent grid", "performance potential"}, baseScore: 0.92, rationale: "9-box talent grid for performance vs. potential"},
	{diagramType: "kpi_dashboard", keywords: []string{"kpi dashboard", "metrics dashboard", "dashboard layout"}, baseScore: 0.88, rationale: "KPI dashboard layout for multiple metrics"},
	{diagramType: "panel_layout", keywords: []string{"panel", "panels", "info panel", "multi-panel"}, baseScore: 0.85, rationale: "Panel layout for organized information display"},
	{diagramType: "icon_columns", keywords: []string{"icon column", "feature column", "vertical icons"}, baseScore: 0.85, rationale: "Icon columns for vertical feature display"},
	{diagramType: "icon_rows", keywords: []string{"icon row", "feature row", "horizontal icons"}, baseScore: 0.85, rationale: "Icon rows for horizontal feature display"},
	{diagramType: "stat_cards", keywords: []string{"stat card", "metric card", "number card"}, baseScore: 0.85, rationale: "Stat cards for metric display"},
}

var placeholderRules = []placeholderRule{
	{slideType: "title", keywords: []string{"title slide", "opening slide", "cover", "intro slide", "welcome"}, baseScore: 0.95, rationale: "Title slide — use a standard placeholder layout for the opening"},
	{slideType: "section", keywords: []string{"section", "divider", "chapter", "section break", "transition"}, baseScore: 0.92, rationale: "Section divider slide for separating deck sections"},
	{slideType: "content", keywords: []string{"bullet", "text", "talking points", "key points", "summary", "overview", "agenda"}, baseScore: 0.85, rationale: "Standard content slide with title and bullets"},
	{slideType: "two-column", keywords: []string{"two column", "side by side", "left right", "dual", "split layout"}, baseScore: 0.88, rationale: "Two-column layout for side-by-side content"},
	{slideType: "image", keywords: []string{"image", "photo", "picture", "screenshot", "visual"}, baseScore: 0.88, rationale: "Image-focused slide layout"},
	{slideType: "blank", keywords: []string{"blank", "empty", "free form", "custom layout"}, baseScore: 0.80, rationale: "Blank slide for fully custom content"},
}

// RecommendVisual scores all visual categories against the given intent and hints,
// returning up to maxCandidates results ranked by score. This is the unified
// replacement for recommend_pattern — it routes to placeholder layouts, named
// patterns, charts, diagrams, or raw shape_grid as appropriate.
func RecommendVisual(reg *Registry, intent string, hints *VisualHints, maxCandidates int, opts ...*RecommendOptions) RecommendVisualResult {
	if maxCandidates <= 0 {
		maxCandidates = 5
	}
	if hints == nil {
		hints = &VisualHints{}
	}
	var options *RecommendOptions
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	intentLower := strings.ToLower(intent)

	// Build recency count map for variety penalty on patterns.
	recencyCount := make(map[string]int)
	if options != nil {
		for _, p := range options.RecentPatterns {
			recencyCount[p]++
		}
	}
	applyVariety := options != nil && options.PreferVariety && len(recencyCount) > 0

	// Explicit-candidates mode: rank only the supplied shortlist across all
	// visual categories with no threshold cutoff and no truncation.
	if options != nil && len(options.Candidates) > 0 {
		return recommendVisualOnlyCandidates(reg, intentLower, hints, options.Candidates, recencyCount, applyVariety)
	}

	var all []VisualCandidate

	// 1. Score placeholder layouts.
	all = append(all, scorePlaceholders(intentLower)...)

	// 2. Score named patterns (reuse existing Recommend logic).
	patternCandidates := scorePatterns(reg, intentLower, &hints.ContentHints, recencyCount, applyVariety)
	all = append(all, patternCandidates...)

	// 3. Score chart types.
	all = append(all, scoreCharts(intentLower, hints)...)

	// 4. Score diagram types.
	all = append(all, scoreDiagrams(intentLower)...)

	// 5. Score compose envelopes — emitted when the intent suggests combining
	// multiple patterns on one slide OR when the top pattern candidates have
	// declared compose-affinity (PatternTaxonomy.ComposesWith).
	all = append(all, scoreCompose(reg, intentLower, patternCandidates)...)

	// Sort by score descending.
	sort.Slice(all, func(i, j int) bool {
		if all[i].Score != all[j].Score {
			return all[i].Score > all[j].Score
		}
		return all[i].Name < all[j].Name
	})

	// Truncate and filter.
	var filtered []VisualCandidate
	for _, c := range all {
		if c.Score < 0.5 {
			continue
		}
		filtered = append(filtered, c)
		if len(filtered) >= maxCandidates {
			break
		}
	}

	// If nothing scored above threshold, add raw_shape_grid as fallback.
	if len(filtered) == 0 {
		filtered = append(filtered, VisualCandidate{
			Category:       VisualCategoryShapeGrid,
			Name:           "raw_shape_grid",
			Score:          0.30,
			Rationale:      "No specific visual type matched this intent. Consider using shape_grid directly to build a custom layout, or try rephrasing with more specific keywords.",
			ConfidenceBand: "low",
		})
	}

	result := RecommendVisualResult{
		Candidates:      filtered,
		QueryUnderstood: summarizeVisualIntent(intentLower, hints),
	}

	result.DisambiguatingQuestions = suggestVisualQuestions(hints, intentLower, filtered)

	return result
}

// recommendVisualOnlyCandidates ranks an explicit list of candidate names
// across all visual categories. Threshold cutoff and truncation are skipped —
// every supplied name is returned. The category for each name is resolved
// from the catalogs (placeholder rules, pattern registry, chart capabilities,
// diagram capabilities); names not found anywhere are returned with category
// raw_shape_grid and score 0, with a rationale noting the miss.
func recommendVisualOnlyCandidates(reg *Registry, intentLower string, hints *VisualHints, names []string, recencyCount map[string]int, applyVariety bool) RecommendVisualResult {
	placeholderByType := make(map[string]placeholderRule, len(placeholderRules))
	for _, r := range placeholderRules {
		placeholderByType[r.slideType] = r
	}
	chartByType := make(map[string]chartRule, len(chartRules))
	for _, r := range chartRules {
		chartByType[r.chartType] = r
	}
	diagramByType := make(map[string]diagramRule, len(diagramRules))
	for _, r := range diagramRules {
		diagramByType[r.diagramType] = r
	}
	readyCharts := make(map[string]bool)
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			readyCharts[c.Type] = true
		}
	}
	readyDiagrams := make(map[string]bool)
	for _, d := range svggen.DiagramCapabilitiesReady() {
		readyDiagrams[d.Type] = true
	}

	out := make([]VisualCandidate, 0, len(names))
	for _, name := range names {
		out = append(out, scoreVisualCandidate(reg, intentLower, hints, name,
			placeholderByType, chartByType, diagramByType, readyCharts, readyDiagrams,
			recencyCount, applyVariety))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})

	return RecommendVisualResult{
		Candidates:              out,
		QueryUnderstood:         summarizeVisualIntent(intentLower, hints),
		DisambiguatingQuestions: suggestVisualQuestions(hints, intentLower, out),
	}
}

// scoreVisualCandidate resolves a single candidate name to its category and
// score. It returns a VisualCandidate even when the name matches no rule —
// the caller chose this name and is owed a ranked entry.
func scoreVisualCandidate( //nolint:gocognit,gocyclo
	reg *Registry, intentLower string, hints *VisualHints, name string,
	placeholderByType map[string]placeholderRule,
	chartByType map[string]chartRule,
	diagramByType map[string]diagramRule,
	readyCharts, readyDiagrams map[string]bool,
	recencyCount map[string]int, applyVariety bool,
) VisualCandidate {
	// 1. Placeholder layouts.
	if pr, ok := placeholderByType[name]; ok {
		s := scoreKeywords(pr.keywords, intentLower, pr.baseScore)
		return VisualCandidate{
			Category:       VisualCategoryPlaceholder,
			Name:           name,
			Score:          roundScore(s),
			Rationale:      pr.rationale,
			ConfidenceBand: confidenceBand(s),
		}
	}

	// 2. Named patterns in the registry.
	if reg != nil {
		if _, ok := reg.Get(name); ok {
			var (
				bestRule  rule
				bestScore float64
				haveRule  bool
			)
			for i := range rules {
				if rules[i].pattern != name {
					continue
				}
				s := scoreRule(rules[i], intentLower, &hints.ContentHints)
				if !haveRule || s > bestScore {
					bestRule = rules[i]
					bestScore = s
					haveRule = true
				}
			}
			rationale := "Named pattern from the catalog (no scoring rule matched intent keywords)."
			if haveRule {
				rationale = bestRule.rationale
			}
			// Apply variety / density adjustments inline.
			if applyVariety {
				if count, ok := recencyCount[name]; ok && count > 0 {
					penalty := 0.15 * float64(count)
					bestScore -= penalty
					if bestScore < 0 {
						bestScore = 0
					}
				}
			}
			if hints.DensityHint != "" && reg != nil {
				if pat, ok := reg.Get(name); ok {
					patDensity := pat.Taxonomy().DensityClass
					switch {
					case patDensity == hints.DensityHint:
						bestScore += 0.10
					case densityDistance(patDensity, hints.DensityHint) > 1:
						bestScore -= 0.15
					default:
						bestScore -= 0.05
					}
				}
				if bestScore > 1.0 {
					bestScore = 1.0
				}
				if bestScore < 0 {
					bestScore = 0
				}
			}
			return VisualCandidate{
				Category:       VisualCategoryPattern,
				Name:           name,
				Score:          roundScore(bestScore),
				Rationale:      rationale,
				ConfidenceBand: confidenceBand(bestScore),
			}
		}
	}

	// 3. Chart types (ready ones only — non-ready charts can't render).
	if readyCharts[name] {
		if cr, ok := chartByType[name]; ok {
			s := scoreKeywords(cr.keywords, intentLower, cr.baseScore)
			if cr.needsMultiSeries && hints.SeriesCount > 1 {
				s += 0.05
			}
			if cr.needsSingleSeries && hints.SeriesCount > 1 {
				s -= 0.2
			}
			if s > 1.0 {
				s = 1.0
			}
			if s < 0 {
				s = 0
			}
			return VisualCandidate{
				Category:       VisualCategoryChart,
				Name:           name,
				Score:          roundScore(s),
				Rationale:      cr.rationale,
				ConfidenceBand: confidenceBand(s),
			}
		}
	}

	// 4. Diagram types (ready ones only).
	if readyDiagrams[name] {
		if dr, ok := diagramByType[name]; ok {
			s := scoreKeywords(dr.keywords, intentLower, dr.baseScore)
			return VisualCandidate{
				Category:       VisualCategoryDiagram,
				Name:           name,
				Score:          roundScore(s),
				Rationale:      dr.rationale,
				ConfidenceBand: confidenceBand(s),
			}
		}
	}

	// 5. Raw shape_grid escape hatch.
	if name == "raw_shape_grid" {
		return VisualCandidate{
			Category:       VisualCategoryShapeGrid,
			Name:           name,
			Score:          0.30,
			Rationale:      "Raw shape_grid for fully custom layouts.",
			ConfidenceBand: confidenceBand(0.30),
		}
	}

	// 6. Unknown candidate — still returned, with rationale noting the miss.
	return VisualCandidate{
		Category:       VisualCategoryShapeGrid,
		Name:           name,
		Score:          0,
		Rationale:      "Unknown candidate name; not found in placeholder, pattern, chart, or diagram catalogs.",
		ConfidenceBand: confidenceBand(0),
	}
}

// scorePlaceholders evaluates placeholder layout rules.
func scorePlaceholders(intentLower string) []VisualCandidate {
	var candidates []VisualCandidate
	for _, r := range placeholderRules {
		score := scoreKeywords(r.keywords, intentLower, r.baseScore)
		if score < 0.3 {
			continue
		}
		candidates = append(candidates, VisualCandidate{
			Category:       VisualCategoryPlaceholder,
			Name:           r.slideType,
			Score:          roundScore(score),
			Rationale:      r.rationale,
			ConfidenceBand: confidenceBand(score),
		})
	}
	return dedupByName(candidates)
}

// scorePatterns evaluates named pattern rules, reusing the existing scoring logic.
func scorePatterns(reg *Registry, intentLower string, hints *ContentHints, recencyCount map[string]int, applyVariety bool) []VisualCandidate {
	flat := scoreAndDedup(intentLower, hints)

	if applyVariety {
		applyRecencyDecay(flat, recencyCount)
	}

	// Apply density-class preference when the caller provides a density hint.
	if hints != nil && hints.DensityHint != "" && reg != nil {
		applyDensityPreference(flat, reg, hints.DensityHint)
	}

	var candidates []VisualCandidate
	for _, c := range flat {
		if c.score < 0.3 {
			continue
		}
		// Verify pattern actually exists in registry.
		if reg != nil {
			if _, ok := reg.Get(c.rule.pattern); !ok {
				continue
			}
		}
		candidates = append(candidates, VisualCandidate{
			Category:       VisualCategoryPattern,
			Name:           c.rule.pattern,
			Score:          roundScore(c.score),
			Rationale:      c.rule.rationale,
			ConfidenceBand: confidenceBand(c.score),
			DiversityBonus: c.diversityBonus,
		})
	}
	return candidates
}

// applyDensityPreference boosts patterns whose DensityClass matches the hint
// and penalizes those that diverge.
func applyDensityPreference(flat []scored, reg *Registry, densityHint string) {
	for i := range flat {
		pat, ok := reg.Get(flat[i].rule.pattern)
		if !ok {
			continue
		}
		patDensity := pat.Taxonomy().DensityClass
		if patDensity == densityHint {
			flat[i].score += 0.10
		} else if densityDistance(patDensity, densityHint) > 1 {
			// Two steps away (low vs high): larger penalty
			flat[i].score -= 0.15
		} else {
			// One step away: small penalty
			flat[i].score -= 0.05
		}
		if flat[i].score > 1.0 {
			flat[i].score = 1.0
		}
		if flat[i].score < 0.0 {
			flat[i].score = 0.0
		}
	}
}

// densityDistance returns the ordinal distance between two density classes.
func densityDistance(a, b string) int {
	ord := map[string]int{"low": 0, "medium": 1, "high": 2}
	va, oka := ord[a]
	vb, okb := ord[b]
	if !oka || !okb {
		return 0
	}
	d := va - vb
	if d < 0 {
		return -d
	}
	return d
}

// scoreCharts evaluates chart type rules.
func scoreCharts(intentLower string, hints *VisualHints) []VisualCandidate {
	// Build a set of ready chart types for validation.
	readyCharts := make(map[string]bool)
	for _, c := range svggen.ChartCapabilities() {
		if c.Status == "ready" {
			readyCharts[c.Type] = true
		}
	}

	var candidates []VisualCandidate
	for _, r := range chartRules {
		if !readyCharts[r.chartType] {
			continue
		}
		score := scoreKeywords(r.keywords, intentLower, r.baseScore)
		if score < 0.3 {
			continue
		}
		// Adjust for series count hints.
		if r.needsMultiSeries && hints.SeriesCount > 0 && hints.SeriesCount > 1 {
			score += 0.05
		}
		if r.needsSingleSeries && hints.SeriesCount > 1 {
			score -= 0.2
		}
		if score > 1.0 {
			score = 1.0
		}
		if score < 0.0 {
			score = 0.0
		}
		candidates = append(candidates, VisualCandidate{
			Category:       VisualCategoryChart,
			Name:           r.chartType,
			Score:          roundScore(score),
			Rationale:      r.rationale,
			ConfidenceBand: confidenceBand(score),
		})
	}
	return dedupByName(candidates)
}

// scoreDiagrams evaluates diagram type rules.
func scoreDiagrams(intentLower string) []VisualCandidate {
	// Build a set of ready diagram types for validation.
	readyDiagrams := make(map[string]bool)
	for _, d := range svggen.DiagramCapabilitiesReady() {
		readyDiagrams[d.Type] = true
	}

	var candidates []VisualCandidate
	for _, r := range diagramRules {
		if !readyDiagrams[r.diagramType] {
			continue
		}
		score := scoreKeywords(r.keywords, intentLower, r.baseScore)
		if score < 0.3 {
			continue
		}
		candidates = append(candidates, VisualCandidate{
			Category:       VisualCategoryDiagram,
			Name:           r.diagramType,
			Score:          roundScore(score),
			Rationale:      r.rationale,
			ConfidenceBand: confidenceBand(score),
		})
	}
	return dedupByName(candidates)
}

// scoreKeywords computes a score based on keyword matches against intent.
func scoreKeywords(keywords []string, intentLower string, baseScore float64) float64 {
	matched := false
	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(intentLower, kw) {
			matched = true
			matchCount++
		}
	}
	if !matched {
		return 0
	}
	score := baseScore
	if matchCount > 1 {
		score += 0.05 * float64(min(matchCount-1, 3))
	}
	return score
}

// dedupByName keeps the highest-scoring candidate for each name.
func dedupByName(candidates []VisualCandidate) []VisualCandidate {
	best := make(map[string]VisualCandidate)
	for _, c := range candidates {
		if prev, ok := best[c.Name]; !ok || c.Score > prev.Score {
			best[c.Name] = c
		}
	}
	result := make([]VisualCandidate, 0, len(best))
	for _, c := range best {
		result = append(result, c)
	}
	return result
}

func roundScore(s float64) float64 {
	return math.Round(s*100) / 100
}

// summarizeVisualIntent creates a human-readable echo of how the query was parsed.
func summarizeVisualIntent(intent string, hints *VisualHints) string {
	base := summarizeIntent(intent, &hints.ContentHints)
	var sb strings.Builder
	sb.WriteString(base)
	if hints.DataPoints > 0 {
		fmt.Fprintf(&sb, ", data_points=%d", hints.DataPoints)
	}
	if hints.SeriesCount > 0 {
		fmt.Fprintf(&sb, ", series_count=%d", hints.SeriesCount)
	}
	if hints.Audience != "" {
		fmt.Fprintf(&sb, ", audience=%q", hints.Audience)
	}
	return sb.String()
}

// composeIntentKeywords lists intent fragments that signal the agent is asking
// for a single slide that combines multiple visual elements. A match boosts
// the compose envelope candidate so agents discover the feature without
// reading the raw input schema.
var composeIntentKeywords = []string{
	"side by side", "side-by-side", "alongside", "next to",
	"combine", "combined", "composition", "composed",
	"compose", "merge", "split layout",
	"with chart", "with diagram", "with kpi", "with quote", "with sparkline",
	"kpi and chart", "kpi + chart", "panels and quote",
	"two patterns", "multi-pattern", "multi pattern",
	"hybrid layout", "hybrid slide",
	"one slide showing both", "both on one slide",
}

// composeNameFor returns the canonical compose-candidate name for the given
// (left, right) sibling pair. The pair is sorted so "stylish-panels +
// pull-quote" and "pull-quote + stylish-panels" produce the same name and
// thus dedupe in the final ranking.
func composeNameFor(left, right string) string {
	if left == right {
		return "compose:" + left
	}
	a, b := left, right
	if a > b {
		a, b = b, a
	}
	return "compose:" + a + "+" + b
}

// scoreCompose returns compose-envelope candidates. Two signals fire:
//
//  1. The intent contains an explicit "combine these two things" keyword
//     (composeIntentKeywords). In that case the top two ComposesWith-bearing
//     pattern candidates are paired into a compose suggestion.
//  2. The top two pattern candidates declare mutual compose-affinity via
//     PatternTaxonomy.ComposesWith. This surfaces compose even when the user
//     did not name it, because the underlying patterns are known sibling-
//     compatible (e.g., stylish-panels ↔ pull-quote).
//
// Each compose candidate carries placement.composable_with populated with the
// pair of sibling pattern names so agents have the exact names to plug into a
// ComposeInput envelope without a second discovery call.
func scoreCompose(reg *Registry, intentLower string, patternCandidates []VisualCandidate) []VisualCandidate {
	if reg == nil || len(patternCandidates) < 2 {
		return nil
	}

	// Detect explicit compose intent.
	explicitIntent := false
	for _, kw := range composeIntentKeywords {
		if strings.Contains(intentLower, kw) {
			explicitIntent = true
			break
		}
	}

	// Find the top two pattern candidates that declare mutual compose-affinity.
	// patternCandidates is already in registry-scored order; iterate to find
	// the first pair where each declares the other in its ComposesWith axis.
	type pairCand struct {
		left, right string
		score       float64
	}
	var bestPair *pairCand

	for i := 0; i < len(patternCandidates); i++ {
		pi, ok := reg.Get(patternCandidates[i].Name)
		if !ok {
			continue
		}
		ciWith := pi.Taxonomy().ComposesWith
		if len(ciWith) == 0 {
			continue
		}
		for j := i + 1; j < len(patternCandidates); j++ {
			pj, ok := reg.Get(patternCandidates[j].Name)
			if !ok {
				continue
			}
			if !sliceContains(ciWith, patternCandidates[j].Name) {
				// One-way affinity isn't enough — require mutual to avoid
				// recommending a pair the second pattern doesn't endorse.
				if !sliceContains(pj.Taxonomy().ComposesWith, patternCandidates[i].Name) {
					continue
				}
			}
			// Average the two scores so the compose candidate sits between
			// its constituent patterns (compose is an additional option, not
			// a replacement for the individual recommendations).
			combined := (patternCandidates[i].Score + patternCandidates[j].Score) / 2
			if bestPair == nil || combined > bestPair.score {
				bestPair = &pairCand{
					left:  patternCandidates[i].Name,
					right: patternCandidates[j].Name,
					score: combined,
				}
			}
			// Stop on the first pair for each i — the candidates are already
			// score-ordered so the next j only ties or loses.
			break
		}
	}

	if bestPair == nil {
		return nil
	}

	// Boost the score when the user explicitly asked for a composed layout.
	score := bestPair.score
	rationale := fmt.Sprintf(
		"Compose envelope combining %q and %q — both patterns declare mutual compose-affinity in their taxonomy.",
		bestPair.left, bestPair.right,
	)
	if explicitIntent {
		score += 0.10
		rationale = fmt.Sprintf(
			"Compose envelope combining %q and %q — the intent requests a multi-pattern layout and the pair declares mutual compose-affinity.",
			bestPair.left, bestPair.right,
		)
	}
	if score > 1.0 {
		score = 1.0
	}

	return []VisualCandidate{{
		Category:       VisualCategoryCompose,
		Name:           composeNameFor(bestPair.left, bestPair.right),
		Score:          roundScore(score),
		Rationale:      rationale,
		ConfidenceBand: confidenceBand(score),
		Placement: &PlacementGuidance{
			PreferredPlacement: "placeholder",
			HostStrategy:       "pattern_expansion",
			GridEmbeddable:     false,
			RenderPipeline:     "native_ooxml",
			ComposableWith:     []string{bestPair.left, bestPair.right},
		},
	}}
}

// sliceContains reports whether needle appears anywhere in haystack.
func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// suggestVisualQuestions returns clarifying questions to refine the recommendation.
func suggestVisualQuestions(hints *VisualHints, intent string, candidates []VisualCandidate) []string {
	var qs []string

	// Check if multiple categories are competing.
	if len(candidates) >= 2 {
		cat1, cat2 := candidates[0].Category, candidates[1].Category
		gap := candidates[0].Score - candidates[1].Score
		if cat1 != cat2 && gap < 0.1 {
			qs = append(qs, "Should this slide show data (chart/diagram) or structured text (pattern/layout)?")
		}
	}

	if hints.ItemCount == 0 {
		qs = append(qs, "How many items will this slide contain?")
	}

	// Data-oriented questions.
	hasDataKeywords := false
	for _, kw := range []string{"chart", "graph", "data", "trend", "compare", "metric"} {
		if strings.Contains(intent, kw) {
			hasDataKeywords = true
			break
		}
	}
	if hasDataKeywords && hints.DataPoints == 0 {
		qs = append(qs, "How many data points or categories are in the dataset?")
	}
	if hasDataKeywords && hints.SeriesCount == 0 {
		qs = append(qs, "Is this a single series or multiple series comparison?")
	}

	if len(qs) > 3 {
		qs = qs[:3]
	}
	return qs
}
