package render

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// These tests cover go-slide-creator-0ixs: LibreOffice conversions must run in a
// profile no other process owns. Sharing the default profile makes a concurrent
// soffice exit 0 and write nothing at all — measured on macOS, 2 of 4 concurrent
// conversions produced no PDF and logged nothing.

// fakeOffice installs a stub `soffice` on PATH that records each invocation's
// argv (one line per call) in the returned log path. When writePDF is true it
// also produces the PDF LibreOffice would have written, so the caller can choose
// between the success path and the silent-failure path this bead is about.
func fakeOffice(t *testing.T, writePDF bool) (logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub relies on POSIX shell semantics")
	}
	dir := t.TempDir()
	logPath = filepath.Join(dir, "argv.log")

	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\n"
	if writePDF {
		// Mirror LibreOffice's output naming: <outdir>/<input base>.pdf.
		body += `outdir=""
input=""
while [ $# -gt 0 ]; do
  case "$1" in
    --outdir) outdir="$2"; shift 2 ;;
    *.pptx) input="$1"; shift ;;
    *) shift ;;
  esac
done
# PATH holds only this stub, so use shell expansion rather than basename(1).
base="${input##*/}"
base="${base%.pptx}"
printf '%%PDF-1.4 stub' > "$outdir/$base.pdf"
`
	}
	body += "exit 0\n"

	script := filepath.Join(dir, "soffice")
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil { //nolint:gosec // test stub must be executable
		t.Fatal(err)
	}
	// PATH is the stub dir alone so officeCommand cannot resolve a real
	// libreoffice ahead of the stub.
	t.Setenv("PATH", dir)
	return logPath
}

func readInvocations(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// TestPPTXToPDFUsesPrivateProfile is the regression test for the bug: the
// conversion must carry -env:UserInstallation. Before the fix it carried none
// and every process on the machine shared one profile.
func TestPPTXToPDFUsesPrivateProfile(t *testing.T) {
	logPath := fakeOffice(t, true)
	tmpDir := t.TempDir()
	pptx := filepath.Join(tmpDir, "deck.pptx")
	if err := os.WriteFile(pptx, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}

	pdf, err := pptxToPDF(context.Background(), pptx, tmpDir)
	if err != nil {
		t.Fatalf("pptxToPDF: %v", err)
	}
	if pdf != filepath.Join(tmpDir, "deck.pdf") {
		t.Errorf("pdf path = %q, want %q", pdf, filepath.Join(tmpDir, "deck.pdf"))
	}

	calls := readInvocations(t, logPath)
	if len(calls) != 1 {
		t.Fatalf("got %d soffice invocations, want 1: %v", len(calls), calls)
	}
	if !strings.Contains(calls[0], "-env:UserInstallation=file://") {
		t.Errorf("conversion ran without a private profile — concurrent renders will collide.\nargv: %s", calls[0])
	}
}

// TestPPTXToPDFRetriesWithFreshProfile pins the recovery path: when LibreOffice
// exits successfully and writes no PDF (the collision signature), the conversion
// is retried once against a profile nothing has touched, and the final error
// tells the caller it is an environment collision rather than a broken deck.
func TestPPTXToPDFRetriesWithFreshProfile(t *testing.T) {
	logPath := fakeOffice(t, false)
	tmpDir := t.TempDir()
	pptx := filepath.Join(tmpDir, "deck.pptx")
	if err := os.WriteFile(pptx, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := pptxToPDF(context.Background(), pptx, tmpDir)
	if err == nil {
		t.Fatal("expected an error when no PDF is produced")
	}

	calls := readInvocations(t, logPath)
	if len(calls) != 2 {
		t.Fatalf("got %d soffice invocations, want 2 (one attempt + one retry): %v", len(calls), calls)
	}
	first, second := profileOf(t, calls[0]), profileOf(t, calls[1])
	if first == "" || second == "" {
		t.Fatalf("both attempts must pass a profile; got %q and %q", first, second)
	}
	if first == second {
		t.Errorf("retry reused the same profile %q; a profile the first attempt could not own is not worth a second try", first)
	}

	for _, want := range []string{"another LibreOffice instance", "retry this call", "not that the deck is invalid"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message does not mention %q, so an agent will blame the deck: %v", want, err)
		}
	}
}

// profileOf extracts the -env:UserInstallation directory from a recorded argv.
func profileOf(t *testing.T, argv string) string {
	t.Helper()
	const prefix = "-env:UserInstallation=file://"
	for _, f := range strings.Fields(argv) {
		if strings.HasPrefix(f, prefix) {
			return strings.TrimPrefix(f, prefix)
		}
	}
	return ""
}

// TestLibreOfficeProfileIsStableAndProcessScoped checks the profile is created
// once and reused (a cold profile costs LibreOffice ~0.55s to bootstrap) and
// that its name carries this process's pid, so two servers on one machine cannot
// land on the same directory.
func TestLibreOfficeProfileIsStableAndProcessScoped(t *testing.T) {
	first, err := libreOfficeProfile()
	if err != nil {
		t.Fatalf("libreOfficeProfile: %v", err)
	}
	second, err := libreOfficeProfile()
	if err != nil {
		t.Fatalf("libreOfficeProfile (second call): %v", err)
	}
	if first != second {
		t.Errorf("profile is not reused: %q then %q", first, second)
	}
	if info, err := os.Stat(first); err != nil || !info.IsDir() {
		t.Errorf("profile %q is not an existing directory: %v", first, err)
	}
	if !strings.Contains(filepath.Base(first), fmt.Sprintf("-%d-", os.Getpid())) {
		t.Errorf("profile %q does not carry this process's pid %d", first, os.Getpid())
	}
}

func TestLOProfileArg(t *testing.T) {
	if got := loProfileArg(""); got != nil {
		t.Errorf("empty dir should yield no argument, got %v", got)
	}
	got := loProfileArg(t.TempDir())
	if len(got) != 1 || !strings.HasPrefix(got[0], "-env:UserInstallation=file:///") {
		t.Errorf("loProfileArg = %v, want a single absolute file:// URL argument", got)
	}
}

// TestConcurrentConversion is the bead's named acceptance test
// (go test ./internal/render -run TestConcurrentConversion): four independent
// PROCESSES converting the same deck at once must all produce a PDF. The
// in-process mutex cannot make this pass — only a per-process profile can — so
// the children are real subprocesses, re-execing this test binary.
func TestConcurrentConversion(t *testing.T) {
	if os.Getenv("RENDER_CONVERT_CHILD") != "" {
		runConversionChild(t)
		return
	}
	if err := CheckDependencies(); err != nil {
		t.Skipf("skipping integration test: %v", err)
	}
	pptxPath := integrationPPTXPath(t)

	const workers = 4
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make([]string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outDir := filepath.Join(t.TempDir(), fmt.Sprintf("w%d", i))
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				failures[i] = err.Error()
				return
			}
			cmd := exec.Command(exe, "-test.run", "^TestConcurrentConversion$", "-test.v") //nolint:gosec // re-exec of this test binary
			cmd.Env = append(os.Environ(),
				"RENDER_CONVERT_CHILD=1",
				"RENDER_CONVERT_PPTX="+pptxPath,
				"RENDER_CONVERT_OUTDIR="+outDir,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				failures[i] = fmt.Sprintf("worker %d: %v\n%s", i, err, out)
			}
		}(i)
	}
	wg.Wait()

	var failed []string
	for _, f := range failures {
		if f != "" {
			failed = append(failed, f)
		}
	}
	if len(failed) > 0 {
		t.Fatalf("%d of %d concurrent conversions failed — LibreOffice profiles are colliding:\n%s",
			len(failed), workers, strings.Join(failed, "\n"))
	}
}

// runConversionChild performs the single conversion a TestConcurrentConversion
// subprocess exists to do.
func runConversionChild(t *testing.T) {
	t.Helper()
	pptx, outDir := os.Getenv("RENDER_CONVERT_PPTX"), os.Getenv("RENDER_CONVERT_OUTDIR")
	if pptx == "" || outDir == "" {
		t.Fatal("child invoked without RENDER_CONVERT_PPTX / RENDER_CONVERT_OUTDIR")
	}
	pdf, err := pptxToPDF(context.Background(), pptx, outDir)
	if err != nil {
		t.Fatalf("child conversion failed: %v", err)
	}
	if _, err := os.Stat(pdf); err != nil {
		t.Fatalf("child produced no PDF at %s: %v", pdf, err)
	}
	// The retry alone would make this test pass even with the profiles colliding,
	// so require the first attempt to have worked: that is what a private profile
	// buys, and a regression in it shows up here rather than as a slow render.
	if n := LibreOfficeRetryCount(); n != 0 {
		t.Fatalf("conversion needed %d retry/retries — the private profile is not isolating this process", n)
	}
}
