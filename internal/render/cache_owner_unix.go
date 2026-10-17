//go:build !windows

package render

import (
	"os"
	"syscall"
)

func cacheOwnedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid())
}

func cachePrivatePermissions(info os.FileInfo, directory bool) bool {
	mask := os.FileMode(0022)
	if directory {
		mask = 0077
	}
	return info.Mode().Perm()&mask == 0
}
