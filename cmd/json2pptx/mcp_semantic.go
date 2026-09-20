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
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
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
		return nil, "", argRequired(request, tool, "spec", "object|string", map[string]any{
			"meta":   map[string]any{"title": "My Deck", "archetype": "strategy_proposal"},
			"slides": []any{map[string]any{"kind": "title", "title": "My Deck"}},
		}, nil)
	}
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, "", argRequired(request, tool, "spec", "object|string", nil, nil)
		}
		return []byte(v), "spec.yaml", nil
	case map[string]any:
		if len(v) == 0 {
			return nil, "", argRequired(request, tool, "spec", "object|string", nil, nil)
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

// deckSpecArg declares the required `spec` argument carrying the DeckSpec
// OUTLINE: meta, and slides as objects with a `kind` from the registered enum.
//
// The full closed schema — per-kind oneOf variants, each with
// additionalProperties:false — is 17KB, and embedding it in all four spec tools
// put it twice in the core listing and four times in the full one, saying the
// same thing each time (go-slide-creator-uhaq). It lives on validate_deck_spec,
// the tool the workflow already says to call before rendering; everywhere else
// the outline plus the pointer to list_slide_kinds carries the same information
// an agent can act on. The payload contract itself is unchanged: an unknown
// field is rejected by the compiler as SEMANTIC_UNKNOWN_FIELD whichever tool
// receives it.
func deckSpecArg(desc string) mcp.ToolOption {
	return mcp.WithObject("spec",
		mcp.Required(),
		mcp.Description(desc+deckSpecOutlineNote),
		withDeckSpecOutline(),
	)
}

// deckSpecOrHandleArg is deckSpecArg for the tools that also accept a deck_id:
// spec is no longer strictly required, because naming a stored deck is the
// other way to say which deck the call is about (go-slide-creator-voxp). The
// handler requires exactly one of the two and says so when neither or both
// arrive.
func deckSpecOrHandleArg(desc string) mcp.ToolOption {
	return mcp.WithObject("spec",
		mcp.Description(desc+" Send this OR deck_id, not both."+deckSpecOutlineNote),
		withDeckSpecOutline(),
	)
}

// deckSpecFullSchemaArg is deckSpecOrHandleArg carrying the FULL closed schema.
// Exactly one tool uses it: validate_deck_spec, where a client that validates
// arguments can check a deck before anything is rendered.
func deckSpecFullSchemaArg(desc string) mcp.ToolOption {
	return mcp.WithObject("spec",
		mcp.Description(desc+" Send this OR deck_id, not both."+deckSpecSchemaNote),
		withDeckSpecSchema(),
	)
}

// deckSpecSchemaNote is the tail of the description on the tool that carries the
// full schema.
const deckSpecSchemaNote = " Schema: the DeckSpec JSON Schema (same as `json2pptx semantic schema`); each slides[] entry matches exactly one per-kind variant selected by `kind`. Unknown payload fields are schema-invalid and reported as SEMANTIC_UNKNOWN_FIELD. Field prose is omitted here; call list_slide_kinds for per-kind item_schema and a copy-ready example."

// deckSpecOutlineNote is the tail everywhere else: the shape is declared, the
// per-kind contract is one call away, and the contract still binds.
const deckSpecOutlineNote = " Schema: the DeckSpec outline (meta + slides[].kind from the registered enum). The per-kind payload contract is NOT repeated here — call list_slide_kinds for each kind's item_schema and a copy-ready example, or validate_deck_spec, whose `spec` carries the full closed schema. The contract binds either way: an unknown payload field is reported as SEMANTIC_UNKNOWN_FIELD."

// withDeckSpecOutline merges the DeckSpec outline into the property schema.
func withDeckSpecOutline() mcp.PropertyOption {
	return func(schema map[string]any) {
		for k, v := range semantic.OutlineInlineSchema() {
			switch k {
			case "type", "description", "title":
				continue
			}
			schema[k] = v
		}
	}
}

// withDeckSpecSchema merges the inlined DeckSpec schema into the property
// schema, keeping the property's own type/description.
func withDeckSpecSchema() mcp.PropertyOption {
	return func(schema map[string]any) {
		for k, v := range semantic.CompactInlineSchema() {
			switch k {
			case "type", "description", "title":
				continue
			}
			schema[k] = v
		}
	}
}

// --- validate_deck_spec -----------------------------------------------------

func mcpValidateDeckSpecTool() mcp.Tool {
	return mcp.NewTool("validate_deck_spec",
		mcp.WithDescription(`Validate a compact semantic deck spec (DeckSpec) and return the shared finding envelope {schema_version, tool, subcommand, ok, summary, findings[]}. The recommended first check when authoring a NEW deck with the semantic surface: it catches unknown slide kinds/archetypes, missing required payload fields, and advisory rhythm/density issues before you compile or render. ok=false means at least one error-severity finding; warnings/info leave ok=true. Mirrors the `+"`json2pptx semantic validate`"+` CLI. The raw-model equivalent is validate_input over a compiled PresentationInput.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaValidateDeckSpec)),
		deckSpecFullSchemaArg("The semantic DeckSpec to validate, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		deckHandleToolParams()[0],
		deckHandleToolParams()[1],
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict. Controls whether rhythm/density advisories are info, warnings, or errors."),
		),
	)
}

func (mc *mcpConfig) handleValidateDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, errRes := mc.resolveSpecSource("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	data, filename, deckID, changed := src.Data, src.Filename, src.DeckID, src.ChangedSlides
	strictness, errRes := semanticStrictArg("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}

	ds := semantic.Check(filename, data, strictness)
	// Check() sees only the spec. The defects an agent ships — wrapped titles,
	// 100-word bullet walls, placeholder copy — live in the COMPILED deck, so
	// compile it and run the same collectors validate_input runs
	// (go-slide-creator-05wn).
	ds = append(ds, mc.compiledSpecFindings(filename, data, strictness)...)
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "validate_deck_spec",
		InputSHA256: diagnostics.ComputeInputSHA256(data),
	}, ds)

	// Hand back a handle so the next call in the loop — a render, or a patched
	// re-validate — does not have to re-upload the spec (go-slide-creator-voxp).
	resp := deckSpecEnvelopeResponse{
		FindingEnvelope: envelope,
		DeckID:          mc.rememberDeck(deckID, data, filename, ""),
		ChangedSlides:   changed,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal validate_deck_spec response: %v", err)), nil
	}
	return mcpResult, nil
}

// deckSpecEnvelopeResponse is the validate_deck_spec envelope plus the deck
// handle fields. The envelope is inlined, so every field agents already branch
// on keeps its place at the top level.
type deckSpecEnvelopeResponse struct {
	diagnostics.FindingEnvelope
	DeckID        string `json:"deck_id,omitempty"`
	ChangedSlides []int  `json:"changed_slides,omitempty"`
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
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaCompileDeckSpec)),
		deckSpecArg("The semantic DeckSpec to compile, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
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

	// Constrained mode is enforced here too: the raw_json2pptx escape hatch
	// carries an author's slide payload straight through, so the same
	// shape_grid must get the same verdict whichever path compiled it
	// (go-slide-creator-rs4h).
	designViolations := compiledDesignModeDiagnostics(input)

	res := compileDeckSpecResponse{OK: len(designViolations) == 0, SlideCount: len(input.Slides), Template: input.Template}
	if result != nil {
		for _, d := range result.Diagnostics {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
	}
	for _, d := range designViolations {
		res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
	}
	if err := blockingDesignModeError(designViolations); err != nil {
		res.Error = err.Error()
		return semanticSuccessOrInternal(ctx, "compile_deck_spec", res)
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
	OK          bool   `json:"ok"`
	Success     bool   `json:"success"`
	PptxPath    string `json:"pptx_path,omitempty"`
	Overwrote   bool   `json:"overwrote,omitempty"`
	Publishable *bool  `json:"publishable,omitempty"`

	BlockingReasons []string                  `json:"blocking_reasons,omitempty"`
	Template        string                    `json:"template,omitempty"`
	SlideCount      int                       `json:"slide_count,omitempty"`
	ContentHash     string                    `json:"content_hash,omitempty"`
	DurationMs      int64                     `json:"duration_ms,omitempty"`
	Quality         *QualityScore             `json:"quality_summary,omitempty"`
	Warnings        []string                  `json:"warnings,omitempty"`
	Diagnostics     []semanticDiagnostic      `json:"diagnostics,omitempty"`
	Explanation     *semantic.DeckExplanation `json:"explanation_summary,omitempty"`
	Error           string                    `json:"error,omitempty"`

	// DeckID is the handle for the spec this render used. Send it as deck_id on
	// the next call instead of re-uploading the spec (go-slide-creator-voxp).
	DeckID string `json:"deck_id,omitempty"`
	// ChangedSlides names the 0-based slides a patch on this call changed, so
	// only those thumbnails need re-pulling. Absent when nothing was patched.
	ChangedSlides []int `json:"changed_slides,omitempty"`
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
		Overwrote:   r.Overwrote,
		Publishable: r.Publishable,

		BlockingReasons: r.BlockingReasons,
		Template:        r.Template,
		SlideCount:      r.SlideCount,
		ContentHash:     r.ContentHash,
		DurationMs:      r.DurationMs,
		Quality:         r.Quality,
		Warnings:        r.Warnings,
		Diagnostics:     r.Diagnostics,
		Explanation:     explanation,
		Error:           r.Error,
	}
}

func mcpRenderDeckSpecTool() mcp.Tool {
	return mcp.NewTool("render_deck_spec",
		mcp.WithDescription(`Compile a compact semantic deck spec (DeckSpec) and render it straight to a .pptx — the recommended one-call path for producing a NEW deck. Returns {success, pptx_path, publishable, blocking_reasons[], quality_summary, diagnostics[], explanation_summary}: success/ok report whether the artifact was WRITTEN and publishable whether it is fit to SHIP — a deck can be written and still carry an action:refuse diagnostic or fail the deterministic quality gate, so gate your "done" on publishable, not success. blocking_reasons say why not. pptx_path locates it, quality_summary is an input heuristic over the compiled slides (score on the shared 0-100 scale, basis="input"; not a structural or visual verdict — use score_deck / render tools for those), diagnostics carry compile findings plus render-time fit findings mapped back to the semantic source paths you wrote (raw paths retained as fallback), and explanation_summary reports the compiler's planned archetype/template and per-slide kind/role/family/density/pattern. Strict output validation is the default. A blocking failure returns success=false with the reason in error/diagnostics. Mirrors the `+"`json2pptx semantic render`"+` CLI; the raw-model equivalent is generate_presentation over a compiled PresentationInput.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaRenderDeckSpec)),
		deckSpecOrHandleArg("The semantic DeckSpec to render, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		deckHandleToolParams()[0],
		deckHandleToolParams()[1],
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict."),
		),
		mcp.WithString("template",
			mcp.Description("Default template used when the spec pins none (spec template > this > archetype default). Use list_templates to discover names."),
		),
		mcp.WithString("template_path",
			mcp.Description("Local .pptx to render with, for a template that is not registered on the server. Resolved against base_dir (the server CWD when absent) and MUST stay inside it. Mutually exclusive with template; a template pinned by the spec's meta.template wins over both. Run examine_template(template_path=...) first to check the file has the layouts a deck needs."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory that bounds template_path resolution (the allowed root). Relative template_path values resolve against it; the resolved file must stay inside it. Ignored when template_path is absent."),
		),
		mcp.WithString("output_validation",
			mcp.Description("Post-generation output validation: off, warn, or strict (default). strict refuses to emit a deck with text overflow."),
		),
		mcp.WithString("output_filename",
			mcp.Description("Filename for the rendered .pptx inside the server's output directory. Path components are stripped and a .pptx suffix is added if missing. Omit it and the name is derived from meta.title plus a short digest of the spec, so two different specs never collide and re-rendering the same spec is idempotent. The response reports overwrote:true when the render replaced an existing file."),
		),
	)
}

func (mc *mcpConfig) handleRenderDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, errRes := mc.resolveSpecSource("render_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	data, filename, deckID, changed := src.Data, src.Filename, src.DeckID, src.ChangedSlides
	strictness, errRes := semanticStrictArg("render_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	templateName, _, errRes := semanticOptionalString("render_deck_spec", "template", request)
	if errRes != nil {
		return errRes, nil
	}
	// A handle remembers the template its last render resolved to. Without this,
	// re-rendering a stored deck without repeating the template argument would
	// silently restyle it (go-slide-creator-voxp).
	if templateName == "" {
		templateName = src.Template
	}
	rawTemplatePath, _, errRes := semanticOptionalString("render_deck_spec", "template_path", request)
	if errRes != nil {
		return errRes, nil
	}

	outputFilename, _, errRes := semanticOptionalString("render_deck_spec", "output_filename", request)
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
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck spec parse failed", map[string]any{"tool": "render_deck_spec"})
		res := renderDeckSpecResponse{OK: false, Success: false, Error: "render_deck_spec: spec could not be parsed"}
		for _, d := range parseDiags.ToDiagnostics() {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
		return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	}

	// Every render used to land on <output_dir>/output.pptx, so two calls in one
	// session silently destroyed each other and the reported content_hash stopped
	// matching the file (go-slide-creator-tngh). Default to a name derived from
	// the deck title and a digest of the spec.
	if outputFilename == "" {
		outputFilename = deckSpecOutputFilename(spec.Meta.Title, data)
	}
	mc.logRenderEvent(ctx, mcp.LoggingLevelInfo, "deck render started", map[string]any{
		"tool": "render_deck_spec", "slide_count": len(spec.Slides), "template": spec.Meta.Template,
	})

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
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck spec compilation failed", map[string]any{"tool": "render_deck_spec"})
		res := semanticRenderToMCP(buildSemanticRenderFailure(compileResult, err), &explanation)
		return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	}

	// Constrained mode is enforced before rendering: the raw_json2pptx escape
	// hatch passes an author's slide payload through unchanged, and the same
	// payload is refused by generate_presentation (go-slide-creator-rs4h).
	if designViolations := compiledDesignModeDiagnostics(input); len(designViolations) > 0 {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck render refused by design mode", map[string]any{"tool": "render_deck_spec"})
		appendCompiledDesignModeDiags(compileResult, input)
		failure := buildSemanticRenderFailure(compileResult, blockingDesignModeError(designViolations))
		return semanticSuccessOrInternal(ctx, "render_deck_spec", semanticRenderToMCP(failure, &explanation))
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

	// A caller-supplied .pptx renders the deck when the spec pins no template of
	// its own — the bring-your-own path for a template the server does not have
	// registered (go-slide-creator-ydbk).
	var resolvedTemplatePath string
	if rawTemplatePath != "" && spec.Meta.Template == "" {
		path, d := resolveRequestTemplatePath(request, "render_deck_spec", rawTemplatePath)
		if d != nil {
			mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck template path invalid", map[string]any{"tool": "render_deck_spec"})
			res := renderDeckSpecResponse{OK: false, Success: false, Error: d.Message}
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(*d))
			return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
		}
		resolvedTemplatePath = path
	}

	runRes, cleanup, renderErr := RunPresentation(ctx, input, RenderOptions{
		OutputDir:            mc.outputDir,
		OutputFilename:       outputFilename,
		TemplatesDir:         cfg.Templates.Dir,
		ResolvedTemplatePath: resolvedTemplatePath,
		OutputValidation:     outputValidation,
		AccentStrategy:       patterns.AccentStrategy(input.AccentStrategy),
		SVGStrategy:          string(cfg.SVG.Strategy),
		SVGScale:             cfg.SVG.Scale,
		SVGNativeCompat:      string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:          cfg.SVG.MaxPNGWidth,
	})
	defer cleanup()
	if renderErr != nil {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck render failed", map[string]any{"tool": "render_deck_spec"})
		res := semanticRenderToMCP(buildSemanticRenderFailure(compileResult, renderErr), &explanation)
		res.DeckID = mc.rememberDeck(deckID, data, filename, res.Template)
		res.ChangedSlides = changed
		return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	}

	res := semanticRenderToMCP(buildSemanticRenderSuccess(input, compileResult, runRes, startTime), &explanation)
	// Hand back a handle for the rendered spec, and say which slides the patch
	// that produced this render changed, so the agent re-pulls only those
	// thumbnails (go-slide-creator-voxp).
	res.DeckID = mc.rememberDeck(deckID, data, filename, res.Template)
	res.ChangedSlides = changed
	mc.logRenderEvent(ctx, mcp.LoggingLevelInfo, "deck render finished", map[string]any{
		"tool": "render_deck_spec", "slide_count": len(spec.Slides), "pptx_path": res.PptxPath,
		"duration_ms": time.Since(startTime).Milliseconds(),
	})
	result, err := semanticSuccessOrInternal(ctx, "render_deck_spec", res)
	// The deck itself, as a resource a host can read without touching the
	// server's filesystem (go-slide-creator-fx52).
	return withDeckResourceLink(result, res.PptxPath), err
}

// --- explain_deck_spec ------------------------------------------------------

func mcpExplainDeckSpecTool() mcp.Tool {
	return mcp.NewTool("explain_deck_spec",
		mcp.WithDescription(`Explain the compiler's planned decisions for a semantic deck spec (DeckSpec) WITHOUT compiling or rendering. Returns {title, archetype, template, rhythm, rhythm_warnings[], slides[{index, kind, role, visual_family, density, title, takeaway, pattern, layout}]}: the resolved archetype/template, the deck-rhythm summary and the advisories to address before rendering, and the concrete pattern/layout each slide will compile into. Use during planning to preview how the spec reads and which visuals it will pick. A spec that cannot be parsed returns a structured error envelope. Mirrors the `+"`json2pptx semantic explain`"+` CLI.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaExplainDeckSpec)),
		deckSpecOrHandleArg("The semantic DeckSpec to explain, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		deckHandleToolParams()[0],
		deckHandleToolParams()[1],
	)
}

func (mc *mcpConfig) handleExplainDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, errRes := mc.resolveSpecSource("explain_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	data, filename, deckID, changed := src.Data, src.Filename, src.DeckID, src.ChangedSlides

	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		return api.MCPDiagnosticsError(parseDiags.ToDiagnostics()), nil
	}

	resp := explainDeckSpecResponse{
		DeckExplanation: semantic.ExplainSpec(spec),
		DeckID:          mc.rememberDeck(deckID, data, filename, ""),
		ChangedSlides:   changed,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal explain_deck_spec response: %v", err)), nil
	}
	return mcpResult, nil
}

// explainDeckSpecResponse is the explanation plus the deck handle fields.
type explainDeckSpecResponse struct {
	semantic.DeckExplanation
	DeckID        string `json:"deck_id,omitempty"`
	ChangedSlides []int  `json:"changed_slides,omitempty"`
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
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaListDeckArchetypes)),
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
	// ItemSchema is the closed JSON Schema for one slide of this kind (every
	// payload field the compiler reads; additionalProperties:false).
	ItemSchema map[string]any `json:"item_schema"`
	// Example is a minimal copy-ready slide of this kind (including "kind")
	// that validates with zero findings.
	Example map[string]any `json:"example"`
	// Compositions are the values this kind's optional "pattern" / "layout"
	// override accepts, with the reason each exists. The override silently did
	// nothing for anything outside this list, and the list was not published
	// anywhere, so an agent varying a monotonous run had no way to know what it
	// could ask for (go-slide-creator-u5az).
	Compositions []slideKindComposition `json:"compositions,omitempty"`
}

// slideKindComposition is one composition a kind can be asked for.
type slideKindComposition struct {
	Pattern string `json:"pattern,omitempty"`
	Layout  string `json:"layout,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func mcpListSlideKindsTool() mcp.Tool {
	return mcp.NewTool("list_slide_kinds",
		mcp.WithDescription(`List the slide kinds the semantic compiler recognizes for DeckSpec.slides[].kind. Returns {slide_kinds:[{kind, summary, required_fields, required_aliases, typical_fields, item_schema, example, compositions}]}: the kind selects a slide's semantic payload shape, required_fields are the payload keys the kind needs to compile, required_aliases maps a required field to interchangeable alias keys (required-one-of — e.g. kpi_snapshot accepts "metrics" in place of "kpis"), typical_fields are common optional keys, item_schema is the closed JSON Schema for one slide of the kind (every field the compiler reads, list-entry and chart shapes included; additionalProperties:false), example is a minimal copy-ready slide that validates clean, and compositions lists the values this kind's optional "pattern" / "layout" override accepts (anything else is ignored and reported as SEMANTIC_PATTERN_NOT_AVAILABLE). Call this when authoring a NEW deck spec to choose each slide's kind and learn which fields it expects. The full enum is also embedded in `+"`json2pptx semantic schema`"+`.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaListSlideKinds)),
	)
}

// slideKindCompositions lists the compositions a kind's pattern / layout
// override accepts. The example payload drives the planner, so the list is the
// one a caller authoring that kind would actually be offered.
func slideKindCompositions(k semantic.SlideKind) []slideKindComposition {
	body, _ := semantic.KindExample(k)["body"].(map[string]any)
	if body == nil {
		body = semantic.KindExample(k)
	}
	candidates := semantic.SlideAlternatives(k, body)
	if len(candidates) == 0 {
		return nil
	}
	out := make([]slideKindComposition, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, slideKindComposition{Pattern: c.Pattern, Layout: c.Layout, Reason: c.Reason})
	}
	return out
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
			ItemSchema:      semantic.KindItemSchema(k),
			Example:         semantic.KindExample(k),
			Compositions:    slideKindCompositions(k),
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
// semanticSuccessOrInternal returns a producing tool's result. render_deck_spec
// and compile_deck_spec are the only callers, and both are asked to produce an
// artifact, so MCPResultFor marks a payload that reports ok:false as an error
// (go-slide-creator-swak).
func semanticSuccessOrInternal(ctx context.Context, tool string, v any) (*mcp.CallToolResult, error) {
	mcpResult, err := api.MCPResultFor(ctx, v)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal %s response: %v", tool, err)), nil
	}
	return mcpResult, nil
}

// compiledSpecFindings compiles a spec and runs the shared fit collectors over
// the compiled deck, returning the findings as diagnostics carrying their
// semantic paths. A spec that does not compile returns nothing: its blocking
// errors are already reported by Check.
func (mc *mcpConfig) compiledSpecFindings(filename string, data []byte, strictness semantic.Strictness) []diagnostics.Diagnostic {
	spec, parseDiags := semantic.Parse(filename, data)
	if spec == nil || parseDiags.HasErrors() {
		return nil
	}
	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{Strict: strictness})
	if err != nil || input == nil {
		return nil
	}
	applyDefaults(input)
	resolveInputNamedSettingsForDir(mc.templatesDir, input)

	// validate must say what render will say (go-slide-creator-rs4h).
	designViolations := compiledDesignModeDiagnostics(input)

	var layouts []types.LayoutMetadata
	var slideWidth, slideHeight int64
	var theme *types.ThemeInfo
	if templatePath, cleanup, terr := resolveTemplatePath(input.Template, mc.templatesDir); terr == nil {
		defer cleanup()
		cache := mc.cache
		if cache == nil {
			cache = template.NewMemoryCache(time.Hour)
		}
		if analysis, aerr := getOrAnalyzeTemplate(templatePath, cache); aerr == nil {
			layouts = analysis.Layouts
			slideWidth, slideHeight = analysis.SlideWidth, analysis.SlideHeight
			theme = &analysis.Theme
			resolveCanonicalLayoutIDs(input.Slides, layouts)
		}
	}

	var sm *semantic.SourceMap
	if compileResult != nil {
		sm = compileResult.SourceMap
	}
	findings := collectFitFindings(input, layouts, slideWidth, slideHeight, theme)
	out := make([]diagnostics.Diagnostic, 0, len(findings)+len(designViolations))
	out = append(out, designViolations...)
	for _, f := range findings {
		d := diagnostics.FromFitFinding(f)
		// Geometry-airiness advisories belong to the render/score path: a
		// one-slide spec is inherently sparse, and validate is about what the
		// author wrote. Everything else — text that will not fit, placeholder
		// copy, a wrapped title — is exactly what this pass exists to surface.
		if specValidateAiriness[f.Code] {
			continue
		}
		if semPath, _, mapped := sm.ResolveSemantic(f.Path); mapped && semPath != "" {
			d.Path = semPath
		}
		out = append(out, d)
	}
	return out
}

// specValidateAiriness are the geometry advisories validate_deck_spec does not
// report: they describe how much of the box the text fills, which a one-slide
// spec cannot control and which the render and score paths already report.
var specValidateAiriness = map[string]bool{
	patterns.ErrCodeSparseLayout:        true,
	patterns.ErrCodeSparseFill:          true,
	patterns.ErrCodeSlideUnderused:      true,
	patterns.ErrCodeCellUnderfilled:     true,
	patterns.ErrCodePatternUnderfilled:  true,
	patterns.ErrCodeSparseSingleRowFlow: true,
	patterns.ErrCodeOvertallFlowLane:    true,
	patterns.ErrCodeSlideNearlyEmpty:    true,
}
