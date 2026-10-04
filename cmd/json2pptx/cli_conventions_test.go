package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// cliRun runs the shared test binary and returns stdout, stderr and the exit
// code. env entries are added to the process environment.
func cliRun(t *testing.T, env []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(buildTestBinary(t), args...) //nolint:gosec // test-controlled arguments
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run %v: %v", args, err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

// cliRenderToolsAvailable reports whether LibreOffice and ImageMagick are on
// PATH, as the CLI itself decides it.
func cliRenderToolsAvailable() bool {
	return cliMCPConfig("../../templates", "").getStartedRuntime().RenderAvailable
}

// flagSetNamesInSource returns the name of every flag.NewFlagSet in this
// package's non-test sources.
func flagSetNamesInSource(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`flag\.NewFlagSet\("([^"]+)"`)
	seen := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f) //nolint:gosec // package source file
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			seen[m[1]] = true
		}
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// TestCLIConventionCoversEveryFlagSet enumerates every flag set the CLI
// defines and checks the shared convention on each: a convention entry exists,
// -h prints to stdout and exits 0, and the help lists --format, --verbose and
// (where the command writes a file) --out (go-slide-creator-e6gsz).
func TestCLIConventionCoversEveryFlagSet(t *testing.T) {
	conventions := cliConventions()
	inSource := flagSetNamesInSource(t)
	if len(inSource) < 40 {
		t.Fatalf("found only %d flag sets in source — the scan is broken", len(inSource))
	}
	for _, name := range inSource {
		if _, ok := conventions[name]; !ok {
			t.Errorf("flag set %q has no entry in cliConventions() — add one so it follows the shared flag convention", name)
		}
	}
	for name := range conventions {
		found := false
		for _, n := range inSource {
			found = found || n == name
		}
		if !found {
			t.Errorf("cliConventions() has %q but no flag.NewFlagSet(%q) exists — remove the stale entry", name, name)
		}
	}

	for _, name := range cliFlagSetNames() {
		conv := conventions[name]
		t.Run(name, func(t *testing.T) {
			args := append(strings.Fields(name), "-h")
			stdout, stderr, code := cliRun(t, []string{"HOME=" + t.TempDir()}, args...)
			if code != 0 {
				t.Fatalf("%s -h exited %d, want 0\nstderr: %s", name, code, stderr)
			}
			if strings.TrimSpace(stderr) != "" {
				t.Errorf("%s -h wrote to stderr; help belongs on stdout:\n%s", name, stderr)
			}
			if !strings.Contains(stdout, "Usage") {
				t.Fatalf("%s -h printed no usage on stdout:\n%s", name, stdout)
			}
			if regexp.MustCompile(`(?m)^  -[a-z]{2,}`).MatchString(stdout) {
				t.Errorf("%s -h lists a long flag with a single dash; help uses --flag:\n%s", name, stdout)
			}
			if !strings.Contains(stdout, "--verbose") {
				t.Errorf("%s -h does not list --verbose", name)
			}
			if !conv.Server && !strings.Contains(stdout, "--format") {
				t.Errorf("%s -h does not list --format", name)
			}
			if conv.Out != "" {
				for _, want := range []string{"--out ", "--" + conv.Out + " "} {
					if !strings.Contains(stdout, "  "+want) {
						t.Errorf("%s -h does not list %s (canonical --out plus the legacy alias)", name, strings.TrimSpace(want))
					}
				}
			}
			if conv.Input != "" && !strings.Contains(stdout, "  --"+conv.Input+" ") {
				t.Errorf("%s -h does not list the legacy input flag --%s", name, conv.Input)
			}
		})
	}
}

// TestCLIHelpExitsZeroOnStdoutEverywhere runs --help on every command the
// dispatcher knows (and "help" itself).
func TestCLIHelpExitsZeroOnStdoutEverywhere(t *testing.T) {
	commands := dispatchCommandNames(t)
	if len(commands) < 40 {
		t.Fatalf("only %d dispatch commands parsed", len(commands))
	}
	for _, name := range commands {
		for _, helpFlag := range []string{"-h", "--help"} {
			if name == "help" {
				helpFlag = ""
			}
			args := []string{name}
			if helpFlag != "" {
				args = append(args, helpFlag)
			}
			stdout, stderr, code := cliRun(t, []string{"HOME=" + t.TempDir()}, args...)
			if code != 0 || strings.TrimSpace(stdout) == "" || strings.TrimSpace(stderr) != "" {
				t.Errorf("%v: exit=%d stdout=%dB stderr=%q; want exit 0, help on stdout, nothing on stderr",
					args, code, len(stdout), stderr)
			}
		}
	}
	// No arguments at all is a help request too.
	stdout, stderr, code := cliRun(t, nil)
	if code != 0 || !strings.Contains(stdout, "Usage: json2pptx") || stderr != "" {
		t.Errorf("bare invocation: exit=%d stderr=%q; want usage on stdout", code, stderr)
	}
}

// TestCLIParseAcceptsFlagsAfterPositionals pins the parser: flags before or
// after positional arguments, "--" ends flag parsing, the first positional
// fills the legacy input flag, and a stray argument is refused.
func TestCLIParseAcceptsFlagsAfterPositionals(t *testing.T) {
	newScore := func() (*flag.FlagSet, *string, *string) {
		fs := flag.NewFlagSet("score", flag.ContinueOnError)
		fs.SetOutput(&bytes.Buffer{})
		in := fs.String("json", "", "input")
		mode := fs.String("mode", "", "mode")
		return fs, in, mode
	}

	fs, in, mode := newScore()
	if err := cliParse(fs, []string{"deck.json", "--mode", "strict"}); err != nil {
		t.Fatal(err)
	}
	if *in != "deck.json" || *mode != "strict" || fs.NArg() != 0 {
		t.Errorf("positional input then flag: json=%q mode=%q rest=%v", *in, *mode, fs.Args())
	}

	fs, in, mode = newScore()
	if err := cliParse(fs, []string{"--mode", "strict", "--json", "legacy.json"}); err != nil {
		t.Fatal(err)
	}
	if *in != "legacy.json" || *mode != "strict" {
		t.Errorf("legacy input flag: json=%q mode=%q", *in, *mode)
	}

	fs, _, _ = newScore()
	if err := cliParse(fs, []string{"deck.json", "extra.json"}); err == nil || !strings.Contains(err.Error(), `unexpected argument "extra.json"`) {
		t.Errorf("stray positional: err=%v, want an unexpected-argument error", err)
	}

	// A command that reads its own positionals sees all of them, in order,
	// with flags picked out from between them.
	vfs := flag.NewFlagSet("validate", flag.ContinueOnError)
	vfs.SetOutput(&bytes.Buffer{})
	asJSON := vfs.Bool("json", false, "emit json")
	if err := cliParse(vfs, []string{"a.json", "--json", "b.json", "--", "--odd-name.json"}); err != nil {
		t.Fatal(err)
	}
	if !*asJSON || strings.Join(vfs.Args(), ",") != "a.json,b.json,--odd-name.json" {
		t.Errorf("validate: json=%v args=%v", *asJSON, vfs.Args())
	}

	// --format json|text maps onto the legacy --json boolean; --out onto the
	// legacy output flag.
	pfs := flag.NewFlagSet("patterns list", flag.ContinueOnError)
	pfs.SetOutput(&bytes.Buffer{})
	pj := pfs.Bool("json", false, "emit json")
	if err := cliParse(pfs, []string{"--format", "json"}); err != nil || !*pj {
		t.Errorf("--format json: err=%v json=%v", err, *pj)
	}
	gfs := flag.NewFlagSet("generate", flag.ContinueOnError)
	gfs.SetOutput(&bytes.Buffer{})
	gin := gfs.String("json", "", "input")
	gout := gfs.String("output", "./output", "output")
	if err := cliParse(gfs, []string{"deck.json", "--out", "/tmp/x.pptx", "--format", "json"}); err != nil {
		t.Fatal(err)
	}
	if *gin != "deck.json" || *gout != "/tmp/x.pptx" {
		t.Errorf("generate: json=%q output=%q", *gin, *gout)
	}
	gfs2 := flag.NewFlagSet("generate", flag.ContinueOnError)
	gfs2.SetOutput(&bytes.Buffer{})
	gfs2.String("json", "", "input")
	gfs2.String("output", "./output", "output")
	if err := cliParse(gfs2, []string{"deck.json", "--format", "text"}); err == nil {
		t.Error("--format text on a JSON-only command should be refused")
	}
}

func TestCLIHelpRequested(t *testing.T) {
	yes := [][]string{{}, {"help"}, {"-h"}, {"generate", "--help"}, {"icons"}, {"semantic", "help"}, {"validate", "x.json", "-h"}}
	no := [][]string{{"validate", "x.json"}, {"plan-deck", "help"}, {"validate", "--", "-h"}, {"recommend-pattern", "show", "help"}}
	for _, args := range yes {
		if !cliHelpRequested(args) {
			t.Errorf("cliHelpRequested(%v) = false, want true", args)
		}
	}
	for _, args := range no {
		if cliHelpRequested(args) {
			t.Errorf("cliHelpRequested(%v) = true, want false", args)
		}
	}
}

// TestCLIFlagsAfterPositionalEndToEnd is the reported failure: `validate
// <file> --json` printed non-JSON, and `score <file>` was a usage error.
func TestCLIFlagsAfterPositionalEndToEnd(t *testing.T) {
	deck := "../../examples/basic-deck.json"
	for _, args := range [][]string{
		{"validate", deck, "--json", "--templates-dir", "../../templates"},
		{"validate", "--format", "json", deck, "--templates-dir", "../../templates"},
		{"score", deck, "--templates-dir", "../../templates"},
		{"analyze-rhythm", deck},
	} {
		stdout, stderr, code := cliRun(t, nil, args...)
		if code != 0 {
			t.Errorf("%v: exit %d\nstderr: %s", args, code, stderr)
			continue
		}
		var v map[string]any
		if err := json.Unmarshal([]byte(stdout), &v); err != nil {
			t.Errorf("%v: stdout is not one JSON object: %v\n%.200s", args, err, stdout)
		}
	}
}

// TestCLILogHandlerLevelAndDedup: INFO is dropped below the configured level,
// and an identical record is written once (go-slide-creator-pikfw).
func TestCLILogHandlerLevelAndDedup(t *testing.T) {
	var buf bytes.Buffer
	level := &slog.LevelVar{}
	level.Set(slog.LevelWarn)
	logger := slog.New(newCLILogHandler(&buf, level))

	logger.Info("pattern expanded", "pattern", "kpi-3up")
	if buf.Len() != 0 {
		t.Fatalf("INFO written at WARN level: %s", buf.String())
	}
	logger.Warn("media content failed", "slide", 2)
	logger.Warn("media content failed", "slide", 2)
	logger.Warn("media content failed", "slide", 3)
	if got := strings.Count(buf.String(), "\n"); got != 2 {
		t.Errorf("wrote %d lines, want 2 (the repeated record once, the distinct one once):\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), "WARN media content failed slide=2") {
		t.Errorf("unexpected line format:\n%s", buf.String())
	}

	buf.Reset()
	level.Set(slog.LevelInfo)
	for i := 0; i < 5; i++ {
		logger.Info("pattern expanded", "pattern", "kpi-3up", "version", 1)
	}
	if got := strings.Count(buf.String(), "\n"); got != 1 {
		t.Errorf("five identical INFO records wrote %d lines, want 1:\n%s", got, buf.String())
	}
}

// TestCLIGenerateIsQuietAndReportsOutputOnStdout: no INFO lines by default,
// the output path on stdout as JSON, and --verbose logs each line once.
func TestCLIGenerateIsQuietAndReportsOutputOnStdout(t *testing.T) {
	out := filepath.Join(t.TempDir(), "deck.pptx")
	deck := "../../examples/chart-insights-split.json"

	stdout, stderr, code := cliRun(t, nil, "generate", deck, "--out", out, "--templates-dir", "../../templates")
	if code != 0 {
		t.Fatalf("generate exited %d\nstderr: %s", code, stderr)
	}
	if strings.Contains(stderr, " INFO ") {
		t.Errorf("INFO logged without --verbose:\n%s", stderr)
	}
	var res JSONOutput
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("stdout is not the result JSON: %v\n%s", err, stdout)
	}
	if !res.Success || res.OutputPath != out || res.SlideCount == 0 {
		t.Errorf("result = %+v; want success with output_path %s", res, out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("deck not written at the reported path: %v", err)
	}

	_, stderr, code = cliRun(t, nil, "generate", "--json", deck, "--output", out, "--templates-dir", "../../templates", "--verbose")
	if code != 0 {
		t.Fatalf("generate --verbose exited %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, " INFO ") {
		t.Errorf("--verbose logged no INFO line:\n%s", stderr)
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		// Drop the timestamp: two identical records a second apart are still
		// the duplicate the handler is meant to drop.
		if parts := strings.SplitN(line, " ", 3); len(parts) == 3 {
			line = parts[2]
		}
		if seen[line] {
			t.Errorf("log line written more than once: %s", line)
		}
		seen[line] = true
	}
}

// TestCLIErrorEnvelopeOnStdoutWithCommandName: a failing tool-backed command
// prints its finding envelope on stdout, named after the command that ran.
func TestCLIErrorEnvelopeOnStdoutWithCommandName(t *testing.T) {
	for _, tc := range []struct {
		args []string
		name string
	}{
		{[]string{"render-thumbnails", "no-such-deck.pptx"}, "render-thumbnails"},
		{[]string{"describe-finding", "NOT_A_CODE"}, "describe-finding"},
		{[]string{"semantic", "kinds", "not_a_kind"}, "semantic kinds"},
	} {
		stdout, stderr, code := cliRun(t, nil, tc.args...)
		if code == 0 {
			t.Errorf("%v exited 0", tc.args)
		}
		var env struct {
			Subcommand string `json:"subcommand"`
			OK         bool   `json:"ok"`
			Findings   []any  `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &env); err != nil {
			t.Errorf("%v: stdout is not a finding envelope: %v\nstdout=%.200s\nstderr=%s", tc.args, err, stdout, stderr)
			continue
		}
		if env.OK || len(env.Findings) == 0 || env.Subcommand != tc.name {
			t.Errorf("%v: envelope ok=%v findings=%d subcommand=%q; want a failure named %q",
				tc.args, env.OK, len(env.Findings), env.Subcommand, tc.name)
		}
	}
}

// validateJSONShape is what every `validate --format json` answer decodes to.
type validateJSONShape struct {
	Valid    *bool `json:"valid"`
	Findings struct {
		OK       *bool `json:"ok"`
		Findings []struct {
			Code     string         `json:"code"`
			Message  string         `json:"message"`
			Evidence map[string]any `json:"evidence"`
		} `json:"findings"`
	} `json:"findings"`
}

const unrenderableChartDeck = `{
  "template": "midnight-blue",
  "slides": [
    {"slide_type": "title", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Bad chart deck"},
      {"placeholder_id": "subtitle", "type": "text", "text_value": "Chart type typo"}
    ]},
    {"slide_type": "chart", "content": [
      {"placeholder_id": "title", "type": "text", "text_value": "Revenue grew 12% across both regions"},
      {"placeholder_id": "body", "type": "chart", "chart_value": {
        "type": "stacked-bar", "title": "Revenue",
        "data": {"categories": ["Q1", "Q2"], "series": [{"name": "A", "values": [1, 2]}, {"name": "B", "values": [2, 3]}]}
      }}
    ]}
  ]
}`

// TestCLIValidateJSONHasOneShape: a passing deck, a refused deck and a file
// that is not JSON all decode to {valid, findings:{ok, findings:[...]}}
// (go-slide-creator-pikfw).
func TestCLIValidateJSONHasOneShape(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad-chart.json")
	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(bad, []byte(unrenderableChartDeck), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(malformed, []byte("{\"template\": \"midnight-blue\",\n  \"slides\": ["), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		file      string
		wantValid bool
		wantCode  string
	}{
		{"../../examples/basic-deck.json", true, ""},
		{bad, false, "RENDER.diagram_render_failed"},
		{malformed, false, "INPUT.INVALID_JSON"},
		{filepath.Join(dir, "absent.json"), false, "INPUT.FILE_NOT_FOUND"},
	} {
		stdout, stderr, code := cliRun(t, nil, "validate", tc.file, "--format", "json", "--templates-dir", "../../templates")
		var shape validateJSONShape
		if err := json.Unmarshal([]byte(stdout), &shape); err != nil {
			t.Errorf("%s: stdout does not decode to the validate shape: %v\nstdout=%.300s\nstderr=%s", tc.file, err, stdout, stderr)
			continue
		}
		if shape.Valid == nil || shape.Findings.OK == nil {
			t.Errorf("%s: valid / findings.ok missing from %.300s", tc.file, stdout)
			continue
		}
		if *shape.Valid != tc.wantValid || *shape.Findings.OK != tc.wantValid || (code == 0) != tc.wantValid {
			t.Errorf("%s: valid=%v findings.ok=%v exit=%d; want valid=%v", tc.file, *shape.Valid, *shape.Findings.OK, code, tc.wantValid)
		}
		if tc.wantCode == "" {
			continue
		}
		found := false
		for _, f := range shape.Findings.Findings {
			found = found || f.Code == tc.wantCode
		}
		if !found {
			t.Errorf("%s: no %s finding in %.400s", tc.file, tc.wantCode, stdout)
		}
	}

	// The malformed file's finding says where.
	stdout, _, _ := cliRun(t, nil, "validate", malformed, "--format", "json")
	if !strings.Contains(stdout, "line 2, column") {
		t.Errorf("INVALID_JSON finding carries no line/column: %.300s", stdout)
	}
}

// TestCLIValidateReportsUnrenderableChart: plain `validate` — no --fit-report
// — refuses a chart whose type cannot render, with the suggestion, at the
// chart's path (go-slide-creator-gr64x).
func TestCLIValidateReportsUnrenderableChart(t *testing.T) {
	deck := filepath.Join(t.TempDir(), "bad-chart.json")
	if err := os.WriteFile(deck, []byte(unrenderableChartDeck), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := cliRun(t, nil, "validate", deck, "--templates-dir", "../../templates")
	if code == 0 {
		t.Errorf("plain validate exited 0 for a deck whose chart cannot render")
	}
	if !strings.Contains(stdout, "INVALID") || !strings.Contains(stdout, `did you mean "stacked_bar"`) {
		t.Errorf("human output does not name the unknown type with its suggestion:\n%s", stdout)
	}

	stdout, _, code = cliRun(t, nil, "validate", deck, "--format", "json", "--templates-dir", "../../templates")
	if code == 0 {
		t.Errorf("validate --format json exited 0")
	}
	var shape validateJSONShape
	if err := json.Unmarshal([]byte(stdout), &shape); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, stdout)
	}
	found := false
	for _, f := range shape.Findings.Findings {
		if f.Code != "RENDER.diagram_render_failed" {
			continue
		}
		found = true
		if path, _ := f.Evidence["path"].(string); path != "/slides/1/content/1/chart_value" {
			t.Errorf("finding path = %q, want /slides/1/content/1/chart_value", path)
		}
		if !strings.Contains(f.Message, `did you mean "stacked_bar"`) {
			t.Errorf("finding message carries no suggestion: %s", f.Message)
		}
	}
	if !found {
		t.Errorf("no diagram_render_failed finding: %s", stdout)
	}

	// A renderable deck is unaffected, and the charts its patterns draw are
	// counted: this deck's two charts both sit inside chart-insights-split
	// and used to be summarised as "Charts: 0".
	stdout, stderr, code := cliRun(t, nil, "validate", "../../examples/chart-insights-split.json", "--templates-dir", "../../templates")
	if code != 0 {
		t.Errorf("a deck with a valid chart no longer validates: %s", stderr)
	}
	if !strings.Contains(stdout, "Charts: 2 ") {
		t.Errorf("pattern-drawn charts are not counted in the summary:\n%s", stdout)
	}
}

// TestCLIGenerateFailsOnUnrenderableChart: generate exits non-zero and leaves
// no deck (so no placeholder image) at every strictness level, with and
// without --partial (go-slide-creator-gr64x).
func TestCLIGenerateFailsOnUnrenderableChart(t *testing.T) {
	dir := t.TempDir()
	deck := filepath.Join(dir, "bad-chart.json")
	if err := os.WriteFile(deck, []byte(unrenderableChartDeck), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{
		{"--strict-fit=off"}, {"--strict-fit=warn"}, {"--strict-fit=strict"},
		{"--strict-fit=off", "--partial"}, {"--strict-fit=off", "--output-validation=off"},
	} {
		out := filepath.Join(dir, "out-"+strings.NewReplacer("=", "-", "--", "").Replace(strings.Join(extra, ""))+".pptx")
		args := append([]string{"generate", deck, "--out", out, "--templates-dir", "../../templates"}, extra...)
		stdout, stderr, code := cliRun(t, nil, args...)
		if code == 0 {
			t.Errorf("%v: exited 0 for a deck whose chart cannot render", extra)
		}
		if _, err := os.Stat(out); err == nil {
			t.Errorf("%v: a deck was left at %s — it carries the placeholder image", extra, out)
		}
		var res JSONOutput
		if err := json.Unmarshal([]byte(strings.TrimSpace(lastLine(stdout))), &res); err != nil || res.Success {
			t.Errorf("%v: stdout result is not a failure: err=%v stdout=%.300s", extra, err, stdout)
		}
		if !strings.Contains(stdout+stderr, "stacked_bar") {
			t.Errorf("%v: the failure does not carry the did-you-mean suggestion", extra)
		}
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}

// TestSemanticKindsMatchesListSlideKinds keeps the CLI catalogue and the
// list_slide_kinds MCP tool in sync: same kinds, same summaries' first
// sentence, same required fields, and the same example for every kind
// (go-slide-creator-e7jxr).
func TestSemanticKindsMatchesListSlideKinds(t *testing.T) {
	// The catalogue with each kind's example (the bare catalogue leaves them
	// out; naming kinds, or fields:["example"], returns them).
	result, err := handleListSlideKinds(context.Background(), mcpRequestWithArgs(map[string]any{"fields": []any{"example"}}))
	if err != nil || result.IsError {
		t.Fatalf("list_slide_kinds failed: %v", err)
	}
	var mcpCatalogue semanticKindsResult
	if err := json.Unmarshal([]byte(cliResultText(result)), &mcpCatalogue); err != nil {
		t.Fatal(err)
	}
	if len(mcpCatalogue.SlideKinds) != len(semantic.AllSlideKinds()) {
		t.Fatalf("list_slide_kinds returned %d kinds, the registry has %d", len(mcpCatalogue.SlideKinds), len(semantic.AllSlideKinds()))
	}

	stdout, stderr, code := cliRun(t, nil, "semantic", "kinds", "--format", "json")
	if code != 0 {
		t.Fatalf("semantic kinds exited %d: %s", code, stderr)
	}
	var listing struct {
		SlideKinds []semanticKindSummary `json:"slide_kinds"`
	}
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		t.Fatalf("semantic kinds --format json: %v\n%s", err, stdout)
	}
	if len(listing.SlideKinds) != len(mcpCatalogue.SlideKinds) {
		t.Fatalf("CLI lists %d kinds, list_slide_kinds %d", len(listing.SlideKinds), len(mcpCatalogue.SlideKinds))
	}
	for i, want := range mcpCatalogue.SlideKinds {
		got := listing.SlideKinds[i]
		if got.Kind != want.Kind || got.Summary != oneLineSummary(want.Summary) ||
			strings.Join(got.RequiredFields, ",") != strings.Join(want.RequiredFields, ",") {
			t.Errorf("kind %d: CLI %+v does not match list_slide_kinds %s", i, got, want.Kind)
		}
		if strings.ContainsAny(got.Summary, "\n") || len([]rune(got.Summary)) > 110 {
			t.Errorf("%s: listing summary is not one short line: %q", got.Kind, got.Summary)
		}
	}

	// The text listing: one line per kind, small.
	text, _, _ := cliRun(t, nil, "semantic", "kinds")
	if len(text) > 4096 {
		t.Errorf("semantic kinds prints %d bytes; the listing should stay under 4 KB", len(text))
	}
	for _, k := range mcpCatalogue.SlideKinds {
		if !regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(k.Kind) + `\s`).MatchString(text) {
			t.Errorf("text listing has no line for %s", k.Kind)
		}
	}

	// Per-kind detail: the JSON example is list_slide_kinds' example, and the
	// text form carries the same example as YAML ready to paste.
	for _, want := range mcpCatalogue.SlideKinds {
		stdout, stderr, code := cliRun(t, nil, "semantic", "kinds", want.Kind, "--format", "json")
		if code != 0 {
			t.Errorf("semantic kinds %s exited %d: %s", want.Kind, code, stderr)
			continue
		}
		var detail struct {
			SlideKind slideKindListEntry `json:"slide_kind"`
		}
		if err := json.Unmarshal([]byte(stdout), &detail); err != nil {
			t.Errorf("semantic kinds %s --format json: %v", want.Kind, err)
			continue
		}
		wantExample, _ := json.Marshal(want.Example)
		gotExample, _ := json.Marshal(detail.SlideKind.Example)
		if string(wantExample) != string(gotExample) {
			t.Errorf("%s: CLI example differs from list_slide_kinds", want.Kind)
		}
		if len(detail.SlideKind.ItemSchema) == 0 {
			t.Errorf("%s: detail carries no item_schema", want.Kind)
		}

		text, _, code := cliRun(t, nil, "semantic", "kinds", want.Kind)
		if code != 0 {
			t.Errorf("semantic kinds %s (text) exited %d", want.Kind, code)
			continue
		}
		_, example, ok := strings.Cut(text, "validates with zero findings):\n\n")
		if !ok || !strings.Contains(text, "Fields (type; budget; meaning):") {
			t.Errorf("%s: text detail lacks the fields table or the example:\n%.300s", want.Kind, text)
			continue
		}
		var slides []map[string]any
		if err := yaml.Unmarshal([]byte(example), &slides); err != nil || len(slides) != 1 {
			t.Errorf("%s: example is not a one-item YAML slide list: %v\n%s", want.Kind, err, example)
			continue
		}
		roundTrip, _ := json.Marshal(slides[0])
		if string(roundTrip) != string(wantExample) {
			t.Errorf("%s: YAML example does not round-trip to list_slide_kinds' example\n got %s\nwant %s", want.Kind, roundTrip, wantExample)
		}
	}
}

// TestSemanticKindExamplesValidate: the example `semantic kinds <kind>` prints
// is copy-ready — wrapped in a minimal DeckSpec it validates without errors.
func TestSemanticKindExamplesValidate(t *testing.T) {
	dir := t.TempDir()
	for _, k := range semantic.AllSlideKinds() {
		text, _, code := cliRun(t, nil, "semantic", "kinds", string(k))
		if code != 0 {
			t.Fatalf("semantic kinds %s exited %d", k, code)
		}
		_, example, ok := strings.Cut(text, "validates with zero findings):\n\n")
		if !ok {
			t.Fatalf("%s: no example in output", k)
		}
		spec := "meta:\n  title: Example deck\n  archetype: qbr\nslides:\n"
		for _, line := range strings.Split(strings.TrimRight(example, "\n"), "\n") {
			spec += "  " + line + "\n"
		}
		path := filepath.Join(dir, string(k)+".yaml")
		if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, _, _ := cliRun(t, nil, "semantic", "validate", path, "--strict", "off")
		var env struct {
			Findings []struct {
				Code     string `json:"code"`
				Severity string `json:"severity"`
				Message  string `json:"message"`
				Evidence struct {
					Path string `json:"path"`
				} `json:"evidence"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &env); err != nil {
			t.Errorf("%s: semantic validate output is not an envelope: %v\n%.300s", k, err, stdout)
			continue
		}
		for _, f := range env.Findings {
			// Deck-level advice about a one-slide deck is not about the example.
			if f.Severity == "error" && strings.HasPrefix(f.Evidence.Path, "slides[0]") {
				t.Errorf("%s: pasted example is refused: %s %s", k, f.Code, f.Message)
			}
		}
	}
}

// TestCLIGetStartedSpeaksCLI: every step carries the command line that runs
// it, the task is accepted positionally, and an unknown task is refused
// rather than answered as "brief" (go-slide-creator-e7jxr).
func TestCLIGetStartedSpeaksCLI(t *testing.T) {
	dispatchable := map[string]bool{}
	for _, name := range dispatchCommandNames(t) {
		dispatchable[name] = true
	}
	toolRE := regexp.MustCompile(`(?m)^\s*"tool": "([a-z_]+)",\n\s*"cli": "((?:[^"\\]|\\.)*)"`)
	anyTool := regexp.MustCompile(`"tool": "`)

	for _, task := range getStartedAvailableTasks() {
		stdout, stderr, code := cliRun(t, nil, "get-started", task, "--templates-dir", "../../templates")
		if code != 0 {
			t.Fatalf("get-started %s exited %d: %s", task, code, stderr)
		}
		var resp struct {
			Task string `json:"task"`
		}
		if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
			t.Fatalf("get-started %s: output is not JSON: %v", task, err)
		}
		if resp.Task != task {
			t.Errorf("get-started %s answered for task %q", task, resp.Task)
		}
		steps := toolRE.FindAllStringSubmatch(stdout, -1)
		if len(steps) == 0 || len(steps) != len(anyTool.FindAllString(stdout, -1)) {
			t.Errorf("get-started %s: %d tool steps, %d with a cli command", task, len(anyTool.FindAllString(stdout, -1)), len(steps))
		}
		for _, m := range steps {
			tool, cli := m[1], m[2]
			if strings.HasPrefix(cli, "(") {
				if !strings.Contains(cli, "MCP-only") {
					t.Errorf("%s: step %s has neither a command nor an MCP-only note: %s", task, tool, cli)
				}
				continue
			}
			words := strings.Fields(cli)
			if len(words) < 2 || words[0] != "json2pptx" || !dispatchable[words[1]] {
				t.Errorf("%s: step %s names a command the CLI does not have: %s", task, tool, cli)
			}
		}
	}

	// The DeckSpec path names the commands a CLI agent needs.
	stdout, _, _ := cliRun(t, nil, "get-started", "--templates-dir", "../../templates")
	wants := []string{"json2pptx semantic kinds", "json2pptx semantic validate", "json2pptx semantic render"}
	// A machine that cannot render is told so and loses the render steps
	// (degradeForMissingRenderTooling), so the thumbnails command is only
	// named where LibreOffice and ImageMagick are installed.
	if _, missing := renderDependencyStatus(); len(missing) == 0 {
		wants = append(wants, "json2pptx render-thumbnails")
	}
	for _, want := range wants {
		if !strings.Contains(stdout, want) {
			t.Errorf("get-started does not name %q", want)
		}
	}

	_, stderr, code := cliRun(t, nil, "get-started", "revize")
	if code == 0 || !strings.Contains(stderr, "unknown task") || !strings.Contains(stderr, "revise") {
		t.Errorf("unknown positional task: exit=%d stderr=%q; want a refusal listing the valid tasks", code, stderr)
	}
	if _, stderr, code := cliRun(t, nil, "get-started", "revise", "extra"); code == 0 || !strings.Contains(stderr, "unexpected argument") {
		t.Errorf("second positional: exit=%d stderr=%q; want it refused", code, stderr)
	}

	// --tool <name> is MCP get_started tool:"<name>": the tool's own
	// tool_detail, byte for byte, plus the command that does its job
	// (go-slide-creator-kkixz).
	for _, tc := range []struct{ tool, cli, then string }{
		{"render_deck_spec", "json2pptx semantic render <deck.yaml> --out <deck.pptx>", ""},
		{"list_slide_kinds", "json2pptx semantic kinds", "json2pptx semantic kinds <kind>"},
	} {
		stdout, stderr, code := cliRun(t, nil, "get-started", "--tool", tc.tool)
		if code != 0 {
			t.Fatalf("get-started --tool %s exited %d: %s", tc.tool, code, stderr)
		}
		var got struct {
			ToolDetail json.RawMessage `json:"tool_detail"`
			CLI        string          `json:"cli"`
			CLIThen    string          `json:"cli_then"`
		}
		decodeOneJSON(t, stdout, &got)
		mcpResult, err := cliMCPConfig("", "").handleGetStarted(context.Background(), mcpRequestWithArgs(map[string]any{"tool": tc.tool}))
		if err != nil || mcpResult.IsError {
			t.Fatalf("MCP get_started tool:%q: %v", tc.tool, err)
		}
		var want struct {
			ToolDetail json.RawMessage `json:"tool_detail"`
		}
		if err := json.Unmarshal([]byte(cliResultText(mcpResult)), &want); err != nil {
			t.Fatal(err)
		}
		var gotDetail, wantDetail any
		if json.Unmarshal(got.ToolDetail, &gotDetail) != nil || json.Unmarshal(want.ToolDetail, &wantDetail) != nil || !reflect.DeepEqual(gotDetail, wantDetail) {
			t.Errorf("get-started --tool %s: tool_detail differs from MCP get_started tool:%q", tc.tool, tc.tool)
		}
		if got.CLI != tc.cli || got.CLIThen != tc.then {
			t.Errorf("get-started --tool %s: cli %q then %q, want %q then %q", tc.tool, got.CLI, got.CLIThen, tc.cli, tc.then)
		}
	}
	if _, stderr, code := cliRun(t, nil, "get-started", "--tool", "render_deck"); code == 0 || !strings.Contains(stderr, "unknown tool") || !strings.Contains(stderr, "render_deck_spec") {
		t.Errorf("unknown tool: exit=%d stderr=%.200q; want a refusal listing the tools", code, stderr)
	}
	if _, stderr, code := cliRun(t, nil, "get-started", "revise", "--tool", "render_deck_spec"); code == 0 || !strings.Contains(stderr, "not both") {
		t.Errorf("--tool with a task: exit=%d stderr=%q; want it refused", code, stderr)
	}
}

// TestCLITemplatesAndIconDiscoveryAreSmall: template names in under 2 KB, an
// icon search that returns names, and a names-only icon listing
// (go-slide-creator-l7tg3).
func TestCLITemplatesAndIconDiscoveryAreSmall(t *testing.T) {
	stdout, stderr, code := cliRun(t, nil, "templates", "--templates-dir", "../../templates")
	if code != 0 {
		t.Fatalf("templates exited %d: %s", code, stderr)
	}
	if len(stdout) >= 2048 {
		t.Errorf("templates prints %d bytes, want under 2 KB", len(stdout))
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, name := range []string{"forest-green", "midnight-blue", "modern-template", "warm-coral"} {
		found := false
		for _, line := range lines {
			found = found || strings.HasPrefix(line, name+" ")
		}
		if !found {
			t.Errorf("templates has no line for %s:\n%s", name, stdout)
		}
	}

	stdout, _, code = cliRun(t, nil, "templates", "--format", "json", "--templates-dir", "../../templates")
	var listing struct {
		Templates []templateNameEntry `json:"templates"`
	}
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil || code != 0 || len(listing.Templates) != len(lines) {
		t.Errorf("templates --format json: err=%v exit=%d entries=%d lines=%d", err, code, len(listing.Templates), len(lines))
	}
	if len(stdout) >= 2048 {
		t.Errorf("templates --format json prints %d bytes, want under 2 KB", len(stdout))
	}

	// icons search: by name, and by business concept.
	stdout, _, code = cliRun(t, nil, "icons", "search", "risk")
	if code != 0 || !strings.Contains(stdout, "outline:alert-triangle") || len(stdout) > 2048 {
		t.Errorf("icons search risk: exit=%d bytes=%d\n%s", code, len(stdout), stdout)
	}
	stdout, _, code = cliRun(t, nil, "icons", "search", "chart", "--limit", "5", "--format", "json")
	var found iconSearchResult
	if err := json.Unmarshal([]byte(stdout), &found); err != nil || code != 0 {
		t.Fatalf("icons search --format json: err=%v exit=%d\n%s", err, code, stdout)
	}
	if len(found.Names) != 5 || found.TotalCount <= 5 {
		t.Errorf("icons search chart --limit 5: %d names of %d", len(found.Names), found.TotalCount)
	}
	for _, n := range found.Names {
		if !strings.Contains(n, ":") {
			t.Errorf("icon name %q is not qualified <set>:<name>", n)
		}
	}
	if _, stderr, code := cliRun(t, nil, "icons", "search"); code == 0 || !strings.Contains(stderr, "<term>") {
		t.Errorf("icons search without a term: exit=%d stderr=%q", code, stderr)
	}

	// icons list --names: nothing but qualified names, one per line.
	stdout, _, code = cliRun(t, nil, "icons", "list", "--names", "--set", "filled")
	if code != 0 {
		t.Fatalf("icons list --names exited %d", code)
	}
	names := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(names) < 100 {
		t.Fatalf("icons list --names returned %d lines", len(names))
	}
	for _, n := range names {
		if !strings.HasPrefix(n, "filled:") || strings.ContainsAny(n, " ,") {
			t.Fatalf("icons list --names line is not a bare qualified name: %q", n)
		}
	}
	stdout, _, _ = cliRun(t, nil, "icons", "list", "--names", "--set", "filled", "--format", "json")
	var flat []string
	if err := json.Unmarshal([]byte(stdout), &flat); err != nil || len(flat) != len(names) {
		t.Errorf("icons list --names --format json: err=%v count=%d want %d", err, len(flat), len(names))
	}

	// The top-level help states each discovery command's size class.
	help, _, _ := cliRun(t, nil, "help")
	for _, want := range []string{"Discovery commands by output size", "small", "templates", "semantic kinds", "icons search", "huge"} {
		if !strings.Contains(help, want) {
			t.Errorf("help does not mention %q in the discovery size table", want)
		}
	}
	for _, args := range [][]string{{"templates", "-h"}, {"icons", "list", "-h"}, {"icons", "search", "-h"}, {"semantic", "kinds", "-h"}} {
		out, _, _ := cliRun(t, nil, args...)
		if !strings.Contains(out, "small") && !strings.Contains(out, "large") {
			t.Errorf("%v does not state its output size class:\n%s", args, out)
		}
	}
}

// TestCLIRenderThumbnailsWritesFiles: one command writes slide-<index>.png
// files named the way `inspect` reads them and prints a small manifest whose
// hashes are the hashes of the files; base64 is opt-in; and the render leaves
// no temp directory behind (go-slide-creator-92gox, go-slide-creator-12dkn).
func TestCLIRenderThumbnailsWritesFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("renders through LibreOffice")
	}
	if !cliRenderToolsAvailable() {
		t.Skip("LibreOffice / ImageMagick not on PATH")
	}
	work := t.TempDir()
	tmp := filepath.Join(work, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"TMPDIR=" + tmp}
	deck := filepath.Join(work, "deck.pptx")
	if _, stderr, code := cliRun(t, env, "generate", "../../examples/basic-deck.json", "--out", deck, "--templates-dir", "../../templates"); code != 0 {
		t.Fatalf("generate failed: %s", stderr)
	}

	slides := filepath.Join(work, "slides")
	// A stale file from a longer earlier deck must not survive.
	if err := os.MkdirAll(slides, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(slides, "slide-99.png")
	if err := os.WriteFile(stale, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := cliRun(t, env, "render-thumbnails", deck, "--out-dir", slides)
	if code != 0 {
		t.Fatalf("render-thumbnails exited %d: %s\n%s", code, stderr, stdout)
	}
	if strings.Contains(stdout, "png_base64") {
		t.Error("base64 image data in the default output")
	}
	if len(stdout) > 4096 {
		t.Errorf("manifest is %d bytes for 7 slides; it should be small", len(stdout))
	}
	var manifest cliRenderManifest
	if err := json.Unmarshal([]byte(stdout), &manifest); err != nil {
		t.Fatalf("stdout is not the manifest: %v\n%s", err, stdout)
	}
	if manifest.SlideCount != 7 || len(manifest.Slides) != 7 || manifest.OutDir != slides {
		t.Fatalf("manifest = %+v; want 7 slides in %s", manifest, slides)
	}
	for i, s := range manifest.Slides {
		want := filepath.Join(slides, "slide-"+string(rune('0'+i))+".png")
		if s.Index != i || s.Path != want {
			t.Errorf("slide %d: index=%d path=%s, want %s", i, s.Index, s.Path, want)
		}
		data, err := os.ReadFile(s.Path)
		if err != nil {
			t.Errorf("slide %d: %v", i, err)
			continue
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != s.SHA256 || int64(len(data)) != s.Bytes {
			t.Errorf("slide %d: manifest sha256/bytes do not describe the file written", i)
		}
		if !bytes.HasPrefix(data, []byte("\x89PNG")) {
			t.Errorf("slide %d is not a PNG", i)
		}
		if !numberedSlideImageRE.MatchString(filepath.Base(s.Path)) {
			t.Errorf("slide file %s is not a name `inspect` reads", filepath.Base(s.Path))
		}
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a stale slide-99.png from an earlier deck was left in the output directory")
	}

	// --base64 restores the old envelope; --out is the canonical alias.
	stdout, _, code = cliRun(t, env, "render-thumbnails", deck, "--base64", "--max-slides", "1")
	if code != 0 || !strings.Contains(stdout, "png_base64") {
		t.Errorf("--base64: exit=%d, png_base64 present=%v", code, strings.Contains(stdout, "png_base64"))
	}
	alias := filepath.Join(work, "alias")
	if _, stderr, code := cliRun(t, env, "render-thumbnails", "--pptx", deck, "--out", alias, "--slides", "2"); code != 0 {
		t.Errorf("--pptx / --out aliases failed: %s", stderr)
	} else if _, err := os.Stat(filepath.Join(alias, "slide-2.png")); err != nil {
		t.Errorf("--slides 2 --out did not write slide-2.png: %v", err)
	}

	// render-slide writes one file.
	one := filepath.Join(work, "one.png")
	if _, stderr, code := cliRun(t, env, "render-slide", deck, "--slide-index", "1", "--out", one); code != 0 {
		t.Errorf("render-slide --out failed: %s", stderr)
	} else if _, err := os.Stat(one); err != nil {
		t.Errorf("render-slide --out did not write the file: %v", err)
	}

	// Nothing but the render cache is left in the temp directory.
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "json2pptx-render-cache" {
			t.Errorf("temp directory still holds %s after the commands exited", e.Name())
		}
	}
}

// TestCLISemanticRenderSidecar: the authoring manifest is written beside a
// rendered deck, --no-manifest turns it off, a failed render writes none and
// removes a stale one, and the help documents it (go-slide-creator-12dkn).
func TestCLISemanticRenderSidecar(t *testing.T) {
	work := t.TempDir()
	spec := "../../examples/semantic/qbr.yaml"

	deck := filepath.Join(work, "qbr.pptx")
	stdout, stderr, _ := cliRun(t, nil, "semantic", "render", spec, "--out", deck, "--templates-dir", "../../templates")
	if _, err := os.Stat(deck); err != nil {
		t.Fatalf("semantic render wrote no deck: %v\nstdout=%.300s\nstderr=%s", err, stdout, stderr)
	}
	if _, err := os.Stat(deck + ".authoring.json"); err != nil {
		t.Errorf("no sidecar beside the deck: %v", err)
	}
	if !strings.Contains(stdout, `"manifest_path"`) {
		t.Errorf("result does not name the sidecar in manifest_path")
	}

	quiet := filepath.Join(work, "quiet.pptx")
	stdout, _, _ = cliRun(t, nil, "semantic", "render", spec, "--out", quiet, "--no-manifest", "--templates-dir", "../../templates")
	if _, err := os.Stat(quiet); err != nil {
		t.Fatalf("--no-manifest render wrote no deck: %v", err)
	}
	if _, err := os.Stat(quiet + ".authoring.json"); err == nil {
		t.Error("--no-manifest still wrote the sidecar")
	}
	if strings.Contains(stdout, `"manifest_path"`) {
		t.Error("--no-manifest result still names a manifest_path")
	}

	// A failed render: no deck, no sidecar — including one an earlier render
	// of the same path left behind.
	failed := filepath.Join(work, "failed.pptx")
	if err := os.WriteFile(failed+".authoring.json", []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := cliRun(t, nil, "semantic", "render", "../../examples/semantic/invalid.yaml", "--out", failed, "--templates-dir", "../../templates")
	if code == 0 {
		t.Fatal("rendering invalid.yaml exited 0")
	}
	if _, err := os.Stat(failed); err == nil {
		t.Error("a failed render left a deck")
	}
	if _, err := os.Stat(failed + ".authoring.json"); err == nil {
		t.Error("a failed render left a sidecar for a deck that does not exist")
	}
	var res semanticRenderResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil || res.OK {
		t.Errorf("the failure result is not on stdout as JSON: err=%v\n%.300s", err, stdout)
	}

	help, _, _ := cliRun(t, nil, "semantic", "render", "-h")
	for _, want := range []string{".authoring.json", "--no-manifest", "manifest_path"} {
		if !strings.Contains(help, want) {
			t.Errorf("semantic render -h does not document %q", want)
		}
	}
}
