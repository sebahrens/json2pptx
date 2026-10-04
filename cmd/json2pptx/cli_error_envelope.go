package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"

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
// --json. It reads the flag set cliParse saw, so it is false before any
// command parsed its flags (an unknown command, a group without a subcommand).
func cliWantsJSON() bool {
	set := cliCurrentFlagSet
	if set == nil || cliConventions()[set.Name()].Server {
		return false
	}
	f := set.Lookup("format")
	if f == nil {
		return false
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

// cliErrorCode is the envelope code of a plain error. Nothing is read from the
// message: a missing file and malformed JSON are recognised by their error
// types wherever they were wrapped, a command's own argument error carries its
// code (cliCodedError), and an argument list the flag parser refused is an
// invalid parameter. Anything else is a failure of the command itself.
func cliErrorCode(err error) diagnostics.Code {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	var coded *cliCodedError
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return diagnostics.CodeFileNotFound
	case errors.As(err, &syn), errors.As(err, &typ):
		return diagnostics.CodeInvalidJSON
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
