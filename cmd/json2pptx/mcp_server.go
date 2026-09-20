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

	"github.com/sebahrens/json2pptx/internal/api"
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

	// loopSessions stores resumable per-pass checkpoints for auto_repair and
	// make_deck so a caller can continue a convergence loop via resume_token
	// without repeating completed passes. Per-process and TTL'd, like
	// idempotency. Nil in tests that don't exercise resume — Save/Load tolerate
	// a nil receiver (resume is simply not offered).
	loopSessions *loopSessionStore

	// deckHandles stores DeckSpecs server-side so a revision does not have to
	// re-upload the whole spec (go-slide-creator-voxp). Per-process and TTL'd,
	// like the two above. Nil in tests that don't exercise handles — Save/Load
	// tolerate a nil receiver, so deck_id is simply never offered.
	deckHandles *deckHandleStore

	// resolverOpts customizes the URL resource resolver used by
	// handleGenerate / handleValidate. Production leaves this zero-valued
	// (SSRF-safe defaults). Tests inject a custom HTTPClient so they can
	// point at httptest.NewServer instances on loopback addresses without
	// tripping SSRF blocks.
	resolverOpts resource.ResolverOptions

	// progressSender forwards notifications/progress through the active MCP
	// server. Nil in direct-handler tests and CLI calls.
	progressSender func(context.Context, map[string]any) error

	// logSender forwards a render event only to the MCP session carried by ctx.
	// Nil for direct-handler and CLI callers, which still log to stderr.
	logSender func(context.Context, mcp.LoggingLevel, map[string]any) error
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

// newServerMCPConfig builds the mcpConfig used by the production stdio server.
// It wires the idempotency cache so idempotency_key works in real MCP server
// use (not just the CLI/test helper path) and the long-lived template cache.
func newServerMCPConfig(cfg config.Config) *mcpConfig {
	return &mcpConfig{
		templatesDir: cfg.Templates.Dir,
		outputDir:    cfg.Storage.OutputDir,
		cfg:          cfg,
		cache:        template.NewMemoryCache(24 * time.Hour),
		idempotency:  newIdempotencyCache(idempotencyCacheTTL),
		loopSessions: newLoopSessionStore(loopSessionTTL),
		deckHandles:  newDeckHandleStore(deckHandleTTL),
	}
}

// newMCPServer builds the json2pptx MCP server with every tool registered and
// the server-wide options production relies on: the quality-workflow
// `instructions` sent in the initialize response (mcp_instructions.go), strict
// argument decoding
// (unknown tool arguments are rejected with UNKNOWN_PARAMETER + did_you_mean,
// see mcp_strict_args.go). runMCP and the server-level tests share it so the
// behaviour under test is the behaviour that ships. extra options (e.g. hooks)
// are appended.
func newMCPServer(mc *mcpConfig, extra ...server.ServerOption) *server.MCPServer {
	var s *server.MCPServer
	opts := []server.ServerOption{
		server.WithToolCapabilities(false),
		// Resources are how the deck itself leaves the server
		// (go-slide-creator-fx52): a generated .pptx is readable as a blob at
		// json2pptx://deck/<name>, so a host that cannot see the server's
		// filesystem can still hand the user the file. No subscriptions — the
		// resources are immutable once written — and no list-changed
		// notifications, since the static set is fixed at startup.
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
		server.WithLogging(),
		server.WithCompletions(),
		server.WithPromptCompletionProvider(&mcpCompletionProvider{mc: mc}),
		server.WithResourceCompletionProvider(&mcpCompletionProvider{mc: mc}),
		// The instructions say so when this server cannot render: a client that
		// never reads get_capabilities would otherwise be told to finish with a
		// step that cannot run here (go-slide-creator-a7fh).
		server.WithInstructions(mcpInstructionsFor(renderDependencyStatus())),
		// Alias normalisation runs BEFORE the strict check so a sibling name is
		// rewritten rather than rejected (go-slide-creator-r1m3).
		server.WithToolHandlerMiddleware(argAliasMiddleware()),
		server.WithToolHandlerMiddleware(strictArgsMiddleware(func(name string) *server.ServerTool {
			return s.GetTool(name)
		})),
	}
	s = server.NewMCPServer("json2pptx", Version, append(opts, extra...)...)
	mc.progressSender = func(ctx context.Context, params map[string]any) error {
		return s.SendNotificationToClient(ctx, "notifications/progress", params)
	}
	mc.logSender = func(ctx context.Context, level mcp.LoggingLevel, data map[string]any) error {
		return s.SendLogMessageToClient(ctx, mcp.NewLoggingMessageNotification(level, "json2pptx", data))
	}
	registerMCPTools(s, mc)
	registerMCPResources(s, mc)
	registerMCPPrompts(s)
	return s
}

// runMCP starts an MCP server over stdio, exposing json2pptx tools.
func runMCP() error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	outputDir := fs.String("output", "./output", "Output directory for generated PPTX files")
	configPath := fs.String("config", "", "Path to config file (optional)")
	toolsProfile := fs.String("tools", toolProfileCore, "Tool profile advertised in tools/list: core (default; ~21 tools, no outputSchema) or all (full catalog). Env: "+toolProfileEnv)
	textFallback := fs.String("text-fallback", string(api.TextFallbackAuto),
		"Whether tool results also carry the payload as JSON text alongside structuredContent: "+
			"auto (default; omitted for clients that negotiated protocol 2025-06-18 or later, kept for older ones), "+
			"always (keep it for every client), never. Env: "+textFallbackEnv)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx mcp [options]\n\n")
		fmt.Fprintf(os.Stderr, "Start an MCP (Model Context Protocol) server over stdio.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	// Record whether the directory flags were explicitly provided so their
	// non-empty default values don't overwrite config-file/env directories.
	var templatesDirSet, outputDirSet, toolsSet bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "templates-dir":
			templatesDirSet = true
		case "output":
			outputDirSet = true
		case "tools":
			toolsSet = true
		}
	})

	profile, err := resolveToolProfile(*toolsProfile, toolsSet)
	if err != nil {
		return err
	}

	// Fail fast if the font subsystem is broken.
	if err := fontcache.Verify(); err != nil {
		return fmt.Errorf("font subsystem check failed: %w", err)
	}

	// Load configuration. config.Load is always called (even with an empty
	// path) so environment overrides apply without --config.
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply flag overrides only when the flag was explicitly set, so default
	// flag values don't clobber config-file/env values.
	if templatesDirSet {
		cfg.Templates.Dir = *templatesDir
	}
	if outputDirSet {
		cfg.Storage.OutputDir = *outputDir
	}

	// Logging goes to stderr so stdio transport stays clean
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	mc := newServerMCPConfig(cfg)

	// Sending the whole payload as text AND as structuredContent doubled every
	// response — 389 KB on the wire for 168 KB of information across one pass of
	// the core tools (go-slide-creator-vxre).
	api.SetTextFallbackMode(resolveTextFallbackMode(*textFallback))

	// Responses are always compact JSON; the server still advertises
	// experimental.compact_responses: true and still honours the client
	// capability and the deprecated MCP_COMPACT_RESPONSES=1 environment
	// variable, but neither changes anything.
	hooks := &server.Hooks{}
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, request *mcp.InitializeRequest, result *mcp.InitializeResult) {
		if result.Capabilities.Experimental == nil {
			result.Capabilities.Experimental = make(map[string]any)
		}
		result.Capabilities.Experimental["compact_responses"] = true
		// The session interface exposes client info and capabilities but not
		// the negotiated protocol version, so record it here: it decides
		// whether the text fallback is worth sending.
		if request != nil {
			api.RecordProtocolVersion(ctx, request.Params.ProtocolVersion)
		}
	})
	hooks.AddOnUnregisterSession(func(ctx context.Context, _ server.ClientSession) {
		api.ForgetProtocolVersion(ctx)
	})

	s := newJSON2PPTXMCPServer(mc, profile, server.WithHooks(hooks))

	slog.Info("starting json2pptx MCP server",
		"version", Version,
		"tool_profile", profile,
		"templates_dir", mc.templatesDir,
		"output_dir", mc.outputDir,
	)

	return server.ServeStdio(s)
}

// textFallbackEnv is the environment variable form of --text-fallback.
const textFallbackEnv = "JSON2PPTX_MCP_TEXT_FALLBACK"

// resolveTextFallbackMode reads the flag, letting the environment variable win
// only when the flag is at its default (mirroring how --tools resolves).
func resolveTextFallbackMode(flagValue string) api.TextFallbackMode {
	value := flagValue
	if value == "" || value == string(api.TextFallbackAuto) {
		if env := os.Getenv(textFallbackEnv); env != "" {
			value = env
		}
	}
	switch api.TextFallbackMode(value) {
	case api.TextFallbackAlways:
		return api.TextFallbackAlways
	case api.TextFallbackNever:
		return api.TextFallbackNever
	default:
		return api.TextFallbackAuto
	}
}
