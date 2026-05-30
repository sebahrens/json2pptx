package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
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
  schema     Print the DeckSpec JSON Schema (draft 2020-12)

Examples:
  json2pptx semantic validate --spec deck.yaml
  json2pptx semantic validate --spec deck.yaml --strict strict
  json2pptx semantic compile --spec deck.yaml --output compiled.json
  json2pptx semantic compile --spec deck.yaml --output -      # stdout
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
