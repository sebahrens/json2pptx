//go:build unix

package render

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// RunGuardedLibreOffice keeps a tiny independent process alive while a
// conversion runs. Its stdin is held open only by the caller. If the caller is
// killed (so no Go defer or context callback can run), EOF makes the guard
// kill precisely the process group it created for this conversion. Normal
// completion closes the pipe too: if the launcher exited before a worker, that
// worker is still cleaned up. The guard is the group leader, reserving its ID
// until cleanup.
// No process discovery or broad "killall soffice" is involved.
func RunGuardedLibreOffice(cmd *exec.Cmd) error {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create LibreOffice process guard pipe: %w", err)
	}
	defer readEnd.Close()
	defer writeEnd.Close()

	guard := exec.Command("/bin/sh", "-c", `
IFS= read -r _ || :
kill -KILL -- "-$$" 2>/dev/null || :
`) //nolint:gosec // constant shell program; kills only the guard's own process group
	guard.Stdin = readEnd
	guard.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := guard.Start(); err != nil {
		return fmt.Errorf("start LibreOffice process guard: %w", err)
	}
	groupID := guard.Process.Pid
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: groupID}
	cmd.Cancel = func() error { return syscall.Kill(-groupID, syscall.SIGKILL) }
	runErr := cmd.Run()
	_ = writeEnd.Close()
	guardErr := guard.Wait()
	if runErr != nil {
		return runErr
	}
	if exitErr, ok := guardErr.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGKILL {
			return nil
		}
	}
	return fmt.Errorf("LibreOffice process guard did not clean up its group: %v", guardErr)
}

// setProcessGroup keeps the existing group-cancellation behavior for other
// render subprocesses (ImageMagick). LibreOffice uses RunGuardedLibreOffice so
// cleanup also works when the caller process dies before Cancel can run.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			// Fall back to killing just the direct child if the group send fails
			// (e.g. the process exited between the deadline and this call).
			return cmd.Process.Kill()
		}
		return nil
	}
}
