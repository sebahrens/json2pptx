package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func TestStatHeroCombinedWideCopyWarning(t *testing.T) {
	p := &statHero{}
	full := &StatHeroValues{Value: strings.Repeat("W", 20), Unit: strings.Repeat("W", 10), Label: strings.Repeat("W", 80), Context: strings.Repeat("W", 120), Source: strings.Repeat("W", 80)}
	got := p.PostExpandWarnings(testThemeCtx(), full, nil)
	if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) || !strings.Contains(got[0], "values.value/unit/label/context/source") {
		t.Fatalf("full wide stack warning: %v", got)
	}
	// Every field at its maximum does not write readably on every shipped
	// template even with word breaks (go-slide-creator-n1muf).
	wordy := &StatHeroValues{Value: budgetLikeCopy(20), Unit: budgetLikeCopy(10), Label: budgetLikeCopy(80), Context: budgetLikeCopy(120), Source: budgetLikeCopy(80)}
	if got := p.PostExpandWarnings(testThemeCtx(), wordy, nil); len(got) != 1 {
		t.Fatalf("full word-like stack should warn: %v", got)
	}
	balanced := &StatHeroValues{Value: budgetLikeCopy(8), Unit: budgetLikeCopy(4), Label: budgetLikeCopy(32), Context: budgetLikeCopy(49), Source: budgetLikeCopy(32)}
	if got := p.PostExpandWarnings(testThemeCtx(), balanced, nil); len(got) != 0 {
		t.Fatalf("measured balanced stack should fit: %v", got)
	}
	isolated := &StatHeroValues{Value: strings.Repeat("W", 20), Label: "Market size"}
	if got := p.PostExpandWarnings(testThemeCtx(), isolated, nil); len(got) != 0 {
		t.Fatalf("isolated wide value should fit: %v", got)
	}
}

// TestStatHeroSizesToItsCell pins go-slide-creator-hidji: the 120pt figure
// is for a whole slide; in a compose segment, regions cell or grid cell the
// stack is sized to that rectangle (no stored shrink, so no figure drawn
// over its label), keeps the figure dominant, and reports BODY_TOO_LONG only
// when even the floor sizes do not fit.
func TestStatHeroSizesToItsCell(t *testing.T) {
	p := &statHero{}
	v := &StatHeroValues{Value: "32%", Label: "Gross margin", Context: "Up 4 points on plan"}
	cell := func(wPt, hPt float64) ExpandContext {
		ctx := testThemeCtx()
		ctx.LayoutBounds = LayoutBounds{Width: int64(wPt * 12700), Height: int64(hPt * 12700)}
		return ctx
	}
	sizes := func(ctx ExpandContext) (value, label float64, fits bool) {
		t.Helper()
		grid, err := p.Expand(ctx, v, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		text := grid.Rows[0].Cells[0].Shape.Text
		tb, err := shapegrid.ResolveTextInput(text)
		if err != nil {
			t.Fatal(err)
		}
		value = float64(tb.Paragraphs[0].Runs[0].FontSize) / 100
		label = float64(tb.Paragraphs[1].Runs[0].FontSize) / 100
		w, h := contentAreaPt(ctx)
		return value, label, writtenFitsAt(ctx.themeFonts(), text, w, h)
	}

	// The whole content area keeps the 120pt figure.
	if value, label, _ := sizes(cell(830, 400)); value != sizeHeroFigurePt || label != scaleLeadPt {
		t.Errorf("full slide: figure %.0fpt label %.0fpt, want %.0f / %.0f", value, label, sizeHeroFigurePt, scaleLeadPt)
	}
	prev := sizeHeroFigurePt + 1
	for _, c := range []struct{ w, h float64 }{{300, 260}, {300, 150}, {290, 120}, {220, 100}} {
		ctx := cell(c.w, c.h)
		value, label, fits := sizes(ctx)
		if !fits {
			t.Errorf("%.0f×%.0fpt: the stack (figure %.0fpt, label %.0fpt) is written shrunk", c.w, c.h, value, label)
		}
		if value < statHeroMinValuePt || value < 2*label {
			t.Errorf("%.0f×%.0fpt: figure %.0fpt no longer dominates the %.0fpt label", c.w, c.h, value, label)
		}
		if value > prev {
			t.Errorf("%.0f×%.0fpt: figure %.0fpt grew in a smaller cell (was %.0fpt)", c.w, c.h, value, prev)
		}
		t.Logf("%.0f×%.0fpt: figure %.0fpt label %.0fpt", c.w, c.h, value, label)
		prev = value
		if w := p.PostExpandWarnings(ctx, v, nil); len(w) != 0 {
			t.Errorf("%.0f×%.0fpt: a stack that fits warned: %v", c.w, c.h, w)
		}
	}

	// Below the floor the figure stays at the minimum and a finding says so.
	tiny := cell(160, 50)
	if value, _, _ := sizes(tiny); value != statHeroMinValuePt {
		t.Errorf("tiny cell figure %.0fpt, want the %.0fpt floor", value, statHeroMinValuePt)
	}
	if w := p.PostExpandWarnings(tiny, v, nil); len(w) != 1 || !strings.Contains(w[0], ErrCodeBodyTooLong) {
		t.Errorf("tiny cell should report BODY_TOO_LONG: %v", w)
	}

	// A figure wider than its cell shrinks rather than breaking mid-number.
	wide := &StatHeroValues{Value: "€1,234,567", Label: "Savings"}
	grid, err := p.Expand(cell(220, 300), wide, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tb, _ := shapegrid.ResolveTextInput(grid.Rows[0].Cells[0].Shape.Text)
	if got := float64(tb.Paragraphs[0].Runs[0].FontSize) / 100; got >= sizeHeroFigurePt || !statHeroValueUnbroken("", wide.Value, got, 220-2*defaultShapeInsetLRPt) {
		t.Errorf("wide figure in a narrow cell kept %.0fpt", got)
	}

	// Authored sizes are kept.
	grid, err = p.Expand(cell(290, 120), v, &StatHeroOverrides{ValueSize: 60}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tb, _ = shapegrid.ResolveTextInput(grid.Rows[0].Cells[0].Shape.Text)
	if got := tb.Paragraphs[0].Runs[0].FontSize; got != 6000 {
		t.Errorf("authored value_size 60 became %d (pt×100)", got)
	}
}

func budgetLikeCopy(length int) string {
	return strings.Repeat("word ", length/5) + strings.Repeat("w", length%5)
}

func TestStatHero(t *testing.T) {
	p := &statHero{}

	t.Run("metadata", func(t *testing.T) {
		if p.Name() != "stat-hero" {
			t.Errorf("Name() = %q, want %q", p.Name(), "stat-hero")
		}
		if p.Version() != 1 {
			t.Errorf("Version() = %d, want 1", p.Version())
		}
		if p.CellsHint() != "1" {
			t.Errorf("CellsHint() = %q, want %q", p.CellsHint(), "1")
		}
	})

	t.Run("schema_valid_json_schema", func(t *testing.T) {
		s := p.Schema()
		if s == nil {
			t.Fatal("Schema() returned nil")
		}
		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			t.Fatalf("Schema marshal: %v", err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("Schema unmarshal: %v", err)
		}
		if m["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
			t.Errorf("missing $schema draft 2020-12")
		}
		if m["type"] != "object" {
			t.Errorf("root type = %v, want object", m["type"])
		}
	})

	t.Run("validate_happy_path", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: "TAM by FY27"}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_full_fields", func(t *testing.T) {
		v := &StatHeroValues{
			Value:   "99.9%",
			Unit:    "uptime",
			Label:   "Service availability",
			Context: "Measured over trailing 12 months",
			Source:  "Internal monitoring dashboard",
		}
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("validate_missing_value", func(t *testing.T) {
		v := &StatHeroValues{Value: "", Label: "TAM"}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "values.value is required") {
			t.Errorf("error %q does not mention values.value required", err)
		}
	})

	t.Run("validate_missing_label", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: ""}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "values.label is required") {
			t.Errorf("error %q does not mention values.label required", err)
		}
	})

	t.Run("validate_value_too_long", func(t *testing.T) {
		v := &StatHeroValues{Value: "this value is way too long!!", Label: "TAM"}
		err := p.Validate(v, nil, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "maxLength 20") {
			t.Errorf("error %q does not mention maxLength", err)
		}
	})

	t.Run("expand_minimal", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: "Addressable market"}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		if grid == nil {
			t.Fatal("Expand returned nil grid")
		}
		if len(grid.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(grid.Rows))
		}
		if len(grid.Rows[0].Cells) != 1 {
			t.Fatalf("expected 1 cell, got %d", len(grid.Rows[0].Cells))
		}
		// Verify text contains the value
		cell := grid.Rows[0].Cells[0]
		if cell.Shape == nil {
			t.Fatal("cell.Shape is nil")
		}
		textStr := string(cell.Shape.Text)
		if !strings.Contains(textStr, "$2.4B") {
			t.Errorf("text does not contain value: %s", textStr)
		}
		if !strings.Contains(textStr, "Addressable market") {
			t.Errorf("text does not contain label: %s", textStr)
		}
	})

	t.Run("expand_with_unit", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Unit: "TAM", Label: "By FY27"}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		// The unit is a smaller trailing run on the value's baseline, not
		// part of the display-size figure (go-slide-creator-yn2pw).
		tb, err := shapegrid.ResolveTextInput(grid.Rows[0].Cells[0].Shape.Text)
		if err != nil {
			t.Fatal(err)
		}
		runs := tb.Paragraphs[0].Runs
		if len(runs) != 2 || runs[0].Text != "$2.4B" || strings.TrimSpace(runs[1].Text) != "TAM" {
			t.Fatalf("value paragraph runs = %+v, want value then unit", runs)
		}
		ratio := float64(runs[1].FontSize) / float64(runs[0].FontSize)
		if ratio < 0.35 || ratio > 0.45 {
			t.Errorf("unit is %.0f%% of the value size, want 35–45%%", ratio*100)
		}
		if runs[1].Bold != runs[0].Bold {
			t.Error("unit should share the value's weight")
		}
	})

	t.Run("expand_accent_override", func(t *testing.T) {
		v := &StatHeroValues{Value: "42%", Label: "Growth"}
		ovr := &StatHeroOverrides{Accent: "accent3"}
		grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}
		textStr := string(grid.Rows[0].Cells[0].Shape.Text)
		if !strings.Contains(textStr, "accent3") {
			t.Errorf("text should use accent3 color: %s", textStr)
		}
	})

	t.Run("golden_default", func(t *testing.T) {
		v := &StatHeroValues{Value: "$2.4B", Label: "Addressable AI consulting market by FY27"}
		grid, err := p.Expand(ExpandContext{}, v, nil, nil)
		if err != nil {
			t.Fatalf("Expand: %v", err)
		}

		got, err := json.MarshalIndent(grid, "", "  ")
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}

		goldenPath := filepath.Join("testdata", "stat-hero", "default.golden.json")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
				t.Fatalf("write golden: %v", err)
			}
			t.Log("golden file updated")
			return
		}

		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden (run with UPDATE_GOLDEN=1 to create): %v", err)
		}

		if string(got) != string(want) {
			t.Errorf("golden mismatch.\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}
