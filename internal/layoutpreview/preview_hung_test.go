//go:build unix

package layoutpreview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/render"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Fake renderers for go-slide-creator-b7qqg.6. Each answers --version (the
// cache-identity probe) and otherwise either hangs — after forking a
// grandchild that records its PID, like LibreOffice's soffice.bin worker — or
// does the minimum real work. The binaries are named libreoffice / magick so
// they win the lookup order over any real install on CI.
const (
	fakeHungScript = `#!/bin/sh
if [ "$1" = "--version" ]; then echo "fake 1.0"; exit 0; fi
sleep 300 &
pidf="$FAKE_PID_DIR/$(basename "$0").pid"
echo $! > "$pidf.tmp" && mv "$pidf.tmp" "$pidf"
wait
`
	fakeOfficeWritesPDF = `#!/bin/sh
if [ "$1" = "--version" ]; then echo "fake 1.0"; exit 0; fi
out=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--outdir" ]; then out="$2"; shift; fi
  shift
done
: > "$out/layouts.pdf"
`
)

func installFakeRenderers(t *testing.T, office, magick string) (pidDir string) {
	t.Helper()
	bin := t.TempDir()
	pidDir = t.TempDir()
	for name, body := range map[string]string{"libreoffice": office, "magick": magick} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil { //nolint:gosec // test fixture must be executable
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/bin"+string(os.PathListSeparator)+"/usr/bin")
	t.Setenv("FAKE_PID_DIR", pidDir)
	return pidDir
}

func hungPreviewFixture(t *testing.T) (string, *types.TemplateAnalysis) {
	t.Helper()
	templatePath := "../../templates/midnight-blue.pptx"
	reader, err := template.OpenTemplate(templatePath)
	if err != nil {
		t.Skipf("template unavailable: %v", err)
	}
	defer func() { _ = reader.Close() }()
	layouts, err := template.ParseLayouts(reader)
	if err != nil {
		t.Fatal(err)
	}
	return templatePath, &types.TemplateAnalysis{TemplatePath: templatePath, Layouts: layouts, Theme: template.ParseTheme(reader)}
}

func shrinkPreviewTimeouts(t *testing.T, office, magick time.Duration) {
	t.Helper()
	oldLO, oldIM := previewLibreOfficeTimeout, previewImageMagickTimeout
	previewLibreOfficeTimeout, previewImageMagickTimeout = office, magick
	t.Cleanup(func() { previewLibreOfficeTimeout, previewImageMagickTimeout = oldLO, oldIM })
}

// assertGrandchildReaped checks the fake's forked worker was killed with its
// process group rather than orphaned.
func assertGrandchildReaped(t *testing.T, pidFile string) {
	t.Helper()
	data, err := os.ReadFile(pidFile) //nolint:gosec // test temp dir
	if err != nil {
		t.Fatalf("fake renderer never started its worker: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("worker %d outlived the timed-out renderer", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func assertNoPreviewTempDirs(t *testing.T, tmp string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(tmp, "layoutpreview-*"))
	if len(left) > 0 {
		t.Fatalf("preview temp dirs left behind: %v", left)
	}
}

func assertNoPartialPreviews(t *testing.T, cacheDir string) {
	t.Helper()
	var leftovers []string
	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			leftovers = append(leftovers, path)
		}
		return nil
	})
	if len(leftovers) > 0 {
		t.Fatalf("failed preview build left cache files: %v", leftovers)
	}
}

func TestGenerateContext_HungLibreOfficeTimesOut(t *testing.T) {
	templatePath, analysis := hungPreviewFixture(t)
	pidDir := installFakeRenderers(t, fakeHungScript, fakeHungScript)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	shrinkPreviewTimeouts(t, 300*time.Millisecond, 300*time.Millisecond)
	cacheDir := t.TempDir()

	start := time.Now()
	res, err := GenerateContext(context.Background(), templatePath, analysis, &Options{CacheDir: cacheDir})
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("hung LibreOffice held discovery for %s", elapsed)
	}
	if res != nil {
		t.Fatalf("result = %+v, want nil on timeout", res)
	}
	var te *render.TimeoutError
	if !errors.As(err, &te) || te.Code != "LIBREOFFICE_TIMEOUT" {
		t.Fatalf("err = %v, want a LIBREOFFICE_TIMEOUT *render.TimeoutError", err)
	}
	for _, want := range []string{"layout preview libreoffice convert", "read_only=true", "--no-preview"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks actionable guidance %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "force=true") {
		t.Errorf("error %q cites the render tools' force=true, which discovery does not take", err)
	}
	assertGrandchildReaped(t, filepath.Join(pidDir, "libreoffice.pid"))
	assertNoPreviewTempDirs(t, tmp)
	assertNoPartialPreviews(t, cacheDir)
}

func TestGenerateContext_HungRasterizerTimesOut(t *testing.T) {
	templatePath, analysis := hungPreviewFixture(t)
	pidDir := installFakeRenderers(t, fakeOfficeWritesPDF, fakeHungScript)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	shrinkPreviewTimeouts(t, 5*time.Second, 300*time.Millisecond)
	cacheDir := t.TempDir()

	start := time.Now()
	_, err := GenerateContext(context.Background(), templatePath, analysis, &Options{CacheDir: cacheDir})
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("hung rasterizer held discovery for %s", elapsed)
	}
	var te *render.TimeoutError
	if !errors.As(err, &te) || te.Code != "IMAGEMAGICK_TIMEOUT" {
		t.Fatalf("err = %v, want an IMAGEMAGICK_TIMEOUT *render.TimeoutError", err)
	}
	if !strings.Contains(err.Error(), "rasterize layout") {
		t.Errorf("error %q does not name the failing step", err)
	}
	assertGrandchildReaped(t, filepath.Join(pidDir, "magick.pid"))
	assertNoPreviewTempDirs(t, tmp)
	assertNoPartialPreviews(t, cacheDir)
}

func TestGenerateContext_CancellationStopsHungRenderer(t *testing.T) {
	templatePath, analysis := hungPreviewFixture(t)
	pidDir := installFakeRenderers(t, fakeHungScript, fakeHungScript)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	// Deadlines far beyond the test: only cancellation can end the run.
	shrinkPreviewTimeouts(t, time.Hour, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pidFile := filepath.Join(pidDir, "libreoffice.pid")
	go func() {
		// Cancel once the fake has actually started its worker. The PID file
		// is renamed into place, so it never appears empty; allow a loaded
		// host up to 30s to get there.
		for i := 0; i < 3000; i++ {
			if _, err := os.Stat(pidFile); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()
	start := time.Now()
	_, err := GenerateContext(ctx, templatePath, analysis, &Options{CacheDir: t.TempDir()})
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("cancellation took %s", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	assertGrandchildReaped(t, pidFile)
	assertNoPreviewTempDirs(t, tmp)
}
