package patterns

import (
	"fmt"
	"strings"
)

// recommend_pattern is pattern-only (go-slide-creator-m2u2).
//
// Its universe is the named-pattern registry, so an intent whose right answer
// is a chart or a diagram still comes back with the best-scoring PATTERN — and
// came back at "high". "org chart of the leadership team" returned team-bios
// at 0.97 high; "sankey of cost flows" returned process-flow at 0.9 high. A
// high band on a wrong answer is worse than no answer, and nothing in the
// response told the agent the tool could not see charts or diagrams.
//
// Rather than guess at "generic keyword overlap", the recommender asks the
// multi-category scorer the same question. When a chart or diagram scores
// competitively, the pattern's band is capped and the response says where the
// better answer lives.

// beyondPatternMargin is how far below the top pattern a chart/diagram may
// score and still count as competitive. org_chart scores 0.92 against
// team-bios' 0.97 on the same intent, so the margin has to cover that gap;
// anything further behind is a genuine also-ran rather than a missed answer.
const beyondPatternMargin = 0.08

// BeyondPatterns names a non-pattern visual that scored competitively with the
// best pattern for the same intent.
type BeyondPatterns struct {
	Category string  `json:"category"` // "chart" or "diagram"
	Name     string  `json:"name"`     // e.g. "org_chart"
	Score    float64 `json:"score"`
}

// bestNonPatternCandidate returns the highest-scoring chart/diagram for intent,
// or nil when the multi-category scorer offers none.
func bestNonPatternCandidate(reg *Registry, intent string, hints *ContentHints, opts ...*RecommendOptions) *BeyondPatterns {
	visual := RecommendVisual(reg, intent, visualHintsFrom(hints), 8, opts...)
	for _, c := range visual.Candidates {
		if c.Category != VisualCategoryChart && c.Category != VisualCategoryDiagram {
			continue
		}
		return &BeyondPatterns{Category: string(c.Category), Name: c.Name, Score: c.Score}
	}
	return nil
}

// applyBeyondPatterns caps the confidence band and attaches the pointer to
// recommend_visual when a chart or diagram is competitive with the top pattern.
// A result whose best answer really is a pattern is left untouched.
func applyBeyondPatterns(result *RecommendResult, reg *Registry, intent string, hints *ContentHints, opts ...*RecommendOptions) {
	if result == nil || len(result.Candidates) == 0 {
		return
	}
	top := result.Candidates[0]
	best := bestNonPatternCandidate(reg, intent, hints, opts...)
	if best == nil || best.Score < top.Score-beyondPatternMargin {
		return
	}
	// Several frameworks exist as BOTH a pattern and a native diagram —
	// bmc-canvas / business_model_canvas, process-flow / process_flow. Pointing
	// at the diagram there redirects the agent to the same picture under
	// another name, which is noise. Only a genuinely different visual is worth
	// capping a band for.
	if sameVisual(top.PatternName, best.Name) {
		return
	}
	result.BeyondPatterns = best
	// The tool cannot return the thing it just found, so it must not claim
	// high confidence in the thing it can return.
	for i := range result.Candidates {
		if result.Candidates[i].ConfidenceBand == confidenceHigh {
			result.Candidates[i].ConfidenceBand = confidenceMedium
		}
	}
	result.NextToolCall = &ToolCallSuggestion{
		Tool: "recommend_visual",
		ArgsTemplate: map[string]any{
			"intent": intent,
			"reason": fmt.Sprintf("recommend_pattern ranks named patterns only; the %s %q scores %.2f on this intent and recommend_visual can return it", best.Category, best.Name, best.Score),
		},
	}
}

// visualHintsFrom adapts ContentHints to the multi-category scorer's hints so
// both sides judge the same intent with the same context.
func visualHintsFrom(h *ContentHints) *VisualHints {
	if h == nil {
		return nil
	}
	return &VisualHints{ContentHints: *h}
}

// visualAliases maps a pattern name to the diagram name that draws the same
// thing under a different convention, where normalisation alone cannot tell.
var visualAliases = map[string]string{
	"bmc-canvas": "business_model_canvas",
}

// sameVisual reports whether a pattern and a chart/diagram are the same picture.
// Names are compared with separators removed, so process-flow matches
// process_flow and timeline-horizontal matches timeline; the alias table covers
// the pairs whose names share no stem.
func sameVisual(patternName, otherName string) bool {
	p, o := normalizeVisualName(patternName), normalizeVisualName(otherName)
	if p == "" || o == "" {
		return false
	}
	// Checked after the empty guard: a missing alias yields "", which would
	// otherwise match an empty otherName.
	if visualAliases[patternName] == otherName {
		return true
	}
	return strings.Contains(p, o) || strings.Contains(o, p)
}

// normalizeVisualName strips the separators that differ between the pattern and
// diagram naming conventions.
func normalizeVisualName(s string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(s))
}
