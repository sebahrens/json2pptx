package main

import (
	"context"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/svggen"
)

// MCP completion references are prompts or resources, never tools. The
// existing catalogue resources supply the same vocabularies tool arguments
// accept (for example, templatesResourceURI/template completes the value for
// render_deck_spec.template).
type mcpCompletionProvider struct{ mc *mcpConfig }

func (p *mcpCompletionProvider) CompletePromptArgument(_ context.Context, name string, argument mcp.CompleteArgument, _ mcp.CompleteContext) (*mcp.Completion, error) {
	if name == "deck-from-brief" && argument.Name == "template" {
		return matchCompletion(listAvailableTemplates(p.mc.templatesDir), argument.Value), nil
	}
	return emptyCompletion(), nil
}

func (p *mcpCompletionProvider) CompleteResourceArgument(_ context.Context, uri string, argument mcp.CompleteArgument, _ mcp.CompleteContext) (*mcp.Completion, error) {
	var names []string
	switch {
	case uri == templatesResourceURI && argument.Name == "template":
		names = listAvailableTemplates(p.mc.templatesDir)
	case uri == patternsResourceURI && argument.Name == "pattern":
		for _, pattern := range patterns.Default().List() {
			names = append(names, pattern.Name())
		}
	case uri == deckSpecResourceURI && argument.Name == "kind":
		for _, kind := range semantic.AllSlideKinds() {
			names = append(names, string(kind))
		}
	case uri == deckSpecResourceURI && argument.Name == "archetype":
		for _, archetype := range semantic.AllArchetypes() {
			names = append(names, string(archetype))
		}
	case uri == deckSpecResourceURI && argument.Name == "chart_type":
		names = svggen.Types()
	case uri == skillResourceURI && argument.Name == "fix_kind":
		names = patterns.AllFixKinds()
	case uri == skillResourceURI && argument.Name == "finding_code":
		names = patterns.AllFitFindingCodes()
	default:
		return emptyCompletion(), nil
	}
	return matchCompletion(names, argument.Value), nil
}

func emptyCompletion() *mcp.Completion {
	return &mcp.Completion{Values: []string{}}
}

func matchCompletion(names []string, prefix string) *mcp.Completion {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	values := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			values = append(values, name)
		}
	}
	sort.Strings(values)
	total := len(values)
	if total > 100 {
		values = values[:100]
	}
	return &mcp.Completion{Values: values, Total: total, HasMore: total > len(values)}
}
