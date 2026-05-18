package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestDetectTablePreflight_FontScaledByColumns(t *testing.T) {
	headers := []string{"A", "B", "C", "D", "E", "F"} // 6 cols > 4 → scale
	rows := [][]types.TableCell{
		{{Content: "1"}, {Content: "2"}, {Content: "3"}, {Content: "4"}, {Content: "5"}, {Content: "6"}},
	}
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/0",
		Headers: headers,
		Rows:    rows,
		Bounds:  types.BoundingBox{Width: 9144000, Height: 4000000},
	})

	if !containsCode(findings, patterns.ErrCodeTableFontScaled) {
		t.Errorf("expected table_font_scaled finding; got codes: %v", findingCodes(findings))
	}
	for _, f := range findings {
		if f.Code != patterns.ErrCodeTableFontScaled {
			continue
		}
		if !strings.Contains(f.Message, "predicted") {
			t.Errorf("message should mark prediction: %q", f.Message)
		}
		if f.Fix == nil || f.Fix.Kind != "review" {
			t.Errorf("fix should be review, got %v", f.Fix)
		}
		if f.Action != "review" {
			t.Errorf("action = %q, want review", f.Action)
		}
	}
}

func TestDetectTablePreflight_NoFindingsForSmallTable(t *testing.T) {
	headers := []string{"A", "B"}
	rows := [][]types.TableCell{
		{{Content: "1"}, {Content: "2"}},
		{{Content: "3"}, {Content: "4"}},
	}
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/0",
		Headers: headers,
		Rows:    rows,
		Bounds:  types.BoundingBox{Width: 9144000, Height: 4000000},
	})

	if len(findings) != 0 {
		t.Errorf("expected no findings for small 2x2 table, got %d: %v", len(findings), findingCodes(findings))
	}
}

func TestDetectTablePreflight_RowsTruncated(t *testing.T) {
	headers := []string{"A", "B"}
	// 30 rows in a tiny bounds → should predict truncation.
	rows := make([][]types.TableCell, 30)
	for i := range rows {
		rows[i] = []types.TableCell{{Content: "x"}, {Content: "y"}}
	}
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/0",
		Headers: headers,
		Rows:    rows,
		Bounds:  types.BoundingBox{Width: 9144000, Height: 1500000},
	})

	if !containsCode(findings, patterns.ErrCodeTableRowsTruncated) {
		t.Errorf("expected table_rows_truncated finding; got codes: %v", findingCodes(findings))
	}
}

func TestDetectTablePreflight_ColumnWidthDeficit(t *testing.T) {
	// Two columns where one has a very long unbreakable word that won't fit.
	headers := []string{"A", "VeryLongTokenThatCannotBreakAndShouldOverflowAvailableWidth"}
	rows := [][]types.TableCell{
		{{Content: "1"}, {Content: "AnotherLongTokenThatAlsoShouldOverflowEasily"}},
	}
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/0",
		Headers: headers,
		Rows:    rows,
		Bounds:  types.BoundingBox{Width: 914400, Height: 4000000}, // 1 inch wide — tiny
	})

	if !containsCode(findings, patterns.ErrCodeColumnWidthDeficit) {
		t.Errorf("expected column_width_deficit finding; got codes: %v", findingCodes(findings))
	}
}

func TestDetectTablePreflight_EmptyHeadersNoFinding(t *testing.T) {
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/0",
		Headers: nil,
		Rows:    nil,
		Bounds:  types.BoundingBox{Width: 9144000, Height: 4000000},
	})
	if findings != nil {
		t.Errorf("expected nil for empty headers, got %v", findings)
	}
}

// --- helpers ---

func containsCode(findings []patterns.FitFinding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func findingCodes(findings []patterns.FitFinding) []string {
	codes := make([]string, len(findings))
	for i, f := range findings {
		codes[i] = f.Code
	}
	return codes
}
