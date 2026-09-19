package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// MCP server instructions — the quality workflow in one place
//
// MCP-only clients never see skills/generate-deck/SKILL.md, so the workflow
// that produces good decks must travel with the server itself. mcpQualityWorkflow
// is sent as the `instructions` field of the initialize response (see
// newMCPServer) AND echoed verbatim by get_started (quality_workflow), so the two
// surfaces cannot drift. mcpCompletionRule is the single definition of "done"
// shared by the instructions, get_started.completion_protocol.rule, and the
// get_started notes.
// ---------------------------------------------------------------------------

// mcpCompletionRule is the one completion rule every surface states.
const mcpCompletionRule = "A deck is done only after every slide of the CURRENT revision has been rendered (render_deck_thumbnails) and looked at by you. A passing deterministic gate, score, or validate result is a precondition for that review, never completion. After a repair, re-render and re-inspect the slides that changed (render_deck_thumbnails with slide_indices, or render_slide_image for a single one), then make one full-deck pass over the final revision: the revision you ship is the one that has to have been seen."

// mcpQualityWorkflow is the server `instructions` text and get_started's
// quality_workflow field.
const mcpQualityWorkflow = `json2pptx quality workflow:
1. Call get_started first (task: brief | revise | validate-only) for the recommended path.
2. New deck from a brief: author a DeckSpec (list_slide_kinds gives each kind's item_schema and a copy-ready example), check it with validate_deck_spec, then render it with render_deck_spec. make_deck is a skeleton/wireframe only: it fills slides with exemplar placeholder copy and its gate always fails.
3. ` + mcpCompletionRule + `
4. Fix what you see or what diagnostics report at their semantic_path in the DeckSpec (raw decks: repair_slide), then re-render and re-inspect.
5. Never ship exemplar or placeholder content (uses_exemplar_content=true, an "exemplar_content" blocking reason, __FILL__ tokens, SEMANTIC_WEAK_CONTENT).
Unknown tool arguments are rejected with UNKNOWN_PARAMETER and a did_you_mean hint; unknown DeckSpec fields are reported as SEMANTIC_UNKNOWN_FIELD.`

// renderToolingWarning is the line appended to the server instructions when the
// render toolchain is absent. Without it the whole surface looked identical on a
// server that cannot render: initialize said nothing, tools/list still
// advertised render_deck_thumbnails, and the agent learned the truth only when
// the MANDATORY completion step failed — with no documented alternative
// (go-slide-creator-a7fh).
func renderToolingWarning(missing []string) string {
	return fmt.Sprintf(
		"RENDER TOOLING MISSING (%s): render_deck_thumbnails / render_slide_image / inspect_slide_images will fail on this server, so a deck CANNOT be visually approved here. Build and validate the deck as usual, hand back pptx_path (or the json2pptx://deck/<name> resource), and say plainly that the deck is UNREVIEWED — do not claim the completion rule was met. Install LibreOffice and ImageMagick to restore the visual step.",
		strings.Join(missing, ", "))
}

// mcpInstructionsFor returns the server instructions, degraded when the render
// toolchain is missing.
func mcpInstructionsFor(renderAvailable bool, missing []string) string {
	if renderAvailable || len(missing) == 0 {
		return mcpQualityWorkflow
	}
	return mcpQualityWorkflow + "\n" + renderToolingWarning(missing)
}
