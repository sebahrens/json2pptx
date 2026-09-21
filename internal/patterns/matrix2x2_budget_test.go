package patterns

import (
	"strings"
	"testing"
)

func TestMatrix2x2PairedQuadrantBudget(t *testing.T) {
	p := &matrix2x2{}
	for _, tc := range []struct {
		name   string
		header string
		body   string
		warn   bool
	}{
		{"wide_pair_over_budget", strings.Repeat("W", 80), strings.Repeat("W", 177), true},
		{"wide_pair_at_budget", strings.Repeat("W", 80), strings.Repeat("W", 176), false},
		{"shorter_header", strings.Repeat("W", 79), strings.Repeat("W", 200), false},
		{"word_like_header", strings.Repeat("word ", 16), strings.Repeat("W", 200), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := &Matrix2x2Values{TopLeft: Matrix2x2Quadrant{Header: tc.header, Body: tc.body}}
			got := p.PostExpandWarnings(ExpandContext{}, values, nil)
			if !tc.warn {
				if len(got) != 0 {
					t.Fatalf("unexpected warnings: %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], "top_left.header/body") ||
				!strings.Contains(got[0], "about 176 wide characters") {
				t.Fatalf("warnings = %v, want paired quadrant budget", got)
			}
		})
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values produced warnings: %v", got)
	}
}
