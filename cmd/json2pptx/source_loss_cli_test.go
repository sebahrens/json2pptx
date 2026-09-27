package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

func TestWriteJSONErrorRetainsTypedSourceLoss(t *testing.T) {
	for _, code := range []string{patterns.ErrCodeTextTrimmed, patterns.ErrCodeReadabilityTrimmed, patterns.ErrCodeTableRowsTruncated, patterns.ErrCodeTextBelowReadableMin} {
		t.Run(code, func(t *testing.T) {
			loss := &patterns.ValidationError{Code: code, Path: "/slides/0/content/1", Message: "required source omitted", Fix: &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"split_at_row": 7, "hidden_rows": 73}}}
			err := fmt.Errorf("generation refused: %w", loss)
			report := filepath.Join(t.TempDir(), "result.json")
			if got := writeJSONError(report, err); !errors.Is(got, loss) {
				t.Fatalf("original failure not returned: %v", got)
			}
			data, readErr := os.ReadFile(report)
			if readErr != nil {
				t.Fatal(readErr)
			}
			var output JSONOutput
			if err := json.Unmarshal(data, &output); err != nil {
				t.Fatal(err)
			}
			if output.Success || output.OutputPath != "" || output.Error != err.Error() || len(output.FitFindings) != 1 {
				t.Fatalf("refusal envelope incomplete: %+v", output)
			}
			finding := output.FitFindings[0]
			if finding.Code != code || finding.Path != loss.Path || finding.Action != "refuse" || finding.Fix == nil || finding.Fix.Kind != loss.Fix.Kind || finding.Fix.Params["hidden_rows"] != float64(73) {
				t.Fatalf("source-loss repair missing: %+v", finding)
			}
			if writeJSONError("", err) != err {
				t.Fatal("no-report path changed original error")
			}
		})
	}
}

func TestWriteJSONErrorDoesNotInventSourceLoss(t *testing.T) {
	for _, err := range []error{errors.New("ordinary failure"), &patterns.ValidationError{Code: patterns.ErrCodeFitOverflow, Message: "different finding"}, fmt.Errorf("wrapped: %w", (*patterns.ValidationError)(nil))} {
		report := filepath.Join(t.TempDir(), "result.json")
		if got := writeJSONError(report, err); got != err {
			t.Fatalf("changed unrelated error: %v", got)
		}
		data, readErr := os.ReadFile(report)
		if readErr != nil {
			t.Fatal(readErr)
		}
		var output JSONOutput
		if err := json.Unmarshal(data, &output); err != nil {
			t.Fatal(err)
		}
		if output.Success || len(output.FitFindings) != 0 {
			t.Fatalf("invented source-loss finding: %+v", output)
		}
	}
	err := errors.New("original generation failure")
	if got := writeJSONError(filepath.Join(t.TempDir(), "missing", "result.json"), err); got == nil || !strings.Contains(got.Error(), err.Error()) || !strings.Contains(got.Error(), "also failed to write JSON output") {
		t.Fatalf("report-write failure swallowed original error: %v", got)
	}
}

func TestCLIActualSourceLossRetainsRepairWithoutPublishing(t *testing.T) {
	// Each case owns its input, report and destination; templates are read-only.
	t.Parallel()
	for _, kind := range []string{"table", "bullets"} {
		for _, mode := range []string{"", "warn", "off", "strict"} {
			t.Run(kind+"/mode="+mode, func(t *testing.T) {
				dir := t.TempDir()
				outputDir := filepath.Join(dir, "output")
				if err := os.Mkdir(outputDir, 0700); err != nil {
					t.Fatal(err)
				}
				destination := filepath.Join(outputDir, "deck.pptx")
				if err := os.WriteFile(destination, []byte("existing-deck"), 0600); err != nil {
					t.Fatal(err)
				}
				rows := make([][]string, 80)
				for i := range rows {
					rows[i] = []string{fmt.Sprintf("Required-R%02d", i), "Owner", "Status"}
				}
				item := map[string]any{"placeholder_id": "body", "type": "table", "table_value": map[string]any{"headers": []string{"Evidence", "Owner", "Status"}, "rows": rows}}
				code := patterns.ErrCodeTableRowsTruncated
				if kind == "bullets" {
					bullets := make([]string, 10)
					for i := range bullets {
						bullets[i] = fmt.Sprintf("P%02d %s", i, strings.Repeat("Required ownership and reporting evidence. ", 24))
					}
					item = map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": bullets}
					code = patterns.ErrCodeTextTrimmed
				}
				deck := map[string]any{"template": "abstract", "output_filename": "deck.pptx", "slides": []any{map[string]any{"layout_id": "slideLayout3", "content": []any{item}}}}
				data, err := json.Marshal(deck)
				if err != nil {
					t.Fatal(err)
				}
				input := filepath.Join(dir, "input.json")
				if err := os.WriteFile(input, data, 0600); err != nil {
					t.Fatal(err)
				}
				report := filepath.Join(dir, "result.json")
				if err := runJSONMode(input, report, testutil.TemplatesDir(), outputDir, "", false, false, "", mode, false, "strict", "", false); err == nil {
					t.Fatal("source-loss input unexpectedly published")
				}
				reportData, err := os.ReadFile(report)
				if err != nil {
					t.Fatal(err)
				}
				var result JSONOutput
				if err := json.Unmarshal(reportData, &result); err != nil {
					t.Fatal(err)
				}
				if result.Success || result.OutputPath != "" {
					t.Fatalf("refusal misrepresented as output: %+v", result)
				}
				finding := firstFindingCode(result.FitFindings, code)
				if finding == nil || finding.Path != "/slides/0/content/0" || finding.Action != "refuse" || finding.Fix == nil {
					t.Fatalf("CLI source-loss repair lost: %+v", result)
				}
				if kind == "table" && (finding.Fix.Params["split_at_row"] != float64(7) || finding.Fix.Params["hidden_rows"] != float64(73)) {
					t.Fatalf("table split parameters lost: %+v", finding)
				}
				if kind == "bullets" && (finding.Fix.Kind != "split_bullets" || finding.Fix.Params["max_items"] != float64(1)) {
					t.Fatalf("CLI lost source-preserving paragraph repair: %+v", finding)
				}
				preserved, err := os.ReadFile(destination)
				if err != nil || string(preserved) != "existing-deck" {
					t.Fatalf("destination changed: %q %v", preserved, err)
				}
				files, err := os.ReadDir(outputDir)
				if err != nil || len(files) != 1 {
					t.Fatalf("refusal leaked files: %v %v", files, err)
				}
				after, err := os.ReadFile(input)
				if err != nil || string(after) != string(data) {
					t.Fatal("authored input changed")
				}
			})
		}
	}
}

func TestCLIFittingParagraphsStillPublishCompleteSource(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"", "warn", "off", "strict"} {
		t.Run("mode="+mode, func(t *testing.T) {
			dir := t.TempDir()
			bullets := []string{"Owner confirms reporting deadlines.", "Reviewer resolves outstanding handoffs."}
			deck := map[string]any{"template": "abstract", "output_filename": "complete.pptx", "slides": []any{map[string]any{"layout_id": "slideLayout3", "content": []any{map[string]any{"placeholder_id": "body", "type": "bullets", "bullets_value": bullets}}}}}
			data, err := json.Marshal(deck)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(dir, "input.json")
			if err := os.WriteFile(input, data, 0600); err != nil {
				t.Fatal(err)
			}
			report := filepath.Join(dir, "result.json")
			outputDir := filepath.Join(dir, "output")
			if err := runJSONMode(input, report, testutil.TemplatesDir(), outputDir, "", false, false, "", mode, false, "strict", "", false); err != nil {
				t.Fatal(err)
			}
			reportData, err := os.ReadFile(report)
			if err != nil {
				t.Fatal(err)
			}
			var result JSONOutput
			if err := json.Unmarshal(reportData, &result); err != nil {
				t.Fatal(err)
			}
			if !result.Success || result.OutputPath == "" {
				t.Fatalf("fitting source not published: %+v", result)
			}
			slideXML := readZipText(t, filepath.Join(outputDir, "complete.pptx"), "ppt/slides/slide")
			for _, text := range bullets {
				if !strings.Contains(slideXML, text) {
					t.Fatalf("missing required source %q", text)
				}
			}
		})
	}
}
