package resource

import (
	"os"
	"path/filepath"
	"sync"
)

// cache provides a session-scoped, thread-safe download cache.
// Files are stored in a temp directory and tracked by a (kind, URL) key to
// avoid duplicates (see cacheKey).
type cache struct {
	mu      sync.Mutex
	dir     string            // temp directory for cached files
	entries map[string]string // cache key -> local file path
}

// newCache creates a cache backed by a new temporary directory.
// The caller must call cleanup() when done.
func newCache() (*cache, error) {
	dir, err := os.MkdirTemp("", "go-slide-creator-resources-*")
	if err != nil {
		return nil, err
	}
	return &cache{
		dir:     dir,
		entries: make(map[string]string),
	}, nil
}

// get returns the cached local path for a key, or "" if not cached.
func (c *cache) get(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entries[key]
}

// put stores a downloaded file in the cache and returns its local path.
// The data is written to a file named by the given filename.
func (c *cache) put(key, filename string, data []byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check under lock
	if p, ok := c.entries[key]; ok {
		return p, nil
	}

	localPath := filepath.Join(c.dir, filename)
	if err := os.WriteFile(localPath, data, 0600); err != nil {
		return "", err
	}
	c.entries[key] = localPath
	return localPath, nil
}

// cleanup removes the temp directory and all cached files.
func (c *cache) cleanup() {
	_ = os.RemoveAll(c.dir)
}
