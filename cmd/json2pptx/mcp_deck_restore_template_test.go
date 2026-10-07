package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// go-slide-creator-dmiz6: restoring a revision restores the template the deck
// was bound to on it, however that template was named.

const restoreTemplateSpec = `{"meta":{"title":"Restore template test"},"slides":[{"id":"opening","kind":"title","title":"Restore template test"}]}`

func setMetaTemplate(name string) []any {
	return []any{map[string]any{"op": "add", "path": "/meta/template", "value": name}}
}

func boundTemplate(t *testing.T, mc *mcpConfig, deckID string) string {
	t.Helper()
	h, ok := mc.deckHandles.Load(deckID)
	if !ok {
		t.Fatalf("deck_id %q is not stored", deckID)
	}
	return firstNonEmpty(h.Template, h.TemplatePath)
}

func TestRestoreBringsBackAnArgumentBoundTemplate(t *testing.T) {
	mc := handleTestConfig(t)
	first := renderDeckSpec(t, mc, map[string]any{"spec": restoreTemplateSpec, "template": "blue-corporate"})
	if first.Template != "blue-corporate" || first.Revision != 1 {
		t.Fatalf("first render: template=%q revision=%d", first.Template, first.Revision)
	}
	id := first.DeckID
	second := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "patch": setMetaTemplate("modern-template")})
	if second.Template != "modern-template" || second.Revision != 2 {
		t.Fatalf("patched render: template=%q revision=%d", second.Template, second.Revision)
	}

	restored := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1)})
	if restored.Template != "blue-corporate" || restored.Revision != 3 {
		t.Errorf("restore: template=%q revision=%d, want blue-corporate at revision 3", restored.Template, restored.Revision)
	}
	if !equalInts(restored.ChangedSlides, []int{0}) {
		t.Errorf("restore changed the template, so every slide looks different: changed_slides=%v", restored.ChangedSlides)
	}
	if got := boundTemplate(t, mc, id); got != "blue-corporate" {
		t.Errorf("deck is bound to %q after the restore, want blue-corporate", got)
	}
	// The binding holds for a call that names nothing.
	if again := renderDeckSpec(t, mc, map[string]any{"deck_id": id}); again.Template != "blue-corporate" || len(again.ChangedSlides) != 0 {
		t.Errorf("re-render after restore: template=%q changed=%v", again.Template, again.ChangedSlides)
	}
	history := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "read": "history"}))
	if n := len(history.Revisions); n != 3 || !strings.Contains(history.Revisions[n-1].Note, "restored revision 1 on its template blue-corporate") {
		t.Errorf("history does not say the template came back: %+v", history.Revisions)
	}

	// Restoring the revision that pinned modern-template brings that back too.
	if modern := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(2)}); modern.Template != "modern-template" {
		t.Errorf("restore of the pinned revision: template=%q, want modern-template", modern.Template)
	}
}

func TestRestoreKeepsAMetaPinnedTemplate(t *testing.T) {
	mc := handleTestConfig(t)
	pinned := strings.Replace(restoreTemplateSpec, `"meta":{`, `"meta":{"template":"blue-corporate",`, 1)
	id := renderDeckSpec(t, mc, map[string]any{"spec": pinned}).DeckID
	renderDeckSpec(t, mc, map[string]any{"deck_id": id, "patch": setMetaTemplate("modern-template")})
	if restored := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1)}); restored.Template != "blue-corporate" {
		t.Errorf("restore of a meta-pinned revision: template=%q, want blue-corporate", restored.Template)
	}
}

func TestRestoreTemplatePrecedence(t *testing.T) {
	setup := func(t *testing.T) (*mcpConfig, string) {
		mc := handleTestConfig(t)
		id := renderDeckSpec(t, mc, map[string]any{"spec": restoreTemplateSpec, "template": "blue-corporate"}).DeckID
		renderDeckSpec(t, mc, map[string]any{"deck_id": id, "patch": setMetaTemplate("modern-template")})
		return mc, id
	}

	t.Run("a template argument is a one-off on the restored binding", func(t *testing.T) {
		mc, id := setup(t)
		res := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1), "template": "forest-green"})
		if res.Template != "forest-green" {
			t.Errorf("template=%q, want the argument forest-green for this call", res.Template)
		}
		if got := boundTemplate(t, mc, id); got != "blue-corporate" {
			t.Errorf("deck is bound to %q, want the restored blue-corporate", got)
		}
		if !strings.Contains(strings.Join(res.Warnings, " "), `stays bound to "blue-corporate"`) {
			t.Errorf("the one-off warning should name the restored binding: %v", res.Warnings)
		}
	})

	t.Run("a patch to meta.template wins", func(t *testing.T) {
		mc, id := setup(t)
		res := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1), "patch": setMetaTemplate("forest-green")})
		if res.Template != "forest-green" || boundTemplate(t, mc, id) != "forest-green" {
			t.Errorf("template=%q bound=%q, want forest-green for both", res.Template, boundTemplate(t, mc, id))
		}
	})

	t.Run("a fork of the restored revision takes its template", func(t *testing.T) {
		mc, id := setup(t)
		fork := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1), "fork": true})
		if fork.DeckID == id || fork.Template != "blue-corporate" || boundTemplate(t, mc, fork.DeckID) != "blue-corporate" {
			t.Errorf("fork: deck_id=%q template=%q bound=%q", fork.DeckID, fork.Template, boundTemplate(t, mc, fork.DeckID))
		}
		if got := boundTemplate(t, mc, id); got != "modern-template" {
			t.Errorf("the fork rebound its source to %q", got)
		}
	})

	t.Run("validate restores the binding too", func(t *testing.T) {
		mc, id := setup(t)
		env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": id, "restore": float64(1)}))
		if !env.Stored || env.Revision != 3 || boundTemplate(t, mc, id) != "blue-corporate" {
			t.Errorf("validate restore: stored=%v revision=%d bound=%q", env.Stored, env.Revision, boundTemplate(t, mc, id))
		}
	})

	t.Run("a dry run restores nothing", func(t *testing.T) {
		mc, id := setup(t)
		res := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1), "dry_run": true})
		if res.Template != "blue-corporate" || res.Stored {
			t.Errorf("dry run: template=%q stored=%v, want a blue-corporate preview that is not stored", res.Template, res.Stored)
		}
		if got := boundTemplate(t, mc, id); got != "modern-template" {
			t.Errorf("a dry run rebound the deck to %q", got)
		}
	})
}

// A bring-your-own template and the root it was vetted against come back with
// the revision that used them.
func TestRestoreBringsBackABYOTemplate(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	mc := handleTestConfig(t)
	first := renderDeckSpecCall(t, mc, map[string]any{
		"spec":          restoreTemplateSpec,
		"template_path": "tests/quality/fixtures/portability/templates/portability-4x3.pptx",
		"base_dir":      repoRoot,
	})
	if !first.Success {
		t.Fatalf("BYO render failed: %s %+v", first.Error, first.Diagnostics)
	}
	id := first.DeckID
	before, _ := mc.deckHandles.Load(id)
	renderDeckSpec(t, mc, map[string]any{"deck_id": id, "patch": setMetaTemplate("modern-template")})

	restored := renderDeckSpec(t, mc, map[string]any{"deck_id": id, "restore": float64(1)})
	if !bytes.Contains(pptxParts(t, restored.PptxPath)["ppt/presentation.xml"], []byte(fourByThreeSldSz)) {
		t.Error("the restored deck is not on the 4:3 bring-your-own template")
	}
	after, _ := mc.deckHandles.Load(id)
	if after.TemplatePath != before.TemplatePath || after.BaseDir != before.BaseDir || after.Template != "" {
		t.Errorf("binding after restore = %+v, want %+v", after.binding(), before.binding())
	}
}
