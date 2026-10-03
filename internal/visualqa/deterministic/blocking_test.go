package deterministic

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func blockingFinding(code, action, path string) patterns.FitFinding {
	return patterns.FitFinding{ValidationError: patterns.ValidationError{Code: code, Path: path}, Action: action}
}

// go-slide-creator-x9rhq: BlocksGate is the per-finding half of the gate. A
// finding it reports fails the gate by itself, and one it does not report
// never does.
func TestBlocksGateMatchesTheGateOneFindingAtATime(t *testing.T) {
	criteria := DefaultQualityGateCriteria()
	cases := []struct {
		f    patterns.FitFinding
		want bool
	}{
		{blockingFinding(patterns.ErrCodeTextBelowReadableMin, "refuse", "/slides/1/x"), true},
		{blockingFinding("fit_overflow", "shrink_or_split", "/slides/1/x"), true},
		{blockingFinding(patterns.ErrCodeBodyTooLong, "review", "/slides/1/pattern"), true},
		{blockingFinding(patterns.ErrCodeBodyTooLong, "info", "/slides/1/pattern"), false},
		{blockingFinding(patterns.ErrCodeSlideNearlyEmpty, "review", "/slides/1"), true},
		{blockingFinding(patterns.ErrCodeNoExecutiveSummary, "review", "/slides/1"), true},
		{blockingFinding(patterns.ErrCodeClosingWithoutNextSteps, "review", "/slides/1"), true},
		{blockingFinding(patterns.ErrCodeTakeawayMissing, "review", "/slides/1"), true},
		{blockingFinding(patterns.ErrCodeAccentOverload, "info", "/slides/1"), true},
		{blockingFinding(patterns.ErrCodeTitleNotAction, "review", "/slides/1/title"), false},
		{blockingFinding(patterns.ErrCodeSlideUnderused, "review", "/slides/1"), false},
		{blockingFinding(patterns.ErrCodeTitleWraps, "info", "/slides/1/title"), false},
		// A predicted (not authored or generated) shrink is an advisory.
		{blockingFinding(patterns.ErrCodeTextBelowReadableMin, "review", "/slides/1/x"), false},
	}
	for _, c := range cases {
		name := fmt.Sprintf("%s/%s", c.f.Code, c.f.Action)
		if got := BlocksGate(c.f, criteria); got != c.want {
			t.Errorf("%s: BlocksGate = %v, want %v", name, got, c.want)
		}
		// Ten slides, so one review finding cannot trip the score floor or the
		// problem-slide share: the gate's verdict is this finding's alone.
		findings := []patterns.FitFinding{c.f}
		gate := EvaluateQualityGate(ScoreFromFindings(findings, 10), findings, criteria)
		if gate.Passed == c.want {
			t.Errorf("%s: gate passed=%v but BlocksGate=%v (reasons %v)", name, gate.Passed, c.want, gate.Reasons)
		}
		if c.want && len(AggregateGateReasons(gate)) != 0 {
			t.Errorf("%s: a per-finding blocker was classed as an aggregate reason: %v", name, AggregateGateReasons(gate))
		}
	}

	// The criteria decide: with the storyline rule off, the storyline findings
	// stop blocking.
	relaxed := criteria
	relaxed.RequireStoryline = false
	if BlocksGate(blockingFinding(patterns.ErrCodeNoExecutiveSummary, "review", "/slides/1"), relaxed) {
		t.Error("NO_EXECUTIVE_SUMMARY blocks with require_storyline off")
	}
}

// Every reason the gate can give is either the sum of findings BlocksGate
// reports or an aggregate reason: a failed gate always has something to name.
func TestEveryGateReasonIsAFindingOrAnAggregate(t *testing.T) {
	criteria := DefaultQualityGateCriteria()
	// A deck that trips the aggregate criteria only: topic titles on 3 of 4
	// slides and many review advisories, with no per-finding blocker.
	var findings []patterns.FitFinding
	for i := 0; i < 4; i++ {
		if i < 3 {
			findings = append(findings, blockingFinding(patterns.ErrCodeTitleNotAction, "review", fmt.Sprintf("/slides/%d/title", i)))
		}
		for j := 0; j < 6; j++ {
			findings = append(findings, blockingFinding("DATA_LABEL_DENSE", "review", fmt.Sprintf("/slides/%d/content/%d", i, j)))
		}
	}
	for _, f := range findings {
		if BlocksGate(f, criteria) {
			t.Fatalf("fixture carries a per-finding blocker: %+v", f)
		}
	}
	gate := EvaluateQualityGate(ScoreFromFindings(findings, 4), findings, criteria)
	if gate.Passed {
		t.Fatal("fixture no longer fails the gate")
	}
	aggregate := AggregateGateReasons(gate)
	if len(aggregate) != len(gate.Reasons) {
		t.Errorf("a gate reason is neither a finding nor an aggregate:\n  all:       %v\n  aggregate: %v", gate.Reasons, aggregate)
	}
	if AggregateGateReasons(nil) != nil {
		t.Error("nil gate must have no aggregate reasons")
	}
}

func TestBlockingProfile(t *testing.T) {
	cases := []struct {
		code, action, want string
	}{
		{"fit_overflow", "refuse", BlocksAlways},
		{"density_exceeded", "shrink_or_split", BlocksAlways},
		{patterns.ErrCodeAccentOverload, "info", BlocksAlways},
		{patterns.ErrCodeNoExecutiveSummary, "review", BlocksSometimes},
		{patterns.ErrCodeClosingWithoutNextSteps, "review", BlocksSometimes},
		{patterns.ErrCodeTakeawayMissing, "review", BlocksSometimes},
		{patterns.ErrCodeTextBelowReadableMin, "review", BlocksSometimes},
		{patterns.ErrCodeBodyTooLong, "review", BlocksSometimes},
		{patterns.ErrCodeTitleNotAction, "review", BlocksSometimes},
		{patterns.ErrCodeSlideUnderused, "review", BlocksNever},
		{patterns.ErrCodeTitleWraps, "info", BlocksNever},
	}
	for _, c := range cases {
		got, when := BlockingProfile(c.code, c.action)
		if got != c.want || when == "" {
			t.Errorf("%s/%s: profile %q (%q), want %q", c.code, c.action, got, when, c.want)
		}
	}
}
