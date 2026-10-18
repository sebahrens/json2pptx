package main

import (
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestFindingBudgetPreservesOmissionsAndRefusals(t *testing.T) {
	budgets := map[string]func([]patterns.FitFinding, int) []patterns.FitFinding{
		"shared": func(findings []patterns.FitFinding, budget int) []patterns.FitFinding {
			return BudgetFitFindings(findings, budget, false)
		},
		"local": func(findings []patterns.FitFinding, budget int) []patterns.FitFinding {
			local := make([]fitFinding, len(findings))
			for i, f := range findings {
				local[i] = fitFinding{Path: f.Path, Code: f.Code, Action: f.Action, Fix: f.Fix}
			}
			var result []patterns.FitFinding
			for _, f := range budgetLocalFindings(local, budget, false) {
				result = append(result, patterns.FitFinding{ValidationError: patterns.ValidationError{Path: f.Path, Code: f.Code, Fix: f.Fix}, Action: f.Action})
			}
			return result
		},
	}
	for name, budgetFindings := range budgets {
		t.Run(name, func(t *testing.T) {
			for _, budget := range []int{2, 6} {
				t.Run(fmt.Sprint(budget), func(t *testing.T) {
					var findings []patterns.FitFinding
					// Earlier actionable review advice must not conceal a later omission.
					for i := range 4 {
						findings = append(findings, makeFinding(0, "review", fmt.Sprintf("advice_%d", i), true))
					}
					mandatory := []patterns.FitFinding{
						makeFinding(0, "refuse", "refusal_a", false),
						makeFinding(0, "refuse", "refusal_b", false),
						makeFinding(0, "refuse", "refusal_c", false),
						patterns.ContentDropped("/slides/0/content/0", "first block", "omitted"),
						patterns.ContentDropped("/slides/0/content/1", "second block", "omitted"),
					}
					findings = append(findings, mandatory...)
					findings = append(findings, makeFinding(1, "info", "other_slide", false))
					got := budgetFindings(findings, budget)
					for _, want := range mandatory {
						count := 0
						for _, f := range got {
							if f.Code == want.Code && f.Path == want.Path && f.Action == want.Action {
								count++
							}
						}
						if count != 1 {
							t.Errorf("mandatory finding %s at %s retained %d times", want.Code, want.Path, count)
						}
					}
					suppressed := 9 - max(budget, len(mandatory))
					if len(got) != 11-suppressed { // nine on slide 0, its summary, and slide 1.
						t.Fatalf("unexpected retained count: %+v", got)
					}
					summaries := 0
					for _, f := range got {
						if f.Code != "findings_truncated" {
							continue
						}
						summaries++
						if f.Path != "/slides/0" || f.Fix == nil || f.Fix.Params["suppressed_count"] != suppressed {
							t.Fatalf("incorrect suppression summary: %+v", f)
						}
						codes, ok := f.Fix.Params["top_codes"].([]string)
						if !ok || len(codes) == 0 {
							t.Fatalf("missing suppressed advice histogram: %+v", f)
						}
						for _, code := range codes {
							if len(code) < 7 || code[:7] != "advice_" {
								t.Errorf("mandatory finding reported as suppressed: %s", code)
							}
						}
					}
					if summaries != 1 {
						t.Errorf("summary count = %d, want 1", summaries)
					}
				})
			}
		})
	}
}
