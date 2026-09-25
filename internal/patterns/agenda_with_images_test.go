package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
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

func TestAgendaWithImagesRowsFillContentHeight(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	for _, count := range []int{3, 6} {
		grid, err := p.Expand(ExpandContext{}, validAgendaWithImagesValues(count), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, areaH := sizingAreaPt(ExpandContext{})
		occupied := float64(2*count-2) * grid.RowGap
		for _, row := range grid.Rows {
			if row.AutoHeight {
				occupied += row.MinHeight
			} else {
				occupied += areaH * row.Height / 100
			}
		}
		if occupied < areaH*0.70 {
			t.Errorf("%d rows occupy %.1f%% of content height, want at least 70%%", count, occupied/areaH*100)
		}
	}
}

// go-slide-creator-jodu: the image column is all-or-nothing. One row skipping
// its box punched a hole in the column and made the whole grid read as ragged
// — most visibly on a five-item agenda whose last row has no preview.
func TestAgendaWithImages_Expand_MixedImageLabelsKeepEveryPlaceholder(t *testing.T) {
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
	// Content rows are at 0, 2, 4 (dividers at 1 and 3).
	for _, rowIdx := range []int{0, 2, 4} {
		row := grid.Rows[rowIdx]
		if len(row.Cells) != 3 {
			t.Fatalf("row %d: expected number + title + placeholder, got %d cells", rowIdx, len(row.Cells))
		}
		if row.Cells[1].ColSpan != 0 {
			t.Errorf("row %d: title cell must not span the image column, got col_span=%d", rowIdx, row.Cells[1].ColSpan)
		}
		if row.Cells[2].Shape == nil || string(row.Cells[2].Shape.Fill) != `"lt2"` {
			t.Errorf("row %d: placeholder fill = %s, want lt2 on every row", rowIdx, row.Cells[2].Shape.Fill)
		}
	}
	// Only the labelled row carries a caption; the others are empty boxes.
	if len(grid.Rows[0].Cells[2].Shape.Text) != 0 {
		t.Error("row 0 has no image_label, so its placeholder must carry no caption")
	}
	if len(grid.Rows[2].Cells[2].Shape.Text) == 0 {
		t.Error("row 1 has an image_label, so its placeholder must carry the caption")
	}
}

// No item has a label: there is no column, so every title spans it.
func TestAgendaWithImages_Expand_CollapsesRightZoneWhenNoImageLabelAnywhere(t *testing.T) {
	p, _ := Default().Get("agenda-with-images")
	v := &AgendaWithImagesValues{
		Items: []AgendaWithImagesItem{
			{Title: "Why"},
			{Title: "What"},
			{Title: "How"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	for _, rowIdx := range []int{0, 2, 4} {
		row := grid.Rows[rowIdx]
		if len(row.Cells) != 2 {
			t.Fatalf("row %d: expected number + title only, got %d cells", rowIdx, len(row.Cells))
		}
		if row.Cells[1].ColSpan != 2 {
			t.Errorf("row %d: title cell should span the collapsed column, got col_span=%d", rowIdx, row.Cells[1].ColSpan)
		}
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
		numberCell := agendaBadgeShape(t, grid.Rows[rowIdx].Cells[0])
		var textObj struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(numberCell.Text, &textObj); err != nil {
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
		numberCell := agendaBadgeShape(t, grid.Rows[rowIdx].Cells[0])
		var textObj struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		_ = json.Unmarshal(numberCell.Text, &textObj)
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

// agendaBadgeShape returns the badge shape from a number cell. The badge is
// nested in a [gutter, badge, gutter] sub-grid when it has been narrowed to
// render square, and sits directly on the cell when it fills it
// (go-slide-creator-tiarr).
func agendaBadgeShape(t *testing.T, cell *jsonschema.GridCellInput) *jsonschema.ShapeSpecInput {
	t.Helper()
	if cell == nil {
		t.Fatal("number cell is nil")
	}
	if cell.Shape != nil {
		return cell.Shape
	}
	if cell.Grid == nil || len(cell.Grid.Rows) == 0 {
		t.Fatal("number cell has neither a shape nor a sub-grid")
	}
	for _, c := range cell.Grid.Rows[0].Cells {
		if c != nil && c.Shape != nil {
			return c.Shape
		}
	}
	t.Fatal("no badge shape inside the number cell's sub-grid")
	return nil
}

// go-slide-creator-tiarr: the badge filled its cell, which is 18% of the
// content width by whatever height the row took — 1.5:1 on a three-item agenda
// and 2.4:1 on a five-item one, so the "numbered square" read as an accent slab.
func TestAgendaBadgeIsNarrowedTowardSquare(t *testing.T) {
	ctx := ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: LayoutBounds{Width: 10515600, Height: 3913340},
	}
	w, h := contentAreaPt(ctx)
	colWidth := w * agendaBadgeColumnFrac

	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprintf("%d items", n), func(t *testing.T) {
			pct := agendaBadgeWidthPct(ctx, n)
			if pct >= 100 {
				t.Fatalf("badge still fills its cell at %d items", n)
			}
			// The badge's rendered aspect should be close to square.
			dividers := float64(n-1) * h * agendaDividerHeightPct / 100
			gaps := float64(2*n-2) * agendaRowGapPt
			rowHeight := (h - dividers - gaps) / float64(n)
			badgeWidth := colWidth * pct / 100
			if aspect := badgeWidth / rowHeight; aspect < 0.9 || aspect > 1.1 {
				t.Errorf("%d items: badge is %.0fx%.0fpt, aspect %.2f — not square",
					n, badgeWidth, rowHeight, aspect)
			}
		})
	}
}

// More items means shorter rows means a narrower badge — the relationship has
// to be monotonic, or some count gets a slab again.
func TestAgendaBadgeNarrowsAsItemsGrow(t *testing.T) {
	ctx := ExpandContext{
		SlideWidth: 12192000, SlideHeight: 6858000,
		LayoutBounds: LayoutBounds{Width: 10515600, Height: 3913340},
	}
	prev := 101.0
	for n := 3; n <= 6; n++ {
		pct := agendaBadgeWidthPct(ctx, n)
		if pct >= prev {
			t.Errorf("%d items gives %.0f%%, not narrower than the %.0f%% before it", n, pct, prev)
		}
		prev = pct
	}
}

// A badge that fills its cell is emitted flat, with no pointless sub-grid.
func TestAgendaBadgeCellIsFlatWhenItFills(t *testing.T) {
	cell := buildAgendaBadgeCell("01", "accent1", 18, 100)
	if cell.Shape == nil || cell.Grid != nil {
		t.Errorf("a full-width badge should be the cell itself, got %+v", cell)
	}
	narrowed := buildAgendaBadgeCell("01", "accent1", 18, 60)
	if narrowed.Grid == nil {
		t.Fatal("a narrowed badge needs its centring sub-grid")
	}
	var cols []float64
	if err := json.Unmarshal(narrowed.Grid.Columns, &cols); err != nil {
		t.Fatalf("columns: %v", err)
	}
	if len(cols) != 3 || cols[1] != 60 || cols[0] != cols[2] {
		t.Errorf("columns = %v, want equal gutters around a 60%% badge", cols)
	}
}
