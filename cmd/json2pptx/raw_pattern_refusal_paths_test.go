package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A readability refusal on a raw pattern slide names the value and the text
// (go-slide-creator-llxzd).
//
// Three agent runs met TEXT_BELOW_READABLE_MIN they could not act on: on a
// raw_json2pptx slide it sat at /slides/N/slide with the generated grid cell
// in debug and no text, and the value behind it surfaced only one per render;
// on two DeckSpec slides it was one entry for both, "measured": null.

const rawRoadmapPhasedSlide = `{"slide_type":"content","layout_id":"blank-title","content":[{"placeholder_id":"title","type":"text","text_value":"Four phases and two parallel tracks finish before the 30 June 2027 deadline"}],"pattern":{"name":"roadmap-phased","values":{"phases":["Jul-Aug 2026","Sep-Nov 2026","Dec 26-Mar 27","Apr-Jun 2027"],"workstreams":[{"name":"Programme phases","bars":[{"label":"Mobilise","start":"Jul-Aug 2026","end":"Jul-Aug 2026"},{"label":"Design","start":"Sep-Nov 2026","end":"Sep-Nov 2026"},{"label":"Build","start":"Dec 26-Mar 27","end":"Dec 26-Mar 27"},{"label":"Embed","start":"Apr-Jun 2027","end":"Apr-Jun 2027"}]},{"name":"Data and reporting platform","bars":[{"label":"Platform build and integrated report","start":"Sep-Nov 2026","end":"Apr-Jun 2027"}]},{"name":"Training 1,200 control owners","bars":[{"label":"Training waves","start":"Dec 26-Mar 27","end":"Apr-Jun 2027"}]}]}}}`

const rawPhaseRoadmapSlide = `{"slide_type":"content","layout_id":"blank-title","content":[{"placeholder_id":"title","type":"text","text_value":"Three phases decommission the warehouse by the end of 2028"}],"pattern":{"name":"phase-roadmap","values":{"phases":[{"name":"Foundation","date_label":"Q1-Q2 2027","description":"Landing zone, governance model and FinOps guardrails; 3 pilot sources live on the lakehouse.","active":true,"milestone":"Pilot sources live"},{"name":"Migrate","date_label":"Q3 2027-Q2 2028","description":"All 14 sources onboarded by CDC; 1,900 ETL jobs refactored into 400 governed pipelines.","milestone":"14 sources, 400 pipelines"},{"name":"Optimise","date_label":"Q3-Q4 2028","description":"Decommission the 2009 warehouse; tune compute and hand over to run.","milestone":"Warehouse decommissioned"}],"parallel_label":"In parallel","parallel_tracks":["Data literacy programme for 300 analysts","FinOps guardrails: budgets, cluster policies, showback"]}}}`

// readabilityFindings are a validate envelope's TEXT_BELOW_READABLE_MIN
// findings, as decoded JSON.
func readabilityFindings(t *testing.T, structured any) []map[string]any {
	t.Helper()
	var env struct {
		Findings []map[string]any `json:"findings"`
	}
	structuredInto(t, structured, &env)
	var out []map[string]any
	for _, f := range env.Findings {
		if code, _ := f["code"].(string); strings.HasSuffix(code, "TEXT_BELOW_READABLE_MIN") {
			out = append(out, f)
		}
	}
	return out
}

// factList is an evidence fact of a finding that stands for n paths: one
// value for all of them, or a list in the order of paths.
func factList(v any, n int) []any {
	if list, ok := v.([]any); ok && len(list) == n {
		return list
	}
	out := make([]any, n)
	for i := range out {
		out[i] = v
	}
	return out
}

// resolveSpecPointer reads the value at a JSON Pointer of spec.
func resolveSpecPointer(t *testing.T, spec map[string]any, pointer string) (any, bool) {
	t.Helper()
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	node, _, whole := descend(doc, pointerTokens(pointer))
	return node, whole
}

func TestRawPatternReadabilityRefusalNamesValueAndText(t *testing.T) {
	cases := []struct {
		name, template, slide string
		chrome, optional      bool
	}{
		{"roadmap-phased workstream name (g-A5)", "midnight-blue", rawRoadmapPhasedSlide, false, false},
		{"phase-roadmap phase (h-A6)", "modern-template", rawPhaseRoadmapSlide, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mc := semanticTestConfig(t)
			spec := rawContractSpec(c.slide)
			if c.chrome {
				spec["meta"] = map[string]any{"title": "Raw probe", "chrome": map[string]any{
					"client_name": "Atlas Retail", "confidentiality": "Confidential", "page_numbers": map[string]any{"enabled": true},
				}}
				spec["slides"].([]any)[1].(map[string]any)["source"] = "Atlas Retail migration plan, September 2026"
			}
			for _, tool := range []string{"validate_deck_spec", "render_deck_spec"} {
				args := map[string]any{"spec": spec, "template": c.template}
				var structured any
				if tool == "validate_deck_spec" {
					res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(args))
					if err != nil {
						t.Fatal(err)
					}
					structured = res.StructuredContent
				} else {
					res, err := mc.handleRenderDeckSpec(context.Background(), makeRequest(args))
					if err != nil {
						t.Fatal(err)
					}
					var probe struct {
						Diagnostics []map[string]any `json:"diagnostics"`
					}
					structuredInto(t, res.StructuredContent, &probe)
					structured = map[string]any{"findings": probe.Diagnostics}
				}
				findings := readabilityFindings(t, structured)
				if len(findings) == 0 {
					if c.optional {
						// The phase boxes of this pattern are sized by the template's
						// content area; the roadmap-phased case pins the contract.
						t.Logf("%s: no TEXT_BELOW_READABLE_MIN finding on %s", tool, c.template)
						continue
					}
					t.Fatalf("%s: no TEXT_BELOW_READABLE_MIN finding; the fixture no longer overflows", tool)
				}
				seen := map[string]bool{}
				for _, f := range findings {
					path, _ := f["path"].(string)
					paths := []string{path}
					if list, ok := f["paths"].([]any); ok {
						paths = paths[:0]
						for _, p := range list {
							paths = append(paths, p.(string))
						}
					}
					evidence, _ := f["evidence"].(map[string]any)
					texts := factList(evidence["text"], len(paths))
					sizes := factList(evidence["measured"], len(paths))
					for i, path := range paths {
						if !strings.Contains(path, "/slide/pattern/values/") {
							t.Errorf("%s: finding at %q, want a pointer into /slides/1/slide/pattern/values (message %q)", tool, path, f["message"])
							continue
						}
						if seen[path] {
							t.Errorf("%s: %s reported twice", tool, path)
						}
						seen[path] = true
						value, ok := resolveSpecPointer(t, spec, path)
						if !ok {
							t.Errorf("%s: path %q does not resolve in the spec", tool, path)
							continue
						}
						text, _ := texts[i].(string)
						if text == "" {
							t.Errorf("%s: finding at %s carries no evidence.text: %v", tool, path, evidence)
							continue
						}
						if s, isString := value.(string); isString && s != text {
							t.Errorf("%s: path %s holds %q, evidence.text is %q", tool, path, s, text)
						}
						if measured, _ := sizes[i].(map[string]any); measured["font_pt"] == nil {
							t.Errorf("%s: finding at %s carries no measured.font_pt: %v", tool, path, evidence)
						}
					}
				}
			}
		})
	}
}

// Every unreadable value of a slide is in the first response: the agent that
// shortened the one text a refusal quoted met the next on the following
// render.
func TestRawPatternReadabilityReportsEveryHitAtOnce(t *testing.T) {
	mc := semanticTestConfig(t)
	spec := rawContractSpec(rawRoadmapPhasedSlide)
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec, "template": "midnight-blue"}))
	if err != nil {
		t.Fatal(err)
	}
	texts := map[string]bool{}
	for _, f := range readabilityFindings(t, res.StructuredContent) {
		evidence, _ := f["evidence"].(map[string]any)
		n := 1
		if list, ok := f["paths"].([]any); ok {
			n = len(list)
		}
		for _, text := range factList(evidence["text"], n) {
			s, _ := text.(string)
			texts[s] = true
		}
	}
	// The two workstream names are unreadable; their bar labels were too
	// until the sparse block's rows were grown on a slide of its own
	// (shapegrid ComposeZoom, go-slide-creator-cyyiy).
	for _, want := range []string{"Data and reporting platform", "Training 1,200 control owners"} {
		if !texts[want] {
			t.Errorf("no finding quotes %q; got %v", want, texts)
		}
	}
}

func TestCollapsedReadabilityStaysOnOneSlide(t *testing.T) {
	facts := func(pt float64) map[string]any { return map[string]any{"font_pt": pt} }
	at := func(path string, slide int, measured any) semanticDiagnostic {
		d := semanticDiagnostic{
			Code: "TEXT_BELOW_READABLE_MIN", Severity: "error", Blocking: true, Action: "refuse",
			Message: "text renders at 10.6pt, below the 12pt minimum", SemanticPath: path, SlideIndex: &slide,
		}
		d.diag = semanticDiagFromCompile(envelopeDiagnostics([]semanticDiagnostic{d})[0]).diag
		d.diag.Details = map[string]any{"allowed": map[string]any{"min_font_pt": 12.0}}
		if measured != nil {
			d.diag.Details["measured"] = measured
		}
		return d
	}
	out := collapseDiagnostics([]semanticDiagnostic{
		at("slides[2].regions[2].bullets", 2, facts(10.6)),
		at("slides[4].eyebrow", 4, facts(9.4)),
		at("slides[2].regions[2].bullets[0]", 2, nil),
	})
	if len(out) != 2 {
		t.Fatalf("got %d entries, want one per slide: %+v", len(out), out)
	}
	for _, d := range out {
		for _, m := range d.members {
			if slideContainer(m.Path) != slideContainer(d.SemanticPath) {
				t.Errorf("entry at %s holds %s from another slide", d.SemanticPath, m.Path)
			}
		}
		if v, has := d.diag.Details["measured"]; has && v == nil {
			t.Errorf("entry at %s has measured: null", d.SemanticPath)
		}
	}
	if got := out[1].diag.Details["measured"]; got == nil {
		t.Errorf("the second slide's entry lost its measurement")
	}
}
