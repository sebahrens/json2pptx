package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/svggen/safeyaml"
)

// One finding set and one severity model for a DeckSpec: the acceptance tests
// of go-slide-creator-x9rhq, -oh3qr, -2dit4, -3rn3s and -uon4b.

// overfullExecSummarySpec is the agent-journey A2 / C5 deck: an executive
// summary whose five points need more height than the "modern" template's
// content area. Generation refuses it, and the cause is the pattern's height
// deficit, not any one of the ten shrunk fields. The journey's own copy (five
// two-line supports) fits modern since the rows give up padding before text
// (go-slide-creator-vg73u), so two supports here run to a third line: 327pt
// against modern's 311pt, inside the 347-360pt of the standard templates.
const overfullExecSummarySpec = `{"meta":{"title":"Cost programme","archetype":"strategy_proposal"},"slides":[
 {"kind":"title","title":"Cost programme delivers EUR 40m by 2027","subtitle":"Steering committee, October 2026"},
 {"kind":"executive_summary","title":"Three levers deliver EUR 40m of savings by 2027","points":[
   {"lead":"Procurement consolidation saves EUR 18m","support":"Moving from 14 regional contracts to three frame agreements cuts unit prices by 9 percent across categories, and the first two agreements are already signed with incumbent suppliers."},
   {"lead":"Shared services save EUR 12m","support":"Finance and HR transactional work moves to the Krakow centre in two waves during 2026 and early 2027."},
   {"lead":"Footprint reduction saves EUR 10m","support":"Closing two of nine warehouses removes fixed cost while keeping next-day delivery for 96 percent of orders, because the remaining seven sites absorb the volume without new capacity."},
   {"lead":"One-off costs stay under EUR 15m","support":"Severance, lease exits and system migration are phased so that payback arrives within fourteen months."},
   {"lead":"Risks are concentrated in supplier transition","support":"Dual sourcing for the first two quarters protects service levels while new agreements ramp up."}
 ],"takeaway":"Approve the programme now to book the first EUR 9m in 2026."},
 {"kind":"next_steps","title":"Approve the programme and name three owners this month","steps":[{"action":"Approve budget","owner":"CFO","date":"Oct 2026"},{"action":"Launch tender","owner":"CPO","date":"Nov 2026"},{"action":"Start wave one","owner":"COO","date":"Jan 2027"}]}
]}`

// sevenSlidePitchSpec is the agent-journey A6 deck: the seven slides a user
// asked for (problem, solution, market, traction, team, close), which has no
// executive summary and ends on questions. %s is spliced into meta.
const sevenSlidePitchSpec = `{"meta":{"title":"Tidewater seed round"%s},"slides":[
 {"kind":"title","title":"Tidewater cuts harbour dwell time by a third","subtitle":"Seed round, October 2026"},
 {"kind":"comparison","title":"Ports lose two days per vessel to manual berth planning","columns":[{"header":"Today","items":["Planners juggle spreadsheets and radio calls","Berth conflicts found on arrival"]},{"header":"With Tidewater","items":["One live plan shared by every party","Conflicts flagged 36 hours ahead"]}],"takeaway":"Dwell time is a planning problem, not a capacity problem."},
 {"kind":"process","title":"Tidewater plans berths in three automated steps","steps":[{"title":"Ingest","description":"AIS and terminal data"},{"title":"Optimise","description":"Berth and crane plan"},{"title":"Publish","description":"Shared live schedule"}],"takeaway":"Planning moves from the radio to one plan."},
 {"kind":"stat","title":"The addressable market is 4.1 billion euros a year","value":"€4.1bn","label":"Port software spend, 2026","source":"Drewry 2026","takeaway":"Mid-size ports are the unserved half."},
 {"kind":"kpi_snapshot","title":"Three pilots already cut dwell time by 31 percent","kpis":[{"value":"31%","label":"Dwell time cut"},{"value":"3","label":"Paying pilots"},{"value":"€420k","label":"ARR"}],"source":"Company data, Q3 2026","takeaway":"Pilots convert to annual contracts."},
 {"kind":"team","title":"The team has run ports and built planning software","members":[{"name":"Ada Voss","role":"CEO","bio":"Ran operations at Rotterdam terminal."},{"name":"Ben Okoro","role":"CTO","bio":"Built routing at a logistics unicorn."}]},
 {"kind":"closing","title":"Questions?"}
]}`

func pitchSpec(meta string) string { return strings.Replace(sevenSlidePitchSpec, "%s", meta, 1) }

// templateSensitiveSpec is overfullExecSummarySpec read as the deck that
// depends on its template: it fits the standard templates and not "modern",
// whose content area is 311pt against 347-360pt there. This is agent-journey
// C5: validated without a template, then rendered on modern.
func templateSensitiveSpec(t *testing.T) map[string]any {
	t.Helper()
	return decodeSpecObject(t, overfullExecSummarySpec)
}

// overfullOnModernVerdicts is what validate_deck_spec and render_deck_spec
// answer for overfullExecSummarySpec on "modern", from a config of their own,
// asked once per test binary. The pair is a search for the cut that clears the
// slide, in each tool, more than a minute under -race, and four tests read
// the answer to the same request: the parity corpus, the root cause and its
// symptoms, the template echo, and the journey delight that an over-full
// slide is refused (go-slide-creator-efhg2). An answer is kept only when the
// asking test had not failed by then, so every test that asks reports a
// broken one; read it only.
func overfullOnModernVerdicts(t *testing.T) deckSpecVerdict {
	t.Helper()
	overfullOnModern.mu.Lock()
	defer overfullOnModern.mu.Unlock()
	if overfullOnModern.have {
		return overfullOnModern.verdicts
	}
	failedBefore := t.Failed()
	v := deckSpecVerdicts(t, refusalTestConfig(t), map[string]any{"spec": decodeSpecObject(t, overfullExecSummarySpec), "template": overfullTemplate})
	if !failedBefore && !t.Failed() {
		overfullOnModern.verdicts, overfullOnModern.have = v, true
	}
	return v
}

var overfullOnModern struct {
	mu       sync.Mutex
	have     bool
	verdicts deckSpecVerdict
}

// overfullTemplate is the template that refuses overfullExecSummarySpec, and
// overfullCorpusName the spec's name in parityCorpus.
const (
	overfullTemplate   = "modern"
	overfullCorpusName = "overfull-exec-summary"
)

// topicTitleSpec trips the gate on an aggregate criterion alone: more than a
// quarter of its slides carry a topic title, and no single finding blocks.
const topicTitleSpec = `{"meta":{"title":"Market review","archetype":"market_analysis"},"slides":[
 {"kind":"title","title":"Market review","subtitle":"October 2026"},
 {"kind":"comparison","title":"Options","columns":[{"header":"Today","items":["Planners juggle spreadsheets and radio calls","Berth conflicts found on arrival"]},{"header":"With Tidewater","items":["One live plan shared by every party","Conflicts flagged 36 hours ahead"]}],"takeaway":"Dwell time is a planning problem."},
 {"kind":"stat","title":"The addressable market is 4.1 billion euros a year","value":"€4.1bn","label":"Port software spend, 2026","source":"Drewry 2026","takeaway":"Mid-size ports are the unserved half."},
 {"kind":"comparison","title":"Overview","columns":[{"header":"Today","items":["Planners juggle spreadsheets and radio calls","Berth conflicts found on arrival"]},{"header":"With Tidewater","items":["One live plan shared by every party","Conflicts flagged 36 hours ahead"]}],"takeaway":"One plan replaces the radio."},
 {"kind":"next_steps","title":"Approve the budget and launch the tender this quarter","steps":[{"action":"Approve budget","owner":"CFO","date":"Oct 2026"},{"action":"Launch tender","owner":"CPO","date":"Nov 2026"}]}
]}`

// rawContractSlides are raw_json2pptx payloads that are well-formed json2pptx
// slides but break a pattern or diagram data contract. Each used to validate
// clean and be refused at render (agent-journey B2).
var rawContractSlides = []struct {
	name     string
	slide    string
	wantPath string // the DeckSpec path the finding must sit at or under
}{
	{
		name:     "pattern unknown value key",
		slide:    `{"slide_type":"blank","content":[{"placeholder_id":"title","type":"text","text_value":"Maturity grows in three stages toward autonomy"}],"pattern":{"name":"journey-maturity-model","values":{"current":2,"stages":[{"label":"Ad hoc","description":"Manual work"},{"label":"Defined","description":"Documented"},{"label":"Managed","description":"Measured"}]}}}`,
		wantPath: "slides[1].slide.pattern",
	},
	{
		name:     "diagram unknown item key",
		slide:    `{"slide_type":"diagram","content":[{"placeholder_id":"title","type":"text","text_value":"Operations carry most of the margin in the chain"},{"placeholder_id":"body","type":"diagram","diagram_value":{"type":"value_chain","data":{"primary":[{"label":"Inbound"},{"label":"Operations"},{"label":"Outbound","highlight":true}],"support":[{"label":"HR"}]}}}]}`,
		wantPath: "slides[1].slide.content[1].diagram_value.data.primary[2].highlight",
	},
	{
		name:     "pattern value of the wrong shape",
		slide:    `{"slide_type":"blank","content":[{"placeholder_id":"title","type":"text","text_value":"Three numbers carry the quarter for the board"}],"pattern":{"name":"kpi-3up","values":{"items":"three numbers"}}}`,
		wantPath: "slides[1].slide.pattern",
	},
	{
		name:     "pattern required value missing",
		slide:    `{"slide_type":"blank","content":[{"placeholder_id":"title","type":"text","text_value":"Maturity grows in three stages toward autonomy"}],"pattern":{"name":"journey-maturity-model","values":{"stages":[{"description":"Manual work"},{"label":"Defined"},{"label":"Managed"}]}}}`,
		wantPath: "slides[1].slide.pattern",
	},
	{
		name:     "unknown pattern name",
		slide:    `{"slide_type":"blank","content":[{"placeholder_id":"title","type":"text","text_value":"Three numbers carry the quarter for the board"}],"pattern":{"name":"kpi-thirty-up","values":{}}}`,
		wantPath: "slides[1].slide.pattern",
	},
	{
		name:     "chart unknown data key",
		slide:    `{"slide_type":"chart","content":[{"placeholder_id":"title","type":"text","text_value":"Revenue grew every quarter of the year"},{"placeholder_id":"body","type":"chart","chart_value":{"type":"bar_chart","data":{"categories":["Q1","Q2"],"series":[{"name":"Revenue","values":[12,14],"colour":"red"}]}}}]}`,
		wantPath: "slides[1].slide.content[1]",
	},
}

func rawContractSpec(slide string) map[string]any {
	var raw map[string]any
	if err := json.Unmarshal([]byte(slide), &raw); err != nil {
		panic(err)
	}
	return map[string]any{
		"meta": map[string]any{"title": "Raw probe"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Raw probe", "subtitle": "Contract parity"},
			map[string]any{"kind": "raw_json2pptx", "slide": raw},
		},
	}
}

// findingPath is a DeckSpec finding's address in the pipeline's dotted form:
// the field the finding is about (missing_path when the spec does not have
// it), else path. A finding that was not shaped for a DeckSpec surface (an
// argument error) keeps evidence.path.
func findingPath(f diagnostics.Finding) string {
	switch {
	case f.MissingPath != "":
		return dottedPath(f.MissingPath)
	case f.Path != nil:
		return dottedPath(*f.Path)
	}
	p, _ := f.Evidence["path"].(string)
	return p
}

// findingKey is what two tools must agree on for one finding.
type findingKey struct{ Code, Path, Severity string }

func sortedKeys(keys []findingKey) []findingKey {
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Severity < b.Severity
	})
	return keys
}

// envelopeKeys are an envelope's findings as comparison keys. It also asserts
// the one-flag rule: every finding says whether it blocks, and blocks exactly
// when it is an error.
func envelopeKeys(t *testing.T, label string, env diagnostics.FindingEnvelope) []findingKey {
	t.Helper()
	keys := make([]findingKey, 0, len(env.Findings))
	for _, f := range env.Findings {
		path := findingPath(f)
		keys = append(keys, findingKey{f.Code, path, string(f.Severity)})
		if f.Blocking == nil {
			continue // a transport error envelope carries no gate verdict
		}
		if *f.Blocking != (f.Severity == diagnostics.SeverityError) {
			t.Errorf("%s: finding %s at %s has severity %s but blocking=%v", label, f.Code, path, f.Severity, *f.Blocking)
		}
	}
	return sortedKeys(keys)
}

// diagnosticKeys are a render's diagnostics as comparison keys, with the same
// one-flag assertion.
func diagnosticKeys(t *testing.T, label string, diags []semanticDiagnostic) []findingKey {
	t.Helper()
	keys := make([]findingKey, 0, len(diags))
	for _, d := range diags {
		code := diagnostics.DottedCode(diagnostics.ClassifyCode(d.Code), d.Code)
		path := firstNonEmpty(d.SemanticPath, d.RawPath)
		keys = append(keys, findingKey{code, path, d.Severity})
		if d.Blocking != (d.Severity == "error") {
			t.Errorf("%s: diagnostic %s at %s has severity %s but blocking=%v", label, d.Code, path, d.Severity, d.Blocking)
		}
	}
	return sortedKeys(keys)
}

// deckSpecVerdicts runs validate_deck_spec and render_deck_spec with the same
// arguments and returns both verdicts.
type deckSpecVerdict struct {
	Validate deckSpecEnvelopeResponse
	Render   renderDeckSpecResponse
	// RenderEnvelope is set instead of Render when render answered with a
	// finding envelope (an unresolvable template).
	RenderEnvelope *diagnostics.FindingEnvelope
	// ValidateStructured and RenderStructured are the two tools' structured
	// content as returned, for a test that reads the response as JSON.
	ValidateStructured, RenderStructured any
}

func deckSpecVerdicts(t *testing.T, mc *mcpConfig, args map[string]any) deckSpecVerdict {
	t.Helper()
	ctx := context.Background()
	var out deckSpecVerdict
	vres, err := mc.handleValidateDeckSpec(ctx, makeRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	structuredInto(t, vres.StructuredContent, &out.Validate)
	rres, err := mc.handleRenderDeckSpec(ctx, makeRequest(args))
	if err != nil {
		t.Fatal(err)
	}
	out.ValidateStructured, out.RenderStructured = vres.StructuredContent, rres.StructuredContent
	var probe map[string]any
	structuredInto(t, rres.StructuredContent, &probe)
	if _, isEnvelope := probe["findings"]; isEnvelope {
		out.RenderEnvelope = &diagnostics.FindingEnvelope{}
		structuredInto(t, rres.StructuredContent, out.RenderEnvelope)
		return out
	}
	structuredInto(t, rres.StructuredContent, &out.Render)
	return out
}

// assertFindingParity is the go-slide-creator-3rn3s contract: for one spec
// revision and template, validate and render report the same findings (code,
// path, severity) and the same verdict.
func assertFindingParity(t *testing.T, label string, v deckSpecVerdict) {
	t.Helper()
	for _, problem := range findingParityProblems(t, label, v) {
		t.Error(problem)
	}
}

// findingParityProblems lists where validate and render disagree on one spec
// and template; none is parity.
func findingParityProblems(t *testing.T, label string, v deckSpecVerdict) (problems []string) {
	t.Helper()
	got := envelopeKeys(t, label+" validate", v.Validate.FindingEnvelope)
	var want []findingKey
	renderReady := false
	if v.RenderEnvelope != nil {
		want = envelopeKeys(t, label+" render", *v.RenderEnvelope)
	} else {
		want = diagnosticKeys(t, label+" render", v.Render.Diagnostics)
		renderReady = v.Render.OK && v.Render.DeterministicReady != nil && *v.Render.DeterministicReady
		if v.Render.OK && v.Validate.Template != v.Render.Template {
			problems = append(problems, fmt.Sprintf("%s: validate measured on template %q, render used %q", label, v.Validate.Template, v.Render.Template))
		}
	}
	if !equalFindingKeys(got, want) {
		problems = append(problems, fmt.Sprintf("%s: validate and render disagree\n  validate: %s\n  render:   %s", label, formatKeys(got), formatKeys(want)))
	}
	if v.Validate.OK != renderReady {
		problems = append(problems, fmt.Sprintf("%s: validate ok=%v but render deterministic_ready=%v (error %q, reasons %v)", label, v.Validate.OK, renderReady, v.Render.Error, v.Render.DeterministicBlockingReasons))
	}
	return problems
}

func equalFindingKeys(a, b []findingKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatKeys(keys []findingKey) string {
	if len(keys) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k.Severity+" "+k.Code+"@"+k.Path)
	}
	return strings.Join(parts, "; ")
}

// shippedTemplateNames lists every template in the repo's templates/ directory:
// the embedded ones, plus a local p-style when present.
func shippedTemplateNames(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("../../templates/*.pptx")
	if err != nil || len(paths) < 9 {
		t.Fatalf("shipped templates: %v %v", paths, err)
	}
	names := make([]string, 0, len(paths))
	for _, p := range paths {
		names = append(names, strings.TrimSuffix(filepath.Base(p), ".pptx"))
	}
	sort.Strings(names)
	return names
}

// parityCorpus is the spec corpus the parity test runs: the shipped semantic
// examples, every slide kind's example in one deck, and the journey decks that
// used to split the two tools. No spec pins a template; the test supplies it.
func parityCorpus(t *testing.T) map[string]map[string]any {
	t.Helper()
	corpus := map[string]map[string]any{}
	for _, name := range []string{"qbr", "sales_pitch", "regions", "invalid"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "semantic", name+".yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var spec map[string]any
		if err := safeyaml.Unmarshal(raw, &spec); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if meta, ok := spec["meta"].(map[string]any); ok {
			delete(meta, "template")
		}
		corpus["example/"+name] = spec
	}
	kinds := []any{}
	for _, k := range semantic.AllSlideKinds() {
		kinds = append(kinds, semantic.KindExample(k))
	}
	corpus["all-kinds"] = map[string]any{"meta": map[string]any{"title": "Every kind"}, "slides": kinds}
	corpus[overfullCorpusName] = decodeSpecObject(t, overfullExecSummarySpec)
	corpus["pitch"] = decodeSpecObject(t, pitchSpec(""))
	corpus["pitch-waived"] = decodeSpecObject(t, pitchSpec(`,"archetype":"sales_pitch","waivers":[{"code":"CLOSING_WITHOUT_NEXT_STEPS","reason":"The brief fixes seven slides; the ask is made verbally."}]`))
	corpus["topic-titles"] = decodeSpecObject(t, topicTitleSpec)
	// A spec that parses but does not compile: both tools report the compile
	// diagnostics.
	corpus["does-not-compile"] = decodeSpecObject(t, `{"meta":{"title":"Executive review"},"slides":[
	 {"kind":"title","title":"Executive review","subtitle":"Board, October"},
	 {"kind":"regions","title":"Gross margin holds at 32% while the launch stays on plan","arrangement":"main_top","regions":[
	  {"kind":"stat","size_pct":50,"value":"32%","label":"Gross margin, Q4"},
	  {"kind":"timeline","size_pct":50,"heading":"Launch plan","milestones":[{"label":"Design","date":"Oct"},{"label":"Pilot","date":"Nov"}]}]}]}`)
	corpus["matrix-refusal"] = decodeSpecObject(t, strings.Replace(regionsTimelineOnModernSpec, `"template":"modern",`, "", 1))
	for _, c := range rawContractSlides {
		corpus["raw/"+c.name] = rawContractSpec(c.slide)
	}
	return corpus
}

// parityRun is what comparing validate with render over a corpus found: the
// (template, spec) pairs compared and every disagreement.
type parityRun struct {
	Pairs    int
	Problems []string
	// Structured holds, by "template/spec name", what validate_deck_spec and
	// render_deck_spec returned for the pair: a test that checks another
	// property of the same responses reads them here instead of running the
	// two tools again (TestEveryDeckSpecFindingPathResolves).
	Structured map[string]parityResponses
}

// parityResponses are the structured content of the two tools for one
// (template, spec) pair.
type parityResponses struct {
	Validate, Render any
}

// runParityCorpus compares validate_deck_spec with render_deck_spec for every
// spec of the corpus on every template.
func runParityCorpus(t *testing.T, mc *mcpConfig, templates []string, corpus map[string]map[string]any) (run parityRun) {
	t.Helper()
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)
	run.Structured = map[string]parityResponses{}
	for _, tpl := range templates {
		for _, name := range names {
			label := tpl + "/" + name
			var v deckSpecVerdict
			if name == overfullCorpusName && tpl == overfullTemplate {
				// The same request as the shared answer's.
				v = overfullOnModernVerdicts(t)
			} else {
				v = deckSpecVerdicts(t, mc, map[string]any{"spec": corpus[name], "template": tpl})
			}
			run.Structured[label] = parityResponses{Validate: v.ValidateStructured, Render: v.RenderStructured}
			run.Pairs++
			run.Problems = append(run.Problems, findingParityProblems(t, label, v)...)
			// A spec that does not parse was measured on nothing.
			if v.RenderEnvelope == nil && v.Validate.Template != tpl {
				run.Problems = append(run.Problems, fmt.Sprintf("%s: validate echoed template %q", label, v.Validate.Template))
			}
		}
	}
	return run
}

// shortParityRun is the short run's parity corpus, compared once per test
// binary: the template with the tightest content area, without the deck of
// every kind (the long run renders each kind on each template). The parity
// test and TestAgentJourneyMetrics both read it.
func shortParityRun(t *testing.T) parityRun {
	t.Helper()
	shortParity.once.Do(func() {
		corpus := parityCorpus(t)
		delete(corpus, "all-kinds")
		shortParity.run = runParityCorpus(t, refusalTestConfig(t), []string{shortParityTemplate}, corpus)
	})
	return shortParity.run
}

var shortParity struct {
	once sync.Once
	run  parityRun
}

// shortParityTemplate is the template of the short run's parity corpus.
const shortParityTemplate = "modern"

// TestDeckSpecFindingParityCorpus is the go-slide-creator-3rn3s / -2dit4
// acceptance test: over a corpus of specs and every shipped template,
// validate_deck_spec and render_deck_spec return the same findings (code,
// path, severity) and the same verdict.
func TestDeckSpecFindingParityCorpus(t *testing.T) {
	mc := refusalTestConfig(t)
	corpus := parityCorpus(t)
	var run parityRun
	if testing.Short() {
		run = shortParityRun(t)
	} else {
		run = runParityCorpus(t, mc, shippedTemplateNames(t), corpus)
	}
	for _, problem := range run.Problems {
		t.Error(problem)
	}

	// A template that does not resolve is the same single finding in both.
	missing := deckSpecVerdicts(t, mc, map[string]any{"spec": corpus["pitch"], "template": "definitely-not-a-template"})
	assertFindingParity(t, "missing template", missing)
	if missing.Validate.OK || len(missing.Validate.Findings) != 1 || !strings.HasSuffix(missing.Validate.Findings[0].Code, "TEMPLATE_NOT_FOUND") {
		t.Errorf("missing template: %+v", missing.Validate.Findings)
	}
}

// TestDeckSpecRawSlideContractParity is the go-slide-creator-uon4b acceptance
// test: every contract error render raises for a raw slide is raised by
// validate first, at a path inside the raw slide.
func TestDeckSpecRawSlideContractParity(t *testing.T) {
	mc := refusalTestConfig(t)
	for _, c := range rawContractSlides {
		t.Run(c.name, func(t *testing.T) {
			v := deckSpecVerdicts(t, mc, map[string]any{"spec": rawContractSpec(c.slide), "template": "midnight-blue"})
			assertFindingParity(t, c.name, v)
			if v.Validate.OK {
				t.Fatalf("validate passed a raw slide render refuses (render error %q): %+v", v.Render.Error, v.Validate.Findings)
			}
			if v.RenderEnvelope == nil && v.Render.OK {
				t.Fatal("fixture no longer fails at render")
			}
			inside := false
			for _, f := range v.Validate.Findings {
				path := findingPath(f)
				if f.Severity == diagnostics.SeverityError && strings.HasPrefix(path, c.wantPath) {
					inside = true
				}
			}
			if !inside {
				t.Errorf("no blocking finding at or under %s: %+v", c.wantPath, v.Validate.Findings)
			}
		})
	}
}

// go-slide-creator-x9rhq: what blocks is an error, and it is named — code and
// path — in the blocking reason. An info never stops the gate.
func TestDeckSpecBlockingFindingIsNamedError(t *testing.T) {
	mc := refusalTestConfig(t)
	v := deckSpecVerdicts(t, mc, map[string]any{"spec": decodeSpecObject(t, pitchSpec("")), "template": "warm-coral"})
	assertFindingParity(t, "pitch", v)
	if v.Validate.OK || v.Render.DeterministicReady == nil || *v.Render.DeterministicReady {
		t.Fatalf("a deck with storyline gaps passed: validate ok=%v render=%+v", v.Validate.OK, v.Render.DeterministicBlockingReasons)
	}
	if !v.Render.OK {
		t.Fatalf("render must still write the deck: %s", v.Render.Error)
	}
	blocking := map[string]string{}
	for _, d := range v.Render.Diagnostics {
		if d.Blocking {
			blocking[d.Code] = d.SemanticPath
		}
	}
	for _, code := range []string{patterns.ErrCodeNoExecutiveSummary, patterns.ErrCodeClosingWithoutNextSteps} {
		path, ok := blocking[code]
		if !ok || path == "" {
			t.Errorf("%s is not a blocking diagnostic with a DeckSpec path: %v", code, blocking)
			continue
		}
		named := false
		for _, r := range v.Render.DeterministicBlockingReasons {
			if strings.HasPrefix(r, code+" at "+specPointer(path)) {
				named = true
			}
		}
		if !named {
			t.Errorf("blocking reasons do not name %s at %s: %v", code, path, v.Render.DeterministicBlockingReasons)
		}
	}
	if len(v.Render.DeterministicBlockingReasons) != len(blocking) {
		t.Errorf("one reason per blocking code expected: reasons %v, blocking %v", v.Render.DeterministicBlockingReasons, blocking)
	}
}

// A gate criterion no single finding accounts for is itself a finding: one
// QUALITY_GATE error at `slides`, in both tools.
func TestDeckSpecAggregateGateFailureIsAFinding(t *testing.T) {
	mc := refusalTestConfig(t)
	v := deckSpecVerdicts(t, mc, map[string]any{"spec": decodeSpecObject(t, topicTitleSpec), "template": "midnight-blue"})
	assertFindingParity(t, "topic titles", v)
	var gate *semanticDiagnostic
	for i, d := range v.Render.Diagnostics {
		if d.Code == codeQualityGate {
			gate = &v.Render.Diagnostics[i]
		}
		if d.Code == patterns.ErrCodeTitleNotAction && d.Blocking {
			t.Errorf("a single topic title blocks by itself: %+v", d)
		}
	}
	if gate == nil {
		t.Fatalf("no %s diagnostic on a deck of topic titles: %+v (gate %+v)", codeQualityGate, v.Render.Diagnostics, v.Render.Quality.QualityGate)
	}
	if !gate.Blocking || gate.Severity != "error" || gate.SemanticPath != "slides" ||
		!strings.Contains(gate.Message, "lack an action title") || !strings.Contains(gate.Message, "TITLE_NOT_ACTION at /slides/") {
		t.Errorf("gate finding = %+v", *gate)
	}
	if len(v.Render.DeterministicBlockingReasons) != 1 || !strings.HasPrefix(v.Render.DeterministicBlockingReasons[0], codeQualityGate+" at /slides: ") {
		t.Errorf("blocking reasons = %v", v.Render.DeterministicBlockingReasons)
	}

	// Waiving the rule clears the criterion.
	waived := decodeSpecObject(t, topicTitleSpec)
	waived["meta"].(map[string]any)["waivers"] = []any{map[string]any{"code": "TITLE_NOT_ACTION", "reason": "Appendix-style reference deck with label titles."}}
	w := deckSpecVerdicts(t, mc, map[string]any{"spec": waived, "template": "midnight-blue"})
	assertFindingParity(t, "topic titles waived", w)
	if !w.Validate.OK {
		t.Errorf("waiving TITLE_NOT_ACTION did not clear the gate: %+v", w.Validate.Findings)
	}
}

// go-slide-creator-oh3qr: an archetype that does not call for the rule, or an
// explicit waiver with a reason, turns a storyline finding into an advisory,
// and the result records it. Without one the behaviour is unchanged.
func TestDeckSpecStorylineWaivers(t *testing.T) {
	mc := refusalTestConfig(t)
	find := func(diags []semanticDiagnostic, code string) *semanticDiagnostic {
		for i := range diags {
			if diags[i].Code == code {
				return &diags[i]
			}
		}
		return nil
	}

	// The archetype alone waives the executive summary, not the closing.
	arch := deckSpecVerdicts(t, mc, map[string]any{"spec": decodeSpecObject(t, pitchSpec(`,"archetype":"sales_pitch"`)), "template": "warm-coral"})
	assertFindingParity(t, "archetype", arch)
	if d := find(arch.Render.Diagnostics, patterns.ErrCodeNoExecutiveSummary); d == nil || d.Blocking || d.Severity != "info" || !strings.Contains(d.Waived, "sales_pitch") {
		t.Errorf("sales_pitch did not waive NO_EXECUTIVE_SUMMARY: %+v", d)
	}
	if d := find(arch.Render.Diagnostics, patterns.ErrCodeClosingWithoutNextSteps); d == nil || !d.Blocking {
		t.Errorf("an archetype must not waive the closing rule: %+v", d)
	}
	if len(arch.Render.Waivers) != 1 || arch.Render.Waivers[0].Source != "meta.archetype" || arch.Render.Waivers[0].Findings != 1 {
		t.Errorf("render waivers = %+v", arch.Render.Waivers)
	}

	// An explicit waiver with a reason clears the rest, and both tools record it.
	meta := `,"archetype":"sales_pitch","waivers":[{"code":"CLOSING_WITHOUT_NEXT_STEPS","reason":"The brief fixes seven slides; the ask is made verbally."}]`
	full := deckSpecVerdicts(t, mc, map[string]any{"spec": decodeSpecObject(t, pitchSpec(meta)), "template": "warm-coral"})
	assertFindingParity(t, "waived", full)
	if !full.Validate.OK || full.Render.DeterministicReady == nil || !*full.Render.DeterministicReady {
		t.Fatalf("waived pitch still blocked: %v / %+v", full.Render.DeterministicBlockingReasons, full.Validate.Findings)
	}
	for _, list := range [][]findingWaiver{full.Render.Waivers, full.Validate.Waivers} {
		if len(list) != 2 || list[0].Code != "CLOSING_WITHOUT_NEXT_STEPS" || list[0].Source != "meta.waivers" ||
			list[0].Reason != "The brief fixes seven slides; the ask is made verbally." || list[0].Findings != 1 {
			t.Errorf("recorded waivers = %+v", list)
		}
	}
	waivedInEnvelope := false
	for _, f := range full.Validate.Findings {
		if strings.HasSuffix(f.Code, "CLOSING_WITHOUT_NEXT_STEPS") {
			reason, _ := f.Evidence["waived"].(string)
			waivedInEnvelope = reason != "" && f.Severity == diagnostics.SeverityInfo
		}
	}
	if !waivedInEnvelope {
		t.Errorf("validate does not mark the waived finding: %+v", full.Validate.Findings)
	}

	// A waiver must name a waivable code and say why.
	for name, waiver := range map[string]string{
		"not waivable": `{"code":"BODY_TOO_LONG","reason":"We like long text."}`,
		"no reason":    `{"code":"NO_EXECUTIVE_SUMMARY","reason":" "}`,
	} {
		bad := deckSpecVerdicts(t, mc, map[string]any{"spec": decodeSpecObject(t, pitchSpec(`,"waivers":[`+waiver+`]`)), "template": "warm-coral"})
		rejected := false
		for _, f := range bad.Validate.Findings {
			path := findingPath(f)
			if f.Severity == diagnostics.SeverityError && strings.HasPrefix(path, "meta.waivers[0]") {
				rejected = true
			}
		}
		if !rejected || bad.Validate.OK {
			t.Errorf("%s: waiver accepted: %+v", name, bad.Validate.Findings)
		}
	}
}

// go-slide-creator-3rn3s: the root cause carries the blocking severity and its
// symptoms are grouped beneath it, identically in both tools.
func TestDeckSpecRootCauseCarriesSymptoms(t *testing.T) {
	v := overfullOnModernVerdicts(t)
	assertFindingParity(t, "overfull", v)
	if v.Render.OK {
		t.Fatal("fixture no longer refused on modern")
	}
	var root *semanticDiagnostic
	for i, d := range v.Render.Diagnostics {
		switch {
		case d.Code == patterns.ErrCodeBodyTooLong && d.SemanticPath == "slides[1]":
			root = &v.Render.Diagnostics[i]
		case d.Code == patterns.ErrCodeTextBelowReadableMin && d.SlideIndex != nil && *d.SlideIndex == 1:
			t.Errorf("a symptom is still its own blocker beside its root cause: %+v", d)
		}
	}
	if root == nil {
		t.Fatalf("no root-cause finding on the over-full slide: %+v", v.Render.Diagnostics)
	}
	if !root.Blocking || root.Severity != "error" || len(root.Symptoms) < 2 {
		t.Fatalf("root cause = %+v", *root)
	}
	if !strings.Contains(root.Message, "needs") || !strings.Contains(root.Message, "holds") {
		t.Errorf("root cause does not state the deficit: %q", root.Message)
	}
	for _, s := range root.Symptoms {
		// The points, and the takeaway band squeezed with them.
		if s.Code != patterns.ErrCodeTextBelowReadableMin || !(strings.HasPrefix(s.Path, "slides[1].points[") || s.Path == "slides[1].takeaway") {
			t.Errorf("symptom = %+v", s)
		}
	}
	if root.NextToolCall == nil {
		t.Error("root cause offers no next step")
	}
	// The envelope lists the same symptoms under evidence.symptoms.
	for _, f := range v.Validate.Findings {
		if !strings.HasSuffix(f.Code, patterns.ErrCodeBodyTooLong) {
			continue
		}
		symptoms, _ := f.Evidence[symptomsDetail].([]any)
		if len(symptoms) != len(root.Symptoms) {
			t.Errorf("validate lists %d symptoms, render %d", len(symptoms), len(root.Symptoms))
		}
		return
	}
	t.Errorf("validate reports no root cause: %+v", v.Validate.Findings)
}

// go-slide-creator-2dit4: validate measures on the template the deck renders
// on, says which one and why, and warns when nothing pins it.
func TestValidateDeckSpecEchoesAndWarnsAboutTemplate(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := templateSensitiveSpec(t)

	// The journey case: the same spec is clean on the archetype default and
	// over-full on modern. Validate must say what it measured on.
	def := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	if !def.OK || def.Template != "forest-green" || def.TemplateSource != "archetype default" {
		t.Errorf("unpinned validate: ok=%v template %q source %q", def.OK, def.Template, def.TemplateSource)
	}
	if len(def.Warnings) != 1 || !strings.Contains(def.Warnings[0], "pins no template") || !strings.Contains(def.Warnings[0], "forest-green") {
		t.Errorf("unpinned validate did not warn: %v", def.Warnings)
	}
	// One validate of the over-full deck serves the echo and the parity with
	// its render: each is a search for the cut that clears it
	// (go-slide-creator-q7cpq). templateSensitiveSpec is the over-full deck,
	// so the pair is the one the test binary asks once.
	onModern := overfullOnModernVerdicts(t)
	modern := onModern.Validate
	if modern.Template != "modern" || modern.TemplateSource != "template argument" || modern.OK {
		t.Errorf("template=modern: template %q source %q ok=%v", modern.Template, modern.TemplateSource, modern.OK)
	}

	// The render on modern says what that validate said.
	assertFindingParity(t, "modern", onModern)

	// A pinned spec needs no warning, and an argument it overrides is called out.
	pinned := templateSensitiveSpec(t)
	pinned["meta"].(map[string]any)["template"] = "midnight-blue"
	pin := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": pinned}))
	if pin.Template != "midnight-blue" || pin.TemplateSource != "meta.template" || len(pin.Warnings) != 0 {
		t.Errorf("pinned validate: template %q source %q warnings %v", pin.Template, pin.TemplateSource, pin.Warnings)
	}
	// go-slide-creator-ifkxs: the call's template replaces the spec's pin for
	// that call, and the response says so; the deck stays bound to the pin.
	pinnedOnModern := deckSpecVerdicts(t, mc, map[string]any{"spec": pinned, "template": "modern"})
	over := pinnedOnModern.Validate
	if over.Template != "modern" || over.TemplateSource != "template argument" || over.OK {
		t.Errorf("template argument beside a pin: template %q source %q ok=%v", over.Template, over.TemplateSource, over.OK)
	}
	if len(over.Warnings) != 1 || !strings.Contains(over.Warnings[0], `overrides meta.template "midnight-blue" for this call only`) {
		t.Errorf("the response does not say the pin was overridden: %v", over.Warnings)
	}
	if h, ok := mc.deckHandles.Load(over.DeckID); !ok || h.Template != "midnight-blue" {
		t.Errorf("the deck is no longer bound to its pinned template: %+v", h)
	}
	assertFindingParity(t, "pinned, rendered on modern", pinnedOnModern)
	again := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": over.DeckID}))
	if again.Template != "midnight-blue" || !again.OK {
		t.Errorf("the next call without the argument is not back on the pin: template %q ok=%v", again.Template, again.OK)
	}
}

// go-slide-creator-ifkxs: one validate call says on which templates a spec
// renders cleanly.
func TestValidateDeckSpecAcrossTemplates(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := templateSensitiveSpec(t)
	spec["meta"].(map[string]any)["template"] = "midnight-blue"

	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "templates": []any{"midnight-blue", "modern"}}))
	if env.Template != "midnight-blue" || !env.OK || len(env.TemplateResults) != 2 {
		t.Fatalf("template %q ok=%v results %+v", env.Template, env.OK, env.TemplateResults)
	}
	own, modern := env.TemplateResults[0], env.TemplateResults[1]
	if own.Template != "midnight-blue" || !own.OK || len(own.Findings) != 0 {
		t.Errorf("the spec's own template is not reported clean: %+v", own)
	}
	if modern.Template != "modern" || modern.OK || len(modern.Findings) == 0 {
		t.Fatalf("modern is not reported as refusing the spec: %+v", modern)
	}
	alone := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "modern"}))
	want := map[string]bool{}
	for _, f := range alone.Findings {
		if f.Severity != diagnostics.SeverityInfo {
			want[f.Code+" "+*f.Path] = true
		}
	}
	for _, f := range modern.Findings {
		if !want[f.Code+" "+f.Path] || f.Message == "" || !strings.HasPrefix(f.Path, "/") {
			t.Errorf("modern's entry lists a finding a validate on modern does not report: %+v", f)
		}
		delete(want, f.Code+" "+f.Path)
	}
	if len(want) != 0 {
		t.Errorf("modern's entry leaves out blocking findings or warnings: %v", want)
	}
	if h, ok := mc.deckHandles.Load(env.DeckID); !ok || h.Template != "midnight-blue" {
		t.Errorf("checking other templates rebound the deck: %+v", h)
	}

	// "all" is every template shipped with the server. With the spec above it
	// is a search for the cut on each template that refuses it, nine times the
	// call above: the short race run asks for a deck of one title slide, which
	// names the same nine, and the integration corpus job for the spec itself
	// (go-slide-creator-efhg2).
	onAll := spec
	if testing.Short() {
		onAll = map[string]any{"meta": map[string]any{"title": "Margin plan", "template": "midnight-blue"}, "slides": []any{
			map[string]any{"kind": "title", "title": "Margin plan lifts EBITDA by two points", "subtitle": "Board, October 2026"},
		}}
	}
	all := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": onAll, "templates": []any{"all"}}))
	if len(all.TemplateResults) != len(embeddedTemplateNames()) {
		t.Errorf("all: %d results for %d shipped templates", len(all.TemplateResults), len(embeddedTemplateNames()))
	}
	for _, r := range all.TemplateResults {
		if r.Template == "" || r.Summary == "" {
			t.Errorf("an entry does not name its template or its result: %+v", r)
		}
	}

	// A name that is not a template is that entry's finding, not the call's.
	bad := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "templates": []any{"no-such-template"}}))
	if !bad.OK || len(bad.TemplateResults) != 1 || bad.TemplateResults[0].OK || len(bad.TemplateResults[0].Findings) != 1 ||
		!strings.HasSuffix(bad.TemplateResults[0].Findings[0].Code, "TEMPLATE_NOT_FOUND") {
		t.Errorf("unknown template: ok=%v results %+v", bad.OK, bad.TemplateResults)
	}
}

// A deck handle's template changes only by patch: a later call naming another
// template is measured on it for that call alone.
func TestDeckHandleTemplateChangesOnlyByPatch(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := decodeSpecObject(t, pitchSpec(`,"archetype":"sales_pitch","waivers":[{"code":"CLOSING_WITHOUT_NEXT_STEPS","reason":"Fixed seven-slide brief."}]`))
	first := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": "midnight-blue"}))
	if first.DeckID == "" || first.Template != "midnight-blue" {
		t.Fatalf("first validate: %+v", first)
	}
	bound := func() string {
		h, ok := mc.deckHandles.Load(first.DeckID)
		if !ok {
			t.Fatal("handle lost")
		}
		return h.Template
	}
	if bound() != "midnight-blue" {
		t.Fatalf("the first template named did not bind the handle: %q", bound())
	}

	// A one-off render on another template.
	other := renderDeckSpecCall(t, mc, map[string]any{"deck_id": first.DeckID, "template": "forest-green"})
	if !other.Success || other.Template != "forest-green" {
		t.Fatalf("one-off render: %s (template %q)", other.Error, other.Template)
	}
	if len(other.Warnings) == 0 || !strings.Contains(other.Warnings[0], "stays bound") {
		t.Errorf("one-off render did not say the deck keeps its template: %v", other.Warnings)
	}
	if bound() != "midnight-blue" {
		t.Errorf("a template argument rebound the handle to %q", bound())
	}

	// Later calls by deck_id are back on the bound template, and say so.
	again := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": first.DeckID}))
	if again.Template != "midnight-blue" || again.TemplateSource != "deck_id" || len(again.Warnings) != 0 {
		t.Errorf("validate by deck_id: template %q source %q warnings %v", again.Template, again.TemplateSource, again.Warnings)
	}

	// The explicit patch changes it.
	patched := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{
		"deck_id": first.DeckID,
		"patch":   []any{map[string]any{"op": "add", "path": "/meta/template", "value": "forest-green"}},
	}))
	if patched.Template != "forest-green" || patched.TemplateSource != "meta.template" || bound() != "forest-green" {
		t.Errorf("patching /meta/template: template %q source %q handle %q", patched.Template, patched.TemplateSource, bound())
	}
}

// get_started's examples carry the template on validate as well as render.
func TestGetStartedValidateExampleNamesTemplate(t *testing.T) {
	for _, tool := range []string{"validate_deck_spec", "render_deck_spec"} {
		if _, ok := argsTemplateFor(tool)["template"]; !ok {
			t.Errorf("get_started args_template for %s has no template", tool)
		}
	}
}

// go-slide-creator-x9rhq: `semantic render` exits 0 exactly when the deck is
// written and no blocking finding remains, "ok" agrees with the exit code, and
// `semantic validate` reaches the same verdict on the same findings.
func TestSemanticCLIExitFollowsBlockingFindings(t *testing.T) {
	type cliResult struct {
		OK          bool                 `json:"ok"`
		OutputPath  string               `json:"output_path"`
		Error       string               `json:"error"`
		Diagnostics []semanticDiagnostic `json:"diagnostics"`
		Waivers     []findingWaiver      `json:"waivers"`
	}
	cases := []struct {
		name    string
		meta    string
		blocked bool
	}{
		{"blocked", `,"template":"warm-coral"`, true},
		{"waived", `,"template":"warm-coral","archetype":"sales_pitch","waivers":[{"code":"CLOSING_WITHOUT_NEXT_STEPS","reason":"Fixed seven-slide brief."}]`, false},
	}
	for _, c := range cases {
		for _, mode := range []string{"strict", "off"} {
			t.Run(c.name+"/output-validation="+mode, func(t *testing.T) {
				path := writeSpec(t, "pitch.json", pitchSpec(c.meta))
				out := filepath.Join(t.TempDir(), "pitch.pptx")
				stdout, err := runSemanticArgs(t, "render", "--spec", path, "--output", out,
					"--templates-dir", testTemplatesDir, "--output-validation", mode)
				var res cliResult
				if jerr := json.Unmarshal([]byte(stdout), &res); jerr != nil {
					t.Fatalf("render output: %v\n%s", jerr, stdout)
				}
				if _, statErr := os.Stat(out); statErr != nil {
					t.Fatalf("deck not written: %v", statErr)
				}
				if (err != nil) != c.blocked || res.OK == c.blocked {
					t.Fatalf("exit error=%v ok=%v, want blocked=%v", err, res.OK, c.blocked)
				}
				blocking := 0
				for _, d := range res.Diagnostics {
					if d.Blocking {
						blocking++
						if !strings.Contains(res.Error, d.Code+" at "+specPointer(d.SemanticPath)) {
							t.Errorf("error does not name %s at %s: %q", d.Code, d.SemanticPath, res.Error)
						}
					}
				}
				if (blocking > 0) != c.blocked {
					t.Errorf("%d blocking diagnostics, blocked=%v", blocking, c.blocked)
				}

				vout, verr := runSemanticArgs(t, "validate", "--spec", path, "--templates-dir", testTemplatesDir)
				var env semanticValidateEnvelope
				if jerr := json.Unmarshal([]byte(vout), &env); jerr != nil {
					t.Fatalf("validate output: %v\n%s", jerr, vout)
				}
				if (verr != nil) != c.blocked || env.OK == c.blocked {
					t.Errorf("validate exit error=%v ok=%v, want blocked=%v", verr, env.OK, c.blocked)
				}
				if got, want := envelopeKeys(t, "cli validate", env.FindingEnvelope), diagnosticKeys(t, "cli render", res.Diagnostics); !equalFindingKeys(got, want) {
					t.Errorf("CLI validate and render disagree\n  validate: %s\n  render:   %s", formatKeys(got), formatKeys(want))
				}
				if env.Template != "warm-coral" || env.TemplateSource != "meta.template" {
					t.Errorf("validate template %q source %q", env.Template, env.TemplateSource)
				}
				if len(env.Waivers) != len(res.Waivers) {
					t.Errorf("waivers differ: validate %+v render %+v", env.Waivers, res.Waivers)
				}
			})
		}
	}
}

// `semantic validate --template` measures on the template render will use.
func TestSemanticValidateCLITemplateFlag(t *testing.T) {
	raw, err := json.Marshal(templateSensitiveSpec(t))
	if err != nil {
		t.Fatal(err)
	}
	path := writeSpec(t, "sensitive.json", string(raw))
	for tpl, wantOK := range map[string]bool{"midnight-blue": true, "modern": false} {
		out, err := runSemanticArgs(t, "validate", "--spec", path, "--template", tpl, "--templates-dir", testTemplatesDir)
		var env semanticValidateEnvelope
		if jerr := json.Unmarshal([]byte(out), &env); jerr != nil {
			t.Fatalf("%s: %v\n%s", tpl, jerr, out)
		}
		if env.OK != wantOK || (err == nil) != wantOK {
			t.Errorf("%s: ok=%v err=%v, want ok=%v: %s", tpl, env.OK, err, wantOK, env.Summary)
		}
		if env.Template != tpl || env.TemplateSource != "template argument" || len(env.Warnings) != 1 {
			t.Errorf("%s: template %q source %q warnings %v", tpl, env.Template, env.TemplateSource, env.Warnings)
		}
	}
}

// describe_finding states, for a code, whether the DeckSpec tools report it as
// a blocking error — the same rule that sets a finding's severity.
func TestDescribeFindingStatesBlockingProfile(t *testing.T) {
	want := map[string]string{
		"NO_EXECUTIVE_SUMMARY":       "sometimes",
		"CLOSING_WITHOUT_NEXT_STEPS": "sometimes",
		"TEXT_BELOW_READABLE_MIN":    "sometimes",
		"BODY_TOO_LONG":              "sometimes",
		"QUALITY_GATE":               "always",
		"SLIDE_UNDERUSED":            "never",
		"TITLE_NOT_ACTION":           "sometimes",
	}
	for code, blocks := range want {
		res, err := handleDescribeFinding(context.Background(), makeRequest(map[string]any{"code": code}))
		if err != nil || res.IsError {
			t.Fatalf("%s: describe_finding failed: %v %s", code, err, textContent(res))
		}
		var out struct {
			Code       string `json:"code"`
			Severity   string `json:"severity"`
			Blocks     string `json:"blocks"`
			BlocksWhen string `json:"blocks_when"`
		}
		structuredInto(t, res.StructuredContent, &out)
		if out.Blocks != blocks || out.BlocksWhen == "" || out.Severity == "" {
			t.Errorf("%s: blocks=%q (want %q) when=%q severity=%q", code, out.Blocks, blocks, out.BlocksWhen, out.Severity)
		}
	}
}
