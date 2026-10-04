package json2pptx

import (
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
)

// dockerIgnored applies .dockerignore rules to a repository-relative path the
// way the Docker builder does for the patterns this repository uses: patterns
// are anchored at the context root, a pattern excludes everything beneath a
// directory it matches, "dir/**" excludes everything beneath dir, a leading
// "!" re-includes, and the last matching rule wins.
func dockerIgnored(rules []string, p string) bool {
	ignored := false
	for _, rule := range rules {
		negate := strings.HasPrefix(rule, "!")
		pat := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(rule, "!"), "/"), "/")
		matched := false
		switch {
		case strings.HasSuffix(pat, "/**"):
			matched = strings.HasPrefix(p, strings.TrimSuffix(pat, "**"))
		default:
			// The pattern may match the path itself or any parent directory.
			for cur := p; cur != "." && cur != "/" && !matched; cur = path.Dir(cur) {
				matched, _ = path.Match(pat, cur)
			}
		}
		if matched {
			ignored = !negate
		}
	}
	return ignored
}

// TestDockerContextShipsEveryEmbeddedReference keeps the image build working:
// references.go embeds files from docs/, examples/, internal/ and tests/, and
// `COPY . .` leaves out whatever .dockerignore excludes, which fails the Go
// build inside the image with "pattern …: no matching files found"
// (go-slide-creator-b94hz).
func TestDockerContextShipsEveryEmbeddedReference(t *testing.T) {
	src, err := os.ReadFile("references.go")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatal(err)
	}
	var rules []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			rules = append(rules, line)
		}
	}

	var embedded []string
	for _, m := range regexp.MustCompile(`(?m)^//go:embed (.+)$`).FindAllStringSubmatch(string(src), -1) {
		for _, p := range strings.Fields(m[1]) {
			embedded = append(embedded, strings.TrimPrefix(p, "all:"))
		}
	}
	if len(embedded) < 10 {
		t.Fatalf("found only %d embedded paths in references.go; the directive parser is out of date", len(embedded))
	}
	for _, p := range embedded {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("embedded path %s: %v", p, err)
		}
		if dockerIgnored(rules, p) {
			t.Errorf(".dockerignore excludes %s, which references.go embeds: the image build would fail; add a `!%s` rule", p, p)
		}
	}

	// The matcher itself: what the rules are meant to exclude stays excluded.
	for p, want := range map[string]bool{
		"docs/PATTERNS.md":              false,
		"docs/SCHEMA_CHANGELOG.md":      true,
		"CLAUDE.md":                     true,
		"README.md":                     false,
		"skills/generate-deck/SKILL.md": false,
	} {
		if got := dockerIgnored(rules, p); got != want {
			t.Errorf("dockerIgnored(%s) = %v, want %v", p, got, want)
		}
	}
}
