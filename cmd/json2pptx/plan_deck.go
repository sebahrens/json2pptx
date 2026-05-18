package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// --- Tool definition ---

func mcpPlanDeckTool() mcp.Tool {
	return mcp.NewTool("plan_deck",
		mcp.WithDescription(`Plan a presentation deck from a brief — returns an ordered slide outline with recommended patterns and narrative roles.

Use this BEFORE generate_presentation to get a structured plan. The output includes per-slide pattern recommendations, content seeds, and narrative roles (opening, evidence, comparison, close). The plan enforces deck-rhythm rules:
- No 3 consecutive slides with the same pattern
- At least one emphasis slide (stat-hero or pull-quote) every ~5 slides
- Accent color rotation for visual variety

The output is directly consumable as the slides array in generate_presentation — just fill in the content values.`),
		mcp.WithRawOutputSchema(outputSchemaPlanDeck),
		mcp.WithString("brief",
			mcp.Required(),
			mcp.Description("Natural-language description of the deck purpose and content (e.g., 'Pitch our Series B for an AI infra company')."),
		),
		mcp.WithNumber("slide_budget",
			mcp.Description("Target number of slides (default: 10, range: 3–30)."),
		),
		mcp.WithString("audience",
			mcp.Description("Target audience (e.g., 'board of directors', 'engineering team', 'investors'). Influences pattern selection."),
		),
		mcp.WithArray("must_include",
			mcp.Description("Pattern names that must appear in the plan (e.g., [\"bmc-canvas\", \"kpi-3up\"])."),
		),
	)
}

// --- Response types ---

// planSlide describes one slide in the planned deck outline.
type planSlide struct {
	SlideIndex   int    `json:"slide_index"`
	NarrativeRole string `json:"narrative_role"` // "opening", "evidence", "comparison", "emphasis", "framework", "closing"
	RecommendedPattern string `json:"recommended_pattern"`
	ContentSeed string `json:"content_seed"` // brief hint of what content should go here
	Rationale   string `json:"rationale"`

	// PredictedCellBudgets reports per-grid-configuration character budgets
	// derived from the recommended pattern's BudgetConfigProvider. Empty when
	// the pattern is not grid-shaped or declares no budget configurations.
	PredictedCellBudgets []predictedCellBudget `json:"predicted_cell_budgets,omitempty"`

	// PredictedFindings lists the top-ranked fit-report predictions that
	// expanding the recommended pattern with exemplar (role-default) content
	// would emit. Limited to the top 3 by ActionRank. Empty when the pattern
	// has no exemplar or expansion fails.
	PredictedFindings []predictedFinding `json:"predicted_findings,omitempty"`

	// Alternatives are the next-best ranked patterns for this slot, after
	// the recommended one. Up to 2 entries.
	Alternatives []plannedAlternative `json:"alternatives,omitempty"`
}

// predictedCellBudget is a single (columns × rows) configuration with the
// character budgets the renderer would impose on body and header text.
type predictedCellBudget struct {
	Columns        int `json:"columns"`
	Rows           int `json:"rows"`
	BodyMaxChars   int `json:"body_max_chars"`
	HeaderMaxChars int `json:"header_max_chars"`
}

// predictedFinding is a forecast fit-finding for the planned slide. It is a
// strict subset of patterns.FitFinding — only the fields agents act on at the
// planning stage.
type predictedFinding struct {
	Code         string `json:"code"`
	Path         string `json:"path,omitempty"`
	Message      string `json:"message"`
	Action       string `json:"action,omitempty"`
	NextToolCall string `json:"next_tool_call,omitempty"`
}

// plannedAlternative is a runner-up pattern for the slot.
type plannedAlternative struct {
	PatternName string  `json:"pattern_name"`
	Score       float64 `json:"score"`
	Rationale   string  `json:"rationale"`
}

// planDeckResult is the top-level plan_deck response.
type planDeckResult struct {
	Slides          []planSlide `json:"slides"`
	Brief           string      `json:"brief"`
	SlideBudget     int         `json:"slide_budget"`
	RhythmCheck     rhythmCheck `json:"rhythm_check"`

	// ResponseFingerprint is a sha256 hex digest of the canonical JSON of this
	// response with the field zeroed. Agents may use it as a cache key.
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`
}

// rhythmCheck summarizes the plan's adherence to rhythm rules.
type rhythmCheck struct {
	LongestPatternRun int  `json:"longest_pattern_run"`
	HasEmphasis       bool `json:"has_emphasis"`
	EmphasisCount     int  `json:"emphasis_count"`
	PatternVariety    int  `json:"pattern_variety"` // unique pattern count
}

// --- Handler ---

func handlePlanDeck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	brief, err := request.RequireString("brief")
	if err != nil {
		return mcpErrorWithNext("MISSING_PARAMETER", "brief is required", nextCallRetry("plan_deck", "brief")), nil
	}

	slideBudget := 10
	if sb, ok := request.GetArguments()["slide_budget"]; ok {
		if f, ok := sb.(float64); ok {
			slideBudget = int(f)
		}
	}
	if slideBudget < 3 {
		slideBudget = 3
	}
	if slideBudget > 30 {
		slideBudget = 30
	}

	audience := ""
	if a, ok := request.GetArguments()["audience"]; ok {
		if s, ok := a.(string); ok {
			audience = s
		}
	}

	var mustInclude []string
	if miRaw, ok := request.GetArguments()["must_include"]; ok && miRaw != nil {
		miJSON, err := json.Marshal(miRaw)
		if err == nil {
			_ = json.Unmarshal(miJSON, &mustInclude)
		}
	}

	// Validate must_include patterns exist.
	reg := patterns.Default()
	for _, name := range mustInclude {
		if _, ok := reg.Get(name); !ok {
			return mcpErrorWithNext("INVALID_PARAMETER", fmt.Sprintf("must_include pattern %q not found; use list_patterns to see available patterns", name), nextCallListPatterns()), nil
		}
	}

	result := buildDeckPlan(reg, brief, slideBudget, audience, mustInclude)

	if err := api.ComputeResponseFingerprint(result); err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to compute response fingerprint: %v", err), nextCallRetry("plan_deck", "brief")), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, result)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("failed to marshal response: %v", err), nextCallRetry("plan_deck", "brief")), nil
	}
	return mcpResult, nil
}

// --- Planning logic ---

// narrativeArc defines the standard deck narrative structure.
// Each entry maps a narrative role to the fraction of the deck it should occupy.
type narrativeArc struct {
	role     string
	fraction float64
	hint     string // content seed hint
}

var standardArc = []narrativeArc{
	{"opening", 0.10, "title and context-setting"},
	{"framework", 0.15, "structure or framework overview"},
	{"evidence", 0.40, "supporting data and details"},
	{"comparison", 0.15, "comparison or decision point"},
	{"emphasis", 0.10, "key statistic or memorable quote"},
	{"closing", 0.10, "summary and next steps"},
}

// emphasisPatterns are patterns that serve as visual emphasis / breathing room.
var emphasisPatterns = map[string]bool{
	"stat-hero":  true,
	"pull-quote": true,
}

// narrativeRoleToTaxonomy maps our arc roles to taxonomy NarrativeRole values.
var narrativeRoleToTaxonomy = map[string][]string{
	"opening":    {"open"},
	"framework":  {"frame"},
	"evidence":   {"evidence"},
	"comparison": {"compare"},
	"emphasis":   {"open", "conclude"}, // emphasis patterns overlap with open/conclude
	"closing":    {"conclude"},
}

func buildDeckPlan(reg *patterns.Registry, brief string, budget int, audience string, mustInclude []string) *planDeckResult {
	// 1. Distribute slides across narrative roles.
	roleSlots := distributeRoles(budget)

	// 2. Assign patterns to each slot.
	slides := assignPatterns(reg, brief, audience, roleSlots, mustInclude)

	// 3. Enforce rhythm rules — break runs of 3+ and inject emphasis.
	slides = enforceRhythm(reg, slides, brief)

	// 4. Attach per-slot predictions: cell budgets, top-3 fit findings,
	//    and ranked alternatives. Done after rhythm enforcement so the
	//    predictions reflect the final pattern choice for each slot.
	attachSlidePredictions(reg, slides, brief, audience)

	// 5. Build rhythm check.
	check := computeRhythmCheck(slides)

	return &planDeckResult{
		Slides:      slides,
		Brief:       brief,
		SlideBudget: budget,
		RhythmCheck: check,
	}
}

// distributeRoles allocates slide indices to narrative roles based on the arc.
func distributeRoles(budget int) []string {
	roles := make([]string, budget)

	// Always reserve first and last slots.
	roles[0] = "opening"
	roles[budget-1] = "closing"

	// Distribute remaining slots proportionally.
	remaining := budget - 2
	if remaining <= 0 {
		return roles
	}

	// Build pool of middle roles with their target counts.
	type roleCount struct {
		role  string
		count int
	}
	middleArc := standardArc[1 : len(standardArc)-1] // skip opening/closing
	totalFraction := 0.0
	for _, a := range middleArc {
		totalFraction += a.fraction
	}

	var pool []roleCount
	assigned := 0
	for i, a := range middleArc {
		count := int(math.Round(float64(remaining) * a.fraction / totalFraction))
		if count < 1 && remaining-assigned > len(middleArc)-i {
			count = 1
		}
		if assigned+count > remaining {
			count = remaining - assigned
		}
		if count > 0 {
			pool = append(pool, roleCount{a.role, count})
			assigned += count
		}
	}

	// Fill any remaining slots with evidence.
	for assigned < remaining {
		if len(pool) > 0 {
			// Add to evidence slot.
			for i := range pool {
				if pool[i].role == "evidence" {
					pool[i].count++
					assigned++
					break
				}
			}
			if assigned < remaining {
				pool = append(pool, roleCount{"evidence", 1})
				assigned++
			}
		}
	}

	// Lay out roles in order.
	idx := 1
	for _, rc := range pool {
		for range rc.count {
			if idx < budget-1 {
				roles[idx] = rc.role
				idx++
			}
		}
	}

	// Fill any gaps.
	for i := range roles {
		if roles[i] == "" {
			roles[i] = "evidence"
		}
	}

	return roles
}

// assignPatterns picks a pattern for each narrative role slot.
func assignPatterns(reg *patterns.Registry, brief, audience string, roleSlots []string, mustInclude []string) []planSlide {
	slides := make([]planSlide, len(roleSlots))
	usedPatterns := make([]string, 0, len(roleSlots))
	mustIncludeUsed := make(map[string]bool)

	// Build a pattern-to-roles index from taxonomy.
	allPatterns := reg.List()
	var patternList []patInfo
	for _, p := range allPatterns {
		patternList = append(patternList, patInfo{name: p.Name(), taxonomy: p.Taxonomy()})
	}

	for i, role := range roleSlots {
		slides[i].SlideIndex = i
		slides[i].NarrativeRole = role
		slides[i].ContentSeed = contentSeedForRole(role, brief, i, len(roleSlots))

		// Check if we should place a must_include pattern here.
		if pat := pickMustInclude(role, mustInclude, mustIncludeUsed, patternList); pat != "" {
			slides[i].RecommendedPattern = pat
			slides[i].Rationale = "required by must_include"
			mustIncludeUsed[pat] = true
			usedPatterns = append(usedPatterns, pat)
			continue
		}

		// Use recommend_pattern with variety awareness.
		intent := buildIntent(role, brief, audience)
		opts := &patterns.RecommendOptions{
			RecentPatterns: usedPatterns,
			PreferVariety:  true,
			SlideIndex:     i,
		}
		rec := patterns.Recommend(reg, intent, nil, 1, opts)

		if len(rec.Candidates) > 0 {
			c := rec.Candidates[0]
			slides[i].RecommendedPattern = c.PatternName
			slides[i].Rationale = c.Rationale
		} else {
			// Fallback: pick from taxonomy.
			slides[i].RecommendedPattern = fallbackPattern(role, patternList, usedPatterns)
			slides[i].Rationale = "fallback selection for " + role
		}
		usedPatterns = append(usedPatterns, slides[i].RecommendedPattern)
	}

	// Place any remaining must_include patterns that weren't used.
	for _, mi := range mustInclude {
		if mustIncludeUsed[mi] {
			continue
		}
		// Find the best slot to swap — prefer evidence slots with generic patterns.
		bestIdx := -1
		for i := 1; i < len(slides)-1; i++ {
			if !mustIncludeUsed[slides[i].RecommendedPattern] && slides[i].NarrativeRole == "evidence" {
				bestIdx = i
				break
			}
		}
		if bestIdx == -1 {
			// Use any middle slot.
			for i := 1; i < len(slides)-1; i++ {
				if !containsStr(mustInclude, slides[i].RecommendedPattern) || mustIncludeUsed[slides[i].RecommendedPattern] {
					bestIdx = i
					break
				}
			}
		}
		if bestIdx >= 0 {
			slides[bestIdx].RecommendedPattern = mi
			slides[bestIdx].Rationale = "required by must_include"
			mustIncludeUsed[mi] = true
		}
	}

	return slides
}

// pickMustInclude checks if any unused must_include pattern fits the current role.
func pickMustInclude(role string, mustInclude []string, used map[string]bool, patternList []patInfo) string {
	taxRoles := narrativeRoleToTaxonomy[role]
	for _, mi := range mustInclude {
		if used[mi] {
			continue
		}
		for _, pi := range patternList {
			if pi.name != mi {
				continue
			}
			// Check if this pattern's taxonomy roles overlap with the current narrative role.
			for _, tr := range taxRoles {
				for _, nr := range pi.taxonomy.NarrativeRole {
					if tr == nr {
						return mi
					}
				}
			}
		}
	}
	return ""
}

type patInfo struct {
	name     string
	taxonomy patterns.PatternTaxonomy
}

// buildIntent constructs a recommend_pattern intent string from the role and brief.
func buildIntent(role, brief, audience string) string {
	var parts []string

	switch role {
	case "opening":
		parts = append(parts, "title slide or opening statement")
	case "framework":
		parts = append(parts, "framework overview or structure")
	case "evidence":
		parts = append(parts, "supporting data or details")
	case "comparison":
		parts = append(parts, "compare options or decisions")
	case "emphasis":
		parts = append(parts, "key statistic or memorable quote for emphasis")
	case "closing":
		parts = append(parts, "summary and call to action")
	}

	// Add brief context.
	if brief != "" {
		parts = append(parts, "for: "+brief)
	}
	if audience != "" {
		parts = append(parts, "audience: "+audience)
	}

	return strings.Join(parts, "; ")
}

// contentSeedForRole generates a content hint based on the role and position.
func contentSeedForRole(role, brief string, idx, total int) string {
	switch role {
	case "opening":
		return "Title and context: " + truncateBrief(brief, 80)
	case "framework":
		return "Structure or methodology overview"
	case "evidence":
		position := float64(idx) / float64(total)
		if position < 0.4 {
			return "Key data point or supporting detail"
		}
		return "Detailed evidence or case study"
	case "comparison":
		return "Comparison of alternatives or trade-offs"
	case "emphasis":
		return "Standout metric or memorable takeaway"
	case "closing":
		return "Summary, next steps, or call to action"
	default:
		return "Supporting content"
	}
}

// fallbackPattern picks a pattern from taxonomy when recommend_pattern returns nothing.
func fallbackPattern(role string, patternList []patInfo, used []string) string {
	taxRoles := narrativeRoleToTaxonomy[role]
	usedCount := make(map[string]int)
	for _, u := range used {
		usedCount[u]++
	}

	// Score candidates by taxonomy match and freshness.
	type candidate struct {
		name  string
		score float64
	}
	var candidates []candidate
	for _, pi := range patternList {
		score := 0.0
		for _, tr := range taxRoles {
			for _, nr := range pi.taxonomy.NarrativeRole {
				if tr == nr {
					score += 1.0
				}
			}
		}
		if score > 0 {
			// Penalize recently used.
			score -= float64(usedCount[pi.name]) * 0.3
			candidates = append(candidates, candidate{pi.name, score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if len(candidates) > 0 {
		return candidates[0].name
	}

	// Ultimate fallback.
	switch role {
	case "opening":
		return "stat-hero"
	case "closing":
		return "pull-quote"
	default:
		return "card-grid"
	}
}

// enforceRhythm applies rhythm rules: break runs of 3+ and inject emphasis.
func enforceRhythm(reg *patterns.Registry, slides []planSlide, brief string) []planSlide {
	// Pass 1: Break pattern runs of 3+.
	slides = breakLongRuns(reg, slides)

	// Pass 2: Ensure emphasis slides every ~5 slides.
	slides = ensureEmphasis(reg, slides)

	// Pass 3: Break any new runs introduced by emphasis injection.
	slides = breakLongRuns(reg, slides)

	return slides
}

// breakLongRuns detects consecutive runs of 3+ and swaps the middle slide.
func breakLongRuns(reg *patterns.Registry, slides []planSlide) []planSlide {
	for iteration := 0; iteration < 3; iteration++ { // max 3 passes to converge
		changed := false
		runStart := 0
		for i := 1; i <= len(slides); i++ {
			if i < len(slides) && slides[i].RecommendedPattern == slides[runStart].RecommendedPattern {
				continue
			}
			runLen := i - runStart
			if runLen >= 3 {
				// Swap the middle element of the run.
				mid := runStart + runLen/2
				if mid > 0 && mid < len(slides)-1 {
					replacement := findBreakPattern(reg, slides, mid)
					if replacement != slides[mid].RecommendedPattern {
						slides[mid].RecommendedPattern = replacement
						slides[mid].Rationale = fmt.Sprintf("rhythm break: interrupted %d-slide run of %s", runLen, slides[runStart].RecommendedPattern)
						changed = true
					}
				}
			}
			runStart = i
		}
		if !changed {
			break
		}
	}
	return slides
}

// findBreakPattern finds a pattern that breaks a run at the given index.
func findBreakPattern(reg *patterns.Registry, slides []planSlide, idx int) string {
	current := slides[idx].RecommendedPattern
	role := slides[idx].NarrativeRole

	// Build recent patterns excluding the current.
	recent := make([]string, 0, len(slides))
	for _, s := range slides {
		recent = append(recent, s.RecommendedPattern)
	}

	// Ask recommend for a variety pick.
	opts := &patterns.RecommendOptions{
		RecentPatterns: recent,
		PreferVariety:  true,
		SlideIndex:     idx,
	}
	intent := buildIntent(role, "", "")
	rec := patterns.Recommend(reg, intent, nil, 3, opts)

	for _, c := range rec.Candidates {
		if c.PatternName != current {
			return c.PatternName
		}
	}

	// Fallback: use an emphasis pattern.
	if !emphasisPatterns[current] {
		return "stat-hero"
	}
	return "card-grid"
}

// ensureEmphasis checks that at least one emphasis pattern appears every ~5 slides.
func ensureEmphasis(reg *patterns.Registry, slides []planSlide) []planSlide {
	if len(slides) <= 4 {
		// Too short to need emphasis injection.
		return slides
	}

	// Find existing emphasis positions.
	emphasisPositions := make([]int, 0)
	for i, s := range slides {
		if emphasisPatterns[s.RecommendedPattern] {
			emphasisPositions = append(emphasisPositions, i)
		}
	}

	if len(emphasisPositions) > 0 {
		// Check if any gap exceeds 6 slides.
		needsInsertion := false
		prev := 0
		for _, pos := range emphasisPositions {
			if pos-prev > 6 {
				needsInsertion = true
				break
			}
			prev = pos
		}
		if len(slides)-1-prev > 6 {
			needsInsertion = true
		}
		if !needsInsertion {
			return slides
		}
	}

	// Insert emphasis at ~every 5th slide, preferring evidence slots.
	for i := 4; i < len(slides)-1; i += 5 {
		if emphasisPatterns[slides[i].RecommendedPattern] {
			continue // already emphasis
		}
		// Don't replace opening/closing.
		if slides[i].NarrativeRole == "opening" || slides[i].NarrativeRole == "closing" {
			continue
		}
		// Alternate between stat-hero and pull-quote.
		emphPat := "stat-hero"
		if len(emphasisPositions)%2 == 1 {
			emphPat = "pull-quote"
		}
		slides[i].RecommendedPattern = emphPat
		slides[i].NarrativeRole = "emphasis"
		slides[i].ContentSeed = "Standout metric or memorable takeaway"
		slides[i].Rationale = "emphasis injection: visual breathing room every ~5 slides"
		emphasisPositions = append(emphasisPositions, i)
	}

	return slides
}

// computeRhythmCheck summarizes rhythm metrics for the plan.
func computeRhythmCheck(slides []planSlide) rhythmCheck {
	longestRun := 1
	currentRun := 1
	for i := 1; i < len(slides); i++ {
		if slides[i].RecommendedPattern == slides[i-1].RecommendedPattern {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
		} else {
			currentRun = 1
		}
	}

	emphasisCount := 0
	for _, s := range slides {
		if emphasisPatterns[s.RecommendedPattern] {
			emphasisCount++
		}
	}

	unique := make(map[string]bool)
	for _, s := range slides {
		unique[s.RecommendedPattern] = true
	}

	return rhythmCheck{
		LongestPatternRun: longestRun,
		HasEmphasis:       emphasisCount > 0,
		EmphasisCount:     emphasisCount,
		PatternVariety:    len(unique),
	}
}

// --- Helpers ---

func truncateBrief(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// --- Prediction (cell budgets, fit findings, alternatives) ---

// maxPredictedFindings caps the predicted_findings list emitted per slide.
const maxPredictedFindings = 3

// maxAlternatives caps the alternatives list emitted per slide.
const maxAlternatives = 2

// attachSlidePredictions enriches each slide in-place with predicted cell
// budgets, predicted fit findings, and ranked alternative patterns. Pure
// planning — no render is performed.
func attachSlidePredictions(reg *patterns.Registry, slides []planSlide, brief, audience string) {
	for i := range slides {
		slides[i].PredictedCellBudgets = predictCellBudgetsForPattern(reg, slides[i].RecommendedPattern)
		slides[i].PredictedFindings = predictFitFindingsForPattern(reg, slides[i].RecommendedPattern, i)
		slides[i].Alternatives = computeAlternativesForSlot(reg, slides, i, brief, audience)
	}
}

// predictCellBudgetsForPattern returns the per-configuration cell budgets the
// renderer would impose on the pattern. Uses the existing text_budget_guide
// computation; returns nil for non-grid patterns.
func predictCellBudgetsForPattern(reg *patterns.Registry, name string) []predictedCellBudget {
	pat, ok := reg.Get(name)
	if !ok {
		return nil
	}
	guide := computeTextBudgetGuide(pat)
	if guide == nil || len(guide.Configurations) == 0 {
		return nil
	}
	out := make([]predictedCellBudget, 0, len(guide.Configurations))
	for _, c := range guide.Configurations {
		out = append(out, predictedCellBudget{
			Columns:        c.Columns,
			Rows:           c.Rows,
			BodyMaxChars:   c.BodyMaxChars,
			HeaderMaxChars: c.HeaderMaxChars,
		})
	}
	return out
}

// predictFitFindingsForPattern expands a pattern with its declared exemplar
// values and runs the full fit-finding collector against a synthetic
// PresentationInput. Returns the top-ranked findings (up to maxPredictedFindings).
//
// No template/theme are available at planning time, so structural checks that
// need a layout (placeholder overflow, footer collision) are skipped and only
// shape-grid-resident detectors fire (text overflow, sparse layout, pattern
// occupancy, table preflight). That is intentional — the plan path must not
// render.
func predictFitFindingsForPattern(reg *patterns.Registry, name string, slideIdx int) []predictedFinding {
	pat, ok := reg.Get(name)
	if !ok {
		return nil
	}
	ex, ok := pat.(patterns.Exemplar)
	if !ok {
		return nil
	}
	values := ex.ExemplarValues()
	if values == nil {
		return nil
	}

	// Default 16:9 slide bounds, matching computeTextBudgetGuide so budgets
	// and findings are derived from the same canonical geometry.
	const (
		slideWidth  int64 = 9144000
		slideHeight int64 = 5143500
	)
	expandCtx := patterns.ExpandContext{
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
		LayoutBounds: patterns.LayoutBounds{
			X:      457200,
			Y:      457200,
			Width:  8229600,
			Height: 4229100,
		},
	}

	grid, err := pat.Expand(expandCtx, values, nil, nil)
	if err != nil || grid == nil {
		return nil
	}

	input := &PresentationInput{
		Slides: []SlideInput{{
			ShapeGrid: grid,
			Pattern:   &PatternInput{Name: name},
		}},
	}

	findings := collectFitFindings(input, nil, slideWidth, slideHeight, nil)
	if len(findings) == 0 {
		return nil
	}

	limit := maxPredictedFindings
	if len(findings) < limit {
		limit = len(findings)
	}
	out := make([]predictedFinding, 0, limit)
	for i := 0; i < limit; i++ {
		f := findings[i]
		nextTool := ""
		if f.NextToolCall != nil {
			nextTool = f.NextToolCall.Tool
		}
		out = append(out, predictedFinding{
			Code:         f.Code,
			Path:         f.Path,
			Message:      f.Message,
			Action:       f.Action,
			NextToolCall: nextTool,
		})
	}
	return out
}

// computeAlternativesForSlot returns the next-best ranked patterns for this
// slot after the recommended one, excluding patterns already used on adjacent
// slides to preserve the rhythm guarantees.
func computeAlternativesForSlot(reg *patterns.Registry, slides []planSlide, idx int, brief, audience string) []plannedAlternative {
	if idx < 0 || idx >= len(slides) {
		return nil
	}
	current := slides[idx].RecommendedPattern
	role := slides[idx].NarrativeRole

	// Build recent-patterns context from neighbors so variety scoring matches
	// the original placement.
	recent := make([]string, 0, len(slides))
	for j, s := range slides {
		if j == idx {
			continue
		}
		recent = append(recent, s.RecommendedPattern)
	}

	intent := buildIntent(role, brief, audience)
	opts := &patterns.RecommendOptions{
		RecentPatterns: recent,
		PreferVariety:  true,
		SlideIndex:     idx,
	}
	rec := patterns.Recommend(reg, intent, nil, maxAlternatives+2, opts)

	out := make([]plannedAlternative, 0, maxAlternatives)
	seen := map[string]bool{current: true}
	for _, c := range rec.Candidates {
		if seen[c.PatternName] {
			continue
		}
		seen[c.PatternName] = true
		out = append(out, plannedAlternative{
			PatternName: c.PatternName,
			Score:       c.Score,
			Rationale:   c.Rationale,
		})
		if len(out) >= maxAlternatives {
			break
		}
	}

	// Fall back to taxonomy-matching patterns when Recommend returns too few
	// candidates (e.g. niche roles where rule-based scoring drops below the
	// threshold).
	if len(out) < maxAlternatives {
		var patternList []patInfo
		for _, p := range reg.List() {
			patternList = append(patternList, patInfo{name: p.Name(), taxonomy: p.Taxonomy()})
		}
		for _, fallback := range taxonomyFallbackCandidates(role, patternList, recent) {
			if seen[fallback] {
				continue
			}
			seen[fallback] = true
			out = append(out, plannedAlternative{
				PatternName: fallback,
				Score:       0,
				Rationale:   "taxonomy fallback for " + role,
			})
			if len(out) >= maxAlternatives {
				break
			}
		}
	}
	return out
}

// taxonomyFallbackCandidates returns pattern names that match the narrative
// role by taxonomy, ranked by freshness (least-used first).
func taxonomyFallbackCandidates(role string, patternList []patInfo, used []string) []string {
	taxRoles := narrativeRoleToTaxonomy[role]
	usedCount := make(map[string]int)
	for _, u := range used {
		usedCount[u]++
	}

	type cand struct {
		name  string
		score float64
	}
	var cands []cand
	for _, pi := range patternList {
		score := 0.0
		for _, tr := range taxRoles {
			for _, nr := range pi.taxonomy.NarrativeRole {
				if tr == nr {
					score += 1.0
				}
			}
		}
		if score <= 0 {
			continue
		}
		score -= float64(usedCount[pi.name]) * 0.3
		cands = append(cands, cand{pi.name, score})
	}
	sort.Slice(cands, func(i, j int) bool {
		return cands[i].score > cands[j].score
	})
	names := make([]string, 0, len(cands))
	for _, c := range cands {
		names = append(names, c.name)
	}
	return names
}
