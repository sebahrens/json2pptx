package main

import (
	"strings"
	"testing"
)

// go-slide-creator-oqu4a: a revision records the template it was validated or
// rendered on, so a diff shows a template change made by a call's template
// argument — not only one written to meta.template. Both sides used to be read
// on the handle's current template, and the change did not show.
func TestDeckRevisionDiffShowsATemplateArgumentChange(t *testing.T) {
	mc := handleTestConfig(t)
	unpinned := strings.Replace(revisionTestSpec, `, "template": "midnight-blue"`, "", 1)
	validate := func(args map[string]any) deckSpecEnvelopeResponse {
		t.Helper()
		return deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, args))
	}
	retitle := func(title string) []any {
		return []any{map[string]any{"op": "replace", "path": "/slides/costs/title", "value": title}}
	}
	id := validate(map[string]any{"spec": unpinned, "template": "midnight-blue"}).DeckID
	// Revision 2 is validated on another template for that call; revision 3
	// is back on the template the deck is bound to.
	validate(map[string]any{"deck_id": id, "template": "forest-green", "patch": retitle("Costs fell 9%")})
	validate(map[string]any{"deck_id": id, "patch": retitle("Costs fell 10%")})
	h, _ := mc.deckHandles.Load(id)
	if h.Template != "midnight-blue" || len(h.Revisions) != 3 {
		t.Fatalf("handle: template=%q revisions=%d, want midnight-blue and 3", h.Template, len(h.Revisions))
	}
	for i, want := range []string{"midnight-blue", "forest-green", "midnight-blue"} {
		if got := h.Revisions[i].Template; got != want {
			t.Errorf("revision %d records template %q, want %q", i+1, got, want)
		}
	}

	for _, tc := range []struct {
		read, summary string
		restyled      bool
	}{
		{"diff:1..2", "(template midnight-blue → forest-green)", true},
		{"diff:2..3", "(template forest-green → midnight-blue)", true},
		{"diff:1..3", "", false},
	} {
		got := validate(map[string]any{"deck_id": id, "read": tc.read})
		if got.Diff == nil || changeOf(got.Diff.Changes, "costs") != slideChangeEdited {
			t.Fatalf("%s: %+v; want costs edited", tc.read, got.Diff)
		}
		if restyled := changeOf(got.Diff.Changes, "s1") == slideChangeRestyled; restyled != tc.restyled {
			t.Errorf("%s: s1 = %q, restyled want %v", tc.read, changeOf(got.Diff.Changes, "s1"), tc.restyled)
		}
		if tc.restyled != strings.Contains(got.Summary, "(template ") || !strings.Contains(got.Summary, tc.summary) {
			t.Errorf("%s: summary = %q, want it to carry %q", tc.read, got.Summary, tc.summary)
		}
		if tc.restyled && len(got.ChangedSlides) != 4 {
			t.Errorf("%s: changed_slides = %v, want every slide", tc.read, got.ChangedSlides)
		}
	}
	// The history row of a revision reads as the diff with the one before it.
	history := validate(map[string]any{"deck_id": id, "read": "history"})
	if changeOf(history.Revisions[1].Changes, "s1") != slideChangeRestyled || changeOf(history.Revisions[2].Changes, "s1") != slideChangeRestyled {
		t.Errorf("history rows do not show the restyle: %+v", history.Revisions)
	}

	// A later call on the same revision moves the record with it: revision 3
	// is rendered on forest-green, so against revision 2 it is no longer
	// restyled.
	validate(map[string]any{"deck_id": id, "template": "forest-green"})
	if got := validate(map[string]any{"deck_id": id, "read": "diff:2..3"}); changeOf(got.Diff.Changes, "s1") != "" || strings.Contains(got.Summary, "(template ") {
		t.Errorf("diff:2..3 after re-validating revision 3 on forest-green: %+v %q", got.Diff.Changes, got.Summary)
	}
}

// go-slide-creator-rq1z9: any two kept revisions can be compared, not only a
// revision with the one before it. The diff spans every patch between the
// two, names edited fields, and reads either way round.
func TestDeckRevisionDiffOfArbitraryRevisions(t *testing.T) {
	mc := handleTestConfig(t)
	id := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revisionTestSpec})).DeckID
	// Revision 2 edits costs, 3 removes s2, 4 adds a slide before the closing,
	// 5 retitles the closing slide.
	patchValidate(t, mc, id, map[string]any{"op": "replace", "path": "/slides/costs/title", "value": "Costs fell 9%"})
	patchValidate(t, mc, id, map[string]any{"op": "remove", "path": "/slides/s2"})
	newSlide := map[string]any{"kind": "kpi_snapshot", "title": "Churn is flat", "kpis": []any{map[string]any{"value": "2%", "label": "Churn"}, map[string]any{"value": "0", "label": "Change"}}}
	patchValidate(t, mc, id, map[string]any{"op": "add", "path": "/slides/s3", "value": newSlide})
	patchValidate(t, mc, id, map[string]any{"op": "replace", "path": "/slides/s3/title", "value": "Thank you"})

	diff := func(read string) deckSpecEnvelopeResponse {
		t.Helper()
		return deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "read": read}))
	}
	fields := func(changes []slideChange, slideID string) string {
		for _, c := range changes {
			if c.ID == slideID {
				return strings.Join(c.Fields, ",")
			}
		}
		return ""
	}

	// 1 → 4 spans three patches: an edit, a removal and an insert.
	got := diff("diff:1..4")
	if got.Diff == nil || got.Diff.From != 1 || got.Diff.To != 4 {
		t.Fatalf("diff:1..4 returned %+v", got.Diff)
	}
	c := got.Diff.Changes
	if changeOf(c, "costs") != slideChangeEdited || fields(c, "costs") != "title" {
		t.Errorf("costs: change=%q fields=%q; want edited, title", changeOf(c, "costs"), fields(c, "costs"))
	}
	if changeOf(c, "s2") != slideChangeRemoved || changeOf(c, "s4") != slideChangeInserted {
		t.Errorf("s2 / s4: %q / %q; want removed / inserted", changeOf(c, "s2"), changeOf(c, "s4"))
	}
	if changeOf(c, "s1") != "" {
		t.Errorf("an unchanged slide is listed: %+v", c)
	}
	// Revision 4 is s1, costs, s4, s3: costs and s4 look different.
	if !equalInts(got.ChangedSlides, []int{1, 2}) {
		t.Errorf("changed_slides = %v, want [1 2]", got.ChangedSlides)
	}
	if !got.Stored || got.Revision != 5 {
		t.Errorf("a diff must not change the deck: stored=%v revision=%d", got.Stored, got.Revision)
	}
	if !strings.Contains(got.Summary, "revision 1 → 4") || !strings.Contains(got.Summary, "1 edited") {
		t.Errorf("summary = %q", got.Summary)
	}

	// Non-adjacent, neither end current.
	mid := diff("diff:2..4").Diff.Changes
	// costs was edited in revision 2, so between 2 and 4 it only moved up.
	if changeOf(mid, "costs") != slideChangeRenumbered || changeOf(mid, "s2") != slideChangeRemoved || changeOf(mid, "s4") != slideChangeInserted {
		t.Errorf("diff:2..4 = %+v; want s2 removed, s4 inserted and costs only renumbered", mid)
	}
	// Backwards: what going back to revision 1 would undo.
	back := diff("diff:5..1").Diff.Changes
	if changeOf(back, "s2") != slideChangeInserted || changeOf(back, "s4") != slideChangeRemoved || fields(back, "s3") != "title" {
		t.Errorf("diff:5..1 = %+v; want s2 inserted, s4 removed, s3 title edited", back)
	}
	// One number compares with the current revision; equal ends are identical.
	if short := diff("diff:4"); short.Diff.To != 5 || changeOf(short.Diff.Changes, "s3") != slideChangeEdited || len(short.Diff.Changes) != 1 {
		t.Errorf("diff:4 = %+v; want only s3 edited against revision 5", short.Diff)
	}
	if same := diff("diff:3..3"); len(same.Diff.Changes) != 0 || !strings.Contains(same.Summary, "identical") {
		t.Errorf("diff:3..3 = %+v %q", same.Diff, same.Summary)
	}
	// An adjacent diff names the slide the later revision's history row names.
	history := diff("history")
	adjacent := diff("diff:2..3").Diff.Changes
	if changeOf(adjacent, "s2") != slideChangeRemoved || changeOf(history.Revisions[2].Changes, "s2") != slideChangeRemoved {
		t.Errorf("diff:2..3 = %+v; history row 3 = %+v", adjacent, history.Revisions[2].Changes)
	}

	for read, want := range map[string]string{
		"diff:1..9": "revision 9 is not kept",
		"diff:x":    "not a revision diff",
	} {
		res := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "read": read})
		if !res.IsError || !strings.Contains(resultText(res), want) {
			t.Errorf("read %q: want a refusal containing %q, got:\n%s", read, want, resultText(res))
		}
	}
	noHandle := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revisionTestSpec, "read": "diff:1..2"})
	if !noHandle.IsError || !strings.Contains(resultText(noHandle), "deck_id") {
		t.Errorf("a diff without a deck_id should ask for one, got:\n%s", resultText(noHandle))
	}
}
