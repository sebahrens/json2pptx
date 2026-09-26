package qualitybench

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerInvokesConfiguredAgentAndCapturesEvidence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	agent := filepath.Join(dir, "agent")
	script := "#!/bin/sh\ncat >/dev/null\nprintf '%s' '{\"model\":\"fake-1\",\"version\":\"test\",\"prompt\":\"captured\",\"tool_calls\":[{\"tool\":\"make_deck\"}],\"artifacts\":[\"deck.pptx\"],\"cost_usd\":0,\"iterations\":1}'\n"
	if err := os.WriteFile(agent, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	briefs := make([]Brief, 12)
	for i := range briefs {
		briefs[i] = Brief{ID: string(rune('a' + i)), Category: "kpi", Prompt: "brief"}
	}
	r := Runner{Agent: []string{agent}, AgentModel: "canonical-model", AgentVersion: "canonical-version", OutputDir: dir, Configurations: []string{"redesigned"}, Briefs: briefs, Templates: []Template{{Name: "a", Family: "corporate"}, {Name: "b", Family: "editorial"}, {Name: "c", Family: "seven-layout", HeldOut: true}}, Repetitions: 2}
	report, err := r.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Evidence) != 72 {
		t.Fatalf("runs=%d", len(report.Evidence))
	}
	if report.Evidence[0].Error != "" || len(report.Evidence[0].Result.ToolCalls) != 1 {
		t.Fatalf("bad evidence: %+v", report.Evidence[0])
	}
	for i, ev := range report.Evidence {
		if ev.Result.Model != "canonical-model" || ev.Result.Version != "canonical-version" {
			t.Fatalf("evidence[%d] identity = %q / %q", i, ev.Result.Model, ev.Result.Version)
		}
	}
}

func TestRunRequestsBoundsConcurrencyAndPreservesOrder(t *testing.T) {
	requests := make([]Request, 12)
	for i := range requests {
		requests[i].RunID = string(rune('a' + i))
	}
	var active, maximum atomic.Int32
	evidence := runRequests(context.Background(), requests, 3, func(_ context.Context, req Request) Evidence {
		current := active.Add(1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		active.Add(-1)
		return Evidence{Request: req}
	})
	if got := maximum.Load(); got < 2 || got > 3 {
		t.Fatalf("maximum concurrency = %d, want 2..3", got)
	}
	for i, ev := range evidence {
		if ev.Request.RunID != requests[i].RunID {
			t.Fatalf("evidence[%d] run = %q, want %q", i, ev.Request.RunID, requests[i].RunID)
		}
	}
}

func TestRunnerRefusesUnconfiguredAgent(t *testing.T) {
	_, err := (Runner{}).Run(context.Background())
	if err == nil {
		t.Fatal("expected explicit agent requirement")
	}
}

func TestApplyRatingsKeepsReleaseBarHonest(t *testing.T) {
	report := &Report{Evidence: []Evidence{{Request: Request{RunID: "r"}}}}
	s := ApplyRatings(report, []Rating{{RunID: "r", Reviewer: "a", Usability: 5}, {RunID: "r", Reviewer: "b", Usability: 5}})
	if s.ReleaseDecision == "pass" {
		t.Fatal("must not pass without paired improvement evidence")
	}
}

func TestApplyRatingsAcceptsOneBlindReviewerAndDeduplicates(t *testing.T) {
	report := &Report{Evidence: []Evidence{{Request: Request{RunID: "r", Configuration: "redesigned"}}}}
	ratings := []Rating{
		{RunID: "r", Reviewer: "alice", ReviewerType: "human", Usability: 5},
		{RunID: "r", Reviewer: "alice", ReviewerType: "human", Usability: 5},
		{RunID: "r", Reviewer: "vision-rater", ReviewerType: "llm", Usability: 5},
	}
	if got := ApplyRatings(report, ratings).RatedPairs; got != 1 {
		t.Fatalf("rated decks = %d, want 1 with one human reviewer", got)
	}
	if got := ApplyRatings(report, ratings[2:]); got.RatedPairs != 1 || len(got.ReviewerTypes) != 1 || got.ReviewerTypes[0] != "llm" {
		t.Fatalf("blind AI rating should be eligible and labelled: %+v", got)
	}
	if got := ApplyRatings(report, []Rating{{RunID: "r", Reviewer: "pixels", ReviewerType: "heuristic", Usability: 5}}).RatedPairs; got != 0 {
		t.Fatal("heuristic score must not count as blind review")
	}
}

func TestSingleHumanCanReachReleaseDecision(t *testing.T) {
	report := &Report{Evidence: []Evidence{{Request: Request{RunID: "b", Configuration: "baseline"}}, {Request: Request{RunID: "n", Configuration: "redesigned"}}}}
	ratings := []Rating{{RunID: "b", Reviewer: "human", ReviewerType: "human", Usability: 4}, {RunID: "n", Reviewer: "human", ReviewerType: "human", Usability: 5}}
	if got := ApplyRatings(report, ratings); got.ReleaseDecision != "pass" || got.RatedPairs != 2 || got.Disagreements != 0 {
		t.Fatalf("one human should satisfy the gate: %+v", got)
	}
	ratings[1].CriticalTemplateDefect = true
	if got := ApplyRatings(report, ratings); got.ReleaseDecision == "pass" {
		t.Fatal("one reviewer must still enforce the defect gate")
	}
	if got := ApplyRatings(report, ratings[:1]); got.ReleaseDecision == "pass" {
		t.Fatal("partial human review must not pass")
	}
}

func TestSingleAIReviewIsLabelledAndFailedRunsBlockApproval(t *testing.T) {
	report := &Report{Evidence: []Evidence{{Request: Request{RunID: "b", Configuration: "baseline"}}, {Request: Request{RunID: "n", Configuration: "redesigned"}}}}
	ratings := []Rating{{RunID: "b", Reviewer: "blind-ai", ReviewerType: "llm", Usability: 4}, {RunID: "n", Reviewer: "blind-ai", ReviewerType: "llm", Usability: 5}}
	s := ApplyRatings(report, ratings)
	if s.ReleaseDecision != "pass" || len(s.ReviewerTypes) != 1 || s.ReviewerTypes[0] != "llm" || len(report.Ratings) != 2 {
		t.Fatalf("AI review provenance missing: %+v", report)
	}
	report.Evidence[1].Error = "generation failed"
	if s := ApplyRatings(report, ratings); s.ReleaseDecision == "pass" || s.FailedRuns != 1 {
		t.Fatalf("failed artifact passed release gate: %+v", s)
	}
}
