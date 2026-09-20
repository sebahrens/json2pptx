package deckplan

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// Role capacity (go-slide-creator-whp97).
//
// Role allocation used to scale every slot count with the slide budget alone.
// A brief with one comparison — "compare build vs buy" — produced three
// comparison slots at a 20-slide budget, and the comparison role may only use
// the closed comparisonFamily set. Pattern selection was then asked to make
// three different-looking slides out of one idea and two available patterns,
// which no repeat cap can do: the residual surfaced as
// rhythm_check.repeated_families, which reported the problem rather than
// avoiding it.
//
// A role's slots are now bounded twice over: by the distinct pattern families
// it can draw on (three slots from two patterns must repeat) and by what the
// brief actually carries for it (one comparison in the text is one comparison
// slide). Surplus slots move to roles with headroom; if none has any, the plan
// comes back shorter than the budget and says why.

// comparisonCues mark a clause that proposes a choice between alternatives.
// They are matched against lower-cased clause text.
var comparisonCues = []string{
	" vs ", " vs. ", "versus", "compare", "comparison",
	"trade-off", "tradeoff", "instead of", "alternative",
	"option", "either ", "or buy", "before and after", "as-is", "to-be",
}

// comparisonClauseCount counts the clauses of a brief that propose a choice.
// Clauses, not cue occurrences: "compare build vs buy" carries three cues but
// is one comparison, and one comparison is one slide.
func comparisonClauseCount(brief string) int {
	n := 0
	for _, raw := range factClauseSplit.Split(brief, -1) {
		c := strings.ToLower(strings.TrimSpace(raw))
		if c == "" {
			continue
		}
		for _, cue := range comparisonCues {
			if strings.Contains(c, cue) {
				n++
				break
			}
		}
	}
	return n
}

// roleFamilyCapacity is how many slots a role can fill before it must repeat a
// pattern family. The comparison role draws from a closed set; every other role
// draws from the registry, counted by family so before-after and
// before-after-compact are one option, not two.
func roleFamilyCapacity(reg *patterns.Registry, role string) int {
	if role == "comparison" {
		return len(comparisonFamily)
	}
	taxRoles := narrativeRoleToTaxonomy[role]
	if len(taxRoles) == 0 || reg == nil {
		return 0 // unknown role: no basis for a cap
	}
	families := map[string]bool{}
	for _, pat := range reg.List() {
		tax := pat.Taxonomy()
		for _, want := range taxRoles {
			if containsStr(tax.NarrativeRole, want) {
				families[patternFamily(pat.Name())] = true
				break
			}
		}
	}
	return len(families)
}

// roleBriefCapacity is how many slots of a role the brief's own content
// supports, or 0 when the brief carries no signal the planner can read for that
// role (in which case only the family capacity applies).
//
// Only comparison has a signal specific enough to act on: a choice between
// alternatives is stated in words. Evidence and framework slots are fed by the
// fact extractor's quantity and entity clauses, which the arc already spreads
// across them, so capping those on the same count would shrink ordinary decks.
func roleBriefCapacity(role, brief string) int {
	if role != "comparison" {
		return 0
	}
	// A brief that proposes no choice still gets the arc's single decision
	// moment — the planner has always placed one, and removing it would change
	// every small plan. What it does not get is one per five slides.
	if n := comparisonClauseCount(brief); n > 0 {
		return n
	}
	return 1
}

// roleCapacity is the binding limit on a role's slots: the smaller of what its
// patterns and the brief support. 0 means "no limit known".
func roleCapacity(reg *patterns.Registry, role, brief string) int {
	fam := roleFamilyCapacity(reg, role)
	brf := roleBriefCapacity(role, brief)
	switch {
	case fam > 0 && brf > 0:
		return min(fam, brf)
	case fam > 0:
		return fam
	default:
		return brf
	}
}

// absorbOrder is the order surplus slots are offered to other roles: evidence
// carries detail without repeating itself, framework can hold another lens, and
// emphasis is last because its own count is capped downstream.
var absorbOrder = []string{"evidence", "framework", "emphasis"}

// applyRoleCapacity trims each role's slot count to its capacity and moves the
// surplus to roles with headroom. It returns the adjusted counts and, when the
// surplus could not be placed anywhere, a note naming what the plan gave up.
// counts is keyed by role; the returned map is the same object, mutated.
func applyRoleCapacity(reg *patterns.Registry, brief string, counts map[string]int, order []string) (map[string]int, string) {
	capacity := map[string]int{}
	for _, role := range order {
		capacity[role] = roleCapacity(reg, role, brief)
	}

	surplus := 0
	var trimmed []string
	for _, role := range order {
		limit := capacity[role]
		if limit <= 0 || counts[role] <= limit {
			continue
		}
		trimmed = append(trimmed, fmt.Sprintf("%s %d→%d", role, counts[role], limit))
		surplus += counts[role] - limit
		counts[role] = limit
	}
	if surplus == 0 {
		return counts, ""
	}

	for _, role := range absorbOrder {
		if surplus == 0 {
			break
		}
		if _, planned := counts[role]; !planned {
			continue
		}
		limit := capacity[role]
		if limit <= 0 {
			counts[role] += surplus
			surplus = 0
			break
		}
		if room := limit - counts[role]; room > 0 {
			take := min(room, surplus)
			counts[role] += take
			surplus -= take
		}
	}
	if surplus == 0 {
		return counts, ""
	}
	return counts, fmt.Sprintf(
		"planned %d fewer slide(s) than the budget: this brief supports %s, and the remaining slots would have repeated a pattern rather than said anything new — shorten the budget, or add the detail the extra slides would carry",
		surplus, strings.Join(trimmed, ", "))
}
