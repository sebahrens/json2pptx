package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

type agentReviewManifest struct {
	SchemaVersion int                        `json:"schema_version"`
	Template      string                     `json:"template"`
	ReviewerType  string                     `json:"reviewer_type"`
	Agents        []agentReviewManifestAgent `json:"agents"`
}

type agentReviewManifestAgent struct {
	AgentID string                   `json:"agent_id"`
	Runs    []agentReviewManifestRun `json:"runs"`
}

type agentReviewManifestRun struct {
	Run      int    `json:"run"`
	Reviewer string `json:"reviewer"`
	Ratings  string `json:"ratings"`
}

func prepareAgentReviewPackets(templatePath, reviewerSpec string, runs int, outDir string) error {
	if runs != 2 {
		return fmt.Errorf("-agent-review-runs must be 2")
	}
	agents := splitNonempty(reviewerSpec)
	if len(agents) != 3 || !uniqueStrings(agents) {
		return fmt.Errorf("-agent-reviewers must contain exactly 3 distinct IDs")
	}
	fileParts := make([]string, 0, len(agents))
	for _, agent := range agents {
		part := safeFilePart(agent)
		if part == "" {
			return fmt.Errorf("agent reviewer ID %q cannot be used in a filename", agent)
		}
		fileParts = append(fileParts, part)
	}
	if !uniqueStrings(fileParts) {
		return fmt.Errorf("agent reviewer IDs must produce distinct filenames")
	}
	header, blindIDs, err := readBlankRatingsTemplate(templatePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	absTemplate, err := filepath.Abs(templatePath)
	if err != nil {
		return err
	}
	manifest := agentReviewManifest{SchemaVersion: qualitybench.AgentConsensusSchemaVersion, Template: absTemplate, ReviewerType: "llm"}
	for agentIndex, agent := range agents {
		entry := agentReviewManifestAgent{AgentID: agent}
		for run := 1; run <= runs; run++ {
			reviewer := fmt.Sprintf("%s-run-%d", agent, run)
			name := fileParts[agentIndex] + fmt.Sprintf("-run-%d.csv", run)
			path := filepath.Join(outDir, name)
			if err := writeBlankAgentBallot(path, header, blindIDs, reviewer); err != nil {
				return err
			}
			entry.Runs = append(entry.Runs, agentReviewManifestRun{Run: run, Reviewer: reviewer, Ratings: name})
		}
		manifest.Agents = append(manifest.Agents, entry)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := writeJSON(manifestPath, manifest); err != nil {
		return err
	}
	fmt.Printf("prepared %d blank ballots for %d agents in %s\nmanifest: %s\nno agents were launched\n", len(agents)*runs, len(agents), outDir, manifestPath)
	return nil
}

func compareAgentReviewPackets(manifestPath, outDir string) error {
	data, err := os.ReadFile(manifestPath) // #nosec G304 -- operator-supplied path
	if err != nil {
		return err
	}
	var manifest agentReviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if manifest.SchemaVersion != qualitybench.AgentConsensusSchemaVersion || manifest.ReviewerType != "llm" || len(manifest.Agents) != 3 {
		return fmt.Errorf("manifest must use schema version %d, reviewer_type llm, and exactly 3 agents", qualitybench.AgentConsensusSchemaVersion)
	}
	_, blindIDs, err := readBlankRatingsTemplate(manifest.Template)
	if err != nil {
		return err
	}
	runByBlind := make(map[string]string, len(blindIDs))
	for _, id := range blindIDs {
		runByBlind[id] = id
	}
	base := filepath.Dir(manifestPath)
	var ballots []qualitybench.AgentBallot
	seenAgents := map[string]bool{}
	for _, agent := range manifest.Agents {
		if agent.AgentID == "" || seenAgents[agent.AgentID] || len(agent.Runs) != 2 {
			return fmt.Errorf("manifest agents must be distinct and contain exactly two runs")
		}
		seenAgents[agent.AgentID] = true
		for _, run := range agent.Runs {
			path := run.Ratings
			if !filepath.IsAbs(path) {
				path = filepath.Join(base, path)
			}
			ratings, err := readRatingsCSV(path, runByBlind)
			if err != nil {
				return err
			}
			for _, rating := range ratings {
				if rating.Reviewer != run.Reviewer {
					return fmt.Errorf("%s: reviewer must be %q", path, run.Reviewer)
				}
			}
			ballots = append(ballots, qualitybench.AgentBallot{AgentID: agent.AgentID, Run: run.Run, Ratings: ratings})
		}
	}
	comparison, err := qualitybench.CompareAgentBallots(ballots, blindIDs)
	if err != nil {
		return err
	}
	if outDir == "" {
		outDir = filepath.Join(base, "comparison")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	jsonPath := filepath.Join(outDir, "agent-consensus.json")
	if err := writeJSON(jsonPath, comparison); err != nil {
		return err
	}
	csvPath := filepath.Join(outDir, "agent-consensus.csv")
	if err := writeAgentConsensusCSV(csvPath, comparison); err != nil {
		return err
	}
	fmt.Printf("compared %d ballots across %d blind IDs\nconsensus JSON: %s\nconsensus CSV: %s\nunresolved ties: lost fact %d, template defect %d\n", len(ballots), len(comparison.Items), jsonPath, csvPath, comparison.UnresolvedLostFactTies, comparison.UnresolvedTemplateTies)
	return nil
}

func readBlankRatingsTemplate(path string) ([]string, []string, error) {
	f, err := os.Open(path) // #nosec G304 -- operator-supplied path
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(rows) < 2 {
		return nil, nil, fmt.Errorf("ratings template %s has no rows", path)
	}
	required := []string{"blind_id", "reviewer", "reviewer_type", "readability", "hierarchy", "template_fidelity", "factual_completeness", "usability", "lost_critical_fact", "critical_template_defect"}
	if len(rows[0]) != len(required) {
		return nil, nil, fmt.Errorf("ratings template %s has unexpected columns", path)
	}
	for i := range required {
		if rows[0][i] != required[i] {
			return nil, nil, fmt.Errorf("ratings template %s column %d must be %q", path, i+1, required[i])
		}
	}
	seen, ids := map[string]bool{}, make([]string, 0, len(rows)-1)
	for i, row := range rows[1:] {
		if len(row) != len(required) || row[0] == "" || seen[row[0]] {
			return nil, nil, fmt.Errorf("ratings template %s row %d has invalid blind_id", path, i+2)
		}
		seen[row[0]], ids = true, append(ids, row[0])
	}
	return rows[0], ids, nil
}

func writeBlankAgentBallot(path string, header, blindIDs []string, reviewer string) error {
	f, err := os.Create(path) // #nosec G304 -- path is operator-selected output
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		_ = f.Close()
		return err
	}
	for _, id := range blindIDs {
		if err := w.Write([]string{id, reviewer, "llm", "", "", "", "", "", "", ""}); err != nil {
			_ = f.Close()
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func writeAgentConsensusCSV(path string, comparison qualitybench.AgentComparison) error {
	f, err := os.Create(path) // #nosec G304 -- path is operator-selected output
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"blind_id", "readability", "hierarchy", "template_fidelity", "factual_completeness", "usability", "lost_critical_fact", "lost_true_votes", "critical_template_defect", "defect_true_votes"})
	for _, item := range comparison.Items {
		row := []string{item.BlindID, formatHalf(item.Consensus.Readability), formatHalf(item.Consensus.Hierarchy), formatHalf(item.Consensus.TemplateFidelity), formatHalf(item.Consensus.FactualCompleteness), formatHalf(item.Consensus.Usability), boolConsensus(item.LostCriticalFact), strconv.Itoa(item.LostCriticalFactVotes), boolConsensus(item.CriticalTemplateDefect), strconv.Itoa(item.CriticalTemplateDefectVotes)}
		if err := w.Write(row); err != nil {
			_ = f.Close()
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func splitNonempty(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func uniqueStrings(v []string) bool {
	seen := map[string]bool{}
	for _, s := range v {
		if seen[s] {
			return false
		}
		seen[s] = true
	}
	return true
}
func safeFilePart(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
func formatHalf(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }
func boolConsensus(v *bool) string {
	if v == nil {
		return "tie"
	}
	return strconv.FormatBool(*v)
}
