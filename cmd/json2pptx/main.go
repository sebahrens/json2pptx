// Package main provides a CLI for JSON to PPTX conversion.
package main

import (
	"fmt"
	"os"
)

var (
	// Version is the release version, set at build time via -ldflags.
	Version = "dev"
	// CommitSHA is the git commit hash, set at build time via -ldflags.
	CommitSHA = "unknown"
	// BuildTime is the build timestamp, set at build time via -ldflags.
	BuildTime = "unknown"
)

func main() {
	if err := dispatch(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func dispatch() error {
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	subcmd := os.Args[1]

	// Shift args so each subcommand sees its own flags
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "generate":
		return runGenerate()
	case "serve":
		return runServe()
	case "mcp":
		return runMCP()
	case "validate":
		return runValidate()
	case "validate-template":
		return runValidateTemplate()
	case "skill-info":
		return runSkillInfo()
	case "version", "--version", "-V":
		return runVersion()
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		// Backward compatibility: if first arg is a flag, treat as implicit "generate" mode
		if len(subcmd) > 0 && subcmd[0] == '-' {
			os.Args = append([]string{os.Args[0], subcmd}, os.Args[1:]...)
			return runGenerate()
		}
		return fmt.Errorf("unknown command %q — run 'json2pptx help' for usage", subcmd)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx <command> [options]

Commands:
  generate           Convert JSON to PPTX (default if omitted)
  validate           Validate input without generating
  validate-template  Check template compatibility
  skill-info         Show template capabilities for Claude Code skill
  serve              Start HTTP API server
  mcp                Start MCP (Model Context Protocol) server over stdio
  version            Show version information
  help               Show this help

Examples:
  json2pptx generate -template corporate slides.json
  json2pptx -template corporate slides.json          (implicit generate)
  json2pptx validate slides.json
  json2pptx validate-template templates/corporate.pptx
  json2pptx skill-info --templates-dir ./templates
  json2pptx serve --port 3000
  json2pptx mcp --templates-dir ./templates --output ./output

Run 'json2pptx <command> -h' for command-specific help.
`)
}
