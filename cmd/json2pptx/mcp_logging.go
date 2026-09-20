package main

import (
	"context"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// logRenderEvent keeps the process log on stderr and forwards an event only to
// the MCP session attached to ctx. The SDK applies that session's setLevel
// filter; the default error threshold suppresses our info/warning events.
func (mc *mcpConfig) logRenderEvent(ctx context.Context, level mcp.LoggingLevel, message string, fields map[string]any) {
	data := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		data[key] = value
	}
	data["message"] = message
	logLevel := slog.LevelInfo
	if level == mcp.LoggingLevelWarning {
		logLevel = slog.LevelWarn
	}
	slog.LogAttrs(ctx, logLevel, message, slog.Any("details", fields))
	if mc != nil && mc.logSender != nil {
		if session := server.ClientSessionFromContext(ctx); session != nil && session.Initialized() {
			if err := mc.logSender(ctx, level, data); err != nil {
				slog.WarnContext(ctx, "MCP log notification unavailable", "error", err)
			}
		}
	}
}
