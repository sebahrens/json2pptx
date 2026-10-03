package layoutpreview

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writePreviewSet creates <cache>/<template>/<identity>/ with one PNG of the
// given size and a completion marker last touched at lastUse.
func writePreviewSet(t *testing.T, cache, template, identity string, size int, lastUse time.Time) string {
	t.Helper()
	dir := filepath.Join(cache, template, identity)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	png := filepath.Join(dir, "slideLayout1.png")
	if err := os.WriteFile(png, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, ".done")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The PNG is as old as the set's creation; only the marker tracks use.
	created := lastUse.Add(-time.Hour)
	if err := os.Chtimes(png, created, created); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(marker, lastUse, lastUse); err != nil {
		t.Fatal(err)
	}
	return dir
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Sets unused past the age bound are removed, along with the template
// directory they leave empty (go-slide-creator-12dkn).
func TestSweepCacheAge(t *testing.T) {
	cache := t.TempDir()
	now := time.Now()
	stale := writePreviewSet(t, cache, "old-template", "aaa", 100, now.Add(-40*24*time.Hour))
	fresh := writePreviewSet(t, cache, "midnight-blue", "bbb", 100, now.Add(-time.Hour))

	reclaimed, err := SweepCache(cache, CacheMaxAge, CacheMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed != 100 {
		t.Errorf("reclaimed %d bytes, want 100", reclaimed)
	}
	if exists(stale) {
		t.Error("the set unused for 40 days should have been removed")
	}
	if exists(filepath.Dir(stale)) {
		t.Error("the emptied template directory should have been removed")
	}
	if !exists(fresh) {
		t.Error("the recently used set should have been kept")
	}
}

// Over the size bound the least recently used sets go first, and a set named
// in keep survives even when it is the oldest.
func TestSweepCacheSizeEvictsLeastRecentlyUsed(t *testing.T) {
	cache := t.TempDir()
	now := time.Now()
	oldest := writePreviewSet(t, cache, "t", "oldest", 400, now.Add(-5*time.Hour))
	middle := writePreviewSet(t, cache, "t", "middle", 400, now.Add(-3*time.Hour))
	newest := writePreviewSet(t, cache, "t", "newest", 400, now.Add(-time.Hour))

	if _, err := SweepCache(cache, 0, 900); err != nil {
		t.Fatal(err)
	}
	if exists(oldest) || !exists(middle) || !exists(newest) {
		t.Errorf("want only the oldest set evicted: oldest=%v middle=%v newest=%v", exists(oldest), exists(middle), exists(newest))
	}

	// keep protects a set regardless of its age.
	cache2 := t.TempDir()
	keepOld := writePreviewSet(t, cache2, "t", "keep", 400, now.Add(-9*time.Hour))
	other := writePreviewSet(t, cache2, "t", "other", 400, now.Add(-time.Hour))
	if _, err := SweepCache(cache2, time.Hour*2, 100, keepOld); err != nil {
		t.Fatal(err)
	}
	if !exists(keepOld) {
		t.Error("a set passed in keep must never be removed")
	}
	if exists(other) {
		t.Error("the unprotected set should have been evicted to meet the size bound")
	}
}

func TestSweepCacheMissingDir(t *testing.T) {
	reclaimed, err := SweepCache(filepath.Join(t.TempDir(), "absent"), CacheMaxAge, CacheMaxBytes)
	if err != nil || reclaimed != 0 {
		t.Errorf("missing dir: reclaimed=%d err=%v, want 0, nil", reclaimed, err)
	}
}
