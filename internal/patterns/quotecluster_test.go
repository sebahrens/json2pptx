package patterns

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuoteCluster_Registration(t *testing.T) {
	p, ok := Default().Get("quote-cluster")
	if !ok {
		t.Fatal("expected quote-cluster to be registered in default registry")
	}
	if p.Name() != "quote-cluster" {
		t.Errorf("Name() = %q, want %q", p.Name(), "quote-cluster")
	}
	if p.Version() != 1 {
		t.Errorf("Version() = %d, want 1", p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" {
		t.Errorf("UseWhen()/NotWhen() must be non-empty")
	}
}

func TestQuoteCluster_Taxonomy(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" {
		t.Errorf("taxonomy fields must be populated: %+v", tx)
	}
	if len(tx.NarrativeRole) == 0 {
		t.Errorf("narrative role must have at least one value")
	}
}

func validQuoteClusterValues(n int) *QuoteClusterValues {
	pool := []QuoteClusterItem{
		{Text: "The new platform cut our cycle time in half.", Name: "J. Lin", Title: "Head of Operations"},
		{Text: "Our analysts finally trust the numbers they see.", Name: "P. Reyes", Title: "Director of Finance"},
		{Text: "Adoption was easier than any tool we have rolled out.", Name: "K. Müller", Title: "CTO"},
		{Text: "We are catching issues weeks earlier than before.", Name: "S. Patel", Title: "VP Customer Success"},
		{Text: "Reporting that used to take days is now a click.", Name: "M. Tanaka", Title: "Controller"},
		{Text: "The team's data fluency has stepped up.", Name: "A. Okafor", Title: "CDO"},
		{Text: "Approvals that languished now move within days.", Name: "L. Berg", Title: "Head of Procurement"},
		{Text: "We finally have a single source of truth.", Name: "T. Costa", Title: "VP Analytics"},
	}
	if n > len(pool) {
		n = len(pool)
	}
	out := make([]QuoteClusterItem, n)
	copy(out, pool[:n])
	return &QuoteClusterValues{Quotes: out}
}

func TestQuoteCluster_Validate_Valid(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	for _, n := range []int{3, 4, 5, 6, 7, 8} {
		if err := p.Validate(validQuoteClusterValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected validation error: %v", n, err)
		}
	}
}

func TestQuoteCluster_Validate_TooFew(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(2)
	err := p.Validate(v, nil, nil)
	if err == nil {
		t.Fatal("expected validation error for fewer than 3 quotes")
	}
	if !strings.Contains(err.Error(), "pull-quote") {
		t.Errorf("expected hint to mention pull-quote, got: %v", err)
	}
}

func TestQuoteCluster_Validate_TooMany(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(8)
	v.Quotes = append(v.Quotes, QuoteClusterItem{Text: "Ninth quote.", Name: "Extra"})
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for more than 8 quotes")
	}
}

func TestQuoteCluster_Validate_MissingText(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(3)
	v.Quotes[1].Text = "   "
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for blank text")
	}
}

func TestQuoteCluster_Validate_MissingName(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(3)
	v.Quotes[0].Name = ""
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for missing name")
	}
}

func TestQuoteCluster_Validate_TextTooLong(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(3)
	v.Quotes[0].Text = strings.Repeat("X", quoteClusterTextMax+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Fatal("expected validation error for text over char cap")
	}
}

func TestQuoteCluster_Expand_SixQuotes_TwoFullRows(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(6)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Fatalf("expected 2 rows for 6 quotes, got %d", len(grid.Rows))
	}
	for i, row := range grid.Rows {
		if len(row.Cells) != 3 {
			t.Errorf("row %d: expected 3 cells, got %d", i, len(row.Cells))
		}
		for j, cell := range row.Cells {
			if cell.Shape == nil {
				t.Errorf("row %d col %d: missing shape", i, j)
				continue
			}
			if len(cell.Shape.Text) == 0 {
				t.Errorf("row %d col %d: expected non-empty text", i, j)
			}
		}
	}
}

func TestQuoteCluster_Expand_EightQuotes_ThreeRowsLeftAligned(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(8)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Fatalf("expected 3 rows for 8 quotes, got %d", len(grid.Rows))
	}
	// Last row: cells 0 and 1 hold quotes, cell 2 is filler with no text.
	last := grid.Rows[2]
	if len(last.Cells) != 3 {
		t.Fatalf("last row: expected 3 cells, got %d", len(last.Cells))
	}
	if len(last.Cells[0].Shape.Text) == 0 || len(last.Cells[1].Shape.Text) == 0 {
		t.Errorf("last row cells 0/1 should hold quotes")
	}
	if len(last.Cells[2].Shape.Text) != 0 {
		t.Errorf("last row cell 2 should be a filler, got text: %s", string(last.Cells[2].Shape.Text))
	}
}

func TestQuoteCluster_Expand_FourQuotes_SecondRowSingleQuote(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(4)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 2 {
		t.Fatalf("expected 2 rows for 4 quotes, got %d", len(grid.Rows))
	}
	second := grid.Rows[1]
	if len(second.Cells) != 3 {
		t.Fatalf("second row: expected 3 cells (1 quote + 2 fillers), got %d", len(second.Cells))
	}
	if len(second.Cells[0].Shape.Text) == 0 {
		t.Errorf("second row cell 0 should hold the 4th quote")
	}
	for i := 1; i < 3; i++ {
		if len(second.Cells[i].Shape.Text) != 0 {
			t.Errorf("second row cell %d should be filler, got text", i)
		}
	}
}

func TestQuoteCluster_Expand_AlternatingFills(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(6)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// Even-indexed quotes use lt1 (subtle), odd-indexed use lt2 (paper). Verify
	// the fills alternate across the first row.
	row := grid.Rows[0]
	var fills []string
	for _, cell := range row.Cells {
		fills = append(fills, string(cell.Shape.Fill))
	}
	if fills[0] == fills[1] {
		t.Errorf("expected alternating fills, got: %v", fills)
	}
}

func TestQuoteCluster_Expand_QuoteUsesAccentColorForName(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(3)
	ovr := &QuoteClusterOverrides{Accent: "accent3"}
	grid, err := p.Expand(ExpandContext{}, v, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	var obj struct {
		Paragraphs []struct {
			Content string `json:"content"`
			Color   string `json:"color"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal cell text: %v", err)
	}
	// Paragraph 1 is the speaker name and must be accent3.
	if len(obj.Paragraphs) < 2 {
		t.Fatalf("expected at least 2 paragraphs (quote + name), got %d", len(obj.Paragraphs))
	}
	if obj.Paragraphs[1].Color != "accent3" {
		t.Errorf("name color = %q, want %q", obj.Paragraphs[1].Color, "accent3")
	}
}

func TestQuoteCluster_Expand_QuoteIsItalicized(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(3)
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	var obj struct {
		Paragraphs []struct {
			Content string `json:"content"`
			Italic  bool   `json:"italic"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal cell text: %v", err)
	}
	if len(obj.Paragraphs) == 0 || !obj.Paragraphs[0].Italic {
		t.Errorf("first paragraph (quote) must be italic, got: %+v", obj.Paragraphs)
	}
	if !strings.Contains(obj.Paragraphs[0].Content, "“") || !strings.Contains(obj.Paragraphs[0].Content, "”") {
		t.Errorf("quote content should be wrapped in typographic quotes, got: %q", obj.Paragraphs[0].Content)
	}
}

func TestQuoteCluster_Expand_OmittedTitleDropsParagraph(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := &QuoteClusterValues{
		Quotes: []QuoteClusterItem{
			{Text: "One.", Name: "A. Solo"},
			{Text: "Two.", Name: "B. Solo"},
			{Text: "Three.", Name: "C. Solo"},
		},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	var obj struct {
		Paragraphs []struct {
			Content string `json:"content"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &obj); err != nil {
		t.Fatalf("unmarshal cell text: %v", err)
	}
	if len(obj.Paragraphs) != 2 {
		t.Errorf("expected exactly 2 paragraphs (quote + name) when title omitted, got %d", len(obj.Paragraphs))
	}
}

func TestQuoteCluster_Expand_AppliesCellOverride(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	v := validQuoteClusterValues(3)
	co := map[int]any{
		1: &QuoteClusterCellOverride{AccentBar: true},
	}
	grid, err := p.Expand(ExpandContext{}, v, nil, co)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	cell := grid.Rows[0].Cells[1]
	if cell.AccentBar == nil {
		t.Fatal("expected accent bar on quote index 1")
	}
	if cell.AccentBar.Position != "left" {
		t.Errorf("accent bar position = %q, want %q", cell.AccentBar.Position, "left")
	}
}

func TestQuoteCluster_Schema_Valid(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
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

func TestQuoteCluster_ExemplarValues_ExpandsCleanly(t *testing.T) {
	p, _ := Default().Get("quote-cluster")
	ex, ok := p.(Exemplar)
	if !ok {
		t.Fatal("quote-cluster does not implement Exemplar")
	}
	vals := ex.ExemplarValues()
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("exemplar values failed validation: %v", err)
	}
	if _, err := p.Expand(ExpandContext{}, vals, nil, nil); err != nil {
		t.Fatalf("exemplar Expand: %v", err)
	}
}
