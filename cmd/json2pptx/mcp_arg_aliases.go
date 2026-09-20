package main

import (
	"context"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Sibling-name argument aliases (go-slide-creator-r1m3)
//
// Three tools name the same concept differently from their siblings, and each
// mismatch cost a round-trip: validate_presentation_output takes `path` while
// five other PPTX tools take `pptx_path`; resolve_theme takes `template_name`
// while eleven tools take `template`; plan_deck takes `slide_budget` while the
// rest of the world says `slide_count`.
//
// The UNKNOWN_PARAMETER error an agent got back was already good — did_you_mean,
// next_tool_call and a rename_field patch, from go-slide-creator-s9uq — but the
// cheapest fix is not to need it. The majority spelling is now accepted on each
// of these tools and rewritten to the declared one before anything else runs,
// so handlers still read a single name and the schema still advertises a single
// name.

// mcpArgAliasTargets maps, per tool, an accepted argument name to the name that
// tool's schema declares. Renaming the declared names instead would break every
// existing caller, so the aliases go the other way.
var mcpArgAliasTargets = map[string]map[string]string{
	// The convention across audit_palette, read_presentation,
	// render_deck_thumbnails, render_slide_image and submit_visual_review.
	"validate_presentation_output": {"pptx_path": "path"},
	// The convention across auto_repair, compile_deck_spec, list_templates,
	// make_deck, plan_deck, recommend_visual, render_deck_spec,
	// render_slide_image_from_json, score_candidates, score_deck and
	// table_density_guide.
	"resolve_theme":             {"template": "template_name"},
	"examine_template":          {"template": "template_name"},
	"list_template_settings":    {"template": "template_name"},
	"register_template_setting": {"template": "template_name"},
	"delete_template_setting":   {"template": "template_name"},
	// No tool declares slide_count as an input, but every response field that
	// counts slides is called slide_count, so that is what an agent reaches for.
	"plan_deck": {"slide_count": "slide_budget"},
}

// argAliasMiddleware rewrites accepted sibling names to the names the called
// tool declares. It runs before strictArgsMiddleware, so an alias never reaches
// the unknown-argument check.
//
// An explicit declared name always wins: a call that sends both is honouring
// the schema, and silently preferring the alias would discard the value the
// author actually meant.
func argAliasMiddleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			aliases := mcpArgAliasTargets[request.Params.Name]
			if len(aliases) == 0 {
				return next(ctx, request)
			}
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok || len(args) == 0 {
				return next(ctx, request)
			}
			rewritten := make(map[string]any, len(args))
			for k, v := range args {
				rewritten[k] = v
			}
			changed := false
			for alias, declared := range aliases {
				v, present := rewritten[alias]
				if !present {
					continue
				}
				delete(rewritten, alias)
				changed = true
				if _, already := rewritten[declared]; already {
					continue // the declared name wins
				}
				rewritten[declared] = v
			}
			if !changed {
				return next(ctx, request)
			}
			request.Params.Arguments = rewritten
			return next(ctx, request)
		}
	}
}

// acceptedArgNames returns every argument name a tool accepts: the names its
// schema declares, plus the sibling aliases and legacy undeclared names.
func acceptedArgNames(tool mcp.Tool) []string {
	seen := map[string]bool{}
	for _, n := range toolArgNames(tool) {
		seen[n] = true
	}
	for alias := range mcpArgAliasTargets[tool.Name] {
		seen[alias] = true
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
