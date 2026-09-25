package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// TestWrongTypedArgIsNeverReportedAsMissing is the go-slide-creator-6072
// acceptance test, and it sweeps the whole catalogue rather than a hand-written
// list: for every registered tool and every argument its schema marks required,
// a value of the WRONG type must not be reported as MISSING_PARAMETER.
//
// The reviewer's sweep found describe_finding{code: 12345} answering "code is
// required", plan_deck{brief: 12345} answering "brief is required", and four
// more. An agent told a field is missing when it is sitting in the call it just
// sent re-sends it unchanged or drops it; neither is the fix.
func TestWrongTypedArgIsNeverReportedAsMissing(t *testing.T) {
	withToolProfile(t, toolProfileAll)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileAll)
	_, tools := listToolsOverWire(t, s)
	if len(tools) == 0 {
		t.Fatal("no tools advertised")
	}

	var checked int
	for _, tool := range tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s inputSchema: %v", tool.Name, err)
		}
		var schema struct {
			Properties map[string]struct {
				Type any `json:"type"`
			} `json:"properties"`
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("decode %s inputSchema: %v", tool.Name, err)
		}

		for _, arg := range schema.Required {
			var wrong any = 12345
			if typ, _ := schema.Properties[arg].Type.(string); typ == "number" || typ == "integer" {
				wrong = "not-a-number"
			}
			t.Run(tool.Name+"/"+arg, func(t *testing.T) {
				checked++
				res := callToolViaServer(t, s, tool.Name, map[string]any{arg: wrong})
				if !res.IsError {
					t.Fatalf("a wrong-typed %s was accepted: %s", arg, resultText(res))
				}
				finding := findingForPath(t, res, arg)
				if finding == nil {
					// Another required argument was reported first (it really is
					// missing from this one-argument call); that is a different,
					// correct answer.
					return
				}
				if strings.Contains(finding.Code, "MISSING") {
					t.Errorf("%s is present but wrong-typed, reported as %s: %q",
						arg, finding.Code, finding.Message)
				}
				if finding.Evidence["expected_type"] == nil && finding.NextToolCall == nil {
					t.Errorf("%s: no expected_type and no next_tool_call to recover from: %q", arg, finding.Message)
				}
			})
		}
	}
	if checked == 0 {
		t.Fatal("no required arguments were exercised")
	}
	t.Logf("checked %d required arguments across %d tools", checked, len(tools))
}

// findingForPath returns the finding a result reports against one argument path,
// or nil when the result is about something else.
func findingForPath(t *testing.T, res *mcp.CallToolResult, path string) *diagnostics.Finding {
	t.Helper()
	env := parseMCPFindingEnvelope(t, res)
	for i := range env.Findings {
		f := env.Findings[i]
		if p, _ := f.Evidence["path"].(string); p == path {
			return &f
		}
	}
	return nil
}

// TestGetStartedRejectsAWrongTypedTask pins the one silent ignore the reviewer
// found: get_started{task: 12345} answered with the brief workflow as though
// nothing had been asked. An unknown STRING still falls back to brief, which is
// a documented default rather than a mistake.
func TestGetStartedRejectsAWrongTypedTask(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore)

	res := callToolViaServer(t, s, "get_started", map[string]any{"task": 12345})
	if !res.IsError {
		t.Fatalf("a non-string task must be an error, got: %s", resultText(res)[:200])
	}
	text := resultText(res)
	for _, want := range []string{"task must be a string", "brief"} {
		if !strings.Contains(text, want) {
			t.Errorf("error should mention %q, got:\n%s", want, text)
		}
	}

	// An unknown string is still the documented fallback, not an error.
	ok := callToolViaServer(t, s, "get_started", map[string]any{"task": "not-a-real-task"})
	if ok.IsError {
		t.Errorf("an unknown task string must still fall back to brief, got: %s", resultText(ok))
	}
}

// TestListPatternsFieldsErrorMatchesListTemplates pins that the identical
// mistake on the identical argument comes back in one shape, not two:
// list_patterns used to answer without the expected_type / example_value /
// next_tool_call its sibling carries.
func TestListPatternsFieldsErrorMatchesListTemplates(t *testing.T) {
	withToolProfile(t, toolProfileCore)
	s := newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileCore)

	for _, tool := range []string{"list_patterns", "list_templates"} {
		t.Run(tool, func(t *testing.T) {
			res := callToolViaServer(t, s, tool, map[string]any{"fields": 12345})
			if !res.IsError {
				t.Fatalf("a wrong-typed fields must be an error: %s", resultText(res))
			}
			f := findingForPath(t, res, "fields")
			if f == nil {
				t.Fatalf("no finding at path fields: %s", resultText(res))
			}
			if got, _ := f.Evidence["expected_type"].(string); got != "string" {
				t.Errorf("expected_type = %q, want string", got)
			}
			if f.ExampleValue == nil {
				t.Error("no example_value to copy")
			}
			if f.NextToolCall == nil {
				t.Error("no next_tool_call to replay")
			}
		})
	}
}
