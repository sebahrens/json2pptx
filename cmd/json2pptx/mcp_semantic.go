package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/slidepath"
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
const deckSpecSchemaNote = " Schema: full closed DeckSpec contract. Call list_slide_kinds for field prose and examples. Unknown payload fields report SEMANTIC_UNKNOWN_FIELD."

// deckSpecOutlineNote is the tail everywhere else: the shape is declared, the
// per-kind contract is one call away, and the contract still binds.
const deckSpecOutlineNote = " Schema: DeckSpec outline. Call list_slide_kinds for examples; pass kinds and fields:[item_schema] for a chosen kind's closed schema, then validate_deck_spec. Unknown payload fields still report SEMANTIC_UNKNOWN_FIELD."

// withDeckSpecOutline merges the DeckSpec outline into the property schema.
func withDeckSpecOutline() mcp.PropertyOption {
	return func(schema map[string]any) {
		// semanticSpecBytes accepts an object or a YAML/JSON document string.
		// Object-only keywords below remain applicable only to object values.
		schema["type"] = []string{"object", "string"}
		for k, v := range semantic.OutlineInlineSchema() {
			switch k {
			case "type", "description", "title":
				continue
			}
			schema[k] = v
		}
		admitSpecDocumentString(schema)
	}
}

// admitSpecDocumentString makes the spec property's object-only composition
// keywords accept the documented YAML/JSON string form too. "type" already
// lists string, but the root oneOf (slides XOR structure) did not: for a
// string, "required" is vacuously true, so each branch's "not required" fails,
// no branch matches, and a validating MCP host rejected the string before it
// reached the server (go-slide-creator-b7qqg.5). Each branch now applies to
// objects only, and one more branch admits the string.
func admitSpecDocumentString(schema map[string]any) {
	branches, ok := schema["oneOf"].([]any)
	if !ok {
		return
	}
	out := make([]any, 0, len(branches)+1)
	for _, b := range branches {
		branch, ok := b.(map[string]any)
		if !ok {
			out = append(out, b)
			continue
		}
		guarded := make(map[string]any, len(branch)+1)
		for k, v := range branch {
			guarded[k] = v
		}
		guarded["type"] = "object"
		out = append(out, guarded)
	}
	schema["oneOf"] = append(out, map[string]any{
		"type":        "string",
		"minLength":   1,
		"description": "The DeckSpec as a YAML or JSON document string.",
	})
}

// withDeckSpecSchema merges the compact, reference-preserving DeckSpec schema
// into the property, keeping the property's own type/description.
func withDeckSpecSchema() mcp.PropertyOption {
	return func(schema map[string]any) {
		schema["type"] = []string{"object", "string"}
		for k, v := range semantic.CompactSchemaAt("#/properties/spec") {
			switch k {
			case "type", "description", "title":
				continue
			}
			schema[k] = v
		}
		admitSpecDocumentString(schema)
	}
}

// withSpecOrDeckIDChoice adds the top-level XOR that ToolInputSchema cannot
// represent directly. Keep InputSchema populated for in-process tooling; the
// raw form is what MCP clients receive over tools/list.
func withSpecOrDeckIDChoice(tool mcp.Tool) mcp.Tool {
	schema := map[string]any{
		"type":       "object",
		"properties": tool.InputSchema.Properties,
		"oneOf": []any{
			map[string]any{"required": []string{"spec"}},
			map[string]any{"required": []string{"deck_id"}},
		},
		"dependentRequired": map[string]any{"patch": []string{"deck_id"}},
	}
	if len(tool.InputSchema.Required) > 0 {
		schema["required"] = tool.InputSchema.Required
	}
	if len(tool.InputSchema.Defs) > 0 {
		schema["$defs"] = tool.InputSchema.Defs
	}
	if tool.InputSchema.AdditionalProperties != nil {
		schema["additionalProperties"] = tool.InputSchema.AdditionalProperties
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("%s input schema: %v", tool.Name, err))
	}
	tool.RawInputSchema = raw
	// mcp-go selects RawInputSchema only when the structured Type is empty.
	// Keep Properties for in-process argument discovery and parity checks.
	tool.InputSchema.Type = ""
	return tool
}

// --- validate_deck_spec -----------------------------------------------------

func mcpValidateDeckSpecTool() mcp.Tool {
	return withSpecOrDeckIDChoice(mcp.NewTool("validate_deck_spec", withToolOptions([]mcp.ToolOption{
		mcp.WithDescription(`Validate a DeckSpec by running its render into a scratch directory: the same findings render_deck_spec reports for that spec and template. template echoes the template measured on. A finding blocks only when severity is error (blocking:true); ok=false means one does. patch_verified:true: the finding's next_tool_call patch was tried and clears it. Also reads a stored deck (read, find).`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaValidateDeckSpec)),
		deckSpecFullSchemaArg("The semantic DeckSpec to validate, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
	}, deckHandleToolParams("validate_deck_spec"), []mcp.ToolOption{
		mcp.WithString("read",
			mcp.Description(`Return the stored deck instead of validating: "spec", "history" (revisions and each slide's last change), "diff:2..5" (changes between two revisions), or a slide id / index.`),
		),
		mcp.WithString("find",
			mcp.Description("Text or number to locate anywhere in the spec; alone it returns hits[{path, slide_id, index, excerpt}] instead of validating."),
		),
		mcp.WithString("replace",
			mcp.Description("With find: rewrite every hit as one stored patch, then validate."),
		),
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness (default warn): whether rhythm/density advisories are info, warnings, or errors."),
			mcp.Enum("off", "warn", "strict"),
		),
		mcp.WithString("template",
			mcp.Description("Template to measure on; overrides meta.template for this call."),
		),
		mcp.WithArray("templates", mcp.WithStringItems(),
			mcp.Description(`Also measure on these, or ["all"]: template_results[{template, ok, summary, findings}].`),
		),
		mcp.WithString("base_dir",
			mcp.Description("Root for relative asset paths, as on render_deck_spec."),
		),
	})...))
}

func (mc *mcpConfig) handleValidateDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, errRes := mc.resolveSpecSource("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	// The read side of the deck store: read-back, history and find return
	// what the server holds without validating it (go-slide-creator-83kru,
	// yxf1k, rq1z9).
	readArgs, errRes := parseDeckStoreReadArgs("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	if res, handled := mc.deckSpecReadResult(ctx, "validate_deck_spec", readArgs, src); handled {
		return res, nil
	}
	var replaced []deckFindHit
	if readArgs.Replace != nil {
		if src, replaced, errRes = replaceInSpecSource("validate_deck_spec", src, readArgs); errRes != nil {
			return errRes, nil
		}
	}
	data, filename := src.Data, src.Filename
	strictness, errRes := semanticStrictArg("validate_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	argTemplate, _, errRes := semanticOptionalString("validate_deck_spec", "template", request)
	if errRes != nil {
		return errRes, nil
	}

	parsedSpec, parseDiags := semantic.Parse(filename, data)
	// Check() sees only the spec. Everything else an agent ships — a wrapped
	// title, a pattern that needs more height than the template gives it, a raw
	// slide whose pattern rejects a key — is found by rendering, so validate
	// renders: the same run render_deck_spec makes, into a scratch directory
	// (go-slide-creator-05wn, go-slide-creator-3rn3s). A spec with blocking
	// spec-level errors is still evaluated, with the errors set aside
	// (go-slide-creator-ipahe).
	eval, ds := evaluateSpecFindings(filename, data, strictness, func(spec *semantic.DeckSpec, _ []byte) specEvaluation {
		return mc.evaluateDeckSpec(ctx, request, src, spec, strictness, argTemplate)
	})
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "validate_deck_spec",
		InputSHA256: diagnostics.ComputeInputSHA256(data),
		Template:    eval.Template,
	}, ds)
	stampEnvelopeFindings(&envelope, eval.Diagnostics)
	// A spec that parses is stored even when it has findings, so the next
	// patch can fix them; dry_run stores nothing (go-slide-creator-j77xe). The
	// scratch render above stored nothing and marked nothing as rendered: this
	// commit is the call's only write, and it binds the deck to the first
	// template a call named for it (go-slide-creator-2dit4).
	outcome := mc.commitDeck(deckCommit{
		Tool: "validate_deck_spec", Src: src, Spec: data, Filename: filename,
		Template: deckTemplateSource{Template: eval.Choice.Bind},
		Store:    parsedSpec != nil && !parseDiags.HasErrors() && !src.DryRun,
	})
	if outcome.Stale {
		return staleDeckSpecResult("validate_deck_spec", src.DeckID), nil
	}
	// One remedy per finding; a patch that is complete as written is tried on
	// the spec before it is offered (go-slide-creator-micna, -vihnl).
	remedies := newRemedyContext(filename, data, outcome.DeckID)
	remedies.run = mc.specTrialRunner(ctx, request, src, strictness, argTemplate)
	remedies.before = trialFindingsOfSpecCheck(ds)
	if eval.Evaluated {
		remedies.before = trialFindings(eval.Diagnostics)
	}
	remedies.gateUnknown = eval.RunFailed
	remedies.remedyEnvelope(&envelope, eval.Diagnostics)
	for i := range envelope.Findings {
		f := &envelope.Findings[i]
		if !outcome.Stored {
			prefixUnstoredPatch(f.NextToolCall, src)
		}
		if path, ok := f.Evidence["path"].(string); ok {
			if id := outcome.State.slideIDForPath(path); id != "" {
				f.Evidence["slide_id"] = id
			}
		}
	}
	trimEnvelopeForMCP(&envelope)
	for i := range envelope.Findings {
		// The code carries its namespace; a second copy on every finding is
		// 300 bytes of a twelve-flaw response (go-slide-creator-c2j5b).
		envelope.Findings[i].Category = ""
	}
	// One address per finding: a JSON Pointer into the spec the author sent,
	// and the slide's 1-based number (go-slide-creator-pilpn).
	shapeEnvelopeFindings(&envelope, eval.Diagnostics, newSpecDoc(filename, data))
	// The other templates the call asked about, each as its blocking findings
	// and warnings (go-slide-creator-ifkxs).
	templateResults, errRes := mc.templateResults(ctx, request, src, strictness, eval, envelope)
	if errRes != nil {
		return errRes, nil
	}

	// Hand back a handle so the next call in the loop — a render, or a patched
	// re-validate — does not have to re-upload the spec (go-slide-creator-voxp).
	resp := deckSpecEnvelopeResponse{
		FindingEnvelope: envelope,
		TemplateSource:  eval.TemplateSource,
		Warnings:        eval.Warnings,
		Waivers:         eval.Waivers,
		TemplateResults: templateResults,
		DeckID:          outcome.DeckID,
		Stored:          outcome.Stored,
		Revision:        outcome.Revision,
		ChangedSlides:   outcome.Changed,
		SlideChanges:    outcome.Changes,
	}
	if readArgs.Replace != nil {
		resp.setHits(replaced)
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal validate_deck_spec response: %v", err)), nil
	}
	// A spec that cannot even be decoded (malformed document, wrong container
	// or field type) is a failed call, not a validation verdict on a deck:
	// flag it so hosts that branch on isError see it (go-slide-creator-6p9mm).
	// A decodable spec with invalid content stays an ok=false verdict.
	if parsedSpec == nil {
		mcpResult.IsError = true
	}
	return mcpResult, nil
}

// specEvaluation is what validate_deck_spec learned by running the spec.
type specEvaluation struct {
	// Evaluated reports that the spec compiled and Diagnostics is its finding
	// set; false leaves the caller with the spec-level diagnostics.
	Evaluated   bool
	Diagnostics []semanticDiagnostic
	// Template is the template the findings were measured on; TemplateSource
	// says what chose it.
	Template       string
	TemplateSource string
	Warnings       []string
	Waivers        []findingWaiver
	Choice         specTemplateChoice
	// CompileFailed reports that the spec did not compile: Diagnostics are the
	// compile diagnostics and nothing was rendered.
	CompileFailed bool
	// RunFailed reports that the run was refused or failed, so the checks made
	// on a finished deck (the quality gate) did not run.
	RunFailed bool
	// Salvaged reports that the spec has blocking spec-level errors and
	// Diagnostics also carry the findings of the rest of it
	// (go-slide-creator-ipahe).
	Salvaged bool
}

// evaluateDeckSpec compiles a parsed spec and runs it exactly as
// render_deck_spec would, returning that run's diagnostics.
func (mc *mcpConfig) evaluateDeckSpec(ctx context.Context, request mcp.CallToolRequest, src specSource, spec *semantic.DeckSpec, strictness semantic.Strictness, argTemplate string) specEvaluation {
	choice := resolveSpecTemplate(spec.Meta.Template, argTemplate, "", src)
	spec = choice.evaluated(spec)
	eval := specEvaluation{Choice: choice, TemplateSource: choice.Source, Warnings: choice.Warnings}
	eval.Template = explainSpecWithTemplate(spec, choice.Default).Template

	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{Strict: strictness, DefaultTemplate: choice.Default})
	if err != nil || input == nil {
		// A spec that does not compile reports what render reports for it: the
		// compile diagnostics, which include the post-compile preflight that
		// the spec-level check does not run.
		eval.CompileFailed = true
		if err != nil && compileResult != nil {
			eval.Evaluated = true
			eval.Diagnostics = buildSemanticRenderFailure(compileResult, err).Diagnostics
		}
		return eval
	}
	eval.Evaluated = true
	eval.Template = input.Template

	// validate must say what render will say (go-slide-creator-rs4h).
	if designViolations := compiledDesignModeDiagnostics(input); len(designViolations) > 0 {
		appendCompiledDesignModeDiags(compileResult, input)
		eval.Diagnostics = buildSemanticRenderFailure(compileResult, blockingDesignModeError(designViolations)).Diagnostics
		return eval
	}

	byo := src.TemplatePath != "" && argTemplate == "" && spec.Meta.Template == ""
	if input.Template == "" && !byo {
		// No template anywhere: nothing to render on. Report what does not
		// depend on one and say that the rest did not run.
		eval.TemplateSource = ""
		eval.Warnings = append(eval.Warnings, unpinnedTemplateWarning("", ""))
		eval.Diagnostics, eval.Waivers = templateFreeDiagnostics(input, compileResult)
		return eval
	}

	dir, removeDir, dirErr := trialRenderDir()
	if dirErr != nil {
		d := diagnostics.Diagnostic{Code: string(diagnostics.CodeOutputDir), Message: "validate_deck_spec could not create its scratch directory: " + dirErr.Error(), Severity: diagnostics.SeverityError}
		eval.Diagnostics = []semanticDiagnostic{semanticDiagFromCompile(d)}
		return eval
	}
	defer removeDir()
	run := mc.runCompiledDeckSpec(ctx, "validate_deck_spec", request, src, spec, input, compileResult, specRunOptions{
		ArgTemplate:      argTemplate,
		OutputDir:        dir,
		OutputFilename:   "validate.pptx",
		OutputValidation: "strict",
		Start:            time.Now(),
	})
	if run.TemplatePath != "" {
		eval.Template = filepath.Base(run.TemplatePath)
	}
	if run.Early != nil {
		// The run could not start (an unresolvable template): render answers
		// with these findings alone, so validate does too.
		eval.Diagnostics = []semanticDiagnostic{}
		for _, d := range run.EarlyDiagnostics {
			eval.Diagnostics = append(eval.Diagnostics, semanticDiagFromCompile(d))
		}
		return eval
	}
	eval.Diagnostics = run.Result.Diagnostics
	eval.Waivers = run.Result.Waivers
	eval.RunFailed = !run.Result.OK
	if spec.Meta.Template == "" && choice.Source != "deck_id" {
		eval.Warnings = append(eval.Warnings, unpinnedTemplateWarning(eval.Template, choice.Source))
	}
	return eval
}

// templateResult is a spec's verdict on one template: whether it renders
// ready there, and the findings that block or warn. Notes (info) are counted
// in the summary and left out.
type templateResult struct {
	Template string                  `json:"template"`
	OK       bool                    `json:"ok"`
	Summary  string                  `json:"summary"`
	Findings []templateResultFinding `json:"findings,omitempty"`
}

type templateResultFinding struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Path        string `json:"path"`
	SlideNumber int    `json:"slide_number,omitempty"`
	Occurrences int    `json:"occurrences,omitempty"`
	Message     string `json:"message"`
}

// maxTemplateResults bounds how many templates one call measures on.
const maxTemplateResults = 16

// templateResults evaluates the spec on every template the call's templates
// argument lists. The call's own template, when listed, is reported from the
// evaluation already made.
func (mc *mcpConfig) templateResults(ctx context.Context, request mcp.CallToolRequest, src specSource, strictness semantic.Strictness, own specEvaluation, ownEnvelope diagnostics.FindingEnvelope) ([]templateResult, *mcp.CallToolResult) {
	raw, present := request.GetArguments()["templates"]
	if !present || raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, argInvalidValue("validate_deck_spec", diagnostics.CodeInvalidParameter, "templates",
			`templates must be a non-empty list of template names, or ["all"]`, "array", []any{"midnight-blue", "modern"}, nil)
	}
	var names []string
	for _, v := range list {
		name, isText := v.(string)
		name = strings.TrimSpace(name)
		if !isText || name == "" {
			return nil, argInvalidValue("validate_deck_spec", diagnostics.CodeInvalidParameter, "templates",
				`templates must be a non-empty list of template names, or ["all"]`, "array", []any{"midnight-blue", "modern"}, nil)
		}
		if name == "all" {
			names = append(names, embeddedTemplateNames()...)
			continue
		}
		names = append(names, name)
	}
	seen := map[string]bool{}
	out := make([]templateResult, 0, len(names))
	doc := newSpecDoc(src.Filename, src.Data)
	for _, name := range names {
		if seen[name] || len(out) == maxTemplateResults {
			continue
		}
		seen[name] = true
		if name == own.Template {
			out = append(out, summarizeTemplateResult(name, ownEnvelope))
			continue
		}
		eval, ds := evaluateSpecFindings(src.Filename, src.Data, strictness, func(spec *semantic.DeckSpec, _ []byte) specEvaluation {
			return mc.evaluateDeckSpec(ctx, request, src, spec, strictness, name)
		})
		envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "validate_deck_spec", Template: name}, ds)
		shapeEnvelopeFindings(&envelope, eval.Diagnostics, doc)
		out = append(out, summarizeTemplateResult(name, envelope))
	}
	return out, nil
}

// summarizeTemplateResult reduces one template's finding envelope, already
// addressed in the authored spec, to its verdict.
func summarizeTemplateResult(name string, envelope diagnostics.FindingEnvelope) templateResult {
	res := templateResult{Template: name, OK: envelope.OK, Summary: envelope.Summary}
	for _, f := range envelope.Findings {
		if f.Severity == diagnostics.SeverityInfo || f.Path == nil {
			continue
		}
		entry := templateResultFinding{Code: f.Code, Severity: string(f.Severity), Path: *f.Path, Occurrences: f.Occurrences, Message: f.Message}
		if f.SlideNumber != nil {
			entry.SlideNumber = *f.SlideNumber
		}
		res.Findings = append(res.Findings, entry)
	}
	return res
}

// specTrialRunner returns the function that validates a variant of the call's
// spec exactly as the call's own spec is validated: same template choice, same
// strictness, a scratch render. It stores nothing.
func (mc *mcpConfig) specTrialRunner(ctx context.Context, request mcp.CallToolRequest, src specSource, strictness semantic.Strictness, argTemplate string) func([]byte) ([]trialFinding, bool) {
	return func(spec []byte) ([]trialFinding, bool) {
		trial := src
		trial.Data, trial.Filename = spec, jsonSpecFilename(src.Filename)
		eval, ds := evaluateSpecFindings(trial.Filename, spec, strictness, func(parsed *semantic.DeckSpec, _ []byte) specEvaluation {
			return mc.evaluateDeckSpec(ctx, request, trial, parsed, strictness, argTemplate)
		})
		if eval.Evaluated {
			return trialFindings(eval.Diagnostics), true
		}
		return trialFindingsOfSpecCheck(ds), true
	}
}

// templateFreeDiagnostics is the finding set of a compiled deck with no
// template to render on: its compile diagnostics and the fit collectors that
// need none.
func templateFreeDiagnostics(input *PresentationInput, cr *semantic.CompileResult) ([]semanticDiagnostic, []findingWaiver) {
	var diags []semanticDiagnostic
	for _, d := range cr.Diagnostics {
		diags = append(diags, semanticDiagFromCompile(d))
	}
	fit := collapseRotatedAccentFindings(dedupFitFindings(collectFitFindings(input, nil, 0, 0, nil)))
	patterns.SortCanonical(fit, slidepath.SlideIndex)
	diags = append(diags, finishFitDiagnostics(cr.SourceMap, cr.IR, fit)...)
	policy := newFindingPolicy(cr.IR)
	policy.applyWaivers(diags)
	deckSpecWording(diags, input, cr.IR)
	return groupRootCauses(diags), policy.recorded()
}

// deckSpecEnvelopeResponse is the validate_deck_spec envelope plus the deck
// handle fields. The envelope is inlined, so every field agents already branch
// on keeps its place at the top level.
type deckSpecEnvelopeResponse struct {
	diagnostics.FindingEnvelope
	// TemplateSource says what chose the envelope's template: "meta.template",
	// "template argument", "deck_id" or "archetype default"
	// (go-slide-creator-2dit4).
	TemplateSource string `json:"template_source,omitempty"`
	// Warnings are call-level notes that are not findings on the deck: an
	// unpinned template, an argument the spec overrides.
	Warnings []string `json:"warnings,omitempty"`
	// Waivers records the storyline findings the deck waived.
	Waivers []findingWaiver `json:"waivers,omitempty"`
	// TemplateResults is the spec's verdict on each template the call listed in
	// templates (go-slide-creator-ifkxs).
	TemplateResults []templateResult `json:"template_results,omitempty"`
	DeckID          string           `json:"deck_id,omitempty"`
	// Stored says whether deck_id now holds the spec this call acted on, and
	// Revision which revision that is (go-slide-creator-j77xe).
	Stored   bool `json:"stored"`
	Revision int  `json:"revision,omitempty"`
	// ChangedSlides lists the 0-based slides that look different from the
	// stored revision the call started from; SlideChanges classifies every
	// affected slide (go-slide-creator-v5e9h). ChangedSlides is always present.
	ChangedSlides []int         `json:"changed_slides"`
	SlideChanges  []slideChange `json:"slide_changes,omitempty"`

	// Read results (read / find / replace).
	Spec      json.RawMessage     `json:"spec,omitempty"`
	Slide     json.RawMessage     `json:"slide,omitempty"`
	SlideRef  *slideRef           `json:"slide_ref,omitempty"`
	Hits      *[]deckFindHit      `json:"hits,omitempty"`
	HitCount  *int                `json:"hit_count,omitempty"`
	Revisions []deckRevisionEntry `json:"revisions,omitempty"`
	Slides    []deckSlideHistory  `json:"slides,omitempty"`
	// Diff answers read "diff:A..B": revision B against revision A.
	Diff *deckRevisionDiff `json:"diff,omitempty"`
}

// replaceInSpecSource applies find + replace to the call's spec as one edit
// and returns the rewritten hits (go-slide-creator-yxf1k).
func replaceInSpecSource(tool string, src specSource, a deckStoreReadArgs) (specSource, []deckFindHit, *mcp.CallToolResult) {
	canonical, name := canonicalSpec(src.Filename, src.Data)
	var doc any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return src, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "find",
			fmt.Sprintf("the spec could not be decoded for find: %v", err), "string", nil, nil)
	}
	next := 0
	if src.Handle != nil {
		next = src.Handle.NextSlideID
	}
	assignSlideIDs(doc, next)
	withIDs, err := json.Marshal(doc)
	if err != nil {
		return src, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "find",
			fmt.Sprintf("the spec could not be re-encoded: %v", err), "string", nil, nil)
	}
	hits, ops := findInSpec(doc, specDeckState(name, withIDs, src.Template), a.Find, a.Replace)
	if len(ops) == 0 {
		return src, hits, nil
	}
	rewritten, err := json.Marshal(doc)
	if err != nil {
		return src, nil, argInvalidValue(tool, diagnostics.CodeInvalidParameter, "replace",
			fmt.Sprintf("the rewritten spec could not be re-encoded: %v", err), "string", nil, nil)
	}
	src.Data, src.Filename = rewritten, name
	src.RawPatch = append(append([]any{}, src.RawPatch...), ops...)
	return src, hits, nil
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
		mcp.WithDescription(`Compile a semantic DeckSpec into raw PresentationInput. Returns {ok, slide_count, template, diagnostics[]} without the full deck by default; include_compiled_json=true adds compiled_json for validate_input or generate_presentation. Parse errors use a finding envelope; unknown kinds include available kinds and a discovery call. Other blocking failures return ok=false with diagnostics. Mirrors the `+"`json2pptx semantic compile`"+` CLI.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaCompileDeckSpec)),
		deckSpecArg("The semantic DeckSpec to compile, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict."),
			mcp.Enum("off", "warn", "strict"),
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

	doc := newSpecDoc(filename, data)
	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		ds := parseDiags.ToDiagnostics()
		enrichSemanticKindDiagnostics(ds)
		result := api.MCPDiagnosticsError(ds)
		if envelope, ok := result.StructuredContent.(diagnostics.FindingEnvelope); ok {
			shapeEnvelopeFindings(&envelope, nil, doc)
			result.StructuredContent = envelope
		}
		return result, nil
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
		shapeRenderDiagnostics(res.Diagnostics, doc)
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
	shapeRenderDiagnostics(res.Diagnostics, doc)
	if err := blockingDesignModeError(designViolations); err != nil {
		res.Error = err.Error()
		return semanticSuccessOrInternal(ctx, "compile_deck_spec", res)
	}
	// A blocking diagnostic is never beside ok:true: placeholder copy the
	// product itself emitted compiles, and still is not a deck
	// (go-slide-creator-327g6).
	if blocking := blockingFindingReasons(res.Diagnostics); len(blocking) > 0 {
		res.OK = false
		res.Error = "the spec compiles but is not ready: " + strings.Join(blocking, "; ")
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
	OK                   bool   `json:"ok"`
	Success              bool   `json:"success"`
	PptxPath             string `json:"pptx_path,omitempty"`
	Overwrote            bool   `json:"overwrote,omitempty"`
	DeterministicReady   *bool  `json:"deterministic_ready,omitempty"`
	Publishable          *bool  `json:"publishable,omitempty"`
	ManualReviewRequired *bool  `json:"manual_review_required,omitempty"`

	BlockingReasons              []string             `json:"blocking_reasons,omitempty"`
	DeterministicBlockingReasons []string             `json:"deterministic_blocking_reasons,omitempty"`
	Template                     string               `json:"template,omitempty"`
	SlideCount                   int                  `json:"slide_count,omitempty"`
	ContentHash                  string               `json:"content_hash,omitempty"`
	DurationMs                   int64                `json:"duration_ms,omitempty"`
	Quality                      *QualityScore        `json:"quality_summary,omitempty"`
	Warnings                     []string             `json:"warnings,omitempty"`
	Diagnostics                  []semanticDiagnostic `json:"diagnostics,omitempty"`
	// Waivers records the storyline findings the deck waived (meta.waivers or
	// its archetype) and how many findings each turned into an advisory.
	Waivers     []findingWaiver           `json:"waivers,omitempty"`
	Explanation *semantic.DeckExplanation `json:"explanation_summary,omitempty"`
	Error       string                    `json:"error,omitempty"`

	// DiagnosticsOmitted counts the diagnostics a compact patch response left
	// out: non-blocking findings on slides the patch did not change.
	DiagnosticsOmitted int `json:"diagnostics_omitted,omitempty"`

	// DeckID is the handle for the spec this render used. Send it as deck_id on
	// the next call instead of re-uploading the spec (go-slide-creator-voxp).
	DeckID string `json:"deck_id,omitempty"`
	// Stored says whether deck_id now holds the spec this call rendered: a
	// refused patched render and a dry run store nothing. Revision numbers the
	// stored spec (go-slide-creator-j77xe).
	Stored   bool `json:"stored"`
	Revision int  `json:"revision,omitempty"`
	// ChangedSlides names the 0-based slides that LOOK different from the last
	// rendered revision (every slide on a first render), so only those
	// thumbnails need pulling. Always present. SlideChanges classifies every
	// affected slide, including the ones that only moved, were renumbered or
	// had their notes edited (go-slide-creator-v5e9h).
	ChangedSlides []int         `json:"changed_slides"`
	SlideChanges  []slideChange `json:"slide_changes,omitempty"`
	// Slides is the deck's table of contents: id, index and slide_number per
	// slide (go-slide-creator-1w3uo). A compact patch response leaves it out;
	// slide_changes already names every slide whose index moved.
	Slides []slideRef `json:"slides,omitempty"`

	// compact selects the patch-render encoding of the two summaries; see
	// MarshalJSON.
	compact bool
	// NextToolCall chains the loop: the first blocking diagnostic's fix, else
	// render_deck_thumbnails for the changed slides (go-slide-creator-z3pbp).
	NextToolCall *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
}

// semanticRenderToMCP adapts the CLI-shaped semanticRenderResult into the MCP
// render response, attaching the planned-decisions explanation and exposing the
// artifact path as pptx_path / the ok flag as success (the stable field names
// agents already branch on for generate_presentation).
func semanticRenderToMCP(r semanticRenderResult, explanation *semantic.DeckExplanation) renderDeckSpecResponse {
	return renderDeckSpecResponse{
		OK:                   r.OK,
		Success:              r.OK,
		PptxPath:             r.OutputPath,
		Overwrote:            r.Overwrote,
		DeterministicReady:   r.DeterministicReady,
		Publishable:          r.Publishable,
		ManualReviewRequired: r.ManualReviewRequired,

		BlockingReasons:              r.BlockingReasons,
		DeterministicBlockingReasons: r.DeterministicBlockingReasons,
		Template:                     r.Template,
		SlideCount:                   r.SlideCount,
		ContentHash:                  r.ContentHash,
		DurationMs:                   r.DurationMs,
		Quality:                      r.Quality,
		Warnings:                     r.Warnings,
		Diagnostics:                  r.Diagnostics,
		Waivers:                      r.Waivers,
		Explanation:                  explanation,
		Error:                        r.Error,
	}
}

func mcpRenderDeckSpecTool() mcp.Tool {
	return withSpecOrDeckIDChoice(mcp.NewTool("render_deck_spec", withToolOptions([]mcp.ToolOption{
		mcp.WithDescription(`Compile a DeckSpec and render it to a .pptx — the recommended one-call path for a NEW deck. Returns {success, pptx_path, deterministic_ready, publishable, blocking_reasons[], quality_summary, diagnostics[], waivers[], explanation_summary}. success/ok mean the artifact was WRITTEN; deterministic_ready means no blocking diagnostic (severity error, blocking:true) remains, and deterministic_blocking_reasons names each by code and path. publishable also needs an approved all-slide visual verdict and is false on a fresh render: render every slide with render_deck_thumbnails, inspect the images, then record the verdict with submit_visual_review. diagnostics are validate_deck_spec's findings for the same spec and template, at JSON Pointer paths; quality_summary is an input heuristic (0-100, basis="input"; not a visual verdict). Parse/template errors use a finding envelope; other failures use success=false.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaRenderDeckSpec)),
		deckSpecOrHandleArg("The semantic DeckSpec to render, as a JSON object ({meta:{…}, slides:[{kind, …}]}) or a YAML/JSON string."),
	}, deckHandleToolParams("render_deck_spec"), []mcp.ToolOption{
		mcp.WithBoolean("verbose",
			mcp.Description("true: full summaries on a patch render (default: changed slides only)."),
		),
		mcp.WithString("strict",
			mcp.Description("Advisory-rule strictness: off, warn (default), or strict."),
			mcp.Enum("off", "warn", "strict"),
		),
		mcp.WithString("template",
			mcp.Description("Template for this call; overrides meta.template (patch /meta/template to keep it). list_templates lists names. The first one named binds the deck_id."),
		),
		mcp.WithString("template_path",
			mcp.Description("Local .pptx to render with (a template the server does not have); resolved against base_dir and MUST stay inside it. Not with template; meta.template wins over it. Run examine_template(template_path=...) first to check its layouts."),
		),
		mcp.WithString("base_dir",
			mcp.Description("Absolute directory relative paths resolve against: asset references in the spec (image.path, photos, raw_json2pptx images / icons / backgrounds), with the same guards as raw deck generation, and template_path, which must stay inside it. Defaults to the deck_id's last render root, else the server CWD. The deck_id remembers it along with a template_path, so later deck_id-only calls reuse the same template file and root."),
		),
		mcp.WithString("output_validation",
			mcp.Description("Post-generation output validation (default strict: refuses to emit a deck with text overflow)."),
			mcp.Enum("off", "warn", "strict"),
		),
		mcp.WithString("output_filename",
			mcp.Description("Filename for the rendered .pptx inside the server's output directory. Path components are stripped and a .pptx suffix is added if missing. Omit it and the name derives from meta.title plus a short digest of the spec: different specs never collide and re-rendering one is idempotent. overwrote:true reports a replaced file."),
		),
	})...))
}

func (mc *mcpConfig) handleRenderDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, errRes := mc.resolveSpecSource("render_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	data, filename := src.Data, src.Filename
	verbose, errRes := semanticOptionalBool("render_deck_spec", "verbose", request)
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
	// A handle remembers the template it is bound to. Without this,
	// re-rendering a stored deck without repeating the template argument would
	// silently restyle it (go-slide-creator-voxp).
	argTemplate := templateName
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
	var choice specTemplateChoice
	// byoTemplatePath / assetBaseDir are what this render resolved, stored on
	// the handle so a deck_id-only follow-up keeps them (go-slide-creator-b7qqg.8).
	var byoTemplatePath, assetBaseDir string
	finish := func(res renderDeckSpecResponse) (*mcp.CallToolResult, error) {
		// The handle keeps the template it is bound to; only the first call
		// that names one binds it (go-slide-creator-2dit4).
		stored := deckTemplateSource{Template: choice.Bind, BaseDir: assetBaseDir}
		if byoTemplatePath != "" && !choice.OneOff && choice.Bind == "" {
			stored = deckTemplateSource{TemplatePath: byoTemplatePath, BaseDir: assetBaseDir}
		}
		if choice.OneOff && src.TemplatePath != "" {
			// A bound bring-your-own template was vetted against its own root.
			stored.BaseDir = ""
		}
		// A call rendered on a template the deck is not bound to leaves a file
		// that looks unlike the bound template's: the change list and the
		// last-render baseline are measured on what was actually rendered.
		renderIdentity := ""
		if choice.OneOff {
			renderIdentity = firstNonEmpty(byoTemplatePath, res.Template)
		}
		fin := renderDeckSpecFinish{Src: src, Template: stored, RenderIdentity: renderIdentity, TemplateWarnings: choice.Warnings, Verbose: verbose}
		if rawTemplatePath == "" {
			fin.Trial = mc.specTrialRunner(ctx, request, src, strictness, argTemplate)
		}
		return mc.finishRenderDeckSpec(ctx, res, fin)
	}

	// Parse the spec. A parse error is fatal and has no source map yet, so the
	// findings carry their native semantic paths.
	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck spec parse failed", map[string]any{"tool": "render_deck_spec"})
		// The findings validate_deck_spec reports for the same spec: every
		// spec-level problem in one pass, and the findings of the slides the
		// errors do not touch (go-slide-creator-ipahe).
		eval, ds := evaluateSpecFindings(filename, data, strictness, func(reduced *semantic.DeckSpec, _ []byte) specEvaluation {
			return mc.evaluateDeckSpec(ctx, request, src, reduced, strictness, argTemplate)
		})
		return specFailureResult(filename, data, eval, ds), nil
	}

	// meta.template outranks the template / template_path arguments. That was
	// silent: an agent that passed template=X rendered Y and never knew why
	// (go-slide-creator-6p9mm). Say which one won and how to switch.
	choice = resolveSpecTemplate(spec.Meta.Template, argTemplate, rawTemplatePath, src)
	spec = choice.evaluated(spec)
	templateName = choice.Default

	// Every render used to land on <output_dir>/output.pptx, so two calls in one
	// session silently destroyed each other and the reported content_hash stopped
	// matching the file (go-slide-creator-tngh). Default to a name derived from
	// the deck title and a digest of the spec.
	if outputFilename == "" {
		// Slide ids never render, so they stay out of the digest: the spec an
		// agent sent and the stored copy (which has ids assigned) share a name.
		outputFilename = deckSpecOutputFilename(spec.Meta.Title, specWithoutSlideIDs(data))
	}
	mc.logRenderEvent(ctx, mcp.LoggingLevelInfo, "deck render started", map[string]any{
		"tool": "render_deck_spec", "slide_count": semantic.ExpandedSlideCount(spec), "template": spec.Meta.Template,
	})

	// The explanation is a pure projection of the plan and works even when the
	// spec still carries advisory findings, so compute it up front.
	explanation := explainSpecWithTemplate(spec, templateName)

	// Validate + compile to a raw PresentationInput. Blocking findings abort the
	// render with the diagnostics surfaced on the result.
	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: templateName,
	})
	if err != nil {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck spec compilation failed", map[string]any{"tool": "render_deck_spec"})
		failure := buildSemanticRenderFailure(compileResult, err)
		// A spec that does not compile still reports the findings of the slides
		// its errors do not touch, as validate_deck_spec does.
		if eval, ok := salvagedEvaluation(filename, data, strictness, func(reduced *semantic.DeckSpec, _ []byte) specEvaluation {
			return mc.evaluateDeckSpec(ctx, request, src, reduced, strictness, argTemplate)
		}); ok {
			failure.Diagnostics = eval.Diagnostics
			failure.Warnings = append(failure.Warnings, eval.Warnings...)
			failure.Waivers = eval.Waivers
		}
		return finish(semanticRenderToMCP(failure, &explanation))
	}

	reconcileExplanationWithCompiled(&explanation, input)

	// Constrained mode is enforced before rendering: the raw_json2pptx escape
	// hatch passes an author's slide payload through unchanged, and the same
	// payload is refused by generate_presentation (go-slide-creator-rs4h).
	if designViolations := compiledDesignModeDiagnostics(input); len(designViolations) > 0 {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck render refused by design mode", map[string]any{"tool": "render_deck_spec"})
		appendCompiledDesignModeDiags(compileResult, input)
		failure := buildSemanticRenderFailure(compileResult, blockingDesignModeError(designViolations))
		return finish(semanticRenderToMCP(failure, &explanation))
	}

	// Run the compiled deck through the shared runner. validate_deck_spec calls
	// the same function with a scratch output directory, which is what makes
	// its findings this render's findings (go-slide-creator-3rn3s).
	run := mc.runCompiledDeckSpec(ctx, "render_deck_spec", request, src, spec, input, compileResult, specRunOptions{
		ArgTemplate:      argTemplate,
		RawTemplatePath:  rawTemplatePath,
		OutputDir:        mc.outputDir,
		OutputFilename:   outputFilename,
		OutputValidation: outputValidation,
		Start:            startTime,
	})
	assetBaseDir, byoTemplatePath = run.BaseDir, run.TemplatePath
	if run.Early != nil {
		return run.Early, nil
	}
	if !run.Result.OK {
		mc.logRenderEvent(ctx, mcp.LoggingLevelWarning, "deck render failed", map[string]any{"tool": "render_deck_spec"})
		return finish(semanticRenderToMCP(run.Result, &explanation))
	}
	built := run.Result
	recordDeterministicGate(built.OutputPath, built.DeterministicBlockingReasons)
	res := semanticRenderToMCP(built, &explanation)
	// Hand back a handle for the rendered spec, and say which slides the patch
	// that produced this render changed, so the agent re-pulls only those
	// thumbnails (go-slide-creator-voxp).
	mc.logRenderEvent(ctx, mcp.LoggingLevelInfo, "deck render finished", map[string]any{
		"tool": "render_deck_spec", "slide_count": len(input.Slides), "pptx_path": res.PptxPath,
		"duration_ms": time.Since(startTime).Milliseconds(),
	})
	result, err := finish(res)
	// The deck itself, as a resource a host can read without touching the
	// server's filesystem (go-slide-creator-fx52).
	return withDeckResourceLink(result, res.PptxPath), err
}

// --- explain_deck_spec ------------------------------------------------------

func mcpExplainDeckSpecTool() mcp.Tool {
	return withSpecOrDeckIDChoice(mcp.NewTool("explain_deck_spec", withToolOptions([]mcp.ToolOption{
		mcp.WithDescription(`Explain the compiler's planned decisions for a semantic deck spec (DeckSpec) WITHOUT compiling or rendering. Returns {title, archetype, template, rhythm, rhythm_warnings[], slides[{index, kind, role, visual_family, density, title, takeaway, pattern, layout, alternatives[{pattern,layout,reason}]}]}: the resolved archetype/template, deck-rhythm advisories, selected composition, and supported alternatives for each slide. Use during planning to preview how the spec reads and which visuals it will pick. A spec that cannot be parsed returns a structured error envelope. Mirrors the ` + "`json2pptx semantic explain`" + ` CLI.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaExplainDeckSpec)),
		deckSpecOrHandleArg("The semantic DeckSpec to explain, as a JSON object ({meta:{…}, slides:[{kind, …}]}). A raw YAML/JSON string is also accepted."),
	}, deckHandleToolParams("explain_deck_spec"))...))
}

func (mc *mcpConfig) handleExplainDeckSpec(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, errRes := mc.resolveSpecSource("explain_deck_spec", request)
	if errRes != nil {
		return errRes, nil
	}
	data, filename, deckID := src.Data, src.Filename, src.DeckID

	spec, parseDiags := semantic.Parse(filename, data)
	if parseDiags.HasErrors() {
		return api.MCPDiagnosticsError(parseDiags.ToDiagnostics()), nil
	}

	explanation := explainSpecWithTemplate(spec, src.Template)
	commit := deckCommit{Tool: "explain_deck_spec", Src: src, Spec: data, Filename: filename, Store: !src.DryRun}
	if explanation.Template == "" {
		outcome := mc.commitDeck(commit)
		if outcome.Stale {
			return staleDeckSpecResult("explain_deck_spec", deckID), nil
		}
		resp := explainDeckSpecResponse{
			DeckExplanation: explanation,
			DeckID:          outcome.DeckID,
			ChangedSlides:   outcome.Changed,
		}
		return api.MCPSuccessResult(ctx, resp)
	}
	var cache types.TemplateCache
	if mc.cache != nil {
		cache = mc.cache
	}
	if layouts, templateDiagnostic := semanticTemplateLayouts(explanation.Template, mc.templatesDir, cache); templateDiagnostic == nil {
		reconcileExplanationTemplateCoverage(&explanation, layouts)
	} else {
		return api.MCPDiagnosticsError([]diagnostics.Diagnostic{*templateDiagnostic}), nil
	}
	outcome := mc.commitDeck(commit)
	if outcome.Stale {
		return staleDeckSpecResult("explain_deck_spec", deckID), nil
	}
	resp := explainDeckSpecResponse{
		DeckExplanation: explanation,
		DeckID:          outcome.DeckID,
		ChangedSlides:   outcome.Changed,
	}
	mcpResult, err := api.MCPSuccessResult(ctx, resp)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal explain_deck_spec response: %v", err)), nil
	}
	return mcpResult, nil
}

// explainSpecWithTemplate applies the same template precedence as Compile:
// a spec pin wins, then the caller's selected or remembered template, then
// the archetype default already supplied by ExplainSpec.
func explainSpecWithTemplate(spec *semantic.DeckSpec, defaultTemplate string) semantic.DeckExplanation {
	explanation := semantic.ExplainSpec(spec)
	if spec.Meta.Template == "" && defaultTemplate != "" {
		explanation.Template = defaultTemplate
	}
	return explanation
}

// reconcileExplanationWithCompiled makes the render's explanation_summary
// report what was actually compiled. The plan keeps a visual kind's
// blank-title layout and visual family even when the slide degraded to a
// content or two-column slide, so an agent checking "did the visual land"
// read the pre-degrade answer. Only a 1:1 slide mapping is reconciled.
func reconcileExplanationWithCompiled(explanation *semantic.DeckExplanation, input *PresentationInput) {
	if explanation == nil || input == nil || len(explanation.Slides) != len(input.Slides) {
		return
	}
	for i := range explanation.Slides {
		planned := &explanation.Slides[i]
		compiled := input.Slides[i]
		if compiled.Pattern == nil && compiled.ShapeGrid == nil {
			planned.Pattern = ""
		}
		layout := compiled.LayoutID
		if layout == "" {
			layout = compiled.SlideType
		}
		if planned.Layout == "blank-title" && layout != "" && layout != "blank-title" {
			planned.Layout = layout
			if planned.VisualFamily != semantic.FamilyChart {
				planned.VisualFamily = semantic.FamilyText
			}
		}
	}
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
	ItemSchema map[string]any `json:"item_schema,omitempty"`
	// Example is a minimal copy-ready slide of this kind (including "kind")
	// that validates with no error or warning.
	Example map[string]any `json:"example"`
	// Compositions are the values this kind's optional "pattern" / "layout"
	// override accepts, with the reason each exists. The override silently did
	// nothing for anything outside this list, and the list was not published
	// anywhere, so an agent varying a monotonous run had no way to know what it
	// could ask for (go-slide-creator-u5az).
	Compositions []slideKindComposition `json:"compositions,omitempty"`
	// ComposedExample is a copy-ready raw_json2pptx slide whose compose
	// envelope puts several supporting views that prove ONE title on one
	// slide (dominant chart + KPI region + timeline footer). It rides only the
	// raw_json2pptx row and only when kinds names raw_json2pptx, so the
	// compact catalog stays small. Same source as skill-info's last compose
	// example (go-slide-creator-tknmi).
	ComposedExample map[string]any `json:"composed_example,omitempty"`
	// Budgets are the kind's per-field text budgets: title, subtitle and
	// takeaway measured on the requested template (or the tightest of the
	// shipped ones), then the fixed budgets the compiler enforces. Present
	// only when a template or fields:["budgets"] is requested
	// (go-slide-creator-iubjb).
	Budgets []slideKindBudget `json:"budgets,omitempty"`
}

// slideKindComposition is one composition a kind can be asked for.
type slideKindComposition struct {
	Pattern string `json:"pattern,omitempty"`
	Layout  string `json:"layout,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func mcpListSlideKindsTool() mcp.Tool {
	return mcp.NewTool("list_slide_kinds",
		mcp.WithDescription(`Discover DeckSpec slide kinds: each kind's summary, required_fields, required_aliases, typical_fields and one copy-ready example. fields adds detail, omitted by default: item_schema (the kind's closed JSON Schema), compositions (its pattern/layout overrides), budgets (per-field text budgets; title, subtitle and takeaway measured on template, else the tightest across shipped templates). kinds:["raw_json2pptx"] also returns composed_example: one slide whose compose envelope puts several views (chart + KPIs + timeline) under one title.`),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaListSlideKinds)),
		mcp.WithArray("kinds",
			mcp.Description("Exact kind names to return, e.g. [\"kpi_snapshot\"]. Omit for all."),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("fields",
			mcp.Description("Detail to include."),
			mcp.Items(map[string]any{"type": "string", "enum": []string{"item_schema", "compositions", "budgets"}}),
		),
		mcp.WithString("template",
			mcp.Description("Template name; adds budgets measured on it."),
		),
		mcp.WithBoolean("preview",
			mcp.Description("true: also return an image of each named kind's example (kinds: 1-4) as render_deck_spec renders it on template."),
		),
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

// handleListSlideKinds serves list_slide_kinds without a server config: the
// CLI's `semantic kinds` reads the same catalogue through it.
func handleListSlideKinds(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return (&mcpConfig{cache: newBudgetTemplateCache()}).handleListSlideKinds(ctx, request)
}

// slideKindBudgetSource resolves the measured budgets a request asks for: the
// named template's, or the tightest across the shipped templates when budgets
// are requested without one. wanted is false when the request asks for none.
func (mc *mcpConfig) slideKindBudgetSource(request mcp.CallToolRequest, budgetsField bool) (measured templateTextBudgets, basis *slideKindBudgetBasis, wanted bool, errRes *mcp.CallToolResult) {
	name := strings.TrimSpace(request.GetString("template", ""))
	if name == "" {
		if !budgetsField {
			return measured, nil, false, nil
		}
		tightest, names, err := tightestShippedBudgets()
		if err != nil {
			return measured, nil, false, api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to measure the shipped templates: %v", err))
		}
		return tightest, &slideKindBudgetBasis{Templates: names,
			Note: "title, subtitle and takeaway budgets are the tightest across the shipped templates: copy within them fits every one. Pass template for that template's own."}, true, nil
	}
	path, cleanup, err := resolveTemplatePath(name, mc.templatesDir)
	if err != nil {
		return measured, nil, false, mcpErrorWithNext("TEMPLATE_NOT_FOUND", templateNotFoundError(name, mc.templatesDir), nextCallListTemplates())
	}
	defer cleanup()
	analysis, err := getOrAnalyzeTemplate(path, mc.cache)
	if err != nil {
		return measured, nil, false, mcpErrorWithNext("TEMPLATE_ERROR", fmt.Sprintf("failed to analyze template %q: %v", name, err), nextCallListTemplates())
	}
	return measureTemplateBudgets(analysis), &slideKindBudgetBasis{Template: strings.TrimSuffix(name, ".pptx"),
		Note: "title, subtitle and takeaway budgets are measured on this template; fixed budgets hold on every template."}, true, nil
}

func (mc *mcpConfig) handleListSlideKinds(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	kindFilter, errRes := slideKindListSelection(request, "kinds", false)
	if errRes != nil {
		return errRes, nil
	}
	fieldFilter, errRes := slideKindListSelection(request, "fields", true)
	if errRes != nil {
		return errRes, nil
	}
	measured, budgetBasis, withBudgets, errRes := mc.slideKindBudgetSource(request, fieldFilter["budgets"])
	if errRes != nil {
		return errRes, nil
	}
	wantPreview, errRes := previewArg("list_slide_kinds", request)
	if errRes != nil {
		return errRes, nil
	}
	if wantPreview && (len(kindFilter) == 0 || len(kindFilter) > maxPreviewImages) {
		return argInvalidValue("list_slide_kinds", "INVALID_PARAMETER", "kinds",
			fmt.Sprintf("preview renders one image per kind: name 1 to %d kinds", maxPreviewImages), "array", []string{"kpi_snapshot"},
			&patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{"kinds": []string{"kpi_snapshot"}, "preview": true}}), nil
	}
	out := make([]slideKindListEntry, 0)
	for _, k := range semantic.AllSlideKinds() {
		if kindFilter != nil && !kindFilter[string(k)] {
			continue
		}
		info, _ := semantic.LookupKind(k)
		entry := slideKindListEntry{
			Kind:            string(k),
			Summary:         info.Summary,
			RequiredFields:  info.RequiredFields,
			RequiredAliases: info.RequiredAliases,
			TypicalFields:   info.TypicalFields,
			Example:         semantic.KindExample(k),
		}
		if fieldFilter["item_schema"] {
			entry.ItemSchema = semantic.KindItemSchema(k)
		}
		if fieldFilter["compositions"] {
			entry.Compositions = slideKindCompositions(k)
		}
		if k == semantic.KindRawJSON2pptx && kindFilter[string(k)] {
			entry.ComposedExample = composedProofSlideExample()
		}
		if withBudgets {
			entry.Budgets = slideKindBudgets(k, measured)
		}
		out = append(out, entry)
	}
	takeaway := map[string]any{
		"font_pt": 14, "max_lines": 1,
		"note": "Keep takeaway (or chart insight) to one 14pt line in the template's chrome band. Width varies by template; validate_deck_spec reports BODY_TOO_LONG at slides[i].takeaway when measured text wraps.",
	}
	result := map[string]any{"slide_kinds": out, "takeaway_budget": takeaway}
	if withBudgets {
		if measured.Takeaway.MaxChars > 0 {
			takeaway["max_chars"] = measured.Takeaway.MaxChars
		}
		result["budget_basis"] = budgetBasis
	}
	// Each selected kind's example, rendered as render_deck_spec renders it
	// (go-slide-creator-ueopl).
	var previewImages []api.MCPImage
	if wantPreview {
		names := make([]string, len(out))
		for i := range out {
			names[i] = out[i].Kind
		}
		result["previews"], previewImages = mc.renderPreviews(ctx, kindPreviewRequests(names, strings.TrimSpace(request.GetString("template", ""))))
	}
	mcpResult, err := previewableResult(ctx, result, previewImages)
	if err != nil {
		return api.MCPSimpleError("INTERNAL", fmt.Sprintf("failed to marshal list_slide_kinds response: %v", err)), nil
	}
	return mcpResult, nil
}

func slideKindListSelection(request mcp.CallToolRequest, arg string, detail bool) (map[string]bool, *mcp.CallToolResult) {
	retryCatalog := &patterns.ToolCallSuggestion{Tool: "list_slide_kinds", ArgsTemplate: map[string]any{}}
	raw, present := request.GetArguments()[arg]
	if !present {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, argInvalidValue("list_slide_kinds", "INVALID_PARAMETER", arg, arg+" must be an array of strings", "array", nil, retryCatalog)
	}
	allowed := make(map[string]bool)
	if detail {
		allowed["item_schema"] = true
		allowed["compositions"] = true
		allowed["budgets"] = true
	} else {
		for _, k := range semantic.AllSlideKinds() {
			allowed[string(k)] = true
		}
	}
	selected := make(map[string]bool, len(items))
	for i, item := range items {
		name, ok := item.(string)
		if !ok || !allowed[name] {
			return nil, argInvalidValue("list_slide_kinds", "INVALID_PARAMETER", fmt.Sprintf("%s[%d]", arg, i),
				fmt.Sprintf("unknown %s value %v", arg, item), "string", nil, retryCatalog)
		}
		selected[name] = true
	}
	return selected, nil
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

// templatePrecedenceWarning explains a template_path argument the spec's
// meta.template overrides; empty when nothing was overridden. (A template
// argument overrides meta.template: see resolveSpecTemplate.)
func templatePrecedenceWarning(metaTemplate, argTemplate, argTemplatePath string) string {
	if metaTemplate == "" || argTemplatePath == "" {
		return ""
	}
	return fmt.Sprintf("template_path %q was ignored: the spec pins meta.template %q, which wins over a template file (template > meta.template > template_path > archetype default); remove meta.template to render with the file", argTemplatePath, metaTemplate)
}

// completeRenderDeckSpecResponse adds the template-precedence warning and the
// next step of the render loop (go-slide-creator-6p9mm, go-slide-creator-z3pbp).
func completeRenderDeckSpecResponse(res *renderDeckSpecResponse, templateWarnings []string) {
	if len(templateWarnings) > 0 {
		res.Warnings = append(append([]string(nil), templateWarnings...), res.Warnings...)
	}
	if !res.OK || res.PptxPath == "" {
		// A refused render still has a next step: the blocking diagnostic's
		// patch (go-slide-creator-b7qqg.4).
		if !res.OK {
			res.NextToolCall = firstBlockingSemanticCall(res.Diagnostics)
		}
		return
	}
	ready := res.DeterministicReady != nil && *res.DeterministicReady
	res.NextToolCall = renderNextToolCall(ready, firstBlockingSemanticCall(res.Diagnostics), res.PptxPath, res.ChangedSlides)
}

// renderDeckSpecFinish is what finishRenderDeckSpec needs besides the result.
type renderDeckSpecFinish struct {
	Src      specSource
	Template deckTemplateSource
	// RenderIdentity is the template this call rendered on when that is not
	// the template the deck is bound to (a one-off template argument).
	RenderIdentity string
	// TemplateWarnings explain the template choice (go-slide-creator-2dit4).
	TemplateWarnings []string
	Verbose          bool
	// Trial validates a variant of the spec as validate_deck_spec would; nil
	// when the render used a template file a trial would not resolve.
	Trial func([]byte) ([]trialFinding, bool)
}

// finishRenderDeckSpec commits the rendered spec to the deck store and
// completes the response: handle, revision, change classes, semantic patch
// suggestions and the next step.
func (mc *mcpConfig) finishRenderDeckSpec(ctx context.Context, res renderDeckSpecResponse, f renderDeckSpecFinish) (*mcp.CallToolResult, error) {
	src := f.Src
	// A patch is transactional with its render: a refused render leaves the
	// stored deck as it was, and a dry run stores nothing either way
	// (go-slide-creator-j77xe). A spec sent in the call still gets a handle
	// when its render is refused, so it can be patched into shape.
	renderedPptx := ""
	if res.OK {
		renderedPptx = res.PptxPath
	}
	outcome := mc.commitDeck(deckCommit{
		Tool: "render_deck_spec", Src: src, Spec: src.Data, Filename: src.Filename, Template: f.Template,
		Store:        !src.DryRun && (res.OK || !src.mutated()),
		RenderedPptx: renderedPptx, AgainstRender: true,
		RenderIdentity: f.RenderIdentity,
	})
	if outcome.Stale {
		return staleDeckSpecResult("render_deck_spec", src.DeckID), nil
	}
	res.DeckID, res.Stored, res.Revision = outcome.DeckID, outcome.Stored, outcome.Revision
	res.ChangedSlides, res.SlideChanges = outcome.Changed, outcome.Changes
	res.Slides = outcome.State.refs()
	res.Diagnostics = collapseDiagnostics(res.Diagnostics)
	// The image tools resolve a slide id against this exact file.
	recordRenderedSlides(renderedPptx, res.Slides)
	remedies := newRemedyContext(src.Filename, src.Data, res.DeckID)
	remedies.run, remedies.before, remedies.gateUnknown = f.Trial, trialFindings(res.Diagnostics), !res.OK
	remedies.remedyDiagnostics(res.Diagnostics)
	for i := range res.Diagnostics {
		d := &res.Diagnostics[i]
		if !outcome.Stored {
			prefixUnstoredPatch(d.NextToolCall, src)
		}
		d.SlideID = outcome.State.slideIDForPath(d.SemanticPath)
	}
	shapeRenderDiagnostics(res.Diagnostics, newSpecDoc(src.Filename, src.Data))
	completeRenderDeckSpecResponse(&res, f.TemplateWarnings)
	// A patch on a deck the agent has already seen rendered gets the compact
	// response; a first render keeps the full one.
	renderedBefore := src.Handle != nil && src.Handle.Rendered != nil
	narrowThumbnailCall(&res, renderedBefore)
	if renderedBefore && (len(src.RawPatch) > 0 || src.Restore > 0) && !f.Verbose {
		compactRenderResponse(&res)
	}
	return semanticSuccessOrInternal(ctx, "render_deck_spec", res)
}

// narrowThumbnailCall fits the thumbnails step to what the render changed: the
// whole deck when every slide is new to the eye (a first render, a restyle),
// only the slides that look different otherwise, and no call at all when a
// re-render changed nothing visible — a notes edit, a reorder
// (go-slide-creator-v5e9h).
func narrowThumbnailCall(res *renderDeckSpecResponse, renderedBefore bool) {
	call := res.NextToolCall
	if call == nil || call.Tool != "render_deck_thumbnails" {
		return
	}
	switch {
	case len(res.ChangedSlides) == 0 && renderedBefore:
		res.NextToolCall = nil
	case len(res.ChangedSlides) == 0 || len(res.ChangedSlides) >= res.SlideCount:
		delete(call.ArgsTemplate, "slide_indices")
	}
}

// compactRenderResponse trims a patch render's response to what the patch
// changed (go-slide-creator-veqn2). Measured on an eight-slide deck, 88% of a
// 7-9 KB revision response repeated the slide list, per-slide scores and
// composition alternatives of slides that had not changed. The compact form
// keeps the verdict, the change list, every blocking diagnostic, and the
// scores, explanation rows and findings of the slides that look different;
// verbose:true returns the full response.
func compactRenderResponse(res *renderDeckSpecResponse) {
	visual := make(map[int]bool, len(res.ChangedSlides))
	for _, i := range res.ChangedSlides {
		visual[i] = true
	}
	res.Slides = nil
	if res.Explanation != nil {
		var rows []semantic.SlideExplanation
		for _, row := range res.Explanation.Slides {
			if visual[row.Index] {
				row.Alternatives = nil
				rows = append(rows, row)
			}
		}
		if len(rows) == 0 {
			res.Explanation = nil
		} else {
			compact := *res.Explanation
			compact.Slides = rows
			res.Explanation = &compact
		}
	}
	if res.Quality != nil {
		compact := *res.Quality
		compact.SlideScores = nil
		for _, score := range res.Quality.SlideScores {
			if visual[score.SlideNumber-1] {
				compact.SlideScores = append(compact.SlideScores, score)
			}
		}
		res.Quality = &compact
	}
	kept := res.Diagnostics[:0:0]
	for _, d := range res.Diagnostics {
		blocking := d.Blocking || d.Severity == "error" || d.Action == "refuse"
		onChanged := d.SlideIndex == nil || visual[*d.SlideIndex]
		for _, m := range d.members {
			// A folded entry is kept when any slide it stands for changed.
			onChanged = onChanged || m.SlideIndex == nil || visual[*m.SlideIndex]
		}
		if blocking || onChanged {
			kept = append(kept, d)
		}
	}
	res.DiagnosticsOmitted = len(res.Diagnostics) - len(kept)
	res.Diagnostics = kept
	res.compact = true
}

// MarshalJSON writes a compact patch render with the summaries reduced to the
// fields a revision reads: the scores and gate verdict without the fixed
// criteria and evidence blocks, and the changed slides' plan rows without the
// deck-wide rhythm tables. Both decode into the full types.
func (r renderDeckSpecResponse) MarshalJSON() ([]byte, error) {
	type plain renderDeckSpecResponse
	if !r.compact {
		return json.Marshal(plain(r))
	}
	type compactGate struct {
		Passed  bool     `json:"passed"`
		Reasons []string `json:"reasons"`
	}
	type compactQuality struct {
		Score           float64        `json:"score"`
		Basis           string         `json:"basis"`
		SlideScores     []SlideQuality `json:"slide_scores,omitempty"`
		Issues          []string       `json:"issues,omitempty"`
		Scope           string         `json:"scope"`
		StructuralScore int            `json:"structural_score,omitempty"`
		QualityGate     *compactGate   `json:"quality_gate,omitempty"`
	}
	type compactExplanation struct {
		Slides []semantic.SlideExplanation `json:"slides"`
	}
	out := struct {
		plain
		Quality     *compactQuality     `json:"quality_summary,omitempty"`
		Explanation *compactExplanation `json:"explanation_summary,omitempty"`
	}{plain: plain(r)}
	if q := r.Quality; q != nil {
		out.Quality = &compactQuality{
			Score: q.Score, Basis: q.Basis, SlideScores: q.SlideScores, Issues: q.Issues,
			Scope: q.Scope, StructuralScore: q.StructuralScore,
		}
		if q.QualityGate != nil {
			out.Quality.QualityGate = &compactGate{Passed: q.QualityGate.Passed, Reasons: q.QualityGate.Reasons}
		}
	}
	if r.Explanation != nil {
		out.Explanation = &compactExplanation{Slides: r.Explanation.Slides}
	}
	return json.Marshal(out)
}
