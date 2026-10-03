package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// One flag convention for every subcommand (go-slide-creator-e6gsz).
//
// The CLI grew one subcommand at a time, so the same idea was spelled four
// ways: the input file was positional for validate, --json for generate,
// --spec for semantic and --pptx for the render commands; the output path was
// --output, --output-dir, --out or missing; and --json meant "read this file"
// in one command and "emit JSON" in the next. The convention is now:
//
//	json2pptx <command> [<input>] [--out <path>] [--format json|text] [--verbose]
//
//   - the primary input is the first positional argument;
//   - --out names the output file or directory;
//   - --format selects the output shape (json, or text where the command has a
//     human-readable form);
//   - --verbose turns on INFO logging on stderr;
//   - -h / --help print to stdout and exit 0;
//   - flags may come before or after positional arguments.
//
// Every older spelling keeps working as a deprecated alias, so existing
// scripts, the CI corpus job and the skill docs are unaffected.
// cliConventions is the per-command record of which legacy flag each part of
// the convention maps onto; cliParse applies it.

// cliConvention maps one flag set onto the shared convention.
type cliConvention struct {
	// Input is the legacy flag that names the primary input (--json, --spec,
	// --pptx, ...). The first positional argument fills it when the flag is
	// absent. Empty when the command has no input or reads it positionally
	// itself (see Positional).
	Input string
	// Out is the legacy flag that names the output path (--output,
	// --output-dir, --out-dir). --out is registered as the canonical spelling
	// of the same value. Empty when the command writes no file, already spells
	// it --out, or writes several artifacts through --out-<kind> flags.
	Out string
	// JSONFlag is the legacy boolean that switches a human-readable command to
	// JSON (--json). --format json|text maps onto it. Empty when the command
	// only emits JSON (then --format json is accepted and text is refused) or
	// defines its own --format.
	JSONFlag string
	// Positional reports that the command reads its own positional arguments
	// (file lists, pattern names, a subject). Otherwise an argument left over
	// after Input is taken is an error rather than silently ignored.
	Positional bool
	// Server marks long-running commands, which take no --format.
	Server bool
}

// cliConventions records the convention for every flag set the CLI defines,
// keyed by the flag set name. TestCLIConventionCoversEveryFlagSet fails when a
// flag.NewFlagSet in this package has no entry here.
func cliConventions() map[string]cliConvention {
	return map[string]cliConvention{
		"generate":          {Input: "json", Out: "output"},
		"read":              {Positional: true},
		"export":            {Input: "pptx", Out: "output-dir"},
		"validate":          {Positional: true},
		"preflight":         {Input: "json"},
		"validate-template": {Positional: true, JSONFlag: "json"},
		"template-check":    {Positional: true, JSONFlag: "json"},
		"examine-template":  {Positional: true},
		"validate-output":   {Positional: true, JSONFlag: "json"},
		"patterns list":     {JSONFlag: "json"},
		"patterns show":     {Positional: true, JSONFlag: "json"},
		"patterns validate": {Positional: true, JSONFlag: "json"},
		"patterns expand":   {Positional: true, JSONFlag: "json"},
		"icons list":        {JSONFlag: "json"},
		"icons search":      {Positional: true, JSONFlag: "json"},
		"templates":         {JSONFlag: "json"},
		"preview-icon":      {Input: "icon"},
		"tables guide":      {JSONFlag: "json"},
		"skill-info":        {},
		"capabilities":      {},
		"get-started":       {Input: "task"},
		"describe-finding":  {Input: "code"},
		"input-schema":      {},
		"resolve-theme":     {Input: "template"},
		"recommend-pattern": {Input: "intent"},
		"preview":           {Input: "json"},
		"preview-wireframe": {Input: "json"},
		"preview-patterns":  {Out: "output"},
		"repair":            {Input: "json"},
		"score":             {Input: "json"},
		"score-candidates":  {Input: "json"},
		"inspect":           {Input: "images"},
		"analyze-rhythm":    {Input: "json"},
		"plan-deck":         {Input: "brief"},
		"recommend-visual":  {Input: "intent"},
		"render-slide":      {Input: "pptx"},
		"render-slide-from-json": {
			Input: "slide",
		},
		"render-thumbnails":          {Input: "pptx", Out: "out-dir"},
		"purge-render-cache":         {},
		"template-settings list":     {Input: "template"},
		"template-settings register": {},
		"template-settings delete":   {},
		"data-format-hints":          {},
		"shape-catalog":              {},
		"audit-palette":              {Positional: true, Out: "output"},
		"semantic validate":          {Input: "spec"},
		"semantic compile":           {Input: "spec", Out: "output"},
		"semantic render":            {Input: "spec", Out: "output"},
		"semantic explain":           {Input: "spec"},
		"semantic schema":            {},
		"semantic kinds":             {Positional: true, JSONFlag: "json"},
		"skill install":              {},
		"skill status":               {},
		"skill cli-map":              {},
		"serve":                      {Server: true},
		"mcp":                        {Server: true, Out: "output"},
	}
}

// cliCurrentCommand is the name of the flag set most recently parsed: the
// subcommand as the user typed it ("render-thumbnails", "semantic render").
// Error envelopes produced by the shared tool handlers are stamped with it so
// a CLI caller does not read "subcommand":"mcp" (go-slide-creator-pikfw).
var cliCurrentCommand string

// cliCurrentFlagSet is the flag set cliParse most recently parsed, and
// cliParseFailed whether that parse refused the arguments; the catch-all error
// envelope reads both (cli_error_envelope.go).
var (
	cliCurrentFlagSet *flag.FlagSet
	cliParseFailed    bool
)

// cliFormatValue implements --format for a command that does not define its
// own: it maps json|text onto the command's legacy --json boolean.
type cliFormatValue struct {
	jsonFlag flag.Value // nil when the command only emits JSON
	value    string
}

func (v *cliFormatValue) String() string { return v.value }

func (v *cliFormatValue) Set(s string) error {
	switch s {
	case "json":
		if v.jsonFlag != nil {
			if err := v.jsonFlag.Set("true"); err != nil {
				return err
			}
		}
	case "text", "human":
		if v.jsonFlag == nil {
			return errors.New("this command only emits json")
		}
		if err := v.jsonFlag.Set("false"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("must be json or text")
	}
	v.value = s
	return nil
}

// cliApplyConvention registers the convention's flags (--out, --format,
// --verbose) on fs and marks the legacy spellings as deprecated aliases in the
// help text. It is idempotent per flag: a flag the command already defines is
// left alone.
func cliApplyConvention(fs *flag.FlagSet) cliConvention {
	conv := cliConventions()[fs.Name()]
	cliCurrentCommand = fs.Name()

	if conv.Out != "" && fs.Lookup("out") == nil {
		if legacy := fs.Lookup(conv.Out); legacy != nil {
			fs.Var(legacy.Value, "out", "Output path: "+legacy.Usage)
			legacy.Usage += " [deprecated alias of --out]"
		}
	}
	if conv.Input != "" {
		if legacy := fs.Lookup(conv.Input); legacy != nil {
			legacy.Usage += " [may also be given as the first argument]"
		}
	}
	if !conv.Server && fs.Lookup("format") == nil {
		fv := &cliFormatValue{}
		usage := "Output format: json (the only format this command emits)"
		if conv.JSONFlag != "" {
			if legacy := fs.Lookup(conv.JSONFlag); legacy != nil {
				fv.jsonFlag = legacy.Value
				usage = "Output format: text (default) or json"
				legacy.Usage += " [deprecated alias of --format json]"
			}
		}
		fs.Var(fv, "format", usage)
	}
	if fs.Lookup("verbose") == nil {
		fs.Bool("verbose", false, "Log INFO-level progress on stderr (default: warnings and errors only)")
	}
	return conv
}

// cliParse parses args into fs under the shared convention. It differs from
// fs.Parse in three ways: flags are accepted before or after positional
// arguments; the convention's --out / --format / --verbose flags are
// registered; and the first positional argument fills the command's primary
// input when the legacy input flag was not given. After it returns, fs.Args()
// holds the positional arguments the command itself should read.
func cliParse(fs *flag.FlagSet, args []string) error {
	cliCurrentFlagSet, cliParseFailed = fs, true
	if err := cliParseArgs(fs, args); err != nil {
		return err
	}
	cliParseFailed = false
	return nil
}

func cliParseArgs(fs *flag.FlagSet, args []string) error {
	conv := cliApplyConvention(fs)

	// Everything after a literal "--" is positional.
	head, tail := args, []string(nil)
	for i, a := range args {
		if a == "--" {
			head, tail = args[:i], args[i+1:]
			break
		}
	}
	var positional []string
	for rest := head; ; {
		if err := fs.Parse(rest); err != nil {
			return err
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		rest = rest[1:]
	}
	positional = append(positional, tail...)

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if conv.Input != "" && !set[conv.Input] && len(positional) > 0 && fs.Lookup(conv.Input) != nil {
		if err := fs.Set(conv.Input, positional[0]); err != nil {
			return fmt.Errorf("invalid argument %q: %w", positional[0], err)
		}
		positional = positional[1:]
	}
	if !conv.Positional && len(positional) > 0 {
		return fmt.Errorf("%s: unexpected argument %q — run 'json2pptx %s -h' for usage",
			fs.Name(), positional[0], fs.Name())
	}
	// --verbose registered by the convention (or generate's own) turns INFO
	// logging on; validate-template's --verbose keeps its own meaning.
	if v := fs.Lookup("verbose"); v != nil && v.Value.String() == "true" && fs.Name() != "validate-template" {
		cliLogLevel.Set(slog.LevelInfo)
	}

	// Leave the positional arguments where commands already look for them.
	return fs.Parse(append([]string{"--"}, positional...))
}

// cliHelpRequested reports whether the argument list (everything after the
// program name) asks for help: no command, a help command, or -h / --help
// anywhere before a literal "--".
func cliHelpRequested(args []string) bool {
	if len(args) == 0 {
		return true
	}
	for i, a := range args {
		if a == "--" {
			break
		}
		switch a {
		case "-h", "--help", "-help":
			return true
		case "help":
			// "help" is a command only in the command slot, or the
			// sub-command slot of a command group.
			if i == 0 || (i == 1 && cliCommandGroups[args[0]]) {
				return true
			}
		}
	}
	// A command group invoked without a subcommand prints its usage.
	return len(args) == 1 && cliCommandGroups[args[0]]
}

// cliCommandGroups are the commands that dispatch a sub-subcommand.
var cliCommandGroups = map[string]bool{
	"patterns": true, "icons": true, "tables": true, "template-settings": true, "semantic": true,
}

// cliDefaultUsage prints the standard double-dash flag listing for a flag set
// that has no hand-written usage, so every command's help reads the same.
func cliDefaultUsage(fs *flag.FlagSet, synopsis, summary string) func() {
	return func() {
		out := fs.Output()
		fmt.Fprintf(out, "Usage: json2pptx %s\n\n", synopsis)
		if summary != "" {
			fmt.Fprintf(out, "%s\n\n", summary)
		}
		fmt.Fprintf(out, "Options:\n")
		printDoubleDashUsage(fs)
	}
}

// ---------------------------------------------------------------------------
// Logging (go-slide-creator-pikfw)
// ---------------------------------------------------------------------------

// cliLogLevel is the level of the CLI's stderr logger: WARN unless --verbose
// (or JSON2PPTX_LOG_LEVEL=info|debug) asks for more.
var cliLogLevel = func() *slog.LevelVar {
	v := &slog.LevelVar{}
	v.Set(slog.LevelWarn)
	return v
}()

// cliLogHandler writes one line per record to w in the format the CLI has
// always logged in ("2006/01/02 15:04:05 LEVEL message key=value"), and drops
// a record identical to one it has already written. The pipeline legitimately
// runs layout selection and pattern expansion once per pass (validate, fit
// measurement, render), and each pass logged the same line again — five
// copies of every "pattern expanded" for one render.
type cliLogHandler struct {
	w     io.Writer
	level slog.Leveler
	attrs []slog.Attr
	state *cliLogState
}

type cliLogState struct {
	mu   sync.Mutex
	seen map[string]bool
}

func newCLILogHandler(w io.Writer, level slog.Leveler) *cliLogHandler {
	return &cliLogHandler{w: w, level: level, state: &cliLogState{seen: map[string]bool{}}}
}

func (h *cliLogHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level.Level()
}

func (h *cliLogHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Level.String())
	b.WriteByte(' ')
	b.WriteString(r.Message)
	writeAttr := func(a slog.Attr) {
		if a.Equal(slog.Attr{}) {
			return
		}
		val := a.Value.Resolve().String()
		if val == "" || strings.ContainsAny(val, " \t\n\"=") {
			val = fmt.Sprintf("%q", val)
		}
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(val)
	}
	for _, a := range h.attrs {
		writeAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(a)
		return true
	})
	line := b.String()

	h.state.mu.Lock()
	defer h.state.mu.Unlock()
	if h.state.seen[line] {
		return nil
	}
	h.state.seen[line] = true
	ts := r.Time
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := fmt.Fprintf(h.w, "%s %s\n", ts.Format("2006/01/02 15:04:05"), line)
	return err
}

func (h *cliLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &next
}

// WithGroup flattens groups: the CLI log line has no nesting.
func (h *cliLogHandler) WithGroup(string) slog.Handler { return h }

// setupCLILogging installs the CLI logger: stderr, WARN by default, each
// distinct line once. The serve and mcp commands install their own loggers
// afterwards.
func setupCLILogging() {
	switch strings.ToLower(os.Getenv("JSON2PPTX_LOG_LEVEL")) {
	case "debug":
		cliLogLevel.Set(slog.LevelDebug)
	case "info":
		cliLogLevel.Set(slog.LevelInfo)
	case "error":
		cliLogLevel.Set(slog.LevelError)
	}
	slog.SetDefault(slog.New(newCLILogHandler(os.Stderr, cliLogLevel)))
}

// cliFlagSetNames returns the sorted flag set names the convention table
// covers.
func cliFlagSetNames() []string {
	names := make([]string, 0, len(cliConventions()))
	for name := range cliConventions() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
