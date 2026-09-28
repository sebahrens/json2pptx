package main

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// decodeOptionalArg decodes the optional structured argument name into dst.
// An absent or null argument leaves dst untouched and returns nil. A value
// that does not decode into dst's type returns an INVALID_PARAMETER error
// result: these arguments used to be parsed with `_ = json.Unmarshal`, so a
// malformed content_hints / recent_patterns / candidates / must_include was
// silently dropped and the tool answered a different question than the one
// asked (go-slide-creator-tcxsq).
func decodeOptionalArg(request mcp.CallToolRequest, tool, name string, dst any, expectedType string, example any) *mcp.CallToolResult {
	raw, ok := request.GetArguments()[name]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err == nil {
		err = json.Unmarshal(b, dst)
	}
	if err != nil {
		return argInvalidValue(tool, string(diagnostics.CodeInvalidParameter), name,
			fmt.Sprintf("%s must be %s: %v", name, expectedType, err), expectedType, example, nil)
	}
	return nil
}
