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

// defaultTemplates are the benchmark's three template families. They must
// exist in -templates-dir; business-template is held out from day-to-day
// development and pattern tuning (the four CLAUDE.md templates are not).
const defaultTemplates = "midnight-blue:corporate,warm-coral:editorial,business-template:held-out-seven-layout:heldout"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "qualitybench:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		agent, out, resultsDir, templatesDir, templatesSpec, briefsPath, bin, reportPath, ratingsPath string
		dryRun                                                                                        bool
		reps                                                                                          int
	)
	flag.BoolVar(&dryRun, "dry-run", false, "render deterministic reference decks (no agent, no provider call)")
	flag.StringVar(&agent, "agent", "", "agent executable plus optional space-separated arguments")
	flag.StringVar(&out, "out", "output/quality-benchmark", "evidence directory (decks, renders, blind contact sheets)")
	flag.StringVar(&resultsDir, "results-dir", "tests/quality/results", "directory for the results JSON")
	flag.StringVar(&templatesDir, "templates-dir", "templates", "templates directory")
	flag.StringVar(&templatesSpec, "templates", defaultTemplates, "comma list of name:family[:heldout]")
	flag.StringVar(&briefsPath, "briefs", "tests/quality/agent_briefs.json", "briefs JSON")
	flag.StringVar(&bin, "json2pptx", "", "json2pptx binary (default: build ./cmd/json2pptx into a temp dir)")
	flag.IntVar(&reps, "repetitions", 2, "agent-mode repetitions per configuration")
	flag.StringVar(&reportPath, "report", "", "results JSON to apply -ratings to")
	flag.StringVar(&ratingsPath, "ratings", "", "filled ratings CSV (blind_id,reviewer,...) to apply to -report")
	flag.Parse()

	if ratingsPath != "" {
		return applyRatings(reportPath, ratingsPath)
	}
	if !dryRun && agent == "" {
		return fmt.Errorf("pass -dry-run or -agent; no provider calls occur by default")
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

	r := qualitybench.Runner{Agent: strings.Fields(agent), OutputDir: out, Briefs: briefs, Templates: templates, Repetitions: reps}
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

// parseTemplates parses name:family[:heldout] entries and fails fast when a
// template file is missing, so the benchmark never silently references
// templates that do not exist.
func parseTemplates(spec, dir string) ([]qualitybench.Template, error) {
	var out []qualitybench.Template
	for _, entry := range strings.Split(spec, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ":")
		if len(parts) < 2 || parts[0] == "" {
			return nil, fmt.Errorf("template entry %q must be name:family[:heldout]", entry)
		}
		if _, err := os.Stat(filepath.Join(dir, parts[0]+".pptx")); err != nil {
			return nil, fmt.Errorf("benchmark template %q not found in %s", parts[0], dir)
		}
		out = append(out, qualitybench.Template{Name: parts[0], Family: parts[1], HeldOut: len(parts) > 2 && parts[2] == "heldout"})
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
	f, err := os.Open(ratingsPath) // #nosec G304 -- operator-supplied path
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return err
	}
	var ratings []qualitybench.Rating
	for i, row := range rows {
		if i == 0 || len(row) < 9 || row[1] == "" {
			continue
		}
		run, ok := runByBlind[row[0]]
		if !ok {
			return fmt.Errorf("ratings row %d: unknown blind_id %q", i+1, row[0])
		}
		n := func(s string) int { v, _ := strconv.Atoi(strings.TrimSpace(s)); return v }
		b := func(s string) bool { v, _ := strconv.ParseBool(strings.TrimSpace(s)); return v }
		ratings = append(ratings, qualitybench.Rating{RunID: run, Reviewer: row[1], Readability: n(row[2]), Hierarchy: n(row[3]),
			TemplateFidelity: n(row[4]), FactualCompleteness: n(row[5]), Usability: n(row[6]), LostCriticalFact: b(row[7]), CriticalTemplateDefect: b(row[8])})
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

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644) // #nosec G306 -- shareable results file
}
