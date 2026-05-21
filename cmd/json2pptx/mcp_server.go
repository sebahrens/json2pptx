package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/resource"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/svggen/fontcache"
)

// mcpConfig holds the resolved configuration for MCP tool handlers.
type mcpConfig struct {
	templatesDir string
	outputDir    string
	cfg          config.Config
	cache        *template.MemoryCache

	// idempotency caches success responses for generate_presentation,
	// auto_repair, and make_deck so transport-layer retries with the same
	// idempotency_key return the original result instead of regenerating.
	// Nil in tests that don't exercise the path — Get/Set tolerate a nil
	// receiver and degrade to no-op behaviour.
	idempotency *idempotencyCache

	// resolverOpts customizes the URL resource resolver used by
	// handleGenerate / handleValidate. Production leaves this zero-valued
	// (SSRF-safe defaults). Tests inject a custom HTTPClient so they can
	// point at httptest.NewServer instances on loopback addresses without
	// tripping SSRF blocks.
	resolverOpts resource.ResolverOptions
}

// resolvePresentationURLs downloads every URL reference in slides via a
// session-scoped resource.Resolver and rewrites the URL fields to point at
// cached local files. Each failed URL is returned as a structured
// diagnostic.
//
// The returned cleanup must be called by the caller (via defer) to remove
// the on-disk cache; it is always non-nil and safe to invoke even when
// findings or err are returned. The cache must stay alive at least until
// generation finishes — closing it earlier invalidates the local paths the
// slides now reference.
//
// Returns (nil, noop, nil) when no URL references are present.
func (mc *mcpConfig) resolvePresentationURLs(slides []SlideInput) ([]diagnostics.Diagnostic, func(), error) {
	noop := func() {}
	if !hasURLReferences(slides) {
		return nil, noop, nil
	}
	resolver, err := resource.NewResolver(mc.resolverOpts)
	if err != nil {
		return nil, noop, err
	}
	findings := resolveURLs(slides, resolver)
	return findings, resolver.Close, nil
}

// runMCP starts an MCP server over stdio, exposing json2pptx tools.
func runMCP() error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	outputDir := fs.String("output", "./output", "Output directory for generated PPTX files")
	configPath := fs.String("config", "", "Path to config file (optional)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx mcp [options]\n\n")
		fmt.Fprintf(os.Stderr, "Start an MCP (Model Context Protocol) server over stdio.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Fail fast if the font subsystem is broken.
	if err := fontcache.Verify(); err != nil {
		return fmt.Errorf("font subsystem check failed: %w", err)
	}

	// Load configuration
	cfg := config.DefaultConfig()
	if *configPath != "" {
		var err error
		cfg, err = config.Load(*configPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
	}

	// Apply flag overrides
	if *templatesDir != "" {
		cfg.Templates.Dir = *templatesDir
	}
	if *outputDir != "" {
		cfg.Storage.OutputDir = *outputDir
	}

	// Logging goes to stderr so stdio transport stays clean
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	mc := &mcpConfig{
		templatesDir: cfg.Templates.Dir,
		outputDir:    cfg.Storage.OutputDir,
		cfg:          cfg,
		cache:        template.NewMemoryCache(24 * time.Hour),
	}

	// The server advertises experimental.compact_responses: true in its
	// initialize response; compaction itself is controlled by client opt-in
	// (the client sends experimental.compact_responses: true in its
	// capabilities) or the deprecated MCP_COMPACT_RESPONSES=1 environment
	// variable.
	hooks := &server.Hooks{}
	hooks.AddAfterInitialize(func(_ context.Context, _ any, _ *mcp.InitializeRequest, result *mcp.InitializeResult) {
		if result.Capabilities.Experimental == nil {
			result.Capabilities.Experimental = make(map[string]any)
		}
		result.Capabilities.Experimental["compact_responses"] = true
	})

	s := server.NewMCPServer(
		"json2pptx",
		Version,
		server.WithToolCapabilities(false),
		server.WithHooks(hooks),
	)

	registerMCPTools(s, mc)

	slog.Info("starting json2pptx MCP server",
		"version", Version,
		"templates_dir", mc.templatesDir,
		"output_dir", mc.outputDir,
	)

	return server.ServeStdio(s)
}
