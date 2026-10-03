package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// The agent journey, tracked (go-slide-creator-3pxl6).
//
// Five first-time-user agents scored the tool 6-7 of 10 on 2026-10-03
// (tests/quality/results/agent-journey-20261003/report.json). What made their
// journeys long is deterministic and each piece has its own acceptance test;
// TestAgentJourneyMetrics gathers those measurements into one set of numbers,
// holds each to its threshold, and compares the set with the last row of
// tests/quality/journey/results.jsonl, so a journey that gets longer or
// heavier fails here and a row per recorded run shows the trend.
//
//	JOURNEY_RECORD=1 go test -short ./cmd/json2pptx -run TestAgentJourneyMetrics
//
// appends the current tree's row. tests/quality/journey/README.md has the
// harness the agents themselves run through.

// journeyResultsPath is the committed trend file, one JSON row per line.
var journeyResultsPath = filepath.Join("..", "..", "tests", "quality", "journey", "results.jsonl")

// journeyRecordEnv, set to 1, appends the measured row to the results file.
const journeyRecordEnv = "JOURNEY_RECORD"

// journeyTestSource marks the rows this test writes; an agent run's row has
// another source and carries personas instead of metrics.
const journeyTestSource = "TestAgentJourneyMetrics"

// journeyResultRow is one recorded run.
type journeyResultRow struct {
	Date              string `json:"date"`
	Commit            string `json:"commit"`
	SchemaVersion     string `json:"schema_version,omitempty"`
	SchemaFingerprint string `json:"schema_fingerprint,omitempty"`
	Source            string `json:"source"`
	Note              string `json:"note,omitempty"`
	// Metrics are the deterministic numbers of journeyMetricSpecs.
	Metrics map[string]int `json:"metrics,omitempty"`
	// Personas are an agent run's self-reported numbers per persona
	// (tool_calls, total_response_bytes, enjoyment_1_to_10, ...).
	Personas map[string]map[string]any `json:"personas,omitempty"`
}

// onboardingPayload is what a default-profile agent reads before it can
// author a slide, in bytes of each response.
type onboardingPayload struct {
	Instructions     int
	ToolsList        int
	GetStarted       int
	SlideKinds       int
	TemplateNames    int
	TemplatesCompact int
	// MachinePathBytes is how much of GetStarted is this machine's paths
	// (runtime.templates_dir and runtime.output_dir): a temp directory is
	// longer on one host than on another.
	MachinePathBytes int
}

// firstContact is the path get_started lays out: the instructions,
// tools/list, get_started, the template names and the kind catalogue.
func (p onboardingPayload) firstContact() int {
	return p.Instructions + p.ToolsList + p.GetStarted + p.SlideKinds + p.TemplateNames
}

// measureOnboardingPayload measures first contact on the default profile with
// only the templates the binary embeds, in a fresh directory: a local,
// gitignored templates/p-style.pptx must not move a number.
func measureOnboardingPayload(t *testing.T) (*mcpConfig, onboardingPayload) {
	t.Helper()
	withToolProfile(t, toolProfileDeckSpec)
	withRenderStatus(t, true, nil)
	mc := refusalTestConfig(t)
	mc.templatesDir = shippedTemplatesDir(t)

	rawTools, _ := listToolsOverWire(t, newJSON2PPTXMCPServer(profileTestConfig(t), toolProfileDeckSpec))
	p := onboardingPayload{
		Instructions:     len(mcpInstructionsFor(true, nil)),
		ToolsList:        len(rawTools),
		GetStarted:       onboardingBytes(t, mustCall(t, mc.handleGetStarted, map[string]any{"task": "brief"})),
		SlideKinds:       onboardingBytes(t, mustCall(t, mc.handleListSlideKinds, map[string]any{})),
		TemplateNames:    onboardingBytes(t, mustCall(t, mc.handleListTemplates, map[string]any{"fields": "names"})),
		TemplatesCompact: onboardingBytes(t, mustCall(t, mc.handleListTemplates, map[string]any{"read_only": true})),
	}
	var started getStartedResponse
	structuredInto(t, mustCall(t, mc.handleGetStarted, map[string]any{"task": "brief"}).StructuredContent, &started)
	for _, dir := range []string{started.Runtime.TemplatesDir, started.Runtime.OutputDir} {
		quoted, _ := json.Marshal(dir)
		p.MachinePathBytes += len(quoted) - len(`""`)
	}
	return mc, p
}

// journeyMetrics are the deterministic numbers of an agent's journey.
type journeyMetrics struct {
	// Onboarding: bytes read before the first slide is authored.
	Onboarding onboardingPayload
	// The c-repair persona's twelve-flaw draft, repaired from the findings
	// alone on midnight-blue: validates to a clean one, and the size of the
	// first response.
	TwelveFlawValidates     int
	TwelveFlawFirstBytes    int
	TwelveFlawFirstFindings int
	// The a-coldstart persona's finished deck sent as a first draft: validates
	// until ok, and renders until deterministic_ready.
	CleanSpecValidates int
	CleanSpecRenders   int
	// validate_deck_spec against render_deck_spec over the short corpus.
	Parity parityRun
	// Every patch a response offers, applied.
	Patches patchHarnessRun
}

// journeyMetricSpec says how one number is judged.
type journeyMetricSpec struct {
	Name string
	// Get reads the number from a measurement.
	Get func(journeyMetrics) int
	// HigherIsBetter marks a coverage count; every other metric is a cost.
	HigherIsBetter bool
	// Limit is the absolute threshold: a ceiling for a cost, a floor for a
	// coverage count.
	Limit int
	// TolerancePct and ToleranceAbs are how far the number may move the wrong
	// way against the last recorded row before the test fails. Bytes get a
	// percentage (a sentence added to a description is not a regression);
	// counts that follow text measurement get a small absolute slack, since
	// font metrics differ between macOS and the Linux CI; structural counts
	// get none.
	TolerancePct float64
	ToleranceAbs int
}

const journeyByteTolerancePct = 5

// journeyMetricSpecs is the metric set, in the order it is reported. The
// limits are the ones the acceptance tests of each piece hold
// (TestOnboardingPayloadBudgets, TestTwelveFlawDraftCleanInThreeRoundTrips,
// TestTwelveFlawDraftFirstResponse, TestDeckSpecFindingParityCorpus,
// TestEveryEmittedPatchClearsItsFinding).
var journeyMetricSpecs = []journeyMetricSpec{
	{Name: "onboarding_instructions_bytes", Limit: 1536, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.Onboarding.Instructions }},
	{Name: "onboarding_tools_list_bytes", Limit: deckSpecToolListByteBudget, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.Onboarding.ToolsList }},
	{Name: "onboarding_get_started_bytes", Limit: 5632, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.Onboarding.GetStarted - m.Onboarding.MachinePathBytes }},
	{Name: "onboarding_slide_kinds_bytes", Limit: 6 * 1024, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.Onboarding.SlideKinds }},
	{Name: "onboarding_template_names_bytes", Limit: 2 * 1024, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.Onboarding.TemplateNames }},
	{Name: "onboarding_first_contact_bytes", Limit: 40 * 1024, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.Onboarding.firstContact() - m.Onboarding.MachinePathBytes }},

	{Name: "twelve_flaw_validate_round_trips", Limit: 3,
		Get: func(m journeyMetrics) int { return m.TwelveFlawValidates }},
	// The validates and the one render of the spec they left clean.
	{Name: "twelve_flaw_calls_to_ready_render", Limit: 4,
		Get: func(m journeyMetrics) int { return m.TwelveFlawValidates + 1 }},
	{Name: "twelve_flaw_first_response_bytes", Limit: 8192, TolerancePct: journeyByteTolerancePct,
		Get: func(m journeyMetrics) int { return m.TwelveFlawFirstBytes }},
	{Name: "twelve_flaw_first_response_findings", Limit: 16, ToleranceAbs: 2,
		Get: func(m journeyMetrics) int { return m.TwelveFlawFirstFindings }},

	{Name: "clean_spec_validate_round_trips", Limit: 1,
		Get: func(m journeyMetrics) int { return m.CleanSpecValidates }},
	{Name: "clean_spec_calls_to_ready_render", Limit: 2,
		Get: func(m journeyMetrics) int { return m.CleanSpecValidates + m.CleanSpecRenders }},

	{Name: "parity_pairs_compared", HigherIsBetter: true, Limit: 10,
		Get: func(m journeyMetrics) int { return m.Parity.Pairs }},
	{Name: "parity_disagreements", Limit: 0,
		Get: func(m journeyMetrics) int { return len(m.Parity.Problems) }},

	{Name: "patches_offered", HigherIsBetter: true, Limit: 8, ToleranceAbs: 3,
		Get: func(m journeyMetrics) int { return m.Patches.Patches }},
	{Name: "patches_server_verified", HigherIsBetter: true, Limit: 3, ToleranceAbs: 3,
		Get: func(m journeyMetrics) int { return m.Patches.Verified }},
	{Name: "patches_not_clearing", Limit: 0,
		Get: func(m journeyMetrics) int { return len(m.Patches.Problems) }},
}

// coldStartSpec is the deck the a-coldstart persona finished with on
// 2026-10-04: seven slides written from the catalogue's examples.
func coldStartSpec(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "tests", "quality", "journey", "fixtures", "coldstart-board-update.json"))
	if err != nil {
		t.Fatal(err)
	}
	return decodeSpecObject(t, string(raw))
}

// measureJourney takes every measurement. The corpora behind parity and the
// patches are the short run's whatever -short says, so a recorded row means
// the same thing in every run.
func measureJourney(t *testing.T) journeyMetrics {
	t.Helper()
	var m journeyMetrics
	_, m.Onboarding = measureOnboardingPayload(t)

	const template = "midnight-blue"
	m.TwelveFlawValidates = twelveFlawJourney(t, refusalTestConfig(t), template, true, 3)
	first, size := twelveFlawFirstResponse(t, refusalTestConfig(t))
	m.TwelveFlawFirstBytes, m.TwelveFlawFirstFindings = size, len(first.Findings)

	mc := refusalTestConfig(t)
	spec := coldStartSpec(t)
	m.CleanSpecValidates = 1
	if env := deckSpecEnvelope(t, mustCall(t, mc.handleValidateDeckSpec, map[string]any{"spec": spec, "template": template})); !env.OK {
		// One more round would be needed; the limit of 1 reports it.
		m.CleanSpecValidates = 2
		t.Logf("the cold-start deck no longer validates on its first send: %s", env.Summary)
		for _, f := range env.Findings {
			t.Logf("  %s %s %v: %s", f.Severity, f.Code, pathsOf(f), f.Message)
		}
	}
	m.CleanSpecRenders = 1
	if render := renderDeckSpecCall(t, mc, map[string]any{"spec": spec, "template": template}); !render.Success || render.DeterministicReady == nil || !*render.DeterministicReady {
		m.CleanSpecRenders = 2
		t.Logf("the cold-start deck does not render ready on its first render: %q %v", render.Error, render.DeterministicBlockingReasons)
	}

	m.Parity = shortParityRun(t)
	m.Patches = shortPatchHarnessRun(t)
	return m
}

// readJourneyResults reads the results file.
func readJourneyResults(t *testing.T) []journeyResultRow {
	t.Helper()
	f, err := os.Open(journeyResultsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var rows []journeyResultRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<16), 1<<22)
	for line := 1; sc.Scan(); line++ {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		var row journeyResultRow
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&row); err != nil {
			t.Fatalf("%s line %d: %v", journeyResultsPath, line, err)
		}
		rows = append(rows, row)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// journeyRegression says whether got moved the wrong way against the recorded
// value by more than the metric's tolerance.
func journeyRegression(spec journeyMetricSpec, got, recorded int) bool {
	slack := spec.ToleranceAbs
	if pct := int(float64(recorded) * spec.TolerancePct / 100); pct > slack {
		slack = pct
	}
	if spec.HigherIsBetter {
		return got < recorded-slack
	}
	return got > recorded+slack
}

// journeyCommit names the tree a recorded row measured: JOURNEY_COMMIT when
// set, else HEAD, marked +dirty when the tree has other uncommitted changes.
func journeyCommit(t *testing.T) string {
	t.Helper()
	if c := os.Getenv("JOURNEY_COMMIT"); c != "" {
		return c
	}
	head, err := exec.Command("git", "rev-parse", "--short=8", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v (set JOURNEY_COMMIT to record outside a checkout)", err)
	}
	commit := strings.TrimSpace(string(head))
	status, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(status)), "\n") {
		if line != "" && !strings.HasSuffix(line, "tests/quality/journey/results.jsonl") {
			return commit + "+dirty"
		}
	}
	return commit
}

func TestAgentJourneyMetrics(t *testing.T) {
	m := measureJourney(t)
	for _, problem := range m.Parity.Problems {
		t.Errorf("parity: %s", problem)
	}
	for _, problem := range m.Patches.Problems {
		t.Errorf("patch: %s", problem)
	}

	rows := readJourneyResults(t)
	var last *journeyResultRow
	for i := range rows {
		if rows[i].Date == "" || rows[i].Commit == "" || rows[i].Source == "" {
			t.Errorf("results row %d lacks a date, a commit or a source: %+v", i+1, rows[i])
		}
		if _, err := time.Parse("2006-01-02", rows[i].Date); err != nil {
			t.Errorf("results row %d: date %q is not YYYY-MM-DD", i+1, rows[i].Date)
		}
		if i > 0 && rows[i].Date < rows[i-1].Date {
			t.Errorf("results row %d is dated before the row above it; rows are appended in order", i+1)
		}
		if len(rows[i].Metrics) == 0 && len(rows[i].Personas) == 0 {
			t.Errorf("results row %d records neither metrics nor personas", i+1)
		}
		if rows[i].Source == journeyTestSource {
			last = &rows[i]
		}
	}
	recording := os.Getenv(journeyRecordEnv) == "1"
	if last == nil {
		if !recording {
			t.Fatalf("%s has no row written by this test; record one with %s=1", journeyResultsPath, journeyRecordEnv)
		}
		last = &journeyResultRow{}
	}

	current := map[string]int{}
	known := map[string]bool{}
	for _, spec := range journeyMetricSpecs {
		got := spec.Get(m)
		current[spec.Name] = got
		known[spec.Name] = true
		bound, over := "at most", got > spec.Limit
		if spec.HigherIsBetter {
			bound, over = "at least", got < spec.Limit
		}
		recorded, has := last.Metrics[spec.Name]
		was := "not recorded"
		if has {
			was = fmt.Sprintf("%d on %s at %s", recorded, last.Date, last.Commit)
		}
		t.Logf("%-38s %6d  (%s %d; %s)", spec.Name, got, bound, spec.Limit, was)
		if over {
			t.Errorf("%s is %d, want %s %d", spec.Name, got, bound, spec.Limit)
		}
		if has && !recording && journeyRegression(spec, got, recorded) {
			t.Errorf("%s regressed: %d, recorded %d on %s at %s (tolerance %g%% / %d). If the change is intended, record a row: %s=1 go test -short ./cmd/json2pptx -run TestAgentJourneyMetrics",
				spec.Name, got, recorded, last.Date, last.Commit, spec.TolerancePct, spec.ToleranceAbs, journeyRecordEnv)
		}
	}
	// A metric that left the set would stop being compared without anyone
	// deciding so.
	dropped := []string{}
	for name := range last.Metrics {
		if !known[name] {
			dropped = append(dropped, name)
		}
	}
	sort.Strings(dropped)
	if len(dropped) > 0 && !recording {
		t.Errorf("the last recorded row has metrics this test no longer measures: %v; record a row once the change is intended", dropped)
	}

	if !recording {
		return
	}
	if t.Failed() {
		t.Fatal("not recording a row for a run that fails its thresholds")
	}
	row := journeyResultRow{
		Date:              time.Now().UTC().Format("2006-01-02"),
		Commit:            journeyCommit(t),
		SchemaVersion:     SchemaVersion,
		SchemaFingerprint: schemaFingerprint(),
		Source:            journeyTestSource,
		Note:              os.Getenv("JOURNEY_NOTE"),
		Metrics:           current,
	}
	line, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(journeyResultsPath, os.O_APPEND|os.O_WRONLY, 0o644) //nolint:gosec // a committed test data file
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded a row in %s", journeyResultsPath)
}

// TestJourneyRegressionTolerance pins how a number is compared with its
// recorded value.
func TestJourneyRegressionTolerance(t *testing.T) {
	bytesSpec := journeyMetricSpec{TolerancePct: 5}
	count := journeyMetricSpec{}
	fontCount := journeyMetricSpec{ToleranceAbs: 2}
	coverage := journeyMetricSpec{HigherIsBetter: true, ToleranceAbs: 3}
	for _, c := range []struct {
		name          string
		spec          journeyMetricSpec
		got, recorded int
		want          bool
	}{
		{"bytes inside 5%", bytesSpec, 10500, 10000, false},
		{"bytes past 5%", bytesSpec, 10501, 10000, true},
		{"fewer bytes", bytesSpec, 9000, 10000, false},
		{"a round-trip more", count, 4, 3, true},
		{"the same round-trips", count, 3, 3, false},
		{"two findings more", fontCount, 18, 16, false},
		{"three findings more", fontCount, 19, 16, true},
		{"three patches fewer", coverage, 10, 13, false},
		{"four patches fewer", coverage, 9, 13, true},
		{"more patches", coverage, 20, 13, false},
	} {
		if got := journeyRegression(c.spec, c.got, c.recorded); got != c.want {
			t.Errorf("%s: regression=%v, want %v", c.name, got, c.want)
		}
	}
}
