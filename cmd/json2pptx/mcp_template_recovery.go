package main

import (
	"encoding/json"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"strings"
)

func closestTemplateName(name string, available []string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".pptx")
	if name == "" || looksLikeTemplatePath(name) {
		return ""
	}
	limit := len([]rune(name)) / 2
	if limit > 4 {
		limit = 4
	}
	match, _ := generator.ClosestMatch(name, available, limit)
	return match
}

// withTemplateRetry preserves every supplied argument and changes only the
// template selector. It offers a call, never changes the stored deck or runs it.
func withTemplateRetry(d *diagnostics.Diagnostic, request mcp.CallToolRequest, tool, arg string) *diagnostics.Diagnostic {
	if d == nil || d.Code != diagnostics.CodeTemplateNotFound {
		return d
	}
	match, _ := d.Details["did_you_mean"].(string)
	if match == "" {
		return d
	}
	raw, err := json.Marshal(request.GetArguments())
	if err != nil {
		return d
	}
	var args map[string]any
	if json.Unmarshal(raw, &args) != nil {
		return d
	}
	if args == nil {
		args = map[string]any{}
	}
	node := args
	parts := strings.Split(arg, ".")
	for _, part := range parts[:len(parts)-1] {
		next, ok := node[part].(map[string]any)
		if !ok {
			return d
		}
		node = next
	}
	node[parts[len(parts)-1]] = match
	d.NextToolCall = &patterns.ToolCallSuggestion{Tool: tool, ArgsTemplate: args}
	return d
}

func (mc *mcpConfig) templateMissingResult(request mcp.CallToolRequest, tool, arg, name string) *mcp.CallToolResult {
	d := semanticTemplateDiagnostic(name, mc.templatesDir, diagnostics.CodeTemplateNotFound, templateNotFound(name, mc.templatesDir))
	d.Path = argFieldPath(arg)
	return api.MCPDiagnosticsError([]diagnostics.Diagnostic{*withTemplateRetry(d, request, tool, arg)})
}
