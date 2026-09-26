package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutputCleaner_cleanExpiredFiles(t *testing.T) {
	dir := t.TempDir()

	// Create an "old" file and a "new" file
	// Names follow generateUniqueFilename: the cleaner only touches those.
	oldFile := filepath.Join(dir, "0123456789abcdef0123456789abcdef.pptx")
	newFile := filepath.Join(dir, "fedcba9876543210fedcba9876543210.pptx")

	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	// Backdate the old file
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldFile, past, past); err != nil {
		t.Fatal(err)
	}

	cleaner := NewOutputCleaner(OutputCleanerConfig{
		OutputDir: dir,
		Retention: 1 * time.Hour,
		Interval:  1 * time.Minute,
	})

	cleaned, err := cleaner.CleanupNow()
	if err != nil {
		t.Fatalf("CleanupNow returned error: %v", err)
	}
	if cleaned != 1 {
		t.Errorf("expected 1 file cleaned, got %d", cleaned)
	}

	// Old file should be gone
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("expected old file to be removed")
	}

	// New file should still exist
	if _, err := os.Stat(newFile); err != nil {
		t.Error("expected new file to still exist")
	}
}

// go-slide-creator-s1uvj.22: the cleaner deleted every old non-directory
// file in OUTPUT_DIR, including files the server never generated (an operator
// pointing OUTPUT_DIR at a shared directory lost unrelated files).
func TestOutputCleaner_onlyRemovesGeneratedDownloads(t *testing.T) {
	dir := t.TempDir()
	generated := filepath.Join(dir, "0123456789abcdef0123456789abcdef.pptx")
	orphanTemp := filepath.Join(dir, ".0123456789abcdef0123456789abcdef.pptx.123456.tmp")
	unrelated := []string{
		filepath.Join(dir, "report.pptx"),
		filepath.Join(dir, "config.yaml"),
		// Use a different value too: a case-only change aliases generated on
		// case-insensitive filesystems and cannot be an unrelated file.
		filepath.Join(dir, "ABCDEF0123456789ABCDEF0123456789.pptx"),
		filepath.Join(dir, "0123456789abcdef0123456789abcdef.pptx.bak"),
	}
	past := time.Now().Add(-2 * time.Hour)
	for _, p := range append([]string{generated, orphanTemp}, unrelated...) {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, past, past); err != nil {
			t.Fatal(err)
		}
	}

	cleaner := NewOutputCleaner(OutputCleanerConfig{OutputDir: dir, Retention: time.Hour})
	cleaned, err := cleaner.CleanupNow()
	if err != nil {
		t.Fatalf("CleanupNow: %v", err)
	}
	if cleaned != 2 {
		t.Errorf("cleaned = %d, want 2", cleaned)
	}
	for _, p := range []string{generated, orphanTemp} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expired generated file %s was not removed", filepath.Base(p))
		}
	}
	for _, p := range unrelated {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("unrelated file %s was removed", filepath.Base(p))
		}
	}
}

// go-slide-creator-s1uvj.22: a zero/negative retention used to delete every
// file immediately while convert advertised a one-hour expiry. The cleaner now
// falls back to DefaultFileRetention, matching the advertised expires_at.
func TestOutputCleaner_nonPositiveRetentionUsesDefault(t *testing.T) {
	for _, retention := range []time.Duration{0, -time.Minute} {
		dir := t.TempDir()
		fresh := filepath.Join(dir, "0123456789abcdef0123456789abcdef.pptx")
		if err := os.WriteFile(fresh, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		recent := time.Now().Add(-time.Minute)
		if err := os.Chtimes(fresh, recent, recent); err != nil {
			t.Fatal(err)
		}
		cleaner := NewOutputCleaner(OutputCleanerConfig{OutputDir: dir, Retention: retention})
		if _, err := cleaner.CleanupNow(); err != nil {
			t.Fatalf("CleanupNow: %v", err)
		}
		if _, err := os.Stat(fresh); err != nil {
			t.Errorf("retention %v: a minute-old download was deleted", retention)
		}
	}
}

func TestOutputCleaner_skipsDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory with an old mod time
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(subdir, past, past); err != nil {
		t.Fatal(err)
	}

	cleaner := NewOutputCleaner(OutputCleanerConfig{
		OutputDir: dir,
		Retention: 1 * time.Hour,
	})

	cleaned, err := cleaner.CleanupNow()
	if err != nil {
		t.Fatalf("CleanupNow returned error: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 files cleaned, got %d", cleaned)
	}

	// Subdirectory should still exist
	if _, err := os.Stat(subdir); err != nil {
		t.Error("expected subdirectory to still exist")
	}
}

func TestOutputCleaner_StartStop(t *testing.T) {
	dir := t.TempDir()

	cleaner := NewOutputCleaner(OutputCleanerConfig{
		OutputDir: dir,
		Retention: 1 * time.Hour,
		Interval:  50 * time.Millisecond,
	})

	cleaner.Start()
	// Double start is a no-op
	cleaner.Start()

	time.Sleep(100 * time.Millisecond)

	cleaner.Stop()
	// Double stop is a no-op
	cleaner.Stop()
}
