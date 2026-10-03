package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// The shape of a DeckSpec finding: one address (go-slide-creator-pilpn), one
// entry per cause (go-slide-creator-c2j5b), everything knowable in the first
// response (go-slide-creator-ipahe) and the nearest name for an unknown one
// (go-slide-creator-t0c1m).

// structuredSpec is a deck in the structure form: a cover, two sections and a
// closing. The second section's slide lacks a takeaway.
const structuredSpec = `{"meta":{"title":"Operations review","archetype":"sales_pitch"},"structure":{
 "cover":{"kind":"title","title":"Operations held the line in a hard quarter","subtitle":"Board, October 2026"},
 "sections":[
  {"title":"Performance","slides":[{"kind":"kpi_snapshot","title":"Momentum held: 42 wins closed this quarter","kpis":[{"value":"42","label":"Wins"},{"value":"31%","label":"Win rate"}],"takeaway":"Execution accelerated.","source":"CRM, Q3"}]},
  {"title":"Outlook","slides":[{"kind":"kpi_snapshot","title":"Pipeline covers 3.1 times the next quarter's target","kpis":[{"value":"3.1x","label":"Coverage"},{"value":"€18M","label":"Pipeline"}],"source":"CRM, Q3"}]}],
 "closing":{"kind":"next_steps","title":"Approve two hires and review the pipeline in November","steps":[{"action":"Approve hires","owner":"COO","date":"Oct 2026"},{"action":"Review pipeline","owner":"CRO","date":"Nov 2026"}]}}}`

// addressCorpus is the spec corpus the address test runs: the parity corpus,
// the reviewer's twelve-flaw draft and a deck in the structure form.
func addressCorpus(t *testing.T) map[string]map[string]any {
	t.Helper()
	corpus := parityCorpus(t)
	delete(corpus, "all-kinds")
	corpus["twelve-flaws"] = twelveFlawDraft(t)
	corpus["structure"] = decodeSpecObject(t, structuredSpec)
	return corpus
}

var dottedSlideRE = regexp.MustCompile(`\bslides\[\d+\]`)

// resolves reports whether a JSON Pointer names something in doc.
func pointerResolves(doc any, pointer string) bool {
	node := doc
	for _, tok := range pointerTokens(pointer) {
		switch cur := node.(type) {
		case map[string]any:
			v, ok := cur[tok]
			if !ok {
				return false
			}
			node = v
		case []any:
			i := -1
			if n, err := json.Number(tok).Int64(); err == nil {
				i = int(n)
			}
			if i < 0 || i >= len(cur) {
				return false
			}
			node = cur[i]
		default:
			return false
		}
	}
	return true
}

// assertAuthoredAddress checks one finding or diagnostic, decoded as generic
// JSON, against the spec that produced it.
func assertAuthoredAddress(t *testing.T, label string, spec any, finding map[string]any) {
	t.Helper()
	code, _ := finding["code"].(string)
	where := label + " " + code
	path, hasPath := finding["path"].(string)
	if !hasPath {
		t.Errorf("%s: no path", where)
		return
	}
	if !pointerResolves(spec, path) {
		t.Errorf("%s: path %q does not resolve in the spec", where, path)
	}
	if missing, ok := finding["missing_path"].(string); ok && !strings.HasPrefix(missing, path) {
		t.Errorf("%s: missing_path %q is not under path %q", where, missing, path)
	}
	if paths, ok := finding["paths"].([]any); ok {
		if n, _ := finding["occurrences"].(float64); int(n) != len(paths) {
			t.Errorf("%s: occurrences %v but %d paths", where, finding["occurrences"], len(paths))
		}
		for _, p := range paths {
			if s, _ := p.(string); !pointerResolves(spec, s) {
				t.Errorf("%s: paths entry %q does not resolve", where, s)
			}
		}
	}
	// slide_number is the 1-based deck position; for a flat deck that is the
	// pointer's index plus one.
	toks := pointerTokens(path)
	if len(toks) >= 2 && toks[0] == "slides" {
		if _, multi := finding["paths"]; !multi {
			n, _ := finding["slide_number"].(float64)
			if idx, err := json.Number(toks[1]).Int64(); err == nil && int(n) != int(idx)+1 {
				t.Errorf("%s: slide_number %v for path %s", where, finding["slide_number"], path)
			}
		}
	}
	for _, retired := range []string{"semantic_path", "raw_path", "slide_index", "where"} {
		if _, has := finding[retired]; has {
			t.Errorf("%s: carries %q; the address is path / slide_number, and compiled-deck locators go under debug", where, retired)
		}
	}
	if ev, ok := finding["evidence"].(map[string]any); ok {
		if _, has := ev["path"]; has {
			t.Errorf("%s: evidence.path is back; the address is the finding's path", where)
		}
	}
	// Outside debug and the suggested call, nothing names a compiled object or
	// uses another notation.
	var walk func(key string, v any)
	walk = func(key string, v any) {
		switch tv := v.(type) {
		case string:
			if dottedSlideRE.MatchString(tv) {
				t.Errorf("%s: %s uses dotted notation: %q", where, key, tv)
			}
			if strings.HasPrefix(tv, "/slides/") && !pointerResolves(spec, tv) && key != "missing_path" {
				t.Errorf("%s: %s = %q does not resolve in the spec", where, key, tv)
			}
		case map[string]any:
			for k, child := range tv {
				if k == "debug" || k == "next_tool_call" || k == "ops" || k == "value" || k == "hint" {
					continue
				}
				if key == "params" && k == "path" {
					continue // a patch target: may be a field to add
				}
				walk(k, child)
			}
		case []any:
			for _, child := range tv {
				walk(key, child)
			}
		}
	}
	walk("finding", finding)
}

// TestEveryDeckSpecFindingPathResolves is the go-slide-creator-pilpn
// acceptance test: every finding validate_deck_spec and render_deck_spec emit
// carries path as a JSON Pointer that resolves in the spec that produced it,
// slide_number as the one human index, and compiled-deck pointers under debug
// only.
func TestEveryDeckSpecFindingPathResolves(t *testing.T) {
	mc := refusalTestConfig(t)
	templates := []string{"modern"}
	if !testing.Short() {
		templates = shippedTemplateNames(t)
	}
	checked := 0
	for _, tpl := range templates {
		for name, spec := range addressCorpus(t) {
			raw, _ := json.Marshal(spec)
			var doc any
			_ = json.Unmarshal(raw, &doc)
			for tool, handler := range map[string]mcpHandler{
				"validate": mc.handleValidateDeckSpec,
				"render":   mc.handleRenderDeckSpec,
			} {
				res, err := handler(context.Background(), makeRequest(map[string]any{"spec": spec, "template": tpl}))
				if err != nil {
					t.Fatal(err)
				}
				var out map[string]any
				structuredInto(t, res.StructuredContent, &out)
				label := tpl + "/" + name + "/" + tool
				for _, key := range []string{"findings", "diagnostics"} {
					list, _ := out[key].([]any)
					for _, f := range list {
						assertAuthoredAddress(t, label, doc, f.(map[string]any))
						checked++
					}
				}
				for _, key := range []string{"deterministic_blocking_reasons", "blocking_reasons"} {
					reasons, _ := out[key].([]any)
					for _, r := range reasons {
						if s, _ := r.(string); dottedSlideRE.MatchString(s) {
							t.Errorf("%s: %s uses dotted notation: %q", label, key, s)
						}
					}
				}
				if e, _ := out["error"].(string); dottedSlideRE.MatchString(e) {
					t.Errorf("%s: error uses dotted notation: %q", label, e)
				}
			}
		}
	}
	if checked < 40 {
		t.Fatalf("only %d findings checked; the corpus no longer exercises the finding shapes", checked)
	}
}

// mcpHandler is an MCP tool handler.
type mcpHandler = func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

// In the structure form a finding's path names the slide where the author
// wrote it and slide_number its position in the rendered deck — not its index
// inside the section, which two slides share.
func TestStructuredSpecFindingAddress(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := decodeSpecObject(t, structuredSpec)
	parsed, diags := semantic.ParseJSON([]byte(structuredSpec))
	if parsed == nil || diags.HasErrors() {
		t.Fatalf("fixture does not parse: %+v", diags)
	}
	want := 0
	for i, s := range semantic.ExpandedSlideSources(parsed) {
		if s.SourcePath == "structure.sections[1].slides[0]" {
			want = i + 1
		}
	}
	if want < 4 {
		t.Fatalf("fixture slide is at deck position %d; expected it after the cover, the agenda or dividers and the first section", want)
	}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "midnight-blue"}))
	found := false
	for _, f := range env.Findings {
		if !strings.HasSuffix(f.Code, diagnostics.CodeSemanticTakeawayRequired) {
			continue
		}
		found = true
		if f.Path == nil || *f.Path != "/structure/sections/1/slides/0" || f.MissingPath != "/structure/sections/1/slides/0/takeaway" {
			t.Errorf("path %v, missing_path %q", f.Path, f.MissingPath)
		}
		if f.SlideNumber == nil || *f.SlideNumber != want {
			t.Errorf("slide_number = %v, want the deck position %d", f.SlideNumber, want)
		}
	}
	if !found {
		t.Fatalf("no takeaway finding on the second section's slide: %+v", env.Findings)
	}
}

// journeyRT3Spec is the reviewer's third validate: a process slide whose four
// steps each carry an unknown "value" key, and a KPI slide with a misspelled
// takeaway.
const journeyRT3Spec = `{"meta":{"title":"Cloud cost programme","archetype":"strategy_proposal","source":"Billing exports"},"slides":[
 {"kind":"title","title":"Cloud cost programme recovers €9.6M a year","subtitle":"Steering committee, October 2026"},
 {"kind":"kpi_snapshot","title":"Spend is €7.8M over budget with a third of compute idle","kpis":[{"label":"Over budget","value":"€7.8M"},{"label":"Idle compute","value":"31%"}],"takeway":"Spend is out of control."},
 {"kind":"process","title":"€9.6M of €14.2M identified savings is committed","steps":[
  {"label":"Identified","value":"€14.2M"},{"label":"Validated","value":"€11.0M"},{"label":"Committed","value":"€9.6M"},{"label":"Delivered","value":"€0.9M"}],"takeaway":"Most savings are committed."},
 {"kind":"next_steps","title":"Approve option B and name three owners this month","steps":[{"action":"Approve budget","owner":"CFO","date":"Oct 2026"},{"action":"Hire FinOps lead","owner":"CTO","date":"Nov 2026"}]}]}`

// go-slide-creator-c2j5b: findings with the same code, slide and cause are one
// entry with occurrences and the affected paths, in both tools, and the
// suggested patch covers every one of them.
func TestSameCauseFindingsAreOneEntry(t *testing.T) {
	mc := refusalTestConfig(t)
	v := deckSpecVerdicts(t, mc, map[string]any{"spec": decodeSpecObject(t, journeyRT3Spec), "template": "midnight-blue"})
	assertFindingParity(t, "rt3", v)

	var unknown []diagnostics.Finding
	for _, f := range v.Validate.Findings {
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticUnknownField) && f.Path != nil && strings.HasPrefix(*f.Path, "/slides/2/") {
			unknown = append(unknown, f)
		}
		// go-slide-creator-t0c1m: with "takeway" read as "takeaway" the slide has
		// its takeaway; reporting it missing as well named one cause twice.
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticTakeawayRequired) {
			t.Errorf("a missing takeaway is reported beside the misspelled one: %v %s", pathsOf(f), f.Message)
		}
	}
	if len(unknown) != 1 {
		t.Fatalf("four steps with the same unknown key should be one finding, got %d: %+v", len(unknown), unknown)
	}
	f := unknown[0]
	wantPaths := []string{"/slides/2/steps/0/value", "/slides/2/steps/1/value", "/slides/2/steps/2/value", "/slides/2/steps/3/value"}
	if f.Occurrences != 4 || strings.Join(f.Paths, ",") != strings.Join(wantPaths, ",") || *f.Path != wantPaths[0] {
		t.Errorf("occurrences %d, path %s, paths %v", f.Occurrences, *f.Path, f.Paths)
	}
	if f.SlideNumber == nil || *f.SlideNumber != 3 {
		t.Errorf("slide_number = %v, want 3", f.SlideNumber)
	}

	// The render side reports the same entry.
	for _, d := range v.Render.Diagnostics {
		if d.Code == diagnostics.CodeSemanticUnknownField && strings.HasPrefix(d.SemanticPath, "slides[2]") && len(d.members) != 4 {
			t.Errorf("render reports %d members for the folded finding", len(d.members))
		}
	}
}

func TestCollapseDiagnostics(t *testing.T) {
	diag := func(code, sev, path, msg string) semanticDiagnostic {
		return semanticDiagnostic{Code: code, Severity: sev, Blocking: sev == "error", SemanticPath: path, Message: msg}
	}
	in := []semanticDiagnostic{
		diag("SEMANTIC_UNKNOWN_FIELD", "error", "slides[1].kpis[0].lable", `unknown field "lable"; did you mean "label"?`),
		diag("SEMANTIC_UNKNOWN_FIELD", "error", "slides[1].kpis[1].lable", `unknown field "lable"; did you mean "label"?`),
		// A different misspelling is a different thing to fix.
		diag("SEMANTIC_UNKNOWN_FIELD", "error", "slides[1].kpis[1].valu", `unknown field "valu"; did you mean "value"?`),
		// Advisories that differ only in what they quote, across slides.
		diag("TITLE_NOT_ACTION", "info", "slides[1].title", `slide 2: title "Key metrics" names a topic`),
		diag("TITLE_NOT_ACTION", "info", "slides[3].title", `slide 4: title "Options" names a topic`),
		// No authored path: never folded.
		{Code: "QUALITY_GATE", Severity: "error", Blocking: true, Message: "quality gate: failed"},
	}
	out := collapseDiagnostics(in)
	if len(out) != 4 {
		t.Fatalf("got %d entries, want 4: %+v", len(out), out)
	}
	if len(out[0].members) != 2 || !strings.Contains(out[0].Message, "2 like this on this slide") {
		t.Errorf("same key on two entries: %+v", out[0])
	}
	if len(out[1].members) != 0 {
		t.Errorf("a different key was folded: %+v", out[1])
	}
	if len(out[2].members) != 2 || !strings.Contains(out[2].Message, "2 like this in the deck") || !spansSlides(out[2].members) {
		t.Errorf("advisories across slides: %+v", out[2])
	}
	again := collapseDiagnostics(out)
	if len(again) != len(out) || again[0].Message != out[0].Message || len(again[0].members) != 2 {
		t.Errorf("collapsing twice changed the result: %+v", again)
	}
	// A later finding of the same cause joins the entry.
	more := collapseDiagnostics(append(out, diag("TITLE_NOT_ACTION", "info", "slides[5].title", `slide 6: title "Plan" names a topic`)))
	if len(more) != 4 || len(more[2].members) != 3 || !strings.Contains(more[2].Message, "3 like this in the deck") || strings.Count(more[2].Message, "like this") != 1 {
		t.Errorf("joining an entry: %+v", more[2])
	}
}

// A slide that fell back from its visual carries the fit findings about the
// fallback as symptoms, and blocks when one of them does.
func TestFallbackFindingsAreSymptomsOfTheirCause(t *testing.T) {
	mc := refusalTestConfig(t)
	draft := twelveFlawDraft(t)
	slides := draft["slides"].([]any)
	spec := map[string]any{
		"meta":   map[string]any{"title": "Cloud cost programme", "source": "Billing exports"},
		"slides": []any{slides[0], slides[1]},
	}
	v := deckSpecVerdicts(t, mc, map[string]any{"spec": spec, "template": "midnight-blue"})
	assertFindingParity(t, "six-point summary", v)
	var cause *diagnostics.Finding
	for i, f := range v.Validate.Findings {
		if strings.HasSuffix(f.Code, patterns.ErrCodeBodyTooLong) || strings.HasSuffix(f.Code, patterns.ErrCodeSlideTextDense) {
			t.Errorf("a finding about the bullet-list fallback stands beside its cause: %s %s", f.Code, f.Message)
		}
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticPatternDegraded) && f.Path != nil && *f.Path == "/slides/1/points" {
			cause = &v.Validate.Findings[i]
		}
	}
	if cause == nil {
		t.Fatalf("no count finding on the summary: %+v", v.Validate.Findings)
	}
	symptoms, _ := cause.Evidence[symptomsDetail].([]any)
	if len(symptoms) == 0 || cause.Severity != diagnostics.SeverityError || !strings.Contains(cause.Message, "see symptoms") {
		t.Errorf("the cause does not carry its symptoms as a blocker: %+v", *cause)
	}
	if n, _ := cause.Remediation.Primary.Params["max_items"].(float64); n != 5 {
		t.Errorf("the count finding does not name the limit: %+v", cause.Remediation.Primary.Params)
	}
}

// go-slide-creator-ipahe: every over-budget item of a list is reported in one
// response, at its own path, with the length measured and the length that
// fits.
func TestEveryOverBudgetItemIsReportedAtOnce(t *testing.T) {
	mc := refusalTestConfig(t)
	draft := twelveFlawDraft(t)
	slides := draft["slides"].([]any)
	spec := map[string]any{
		"meta":   map[string]any{"title": "Cloud cost programme", "source": "Billing exports"},
		"slides": []any{slides[0], slides[7], slides[8]},
	}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "midnight-blue"}))
	budgets := map[string]diagnostics.Finding{}
	for _, f := range env.Findings {
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticPatternDegraded) && f.Path != nil {
			budgets[*f.Path] = f
		}
	}
	labels, ok := budgets["/slides/1/options/0/label"]
	if !ok {
		t.Fatalf("no finding at the first option's label: %+v", env.Findings)
	}
	if labels.Occurrences != 3 || strings.Join(labels.Paths, ",") != "/slides/1/options/0/label,/slides/1/options/1/label,/slides/1/options/2/label" {
		t.Errorf("the three over-long labels are not all listed: %d %v", labels.Occurrences, labels.Paths)
	}
	measured, _ := labels.Evidence["measured"].([]any)
	if len(measured) != 3 || measured[0] != float64(81) || measured[1] != float64(80) || measured[2] != float64(78) || labels.Evidence["allowed"] != float64(60) {
		t.Errorf("measured %v allowed %v, want [81 80 78] and 60", labels.Evidence["measured"], labels.Evidence["allowed"])
	}
	steps, ok := budgets["/slides/2/steps/0/description"]
	if !ok || steps.Occurrences != 8 {
		t.Fatalf("the eight over-long steps are not one finding at the first description: %+v", env.Findings)
	}
	allowed, _ := steps.Evidence["allowed"].([]any)
	if len(allowed) != 8 {
		t.Errorf("each step has its own budget (the box less its label): %v", steps.Evidence["allowed"])
	}
}

// go-slide-creator-ipahe: a slide that compiles is fit-checked although
// another slide has a spec-level error, in validate and in the refused render.
func TestCompilingSlidesAreCheckedBesideSpecErrors(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := decodeSpecObject(t, overfullExecSummarySpec)
	spec["slides"] = append(spec["slides"].([]any), map[string]any{
		"kind": "funnel", "title": "Savings funnel", "stages": []any{map[string]any{"label": "Identified", "value": "€14.2M"}},
	})
	v := deckSpecVerdicts(t, mc, map[string]any{"spec": spec, "template": "modern"})
	assertFindingParity(t, "error beside an over-full slide", v)
	codes := map[string]string{}
	for _, f := range v.Validate.Findings {
		if f.Path != nil {
			codes[f.Code[strings.IndexByte(f.Code, '.')+1:]] = *f.Path
		}
	}
	if codes[diagnostics.CodeSemanticUnknownKind] != "/slides/3/kind" {
		t.Errorf("the unknown kind is not reported: %v", codes)
	}
	if codes[patterns.ErrCodeBodyTooLong] != "/slides/1" {
		t.Errorf("the over-full summary on slide 2 is not reported beside the error on slide 4: %v", codes)
	}
	skipped := false
	for _, w := range v.Validate.Warnings {
		skipped = skipped || (strings.HasPrefix(w, "checks_not_run: /slides/3 ") && strings.Contains(w, "deck-wide"))
	}
	if !skipped {
		t.Errorf("the response does not say which checks waited: %v", v.Validate.Warnings)
	}
	if v.Validate.OK {
		t.Error("a spec with an unknown kind validated ok")
	}
}

// go-slide-creator-t0c1m: an unknown kind names the nearest kind and, for a
// chart or diagram type, the kind that hosts it.
func TestUnknownKindNamesTheKindToUse(t *testing.T) {
	mc := refusalTestConfig(t)
	cases := []struct {
		kind, want, hosted string
		available          bool
	}{
		{"funnel", "chart_insight", "funnel", false},
		{"swot", "framework", "swot", false},
		{"kpi", "kpi_snapshot", "", false},
		{"proces", "process", "", false},
		{"not_a_kind", "", "", true},
	}
	for _, c := range cases {
		spec := map[string]any{"meta": map[string]any{"title": "Probe"}, "slides": []any{map[string]any{"kind": c.kind, "title": "A probe slide"}}}
		env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
		var f *diagnostics.Finding
		for i := range env.Findings {
			if strings.HasSuffix(env.Findings[i].Code, diagnostics.CodeSemanticUnknownKind) {
				f = &env.Findings[i]
			}
		}
		if f == nil || f.Remediation == nil || f.Remediation.Primary == nil {
			t.Fatalf("%s: no unknown-kind finding with a remediation: %+v", c.kind, env.Findings)
		}
		params := f.Remediation.Primary.Params
		got, _ := params["did_you_mean"].(string)
		hosted, _ := params["hosted_type"].(string)
		if got != c.want || hosted != c.hosted {
			t.Errorf("%s: did_you_mean %q hosted_type %q, want %q %q", c.kind, got, hosted, c.want, c.hosted)
		}
		if c.want != "" && !strings.Contains(f.Message, `"`+c.want+`"`) {
			t.Errorf("%s: the message does not name %s: %s", c.kind, c.want, f.Message)
		}
		_, listed := f.Evidence["available"]
		if listed != c.available {
			t.Errorf("%s: available listed = %v, want %v", c.kind, listed, c.available)
		}
		if c.want != "" {
			kinds, _ := f.NextToolCall.ArgsTemplate["kinds"].([]any)
			if f.NextToolCall.Tool != "list_slide_kinds" || len(kinds) != 1 || kinds[0] != c.want {
				t.Errorf("%s: next_tool_call = %+v, want list_slide_kinds for %s alone", c.kind, f.NextToolCall, c.want)
			}
		}
	}
}

// go-slide-creator-t0c1m: what follows from a dropped key is not a second
// cause. With a did_you_mean the content is read under the intended key; with
// none, the finding that follows is marked caused_by.
func TestFindingsDownstreamOfADroppedField(t *testing.T) {
	mc := refusalTestConfig(t)
	column := func(key string) map[string]any {
		return map[string]any{"header": "C: Re-platform", key: []any{"Move to containers", "€12M savings after 24 months", "€6M investment", "High delivery risk"}}
	}
	comparison := func(key string) map[string]any {
		return map[string]any{"meta": map[string]any{"title": "Options", "source": "Finance"}, "slides": []any{
			map[string]any{"kind": "title", "title": "Three options to cut cloud spend by 2027", "subtitle": "October 2026"},
			map[string]any{"kind": "comparison", "title": "Option B saves three times more than a clean-up", "takeaway": "Option B pays back fastest.", "columns": []any{
				map[string]any{"header": "A: Clean-up", "items": []any{"Rightsizing only", "€3.2M savings", "No new team", "Savings erode"}},
				map[string]any{"header": "B: FinOps", "items": []any{"Rightsizing and commitments", "€9.6M savings", "Team of five", "Savings sustained"}},
				column(key),
			}},
		}}
	}

	// "bulets" is one letter from bullets, the family of "items".
	typo := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": comparison("bulets"), "template": "midnight-blue"}))
	renamed := false
	for _, f := range typo.Findings {
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticUnknownField) {
			renamed = f.Remediation != nil && f.Remediation.Primary.Params["did_you_mean"] == "items"
		}
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticPatternDegraded) {
			t.Errorf("the comparison is reported as degraded although its third column is only misspelled: %s", f.Message)
		}
	}
	if !renamed {
		t.Errorf("no did_you_mean items for bulets: %+v", typo.Findings)
	}
	noted := false
	for _, w := range typo.Warnings {
		noted = noted || strings.Contains(w, "bulets → items at /slides/1/columns/2")
	}
	if !noted {
		t.Errorf("the response does not say the key was read as items: %v", typo.Warnings)
	}

	// A key with no close match is left out, and what follows says so.
	stray := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": comparison("zzqq"), "template": "midnight-blue"}))
	marked := false
	for _, f := range stray.Findings {
		if strings.HasSuffix(f.Code, diagnostics.CodeSemanticPatternDegraded) {
			marked = f.Evidence[causedByDetail] == "/slides/1/columns/2/zzqq" && strings.Contains(f.Message, "/slides/1/columns/2/zzqq")
		}
	}
	if !marked {
		t.Errorf("the degraded comparison is not marked as caused by the dropped key: %+v", stray.Findings)
	}
}

// The first response to the twelve-flaw draft carries every tier of findings
// in fewer entries than the first tier alone took before
// (go-slide-creator-c2j5b). It took 17 findings and 11.5 KB to report the
// spec-level tier; the fit tier followed in a 21.5 KB response of its own.
func TestTwelveFlawDraftFirstResponse(t *testing.T) {
	mc := refusalTestConfig(t)
	res := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": twelveFlawDraft(t), "strict": "warn"})
	env := deckSpecEnvelope(t, res)
	structured, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("first response: %d findings, %d bytes of structured content", len(env.Findings), len(structured))
	if len(env.Findings) > 16 {
		t.Errorf("%d findings; the draft's twelve flaws used to take 17 for the first tier alone", len(env.Findings))
	}
	// The bead's target: under 8 KB of structured content (the text block
	// beside it is a synopsis of at most 1 KB). It was 11.4 KB; what came off
	// is the remediation blocks' prose and patch copies (2.7 KB to 1 KB), the
	// describe_finding pointer on every code, the symptoms' sentences and the
	// category that repeats each code's prefix.
	const ceiling = 8192
	if len(structured) > ceiling {
		t.Errorf("first response is %d bytes of structured content, over the %d-byte ceiling", len(structured), ceiling)
	}
	seen := map[string]int{}
	for _, f := range env.Findings {
		seen[f.Code+"@"+*f.Path]++
	}
	for key, n := range seen {
		if n > 1 {
			t.Errorf("%s is reported %d times", key, n)
		}
	}
}
