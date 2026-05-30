package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/semantic"
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
		return fmt.Errorf("semantic requires a subcommand: validate, compile, or schema")
	}

	sub := os.Args[1]
	// Shift args so each sub-subcommand sees its own flags (mirrors dispatch).
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch sub {
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
		return fmt.Errorf("unknown semantic subcommand %q — run 'json2pptx semantic help' for usage", sub)
	}
}

// printSemanticUsage prints the semantic command group help.
func printSemanticUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx semantic <subcommand> [options]

Compile compact semantic deck specs (DeckSpec) into the raw json2pptx
PresentationInput model.

Subcommands:
  validate   Validate a semantic spec; emit the shared finding envelope
  compile    Compile a semantic spec to raw PresentationInput JSON
  render     Compile a semantic spec and render it straight to a .pptx
  explain    Print the compiler's planned decisions and rhythm warnings
  schema     Print the DeckSpec JSON Schema (draft 2020-12)

Examples:
  json2pptx semantic validate --spec deck.yaml
  json2pptx semantic validate --spec deck.yaml --strict strict
  json2pptx semantic compile --spec deck.yaml --output compiled.json
  json2pptx semantic compile --spec deck.yaml --output -      # stdout
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
		return "", fmt.Errorf("invalid --strict value %q: must be off, warn, or strict", v)
	}
}

// runSemanticValidate implements "semantic validate". It parses and validates a
// semantic spec and prints the shared FindingEnvelope (the same shape every
// other diagnostic-bearing surface emits) to stdout. The process exits non-zero
// when any finding has error severity.
func runSemanticValidate() error {
	fs := flag.NewFlagSet("semantic validate", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	strict := fs.String("strict", "warn", "Advisory-rule strictness: off, warn, or strict")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic validate --spec <file> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Validate a semantic deck spec and print the shared finding envelope.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return fmt.Errorf("--spec is required")
	}
	strictness, err := parseStrictness(*strict)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(specReadPath(*specPath))
	if err != nil {
		return fmt.Errorf("semantic validate: read %s: %w", *specPath, err)
	}

	ds := semantic.Check(*specPath, data, strictness)
	envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
		Subcommand:  "semantic validate",
		InputSHA256: diagnostics.ComputeInputSHA256(data),
	}, ds)

	if err := printJSONIndent(envelope); err != nil {
		return err
	}
	if !envelope.OK {
		return fmt.Errorf("semantic validation failed")
	}
	return nil
}

// runSemanticCompile implements "semantic compile". It parses, validates, and
// compiles a semantic spec into a raw PresentationInput and writes the indented
// JSON to --output (a path, or - for stdout). The raw JSON is consumable by
// `json2pptx validate` and `json2pptx generate` for debugging or advanced edits.
// Blocking (error-severity) findings abort the compile: the finding envelope is
// printed to stderr and the process exits non-zero.
func runSemanticCompile() error {
	fs := flag.NewFlagSet("semantic compile", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	output := fs.String("output", "-", "Where to write the raw PresentationInput JSON; use - for stdout")
	strict := fs.String("strict", "warn", "Advisory-rule strictness: off, warn, or strict")
	templateName := fs.String("template", "", "Default template used when the spec pins none")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic compile --spec <file> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Compile a semantic deck spec to raw PresentationInput JSON.\n")
		fmt.Fprintf(os.Stderr, "The output is accepted by 'json2pptx validate' and 'json2pptx generate'.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return fmt.Errorf("--spec is required")
	}
	strictness, err := parseStrictness(*strict)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(specReadPath(*specPath))
	if err != nil {
		return fmt.Errorf("semantic compile: read %s: %w", *specPath, err)
	}

	spec, parseDiags := semantic.Parse(*specPath, data)
	if parseDiags.HasErrors() {
		envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "semantic compile",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}, parseDiags.ToDiagnostics())
		_ = fprintJSONIndent(os.Stderr, envelope)
		return fmt.Errorf("semantic compile: spec could not be parsed")
	}

	input, result, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: *templateName,
	})
	if err != nil {
		envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "semantic compile",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}, result.Diagnostics)
		_ = fprintJSONIndent(os.Stderr, envelope)
		return fmt.Errorf("semantic compile: %w", err)
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
	OK          bool                 `json:"ok"`
	OutputPath  string               `json:"output_path,omitempty"`
	Template    string               `json:"template,omitempty"`
	SlideCount  int                  `json:"slide_count,omitempty"`
	ContentHash string               `json:"content_hash,omitempty"`
	DurationMs  int64                `json:"duration_ms,omitempty"`
	Quality     *QualityScore        `json:"quality,omitempty"`
	Warnings    []string             `json:"warnings,omitempty"`
	Diagnostics []semanticDiagnostic `json:"diagnostics,omitempty"`
	Error       string               `json:"error,omitempty"`
}

// semanticDiagnostic is one compact finding in a render result. SemanticPath
// points at the field in the semantic DeckSpec the author wrote; RawPath is the
// originating generated pointer, retained as fallback evidence (and the only
// locator when a finding could not be traced back to a semantic source path).
// SlideIndex is the semantic slide the finding belongs to, or -1. RecommendedEdit
// names a semantic edit that should resolve the finding, when one is known.
type semanticDiagnostic struct {
	Code            string                 `json:"code"`
	Severity        string                 `json:"severity,omitempty"`
	Message         string                 `json:"message"`
	SemanticPath    string                 `json:"semantic_path,omitempty"`
	RawPath         string                 `json:"raw_path,omitempty"`
	SlideIndex      *int                   `json:"slide_index,omitempty"`
	Action          string                 `json:"action,omitempty"`
	RecommendedEdit *semantic.SemanticEdit `json:"recommended_edit,omitempty"`
}

// runSemanticRender implements "semantic render": the target one-command flow
// from a compact semantic spec to a rendered .pptx. It parses and validates the
// spec, compiles it to a raw PresentationInput, runs the shared in-memory render
// runner (RunPresentation, which keeps strict output validation as the default),
// maps raw render findings back to the semantic source paths the author wrote
// (falling back to the raw path only when no mapping exists), and prints a
// compact result with a quality summary. Blocking failures print the same
// compact result (OK=false) to stderr and exit non-zero.
func runSemanticRender() error {
	fs := flag.NewFlagSet("semantic render", flag.ContinueOnError)
	specPath := fs.String("spec", "", "Path to the semantic deck spec (.yaml/.yml/.json); use - for stdin")
	output := fs.String("output", "", "Output .pptx path (or directory); required")
	strict := fs.String("strict", "warn", "Advisory-rule strictness: off, warn, or strict")
	templateName := fs.String("template", "", "Default template used when the spec pins none")
	templatesDir := fs.String("templates-dir", "", "Template search directory")
	outputValidation := fs.String("output-validation", "strict", "Post-generation output validation: off, warn, or strict")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic render --spec <file> --output <file.pptx> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Compile a semantic deck spec and render it straight to a .pptx using the\n")
		fmt.Fprintf(os.Stderr, "shared generation pipeline. Strict output validation is the default.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return fmt.Errorf("--spec is required")
	}
	if *output == "" {
		fs.Usage()
		return fmt.Errorf("--output is required")
	}
	strictness, err := parseStrictness(*strict)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(specReadPath(*specPath))
	if err != nil {
		return fmt.Errorf("semantic render: read %s: %w", *specPath, err)
	}

	startTime := time.Now()

	// Parse the spec. A parse error is fatal and has no source map yet, so the
	// findings carry their native semantic paths.
	spec, parseDiags := semantic.Parse(*specPath, data)
	if parseDiags.HasErrors() {
		res := semanticRenderResult{OK: false, Error: "semantic render: spec could not be parsed"}
		for _, d := range parseDiags.ToDiagnostics() {
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromCompile(d))
		}
		_ = fprintJSONIndent(os.Stderr, res)
		return fmt.Errorf("%s", res.Error)
	}

	// Validate + compile to a raw PresentationInput. Blocking findings abort the
	// render with the diagnostics surfaced on the result.
	input, compileResult, err := semantic.Compile(spec, semantic.CompileOptions{
		Strict:          strictness,
		DefaultTemplate: *templateName,
	})
	if err != nil {
		res := buildSemanticRenderFailure(compileResult, err)
		_ = fprintJSONIndent(os.Stderr, res)
		return fmt.Errorf("semantic render: %w", err)
	}

	// Accept a .pptx file path or a directory for --output (mirrors `generate`):
	// a file destination splits into parent dir + filename.
	outputDir := *output
	if strings.HasSuffix(strings.ToLower(outputDir), ".pptx") {
		input.OutputFilename = filepath.Base(outputDir)
		outputDir = filepath.Dir(outputDir)
	}

	// Apply the shared pre-render prep that a compiled deck still needs: deck
	// defaults (table/cell styles) and named style references. Most compiled
	// slides carry constrained design mode and no URL/asset refs, so structure
	// expansion and design-mode revalidation do not apply — but the raw_json2pptx
	// escape hatch passes author-authored slide payloads straight through, so the
	// URL/asset resolution PreConvert hook below still does (see preConvert).
	applyDefaults(input)
	resolveInputNamedSettingsForDir(*templatesDir, input)

	// Default SVG knobs from the standard config so charts/diagrams render with
	// the same native-SVG strategy the CLI generate path uses.
	cfg := config.DefaultConfig()
	if *templatesDir != "" {
		cfg.Templates.Dir = *templatesDir
	}

	// A raw_json2pptx slide can still contain image/icon URLs or relative asset
	// paths that need the same guarded resolution `generate` performs, so a deck
	// using the escape hatch behaves identically under render and generate. The
	// resolver cache must outlive generation because resolved local paths are
	// embedded in the slides; the cleanup is deferred here and installed by the
	// hook. Asset-resolution warnings (non-error findings) are collected and
	// merged into the success result's warnings, mirroring `generate`.
	urlResolverCleanup := func() {}
	defer func() { urlResolverCleanup() }()
	var preConvertWarnings []string
	preConvert := func() error {
		// Resolve URL references (icon.url, image.url, background.url) by
		// downloading them to a session-scoped cache with SSRF protection.
		if hasURLReferences(input.Slides) {
			resolver, resolverErr := resource.NewResolver(resource.ResolverOptions{})
			if resolverErr != nil {
				return fmt.Errorf("resource resolver: %w", resolverErr)
			}
			urlResolverCleanup = func() { resolver.Close() }
			if urlFindings := resolveURLs(input.Slides, resolver); len(urlFindings) > 0 {
				return iconFindingsToError(urlFindings)
			}
		}
		// Resolve relative asset paths against the spec's own directory so a raw
		// slide's relative image path resolves the same way `generate` resolves
		// it against the deck JSON's directory. Skipped for stdin specs, which
		// have no base directory (mirrors generate's jsonPath != "-" guard).
		if *specPath != "-" {
			baseDir := validateBaseDir(*specPath, "")
			assetFindings := resolveLocalAssetPaths(input.Slides, baseDir)
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
		OutputDir:        outputDir,
		TemplatesDir:     cfg.Templates.Dir,
		OutputValidation: *outputValidation,
		AccentStrategy:   patterns.AccentStrategy(input.AccentStrategy),
		SVGStrategy:      string(cfg.SVG.Strategy),
		SVGScale:         cfg.SVG.Scale,
		SVGNativeCompat:  string(cfg.SVG.NativeCompatibility),
		MaxPNGWidth:      cfg.SVG.MaxPNGWidth,
		PreConvert:       preConvert,
	})
	defer cleanup()
	if renderErr != nil {
		res := buildSemanticRenderFailure(compileResult, renderErr)
		_ = fprintJSONIndent(os.Stderr, res)
		return fmt.Errorf("semantic render: %w", renderErr)
	}

	res := buildSemanticRenderSuccess(input, compileResult, runRes, startTime)
	res.Warnings = append(res.Warnings, preConvertWarnings...)
	return printJSONIndent(res)
}

// buildSemanticRenderSuccess assembles the compact success result: compile-time
// advisory diagnostics (already semantic-path-scoped) plus render-time fit
// findings mapped back to semantic source paths, a merged warnings list, and a
// quality summary computed over the compiled slides.
func buildSemanticRenderSuccess(input *PresentationInput, cr *semantic.CompileResult, rr RenderResult, start time.Time) semanticRenderResult {
	var sm *semantic.SourceMap
	if cr != nil {
		sm = cr.SourceMap
	}

	var diags []semanticDiagnostic
	if cr != nil {
		for _, d := range cr.Diagnostics {
			diags = append(diags, semanticDiagFromCompile(d))
		}
	}

	var fit []patterns.FitFinding
	fit = append(fit, rr.SynthesisFindings...)
	if rr.GenResult != nil {
		fit = append(fit, rr.GenResult.FitFindings...)
	}
	fit = append(fit, rr.StrictFitFindings...)
	fit = append(fit, rr.GridVisualFindings...)
	for _, f := range fit {
		diags = append(diags, semanticDiagFromFit(sm, f))
	}

	var warnings []string
	warnings = append(warnings, rr.GridDiagWarnings...)
	if rr.GenResult != nil {
		warnings = append(warnings, rr.GenResult.Warnings...)
	}

	res := semanticRenderResult{
		OK:          true,
		OutputPath:  rr.OutputPath,
		Template:    input.Template,
		DurationMs:  time.Since(start).Milliseconds(),
		Warnings:    warnings,
		Diagnostics: diags,
		Quality:     computeQualityScore(input.Slides, warnings),
	}
	if rr.GenResult != nil {
		res.SlideCount = rr.GenResult.SlideCount
		res.ContentHash = rr.GenResult.ContentHash
	}
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
			res.Diagnostics = append(res.Diagnostics, semanticDiagFromFit(sm, convertTextFitFinding(f)))
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
		Message:      d.Message,
		SemanticPath: d.Path,
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
	if mapped.SlideIndex >= 0 {
		idx := mapped.SlideIndex
		d.SlideIndex = &idx
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

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx semantic explain --spec <file>\n\n")
		fmt.Fprintf(os.Stderr, "Print the compiler's planned decisions (archetype, template, per-slide\n")
		fmt.Fprintf(os.Stderr, "kind/role/family/density/pattern) and deck-rhythm warnings as JSON.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *specPath == "" {
		fs.Usage()
		return fmt.Errorf("--spec is required")
	}

	data, err := os.ReadFile(specReadPath(*specPath))
	if err != nil {
		return fmt.Errorf("semantic explain: read %s: %w", *specPath, err)
	}

	spec, parseDiags := semantic.Parse(*specPath, data)
	if parseDiags.HasErrors() {
		envelope := diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{
			Subcommand:  "semantic explain",
			InputSHA256: diagnostics.ComputeInputSHA256(data),
		}, parseDiags.ToDiagnostics())
		_ = fprintJSONIndent(os.Stderr, envelope)
		return fmt.Errorf("semantic explain: spec could not be parsed")
	}

	return printJSONIndent(semantic.ExplainSpec(spec))
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
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	out, err := semantic.SchemaJSON()
	if err != nil {
		return fmt.Errorf("semantic schema: %w", err)
	}
	_, err = os.Stdout.Write(append(out, '\n'))
	return err
}

// specReadPath maps the --spec value to a path readable by os.ReadFile, routing
// "-" to stdin via /dev/stdin (matching readJSONInput's stdin convention).
func specReadPath(path string) string {
	if path == "-" {
		return "/dev/stdin"
	}
	return path
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
