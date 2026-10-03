package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/sebahrens/json2pptx/skills"
)

// The installed skill and the binary (go-slide-creator-4eu2o).
//
// The agent-facing skill is a set of Markdown files copied to the host
// (~/.claude/skills). `make install-skill` was the only way to copy them, so a
// binary upgraded on its own left the old instructions in place: the skill
// taught a workflow the binary no longer recommended, cited flags that did not
// exist, and nothing said so.
//
//   - `json2pptx skill install` writes the skills this binary was built with.
//   - `json2pptx skill status` reports whether the installed copy matches.
//   - get-started, capabilities, semantic and generate print one line on stderr
//     when the installed generate-deck skill is older than the binary or lacks
//     files it ships.

const (
	// skillRefreshCommand is the command every stale-skill warning names.
	skillRefreshCommand = "json2pptx skill install"
	// skillDirEnv overrides where the skills are installed and looked for.
	skillDirEnv = "JSON2PPTX_SKILL_DIR"
	// skillCheckEnv set to "off" silences the stale-skill line.
	skillCheckEnv = "JSON2PPTX_SKILL_CHECK"
	// skillManifestName records, inside the installed generate-deck skill,
	// which binary wrote it.
	skillManifestName = ".json2pptx-skill.json"
	// skillRepositoryURL is where an installed guide's links into the
	// repository (docs/, examples/, internal/, tests/) point: the binary
	// carries the skills, not the repository around them.
	skillRepositoryURL = "https://github.com/sebahrens/json2pptx/blob/main/"
)

// skillManifest is written beside the installed generate-deck skill.
type skillManifest struct {
	SchemaVersion string   `json:"schema_version"`
	BinaryVersion string   `json:"binary_version"`
	Files         []string `json:"files"`
}

// skillStatus is the result of comparing an installed skill with the binary.
type skillStatus struct {
	Dir              string   `json:"dir"`
	Installed        bool     `json:"installed"`
	Current          bool     `json:"current"`
	InstalledVersion string   `json:"installed_schema_version,omitempty"`
	BinaryVersion    string   `json:"binary_schema_version"`
	MissingFiles     []string `json:"missing_files,omitempty"`
	Message          string   `json:"message,omitempty"`
	Refresh          string   `json:"refresh,omitempty"`
}

// skillInstallDir returns where skills are installed: $JSON2PPTX_SKILL_DIR,
// else ~/.claude/skills (the Makefile's SKILL_DEST default).
func skillInstallDir() string {
	if dir := strings.TrimSpace(os.Getenv(skillDirEnv)); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".claude", "skills")
}

// skillFrontmatterVersion matches the schema_version line of a SKILL.md.
var skillFrontmatterVersion = regexp.MustCompile(`(?m)^schema_version:\s*"?([0-9]+\.[0-9]+\.[0-9]+)"?\s*$`)

// repositoryLink matches a Markdown link target that leaves the skills tree
// for the repository around it.
var repositoryLink = regexp.MustCompile(`\]\(\.\./\.\./(docs|examples|internal|tests)/`)

// bundledSkillFiles lists every file of the embedded skills, slash-separated
// and relative to the skills root (generate-deck/SKILL.md, …).
func bundledSkillFiles() ([]string, error) {
	var files []string
	err := fs.WalkDir(skills.Bundle(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// installSkills writes the embedded skills into dest, one directory per
// skill, and the manifest. Files it does not ship are left alone.
func installSkills(dest string) (skillManifest, error) {
	manifest := skillManifest{SchemaVersion: SchemaVersion, BinaryVersion: Version}
	if strings.TrimSpace(dest) == "" {
		return manifest, fmt.Errorf("no destination: set --dest or %s", skillDirEnv)
	}
	files, err := bundledSkillFiles()
	if err != nil {
		return manifest, err
	}
	for _, name := range files {
		data, err := fs.ReadFile(skills.Bundle(), name)
		if err != nil {
			return manifest, err
		}
		// Only a skill's own guides are rerouted, as scripts/stage-skills.sh
		// does: its examples keep their text.
		if strings.HasSuffix(name, ".md") && strings.Count(name, "/") == 1 {
			data = repositoryLink.ReplaceAll(data, []byte("]("+skillRepositoryURL+"$1/"))
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return manifest, err
		}
		if err := os.WriteFile(target, data, 0o600); err != nil {
			return manifest, err
		}
		manifest.Files = append(manifest.Files, name)
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	if err := os.WriteFile(filepath.Join(dest, "generate-deck", skillManifestName), append(body, '\n'), 0o600); err != nil {
		return manifest, err
	}
	return manifest, nil
}

// checkInstalledSkill compares the generate-deck skill under dir with the
// binary. A directory with no generate-deck skill is "not installed", which is
// not a fault: plenty of callers never use the skill.
func checkInstalledSkill(dir string) skillStatus {
	status := skillStatus{Dir: dir, BinaryVersion: SchemaVersion}
	if dir == "" {
		return status
	}
	root := filepath.Join(dir, "generate-deck")
	body, err := os.ReadFile(filepath.Join(root, "SKILL.md")) //nolint:gosec // the skill directory the user configured
	if err != nil {
		return status
	}
	status.Installed = true
	status.Refresh = skillRefreshCommand
	if m := skillFrontmatterVersion.FindSubmatch(body); m != nil {
		status.InstalledVersion = string(m[1])
	}
	files, _ := bundledSkillFiles()
	for _, name := range files {
		rest, ok := strings.CutPrefix(name, "generate-deck/")
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rest))); err != nil {
			status.MissingFiles = append(status.MissingFiles, rest)
		}
	}
	var problems []string
	switch {
	case status.InstalledVersion == "":
		problems = append(problems, "carries no schema_version (it predates version stamps)")
	default:
		if cmp, err := compareSkillSchemaVersions(status.InstalledVersion, SchemaVersion); err == nil && cmp < 0 {
			problems = append(problems, fmt.Sprintf("is at %s, older than this binary (%s)", status.InstalledVersion, SchemaVersion))
		}
	}
	if n := len(status.MissingFiles); n > 0 {
		shown := status.MissingFiles
		if n > 4 {
			shown = append(append([]string(nil), shown[:4]...), fmt.Sprintf("… %d more", n-4))
		}
		problems = append(problems, "lacks "+strings.Join(shown, ", "))
	}
	if len(problems) == 0 {
		status.Current = true
		status.Refresh = ""
		return status
	}
	status.Message = fmt.Sprintf("the generate-deck skill installed at %s %s: run `%s`", root, strings.Join(problems, " and "), skillRefreshCommand)
	return status
}

// skillWarningOnce keeps the stale-skill line to one per process.
var skillWarningOnce sync.Once

// warnIfSkillStale prints one line on stderr when the installed skill does
// not match the binary. Stdout stays the command's result.
func warnIfSkillStale() {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(skillCheckEnv)), "off") {
		return
	}
	skillWarningOnce.Do(func() {
		if status := checkInstalledSkill(skillInstallDir()); status.Installed && !status.Current {
			fmt.Fprintln(os.Stderr, "json2pptx: "+status.Message)
		}
	})
}

// runSkill implements "skill install" and "skill status".
func runSkill() error {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		printSkillUsage()
		if len(os.Args) >= 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
			return nil
		}
		return fmt.Errorf("skill requires a subcommand: install or status")
	}
	sub := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	switch sub {
	case "install":
		return runSkillInstall()
	case "status":
		return runSkillStatus()
	default:
		printSkillUsage()
		return fmt.Errorf("unknown skill subcommand %q: want install or status", sub)
	}
}

func printSkillUsage() {
	fmt.Fprint(os.Stderr, `Usage: json2pptx skill <install|status> [--dest DIR]

  install   Write the agent skills this binary was built with (generate-deck,
            template-deck, slide-visual-qa, render-diagram) into DIR.
  status    Report whether the installed generate-deck skill matches this binary.

DIR defaults to $JSON2PPTX_SKILL_DIR, else ~/.claude/skills.
`)
}

func runSkillInstall() error {
	flags := flag.NewFlagSet("skill install", flag.ContinueOnError)
	dest := flags.String("dest", skillInstallDir(), "Directory the skills are written into (one sub-directory per skill)")
	flags.Usage = cliDefaultUsage(flags, "skill install [--dest DIR]",
		"Write the agent skills this binary was built with into DIR and stamp them with its schema version.\nLinks into the repository (docs/, examples/) point at "+skillRepositoryURL+"; `make install-skill` from a checkout installs an offline copy of those too.")
	if err := cliParse(flags, os.Args[1:]); err != nil {
		return err
	}
	manifest, err := installSkills(*dest)
	if err != nil {
		return fmt.Errorf("skill install: %w", err)
	}
	return cliPrintJSON(map[string]any{
		"installed":      true,
		"dir":            *dest,
		"schema_version": manifest.SchemaVersion,
		"file_count":     len(manifest.Files),
	})
}

func runSkillStatus() error {
	flags := flag.NewFlagSet("skill status", flag.ContinueOnError)
	dest := flags.String("dest", skillInstallDir(), "Directory the skills are installed in")
	flags.Usage = cliDefaultUsage(flags, "skill status [--dest DIR]",
		"Report whether the generate-deck skill installed in DIR matches this binary: its schema_version and the files it lacks.")
	if err := cliParse(flags, os.Args[1:]); err != nil {
		return err
	}
	return cliPrintJSON(checkInstalledSkill(*dest))
}
