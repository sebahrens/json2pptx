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
	"unicode"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/templates"
)

// fileReference matches a pointer to a Markdown file: an MCP-only agent has
// no SKILL.md, QUALITY.md, WORKFLOW.md or docs/ tree to open.
var fileReference = regexp.MustCompile(`[A-Za-z0-9_/.-]*[A-Za-z0-9_]\.md\b`)

// checkDefaultProfileText fails when a default-profile response names a tool
// its tools/list does not carry without marking it in hidden_tools, or points
// at a Markdown file.
func checkDefaultProfileText(t *testing.T, label, text string, marked []string) {
	t.Helper()
	allowed := map[string]bool{}
	for _, name := range marked {
		allowed[name] = true
	}
	for _, name := range unlistedToolsIn(text) {
		if !allowed[name] {
			t.Errorf("%s names %q, which the default profile does not list and the response does not mark in hidden_tools", label, name)
		}
	}
	if refs := fileReference.FindAllString(text, -1); len(refs) > 0 {
		t.Errorf("%s points at files an MCP-only agent does not have: %v", label, refs)
	}
}

// markedHiddenTools reads hidden_tools.names from a structured result.
func markedHiddenTools(t *testing.T, structured any) []string {
	t.Helper()
	raw, err := json.Marshal(structured)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		HiddenTools *hiddenToolsNote `json:"hidden_tools"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.HiddenTools == nil {
		return nil
	}
	if body.HiddenTools.Note == "" {
		t.Error("hidden_tools carries names but no note saying they are callable")
	}
	return body.HiddenTools.Names
}

// checkDefaultProfileResult applies checkDefaultProfileText to a tool result.
func checkDefaultProfileResult(t *testing.T, label string, res *mcp.CallToolResult) {
	t.Helper()
	checkDefaultProfileText(t, label, resultText(res), markedHiddenTools(t, res.StructuredContent))
}

// TestDefaultProfileResponsesNameOnlyReachableTools is the
// go-slide-creator-7bdn6 acceptance test. It walks what a default-profile
// agent reads — the initialize instructions, tools/list, every get_started
// task, describe_finding for every code, the kind catalogue with every detail
// field, the plan and recommendation responses, the findings of a flawed draft
// and the next_tool_call helpers — and fails on a tool name that is neither
// listed nor marked hidden-but-callable, and on a reference to a skill file.
func TestDefaultProfileResponsesNameOnlyReachableTools(t *testing.T) {
	withToolProfile(t, toolProfileDeckSpec)
	withRenderStatus(t, true, nil)
	mc := refusalTestConfig(t)

	checkDefaultProfileText(t, "initialize instructions", mcpInstructionsFor(true, nil), nil)
	checkDefaultProfileText(t, "initialize instructions (no render tooling)", mcpInstructionsFor(false, []string{"magick"}), nil)

	raw, tools := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileDeckSpec))
	if len(tools) == 0 {
		t.Fatal("no tools listed")
	}
	checkDefaultProfileText(t, "tools/list", string(raw), nil)

	for _, task := range getStartedAvailableTasks() {
		res := mustCall(t, mc.handleGetStarted, map[string]any{"task": task})
		checkDefaultProfileResult(t, "get_started("+task+")", res)
		// Every step is a tool the profile lists: a hidden tool is named in
		// hidden_tools, never recommended as a step.
		var resp getStartedResponse
		structuredInto(t, res.StructuredContent, &resp)
		steps := append(append([]getStartedStep{}, resp.Sequence...), resp.RawSequence...)
		if resp.FastPath != nil {
			steps = append(steps, resp.FastPath.Steps...)
			if !toolIsAdvertised(resp.FastPath.Tool) {
				t.Errorf("get_started(%s) fast_path.tool %q is not listed", task, resp.FastPath.Tool)
			}
			for _, name := range resp.FastPath.FallsBackTo {
				if !toolIsAdvertised(name) {
					t.Errorf("get_started(%s) falls_back_to names unlisted %q", task, name)
				}
			}
		}
		constructors := toolConstructors()
		for _, step := range steps {
			if !toolIsAdvertised(step.Tool) {
				t.Errorf("get_started(%s) recommends %q as a step; the default profile does not list it", task, step.Tool)
				continue
			}
			tool := constructors[step.Tool]()
			var schema struct {
				Properties map[string]any `json:"properties"`
			}
			if len(tool.RawInputSchema) > 0 {
				if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
					t.Fatal(err)
				}
			} else {
				schema.Properties = tool.InputSchema.Properties
			}
			for arg := range step.ArgsTemplate {
				if _, ok := schema.Properties[arg]; !ok {
					t.Errorf("get_started(%s) step %s: args_template names %q, which the tool does not accept", task, step.Tool, arg)
				}
			}
		}
	}

	// get_started(revise) said "send deck_id + patch" beside templates that
	// showed spec.
	var revise getStartedResponse
	structuredInto(t, mustCall(t, mc.handleGetStarted, map[string]any{"task": "revise"}).StructuredContent, &revise)
	if len(revise.Sequence) == 0 || revise.Sequence[0].Tool != "validate_deck_spec" {
		t.Fatalf("revise does not open on validate_deck_spec: %+v", revise.Sequence)
	}
	for _, step := range revise.Sequence[:2] {
		if _, ok := step.ArgsTemplate["deck_id"]; !ok {
			t.Errorf("revise step %s: args_template has no deck_id: %v", step.Tool, step.ArgsTemplate)
		}
		if _, ok := step.ArgsTemplate["spec"]; ok {
			t.Errorf("revise step %s: args_template resends spec: %v", step.Tool, step.ArgsTemplate)
		}
	}
	if _, ok := revise.Sequence[0].ArgsTemplate["patch"].([]any); !ok {
		t.Errorf("revise step 1 does not show a patch: %v", revise.Sequence[0].ArgsTemplate)
	}

	for _, code := range diagnostics.AllDescribableCodes() {
		res, err := handleDescribeFinding(context.Background(), makeRequest(map[string]any{"code": code}))
		if err != nil || res.IsError {
			t.Fatalf("describe_finding(%s): %v %+v", code, err, res)
		}
		checkDefaultProfileResult(t, "describe_finding("+code+")", res)
	}

	checkDefaultProfileResult(t, "list_slide_kinds", mustCall(t, mc.handleListSlideKinds, map[string]any{}))
	checkDefaultProfileResult(t, "list_slide_kinds(all detail)", mustCall(t, mc.handleListSlideKinds,
		map[string]any{"fields": []any{"brief", "item_schema", "item_schema_full", "compositions", "budgets", "example"}}))
	checkDefaultProfileResult(t, "list_templates(names)", mustCall(t, mc.handleListTemplates, map[string]any{"fields": "names"}))
	checkDefaultProfileResult(t, "plan_deck", mustCall(t, mc.handlePlanDeck,
		map[string]any{"brief": "Board update: revenue up 12%, three options to fix margin, next steps", "format": "deckspec"}))
	checkDefaultProfileResult(t, "recommend_visual", mustCall(t, mc.handleRecommendVisual,
		map[string]any{"intent": "compare three vendors on five criteria", "template": "midnight-blue"}))

	// The findings of a flawed draft, with their next_tool_call templates.
	flawed := mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": twelveFlawDraft(t), "strict": "warn"})
	checkDefaultProfileResult(t, "validate_deck_spec(twelve-flaw draft)", flawed)
	var walk func(any)
	walk = func(v any) {
		switch n := v.(type) {
		case map[string]any:
			if call, ok := n["next_tool_call"].(map[string]any); ok {
				if name, _ := call["tool"].(string); name != "" && !toolIsAdvertised(name) {
					t.Errorf("a finding's next_tool_call names %q, which the default profile does not list", name)
				}
			}
			for _, c := range n {
				walk(c)
			}
		case []any:
			for _, c := range n {
				walk(c)
			}
		}
	}
	var generic any
	structuredInto(t, flawed.StructuredContent, &generic)
	walk(generic)

	for name, build := range map[string]func() *patterns.ToolCallSuggestion{
		"nextCallGetInputSchema":     nextCallGetInputSchema,
		"nextCallListTemplates":      nextCallListTemplates,
		"nextCallListPatterns":       nextCallListPatterns,
		"nextCallInspectSlideImages": nextCallInspectSlideImages,
		"nextCallReadPresentation":   func() *patterns.ToolCallSuggestion { return nextCallReadPresentation("/tmp/deck.pptx") },
		"nextCallValidateOutput":     func() *patterns.ToolCallSuggestion { return nextCallValidateOutput("/tmp/deck.pptx") },
	} {
		if s := build(); s != nil && !toolIsAdvertised(s.Tool) {
			t.Errorf("%s suggests %q, which the default profile does not list", name, s.Tool)
		}
	}
}

// The wider profiles keep the raw path in get_started: the projection is for
// the profile that hides it.
func TestGetStartedKeepsTheRawPathWhereItIsListed(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	resp := buildGetStartedResponse("brief", testRenderReady())
	if len(resp.RawSequence) == 0 || resp.HiddenTools != nil {
		t.Errorf("core profile brief: raw_sequence %d steps, hidden_tools %+v", len(resp.RawSequence), resp.HiddenTools)
	}
	for _, note := range append(append([]string{}, resp.Notes...), resp.FastPath.WhenToCall) {
		if fileReference.MatchString(note) {
			t.Errorf("get_started still points at a file: %q", note)
		}
	}
}

// onboardingBytes is the wire size of a tool result's structured content.
func onboardingBytes(t *testing.T, res *mcp.CallToolResult) int {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool returned an error: %s", resultText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	return len(raw)
}

// shippedTemplatesDir writes the templates the binary embeds into a fresh
// directory. A budget measured on the checkout's templates/ grows with a
// local, gitignored template (p-style.pptx): the listing it measures must be
// the one a shipped binary serves.
func shippedTemplatesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := fs.ReadDir(templates.Embedded, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".pptx") {
			continue
		}
		data, err := fs.ReadFile(templates.Embedded, e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestOnboardingPayloadBudgets is the go-slide-creator-mvdt5 acceptance test.
// What a default-profile agent reads before it can author a slide was about
// 95 KB (tools/list 41.6, get_started 12.2, list_slide_kinds 22.9,
// list_templates 17.9). First contact now has a 40 KiB budget and each piece
// a ceiling inside it; depth (a tool's full description, examples, field
// schemas, a template's colour roles) is fetched for the tool, the kinds and
// the template the agent chose.
func TestOnboardingPayloadBudgets(t *testing.T) {
	// Measured on the shipped templates only: a local templates/p-style.pptx
	// must not move a budget.
	mc, p := measureOnboardingPayload(t)

	for _, b := range []struct {
		name          string
		got, ceiling  int
		measuredToday int // on main before this change
	}{
		{"initialize instructions", p.Instructions, 1536, 1355},
		{"tools/list", p.ToolsList, deckSpecToolListByteBudget, 40628},
		{"get_started(brief)", p.GetStarted, 5632, 12066},
		{"list_slide_kinds catalogue", p.SlideKinds, 6 * 1024, 21661},
		{"list_templates fields:names", p.TemplateNames, 2 * 1024, 14652},
		{"list_templates compact", p.TemplatesCompact, 15 * 1024, 14652},
	} {
		t.Logf("%-28s %6d bytes (ceiling %d, was %d)", b.name, b.got, b.ceiling, b.measuredToday)
		if b.got > b.ceiling {
			t.Errorf("%s is %d bytes, over its %d-byte budget", b.name, b.got, b.ceiling)
		}
	}
	// The path get_started lays out: tools/list, get_started, the template
	// names and the kind catalogue.
	const firstContactBudget = 40 * 1024
	total := p.firstContact()
	t.Logf("first contact: %d bytes (budget %d)", total, firstContactBudget)
	if total > firstContactBudget {
		t.Errorf("first contact is %d bytes, over the %d-byte budget", total, firstContactBudget)
	}
	// The ceilings themselves fit the budget, so no piece can grow into
	// another's room unnoticed.
	if sum := 1536 + deckSpecToolListByteBudget + 5632 + 6*1024 + 2*1024; sum > firstContactBudget {
		t.Errorf("the per-piece ceilings sum to %d bytes, over the %d-byte first-contact budget", sum, firstContactBudget)
	}

	// The catalogue is one line per kind; the rest comes with a named kind.
	var catalogue struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, mustCall(t, mc.handleListSlideKinds, map[string]any{}).StructuredContent, &catalogue)
	if len(catalogue.SlideKinds) != len(semantic.AllSlideKinds()) {
		t.Errorf("catalogue lists %d kinds, want %d", len(catalogue.SlideKinds), len(semantic.AllSlideKinds()))
	}
	for _, k := range catalogue.SlideKinds {
		info, _ := semantic.LookupKind(semantic.SlideKind(k.Kind))
		if k.Summary == "" || len(k.Summary) > 200 || strings.Contains(k.Summary, ". ") {
			t.Errorf("catalogue line for %s is not one short sentence: %q", k.Kind, k.Summary)
		}
		if !endsOnWholeWord(k.Summary, info.Summary) {
			t.Errorf("catalogue line for %s is cut inside a word: %q", k.Kind, k.Summary)
		}
		if len(k.RequiredFields) != len(info.RequiredFields) {
			t.Errorf("catalogue dropped required_fields of %s", k.Kind)
		}
		if k.Example != nil || len(k.TypicalFields) > 0 {
			t.Errorf("catalogue row for %s carries authoring detail", k.Kind)
		}
	}
	var named struct {
		SlideKinds []slideKindListEntry `json:"slide_kinds"`
	}
	structuredInto(t, mustCall(t, mc.handleListSlideKinds, map[string]any{"kinds": []any{"agenda"}}).StructuredContent, &named)
	if info, _ := semantic.LookupKind("agenda"); len(named.SlideKinds) != 1 || named.SlideKinds[0].Summary != info.Summary ||
		len(named.SlideKinds[0].TypicalFields) == 0 || named.SlideKinds[0].Example == nil {
		t.Errorf("a named kind does not return its full summary, typical fields and example: %+v", named.SlideKinds)
	}

	// get_started sends the agent to the names projection, not the compact one.
	var resp getStartedResponse
	structuredInto(t, mustCall(t, mc.handleGetStarted, map[string]any{"task": "brief"}).StructuredContent, &resp)
	sawNames := false
	for _, step := range resp.Sequence {
		if step.Tool == "list_templates" && step.ArgsTemplate["fields"] == listFieldsNames {
			sawNames = true
		}
	}
	if !sawNames {
		t.Error("get_started(brief) does not ask for list_templates fields:names")
	}
}

// --tools all listed 335 KB, 200 KB of it outputSchema. The schemas are
// omitted unless asked for, and one tool's is a call away.
func TestAllProfileOmitsOutputSchemasByDefault(t *testing.T) {
	withToolProfile(t, activeToolProfile())
	t.Setenv(outputSchemasEnv, "")
	raw, tools := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll))
	for _, tool := range tools {
		if tool.RawOutputSchema != nil || tool.OutputSchema.Type != "" {
			t.Fatalf("all profile lists an outputSchema for %s by default", tool.Name)
		}
	}
	if len(raw) > allToolListByteBudget {
		t.Errorf("all tools/list is %d bytes, over the %d-byte budget", len(raw), allToolListByteBudget)
	}
	t.Logf("all profile: %d tools, %d bytes", len(tools), len(raw))

	t.Setenv(outputSchemasEnv, "1")
	_, inline := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll))
	withSchema := 0
	for _, tool := range inline {
		if tool.RawOutputSchema != nil || tool.OutputSchema.Type != "" {
			withSchema++
		}
	}
	if withSchema == 0 {
		t.Errorf("%s=1 did not restore the inline output schemas", outputSchemasEnv)
	}

	mc := refusalTestConfig(t)
	var one struct {
		Tool         string         `json:"tool"`
		OutputSchema map[string]any `json:"output_schema"`
	}
	structuredInto(t, mustCall(t, mc.handleGetCapabilities, map[string]any{"output_schema": "render_deck_spec"}).StructuredContent, &one)
	if one.Tool != "render_deck_spec" || one.OutputSchema["type"] != "object" {
		t.Errorf("get_capabilities output_schema: %+v", one)
	}
	if res := mustCall(t, mc.handleGetCapabilities, map[string]any{"output_schema": "no_such_tool"}); !res.IsError {
		t.Error("an unknown tool name was not refused")
	}
}

// TestSlideKindLookupIsCompact is the go-slide-creator-l6mcj acceptance test
// on the tool: item_schema lists canonical fields with their aliases named,
// fields:["brief"] answers "what does this kind take" in under 3 KB for every
// kind, and the expanded schema is still there for a validator.
func TestSlideKindLookupIsCompact(t *testing.T) {
	mc := refusalTestConfig(t)
	type row struct {
		Kind           string            `json:"kind"`
		ItemSchema     map[string]any    `json:"item_schema"`
		ItemSchemaFull map[string]any    `json:"item_schema_full"`
		Brief          map[string]string `json:"brief"`
		Budgets        []slideKindBudget `json:"budgets"`
		Example        map[string]any    `json:"example"`
	}
	type listing struct {
		SlideKinds []row `json:"slide_kinds"`
	}
	call := func(args map[string]any) (listing, int) {
		res := mustCall(t, mc.handleListSlideKinds, args)
		var out listing
		structuredInto(t, res.StructuredContent, &out)
		return out, onboardingBytes(t, res)
	}

	catalogue, _ := call(map[string]any{})
	for _, k := range catalogue.SlideKinds {
		if k.Example != nil || k.ItemSchema != nil {
			t.Fatalf("the catalogue row for %s carries an example or a schema", k.Kind)
		}
	}
	named, _ := call(map[string]any{"kinds": []any{"kpi_snapshot"}})
	if len(named.SlideKinds) != 1 || named.SlideKinds[0].Example["kind"] != "kpi_snapshot" {
		t.Fatalf("a named kind does not return its example: %+v", named.SlideKinds)
	}

	const briefCeiling = 3 * 1024
	for _, k := range semantic.AllSlideKinds() {
		brief, size := call(map[string]any{"kinds": []any{string(k)}, "fields": []any{"brief"}})
		r := brief.SlideKinds[0]
		if size >= briefCeiling {
			t.Errorf("%s: fields:[brief] is %d bytes, want under %d", k, size, briefCeiling)
		}
		if k != semantic.KindRawJSON2pptx && len(r.Budgets) == 0 {
			t.Errorf("%s: the brief comes without budgets", k)
		}
		info, _ := semantic.LookupKind(k)
		for _, f := range info.RequiredFields {
			if !strings.Contains(r.Brief[f], "required") {
				t.Errorf("%s: brief does not mark %q required: %q", k, f, r.Brief[f])
			}
		}
	}

	both, _ := call(map[string]any{"kinds": []any{"kpi_snapshot"}, "fields": []any{"item_schema", "item_schema_full"}})
	compact, _ := both.SlideKinds[0].ItemSchema["properties"].(map[string]any)
	full, _ := both.SlideKinds[0].ItemSchemaFull["properties"].(map[string]any)
	if _, ok := compact["metrics"]; ok {
		t.Error("item_schema still lists the alias metrics as a property")
	}
	if _, ok := full["metrics"]; !ok {
		t.Error("item_schema_full lost the alias metrics")
	}
	kpis, _ := compact["kpis"].(map[string]any)
	if aliases, _ := kpis["aliases"].([]any); len(aliases) != 1 || aliases[0] != "metrics" {
		t.Errorf("kpis.aliases = %v, want [metrics]", kpis["aliases"])
	}

	// An alias is still accepted on input.
	spec := map[string]any{
		"meta": map[string]any{"title": "Aliases", "template": "midnight-blue"},
		"slides": []any{
			map[string]any{"kind": "title", "title": "Alias check"},
			map[string]any{"kind": "kpi_snapshot", "title": "Three figures moved this quarter", "source": "Board pack",
				"metrics": []any{
					map[string]any{"big": "$48M", "small": "Revenue"},
					map[string]any{"big": "118%", "small": "Net retention"},
					map[string]any{"big": "41d", "small": "Sales cycle"},
				}},
		},
	}
	env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec}))
	for _, f := range env.Findings {
		if strings.Contains(f.Code, string(diagnostics.CodeSemanticUnknownField)) || strings.Contains(f.Code, "SEMANTIC_REQUIRED") {
			t.Errorf("a spec written with aliases is refused: %s %s", f.Code, f.Message)
		}
	}
}

// endsOnWholeWord reports whether a catalogue line is its full summary's
// sentence, or a cut of it that stops at the end of a word.
func endsOnWholeWord(line, full string) bool {
	kept, cut := strings.CutSuffix(line, "…")
	if !cut {
		return true
	}
	full = strings.Join(strings.Fields(full), " ")
	rest, ok := strings.CutPrefix(full, kept)
	if !ok || kept == "" {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(kept)
	next, _ := utf8.DecodeRuneInString(rest)
	wordRune := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	return (wordRune(last) || last == ')') && !wordRune(next) &&
		!danglingWords[strings.ToLower(kept[strings.LastIndex(kept, " ")+1:])]
}

// TestCatalogueLineEndsOnAWholeWord: a summary longer than the listing's line
// is cut after a whole word, not wherever the limit falls
// (go-slide-creator-0ae6a).
func TestCatalogueLineEndsOnAWholeWord(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Short enough.", "Short enough."},
		{"One number, made the whole slide: the value, the words beneath it, and optionally a line of context and a source line under it",
			"One number, made the whole slide: the value, the words beneath it, and optionally a line of context…"},
		{"A named framework with fixed parts: swot (4 quadrants), porters_five_forces (5 forces) or bmc (the 9-cell Business Model Canvas)",
			"A named framework with fixed parts: swot (4 quadrants), porters_five_forces (5 forces) or bmc…"},
		{strings.Repeat("x", 150), strings.Repeat("x", 109) + "…"},
	} {
		got := cutSummaryAtWord(tc.in, 110)
		if got != tc.want {
			t.Errorf("cutSummaryAtWord(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if n := len([]rune(got)); n > 110 {
			t.Errorf("cutSummaryAtWord(%q) is %d characters, over 110", tc.in, n)
		}
		if again := cutSummaryAtWord(got, 110); again != got {
			t.Errorf("cutSummaryAtWord is not stable on its own result: %q -> %q", got, again)
		}
	}
}
