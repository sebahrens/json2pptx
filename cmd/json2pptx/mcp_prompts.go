package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerMCPPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("deck-from-brief",
		mcp.WithPromptDescription("Create and visually review a deck from a real-content brief."),
		mcp.WithArgument("brief", mcp.ArgumentDescription("The user's brief and required content."), mcp.RequiredArgument()),
		mcp.WithArgument("template", mcp.ArgumentDescription("Optional template name.")),
		mcp.WithArgument("slide_budget", mcp.ArgumentDescription("Optional maximum slide count (1-100).")),
	), deckFromBriefPrompt)
	s.AddPrompt(mcp.NewPrompt("revise-deck",
		mcp.WithPromptDescription("Revise an existing deck, then render and review the changed revision."),
		mcp.WithArgument("goal", mcp.ArgumentDescription("The change the user wants."), mcp.RequiredArgument()),
		mcp.WithArgument("pptx_path", mcp.ArgumentDescription("Path to an existing PPTX; set this or deck_json.")),
		mcp.WithArgument("deck_json", mcp.ArgumentDescription("DeckSpec or raw PresentationInput JSON; set this or pptx_path.")),
	), reviseDeckPrompt)
}

func deckFromBriefPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	brief := strings.TrimSpace(request.Params.Arguments["brief"])
	if brief == "" {
		return nil, fmt.Errorf("deck-from-brief requires a non-empty brief")
	}
	template := strings.TrimSpace(request.Params.Arguments["template"])
	if template == "" {
		template = "select a template with list_templates"
	}
	budget := strings.TrimSpace(request.Params.Arguments["slide_budget"])
	if budget != "" {
		n, err := strconv.Atoi(budget)
		if err != nil || n < 1 || n > 100 {
			return nil, fmt.Errorf("slide_budget must be an integer from 1 to 100")
		}
	}
	text := fmt.Sprintf("Create a presentation from this brief:\n%s\n\nTemplate: %s\n", brief, template)
	if budget != "" {
		text += "Maximum slides: " + budget + "\n"
	}
	text += "\n" + promptWorkflow("brief")
	return mcp.NewGetPromptResult("New deck workflow", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

func reviseDeckPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	goal := strings.TrimSpace(request.Params.Arguments["goal"])
	pptxPath := strings.TrimSpace(request.Params.Arguments["pptx_path"])
	deckJSON := strings.TrimSpace(request.Params.Arguments["deck_json"])
	if goal == "" || (pptxPath == "") == (deckJSON == "") {
		return nil, fmt.Errorf("revise-deck requires goal and exactly one of pptx_path or deck_json")
	}
	source := "PPTX path: " + pptxPath
	guidance := "A PPTX alone is not editable source JSON. Obtain its authoritative DeckSpec or raw deck JSON before editing. If unavailable, re-author from the brief."
	if toolIsAdvertised("read_presentation") {
		guidance += " Use read_presentation to inspect the PPTX; its output is not editable source JSON."
	}
	if deckJSON != "" {
		source = "Deck JSON:\n" + deckJSON
		guidance = "If the JSON is a DeckSpec, edit it with validate_deck_spec and render_deck_spec. If it is raw PresentationInput, use the raw revision sequence below."
	}
	text := fmt.Sprintf("Revise this presentation.\nGoal: %s\n%s\n\n%s\n\n%s", goal, source, guidance, promptWorkflow("revise"))
	return mcp.NewGetPromptResult("Deck revision workflow", []mcp.PromptMessage{mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text))}), nil
}

// promptWorkflow uses the same task-specific steps as get_started. The prompt
// carries them so a host's slash command can start work without discovery.
func promptWorkflow(task string) string {
	available, missing := renderDependencyStatus()
	started := buildGetStartedResponseOpts(task, getStartedRuntime{
		RenderAvailable: available,
		MissingCommands: missing,
	}, false)
	var b strings.Builder
	b.WriteString("Follow this json2pptx DeckSpec workflow with the user's real content:\n")
	if started.FastPath != nil {
		for i, step := range started.FastPath.Steps {
			fmt.Fprintf(&b, "%d. %s — %s\n", i+1, step.Tool, step.WhenToCall)
		}
	}
	if task == "revise" {
		b.WriteString("For raw PresentationInput, use this get_started revision sequence with the authoritative deck JSON:\n")
		for i, step := range started.Sequence {
			fmt.Fprintf(&b, "%d. %s — %s\n", i+1, step.Tool, step.WhenToCall)
		}
	}
	b.WriteString("Call get_started for full guidance when the listed steps cannot express the requested change. Never ship exemplar or placeholder content.\n")
	b.WriteString(started.Completion.Rule)
	return b.String()
}
