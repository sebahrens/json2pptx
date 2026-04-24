package svggen

import (
	"strings"
	"unicode"

	"github.com/tdewolff/canvas"
)

// TextLine represents a single line of text with its dimensions.
type TextLine struct {
	Text   string
	Width  float64
	Height float64
}

// TextBlock represents multiple lines of text with layout information.
type TextBlock struct {
	Lines       []TextLine
	TotalWidth  float64 // Width of the widest line
	LineHeight  float64 // Height of a single line
	TotalHeight float64 // Total height including line spacing
}

// TextOverflow specifies how to handle text overflow.
type TextOverflow int

const (
	// TextOverflowClip clips text at the boundary without ellipsis.
	TextOverflowClip TextOverflow = iota
	// TextOverflowEllipsis adds ellipsis (...) when text overflows.
	TextOverflowEllipsis
	// TextOverflowWordBreak breaks at word boundaries even if it means more overflow.
	TextOverflowWordBreak
)

// VerticalAlign specifies vertical alignment within a box.
type VerticalAlign int

const (
	VerticalAlignTop VerticalAlign = iota
	VerticalAlignMiddle
	VerticalAlignBottom
)

// HorizontalAlign specifies horizontal alignment within a box.
type HorizontalAlign int

const (
	HorizontalAlignLeft HorizontalAlign = iota
	HorizontalAlignCenter
	HorizontalAlignRight
)

// BoxAlign combines horizontal and vertical alignment.
type BoxAlign struct {
	Horizontal HorizontalAlign
	Vertical   VerticalAlign
}

// Common alignments
var (
	AlignTopLeft      = BoxAlign{HorizontalAlignLeft, VerticalAlignTop}
	AlignTopCenter    = BoxAlign{HorizontalAlignCenter, VerticalAlignTop}
	AlignTopRight     = BoxAlign{HorizontalAlignRight, VerticalAlignTop}
	AlignMiddleLeft   = BoxAlign{HorizontalAlignLeft, VerticalAlignMiddle}
	AlignCenter       = BoxAlign{HorizontalAlignCenter, VerticalAlignMiddle}
	AlignMiddleRight  = BoxAlign{HorizontalAlignRight, VerticalAlignMiddle}
	AlignBottomLeft   = BoxAlign{HorizontalAlignLeft, VerticalAlignBottom}
	AlignBottomCenter = BoxAlign{HorizontalAlignCenter, VerticalAlignBottom}
	AlignBottomRight  = BoxAlign{HorizontalAlignRight, VerticalAlignBottom}
)

// WrapText wraps text to fit within the specified width, returning multiple lines.
// It breaks at word boundaries when possible. The maxWidth is in points.
func (b *SVGBuilder) WrapText(text string, maxWidth float64) TextBlock {
	if text == "" || maxWidth <= 0 {
		return TextBlock{}
	}

	// Get line height from font metrics
	face := b.getFontFace()
	if face == nil {
		return TextBlock{}
	}
	metrics := face.Metrics()
	lineHeight := (metrics.Ascent + metrics.Descent) * mmToPt

	// Split input into paragraphs (preserve explicit line breaks)
	paragraphs := strings.Split(text, "\n")
	var lines []TextLine

	for _, para := range paragraphs {
		if para == "" {
			// Empty paragraph = blank line
			lines = append(lines, TextLine{Text: "", Width: 0, Height: lineHeight})
			continue
		}

		// Wrap this paragraph
		paraLines := b.wrapParagraph(para, maxWidth, lineHeight)
		lines = append(lines, paraLines...)
	}

	// Calculate totals
	var totalWidth float64
	for _, line := range lines {
		if line.Width > totalWidth {
			totalWidth = line.Width
		}
	}

	totalHeight := float64(len(lines)) * lineHeight
	if b.style != nil && b.style.Typography != nil {
		// Apply line height multiplier
		totalHeight = float64(len(lines)) * lineHeight * b.style.Typography.LineHeight
	}

	return TextBlock{
		Lines:       lines,
		TotalWidth:  totalWidth,
		LineHeight:  lineHeight,
		TotalHeight: totalHeight,
	}
}

// lineBuilder helps build lines while wrapping text.
type lineBuilder struct {
	text  strings.Builder
	width float64
}

// wrapParagraph wraps a single paragraph of text.
func (b *SVGBuilder) wrapParagraph(text string, maxWidth float64, lineHeight float64) []TextLine {
	words := splitIntoWords(text)
	if len(words) == 0 {
		return nil
	}

	spaceWidth, _ := b.MeasureText(" ")
	var lines []TextLine
	current := &lineBuilder{}

	for _, word := range words {
		wordWidth, _ := b.MeasureText(word)
		lines = b.addWordToLine(lines, current, word, wordWidth, spaceWidth, maxWidth, lineHeight)
	}

	// Finish last line
	if current.text.Len() > 0 {
		lines = append(lines, TextLine{
			Text:   current.text.String(),
			Width:  current.width,
			Height: lineHeight,
		})
	}

	return lines
}

// addWordToLine adds a word to the current line or starts a new line if needed.
func (b *SVGBuilder) addWordToLine(lines []TextLine, current *lineBuilder, word string, wordWidth, spaceWidth, maxWidth, lineHeight float64) []TextLine {
	if current.text.Len() == 0 {
		return b.startLineWithWord(lines, current, word, wordWidth, maxWidth, lineHeight)
	}

	// Check if word fits on current line
	newWidth := current.width + spaceWidth + wordWidth
	if newWidth <= maxWidth {
		current.text.WriteString(" ")
		current.text.WriteString(word)
		current.width = newWidth
		return lines
	}

	// Finish current line and start new one
	lines = append(lines, TextLine{
		Text:   current.text.String(),
		Width:  current.width,
		Height: lineHeight,
	})

	current.text.Reset()
	current.width = 0
	return b.startLineWithWord(lines, current, word, wordWidth, maxWidth, lineHeight)
}

// startLineWithWord starts a new line with the given word, breaking if necessary.
func (b *SVGBuilder) startLineWithWord(lines []TextLine, current *lineBuilder, word string, wordWidth, maxWidth, lineHeight float64) []TextLine {
	if wordWidth <= maxWidth {
		current.text.WriteString(word)
		current.width = wordWidth
		return lines
	}

	// Word is too long, break it
	brokenLines := b.breakLongWord(word, maxWidth, lineHeight)
	if len(brokenLines) == 0 {
		return lines
	}

	// Add all but last line to output
	lines = append(lines, brokenLines[:len(brokenLines)-1]...)

	// Start current line with last fragment
	lastLine := brokenLines[len(brokenLines)-1]
	current.text.WriteString(lastLine.Text)
	current.width = lastLine.Width
	return lines
}

// breakLongWord handles a word that is too long to fit on a single line.
// It splits the word across multiple lines with a hyphen at each break point,
// so long words like "Engineering" render fully instead of being truncated.
// Break positions prefer vowel-consonant boundaries to approximate syllable
// breaks (e.g., "Inte-grated" instead of "Integr-ated").
func (b *SVGBuilder) breakLongWord(word string, maxWidth float64, lineHeight float64) []TextLine {
	hyphen := "-"
	hyphenWidth, _ := b.MeasureText(hyphen)

	runes := []rune(word)
	if len(runes) == 0 {
		return nil
	}

	var lines []TextLine
	start := 0

	for start < len(runes) {
		isLast := false
		var currentWidth float64
		cutIdx := start

		for i := start; i < len(runes); i++ {
			charWidth, _ := b.MeasureText(string(runes[i]))
			// For non-last segments, reserve space for the hyphen.
			if i+1 < len(runes) {
				if currentWidth+charWidth+hyphenWidth > maxWidth && i > start {
					break
				}
			} else {
				// Last character — no hyphen needed.
				if currentWidth+charWidth > maxWidth && i > start {
					break
				}
				isLast = true
			}
			currentWidth += charWidth
			cutIdx = i + 1
		}

		// Force at least one character per line to avoid infinite loop.
		if cutIdx <= start {
			cutIdx = start + 1
		}

		if cutIdx >= len(runes) {
			isLast = true
		}

		// For non-final segments, prefer a better break position at a
		// vowel-consonant boundary to approximate syllable breaks.
		if !isLast && cutIdx-start >= 4 {
			cutIdx = preferBreakPosition(runes, start, cutIdx)
		}

		segment := string(runes[start:cutIdx])
		if isLast {
			w, _ := b.MeasureText(segment)
			lines = append(lines, TextLine{Text: segment, Width: w, Height: lineHeight})
		} else {
			segWithHyphen := segment + hyphen
			w, _ := b.MeasureText(segWithHyphen)
			lines = append(lines, TextLine{Text: segWithHyphen, Width: w, Height: lineHeight})
		}

		start = cutIdx
	}

	return lines
}

// preferBreakPosition looks backwards from maxPos for a better word break
// position. It prefers breaking at vowel-consonant boundaries, which roughly
// approximates syllable boundaries in English (e.g., "Inte-grated" instead
// of "Integr-ated"). Returns the original maxPos if no better position found.
func preferBreakPosition(runes []rune, start, maxPos int) int {
	// Only look back a few characters to keep lines reasonably balanced.
	const lookBack = 3
	// Ensure at least 2 characters remain on the current line.
	minPos := start + 2

	for offset := 0; offset <= lookBack; offset++ {
		pos := maxPos - offset
		if pos <= minPos || pos >= len(runes)-1 {
			continue
		}
		prev := unicode.ToLower(runes[pos-1])
		curr := unicode.ToLower(runes[pos])
		// Break after a vowel when followed by a consonant letter.
		if isBreakVowel(prev) && unicode.IsLetter(curr) && !isBreakVowel(curr) {
			// Ensure the remaining segment has at least 2 characters.
			if len(runes)-pos >= 2 {
				return pos
			}
		}
	}
	return maxPos
}

// isBreakVowel returns true for Latin vowels used in word break heuristics.
func isBreakVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// splitIntoWords splits text into words, handling multiple spaces.
func splitIntoWords(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

// TruncateText truncates text to fit within maxWidth, adding ellipsis if needed.
// Returns the truncated text and its width.
func (b *SVGBuilder) TruncateText(text string, maxWidth float64, overflow TextOverflow) (string, float64) {
	if text == "" || maxWidth <= 0 {
		return "", 0
	}

	// First check if text already fits
	width, _ := b.MeasureText(text)
	if width <= maxWidth {
		return text, width
	}

	switch overflow {
	case TextOverflowClip:
		return b.truncateClip(text, maxWidth)
	case TextOverflowEllipsis:
		return b.truncateEllipsis(text, maxWidth)
	case TextOverflowWordBreak:
		return b.truncateWordBreak(text, maxWidth)
	default:
		return b.truncateEllipsis(text, maxWidth)
	}
}

// truncateClip truncates text at the exact boundary.
func (b *SVGBuilder) truncateClip(text string, maxWidth float64) (string, float64) {
	runes := []rune(text)
	var currentWidth float64

	for i, r := range runes {
		charWidth, _ := b.MeasureText(string(r))
		if currentWidth+charWidth > maxWidth {
			if i == 0 {
				return "", 0
			}
			result := string(runes[:i])
			resultWidth, _ := b.MeasureText(result)
			return result, resultWidth
		}
		currentWidth += charWidth
	}

	return text, currentWidth
}

// truncateEllipsis truncates text and adds "..." at the end.
func (b *SVGBuilder) truncateEllipsis(text string, maxWidth float64) (string, float64) {
	ellipsis := "..."
	ellipsisWidth, _ := b.MeasureText(ellipsis)

	// If ellipsis alone doesn't fit, return empty
	if ellipsisWidth >= maxWidth {
		return "", 0
	}

	availableWidth := maxWidth - ellipsisWidth
	runes := []rune(text)
	var currentWidth float64

	for i, r := range runes {
		charWidth, _ := b.MeasureText(string(r))
		if currentWidth+charWidth > availableWidth {
			if i == 0 {
				return ellipsis, ellipsisWidth
			}
			result := string(runes[:i]) + ellipsis
			resultWidth, _ := b.MeasureText(result)
			return result, resultWidth
		}
		currentWidth += charWidth
	}

	// Text fits with ellipsis (shouldn't reach here normally)
	return text, currentWidth
}

// truncateWordBreak truncates at word boundary.
func (b *SVGBuilder) truncateWordBreak(text string, maxWidth float64) (string, float64) {
	ellipsis := "..."
	ellipsisWidth, _ := b.MeasureText(ellipsis)

	if ellipsisWidth >= maxWidth {
		return "", 0
	}

	words := splitIntoWords(text)
	if len(words) == 0 {
		return "", 0
	}

	availableWidth := maxWidth - ellipsisWidth
	var result strings.Builder
	var currentWidth float64
	spaceWidth, _ := b.MeasureText(" ")

	for i, word := range words {
		wordWidth, _ := b.MeasureText(word)

		if i == 0 {
			if wordWidth > availableWidth {
				// First word doesn't fit, fall back to char truncate
				return b.truncateEllipsis(text, maxWidth)
			}
			result.WriteString(word)
			currentWidth = wordWidth
		} else {
			newWidth := currentWidth + spaceWidth + wordWidth
			if newWidth > availableWidth {
				// This word doesn't fit, stop here
				break
			}
			result.WriteString(" ")
			result.WriteString(word)
			currentWidth = newWidth
		}
	}

	finalText := result.String() + ellipsis
	finalWidth, _ := b.MeasureText(finalText)
	return finalText, finalWidth
}

// AlignTextInRect calculates the position to draw text within a rectangle.
// Returns the x, y coordinates for drawing text with the specified alignment.
func (b *SVGBuilder) AlignTextInRect(text string, rect Rect, align BoxAlign) (x, y float64) {
	width, height := b.MeasureText(text)

	// Calculate X position
	switch align.Horizontal {
	case HorizontalAlignLeft:
		x = rect.X
	case HorizontalAlignCenter:
		x = rect.X + (rect.W-width)/2
	case HorizontalAlignRight:
		x = rect.X + rect.W - width
	}

	// Calculate Y position (using baseline-relative positioning)
	// Get font metrics for accurate positioning
	face := b.getFontFace()
	var ascent float64
	if face != nil {
		metrics := face.Metrics()
		ascent = metrics.Ascent * mmToPt
	} else {
		// Fallback: assume ascent is ~80% of height
		ascent = height * 0.8
	}

	switch align.Vertical {
	case VerticalAlignTop:
		y = rect.Y + ascent
	case VerticalAlignMiddle:
		y = rect.Y + (rect.H+height)/2 - (height - ascent)
	case VerticalAlignBottom:
		y = rect.Y + rect.H - (height - ascent)
	}

	return x, y
}

// AlignBlockInRect calculates the position to draw a text block within a rectangle.
// Returns the starting x, y coordinates for drawing the first line.
func (b *SVGBuilder) AlignBlockInRect(block TextBlock, rect Rect, align BoxAlign) (x, y float64) {
	if len(block.Lines) == 0 {
		return rect.X, rect.Y
	}

	// Calculate X position based on widest line
	switch align.Horizontal {
	case HorizontalAlignLeft:
		x = rect.X
	case HorizontalAlignCenter:
		x = rect.X + rect.W/2 // Lines will be centered individually
	case HorizontalAlignRight:
		x = rect.X + rect.W // Lines will be right-aligned individually
	}

	// Calculate Y position for entire block
	switch align.Vertical {
	case VerticalAlignTop:
		y = rect.Y + block.LineHeight
	case VerticalAlignMiddle:
		y = rect.Y + (rect.H-block.TotalHeight)/2 + block.LineHeight
	case VerticalAlignBottom:
		y = rect.Y + rect.H - block.TotalHeight + block.LineHeight
	}

	return x, y
}

// DrawTextBlock draws a text block at the specified position with alignment.
func (b *SVGBuilder) DrawTextBlock(block TextBlock, startX, startY float64, hAlign HorizontalAlign) *SVGBuilder {
	lineSpacing := block.LineHeight
	if b.style != nil && b.style.Typography != nil {
		lineSpacing = block.LineHeight * b.style.Typography.LineHeight
	}

	for i, line := range block.Lines {
		if line.Text == "" {
			continue
		}

		// Calculate x position for this line based on alignment
		var x float64
		switch hAlign {
		case HorizontalAlignLeft:
			x = startX
		case HorizontalAlignCenter:
			x = startX // Text will be centered around this point
		case HorizontalAlignRight:
			x = startX // Text will be right-aligned to this point
		}

		// Convert to TextAlign
		var align TextAlign
		switch hAlign {
		case HorizontalAlignLeft:
			align = TextAlignLeft
		case HorizontalAlignCenter:
			align = TextAlignCenter
		case HorizontalAlignRight:
			align = TextAlignRight
		}

		y := startY + float64(i)*lineSpacing
		b.DrawText(line.Text, x, y, align, TextBaselineAlphabetic)
	}

	return b
}

// DrawWrappedText wraps and draws text within a rectangle.
// If the wrapped text exceeds the rectangle height, lines that do not fit are
// dropped and the last visible line is truncated with an ellipsis ("…").
func (b *SVGBuilder) DrawWrappedText(text string, rect Rect, align BoxAlign) *SVGBuilder {
	block := b.WrapText(text, rect.W)
	if len(block.Lines) == 0 {
		return b
	}

	// Calculate how many lines fit within the rect height.
	lineSpacing := block.LineHeight
	if b.style != nil && b.style.Typography != nil {
		lineSpacing = block.LineHeight * b.style.Typography.LineHeight
	}
	if lineSpacing > 0 && block.TotalHeight > rect.H && len(block.Lines) > 1 {
		// Determine max lines that fit. The first line takes LineHeight,
		// subsequent lines take lineSpacing each.
		maxLines := int(rect.H / lineSpacing)
		if maxLines < 1 {
			maxLines = 1
		}
		if maxLines < len(block.Lines) {
			block.Lines = block.Lines[:maxLines]
			// Truncate the last visible line with ellipsis
			last := &block.Lines[maxLines-1]
			last.Text = b.TruncateToWidth(last.Text+"…", rect.W)
			// Recalculate TotalHeight
			block.TotalHeight = float64(len(block.Lines)) * lineSpacing
		}
	}

	x, y := b.AlignBlockInRect(block, rect, align)
	return b.DrawTextBlock(block, x, y, align.Horizontal)
}

// getFontFace returns the current font face, or nil if the font isn't available.
func (b *SVGBuilder) getFontFace() *canvas.FontFace {
	face, err := b.safeFace(colorToRGBA(b.style.Palette.TextPrimary))
	if err != nil {
		if b.fontErr == nil {
			b.fontErr = err
		}
		return nil
	}
	return face
}

// =============================================================================
// Box Layout Helpers
// =============================================================================

// Box represents a layout box with optional padding.
type Box struct {
	Bounds  Rect
	Padding BoxPadding
}

// BoxPadding represents padding for a box.
type BoxPadding struct {
	Top, Right, Bottom, Left float64
}

// UniformPadding creates padding with the same value on all sides.
func UniformPadding(p float64) BoxPadding {
	return BoxPadding{Top: p, Right: p, Bottom: p, Left: p}
}

// SymmetricPadding creates padding with separate horizontal and vertical values.
func SymmetricPadding(vertical, horizontal float64) BoxPadding {
	return BoxPadding{Top: vertical, Right: horizontal, Bottom: vertical, Left: horizontal}
}

// ContentRect returns the inner content rectangle after applying padding.
func (b Box) ContentRect() Rect {
	return b.Bounds.Inset(b.Padding.Top, b.Padding.Right, b.Padding.Bottom, b.Padding.Left)
}

// NewBox creates a new box from a rectangle with optional padding.
func NewBox(bounds Rect, padding BoxPadding) Box {
	return Box{Bounds: bounds, Padding: padding}
}

// SplitHorizontal splits a rectangle into n equal horizontal parts.
func SplitHorizontal(r Rect, n int, gap float64) []Rect {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []Rect{r}
	}

	totalGap := gap * float64(n-1)
	availableWidth := r.W - totalGap
	partWidth := availableWidth / float64(n)

	parts := make([]Rect, n)
	for i := range n {
		parts[i] = Rect{
			X: r.X + float64(i)*(partWidth+gap),
			Y: r.Y,
			W: partWidth,
			H: r.H,
		}
	}
	return parts
}

// SplitVertical splits a rectangle into n equal vertical parts.
func SplitVertical(r Rect, n int, gap float64) []Rect {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []Rect{r}
	}

	totalGap := gap * float64(n-1)
	availableHeight := r.H - totalGap
	partHeight := availableHeight / float64(n)

	parts := make([]Rect, n)
	for i := range n {
		parts[i] = Rect{
			X: r.X,
			Y: r.Y + float64(i)*(partHeight+gap),
			W: r.W,
			H: partHeight,
		}
	}
	return parts
}

// SplitGrid splits a rectangle into a grid of cells.
func SplitGrid(r Rect, cols, rows int, hGap, vGap float64) [][]Rect {
	if cols <= 0 || rows <= 0 {
		return nil
	}

	// Split horizontally first to get columns
	rowRects := SplitVertical(r, rows, vGap)

	grid := make([][]Rect, rows)
	for i, rowRect := range rowRects {
		grid[i] = SplitHorizontal(rowRect, cols, hGap)
	}
	return grid
}

// FlatGrid returns a flat slice of grid cells in row-major order.
func FlatGrid(grid [][]Rect) []Rect {
	var cells []Rect
	for _, row := range grid {
		cells = append(cells, row...)
	}
	return cells
}

// StackVertical arranges rectangles vertically with the given gap.
// The height of each rect is preserved.
func StackVertical(rects []Rect, gap float64) []Rect {
	if len(rects) == 0 {
		return nil
	}

	result := make([]Rect, len(rects))
	y := rects[0].Y
	for i, r := range rects {
		result[i] = Rect{
			X: r.X,
			Y: y,
			W: r.W,
			H: r.H,
		}
		y += r.H + gap
	}
	return result
}

// StackHorizontal arranges rectangles horizontally with the given gap.
// The width of each rect is preserved.
func StackHorizontal(rects []Rect, gap float64) []Rect {
	if len(rects) == 0 {
		return nil
	}

	result := make([]Rect, len(rects))
	x := rects[0].X
	for i, r := range rects {
		result[i] = Rect{
			X: x,
			Y: r.Y,
			W: r.W,
			H: r.H,
		}
		x += r.W + gap
	}
	return result
}

// CenterInRect returns a rectangle centered within the parent.
func CenterInRect(parent Rect, width, height float64) Rect {
	return Rect{
		X: parent.X + (parent.W-width)/2,
		Y: parent.Y + (parent.H-height)/2,
		W: width,
		H: height,
	}
}

// AlignInRect positions a rectangle within a parent based on alignment.
func AlignInRect(parent Rect, width, height float64, align BoxAlign) Rect {
	var x, y float64

	switch align.Horizontal {
	case HorizontalAlignLeft:
		x = parent.X
	case HorizontalAlignCenter:
		x = parent.X + (parent.W-width)/2
	case HorizontalAlignRight:
		x = parent.X + parent.W - width
	}

	switch align.Vertical {
	case VerticalAlignTop:
		y = parent.Y
	case VerticalAlignMiddle:
		y = parent.Y + (parent.H-height)/2
	case VerticalAlignBottom:
		y = parent.Y + parent.H - height
	}

	return Rect{X: x, Y: y, W: width, H: height}
}

// ExpandRect expands a rectangle by the given margins.
func ExpandRect(r Rect, top, right, bottom, left float64) Rect {
	return Rect{
		X: r.X - left,
		Y: r.Y - top,
		W: r.W + left + right,
		H: r.H + top + bottom,
	}
}

// ExpandRectAll expands a rectangle by the same margin on all sides.
func ExpandRectAll(r Rect, margin float64) Rect {
	return ExpandRect(r, margin, margin, margin, margin)
}

// BoundingRect returns the smallest rectangle that contains all given rectangles.
func BoundingRect(rects []Rect) Rect {
	if len(rects) == 0 {
		return Rect{}
	}

	minX := rects[0].X
	minY := rects[0].Y
	maxX := rects[0].X + rects[0].W
	maxY := rects[0].Y + rects[0].H

	for _, r := range rects[1:] {
		if r.X < minX {
			minX = r.X
		}
		if r.Y < minY {
			minY = r.Y
		}
		if r.X+r.W > maxX {
			maxX = r.X + r.W
		}
		if r.Y+r.H > maxY {
			maxY = r.Y + r.H
		}
	}

	return Rect{
		X: minX,
		Y: minY,
		W: maxX - minX,
		H: maxY - minY,
	}
}

// Intersects returns true if two rectangles overlap.
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.X+other.W && r.X+r.W > other.X &&
		r.Y < other.Y+other.H && r.Y+r.H > other.Y
}

// Intersection returns the overlapping area of two rectangles.
// Returns an empty rect if they don't overlap.
func (r Rect) Intersection(other Rect) Rect {
	if !r.Intersects(other) {
		return Rect{}
	}

	x := max(r.X, other.X)
	y := max(r.Y, other.Y)
	x2 := min(r.X+r.W, other.X+other.W)
	y2 := min(r.Y+r.H, other.Y+other.H)

	return Rect{X: x, Y: y, W: x2 - x, H: y2 - y}
}

// Union returns the smallest rectangle that contains both rectangles.
func (r Rect) Union(other Rect) Rect {
	return BoundingRect([]Rect{r, other})
}

// IsEmpty returns true if the rectangle has zero or negative dimensions.
func (r Rect) IsEmpty() bool {
	return r.W <= 0 || r.H <= 0
}

// Area returns the area of the rectangle.
func (r Rect) Area() float64 {
	if r.IsEmpty() {
		return 0
	}
	return r.W * r.H
}

// AspectRatio returns the width/height ratio.
func (r Rect) AspectRatio() float64 {
	if r.H == 0 {
		return 0
	}
	return r.W / r.H
}

// ScaleToFit returns a rectangle scaled to fit within the target while preserving aspect ratio.
func (r Rect) ScaleToFit(target Rect) Rect {
	if r.IsEmpty() {
		return r
	}

	scaleW := target.W / r.W
	scaleH := target.H / r.H
	scale := min(scaleW, scaleH)

	newW := r.W * scale
	newH := r.H * scale

	return CenterInRect(target, newW, newH)
}

// ScaleToFill returns a rectangle scaled to fill the target while preserving aspect ratio.
// Some parts may extend beyond the target bounds.
func (r Rect) ScaleToFill(target Rect) Rect {
	if r.IsEmpty() {
		return r
	}

	scaleW := target.W / r.W
	scaleH := target.H / r.H
	scale := max(scaleW, scaleH)

	newW := r.W * scale
	newH := r.H * scale

	return CenterInRect(target, newW, newH)
}
