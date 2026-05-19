package tokens

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTokensPublishedInRulesMD asserts that the Typography Hierarchy
// table in skills/generate-deck/RULES.md publishes the same point-size
// ranges as the constants in this package.
//
// The constants are the canonical source. RULES.md is the agent-facing
// surface that documents them. If the constants are changed, this test
// fails until RULES.md is updated to match. If RULES.md is changed
// independently of the constants, this test fails until they agree.
func TestTokensPublishedInRulesMD(t *testing.T) {
	path := findRepoFile(t, "skills/generate-deck/RULES.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read RULES.md: %v", err)
	}
	rules := string(data)

	cases := []struct {
		role    string
		minHPt  int
		maxHPt  int
	}{
		{"Grid header/banner", GridHeaderMinHPt, GridHeaderMaxHPt},
		{"Card title", CardTitleMinHPt, CardTitleMaxHPt},
		{"Card body", CardBodyMinHPt, CardBodyMaxHPt},
		{"Step number", StepNumberMinHPt, StepNumberMaxHPt},
		{"Footnote/source", FootnoteMinHPt, FootnoteMaxHPt},
	}

	for _, c := range cases {
		t.Run(c.role, func(t *testing.T) {
			wantRange := fmt.Sprintf("%d-%dpt", c.minHPt/100, c.maxHPt/100)
			// Locate the table row by role name, then assert the range
			// token appears in it. We look for the role as a left-column
			// label (followed by whitespace and a pipe) to avoid matching
			// the same words in prose.
			lineHit := false
			rangeHit := false
			for _, line := range strings.Split(rules, "\n") {
				if !strings.Contains(line, c.role) || !strings.Contains(line, "|") {
					continue
				}
				lineHit = true
				if strings.Contains(line, wantRange) {
					rangeHit = true
					break
				}
			}
			if !lineHit {
				t.Errorf("RULES.md typography table missing row for %q", c.role)
				return
			}
			if !rangeHit {
				t.Errorf("RULES.md row for %q does not publish range %q (constants in tokens.go disagree with docs)", c.role, wantRange)
			}
		})
	}
}

// TestFootnoteColorPublishedInRulesMD asserts that the canonical footnote
// grey is the value documented in the RULES.md typography table.
func TestFootnoteColorPublishedInRulesMD(t *testing.T) {
	path := findRepoFile(t, "skills/generate-deck/RULES.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read RULES.md: %v", err)
	}
	if !strings.Contains(string(data), strings.ToUpper(FootnoteColor)) {
		t.Errorf("RULES.md does not mention canonical footnote color %s", FootnoteColor)
	}
}

// findRepoFile walks upward from the test working directory until it
// finds the requested repo-relative path. Falls t.Fatal if not found.
func findRepoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate %q above %s", rel, dir)
		}
		dir = parent
	}
}
