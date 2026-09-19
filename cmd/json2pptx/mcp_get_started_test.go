package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

func callGetStarted(t *testing.T, task string) getStartedResponse {
	t.Helper()
	args := map[string]any{}
	if task != "" {
		args["task"] = task
	}
	mc := &mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()}
	result, err := mc.handleGetStarted(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		t.Fatalf("handleGetStarted error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcp.TextContent).Text
	var resp getStartedResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return resp
}

func TestGetStartedBriefSequence(t *testing.T) {
	resp := callGetStarted(t, "brief")
	if resp.Task != "brief" {
		t.Errorf("task = %q, want %q", resp.Task, "brief")
	}
	want := []string{
		"get_capabilities",
		"list_templates",
		"plan_deck",
		"recommend_visual",
		"validate_input",
		"preview_presentation_plan",
		"generate_presentation",
		"score_deck",
		"render_deck_thumbnails",
		"inspect_slide_images",
	}
	if len(resp.Sequence) != len(want) {
		t.Fatalf("sequence length = %d, want %d (%v)", len(resp.Sequence), len(want), resp.Sequence)
	}
	for i, step := range resp.Sequence {
		if step.Tool != want[i] {
			t.Errorf("sequence[%d].tool = %q, want %q", i, step.Tool, want[i])
		}
		if step.WhenToCall == "" {
			t.Errorf("sequence[%d].when_to_call is empty for tool %q", i, step.Tool)
		}
	}
}

func TestGetStartedRequiresCurrentRevisionPixelReview(t *testing.T) {
	for _, task := range []string{"brief", "revise"} {
		resp := callGetStarted(t, task)
		if resp.Completion.DraftStatus != "draft_needs_visual_review" || resp.Completion.CompleteStatus != "visually_reviewed_current_revision" {
			t.Fatalf("%s completion protocol=%+v", task, resp.Completion)
		}
		joined := ""
		for _, step := range resp.Sequence {
			joined += step.Tool + " "
		}
		if !strings.Contains(joined, "render_deck_thumbnails") || !strings.Contains(joined, "inspect_slide_images") {
			t.Fatalf("%s omits pixel workflow: %s", task, joined)
		}
	}
}

// TestGetStartedSequencesAreClassifiedTools is the drift gate for first-call
// guidance: every tool named in any get_started sequence must be a registered,
// classified MCP tool. This proves get_started's recommended workflow agrees
// with the tool catalog and its classification metadata — if a tool is renamed
// or removed, the guidance breaks loudly here.
func TestGetStartedSequencesAreClassifiedTools(t *testing.T) {
	registered := map[string]bool{}
	for _, name := range mcpToolNames() {
		registered[name] = true
	}
	classes := toolClassifications()

	for _, task := range getStartedAvailableTasks() {
		resp := buildGetStartedResponse(task, testRenderReady())
		if len(resp.Sequence) == 0 {
			t.Errorf("task %q: empty sequence", task)
		}
		for i, step := range resp.Sequence {
			if !registered[step.Tool] {
				t.Errorf("task %q sequence[%d]: %q is not a registered MCP tool", task, i, step.Tool)
			}
			if _, ok := classes[step.Tool]; !ok {
				t.Errorf("task %q sequence[%d]: %q has no classification metadata", task, i, step.Tool)
			}
		}
	}
}

// TestGetStartedBriefRecommendsRenderDeckSpec pins go-slide-creator-o8kl: the
// brief fast path is the DeckSpec path (list_slide_kinds → validate_deck_spec →
// render_deck_spec), not make_deck (exemplar skeleton), and the response says
// so.
func TestGetStartedBriefRecommendsRenderDeckSpec(t *testing.T) {
	resp := callGetStarted(t, "brief")
	if resp.FastPath == nil {
		t.Fatal("brief response must carry a fast_path (the recommended best-deck path)")
	}
	if resp.FastPath.Tool != "render_deck_spec" {
		t.Errorf("brief fast_path.tool = %q, want %q", resp.FastPath.Tool, "render_deck_spec")
	}
	if !strings.Contains(resp.FastPath.WhenToCall, "DeckSpec") {
		t.Errorf("brief fast_path.when_to_call must mention DeckSpec, got %q", resp.FastPath.WhenToCall)
	}
	wantSteps := []string{"list_slide_kinds", "validate_deck_spec", "render_deck_spec", "render_deck_thumbnails"}
	if len(resp.FastPath.Steps) != len(wantSteps) {
		t.Fatalf("fast_path.steps = %+v, want %v", resp.FastPath.Steps, wantSteps)
	}
	for i, w := range wantSteps {
		if resp.FastPath.Steps[i].Tool != w {
			t.Errorf("fast_path.steps[%d] = %q, want %q", i, resp.FastPath.Steps[i].Tool, w)
		}
	}
	// make_deck must be positioned as a skeleton/wireframe, never the fast path.
	if !strings.Contains(resp.FastPath.WhenToCall, "skeleton/wireframe") {
		t.Errorf("fast_path.when_to_call must position make_deck as skeleton/wireframe only")
	}
	// falls_back_to must mirror the manual sequence so the facade and the
	// controllable path it collapses stay in lockstep.
	seqTools := make([]string, len(resp.Sequence))
	for i, s := range resp.Sequence {
		seqTools[i] = s.Tool
	}
	if len(resp.FastPath.FallsBackTo) != len(seqTools) {
		t.Fatalf("fast_path.falls_back_to = %v, want it to mirror sequence %v", resp.FastPath.FallsBackTo, seqTools)
	}
	for i, tool := range resp.FastPath.FallsBackTo {
		if tool != seqTools[i] {
			t.Errorf("falls_back_to[%d] = %q, want %q (must mirror sequence)", i, tool, seqTools[i])
		}
	}
	// Notes must name DeckSpec, demote make_deck, and state the completion rule.
	joined := strings.Join(resp.Notes, "\n")
	for _, must := range []string{"DeckSpec", "make_deck", "skeleton/wireframe", "raw-primitive", mcpCompletionRule} {
		if !strings.Contains(joined, must) {
			t.Errorf("brief notes must contain %q; notes:\n%s", must, joined)
		}
	}
	if resp.QualityWorkflow != mcpQualityWorkflow {
		t.Error("quality_workflow must echo the server instructions const verbatim")
	}
	if resp.Completion.Rule != mcpCompletionRule {
		t.Errorf("completion_protocol.rule = %q, want the shared completion rule", resp.Completion.Rule)
	}
}

// TestGetStartedFastPathIsClassifiedFacade is the drift gate for the fast path:
// when a task advertises a fast_path, the named tool must be a registered MCP
// tool classified as a workflow_facade. validate-only has no facade and must
// omit fast_path entirely.
func TestGetStartedFastPathIsClassifiedFacade(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	registered := map[string]bool{}
	for _, name := range mcpToolNames() {
		registered[name] = true
	}
	classes := toolClassifications()

	for _, task := range getStartedAvailableTasks() {
		resp := buildGetStartedResponse(task, testRenderReady())
		switch task {
		// validate-only is pure diagnostics; onboard-template vets a file the
		// server has never seen (go-slide-creator-ydbk) and every step of it
		// needs the agent's judgement — there is no facade that can decide a
		// template is fit for a deck.
		case "validate-only", "onboard-template":
			if resp.FastPath != nil {
				t.Errorf("task %q must NOT advertise a fast_path (no facade); got %q", task, resp.FastPath.Tool)
			}
		default:
			if resp.FastPath == nil {
				t.Errorf("task %q must advertise a fast_path facade", task)
				continue
			}
			if !registered[resp.FastPath.Tool] {
				t.Errorf("task %q fast_path.tool %q is not a registered MCP tool", task, resp.FastPath.Tool)
			}
			c, ok := classes[resp.FastPath.Tool]
			if !ok {
				t.Errorf("task %q fast_path.tool %q has no classification metadata", task, resp.FastPath.Tool)
				continue
			}
			if c.Kind != toolKindWorkflowFacade {
				t.Errorf("task %q fast_path.tool %q is kind %q, want %q (the fast path must be a workflow facade)",
					task, resp.FastPath.Tool, c.Kind, toolKindWorkflowFacade)
			}
		}
	}
}

func TestGetStartedReviseSequence(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	resp := callGetStarted(t, "revise")
	if resp.Task != "revise" {
		t.Errorf("task = %q, want %q", resp.Task, "revise")
	}
	want := []string{
		"get_capabilities",
		"read_presentation",
		"validate_input",
		"preview_presentation_plan",
		"repair_slide",
		"generate_presentation",
		"score_deck",
		"render_deck_thumbnails",
		"inspect_slide_images",
	}
	if len(resp.Sequence) != len(want) {
		t.Fatalf("sequence length = %d, want %d (%v)", len(resp.Sequence), len(want), resp.Sequence)
	}
	for i, step := range resp.Sequence {
		if step.Tool != want[i] {
			t.Errorf("sequence[%d].tool = %q, want %q", i, step.Tool, want[i])
		}
	}
}

func TestGetStartedValidateOnlySequence(t *testing.T) {
	resp := callGetStarted(t, "validate-only")
	if resp.Task != "validate-only" {
		t.Errorf("task = %q, want %q", resp.Task, "validate-only")
	}
	want := []string{
		"get_capabilities",
		"list_templates",
		"validate_input",
		"preview_presentation_plan",
	}
	if len(resp.Sequence) != len(want) {
		t.Fatalf("sequence length = %d, want %d (%v)", len(resp.Sequence), len(want), resp.Sequence)
	}
	for i, step := range resp.Sequence {
		if step.Tool != want[i] {
			t.Errorf("sequence[%d].tool = %q, want %q", i, step.Tool, want[i])
		}
	}
}

func TestGetStartedDefaultsToBrief(t *testing.T) {
	// Empty and unknown tasks both fall back to "brief".
	for _, task := range []string{"", "garbage", "build"} {
		resp := callGetStarted(t, task)
		if resp.Task != "brief" {
			t.Errorf("task=%q: response.task = %q, want %q", task, resp.Task, "brief")
		}
		if len(resp.Sequence) == 0 {
			t.Errorf("task=%q: sequence is empty", task)
		}
	}
}

// TestGetStartedBriefAdvertisesDeckChromeAndStructure ensures the brief flow
// surfaces the deck-chrome / structure / page-numbers / section-crumb opt-in
// fields. Agents staying inside MCP discovery should learn these exist without
// reading SKILL.md or scanning the generate_presentation description.
func TestGetStartedBriefAdvertisesDeckChromeAndStructure(t *testing.T) {
	resp := callGetStarted(t, "brief")
	joined := strings.Join(resp.Notes, "\n")
	for _, must := range []string{"chrome", "structure", "page_numbers", "section_crumb"} {
		if !strings.Contains(joined, must) {
			t.Errorf("brief notes must advertise %q so MCP-only agents can discover the advanced deck-level field; notes:\n%s", must, joined)
		}
	}
	if !strings.Contains(joined, "deck_chrome") || !strings.Contains(joined, "section_structure") {
		t.Errorf("brief notes should point agents at the get_capabilities.features flags (deck_chrome, section_structure); notes:\n%s", joined)
	}
}

func TestGetStartedAvailableTasksEchoed(t *testing.T) {
	resp := callGetStarted(t, "brief")
	want := getStartedAvailableTasks()
	if len(resp.AvailableTasks) != len(want) {
		t.Fatalf("available_tasks length = %d, want %d", len(resp.AvailableTasks), len(want))
	}
	for i, tk := range resp.AvailableTasks {
		if tk != want[i] {
			t.Errorf("available_tasks[%d] = %q, want %q", i, tk, want[i])
		}
	}
}

func TestGetStartedToolReferencesAreRegistered(t *testing.T) {
	// Every tool name referenced in any sequence must be a real MCP tool.
	registered := make(map[string]bool)
	for _, name := range mcpToolNames() {
		registered[name] = true
	}
	for _, task := range getStartedAvailableTasks() {
		resp := callGetStarted(t, task)
		for _, step := range resp.Sequence {
			if !registered[step.Tool] {
				t.Errorf("task=%q: step references unregistered tool %q", task, step.Tool)
			}
		}
	}
}

func TestGetStartedToolIsInCapabilities(t *testing.T) {
	names := mcpToolNames()
	found := false
	for _, n := range names {
		if n == "get_started" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected get_started in mcpToolCatalog()")
	}
}

// TestGetStartedSequences_Executable proves every step in every published
// get_started sequence can actually be invoked end-to-end against a fixture
// deck — no hidden human translation required between consecutive tools.
//
// The test wires outputs from prior steps into the inputs of later steps the
// same way an agent would: the deck JSON authored after recommend_visual is
// the same object passed to validate_input, preview_presentation_plan,
// repair_slide, generate_presentation, and score_deck; the pptx produced by
// generate_presentation is what read_presentation reads back.
func TestGetStartedSequences_Executable(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	ctx := context.Background()

	// Fixture deck — minimal but valid PresentationInput, used as the
	// authoritative deck JSON the agent already holds in memory. The same
	// object is passed to every downstream tool, including in the revise
	// flow (because read_presentation does NOT produce a PresentationInput).
	fixtureDeck := mustParseJSON(`{
		"template": "midnight-blue",
		"slides": [
			{
				"layout_id": "slideLayout2",
				"content": [
					{"placeholder_id": "title", "type": "text", "text_value": "Quarterly Review"},
					{"placeholder_id": "body", "type": "bullets", "bullets_value": ["Revenue up 12%", "Margin steady", "New market entry on track"]}
				]
			}
		]
	}`)

	// runStep dispatches one sequence step against the fixture. It returns
	// any side-effect the step produced (currently: generated pptx path).
	runStep := func(t *testing.T, tool string, generatedPath string) string {
		t.Helper()
		var result *mcp.CallToolResult
		var err error
		switch tool {
		case "get_capabilities":
			result, err = mc.handleGetCapabilities(ctx, makeRequest(map[string]any{}))
		case "list_templates":
			result, err = mc.handleListTemplates(ctx, makeRequest(map[string]any{}))
		case "plan_deck":
			result, err = mc.handlePlanDeck(ctx, makeRequest(map[string]any{
				"brief": "Quarterly review for the leadership team",
			}))
		case "recommend_visual":
			result, err = mc.handleRecommendVisual(ctx, makeRequest(map[string]any{
				"intent": "summarize the quarter's key results",
			}))
		case "validate_input":
			result, err = mc.handleValidate(ctx, makeRequest(map[string]any{
				"presentation": fixtureDeck,
				"fit_report":   true,
			}))
		case "preview_presentation_plan":
			result, err = mc.handlePreviewPlan(ctx, makeRequest(map[string]any{
				"presentation": fixtureDeck,
			}))
		case "generate_presentation":
			result, err = mc.handleGenerate(ctx, makeRequest(map[string]any{
				"presentation": fixtureDeck,
			}))
			if err == nil && result != nil && !result.IsError {
				var out JSONOutput
				if jerr := json.Unmarshal([]byte(textContent(result)), &out); jerr == nil {
					generatedPath = out.OutputPath
				}
			}
		case "read_presentation":
			if generatedPath == "" {
				t.Fatalf("read_presentation requires a generated pptx — none available; sequence ordering bug")
			}
			result, err = handleReadPresentation(ctx, makeRequest(map[string]any{
				"pptx_path": generatedPath,
			}))
		case "repair_slide":
			result, err = mc.handleRepairSlide(ctx, makeRequest(map[string]any{
				"presentation": fixtureDeck,
				"slide_index":  float64(0),
				"fixes":        []any{map[string]any{"kind": "reduce_text", "params": map[string]any{"max_items": float64(2)}}},
			}))
		case "score_deck":
			result, err = mc.handleScoreDeck(ctx, makeRequest(map[string]any{
				"presentation": fixtureDeck,
			}))
		case "render_deck_thumbnails":
			if os.Getenv("GET_STARTED_RENDER_INTEGRATION") != "1" {
				return generatedPath
			}
			if ok, _ := render.DependencyStatus(); !ok {
				return generatedPath
			}
			result, err = mc.handleRenderDeckThumbnails(ctx, makeRequest(map[string]any{"pptx_path": generatedPath}))
		case "inspect_slide_images":
			if os.Getenv("GET_STARTED_RENDER_INTEGRATION") != "1" {
				return generatedPath
			}
			if ok, _ := render.DependencyStatus(); !ok {
				return generatedPath
			}
			rendered, rerr := mc.handleRenderDeckThumbnails(ctx, makeRequest(map[string]any{"pptx_path": generatedPath}))
			if rerr != nil || rendered == nil {
				t.Fatalf("render prerequisite failed: %v", rerr)
			}
			if rendered.IsError {
				t.Fatalf("render prerequisite failed: %s", textContent(rendered))
			}
			var deck render.DeckResult
			if jerr := json.Unmarshal([]byte(textContent(rendered)), &deck); jerr != nil {
				t.Fatal(jerr)
			}
			images := make([]any, 0, len(deck.Slides))
			for _, slide := range deck.Slides {
				entry := map[string]any{"index": float64(slide.Index)}
				if slide.Path != "" {
					entry["path"] = slide.Path
				} else {
					entry["png_base64"] = slide.PNG64
				}
				images = append(images, entry)
			}
			result, err = mc.handleInspectSlideImages(ctx, makeRequest(map[string]any{"slide_images": images}))
		case "examine_template":
			// The bring-your-own form, which is the point of the
			// onboard-template task: a .pptx addressed by path, bounded by
			// base_dir (go-slide-creator-ydbk).
			dir, aerr := filepath.Abs("../../templates")
			if aerr != nil {
				t.Fatalf("abs templates dir: %v", aerr)
			}
			result, err = mc.handleExamineTemplate(ctx, makeRequest(map[string]any{
				"template_path": filepath.Join(dir, "midnight-blue.pptx"),
				"base_dir":      dir,
			}))
		case "describe_finding":
			result, err = handleDescribeFinding(ctx, makeRequest(map[string]any{"code": "LAYOUT_UNRESOLVABLE"}))
		default:
			t.Fatalf("integration test does not know how to invoke tool %q — add a case to runStep", tool)
		}
		if err != nil {
			t.Fatalf("tool %q returned transport error: %v", tool, err)
		}
		if result == nil {
			t.Fatalf("tool %q returned nil result", tool)
		}
		if result.IsError {
			t.Fatalf("tool %q returned IsError result: %s", tool, textContent(result))
		}
		return generatedPath
	}

	// For the revise flow the agent is editing a deck that has already been
	// rendered, so read_presentation has something to read. Pre-generate a
	// pptx from the same fixture and seed it as the starting "generated"
	// artifact for that task.
	preGenerate := func(t *testing.T) string {
		t.Helper()
		result, err := mc.handleGenerate(ctx, makeRequest(map[string]any{
			"presentation": fixtureDeck,
		}))
		if err != nil {
			t.Fatalf("pre-generate transport error: %v", err)
		}
		if result.IsError {
			t.Fatalf("pre-generate IsError: %s", textContent(result))
		}
		var out JSONOutput
		if jerr := json.Unmarshal([]byte(textContent(result)), &out); jerr != nil {
			t.Fatalf("pre-generate parse: %v", jerr)
		}
		if out.OutputPath == "" {
			t.Fatal("pre-generate produced empty output_path")
		}
		return out.OutputPath
	}

	for _, task := range getStartedAvailableTasks() {
		task := task
		t.Run(task, func(t *testing.T) {
			resp := callGetStarted(t, task)
			if len(resp.Sequence) == 0 {
				t.Fatalf("task=%q returned empty sequence", task)
			}
			var generatedPath string
			if task == "revise" {
				generatedPath = preGenerate(t)
				defer os.Remove(generatedPath)
			}
			for _, step := range resp.Sequence {
				step := step
				t.Run(step.Tool, func(t *testing.T) {
					generatedPath = runStep(t, step.Tool, generatedPath)
				})
			}
			if generatedPath != "" {
				_ = os.Remove(generatedPath)
			}
		})
	}
}

// TestGetStartedRevise_RequiresGenerateBeforeRead documents the invariant
// the revise sequence is built around: read_presentation is inspection-only
// and cannot serve as the source of the deck JSON the downstream editing
// tools (preview/repair/generate) require. If a future edit reorders revise
// so read_presentation precedes any downstream tool without a separate deck
// JSON source, this test will surface the silent contract violation.
func TestGetStartedRevise_ReadPresentationIsInspectionOnly(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	resp := callGetStarted(t, "revise")
	readIdx := -1
	for i, step := range resp.Sequence {
		if step.Tool == "read_presentation" {
			readIdx = i
			break
		}
	}
	if readIdx == -1 {
		t.Fatal("expected read_presentation in revise sequence")
	}
	hint := resp.Sequence[readIdx].WhenToCall
	for _, must := range []string{"Inspection-only", "NOT a PresentationInput"} {
		if !strings.Contains(hint, must) {
			t.Errorf("read_presentation when_to_call must contain %q to prevent agents from feeding its output downstream; got: %s", must, hint)
		}
	}
	// Agents are warned in notes that they must supply the deck JSON themselves.
	noteHit := false
	for _, n := range resp.Notes {
		if strings.Contains(n, "authoritative deck JSON") {
			noteHit = true
			break
		}
	}
	if !noteHit {
		t.Error("revise notes must explicitly require the agent to supply the deck JSON")
	}
}

// withToolProfile pins the advertised tool profile for a test and restores it
// afterwards. get_started is profile-aware: in the core profile it must not
// recommend a tool the agent cannot see (go-slide-creator-mvny), so a test
// asserting the full-catalog shape has to say so.
func withToolProfile(t *testing.T, profile string) {
	t.Helper()
	prev := activeToolProfile()
	setActiveToolProfile(profile)
	t.Cleanup(func() { setActiveToolProfile(prev) })
}

// go-slide-creator-mvny: in the core profile get_started's "revise" fast_path
// was auto_repair and its sequence step 2 was read_presentation — both hidden.
// A client model cannot emit a call to a tool absent from tools/list, so the
// RECOMMENDED path for the whole task was uncallable as shipped.
func TestGetStarted_CoreProfileRecommendsOnlyAdvertisedTools(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	core := coreToolSet()

	for _, task := range getStartedAvailableTasks() {
		t.Run(task, func(t *testing.T) {
			resp := buildGetStartedResponse(task, testRenderReady())

			for i, step := range resp.Sequence {
				if !core[step.Tool] {
					t.Errorf("sequence[%d] recommends %q, which the core profile does not advertise", i, step.Tool)
				}
			}
			if resp.FastPath == nil {
				return
			}
			if !core[resp.FastPath.Tool] {
				t.Errorf("fast_path.tool = %q, which the core profile does not advertise", resp.FastPath.Tool)
			}
			for i, step := range resp.FastPath.Steps {
				if !core[step.Tool] {
					t.Errorf("fast_path.steps[%d] recommends %q, which the core profile does not advertise", i, step.Tool)
				}
			}
			// falls_back_to mirrors the sequence, so it must be callable too.
			for _, name := range resp.FastPath.FallsBackTo {
				if !core[name] {
					t.Errorf("fast_path.falls_back_to names %q, which the core profile does not advertise", name)
				}
			}
		})
	}
}

// go-slide-creator-voxp: revise's recommended path is the DeckSpec the agent
// just authored — the same in both profiles. It used to be the raw-deck repair
// chain (auto_repair in the full profile, repair_slide in core), which named no
// DeckSpec at all, so a deck authored the recommended way had no documented way
// to be changed and the agent re-sent the whole spec by hand. The raw chain is
// still there as the second branch, in `sequence`.
func TestGetStarted_ReviseIsDeckSpecFirstInBothProfiles(t *testing.T) {
	for _, profile := range []string{toolProfileCore, toolProfileAll} {
		t.Run(profile, func(t *testing.T) {
			withToolProfile(t, profile)
			resp := buildGetStartedResponse("revise", testRenderReady())
			if resp.FastPath == nil {
				t.Fatal("revise has no fast_path")
			}
			if resp.FastPath.Tool != "render_deck_spec" {
				t.Errorf("revise fast_path.tool = %q, want render_deck_spec", resp.FastPath.Tool)
			}
			for _, want := range []string{"deck_id", "patch", "changed_slides"} {
				if !strings.Contains(resp.FastPath.WhenToCall, want) {
					t.Errorf("revise fast_path does not mention %q — the point of the path is not resending the spec", want)
				}
			}
			// The raw-deck chain remains the second branch.
			var sawGenerate bool
			for _, s := range resp.Sequence {
				if s.Tool == "generate_presentation" {
					sawGenerate = true
				}
			}
			if !sawGenerate {
				t.Error("revise sequence should still carry the raw-deck chain")
			}
			if profile == toolProfileAll {
				var sawReadPresentation bool
				for _, s := range resp.Sequence {
					if s.Tool == "read_presentation" {
						sawReadPresentation = true
					}
				}
				if !sawReadPresentation {
					t.Error("full profile revise sequence should still include read_presentation")
				}
			}
		})
	}
}

// A note that tells the agent what the OTHER profile has must not be emitted in
// that other profile, where it reads as a lie about the tools on the table.
func TestGetStartedNotesDoNotMisreportTheProfile(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	for _, n := range buildGetStartedResponse("revise", testRenderReady()).Notes {
		if strings.Contains(n, "this profile hides") {
			t.Errorf("full profile note claims tools are hidden: %q", n)
		}
	}
}

// go-slide-creator-c66z: get_started(brief)'s notes advertised top-level
// `chrome` and `structure` while its fast_path is the DeckSpec path, which
// rejected both — a direct contradiction inside one response, and an agent that
// followed the note got a blocked render. The note must now name the field that
// works on the path it is describing.
func TestGetStartedChromeNoteMatchesTheRecommendedPath(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	resp := buildGetStartedResponse("brief", testRenderReady())

	var note string
	for _, n := range resp.Notes {
		if strings.HasPrefix(n, "DECK CHROME") {
			note = n
			break
		}
	}
	if note == "" {
		t.Fatalf("no deck-chrome note in brief notes: %v", resp.Notes)
	}
	if !strings.Contains(note, "meta.chrome") {
		t.Error("the note must name meta.chrome — the spelling the DeckSpec fast path accepts")
	}
	if !strings.Contains(note, "structure") || !strings.Contains(note, "no DeckSpec equivalent") {
		t.Error("the note must say that structure is raw-path only, and how to get to it")
	}

	// And the claim must be true: meta.chrome compiles.
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{
			Title:    "Board update",
			Template: "midnight-blue",
			Date:     "September 2026",
			Chrome:   &semantic.ChromeSpec{Confidentiality: "Strictly confidential", ClientName: "Acme Corp"},
		},
		Slides: []semantic.SlideSpec{{Kind: semantic.KindTitle, Body: map[string]any{"title": "Board update"}}},
	}
	input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("meta.chrome does not compile, so the note is still a lie: %v (%+v)", err, result.Diagnostics)
	}
	if input.Chrome == nil || input.Chrome.Confidentiality == "" {
		t.Errorf("compiled chrome = %+v", input.Chrome)
	}
}
