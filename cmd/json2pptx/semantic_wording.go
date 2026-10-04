package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// Fit findings in the words of the spec (go-slide-creator-micna, -vg73u).
//
// A fit finding is written by the pattern the slide compiled to, in that
// pattern's terms: "numbered-step-strip steps[3].body is 59 characters … drop
// the bodies or use fewer steps". The author of a decision slide wrote
// options[3].detail and has no steps, no bodies and no numbered-step-strip.
// The same goes for the pattern's own controls (show_legend, max_height_pct,
// "choose dots or chevron style"), which a slide kind does not have, and for
// text the layout itself writes (a "RECOMMENDED" badge): the finding asked
// the author to shorten it.
//
// deckSpecWording rewrites such a finding for the slide's kind: the pattern's
// name comes off the front, a value the pattern names is located in the spec
// by the text it carries, the pattern's words for its items become the kind's,
// and advice about a control the kind does not have is replaced by what the
// kind can do. A raw_json2pptx slide keeps the pattern's wording: its author
// wrote the pattern.

// patternLeadRE matches the "slide N: <pattern>: <pattern> " lead of a
// pattern's own finding.
var patternLeadRE = regexp.MustCompile(`^(slide \d+: )?([a-z0-9]+(?:-[a-z0-9]+)*): ([a-z0-9]+(?:-[a-z0-9]+)*) `)

// valueMentionRE matches a pattern value named in a message: "steps[3].body",
// "values[1].label", "options[2].detail".
var valueMentionRE = regexp.MustCompile(`\b[a-z_]+\[\d+\](?:\.[a-z_]+(?:\[\d+\])?)*`)

// kindAdvice replaces advice that names a control the slide kinds do not have.
// Each entry is tried on every reworded finding.
var kindAdvice = []struct {
	re   *regexp.Regexp
	with string
}{
	// option_matrix has no legend switch, and dropping highlight_label made the
	// table taller (the label then moved into the option's cell).
	{regexp.MustCompile(`drop the option details or highlight_label, hide the legend \(show_legend: false\), drop the slide takeaway, or split the table`),
		"drop the option details, drop the slide takeaway, or use fewer options or criteria"},
	// A timeline draws bars when a milestone has an end_date; the kind has no
	// style field, and a bar's label holds less than a dot's.
	{regexp.MustCompile(`is not rendered in gantt style — move the detail into the label, choose dots or chevron style, or remove the body`),
		"is not drawn while a milestone has an end_date (the timeline is then bars, which carry a label only) — remove the body, or remove every end_date so the milestones are dots that show it"},
	{regexp.MustCompile(`a gantt bar holds`), "a bar (a milestone with an end_date) holds"},
	// A process without descriptions is a row of boxes; with them it is a
	// numbered strip.
	{regexp.MustCompile(`switch to numbered-step-strip / process-grid-\d+row, or cap max_height_pct to ~\d+`),
		"give each step a description (the slide is then a numbered strip) or add steps"},
	// A row of bare steps or milestones alone on a slide (go-slide-creator-d6wvb,
	// -d0g2j). The pattern's remedies are other patterns and a compose
	// envelope; a process reaches the numbered strip through its steps'
	// descriptions once nothing pins it to the flow, and a timeline carries a
	// body under each milestone or sits in a region of a regions slide.
	{regexp.MustCompile(`process-flow is the slide's only content: one row of (\d+) short cells \(avg (\d+) chars\), sized to its text, so most of the slide stays empty — give each step a detail line .*$`),
		"the steps are the slide's only content: one row of ${1} short boxes (avg ${2} chars), sized to their text, so most of the slide stays empty — remove the slide's pattern field and give each step a description (the slide is then a numbered strip with a line of detail under each step)"},
	{regexp.MustCompile(`timeline-horizontal is the slide's only content: one row of (\d+) short cells \(avg (\d+) chars\), sized to its text, so most of the slide stays empty — give each step a detail line .*$`),
		"the milestones are the slide's only content: one row of ${1} short entries (avg ${2} chars), sized to their text, so most of the slide stays empty — give each milestone a body (a line of detail under its label), or make the timeline one region of a regions slide beside what it dates"},
	{regexp.MustCompile(`, or give the house a taller region`), ", or drop the slide's takeaway"},
	{regexp.MustCompile(`this (\d+pt-high )?region`), "the ${1}content area"},
	{regexp.MustCompile(`stacked-box rows`), "numbered rows"},
	// An image_case callout is an overlay on a grid cell only in the compiled
	// deck; the author wrote a point on the picture.
	{regexp.MustCompile(`^overlay \d+: image target \(([0-9.]+), ([0-9.]+)\) on shape_grid image cell \[\d+,\d+\] is cropped away — the frame shows source x ([0-9.]+)–([0-9.]+), y ([0-9.]+)–([0-9.]+); `),
		"this callout points at x ${1}, y ${2} of the picture, which the picture's frame crops away (it shows x ${3}–${4}, y ${5}–${6}): set image.fit to \"contain\" to keep the whole picture, or move the point inside that range; "},
	{regexp.MustCompile(`^row content ~(\d+)pt exceeds max_height (\d+)pt`), "a row's content needs about ${1}pt of height and has ${2}pt"},
	// The narrow-wrap finding kept its advice in params.hint, beside the names
	// of raw patterns; the remedy of a DeckSpec finding is its sentence.
	{regexp.MustCompile(`a column of fragments, not a label$`),
		"a column of fragments, not a label: cut each box to a label, or use fewer boxes so each is wider"},
}

// proseBudgetRE reads the character budget a pattern states in its sentence:
// "a bar holds about 36 readable label characters". A height ("holds about
// 311pt") is not one.
var proseBudgetRE = regexp.MustCompile(`\bholds? about (\d+) `)

// setFieldBudget gives a finding that names a field and its budget in prose
// the same budget as a fact, so the rewrite it suggests says how long.
func setFieldBudget(d *semanticDiagnostic, maxChars int) {
	if maxChars <= 0 {
		return
	}
	params := map[string]any{"max_chars": maxChars}
	if d.RecommendedEdit == nil {
		d.RecommendedEdit = &semantic.SemanticEdit{Kind: semantic.EditShortenText, Hint: "Shorten this field to the budget in params.", Params: params}
	}
	if d.diag != nil && d.diag.Fix == nil {
		source := *d.diag
		source.Fix = &diagnostics.Fix{Kind: "reduce_text", Params: params}
		d.diag = &source
	}
}

// readableRefusalRE matches the readability finding's sentence, in both the
// predicted and the generated form.
var readableRefusalRE = regexp.MustCompile(`^(?:[a-z-]+ )?text renders at ([0-9.]+)pt, below the ([0-9.]+)pt minimum`)

// layoutLabels are texts a kind's layout writes itself, with what in the spec
// controls each.
var layoutLabels = map[string]string{
	"recommended":         "it marks the option with recommended: true",
	"decisions requested": "decisions_label sets it",
	"in parallel":         "it heads the roadmap's parallel tracks",
}

// deckSpecWording rewrites the fit findings of a compiled DeckSpec in the
// terms of the spec. It is idempotent.
func deckSpecWording(diags []semanticDiagnostic, input *PresentationInput, ir *semantic.DeckIR) {
	if input == nil || ir == nil {
		return
	}
	w := specWording{input: input, ir: ir, vocab: map[int][][2]string{}}
	for i := range diags {
		d := &diags[i]
		if d.Action == "" {
			continue // a spec-level finding is already in the spec's words
		}
		rawIdx := slidepath.SlideIndex(d.RawPath)
		if rawIdx < 0 && d.SlideIndex != nil {
			rawIdx = *d.SlideIndex
		}
		if rawIdx < 0 || rawIdx >= len(ir.Slides) || rawIdx >= len(input.Slides) || ir.Slides[rawIdx].Kind == semantic.KindRawJSON2pptx {
			continue
		}
		msg := d.Message
		if d.Code == patterns.ErrCodeTextBelowReadableMin {
			msg = readabilitySentence(*d, ir, rawIdx)
		}
		msg = w.patternSentence(d, msg, rawIdx)
		for _, a := range kindAdvice {
			msg = a.re.ReplaceAllString(msg, a.with)
		}
		if msg == d.Message {
			continue
		}
		if d.Code == patterns.ErrCodeSparseSingleRowFlow {
			sparseFlowKindRemedy(d)
		}
		if d.baseMessage == d.Message {
			// The entry already lists its items (a narrow-wrap finding's
			// boxes): the sentence its count is added to is reworded with it.
			d.baseMessage = msg
		}
		d.Message = msg
		if d.diag != nil {
			source := *d.diag
			source.Message = msg
			d.diag = &source
		}
	}
}

// sparseFlowKindRemedy replaces the pattern swap a sparse single-row flow
// suggests — its facts name the pattern the slide compiled to and the patterns
// to swap it for, none of which a slide kind has — with the advice the
// reworded sentence gives.
func sparseFlowKindRemedy(d *semanticDiagnostic) {
	d.RecommendedEdit = &semantic.SemanticEdit{
		Kind: semantic.EditAddDetailOrMerge,
		Hint: "Give each entry of the row a line of detail, choose a denser slide kind, or merge this slide with a related one.",
	}
	if d.diag != nil && d.diag.Fix != nil {
		source := *d.diag
		source.Fix = nil
		d.diag = &source
	}
}

// specWording is one deck's rewording state: the item vocabulary of each
// slide, worked out once.
type specWording struct {
	input *PresentationInput
	ir    *semantic.DeckIR
	vocab map[int][][2]string
}

// patternSentence rewrites a finding written by the slide's pattern: the
// pattern's name comes off, the values it names become pointers into the spec
// and its item words the kind's. A finding that names one field is moved to
// that field, with the budget its sentence states.
func (w *specWording) patternSentence(d *semanticDiagnostic, msg string, rawIdx int) string {
	lead := patternLeadRE.FindStringSubmatch(msg)
	if lead == nil || lead[2] != lead[3] {
		return msg
	}
	values := patternValues(&w.input.Slides[rawIdx])
	renames, seen := w.vocab[rawIdx]
	if !seen {
		renames = kindVocabulary(values, w.ir, rawIdx)
		w.vocab[rawIdx] = renames
	}
	field := ""
	rest := valueMentionRE.ReplaceAllStringFunc(msg[len(lead[0]):], func(mention string) string {
		at := w.fieldOf(values, mention, rawIdx)
		if at == "" {
			return mention
		}
		if field == "" {
			field = at
		}
		return specPointer(at)
	})
	rest = renameWords(rest, renames)
	if strings.HasPrefix(rest, "needs ") || strings.HasPrefix(rest, "fills ") {
		rest = "the layout " + rest
	}
	// The finding is about the field it names, not the whole slide.
	if field != "" && d.SemanticPath == slideContainer(d.SemanticPath) {
		d.SemanticPath = field
		if d.diag != nil {
			source := *d.diag
			source.Path = field
			d.diag = &source
		}
		if m := proseBudgetRE.FindStringSubmatch(rest); m != nil {
			n, _ := strconv.Atoi(m[1])
			setFieldBudget(d, n)
		}
	}
	return lead[1] + rest
}

// fieldOf locates the spec field behind a pattern value a message names, or
// returns "".
func (w *specWording) fieldOf(values any, mention string, rawIdx int) string {
	text, ok := patternValueText(values, mention)
	if !ok {
		return ""
	}
	if at := itemField(w.ir.Slides[rawIdx], mention, text); at != "" {
		return at
	}
	return semanticFieldForText(w.ir, rawIdx, text)
}

// readabilitySentence states a readability finding without the renderer's
// vocabulary (text roles, autofit percentages, bullet splitting), and says so
// when the text that shrank is the layout's own.
func readabilitySentence(d semanticDiagnostic, ir *semantic.DeckIR, rawIdx int) string {
	m := readableRefusalRE.FindStringSubmatch(d.Message)
	if m == nil || strings.Contains(d.Message, "the slide holds more than fits") {
		return d.Message
	}
	msg := fmt.Sprintf("text renders at %spt, below the %spt minimum: the slide holds more than fits at a readable size", m[1], m[2])
	text := refusedText(d)
	if text != "" && !specCarriesText(ir, rawIdx, text) {
		control := layoutLabels[strings.ToLower(strings.TrimSpace(text))]
		if control == "" {
			control = "no field of the slide holds it"
		}
		// A label the layout writes cannot be shortened by its author.
		return msg + fmt.Sprintf(" — the text is %s, a label the layout writes, not copy from the spec (%s); it shrinks with the rest of the slide, so cut what the slide holds", strconv.Quote(text), control)
	}
	return msg + " — cut text or items on it, or split it across two slides"
}

// refusedText is the paragraph a readability finding measured.
func refusedText(d semanticDiagnostic) string {
	if text, _ := d.Evidence["text"].(string); text != "" {
		return text
	}
	if d.diag != nil && d.diag.Fix != nil {
		for _, key := range []string{"paragraph_text", "rendered_shape_text"} {
			if text, _ := d.diag.Fix.Params[key].(string); text != "" {
				return text
			}
		}
	}
	return ""
}

// specCarriesText reports whether the slide's authored fields carry text.
func specCarriesText(ir *semantic.DeckIR, rawIdx int, text string) bool {
	slide := ir.Slides[rawIdx]
	want := normalizeMatchText(text)
	for _, own := range []string{slide.Title, slide.Takeaway} {
		if own != "" && strings.Contains(normalizeMatchText(own), want) {
			return true
		}
	}
	leaves := map[string]string{}
	collectStringLeaves(slide.Body, "", leaves)
	for _, v := range leaves {
		if n := normalizeMatchText(v); n != "" && (strings.Contains(n, want) || strings.Contains(want, n) && len(n) >= minTextMatchRunes) {
			return true
		}
	}
	return false
}

// patternValues decodes the values of the pattern a slide compiled to.
func patternValues(slide *SlideInput) any {
	if slide.Pattern == nil || len(slide.Pattern.Values) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(slide.Pattern.Values, &v) != nil {
		return nil
	}
	return v
}

// patternValueText returns the string a message's value mention names. A
// pattern whose values are a list is addressed as "values[i]".
func patternValueText(values any, mention string) (string, bool) {
	tokens := dottedTokens(mention)
	if _, isList := values.([]any); isList && len(tokens) > 0 && tokens[0] == "values" {
		tokens = tokens[1:]
	}
	text, ok := nodeAtTokens(values, tokens).(string)
	return text, ok && text != ""
}

// mentionIndexRE reads the item index of a value mention ("steps[3].body").
var mentionIndexRE = regexp.MustCompile(`^[a-z_]+\[(\d+)\]`)

// itemField locates the spec field behind a pattern value named with an item
// index: the entry at that index of one of the slide's lists that carries the
// same text. Entries often repeat a text ("TBD" twice); the index settles
// which one is meant.
func itemField(slide semantic.SlideIR, mention, text string) string {
	m := mentionIndexRE.FindStringSubmatch(mention)
	if m == nil || slide.SourcePath == "" {
		return ""
	}
	idx, _ := strconv.Atoi(m[1])
	list, key, ok := bodyItemKey(slide.Body, idx, text)
	if !ok {
		return ""
	}
	at := fmt.Sprintf("%s.%s[%d]", slide.SourcePath, list, idx)
	if key != "" {
		at += "." + key
	}
	return at
}

// bodyItemKey finds the list and entry key of a slide's payload whose entry
// idx carries text.
func bodyItemKey(body map[string]any, idx int, text string) (list, key string, ok bool) {
	want := normalizeMatchText(text)
	for _, name := range sortedMapKeys(body) {
		items, isList := body[name].([]any)
		if !isList || idx >= len(items) {
			continue
		}
		switch item := items[idx].(type) {
		case string:
			if normalizeMatchText(item) == want {
				return name, "", true
			}
		case map[string]any:
			for _, k := range sortedMapKeys(item) {
				if s, isText := item[k].(string); isText && normalizeMatchText(s) == want {
					return name, k, true
				}
			}
		}
	}
	return "", "", false
}

// kindVocabulary pairs the pattern's names for a slide's items with the
// kind's: the list and entry keys under which the same text sits in the
// pattern's values and in the spec ("steps" / "options", "body" / "detail").
func kindVocabulary(values any, ir *semantic.DeckIR, rawIdx int) [][2]string {
	var out [][2]string
	add := func(from, to string) {
		if from == to || from == "" || to == "" || from == "values" {
			return
		}
		for _, have := range out {
			if have[0] == from {
				return
			}
		}
		out = append(out, [2]string{from, to})
	}
	body := ir.Slides[rawIdx].Body
	entries := func(list string, items []any) {
		for i, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, key := range sortedMapKeys(m) {
				text, isText := m[key].(string)
				if !isText || text == "" {
					continue
				}
				if to, toKey, found := bodyItemKey(body, i, text); found {
					add(list, to)
					add(key, toKey)
				}
			}
		}
	}
	switch t := values.(type) {
	case []any:
		entries("values", t)
	case map[string]any:
		for _, list := range sortedMapKeys(t) {
			if items, ok := t[list].([]any); ok {
				entries(list, items)
			}
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// renameWords replaces the pattern's words for a slide's items with the
// kind's, in singular and plural.
func renameWords(text string, renames [][2]string) string {
	for _, r := range renames {
		for _, form := range wordForms(r[0], r[1]) {
			text = regexp.MustCompile(`\b`+regexp.QuoteMeta(form[0])+`\b`).ReplaceAllString(text, form[1])
		}
	}
	return text
}

// wordForms lists the plural and singular pairs of a rename, longest first.
func wordForms(from, to string) [][2]string {
	singular := func(s string) string {
		switch {
		case strings.HasSuffix(s, "ies"):
			return s[:len(s)-3] + "y"
		case strings.HasSuffix(s, "s"):
			return s[:len(s)-1]
		}
		return s
	}
	plural := func(s string) string {
		switch {
		case strings.HasSuffix(s, "s"):
			return s
		case strings.HasSuffix(s, "y"):
			return s[:len(s)-1] + "ies"
		}
		return s + "s"
	}
	return [][2]string{{plural(from), plural(to)}, {singular(from), singular(to)}}
}
