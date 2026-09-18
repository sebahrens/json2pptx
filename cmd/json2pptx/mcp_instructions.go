package main

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
const mcpCompletionRule = "A deck is done only after you render ALL slides of the current revision (render_deck_thumbnails) and inspect every returned image yourself. A passing deterministic gate, score, or validate result is a precondition for that review, never completion. After any repair, re-render and re-inspect."

// mcpQualityWorkflow is the server `instructions` text and get_started's
// quality_workflow field.
const mcpQualityWorkflow = `json2pptx quality workflow:
1. Call get_started first (task: brief | revise | validate-only) for the recommended path.
2. New deck from a brief: author a DeckSpec (list_slide_kinds gives each kind's item_schema and a copy-ready example), check it with validate_deck_spec, then render it with render_deck_spec. make_deck is a skeleton/wireframe only: it fills slides with exemplar placeholder copy and its gate always fails.
3. ` + mcpCompletionRule + `
4. Fix what you see or what diagnostics report at their semantic_path in the DeckSpec (raw decks: repair_slide), then re-render and re-inspect.
5. Never ship exemplar or placeholder content (uses_exemplar_content=true, an "exemplar_content" blocking reason, __FILL__ tokens, SEMANTIC_WEAK_CONTENT).
Unknown tool arguments are rejected with UNKNOWN_PARAMETER and a did_you_mean hint; unknown DeckSpec fields are reported as SEMANTIC_UNKNOWN_FIELD.`
