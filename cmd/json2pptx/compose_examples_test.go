package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// composeExampleTemplates are the templates the advertised compose examples
// must survive: the four fully covered tracked templates plus the local
// p-style template when it is present (it is gitignored, so CI runs without
// it). modern-template has the shortest content area, so it is the one that
// catches a region sized for the roomier templates.
func composeExampleTemplates() []string {
	templates := []string{"forest-green", "midnight-blue", "modern-template", "warm-coral"}
	if _, err := os.Stat(filepath.Join("..", "..", "templates", "p-style.pptx")); err == nil {
		templates = append(templates, "p-style")
	}
	return templates
}

// blockingFindings returns the findings an agent copying an example would
// have to repair: errors, warnings, and the info-severity findings that still
// ask for a change (evidence action review / refuse — e.g. BODY_TOO_LONG,
// VERTICAL_IMBALANCE, DATA_WITHOUT_SOURCE, MISSING_ALT_TEXT). A copy-ready
// example must not teach any of them. Pure advice (action info, such as
// p-style's predicted contrast swap) passes.
func blockingFindings(env diagnostics.FindingEnvelope) []string {
	var out []string
	for _, f := range env.Findings {
		action, _ := f.Evidence["action"].(string)
		if f.Severity == diagnostics.SeverityError || f.Severity == diagnostics.SeverityWarning ||
			action == "review" || action == "refuse" {
			out = append(out, string(f.Severity)+" "+f.Code+": "+f.Message)
		}
	}
	return out
}

// TestComposeExamplesRoundTrip is the go-slide-creator-8wv63 acceptance test.
// It takes the compose examples EXACTLY as skill-info emits them (marshalled
// and decoded the way a consumer reads them), drops each into a deck as its
// only slide, and requires validate_input(fit_report:true) to report no error
// or warning and generate_presentation to produce the file. The examples used
// to carry type:"blank" and {panels:[…]} / {kpis:[…]} wrappers that the live
// contract rejected while the old test only checked JSON syntax.
func TestComposeExamplesRoundTrip(t *testing.T) {
	raw, err := json.Marshal(buildComposeEntry())
	if err != nil {
		t.Fatal(err)
	}
	var emitted struct {
		Examples []struct {
			Title string          `json:"title"`
			JSON  json.RawMessage `json:"json"`
		} `json:"examples"`
	}
	if err := json.Unmarshal(raw, &emitted); err != nil {
		t.Fatal(err)
	}
	if len(emitted.Examples) < 4 {
		t.Fatalf("skill-info should advertise ≥4 compose examples, got %d", len(emitted.Examples))
	}
	mc := testMCPConfig(t)
	for _, tpl := range composeExampleTemplates() {
		for i, ex := range emitted.Examples {
			t.Run(tpl+"/"+ex.Title, func(t *testing.T) {
				var slide map[string]any
				if err := json.Unmarshal(ex.JSON, &slide); err != nil {
					t.Fatalf("example[%d] is not JSON: %v", i, err)
				}
				assertCompleteComposeSlide(t, slide)
				deck := map[string]any{"template": tpl, "slides": []any{slide}}

				res, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{
					"presentation": deck, "fit_report": true,
				}))
				if err != nil {
					t.Fatal(err)
				}
				if res.IsError {
					// A rejection is a bare findings envelope, not dryRunOutput.
					var env diagnostics.FindingEnvelope
					_ = json.Unmarshal([]byte(textContent(res)), &env)
					t.Fatalf("validate_input rejected the example:\n  %s", strings.Join(blockingFindings(env), "\n  "))
				}
				var out dryRunOutput
				if err := json.Unmarshal([]byte(textContent(res)), &out); err != nil {
					t.Fatalf("validate_input response: %v\n%s", err, textContent(res))
				}
				if !out.Valid {
					t.Fatalf("validate_input: valid=false: %s", textContent(res))
				}
				if bad := blockingFindings(out.Findings); len(bad) > 0 {
					t.Errorf("validate_input(fit_report) findings on the example:\n  %s", strings.Join(bad, "\n  "))
				}

				gen, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{"presentation": deck}))
				if err != nil {
					t.Fatal(err)
				}
				var g JSONOutput
				if err := json.Unmarshal([]byte(textContent(gen)), &g); err != nil {
					t.Fatalf("generate_presentation response: %v\n%s", err, textContent(gen))
				}
				if gen.IsError || !g.Success || g.OutputPath == "" {
					t.Fatalf("generate_presentation failed: %s", textContent(gen))
				}
				if _, err := os.Stat(g.OutputPath); err != nil {
					t.Fatalf("generated file missing: %v", err)
				}
			})
		}
	}
}

// assertCompleteComposeSlide pins the authoring contract the examples teach:
// a raw slide envelope (slide_type / layout_id, never the obsolete "type"), an
// action title and a compose envelope.
func assertCompleteComposeSlide(t *testing.T, slide map[string]any) {
	t.Helper()
	if _, ok := slide["type"]; ok {
		t.Error(`example carries the obsolete "type" key`)
	}
	if slide["slide_type"] == nil && slide["layout_id"] == nil {
		t.Error("example sets neither slide_type nor layout_id")
	}
	if _, ok := slide["compose"].(map[string]any); !ok {
		t.Error("example has no compose envelope")
	}
	title := ""
	if content, ok := slide["content"].([]any); ok {
		for _, c := range content {
			if m, ok := c.(map[string]any); ok && m["placeholder_id"] == "title" {
				title, _ = m["text_value"].(string)
			}
		}
	}
	if len(strings.Fields(title)) < 6 {
		t.Errorf("example title %q is not a full-sentence action title", title)
	}
}

// TestComposedExampleDeckSpecRoundTrip is the go-slide-creator-tknmi
// acceptance test: an author following only discovery guidance — get_started
// names list_slide_kinds kinds:["raw_json2pptx"], whose composed_example is the
// composed slide — drops it into an otherwise semantic DeckSpec, and the spec
// validates and renders without the author touching the raw envelope.
func TestComposedExampleDeckSpecRoundTrip(t *testing.T) {
	gs := getStartedResponseFor(t, "brief")
	if !strings.Contains(strings.Join(gs.Notes, "\n"), `kinds:["raw_json2pptx"] composed_example`) {
		t.Fatal("get_started(brief) no longer points authors at list_slide_kinds composed_example")
	}

	mc := handleTestConfig(t)
	res := mustCall(t, handleListSlideKinds, map[string]any{"kinds": []any{"raw_json2pptx"}})
	var listed struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, res.StructuredContent, &listed)
	if len(listed.SlideKinds) != 1 || listed.SlideKinds[0].ComposedExample == nil {
		t.Fatalf("list_slide_kinds kinds:[raw_json2pptx] returned no composed_example: %+v", listed.SlideKinds)
	}
	composed := listed.SlideKinds[0].ComposedExample

	for _, tpl := range composeExampleTemplates() {
		t.Run(tpl, func(t *testing.T) {
			spec := map[string]any{
				"meta": map[string]any{"title": "Launch readiness", "template": tpl},
				"slides": []any{
					map[string]any{"kind": "title", "title": "Launch readiness", "subtitle": "Board review"},
					composed,
					map[string]any{"kind": "closing", "title": "Questions and discussion"},
				},
			}
			specJSON, err := json.Marshal(spec)
			if err != nil {
				t.Fatal(err)
			}
			validated := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": string(specJSON)}))
			// Judge the composed slide (index 1) and deck-wide findings; the
			// surrounding title/closing slides are scaffolding (a cover title
			// legitimately wraps on p-style's narrow title box).
			onComposed := validated.FindingEnvelope
			onComposed.Findings = nil
			for _, f := range validated.Findings {
				if f.SlideNumber == nil || *f.SlideNumber == 2 {
					onComposed.Findings = append(onComposed.Findings, f)
				}
			}
			if bad := blockingFindings(onComposed); len(bad) > 0 || !validated.OK {
				t.Errorf("validate_deck_spec on the composed example (ok=%t):\n  %s", validated.OK, strings.Join(bad, "\n  "))
			}
			var render renderDeckSpecResponse
			structuredInto(t, mustCall(t, mc.handleRenderDeckSpec, map[string]any{"spec": string(specJSON)}).StructuredContent, &render)
			if !render.OK || render.PptxPath == "" || render.SlideCount != 3 {
				t.Fatalf("render_deck_spec: ok=%t path=%q slides=%d error=%q diagnostics=%+v", render.OK, render.PptxPath, render.SlideCount, render.Error, render.Diagnostics)
			}
			for _, d := range render.Diagnostics {
				if d.Severity == "error" || d.Severity == "warning" {
					t.Errorf("render_deck_spec diagnostic: %s %s %s", d.Severity, d.Code, d.Message)
				}
			}
		})
	}
}
