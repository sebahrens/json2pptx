package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-bxve: every step in get_started was {tool, when_to_call}
// with no arguments, so agents called the tools exactly as the prose named
// them — list_templates{} at 153KB instead of 44KB with fields:"compact".

// TestGetStartedArgsTemplatesMatchToolSchemas is the bead's VERIFY: every
// args_template key must be an argument that tool actually declares. A hint
// naming a parameter that does not exist is worse than no hint — the agent
// sends it and gets UNKNOWN_PARAMETER back.
func TestGetStartedArgsTemplatesMatchToolSchemas(t *testing.T) {
	s := strictArgsTestServer(t)
	tools := s.ListTools()
	for tool, args := range getStartedArgTemplates {
		st, ok := tools[tool]
		if !ok {
			// A tool hidden by the active profile is fine to have a hint for —
			// the hint only appears in a sequence that names the tool.
			continue
		}
		declared := map[string]bool{}
		for _, n := range declaredArgNames(st.Tool) {
			declared[n] = true
		}
		for key := range args {
			if !declared[key] {
				t.Errorf("get_started hints %s{%s}, which that tool does not declare (declares %v)",
					tool, key, declaredArgNames(st.Tool))
			}
		}
	}
}

// Every step that names a tool with a hint must carry it, across every task —
// the table is applied centrally so this cannot drift per sequence.
func TestGetStartedStepsCarryTheirArgs(t *testing.T) {
	for _, task := range []string{"brief", "revise", "validate-only", "onboard-template"} {
		t.Run(task, func(t *testing.T) {
			resp := getStartedResponseFor(t, task)
			steps := append([]getStartedStep{}, resp.Sequence...)
			if resp.FastPath != nil {
				steps = append(steps, resp.FastPath.Steps...)
			}
			if len(steps) == 0 {
				t.Fatalf("task %q produced no steps", task)
			}
			for _, step := range steps {
				want := argsTemplateFor(step.Tool)
				if want == nil {
					continue // a tool with nothing worth pre-filling
				}
				if len(step.ArgsTemplate) == 0 {
					t.Errorf("step %q carries no args_template though one is defined", step.Tool)
				}
			}
		})
	}
}

// The token-relevant projections are the whole point: these three default to
// the full payload and almost no first call needs it.
func TestGetStartedHintsTheCheapProjections(t *testing.T) {
	want := map[string]map[string]any{
		"get_capabilities": {"sections": nil},
		"list_templates":   {"fields": "compact"},
		"list_patterns":    {"fields": "compact"},
	}
	for tool, keys := range want {
		got := argsTemplateFor(tool)
		if got == nil {
			t.Errorf("%s has no args_template; its default response is the full payload", tool)
			continue
		}
		for k, v := range keys {
			gv, ok := got[k]
			if !ok {
				t.Errorf("%s args_template is missing %q: %v", tool, k, got)
				continue
			}
			if v != nil && gv != v {
				t.Errorf("%s args_template %s = %v, want %v", tool, k, gv, v)
			}
		}
	}
}

// getStartedResponseFor drives the real handler and decodes its structured
// result.
func getStartedResponseFor(t *testing.T, task string) getStartedResponse {
	t.Helper()
	s := strictArgsTestServer(t)
	res := callToolViaServer(t, s, "get_started", map[string]any{"task": task})
	if res.IsError {
		t.Fatalf("get_started{task:%q}: %s", task, resultText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal get_started structured content: %v", err)
	}
	var resp getStartedResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode get_started response: %v\n%s", err, raw)
	}
	return resp
}

// An unrecognised task still answers with the brief workflow — blocking an
// agent's first call helps nobody — but it must say so. It used to echo
// task:"brief" with no hint that something else had been asked for.
func TestGetStartedWarnsOnAnUnknownTask(t *testing.T) {
	resp := getStartedResponseFor(t, "definitely-not-a-task")
	if resp.Task != "brief" {
		t.Errorf("task = %q, want the brief fallback", resp.Task)
	}
	if resp.TaskWarning == "" {
		t.Fatal("an unknown task was answered silently")
	}
	for _, want := range []string{"definitely-not-a-task", "brief", "revise"} {
		if !strings.Contains(resp.TaskWarning, want) {
			t.Errorf("warning should name the bad task and the valid ones, missing %q: %s", want, resp.TaskWarning)
		}
	}
}

// A valid task carries no warning.
func TestGetStartedValidTaskHasNoWarning(t *testing.T) {
	for _, task := range []string{"", "brief", "revise", "validate-only", "onboard-template"} {
		if w := getStartedResponseFor(t, task).TaskWarning; w != "" {
			t.Errorf("task %q produced a warning: %s", task, w)
		}
	}
}

// quality_workflow repeats the MCP initialize instructions verbatim, which
// every client already received. It is opt-in, and opting in still works.
func TestGetStartedQualityWorkflowIsOptIn(t *testing.T) {
	quiet := getStartedResponseFor(t, "brief")
	if quiet.QualityWorkflow != "" {
		t.Errorf("quality_workflow should be omitted by default, got %d bytes", len(quiet.QualityWorkflow))
	}
	// completion_protocol still carries the rule, in structured form.
	if quiet.Completion.Rule == "" {
		t.Error("completion_protocol.rule must survive the trim — it is the actionable half")
	}

	loud := callGetStartedVerbose(t, "brief")
	if loud.QualityWorkflow == "" {
		t.Error("verbose:true must still return the prose narrative")
	}
}

// The trim has to actually save bytes in the response every agent reads first.
func TestGetStartedDefaultIsSmallerThanVerbose(t *testing.T) {
	quiet := mustMarshalLen(t, getStartedResponseFor(t, "brief"))
	loud := mustMarshalLen(t, callGetStartedVerbose(t, "brief"))
	if quiet >= loud {
		t.Errorf("default response is %d bytes, verbose %d — the trim saved nothing", quiet, loud)
	}
	if quiet > 12*1024 {
		t.Errorf("default get_started response is %d bytes; the budget is 12 KB", quiet)
	}
}

func mustMarshalLen(t *testing.T, v any) int {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return len(raw)
}
