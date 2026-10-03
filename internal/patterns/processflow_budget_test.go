package patterns

import (
	"strings"
	"testing"
)

func TestProcessFlowLabelBudgetWarnings(t *testing.T) {
	p := &processFlow{}
	for _, tc := range []struct {
		name     string
		count    int
		shape    string
		label    string
		wantWarn bool
	}{
		{"sparse_chevron_wide_at_limit", 4, "chevron", strings.Repeat("W", 69), false},
		{"sparse_chevron_wide_over_limit", 4, "chevron", strings.Repeat("W", 70), true},
		{"sparse_chevron_word_like_max", 4, "chevron", strings.Repeat("word ", 16), false},
		{"five_chevrons_at_limit", 5, "chevron", strings.Repeat("word ", 12) + "w", false},
		{"five_chevrons_over_limit", 5, "chevron", strings.Repeat("word ", 12) + "ww", true},
		{"six_arrows_at_limit", 6, "arrow", strings.Repeat("word ", 6) + "w", false},
		{"six_arrows_over_limit", 6, "arrow", strings.Repeat("word ", 6) + "xx", true},
		{"seven_steps_wide_at_limit", 7, "step", strings.Repeat("W", 68), false},
		{"seven_steps_wide_over_limit", 7, "step", strings.Repeat("W", 69), true},
		{"eight_steps_wide_at_limit", 8, "step", strings.Repeat("W", 59), false},
		{"eight_steps_wide_over_limit", 8, "step", strings.Repeat("W", 60), true},
		{"eight_chevrons_word_like_at_limit", 8, "chevron", "word wordw", false},
		{"eight_chevrons_word_like_over_limit", 8, "chevron", "word wordww", true},
		{"eight_arrows_wide_at_limit", 8, "arrow", strings.Repeat("W", 8), false},
		{"eight_arrows_wide_over_limit", 8, "arrow", strings.Repeat("W", 9), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &ProcessFlowValues{Steps: make([]ProcessFlowStep, tc.count)}
			for i := range v.Steps {
				v.Steps[i] = ProcessFlowStep{Label: "Stage", Type: tc.shape}
			}
			v.Steps[0].Label = tc.label
			got := p.PostExpandWarnings(ExpandContext{}, v, nil)
			if !tc.wantWarn {
				if len(got) != 0 {
					t.Fatalf("unexpected warnings: %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], "steps[0].label") || !strings.Contains(got[0], "BODY_TOO_LONG") {
				t.Fatalf("warnings = %v, want step 0 budget finding", got)
			}
		})
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values produced warnings: %v", got)
	}
}
