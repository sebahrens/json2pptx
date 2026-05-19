package patterns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCQASummary_Registration(t *testing.T) {
	p, ok := Default().Get("scqa-summary")
	if !ok {
		t.Fatal("expected scqa-summary to be registered in default registry")
	}
	if p.Name() != "scqa-summary" {
		t.Errorf("Name() = %q, want %q", p.Name(), "scqa-summary")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
	if !strings.Contains(p.UseWhen(), "prefer") {
		t.Errorf("UseWhen() should contain contrastive language: %q", p.UseWhen())
	}
}

func validSCQAValues() *SCQASummaryValues {
	return &SCQASummaryValues{
		Situation:    SCQAText{"Cloud spend grew 38% YoY in FY25."},
		Complication: SCQAText{"Margin eroded as workloads scaled."},
		Questions:    []string{"What drives the spend?", "Where can we recover margin?"},
		Answer:       []string{"Right-size top workloads.", "Stand up FinOps governance."},
	}
}

func TestSCQASummary_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	if err := p.Validate(validSCQAValues(), nil, nil); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestSCQASummary_Validate_RequiredFields(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	cases := []struct {
		name    string
		mutate  func(*SCQASummaryValues)
		wantErr string
	}{
		{
			"empty_situation",
			func(v *SCQASummaryValues) { v.Situation = SCQAText{} },
			"at least 1 items",
		},
		{
			"empty_complication",
			func(v *SCQASummaryValues) { v.Complication = SCQAText{} },
			"at least 1 items",
		},
		{
			"empty_questions",
			func(v *SCQASummaryValues) { v.Questions = nil },
			"at least 1 items",
		},
		{
			"empty_answer",
			func(v *SCQASummaryValues) { v.Answer = nil },
			"at least 1 items",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := validSCQAValues()
			tc.mutate(v)
			err := p.Validate(v, nil, nil)
			if err == nil {
				t.Fatalf("expected validation error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSCQASummary_Validate_TooManyItems(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Answer = []string{"a", "b", "c", "d", "e"}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for more than 4 answer items")
	}
	if !strings.Contains(err.Error(), "at most 4") {
		t.Errorf("expected max_items error, got: %v", err)
	}
}

func TestSCQASummary_Validate_ItemTooLong(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Questions[0] = strings.Repeat("x", 241)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for too-long item")
	}
	if !strings.Contains(err.Error(), "exceeds maxLength 240") {
		t.Errorf("expected max_length error, got: %v", err)
	}
}

func TestSCQASummary_Validate_EmptyItem(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Answer = []string{"Real answer", ""}
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for empty item")
	}
	if !strings.Contains(err.Error(), "answer[1] is required") {
		t.Errorf("expected required error for empty item, got: %v", err)
	}
}

func TestSCQASummary_Validate_CellOverrideOutOfRange(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	overrides := map[int]any{99: &SCQASummaryCellOverride{AccentBar: true}}
	err := p.Validate(validSCQAValues(), nil, overrides)
	if err == nil {
		t.Fatal("expected validation error for out-of-range cell override key")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected out_of_range error, got: %v", err)
	}
}

func TestSCQASummary_Validate_CellOverrideBadKey(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	overrides := map[int]any{
		0: &struct {
			BadKey string `json:"bad_key"`
		}{BadKey: "nope"},
	}
	err := p.Validate(validSCQAValues(), nil, overrides)
	if err == nil {
		t.Fatal("expected validation error for bad cell override key")
	}
	if !strings.Contains(err.Error(), `unknown key "bad_key"`) {
		t.Errorf("expected unknown_key error, got: %v", err)
	}
}

func TestSCQASummary_UnmarshalJSON_StringForm(t *testing.T) {
	raw := []byte(`{
        "situation": "Single-paragraph situation.",
        "complication": "Single-paragraph complication.",
        "questions": ["Q1", "Q2"],
        "answer": ["A1", "A2"]
    }`)
	var v SCQASummaryValues
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(v.Situation) != 1 || v.Situation[0] != "Single-paragraph situation." {
		t.Errorf("situation = %#v, want single-item list", v.Situation)
	}
	if len(v.Complication) != 1 || v.Complication[0] != "Single-paragraph complication." {
		t.Errorf("complication = %#v, want single-item list", v.Complication)
	}
}

func TestSCQASummary_UnmarshalJSON_ArrayForm(t *testing.T) {
	raw := []byte(`{
        "situation": ["s1", "s2"],
        "complication": ["c1"],
        "questions": ["Q1"],
        "answer": ["A1"]
    }`)
	var v SCQASummaryValues
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(v.Situation) != 2 || v.Situation[0] != "s1" || v.Situation[1] != "s2" {
		t.Errorf("situation = %#v, want [s1, s2]", v.Situation)
	}
	if len(v.Complication) != 1 || v.Complication[0] != "c1" {
		t.Errorf("complication = %#v, want [c1]", v.Complication)
	}
}

func TestSCQASummary_Expand_DefaultLayout(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid == nil {
		t.Fatal("expected non-nil grid")
	}
	if got := len(grid.Rows); got != 4 {
		t.Fatalf("expected 4 rows, got %d", got)
	}
	for i, row := range grid.Rows {
		if got := len(row.Cells); got != 2 {
			t.Errorf("row[%d]: expected 2 cells, got %d", i, got)
		}
	}
	// Columns spec is [1, 4] so left column is ~20%.
	if string(grid.Columns) != "[1, 4]" {
		t.Errorf("columns = %s, want [1, 4]", string(grid.Columns))
	}
}

func TestSCQASummary_Expand_LabelCellsHaveAccentFill(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	labels := []string{"Situation", "Complication", "Questions", "Answer"}
	for i, row := range grid.Rows {
		labelCell := row.Cells[0]
		var fill string
		if err := json.Unmarshal(labelCell.Shape.Fill, &fill); err != nil {
			t.Fatalf("row[%d] label fill unmarshal: %v", i, err)
		}
		if fill != "accent1" {
			t.Errorf("row[%d] label fill = %q, want accent1", i, fill)
		}
		if !strings.Contains(string(labelCell.Shape.Text), labels[i]) {
			t.Errorf("row[%d] label text missing %q: %s", i, labels[i], string(labelCell.Shape.Text))
		}
	}
}

func TestSCQASummary_Expand_ContentCellsNoFill(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	for i, row := range grid.Rows {
		contentCell := row.Cells[1]
		var fill string
		if err := json.Unmarshal(contentCell.Shape.Fill, &fill); err != nil {
			t.Fatalf("row[%d] content fill unmarshal: %v", i, err)
		}
		if fill != "none" {
			t.Errorf("row[%d] content fill = %q, want none", i, fill)
		}
	}
}

func TestSCQASummary_Expand_BulletsRenderForMultiItem(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	v := validSCQAValues()
	v.Answer = []string{"First answer.", "Second answer."}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// Row 3 is Answer; cell 1 is the content cell.
	contentText := string(grid.Rows[3].Cells[1].Shape.Text)
	if !strings.Contains(contentText, "• First answer.") {
		t.Errorf("expected bullet prefix on multi-item content, got: %s", contentText)
	}
	if !strings.Contains(contentText, "• Second answer.") {
		t.Errorf("expected bullet prefix on second item, got: %s", contentText)
	}
}

func TestSCQASummary_Expand_NoBulletForSingleItem(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	// validSCQAValues has a single-item situation/complication; should not be bulleted.
	situationText := string(grid.Rows[0].Cells[1].Shape.Text)
	if strings.Contains(situationText, "•") {
		t.Errorf("single-item situation should not have bullet prefix, got: %s", situationText)
	}
}

func TestSCQASummary_Expand_AccentOverride(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	ovr := &SCQASummaryOverrides{Accent: "accent3"}
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	var fill string
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Fill, &fill); err != nil {
		t.Fatalf("label fill unmarshal: %v", err)
	}
	if fill != "accent3" {
		t.Errorf("label fill = %q, want accent3", fill)
	}
}

func TestSCQASummary_Expand_CellOverrideAccentBar(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	overrides := map[int]any{
		0: &SCQASummaryCellOverride{AccentBar: true},
		7: &SCQASummaryCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, overrides)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}
	if grid.Rows[0].Cells[0].AccentBar == nil {
		t.Error("expected cell[0] (Situation label) to have an accent bar")
	}
	if grid.Rows[3].Cells[1].AccentBar == nil {
		t.Error("expected cell[7] (Answer content) to have an accent bar")
	}
	if grid.Rows[0].Cells[1].AccentBar != nil {
		t.Error("cell[1] should not have an accent bar")
	}
}

func TestSCQASummary_Schema(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
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
}

func TestSCQASummary_Taxonomy(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	tax := p.Taxonomy()
	if tax.Category == "" {
		t.Error("Category should be set")
	}
	if tax.DensityClass == "" {
		t.Error("DensityClass should be set")
	}
	if tax.AccentWeight == "" {
		t.Error("AccentWeight should be set")
	}
	if len(tax.NarrativeRole) == 0 {
		t.Error("NarrativeRole should be non-empty")
	}
	if len(tax.PairsWith) == 0 {
		t.Error("PairsWith should be non-empty")
	}
}

func TestSCQASummary_Golden_Default(t *testing.T) {
	p, _ := Default().Get("scqa-summary")
	grid, err := p.Expand(ExpandContext{}, validSCQAValues(), nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	got, err := json.MarshalIndent(grid, "", "  ")
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "scqa-summary", "default.golden.json")
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
}
