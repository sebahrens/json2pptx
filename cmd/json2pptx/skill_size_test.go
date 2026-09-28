package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// generateDeckSkillCaps is every guide in the generate-deck bundle with its
// own byte cap. A per-file cap stops one guide from silently eating the
// bundle budget the others need; a file missing from this table fails the
// bundle test, so a new guide cannot dodge the budget.
var generateDeckSkillCaps = []struct {
	name string
	max  int64
}{
	{"SKILL.md", 12 * 1024},
	{"QUALITY.md", 8 * 1024},
	{"DECKSPEC.md", 16 * 1024},
	{"RAW_PATH.md", 20 * 1024},
	{"TOOLS.md", 10 * 1024},
	{"FINDINGS.md", 6 * 1024},
	{"WORKFLOW.md", 16 * 1024},
	{"RULES.md", 24 * 1024},
	{"PATTERNS.md", 16 * 1024},
}

// TestGenerateDeckSkillBudgets keeps the entrypoint and mode-specific guides
// small enough to load only what a deck-authoring task actually needs.
func TestGenerateDeckSkillBudgets(t *testing.T) {
	dir := filepath.Join("..", "..", "skills", "generate-deck")
	var total int64
	capped := map[string]bool{}
	for _, tc := range generateDeckSkillCaps {
		capped[tc.name] = true
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !capped[e.Name()] {
			t.Errorf("%s has no byte cap in generateDeckSkillCaps", e.Name())
		}
	}
	if total > 100*1024 {
		t.Errorf("generate-deck skill bundle is %d bytes; budget is 100 KiB", total)
	}
}

var skillLinkRE = regexp.MustCompile(`\[[^]]*\]\(([^)]+)\)`)

func TestGenerateDeckSkillLocalLinksResolve(t *testing.T) {
	dir := filepath.Join("..", "..", "skills", "generate-deck")
	for _, tc := range generateDeckSkillCaps {
		name := tc.name
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

// TestSkillQualityGuideIsMustRead keeps the consulting-quality guide on the
// entrypoint's critical path: a guide the entrypoint does not send agents to
// is a guide nobody loads.
func TestSkillQualityGuideIsMustRead(t *testing.T) {
	skill := readRepoFile(t, filepath.Join("skills", "generate-deck", "SKILL.md"))
	if !strings.Contains(skill, "[QUALITY.md](QUALITY.md)") {
		t.Error("SKILL.md must link QUALITY.md")
	}
	quality := readRepoFile(t, filepath.Join("skills", "generate-deck", "QUALITY.md"))
	for _, want := range []string{"ghost deck", "action title", "one message", "takeaway", "source"} {
		if !strings.Contains(strings.ToLower(quality), want) {
			t.Errorf("QUALITY.md does not teach %q", want)
		}
	}
}
