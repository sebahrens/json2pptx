package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// The rule was never evaluated: it only picked a default fill, so a cell tagged
// rule:"negative" was tinted red whatever it said, and the only threshold shape
// the parser accepted was a number (go-slide-creator-6hlu).
func TestConditionalMatches(t *testing.T) {
	cases := []struct {
		name      string
		rule      string
		threshold any
		content   string
		want      bool
	}{
		{"empty rule always tints", "", nil, "anything", true},
		{"always", types.ConditionalRuleAlways, nil, "", true},

		{"positive on a plus figure", "positive", nil, "+4%", true},
		{"positive on a negative figure", "positive", nil, "-9%", false},
		{"positive on text", "positive", nil, "On track", false},
		{"negative on a minus figure", "negative", nil, "-9%", true},
		{"negative on accounting parentheses", "negative", nil, "(3.2)", true},
		{"negative on a plus figure", "negative", nil, "+8", false},

		{"threshold is gte", "threshold", 90.0, "95", true},
		{"threshold below the bar", "threshold", 90.0, "89.9", false},
		{"gte at the bar", "gte", 90.0, "90", true},
		{"lte under the bar", "lte", 5.0, "4.5", true},
		{"lte over the bar", "lte", 5.0, "5.5", false},
		{"numeric threshold written as a string still compares", "gte", "90", "95", true},
		{"non-numeric threshold for a numeric rule matches nothing", "gte", "soon", "95", false},

		{"between inside the range", "between", []float64{0, 5}, "+1%", true},
		{"between outside the range", "between", []float64{0, 5}, "9", false},
		{"between accepts reversed bounds", "between", []float64{5, 0}, "3", true},
		{"between without a pair matches nothing", "between", 3.0, "3", false},

		{"equals on text", "equals", "On track", "On track", true},
		{"equals ignores case and padding", "equals", "on track", "  On Track ", true},
		{"equals on a different status", "equals", "On track", "At risk", false},
		{"equals compares numbers when both are numbers", "equals", 50.0, "50%", true},

		{"contains a substring", "contains", "track", "On track", true},
		{"contains is case-insensitive", "contains", "TRACK", "On track", true},
		{"contains misses", "contains", "risk", "On track", false},
		{"contains with no threshold matches nothing", "contains", nil, "On track", false},

		{"an unknown rule applies nothing", "eqals", "On track", "On track", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cond := &types.ConditionalFormat{Rule: tc.rule, Threshold: tc.threshold, Fill: "accent3"}
			if got := conditionalMatches(cond, tc.content); got != tc.want {
				t.Errorf("rule %q threshold %v on %q = %v, want %v", tc.rule, tc.threshold, tc.content, got, tc.want)
			}
		})
	}
}

// A rule that does not match emits no fill at all, so the cell keeps the
// table's normal appearance.
func TestResolveConditionalFillRespectsTheRule(t *testing.T) {
	match := resolveConditionalFill(&types.ConditionalFormat{Rule: "equals", Threshold: "On track", Fill: "accent3"}, "On track")
	if match == "" {
		t.Error("a matching rule must emit a fill")
	}
	miss := resolveConditionalFill(&types.ConditionalFormat{Rule: "equals", Threshold: "On track", Fill: "accent3"}, "At risk")
	if miss != "" {
		t.Errorf("a non-matching rule must emit nothing, got %q", miss)
	}
}

func TestParseCellNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"+5%", 5, true},
		{"-0.5", -0.5, true},
		{"(3.2)", -3.2, true},
		{"EUR 1,186.4", 1186.4, true},
		{"95", 95, true},
		{"On track", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := parseCellNumber(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseCellNumber(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
