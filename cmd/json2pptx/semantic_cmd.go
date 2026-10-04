package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pipeline"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/semantic"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/internal/visualqa/deterministic"
)

// runSemantic implements the "semantic" command group: a thin CLI surface over
// internal/semantic that lets an author validate, compile, and inspect the
// schema of a compact semantic deck spec without touching the raw
// PresentationInput model. It dispatches the sub-subcommand (validate, compile,
// schema) the same way main.dispatch dispatches top-level commands: the leading
// argument selects the action and the remaining args are reshaped so each
// handler parses its own flags.
func runSemantic() error {
	if len(os.Args) < 2 {
		printSemanticUsage()
		return cliMissingArg("semantic requires a subcommand: kinds, validate, compile, render, explain, or schema")
	}

	sub := os.Args[1]
	// Shift args so each sub-subcommand sees its own flags (mirrors dispatch).
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	err := dispatchSemanticSub(sub)
	// A subcommand's `-h`/`--help` makes flag.Parse return flag.ErrHelp after the
	// flag package has already printed usage to stderr. Help is a successful,
	// intentional invocation, so translate it to a clean exit (code 0) rather than
	// letting it bubble up to main as an error — automated probes treat a non-zero
	// `--help` as a failure.
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

// dispatchSemanticSub routes a semantic sub-subcommand to its handler. It is split
// out of runSemantic so the caller can uniformly translate flag.ErrHelp (returned
// by any handler's flag parsing on -h/--help) into a clean exit.
func dispatchSemanticSub(sub string) error {
	switch sub {
	case "kinds":
		return runSemanticKinds()
	case "validate":
		return runSemanticValidate()
	case "compile":
		return runSemanticCompile()
	case "render":
		return runSemanticRender()
	case "explain":
		return runSemanticExplain()
	case "schema":
		return runSemanticSchema()
	case "help", "-h", "--help":
		printSemanticUsage()
		return nil
	default:
		return cliInvalidArg("unknown semantic subcommand %q — run 'json2pptx semantic help' for usage", sub)
	}
}

// printSemanticUsage prints the semantic command group help.
func printSemanticUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx semantic <subcommand> [options]

Compile compact semantic deck specs (DeckSpec) into the raw json2pptx
PresentationInput model.

Subcommands:
  kinds      List slide kinds (one line each); 'kinds <kind>' prints fields,
             budgets and a copy-ready example
  validate   Validate a semantic spec; emit the shared finding envelope
  compile    Compile a semantic spec to raw PresentationInput JSON
  render     Compile a semantic spec and render it straight to a .pptx
  explain    Print the compiler's planned decisions and rhythm warnings
  schema     Print the DeckSpec JSON Schema (draft 2020-12; large, ~170 KB)

The spec file is the first argument (--spec <file> still works); --out names
the output (--output still works).

Examples:
  json2pptx semantic kinds
  json2pptx semantic kinds kpi_snapshot
  json2pptx semantic validate deck.yaml
  json2pptx semantic render deck.yaml --out deck.pptx
  json2pptx semantic validate --spec deck.yaml
  json2pptx semantic validate --spec deck.yaml --strict strict
  json2pptx semantic compile --spec deck.yaml --output compiled.json
  json2pptx semantic compile --spec deck.yaml --output -      # stdout
  json2pptx semantic compile --spec deck.yaml --envelope      # JSON + diagnostics
  json2pptx semantic compile --spec - --envelope < deck.yaml  # read stdin
  json2pptx semantic render --spec deck.yaml --output deck.pptx
  json2pptx semantic explain --spec deck.yaml
  json2pptx semantic schema

Run 'json2pptx semantic <subcommand> -h' for subcommand-specific help.
`)
}

// parseStrictness maps a --strict flag value to a semantic.Strictness, rejecting
// unrecognized values so a typo fails fast rather than silently defaulting.
func parseStrictness(v string) (semantic.Strictness, error) {
	switch semantic.Strictness(v) {
	case semantic.StrictnessOff, semantic.StrictnessWarn, semantic.StrictnessStrict:
		return semantic.Strictness(v), nil
	default:
		return "", cliInvalidArg("invalid --strict value %q: must be off, warn, or strict", v)
	}
}

// parseOutputValidation validates a post-generation output-validation mode,
// rejecting typos so the CLI cannot silently fall back to a non-strict run. The
// render runner treats only the exact "strict" value as blocking, so an
// unrecognized value (e.g. a "stric" typo) would quietly skip strict gating
// while still exiting 0 — leaving the caller to believe strict validation ran.
// Failing fast here keeps `semantic render` in parity with MCP render_deck_spec,
// which already rejects the same typos. The validated value is returned
// unchanged for passing straight to RunPresentation.
func parseOutputValidation(v string) (string, error) {
	switch v {
	case "off", "warn", "strict":
		return v, nil
	default:
		return "", cliInvalidArg("invalid --output-validation value %q: must be off, warn, or strict", v)
	}
}

// semanticValidateEnvelope is the result of "semantic validate": the shared
// finding envelope plus what the run was measured on and what it waived.
type semanticValidateEnvelope struct {
	diagnostics.FindingEnvelope
	// TemplateSource says what chose the envelope's template: "meta.template",
	// "template argument" or "archetype default".
	TemplateSource string `json:"template_source,omitempty"`
	// Warnings are command-level notes that are not findings on the deck.
	Warnings []string `json:"warnings,omitempty"`
	// Waivers records the storyline findings the deck waived.
	Waivers []findingWaiver `json:"waivers,omitempty"`
}

// runSemanticValidate implements "semantic validate". It parses and validates a
// semantic spec, then renders it into a scratch directory exactly as "semantic
// render" would and reports that run's findings as the shared FindingEnvelope
// (go-slide-creator-3rn3s, go-slide-creator-2dit4). The envelope names the
// template the findings were measured on. The process exits non-zero when any
// finding has error severity, which is exactly when "semantic render" would.
func runSemanticValidate() error {
	fs := flag.NewFlagSet("semantic validate", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	strict := fs.String("strict", "warn", "Advisory-rule strictness: off, warn, or strict")
	templateName := fs.String("template", "", "Template to measure on; replaces the spec's meta.template for this run")
	templatesDir := fs.String("templates-dir", "", "Template search directory")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic validate <spec> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Validate a semantic deck spec and print the shared finding envelope. The spec is\n")
		fmt.Fprintf(os.Stderr, "rendered into a scratch directory, so the findings are the ones 'semantic render'\n")
		fmt.Fprintf(os.Stderr, "reports for the same spec and template; the envelope's template field says which\n")
		fmt.Fprintf(os.Stderr, "template that was.\n\n")
		fmt.Fprintf(os.Stderr, "Exit status: 0 when no finding has severity error; 1 otherwise. A finding blocks\n")
		fmt.Fprintf(os.Stderr, "exactly when its severity is error (blocking: true).\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return cliMissingArg("--spec is required")
	}
	strictness, err := parseStrictness(*strict)
	if err != nil {
		return err
	}

	data, err := readSpec(*specPath)
	if err != nil {
		return fmt.Errorf("semantic validate: read %s: %w", *specPath, err)
	}

	out := semanticValidateEnvelope{}
	var evalErr error
	// A spec with blocking spec-level errors is still evaluated with those
	// errors set aside, so one run reports everything knowable
	// (go-slide-creator-ipahe).
	eval, ds := evaluateSpecFindings(*specPath, data, strictness, func(spec *semantic.DeckSpec, _ []byte) specEvaluation {
		e, err := evaluateSpecCLI(*specPath, spec, strictness, *templateName, *templatesDir)
		if err != nil {
			evalErr = err
		}
		return e
	})
	if evalErr != nil {
		return evalErr
	}
	out.TemplateSource, out.Warnings, out.Waivers = eval.TemplateSource, eval.Warnings, eval.Waivers
	out.FindingEnvelope = diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "semantic validate",
		InputSHA256: diagnostics.ComputeInputSHA256(data),
		Template:    eval.Template,
	}, ds)
	stampEnvelopeFindings(&out.FindingEnvelope, eval.Diagnostics)
	// The CLI has no stored deck to patch: a finding carries its remedy's
	// facts, in the fields of the spec.
	cliRemedyContext(*specPath, data).remedyEnvelope(&out.FindingEnvelope, eval.Diagnostics)
	shapeEnvelopeFindings(&out.FindingEnvelope, eval.Diagnostics, newSpecDoc(*specPath, data))

	if err := printJSONIndent(out); err != nil {
		return err
	}
	if !out.OK {
		return fmt.Errorf("semantic validation failed")
	}
	return nil
}

// evaluateSpecCLI compiles a parsed spec and runs it as "semantic render"
// would, into a scratch directory.
func evaluateSpecCLI(specPath string, spec *semantic.DeckSpec, strictness semantic.Strictness, argTemplate, templatesDir string) (specEvaluation, error) {
	choice := resolveSpecTemplate(spec.Meta.Template, argTemplate, "", specSource{})
	spec = choice.evaluated(spec)
	eval := specEvaluation{Choice: choice, TemplateSource: choice.Source, Warnings: choice.Warnings}
	eval.Template = explainSpecWithTemplate(spec, argTemplate).Template

	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{Strict: strictness, DefaultTemplate: argTemplate})
	if err != nil || input == nil {
		eval.CompileFailed = true
		// A spec that does not compile reports what render reports for it.
		if err != nil && compileResult != nil {
			eval.Evaluated = true
			eval.Diagnostics = buildSemanticRenderFailure(compileResult, err).Diagnostics
		}
		return eval, nil
	}
	eval.Evaluated = true
	eval.Template = input.Template

	// Design-mode violations live in the COMPILED deck (the raw_json2pptx
	// escape hatch carries an author's payload through), so validate compiles
	// to see them — otherwise it passes a spec render then refuses
	// (go-slide-creator-rs4h).
	if designViolations := compiledDesignModeDiagnostics(input); len(designViolations) > 0 {
		appendCompiledDesignModeDiags(compileResult, input)
		eval.Diagnostics = buildSemanticRenderFailure(compileResult, blockingDesignModeError(designViolations)).Diagnostics
		return eval, nil
	}
	if input.Template == "" {
		eval.TemplateSource = ""
		eval.Warnings = append(eval.Warnings, unpinnedTemplateWarning("", ""))
		eval.Diagnostics, eval.Waivers = templateFreeDiagnostics(input, compileResult)
		return eval, nil
	}

	dir, removeDir, dirErr := trialRenderDir()
	if dirErr != nil {
		return eval, fmt.Errorf("semantic validate: create scratch directory: %w", dirErr)
	}
	defer removeDir()
	input.OutputFilename = "validate.pptx"
	res, _, runErr := runCompiledSpecCLI(specPath, input, compileResult, templatesDir, dir, "strict", time.Now())
	if runErr != nil {
		return eval, runErr
	}
	eval.Diagnostics = res.Diagnostics
	eval.Waivers = res.Waivers
	if spec.Meta.Template == "" {
		eval.Warnings = append(eval.Warnings, unpinnedTemplateWarning(eval.Template, choice.Source))
	}
	return eval, nil
}

// semanticCompileEnvelope is the structured result emitted by "semantic compile
// --envelope". It mirrors the HTTP POST /api/v1/semantic/compile response shape
// so the surfaces stay aligned: on success ok is true, slide_count/template
// summarize the compiled deck, and compiled_json carries the full raw
// PresentationInput; on a blocking parse/compile failure ok is false and error
// names the blocking reason. findings always carries the shared envelope of
// compile diagnostics, so an agent driving a compile-only flow sees non-blocking
// warnings (density, rhythm, raw-pattern preflight) without a separate validate
// run.
type semanticCompileEnvelope struct {
	OK           bool                        `json:"ok"`
	SlideCount   int                         `json:"slide_count,omitempty"`
	Template     string                      `json:"template,omitempty"`
	Warnings     []string                    `json:"warnings,omitempty"`
	Findings     diagnostics.FindingEnvelope `json:"findings"`
	CompiledJSON json.RawMessage             `json:"compiled_json,omitempty"`
	Error        string                      `json:"error,omitempty"`
}

// runSemanticCompile implements "semantic compile". It parses, validates, and
// compiles a semantic spec into a raw PresentationInput and writes the indented
// JSON to --output (a path, or - for stdout). The raw JSON is consumable by
// `json2pptx validate` and `json2pptx generate` for debugging or advanced edits.
// Blocking (error-severity) findings abort the compile: the finding envelope is
// printed to stderr and the process exits non-zero.
//
// With --envelope, the command instead emits the structured semanticCompileEnvelope
// (compiled_json plus the shared finding envelope) to --output/stdout, so a
// compile-only flow can read non-blocking diagnostics without a separate validate
// pass. A blocking failure under --envelope still writes the (ok=false) envelope
// and exits non-zero.
func runSemanticCompile() error {
	fs := flag.NewFlagSet("semantic compile", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	output := fs.String("output", "-", "Where to write the output; use - for stdout")
	strict := fs.String("strict", "warn", "Advisory-rule strictness: off, warn, or strict")
	templateName := fs.String("template", "", "Template to compile for; replaces the spec's meta.template for this run")
	envelopeMode := fs.Bool("envelope", false, "Emit a structured envelope (compiled_json + diagnostics) instead of raw JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic compile --spec <file> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Compile a semantic deck spec to raw PresentationInput JSON.\n")
		fmt.Fprintf(os.Stderr, "The output is accepted by 'json2pptx validate' and 'json2pptx generate'.\n")
		fmt.Fprintf(os.Stderr, "Pass --envelope to wrap the compiled JSON with non-blocking diagnostics.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return cliMissingArg("--spec is required")
	}
	strictness, err := parseStrictness(*strict)
	if err != nil {
		return err
	}

	data, err := readSpec(*specPath)
	if err != nil {
		return fmt.Errorf("semantic compile: read %s: %w", *specPath, err)
	}

	envOpts := diagnostics.EnvelopeOptions{
		Subcommand:  "semantic compile",
		InputSHA256: diagnostics.ComputeInputSHA256(data),
	}

	spec, parseDiags := semantic.Parse(*specPath, data)
	if parseDiags.HasErrors() {
		envelope := diagnostics.BuildEnvelope(envOpts, parseDiags.ToDiagnostics())
		if *envelopeMode {
			res := semanticCompileEnvelope{OK: false, Findings: envelope, Error: "semantic compile: spec could not be parsed"}
			_ = writeCompileOutput(*output, res)
			return fmt.Errorf("semantic compile: spec could not be parsed")
		}
		_ = fprintJSONIndent(os.Stdout, envelope)
		return fmt.Errorf("semantic compile: spec could not be parsed")
	}

	// --template replaces meta.template for this run, as it does on `semantic
	// validate` and `semantic render` (go-slide-creator-fjuhm). The envelope
	// carries the notice; the raw deck on stdout stays the deck alone.
	choice := resolveSpecTemplate(spec.Meta.Template, *templateName, "", specSource{})
	spec = choice.evaluated(spec)
	if !*envelopeMode {
		for _, w := range choice.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		}
	}

	input, result, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: *templateName,
	})
	if err != nil {
		var ds []diagnostics.Diagnostic
		if result != nil {
			ds = result.Diagnostics
		}
		envelope := diagnostics.BuildEnvelope(envOpts, ds)
		if *envelopeMode {
			res := semanticCompileEnvelope{OK: false, Warnings: choice.Warnings, Findings: envelope, Error: fmt.Sprintf("semantic compile: %v", err)}
			_ = writeCompileOutput(*output, res)
			return fmt.Errorf("semantic compile: %w", err)
		}
		_ = fprintJSONIndent(os.Stdout, envelope)
		return fmt.Errorf("semantic compile: %w", err)
	}

	// The compiled deck answers for its own design mode, so `semantic compile`
	// reports what `semantic render` would refuse (go-slide-creator-rs4h).
	appendCompiledDesignModeDiags(result, input)

	if *envelopeMode {
		var ds []diagnostics.Diagnostic
		if result != nil {
			ds = result.Diagnostics
		}
		compiled, marshalErr := json.Marshal(input)
		if marshalErr != nil {
			return fmt.Errorf("semantic compile: marshal compiled deck: %w", marshalErr)
		}
		res := semanticCompileEnvelope{
			OK:           true,
			SlideCount:   len(input.Slides),
			Template:     input.Template,
			Warnings:     choice.Warnings,
			Findings:     diagnostics.BuildEnvelope(envOpts, ds),
			CompiledJSON: compiled,
		}
		return writeCompileOutput(*output, res)
	}

	raw, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return fmt.Errorf("semantic compile: marshal compiled deck: %w", err)
	}
	raw = append(raw, '\n')

	if *output == "" || *output == "-" {
		_, err = os.Stdout.Write(raw)
		return err
	}
	if err := os.WriteFile(*output, raw, 0o644); err != nil { //nolint:gosec // generated deck JSON is not sensitive
		return fmt.Errorf("semantic compile: write %s: %w", *output, err)
	}
	fmt.Fprintf(os.Stderr, "Wrote %d slide(s) to %s\n", len(input.Slides), *output)
	return nil
}

// semanticRenderResult is the compact, machine-readable result of "semantic
// render". On success it carries the artifact path, slide count, content hash, a
// quality summary, and any advisory diagnostics. On failure OK is false, Error
// names the blocking reason, and Diagnostics carry the findings — each pointing
// at the semantic source path the author wrote (raw paths only as a fallback
// when no mapping exists).
type semanticRenderResult struct {
	OK         bool   `json:"ok"`
	OutputPath string `json:"output_path,omitempty"`
	Overwrote  bool   `json:"overwrote,omitempty"`
	// A written deck can clear deterministic checks while still needing a
	// current all-slide visual review before it is publishable.
	DeterministicReady           *bool    `json:"deterministic_ready,omitempty"`
	Publishable                  *bool    `json:"publishable,omitempty"`
	ManualReviewRequired         *bool    `json:"manual_review_required,omitempty"`
	BlockingReasons              []string `json:"blocking_reasons,omitempty"`
	DeterministicBlockingReasons []string `json:"deterministic_blocking_reasons,omitempty"`
	Template                     string   `json:"template,omitempty"`
	SlideCount                   int      `json:"slide_count,omitempty"`
	ContentHash                  string   `json:"content_hash,omitempty"`
	Revision                     string   `json:"revision,omitempty"`
	ManifestPath                 string   `json:"manifest_path,omitempty"`
	// Slides is the deck's table of contents, written with the sidecar that
	// records it: each slide's stable id (the author's, or the s<N> assigned
	// to a slide without one — the ids MCP render_deck_spec assigns), index
	// and slide_number. render-slide --slide-id and render-thumbnails --slides
	// take the id (go-slide-creator-cmwmg).
	Slides      []slideRef           `json:"slides,omitempty"`
	DurationMs  int64                `json:"duration_ms,omitempty"`
	Quality     *QualityScore        `json:"quality,omitempty"`
	Warnings    []string             `json:"warnings,omitempty"`
	Diagnostics []semanticDiagnostic `json:"diagnostics,omitempty"`
	// Waivers records the storyline findings this deck waived, by meta.waivers
	// or by its archetype, and how many findings each one turned into an
	// advisory (go-slide-creator-oh3qr).
	Waivers []findingWaiver `json:"waivers,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// semanticDiagnostic is one compact finding in a render result. SemanticPath
// points at the field in the semantic DeckSpec the author wrote; RawPath is the
// originating generated pointer, retained as fallback evidence (and the only
// locator when a finding could not be traced back to a semantic source path).
// SlideIndex is the semantic slide the finding belongs to, or -1. RecommendedEdit
// names a semantic edit that should resolve the finding, when one is known.
type semanticDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity,omitempty"`
	// Blocking is the one flag that says whether this finding stops
	// deterministic_ready. It is true exactly when Severity is "error": a
	// blocking finding is never reported as a warning or an info, and an
	// advisory never blocks (go-slide-creator-x9rhq).
	Blocking     bool   `json:"blocking"`
	Message      string `json:"message"`
	SemanticPath string `json:"semantic_path,omitempty"`
	RawPath      string `json:"raw_path,omitempty"`
	SlideIndex   *int   `json:"slide_index,omitempty"`
	// SlideID is the stable id of the slide at SlideIndex in a stored deck: a
	// patch may address the slide by it (go-slide-creator-1w3uo).
	SlideID         string                       `json:"slide_id,omitempty"`
	Action          string                       `json:"action,omitempty"`
	RecommendedEdit *semantic.SemanticEdit       `json:"recommended_edit,omitempty"`
	NextToolCall    *patterns.ToolCallSuggestion `json:"next_tool_call,omitempty"`
	// Evidence carries a generation refusal's measurement: measured.font_pt
	// against allowed.min_font_pt, the text role, viewing mode and the refused
	// paragraph's text (go-slide-creator-b7qqg.4).
	Evidence map[string]any `json:"evidence,omitempty"`
	// Waived is the reason a storyline finding was turned into an advisory
	// (meta.waivers, or the deck's archetype).
	Waived string `json:"waived,omitempty"`
	// Symptoms are the per-field findings a root-cause finding accounts for:
	// one slide whose pattern needs more height than it has shrinks every cell,
	// and each shrunk field is listed here instead of as its own blocker
	// (go-slide-creator-3rn3s).
	Symptoms []findingSymptom `json:"symptoms,omitempty"`

	// diag is the transport-neutral diagnostic this finding was built from,
	// with its path already semantic. validate_deck_spec renders its envelope
	// from it, so both tools report one finding set from one collection.
	diag *diagnostics.Diagnostic
	// fallbackPatch is the DeckSpec patch to suggest when the semantic path
	// names no single rewritable field (a refused list switches composition).
	fallbackPatch []any
	// members are the findings this one stands for when several with the same
	// code, slide and cause were folded into it (go-slide-creator-c2j5b); the
	// first member is the finding itself. memberPaths are their authored
	// pointers once resolved.
	members     []diagMember
	memberPaths []string
	// baseMessage is the first member's message, before the count was added.
	baseMessage string
	// address is the finding's location in the authored spec, resolved by
	// shapeRenderDiagnostics (go-slide-creator-pilpn).
	address *authoredAddress
	// debug holds the compiled-deck locators moved out of Evidence and the
	// recommended edit's params.
	debug map[string]any
	// patchVerified reports that NextToolCall's patch was applied to the spec
	// and validated: the finding is gone and nothing new blocks
	// (go-slide-creator-vihnl).
	patchVerified bool
}

// runSemanticRender implements "semantic render": the target one-command flow
// from a compact semantic spec to a rendered .pptx. It parses and validates the
// spec, compiles it to a raw PresentationInput, runs the shared in-memory render
// runner (RunPresentation, which keeps strict output validation as the default),
// maps raw render findings back to the semantic source paths the author wrote
// (falling back to the raw path only when no mapping exists), and prints a
// compact result with a quality summary. Blocking failures print the same
// compact result (OK=false) to stdout and exit non-zero.
func runSemanticRender() error { //nolint:gocognit // Orchestrates validation, compilation, rendering, and atomic manifest persistence.
	fs := flag.NewFlagSet("semantic render", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	output := fs.String("output", "", "Output .pptx path (or directory); required")
	strict := fs.String("strict", "warn", "Advisory-rule strictness: off, warn, or strict")
	templateName := fs.String("template", "", "Template to render on; replaces the spec's meta.template for this run")
	templatesDir := fs.String("templates-dir", "", "Template search directory")
	outputValidation := fs.String("output-validation", "strict", "Post-generation output validation: off, warn, or strict")
	noManifest := fs.Bool("no-manifest", false, "Do not write the <deck>.pptx.authoring.json sidecar next to the deck")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic render <spec> --out <file.pptx> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Compile a semantic deck spec and render it straight to a .pptx using the\n")
		fmt.Fprintf(os.Stderr, "shared generation pipeline. Strict output validation is the default.\n\n")
		fmt.Fprintf(os.Stderr, "Files written: the deck, and beside it <deck>.pptx.authoring.json - the authoring\n")
		fmt.Fprintf(os.Stderr, "manifest (spec, compiled input, source map, revision) that lets a later patch or\n")
		fmt.Fprintf(os.Stderr, "visual review be tied to this exact deck. It is named in the result's manifest_path,\n")
		fmt.Fprintf(os.Stderr, "is written only when the deck was written, and --no-manifest turns it off.\n\n")
		fmt.Fprintf(os.Stderr, "Exit status: 0 when the deck was written and no blocking finding remains (\"ok\":\n")
		fmt.Fprintf(os.Stderr, "true); 1 otherwise. A finding blocks exactly when its severity is error; warnings\n")
		fmt.Fprintf(os.Stderr, "and infos never change the exit status. A deck with blocking findings is still\n")
		fmt.Fprintf(os.Stderr, "written, and the result names each one in deterministic_blocking_reasons. A\n")
		fmt.Fprintf(os.Stderr, "storyline finding the brief rules out can be waived in meta.waivers.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return cliMissingArg("--spec is required")
	}
	if *output == "" {
		fs.Usage()
		return cliMissingArg("--out is required")
	}
	// A render that fails must not leave a sidecar describing a deck that is
	// not there: an earlier render's manifest for the same path is removed
	// when this call ends without a deck at that path (go-slide-creator-12dkn).
	defer removeOrphanAuthoringManifest(*output)
	strictness, err := parseStrictness(*strict)
	if err != nil {
		return err
	}
	// Reject an --output-validation typo before any spec read or render so the
	// caller cannot believe strict output validation was applied when it was not.
	if _, err = parseOutputValidation(*outputValidation); err != nil {
		return err
	}

	data, err := readSpec(*specPath)
	if err != nil {
		return fmt.Errorf("semantic render: read %s: %w", *specPath, err)
	}

	startTime := time.Now()

	// Every printed result names its findings one way: folded by cause, each at
	// a JSON Pointer into this spec (go-slide-creator-c2j5b, -pilpn).
	doc := newSpecDoc(*specPath, data)
	finishDiagnostics := func(res *semanticRenderResult) {
		res.Diagnostics = collapseDiagnostics(res.Diagnostics)
		cliRemedyContext(*specPath, data).remedyDiagnostics(res.Diagnostics)
		shapeRenderDiagnostics(res.Diagnostics, doc)
	}
	evaluateReduced := func(reduced *semantic.DeckSpec, _ []byte) specEvaluation {
		e, _ := evaluateSpecCLI(*specPath, reduced, strictness, *templateName, *templatesDir)
		return e
	}

	// Parse the spec. A parse error is fatal and has no source map yet, so the
	// findings carry their native semantic paths.
	spec, parseDiags := semantic.Parse(*specPath, data)
	if parseDiags.HasErrors() {
		res := semanticRenderResult{OK: false, Error: "semantic render: spec could not be parsed"}
		// The findings `semantic validate` reports for the same spec: its
		// errors, and the findings of the slides they do not touch.
		eval, ds := evaluateSpecFindings(*specPath, data, strictness, evaluateReduced)
		if eval.Evaluated {
			res.Diagnostics, res.Warnings, res.Waivers = eval.Diagnostics, eval.Warnings, eval.Waivers
		} else {
			for _, d := range ds {
				res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
			}
		}
		finishDiagnostics(&res)
		_ = fprintJSONIndent(os.Stdout, res)
		return fmt.Errorf("%s", res.Error)
	}

	// --template replaces meta.template for this render, and the result says
	// so (go-slide-creator-ifkxs).
	choice := resolveSpecTemplate(spec.Meta.Template, *templateName, "", specSource{})
	spec = choice.evaluated(spec)

	// Validate + compile to a raw PresentationInput. Blocking findings abort the
	// render with the diagnostics surfaced on the result.
	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: *templateName,
	})
	if err != nil {
		res := buildSemanticRenderFailure(compileResult, err)
		if eval, ok := salvagedEvaluation(*specPath, data, strictness, evaluateReduced); ok {
			res.Diagnostics, res.Waivers = eval.Diagnostics, eval.Waivers
			res.Warnings = append(res.Warnings, eval.Warnings...)
		}
		finishDiagnostics(&res)
		_ = fprintJSONIndent(os.Stdout, res)
		return fmt.Errorf("semantic render: %w", err)
	}

	// Accept a .pptx file path or a directory for --output (mirrors `generate`):
	// a file destination splits into parent dir + filename.
	outputDir := *output
	if strings.HasSuffix(strings.ToLower(outputDir), ".pptx") {
		input.OutputFilename = filepath.Base(outputDir)
		outputDir = filepath.Dir(outputDir)
	}

	// Constrained mode is enforced here as it is on generate: the raw_json2pptx
	// escape hatch passes author-authored slide payloads straight through, so
	// the same shape_grid must get the same verdict whichever path compiled it
	// (go-slide-creator-rs4h). meta.design_mode: "free" opts out.
	if designViolations := compiledDesignModeDiagnostics(input); len(designViolations) > 0 {
		appendCompiledDesignModeDiags(compileResult, input)
		err := blockingDesignModeError(designViolations)
		res := buildSemanticRenderFailure(compileResult, err)
		finishDiagnostics(&res)
		_ = fprintJSONIndent(os.Stdout, res)
		return fmt.Errorf("semantic render: %w", err)
	}

	res, runRes, runErr := runCompiledSpecCLI(*specPath, input, compileResult, *templatesDir, outputDir, *outputValidation, startTime)
	if runErr != nil {
		return runErr
	}
	if choice.Override != "" {
		res.Warnings = append(append([]string{}, choice.Warnings...), res.Warnings...)
	}
	finishDiagnostics(&res)
	if !res.OK {
		_ = fprintJSONIndent(os.Stdout, res)
		return fmt.Errorf("semantic render: %s", res.Error)
	}
	if *noManifest {
		_ = os.Remove(runRes.OutputPath + authoringManifestSuffix)
		return emitSemanticRenderResult(res)
	}
	if err := writeAuthoringSidecar(&res, spec, data, *specPath, input, compileResult, runRes); err != nil {
		return err
	}
	return emitSemanticRenderResult(res)
}

// runCompiledSpecCLI renders a compiled DeckSpec for the CLI: deck defaults and
// named settings, the standard config, guarded asset resolution, the shared
// runner and the result builders. `semantic render` writes its deck with it,
// and `semantic validate` runs the same function into a scratch directory, so
// the two commands report one finding set (go-slide-creator-3rn3s). A returned
// error is a failure of the command itself (config, resolver); a deck the
// runner refuses comes back as a result with OK=false.
func runCompiledSpecCLI(specPath string, input *PresentationInput, compileResult *semantic.CompileResult, templatesDir, outputDir, outputValidation string, startTime time.Time) (semanticRenderResult, RenderResult, error) {
	// Apply the shared pre-render prep that a compiled deck still needs: deck
	// defaults (table/cell styles) and named style references. Structure
	// expansion does not apply to a compiled deck — but the raw_json2pptx escape
	// hatch passes author-authored slide payloads straight through, so the
	// URL/asset resolution PreConvert hook below still does (see preConvert).
	applyDefaults(input)
	resolveInputNamedSettingsForDir(templatesDir, input)

	// Load the standard config (defaults + environment overrides, as generate
	// does) so charts/diagrams render with the same native-SVG strategy and
	// ALLOWED_IMAGE_PATHS restricts image roots here too
	// (go-slide-creator-s1uvj.44).
	cfg, err := config.Load("")
	if err != nil {
		return semanticRenderResult{}, RenderResult{}, fmt.Errorf("semantic render: load config: %w", err)
	}
	if templatesDir != "" {
		cfg.Templates.Dir = templatesDir
	}

	// A template that does not resolve is a finding at meta.template with the
	// nearest name, not a bare error string.
	if _, templateCleanup, tplErr := resolveTemplatePath(input.Template, cfg.Templates.Dir); tplErr != nil {
		d := semanticTemplateDiagnostic(input.Template, cfg.Templates.Dir, templateResolutionCode(tplErr), tplErr)
		res := buildSemanticRenderFailure(compileResult, errors.New(d.Message))
		res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(*d))
		return res, RenderResult{}, nil
	} else {
		templateCleanup()
	}

	// A raw_json2pptx slide can still contain image/icon URLs or relative asset
	// paths that need the same guarded resolution `generate` performs, so a deck
	// using the escape hatch behaves identically under render and generate. The
	// resolver cache must outlive generation because resolved local paths are
	// embedded in the slides, so its Close is deferred here. Asset-resolution
	// warnings (non-error findings) are collected and merged into the success
	// result's warnings, mirroring `generate`.
	//
	// The URL resolver is created up front (it is only used inside preConvert)
	// so its download dir can join the image allow-list: with
	// ALLOWED_IMAGE_PATHS configured, validated URL downloads must not be
	// rejected as files outside the allowed roots (mirrors generate).
	urlResolver, urlCacheDir, closeResolver, err := newSlideURLResolver(input.Slides)
	if err != nil {
		return semanticRenderResult{}, RenderResult{}, fmt.Errorf("semantic render: %w", err)
	}
	defer closeResolver()
	var preConvertWarnings []string
	preConvert := func() error {
		// Resolve URL references (icon.url, image.url, background.url) by
		// downloading them to a session-scoped cache with SSRF protection.
		if urlResolver != nil {
			if urlFindings := resolveURLs(input.Slides, urlResolver); len(urlFindings) > 0 {
				return iconFindingsToError(urlFindings)
			}
		}
		// Resolve relative asset paths against the spec's own directory so a raw
		// slide's relative image path resolves the same way `generate` resolves
		// it against the deck JSON's directory. Skipped for stdin specs, which
		// have no base directory (mirrors generate's jsonPath != "-" guard).
		if specPath != "-" {
			baseDir := validateBaseDir(specPath, "")
			assetFindings := resolveLocalAssetPaths(input.Slides, baseDir, imageAllowList(cfg.Images.AllowedBasePaths, urlCacheDir)...)
			if assetErr := iconFindingsToError(assetFindings); assetErr != nil {
				return assetErr
			}
			for _, d := range assetFindings {
				if d.Severity != diagnostics.SeverityError {
					preConvertWarnings = append(preConvertWarnings, fmt.Sprintf("%s at %s: %s", d.Code, d.Path, d.Message))
				}
			}
		}
		return nil
	}

	runRes, cleanup, renderErr := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir:         outputDir,
		AllowedImagePaths: imageAllowList(cfg.Images.AllowedBasePaths, urlCacheDir),
		TemplatesDir:      cfg.Templates.Dir,
		OutputValidation:  outputValidation,
		AccentStrategy:    patterns.AccentStrategy(input.AccentStrategy),
		SVGStrategy:       string(cfg.SVG.Strategy),
		SVGScale:          cfg.SVG.Scale,
		SVGNativeCompat:   string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:       cfg.SVG.MaxPNGWidth,
		PreConvert:        preConvert,
	})
	defer cleanup()
	if renderErr != nil {
		return buildSemanticRunFailure(input, compileResult, runRes, renderErr), runRes, nil
	}

	res := buildSemanticRenderSuccess(input, compileResult, runRes, startTime)
	res.Warnings = append(res.Warnings, preConvertWarnings...)
	return res, runRes, nil
}

// writeAuthoringSidecar writes <deck>.pptx.authoring.json beside a rendered
// deck — the spec, the compiled input, the source map and the revision that
// tie a later patch or visual review to this exact file — and records its path
// and revision on the result.
func writeAuthoringSidecar(res *semanticRenderResult, spec *semantic.DeckSpec, source []byte, specPath string, input *PresentationInput, cr *semantic.CompileResult, rr RenderResult) error {
	compiledJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("semantic render: marshal authoring input: %w", err)
	}
	slidePayloads, err := semantic.ExpandedSlidePayloads(spec)
	if err != nil {
		return fmt.Errorf("semantic render: build manifest slide payloads: %w", err)
	}
	diagnosticJSON, _ := json.Marshal(res.Diagnostics)
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(specPath)), ".")
	manifest := pipeline.NewAuthoringManifest(source, format, rr.TemplatePath, rr.TemplateHash, compiledJSON, rr.OutputPath, res.ContentHash, cr.SourceMap, slidePayloads, diagnosticJSON)
	refs := assignedSlideRefs(specPath, source)
	manifest.SlideIDs = make([]string, len(refs))
	for i, r := range refs {
		manifest.SlideIDs[i] = r.ID
	}
	manifestPath := rr.OutputPath + authoringManifestSuffix
	if err := pipeline.WriteAuthoringManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("semantic render: write authoring manifest: %w", err)
	}
	res.Revision = manifest.Revision
	res.ManifestPath = manifestPath
	res.Slides = refs
	return nil
}

// assignedSlideRefs is the table of contents of a spec rendered from the
// command line, with the ids MCP render_deck_spec gives the same spec on its
// first store: an authored id is kept and every other authored slide takes
// the next free s<N> (withSlideIDs from a fresh counter). The CLI used to
// assign none, so render-slide --slide-id and render-thumbnails --slides only
// worked for a deck whose author had written ids (go-slide-creator-cmwmg).
// The spec file itself is not rewritten; the ids are recorded in the sidecar.
func assignedSlideRefs(specPath string, source []byte) []slideRef {
	canonical, name := canonicalSpec(specPath, source)
	withIDs, _ := withSlideIDs(canonical, 0)
	return specDeckState(name, withIDs, "").refs()
}

// authoringManifestSuffix is appended to a deck's path to name its authoring
// manifest sidecar.
const authoringManifestSuffix = ".authoring.json"

// removeOrphanAuthoringManifest deletes <deck>.pptx.authoring.json when the
// deck it describes does not exist. output is the --out value; only a .pptx
// file destination names a deck path that can be checked.
func removeOrphanAuthoringManifest(output string) {
	if !strings.HasSuffix(strings.ToLower(output), ".pptx") {
		return
	}
	if _, err := os.Stat(output); err == nil {
		return
	}
	_ = os.Remove(output + authoringManifestSuffix)
}

// newSlideURLResolver creates the guarded URL resolver a deck needs, or returns
// a nil resolver and empty dir when its slides reference no URLs. dir is the
// resolver's download cache, which must join the image allow-list; closeFn is
// always non-nil.
func newSlideURLResolver(slides []SlideInput) (resolver *resource.Resolver, dir string, closeFn func(), err error) {
	if !hasURLReferences(slides) {
		return nil, "", func() {}, nil
	}
	resolver, err = resource.NewResolver(resource.ResolverOptions{})
	if err != nil {
		return nil, "", func() {}, fmt.Errorf("resource resolver: %w", err)
	}
	return resolver, resolver.Dir(), resolver.Close, nil
}

// emitSemanticRenderResult prints a completed render result and decides the
// process exit.
//
// The exit code follows the one severity model (go-slide-creator-x9rhq): 0
// exactly when the deck was written and no blocking (error-severity) finding
// remains, whatever --output-validation is; "ok" in the printed result agrees
// with it. A clean render exits 0 while publishable remains false pending an
// all-slide visual verdict.
func emitSemanticRenderResult(res semanticRenderResult) error {
	ready := res.DeterministicReady == nil || *res.DeterministicReady
	if !ready {
		// "ok" and the exit code say the same thing: the deck is written AND no
		// blocking finding remains. The file stays on disk either way.
		res.OK = false
		res.Error = fmt.Sprintf("deck written to %s but blocking findings remain: %s",
			res.OutputPath, strings.Join(res.DeterministicBlockingReasons, "; "))
	}
	if err := printJSONIndent(res); err != nil {
		return err
	}
	if ready {
		return nil
	}
	return fmt.Errorf("semantic render: %s", res.Error)
}

// buildSemanticRenderSuccess assembles the compact success result: compile-time
// advisory diagnostics (already semantic-path-scoped) plus render-time fit
// findings mapped back to semantic source paths, a merged warnings list, and a
// quality summary computed over the compiled slides.
func buildSemanticRenderSuccess(input *PresentationInput, cr *semantic.CompileResult, rr RenderResult, start time.Time) semanticRenderResult {
	var sm *semantic.SourceMap
	var ir *semantic.DeckIR
	if cr != nil {
		sm = cr.SourceMap
		ir = cr.IR
	}

	var diags []semanticDiagnostic
	if cr != nil {
		for _, d := range cr.Diagnostics {
			diags = append(diags, semanticDiagFromCompile(d))
		}
		if cr.IR != nil {
			for _, d := range requiredLayoutTemplateDiagnostics(cr.IR.LayoutCoverage.Requested, input.Template, rr.TemplateLayouts) {
				diags = append(diags, semanticDiagFromCompile(d))
			}
		}
	}

	var fit []patterns.FitFinding
	fit = append(fit, rr.SynthesisFindings...)
	if rr.GenResult != nil {
		fit = append(fit, rr.GenResult.FitFindings...)
	}
	fit = append(fit, rr.StrictFitFindings...)
	fit = append(fit, rr.GridVisualFindings...)
	// The render path used to report only what generation happened to emit, so
	// the recommended new-deck path was the blindest tool in the server: a deck
	// whose titles all wrap and whose bullets run to 100 words came back with
	// diagnostics=null and score 100, while validate_input on the SAME compiled
	// deck reported eight findings. Run the shared collectors here too
	// (go-slide-creator-05wn).
	fit = append(fit, collectFitFindings(input, rr.TemplateLayouts, rr.SlideWidth, rr.SlideHeight, &rr.TemplateTheme)...)
	fit = collapseRotatedAccentFindings(dedupFitFindings(fit))
	patterns.SortCanonical(fit, slidepath.SlideIndex)
	diags = append(diags, finishFitDiagnostics(sm, ir, fit)...)

	// The deck's waivers turn the storyline findings it names into advisories:
	// they stay in the list, and neither the score nor the gate counts them
	// (go-slide-creator-oh3qr).
	policy := newFindingPolicy(ir)
	policy.applyWaivers(diags)
	gateFit := policy.gateFindings(fit)

	var warnings []string
	warnings = append(warnings, rr.GridDiagWarnings...)
	if rr.GenResult != nil {
		warnings = append(warnings, rr.GenResult.Warnings...)
	}

	res := semanticRenderResult{
		OK:         true,
		OutputPath: rr.OutputPath,
		Overwrote:  rr.Overwrote,
		Template:   input.Template,
		DurationMs: time.Since(start).Milliseconds(),
		Warnings:   warnings,
		Quality:    semanticQualityScorePtr(input, gateFit, warnings, rr.TemplateLayouts),
		Waivers:    policy.recorded(),
	}
	// A gate criterion no single finding accounts for is reported as one
	// deck-level error, so the gate never fails without a finding to name.
	diags = append(diags, qualityGateDiagnostics(res.Quality.QualityGate, diags)...)
	deckSpecWording(diags, input, ir)
	res.Diagnostics = groupRootCauses(diags)
	if rr.GenResult != nil {
		res.SlideCount = rr.GenResult.SlideCount
		res.ContentHash = rr.GenResult.ContentHash
	}
	evidence := &pipeline.QualityEvidence{ArtifactSHA256: res.ContentHash, SchemaValid: true, Generated: true, FitChecked: true, StructuralValid: !hasBlockingOutputFinding(rr.OutputValidationFindings), TotalSlides: res.SlideCount}
	evidence.Finalize()
	res.Quality.Evidence = evidence

	status := semanticPublicationStatus(res.Diagnostics, res.Quality, res.ContentHash)
	res.DeterministicReady = &status.DeterministicReady
	res.Publishable = &status.Publishable
	res.ManualReviewRequired = &status.ManualReviewRequired
	res.BlockingReasons = status.BlockingReasons
	res.DeterministicBlockingReasons = status.DeterministicBlockingReasons
	return res
}

// buildSemanticRenderFailure assembles a compact failure result from the
// compile diagnostics (when compilation got far enough to produce them) and any
// render-time refusal findings, mapping the latter back to semantic source
// paths where the source map allows it.
func buildSemanticRenderFailure(cr *semantic.CompileResult, err error) semanticRenderResult {
	res := semanticRenderResult{OK: false, Error: err.Error()}

	var sm *semantic.SourceMap
	if cr != nil {
		sm = cr.SourceMap
		for _, d := range cr.Diagnostics {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
	}

	// A strict_fit refusal carries raw text-fit findings; trace each back to its
	// semantic source path via the source map (raw path only as a fallback).
	var refusal *StrictFitRefusal
	if errors.As(err, &refusal) {
		for _, f := range refusal.Findings {
			var ir *semantic.DeckIR
			if cr != nil {
				ir = cr.IR
			}
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromFitWithIR(sm, ir, f))
		}
	}
	// A generation refusal (unreadable or lost text) carries its code, path
	// and measurement; keep them as a source-addressed diagnostic rather than
	// leaving the error string as the only diagnosis (go-slide-creator-b7qqg.4).
	if d := semanticRefusalDiagnostic(cr, err); d != nil {
		res.Diagnostics = append(res.Diagnostics, *d)
	}
	// A pattern that refused the area it was given (a kpis region too short
	// for a value over its caption) names the region to resize
	// (go-slide-creator-uj9zq).
	var spe *slidePatternError
	if errors.As(err, &spe) {
		for _, f := range patternAreaRefusals(spe.err, slidepath.SlideField(spe.slideIdx, spe.field)) {
			d := semanticDiagFromFit(sm, f)
			d.Severity = string(diagnostics.SeverityError)
			res.Diagnostics = append(res.Diagnostics, d)
		}
	}
	return res
}

// semanticDiagFromCompile adapts a semantic compile/validate diagnostic into the
// compact render diagnostic. These diagnostics are authored against the semantic
// DeckSpec, so their path is already a semantic source path. Post-compile raw
// preflight findings additionally stash the originating raw path and a
// recommended semantic edit under Details (see internal/semantic/preflight.go);
// they are lifted onto the compact fields here so an agent sees the same fix
// guidance a render-time fit finding carries.
func semanticDiagFromCompile(d diagnostics.Diagnostic) semanticDiagnostic {
	sd := semanticDiagnostic{
		Code:         d.Code,
		Severity:     string(d.Severity),
		Blocking:     d.Severity == diagnostics.SeverityError,
		Message:      d.Message,
		SemanticPath: d.Path,
	}
	source := d
	sd.diag = &source
	if isProductPlaceholder(d) {
		// Scaffolding the product itself emitted is never a finished deck: the
		// DeckSpec surfaces block on it (go-slide-creator-327g6).
		source.Severity = diagnostics.SeverityError
		sd.Severity, sd.Blocking = string(diagnostics.SeverityError), true
		sd.Evidence = map[string]any{semantic.PlaceholderDetail: d.Details[semantic.PlaceholderDetail]}
	}
	if d.Details != nil {
		if rp, ok := d.Details["raw_path"].(string); ok {
			sd.RawPath = rp
		}
		if e, ok := d.Details["recommended_edit"].(*semantic.SemanticEdit); ok {
			sd.RecommendedEdit = e
		}
	}
	return sd
}

// semanticDiagFromFit adapts a raw render fit finding into the compact render
// diagnostic, mapping its raw JSON path back to the semantic source path the
// author wrote (exact match first, then nearest ancestor). The raw path is
// always retained as fallback evidence so the precise generated location is
// never lost, the semantic slide index is recovered even on a full miss, and a
// recommended semantic edit is attached for common density/overflow failures.
func semanticDiagFromFit(sm *semantic.SourceMap, f patterns.FitFinding) semanticDiagnostic {
	mapped := semantic.MapFinding(sm, semantic.RawFinding{
		Code:     f.Code,
		Message:  f.Message,
		Severity: diagnostics.FromFitFinding(f).Severity,
		Action:   f.Action,
		RawPath:  f.Path,
	})
	d := semanticDiagnostic{
		Code:            mapped.Code,
		Severity:        string(mapped.Severity),
		Message:         mapped.Message,
		SemanticPath:    mapped.SemanticPath,
		RawPath:         mapped.RawPath,
		Action:          mapped.Action,
		RecommendedEdit: mapped.Edit,
	}
	// One severity model (go-slide-creator-x9rhq): a finding the quality gate
	// blocks on is an error, and everything else from the fit pass is an
	// advisory. A blocking BODY_TOO_LONG used to read "warning" here and "info"
	// in validate_deck_spec.
	d.Severity, d.Blocking = fitFindingSeverity(f)
	if mapped.SlideIndex >= 0 {
		idx := mapped.SlideIndex
		d.SlideIndex = &idx
		d.Message = slideNumberMessage(d.Message, slidepath.SlideIndex(f.Path), idx)
	}
	attachFixParams(&d, f.Fix)
	source := diagnostics.FromFitFinding(f)
	source.Severity = diagnostics.Severity(d.Severity)
	source.Message = d.Message
	if d.SemanticPath != "" {
		source.Path = d.SemanticPath
	}
	d.diag = &source
	return d
}

// attachFixParams carries a raw fix's budgets onto the recommended semantic
// edit (go-slide-creator-pi6ea). A finding with a budget but no recommended
// edit gets the edit its budget implies.
func attachFixParams(d *semanticDiagnostic, fix *patterns.FixSuggestion) {
	if fix == nil || d.SemanticPath == "" {
		return
	}
	params := semanticFixParams(fix.Kind, fix.Params)
	if params == nil {
		return
	}
	if d.RecommendedEdit == nil {
		switch {
		case params["max_items"] != nil:
			d.RecommendedEdit = &semantic.SemanticEdit{Kind: semantic.EditReduceItems, Hint: "Remove or merge items to fit the max_items budget."}
		case params["max_chars"] != nil || params["max_length"] != nil || params["max_words"] != nil:
			d.RecommendedEdit = &semantic.SemanticEdit{Kind: semantic.EditShortenText, Hint: "Shorten this field to the budget in params."}
		default:
			return
		}
	} else {
		edit := *d.RecommendedEdit
		d.RecommendedEdit = &edit
	}
	d.RecommendedEdit.Params = params
}

// semanticDiagFromFitWithIR resolves a generated shape's text back to a
// DeckSpec matrix axis-end field when the source map cannot: generated OOXML
// shape IDs are allocated after compilation and therefore have no raw JSON
// source-map entry. Duplicate labels are deliberately left unmapped rather
// than guessing which axis the author should edit.
// matrixTextAmbiguous reports whether text also appears as the slide's title,
// takeaway, an axis title or a quadrant. A rendered shape is identified by its
// text, so any such match would make axis-end attribution ambiguous.
func matrixTextAmbiguous(slide semantic.SlideIR, text string) bool {
	if slide.Title == text || slide.Takeaway == text || slide.Body["x_axis"] == text || slide.Body["x_axis_label"] == text || slide.Body["y_axis"] == text || slide.Body["y_axis_label"] == text {
		return true
	}
	for _, quadrant := range slides.MatrixQuadrants(slide.Body) {
		if quadrant.Header == text || quadrant.Body == text {
			return true
		}
	}
	return false
}

// uniqueBodyFieldWithText attributes a rendered shape on a non-matrix slide to
// the one top-level string body field carrying exactly this text (e.g. a
// bridge caption); anything ambiguous returns "" rather than a guess.
func uniqueBodyFieldWithText(slide semantic.SlideIR, text string) string {
	if slide.Title == text || slide.Takeaway == text {
		return ""
	}
	var only string
	for field, v := range slide.Body {
		if s, ok := v.(string); ok && s == text {
			if only != "" {
				return ""
			}
			only = field
		}
	}
	return only
}

func semanticDiagFromFitWithIR(sm *semantic.SourceMap, ir *semantic.DeckIR, f patterns.FitFinding) semanticDiagnostic {
	d := semanticDiagFromFitField(sm, ir, f)
	// A render finding always names where in the DeckSpec to act. A slide-level
	// finding with no source link (MISSING_TITLE addresses the whole raw slide)
	// carries the slide's own locator, and MISSING_TITLE the field to write
	// (go-slide-creator-y81vn).
	rawIdx := slidepath.SlideIndex(f.Path)
	if rawIdx < 0 {
		return d
	}
	slide := slideSemanticPath(sm, rawIdx)
	path := d.SemanticPath
	if path == "" {
		path = slide
	}
	if f.Code == patterns.ErrCodeMissingTitle && path != "" && path == slide {
		path += ".title"
	}
	if path != d.SemanticPath {
		d.SemanticPath = path
		if d.diag != nil {
			d.diag.Path = path
		}
	}
	return d
}

// semanticDiagFromFitField is semanticDiagFromFit plus the field-level
// attribution of a rendered shape's text that the source map cannot make.
func semanticDiagFromFitField(sm *semantic.SourceMap, ir *semantic.DeckIR, f patterns.FitFinding) semanticDiagnostic {
	d := semanticDiagFromFit(sm, f)
	resolveLateBoundSemanticPath(&d, ir, f)
	rawIdx := slidepath.SlideIndex(f.Path)
	if d.SemanticPath == "" && rawIdx >= 0 && sm != nil {
		// A finding on a generated object the author never wrote (the slide's
		// pattern, a grid cell, a chrome placeholder) still names the slide to
		// edit, never only a compiled pointer.
		d.SemanticPath = sm.SlidePath(rawIdx)
	}
	if d.diag != nil {
		if d.SemanticPath != "" {
			d.diag.Path = d.SemanticPath
		}
		decorateReadabilityRefusal(d.diag, ir, sm, f)
	}
	narrowWrapBoxPaths(&d, ir, f)
	return d
}

// stringList reads a fix param that is a list of strings, as the collector
// builds it or as JSON decodes it.
func stringList(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// narrowWrapBoxPaths gives a TEXT_WRAPS_NARROW finding the authored items
// behind its boxes (go-slide-creator-pfyeg). The finding sat on the slide and
// said "8 boxes": an agent had to guess that the boxes of a process slide are
// its steps. Each box's paragraph locates the field (or the item, when a box
// joins a label and a description) that carries it; the finding moves to the
// first and lists them all in paths.
func narrowWrapBoxPaths(d *semanticDiagnostic, ir *semantic.DeckIR, f patterns.FitFinding) {
	if f.Code != patterns.ErrCodeTextWrapsNarrow || f.Fix == nil || ir == nil || len(d.members) > 0 {
		return
	}
	rawIdx := slidepath.SlideIndex(f.Path)
	if rawIdx < 0 || rawIdx >= len(ir.Slides) || ir.Slides[rawIdx].Kind == semantic.KindRawJSON2pptx {
		return
	}
	var fields []string
	for _, text := range stringList(f.Fix.Params["texts"]) {
		if field := semanticFieldForText(ir, rawIdx, text); field != "" && !slices.Contains(fields, field) {
			fields = append(fields, field)
		}
	}
	if len(fields) == 0 {
		return
	}
	d.SemanticPath = fields[0]
	if d.diag != nil {
		d.diag.Path = fields[0]
	}
	if len(fields) < 2 {
		return
	}
	var params map[string]any
	if d.RecommendedEdit != nil {
		params = d.RecommendedEdit.Params
	}
	for _, field := range fields {
		d.members = append(d.members, diagMember{Path: field, SlideIndex: d.SlideIndex, fixParams: params})
	}
	d.baseMessage = d.Message
}

// resolveLateBoundSemanticPath names the DeckSpec field behind a finding the
// source map cannot place: text in a pattern-generated grid cell, or in a
// rendered shape whose id is allocated after compilation.
func resolveLateBoundSemanticPath(d *semanticDiagnostic, ir *semantic.DeckIR, f patterns.FitFinding) {
	if d.SemanticPath != "" || ir == nil || f.Code != patterns.ErrCodeTextBelowReadableMin || f.Fix == nil {
		return
	}
	if !strings.Contains(f.Path, "/rendered_shapes/") {
		if field := generatedCellSemanticPath(ir, f); field != "" {
			d.SemanticPath = field
		}
		return
	}
	*d = renderedShapeSemanticPath(*d, ir, f)
}

func renderedShapeSemanticPath(d semanticDiagnostic, ir *semantic.DeckIR, f patterns.FitFinding) semanticDiagnostic {
	text, _ := f.Fix.Params["rendered_shape_text"].(string)
	idx := slidepath.SlideIndex(f.Path)
	if text == "" || idx < 0 || idx >= len(ir.Slides) {
		return d
	}
	slide := ir.Slides[idx]
	if slide.SourcePath == "" {
		return d
	}
	if slide.Kind != semantic.KindMatrix2x2 || slide.Visual.Pattern != "matrix-2x2" {
		if field := uniqueBodyFieldWithText(slide, text); field != "" {
			d.SemanticPath = slide.SourcePath + "." + field
		}
		return d
	}
	var matched string
	for _, field := range []string{"x_low", "x_high", "y_low", "y_high"} {
		value, _ := slide.Body[field].(string)
		if value != text {
			continue
		}
		if matched != "" {
			return d
		}
		matched = field
	}
	if matched != "" {
		if matrixTextAmbiguous(slide, text) {
			return d
		}
		d.SemanticPath = slide.SourcePath + "." + matched
		d.RecommendedEdit = &semantic.SemanticEdit{Kind: semantic.EditShortenText, Hint: "Shorten this matrix axis-end label; use at most 11 characters and re-render."}
	}
	return d
}

// runSemanticExplain implements "semantic explain". It parses a semantic spec,
// normalizes it to the compiler IR, and prints the planned decisions — selected
// archetype and resolved template, plus per-slide kind, narrative role, visual
// family, density, and the chosen pattern/layout — together with the deck-rhythm
// warnings the author should address before rendering. It is a read-only
// projection of the compiler's plan: no raw PresentationInput or .pptx is
// emitted, so it works even on specs that still carry advisory findings. A parse
// error is fatal (the spec cannot be planned); it prints the finding envelope to
// stderr and exits non-zero.
func runSemanticExplain() error {
	fs := flag.NewFlagSet("semantic explain", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	templateName := fs.String("template", "", "Template to plan for; replaces the spec's meta.template for this run")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic explain --spec <file> [--template <name>]\n\n")
		fmt.Fprintf(os.Stderr, "Print the compiler's planned decisions (archetype, template, per-slide\n")
		fmt.Fprintf(os.Stderr, "kind/role/family/density/pattern) and deck-rhythm warnings as JSON.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return cliMissingArg("--spec is required")
	}

	data, err := readSpec(*specPath)
	if err != nil {
		return fmt.Errorf("semantic explain: read %s: %w", *specPath, err)
	}

	spec, parseDiags := semantic.Parse(*specPath, data)
	if parseDiags.HasErrors() {
		envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "semantic explain",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}, parseDiags.ToDiagnostics())
		_ = fprintJSONIndent(os.Stdout, envelope)
		return fmt.Errorf("semantic explain: spec could not be parsed")
	}

	// --template replaces meta.template for this run, as on the other
	// `semantic` commands (go-slide-creator-fjuhm).
	choice := resolveSpecTemplate(spec.Meta.Template, *templateName, "", specSource{})
	for _, w := range choice.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	explanation := explainSpecWithTemplate(choice.evaluated(spec), choice.Default)
	if explanation.Template == "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", explainNoTemplateWarning)
		return printJSONIndent(explanation)
	} else if layouts, templateDiagnostic := semanticTemplateLayouts(explanation.Template, "", nil); templateDiagnostic == nil {
		reconcileExplanationTemplateCoverage(&explanation, layouts)
	} else {
		envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "semantic explain",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}, []diagnostics.Diagnostic{*templateDiagnostic})
		_ = fprintJSONIndent(os.Stdout, envelope)
		return fmt.Errorf("semantic explain: template is unavailable")
	}
	return printJSONIndent(explanation)
}

// runSemanticSchema implements "semantic schema". It prints the DeckSpec JSON
// Schema (draft 2020-12) to stdout. The slide-kind and archetype enums are
// derived from the canonical registries, so the schema stays in sync with code.
func runSemanticSchema() error {
	fs := flag.NewFlagSet("semantic schema", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic schema\n\n")
		fmt.Fprintf(os.Stderr, "Print the DeckSpec JSON Schema (draft 2020-12).\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}
	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	out, err := semantic.SchemaJSON()
	if err != nil {
		return fmt.Errorf("semantic schema: %w", err)
	}
	_, err = os.Stdout.Write(append(out, '\n'))
	return err
}

// readSpec reads the semantic spec named by the --spec value. "-" reads the whole
// of os.Stdin directly (portable to Windows, which has no /dev/stdin); any other
// value is read as a file path.
func readSpec(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// writeCompileOutput marshals v as indented JSON (with a trailing newline) and
// writes it to the "semantic compile" --output destination: stdout when output is
// "" or "-", otherwise the named file. It centralizes the envelope-mode output so
// success and blocking-failure paths share one destination convention.
func writeCompileOutput(output string, v any) error {
	if output == "" || output == "-" {
		return printJSONIndent(v)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(output, out, 0o644); err != nil { //nolint:gosec // generated deck JSON is not sensitive
		return fmt.Errorf("semantic compile: write %s: %w", output, err)
	}
	return nil
}

// printJSONIndent writes v as indented JSON (with a trailing newline) to stdout.
func printJSONIndent(v any) error {
	return fprintJSONIndent(os.Stdout, v)
}

// fprintJSONIndent writes v as indented JSON (with a trailing newline) to w.
func fprintJSONIndent(w *os.File, v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	_, err = w.Write(append(out, '\n'))
	return err
}

// semanticQualityScore reports the render's quality summary with the
// deterministic structural score and gate folded in.
//
// quality_summary alone returned 100 for 15 of 16 calibration decks — including
// one whose own diagnostics carried five SEMANTIC_WEAK_CONTENT findings — so the
// recommended path's only numeric feedback was a heuristic that could not fail,
// and the gate SKILL.md calls "the machine-readable definition of done" sat off
// the path behind compile_deck_spec + score_deck (go-slide-creator-05wn).
func semanticQualityScorePtr(input *PresentationInput, fit []patterns.FitFinding, warnings []string, layouts []types.LayoutMetadata) *QualityScore {
	q := computeQualityScoreWithLayouts(input.Slides, warnings, layouts)
	ds := deterministic.ScoreFromFindings(fit, len(input.Slides))
	gate := deterministic.EvaluateQualityGate(ds, fit, deterministic.DefaultQualityGateCriteria())
	q.StructuralScore = ds.OverallScore
	q.QualityGate = gate
	// The headline number must not exceed the structural verdict: an agent reads
	// it first, and "100" on a deck the gate rejects is the failure this fixes.
	if float64(ds.OverallScore) < q.Score {
		q.Score = float64(ds.OverallScore)
	}
	// The per-slide breakdown takes the same structural deductions, so a
	// one-slide deck cannot read 75 overall and 100 for its only slide.
	for i := range q.SlideScores {
		if i >= len(ds.PerSlide) {
			break
		}
		ps := ds.PerSlide[i]
		if float64(ps.Score) < q.SlideScores[i].Score {
			q.SlideScores[i].Score = float64(ps.Score)
		}
		for _, f := range ps.Findings {
			if f.Code != "" && ps.Score < 100 {
				q.SlideScores[i].Issues = append(q.SlideScores[i].Issues, f.Code)
			}
		}
	}
	return q
}
