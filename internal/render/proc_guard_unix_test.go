//go:build unix

package render

import (
	"context"
	"errors"
	"fmt"
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

// TestLibreOfficeGuardSurvivesCallerDeath exercises the condition that a
// context-cancel unit test cannot: SIGKILL prevents the caller's defer and
// CommandContext.Cancel from running. The fake converter execs sleep, so its
// PID remains stable and is safe to inspect. An unrelated process must live.
func TestLibreOfficeGuardSurvivesCallerDeath(t *testing.T) {
	if os.Getenv("JSON2PPTX_GUARD_HELPER") == "1" {
		pidFile := os.Getenv("JSON2PPTX_GUARD_PID_FILE")
		workerFile := os.Getenv("JSON2PPTX_GUARD_WORKER_FILE")
		_, _, _ = runBounded(context.Background(), toolLibreOffice, pidFile, 30*time.Second,
			"/bin/sh", "-c", `printf '%s\n' "$$" > "$1"; /bin/sleep 30 & printf '%s\n' "$!" > "$2"; wait`, "guarded-converter", pidFile, workerFile)
		return
	}

	unrelated := exec.Command("/bin/sleep", "10")
	unrelated.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := unrelated.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unrelated.Process.Kill(); _ = unrelated.Wait() }()

	pidFile := filepath.Join(t.TempDir(), "converter.pid")
	workerFile := filepath.Join(filepath.Dir(pidFile), "worker.pid")
	helper := exec.Command(os.Args[0], "-test.run=^TestLibreOfficeGuardSurvivesCallerDeath$") //nolint:gosec // re-executes only this fixed test binary and test name
	helper.Env = append(os.Environ(), "JSON2PPTX_GUARD_HELPER=1", "JSON2PPTX_GUARD_PID_FILE="+pidFile, "JSON2PPTX_GUARD_WORKER_FILE="+workerFile)
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = helper.Process.Kill(); _ = helper.Wait() }()

	var converterPID, workerPID int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		converterPID = readTestPID(t, pidFile)
		workerPID = readTestPID(t, workerFile)
		if converterPID > 0 && workerPID > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if converterPID <= 0 || workerPID <= 0 {
		t.Fatal("fake converter and worker did not start")
	}
	for _, pid := range []int{converterPID, workerPID} {
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("fake converter process %d exited before caller death: %v", pid, err)
		}
	}
	defer func() {
		if groupID, err := syscall.Getpgid(converterPID); err == nil {
			_ = syscall.Kill(-groupID, syscall.SIGKILL)
		}
	}()
	if err := helper.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := helper.Wait(); err == nil {
		t.Fatal("SIGKILL unexpectedly allowed helper to exit cleanly")
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processExited(converterPID) && processExited(workerPID) {
			if err := syscall.Kill(unrelated.Process.Pid, 0); err != nil {
				t.Fatalf("guard affected unrelated process %d: %v", unrelated.Process.Pid, err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("converter PIDs %d/%d survived caller death; guard did not kill their process group", converterPID, workerPID)
}

func TestLibreOfficeGuardCleansWorkerAfterLauncherExits(t *testing.T) {
	workerFile := filepath.Join(t.TempDir(), "worker.pid")
	_, _, err := runBounded(context.Background(), toolLibreOffice, workerFile, 5*time.Second,
		"/bin/sh", "-c", `/bin/sleep 30 </dev/null >/dev/null 2>&1 & printf '%s\n' "$!" > "$1"`, "guarded-converter", workerFile)
	if err != nil {
		t.Fatalf("launcher should complete cleanly: %v", err)
	}
	workerPID := readTestPID(t, workerFile)
	if workerPID <= 0 {
		t.Fatal("launcher did not record its worker PID")
	}
	defer func() {
		if groupID, err := syscall.Getpgid(workerPID); err == nil {
			_ = syscall.Kill(-groupID, syscall.SIGKILL)
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if processExited(workerPID) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker PID %d survived successful launcher exit", workerPID)
}

// A SIGKILLed orphan may remain as a non-running zombie until the container's
// PID 1 reaps it. kill(pid, 0) still succeeds for that state, even though the
// converter cannot execute or hold LibreOffice resources anymore.
func processExited(pid int) bool {
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
	// The command name can contain spaces and parentheses; state follows its
	// LAST closing parenthesis in /proc/<pid>/stat.
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return false
	}
	fields := strings.Fields(string(data[end+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}

func readTestPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return 0 // The shell may have created the file but not written the PID yet.
	}
	pid, err := strconv.Atoi(value)
	if err != nil {
		t.Fatalf("parse PID from %s: %v", path, err)
	}
	return pid
}
