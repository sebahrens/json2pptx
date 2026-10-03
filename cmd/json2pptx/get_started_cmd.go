package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// runGetStarted implements the "get-started" CLI subcommand.
// It outputs the same getStartedResponse as the get_started MCP tool, with the
// CLI command for every step added beside the tool name.
func runGetStarted() error {
	fs := flag.NewFlagSet("get-started", flag.ContinueOnError)

	task := fs.String("task", "", "Task scope: brief (new deck, default), revise (modify existing deck), validate-only (validate JSON without generating)")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates (reported in the runtime block)")
	outputDir := fs.String("output", "", "Directory decks are written to (reported in the runtime block)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx get-started [<task>] [options]\n\n")
		fmt.Fprintf(os.Stderr, "Print the recommended ordered call sequence for a task: %s.\n", strings.Join(getStartedAvailableTasks(), ", "))
		fmt.Fprintf(os.Stderr, "Each step names the MCP tool and, in \"cli\", the json2pptx command that does the\n")
		fmt.Fprintf(os.Stderr, "same thing from a shell. Output size: medium (4-16 KB).\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx get-started\n")
		fmt.Fprintf(os.Stderr, "  json2pptx get-started revise\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	args := map[string]any{
		// A CLI caller never received the MCP initialize instructions, so the
		// prose workflow is not a duplicate for them (go-slide-creator-bxve).
		"verbose": true,
	}
	// The installed skill's stamp, so the response carries skill_warning when
	// it is behind this binary (go-slide-creator-4eu2o).
	skill := checkInstalledSkill(skillInstallDir())
	if skill.InstalledVersion != "" {
		args["skill_version"] = skill.InstalledVersion
	}
	if *task != "" {
		// The MCP tool answers an unknown task with the default and a warning;
		// on a command line a mistyped task is a usage error, not something to
		// answer a different question for (go-slide-creator-e7jxr).
		known := false
		for _, t := range getStartedAvailableTasks() {
			known = known || t == *task
		}
		if !known {
			return fmt.Errorf("get-started: unknown task %q — valid tasks: %s", *task, strings.Join(getStartedAvailableTasks(), ", "))
		}
		args["task"] = *task
	}

	mc := cliMCPConfig(*templatesDir, *outputDir)
	result, err := mc.handleGetStarted(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("get-started: %w", err)
	}
	if result.IsError {
		return printMCPResultJSON(result)
	}

	var raw getStartedResponse
	text := cliResultText(result)
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return printMCPResultJSON(result)
	}
	if skill.Installed && !skill.Current && raw.SkillWarning == "" {
		raw.SkillWarning = skill.Message
	}
	pretty, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return printMCPResultJSON(result)
	}
	_, err = os.Stdout.WriteString(annotateStepsWithCLI(string(pretty)) + "\n")
	return err
}

// toolStepLine matches the `"tool": "<name>"` member of a step in indented
// JSON, with or without a trailing comma.
var toolStepLine = regexp.MustCompile(`(?m)^(\s*)"tool": "([a-z_]+)"(,?)$`)

// annotateStepsWithCLI adds a "cli" member after every "tool" member of an
// indented get-started response, naming the command line that performs the
// same step. The response is the MCP tool's, byte for byte, plus these lines:
// the CLI used to print MCP tool names and argument objects only, and the
// mapping to commands lived in the 90 KB capabilities output
// (go-slide-creator-e7jxr). Working on the indented text keeps the response's
// field order, which a decode/re-encode through a map would sort away.
func annotateStepsWithCLI(pretty string) string {
	return toolStepLine.ReplaceAllStringFunc(pretty, func(line string) string {
		m := toolStepLine.FindStringSubmatch(line)
		indent, tool, comma := m[1], m[2], m[3]
		return fmt.Sprintf("%s\"tool\": %q,\n%s\"cli\": %s%s", indent, tool, indent, jsonStringNoHTMLEscape(cliCommandForTool(tool)), comma)
	})
}

// jsonStringNoHTMLEscape encodes s as a JSON string, leaving < and > readable.
func jsonStringNoHTMLEscape(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// cliStepCommands spells out the command line for the tools get-started
// names, with the arguments a caller actually passes. Tools not listed fall
// back to their classification's CLI counterpart.
var cliStepCommands = map[string]string{
	"get_started":               "json2pptx get-started [brief|revise|validate-only]",
	"get_capabilities":          "json2pptx capabilities",
	"list_templates":            "json2pptx templates",
	"plan_deck":                 `json2pptx plan-deck "<brief>" --format deckspec`,
	"list_slide_kinds":          "json2pptx semantic kinds            # then: json2pptx semantic kinds <kind>",
	"list_deck_archetypes":      "json2pptx semantic schema",
	"validate_deck_spec":        "json2pptx semantic validate <deck.yaml>",
	"explain_deck_spec":         "json2pptx semantic explain <deck.yaml>",
	"compile_deck_spec":         "json2pptx semantic compile <deck.yaml> --out compiled.json",
	"render_deck_spec":          "json2pptx semantic render <deck.yaml> --out <deck.pptx>",
	"render_deck_thumbnails":    "json2pptx render-thumbnails <deck.pptx> --out-dir <slides-dir>",
	"render_slide_image":        "json2pptx render-slide <deck.pptx> --slide-index <n> --out <slide.png>",
	"submit_visual_review":      "(MCP-only — on the CLI, look at every PNG render-thumbnails wrote; json2pptx inspect <slides-dir> runs the automated checks)",
	"inspect_slide_images":      "json2pptx inspect <slides-dir>",
	"validate_input":            "json2pptx validate <deck.json> --fit-report --format json",
	"preview_presentation_plan": "json2pptx preview <deck.json>",
	"generate_presentation":     "json2pptx generate <deck.json> --out <deck.pptx>",
	"score_deck":                "json2pptx score <deck.json>",
	"recommend_visual":          `json2pptx recommend-visual "<slide intent>"`,
	"recommend_pattern":         `json2pptx recommend-pattern "<slide intent>"`,
	"list_patterns":             "json2pptx patterns list",
	"show_pattern":              "json2pptx patterns show <name>",
	"expand_pattern":            "json2pptx patterns expand <name> <values.json>",
	"validate_pattern":          "json2pptx patterns validate <name> <values.json>",
	"list_icons":                "json2pptx icons search <term>",
	"analyze_deck_rhythm":       "json2pptx analyze-rhythm <deck.json>",
	"read_presentation":         "json2pptx read <deck.pptx>",
	"examine_template":          "json2pptx examine-template <template.pptx>",
	"describe_finding":          "json2pptx describe-finding <code>",
}

// cliCommandForTool returns the command line that performs an MCP tool's step
// from a shell, or a note saying the tool has none.
func cliCommandForTool(tool string) string {
	if cmd, ok := cliStepCommands[tool]; ok {
		return cmd
	}
	class, ok := toolClassifications()[tool]
	if !ok {
		return "(no CLI command)"
	}
	if class.MCPOnlyReason != "" || class.CLICounterpart == "" {
		return "(MCP-only — no CLI command; start the server with 'json2pptx mcp')"
	}
	return "json2pptx " + class.CLICounterpart
}
