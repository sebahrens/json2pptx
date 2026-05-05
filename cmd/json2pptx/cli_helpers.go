package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/template"
)

// cliMCPConfig builds an mcpConfig from common CLI flags.
func cliMCPConfig(templatesDir, outputDir string) *mcpConfig {
	return &mcpConfig{
		templatesDir: templatesDir,
		outputDir:    outputDir,
		cfg:          config.DefaultConfig(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
}

// mcpNoopRequest builds an MCP CallToolRequest with no arguments.
func mcpNoopRequest() mcpgo.CallToolRequest {
	return mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{},
		},
	}
}

// mcpRequestWithArgs builds an MCP CallToolRequest with the given arguments.
func mcpRequestWithArgs(args map[string]any) mcpgo.CallToolRequest {
	return mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: args,
		},
	}
}

// printMCPResultJSON extracts the JSON text from an MCP CallToolResult and
// pretty-prints it to stdout. If the result is an error, it prints to stderr
// and returns an error.
func printMCPResultJSON(result *mcpgo.CallToolResult) error {
	if result == nil {
		return fmt.Errorf("nil result")
	}

	if result.IsError {
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(mcpgo.TextContent); ok {
				fmt.Fprintln(os.Stderr, tc.Text)
			}
		}
		return fmt.Errorf("tool returned an error")
	}

	if len(result.Content) == 0 {
		return fmt.Errorf("empty result")
	}

	tc, ok := result.Content[0].(mcpgo.TextContent)
	if !ok {
		return fmt.Errorf("unexpected content type")
	}

	// Pretty-print the JSON for CLI readability.
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(tc.Text), &raw); err != nil {
		// Not JSON — print as-is.
		fmt.Println(tc.Text)
		return nil
	}

	pretty, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		fmt.Println(tc.Text)
		return nil
	}

	_, err = os.Stdout.Write(append(pretty, '\n'))
	return err
}

// readJSONInput reads JSON from a file path or stdin (when path is "-").
func readJSONInput(path string) (string, error) {
	if path == "-" {
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return "", fmt.Errorf("failed to read stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	return string(data), nil
}
