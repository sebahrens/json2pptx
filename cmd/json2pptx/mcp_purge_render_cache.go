package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/render"
)

// purge_render_cache (go-slide-creator-dpys)
//
// Renders bound the cache themselves, so an agent never has to think about it.
// This is the deliberate reclaim: a constrained disk, or a session that just
// rendered a 400-slide deck at high density and wants the space back now rather
// than at the 24h mark.

func mcpPurgeRenderCacheTool() mcp.Tool {
	return mcp.NewTool("purge_render_cache",
		mcp.WithDescription("Reclaim the on-disk render cache. Renders already sweep it (artifacts are evicted after 24h unused, or oldest-first once the cache exceeds 500 MiB), so this is for reclaiming space deliberately rather than routine hygiene. get_capabilities().runtime.render_cache_bytes reports current usage. Cached renders are content-addressed, so purging costs re-render time, never correctness."),
		mcp.WithRawOutputSchema(withErrorEnvelope(outputSchemaPurgeRenderCache)),
		mcp.WithBoolean("all",
			mcp.Description("When true, remove every cached render regardless of age or size. Default false: apply the standard age and size bounds, which is what a render does."),
		),
	)
}

func handlePurgeRenderCache(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	before := render.CacheBytes()

	all := request.GetArguments()["all"] == true
	maxAge, maxBytes := render.CacheMaxAge, int64(render.CacheMaxBytes)
	if all {
		// Any positive age is in the past for every file once maxBytes is 0.
		maxAge, maxBytes = time.Nanosecond, 0
	}
	reclaimed, err := render.SweepCache(maxAge, maxBytes)
	if err != nil {
		return mcpErrorWithNext("INTERNAL", fmt.Sprintf("purge render cache: %v", err), nil), nil
	}

	return api.MCPSuccessResult(ctx, map[string]any{
		"bytes_before":    before,
		"bytes_reclaimed": reclaimed,
		"bytes_after":     render.CacheBytes(),
		"scope":           purgeScope(all),
		"policy":          render.ArtifactCleanupPolicy,
	})
}

func purgeScope(all bool) string {
	if all {
		return "all"
	}
	return "bounds"
}

// runPurgeRenderCache implements the "purge-render-cache" CLI subcommand, the
// counterpart to the purge_render_cache MCP tool.
func runPurgeRenderCache() error {
	fs := flag.NewFlagSet("purge-render-cache", flag.ContinueOnError)
	all := fs.Bool("all", false, "Remove every cached render, ignoring the age and size bounds")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx purge-render-cache [--all]\n\n")
		fmt.Fprintf(os.Stderr, "Reclaims the on-disk render cache. Renders already sweep it;\n")
		fmt.Fprintf(os.Stderr, "this is for reclaiming space deliberately.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := map[string]any{}
	if *all {
		args["all"] = true
	}
	result, err := handlePurgeRenderCache(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("purge-render-cache: %w", err)
	}
	return printMCPResultJSON(result)
}
