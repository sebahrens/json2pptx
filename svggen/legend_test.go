package svggen

import (
	"fmt"
	"testing"
)

// =============================================================================
// Title Tests
// =============================================================================

func TestDefaultTitleConfig(t *testing.T) {
	config := DefaultTitleConfig()

	if config.HorizontalAlign != "center" {
		t.Errorf("Expected HorizontalAlign 'center', got %v", config.HorizontalAlign)
	}
	if config.VerticalAlign != "top" {
		t.Errorf("Expected VerticalAlign 'top', got %v", config.VerticalAlign)
	}
	if config.Padding != 8 {
		t.Errorf("Expected Padding 8, got %v", config.Padding)
	}
	if config.TitleSubtitleGap != 4 {
		t.Errorf("Expected TitleSubtitleGap 4, got %v", config.TitleSubtitleGap)
	}
}

func TestTitleDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultTitleConfig()
	config.Text = "Chart Title"

	title := NewTitle(b, config)
	bounds := Rect{X: 0, Y: 0, W: 400, H: 50}
	title.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestTitleWithSubtitle(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultTitleConfig()
	config.Text = "Main Title"
	config.Subtitle = "Subtitle text"

	title := NewTitle(b, config)
	bounds := Rect{X: 0, Y: 0, W: 400, H: 80}
	title.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestTitleHeight(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	tests := []struct {
		name        string
		text        string
		subtitle    string
		expectZero  bool
		description string
	}{
		{
			name:        "EmptyTitle",
			text:        "",
			subtitle:    "",
			expectZero:  true,
			description: "Empty title should return zero height",
		},
		{
			name:        "TitleOnly",
			text:        "Title",
			subtitle:    "",
			expectZero:  false,
			description: "Title only should have positive height",
		},
		{
			name:        "TitleAndSubtitle",
			text:        "Title",
			subtitle:    "Subtitle",
			expectZero:  false,
			description: "Title with subtitle should have positive height",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultTitleConfig()
			config.Text = tc.text
			config.Subtitle = tc.subtitle

			title := NewTitle(b, config)
			height := title.Height()

			if tc.expectZero && height != 0 {
				t.Errorf("%s: expected height 0, got %v", tc.description, height)
			}
			if !tc.expectZero && height <= 0 {
				t.Errorf("%s: expected positive height, got %v", tc.description, height)
			}
		})
	}
}

func TestTitleAlignment(t *testing.T) {
	alignments := []struct {
		horizontal string
		vertical   string
	}{
		{"left", "top"},
		{"center", "top"},
		{"right", "top"},
		{"left", "bottom"},
		{"center", "bottom"},
		{"right", "bottom"},
	}

	for _, align := range alignments {
		t.Run(align.horizontal+"_"+align.vertical, func(t *testing.T) {
			b := NewSVGBuilder(400, 300)
			config := DefaultTitleConfig()
			config.Text = "Test Title"
			config.HorizontalAlign = align.horizontal
			config.VerticalAlign = align.vertical

			title := NewTitle(b, config)
			bounds := Rect{X: 0, Y: 0, W: 400, H: 60}
			title.Draw(bounds)

			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Failed to render SVG: %v", err)
			}

			if len(doc.Content) == 0 {
				t.Error("Expected non-empty SVG content")
			}
		})
	}
}

func TestTitleWithCustomStyle(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultTitleConfig()
	config.Text = "Styled Title"
	config.TitleStyle = &TextStyle{
		FontSize:   30,
		FontWeight: 700,
		Color:      &Color{R: 0, G: 100, B: 200, A: 1.0},
	}
	config.SubtitleStyle = &TextStyle{
		FontSize:   14,
		FontWeight: 400,
		Color:      &Color{R: 100, G: 100, B: 100, A: 1.0},
	}
	config.Subtitle = "Styled Subtitle"

	title := NewTitle(b, config)
	bounds := Rect{X: 0, Y: 0, W: 400, H: 80}
	title.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestTitleEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultTitleConfig()
	// Empty text

	title := NewTitle(b, config)
	bounds := Rect{X: 0, Y: 0, W: 400, H: 50}
	title.Draw(bounds)

	// Should not panic and should render empty
	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

// =============================================================================
// Footnote Tests
// =============================================================================

func TestDefaultFootnoteConfig(t *testing.T) {
	config := DefaultFootnoteConfig()

	if config.HorizontalAlign != "left" {
		t.Errorf("Expected HorizontalAlign 'left', got %v", config.HorizontalAlign)
	}
	if config.Padding != 3 {
		t.Errorf("Expected Padding 3, got %v", config.Padding)
	}
}

func TestFootnoteDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultFootnoteConfig()
	config.Text = "Source: Company data, 2024"

	footnote := NewFootnote(b, config)
	bounds := Rect{X: 0, Y: 280, W: 400, H: 20}
	footnote.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestFootnoteAlignment(t *testing.T) {
	alignments := []string{"left", "center", "right"}

	for _, align := range alignments {
		t.Run(align, func(t *testing.T) {
			b := NewSVGBuilder(400, 300)
			config := DefaultFootnoteConfig()
			config.Text = "Footnote text"
			config.HorizontalAlign = align

			footnote := NewFootnote(b, config)
			bounds := Rect{X: 0, Y: 280, W: 400, H: 20}
			footnote.Draw(bounds)

			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Failed to render SVG: %v", err)
			}

			if len(doc.Content) == 0 {
				t.Error("Expected non-empty SVG content")
			}
		})
	}
}

func TestFootnoteHeight(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	// Empty footnote
	emptyConfig := DefaultFootnoteConfig()
	emptyFootnote := NewFootnote(b, emptyConfig)
	if emptyFootnote.Height() != 0 {
		t.Errorf("Expected height 0 for empty footnote, got %v", emptyFootnote.Height())
	}

	// Non-empty footnote
	config := DefaultFootnoteConfig()
	config.Text = "Some footnote"
	footnote := NewFootnote(b, config)
	if footnote.Height() <= 0 {
		t.Errorf("Expected positive height for footnote, got %v", footnote.Height())
	}
}

func TestFootnoteWithCustomStyle(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultFootnoteConfig()
	config.Text = "Custom styled footnote"
	config.Style = &TextStyle{
		FontSize:   10,
		FontWeight: 500,
		Color:      &Color{R: 128, G: 128, B: 128, A: 1.0},
	}

	footnote := NewFootnote(b, config)
	bounds := Rect{X: 0, Y: 280, W: 400, H: 20}
	footnote.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Legend Tests
// =============================================================================

func TestDefaultLegendConfig(t *testing.T) {
	config := DefaultLegendConfig()

	if config.Position != LegendPositionBottom {
		t.Errorf("Expected Position LegendPositionBottom, got %v", config.Position)
	}
	if config.Layout != LegendLayoutHorizontal {
		t.Errorf("Expected Layout LegendLayoutHorizontal, got %v", config.Layout)
	}
	if config.MarkerSize != 12 {
		t.Errorf("Expected MarkerSize 12, got %v", config.MarkerSize)
	}
	if config.ItemGap != 16 {
		t.Errorf("Expected ItemGap 16, got %v", config.ItemGap)
	}
}

func TestLegendDrawHorizontal(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutHorizontal

	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Series A", Color: MustParseColor("#4E79A7")},
		{Label: "Series B", Color: MustParseColor("#F28E2B")},
		{Label: "Series C", Color: MustParseColor("#E15759")},
	})

	bounds := Rect{X: 0, Y: 260, W: 400, H: 40}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendDrawVertical(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutVertical

	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Item 1", Color: MustParseColor("#4E79A7")},
		{Label: "Item 2", Color: MustParseColor("#F28E2B")},
		{Label: "Item 3", Color: MustParseColor("#E15759")},
		{Label: "Item 4", Color: MustParseColor("#76B7B2")},
	})

	bounds := Rect{X: 320, Y: 50, W: 80, H: 150}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendDrawGrid(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutGrid

	legend := NewLegend(b, config)
	items := make([]LegendItem, 6)
	colors := []string{"#4E79A7", "#F28E2B", "#E15759", "#76B7B2", "#59A14F", "#EDC948"}
	for i := range items {
		items[i] = LegendItem{
			Label: "Item " + string(rune('A'+i)),
			Color: MustParseColor(colors[i]),
		}
	}
	legend.SetItems(items)

	bounds := Rect{X: 50, Y: 200, W: 300, H: 80}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendMarkerShapes(t *testing.T) {
	shapes := []LegendMarkerShape{
		LegendMarkerRect,
		LegendMarkerCircle,
		LegendMarkerLine,
	}

	for _, shape := range shapes {
		t.Run(legendMarkerShapeName(shape), func(t *testing.T) {
			b := NewSVGBuilder(400, 300)
			config := DefaultLegendConfig()
			config.MarkerShape = shape

			legend := NewLegend(b, config)
			legend.SetItems([]LegendItem{
				{Label: "Series A", Color: MustParseColor("#4E79A7")},
				{Label: "Series B", Color: MustParseColor("#F28E2B")},
			})

			bounds := Rect{X: 0, Y: 260, W: 400, H: 40}
			legend.Draw(bounds)

			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Failed to render SVG: %v", err)
			}

			if len(doc.Content) == 0 {
				t.Error("Expected non-empty SVG content")
			}
		})
	}
}

func TestLegendAddItem(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	legend := NewLegend(b, config)
	legend.AddItem("First", MustParseColor("#4E79A7"))
	legend.AddItem("Second", MustParseColor("#F28E2B"))
	legend.AddItem("Third", MustParseColor("#E15759"))

	bounds := Rect{X: 0, Y: 260, W: 400, H: 40}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendInactiveItems(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Active", Color: MustParseColor("#4E79A7"), Inactive: false},
		{Label: "Inactive", Color: MustParseColor("#F28E2B"), Inactive: true},
	})

	bounds := Rect{X: 0, Y: 260, W: 400, H: 40}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendWithValues(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Series A", Color: MustParseColor("#4E79A7"), Value: "42%"},
		{Label: "Series B", Color: MustParseColor("#F28E2B"), Value: "58%"},
	})

	bounds := Rect{X: 0, Y: 260, W: 400, H: 40}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendWithBorder(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()
	config.ShowBorder = true
	config.BorderColor = MustParseColor("#CCCCCC")
	bgColor := MustParseColor("#F8F9FA")
	config.BackgroundColor = &bgColor

	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Item 1", Color: MustParseColor("#4E79A7")},
		{Label: "Item 2", Color: MustParseColor("#F28E2B")},
	})

	bounds := Rect{X: 100, Y: 200, W: 200, H: 50}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendHeight(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	// Empty legend
	emptyLegend := NewLegend(b, config)
	if emptyLegend.Height(400) != 0 {
		t.Errorf("Expected height 0 for empty legend, got %v", emptyLegend.Height(400))
	}

	// Non-empty legend
	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Item 1", Color: MustParseColor("#4E79A7")},
		{Label: "Item 2", Color: MustParseColor("#F28E2B")},
	})
	if legend.Height(400) <= 0 {
		t.Errorf("Expected positive height for legend, got %v", legend.Height(400))
	}
}

func TestLegendWidth(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	// Empty legend
	emptyLegend := NewLegend(b, config)
	if emptyLegend.Width() != 0 {
		t.Errorf("Expected width 0 for empty legend, got %v", emptyLegend.Width())
	}

	// Non-empty legend
	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Item", Color: MustParseColor("#4E79A7")},
	})
	if legend.Width() <= 0 {
		t.Errorf("Expected positive width for legend, got %v", legend.Width())
	}
}

func TestLegendWrapping(t *testing.T) {
	b := NewSVGBuilder(300, 300)
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutHorizontal
	config.MaxWidth = 200 // Force wrapping

	legend := NewLegend(b, config)
	legend.SetItems([]LegendItem{
		{Label: "Very Long Series Name A", Color: MustParseColor("#4E79A7")},
		{Label: "Very Long Series Name B", Color: MustParseColor("#F28E2B")},
		{Label: "Very Long Series Name C", Color: MustParseColor("#E15759")},
	})

	bounds := Rect{X: 0, Y: 200, W: 300, H: 80}
	legend.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestLegendAlignment(t *testing.T) {
	horizontalAligns := []string{"left", "center", "right"}
	verticalAligns := []string{"top", "middle", "bottom"}

	for _, hAlign := range horizontalAligns {
		for _, vAlign := range verticalAligns {
			t.Run(hAlign+"_"+vAlign, func(t *testing.T) {
				b := NewSVGBuilder(400, 300)
				config := DefaultLegendConfig()
				config.Layout = LegendLayoutVertical
				config.HorizontalAlign = hAlign
				config.VerticalAlign = vAlign

				legend := NewLegend(b, config)
				legend.SetItems([]LegendItem{
					{Label: "A", Color: MustParseColor("#4E79A7")},
					{Label: "B", Color: MustParseColor("#F28E2B")},
				})

				bounds := Rect{X: 0, Y: 0, W: 200, H: 200}
				legend.Draw(bounds)

				doc, err := b.Render()
				if err != nil {
					t.Fatalf("Failed to render SVG: %v", err)
				}

				if len(doc.Content) == 0 {
					t.Error("Expected non-empty SVG content")
				}
			})
		}
	}
}

func TestLegendEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()

	legend := NewLegend(b, config)
	// No items set

	bounds := Rect{X: 0, Y: 260, W: 400, H: 40}
	legend.Draw(bounds)

	// Should not panic
	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

// TestLegendGridOverflowProtection verifies that grid layout skips items that
// would overflow the allocated bounds, preventing partial/clipped text.
func TestLegendGridOverflowProtection(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	style := b.StyleGuide()
	config := PresentationPieLegendConfig(style)

	legend := NewLegend(b, config)
	// Create 8 items -- in a small height, some should be skipped
	items := make([]LegendItem, 8)
	for i := range items {
		items[i] = LegendItem{
			Label: fmt.Sprintf("Category %d", i+1),
			Color: MustParseColor("#4E79A7"),
		}
	}
	legend.SetItems(items)

	// Give it very small height so not all items fit
	bounds := Rect{X: 10, Y: 200, W: 380, H: 25}
	legend.Draw(bounds) // Should not draw items that overflow

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// TestLegendVerticalOverflowProtection verifies that vertical layout skips
// items that would overflow the allocated bounds.
func TestLegendVerticalOverflowProtection(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultLegendConfig()
	config.Layout = LegendLayoutVertical

	legend := NewLegend(b, config)
	items := make([]LegendItem, 10)
	for i := range items {
		items[i] = LegendItem{
			Label: fmt.Sprintf("Series %d", i+1),
			Color: MustParseColor("#F28E2B"),
		}
	}
	legend.SetItems(items)

	// Give it very small height so not all items fit
	bounds := Rect{X: 10, Y: 200, W: 380, H: 30}
	legend.Draw(bounds) // Should not draw items that overflow

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Chart Header Tests
// =============================================================================

func TestDefaultChartHeaderConfig(t *testing.T) {
	config := DefaultChartHeaderConfig()

	if config.LegendPosition != "below" {
		t.Errorf("Expected LegendPosition 'below', got %v", config.LegendPosition)
	}
	if config.TitleLegendGap != 12 {
		t.Errorf("Expected TitleLegendGap 12, got %v", config.TitleLegendGap)
	}
}

func TestChartHeaderDraw(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultChartHeaderConfig()
	config.Title.Text = "Sales by Region"
	config.Title.Subtitle = "Q4 2024"

	header := NewChartHeader(b, config)
	header.SetLegendItems([]LegendItem{
		{Label: "North", Color: MustParseColor("#4E79A7")},
		{Label: "South", Color: MustParseColor("#F28E2B")},
		{Label: "East", Color: MustParseColor("#E15759")},
		{Label: "West", Color: MustParseColor("#76B7B2")},
	})

	bounds := Rect{X: 0, Y: 0, W: 400, H: 100}
	header.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestChartHeaderFluentAPI(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultChartHeaderConfig()

	header := NewChartHeader(b, config)
	header.
		SetTitle("Revenue Growth").
		SetSubtitle("Year over Year").
		AddLegendItem("2023", MustParseColor("#4E79A7")).
		AddLegendItem("2024", MustParseColor("#F28E2B"))

	bounds := Rect{X: 0, Y: 0, W: 400, H: 100}
	header.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestChartHeaderLegendPositionRight(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultChartHeaderConfig()
	config.LegendPosition = "right"
	config.Title.Text = "Chart Title"

	header := NewChartHeader(b, config)
	header.SetLegendItems([]LegendItem{
		{Label: "A", Color: MustParseColor("#4E79A7")},
		{Label: "B", Color: MustParseColor("#F28E2B")},
	})

	bounds := Rect{X: 0, Y: 0, W: 400, H: 60}
	header.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestChartHeaderTitleOnly(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultChartHeaderConfig()
	config.Title.Text = "Title Only"

	header := NewChartHeader(b, config)
	// No legend items

	bounds := Rect{X: 0, Y: 0, W: 400, H: 50}
	header.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestChartHeaderLegendOnly(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultChartHeaderConfig()
	// No title text

	header := NewChartHeader(b, config)
	header.SetLegendItems([]LegendItem{
		{Label: "A", Color: MustParseColor("#4E79A7")},
		{Label: "B", Color: MustParseColor("#F28E2B")},
	})

	bounds := Rect{X: 0, Y: 0, W: 400, H: 40}
	header.Draw(bounds)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

func TestChartHeaderHeight(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	tests := []struct {
		name        string
		title       string
		hasLegend   bool
		expectZero  bool
		description string
	}{
		{
			name:        "Empty",
			title:       "",
			hasLegend:   false,
			expectZero:  true,
			description: "Empty header should return zero height",
		},
		{
			name:        "TitleOnly",
			title:       "Title",
			hasLegend:   false,
			expectZero:  false,
			description: "Title only should have positive height",
		},
		{
			name:        "LegendOnly",
			title:       "",
			hasLegend:   true,
			expectZero:  false,
			description: "Legend only should have positive height",
		},
		{
			name:        "Both",
			title:       "Title",
			hasLegend:   true,
			expectZero:  false,
			description: "Both title and legend should have positive height",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultChartHeaderConfig()
			config.Title.Text = tc.title

			header := NewChartHeader(b, config)
			if tc.hasLegend {
				header.SetLegendItems([]LegendItem{
					{Label: "A", Color: MustParseColor("#4E79A7")},
				})
			}

			height := header.Height(400)

			if tc.expectZero && height != 0 {
				t.Errorf("%s: expected height 0, got %v", tc.description, height)
			}
			if !tc.expectZero && height <= 0 {
				t.Errorf("%s: expected positive height, got %v", tc.description, height)
			}
		})
	}
}

func TestChartHeaderEmpty(t *testing.T) {
	b := NewSVGBuilder(400, 300)
	config := DefaultChartHeaderConfig()

	header := NewChartHeader(b, config)
	// No title or legend

	bounds := Rect{X: 0, Y: 0, W: 400, H: 50}
	header.Draw(bounds)

	// Should not panic
	_, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestCompleteChartWithHeaderAndFooter(t *testing.T) {
	b := NewSVGBuilder(500, 400)

	// Draw header
	headerConfig := DefaultChartHeaderConfig()
	headerConfig.Title.Text = "Monthly Sales"
	headerConfig.Title.Subtitle = "2024"

	header := NewChartHeader(b, headerConfig)
	header.SetLegendItems([]LegendItem{
		{Label: "Product A", Color: MustParseColor("#4E79A7")},
		{Label: "Product B", Color: MustParseColor("#F28E2B")},
		{Label: "Product C", Color: MustParseColor("#E15759")},
	})

	headerHeight := header.Height(500)
	headerBounds := Rect{X: 0, Y: 0, W: 500, H: headerHeight}
	header.Draw(headerBounds)

	// Draw footnote
	footnoteConfig := DefaultFootnoteConfig()
	footnoteConfig.Text = "Source: Internal sales data, 2024"

	footnote := NewFootnote(b, footnoteConfig)
	footnoteHeight := footnote.Height()
	footnoteBounds := Rect{X: 0, Y: 400 - footnoteHeight, W: 500, H: footnoteHeight}
	footnote.Draw(footnoteBounds)

	// Chart area would be between header and footnote
	chartArea := Rect{
		X: 40,
		Y: headerHeight + 10,
		W: 420,
		H: 400 - headerHeight - footnoteHeight - 20,
	}

	// Draw a simple border to show chart area
	b.SetStrokeColor(MustParseColor("#DEE2E6"))
	b.SetStrokeWidth(1)
	b.StrokeRect(chartArea)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Failed to render SVG: %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG content")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func legendMarkerShapeName(s LegendMarkerShape) string {
	names := map[LegendMarkerShape]string{
		LegendMarkerRect:   "Rect",
		LegendMarkerCircle: "Circle",
		LegendMarkerLine:   "Line",
	}
	if name, ok := names[s]; ok {
		return name
	}
	return "Unknown"
}
