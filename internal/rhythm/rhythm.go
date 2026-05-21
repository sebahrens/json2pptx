// Package rhythm performs static "visual rhythm" analysis on a deck: pattern
// repetition, density variation, accent usage, and actionable recommendations
// for breaking repetitive runs.
//
// It operates on a minimal, package-local Slide model so it stays decoupled
// from the JSON input schema. Callers project their own slide types into
// []Slide (see the adapter in cmd/json2pptx) before calling Analyze. The same
// engine backs both the analyze_deck_rhythm MCP tool and the composition axis
// of score_deck.
package rhythm

import (
	"fmt"
	"math"
	"sort"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
)

// Slide is the minimal per-slide projection the analyzer needs. Callers build
// it from their own input types; the analyzer never sees the full JSON schema.
type Slide struct {
	// SlideType is the optional slide_type hint (e.g. "title", "content").
	SlideType string
	// PatternName is the named pattern, or "" when the slide has no named pattern.
	PatternName string
	// HasPattern reports whether the slide carries a pattern block (even one
	// without a name).
	HasPattern bool
	// HasShapeGrid reports whether the slide carries a shape_grid.
	HasShapeGrid bool
	// HasCompose reports whether the slide uses the compose envelope.
	HasCompose bool
	// ContentKinds lists the content item types in document order
	// (e.g. "text", "bullets", "table").
	ContentKinds []string
	// CellAccents lists accent color references (accent1..accent6) found in
	// shape_grid cells, in iteration order. Drives primary-accent and
	// accent-variety analysis.
	CellAccents []string
	// CellCount is the total number of shape_grid cells on the slide.
	CellCount int
	// Grid is a resolution-ready shape_grid for density measurement, or nil to
	// skip the slide in density-distribution accounting.
	Grid *shapegrid.Grid
}

// SlideInfo describes the visual fingerprint of a single slide.
type SlideInfo struct {
	SlideIndex               int    `json:"slide_index"`
	Pattern                  string `json:"pattern"`                     // pattern name, "shape_grid", "content", or slide_type
	DensityClass             string `json:"density_class"`               // "low", "med", "high"
	AccentRole               string `json:"accent_role"`                 // primary accent color used, or "none"
	DominantVisual           string `json:"dominant_visual"`             // "chart", "diagram", "table", "text", "grid", "pattern"
	WithinSlideAccentVariety int    `json:"within_slide_accent_variety"` // distinct accent slots used across cells
	cellCount                int    // internal: total cells in shape_grid (not serialized)
}

// PatternRun describes a consecutive run of the same pattern.
type PatternRun struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	Len   int    `json:"len"`
}

// AccentBalance maps accent role names to their usage fraction (0.0–1.0).
type AccentBalance map[string]float64

// DensityDistribution counts cells across the deck by textcapacity status.
type DensityDistribution struct {
	UnderfilledCells int `json:"underfilled_cells"`
	OptimalCells     int `json:"optimal_cells"`
	OverflowCells    int `json:"overflow_cells"`
}

// Aggregates holds deck-level aggregate metrics.
type Aggregates struct {
	PatternRuns         []PatternRun        `json:"pattern_runs"`
	LongestRun          int                 `json:"longest_run"`
	RepetitionIndex     float64             `json:"repetition_index"` // 0.0 (all unique) to 1.0 (all same)
	AccentBalance       AccentBalance       `json:"accent_balance"`
	DensityCV           float64             `json:"density_cv"` // coefficient of variation of density scores
	DensityDistribution DensityDistribution `json:"density_distribution"`
}

// Recommendation is an actionable suggestion to improve deck rhythm.
type Recommendation struct {
	SlideIndex       int      `json:"slide_index"`
	Message          string   `json:"message"`
	RecommendedBreak []string `json:"recommended_break_patterns"`
}

// Result is the top-level rhythm-analysis output. Its JSON shape is the
// analyze_deck_rhythm MCP response.
type Result struct {
	PerSlide         []SlideInfo      `json:"per_slide"`
	Aggregates       Aggregates       `json:"aggregates"`
	Recommendations  []Recommendation `json:"recommendations"`
	CompositionScore int              `json:"composition_score"` // 0–100
}

// Analyze performs the core rhythm analysis on the slide projections.
func Analyze(slides []Slide) *Result {
	perSlide := make([]SlideInfo, len(slides))

	for i, s := range slides {
		perSlide[i] = fingerprint(i, s)
	}

	runs := detectPatternRuns(perSlide)
	longestRun := 0
	for _, r := range runs {
		if r.Len > longestRun {
			longestRun = r.Len
		}
	}

	dd := computeDensityDistribution(slides)

	result := &Result{
		PerSlide: perSlide,
		Aggregates: Aggregates{
			PatternRuns:         runs,
			LongestRun:          longestRun,
			RepetitionIndex:     computeRepetitionIndex(perSlide),
			AccentBalance:       computeAccentBalance(slides),
			DensityCV:           computeDensityCV(perSlide),
			DensityDistribution: dd,
		},
	}

	result.Recommendations = generateRecommendations(perSlide, runs, dd)
	result.CompositionScore = computeCompositionScore(result)

	return result
}

// fingerprint extracts the visual fingerprint of a single slide.
func fingerprint(idx int, s Slide) SlideInfo {
	info := SlideInfo{
		SlideIndex: idx,
	}

	// Determine pattern name.
	switch {
	case s.HasCompose:
		info.Pattern = "compose"
	case s.PatternName != "":
		info.Pattern = s.PatternName
	case s.HasShapeGrid:
		info.Pattern = "shape_grid"
	default:
		// Use slide_type if available, else infer from content.
		if s.SlideType != "" {
			info.Pattern = s.SlideType
		} else {
			info.Pattern = "content"
		}
	}

	info.DominantVisual = dominantVisual(s)
	info.DensityClass = densityClass(s)
	info.AccentRole = primaryAccent(s)
	info.WithinSlideAccentVariety = countDistinctAccents(s)
	info.cellCount = s.CellCount

	return info
}

// dominantVisual returns the primary visual type on a slide.
func dominantVisual(s Slide) string {
	if s.HasCompose {
		return "compose"
	}
	if s.HasPattern {
		return "pattern"
	}
	if s.HasShapeGrid {
		return "grid"
	}
	for _, kind := range s.ContentKinds {
		switch kind {
		case "chart":
			return "chart"
		case "diagram":
			return "diagram"
		case "table":
			return "table"
		case "image":
			return "image"
		}
	}
	return "text"
}

// densityClass estimates content density from the slide projection.
func densityClass(s Slide) string {
	weight := 0

	for _, kind := range s.ContentKinds {
		switch kind {
		case "text":
			weight += 1
		case "bullets":
			weight += 2
		case "body_and_bullets", "bullet_groups":
			weight += 3
		case "table":
			weight += 4
		case "chart", "diagram":
			weight += 3
		case "image":
			weight += 2
		default:
			weight += 1
		}
	}

	if s.HasShapeGrid {
		weight += 4
	}
	if s.HasPattern {
		weight += 3
	}
	if s.HasCompose {
		weight += 5
	}

	switch {
	case weight <= 2:
		return "low"
	case weight <= 5:
		return "med"
	default:
		return "high"
	}
}

// primaryAccent returns the first accent color reference on a slide, or "none".
func primaryAccent(s Slide) string {
	if len(s.CellAccents) > 0 {
		return s.CellAccents[0]
	}
	return "none"
}

// countDistinctAccents counts unique accent color slots across a slide's cells.
func countDistinctAccents(s Slide) int {
	if len(s.CellAccents) == 0 {
		return 0
	}
	seen := map[string]bool{}
	for _, a := range s.CellAccents {
		seen[a] = true
	}
	return len(seen)
}

// detectPatternRuns finds consecutive runs of the same pattern.
func detectPatternRuns(slides []SlideInfo) []PatternRun {
	if len(slides) == 0 {
		return nil
	}

	var runs []PatternRun
	current := PatternRun{Name: slides[0].Pattern, Start: 0, Len: 1}

	for i := 1; i < len(slides); i++ {
		if slides[i].Pattern == current.Name {
			current.Len++
		} else {
			if current.Len >= 2 {
				runs = append(runs, current)
			}
			current = PatternRun{Name: slides[i].Pattern, Start: i, Len: 1}
		}
	}
	if current.Len >= 2 {
		runs = append(runs, current)
	}

	return runs
}

// computeRepetitionIndex measures how repetitive the pattern sequence is.
// 0.0 = all unique patterns, 1.0 = all the same pattern.
func computeRepetitionIndex(slides []SlideInfo) float64 {
	if len(slides) <= 1 {
		return 0.0
	}

	unique := map[string]bool{}
	for _, s := range slides {
		unique[s.Pattern] = true
	}

	// repetition_index = 1 - (unique_count / total_count)
	ri := 1.0 - float64(len(unique))/float64(len(slides))
	return math.Round(ri*100) / 100
}

// computeAccentBalance returns the fraction of slides using each accent.
func computeAccentBalance(slides []Slide) AccentBalance {
	counts := map[string]int{}
	total := 0
	for _, s := range slides {
		role := primaryAccent(s)
		if role != "none" {
			counts[role]++
			total++
		}
	}

	if total == 0 {
		return AccentBalance{}
	}

	balance := AccentBalance{}
	for accent, count := range counts {
		balance[accent] = math.Round(float64(count)/float64(total)*100) / 100
	}
	return balance
}

// computeDensityCV calculates the coefficient of variation of density scores.
func computeDensityCV(slides []SlideInfo) float64 {
	if len(slides) <= 1 {
		return 0.0
	}

	// Map density_class to numeric for CV calculation.
	classToScore := map[string]float64{"low": 1, "med": 2, "high": 3}

	sum := 0.0
	for _, s := range slides {
		sum += classToScore[s.DensityClass]
	}
	mean := sum / float64(len(slides))
	if mean == 0 {
		return 0.0
	}

	variance := 0.0
	for _, s := range slides {
		diff := classToScore[s.DensityClass] - mean
		variance += diff * diff
	}
	variance /= float64(len(slides))
	stddev := math.Sqrt(variance)

	cv := stddev / mean
	return math.Round(cv*100) / 100
}

// computeDensityDistribution resolves each slide's grid (if any) and tallies
// cells by textcapacity status. Slides without a grid are skipped, as are
// grids that fail validation or resolution.
func computeDensityDistribution(slides []Slide) DensityDistribution {
	var dd DensityDistribution
	alloc := pptx.NewShapeIDAllocator(nil)

	for _, s := range slides {
		if s.Grid == nil {
			continue
		}

		if vErr := shapegrid.Validate(s.Grid); vErr != nil {
			continue
		}

		result, err := shapegrid.Resolve(s.Grid, alloc)
		if err != nil || result == nil {
			continue
		}

		densities := textcapacity.ForResolvedGrid(result)
		for _, d := range densities {
			switch d.Status {
			case textcapacity.StatusUnderfilled:
				dd.UnderfilledCells++
			case textcapacity.StatusOptimal:
				dd.OptimalCells++
			case textcapacity.StatusOverflow:
				dd.OverflowCells++
			}
		}
	}
	return dd
}

// generateRecommendations produces actionable suggestions for runs of 3+,
// low accent variety, and density distribution imbalance.
func generateRecommendations(slides []SlideInfo, runs []PatternRun, dd DensityDistribution) []Recommendation {
	var recs []Recommendation

	for _, run := range runs {
		if run.Len < 3 {
			continue
		}

		// Recommend break points at every 3rd slide in the run.
		breakPatterns := suggestBreakPatterns(run.Name)

		for offset := 2; offset < run.Len; offset += 3 {
			insertIdx := run.Start + offset
			recs = append(recs, Recommendation{
				SlideIndex:       insertIdx,
				Message:          fmt.Sprintf("break a %s run (length %d); consider inserting a different pattern at slide %d", run.Name, run.Len, insertIdx),
				RecommendedBreak: breakPatterns,
			})
		}
	}

	// Rule: slide has 5+ cells AND within_slide_accent_variety == 1 → recommend progressive accent.
	for _, s := range slides {
		if s.cellCount >= 5 && s.WithinSlideAccentVariety == 1 {
			recs = append(recs, Recommendation{
				SlideIndex:       s.SlideIndex,
				Message:          fmt.Sprintf("slide %d has %d cells but only 1 accent — add cell_accent_mode: progressive to the pattern overrides for visual hierarchy", s.SlideIndex, s.cellCount),
				RecommendedBreak: []string{"cell_accent_mode: progressive"},
			})
		}
	}

	// Rule: >30% underfilled cells across the deck → recommend adding detail or smaller grids.
	totalCells := dd.UnderfilledCells + dd.OptimalCells + dd.OverflowCells
	if totalCells > 0 {
		underfilledPct := float64(dd.UnderfilledCells) / float64(totalCells) * 100
		if underfilledPct > 30 {
			recs = append(recs, Recommendation{
				SlideIndex:       -1, // deck-level
				Message:          fmt.Sprintf("%.0f%% of cells (%d/%d) are underfilled — add detail text or use smaller grid patterns", underfilledPct, dd.UnderfilledCells, totalCells),
				RecommendedBreak: []string{"kpi-3up", "kpi-2up", "comparison-2col"},
			})
		}
	}

	return recs
}

// suggestBreakPatterns returns patterns that contrast well with the given pattern.
func suggestBreakPatterns(current string) []string {
	// Categorize patterns into visual "families" and recommend from other families.
	gridFamily := map[string]bool{
		"card-grid": true, "kpi-3up": true, "kpi-4up": true, "icon-row": true,
	}
	narrativeFamily := map[string]bool{
		"stat-hero": true, "pull-quote": true,
	}
	structureFamily := map[string]bool{
		"timeline-horizontal": true, "process-flow": true, "roadmap-phased": true,
		"swimlane": true, "agenda": true,
	}
	matrixFamily := map[string]bool{
		"bmc-canvas": true, "matrix-2x2": true, "comparison-2col": true,
		"arch-stack": true, "pyramid": true, "before-after": true,
		"strategy-house": true,
	}

	families := []map[string]bool{gridFamily, narrativeFamily, structureFamily, matrixFamily}

	// Find which family the current pattern belongs to.
	currentFamily := -1
	for i, fam := range families {
		if fam[current] {
			currentFamily = i
			break
		}
	}

	// Collect candidates from other families.
	var candidates []string
	for i, fam := range families {
		if i == currentFamily {
			continue
		}
		for p := range fam {
			candidates = append(candidates, p)
		}
	}

	sort.Strings(candidates)

	// Return up to 3 suggestions.
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates
}

// computeCompositionScore produces a 0–100 score reflecting deck composition quality.
func computeCompositionScore(r *Result) int {
	score := 100.0

	// Penalize long runs: -10 per run of 3+, -5 extra per additional slide beyond 3.
	for _, run := range r.Aggregates.PatternRuns {
		if run.Len >= 3 {
			score -= 10
			score -= float64(run.Len-3) * 5
		}
	}

	// Penalize high repetition index.
	if r.Aggregates.RepetitionIndex > 0.7 {
		score -= 20
	} else if r.Aggregates.RepetitionIndex > 0.5 {
		score -= 10
	}

	// Penalize very low density variation (all slides same density = monotonous).
	if r.Aggregates.DensityCV < 0.1 && len(r.PerSlide) > 3 {
		score -= 10
	}

	// Penalize accent imbalance — if one accent dominates >80%.
	for _, frac := range r.Aggregates.AccentBalance {
		if frac > 0.8 && len(r.Aggregates.AccentBalance) > 1 {
			score -= 10
			break
		}
	}

	if score < 0 {
		score = 0
	}
	return int(math.Round(score))
}
