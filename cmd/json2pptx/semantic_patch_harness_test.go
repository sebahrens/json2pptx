package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// The go-slide-creator-vihnl acceptance harness: every patch a DeckSpec
// response offers is applied to the deck it was offered for and the result is
// validated again. The finding the patch came with is gone and nothing new
// blocks — or the patch is not offered and the finding says what to do in its
// message and remediation instead.

// optionMatrixTightSpec is an option matrix one line too tall for
// modern-template under a takeaway and a source line: five options by three
// criteria, a detail under each name, and one detail that wraps in that
// template's wider face even in the widest option column, which makes every
// row a line taller (rows are uniform). The e-revise journey's matrix, three
// options by four criteria refused with the option's name quoted as the text
// to shorten, fits since the rows give up padding before text
// (go-slide-creator-vg73u) and the table everything but its text
// (go-slide-creator-dwha2).
const optionMatrixTightSpec = `{"meta":{"title":"Tidewater Carbon investor update","template":"modern-template","source":"Management assessment"},"slides":[
 {"kind":"title","title":"Tidewater Carbon investor update","subtitle":"October 2026"},
 {"kind":"option_matrix","title":"We lead four competitors on cost and deployment speed","criteria":["Cost per tonne","Energy use","Deployment speed"],
  "options":[{"name":"Tidewater Carbon","detail":"Modular units, pilot plant in Q1 2027","scores":[4,3,4]},{"name":"Northgate Capture","detail":"Large fixed plants with long build times and high capex","scores":[2,2,1]},{"name":"Helio Sorbents","detail":"Low energy use, pre-revenue","scores":[2,4,2]},{"name":"Carbonline","detail":"Pipeline access, no capture unit","scores":[1,2,2]},{"name":"Arden Minerals","detail":"Mineralisation, early trials","scores":[2,1,1]}],
  "recommended":"Tidewater Carbon","highlight_label":"Us","decisive_criterion":"Cost per tonne","scale":"harvey","corner_label":"Company","source":"Illustrative management assessment, October 2026","takeaway":"We win on cost and speed; traction is the gap the partners close."}]}`

// timelineStyleSpec is the cold-start journey's probe: a key the kind does not
// read, and a body on a milestone of a timeline that has an end date.
const timelineStyleSpec = `{"meta":{"title":"Margin plan","template":"warm-coral","source":"Management accounts"},"slides":[
 {"kind":"title","title":"Both savings are in place by April 2027","subtitle":"Board, October 2026"},
 {"kind":"timeline","title":"Both savings are in place by April 2027 if work starts in October","style":"dots","takeaway":"Renegotiation pays back first; re-platforming delivers the larger saving.","milestones":[
  {"label":"Board decision","date":"Oct 2026","body":"Approve both cost options."},
  {"label":"Cloud contract renegotiated","date":"Oct 2026","end_date":"Dec 2026"},
  {"label":"Storage tier re-platformed","date":"Oct 2026","end_date":"Apr 2027"}]}]}`

// patchHarnessCorpus are flawed decks with the template each is measured on.
func patchHarnessCorpus(t *testing.T) map[string]map[string]any {
	t.Helper()
	deck := func(spec map[string]any, template string) map[string]any {
		return map[string]any{"spec": spec, "template": template}
	}
	corpus := map[string]map[string]any{
		// The four cases of the review.
		"unknown key on a list entry":       deck(decodeSpecObject(t, journeyRT3Spec), "midnight-blue"),
		"over-full summary":                 deck(decodeSpecObject(t, overfullExecSummarySpec), "modern"),
		"option matrix with a detail line":  deck(decodeSpecObject(t, optionMatrixTightSpec), ""),
		"unknown key and an undrawn body":   deck(decodeSpecObject(t, timelineStyleSpec), ""),
		"architecture tier":                 deck(decodeSpecObject(t, archOnModernSpec), ""),
		"topic titles":                      deck(decodeSpecObject(t, topicTitleSpec), "midnight-blue"),
		"pitch":                             deck(decodeSpecObject(t, pitchSpec("")), "midnight-blue"),
		"nested timeline":                   deck(decodeSpecObject(t, regionsTimelineOnModernSpec), ""),
		"twelve flaws (nothing is stored)":  deck(twelveFlawDraft(t), "modern"),
		"missing template with a near name": deck(decodeSpecObject(t, strings.Replace(pitchSpec(""), `"meta":{`, `"meta":{"template":"midnight-blu",`, 1)), ""),
	}
	// One slide past each kind's documented count, and each kind at its
	// largest count with the example's full copy.
	for _, c := range kindCountRanges {
		for label, n := range map[string]int{"over": c.max + 1, "full": c.max} {
			slide := semantic.KindExample(c.kind)
			entries := slide[c.list].([]any)
			list := make([]any, 0, n)
			for i := 0; i < n; i++ {
				list = append(list, deepCopyJSON(entries[i%len(entries)]))
			}
			slide[c.list] = list
			corpus[fmt.Sprintf("%s %s", label, c.kind)] = deck(map[string]any{
				"meta":   map[string]any{"title": "Counts", "source": "Illustrative"},
				"slides": []any{map[string]any{"kind": "title", "title": "Counts every kind documents", "subtitle": "October 2026"}, slide},
			}, "modern-template")
		}
	}
	return corpus
}

// shortHarnessDeck reports whether a corpus deck is part of the short run:
// the cases of the review, and one kind past and at its documented count.
func shortHarnessDeck(name string) bool {
	switch name {
	case "unknown key on a list entry", "over-full summary", "option matrix with a detail line",
		"unknown key and an undrawn body", "architecture tier", "twelve flaws (nothing is stored)",
		"over decision", "full decision":
		return true
	}
	return false
}

// harnessKey names a finding for comparison across a patch: its code and the
// slide it is on.
func harnessKey(code, path string) string {
	toks := pointerTokens(path)
	if len(toks) >= 2 && toks[0] == "slides" {
		return code + " /slides/" + toks[1]
	}
	return code + " " + path
}

// completePatch fills the values a patch leaves to the author: text at the
// budget the finding states, so a budget that does not clear the finding
// fails the harness.
func completePatch(t *testing.T, f diagnostics.Finding, ops []any) (filled []any, template bool) {
	t.Helper()
	params := map[string]any{}
	if f.Remediation != nil && f.Remediation.Primary != nil {
		params = f.Remediation.Primary.Params
	}
	for i, raw := range ops {
		op := deepCopyJSON(raw).(map[string]any)
		if hint, ok := op["value"].(string); ok && strings.HasPrefix(hint, "<") && strings.HasSuffix(hint, ">") {
			template = true
			text := "Revenue grew 12% in the north region"
			if n, ok := budgetAt(params["max_chars"], i); ok {
				// Words up to exactly the stated budget.
				text = shorten(strings.Repeat("Margin grew two points on mix. ", n/30+1), n)
			} else if n, ok := budgetAt(params["max_words"], i); ok && n < 7 {
				text = strings.Join(strings.Fields(text)[:n], " ")
			}
			op["value"] = text
		}
		filled = append(filled, op)
	}
	return filled, template
}

// patchHarnessRun is what the harness found: the patches offered, how many of
// them the server had verified, and every failure.
type patchHarnessRun struct {
	Patches  int
	Verified int
	Problems []string
	// FirstValidate holds, by corpus deck name, the structured content of the
	// validate_deck_spec response the patches were read from: a test that
	// checks another property of that response reads it here instead of
	// validating the deck again (TestDeckSpecAdviceNamesOnlyFieldsOfTheKind).
	FirstValidate map[string]any
}

// shortPatchHarnessRun is the short run's harness (the cases of the review,
// and one kind past and at its documented count), run once per test binary.
// The harness test and TestAgentJourneyMetrics both read it.
func shortPatchHarnessRun(t *testing.T) patchHarnessRun {
	t.Helper()
	shortPatchHarness.once.Do(func() {
		shortPatchHarness.run = runPatchHarness(t, shortHarnessDeck)
	})
	return shortPatchHarness.run
}

var shortPatchHarness struct {
	once sync.Once
	run  patchHarnessRun
}

func TestEveryEmittedPatchClearsItsFinding(t *testing.T) {
	wantPatches, wantVerified := 12, 4
	var run patchHarnessRun
	if testing.Short() {
		run, wantPatches, wantVerified = shortPatchHarnessRun(t), 8, 3
	} else {
		run = runPatchHarness(t, func(string) bool { return true })
	}
	for _, problem := range run.Problems {
		t.Error(problem)
	}
	t.Logf("%d patches applied, %d of them verified by the server", run.Patches, run.Verified)
	if run.Patches < wantPatches || run.Verified < wantVerified {
		t.Errorf("the corpus exercised %d patches (%d verified); it no longer covers the patch kinds", run.Patches, run.Verified)
	}
}

// runPatchHarness applies every patch the decks of the corpus that keep
// selects are offered, and validates again.
func runPatchHarness(t *testing.T, keep func(name string) bool) (run patchHarnessRun) {
	t.Helper()
	fail := func(format string, args ...any) {
		run.Problems = append(run.Problems, fmt.Sprintf(format, args...))
	}
	mc := refusalTestConfig(t)
	corpus := patchHarnessCorpus(t)
	run.FirstValidate = map[string]any{}
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		if keep(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		args := map[string]any{"spec": corpus[name]["spec"]}
		if tpl, _ := corpus[name]["template"].(string); tpl != "" {
			args["template"] = tpl
		}
		first := mustCall(t, mc.handleValidateDeckSpec, args)
		run.FirstValidate[name] = first.StructuredContent
		env := deckSpecEnvelope(t, first)
		blocked := map[string]bool{}
		for _, f := range env.Findings {
			if f.Severity == diagnostics.SeverityError {
				blocked[harnessKey(f.Code, *f.Path)] = true
			}
		}
		for _, f := range env.Findings {
			label := fmt.Sprintf("%s: %s at %s", name, f.Code, *f.Path)
			isPatch := f.NextToolCall != nil && f.NextToolCall.Tool == "validate_deck_spec"
			if f.Remediation != nil && f.Remediation.Primary != nil && f.Remediation.Primary.Action == diagnostics.ActionApplyPatch && !isPatch {
				fail("%s: the remediation says apply_patch and no patch is offered", label)
			}
			if !isPatch {
				if f.PatchVerified {
					fail("%s: patch_verified without a patch", label)
				}
				continue
			}
			run.Patches++
			ops := patchOf(t, f.NextToolCall)
			filled, template := completePatch(t, f, ops)
			if template == f.PatchVerified {
				fail("%s: patch_verified=%v on a patch that is complete=%v: %v", label, f.PatchVerified, !template, ops)
			}
			if strings.HasSuffix(f.Code, diagnostics.CodeTemplateNotFound) && template {
				continue // the author picks a template; any text is not one
			}
			if f.PatchVerified {
				run.Verified++
			}
			again := map[string]any{"deck_id": env.DeckID, "patch": filled, "dry_run": true}
			if tpl, ok := args["template"]; ok {
				again["template"] = tpl
			}
			after := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, again))
			subjects := map[string]bool{}
			for _, p := range pathsOf(f) {
				subjects[p] = true
			}
			for _, g := range after.Findings {
				if g.Code == f.Code {
					for _, p := range pathsOf(g) {
						if subjects[p] {
							fail("%s: still reported after its patch %v: %s", label, filled, g.Message)
						}
					}
				}
				if g.Severity == diagnostics.SeverityError && !blocked[harnessKey(g.Code, *g.Path)] {
					fail("%s: its patch %v raises a new error: %s at %s: %s", label, filled, g.Code, *g.Path, g.Message)
				}
			}
		}
	}
	return run
}
