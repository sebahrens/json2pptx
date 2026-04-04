// Package main provides the entry point for the standalone SVG generation API server.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sebahrens/json2pptx/svggen"
	"github.com/sebahrens/json2pptx/svggen/server/httpserver"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// serverConfig holds the parsed server configuration.
type serverConfig struct {
	Port                 int
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	ShutdownTimeout      time.Duration
	MaxRequestSize       int64
	CacheTTL             time.Duration
	CacheMaxEntries      int
	CacheCleanupInterval time.Duration

	// Security configuration
	AuthEnabled       bool
	APIKeys           string // Comma-separated list of valid API keys
	RateLimitEnabled  bool
	RateLimitRequests int
	RateLimitWindow   time.Duration
	AllowedOrigins    string // Comma-separated list of allowed CORS origins
	TrustedProxies    string // Comma-separated list of trusted proxy CIDRs
}

// runOptions allows customizing run() behavior for testing.
type runOptions struct {
	// config provides all server configuration values directly.
	config serverConfig
	// skipServer if true, returns after validation without starting the HTTP server.
	skipServer bool
	// logWriter is the destination for log output. If nil, uses os.Stdout.
	logWriter io.Writer
}

func run() error {
	// Parse command-line flags
	port := flag.Int("port", 3001, "HTTP server port")
	readTimeout := flag.Duration("read-timeout", 30*time.Second, "HTTP read timeout")
	writeTimeout := flag.Duration("write-timeout", 60*time.Second, "HTTP write timeout")
	idleTimeout := flag.Duration("idle-timeout", 120*time.Second, "HTTP idle timeout")
	shutdownTimeout := flag.Duration("shutdown-timeout", 30*time.Second, "Graceful shutdown timeout")
	maxRequestSize := flag.Int64("max-request-size", 10*1024*1024, "Maximum request body size in bytes")

	// Cache configuration flags
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "Cache TTL for rendered SVGs (0 to disable)")
	cacheMaxEntries := flag.Int("cache-max-entries", 1000, "Maximum cache entries")
	cacheCleanupInterval := flag.Duration("cache-cleanup-interval", 1*time.Minute, "Cache cleanup interval")

	// Security configuration flags
	authEnabled := flag.Bool("auth-enabled", false, "Enable API key authentication")
	apiKeys := flag.String("api-keys", "", "Comma-separated list of valid API keys")
	rateLimitEnabled := flag.Bool("rate-limit-enabled", false, "Enable rate limiting")
	rateLimitRequests := flag.Int("rate-limit-requests", 100, "Maximum requests per window")
	rateLimitWindow := flag.Duration("rate-limit-window", time.Minute, "Rate limit time window")
	allowedOrigins := flag.String("allowed-origins", "", "Comma-separated list of allowed CORS origins")
	trustedProxies := flag.String("trusted-proxies", "", "Comma-separated list of trusted proxy CIDRs")

	flag.Parse()

	opts := runOptions{
		config: serverConfig{
			Port:                 *port,
			ReadTimeout:          *readTimeout,
			WriteTimeout:         *writeTimeout,
			IdleTimeout:          *idleTimeout,
			ShutdownTimeout:      *shutdownTimeout,
			MaxRequestSize:       *maxRequestSize,
			CacheTTL:             *cacheTTL,
			CacheMaxEntries:      *cacheMaxEntries,
			CacheCleanupInterval: *cacheCleanupInterval,
			AuthEnabled:          *authEnabled,
			APIKeys:              *apiKeys,
			RateLimitEnabled:     *rateLimitEnabled,
			RateLimitRequests:    *rateLimitRequests,
			RateLimitWindow:      *rateLimitWindow,
			AllowedOrigins:       *allowedOrigins,
			TrustedProxies:       *trustedProxies,
		},
		logWriter: os.Stdout,
	}

	return runWithOptions(opts)
}

func runWithOptions(opts runOptions) error {
	cfg := opts.config

	// Validate port
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d (must be 1-65535)", cfg.Port)
	}

	// Validate timeouts
	if cfg.ReadTimeout <= 0 {
		return fmt.Errorf("invalid read timeout: %v (must be positive)", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout <= 0 {
		return fmt.Errorf("invalid write timeout: %v (must be positive)", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout <= 0 {
		return fmt.Errorf("invalid idle timeout: %v (must be positive)", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout <= 0 {
		return fmt.Errorf("invalid shutdown timeout: %v (must be positive)", cfg.ShutdownTimeout)
	}
	if cfg.MaxRequestSize <= 0 {
		return fmt.Errorf("invalid max request size: %d (must be positive)", cfg.MaxRequestSize)
	}

	// Setup logging
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	logWriter := opts.logWriter
	if logWriter == nil {
		logWriter = os.Stdout
	}
	logger := slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	// Create cache configuration
	var cacheConfig *svggen.CacheConfig
	if cfg.CacheTTL > 0 {
		cacheConfig = &svggen.CacheConfig{
			TTL:             cfg.CacheTTL,
			MaxEntries:      cfg.CacheMaxEntries,
			CleanupInterval: cfg.CacheCleanupInterval,
		}
		logger.Info("cache enabled",
			"ttl", cfg.CacheTTL,
			"max_entries", cfg.CacheMaxEntries,
			"cleanup_interval", cfg.CacheCleanupInterval,
		)
	} else {
		// Disable cache by setting TTL to 0
		cacheConfig = &svggen.CacheConfig{TTL: 0}
		logger.Info("cache disabled")
	}

	// Build security configuration
	securityCfg := httpserver.DefaultSecurityConfig()

	// Configure authentication
	if cfg.AuthEnabled {
		securityCfg.Auth.Enabled = true
		if cfg.APIKeys != "" {
			securityCfg.Auth.APIKeys = splitAndTrim(cfg.APIKeys)
		}
		logger.Info("API key authentication enabled",
			"key_count", len(securityCfg.Auth.APIKeys),
		)
	}

	// Configure rate limiting
	if cfg.RateLimitEnabled {
		securityCfg.RateLimit.Enabled = true
		securityCfg.RateLimit.RequestsPerWindow = cfg.RateLimitRequests
		securityCfg.RateLimit.WindowDuration = cfg.RateLimitWindow
		logger.Info("rate limiting enabled",
			"requests_per_window", cfg.RateLimitRequests,
			"window", cfg.RateLimitWindow,
		)
	}

	// Configure CORS
	if cfg.AllowedOrigins != "" {
		securityCfg.AllowedOrigins = splitAndTrim(cfg.AllowedOrigins)
		logger.Info("CORS protection enabled",
			"allowed_origins", securityCfg.AllowedOrigins,
		)
	}

	// Configure trusted proxies
	if cfg.TrustedProxies != "" {
		securityCfg.TrustedProxies = splitAndTrim(cfg.TrustedProxies)
		logger.Info("trusted proxies configured",
			"proxies", securityCfg.TrustedProxies,
		)
	}

	// Create server configuration
	apiCfg := httpserver.Config{
		Port:            cfg.Port,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		IdleTimeout:     cfg.IdleTimeout,
		ShutdownTimeout: cfg.ShutdownTimeout,
		MaxRequestSize:  cfg.MaxRequestSize,
		Logger:          logger,
		CacheConfig:     cacheConfig,
		Security:        &securityCfg,
	}

	// Early exit for testing
	if opts.skipServer {
		return nil
	}

	// Create server (uses default registry with all registered diagram types)
	server := httpserver.NewServer(apiCfg, nil)

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run server
	return server.Run(ctx)
}

// splitAndTrim splits a comma-separated string and trims whitespace from each element.
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
