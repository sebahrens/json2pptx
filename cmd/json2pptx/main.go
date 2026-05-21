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

// SchemaVersion tracks backward-incompatible changes to the JSON input schema.
// Bump the major version when fields are removed or renamed; bump the minor
// version when new fields are added; bump the patch for documentation-only
// changes. Agents compare this value across sessions to detect contract drift.
const SchemaVersion = "4.47.0"

func main() {
	if err := dispatch(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func dispatch() error { //nolint:gocyclo
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
	case "read":
		return runRead()
	case "serve":
		return runServe()
	case "mcp":
		return runMCP()
	case "validate":
		return runValidate()
	case "preflight":
		return runPreflight()
	case "validate-template":
		return runValidateTemplate()
	case "template-check":
		return runTemplateCheck()
	case "examine-template":
		return runExamineTemplate()
	case "validate-output":
		return runValidateOutput()
	case "patterns":
		return runPatterns()
	case "icons":
		return runIcons()
	case "preview-icon":
		return runPreviewIcon()
	case "tables":
		return runTables()
	case "skill-info":
		return runSkillInfo()
	case "capabilities":
		return runCapabilities()
	case "get-started":
		return runGetStarted()
	case "describe-finding":
		return runDescribeFinding()
	case "input-schema":
		return runInputSchema()
	case "resolve-theme":
		return runResolveTheme()
	case "recommend-pattern":
		return runRecommendPattern()
	case "preview":
		return runPreview()
	case "preview-wireframe":
		return runPreviewWireframe()
	case "repair":
		return runRepair()
	case "score":
		return runScore()
	case "score-candidates":
		return runScoreCandidates()
	case "inspect":
		return runInspect()
	case "analyze-rhythm":
		return runAnalyzeRhythm()
	case "plan-deck":
		return runPlanDeck()
	case "recommend-visual":
		return runRecommendVisual()
	case "render-slide":
		return runRenderSlide()
	case "render-slide-from-json":
		return runRenderSlideFromJSON()
	case "render-thumbnails":
		return runRenderThumbnails()
	case "template-settings":
		return runTemplateSettings()
	case "data-format-hints":
		return runDataFormatHints()
	case "preview-patterns":
		return runPreviewPatterns()
	case "shape-catalog":
		return runShapeCatalog()
	case "audit-palette":
		return runAuditPalette()
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
  generate            Convert JSON to PPTX (default if omitted)
  read                Read PPTX and output extracted content as JSON
  validate            Validate input without generating
  preflight           Run every static check on a deck (stage-based, emits the finding envelope)
  validate-output     Check generated PPTX for OOXML correctness
  validate-template   Check template compatibility
  template-check      Check template conformance against spec
  examine-template    Emit a full template capability report (visual + XML + canonical roles)
  patterns            Discover, validate, and expand named patterns
  icons               List available icon names
  preview-icon        Render a single icon spec to SVG + PNG preview
  tables              Table density and sizing reference
  skill-info          Show template capabilities for Claude Code skill
  capabilities        Show schema version, tools, features, and vocabularies
  get-started         Print the recommended MCP-call sequence for a task (brief|revise|validate-only)
  describe-finding    Print the agent-facing description for a single finding code
  input-schema        Print the JSON input schema (full or compact)
  resolve-theme       Resolve theme colors and fonts for a template
  recommend-pattern   Recommend patterns matching an intent
  preview             Preview generation plan without rendering
  preview-wireframe   Render a slide-plan wireframe (PNG) before generating
  preview-patterns    Pre-render PNG previews for every named pattern
  repair              Apply targeted fixes to a single slide
  score               Score a JSON deck spec for visual quality (deterministic)
  score-candidates    Rank candidate slides for one slot without rendering
  inspect             Run vision-based visual QA on rendered slide images
  analyze-rhythm      Analyze deck visual rhythm and pattern repetition
  plan-deck           Plan a deck outline from a brief
  recommend-visual    Recommend visual approaches for a slide intent
  render-slide        Render a single slide from a PPTX to PNG
  render-slide-from-json  Render one slide directly from JSON (no full deck render)
  render-thumbnails   Render all slides as PNG thumbnails
  template-settings   Manage named styles (list/register/delete)
  data-format-hints   Show data format hints for chart/diagram types
  shape-catalog       List available preset geometries
  audit-palette       Render PPTX to PNG and report ΔE between chart pics and adjacent solid-filled shapes
  serve               Start HTTP API server
  mcp                 Start MCP (Model Context Protocol) server over stdio
  version             Show version information
  help                Show this help

MCP-only tools (no direct CLI subcommand — use 'json2pptx mcp'):
  expand_patterns     [MCP-only] Batch-expand multiple patterns under one template load.
                      CLI workaround: loop 'json2pptx patterns expand' per pattern.
  propose_repairs     [MCP-only] Translate fit/visual QA findings into ranked
                      repair_slide fix directives (no deck mutation).

CLI parity gaps (CLI accepts a subset of the matching MCP tool's parameters):
  recommend-visual    CLI takes -intent only. MCP recommend_visual also accepts
                      content_hints, recent_patterns, prefer_variety, slide_index,
                      and candidates (explicit shortlist) — call via 'json2pptx mcp'.

Examples:
  json2pptx generate --json slides.json --template corporate
  json2pptx --json slides.json --template corporate     (implicit generate)
  json2pptx validate slides.json
  json2pptx validate-template templates/corporate.pptx
  json2pptx skill-info --templates-dir ./templates
  json2pptx serve --port 3000
  json2pptx mcp --templates-dir ./templates --output ./output

Flags accept both --flag and -flag (single-dash kept for back-compat).
Run 'json2pptx <command> -h' for command-specific help.
`)
}
