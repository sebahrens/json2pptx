package main

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/render"
)

// chromeDeckSpec is a four-slide deck with footer chrome and an id on every
// slide: the cover prints no footer, the content slides do.
func chromeDeckSpec(t *testing.T, template string, edit func(doc map[string]any)) map[string]any {
	t.Helper()
	const spec = `{
  "meta": {"title": "Project Falcon", "template": "midnight-blue",
           "chrome": {"client_name": "Meridian Capital", "confidentiality": "Confidential", "project_code": "Project Falcon"}},
  "slides": [
    {"id": "cover", "kind": "title", "title": "Project Falcon", "subtitle": "Commercial due diligence"},
    {"id": "growth", "kind": "kpi_snapshot", "title": "Growth held at 42%", "kpis": [{"value": "42%", "label": "Growth"}, {"value": "1.2M", "label": "ARR"}]},
    {"id": "costs", "kind": "kpi_snapshot", "title": "Costs fell 8%", "kpis": [{"value": "8%", "label": "Cost cut"}, {"value": "3.1M", "label": "Opex"}]},
    {"id": "margin", "kind": "kpi_snapshot", "title": "Margin rose two points", "kpis": [{"value": "14.6%", "label": "Margin"}, {"value": "31M", "label": "EBITDA"}]}
  ]
}`
	var doc map[string]any
	if err := json.Unmarshal([]byte(spec), &doc); err != nil {
		t.Fatal(err)
	}
	doc["meta"].(map[string]any)["template"] = template
	if edit != nil {
		edit(doc)
	}
	return doc
}

// renderedChangeTemplates are the templates the o477e tests run on: a shipped
// one, and the private p-style when the checkout has it.
func renderedChangeTemplates() []string {
	out := []string{"midnight-blue"}
	if _, err := os.Stat("../../templates/p-style.pptx"); err == nil {
		out = append(out, "p-style")
	}
	return out
}

// go-slide-creator-o477e (f-A10): removing meta.chrome.project_code is a
// deck-level edit, and changed_slides listed every slide as restyled although
// the cover prints no footer; the thumbnails then called the cover unchanged.
// changed_slides now follows the rendered slides: it names exactly the slides
// whose rendered parts differ between the two files.
func TestChromePatchListsOnlyTheSlidesThatRenderDifferently(t *testing.T) {
	for _, template := range renderedChangeTemplates() {
		t.Run(template, func(t *testing.T) {
			mc := handleTestConfig(t)
			first := renderDeckSpec(t, mc, map[string]any{"spec": chromeDeckSpec(t, template, nil)})
			second := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "patch": []any{
				map[string]any{"op": "remove", "path": "/meta/chrome/project_code"},
			}})

			before, err := render.VisibleSlideKeys(first.PptxPath)
			if err != nil {
				t.Fatal(err)
			}
			after, err := render.VisibleSlideKeys(second.PptxPath)
			if err != nil {
				t.Fatal(err)
			}
			var differ []int
			for i := range after {
				if before[i] != after[i] {
					differ = append(differ, i)
				}
			}
			if len(differ) == 0 || len(differ) == len(after) || slices.Contains(differ, 0) {
				t.Fatalf("fixture: the footer edit should change some slides and leave the cover alone, changed %v", differ)
			}
			if !equalInts(second.ChangedSlides, differ) {
				t.Errorf("changed_slides = %v, want the slides that render differently %v", second.ChangedSlides, differ)
			}
			if got := changeOf(second.SlideChanges, "cover"); got != "" {
				t.Errorf("the cover renders the same and must not be listed, got %q", got)
			}
			if n := countChange(second.SlideChanges, slideChangeRestyled); n != len(differ) {
				t.Errorf("%d slides restyled, want %d: %+v", n, len(differ), second.SlideChanges)
			}
			if idx, ok := thumbnailIndices(t, second); !ok || !equalInts(idx, differ) {
				t.Errorf("the thumbnails step should name the slides that changed %v, got %+v", differ, second.NextToolCall)
			}

			// The revision's own history row agrees with the response.
			history := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "read": "history"}))
			last := history.Revisions[len(history.Revisions)-1]
			if changeOf(last.Changes, "cover") != "" || countChange(last.Changes, slideChangeRestyled) != len(differ) {
				t.Errorf("history revision %d: %+v, want the same %d restyled slides", last.Revision, last.Changes, len(differ))
			}

			// A validate in between renders nothing and must not lose the
			// rendered baseline: the next render still compares slide by slide.
			deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID}))
			third := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "patch": []any{
				map[string]any{"op": "add", "path": "/meta/chrome/project_code", "value": "Project Falcon"},
			}})
			if !equalInts(third.ChangedSlides, differ) {
				t.Errorf("restoring the footer: changed_slides = %v, want %v", third.ChangedSlides, differ)
			}

			// A template switch still restyles every slide.
			other := "forest-green"
			restyled := renderDeckSpec(t, mc, map[string]any{"deck_id": first.DeckID, "patch": []any{
				map[string]any{"op": "replace", "path": "/meta/template", "value": other},
			}})
			if len(restyled.ChangedSlides) != 4 || countChange(restyled.SlideChanges, slideChangeRestyled) != 4 {
				t.Errorf("template switch: changed=%v changes=%+v, want all four restyled", restyled.ChangedSlides, restyled.SlideChanges)
			}
		})
	}
}

// go-slide-creator-o477e (h-A13): a revised spec sent whole, not as a patch,
// started a new deck_id: revision 1, every slide "inserted", and a thumbnails
// step without known_hashes, so twelve images came back for an edit to two
// slides. A spec whose slides carry the ids of a stored deck with the same
// title and template now continues that deck.
func TestResentSpecContinuesItsDeck(t *testing.T) {
	for _, template := range renderedChangeTemplates() {
		t.Run(template, func(t *testing.T) {
			mc := handleTestConfig(t)
			first := renderDeckSpec(t, mc, map[string]any{"spec": chromeDeckSpec(t, template, nil)})
			if first.Revision != 1 || len(first.ChangedSlides) != 4 {
				t.Fatalf("first render: revision %d changed %v", first.Revision, first.ChangedSlides)
			}
			// The agent looked at the first render's thumbnails.
			held := map[int]string{0: "h0", 1: "h1", 2: "h2", 3: "h3"}
			recordDeliveredThumbnails(first.PptxPath, defaultThumbnailDensity, held)

			revised := chromeDeckSpec(t, template, func(doc map[string]any) {
				doc["slides"].([]any)[2].(map[string]any)["title"] = "Costs fell 9%"
			})
			second := renderDeckSpec(t, mc, map[string]any{"spec": revised})
			if second.DeckID != first.DeckID || second.Revision != 2 || !second.Stored {
				t.Fatalf("the revised spec should be revision 2 of %s, got %s revision %d stored=%v", first.DeckID, second.DeckID, second.Revision, second.Stored)
			}
			if !equalInts(second.ChangedSlides, []int{2}) || changeOf(second.SlideChanges, "costs") != slideChangeEdited || len(second.SlideChanges) != 1 {
				t.Errorf("changed_slides=%v slide_changes=%+v, want only costs edited", second.ChangedSlides, second.SlideChanges)
			}
			call := second.NextToolCall
			if call == nil || call.Tool != "render_deck_thumbnails" {
				t.Fatalf("next step should be the thumbnails, got %+v", call)
			}
			var known []string
			structuredInto(t, call.ArgsTemplate[argKnownHashes], &known)
			if len(known) != 4 {
				t.Errorf("the thumbnails step should carry the 4 hashes already delivered, got %+v", call.ArgsTemplate)
			}
			// A whole spec was sent, so the response is the full one.
			if len(second.Slides) != 4 {
				t.Errorf("a resent spec keeps the full response, slides = %+v", second.Slides)
			}
			history := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID, "read": "history"}))
			if len(history.Revisions) != 2 || !strings.Contains(history.Revisions[1].Note, "spec sent again") {
				t.Errorf("history should record the resent spec as revision 2: %+v", history.Revisions)
			}

			// The same spec again is no new revision; validate finds the deck too.
			again := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": revised}))
			if again.DeckID != first.DeckID || again.Revision != 2 || len(again.ChangedSlides) != 0 {
				t.Errorf("validating the same spec: deck %s revision %d changed %v, want %s revision 2 and no change", again.DeckID, again.Revision, again.ChangedSlides, first.DeckID)
			}

			// What does not continue the deck.
			for name, args := range map[string]map[string]any{
				"fork asks for a new deck": {"spec": revised, "fork": true},
				"another title": {"spec": chromeDeckSpec(t, template, func(doc map[string]any) {
					doc["meta"].(map[string]any)["title"] = "Project Kestrel"
				})},
				"another template": {"spec": chromeDeckSpec(t, "forest-green", nil)},
				"slides without ids": {"spec": chromeDeckSpec(t, template, func(doc map[string]any) {
					for _, s := range doc["slides"].([]any) {
						delete(s.(map[string]any), "id")
					}
				})},
				"other slides": {"spec": chromeDeckSpec(t, template, func(doc map[string]any) {
					for i, s := range doc["slides"].([]any) {
						if i > 0 {
							s.(map[string]any)["id"] = s.(map[string]any)["id"].(string) + "-b"
						}
					}
				})},
			} {
				got := renderDeckSpec(t, mc, args)
				if got.DeckID == first.DeckID || got.DeckID == "" || got.Revision != 1 {
					t.Errorf("%s: got deck %s revision %d, want a new deck at revision 1", name, got.DeckID, got.Revision)
				}
			}
			// dry_run stores nothing and names no deck.
			dry := renderDeckSpec(t, mc, map[string]any{"spec": revised, "dry_run": true})
			if dry.Stored || dry.DeckID != "" {
				t.Errorf("dry_run: stored=%v deck_id=%q, want nothing stored", dry.Stored, dry.DeckID)
			}
			if kept, _ := mc.deckHandles.Load(first.DeckID); kept == nil || kept.Revision != 2 {
				t.Errorf("the deck should still be at revision 2: %+v", kept)
			}
		})
	}
}
