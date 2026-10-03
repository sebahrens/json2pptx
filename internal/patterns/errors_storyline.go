package patterns

import "errors"

// Storyline and density findings: the consulting rules the skill states but
// the deterministic loop used to leave unenforced (go-slide-creator-kuurd,
// go-slide-creator-fle6s). All are review-level: they cost score points and
// feed the score_deck quality gate's storyline criteria, but never block a
// render.
//
//   - NoExecutiveSummary: a deck of six or more slides has no executive
//     summary (exec-summary / scqa-summary pattern, or a slide titled as one).
//   - ClosingWithoutNextSteps: the deck ends on a "Thank you" / "Questions?"
//     slide and nowhere states its next steps.
//   - SlideTextDense: a slide's placeholder body text is a wall — more than six
//     bullets, more words than the viewing mode's budget, or a bullet that wraps
//     past two lines at its rendered size.
//
// QualityGate is not a fit finding: the DeckSpec surfaces report it when the
// quality gate fails on a criterion no single finding accounts for (the score
// floor, the action-title share, the composition floor, the problem-slide
// share), so a failed gate always has a finding to name
// (go-slide-creator-x9rhq).
const (
	ErrCodeQualityGate             = "QUALITY_GATE"
	ErrCodeNoExecutiveSummary      = "NO_EXECUTIVE_SUMMARY"
	ErrCodeClosingWithoutNextSteps = "CLOSING_WITHOUT_NEXT_STEPS"
	ErrCodeSlideTextDense          = "SLIDE_TEXT_DENSE"
)

var (
	ErrNoExecutiveSummary      = errors.New("deck has no executive summary")
	ErrClosingWithoutNextSteps = errors.New("deck closes on a thank-you slide without next steps")
	ErrSlideTextDense          = errors.New("slide carries more body text than an audience can read at a glance")
)

func init() {
	codeSentinel[ErrCodeNoExecutiveSummary] = ErrNoExecutiveSummary
	codeSentinel[ErrCodeClosingWithoutNextSteps] = ErrClosingWithoutNextSteps
	codeSentinel[ErrCodeSlideTextDense] = ErrSlideTextDense

	contentCodes[ErrCodeNoExecutiveSummary] = true
	contentCodes[ErrCodeClosingWithoutNextSteps] = true
	contentCodes[ErrCodeSlideTextDense] = true

	findingMetaRegistry[ErrCodeQualityGate] = FindingMeta{
		Code:        ErrCodeQualityGate,
		Summary:     "The deck fails a quality-gate criterion that no single finding accounts for.",
		Severity:    "refuse",
		WhenEmitted: "validate_deck_spec, render_deck_spec and `semantic validate` / `semantic render` report it at `slides` when the quality gate fails on an aggregate criterion: the structural score is under min_score, too many slides lack an action title (max_topic_title_pct), the composition score is under min_composition_score, or too many slides carry findings (max_problem_slides_pct). The message carries the gate's own sentence and names the advisories counted. Always severity error and blocking; every other gate failure is reported on the finding that caused it.",
		RemediationSteps: []string{
			"Read the message: it states the criterion and the measured value against its threshold.",
			"Resolve the advisories it names (rewrite topic titles as action titles, fix the review findings on the slides listed) until the criterion is met; re-validate.",
			"For a deck whose brief rules out action titles, waive TITLE_NOT_ACTION in meta.waivers with a reason.",
		},
		RelatedCodes: []string{ErrCodeTitleNotAction, ErrCodeNoExecutiveSummary, ErrCodeClosingWithoutNextSteps},
	}
	findingMetaRegistry[ErrCodeNoExecutiveSummary] = FindingMeta{
		Code:        ErrCodeNoExecutiveSummary,
		Summary:     "A deck of six or more slides has no executive summary up front.",
		Severity:    "review",
		WhenEmitted: "The deck has at least six slides and none of them is an executive summary: no exec-summary or scqa-summary pattern (DeckSpec kind executive_summary compiles to one) and no slide titled \"Executive summary\", \"Summary\", \"Key takeaways\" or \"At a glance\". Reported once, on the second slide (where the summary belongs). Feeds the quality gate's require_storyline criterion.",
		RemediationSteps: []string{
			"Add an executive summary right after the title slide: DeckSpec kind executive_summary, or the exec-summary / scqa-summary pattern on the raw path.",
			"Give it an action title and 3-5 bold lead-in statements that together are the whole answer.",
		},
		RelatedCodes: []string{ErrCodeClosingWithoutNextSteps, ErrCodeTitleNotAction},
	}
	findingMetaRegistry[ErrCodeClosingWithoutNextSteps] = FindingMeta{
		Code:        ErrCodeClosingWithoutNextSteps,
		Summary:     "The deck ends on a \"Thank you\" / \"Questions?\" slide and never states its next steps.",
		Severity:    "review",
		WhenEmitted: "The last slide of a deck of three or more slides is titled as a courtesy closer (\"Thank you\", \"Thanks\", \"Questions\", \"Q&A\", \"Any questions?\") and no slide uses the next-steps pattern (DeckSpec kind next_steps) or carries a \"Next steps\" title. Feeds the quality gate's require_storyline criterion.",
		RemediationSteps: []string{
			"End on the ask: DeckSpec kind next_steps (raw: the next-steps pattern) with owners, dates and the decisions requested.",
			"Or retitle the closing slide as the action (\"Approve the pilot budget by 15 March to launch in Q3\").",
		},
		RelatedCodes: []string{ErrCodeNoExecutiveSummary, ErrCodeTitleNotAction},
	}
	findingMetaRegistry[ErrCodeSlideTextDense] = FindingMeta{
		Code:        ErrCodeSlideTextDense,
		Summary:     "A slide's placeholder body text is too dense to read at a glance (6x6 / ~75-word rule).",
		Severity:    "review",
		WhenEmitted: "A content slide's placeholder body text (text, bullets, body_and_bullets, body_and_lead, bullet_groups; titles and subtitles excluded) carries more than six bullets, more words than the viewing-mode budget (80 for viewing_mode \"present\", the default — the same budget as one placeholder's BODY_TOO_LONG, so reduce_text converges both; 110 for \"read\"), or a bullet that wraps past two lines at the size generation renders it. fix.params.limits names which limits tripped (bullets / words / long_bullets); fix.kind is split_bullets (max_items 6) for a plain-bullet wall, rewrite_field otherwise. Pattern text is budgeted by each pattern's own BODY_TOO_LONG checks instead.",
		RemediationSteps: []string{
			"Split the slide (fix.kind split_slide) so each slide carries one message with at most six bullets.",
			"Cut every bullet that restates the title or would be true of any company; keep each bullet to one or two lines.",
			"Or move the points into a pattern (labeled-rows, card-grid, exec-summary) that gives each a bold lead-in.",
		},
		RelatedCodes: []string{ErrCodeBodyTooLong, ErrCodeDeckMonotony},
	}
}
