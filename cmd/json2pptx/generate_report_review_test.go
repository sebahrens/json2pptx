package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestGenerateJSONOutputReportIncludesReviewFindings(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "deck.json")
	reportPath := filepath.Join(dir, "report.json")
	input := `{
		"template": "midnight-blue",
		"slides": [{
			"layout_id": "content",
			"content": [
				{"placeholder_id": "title", "type": "text", "text_value": "Market context"},
				{"placeholder_id": "body", "type": "text", "text_value": "Brief."}
			]
		}]
	}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runJSONMode(inputPath, reportPath, filepath.Join("..", "..", "templates"), dir,
		"", false, false, "", "off", false, "off", "", false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report JSONOutput
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, finding := range report.FitFindings {
		if finding.Code == patterns.ErrCodeSlideNearlyEmpty && finding.Action == "review" {
			count++
			if finding.Fix == nil || finding.Fix.Kind != "add_items" {
				t.Errorf("near-empty review has no actionable fix: %+v", finding)
			}
		}
	}
	if count != 1 {
		t.Fatalf("JSON report has %d near-empty review findings, want 1: %+v", count, report.FitFindings)
	}
}
