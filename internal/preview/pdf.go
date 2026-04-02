// Package preview provides PDF preview output for presentations.
// It renders slide content to a multi-page PDF using tdewolff/canvas,
// providing a quick preview without requiring PowerPoint.
//
// Unlike the PPTX generator, this renders directly from parsed slide definitions
// without going through the layout/placeholder mapping pipeline.
package preview

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/pdf"

	"github.com/ahrens/svggen"
	"github.com/sebahrens/json2pptx/internal/types"
)

const (
	// Slide dimensions in mm (16:9 at 254mm width ≈ 10 inches)
	slideWidthMM  = 254.0
	slideHeightMM = 142.875 // 254 × 9/16

	// Margins in mm
	marginMM = 12.7 // ~0.5 inch

	// Font sizes in mm
	titleFontMM = 8.5  // ~24pt
	bodyFontMM  = 4.2  // ~12pt
	smallFontMM = 3.17 // ~9pt

	// Conversion factor: 1 point = 25.4/72 mm
	ptToMM = 25.4 / 72.0

	// Spacing
	lineSpacingMM    = 1.5
	bulletIndentMM   = 6.0
	paragraphGapMM   = 3.0
	titleBottomGapMM = 6.0
)

// Request contains inputs for PDF generation.
type Request struct {
	OutputPath   string
	Presentation *types.PresentationDefinition
	Theme        *types.ThemeInfo
}

// Result contains the output of PDF generation.
type Result struct {
	OutputPath string
	PageCount  int
}

// GeneratePDF renders a presentation to a multi-page PDF file.
func GeneratePDF(req Request) (*Result, error) {
	if req.Presentation == nil || len(req.Presentation.Slides) == 0 {
		return nil, fmt.Errorf("preview: no slides to render")
	}

	f, err := os.Create(req.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("preview: create output file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := renderPDF(f, req); err != nil {
		_ = os.Remove(req.OutputPath)
		return nil, err
	}

	return &Result{
		OutputPath: req.OutputPath,
		PageCount:  len(req.Presentation.Slides),
	}, nil
}

// renderPDF writes the multi-page PDF to w.
func renderPDF(w io.Writer, req Request) error {
	p := pdf.New(w, slideWidthMM, slideHeightMM, nil)

	ff := loadFont(req.Theme)

	for i, slide := range req.Presentation.Slides {
		if i > 0 {
			p.NewPage(slideWidthMM, slideHeightMM)
		}

		ctx := canvas.NewContext(p)
		renderSlide(ctx, slide, req.Theme, ff)
	}

	return p.Close()
}

// loadFont returns a font family from the theme, falling back to system defaults.
func loadFont(theme *types.ThemeInfo) *canvas.FontFamily {
	names := []string{"Arial", "Helvetica", "Liberation Sans"}
	if theme != nil && theme.BodyFont != "" {
		names = append([]string{theme.BodyFont}, names...)
	}

	for _, name := range names {
		ff := canvas.NewFontFamily(name)
		if err := ff.LoadSystemFont(name, canvas.FontRegular); err == nil {
			return ff
		}
	}

	ff := canvas.NewFontFamily("sans-serif")
	_ = ff.LoadSystemFont("DejaVu Sans", canvas.FontRegular)
	return ff
}

// renderSlide draws a single slide onto the canvas context.
func renderSlide(ctx *canvas.Context, slide types.SlideDefinition, theme *types.ThemeInfo, ff *canvas.FontFamily) {
	// Background
	ctx.SetFillColor(color.White)
	ctx.DrawPath(0, 0, canvas.Rectangle(slideWidthMM, slideHeightMM))

	// Border
	ctx.SetStrokeColor(color.RGBA{R: 200, G: 200, B: 200, A: 255})
	ctx.SetStrokeWidth(0.3)
	ctx.DrawPath(0, 0, canvas.Rectangle(slideWidthMM, slideHeightMM))

	// Accent color
	accentColor := color.RGBA{R: 0, G: 100, B: 180, A: 255}
	if theme != nil && len(theme.Colors) > 0 {
		if c, err := parseHexColor(theme.Colors[0].RGB); err == nil {
			accentColor = c
		}
	}

	textColor := color.RGBA{R: 51, G: 51, B: 51, A: 255}

	x := marginMM
	y := slideHeightMM - marginMM
	maxWidth := slideWidthMM - 2*marginMM

	// Title
	if slide.Title != "" {
		titleFace := ff.Face(titleFontMM, accentColor, canvas.FontRegular, canvas.FontNormal)
		y = drawText(ctx, titleFace, slide.Title, x, y, maxWidth)
		y -= titleBottomGapMM
	}

	content := slide.Content
	hasDiagram := content.DiagramSpec != nil
	hasText := content.Body != "" || len(content.Bullets) > 0 || len(content.BulletGroups) > 0

	bodyFace := ff.Face(bodyFontMM, textColor, canvas.FontRegular, canvas.FontNormal)
	bulletFace := ff.Face(bodyFontMM, textColor, canvas.FontRegular, canvas.FontNormal)
	boldFace := ff.Face(bodyFontMM, textColor, canvas.FontBold, canvas.FontNormal)

	if hasText && hasDiagram {
		// Two-column layout: text left, diagram right
		colWidth := (maxWidth - marginMM) / 2
		leftY := y
		leftY = renderTextContent(ctx, content, bodyFace, bulletFace, boldFace, x, leftY, colWidth)
		_ = leftY

		rightX := x + colWidth + marginMM
		renderDiagram(ctx, content.DiagramSpec, ff, rightX, y, colWidth, y-marginMM, theme)
	} else if hasDiagram {
		// Full-width diagram
		renderDiagram(ctx, content.DiagramSpec, ff, x, y, maxWidth, y-marginMM, theme)
	} else if slide.Type == types.SlideTypeTwoColumn {
		// Two-column text layout
		colWidth := (maxWidth - marginMM) / 2
		leftY := y
		rightY := y

		if len(content.Left) > 0 {
			for _, b := range content.Left {
				drawText(ctx, bulletFace, "\u2022", x, leftY, bulletIndentMM)
				leftY = drawText(ctx, bulletFace, b, x+bulletIndentMM, leftY, colWidth-bulletIndentMM)
				leftY -= lineSpacingMM
			}
		}

		rightX := x + colWidth + marginMM
		if len(content.Right) > 0 {
			for _, b := range content.Right {
				drawText(ctx, bulletFace, "\u2022", rightX, rightY, bulletIndentMM)
				rightY = drawText(ctx, bulletFace, b, rightX+bulletIndentMM, rightY, colWidth-bulletIndentMM)
				rightY -= lineSpacingMM
			}
		}
	} else {
		// Standard text content
		renderTextContent(ctx, content, bodyFace, bulletFace, boldFace, x, y, maxWidth)
	}

	// Source note
	if slide.Source != "" {
		noteFace := ff.Face(smallFontMM, color.RGBA{R: 128, G: 128, B: 128, A: 255}, canvas.FontRegular, canvas.FontNormal)
		drawText(ctx, noteFace, "Source: "+slide.Source, x, marginMM+smallFontMM, maxWidth)
	}
}

// renderTextContent draws body text, bullets, and bullet groups.
func renderTextContent(ctx *canvas.Context, content types.SlideContent, bodyFace, bulletFace, boldFace *canvas.FontFace, x, y, maxWidth float64) float64 {
	// Body text
	if content.Body != "" {
		y = drawText(ctx, bodyFace, content.Body, x, y, maxWidth)
		y -= paragraphGapMM
	}

	// Bullet groups (hierarchical)
	if len(content.BulletGroups) > 0 {
		for _, g := range content.BulletGroups {
			if g.Header != "" {
				y = drawText(ctx, boldFace, g.Header, x, y, maxWidth)
				y -= lineSpacingMM
			}
			for _, b := range g.Bullets {
				drawText(ctx, bulletFace, "\u2022", x+bulletIndentMM/2, y, bulletIndentMM)
				y = drawText(ctx, bulletFace, b, x+bulletIndentMM, y, maxWidth-bulletIndentMM)
				y -= lineSpacingMM
			}
		}
	} else if len(content.Bullets) > 0 {
		// Flat bullets
		for _, b := range content.Bullets {
			drawText(ctx, bulletFace, "\u2022", x, y, bulletIndentMM)
			y = drawText(ctx, bulletFace, b, x+bulletIndentMM, y, maxWidth-bulletIndentMM)
			y -= lineSpacingMM
		}
	}

	// Table (simple text rendering)
	if content.TableRaw != "" {
		y -= paragraphGapMM
		grayFace := bodyFace // reuse body face for table
		for _, line := range strings.Split(content.TableRaw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "|---") || strings.HasPrefix(line, "| ---") {
				continue
			}
			y = drawText(ctx, grayFace, line, x, y, maxWidth)
			y -= lineSpacingMM / 2
		}
	}

	return y
}

// renderDiagram renders a diagram spec as a PNG image embedded in the PDF.
func renderDiagram(ctx *canvas.Context, spec *types.DiagramSpec, ff *canvas.FontFamily, x, y, maxWidth, maxHeight float64, theme *types.ThemeInfo) {
	if spec == nil {
		return
	}

	var themeColors []types.ThemeColor
	if theme != nil {
		themeColors = theme.Colors
	}

	pngBytes, err := renderDiagramPNG(spec, themeColors)
	if err != nil {
		slog.Warn("preview: diagram render failed", "type", spec.Type, "error", err)
		gray := color.RGBA{R: 128, G: 128, B: 128, A: 255}
		face := ff.Face(bodyFontMM, gray, canvas.FontRegular, canvas.FontNormal)
		drawText(ctx, face, fmt.Sprintf("[%s diagram]", spec.Type), x, y, maxWidth)
		return
	}

	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		slog.Warn("preview: PNG decode failed", "error", err)
		return
	}

	imgW := float64(img.Bounds().Dx())
	imgH := float64(img.Bounds().Dy())

	targetW := maxWidth
	targetH := maxHeight
	if targetH <= 0 {
		targetH = slideHeightMM / 2
	}

	scaleX := targetW / (imgW * ptToMM)
	scaleY := targetH / (imgH * ptToMM)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	drawW := imgW * ptToMM * scale
	drawH := imgH * ptToMM * scale

	imgY := y - drawH
	ppmm := imgW / drawW
	ctx.DrawImage(x, imgY, img, canvas.DPMM(ppmm))
}

// renderDiagramPNG renders a DiagramSpec to PNG bytes using svggen.
func renderDiagramPNG(spec *types.DiagramSpec, themeColors []types.ThemeColor) ([]byte, error) {
	style := svggen.StyleSpec{}
	if len(themeColors) > 0 {
		inputs := make([]svggen.ThemeColorInput, len(themeColors))
		for i, tc := range themeColors {
			inputs[i] = svggen.ThemeColorInput{Name: tc.Name, RGB: tc.RGB}
		}
		style.ThemeColors = inputs
	}

	output := svggen.OutputSpec{
		Format:  "png",
		FitMode: spec.FitMode,
		Width:   spec.Width,
		Height:  spec.Height,
	}
	if output.Width <= 0 {
		output.Width = types.DefaultChartWidth
	}
	if output.Height <= 0 {
		output.Height = types.DefaultChartHeight
	}
	if spec.Scale > 0 {
		output.Scale = spec.Scale
	} else {
		output.Scale = types.DefaultMinScale
	}

	req := &svggen.RequestEnvelope{
		Type:   spec.Type,
		Title:  spec.Title,
		Data:   spec.Data,
		Output: output,
		Style:  style,
	}

	result, err := svggen.RenderMultiFormat(req, "png")
	if err != nil {
		return nil, err
	}
	return result.PNG, nil
}

// drawText draws wrapped text and returns the new Y position.
func drawText(ctx *canvas.Context, face *canvas.FontFace, text string, x, y, maxWidth float64) float64 {
	if text == "" || maxWidth <= 0 {
		return y
	}

	lines := wrapText(text, face, maxWidth)
	lineHeight := face.Metrics().LineHeight

	for _, line := range lines {
		txt := canvas.NewTextLine(face, line, canvas.Left)
		ctx.DrawText(x, y, txt)
		y -= lineHeight + lineSpacingMM
	}

	return y
}

// wrapText breaks text into lines that fit within maxWidth.
func wrapText(text string, face *canvas.FontFace, maxWidth float64) []string {
	if maxWidth <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		test := current + " " + word
		if face.TextWidth(test) > maxWidth && current != "" {
			lines = append(lines, current)
			current = word
		} else {
			current = test
		}
	}
	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

// parseHexColor parses a hex color string like "#FF0000" into a color.RGBA.
func parseHexColor(hex string) (color.RGBA, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid hex color: %s", hex)
	}

	var r, g, b uint8
	_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}, nil
}
