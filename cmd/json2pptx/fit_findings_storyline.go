package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Storyline findings: the consulting rules QUALITY.md states and the
// deterministic loop used to leave unenforced — a so-what on every evidence
// slide, an executive summary up front, a closer that asks for something, and
// text an audience can read at a glance (go-slide-creator-cti7s, -kuurd,
// -fle6s). All are review-weighted and feed score_deck's quality gate.

// slideMissesTakeaway reports a slide that argues from data, states no
// takeaway, and whose title is not itself the takeaway. validate_input's
// warning and the fit-report finding share it, so the two never disagree.
func slideMissesTakeaway(s SlideInput) bool {
	return strings.TrimSpace(s.Takeaway) == "" && slideRequiresTakeaway(s) &&
		!slideTitleStatesTakeaway(s) && !slideHasChartSoWhat(s)
}

// takeawayMissingError is the takeaway_missing diagnostic for slide i. One
// constructor keeps validate_input's warning and the fit finding identical
// (same path, code and message), so the envelope dedupes them.
func takeawayMissingError(i int) *patterns.ValidationError {
	return &patterns.ValidationError{
		Path:    slidepath.SlideField(i, "takeaway"),
		Code:    patterns.ErrCodeTakeawayMissing,
		Message: fmt.Sprintf("slide %d: this slide argues from data — set a takeaway headline (or make the title a full sentence) so the audience knows the 'so what'", i+1),
		Fix: &patterns.FixSuggestion{
			Kind:   "provide_value",
			Params: map[string]any{"field": "takeaway"},
		},
	}
}

// collectTakeawayMissingFindings reports takeaway_missing into the fit-finding
// set. It used to be emitted only by validate_input's template pass, so the
// score_deck gate criterion require_takeaway_on_charts counted findings that
// never reached it and could not fire (go-slide-creator-cti7s). It reads the
// authored slides, before pattern expansion erases which pattern was asked for.
func collectTakeawayMissingFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var out []patterns.FitFinding
	for i := range input.Slides {
		if !slideMissesTakeaway(input.Slides[i]) {
			continue
		}
		out = append(out, patterns.FitFinding{ValidationError: *takeawayMissingError(i), Action: "review"})
	}
	return out
}

// --- Action titles -----------------------------------------------------------

// titleActionMaxWords is the longest action title that still holds on two
// lines and reads as one claim (QUALITY.md §2; Deckary "maximum 15 words").
const titleActionMaxWords = 15

// titleSentenceMinWords is the length from which a title written in sentence
// case is taken as a sentence even when its verb is outside the lexicon. Topic
// labels that long are written in Title Case; the rule keeps the verb lexicon's
// gaps from flagging real action titles.
const titleSentenceMinWords = 6

// titleStockLabelMaxWords bounds the "<topic> overview" suffix rule to short
// labels: "Approve the plan at the next board review" ends in a stock word and
// is a sentence.
const titleStockLabelMaxWords = 4

// titleStockLabels are the generic labels agents reach for instead of a claim.
var titleStockLabels = map[string]bool{
	"overview": true, "summary": true, "executive summary": true, "exec summary": true,
	"key metrics": true, "key findings": true, "key takeaways": true, "key highlights": true,
	"key insights": true, "highlights": true, "next steps": true, "background": true,
	"introduction": true, "context": true, "results": true, "findings": true,
	"recommendation": true, "recommendations": true, "conclusion": true, "conclusions": true,
	"update": true, "status": true, "status update": true, "analysis": true,
	"discussion": true, "objectives": true, "situation": true, "approach": true,
	"methodology": true, "performance": true, "kpis": true, "metrics": true,
	"options": true, "risks": true, "issues": true, "timeline": true, "roadmap": true,
	"plan": true, "strategy": true, "solution": true, "opportunity": true,
	"market overview": true, "financial overview": true, "financials": true,
}

// titleStockSuffixes turn a short "<topic> <suffix>" title into a stock label:
// "Revenue Overview", "Margin Analysis", "Pipeline Metrics".
var titleStockSuffixes = []string{"overview", "analysis", "update", "summary", "highlights", "metrics", "review", "results", "deep dive", "deep-dive"}

// titleNavigationPatterns are patterns whose slide is navigation or the
// closing ask: a short label is the convention there.
var titleNavigationPatterns = map[string]bool{
	"agenda":             true,
	"agenda-with-images": true,
	"next-steps":         true,
}

// extraTitleVerbs widen the action-title lexicon with verbs that commonly
// carry consulting claims and are rarely the head of a topic label.
var extraTitleVerbs = map[string]bool{
	"explain": true, "carry": true, "show": true, "face": true, "put": true,
	"set": true, "move": true, "cost": true, "threaten": true, "protect": true,
	"create": true, "depend": true, "hinge": true, "signal": true, "suggest": true,
	"confirm": true, "offset": true, "return": true, "recover": true, "peak": true,
	"concentrate": true, "sit": true, "come": true, "go": true, "get": true,
	"give": true, "hold": true, "run": true, "want": true, "outgrow": true,
	"overtake": true, "own": true, "cover": true, "support": true, "justify": true,
	"deserve": true, "demand": true, "warrant": true, "block": true, "limit": true,
	"erode": true, "compress": true, "squeeze": true, "boost": true, "raise": true,
	"lower": true, "help": true, "hurt": true, "fail": true, "succeed": true,
	"struggle": true, "account": true, "represent": true, "generate": true,
	"contribute": true, "drag": true, "weigh": true, "approve": true, "invest": true,
	"prioritise": true, "prioritize": true, "focus": true, "choose": true,
	"stop": true, "start": true, "accept": true, "reject": true, "recommend": true,
	"exit": true, "buy": true, "sell": true, "act": true, "fix": true, "avoid": true,
	"capture": true, "reach": true, "land": true, "clear": true, "turn": true,
	"bring": true, "outweigh": true, "point": true, "flatten": true, "plateau": true,
	"frame": true, "change": true, "pull": true, "push": true, "say": true, "tell": true, "know": true, "see": true,
	"expect": true, "earn": true,
}

func titleWordIsVerb(word string) bool {
	if isTitleVerb(word) {
		return true
	}
	w := strings.ToLower(strings.TrimFunc(word, func(r rune) bool { return !unicode.IsLetter(r) }))
	if w == "" {
		return false
	}
	if takeawayTitleVerbs[w] || extraTitleVerbs[w] {
		return true
	}
	return (strings.HasSuffix(w, "s") && extraTitleVerbs[strings.TrimSuffix(w, "s")]) ||
		(strings.HasSuffix(w, "es") && extraTitleVerbs[strings.TrimSuffix(w, "es")])
}

// titleIsStockLabel reports a generic label: an exact stock title, or a short
// "<topic> overview / analysis / ..." title.
func titleIsStockLabel(title string) bool {
	norm := normalizeTitleText(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) || r == '-' || r == '&' {
			return r
		}
		return ' '
	}, title))
	if titleStockLabels[norm] {
		return true
	}
	words := strings.Fields(norm)
	if len(words) < 2 || len(words) > titleStockLabelMaxWords {
		return false
	}
	for _, suffix := range titleStockSuffixes {
		if strings.HasSuffix(norm, " "+suffix) {
			return true
		}
	}
	return false
}

// titleInSentenceCase reports a title whose longer words after the first are
// mostly lower case — the shape of a sentence, not a Title Case label.
func titleInSentenceCase(words []string) bool {
	upper, total := 0, 0
	for _, w := range words[1:] {
		letters := strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) })
		if len([]rune(letters)) < 4 {
			continue
		}
		total++
		if r := []rune(letters)[0]; unicode.IsUpper(r) {
			upper++
		}
	}
	return total > 0 && upper*2 < total
}

// titleNotActionReason classifies a content-slide title, returning "" when it
// reads as an action title.
func titleNotActionReason(title string, hasTakeaway bool) string {
	words := strings.Fields(title)
	switch {
	case len(words) > titleActionMaxWords:
		return "too_long"
	case titleIsStockLabel(title):
		return "stock_label"
	}
	// A continuation marker is not a number in the claim: "Savings by lever
	// (1/2)" is the label "Savings by lever".
	if part, ok := rhythm.ParseContinuation(title); ok {
		words = strings.Fields(part.Base)
		title = part.Base
	}
	if strings.ContainsAny(title, "0123456789") {
		return ""
	}
	if len(words) >= titleSentenceMinWords && titleInSentenceCase(words) {
		return ""
	}
	for _, w := range words {
		if titleWordIsVerb(w) {
			return ""
		}
	}
	if hasTakeaway {
		// A slide that states its point in a takeaway may title the topic in
		// a phrase (the DeckSpec title + takeaway convention), but a two- or
		// three-word label over a takeaway is still a slide without a
		// headline: six of them scored 100 (go-slide-creator-wwmod).
		if len(words) <= titleTopicLabelMaxWords {
			return "topic_label"
		}
		return ""
	}
	return "no_verb_or_number"
}

// titleTopicLabelMaxWords is the longest verbless, numberless title that
// counts as a bare label on a slide that carries a takeaway.
const titleTopicLabelMaxWords = 4

// titleExemptFromAction reports slides whose title may stay a label: navigation
// patterns, navigation titles, and a short label on the deck's last slide (the
// closer — CLOSING_WITHOUT_NEXT_STEPS judges a thank-you closer instead).
func titleExemptFromAction(input *PresentationInput, si int, title string) bool {
	slide := input.Slides[si]
	if slide.Pattern != nil && titleNavigationPatterns[slide.Pattern.Name] {
		return true
	}
	norm := normalizeTitleText(strings.TrimRight(title, ".:!?"))
	if titleTopicExempt[norm] || strings.Contains(norm, "agenda") || strings.Contains(norm, "table of contents") {
		return true
	}
	return si == len(input.Slides)-1 && len(input.Slides) > 1 && len(strings.Fields(title)) <= titleStockLabelMaxWords
}

func titleNotActionMessage(si int, title, reason string, words int) (string, string) {
	switch reason {
	case "too_long":
		return fmt.Sprintf("slide %d: title has %d words — an action title states one claim in at most %d words so it holds on two lines", si+1, words, titleActionMaxWords),
			"cut the title to the claim (<= 15 words) and move the qualifiers into the body or takeaway"
	case "stock_label":
		return fmt.Sprintf("slide %d: title %q is a stock label, not the slide's point — state the conclusion the slide proves", si+1, title),
			"rewrite as an action title, e.g. \"Key Metrics\" -> \"ARR reached $48M and every metric improved except SMB churn\""
	case "topic_label":
		return fmt.Sprintf("slide %d: title %q is a label; the slide's point sits in the takeaway — put the claim in the title", si+1, title),
			"promote the takeaway to the title (a sentence with a verb or a number) and keep or drop the takeaway line"
	default:
		return fmt.Sprintf("slide %d: title %q names a topic, not the slide's point — state the takeaway as a sentence with a verb or a number", si+1, title),
			"rewrite as an action title, e.g. \"Market Overview\" -> \"Mid-market demand doubled while enterprise stalled\""
	}
}

// --- Deck storyline --------------------------------------------------------------

// execSummaryMinSlides is the deck size from which an executive summary is
// expected: a five-slide update can open on its evidence, a board deck cannot.
const execSummaryMinSlides = 6

// closingMinSlides is the deck size from which a courtesy closer is judged.
const closingMinSlides = 3

var execSummaryPatterns = map[string]bool{"exec-summary": true, "scqa-summary": true}

// execSummaryTitleMarkers are title fragments that mark a slide as the
// executive summary on the raw path.
var execSummaryTitleMarkers = []string{"executive summary", "exec summary", "summary", "key takeaways", "at a glance", "bottom line", "tl;dr"}

func slideIsExecSummary(slide SlideInput) bool {
	if slide.Pattern != nil && execSummaryPatterns[slide.Pattern.Name] {
		return true
	}
	if slide.Compose != nil && composeHasPattern(slide.Compose, execSummaryPatterns) {
		return true
	}
	_, title := extractTitleText(slide)
	norm := normalizeTitleText(title)
	for _, marker := range execSummaryTitleMarkers {
		if strings.Contains(norm, marker) {
			return true
		}
	}
	return false
}

func slideIsNextSteps(slide SlideInput) bool {
	names := map[string]bool{"next-steps": true}
	if slide.Pattern != nil && names[slide.Pattern.Name] {
		return true
	}
	if slide.Compose != nil && composeHasPattern(slide.Compose, names) {
		return true
	}
	_, title := extractTitleText(slide)
	return strings.Contains(normalizeTitleText(title), "next step")
}

func composeHasPattern(c *ComposeInput, names map[string]bool) bool {
	for i := range c.Segments {
		seg := &c.Segments[i]
		if seg.HasPattern() && names[seg.Pattern.Name] {
			return true
		}
		if seg.Compose != nil && composeHasPattern(seg.Compose, names) {
			return true
		}
	}
	return false
}

// titleIsCourtesyCloser reports "Thank you", "Questions?", "Q&A" and kin.
func titleIsCourtesyCloser(title string) bool {
	norm := normalizeTitleText(strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsSpace(r) || r == '&' {
			return r
		}
		return ' '
	}, title))
	if norm == "" {
		return false
	}
	switch norm {
	case "thanks", "questions", "any questions", "q&a", "q & a", "questions and answers", "questions & answers", "discussion", "open discussion":
		return true
	}
	return strings.HasPrefix(norm, "thank you") || strings.HasPrefix(norm, "thanks ")
}

// collectStorylineFindings reports the deck-level storyline gaps: no executive
// summary on a deck of six or more slides (NO_EXECUTIVE_SUMMARY) and a
// thank-you closer with no next steps anywhere (CLOSING_WITHOUT_NEXT_STEPS)
// — go-slide-creator-kuurd.
func collectStorylineFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	n := len(input.Slides)
	var out []patterns.FitFinding
	if n >= execSummaryMinSlides {
		found := false
		for i := range input.Slides {
			if slideIsExecSummary(input.Slides[i]) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path:    slidepath.Slide(1),
					Code:    patterns.ErrCodeNoExecutiveSummary,
					Message: fmt.Sprintf("deck of %d slides has no executive summary — open with the whole answer (3-5 lead-in statements) right after the title slide", n),
					Fix: &patterns.FixSuggestion{
						Kind: "adopt_pattern",
						Params: map[string]any{
							"pattern":  "exec-summary",
							"kind":     "executive_summary",
							"position": 1,
							"hint":     "insert an executive_summary slide (DeckSpec) or an exec-summary / scqa-summary pattern slide (raw) after the cover",
						},
					},
				},
				Action: "review",
			})
		}
	}
	if n >= closingMinSlides {
		last := n - 1
		_, title, ci := extractTitleTextAt(input.Slides[last])
		if titleIsCourtesyCloser(title) && !deckHasNextSteps(input) {
			path := slidepath.Slide(last)
			if ci >= 0 {
				path = slidepath.ContentIndex(last, ci)
			}
			out = append(out, patterns.FitFinding{
				ValidationError: patterns.ValidationError{
					Path:    path,
					Code:    patterns.ErrCodeClosingWithoutNextSteps,
					Message: fmt.Sprintf("slide %d: the deck closes on %q and never states its next steps — end on the ask: owners, dates and the decisions requested", last+1, title),
					Fix: &patterns.FixSuggestion{
						Kind: "adopt_pattern",
						Params: map[string]any{
							"pattern": "next-steps",
							"kind":    "next_steps",
							"hint":    "replace the closer with a next_steps slide (DeckSpec) / next-steps pattern (raw), or retitle it as the action",
						},
					},
				},
				Action: "review",
			})
		}
	}
	return out
}

// askTitleMarkers are title fragments of a slide that states the ask — the
// role a next-steps slide plays ("We ask the committee for three decisions
// today", "Approve the pilot budget by 15 March").
var askTitleMarkers = []string{"next step", "decision", "we ask", "the ask", "approve", "call to action", "action plan"}

func deckHasNextSteps(input *PresentationInput) bool {
	for i := range input.Slides {
		slide := input.Slides[i]
		if slideIsNextSteps(slide) {
			return true
		}
		_, title := extractTitleText(slide)
		norm := normalizeTitleText(title)
		for _, marker := range askTitleMarkers {
			if strings.Contains(norm, marker) {
				return true
			}
		}
	}
	return false
}

// --- Text density ---------------------------------------------------------------

// Per-slide text ceilings (6x6 / ~75-80-word glance test, go-slide-creator-fle6s).
// The present-mode word budget equals one placeholder's BODY_TOO_LONG budget
// so a reduce_text repair clears both findings at once.
// The word budget follows the viewing mode: a projected deck is read in a
// glance, a read-ahead deck can carry more.
const (
	slideDenseMaxBullets       = 6
	slideDenseMaxWordsPresent  = maxBodyWords
	slideDenseMaxWordsRead     = 110
	slideDenseMaxBulletLines   = 2
	slideDenseBulletIndentsEMU = 2*91440 + 342900 // text insets + hanging bullet indent
)

func slideDenseWordBudget(viewingMode string) int {
	if strings.EqualFold(strings.TrimSpace(viewingMode), "read") {
		return slideDenseMaxWordsRead
	}
	return slideDenseMaxWordsPresent
}

// bodyTextItem reports a placeholder content item that carries body copy.
func bodyTextItem(c *ContentInput) bool {
	if isHeadlinePlaceholderID(c.PlaceholderID) || strings.Contains(strings.ToLower(c.PlaceholderID), "subtitle") {
		return false
	}
	switch c.Type {
	case "text", "bullets", "body_and_bullets", "body_and_lead", "bullet_groups":
		return true
	}
	return false
}

// contentBullets returns the bullet paragraphs of a body item (not its lead or
// body paragraphs, which are prose).
func contentBullets(c *ContentInput) []string {
	switch c.Type {
	case "bullets":
		if c.BulletsValue != nil {
			return *c.BulletsValue
		}
	case "body_and_bullets":
		if c.BodyAndBulletsValue != nil {
			return c.BodyAndBulletsValue.Bullets
		}
	case "body_and_lead":
		if c.BodyAndLeadValue != nil {
			return c.BodyAndLeadValue.Bullets
		}
	case "bullet_groups":
		if c.BulletGroupsValue != nil {
			var out []string
			for _, g := range c.BulletGroupsValue.Groups {
				out = append(out, g.Bullets...)
			}
			return out
		}
	}
	return nil
}

// collectTextDensityFindings reports SLIDE_TEXT_DENSE: a content slide whose
// placeholder body text carries more than six bullets in one placeholder, more
// words than the viewing mode's budget, or a bullet that wraps past two lines
// at its rendered size. The only ceilings before were per placeholder (80
// words) and in the input-only quality score, so a seven-bullet wall passed
// every gate (go-slide-creator-fle6s).
func collectTextDensityFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var predicted []*types.LayoutMetadata
	if len(layouts) > 0 {
		predicted = predictSlideLayouts(input, layouts)
	}
	wordBudget := slideDenseWordBudget(input.ViewingMode)
	var out []patterns.FitFinding
	for si := range input.Slides {
		slide := &input.Slides[si]
		if !slideQualifiesForDuplicateTitleCheck(*slide) || !slideCarriesArgument(*slide, layouts...) {
			continue
		}
		d := measureSlideDensity(slide, si, predicted)
		words, maxBullets, longBullets, firstIdx, bulletIdx, plainBulletsOnly := d.words, d.maxBullets, d.longBullets, d.firstIdx, d.bulletIdx, d.plainBulletsOnly
		if firstIdx < 0 {
			continue
		}
		var tripped []string
		if maxBullets > slideDenseMaxBullets {
			tripped = append(tripped, "bullets")
		}
		if words > wordBudget {
			tripped = append(tripped, "words")
		}
		if longBullets > 0 {
			tripped = append(tripped, "long_bullets")
		}
		if len(tripped) == 0 {
			continue
		}
		path := slidepath.ContentIndex(si, firstIdx)
		if bulletIdx >= 0 && maxBullets > slideDenseMaxBullets {
			path = slidepath.ContentIndex(si, bulletIdx)
		}
		params := map[string]any{
			"limits":             tripped,
			"words":              words,
			"max_words":          wordBudget,
			"bullets":            maxBullets,
			"max_bullets":        slideDenseMaxBullets,
			"long_bullets":       longBullets,
			"max_lines_per_item": slideDenseMaxBulletLines,
			"hint":               "split the slide so each carries one message with at most 6 one-to-two-line bullets, or cut the bullets that restate the title",
		}
		fix := &patterns.FixSuggestion{Kind: "rewrite_field", Params: params}
		if maxBullets > slideDenseMaxBullets && plainBulletsOnly {
			params["max_items"] = slideDenseMaxBullets
			fix = &patterns.FixSuggestion{Kind: "split_bullets", Params: params}
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    path,
				Code:    patterns.ErrCodeSlideTextDense,
				Message: denseMessage(si, words, wordBudget, maxBullets, longBullets),
				Fix:     fix,
			},
			Action: "review",
		})
	}
	return out
}

// slideDensity is one slide's placeholder body text, measured.
type slideDensity struct {
	words, maxBullets, longBullets int
	firstIdx, bulletIdx            int
	plainBulletsOnly               bool
}

func measureSlideDensity(slide *SlideInput, si int, predicted []*types.LayoutMetadata) slideDensity {
	d := slideDensity{firstIdx: -1, bulletIdx: -1, plainBulletsOnly: true}
	for ci := range slide.Content {
		item := &slide.Content[ci]
		if !bodyTextItem(item) {
			continue
		}
		if d.firstIdx < 0 {
			d.firstIdx = ci
		}
		if item.Type != "bullets" {
			d.plainBulletsOnly = false
		}
		for _, p := range extractContentParagraphs(item) {
			d.words += len(strings.Fields(p))
		}
		bullets := contentBullets(item)
		if len(bullets) > d.maxBullets {
			d.maxBullets, d.bulletIdx = len(bullets), ci
		}
		d.longBullets += countLongBullets(item, bullets, si, predicted)
	}
	return d
}

func denseMessage(si, words, budget, bullets, long int) string {
	var parts []string
	if bullets > slideDenseMaxBullets {
		parts = append(parts, fmt.Sprintf("%d bullets (max %d)", bullets, slideDenseMaxBullets))
	}
	if words > budget {
		parts = append(parts, fmt.Sprintf("%d words of body text (budget %d)", words, budget))
	}
	if long > 0 {
		parts = append(parts, fmt.Sprintf("%d bullet(s) wrapping past %d lines", long, slideDenseMaxBulletLines))
	}
	return fmt.Sprintf("slide %d: text wall — %s; an audience reads a slide at a glance, so split it or cut to the points that prove the title", si+1, strings.Join(parts, ", "))
}

// countLongBullets measures each bullet at the size generation renders the
// placeholder (template size, density-normalised) and counts those that wrap
// past two lines. Without a resolved placeholder it measures nothing.
func countLongBullets(item *ContentInput, bullets []string, si int, predicted []*types.LayoutMetadata) int {
	if len(bullets) == 0 || si >= len(predicted) || predicted[si] == nil {
		return 0
	}
	ph := findPlaceholderByID(item.PlaceholderID, predicted[si].Placeholders)
	if ph == nil || ph.Bounds.Width <= slideDenseBulletIndentsEMU {
		return 0
	}
	size := effectivePlaceholderFontSizeHPt(item, ph, len(extractContentParagraphs(item)))
	if size <= 0 {
		return 0
	}
	font := ph.FontFamily
	if font == "" {
		font = "Liberation Sans"
	}
	width := ph.Bounds.Width - slideDenseBulletIndentsEMU
	long := 0
	for _, b := range bullets {
		m, err := textfit.MeasureRun(b, font, float64(size)/100, width, 0)
		if err == nil && m.Lines > slideDenseMaxBulletLines {
			long++
		}
	}
	return long
}
