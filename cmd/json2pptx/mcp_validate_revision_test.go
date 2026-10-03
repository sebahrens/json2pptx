package main

import (
	"os"
	"strings"
	"testing"
)

// dirEntryNames lists a directory's entries.
func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// The deck store and the validating render meet in validate_deck_spec: the
// call renders into a scratch directory to collect its findings
// (go-slide-creator-3rn3s) and stores the spec as a revision
// (go-slide-creator-j77xe). These tests pin what each side needs of the other.

// validateRevisionSpec is revisionTestSpec without a pinned template, and with
// the storyline rules a four-slide fixture cannot meet waived, so a render of
// it is deterministic_ready.
var validateRevisionSpec = strings.Replace(revisionTestSpec, `"template": "midnight-blue"`,
	`"waivers": [{"code": "CLOSING_WITHOUT_NEXT_STEPS", "reason": "Fixture closes on questions."}, `+
		`{"code": "NO_EXECUTIVE_SUMMARY", "reason": "Fixture is a short review."}]`, 1)

// A validating render is not a render of the deck: it leaves no artifact in
// the output directory, marks no revision as rendered and advances no
// revision, so the first real render still reports every slide as new.
func TestValidateScratchRenderLeavesTheDeckStoreAlone(t *testing.T) {
	mc := handleTestConfig(t)
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": validateRevisionSpec, "template": "midnight-blue"}))
	if first.Template != "midnight-blue" || first.TemplateSource != "template argument" {
		t.Errorf("validate echoed template=%q source=%q, want midnight-blue / template argument", first.Template, first.TemplateSource)
	}
	if !first.Stored || first.Revision != 1 {
		t.Fatalf("first validate: stored=%v revision=%d, want stored revision 1", first.Stored, first.Revision)
	}
	h, _ := mc.deckHandles.Load(first.DeckID)
	if h.Rendered != nil || h.RenderedPptx != "" {
		t.Errorf("validate marked the deck as rendered (%q)", h.RenderedPptx)
	}
	if h.Template != "midnight-blue" {
		t.Errorf("validate bound the deck to %q, want midnight-blue", h.Template)
	}
	if entries := dirEntryNames(t, mc.outputDir); len(entries) != 0 {
		t.Errorf("validate left files in the output directory: %v", entries)
	}

	// Validating again changes nothing and stores no new revision.
	again := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID}))
	if again.Revision != 1 || again.Template != "midnight-blue" || again.TemplateSource != "deck_id" {
		t.Errorf("re-validate: revision=%d template=%q source=%q, want 1 / midnight-blue / deck_id", again.Revision, again.Template, again.TemplateSource)
	}

	// dry_run runs the scratch render on the patched spec and stores nothing.
	dry := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "dry_run": true,
		"patch": []any{map[string]any{"op": "replace", "path": "/slides/1/title", "value": "Growth held at 44%"}}}))
	if dry.Stored || dry.Revision != 1 || !equalInts(dry.ChangedSlides, []int{1}) {
		t.Errorf("dry-run validate: stored=%v revision=%d changed=%v, want stored=false revision=1 changed=[1]", dry.Stored, dry.Revision, dry.ChangedSlides)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h.Revision != 1 || strings.Contains(string(h.Spec), "44%") {
		t.Errorf("dry-run validate changed the stored deck (revision %d)", h.Revision)
	}

	// The first real render is still a first render.
	rendered := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID})
	if len(rendered.ChangedSlides) != 4 || rendered.Revision != 1 || rendered.Template != "midnight-blue" {
		t.Errorf("first render after validates: changed=%v revision=%d template=%q, want all four slides, revision 1, midnight-blue",
			rendered.ChangedSlides, rendered.Revision, rendered.Template)
	}

	// A validate after the render does not disturb the last-render baseline:
	// a re-render reports nothing new to look at.
	deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID}))
	rerender := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID})
	if len(rerender.ChangedSlides) != 0 || rerender.NextToolCall != nil {
		t.Errorf("re-render after a validate: changed=%v next=%+v, want nothing changed", rerender.ChangedSlides, rerender.NextToolCall)
	}
}

// A deck_id rendered once on a template it is not bound to keeps its binding
// (go-slide-creator-2dit4), and the change list says what the agent last saw
// (go-slide-creator-v5e9h): that render restyles every slide, and so does the
// next render back on the bound template.
func TestOneOffTemplateRenderIsARestyleBothWays(t *testing.T) {
	mc := handleTestConfig(t)
	first := renderDeckSpec(t, mc, map[string]any{"spec": validateRevisionSpec, "template": "midnight-blue"})
	oneOff := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "template": "forest-green"})
	if oneOff.Template != "forest-green" || len(oneOff.ChangedSlides) != 4 || countChange(oneOff.SlideChanges, slideChangeRestyled) != 4 {
		t.Errorf("one-off render: template=%q changed=%v changes=%+v, want forest-green and four restyled slides", oneOff.Template, oneOff.ChangedSlides, oneOff.SlideChanges)
	}
	if len(oneOff.Warnings) == 0 || !strings.Contains(oneOff.Warnings[0], "stays bound") {
		t.Errorf("one-off render did not say the deck stays bound: %v", oneOff.Warnings)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h.Template != "midnight-blue" || h.Revision != 1 {
		t.Errorf("one-off render rebound the deck: template=%q revision=%d", h.Template, h.Revision)
	}
	back := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID})
	if back.Template != "midnight-blue" || len(back.ChangedSlides) != 4 || countChange(back.SlideChanges, slideChangeRestyled) != 4 {
		t.Errorf("render back on the bound template: template=%q changed=%v, want midnight-blue and four restyled slides", back.Template, back.ChangedSlides)
	}
	steady := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID})
	if len(steady.ChangedSlides) != 0 {
		t.Errorf("a further render on the bound template changed %v, want nothing", steady.ChangedSlides)
	}
}

// fork with a /meta/template patch is how an approved deck goes onto a second
// template (go-slide-creator-rq1z9): the fork is bound to the new template,
// the source keeps its own, and restore brings the fork back.
func TestForkOntoASecondTemplateBindsTheForkOnly(t *testing.T) {
	mc := handleTestConfig(t)
	first := renderDeckSpec(t, mc, map[string]any{"spec": validateRevisionSpec, "template": "midnight-blue"})
	fork := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "fork": true,
		"patch": []any{map[string]any{"op": "add", "path": "/meta/template", "value": "forest-green"}}})
	if fork.DeckID == "" || fork.DeckID == first.DeckID || fork.Template != "forest-green" || !fork.Stored {
		t.Fatalf("fork: deck_id=%q template=%q stored=%v, want a new stored deck on forest-green", fork.DeckID, fork.Template, fork.Stored)
	}
	if h, _ := mc.deckHandles.Load(first.DeckID); h.Template != "midnight-blue" || h.Revision != 1 {
		t.Errorf("fork changed its source: template=%q revision=%d", h.Template, h.Revision)
	}
	if h, _ := mc.deckHandles.Load(fork.DeckID); h.Template != "forest-green" {
		t.Errorf("fork is bound to %q, want forest-green", h.Template)
	}
	onFork := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": fork.DeckID}))
	if onFork.Template != "forest-green" || onFork.TemplateSource != "meta.template" {
		t.Errorf("validate on the fork measured %q (%s), want forest-green (meta.template)", onFork.Template, onFork.TemplateSource)
	}
	onSource := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID}))
	if onSource.Template != "midnight-blue" {
		t.Errorf("validate on the source measured %q, want midnight-blue", onSource.Template)
	}
}
