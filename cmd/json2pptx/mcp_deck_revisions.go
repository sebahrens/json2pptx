package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// Revision machinery for a stored DeckSpec (agent-journey-20261003, e-revise).
//
// Eight revision turns on one deck_id showed what a handle that only stores
// bytes cannot answer: which slide is "the risk slide" after two inserts
// (go-slide-creator-1w3uo), which slides LOOK different after a move or a
// notes edit (v5e9h), what the deck was three patches ago (rq1z9), and whether
// a refused render kept the patch (j77xe). A handle therefore keeps, next to
// its spec: a stable id per slide, a digest snapshot of the current and the
// last rendered revision, and a bounded list of earlier revisions.

// maxDeckRevisions bounds the history one handle keeps. A revision session is
// tens of patches; the cap only stops a runaway loop from pinning memory.
const maxDeckRevisions = 50

// Slide change classes (go-slide-creator-v5e9h). The first three change what a
// slide looks like; the rest do not, so their thumbnails need no second look.
const (
	slideChangeEdited     = "edited"     // visible content differs
	slideChangeInserted   = "inserted"   // new since the baseline (every slide on a first render)
	slideChangeRestyled   = "restyled"   // same content, deck-level settings or template changed
	slideChangeMoved      = "moved"      // same content, reordered relative to its neighbours
	slideChangeRenumbered = "renumbered" // same content and order, index shifted by an insert or removal
	slideChangeNotesOnly  = "notes_only" // only speaker notes differ
	slideChangeRemoved    = "removed"    // gone since the baseline; carries no index
)

// slideChange is one row of slide_changes.
type slideChange struct {
	ID          string `json:"id,omitempty"`
	Index       *int   `json:"index,omitempty"`
	SlideNumber int    `json:"slide_number,omitempty"`
	Change      string `json:"change"`
	WasIndex    *int   `json:"was_index,omitempty"`
	// Fields names the slide's top-level fields that differ, on an edited
	// slide of a revision diff (read "diff:A..B").
	Fields []string `json:"fields,omitempty"`
}

// visual reports whether the change alters the rendered slide.
func (c slideChange) visual() bool {
	switch c.Change {
	case slideChangeEdited, slideChangeInserted, slideChangeRestyled:
		return true
	}
	return false
}

// slideRef is one row of a response's slides table of contents.
type slideRef struct {
	ID          string `json:"id,omitempty"`
	Index       int    `json:"index"`
	SlideNumber int    `json:"slide_number"`
	Kind        string `json:"kind,omitempty"`
}

// slideState is the identity and content digests of one rendered slide.
type slideState struct {
	// Key identifies the slide across revisions: its id, or "@<source path>"
	// for a generated agenda or divider slide, which has no id of its own.
	Key    string
	ID     string
	Kind   string
	Title  string
	Visual string // digest of everything that renders
	Notes  string // digest of the speaker notes
}

// deckState is the digest snapshot of one spec revision, in rendered order.
type deckState struct {
	Slides []slideState
	// DeckLevel digests the settings every slide inherits (meta, structure
	// options); Template is the template the revision resolved to.
	DeckLevel string
	Template  string
}

// deckRevision is one stored revision of a handle's spec.
type deckRevision struct {
	Number  int
	Time    time.Time
	Tool    string
	Note    string
	Spec    []byte
	Changes []slideChange
}

// --- slide ids ---

// authoredSlides returns the slide objects of a decoded spec, flat or
// structured, in authored order. The maps alias the document.
func authoredSlides(doc any) []map[string]any {
	root, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	var out []map[string]any
	add := func(v any) {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	if slides, ok := root["slides"].([]any); ok {
		for _, s := range slides {
			add(s)
		}
	}
	if st, ok := root["structure"].(map[string]any); ok {
		add(st["cover"])
		if sections, ok := st["sections"].([]any); ok {
			for _, sec := range sections {
				if sm, ok := sec.(map[string]any); ok {
					if slides, ok := sm["slides"].([]any); ok {
						for _, s := range slides {
							add(s)
						}
					}
				}
			}
		}
		add(st["closing"])
	}
	return out
}

var assignedSlideID = regexp.MustCompile(`^s([0-9]{1,9})$`)

// assignSlideIDs gives every authored slide without an id the next free
// "s<N>". next is the handle's counter: it only moves forward, so an id is
// never handed out twice in one deck, even after its slide was removed.
func assignSlideIDs(doc any, next int) int {
	slides := authoredSlides(doc)
	used := make(map[string]bool, len(slides))
	for _, s := range slides {
		id, ok := s["id"].(string)
		if !ok {
			continue
		}
		used[id] = true
		if m := assignedSlideID.FindStringSubmatch(id); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n >= next {
				next = n + 1
			}
		}
	}
	if next < 1 {
		next = 1
	}
	for _, s := range slides {
		if _, has := s["id"]; has {
			continue
		}
		for used["s"+strconv.Itoa(next)] {
			next++
		}
		id := "s" + strconv.Itoa(next)
		s["id"] = id
		used[id] = true
		next++
	}
	return next
}

// duplicateSlideID names the first id two slides of a decoded spec share.
func duplicateSlideID(doc any) (string, bool) {
	seen := map[string]bool{}
	for _, s := range authoredSlides(doc) {
		id, ok := s["id"].(string)
		if !ok {
			continue
		}
		if seen[id] {
			return id, true
		}
		seen[id] = true
	}
	return "", false
}

// withSlideIDs returns spec with ids assigned, plus the advanced counter. A
// spec that is not a JSON object is returned unchanged.
func withSlideIDs(spec []byte, next int) ([]byte, int) {
	var doc any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return spec, next
	}
	if _, ok := doc.(map[string]any); !ok {
		return spec, next
	}
	next = assignSlideIDs(doc, next)
	out, err := json.Marshal(doc)
	if err != nil {
		return spec, next
	}
	return out, next
}

// specWithoutSlideIDs strips slide ids, for digests that must not depend on
// them (an id never renders).
func specWithoutSlideIDs(spec []byte) []byte {
	var doc any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return spec
	}
	slides := authoredSlides(doc)
	stripped := false
	for _, s := range slides {
		if _, has := s["id"]; has {
			delete(s, "id")
			stripped = true
		}
	}
	if !stripped {
		return spec
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return spec
	}
	return out
}

// --- state snapshots ---

// specDeckState digests a spec revision slide by slide. It returns nil for a
// spec that does not parse: there is nothing to compare.
func specDeckState(filename string, spec []byte, template string) *deckState {
	parsed, diags := semantic.Parse(filename, spec)
	if parsed == nil || diags.HasErrors() {
		return nil
	}
	chromeSections := parsed.Meta.Chrome != nil && (parsed.Meta.Chrome.Tracker || parsed.Meta.Chrome.SectionCrumb)
	expanded := semantic.ExpandedSlideSources(parsed)
	state := &deckState{
		Slides:    make([]slideState, 0, len(expanded)),
		DeckLevel: deckSettingsDigest(spec),
		Template:  template,
	}
	section, chapter := "", 0
	for _, e := range expanded {
		body := make(map[string]any, len(e.Slide.Body))
		notes := map[string]any{}
		for k, v := range e.Slide.Body {
			switch k {
			case "id":
			case "notes", "speaker_notes":
				notes[k] = v
			default:
				body[k] = v
			}
		}
		visual := map[string]any{"kind": string(e.Slide.Kind), "body": body}
		// Neighbours a slide's own fields do not show: a divider's chapter
		// number, and the running section name deck chrome prints on the
		// slides that follow it.
		if e.Slide.Kind == semantic.KindSection {
			if appendix, _ := e.Slide.Body["appendix"].(bool); !appendix {
				chapter++
				visual["chapter"] = chapter
			}
			section = e.Slide.String("title")
		} else if chromeSections {
			visual["section"] = section
		}
		id, _ := e.Slide.Body["id"].(string)
		key := id
		if key == "" {
			key = "@" + e.SourcePath
		}
		state.Slides = append(state.Slides, slideState{
			Key:    key,
			ID:     id,
			Kind:   string(e.Slide.Kind),
			Title:  e.Slide.String("title"),
			Visual: digestJSON(visual),
			Notes:  digestJSON(notes),
		})
	}
	return state
}

func digestJSON(v any) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return diagnostics.ComputeInputSHA256(encoded)
}

// deckSettingsDigest digests the deck-level settings of a spec: everything
// except the slides themselves. In the structured form the cover, closing and
// section contents are slides too (each tracked on its own), so only the
// structure's options remain.
func deckSettingsDigest(spec []byte) string {
	var doc map[string]any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return ""
	}
	delete(doc, "slides")
	if st, ok := doc["structure"].(map[string]any); ok {
		options := make(map[string]any, len(st))
		for k, v := range st {
			switch k {
			case "cover", "closing", "sections":
			default:
				options[k] = v
			}
		}
		doc["structure"] = options
	}
	return digestJSON(doc)
}

// refs lists a state as the slides table of contents.
func (s *deckState) refs() []slideRef {
	if s == nil {
		return nil
	}
	out := make([]slideRef, len(s.Slides))
	for i, sl := range s.Slides {
		out[i] = slideRef{ID: sl.ID, Index: i, SlideNumber: i + 1, Kind: sl.Kind}
	}
	return out
}

// classifySlideChanges compares a revision with a baseline and says, per
// slide, what kind of change it had. A nil baseline means nothing was seen
// yet: every slide is new. Unchanged slides are not listed.
//
// movedHint names the slides a move op picked up. A reorder is ambiguous on
// its own — swapping two neighbours moved either one — so the slide the patch
// named is the one reported as moved, and the slides it passed as renumbered.
func classifySlideChanges(before, after *deckState, movedHint map[string]bool) []slideChange {
	if after == nil {
		return nil
	}
	intPtr := func(i int) *int { return &i }
	out := make([]slideChange, 0)
	if before == nil {
		for i, s := range after.Slides {
			out = append(out, slideChange{ID: s.ID, Index: intPtr(i), SlideNumber: i + 1, Change: slideChangeInserted})
		}
		return out
	}
	was := make(map[string]int, len(before.Slides))
	for i, s := range before.Slides {
		was[s.Key] = i
	}
	restyled := before.DeckLevel != after.DeckLevel || before.Template != after.Template
	inOrder := slidesInOrder(before, after, was, movedHint)

	now := make(map[string]bool, len(after.Slides))
	for i, s := range after.Slides {
		now[s.Key] = true
		j, existed := was[s.Key]
		change := slideChangeInserted
		if existed {
			change = survivingSlideChange(before.Slides[j], s, restyled, inOrder[j], j != i)
		}
		if change == "" {
			continue
		}
		c := slideChange{ID: s.ID, Index: intPtr(i), SlideNumber: i + 1, Change: change}
		if existed && j != i {
			c.WasIndex = intPtr(j)
		}
		out = append(out, c)
	}
	for j, s := range before.Slides {
		if !now[s.Key] {
			out = append(out, slideChange{ID: s.ID, Change: slideChangeRemoved, WasIndex: intPtr(j)})
		}
	}
	return out
}

// survivingSlideChange classifies a slide present in both revisions; "" means
// it did not change. The classes are ordered by what the agent must do: a
// visible change outranks a positional one.
func survivingSlideChange(before, after slideState, restyled, inOrder, shifted bool) string {
	switch {
	case before.Visual != after.Visual:
		return slideChangeEdited
	case restyled:
		return slideChangeRestyled
	case !inOrder:
		return slideChangeMoved
	case before.Notes != after.Notes:
		return slideChangeNotesOnly
	case shifted:
		return slideChangeRenumbered
	}
	return ""
}

// slidesInOrder returns the baseline indices of the slides that kept their
// relative order: the longest increasing run of baseline indices in the new
// order. The others moved.
func slidesInOrder(before, after *deckState, was map[string]int, movedHint map[string]bool) map[int]bool {
	var common, unhinted []int // baseline index of each surviving slide, in new order
	hinted := map[int]bool{}
	for _, s := range after.Slides {
		j, ok := was[s.Key]
		if !ok {
			continue
		}
		common = append(common, j)
		if movedHint[s.ID] {
			hinted[j] = true
		} else {
			unhinted = append(unhinted, j)
		}
	}
	if len(hinted) == 0 {
		return longestIncreasing(common)
	}
	// Keep the un-hinted slides' order first; a hinted slide stayed in order
	// only if it still sits between the same neighbours.
	inOrder := longestIncreasing(unhinted)
	prev := -1
	for i, j := range common {
		if inOrder[j] {
			prev = j
			continue
		}
		if !hinted[j] {
			continue
		}
		next := len(before.Slides)
		for _, k := range common[i+1:] {
			if inOrder[k] {
				next = k
				break
			}
		}
		if prev < j && j < next {
			inOrder[j] = true
			prev = j
		}
	}
	return inOrder
}

// longestIncreasing returns the members of one longest strictly increasing
// subsequence of seq, as a set.
func longestIncreasing(seq []int) map[int]bool {
	n := len(seq)
	length := make([]int, n)
	prev := make([]int, n)
	best, bestEnd := 0, -1
	for i := 0; i < n; i++ {
		length[i], prev[i] = 1, -1
		for j := 0; j < i; j++ {
			if seq[j] < seq[i] && length[j]+1 > length[i] {
				length[i], prev[i] = length[j]+1, j
			}
		}
		if length[i] > best {
			best, bestEnd = length[i], i
		}
	}
	out := make(map[int]bool, best)
	for i := bestEnd; i >= 0; i = prev[i] {
		out[seq[i]] = true
	}
	return out
}

// visualSlideIndices returns the indices changed_slides reports: the slides
// whose rendered image differs from the baseline. Never nil, so the key is
// always present in a response (go-slide-creator-v5e9h).
func visualSlideIndices(changes []slideChange) []int {
	out := make([]int, 0, len(changes))
	for _, c := range changes {
		if c.visual() && c.Index != nil {
			out = append(out, *c.Index)
		}
	}
	sort.Ints(out)
	return out
}

// --- handle history ---

// inherit carries a handle's history across a store: slide ids, the revision
// list and the last-render snapshot. old is nil for a new handle. tool and
// note label the revision a changed spec creates.
func (h *deckHandle) inherit(old *deckHandle, tool, note string, now time.Time) {
	if h.RawPresentation != nil {
		return
	}
	next := h.NextSlideID
	if old != nil {
		if old.NextSlideID > next {
			next = old.NextSlideID
		}
		h.Revision = old.Revision
		h.Revisions = old.Revisions
		h.Rendered, h.RenderedPptx = old.Rendered, old.RenderedPptx
	}
	h.Spec, h.NextSlideID = withSlideIDs(h.Spec, next)
	h.SlideDigests = slideDigests(h.Spec)
	h.State = specDeckState(h.Filename, h.Spec, h.templateIdentity())
	if old != nil && string(old.Spec) == string(h.Spec) {
		h.applyPendingRender()
		return
	}
	var before *deckState
	if old != nil {
		before = old.State
	}
	h.Revision++
	rev := deckRevision{
		Number: h.Revision, Time: now, Tool: tool, Note: note,
		Spec:    h.Spec,
		Changes: classifySlideChanges(before, h.State, h.storeMoved),
	}
	revisions := make([]deckRevision, 0, len(h.Revisions)+1)
	revisions = append(revisions, h.Revisions...)
	revisions = append(revisions, rev)
	if len(revisions) > maxDeckRevisions {
		revisions = revisions[len(revisions)-maxDeckRevisions:]
	}
	h.Revisions = revisions
	h.applyPendingRender()
}

// applyPendingRender records a successful render of this revision as the
// baseline the next render's changed_slides is measured against.
func (h *deckHandle) applyPendingRender() {
	if h.pendingRenderPptx == "" {
		return
	}
	h.Rendered, h.RenderedPptx = h.State.renderedOn(h.pendingRenderIdentity), h.pendingRenderPptx
	h.pendingRenderPptx, h.pendingRenderIdentity = "", ""
}

// renderedOn returns the state as it looks when rendered on template: the
// state itself for "" or its own template, else a copy carrying that template.
// A deck_id rendered once on a template it is not bound to keeps its binding,
// but what the agent last saw is that other template (go-slide-creator-2dit4).
func (s *deckState) renderedOn(template string) *deckState {
	if s == nil || template == "" || template == s.Template {
		return s
	}
	shown := *s
	shown.Template = template
	return &shown
}

func (h *deckHandle) templateIdentity() string {
	if h.TemplatePath != "" {
		return h.TemplatePath
	}
	return h.Template
}

// revision returns a kept revision by number.
func (h *deckHandle) revision(n int) (deckRevision, bool) {
	for _, r := range h.Revisions {
		if r.Number == n {
			return r, true
		}
	}
	return deckRevision{}, false
}

// keptRevisions describes which revision numbers a handle still holds.
func (h *deckHandle) keptRevisions() string {
	if len(h.Revisions) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d–%d", h.Revisions[0].Number, h.Revisions[len(h.Revisions)-1].Number)
}

// --- history report ---

// deckRevisionEntry is one row of a history read.
type deckRevisionEntry struct {
	Revision int           `json:"revision"`
	Time     string        `json:"time"`
	Tool     string        `json:"tool,omitempty"`
	Note     string        `json:"note,omitempty"`
	Changes  []slideChange `json:"changes"`
}

// deckSlideHistory is a slide's row in a history read: where it is now and the
// last revision that changed it.
type deckSlideHistory struct {
	ID          string `json:"id,omitempty"`
	Index       int    `json:"index"`
	SlideNumber int    `json:"slide_number"`
	Kind        string `json:"kind,omitempty"`
	Title       string `json:"title,omitempty"`
	// LastChanged is the latest revision in which this slide's content,
	// notes, style or position changed; LastChange says which.
	LastChanged int    `json:"last_changed"`
	LastChange  string `json:"last_change"`
}

// history renders the handle's revision list and per-slide last change.
func (h *deckHandle) history() ([]deckRevisionEntry, []deckSlideHistory) {
	revisions := make([]deckRevisionEntry, 0, len(h.Revisions))
	type last struct {
		rev    int
		change string
	}
	lastByKey := map[string]last{}
	for _, r := range h.Revisions {
		// A history row names slides by id; its indices are those of that
		// revision, not of the current deck.
		changes := append([]slideChange{}, r.Changes...)
		for _, c := range r.Changes {
			if c.ID != "" && c.Change != slideChangeRenumbered && c.Change != slideChangeRemoved {
				lastByKey[c.ID] = last{r.Number, c.Change}
			}
		}
		revisions = append(revisions, deckRevisionEntry{
			Revision: r.Number, Time: r.Time.UTC().Format(time.RFC3339), Tool: r.Tool, Note: r.Note, Changes: changes,
		})
	}
	var slides []deckSlideHistory
	if h.State != nil {
		slides = make([]deckSlideHistory, 0, len(h.State.Slides))
		for i, s := range h.State.Slides {
			row := deckSlideHistory{ID: s.ID, Index: i, SlideNumber: i + 1, Kind: s.Kind, Title: s.Title}
			if l, ok := lastByKey[s.ID]; ok && s.ID != "" {
				row.LastChanged, row.LastChange = l.rev, l.change
			}
			slides = append(slides, row)
		}
	}
	return revisions, slides
}

// --- diff of two revisions ---

// deckRevisionDiff is what read "diff:A..B" answers: how revision To differs
// from revision From, slide by slide. Indices are those of To; a removed slide
// carries only the index it had in From (go-slide-creator-rq1z9).
type deckRevisionDiff struct {
	From    int           `json:"from"`
	To      int           `json:"to"`
	Changes []slideChange `json:"changes"`
}

// revisionDiffPrefix starts the read value that asks for a diff.
const revisionDiffPrefix = "diff:"

var revisionDiffSpec = regexp.MustCompile(`^diff:\s*([0-9]{1,6})\s*(?:\.\.\s*([0-9]{1,6})\s*)?$`)

// parseRevisionDiff reads "diff:A..B", or "diff:A" for A against the current
// revision.
func parseRevisionDiff(read string, current int) (from, to int, err error) {
	m := revisionDiffSpec.FindStringSubmatch(read)
	if m == nil {
		return 0, 0, fmt.Errorf("read %q is not a revision diff; write \"diff:A..B\" with two revision numbers from read \"history\" (\"diff:A\" compares A with the current revision)", read)
	}
	from, _ = strconv.Atoi(m[1])
	to = current
	if m[2] != "" {
		to, _ = strconv.Atoi(m[2])
	}
	return from, to, nil
}

// diffRevisions compares two kept revisions of the handle. The comparison is
// the one a store makes between neighbouring revisions, so a diff of N-1..N
// reads the same as revision N's history row.
func (h *deckHandle) diffRevisions(from, to int) (*deckRevisionDiff, error) {
	a, okA := h.revision(from)
	b, okB := h.revision(to)
	for _, miss := range []struct {
		n  int
		ok bool
	}{{from, okA}, {to, okB}} {
		if !miss.ok {
			return nil, fmt.Errorf("revision %d is not kept for this deck (kept: %s; current: %d)", miss.n, h.keptRevisions(), h.Revision)
		}
	}
	template := h.templateIdentity()
	before := specDeckState(h.Filename, a.Spec, template)
	after := specDeckState(h.Filename, b.Spec, template)
	if before == nil || after == nil {
		return nil, fmt.Errorf("revision %d or %d does not parse as a DeckSpec, so the two cannot be compared", from, to)
	}
	changes := classifySlideChanges(before, after, nil)
	was, now := slidesByID(a.Spec), slidesByID(b.Spec)
	for i := range changes {
		if c := &changes[i]; c.Change == slideChangeEdited && c.ID != "" {
			c.Fields = changedSlideFields(was[c.ID], now[c.ID])
		}
	}
	return &deckRevisionDiff{From: from, To: to, Changes: changes}, nil
}

// slidesByID indexes a spec's authored slides by id.
func slidesByID(spec []byte) map[string]map[string]any {
	var doc any
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil
	}
	out := map[string]map[string]any{}
	for _, s := range authoredSlides(doc) {
		if id, ok := s["id"].(string); ok {
			out[id] = s
		}
	}
	return out
}

// changedSlideFields names the top-level fields whose value differs between
// two versions of a slide, sorted.
func changedSlideFields(before, after map[string]any) []string {
	var out []string
	for k, v := range after {
		old, had := before[k]
		if !had || !sameJSONValue(old, v) {
			out = append(out, k)
		}
	}
	for k := range before {
		if _, kept := after[k]; !kept {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func sameJSONValue(a, b any) bool {
	x, errA := json.Marshal(a)
	y, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(x, y)
}

// summary counts a diff's changes by class, in a fixed order.
func (d *deckRevisionDiff) summary() string {
	if len(d.Changes) == 0 {
		return fmt.Sprintf("revision %d and revision %d are identical", d.From, d.To)
	}
	counts := map[string]int{}
	for _, c := range d.Changes {
		counts[c.Change]++
	}
	var parts []string
	for _, class := range []string{slideChangeEdited, slideChangeInserted, slideChangeRemoved, slideChangeRestyled, slideChangeMoved, slideChangeNotesOnly, slideChangeRenumbered} {
		if n := counts[class]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, class))
		}
	}
	return fmt.Sprintf("revision %d → %d: %s", d.From, d.To, strings.Join(parts, ", "))
}
