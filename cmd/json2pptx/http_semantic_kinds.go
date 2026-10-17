package main

import (
	"context"
	"encoding/json"

	"github.com/sebahrens/json2pptx/internal/api"
	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/template"
)

func newHTTPSemanticKinds(cfg config.Config, cache *template.MemoryCache) api.SemanticKindsProvider {
	mc := &mcpConfig{templatesDir: cfg.Templates.Dir, cache: cache}
	return func(ctx context.Context, kinds, fields []string, templateName string) (json.RawMessage, bool, error) {
		args := map[string]any{}
		if len(kinds) > 0 {
			args["kinds"] = toAnySlice(kinds)
		}
		if len(fields) > 0 {
			args["fields"] = toAnySlice(fields)
		}
		if templateName != "" {
			args["template"] = templateName
		}
		result, err := mc.handleListSlideKinds(ctx, mcpRequestWithArgs(args))
		if err != nil {
			return nil, false, err
		}
		body, err := json.Marshal(result.StructuredContent)
		return body, !result.IsError, err
	}
}
