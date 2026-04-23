package main

import (
	"fmt"
	"os"
)

// runTables implements the "tables" subcommand with sub-subcommands.
func runTables() error {
	if len(os.Args) < 2 {
		printTablesUsage()
		return nil
	}

	subcmd := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)

	switch subcmd {
	case "guide":
		return runTablesGuide()
	case "help", "-h", "--help":
		printTablesUsage()
		return nil
	default:
		printTablesUsage()
		return fmt.Errorf("unknown tables subcommand %q", subcmd)
	}
}

func printTablesUsage() {
	fmt.Fprintf(os.Stderr, `Usage: json2pptx tables <command>

Commands:
  guide   Show table density and sizing reference

Run 'json2pptx tables <command> -h' for command-specific help.
`)
}

// runTablesGuide prints the table density reference for shape_grid tables.
func runTablesGuide() error {
	fmt.Fprint(os.Stdout, `Table Density Reference
=======================

Rules of thumb for fitting a table in a shape_grid.
Assumes auto_height: true on the table's row, bounds.height: 82, tight gaps.

Data rows  font_size  Max columns  Notes
---------  ---------  -----------  -----
1-4        12-14      6            Default font works; keep spacing generous
5-7        10-11      6            Explicit font_size required
8-10       9          6            Use bounds.y: 15, row_gap: 1-2
11-13      8          5            Tight; consider dropping a column
14-16      7          4            The last stop before splitting
17+        —          —            Split across two slides

Multiline cells eat budget. A cell with 3 text lines at font_size 8 needs
roughly the same vertical space as 3 single-line rows. If you use multiline
cells, count each line as a row when sizing.
`)
	return nil
}
