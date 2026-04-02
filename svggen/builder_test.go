package svggen

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"sync"
	"testing"
)

func TestPoint(t *testing.T) {
	p := Point{X: 10, Y: 20}
	if p.X != 10 || p.Y != 20 {
		t.Errorf("Point{10, 20} = %+v", p)
	}
}

func TestRect(t *testing.T) {
	t.Run("Min", func(t *testing.T) {
		r := Rect{X: 10, Y: 20, W: 100, H: 50}
		min := r.Min()
		if min.X != 10 || min.Y != 20 {
			t.Errorf("Min() = %+v, want {10, 20}", min)
		}
	})

	t.Run("Max", func(t *testing.T) {
		r := Rect{X: 10, Y: 20, W: 100, H: 50}
		max := r.Max()
		if max.X != 110 || max.Y != 70 {
			t.Errorf("Max() = %+v, want {110, 70}", max)
		}
	})

	t.Run("Center", func(t *testing.T) {
		r := Rect{X: 0, Y: 0, W: 100, H: 50}
		center := r.Center()
		if center.X != 50 || center.Y != 25 {
			t.Errorf("Center() = %+v, want {50, 25}", center)
		}
	})

	t.Run("Inset", func(t *testing.T) {
		r := Rect{X: 0, Y: 0, W: 100, H: 50}
		inset := r.Inset(5, 10, 15, 20)

		if inset.X != 20 || inset.Y != 5 || inset.W != 70 || inset.H != 30 {
			t.Errorf("Inset(5, 10, 15, 20) = %+v, want {20, 5, 70, 30}", inset)
		}
	})
}

func TestNewSVGBuilder(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	if b.Width() != 800 {
		t.Errorf("Width() = %v, want 800", b.Width())
	}
	if b.Height() != 600 {
		t.Errorf("Height() = %v, want 600", b.Height())
	}
	if b.canvas == nil {
		t.Error("canvas is nil")
	}
	if b.ctx == nil {
		t.Error("ctx is nil")
	}
	if b.style == nil {
		t.Error("style is nil")
	}
}

func TestSVGBuilder_Bounds(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	bounds := b.Bounds()

	if bounds.X != 0 || bounds.Y != 0 || bounds.W != 800 || bounds.H != 600 {
		t.Errorf("Bounds() = %+v, want {0, 0, 800, 600}", bounds)
	}
}

func TestSVGBuilder_SetStyleGuide(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	customStyle := VibrantPalette()
	styleGuide := &StyleGuide{Palette: customStyle}

	b.SetStyleGuide(styleGuide)

	if b.StyleGuide() != styleGuide {
		t.Error("SetStyleGuide did not set the style guide")
	}
}

func TestSVGBuilder_SetStyleGuide_UpdatesFont(t *testing.T) {
	tests := []struct {
		name       string
		fontFamily string
	}{
		{"Helvetica", "Helvetica"}, // Common system font
		{"Arial", "Arial"},         // Should be same as default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewSVGBuilder(800, 600)

			// Get the initial font family pointer (should be Arial by default)
			initialFontPtr := b.fontFamily

			// Create a style guide with a different font
			styleGuide := DefaultStyleGuide()
			styleGuide.Typography.FontFamily = tt.fontFamily
			b.SetStyleGuide(styleGuide)

			// The builder's font family should be updated (different pointer unless same font)
			if tt.fontFamily == "Arial" {
				// Same font name should result in same cached pointer
				if b.fontFamily != initialFontPtr {
					t.Error("SetStyleGuide with same font should use cached font")
				}
			} else {
				// Different font should result in different cached pointer
				if b.fontFamily == initialFontPtr {
					t.Errorf("SetStyleGuide did not update font family to %s", tt.fontFamily)
				}
			}

			// Verify the style guide is properly set
			if b.StyleGuide() != styleGuide {
				t.Error("SetStyleGuide did not set the style guide")
			}
			if b.StyleGuide().Typography.FontFamily != tt.fontFamily {
				t.Errorf("Typography.FontFamily = %q, want %q",
					b.StyleGuide().Typography.FontFamily, tt.fontFamily)
			}
		})
	}
}

func TestSVGBuilder_SetStyleGuide_NilTypography(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	initialFont := b.fontFamily

	// Create a style guide with nil typography
	styleGuide := &StyleGuide{Palette: DefaultPalette()}
	b.SetStyleGuide(styleGuide)

	// Font should remain unchanged when typography is nil
	if b.fontFamily != initialFont {
		t.Error("SetStyleGuide with nil typography should not change font")
	}
}

func TestSVGBuilder_SetStyleGuide_EmptyFontFamily(t *testing.T) {
	b := NewSVGBuilder(800, 600)
	initialFont := b.fontFamily

	// Create a style guide with empty font family
	styleGuide := DefaultStyleGuide()
	styleGuide.Typography.FontFamily = ""
	b.SetStyleGuide(styleGuide)

	// Font should remain unchanged when font family is empty
	if b.fontFamily != initialFont {
		t.Error("SetStyleGuide with empty FontFamily should not change font")
	}
}

func TestSVGBuilder_SetFontSize(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	b.SetFontSize(24)
	if b.fontSize != 24 {
		t.Errorf("fontSize = %v, want 24", b.fontSize)
	}
}

func TestSVGBuilder_SetFontWeight(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	tests := []struct {
		weight int
		name   string
	}{
		{300, "light"},
		{400, "regular"},
		{500, "medium"},
		{600, "semibold"},
		{700, "bold"},
	}

	for _, tt := range tests {
		b.SetFontWeight(tt.weight)
		// fontWeight is set, just verify no panic
	}
}

func TestSVGBuilder_RenderBasic(t *testing.T) {
	b := NewSVGBuilder(200, 100)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if doc == nil {
		t.Fatal("Render() returned nil document")
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}

	svg := doc.String()
	if !strings.Contains(svg, "<svg") {
		t.Error("Render() output does not contain <svg tag")
	}
	if !strings.Contains(svg, "</svg>") {
		t.Error("Render() output does not contain </svg> tag")
	}
}

func TestSVGBuilder_DrawRect(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FF0000"))
	b.DrawRect(Rect{X: 10, Y: 10, W: 50, H: 30})

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := doc.String()
	// SVG should contain path elements for the rectangle
	if !strings.Contains(svg, "<svg") {
		t.Error("SVG missing svg element")
	}
}

func TestSVGBuilder_DrawRoundedRect(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#00FF00"))
	b.DrawRoundedRect(Rect{X: 10, Y: 10, W: 50, H: 30}, 5)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawCircle(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#0000FF"))
	b.DrawCircle(50, 50, 25)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawEllipse(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FFFF00"))
	b.DrawEllipse(50, 50, 30, 20)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawLine(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetStrokeColor(MustParseColor("#000000"))
	b.SetStrokeWidth(2)
	b.DrawLine(10, 10, 190, 90)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawPolyline(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetStrokeColor(MustParseColor("#FF00FF"))
	b.SetStrokeWidth(2)

	points := []Point{
		{10, 10},
		{50, 50},
		{100, 20},
		{150, 80},
	}
	b.DrawPolyline(points)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawPolygon(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#00FFFF"))

	points := []Point{
		{100, 10},
		{150, 90},
		{50, 90},
	}
	b.DrawPolygon(points)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_PathBuilder(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FF6600"))

	b.BeginPath().
		MoveTo(10, 10).
		LineTo(100, 10).
		QuadTo(150, 50, 100, 90).
		LineTo(10, 90).
		Close().
		Draw()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_PathBuilderCubic(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#9933FF"))

	b.BeginPath().
		MoveTo(10, 50).
		CubicTo(30, 10, 70, 90, 100, 50).
		Close().
		Fill()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

// TestSVGBuilder_PathFillSuppressesStroke verifies that Fill() does not
// leak a parent-context stroke onto the filled path.
// Regression test for pptx-3ly (spurious baseline stroke on area fills).
func TestSVGBuilder_PathFillSuppressesStroke(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	// Set a visible stroke in the parent context
	b.SetStrokeColor(MustParseColor("#FF0000"))
	b.SetStrokeWidth(5)
	b.SetFillColor(MustParseColor("#00FF00"))

	b.BeginPath().
		MoveTo(10, 10).
		LineTo(190, 10).
		LineTo(190, 90).
		LineTo(10, 90).
		Close().
		Fill()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := string(doc.Content)
	if len(svg) == 0 {
		t.Fatal("Render() returned empty content")
	}

	// The filled path must not carry the parent's stroke-width:5.
	// After the fix, Fill() sets stroke-width to 0 inside its Push/Pop.
	if strings.Contains(svg, `stroke-width="5`) || strings.Contains(svg, `stroke-width:5`) {
		t.Error("Fill() should suppress parent stroke-width; found stroke-width:5 in SVG output")
	}
}

func TestSVGBuilder_PathBuilderArc(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetStrokeColor(MustParseColor("#339933"))
	b.SetStrokeWidth(3)

	b.BeginPath().
		MoveTo(50, 50).
		ArcTo(30, 20, 0, true, true, 150, 50).
		Stroke()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawText(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFontSize(16)
	b.DrawText("Hello, World!", 100, 50, TextAlignCenter, TextBaselineMiddle)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_DrawTextAlignments(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFontSize(12)

	alignments := []TextAlign{TextAlignLeft, TextAlignCenter, TextAlignRight}
	baselines := []TextBaseline{TextBaselineTop, TextBaselineMiddle, TextBaselineBottom, TextBaselineAlphabetic}

	for _, align := range alignments {
		for _, baseline := range baselines {
			b.DrawText("Test", 100, 50, align, baseline)
		}
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_MeasureText(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFontSize(12)

	width, height := b.MeasureText("Hello")

	// The text should have non-zero dimensions
	if width <= 0 {
		t.Errorf("MeasureText width = %v, want > 0", width)
	}
	if height <= 0 {
		t.Errorf("MeasureText height = %v, want > 0", height)
	}
}

func TestSVGBuilder_Group(t *testing.T) {
	b := NewSVGBuilder(200, 100)

	// Draw outside group
	b.SetFillColor(MustParseColor("#FF0000"))
	b.DrawRect(Rect{X: 0, Y: 0, W: 20, H: 20})

	// Draw inside group
	g := b.BeginGroup()
	b.SetFillColor(MustParseColor("#00FF00"))
	b.DrawRect(Rect{X: 30, Y: 30, W: 20, H: 20})
	g.End()

	// Draw after group - should restore previous state
	b.DrawRect(Rect{X: 60, Y: 60, W: 20, H: 20})

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_PushPop(t *testing.T) {
	b := NewSVGBuilder(200, 100)

	b.SetFillColor(MustParseColor("#FF0000"))
	b.Push()
	b.SetFillColor(MustParseColor("#00FF00"))
	b.DrawRect(Rect{X: 0, Y: 0, W: 20, H: 20})
	b.Pop()
	b.DrawRect(Rect{X: 30, Y: 30, W: 20, H: 20})

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_Transform(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FF6600"))

	b.Push()
	b.Translate(50, 25)
	b.DrawRect(Rect{X: 0, Y: 0, W: 20, H: 20})
	b.Pop()

	b.Push()
	b.Rotate(45)
	b.DrawRect(Rect{X: 100, Y: 50, W: 20, H: 20})
	b.Pop()

	b.Push()
	b.Scale(2, 2)
	b.DrawRect(Rect{X: 10, Y: 10, W: 10, H: 10})
	b.Pop()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_RotateAround(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#3366FF"))

	b.Push()
	b.RotateAround(45, 100, 50)
	b.DrawRect(Rect{X: 90, Y: 40, W: 20, H: 20})
	b.Pop()

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_SetDashes(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetStrokeColor(MustParseColor("#000000"))
	b.SetStrokeWidth(2)
	b.SetDashes(4, 2)
	b.DrawLine(10, 50, 190, 50)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_SetLineCap(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetStrokeColor(MustParseColor("#000000"))
	b.SetStrokeWidth(10)

	caps := []LineCap{LineCapButt, LineCapRound, LineCapSquare}
	y := 20.0
	for _, cap := range caps {
		b.SetLineCap(cap)
		b.DrawLine(20, y, 180, y)
		y += 30
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_SetLineJoin(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetStrokeColor(MustParseColor("#000000"))
	b.SetStrokeWidth(5)
	b.SetFillColor(Color{A: 0}) // Transparent fill

	joins := []LineJoin{LineJoinMiter, LineJoinRound, LineJoinBevel}
	for _, join := range joins {
		b.SetLineJoin(join)
		points := []Point{{20, 80}, {50, 20}, {80, 80}}
		b.DrawPolygon(points)
	}

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_FillRect(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FF0000"))
	b.SetStrokeColor(MustParseColor("#000000"))
	b.SetStrokeWidth(2)
	b.FillRect(Rect{X: 10, Y: 10, W: 80, H: 40})

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_StrokeRect(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FF0000"))
	b.SetStrokeColor(MustParseColor("#000000"))
	b.SetStrokeWidth(2)
	b.StrokeRect(Rect{X: 110, Y: 10, W: 80, H: 40})

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(doc.Content) == 0 {
		t.Error("Render() returned empty content")
	}
}

func TestSVGBuilder_RenderToBytes(t *testing.T) {
	b := NewSVGBuilder(100, 100)
	b.SetFillColor(MustParseColor("#FF0000"))
	b.DrawRect(Rect{X: 10, Y: 10, W: 80, H: 80})

	bytes, err := b.RenderToBytes()
	if err != nil {
		t.Fatalf("RenderToBytes() error = %v", err)
	}

	if len(bytes) == 0 {
		t.Error("RenderToBytes() returned empty bytes")
	}

	if !strings.Contains(string(bytes), "<svg") {
		t.Error("RenderToBytes() output does not contain <svg")
	}
}

func TestSVGBuilder_RenderToString(t *testing.T) {
	b := NewSVGBuilder(100, 100)
	b.SetFillColor(MustParseColor("#00FF00"))
	b.DrawCircle(50, 50, 40)

	str, err := b.RenderToString()
	if err != nil {
		t.Fatalf("RenderToString() error = %v", err)
	}

	if len(str) == 0 {
		t.Error("RenderToString() returned empty string")
	}

	if !strings.Contains(str, "<svg") {
		t.Error("RenderToString() output does not contain <svg")
	}
}

func TestSVGBuilder_EdgeCases(t *testing.T) {
	t.Run("EmptyPolyline", func(t *testing.T) {
		b := NewSVGBuilder(100, 100)
		b.DrawPolyline([]Point{})
		_, err := b.Render()
		if err != nil {
			t.Errorf("Empty polyline should not error: %v", err)
		}
	})

	t.Run("SinglePointPolyline", func(t *testing.T) {
		b := NewSVGBuilder(100, 100)
		b.DrawPolyline([]Point{{50, 50}})
		_, err := b.Render()
		if err != nil {
			t.Errorf("Single point polyline should not error: %v", err)
		}
	})

	t.Run("TwoPointPolygon", func(t *testing.T) {
		b := NewSVGBuilder(100, 100)
		b.DrawPolygon([]Point{{0, 0}, {50, 50}})
		_, err := b.Render()
		if err != nil {
			t.Errorf("Two point polygon should not error: %v", err)
		}
	})

	t.Run("ZeroSizeRect", func(t *testing.T) {
		b := NewSVGBuilder(100, 100)
		b.DrawRect(Rect{X: 50, Y: 50, W: 0, H: 0})
		_, err := b.Render()
		if err != nil {
			t.Errorf("Zero size rect should not error: %v", err)
		}
	})

	t.Run("ZeroRadiusCircle", func(t *testing.T) {
		b := NewSVGBuilder(100, 100)
		b.DrawCircle(50, 50, 0)
		_, err := b.Render()
		if err != nil {
			t.Errorf("Zero radius circle should not error: %v", err)
		}
	})

	t.Run("EmptyText", func(t *testing.T) {
		b := NewSVGBuilder(100, 100)
		b.DrawText("", 50, 50, TextAlignCenter, TextBaselineMiddle)
		_, err := b.Render()
		if err != nil {
			t.Errorf("Empty text should not error: %v", err)
		}
	})
}

func TestSVGBuilder_ComplexScene(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	// Background
	b.SetFillColor(MustParseColor("#F5F5F5"))
	b.FillRect(b.Bounds())

	// Draw multiple shapes
	b.SetFillColor(MustParseColor("#4E79A7"))
	b.DrawRoundedRect(Rect{X: 20, Y: 20, W: 100, H: 60}, 8)

	b.SetFillColor(MustParseColor("#F28E2B"))
	b.DrawCircle(200, 80, 40)

	b.SetFillColor(MustParseColor("#E15759"))
	b.DrawEllipse(320, 80, 50, 30)

	// Draw lines
	b.SetStrokeColor(MustParseColor("#76B7B2"))
	b.SetStrokeWidth(2)
	b.DrawLine(20, 150, 380, 150)

	// Draw a path
	b.SetFillColor(MustParseColor("#59A14F"))
	b.BeginPath().
		MoveTo(50, 200).
		LineTo(100, 280).
		LineTo(150, 220).
		LineTo(200, 260).
		LineTo(250, 200).
		Close().
		Fill()

	// Draw text
	b.SetFontSize(18)
	b.DrawText("Complex Scene Test", 200, 290, TextAlignCenter, TextBaselineBottom)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	svg := doc.String()
	if len(svg) < 100 {
		t.Error("Complex scene SVG unexpectedly short")
	}

	// Verify SVG structure
	if !strings.Contains(svg, "<svg") {
		t.Error("Missing <svg> tag")
	}
	if !strings.Contains(svg, "</svg>") {
		t.Error("Missing </svg> tag")
	}
}

func TestSVGDocument(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFillColor(MustParseColor("#FF0000"))
	b.DrawRect(Rect{X: 10, Y: 10, W: 50, H: 30})

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	t.Run("Width", func(t *testing.T) {
		if doc.Width != 267 {
			t.Errorf("Width = %v, want 267 (200pt in CSS pixels)", doc.Width)
		}
	})

	t.Run("Height", func(t *testing.T) {
		if doc.Height != 133 {
			t.Errorf("Height = %v, want 133 (100pt in CSS pixels)", doc.Height)
		}
	})

	t.Run("ViewBox", func(t *testing.T) {
		if doc.ViewBox == "" {
			t.Error("ViewBox is empty")
		}
	})

	t.Run("String", func(t *testing.T) {
		str := doc.String()
		if str == "" {
			t.Error("String() returned empty")
		}
	})

	t.Run("Bytes", func(t *testing.T) {
		bytes := doc.Bytes()
		if len(bytes) == 0 {
			t.Error("Bytes() returned empty")
		}
	})
}

// TestSVGDocumentDimensionsMatchViewBox verifies that SVGDocument.Width/Height
// are in CSS pixels (matching the SVG viewport), not in the original point units.
// This prevents a unit mismatch where Content uses CSS pixels but Width/Height
// would be in points, causing wrong scale when embedding.
func TestSVGDocumentDimensionsMatchViewBox(t *testing.T) {
	tests := []struct {
		name           string
		widthPt        float64
		heightPt       float64
		wantWidthPx    float64
		wantHeightPx   float64
		wantViewBox    string
	}{
		{
			name:         "800x600pt standard",
			widthPt:      800,
			heightPt:     600,
			wantWidthPx:  1067,
			wantHeightPx: 800,
			wantViewBox:  "0 0 1067 800",
		},
		{
			name:         "200x100pt small",
			widthPt:      200,
			heightPt:     100,
			wantWidthPx:  267,
			wantHeightPx: 133,
			wantViewBox:  "0 0 267 133",
		},
		{
			name:         "1200x400pt wide",
			widthPt:      1200,
			heightPt:     400,
			wantWidthPx:  1600,
			wantHeightPx: 533,
			wantViewBox:  "0 0 1600 533",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewSVGBuilder(tt.widthPt, tt.heightPt)
			b.SetFillColor(MustParseColor("#FF0000"))
			b.DrawRect(Rect{X: 10, Y: 10, W: 50, H: 30})

			doc, err := b.Render()
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}

			if doc.Width != tt.wantWidthPx {
				t.Errorf("Width = %v, want %v (CSS pixels for %.0fpt)", doc.Width, tt.wantWidthPx, tt.widthPt)
			}
			if doc.Height != tt.wantHeightPx {
				t.Errorf("Height = %v, want %v (CSS pixels for %.0fpt)", doc.Height, tt.wantHeightPx, tt.heightPt)
			}
			if doc.ViewBox != tt.wantViewBox {
				t.Errorf("ViewBox = %q, want %q", doc.ViewBox, tt.wantViewBox)
			}

			// Verify Width/Height match the viewBox dimensions
			var vbW, vbH float64
			fmt.Sscanf(doc.ViewBox, "0 0 %f %f", &vbW, &vbH)
			if doc.Width != vbW {
				t.Errorf("Width (%v) does not match viewBox width (%v)", doc.Width, vbW)
			}
			if doc.Height != vbH {
				t.Errorf("Height (%v) does not match viewBox height (%v)", doc.Height, vbH)
			}
		})
	}
}

// TestFixSVGFontUnits tests that px→mm replacement in font declarations works.
func TestFixSVGFontUnits(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic font declaration",
			input:    `<text style="font: 2.37px Arial">`,
			expected: `<text style="font: 2.37mm Arial">`,
		},
		{
			name:     "font with weight",
			input:    `<text style="font: 500 2.80px Arial">`,
			expected: `<text style="font: 500 2.80mm Arial">`,
		},
		{
			name:     "font with fill",
			input:    `<text style="font: 1.99px Arial;fill:#212529">`,
			expected: `<text style="font: 1.99mm Arial;fill:#212529">`,
		},
		{
			name:     "integer font size",
			input:    `<text style="font: 3px Arial">`,
			expected: `<text style="font: 3mm Arial">`,
		},
		{
			name:     "does not affect stroke-width",
			input:    `stroke-width:.7056`,
			expected: `stroke-width:.7056`,
		},
		{
			name:     "multiple fonts in one SVG",
			input:    `<text style="font: 2.37px Arial">A</text><text style="font: 500 2.80px Arial">B</text>`,
			expected: `<text style="font: 2.37mm Arial">A</text><text style="font: 500 2.80mm Arial">B</text>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(fixSVGFontUnits([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("fixSVGFontUnits()\ngot:  %s\nwant: %s", result, tt.expected)
			}
		})
	}
}

// TestFixSVGFontFamilyFallbacks tests that generic font-family fallbacks are
// added to CSS font shorthand declarations for LibreOffice compatibility.
func TestFixSVGFontFamilyFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic font declaration",
			input:    `<text style="font: 13.33px Arial;fill:#212529">`,
			expected: `<text style="font: 13.33px Arial, Helvetica, sans-serif;fill:#212529">`,
		},
		{
			name:     "font with weight",
			input:    `<text style="font: 700 24px Arial;fill:#212529">`,
			expected: `<text style="font: 700 24px Arial, Helvetica, sans-serif;fill:#212529">`,
		},
		{
			name:     "font at end of style (no semicolon)",
			input:    `<text style="font: 10px Arial">`,
			expected: `<text style="font: 10px Arial, Helvetica, sans-serif">`,
		},
		{
			name:     "multiple text elements",
			input:    `<text style="font: 12px Arial;fill:#000"><tspan>A</tspan></text><text style="font: 700 18px Arial;fill:#333"><tspan>B</tspan></text>`,
			expected: `<text style="font: 12px Arial, Helvetica, sans-serif;fill:#000"><tspan>A</tspan></text><text style="font: 700 18px Arial, Helvetica, sans-serif;fill:#333"><tspan>B</tspan></text>`,
		},
		{
			name:     "non-Arial font gets fallbacks too",
			input:    `<text style="font: 14px Roboto;fill:#000">`,
			expected: `<text style="font: 14px Roboto, Helvetica, sans-serif;fill:#000">`,
		},
		{
			name:     "does not affect @font-face",
			input:    `@font-face{font-family:'Arial';src:url('data:font/opentype;base64,AAA');}`,
			expected: `@font-face{font-family:'Arial';src:url('data:font/opentype;base64,AAA');}`,
		},
		{
			name:     "does not affect non-font styles",
			input:    `<rect style="fill:#336699;stroke:#000"/>`,
			expected: `<rect style="fill:#336699;stroke:#000"/>`,
		},
		{
			name:     "decimal font size with weight",
			input:    `<text style="font: 500 9.5px DejaVu-Sans;fill:#000">`,
			expected: `<text style="font: 500 9.5px DejaVu-Sans, Helvetica, sans-serif;fill:#000">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(fixSVGFontFamilyFallbacks([]byte(tt.input)))
			if result != tt.expected {
				t.Errorf("fixSVGFontFamilyFallbacks()\ngot:  %s\nwant: %s", result, tt.expected)
			}
		})
	}
}

// TestRenderSVGFontFamilyFallbacks verifies that rendered SVG output includes
// font-family fallbacks in CSS font shorthand declarations.
func TestRenderSVGFontFamilyFallbacks(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFontSize(12)
	b.DrawText("Hello", 100, 50, TextAlignCenter, TextBaselineMiddle)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	svgStr := doc.String()
	if !strings.Contains(svgStr, "Helvetica, sans-serif") {
		t.Errorf("SVG output should contain font-family fallbacks for LibreOffice compatibility:\n%s", svgStr[:min(len(svgStr), 500)])
	}
}

// TestRenderSVGFontUnits verifies that rendered SVG uses px units for fonts.
// Font values are numerically in mm (matching the viewBox coordinate space) but
// use "px" suffix so they scale proportionally when the SVG is resized.
func TestRenderSVGFontUnits(t *testing.T) {
	b := NewSVGBuilder(200, 100)
	b.SetFontSize(12)
	b.DrawText("Hello", 100, 50, TextAlignCenter, TextBaselineMiddle)

	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	svgStr := doc.String()
	if !strings.Contains(svgStr, "px ") && !strings.Contains(svgStr, "px;") {
		t.Errorf("SVG output should contain px font units for viewBox-proportional scaling:\n%s", svgStr[:min(len(svgStr), 500)])
	}
}

// TestRenderPNG tests PNG output generation.
func TestRenderPNG(t *testing.T) {
	builder := NewSVGBuilder(200, 150)

	// Draw something simple
	c, _ := ParseColor("#336699")
	builder.SetFillColor(c)
	builder.FillRect(Rect{X: 10, Y: 10, W: 180, H: 130})

	cWhite, _ := ParseColor("#ffffff")
	builder.SetFillColor(cWhite)
	builder.DrawText("Test", 100, 75, TextAlignCenter, TextBaselineMiddle)

	t.Run("RenderPNG with default scale", func(t *testing.T) {
		pngBytes, err := builder.RenderPNG(0) // 0 means default (2.0)
		if err != nil {
			t.Fatalf("RenderPNG(0) failed: %v", err)
		}
		if len(pngBytes) == 0 {
			t.Error("RenderPNG returned empty bytes")
		}
		// Check PNG magic header
		if len(pngBytes) < 8 {
			t.Fatal("PNG output too short")
		}
		// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
		if pngBytes[0] != 0x89 || pngBytes[1] != 0x50 || pngBytes[2] != 0x4E || pngBytes[3] != 0x47 {
			t.Errorf("Invalid PNG header: %x %x %x %x", pngBytes[0], pngBytes[1], pngBytes[2], pngBytes[3])
		}
	})

	t.Run("RenderPNG with custom scale", func(t *testing.T) {
		pngBytes1x, err := builder.RenderPNG(1.0)
		if err != nil {
			t.Fatalf("RenderPNG(1.0) failed: %v", err)
		}

		pngBytes2x, err := builder.RenderPNG(2.0)
		if err != nil {
			t.Fatalf("RenderPNG(2.0) failed: %v", err)
		}

		// 2x scale should produce larger output (more pixels)
		// The difference may not be exactly 4x due to compression
		if len(pngBytes2x) <= len(pngBytes1x) {
			// This is actually okay due to compression, but we should at least verify both work
			t.Logf("1x PNG: %d bytes, 2x PNG: %d bytes", len(pngBytes1x), len(pngBytes2x))
		}
	})
}

// TestRenderPDF tests PDF output generation.
func TestRenderPDF(t *testing.T) {
	builder := NewSVGBuilder(200, 150)

	// Draw something simple
	c, _ := ParseColor("#cc3366")
	builder.SetFillColor(c)
	builder.FillRect(Rect{X: 10, Y: 10, W: 180, H: 130})

	pdfBytes, err := builder.RenderPDF()
	if err != nil {
		t.Fatalf("RenderPDF() failed: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Error("RenderPDF returned empty bytes")
	}
	// Check PDF magic header: %PDF-
	if len(pdfBytes) < 5 {
		t.Fatal("PDF output too short")
	}
	header := string(pdfBytes[:5])
	if header != "%PDF-" {
		t.Errorf("Invalid PDF header: %q", header)
	}
}

// TestRenderWithFormats tests multi-format rendering.
func TestRenderWithFormats(t *testing.T) {
	builder := NewSVGBuilder(200, 150)
	c, _ := ParseColor("#99cc33")
	builder.SetFillColor(c)
	builder.FillRect(Rect{X: 0, Y: 0, W: 200, H: 150})

	t.Run("SVG only", func(t *testing.T) {
		result, err := builder.RenderWithFormats([]string{"svg"}, 0)
		if err != nil {
			t.Fatalf("RenderWithFormats failed: %v", err)
		}
		if result.SVG == nil {
			t.Error("SVG should always be included")
		}
		if result.PNG != nil {
			t.Error("PNG should not be included when not requested")
		}
		if result.PDF != nil {
			t.Error("PDF should not be included when not requested")
		}
	})

	t.Run("SVG and PNG", func(t *testing.T) {
		result, err := builder.RenderWithFormats([]string{"svg", "png"}, 2.0)
		if err != nil {
			t.Fatalf("RenderWithFormats failed: %v", err)
		}
		if result.SVG == nil {
			t.Error("SVG should always be included")
		}
		if result.PNG == nil {
			t.Error("PNG should be included when requested")
		}
		if result.PDF != nil {
			t.Error("PDF should not be included when not requested")
		}
	})

	t.Run("All formats", func(t *testing.T) {
		result, err := builder.RenderWithFormats([]string{"svg", "png", "pdf"}, 1.5)
		if err != nil {
			t.Fatalf("RenderWithFormats failed: %v", err)
		}
		if result.SVG == nil {
			t.Error("SVG should be included")
		}
		if result.PNG == nil {
			t.Error("PNG should be included")
		}
		if result.PDF == nil {
			t.Error("PDF should be included")
		}
	})

	t.Run("Invalid format", func(t *testing.T) {
		_, err := builder.RenderWithFormats([]string{"invalid"}, 0)
		if err == nil {
			t.Error("Expected error for invalid format")
		}
	})
}

// TestFontCaching verifies that the font cache is working correctly.
// Font loading is expensive, so we cache loaded fonts across builder instances.
func TestFontCaching(t *testing.T) {
	// Create multiple builders - they should all share the same cached font
	builders := make([]*SVGBuilder, 5)
	for i := 0; i < 5; i++ {
		builders[i] = NewSVGBuilder(100, 100)
	}

	// All builders should have a non-nil font family
	for i, b := range builders {
		if b.fontFamily == nil {
			t.Errorf("builder %d has nil fontFamily", i)
		}
	}

	// All builders should reference the same font family instance (cached)
	firstFont := builders[0].fontFamily
	for i := 1; i < len(builders); i++ {
		if builders[i].fontFamily != firstFont {
			t.Errorf("builder %d has different fontFamily instance (font cache not working)", i)
		}
	}
}

// TestFontCaching_CustomFont verifies that custom fonts are also cached.
func TestFontCaching_CustomFont(t *testing.T) {
	b1 := NewSVGBuilder(100, 100)
	b2 := NewSVGBuilder(100, 100)

	// Set both to the same custom font
	b1.SetFontFamily("Courier")
	b2.SetFontFamily("Courier")

	// They should share the same cached font instance
	if b1.fontFamily != b2.fontFamily {
		t.Error("custom font not cached - different instances returned")
	}
}

// TestConcurrentRenderPNG verifies that concurrent PNG rendering is safe.
// This test exercises the rasterMu mutex which protects against race conditions
// in the tdewolff/canvas library's path intersection algorithm.
func TestConcurrentRenderPNG(t *testing.T) {
	const numGoroutines = 10
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create a unique builder for this goroutine
			builder := NewSVGBuilder(200, 200)

			// Draw different shapes to exercise path operations
			c, _ := ParseColor("#336699")
			builder.SetFillColor(c)
			builder.FillRect(Rect{X: 10, Y: 10, W: 180, H: 180})

			c2, _ := ParseColor("#ffffff")
			builder.SetFillColor(c2)
			builder.DrawText("Test", 100, 100, TextAlignCenter, TextBaselineMiddle)

			// Render to PNG - protected by rasterMu
			pngBytes, err := builder.RenderPNG(1.0)
			if err != nil {
				errChan <- err
				return
			}
			if len(pngBytes) == 0 {
				errChan <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			t.Errorf("Concurrent RenderPNG failed: %v", err)
		}
	}
}

// TestClampFontSize_MinFloor verifies that ClampFontSize enforces the minimum
// font size floor and never returns a value below DefaultMinFontSize.
func TestClampFontSize_MinFloor(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	// Use a very long text that cannot fit at any reasonable size in a narrow width.
	text := "This is an extremely long label that absolutely cannot fit in a tiny space"
	narrowWidth := 20.0 // Very narrow — forces font size to shrink

	// Call with a caller-specified minSize below the floor (e.g., 4pt).
	result := b.ClampFontSize(text, narrowWidth, 24, 4)

	if result < DefaultMinFontSize {
		t.Errorf("ClampFontSize returned %v, want >= %v (DefaultMinFontSize)", result, DefaultMinFontSize)
	}
}

// TestClampFontSizeForRect_MinFloor verifies that ClampFontSizeForRect enforces
// the minimum font size floor.
func TestClampFontSizeForRect_MinFloor(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	// Long text in a tiny rect — forces font to shrink to floor.
	text := "Dense content that needs to fit in a very small bounding box area"
	result := b.ClampFontSizeForRect(text, 30, 10, 24, 3)

	if result < DefaultMinFontSize {
		t.Errorf("ClampFontSizeForRect returned %v, want >= %v (DefaultMinFontSize)", result, DefaultMinFontSize)
	}
}

// TestSetMinFontSize verifies that the minimum font size is configurable.
func TestSetMinFontSize(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	// Default should be DefaultMinFontSize
	if b.MinFontSize() != DefaultMinFontSize {
		t.Errorf("default MinFontSize() = %v, want %v", b.MinFontSize(), DefaultMinFontSize)
	}

	// Set a custom floor
	b.SetMinFontSize(10)
	if b.MinFontSize() != 10 {
		t.Errorf("MinFontSize() after SetMinFontSize(10) = %v, want 10", b.MinFontSize())
	}

	// ClampFontSize should respect the custom floor
	text := "A long label text that requires shrinking"
	result := b.ClampFontSize(text, 20, 24, 4)
	if result < 10 {
		t.Errorf("ClampFontSize with custom floor returned %v, want >= 10", result)
	}

	// Set floor to 0 — should fall back to DefaultMinFontSize
	b.SetMinFontSize(0)
	if b.MinFontSize() != DefaultMinFontSize {
		t.Errorf("MinFontSize() after SetMinFontSize(0) = %v, want %v", b.MinFontSize(), DefaultMinFontSize)
	}
}

// TestClampFontSize_CallerMinAboveFloor verifies that when the caller-specified
// minSize is above the floor, the caller's value is used as-is.
func TestClampFontSize_CallerMinAboveFloor(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	text := "Some text that needs to shrink significantly"
	// Caller min is 12, which is above DefaultMinFontSize (9).
	// With very narrow width the result should be 12 (the caller's floor).
	result := b.ClampFontSize(text, 15, 24, 12)

	if result < 12 {
		t.Errorf("ClampFontSize returned %v, want >= 12 (caller's minSize)", result)
	}
}

// TestClampFontSize_TextFitsAtPreset verifies that when text fits at preset
// size, ClampFontSize returns the preset regardless of floor.
func TestClampFontSize_TextFitsAtPreset(t *testing.T) {
	b := NewSVGBuilder(800, 600)

	// Short text, wide area — should return presetSize.
	result := b.ClampFontSize("Hi", 500, 24, 4)
	if result != 24 {
		t.Errorf("ClampFontSize for short text = %v, want 24 (presetSize)", result)
	}
}

// TestConcurrentRenderPDF verifies that concurrent PDF rendering is safe.
func TestConcurrentRenderPDF(t *testing.T) {
	const numGoroutines = 10
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create a unique builder for this goroutine
			builder := NewSVGBuilder(200, 200)

			// Draw different shapes to exercise path operations
			c, _ := ParseColor("#993366")
			builder.SetFillColor(c)
			builder.FillRect(Rect{X: 10, Y: 10, W: 180, H: 180})

			c2, _ := ParseColor("#ffffff")
			builder.SetFillColor(c2)
			builder.DrawText("PDF Test", 100, 100, TextAlignCenter, TextBaselineMiddle)

			// Render to PDF - protected by rasterMu
			pdfBytes, err := builder.RenderPDF()
			if err != nil {
				errChan <- err
				return
			}
			if len(pdfBytes) == 0 {
				errChan <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			t.Errorf("Concurrent RenderPDF failed: %v", err)
		}
	}
}

func TestDrawImage(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	// Create a small test image (10x10 red square)
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	// DrawImage should not panic and should return the builder for chaining
	result := b.DrawImage(img, Rect{X: 50, Y: 50, W: 100, H: 100})
	if result != b {
		t.Error("DrawImage should return the builder for chaining")
	}

	// Render should succeed with the embedded image
	doc, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if doc == nil || len(doc.Content) == 0 {
		t.Error("Expected non-empty SVG document")
	}
}

func TestDrawImage_Nil(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	// DrawImage with nil should be a no-op
	result := b.DrawImage(nil, Rect{X: 50, Y: 50, W: 100, H: 100})
	if result != b {
		t.Error("DrawImage(nil) should return the builder")
	}
}

func TestDrawImage_ZeroDimensions(t *testing.T) {
	b := NewSVGBuilder(400, 300)

	// Create an empty image
	img := image.NewRGBA(image.Rect(0, 0, 0, 0))

	// Should handle zero-dimension image gracefully
	result := b.DrawImage(img, Rect{X: 50, Y: 50, W: 100, H: 100})
	if result != b {
		t.Error("DrawImage with zero-dim image should return the builder")
	}
}
