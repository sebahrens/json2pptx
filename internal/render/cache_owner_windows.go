//go:build windows

package render

import "os"

// Windows access control is inherited from the per-user cache directory;
// symlink and regular-file checks remain shared across platforms.
func cacheOwnedByCurrentUser(_ os.FileInfo) bool { return true }

func cachePrivatePermissions(_ os.FileInfo, _ bool) bool { return true }
