package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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

// valueMentionsRE matches a value mention with the sibling fields a pattern
// names in the same breath: "options[0].name/detail", "values[2].date/end_date".
var valueMentionsRE = regexp.MustCompile(valueMentionRE.String() + `(?:/[a-z_]+)*`)

// siblingMentions splits "options[0].name/detail" into the values it names:
// options[0].name and options[0].detail.
func siblingMentions(mention string) []string {
	parts := strings.Split(mention, "/")
	out := []string{parts[0]}
	dot := strings.LastIndexByte(parts[0], '.')
	for _, sibling := range parts[1:] {
		if dot < 0 {
			return []string{mention}
		}
		out = append(out, parts[0][:dot+1]+sibling)
	}
	return out
}

// kindAdvice replaces advice that names a control the slide kinds do not have.
// Each entry is tried on every reworded finding.
var kindAdvice = []struct {
	re   *regexp.Regexp
	with string
}{
	// option_matrix has no legend switch, and dropping highlight_label made the
	// table taller (the label then moved into the option's cell). The remedies
	// run from the one that keeps most: the kind requires its takeaway, so
	// dropping it is not one (go-slide-creator-u8orh).
	{regexp.MustCompile(`drop the option details or highlight_label, hide the legend \(show_legend: false\), drop the slide takeaway, or split the table`),
		"shorten the option details, drop them, or use fewer options or criteria"},
	{regexp.MustCompile(`shorten the option copy, hide the legend, or split the table`),
		"shorten the option's name or detail, drop the detail, or use fewer options or criteria"},
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
	// The risk-heatmap pattern can hide its band legend (show_legend); a
	// risk_heatmap slide has no such switch (go-slide-creator-nuq30).
	{regexp.MustCompile(`, or free height \(drop the takeaway, hide the legend\)`), ", or drop the slide's takeaway"},
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

// dropTakeawayAdviceRE matches the last of a sentence's remedies when it is to
// drop the slide's takeaway, with the remedy before it. On a slide whose kind
// requires a takeaway that advice is answered by SEMANTIC_TAKEAWAY_REQUIRED on
// the next validation (go-slide-creator-u8orh).
var dropTakeawayAdviceRE = regexp.MustCompile(`, ([^,—]+), or drop the slide(?:'s)? takeaway`)

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
	w := specWording{input: input, ir: ir, vocab: map[int][][2]string{}, pairs: map[int]*slidePairing{}}
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
		if spec := ir.Slides[rawIdx]; semantic.TakeawayRequired(semantic.SlideSpec{Kind: spec.Kind, Body: spec.Body}) {
			msg = dropTakeawayAdviceRE.ReplaceAllString(msg, ", or ${1}")
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

// specWording is one deck's rewording state: how each slide's pattern lists
// pair with the lists of its spec, worked out once.
type specWording struct {
	input *PresentationInput
	ir    *semantic.DeckIR
	vocab map[int][][2]string
	pairs map[int]*slidePairing
}

// mentionSlot stands in for a located value while the pattern's words around
// it are renamed: a pointer into the spec is in the spec's words already, and
// "items" inside "/slides/1/items/0/name" is not the pattern's word for a list.
const mentionSlot = "\x00"

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
		renames = w.kindVocabulary(values, rawIdx)
		w.vocab[rawIdx] = renames
	}
	field := ""
	var located []string
	rest := valueMentionsRE.ReplaceAllStringFunc(msg[len(lead[0]):], func(mentions string) string {
		// "options[0].name/detail" names two fields; a pointer with "/detail"
		// left on its end is one field that does not exist.
		var slots, found []string
		for _, mention := range siblingMentions(mentions) {
			at := w.fieldOf(values, mention, rawIdx)
			if at == "" {
				slots = append(slots, mention)
				continue
			}
			found = append(found, at)
			slots = append(slots, mentionSlot)
		}
		if len(found) == 0 {
			return mentions
		}
		if field == "" {
			field = found[0]
		}
		for _, at := range found {
			located = append(located, specPointer(at))
		}
		return strings.Join(slots, " and ")
	})
	rest = renameWords(rest, renames)
	for _, pointer := range located {
		rest = strings.Replace(rest, mentionSlot, pointer, 1)
	}
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
// returns "". The value's list is paired with a list of the spec first (see
// slidePairing), and the entry at the value's index searched for its text;
// only a value no pairing places is looked for by its text across the slide.
func (w *specWording) fieldOf(values any, mention string, rawIdx int) string {
	text, ok := patternValueText(values, mention)
	if !ok {
		return ""
	}
	slide := w.ir.Slides[rawIdx]
	if slide.SourcePath == "" {
		return ""
	}
	if m := mentionItemRE.FindStringSubmatch(mention); m != nil {
		idx, _ := strconv.Atoi(m[2])
		if entry := w.pairing(values, rawIdx).entry(m[1], idx); entry != "" {
			if at := fieldInEntry(slide.Body, entry, m[3], text); at != "" {
				return slide.SourcePath + "." + at
			}
		}
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

// mentionItemRE splits a value mention into its list, the item's index and
// what follows: "steps[3].body" is steps, 3, ".body".
var mentionItemRE = regexp.MustCompile(`^([a-z_]+)\[(\d+)\](.*)$`)

// slidePairing is how the lists of the pattern a slide compiled to pair with
// the lists of its spec (go-slide-creator-nuq30).
//
// A pattern list and a spec list are paired by the compiler's own source link
// where it wrote one ("slides[1].pattern.values.items" came from
// "slides[1].items"), and otherwise by the spec list that carries the most of
// the pattern list's texts at the same indices — the list of the same name
// first. One coincidence does not pair two lists: a risk whose impact is
// "Major" at index 3 beside impact_levels whose fourth level is "Major" made
// the pattern's items the kind's impact_levels, when the items list of the
// spec carried all fifteen of the pattern's texts.
type slidePairing struct {
	// lists maps a pattern list to the spec list it came from, as a path
	// relative to the slide ("options").
	lists map[string]string
	// items maps a pattern item ("values[2]") to the spec entry the compiler
	// linked it to, where the links are per item.
	items map[string]string
	// keys are the entry keys the pattern and the spec name differently.
	keys [][2]string
}

// entry returns the spec entry behind item idx of a pattern list, relative to
// the slide, or "".
func (p *slidePairing) entry(list string, idx int) string {
	if at := p.items[fmt.Sprintf("%s[%d]", list, idx)]; at != "" {
		return at
	}
	if at := p.lists[list]; at != "" {
		return fmt.Sprintf("%s[%d]", at, idx)
	}
	return ""
}

// patternLists returns the lists of a pattern's values by the name a message
// gives them. A pattern whose values are a list has one, "values".
func patternLists(values any) map[string][]any {
	out := map[string][]any{}
	switch t := values.(type) {
	case []any:
		out["values"] = t
	case map[string]any:
		for name, v := range t {
			if items, ok := v.([]any); ok {
				out[name] = items
			}
		}
	}
	return out
}

// pairing works out, once per slide, how the pattern's lists pair with the
// spec's.
func (w *specWording) pairing(values any, rawIdx int) *slidePairing {
	if p := w.pairs[rawIdx]; p != nil {
		return p
	}
	p := &slidePairing{lists: map[string]string{}, items: map[string]string{}}
	if w.pairs == nil {
		w.pairs = map[int]*slidePairing{}
	}
	w.pairs[rawIdx] = p
	slide := w.ir.Slides[rawIdx]
	lists := patternLists(values)
	_, valuesAreList := values.([]any)
	valuesRaw := fmt.Sprintf("slides[%d].pattern.values", rawIdx)
	keyVotes := map[string]map[string]int{}
	for _, name := range sortedKeysOf(lists) {
		items := lists[name]
		listRaw := valuesRaw + "." + name
		if valuesAreList {
			listRaw = valuesRaw
		}
		linked := w.linkedList(slide, listRaw, valuesRaw)
		if linked != "" && carried(items, specList(slide.Body, linked)) == 0 {
			linked = ""
		}
		if linked == "" {
			linked = textPairedList(name, items, slide.Body)
		}
		if linked != "" {
			p.lists[name] = linked
		}
		for i := range items {
			if at := w.linkedPath(slide, fmt.Sprintf("%s[%d]", listRaw, i)); at != "" {
				p.items[fmt.Sprintf("%s[%d]", name, i)] = at
			}
			if entry := p.entry(name, i); entry != "" {
				voteKeys(keyVotes, items[i], nodeAtDotted(slide.Body, entry))
			}
		}
	}
	for _, from := range sortedKeysOf(keyVotes) {
		if to := topVote(keyVotes[from], from); to != from {
			p.keys = append(p.keys, [2]string{from, to})
		}
	}
	return p
}

// linkedPath returns the spec path the compiler linked exactly this raw path
// to, relative to the slide, or "".
func (w *specWording) linkedPath(slide semantic.SlideIR, raw string) string {
	e, ok := w.ir.SourceMap.Lookup(raw)
	if !ok || e.RawPath != raw || slide.SourcePath == "" {
		return ""
	}
	rel := strings.TrimPrefix(e.SemanticPath, slide.SourcePath+".")
	if rel == e.SemanticPath {
		return ""
	}
	return rel
}

// linkedList returns the spec list the compiler linked a pattern list to,
// relative to the slide, or "". A compiler that links the whole of the
// pattern's values to one list of the spec (a decision's options are the
// strip's values) links each list inside them to it.
func (w *specWording) linkedList(slide semantic.SlideIR, listRaw, valuesRaw string) string {
	for _, raw := range []string{listRaw, valuesRaw} {
		if rel := w.linkedPath(slide, raw); rel != "" && specList(slide.Body, rel) != nil {
			return rel
		}
	}
	return ""
}

// specList returns the list at a path of a slide's payload, or nil.
func specList(body map[string]any, rel string) []any {
	list, _ := nodeAtDotted(body, rel).([]any)
	return list
}

// entryTexts returns the texts an entry carries directly: itself when it is a
// string (under ""), else its string fields by key.
func entryTexts(entry any) map[string]string {
	out := map[string]string{}
	switch t := entry.(type) {
	case string:
		if n := normalizeMatchText(t); n != "" {
			out[""] = n
		}
	case map[string]any:
		for k, v := range t {
			if s, ok := v.(string); ok {
				if n := normalizeMatchText(s); n != "" {
					out[k] = n
				}
			}
		}
	}
	return out
}

// carried counts the texts of a pattern list that a spec list carries at the
// same index.
func carried(items, spec []any) int {
	n := 0
	for i := 0; i < len(items) && i < len(spec); i++ {
		have := map[string]bool{}
		for _, text := range entryTexts(spec[i]) {
			have[text] = true
		}
		for _, text := range entryTexts(items[i]) {
			if have[text] {
				n++
			}
		}
	}
	return n
}

// textPairedList returns the top-level list of a slide's payload that carries
// the most of a pattern list's texts at the same indices, or "". Lists that
// carry as many are told apart by name: the pattern list's own name first.
func textPairedList(name string, items []any, body map[string]any) string {
	best, most := "", 0
	for _, candidate := range sortedKeysOf(body) {
		spec, isList := body[candidate].([]any)
		if !isList {
			continue
		}
		n := carried(items, spec)
		if n > most || n == most && n > 0 && candidate == name {
			best, most = candidate, n
		}
	}
	return best
}

// voteKeys records, for each text field of a pattern item, the key under which
// the paired spec entry carries the same text.
func voteKeys(votes map[string]map[string]int, item, entry any) {
	theirs, isObject := item.(map[string]any)
	ours := entryTexts(entry)
	if !isObject || len(ours) == 0 {
		return
	}
	for key, text := range entryTexts(theirs) {
		if ours[key] == text {
			vote(votes, key, key)
			continue
		}
		for _, to := range sortedKeysOf(ours) {
			if to != "" && ours[to] == text {
				vote(votes, key, to)
			}
		}
	}
}

func vote(votes map[string]map[string]int, from, to string) {
	if votes[from] == nil {
		votes[from] = map[string]int{}
	}
	votes[from][to]++
}

// topVote returns the name with the most votes; own wins a tie, then the
// first by name.
func topVote(votes map[string]int, own string) string {
	best, most := own, votes[own]
	for _, name := range sortedKeysOf(votes) {
		if votes[name] > most {
			best, most = name, votes[name]
		}
	}
	return best
}

// fieldInEntry locates text inside a spec entry, relative to the slide: at the
// place the pattern names when the spec has it there (tail is what follows the
// item in the mention, ".impact"), else the one field of the entry that
// carries the text, else the entry when its fields make the text up (a label
// and a description set as one paragraph). "" when the entry does not carry
// the text.
func fieldInEntry(body map[string]any, entry, tail, text string) string {
	node := nodeAtDotted(body, entry)
	if node == nil {
		return ""
	}
	want := normalizeMatchText(text)
	if s, ok := nodeAtDotted(body, entry+tail).(string); ok && tail != "" && normalizeMatchText(s) == want {
		return entry + tail
	}
	leaves := map[string]string{}
	collectStringLeaves(node, entry, leaves)
	var exact []string
	part := false
	for at, value := range leaves {
		v := normalizeMatchText(value)
		switch {
		case v == "":
		case v == want:
			exact = append(exact, at)
		case utf8.RuneCountInString(v) >= minTextMatchRunes && strings.Contains(want, v):
			part = true
		}
	}
	switch {
	case len(exact) == 1:
		return exact[0]
	case len(exact) > 1:
		sort.Strings(exact)
		return commonSemanticAncestor(exact)
	case part:
		return entry
	}
	return ""
}

// kindVocabulary pairs the pattern's names for a slide's items with the
// kind's: the lists the pattern and the spec name differently ("steps" /
// "options") and the entry keys ("body" / "detail").
func (w *specWording) kindVocabulary(values any, rawIdx int) [][2]string {
	p := w.pairing(values, rawIdx)
	var out [][2]string
	for _, from := range sortedKeysOf(p.lists) {
		// A list of the spec has a name of its own only at the top of the
		// slide, and "values" is the pattern's word for nothing an author
		// wrote.
		to := p.lists[from]
		if from == "values" || from == to || strings.ContainsAny(to, ".[") {
			continue
		}
		out = append(out, [2]string{from, to})
	}
	for _, key := range p.keys {
		if key[1] != "" && !slices.ContainsFunc(out, func(have [2]string) bool { return have[0] == key[0] }) {
			out = append(out, key)
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string { return sortedKeysOf(m) }

func sortedKeysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// renameWords replaces the pattern's words for a slide's items with the
// kind's, in singular and plural. Every word is replaced once, by its own
// rename: "label" → "name" beside "name" → "title" does not turn a label
// into a title.
func renameWords(text string, renames [][2]string) string {
	to := map[string]string{}
	var forms []string
	for _, r := range renames {
		for _, form := range wordForms(r[0], r[1]) {
			if _, have := to[form[0]]; !have {
				to[form[0]] = form[1]
				forms = append(forms, regexp.QuoteMeta(form[0]))
			}
		}
	}
	if len(forms) == 0 {
		return text
	}
	sort.Slice(forms, func(i, j int) bool {
		if len(forms[i]) != len(forms[j]) {
			return len(forms[i]) > len(forms[j])
		}
		return forms[i] < forms[j]
	})
	return regexp.MustCompile(`\b(?:`+strings.Join(forms, "|")+`)\b`).ReplaceAllStringFunc(text, func(word string) string {
		return to[word]
	})
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
