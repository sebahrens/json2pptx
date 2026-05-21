package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/template"
)

func callGetStarted(t *testing.T, task string) getStartedResponse {
	t.Helper()
	args := map[string]any{}
	if task != "" {
		args["task"] = task
	}
	result, err := handleGetStarted(context.Background(), mcpRequestWithArgs(args))
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
		resp := buildGetStartedResponse(task)
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

func TestGetStartedReviseSequence(t *testing.T) {
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
