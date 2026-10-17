package patterns

import "strings"

func init() {
	chartRules = append(chartRules, chartRule{chartType: "mekko_chart", keywords: []string{"mekko", "marimekko", "variable width composition"}, baseScore: .99, rationale: "Variable-width segments with 100% composition"}, chartRule{chartType: "combo_chart", keywords: []string{"combo chart", "dual axis", "bars and lines"}, baseScore: .99, rationale: "Bars and lines with explicit labeled units"}, chartRule{chartType: "butterfly_chart", keywords: []string{"butterfly", "tornado comparison", "mirrored bars", "population pyramid"}, baseScore: .99, rationale: "Mirrored magnitudes and shared center category labels"})
	for _, r := range []rule{
		{pattern: "fishbone-framework", keywords: []string{"fishbone framework", "cause and effect categories", "root cause categories"}, baseScore: .99, rationale: "Named cause categories connect editable branches to one explicit effect"},
		{pattern: "numeric-heatmap", keywords: []string{"numeric heatmap", "correlation heatmap", "heatmap values"}, baseScore: .99, rationale: "A labeled numeric row-by-column grid uses a consistent value scale"},
		{pattern: "pestel-framework", keywords: []string{"pestel", "macro environment", "political economic social technological environmental legal"}, baseScore: .99, rationale: "Six external-environment lenses keep macro forces separate from internal operating choices"},
		{pattern: "talent-nine-box", keywords: []string{"nine box talent", "talent nine box", "performance potential", "talent calibration"}, baseScore: .99, rationale: "Employees positioned against performance and potential on a nine-box talent grid"},
		{pattern: "venn-framework", keywords: []string{"venn framework", "overlapping sets", "set overlap"}, baseScore: .99, rationale: "Two or three named sets show shared membership without implying numeric area proportions"},
		{pattern: "stakeholder-map", keywords: []string{"stakeholder map", "influence interest", "stakeholder stance"}, baseScore: .99, rationale: "Position, size and explicit supporter/neutral/blocker stance"},
		{pattern: "image-hero", keywords: []string{"image hero", "photo opener", "hero image"}, baseScore: .99, rationale: "Cover image and guaranteed-contrast headline scrim"},
		{pattern: "progress-list", keywords: []string{"bullet chart", "actual vs target", "kpi targets"}, baseScore: .99, rationale: "Native actual bars and explicit target ticks"},
		{pattern: "timeline-vertical", keywords: []string{"vertical timeline", "dated stops"}, baseScore: .99, rationale: "Dated stops on a native vertical spine"},
		{pattern: "spectrum", keywords: []string{"spectrum", "continuum", "positioning scale"}, baseScore: .99, rationale: "Explicitly positioned markers on a continuous axis"},
		{pattern: "issue-tree", keywords: []string{"issue tree", "mece", "hypothesis tree"}, baseScore: .99, rationale: "Bounded parent-child issues with hypothesis status"},
		{pattern: "decision-tree", keywords: []string{"decision tree", "decision branches"}, baseScore: .99, rationale: "Labeled branches and explicit decision outcomes"},
		{pattern: "raci-matrix", keywords: []string{"raci", "responsibility matrix", "responsibility assignment"}, baseScore: .99, rationale: "Activities × roles with validated R/A/C/I letters and exactly one accountable role per activity"},
		{pattern: "status-board", keywords: []string{"status board", "rag", "steerco", "workstream status"}, baseScore: .99, rationale: "RAG dots by workstream and dimension, with trend arrows and comments"},
		{pattern: "checklist", keywords: []string{"checklist", "tick list", "requirements checklist"}, baseScore: .99, rationale: "Done/open/blocked requirements retain explicit state and supporting text"},
		{pattern: "prose-2col", keywords: []string{"running prose", "memo", "two column prose"}, baseScore: .99, rationale: "Two readable prose columns with optional headings"},
		{pattern: "ipo-flow", keywords: []string{"input process output", "ipo flow", "inputs and outputs"}, baseScore: .99, rationale: "Three headed boxes with supporting bullet lists and connecting arrows"},
		{pattern: "three-horizons", keywords: []string{"three horizons", "growth horizons", "s curve"}, baseScore: .99, rationale: "Three ascending growth bands, each with a timeframe and initiatives"},
		{pattern: "sources-list", keywords: []string{"bibliography", "sources appendix", "reference list"}, baseScore: .99, rationale: "Numbered sources with optional URLs at appendix density"},
	} {
		rules = append(rules, r)
		visualNameAliases["named_pattern:"+r.pattern] = append([]string{strings.ReplaceAll(r.pattern, "-", " ")}, r.keywords...)
	}
}
func unsupportedVisualRecipe(name string) string {
	if name == "sankey" {
		return "Sankey is unsupported. Alternative recipe: use stacked_bar with one category per stage and one series per cohort to show stage composition; add a waterfall for the gains/losses between stages, retain the same cohort labels and totals, and state that individual cross-stage links are not encoded."
	}
	return ""
}
