package main

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// go-slide-creator-r1m3: three tools named the same concept differently from
// their siblings, and each mismatch cost an agent a round-trip.

// TestArgAliases_SiblingNamesAreAccepted is the bead's VERIFY.
func TestArgAliases_SiblingNamesAreAccepted(t *testing.T) {
	s := strictArgsTestServer(t)
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"validate_presentation_output", map[string]any{"pptx_path": "/nonexistent/deck.pptx"}},
		{"resolve_theme", map[string]any{"template": "midnight-blue"}},
		{"plan_deck", map[string]any{"brief": "Series B pitch", "slide_count": 8}},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			res := callToolViaServer(t, s, c.tool, c.args)
			// The call may still fail for its own reasons (a missing file), but
			// never because the argument name was not recognised.
			if text := resultText(res); strings.Contains(text, "UNKNOWN_PARAMETER") {
				t.Errorf("%s rejected the sibling name: %s", c.tool, text)
			}
		})
	}
}

// The declared name keeps working, and wins when both are sent — a caller
// honouring the schema must not have its value discarded.
func TestArgAliases_DeclaredNameWins(t *testing.T) {
	var got map[string]any
	handler := argAliasMiddleware()(func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		got = req.GetArguments()
		return nil, nil
	})
	_, _ = handler(context.Background(), aliasRequest("plan_deck", map[string]any{
		"slide_budget": 5,
		"slide_count":  9,
	}))
	if got["slide_budget"] != 5 {
		t.Errorf("slide_budget = %v, want the declared value 5 to win", got["slide_budget"])
	}
	if _, leaked := got["slide_count"]; leaked {
		t.Errorf("the alias reached the handler: %v", got)
	}
}

// The alias is rewritten, not merely tolerated: the handler reads one name.
func TestArgAliases_RewrittenBeforeTheHandler(t *testing.T) {
	var got map[string]any
	handler := argAliasMiddleware()(func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		got = req.GetArguments()
		return nil, nil
	})
	_, _ = handler(context.Background(), aliasRequest("plan_deck", map[string]any{"slide_count": 9}))
	if got["slide_budget"] != 9 {
		t.Errorf("slide_budget = %v, want 9 rewritten from slide_count", got["slide_budget"])
	}
	if _, leaked := got["slide_count"]; leaked {
		t.Errorf("slide_count reached the handler: %v", got)
	}
}

// TestArgNamingIsConsistentAcrossTools is the registration-time check the bead
// asked for: a concept spelled one way on most tools must be ACCEPTED by that
// spelling everywhere it appears, so the next tool cannot reintroduce the cost.
func TestArgNamingIsConsistentAcrossTools(t *testing.T) {
	s := strictArgsTestServer(t)
	tools := s.ListTools()
	if len(tools) < 40 {
		t.Fatalf("expected the full tool catalog, got %d tools", len(tools))
	}
	// concept -> (declared spellings that mean it, the spelling every such tool
	// must accept).
	concepts := []struct {
		name      string
		spellings []string
		canonical string
	}{
		{"path to a pptx", []string{"pptx_path", "path"}, "pptx_path"},
		{"template name", []string{"template", "template_name"}, "template"},
		{"slide count", []string{"slide_count", "slide_budget"}, "slide_count"},
	}
	for _, c := range concepts {
		for name, tool := range tools {
			declared := declaredArgSet(tool.Tool)
			if !anyDeclared(declared, c.spellings) {
				continue
			}
			accepted := acceptedArgNames(tool.Tool)
			if !acceptsArg(accepted, c.canonical) {
				t.Errorf("%s takes a %s but does not accept %q (accepts %v)",
					name, c.name, c.canonical, accepted)
			}
		}
	}
}

// Every alias must point at an argument its tool actually declares, or the
// rewrite produces a name the handler ignores.
func TestArgAliasTargetsAreDeclared(t *testing.T) {
	s := strictArgsTestServer(t)
	tools := s.ListTools()
	for toolName, aliases := range mcpArgAliasTargets {
		tool, ok := tools[toolName]
		if !ok {
			t.Errorf("alias table names %q, which is not a registered tool", toolName)
			continue
		}
		declared := declaredArgSet(tool.Tool)
		for alias, target := range aliases {
			if !declared[target] {
				t.Errorf("%s: alias %q targets %q, which the schema does not declare", toolName, alias, target)
			}
			if declared[alias] {
				t.Errorf("%s: %q is both declared and aliased", toolName, alias)
			}
		}
	}
}

// aliasRequest builds a CallToolRequest for the middleware under test.
func aliasRequest(tool string, args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Name = tool
	req.Params.Arguments = args
	return req
}

// declaredArgSet is the tool's SCHEMA properties only — not what it accepts.
func declaredArgSet(tool mcp.Tool) map[string]bool {
	out := map[string]bool{}
	for _, n := range declaredArgNames(tool) {
		out[n] = true
	}
	return out
}

func anyDeclared(declared map[string]bool, names []string) bool {
	for _, n := range names {
		if declared[n] {
			return true
		}
	}
	return false
}

// acceptsArg reports whether a sorted accepted-name list contains name.
func acceptsArg(accepted []string, name string) bool {
	i := sort.SearchStrings(accepted, name)
	return i < len(accepted) && accepted[i] == name
}
