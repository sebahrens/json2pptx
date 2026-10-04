package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// The catch-all error envelope (go-slide-creator-pikfw).
//
// A command that answers in JSON reported most failures as a finding envelope
// on stdout, but any path that returned a plain Go error — a file that is not
// there, an undefined flag, a value out of range — printed one "Error: ..."
// line on stderr and left stdout empty, so a caller parsing stdout had to
// special-case "no output". main now closes that gap: when a JSON command
// fails and has written nothing to stdout, the error is printed there as the
// shared finding envelope, stamped with the command's own name. A command
// that already reported its failure on stdout is left alone, which is why
// stdout is counted rather than every failure path being marked by hand.

// cliStdoutTap counts what a command writes to stdout. os.Stdout is an
// *os.File, so the count is taken on a pipe that is copied to the real stream.
type cliStdoutTap struct {
	real *os.File
	w    *os.File
	done chan struct{}
	n    int64
}

// cliTap is the active tap; nil when stdout is not tapped (server commands,
// or a pipe could not be opened).
var cliTap *cliStdoutTap

// cliStartStdoutTap routes os.Stdout through a counting pipe for the run of
// one command. The long-running servers keep the real stream: they own stdout
// for their protocol and never end with an envelope.
func cliStartStdoutTap(args []string) {
	if len(args) == 0 || cliConventions()[args[0]].Server {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	// Group commands shift os.Args as they dispatch; the envelope needs the
	// line as it was typed.
	cliRawArgs = append([]string(nil), args...)
	t := &cliStdoutTap{real: os.Stdout, w: w, done: make(chan struct{})}
	go func() {
		t.n, _ = io.Copy(t.real, r)
		_ = r.Close()
		close(t.done)
	}()
	os.Stdout = w
	cliTap = t
}

// cliStopStdoutTap restores os.Stdout once everything written so far has
// reached it, and returns the number of bytes the command wrote. tapped is
// false when stdout was never tapped.
func cliStopStdoutTap() (written int64, tapped bool) {
	t := cliTap
	if t == nil {
		return 0, false
	}
	cliTap = nil
	os.Stdout = t.real
	_ = t.w.Close()
	<-t.done
	return t.n, true
}

// cliExit ends the process with code after flushing the stdout tap; a command
// that exits on its own (preflight's exit codes) calls it instead of os.Exit.
func cliExit(code int) {
	cliStopStdoutTap()
	os.Exit(code)
}

// cliWantsJSON reports whether the command that just ran answers in JSON: a
// command that only emits JSON, or one switched to it with --format json /
// --json. It reads the flag set cliParse saw; before any command parsed its
// flags (an unknown command, a group without a subcommand) it reads the
// argument list for --format json.
func cliWantsJSON() bool {
	set := cliCurrentFlagSet
	if set == nil {
		// No command parsed its flags: an unknown command, or a group given
		// a subcommand it does not have. A caller that wrote --format json
		// still reads stdout for the answer.
		return cliArgsAskForJSON(nil, cliRawArgs)
	}
	if cliConventions()[set.Name()].Server {
		return false
	}
	f := set.Lookup("format")
	if f == nil {
		return false
	}
	// The flag parser stops at the argument it refuses, so a --format json
	// after it was never read: `patterns list --bogus --format json` answered
	// with usage text on stderr and nothing on stdout. The caller still asked
	// for JSON, and the argument list says so.
	if cliParseFailed && cliArgsAskForJSON(set, cliParsedArgs) {
		return true
	}
	if fv, ok := f.Value.(*cliFormatValue); ok {
		// No legacy --json boolean behind it: the command only emits JSON.
		return fv.jsonFlag == nil || fv.jsonFlag.String() == "true"
	}
	// The command defines --format itself. For validate and audit-palette it
	// is the output shape; for plan-deck and export it selects what is
	// produced, and the answer on stdout is JSON whatever it says.
	switch f.Value.String() {
	case "json", "ndjson":
		return true
	case "", "text", "human":
		if j := set.Lookup("json"); j != nil {
			if b, ok := j.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
				return j.Value.String() == "true"
			}
		}
		return false
	}
	return cliConventions()[set.Name()].JSONFlag == ""
}

// cliRawArgs is the process's argument list as it was typed, recorded when
// stdout is tapped.
var cliRawArgs []string

// cliParsedArgs is the argument list the current command's flag set was
// given (see cliParse).
var cliParsedArgs []string

// cliArgsAskForJSON reports whether args name JSON output — --format json, or
// the command's boolean --json — before a literal "--". It is consulted only
// when parsing failed, to honour a format flag the parser never reached.
func cliArgsAskForJSON(set *flag.FlagSet, args []string) bool {
	boolJSON := false
	if set == nil {
		// Without a flag set, --json may be a command's input file flag.
	} else if j := set.Lookup("json"); j != nil {
		if b, ok := j.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			boolJSON = true
		}
	}
	for i, a := range args {
		if a == "--" {
			break
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if !strings.HasPrefix(a, "-") {
			continue
		}
		switch name {
		case "format":
			if !hasValue && i+1 < len(args) {
				value = args[i+1]
			}
			if value == "json" || value == "ndjson" {
				return true
			}
		case "json":
			if boolJSON && (!hasValue || value == "true") {
				return true
			}
		}
	}
	return false
}

// cliCodedError is a command-line failure that states its own finding code.
// The command that refuses an argument knows why — a flag is missing, a value
// is not one it accepts, a file is not there — so it says so where it returns
// the error, instead of the catch-all reading the code back out of the
// message wording (go-slide-creator-fbft2). The message is the wrapped
// error's, unchanged.
type cliCodedError struct {
	code diagnostics.Code
	err  error
}

func (e *cliCodedError) Error() string { return e.err.Error() }
func (e *cliCodedError) Unwrap() error { return e.err }

func cliCoded(code diagnostics.Code, format string, args ...any) error {
	return &cliCodedError{code: code, err: fmt.Errorf(format, args...)}
}

// cliMissingArg is the error for a required flag, argument or subcommand that
// was not given (MISSING_PARAMETER). The format takes %w.
func cliMissingArg(format string, args ...any) error {
	return cliCoded(diagnostics.CodeMissingParameter, format, args...)
}

// cliInvalidArg is the error for a value the command does not accept: an
// unknown name, a value outside its set, flags that exclude each other, an
// argument too many (INVALID_PARAMETER).
func cliInvalidArg(format string, args ...any) error {
	return cliCoded(diagnostics.CodeInvalidParameter, format, args...)
}

// cliNotFound is the error for a named file or resource that does not exist
// (FILE_NOT_FOUND).
func cliNotFound(format string, args ...any) error {
	return cliCoded(diagnostics.CodeFileNotFound, format, args...)
}

// cliInvalidJSON is the error for a flag or file whose JSON does not parse or
// has the wrong shape (INVALID_JSON).
func cliInvalidJSON(format string, args ...any) error {
	return cliCoded(diagnostics.CodeInvalidJSON, format, args...)
}

// cliSlideError is the error for deck content the pipeline cannot place: a
// content item without a type, an overlay with no target, a layout no slide
// type can use (INVALID_SLIDE). A wrapped error that states its own code keeps
// it — "slide 2: shape_grid: <pattern error>" stays a pattern error. The
// shared pipeline files return these to every command that reads a deck, so
// the code is stated here rather than by each caller; an untyped one reached
// the CLI as INTERNAL, whose remediation is "retry" (go-slide-creator-u1c9c).
func cliSlideError(format string, args ...any) error {
	return cliCodedUnlessWrapped(diagnostics.CodeInvalidSlide, format, args...)
}

// cliPatternError is the error for a pattern block that cannot be expanded: a
// values shape the pattern does not take, an override it does not have, a
// nested pattern next to other cell content (PATTERN_ERROR).
func cliPatternError(format string, args ...any) error {
	return cliCodedUnlessWrapped(diagnostics.CodePatternError, format, args...)
}

// cliCodedUnlessWrapped builds a coded error that keeps the code of an error
// it wraps: the innermost site knows best what is wrong.
func cliCodedUnlessWrapped(code diagnostics.Code, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	var inner *cliCodedError
	if errors.As(err, &inner) {
		code = inner.code
	}
	return &cliCodedError{code: code, err: err}
}

// cliErrorCode is the envelope code of a plain error. Nothing is read from the
// message: a missing file and malformed JSON are recognised by their error
// types wherever they were wrapped, as are a pattern's input findings, a
// command's own argument error carries its code (cliCodedError), and an
// argument list the flag parser refused is an invalid parameter. Anything else is a failure of the command itself.
func cliErrorCode(err error) diagnostics.Code {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	var coded *cliCodedError
	var patternInput *patternInputError
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return diagnostics.CodeFileNotFound
	case errors.As(err, &syn), errors.As(err, &typ):
		return diagnostics.CodeInvalidJSON
	case errors.As(err, &patternInput):
		return diagnostics.CodePatternError
	case errors.As(err, &coded):
		return coded.code
	case cliParseFailed:
		return diagnostics.CodeInvalidParameter
	}
	return diagnostics.CodeInternal
}

// cliErrorEnvelope is the finding envelope for a command that failed with a
// plain error.
func cliErrorEnvelope(err error) diagnostics.FindingEnvelope {
	name := ""
	if cliCurrentFlagSet != nil {
		name = cliCurrentFlagSet.Name()
	}
	// A pattern's own input findings are located and coded one per field;
	// they are reported as they are rather than joined into one message.
	if ds := patternInputDiagnostics(err, "", ""); len(ds) > 0 {
		return diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: name}, ds)
	}
	return diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: name}, []diagnostics.Diagnostic{{
		Code: cliErrorCode(err), Message: err.Error(), Severity: diagnostics.SeverityError,
	}})
}

// cliReportFailure writes the catch-all envelope for err to w when the
// command answers in JSON and wrote nothing itself. It reports whether it
// wrote one.
func cliReportFailure(w io.Writer, err error, written int64) bool {
	if err == nil || written != 0 || !cliWantsJSON() || errors.Is(err, flag.ErrHelp) {
		return false
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(cliErrorEnvelope(err)) == nil
}
