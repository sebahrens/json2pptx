// Package generator provides PPTX file generation from slide specifications.
package generator

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// SVGFallbackImage generates a placeholder PNG image indicating that SVG rendering
// is unavailable. This is used when rsvg-convert or other SVG conversion tools
// are not installed, to avoid leaving placeholders empty.
//
// The image is a light gray rectangle with centered text explaining the issue.
// Width and height are in pixels. Returns PNG-encoded bytes.
func SVGFallbackImage(width, height int) ([]byte, error) {
	// Minimum size for legibility
	if width < 100 {
		width = 100
	}
	if height < 50 {
		height = 50
	}

	// Create a light gray background
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bgColor := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Draw a subtle border
	borderColor := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	drawRectBorder(img, 0, 0, width-1, height-1, borderColor)

	// Draw the SVG unavailable icon (simple X in a box)
	iconSize := min(width, height) / 4
	if iconSize < 16 {
		iconSize = 16
	}
	iconX := (width - iconSize) / 2
	iconY := (height - iconSize) / 3

	iconColor := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	drawXIcon(img, iconX, iconY, iconSize, iconColor)

	// Draw text below the icon
	textColor := color.RGBA{R: 128, G: 128, B: 128, A: 255}
	line1 := "SVG Unavailable"
	line2 := "Install rsvg-convert"

	// Use basic font for simplicity
	face := basicfont.Face7x13

	// Calculate text positions for centering
	line1Width := font.MeasureString(face, line1).Ceil()
	line2Width := font.MeasureString(face, line2).Ceil()
	textY := iconY + iconSize + 20

	// Draw first line
	if line1Width < width-10 && textY < height-20 {
		drawText(img, (width-line1Width)/2, textY, line1, textColor, face)
	}

	// Draw second line (smaller info)
	textY += 16
	if line2Width < width-10 && textY < height-10 {
		drawText(img, (width-line2Width)/2, textY, line2, textColor, face)
	}

	// Encode to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// drawRectBorder draws a rectangle border on the image.
func drawRectBorder(img *image.RGBA, x1, y1, x2, y2 int, c color.Color) {
	// Top and bottom
	for x := x1; x <= x2; x++ {
		img.Set(x, y1, c)
		img.Set(x, y2, c)
	}
	// Left and right
	for y := y1; y <= y2; y++ {
		img.Set(x1, y, c)
		img.Set(x2, y, c)
	}
}

// drawXIcon draws an X symbol inside a square.
func drawXIcon(img *image.RGBA, x, y, size int, c color.Color) {
	// Draw a square border
	drawRectBorder(img, x, y, x+size, y+size, c)

	// Draw diagonals (X)
	for i := 0; i <= size; i++ {
		// Main diagonal
		img.Set(x+i, y+i, c)
		// Anti-diagonal
		img.Set(x+size-i, y+i, c)
	}
}

// DiagramPlaceholderImage generates a placeholder PNG for chart/diagram validation
// failures. Instead of leaving the slide blank, this shows a professional placeholder
// indicating the diagram type and that data was unavailable.
//
// diagramType is the snake_case type (e.g. "bar_chart", "gantt").
// Width and height are in pixels. Returns PNG-encoded bytes.
func DiagramPlaceholderImage(width, height int, diagramType string) ([]byte, error) {
	if width < 100 {
		width = 100
	}
	if height < 50 {
		height = 50
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Light warm-gray background (slightly warmer than SVG fallback)
	bgColor := color.RGBA{R: 245, G: 243, B: 240, A: 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Subtle border
	borderColor := color.RGBA{R: 210, G: 205, B: 200, A: 255}
	drawRectBorder(img, 0, 0, width-1, height-1, borderColor)

	// Draw a simple chart icon (three vertical bars)
	iconColor := color.RGBA{R: 190, G: 185, B: 180, A: 255}
	barWidth := min(width, height) / 12
	if barWidth < 4 {
		barWidth = 4
	}
	gap := barWidth / 2
	if gap < 2 {
		gap = 2
	}
	totalIconWidth := 3*barWidth + 2*gap
	iconStartX := (width - totalIconWidth) / 2
	iconBaseY := height / 2
	barHeights := []int{height / 5, height / 3, height / 4}
	for i, bh := range barHeights {
		bx := iconStartX + i*(barWidth+gap)
		by := iconBaseY - bh
		for dy := 0; dy < bh; dy++ {
			for dx := 0; dx < barWidth; dx++ {
				img.Set(bx+dx, by+dy, iconColor)
			}
		}
	}

	// Text below icon
	face := basicfont.Face7x13
	textColor := color.RGBA{R: 140, G: 135, B: 130, A: 255}

	label := formatDiagramType(diagramType)
	line1 := label
	line2 := "Data unavailable"

	line1Width := font.MeasureString(face, line1).Ceil()
	line2Width := font.MeasureString(face, line2).Ceil()
	textY := iconBaseY + 16

	if line1Width < width-10 && textY < height-20 {
		drawText(img, (width-line1Width)/2, textY, line1, textColor, face)
	}
	textY += 16
	if line2Width < width-10 && textY < height-10 {
		drawText(img, (width-line2Width)/2, textY, line2, textColor, face)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// formatDiagramType converts a snake_case diagram type to a human-readable label.
// e.g. "bar_chart" -> "Bar Chart", "matrix_2x2" -> "Matrix 2x2"
func formatDiagramType(t string) string {
	if t == "" {
		return "Diagram"
	}
	result := make([]byte, 0, len(t))
	capitalizeNext := true
	for i := 0; i < len(t); i++ {
		c := t[i]
		if c == '_' {
			result = append(result, ' ')
			capitalizeNext = true
		} else if capitalizeNext && c >= 'a' && c <= 'z' {
			result = append(result, c-32) // uppercase
			capitalizeNext = false
		} else {
			result = append(result, c)
			capitalizeNext = false
		}
	}
	return string(result)
}

// drawText draws text at the specified position using the basic font.
func drawText(img *image.RGBA, x, y int, text string, c color.Color, face font.Face) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}
