package deckplan

import (
	"fmt"
	"strings"
)

// Budget accounting (go-slide-creator-58qda).
//
// A plan asked for 8 slides used to meet the number with an agenda and two
// section dividers, leaving five authored slides for seven topics, and said
// nothing about it. The budget now counts content slides first: an agenda and
// dividers are drafted only at chapterBudget slides or more, or when the brief
// asks for them, and only from room the content left over. Every plan states
// how the budget was spent and what was cut.

// Budget is how a plan spent its slide budget.
type Budget struct {
	// Requested is the budget the plan was drafted to: the slide_budget
	// argument, else the slide count the brief states, else the default.
	Requested int `json:"requested"`
	// Planned is the rendered length of the plan, generated agenda and
	// dividers included.
	Planned int `json:"planned"`
	// Content counts the slides that carry the brief's argument.
	Content int `json:"content"`
	// Structural counts the slides that only frame it: the cover, an agenda,
	// section dividers, an appendix divider, a pattern-less closing page.
	Structural int `json:"structural"`
	// StructuralSlides names each structural slide. Always present.
	StructuralSlides []string `json:"structural_slides"`
	// Cut lists what the budget left out, each with the reason. Always
	// present; empty when nothing was cut.
	Cut []BudgetCut `json:"cut"`
}

// BudgetCut is one thing the budget left out of the plan.
type BudgetCut struct {
	What   string `json:"what"`
	Reason string `json:"reason"`
}

// chapterBudget is the smallest budget drafted with an agenda and section
// dividers when the brief did not ask for them: below it they would be bought
// with content slides.
const chapterBudget = 12

// newBudget returns an empty account for the requested budget, with the
// always-present arrays initialised.
func newBudget(requested int) Budget {
	return Budget{Requested: requested, StructuralSlides: []string{}, Cut: []BudgetCut{}}
}

// structural records one structural slide.
func (b *Budget) structural(name string) {
	b.Structural++
	b.StructuralSlides = append(b.StructuralSlides, name)
}

// cut records one thing left out.
func (b *Budget) cut(what, reason string) {
	b.Cut = append(b.Cut, BudgetCut{What: what, Reason: reason})
}

// note is the one-line account every plan carries in budget_note, followed by
// any further explanation.
func (b Budget) note(extra ...string) string {
	var s strings.Builder
	fmt.Fprintf(&s, "Planned %d of %d slides: %d content", b.Planned, b.Requested, b.Content)
	if b.Structural > 0 {
		fmt.Fprintf(&s, ", %d structural (%s)", b.Structural, strings.Join(b.StructuralSlides, ", "))
	} else {
		s.WriteString(", no structural slides")
	}
	s.WriteString(".")
	if len(b.Cut) == 0 {
		s.WriteString(" Nothing was cut.")
	} else {
		parts := make([]string, len(b.Cut))
		for i, c := range b.Cut {
			parts[i] = c.What + " (" + c.Reason + ")"
		}
		s.WriteString(" Cut: " + strings.Join(parts, "; ") + ".")
	}
	for _, e := range extra {
		if e = strings.TrimSpace(e); e != "" {
			s.WriteString(" " + strings.ToUpper(e[:1]) + e[1:])
			if !strings.HasSuffix(e, ".") {
				s.WriteString(".")
			}
		}
	}
	return s.String()
}

// chaptersAllowed reports whether a plan may carry an agenda and section
// dividers: the brief asks for them, or the budget is chapterBudget or more
// and the brief does not rule them out.
func chaptersAllowed(budget int, cs []Constraint) (allowed, asked bool) {
	agenda, noAgenda := structureAsked(cs, "agenda")
	dividers, noDividers := structureAsked(cs, "dividers")
	if noAgenda || noDividers {
		return false, false
	}
	if agenda || dividers {
		return true, true
	}
	return budget >= chapterBudget, false
}
