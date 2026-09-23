//go:build integration && unix

package main

import (
	"os/exec"
	"syscall"
)

// LibreOffice's Homebrew script and Linux launchers may leave a soffice child
// behind when only the direct command is killed on timeout.
func configureHeadlessProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}
