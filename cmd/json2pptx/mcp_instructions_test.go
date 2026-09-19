package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// go-slide-creator-f6kq: the initialize response carries the quality-workflow
// instructions, and they name the DeckSpec render path and the thumbnail
// inspection step.
func TestMCPInitializeCarriesInstructions(t *testing.T) {
	s := newMCPServer(&mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()})
	msg, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "0"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := s.HandleMessage(context.Background(), msg)
	resp, ok := out.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSON-RPC response, got %T: %+v", out, out)
	}
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var init struct {
		Instructions string `json:"instructions"`
	}
	if err := json.Unmarshal(raw, &init); err != nil {
		t.Fatal(err)
	}
	if init.Instructions != mcpQualityWorkflow {
		t.Fatalf("initialize instructions = %q, want mcpQualityWorkflow", init.Instructions)
	}
	for _, want := range []string{"get_started", "render_deck_spec", "render_deck_thumbnails", "semantic_path", "exemplar"} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("instructions must mention %q", want)
		}
	}
}

// get_started and the server instructions share one const, so they cannot
// drift; every task echoes it.
func TestGetStartedEchoesQualityWorkflow(t *testing.T) {
	for _, task := range getStartedAvailableTasks() {
		resp := buildGetStartedResponse(task, testRenderReady())
		if resp.QualityWorkflow != mcpQualityWorkflow {
			t.Errorf("task %q: quality_workflow does not echo the instructions const", task)
		}
		if resp.Completion.Rule != mcpCompletionRule {
			t.Errorf("task %q: completion rule drifted from mcpCompletionRule", task)
		}
	}
}
