package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/testutil"
)

// Exercise the real installer outside the repository. Repository-relative links
// must not accidentally pass because documentation exists in the checkout.
func TestInstalledSkillReferencesAreSelfContained(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make unavailable")
	}
	destination := filepath.Join(t.TempDir(), "skills with spaces")
	cmd := exec.Command("make", "install-skill", "SKILL_DEST="+destination, "SKIP_SKILL=") // #nosec G204 -- fixed target and testing-owned temporary destination, no user input.
	cmd.Dir = filepath.Join("..", "..")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install skills: %v\n%s", err, output)
	}
	custom := filepath.Join(destination, "generate-deck", "personal.md")
	customBody := []byte("Personal reference [keep](../../docs/not-owned.md)\n")
	if err := os.WriteFile(custom, customBody, 0600); err != nil {
		t.Fatal(err)
	}
	repeat := exec.Command("make", "install-skill", "SKILL_DEST="+destination, "SKIP_SKILL=") // #nosec G204 -- repeat the same fixed target and testing-owned destination.
	repeat.Dir = cmd.Dir
	if output, err := repeat.CombinedOutput(); err != nil {
		t.Fatalf("repeat install: %v\n%s", err, output)
	}
	if body, err := os.ReadFile(custom); err != nil || string(body) != string(customBody) {
		t.Fatal("repeat install changed an unrelated personal guide")
	}
	for _, name := range []string{"generate-deck", "template-deck", "slide-visual-qa"} {
		if _, err := os.Stat(filepath.Join(destination, name)); err != nil {
			t.Fatal(err)
		}
	}
	queue := []string{}
	if err := filepath.WalkDir(destination, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".md") && path != custom {
			queue = append(queue, path)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range queue {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range skillLinkRE.FindAllStringSubmatch(string(body), -1) {
			target, _, _ := strings.Cut(match[1], "#")
			if target == "" || strings.Contains(target, ":") {
				continue
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), target))
			rel, err := filepath.Rel(destination, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Errorf("installed link escapes bundle: %s -> %s", path, target)
				continue
			}
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("broken installed link: %s -> %s: %v", path, target, err)
			}
		}
	}
	// Resolve and execute the real examples after installation outside the
	// checkout. Checking the Markdown links alone misses absent relative media.
	evidence := filepath.Join("tests", "quality", "evidence", "connectors", "midnight-blue")
	installed := filepath.Join(destination, "generate-deck", "references", "repository", evidence)
	for _, resource := range []string{"source-aware-evidence-route.json", "readable-source-companion-route.json", "powerpoint-slide-4.png"} {
		original, err := os.ReadFile(filepath.Join(testutil.RepoRoot(), evidence, resource))
		if err != nil {
			t.Fatal(err)
		}
		copy, err := os.ReadFile(filepath.Join(installed, resource))
		if err != nil || !bytes.Equal(copy, original) {
			t.Fatalf("installed source evidence changed or missing: %s: %v", resource, err)
		}
	}
	for _, example := range []struct {
		name   string
		slides int
	}{
		{"source-aware-evidence-route.json", 2},
		{"readable-source-companion-route.json", 3},
	} {
		t.Run(example.name, func(t *testing.T) {
			dir := t.TempDir()
			report := filepath.Join(dir, "result.json")
			if err := runJSONMode(filepath.Join(installed, example.name), report, testutil.TemplatesDir(), dir, "", false, false, "", "strict", true, "strict", "", false); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			var result JSONOutput
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if !result.Success || result.SlideCount != example.slides || result.OutputPath == "" {
				t.Fatalf("installed example did not generate its complete deck: %+v", result)
			}
			if _, err := os.Stat(result.OutputPath); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSkillStagingRejectsCanonicalSources(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {""}, {filepath.Join(repo, "skills")}} {
		cmd := exec.Command("bash", append([]string{filepath.Join(repo, "scripts", "stage-skills.sh")}, args...)...) // #nosec G204 -- repository script and hard-coded rejection cases only.
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("invalid destination accepted: %v: %s", args, output)
		}
	}
}

func TestSkipSkillInstallDoesNotWrite(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make unavailable")
	}
	destination := filepath.Join(t.TempDir(), "not-created")
	cmd := exec.Command("make", "install-skill", "SKILL_DEST="+destination, "SKIP_SKILL=1") // #nosec G204 -- fixed target and testing-owned temporary destination, no user input.
	cmd.Dir = filepath.Join("..", "..")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("skip install: %v: %s", err, output)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("skipped install created destination: %v", err)
	}
}
