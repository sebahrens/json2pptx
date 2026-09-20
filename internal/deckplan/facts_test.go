package deckplan

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// planContains reports whether s appears in any slide's content_seed / facts
// or in unplaced_facts.
func planContains(res *Result, s string) bool {
	for _, sl := range res.Slides {
		if sl.NarrativeRole == "opening" {
			continue // the title seed echoes the whole brief; it doesn't count
		}
		if strings.Contains(sl.ContentSeed, s) {
			return true
		}
		for _, f := range sl.Facts {
			if strings.Contains(f, s) {
				return true
			}
		}
	}
	for _, f := range res.UnplacedFacts {
		if strings.Contains(f, s) {
			return true
		}
	}
	return false
}

// TestBuildDeckPlan_CarriesBriefFacts is the go-slide-creator-kndv acceptance
// test: every quantity in the brief reaches a content seed or unplaced_facts.
func TestBuildDeckPlan_CarriesBriefFacts(t *testing.T) {
	reg := patterns.Default()
	cases := []struct {
		brief  string
		budget int
		want   []string
	}{
		{"+23% revenue, churn 4%", 10, []string{"+23% revenue", "churn 4%"}},
		{"+23% revenue, churn 4%", 3, []string{"+23% revenue", "churn 4%"}},
		{"Q3 QBR for the executive team: revenue grew +23% year over year, churn fell to 4%, EU expansion is on track, hiring plan adds 40 engineers.", 10,
			[]string{"+23%", "4%", "EU expansion", "40 engineers"}},
		{"Q3 QBR for the executive team: revenue grew +23% year over year, churn fell to 4%, EU expansion is on track, hiring plan adds 40 engineers.", 3,
			[]string{"+23%", "4%", "EU expansion", "40 engineers"}},
	}
	for _, tc := range cases {
		res := BuildDeckPlan(reg, Params{Brief: tc.brief, SlideBudget: tc.budget}, nil)
		for _, w := range tc.want {
			if !planContains(res, w) {
				t.Errorf("budget %d brief %q: %q missing from seeds and unplaced_facts", tc.budget, tc.brief, w)
			}
		}
		// Title and closing slides never receive facts.
		for _, s := range []Slide{res.Slides[0], res.Slides[len(res.Slides)-1]} {
			if len(s.Facts) != 0 {
				t.Errorf("slide %d (%s) received facts %v", s.SlideIndex, s.NarrativeRole, s.Facts)
			}
		}
		// Seeds carry each slide's facts verbatim.
		for _, s := range res.Slides {
			for _, f := range s.Facts {
				if !strings.Contains(s.ContentSeed, f) {
					t.Errorf("slide %d: fact %q not in content_seed %q", s.SlideIndex, f, s.ContentSeed)
				}
			}
			if n := len(s.Facts); n > factCapacity(s.RecommendedPattern) {
				t.Errorf("slide %d: %d facts exceeds capacity %d for %s", s.SlideIndex, n, factCapacity(s.RecommendedPattern), s.RecommendedPattern)
			}
		}
	}
}

func TestBuildDeckPlan_UnplacedFactsAlwaysArray(t *testing.T) {
	res := BuildDeckPlan(patterns.Default(), Params{Brief: "Pitch our Series B for an AI infra company", SlideBudget: 5}, nil)
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"unplaced_facts":[]`) {
		t.Errorf("unplaced_facts should serialize as [] when empty; got %s", raw)
	}
}

func TestExtractBriefFacts(t *testing.T) {
	cases := []struct {
		brief string
		want  []briefFact
	}{
		{"+23% revenue, churn 4%", []briefFact{{"+23% revenue", true}, {"churn 4%", true}}},
		// First clause is the topic: an entity-only first clause is not a fact,
		// and period labels like Q3 are not quantities.
		{"Pitch our Series B for an AI infra company", nil},
		{"Q3 QBR for the board: EU expansion is on track; hiring adds 40 engineers.",
			[]briefFact{{"EU expansion is on track", false}, {"hiring adds 40 engineers", true}}},
		// Decimals and thousands separators survive the clause split; leading
		// conjunctions are stripped; duplicates collapse.
		{"ARR hit $1.5M. NRR is 140%, and we serve 1,200 customers. NRR is 140%.",
			[]briefFact{{"ARR hit $1.5M", true}, {"NRR is 140%", true}, {"we serve 1,200 customers", true}}},
		{"", nil},
		{"a plain brief with no data", nil},
	}
	for _, tc := range cases {
		got := extractBriefFacts(tc.brief)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("extractBriefFacts(%q) = %+v, want %+v", tc.brief, got, tc.want)
		}
	}
}

func TestAssignBriefFacts_NumericFirstAndOverflow(t *testing.T) {
	slides := []Slide{
		{SlideIndex: 0, NarrativeRole: "opening"},
		{SlideIndex: 1, NarrativeRole: "evidence", RecommendedPattern: "card-grid", ContentSeed: "seed"},
		{SlideIndex: 2, NarrativeRole: "evidence", RecommendedPattern: "kpi-2up", ContentSeed: "seed"},
		{SlideIndex: 3, NarrativeRole: "closing"},
	}
	unplaced := assignBriefFacts(slides, "Topic line: revenue +10%, margin 30%, churn 2%, EU launch, APAC pilot, LATAM hold")
	// Quantities fill the KPI slide (capacity 2) first.
	if want := []string{"revenue +10%", "margin 30%"}; !reflect.DeepEqual(slides[2].Facts, want) {
		t.Errorf("kpi-2up facts = %v, want %v", slides[2].Facts, want)
	}
	// Remaining facts fill card-grid (capacity 2); the rest are unplaced.
	if want := []string{"churn 2%", "EU launch"}; !reflect.DeepEqual(slides[1].Facts, want) {
		t.Errorf("card-grid facts = %v, want %v", slides[1].Facts, want)
	}
	if want := []string{"APAC pilot", "LATAM hold"}; !reflect.DeepEqual(unplaced, want) {
		t.Errorf("unplaced = %v, want %v", unplaced, want)
	}
	// Facts are separate sentences in the seed: joining them with "; " read as
	// one malformed clause, and the semicolon was the character the clause
	// splitter had just cut on (go-slide-creator-vmiy).
	if !strings.HasPrefix(slides[2].ContentSeed, "revenue +10%. margin 30% — ") {
		t.Errorf("seed not prefixed with facts: %q", slides[2].ContentSeed)
	}
	if len(slides[0].Facts)+len(slides[3].Facts) != 0 {
		t.Error("structural slides must not receive facts")
	}
}

// TestExtractBriefFacts_BulletedBrief pins the three defects a normal bulleted
// brief used to produce (go-slide-creator-vmiy): list markers surviving into
// the fact, a clause split landing inside a parenthesis, and the two together
// shipping "- ARR reached €12.5m (up 31%" as a content seed.
func TestExtractBriefFacts_BulletedBrief(t *testing.T) {
	brief := strings.Join([]string{
		"Q3 FY26 business review for the executive team.",
		"- ARR reached €12.5m (up 31%; ahead of plan)",
		"* 1,250 customers",
		"• Gross margin 78.2%",
		"1. 14 enterprise logos",
		"2) Net revenue retention 118%",
	}, "\n")

	facts := extractBriefFacts(brief)
	if len(facts) == 0 {
		t.Fatal("no facts extracted from a bulleted brief")
	}
	for _, f := range facts {
		if strings.ContainsAny(f.text[:1], "-*•") {
			t.Errorf("fact keeps its list marker: %q", f.text)
		}
		if strings.Contains(f.text, ";") {
			t.Errorf("fact contains a semicolon, which is a clause boundary: %q", f.text)
		}
		if strings.Count(f.text, "(") != strings.Count(f.text, ")") {
			t.Errorf("fact has unbalanced brackets: %q", f.text)
		}
	}

	// A semicolon is a clause boundary wherever it is, so the parenthetical is
	// cut there — and the half-open bracket it leaves behind is dropped rather
	// than shipped. Losing "ahead of plan" from a hint beats shipping
	// "ARR reached €12.5m (up 31%".
	if got := facts[0].text; got != "ARR reached €12.5m" {
		t.Errorf("first fact = %q, want the half-open parenthetical dropped", got)
	}
}

// TestBalanceFactBrackets covers the truncation backstop: a cut inside a
// parenthetical drops the half-open clause rather than shipping it.
func TestBalanceFactBrackets(t *testing.T) {
	tests := []struct{ in, want string }{
		{"ARR reached €12.5m (up 31%)", "ARR reached €12.5m (up 31%)"},
		{"ARR reached €12.5m (up 31%", "ARR reached €12.5m"},
		{"ARR reached €12.5m (up 31%, ahead", "ARR reached €12.5m"},
		{"no brackets here", "no brackets here"},
		{"(all of it is open", ""},
	}
	for _, tt := range tests {
		if got := balanceFactBrackets(tt.in); got != tt.want {
			t.Errorf("balanceFactBrackets(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
