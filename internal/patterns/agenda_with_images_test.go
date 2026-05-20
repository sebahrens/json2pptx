package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgendaWithImages_Registration(t *testing.T) {
	p, ok := Default().Get("agenda-with-images")
	if !ok {
		t.Fatal("expected agenda-with-images to be registered in default registry")
	}
	if p.Name() != "agenda-with-images" {
		t.Errorf("Name() = %q, want %q", p.Name(), "agenda-with-images")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty (D6)")
	}
}

func TestAgendaWithImages_Taxonomy(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" {
		t.Errorf("taxonomy fields must be populated: %+v", tx)
	}
	if len(tx.NarrativeRole) == 0 {
		t.Errorf("narrative role must have at least one value")
	}
}

func validAgendaWithImagesValues(n int) *AgendaWithImagesValues {
	items := make([]AgendaWithImagesItem, n)
	titles := []string{"Executive Summary", "Market Analysis", "Strategy", "Roadmap", "Next Steps", "Appendix"}
	for i := 0; i < n; i++ {
		items[i] = AgendaWithImagesItem{Title: titles[i], Subtitle: "Brief context", ImageLabel: "Photo placeholder"}
	}
	return &AgendaWithImagesValues{Items: items}
}

func TestAgendaWithImages_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	for _, n := range []int{3, 4, 5, 6} {
		if err := p.Validate(validAgendaWithImagesValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected validation error: %v", n, err)
		}
	}
}

func TestAgendaWithImages_Validate_TooFew(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(2)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 items")
	}
	if !strings.Contains(err.Error(), "agenda") {
		t.Errorf("expected sibling hint mentioning agenda, got: %v", err)
	}
}

func TestAgendaWithImages_Validate_TooMany(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(6)
	v.Items = append(v.Items, AgendaWithImagesItem{Title: "Seventh"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 6 items")
	}
}

func TestAgendaWithImages_Validate_MissingTitle(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(4)
	v.Items[1].Title = "  "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank title")
	}
}

func TestAgendaWithImages_Validate_TitleTooLong(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(4)
	v.Items[0].Title = strings.Repeat("X", 81)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for title > 80 chars")
	}
}

func TestAgendaWithImages_Validate_NegativeNumber(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(4)
	v.Items[2].Number = -1
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for negative number")
	}
}

func TestAgendaWithImages_Expand_FiveItems_ProducesRows(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(5)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// 5 content rows + 4 divider rows = 9
	if len(grid.Rows) != 9 {
		t.Fatalf("expected 9 rows (5 content + 4 dividers), got %d", len(grid.Rows))
	}
	// First row should have 3 cells (number, title, image) because ImageLabel is set
	if len(grid.Rows[0].Cells) != 3 {
		t.Errorf("content row should have 3 cells when image_label present, got %d", len(grid.Rows[0].Cells))
	}
	// Second row should be a divider (1 cell with col_span 3)
	if len(grid.Rows[1].Cells) != 1 {
		t.Errorf("divider row should have 1 cell, got %d", len(grid.Rows[1].Cells))
	}
	if grid.Rows[1].Cells[0].ColSpan != 3 {
		t.Errorf("divider cell should span 3 columns, got col_span=%d", grid.Rows[1].Cells[0].ColSpan)
	}
}

func TestAgendaWithImages_Expand_CollapsesRightZoneWhenNoImageLabel(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := &AgendaWithImagesValues{
		Items: []AgendaWithImagesItem{
			{Title: "First"},
			{Title: "Second", ImageLabel: "Has image"},
			{Title: "Third"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// Row 0 (no image): 2 cells (number + title with col_span=2)
	if len(grid.Rows[0].Cells) != 2 {
		t.Errorf("row 0 should have 2 cells when no image_label, got %d", len(grid.Rows[0].Cells))
	}
	if grid.Rows[0].Cells[1].ColSpan != 2 {
		t.Errorf("title cell should span 2 columns when right zone collapses, got col_span=%d", grid.Rows[0].Cells[1].ColSpan)
	}
	// Row 2 (with image, index 2 in rows array because divider is at 1): 3 cells
	if len(grid.Rows[2].Cells) != 3 {
		t.Errorf("row with image_label should have 3 cells, got %d", len(grid.Rows[2].Cells))
	}
}

func TestAgendaWithImages_Expand_AutoAssignsNumbers(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(3)
	// All numbers default to 0 → should auto-assign 01, 02, 03
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	expected := []string{"01", "02", "03"}
	contentRows := []int{0, 2, 4} // skip divider rows
	for i, rowIdx := range contentRows {
		numberCell := grid.Rows[rowIdx].Cells[0]
		var textObj struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(numberCell.Shape.Text, &textObj); err != nil {
			t.Fatalf("unmarshal badge text: %v", err)
		}
		if len(textObj.Paragraphs) == 0 || textObj.Paragraphs[0].Content != expected[i] {
			t.Errorf("row %d badge: expected %q, got %q", rowIdx, expected[i], textObj.Paragraphs[0].Content)
		}
	}
}

func TestAgendaWithImages_Expand_HonorsExplicitNumber(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := &AgendaWithImagesValues{
		Items: []AgendaWithImagesItem{
			{Number: 1, Title: "Alpha"},
			{Number: 5, Title: "Echo"},
			{Number: 9, Title: "India"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	expected := []string{"01", "05", "09"}
	contentRows := []int{0, 2, 4}
	for i, rowIdx := range contentRows {
		numberCell := grid.Rows[rowIdx].Cells[0]
		var textObj struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		_ = json.Unmarshal(numberCell.Shape.Text, &textObj)
		if textObj.Paragraphs[0].Content != expected[i] {
			t.Errorf("row %d badge: expected %q, got %q", rowIdx, expected[i], textObj.Paragraphs[0].Content)
		}
	}
}

func TestAgendaWithImages_Expand_AppliesCellOverride(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(3)
	co := map[int]any{
		1: &AgendaWithImagesCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, co)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// Item 1's title cell is at row index 2 (item 0 row, divider, item 1 row), cell 1.
	titleCell := grid.Rows[2].Cells[1]
	if titleCell.AccentBar == nil {
		t.Fatal("expected accent bar on title cell of item 1")
	}
	if titleCell.AccentBar.Position != "left" {
		t.Errorf("accent bar position = %q, want %q", titleCell.AccentBar.Position, "left")
	}
}

func TestAgendaWithImages_Expand_DividerRowsHaveFill(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := validAgendaWithImagesValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// Dividers at rows 1, 3, 5
	for _, dividerIdx := range []int{1, 3, 5} {
		row := grid.Rows[dividerIdx]
		if len(row.Cells) != 1 || row.Cells[0].Shape == nil {
			t.Fatalf("divider row %d malformed: %+v", dividerIdx, row)
		}
		var fill string
		if err := json.Unmarshal(row.Cells[0].Shape.Fill, &fill); err != nil {
			t.Fatalf("unmarshal divider fill: %v", err)
		}
		if fill != "lt2" {
			t.Errorf("divider fill = %q, want %q", fill, "lt2")
		}
	}
}

func TestAgendaWithImages_Schema_Valid(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	sch := p.Schema()
	if sch == nil {
		t.Fatal("Schema() returned nil")
	}
	data, err := json.Marshal(sch)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	if len(data) == 0 {
		t.Error("schema marshalled to empty bytes")
	}
}

func TestAgendaWithImages_ExemplarValues_ExpandsCleanly(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	ex, ok := p.(Exemplar)
	if !ok {
		t.Fatal("agenda-with-images does not implement Exemplar")
	}
	vals := ex.ExemplarValues()
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("exemplar values failed validation: %v", err)
	}
	if _, err := p.Expand(ExpandContext{}, vals, nil, nil); err != nil {
		t.Fatalf("exemplar Expand: %v", err)
	}
}
