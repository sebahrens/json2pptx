package generator

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// tableWithRows builds a table spec with n data rows and 5 columns.
func tableWithRows(n int) *types.TableSpec {
	spec := &types.TableSpec{Headers: []string{"Region", "ARR", "NRR", "Headcount", "Delta"}}
	for i := 1; i <= n; i++ {
		row := make([]types.TableCell, 5)
		for j := range row {
			row[j] = types.TableCell{Content: "cell", ColSpan: 1, RowSpan: 1}
		}
		row[0].Content = "Region " + strings.Repeat("x", i%3)
		spec.Rows = append(spec.Rows, row)
	}
	return spec
}

// go-slide-creator-oaif: a 12-row table shipped 9 rows plus a literal
// "…and 3 more rows" cell — three regions' financials simply absent from the
// deck — reported at action=review / severity=info with the quality score
// still 100 and the gate still passing. Dropping authored facts is content
// loss, not a styling nit.
func TestDetectTablePreflight_TruncationRefuses(t *testing.T) {
	// A placeholder tall enough for ~9 rows, given 12.
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/1",
		Headers: tableWithRows(12).Headers,
		Rows:    tableWithRows(12).Rows,
		Bounds:  types.BoundingBox{Width: 8229600, Height: 2400000},
	})

	var trunc *patterns.FitFinding
	for i := range findings {
		if findings[i].Code == patterns.ErrCodeTableRowsTruncated {
			trunc = &findings[i]
			break
		}
	}
	if trunc == nil {
		t.Fatalf("12 rows in a 9-row placeholder must predict truncation, got %+v", findings)
	}
	if trunc.Action != "refuse" {
		t.Errorf("action = %q, want refuse — dropping authored rows is data loss", trunc.Action)
	}
	if trunc.Fix == nil || trunc.Fix.Kind != "split_at_row" {
		t.Fatalf("fix = %+v, want kind split_at_row", trunc.Fix)
	}
	// The split point must be supplied, not left for the agent to derive.
	if _, ok := trunc.Fix.Params["split_at_row"]; !ok {
		t.Errorf("fix.params must carry split_at_row, got %v", trunc.Fix.Params)
	}
	if !strings.Contains(trunc.Message, "absent from the deck") {
		t.Errorf("message should say the rows are absent, got: %s", trunc.Message)
	}
}

// A table that fits must predict nothing.
func TestDetectTablePreflight_FittingTableIsQuiet(t *testing.T) {
	spec := tableWithRows(4)
	findings := DetectTablePreflight(TablePreflightInput{
		Path:    "/slides/0/content/1",
		Headers: spec.Headers,
		Rows:    spec.Rows,
		Bounds:  types.BoundingBox{Width: 8229600, Height: 4525963},
	})
	for _, f := range findings {
		if f.Code == patterns.ErrCodeTableRowsTruncated {
			t.Errorf("a 4-row table in a full-height placeholder must not predict truncation: %+v", f)
		}
	}
}

// The render-time site must agree with the prediction: same code, same action.
func TestPopulateTable_TruncationRefuses(t *testing.T) {
	spec := tableWithRows(12)
	placeholder := types.PlaceholderInfo{
		ID:     "body",
		Bounds: types.BoundingBox{X: 0, Y: 0, Width: 8229600, Height: 2400000},
	}

	result, err := PopulateTableInShape(spec, placeholder, nil, nil)
	if err != nil {
		t.Fatalf("PopulateTableInShape: %v", err)
	}

	var trunc *patterns.FitFinding
	for i := range result.Findings {
		if result.Findings[i].Code == patterns.ErrCodeTableRowsTruncated {
			trunc = &result.Findings[i]
			break
		}
	}
	if trunc == nil {
		t.Fatalf("render-time truncation must emit %s, got %+v", patterns.ErrCodeTableRowsTruncated, result.Findings)
	}
	if trunc.Action != "refuse" {
		t.Errorf("action = %q, want refuse (must match the preflight prediction)", trunc.Action)
	}
	if trunc.Fix == nil || trunc.Fix.Params["split_at_row"] == nil {
		t.Errorf("fix must carry split_at_row, got %+v", trunc.Fix)
	}
}
