package main

import (
	"context"
	"encoding/json"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func templateRecoveryFinding(t *testing.T, result any) diagnostics.Finding {
	t.Helper()
	var env diagnostics.FindingEnvelope
	structuredInto(t, result, &env)
	for _, f := range env.Findings {
		if strings.HasSuffix(f.Code, diagnostics.CodeTemplateNotFound) {
			return f
		}
	}
	t.Fatalf("missing template finding: %+v", env)
	return diagnostics.Finding{}
}

func TestTemplateNameRecoveryAcrossDiscoveryAndAuthoring(t *testing.T) {
	for _, custom := range []bool{false, true} {
		mc := testMCPConfig(t)
		typo, want := "midnite-blue", "midnight-blue"
		if custom {
			mc.templatesDir = t.TempDir()
			source, cleanup, err := resolveTemplatePath("midnight-blue", "../../templates")
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			raw, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(mc.templatesDir, "corporate-blue.pptx"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			typo, want = "corporate-blu", "corporate-blue"
		}
		s := newMCPServer(mc)
		for _, tc := range []struct {
			tool, key string
			args      map[string]any
		}{
			{"list_slide_kinds", "template", map[string]any{"kinds": []string{"stat"}, "fields": []string{"budgets"}}},
			{"resolve_theme", "template_name", map[string]any{"color_names": "accent1,dk1"}},
			{"explain_deck_spec", "template", map[string]any{"spec": validSemanticSpec}},
			{"validate_deck_spec", "template", map[string]any{"spec": validSemanticSpec, "strict": "warn"}},
			{"render_deck_spec", "template", map[string]any{"spec": validSemanticSpec, "output_filename": "retry-output.pptx"}},
			{"score_deck", "template", map[string]any{"presentation": map[string]any{"slides": []any{map[string]any{"layout_id": "title"}}}, "slide_indices": []int{0}}},
		} {
			t.Run(want+"/"+tc.tool, func(t *testing.T) {
				tc.args[tc.key] = typo
				res := assertLiveOutputSchema(t, s, tc.tool, tc.args)
				f := templateRecoveryFinding(t, res.StructuredContent)
				if f.Evidence["did_you_mean"] != want || f.NextToolCall == nil || f.NextToolCall.Tool != tc.tool {
					t.Fatalf("not actionable: %+v", f)
				}
				expected := map[string]any{}
				raw, _ := json.Marshal(tc.args)
				_ = json.Unmarshal(raw, &expected)
				expected[tc.key] = want
				if !reflect.DeepEqual(roundTripJSON(t, f.NextToolCall.ArgsTemplate), roundTripJSON(t, expected)) {
					t.Errorf("retry lost arguments: %+v", f.NextToolCall)
				}
				if tc.args[tc.key] != typo {
					t.Fatal("recovery mutated original arguments")
				}
				if tc.tool == "resolve_theme" {
					retry := assertLiveOutputSchema(t, s, tc.tool, f.NextToolCall.ArgsTemplate)
					if retry.IsError {
						t.Fatalf("suggested template retry failed: %s", resultText(retry))
					}
				}
			})
		}
		if msg := templateNotFoundError(typo, mc.templatesDir); !strings.Contains(msg, "did you mean \""+want+"\"") {
			t.Errorf("CLI suggestion missing: %s", msg)
		}
	}
}

func TestTemplateRetryPreservesStoredDeckAndRequest(t *testing.T) {
	mc := testMCPConfig(t)
	mc.deckHandles = newDeckHandleStore(time.Hour)
	stored := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": validSemanticSpec})
	var saved struct {
		DeckID string `json:"deck_id"`
	}
	structuredInto(t, stored.StructuredContent, &saved)
	if saved.DeckID == "" {
		t.Fatal("validation did not store deck")
	}
	before, _ := mc.deckHandles.Load(saved.DeckID)
	args := map[string]any{"deck_id": saved.DeckID, "template": "midnite-blue", "output_filename": "kept-name.pptx", "verbose": true}
	failed := mustCall(t, mc.handleRenderDeckSpec, args)
	f := templateRecoveryFinding(t, failed.StructuredContent)
	if f.NextToolCall == nil || f.NextToolCall.ArgsTemplate["deck_id"] != saved.DeckID {
		t.Fatalf("lost handle: %+v", f)
	}
	after, _ := mc.deckHandles.Load(saved.DeckID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed template lookup changed stored deck")
	}
	retry, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(f.NextToolCall.ArgsTemplate))
	if err != nil {
		t.Fatal(err)
	}
	var out renderDeckSpecResponse
	structuredInto(t, retry.StructuredContent, &out)
	if !out.Success || filepath.Base(out.PptxPath) != "kept-name.pptx" {
		t.Fatalf("retry failed: %+v", out)
	}
}

func TestTemplateSuggestionsStayWithinNameVocabulary(t *testing.T) {
	available := []string{"midnight-blue", "forest-green", "corporate-blue"}
	for _, name := range []string{"", "x", "unrelated-long-name", "../midnight-blu.pptx", "/tmp/midnight-blu.pptx"} {
		if got := closestTemplateName(name, available); got != "" {
			t.Errorf("%q guessed %q", name, got)
		}
	}
	d := templateNotFoundDiagnostic("presentation.template", "presentation.template_path", "midnite-blue", "../../templates")
	args := map[string]any{"presentation": map[string]any{"template": "midnite-blue", "slides": []any{"untouched"}}, "fit_report": true}
	withTemplateRetry(d, makeRequest(args), "validate_input", "presentation.template")
	if d.NextToolCall == nil || d.NextToolCall.ArgsTemplate["presentation"].(map[string]any)["template"] != "midnight-blue" || args["presentation"].(map[string]any)["template"] != "midnite-blue" {
		t.Fatalf("nested retry mutates/loses source: %+v", d)
	}
}
