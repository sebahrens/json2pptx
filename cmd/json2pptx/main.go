// Package main provides a CLI for JSON to PPTX conversion.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/sebahrens/json2pptx/internal/render"
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
const SchemaVersion = "4.161.0"

func main() {
	setupCLILogging()
	cliStartStdoutTap(os.Args[1:])
	err := run()
	// The private LibreOffice profile (~0.5 MB) would otherwise outlive every
	// render CLI call in TMPDIR (go-slide-creator-csclk.27).
	render.CleanupLibreOfficeProfile()
	// A JSON command that failed without writing a result gets the catch-all
	// finding envelope on stdout (go-slide-creator-pikfw).
	if written, tapped := cliStopStdoutTap(); tapped {
		cliReportFailure(os.Stdout, err, written)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run dispatches the command line. Help is a result, not a diagnostic: when the
// arguments ask for it (no command, a help command, -h / --help), everything
// the usage printers write goes to stdout and the process exits 0, for every
// subcommand (go-slide-creator-e6gsz). The usage printers write to os.Stderr —
// they also run when a required flag is missing — so the stream is swapped for
// the duration of a help request instead of threading a writer through each.
func run() error {
	if cliHelpRequested(os.Args[1:]) {
		stderr := os.Stderr
		os.Stderr = os.Stdout
		defer func() { os.Stderr = stderr }()
	}
	err := dispatch()
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func dispatch() error { //nolint:gocyclo
	if len(os.Args) < 2 {
		printUsage()
		return nil
	}

	subcmd := os.Args[1]
	switch subcmd {
	case "get-started", "capabilities", "semantic", "generate":
		// The commands an agent starts with say when its installed skill is
		// behind the binary (go-slide-creator-4eu2o).
		warnIfSkillStale()
	}

	// Shift args so each subcommand sees its own flags
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "generate":
		return runGenerate()
	case "read":
		return runRead()
	case "export":
		return runExportDeck()
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
	case "templates":
		return runTemplates()
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
	case "purge-render-cache":
		return runPurgeRenderCache()
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
	case "semantic":
		return runSemantic()
	case "skill":
		return runSkill()
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
		return cliInvalidArg("unknown command %q — run 'json2pptx help' for usage", subcmd)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage: json2pptx <command> [options]

Commands:
  generate            Convert JSON to PPTX (default if omitted)
  read                Read PPTX and output extracted content as JSON
  export              Export an existing PPTX to PDF or a speaker-notes handout
  validate            Validate input without generating
  preflight           Run every static check on a deck (stage-based, emits the finding envelope)
  validate-output     Check generated PPTX for OOXML correctness
  validate-template   Check template compatibility
  template-check      Check template conformance against spec
  examine-template    Emit a full template capability report (visual + XML + canonical roles)
  patterns            Discover, validate, and expand named patterns
  templates           List template names, one line each (under 2 KB)
  icons               List or search icon names (icons search <term>)
  preview-icon        Render a single icon spec to SVG + PNG preview
  tables              Table density and sizing reference
  skill-info          Show template capabilities for Claude Code skill
  capabilities        Show schema version, tools, features, and vocabularies
  get-started         Print the recommended call sequence for a task, with the CLI command for each step (brief|revise|validate-only)
  describe-finding    Print the agent-facing description for a single finding code
  input-schema        Print the JSON input schema
  resolve-theme       Resolve theme colors and fonts for a template
  recommend-pattern   Recommend patterns matching an intent
  preview             Preview generation plan without rendering
  preview-wireframe   Render a slide-plan wireframe (PNG) before generating
  preview-patterns    Render a local PNG gallery of every named pattern
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
  purge-render-cache  Reclaim the on-disk render cache
  template-settings   Manage named styles (list/register/delete)
  data-format-hints   Show data format hints for chart/diagram types
  shape-catalog       List available preset geometries
  audit-palette       Render PPTX to PNG and compare chart colors with theme accents/tints
  semantic            Validate/compile/render compact semantic deck specs (kinds|validate|compile|render|explain|schema)
  skill               Install the agent skills that match this binary, check the installed copy, or print the MCP-to-CLI table (install|status|cli-map)
  serve               Start HTTP API server
  mcp                 Start MCP (Model Context Protocol) server over stdio
  version             Show version information
  help                Show this help

MCP-only tools (no direct CLI subcommand — use 'json2pptx mcp'):
  make_deck           [MCP-only] Cold-start workflow facade: ONE call from an
                      outline to a validated, auto-repaired PPTX (plan → expand →
                      auto_repair). CLI workaround: assemble JSON and call
                      'json2pptx generate' manually.
  auto_repair         [MCP-only] Server-side convergence loop (generate → inspect
                      → repair) against a quality gate. CLI workaround: chain
                      'json2pptx generate' / 'validate' / 'repair' manually.
  apply_deck_patch    [MCP-only] Pure deck-JSON transform (bounded structural
                      ops). CLI workaround: edit the JSON directly or invoke
                      'json2pptx repair'.
  expand_patterns     [MCP-only] Batch-expand multiple patterns under one template
                      load. CLI workaround: loop 'json2pptx patterns expand' per
                      pattern.
  propose_repairs     [MCP-only] Translate fit/visual QA findings into ranked
                      repair_slide fix directives (no deck mutation). CLI
                      workaround: map findings to fixes manually and invoke
                      'json2pptx repair'.
  repair_slides_batch [MCP-only] Apply fixes to multiple slides in one call. CLI
                      workaround: loop 'json2pptx repair' for each slide.
  submit_visual_review [MCP-only] Record a host/manual all-slide visual review
                      verdict as quality evidence bound to the current PPTX
                      revision. CLI workaround: 'json2pptx inspect'.

CLI parity gaps (CLI accepts a subset of the matching MCP tool's parameters):
  recommend-visual    CLI takes -intent only. MCP recommend_visual also accepts
                      content_hints, recent_patterns, prefer_variety, slide_index,
                      and candidates (explicit shortlist) — call via 'json2pptx mcp'.

CLI quick path (DeckSpec, the recommended authoring path):
  json2pptx get-started                         # the steps below, with arguments
  json2pptx templates                           # template names
  json2pptx semantic kinds                      # slide kinds, one line each
  json2pptx semantic kinds <kind>               # fields, budgets, copy-ready example
  json2pptx semantic validate deck.yaml         # findings envelope
  json2pptx semantic render deck.yaml --out deck.pptx
  json2pptx render-thumbnails deck.pptx --out-dir slides/   # slide-0.png, slide-1.png, ...

Conventions (every command):
  <input>             The primary input is the first argument (older --json / --spec /
                      --pptx <file> spellings still work).
  --out <path>        Output file or directory (older --output / --output-dir still work).
  --format json|text  Output shape, where a command has both (older --json still works).
  --verbose           INFO logs on stderr. Default: warnings and errors only.
  -h, --help          Help on stdout, exit 0. Flags may come before or after arguments.
  Results (including the output path) go to stdout; logs go to stderr.

Discovery commands by output size:
  small  (<5 KB)      templates, semantic kinds, semantic kinds <kind>,
                      icons search <term>, describe-finding <code>
  medium (5-25 KB)    get-started, patterns list, patterns show <name>,
                      data-format-hints, shape-catalog
  large  (25-250 KB)  input-schema, skill-info --mode=list, capabilities,
                      icons list [--names], semantic schema, skill-info
  huge   (>250 KB)    icons list --json, skill-info --mode=full

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
