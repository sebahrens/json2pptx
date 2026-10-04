package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestSkillCLIMapNamesRealCommands is the go-slide-creator-4eu2o guard on the
// MCP-to-CLI table: it is generated from the tool classifications, and every
// tool of every profile either maps to a CLI command that exists, with flags
// that command accepts, or is marked MCP-only with the reason. The table the
// installed skill used to carry was wrong for fourteen tools.
func TestSkillCLIMapNamesRealCommands(t *testing.T) {
	cache := map[string]map[string]bool{}
	commandExists := func(row cliMapRow, line string) {
		t.Helper()
		invs := extractDocInvocations(line)
		if len(invs) == 0 {
			t.Errorf("%s: %q names no json2pptx command", row.Tool, line)
			return
		}
		for _, inv := range invs {
			key := strings.Join(inv.cmd, " ")
			flags, seen := cache[key]
			if !seen {
				var ok bool
				if flags, ok = cliFlagSet(t, inv.cmd); !ok {
					t.Errorf("%s: `json2pptx %s` is not a CLI command", row.Tool, key)
				}
				cache[key] = flags
			}
			for _, f := range inv.flags {
				if flags != nil && !flags[f] {
					t.Errorf("%s: `json2pptx %s` has no flag --%s", row.Tool, key, f)
				}
			}
		}
	}
	defaults := cliMapRows(toolProfileDeckSpec)
	if len(defaults) != len(deckSpecToolNames) {
		t.Fatalf("the default map has %d rows, the profile %d tools", len(defaults), len(deckSpecToolNames))
	}
	for _, profile := range []string{toolProfileDeckSpec, toolProfileCore, toolProfileAll} {
		for _, row := range cliMapRows(profile) {
			if row.MCPOnly {
				if row.MCPOnlyReason == "" {
					t.Errorf("%s is MCP-only with no reason", row.Tool)
				}
				// The closest workflow it names must still exist.
				if strings.Contains(row.CLI, "json2pptx ") {
					commandExists(row, row.CLI)
				}
				continue
			}
			if row.CLICounterpart == "" {
				t.Errorf("%s has neither a CLI counterpart nor an MCP-only reason", row.Tool)
				continue
			}
			commandExists(row, row.CLI)
			commandExists(row, "json2pptx "+row.CLICounterpart)
			if row.CLIThen != "" {
				commandExists(row, row.CLIThen)
			}
			// One command per field: a second one behind a shell comment was
			// printed inside the first one's code span (go-slide-creator-kkixz).
			if strings.Contains(row.CLI+row.CLIThen, "#") {
				t.Errorf("%s: a command carries a shell comment: %q / %q", row.Tool, row.CLI, row.CLIThen)
			}
		}
	}

	bin := sharedTestBinary(t)
	out, err := exec.Command(bin, "skill", "cli-map", "--format", "md").Output() //nolint:gosec // the test binary with fixed arguments
	if err != nil {
		t.Fatal(err)
	}
	table := string(out)
	for _, name := range deckSpecToolNames {
		if !strings.Contains(table, "| `"+name+"` |") {
			t.Errorf("skill cli-map --format md has no row for %s", name)
		}
	}
	if !strings.Contains(table, "| `list_slide_kinds` | `json2pptx semantic kinds`, then `json2pptx semantic kinds <kind>` |") {
		t.Errorf("list_slide_kinds must list its two commands, each in its own code span:\n%s", table)
	}
	if !strings.Contains(table, "`json2pptx semantic render <deck.yaml> --out <deck.pptx>`") || !strings.Contains(table, "| `submit_visual_review` | MCP-only.") {
		t.Errorf("skill cli-map table:\n%s", table)
	}
	raw, err := exec.Command(bin, "skill", "cli-map", "--tools", "all").Output() //nolint:gosec // the test binary with fixed arguments
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Profile string      `json:"profile"`
		Tools   []cliMapRow `json:"tools"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || doc.Profile != toolProfileAll || len(doc.Tools) <= len(defaults) {
		t.Errorf("skill cli-map --tools all: %v, profile %q, %d tools", err, doc.Profile, len(doc.Tools))
	}
}
