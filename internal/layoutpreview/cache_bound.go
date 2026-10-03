package layoutpreview

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Layout-preview cache bounds (go-slide-creator-12dkn).
//
// Every distinct template, generator recipe, renderer version and resolution
// gets its own preview set under <cache>/<template>/<identity>/, and nothing
// ever removed one: an upgrade, a re-themed template or a temporary template
// copy each left a set behind for good. One machine held 28,322 files
// (311 MB).
//
// The cache is now swept whenever a new set is written: sets not used for
// CacheMaxAge go, and if the rest is still over CacheMaxBytes the least
// recently used go until it fits. A cache hit refreshes its set's marker, so
// "age" means time since last use, not since creation.

const (
	// CacheMaxAge is how long a preview set survives without being used.
	CacheMaxAge = 30 * 24 * time.Hour
	// CacheMaxBytes is the ceiling for everything under the cache directory.
	CacheMaxBytes = 256 << 20 // 256 MiB
)

// previewSet is one evictable unit: a <template>/<identity> directory, or the
// loose files of a legacy template-only cache.
type previewSet struct {
	paths   []string // what to remove: one directory, or the loose files
	size    int64
	lastUse time.Time
}

// SweepCache removes preview sets under cacheDir that have not been used for
// maxAge, then evicts least-recently-used sets until the total is within
// maxBytes. Sets whose directory is listed in keep are never removed (the set
// a caller has just written or is about to read). It returns the bytes
// reclaimed. A missing cache directory is not an error.
//
// maxAge <= 0 skips the age pass; maxBytes <= 0 skips the size pass.
func SweepCache(cacheDir string, maxAge time.Duration, maxBytes int64, keep ...string) (int64, error) {
	templates, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	kept := map[string]bool{}
	for _, k := range keep {
		kept[filepath.Clean(k)] = true
	}

	var sets []previewSet
	var templateDirs []string
	for _, t := range templates {
		if !t.IsDir() {
			continue
		}
		templateDir := filepath.Join(cacheDir, t.Name())
		templateDirs = append(templateDirs, templateDir)
		sets = append(sets, templatePreviewSets(templateDir, kept)...)
	}

	reclaimed := evictPreviewSets(sets, maxAge, maxBytes)

	// A template directory emptied by the sweep is removed too (os.Remove
	// refuses a directory that still holds a set).
	for _, dir := range templateDirs {
		_ = os.Remove(dir)
	}
	return reclaimed, nil
}

// templatePreviewSets lists the evictable sets under one template directory:
// each identity sub-directory, plus the loose files of a legacy template-only
// cache as one set. Directories named in kept are skipped.
func templatePreviewSets(templateDir string, kept map[string]bool) []previewSet {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil
	}
	var sets []previewSet
	var legacy previewSet
	for _, e := range entries {
		path := filepath.Join(templateDir, e.Name())
		if e.IsDir() {
			if !kept[filepath.Clean(path)] {
				set := measurePreviewSet(path)
				set.paths = []string{path}
				sets = append(sets, set)
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		legacy.paths = append(legacy.paths, path)
		legacy.size += info.Size()
		if info.ModTime().After(legacy.lastUse) {
			legacy.lastUse = info.ModTime()
		}
	}
	if len(legacy.paths) > 0 && !kept[filepath.Clean(templateDir)] {
		sets = append(sets, legacy)
	}
	return sets
}

// evictPreviewSets applies the age bound, then the size bound (least recently
// used first), and returns the bytes reclaimed.
func evictPreviewSets(sets []previewSet, maxAge time.Duration, maxBytes int64) int64 {
	var reclaimed int64
	remove := func(s previewSet) {
		ok := true
		for _, p := range s.paths {
			if os.RemoveAll(p) != nil {
				ok = false
			}
		}
		if ok {
			reclaimed += s.size
		}
	}

	live := sets
	if maxAge > 0 {
		cutoff := time.Now().Add(-maxAge)
		live = make([]previewSet, 0, len(sets))
		for _, s := range sets {
			if s.lastUse.Before(cutoff) {
				remove(s)
				continue
			}
			live = append(live, s)
		}
	}
	if maxBytes <= 0 {
		return reclaimed
	}

	var total int64
	for _, s := range live {
		total += s.size
	}
	sort.Slice(live, func(i, j int) bool { return live[i].lastUse.Before(live[j].lastUse) })
	for _, s := range live {
		if total <= maxBytes {
			break
		}
		remove(s)
		total -= s.size
	}
	return reclaimed
}

// measurePreviewSet sizes one <template>/<identity> directory. Its last use
// is the completion marker's mtime (refreshed on every cache hit), or the
// newest file when the set was never completed.
func measurePreviewSet(dir string) previewSet {
	var set previewSet
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		set.size += info.Size()
		if info.ModTime().After(set.lastUse) {
			set.lastUse = info.ModTime()
		}
		return nil
	})
	if info, err := os.Stat(filepath.Join(dir, ".done")); err == nil {
		set.lastUse = info.ModTime()
	}
	return set
}
