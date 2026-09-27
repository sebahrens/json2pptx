package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// maxMCPArgumentBytes caps the total string content of one tool call's
// arguments. Text analysis costs tens of microseconds per character, so an
// unbounded argument (a 20 MB text field) would hold the stdio session for
// many minutes; a real deck is far below this budget.
const maxMCPArgumentBytes = 2 << 20

// argSizeMiddleware rejects a tool call whose arguments carry more than
// maxMCPArgumentBytes of string content, before any handler parses them.
func argSizeMiddleware() server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args, ok := request.Params.Arguments.(map[string]any)
			if !ok {
				return next(ctx, request)
			}
			total, largest, largestSize := 0, "", 0
			for k, v := range args {
				n := argStringBytes(v)
				total += n
				if n > largestSize {
					largest, largestSize = k, n
				}
			}
			if total > maxMCPArgumentBytes {
				return argError(argErrorEnvelope{
					Code: diagnostics.CodeInvalidParameter,
					Path: largest,
					Message: fmt.Sprintf("tool arguments carry %d bytes of text (largest: %q, %d bytes); the maximum is %d — split the deck or shorten the text",
						total, largest, largestSize, maxMCPArgumentBytes),
				}), nil
			}
			return next(ctx, request)
		}
	}
}

// argStringBytes sums the string lengths (keys and values) inside an argument.
func argStringBytes(v any) int {
	switch t := v.(type) {
	case string:
		return len(t)
	case map[string]any:
		n := 0
		for k, e := range t {
			n += len(k) + argStringBytes(e)
		}
		return n
	case []any:
		n := 0
		for _, e := range t {
			n += argStringBytes(e)
		}
		return n
	default:
		return 0
	}
}
