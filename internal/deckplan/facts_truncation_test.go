package deckplan

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// reviewBriefPilot is the go-slide-creator-ze5u7 repro: its comparison clause
// is longer than maxFactLen, carries no and/while/whereas/but, and puts a
// euro amount inside each option's parenthetical.
const reviewBriefPilot = "Board decision on a fictional service pilot. Revenue rose from €10m in 2024 to €12m in 2025. Gross margin fell from 45% to 38% because service costs rose. Compare building an internal support team (€0.8m per year, launch in 6 months) with outsourcing (€0.5m per year, launch in 2 months). Recommend the internal team to retain customer relationships. Pilot starts in January 2027; CFO owns funding approval in November 2026. Include backup assumptions."

func factTexts(fs []briefFact) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.text)
	}
	return out
}

// assertFactsWellFormed checks what every fact must be: valid UTF-8, never
// cut with an ellipsis, with balanced brackets.
func assertFactsWellFormed(t *testing.T, facts []string) {
	t.Helper()
	for _, f := range facts {
		if !utf8.ValidString(f) {
			t.Errorf("fact is not valid UTF-8 (a cut split a rune): %q", f)
		}
		if strings.HasSuffix(f, "...") || strings.HasSuffix(f, "…") {
			t.Errorf("fact was truncated: %q", f)
		}
		if strings.Count(f, "(") != strings.Count(f, ")") {
			t.Errorf("fact has unbalanced brackets: %q", f)
		}
	}
}

// TestExtractBriefFacts_LongComparisonKeepsBothOptions pins
// go-slide-creator-ze5u7: the comparison clause was truncated at 120 runes to
// "... with outsourcing (€", losing the second option and its amounts while
// unplaced_facts stayed empty.
func TestExtractBriefFacts_LongComparisonKeepsBothOptions(t *testing.T) {
	facts := factTexts(extractBriefFacts(reviewBriefPilot))
	assertFactsWellFormed(t, facts)
	all := strings.Join(facts, " | ")
	for _, want := range []string{"€10m", "2024", "€12m", "2025", "45%", "38%", "€0.8m", "6 months", "€0.5m", "2 months", "January 2027", "November 2026"} {
		if !strings.Contains(all, want) {
			t.Errorf("brief amount/date %q lost: %s", want, all)
		}
	}
	whole := false
	for _, f := range facts {
		if strings.Contains(f, "€0.8m") && strings.Contains(f, "€0.5m") && strings.Contains(f, "2 months)") {
			whole = true
		}
	}
	if !whole {
		t.Errorf("the comparison should stay one fact carrying both options: %s", all)
	}
}

// A long clause with none of the four joining words is split at word
// boundaries outside brackets — never truncated, never inside an amount.
func TestExtractBriefFacts_LongClauseWithoutConjunctions(t *testing.T) {
	brief := "Network review. Over the next eighteen months the regional operations programme consolidates 14 depots into 6 hubs across 9 countries saving €4.2m per year from Q3 2027 onward with 120 fewer vehicles on the road (fleet of 1,450 today) in every market we serve"
	if n := utf8.RuneCountInString(brief); n < 2*maxFactLen-60 {
		t.Fatalf("test brief too short (%d runes) to exercise the cap", n)
	}
	facts := factTexts(extractBriefFacts(brief))
	assertFactsWellFormed(t, facts)
	all := strings.Join(facts, " | ")
	for _, want := range []string{"14 depots", "6 hubs", "9 countries", "€4.2m per year", "Q3 2027", "120 fewer", "1,450", "every market we serve"} {
		if !strings.Contains(all, want) {
			t.Errorf("%q lost or split: %s", want, all)
		}
	}
	for _, f := range facts {
		if utf8.RuneCountInString(f) > maxFactLen {
			t.Errorf("fact over the %d-rune cap although it has break points: %q", maxFactLen, f)
		}
	}
}

// A comparison with CJK option names, euro amounts inside brackets and no
// spaces in its CJK runs keeps every amount and stays one fact.
func TestExtractBriefFacts_CJKComparison(t *testing.T) {
	brief := "董事会决策。Compare 建立内部客户支持团队并在六个月内完成招聘与培训 (每年€0.8m, 6个月上线) with 外包给区域服务合作伙伴并按服务等级协议计费 (每年€0.5m, 2个月上线) on cost and speed。"
	facts := factTexts(extractBriefFacts(brief))
	assertFactsWellFormed(t, facts)
	all := strings.Join(facts, " | ")
	for _, want := range []string{"€0.8m", "6个月上线", "€0.5m", "2个月上线"} {
		if !strings.Contains(all, want) {
			t.Errorf("CJK comparison lost %q: %s", want, all)
		}
	}
	if len(facts) != 1 {
		t.Errorf("CJK comparison should stay one fact, got %d: %s", len(facts), all)
	}

	// A CJK run with no space at all cannot be split: it stays whole rather
	// than being cut inside a number.
	long := "背景。" + strings.Repeat("收入", 60) + "增长23%至€48m"
	got := factTexts(extractBriefFacts(long))
	if len(got) != 1 || !strings.HasSuffix(got[0], "增长23%至€48m") {
		t.Errorf("unbreakable CJK clause was cut: %q", got)
	}
}

// TestBracketDepthsByRune pins the byte-index bug: continuation bytes of a
// multi-byte rune inside a bracket read depth 0, so "(€" looked closed and a
// comma after a CJK character inside brackets read as a clause boundary.
func TestBracketDepthsByRune(t *testing.T) {
	s := "a (€0.5m, 外包) b"
	depths := bracketDepths(s)
	open := strings.IndexByte(s, '(')
	closeIdx := strings.IndexByte(s, ')')
	for i := open; i < closeIdx; i++ {
		if depths[i] != 1 {
			t.Errorf("byte %d (%q) depth %d, want 1", i, s[i], depths[i])
		}
	}
	if depths[closeIdx] != 0 || depths[len(s)-1] != 0 {
		t.Errorf("depth after the closer should be 0: %v", depths)
	}

	if got := balanceFactBrackets("Compare building with outsourcing (€"); got != "Compare building with outsourcing" {
		t.Errorf("balanceFactBrackets left the euro bracket open: %q", got)
	}
	if got := balanceFactBrackets("营收 (增长"); got != "营收" {
		t.Errorf("balanceFactBrackets on CJK = %q", got)
	}
	if got := splitBriefClauses("选项 (内部团队, 外包) 比较"); len(got) != 1 {
		t.Errorf("a comma after a CJK character inside brackets split the clause: %q", got)
	}
}

// A clause cut inside a parenthetical keeps the bracketed tail as a fact of
// its own instead of dropping its numbers.
func TestSplitOpenBrackets(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"ARR reached €12.5m (up 31%", []string{"ARR reached €12.5m", "up 31%"}},
		{"ARR reached €12.5m (up 31%)", []string{"ARR reached €12.5m (up 31%)"}},
		{"ahead of plan)", []string{"ahead of plan"}},
		{"(all open", []string{"all open"}},
		{"cost (€0.8m [per year", []string{"cost", "€0.8m", "per year"}},
	}
	for _, tt := range tests {
		if got := splitOpenBrackets(tt.in); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitOpenBrackets(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	facts := factTexts(extractBriefFacts("Q3 review.\n- ARR reached €12.5m (up 31%; ahead of plan)"))
	if !reflect.DeepEqual(facts, []string{"ARR reached €12.5m", "up 31%"}) {
		t.Errorf("bracketed tail dropped: %q", facts)
	}
}

// chunkFact never splits a number from its currency, unit or date word.
func TestChunkFactKeepsNumericTokens(t *testing.T) {
	for _, tok := range []string{"€ 12m", "EUR 12m", "12 %", "6 months", "January 2027", "30 November", "4 million", "12m EUR"} {
		filler := strings.Repeat("x ", 28) // 56 runes
		s := filler + tok + " " + filler
		limit := utf8.RuneCountInString(filler) + 1 // the only in-cap break is inside the token
		for _, piece := range chunkFact(s, limit) {
			if strings.HasSuffix(piece, strings.Fields(tok)[0]) {
				t.Errorf("chunkFact split %q after %q: %q", tok, strings.Fields(tok)[0], chunkFact(s, limit))
			}
		}
		if !strings.Contains(strings.Join(chunkFact(s, limit), " | "), tok) {
			t.Errorf("chunkFact split the token %q: %q", tok, chunkFact(s, limit))
		}
	}
}

// TestComparesAlternatives pins go-slide-creator-fu6uy: "compare A with B"
// weighs two alternatives; a comparison against a benchmark is a metric.
func TestComparesAlternatives(t *testing.T) {
	for _, s := range []string{
		"Compare internal support (€0.8m per year) with outsourcing (€0.5m per year) on cost and launch speed",
		"compare building in-house to buying a platform",
		"Outsourcing compared to an internal team",
		"comparing the internal team against outsourcing",
		"Compare option A versus option B",
	} {
		if !comparesAlternatives(s) {
			t.Errorf("comparesAlternatives(%q) = false, want true", s)
		}
		if f := classifyFact(s, factQuantity.MatchString(s)); !f.option {
			t.Errorf("classifyFact(%q).option = false", s)
		}
	}
	for _, s := range []string{
		"Revenue compared with plan rose 12%",
		"revenue compared to last year is up 8%",
		"margin compared to 2024 fell 3 pts",
		"€5m compared with $4m",
		"Compare revenue with the budget",
		"churn compared with a year ago",
		"bookings compared to Q3",
		"growth compared with peers",
	} {
		if comparesAlternatives(s) {
			t.Errorf("comparesAlternatives(%q) = true, want false (a benchmark)", s)
		}
		if f := classifyFact(s, factQuantity.MatchString(s)); f.option {
			t.Errorf("classifyFact(%q).option = true, want a metric", s)
		}
	}
}
