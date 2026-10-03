package rhythm

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Motif analysis (go-slide-creator-rd7oj). A pattern run sees kpi-4up,
// stylish-panels and icon-row as three different slides; the audience sees a
// row of open columns three times. Every pattern declares the motif it draws
// (patterns.MotifFor), and the analyzer counts motif runs and each motif's
// share of the content slides next to the pattern runs.

// Recommendation codes of the motif and continuation checks.
const (
	CodeBreakMotifRun           = "break_motif_run"
	CodeMotifDominant           = "motif_dominant"
	CodeContinuationInterrupted = "continuation_interrupted"
)

const (
	// motifRunMinLen is the run length (in exhibits) at which one motif
	// repeated reads as the same slide again.
	motifRunMinLen = 3
	// motifDominantShare is the share of the content slides one motif may
	// cover; above it the deck has one look.
	motifDominantShare = 0.5
	// motifDominantMinContent is the fewest content slides a deck needs for a
	// share to mean anything: two of three is a short deck, not a habit.
	motifDominantMinContent = 4
)

// MotifRun is a run of consecutive content slides drawn in one motif. Len
// counts exhibits: the parts of a continued slide ("(1/2)", "(2/2)") count
// once. Start and End are slide indices, End inclusive.
type MotifRun struct {
	Motif string `json:"motif"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Len   int    `json:"len"`

	// members are the first slide index of each exhibit in the run.
	members []int
}

// slideMotif returns the slide's motif and, for the distinctive motifs, the
// variant that tells two slides of it apart. A projection that sets Motif is
// trusted; otherwise the motif is derived from the pattern's default look or
// the slide's content. Hand-built grids and composed slides have no single
// motif and return "".
func slideMotif(s Slide) (motif, variant string) {
	if s.Motif != "" {
		variant = s.MotifVariant
		if variant == "" {
			variant = s.PatternName
		}
		return s.Motif, variant
	}
	switch {
	case s.HasCompose:
		return "", ""
	case s.PatternName != "":
		return string(patterns.PatternMotif(s.PatternName)), s.PatternName
	case s.HasPattern, s.HasShapeGrid:
		return "", ""
	}
	if isStructural(s) {
		return "", ""
	}
	switch v := dominantVisual(s); v {
	case "chart", "diagram", "table", "image":
		return v, s.MotifVariant
	}
	return string(patterns.MotifText), ""
}

// motifKey is the identity two slides must share to be look-alikes: the
// motif, plus the variant for the motifs whose slides differ from each other
// (three different diagrams are not a run; three bar charts are).
func motifKey(motif, variant string) string {
	if motif == "" {
		return ""
	}
	if patterns.Motif(motif).Distinctive() {
		return motif + ":" + variant
	}
	return motif
}

// motifSlide is the per-slide input of the motif checks.
type motifSlide struct {
	key     string // "" when the slide takes no part (structural, appendix, no motif)
	content bool   // an argument slide: not structural, not back matter
	unit    int    // first slide index of the exhibit the slide belongs to
	visual  string // the pattern-run fingerprint
}

func motifSlides(inputs []Slide, infos []SlideInfo, units []int) []motifSlide {
	out := make([]motifSlide, len(inputs))
	for i, s := range inputs {
		ms := motifSlide{unit: units[i], visual: visualFingerprint(infos[i])}
		ms.content = !isStructural(s) && !s.Appendix
		if ms.content {
			ms.key = motifKey(slideMotif(s))
		}
		out[i] = ms
	}
	return out
}

// detectMotifRuns finds the runs of motifRunMinLen or more exhibits in one
// motif. A structural slide, a back-matter slide or a slide without a motif
// ends the run.
func detectMotifRuns(slides []motifSlide) []MotifRun {
	var runs []MotifRun
	var cur MotifRun
	curKey := ""
	flush := func() {
		if curKey != "" && cur.Len >= motifRunMinLen {
			runs = append(runs, cur)
		}
		curKey = ""
	}
	for i, s := range slides {
		if s.key == "" {
			flush()
			continue
		}
		if s.key == curKey {
			cur.End = i
			if s.unit != slides[i-1].unit {
				cur.Len++
				cur.members = append(cur.members, i)
			}
			continue
		}
		flush()
		curKey = s.key
		cur = MotifRun{Motif: s.key, Start: i, End: i, Len: 1, members: []int{i}}
	}
	flush()
	return runs
}

// motifShares returns each motif's share of the deck's content exhibits (by
// plain motif name, for the report), and the dominant look-alike key with its
// slides when one covers more than motifDominantShare of them.
func motifShares(inputs []Slide, slides []motifSlide) (shares map[string]float64, dominant string, dominantSlides []int, total int) {
	byName := map[string]int{}
	byKey := map[string][]int{}
	for i, s := range slides {
		if !s.content || s.unit != i {
			continue // structural, back matter, or a later part of a continued exhibit
		}
		total++
		if s.key == "" {
			continue
		}
		name, _ := slideMotif(inputs[i])
		byName[name]++
		byKey[s.key] = append(byKey[s.key], i)
	}
	shares = map[string]float64{}
	if total == 0 {
		return shares, "", nil, 0
	}
	for name, n := range byName {
		shares[name] = math.Round(float64(n)/float64(total)*100) / 100
	}
	if total < motifDominantMinContent {
		return shares, "", nil, total
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if float64(len(byKey[k])) > motifDominantShare*float64(total) {
			return shares, k, byKey[k], total
		}
	}
	return shares, "", nil, total
}

// motifPenalty is what the motif and continuation checks cost the
// composition score.
type motifPenalty struct {
	// runs are the motif runs no pattern run already accounts for.
	runs []MotifRun
	// dominant is set when one motif dominates across more than one pattern.
	dominant bool
	// gaps counts continued exhibits with slides between their parts.
	gaps int
}

// samePatternRun reports whether a pattern run of 3+ covers exactly the motif
// run, in which case break_run already reports it.
func samePatternRun(m MotifRun, runs []PatternRun) bool {
	for _, r := range runs {
		if r.Len >= motifRunMinLen && r.Start <= m.Start && r.End >= m.End {
			return true
		}
	}
	return false
}

// distinctVisuals counts the pattern-run fingerprints among the given slides.
func distinctVisuals(slides []motifSlide, idx []int) int {
	seen := map[string]bool{}
	for _, i := range idx {
		seen[slides[i].visual] = true
	}
	return len(seen)
}

// motifRecommendations reports motif runs, a dominant motif and interrupted
// continuations, each with alternatives of a different motif chosen from what
// the slide says.
func motifRecommendations(inputs []Slide, slides []motifSlide, runs []PatternRun, motifRuns []MotifRun, dominant string, dominantSlides []int, total int) ([]Recommendation, motifPenalty) {
	var recs []Recommendation
	var penalty motifPenalty

	for _, run := range motifRuns {
		if samePatternRun(run, runs) {
			continue
		}
		penalty.runs = append(penalty.runs, run)
		for offset := 2; offset < len(run.members); offset += 3 {
			at := run.members[offset]
			recs = append(recs, Recommendation{
				Code:       CodeBreakMotifRun,
				SlideIndex: at,
				Message: fmt.Sprintf("slides %d–%d use different patterns but draw the same motif (%s) %d times in a row — they read as one repeated slide; give slide %d a different motif",
					run.Start, run.End, motifLabel(run.Motif), run.Len, at),
				RecommendedBreak: suggestMotifBreak(inputs[at]),
			})
		}
	}

	if dominant != "" {
		penalty.dominant = distinctVisuals(slides, dominantSlides) > 1
		idx := make([]string, len(dominantSlides))
		var text strings.Builder
		for i, v := range dominantSlides {
			idx[i] = fmt.Sprint(v)
			text.WriteString(inputs[v].Title + " " + inputs[v].Text + " ")
		}
		// Suggest for the deck as a whole from what those slides say; the
		// motif to leave is the same for all of them.
		probe := inputs[dominantSlides[0]]
		probe.Title, probe.Text = text.String(), ""
		recs = append(recs, Recommendation{
			Code:       CodeMotifDominant,
			SlideIndex: -1,
			Message: fmt.Sprintf("%d of %d content slides draw one motif (%s; slides %s) — the deck has one look; redraw some of them in a different motif",
				len(dominantSlides), total, motifLabel(dominant), strings.Join(idx, ", ")),
			RecommendedBreak: suggestMotifBreak(probe),
		})
	}

	titles := make([]string, len(inputs))
	for i, s := range inputs {
		titles[i] = s.Title
	}
	for _, gap := range ContinuationGaps(titles) {
		penalty.gaps++
		what := "slide"
		if len(gap.Between) > 1 {
			what = "slides"
		}
		kind := ""
		if isStructural(inputs[gap.Between[0]]) {
			kind = " (a " + structuralLabel(inputs[gap.Between[0]]) + ")"
		}
		recs = append(recs, Recommendation{
			Code:       CodeContinuationInterrupted,
			SlideIndex: gap.Between[0],
			Message: fmt.Sprintf("%s %s%s sits between %q (slide %d) and its continuation %q (slide %d) — a continued exhibit is one unit; move slide %d before slide %d or after slide %d",
				what, indexList(gap.Between), kind, strings.TrimSpace(inputs[gap.Before].Title), gap.Before,
				strings.TrimSpace(inputs[gap.After].Title), gap.After, gap.Between[0], gap.Before, gap.After),
			RecommendedBreak: []string{},
		})
	}
	return recs, penalty
}

// structuralLabel names a structural slide's role for a message.
func structuralLabel(s Slide) string {
	switch s.Role {
	case "section":
		return "section divider"
	case "title":
		return "title slide"
	case "closing":
		return "closing slide"
	}
	return "agenda slide"
}

func indexList(idx []int) string {
	parts := make([]string, len(idx))
	for i, v := range idx {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ", ")
}

// motifLabel renders a motif key for a message: "open-columns", or
// "chart: bar" for a distinctive motif's variant.
func motifLabel(key string) string {
	if motif, variant, ok := strings.Cut(key, ":"); ok {
		if variant == "" {
			return motif
		}
		return motif + ": " + variant
	}
	return key
}

// suggestMotifBreak ranks the patterns of a different motif for one slide by
// what the slide says (numbers, options, dates), then by its family's usual
// alternatives.
func suggestMotifBreak(slide Slide) []string {
	motif, _ := slideMotif(slide)
	family := visualFamily(slide.PatternName)
	if family == "" {
		family = dominantVisual(slide)
	}
	signals := textSignals(slide.Title + " " + slide.Text)
	preferences := append(contentBreakPreferences(signals), breakPreferences(family)...)
	return rankBreakPatterns(slide, family, motif, signals, preferences)
}
