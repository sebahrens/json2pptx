//go:build unix

package render

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group and installs a Cancel
// hook that kills the entire group on context cancellation. LibreOffice forks a
// soffice.bin worker; killing only the direct child (exec.CommandContext's
// default) would orphan that worker and leave it holding the document. Signaling
// the negative PID delivers SIGKILL to every process in the group.
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
