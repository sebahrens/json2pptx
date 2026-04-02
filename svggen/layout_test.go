package svggen

import (
	"strings"
	"testing"
)

func TestTextLine(t *testing.T) {
	line := TextLine{Text: "Hello", Width: 50, Height: 12}
	if line.Text != "Hello" {
		t.Errorf("TextLine.Text = %q, want %q", line.Text, "Hello")
	}
	if line.Width != 50 {
		t.Errorf("TextLine.Width = %v, want 50", line.Width)
	}
	if line.Height != 12 {
		t.Errorf("TextLine.Height = %v, want 12", line.Height)
	}
}

func TestTextBlock(t *testing.T) {
	block := TextBlock{
		Lines: []TextLine{
			{Text: "Line 1", Width: 60, Height: 12},
			{Text: "Line 2", Width: 40, Height: 12},
		},
		TotalWidth:  60,
		LineHeight:  12,
		TotalHeight: 24,
	}

	if len(block.Lines) != 2 {
		t.Errorf("TextBlock.Lines length = %d, want 2", len(block.Lines))
	}
	if block.TotalWidth != 60 {
		t.Errorf("TextBlock.TotalWidth = %v, want 60", block.TotalWidth)
	}
}

func TestWrapText(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(12)

	t.Run("EmptyText", func(t *testing.T) {
		block := b.WrapText("", 100)
		if len(block.Lines) != 0 {
			t.Errorf("WrapText empty = %d lines, want 0", len(block.Lines))
		}
	})

	t.Run("ZeroWidth", func(t *testing.T) {
		block := b.WrapText("Hello", 0)
		if len(block.Lines) != 0 {
			t.Errorf("WrapText zero width = %d lines, want 0", len(block.Lines))
		}
	})

	t.Run("ShortText", func(t *testing.T) {
		block := b.WrapText("Hello", 500)
		if len(block.Lines) != 1 {
			t.Errorf("WrapText short = %d lines, want 1", len(block.Lines))
		}
		if block.Lines[0].Text != "Hello" {
			t.Errorf("WrapText short text = %q, want %q", block.Lines[0].Text, "Hello")
		}
	})

	t.Run("MultiWordWrap", func(t *testing.T) {
		// Use a very narrow width to force wrapping (30pt is very small)
		block := b.WrapText("Hello World Test More Words Here", 30)
		if len(block.Lines) < 2 {
			t.Errorf("WrapText multi-word = %d lines, want >= 2 (lines: %v)", len(block.Lines), block.Lines)
		}
	})

	t.Run("ExplicitNewlines", func(t *testing.T) {
		block := b.WrapText("Line 1\nLine 2\nLine 3", 500)
		if len(block.Lines) != 3 {
			t.Errorf("WrapText newlines = %d lines, want 3", len(block.Lines))
		}
	})

	t.Run("EmptyParagraph", func(t *testing.T) {
		block := b.WrapText("Line 1\n\nLine 3", 500)
		if len(block.Lines) != 3 {
			t.Errorf("WrapText empty paragraph = %d lines, want 3", len(block.Lines))
		}
	})

	t.Run("TotalDimensions", func(t *testing.T) {
		block := b.WrapText("Test", 500)
		if block.TotalWidth <= 0 {
			t.Error("TotalWidth should be > 0")
		}
		if block.LineHeight <= 0 {
			t.Error("LineHeight should be > 0")
		}
		if block.TotalHeight <= 0 {
			t.Error("TotalHeight should be > 0")
		}
	})

	t.Run("LongWordBreaksWithHyphen", func(t *testing.T) {
		// A single long word in a very narrow column should be split across
		// multiple lines with hyphens, preserving the full text.
		block := b.WrapText("Supercalifragilistic", 30)
		if len(block.Lines) < 2 {
			t.Errorf("WrapText long word = %d lines, want >=2 (hyphenated); lines: %v",
				len(block.Lines), block.Lines)
		}
		// All lines except the last should end with a hyphen
		for i := 0; i < len(block.Lines)-1; i++ {
			text := block.Lines[i].Text
			if !strings.HasSuffix(text, "-") {
				t.Errorf("WrapText line %d should end with hyphen, got %q", i, text)
			}
		}
	})
}

func TestSplitIntoWords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"Hello", 1},
		{"Hello World", 2},
		{"  Hello   World  ", 2},
		{"A B C D", 4},
		{"\tHello\nWorld\r\n", 2},
	}

	for _, tt := range tests {
		words := splitIntoWords(tt.input)
		if len(words) != tt.want {
			t.Errorf("splitIntoWords(%q) = %d words, want %d", tt.input, len(words), tt.want)
		}
	}
}

func TestTruncateText(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(12)

	t.Run("EmptyText", func(t *testing.T) {
		result, width := b.TruncateText("", 100, TextOverflowEllipsis)
		if result != "" || width != 0 {
			t.Errorf("TruncateText empty = (%q, %v), want (\"\", 0)", result, width)
		}
	})

	t.Run("ZeroWidth", func(t *testing.T) {
		result, width := b.TruncateText("Hello", 0, TextOverflowEllipsis)
		if result != "" || width != 0 {
			t.Errorf("TruncateText zero width = (%q, %v), want (\"\", 0)", result, width)
		}
	})

	t.Run("TextFits", func(t *testing.T) {
		result, width := b.TruncateText("Hi", 500, TextOverflowEllipsis)
		if result != "Hi" {
			t.Errorf("TruncateText fits = %q, want %q", result, "Hi")
		}
		if width <= 0 {
			t.Error("Width should be > 0")
		}
	})

	t.Run("TextOverflowClip", func(t *testing.T) {
		result, _ := b.TruncateText("Hello World Long Text", 40, TextOverflowClip)
		// Should truncate without ellipsis
		if len(result) >= len("Hello World Long Text") {
			t.Errorf("TruncateText clip should truncate: %q", result)
		}
	})

	t.Run("TextOverflowEllipsis", func(t *testing.T) {
		result, _ := b.TruncateText("Hello World Long Text", 50, TextOverflowEllipsis)
		// Should end with ellipsis
		if len(result) < 3 {
			t.Errorf("TruncateText ellipsis too short: %q", result)
		}
	})

	t.Run("TextOverflowWordBreak", func(t *testing.T) {
		result, _ := b.TruncateText("Hello World Long Text", 60, TextOverflowWordBreak)
		// Should break at word boundary
		if len(result) < 3 {
			t.Errorf("TruncateText word break too short: %q", result)
		}
	})
}

func TestAlignTextInRect(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(12)

	rect := Rect{X: 100, Y: 100, W: 200, H: 50}

	t.Run("TopLeft", func(t *testing.T) {
		x, y := b.AlignTextInRect("Test", rect, AlignTopLeft)
		if x != rect.X {
			t.Errorf("AlignTopLeft X = %v, want %v", x, rect.X)
		}
		// Y should be somewhere within the rect (accounting for baseline)
		if y < rect.Y || y > rect.Y+rect.H {
			t.Errorf("AlignTopLeft Y = %v, should be in [%v, %v]", y, rect.Y, rect.Y+rect.H)
		}
	})

	t.Run("Center", func(t *testing.T) {
		x, y := b.AlignTextInRect("Test", rect, AlignCenter)
		// X should be somewhere in the middle
		if x < rect.X || x > rect.X+rect.W {
			t.Errorf("AlignCenter X = %v, should be in [%v, %v]", x, rect.X, rect.X+rect.W)
		}
		// Y should be somewhere in the middle
		if y < rect.Y || y > rect.Y+rect.H {
			t.Errorf("AlignCenter Y = %v, should be in [%v, %v]", y, rect.Y, rect.Y+rect.H)
		}
	})

	t.Run("BottomRight", func(t *testing.T) {
		x, y := b.AlignTextInRect("Test", rect, AlignBottomRight)
		// X should be near right edge
		if x < rect.X || x > rect.X+rect.W {
			t.Errorf("AlignBottomRight X = %v, should be in [%v, %v]", x, rect.X, rect.X+rect.W)
		}
		// Y should be near bottom
		if y < rect.Y || y > rect.Y+rect.H {
			t.Errorf("AlignBottomRight Y = %v, should be in [%v, %v]", y, rect.Y, rect.Y+rect.H)
		}
	})
}

func TestAlignBlockInRect(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(12)

	rect := Rect{X: 100, Y: 100, W: 200, H: 100}
	block := TextBlock{
		Lines: []TextLine{
			{Text: "Line 1", Width: 50, Height: 12},
			{Text: "Line 2", Width: 40, Height: 12},
		},
		TotalWidth:  50,
		LineHeight:  12,
		TotalHeight: 24,
	}

	t.Run("TopLeft", func(t *testing.T) {
		x, y := b.AlignBlockInRect(block, rect, AlignTopLeft)
		if x != rect.X {
			t.Errorf("AlignTopLeft X = %v, want %v", x, rect.X)
		}
		// Y should account for first line
		if y < rect.Y {
			t.Errorf("AlignTopLeft Y = %v, should be >= %v", y, rect.Y)
		}
	})

	t.Run("Center", func(t *testing.T) {
		x, y := b.AlignBlockInRect(block, rect, AlignCenter)
		expectedX := rect.X + rect.W/2
		if x != expectedX {
			t.Errorf("AlignCenter X = %v, want %v", x, expectedX)
		}
		// Y should be centered
		if y < rect.Y || y > rect.Y+rect.H {
			t.Errorf("AlignCenter Y = %v, should be in [%v, %v]", y, rect.Y, rect.Y+rect.H)
		}
	})

	t.Run("EmptyBlock", func(t *testing.T) {
		emptyBlock := TextBlock{}
		x, y := b.AlignBlockInRect(emptyBlock, rect, AlignCenter)
		if x != rect.X || y != rect.Y {
			t.Errorf("Empty block align = (%v, %v), want (%v, %v)", x, y, rect.X, rect.Y)
		}
	})
}

func TestDrawTextBlock(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(12)

	block := TextBlock{
		Lines: []TextLine{
			{Text: "Line 1", Width: 50, Height: 12},
			{Text: "Line 2", Width: 40, Height: 12},
		},
		TotalWidth:  50,
		LineHeight:  12,
		TotalHeight: 24,
	}

	// Should not panic
	b.DrawTextBlock(block, 100, 100, HorizontalAlignLeft)
	b.DrawTextBlock(block, 200, 100, HorizontalAlignCenter)
	b.DrawTextBlock(block, 300, 100, HorizontalAlignRight)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestDrawWrappedText(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(12)

	rect := Rect{X: 50, Y: 50, W: 200, H: 100}
	text := "This is a longer text that should wrap across multiple lines."

	b.DrawWrappedText(text, rect, AlignCenter)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestDrawWrappedText_TruncatesOverflow(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(14)

	// Create a rect that is very short — can hold at most ~1 line of 14pt text.
	rect := Rect{X: 50, Y: 50, W: 200, H: 18}
	text := "Line one of the text. Line two of the text. Line three of the text. Line four."

	// This should not panic and should render without error.
	b.DrawWrappedText(text, rect, AlignTopLeft)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestDrawWrappedText_SmallRect_NoOverflow(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	b.SetFontSize(10)

	// Rect tall enough for a few lines — text should not be truncated.
	rect := Rect{X: 0, Y: 0, W: 100, H: 200}
	text := "Short text"

	b.DrawWrappedText(text, rect, AlignCenter)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestBoxPadding(t *testing.T) {
	t.Run("UniformPadding", func(t *testing.T) {
		p := UniformPadding(10)
		if p.Top != 10 || p.Right != 10 || p.Bottom != 10 || p.Left != 10 {
			t.Errorf("UniformPadding(10) = %+v, want all 10", p)
		}
	})

	t.Run("SymmetricPadding", func(t *testing.T) {
		p := SymmetricPadding(5, 10)
		if p.Top != 5 || p.Bottom != 5 || p.Left != 10 || p.Right != 10 {
			t.Errorf("SymmetricPadding(5, 10) = %+v", p)
		}
	})
}

func TestBox(t *testing.T) {
	bounds := Rect{X: 0, Y: 0, W: 100, H: 50}
	padding := BoxPadding{Top: 5, Right: 10, Bottom: 5, Left: 10}
	box := NewBox(bounds, padding)

	content := box.ContentRect()
	expected := Rect{X: 10, Y: 5, W: 80, H: 40}

	if content != expected {
		t.Errorf("ContentRect() = %+v, want %+v", content, expected)
	}
}

func TestSplitHorizontal(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 100, H: 50}

	t.Run("Zero", func(t *testing.T) {
		parts := SplitHorizontal(r, 0, 0)
		if parts != nil {
			t.Error("SplitHorizontal(0) should return nil")
		}
	})

	t.Run("One", func(t *testing.T) {
		parts := SplitHorizontal(r, 1, 0)
		if len(parts) != 1 || parts[0] != r {
			t.Errorf("SplitHorizontal(1) = %+v, want [%+v]", parts, r)
		}
	})

	t.Run("Two", func(t *testing.T) {
		parts := SplitHorizontal(r, 2, 0)
		if len(parts) != 2 {
			t.Fatalf("SplitHorizontal(2) = %d parts, want 2", len(parts))
		}
		if parts[0].W != 50 || parts[1].W != 50 {
			t.Errorf("Parts widths = %v, %v, want 50, 50", parts[0].W, parts[1].W)
		}
	})

	t.Run("TwoWithGap", func(t *testing.T) {
		parts := SplitHorizontal(r, 2, 10)
		if len(parts) != 2 {
			t.Fatalf("SplitHorizontal(2, gap=10) = %d parts, want 2", len(parts))
		}
		// With 10pt gap: (100 - 10) / 2 = 45 each
		if parts[0].W != 45 || parts[1].W != 45 {
			t.Errorf("Parts widths = %v, %v, want 45, 45", parts[0].W, parts[1].W)
		}
		// Second part should start at 45 + 10 = 55
		if parts[1].X != 55 {
			t.Errorf("Second part X = %v, want 55", parts[1].X)
		}
	})
}

func TestSplitVertical(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 100, H: 60}

	t.Run("Three", func(t *testing.T) {
		parts := SplitVertical(r, 3, 0)
		if len(parts) != 3 {
			t.Fatalf("SplitVertical(3) = %d parts, want 3", len(parts))
		}
		if parts[0].H != 20 || parts[1].H != 20 || parts[2].H != 20 {
			t.Errorf("Parts heights = %v, %v, %v, want 20, 20, 20",
				parts[0].H, parts[1].H, parts[2].H)
		}
	})
}

func TestSplitGrid(t *testing.T) {
	r := Rect{X: 0, Y: 0, W: 100, H: 80}

	t.Run("Invalid", func(t *testing.T) {
		grid := SplitGrid(r, 0, 2, 0, 0)
		if grid != nil {
			t.Error("SplitGrid(0, 2) should return nil")
		}
	})

	t.Run("2x2", func(t *testing.T) {
		grid := SplitGrid(r, 2, 2, 0, 0)
		if len(grid) != 2 || len(grid[0]) != 2 {
			t.Fatalf("SplitGrid(2, 2) = %dx%d, want 2x2", len(grid), len(grid[0]))
		}
		// Each cell should be 50x40
		if grid[0][0].W != 50 || grid[0][0].H != 40 {
			t.Errorf("Cell size = %vx%v, want 50x40", grid[0][0].W, grid[0][0].H)
		}
	})

	t.Run("2x2WithGaps", func(t *testing.T) {
		grid := SplitGrid(r, 2, 2, 10, 10)
		if len(grid) != 2 || len(grid[0]) != 2 {
			t.Fatalf("SplitGrid = %dx%d, want 2x2", len(grid), len(grid[0]))
		}
		// Width: (100 - 10) / 2 = 45
		// Height: (80 - 10) / 2 = 35
		if grid[0][0].W != 45 || grid[0][0].H != 35 {
			t.Errorf("Cell size = %vx%v, want 45x35", grid[0][0].W, grid[0][0].H)
		}
	})
}

func TestFlatGrid(t *testing.T) {
	grid := [][]Rect{
		{{X: 0, Y: 0}, {X: 1, Y: 0}},
		{{X: 0, Y: 1}, {X: 1, Y: 1}},
	}

	flat := FlatGrid(grid)
	if len(flat) != 4 {
		t.Errorf("FlatGrid = %d cells, want 4", len(flat))
	}
	// Check row-major order
	if flat[0].X != 0 || flat[1].X != 1 || flat[2].X != 0 || flat[3].X != 1 {
		t.Error("FlatGrid not in row-major order")
	}
}

func TestStackVertical(t *testing.T) {
	rects := []Rect{
		{X: 10, Y: 0, W: 80, H: 20},
		{X: 10, Y: 0, W: 80, H: 30},
		{X: 10, Y: 0, W: 80, H: 25},
	}

	result := StackVertical(rects, 5)
	if len(result) != 3 {
		t.Fatalf("StackVertical = %d rects, want 3", len(result))
	}

	// Check Y positions
	if result[0].Y != 0 {
		t.Errorf("First rect Y = %v, want 0", result[0].Y)
	}
	if result[1].Y != 25 { // 20 + 5
		t.Errorf("Second rect Y = %v, want 25", result[1].Y)
	}
	if result[2].Y != 60 { // 25 + 30 + 5
		t.Errorf("Third rect Y = %v, want 60", result[2].Y)
	}
}

func TestStackHorizontal(t *testing.T) {
	rects := []Rect{
		{X: 0, Y: 10, W: 30, H: 50},
		{X: 0, Y: 10, W: 40, H: 50},
	}

	result := StackHorizontal(rects, 10)
	if len(result) != 2 {
		t.Fatalf("StackHorizontal = %d rects, want 2", len(result))
	}

	if result[0].X != 0 {
		t.Errorf("First rect X = %v, want 0", result[0].X)
	}
	if result[1].X != 40 { // 30 + 10
		t.Errorf("Second rect X = %v, want 40", result[1].X)
	}
}

func TestCenterInRect(t *testing.T) {
	parent := Rect{X: 0, Y: 0, W: 100, H: 80}
	child := CenterInRect(parent, 40, 20)

	expected := Rect{X: 30, Y: 30, W: 40, H: 20}
	if child != expected {
		t.Errorf("CenterInRect = %+v, want %+v", child, expected)
	}
}

func TestAlignInRect(t *testing.T) {
	parent := Rect{X: 0, Y: 0, W: 100, H: 80}

	tests := []struct {
		align BoxAlign
		want  Rect
	}{
		{AlignTopLeft, Rect{X: 0, Y: 0, W: 40, H: 20}},
		{AlignTopCenter, Rect{X: 30, Y: 0, W: 40, H: 20}},
		{AlignTopRight, Rect{X: 60, Y: 0, W: 40, H: 20}},
		{AlignMiddleLeft, Rect{X: 0, Y: 30, W: 40, H: 20}},
		{AlignCenter, Rect{X: 30, Y: 30, W: 40, H: 20}},
		{AlignMiddleRight, Rect{X: 60, Y: 30, W: 40, H: 20}},
		{AlignBottomLeft, Rect{X: 0, Y: 60, W: 40, H: 20}},
		{AlignBottomCenter, Rect{X: 30, Y: 60, W: 40, H: 20}},
		{AlignBottomRight, Rect{X: 60, Y: 60, W: 40, H: 20}},
	}

	for _, tt := range tests {
		result := AlignInRect(parent, 40, 20, tt.align)
		if result != tt.want {
			t.Errorf("AlignInRect(%+v) = %+v, want %+v", tt.align, result, tt.want)
		}
	}
}

func TestExpandRect(t *testing.T) {
	r := Rect{X: 50, Y: 50, W: 100, H: 80}

	t.Run("Asymmetric", func(t *testing.T) {
		expanded := ExpandRect(r, 10, 20, 30, 40)
		expected := Rect{X: 10, Y: 40, W: 160, H: 120}
		if expanded != expected {
			t.Errorf("ExpandRect = %+v, want %+v", expanded, expected)
		}
	})

	t.Run("Uniform", func(t *testing.T) {
		expanded := ExpandRectAll(r, 10)
		expected := Rect{X: 40, Y: 40, W: 120, H: 100}
		if expanded != expected {
			t.Errorf("ExpandRectAll = %+v, want %+v", expanded, expected)
		}
	})
}

func TestBoundingRect(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		result := BoundingRect(nil)
		if result != (Rect{}) {
			t.Errorf("BoundingRect(nil) = %+v, want empty", result)
		}
	})

	t.Run("Single", func(t *testing.T) {
		r := Rect{X: 10, Y: 20, W: 30, H: 40}
		result := BoundingRect([]Rect{r})
		if result != r {
			t.Errorf("BoundingRect single = %+v, want %+v", result, r)
		}
	})

	t.Run("Multiple", func(t *testing.T) {
		rects := []Rect{
			{X: 0, Y: 10, W: 50, H: 30},
			{X: 30, Y: 0, W: 70, H: 50},
		}
		result := BoundingRect(rects)
		expected := Rect{X: 0, Y: 0, W: 100, H: 50}
		if result != expected {
			t.Errorf("BoundingRect = %+v, want %+v", result, expected)
		}
	})
}

func TestRectIntersects(t *testing.T) {
	r1 := Rect{X: 0, Y: 0, W: 50, H: 50}

	tests := []struct {
		r2   Rect
		want bool
	}{
		{Rect{X: 25, Y: 25, W: 50, H: 50}, true},    // Overlapping
		{Rect{X: 0, Y: 0, W: 50, H: 50}, true},      // Same
		{Rect{X: 10, Y: 10, W: 20, H: 20}, true},    // Inside
		{Rect{X: 50, Y: 0, W: 50, H: 50}, false},    // Adjacent (no overlap)
		{Rect{X: 100, Y: 100, W: 50, H: 50}, false}, // Far away
	}

	for _, tt := range tests {
		result := r1.Intersects(tt.r2)
		if result != tt.want {
			t.Errorf("Intersects(%+v) = %v, want %v", tt.r2, result, tt.want)
		}
	}
}

func TestRectIntersection(t *testing.T) {
	r1 := Rect{X: 0, Y: 0, W: 100, H: 100}

	t.Run("Overlapping", func(t *testing.T) {
		r2 := Rect{X: 50, Y: 50, W: 100, H: 100}
		result := r1.Intersection(r2)
		expected := Rect{X: 50, Y: 50, W: 50, H: 50}
		if result != expected {
			t.Errorf("Intersection = %+v, want %+v", result, expected)
		}
	})

	t.Run("NoOverlap", func(t *testing.T) {
		r2 := Rect{X: 200, Y: 200, W: 50, H: 50}
		result := r1.Intersection(r2)
		if result != (Rect{}) {
			t.Errorf("Intersection no overlap = %+v, want empty", result)
		}
	})
}

func TestRectUnion(t *testing.T) {
	r1 := Rect{X: 0, Y: 0, W: 50, H: 50}
	r2 := Rect{X: 100, Y: 100, W: 50, H: 50}

	result := r1.Union(r2)
	expected := Rect{X: 0, Y: 0, W: 150, H: 150}
	if result != expected {
		t.Errorf("Union = %+v, want %+v", result, expected)
	}
}

func TestRectIsEmpty(t *testing.T) {
	tests := []struct {
		r    Rect
		want bool
	}{
		{Rect{X: 0, Y: 0, W: 0, H: 0}, true},
		{Rect{X: 0, Y: 0, W: 10, H: 0}, true},
		{Rect{X: 0, Y: 0, W: 0, H: 10}, true},
		{Rect{X: 0, Y: 0, W: -1, H: 10}, true},
		{Rect{X: 0, Y: 0, W: 10, H: 10}, false},
	}

	for _, tt := range tests {
		result := tt.r.IsEmpty()
		if result != tt.want {
			t.Errorf("IsEmpty(%+v) = %v, want %v", tt.r, result, tt.want)
		}
	}
}

func TestRectArea(t *testing.T) {
	tests := []struct {
		r    Rect
		want float64
	}{
		{Rect{X: 0, Y: 0, W: 10, H: 5}, 50},
		{Rect{X: 0, Y: 0, W: 0, H: 5}, 0},
		{Rect{X: 0, Y: 0, W: -10, H: 5}, 0},
	}

	for _, tt := range tests {
		result := tt.r.Area()
		if result != tt.want {
			t.Errorf("Area(%+v) = %v, want %v", tt.r, result, tt.want)
		}
	}
}

func TestRectAspectRatio(t *testing.T) {
	tests := []struct {
		r    Rect
		want float64
	}{
		{Rect{W: 100, H: 50}, 2.0},
		{Rect{W: 50, H: 100}, 0.5},
		{Rect{W: 100, H: 0}, 0},
	}

	for _, tt := range tests {
		result := tt.r.AspectRatio()
		if result != tt.want {
			t.Errorf("AspectRatio(%+v) = %v, want %v", tt.r, result, tt.want)
		}
	}
}

func TestRectScaleToFit(t *testing.T) {
	source := Rect{X: 0, Y: 0, W: 100, H: 50}
	target := Rect{X: 0, Y: 0, W: 50, H: 50}

	result := source.ScaleToFit(target)

	// Width is limiting factor: 100 -> 50, so scale is 0.5
	// Height becomes 50 * 0.5 = 25
	if result.W != 50 {
		t.Errorf("ScaleToFit width = %v, want 50", result.W)
	}
	if result.H != 25 {
		t.Errorf("ScaleToFit height = %v, want 25", result.H)
	}
	// Should be centered
	if result.X != 0 {
		t.Errorf("ScaleToFit X = %v, want 0", result.X)
	}
	if result.Y != 12.5 {
		t.Errorf("ScaleToFit Y = %v, want 12.5", result.Y)
	}
}

func TestRectScaleToFill(t *testing.T) {
	source := Rect{X: 0, Y: 0, W: 100, H: 50}
	target := Rect{X: 0, Y: 0, W: 50, H: 50}

	result := source.ScaleToFill(target)

	// Height is limiting factor: 50 -> 50, so scale is 1.0
	// Width stays 100
	if result.W != 100 {
		t.Errorf("ScaleToFill width = %v, want 100", result.W)
	}
	if result.H != 50 {
		t.Errorf("ScaleToFill height = %v, want 50", result.H)
	}
}

func TestCommonAlignments(t *testing.T) {
	// Verify common alignments are set correctly
	if AlignTopLeft.Horizontal != HorizontalAlignLeft || AlignTopLeft.Vertical != VerticalAlignTop {
		t.Error("AlignTopLeft incorrect")
	}
	if AlignCenter.Horizontal != HorizontalAlignCenter || AlignCenter.Vertical != VerticalAlignMiddle {
		t.Error("AlignCenter incorrect")
	}
	if AlignBottomRight.Horizontal != HorizontalAlignRight || AlignBottomRight.Vertical != VerticalAlignBottom {
		t.Error("AlignBottomRight incorrect")
	}
}
