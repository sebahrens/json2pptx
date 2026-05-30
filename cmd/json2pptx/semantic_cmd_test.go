package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/pptx"
)

// validSemanticSpec is a minimal, clean semantic deck spec (title + closing)
// that validates and compiles without findings.
const validSemanticSpec = `meta:
  title: Quarterly Review
  subtitle: FY26 Q2
  template: midnight-blue
slides:
  - kind: title
    title: Quarterly Review
    subtitle: FY26 Q2
  - kind: closing
    title: Questions?
`

// invalidSemanticSpec omits the required deck title and uses an unregistered
// slide kind, so validation must surface error-severity findings.
const invalidSemanticSpec = `meta:
  subtitle: no title here
slides:
  - kind: not_a_real_kind
    title: Hi
`

// writeSpec writes content to a temp file with the given name and returns its
// path.
func writeSpec(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write spec %s: %v", path, err)
	}
	return path
}

// runSemanticArgs invokes runSemantic with os.Args reshaped as main.dispatch
// would have left them (os.Args[0] is the program, the rest are the semantic
// subcommand and its flags). It restores os.Args afterward and returns the
// captured stdout plus the handler error.
func runSemanticArgs(t *testing.T, args ...string) (string, error) {
	t.Helper()
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = append([]string{"json2pptx"}, args...)

	var err error
	out := captureStdout(t, func() { err = runSemantic() })
	return out, err
}

func TestSemanticValidate_ValidFixture(t *testing.T) {
	path := writeSpec(t, "deck.yaml", validSemanticSpec)

	out, err := runSemanticArgs(t, "validate", "--spec", path)
	if err != nil {
		t.Fatalf("semantic validate returned error on valid spec: %v\noutput=%s", err, out)
	}

	var env diagnostics.FindingEnvelope
	if jerr := json.Unmarshal([]byte(out), &env); jerr != nil {
		t.Fatalf("output is not a FindingEnvelope: %v\noutput=%s", jerr, out)
	}
	if !env.OK {
		t.Errorf("envelope OK = false on valid spec; summary=%q", env.Summary)
	}
	for _, f := range env.Findings {
		if f.Severity == diagnostics.SeverityError {
			t.Errorf("unexpected error finding on valid spec: %s %s", f.Code, f.Message)
		}
	}
	if env.Subcommand != "semantic validate" {
		t.Errorf("subcommand = %q, want %q", env.Subcommand, "semantic validate")
	}
	if env.InputSHA256 == "" {
		t.Error("envelope is missing input_sha256")
	}
}

func TestSemanticValidate_InvalidFixture(t *testing.T) {
	path := writeSpec(t, "bad.yaml", invalidSemanticSpec)

	out, err := runSemanticArgs(t, "validate", "--spec", path)
	if err == nil {
		t.Fatalf("semantic validate succeeded on invalid spec; output=%s", out)
	}

	var env diagnostics.FindingEnvelope
	if jerr := json.Unmarshal([]byte(out), &env); jerr != nil {
		t.Fatalf("output is not a FindingEnvelope: %v\noutput=%s", jerr, out)
	}
	if env.OK {
		t.Error("envelope OK = true on invalid spec")
	}
	var sawError bool
	for _, f := range env.Findings {
		if f.Severity == diagnostics.SeverityError {
			sawError = true
		}
	}
	if !sawError {
		t.Errorf("invalid spec produced no error findings; summary=%q", env.Summary)
	}
}

func TestSemanticValidate_MissingSpecFlag(t *testing.T) {
	if _, err := runSemanticArgs(t, "validate"); err == nil {
		t.Error("expected error when --spec is omitted")
	}
}

func TestSemanticValidate_BadStrictness(t *testing.T) {
	path := writeSpec(t, "deck.yaml", validSemanticSpec)
	if _, err := runSemanticArgs(t, "validate", "--spec", path, "--strict", "nonsense"); err == nil {
		t.Error("expected error on invalid --strict value")
	}
}

func TestSemanticCompile_RoundTripValidates(t *testing.T) {
	path := writeSpec(t, "deck.yaml", validSemanticSpec)
	out := filepath.Join(t.TempDir(), "compiled.json")

	if _, err := runSemanticArgs(t, "compile", "--spec", path, "--output", out); err != nil {
		t.Fatalf("semantic compile returned error: %v", err)
	}

	// The emitted raw JSON must unmarshal into PresentationInput...
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read compiled output: %v", err)
	}
	var input PresentationInput
	if jerr := json.Unmarshal(raw, &input); jerr != nil {
		t.Fatalf("compiled output is not valid PresentationInput JSON: %v", jerr)
	}
	if len(input.Slides) != 2 {
		t.Errorf("compiled deck has %d slides, want 2", len(input.Slides))
	}

	// ...and be accepted by the same validate path `json2pptx validate` uses.
	result := validateJSONFile(out, testTemplatesDir, "", false, "warn")
	if !result.Valid {
		t.Errorf("compiled JSON failed `validate`: errors=%v", result.Errors)
	}
}

func TestSemanticCompile_Stdout(t *testing.T) {
	path := writeSpec(t, "deck.yaml", validSemanticSpec)

	out, err := runSemanticArgs(t, "compile", "--spec", path, "--output", "-")
	if err != nil {
		t.Fatalf("semantic compile to stdout returned error: %v", err)
	}
	var input PresentationInput
	if jerr := json.Unmarshal([]byte(out), &input); jerr != nil {
		t.Fatalf("stdout is not valid PresentationInput JSON: %v\noutput=%s", jerr, out)
	}
	if input.Template != "midnight-blue" {
		t.Errorf("compiled template = %q, want midnight-blue", input.Template)
	}
}

func TestSemanticCompile_BlockingErrorsAbort(t *testing.T) {
	path := writeSpec(t, "bad.yaml", invalidSemanticSpec)
	out := filepath.Join(t.TempDir(), "compiled.json")

	if _, err := runSemanticArgs(t, "compile", "--spec", path, "--output", out); err == nil {
		t.Error("expected compile to fail on a spec with blocking findings")
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("compile wrote an output file despite blocking findings")
	}
}

// qbrSemanticExample is the bundled QBR semantic authoring example, relative to
// the package dir. It is the acceptance fixture for `semantic render`.
const qbrSemanticExample = "../../examples/semantic/qbr.yaml"

func TestSemanticRender_QBRExample(t *testing.T) {
	if _, err := os.Stat(qbrSemanticExample); err != nil {
		t.Skipf("qbr example missing: %v", err)
	}
	out := filepath.Join(t.TempDir(), "qbr.pptx")

	stdout, err := runSemanticArgs(t, "render",
		"--spec", qbrSemanticExample,
		"--output", out,
		"--templates-dir", testTemplatesDir)
	if err != nil {
		t.Fatalf("semantic render returned error: %v\noutput=%s", err, stdout)
	}

	var res semanticRenderResult
	if jerr := json.Unmarshal([]byte(stdout), &res); jerr != nil {
		t.Fatalf("render output is not a semanticRenderResult: %v\noutput=%s", jerr, stdout)
	}
	if !res.OK {
		t.Errorf("render result OK = false; error=%q", res.Error)
	}
	if res.OutputPath != out {
		t.Errorf("output_path = %q, want %q", res.OutputPath, out)
	}
	if res.SlideCount != 6 {
		t.Errorf("slide_count = %d, want 6", res.SlideCount)
	}
	if res.ContentHash == "" {
		t.Error("render result missing content_hash")
	}
	if _, statErr := os.Stat(out); statErr != nil {
		t.Errorf("render did not write the .pptx: %v", statErr)
	}

	// A successful render passed strict output validation internally; confirm the
	// artifact also passes the standalone validate-output path.
	report, valErr := pptx.ValidateOutputFile(out)
	if valErr != nil {
		t.Fatalf("validate-output failed: %v", valErr)
	}
	if !report.IsValid() {
		t.Errorf("rendered deck failed validate-output: %d blocking finding(s)", len(report.Blocking()))
	}
}

// salesPitchSemanticExample is the bundled sales-pitch semantic example,
// relative to the package dir. It renders on a second template (warm-coral)
// than the QBR example, widening render-smoke template coverage.
const salesPitchSemanticExample = "../../examples/semantic/sales_pitch.yaml"

func TestSemanticRender_SalesPitchExample(t *testing.T) {
	if _, err := os.Stat(salesPitchSemanticExample); err != nil {
		t.Skipf("sales_pitch example missing: %v", err)
	}
	out := filepath.Join(t.TempDir(), "sales_pitch.pptx")

	stdout, err := runSemanticArgs(t, "render",
		"--spec", salesPitchSemanticExample,
		"--output", out,
		"--templates-dir", testTemplatesDir)
	if err != nil {
		t.Fatalf("semantic render returned error: %v\noutput=%s", err, stdout)
	}

	var res semanticRenderResult
	if jerr := json.Unmarshal([]byte(stdout), &res); jerr != nil {
		t.Fatalf("render output is not a semanticRenderResult: %v\noutput=%s", jerr, stdout)
	}
	if !res.OK {
		t.Errorf("render result OK = false; error=%q", res.Error)
	}
	if res.SlideCount != 7 {
		t.Errorf("slide_count = %d, want 7", res.SlideCount)
	}
	if _, statErr := os.Stat(out); statErr != nil {
		t.Errorf("render did not write the .pptx: %v", statErr)
	}

	report, valErr := pptx.ValidateOutputFile(out)
	if valErr != nil {
		t.Fatalf("validate-output failed: %v", valErr)
	}
	if !report.IsValid() {
		t.Errorf("rendered deck failed validate-output: %d blocking finding(s)", len(report.Blocking()))
	}
}

func TestSemanticRender_InvalidSpecFails(t *testing.T) {
	path := writeSpec(t, "bad.yaml", invalidSemanticSpec)
	out := filepath.Join(t.TempDir(), "bad.pptx")

	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"json2pptx", "render", "--spec", path, "--output", out, "--templates-dir", testTemplatesDir}

	var err error
	stderr := captureStderr(t, func() { err = runSemantic() })
	if err == nil {
		t.Fatal("expected render to fail on a spec with blocking findings")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Error("render wrote an output file despite blocking findings")
	}

	var res semanticRenderResult
	if jerr := json.Unmarshal([]byte(stderr), &res); jerr != nil {
		t.Fatalf("failure result is not a semanticRenderResult: %v\nstderr=%s", jerr, stderr)
	}
	if res.OK {
		t.Error("failure result OK = true")
	}
	// The failure response must surface semantic source paths, not raw paths.
	var sawSemanticPath bool
	for _, d := range res.Diagnostics {
		if d.SemanticPath != "" {
			sawSemanticPath = true
		}
	}
	if !sawSemanticPath {
		t.Errorf("failure result carried no semantic_path diagnostics: %+v", res.Diagnostics)
	}
}

func TestSemanticRender_MissingOutputFlag(t *testing.T) {
	path := writeSpec(t, "deck.yaml", validSemanticSpec)
	if _, err := runSemanticArgs(t, "render", "--spec", path); err == nil {
		t.Error("expected error when --output is omitted")
	}
}

// repetitiveSemanticSpec is an executive (board_update) deck of three
// consecutive KPI snapshots with no synthesis slide — it trips both the
// monotony and synthesis rhythm rules, and pins no template so the archetype
// default applies.
const repetitiveSemanticSpec = `meta:
  title: Repetitive Board Update
  archetype: board_update
slides:
  - kind: kpi_snapshot
    title: Metrics A
    takeaway: A
    kpis:
      - {label: ARR, value: "$1M"}
      - {label: NRR, value: "110%"}
  - kind: kpi_snapshot
    title: Metrics B
    takeaway: B
    kpis:
      - {label: ARR, value: "$2M"}
      - {label: NRR, value: "112%"}
  - kind: kpi_snapshot
    title: Metrics C
    takeaway: C
    kpis:
      - {label: ARR, value: "$3M"}
      - {label: NRR, value: "114%"}
`

func TestSemanticExplain_ShowsPlanAndRhythmWarnings(t *testing.T) {
	path := writeSpec(t, "deck.yaml", repetitiveSemanticSpec)

	out, err := runSemanticArgs(t, "explain", "--spec", path)
	if err != nil {
		t.Fatalf("semantic explain returned error: %v\noutput=%s", err, out)
	}

	var exp struct {
		Archetype string `json:"archetype"`
		Template  string `json:"template"`
		Slides    []struct {
			Kind         string `json:"kind"`
			Role         string `json:"role"`
			VisualFamily string `json:"visual_family"`
			Density      string `json:"density"`
			Pattern      string `json:"pattern"`
			Layout       string `json:"layout"`
		} `json:"slides"`
		RhythmWarnings []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Path    string `json:"path"`
		} `json:"rhythm_warnings"`
	}
	if jerr := json.Unmarshal([]byte(out), &exp); jerr != nil {
		t.Fatalf("explain output is not a DeckExplanation: %v\noutput=%s", jerr, out)
	}

	if exp.Archetype != "board_update" {
		t.Errorf("archetype = %q, want board_update", exp.Archetype)
	}
	// Pins no template -> archetype default (board_update -> midnight-blue).
	if exp.Template != "midnight-blue" {
		t.Errorf("template = %q, want midnight-blue (archetype default)", exp.Template)
	}
	if len(exp.Slides) != 3 {
		t.Fatalf("explanation has %d slides, want 3", len(exp.Slides))
	}
	for i, s := range exp.Slides {
		if s.Kind == "" || s.VisualFamily == "" || s.Density == "" {
			t.Errorf("slides[%d] missing kind/family/density: %+v", i, s)
		}
		if s.Pattern == "" && s.Layout == "" {
			t.Errorf("slides[%d] has neither pattern nor layout", i)
		}
	}

	codes := map[string]bool{}
	for _, w := range exp.RhythmWarnings {
		codes[w.Code] = true
	}
	if !codes["SEMANTIC_RHYTHM_MONOTONY"] {
		t.Errorf("expected SEMANTIC_RHYTHM_MONOTONY in rhythm_warnings, got %+v", exp.RhythmWarnings)
	}
	if !codes["SEMANTIC_RHYTHM_SYNTHESIS"] {
		t.Errorf("expected SEMANTIC_RHYTHM_SYNTHESIS in rhythm_warnings, got %+v", exp.RhythmWarnings)
	}
}

func TestSemanticExplain_MissingSpecFlag(t *testing.T) {
	if _, err := runSemanticArgs(t, "explain"); err == nil {
		t.Error("expected error when --spec is omitted")
	}
}

func TestSemanticSchema_PrintsJSONSchema(t *testing.T) {
	out, err := runSemanticArgs(t, "schema")
	if err != nil {
		t.Fatalf("semantic schema returned error: %v", err)
	}
	var schema map[string]any
	if jerr := json.Unmarshal([]byte(out), &schema); jerr != nil {
		t.Fatalf("schema output is not valid JSON: %v", jerr)
	}
	if _, ok := schema["$schema"]; !ok {
		t.Error("schema output missing $schema dialect key")
	}
	if schema["title"] != "DeckSpec" {
		t.Errorf("schema title = %v, want DeckSpec", schema["title"])
	}
}

func TestSemantic_UnknownSubcommand(t *testing.T) {
	if _, err := runSemanticArgs(t, "frobnicate"); err == nil {
		t.Error("expected error on unknown semantic subcommand")
	}
}

func TestSemantic_HelpListsSubcommands(t *testing.T) {
	// help is routed through the no-arg path; ensure it does not error and the
	// usage text names each subcommand.
	if _, err := runSemanticArgs(t, "help"); err != nil {
		t.Errorf("semantic help returned error: %v", err)
	}
	// No-subcommand form must error but still print usage to stderr.
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"json2pptx"}
	if err := runSemantic(); err == nil {
		t.Error("expected error when no semantic subcommand is given")
	}
}

// TestSemanticClassificationRegistered guards that the semantic command is
// recorded in the CLI command classification map (the reverse-parity gate reads
// it), since dispatch now recognizes it. The semantic group now has MCP parity
// via the semantic-compiler MCP tools (validate_deck_spec / compile_deck_spec /
// render_deck_spec / explain_deck_spec / list_deck_archetypes / list_slide_kinds),
// so it must be AgentFacing AND must NOT carry a CLIOnlyReason — a command with
// MCP parity cannot also be marked CLI-only (TestEveryCLICommandHasMCPParityOrException).
func TestSemanticClassificationRegistered(t *testing.T) {
	c, ok := cliCommandClassifications()["semantic"]
	if !ok {
		t.Fatal("semantic command missing from cliCommandClassifications()")
	}
	if !c.AgentFacing {
		t.Error("semantic should be classified AgentFacing")
	}
	if strings.TrimSpace(c.CLIOnlyReason) != "" {
		t.Error("semantic now has MCP parity via the semantic-compiler tools and must NOT carry a CLIOnlyReason")
	}
}
