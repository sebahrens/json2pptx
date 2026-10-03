package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillInstallMatchesTheBinary is the go-slide-creator-4eu2o acceptance
// test for the installer: the binary writes the skills it was built with,
// stamped with its schema version, with no link left pointing into a
// repository that is not there, and `skill status` then reports them current.
func TestSkillInstallMatchesTheBinary(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "skills with spaces")
	manifest, err := installSkills(dest)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != SchemaVersion || len(manifest.Files) == 0 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, name := range []string{"generate-deck", "template-deck", "slide-visual-qa", "render-diagram"} {
		if _, err := os.Stat(filepath.Join(dest, name, "SKILL.md")); err != nil {
			t.Errorf("skill %s was not installed: %v", name, err)
		}
	}
	// The files the journey's stale copy lacked.
	for _, name := range []string{"QUALITY.md", "DECKSPEC.md", "RAW_PATH.md", "WORKFLOW.md", skillManifestName} {
		if _, err := os.Stat(filepath.Join(dest, "generate-deck", name)); err != nil {
			t.Errorf("generate-deck/%s was not installed: %v", name, err)
		}
	}
	// Every installed guide is byte-identical to the source apart from its
	// links into the repository, and none of those is left relative.
	source := filepath.Join("..", "..", "skills")
	for _, name := range manifest.Files {
		installed, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.ReadFile(filepath.Join(source, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(name, ".md") || strings.Count(name, "/") != 1 {
			if !bytes.Equal(installed, original) {
				t.Errorf("%s changed on install", name)
			}
			continue
		}
		if repositoryLink.Match(installed) {
			t.Errorf("%s still links into a repository checkout", name)
		}
		if want := repositoryLink.ReplaceAll(original, []byte("]("+skillRepositoryURL+"$1/")); !bytes.Equal(installed, want) {
			t.Errorf("%s differs from the source beyond its repository links", name)
		}
	}

	status := checkInstalledSkill(dest)
	if !status.Installed || !status.Current || status.InstalledVersion != SchemaVersion || status.Message != "" {
		t.Errorf("a fresh install is not current: %+v", status)
	}

	// Installing again leaves a file the user added alone.
	personal := filepath.Join(dest, "generate-deck", "personal.md")
	if err := os.WriteFile(personal, []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installSkills(dest); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(personal); err != nil || string(body) != "mine\n" {
		t.Error("a second install touched a file it does not ship")
	}
}

// staleSkillDir writes a generate-deck skill as an older install left it.
func staleSkillDir(t *testing.T, frontmatter string, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "generate-deck")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: generate-deck\n"+frontmatter+"---\n# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(root, f), []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestInstalledSkillCheck(t *testing.T) {
	if status := checkInstalledSkill(t.TempDir()); status.Installed || status.Message != "" {
		t.Errorf("a directory with no skill is not a stale skill: %+v", status)
	}

	// No version stamp and files missing: the copy the journey found.
	unstamped := checkInstalledSkill(staleSkillDir(t, "", "TOOLS.md", "WORKFLOW.md"))
	if unstamped.Current || !strings.Contains(unstamped.Message, "no schema_version") ||
		!strings.Contains(unstamped.Message, "QUALITY.md") || !strings.Contains(unstamped.Message, skillRefreshCommand) {
		t.Errorf("unstamped skill: %+v", unstamped)
	}

	older := checkInstalledSkill(staleSkillDir(t, "schema_version: 1.2.3\n"))
	if older.Current || older.InstalledVersion != "1.2.3" || !strings.Contains(older.Message, "older than this binary") {
		t.Errorf("older skill: %+v", older)
	}

	// A copy from a newer binary is not this binary's business.
	dest := t.TempDir()
	if _, err := installSkills(dest); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(dest, "generate-deck", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	newer := strings.Replace(string(body), "schema_version: "+SchemaVersion, "schema_version: 999.0.0", 1)
	if newer == string(body) {
		t.Fatal("the installed SKILL.md carries no schema_version line to rewrite")
	}
	if err := os.WriteFile(skill, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}
	if status := checkInstalledSkill(dest); !status.Current {
		t.Errorf("a newer skill is reported stale: %+v", status)
	}
}

// Every command an agent starts with says, once and on stderr, that the
// installed skill is behind — and stdout stays the command's own result.
func TestCLIEntryPointsWarnAboutAStaleSkill(t *testing.T) {
	bin := sharedTestBinary(t)
	stale := staleSkillDir(t, "schema_version: 1.2.3\n")
	work := t.TempDir()
	deck := filepath.Join(work, "deck.json")
	if err := os.WriteFile(deck, []byte(`{"template":"midnight-blue","slides":[{"layout_id":"title","content":[{"placeholder_id":"title","type":"text","text_value":"Stale skill check"}]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	templates, err := filepath.Abs(filepath.Join("..", "..", "templates"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(env []string, args ...string) (stdout, stderr string) {
		cmd := exec.Command(bin, args...) //nolint:gosec // the test binary with fixed arguments
		cmd.Dir = work
		cmd.Env = append(os.Environ(), env...)
		var out, errOut bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errOut
		if err := cmd.Run(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, errOut.String())
		}
		return out.String(), errOut.String()
	}
	staleEnv := []string{skillDirEnv + "=" + stale, skillCheckEnv + "="}
	for _, args := range [][]string{
		{"get-started"},
		{"capabilities"},
		{"semantic", "kinds"},
		{"generate", deck, "--templates-dir", templates, "--out", filepath.Join(work, "out")},
	} {
		_, stderr := run(staleEnv, args...)
		if n := strings.Count(stderr, skillRefreshCommand); n != 1 {
			t.Errorf("%v: stderr names the refresh command %d times, want once:\n%s", args, n, stderr)
		}
		if !strings.Contains(stderr, "older than this binary") {
			t.Errorf("%v: stderr does not say the skill is older:\n%s", args, stderr)
		}
	}

	// get-started also carries it in the result, for a caller that reads stdout.
	stdout, _ := run(staleEnv, "get-started")
	var started struct {
		SkillWarning string `json:"skill_warning"`
	}
	if err := json.Unmarshal([]byte(stdout), &started); err != nil {
		t.Fatalf("get-started stdout is not one JSON document: %v", err)
	}
	if !strings.Contains(started.SkillWarning, skillRefreshCommand) {
		t.Errorf("get-started skill_warning = %q", started.SkillWarning)
	}

	// A matching install, or no install, says nothing.
	fresh := t.TempDir()
	run(nil, "skill", "install", "--dest", fresh)
	for _, dir := range []string{fresh, t.TempDir()} {
		if _, stderr := run([]string{skillDirEnv + "=" + dir, skillCheckEnv + "="}, "semantic", "kinds"); strings.Contains(stderr, skillRefreshCommand) {
			t.Errorf("a current or absent skill drew the warning:\n%s", stderr)
		}
	}
	status, _ := run(nil, "skill", "status", "--dest", stale)
	if !strings.Contains(status, `"current":false`) || !strings.Contains(status, skillRefreshCommand) {
		t.Errorf("skill status of a stale copy: %s", status)
	}
}
