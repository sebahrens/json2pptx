package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/sebahrens/json2pptx/icons"
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
	case "help", "-h", "--help":
		printIconsUsage()
		return nil
	default:
		printIconsUsage()
		return fmt.Errorf("unknown icons subcommand %q", subcmd)
	}
}

func printIconsUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx icons <command> [options]

Commands:
  list    List all available icon names

Run 'json2pptx icons <command> -h' for command-specific help.
`)
}

// runIconsList lists all available icon names.
func runIconsList() error {
	fs := flag.NewFlagSet("icons list", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON array")
	set := fs.String("set", "", "Icon set to list (outline, filled). Default: all sets")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	sets := []string{"outline", "filled"}
	if *set != "" {
		sets = []string{*set}
	}

	if *jsonOutput {
		return runIconsListJSON(sets)
	}
	return runIconsListTable(sets)
}

func runIconsListTable(sets []string) error {
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return fmt.Errorf("listing %s icons: %w", s, err)
		}
		fmt.Fprintf(os.Stdout, "%s (%d icons):\n", s, len(names))
		fmt.Fprintln(os.Stdout, "  "+strings.Join(names, ", "))
		fmt.Fprintln(os.Stdout)
	}
	return nil
}

type iconSetJSON struct {
	Set   string   `json:"set"`
	Count int      `json:"count"`
	Names []string `json:"names"`
}

func runIconsListJSON(sets []string) error {
	result := make([]iconSetJSON, 0, len(sets))
	for _, s := range sets {
		names, err := icons.List(s)
		if err != nil {
			return fmt.Errorf("listing %s icons: %w", s, err)
		}
		result = append(result, iconSetJSON{
			Set:   s,
			Count: len(names),
			Names: names,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
