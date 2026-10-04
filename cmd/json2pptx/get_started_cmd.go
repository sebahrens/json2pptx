package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// runGetStarted implements the "get-started" CLI subcommand.
// It outputs the same getStartedResponse as the get_started MCP tool, with the
// CLI command for every step added beside the tool name.
func runGetStarted() error {
	fs := flag.NewFlagSet("get-started", flag.ContinueOnError)

	task := fs.String("task", "", "Task scope: brief (new deck, default), revise (modify existing deck), validate-only (validate JSON without generating)")
	tool := fs.String("tool", "", "An MCP tool name: print its full description, input schema and CLI command instead of a workflow (MCP get_started tool:\"<name>\")")
	templatesDir := fs.String("templates-dir", "./templates", "Directory containing templates (reported in the runtime block)")
	outputDir := fs.String("output", "", "Directory decks are written to (reported in the runtime block)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: json2pptx get-started [<task>] [options]\n")
		fmt.Fprintf(os.Stderr, "       json2pptx get-started --tool <mcp_tool_name>\n\n")
		fmt.Fprintf(os.Stderr, "Print the recommended ordered call sequence for a task: %s.\n", strings.Join(getStartedAvailableTasks(), ", "))
		fmt.Fprintf(os.Stderr, "Each step names the MCP tool and, in \"cli\", the json2pptx command that does the\n")
		fmt.Fprintf(os.Stderr, "same thing from a shell. Output size: medium (4-16 KB).\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  json2pptx get-started\n")
		fmt.Fprintf(os.Stderr, "  json2pptx get-started revise\n")
		fmt.Fprintf(os.Stderr, "  json2pptx get-started --tool render_deck_spec   # one tool's description, schema and CLI command\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		printDoubleDashUsage(fs)
	}

	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	if *tool != "" {
		if *task != "" {
			return cliInvalidArg("get-started: --tool and a task are separate questions: name a tool OR a task, not both")
		}
		return runGetStartedTool(strings.TrimSpace(*tool), *templatesDir, *outputDir)
	}

	args := map[string]any{
		// A CLI caller never received the MCP initialize instructions, so the
		// prose workflow is not a duplicate for them (go-slide-creator-bxve).
		"verbose": true,
	}
	// The installed skill's stamp, so the response carries skill_warning when
	// it is behind this binary (go-slide-creator-4eu2o): the stamp alone, not
	// a read of every installed file (go-slide-creator-v25ae).
	skill := checkSkillStamp(skillInstallDir())
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
			return cliInvalidArg("get-started: unknown task %q — valid tasks: %s", *task, strings.Join(getStartedAvailableTasks(), ", "))
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

// cliToolDetail is `get-started --tool <name>`: the MCP tool's tool_detail
// (description and input schema, as get_started tool:"<name>" returns it)
// with the command line that does the same job from a shell.
type cliToolDetail struct {
	ToolDetail json.RawMessage `json:"tool_detail"`
	// CLI is the command, or a note that the tool is MCP-only; CLIThen the
	// follow-up command when the tool's job takes two.
	CLI         string          `json:"cli"`
	CLIThen     string          `json:"cli_then,omitempty"`
	HiddenTools json.RawMessage `json:"hidden_tools,omitempty"`
}

// runGetStartedTool prints one tool's detail (go-slide-creator-kkixz). The
// default MCP profile abridges tools/list and serves the full text through
// get_started tool:"<name>"; the CLI had no way to ask.
func runGetStartedTool(name, templatesDir, outputDir string) error {
	if _, known := toolConstructors()[name]; !known {
		names := mcpToolNames()
		sort.Strings(names)
		return cliInvalidArg("get-started: unknown tool %q — tools: %s", name, strings.Join(names, ", "))
	}
	mc := cliMCPConfig(templatesDir, outputDir)
	result, err := mc.handleGetStarted(context.Background(), mcpRequestWithArgs(map[string]any{"tool": name}))
	if err != nil {
		return fmt.Errorf("get-started: %w", err)
	}
	if result.IsError {
		return printMCPResultJSON(result)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(cliResultText(result)), &raw); err != nil {
		return printMCPResultJSON(result)
	}
	out := cliToolDetail{
		ToolDetail:  raw["tool_detail"],
		CLI:         cliCommandForTool(name),
		CLIThen:     cliStepFollowUps[name],
		HiddenTools: raw["hidden_tools"],
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return printMCPResultJSON(result)
	}
	_, err = os.Stdout.WriteString(b.String())
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
		cli := jsonStringNoHTMLEscape(cliCommandForTool(tool))
		if then := cliStepFollowUps[tool]; then != "" {
			cli += fmt.Sprintf(",\n%s\"cli_then\": %s", indent, jsonStringNoHTMLEscape(then))
		}
		return fmt.Sprintf("%s\"tool\": %q,\n%s\"cli\": %s%s", indent, tool, indent, cli, comma)
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
	"list_slide_kinds":          "json2pptx semantic kinds",
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

// cliStepFollowUps is the second command of a tool whose job takes two on the
// CLI. It used to ride in the first command's string behind a shell comment
// ("… # then: …"), which the cli-map table printed inside one code span
// (go-slide-creator-kkixz).
var cliStepFollowUps = map[string]string{
	"list_slide_kinds": "json2pptx semantic kinds <kind>",
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
