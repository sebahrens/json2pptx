package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// Everything knowable in the first response (go-slide-creator-ipahe,
// go-slide-creator-t0c1m).
//
// A DeckSpec with one spec-level error — an unknown kind on slide 5, a
// misspelled key on slide 3 — does not compile, so the render behind validate
// never ran and nothing else was reported: no fit finding, no topic title, no
// missing source, no "Thank you" closer. Each tier appeared only once the tier
// above it was clean, and a draft with twelve planted flaws took seven
// validates. Every one of those findings was knowable from the first call: they
// sit on slides the error does not touch.
//
// When a spec has blocking spec-level errors the tools now evaluate what is
// left once the errors are set aside:
//
//   - a key with a confident did_you_mean is read under the name it was meant
//     to have, so what followed from its being dropped (a "takeaway is
//     missing" beside "takeway is unknown", a comparison that "degrades"
//     because its third column lost its items) is not reported as a second
//     cause;
//   - any other unknown key is left out, as the compiler would leave it out;
//   - a slide that still has an error is left out of the run, with every
//     finding known about it;
//   - an invalid meta field is left out.
//
// What remains is compiled and rendered into a scratch directory exactly as a
// clean spec is, and its findings are reported at the paths of the spec the
// author sent. Checks that judge the deck as a whole (rhythm, the storyline,
// the quality gate) are not run on a deck with a slide missing, and the
// response says so.

// salvageResult is a spec with its blocking errors set aside.
type salvageResult struct {
	// Data is the reduced spec, as JSON.
	Data []byte
	// Kept are the findings about the authored spec that the reduction acted
	// on, at authored paths: the errors themselves, and everything known about
	// a slide that was left out.
	Kept []diagnostics.Diagnostic
	// Renamed lists the keys read under their did_you_mean and Removed the
	// slides left out, at authored paths.
	Renamed []string
	Removed []string
	// removedSlides are the authored objects of the slides left out.
	removedSlides []map[string]any
	// droppedFields are the authored paths of unknown keys left out without a
	// suggestion, for attributing what follows from them.
	droppedFields []string
	remap         salvageRemap
}

// salvageRemap translates a path in the reduced spec to the authored spec.
type salvageRemap struct {
	// index maps an array's authored path ("slides",
	// "structure.sections[0].slides") to the authored index of each element
	// it still has.
	index map[string][]int
	// renames are applied in order: under parent (authored path), key to was
	// authored as from.
	renames []salvageRename
}

type salvageRename struct{ parent, to, from string }

// salvageArrayRE matches a path inside an element the reduction can leave
// out: a slide of a slide list, or an entry of a meta list (a waiver, a
// required layout).
var salvageArrayRE = regexp.MustCompile(`^(slides|structure\.sections\[\d+\]\.slides|meta\.[a-z_]+)\[(\d+)\]`)

// authored translates a dotted path in the reduced spec to the authored spec.
func (r salvageRemap) authored(path string) string {
	if m := salvageArrayRE.FindStringSubmatch(path); m != nil {
		if kept, ok := r.index[m[1]]; ok {
			if i, err := strconv.Atoi(m[2]); err == nil && i < len(kept) {
				path = m[1] + "[" + strconv.Itoa(kept[i]) + "]" + path[len(m[0]):]
			}
		}
	}
	for _, rn := range r.renames {
		prefix := strings.TrimPrefix(rn.parent+"."+rn.to, ".")
		if path == prefix || strings.HasPrefix(path, prefix+".") || strings.HasPrefix(path, prefix+"[") {
			path = strings.TrimPrefix(rn.parent+"."+rn.from, ".") + path[len(prefix):]
		}
	}
	return path
}

// salvageMentionRE finds a slide reference inside a message.
var salvageMentionRE = regexp.MustCompile(`\b(?:structure\.sections\[\d+\]\.)?slides\[\d+\]`)

// authoredText translates the slide references inside a message.
func (r salvageRemap) authoredText(text string) string {
	if len(r.index) == 0 {
		return text
	}
	return salvageMentionRE.ReplaceAllStringFunc(text, r.authored)
}

// salvageMaxPasses bounds the reduction: each pass removes at least one error.
const salvageMaxPasses = 8

// salvageSpec sets a spec's blocking errors aside. ok is false when nothing
// evaluable is left (the document does not decode, every slide has an error,
// an error sits where nothing can be left out).
func salvageSpec(filename string, data []byte, strict semantic.Strictness) (*salvageResult, bool) {
	canonical, name := canonicalSpec(filename, data)
	var root map[string]any
	if json.Unmarshal(canonical, &root) != nil || root == nil {
		return nil, false
	}
	res := &salvageResult{remap: salvageRemap{index: map[string][]int{}}}
	edited := false
	for pass := 0; pass < salvageMaxPasses; pass++ {
		cur, err := json.Marshal(root)
		if err != nil {
			return nil, false
		}
		findings := semantic.Check(name, cur, strict)
		if !diagnostics.HasErrors(findings) {
			// Compilation runs checks the spec-level pass does not (the lowered
			// patterns' own contracts).
			spec, parseDiags := semantic.Parse(name, cur)
			if spec == nil || parseDiags.HasErrors() {
				return nil, false
			}
			_, cr, compileErr := semantic.Compile(spec, semantic.CompileOptions{Strict: strict})
			if compileErr == nil {
				res.Data = cur
				return res, edited
			}
			if cr == nil || !diagnostics.HasErrors(cr.Diagnostics) {
				return nil, false
			}
			findings = cr.Diagnostics
		}
		if !res.setAside(root, findings) {
			return nil, false
		}
		edited = true
	}
	return nil, false
}

// setAside removes one layer of errors from root: the field-level ones when
// there are any (what follows from them is re-read on the next pass),
// otherwise the slides that still have an error.
func (res *salvageResult) setAside(root map[string]any, findings []diagnostics.Diagnostic) bool {
	fieldEdits := 0
	for _, d := range findings {
		if d.Severity == diagnostics.SeverityError && res.setFieldAside(root, d) {
			fieldEdits++
		}
	}
	if fieldEdits > 0 {
		return true
	}
	remove, ok := erroredElements(findings)
	if !ok {
		return false
	}
	// Everything known about an element that is left out stays in the report.
	for _, d := range findings {
		if m := salvageArrayRE.FindStringSubmatch(d.Path); m != nil {
			if i, _ := strconv.Atoi(m[2]); remove[m[1]][i] {
				res.keep(d)
			}
		}
	}
	arrays := make([]string, 0, len(remove))
	for arr := range remove {
		arrays = append(arrays, arr)
	}
	sort.Strings(arrays)
	for _, arr := range arrays {
		if !res.removeElements(root, arr, remove[arr]) {
			return false
		}
	}
	return true
}

// erroredElements returns, per list, the elements that carry an error. ok is
// false when there is none, or an error sits outside any element that can be
// left out (a slide of a list, an entry of a meta list).
func erroredElements(findings []diagnostics.Diagnostic) (map[string]map[int]bool, bool) {
	remove := map[string]map[int]bool{}
	for _, d := range findings {
		if d.Severity != diagnostics.SeverityError {
			continue
		}
		m := salvageArrayRE.FindStringSubmatch(d.Path)
		if m == nil {
			return nil, false
		}
		i, _ := strconv.Atoi(m[2])
		if remove[m[1]] == nil {
			remove[m[1]] = map[int]bool{}
		}
		remove[m[1]][i] = true
	}
	return remove, len(remove) > 0
}

// removeElements leaves the marked elements of one list out of root and
// records where the remaining ones were authored. A slide list must keep at
// least one slide.
func (res *salvageResult) removeElements(root map[string]any, arr string, remove map[int]bool) bool {
	list, ok := nodeAtDotted(root, arr).([]any)
	if !ok {
		return false
	}
	kept, had := res.remap.index[arr]
	if !had {
		kept = make([]int, len(list))
		for i := range kept {
			kept[i] = i
		}
	}
	isSlides := !strings.HasPrefix(arr, "meta.")
	nextList := []any{}
	var nextKept []int
	for i, element := range list {
		if !remove[i] {
			nextList = append(nextList, element)
			nextKept = append(nextKept, kept[i])
			continue
		}
		if isSlides {
			res.Removed = append(res.Removed, fmt.Sprintf("%s[%d]", arr, kept[i]))
			if m, isMap := element.(map[string]any); isMap {
				res.removedSlides = append(res.removedSlides, m)
			}
		}
	}
	if len(nextList) == 0 && isSlides {
		return false
	}
	res.remap.index[arr] = nextKept
	return setNodeAtDotted(root, arr, nextList)
}

// keep records a finding at its authored path.
func (res *salvageResult) keep(d diagnostics.Diagnostic) {
	d.Path = res.remap.authored(d.Path)
	d.Message = res.remap.authoredText(d.Message)
	for _, have := range res.Kept {
		if have.Code == d.Code && have.Path == d.Path {
			return
		}
	}
	res.Kept = append(res.Kept, d)
}

// setFieldAside handles an error that names one removable field: an unknown
// key (renamed to its suggestion, or left out) or an invalid meta field.
func (res *salvageResult) setFieldAside(root map[string]any, d diagnostics.Diagnostic) bool {
	tokens := dottedTokens(d.Path)
	if len(tokens) == 0 {
		return false
	}
	isMeta := tokens[0] == "meta" && len(tokens) >= 2 && tokens[1] != "title"
	if d.Code != diagnostics.CodeSemanticUnknownField && !isMeta {
		return false
	}
	if isMeta && d.Code != diagnostics.CodeSemanticUnknownField && salvageArrayRE.MatchString(d.Path) {
		// One entry of a meta list (a waiver): the entry goes, not the list.
		return false
	}
	if isMeta && d.Code != diagnostics.CodeSemanticUnknownField {
		// The whole meta field goes: an invalid archetype, a malformed waiver.
		tokens = tokens[:2]
	}
	parent, ok := nodeAtTokens(root, tokens[:len(tokens)-1]).(map[string]any)
	key := tokens[len(tokens)-1]
	if !ok {
		return false
	}
	value, present := parent[key]
	if !present {
		return false
	}
	res.keep(d)
	delete(parent, key)
	parentPath := res.remap.authored(parentDotted(d.Path, isMeta && d.Code != diagnostics.CodeSemanticUnknownField))
	if d.Code == diagnostics.CodeSemanticUnknownField && d.Fix != nil && d.Fix.Kind == "rename_field" {
		if to, _ := d.Fix.Params["to"].(string); to != "" {
			if _, taken := parent[to]; !taken {
				parent[to] = value
				res.remap.renames = append(res.remap.renames, salvageRename{parent: parentPath, to: to, from: key})
				res.Renamed = append(res.Renamed, fmt.Sprintf("%s → %s at %s", key, to, specPointer(parentPath)))
				return true
			}
		}
	}
	if d.Code == diagnostics.CodeSemanticUnknownField {
		res.droppedFields = append(res.droppedFields, res.remap.authored(d.Path))
	}
	return true
}

// parentDotted returns the dotted path of the object holding the field a
// finding names; metaField cuts a deeper meta path back to "meta".
func parentDotted(path string, metaField bool) string {
	if metaField {
		return "meta"
	}
	if strings.HasSuffix(path, "]") {
		return path[:strings.LastIndexByte(path, '[')]
	}
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[:i]
	}
	return ""
}

func nodeAtDotted(root any, path string) any { return nodeAtTokens(root, dottedTokens(path)) }

func nodeAtTokens(root any, tokens []string) any {
	node := root
	for _, tok := range tokens {
		switch current := node.(type) {
		case map[string]any:
			node = current[tok]
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(current) {
				return nil
			}
			node = current[i]
		default:
			return nil
		}
	}
	return node
}

func setNodeAtDotted(root map[string]any, path string, value any) bool {
	tokens := dottedTokens(path)
	if len(tokens) == 0 {
		return false
	}
	switch parent := nodeAtTokens(root, tokens[:len(tokens)-1]).(type) {
	case map[string]any:
		parent[tokens[len(tokens)-1]] = value
		return true
	case []any:
		i, err := strconv.Atoi(tokens[len(tokens)-1])
		if err != nil || i < 0 || i >= len(parent) {
			return false
		}
		parent[i] = value
		return true
	}
	return false
}

// deckWideCodes are findings that judge the deck as a whole. They are not
// reported from a run that left a slide out: a missing slide can be the
// executive summary, the break in a monotonous run, or the score's shortfall.
var deckWideCodes = map[string]bool{
	patterns.ErrCodeQualityGate:                 true,
	patterns.ErrCodeNoExecutiveSummary:          true,
	diagnostics.CodeSemanticRhythmMonotony:      true,
	diagnostics.CodeSemanticRhythmDensity:       true,
	diagnostics.CodeSemanticRhythmSectioning:    true,
	diagnostics.CodeSemanticRhythmSynthesis:     true,
	diagnostics.CodeSemanticReferenceUnresolved: true,
}

// removedSlideStatesAsk reports whether a slide left out of the run could be
// the deck's next steps, which is what CLOSING_WITHOUT_NEXT_STEPS looks for.
func (res *salvageResult) removedSlideStatesAsk() bool {
	for _, slide := range res.removedSlides {
		if kind, _ := slide["kind"].(string); kind == string(semantic.KindNextSteps) {
			return true
		}
		title, _ := slide["title"].(string)
		lower := strings.ToLower(title)
		for _, marker := range askTitleMarkers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}

// downstreamCodes are findings an unknown key that was left out can cause:
// the content it carried is missing.
var downstreamCodes = map[string]bool{
	diagnostics.CodeSemanticRequired:         true,
	diagnostics.CodeSemanticTakeawayRequired: true,
	diagnostics.CodeSemanticPatternDegraded:  true,
}

// causedBy returns the authored path of an unknown key that was left out and
// that the finding at path most likely follows from: the finding names a
// sibling of the key, or a list the key's object is an entry of.
func (res *salvageResult) causedBy(code, path string) string {
	if !downstreamCodes[code] {
		return ""
	}
	for _, field := range res.droppedFields {
		holder := parentDotted(field, false)
		if parentDotted(path, false) == holder || strings.HasPrefix(holder, path+"[") {
			return field
		}
	}
	return ""
}

// CausedByDetail is the evidence key naming the dropped field a finding
// follows from, as a pointer into the authored spec.
const causedByDetail = "caused_by"

// markCausedBy attributes a diagnostic to the unknown key it follows from.
func (res *salvageResult) markCausedBy(d *diagnostics.Diagnostic) {
	field := res.causedBy(d.Code, d.Path)
	if field == "" {
		return
	}
	details := map[string]any{}
	for k, v := range d.Details {
		details[k] = v
	}
	details[causedByDetail] = specPointer(field)
	d.Details = details
	d.Message += fmt.Sprintf(" — likely because %s is an unknown key and its content was dropped; fix that first", specPointer(field))
}

// notes says what the reduction did, for the response's warnings.
func (res *salvageResult) notes() []string {
	var out []string
	if len(res.Renamed) > 0 {
		out = append(out, "measured with each misspelled key read as its did_you_mean ("+strings.Join(res.Renamed, "; ")+")")
	}
	if len(res.Removed) > 0 {
		paths := make([]string, len(res.Removed))
		for i, p := range res.Removed {
			paths[i] = specPointer(p)
		}
		out = append(out, "checks_not_run: "+strings.Join(paths, ", ")+" did not compile, so their fit and the deck-wide checks (rhythm, executive summary, quality gate) wait for their errors to be fixed")
	}
	return out
}

// salvagedEvaluation is what a spec with blocking errors still tells: its
// errors, and the findings of the rest of it.
//
// evaluate runs a spec exactly as the calling tool evaluates a clean one.
func salvagedEvaluation(filename string, data []byte, strict semantic.Strictness, evaluate func(spec *semantic.DeckSpec, data []byte) specEvaluation) (specEvaluation, bool) {
	res, ok := salvageSpec(filename, data, strict)
	if !ok {
		return specEvaluation{}, false
	}
	spec, parseDiags := semantic.Parse("salvaged.json", res.Data)
	if spec == nil || parseDiags.HasErrors() {
		return specEvaluation{}, false
	}
	eval := evaluate(spec, res.Data)
	if !eval.Evaluated || eval.CompileFailed {
		return specEvaluation{}, false
	}
	enrichSemanticKindDiagnostics(res.Kept)

	out := make([]semanticDiagnostic, 0, len(res.Kept)+len(eval.Diagnostics))
	for _, d := range res.Kept {
		res.markCausedBy(&d)
		out = append(out, semanticDiagFromCompile(d))
	}
	slidesRemoved := len(res.Removed) > 0
	statesAsk := res.removedSlideStatesAsk()
	for _, d := range eval.Diagnostics {
		if slidesRemoved && (deckWideCodes[d.Code] || (statesAsk && d.Code == patterns.ErrCodeClosingWithoutNextSteps)) {
			continue
		}
		d = res.authoredDiagnostic(d)
		out = appendDistinctDiagnostic(out, d)
	}
	eval.Diagnostics = out
	eval.Warnings = append(eval.Warnings, res.notes()...)
	eval.Salvaged = true
	return eval, true
}

// authoredDiagnostic moves a diagnostic of the reduced spec to the authored
// spec's paths.
func (res *salvageResult) authoredDiagnostic(d semanticDiagnostic) semanticDiagnostic {
	d.Message = res.remap.authoredText(d.Message)
	before := d.SemanticPath
	if d.SemanticPath != "" && !strings.HasPrefix(d.SemanticPath, "/") {
		d.SemanticPath = res.remap.authored(d.SemanticPath)
	}
	if len(res.Removed) > 0 {
		// The compiled deck of the reduced spec is not the author's deck: its
		// pointers would name other slides.
		d.RawPath = ""
	}
	reduced := d.SlideIndex
	if reduced == nil {
		reduced = flatSlideIndex(before)
	}
	d.SlideIndex = authoredSlideIndex(d.SemanticPath, d.SlideIndex, len(res.Removed) > 0)
	d.baseMessage = res.remap.authoredText(d.baseMessage)
	authoredIdx := d.SlideIndex
	if authoredIdx == nil {
		authoredIdx = flatSlideIndex(d.SemanticPath)
	}
	if reduced != nil && authoredIdx != nil {
		// "slide N:" names the slide by its number in the authored deck.
		d.Message = slideNumberMessage(d.Message, *reduced, *authoredIdx)
		d.baseMessage = slideNumberMessage(d.baseMessage, *reduced, *authoredIdx)
	}
	for i := range d.Symptoms {
		if p := d.Symptoms[i].Path; p != "" && !strings.HasPrefix(p, "/") {
			d.Symptoms[i].Path = res.remap.authored(p)
		}
		d.Symptoms[i].Message = res.remap.authoredText(d.Symptoms[i].Message)
		if reduced != nil && authoredIdx != nil {
			d.Symptoms[i].Message = slideNumberMessage(d.Symptoms[i].Message, *reduced, *authoredIdx)
		}
	}
	for i := range d.members {
		d.members[i].Path = res.remap.authored(d.members[i].Path)
		d.members[i].SlideIndex = authoredSlideIndex(d.members[i].Path, d.members[i].SlideIndex, len(res.Removed) > 0)
	}
	if d.diag != nil {
		source := *d.diag
		source.Message = d.Message
		if source.Path != "" && !strings.HasPrefix(source.Path, "/") {
			source.Path = res.remap.authored(source.Path)
		}
		if cause := res.causedBy(d.Code, d.SemanticPath); cause != "" {
			res.markCausedBy(&source)
			d.Message = source.Message
			if d.Evidence == nil {
				d.Evidence = map[string]any{}
			}
			d.Evidence[causedByDetail] = specPointer(cause)
		}
		if len(d.Symptoms) > 0 {
			details := map[string]any{}
			for k, v := range source.Details {
				details[k] = v
			}
			details[symptomsDetail] = symptomsEvidence(d.Symptoms)
			source.Details = details
		}
		d.diag = &source
	}
	return d
}

// authoredSlideIndex is the slide a diagnostic reports once its path is back in
// the authored spec. With a slide left out, the reduced deck's positions are
// not the authored deck's: a flat path names its own index, anything else has
// no known position.
func authoredSlideIndex(path string, index *int, slidesRemoved bool) *int {
	if !slidesRemoved {
		return index
	}
	if m := flatSlidePath.FindStringSubmatch(path); m != nil {
		if i, err := strconv.Atoi(m[1]); err == nil {
			return &i
		}
	}
	return nil
}

// evaluateSpecFindings is the one finding collection of a DeckSpec call
// (validate, or a render that was refused before it ran): the evaluation of the
// spec, or — when the spec has blocking spec-level errors — those errors with
// the findings of the rest of it. ds are the findings as transport-neutral
// diagnostics, index-aligned with eval.Diagnostics when eval.Evaluated.
func evaluateSpecFindings(filename string, data []byte, strict semantic.Strictness, evaluate func(spec *semantic.DeckSpec, data []byte) specEvaluation) (specEvaluation, []diagnostics.Diagnostic) {
	var eval specEvaluation
	spec, parseDiags := semantic.Parse(filename, data)
	if spec != nil && !parseDiags.HasErrors() {
		eval = evaluate(spec, data)
	}
	if !eval.Evaluated || eval.CompileFailed {
		if salvaged, ok := salvagedEvaluation(filename, data, strict, evaluate); ok {
			if eval.Evaluated {
				// The authored spec parsed: its own template choice stands.
				salvaged.Choice = eval.Choice
			}
			eval = salvaged
		}
	}
	if eval.Evaluated {
		eval.Diagnostics = collapseDiagnostics(eval.Diagnostics)
		return eval, envelopeDiagnostics(eval.Diagnostics)
	}
	ds := semantic.Check(filename, data, strict)
	enrichSemanticKindDiagnostics(ds)
	promoteProductPlaceholders(ds)
	return eval, ds
}

// promoteProductPlaceholders reports placeholder copy the product itself
// emitted as a blocking error (go-slide-creator-327g6).
func promoteProductPlaceholders(ds []diagnostics.Diagnostic) {
	for i := range ds {
		if isProductPlaceholder(ds[i]) {
			ds[i].Severity = diagnostics.SeverityError
		}
	}
}

// isProductPlaceholder reports whether a diagnostic is the weak-content
// finding for a registered product placeholder.
func isProductPlaceholder(d diagnostics.Diagnostic) bool {
	if d.Code != diagnostics.CodeSemanticWeakContent || d.Details == nil {
		return false
	}
	_, ok := d.Details[semantic.PlaceholderDetail]
	return ok
}

// specFailureResult is the error result of a render refused before it ran: the
// finding envelope validate_deck_spec returns for the same spec.
func specFailureResult(filename string, data []byte, eval specEvaluation, ds []diagnostics.Diagnostic) *mcp.CallToolResult {
	result := api.MCPDiagnosticsError(ds)
	envelope, ok := result.StructuredContent.(diagnostics.FindingEnvelope)
	if !ok {
		return result
	}
	stampEnvelopeFindings(&envelope, eval.Diagnostics)
	for i := range envelope.Findings {
		// A refused render has no deck handle to patch.
		if strings.HasSuffix(envelope.Findings[i].Code, diagnostics.CodeSemanticUnknownKind) {
			continue
		}
		semanticizeFinding(&envelope.Findings[i], data, "")
	}
	shapeEnvelopeFindings(&envelope, eval.Diagnostics, newSpecDoc(filename, data))
	trimEnvelopeForMCP(&envelope)
	result.StructuredContent = envelope
	if text, err := json.Marshal(envelope); err == nil {
		result.Content = []mcp.Content{mcp.TextContent{Type: "text", Text: string(text)}}
	}
	return result
}

// flatSlideIndex is the index a flat "slides[i]…" path names, or nil.
func flatSlideIndex(path string) *int {
	if m := flatSlidePath.FindStringSubmatch(path); m != nil {
		if i, err := strconv.Atoi(m[1]); err == nil {
			return &i
		}
	}
	return nil
}
