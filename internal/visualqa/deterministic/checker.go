// Package deterministic provides rule-based visual quality checks that produce
// zero false positives. Unlike the heuristic (vision-model) layer in the parent
// visualqa package, these checks use geometry math, theme color resolution, and
// computed measurements — no LLM calls.
package deterministic

import (
	"fmt"
	"sort"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// SeverityWeight maps FitFinding action strings to point deductions.
// Higher values = more severe. The score formula is:
//
//	100 - sum(weight(action) for each finding)
//
// clamped to [0, 100].
var SeverityWeight = map[string]int{
	"refuse":         25,
	"shrink_or_split": 15,
	"review":         5,
	"info":           0,
}

// ScoreFinding is a single deterministic finding in the score_deck output.
// It reuses the FitFinding envelope for consistency with the fit-report path.
type ScoreFinding struct {
	Code     string                  `json:"code"`
	Severity string                  `json:"severity"` // "error", "warning", "info"
	Message  string                  `json:"message"`
	Fix      *patterns.FixSuggestion `json:"fix,omitempty"`
}

// SlideScore holds the score and findings for a single slide.
type SlideScore struct {
	Index    int            `json:"index"`
	Score    int            `json:"score"`
	Findings []ScoreFinding `json:"findings"`
}

// DeckScore is the top-level score_deck response.
type DeckScore struct {
	OverallScore int          `json:"overall_score"`
	PerSlide     []SlideScore `json:"per_slide"`
	Summary      DeckSummary  `json:"summary"`
	ModeUsed     string       `json:"mode_used"`
}

// DeckSummary provides aggregate stats.
type DeckSummary struct {
	TopCodes          []CodeCount `json:"top_codes"`
	SlideCount        int         `json:"slide_count"`
	ProblemSlidesCount int        `json:"problem_slides_count"`
}

// CodeCount pairs a finding code with its occurrence count.
type CodeCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// actionToSeverity maps FitFinding action to a human-friendly severity label.
func actionToSeverity(action string) string {
	switch action {
	case "refuse":
		return "error"
	case "shrink_or_split":
		return "warning"
	case "review":
		return "warning"
	case "info":
		return "info"
	default:
		return "info"
	}
}

// ScoreFromFindings computes a DeckScore from a slice of FitFindings.
// slideCount is the total number of slides in the deck.
func ScoreFromFindings(findings []patterns.FitFinding, slideCount int) *DeckScore {
	// Group findings by slide index.
	bySlide := map[int][]patterns.FitFinding{}
	for _, f := range findings {
		si := slideIndexFromPath(f.Path)
		bySlide[si] = append(bySlide[si], f)
	}

	perSlide := make([]SlideScore, slideCount)
	codeCounts := map[string]int{}
	problemSlides := 0

	for i := 0; i < slideCount; i++ {
		ffs := bySlide[i]
		slideScore := 100
		var scoreFindings []ScoreFinding

		for _, f := range ffs {
			w := SeverityWeight[f.Action]
			slideScore -= w
			codeCounts[f.Code]++

			scoreFindings = append(scoreFindings, ScoreFinding{
				Code:     f.Code,
				Severity: actionToSeverity(f.Action),
				Message:  f.Message,
				Fix:      f.Fix,
			})
		}

		if slideScore < 0 {
			slideScore = 0
		}
		if slideScore < 100 {
			problemSlides++
		}

		perSlide[i] = SlideScore{
			Index:    i,
			Score:    slideScore,
			Findings: scoreFindings,
		}
	}

	// Compute overall score as average of per-slide scores.
	overall := 100
	if slideCount > 0 {
		total := 0
		for _, ss := range perSlide {
			total += ss.Score
		}
		overall = total / slideCount
	}

	// Build top_codes sorted by count descending.
	topCodes := make([]CodeCount, 0, len(codeCounts))
	for code, count := range codeCounts {
		topCodes = append(topCodes, CodeCount{Code: code, Count: count})
	}
	sort.Slice(topCodes, func(i, j int) bool {
		return topCodes[i].Count > topCodes[j].Count
	})
	// Cap to top 10.
	if len(topCodes) > 10 {
		topCodes = topCodes[:10]
	}

	return &DeckScore{
		OverallScore: overall,
		PerSlide:     perSlide,
		Summary: DeckSummary{
			TopCodes:           topCodes,
			SlideCount:         slideCount,
			ProblemSlidesCount: problemSlides,
		},
		ModeUsed: "deterministic",
	}
}

// slideIndexFromPath extracts the slide index from a JSON path like "slides[3].foo".
func slideIndexFromPath(path string) int {
	const prefix = "slides["
	if len(path) < len(prefix)+2 {
		return -1
	}
	if path[:len(prefix)] != prefix {
		return -1
	}
	rest := path[len(prefix):]
	idx := 0
	for i := 0; i < len(rest); i++ {
		if rest[i] == ']' {
			return idx
		}
		if rest[i] < '0' || rest[i] > '9' {
			return -1
		}
		idx = idx*10 + int(rest[i]-'0')
	}
	return -1
}

// FormatTopCodes returns a concise human-readable summary of the top finding codes.
func FormatTopCodes(codes []CodeCount) string {
	if len(codes) == 0 {
		return "no issues"
	}
	s := ""
	for i, c := range codes {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%s(%d)", c.Code, c.Count)
	}
	return s
}
