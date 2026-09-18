package main

import "github.com/sebahrens/json2pptx/internal/visualqa/deterministic"

// Score basis vocabulary (go-slide-creator-n1t7). Every deck-quality score the
// server reports is on ONE 0-100 scale and carries a basis naming exactly what
// it measured, so agents never compare an input heuristic against a
// structural gate or mistake either for a visual verdict:
//
//   - input:      static heuristics over the input JSON (title length, bullet
//     counts, chart data shape). generate_presentation.quality and
//     render_deck_spec.quality_summary.
//   - structural: deterministic rules over the GENERATED deck structure (fit
//     findings, pagination, autofit, contrast swaps, rhythm) — no pixels.
//     score_deck.overall_score, auto_repair/make_deck.final_score.
//   - rendered:   reserved for scores derived from inspecting rendered pixels.
const (
	scoreBasisInput      = "input"
	scoreBasisStructural = deterministic.ScoreBasisStructural
	scoreBasisRendered   = "rendered"
)

// scoreBases is the closed basis enum, in documentation order.
var scoreBases = []string{scoreBasisInput, scoreBasisStructural, scoreBasisRendered}

// toScore100 maps a 0-1 fraction onto the shared 0-100 integer scale, clamped.
func toScore100(frac float64) float64 {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return float64(int(frac*100 + 0.5))
}
