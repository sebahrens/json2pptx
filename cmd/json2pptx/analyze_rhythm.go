package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
)

// --- Tool definition ---

func mcpAnalyzeDeckRhythmTool() mcp.Tool {
	return mcp.NewTool("analyze_deck_rhythm",
		mcp.WithDescription(`Analyze a presentation's visual rhythm — pattern repetition, density variation, and accent usage across slides.

Use this BEFORE calling generate_presentation to detect monotony and inform pattern choices. Unlike score_deck (which requires a full generation pass), this tool performs lightweight static analysis on the JSON input.

Returns per-slide fingerprints, pattern run detection, a density coefficient of variation, accent balance, and actionable recommendations for breaking repetitive runs.`),
		mcp.WithRawOutputSchema(outputSchemaAnalyzeDeckRhythm),
		mcp.WithObject("presentation",
			mcp.Required(),
			mcp.Description("Presentation definition. Same schema as generate_presentation."),
			mcp.Properties(map[string]any{
				"template": map[string]any{"type": "string", "description": "Template name"},
				"slides":   map[string]any{"type": "array", "description": "Array of slide definitions", "items": map[string]any{"type": "object"}},
			}),
		),
	)
}

// --- Response types ---

// rhythmSlideInfo describes the visual fingerprint of a single slide.
type rhythmSlideInfo struct {
	SlideIndex    int    `json:"slide_index"`
	Pattern       string `json:"pattern"`        // pattern name, "shape_grid", "content", or slide_type
	DensityClass  string `json:"density_class"`  // "low", "med", "high"
	AccentRole    string `json:"accent_role"`     // primary accent color used, or "none"
	DominantVisual string `json:"dominant_visual"` // "chart", "diagram", "table", "text", "grid", "pattern"
}

// patternRun describes a consecutive run of the same pattern.
type patternRun struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	Len   int    `json:"len"`
}

// accentBalance maps accent role names to their usage fraction (0.0–1.0).
type accentBalance map[string]float64

// rhythmAggregates holds deck-level aggregate metrics.
type rhythmAggregates struct {
	PatternRuns     []patternRun   `json:"pattern_runs"`
	LongestRun      int            `json:"longest_run"`
	RepetitionIndex float64        `json:"repetition_index"` // 0.0 (all unique) to 1.0 (all same)
	AccentBalance   accentBalance  `json:"accent_balance"`
	DensityCV       float64        `json:"density_cv"` // coefficient of variation of density scores
}

// rhythmRecommendation is an actionable suggestion to improve deck rhythm.
type rhythmRecommendation struct {
	SlideIndex          int      `json:"slide_index"`
	Message             string   `json:"message"`
	RecommendedBreak    []string `json:"recommended_break_patterns"`
}

// rhythmResult is the top-level analyze_deck_rhythm response.
type rhythmResult struct {
	PerSlide        []rhythmSlideInfo      `json:"per_slide"`
	Aggregates      rhythmAggregates       `json:"aggregates"`
	Recommendations []rhythmRecommendation `json:"recommendations"`
	CompositionScore int                   `json:"composition_score"` // 0–100
}

// --- Handler ---

func handleAnalyzeDeckRhythm(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, paramErr := objectParamAsJSON(request, "presentation")
	if paramErr != nil {
		return paramErr, nil
	}
	if jsonStr == "" {
		return api.MCPSimpleError("MISSING_PARAMETER", "presentation is required"), nil
	}

	var input PresentationInput
	if err := strictUnmarshalJSON([]byte(jsonStr), &input); err != nil {
		return mcpParseError("INVALID_JSON", "presentation", fmt.Sprintf("invalid JSON: %v", err)), nil
	}

	if len(input.Slides) == 0 {
		return api.MCPSimpleError("MISSING_PARAMETER", "at least one slide is required in presentation"), nil
	}

	result := analyzeDeckRhythm(input.Slides)

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- Analysis logic ---

// analyzeDeckRhythm performs the core rhythm analysis on slide inputs.
func analyzeDeckRhythm(slides []SlideInput) *rhythmResult {
	perSlide := make([]rhythmSlideInfo, len(slides))

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

	result := &rhythmResult{
		PerSlide: perSlide,
		Aggregates: rhythmAggregates{
			PatternRuns:     runs,
			LongestRun:      longestRun,
			RepetitionIndex: computeRepetitionIndex(perSlide),
			AccentBalance:   computeAccentBalance(slides),
			DensityCV:       computeDensityCV(perSlide),
		},
	}

	result.Recommendations = generateRecommendations(perSlide, runs)
	result.CompositionScore = computeCompositionScore(result)

	return result
}

// fingerprint extracts the visual fingerprint of a single slide.
func fingerprint(idx int, s SlideInput) rhythmSlideInfo {
	info := rhythmSlideInfo{
		SlideIndex: idx,
	}

	// Determine pattern name.
	switch {
	case s.Pattern != nil && s.Pattern.Name != "":
		info.Pattern = s.Pattern.Name
	case s.ShapeGrid != nil:
		info.Pattern = "shape_grid"
	default:
		// Use slide_type if available, else infer from content.
		if s.SlideType != "" {
			info.Pattern = s.SlideType
		} else {
			info.Pattern = "content"
		}
	}

	// Determine dominant visual.
	info.DominantVisual = dominantVisual(s)

	// Determine density class based on content weight.
	info.DensityClass = densityClass(s)

	// Determine accent role — look for the first explicit color reference.
	info.AccentRole = primaryAccent(s)

	return info
}

// dominantVisual returns the primary visual type on a slide.
func dominantVisual(s SlideInput) string {
	if s.Pattern != nil {
		return "pattern"
	}
	if s.ShapeGrid != nil {
		return "grid"
	}
	for _, c := range s.Content {
		switch c.Type {
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

// densityClass estimates content density from the slide definition.
func densityClass(s SlideInput) string {
	weight := 0

	for _, c := range s.Content {
		switch c.Type {
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

	if s.ShapeGrid != nil {
		weight += 4
	}
	if s.Pattern != nil {
		weight += 3
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

// primaryAccent extracts the first accent color reference from slide content.
func primaryAccent(s SlideInput) string {
	// Check shape_grid cells for fill colors referencing accents.
	if s.ShapeGrid != nil {
		for _, row := range s.ShapeGrid.Rows {
			for _, cell := range row.Cells {
				if cell != nil && cell.Shape != nil {
					if fill := extractAccentFromFill(cell.Shape.Fill); fill != "" {
						return fill
					}
				}
			}
		}
	}
	return "none"
}

// extractAccentFromFill checks if a fill json.RawMessage references an accent color.
// Fill can be a JSON string ("accent1") or object ({"color":"accent1","alpha":80}).
func extractAccentFromFill(fill json.RawMessage) string {
	if len(fill) == 0 {
		return ""
	}

	// Try string form first.
	var s string
	if err := json.Unmarshal(fill, &s); err == nil {
		if isAccentColor(s) {
			return s
		}
		return ""
	}

	// Try object form.
	var obj struct {
		Color string `json:"color"`
	}
	if err := json.Unmarshal(fill, &obj); err == nil && isAccentColor(obj.Color) {
		return obj.Color
	}
	return ""
}

// isAccentColor returns true if the color name is a scheme accent.
func isAccentColor(c string) bool {
	switch c {
	case "accent1", "accent2", "accent3", "accent4", "accent5", "accent6":
		return true
	}
	return false
}

// detectPatternRuns finds consecutive runs of the same pattern.
func detectPatternRuns(slides []rhythmSlideInfo) []patternRun {
	if len(slides) == 0 {
		return nil
	}

	var runs []patternRun
	current := patternRun{Name: slides[0].Pattern, Start: 0, Len: 1}

	for i := 1; i < len(slides); i++ {
		if slides[i].Pattern == current.Name {
			current.Len++
		} else {
			if current.Len >= 2 {
				runs = append(runs, current)
			}
			current = patternRun{Name: slides[i].Pattern, Start: i, Len: 1}
		}
	}
	if current.Len >= 2 {
		runs = append(runs, current)
	}

	return runs
}

// computeRepetitionIndex measures how repetitive the pattern sequence is.
// 0.0 = all unique patterns, 1.0 = all the same pattern.
func computeRepetitionIndex(slides []rhythmSlideInfo) float64 {
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
func computeAccentBalance(slides []SlideInput) accentBalance {
	counts := map[string]int{}
	total := 0
	for _, s := range slides {
		si := rhythmSlideInfo{}
		si.AccentRole = primaryAccent(s)
		if si.AccentRole != "none" {
			counts[si.AccentRole]++
			total++
		}
	}

	if total == 0 {
		return accentBalance{}
	}

	balance := accentBalance{}
	for accent, count := range counts {
		balance[accent] = math.Round(float64(count)/float64(total)*100) / 100
	}
	return balance
}

// computeDensityCV calculates the coefficient of variation of density scores.
func computeDensityCV(slides []rhythmSlideInfo) float64 {
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

// generateRecommendations produces actionable suggestions for runs of 3+.
func generateRecommendations(slides []rhythmSlideInfo, runs []patternRun) []rhythmRecommendation {
	var recs []rhythmRecommendation

	for _, run := range runs {
		if run.Len < 3 {
			continue
		}

		// Recommend break points at every 3rd slide in the run.
		breakPatterns := suggestBreakPatterns(run.Name)

		for offset := 2; offset < run.Len; offset += 3 {
			insertIdx := run.Start + offset
			recs = append(recs, rhythmRecommendation{
				SlideIndex:       insertIdx,
				Message:          fmt.Sprintf("break a %s run (length %d); consider inserting a different pattern at slide %d", run.Name, run.Len, insertIdx),
				RecommendedBreak: breakPatterns,
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
func computeCompositionScore(r *rhythmResult) int {
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
