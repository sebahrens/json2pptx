// Command qualitybench runs the end-to-end deck-quality benchmark.
//
//	go run ./cmd/qualitybench -dry-run
//	    Agent-free harness run: renders a deterministic reference deck for every
//	    brief on every benchmark template, writes blind contact sheets +
//	    ratings template under -out and a results JSON under -results-dir.
//	go run ./cmd/qualitybench -agent "./my-agent --flag"
//	    Real benchmark: invokes the configured agent per brief/template/
//	    repetition/configuration, then renders its .pptx artifacts to blind
//	    contact sheets. No provider call happens unless -agent is given.
//	go run ./cmd/qualitybench -report results.json -ratings ratings.csv
//	    Applies two-reviewer blind ratings and prints the release decision.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

// defaultTemplates includes two bundled templates and one purpose-built
// portability fixture that lives outside -templates-dir. Explicit paths are
// resolved from the working directory, then relative to -templates-dir.
const defaultTemplates = "midnight-blue:corporate,warm-coral:editorial,portability-side-logo=../tests/quality/fixtures/portability/templates/portability-side-logo.pptx:portability:heldout"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "qualitybench:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		agent, agentModel, agentVersion, out, resultsDir, templatesDir, templatesSpec, briefsPath, bin, reportPath, ratingsPath string
		prepareAgentRatings, compareAgentRatings, agentReviewers, agentReviewDir, comparisonOut                                 string
		dryRun                                                                                                                  bool
		reps, parallel, agentReviewRuns                                                                                         int
	)
	flag.BoolVar(&dryRun, "dry-run", false, "render deterministic reference decks (no agent, no provider call)")
	flag.StringVar(&agent, "agent", "", "agent executable plus optional space-separated arguments")
	flag.StringVar(&agentModel, "agent-model", "", "canonical model identity recorded for every agent run")
	flag.StringVar(&agentVersion, "agent-version", "", "canonical agent/CLI version recorded for every agent run")
	flag.StringVar(&out, "out", "output/quality-benchmark", "evidence directory (decks, renders, blind contact sheets)")
	flag.StringVar(&resultsDir, "results-dir", "tests/quality/results", "directory for the results JSON")
	flag.StringVar(&templatesDir, "templates-dir", "templates", "templates directory")
	flag.StringVar(&templatesSpec, "templates", defaultTemplates, "comma list of name:family[:heldout] or name=path:family[:heldout]")
	flag.StringVar(&briefsPath, "briefs", "tests/quality/agent_briefs.json", "briefs JSON")
	flag.StringVar(&bin, "json2pptx", "", "json2pptx binary (default: build ./cmd/json2pptx into a temp dir)")
	flag.IntVar(&reps, "repetitions", 2, "agent-mode repetitions per configuration")
	flag.IntVar(&parallel, "parallel", 1, "maximum concurrent agent runs (agent mode only)")
	flag.StringVar(&reportPath, "report", "", "results JSON to apply -ratings to")
	flag.StringVar(&ratingsPath, "ratings", "", "comma-separated filled reviewer CSVs to apply to -report")
	flag.StringVar(&prepareAgentRatings, "prepare-agent-ratings", "", "blank ratings template used to prepare a blinded multi-agent review")
	flag.StringVar(&compareAgentRatings, "compare-agent-ratings", "", "multi-agent review manifest whose completed ballots should be compared")
	flag.StringVar(&agentReviewers, "agent-reviewers", "agent-a,agent-b,agent-c", "three comma-separated stable agent reviewer IDs")
	flag.IntVar(&agentReviewRuns, "agent-review-runs", 2, "independent rating runs per agent (must be 2)")
	flag.StringVar(&agentReviewDir, "agent-review-dir", "output/quality-benchmark/agent-reviews", "directory for blank agent rating packets")
	flag.StringVar(&comparisonOut, "comparison-out", "", "comparison output directory (default: manifest directory/comparison)")
	flag.Parse()

	if prepareAgentRatings != "" {
		return prepareAgentReviewPackets(prepareAgentRatings, agentReviewers, agentReviewRuns, agentReviewDir)
	}
	if compareAgentRatings != "" {
		return compareAgentReviewPackets(compareAgentRatings, comparisonOut)
	}
	if ratingsPath != "" {
		return applyRatings(reportPath, ratingsPath)
	}
	if !dryRun && agent == "" {
		return fmt.Errorf("pass -dry-run or -agent; no provider calls occur by default")
	}
	if parallel < 1 {
		return fmt.Errorf("-parallel must be at least 1")
	}
	briefs, err := loadBriefs(briefsPath)
	if err != nil {
		return err
	}
	templates, err := parseTemplates(templatesSpec, templatesDir)
	if err != nil {
		return err
	}
	if bin == "" {
		if bin, err = buildJSON2PPTX(); err != nil {
			return err
		}
	}
	renderer := &qualitybench.Renderer{JSON2PPTX: bin, TemplatesDir: templatesDir}
	ctx := context.Background()

	if dryRun {
		report, path, err := qualitybench.DryRun{Renderer: renderer, Briefs: briefs, Templates: templates, OutputDir: out, ResultsDir: resultsDir}.Run(ctx)
		if err != nil {
			return err
		}
		return summarize(report, path, out)
	}

	r := qualitybench.Runner{Agent: strings.Fields(agent), AgentModel: agentModel, AgentVersion: agentVersion, OutputDir: out, Briefs: briefs, Templates: templates, Repetitions: reps, Parallelism: parallel}
	report, err := r.Run(ctx)
	if err != nil {
		return err
	}
	if err := renderer.Resolve(); err != nil {
		return err
	}
	if err := qualitybench.AttachBlindContactSheets(ctx, report, renderer, out); err != nil {
		return err
	}
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(resultsDir, "agent-"+report.GeneratedAt.Format("20060102T150405Z")+".json")
	if err := writeJSON(path, report); err != nil {
		return err
	}
	return summarize(report, path, out)
}

func summarize(report *qualitybench.Report, path, out string) error {
	sheets, failed := 0, 0
	briefsWithSheet := map[string]bool{}
	for _, ev := range report.Evidence {
		if ev.ContactSheet != "" {
			sheets++
			briefsWithSheet[ev.Request.Brief.ID] = true
		}
		if ev.Error != "" {
			failed++
			fmt.Fprintf(os.Stderr, "run %s failed: %s\n", ev.Request.RunID, ev.Error)
		}
	}
	fmt.Printf("runs: %d, contact sheets: %d (covering %d briefs), failed: %d\n", len(report.Evidence), sheets, len(briefsWithSheet), failed)
	fmt.Printf("results: %s\nreviewer bundle: %s (give reviewers only sheets/ and ratings_template.csv; keep blind_key.json)\n",
		path, filepath.Join(out, "sheets"))
	fmt.Printf("release decision: %s\n", report.Summary.ReleaseDecision)
	if failed > 0 {
		return fmt.Errorf("%d run(s) failed", failed)
	}
	return nil
}

func loadBriefs(path string) ([]qualitybench.Brief, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied path
	if err != nil {
		return nil, err
	}
	var briefs []qualitybench.Brief
	if err := json.Unmarshal(data, &briefs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return briefs, nil
}

// parseTemplates parses name:family[:heldout] and name=path:family[:heldout]
// entries and fails fast when a template file is missing. Relative explicit
// paths are tried from the working directory and then from -templates-dir.
func parseTemplates(spec, dir string) ([]qualitybench.Template, error) {
	var out []qualitybench.Template
	for _, raw := range strings.Split(spec, ",") {
		entry := strings.TrimSpace(raw)
		heldOut := strings.HasSuffix(entry, ":heldout")
		if heldOut {
			entry = strings.TrimSuffix(entry, ":heldout")
		}
		sep := strings.LastIndex(entry, ":")
		if sep <= 0 || sep == len(entry)-1 {
			return nil, fmt.Errorf("template entry %q must be name:family[:heldout] or name=path:family[:heldout]", raw)
		}
		locator, family := entry[:sep], entry[sep+1:]
		name, explicitPath, hasPath := strings.Cut(locator, "=")
		if name == "" || family == "" || (hasPath && explicitPath == "") {
			return nil, fmt.Errorf("invalid template entry %q", raw)
		}
		tmpl := qualitybench.Template{Name: name, Family: family, HeldOut: heldOut}
		path := filepath.Join(dir, name+".pptx")
		if hasPath {
			path = explicitPath
			if _, err := os.Stat(path); err != nil && !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			tmpl.Path, _ = filepath.Abs(path)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("benchmark template %q not found at %s", name, path)
		}
		out = append(out, tmpl)
	}
	return out, nil
}

func buildJSON2PPTX() (string, error) {
	dir, err := os.MkdirTemp("", "qualitybench-bin-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "json2pptx")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/json2pptx") // #nosec G204 -- fixed go toolchain command; bin is a fresh temp path
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build json2pptx (run from the repo root or pass -json2pptx): %w", err)
	}
	return bin, nil
}

// applyRatings maps blind-ID ratings back to runs and applies them.
func applyRatings(reportPath, ratingsPath string) error {
	if reportPath == "" {
		return fmt.Errorf("-ratings requires -report")
	}
	data, err := os.ReadFile(reportPath) // #nosec G304 -- operator-supplied path
	if err != nil {
		return err
	}
	var report qualitybench.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	runByBlind := map[string]string{}
	for _, ev := range report.Evidence {
		if ev.BlindID != "" {
			runByBlind[ev.BlindID] = ev.Request.RunID
		}
	}
	var ratings []qualitybench.Rating
	for _, path := range strings.Split(ratingsPath, ",") {
		parsed, err := readRatingsCSV(strings.TrimSpace(path), runByBlind)
		if err != nil {
			return err
		}
		ratings = append(ratings, parsed...)
	}
	s := qualitybench.ApplyRatings(&report, ratings)
	report.GeneratedAt = time.Now().UTC()
	if err := writeJSON(reportPath, &report); err != nil {
		return err
	}
	fmt.Printf("rated pairs: %d, usable: %.0f%% (Wilson %.2f–%.2f), disagreements: %d\nrelease decision: %s\n",
		s.RatedPairs, 100*s.UsableRate, s.WilsonLow, s.WilsonHigh, s.Disagreements, s.ReleaseDecision)
	return nil
}

func readRatingsCSV(path string, runByBlind map[string]string) ([]qualitybench.Rating, error) {
	f, err := os.Open(path) // #nosec G304 -- operator-supplied path
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ratings file %s is empty", path)
	}
	parser := ratingCSV{path: path, cols: map[string]int{}}
	for i, name := range rows[0] {
		parser.cols[strings.TrimSpace(name)] = i
	}
	required := []string{"blind_id", "reviewer", "readability", "hierarchy", "template_fidelity", "factual_completeness", "usability", "lost_critical_fact", "critical_template_defect"}
	for _, name := range required {
		if _, ok := parser.cols[name]; !ok {
			return nil, fmt.Errorf("ratings file %s missing column %q", path, name)
		}
	}
	var ratings []qualitybench.Rating
	for i, row := range rows[1:] {
		blindID, reviewer := parser.value(row, "blind_id"), parser.value(row, "reviewer")
		if reviewer == "" {
			continue
		}
		run, ok := runByBlind[blindID]
		if !ok {
			return nil, fmt.Errorf("ratings %s row %d: unknown blind_id %q", path, i+2, blindID)
		}
		rating, err := parser.parse(row, i+2, run, reviewer)
		if err != nil {
			return nil, err
		}
		ratings = append(ratings, rating)
	}
	return ratings, nil
}

type ratingCSV struct {
	path string
	cols map[string]int
}

func (p ratingCSV) value(row []string, name string) string {
	i, ok := p.cols[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func (p ratingCSV) score(row []string, line int, name string) (int, error) {
	v, err := strconv.Atoi(p.value(row, name))
	if err != nil || v < 1 || v > 5 {
		return 0, fmt.Errorf("ratings %s row %d: %s must be an integer from 1 to 5", p.path, line, name)
	}
	return v, nil
}

func (p ratingCSV) boolean(row []string, line int, name string) (bool, error) {
	v, err := strconv.ParseBool(p.value(row, name))
	if err != nil {
		return false, fmt.Errorf("ratings %s row %d: %s must be true or false", p.path, line, name)
	}
	return v, nil
}

func (p ratingCSV) parse(row []string, line int, run, reviewer string) (qualitybench.Rating, error) {
	values := make([]int, 5)
	for i, name := range []string{"readability", "hierarchy", "template_fidelity", "factual_completeness", "usability"} {
		v, err := p.score(row, line, name)
		if err != nil {
			return qualitybench.Rating{}, err
		}
		values[i] = v
	}
	lost, err := p.boolean(row, line, "lost_critical_fact")
	if err != nil {
		return qualitybench.Rating{}, err
	}
	defect, err := p.boolean(row, line, "critical_template_defect")
	if err != nil {
		return qualitybench.Rating{}, err
	}
	reviewerType := strings.ToLower(p.value(row, "reviewer_type"))
	if reviewerType == "" {
		reviewerType = "human"
	}
	if reviewerType != "human" && reviewerType != "llm" {
		return qualitybench.Rating{}, fmt.Errorf("ratings %s row %d: reviewer_type must be human or llm", p.path, line)
	}
	return qualitybench.Rating{RunID: run, Reviewer: reviewer, ReviewerType: reviewerType,
		Readability: values[0], Hierarchy: values[1], TemplateFidelity: values[2],
		FactualCompleteness: values[3], Usability: values[4], LostCriticalFact: lost, CriticalTemplateDefect: defect}, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644) // #nosec G306 -- shareable results file
}
