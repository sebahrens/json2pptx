// mcp_tool_profile.go implements MCP tool profiles (go-slide-creator-vdxa).
//
// The full catalog is ~50 tools and a ~210KB tools/list payload (~50K tokens
// before the first call). The default "core" profile advertises only the
// tools an agent needs for the recommended DeckSpec / raw-JSON authoring and
// render-inspect-repair loop, and omits their outputSchema (responses still
// carry structuredContent). The "all" profile advertises the full, unchanged
// surface. Every tool stays REGISTERED in both profiles — the profile only
// filters what tools/list advertises — so a non-core tool called by name still
// works, and registration-based parity/coverage tests see the full catalog.
package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	// toolProfileCore advertises the coreToolNames subset without outputSchema.
	toolProfileCore = "core"
	// toolProfileAll advertises every registered tool with full schemas.
	toolProfileAll = "all"

	// toolProfileEnv selects the profile when the --tools flag is not given.
	toolProfileEnv = "JSON2PPTX_MCP_TOOLS"

	// coreToolLimit caps the core profile. TestCoreToolProfileBudget enforces it
	// together with coreToolListByteBudget.
	coreToolLimit = 24
	// coreToolListByteBudget is the max marshalled tools/list size (bytes) for
	// the core profile.
	//
	// The full closed DeckSpec schema — per-kind oneOf variants, each closed with
	// additionalProperties:false — is ~17KB and used to be embedded in every spec
	// tool: twice in this listing and four times in the full one, saying the same
	// thing each time. It now lives on validate_deck_spec alone, the tool the
	// workflow says to call before rendering; the others carry the outline (meta
	// + slides[].kind) and point at list_slide_kinds (go-slide-creator-uhaq).
	// That took the listing from 99KB to 85KB and removed the reason it kept
	// growing by ~2.4KB per new slide kind.
	//
	// The budget has been raised four times before that (72 -> 80 -> 88 -> 96 ->
	// 104KB) as deck chrome, examine_template and the option_matrix / table /
	// architecture kinds arrived; it came back down to 92KB there. The full
	// profile is ~313KB.
	//
	// 92 -> 100KB: the agenda, team, stat, timeline and matrix_2x2 kinds. Each
	// new kind still costs ~2.4KB of validate_deck_spec's closed schema — the
	// one tool that kept it — so this buys roughly three more before the next
	// raise. If that becomes a habit rather than an event, move the closed
	// schema behind list_slide_kinds entirely rather than raising again.
	coreToolListByteBudget = 100 * 1024
)

// coreToolNames is the tool set advertised by the default "core" profile.
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
	"render_slide_image",
	"score_deck",
	"repair_slide",
	"preview_presentation_plan",
	"inspect_slide_images",
	"submit_visual_review",
}

// coreToolSet returns coreToolNames as a lookup set.
func coreToolSet() map[string]bool {
	set := make(map[string]bool, len(coreToolNames))
	for _, n := range coreToolNames {
		set[n] = true
	}
	return set
}

// parseToolProfile validates a --tools / JSON2PPTX_MCP_TOOLS value. Empty
// selects the default (core).
func parseToolProfile(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", toolProfileCore:
		return toolProfileCore, nil
	case toolProfileAll:
		return toolProfileAll, nil
	default:
		return "", fmt.Errorf("invalid tool profile %q: want %q or %q", v, toolProfileCore, toolProfileAll)
	}
}

// resolveToolProfile picks the profile from the flag value (when explicitly
// set) or the environment, defaulting to core.
func resolveToolProfile(flagValue string, flagSet bool) (string, error) {
	if flagSet {
		return parseToolProfile(flagValue)
	}
	return parseToolProfile(os.Getenv(toolProfileEnv))
}

// toolProfileFilter returns the tools/list filter for a profile, or nil when
// the profile advertises the full surface.
func toolProfileFilter(profile string) server.ToolFilterFunc {
	if profile == toolProfileAll {
		return nil
	}
	core := coreToolSet()
	return func(_ context.Context, tools []mcp.Tool) []mcp.Tool {
		return filterCoreTools(tools, core)
	}
}

// filterCoreTools keeps only core tools, sorted by name, with outputSchema
// stripped to keep the advertised payload small.
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
// The default is toolProfileAll — "nothing is filtered" — because the non-MCP
// entry points (generate, validate -fit-report) advertise no tool list at all,
// and their findings should keep naming the canonical tool. Every MCP server
// goes through newJSON2PPTXMCPServer, which sets the real profile.
var activeProfile = toolProfileAll

// setActiveToolProfile records the profile for profile-aware handlers.
func setActiveToolProfile(profile string) {
	if profile == "" {
		profile = toolProfileCore
	}
	activeProfile = profile
}

// activeToolProfile returns the profile this process advertises.
func activeToolProfile() string { return activeProfile }

// toolIsAdvertised reports whether a tool appears in the active profile's
// tools/list. Handlers use it to avoid pointing an agent at a tool it cannot
// call.
func toolIsAdvertised(name string) bool {
	if activeToolProfile() == toolProfileAll {
		return true
	}
	return coreToolSet()[name]
}
