package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

// runRecommendPattern implements the "recommend-pattern" CLI subcommand.
// It outputs the same response as the recommend_pattern MCP tool.
func runRecommendPattern() error {
	fs := flag.NewFlagSet("recommend-pattern", flag.ContinueOnError)

	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates")
	intent := fs.String("intent", "", "Natural language description of what you want to show (required)")
	contentHints := fs.String("content-hints", "", "Optional structured ranking hints as inline JSON or @path/to/file.json (e.g. '{\"item_count\":3,\"has_metrics\":true}')")
	recentPatterns := fs.String("recent-patterns", "", "Comma-separated pattern names used on preceding slides (used with --prefer-variety for diversity scoring)")
	preferVariety := fs.Bool("prefer-variety", false, "Penalize recently-used patterns and inject a diversity bonus candidate")
	slideIndex := fs.Int("slide-index", -1, "0-based index of the slide being built; pass a non-negative value to forward it")
	candidates := fs.String("candidates", "", "Comma-separated explicit shortlist of pattern names to rank against intent/hints (when set, ALL listed names are scored and returned ranked)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx recommend-pattern --intent <description> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Recommend named patterns that match a given intent.\n\n")
		fmt.Fprintf(os.Stderr, "This subcommand is at parity with the recommend_pattern MCP tool: every\n")
		fmt.Fprintf(os.Stderr, "ranking-context parameter accepted over MCP (content_hints, recent_patterns,\n")
		fmt.Fprintf(os.Stderr, "prefer_variety, slide_index, candidates) has a matching CLI flag, and the\n")
		fmt.Fprintf(os.Stderr, "JSON response shape is identical.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern --intent \"show 3 KPIs\"\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern --intent \"compare two options side by side\"\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern --intent \"3 cards\" --content-hints '{\"item_count\":3}'\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern --intent \"summary\" --recent-patterns kpi-3up,card-grid --prefer-variety\n")
		fmt.Fprintf(os.Stderr, "  json2pptx recommend-pattern --intent \"compare vendors\" --candidates comparison-2col,matrix-2x2\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	if *intent == "" {
		fs.Usage()
		return fmt.Errorf("--intent is required")
	}

	mc := cliMCPConfig(*templatesDir, "")

	args := map[string]any{
		"intent": *intent,
	}

	if *contentHints != "" {
		hints, err := parseContentHintsArg(*contentHints)
		if err != nil {
			return fmt.Errorf("recommend-pattern: %w", err)
		}
		args["content_hints"] = hints
	}

	if recent := splitCSVNonEmpty(*recentPatterns); len(recent) > 0 {
		args["recent_patterns"] = stringsToAnySlice(recent)
	}

	if *preferVariety {
		args["prefer_variety"] = true
	}

	if *slideIndex >= 0 {
		args["slide_index"] = float64(*slideIndex)
	}

	if cands := splitCSVNonEmpty(*candidates); len(cands) > 0 {
		args["candidates"] = stringsToAnySlice(cands)
	}

	result, err := mc.handleRecommendPattern(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("recommend-pattern: %w", err)
	}

	return printMCPResultJSON(result)
}

// parseContentHintsArg accepts inline JSON or @path/to/file.json and returns
// the parsed object suitable for the recommend_pattern tool's content_hints
// parameter. Mirrors the resolve-theme --override behavior.
func parseContentHintsArg(arg string) (map[string]any, error) {
	var raw string
	if strings.HasPrefix(arg, "@") {
		path := strings.TrimPrefix(arg, "@")
		if path == "" {
			return nil, fmt.Errorf("--content-hints: empty path after '@'")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("--content-hints: failed to read %s: %w", path, err)
		}
		raw = string(data)
	} else {
		raw = arg
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("--content-hints: invalid JSON: %w", err)
	}
	return obj, nil
}

// splitCSVNonEmpty splits a comma-separated flag value into trimmed, non-empty
// tokens. Returns nil when the input has no tokens.
func splitCSVNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// stringsToAnySlice converts []string to []any so the slice survives the
// JSON-marshal round-trip the MCP handler performs on array arguments.
func stringsToAnySlice(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}
