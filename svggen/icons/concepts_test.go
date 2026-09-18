package icons

import (
	"sort"
	"strings"
	"testing"
)

// go-slide-creator-3ojy: the concept map is only useful if every name in it
// actually ships. This test is what keeps it from drifting from the icon set.
func TestConceptsResolveToRealIcons(t *testing.T) {
	names, err := List("outline")
	if err != nil {
		t.Fatalf("listing outline icons: %v", err)
	}
	have := make(map[string]bool, len(names))
	for _, n := range names {
		have[n] = true
	}

	var missing []string
	for concept, icons := range Concepts {
		if len(icons) == 0 {
			t.Errorf("concept %q maps to no icons", concept)
		}
		for _, n := range icons {
			if !have[n] {
				missing = append(missing, concept+" → "+n)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d concept mapping(s) name an icon that is not in the outline set:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// The concepts the bead measured as returning zero substring hits must all
// resolve now.
func TestMatchConcepts_BusinessVocabulary(t *testing.T) {
	// Every one of these returned 0 results from the substring filter.
	zeroHitConcepts := []string{
		"strategy", "revenue", "customer", "efficiency", "governance",
		"compliance", "innovation", "cost", "profit", "roadmap", "milestone",
		"process", "security", "quality",
	}
	for _, c := range zeroHitConcepts {
		t.Run(c, func(t *testing.T) {
			got := ConceptNames(c)
			if len(got) == 0 {
				t.Fatalf("concept %q resolves to no icons", c)
			}
		})
	}
}

// "risk" used to return eight icons whose only connection was containing
// "asterisk", and "team" returned "brand-teams" / "ironing-steam". Whole-word
// concept matching must not reproduce that.
func TestMatchConcepts_NoSubstringNoise(t *testing.T) {
	for _, tc := range []struct{ query, mustNotContain string }{
		{"risk", "asterisk"},
		{"team", "brand-teams"},
		{"team", "ironing-steam"},
	} {
		t.Run(tc.query+"/"+tc.mustNotContain, func(t *testing.T) {
			for _, n := range ConceptNames(tc.query) {
				if n == tc.mustNotContain {
					t.Errorf("concept %q returned the substring-noise icon %q", tc.query, n)
				}
			}
		})
	}

	// And the concepts still return something sensible.
	if got := ConceptNames("risk"); len(got) == 0 || got[0] != "alert-triangle" {
		t.Errorf("risk → %v, want alert-triangle first", got)
	}
	if got := ConceptNames("team"); len(got) == 0 || got[0] != "users" {
		t.Errorf("team → %v, want users first", got)
	}
}

// A multi-word query reaches the concepts inside it, and an exact concept match
// outranks a word match.
func TestMatchConcepts_PhraseQueries(t *testing.T) {
	t.Run("phrase reaches its concept", func(t *testing.T) {
		got := ConceptNames("cost reduction")
		if len(got) == 0 {
			t.Fatal("\"cost reduction\" resolves to nothing")
		}
		if got[0] != "receipt" {
			t.Errorf("cost reduction → %v, want the cost icons first", got)
		}
	})

	t.Run("phrase reaching two concepts returns both", func(t *testing.T) {
		matches := MatchConcepts("customer retention")
		concepts := map[string]bool{}
		for _, m := range matches {
			concepts[m.Concept] = true
		}
		if !concepts["customer"] || !concepts["retention"] {
			t.Errorf("\"customer retention\" matched %v, want both customer and retention", concepts)
		}
	})

	t.Run("exact match outranks a word match", func(t *testing.T) {
		// "scale" is both a concept and a word inside no other key, so use
		// "process": exact "process" must outrank the phrase-only hits.
		matches := MatchConcepts("process")
		if len(matches) == 0 {
			t.Fatal("no matches")
		}
		if matches[0].Concept != "process" {
			t.Errorf("first match concept = %q, want the exact concept", matches[0].Concept)
		}
	})

	t.Run("hyphens and case are ignored", func(t *testing.T) {
		a := ConceptNames("Cost-Reduction")
		b := ConceptNames("cost reduction")
		if strings.Join(a, ",") != strings.Join(b, ",") {
			t.Errorf("Cost-Reduction → %v but cost reduction → %v", a, b)
		}
	})

	t.Run("unknown query resolves to nothing", func(t *testing.T) {
		if got := ConceptNames("zzzz nonsense"); len(got) != 0 {
			t.Errorf("expected no matches, got %v", got)
		}
	})
}

// Results must be deterministic and free of duplicate icon names.
func TestMatchConcepts_DeterministicAndDeduped(t *testing.T) {
	first := ConceptNames("customer retention")
	for i := 0; i < 20; i++ {
		if got := ConceptNames("customer retention"); strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("call %d returned %v, first returned %v", i, got, first)
		}
	}
	seen := map[string]bool{}
	for _, n := range first {
		if seen[n] {
			t.Errorf("duplicate icon name %q in results", n)
		}
		seen[n] = true
	}
}

func TestConceptKeys(t *testing.T) {
	keys := ConceptKeys()
	if len(keys) < 100 {
		t.Errorf("concept index has only %d keys; the bead asked for ~150 business concepts", len(keys))
	}
	if !sort.StringsAreSorted(keys) {
		t.Error("ConceptKeys must be sorted")
	}
}
