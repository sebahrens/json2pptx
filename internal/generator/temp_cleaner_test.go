package generator

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNewTempFileCleaner tests the cleaner constructor with various configurations.
func TestNewTempFileCleaner(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := DefaultTempFileCleanerConfig()
		cleaner := NewTempFileCleaner(cfg)

		if cleaner.interval != 5*time.Minute {
			t.Errorf("interval = %v, want 5m", cleaner.interval)
		}
		if cleaner.maxAge != DefaultTempFileMaxAge {
			t.Errorf("maxAge = %v, want %v", cleaner.maxAge, DefaultTempFileMaxAge)
		}
		if cleaner.IsRunning() {
			t.Error("cleaner should not be running after creation")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := TempFileCleanerConfig{
			Interval: 10 * time.Second,
			MaxAge:   30 * time.Minute,
		}
		cleaner := NewTempFileCleaner(cfg)

		if cleaner.interval != 10*time.Second {
			t.Errorf("interval = %v, want 10s", cleaner.interval)
		}
		if cleaner.maxAge != 30*time.Minute {
			t.Errorf("maxAge = %v, want 30m", cleaner.maxAge)
		}
	})

	t.Run("zero values use defaults", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{})

		if cleaner.interval != 5*time.Minute {
			t.Errorf("interval = %v, want 5m", cleaner.interval)
		}
		if cleaner.maxAge != DefaultTempFileMaxAge {
			t.Errorf("maxAge = %v, want %v", cleaner.maxAge, DefaultTempFileMaxAge)
		}
	})

	t.Run("negative values use defaults", func(t *testing.T) {
		cfg := TempFileCleanerConfig{
			Interval: -1 * time.Second,
			MaxAge:   -1 * time.Hour,
		}
		cleaner := NewTempFileCleaner(cfg)

		if cleaner.interval != 5*time.Minute {
			t.Errorf("interval = %v, want 5m", cleaner.interval)
		}
		if cleaner.maxAge != DefaultTempFileMaxAge {
			t.Errorf("maxAge = %v, want %v", cleaner.maxAge, DefaultTempFileMaxAge)
		}
	})

	t.Run("custom logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cfg := TempFileCleanerConfig{
			Logger: logger,
		}
		cleaner := NewTempFileCleaner(cfg)

		if cleaner.logger != logger {
			t.Error("custom logger not set")
		}
	})
}

// TestTempFileCleanerStartStop tests the start/stop lifecycle.
func TestTempFileCleanerStartStop(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			Interval: 1 * time.Hour, // Long interval to avoid actual cleanup
		})

		if cleaner.IsRunning() {
			t.Fatal("should not be running initially")
		}

		cleaner.Start()
		if !cleaner.IsRunning() {
			t.Error("should be running after Start()")
		}

		cleaner.Stop()
		if cleaner.IsRunning() {
			t.Error("should not be running after Stop()")
		}
	})

	t.Run("multiple starts are no-op", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			Interval: 1 * time.Hour,
		})

		cleaner.Start()
		cleaner.Start() // Should be safe
		cleaner.Start() // Should be safe

		if !cleaner.IsRunning() {
			t.Error("should be running")
		}

		cleaner.Stop()
	})

	t.Run("multiple stops are no-op", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			Interval: 1 * time.Hour,
		})

		cleaner.Start()
		cleaner.Stop()
		cleaner.Stop() // Should be safe
		cleaner.Stop() // Should be safe

		if cleaner.IsRunning() {
			t.Error("should not be running")
		}
	})

	t.Run("stop without start is no-op", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{})
		cleaner.Stop() // Should not panic
	})

	t.Run("restart after stop", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			Interval: 1 * time.Hour,
		})

		cleaner.Start()
		cleaner.Stop()

		// Should be able to start again
		cleaner.Start()
		if !cleaner.IsRunning() {
			t.Error("should be running after restart")
		}
		cleaner.Stop()
	})
}

// TestTempFileCleanerCleanupNow tests on-demand cleanup.
func TestTempFileCleanerCleanupNow(t *testing.T) {
	t.Run("cleanup without old files", func(t *testing.T) {
		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			MaxAge: 1 * time.Hour,
		})

		cleaned, err := cleaner.CleanupNow()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should succeed even if no files to clean
		_ = cleaned
	})

	t.Run("cleanup removes old files", func(t *testing.T) {
		tempDir := os.TempDir()

		// Create an old temp file matching the pattern
		oldFile := filepath.Join(tempDir, "svg-converted-test-old.png")
		if err := os.WriteFile(oldFile, []byte("old content"), 0644); err != nil {
			t.Fatalf("failed to create old file: %v", err)
		}
		// Set modification time to 2 hours ago
		oldTime := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
			t.Fatalf("failed to set file time: %v", err)
		}
		defer os.Remove(oldFile) // Cleanup in case test fails

		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			MaxAge: 1 * time.Hour,
		})

		cleaned, err := cleaner.CleanupNow()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if cleaned < 1 {
			t.Error("expected at least 1 file to be cleaned")
		}

		// Verify file is gone
		if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
			t.Error("old file should have been removed")
		}
	})

	t.Run("cleanup preserves new files", func(t *testing.T) {
		tempDir := os.TempDir()

		// Create a new temp file matching the pattern
		newFile := filepath.Join(tempDir, "svg-converted-test-new.png")
		if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
			t.Fatalf("failed to create new file: %v", err)
		}
		defer os.Remove(newFile)

		cleaner := NewTempFileCleaner(TempFileCleanerConfig{
			MaxAge: 1 * time.Hour,
		})

		_, err := cleaner.CleanupNow()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify file still exists (it's new)
		if _, err := os.Stat(newFile); os.IsNotExist(err) {
			t.Error("new file should not have been removed")
		}
	})
}

// TestTempFileCleanerPeriodicCleanup tests that periodic cleanup runs.
func TestTempFileCleanerPeriodicCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping periodic cleanup test in short mode")
	}

	tempDir := os.TempDir()

	// Create an old temp file
	oldFile := filepath.Join(tempDir, "svg-converted-periodic-test.png")
	if err := os.WriteFile(oldFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create old file: %v", err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set file time: %v", err)
	}
	defer os.Remove(oldFile) // Cleanup in case test fails

	// Use a very short interval for testing
	cleaner := NewTempFileCleaner(TempFileCleanerConfig{
		Interval: 50 * time.Millisecond,
		MaxAge:   1 * time.Hour,
	})

	cleaner.Start()
	defer cleaner.Stop()

	// Wait for at least one cleanup cycle
	time.Sleep(100 * time.Millisecond)

	// Verify file was cleaned
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old file should have been removed by periodic cleanup")
	}
}

// TestTempFileCleanerGracefulShutdown tests that stop waits for cleanup to finish.
func TestTempFileCleanerGracefulShutdown(t *testing.T) {
	cleaner := NewTempFileCleaner(TempFileCleanerConfig{
		Interval: 10 * time.Millisecond,
	})

	cleaner.Start()

	// Give it a moment to enter the loop
	time.Sleep(5 * time.Millisecond)

	// Stop should block until goroutine exits
	done := make(chan struct{})
	go func() {
		cleaner.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good, stopped within reasonable time
	case <-time.After(1 * time.Second):
		t.Error("Stop() did not return within timeout")
	}

	if cleaner.IsRunning() {
		t.Error("should not be running after Stop()")
	}
}

// TestDefaultTempFileCleanerConfig tests the default config values.
func TestDefaultTempFileCleanerConfig(t *testing.T) {
	cfg := DefaultTempFileCleanerConfig()

	if cfg.Interval != 5*time.Minute {
		t.Errorf("Interval = %v, want 5m", cfg.Interval)
	}
	if cfg.MaxAge != DefaultTempFileMaxAge {
		t.Errorf("MaxAge = %v, want %v", cfg.MaxAge, DefaultTempFileMaxAge)
	}
	if cfg.Logger != nil {
		t.Error("Logger should be nil by default")
	}
}
