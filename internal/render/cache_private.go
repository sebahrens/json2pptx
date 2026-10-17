package render

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var fallbackCache struct {
	sync.Once
	path string
}

// cacheDir never uses the predictable shared /tmp cache. The explicit override
// is useful for isolated servers and tests; it has the same ownership checks.
func cacheDir() string {
	if override := os.Getenv("JSON2PPTX_RENDER_CACHE_DIR"); override != "" {
		return override
	}
	if root, err := os.UserCacheDir(); err == nil {
		return filepath.Join(root, "json2pptx-render")
	}
	fallbackCache.Do(func() { fallbackCache.path, _ = os.MkdirTemp("", "json2pptx-render-*") })
	return fallbackCache.path
}

func privateCacheDirectory(path string, create bool) error {
	if path == "" {
		return fmt.Errorf("private render cache unavailable")
	}
	if create {
		if err := os.MkdirAll(path, 0700); err != nil {
			return err
		}
	}
	info, err := os.Lstat(path) //nolint:gosec // Lstat validates cache ownership/type without following links; content is read only after this check.
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !cachePrivatePermissions(info, true) || !cacheOwnedByCurrentUser(info) {
		return fmt.Errorf("render cache directory must be owned by the current user, private (0700), and not a symlink")
	}
	return nil
}

func trustedCacheFile(path string) bool {
	info, err := os.Lstat(path) //nolint:gosec // Lstat validates cache ownership/type without following links; content is read only after this check.
	return err == nil && info.Mode().IsRegular() && cachePrivatePermissions(info, false) && cacheOwnedByCurrentUser(info)
}
