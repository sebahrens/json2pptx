package main

import "sort"

// CLI command classification — the reverse of the MCP tool classification.
//
// toolClassifications() (mcp_tool_classification.go) records, for every MCP
// tool, its closest CLI subcommand and — when there is no 1:1 subcommand — an
// MCPOnlyReason. This file records the symmetric metadata for the other
// direction: for every command main.dispatch() recognizes, whether an agent
// would ever invoke it and — when an agent-facing command has no MCP tool — an
// explicit CLIOnlyReason.
//
// The reverse parity gate (TestEveryCLICommandHasMCPParityOrException) uses
// this map to assert that every agent-facing CLI command either mirrors an MCP
// tool (appears as a counterpart in the mcpToCLI parity table) or carries a
// documented CLIOnlyReason. Without it, a CLI-only surface — preflight,
// validate-template, template-check, preview-patterns, or a future command —
// could drift away from the MCP catalog with no agent-visible record of why.
//
// The CLIOnlyReason values are surfaced in get_capabilities().cli_only_commands
// (mcp_capabilities.go), mirroring how MCPOnlyReason surfaces MCP-only tools in
// mcp_tools_available[].

// cliCommandClass describes how one dispatchable CLI command relates to the MCP
// surface.
type cliCommandClass struct {
	// AgentFacing reports whether an agent composing a deck would invoke this
	// command as a workflow step. Server-lifecycle commands (serve, mcp) and
	// meta commands (version, help) are infrastructure, not deck-authoring
	// steps, so they are exempt from MCP parity.
	AgentFacing bool
	// CLIOnlyReason documents why an agent-facing command has no MCP tool. It
	// must be non-empty exactly when the command is agent-facing and has no MCP
	// counterpart (i.e. it is not a value in the mcpToCLI parity table); it must
	// be empty for lifecycle/meta commands and for commands that have MCP
	// parity. The reverse parity gate enforces this invariant.
	CLIOnlyReason string
}

// cliCommandClassifications classifies every command recognized by
// main.dispatch(). Keep it in lockstep with the dispatch switch:
// TestCLICommandClassificationCoversDispatch fails when a dispatch command is
// missing here (or when a stale entry names a command dispatch no longer
// recognizes), and TestEveryCLICommandHasMCPParityOrException fails when an
// agent-facing command lacks both MCP parity and a CLIOnlyReason.
func cliCommandClassifications() map[string]cliCommandClass {
	return map[string]cliCommandClass{
		// --- Server lifecycle / meta (not agent-facing, exempt from parity) ---
		"serve":   {AgentFacing: false},
		"mcp":     {AgentFacing: false},
		"version": {AgentFacing: false},
		"help":    {AgentFacing: false},

		// --- Agent-facing commands with an MCP counterpart (parity via mcpToCLI) ---
		"generate":               {AgentFacing: true},
		"read":                   {AgentFacing: true},
		"validate":               {AgentFacing: true},
		"examine-template":       {AgentFacing: true},
		"validate-output":        {AgentFacing: true},
		"patterns":               {AgentFacing: true},
		"icons":                  {AgentFacing: true},
		"preview-icon":           {AgentFacing: true},
		"tables":                 {AgentFacing: true},
		"skill-info":             {AgentFacing: true},
		"capabilities":           {AgentFacing: true},
		"get-started":            {AgentFacing: true},
		"describe-finding":       {AgentFacing: true},
		"input-schema":           {AgentFacing: true},
		"resolve-theme":          {AgentFacing: true},
		"recommend-pattern":      {AgentFacing: true},
		"preview":                {AgentFacing: true},
		"preview-wireframe":      {AgentFacing: true},
		"repair":                 {AgentFacing: true},
		"score":                  {AgentFacing: true},
		"score-candidates":       {AgentFacing: true},
		"inspect":                {AgentFacing: true},
		"analyze-rhythm":         {AgentFacing: true},
		"plan-deck":              {AgentFacing: true},
		"recommend-visual":       {AgentFacing: true},
		"render-slide":           {AgentFacing: true},
		"render-slide-from-json": {AgentFacing: true},
		"render-thumbnails":      {AgentFacing: true},
		"template-settings":      {AgentFacing: true},
		"data-format-hints":      {AgentFacing: true},
		"shape-catalog":          {AgentFacing: true},
		"audit-palette":          {AgentFacing: true},

		// --- Agent-facing CLI-only commands (no MCP tool; explicit reason) ---
		"preflight": {
			AgentFacing: true,
			CLIOnlyReason: "Developer/CI convenience that bundles every static-check stage " +
				"(input → policy → template → layout → placeholder → grid → pattern → render-projection) " +
				"into one staged finding envelope. Over MCP agents obtain the same diagnostics from " +
				"validate_input and compose score_deck / analyze_deck_rhythm, so the aggregate is not a separate tool.",
		},
		"validate-template": {
			AgentFacing: true,
			CLIOnlyReason: "Emits the standalone validate-template verdict (valid flag + detected capabilities) for a " +
				".pptx path (authoring/CI). MCP agents inspect a registered template via list_templates / " +
				"examine_template(template_name=...), or a not-yet-registered local file via the guarded " +
				"examine_template(template_path=..., base_dir=...) form — whose findings envelope already folds the " +
				"validate-template metadata diagnostics. The boolean valid verdict and the capabilities roll-up are " +
				"not separately surfaced, so this stays CLI-only.",
		},
		"template-check": {
			AgentFacing: true,
			CLIOnlyReason: "Checks a template against the conformance spec (internal/template/conformance.go) for " +
				"template authors/CI; the same logic backs the conformance corpus test. examine_template(template_path=...) " +
				"now gives MCP agents guarded path-based structural inspection, but the conformance ConformanceReport (which " +
				"the CLI merges into conformance.json) is not surfaced over MCP, so this stays CLI-only.",
		},
		// The semantic command group now mirrors the MCP semantic tools
		// (validate_deck_spec / compile_deck_spec / render_deck_spec /
		// explain_deck_spec / list_deck_archetypes / list_slide_kinds), so it has
		// MCP parity via the mcpToCLI table and carries no CLIOnlyReason.
		"semantic": {AgentFacing: true},
		"preview-patterns": {
			AgentFacing: true,
			CLIOnlyReason: "Pre-renders PNG previews for every named pattern — a local gallery-build/CI step that " +
				"needs the render toolchain (LibreOffice + ImageMagick). Over MCP, reproduce a single pattern's preview " +
				"by composing list_patterns -> show_pattern -> expand_pattern -> render_slide_image_from_json, then loop " +
				"over the patterns/templates you care about and assemble your own manifest; the batch gallery itself stays " +
				"CLI-only. See skills/generate-deck/TOOLS.md (Composition recipes).",
		},
	}
}

// previewPatternsMCPRecipe lists, in call order, the MCP tools an agent composes
// to reproduce one tile of the CLI-only `preview-patterns` gallery: discover a
// pattern, learn its value schema + example_values, expand it against a template
// theme into a shape_grid, then render that single-slide JSON to a PNG. The agent
// loops this over the patterns/templates it cares about and assembles its own
// preview manifest from the returned image hashes.
//
// This is the single source of truth behind the preview-patterns CLIOnlyReason
// (surfaced in get_capabilities().cli_only_commands) and the TOOLS.md
// "Composition recipes" section. TestPreviewPatternsCompositionRecipe asserts
// every step is a registered MCP tool and is named on both surfaces, so the
// documented composition path cannot drift away from the tool catalog.
func previewPatternsMCPRecipe() []string {
	return []string{
		"list_patterns",
		"show_pattern",
		"expand_pattern",
		"render_slide_image_from_json",
	}
}

// cliOnlyCommandNames returns the sorted names of every dispatchable CLI command
// that has no MCP tool — i.e. its classification carries a non-empty
// CLIOnlyReason. This is the reverse of mcpOnlyToolNames() and the single source
// of truth for the cli_only_commands list surfaced by get_capabilities.
func cliOnlyCommandNames() []string {
	var names []string
	for name, c := range cliCommandClassifications() {
		if c.CLIOnlyReason != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
