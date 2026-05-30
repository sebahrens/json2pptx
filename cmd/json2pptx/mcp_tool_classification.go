package main

import "sort"

// Tool classification metadata.
//
// Every registered MCP tool carries structured metadata so agents can decide
// how to use it when composing a workflow: whether it is a composable
// primitive or an opinionated workflow facade, the workflow phase it serves,
// its side effects (server state, files, render and API-key dependencies), the
// closest CLI subcommand, and — for facades — the primitives it composes.
//
// The metadata is merged into each mcpToolEntry by mcpToolCatalog(), so it
// surfaces on both get_capabilities().mcp_tools_available and the
// `json2pptx capabilities` CLI subcommand. TestEveryRegisteredToolIsClassified
// fails if any registered tool lacks an entry here.

// Tool kind taxonomy: how an agent should treat a tool when composing a
// workflow. Tools are classified by their PRIMARY purpose, not their side
// effects — inspect_slide_images writes thumbnails but exists to return
// findings, so it is a diagnostic, while render_slide_image exists to produce
// the image, so it is a primitive.
const (
	// toolKindPrimitive is a composable building block: a single action or pure
	// transform with a narrow contract (generate a deck, expand a pattern,
	// repair a slide, write a setting). Primitives are the atoms that workflow
	// facades orchestrate.
	toolKindPrimitive = "primitive"
	// toolKindWorkflowFacade is an opinionated multi-step orchestration that
	// chains primitives behind one call (make_deck, auto_repair). Each facade
	// lists the primitives it composes in primitive_alternatives so agents can
	// drive the steps by hand when they need finer control.
	toolKindWorkflowFacade = "workflow_facade"
	// toolKindDiagnostic is a read-only discovery, validation, inspection,
	// scoring, or recommendation tool. It neither mutates server state nor is a
	// terminal deck-producing action.
	toolKindDiagnostic = "diagnostic"
)

// Tool phase taxonomy: the workflow stage a tool normally serves. Mirrors the
// PLAN → VARY → RENDER → REPAIR workflow documented in
// skills/generate-deck/TOOLS.md, plus the session-entry discovery phase and the
// gated settings phase.
const (
	toolPhaseDiscovery = "discovery"
	toolPhasePlan      = "plan"
	toolPhaseVary      = "vary"
	toolPhaseRender    = "render"
	toolPhaseRepair    = "repair"
	toolPhaseSettings  = "settings"
)

// toolClassification carries the structured metadata for one MCP tool.
type toolClassification struct {
	// Kind is one of the toolKind* constants.
	Kind string
	// Phase is one of the toolPhase* constants.
	Phase string
	// MutatesState is true when the tool persists server-side state beyond the
	// response (only the template-settings writers do this). Writing artifact
	// files is reported by WritesFiles, not here.
	MutatesState bool
	// WritesFiles is true when the tool produces artifact files on disk (PPTX,
	// PNG) in the normal default mode. Opt-in modes that add file output (e.g.
	// make_deck/auto_repair visual_qa) do not flip this.
	WritesFiles bool
	// RenderDependency is true when the tool requires the render toolchain
	// (LibreOffice + ImageMagick/pdftoppm) in its default mode.
	RenderDependency bool
	// APIKeyDependency is true when the tool uses ANTHROPIC_API_KEY in its
	// default mode (degrading to a heuristic fallback when unset).
	APIKeyDependency bool
	// CLICounterpart is the closest CLI subcommand. Kept in lockstep with the
	// mcpToCLI parity table by TestToolClassificationMatchesCLIParityTable.
	CLICounterpart string
	// MCPOnlyReason explains why a tool has no 1:1 CLI subcommand (the
	// CLICounterpart is then an approximate workflow). Empty for tools with a
	// direct CLI subcommand.
	MCPOnlyReason string
	// PrimitiveAlternatives lists the primitive tools an agent can drive by hand
	// instead of this tool. Required for every workflow_facade; empty otherwise.
	PrimitiveAlternatives []string
}

// mcpOnlyToolNames returns the sorted names of every registered MCP tool that
// has no 1:1 CLI subcommand — i.e. its classification carries a non-empty
// MCPOnlyReason. This is the single source of truth for the "MCP-only tools"
// list that the CLI help, README, and skills/generate-deck/TOOLS.md must agree
// on; the discovery-doc drift tests assert each of these surfaces lists every
// name returned here.
func mcpOnlyToolNames() []string {
	var names []string
	for name, c := range toolClassifications() {
		if c.MCPOnlyReason != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// validToolKinds is the closed set of accepted Kind values.
func validToolKinds() map[string]bool {
	return map[string]bool{
		toolKindPrimitive:      true,
		toolKindWorkflowFacade: true,
		toolKindDiagnostic:     true,
	}
}

// validToolPhases is the closed set of accepted Phase values.
func validToolPhases() map[string]bool {
	return map[string]bool{
		toolPhaseDiscovery: true,
		toolPhasePlan:      true,
		toolPhaseVary:      true,
		toolPhaseRender:    true,
		toolPhaseRepair:    true,
		toolPhaseSettings:  true,
	}
}

// toolClassifications returns the classification for every registered MCP tool,
// keyed by tool name. Keep this map in lockstep with mcpToolCatalog() — the
// parity is enforced by TestEveryRegisteredToolIsClassified.
func toolClassifications() map[string]toolClassification {
	return map[string]toolClassification{
		// --- Discovery / introspection (read-only) ---
		"get_started":              {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "get-started"},
		"get_capabilities":         {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "capabilities"},
		"get_input_schema":         {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "input-schema"},
		"get_data_format_hints":    {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "data-format-hints"},
		// list_templates writes layout-preview PNG cache files in its default
		// mode (when LibreOffice + ImageMagick are present), so WritesFiles is
		// true. Pass read_only=true to suppress those cache writes; the response
		// side_effects block reports what happened.
		"list_templates":           {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, WritesFiles: true, CLICounterpart: "skill-info"},
		// The detailed chart/diagram capability arrays these tools return are
		// inlined in `json2pptx skill-info` (supported_types.chart_capabilities /
		// diagram_capabilities), not in `json2pptx capabilities` (which carries
		// only the type-name registry). skill-info is therefore the accurate CLI
		// counterpart — see TestChartDiagramCapabilitiesCLICounterpartIsSkillInfo.
		"get_chart_capabilities":   {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "skill-info"},
		"get_diagram_capabilities": {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "skill-info"},
		"get_shape_catalog":        {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "shape-catalog"},
		"resolve_theme":            {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "resolve-theme"},
		"list_icons":               {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "icons"},
		"preview_icon":             {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "preview-icon"},
		"list_patterns":            {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "patterns list"},
		"show_pattern":             {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "patterns show"},
		"table_density_guide":      {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "tables"},
		"list_template_settings":   {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "template-settings list"},
		"examine_template":         {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "examine-template"},
		"describe_finding":         {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "describe-finding"},

		// --- PLAN (per slide intent) ---
		"plan_deck":         {Kind: toolKindDiagnostic, Phase: toolPhasePlan, CLICounterpart: "plan-deck"},
		"recommend_visual":  {Kind: toolKindDiagnostic, Phase: toolPhasePlan, CLICounterpart: "recommend-visual"},
		"recommend_pattern": {Kind: toolKindDiagnostic, Phase: toolPhasePlan, CLICounterpart: "recommend-pattern"},
		"validate_pattern":  {Kind: toolKindDiagnostic, Phase: toolPhasePlan, CLICounterpart: "patterns validate"},
		"expand_pattern":    {Kind: toolKindPrimitive, Phase: toolPhasePlan, CLICounterpart: "patterns expand"},
		"expand_patterns": {
			Kind: toolKindPrimitive, Phase: toolPhasePlan, CLICounterpart: "patterns expand",
			MCPOnlyReason:         "Batch convenience over expand_pattern; CLI users loop `json2pptx patterns expand` per pattern.",
			PrimitiveAlternatives: []string{"expand_pattern"},
		},

		// --- VARY ---
		"analyze_deck_rhythm": {Kind: toolKindDiagnostic, Phase: toolPhaseVary, CLICounterpart: "analyze-rhythm"},
		"score_candidates":    {Kind: toolKindDiagnostic, Phase: toolPhaseVary, CLICounterpart: "score-candidates"},

		// --- RENDER (validate → generate) ---
		"validate_input":               {Kind: toolKindDiagnostic, Phase: toolPhaseRender, CLICounterpart: "validate"},
		"preview_presentation_plan":    {Kind: toolKindDiagnostic, Phase: toolPhaseRender, CLICounterpart: "preview"},
		"preview_slide_wireframe":      {Kind: toolKindDiagnostic, Phase: toolPhaseRender, CLICounterpart: "preview-wireframe"},
		"validate_presentation_output": {Kind: toolKindDiagnostic, Phase: toolPhaseRender, CLICounterpart: "validate-output"},
		"generate_presentation":        {Kind: toolKindPrimitive, Phase: toolPhaseRender, WritesFiles: true, CLICounterpart: "generate"},
		"make_deck": {
			Kind: toolKindWorkflowFacade, Phase: toolPhaseRender, WritesFiles: true, CLICounterpart: "generate",
			MCPOnlyReason:         "Cold-start facade; CLI users assemble JSON and call `json2pptx generate` manually.",
			PrimitiveAlternatives: []string{"plan_deck", "recommend_visual", "expand_pattern", "validate_input", "generate_presentation", "auto_repair"},
		},

		// --- REPAIR (inspect → repair) ---
		"repair_slide": {Kind: toolKindPrimitive, Phase: toolPhaseRepair, CLICounterpart: "repair"},
		"repair_slides_batch": {
			Kind: toolKindPrimitive, Phase: toolPhaseRepair, CLICounterpart: "repair",
			MCPOnlyReason:         "Batches repair_slide; CLI users loop `json2pptx repair` for each slide.",
			PrimitiveAlternatives: []string{"repair_slide"},
		},
		"propose_repairs": {
			Kind: toolKindDiagnostic, Phase: toolPhaseRepair, CLICounterpart: "repair",
			MCPOnlyReason: "Translates findings into repair directives; CLI users map findings to fixes manually and invoke `json2pptx repair`.",
		},
		"apply_deck_patch": {
			Kind: toolKindPrimitive, Phase: toolPhaseRepair, CLICounterpart: "repair",
			MCPOnlyReason: "Pure deck JSON transform; CLI users edit the JSON directly or invoke `json2pptx repair`.",
		},
		"auto_repair": {
			Kind: toolKindWorkflowFacade, Phase: toolPhaseRepair, WritesFiles: true, CLICounterpart: "repair",
			MCPOnlyReason:         "Server-side convergence loop; CLI users chain `json2pptx generate` / `validate` / `repair` manually.",
			PrimitiveAlternatives: []string{"generate_presentation", "score_deck", "propose_repairs", "repair_slides_batch"},
		},
		"inspect_slide_images":         {Kind: toolKindDiagnostic, Phase: toolPhaseRepair, WritesFiles: true, RenderDependency: true, APIKeyDependency: true, CLICounterpart: "inspect"},
		"render_slide_image":           {Kind: toolKindPrimitive, Phase: toolPhaseRepair, WritesFiles: true, RenderDependency: true, CLICounterpart: "render-slide"},
		"render_slide_image_from_json": {Kind: toolKindPrimitive, Phase: toolPhaseRepair, WritesFiles: true, RenderDependency: true, CLICounterpart: "render-slide-from-json"},
		"render_deck_thumbnails":       {Kind: toolKindPrimitive, Phase: toolPhaseRepair, WritesFiles: true, RenderDependency: true, CLICounterpart: "render-thumbnails"},
		"read_presentation":            {Kind: toolKindDiagnostic, Phase: toolPhaseRepair, CLICounterpart: "read"},
		"audit_palette":                {Kind: toolKindDiagnostic, Phase: toolPhaseRepair, RenderDependency: true, CLICounterpart: "audit-palette"},
		"score_deck":                   {Kind: toolKindDiagnostic, Phase: toolPhaseRepair, RenderDependency: true, CLICounterpart: "score"},

		// --- Settings (gated writes) ---
		"register_template_setting": {Kind: toolKindPrimitive, Phase: toolPhaseSettings, MutatesState: true, CLICounterpart: "template-settings register"},
		"delete_template_setting":   {Kind: toolKindPrimitive, Phase: toolPhaseSettings, MutatesState: true, CLICounterpart: "template-settings delete"},

		// --- Semantic compiler (compact DeckSpec authoring; recommended default for new decks) ---
		"validate_deck_spec":   {Kind: toolKindDiagnostic, Phase: toolPhaseRender, CLICounterpart: "semantic validate"},
		"compile_deck_spec":    {Kind: toolKindPrimitive, Phase: toolPhaseRender, CLICounterpart: "semantic compile"},
		"render_deck_spec":     {Kind: toolKindPrimitive, Phase: toolPhaseRender, WritesFiles: true, CLICounterpart: "semantic render"},
		"explain_deck_spec":    {Kind: toolKindDiagnostic, Phase: toolPhasePlan, CLICounterpart: "semantic explain"},
		"list_deck_archetypes": {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "semantic schema"},
		"list_slide_kinds":     {Kind: toolKindDiagnostic, Phase: toolPhaseDiscovery, CLICounterpart: "semantic schema"},
	}
}
