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
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
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
	// Compose is the envelope's region structure. When nil on a compose
	// slide the fingerprint falls back to the bare "compose" family.
	Compose *Compose
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

	// Narrative-structure signals (go-slide-creator-hl17m). All optional: a
	// projection that leaves them zero simply skips the narrative rules.

	// Role is the slide's structural role: "title", "section", "closing",
	// "agenda", or "" for an ordinary content slide.
	Role string
	// Title is the slide's headline text.
	Title string
	// Text is the slide's visible body text (bullets, text, pattern values),
	// used to derive content-aware break suggestions.
	Text string
	// HasTakeaway / HasSource report a so-what line and a source note.
	HasTakeaway bool
	HasSource   bool
	// PatternAccent is the accent a pattern slide resolves to from the deck's
	// accent_strategy (or the pattern's explicit accent override). It feeds
	// accent_balance on the pattern / DeckSpec path, where no raw cell fill
	// exists to read.
	PatternAccent string
	// SolidAccentCells counts raw shape_grid cells filled with a solid
	// (opaque) accent colour.
	SolidAccentCells int
	// Appendix marks back matter (after an Appendix / Backup divider, or in a
	// structure section marked appendix). Backup pages neither form nor
	// extend a pattern run and do not count toward bullets_heavy
	// (go-slide-creator-khzni).
	Appendix bool
	// Motif is the visual motif the slide draws (a patterns.Motif value:
	// "tiles", "open-columns", "table", "chart", …), resolved by the caller
	// from the pattern and its explicit style. Empty lets the analyzer derive
	// it from the pattern's default look or the slide's content kinds; a
	// hand-built grid or a composed slide has none (go-slide-creator-rd7oj).
	Motif string
	// MotifVariant tells two slides of a distinctive motif apart: the chart or
	// diagram type. Three different charts are not a look-alike run.
	MotifVariant string
}

// Compose is the structural projection of a compose envelope: its direction
// and its regions in document order (go-slide-creator-dzv7d).
type Compose struct {
	// Direction is "vertical" or "horizontal".
	Direction string
	Regions   []ComposeRegion
}

// ComposeRegion is one segment of a compose envelope: a leaf pattern, a
// chart or diagram, or a nested envelope.
type ComposeRegion struct {
	// Visual is the leaf pattern name, or "chart" / "diagram" for a diagram
	// segment. Ignored when Nested is set.
	Visual string
	// SizePct is the segment's explicit share; 0 takes an equal part of the
	// remainder, as the compose engine allocates it.
	SizePct float64
	// Nested is a compose envelope inside this segment.
	Nested *Compose
}

// SlideInfo describes the visual fingerprint of a single slide.
type SlideInfo struct {
	SlideIndex               int    `json:"slide_index"`
	Pattern                  string `json:"pattern"`                     // pattern name, "shape_grid", "content", or slide_type
	DensityClass             string `json:"density_class"`               // "low", "med", "high"
	AccentRole               string `json:"accent_role"`                 // primary accent color used, or "none"
	DominantVisual           string `json:"dominant_visual"`             // "chart", "diagram", "table", "text", "grid", "pattern"
	Motif                    string `json:"motif"`                       // visual motif ("tiles", "open-columns", "table", …) or "none"
	WithinSlideAccentVariety int    `json:"within_slide_accent_variety"` // distinct accent slots used across cells
	cellCount                int    // internal: total cells in shape_grid (not serialized)
	appendix                 bool   // internal: back matter, outside the run checks
}

// PatternRun describes a consecutive run of one visual family. Name is the
// canonical family name (and remains the pattern name for ungrouped patterns).
// Len counts exhibits: the parts of a continued slide ("(1/2)", "(2/2)") are
// one (go-slide-creator-xy51l). Start and End are slide indices, End
// inclusive.
type PatternRun struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Len   int    `json:"len"`

	// members are the first slide index of each exhibit in the run.
	members []int
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
	// MotifRuns lists runs of 3+ consecutive content slides drawn in one
	// visual motif, whatever patterns drew them. MotifShare is each motif's
	// share of the content slides; DominantMotif names the one covering more
	// than half of them, when there is one (go-slide-creator-rd7oj).
	MotifRuns     []MotifRun         `json:"motif_runs"`
	MotifShare    map[string]float64 `json:"motif_share"`
	DominantMotif string             `json:"dominant_motif,omitempty"`
}

// Recommendation is an actionable suggestion to improve deck rhythm.
type Recommendation struct {
	// Code classifies the recommendation (break_run, missing_executive_summary,
	// ...); see SKILL.md. Stable for programmatic handling.
	Code             string   `json:"code,omitempty"`
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

	// motif is what the motif and continuation checks cost the score.
	motif motifPenalty
}

// Analyze performs the core rhythm analysis on the slide projections.
func Analyze(slides []Slide) *Result {
	perSlide := make([]SlideInfo, len(slides))

	for i, s := range slides {
		perSlide[i] = fingerprint(i, s)
	}

	// A nil slice marshals to JSON null, but the wire schema declares these as
	// arrays — three success responses were schema-invalid for exactly this
	// reason, and a validating MCP client rejects them
	// (go-slide-creator-vtqo). Empty-but-present is also the friendlier shape:
	// a caller can iterate without a nil check.
	titles := make([]string, len(slides))
	for i, s := range slides {
		titles[i] = s.Title
	}
	units := ContinuationUnits(titles)
	runs := detectPatternRuns(perSlide, units)
	if runs == nil {
		runs = []PatternRun{}
	}
	mSlides := motifSlides(slides, perSlide, units)
	motifRuns := detectMotifRuns(mSlides)
	if motifRuns == nil {
		motifRuns = []MotifRun{}
	}
	shares, dominant, dominantSlides, contentSlides := motifShares(slides, mSlides)
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
			MotifRuns:           motifRuns,
			MotifShare:          shares,
			DominantMotif:       motifLabel(dominant),
		},
	}

	result.Recommendations = generateRecommendations(slides, perSlide, runs, dd)
	motifRecs, penalty := motifRecommendations(slides, mSlides, runs, motifRuns, dominant, dominantSlides, contentSlides)
	result.Recommendations = append(result.Recommendations, motifRecs...)
	result.motif = penalty
	if result.Recommendations == nil {
		result.Recommendations = []Recommendation{}
	}
	result.CompositionScore = computeCompositionScore(result)

	return result
}

// fingerprint extracts the visual fingerprint of a single slide.
func fingerprint(idx int, s Slide) SlideInfo {
	info := SlideInfo{
		SlideIndex:     idx,
		DominantVisual: dominantVisual(s),
	}

	// Determine pattern name.
	switch {
	case s.HasCompose:
		// Every composed slide used to be the one family "compose", so three
		// structurally different envelopes read as a three-slide run and drew
		// break_run advice (go-slide-creator-dzv7d). Name the envelope by its
		// structure instead.
		info.Pattern = composeFingerprint(s.Compose)
	case s.PatternName != "":
		info.Pattern = s.PatternName
	case s.HasShapeGrid:
		// A hand-authored grid has no pattern name, and calling every one of
		// them "shape_grid" made six structurally different layouts read as a
		// six-slide run — the flaw that would have turned the composition gate
		// into a false block on decks that are genuinely varied
		// (go-slide-creator-xx9i). Fingerprint by the grid's shape instead:
		// two 3x1 grids in a row still look alike and still count as a run, a
		// 3x1 followed by a 2x2 does not.
		info.Pattern = shapeGridFingerprint(s)
	default:
		// Use slide_type if available, else infer from content.
		if s.SlideType != "" && s.SlideType != "content" {
			info.Pattern = s.SlideType
		} else {
			info.Pattern = "content"
		}
	}

	info.DensityClass = densityClass(s)
	info.AccentRole = primaryAccent(s)
	info.WithinSlideAccentVariety = countDistinctAccents(s)
	info.cellCount = s.CellCount
	info.appendix = s.Appendix
	if info.Motif, _ = slideMotif(s); info.Motif == "" {
		info.Motif = "none"
	}

	return info
}

// FingerprintKey is the canonical visual identity used by both deck rhythm
// and candidate scoring. Family alone is insufficient: a content chart and
// a content bullet slide are different, even when both use slide_type=content.
func FingerprintKey(s Slide) string {
	return visualFingerprint(fingerprint(0, s))
}

func visualFingerprint(s SlideInfo) string {
	return visualFamily(s.Pattern) + "|" + s.DominantVisual
}

// shapeGridFingerprint names a raw shape_grid by its shape, so two grids count
// as the same "pattern" only when they lay out the same way.
func shapeGridFingerprint(s Slide) string {
	if s.Grid == nil || len(s.Grid.Rows) == 0 {
		if s.CellCount > 0 {
			return fmt.Sprintf("shape_grid:%dcell", s.CellCount)
		}
		return "shape_grid"
	}
	cols := make([]string, 0, len(s.Grid.Rows))
	for _, row := range s.Grid.Rows {
		cols = append(cols, strconv.Itoa(len(row.Cells)))
	}
	return "shape_grid:" + strings.Join(cols, "-")
}

// composeDominantMargin is how far above an equal split a region's share must
// sit to mark it as dominant: 65/35 marks the larger half of a two-region
// envelope, 50/50 or 55/45 does not.
const composeDominantMargin = 0.125

// composeFingerprint names a compose envelope by coarse structure: direction,
// the visual family of each region (nested envelopes recursively), and which
// region dominates the space. Regions are sorted, so the same regions in a
// different order, or a trivially different split, keep one identity and
// still form a run; a different direction, region mix or dominant region
// does not. Example: "compose:v[kpi*+pull-quote]".
func composeFingerprint(c *Compose) string {
	if c == nil || len(c.Regions) == 0 {
		return "compose"
	}
	dir := "v"
	if c.Direction == "horizontal" {
		dir = "h"
	}
	shares := composeShares(c.Regions)
	equal := 1.0 / float64(len(c.Regions))
	tokens := make([]string, len(c.Regions))
	for i, r := range c.Regions {
		token := visualFamily(r.Visual)
		if r.Nested != nil {
			token = composeFingerprint(r.Nested)
		}
		if token == "" {
			token = "region"
		}
		if len(c.Regions) > 1 && shares[i] >= equal+composeDominantMargin {
			token += "*"
		}
		tokens[i] = token
	}
	sort.Strings(tokens)
	return "compose:" + dir + "[" + strings.Join(tokens, "+") + "]"
}

// composeShares resolves each region's fraction of the envelope the way the
// compose engine allocates it: explicit size_pct values first, the remainder
// split equally among regions without one, then normalised to sum to 1.
func composeShares(regions []ComposeRegion) []float64 {
	shares := make([]float64, len(regions))
	explicit, implicit := 0.0, 0
	for i, r := range regions {
		if r.SizePct > 0 {
			shares[i] = r.SizePct
			explicit += r.SizePct
		} else {
			implicit++
		}
	}
	if implicit > 0 {
		rest := math.Max(100-explicit, 0) / float64(implicit)
		for i := range shares {
			if shares[i] == 0 {
				shares[i] = rest
			}
		}
	}
	total := 0.0
	for _, v := range shares {
		total += v
	}
	for i := range shares {
		if total > 0 {
			shares[i] /= total
		} else {
			shares[i] = 1 / float64(len(shares))
		}
	}
	return shares
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
	// Document order is not visual prominence: a caption before a chart
	// should not change the slide's identity. Keep one stable priority.
	for _, want := range []string{"chart", "diagram", "table", "image"} {
		for _, kind := range s.ContentKinds {
			if kind == want {
				return want
			}
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

// primaryAccent returns the first accent color reference on a slide (a raw
// cell fill, else the pattern's resolved accent), or "none".
func primaryAccent(s Slide) string {
	if len(s.CellAccents) > 0 {
		return s.CellAccents[0]
	}
	if s.PatternAccent != "" {
		return s.PatternAccent
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

// visualFamily groups variants that present the same visual idea. Raw grids
// retain their structural fingerprint so distinct layouts do not form a run.
func visualFamily(name string) string {
	if strings.HasPrefix(name, "kpi-") {
		return "kpi"
	}
	base := strings.TrimSuffix(name, "-compact")
	switch base {
	case "card-grid", "stylish-panels", "icon-row", "team-bios", "framework-grid", "contact-directory":
		return "card-grid"
	case "process-flow", "process-grid-2row", "numbered-step-strip", "swimlane", "value-chain":
		return "process-flow"
	default:
		return base
	}
}

// detectPatternRuns finds consecutive runs of the same visual family. units
// maps each slide to the first slide of the exhibit it belongs to
// (ContinuationUnits); the later parts of a continued exhibit extend a run's
// span without adding to its length.
func detectPatternRuns(slides []SlideInfo, units []int) []PatternRun {
	if len(slides) == 0 {
		return nil
	}

	// Appendix back matter gets a per-slide key no other slide shares, so it
	// neither forms nor extends a run.
	keyOf := func(i int) string {
		if slides[i].appendix {
			return fmt.Sprintf("\x00appendix-%d", i)
		}
		return visualFingerprint(slides[i])
	}
	unitOf := func(i int) int {
		if i < len(units) {
			return units[i]
		}
		return i
	}

	var runs []PatternRun
	current := PatternRun{Name: visualFamily(slides[0].Pattern), Start: 0, End: 0, Len: 1, members: []int{0}}
	currentKey := keyOf(0)

	for i := 1; i < len(slides); i++ {
		family := visualFamily(slides[i].Pattern)
		key := keyOf(i)
		if key == currentKey {
			current.End = i
			if unitOf(i) != unitOf(i-1) {
				current.Len++
				current.members = append(current.members, i)
			}
		} else {
			if current.Len >= 2 {
				runs = append(runs, current)
			}
			current = PatternRun{Name: family, Start: i, End: i, Len: 1, members: []int{i}}
			currentKey = key
		}
	}
	if current.Len >= 2 {
		runs = append(runs, current)
	}

	return runs
}

// computeRepetitionIndex measures how repetitive the visual-family sequence is.
// 0.0 = all unique families, approaching 1.0 as one family dominates.
func computeRepetitionIndex(slides []SlideInfo) float64 {
	if len(slides) <= 1 {
		return 0.0
	}

	unique := map[string]bool{}
	for _, s := range slides {
		unique[visualFingerprint(s)] = true
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
func generateRecommendations(inputs []Slide, slides []SlideInfo, runs []PatternRun, dd DensityDistribution) []Recommendation {
	var recs []Recommendation

	for _, run := range runs {
		if run.Len < 3 {
			continue
		}

		// Recommend break points at every 3rd exhibit in the run.
		for offset := 2; offset < len(run.members); offset += 3 {
			insertIdx := run.members[offset]
			recs = append(recs, Recommendation{
				Code:             CodeBreakRun,
				SlideIndex:       insertIdx,
				Message:          fmt.Sprintf("break a %s run (length %d); consider inserting a different pattern at slide %d", run.Name, run.Len, insertIdx+1),
				RecommendedBreak: suggestBreakPatterns(run.Name, inputs[insertIdx]),
			})
		}
	}

	// The old "N cells but only 1 accent: add cell_accent_mode: progressive"
	// rule is gone (go-slide-creator-hl17m): it fired on exactly the
	// restrained slides (card-grid, kpi-5up, framework-grid) and pushed agents
	// to rotate accents across cells, the opposite of one emphasis per slide.
	// Accent heaviness is checked instead.
	recs = append(recs, accentHeavinessRecommendations(inputs)...)

	// Rule: >30% underfilled cells across the deck → recommend adding detail or smaller grids.
	totalCells := dd.UnderfilledCells + dd.OptimalCells + dd.OverflowCells
	if totalCells > 0 {
		underfilledPct := float64(dd.UnderfilledCells) / float64(totalCells) * 100
		if underfilledPct > 30 {
			recs = append(recs, Recommendation{
				Code:             CodeUnderfilledCells,
				SlideIndex:       -1, // deck-level
				Message:          fmt.Sprintf("%.0f%% of cells (%d/%d) are underfilled — add detail text or use smaller grid patterns", underfilledPct, dd.UnderfilledCells, totalCells),
				RecommendedBreak: []string{"kpi-3up", "kpi-2up", "comparison-2col"},
			})
		}
	}

	recs = append(recs, narrativeRecommendations(inputs)...)
	return recs
}

// suggestBreakPatterns ranks the full registered catalog, excluding the run's
// visual family and — so the break actually looks different — the patterns
// that draw the run's motif. A slide's content kinds take precedence over the
// run's usual topic so chart/table slides receive relevant alternatives.
func suggestBreakPatterns(runFamily string, slide Slide) []string {
	intent := runFamily
	priority := map[string]int{"chart": 4, "table": 3, "diagram": 2, "image": 1}
	for _, kind := range slide.ContentKinds {
		if priority[kind] > priority[intent] {
			intent = kind
		}
	}
	if intent == runFamily && slide.SlideType == "comparison" {
		intent = "comparison"
	}
	signals := textSignals(slide.Title + " " + slide.Text)
	preferences := breakPreferences(intent)
	if !specificBreakIntents[intent] {
		// A run of plain slides: what the slide says picks the alternative
		// (numbers -> KPI / stat, options -> comparison, dates -> timeline),
		// not a fixed list (go-slide-creator-hl17m).
		preferences = contentBreakPreferences(signals)
	}
	motif := ""
	if slide.PatternName != "" || slide.Motif != "" {
		motif, _ = slideMotif(slide)
	}
	return rankBreakPatterns(slide, runFamily, motif, signals, preferences)
}

// rankBreakPatterns orders the registered patterns as alternatives for slide:
// preferences first (in order), then its pairs_with siblings, then narrative
// patterns. It leaves out the run's own family, the patterns a composed slide
// already shows, the patterns whose default look is leaveMotif (a break must
// look different, go-slide-creator-rd7oj), and time-based patterns when the
// slide carries no dates.
func rankBreakPatterns(slide Slide, runFamily, leaveMotif string, signals contentSignals, preferences []string) []string {
	preferenceRank := make(map[string]int, len(preferences))
	for i, name := range preferences {
		if _, seen := preferenceRank[name]; !seen {
			preferenceRank[name] = len(preferences) - i
		}
	}

	// A run of composed slides already shows its regions' patterns; breaking
	// it with one of them would repeat the same visual.
	runRegions := map[string]bool{}
	composeRegionFamilies(slide.Compose, runRegions)

	// The distinctive motifs (chart, diagram) differ slide to slide, so
	// another chart is a fair break for a run of one chart type.
	excludeMotif := leaveMotif != "" && !patterns.Motif(leaveMotif).Distinctive()

	registry := patterns.Default()
	pairs := map[string]bool{}
	if current, ok := registry.Get(slide.PatternName); ok {
		for _, name := range current.Taxonomy().PairsWith {
			pairs[name] = true
		}
	}
	type ranked struct {
		name  string
		score int
	}
	var candidates []ranked
	for _, pat := range registry.List() {
		name := pat.Name()
		if family := visualFamily(name); family == runFamily || runRegions[family] {
			continue
		}
		if excludeMotif && string(patterns.PatternMotif(name)) == leaveMotif {
			continue
		}
		// A timeline without dates is decoration (RULES.md): only propose the
		// time-based patterns when the slide carries date-like content.
		if timeBasedPatterns[name] && !signals.dates {
			continue
		}
		score := preferenceRank[name] * 10
		if pairs[name] {
			score += 3
		}
		if pat.Taxonomy().Category == "narrative" {
			score++
		}
		candidates = append(candidates, ranked{name: name, score: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].name < candidates[j].name
	})
	result := make([]string, 0, 3)
	for _, candidate := range candidates {
		if len(result) == 3 {
			break
		}
		result = append(result, candidate.name)
	}
	return result
}

// composeRegionFamilies adds the visual family of every leaf region in c
// (nested envelopes included) to out.
func composeRegionFamilies(c *Compose, out map[string]bool) {
	if c == nil {
		return
	}
	for _, r := range c.Regions {
		if r.Nested != nil {
			composeRegionFamilies(r.Nested, out)
			continue
		}
		if r.Visual != "" {
			out[visualFamily(r.Visual)] = true
		}
	}
}

func breakPreferences(intent string) []string {
	switch intent {
	case "chart":
		return []string{"chart-insights-split", "horizontal-bar-with-callouts", "waterfall-bridge", "stat-hero"}
	case "table":
		return []string{"table-highlight", "comparison-2col", "matrix-2x2", "chart-insights-split"}
	case "diagram", "process-flow":
		return []string{"timeline-horizontal", "phase-roadmap", "journey-maturity-model", "before-after"}
	case "image":
		return []string{"image-text-split", "agenda-with-images", "quote-cluster", "pull-quote"}
	case "kpi":
		return []string{"stat-hero", "horizontal-bar-with-callouts", "chart-insights-split", "table-highlight"}
	case "card-grid", "comparison":
		return []string{"comparison-2col", "before-after", "matrix-2x2", "process-flow"}
	default:
		return []string{"stat-hero", "comparison-2col", "timeline-horizontal", "pull-quote"}
	}
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

	// Look-alike runs no pattern run already accounts for cost the same as a
	// pattern run; a motif that dominates the deck across several patterns
	// and a continued exhibit with a slide between its parts cost 10 each
	// (go-slide-creator-rd7oj, -xy51l).
	for _, run := range r.motif.runs {
		score -= 10
		score -= float64(run.Len-motifRunMinLen) * 5
	}
	if r.motif.dominant {
		score -= 10
	}
	score -= float64(r.motif.gaps) * 10

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
