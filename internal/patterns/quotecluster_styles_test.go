package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

type quoteParas struct {
	Paragraphs []struct {
		Content string  `json:"content"`
		Color   string  `json:"color"`
		Italic  bool    `json:"italic"`
		Alpha   float64 `json:"alpha"`
		Size    float64 `json:"size"`
	} `json:"paragraphs"`
	VerticalAlign string `json:"vertical_align"`
}

func quoteCellParas(t *testing.T, cell *jsonschema.GridCellInput) quoteParas {
	t.Helper()
	var obj quoteParas
	if err := json.Unmarshal(cell.Shape.Text, &obj); err != nil {
		t.Fatalf("cell text: %v", err)
	}
	return obj
}

// quoteOpenRow is one row of open quotes: the quote cells and the rule and
// attribution cells under them.
type quoteOpenRow struct{ quotes, attributions []*jsonschema.GridCellInput }

// quoteOpenRows returns the open cluster's rows of quotes, skipping the
// whitespace band between two of them.
func quoteOpenRows(t *testing.T, grid *jsonschema.ShapeGridInput) []quoteOpenRow {
	t.Helper()
	var rows []quoteOpenRow
	var content []jsonschema.GridRowInput
	for _, row := range grid.Rows {
		if len(row.Cells) == quoteClusterColumns {
			content = append(content, row)
		}
	}
	if len(content)%2 != 0 {
		t.Fatalf("open cluster has %d content rows; want quote and attribution rows in pairs", len(content))
	}
	for i := 0; i < len(content); i += 2 {
		rows = append(rows, quoteOpenRow{quotes: content[i].Cells, attributions: content[i+1].Cells})
	}
	return rows
}

// go-slide-creator-5cie9, -rxdkf: the default quote is open text under a
// large accent quote mark, with its attribution on its own line under a short
// rule, and no tile.
func TestQuoteClusterOpenDefault(t *testing.T) {
	p := &quoteCluster{}
	for _, n := range []int{3, 6, 8} {
		grid, err := p.Expand(testThemeCtx(), validQuoteClusterValues(n), nil, nil)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		for ri, row := range grid.Rows {
			if row.MaxHeight <= 0 {
				t.Errorf("n=%d row %d is not content-sized", n, ri)
			}
		}
		seen := 0
		for ri, row := range quoteOpenRows(t, grid) {
			for ci, cell := range row.quotes {
				if len(cell.Shape.Text) == 0 {
					continue
				}
				seen++
				attribution := row.attributions[ci]
				for _, c := range []*jsonschema.GridCellInput{cell, attribution} {
					if string(c.Shape.Fill) != `"none"` || string(c.Shape.Line) != `"none"` || c.Shape.Geometry != "rect" {
						t.Errorf("n=%d row %d col %d: an open quote has no container, got fill %s line %s", n, ri, ci, c.Shape.Fill, c.Shape.Line)
					}
				}
				obj := quoteCellParas(t, cell)
				if len(obj.Paragraphs) != 2 || obj.Paragraphs[0].Content != quoteClusterMark || !obj.Paragraphs[1].Italic {
					t.Fatalf("n=%d: want mark and italic quote; got %+v", n, obj.Paragraphs)
				}
				if obj.Paragraphs[0].Color != "accent1" || obj.Paragraphs[1].Color != "dk1" {
					t.Errorf("n=%d: the mark alone carries the accent, got %+v", n, obj.Paragraphs)
				}
				if obj.Paragraphs[0].Size <= obj.Paragraphs[1].Size {
					t.Errorf("n=%d: the mark is display type over the quote, got %vpt over %vpt", n, obj.Paragraphs[0].Size, obj.Paragraphs[1].Size)
				}
				if obj.VerticalAlign != "t" {
					t.Errorf("n=%d: open quotes are top-anchored so the marks line up", n)
				}
				attr := quoteCellParas(t, attribution)
				if len(attr.Paragraphs) != 2 || attr.Paragraphs[0].Content != quoteClusterOpenRule {
					t.Fatalf("n=%d: want a rule and the attribution under it; got %+v", n, attr.Paragraphs)
				}
				if !strings.HasPrefix(attr.Paragraphs[1].Content, "<b>") || !strings.Contains(attr.Paragraphs[1].Content, "</b>, ") {
					t.Errorf("n=%d: attribution should be the bold name then the title, got %q", n, attr.Paragraphs[1].Content)
				}
				if attr.Paragraphs[1].Color != "dk1" || attr.Paragraphs[1].Size < scaleBodyPt {
					t.Errorf("n=%d: attribution is neutral ink at the body size or more, got %+v", n, attr.Paragraphs[1])
				}
			}
		}
		if seen != n {
			t.Errorf("n=%d: %d quotes rendered", n, seen)
		}
	}
	// With room to spare the quotes are set a step above the body size under
	// a 36pt mark.
	grid, err := p.Expand(testThemeCtx(), validQuoteClusterValues(6), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if obj := quoteCellParas(t, quoteOpenRows(t, grid)[0].quotes[0]); obj.Paragraphs[0].Size != sizeQuotePt || obj.Paragraphs[1].Size != scaleSubheadPt {
		t.Errorf("six short quotes: want a %vpt mark over %vpt quotes, got %+v", sizeQuotePt, scaleSubheadPt, obj.Paragraphs)
	}
}

// One highlighted quote takes the cluster's one tint panel (its quote and
// attribution cells, lapped so they read as one) and keeps the only accent
// mark; the other marks go neutral.
func TestQuoteClusterHighlightIsTheOnlyAccent(t *testing.T) {
	p := &quoteCluster{}
	v := validQuoteClusterValues(6)
	v.Quotes[4].Highlight = true
	if err := p.Validate(v, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	grid, err := p.Expand(testThemeCtx(), v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for ri, row := range quoteOpenRows(t, grid) {
		for ci, cell := range row.quotes {
			idx := ri*quoteClusterColumns + ci
			attribution := row.attributions[ci]
			obj := quoteCellParas(t, cell)
			filled := string(cell.Shape.Fill) != `"none"`
			if filled != (idx == 4) || string(attribution.Shape.Fill) != string(cell.Shape.Fill) {
				t.Errorf("quote %d filled = %t (attribution fill %s)", idx, filled, attribution.Shape.Fill)
			}
			if lapped := attribution.BleedTop > 0; lapped != (idx == 4) {
				t.Errorf("quote %d: attribution laps its quote = %t", idx, lapped)
			}
			mark := obj.Paragraphs[0]
			if idx == 4 && mark.Alpha != 0 {
				t.Errorf("highlighted mark should stay opaque, got %+v", mark)
			}
			if idx != 4 && (mark.Color != "dk1" || mark.Alpha == 0) {
				t.Errorf("quote %d mark should be a dimmed neutral, got %+v", idx, mark)
			}
			if (strings.Contains(string(cell.Shape.Text), `"accent`) || strings.Contains(string(attribution.Shape.Text), `"accent`)) && idx != 4 {
				t.Errorf("quote %d carries an accent although quote 4 is highlighted", idx)
			}
		}
	}

	v.Quotes[1].Highlight = true
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "at most one quote") {
		t.Errorf("two highlights must be refused, got %v", err)
	}
}

// The bubble style stacks a speech-bubble shape over its attribution; 7–8
// bubbles take four columns so the cluster stays on two rows.
func TestQuoteClusterBubbleStyle(t *testing.T) {
	p := &quoteCluster{}
	ovr := &QuoteClusterOverrides{Style: "bubble"}
	for _, tc := range []struct{ n, cols, rows int }{{3, 3, 1}, {6, 3, 2}, {7, 4, 2}, {8, 4, 2}} {
		v := validQuoteClusterValues(tc.n)
		v.Quotes[0].Highlight = true
		grid, err := p.Expand(testThemeCtx(), v, ovr, nil)
		if err != nil {
			t.Fatalf("n=%d: %v", tc.n, err)
		}
		if string(grid.Columns) != string(rune('0'+tc.cols)) || len(grid.Rows) != tc.rows {
			t.Errorf("n=%d: %s columns x %d rows, want %d x %d", tc.n, grid.Columns, len(grid.Rows), tc.cols, tc.rows)
		}
		stack := grid.Rows[0].Cells[0].Grid
		if stack == nil || len(stack.Rows) != 2 {
			t.Fatalf("n=%d: a bubble cell is a bubble row over an attribution row", tc.n)
		}
		bubble, attribution := stack.Rows[0].Cells[0].Shape, stack.Rows[1].Cells[0].Shape
		if bubble.Geometry != "wedgeRoundRectCallout" || bubble.Adjustments["adj2"] <= 50000 {
			t.Errorf("n=%d: bubble geometry %q adj %v: the tail must reach below the body", tc.n, bubble.Geometry, bubble.Adjustments)
		}
		if string(bubble.Fill) != `"accent1"` && !strings.Contains(string(bubble.Fill), "accent1") {
			t.Errorf("n=%d: highlighted bubble fill = %s, want the accent", tc.n, bubble.Fill)
		}
		if string(attribution.Fill) != `"none"` || !strings.Contains(string(attribution.Text), v.Quotes[0].Name) {
			t.Errorf("n=%d: attribution under the bubble is unfilled text naming the speaker", tc.n)
		}
		other := grid.Rows[0].Cells[1].Grid.Rows[0].Cells[0].Shape
		if strings.Contains(string(other.Fill), "accent") {
			t.Errorf("n=%d: only the highlighted bubble takes the accent, got %s", tc.n, other.Fill)
		}
		want := stack.Rows[0].MaxHeight + stack.RowGap + stack.Rows[1].MaxHeight + 2*SubGridInsetPt
		if grid.Rows[0].MaxHeight != want {
			t.Errorf("n=%d: row height %.0f, want bubble + tail + attribution + sub-grid inset = %.0f", tc.n, grid.Rows[0].MaxHeight, want)
		}
	}
}

func TestQuoteClusterStyleValidated(t *testing.T) {
	err := (&quoteCluster{}).Validate(validQuoteClusterValues(3), &QuoteClusterOverrides{Style: "card"}, nil)
	if err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Fatalf("want an overrides.style enum error, got %v", err)
	}
}

// A highlighted tile is the one solid accent tile.
func TestQuoteClusterTileHighlight(t *testing.T) {
	v := validQuoteClusterValues(3)
	v.Quotes[2].Highlight = true
	grid, err := (&quoteCluster{}).Expand(testThemeCtx(), v, &QuoteClusterOverrides{Style: "tile"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, cell := range grid.Rows[0].Cells {
		if got := strings.Contains(string(cell.Shape.Fill), "accent1"); got != (i == 2) {
			t.Errorf("tile %d accent fill = %t", i, got)
		}
	}
}
