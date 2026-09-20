package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func promptRPC(t *testing.T, s interface {
	HandleMessage(context.Context, json.RawMessage) mcp.JSONRPCMessage
}, method string, params map[string]any) any {
	t.Helper()
	message, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 2, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	response, ok := s.HandleMessage(context.Background(), message).(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("%s did not return a success response", method)
	}
	return response.Result
}

func TestMCPWorkflowPrompts(t *testing.T) {
	withRenderStatus(t, true, nil)
	s := newMCPServer(&mcpConfig{templatesDir: "../../templates", outputDir: t.TempDir()})
	initialized := promptRPC(t, s, "initialize", map[string]any{
		"protocolVersion": mcp.LATEST_PROTOCOL_VERSION,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "prompt-test", "version": "0"},
	})
	initJSON, err := json.Marshal(initialized)
	if err != nil {
		t.Fatal(err)
	}
	var init struct {
		Capabilities struct {
			Prompts *struct {
				ListChanged bool `json:"listChanged"`
			} `json:"prompts"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(initJSON, &init); err != nil {
		t.Fatal(err)
	}
	if init.Capabilities.Prompts == nil || init.Capabilities.Prompts.ListChanged {
		t.Fatalf("initialize did not advertise static prompts: %s", initJSON)
	}
	listedResult := promptRPC(t, s, "prompts/list", nil)
	listed, ok := listedResult.(mcp.ListPromptsResult)
	if !ok {
		t.Fatalf("prompts/list returned %T", listedResult)
	}
	if len(listed.Prompts) != 2 || listed.Prompts[0].Name != "deck-from-brief" || listed.Prompts[1].Name != "revise-deck" {
		t.Fatalf("unexpected prompts: %+v", listed.Prompts)
	}
	for _, prompt := range listed.Prompts {
		if prompt.Description == "" || len(prompt.Arguments) == 0 {
			t.Errorf("prompt %q lacks discovery metadata", prompt.Name)
		}
	}
	brief := promptRPC(t, s, "prompts/get", map[string]any{"name": "deck-from-brief", "arguments": map[string]string{
		"brief": "Explain our cloud migration plan", "template": "midnight-blue", "slide_budget": "8",
	}}).(mcp.GetPromptResult)
	if len(brief.Messages) != 1 || brief.Messages[0].Role != mcp.RoleUser {
		t.Fatalf("unexpected brief prompt messages: %+v", brief.Messages)
	}
	content, ok := brief.Messages[0].Content.(mcp.TextContent)
	if !ok {
		t.Fatal("brief prompt did not return text")
	}
	for _, want := range []string{"cloud migration plan", "midnight-blue", "Maximum slides: 8", "list_slide_kinds", "validate_deck_spec", "render_deck_spec", "render_deck_thumbnails", mcpCompletionRule} {
		if !strings.Contains(content.Text, want) {
			t.Errorf("brief prompt lacks %q", want)
		}
	}
	revise := promptRPC(t, s, "prompts/get", map[string]any{"name": "revise-deck", "arguments": map[string]string{
		"pptx_path": "/tmp/old.pptx", "goal": "Fix slide 4's chart",
	}}).(mcp.GetPromptResult)
	got := revise.Messages[0].Content.(mcp.TextContent).Text
	for _, want := range []string{"/tmp/old.pptx", "Fix slide 4's chart", mcpCompletionRule} {
		if !strings.Contains(got, want) {
			t.Errorf("revise prompt lacks %q", want)
		}
	}
	if !strings.Contains(got, "A PPTX alone is not editable source JSON") {
		t.Fatal("PPTX revision prompt must explain the missing source contract")
	}
	fromJSON := promptRPC(t, s, "prompts/get", map[string]any{"name": "revise-deck", "arguments": map[string]string{
		"deck_json": `{"meta":{"title":"Deck"},"slides":[]}`, "goal": "Retitle the cover",
	}}).(mcp.GetPromptResult)
	jsonText := fromJSON.Messages[0].Content.(mcp.TextContent).Text
	for _, want := range []string{"Retitle the cover", "Deck JSON:", "validate_deck_spec", "validate_input", "generate_presentation"} {
		if !strings.Contains(jsonText, want) {
			t.Errorf("deck JSON revision prompt lacks %q", want)
		}
	}
}

func TestMCPWorkflowPromptWithoutRenderTooling(t *testing.T) {
	withRenderStatus(t, false, []string{"libreoffice/soffice"})
	result, err := deckFromBriefPrompt(context.Background(), mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{Arguments: map[string]string{"brief": "Quarterly update"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := result.Messages[0].Content.(mcp.TextContent).Text
	if !strings.Contains(got, "RENDER TOOLING MISSING") || !strings.Contains(got, "UNREVIEWED") {
		t.Fatalf("prompt did not explain unavailable visual review: %s", got)
	}
}

func TestMCPRevisePromptRespectsToolProfile(t *testing.T) {
	withRenderStatus(t, true, nil)
	request := mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: map[string]string{
		"pptx_path": "/tmp/deck.pptx", "goal": "Update chart",
	}}}
	withToolProfile(t, toolProfileCore)
	core, err := reviseDeckPrompt(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	coreText := core.Messages[0].Content.(mcp.TextContent).Text
	if strings.Contains(coreText, "Use read_presentation") {
		t.Fatal("core prompt suggested a hidden tool")
	}
	withToolProfile(t, toolProfileAll)
	full, err := reviseDeckPrompt(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	fullText := full.Messages[0].Content.(mcp.TextContent).Text
	if !strings.Contains(fullText, "Use read_presentation") {
		t.Fatal("full profile prompt omitted its PPTX inspection tool")
	}
}

func TestMCPWorkflowPromptRequiredArguments(t *testing.T) {
	for _, test := range []struct {
		name string
		args map[string]string
	}{
		{"empty brief", map[string]string{}},
		{"invalid slide budget", map[string]string{"brief": "A", "slide_budget": "zero"}},
		{"revise without source", map[string]string{"goal": "Fix"}},
		{"revise with two sources", map[string]string{"goal": "Fix", "pptx_path": "a.pptx", "deck_json": "{}"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{Params: mcp.GetPromptParams{Arguments: test.args}}
			var err error
			if strings.HasPrefix(test.name, "revise") {
				_, err = reviseDeckPrompt(context.Background(), request)
			} else {
				_, err = deckFromBriefPrompt(context.Background(), request)
			}
			if err == nil {
				t.Fatal("expected prompt argument error")
			}
		})
	}
}
