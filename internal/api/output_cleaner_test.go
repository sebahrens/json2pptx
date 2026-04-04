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
	oldFile := filepath.Join(dir, "old.pptx")
	newFile := filepath.Join(dir, "new.pptx")

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
