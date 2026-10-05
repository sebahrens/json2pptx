package quality

import (
	"encoding/csv"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestPublicationStatusesSurviveCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.csv")
	errorText := "generation refused, required rows missing\nnot approved"
	writeCSV(t, path, []metrics{{Name: "complete", GenerationStatus: "generated_unreviewed"}, {Name: "adverse", GenerationStatus: "source_loss_refused", GenerationError: errorText}})
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("publication ledger unreadable: %v %v", rows, err)
	}
	columns := make(map[string]int)
	for i, name := range rows[0] {
		columns[name] = i
	}
	status, hasStatus := columns["generation_status"]
	failure, hasFailure := columns["generation_error"]
	if !hasStatus || !hasFailure {
		t.Fatalf("publication status omitted: %v", rows[0])
	}
	if rows[1][status] != "generated_unreviewed" || rows[2][status] != "source_loss_refused" || rows[2][failure] != errorText {
		t.Fatalf("publication outcomes misrepresented: %v", rows)
	}
}

func TestPublicationContractExitHelper(t *testing.T) {
	if os.Getenv("JSON2PPTX_PUBLICATION_EXIT_HELPER") == "1" {
		os.Exit(1)
	}
	if os.Getenv("JSON2PPTX_PUBLICATION_EXIT_HELPER") == "2" {
		os.Exit(2)
	}
}

func TestPublicationContractRejectsFalseApprovals(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestPublicationContractExitHelper$") //nolint:gosec // controlled test binary
	cmd.Env = append(os.Environ(), "JSON2PPTX_PUBLICATION_EXIT_HELPER=1")
	exitErr := cmd.Run()
	if exitErr == nil {
		t.Fatal("helper must fail")
	}
	otherExit := exec.Command(os.Args[0], "-test.run=^TestPublicationContractExitHelper$") //nolint:gosec // controlled test binary
	otherExit.Env = append(os.Environ(), "JSON2PPTX_PUBLICATION_EXIT_HELPER=2")
	otherExitErr := otherExit.Run()
	if otherExitErr == nil {
		t.Fatal("second helper must fail")
	}
	input := "tests/quality/fixtures/dense-table-16x6.json"
	source, err := os.ReadFile(filepath.Join(findProjectRoot(t), filepath.FromSlash(input)))
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"valid_refusal", "valid_success", "changed_input", "unknown_failure", "process_error", "wrong_exit", "missing_success", "success_with_error", "missing_artifact", "empty_artifact", "outside_artifact", "published_loss", "adverse_success", "wrong_code", "wrong_path", "wrong_action", "missing_fix", "wrong_fix", "wrong_counts", "wrong_visible", "wrong_split", "leaked_artifact", "refusal_output_path", "refusal_success", "empty_error", "extra_findings"} {
		t.Run(scenario, func(t *testing.T) {
			output := t.TempDir()
			ok := false
			report := generationReport{Success: &ok, Error: "source loss", FitFindings: []patterns.FitFinding{{ValidationError: patterns.ValidationError{Code: patterns.ErrCodeTableRowsTruncated, Path: "/slides/0/content/1", Fix: &patterns.FixSuggestion{Kind: "split_at_row", Params: map[string]any{"visible_rows": float64(13), "split_at_row": float64(13), "hidden_rows": float64(3)}}}, Action: "refuse"}}}
			path, data, runErr := input, source, exitErr
			artifact := filepath.Join(output, "deck.pptx")
			writeArtifact := func(path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("artifact"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "valid_success" || scenario == "success_with_error" || scenario == "missing_artifact" || scenario == "empty_artifact" || scenario == "outside_artifact" || scenario == "published_loss" || scenario == "adverse_success" {
				ok, runErr, path = true, nil, "examples/basic-deck.json"
				report.OutputPath, report.Error = artifact, ""
				if scenario != "published_loss" {
					report.FitFindings = nil
				}
				if scenario != "missing_artifact" {
					writeArtifact(artifact)
				}
			}
			switch scenario {
			case "changed_input":
				data = append(append([]byte(nil), source...), '\n')
			case "unknown_failure":
				path = "examples/basic-deck.json"
			case "process_error":
				runErr = errors.New("failed to start process")
			case "wrong_exit":
				runErr = otherExitErr
			case "missing_success":
				report.Success = nil
			case "success_with_error":
				report.Error = "failure"
			case "empty_artifact":
				if err := os.WriteFile(artifact, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "outside_artifact":
				report.OutputPath = filepath.Join(t.TempDir(), "outside.pptx")
				writeArtifact(report.OutputPath)
			case "adverse_success":
				path = input
			case "wrong_code":
				report.FitFindings[0].Code = patterns.ErrCodeFitOverflow
			case "wrong_path":
				report.FitFindings[0].Path = "/slides/9/content/8"
			case "wrong_action":
				report.FitFindings[0].Action = "review"
			case "missing_fix":
				report.FitFindings[0].Fix = nil
			case "wrong_fix":
				report.FitFindings[0].Fix.Kind = "reduce_text"
			case "wrong_counts":
				report.FitFindings[0].Fix.Params["hidden_rows"] = float64(1)
			case "wrong_visible":
				report.FitFindings[0].Fix.Params["visible_rows"] = float64(1)
			case "wrong_split":
				report.FitFindings[0].Fix.Params["split_at_row"] = float64(1)
			case "leaked_artifact":
				writeArtifact(artifact)
			case "refusal_output_path":
				report.OutputPath = artifact
			case "refusal_success":
				ok = true
			case "empty_error":
				report.Error = ""
			case "extra_findings":
				report.FitFindings = append(report.FitFindings, report.FitFindings[0])
			}
			status, err := classifyPublication(path, data, report, runErr, output)
			expectValid := scenario == "valid_success" || scenario == "valid_refusal"
			if (err == nil) != expectValid {
				t.Fatalf("scenario accepted=%t, expected=%t: status=%q err=%v", err == nil, expectValid, status, err)
			}
			if scenario == "valid_success" && status != "generated_unreviewed" {
				t.Fatalf("success became approval: %q", status)
			}
			if scenario == "valid_refusal" && status != "source_loss_refused" {
				t.Fatalf("refusal became approval: %q", status)
			}
			if !expectValid && status != "" {
				t.Fatalf("invalid outcome given status %q", status)
			}
		})
	}
}
