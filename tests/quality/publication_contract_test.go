package quality

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

type generationReport struct {
	Success     *bool                 `json:"success"`
	OutputPath  string                `json:"output_path"`
	Error       string                `json:"error"`
	FitFindings []patterns.FitFinding `json:"fit_findings"`
}

// These are unchanged adverse inputs, not approved missing decks. A changed
// input or changed loss must be investigated rather than silently reclassified.
var expectedSourceRefusals = map[string]struct {
	SHA             string
	Visible, Hidden int
}{
	"tests/quality/fixtures/dense-table-16x6.json":      {"c9379a66122bb5ca3e525d93293800f088e714d158f0e40f2af22cebb329dae6", 9, 7},
	"tests/quality/fixtures/multiline-cells-table.json": {"d631f632d60d8e1bd21de7d874a33f82582c7685aab8fe707b6b6092241c4737", 3, 2},
}

func classifyPublication(input string, source []byte, report generationReport, runErr error, outputDir string) (string, error) {
	if report.Success == nil {
		return "", fmt.Errorf("generation report omits success")
	}
	expected, adverse := expectedSourceRefusals[filepath.ToSlash(input)]
	if adverse && fmt.Sprintf("%x", sha256.Sum256(source)) != expected.SHA {
		return "", fmt.Errorf("adverse input changed: %s", input)
	}
	if runErr == nil {
		if adverse {
			return "", fmt.Errorf("adverse source unexpectedly published: %s", input)
		}
		if !*report.Success || report.Error != "" || report.OutputPath == "" {
			return "", fmt.Errorf("successful process has contradictory report")
		}
		for _, finding := range report.FitFindings {
			if finding.Code == patterns.ErrCodeTableRowsTruncated || finding.Code == patterns.ErrCodeTextTrimmed || finding.Code == patterns.ErrCodeReadabilityTrimmed {
				return "", fmt.Errorf("successful output reports source loss")
			}
		}
		rel, err := filepath.Rel(outputDir, report.OutputPath)
		if err != nil || rel == ".." || filepath.IsAbs(rel) || (len(rel) > 3 && rel[:3] == ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("output outside isolated directory")
		}
		info, err := os.Stat(report.OutputPath)
		if err != nil || info.IsDir() || info.Size() == 0 {
			return "", fmt.Errorf("successful report has no output artifact")
		}
		return "generated_unreviewed", nil
	}
	var exitErr *exec.ExitError
	if !adverse || !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 1 {
		return "", fmt.Errorf("unexpected generation failure: %w", runErr)
	}
	if *report.Success || report.OutputPath != "" || report.Error == "" || len(report.FitFindings) != 1 {
		return "", fmt.Errorf("invalid source-loss refusal envelope")
	}
	finding := report.FitFindings[0]
	if finding.Code != patterns.ErrCodeTableRowsTruncated || finding.Path != "/slides/0/content/1" || finding.Action != "refuse" || finding.Fix == nil || finding.Fix.Kind != "split_at_row" {
		return "", fmt.Errorf("source-loss diagnostic changed")
	}
	params := finding.Fix.Params
	if params["visible_rows"] != float64(expected.Visible) || params["split_at_row"] != float64(expected.Visible) || params["hidden_rows"] != float64(expected.Hidden) {
		return "", fmt.Errorf("source-loss counts changed")
	}
	files, err := os.ReadDir(outputDir)
	if err != nil || len(files) != 0 {
		return "", fmt.Errorf("refusal leaked output artifacts")
	}
	return "source_loss_refused", nil
}
