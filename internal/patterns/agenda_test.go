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
	v.Items[2] = strings.Repeat("W", 60)
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "items[2]") || !strings.Contains(got[0], "about 59") {
		t.Fatalf("dense unbroken title warning: %v", got)
	}
	v.Items[2] = strings.Repeat("W", 59)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("measured wide target should fit: %v", got)
	}
	v.Items[2] = strings.Repeat("word ", 20)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("word-like schema maximum should fit: %v", got)
	}
	v.Items = v.Items[:5]
	v.Items[2] = strings.Repeat("W", 62)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 1 || !strings.Contains(got[0], "about 61") {
		t.Fatalf("five-row unbroken title warning: %v", got)
	}
	v.Items[2] = strings.Repeat("W", 61)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("five-row measured target should fit: %v", got)
	}
	v.Items = v.Items[:4]
	v.Items[2] = strings.Repeat("W", 100)
	if got := p.PostExpandWarnings(ExpandContext{}, v, nil); len(got) != 0 {
		t.Fatalf("four-row schema maximum should fit: %v", got)
	}
}

// agendaTestPara decodes one agenda cell's single paragraph.
type agendaTestPara struct {
	Content string  `json:"content"`
	Size    float64 `json:"size"`
	Bold    bool    `json:"bold"`
	Color   string  `json:"color"`
	Font    string  `json:"font"`
	Alpha   float64 `json:"alpha"`
}

func agendaCellPara(t *testing.T, text json.RawMessage) agendaTestPara {
	t.Helper()
	var body struct {
		Paragraphs []agendaTestPara `json:"paragraphs"`
	}
	if err := json.Unmarshal(text, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Paragraphs) != 1 {
		t.Fatalf("want one paragraph, got %+v", body.Paragraphs)
	}
	return body.Paragraphs[0]
}

// TestAgendaRuleBasedStyle pins design review C5 (go-slide-creator-r3gsw):
// 28pt accent serif numerals, 14pt items, 0.5pt rules between rows, no
// filled tiles.
func TestAgendaRuleBasedStyle(t *testing.T) {
	grid, err := (&agenda{}).Expand(ExpandContext{}, &AgendaValues{Items: []string{"Intro", "Analysis", "Decision"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 5 {
		t.Fatalf("rows = %d, want 3 items + 2 rules", len(grid.Rows))
	}
	for i, row := range grid.Rows {
		if i%2 == 1 {
			if row.MaxHeight != agendaRulePt || len(row.Cells) != 1 || row.Cells[0].ColSpan != 2 {
				t.Errorf("row %d is not a 0.5pt full-width rule: %+v", i, row)
			}
			continue
		}
		for _, c := range row.Cells {
			if string(c.Shape.Fill) != `"none"` {
				t.Errorf("row %d cell fill = %s, want none (no tiles)", i, c.Shape.Fill)
			}
		}
		num := agendaCellPara(t, row.Cells[0].Shape.Text)
		if num.Size != 28 || num.Font != "+mj-lt" || num.Color != "accent1" || num.Alpha != 0 {
			t.Errorf("row %d numeral = %+v, want 28pt +mj-lt accent1", i, num)
		}
		title := agendaCellPara(t, row.Cells[1].Shape.Text)
		if title.Size != agendaTitleSize || title.Bold || title.Color != "dk1" || title.Alpha != 0 {
			t.Errorf("row %d item = %+v, want plain 14pt dk1", i, title)
		}
	}
	if grid.VerticalAlign != "center" {
		t.Errorf("vertical_align = %q, want center", grid.VerticalAlign)
	}
}

// TestAgendaRepeatHighlightsCurrent: on a repeated agenda the current item is
// bold dk1 and every other row is at 50%.
func TestAgendaRepeatHighlightsCurrent(t *testing.T) {
	grid, err := (&agenda{}).Expand(ExpandContext{}, &AgendaValues{Items: []string{"A", "B", "C"}}, &AgendaOverrides{Highlight: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for item := 0; item < 3; item++ {
		row := grid.Rows[2*item]
		title := agendaCellPara(t, row.Cells[1].Shape.Text)
		num := agendaCellPara(t, row.Cells[0].Shape.Text)
		if item == 1 {
			if !title.Bold || title.Alpha != 0 || num.Alpha != 0 {
				t.Errorf("current item = %+v / %+v, want bold at full opacity", title, num)
			}
			continue
		}
		if title.Bold || title.Alpha != 50 || num.Alpha != 50 {
			t.Errorf("item %d = %+v / %+v, want regular at 50%%", item, title, num)
		}
	}
}

func TestAgendaOverridesKeepSizes(t *testing.T) {
	grid, err := (&agenda{}).Expand(ExpandContext{}, &AgendaValues{Items: []string{"Intro", "Analysis"}}, &AgendaOverrides{TitleSize: 13, NumberSize: 20}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := agendaCellPara(t, grid.Rows[0].Cells[1].Shape.Text).Size; got != 13 {
		t.Errorf("title size = %g, want 13", got)
	}
	if got := agendaCellPara(t, grid.Rows[0].Cells[0].Shape.Text).Size; got != 20 {
		t.Errorf("number size = %g, want 20", got)
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

	if len(grid.Rows) != 5 {
		t.Fatalf("expected 3 items + 2 rules, got %d rows", len(grid.Rows))
	}

	// Each item row has 2 cells (numeral + title); odd rows are the rules.
	for i, row := range grid.Rows {
		if i%2 == 0 && len(row.Cells) != 2 {
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

	if len(grid.Rows) != 5 {
		t.Fatalf("expected 3 items + 2 rules, got %d rows", len(grid.Rows))
	}
}

func TestAgenda_Registry(t *testing.T) {
	_, ok := Default().Get("agenda")
	if !ok {
		t.Error("agenda pattern not found in default registry")
	}
}
