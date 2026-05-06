package textcapacity

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestComputeBudgetGuide_EmptyConfigs(t *testing.T) {
	guide := ComputeBudgetGuide(nil, nil, pptx.RectEmu{}, 0, 0)
	if guide != nil {
		t.Fatal("expected nil for empty configs")
	}
}

func TestComputeBudgetGuide_ExpandError(t *testing.T) {
	configs := []GridBudgetConfig{{Columns: 2, Rows: 2}}
	expandFn := func(cols, rows int) (*jsonschema.ShapeGridInput, error) {
		return nil, nil // simulate failure
	}
	guide := ComputeBudgetGuide(configs, expandFn, pptx.RectEmu{}, 0, 0)
	if guide != nil {
		t.Fatal("expected nil when expand returns nil")
	}
}

func TestComputeBudgetGuide_SyntheticGrid(t *testing.T) {
	// Build a simple 2×2 equal-column grid with text shapes.
	expandFn := func(cols, rows int) (*jsonschema.ShapeGridInput, error) {
		gridRows := make([]jsonschema.GridRowInput, rows)
		for r := 0; r < rows; r++ {
			cells := make([]*jsonschema.GridCellInput, cols)
			for c := 0; c < cols; c++ {
				cells[c] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(`"accent1"`),
						Text:     json.RawMessage(`"Placeholder text"`),
					},
				}
			}
			gridRows[r] = jsonschema.GridRowInput{Cells: cells}
		}
		return &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(`2`),
			Gap:     10,
			Rows:    gridRows,
		}, nil
	}

	bounds := shapegrid.DefaultBounds(9144000, 5143500)

	configs := []GridBudgetConfig{
		{Columns: 2, Rows: 2},
		{Columns: 3, Rows: 2},
	}

	guide := ComputeBudgetGuide(configs, expandFn, bounds, 9144000, 5143500)
	if guide == nil {
		t.Fatal("expected non-nil guide")
	}

	// Check target density constants
	if guide.TargetDensity.MinPct != 60 {
		t.Errorf("expected min_pct=60, got %d", guide.TargetDensity.MinPct)
	}
	if guide.TargetDensity.IdealPct != 85 {
		t.Errorf("expected ideal_pct=85, got %d", guide.TargetDensity.IdealPct)
	}
	if guide.TargetDensity.MaxPct != 110 {
		t.Errorf("expected max_pct=110, got %d", guide.TargetDensity.MaxPct)
	}

	// The 2×2 config should have computed; the 3×2 returns grid with 2 cols
	// (expand function always returns 2-col grid), so only 2×2 produces valid results.
	if len(guide.Configurations) == 0 {
		t.Fatal("expected at least one configuration")
	}

	cfg := guide.Configurations[0]
	if cfg.Columns != 2 || cfg.Rows != 2 {
		t.Errorf("expected 2×2, got %d×%d", cfg.Columns, cfg.Rows)
	}
	if cfg.BodyMaxChars <= 0 {
		t.Errorf("expected positive body_max_chars, got %d", cfg.BodyMaxChars)
	}
	if cfg.HeaderMaxChars <= 0 {
		t.Errorf("expected positive header_max_chars, got %d", cfg.HeaderMaxChars)
	}
	// Body at 12pt should fit more chars than header at 16pt
	if cfg.BodyMaxChars <= cfg.HeaderMaxChars {
		t.Errorf("expected body (%d) > header (%d) since body font is smaller", cfg.BodyMaxChars, cfg.HeaderMaxChars)
	}
}

func TestDefaultTargetDensity(t *testing.T) {
	td := DefaultTargetDensity()
	if td.MinPct != 60 || td.IdealPct != 85 || td.MaxPct != 110 {
		t.Errorf("unexpected target density: %+v", td)
	}
}

func TestComputeBudgetGuide_LargerGridHasSmallerBudgets(t *testing.T) {
	expandFn := func(cols, rows int) (*jsonschema.ShapeGridInput, error) {
		gridRows := make([]jsonschema.GridRowInput, rows)
		for r := 0; r < rows; r++ {
			cells := make([]*jsonschema.GridCellInput, cols)
			for c := 0; c < cols; c++ {
				cells[c] = &jsonschema.GridCellInput{
					Shape: &jsonschema.ShapeSpecInput{
						Geometry: "roundRect",
						Fill:     json.RawMessage(`"accent1"`),
						Text:     json.RawMessage(`"Text"`),
					},
				}
			}
			gridRows[r] = jsonschema.GridRowInput{Cells: cells}
		}
		return &jsonschema.ShapeGridInput{
			Columns: json.RawMessage(json.RawMessage([]byte{byte('0' + cols)})),
			Gap:     10,
			Rows:    gridRows,
		}, nil
	}

	bounds := shapegrid.DefaultBounds(9144000, 5143500)

	configs := []GridBudgetConfig{
		{Columns: 2, Rows: 2},
		{Columns: 3, Rows: 3},
	}

	guide := ComputeBudgetGuide(configs, expandFn, bounds, 9144000, 5143500)
	if guide == nil {
		t.Fatal("expected non-nil guide")
	}
	if len(guide.Configurations) != 2 {
		t.Fatalf("expected 2 configurations, got %d", len(guide.Configurations))
	}

	small := guide.Configurations[0] // 2×2
	large := guide.Configurations[1] // 3×3

	// More cells = smaller cells = fewer chars per cell
	if large.BodyMaxChars >= small.BodyMaxChars {
		t.Errorf("3×3 body (%d) should be less than 2×2 body (%d)", large.BodyMaxChars, small.BodyMaxChars)
	}
	if large.HeaderMaxChars >= small.HeaderMaxChars {
		t.Errorf("3×3 header (%d) should be less than 2×2 header (%d)", large.HeaderMaxChars, small.HeaderMaxChars)
	}
}
