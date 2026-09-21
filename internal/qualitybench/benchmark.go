package qualitybench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Brief struct{ ID, Category, Prompt string }
type Template struct {
	Name, Family string
	Path         string `json:"Path,omitempty"`
	HeldOut      bool
}
type Request struct {
	RunID         string   `json:"run_id"`
	Configuration string   `json:"configuration"`
	Brief         Brief    `json:"brief"`
	Template      Template `json:"template"`
	Repetition    int      `json:"repetition"`
}
type AgentResult struct {
	Model      string            `json:"model"`
	Version    string            `json:"version"`
	Prompt     string            `json:"prompt"`
	ToolCalls  []json.RawMessage `json:"tool_calls"`
	Artifacts  []string          `json:"artifacts"`
	CostUSD    float64           `json:"cost_usd"`
	Iterations int               `json:"iterations"`
}
type Evidence struct {
	Request    Request     `json:"request"`
	Result     AgentResult `json:"result"`
	StartedAt  time.Time   `json:"started_at"`
	DurationMS int64       `json:"duration_ms"`
	Error      string      `json:"error,omitempty"`
	// BlindID / ContactSheet / SlideCount are filled by
	// AttachBlindContactSheets for runs that produced a .pptx artifact.
	BlindID      string `json:"blind_id,omitempty"`
	ContactSheet string `json:"contact_sheet,omitempty"`
	SlideCount   int    `json:"slide_count,omitempty"`
}
type Rating struct {
	RunID, Reviewer, ReviewerType                                            string
	Readability, Hierarchy, TemplateFidelity, FactualCompleteness, Usability int
	LostCriticalFact, CriticalTemplateDefect                                 bool
}
type Summary struct {
	Runs, RatedPairs, Usable                                  int
	UsableRate, WilsonLow, WilsonHigh                         float64
	LostCriticalFacts, CriticalTemplateDefects, Disagreements int
	PairedImprovement                                         bool
	ReleaseDecision                                           string
}
type Report struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Evidence    []Evidence `json:"evidence"`
	Summary     Summary    `json:"summary"`
}

type Runner struct {
	Agent          []string
	AgentModel     string
	AgentVersion   string
	OutputDir      string
	Configurations []string
	Briefs         []Brief
	Templates      []Template
	Repetitions    int
	Parallelism    int
}

func (r Runner) Run(ctx context.Context) (*Report, error) {
	if len(r.Agent) == 0 || r.Agent[0] == "" {
		return nil, fmt.Errorf("agent executable is required; benchmark never starts a paid/provider run by default")
	}
	if len(r.Briefs) < 12 {
		return nil, fmt.Errorf("at least 12 briefs are required")
	}
	if len(r.Templates) < 3 {
		return nil, fmt.Errorf("at least 3 template families are required")
	}
	if r.Repetitions < 2 {
		return nil, fmt.Errorf("at least 2 repetitions are required")
	}
	if len(r.Configurations) == 0 {
		r.Configurations = []string{"baseline", "redesigned"}
	}
	if r.Parallelism <= 0 {
		r.Parallelism = 1
	}
	if err := os.MkdirAll(r.OutputDir, 0o755); err != nil {
		return nil, err
	}
	requests := make([]Request, 0, len(r.Configurations)*len(r.Briefs)*len(r.Templates)*r.Repetitions)
	for _, config := range r.Configurations {
		for _, brief := range r.Briefs {
			for _, tmpl := range r.Templates {
				for rep := 1; rep <= r.Repetitions; rep++ {
					requests = append(requests, Request{RunID: fmt.Sprintf("%s-%s-%s-%d", config, brief.ID, tmpl.Name, rep), Configuration: config, Brief: brief, Template: tmpl, Repetition: rep})
				}
			}
		}
	}
	evidence := runRequests(ctx, requests, r.Parallelism, func(ctx context.Context, req Request) Evidence {
		ev := r.invoke(ctx, req)
		if r.AgentModel != "" {
			ev.Result.Model = r.AgentModel
		}
		if r.AgentVersion != "" {
			ev.Result.Version = r.AgentVersion
		}
		data, _ := json.MarshalIndent(ev, "", "  ")
		_ = os.WriteFile(filepath.Join(r.OutputDir, req.RunID+".json"), append(data, '\n'), 0o644)
		return ev
	})
	report := &Report{GeneratedAt: time.Now().UTC(), Evidence: evidence}
	report.Summary.Runs = len(report.Evidence)
	report.Summary.ReleaseDecision = "inconclusive: blind ratings from two reviewers are required"
	return report, nil
}

func runRequests(ctx context.Context, requests []Request, parallelism int, invoke func(context.Context, Request) Evidence) []Evidence {
	if parallelism < 1 {
		parallelism = 1
	}
	if parallelism > len(requests) {
		parallelism = len(requests)
	}
	if len(requests) == 0 {
		return []Evidence{}
	}
	type job struct {
		index int
		req   Request
	}
	jobs := make(chan job)
	evidence := make([]Evidence, len(requests))
	var workers sync.WaitGroup
	workers.Add(parallelism)
	for range parallelism {
		go func() {
			defer workers.Done()
			for next := range jobs {
				evidence[next.index] = invoke(ctx, next.req)
			}
		}()
	}
	for i, req := range requests {
		jobs <- job{index: i, req: req}
	}
	close(jobs)
	workers.Wait()
	return evidence
}

func (r Runner) invoke(ctx context.Context, req Request) Evidence {
	started := time.Now().UTC()
	payload, _ := json.Marshal(req)
	// #nosec G204 -- executing the explicitly configured agent is the purpose of
	// this opt-in benchmark; default CI supplies no executable and cannot run it.
	cmd := exec.CommandContext(ctx, r.Agent[0], r.Agent[1:]...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	ev := Evidence{Request: req, StartedAt: started, DurationMS: time.Since(started).Milliseconds()}
	if err != nil {
		ev.Error = fmt.Sprintf("%v: %s", err, stderr.String())
		return ev
	}
	if err := json.Unmarshal(stdout.Bytes(), &ev.Result); err != nil {
		ev.Error = "invalid agent evidence: " + err.Error()
		return ev
	}
	if ev.Result.Model == "" || len(ev.Result.ToolCalls) == 0 || len(ev.Result.Artifacts) == 0 {
		ev.Error = "agent evidence must include model, tool_calls, and artifacts"
	}
	return ev
}

func ApplyRatings(report *Report, ratings []Rating) Summary {
	s := Summary{Runs: len(report.Evidence)}
	byRun := map[string][]Rating{}
	for _, r := range ratings {
		if r.Reviewer == "" || (r.ReviewerType != "" && !strings.EqualFold(r.ReviewerType, "human")) {
			continue
		}
		byRun[r.RunID] = append(byRun[r.RunID], r)
	}
	configByRun := map[string]string{}
	for _, ev := range report.Evidence {
		configByRun[ev.Request.RunID] = ev.Request.Configuration
	}
	var baseTotal, newTotal, baseN, newN int
	for _, candidates := range byRun {
		pair := distinctReviewerPair(candidates)
		if len(pair) < 2 {
			continue
		}
		s.RatedPairs++
		usableVotes := 0
		for _, r := range pair[:2] {
			if r.Usability >= 4 {
				usableVotes++
			}
			if r.LostCriticalFact {
				s.LostCriticalFacts++
			}
			if r.CriticalTemplateDefect {
				s.CriticalTemplateDefects++
			}
		}
		if usableVotes == 2 {
			s.Usable++
		}
		if abs(pair[0].Usability-pair[1].Usability) >= 2 {
			s.Disagreements++
		}
		avg := pair[0].Usability + pair[1].Usability
		if configByRun[pair[0].RunID] == "redesigned" {
			newTotal += avg
			newN += 2
		} else if configByRun[pair[0].RunID] == "baseline" {
			baseTotal += avg
			baseN += 2
		}
	}
	if baseN > 0 && newN > 0 {
		s.PairedImprovement = float64(newTotal)/float64(newN) > float64(baseTotal)/float64(baseN)
	}
	if s.RatedPairs > 0 {
		s.UsableRate = float64(s.Usable) / float64(s.RatedPairs)
		s.WilsonLow, s.WilsonHigh = wilson(s.Usable, s.RatedPairs)
	}
	s.ReleaseDecision = "hold: target not achieved or evidence incomplete"
	if s.RatedPairs == len(report.Evidence) && s.UsableRate >= .80 && s.LostCriticalFacts == 0 && s.CriticalTemplateDefects == 0 && s.PairedImprovement {
		s.ReleaseDecision = "pass"
	}
	report.Summary = s
	return s
}

func distinctReviewerPair(candidates []Rating) []Rating {
	seen := map[string]bool{}
	pair := make([]Rating, 0, 2)
	for _, rating := range candidates {
		key := strings.ToLower(strings.TrimSpace(rating.Reviewer))
		if seen[key] {
			continue
		}
		seen[key] = true
		pair = append(pair, rating)
		if len(pair) == 2 {
			break
		}
	}
	return pair
}
func wilson(k, n int) (float64, float64) {
	if n == 0 {
		return 0, 0
	}
	z := 1.96
	p := float64(k) / float64(n)
	d := 1 + z*z/float64(n)
	c := (p + z*z/(2*float64(n))) / d
	h := z * math.Sqrt((p*(1-p)+z*z/(4*float64(n)))/float64(n)) / d
	return c - h, c + h
}
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
