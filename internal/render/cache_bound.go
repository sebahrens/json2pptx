package render

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Render-cache bounds (go-slide-creator-dpys).
//
// Every render_* call writes content-addressed PNGs under the render cache, and
// the documented cleanup story was "removed by InvalidateCache or OS temp
// cleanup". InvalidateCache had no callers anywhere in the repo, no size or age
// bound existed, and no tool exposed it — one review session left 418 files and
// 13MB behind. On a long-lived MCP server the cache only grew.
//
// The cache is now swept on render: anything past its age is removed, and if
// the remainder is still over the size bound the oldest entries go until it
// fits. The sweep is stat-based and throttled, so it costs a directory walk at
// most once an interval rather than once per render.

const (
	// CacheMaxAge is how long a cached artifact survives without being
	// rewritten. Content-addressed writes refresh the mtime, so anything still
	// in use by a repeating render keeps resetting its own clock.
	CacheMaxAge = 24 * time.Hour
	// CacheMaxBytes is the ceiling for everything under the cache directory.
	// Past it, the oldest entries are evicted until the total fits.
	CacheMaxBytes = 500 << 20 // 500 MiB
	// cacheSweepInterval throttles the walk so a burst of renders pays for one
	// sweep, not one per call.
	cacheSweepInterval = 5 * time.Minute
)

var (
	sweepMu       sync.Mutex
	lastSweepTime time.Time
)

// maybeSweepCache runs SweepCache at most once per cacheSweepInterval. Called
// from the render entry points so the bound is enforced without the caller
// having to think about it.
func maybeSweepCache() {
	sweepMu.Lock()
	if !lastSweepTime.IsZero() && time.Since(lastSweepTime) < cacheSweepInterval {
		sweepMu.Unlock()
		return
	}
	lastSweepTime = time.Now()
	sweepMu.Unlock()
	_, _ = SweepCache(CacheMaxAge, CacheMaxBytes)
}

// cacheEntry is one file under the cache directory.
type cacheEntry struct {
	path    string
	size    int64
	modTime time.Time
}

// SweepCache removes cached artifacts older than maxAge, then evicts
// oldest-first until the total is within maxBytes. It returns the bytes
// reclaimed. A missing cache directory is not an error — there is nothing to
// sweep.
//
// maxAge <= 0 skips the age pass; maxBytes <= 0 skips the size pass.
func SweepCache(maxAge time.Duration, maxBytes int64) (int64, error) {
	entries, err := scanCache()
	if err != nil {
		return 0, err
	}

	var reclaimed int64
	kept := entries[:0]
	if maxAge > 0 {
		cutoff := time.Now().Add(-maxAge)
		for _, e := range entries {
			if e.modTime.Before(cutoff) {
				if os.Remove(e.path) == nil {
					reclaimed += e.size
				}
				continue
			}
			kept = append(kept, e)
		}
	} else {
		kept = entries
	}

	if maxBytes > 0 {
		var total int64
		for _, e := range kept {
			total += e.size
		}
		if total > maxBytes {
			// Oldest first: a cached render nobody has asked for in days is the
			// cheapest thing to lose, and re-rendering it is one LibreOffice
			// call.
			sort.Slice(kept, func(i, j int) bool { return kept[i].modTime.Before(kept[j].modTime) })
			for _, e := range kept {
				if total <= maxBytes {
					break
				}
				if os.Remove(e.path) == nil {
					total -= e.size
					reclaimed += e.size
				}
			}
		}
	}

	pruneEmptyCacheDirs()
	return reclaimed, nil
}

// CacheBytes reports the total size of the render cache on disk, so
// get_capabilities can tell an agent what it is using. A missing cache
// directory reports 0.
func CacheBytes() int64 {
	entries, err := scanCache()
	if err != nil {
		return 0
	}
	var total int64
	for _, e := range entries {
		total += e.size
	}
	return total
}

// scanCache lists every regular file under the cache directory.
func scanCache() ([]cacheEntry, error) {
	root := cacheDir()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	var out []cacheEntry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A file removed by another process mid-walk is not our problem.
			return nil //nolint:nilerr // best-effort sweep
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // best-effort sweep
		}
		out = append(out, cacheEntry{path: path, size: info.Size(), modTime: info.ModTime()})
		return nil
	})
	return out, err
}

// pruneEmptyCacheDirs removes directories left empty by a sweep, so a cache
// that has aged out entirely does not leave a tree of empty per-deck folders.
func pruneEmptyCacheDirs() {
	root := cacheDir()
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil //nolint:nilerr // best-effort sweep
		}
		dirs = append(dirs, path)
		return nil
	})
	// Deepest first, so a parent emptied by its children is also removed.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}
}
