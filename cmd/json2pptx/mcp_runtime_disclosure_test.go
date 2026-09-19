package main

import (
	"strings"
	"testing"
)

// noRenderRuntime is a server whose render toolchain is absent.
func noRenderRuntime() getStartedRuntime {
	return getStartedRuntime{
		RenderAvailable: false,
		MissingCommands: []string{"libreoffice/soffice", "magick"},
		TemplatesDir:    "../../templates",
		OutputDir:       "/tmp/out",
	}
}

// TestGetStartedCarriesRuntime is the go-slide-creator-a7fh acceptance test:
// what the server can actually do rides the FIRST call. The reviewer had to
// reach get_capabilities — 183KB into the session — to learn that this server
// cannot render, after being handed a workflow that ends in rendering.
func TestGetStartedCarriesRuntime(t *testing.T) {
	for _, task := range getStartedAvailableTasks() {
		t.Run(task+"/ready", func(t *testing.T) {
			resp := buildGetStartedResponse(task, testRenderReady())
			if !resp.Runtime.RenderAvailable {
				t.Error("runtime.render_available should be true on a ready server")
			}
			if len(resp.Runtime.MissingCommands) != 0 {
				t.Errorf("missing_commands = %v, want none", resp.Runtime.MissingCommands)
			}
			if resp.Runtime.TemplatesDir == "" || resp.Runtime.OutputDir == "" {
				t.Errorf("runtime should name both directories: %+v", resp.Runtime)
			}
			if resp.Completion.CompleteStatus != "visually_reviewed_current_revision" {
				t.Errorf("complete_status = %q, want the visual-review status", resp.Completion.CompleteStatus)
			}
			if strings.Contains(resp.QualityWorkflow, "RENDER TOOLING MISSING") {
				t.Error("a ready server must not warn about missing tooling")
			}
		})
	}
}

// TestGetStartedDegradesWithoutRenderTooling pins the other half: on a server
// that cannot render, the recommended path must not end in a step that cannot
// run, and the completion rule must be the one that CAN be honoured.
func TestGetStartedDegradesWithoutRenderTooling(t *testing.T) {
	renderTools := map[string]bool{
		"render_deck_thumbnails": true, "render_slide_image": true,
		"inspect_slide_images": true, "submit_visual_review": true,
	}

	for _, task := range getStartedAvailableTasks() {
		t.Run(task, func(t *testing.T) {
			ready := buildGetStartedResponse(task, testRenderReady())
			resp := buildGetStartedResponse(task, noRenderRuntime())

			if resp.Runtime.RenderAvailable {
				t.Fatal("runtime.render_available should be false")
			}
			if len(resp.Runtime.MissingCommands) == 0 {
				t.Error("missing_commands should name what is absent")
			}
			for _, step := range resp.Sequence {
				if renderTools[step.Tool] {
					t.Errorf("sequence still recommends %s, which cannot run here", step.Tool)
				}
			}
			if resp.FastPath != nil {
				for _, step := range resp.FastPath.Steps {
					if renderTools[step.Tool] {
						t.Errorf("fast_path still recommends %s, which cannot run here", step.Tool)
					}
				}
			}
			if resp.Completion.CompleteStatus != "draft_needs_visual_review" {
				t.Errorf("complete_status = %q — a deck cannot be visually approved on this server", resp.Completion.CompleteStatus)
			}
			if !strings.Contains(resp.Completion.Rule, "RENDER TOOLING MISSING") {
				t.Errorf("completion rule does not say why: %q", resp.Completion.Rule)
			}
			if len(resp.Notes) == 0 || !strings.HasPrefix(resp.Notes[0], "RENDER TOOLING MISSING") {
				t.Error("the missing-tooling note should lead the notes")
			}
			if !strings.Contains(resp.QualityWorkflow, "RENDER TOOLING MISSING") {
				t.Error("quality_workflow should carry the same warning as the server instructions")
			}

			// A path that lost its render tail says what to do instead; a path
			// that never rendered (validate-only) is left alone.
			lostSteps := len(ready.Sequence) != len(resp.Sequence)
			last := resp.Sequence[len(resp.Sequence)-1].WhenToCall
			if lostSteps && !strings.Contains(last, "UNREVIEWED") {
				t.Errorf("the final step should say the deck is unreviewed: %q", last)
			}
			if !lostSteps && strings.Contains(last, "UNREVIEWED") {
				t.Errorf("a path that never renders should not disclaim a review: %q", last)
			}
		})
	}
}

// TestInstructionsDegradeWithoutRenderTooling pins the server-instructions half:
// an MCP client that reads nothing but `initialize` still learns that this
// server cannot finish the workflow it is being handed.
func TestInstructionsDegradeWithoutRenderTooling(t *testing.T) {
	ready := mcpInstructionsFor(true, nil)
	if ready != mcpQualityWorkflow {
		t.Error("a ready server's instructions must be the workflow verbatim")
	}

	degraded := mcpInstructionsFor(false, []string{"libreoffice/soffice", "magick"})
	if !strings.HasPrefix(degraded, mcpQualityWorkflow) {
		t.Error("the degraded instructions should still carry the whole workflow")
	}
	for _, want := range []string{
		"RENDER TOOLING MISSING", "libreoffice/soffice", "magick",
		"render_deck_thumbnails", "UNREVIEWED", "pptx_path",
	} {
		if !strings.Contains(degraded, want) {
			t.Errorf("degraded instructions do not mention %q:\n%s", want, degraded)
		}
	}

	// An empty missing list is not a reason to warn.
	if got := mcpInstructionsFor(false, nil); got != mcpQualityWorkflow {
		t.Error("no named missing command means nothing to warn about")
	}
}
