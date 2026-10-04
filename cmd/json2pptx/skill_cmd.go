package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	json2pptx "github.com/sebahrens/json2pptx"
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
//   - `json2pptx skill install` writes the skills this binary was built with,
//     and beside them the repository files they link to
//     (generate-deck/references/repository/), so every link resolves on a
//     machine with no checkout. `make install-skill` and the install scripts
//     run this same command.
//   - Every Markdown file it writes ends in a version stamp, and
//     `json2pptx skill status` checks each installed file against the binary.
//   - get-started, capabilities, semantic and generate print one line on stderr
//     when the installed generate-deck skill is older than the binary: once
//     per mismatch, remembered in a state file. They read the install
//     manifest's stamp and nothing else; the file-by-file comparison is
//     `skill status` (go-slide-creator-v25ae).
//   - `json2pptx skill cli-map` prints the MCP-tool → CLI-command table from
//     the classifications get_capabilities serves, so no skill has to carry a
//     hand-maintained copy.

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
	// skillWarnedName remembers, beside the manifest, the mismatch the user
	// has already been told about.
	skillWarnedName = ".json2pptx-skill-warned"
	// skillReferencesDir is where the repository files the skills link to are
	// installed, repository-shaped so links between them keep resolving.
	skillReferencesDir = "generate-deck/references/repository"
	// skillStampPrefix opens the last line of every installed Markdown file.
	skillStampPrefix = "<!-- json2pptx-skill schema_version: "
)

// skillManifest is written beside the installed generate-deck skill.
type skillManifest struct {
	SchemaVersion string   `json:"schema_version"`
	BinaryVersion string   `json:"binary_version"`
	Files         []string `json:"files"`
}

// skillStatus is the result of comparing an installed skill with the binary.
type skillStatus struct {
	Dir              string `json:"dir"`
	Installed        bool   `json:"installed"`
	Current          bool   `json:"current"`
	InstalledVersion string `json:"installed_schema_version,omitempty"`
	BinaryVersion    string `json:"binary_schema_version"`
	// FilesChecked counts the files this binary ships that were looked at.
	FilesChecked int `json:"files_checked,omitempty"`
	// MissingFiles are shipped files the installed copy lacks.
	MissingFiles []string `json:"missing_files,omitempty"`
	// UnstampedFiles are installed Markdown files with no version stamp:
	// copied by hand or by an installer that predates the stamps.
	UnstampedFiles []string `json:"unstamped_files,omitempty"`
	// StaleFiles are installed Markdown files stamped with an older version.
	StaleFiles []string `json:"stale_files,omitempty"`
	// ChangedFiles are installed non-Markdown files (examples, images) whose
	// bytes differ from the ones this binary ships.
	ChangedFiles []string `json:"changed_files,omitempty"`
	Message      string   `json:"message,omitempty"`
	Refresh      string   `json:"refresh,omitempty"`
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

// skillStampLine matches the version stamp that ends an installed file.
var skillStampLine = regexp.MustCompile(`<!-- json2pptx-skill schema_version: ([0-9]+\.[0-9]+\.[0-9]+) -->\s*$`)

// repositoryLink matches a Markdown link target that leaves the skills tree
// for the repository around it.
var repositoryLink = regexp.MustCompile(`\]\(\.\./\.\./(docs|examples|internal|tests)/`)

// skillStamp is the line appended to every installed Markdown file.
func skillStamp() string { return skillStampPrefix + SchemaVersion + " -->" }

// stampSkillFile appends the version stamp to a Markdown file's bytes. An
// HTML comment renders as nothing and, at the end of the file, leaves the
// frontmatter and every heading where they were.
func stampSkillFile(data []byte) []byte {
	out := bytes.TrimRight(data, "\n")
	return append(append(append([]byte{}, out...), '\n', '\n'), (skillStamp() + "\n")...)
}

// shippedSkillFile is one file `skill install` writes, as it writes it.
type shippedSkillFile struct {
	// Path is slash-separated and relative to the skills directory.
	Path string
	Data []byte
}

// shippedSkillFiles lists every file this binary installs, sorted by path:
// the four skills (their own guides' links into the repository rerouted to the
// installed snapshot), the snapshot itself — the repository files the skills
// link to and an unrewritten copy of the skills, whose relative links then
// resolve inside it — and a version stamp on every Markdown file.
func shippedSkillFiles() ([]shippedSkillFile, error) {
	var files []shippedSkillFile
	add := func(name string, data []byte) {
		if strings.HasSuffix(name, ".md") {
			data = stampSkillFile(data)
		}
		files = append(files, shippedSkillFile{Path: name, Data: data})
	}
	walk := func(fsys fs.FS, visit func(name string, data []byte)) error {
		return fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := fs.ReadFile(fsys, name)
			if err != nil {
				return err
			}
			visit(name, data)
			return nil
		})
	}
	if err := walk(skills.Bundle(), func(name string, data []byte) {
		add(path.Join(skillReferencesDir, "skills", name), data)
		// Only a skill's own guides are rerouted: its examples keep their
		// text, and the snapshot keeps the repository's relative links.
		if strings.HasSuffix(name, ".md") && strings.Count(name, "/") == 1 {
			data = repositoryLink.ReplaceAll(data, []byte("](../"+skillReferencesDir+"/$1/"))
		}
		add(name, data)
	}); err != nil {
		return nil, err
	}
	if err := walk(json2pptx.SkillReferences(), func(name string, data []byte) {
		add(path.Join(skillReferencesDir, name), data)
	}); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// installSkills writes the shipped files into dest, one directory per skill,
// and the manifest. Files it does not ship are left alone.
func installSkills(dest string) (skillManifest, error) {
	manifest := skillManifest{SchemaVersion: SchemaVersion, BinaryVersion: Version}
	if strings.TrimSpace(dest) == "" {
		return manifest, fmt.Errorf("no destination: set --dest or %s", skillDirEnv)
	}
	files, err := shippedSkillFiles()
	if err != nil {
		return manifest, err
	}
	for _, f := range files {
		target := filepath.Join(dest, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return manifest, err
		}
		if err := os.WriteFile(target, f.Data, 0o600); err != nil {
			return manifest, err
		}
		manifest.Files = append(manifest.Files, f.Path)
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	root := filepath.Join(dest, "generate-deck")
	if err := os.WriteFile(filepath.Join(root, skillManifestName), append(body, '\n'), 0o600); err != nil {
		return manifest, err
	}
	// A fresh install has nothing to warn about, and the next mismatch is a
	// new one.
	_ = os.Remove(filepath.Join(root, skillWarnedName))
	return manifest, nil
}

// checkInstalledSkill compares the skills under dir with the binary, file by
// file. A directory with no generate-deck skill is "not installed", which is
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
	// A copy from a newer binary is not this binary's business: its files
	// are not expected to match the ones shipped here.
	if cmp, err := compareSkillSchemaVersions(status.InstalledVersion, SchemaVersion); status.InstalledVersion != "" && err == nil && cmp > 0 {
		status.Current = true
		status.Refresh = ""
		return status
	}
	checkSkillFiles(dir, &status)
	problems := skillProblems(status)
	if len(problems) == 0 {
		status.Current = true
		status.Refresh = ""
		return status
	}
	status.Message = fmt.Sprintf("the generate-deck skill installed at %s %s: run `%s`", root, strings.Join(problems, " and "), skillRefreshCommand)
	return status
}

// checkSkillStamp is the check an entry command can afford: the schema
// version the install manifest records against the binary's, one small file
// read. checkInstalledSkill reads all ~60 installed files (about 1 MB, 1.3 ms
// and 3.3 MB of allocations on a warm cache; BenchmarkInstalledSkillCheck),
// which every get-started, capabilities, semantic and generate run paid, and
// get-started twice (go-slide-creator-v25ae). A copy with no manifest — made
// by hand, or by an installer that predates it — is read from its SKILL.md
// frontmatter instead and told to reinstall.
//
// It does not see a file that was deleted or edited after a current install;
// `json2pptx skill status` does.
func checkSkillStamp(dir string) skillStatus {
	status := skillStatus{Dir: dir, BinaryVersion: SchemaVersion}
	if dir == "" {
		return status
	}
	root := filepath.Join(dir, "generate-deck")
	var manifest skillManifest
	hasManifest := false
	if body, err := os.ReadFile(filepath.Join(root, skillManifestName)); err == nil { //nolint:gosec // the skill directory the user configured
		hasManifest = json.Unmarshal(body, &manifest) == nil && manifest.SchemaVersion != ""
	}
	if hasManifest {
		status.InstalledVersion = manifest.SchemaVersion
	} else {
		body, err := os.ReadFile(filepath.Join(root, "SKILL.md")) //nolint:gosec // the skill directory the user configured
		if err != nil {
			return status
		}
		if m := skillFrontmatterVersion.FindSubmatch(body); m != nil {
			status.InstalledVersion = string(m[1])
		}
	}
	status.Installed = true
	cmp, err := compareSkillSchemaVersions(status.InstalledVersion, SchemaVersion)
	var problem string
	switch {
	case status.InstalledVersion == "":
		problem = "carries no schema_version (it predates version stamps)"
	case err == nil && cmp < 0:
		problem = fmt.Sprintf("is at %s, older than this binary (%s)", status.InstalledVersion, SchemaVersion)
	case err == nil && cmp == 0 && !hasManifest:
		problem = "has no install manifest (it was copied by hand, so its files are unchecked)"
	default:
		// In step with the binary, or from a newer one.
		status.Current = true
		return status
	}
	status.Refresh = skillRefreshCommand
	status.Message = fmt.Sprintf("the generate-deck skill installed at %s %s: run `%s`", root, problem, skillRefreshCommand)
	return status
}

// checkSkillFiles compares every file this binary ships with the copy under
// dir and records the ones that are missing, unstamped, stale or changed.
func checkSkillFiles(dir string, status *skillStatus) {
	files, _ := shippedSkillFiles()
	for _, f := range files {
		// A skill the user chose not to keep is theirs to remove; the
		// generate-deck skill, which the binary's own output cites, and the
		// snapshot under it are checked whole.
		if !strings.HasPrefix(f.Path, "generate-deck/") {
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path.Dir(f.Path)))); err != nil {
				continue
			}
		}
		status.FilesChecked++
		shown := strings.TrimPrefix(f.Path, "generate-deck/")
		installed, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Path))) //nolint:gosec // a path this binary ships, under the configured skill directory
		switch {
		case err != nil:
			status.MissingFiles = append(status.MissingFiles, shown)
		case !strings.HasSuffix(f.Path, ".md"):
			if !bytes.Equal(installed, f.Data) {
				status.ChangedFiles = append(status.ChangedFiles, shown)
			}
		default:
			m := skillStampLine.FindSubmatch(installed)
			if m == nil {
				status.UnstampedFiles = append(status.UnstampedFiles, shown)
			} else if cmp, err := compareSkillSchemaVersions(string(m[1]), SchemaVersion); err == nil && cmp < 0 {
				status.StaleFiles = append(status.StaleFiles, shown)
			}
		}
	}
}

// skillProblems words what checkInstalledSkill found, or returns nothing for
// a copy that matches the binary.
func skillProblems(status skillStatus) []string {
	some := func(names []string) string {
		if len(names) > 4 {
			names = append(append([]string(nil), names[:4]...), fmt.Sprintf("… %d more", len(names)-4))
		}
		return strings.Join(names, ", ")
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
	if len(status.MissingFiles) > 0 {
		problems = append(problems, "lacks "+some(status.MissingFiles))
	}
	if len(status.StaleFiles) > 0 {
		problems = append(problems, "has files stamped with an older version ("+some(status.StaleFiles)+")")
	}
	// Unstamped and changed files only add to a copy that is otherwise in
	// step: an old copy is already told to refresh.
	if len(problems) == 0 {
		if len(status.UnstampedFiles) > 0 {
			problems = append(problems, "has files with no version stamp ("+some(status.UnstampedFiles)+")")
		}
		if len(status.ChangedFiles) > 0 {
			problems = append(problems, "has files that differ from the ones this binary ships ("+some(status.ChangedFiles)+")")
		}
	}
	return problems
}

// skillWarningOnce keeps the stale-skill check to one per process.
var skillWarningOnce sync.Once

// skillWarningKey identifies one mismatch: this binary's version against that
// installed copy's state. A different binary, or a copy that changed, is a
// different mismatch and is said again.
func skillWarningKey(status skillStatus) string {
	sum := sha256.Sum256([]byte(SchemaVersion + "\n" + status.Message))
	return hex.EncodeToString(sum[:8])
}

// skillWarningStatePaths are where the "already said" key may live: beside
// the manifest, else (a read-only skill directory) in the user cache.
func skillWarningStatePaths(dir string) []string {
	paths := []string{filepath.Join(dir, "generate-deck", skillWarnedName)}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		sum := sha256.Sum256([]byte(dir))
		paths = append(paths, filepath.Join(cache, "json2pptx", "skill-warned-"+hex.EncodeToString(sum[:6])))
	}
	return paths
}

// staleSkillWarning returns the line to print for dir, or "" when the skill
// is current, absent, or this mismatch was already reported. It records the
// mismatch so later commands stay quiet (go-slide-creator-4eu2o): the first
// version printed the line on every command, which an agent running forty of
// them learned to ignore.
func staleSkillWarning(dir string) string {
	status := checkSkillStamp(dir)
	if !status.Installed || status.Current {
		return ""
	}
	key := skillWarningKey(status)
	paths := skillWarningStatePaths(dir)
	for _, p := range paths {
		if seen, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(seen)) == key { //nolint:gosec // a state file this binary wrote
			return ""
		}
	}
	for _, p := range paths {
		if os.MkdirAll(filepath.Dir(p), 0o750) == nil && os.WriteFile(p, []byte(key+"\n"), 0o600) == nil {
			break
		}
	}
	return "json2pptx: " + status.Message + " (said once; `json2pptx skill status` repeats it and checks every file)"
}

// warnIfSkillStale prints one line on stderr when the installed skill does
// not match the binary. Stdout stays the command's result.
func warnIfSkillStale() {
	if strings.EqualFold(strings.TrimSpace(os.Getenv(skillCheckEnv)), "off") {
		return
	}
	skillWarningOnce.Do(func() {
		if line := staleSkillWarning(skillInstallDir()); line != "" {
			fmt.Fprintln(os.Stderr, line)
		}
	})
}

// runSkill implements "skill install", "skill status" and "skill cli-map".
func runSkill() error {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		printSkillUsage()
		if len(os.Args) >= 2 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
			return nil
		}
		return fmt.Errorf("skill requires a subcommand: install, status or cli-map")
	}
	sub := os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	switch sub {
	case "install":
		return runSkillInstall()
	case "status":
		return runSkillStatus()
	case "cli-map":
		return runSkillCLIMap()
	default:
		printSkillUsage()
		return fmt.Errorf("unknown skill subcommand %q: want install, status or cli-map", sub)
	}
}

func printSkillUsage() {
	fmt.Fprint(os.Stderr, `Usage: json2pptx skill <install|status|cli-map> [options]

  install   Write the agent skills this binary was built with (generate-deck,
            template-deck, slide-visual-qa, render-diagram) into DIR, with the
            docs and examples they link to and a version stamp on every file.
  status    Report whether each installed skill file matches this binary.
  cli-map   Print the MCP tool -> CLI command table (--tools, --format md).

install and status take --dest DIR; DIR defaults to $JSON2PPTX_SKILL_DIR, else
~/.claude/skills.
`)
}

func runSkillInstall() error {
	flags := flag.NewFlagSet("skill install", flag.ContinueOnError)
	dest := flags.String("dest", skillInstallDir(), "Directory the skills are written into (one sub-directory per skill)")
	flags.Usage = cliDefaultUsage(flags, "skill install [--dest DIR]",
		"Write the agent skills this binary was built with into DIR, and under generate-deck/references/repository/ the docs and examples they link to, so every link resolves offline.\nEvery Markdown file ends in a version stamp; `json2pptx skill status` checks them. Files this binary does not ship are left alone.")
	if err := cliParse(flags, os.Args[1:]); err != nil {
		return err
	}
	manifest, err := installSkills(*dest)
	if err != nil {
		return fmt.Errorf("skill install: %w", err)
	}
	references := 0
	for _, name := range manifest.Files {
		if strings.HasPrefix(name, skillReferencesDir+"/") {
			references++
		}
	}
	return cliPrintJSON(map[string]any{
		"installed":            true,
		"dir":                  *dest,
		"schema_version":       manifest.SchemaVersion,
		"file_count":           len(manifest.Files),
		"reference_file_count": references,
	})
}

func runSkillStatus() error {
	flags := flag.NewFlagSet("skill status", flag.ContinueOnError)
	dest := flags.String("dest", skillInstallDir(), "Directory the skills are installed in")
	flags.Usage = cliDefaultUsage(flags, "skill status [--dest DIR]",
		"Report whether the skills installed in DIR match this binary: the generate-deck schema_version, and for every file the binary ships whether it is there, carries the current version stamp (Markdown) or has the shipped bytes (examples, images).")
	if err := cliParse(flags, os.Args[1:]); err != nil {
		return err
	}
	return cliPrintJSON(checkInstalledSkill(*dest))
}

// cliMapRow is one MCP tool and what a CLI caller runs instead.
type cliMapRow struct {
	Tool string `json:"tool"`
	// CLI is the command line, with the arguments a caller passes.
	CLI string `json:"cli"`
	// CLIThen is the follow-up command when the tool's job takes two.
	CLIThen string `json:"cli_then,omitempty"`
	// CLICounterpart is the subcommand, as get_capabilities reports it.
	CLICounterpart string `json:"cli_counterpart,omitempty"`
	// MCPOnly is true when no CLI subcommand does the tool's job; CLI is then
	// the closest workflow and MCPOnlyReason says why.
	MCPOnly       bool   `json:"mcp_only"`
	MCPOnlyReason string `json:"mcp_only_reason,omitempty"`
}

// cliMapRows builds the MCP-to-CLI table for a tool profile from the tool
// classifications (the same source as get_capabilities tool_list). It is
// generated, so it cannot drift from the binary the way a table written into a
// skill file did: that one was wrong for fourteen tools.
func cliMapRows(profile string) []cliMapRow {
	var names []string
	if profile == toolProfileAll {
		for _, name := range mcpToolNames() {
			if _, folded := foldedTools[name]; !folded {
				names = append(names, name)
			}
		}
	} else {
		for name := range profileToolSet(profile) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	classes := toolClassifications()
	rows := make([]cliMapRow, 0, len(names))
	for _, name := range names {
		class := classes[name]
		rows = append(rows, cliMapRow{
			Tool:           name,
			CLI:            cliCommandForTool(name),
			CLIThen:        cliStepFollowUps[name],
			CLICounterpart: class.CLICounterpart,
			MCPOnly:        class.MCPOnlyReason != "" || class.CLICounterpart == "",
			MCPOnlyReason:  class.MCPOnlyReason,
		})
	}
	return rows
}

func runSkillCLIMap() error {
	flags := flag.NewFlagSet("skill cli-map", flag.ContinueOnError)
	tools := flags.String("tools", toolProfileDeckSpec, "Tool profile to map: deckspec (the default MCP listing), core or all")
	format := flags.String("format", "json", "Output format: json or md (a Markdown table)")
	flags.Usage = cliDefaultUsage(flags, "skill cli-map [--tools deckspec|core|all] [--format json|md]",
		"Print, for every MCP tool of a profile, the CLI command that does the same job, or that the tool is MCP-only and why.\nGenerated from the tool classifications get_capabilities serves; small (deckspec: about 2 KB).")
	if err := cliParse(flags, os.Args[1:]); err != nil {
		return err
	}
	profile, err := parseToolProfile(*tools)
	if err != nil {
		return err
	}
	rows := cliMapRows(profile)
	switch *format {
	case "json":
		return cliPrintJSON(map[string]any{"profile": profile, "schema_version": SchemaVersion, "tools": rows})
	case "md", "markdown":
		var b strings.Builder
		b.WriteString("| MCP tool | CLI |\n|---|---|\n")
		for _, r := range rows {
			cli := "`" + strings.Join(strings.Fields(r.CLI), " ") + "`"
			if r.CLIThen != "" {
				cli += ", then `" + strings.Join(strings.Fields(r.CLIThen), " ") + "`"
			}
			if r.MCPOnly {
				cli = "MCP-only. " + strings.TrimSuffix(r.MCPOnlyReason, ".") + "."
			}
			fmt.Fprintf(&b, "| `%s` | %s |\n", r.Tool, strings.ReplaceAll(cli, "|", `\|`))
		}
		_, err := os.Stdout.WriteString(b.String())
		return err
	default:
		return fmt.Errorf("skill cli-map: invalid --format %q: want json or md", *format)
	}
}
