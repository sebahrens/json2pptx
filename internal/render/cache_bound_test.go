package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// go-slide-creator-dpys: the render cache had no bound. InvalidateCache had no
// callers anywhere in the repo, nothing capped size or age, and one review
// session left 418 files / 13MB behind.

// withTempCache points the cache at a scratch dir for the duration of a test.
func withTempCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	root := cacheDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("make cache dir: %v", err)
	}
	return root
}

// writeCacheFile puts a file of n bytes in the cache, aged by the given amount.
func writeCacheFile(t *testing.T, root, name string, n int, age time.Duration) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return path
}

func TestSweepCacheEvictsByAge(t *testing.T) {
	root := withTempCache(t)
	old := writeCacheFile(t, root, "artifacts/old.png", 1000, 48*time.Hour)
	fresh := writeCacheFile(t, root, "artifacts/fresh.png", 1000, time.Minute)

	reclaimed, err := SweepCache(24*time.Hour, 0)
	if err != nil {
		t.Fatalf("SweepCache: %v", err)
	}
	if reclaimed != 1000 {
		t.Errorf("reclaimed %d bytes, want 1000", reclaimed)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("a 48h-old artifact survived a 24h age bound")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a fresh artifact was evicted: %v", err)
	}
}

func TestSweepCacheEvictsOldestUntilUnderSize(t *testing.T) {
	root := withTempCache(t)
	// All within the age bound; only the size bound should bite.
	oldest := writeCacheFile(t, root, "a.png", 600, 3*time.Hour)
	middle := writeCacheFile(t, root, "b.png", 600, 2*time.Hour)
	newest := writeCacheFile(t, root, "c.png", 600, time.Hour)

	if _, err := SweepCache(24*time.Hour, 1500); err != nil {
		t.Fatalf("SweepCache: %v", err)
	}
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Error("the oldest entry survived an over-size cache")
	}
	for _, keep := range []string{middle, newest} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s was evicted though the cache already fit: %v", keep, err)
		}
	}
	if got := CacheBytes(); got > 1500 {
		t.Errorf("cache is %d bytes, still over the 1500 bound", got)
	}
}

// A cache already inside both bounds must be left completely alone: the sweep
// runs on every render, so it cannot be destructive.
func TestSweepCacheLeavesAHealthyCacheAlone(t *testing.T) {
	root := withTempCache(t)
	keep := writeCacheFile(t, root, "artifacts/keep.png", 100, time.Minute)
	reclaimed, err := SweepCache(24*time.Hour, 500<<20)
	if err != nil {
		t.Fatalf("SweepCache: %v", err)
	}
	if reclaimed != 0 {
		t.Errorf("reclaimed %d bytes from a healthy cache", reclaimed)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("a healthy artifact was removed: %v", err)
	}
}

// A missing cache directory is not an error — there is nothing to sweep.
func TestSweepCacheWithNoCacheDir(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	if _, err := SweepCache(time.Hour, 1); err != nil {
		t.Errorf("sweeping an absent cache failed: %v", err)
	}
	if got := CacheBytes(); got != 0 {
		t.Errorf("CacheBytes on an absent cache = %d, want 0", got)
	}
}

func TestCacheBytesCountsEverything(t *testing.T) {
	root := withTempCache(t)
	writeCacheFile(t, root, "artifacts/a.png", 300, time.Minute)
	writeCacheFile(t, root, "deadbeef-d150/b.png", 700, time.Minute)
	if got := CacheBytes(); got != 1000 {
		t.Errorf("CacheBytes = %d, want 1000 across both subdirectories", got)
	}
}

// Directories emptied by a sweep are removed, so an aged-out cache does not
// leave a tree of empty per-deck folders behind.
func TestSweepPrunesEmptyDirs(t *testing.T) {
	root := withTempCache(t)
	writeCacheFile(t, root, "deadbeef-d150/slide.png", 10, 48*time.Hour)
	if _, err := SweepCache(24*time.Hour, 0); err != nil {
		t.Fatalf("SweepCache: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "deadbeef-d150")); !os.IsNotExist(err) {
		t.Error("an emptied cache subdirectory was left behind")
	}
}

// The policy string must describe the bound that actually exists. It used to
// promise removal by InvalidateCache, which nothing called.
func TestArtifactCleanupPolicyDescribesTheRealBound(t *testing.T) {
	for _, want := range []string{"24h", "500 MiB", "render_cache_bytes"} {
		if !strings.Contains(ArtifactCleanupPolicy, want) {
			t.Errorf("cleanup policy does not mention %q: %s", want, ArtifactCleanupPolicy)
		}
	}
	if strings.Contains(ArtifactCleanupPolicy, "InvalidateCache") {
		t.Error("cleanup policy still points at InvalidateCache, which no render path calls")
	}
}
