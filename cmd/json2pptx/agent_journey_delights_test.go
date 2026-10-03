package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/templates"
)

// The delights of the agent-journey review, kept (go-slide-creator-3pxl6).
//
// tests/quality/results/agent-journey-20261003/report.json lists what the five
// agents said worked noticeably well. Each one that a handler call can show is
// a check here; the rest say why not. TestAgentJourneyDelights fails when the
// report lists a delight this table does not know, so a later journey's
// delights are either put under test or declined in writing.

// journeyDelight is one delight of the report: the check that keeps it, or
// the reason no cheap check can.
type journeyDelight struct {
	Persona string
	Title   string
	// Check names the entry of delightChecks that keeps it.
	Check string
	// NotChecked says why there is no check, and what covers the behaviour
	// instead when something does.
	NotChecked string
}

const (
	needsRender = "needs a LibreOffice render (render_deck_thumbnails); the image-hash protocol is covered with a render-cache seam by the submit_visual_review tests (mcp_submit_visual_review_test.go) and by the journey harness itself"
	wallClock   = "a wall-clock claim: a timing assertion flakes under -race and on a loaded CI runner; the journey harness records seconds per call (mcpd.py summary)"
	visualOnly  = "a judgement about how the rendered slide looks; only the journey harness (an agent looking at the thumbnails) can make it"
)

var journeyDelights = []journeyDelight{
	// a-coldstart
	{Persona: "a-coldstart", Title: "deck_id + patch makes repair nearly free", Check: "deck_id_and_patch"},
	{Persona: "a-coldstart", Title: "Semantic validation errors are exact and one-step fixable", Check: "exact_errors_in_one_pass"},
	{Persona: "a-coldstart", Title: "Speed: the whole loop is sub-second apart from thumbnails", NotChecked: wallClock},
	{Persona: "a-coldstart", Title: "Copy-ready examples: zero unknown-field errors while authoring, first render wrote a real deck", Check: "catalogue_examples_first_try"},
	{Persona: "a-coldstart", Title: "Pixel content hashes make the review protocol honest and cheap", NotChecked: needsRender},
	{Persona: "a-coldstart", Title: "Good defaults without asking: highlighted final bar and data labels, takeaway band, source line, date footer and page numbers", Check: "good_defaults"},
	{Persona: "a-coldstart", Title: "Refuses to shrink text below 12pt instead of shipping unreadable slides, and names the field", Check: "refuses_unreadable_text"},
	{Persona: "a-coldstart", Title: "list_templates compact is small and useful", Check: "template_names_small"},

	// b-discovery
	{Persona: "b-discovery", Title: "Chart and diagram candidates come with a data_contract and a next_tool_call that runs verbatim", Check: "candidate_runs_verbatim"},
	{Persona: "b-discovery", Title: "Semantic kinds for org, 8 quotes, option matrix, funnel+insights and regions were valid on the first attempt and looked right", Check: "kinds_valid_first_try"},
	{Persona: "b-discovery", Title: "`regions` kind: chart + stat + timeline in one slide from ~15 lines", Check: "regions_kind"},
	{Persona: "b-discovery", Title: "Render-time unknown-key errors name the key, why it matters and the accepted keys", Check: "unknown_key_named"},
	{Persona: "b-discovery", Title: "show_pattern is an authoritative contract: closed schema, min/max items, per-count character budgets, example_values", Check: "show_pattern_contract"},
	{Persona: "b-discovery", Title: "validate_pattern answers a wrong key with did_you_mean, a patch and a pointer to show_pattern", Check: "validate_pattern_did_you_mean"},
	{Persona: "b-discovery", Title: "Single-slide render loop is fast and returns a real image", NotChecked: needsRender},
	{Persona: "b-discovery", Title: "Capacity strings on candidates ('3-8', 'objective + 3-5 pillars + foundation') and limits in kind summaries let me check item counts before authoring", Check: "capacity_before_authoring"},
	{Persona: "b-discovery", Title: "journey-maturity-model 'We are here' marker and value-chain highlight render exactly as intended", NotChecked: visualOnly},

	// c-repair
	{Persona: "c-repair", Title: "deck_id + patch makes repair rounds tiny and tells me what to re-inspect", Check: "deck_id_and_patch"},
	{Persona: "c-repair", Title: "Every validate finding carries a path into my spec, and dropped content is called out as DROPPED", Check: "paths_and_dropped"},
	{Persona: "c-repair", Title: "Chart series/category mismatch caught before render with exact counts", Check: "chart_series_mismatch"},
	{Persona: "c-repair", Title: "Title, source and closing fixes worked first time; meta.source cleared four findings with one op", Check: "meta_source_one_op"},
	{Persona: "c-repair", Title: "Thumbnails -> review hand-off is prefilled and the review was accepted first try", NotChecked: needsRender},
	{Persona: "c-repair", Title: "Fast: validate <0.25s, render 0.3-0.45s, 12 thumbnails ~2s", NotChecked: wallClock},
	{Persona: "c-repair", Title: "Same spec rendered on midnight-blue and p-style with no layout breakage", Check: "same_spec_across_templates"},
	{Persona: "c-repair", Title: "describe_finding for PATTERN_DEGRADED explains from/to/reason and says content is never lost", Check: "describe_pattern_degraded"},

	// d-cli
	{Persona: "d-cli", Title: "First DeckSpec was valid on the first try and rendered a good 7-slide deck in 0.3 s; thumbnails took 3 s", Check: "first_spec_valid"},
	{Persona: "d-cli", Title: "Schema descriptions state counts, character budgets and degradation behaviour per kind; `semantic explain` shows the chosen pattern, layout and alternatives per slide", Check: "kind_descriptions_and_explanation"},
	{Persona: "d-cli", Title: "Errors are specific, list everything in one pass and suggest the fix", Check: "exact_errors_in_one_pass"},
	{Persona: "d-cli", Title: "Templates are embedded, so the CLI works with no --templates-dir", Check: "templates_embedded"},
	{Persona: "d-cli", Title: "Output is deterministic and stdout JSON is never mixed with log lines", Check: "deterministic_output"},
	{Persona: "d-cli", Title: "Placeholder copy is caught (WEAK_CONTENT) and capabilities carries a correct MCP-to-CLI mapping; top-level help lists MCP-only tools with CLI workarounds", Check: "weak_content_and_cli_map"},

	// e-revise
	{Persona: "e-revise", Title: "deck_id + patch held state across 30+ calls; a whole-deck template switch was one 169-byte call", Check: "template_switch_one_call"},
	{Persona: "e-revise", Title: "changed_slides + next_tool_call with slide_indices prefilled: one changed slide re-inspected in about 1.5 s and 37 KB instead of 268 KB", Check: "deck_id_and_patch"},
	{Persona: "e-revise", Title: "Patch errors are precise, and the patch is atomic", Check: "patch_atomic"},
	{Persona: "e-revise", Title: "Unknown arguments are rejected with the accepted list rather than ignored", Check: "unknown_argument_rejected"},
	{Persona: "e-revise", Title: "First render succeeded on the first attempt from catalog examples; turns 1, 4, 5, 6 and 8 each rendered on their first call", Check: "catalogue_examples_first_try"},
	{Persona: "e-revise", Title: "Thumbnails are content-addressed and cached: an old revision's slide came back in 0.01 s and its hash proved the title slide had not changed", NotChecked: needsRender},
	{Persona: "e-revise", Title: "Appendix handling just worked: section with appendix:true is unnumbered and the slide after it is paged 'A1'; notes landed in all ten notes slides", Check: "appendix_and_notes"},
	{Persona: "e-revise", Title: "The slide list (index, kind, title) in every render response is a cheap way to confirm what sits at each index after a structural patch", Check: "slide_list_in_render"},
	{Persona: "e-revise", Title: "submit_visual_review verified all ten image hashes against the server's own render in one call", NotChecked: needsRender},
	{Persona: "e-revise", Title: "Switching template surfaced template-specific findings with a ready patch", NotChecked: "which findings a template raises follows its fonts and content area, so a fixed expectation breaks between macOS and the Linux CI; that every offered patch clears its finding is TestEveryEmittedPatchClearsItsFinding, and the one-call switch is checked under the e-revise deck_id delight"},
}

func TestAgentJourneyDelights(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tests", "quality", "results", "agent-journey-20261003", "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Delights []struct {
			Persona string `json:"persona"`
			Title   string `json:"title"`
		} `json:"delights"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, d := range journeyDelights {
		listed[d.Persona+": "+d.Title] = true
		if (d.Check == "") == (d.NotChecked == "") {
			t.Errorf("%s: %s needs a check or a reason it has none, not both", d.Persona, d.Title)
		}
	}
	for _, d := range report.Delights {
		if !listed[d.Persona+": "+d.Title] {
			t.Errorf("the report's delight is neither checked nor declined: %s: %s", d.Persona, d.Title)
		}
	}
	if len(journeyDelights) != len(report.Delights) {
		t.Errorf("%d delights in the table, %d in the report", len(journeyDelights), len(report.Delights))
	}

	// The cold-start deck validated and rendered once, for the checks that
	// read that response or its file.
	coldStart.mc = refusalTestConfig(t)
	coldStart.validate = deckSpecEnvelope(t, mustCall(t, coldStart.mc.handleValidateDeckSpec, map[string]any{"spec": coldStartSpec(t), "template": delightTemplate}))
	coldStart.render = renderDeckSpecCall(t, coldStart.mc, map[string]any{"deck_id": coldStart.validate.DeckID})
	if !coldStart.render.Success {
		t.Fatalf("the cold-start deck does not render: %q", coldStart.render.Error)
	}

	// A check that keeps several delights runs once.
	ran := map[string]bool{}
	unchecked := 0
	for _, d := range journeyDelights {
		if d.Check == "" {
			unchecked++
			t.Logf("not checked: %s: %s (%s)", d.Persona, d.Title, d.NotChecked)
			continue
		}
		check, ok := delightChecks[d.Check]
		if !ok {
			t.Errorf("%s: %s: no check named %q", d.Persona, d.Title, d.Check)
			continue
		}
		if ran[d.Check] {
			continue
		}
		ran[d.Check] = true
		t.Run(d.Check, check)
	}
	for name := range delightChecks {
		if !ran[name] {
			t.Errorf("check %q keeps no delight", name)
		}
	}
	t.Logf("%d delights, %d checks, %d not checked", len(journeyDelights), len(ran), unchecked)
}

// --- helpers ---------------------------------------------------------------

// delightChecks are the checks, by subtest name.
var delightChecks = map[string]func(t *testing.T){
	"deck_id_and_patch":                 delightDeckIDPatch,
	"exact_errors_in_one_pass":          delightExactErrors,
	"catalogue_examples_first_try":      delightExamplesFirstTry,
	"good_defaults":                     delightGoodDefaults,
	"refuses_unreadable_text":           delightRefusesUnreadable,
	"template_names_small":              delightTemplateNames,
	"candidate_runs_verbatim":           delightCandidateRunsVerbatim,
	"kinds_valid_first_try":             delightKindsValidFirstTry,
	"regions_kind":                      delightRegions,
	"unknown_key_named":                 delightUnknownKey,
	"show_pattern_contract":             delightShowPattern,
	"validate_pattern_did_you_mean":     delightValidatePattern,
	"capacity_before_authoring":         delightCapacity,
	"paths_and_dropped":                 delightPathsAndDropped,
	"chart_series_mismatch":             delightChartMismatch,
	"meta_source_one_op":                delightMetaSource,
	"same_spec_across_templates":        delightSameSpecAcrossTemplates,
	"describe_pattern_degraded":         delightDescribeDegraded,
	"first_spec_valid":                  delightFirstSpecValid,
	"kind_descriptions_and_explanation": delightKindDescriptions,
	"templates_embedded":                delightTemplatesEmbedded,
	"deterministic_output":              delightDeterministic,
	"weak_content_and_cli_map":          delightWeakContent,
	"template_switch_one_call":          delightTemplateSwitch,
	"patch_atomic":                      delightPatchAtomic,
	"unknown_argument_rejected":         delightUnknownArgument,
	"appendix_and_notes":                delightAppendixAndNotes,
	"slide_list_in_render":              delightSlideList,
}

// structuredText is a tool result's structured content as JSON text.
func structuredText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// wantAll reports each needle the text lacks.
func wantAll(t *testing.T, what, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s does not contain %q:\n%s", what, n, shorten(text, 1500))
		}
	}
}

// twoSlideDeck wraps one slide in a deck with a title slide and a source.
func twoSlideDeck(slide map[string]any) map[string]any {
	return map[string]any{
		"meta": map[string]any{"title": "Journey delight", "source": "Illustrative"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Margin plan lifts EBITDA by two points", "subtitle": "Board, October 2026"},
			slide,
		},
	}
}

// errorsUnder lists the blocking findings at or under a path prefix.
func errorsUnder(env deckSpecEnvelopeResponse, prefix string) []string {
	var out []string
	for _, f := range env.Findings {
		if f.Severity != diagnostics.SeverityError {
			continue
		}
		for _, p := range pathsOf(f) {
			if strings.HasPrefix(p, prefix) {
				out = append(out, f.Code+" at "+p+": "+f.Message)
				break
			}
		}
	}
	return out
}

const delightTemplate = "midnight-blue"

// coldStart is the cold-start deck's first validate and first render on
// delightTemplate, made by TestAgentJourneyDelights before its checks run.
var coldStart struct {
	mc       *mcpConfig
	validate deckSpecEnvelopeResponse
	render   renderDeckSpecResponse
}

// --- checks ----------------------------------------------------------------

// A repair is the deck_id and one op; the response says which slide changed
// and hands back the thumbnail call for that slide alone.
func delightDeckIDPatch(t *testing.T) {
	mc := refusalTestConfig(t)
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": coldStartSpec(t), "template": delightTemplate}))
	if env.DeckID == "" || !env.Stored {
		t.Fatalf("validate returned no stored deck_id: %+v", env)
	}
	args := map[string]any{"deck_id": env.DeckID, "patch": []any{map[string]any{"op": "replace", "path": "/slides/2/kpis/0/value", "value": "€48.3M"}}}
	if sent, _ := json.Marshal(args); len(sent) > 200 {
		t.Errorf("a one-value repair is a %d-byte request", len(sent))
	}
	// The first render of the stored deck; the repair is measured against it.
	if first := renderDeckSpecCall(t, mc, map[string]any{"deck_id": env.DeckID}); !first.Success || len(first.ChangedSlides) != 7 {
		t.Fatalf("first render: success=%v changed_slides=%v %q", first.Success, first.ChangedSlides, first.Error)
	}
	render := renderDeckSpecCall(t, mc, args)
	if !render.Success || render.DeckID != env.DeckID {
		t.Fatalf("deck_id + patch did not render: %q", render.Error)
	}
	if len(render.ChangedSlides) != 1 || render.ChangedSlides[0] != 2 {
		t.Errorf("changed_slides = %v, want [2]", render.ChangedSlides)
	}
	if render.NextToolCall == nil || render.NextToolCall.Tool != "render_deck_thumbnails" {
		t.Fatalf("next_tool_call = %+v, want render_deck_thumbnails", render.NextToolCall)
	}
	next, _ := json.Marshal(render.NextToolCall.ArgsTemplate)
	wantAll(t, "next_tool_call", string(next), `"slide_indices":[2]`, `"pptx_path"`)
}

// Every missing field of every slide comes back in one response, each at its
// slide with the field named.
func delightExactErrors(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := map[string]any{"meta": map[string]any{"title": "T", "source": "S"}, "slides": []any{
		map[string]any{"kind": "title", "title": "Margin plan lifts EBITDA by two points", "subtitle": "Board"},
		map[string]any{"kind": "kpi_snapshot", "title": "Three KPIs beat plan in Q3"},
		map[string]any{"kind": "option_matrix", "title": "Option B wins on payback and risk"},
	}}
	res := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": delightTemplate})
	if env := deckSpecEnvelope(t, res); env.OK {
		t.Fatal("a deck with three missing required fields validates")
	}
	wantAll(t, "the response", structuredText(t, res),
		`"missing_path":"/slides/1/kpis"`, `"missing_path":"/slides/2/criteria"`, `"missing_path":"/slides/2/options"`,
		`requires a \"kpis\" (or \"metrics\") field`)
}

// A deck of every kind's catalogue example has no field the compiler does not
// know or lacks, and renders to a file.
func delightExamplesFirstTry(t *testing.T) {
	mc := refusalTestConfig(t)
	slides := []any{}
	for _, k := range semantic.AllSlideKinds() {
		slides = append(slides, semantic.KindExample(k))
	}
	spec := map[string]any{"meta": map[string]any{"title": "Every kind", "source": "Illustrative"}, "slides": slides}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": delightTemplate}))
	for _, f := range env.Findings {
		for _, code := range []string{diagnostics.CodeSemanticUnknownField, "SEMANTIC_REQUIRED", "UNKNOWN_ENUM", "SEMANTIC_UNKNOWN_KIND"} {
			if strings.HasSuffix(f.Code, code) {
				t.Errorf("a catalogue example raises %s at %v: %s", f.Code, pathsOf(f), f.Message)
			}
		}
	}
	render := renderDeckSpecCall(t, mc, map[string]any{"spec": spec, "template": delightTemplate})
	if !render.Success || render.SlideCount < len(slides) {
		t.Errorf("the deck of examples did not render: success=%v, %d slides, %q", render.Success, render.SlideCount, render.Error)
	}
	if _, err := os.Stat(render.PptxPath); err != nil {
		t.Errorf("no file at pptx_path: %v", err)
	}
}

// What the agent did not ask for and got: data labels on the chart, the
// takeaway band, the source line, the date footer and page numbers. (That the
// last bar is the highlighted one is a colour in the picture; the harness
// looks at it.)
func delightGoodDefaults(t *testing.T) {
	render := coldStart.render
	chartSlide := readZipEntry(t, render.PptxPath, "ppt/slides/slide4.xml")
	wantAll(t, "the chart slide", chartSlide,
		"Four straight quarters of growth put FY26 ahead of plan.", // takeaway band
		"Northwind management accounts, Q3 FY26",                   // meta.source as the source line
		`type="slidenum"`,                                          // page number
		"October 2026",                                             // meta.date footer
	)
	if media := readZipText(t, render.PptxPath, "ppt/media/"); !strings.Contains(media, "48.2") {
		t.Error("the chart carries no data label for its last bar (48.2)")
	}
	if title := readZipEntry(t, render.PptxPath, "ppt/slides/slide1.xml"); strings.Contains(title, `type="slidenum"`) {
		t.Error("the title slide carries a page number")
	}
}

// An over-full slide is refused with the field to cut, not shrunk until it
// fits.
func delightRefusesUnreadable(t *testing.T) {
	mc := refusalTestConfig(t)
	res := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": decodeSpecObject(t, overfullExecSummarySpec), "template": "modern"})
	env := deckSpecEnvelope(t, res)
	if env.OK {
		t.Fatal("the over-full executive summary validates on modern")
	}
	if under := errorsUnder(env, "/slides/1"); len(under) == 0 {
		t.Errorf("no blocking finding on the over-full slide: %+v", env.Findings)
	}
	// The readable minimum, and a field of the slide to cut.
	wantAll(t, "the response", structuredText(t, res), `"min_font_pt":12`, "/slides/1/points/")
}

// The template listing an agent picks a name from is one small row each.
func delightTemplateNames(t *testing.T) {
	mc := refusalTestConfig(t)
	mc.templatesDir = shippedTemplatesDir(t)
	res := mustCall(t, mc.handleListTemplates, map[string]any{"fields": "names"})
	var listing struct {
		Templates []struct {
			Name      string `json:"name"`
			TitleFont string `json:"title_font"`
			BodyFont  string `json:"body_font"`
		} `json:"templates"`
	}
	structuredInto(t, res.StructuredContent, &listing)
	if len(listing.Templates) < 9 {
		t.Fatalf("%d templates listed, want the 9 shipped", len(listing.Templates))
	}
	for _, tpl := range listing.Templates {
		if tpl.Name == "" || tpl.TitleFont == "" || tpl.BodyFont == "" {
			t.Errorf("a row lacks its name or fonts: %+v", tpl)
		}
	}
	if size := onboardingBytes(t, res); size > 2*1024 {
		t.Errorf("the names listing is %d bytes", size)
	}
}

// recommend_visual's first candidate for a funnel says what data it takes and
// carries a call that renders as given.
func delightCandidateRunsVerbatim(t *testing.T) {
	mc := refusalTestConfig(t)
	res := mustCall(t, mc.handleRecommendVisual, map[string]any{"intent": "sales funnel with conversion rates", "template": delightTemplate})
	var rec struct {
		Candidates []struct {
			Category     string         `json:"category"`
			Name         string         `json:"name"`
			DataContract map[string]any `json:"data_contract"`
			NextToolCall *struct {
				Tool         string         `json:"tool"`
				ArgsTemplate map[string]any `json:"args_template"`
			} `json:"next_tool_call"`
		} `json:"candidates"`
	}
	structuredInto(t, res.StructuredContent, &rec)
	if len(rec.Candidates) == 0 {
		t.Fatal("no candidates")
	}
	top := rec.Candidates[0]
	if top.Category != "chart" || top.Name != "funnel" {
		t.Errorf("first candidate is %s %s, want the funnel chart", top.Category, top.Name)
	}
	if top.DataContract["description"] == nil || top.DataContract["required_keys"] == nil {
		t.Errorf("the candidate has no data_contract: %+v", top.DataContract)
	}
	if top.NextToolCall == nil || top.NextToolCall.Tool != "render_deck_spec" {
		t.Fatalf("next_tool_call = %+v", top.NextToolCall)
	}
	if render := renderDeckSpecCall(t, mc, top.NextToolCall.ArgsTemplate); !render.Success {
		t.Errorf("the candidate's call does not render as given: %q", render.Error)
	}
}

// The kinds the discovery persona authored from their examples alone.
func delightKindsValidFirstTry(t *testing.T) {
	mc := refusalTestConfig(t)
	for _, kind := range []semantic.SlideKind{"org", "quote", "option_matrix", "chart_insight", "regions"} {
		env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": twoSlideDeck(semantic.KindExample(kind)), "template": delightTemplate}))
		if under := errorsUnder(env, "/slides/1"); len(under) > 0 {
			t.Errorf("%s: its example is refused: %v", kind, under)
		}
	}
}

// Three views under one title from a short slide.
func delightRegions(t *testing.T) {
	mc := refusalTestConfig(t)
	slide := semantic.KindExample("regions")
	regions, _ := slide["regions"].([]any)
	if len(regions) < 2 {
		t.Fatalf("the regions example has %d regions", len(regions))
	}
	if raw, _ := json.Marshal(slide); len(raw) > 1500 {
		t.Errorf("the regions example is %d bytes; it was about fifteen lines", len(raw))
	}
	render := renderDeckSpecCall(t, mc, map[string]any{"spec": twoSlideDeck(slide), "template": delightTemplate})
	if !render.Success || render.SlideCount != 2 {
		t.Errorf("regions did not render as one slide: success=%v, %d slides, %q", render.Success, render.SlideCount, render.Error)
	}
}

// unknownKeySpec has a key the kind does not read, on the slide and on a list
// entry.
func unknownKeySpec() map[string]any {
	return twoSlideDeck(map[string]any{"kind": "kpi_snapshot", "title": "Two KPIs beat plan in Q3", "colour": "red", "takeaway": "Both are ahead.",
		"kpis": []any{map[string]any{"label": "Revenue", "value": "48M", "stile": "x"}, map[string]any{"label": "Margin", "value": "14%"}}})
}

// Render names the unknown key, says its content is dropped and lists the
// keys the kind takes.
func delightUnknownKey(t *testing.T) {
	mc := refusalTestConfig(t)
	render := renderDeckSpecCall(t, mc, map[string]any{"spec": unknownKeySpec(), "template": delightTemplate})
	var messages []string
	for _, d := range render.Diagnostics {
		if d.Code == diagnostics.CodeSemanticUnknownField {
			messages = append(messages, d.SemanticPath+": "+d.Message)
		}
	}
	all := strings.Join(messages, "\n")
	wantAll(t, "render's unknown-field diagnostics", all, `"colour"`, `"stile"`, "expected one of", `"kpis"`, `"delta"`)
}

// Every finding of the twelve-flaw draft points into the spec, and a dropped
// key is called DROPPED.
func delightPathsAndDropped(t *testing.T) {
	mc := refusalTestConfig(t)
	first, _ := twelveFlawFirstResponse(t, mc)
	for _, f := range first.Findings {
		if f.Path == nil || !strings.HasPrefix(*f.Path, "/") {
			t.Errorf("%s has no JSON Pointer path: %v", f.Code, f.Path)
		}
	}
	res := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": unknownKeySpec(), "template": delightTemplate})
	wantAll(t, "the unknown-key findings", structuredText(t, res), "DROPPED", `"path":"/slides/1/colour"`, `"path":"/slides/1/kpis/0/stile"`)
}

// show_pattern is the contract of a pattern: a closed schema with item bounds
// and an example.
func delightShowPattern(t *testing.T) {
	res, err := handleShowPattern(context.Background(), makeRequest(map[string]any{"name": "card-grid"}))
	if err != nil || res.IsError {
		t.Fatalf("show_pattern: %v %s", err, resultText(res))
	}
	wantAll(t, "show_pattern card-grid", structuredText(t, res), `"additionalProperties":false`, `"minItems"`, `"maxItems"`, `"maxLength"`, `"example_values"`)
}

// A misspelled key of a pattern gets the key it meant and the tool that shows
// the contract.
func delightValidatePattern(t *testing.T) {
	mc := refusalTestConfig(t)
	res, err := mc.handleValidatePattern(context.Background(), makeRequest(map[string]any{"name": "card-grid", "values": map[string]any{"cardz": []any{map[string]any{"title": "A", "body": "b"}}}}))
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, "validate_pattern", structuredText(t, res), `"did_you_mean":"cells"`, `"action":"apply_patch"`, `"tool":"show_pattern"`)
}

// How many items a visual takes is known before it is authored.
func delightCapacity(t *testing.T) {
	mc := refusalTestConfig(t)
	res := mustCall(t, mc.handleRecommendVisual, map[string]any{"intent": "eight customer quotes", "template": delightTemplate})
	wantAll(t, "recommend_visual", structuredText(t, res), `"name":"quote-cluster"`, `"capacity":"3-8"`)
	var catalogue struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, mustCall(t, mc.handleListSlideKinds, map[string]any{}).StructuredContent, &catalogue)
	bounds := regexp.MustCompile(`\d+–\d+`)
	for _, k := range catalogue.SlideKinds {
		if k.Kind == "next_steps" || k.Kind == "timeline" || k.Kind == "team" {
			if !bounds.MatchString(k.Summary) {
				t.Errorf("the catalogue line of %s states no count: %q", k.Kind, k.Summary)
			}
		}
	}
}

// A series one value short is an error at the series, with both counts.
func delightChartMismatch(t *testing.T) {
	mc := refusalTestConfig(t)
	slide := map[string]any{"kind": "chart_insight", "title": "Revenue grew 12% in a year", "takeaway": "Growth held.",
		"chart": map[string]any{"type": "bar", "data": map[string]any{"categories": []any{"Q1", "Q2", "Q3", "Q4"}, "series": []any{map[string]any{"name": "Revenue", "values": []any{1, 2, 3}}}}}}
	res := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": twoSlideDeck(slide), "template": delightTemplate})
	if deckSpecEnvelope(t, res).OK {
		t.Fatal("a chart with 4 categories and 3 values validates")
	}
	wantAll(t, "the response", structuredText(t, res), "CHART_SERIES_LENGTH_MISMATCH", `"path":"/slides/1/chart/data/series/0/values"`, "4 categories", "3 value(s)")
}

// One op on meta.source answers every slide that cites no source.
func delightMetaSource(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := coldStartSpec(t)
	delete(spec["meta"].(map[string]any), "source")
	unsourced := func(env deckSpecEnvelopeResponse) (n int) {
		for _, f := range env.Findings {
			if strings.HasSuffix(f.Code, "DATA_WITHOUT_SOURCE") {
				n += len(pathsOf(f))
			}
		}
		return n
	}
	before := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": delightTemplate}))
	if unsourced(before) < 2 {
		t.Fatalf("the deck without meta.source has %d unsourced data slides, want several: %s", unsourced(before), before.Summary)
	}
	after := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": before.DeckID,
		"patch": []any{map[string]any{"op": "add", "path": "/meta/source", "value": "Northwind management accounts, Q3 FY26"}}}))
	if n := unsourced(after); n != 0 {
		t.Errorf("%d slides still cite no source after meta.source was set", n)
	}
}

// The cold-start deck has nothing blocking on three shipped templates. (That
// it also looks right on each is the harness's to see; p-style is local and
// not in the repository.)
func delightSameSpecAcrossTemplates(t *testing.T) {
	mc := refusalTestConfig(t)
	if !coldStart.validate.OK {
		t.Errorf("%s: %s", delightTemplate, coldStart.validate.Summary)
	}
	for _, tpl := range []string{"forest-green", "warm-coral"} {
		env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": coldStartSpec(t), "template": tpl}))
		if !env.OK {
			t.Errorf("%s: %s: %v", tpl, env.Summary, errorsUnder(env, "/"))
		}
	}
}

// describe_finding says what a degraded pattern means and that no content is
// lost.
func delightDescribeDegraded(t *testing.T) {
	res, err := handleDescribeFinding(context.Background(), makeRequest(map[string]any{"code": "SEMANTIC_PATTERN_DEGRADED"}))
	if err != nil || res.IsError {
		t.Fatalf("describe_finding: %v %s", err, resultText(res))
	}
	wantAll(t, "describe_finding", structuredText(t, res), "from is the pattern that was refused", "to is what renders instead", "reason is why", "the content is never lost")
}

// A first DeckSpec written from the catalogue validates and renders ready.
func delightFirstSpecValid(t *testing.T) {
	env, render := coldStart.validate, coldStart.render
	if !env.OK {
		t.Errorf("validate: %s", env.Summary)
	}
	if !render.Success || render.SlideCount != 7 || render.DeterministicReady == nil || !*render.DeterministicReady {
		t.Errorf("render: success=%v, %d slides, ready=%v, %q", render.Success, render.SlideCount, render.DeterministicReady, render.Error)
	}
}

// A kind's description states its count and what it degrades to, its brief
// the text budgets; a render says which pattern and layout each slide got.
func delightKindDescriptions(t *testing.T) {
	mc := refusalTestConfig(t)
	res := mustCall(t, mc.handleListSlideKinds, map[string]any{"kinds": []any{"executive_summary"}, "fields": []any{"brief"}})
	wantAll(t, "list_slide_kinds executive_summary", structuredText(t, res), "3–5 points", "degrades to a bullet list", `"budgets"`)

	render := coldStart.render
	if render.Explanation == nil || len(render.Explanation.Slides) != 7 {
		t.Fatalf("no per-slide explanation: %+v", render.Explanation)
	}
	for _, s := range render.Explanation.Slides {
		if s.Layout == "" {
			t.Errorf("slide %d (%s) names no layout", s.Index, s.Kind)
		}
		if s.Kind == "executive_summary" && s.Pattern != "exec-summary" {
			t.Errorf("the executive summary's pattern is %q", s.Pattern)
		}
	}
}

// The binary carries its templates.
func delightTemplatesEmbedded(t *testing.T) {
	entries, err := fs.ReadDir(templates.Embedded, ".")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".pptx") {
			names[strings.TrimSuffix(e.Name(), ".pptx")] = true
		}
	}
	if len(names) < 9 || !names["midnight-blue"] || !names["forest-green"] {
		t.Errorf("embedded templates: %v", names)
	}
}

// The same spec renders the same bytes. (That the CLI keeps log lines off
// stdout needs the built binary; the d-cli persona audits it.)
func delightDeterministic(t *testing.T) {
	first := coldStart.render
	second := renderDeckSpecCall(t, refusalTestConfig(t), map[string]any{"spec": coldStartSpec(t), "template": delightTemplate})
	if !first.Success || first.ContentHash == "" || first.ContentHash != second.ContentHash {
		t.Errorf("two renders of one spec: %q and %q", first.ContentHash, second.ContentHash)
	}
}

// Placeholder copy blocks, and capabilities says which CLI command stands in
// for each MCP tool.
func delightWeakContent(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := twoSlideDeck(map[string]any{"kind": "next_steps", "title": "__FILL__",
		"actions": []any{map[string]any{"action": "Hire the team", "owner": "COO", "date": "Q4"}, map[string]any{"action": "Run the pilot", "owner": "COO", "date": "Q1"}}})
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": delightTemplate}))
	blocked := false
	for _, f := range env.Findings {
		if strings.HasSuffix(f.Code, "SEMANTIC_WEAK_CONTENT") && f.Severity == diagnostics.SeverityError {
			blocked = true
		}
	}
	if env.OK || !blocked {
		t.Errorf("__FILL__ in a title does not block: %s", env.Summary)
	}
	caps := structuredText(t, mustCall(t, mc.handleGetCapabilities, map[string]any{"sections": []any{"tools"}}))
	wantAll(t, "get_capabilities", caps, `"name":"validate_deck_spec"`, `"cli_counterpart":"`)
}

// Moving the whole deck to another template is one small op on the stored
// deck.
func delightTemplateSwitch(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := coldStartSpec(t)
	spec["meta"].(map[string]any)["template"] = delightTemplate
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	args := map[string]any{"deck_id": env.DeckID, "patch": []any{map[string]any{"op": "replace", "path": "/meta/template", "value": "forest-green"}}}
	if sent, _ := json.Marshal(args); len(sent) > 200 {
		t.Errorf("the template switch is a %d-byte request", len(sent))
	}
	render := renderDeckSpecCall(t, mc, args)
	if !render.Success || render.Template != "forest-green" || render.SlideCount != 7 {
		t.Errorf("after the switch: success=%v template=%q slides=%d %q", render.Success, render.Template, render.SlideCount, render.Error)
	}
	if len(render.ChangedSlides) != 7 {
		t.Errorf("a template switch changes every slide; changed_slides = %v", render.ChangedSlides)
	}
}

// A patch with one bad op says which op and why, and stores none of it.
func delightPatchAtomic(t *testing.T) {
	mc := refusalTestConfig(t)
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": coldStartSpec(t), "template": delightTemplate}))
	res, err := mc.handleValidateDeckSpec(context.Background(), makeRequest(map[string]any{"deck_id": env.DeckID, "patch": []any{
		map[string]any{"op": "replace", "path": "/slides/2/kpis/0/value", "value": "CHANGED"},
		map[string]any{"op": "replace", "path": "/slides/19/title", "value": "x"},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("a patch past the last slide is accepted")
	}
	wantAll(t, "the patch error", structuredText(t, res), "patch[1] replace /slides/19/title", "index 19 is outside the array (length 7)")
	stored := structuredText(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"deck_id": env.DeckID, "read": "spec"}))
	if strings.Contains(stored, "CHANGED") || !strings.Contains(stored, "€48.2M") {
		t.Error("the first op of the refused patch was stored")
	}
}

// An argument the tool does not take is refused with the ones it does.
func delightUnknownArgument(t *testing.T) {
	withToolProfile(t, toolProfileDeckSpec)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileDeckSpec)
	ctx := context.Background()
	s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`))
	resp := s.HandleMessage(ctx, json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"render_deck_spec","arguments":{"deck_id":"deck_x","slide_indexes":[1]}}}`))
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	wantAll(t, "the response", string(raw), "UNKNOWN_PARAMETER", `unknown argument \"slide_indexes\"`, "Accepted arguments:", "deck_id", `"isError":true`)
}

// An appendix divider is unnumbered and the slide after it is paged A1; notes
// reach the notes pages.
func delightAppendixAndNotes(t *testing.T) {
	mc := refusalTestConfig(t)
	spec := map[string]any{"meta": map[string]any{"title": "Appendix check", "source": "Illustrative"}, "slides": []any{
		map[string]any{"kind": "title", "title": "Margin plan lifts EBITDA by two points", "subtitle": "Board, October 2026"},
		map[string]any{"kind": "stat", "title": "Margin is up two points on plan", "value": "14.1%", "label": "EBITDA margin", "notes": "Say why the margin moved."},
		map[string]any{"kind": "section", "title": "Appendix", "appendix": true},
		map[string]any{"kind": "stat", "title": "Churn rose to 3.1% in the SMB segment", "value": "3.1%", "label": "SMB churn", "notes": "Detail by cohort."},
	}}
	render := renderDeckSpecCall(t, mc, map[string]any{"spec": spec, "template": delightTemplate})
	if !render.Success || render.SlideCount != 4 {
		t.Fatalf("render: success=%v, %d slides, %q", render.Success, render.SlideCount, render.Error)
	}
	if divider := readZipEntry(t, render.PptxPath, "ppt/slides/slide3.xml"); strings.Contains(divider, `type="slidenum"`) {
		t.Error("the appendix divider carries a page number")
	}
	if after := readZipEntry(t, render.PptxPath, "ppt/slides/slide4.xml"); !strings.Contains(after, ">A1<") {
		t.Error("the slide after the appendix divider is not paged A1")
	}
	wantAll(t, "the notes pages", readZipText(t, render.PptxPath, "ppt/notesSlides/"), "Say why the margin moved.", "Detail by cohort.")
}

// A render lists what sits at each index: id, kind and, in the explanation,
// the title.
func delightSlideList(t *testing.T) {
	render := coldStart.render
	if len(render.Slides) != 7 {
		t.Fatalf("slides[] has %d entries", len(render.Slides))
	}
	for i, s := range render.Slides {
		if s.Index != i || s.SlideNumber != i+1 || s.Kind == "" || s.ID == "" {
			t.Errorf("slides[%d] = %+v", i, s)
		}
	}
	if render.Explanation == nil || len(render.Explanation.Slides) != 7 || render.Explanation.Slides[6].Title == "" {
		t.Errorf("explanation_summary.slides does not carry the titles: %+v", render.Explanation)
	}
}
