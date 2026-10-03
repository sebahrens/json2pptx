//go:build unix

package render

import (
	"errors"
	"syscall"
)

// processAlive reports whether a process with this pid exists. Signal 0
// probes without delivering anything; EPERM means the process exists but
// belongs to someone else.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
