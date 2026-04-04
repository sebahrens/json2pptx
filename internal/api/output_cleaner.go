package api

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OutputCleaner is a background daemon that periodically removes expired output files.
// It enforces the FileRetention policy configured in StorageConfig, preventing the
// output directory from growing unbounded in server mode.
type OutputCleaner struct {
	outputDir string
	retention time.Duration
	interval  time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
	logger    *slog.Logger
	mu        sync.Mutex
	running   bool
}

// OutputCleanerConfig holds configuration for the output file cleaner.
type OutputCleanerConfig struct {
	// OutputDir is the directory to clean.
	OutputDir string

	// Retention is the maximum age of output files before they are removed.
	// Files older than this are deleted. Must be positive.
	Retention time.Duration

	// Interval is how often the cleanup runs.
	// If zero or negative, defaults to 5 minutes.
	Interval time.Duration

	// Logger is the logger to use. If nil, uses slog.Default().
	Logger *slog.Logger
}

// NewOutputCleaner creates a new output file cleaner.
// The cleaner is not started; call Start() to begin the background cleanup.
func NewOutputCleaner(cfg OutputCleanerConfig) *OutputCleaner {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &OutputCleaner{
		outputDir: cfg.OutputDir,
		retention: cfg.Retention,
		interval:  interval,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
		logger:    logger,
	}
}

// Start begins the background cleanup goroutine.
// If already running, this is a no-op.
func (c *OutputCleaner) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return
	}

	c.running = true
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})

	go c.loop()

	c.logger.Info("output file cleanup daemon started",
		"output_dir", c.outputDir,
		"retention", c.retention,
		"interval", c.interval,
	)
}

// Stop gracefully stops the background cleanup goroutine.
// Blocks until the goroutine has exited.
// If not running, this is a no-op.
func (c *OutputCleaner) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)
	doneCh := c.doneCh
	c.mu.Unlock()

	<-doneCh
	c.logger.Info("output file cleanup daemon stopped")
}

// loop runs the periodic cleanup.
func (c *OutputCleaner) loop() {
	defer close(c.doneCh)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanup()
		}
	}
}

// cleanup performs a single cleanup pass, removing files older than the retention period.
func (c *OutputCleaner) cleanup() {
	cleaned, err := c.cleanExpiredFiles()
	if err != nil {
		c.logger.Warn("output file cleanup had errors",
			"cleaned", cleaned,
			"error", err,
		)
	} else if cleaned > 0 {
		c.logger.Info("cleaned expired output files",
			"count", cleaned,
		)
	}
}

// cleanExpiredFiles removes files in the output directory older than the retention period.
// Returns the number of files removed and any error encountered.
func (c *OutputCleaner) cleanExpiredFiles() (int, error) {
	entries, err := os.ReadDir(c.outputDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-c.retention)
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(c.outputDir, entry.Name())
			if err := os.Remove(path); err != nil {
				c.logger.Warn("failed to remove expired output file",
					"path", path,
					"error", err,
				)
				continue
			}
			cleaned++
		}
	}

	return cleaned, nil
}

// CleanupNow triggers an immediate cleanup outside the normal interval.
func (c *OutputCleaner) CleanupNow() (int, error) {
	return c.cleanExpiredFiles()
}
