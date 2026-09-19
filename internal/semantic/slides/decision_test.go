package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func option(label, detail string) map[string]any {
	m := map[string]any{"label": label}
	if detail != "" {
		m["detail"] = detail
	}
	return m
}

func decisionBody(options ...any) map[string]any {
	return map[string]any{
		"title":          "The H2 retention bet",
		"recommendation": "Stand up a dedicated SMB customer-success pod in Q3.",
		"options":        options,
	}
}

// The ask is the slide a board deck exists for and it compiled to the plainest
// page in the deck: a bold paragraph and a column of dashes
// (go-slide-creator-4ndv).
func TestCompileDecisionGivesTheAskAVisual(t *testing.T) {
	t.Run("three options become numbered boxes", func(t *testing.T) {
		body := decisionBody(
			option("Hold current coverage", "No new cost, and churn keeps climbing."),
			option("Fund a success pod", "Four people from Q3."),
			option("Offshore support", "Cheapest per seat."),
		)
		if got := DecisionPattern(body); got != "numbered-step-strip" {
			t.Fatalf("DecisionPattern = %q, want numbered-step-strip", got)
		}
		slide, _, err := CompileDecision(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		if slide.Pattern == nil || slide.Pattern.Name != "numbered-step-strip" {
			t.Fatalf("pattern = %+v", slide.Pattern)
		}
		var values decisionStripValues
		if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
			t.Fatalf("values %s: %v", slide.Pattern.Values, err)
		}
		if len(values.Steps) != 3 || values.Steps[0].Body != "No new cost, and churn keeps climbing." {
			t.Errorf("steps = %+v", values.Steps)
		}
		// The ask lands in the callout band, which is where it belongs.
		if slide.Pattern.Callout == nil || !strings.Contains(slide.Pattern.Callout.Text, "success pod") {
			t.Errorf("callout = %+v, want the recommendation", slide.Pattern.Callout)
		}
	})

	t.Run("two detailed options become two cards", func(t *testing.T) {
		body := decisionBody(
			option("Build in house", "Full control, nine months before the first wave."),
			option("Buy the platform", "Live in three months on someone else's roadmap."),
		)
		if got := DecisionPattern(body); got != "card-grid" {
			t.Fatalf("DecisionPattern = %q, want card-grid", got)
		}
		slide, _, err := CompileDecision(Input{Body: body})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		var values decisionCardValues
		if err := json.Unmarshal(slide.Pattern.Values, &values); err != nil {
			t.Fatalf("values %s: %v", slide.Pattern.Values, err)
		}
		if values.Columns != 2 || values.Rows != 1 || len(values.Cells) != 2 {
			t.Errorf("grid = %+v", values)
		}
		if slide.Pattern.Callout == nil {
			t.Error("the ask lost its callout band")
		}
	})
}

// Outside the visuals' bounds it keeps the content slide it has always
// produced, and the finding says why.
func TestDecisionDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name:   "one option",
			body:   decisionBody(option("Fund the pod", "Four people from Q3.")),
			reason: "has one option",
		},
		{
			name:   "two options, one bare",
			body:   decisionBody(option("Build in house", "Full control."), option("Buy", "")),
			reason: "says only what it is called",
		},
		{
			name: "seven options",
			body: decisionBody(
				option("a", ""), option("b", ""), option("c", ""), option("d", ""),
				option("e", ""), option("f", ""), option("g", ""),
			),
			reason: "has 7 options",
		},
		{
			name: "a label written as a sentence",
			body: decisionBody(
				option(strings.Repeat("a", 70), ""), option("b", ""), option("c", ""),
			),
			reason: "label is 70 characters",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DecisionPattern(c.body); got != "" {
				t.Errorf("DecisionPattern = %q, want the content slide", got)
			}
			if over := DecisionOverBudget(c.body); !strings.Contains(over, c.reason) {
				t.Errorf("DecisionOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileDecision(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the content slide, got pattern %s", slide.Pattern.Name)
			}
			if len(slide.Content) == 0 {
				t.Error("the content slide lost everything")
			}
		})
	}
}

// Three bare labels still fit the numbered boxes: the strip needs a label, not
// a detail. It is the two-card treatment that needs both.
func TestDecisionBareLabelsStillGetTheStrip(t *testing.T) {
	body := decisionBody("Hold coverage", "Fund the pod", "Offshore support")
	if got := DecisionPattern(body); got != "numbered-step-strip" {
		t.Errorf("DecisionPattern = %q, want numbered-step-strip", got)
	}
}

// The content slide keeps each detail with the option it belongs to.
func TestDecisionContentFallbackKeepsTheDetails(t *testing.T) {
	body := decisionBody(option("Fund the pod", "Four people from Q3."))
	slide, _, err := CompileDecision(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	encoded, err := json.Marshal(slide.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	for _, want := range []string{"Stand up a dedicated SMB", "Fund the pod — Four people from Q3."} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the content slide dropped %q: %s", want, encoded)
		}
	}
}

// The spellings an author reaches for all resolve, and an option written as one
// line keeps its detail.
func TestDecisionOptionForms(t *testing.T) {
	for _, field := range []string{"options", "choices", "alternatives"} {
		body := map[string]any{field: []any{"Hold", "Fund", "Offshore"}}
		if n := UsableDecisionOptionCount(body); n != 3 {
			t.Errorf("%s: resolved %d options, want 3", field, n)
		}
	}
	for _, sep := range []string{" | ", " — ", " - "} {
		body := map[string]any{"options": []any{"Fund the pod" + sep + "Four people from Q3."}}
		got := DecisionOptions(body)
		if len(got) != 1 || got[0].Label != "Fund the pod" || got[0].Detail != "Four people from Q3." {
			t.Errorf("%q: resolved %+v", sep, got)
		}
	}
	for _, key := range []string{"detail", "description", "body", "summary"} {
		body := map[string]any{"options": []any{map[string]any{"label": "Fund", key: "Four people."}}}
		got := DecisionOptions(body)
		if len(got) != 1 || got[0].Detail != "Four people." {
			t.Errorf("%s: resolved %+v", key, got)
		}
	}
	// An entry with no label is dropped rather than drawing a blank box.
	body := map[string]any{"options": []any{"Fund", map[string]any{"detail": "orphan"}}}
	if n := len(DecisionOptions(body)); n != 1 {
		t.Errorf("resolved %d options, want the 1 with a label", n)
	}
}
