package patterns

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

var (
	cycleHorizontalRelationRE = regexp.MustCompile(`\b(?:beside|alongside|next to|to the left of|left of|to the right of|right of)\b`)
	cycleColumnPositionRE     = regexp.MustCompile(`\b(?:left|right|narrow|small) (?:hand )?(?:column|half|panel|region|segment|cell)\b`)
)

// narrowCyclePlacement reads placement attached to a cycle clause. A bare
// "half" (half the year), "right" (the right decision), or "narrow" (a
// capability gap) is not geometry. A vertical share is not a width share.
// Explicit full-width placement wins over contextual text elsewhere.
func narrowCyclePlacement(intent string) bool {
	const horizontalMarker = "explicit-horizontal-placement"
	intent = cycleHorizontalRelationRE.ReplaceAllString(intent, " "+horizontalMarker+" , "+horizontalMarker+" ")
	intent = relationRE.ReplaceAllStringFunc(intent, relationPositions)
	clauses := clauseSplitRE.Split(intent, -1)
	// "Two loops beside each other" describes the figure's own lobes,
	// not a separate region. Relations need a distinct neighboring view.
	neighbor := false
	for _, clause := range clauses {
		words := intentWords(clause)
		if strings.Contains(clause, horizontalMarker) && !cycleClause(words) && (intentHasAny(words, textRegionWords...) || intentHasAny(words, "chart", "graph", "table", "photo", "image", "kpi", "kpis")) {
			neighbor = true
		}
	}
	narrow := false
	for _, clause := range clauses {
		words := intentWords(clause)
		if !cycleClause(words) {
			continue
		}
		hasPhrase := func(s string) bool { return intentContainsPhrase(words, intentWords(s)) }
		if hasPhrase("full width") || hasPhrase("whole width") || hasPhrase("entire width") || hasPhrase("full slide") {
			return false
		}
		normalized := strings.Join(words, " ")
		if (neighbor && strings.Contains(clause, horizontalMarker)) || cycleColumnPositionRE.MatchString(normalized) || hasPhrase("half width") {
			narrow = true
		}
		_, vertical := clausePosition(words)
		share := clauseShare(clause)
		if share > 0 && share <= 50 && (intentHasAny(words, "width", "column") || (vertical == "" && intentHasAny(words, "slide", "region", "segment", "cell"))) {
			narrow = true
		}
		if hasPhrase("side by side") && intentHasAny(words, textRegionWords...) {
			narrow = true
		}
	}
	return narrow
}

func cycleClause(words []string) bool {
	return intentHasAny(words, "cycle", "cycles", "loop", "loops", "infinity") || intentContainsPhrase(words, []string{"figure", "eight"})
}

// applyPatternPlacementGuidance runs after semantic/literal scoring so an
// explicit figure-eight name cannot erase its placement caveat. A narrow
// request is a preference mismatch, not proof of failure on an unknown
// template: report the actual minimum and a change of placement or visual.
// Shortlists retain every supplied candidate and never gain another one.
func applyPatternPlacementGuidance(reg *Registry, intent string, hints *VisualHints, candidates []VisualCandidate, shortlist bool) []VisualCandidate {
	if reg == nil {
		return candidates
	}
	narrow := narrowCyclePlacement(intent)
	constrained := false
	for i := range candidates {
		c := &candidates[i]
		annotateCompositionFit(reg, c.Composition)
		if compositionSharesFigureEightWidth(c.Composition, false) {
			qualifyFigureEightPlacement(c, "horizontal composition shares the figure-eight's content width", cfeMinWidthPt, cfeMinHeightPt)
		}
		if c.Category != VisualCategoryPattern {
			continue
		}
		p, ok := reg.Get(c.Name)
		if !ok {
			continue
		}
		c.SegmentFit = recommendationSegmentFit(p)
		if c.SegmentFit == nil {
			continue
		}
		if c.SegmentFit.SegmentOnly {
			c.Rationale += "; supporting band: place in a compose segment or region; the standalone recipe illustrates this visual in isolation"
		}
		if narrow && c.Name == "cycle-figure-eight" {
			constrained = true
			qualifyFigureEightPlacement(c, "requested narrow placement", c.SegmentFit.MinContentWidthPt, c.SegmentFit.MinContentHeightPt)
		}
	}
	if !constrained || (hints.ItemCount != 0 && (hints.ItemCount < 4 || hints.ItemCount > 8)) {
		return candidates
	}
	p, ok := reg.Get("cycle-ring")
	if !ok {
		return candidates
	}
	for i := range candidates {
		if candidates[i].Category == VisualCategoryPattern && candidates[i].Name == p.Name() {
			preferNarrowCycleRing(&candidates[i])
			return candidates
		}
	}
	if !shortlist {
		c := VisualCandidate{Category: VisualCategoryPattern, Name: p.Name(), SegmentFit: recommendationSegmentFit(p)}
		preferNarrowCycleRing(&c)
		candidates = append(candidates, c)
	}
	return candidates
}

func preferNarrowCycleRing(c *VisualCandidate) {
	c.placementConflict = true
	c.Score = 1
	c.ConfidenceBand = confidenceBand(c.Score)
	c.Rationale = fmt.Sprintf("Single-loop alternative for the requested narrow cycle placement, using a legend when needed. It does not preserve the figure-eight's two coupled lobes. To preserve them, give cycle-figure-eight a full-width segment at least %.0fpt wide and %.0fpt high, with text above/below; use expand_pattern to validate the actual content and segment.", cfeMinWidthPt, cfeMinHeightPt)
}

func qualifyFigureEightPlacement(c *VisualCandidate, reason string, width, height float64) {
	c.Score = roundScore(math.Min(c.Score, 0.49))
	c.ConfidenceBand = confidenceBand(c.Score)
	c.placementConflict = true
	c.Rationale += fmt.Sprintf("; %s: this pattern requires at least %.0fpt content width and %.0fpt height. Give it a full-width segment with text above/below to preserve both loops, or use cycle-ring if a single loop is acceptable; validate the actual segment with expand_pattern", reason, width, height)
}

func compositionSharesFigureEightWidth(comp *VisualComposition, shared bool) bool {
	if comp == nil {
		return false
	}
	shared = shared || (comp.Direction == "horizontal" && len(comp.Regions) > 1)
	for _, r := range comp.Regions {
		if shared && r.Category == VisualCategoryPattern && r.Name == "cycle-figure-eight" {
			return true
		}
		if compositionSharesFigureEightWidth(r.Compose, shared) {
			return true
		}
	}
	return false
}

func patternPlacementQuestions(questions []string, candidates []VisualCandidate) []string {
	for _, c := range candidates {
		if c.placementConflict {
			return append(questions, "Must both coupled loops remain? If so, place cycle-figure-eight full-width with other content above/below; choose cycle-ring only if a single loop is acceptable.")
		}
	}
	return questions
}

func annotateCompositionFit(reg *Registry, composition *VisualComposition) {
	if composition == nil {
		return
	}
	for i := range composition.Regions {
		r := &composition.Regions[i]
		if r.Category == VisualCategoryPattern {
			if p, ok := reg.Get(r.Name); ok {
				r.SegmentFit = recommendationSegmentFit(p)
			}
		}
		annotateCompositionFit(reg, r.Compose)
	}
}

// Discovery's full listings explicitly say "none" for unconstrained patterns.
// Recommendation replies only carry fit metadata that changes a placement
// decision; repeating that empty contract on every candidate costs context.
func recommendationSegmentFit(p Pattern) *SegmentFit {
	fit := SegmentFitFor(p)
	if *fit == (SegmentFit{NarrowFallback: "none"}) {
		return nil
	}
	return fit
}
