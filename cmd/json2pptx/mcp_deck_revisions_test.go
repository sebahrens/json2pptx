package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// revisionTestSpec is a four-slide deck with no section dividers, so a reorder
// changes nothing but positions.
const revisionTestSpec = `{
  "meta": {"title": "Quarterly Review", "template": "midnight-blue"},
  "slides": [
    {"kind": "title", "title": "Quarterly Review", "subtitle": "FY26 Q2"},
    {"kind": "kpi_snapshot", "title": "Growth held at 42%", "kpis": [{"value": "42%", "label": "Growth"}, {"value": "1.2M", "label": "ARR"}]},
    {"id": "costs", "kind": "kpi_snapshot", "title": "Costs fell 8%", "kpis": [{"value": "8%", "label": "Cost cut"}, {"value": "3.1M", "label": "Opex"}]},
    {"kind": "closing", "title": "Questions?"}
  ]
}`

// storedSlideIDs reads a deck back and returns its slide ids in order.
func storedSlideIDs(t *testing.T, mc *mcpConfig, deckID string) []string {
	t.Helper()
	got := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": deckID, "read": "spec"}))
	var spec struct {
		Slides []struct {
			ID string `json:"id"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(got.Spec, &spec); err != nil {
		t.Fatalf("decode read-back spec: %v", err)
	}
	ids := make([]string, len(spec.Slides))
	for i, s := range spec.Slides {
		ids[i] = s.ID
	}
	return ids
}

func patchValidate(t *testing.T, mc *mcpConfig, deckID string, ops ...any) deckSpecEnvelopeResponse {
	t.Helper()
	return deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": deckID, "patch": ops}))
}

// go-slide-creator-1w3uo: every stored slide has an id — the authored one, or
// one assigned on first store — and the id survives insert, remove and move.
func TestSlideIDsAreStableAcrossPatches(t *testing.T) {
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revisionTestSpec}))
	if got := strings.Join(storedSlideIDs(t, mc, first.DeckID), ","); got != "s1,s2,costs,s3" {
		t.Fatalf("ids after first store = %s, want s1,s2,costs,s3 (authored id kept, the rest assigned)", got)
	}

	// Insert before a slide named by id; the new slide gets the next id.
	newSlide := map[string]any{"kind": "kpi_snapshot", "title": "Churn is flat", "kpis": []any{map[string]any{"value": "2%", "label": "Churn"}, map[string]any{"value": "0", "label": "Change"}}}
	inserted := patchValidate(t, mc, first.DeckID, map[string]any{"op": "add", "path": "/slides/costs", "value": newSlide})
	if got := strings.Join(storedSlideIDs(t, mc, first.DeckID), ","); got != "s1,s2,s4,costs,s3" {
		t.Fatalf("ids after insert = %s, want s1,s2,s4,costs,s3", got)
	}
	if !equalInts(inserted.ChangedSlides, []int{2}) || changeOf(inserted.SlideChanges, "costs") != slideChangeRenumbered {
		t.Errorf("insert: changed_slides=%v slide_changes=%+v; want [2] and costs renumbered", inserted.ChangedSlides, inserted.SlideChanges)
	}

	// The id still addresses the slide after the shift; so does its new index.
	byID := patchValidate(t, mc, first.DeckID, map[string]any{"op": "replace", "path": "/slides/costs/title", "value": "Costs fell 9%"})
	if !equalInts(byID.ChangedSlides, []int{3}) {
		t.Errorf("patch by id: changed_slides=%v, want [3]", byID.ChangedSlides)
	}
	byIndex := patchValidate(t, mc, first.DeckID, map[string]any{"op": "replace", "path": "/slides/3/title", "value": "Costs fell 10%"})
	if !equalInts(byIndex.ChangedSlides, []int{3}) || changeOf(byIndex.SlideChanges, "costs") != slideChangeEdited {
		t.Errorf("patch by index: changed_slides=%v slide_changes=%+v", byIndex.ChangedSlides, byIndex.SlideChanges)
	}

	// Remove a slide: its id is gone for good, the next new slide gets a new one.
	removed := patchValidate(t, mc, first.DeckID, map[string]any{"op": "remove", "path": "/slides/s4"})
	if changeOf(removed.SlideChanges, "s4") != slideChangeRemoved || len(removed.ChangedSlides) != 0 {
		t.Errorf("remove: changed_slides=%v slide_changes=%+v; want none visible and s4 removed", removed.ChangedSlides, removed.SlideChanges)
	}
	patchValidate(t, mc, first.DeckID, map[string]any{"op": "add", "path": "/slides/-", "value": newSlide})
	if got := strings.Join(storedSlideIDs(t, mc, first.DeckID), ","); got != "s1,s2,costs,s3,s5" {
		t.Errorf("ids after remove + append = %s, want s1,s2,costs,s3,s5 (s4 is not reused)", got)
	}

	// Replacing a whole slide keeps its handle.
	rewritten := patchValidate(t, mc, first.DeckID, map[string]any{"op": "replace", "path": "/slides/1", "value": newSlide})
	if changeOf(rewritten.SlideChanges, "s2") != slideChangeEdited {
		t.Errorf("replace /slides/1 should edit s2, got %+v", rewritten.SlideChanges)
	}
}

// go-slide-creator-83kru: move and copy, on slides and on fields.
func TestSpecPatchMoveAndCopy(t *testing.T) {
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revisionTestSpec}))
	id := first.DeckID

	move := map[string]any{"op": "move", "from": "/slides/costs", "path": "/slides/1"}
	if raw, _ := json.Marshal(move); len(raw) >= 100 {
		t.Errorf("a slide move is %d bytes, want < 100", len(raw))
	}
	moved := patchValidate(t, mc, id, move)
	if got := strings.Join(storedSlideIDs(t, mc, id), ","); got != "s1,costs,s2,s3" {
		t.Fatalf("ids after move = %s, want s1,costs,s2,s3", got)
	}
	if len(moved.ChangedSlides) != 0 || changeOf(moved.SlideChanges, "costs") != slideChangeMoved || changeOf(moved.SlideChanges, "s2") != slideChangeRenumbered {
		t.Errorf("move: changed_slides=%v slide_changes=%+v; want nothing visible, costs moved, s2 renumbered", moved.ChangedSlides, moved.SlideChanges)
	}

	// Move by index, RFC 6902 style: the path is read after the removal.
	patchValidate(t, mc, id, map[string]any{"op": "move", "from": "/slides/1", "path": "/slides/2"})
	if got := strings.Join(storedSlideIDs(t, mc, id), ","); got != "s1,s2,costs,s3" {
		t.Fatalf("ids after index move = %s, want s1,s2,costs,s3", got)
	}

	// Copy a slide: the copy is a new slide with its own id.
	copied := patchValidate(t, mc, id, map[string]any{"op": "copy", "from": "/slides/costs", "path": "/slides/-"})
	if got := strings.Join(storedSlideIDs(t, mc, id), ","); got != "s1,s2,costs,s3,s4" {
		t.Fatalf("ids after copy = %s, want s1,s2,costs,s3,s4", got)
	}
	if !equalInts(copied.ChangedSlides, []int{4}) || changeOf(copied.SlideChanges, "s4") != slideChangeInserted {
		t.Errorf("copy: changed_slides=%v slide_changes=%+v", copied.ChangedSlides, copied.SlideChanges)
	}

	// Copy and move a field.
	patchValidate(t, mc, id,
		map[string]any{"op": "copy", "from": "/slides/costs/kpis/0", "path": "/slides/s2/kpis/-"},
		map[string]any{"op": "move", "from": "/slides/s2/title", "path": "/slides/s4/title"},
	)
	stored, _ := mc.deckHandles.Load(id)
	var spec struct {
		Slides []struct {
			Title string           `json:"title"`
			KPIs  []map[string]any `json:"kpis"`
		} `json:"slides"`
	}
	if err := json.Unmarshal(stored.Spec, &spec); err != nil {
		t.Fatal(err)
	}
	if len(spec.Slides[1].KPIs) != 3 || spec.Slides[1].KPIs[2]["label"] != "Cost cut" || len(spec.Slides[2].KPIs) != 2 {
		t.Errorf("field copy: s2 kpis=%v costs kpis=%v", spec.Slides[1].KPIs, spec.Slides[2].KPIs)
	}
	if spec.Slides[1].Title != "" || spec.Slides[4].Title != "Growth held at 42%" {
		t.Errorf("field move: s2 title=%q s4 title=%q", spec.Slides[1].Title, spec.Slides[4].Title)
	}
}

// Patch errors stay precise with the new ops, and a refused patch stores nothing.
func TestSpecPatchMoveCopyAndIDErrors(t *testing.T) {
	cases := []struct {
		name string
		ops  []any
		path string
		want string
	}{
		{"move without from", []any{map[string]any{"op": "move", "path": "/slides/1"}}, "patch[0].from", "move needs from"},
		{"copy from a missing slide", []any{map[string]any{"op": "copy", "from": "/slides/9", "path": "/slides/-"}}, "patch[0].from", "outside the array"},
		{"unknown slide id", []any{map[string]any{"op": "replace", "path": "/slides/risk/title", "value": "x"}}, "patch[0].path", `here (ids: s1, s2, costs, s3)`},
		{"move into itself", []any{map[string]any{"op": "move", "from": "/slides/1", "path": "/slides/1/kpis/0"}}, "patch[0].path", "cannot be moved into itself"},
		{"duplicate id", []any{map[string]any{"op": "add", "path": "/slides/-", "value": map[string]any{"id": "costs", "kind": "closing", "title": "Bye"}}}, "patch[0].value", `is already used by another slide`},
		{"second op fails", []any{
			map[string]any{"op": "move", "from": "/slides/costs", "path": "/slides/0"},
			map[string]any{"op": "remove", "path": "/slides/nope"},
		}, "patch[1].path", `here (ids: costs, s1, s2, s3)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mc := handleTestConfig(t)
			first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revisionTestSpec}))
			res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"deck_id": first.DeckID, "patch": tc.ops}))
			if err != nil {
				t.Fatal(err)
			}
			text := resultText(res)
			if !res.IsError || !strings.Contains(text, tc.want) || !strings.Contains(text, tc.path) {
				t.Errorf("want an error at %s mentioning %q, got:\n%s", tc.path, tc.want, text)
			}
			if got := strings.Join(storedSlideIDs(t, mc, first.DeckID), ","); got != "s1,s2,costs,s3" {
				t.Errorf("a refused patch changed the stored deck: %s", got)
			}
			if h, _ := mc.deckHandles.Load(first.DeckID); h.Revision != 1 {
				t.Errorf("a refused patch created revision %d", h.Revision)
			}
		})
	}
}

// An authored id must be usable as an address: well-formed and unique.
func TestSlideIDValidation(t *testing.T) {
	mc := handleTestConfig(t)
	spec := strings.Replace(revisionTestSpec, `{"kind": "closing"`, `{"id": "costs", "kind": "closing"`, 1)
	dup := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	if dup.OK || !strings.Contains(resultText(mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec})), "already used by slides[2]") {
		t.Errorf("duplicate slide ids should be an error finding, got ok=%v %+v", dup.OK, dup.Findings)
	}
	numeric := strings.Replace(revisionTestSpec, `"id": "costs"`, `"id": "3"`, 1)
	bad := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": numeric}))
	if bad.OK {
		t.Errorf("an id that reads as an index should be an error finding, got %+v", bad.Findings)
	}
}

// go-slide-creator-j77xe: a patch is transactional with its render.
func TestPatchedRenderIsTransactional(t *testing.T) {
	mc := handleTestConfig(t)
	first := renderDeckSpec(t, mc, map[string]any{"spec": revisionTestSpec})
	if !first.Stored || first.Revision != 1 || first.DeckID == "" {
		t.Fatalf("first render: stored=%v revision=%d deck_id=%q", first.Stored, first.Revision, first.DeckID)
	}
	before, _ := mc.deckHandles.Load(first.DeckID)

	// A patch the render refuses: a KPI slide without KPIs does not compile.
	breaking := []any{map[string]any{"op": "remove", "path": "/slides/costs/kpis"}}
	res := mustCall(t, mc.handleRenderDeckSpec, map[string]any{"deck_id": first.DeckID, "patch": breaking})
	var refused renderDeckSpecResponse
	structuredInto(t, res.StructuredContent, &refused)
	if refused.Success || !res.IsError {
		t.Fatalf("the breaking patch should be refused, got %s", resultText(res))
	}
	if refused.Stored || refused.Revision != 1 || refused.DeckID != first.DeckID {
		t.Errorf("refused render: stored=%v revision=%d deck_id=%q; want stored=false at revision 1", refused.Stored, refused.Revision, refused.DeckID)
	}
	if !strings.Contains(resultText(res), `"stored":false`) {
		t.Errorf("the response must state stored:false explicitly:\n%s", resultText(res))
	}
	after, _ := mc.deckHandles.Load(first.DeckID)
	if string(after.Spec) != string(before.Spec) || after.Revision != 1 {
		t.Errorf("a refused render changed the stored deck (revision %d)", after.Revision)
	}
	// The next patch builds on what is stored: the slide still has its KPIs.
	next := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID,
		"patch": []any{map[string]any{"op": "replace", "path": "/slides/costs/kpis/0/value", "value": "9%"}}})
	if !next.Stored || next.Revision != 2 || !equalInts(next.ChangedSlides, []int{2}) {
		t.Errorf("follow-up patch: stored=%v revision=%d changed=%v", next.Stored, next.Revision, next.ChangedSlides)
	}

	// dry_run renders the patch and stores nothing.
	dry := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "dry_run": true,
		"patch": []any{map[string]any{"op": "replace", "path": "/slides/0/subtitle", "value": "Dry run"}}})
	if dry.Stored || dry.Revision != 2 || !equalInts(dry.ChangedSlides, []int{0}) || dry.PptxPath == "" {
		t.Errorf("dry run: stored=%v revision=%d changed=%v pptx=%q", dry.Stored, dry.Revision, dry.ChangedSlides, dry.PptxPath)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h.Revision != 2 || strings.Contains(string(h.Spec), "Dry run") {
		t.Errorf("a dry run stored its patch (revision %d)", h.Revision)
	}
	dryValidate := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "dry_run": true,
		"patch": []any{map[string]any{"op": "replace", "path": "/slides/0/subtitle", "value": "Dry run"}}}))
	if dryValidate.Stored || dryValidate.Revision != 2 || !equalInts(dryValidate.ChangedSlides, []int{0}) {
		t.Errorf("validate dry run: stored=%v revision=%d changed=%v", dryValidate.Stored, dryValidate.Revision, dryValidate.ChangedSlides)
	}

	// validate_deck_spec is the way to park an edit that does not render yet:
	// it stores whatever parses, and says so.
	parked := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "patch": breaking}))
	if parked.OK || !parked.Stored || parked.Revision != 3 {
		t.Errorf("validate with a breaking patch: ok=%v stored=%v revision=%d; want ok=false stored=true revision=3", parked.OK, parked.Stored, parked.Revision)
	}
}

// A refused render's suggested patch must apply to the STORED deck, so it
// carries the edit that was not stored.
func TestRefusedRenderSuggestionCarriesTheUnstoredPatch(t *testing.T) {
	src := specSource{DeckID: "deck_x", RawPatch: []any{map[string]any{"op": "add", "path": "/slides/7", "value": map[string]any{"kind": "closing"}}}, Restore: 3}
	call := semanticPatchSuggestion("deck_x", []any{map[string]any{"op": "replace", "path": "/slides/7/title", "value": "x"}})
	prefixUnstoredPatch(call, src)
	ops, _ := call.ArgsTemplate["patch"].([]any)
	if len(ops) != 2 || ops[0].(map[string]any)["op"] != "add" || ops[1].(map[string]any)["op"] != "replace" {
		t.Errorf("suggested patch = %+v, want the unstored add followed by the fix", ops)
	}
	if call.ArgsTemplate["restore"] != 3 {
		t.Errorf("suggested call lost restore: %+v", call.ArgsTemplate)
	}
}

// go-slide-creator-v5e9h: changed_slides is always present, and a first
// validate has nothing to differ from.
func TestChangedSlidesAlwaysPresent(t *testing.T) {
	mc := handleTestConfig(t)
	// The storyline rules the four-slide fixture cannot meet are waived, so the
	// deck is deterministic_ready and the next step is about what changed.
	spec := strings.Replace(revisionTestSpec, `"template": "midnight-blue"`,
		`"template": "midnight-blue", "waivers": [`+
			`{"code": "CLOSING_WITHOUT_NEXT_STEPS", "reason": "Fixture closes on questions."}, `+
			`{"code": "NO_EXECUTIVE_SUMMARY", "reason": "Fixture is a short review."}]`, 1)
	first := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec})
	if !strings.Contains(resultText(first), `"changed_slides":[]`) {
		t.Errorf("a first validate should report changed_slides: [], got:\n%s", resultText(first))
	}
	id := deckSpecEnvelope(t, first).DeckID
	noop := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id,
		"patch": []any{map[string]any{"op": "replace", "path": "/slides/0/title", "value": "Quarterly Review"}}})
	if !strings.Contains(resultText(noop), `"changed_slides":[]`) || strings.Contains(resultText(noop), "slide_changes") {
		t.Errorf("a no-op patch should report changed_slides: [] and no slide_changes, got:\n%s", resultText(noop))
	}

	rendered := renderDeckSpec(t, mc, map[string]any{"deck_id": id})
	if len(rendered.ChangedSlides) != 4 {
		t.Errorf("a first render changes every slide to the eye, got %v", rendered.ChangedSlides)
	}
	again := mustCall(t, mc.handleRenderDeckSpec, map[string]any{"deck_id": id})
	if !strings.Contains(resultText(again), `"changed_slides":[]`) {
		t.Errorf("re-rendering an unchanged deck should report changed_slides: [], got:\n%s", resultText(again))
	}

	// Notes never show on a slide.
	notes := renderDeckSpec(t, mc, map[string]any{"deck_id": id,
		"patch": []any{map[string]any{"op": "add", "path": "/slides/1/notes", "value": "Say the number twice."}}})
	if len(notes.ChangedSlides) != 0 || changeOf(notes.SlideChanges, "s2") != slideChangeNotesOnly || notes.NextToolCall != nil {
		t.Errorf("notes-only edit: changed_slides=%v slide_changes=%+v next=%+v", notes.ChangedSlides, notes.SlideChanges, notes.NextToolCall)
	}

	// Edits validated but not yet rendered still count at the next render.
	patchValidate(t, mc, id, map[string]any{"op": "replace", "path": "/slides/3/title", "value": "Thank you"})
	later := renderDeckSpec(t, mc, map[string]any{"deck_id": id})
	if !equalInts(later.ChangedSlides, []int{3}) {
		t.Errorf("render after a validated patch: changed_slides=%v, want [3]", later.ChangedSlides)
	}
}

// go-slide-creator-rq1z9: revisions are listed, restorable and forkable.
func TestDeckRevisionsRestoreAndFork(t *testing.T) {
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revisionTestSpec}))
	id := first.DeckID
	patchValidate(t, mc, id, map[string]any{"op": "replace", "path": "/slides/costs/title", "value": "Costs fell 9%"})
	patchValidate(t, mc, id, map[string]any{"op": "remove", "path": "/slides/s2"})

	history := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "read": "history"}))
	if history.Revision != 3 || len(history.Revisions) != 3 {
		t.Fatalf("history: revision=%d rows=%d, want 3 and 3", history.Revision, len(history.Revisions))
	}
	if changeOf(history.Revisions[1].Changes, "costs") != slideChangeEdited || changeOf(history.Revisions[2].Changes, "s2") != slideChangeRemoved {
		t.Errorf("history rows name the wrong slides: %+v", history.Revisions)
	}
	last := map[string]int{}
	for _, s := range history.Slides {
		last[s.ID] = s.LastChanged
	}
	if last["s1"] != 1 || last["costs"] != 2 || last["s3"] != 1 {
		t.Errorf("per-slide last change = %v; want s1:1 costs:2 s3:1", last)
	}

	// Restore revision 1: the removed slide and the old title come back, as a
	// new revision.
	restored := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "restore": float64(1)}))
	if !restored.Stored || restored.Revision != 4 {
		t.Errorf("restore: stored=%v revision=%d, want a stored revision 4", restored.Stored, restored.Revision)
	}
	if got := strings.Join(storedSlideIDs(t, mc, id), ","); got != "s1,s2,costs,s3" {
		t.Errorf("ids after restore = %s", got)
	}
	if h, _ := mc.deckHandles.Load(id); strings.Contains(string(h.Spec), "9%") {
		t.Error("restore kept the later title edit")
	}
	unknown := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "restore": float64(9)})
	if !unknown.IsError || !strings.Contains(resultText(unknown), "kept: 1–4") {
		t.Errorf("restoring an unknown revision should say which are kept, got %s", resultText(unknown))
	}

	// Fork: the approved deck stays as it is while a copy is restyled (A14).
	fork := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "fork": true,
		"patch": []any{map[string]any{"op": "replace", "path": "/meta/template", "value": "forest-green"}}})
	if fork.DeckID == "" || fork.DeckID == id || fork.Template != "forest-green" || fork.Revision != 1 {
		t.Fatalf("fork: deck_id=%q (source %q) template=%q revision=%d", fork.DeckID, id, fork.Template, fork.Revision)
	}
	source, _ := mc.deckHandles.Load(id)
	if source.Revision != 4 || strings.Contains(string(source.Spec), "forest-green") {
		t.Errorf("fork changed its source (revision %d)", source.Revision)
	}
	// A fork keeps the slide ids and can start from an older revision.
	if got := strings.Join(storedSlideIDs(t, mc, fork.DeckID), ","); got != "s1,s2,costs,s3" {
		t.Errorf("fork ids = %s", got)
	}
	old := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "fork": true, "restore": float64(3)}))
	if got := strings.Join(storedSlideIDs(t, mc, old.DeckID), ","); old.DeckID == id || got != "s1,costs,s3" {
		t.Errorf("fork of revision 3: deck_id=%q ids=%s", old.DeckID, got)
	}
}

// go-slide-creator-yxf1k: find and replace across the stored spec.
func TestDeckFindAndReplace(t *testing.T) {
	spec := `{
  "meta": {"title": "ARR reached $9.4m", "template": "midnight-blue"},
  "slides": [
    {"kind": "title", "title": "ARR reached $9.4m", "notes": "Open with the 9.4 figure."},
    {"kind": "kpi_snapshot", "title": "Growth of 19.4% took ARR to $9.4m", "kpis": [{"value": "$9.4m", "label": "ARR"}, {"value": "9.45", "label": "NPS"}]},
    {"kind": "chart_insight", "title": "ARR by quarter", "chart": {"type": "bar_chart", "data": {"categories": ["Q1", "Q2"], "series": [{"name": "ARR", "values": [6.1, 9.4]}]}}, "insights": ["Steady growth."]},
    {"kind": "closing", "title": "Questions?"}
  ]
}`
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	found := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "find": "9.4"}))
	if found.Hits == nil {
		t.Fatal("find returned no hits field")
	}
	paths := map[string]string{}
	for _, h := range *found.Hits {
		paths[h.Path] = h.Excerpt
	}
	// Meta, titles, body values, notes and chart data — but not 19.4 or 9.45.
	for _, want := range []string{"/meta/title", "/slides/0/title", "/slides/0/notes", "/slides/1/title", "/slides/1/kpis/0/value", "/slides/2/chart/data/series/0/values/1"} {
		if _, ok := paths[want]; !ok {
			t.Errorf("find missed %s; hits: %v", want, paths)
		}
	}
	if _, ok := paths["/slides/1/kpis/1/value"]; ok || len(paths) != 6 {
		t.Errorf("find should match whole numbers only (not 9.45), got %v", paths)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h.Revision != 1 {
		t.Errorf("find alone must not store anything (revision %d)", h.Revision)
	}

	replaced := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "find": "9.4", "replace": "9.6"}))
	if replaced.HitCount == nil || *replaced.HitCount != 6 || !replaced.Stored || replaced.Revision != 2 {
		t.Fatalf("replace: hit_count=%v stored=%v revision=%d", replaced.HitCount, replaced.Stored, replaced.Revision)
	}
	// meta.title is a deck-level setting, so the closing slide counts as
	// restyled next to the three edited ones.
	if !equalInts(replaced.ChangedSlides, []int{0, 1, 2, 3}) || countChange(replaced.SlideChanges, slideChangeEdited) != 3 {
		t.Errorf("replace changed_slides = %v (%+v), want all four with three edited", replaced.ChangedSlides, replaced.SlideChanges)
	}
	stored, _ := mc.deckHandles.Load(first.DeckID)
	for _, want := range []string{`"title":"ARR reached $9.6m"`, `Growth of 19.4% took ARR to $9.6m`, `"values":[6.1,9.6]`, `"value":"9.45"`} {
		if !strings.Contains(string(stored.Spec), want) {
			t.Errorf("stored spec missing %s after replace:\n%s", want, stored.Spec)
		}
	}
	left := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "find": "9.4"}))
	if left.HitCount == nil || *left.HitCount != 0 {
		t.Errorf("9.4 still occurs after replace: %+v", left.Hits)
	}

	// Text search ignores case; a kind or template name is never rewritten.
	text := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "find": "CLOSING"}))
	if text.HitCount == nil || *text.HitCount != 0 {
		t.Errorf("find must not search kind names, got %+v", text.Hits)
	}
	arr := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "find": "arr by"}))
	if arr.HitCount == nil || *arr.HitCount != 1 {
		t.Errorf("case-insensitive find: %+v", arr.Hits)
	}
}

// The structured (chapter) form gets ids too, and a slide inside a section is
// addressed by id; changed_slides counts in rendered slides, dividers included.
func TestStructuredDeckSlideIDs(t *testing.T) {
	spec := `{
  "meta": {"title": "Structured", "template": "midnight-blue"},
  "structure": {
    "cover": {"kind": "title", "title": "Structured"},
    "sections": [
      {"title": "Where we stand", "slides": [{"kind": "kpi_snapshot", "title": "Growth held", "kpis": [{"value": "42%", "label": "Growth"}, {"value": "1.2M", "label": "ARR"}]}]},
      {"title": "What comes next", "slides": [{"id": "plan", "kind": "kpi_snapshot", "title": "Plan", "kpis": [{"value": "3", "label": "Bets"}, {"value": "2", "label": "Hires"}]}]}
    ],
    "closing": {"kind": "closing", "title": "Questions?"}
  }
}`
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	// Rendered order: cover, divider, slide, divider, plan, closing.
	edited := patchValidate(t, mc, first.DeckID, map[string]any{"op": "replace", "path": "/structure/sections/1/slides/plan/title", "value": "The plan"})
	if !equalInts(edited.ChangedSlides, []int{4}) || changeOf(edited.SlideChanges, "plan") != slideChangeEdited {
		t.Errorf("structured edit: changed_slides=%v slide_changes=%+v, want rendered slide 4", edited.ChangedSlides, edited.SlideChanges)
	}
	one := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "read": "plan"}))
	if one.SlideRef == nil || one.SlideRef.Index != 4 || !strings.Contains(string(one.Slide), "The plan") {
		t.Errorf("read plan: ref=%+v slide=%s", one.SlideRef, one.Slide)
	}
}
