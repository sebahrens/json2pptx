package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

func TestGetStartedReviseSequence(t *testing.T) {
	resp := callGetStarted(t, "revise")
	if resp.Task != "revise" {
		t.Errorf("task = %q, want %q", resp.Task, "revise")
	}
	want := []string{
		"get_capabilities",
		"read_presentation",
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
