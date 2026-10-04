package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/svggen/icons"
)

// runIcons implements the "icons" subcommand with sub-subcommands.
func runIcons() error {
	if len(os.Args) < 2 {
		printIconsUsage()
		return nil
	}

	subcmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "list":
		return runIconsList()
	case "search":
		return runIconsSearch()
	case "help", "-h", "--help":
		printIconsUsage()
		return nil
	default:
		printIconsUsage()
		return cliInvalidArg("unknown icons subcommand %q", subcmd)
	}
}

func printIconsUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx icons <command> [options]

Commands:
  search <term>   Find icons by name or business concept (small: a handful of names)
  list            List all available icon names (large: ~135 KB; --names is the
                  names-only form, ~21 KB for --set filled; --json is huge, ~740 KB)

Run 'json2pptx icons <command> -h' for command-specific help.
`)
}

// runIconsList lists all available icon names.
func runIconsList() error {
	fs := flag.NewFlagSet("icons list", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON array")
	set := fs.String("set", "", "Icon set to list (outline, filled). Default: all sets")
	namesOnly := fs.Bool("names", false, "Names only: one qualified name (<set>:<name>) per line; with --format json, a flat array of names")
	fs.Usage = cliDefaultUsage(fs, "icons list [--names] [--set outline|filled] [--format json]",
		"List icon names. Output size: large (~135 KB of text; --names --set filled is ~21 KB) and\nhuge as JSON (~740 KB). To find one icon use 'json2pptx icons search <term>' instead.")
	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}

	sets := []string{"outline", "filled"}
	if *set != "" {
		sets = []string{*set}
	}

	if *namesOnly {
		return runIconsListNames(sets, *jsonOutput)
	}
	if *jsonOutput {
		return runIconsListJSON(sets)
	}
	return runIconsListTable(sets)
}

// runIconsListNames prints the names-only listing: nothing but the
// identifiers an author pastes into icon.name.
func runIconsListNames(sets []string, asJSON bool) error {
	qualified := make([]string, 0, 4096)
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return fmt.Errorf("listing %s icons: %w", s, err)
		}
		for _, n := range names {
			qualified = append(qualified, s+":"+n)
		}
	}
	if asJSON {
		return cliPrintJSON(qualified)
	}
	_, err := fmt.Fprintln(os.Stdout, strings.Join(qualified, "\n"))
	return err
}

// iconSearchResult is the `icons search --format json` shape.
type iconSearchResult struct {
	Query      string   `json:"query"`
	Names      []string `json:"names"`
	TotalCount int      `json:"total_count"`
	// MatchedVia is "name", "synonym" or "synonym+name" (see list_icons).
	MatchedVia string `json:"matched_via,omitempty"`
	// Concepts is the business-concept vocabulary, present only when the
	// query matched nothing.
	Concepts []string `json:"concepts,omitempty"`
}

// runIconsSearch implements "icons search <term>": the same name +
// business-concept search list_icons(filter=...) runs, reduced to the
// qualified names an author needs (go-slide-creator-l7tg3).
func runIconsSearch() error {
	fs := flag.NewFlagSet("icons search", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	set := fs.String("set", "", "Icon set to search (outline, filled). Default: all sets")
	limit := fs.Int("limit", 20, "Maximum number of names to print")
	fs.Usage = cliDefaultUsage(fs, "icons search <term> [--limit N] [--set outline|filled] [--format json]",
		"Find icons whose name contains <term>, or that the business-concept index maps to it\n(\"risk\", \"growth\", \"governance\"). Output size: small (qualified names, one per line).")
	if err := cliParse(fs, os.Args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return cliMissingArg("icons search: exactly one <term> is required")
	}
	term := fs.Arg(0)
	if *limit < 1 {
		*limit = 1
	}

	args := map[string]any{"filter": term, "page_size": float64(*limit)}
	if *set != "" {
		args["set"] = *set
	}
	result, err := handleListIcons(context.Background(), mcpRequestWithArgs(args))
	if err != nil {
		return fmt.Errorf("icons search: %w", err)
	}
	if result.IsError {
		return printMCPResultJSON(result)
	}
	var resp listIconsResponse
	if err := json.Unmarshal([]byte(cliResultText(result)), &resp); err != nil {
		return fmt.Errorf("icons search: %w", err)
	}
	out := iconSearchResult{Query: term, Names: []string{}, TotalCount: resp.TotalCount, MatchedVia: resp.MatchedVia, Concepts: resp.Concepts}
	for _, s := range resp.Sets {
		for _, n := range s.Names {
			out.Names = append(out.Names, s.Set+":"+n)
		}
	}
	if *jsonOutput {
		return cliPrintJSON(out)
	}
	if len(out.Names) == 0 {
		fmt.Fprintf(os.Stdout, "no icon matches %q; concepts the index knows: %s\n", term, strings.Join(out.Concepts, ", "))
		return nil
	}
	fmt.Fprintln(os.Stdout, strings.Join(out.Names, "\n"))
	if out.TotalCount > len(out.Names) {
		fmt.Fprintf(os.Stdout, "(%d of %d matches; raise --limit for more)\n", len(out.Names), out.TotalCount)
	}
	return nil
}

func runIconsListTable(sets []string) error {
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return fmt.Errorf("listing %s icons: %w", s, err)
		}
		fmt.Fprintf(os.Stdout, "%s (%d icons; use as %s:<name>):\n", s, len(names), s)
		qualified := make([]string, len(names))
		for i, n := range names {
			qualified[i] = s + ":" + n
		}
		fmt.Fprintln(os.Stdout, "  "+strings.Join(qualified, ", "))
		fmt.Fprintln(os.Stdout)
	}
	return nil
}

// iconEntryJSON is the per-icon record in the CLI JSON output. qualified_name
// is the canonical authoring identifier (always "<set>:<name>") suitable for
// dropping into icon.name in deck JSON.
type iconEntryJSON struct {
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
}

// iconSetJSON is the per-set JSON shape from `icons list --json`. `names` is
// preserved for backward compatibility; new consumers should use
// icons[].qualified_name.
type iconSetJSON struct {
	Set   string          `json:"set"`
	Count int             `json:"count"`
	Names []string        `json:"names"`
	Icons []iconEntryJSON `json:"icons"`
}

func runIconsListJSON(sets []string) error {
	result := make([]iconSetJSON, 0, len(sets))
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return fmt.Errorf("listing %s icons: %w", s, err)
		}
		entries := make([]iconEntryJSON, len(names))
		for i, n := range names {
			entries[i] = iconEntryJSON{Name: n, QualifiedName: s + ":" + n}
		}
		result = append(result, iconSetJSON{
			Set:   s,
			Count: len(names),
			Names: names,
			Icons: entries,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
