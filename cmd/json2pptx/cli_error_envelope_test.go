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
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
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
		// go-slide-creator-u1c9c's closing sweep: a --format json the flag
		// parser never reached, a group's unknown subcommand, a command with
		// its own argument parser, and values the MCP schema refuses but the
		// CLI passed through.
		{[]string{"patterns", "list", "--no-such-flag", "--format", "json"}, "patterns list", "INPUT.INVALID_PARAMETER"},
		{[]string{"validate-output", "--no-such-flag", "--json"}, "validate-output", "INPUT.INVALID_PARAMETER"},
		{[]string{"tables", "density", "--format", "json"}, "", "INPUT.INVALID_PARAMETER"},
		{[]string{"audit-palette", filepath.Join(t.TempDir(), "nope.pptx")}, "audit-palette", "INPUT.FILE_NOT_FOUND"},
		{[]string{"audit-palette", "--mode", "sideways", filepath.Join(t.TempDir(), "nope.pptx")}, "audit-palette", "INPUT.INVALID_PARAMETER"},
		{[]string{"shape-catalog", "--category", "no-such-category"}, "shape-catalog", "INPUT.INVALID_PARAMETER"},
		{[]string{"skill-info", "--template", "no-such-template", "--templates-dir", "../../templates", "--format", "json"}, "skill-info", "TPL.TEMPLATE_NOT_FOUND"},
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
	// generate keeps its own failure fields and carries the shared envelope
	// beside them, in the one document (go-slide-creator-fbft2).
	var generated struct {
		JSONOutput
		cliEnvelope
	}
	stdout, _, code = cliRun(t, nil, "generate", missing)
	decodeOneJSON(t, stdout, &generated)
	if code != 1 || generated.Success || generated.Error == "" {
		t.Errorf("generate: exit=%d result=%+v", code, generated)
	}
	if generated.OK == nil || *generated.OK || generated.Subcommand != "generate" || len(generated.Findings) != 1 ||
		generated.Findings[0].Code != "INPUT.FILE_NOT_FOUND" || generated.Findings[0].Message != generated.Error {
		t.Errorf("generate failure carries no shared envelope: %s", strings.TrimSpace(stdout))
	}
	// A typed argument error and a template that does not exist keep their
	// codes; any other failure is generation's own, not INTERNAL.
	deck := filepath.Join(t.TempDir(), "deck.json")
	for body, want := range map[string]string{
		`{"slides":[{"layout_id":"content"}]}`:                               "INPUT.REQUIRED",
		`{"template":"no-such-template","slides":[{"layout_id":"content"}]}`: "TEMPLATE_NOT_FOUND",
		`{"template":"midnight-blue","slides":[]}`:                           "INPUT.REQUIRED",
		// Deck content the shared pipeline refuses states what is wrong, in the
		// finding validate reports for it (go-slide-creator-u1c9c, -9k5fh);
		// what only generation knows stays its own.
		`{"template":"midnight-blue","template_path":"x.pptx","slides":[{"layout_id":"content"}]}`:                                                                                          "AMBIGUOUS_INPUT",
		`{"template":"midnight-blue","slides":[{"layout_id":"content","content":[{"placeholder_id":"body","type":"poem","text_value":"x"}]}]}`:                                              "UNKNOWN_ENUM",
		`{"template":"midnight-blue","slides":[{"layout_id":"blank-title","pattern":{"name":"kpi-3up","values":[{"big":"1","small":"a"}],"vertical_align":"sideways"}}]}`:                   "PATTERN_ERROR",
		`{"template":"midnight-blue","slides":[{"layout_id":"blank-title","shape_grid":{"rows":[{"cells":[{"shape":{"geometry":"rect","text":"a"}}]}]},"overlays":[{"kind":"squiggle"}]}]}`: "INVALID_SLIDE",
	} {
		if err := os.WriteFile(deck, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, _, code = cliRun(t, nil, "generate", deck, "--templates-dir", "../../templates", "--out", t.TempDir())
		generated.Findings = nil
		decodeOneJSON(t, stdout, &generated)
		if code != 1 || len(generated.Findings) == 0 || !strings.HasSuffix(generated.Findings[0].Code, want) {
			t.Errorf("generate %s: exit=%d, want a first finding ending %s: %s", body, code, want, strings.TrimSpace(stdout))
		}
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
		var res struct {
			JSONOutput
			cliEnvelope
		}
		decodeOneJSON(t, stdout, &res)
		if res.Success || !strings.Contains(res.Error, "strict-fit") || len(res.FitFindings) == 0 {
			t.Errorf("%v: result success=%v error=%q fit_findings=%d; want the refusal with its findings", extra, res.Success, res.Error, len(res.FitFindings))
		}
		// The shared envelope names the refusal first, then its findings.
		if res.OK == nil || *res.OK || res.Subcommand != "generate" || len(res.Findings) != 1+len(res.FitFindings) || !strings.HasSuffix(res.Findings[0].Code, "STRICT_FIT") {
			t.Errorf("%v: envelope ok=%v subcommand=%q findings=%d (fit_findings %d); want STRICT_FIT then each fit finding", extra, res.OK, res.Subcommand, len(res.Findings), len(res.FitFindings))
		}
		for _, line := range strings.Split(stderr, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "{") {
				t.Errorf("%v: stderr repeats a finding as JSON: %.160s", extra, line)
			}
		}
	}
}

// TestCLIDeckContentErrorsAreTyped: the errors the shared pipeline files
// return for deck content carry their code from where they are built, so no
// command that reads a deck reports them as INTERNAL ("retry"); a wrapping
// site keeps the code of the error it wraps (go-slide-creator-u1c9c).
func TestCLIDeckContentErrorsAreTyped(t *testing.T) {
	pattern := cliPatternError("pattern %q: does not support cell_overrides", "kpi-3up")
	for label, tc := range map[string]struct {
		err  error
		want diagnostics.Code
	}{
		"slide":                      {cliSlideError("slide %d, content %d: type is required", 2, 1), diagnostics.CodeInvalidSlide},
		"pattern":                    {pattern, diagnostics.CodePatternError},
		"pattern under a slide":      {cliSlideError("slide %d: shape_grid: %w", 2, pattern), diagnostics.CodePatternError},
		"plain under a slide":        {cliSlideError("slide %d: headline: %w", 2, errors.New("too long")), diagnostics.CodeInvalidSlide},
		"pattern input findings":     {cliSlideError("slide %d: %w", 2, newPatternInputError("kpi-3up", []*patterns.ValidationError{{Code: "required", Message: "values[0].small is required"}})), diagnostics.CodePatternError},
		"missing file under a slide": {cliSlideError("slide %d: %w", 2, os.ErrNotExist), diagnostics.CodeFileNotFound},
	} {
		if got := cliErrorCode(tc.err); got != tc.want {
			t.Errorf("%s: code = %s, want %s", label, got, tc.want)
		}
	}
	// The three files build no untyped error for deck content.
	_, _, err := expandPattern(&PatternInput{Name: "kpi-3up", Values: json.RawMessage(`[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]`), VerticalAlign: "sideways"}, patterns.ExpandContext{}, patterns.Default())
	if err == nil || cliErrorCode(err) != diagnostics.CodePatternError {
		t.Errorf("expandPattern vertical_align error: %v has code %s", err, cliErrorCode(err))
	}
	_, _, err = resolveOverlays([]*OverlayShapeInput{{Kind: "squiggle"}}, nil, &pptx.ShapeIDAllocator{}, 12192000, 6858000, overlayEnv{})
	if err == nil || cliErrorCode(err) != diagnostics.CodeInvalidSlide {
		t.Errorf("resolveOverlays unknown kind: %v has code %s", err, cliErrorCode(err))
	}
	_, err = convertPresentationContent([]ContentInput{{PlaceholderID: "body"}}, 1, types.SlideTypeContent)
	if err == nil || cliErrorCode(err) != diagnostics.CodeInvalidSlide {
		t.Errorf("convertPresentationContent missing type: %v has code %s", err, cliErrorCode(err))
	}

	// A pattern's located input findings reach the CLI one per field, coded,
	// instead of joined into one INTERNAL message.
	values := filepath.Join(t.TempDir(), "values.json")
	if err := os.WriteFile(values, []byte(`[{"big":"1"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, code := cliRun(t, nil, "patterns", "expand", "kpi-3up", values, "--format", "json")
	var env cliEnvelope
	decodeOneJSON(t, stdout, &env)
	if code != 1 || env.Subcommand != "patterns expand" || len(env.Findings) < 2 {
		t.Fatalf("patterns expand: exit=%d envelope=%s", code, strings.TrimSpace(stdout))
	}
	for _, f := range env.Findings {
		if strings.HasSuffix(f.Code, "INTERNAL") {
			t.Errorf("patterns expand reports %s: %s", f.Code, f.Message)
		}
	}
}

// TestCLIValidateOutputEnvelope: --format json answers with the shared finding
// envelope and the per-file results under files[]; the deprecated --json alone
// keeps the bare array (go-slide-creator-u1c9c).
func TestCLIValidateOutputEnvelope(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.pptx")
	good := "../../templates/midnight-blue.pptx"
	type fileResult struct {
		FilePath string `json:"file_path"`
		IsValid  bool   `json:"is_valid"`
		Error    string `json:"error"`
	}
	var env struct {
		cliEnvelope
		Files []fileResult `json:"files"`
	}
	stdout, _, code := cliRun(t, nil, "validate-output", "--format", "json", good, missing)
	decodeOneJSON(t, stdout, &env)
	if code != 1 || env.OK == nil || *env.OK || env.Subcommand != "validate-output" || len(env.Files) != 2 ||
		!env.Files[0].IsValid || env.Files[1].Error == "" {
		t.Fatalf("validate-output --format json: exit=%d %s", code, strings.TrimSpace(stdout))
	}
	if n := len(env.Findings); n == 0 || env.Findings[n-1].Code != "INPUT.FILE_NOT_FOUND" || !strings.Contains(env.Findings[n-1].Message, missing) {
		t.Errorf("the unreadable file is not a FILE_NOT_FOUND finding naming it: %+v", env.Findings)
	}

	env.OK, env.Files = nil, nil
	stdout, _, code = cliRun(t, nil, "validate-output", "--format", "json", good)
	decodeOneJSON(t, stdout, &env)
	if code != 0 || env.OK == nil || !*env.OK || len(env.Files) != 1 || !env.Files[0].IsValid {
		t.Errorf("validate-output on a valid file: exit=%d %s", code, strings.TrimSpace(stdout))
	}

	var legacy []fileResult
	stdout, _, code = cliRun(t, nil, "validate-output", "--json", good, missing)
	decodeOneJSON(t, stdout, &legacy)
	if code != 1 || len(legacy) != 2 || !legacy[0].IsValid || legacy[1].Error == "" {
		t.Errorf("validate-output --json no longer prints the array: exit=%d %s", code, strings.TrimSpace(stdout))
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
		// An argument error states its code where it is returned, and keeps
		// it through any wrapping (go-slide-creator-fbft2).
		{cliMissingArg("--fixes is required"), diagnostics.CodeMissingParameter},
		{fmt.Errorf("repair: %w", cliMissingArg("--fixes is required")), diagnostics.CodeMissingParameter},
		{cliNotFound("template file not found: %s", "x.pptx"), diagnostics.CodeFileNotFound},
		{cliInvalidArg("unknown pattern %q", "x"), diagnostics.CodeInvalidParameter},
		{cliInvalidJSON("--fixes: invalid JSON: %w", errors.New("unexpected end")), diagnostics.CodeInvalidJSON},
		{fmt.Errorf("resolve: %w", errTemplateNameNotFound), diagnostics.CodeTemplateNotFound},
		// Nothing is read from the wording: an untyped error is the
		// command's own failure, however it is phrased.
		{errors.New("--fixes is required"), diagnostics.CodeInternal},
		{errors.New(`unknown pattern "x"`), diagnostics.CodeInternal},
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
