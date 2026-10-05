// mcp_tool_profile.go implements MCP tool profiles (go-slide-creator-vdxa,
// go-slide-creator-355t7).
//
// The full catalog is ~55 tools and a ~330KB tools/list payload (~80K tokens
// before the first call). Profiles filter what tools/list advertises:
//
//   - "deckspec" (default): the DeckSpec authoring core — discovery, the
//     semantic validate/render pair, render-and-look, review and scoring.
//     ~12 tools, no outputSchema, and validate_deck_spec carries the DeckSpec
//     outline instead of the closed per-kind schema (list_slide_kinds
//     fields:[item_schema] serves a chosen kind's contract on demand).
//   - "core" (alias "raw"): the deckspec tools plus the raw-JSON path
//     (validate_input, generate_presentation, repair_slide, patterns, ...).
//   - "all": every tool with full schemas, except the folded aliases in
//     foldedTools (still callable by name).
//
// Every tool stays REGISTERED in every profile — the profile only filters
// what tools/list advertises — so a hidden tool called by name still works,
// and registration-based parity/coverage tests see the full catalog.
package main

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	// toolProfileDeckSpec is the default: the DeckSpec authoring core.
	toolProfileDeckSpec = "deckspec"
	// toolProfileCore advertises the coreToolNames subset without outputSchema.
	toolProfileCore = "core"
	// toolProfileRaw is an accepted alias for toolProfileCore: the profile
	// that adds the raw-JSON path to the DeckSpec core.
	toolProfileRaw = "raw"
	// toolProfileAll advertises every registered tool with full schemas.
	toolProfileAll = "all"

	// toolProfileEnv selects the profile when the --tools flag is not given.
	toolProfileEnv = "JSON2PPTX_MCP_TOOLS"

	// outputSchemasEnv set to 1 / true makes the "all" profile advertise each
	// tool's outputSchema inline (--output-schemas does the same).
	outputSchemasEnv = "JSON2PPTX_MCP_OUTPUT_SCHEMAS"

	// deckSpecToolLimit and deckSpecToolListByteBudget cap the default
	// profile (TestDeckSpecToolProfileBudget). 40KB (~10K tokens) was the
	// go-slide-creator-355t7 target for tools/list alone; it is now the budget
	// for the whole of first contact (go-slide-creator-mvdt5), so the listing
	// is abridged (mcp_tool_listing_abridged.go) and capped well under it.
	deckSpecToolLimit          = 13 // 12 + show_pattern (go-slide-creator-cb339)
	deckSpecToolListByteBudget = 24 * 1024

	// allToolListByteBudget caps the "all" profile's default listing, which
	// carries no outputSchema (go-slide-creator-mvdt5): it was 335 KB, 200 KB
	// of it output schemas no model needs to choose or call a tool. Raised
	// from 144 KiB when validate_deck_spec's compact schema gained the fields
	// the journeys asked for (roadmap parallel_tracks, comparison connectors /
	// highlight_*, image_case image_width_pct, chart y_min / y_max —
	// go-slide-creator-ptazs, -929jm): descriptions are stripped there, so
	// only the property names cost bytes.
	allToolListByteBudget = 145 * 1024

	// coreToolLimit caps the core profile. TestCoreToolProfileBudget enforces it
	// together with coreToolListByteBudget.
	coreToolLimit = 24
	// coreToolListByteBudget is the max marshalled tools/list size (bytes) for
	// the core profile.
	//
	// The closed DeckSpec schema lives only on validate_deck_spec; the other
	// semantic tools carry an outline and point at list_slide_kinds
	// (go-slide-creator-uhaq). Repeated list-entry object schemas within that
	// remaining copy now share $defs references (go-slide-creator-cfo3g), taking
	// its compact form from 30,211 to 21,132 bytes and the core listing from
	// 102,032 to 93,582 bytes without changing the accepted payload contract.
	//
	// The budget has been raised four times before that (72 -> 80 -> 88 -> 96 ->
	// 104KB) as deck chrome, examine_template and the option_matrix / table /
	// architecture kinds arrived; it came back down to 92KB there.
	//
	// 92 -> 100KB: the agenda, team, stat, timeline and matrix_2x2 kinds. New
	// kinds still add unique fields to validate_deck_spec's input schema; keep
	// the budget fixed and watch TestCoreToolProfileBudget as the registry grows.
	// The default deckspec profile does not carry that schema at all.
	coreToolListByteBudget = 100 * 1024
)

// deckSpecToolNames is the tool set advertised by the default "deckspec"
// profile (go-slide-creator-355t7). Every name here is also a core tool.
var deckSpecToolNames = []string{
	"get_started",
	"list_templates",
	"examine_template",
	"list_slide_kinds",
	"validate_deck_spec",
	"render_deck_spec",
	"render_deck_thumbnails",
	"submit_visual_review",
	"describe_finding",
	"score_deck",
	"recommend_visual",
	"plan_deck",
	// show_pattern is the one raw-path tool a DeckSpec author needs: a
	// raw_json2pptx slide carries a pattern block, and its fields come from
	// here, not from list_slide_kinds (go-slide-creator-cb339).
	"show_pattern",
}

// foldedTools maps a tool whose job another tool now covers to that
// successor (go-slide-creator-fa3k8). A folded tool is a hidden alias: it
// stays registered and callable by its old name, so no client breaks, but no
// profile advertises it — the successor is the one to learn.
var foldedTools = map[string]string{
	// recommend_visual ranks patterns together with layouts, charts and
	// diagrams through the same intent string.
	"recommend_pattern": "recommend_visual",
	// list_templates fields="full" returns supported_types.chart_capabilities
	// and supported_types.diagram_capabilities.
	"get_chart_capabilities":   "list_templates",
	"get_diagram_capabilities": "list_templates",
	// render_deck_thumbnails slide_indices:[i] renders one slide.
	"render_slide_image": "render_deck_thumbnails",
	// repair_slide takes an ordered fixes[] array; repair_slides_batch only
	// adds per-directive slide_index.
	"repair_slides_batch": "repair_slide",
}

// coreToolNames is the tool set advertised by the "core" profile: the
// deckspec tools plus the raw-JSON authoring and repair path.
// To promote a tool into core, add its name here (one line) — the budget test
// fails if the profile outgrows coreToolLimit / coreToolListByteBudget.
var coreToolNames = []string{
	// Session entry / discovery
	"get_started",
	"get_capabilities",
	"list_templates",
	// examine_template is the bring-your-own-template entry point: it is the
	// only tool that inspects a .pptx the server has not registered, so a core
	// agent handed a client template could not vet it before rendering
	// (go-slide-creator-ydbk).
	"examine_template",
	"describe_finding",
	// Semantic DeckSpec path (recommended for new decks)
	"validate_deck_spec",
	"render_deck_spec",
	"list_slide_kinds",
	// Plan / vary
	"plan_deck",
	"recommend_visual",
	"analyze_deck_rhythm",
	"list_patterns",
	"show_pattern",
	"expand_pattern",
	// Raw JSON render
	"get_input_schema",
	"validate_input",
	"generate_presentation",
	// Inspect / repair
	"render_deck_thumbnails",
	"score_deck",
	"repair_slide",
	"preview_presentation_plan",
	"inspect_slide_images",
	"submit_visual_review",
}

// advertiseOutputSchemas is set by `json2pptx mcp --output-schemas`.
var advertiseOutputSchemas bool

// outputSchemasInline reports whether the "all" profile lists each tool's
// outputSchema in tools/list. Off by default: the schemas were 200 KB of a
// 335 KB listing, and get_capabilities output_schema:"<tool>" returns any one
// of them on request (go-slide-creator-mvdt5).
func outputSchemasInline() bool {
	if advertiseOutputSchemas {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(outputSchemasEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// toolSet returns names as a lookup set.
func toolSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// coreToolSet returns coreToolNames as a lookup set.
func coreToolSet() map[string]bool { return toolSet(coreToolNames) }

// deckSpecToolSet returns deckSpecToolNames as a lookup set.
func deckSpecToolSet() map[string]bool { return toolSet(deckSpecToolNames) }

// profileToolSet returns the tool names a profile advertises, or nil when the
// profile advertises every registered tool that is not folded.
func profileToolSet(profile string) map[string]bool {
	switch profile {
	case toolProfileDeckSpec:
		return deckSpecToolSet()
	case toolProfileCore:
		return coreToolSet()
	default:
		return nil
	}
}

// parseToolProfile validates a --tools / JSON2PPTX_MCP_TOOLS value. Empty
// selects the default (deckspec); raw is an alias for core.
func parseToolProfile(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", toolProfileDeckSpec:
		return toolProfileDeckSpec, nil
	case toolProfileCore, toolProfileRaw:
		return toolProfileCore, nil
	case toolProfileAll:
		return toolProfileAll, nil
	default:
		return "", cliInvalidArg("invalid tool profile %q: want %q, %q (alias %q) or %q", v, toolProfileDeckSpec, toolProfileCore, toolProfileRaw, toolProfileAll)
	}
}

// resolveToolProfile picks the profile from the flag value (when explicitly
// set) or the environment, defaulting to deckspec.
func resolveToolProfile(flagValue string, flagSet bool) (string, error) {
	if flagSet {
		return parseToolProfile(flagValue)
	}
	return parseToolProfile(os.Getenv(toolProfileEnv))
}

// toolProfileFilter returns the tools/list filter for a profile.
func toolProfileFilter(profile string) server.ToolFilterFunc {
	if profile == toolProfileAll {
		return func(_ context.Context, tools []mcp.Tool) []mcp.Tool {
			inline := outputSchemasInline()
			out := make([]mcp.Tool, 0, len(tools))
			for _, t := range tools {
				if _, folded := foldedTools[t.Name]; folded {
					continue
				}
				if !inline {
					// get_capabilities output_schema:"<tool>" serves one.
					t.RawOutputSchema = nil
					t.OutputSchema = mcp.ToolOutputSchema{}
				}
				out = append(out, t)
			}
			return out
		}
	}
	set := profileToolSet(profile)
	deckSpec := profile == toolProfileDeckSpec
	return func(_ context.Context, tools []mcp.Tool) []mcp.Tool {
		out := filterCoreTools(tools, set)
		if deckSpec {
			for i := range out {
				switch out[i].Name {
				case "validate_deck_spec":
					out[i] = withDeckSpecOutlineInput(out[i])
				case "render_deck_spec":
					out[i] = withDeckSpecReferenceInput(out[i])
				}
				// The listing an agent chooses and calls from; get_started
				// tool:"<name>" returns the rest (go-slide-creator-mvdt5).
				out[i] = withAbridgedListing(out[i])
			}
		}
		return out
	}
}

// filterCoreTools keeps only the profile's tools, sorted by name, with
// outputSchema stripped to keep the advertised payload small.
func filterCoreTools(tools []mcp.Tool, core map[string]bool) []mcp.Tool {
	out := make([]mcp.Tool, 0, len(core))
	for _, t := range tools {
		if !core[t.Name] {
			continue
		}
		t.RawOutputSchema = nil
		t.OutputSchema = mcp.ToolOutputSchema{}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// withDeckSpecOutlineInput swaps validate_deck_spec's closed per-kind spec
// schema (~23KB, a quarter of the old default listing) for the same outline the
// other spec tools carry (go-slide-creator-355t7). The accepted payload is
// unchanged — the compiler still rejects an unknown field as
// SEMANTIC_UNKNOWN_FIELD — and list_slide_kinds kinds:[k] fields:[item_schema]
// serves any chosen kind's closed contract on demand. Only the advertised copy
// changes; the registered tool keeps the closed schema for the "all" profile.
func withDeckSpecOutlineInput(tool mcp.Tool) mcp.Tool {
	var schema map[string]any
	if len(tool.RawInputSchema) == 0 || json.Unmarshal(tool.RawInputSchema, &schema) != nil {
		return tool
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return tool
	}
	spec := map[string]any{
		"description": "The semantic DeckSpec to validate, as a JSON object ({meta:{…}, slides:[{kind, …}]}) or a YAML/JSON string. Send this OR deck_id, not both." + deckSpecOutlineNote,
	}
	withDeckSpecOutline()(spec)
	props["spec"] = spec
	delete(schema, "$defs")
	raw, err := json.Marshal(schema)
	if err != nil {
		return tool
	}
	tool.RawInputSchema = raw
	return tool
}

// withDeckSpecReferenceInput replaces render_deck_spec's copy of the DeckSpec
// outline with a pointer to validate_deck_spec's, which the default profile
// lists right beside it: the same 3 KB outline was advertised twice
// (go-slide-creator-mvdt5). The accepted payload is unchanged.
func withDeckSpecReferenceInput(tool mcp.Tool) mcp.Tool {
	var schema map[string]any
	if len(tool.RawInputSchema) == 0 || json.Unmarshal(tool.RawInputSchema, &schema) != nil {
		return tool
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return tool
	}
	props["spec"] = map[string]any{
		"type":        []any{"object", "string"},
		"description": "The semantic DeckSpec to render, as a JSON object or a YAML/JSON string: the shape validate_deck_spec's spec documents. Send this OR deck_id, not both.",
		"oneOf": []any{
			map[string]any{"type": "object", "required": []any{"slides"}, "not": map[string]any{"required": []any{"structure"}}},
			map[string]any{"type": "object", "required": []any{"structure"}, "not": map[string]any{"required": []any{"slides"}}},
			map[string]any{"type": "string", "minLength": 1},
		},
	}
	delete(schema, "$defs")
	raw, err := json.Marshal(schema)
	if err != nil {
		return tool
	}
	tool.RawInputSchema = raw
	return tool
}

// newJSON2PPTXMCPServer builds the MCP server with every tool registered and
// the profile's tools/list filter applied.
func newJSON2PPTXMCPServer(mc *mcpConfig, profile string, opts ...server.ServerOption) *server.MCPServer {
	setActiveToolProfile(profile)
	if f := toolProfileFilter(profile); f != nil {
		opts = append(opts, server.WithToolFilter(f))
	}
	return newMCPServer(mc, opts...)
}

// activeProfile is the tool profile this process advertises. It is set once at
// server construction and read by handlers that must not recommend a tool the
// agent cannot see: a client model cannot emit a call to a tool absent from
// tools/list, so recommending one makes the recommended path uncallable
// (go-slide-creator-mvny).
//
// The default is toolProfileUnfiltered — "nothing is filtered, not even the
// folded aliases" — because the non-MCP entry points (generate, validate
// -fit-report) advertise no tool list at all, and their findings should keep
// naming the canonical tool. Every MCP server goes through
// newJSON2PPTXMCPServer, which sets the real profile.
var activeProfile = toolProfileUnfiltered

// toolProfileUnfiltered is the process default outside an MCP server: every
// tool counts as advertised.
const toolProfileUnfiltered = "unfiltered"

// setActiveToolProfile records the profile for profile-aware handlers.
func setActiveToolProfile(profile string) {
	if profile == "" {
		profile = toolProfileDeckSpec
	}
	activeProfile = profile
}

// activeToolProfile returns the profile this process advertises.
func activeToolProfile() string { return activeProfile }

// toolIsAdvertised reports whether a tool appears in the active profile's
// tools/list. Handlers use it to avoid pointing an agent at a tool it cannot
// call.
func toolIsAdvertised(name string) bool {
	switch profile := activeToolProfile(); profile {
	case toolProfileUnfiltered:
		return true
	case toolProfileAll:
		_, folded := foldedTools[name]
		return !folded
	default:
		return profileToolSet(profile)[name]
	}
}
