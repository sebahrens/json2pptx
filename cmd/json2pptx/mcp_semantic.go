package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
)

// ---------------------------------------------------------------------------
// Semantic MCP tools — the compact DeckSpec authoring surface
//
// These tools expose internal/semantic (the compact semantic deck-spec model)
// over MCP so an agent can validate, compile, render, and explain a deck spec
// without dropping down to the raw PresentationInput model. They are thin
// adapters over the same internal/semantic entry points the `json2pptx
// semantic` CLI subcommands use (semantic_cmd.go), so the two surfaces cannot
// drift in behavior.
//
// The semantic surface is the recommended default for authoring NEW decks: a
// spec is shorter, the compiler chooses patterns/layouts and enforces rhythm,
// and render findings are mapped back to the semantic source paths the author
// wrote. The raw tools (generate_presentation, validate_input, …) remain the
// lower-level escape hatch and stay fully available.
// ---------------------------------------------------------------------------

// semanticSpecBytes extracts the `spec` argument as raw document bytes plus a
// filename whose extension selects the parser. A JSON object is marshaled to
// JSON bytes (filename "spec.json"); a string is passed through verbatim as
// YAML (filename "spec.yaml" — YAML is a JSON superset, so inline JSON text
// also parses). A missing or empty spec, or a value of the wrong type, yields a
// structured arg-validation error result.
func semanticSpecBytes(tool string, request mcp.CallToolRequest) ([]byte, string, *mcp.CallToolResult) {
	raw, ok := request.GetArguments()["spec"]
	if !ok || raw == nil {
		return nil, "", argMissing(tool, "spec", "object|string", map[string]any{
			"meta":   map[string]any{"title": "My Deck", "archetype": "strategy_proposal"},
			"slides": []any{map[string]any{"kind": "title", "title": "My Deck"}},
		}, nil)
	}
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, "", argMissing(tool, "spec", "object|string", nil, nil)
		}
		return []byte(v), "spec.yaml", nil
	case map[string]any:
		if len(v) == 0 {
			return nil, "", argMissing(tool, "spec", "object|string", nil, nil)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, "", argInvalidValue(tool, "INVALID_PARAMETER", "spec", fmt.Sprintf("spec object could not be encoded: %v", err), "object|string", nil, nil)
		}
		return b, "spec.json", nil
	default:
		return nil, "", argInvalidValue(tool, "INVALID_PARAMETER", "spec",
			fmt.Sprintf("spec must be a JSON object (the DeckSpec) or a YAML/JSON string, got %T", raw), "object|string", nil, nil)
	}
}

// semanticOptionalString reads an optional string-typed MCP argument,
// distinguishing absence from a present-but-wrong-type value. An absent (or
// JSON-null) argument returns ("", false, nil) so the caller can apply its
// default. A value present with any non-string JSON type fails fast with a
// structured INVALID_PARAMETER result instead of being silently dropped — the
// covert-leniency bug where {"strict": true} quietly defaulted to warn. A
// present string (including "") returns (value, true, nil).
func semanticOptionalString(tool, path string, request mcp.CallToolRequest) (string, bool, *mcp.CallToolResult) {
	raw, ok := request.GetArguments()[path]
	if !ok || raw == nil {
		return "", false, nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", false, argInvalidValue(tool, "INVALID_PARAMETER", path,
			fmt.Sprintf("%s must be a string, got %T", path, raw), "string", nil, nil)
	}
	return s, true, nil
}

// semanticOptionalBool reads an optional bool-typed MCP argument, failing fast
// on a present-but-wrong-type value rather than silently treating it as false.
// An absent (or JSON-null) argument returns (false, nil).
func semanticOptionalBool(tool, path string, request mcp.CallToolRequest) (bool, *mcp.CallToolResult) {
	raw, ok := request.GetArguments()[path]
	if !ok || raw == nil {
		return false, nil
	}
	b, ok := raw.(bool)
	if !ok {
		return false, argInvalidValue(tool, "INVALID_PARAMETER", path,
			fmt.Sprintf("%s must be a boolean, got %T", path, raw), "boolean", nil, nil)
	}
	return b, nil
}

// semanticStrictArg parses the optional `strict` advisory-rule strictness
// argument, defaulting to warn when absent. A present-but-wrong-type value or an
// unrecognized string yields a structured arg-validation error result rather
// than silently falling back to warn.
func semanticStrictArg(tool string, request mcp.CallToolRequest) (semantic.Strictness, *mcp.CallToolResult) {
	s, present, errRes := semanticOptionalString(tool, "strict", request)
	if errRes != nil {
		return "", errRes
	}
	if !present || s == "" {
		return semantic.StrictnessWarn, nil
	}
	strictness, perr := parseStrictness(s)
	if perr != nil {
		return "", argInvalidValue(tool, "INVALID_PARAMETER", "strict", perr.Error(), "string", "warn", nil)
	}
	return strictness, nil
}

// --- validate_deck_spec -----------------------------------------------------

func mcpValidateDeckSpecTool() mcp.Tool {
	return mcp.NewTool("validate_deck_spec",
		mcp.WithDescription(`Validate a compact semantic deck spec (DeckSpec) and return the shared finding envelope {schema_version, tool, subcommand, ok, summary, findings[]}. The recommended first check when authoring a NEW deck with the semantic surface: it catches unknown slide kinds/archetypes, missing required payload fields, and advisory rhythm/density issues before you compile or render. ok=false means at least one error-severity finding; warnings/info leave ok=true. Mirrors the `+"`json2pptx semantic validate`"+` CLI. The raw-model equivalent is validate_input over a compiled PresentationInput.`),
		mcp.WithRawOutputSchema(outputSchemaValidateDeckSpec),
		mcp.WithObject("spec",
			mcp.Required(),
			mcp.Description("The semantic DeckSpec to validate, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		),
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict. Controls whether rhythm/density advisories are info, warnings, or errors."),
		),
	)
}

func handleValidateDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, filename, errRes := semanticSpecBytes("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	strictness, errRes := semanticStrictArg("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}

	ds := semantic.Check(filename, data, strictness)
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "validate_deck_spec",
		InputSHA256: diagnostics.ComputeInputSHA256(data),
	}, ds)

	mcpResult, err := api.MCPSuccessResult(ctx, envelope)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal validate_deck_spec response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- compile_deck_spec ------------------------------------------------------

// compileDeckSpecResponse is the compact result of compile_deck_spec. By
// default it reports only the compiled-deck summary and any diagnostics; the
// full raw PresentationInput JSON is included under compiled_json ONLY when the
// caller passes include_compiled_json=true (the compiled deck can be large).
type compileDeckSpecResponse struct {
	OK           bool                 `json:"ok"`
	SlideCount   int                  `json:"slide_count,omitempty"`
	Template     string               `json:"template,omitempty"`
	Diagnostics  []semanticDiagnostic `json:"diagnostics,omitempty"`
	CompiledJSON json.RawMessage      `json:"compiled_json,omitempty"`
	Error        string               `json:"error,omitempty"`
}

func mcpCompileDeckSpecTool() mcp.Tool {
	return mcp.NewTool("compile_deck_spec",
		mcp.WithDescription(`Compile a compact semantic deck spec (DeckSpec) into the raw json2pptx PresentationInput model. Returns a COMPACT result by default — {ok, slide_count, template, diagnostics[]} — so you can confirm the spec lowers cleanly and inspect any blocking findings without paying for the whole compiled deck. Pass include_compiled_json=true to also receive the full PresentationInput under compiled_json (consumable by validate_input / generate_presentation for advanced edits or debugging). ok=false carries the blocking error and diagnostics. Mirrors the `+"`json2pptx semantic compile`"+` CLI.`),
		mcp.WithRawOutputSchema(outputSchemaCompileDeckSpec),
		mcp.WithObject("spec",
			mcp.Required(),
			mcp.Description("The semantic DeckSpec to compile, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		),
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict."),
		),
		mcp.WithString("template",
			mcp.Description("Default template used when the spec pins none (spec template > this > archetype default). Use list_templates to discover names."),
		),
		mcp.WithBoolean("include_compiled_json",
			mcp.Description("When true, include the full compiled raw PresentationInput under compiled_json. Defaults to false (compact output)."),
		),
	)
}

func handleCompileDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, filename, errRes := semanticSpecBytes("compile_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	strictness, errRes := semanticStrictArg("compile_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	templateName, _, errRes := semanticOptionalString("compile_deck_spec", "template", request)
	if errRes != nil {
		return errRes, nil
	}
	includeJSON, errRes := semanticOptionalBool("compile_deck_spec", "include_compiled_json", request)
	if errRes != nil {
		return errRes, nil
	}

	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		res := compileDeckSpecResponse{OK: false, Error: "compile_deck_spec: spec could not be parsed"}
		for _, d := range parseDiags.ToDiagnostics() {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
		return semanticSuccessOrInternal(ctx, "compile_deck_spec", res)
	}

	input, result, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: templateName,
	})
	if err != nil {
		res := compileDeckSpecResponse{OK: false, Error: err.Error()}
		if result != nil {
			for _, d := range result.Diagnostics {
				res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
			}
		}
		return semanticSuccessOrInternal(ctx, "compile_deck_spec", res)
	}

	res := compileDeckSpecResponse{OK: true, SlideCount: len(input.Slides), Template: input.Template}
	if result != nil {
		for _, d := range result.Diagnostics {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
	}
	if includeJSON {
		raw, err := json.Marshal(input)
		if err != nil {
			return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal compiled deck: %v", err)), nil
		}
		res.CompiledJSON = raw
	}
	return semanticSuccessOrInternal(ctx, "compile_deck_spec", res)
}

// --- render_deck_spec -------------------------------------------------------

// renderDeckSpecResponse is the compact result of render_deck_spec: the target
// one-call flow from a semantic spec to a rendered .pptx. It carries the
// artifact path, a quality summary, render/compile diagnostics mapped back to
// the semantic source paths the author wrote, and an explanation summary of the
// compiler's planned decisions.
type renderDeckSpecResponse struct {
	OK          bool                       `json:"ok"`
	Success     bool                       `json:"success"`
	PptxPath    string                     `json:"pptx_path,omitempty"`
	Template    string                     `json:"template,omitempty"`
	SlideCount  int                        `json:"slide_count,omitempty"`
	ContentHash string                     `json:"content_hash,omitempty"`
	DurationMs  int64                      `json:"duration_ms,omitempty"`
	Quality     *QualityScore              `json:"quality_summary,omitempty"`
	Warnings    []string                   `json:"warnings,omitempty"`
	Diagnostics []semanticDiagnostic       `json:"diagnostics,omitempty"`
	Explanation *semantic.DeckExplanation  `json:"explanation_summary,omitempty"`
	Error       string                     `json:"error,omitempty"`
}

// semanticRenderToMCP adapts the CLI-shaped semanticRenderResult into the MCP
// render response, attaching the planned-decisions explanation and exposing the
// artifact path as pptx_path / the ok flag as success (the stable field names
// agents already branch on for generate_presentation).
func semanticRenderToMCP(r semanticRenderResult, explanation *semantic.DeckExplanation) renderDeckSpecResponse {
	return renderDeckSpecResponse{
		OK:          r.OK,
		Success:     r.OK,
		PptxPath:    r.OutputPath,
		Template:    r.Template,
		SlideCount:  r.SlideCount,
		ContentHash: r.ContentHash,
		DurationMs:  r.DurationMs,
		Quality:     r.Quality,
		Warnings:    r.Warnings,
		Diagnostics: r.Diagnostics,
		Explanation: explanation,
		Error:       r.Error,
	}
}

func mcpRenderDeckSpecTool() mcp.Tool {
	return mcp.NewTool("render_deck_spec",
		mcp.WithDescription(`Compile a compact semantic deck spec (DeckSpec) and render it straight to a .pptx — the recommended one-call path for producing a NEW deck. Returns {success, pptx_path, quality_summary, diagnostics[], explanation_summary}: success/ok report whether the artifact was written, pptx_path locates it, quality_summary scores the compiled slides, diagnostics carry compile findings plus render-time fit findings mapped back to the semantic source paths you wrote (raw paths retained as fallback), and explanation_summary reports the compiler's planned archetype/template and per-slide kind/role/family/density/pattern. Strict output validation is the default. A blocking failure returns success=false with the reason in error/diagnostics. Mirrors the `+"`json2pptx semantic render`"+` CLI; the raw-model equivalent is generate_presentation over a compiled PresentationInput.`),
		mcp.WithRawOutputSchema(outputSchemaRenderDeckSpec),
		mcp.WithObject("spec",
			mcp.Required(),
			mcp.Description("The semantic DeckSpec to render, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		),
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict."),
		),
		mcp.WithString("template",
			mcp.Description("Default template used when the spec pins none (spec template > this > archetype default). Use list_templates to discover names."),
		),
		mcp.WithString("output_validation",
			mcp.Description("Post-generation output validation: off, warn, or strict (default). strict refuses to emit a deck with text overflow."),
		),
	)
}

func (mc *mcpConfig) handleRenderDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, filename, errRes := semanticSpecBytes("render_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	strictness, errRes := semanticStrictArg("render_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	templateName, _, errRes := semanticOptionalString("render_deck_spec", "template", request)
	if errRes != nil {
		return errRes, nil
	}

	outputValidation := "strict"
	if v, present, errRes := semanticOptionalString("render_deck_spec", "output_validation", request); errRes != nil {
		return errRes, nil
	} else if present && v != "" {
		parsed, perr := parseOutputValidation(v)
		if perr != nil {
			return argInvalidValue("render_deck_spec", "INVALID_PARAMETER", "output_validation",
				fmt.Sprintf("invalid output_validation %q: must be off, warn, or strict", v), "string", "strict", nil), nil
		}
		outputValidation = parsed
	}

	startTime := time.Now()

	// Parse the spec. A parse error is fatal and has no source map yet, so the
	// findings carry their native semantic paths.
	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		res := renderDeckSpecResponse{OK: false, Success: false, Error: "render_deck_spec: spec could not be parsed"}
		for _, d := range parseDiags.ToDiagnostics() {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
		return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	}

	// The explanation is a pure projection of the plan and works even when the
	// spec still carries advisory findings, so compute it up front.
	explanation := semantic.ExplainSpec(spec)

	// Validate + compile to a raw PresentationInput. Blocking findings abort the
	// render with the diagnostics surfaced on the result.
	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: templateName,
	})
	if err != nil {
		res := semanticRenderToMCP(buildSemanticRenderFailure(compileResult, err), &explanation)
		return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	}

	// Apply the shared pre-render prep a compiled deck still needs (deck defaults
	// and named style references), then render through the shared in-memory
	// runner with the standard native-SVG knobs — the same flow as the CLI.
	applyDefaults(input)
	resolveInputNamedSettingsForDir(mc.templatesDir, input)

	cfg := config.DefaultConfig()
	if mc.templatesDir != "" {
		cfg.Templates.Dir = mc.templatesDir
	}

	runRes, cleanup, renderErr := RunPresentation(ctx, input, RenderOptions{
		OutputDir:        mc.outputDir,
		TemplatesDir:     cfg.Templates.Dir,
		OutputValidation: outputValidation,
		AccentStrategy:   patterns.AccentStrategy(input.AccentStrategy),
		SVGStrategy:      string(cfg.SVG.Strategy),
		SVGScale:         cfg.SVG.Scale,
		SVGNativeCompat:  string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:      cfg.SVG.MaxPNGWidth,
	})
	defer cleanup()
	if renderErr != nil {
		res := semanticRenderToMCP(buildSemanticRenderFailure(compileResult, renderErr), &explanation)
		return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	}

	res := semanticRenderToMCP(buildSemanticRenderSuccess(input, compileResult, runRes, startTime), &explanation)
	return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
}

// --- explain_deck_spec ------------------------------------------------------

func mcpExplainDeckSpecTool() mcp.Tool {
	return mcp.NewTool("explain_deck_spec",
		mcp.WithDescription(`Explain the compiler's planned decisions for a semantic deck spec (DeckSpec) WITHOUT compiling or rendering. Returns {title, archetype, template, rhythm, rhythm_warnings[], slides[{index, kind, role, visual_family, density, title, takeaway, pattern, layout}]}: the resolved archetype/template, the deck-rhythm summary and the advisories to address before rendering, and the concrete pattern/layout each slide will compile into. Use during planning to preview how the spec reads and which visuals it will pick. A spec that cannot be parsed returns a structured error envelope. Mirrors the `+"`json2pptx semantic explain`"+` CLI.`),
		mcp.WithRawOutputSchema(outputSchemaExplainDeckSpec),
		mcp.WithObject("spec",
			mcp.Required(),
			mcp.Description("The semantic DeckSpec to explain, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		),
	)
}

func handleExplainDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	data, filename, errRes := semanticSpecBytes("explain_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}

	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		return api.MCPDiagnosticsError(parseDiags.ToDiagnostics()), nil
	}

	mcpResult, err := api.MCPSuccessResult(ctx, semantic.ExplainSpec(spec))
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal explain_deck_spec response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- list_deck_archetypes ---------------------------------------------------

// archetypeListEntry is one row of list_deck_archetypes: the archetype name, a
// one-line summary, the template the archetype prefers when none is pinned, and
// whether it expects a synthesis/decision slide (the executive flag).
type archetypeListEntry struct {
	Archetype       string `json:"archetype"`
	Summary         string `json:"summary"`
	DefaultTemplate string `json:"default_template,omitempty"`
	Executive       bool   `json:"executive"`
}

func mcpListDeckArchetypesTool() mcp.Tool {
	return mcp.NewTool("list_deck_archetypes",
		mcp.WithDescription(`List the deck archetypes the semantic compiler recognizes for DeckSpec.meta.archetype. Returns {archetypes:[{archetype, summary, default_template, executive}]}: the archetype biases template choice, default slide rhythm, and whether the deck is expected to carry a synthesis/decision slide. Call this when authoring a NEW deck spec to pick the archetype that matches your purpose. The full enum is also embedded in `+"`json2pptx semantic schema`"+`.`),
		mcp.WithRawOutputSchema(outputSchemaListDeckArchetypes),
	)
}

func handleListDeckArchetypes(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out := make([]archetypeListEntry, 0)
	for _, a := range semantic.AllArchetypes() {
		info, _ := semantic.LookupArchetype(a)
		def := semantic.DefaultsFor(a)
		out = append(out, archetypeListEntry{
			Archetype:       string(a),
			Summary:         info.Summary,
			DefaultTemplate: def.Template,
			Executive:       def.Executive,
		})
	}
	mcpResult, err := api.MCPSuccessResult(ctx, map[string]any{"archetypes": out})
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal list_deck_archetypes response: %v", err)), nil
	}
	return mcpResult, nil
}

// --- list_slide_kinds -------------------------------------------------------

// slideKindListEntry is one row of list_slide_kinds: the kind discriminator, a
// one-line summary, and the required/typical payload fields for that kind.
type slideKindListEntry struct {
	Kind            string              `json:"kind"`
	Summary         string              `json:"summary"`
	RequiredFields  []string            `json:"required_fields,omitempty"`
	RequiredAliases map[string][]string `json:"required_aliases,omitempty"`
	TypicalFields   []string            `json:"typical_fields,omitempty"`
}

func mcpListSlideKindsTool() mcp.Tool {
	return mcp.NewTool("list_slide_kinds",
		mcp.WithDescription(`List the slide kinds the semantic compiler recognizes for DeckSpec.slides[].kind. Returns {slide_kinds:[{kind, summary, required_fields, required_aliases, typical_fields}]}: the kind selects a slide's semantic payload shape, required_fields are the payload keys the kind needs to compile, required_aliases maps a required field to interchangeable alias keys (required-one-of — e.g. kpi_snapshot accepts "metrics" in place of "kpis"), and typical_fields are common optional keys. Call this when authoring a NEW deck spec to choose each slide's kind and learn which fields it expects. The full enum is also embedded in `+"`json2pptx semantic schema`"+`.`),
		mcp.WithRawOutputSchema(outputSchemaListSlideKinds),
	)
}

func handleListSlideKinds(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	out := make([]slideKindListEntry, 0)
	for _, k := range semantic.AllSlideKinds() {
		info, _ := semantic.LookupKind(k)
		out = append(out, slideKindListEntry{
			Kind:            string(k),
			Summary:         info.Summary,
			RequiredFields:  info.RequiredFields,
			RequiredAliases: info.RequiredAliases,
			TypicalFields:   info.TypicalFields,
		})
	}
	mcpResult, err := api.MCPSuccessResult(ctx, map[string]any{"slide_kinds": out})
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal list_slide_kinds response: %v", err)), nil
	}
	return mcpResult, nil
}

// semanticSuccessOrInternal marshals a compact semantic result (which may carry
// ok=false for a parse/compile/render failure) as an MCP success result, since
// the failure detail lives inside the structured payload rather than the MCP
// error channel. Only a marshal failure surfaces as an INTERNAL error.
func semanticSuccessOrInternal(ctx context.Context, tool string, v any) (*mcp.CallToolResult, error) {
	mcpResult, err := api.MCPSuccessResult(ctx, v)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal %s response: %v", tool, err)), nil
	}
	return mcpResult, nil
}
