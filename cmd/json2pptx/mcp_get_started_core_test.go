package main

import "testing"

// TestGetStartedBriefToolsAreCore guards against get_started steering agents
// toward tools the default core profile does not advertise: MCP clients can
// only call tools they see in tools/list.
func TestGetStartedBriefToolsAreCore(t *testing.T) {
	core := coreToolSet()
	for _, task := range []string{"brief", "validate-only"} {
		resp := buildGetStartedResponse(task)
		if resp.FastPath != nil {
			if !core[resp.FastPath.Tool] {
				t.Errorf("task=%s fast_path tool %q not in core profile", task, resp.FastPath.Tool)
			}
		}
		for _, step := range resp.Sequence {
			if !core[step.Tool] {
				t.Errorf("task=%s sequence tool %q not in core profile", task, step.Tool)
			}
		}
	}
}
