package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// go-slide-creator-iubjb: list_slide_kinds reports per-field text budgets —
// measured on the template when one is named, the tightest across the shipped
// templates otherwise — and the budgets agree with the findings the same deck
// gets when it is rendered.

type slideKindBudgetsResponse struct {
	SlideKinds     []slideKindListEntry  `json:"slide_kinds"`
	BudgetBasis    *slideKindBudgetBasis `json:"budget_basis"`
	TakeawayBudget struct {
		MaxChars int `json:"max_chars"`
	} `json:"takeaway_budget"`
}

func listSlideKindBudgets(t *testing.T, args map[string]any) slideKindBudgetsResponse {
	t.Helper()
	res, err := semanticTestConfig(t).handleListSlideKinds(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("list_slide_kinds %v failed: %+v", args, res.StructuredContent)
	}
	var out slideKindBudgetsResponse
	structuredInto(t, res.StructuredContent, &out)
	return out
}

func (r slideKindBudgetsResponse) budget(t *testing.T, kind, field string) slideKindBudget {
	t.Helper()
	for _, k := range r.SlideKinds {
		if k.Kind != kind {
			continue
		}
		for _, b := range k.Budgets {
			if b.Field == field {
				return b
			}
		}
	}
	t.Fatalf("no %s budget for kind %s", field, kind)
	return slideKindBudget{}
}

// budgetTemplates are the templates the measured budgets are checked on: every
// shipped one, and the local p-style when it is present.
func budgetTemplates(t *testing.T) []string {
	t.Helper()
	names := embeddedTemplateNames()
	if len(names) == 0 {
		t.Fatal("no shipped templates")
	}
	if _, err := os.Stat(filepath.Join(testTemplatesDir, "p-style.pptx")); err == nil {
		names = append(names, "p-style")
	}
	return names
}

// TestListSlideKindsBudgetsAreOptIn: the compact catalogue carries no budgets;
// a template or fields:["budgets"] adds them, with what they were measured on.
func TestListSlideKindsBudgetsAreOptIn(t *testing.T) {
	compact := listSlideKindBudgets(t, map[string]any{})
	if compact.BudgetBasis != nil || compact.TakeawayBudget.MaxChars != 0 {
		t.Errorf("the compact catalogue grew a budget basis: %+v", compact.BudgetBasis)
	}
	for _, k := range compact.SlideKinds {
		if len(k.Budgets) != 0 {
			t.Errorf("the compact catalogue carries budgets for %s", k.Kind)
		}
	}

	onTemplate := listSlideKindBudgets(t, map[string]any{"template": "midnight-blue"})
	if onTemplate.BudgetBasis == nil || onTemplate.BudgetBasis.Template != "midnight-blue" || len(onTemplate.BudgetBasis.Templates) != 0 {
		t.Fatalf("budget_basis = %+v, want template midnight-blue", onTemplate.BudgetBasis)
	}
	for _, k := range onTemplate.SlideKinds {
		if k.Kind == string(semantic.KindRawJSON2pptx) {
			continue
		}
		title := onTemplate.budget(t, k.Kind, "title")
		if title.Basis != "measured" || title.MaxChars <= 0 || title.MaxCharsPerLine <= 0 || title.MaxLines <= 0 {
			t.Errorf("%s: title budget = %+v, want a measured budget", k.Kind, title)
		}
	}
	if got := onTemplate.budget(t, "decision", "options[].label"); got.Basis != "fixed" || got.MaxChars != 60 {
		t.Errorf("decision options[].label = %+v, want the fixed 60", got)
	}

	tightest := listSlideKindBudgets(t, map[string]any{"fields": []any{"budgets"}})
	if tightest.BudgetBasis == nil || tightest.BudgetBasis.Template != "" || len(tightest.BudgetBasis.Templates) != len(embeddedTemplateNames()) {
		t.Fatalf("budget_basis = %+v, want the shipped templates", tightest.BudgetBasis)
	}
	for _, name := range embeddedTemplateNames() {
		own := listSlideKindBudgets(t, map[string]any{"template": name})
		for _, probe := range [][2]string{{"title", "title"}, {"section", "title"}, {"closing", "title"}, {"closing", "subtitle"}, {"kpi_snapshot", "title"}, {"kpi_snapshot", "takeaway"}} {
			tight, mine := tightest.budget(t, probe[0], probe[1]), own.budget(t, probe[0], probe[1])
			if tight.MaxChars > mine.MaxChars || (mine.MaxCharsPerLine > 0 && tight.MaxCharsPerLine > mine.MaxCharsPerLine) {
				t.Errorf("%s.%s: the tightest budget %+v is looser than %s's own %+v", probe[0], probe[1], tight, name, mine)
			}
		}
	}

	res, err := semanticTestConfig(t).handleListSlideKinds(context.Background(), makeRequest(map[string]any{"template": "no-such-template"}))
	if err != nil || !res.IsError {
		t.Errorf("an unknown template should be an error result, got %+v (%v)", res, err)
	}
}

// renderDiagnostic is one diagnostic of `semantic render`.
type renderDiagnostic struct {
	Code         string `json:"code"`
	SemanticPath string `json:"semantic_path"`
	Message      string `json:"message"`
	Edit         *struct {
		Params map[string]any `json:"params"`
	} `json:"recommended_edit"`
}

// renderSpecDiagnostics renders a DeckSpec through the CLI path and returns
// the diagnostics the author would get.
func renderSpecDiagnostics(t *testing.T, spec map[string]any) []renderDiagnostic {
	t.Helper()
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	specPath := filepath.Join(dir, "deck.json")
	if err := os.WriteFile(specPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"json2pptx", "render", "--spec", specPath, "--output", filepath.Join(dir, "deck.pptx"), "--templates-dir", testTemplatesDir}
	var stdout string
	stderr := captureStderr(t, func() {
		stdout = captureStdout(t, func() { _ = runSemantic() })
	})
	var res struct {
		Diagnostics []renderDiagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("render output is not JSON: %v\n%s\n%s", err, stdout, stderr)
	}
	return res.Diagnostics
}

// wordsWithin returns ordinary words filling at most n characters.
func wordsWithin(n int) string {
	const filler = "Margin grew while churn fell and the plan held across every region this year "
	text := strings.Repeat(filler, n/len(filler)+1)
	text = text[:n]
	if i := strings.LastIndexByte(text, ' '); i > 0 {
		text = text[:i]
	}
	return strings.TrimSpace(text)
}

// TestSlideKindBudgetsAgreeWithRenderFindings renders, on every template, a
// deck written to the budgets list_slide_kinds reports for it and a deck
// written past them. A title inside its per-line budget draws no title
// finding; the takeaway findings quote the reported takeaway budget (it is the
// tighter of the template's two content layouts, so the tightest quote equals
// it and none is below it); the stat's combined stack finding quotes the
// reported combined budget.
func TestSlideKindBudgetsAgreeWithRenderFindings(t *testing.T) {
	titleCodes := map[string]bool{"title_wraps": true, "TITLE_OVERFLOW": true, "TITLE_TRUNCATED": true, "TITLE_WRAPS_DENSE": true}
	for _, name := range budgetTemplates(t) {
		t.Run(name, func(t *testing.T) {
			budgets := listSlideKindBudgets(t, map[string]any{"template": name})
			takeaway := budgets.budget(t, "kpi_snapshot", "takeaway")
			if budgets.TakeawayBudget.MaxChars != takeaway.MaxChars {
				t.Errorf("takeaway_budget.max_chars = %d, the kind's takeaway budget %d", budgets.TakeawayBudget.MaxChars, takeaway.MaxChars)
			}
			combined := budgets.budget(t, "stat", "label+context+source")
			kpis := []any{map[string]any{"value": "118%", "label": "Net retention"}, map[string]any{"value": "$4.2M", "label": "New ARR"}}

			within := renderSpecDiagnostics(t, map[string]any{
				"meta": map[string]any{"title": "Budgets", "template": name},
				"slides": []any{
					map[string]any{"kind": "title", "title": wordsWithin(budgets.budget(t, "title", "title").MaxCharsPerLine)},
					map[string]any{"kind": "section", "title": wordsWithin(budgets.budget(t, "section", "title").MaxCharsPerLine)},
					map[string]any{"kind": "kpi_snapshot", "title": wordsWithin(budgets.budget(t, "kpi_snapshot", "title").MaxCharsPerLine), "kpis": kpis, "takeaway": wordsWithin(takeaway.MaxChars * 3 / 4)},
					map[string]any{"kind": "closing", "title": wordsWithin(budgets.budget(t, "closing", "title").MaxCharsPerLine), "subtitle": "Thank you"},
				},
			})
			for _, d := range within {
				if titleCodes[d.Code] || (d.Code == "BODY_TOO_LONG" && strings.HasSuffix(d.SemanticPath, ".takeaway")) {
					t.Errorf("copy inside the reported budgets drew %s at %s: %s", d.Code, d.SemanticPath, d.Message)
				}
			}

			past := renderSpecDiagnostics(t, map[string]any{
				"meta": map[string]any{"title": "Budgets", "template": name},
				"slides": []any{
					map[string]any{"kind": "kpi_snapshot", "title": "Two numbers", "kpis": kpis, "takeaway": wordsWithin(takeaway.MaxChars * 2)},
					map[string]any{"kind": "stat", "title": "One number", "value": "47 min",
						"label": wordsWithin(72), "context": wordsWithin(72)},
					// Two points degrade to bullets on the One Content layout;
					// the pattern slides above sit on Blank + Title.
					map[string]any{"kind": "executive_summary", "title": "Two points", "points": []any{"Margin grew", "Churn fell"}, "takeaway": wordsWithin(takeaway.MaxChars * 2)},
				},
			})
			var sawCombined bool
			tightestQuote := 0
			for _, d := range past {
				if d.Code != "BODY_TOO_LONG" {
					continue
				}
				switch {
				case strings.HasSuffix(d.SemanticPath, ".takeaway"):
					quoted := 0
					if d.Edit != nil {
						if n, ok := d.Edit.Params["max_chars"].(float64); ok {
							quoted = int(n)
						}
					}
					if quoted < takeaway.MaxChars {
						t.Errorf("takeaway finding at %s quotes max_chars %d, below the reported budget %d", d.SemanticPath, quoted, takeaway.MaxChars)
					}
					if tightestQuote == 0 || quoted < tightestQuote {
						tightestQuote = quoted
					}
				case strings.HasPrefix(d.SemanticPath, "slides[1]") && strings.Contains(d.Message, "stack beneath the number"):
					sawCombined = true
					if !strings.Contains(d.Message, fmt.Sprintf("holds about %d", combined.MaxChars)) {
						t.Errorf("stat stack finding does not quote the reported combined budget %d: %s", combined.MaxChars, d.Message)
					}
				}
			}
			if tightestQuote != takeaway.MaxChars {
				t.Errorf("the tightest takeaway finding quotes max_chars %d, list_slide_kinds reports %d", tightestQuote, takeaway.MaxChars)
			}
			if !sawCombined {
				t.Errorf("a stat past its combined budget was not reported: %+v", past)
			}
		})
	}
}
