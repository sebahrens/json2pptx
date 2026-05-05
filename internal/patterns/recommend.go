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

// RecommendOptions carries diversity and context parameters for Recommend.
type RecommendOptions struct {
	RecentPatterns []string `json:"recent_patterns,omitempty"` // patterns used on preceding slides
	PreferVariety  bool     `json:"prefer_variety,omitempty"`  // apply recency decay penalty
	SlideIndex     int      `json:"slide_index,omitempty"`     // 0-based position of the slide being built
}

// Candidate is a single recommendation result.
type Candidate struct {
	PatternName     string  `json:"pattern_name"`
	Score           float64 `json:"score"`
	Rationale       string  `json:"rationale"`
	ConfidenceBand  string  `json:"confidence_band,omitempty"`  // high, medium, low
	DiversityBonus  bool    `json:"diversity_bonus,omitempty"`  // true if boosted for variety
}

// NearMiss describes a candidate that nearly qualified but was edged out.
type NearMiss struct {
	PatternName string `json:"pattern_name"`
	Score       float64 `json:"score"`
	WouldTipIf  string `json:"would_tip_if"`
}

// RecommendResult is the output of Recommend.
type RecommendResult struct {
	Candidates              []Candidate `json:"candidates"`
	QueryUnderstood         string      `json:"query_understood_as"`
	NearMisses              []NearMiss  `json:"near_misses,omitempty"`
	DisambiguatingQuestions []string    `json:"disambiguating_questions,omitempty"`
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
		itemMin:   9,
		itemMax:   9,
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
		keywords:  []string{"timeline", "milestone", "chronological", "history", "evolution", "journey", "schedule"},
		baseScore: 0.90,
		rationale: "Linear horizontal timeline with stops, ideal for roadmaps and milestones",
	},

	// Pyramid
	{
		pattern:   "pyramid",
		keywords:  []string{"pyramid", "hierarchy", "layer", "maslow", "tiered", "funnel", "levels"},
		baseScore: 0.90,
		rationale: "Stacked trapezoid pyramid for hierarchies or layered models",
		itemMin:   3,
		itemMax:   5,
	},
	{
		pattern:   "pyramid",
		keywords:  []string{"pyramid", "hierarchy", "tiered"},
		baseScore: 0.75,
		rationale: "Pyramid hierarchy; specify item_count for better ranking",
	},

	// Before/After
	{
		pattern:   "before-after",
		keywords:  []string{"before", "after", "current state", "future state", "from-to", "transformation", "as-is", "to-be"},
		baseScore: 0.90,
		rationale: "Two-column before/after with transition chevron, ideal for state transformations",
		needsCols: 2,
	},
	{
		pattern:   "before-after",
		keywords:  []string{"before", "after", "transformation"},
		baseScore: 0.80,
		rationale: "Before/after comparison layout",
	},

	// Process flow
	{
		pattern:   "process-flow",
		keywords:  []string{"process", "flow", "workflow", "decision", "flowchart", "procedure", "step-by-step", "pipeline"},
		baseScore: 0.90,
		rationale: "Left-to-right process flow with steps and decision diamonds",
		itemMin:   3,
		itemMax:   8,
	},
	{
		pattern:   "process-flow",
		keywords:  []string{"process", "workflow", "flowchart"},
		baseScore: 0.75,
		rationale: "Process flow diagram",
	},

	// Swimlane
	{
		pattern:   "swimlane",
		keywords:  []string{"swimlane", "swim lane", "cross-functional", "raci", "handoff", "handover", "actors", "responsibility"},
		baseScore: 0.90,
		rationale: "Horizontal swimlane diagram with actors and process steps per lane",
	},
	{
		pattern:   "swimlane",
		keywords:  []string{"swimlane", "cross-functional"},
		baseScore: 0.75,
		rationale: "Cross-functional swimlane process diagram",
	},

	// Roadmap phased
	{
		pattern:   "roadmap-phased",
		keywords:  []string{"phased", "quarter", "quarterly", "release plan", "workstream", "multi-phase", "gantt"},
		baseScore: 0.92,
		rationale: "Phased roadmap with workstreams across time periods, ideal for quarterly planning",
	},
	{
		pattern:   "roadmap-phased",
		keywords:  []string{"roadmap phased", "phased roadmap", "quarterly roadmap"},
		baseScore: 0.85,
		rationale: "Multi-phase roadmap with workstreams",
	},

	// Architecture stack
	{
		pattern:   "arch-stack",
		keywords:  []string{"architecture", "stack", "layer", "tier", "infrastructure", "tech stack", "platform"},
		baseScore: 0.90,
		rationale: "Architecture stack diagram with labeled tiers and optional cross-cutting side rails",
	},
	{
		pattern:   "arch-stack",
		keywords:  []string{"architecture", "stack"},
		baseScore: 0.75,
		rationale: "Layered architecture stack diagram",
	},

	// Stat hero — single big number emphasis
	{
		pattern:      "stat-hero",
		keywords:     []string{"hero", "big number", "headline", "stat", "single metric", "tam", "revenue", "total", "highlight"},
		baseScore:    0.90,
		rationale:    "Full-bleed single oversized number with one-line context, for maximum emphasis",
		itemMax:      1,
		needsMetrics: true,
	},
	{
		pattern:   "stat-hero",
		keywords:  []string{"hero", "emphasis", "single", "highlight", "one number"},
		baseScore: 0.75,
		rationale: "Single-stat hero slide for maximum visual impact",
	},

	// Pull quote — testimonial or emphasis quote
	{
		pattern:   "pull-quote",
		keywords:  []string{"quote", "testimonial", "pull-quote", "pullquote", "citation", "endorsement", "customer voice", "executive quote"},
		baseScore: 0.92,
		rationale: "Italic quote block with attribution, ideal for testimonials or key takeaways",
	},
	{
		pattern:   "pull-quote",
		keywords:  []string{"quote", "testimonial"},
		baseScore: 0.80,
		rationale: "Pull-quote block with attribution line",
	},
}

// Recommend scores all rules against the given intent and content hints,
// returning up to maxCandidates results. If no candidate scores above 0.5,
// an empty candidates list is returned.
//
// opts may be nil. When opts.PreferVariety is true, patterns appearing in
// opts.RecentPatterns receive a decay penalty (each occurrence reduces the
// score by 0.15, cumulative) and a diversity bonus candidate (the strongest
// unused pattern) is injected if one exists.
func Recommend(reg *Registry, intent string, hints *ContentHints, maxCandidates int, opts ...*RecommendOptions) RecommendResult {
	if maxCandidates <= 0 {
		maxCandidates = 3
	}
	if hints == nil {
		hints = &ContentHints{}
	}
	var options *RecommendOptions
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	intentLower := strings.ToLower(intent)

	// Build recency count map.
	recencyCount := make(map[string]int)
	if options != nil {
		for _, p := range options.RecentPatterns {
			recencyCount[p]++
		}
	}

	applyVariety := options != nil && options.PreferVariety && len(recencyCount) > 0

	// Score and deduplicate rules.
	flat := scoreAndDedup(intentLower, hints)

	// Apply recency decay penalty when variety is requested.
	if applyVariety {
		applyRecencyDecay(flat, recencyCount)
	}

	// Sort by score descending.
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].score != flat[j].score {
			return flat[i].score > flat[j].score
		}
		return flat[i].rule.pattern < flat[j].rule.pattern
	})

	// Collect near misses before truncation.
	nearMisses := collectNearMisses(flat, hints)

	// Find diversity bonus candidate.
	var diversityCandidate *scored
	if applyVariety {
		diversityCandidate = findDiversityCandidate(flat, recencyCount)
	}

	// Truncate and filter.
	if len(flat) > maxCandidates {
		flat = flat[:maxCandidates]
	}

	result := RecommendResult{
		QueryUnderstood: summarizeIntent(intentLower, hints),
	}
	for _, c := range flat {
		if c.score < 0.5 {
			continue
		}
		result.Candidates = append(result.Candidates, Candidate{
			PatternName:    c.rule.pattern,
			Score:          math.Round(c.score*100) / 100,
			Rationale:      c.rule.rationale,
			ConfidenceBand: confidenceBand(c.score),
			DiversityBonus: c.diversityBonus,
		})
	}

	// Inject diversity bonus if not already present.
	injectDiversityBonus(&result, diversityCandidate)

	if len(nearMisses) > 2 {
		nearMisses = nearMisses[:2]
	}
	result.NearMisses = nearMisses
	result.DisambiguatingQuestions = suggestDisambiguatingQuestions(hints, intentLower, result.Candidates)

	return result
}

// scored is a candidate with its computed score.
type scored struct {
	rule           rule
	score          float64
	preDecayScore  float64
	diversityBonus bool
}

// scoreAndDedup evaluates all rules and returns the best-scoring entry per pattern.
func scoreAndDedup(intentLower string, hints *ContentHints) []scored {
	seen := make(map[string]float64)
	var candidates []scored

	for _, r := range rules {
		score := scoreRule(r, intentLower, hints)
		if score < 0.3 {
			continue
		}
		if prev, ok := seen[r.pattern]; ok && prev >= score {
			continue
		}
		seen[r.pattern] = score
		candidates = append(candidates, scored{rule: r, score: score})
	}

	best := make(map[string]scored)
	for _, c := range candidates {
		if prev, ok := best[c.rule.pattern]; !ok || c.score > prev.score {
			best[c.rule.pattern] = c
		}
	}

	flat := make([]scored, 0, len(best))
	for _, c := range best {
		flat = append(flat, c)
	}
	return flat
}

// applyRecencyDecay penalizes patterns that appear in the recency map.
func applyRecencyDecay(flat []scored, recencyCount map[string]int) {
	for i := range flat {
		flat[i].preDecayScore = flat[i].score
		if count, ok := recencyCount[flat[i].rule.pattern]; ok && count > 0 {
			penalty := 0.15 * float64(count)
			flat[i].score -= penalty
			if flat[i].score < 0 {
				flat[i].score = 0
			}
		}
	}
}

// collectNearMisses finds patterns scoring between 0.35 and 0.5.
func collectNearMisses(flat []scored, hints *ContentHints) []NearMiss {
	var nearMisses []NearMiss
	for _, c := range flat {
		if c.score >= 0.35 && c.score < 0.5 {
			nearMisses = append(nearMisses, NearMiss{
				PatternName: c.rule.pattern,
				Score:       math.Round(c.score*100) / 100,
				WouldTipIf:  inferWouldTipIf(c.rule, hints),
			})
		}
	}
	return nearMisses
}

// findDiversityCandidate returns the highest-scoring pattern not in recencyCount.
func findDiversityCandidate(flat []scored, recencyCount map[string]int) *scored {
	for i := range flat {
		if _, used := recencyCount[flat[i].rule.pattern]; !used && flat[i].score >= 0.5 {
			c := flat[i]
			c.diversityBonus = true
			return &c
		}
	}
	return nil
}

// injectDiversityBonus adds the diversity candidate if not already in results.
func injectDiversityBonus(result *RecommendResult, dc *scored) {
	if dc == nil {
		return
	}
	for _, c := range result.Candidates {
		if c.PatternName == dc.rule.pattern {
			return
		}
	}
	result.Candidates = append(result.Candidates, Candidate{
		PatternName:    dc.rule.pattern,
		Score:          math.Round(dc.score*100) / 100,
		Rationale:      dc.rule.rationale + " (diversity bonus: unused in recent slides)",
		ConfidenceBand: confidenceBand(dc.score),
		DiversityBonus: true,
	})
}

// confidenceBand classifies a score into high/medium/low.
func confidenceBand(score float64) string {
	switch {
	case score >= 0.85:
		return "high"
	case score >= 0.65:
		return "medium"
	default:
		return "low"
	}
}

// inferWouldTipIf describes what change would push a near-miss over the threshold.
func inferWouldTipIf(r rule, hints *ContentHints) string {
	if r.needsMetrics && !hints.HasMetrics {
		return "provide has_metrics=true in content_hints"
	}
	if r.needsCols > 0 && hints.Columns == 0 {
		return fmt.Sprintf("provide columns=%d in content_hints", r.needsCols)
	}
	if r.itemMin > 0 && hints.ItemCount == 0 {
		return fmt.Sprintf("provide item_count (this pattern works best with %d-%d items)", r.itemMin, r.itemMax)
	}
	return "rephrase intent with more specific keywords"
}

// suggestDisambiguatingQuestions returns questions the agent could answer to
// refine the recommendation.
func suggestDisambiguatingQuestions(hints *ContentHints, intent string, candidates []Candidate) []string {
	var qs []string

	if hints.ItemCount == 0 {
		qs = append(qs, "How many items will this slide contain?")
	}
	if !hints.HasMetrics {
		// Only ask if metrics-oriented patterns are plausible.
		for _, kw := range []string{"kpi", "metric", "number", "stat", "dashboard"} {
			if strings.Contains(intent, kw) {
				qs = append(qs, "Does this slide contain numeric metrics or KPIs?")
				break
			}
		}
	}
	if !hints.HasChart {
		for _, kw := range []string{"chart", "graph", "visualization", "trend"} {
			if strings.Contains(intent, kw) {
				qs = append(qs, "Does this slide include a chart or graph?")
				break
			}
		}
	}

	// If there are multiple close candidates, suggest clarifying.
	if len(candidates) >= 2 {
		gap := candidates[0].Score - candidates[1].Score
		if gap < 0.1 {
			qs = append(qs, fmt.Sprintf("Did you mean more of a %s or a %s layout?",
				candidates[0].PatternName, candidates[1].PatternName))
		}
	}

	if len(qs) > 3 {
		qs = qs[:3]
	}
	return qs
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

// SuggestSwap finds registered patterns whose item-count constraints accept
// itemCount, excluding currentPattern. It returns swap suggestions sorted by
// specificity (patterns with tighter itemMin/itemMax first). This is the
// "reverse recommender" used by validators to attach swap_pattern fixes.
func SuggestSwap(reg *Registry, currentPattern string, itemCount int, hasMetrics bool) []SwapSuggestion {
	if reg == nil || itemCount <= 0 {
		return nil
	}

	type candidate struct {
		pattern   string
		rationale string
		specificity int // smaller range → more specific
	}

	seen := make(map[string]candidate)

	for _, r := range rules {
		if r.pattern == currentPattern {
			continue
		}
		// Must be registered.
		if _, ok := reg.Get(r.pattern); !ok {
			continue
		}
		// Only consider rules that have item-count constraints — unconstrained
		// rules are too broad for reverse-recommendation.
		if r.itemMin == 0 && r.itemMax == 0 {
			continue
		}
		// Must accept this item count.
		if r.itemMin > 0 && itemCount < r.itemMin {
			continue
		}
		if r.itemMax > 0 && itemCount > r.itemMax {
			continue
		}
		// If needsMetrics and user has no metrics, skip.
		if r.needsMetrics && !hasMetrics {
			continue
		}
		// Specificity: tighter range = better match.
		spec := 1000 // unconstrained
		if r.itemMin > 0 && r.itemMax > 0 {
			spec = r.itemMax - r.itemMin
		} else if r.itemMax > 0 {
			spec = r.itemMax
		} else if r.itemMin > 0 {
			spec = 100 - r.itemMin
		}

		// Keep the most specific rule per pattern.
		if prev, ok := seen[r.pattern]; ok && prev.specificity <= spec {
			continue
		}
		seen[r.pattern] = candidate{
			pattern:     r.pattern,
			rationale:   r.rationale,
			specificity: spec,
		}
	}

	// Sort by specificity ascending (most specific first).
	flat := make([]candidate, 0, len(seen))
	for _, c := range seen {
		flat = append(flat, c)
	}
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].specificity != flat[j].specificity {
			return flat[i].specificity < flat[j].specificity
		}
		return flat[i].pattern < flat[j].pattern
	})

	// Build suggestions with field mapping hints.
	out := make([]SwapSuggestion, len(flat))
	for i, c := range flat {
		out[i] = SwapSuggestion{
			From:         currentPattern,
			To:           c.pattern,
			Rationale:    c.rationale,
			FieldMapping: fieldMappingHint(currentPattern, c.pattern),
		}
	}
	return out
}

// fieldMappingHint returns a best-effort mapping from the source pattern's
// fields to the target pattern's fields. Returns nil when the patterns share
// the same schema shape (e.g. both use cells[].header/body).
func fieldMappingHint(from, to string) map[string]string {
	// KPI ↔ KPI: identical schema.
	kpiPatterns := map[string]bool{"kpi-3up": true, "kpi-4up": true}
	if kpiPatterns[from] && kpiPatterns[to] {
		return nil // same schema
	}

	// Card-grid → BMC: cells are compatible (both header+body).
	cardLike := map[string]bool{"card-grid": true, "bmc-canvas": true}
	if cardLike[from] && cardLike[to] {
		return map[string]string{
			"cells[].header": "cells[].header",
			"cells[].body":   "cells[].body",
		}
	}

	// Card-grid/BMC → KPI: header→small, body→(drop), need big.
	if cardLike[from] && kpiPatterns[to] {
		return map[string]string{
			"cells[].header": "values[].small",
			"(new)":          "values[].big (required, provide metric value)",
		}
	}

	// KPI → card-grid/BMC: big→header, small→body.
	if kpiPatterns[from] && cardLike[to] {
		return map[string]string{
			"values[].big":   "cells[].header",
			"values[].small": "cells[].body",
		}
	}

	return nil
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

