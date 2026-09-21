package qualitybench

import "testing"

func TestCompareAgentBallotsConsensusAndTies(t *testing.T) {
	ids := []string{"a", "b"}
	var ballots []AgentBallot
	for agentIndex, agent := range []string{"alpha", "beta", "gamma"} {
		for run := 1; run <= 2; run++ {
			ratings := make([]Rating, 0, len(ids))
			for itemIndex, id := range ids {
				score := 2 + agentIndex
				if run == 2 {
					score++
				}
				ratings = append(ratings, Rating{
					RunID: id, Reviewer: agent, ReviewerType: "llm",
					Readability: score, Hierarchy: score, TemplateFidelity: score,
					FactualCompleteness: score, Usability: score,
					LostCriticalFact:       agentIndex == 0 || (agentIndex == 1 && run == 1),
					CriticalTemplateDefect: itemIndex == 0 && agentIndex < 2,
				})
			}
			ballots = append(ballots, AgentBallot{AgentID: agent, Run: run, Ratings: ratings})
		}
	}
	got, err := CompareAgentBallots(ballots, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[0].Consensus.Usability != 3.5 {
		t.Fatalf("unexpected consensus: %+v", got.Items)
	}
	if got.Items[0].LostCriticalFact != nil || got.UnresolvedLostFactTies != 2 {
		t.Fatalf("3-3 lost-fact vote must remain unresolved: %+v", got.Items[0])
	}
	if got.Items[0].CriticalTemplateDefect == nil || !*got.Items[0].CriticalTemplateDefect || got.Items[0].CriticalTemplateDefectVotes != 4 {
		t.Fatalf("4-2 defect vote must resolve true: %+v", got.Items[0])
	}
	if got.Items[1].CriticalTemplateDefect == nil || *got.Items[1].CriticalTemplateDefect {
		t.Fatalf("0-6 defect vote must resolve false: %+v", got.Items[1])
	}
	if len(got.Repeatability) != 3 || got.Repeatability[0].MeanAbsDelta.Usability != 1 {
		t.Fatalf("repeatability = %+v", got.Repeatability)
	}
}

func TestCompareAgentBallotsRejectsIncompleteMatrix(t *testing.T) {
	_, err := CompareAgentBallots([]AgentBallot{}, []string{"a"})
	if err == nil {
		t.Fatal("missing ballots must fail")
	}
}
