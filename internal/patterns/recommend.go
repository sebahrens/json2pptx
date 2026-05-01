package patterns

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ContentHints carries optional structured hints about the content the agent
// wants to place on a slide. These refine keyword-based scoring.
type ContentHints struct {
	ItemCount  int  `json:"item_count,omitempty"`
	HasChart   bool `json:"has_chart,omitempty"`
	HasMetrics bool `json:"has_metrics,omitempty"`
	Columns    int  `json:"columns,omitempty"`
}

// Candidate is a single recommendation result.
type Candidate struct {
	PatternName string  `json:"pattern_name"`
	Score       float64 `json:"score"`
	Rationale   string  `json:"rationale"`
}

// RecommendResult is the output of Recommend.
type RecommendResult struct {
	Candidates      []Candidate `json:"candidates"`
	QueryUnderstood string      `json:"query_understood_as"`
}

// rule maps keywords and content hints to a pattern with a base confidence.
type rule struct {
	pattern    string
	keywords   []string // any keyword match activates the rule
	baseScore  float64
	rationale  string
	itemMin    int // 0 = no constraint
	itemMax    int // 0 = no constraint
	needsCols  int // 0 = no constraint
	needsMetrics bool
}

// rules is the curated recommendation table. Order does not matter — all
// rules are evaluated and the top candidates are returned.
var rules = []rule{
	// KPI patterns — metrics/numbers
	{
		pattern:      "kpi-3up",
		keywords:     []string{"kpi", "metric", "number", "stat", "scorecard", "dashboard"},
		baseScore:    0.90,
		rationale:    "Best for exactly 3 big-number KPIs with short captions",
		itemMin:      1,
		itemMax:      3,
		needsMetrics: true,
	},
	{
		pattern:      "kpi-4up",
		keywords:     []string{"kpi", "metric", "number", "stat", "scorecard", "dashboard"},
		baseScore:    0.90,
		rationale:    "Best for exactly 4 big-number KPIs with short captions",
		itemMin:      4,
		itemMax:      4,
		needsMetrics: true,
	},
	// Also match KPIs without explicit item count
	{
		pattern:   "kpi-3up",
		keywords:  []string{"kpi", "metric", "scorecard"},
		baseScore: 0.75,
		rationale: "3-up KPI layout; specify item_count for better ranking",
	},
	{
		pattern:   "kpi-4up",
		keywords:  []string{"kpi", "metric", "scorecard"},
		baseScore: 0.70,
		rationale: "4-up KPI layout; specify item_count for better ranking",
	},

	// Comparison
	{
		pattern:   "comparison-2col",
		keywords:  []string{"compare", "comparison", "versus", "vs", "pros", "cons", "pro/con", "advantages", "disadvantages", "tradeoff", "trade-off"},
		baseScore: 0.85,
		rationale: "Two-column comparison with optional headers, ideal for pros/cons or A vs B",
		needsCols: 2,
	},
	{
		pattern:   "comparison-2col",
		keywords:  []string{"compare", "comparison", "versus", "vs", "pros", "cons"},
		baseScore: 0.80,
		rationale: "Two-column comparison layout",
	},

	// BMC
	{
		pattern:   "bmc-canvas",
		keywords:  []string{"bmc", "business model canvas", "osterwalder", "value proposition", "key partners", "key activities", "key resources", "customer segments", "revenue streams", "cost structure", "channels", "customer relationships"},
		baseScore: 0.95,
		rationale: "Purpose-built 9-block Business Model Canvas layout",
	},

	// Matrix
	{
		pattern:   "matrix-2x2",
		keywords:  []string{"matrix", "quadrant", "2x2", "positioning", "four quadrant", "eisenhower", "priority matrix", "impact effort", "magic quadrant", "bcg"},
		baseScore: 0.90,
		rationale: "2×2 quadrant matrix with axis labels, ideal for positioning and prioritization",
	},

	// Icon row — features, capabilities, pillars
	{
		pattern:   "icon-row",
		keywords:  []string{"icon", "feature", "pillar", "capability", "benefit", "value", "service", "offering", "step"},
		baseScore: 0.80,
		rationale: "Horizontal row of icon + caption pairs, good for 3-6 features or capabilities",
		itemMin:   2,
		itemMax:   8,
	},
	{
		pattern:   "icon-row",
		keywords:  []string{"icon", "feature", "pillar", "capability", "capabilities", "benefit"},
		baseScore: 0.70,
		rationale: "Icon + caption row for features or capabilities",
	},

	// Card grid — general-purpose grid of titled cards
	{
		pattern:   "card-grid",
		keywords:  []string{"card", "grid", "cards", "tiles", "tile", "panel", "category", "team", "department"},
		baseScore: 0.80,
		rationale: "N×M grid of titled cards, flexible for any content that fits a card layout",
	},
	{
		pattern:   "card-grid",
		keywords:  []string{"overview", "summary", "categories"},
		baseScore: 0.60,
		rationale: "Card grid can organize categories or summary items",
	},

	// Timeline
	{
		pattern:   "timeline-horizontal",
		keywords:  []string{"timeline", "roadmap", "milestone", "phase", "chronological", "history", "evolution", "journey", "schedule"},
		baseScore: 0.90,
		rationale: "Linear horizontal timeline with stops, ideal for roadmaps and milestones",
	},
}

// Recommend scores all rules against the given intent and content hints,
// returning up to maxCandidates results. If no candidate scores above 0.5,
// an empty candidates list is returned.
func Recommend(reg *Registry, intent string, hints *ContentHints, maxCandidates int) RecommendResult {
	if maxCandidates <= 0 {
		maxCandidates = 3
	}
	if hints == nil {
		hints = &ContentHints{}
	}

	intentLower := strings.ToLower(intent)

	// Score each rule.
	type scored struct {
		rule  rule
		score float64
	}
	var candidates []scored

	seen := make(map[string]float64) // track best score per pattern

	for _, r := range rules {
		score := scoreRule(r, intentLower, hints)
		if score < 0.3 {
			continue
		}
		// Only keep the highest-scoring rule per pattern.
		if prev, ok := seen[r.pattern]; ok && prev >= score {
			continue
		}
		seen[r.pattern] = score
		candidates = append(candidates, scored{rule: r, score: score})
	}

	// De-duplicate: keep only the best rule per pattern.
	best := make(map[string]scored)
	for _, c := range candidates {
		if prev, ok := best[c.rule.pattern]; !ok || c.score > prev.score {
			best[c.rule.pattern] = c
		}
	}

	// Flatten and sort by score descending.
	flat := make([]scored, 0, len(best))
	for _, c := range best {
		flat = append(flat, c)
	}
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].score != flat[j].score {
			return flat[i].score > flat[j].score
		}
		return flat[i].rule.pattern < flat[j].rule.pattern
	})

	// Truncate.
	if len(flat) > maxCandidates {
		flat = flat[:maxCandidates]
	}

	// Filter below threshold.
	result := RecommendResult{
		QueryUnderstood: summarizeIntent(intentLower, hints),
	}
	for _, c := range flat {
		if c.score < 0.5 {
			continue
		}
		result.Candidates = append(result.Candidates, Candidate{
			PatternName: c.rule.pattern,
			Score:       math.Round(c.score*100) / 100,
			Rationale:   c.rule.rationale,
		})
	}

	return result
}

// scoreRule computes a 0–1 score for a single rule against the intent and hints.
func scoreRule(r rule, intentLower string, hints *ContentHints) float64 {
	// Keyword match — at least one keyword must appear.
	matched := false
	matchCount := 0
	for _, kw := range r.keywords {
		if strings.Contains(intentLower, kw) {
			matched = true
			matchCount++
		}
	}
	if !matched {
		return 0
	}

	score := r.baseScore

	// Bonus for multiple keyword hits.
	if matchCount > 1 {
		score += 0.05 * float64(min(matchCount-1, 3))
	}

	// Item count matching.
	if hints.ItemCount > 0 {
		if r.itemMin > 0 && hints.ItemCount < r.itemMin {
			score -= 0.3
		}
		if r.itemMax > 0 && hints.ItemCount > r.itemMax {
			score -= 0.3
		}
		// Exact match within range — bonus.
		if r.itemMin > 0 && r.itemMax > 0 && hints.ItemCount >= r.itemMin && hints.ItemCount <= r.itemMax {
			score += 0.05
		}
	}

	// Column hint.
	if hints.Columns > 0 && r.needsCols > 0 {
		if hints.Columns == r.needsCols {
			score += 0.05
		} else {
			score -= 0.15
		}
	}

	// Metrics hint.
	if hints.HasMetrics && r.needsMetrics {
		score += 0.05
	}

	// Clamp.
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// summarizeIntent creates a human-readable echo of how the query was parsed.
func summarizeIntent(intent string, hints *ContentHints) string {
	var sb strings.Builder
	sb.WriteString("intent=\"")
	sb.WriteString(intent)
	sb.WriteString("\"")
	if hints.ItemCount > 0 {
		sb.WriteString(fmt.Sprintf(", item_count=%d", hints.ItemCount))
	}
	if hints.HasMetrics {
		sb.WriteString(", has_metrics=true")
	}
	if hints.HasChart {
		sb.WriteString(", has_chart=true")
	}
	if hints.Columns > 0 {
		sb.WriteString(fmt.Sprintf(", columns=%d", hints.Columns))
	}
	return sb.String()
}
