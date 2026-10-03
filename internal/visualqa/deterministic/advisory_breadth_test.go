package deterministic

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// go-slide-creator-fabz4: info-severity advisories on most slides must not
// fail max_problem_slides_pct — a 28-slide deck read as "57% problem slides"
// on alt-text, accent and duplicate-title advice alone.
func TestAdvisoryFindingsDoNotCountTowardProblemShare(t *testing.T) {
	advisory := []string{
		patterns.ErrCodeMissingAltText, patterns.ErrCodeAccentOverload, patterns.ErrCodeDuplicateTitle,
		patterns.ErrCodeDeckMonotony, patterns.ErrCodeTableFontScaled, patterns.ErrCodeChartShapeInferred,
		patterns.ErrCodeTitleTooLong, patterns.ErrCodeVerticalImbalance, patterns.ErrCodeHorizontalImbalance, patterns.ErrCodeSparsePlaceholder,
	}
	var findings []patterns.FitFinding
	for i, code := range advisory {
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{Path: fmt.Sprintf("/slides/%d", i), Code: code},
			Action:          "review",
		})
	}
	ds := ScoreFromFindings(findings, 10)
	if ds.Summary.ProblemSlidesCount != 0 {
		t.Errorf("problem slides = %d, want 0 for advisory-only findings", ds.Summary.ProblemSlidesCount)
	}
	// A substantive review still counts.
	findings = append(findings, patterns.FitFinding{
		ValidationError: patterns.ValidationError{Path: "/slides/9", Code: patterns.ErrCodeMissingTitle},
		Action:          "review",
	})
	if ds := ScoreFromFindings(findings, 10); ds.Summary.ProblemSlidesCount != 1 {
		t.Errorf("problem slides = %d, want 1 (MISSING_TITLE still counts)", ds.Summary.ProblemSlidesCount)
	}
}
