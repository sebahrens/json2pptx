package main

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPrepareAgentReviewPacketsCreatesThreeByTwoMatrix(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "ratings.csv")
	header := "blind_id,reviewer,reviewer_type,readability,hierarchy,template_fidelity,factual_completeness,usability,lost_critical_fact,critical_template_defect\na,,,,,,,,,\nb,,,,,,,,,\n"
	if err := os.WriteFile(template, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "reviews")
	if err := prepareAgentReviewPackets(template, "alpha,beta,gamma", 2, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest agentReviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Agents) != 3 {
		t.Fatalf("agents = %d", len(manifest.Agents))
	}
	for _, agent := range manifest.Agents {
		if len(agent.Runs) != 2 {
			t.Fatalf("runs for %s = %d", agent.AgentID, len(agent.Runs))
		}
		for _, run := range agent.Runs {
			f, err := os.Open(filepath.Join(out, run.Ratings))
			if err != nil {
				t.Fatal(err)
			}
			rows, err := csv.NewReader(f).ReadAll()
			_ = f.Close()
			if err != nil || len(rows) != 3 || rows[1][1] != run.Reviewer || rows[1][2] != "llm" || rows[1][3] != "" {
				t.Fatalf("bad ballot %s: rows=%v err=%v", run.Ratings, rows, err)
			}
		}
	}
}

func TestPrepareAgentReviewPacketsRejectsWrongShape(t *testing.T) {
	if err := prepareAgentReviewPackets("missing.csv", "alpha,beta", 2, t.TempDir()); err == nil {
		t.Fatal("two agents must fail before reading template")
	}
}

func TestCompareAgentReviewPacketsWritesAuditableConsensus(t *testing.T) {
	dir := t.TempDir()
	template := filepath.Join(dir, "ratings.csv")
	header := "blind_id,reviewer,reviewer_type,readability,hierarchy,template_fidelity,factual_completeness,usability,lost_critical_fact,critical_template_defect\na,,,,,,,,,\n"
	if err := os.WriteFile(template, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}
	reviews := filepath.Join(dir, "reviews")
	if err := prepareAgentReviewPackets(template, "alpha,beta,gamma", 2, reviews); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(reviews, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest agentReviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for agentIndex, agent := range manifest.Agents {
		for _, run := range agent.Runs {
			path := filepath.Join(reviews, run.Ratings)
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			w := csv.NewWriter(f)
			_ = w.Write([]string{"blind_id", "reviewer", "reviewer_type", "readability", "hierarchy", "template_fidelity", "factual_completeness", "usability", "lost_critical_fact", "critical_template_defect"})
			score := strconv.Itoa(2 + agentIndex + run.Run - 1)
			lost := strconv.FormatBool(agentIndex == 0 || (agentIndex == 1 && run.Run == 1)) // 3-3 tie.
			defect := strconv.FormatBool(agentIndex < 2)                                     // 4-2 true.
			_ = w.Write([]string{"a", run.Reviewer, "llm", score, score, score, score, score, lost, defect})
			w.Flush()
			if err := w.Error(); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	out := filepath.Join(dir, "comparison")
	if err := compareAgentReviewPackets(filepath.Join(reviews, "manifest.json"), out); err != nil {
		t.Fatal(err)
	}
	resultData, err := os.ReadFile(filepath.Join(out, "agent-consensus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Items []struct {
			Consensus struct {
				Usability float64 `json:"usability"`
			} `json:"consensus"`
			LostCriticalFact       *bool `json:"lost_critical_fact"`
			CriticalTemplateDefect *bool `json:"critical_template_defect"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resultData, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Consensus.Usability != 3.5 || result.Items[0].LostCriticalFact != nil || result.Items[0].CriticalTemplateDefect == nil || !*result.Items[0].CriticalTemplateDefect {
		t.Fatalf("unexpected comparison: %+v", result)
	}
}
