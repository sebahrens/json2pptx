package qualitybench

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const AgentConsensusSchemaVersion = 1

// AgentBallot is one complete blinded rating run from one agent.
type AgentBallot struct {
	AgentID string
	Run     int
	Ratings []Rating
}

type DimensionValues struct {
	Readability         float64 `json:"readability"`
	Hierarchy           float64 `json:"hierarchy"`
	TemplateFidelity    float64 `json:"template_fidelity"`
	FactualCompleteness float64 `json:"factual_completeness"`
	Usability           float64 `json:"usability"`
}

type AgentRepeatability struct {
	AgentID       string          `json:"agent_id"`
	MeanAbsDelta  DimensionValues `json:"mean_absolute_delta"`
	LostFactAgree float64         `json:"lost_critical_fact_agreement"`
	TemplateAgree float64         `json:"critical_template_defect_agreement"`
}

type ConsensusItem struct {
	BlindID                     string                      `json:"blind_id"`
	Raw                         map[string][]RatingSnapshot `json:"raw"`
	AgentMeans                  map[string]DimensionValues  `json:"agent_means"`
	Consensus                   DimensionValues             `json:"consensus"`
	LostCriticalFact            *bool                       `json:"lost_critical_fact"`
	LostCriticalFactVotes       int                         `json:"lost_critical_fact_votes"`
	CriticalTemplateDefect      *bool                       `json:"critical_template_defect"`
	CriticalTemplateDefectVotes int                         `json:"critical_template_defect_votes"`
}

type RatingSnapshot struct {
	Run                    int  `json:"run"`
	Readability            int  `json:"readability"`
	Hierarchy              int  `json:"hierarchy"`
	TemplateFidelity       int  `json:"template_fidelity"`
	FactualCompleteness    int  `json:"factual_completeness"`
	Usability              int  `json:"usability"`
	LostCriticalFact       bool `json:"lost_critical_fact"`
	CriticalTemplateDefect bool `json:"critical_template_defect"`
}

type AgentComparison struct {
	SchemaVersion          int                  `json:"schema_version"`
	Agents                 []string             `json:"agents"`
	RunsPerAgent           int                  `json:"runs_per_agent"`
	Items                  []ConsensusItem      `json:"items"`
	Repeatability          []AgentRepeatability `json:"repeatability"`
	UnresolvedLostFactTies int                  `json:"unresolved_lost_critical_fact_ties"`
	UnresolvedTemplateTies int                  `json:"unresolved_critical_template_defect_ties"`
}

// CompareAgentBallots validates a 3x2 blinded review matrix and computes an
// auditable consensus. Numeric consensus is the median of the three agents'
// two-run means. Boolean consensus requires four of six votes; 3-3 stays nil.
func CompareAgentBallots(ballots []AgentBallot, expectedBlindIDs []string) (AgentComparison, error) {
	var out AgentComparison
	agents, indexed, err := validateAndIndexAgentBallots(ballots, expectedBlindIDs)
	if err != nil {
		return out, err
	}
	out.Agents = agents
	out.SchemaVersion, out.RunsPerAgent = AgentConsensusSchemaVersion, 2
	for _, blindID := range expectedBlindIDs {
		item := buildConsensusItem(blindID, out.Agents, indexed)
		if item.LostCriticalFact == nil {
			out.UnresolvedLostFactTies++
		}
		if item.CriticalTemplateDefect == nil {
			out.UnresolvedTemplateTies++
		}
		out.Items = append(out.Items, item)
	}
	for _, agent := range out.Agents {
		out.Repeatability = append(out.Repeatability, measureRepeatability(agent, expectedBlindIDs, indexed))
	}
	return out, nil
}

type agentRatingIndex map[string]map[int]map[string]Rating

func validateAndIndexAgentBallots(ballots []AgentBallot, expectedBlindIDs []string) ([]string, agentRatingIndex, error) {
	if len(ballots) != 6 {
		return nil, nil, fmt.Errorf("expected 6 ballots (3 agents x 2 runs), got %d", len(ballots))
	}
	if len(expectedBlindIDs) == 0 {
		return nil, nil, fmt.Errorf("at least one expected blind ID is required")
	}
	expected := make(map[string]bool, len(expectedBlindIDs))
	for _, id := range expectedBlindIDs {
		if id == "" || expected[id] {
			return nil, nil, fmt.Errorf("expected blind IDs must be nonempty and unique")
		}
		expected[id] = true
	}
	byAgent := map[string]map[int]AgentBallot{}
	for _, ballot := range ballots {
		id := strings.TrimSpace(ballot.AgentID)
		if err := validateAgentBallot(id, ballot, expected); err != nil {
			return nil, nil, err
		}
		if byAgent[id] == nil {
			byAgent[id] = map[int]AgentBallot{}
		}
		if _, exists := byAgent[id][ballot.Run]; exists {
			return nil, nil, fmt.Errorf("duplicate ballot for agent %q run %d", id, ballot.Run)
		}
		byAgent[id][ballot.Run] = ballot
	}
	if len(byAgent) != 3 {
		return nil, nil, fmt.Errorf("expected 3 distinct agents, got %d", len(byAgent))
	}
	agents := make([]string, 0, 3)
	indexed := agentRatingIndex{}
	for id, runs := range byAgent {
		if len(runs) != 2 || runs[1].Run != 1 || runs[2].Run != 2 {
			return nil, nil, fmt.Errorf("agent %q must have runs 1 and 2", id)
		}
		agents = append(agents, id)
		indexed[id] = map[int]map[string]Rating{1: indexRatings(runs[1].Ratings), 2: indexRatings(runs[2].Ratings)}
	}
	sort.Strings(agents)
	return agents, indexed, nil
}

func validateAgentBallot(agentID string, ballot AgentBallot, expected map[string]bool) error {
	if agentID == "" || ballot.Run < 1 || ballot.Run > 2 {
		return fmt.Errorf("invalid agent ballot identity %q run %d", ballot.AgentID, ballot.Run)
	}
	seen := map[string]bool{}
	for _, rating := range ballot.Ratings {
		if !strings.EqualFold(rating.ReviewerType, "llm") {
			return fmt.Errorf("agent %q run %d has non-llm reviewer type", agentID, ballot.Run)
		}
		if !expected[rating.RunID] || seen[rating.RunID] {
			return fmt.Errorf("agent %q run %d has unknown or duplicate blind ID %q", agentID, ballot.Run, rating.RunID)
		}
		seen[rating.RunID] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("agent %q run %d covers %d of %d blind IDs", agentID, ballot.Run, len(seen), len(expected))
	}
	return nil
}

func indexRatings(ratings []Rating) map[string]Rating {
	indexed := make(map[string]Rating, len(ratings))
	for _, rating := range ratings {
		indexed[rating.RunID] = rating
	}
	return indexed
}

func buildConsensusItem(blindID string, agents []string, indexed agentRatingIndex) ConsensusItem {
	item := ConsensusItem{BlindID: blindID, Raw: map[string][]RatingSnapshot{}, AgentMeans: map[string]DimensionValues{}}
	var dimensions [5][]float64
	for _, agent := range agents {
		a, b := indexed[agent][1][blindID], indexed[agent][2][blindID]
		item.Raw[agent] = []RatingSnapshot{snapshot(1, a), snapshot(2, b)}
		mean := meanDimensions(a, b)
		item.AgentMeans[agent] = mean
		dimensions[0] = append(dimensions[0], mean.Readability)
		dimensions[1] = append(dimensions[1], mean.Hierarchy)
		dimensions[2] = append(dimensions[2], mean.TemplateFidelity)
		dimensions[3] = append(dimensions[3], mean.FactualCompleteness)
		dimensions[4] = append(dimensions[4], mean.Usability)
		for _, rating := range []Rating{a, b} {
			if rating.LostCriticalFact {
				item.LostCriticalFactVotes++
			}
			if rating.CriticalTemplateDefect {
				item.CriticalTemplateDefectVotes++
			}
		}
	}
	item.Consensus = DimensionValues{median3(dimensions[0]), median3(dimensions[1]), median3(dimensions[2]), median3(dimensions[3]), median3(dimensions[4])}
	item.LostCriticalFact = majority(item.LostCriticalFactVotes)
	item.CriticalTemplateDefect = majority(item.CriticalTemplateDefectVotes)
	return item
}

func meanDimensions(a, b Rating) DimensionValues {
	return DimensionValues{mean2(a.Readability, b.Readability), mean2(a.Hierarchy, b.Hierarchy), mean2(a.TemplateFidelity, b.TemplateFidelity), mean2(a.FactualCompleteness, b.FactualCompleteness), mean2(a.Usability, b.Usability)}
}

func measureRepeatability(agent string, blindIDs []string, indexed agentRatingIndex) AgentRepeatability {
	var sums DimensionValues
	var lostAgree, templateAgree int
	for _, id := range blindIDs {
		a, b := indexed[agent][1][id], indexed[agent][2][id]
		sums.Readability += math.Abs(float64(a.Readability - b.Readability))
		sums.Hierarchy += math.Abs(float64(a.Hierarchy - b.Hierarchy))
		sums.TemplateFidelity += math.Abs(float64(a.TemplateFidelity - b.TemplateFidelity))
		sums.FactualCompleteness += math.Abs(float64(a.FactualCompleteness - b.FactualCompleteness))
		sums.Usability += math.Abs(float64(a.Usability - b.Usability))
		if a.LostCriticalFact == b.LostCriticalFact {
			lostAgree++
		}
		if a.CriticalTemplateDefect == b.CriticalTemplateDefect {
			templateAgree++
		}
	}
	n := float64(len(blindIDs))
	sums = DimensionValues{sums.Readability / n, sums.Hierarchy / n, sums.TemplateFidelity / n, sums.FactualCompleteness / n, sums.Usability / n}
	return AgentRepeatability{agent, sums, float64(lostAgree) / n, float64(templateAgree) / n}
}

func snapshot(run int, r Rating) RatingSnapshot {
	return RatingSnapshot{run, r.Readability, r.Hierarchy, r.TemplateFidelity, r.FactualCompleteness, r.Usability, r.LostCriticalFact, r.CriticalTemplateDefect}
}
func mean2(a, b int) float64      { return float64(a+b) / 2 }
func median3(v []float64) float64 { sort.Float64s(v); return v[1] }
func majority(trueVotes int) *bool {
	if trueVotes == 3 {
		return nil
	}
	v := trueVotes >= 4
	return &v
}
