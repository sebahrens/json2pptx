package deckplan

import (
	"fmt"
	"regexp"
	"strings"
)

// Brief-gated slots for the raw plan (go-slide-creator-tu35a,
// go-slide-creator-kod2i).
//
// The narrative arc used to allot a fixed share of every plan to a
// "framework" slot seeded "Structure or methodology overview", to emphasis
// slots seeded "Standout metric or memorable takeaway", and to evidence and
// comparison slots seeded with placeholder prose — whatever the brief said.
// make_deck turned those seeds into slide titles verbatim, and an agent copying
// the plan built a pyramid of exemplar tiers and a pull-quote from nobody into
// a QBR. A slot now exists only when the brief carries what it shows: a named
// framework, a headline number or a quote, a fact to prove, an option to
// compare.

// cueFramework marks a brief that names a framework, methodology or structure.
var cueFramework = regexp.MustCompile(`(?i)\b(?:framework|methodology|method|operating model|business model|model|structure|approach|pillars?|architecture|canvas|maturity|capabilit(?:y|ies)|value chain|swot)\b`)

// cueQuote marks a quoted voice in the brief.
var cueQuote = regexp.MustCompile(`["“][^"”]{6,}["”]`)

// frameworkClauses returns the brief clauses that name a framework.
func frameworkClauses(brief string) []string {
	var out []string
	for _, c := range splitBriefClauses(brief) {
		c = strings.TrimSpace(factListMarker.ReplaceAllString(strings.TrimSpace(c), ""))
		if c != "" && cueFramework.MatchString(c) {
			out = append(out, balanceFactBrackets(TruncateBrief(c, maxFactLen)))
		}
	}
	return out
}

// frameworkClause returns the first brief clause that names a framework, or "".
func frameworkClause(brief string) string {
	if cs := frameworkClauses(brief); len(cs) > 0 {
		return cs[0]
	}
	return ""
}

// briefHasQuote reports whether the brief quotes someone.
func briefHasQuote(brief string) bool {
	return cueQuote.MatchString(brief)
}

// briefHasHeadlineNumber reports whether the brief carries a metric an
// emphasis slide could put front and centre.
func briefHasHeadlineNumber(brief string) bool {
	for _, f := range extractBriefFacts(brief) {
		if f.numeric {
			return true
		}
	}
	return false
}

// gateBriefRoles moves the framework and emphasis shares of the arc to
// evidence when the brief carries nothing for them: a framework slot needs a
// named framework (one slot per framework the brief names), an emphasis slot a
// headline number or a quote.
func gateBriefRoles(brief string, order []string, counts map[string]int) []string {
	moved := 0
	if named := len(frameworkClauses(brief)); counts["framework"] > named {
		moved += counts["framework"] - named
		counts["framework"] = named
		if named == 0 {
			delete(counts, "framework")
		}
	}
	if !briefHasQuote(brief) && !briefHasHeadlineNumber(brief) {
		moved += counts["emphasis"]
		delete(counts, "emphasis")
	}
	if moved == 0 {
		return order
	}
	if !containsStr(order, "evidence") {
		order = append(order, "evidence")
	}
	counts["evidence"] += moved
	return order
}

// comparisonClause returns the first brief clause that proposes a choice, so
// a comparison slot is seeded with the comparison the brief actually makes.
func comparisonClause(brief string) string {
	for _, raw := range splitBriefClauses(brief) {
		c := strings.TrimSpace(factListMarker.ReplaceAllString(strings.TrimSpace(raw), ""))
		lower := " " + strings.ToLower(factBenchmark.ReplaceAllString(c, "")) + " "
		for _, cue := range comparisonCues {
			if strings.Contains(lower, cue) {
				// Cut the way extractBriefFacts cuts — at an open bracket or
				// a word boundary, never inside an amount — so a long
				// comparison keeps both options and matches its fact
				// (go-slide-creator-ze5u7).
				pieces := splitOpenBrackets(c)
				if len(pieces) == 0 {
					return ""
				}
				return chunkFact(pieces[0], maxComparisonFactLen)[0]
			}
		}
	}
	return ""
}

// factDrivenRoles are the roles a slide holds only on the strength of the
// brief facts routed to it.
var factDrivenRoles = map[string]bool{"evidence": true, "comparison": true, "emphasis": true}

// dropUnsupportedSlots removes evidence, comparison and emphasis slides that
// received no brief fact — a slot with nothing to show was seeded with
// placeholder prose that make_deck shipped as a title. must_include
// placements are kept. One content slide always remains so the plan is never
// just a cover and a close. Slide indices are renumbered; the return value is
// how many slides were dropped.
func dropUnsupportedSlots(slides []Slide) ([]Slide, int) {
	keep := make([]Slide, 0, len(slides))
	dropped := 0
	middle := 0
	var firstDropped *Slide
	for i := range slides {
		s := slides[i]
		if factDrivenRoles[s.NarrativeRole] && len(s.Facts) == 0 && s.Rationale != mustIncludeRationale {
			if firstDropped == nil {
				firstDropped = &slides[i]
			}
			dropped++
			continue
		}
		if !isStructuralRole(s.NarrativeRole) {
			middle++
		}
		keep = append(keep, s)
	}
	if middle == 0 && firstDropped != nil {
		// Nothing in the brief to route: keep one content slot and let the
		// budget note ask for the facts.
		restored := *firstDropped
		insert := len(keep)
		if insert > 0 && keep[insert-1].NarrativeRole == "closing" {
			insert--
		}
		keep = append(keep[:insert], append([]Slide{restored}, keep[insert:]...)...)
		dropped--
	}
	for i := range keep {
		keep[i].SlideIndex = i
	}
	return keep, dropped
}

// droppedSlotsNote explains a plan shortened by dropUnsupportedSlots.
func droppedSlotsNote(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("planned %d fewer slide(s) than the budget: no brief fact, option or quote supports them, and a slot with nothing to show would only carry placeholder prose — add the numbers, names, options or quotes those slides would prove", n)
}

// cueBranching marks a brief whose process has decision points — the one
// thing process-flow draws that numbered-step-strip does not.
var cueBranching = regexp.MustCompile(`(?i)\b(?:decision points?|decide|if|whether|branch\w*|go/no-go|escalat\w*|exceptions?|approval gates?)\b`)

// preferStepStrip swaps process-flow for numbered-step-strip when the brief
// describes no branching (go-slide-creator-tu35a). A single-row process-flow
// standing alone on a slide is the sparse-flow anti-pattern RULES.md warns
// about; process-flow earns its place only with decision diamonds. must_include
// placements are kept.
func preferStepStrip(slides []Slide, brief string) {
	if cueBranching.MatchString(brief) {
		return
	}
	for i := range slides {
		if slides[i].RecommendedPattern == "process-flow" && slides[i].Rationale != mustIncludeRationale {
			slides[i].RecommendedPattern = "numbered-step-strip"
			slides[i].Rationale = "straight sequence: numbered-step-strip (process-flow is for flows with decision points)"
		}
	}
}
