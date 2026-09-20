package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/generator"
)

// ---------------------------------------------------------------------------
// Strict MCP argument decoding (go-slide-creator-s9uq)
//
// Every tool call is checked against the called tool's declared input schema
// before its handler runs. An argument name the schema does not declare is
// rejected with UNKNOWN_PARAMETER and a did_you_mean suggestion instead of
// being silently ignored — e.g. plan_deck{slide_count: 8} used to plan with the
// default budget while the agent believed it had asked for 8 slides.
//
// The check lives in one server-level middleware (installed by newMCPServer),
// so it covers every registered tool — including tools added later — without
// touching individual handlers.
// ---------------------------------------------------------------------------

// mcpUndeclaredArgAliases lists argument names a tool still honours for
// backward compatibility although its schema does not advertise them. They are
// accepted (not reported) because the handler reads them.
var mcpUndeclaredArgAliases = map[string][]string{
	// make_deck falls back to auto_repair's max_passes name when
	// max_repair_passes is absent (extractMakeDeckMaxPasses).
	"make_deck": {"max_passes"},
}

// strictArgsMiddleware returns a tool-handler middleware that rejects unknown
// arguments. lookup resolves a tool name to its registered definition; a tool
// it cannot resolve is passed through untouched (the server reports unknown
// tools itself).
func strictArgsMiddleware(lookup func(name string) *server.ServerTool) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			st := lookup(request.Params.Name)
			if st == nil {
				return next(ctx, request)
			}
			if res := unknownArgumentsError(st.Tool, request.GetArguments()); res != nil {
				return res, nil
			}
			return next(ctx, request)
		}
	}
}

// toolArgNames returns the sorted argument names a tool accepts: its declared
// input-schema properties plus any legacy aliases its handler still reads.
func toolArgNames(tool mcp.Tool) []string {
	props := tool.InputSchema.Properties
	if len(tool.RawInputSchema) > 0 {
		var raw struct {
			Properties map[string]any `json:"properties"`
		}
		if err := json.Unmarshal(tool.RawInputSchema, &raw); err == nil {
			props = raw.Properties
		}
	}
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	names = append(names, mcpUndeclaredArgAliases[tool.Name]...)
	sort.Strings(names)
	return names
}

// unknownArgumentsError returns an UNKNOWN_PARAMETER error result listing every
// argument not accepted by tool, or nil when all arguments are known. Tools
// whose schema explicitly allows additional properties are not checked.
func unknownArgumentsError(tool mcp.Tool, args map[string]any) *mcp.CallToolResult {
	if len(args) == 0 {
		return nil
	}
	if ap, ok := tool.InputSchema.AdditionalProperties.(bool); ok && ap {
		return nil
	}
	accepted := toolArgNames(tool)
	known := make(map[string]bool, len(accepted))
	for _, n := range accepted {
		known[n] = true
	}
	var unknown []string
	for k := range args {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)

	acceptedList := strings.Join(accepted, ", ")
	if acceptedList == "" {
		acceptedList = "(none — this tool takes no arguments)"
	}
	diags := make([]diagnostics.Diagnostic, 0, len(unknown))
	for _, k := range unknown {
		d := diagnostics.Diagnostic{
			Code:     diagnostics.CodeUnknownParameter,
			Path:     k,
			Severity: diagnostics.SeverityError,
			Message: fmt.Sprintf("unknown argument %q for tool %s — it would be silently ignored, so the call is rejected. Accepted arguments: %s",
				k, tool.Name, acceptedList),
		}
		if sug := suggestArgName(k, accepted); sug != "" {
			d.Message = fmt.Sprintf("unknown argument %q for tool %s; did you mean %q? Accepted arguments: %s",
				k, tool.Name, sug, acceptedList)
			d.Fix = &diagnostics.Fix{Kind: "rename_field", Params: map[string]any{
				"from": k, "to": sug, "did_you_mean": sug,
			}}
			d.NextToolCall = nextCallRetry(tool.Name, sug)
		}
		diags = append(diags, d)
	}
	return api.MCPDiagnosticsError(diags)
}

// suggestArgName returns the accepted argument an unknown key most likely
// meant: the closest name by edit distance when it is close, otherwise the
// accepted name sharing the most underscore-separated tokens with the key
// (slide_count → slide_budget). Returns "" when nothing is plausible.
func suggestArgName(key string, accepted []string) string {
	if len(accepted) == 0 {
		return ""
	}
	maxDist := 2
	if len(key) >= 8 {
		maxDist = 3
	}
	if match, dist := generator.ClosestMatch(key, accepted, maxDist); dist >= 0 {
		return match
	}
	keyTokens := argTokens(key)
	best, bestShared := "", 0
	for _, cand := range accepted {
		shared := 0
		for t := range argTokens(cand) {
			if keyTokens[t] {
				shared++
			}
		}
		if shared > bestShared {
			best, bestShared = cand, shared
		}
	}
	if best != "" {
		return best
	}
	// Neither spelling nor tokens match, but the agent may have used a
	// different WORD for the same thing. show_pattern{pattern: "agenda"} is the
	// reported case: "pattern" and "name" share no letters worth counting, so
	// the server said only "Accepted arguments: name" while its own
	// instructions promise a did_you_mean (go-slide-creator-dwkkf).
	for _, syn := range argSynonyms[key] {
		for _, cand := range accepted {
			if cand == syn {
				return syn
			}
		}
	}
	return ""
}

// argSynonyms maps a word an agent plausibly reaches for to the argument names
// this server actually uses. Only same-meaning pairs belong here — edit
// distance and shared tokens already cover typos and near-misses — and a
// suggestion is made only when the tool really accepts the target.
var argSynonyms = map[string][]string{
	"pattern":       {"name"},
	"pattern_name":  {"name"},
	"diagram":       {"name", "type"},
	"template_name": {"template", "template_path"},
	"deck":          {"presentation", "deck_spec"},
	"spec":          {"deck_spec", "presentation"},
	"slides":        {"presentation", "deck_spec"},
	"json":          {"presentation", "deck_spec"},
	"file":          {"path", "pptx_path", "output_path"},
	"pptx":          {"pptx_path", "path"},
	"dir":           {"base_dir", "output_dir"},
	"directory":     {"base_dir", "output_dir"},
	"prompt":        {"brief", "intent"},
	"description":   {"intent", "brief"},
	"goal":          {"intent", "brief"},
	"slide":         {"slide_index", "slide_json"},
	"index":         {"slide_index"},
	"image":         {"image_path", "images"},
	"model":         {"vision_model"},
}

// argTokens splits an argument name into its lowercase underscore/hyphen
// tokens, ignoring very short ones that carry no meaning ("id" is kept).
func argTokens(name string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return r == '_' || r == '-' }) {
		if len(t) >= 2 {
			out[t] = true
		}
	}
	return out
}
