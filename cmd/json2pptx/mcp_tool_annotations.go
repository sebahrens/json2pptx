// mcp_tool_annotations.go derives each tool's MCP annotations from its
// classification (go-slide-creator-ccqn).
//
// Every tool shipped mcp-go's NewTool() defaults — readOnlyHint:false,
// destructiveHint:true, idempotentHint:false, openWorldHint:true, no title — so
// get_started, get_capabilities, list_templates, describe_finding and every
// validate_* were advertised as destructive, non-idempotent and open-world.
// Hosts that gate destructive tools behind confirmation prompt on every
// discovery call, and planners that avoid destructive tools avoid the tools the
// workflow starts with.
//
// The truth already lived in mcp_tool_classification.go (MutatesState,
// WritesFiles, APIKeyDependency), so the annotations are derived from it rather
// than hand-maintained per tool.
package main

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// annotateTool fills a tool's annotations from its classification. A tool with
// no classification entry still gets a title and the conservative default
// (TestEveryRegisteredToolIsClassified keeps that set empty).
func annotateTool(tool *mcp.Tool) {
	if tool == nil {
		return
	}
	c, classified := toolClassifications()[tool.Name]

	// Read-only means "does not modify the caller's environment". A regenerable
	// preview cache does not: list_templates writes one by default and is still
	// the first call of the workflow.
	readOnly := classified && !c.MutatesState && (!c.WritesFiles || c.CacheWritesOnly)
	destructive := c.MutatesState
	// Repeated calls with the same arguments have no ADDITIONAL effect: reads
	// are idempotent, and so is a render, which overwrites its own output for
	// the same input. Only the settings writers change server state.
	idempotent := !c.MutatesState
	// The server touches its own templates and output directories; the only
	// tools that reach anything external are the ones that call a vision API.
	openWorld := c.APIKeyDependency

	tool.Annotations = mcp.ToolAnnotation{
		Title:           toolTitle(tool.Name),
		ReadOnlyHint:    &readOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  &idempotent,
		OpenWorldHint:   &openWorld,
	}
}

// toolTitle renders a tool name as a short human label: "list_templates" →
// "List templates".
func toolTitle(name string) string {
	if name == "" {
		return ""
	}
	words := strings.Split(name, "_")
	for i, w := range words {
		if i == 0 && w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
			continue
		}
		words[i] = toolTitleWord(w)
	}
	return strings.Join(words, " ")
}

// toolTitleWord keeps known acronyms and file types upper-cased in a title.
func toolTitleWord(w string) string {
	switch w {
	case "pptx", "svg", "json", "json2pptx", "kpi", "qa", "mcp":
		return strings.ToUpper(w)
	}
	return w
}
