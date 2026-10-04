package main

import (
	"context"
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
)

// Committing a call's spec to the deck store, and the read side of the store:
// read-back, find / replace and revision history (go-slide-creator-83kru,
// yxf1k, rq1z9, j77xe).

// deckCommit is what a spec tool asks the store to do with the spec it acted on.
type deckCommit struct {
	Tool     string
	Src      specSource
	Spec     []byte
	Filename string
	Template deckTemplateSource
	// Store is false when the call must leave the deck store alone: a dry run,
	// a spec that does not parse, or a patched render that was refused.
	Store bool
	// RenderedPptx is the artifact a successful render of Spec produced.
	RenderedPptx string
	// AgainstRender measures the change against the last RENDERED revision
	// rather than the stored one: what a render changed is what the agent has
	// not looked at yet.
	AgainstRender bool
	// RenderIdentity is the template the render used when that is not the
	// template the deck is bound to (a one-off template argument on a bound
	// deck_id). The change list and the last-render baseline are then
	// measured on it, so the next render on the bound template reports the
	// slides as restyled rather than unchanged.
	RenderIdentity string
	// Evaluated is the template the call validated or rendered the spec on,
	// in the form of a handle's template identity (a registered name, or a
	// bring-your-own file's path). It is recorded on the revision, so a diff
	// of two revisions shows a template change made by a call's template
	// argument (go-slide-creator-oqu4a). Empty when the call evaluated on no
	// template (explain_deck_spec): the revision then takes the deck's own.
	Evaluated string
}

// deckOutcome is what the store did, and what the call changed.
type deckOutcome struct {
	DeckID   string
	Stored   bool
	Revision int
	// Stale reports a lost race: another call replaced the stored spec while
	// this one ran, so its edit was not stored.
	Stale bool
	// Changes classifies every affected slide; Changed is the subset that
	// looks different (never nil). State is the revision's slide table.
	Changes []slideChange
	Changed []int
	State   *deckState
}

// commitDeck stores (or declines to store) a call's spec and reports the
// handle, revision and per-slide change classes for the response.
func (mc *mcpConfig) commitDeck(c deckCommit) deckOutcome {
	out := deckOutcome{DeckID: c.Src.DeckID, Changed: []int{}}
	source := c.Src.Handle
	var baseline *deckState
	if source != nil {
		out.Revision = source.Revision
		baseline = source.State
		if c.AgainstRender {
			baseline = source.Rendered
		}
	}

	if c.Store {
		id, ok := mc.storeCommit(c)
		if !ok {
			out.Stale = true
			return out
		}
		if live, found := mc.deckHandles.Load(id); found {
			out.DeckID, out.Stored, out.Revision, out.State = id, true, live.Revision, live.State
		}
	}
	if !out.Stored {
		// Not stored: describe the spec the call acted on all the same. The
		// deck_id still holds exactly that spec when the call edited nothing.
		out.State = c.unstoredState()
		out.Stored = source != nil && !c.Src.mutated()
	}
	if source == nil && !c.AgainstRender {
		// A spec seen for the first time has no earlier revision to differ from.
		baseline = out.State
	}
	out.Changes = classifySlideChanges(baseline, out.State.renderedOn(c.RenderIdentity), c.Src.MovedIDs)
	out.Changed = visualSlideIndices(out.Changes)
	return out
}

// storeCommit writes the call's spec to the store: under a new id for a fork
// or a spec seen for the first time, else as the next revision of its deck_id.
// It reports false when another call replaced the stored spec meanwhile.
func (mc *mcpConfig) storeCommit(c deckCommit) (string, bool) {
	source := c.Src.Handle
	h := newDeckHandleFor(c.Spec, c.Filename, c.Template.Template)
	h.TemplatePath, h.BaseDir = c.Template.TemplatePath, c.Template.BaseDir
	if h.TemplatePath != "" {
		h.Template = ""
	}
	h.storeTool, h.pendingRenderPptx, h.storeMoved = c.Tool, c.RenderedPptx, c.Src.MovedIDs
	h.pendingRenderIdentity, h.storeEvaluated = c.RenderIdentity, c.Evaluated
	if c.Src.Restore > 0 {
		h.storeNote = fmt.Sprintf("restored revision %d", c.Src.Restore)
	}
	switch {
	case source != nil && c.Src.Fork:
		// A fork is a new deck that starts as a copy: it keeps the source's
		// template and slide ids, and has its own history.
		h.NextSlideID = source.NextSlideID
		if h.Template == "" && h.TemplatePath == "" {
			h.Template, h.TemplatePath = source.Template, source.TemplatePath
		}
		if h.BaseDir == "" {
			h.BaseDir = source.BaseDir
		}
		from := source.Revision
		if c.Src.Restore > 0 {
			from = c.Src.Restore
		}
		h.storeNote = fmt.Sprintf("fork of %s revision %d", c.Src.DeckID, from)
		return mc.deckHandles.Save(h), true
	case c.Src.DeckID != "":
		if c.Filename == "" {
			// Omitting the filename on an existing deck means "keep its name".
			h.Filename = ""
		}
		return c.Src.DeckID, mc.deckHandles.Update(c.Src.DeckID, c.Src.BaseSpec, h)
	}
	return mc.deckHandles.Save(h), true
}

// unstoredState digests the spec of a call that stored nothing, with the ids
// its new slides would have been given.
func (c deckCommit) unstoredState() *deckState {
	source := c.Src.Handle
	canonical, name := canonicalSpec(c.Filename, c.Spec)
	next := 0
	if source != nil {
		next = source.NextSlideID
	}
	canonical, _ = withSlideIDs(canonical, next)
	identity := c.Template.TemplatePath
	if identity == "" {
		identity = c.Template.Template
	}
	if identity == "" && source != nil {
		identity = source.templateIdentity()
	}
	return specDeckState(name, canonical, identity)
}

// slideIDForPath returns the id of the slide a semantic path (slides[3].title)
// or a 0-based index names, so a finding can be followed by an id-addressed
// patch (go-slide-creator-1w3uo).
func (s *deckState) slideIDForPath(path string) string {
	if s == nil {
		return ""
	}
	m := flatSlidePath.FindStringSubmatch(path)
	if m == nil {
		return ""
	}
	i, err := strconv.Atoi(m[1])
	if err != nil || i < 0 || i >= len(s.Slides) {
		return ""
	}
	return s.Slides[i].ID
}

var flatSlidePath = regexp.MustCompile(`^slides\[(\d+)\]`)

// prefixUnstoredPatch makes a suggested patch call self-contained when the
// call that produced it did not store its own edit: the suggestion's ops
// address the edited spec, so the edit has to travel with them
// (go-slide-creator-j77xe).
func prefixUnstoredPatch(call *patterns.ToolCallSuggestion, src specSource) {
	if call == nil || call.ArgsTemplate == nil {
		return
	}
	ops, ok := call.ArgsTemplate["patch"].([]any)
	if !ok {
		return
	}
	if len(src.RawPatch) > 0 {
		combined := make([]any, 0, len(src.RawPatch)+len(ops))
		combined = append(combined, src.RawPatch...)
		call.ArgsTemplate["patch"] = append(combined, ops...)
	}
	if src.Restore > 0 {
		call.ArgsTemplate["restore"] = src.Restore
	}
}

// --- read-back, find, history ---

// deckFindHit is one occurrence of a find query in a stored spec.
type deckFindHit struct {
	Path    string `json:"path"`
	SlideID string `json:"slide_id,omitempty"`
	Index   *int   `json:"index,omitempty"`
	Excerpt string `json:"excerpt"`
	// Skipped says why a replace left this hit alone.
	Skipped string `json:"skipped,omitempty"`
}

// maxDeckFindHits caps the hits one find reports; hit_count is the full count.
const maxDeckFindHits = 100

// findSkipKeys are fields find and replace leave alone: they select a kind,
// template or composition, and rewriting one as text would break the spec.
var findSkipKeys = map[string]bool{
	"id": true, "kind": true, "type": true, "pattern": true, "layout": true,
	"template": true, "archetype": true,
}

// deckSlidePointers maps each authored slide's JSON Pointer to the slide.
func deckSlidePointers(doc any) map[string]map[string]any {
	out := map[string]map[string]any{}
	root, ok := doc.(map[string]any)
	if !ok {
		return out
	}
	if slides, ok := root["slides"].([]any); ok {
		for i, s := range slides {
			if m, ok := s.(map[string]any); ok {
				out["/slides/"+strconv.Itoa(i)] = m
			}
		}
	}
	st, ok := root["structure"].(map[string]any)
	if !ok {
		return out
	}
	for _, key := range []string{"cover", "closing"} {
		if m, ok := st[key].(map[string]any); ok {
			out["/structure/"+key] = m
		}
	}
	sections, _ := st["sections"].([]any)
	for i, sec := range sections {
		sm, ok := sec.(map[string]any)
		if !ok {
			continue
		}
		slides, _ := sm["slides"].([]any)
		for j, s := range slides {
			if m, ok := s.(map[string]any); ok {
				out[fmt.Sprintf("/structure/sections/%d/slides/%d", i, j)] = m
			}
		}
	}
	return out
}

// findInSpec walks every string and number of a decoded spec and reports the
// ones that match query. With replace non-nil it rewrites them in place and
// returns the equivalent patch ops.
func findInSpec(doc any, state *deckState, query string, replace *string) (hits []deckFindHit, ops []any) {
	f := &specFinder{query: query, replace: replace, slides: deckSlidePointers(doc), indexByID: map[string]int{}}
	if state != nil {
		for i, s := range state.Slides {
			if s.ID != "" {
				f.indexByID[s.ID] = i
			}
		}
	}
	f.queryNumber, f.queryIsNumber = parseFindNumber(query)
	if replace != nil {
		f.replaceNumber, f.replaceIsNumber = parseFindNumber(*replace)
	}
	f.walk(doc, "", func(any) {})
	return f.hits, f.ops
}

// specFinder is one find (or find + replace) pass over a decoded spec.
type specFinder struct {
	query   string
	replace *string

	queryNumber, replaceNumber     float64
	queryIsNumber, replaceIsNumber bool

	slides    map[string]map[string]any // slide pointer → slide
	indexByID map[string]int

	hits []deckFindHit
	ops  []any
}

// walk visits node at pointer; set replaces it in its parent.
func (f *specFinder) walk(node any, pointer string, set func(any)) {
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			if !findSkipKeys[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			f.walk(v[key], pointer+"/"+escapePointerSegment(key), func(nv any) { v[key] = nv })
		}
	case []any:
		for i := range v {
			f.walk(v[i], pointer+"/"+strconv.Itoa(i), func(nv any) { v[i] = nv })
		}
	case string:
		f.visitString(v, pointer, set)
	case float64:
		f.visitNumber(v, pointer, set)
	}
}

func (f *specFinder) visitString(v, pointer string, set func(any)) {
	spans := findSpans(v, f.query)
	if len(spans) == 0 {
		return
	}
	hit := f.hitAt(pointer, findExcerpt(v, spans[0], len(f.query)))
	if f.replace != nil {
		rewritten := replaceSpans(v, spans, len(f.query), *f.replace)
		set(rewritten)
		hit.Excerpt = findExcerpt(rewritten, spans[0], len(*f.replace))
		f.ops = append(f.ops, map[string]any{"op": specPatchReplace, "path": pointer, "value": rewritten})
	}
	f.hits = append(f.hits, hit)
}

func (f *specFinder) visitNumber(v float64, pointer string, set func(any)) {
	if !f.queryIsNumber || v != f.queryNumber {
		return
	}
	hit := f.hitAt(pointer, strconv.FormatFloat(v, 'f', -1, 64))
	switch {
	case f.replace == nil:
	case f.replaceIsNumber:
		set(f.replaceNumber)
		hit.Excerpt = strconv.FormatFloat(f.replaceNumber, 'f', -1, 64)
		f.ops = append(f.ops, map[string]any{"op": specPatchReplace, "path": pointer, "value": f.replaceNumber})
	default:
		hit.Skipped = "a number: the replacement is not numeric"
	}
	f.hits = append(f.hits, hit)
}

// hitAt builds a hit and names the slide the pointer lies in.
func (f *specFinder) hitAt(pointer, excerpt string) deckFindHit {
	hit := deckFindHit{Path: pointer, Excerpt: excerpt}
	for prefix, slide := range f.slides {
		if pointer != prefix && !strings.HasPrefix(pointer, prefix+"/") {
			continue
		}
		if id, ok := slide["id"].(string); ok {
			hit.SlideID = id
			if i, ok := f.indexByID[id]; ok {
				hit.Index = &i
			}
		}
		break
	}
	return hit
}

func escapePointerSegment(seg string) string {
	return strings.ReplaceAll(strings.ReplaceAll(seg, "~", "~0"), "/", "~1")
}

func parseFindNumber(s string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return n, err == nil
}

// findSpans returns the byte offsets where query occurs in text, ignoring
// ASCII case. A query that starts or ends with a digit only matches a whole
// number: "9.4" finds "$9.4m" but not "19.4" or "9.45".
func findSpans(text, query string) []int {
	if query == "" || len(query) > len(text) {
		return nil
	}
	hay, needle := asciiLower(text), asciiLower(query)
	var out []int
	for from := 0; from <= len(hay)-len(needle); {
		i := strings.Index(hay[from:], needle)
		if i < 0 {
			break
		}
		start := from + i
		end := start + len(needle)
		if numberBoundary(hay, needle, start, end) {
			out = append(out, start)
			from = end
		} else {
			from = start + 1
		}
	}
	return out
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func numberBoundary(hay, needle string, start, end int) bool {
	if isDigit(needle[0]) && start > 0 {
		prev := hay[start-1]
		if isDigit(prev) || (prev == '.' && start > 1 && isDigit(hay[start-2])) {
			return false
		}
	}
	if isDigit(needle[len(needle)-1]) && end < len(hay) {
		next := hay[end]
		if isDigit(next) || (next == '.' && end+1 < len(hay) && isDigit(hay[end+1])) {
			return false
		}
	}
	return true
}

func replaceSpans(text string, spans []int, width int, replacement string) string {
	var b strings.Builder
	last := 0
	for _, start := range spans {
		b.WriteString(text[last:start])
		b.WriteString(replacement)
		last = start + width
	}
	b.WriteString(text[last:])
	return b.String()
}

// findExcerpt returns the match with up to 30 bytes of context either side,
// cut on rune boundaries.
func findExcerpt(text string, start, width int) string {
	const context = 30
	from, to := start-context, start+width+context
	prefix, suffix := "…", "…"
	if from <= 0 {
		from, prefix = 0, ""
	}
	if to >= len(text) {
		to, suffix = len(text), ""
	}
	for from > 0 && from < len(text) && !isRuneStart(text[from]) {
		from--
	}
	for to < len(text) && !isRuneStart(text[to]) {
		to++
	}
	return prefix + text[from:to] + suffix
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// deckStoreReadArgs are validate_deck_spec's read-side arguments.
type deckStoreReadArgs struct {
	Read    string
	Find    string
	Replace *string
}

func parseDeckStoreReadArgs(tool string, request mcp.CallToolRequest) (deckStoreReadArgs, *mcp.CallToolResult) {
	var a deckStoreReadArgs
	var errRes *mcp.CallToolResult
	if a.Read, _, errRes = semanticOptionalString(tool, "read", request); errRes != nil {
		return a, errRes
	}
	if a.Find, _, errRes = semanticOptionalString(tool, "find", request); errRes != nil {
		return a, errRes
	}
	replace, present, errRes := semanticOptionalString(tool, "replace", request)
	if errRes != nil {
		return a, errRes
	}
	if present {
		if a.Find == "" {
			return a, argInvalidValue(tool, diagnostics.CodeMissingParameter, "find",
				"replace rewrites what find locates, so it needs find", "string", "9.4", nil)
		}
		a.Replace = &replace
	}
	if a.Read != "" && a.Replace != nil {
		return a, argInvalidValue(tool, diagnostics.CodeAmbiguousInput, "read",
			"read returns the stored deck without changing it, so it cannot be combined with replace; run the replace first", "string", nil, nil)
	}
	return a, nil
}

// deckSpecReadResult answers the read-only forms of validate_deck_spec: read
// back the spec, one slide, or the history, or list where a query occurs. It
// reports handled=false when the call is an ordinary validation.
func (mc *mcpConfig) deckSpecReadResult(ctx context.Context, tool string, a deckStoreReadArgs, src specSource) (*mcp.CallToolResult, bool) {
	if a.Read == "" && (a.Find == "" || a.Replace != nil) {
		return nil, false
	}
	canonical, name := canonicalSpec(src.Filename, src.Data)
	next := 0
	if src.Handle != nil {
		next = src.Handle.NextSlideID
	}
	canonical, _ = withSlideIDs(canonical, next)
	var doc any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return argInvalidValue(tool, diagnostics.CodeInvalidParameter, "spec",
			fmt.Sprintf("the spec could not be decoded: %v", err), "object|string", nil, nil), true
	}
	state := specDeckState(name, canonical, src.Template)

	resp := deckSpecEnvelopeResponse{
		FindingEnvelope: diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  tool,
			InputSHA256: diagnostics.ComputeInputSHA256(src.Data),
		}, nil),
		DeckID:        src.DeckID,
		Stored:        src.DeckID != "" && !src.mutated(),
		ChangedSlides: []int{},
	}
	if src.Handle != nil {
		resp.Revision = src.Handle.Revision
		if src.Restore > 0 {
			resp.Revision = src.Restore
		}
	}

	switch {
	case a.Find != "":
		hits, _ := findInSpec(doc, state, a.Find, nil)
		resp.setHits(hits)
		resp.Summary = fmt.Sprintf("%d occurrence(s) of %q", len(hits), a.Find)
	case a.Read == "spec":
		resp.Spec = canonical
		resp.Summary = "stored spec"
	case a.Read == "history":
		if src.Handle == nil {
			return argInvalidValue(tool, diagnostics.CodeInvalidParameter, "read",
				"history is kept per deck_id; send the deck_id a previous call returned", "string", "history", nextCallRetry(tool, "deck_id")), true
		}
		resp.Revisions, resp.Slides = src.Handle.history()
		resp.Summary = fmt.Sprintf("%d revision(s); current is %d", len(resp.Revisions), src.Handle.Revision)
	case strings.HasPrefix(a.Read, revisionDiffPrefix):
		// Any two kept revisions, not only neighbours (go-slide-creator-rq1z9).
		if src.Handle == nil {
			return argInvalidValue(tool, diagnostics.CodeInvalidParameter, "read",
				"revisions are kept per deck_id; send the deck_id a previous call returned", "string", "diff:1..2", nextCallRetry(tool, "deck_id")), true
		}
		from, to, err := parseRevisionDiff(a.Read, src.Handle.Revision)
		if err == nil {
			resp.Diff, err = src.Handle.diffRevisions(from, to)
		}
		if err != nil {
			return argInvalidValue(tool, diagnostics.CodeInvalidParameter, "read", err.Error(), "string", "diff:1..2", nil), true
		}
		resp.ChangedSlides = visualSlideIndices(resp.Diff.Changes)
		resp.Summary = resp.Diff.summary()
	default:
		slide, ref, err := readStoredSlide(doc, state, a.Read)
		if err != nil {
			return argInvalidValue(tool, diagnostics.CodeInvalidParameter, "read", err.Error(), "string", "spec", nil), true
		}
		resp.Slide, resp.SlideRef = slide, ref
		resp.Summary = "stored slide " + a.Read
	}
	res, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal %s response: %v", tool, err)), true
	}
	return res, true
}

// setHits attaches find results, capped at maxDeckFindHits.
func (r *deckSpecEnvelopeResponse) setHits(hits []deckFindHit) {
	n := len(hits)
	r.HitCount = &n
	if n > maxDeckFindHits {
		hits = hits[:maxDeckFindHits]
	}
	if hits == nil {
		hits = []deckFindHit{}
	}
	r.Hits = &hits
}

// readStoredSlide returns one authored slide, named by id or by 0-based index
// into the rendered deck.
func readStoredSlide(doc any, state *deckState, which string) (json.RawMessage, *slideRef, error) {
	slides := authoredSlides(doc)
	var ids []string
	target := ""
	if i, err := strconv.Atoi(which); err == nil {
		if state == nil || i < 0 || i >= len(state.Slides) {
			n := 0
			if state != nil {
				n = len(state.Slides)
			}
			return nil, nil, fmt.Errorf("read: slide index %d is outside the deck (%d slides)", i, n)
		}
		if target = state.Slides[i].ID; target == "" {
			return nil, nil, fmt.Errorf("read: slide index %d is generated from the structure (%s) and has no stored slide; read \"spec\" instead", i, strings.TrimPrefix(state.Slides[i].Key, "@"))
		}
	} else {
		target = which
	}
	for _, s := range slides {
		id, _ := s["id"].(string)
		if id == "" {
			continue
		}
		if id != target {
			ids = append(ids, id)
			continue
		}
		raw, err := json.Marshal(s)
		if err != nil {
			return nil, nil, fmt.Errorf("read: slide %q could not be encoded: %v", which, err)
		}
		ref := &slideRef{ID: id}
		if state != nil {
			for i, st := range state.Slides {
				if st.ID == id {
					ref.Index, ref.SlideNumber, ref.Kind = i, i+1, st.Kind
				}
			}
		}
		return raw, ref, nil
	}
	return nil, nil, fmt.Errorf("read must be \"spec\", \"history\", \"diff:A..B\", or a slide id or 0-based index; no slide has id %q (ids: %s)", which, strings.Join(ids, ", "))
}
