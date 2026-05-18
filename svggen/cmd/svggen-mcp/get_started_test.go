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
	result, err := handleGetStarted(context.Background(), makeRequest(args))
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

func TestGetStartedRenderSequence(t *testing.T) {
	resp := callGetStarted(t, "render")
	if resp.Task != "render" {
		t.Errorf("task = %q, want %q", resp.Task, "render")
	}
	want := []string{
		"get_capabilities",
		"list_diagram_types",
		"get_diagram_schema",
		"render_diagram",
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

func TestGetStartedPreflightRenderSequence(t *testing.T) {
	resp := callGetStarted(t, "preflight-render")
	if resp.Task != "preflight-render" {
		t.Errorf("task = %q, want %q", resp.Task, "preflight-render")
	}
	want := []string{
		"get_capabilities",
		"list_diagram_types",
		"get_diagram_schema",
		"validate_diagram",
		"render_diagram",
		"render_diagram",
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

func TestGetStartedEmbedInDeckSequence(t *testing.T) {
	resp := callGetStarted(t, "embed-in-deck")
	if resp.Task != "embed-in-deck" {
		t.Errorf("task = %q, want %q", resp.Task, "embed-in-deck")
	}
	want := []string{
		"get_capabilities",
		"list_diagram_types",
		"get_diagram_schema",
		"validate_diagram",
		"render_diagram",
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

func TestGetStartedDefaultsToRender(t *testing.T) {
	// Empty and unknown tasks both fall back to "render".
	for _, task := range []string{"", "garbage", "brief"} {
		resp := callGetStarted(t, task)
		if resp.Task != "render" {
			t.Errorf("task=%q: response.task = %q, want %q", task, resp.Task, "render")
		}
		if len(resp.Sequence) == 0 {
			t.Errorf("task=%q: sequence is empty", task)
		}
	}
}

func TestGetStartedAvailableTasksEchoed(t *testing.T) {
	resp := callGetStarted(t, "render")
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

// TestGetStartedToolReferencesAreRegistered verifies that every tool name in
// any task's sequence is a tool actually registered by this MCP server. This
// catches typos and drift between get_started and the live tool list.
func TestGetStartedToolReferencesAreRegistered(t *testing.T) {
	registered := make(map[string]bool)
	for _, entry := range toolCatalog() {
		registered[entry.Name] = true
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

// TestGetStartedIsInCapabilities verifies that get_started itself appears in
// get_capabilities tool_list — the discovery tool must be discoverable.
func TestGetStartedIsInCapabilities(t *testing.T) {
	found := false
	for _, entry := range toolCatalog() {
		if entry.Name == "get_started" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected get_started in toolCatalog()")
	}
}
