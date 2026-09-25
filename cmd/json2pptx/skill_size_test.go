package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestGenerateDeckSkillBudgets keeps the entrypoint and mode-specific guides
// small enough to load only what a deck-authoring task actually needs.
func TestGenerateDeckSkillBudgets(t *testing.T) {
	dir := filepath.Join("..", "..", "skills", "generate-deck")
	var total int64
	for _, tc := range []struct {
		name string
		max  int64
	}{
		{"SKILL.md", 12 * 1024},
		{"DECKSPEC.md", 25 * 1024},
		{"RAW_PATH.md", 20 * 1024},
		{"TOOLS.md", 10 * 1024},
		{"FINDINGS.md", 10 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(dir, tc.name))
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() > tc.max {
				t.Errorf("%s is %d bytes, budget is %d", tc.name, info.Size(), tc.max)
			}
			total += info.Size()
		})
	}
	for _, name := range []string{"WORKFLOW.md", "RULES.md", "PATTERNS.md"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		total += info.Size()
	}
	if total > 100*1024 {
		t.Errorf("generate-deck skill bundle is %d bytes; budget is 100 KiB", total)
	}
}

var skillLinkRE = regexp.MustCompile(`\[[^]]*\]\(([^)]+)\)`)

func TestGenerateDeckSkillLocalLinksResolve(t *testing.T) {
	dir := filepath.Join("..", "..", "skills", "generate-deck")
	for _, name := range []string{"SKILL.md", "DECKSPEC.md", "RAW_PATH.md", "TOOLS.md"} {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range skillLinkRE.FindAllStringSubmatch(string(body), -1) {
				target, _, _ := strings.Cut(m[1], "#")
				if target == "" || strings.Contains(target, "://") {
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, target)); err != nil {
					t.Errorf("broken link %q: %v", m[1], err)
				}
			}
		})
	}
}
