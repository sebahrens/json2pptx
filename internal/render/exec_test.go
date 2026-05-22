package render

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestRunBounded_Success verifies a fast command runs to completion and its
// stdout is captured.
func TestRunBounded_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX echo semantics")
	}
	stdout, _, err := runBounded(context.Background(), toolLibreOffice, "/tmp/x.pptx", 2*time.Second, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout = %q, want it to contain 'hello'", stdout)
	}
}

// TestRunBounded_TimeoutReturnsStructuredError simulates a hung subprocess (a
// long sleep) and asserts runBounded kills it well before it would finish and
// returns a *TimeoutError carrying tool, path, code, and elapsed time.
func TestRunBounded_TimeoutReturnsStructuredError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX sleep semantics")
	}
	start := time.Now()
	_, _, err := runBounded(context.Background(), toolLibreOffice, "/tmp/deck.pptx", 80*time.Millisecond, "sleep", "5")
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Fatalf("runBounded did not return promptly on timeout: took %s (subprocess was not killed)", elapsed)
	}

	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TimeoutError, got %T: %v", err, err)
	}
	if te.Code != codeLibreOfficeTimeout {
		t.Errorf("code = %q, want %q", te.Code, codeLibreOfficeTimeout)
	}
	if te.Tool != toolLibreOffice {
		t.Errorf("tool = %q, want %q", te.Tool, toolLibreOffice)
	}
	if te.Path != "/tmp/deck.pptx" {
		t.Errorf("path = %q, want /tmp/deck.pptx", te.Path)
	}
	if te.Elapsed <= 0 || te.Elapsed > 3*time.Second {
		t.Errorf("elapsed = %s, want a small positive duration", te.Elapsed)
	}
	if te.Timeout != 80*time.Millisecond {
		t.Errorf("timeout = %s, want 80ms", te.Timeout)
	}

	// The message must carry tool, path, and a recovery action so surfaces that
	// only forward err.Error() still convey an actionable diagnostic.
	msg := te.Error()
	for _, want := range []string{"libreoffice", "/tmp/deck.pptx", "Retry"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q missing %q", msg, want)
		}
	}
}

// TestRunBounded_ImageMagickTimeoutCode confirms the magick tool yields the
// IMAGEMAGICK_TIMEOUT code.
func TestRunBounded_ImageMagickTimeoutCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX sleep semantics")
	}
	_, _, err := runBounded(context.Background(), toolImageMagick, "/tmp/deck.pdf", 60*time.Millisecond, "sleep", "5")
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("expected *TimeoutError, got %T: %v", err, err)
	}
	if te.Code != codeImageMagickTimeout {
		t.Errorf("code = %q, want %q", te.Code, codeImageMagickTimeout)
	}
}

func TestTimeoutCodeForTool(t *testing.T) {
	if got := timeoutCodeForTool(toolImageMagick); got != codeImageMagickTimeout {
		t.Errorf("magick code = %q, want %q", got, codeImageMagickTimeout)
	}
	if got := timeoutCodeForTool(toolLibreOffice); got != codeLibreOfficeTimeout {
		t.Errorf("libreoffice code = %q, want %q", got, codeLibreOfficeTimeout)
	}
}

// TestCapWriter_BoundsOutput verifies the writer retains only the first limit
// bytes but reports a full write so the subprocess never blocks on a full pipe.
func TestCapWriter_BoundsOutput(t *testing.T) {
	w := &capWriter{limit: 10}
	n, err := w.Write([]byte("0123456789ABCDEF"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 16 {
		t.Errorf("Write reported %d, want 16 (full length so the process never blocks)", n)
	}
	if got := w.String(); got != "0123456789" {
		t.Errorf("retained %q, want the first 10 bytes", got)
	}
}
