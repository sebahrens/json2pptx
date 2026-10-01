package rhythm

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Recommendation codes. break_run and underfilled_cells predate the codes;
// the rest are the narrative-structure and accent-heaviness checks added for
// go-slide-creator-hl17m.
const (
	CodeBreakRun                   = "break_run"
	CodeUnderfilledCells           = "underfilled_cells"
	CodeMissingExecutiveSummary    = "missing_executive_summary"
	CodeMissingNextSteps           = "missing_next_steps"
	CodeMissingSections            = "missing_sections"
	CodeEvidenceMissingTakeawaySrc = "evidence_missing_takeaway_or_source"
	CodeBulletsHeavy               = "bullets_heavy"
	CodeAccentHeavySlide           = "accent_heavy_slide"
	CodeStrongAccentRun            = "strong_accent_run"
)

// Thresholds for the narrative and accent rules.
const (
	// execSummaryMinSlides: a deck this long needs the answer up front.
	execSummaryMinSlides = 6
	// nextStepsMinSlides: shorter decks may legitimately end on their point.
	nextStepsMinSlides = 4
	// sectionsMinContentSlides: this many content slides need chapters.
	sectionsMinContentSlides = 10
	// bulletsHeavyMinSlides: this many bullets-only slides anywhere in the
	// deck read as a document, even without a consecutive run.
	bulletsHeavyMinSlides = 3
	// solidAccentCellLimit: a raw grid with this many solid accent cells has
	// no single emphasis left.
	solidAccentCellLimit = 4
	// strongAccentRunLen: consecutive AccentWeight=strong slides.
	strongAccentRunLen = 3
)

// specificBreakIntents are run intents whose break list is fixed by what the
// slide is (a chart, a table, a KPI run…). Every other run — plain content,
// quotes, hero stats — gets alternatives derived from its text.
var specificBreakIntents = map[string]bool{
	"chart": true, "table": true, "diagram": true, "process-flow": true,
	"image": true, "kpi": true, "card-grid": true, "comparison": true,
}

// timeBasedPatterns only make sense with dates on the slide.
var timeBasedPatterns = map[string]bool{
	"timeline-horizontal": true, "phase-roadmap": true, "roadmap-phased": true,
}

// evidencePatterns argue from data and so need a so-what and a source.
var evidencePatterns = map[string]bool{
	"stat-hero": true, "hero-detail": true, "metric-list": true,
	"chart-insights-split": true, "waterfall-bridge": true,
	"horizontal-bar-with-callouts": true,
}

var (
	numberRE = regexp.MustCompile(`(?i)[$€£]\s?\d|\d(\.\d+)?\s?(%|x\b|bn\b|m\b|k\b|pts?\b|pp\b)|\b\d{1,3}(,\d{3})+\b|\b\d+\.\d+\b`)
	dateRE   = regexp.MustCompile(`(?i)\b(19|20)\d{2}\b|\b[qh][1-4]\b|\bfy\s?\d{2,4}\b|\b(january|february|march|april|may|june|july|august|september|october|november|december|jan|feb|mar|apr|jun|jul|aug|sep|sept|oct|nov|dec)\b|\b(milestone|deadline|go-live|launch date)s?\b`)
	optionRE = regexp.MustCompile(`(?i)\b(options?|alternatives?|vs\.?|versus|scenarios?|pros|cons|trade-?offs?|choices?|compare|comparison)\b`)
)

// contentSignals are what a slide's text says it contains.
type contentSignals struct {
	numbers, dates, options bool
}

func textSignals(text string) contentSignals {
	if strings.TrimSpace(text) == "" {
		return contentSignals{}
	}
	return contentSignals{
		numbers: numberRE.MatchString(text),
		dates:   dateRE.MatchString(text),
		options: optionRE.MatchString(text),
	}
}

// contentBreakPreferences ranks alternatives by what the slide says: numbers
// call for a KPI / stat visual, options for a comparison, dates for a
// timeline. Prose with none of these gets the text-shaped patterns, never a
// decorative timeline or an invented number.
func contentBreakPreferences(sig contentSignals) []string {
	var out []string
	if sig.numbers {
		out = append(out, "kpi-3up", "stat-hero", "metric-list")
	}
	if sig.options {
		out = append(out, "comparison-2col", "table-highlight")
	}
	if sig.dates {
		out = append(out, "timeline-horizontal", "phase-roadmap")
	}
	return append(out, "labeled-rows", "text-sidebar", "framework-grid")
}

// isStructural reports whether the slide is navigation rather than argument.
func isStructural(s Slide) bool {
	switch s.Role {
	case "title", "section", "closing", "agenda":
		return true
	}
	return strings.HasPrefix(s.PatternName, "agenda")
}

func lowerTitle(s Slide) string { return strings.ToLower(strings.TrimSpace(s.Title)) }

func isExecSummarySlide(s Slide) bool {
	if s.PatternName == "exec-summary" || s.PatternName == "scqa-summary" {
		return true
	}
	t := lowerTitle(s)
	return strings.Contains(t, "executive summary") || strings.Contains(t, "exec summary")
}

func isNextStepsSlide(s Slide) bool {
	if s.PatternName == "next-steps" {
		return true
	}
	t := lowerTitle(s)
	return strings.Contains(t, "next step") || strings.Contains(t, "decisions requested") ||
		strings.Contains(t, "decision required") || strings.Contains(t, "the ask")
}

func isEvidenceSlide(s Slide) bool {
	if isStructural(s) {
		return false
	}
	if evidencePatterns[s.PatternName] || strings.HasPrefix(s.PatternName, "kpi-") {
		return true
	}
	if s.HasPattern || s.HasShapeGrid || s.HasCompose {
		return false
	}
	v := dominantVisual(s)
	return v == "chart" || v == "table"
}

func isBulletsOnlySlide(s Slide) bool {
	if isStructural(s) || s.HasPattern || s.HasShapeGrid || s.HasCompose {
		return false
	}
	bullets := false
	for _, k := range s.ContentKinds {
		switch k {
		case "bullets", "body_and_bullets", "bullet_groups":
			bullets = true
		case "text":
		default:
			return false
		}
	}
	return bullets
}

// narrativeAware reports whether the projection carries the narrative
// signals at all; a bare structural projection (unit tests, legacy callers)
// skips the narrative rules rather than reporting every deck as headless.
func narrativeAware(inputs []Slide) bool {
	for _, s := range inputs {
		if s.Title != "" || s.Role != "" {
			return true
		}
	}
	return false
}

// narrativeRecommendations checks the deck's argument structure: an
// executive summary up front, a next-steps close, chapters on long decks, a
// so-what and a source on every evidence slide, and not too many
// bullets-only slides (go-slide-creator-hl17m).
func narrativeRecommendations(inputs []Slide) []Recommendation { //nolint:gocognit,gocyclo
	if !narrativeAware(inputs) {
		return nil
	}
	var recs []Recommendation
	n := len(inputs)

	content := 0
	hasExec, hasNext, hasSections := false, false, false
	for _, s := range inputs {
		if !isStructural(s) {
			content++
		}
		if isExecSummarySlide(s) {
			hasExec = true
		}
		if isNextStepsSlide(s) {
			hasNext = true
		}
		if s.Role == "section" || s.Role == "agenda" || strings.HasPrefix(s.PatternName, "agenda") {
			hasSections = true
		}
	}

	if n >= execSummaryMinSlides && !hasExec {
		at := 0
		if inputs[0].Role == "title" {
			at = 1
		}
		recs = append(recs, Recommendation{
			Code:             CodeMissingExecutiveSummary,
			SlideIndex:       at,
			Message:          fmt.Sprintf("a %d-slide deck has no executive summary — put the answer up front at slide %d: 3–5 bold conclusions, each with its evidence (exec-summary pattern / DeckSpec executive_summary)", n, at),
			RecommendedBreak: []string{"exec-summary", "scqa-summary"},
		})
	}

	if n >= nextStepsMinSlides && !hasNext {
		last := n - 1
		msg := fmt.Sprintf("the deck does not close on next steps — end with the decision or actions requested, each with owner and date (next-steps pattern / DeckSpec next_steps), at slide %d", last)
		if t := lowerTitle(inputs[last]); strings.Contains(t, "thank") || strings.Contains(t, "question") || t == "q&a" {
			msg = fmt.Sprintf("slide %d closes on %q — replace or precede it with next steps: the decision or actions requested, each with owner and date (next-steps pattern / DeckSpec next_steps)", last, strings.TrimSpace(inputs[last].Title))
		}
		recs = append(recs, Recommendation{
			Code:             CodeMissingNextSteps,
			SlideIndex:       last,
			Message:          msg,
			RecommendedBreak: []string{"next-steps"},
		})
	}

	if content >= sectionsMinContentSlides && !hasSections {
		recs = append(recs, Recommendation{
			Code:             CodeMissingSections,
			SlideIndex:       -1,
			Message:          fmt.Sprintf("%d content slides with no section dividers or agenda — group them into 2–5 chapters (structure.sections with auto_agenda, or section slides plus an agenda)", content),
			RecommendedBreak: []string{"agenda"},
		})
	}

	for i, s := range inputs {
		if !isEvidenceSlide(s) || (s.HasTakeaway && s.HasSource) {
			continue
		}
		var missing []string
		if !s.HasTakeaway {
			missing = append(missing, "takeaway (what the evidence means)")
		}
		if !s.HasSource {
			missing = append(missing, "source (who, what, when)")
		}
		recs = append(recs, Recommendation{
			Code:             CodeEvidenceMissingTakeawaySrc,
			SlideIndex:       i,
			Message:          fmt.Sprintf("slide %d argues from data but has no %s", i, strings.Join(missing, " and no ")),
			RecommendedBreak: []string{},
		})
	}

	var bulletSlides []int
	var bulletText strings.Builder
	for i, s := range inputs {
		if isBulletsOnlySlide(s) {
			bulletSlides = append(bulletSlides, i)
			bulletText.WriteString(s.Title + " " + s.Text + " ")
		}
	}
	if len(bulletSlides) >= bulletsHeavyMinSlides {
		idx := make([]string, len(bulletSlides))
		for i, v := range bulletSlides {
			idx[i] = fmt.Sprint(v)
		}
		recs = append(recs, Recommendation{
			Code:       CodeBulletsHeavy,
			SlideIndex: -1,
			Message: fmt.Sprintf("%d slides are bullets only (slides %s) — turn the ones whose content has a shape into a visual: numbers → KPI / chart, options → comparison, dates → timeline, a structure → framework",
				len(bulletSlides), strings.Join(idx, ", ")),
			RecommendedBreak: firstN(contentBreakPreferences(textSignals(bulletText.String())), 3),
		})
	}
	return recs
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// accentWeight returns the pattern's taxonomy accent weight ("strong",
// "normal", "subtle"), or "".
func accentWeight(name string) string {
	if name == "" {
		return ""
	}
	if p, ok := patterns.Default().Get(name); ok {
		return p.Taxonomy().AccentWeight
	}
	return ""
}

// isAccentHeavy reports a slide whose visual is dominated by accent colour:
// an AccentWeight=strong pattern, or a raw grid with many solid accent cells.
func isAccentHeavy(s Slide) bool {
	return accentWeight(s.PatternName) == "strong" || s.SolidAccentCells >= solidAccentCellLimit
}

// accentHeavinessRecommendations replaces the old "add more accents" advice:
// one emphasis per slide, and no run of accent-dominated slides
// (go-slide-creator-hl17m).
func accentHeavinessRecommendations(inputs []Slide) []Recommendation {
	var recs []Recommendation
	for i, s := range inputs {
		if s.SolidAccentCells >= solidAccentCellLimit {
			recs = append(recs, Recommendation{
				Code:       CodeAccentHeavySlide,
				SlideIndex: i,
				Message: fmt.Sprintf("slide %d fills %d cells with solid accent colour — keep accent for the one cell the title is about and give the rest a neutral fill (lt2 or a dk1 tint)",
					i, s.SolidAccentCells),
				RecommendedBreak: []string{"shape_grid.rows[].cells[].shape.fill"},
			})
		}
	}
	run := 0
	for i, s := range inputs {
		if !isAccentHeavy(s) {
			run = 0
			continue
		}
		run++
		if run == strongAccentRunLen {
			recs = append(recs, Recommendation{
				Code:       CodeStrongAccentRun,
				SlideIndex: i,
				Message: fmt.Sprintf("slides %d–%d are all accent-heavy (AccentWeight strong) — replace one with a normal or subtle pattern so the emphasis still means something",
					i-strongAccentRunLen+1, i),
				RecommendedBreak: subtleAlternatives(s),
			})
		}
	}
	return recs
}

// subtleAlternatives are content-appropriate patterns whose AccentWeight is
// not strong.
func subtleAlternatives(s Slide) []string {
	sig := textSignals(s.Title + " " + s.Text)
	candidates := append(contentBreakPreferences(sig), "table-highlight", "comparison-2col", "pull-quote")
	var out []string
	seen := map[string]bool{}
	for _, name := range candidates {
		if seen[name] || accentWeight(name) == "strong" || visualFamily(name) == visualFamily(s.PatternName) {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) == 3 {
			break
		}
	}
	return out
}
