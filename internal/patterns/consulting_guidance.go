package patterns

var consultingUseWhen = map[string]string{
	"checklist":          "Prefer this for done/open/blocked requirements; use status-board instead when each workstream needs several status dimensions.",
	"decision-tree":      "Prefer this for conditional choices with labeled branches and outcomes; use issue-tree instead for causal hypotheses.",
	"issue-tree":         "Prefer this for decomposing a problem into hypotheses; use decision-tree instead when branches represent choices and outcomes.",
	"fishbone-framework": "Prefer this for cause categories contributing to one effect; use issue-tree instead for a hierarchical decomposition.",
	"image-hero":         "Prefer this for a full-slide photo opener with a protected headline; use image-text-split instead for a case study with substantial prose.",
	"ipo-flow":           "Prefer this for exactly three input, process and output boxes; use process-flow instead for a longer sequence of individual steps.",
	"numeric-heatmap":    "Prefer this for comparing continuous numeric magnitudes across labeled rows and columns; use status-board instead for discrete RAG judgments.",
	"pestel-framework":   "Prefer this for the six external PESTEL forces; use a SWOT framework instead for internal strengths and weaknesses alongside external factors.",
	"progress-list":      "Prefer this when every KPI needs an actual bar and a target tick; use a kpi-Nup pattern instead for headline values without target comparisons.",
	"prose-2col":         "Prefer this for running paragraphs in two columns; use comparison-2col instead for aligned pairs of comparable facts.",
	"raci-matrix":        "Prefer this for assigning R/A/C/I responsibilities with one accountable role per activity; use status-board instead to report delivery health.",
	"sources-list":       "Prefer this for a numbered appendix bibliography; use each slide's source field instead for attribution tied to one visual.",
	"spectrum":           "Prefer this for positioned markers along one continuous axis; use matrix-2x2 instead when both dimensions carry meaning.",
	"stakeholder-map":    "Prefer this for influence/interest positions with explicit stakeholder stance and size; use matrix-2x2 instead for a generic strategic portfolio.",
	"status-board":       "Prefer this for workstream RAG states with trends and comments; use option_matrix instead for scoring alternatives against decision criteria.",
	"talent-nine-box":    "Prefer this for people or teams rated by performance and potential; use matrix-2x2 instead for a two-level portfolio classification.",
	"three-horizons":     "Prefer this for core, adjacent and future growth initiatives across three horizons; use timeline-vertical instead for dated milestones.",
	"timeline-vertical":  "Prefer this for dated stops in a narrow column; use timeline-horizontal instead for a wide roadmap or date ranges.",
	"venn-framework":     "Prefer this for overlapping membership in two or three sets; use a matrix instead when values must be compared by row and column.",
}
