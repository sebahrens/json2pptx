package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
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

	// Fail fast if the font subsystem is broken.
	if err := fontcache.Verify(); err != nil {
		return fmt.Errorf("font subsystem check failed: %w", err)
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

	// Start output file cleanup daemon (enforces FileRetention policy)
	outputCleaner := api.NewOutputCleaner(api.OutputCleanerConfig{
		OutputDir: cfg.Storage.OutputDir,
		Retention: cfg.Storage.FileRetention,
		Interval:  cfg.Storage.CleanupInterval,
		Logger:    logger,
	})
	outputCleaner.Start()
	defer outputCleaner.Stop()

	// Create HTTP server
	server := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:        apiServer,
		ReadTimeout:    cfg.Server.ReadTimeout,
		WriteTimeout:   cfg.Server.WriteTimeout,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start pprof server if configured
	if cfg.Server.PprofPort > 0 {
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pprof.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		pprofAddr := fmt.Sprintf("%s:%d", cfg.Server.PprofBind, cfg.Server.PprofPort)
		pprofServer := &http.Server{
			Addr:              pprofAddr,
			Handler:           pprofMux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() {
			slog.Info("starting pprof server", "addr", pprofAddr)
			if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("pprof server failed", "error", err)
			}
		}()
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
