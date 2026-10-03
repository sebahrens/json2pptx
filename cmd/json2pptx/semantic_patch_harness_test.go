package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// The go-slide-creator-vihnl acceptance harness: every patch a DeckSpec
// response offers is applied to the deck it was offered for and the result is
// validated again. The finding the patch came with is gone and nothing new
// blocks — or the patch is not offered and the finding says what to do in its
// message and remediation instead.

// optionMatrixTightSpec is the e-revise journey's option matrix: three options
// by four criteria whose recommended row carries a detail line. It was refused
// with the option's name quoted as the text to shorten.
const optionMatrixTightSpec = `{"meta":{"title":"Tidewater Carbon investor update","template":"modern-yellow","source":"Management assessment"},"slides":[
 {"kind":"title","title":"Tidewater Carbon investor update","subtitle":"October 2026"},
 {"kind":"option_matrix","title":"We lead two competitors on cost and deployment speed","criteria":["Cost per tonne","Energy use","Deployment speed","Commercial traction"],
  "options":[{"name":"Tidewater Carbon","detail":"Modular units, pilot plant in Q1 2027","scores":[4,3,4,2]},{"name":"Northgate Capture","detail":"Large fixed plants, long build times","scores":[2,2,1,3]},{"name":"Helio Sorbents","detail":"Low energy use, pre-revenue","scores":[2,4,2,1]}],
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

func TestEveryEmittedPatchClearsItsFinding(t *testing.T) {
	mc := refusalTestConfig(t)
	corpus := patchHarnessCorpus(t)
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)
	wantPatches, wantVerified := 12, 4
	if testing.Short() {
		kept := names[:0]
		for _, name := range names {
			if shortHarnessDeck(name) {
				kept = append(kept, name)
			}
		}
		names, wantPatches, wantVerified = kept, 8, 3
	}
	patches, verified := 0, 0
	for _, name := range names {
		args := map[string]any{"spec": corpus[name]["spec"]}
		if tpl, _ := corpus[name]["template"].(string); tpl != "" {
			args["template"] = tpl
		}
		env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, args))
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
				t.Errorf("%s: the remediation says apply_patch and no patch is offered", label)
			}
			if !isPatch {
				if f.PatchVerified {
					t.Errorf("%s: patch_verified without a patch", label)
				}
				continue
			}
			patches++
			ops := patchOf(t, f.NextToolCall)
			filled, template := completePatch(t, f, ops)
			if template == f.PatchVerified {
				t.Errorf("%s: patch_verified=%v on a patch that is complete=%v: %v", label, f.PatchVerified, !template, ops)
			}
			if strings.HasSuffix(f.Code, diagnostics.CodeTemplateNotFound) && template {
				continue // the author picks a template; any text is not one
			}
			if f.PatchVerified {
				verified++
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
							t.Errorf("%s: still reported after its patch %v: %s", label, filled, g.Message)
						}
					}
				}
				if g.Severity == diagnostics.SeverityError && !blocked[harnessKey(g.Code, *g.Path)] {
					t.Errorf("%s: its patch %v raises a new error: %s at %s: %s", label, filled, g.Code, *g.Path, g.Message)
				}
			}
		}
	}
	t.Logf("%d patches applied, %d of them verified by the server", patches, verified)
	if patches < wantPatches || verified < wantVerified {
		t.Errorf("the corpus exercised %d patches (%d verified); it no longer covers the patch kinds", patches, verified)
	}
}
