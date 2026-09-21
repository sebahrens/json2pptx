package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/qualitybench"
)

func TestDefaultTemplatesExist(t *testing.T) {
	tmpls, err := parseTemplates(defaultTemplates, "../../templates")
	if err != nil {
		t.Fatalf("default benchmark templates must exist: %v", err)
	}
	if len(tmpls) != 3 || !tmpls[2].HeldOut || tmpls[0].HeldOut || tmpls[2].Name != "portability-side-logo" || tmpls[2].Path == "" {
		t.Errorf("unexpected templates: %+v", tmpls)
	}
}

func TestParseTemplatesAcceptsExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.pptx")
	if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	tmpls, err := parseTemplates("held="+path+":portability:heldout", "../../templates")
	if err != nil {
		t.Fatal(err)
	}
	if len(tmpls) != 1 || tmpls[0].Name != "held" || tmpls[0].Family != "portability" || !tmpls[0].HeldOut || tmpls[0].Path != path {
		t.Fatalf("templates = %+v", tmpls)
	}
}

func TestParseTemplatesRejectsMissingAndMalformed(t *testing.T) {
	if _, err := parseTemplates("clean-white:editorial", "../../templates"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("missing template must be rejected, got %v", err)
	}
	if _, err := parseTemplates("midnight-blue", "../../templates"); err == nil {
		t.Error("entry without family must be rejected")
	}
}

func TestLoadBriefsHasTwelve(t *testing.T) {
	b, err := loadBriefs("../../tests/quality/agent_briefs.json")
	if err != nil || len(b) < 12 {
		t.Fatalf("briefs: %d %v", len(b), err)
	}
}

func TestReadRatingsCSVValidatesAndLabelsReviewers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ratings.csv")
	csv := "blind_id,reviewer,reviewer_type,readability,hierarchy,template_fidelity,factual_completeness,usability,lost_critical_fact,critical_template_defect\nabc,reviewer-a,human,5,4,4,5,4,false,false\n"
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	ratings, err := readRatingsCSV(path, map[string]string{"abc": "run-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings) != 1 || ratings[0].RunID != "run-1" || ratings[0].ReviewerType != "human" || ratings[0].Usability != 4 {
		t.Fatalf("ratings = %+v", ratings)
	}
	bad := strings.Replace(csv, ",5,4,4,5,4,", ",6,4,4,5,4,", 1)
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readRatingsCSV(path, map[string]string{"abc": "run-1"}); err == nil || !strings.Contains(err.Error(), "1 to 5") {
		t.Fatalf("invalid score error = %v", err)
	}
}

func TestApplyRatingsCombinesTwoReviewerFiles(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "report.json")
	report := qualitybench.Report{Evidence: []qualitybench.Evidence{{
		Request: qualitybench.Request{RunID: "run-1", Configuration: "redesigned"},
		BlindID: "abc",
	}}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	header := "blind_id,reviewer,reviewer_type,readability,hierarchy,template_fidelity,factual_completeness,usability,lost_critical_fact,critical_template_defect\n"
	paths := make([]string, 2)
	for i, reviewer := range []string{"alice", "bob"} {
		paths[i] = filepath.Join(dir, reviewer+".csv")
		if err := os.WriteFile(paths[i], []byte(header+"abc,"+reviewer+",human,5,5,5,5,5,false,false\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := applyRatings(reportPath, strings.Join(paths, ",")); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(updated, &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.RatedPairs != 1 || strings.HasPrefix(report.Summary.ReleaseDecision, "inconclusive") {
		t.Fatalf("summary = %+v", report.Summary)
	}
}
