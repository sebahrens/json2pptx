package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgendaDenseUnbrokenTitleWarning(t *testing.T) {
	p := &agenda{}
	v := &AgendaValues{Items: make([]string, 8)}
	for i := range v.Items {
		v.Items[i] = "Section title"
	}
	v.Items[2] = strings.Repeat("W", 59)
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "items[2]") || !strings.Contains(got[0], "about 58") {
		t.Fatalf("dense unbroken title warning: %v", got)
	}
	v.Items[2] = strings.Repeat("W", 58)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured wide target should fit: %v", got)
	}
	v.Items[2] = strings.Repeat("word ", 20)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("word-like schema maximum should fit: %v", got)
	}
	v.Items = v.Items[:7]
	v.Items[2] = strings.Repeat("W", 100)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("seven-row schema maximum should fit: %v", got)
	}
}

func TestAgendaSparseTitlesGrowWithoutChangingDenseOrOverrides(t *testing.T) {
	p := &agenda{}
	for _, tc := range []struct {
		name  string
		items []string
		ovr   *AgendaOverrides
		want  float64
	}{
		{"short", []string{"Intro", "Analysis", "Decision"}, nil, 18},
		{"long title", []string{"Introduction to the regional operating model", "Analysis", "Decision"}, nil, 14},
		{"override", []string{"Intro", "Analysis", "Decision"}, &AgendaOverrides{TitleSize: 13}, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var overrides any
			if tc.ovr != nil {
				overrides = tc.ovr
			}
			grid, err := p.Expand(ExpandContext{}, &AgendaValues{Items: tc.items}, overrides, nil)
			if err != nil {
				t.Fatal(err)
			}
			var body struct {
				Paragraphs []struct {
					Size float64 `json:"size"`
				} `json:"paragraphs"`
			}
			if err := json.Unmarshal(grid.Rows[0].Cells[1].Shape.Text, &body); err != nil {
				t.Fatal(err)
			}
			if len(body.Paragraphs) != 1 || body.Paragraphs[0].Size != tc.want {
				t.Errorf("title size = %+v, want %g", body.Paragraphs, tc.want)
			}
		})
	}
}

func TestAgenda_Validate_Basic(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"Introduction", "Analysis", "Strategy"},
	}
	if err := a.Validate(v, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgenda_Validate_TooFew(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"Only One"},
	}
	if err := a.Validate(v, nil, nil); err == nil {
		t.Error("expected error for fewer than 2 items")
	}
}

func TestAgenda_Validate_TooMany(t *testing.T) {
	a := &agenda{}

	items := make([]string, 11)
	for i := range items {
		items[i] = "Section"
	}
	v := &AgendaValues{Items: items}
	if err := a.Validate(v, nil, nil); err == nil {
		t.Error("expected error for more than 10 items")
	}
}

func TestAgenda_Validate_EmptyItem(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"OK", ""},
	}
	if err := a.Validate(v, nil, nil); err == nil {
		t.Error("expected error for empty item")
	}
}

func TestAgenda_Validate_HighlightOutOfRange(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{Items: []string{"A", "B"}}
	ovr := &AgendaOverrides{Highlight: 5}
	if err := a.Validate(v, ovr, nil); err == nil {
		t.Error("expected error for highlight > item count")
	}
}

func TestAgenda_Expand_Basic(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"Introduction", "Analysis", "Strategy"},
	}
	ctx := ExpandContext{}

	grid, err := a.Expand(ctx, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
	}

	// Each row should have 2 cells: number badge + title
	for i, row := range grid.Rows {
		if len(row.Cells) != 2 {
			t.Errorf("row[%d]: expected 2 cells, got %d", i, len(row.Cells))
		}
	}
}

func TestAgenda_Expand_WithHighlight(t *testing.T) {
	a := &agenda{}

	v := &AgendaValues{
		Items: []string{"A", "B", "C"},
	}
	ovr := &AgendaOverrides{Highlight: 2}
	ctx := ExpandContext{}

	grid, err := a.Expand(ctx, v, ovr, nil)
	if err != nil {
		t.Fatalf("Expand failed: %v", err)
	}

	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(grid.Rows))
	}
}

func TestAgenda_Registry(t *testing.T) {
	_, ok := Default().Get("agenda")
	if !ok {
		t.Error("agenda pattern not found in default registry")
	}
}
