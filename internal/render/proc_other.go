//go:build !unix

package render

import "os/exec"

// setProcessGroup is a no-op on platforms without POSIX process groups;
// exec.CommandContext's default cancel (SIGKILL to the direct child) still
// applies, so the subprocess is still bounded by the deadline.
func setProcessGroup(cmd *exec.Cmd) {}
