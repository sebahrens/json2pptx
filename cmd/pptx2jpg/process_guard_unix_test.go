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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			value := strings.TrimSpace(string(data))
			if value != "" {
				pid, err := strconv.Atoi(value)
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
	t.Fatal("fake soffice did not start a worker")
	return 0
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
	prior := pptx2jpgLibreOfficeTimeout
	pptx2jpgLibreOfficeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { pptx2jpgLibreOfficeTimeout = prior })
	runner := &RealCommandRunner{Stdout: io.Discard, Stderr: io.Discard}
	err := runner.Run(office, workerFile)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run error = %v, want deadline exceeded", err)
	}
	assertWorkerGone(t, readOfficeWorkerPID(t, workerFile))
}

func TestRealCommandRunnerLibreOfficeCallerDeath(t *testing.T) {
	if os.Getenv("PPTX2JPG_GUARD_HELPER") == "1" {
		runner := &RealCommandRunner{Stdout: io.Discard, Stderr: io.Discard}
		_ = runner.Run(os.Getenv("PPTX2JPG_GUARD_OFFICE"), os.Getenv("PPTX2JPG_GUARD_WORKER"))
		return
	}
	office := fakeSofficeWithWorker(t)
	workerFile := filepath.Join(t.TempDir(), "worker.pid")
	unrelated := exec.Command("/bin/sleep", "10")
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
	prior := pptx2jpgRasterizerTimeout
	pptx2jpgRasterizerTimeout = 100 * time.Millisecond
	t.Cleanup(func() { pptx2jpgRasterizerTimeout = prior })
	for _, binary := range []string{"pdftoppm", "magick", "convert"} {
		t.Run(binary, func(t *testing.T) {
			rasterizer := filepath.Join(t.TempDir(), binary)
			content := "#!/bin/sh\n/bin/sleep 30 &\nprintf '%s\\n' \"$!\" > \"$1\"\nwait\n"
			if err := os.WriteFile(rasterizer, []byte(content), 0o755); err != nil {
				t.Fatal(err)
			}
			workerFile := filepath.Join(t.TempDir(), "worker.pid")
			runner := &RealCommandRunner{Stdout: io.Discard, Stderr: io.Discard}
			err := runner.Run(rasterizer, workerFile)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("run error = %v, want deadline exceeded", err)
			}
			assertWorkerGone(t, readOfficeWorkerPID(t, workerFile))
		})
	}
}
