//go:build unix

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fakeSofficeWithWorker(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "soffice")
	content := "#!/bin/sh\n/bin/sleep 30 &\nprintf '%s\\n' \"$!\" > \"$1\"\nwait\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIsLibreOfficeCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"soffice", true}, {"libreoffice", true},
		{"/Applications/LibreOffice.app/Contents/MacOS/soffice", true},
		{"/usr/bin/soffice", true}, {"pdftoppm", false}, {"soffice-helper", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLibreOfficeCommand(tc.name); got != tc.want {
				t.Errorf("isLibreOfficeCommand(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestIsRasterizerCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"pdftoppm", true}, {"/usr/bin/magick", true}, {"convert.exe", true},
		{"soffice", false}, {"magick-helper", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRasterizerCommand(tc.name); got != tc.want {
				t.Errorf("isRasterizerCommand(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func readOfficeWorkerPID(t *testing.T, path string) int {
	t.Helper()
	// Startup has its own bounded budget. Conversion cancellation is armed
	// only after this handshake; it must never expire before a worker exists.
	deadline := time.Now().Add(30 * time.Second)
	if testDeadline, ok := t.Deadline(); ok && testDeadline.Before(deadline) {
		deadline = testDeadline.Add(-time.Second)
	}
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			value := strings.TrimSpace(string(data))
			if value != "" {
				pid, err := parseFixtureWorkerPID(value)
				if err != nil {
					t.Fatalf("parse worker PID: %v", err)
				}
				return pid
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fake conversion process did not report worker readiness at %s", path)
	return 0
}

func parseFixtureWorkerPID(value string) (int, error) {
	pid, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("worker PID %q: %w", value, err)
	}
	if pid <= 0 {
		return 0, fmt.Errorf("worker PID must be positive: %d", pid)
	}
	return pid, nil
}

func TestFixtureWorkerPIDRejectsProcessGroups(t *testing.T) {
	for _, value := range []string{"0", "-1", "", "not-a-pid"} {
		if _, err := parseFixtureWorkerPID(value); err == nil {
			t.Errorf("accepted unsafe fixture PID %q", value)
		}
	}
	if pid, err := parseFixtureWorkerPID("12345"); err != nil || pid != 12345 {
		t.Fatalf("positive fixture PID = %d, %v", pid, err)
	}
}

// A controllable deadline keeps process startup separate from the condition
// under test. Only the test timer cancels it; its Err models that deadline.
type fixtureDeadlineContext struct{ context.Context }

func (c fixtureDeadlineContext) Err() error {
	if c.Context.Err() != nil {
		return context.DeadlineExceeded
	}
	return nil
}

func runWorkerUntilDeadline(t *testing.T, executable, workerFile string, wantTimeout time.Duration) int {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	runner := &RealCommandRunner{Stdout: io.Discard, Stderr: io.Discard,
		timeoutContext: func(timeout time.Duration) (context.Context, context.CancelFunc) {
			if timeout != wantTimeout {
				t.Errorf("selected timeout=%s, want %s", timeout, wantTimeout)
			}
			return fixtureDeadlineContext{ctx}, cancel
		}}
	done := make(chan error, 1)
	go func() { done <- runner.Run(executable, workerFile) }()
	pid := readOfficeWorkerPID(t, workerFile)
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("worker exited before deadline: %v", err)
	}
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("run error=%v, want deadline exceeded", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("conversion did not return after deadline")
	}
	return pid
}

func assertWorkerGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if officeWorkerExited(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL) // Test-owned worker only.
	t.Fatalf("fake soffice worker %d survived conversion cancellation", pid)
}

func officeWorkerExited(pid int) bool {
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return true
	}
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return false
	}
	fields := strings.Fields(string(data[end+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}

func TestRealCommandRunnerLibreOfficeTimeoutKillsWorker(t *testing.T) {
	office := fakeSofficeWithWorker(t)
	workerFile := filepath.Join(t.TempDir(), "worker.pid")
	assertWorkerGone(t, runWorkerUntilDeadline(t, office, workerFile, pptx2jpgLibreOfficeTimeout))
}

func TestRealCommandRunnerLibreOfficeCallerDeath(t *testing.T) {
	if os.Getenv("PPTX2JPG_GUARD_HELPER") == "1" {
		runner := &RealCommandRunner{Stdout: io.Discard, Stderr: io.Discard}
		_ = runner.Run(os.Getenv("PPTX2JPG_GUARD_OFFICE"), os.Getenv("PPTX2JPG_GUARD_WORKER"))
		return
	}
	office := fakeSofficeWithWorker(t)
	workerFile := filepath.Join(t.TempDir(), "worker.pid")
	unrelated := exec.Command("/bin/sleep", "60")
	unrelated.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() }()

	helper := exec.Command(os.Args[0], "-test.run=^TestRealCommandRunnerLibreOfficeCallerDeath$") //nolint:gosec // re-executes only this fixed test binary and test name
	helper.Env = append(os.Environ(), "PPTX2JPG_GUARD_HELPER=1", "PPTX2JPG_GUARD_OFFICE="+office, "PPTX2JPG_GUARD_WORKER="+workerFile)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = helper.Process.Kill(); _ = helper.Wait() }()
	workerPID := readOfficeWorkerPID(t, workerFile)
	if err := syscall.Kill(workerPID, 0); err != nil {
		t.Fatalf("fake worker exited before caller death: %v", err)
	}
	if err := helper.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := helper.Wait(); err == nil {
		t.Fatal("helper unexpectedly exited cleanly after SIGKILL")
	}
	assertWorkerGone(t, workerPID)
	if err := syscall.Kill(unrelated.Process.Pid, 0); err != nil {
		t.Fatalf("unrelated process was affected: %v", err)
	}
}

func TestRealCommandRunnerRasterizerTimeoutKillsWorker(t *testing.T) {
	for _, binary := range []string{"pdftoppm", "magick", "convert"} {
		t.Run(binary, func(t *testing.T) {
			rasterizer := filepath.Join(t.TempDir(), binary)
			content := "#!/bin/sh\n/bin/sleep 30 &\nprintf '%s\\n' \"$!\" > \"$1\"\nwait\n"
			if err := os.WriteFile(rasterizer, []byte(content), 0o755); err != nil {
				t.Fatal(err)
			}
			workerFile := filepath.Join(t.TempDir(), "worker.pid")
			assertWorkerGone(t, runWorkerUntilDeadline(t, rasterizer, workerFile, pptx2jpgRasterizerTimeout))
		})
	}
}
