package patterns

import (
	"strings"
	"testing"
)

// go-slide-creator-rv9fe: a described agenda keeps the numbered list and its
// current-section highlight; each subtitle is a smaller muted line under its
// title, dimmed with its row.
func TestAgendaSubtitlesRenderUnderTheirTitles(t *testing.T) {
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: int64(796 * 12700), Height: int64(330 * 12700)}
	v := &AgendaValues{
		Items:     []string{"Where we are", "What we found", "What we recommend", "What happens next"},
		Subtitles: []string{"Q3 against the plan", "Three findings", "The decision", "The first ninety days"},
	}
	p := &agenda{}
	if err := p.Validate(v, &AgendaOverrides{Highlight: 2}, nil); err != nil {
		t.Fatal(err)
	}
	if w := p.PostExpandWarnings(ctx, v, &AgendaOverrides{Highlight: 2}); len(w) != 0 {
		t.Fatalf("warnings: %v", w)
	}
	grid, err := p.Expand(ctx, v, &AgendaOverrides{Highlight: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var rows int
	for _, row := range grid.Rows {
		if len(row.Cells) != 2 {
			continue // rules
		}
		txt := cellText(t, row.Cells[1].Shape.Text)
		if len(txt.Paragraphs) != 2 {
			t.Fatalf("row %d paragraphs = %+v, want title + subtitle", rows, txt.Paragraphs)
		}
		title, sub := txt.Paragraphs[0], txt.Paragraphs[1]
		if sub.Content != v.Subtitles[rows] || sub.Size >= title.Size || sub.Size < 12 {
			t.Errorf("row %d subtitle = %+v under title %+v", rows, sub, title)
		}
		if strings.Contains(sub.Content, "we are here") {
			t.Errorf("literal marker in row %d", rows)
		}
		rows++
	}
	if rows != 4 {
		t.Fatalf("rows = %d", rows)
	}
	if err := p.Validate(&AgendaValues{Items: []string{"a", "b"}, Subtitles: []string{"x", "y", "z"}}, nil, nil); err == nil {
		t.Error("more subtitles than items should be refused")
	}
}

// Subtitles the area cannot hold at a readable size are left off with a
// warning rather than shrinking every title below the floor.
func TestAgendaDropsSubtitlesThatDoNotFit(t *testing.T) {
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: int64(687 * 12700), Height: int64(294 * 12700)}
	v := &AgendaValues{}
	for i := 0; i < 10; i++ {
		v.Items = append(v.Items, "Section title")
		v.Subtitles = append(v.Subtitles, strings.Repeat("A long description of the section ", 3))
	}
	w := (&agenda{}).PostExpandWarnings(ctx, v, nil)
	if len(w) == 0 || !strings.Contains(w[0], "subtitles do not fit") {
		t.Fatalf("warnings = %v", w)
	}
	grid, err := (&agenda{}).Expand(ctx, v, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if txt := cellText(t, grid.Rows[0].Cells[1].Shape.Text); len(txt.Paragraphs) != 1 {
		t.Errorf("subtitles kept: %+v", txt.Paragraphs)
	}
}
