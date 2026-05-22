package render

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// External-subprocess execution policy.
//
// LibreOffice and ImageMagick are invoked as child processes, and a package-
// level mutex serializes LibreOffice (it is single-threaded per process).
// Without a deadline a wedged converter would hold that mutex — and the calling
// render tool (render_slide_image, render_deck_thumbnails, the visual_qa loop) —
// forever. runBounded gives every subprocess an explicit deadline, kills its
// whole process group on timeout (LibreOffice forks a soffice.bin worker that
// outlives a bare kill of the parent), captures bounded stdout/stderr, and
// returns a structured *TimeoutError so callers can emit a machine-readable
// timeout diagnostic with a suggested retry/degrade action.

// Per-step deadlines for the external render subprocesses. They are vars, not
// consts, so tests can shrink them to force a timeout deterministically.
var (
	libreOfficeTimeout = 90 * time.Second
	imageMagickTimeout = 60 * time.Second
)

// maxCapturedOutput bounds how much subprocess stdout/stderr we retain. A
// runaway tool can emit unbounded output; we keep only the leading bytes (errors
// surface early) so a hung or chatty process cannot balloon memory.
const maxCapturedOutput = 8 * 1024

// Tool identifiers, doubling as the binary names invoked. Used in
// TimeoutError.Tool and to select the diagnostic code.
const (
	toolLibreOffice = "libreoffice"
	toolImageMagick = "magick"
)

// Timeout diagnostic codes. They mirror the diagnostics taxonomy
// (diagnostics.CodeLibreOfficeTimeout / CodeImageMagickTimeout) but are declared
// here so the low-level render package stays free of that dependency; the cmd
// layer forwards TimeoutError.Code verbatim.
const (
	codeLibreOfficeTimeout = "LIBREOFFICE_TIMEOUT"
	codeImageMagickTimeout = "IMAGEMAGICK_TIMEOUT"
)

// TimeoutError is returned when a bounded external subprocess exceeds its
// deadline and is killed. It carries enough context for a caller to surface a
// structured timeout diagnostic: which tool timed out, the input path where
// known, how long it ran, the deadline it blew, and a captured stderr tail.
type TimeoutError struct {
	Tool    string        // "libreoffice" or "magick"
	Code    string        // diagnostic code: LIBREOFFICE_TIMEOUT / IMAGEMAGICK_TIMEOUT
	Path    string        // input path being processed, when known
	Elapsed time.Duration // wall time before the process was killed
	Timeout time.Duration // the deadline that was exceeded
	Stderr  string        // bounded leading bytes of subprocess stderr
}

// Error implements error. The message bundles tool, path, elapsed, and the
// suggested recovery action so transport surfaces that only forward err.Error()
// (e.g. api.MCPSimpleError) still convey the full, actionable diagnostic.
func (e *TimeoutError) Error() string {
	msg := fmt.Sprintf("%s timed out after %s (deadline %s)",
		e.Tool, e.Elapsed.Round(time.Millisecond), e.Timeout)
	if e.Path != "" {
		msg += fmt.Sprintf(" while processing %s", e.Path)
	}
	if action := e.Action(); action != "" {
		msg += ". " + action
	}
	return msg
}

// Action returns the suggested retry/degrade guidance for this timeout.
func (e *TimeoutError) Action() string {
	return "Retry with force=true; if it recurs the renderer is likely wedged — " +
		"restart the render environment, or skip image rendering and ship the .pptx without thumbnails."
}

// timeoutCodeForTool maps a tool identifier to its diagnostic code.
func timeoutCodeForTool(tool string) string {
	if tool == toolImageMagick {
		return codeImageMagickTimeout
	}
	return codeLibreOfficeTimeout
}

// capWriter is an io.Writer that retains only the first limit bytes written. It
// always reports a full write so the subprocess is never blocked on a full pipe;
// bytes past the limit are discarded.
type capWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *capWriter) Write(p []byte) (int, error) {
	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		if len(p) <= remaining {
			w.buf.Write(p)
		} else {
			w.buf.Write(p[:remaining])
		}
	}
	return len(p), nil
}

func (w *capWriter) String() string { return w.buf.String() }

// runBounded runs name+args as a child process with an explicit deadline derived
// from ctx, capturing bounded stdout/stderr. On deadline it kills the child's
// whole process group (see setProcessGroup) and returns a *TimeoutError; other
// failures are returned unwrapped so callers can add tool-specific context.
func runBounded(ctx context.Context, tool, path string, timeout time.Duration, name string, args ...string) (stdout, stderr string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, name, args...) //nolint:gosec // callers pass clamped ints / internal temp paths; tool name is a package constant
	setProcessGroup(cmd)                            // platform-specific: own process group + group-kill on cancel
	cmd.WaitDelay = 5 * time.Second

	var outBuf, errBuf capWriter
	outBuf.limit = maxCapturedOutput
	errBuf.limit = maxCapturedOutput
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	if cctx.Err() == context.DeadlineExceeded {
		return outBuf.String(), errBuf.String(), &TimeoutError{
			Tool:    tool,
			Code:    timeoutCodeForTool(tool),
			Path:    path,
			Elapsed: elapsed,
			Timeout: timeout,
			Stderr:  errBuf.String(),
		}
	}
	return outBuf.String(), errBuf.String(), runErr
}
