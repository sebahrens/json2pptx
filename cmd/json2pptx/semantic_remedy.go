package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// One remedy per finding, and patches that were tried
// (go-slide-creator-micna, -vihnl, -4mmvb, -c2j5b).
//
// A DeckSpec finding used to give its advice four times over: in the message,
// in remediation.primary.params.hint (written for the raw deck: "cap the grid
// height (bounds / max_height_pct)"), in a copy of the patch under
// remediation.primary.params, and in next_tool_call. The copies disagreed —
// "shorten the text" beside a patch that switched the slide's layout — and the
// patches were never run: the one for an unknown key rewrote the unknown key,
// and the layout switch traded one refusal for three new findings.
//
// A finding now says each thing once:
//
//   - message: what is wrong and what to do, in the fields of the spec;
//   - remediation.primary: the action and the budgets it must meet (max_chars,
//     max_items, did_you_mean) — facts only, no prose and no patch;
//   - next_tool_call: the patch, when one exists. A patch that is complete as
//     written is applied to the spec and validated before it is offered: it is
//     marked patch_verified, or it is not offered. A patch with a value the
//     author has to write carries an <instruction> in its place and no flag.
//
// For a slide that is over-full, the server looks for the smallest cut that
// clears it — one optional line, the takeaway, the last list item — by
// validating each, and offers that as the verified patch.

// remedyFactKeys are the fix params a DeckSpec author acts on, by the name
// they are reported under. Everything else a raw fix carries either addresses
// the compiled deck or repeats the finding's evidence and message.
var remedyFactKeys = map[string]string{
	"max_chars":      "max_chars",
	"max_length":     "max_chars",
	"max_words":      "max_words",
	"max_items":      "max_items",
	"min_items":      "min_items",
	"max_rows":       "max_rows",
	"row":            "row",
	"split_at_row":   "row",
	"did_you_mean":   "did_you_mean",
	"hosted_type":    "hosted_type",
	"hosted_as":      "hosted_as",
	"expected_shape": "expected_shape",
	"example":        "example",
	"available":      "available",
	"allowed":        "allowed",
	"original":       "original",
	"samples":        "samples",
}

// remedyFacts keeps the facts of a fix's params.
func remedyFacts(raw map[string]any) map[string]any {
	var out map[string]any
	for k, v := range raw {
		name, ok := remedyFactKeys[k]
		if !ok || v == nil {
			continue
		}
		if out == nil {
			out = map[string]any{}
		}
		if _, has := out[name]; !has || k == name {
			out[name] = v
		}
	}
	return out
}

// memberFacts are a collapsed finding's facts: a budget every item shares is
// one value, and a budget that differs per item is a list in the order of
// paths. They used to be the first item's alone.
func memberFacts(own map[string]any, members []diagMember) map[string]any {
	facts := remedyFacts(own)
	if len(members) < 2 {
		return facts
	}
	perMember := make([]map[string]any, len(members))
	keys := map[string]bool{}
	for i, m := range members {
		perMember[i] = remedyFacts(m.fixParams)
		for k := range perMember[i] {
			keys[k] = true
		}
	}
	for k := range keys {
		values := make([]any, 0, len(members))
		differ := false
		for _, f := range perMember {
			v, ok := f[k]
			if !ok {
				values = nil
				break
			}
			if len(values) > 0 && !reflect.DeepEqual(values[0], v) {
				differ = true
			}
			values = append(values, v)
		}
		if values == nil {
			continue
		}
		if facts == nil {
			facts = map[string]any{}
		}
		if differ {
			facts[k] = values
		} else {
			facts[k] = values[0]
		}
	}
	return facts
}

// bareCode strips a finding code's namespace prefix.
func bareCode(code string) string {
	for _, ns := range diagnostics.AllNamespaces() {
		if strings.HasPrefix(code, ns+".") {
			return code[len(ns)+1:]
		}
	}
	return code
}

// trialFinding is what a trial compares of a finding: its code, where it sits
// and whether it blocks.
type trialFinding struct {
	Code     string
	Path     string // dotted DeckSpec path
	Paths    []string
	Blocking bool
	Warning  bool
}

func (f trialFinding) at(path string) bool {
	if f.Path == path {
		return true
	}
	for _, p := range f.Paths {
		if p == path {
			return true
		}
	}
	return false
}

// trialFindings reduces a finding set to what a trial compares.
func trialFindings(diags []semanticDiagnostic) []trialFinding {
	out := make([]trialFinding, 0, len(diags))
	for _, d := range diags {
		f := trialFinding{Code: d.Code, Path: d.SemanticPath, Blocking: diagnosticBlocks(d), Warning: d.Severity == string(diagnostics.SeverityWarning)}
		if f.Path == "" && d.SlideIndex != nil {
			f.Path = "slides[" + strconv.Itoa(*d.SlideIndex) + "]"
		}
		for _, m := range d.members {
			f.Paths = append(f.Paths, m.Path)
		}
		out = append(out, f)
	}
	return out
}

// trialFindingsOfSpecCheck is trialFindings for a spec that was not evaluated.
func trialFindingsOfSpecCheck(ds []diagnostics.Diagnostic) []trialFinding {
	out := make([]trialFinding, 0, len(ds))
	for _, d := range ds {
		out = append(out, trialFinding{Code: d.Code, Path: d.Path, Blocking: d.Severity == diagnostics.SeverityError || d.Severity == "", Warning: d.Severity == diagnostics.SeverityWarning})
	}
	return out
}

// trialCleared reports whether a patched spec's findings show the patch did
// its job: none of the subjects is left, nothing blocks that did not block
// before, and no slide lost its visual in exchange.
//
// gateUnknown says the spec as sent was refused before the quality gate ran:
// the gate's own finding on the patched spec is then not something the patch
// caused.
func trialCleared(before, after []trialFinding, subjects []trialFinding, gateUnknown bool) bool {
	for _, s := range subjects {
		for _, a := range after {
			if a.Code != s.Code {
				continue
			}
			if a.at(s.Path) {
				return false
			}
			for _, p := range s.Paths {
				if a.at(p) {
					return false
				}
			}
		}
	}
	key := func(f trialFinding) string { return f.Code + "\x00" + slideContainer(f.Path) }
	blocked, present := map[string]bool{}, map[string]bool{}
	for _, b := range before {
		present[key(b)] = true
		if b.Blocking {
			blocked[key(b)] = true
		}
	}
	for _, a := range after {
		if gateUnknown && a.Code == codeQualityGate {
			continue
		}
		if a.Blocking && !blocked[key(a)] {
			return false
		}
		if fallbackRootCodes[a.Code] && !present[key(a)] {
			return false
		}
	}
	return true
}

// remedyContext is what one response needs to work out its findings' remedies.
type remedyContext struct {
	data     []byte
	filename string
	// deckID is the handle a suggested patch applies to; without one no patch
	// is offered.
	deckID string
	// run validates a spec as the calling tool does and returns its findings;
	// nil when the call cannot try a patch (a CLI run, a template file the
	// trial would not resolve). A complete patch is then offered untried.
	run func(spec []byte) ([]trialFinding, bool)
	// before are the findings of the spec as sent; gateUnknown says its run
	// was refused before the quality gate was evaluated.
	before      []trialFinding
	gateUnknown bool
	// cli says the findings are printed by the command line, which names no
	// tool to call next (each finding carries describe_command there).
	cli bool
	// trials is how many more specs the response may validate.
	trials int
	doc    any
	// tried holds the findings of each patched spec already validated (nil
	// for a patch that did not apply), and cuts the removal found per slide.
	tried map[string]*[]trialFinding
	cuts  map[string]*cutCandidate
}

// maxPatchTrials bounds the validations one response spends on trying patches,
// and maxCutSearches the over-full slides it looks for a smallest cut on.
const (
	maxPatchTrials   = 24
	maxCutSearches   = 2
	maxCutCandidates = 8
)

// cliRemedyContext is the remedy context of a command-line run: no stored deck
// to patch and no tool to call.
func cliRemedyContext(filename string, data []byte) *remedyContext {
	rc := newRemedyContext(filename, data, "")
	rc.cli = true
	return rc
}

func newRemedyContext(filename string, data []byte, deckID string) *remedyContext {
	return &remedyContext{data: data, filename: filename, deckID: deckID, trials: maxPatchTrials, tried: map[string]*[]trialFinding{}, cuts: map[string]*cutCandidate{}}
}

// spec is the decoded spec the patches address.
func (rc *remedyContext) spec() any {
	if rc.doc == nil {
		canonical, _ := canonicalSpec(rc.filename, rc.data)
		if json.Unmarshal(canonical, &rc.doc) != nil {
			rc.doc = map[string]any{}
		}
	}
	return rc.doc
}

// nodeAt resolves a dotted DeckSpec path in the spec.
func (rc *remedyContext) nodeAt(path string) (string, any, bool) {
	pointer, ok := semanticPointer(path)
	if !ok {
		return "", nil, false
	}
	parts, err := parsePointer(pointer)
	if err != nil {
		return "", nil, false
	}
	node, err := pointerValue(rc.spec(), parts)
	if err != nil {
		return "", nil, false
	}
	return pointer, node, true
}

// applyOps returns the spec with a patch applied, as JSON.
func (rc *remedyContext) applyOps(ops []any) ([]byte, bool) {
	return applyPatchOps(deepCopyJSON(rc.spec()), ops)
}

// applyPatchOps applies patch ops to a decoded document and encodes it.
func applyPatchOps(doc any, ops []any) ([]byte, bool) {
	encoded, err := json.Marshal(ops)
	if err != nil {
		return nil, false
	}
	var parsed []specPatchOp
	if json.Unmarshal(encoded, &parsed) != nil {
		return nil, false
	}
	for _, op := range parsed {
		if doc, err = op.apply(doc); err != nil {
			return nil, false
		}
	}
	out, err := json.Marshal(doc)
	return out, err == nil
}

// clears applies ops to the spec, validates the result and reports whether the
// subjects are gone with nothing new blocking.
func (rc *remedyContext) clears(subjects []trialFinding, ops []any) bool {
	if rc.run == nil || len(ops) == 0 {
		return false
	}
	key, _ := json.Marshal(ops)
	after, seen := rc.tried[string(key)]
	if !seen {
		if rc.trials <= 0 {
			return false
		}
		rc.trials--
		if patched, applied := rc.applyOps(ops); applied {
			if found, ran := rc.run(patched); ran {
				after = &found
			}
		}
		rc.tried[string(key)] = after
	}
	return after != nil && trialCleared(rc.before, *after, subjects, rc.gateUnknown)
}

// remedyTarget is one finding on its way to a remedy.
type remedyTarget struct {
	code     string // without namespace
	hintCode string // as the finding reports it
	path     string // dotted DeckSpec path
	editPath string // the field to rewrite when it is not path
	raw      map[string]any
	members  []diagMember
	fallback []any
	blocking bool
	// fit says the finding measures the rendered slide.
	fit bool

	// choose says the finding lists the values to choose from.
	choose bool

	candidates [][]any
	ops        []any
	template   bool
	verified   bool
	note       string
}

func (t *remedyTarget) subject() trialFinding {
	s := trialFinding{Code: t.code, Path: t.path}
	for _, m := range t.members {
		s.Paths = append(s.Paths, m.Path)
	}
	return s
}

// patchIsTemplate reports whether a patch leaves a value for the author to
// write: an <instruction> where the words go.
func patchIsTemplate(ops []any) bool {
	for _, op := range ops {
		m, _ := op.(map[string]any)
		if s, ok := m["value"].(string); ok && strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
			return true
		}
	}
	return false
}

// plan lists the patches that could resolve a finding, best first.
func (rc *remedyContext) plan(t *remedyTarget) {
	if rc.deckID == "" {
		return
	}
	add := func(ops []any) {
		if len(ops) > 0 {
			t.candidates = append(t.candidates, ops)
		}
	}
	composition := t.raw["repair"] == "composition"
	switch {
	case t.code == string(diagnostics.CodeTemplateNotFound):
		if name, _ := t.raw["did_you_mean"].(string); name != "" {
			if pointer, node, ok := rc.nodeAt(t.path); ok {
				if _, isText := node.(string); isText {
					add([]any{map[string]any{"op": "replace", "path": pointer, "value": name}})
				}
			}
		}
		add(semanticPatchOps(rc.data, t.path, t.hintCode, t.raw))
	case len(t.members) > 1:
		var ops []any
		for _, m := range t.members {
			params := m.fixParams
			if params == nil {
				params = t.raw
			}
			if params["repair"] == "composition" {
				continue
			}
			ops = append(ops, rc.fieldOps(t, m.Path, params)...)
		}
		add(ops)
	default:
		if !composition {
			// A field that shares its box with the rest of the slide has no
			// budget of its own to rewrite it to.
			add(rc.fieldOps(t, firstNonEmpty(t.editPath, t.path), t.raw))
		}
	}
}

// droppedContentCode is the finding for authored text the slide does not draw.
const droppedContentCode = patterns.ErrCodeContentDropped

// fieldOps is the patch for a finding at one field. A text field with a budget
// gets a rewrite to it. A finding that the slide is over-full, or that the
// field is not drawn, has no rewrite without a budget — writing other words of
// any length is not a fix — so an optional second line is removed instead,
// and any other field is left to the search for the smallest cut.
func (rc *remedyContext) fieldOps(t *remedyTarget, path string, params map[string]any) []any {
	ops := semanticPatchOps(rc.data, path, t.hintCode, params)
	if !patchIsTemplate(ops) || (!cutSearchCodes[t.code] && t.code != droppedContentCode) {
		return ops
	}
	for _, key := range []string{"max_chars", "max_length", "max_words", "max_lines"} {
		if n, ok := intFixParam(params, key); ok && n > 0 {
			return ops
		}
	}
	pointer, _ := semanticPointer(path)
	last := pointer[strings.LastIndexByte(pointer, '/')+1:]
	if t.code == patterns.ErrCodeTextBelowReadableMin || !containsString(cutDetailKeys, last) {
		return nil
	}
	return []any{map[string]any{"op": "remove", "path": pointer}}
}

// choose settles each finding's patch: a patch the author completes is offered
// as it is; a complete one is tried first.
func (rc *remedyContext) choose(targets []*remedyTarget) {
	var pending []*remedyTarget
	for _, t := range targets {
		if len(t.candidates) == 0 {
			continue
		}
		first := t.candidates[0]
		switch {
		case patchIsTemplate(first):
			t.ops, t.template = first, true
		case rc.run == nil:
			t.ops = first
		default:
			pending = append(pending, t)
		}
	}
	if len(pending) > 1 {
		// Patches for separate findings rarely interact: one validation of them
		// all settles the common case.
		var all []any
		subjects := make([]trialFinding, 0, len(pending))
		for _, t := range pending {
			all = append(all, t.candidates[0]...)
			subjects = append(subjects, t.subject())
		}
		if rc.clears(subjects, all) {
			for _, t := range pending {
				t.ops, t.verified = t.candidates[0], true
			}
			return
		}
	}
	for _, t := range pending {
		for _, ops := range t.candidates {
			if patchIsTemplate(ops) {
				t.ops, t.template = ops, true
				break
			}
			if rc.clears([]trialFinding{t.subject()}, ops) {
				t.ops, t.verified = ops, true
				break
			}
		}
	}
}

// cutCandidate is one removal that might clear an over-full slide.
type cutCandidate struct {
	// path is the pointer the candidate removes.
	path string
	ops  []any
	cost int
	note string
	// entry marks the removal of a whole list entry rather than one line.
	entry bool
}

// cutDetailKeys are the optional second lines of a list entry.
var cutDetailKeys = []string{"detail", "description", "body", "support", "bio", "subtitle", "comparator"}

// textLength is the number of characters of authored text under v.
func textLength(v any) int {
	switch t := v.(type) {
	case string:
		return utf8.RuneCountInString(t)
	case map[string]any:
		n := 0
		for _, child := range t {
			n += textLength(child)
		}
		return n
	case []any:
		n := 0
		for _, child := range t {
			n += textLength(child)
		}
		return n
	}
	return 0
}

// removal is the candidate that removes the value at path.
func removal(path string, cost int, entry bool, note string) cutCandidate {
	return cutCandidate{path: path, ops: []any{map[string]any{"op": "remove", "path": path}}, cost: cost, entry: entry, note: note}
}

// lineRemoval is the candidate that removes one line of text.
func lineRemoval(path, text string) cutCandidate {
	n := utf8.RuneCountInString(text)
	return removal(path, n, false, fmt.Sprintf("removing %s (%d characters) clears this; the rest fits as written", path, n))
}

func byCost(list []cutCandidate) {
	sort.SliceStable(list, func(i, j int) bool { return list[i].cost < list[j].cost })
}

// focusCuts are the removals of one entry of the list a finding sits on,
// cheapest first.
func focusCuts(focusPointer string, focus any) []cutCandidate {
	list, ok := focus.([]any)
	if !ok || len(list) < 2 {
		return nil
	}
	var out []cutCandidate
	for i, item := range list {
		if text, isText := item.(string); isText && text != "" {
			out = append(out, lineRemoval(fmt.Sprintf("%s/%d", focusPointer, i), text))
		}
	}
	byCost(out)
	return out
}

// listCuts are the removals inside one list of a slide: an optional second
// line of each entry, and the last entry.
func listCuts(base, key string, list []any, lastEntry bool) []cutCandidate {
	var out []cutCandidate
	for i, item := range list {
		entry, isObject := item.(map[string]any)
		if !isObject {
			continue
		}
		for _, dk := range cutDetailKeys {
			if text, isText := entry[dk].(string); isText && text != "" {
				out = append(out, lineRemoval(fmt.Sprintf("%s/%d/%s", base, i, dk), text))
			}
		}
	}
	if lastEntry {
		last := len(list) - 1
		path := fmt.Sprintf("%s/%d", base, last)
		out = append(out, removal(path, textLength(list[last])+1, true,
			fmt.Sprintf("the limit here is the number of %s: %d fit, so removing %s clears this", key, last, path)))
	}
	return out
}

// cutCandidates lists the single removals worth trying on a slide: one
// optional line of one list entry, the takeaway, the last entry of a list.
// When the finding names a list inside the slide (focus, with its pointer),
// the entries of that list come first; the rest follow, cheapest first.
func cutCandidates(pointer string, slide map[string]any, focusPointer string, focus any) []cutCandidate {
	var out []cutCandidate
	if focusPointer != pointer {
		out = focusCuts(focusPointer, focus)
	}
	first := len(out)
	entries := hasObjectList(slide)
	for _, key := range sortedMapKeys(slide) {
		list, ok := slide[key].([]any)
		if !ok || len(list) < 2 {
			continue
		}
		// A list of plain strings beside the slide's items is its axis or its
		// criteria; the items are what there are too many of.
		_, isObject := list[len(list)-1].(map[string]any)
		out = append(out, listCuts(pointer+"/"+escapePointerSegment(key), key, list, isObject || !entries)...)
	}
	if text, ok := slide["takeaway"].(string); ok && text != "" {
		c := lineRemoval(pointer+"/takeaway", text)
		c.note = fmt.Sprintf("removing %s/takeaway clears this; the rest fits as written", pointer)
		out = append(out, c)
	}
	// Cheapest first. The search has only so many tries, and when the limit is
	// the number of rows no single line's removal clears it: the cheapest
	// whole entry always has a try.
	byCost(out[first:])
	if len(out) <= maxCutCandidates {
		return out
	}
	kept := false
	for _, c := range out[:maxCutCandidates] {
		kept = kept || c.entry
	}
	for _, c := range out[maxCutCandidates:] {
		if c.entry && !kept {
			out[maxCutCandidates-1] = c
			break
		}
	}
	return out[:maxCutCandidates]
}

// hasObjectList reports whether a slide carries a list of entries.
func hasObjectList(slide map[string]any) bool {
	for _, v := range slide {
		if list, ok := v.([]any); ok && len(list) > 1 {
			if _, isObject := list[0].(map[string]any); isObject {
				return true
			}
		}
	}
	return false
}

// cutSearchCodes are the findings that say a slide holds more than fits.
var cutSearchCodes = map[string]bool{
	patterns.ErrCodeBodyTooLong:          true,
	patterns.ErrCodeTextBelowReadableMin: true,
	patterns.ErrCodeFitOverflow:          true,
	patterns.ErrCodeDensityExceeded:      true,
}

// slideAlone validates one slide in a deck of its own, under the spec's meta.
func (rc *remedyContext) slideAlone(slide map[string]any) ([]trialFinding, bool) {
	if rc.trials <= 0 {
		return nil, false
	}
	rc.trials--
	mini := map[string]any{"slides": []any{slide}}
	root, _ := rc.spec().(map[string]any)
	if meta, has := root["meta"].(map[string]any); has {
		// The deck's required layouts are not this one slide's to satisfy.
		own := make(map[string]any, len(meta))
		for k, v := range meta {
			if k != "required_layouts" {
				own[k] = v
			}
		}
		mini["meta"] = own
	}
	raw, err := json.Marshal(mini)
	if err != nil {
		return nil, false
	}
	return rc.run(raw)
}

// withoutPath returns a copy of the slide at pointer with the value at path
// (a pointer under it) removed.
func withoutPath(pointer string, slide map[string]any, path string) (map[string]any, bool) {
	local := []any{map[string]any{"op": "remove", "path": "/slides/0" + strings.TrimPrefix(path, pointer)}}
	patched, applied := applyPatchOps(map[string]any{"slides": []any{deepCopyJSON(slide)}}, local)
	if !applied {
		return nil, false
	}
	var doc struct {
		Slides []map[string]any `json:"slides"`
	}
	if json.Unmarshal(patched, &doc) != nil || len(doc.Slides) != 1 {
		return nil, false
	}
	return doc.Slides[0], true
}

// searchCut looks for the smallest single removal that clears an over-full
// slide (go-slide-creator-4mmvb). Each candidate is validated on the slide
// alone, which is quick; the one that clears it is then validated in the deck.
func (rc *remedyContext) searchCut(t *remedyTarget) {
	pointer, node, ok := rc.nodeAt(slideContainer(t.path))
	slide, isSlide := node.(map[string]any)
	if !ok || !isSlide {
		return
	}
	focusPointer, focus, _ := rc.nodeAt(t.path)
	candidates := cutCandidates(pointer, slide, focusPointer, focus)
	if len(candidates) == 0 {
		return
	}
	base, ok := rc.slideAlone(slide)
	// The slide alone must show the finding, or what clears it there says
	// nothing about the deck.
	reproduced := false
	for _, f := range base {
		reproduced = reproduced || (f.Code == t.code && f.Blocking)
	}
	if !ok || !reproduced {
		return
	}
	subject := []trialFinding{t.subject()}
	for _, c := range candidates {
		cut, applied := withoutPath(pointer, slide, c.path)
		if !applied {
			continue
		}
		after, ran := rc.slideAlone(cut)
		if !ran {
			return
		}
		if !cutClearsSlide(base, after, t.code) || !rc.clears(subject, c.ops) {
			continue
		}
		t.ops, t.verified, t.note = c.ops, true, c.note
		// The switch to the kind's own text layout loses no words; say so when
		// it works too, so the author can choose.
		if rc.clears(subject, t.fallback) {
			t.note += " (or keep every word and give up the visual: " + layoutSwitchPhrase(t.fallback) + ")"
		}
		return
	}
}

// layoutSwitchPhrase words the fallback patch: `set /slides/1/layout to "content"`.
func layoutSwitchPhrase(fallback []any) string {
	if len(fallback) == 0 {
		return ""
	}
	op, _ := fallback[0].(map[string]any)
	return fmt.Sprintf("set %v to %q", op["path"], op["value"])
}

// cutClearsSlide compares a slide validated alone before and after a removal:
// the finding is gone, and nothing blocks or warns that did not before.
func cutClearsSlide(base, after []trialFinding, code string) bool {
	had := map[string]bool{}
	for _, f := range base {
		had[f.Code] = true
	}
	for _, f := range after {
		if slideContainer(f.Path) == "" {
			// A deck of one slide fails deck-wide checks whatever the slide holds.
			continue
		}
		if f.Code == code && f.Blocking {
			return false
		}
		if (f.Blocking || f.Warning || fallbackRootCodes[f.Code]) && !had[f.Code] {
			return false
		}
	}
	return true
}

// resolve settles the patch of every target.
func (rc *remedyContext) resolve(targets []*remedyTarget) {
	if rc.deckID == "" {
		return
	}
	for _, t := range targets {
		rc.plan(t)
	}
	rc.choose(targets)
	searches := 0
	for _, t := range targets {
		if rc.run == nil || len(t.ops) > 0 || !t.blocking || !t.fit || !cutSearchCodes[t.code] {
			continue
		}
		slide := slideContainer(t.path)
		cut, searched := rc.cuts[slide]
		if !searched {
			if searches == maxCutSearches {
				continue
			}
			searches++
			rc.searchCut(t)
			if t.verified {
				cut = &cutCandidate{ops: t.ops, note: t.note}
			}
			rc.cuts[slide] = cut
			continue
		}
		// Another finding on a slide whose cut is known: the same removal, when
		// it clears this one too.
		if cut != nil && rc.clears([]trialFinding{t.subject()}, cut.ops) {
			t.ops, t.verified, t.note = cut.ops, true, cut.note
		}
	}
	// Last, the switch to the kind's native layout: it keeps every word and
	// gives up the visual, so a cut that keeps the visual comes first.
	for _, t := range targets {
		if len(t.ops) > 0 || len(t.fallback) == 0 {
			continue
		}
		switch {
		case rc.run == nil:
			t.ops = t.fallback
		case rc.clears([]trialFinding{t.subject()}, t.fallback):
			t.ops, t.verified = t.fallback, true
			t.note = layoutSwitchPhrase(t.fallback) + ": the slide keeps every word and gives up the visual for the template's own text layout"
		}
	}
}

// remedy is what a settled target tells the author: the action, its budgets,
// and the patch call.
func (t *remedyTarget) remedy(deckID string) (*diagnostics.RemediationAction, *patterns.ToolCallSuggestion) {
	facts := memberFacts(t.raw, t.members)
	has := func(keys ...string) bool {
		for _, k := range keys {
			if _, ok := facts[k]; ok {
				return true
			}
		}
		return false
	}
	var call *patterns.ToolCallSuggestion
	action := ""
	switch {
	case len(t.ops) > 0 && !t.template:
		action = diagnostics.ActionApplyPatch
	case has("max_chars", "max_words"):
		action = diagnostics.ActionShortenText
	case has("max_items", "min_items"):
		action = diagnostics.ActionReduceItems
	case has("max_rows", "row"):
		action = diagnostics.ActionSplitSlide
	case len(facts) > 0 || len(t.ops) > 0 || t.choose:
		action = diagnostics.ActionReplaceValue
	}
	if len(t.ops) > 0 {
		call = semanticPatchSuggestion(deckID, t.ops)
	}
	if action == "" {
		return nil, call
	}
	return &diagnostics.RemediationAction{Action: action, Params: facts}, call
}

// verifiedNote is the sentence a verified smallest cut adds to its finding.
func (t *remedyTarget) verifiedNote() string {
	if t.note == "" {
		return ""
	}
	return " — verified fix: " + t.note
}

// --- the validate envelope ----------------------------------------------------

// remedyEnvelope gives every finding of a DeckSpec envelope its remedy. diags
// are the diagnostics the envelope was built from (index-aligned), or nil.
func (rc *remedyContext) remedyEnvelope(envelope *diagnostics.FindingEnvelope, diags []semanticDiagnostic) {
	targets := make([]*remedyTarget, len(envelope.Findings))
	kept := make([]*patterns.ToolCallSuggestion, len(envelope.Findings))
	for i := range envelope.Findings {
		f := &envelope.Findings[i]
		var source *semanticDiagnostic
		if i < len(diags) {
			source = &diags[i]
		}
		t := &remedyTarget{code: bareCode(f.Code), hintCode: f.Code, blocking: f.Severity == diagnostics.SeverityError}
		_, t.raw = findingFixParams(f.Remediation)
		t.path, _ = f.Evidence["path"].(string)
		t.editPath, _ = f.Evidence[editPathDetail].(string)
		t.fallback, _ = f.Evidence[compositionPatchDetail].([]any)
		delete(f.Evidence, compositionPatchDetail)
		delete(f.Evidence, editPathDetail)
		// The fit action is the raw surfaces' grading; here severity and blocking
		// say whether the finding stops the deck.
		_, t.fit = f.Evidence["action"]
		delete(f.Evidence, "action")
		if source != nil {
			t.members = source.members
			t.fit = t.fit || source.Action != ""
			if len(t.fallback) == 0 {
				t.fallback = source.fallbackPatch
			}
		}
		if f.NextToolCall != nil && (t.code == string(diagnostics.CodeSemanticUnknownKind) || t.code == string(diagnostics.CodeTemplateNotFound)) {
			// A discovery call the finding already names.
			kept[i] = f.NextToolCall
		}
		if t.code == string(diagnostics.CodeSemanticUnknownKind) && kept[i] == nil {
			kept[i] = &patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{}}
		}
		if t.code == string(diagnostics.CodeTemplateNotFound) && kept[i] == nil {
			kept[i] = nextCallListTemplates()
		}
		if _, listed := f.Evidence["available"]; listed {
			// The choices are evidence: neither the remedy nor the sentence
			// repeats them.
			delete(t.raw, "available")
			t.choose = true
			if at := strings.Index(f.Message, "; expected one of "); at > 0 {
				f.Message = f.Message[:at] + "; evidence.available lists the choices"
			}
		}
		targets[i] = t
	}
	rc.resolve(targets)
	for i := range envelope.Findings {
		f, t := &envelope.Findings[i], targets[i]
		action, call := t.remedy(rc.deckID)
		f.Remediation = nil
		if action != nil {
			f.Remediation = &diagnostics.Remediation{Primary: action}
		}
		f.NextToolCall = call
		if call == nil {
			f.NextToolCall = kept[i]
		}
		f.PatchVerified = t.verified
		f.Message += t.verifiedNote()
		if f.NextToolCall == nil && f.Remediation == nil && t.blocking && !rc.cli {
			// Nothing else says what to do: the code's own page does.
			f.NextToolCall = &patterns.ToolCallSuggestion{Tool: "describe_finding", ArgsTemplate: map[string]any{"code": f.Code}}
		}
	}
}

// --- render diagnostics --------------------------------------------------------

// remedyDiagnostics gives every render diagnostic its remedy: the same patch,
// flag and facts remedyEnvelope gives the finding validate reports for it.
func (rc *remedyContext) remedyDiagnostics(ds []semanticDiagnostic) {
	targets := make([]*remedyTarget, len(ds))
	for i := range ds {
		d := &ds[i]
		t := &remedyTarget{code: d.Code, hintCode: d.Code, path: d.SemanticPath, members: d.members, fallback: d.fallbackPatch,
			blocking: diagnosticBlocks(*d), fit: d.Action != ""}
		t.raw = map[string]any{}
		if d.diag != nil {
			if d.diag.Fix != nil {
				for k, v := range semanticFixParams(d.diag.Fix.Kind, d.diag.Fix.Params) {
					t.raw[k] = v
				}
			}
			t.editPath, _ = d.diag.Details[editPathDetail].(string)
			if len(t.fallback) == 0 {
				t.fallback, _ = d.diag.Details[compositionPatchDetail].([]any)
			}
		}
		if d.RecommendedEdit != nil {
			for k, v := range d.RecommendedEdit.Params {
				t.raw[k] = v
			}
		}
		targets[i] = t
	}
	rc.resolve(targets)
	described := map[string]bool{}
	for i := range ds {
		d, t := &ds[i], targets[i]
		action, call := t.remedy(rc.deckID)
		d.NextToolCall = call
		d.patchVerified = t.verified
		d.Message += t.verifiedNote()
		if d.RecommendedEdit != nil {
			edit := *d.RecommendedEdit
			edit.Params = nil
			if action != nil {
				edit.Params = action.Params
			}
			d.RecommendedEdit = &edit
		}
		// describe_finding is offered on a blocking finding that carries no
		// remedy of its own, once per code.
		if d.NextToolCall == nil && d.RecommendedEdit == nil && t.blocking && !described[d.Code] && !rc.cli {
			described[d.Code] = true
			d.NextToolCall = &patterns.ToolCallSuggestion{Tool: "describe_finding", ArgsTemplate: map[string]any{"code": d.Code}}
		}
	}
}
