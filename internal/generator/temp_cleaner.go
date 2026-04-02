// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"log/slog"
	"sync"
	"time"
)

// TempFileCleaner is a background daemon that periodically cleans orphaned temp files.
// This addresses MED-05 from the security audit: ensuring temp files from crashed processes
// are eventually cleaned up, not just at startup.
type TempFileCleaner struct {
	interval time.Duration
	maxAge   time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	logger   *slog.Logger
	mu       sync.Mutex
	running  bool
}

// TempFileCleanerConfig holds configuration for the temp file cleaner.
type TempFileCleanerConfig struct {
	// Interval is how often the cleanup runs.
	// If zero or negative, defaults to 5 minutes.
	Interval time.Duration

	// MaxAge is the maximum age of temp files before they are removed.
	// If zero or negative, defaults to DefaultTempFileMaxAge (1 hour).
	MaxAge time.Duration

	// Logger is the logger to use. If nil, uses slog.Default().
	Logger *slog.Logger
}

// DefaultTempFileCleanerConfig returns the default configuration.
func DefaultTempFileCleanerConfig() TempFileCleanerConfig {
	return TempFileCleanerConfig{
		Interval: 5 * time.Minute,
		MaxAge:   DefaultTempFileMaxAge,
	}
}

// NewTempFileCleaner creates a new temp file cleaner with the given configuration.
// The cleaner is not started; call Start() to begin the background cleanup.
func NewTempFileCleaner(cfg TempFileCleanerConfig) *TempFileCleaner {
	interval := cfg.Interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = DefaultTempFileMaxAge
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &TempFileCleaner{
		interval: interval,
		maxAge:   maxAge,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		logger:   logger,
	}
}

// Start begins the background cleanup goroutine.
// If already running, this is a no-op.
func (c *TempFileCleaner) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return
	}

	c.running = true
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})

	go c.loop()

	c.logger.Info("temp file cleanup daemon started",
		"interval", c.interval,
		"max_age", c.maxAge,
	)
}

// Stop gracefully stops the background cleanup goroutine.
// Blocks until the goroutine has exited.
// If not running, this is a no-op.
func (c *TempFileCleaner) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)
	doneCh := c.doneCh
	c.mu.Unlock()

	// Wait for goroutine to finish
	<-doneCh
	c.logger.Info("temp file cleanup daemon stopped")
}

// loop runs the periodic cleanup.
func (c *TempFileCleaner) loop() {
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

// cleanup performs a single cleanup pass.
func (c *TempFileCleaner) cleanup() {
	cleaned, err := CleanupOrphanedTempFiles(c.maxAge)
	if err != nil {
		c.logger.Warn("temp file cleanup had errors",
			"cleaned", cleaned,
			"error", err,
		)
	} else if cleaned > 0 {
		c.logger.Info("cleaned orphaned temp files",
			"count", cleaned,
		)
	}
	// Don't log anything if cleaned == 0 to avoid noise
}

// CleanupNow triggers an immediate cleanup outside the normal interval.
// This is useful for testing or manual intervention.
func (c *TempFileCleaner) CleanupNow() (int, error) {
	return CleanupOrphanedTempFiles(c.maxAge)
}

// IsRunning returns whether the cleanup daemon is currently running.
func (c *TempFileCleaner) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}
