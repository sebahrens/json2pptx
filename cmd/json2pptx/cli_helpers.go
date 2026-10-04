package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/sebahrens/json2pptx/internal/config"
	"github.com/sebahrens/json2pptx/internal/template"
)

// printDoubleDashUsage prints flag defaults in GNU style (--flag) to fs.Output(),
// matching flag.PrintDefaults() output but with a double-dash prefix. Use this
// in subcommand Usage callbacks so help text presents the canonical --flag form.
//
// Both -flag and --flag syntax are accepted at parse time (Go's flag package
// supports both), so existing scripts using single-dash continue to work.
func printDoubleDashUsage(fs *flag.FlagSet) {
	out := fs.Output()
	fs.VisitAll(func(f *flag.Flag) {
		var b strings.Builder
		// Single-letter names keep the GNU short-flag prefix (-x); long names
		// get the double-dash form (--name).
		prefix := "--"
		if len(f.Name) == 1 {
			prefix = "-"
		}
		fmt.Fprintf(&b, "  %s%s", prefix, f.Name)
		name, usage := flag.UnquoteUsage(f)
		if name != "" {
			b.WriteString(" ")
			b.WriteString(name)
		}
		// Mirror flag.PrintDefaults heuristic: short signatures get inline tab,
		// longer ones break to a new line. The "5" accounts for two-space
		// indent + "--" + one-character flag name.
		if b.Len() <= 5 {
			b.WriteString("\t")
		} else {
			b.WriteString("\n    \t")
		}
		b.WriteString(strings.ReplaceAll(usage, "\n", "\n    \t"))
		if showFlagDefault(f) {
			if isStringFlag(f) {
				fmt.Fprintf(&b, " (default %q)", f.DefValue)
			} else {
				fmt.Fprintf(&b, " (default %s)", f.DefValue)
			}
		}
		fmt.Fprintln(out, b.String())
	})
}

// showFlagDefault reports whether PrintDefaults would include a "(default ...)"
// trailer for f. Mirrors the unexported flag.isZeroValue check using the
// public surface (defaults that are the zero value for the underlying type
// are omitted).
func showFlagDefault(f *flag.Flag) bool {
	if f.DefValue == "" {
		return false
	}
	if getter, ok := f.Value.(flag.Getter); ok {
		switch v := getter.Get().(type) {
		case bool:
			return v
		case string:
			return v != ""
		case int, int64, uint, uint64, float64:
			return f.DefValue != "0"
		}
	}
	return f.DefValue != "" && f.DefValue != "0" && f.DefValue != "false"
}

// isStringFlag returns true when f wraps a string value; used to quote the
// default in usage output, matching flag.PrintDefaults behavior.
func isStringFlag(f *flag.Flag) bool {
	if getter, ok := f.Value.(flag.Getter); ok {
		_, ok := getter.Get().(string)
		return ok
	}
	return false
}

// cliMCPConfig builds an mcpConfig from common CLI flags.
func cliMCPConfig(templatesDir, outputDir string) *mcpConfig {
	return &mcpConfig{
		templatesDir: templatesDir,
		outputDir:    outputDir,
		cliCaller:    true,
		cfg:          config.DefaultConfig(),
		cache:        template.NewMemoryCache(24 * time.Hour),
		idempotency:  newIdempotencyCache(idempotencyCacheTTL),
		loopSessions: newLoopSessionStore(loopSessionTTL),
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

// cliResultText returns the JSON text of an MCP CallToolResult, or "" when the
// result carries none.
func cliResultText(result *mcpgo.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	tc, ok := result.Content[0].(mcpgo.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}

// errCLIReported marks a failure whose detail is already on stdout as a
// finding envelope; main prints only a one-line pointer for it on stderr.
var errCLIReported = errors.New("failed — see the finding envelope on stdout")

// cliErrorEnvelopeText rewrites a shared tool handler's error envelope for a
// CLI caller: the handlers stamp "subcommand":"mcp", which is the wrong name
// for someone who ran `json2pptx render-thumbnails` (go-slide-creator-pikfw).
func cliErrorEnvelopeText(text string) string {
	if cliCurrentCommand == "" {
		return text
	}
	// The envelope is compact JSON with "subcommand" in its header, ahead of
	// any finding text, so the first occurrence is the field itself.
	return strings.Replace(text, `"subcommand":"mcp"`, `"subcommand":`+string(jsonStringRaw(cliCurrentCommand)), 1)
}

// printMCPResultJSON extracts the JSON text from an MCP CallToolResult and
// pretty-prints it to stdout. An error result is a result too: its finding
// envelope goes to stdout like every other command's output, stamped with the
// CLI subcommand name, and the returned error makes the process exit non-zero.
func printMCPResultJSON(result *mcpgo.CallToolResult) error {
	if result == nil {
		return fmt.Errorf("nil result")
	}

	if result.IsError {
		if text := cliResultText(result); text != "" {
			fmt.Fprintln(os.Stdout, cliErrorEnvelopeText(text))
		}
		return errCLIReported
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

// cliPrintJSON writes v to stdout as one line of JSON without HTML escaping,
// so a "<placeholder>" in a command line reads as written.
func cliPrintJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
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

// readJSONObject reads a JSON file and parses it into an untyped object
// suitable for passing as an MCP object parameter.
func readJSONObject(path string) (any, error) {
	raw, err := readJSONInput(path)
	if err != nil {
		return nil, err
	}
	var obj any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return obj, nil
}

// jsonStringRaw encodes s as a JSON string, producing a json.RawMessage like
// `"accent1"`. Using json.Marshal instead of fmt.Sprintf prevents a crafted s
// containing '"' from producing malformed JSON.
func jsonStringRaw(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return json.RawMessage(b)
}
