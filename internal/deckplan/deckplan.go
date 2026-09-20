// Package deckplan assembles a presentation deck plan from a brief: an ordered
// slide outline with narrative roles, recommended patterns, content seeds,
// fillable skeletons, and ranked alternatives, all subject to deck-rhythm
// rules (no long pattern runs, periodic emphasis, variety-aware selection).
//
// The planning core is template-agnostic and render-free. When a
// generator.TemplateSupportContext is supplied it additionally swaps any
// recommended pattern the template cannot host for a supported alternative and
// annotates every slide (and alternative) with its template_support.
//
// Render-coupled forecasts — predicted cell budgets and predicted fit
// findings — are injected through the Predictor interface so the machinery
// that needs the full PresentationInput → fit-finding collector stays in
// package main. A nil Predictor (used by this package's own tests) simply
// skips those forecasts. The plan_deck MCP tool and the make_deck facade in
// cmd/json2pptx both call BuildDeckPlan.
package deckplan

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// --- Response types ---

// Slide describes one slide in the planned deck outline.
type Slide struct {
	SlideIndex         int    `json:"slide_index"`
	NarrativeRole      string `json:"narrative_role"`      // "opening", "evidence", "comparison", "emphasis", "framework", "closing"
	RecommendedPattern string `json:"recommended_pattern"` // "" for the title (opening) and closing slides — they use a structural layout, not a pattern

	// Layout is the canonical layout_id the slide should use: "title" for the
	// opening slide, "closing" for the closing slide, and "blank-title" for
	// every pattern-bearing slide (a pattern needs the free canvas below a
	// title). Always equal to the skeleton's layout_id.
	Layout      string `json:"layout"`
	ContentSeed string `json:"content_seed"` // brief hint of what content should go here; brief facts assigned to the slide are prefixed

	// Facts are the brief's own quantity / named-entity clauses routed to this
	// slide (e.g. "+23% revenue", "churn 4%"), verbatim. Quantities go to KPI /
	// stat / chart patterns first. Omitted when the slide received none; title
	// and closing slides never receive facts.
	Facts     []string `json:"facts,omitempty"`
	Rationale string   `json:"rationale"`

	// SuggestedPattern is the first-choice pattern for this slot. Currently
	// always equal to RecommendedPattern; kept as a separate field so the
	// (suggested_pattern, suggested_pattern_fallback, skeleton) triplet reads
	// as a single agent-facing contract.
	SuggestedPattern string `json:"suggested_pattern"`

	// SuggestedPatternFallback is the second-choice pattern when the
	// suggested pattern's content shape does not fit. Drawn from
	// Alternatives[0] when available, empty otherwise.
	SuggestedPatternFallback string `json:"suggested_pattern_fallback,omitempty"`

	// Skeleton is a partial JSON slide object with FillPlaceholder ("__FILL__")
	// tokens for every agent-supplied string. Agents copy the skeleton and
	// replace tokens rather than authoring slide structure from scratch.
	// Empty when the pattern has no exemplar (skeleton generation requires one).
	Skeleton json.RawMessage `json:"skeleton,omitempty"`

	// PredictedCellBudgets reports per-grid-configuration character budgets
	// derived from the recommended pattern's BudgetConfigProvider. Empty when
	// the pattern is not grid-shaped or declares no budget configurations.
	PredictedCellBudgets []CellBudget `json:"predicted_cell_budgets,omitempty"`

	// PredictedFindings lists the top-ranked fit-report predictions that
	// expanding the recommended pattern with exemplar (role-default) content
	// would emit. Empty when the pattern has no exemplar or expansion fails.
	PredictedFindings []Finding `json:"predicted_findings,omitempty"`

	// Alternatives are the next-best ranked patterns for this slot, after
	// the recommended one. Up to MaxAlternatives entries.
	Alternatives []Alternative `json:"alternatives,omitempty"`

	// TemplateSupport reports how well the recommended pattern fits the
	// template passed via Params.TemplateCtx. Populated by the shared
	// recommendation helper (the same one recommend_visual uses); nil when no
	// template context was supplied (template-agnostic plan, unchanged).
	TemplateSupport *patterns.TemplateSupport `json:"template_support,omitempty"`
}

// CellBudget is a single (columns × rows) configuration with the character
// budgets the renderer would impose on body and header text.
type CellBudget struct {
	Columns        int `json:"columns"`
	Rows           int `json:"rows"`
	BodyMaxChars   int `json:"body_max_chars"`
	HeaderMaxChars int `json:"header_max_chars"`
}

// Finding is a forecast fit-finding for the planned slide. It is a strict
// subset of patterns.FitFinding — only the fields agents act on at the
// planning stage.
type Finding struct {
	Code         string `json:"code"`
	Path         string `json:"path,omitempty"`
	Message      string `json:"message"`
	Action       string `json:"action,omitempty"`
	NextToolCall string `json:"next_tool_call,omitempty"`
}

// Alternative is a runner-up pattern for the slot.
type Alternative struct {
	PatternName string  `json:"pattern_name"`
	Score       float64 `json:"score"`
	Rationale   string  `json:"rationale"`

	// TemplateSupport reports the alternative's fit for the template passed via
	// Params.TemplateCtx, so a caller swapping in a fallback can see its
	// feasibility up front. Nil when no template context was supplied.
	TemplateSupport *patterns.TemplateSupport `json:"template_support,omitempty"`
}

// Result is the top-level plan_deck response.
type Result struct {
	Slides      []Slide     `json:"slides"`
	Brief       string      `json:"brief"`
	SlideBudget int         `json:"slide_budget"`
	RhythmCheck RhythmCheck `json:"rhythm_check"`

	// UnplacedFacts lists brief facts (quantity / named-entity clauses) that no
	// slide had capacity for, so none silently disappears. Always present;
	// empty when every fact was placed or the brief had none.
	UnplacedFacts []string `json:"unplaced_facts"`

	// Template echoes the template name the plan was vetted against, when a
	// template context was supplied. Empty for a template-agnostic plan.
	Template string `json:"template,omitempty"`

	// BudgetNote explains a plan shorter than slide_budget. The planner bounds
	// each narrative role by the patterns and the brief content behind it, and
	// when the surplus fits nowhere it returns fewer slides rather than padding
	// the deck with repeats (go-slide-creator-whp97). Empty when the plan used
	// the whole budget.
	BudgetNote string `json:"budget_note,omitempty"`

	// ResponseFingerprint is a sha256 hex digest of the canonical JSON of this
	// response with the field zeroed. Agents may use it as a cache key. The
	// caller (handlePlanDeck) populates it via api.ComputeResponseFingerprint.
	ResponseFingerprint string `json:"response_fingerprint,omitempty"`
}

// RhythmCheck summarizes the plan's adherence to rhythm rules.
type RhythmCheck struct {
	LongestPatternRun int  `json:"longest_pattern_run"`
	HasEmphasis       bool `json:"has_emphasis"`
	EmphasisCount     int  `json:"emphasis_count"`
	PatternVariety    int  `json:"pattern_variety"` // unique pattern count
	// RepeatedFamilies names any pattern family the plan still uses more than
	// maxPatternRepeats times, with its count ("comparison-2col x3"). The
	// planner caps repeats, but a slot whose role has few candidate patterns —
	// four comparison slots in a 20-slide deck — cannot always be given
	// something else, and an agent reading the plan should see that rather
	// than discovering it in the render (go-slide-creator-8fq4).
	RepeatedFamilies []string `json:"repeated_families,omitempty"`
}

// MaxAlternatives caps the alternatives list emitted per slide.
const MaxAlternatives = 2

// mustIncludeRationale marks a slot filled by a must_include pattern; such
// slots are exempt from the emphasis cap.
const mustIncludeRationale = "required by must_include"

// --- Inputs ---

// Params bundles the planning inputs. SlideBudget is assumed already clamped to
// a sane range by the caller (handlePlanDeck and make_deck both clamp to
// [3, 30]); BuildDeckPlan does not re-clamp.
type Params struct {
	Brief       string
	SlideBudget int
	Audience    string
	MustInclude []string

	// TemplateCtx, when non-nil, makes the plan template-aware: a recommended
	// pattern the template cannot host is swapped for a supported alternative
	// before predictions are attached, and every slide / alternative is
	// annotated with its template_support.
	TemplateCtx *generator.TemplateSupportContext

	// TemplateName is echoed in Result.Template for traceability. Pass "" with
	// a nil TemplateCtx for a template-agnostic plan.
	TemplateName string
}

// Predictor supplies the render-coupled forecasts attached to each planned
// slide. It lives behind an interface because computing fit findings requires
// the full PresentationInput → fit-finding collector machinery, which stays in
// package main. A nil Predictor skips both forecasts (used by deckplan tests).
type Predictor interface {
	// CellBudgets returns the per-grid-configuration character budgets for the
	// named pattern, or nil for non-grid / unknown patterns.
	CellBudgets(patternName string) []CellBudget

	// FitFindings returns the top-ranked fit-finding forecasts for expanding
	// the named pattern with exemplar content on the given slide index.
	FitFindings(patternName string, slideIndex int) []Finding
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
// Their count is capped at MaxEmphasisSlides(n) so emphasis stays emphatic.
var emphasisPatterns = map[string]bool{
	"stat-hero":  true,
	"pull-quote": true,
	"kpi-inline": true,
}

// comparisonFamily is the closed set of patterns a "comparison" narrative slot
// may use. Free-form recommendation for "compare options" intents otherwise
// drifts to process / flow patterns that do not show two sides.
var comparisonFamily = []string{"comparison-2col", "before-after"}

// Canonical layout IDs the plan assigns. Resolved against the template at
// generate time (see internal/layout/canonical.go).
const (
	LayoutTitle   = "title"
	LayoutClosing = "closing"
	LayoutPattern = "blank-title"
)

// isStructuralRole reports whether a narrative role is served by a structural
// layout (title / closing) rather than a named pattern.
func isStructuralRole(role string) bool {
	return role == "opening" || role == "closing"
}

// layoutForRole returns the canonical layout for a slide in the given role.
func layoutForRole(role string) string {
	switch role {
	case "opening":
		return LayoutTitle
	case "closing":
		return LayoutClosing
	default:
		return LayoutPattern
	}
}

// MaxEmphasisSlides is the emphasis-pattern cap for an n-slide deck:
// ceil(n/5). A 10-slide deck gets at most 2 stat-hero / pull-quote /
// kpi-inline slides.
func MaxEmphasisSlides(n int) int {
	if n <= 0 {
		return 0
	}
	return (n + 4) / 5
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

// BuildDeckPlan assembles a deck plan from the given parameters. When
// p.TemplateCtx is non-nil the plan becomes template-aware (see Params). The
// predictor supplies per-slide cell-budget and fit-finding forecasts; pass nil
// to skip them.
func BuildDeckPlan(reg *patterns.Registry, p Params, predictor Predictor) *Result {
	// 1. Distribute slides across narrative roles.
	roleSlots, budgetNote := distributeRoles(reg, p.Brief, p.SlideBudget)

	// 2. Assign patterns to each slot.
	slides := assignPatterns(reg, p.Brief, p.Audience, roleSlots, p.MustInclude)

	// 3. Enforce rhythm rules — break runs of 3+ and inject emphasis.
	slides = enforceRhythm(reg, slides, p.Brief)

	// 4. With template context, replace any recommended pattern the template
	//    cannot host with a supported alternative. Done before predictions so the
	//    cell budgets / findings / skeleton reflect the final pattern.
	if p.TemplateCtx != nil {
		swapInfeasiblePatterns(reg, p.TemplateCtx, slides, p.Brief, p.Audience)
	}

	// 4b. Route the brief's facts (quantities, named entities) into the content
	//     seeds of the final pattern slots; leftovers become unplaced_facts.
	unplaced := assignBriefFacts(slides, p.Brief)

	// 5. Attach per-slot predictions: cell budgets, fit findings, ranked
	//    alternatives, suggested-pattern triplet, and skeleton. Done after
	//    rhythm enforcement so the predictions reflect the final pattern choice.
	attachSlidePredictions(reg, slides, p.Brief, p.Audience, predictor)

	// 6. With template context, annotate each slide and alternative with the
	//    shared recommendation helper's support assessment.
	if p.TemplateCtx != nil {
		annotatePlanTemplateSupport(p.TemplateCtx, slides)
	}

	// 7. Build rhythm check.
	check := computeRhythmCheck(slides)

	return &Result{
		Slides:        slides,
		Brief:         p.Brief,
		SlideBudget:   p.SlideBudget,
		RhythmCheck:   check,
		UnplacedFacts: unplaced,
		Template:      p.TemplateName,
		BudgetNote:    budgetNote,
	}
}

// patternFeasibleForTemplate reports whether the named pattern can be hosted by
// the template. "Feasible" means the shared helper does not report it as
// unsupported (supported and risky both count, mirroring recommend_visual's
// demotion policy, which only sinks unsupported candidates below feasible ones).
func patternFeasibleForTemplate(tc *generator.TemplateSupportContext, name string) bool {
	ts := tc.Support(patterns.VisualCategoryPattern, name, nil)
	return ts == nil || ts.Status != patterns.TemplateSupportUnsupported
}

// swapInfeasiblePatterns replaces any recommended pattern the template cannot
// host with the first feasible alternative for that slot, so the plan never
// assigns an impossible pattern when a supported one exists. It is a no-op for
// templates that can host every recommended pattern (the common case — named
// patterns expand into a shape_grid, which any title/body/blank-title canvas
// supports).
func swapInfeasiblePatterns(reg *patterns.Registry, tc *generator.TemplateSupportContext, slides []Slide, brief, audience string) {
	for i := range slides {
		if slides[i].RecommendedPattern == "" || patternFeasibleForTemplate(tc, slides[i].RecommendedPattern) {
			continue
		}
		for _, alt := range computeAlternativesForSlot(reg, slides, i, brief, audience) {
			if !patternFeasibleForTemplate(tc, alt.PatternName) {
				continue
			}
			old := slides[i].RecommendedPattern
			slides[i].RecommendedPattern = alt.PatternName
			slides[i].Rationale = fmt.Sprintf("template feasibility: %q is unsupported by this template; substituted supported alternative %q", old, alt.PatternName)
			break
		}
	}
}

// annotatePlanTemplateSupport sets template_support on every slide (for its
// recommended pattern) and on every alternative, using the same shared helper
// recommend_visual uses so the two tools agree for identical template
// constraints.
func annotatePlanTemplateSupport(tc *generator.TemplateSupportContext, slides []Slide) {
	for i := range slides {
		if slides[i].RecommendedPattern == "" {
			// Structural title/closing slide: vet the layout it needs instead.
			slides[i].TemplateSupport = tc.Support(patterns.VisualCategoryPlaceholder, slides[i].Layout, nil)
			continue
		}
		slides[i].TemplateSupport = tc.Support(patterns.VisualCategoryPattern, slides[i].RecommendedPattern, nil)
		for j := range slides[i].Alternatives {
			slides[i].Alternatives[j].TemplateSupport = tc.Support(patterns.VisualCategoryPattern, slides[i].Alternatives[j].PatternName, nil)
		}
	}
}

// distributeRoles allocates slide indices to narrative roles based on the arc.
// distributeRoles lays the budget out across the narrative arc. reg and brief
// bound each role by what it can actually fill (see role_capacity.go); the
// returned note is non-empty when the plan came back shorter than the budget,
// and names why.
func distributeRoles(reg *patterns.Registry, brief string, budget int) ([]string, string) {
	roles := make([]string, budget)

	// Always reserve first and last slots.
	roles[0] = "opening"
	roles[budget-1] = "closing"

	// Distribute remaining slots proportionally.
	remaining := budget - 2
	if remaining <= 0 {
		return roles, ""
	}

	order, counts := proportionalRoleCounts(remaining)

	// Bound each role by the patterns and the brief content behind it, moving
	// the surplus to roles with headroom (go-slide-creator-whp97).
	counts, note := applyRoleCapacity(reg, brief, counts, order)

	// Lay out roles in arc order, dropping any slot the capacity pass removed.
	idx := 1
	for _, role := range order {
		for range counts[role] {
			if idx < budget-1 {
				roles[idx] = role
				idx++
			}
		}
	}
	planned := idx

	// Fill any gaps left inside the planned range.
	for i := 1; i < planned; i++ {
		if roles[i] == "" {
			roles[i] = "evidence"
		}
	}
	if planned == budget-1 {
		return roles, note
	}
	// The capacity pass gave slots back: close up and end with the closing
	// slide, so the plan is shorter than the budget rather than padded.
	short := make([]string, 0, planned+1)
	short = append(short, roles[:planned]...)
	return append(short, "closing"), note
}

// proportionalRoleCounts splits the middle slots across the narrative arc by
// each role's declared fraction, giving any rounding remainder to evidence. It
// returns the roles in arc order plus their counts.
func proportionalRoleCounts(remaining int) ([]string, map[string]int) {
	middleArc := standardArc[1 : len(standardArc)-1] // skip opening/closing
	totalFraction := 0.0
	for _, a := range middleArc {
		totalFraction += a.fraction
	}

	order := make([]string, 0, len(middleArc))
	counts := make(map[string]int, len(middleArc))
	assigned := 0
	for i, a := range middleArc {
		count := int(math.Round(float64(remaining) * a.fraction / totalFraction))
		if count < 1 && remaining-assigned > len(middleArc)-i {
			count = 1
		}
		if assigned+count > remaining {
			count = remaining - assigned
		}
		if count <= 0 {
			continue
		}
		order = append(order, a.role)
		counts[a.role] = count
		assigned += count
	}

	// Any rounding remainder goes to evidence, which carries detail.
	if assigned < remaining {
		if _, ok := counts["evidence"]; !ok {
			order = append(order, "evidence")
		}
		counts["evidence"] += remaining - assigned
	}
	return order, counts
}

// assignPatterns picks a pattern for each narrative role slot.
func assignPatterns(reg *patterns.Registry, brief, audience string, roleSlots []string, mustInclude []string) []Slide {
	slides := make([]Slide, len(roleSlots))
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
		slides[i].Layout = layoutForRole(role)
		slides[i].ContentSeed = contentSeedForRole(role, brief, i, len(roleSlots))

		// Title and closing slides use the template's structural layouts;
		// a pattern there fights the layout's own title/subtitle treatment.
		if isStructuralRole(role) {
			slides[i].Rationale = fmt.Sprintf("%s slide: use the template's %q layout with no pattern", role, slides[i].Layout)
			continue
		}

		// Check if we should place a must_include pattern here.
		if pat := pickMustInclude(role, mustInclude, mustIncludeUsed, patternList); pat != "" {
			slides[i].RecommendedPattern = pat
			slides[i].Rationale = mustIncludeRationale
			mustIncludeUsed[pat] = true
			usedPatterns = append(usedPatterns, pat)
			continue
		}

		// Comparison slots draw only from the comparison family.
		if role == "comparison" {
			slides[i].RecommendedPattern = pickComparisonPattern(brief, usedPatterns, "")
			slides[i].Rationale = "comparison slot: two-sided comparison pattern"
			usedPatterns = append(usedPatterns, slides[i].RecommendedPattern)
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
			slides[bestIdx].Rationale = mustIncludeRationale
			mustIncludeUsed[mi] = true
		}
	}

	return slides
}

// pickComparisonPattern chooses a comparison-family pattern for a comparison
// slot. A brief that talks about a before/after or current/future state gets
// before-after first; otherwise comparison-2col. The least-used family member
// wins so two comparison slots alternate; exclude (when non-empty) is never
// returned unless it is the only option.
func pickComparisonPattern(brief string, used []string, exclude string) string {
	lower := strings.ToLower(brief)
	order := comparisonFamily
	for _, kw := range []string{"before", "after", "current state", "future state", "as-is", "to-be", "transformation"} {
		if strings.Contains(lower, kw) {
			order = []string{"before-after", "comparison-2col"}
			break
		}
	}
	best := ""
	bestUses := math.MaxInt
	for _, name := range order {
		if name == exclude {
			continue
		}
		n := 0
		for _, u := range used {
			if u == name {
				n++
			}
		}
		if n < bestUses {
			best, bestUses = name, n
		}
	}
	if best == "" {
		return order[0]
	}
	return best
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
		return "Title and context: " + truncateSeed(normalizeBriefForSeed(brief), 80)
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
func enforceRhythm(reg *patterns.Registry, slides []Slide, brief string) []Slide {
	// Pass 1: Break pattern runs of 3+.
	slides = breakLongRuns(reg, slides)

	// Pass 2: Ensure emphasis slides every ~5 slides.
	slides = ensureEmphasis(reg, slides)

	// Pass 3: Break any new runs introduced by emphasis injection.
	slides = breakLongRuns(reg, slides)

	// Pass 4: Cap emphasis patterns at ceil(n/5). Replacements are chosen to
	// differ from both neighbours, so this pass cannot create a new run.
	slides = capEmphasis(reg, slides, brief)

	// Pass 5: Cap how often one pattern FAMILY can appear at all. Breaking
	// runs only looks at neighbours, so a 20-slide plan came back with
	// comparison-2col four times and arch-stack three times, spread out — a
	// deck that reads as one idea repeated (go-slide-creator-8fq4).
	slides = capPatternRepeats(reg, slides)

	// Pass 6: A repeat replacement could have created a new run.
	slides = breakLongRuns(reg, slides)

	return slides
}

// maxPatternRepeats is how many slides in one deck may use the same pattern
// family. Two is a callback; three is a rut.
const maxPatternRepeats = 2

// patternFamily groups patterns that read as the same slide to an audience,
// so the cap counts what the room sees rather than what the registry calls it.
// kpi-2up through kpi-6up are one visual idea at different counts, and a
// pattern's "-compact" variant is the same picture in less height.
func patternFamily(name string) string {
	switch {
	case name == "":
		return ""
	case strings.HasPrefix(name, "kpi-"):
		return "kpi"
	case strings.HasSuffix(name, "-compact"):
		return strings.TrimSuffix(name, "-compact")
	default:
		return name
	}
}

// capPatternRepeats replaces the excess slides of any over-used pattern family
// with something else the slot can host. must_include placements are kept, and
// the earliest uses win: a repeat later in the deck is the one that reads as a
// rut.
func capPatternRepeats(reg *patterns.Registry, slides []Slide) []Slide {
	// The emphasis cap has already run; a replacement must not spend the deck's
	// emphasis budget to fix a repeat.
	emphasisBudget := MaxEmphasisSlides(len(slides))
	emphasisUsed := 0
	for _, s := range slides {
		if emphasisPatterns[s.RecommendedPattern] {
			emphasisUsed++
		}
	}

	// Slots whose pattern is fixed are counted FIRST and never replaced: a
	// must_include placement, and a comparison slot, which may only hold the
	// comparison family. Counting them last let an ordinary slide earlier in
	// the deck spend the family the comparison slot needs, leaving the repeat
	// nowhere to go (go-slide-creator-whp97).
	locked := make([]bool, len(slides))
	seen := map[string]int{}
	for i := range slides {
		family := patternFamily(slides[i].RecommendedPattern)
		if family == "" {
			continue
		}
		if slides[i].Rationale == mustIncludeRationale || slides[i].NarrativeRole == "comparison" {
			locked[i] = true
			seen[family]++
		}
	}

	for i := range slides {
		family := patternFamily(slides[i].RecommendedPattern)
		if family == "" || locked[i] {
			continue
		}
		seen[family]++
		if seen[family] <= maxPatternRepeats {
			continue
		}

		emphasisRoom := emphasisUsed < emphasisBudget || emphasisPatterns[slides[i].RecommendedPattern]
		replacement := findRepeatReplacement(reg, slides, i, seen, emphasisRoom)
		if replacement == "" || patternFamily(replacement) == family {
			continue
		}
		if emphasisPatterns[replacement] != emphasisPatterns[slides[i].RecommendedPattern] {
			if emphasisPatterns[replacement] {
				emphasisUsed++
			} else {
				emphasisUsed--
			}
		}
		seen[family]--
		seen[patternFamily(replacement)]++
		slides[i].RecommendedPattern = replacement
		slides[i].Rationale = fmt.Sprintf("variety: %s already appears %d times in this deck", family, maxPatternRepeats)
	}
	return slides
}

// findRepeatReplacement picks a pattern for a slot whose family is already at
// its cap, preferring one this deck has not used at all.
func findRepeatReplacement(reg *patterns.Registry, slides []Slide, idx int, seen map[string]int, emphasisRoom bool) string {
	recent := make([]string, 0, len(slides))
	for _, s := range slides {
		recent = append(recent, s.RecommendedPattern)
	}
	opts := &patterns.RecommendOptions{
		RecentPatterns: recent,
		PreferVariety:  true,
		SlideIndex:     idx,
	}
	// Ask for a deep candidate list: the first few are often the families this
	// deck has already spent, and the point here is to find one it has not.
	// The list is as deep as the registry, because a 20-slide plan can spend a
	// dozen families before this runs and a short list then returns nothing at
	// all — which is how repeats reached rhythm_check instead of being fixed
	// (go-slide-creator-whp97).
	role := slides[idx].NarrativeRole
	rec := patterns.Recommend(reg, buildIntent(role, "", ""), nil, len(reg.List()), opts)

	// A comparison slot may only hold the comparison family: a wider search
	// would "fix" a repeat by putting a ranked bar chart where the deck
	// promised two sides (go-slide-creator-xmpb's invariant).
	inRole := func(name string) bool {
		if role != "comparison" {
			return true
		}
		return containsStr(comparisonFamily, name)
	}
	usable := func(name string) bool {
		return inRole(name) && (emphasisRoom || !emphasisPatterns[name])
	}
	names := make([]string, 0, len(rec.Candidates))
	for _, c := range rec.Candidates {
		names = append(names, c.PatternName)
	}
	// The recommender ranks by intent and can still leave whole families out of
	// its answer. Every pattern whose taxonomy covers this role is a legitimate
	// last resort — a fresh family in the right role beats a third repeat.
	var patternList []patInfo
	for _, p := range reg.List() {
		patternList = append(patternList, patInfo{name: p.Name(), taxonomy: p.Taxonomy()})
	}
	names = append(names, taxonomyFallbackCandidates(role, patternList, nil)...)

	// First choice: a family this deck has not used.
	for _, name := range names {
		if seen[patternFamily(name)] == 0 && usable(name) {
			return name
		}
	}
	// Second: any family still under the cap.
	for _, name := range names {
		if seen[patternFamily(name)] < maxPatternRepeats && usable(name) {
			return name
		}
	}
	return ""
}

// capEmphasis demotes emphasis-pattern slides beyond MaxEmphasisSlides(n).
// must_include placements are always kept; then dedicated "emphasis" role
// slots, then any other slot, in slide order — and a kept emphasis slide is
// never adjacent to another. Demoted slides get a non-emphasis pattern that
// differs from both neighbours; a demoted "emphasis"-role slot becomes an
// "evidence" slot.
func capEmphasis(reg *patterns.Registry, slides []Slide, brief string) []Slide {
	limit := MaxEmphasisSlides(len(slides))
	kept := make(map[int]bool)
	adjacentKept := func(i int) bool { return kept[i-1] || kept[i+1] }

	var candidates []int
	for i, s := range slides {
		if !emphasisPatterns[s.RecommendedPattern] {
			continue
		}
		if s.Rationale == mustIncludeRationale {
			kept[i] = true
			continue
		}
		candidates = append(candidates, i)
	}
	// Priority: emphasis-role slots first, then others; stable by index.
	sort.SliceStable(candidates, func(a, b int) bool {
		ea := slides[candidates[a]].NarrativeRole == "emphasis"
		eb := slides[candidates[b]].NarrativeRole == "emphasis"
		return ea && !eb
	})
	var demote []int
	for _, i := range candidates {
		if len(kept) < limit && !adjacentKept(i) {
			kept[i] = true
			continue
		}
		demote = append(demote, i)
	}
	sort.Ints(demote)

	for _, i := range demote {
		old := slides[i].RecommendedPattern
		if slides[i].NarrativeRole == "emphasis" {
			slides[i].NarrativeRole = "evidence"
			slides[i].ContentSeed = contentSeedForRole("evidence", brief, i, len(slides))
		}
		slides[i].RecommendedPattern = nonEmphasisReplacement(reg, slides, i)
		slides[i].Rationale = fmt.Sprintf("emphasis cap: %q demoted — at most %d emphasis slides (stat-hero / pull-quote / kpi-inline) per %d-slide deck", old, limit, len(slides))
	}
	return slides
}

// nonEmphasisReplacement picks a variety-aware, non-emphasis pattern for slot
// idx that differs from both neighbours' patterns.
func nonEmphasisReplacement(reg *patterns.Registry, slides []Slide, idx int) string {
	avoid := map[string]bool{slides[idx].RecommendedPattern: true}
	if idx > 0 {
		avoid[slides[idx-1].RecommendedPattern] = true
	}
	if idx+1 < len(slides) {
		avoid[slides[idx+1].RecommendedPattern] = true
	}
	ok := func(name string) bool {
		return name != "" && !avoid[name] && !emphasisPatterns[name]
	}
	if slides[idx].NarrativeRole == "comparison" {
		for _, name := range comparisonFamily {
			if ok(name) {
				return name
			}
		}
	}
	recent := make([]string, 0, len(slides))
	for _, s := range slides {
		recent = append(recent, s.RecommendedPattern)
	}
	rec := patterns.Recommend(reg, buildIntent(slides[idx].NarrativeRole, "", ""), nil, 10, &patterns.RecommendOptions{
		RecentPatterns: recent,
		PreferVariety:  true,
		SlideIndex:     idx,
	})
	for _, c := range rec.Candidates {
		if ok(c.PatternName) {
			return c.PatternName
		}
	}
	for _, name := range []string{"card-grid", "icon-row", "comparison-2col"} {
		if ok(name) {
			return name
		}
	}
	return "card-grid"
}

// breakLongRuns detects consecutive runs of 3+ and swaps the middle slide.
func breakLongRuns(reg *patterns.Registry, slides []Slide) []Slide {
	for iteration := 0; iteration < 3; iteration++ { // max 3 passes to converge
		changed := false
		runStart := 0
		for i := 1; i <= len(slides); i++ {
			if i < len(slides) && slides[i].RecommendedPattern == slides[runStart].RecommendedPattern {
				continue
			}
			runLen := i - runStart
			if runLen >= 3 && slides[runStart].RecommendedPattern != "" {
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
func findBreakPattern(reg *patterns.Registry, slides []Slide, idx int) string {
	current := slides[idx].RecommendedPattern
	role := slides[idx].NarrativeRole

	if role == "comparison" {
		used := make([]string, 0, len(slides))
		for _, s := range slides {
			used = append(used, s.RecommendedPattern)
		}
		return pickComparisonPattern("", used, current)
	}

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
func ensureEmphasis(reg *patterns.Registry, slides []Slide) []Slide {
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
		// Don't replace opening/closing or a comparison slot.
		if isStructuralRole(slides[i].NarrativeRole) || slides[i].NarrativeRole == "comparison" {
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
func computeRhythmCheck(slides []Slide) RhythmCheck {
	longestRun := 1
	currentRun := 1
	for i := 1; i < len(slides); i++ {
		if slides[i].RecommendedPattern != "" && slides[i].RecommendedPattern == slides[i-1].RecommendedPattern {
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
	families := map[string]int{}
	for _, s := range slides {
		if s.RecommendedPattern != "" {
			unique[s.RecommendedPattern] = true
			families[patternFamily(s.RecommendedPattern)]++
		}
	}

	var repeated []string
	for family, n := range families {
		if n > maxPatternRepeats {
			repeated = append(repeated, fmt.Sprintf("%s x%d", family, n))
		}
	}
	sort.Strings(repeated)

	return RhythmCheck{
		LongestPatternRun: longestRun,
		HasEmphasis:       emphasisCount > 0,
		EmphasisCount:     emphasisCount,
		PatternVariety:    len(unique),
		RepeatedFamilies:  repeated,
	}
}

// --- Helpers ---

// TruncateBrief shortens s to at most maxLen runes, appending an ellipsis when
// truncation occurs. Exported so make_deck can derive slide titles with the
// same budget as the planner's content seeds.
//
// It counts and cuts RUNES. Cutting bytes at a fixed offset both under-counts
// a brief written in anything but ASCII and can slice a multi-byte character
// in half — a brief saying "ARR reached €12.5m" is the ordinary case, not an
// exotic one (go-slide-creator-vmiy).
func TruncateBrief(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return strings.TrimRight(string(runes[:maxLen-3]), " \t") + "..."
}

// truncateSeed truncates a seed and then closes what the cut left open: a
// bracket the ellipsis lands inside would otherwise ship "(up 31…" onto a
// slide (go-slide-creator-vmiy).
func truncateSeed(s string, maxLen int) string {
	truncated := TruncateBrief(s, maxLen)
	body := strings.TrimSuffix(truncated, "...")
	if body == truncated {
		return truncated
	}
	balanced := balanceFactBrackets(body)
	if balanced == body || balanced == "" {
		return truncated
	}
	return balanced + "..."
}

// normalizeBriefForSeed flattens a brief into one line of prose: list markers
// dropped, whitespace runs collapsed. A brief written as a bulleted list used
// to reach the opening slide's seed complete with its newlines and "- "
// markers, which an agent then copied onto the slide (go-slide-creator-vmiy).
func normalizeBriefForSeed(brief string) string {
	lines := strings.FieldsFunc(brief, func(r rune) bool { return r == '\n' || r == '\r' })
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(factListMarker.ReplaceAllString(strings.TrimSpace(line), ""))
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
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

// attachSlidePredictions enriches each slide in-place with predicted cell
// budgets, predicted fit findings, ranked alternative patterns, and a fillable
// skeleton. The render-coupled forecasts (cell budgets, fit findings) come from
// the injected predictor; a nil predictor skips them. Alternatives and the
// skeleton are pure planning and always populated.
func attachSlidePredictions(reg *patterns.Registry, slides []Slide, brief, audience string, predictor Predictor) {
	for i := range slides {
		if slides[i].Layout == "" {
			slides[i].Layout = layoutForRole(slides[i].NarrativeRole)
		}
		if slides[i].RecommendedPattern == "" {
			// Structural title / closing slide: no pattern, so no pattern
			// forecasts or alternatives — just a layout skeleton.
			slides[i].SuggestedPattern = ""
			slides[i].Skeleton = structuralSkeleton(slides[i].Layout)
			continue
		}
		if predictor != nil {
			slides[i].PredictedCellBudgets = predictor.CellBudgets(slides[i].RecommendedPattern)
			slides[i].PredictedFindings = predictor.FitFindings(slides[i].RecommendedPattern, i)
		}
		slides[i].Alternatives = computeAlternativesForSlot(reg, slides, i, brief, audience)

		slides[i].SuggestedPattern = slides[i].RecommendedPattern
		if len(slides[i].Alternatives) > 0 {
			slides[i].SuggestedPatternFallback = slides[i].Alternatives[0].PatternName
		}
		if skel, err := patterns.SkeletonForPattern(reg, slides[i].RecommendedPattern, slides[i].NarrativeRole); err == nil {
			slides[i].Skeleton = withLayoutID(skel, slides[i].Layout)
		}
	}
}

// structuralSkeleton builds the fillable skeleton for a pattern-less title or
// closing slide: the layout plus title and subtitle placeholders (the portable
// placeholder IDs both canonical layouts carry).
func structuralSkeleton(layoutID string) json.RawMessage {
	slide := map[string]any{
		"layout_id": layoutID,
		"content": []map[string]any{
			{"placeholder_id": "title", "type": "text", "text_value": patterns.FillPlaceholder},
			{"placeholder_id": "subtitle", "type": "text", "text_value": patterns.FillPlaceholder},
		},
	}
	out, err := json.Marshal(slide)
	if err != nil {
		return nil
	}
	return out
}

// withLayoutID rewrites a pattern skeleton's layout_id so it always agrees with
// the plan's Slide.Layout. Returns the input unchanged if it cannot be decoded.
func withLayoutID(skel json.RawMessage, layoutID string) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(skel, &m); err != nil {
		return skel
	}
	lid, err := json.Marshal(layoutID)
	if err != nil {
		return skel
	}
	m["layout_id"] = lid
	out, err := json.Marshal(m)
	if err != nil {
		return skel
	}
	return out
}

// computeAlternativesForSlot returns the next-best ranked patterns for this
// slot after the recommended one, excluding patterns already used on adjacent
// slides to preserve the rhythm guarantees.
func computeAlternativesForSlot(reg *patterns.Registry, slides []Slide, idx int, brief, audience string) []Alternative {
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
	rec := patterns.Recommend(reg, intent, nil, MaxAlternatives+2, opts)

	out := make([]Alternative, 0, MaxAlternatives)
	seen := map[string]bool{current: true}
	for _, c := range rec.Candidates {
		if seen[c.PatternName] {
			continue
		}
		seen[c.PatternName] = true
		out = append(out, Alternative{
			PatternName: c.PatternName,
			Score:       c.Score,
			Rationale:   c.Rationale,
		})
		if len(out) >= MaxAlternatives {
			break
		}
	}

	// Fall back to taxonomy-matching patterns when Recommend returns too few
	// candidates (e.g. niche roles where rule-based scoring drops below the
	// threshold).
	if len(out) < MaxAlternatives {
		var patternList []patInfo
		for _, p := range reg.List() {
			patternList = append(patternList, patInfo{name: p.Name(), taxonomy: p.Taxonomy()})
		}
		for _, fallback := range taxonomyFallbackCandidates(role, patternList, recent) {
			if seen[fallback] {
				continue
			}
			seen[fallback] = true
			out = append(out, Alternative{
				PatternName: fallback,
				Score:       0,
				Rationale:   "taxonomy fallback for " + role,
			})
			if len(out) >= MaxAlternatives {
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
