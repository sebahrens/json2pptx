package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// cliEnvelope is the finding envelope as the CLI prints it.
type cliEnvelope struct {
	Subcommand string `json:"subcommand"`
	OK         *bool  `json:"ok"`
	Findings   []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"findings"`
}

// decodeOneJSON decodes stdout as exactly one JSON document.
func decodeOneJSON(t *testing.T, stdout string, into any) {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(stdout))
	if err := dec.Decode(into); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%.300s", err, stdout)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout carries more than one JSON document:\n%.600s", stdout)
	}
}

// TestCLICatchAllErrorEnvelope: a JSON command that fails with a plain Go
// error prints one finding envelope on stdout, named after the command; a
// command answering in text keeps stdout empty; and a command that reported
// its own failure is not given a second document (go-slide-creator-pikfw).
func TestCLICatchAllErrorEnvelope(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.json")
	for _, tc := range []struct {
		args       []string
		subcommand string
		code       string
	}{
		{[]string{"score", missing}, "score", "INPUT.FILE_NOT_FOUND"},
		{[]string{"analyze-rhythm", missing}, "analyze-rhythm", "INPUT.FILE_NOT_FOUND"},
		{[]string{"render-slide", "--no-such-flag"}, "render-slide", "INPUT.INVALID_PARAMETER"},
		{[]string{"plan-deck", "--no-such-flag"}, "plan-deck", "INPUT.INVALID_PARAMETER"},
		{[]string{"repair", missing}, "repair", "INPUT.MISSING_PARAMETER"},
		{[]string{"patterns", "show", "no-such-pattern", "--format", "json"}, "patterns show", "INPUT.INVALID_PARAMETER"},
		{[]string{"generate", "--strict-fit", "sometimes", missing}, "generate", "INPUT.INVALID_PARAMETER"},
	} {
		stdout, stderr, code := cliRun(t, nil, tc.args...)
		if code != 1 {
			t.Errorf("%v exited %d, want 1", tc.args, code)
		}
		var env cliEnvelope
		decodeOneJSON(t, stdout, &env)
		if env.OK == nil || *env.OK || env.Subcommand != tc.subcommand || len(env.Findings) != 1 || env.Findings[0].Code != tc.code {
			t.Errorf("%v: envelope = %s; want one %s finding from %q", tc.args, strings.TrimSpace(stdout), tc.code, tc.subcommand)
			continue
		}
		if !strings.Contains(stderr, env.Findings[0].Message) {
			t.Errorf("%v: stderr no longer carries the error line: %q", tc.args, stderr)
		}
	}

	// Text output: the failure stays a stderr line.
	for _, args := range [][]string{
		{"patterns", "show", "no-such-pattern"},
		{"validate-template", filepath.Join(t.TempDir(), "nope.pptx")},
		{"no-such-command"},
	} {
		stdout, stderr, code := cliRun(t, nil, args...)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "Error: ") {
			t.Errorf("%v: exit=%d stdout=%q stderr=%q; want a stderr error and empty stdout", args, code, stdout, stderr)
		}
	}

	// Commands that report their own failure keep their one document.
	var validate validateJSONShape
	stdout, _, code := cliRun(t, nil, "validate", "--format", "json", missing)
	decodeOneJSON(t, stdout, &validate)
	if code != 1 || validate.Valid == nil || *validate.Valid {
		t.Errorf("validate: exit=%d valid=%v", code, validate.Valid)
	}
	var generated JSONOutput
	stdout, _, code = cliRun(t, nil, "generate", missing)
	decodeOneJSON(t, stdout, &generated)
	if code != 1 || generated.Success || generated.Error == "" {
		t.Errorf("generate: exit=%d result=%+v", code, generated)
	}

	// preflight exits on its own; its envelope must still reach stdout.
	var pre cliEnvelope
	stdout, _, code = cliRun(t, nil, "preflight", missing)
	decodeOneJSON(t, stdout, &pre)
	if code != 2 || pre.Subcommand != "preflight" || len(pre.Findings) != 1 {
		t.Errorf("preflight: exit=%d envelope=%s", code, strings.TrimSpace(stdout))
	}
}

// TestCLIStrictFitFindingsReportedOnce: a strict-fit refusal carries its
// findings in the stdout result — with or without a report destination — and
// no longer repeats them as NDJSON on stderr (go-slide-creator-pikfw).
func TestCLIStrictFitFindingsReportedOnce(t *testing.T) {
	dir := t.TempDir()
	deck := filepath.Join(dir, "deck.json")
	long := strings.Repeat("This is very long overflow text that cannot fit. ", 60)
	input := fmt.Sprintf(`{"template":"midnight-blue","slides":[{"layout_id":"content","content":[{"placeholder_id":"body","type":"table","table_value":{"headers":["A","B","C","D","E"],"rows":[[{"content":%q},{"content":"ok"},{"content":"ok"},{"content":"ok"},{"content":"ok"}]]}}]}]}`, long)
	if err := os.WriteFile(deck, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{nil, {"--json-output-report", "-"}} {
		args := append([]string{"generate", deck, "--out", filepath.Join(dir, "out"), "--templates-dir", "../../templates", "--strict-fit", "strict"}, extra...)
		stdout, stderr, code := cliRun(t, nil, args...)
		if code != 1 {
			t.Fatalf("%v exited %d\n%s", extra, code, stderr)
		}
		var res JSONOutput
		decodeOneJSON(t, stdout, &res)
		if res.Success || !strings.Contains(res.Error, "strict-fit") || len(res.FitFindings) == 0 {
			t.Errorf("%v: result success=%v error=%q fit_findings=%d; want the refusal with its findings", extra, res.Success, res.Error, len(res.FitFindings))
		}
		for _, line := range strings.Split(stderr, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "{") {
				t.Errorf("%v: stderr repeats a finding as JSON: %.160s", extra, line)
			}
		}
	}
}

func TestCLIWantsJSON(t *testing.T) {
	saved := cliCurrentFlagSet
	defer func() { cliCurrentFlagSet = saved }()

	parse := func(name string, define func(*flag.FlagSet), args ...string) bool {
		fs := flag.NewFlagSet(name, flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		if define != nil {
			define(fs)
		}
		_ = cliParse(fs, args)
		return cliWantsJSON()
	}
	jsonFlag := func(fs *flag.FlagSet) { fs.Bool("json", false, "") }
	ownFormat := func(def string) func(*flag.FlagSet) {
		return func(fs *flag.FlagSet) { fs.String("format", def, ""); fs.Bool("json", false, "") }
	}
	for _, tc := range []struct {
		label  string
		got    bool
		expect bool
	}{
		{"JSON-only command", parse("score", nil), true},
		{"JSON-only command, bad flag", parse("score", nil, "--nope"), true},
		{"text command", parse("templates", jsonFlag), false},
		{"text command --json", parse("templates", jsonFlag, "--json"), true},
		{"text command --format json", parse("templates", jsonFlag, "--format", "json"), true},
		{"validate, human", parse("validate", ownFormat("")), false},
		{"validate --format ndjson", parse("validate", ownFormat(""), "--format", "ndjson"), true},
		{"validate --json", parse("validate", ownFormat(""), "--json"), true},
		{"plan-deck --format raw selects content", parse("plan-deck", ownFormat("raw")), true},
		{"server", parse("mcp", nil), false},
	} {
		if tc.got != tc.expect {
			t.Errorf("%s: cliWantsJSON = %v, want %v", tc.label, tc.got, tc.expect)
		}
	}
	cliCurrentFlagSet = nil
	if cliWantsJSON() {
		t.Error("no command parsed: cliWantsJSON = true")
	}
}

func TestCLIErrorCode(t *testing.T) {
	saved := cliParseFailed
	defer func() { cliParseFailed = saved }()
	cliParseFailed = false

	_, notExist := os.Stat(filepath.Join(t.TempDir(), "nope"))
	var v struct{}
	syntax := json.Unmarshal([]byte("{"), &v)
	for _, tc := range []struct {
		err  error
		want diagnostics.Code
	}{
		{fmt.Errorf("read deck: %w", notExist), diagnostics.CodeFileNotFound},
		{fmt.Errorf("parse: %w", syntax), diagnostics.CodeInvalidJSON},
		{errors.New("--fixes is required"), diagnostics.CodeMissingParameter},
		{errors.New("template file not found: x.pptx"), diagnostics.CodeFileNotFound},
		{errors.New(`unknown pattern "x"`), diagnostics.CodeInvalidParameter},
		{errors.New("LibreOffice exited with status 1"), diagnostics.CodeInternal},
	} {
		if got := cliErrorCode(tc.err); got != tc.want {
			t.Errorf("cliErrorCode(%q) = %s, want %s", tc.err, got, tc.want)
		}
	}
	cliParseFailed = true
	if got := cliErrorCode(errors.New("flag provided but not defined: -x")); got != diagnostics.CodeInvalidParameter {
		t.Errorf("a refused argument list = %s, want %s", got, diagnostics.CodeInvalidParameter)
	}
}
