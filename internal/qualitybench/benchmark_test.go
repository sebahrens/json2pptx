package qualitybench

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
	r := Runner{Agent: []string{agent}, OutputDir: dir, Configurations: []string{"redesigned"}, Briefs: briefs, Templates: []Template{{"a", "corporate", false}, {"b", "editorial", false}, {"c", "seven-layout", true}}, Repetitions: 2}
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
