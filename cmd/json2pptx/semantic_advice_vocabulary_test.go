package main

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// The go-slide-creator-micna acceptance scan: every remedy a DeckSpec finding
// states — its message, its symptoms, its remediation and recommended edit —
// names fields of the slide kind it is raised on, or none. The cold-start
// journey was told to "choose dots or chevron style" on a timeline (the kind
// has no style), to hide a legend with show_legend, and to cap a grid with
// bounds / max_height_pct.

// fieldLikeRE matches what reads as a field name in advice: a snake_case
// word.
var fieldLikeRE = regexp.MustCompile(`\b[a-z]+(?:_[a-z0-9]+)+\b`)

// rawSurfaceRE matches controls and locators of the raw deck and of the
// renderer that a slide kind does not have.
var rawSurfaceRE = regexp.MustCompile(`\bshape_grid\b|\bcompose\b|process-grid-2row|phase-roadmap|\bbounds\b|\boverrides?\b|chevron|\b[a-z_]+\[\d+\]\.[a-z_]+|explain_deck_spec|stacked-box|card-title|card-body|autofit|plain-bullet|rendered_shapes|continuation slides`)

// adviceWords are snake_case words advice may use that are not fields of a
// kind: tools and their arguments, deck-level fields, and the names of the
// quality gate's criteria.
var adviceWords = map[string]bool{
	"list_slide_kinds": true, "validate_deck_spec": true, "render_deck_spec": true, "describe_finding": true,
	"list_templates": true, "deck_id": true, "raw_json2pptx": true,
	"min_score": true, "max_topic_title_pct": true, "max_problem_slide_pct": true, "min_composition_score": true,
	"did_you_mean": true, "accent_strategy": true, "type_scale": true, "required_layouts": true,
	// The values of the regions kind's arrangement field.
	"main_left": true, "main_right": true, "main_top": true, "main_bottom": true,
}

// adviceTexts collects every sentence and string fact of a finding's remedy.
func adviceTexts(f map[string]any) []string {
	var out []string
	add := func(v any) {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	add(f["message"])
	symptoms, _ := f["symptoms"].([]any)
	if ev, ok := f["evidence"].(map[string]any); ok {
		if list, ok := ev["symptoms"].([]any); ok {
			symptoms = append(symptoms, list...)
		}
	}
	for _, s := range symptoms {
		if m, ok := s.(map[string]any); ok {
			add(m["message"])
		}
	}
	params := func(holder any) {
		m, _ := holder.(map[string]any)
		add(m["hint"])
		p, _ := m["params"].(map[string]any)
		for key, v := range p {
			// A params key is a fact's name, held to the same vocabulary.
			out = append(out, key)
			add(v)
		}
	}
	if r, ok := f["remediation"].(map[string]any); ok {
		params(r["primary"])
	}
	params(f["recommended_edit"])
	return out
}

// remedyParamNames are the names of the facts a remediation carries.
var remedyParamNames = func() map[string]bool {
	out := map[string]bool{}
	for _, name := range remedyFactKeys {
		out[name] = true
	}
	return out
}()

func TestDeckSpecAdviceNamesOnlyFieldsOfTheKind(t *testing.T) {
	mc := refusalTestConfig(t)
	corpus := patchHarnessCorpus(t)
	for name, spec := range addressCorpus(t) {
		if !strings.HasPrefix(name, "raw/") {
			corpus["address/"+name] = map[string]any{"spec": spec, "template": "modern"}
		}
	}
	// Every kind's own example, on each template of the long run.
	corpus["every kind"] = map[string]any{"spec": parityCorpus(t)["all-kinds"], "template": "modern"}
	// go-slide-creator-d6wvb, -d0g2j: a row of bare steps or milestones alone
	// on its slide draws SPARSE_SINGLE_ROW_FLOW, whose pattern-level remedy
	// named numbered-step-strip, process-grid-2row, phase-roadmap and compose.
	for name, slide := range sparseFlowSlides {
		corpus[name] = map[string]any{"template": "modern", "spec": decodeSpecObject(t,
			`{"meta":{"title":"Flow","source":"Illustrative"},"slides":[{"kind":"title","title":"A flow that states its claim","subtitle":"October 2026"},`+slide+`]}`)}
	}
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		// The short run scans the review's decks and one kind past its count;
		// the long run every kind's counts and the address corpus, on three
		// templates.
		if _, sparse := sparseFlowSlides[name]; testing.Short() && !shortHarnessDeck(name) && !sparse {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	templates := []string{""}
	tools := map[string]mcpHandler{"validate": mc.handleValidateDeckSpec}
	wantChecked := 40
	// The short run scans the first validate of the short patch harness's
	// decks, on the template the harness names: the harness has that response,
	// from the same spec and arguments in the same test binary, so it is read
	// from there instead of validating each deck again
	// (go-slide-creator-q7cpq).
	firstValidate := map[string]any{}
	if testing.Short() {
		firstValidate = shortPatchHarnessRun(t).FirstValidate
	} else {
		templates = []string{"", "midnight-blue", "modern-template"}
		tools["render"] = mc.handleRenderDeckSpec
		wantChecked = 150
	}
	checked, reused := 0, 0
	for _, name := range names {
		for _, override := range templates {
			args := map[string]any{"spec": corpus[name]["spec"]}
			if tpl, _ := corpus[name]["template"].(string); tpl != "" {
				args["template"] = tpl
			}
			if override != "" {
				args["template"] = override
			}
			raw, _ := json.Marshal(args["spec"])
			var doc map[string]any
			_ = json.Unmarshal(raw, &doc)
			slides, _ := doc["slides"].([]any)
			for tool, handler := range tools {
				structured, have := firstValidate[name]
				if have = have && tool == "validate" && override == ""; have {
					reused++
				} else {
					res, err := handler(context.Background(), makeRequest(args))
					if err != nil {
						t.Fatal(err)
					}
					structured = res.StructuredContent
				}
				var out map[string]any
				structuredInto(t, structured, &out)
				for _, key := range []string{"findings", "diagnostics"} {
					list, _ := out[key].([]any)
					for _, entry := range list {
						f := entry.(map[string]any)
						path, _ := f["path"].(string)
						toks := pointerTokens(path)
						if len(toks) < 2 || toks[0] != "slides" {
							continue // a deck-level finding
						}
						idx := -1
						if n, err := json.Number(toks[1]).Int64(); err == nil {
							idx = int(n)
						}
						if idx < 0 || idx >= len(slides) {
							continue
						}
						slide, _ := slides[idx].(map[string]any)
						kind, _ := slide["kind"].(string)
						vocabulary := semantic.PayloadVocabulary(semantic.SlideKind(kind))
						if kind == string(semantic.KindRawJSON2pptx) || vocabulary == nil {
							continue // the author of a raw slide wrote the raw surface
						}
						known := map[string]bool{}
						for _, v := range vocabulary {
							known[v] = true
						}
						for _, k := range semantic.AllSlideKinds() {
							known[string(k)] = true
						}
						code, _ := f["code"].(string)
						for _, text := range adviceTexts(f) {
							checked++
							// What the author wrote is quoted; only the advice is held
							// to the kind's vocabulary.
							plain := quotedRE.ReplaceAllString(text, `""`)
							if m := rawSurfaceRE.FindString(plain); m != "" {
								t.Errorf("%s/%s %s on a %s slide names %q, which the kind does not have: %s", name, tool, code, kind, m, text)
							}
							for _, word := range fieldLikeRE.FindAllString(plain, -1) {
								if known[word] || adviceWords[word] || remedyParamNames[word] || strings.Contains(strings.ToLower(code), word) {
									continue
								}
								t.Errorf("%s/%s %s on a %s slide names %q, which is not a field of the kind: %s", name, tool, code, kind, word, text)
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%d advice texts checked; %d responses read from the short patch harness", checked, reused)
	if testing.Short() && reused < len(firstValidate) {
		t.Errorf("%d of the short patch harness's %d first responses were read; the two short corpora have drifted apart, and decks are being validated twice", reused, len(firstValidate))
	}
	if checked < wantChecked {
		t.Fatalf("only %d advice texts checked; the corpus no longer exercises the findings", checked)
	}
	// The sparse rows draw the finding the scan is there for, and the remedy
	// its sentence states clears it.
	sparse := func(slide string) bool {
		spec := decodeSpecObject(t, `{"meta":{"title":"Flow","source":"Illustrative"},"slides":[{"kind":"title","title":"A flow that states its claim","subtitle":"October 2026"},`+slide+`]}`)
		res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"spec": spec, "template": "modern"}))
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		structuredInto(t, res.StructuredContent, &out)
		list, _ := out["findings"].([]any)
		for _, entry := range list {
			if code, _ := entry.(map[string]any)["code"].(string); strings.HasSuffix(code, "SPARSE_SINGLE_ROW_FLOW") {
				return true
			}
		}
		return false
	}
	for name, slide := range sparseFlowSlides {
		if !sparse(slide) {
			t.Errorf("%s: no SPARSE_SINGLE_ROW_FLOW; the scan no longer covers its wording", name)
		}
		if sparse(sparseFlowRemedied[name]) {
			t.Errorf("%s: the remedy the finding states does not clear it", name)
		}
	}
}

// sparseFlowSlides are the slide kinds that compile to a single row of short
// boxes alone on the slide; sparseFlowRemedied is each after the remedy its
// SPARSE_SINGLE_ROW_FLOW states.
var sparseFlowSlides = map[string]string{
	"sparse process with an explicit flow": `{"kind":"process","title":"Four steps take a request to production","takeaway":"Approval is the only step that waits on a person.","pattern":"process-flow","steps":[{"label":"Request"},{"label":"Review"},{"label":"Approve"},{"label":"Deploy"}]}`,
	"sparse timeline":                      `{"kind":"timeline","title":"Four milestones take the plan to production","takeaway":"Every quarter of 2026 ends on a milestone.","milestones":[{"label":"Plan","date":"Q1 2026"},{"label":"Build","date":"Q2 2026"},{"label":"Ship","date":"Q3 2026"},{"label":"Scale","date":"Q4 2026"}]}`,
}

var sparseFlowRemedied = map[string]string{
	"sparse process with an explicit flow": `{"kind":"process","title":"Four steps take a request to production","takeaway":"Approval is the only step that waits on a person.","steps":[{"label":"Request","description":"A team files the change with its risk class"},{"label":"Review","description":"Two engineers read the diff and the rollout plan"},{"label":"Approve","description":"The service owner signs off within a day"},{"label":"Deploy","description":"The pipeline rolls it out region by region"}]}`,
	"sparse timeline":                      `{"kind":"timeline","title":"Four milestones take the plan to production","takeaway":"Every quarter of 2026 ends on a milestone.","milestones":[{"label":"Plan","date":"Q1 2026","body":"Scope and budget agreed with the board"},{"label":"Build","date":"Q2 2026","body":"Core services built and tested with pilots"},{"label":"Ship","date":"Q3 2026","body":"General availability in the first two regions"},{"label":"Scale","date":"Q4 2026","body":"Remaining regions and the partner channel"}]}`,
}
