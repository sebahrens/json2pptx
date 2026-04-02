package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
)

func runServe() error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)

	port := fs.Int("port", 0, "HTTP port (overrides config file)")
	configPath := fs.String("config", "", "Path to config file")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx serve [options]\n\n")
		fmt.Fprintf(os.Stderr, "Start the HTTP API server.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx serve\n")
		fmt.Fprintf(os.Stderr, "  json2pptx serve --port 3000\n")
		fmt.Fprintf(os.Stderr, "  json2pptx serve --config config.yaml --port 8080\n")
		fmt.Fprintf(os.Stderr, "  json2pptx serve --templates-dir /usr/share/templates\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Setup logging
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("starting json2pptx serve",
		"version", Version,
		"commit", CommitSHA,
		"build_time", BuildTime,
	)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply flag overrides
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if fs.Lookup("templates-dir").Value.String() != fs.Lookup("templates-dir").DefValue {
		cfg.Templates.Dir = *templatesDir
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	slog.Info("configuration loaded",
		"port", cfg.Server.Port,
		"templates_dir", cfg.Templates.Dir,
	)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(cfg.Storage.OutputDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	// Create template cache directory if it doesn't exist
	if cfg.Templates.CacheDir != "" {
		if err := os.MkdirAll(cfg.Templates.CacheDir, 0755); err != nil {
			return fmt.Errorf("create cache directory: %w", err)
		}
	}

	// Create template cache
	cache := template.NewMemoryCache(24 * time.Hour)

	// Start periodic temp file cleanup daemon
	tempCleaner := generator.NewTempFileCleaner(generator.TempFileCleanerConfig{
		Interval: cfg.Storage.CleanupInterval,
		MaxAge:   cfg.Storage.TempFileMaxAge,
		Logger:   logger,
	})
	tempCleaner.Start()
	defer tempCleaner.Stop()

	if cleaned, cleanErr := tempCleaner.CleanupNow(); cleanErr != nil {
		slog.Warn("startup temp file cleanup had errors", "cleaned", cleaned, "error", cleanErr)
	} else if cleaned > 0 {
		slog.Info("startup cleanup: removed orphaned temp files", "count", cleaned)
	}

	// Create API server
	apiServer := api.NewServer(api.ServerConfig{
		TemplatesDir:     cfg.Templates.Dir,
		OutputDir:        cfg.Storage.OutputDir,
		Cache:            cache,
		Logger:           logger,
		StrictValidation: cfg.Templates.IsStrictValidation(),
	})

	// Create HTTP server
	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        apiServer,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}

	// Start server
	errChan := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", server.Addr)
		if listenErr := server.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errChan <- listenErr
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigChan:
		slog.Info("shutdown signal received", "signal", sig)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	slog.Info("server stopped")
	return nil
}
