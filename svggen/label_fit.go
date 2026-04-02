package svggen

import "math"

// LabelFitResult holds the output of a label fitting operation.
type LabelFitResult struct {
	// FontSize is the computed font size (≥ strategy MinSize).
	FontSize float64
	// DisplayText is the text to render (may be truncated with ellipsis).
	DisplayText string
	// Wrapped is true if the text should be rendered with DrawWrappedText.
	Wrapped bool
}

// LabelFitStrategy encodes the ordered cascade for fitting text into a
// constrained space. Each diagram type configures its own strategy, but the
// cascade logic (shrink → wrap → truncate) is shared.
//
// This replaces scattered ad-hoc font sizing across timeline.go, process_flow.go,
// waterfall.go, and charts.go with a single decision path.
type LabelFitStrategy struct {
	// PreferredSize is the starting font size (e.g., SizeBody = 12pt).
	PreferredSize float64
	// MinSize is the absolute floor font size (e.g., 9pt).
	// The builder's MinFontSize() is also enforced as a floor.
	MinSize float64
	// AllowWrap enables multi-line wrapping when text overflows at MinSize.
	AllowWrap bool
	// MaxLines limits wrapped text to this many lines (default 1 = no wrap).
	MaxLines int
	// MinCharWidth is the minimum label width expressed as a multiple of
	// MinSize (e.g., 5.5 means ~8 average chars). Prevents illegible
	// truncation in narrow two-column layouts. 0 means no floor.
	MinCharWidth float64
}

// DefaultLabelFit returns a strategy suitable for primary content labels
// (activity names, step labels, milestone labels). Uses SizeBody with a
// 9pt floor and ~8-character minimum width.
func DefaultLabelFit(typo *Typography) LabelFitStrategy {
	return LabelFitStrategy{
		PreferredSize: typo.SizeBody,
		MinSize:       7.0,
		AllowWrap:     true,
		MaxLines:      2,
		MinCharWidth:  5.5,
	}
}

// DescriptionLabelFit returns a strategy suitable for secondary description
// text that can wrap to multiple lines. Uses SizeBody with wrapping.
func DescriptionLabelFit(typo *Typography) LabelFitStrategy {
	return LabelFitStrategy{
		PreferredSize: typo.SizeBody,
		MinSize:       7.0,
		AllowWrap:     true,
		MaxLines:      4,
		MinCharWidth:  5.5,
	}
}

// Fit computes the best font size and display text for the given constraints.
// It applies the cascade: shrink font → wrap (if allowed) → truncate.
//
// maxWidth is the available horizontal space for the label.
// maxHeight is only used when AllowWrap is true (0 means unconstrained).
func (s LabelFitStrategy) Fit(b *SVGBuilder, text string, maxWidth, maxHeight float64) LabelFitResult {
	if text == "" || maxWidth <= 0 {
		return LabelFitResult{FontSize: s.PreferredSize, DisplayText: text}
	}

	// Apply minimum width floor for readability.
	minSize := s.effectiveMinSize(b)
	if s.MinCharWidth > 0 {
		if minW := minSize * s.MinCharWidth; maxWidth < minW {
			maxWidth = minW
		}
	}

	// Step 1: Shrink font to fit within maxWidth.
	fontSize := b.ClampFontSize(text, maxWidth, s.PreferredSize, minSize)
	b.SetFontSize(fontSize)

	// Step 2: If wrapping is allowed and text still overflows, try wrapped fit.
	if s.AllowWrap && s.MaxLines > 1 && maxHeight > 0 {
		w, _ := b.MeasureText(text)
		if w > maxWidth {
			// Use ClampFontSizeForRect for multi-line fitting.
			fontSize = b.ClampFontSizeForRect(text, maxWidth, maxHeight, s.PreferredSize, minSize)
			b.SetFontSize(fontSize)

			// Check if wrapped text fits.
			block := b.WrapText(text, maxWidth)
			if block.TotalHeight <= maxHeight {
				return LabelFitResult{
					FontSize:    fontSize,
					DisplayText: text,
					Wrapped:     true,
				}
			}
		}
	}

	// Step 3: Truncate with ellipsis as last resort.
	displayText := b.TruncateToWidth(text, maxWidth)

	return LabelFitResult{
		FontSize:    fontSize,
		DisplayText: displayText,
	}
}

// effectiveMinSize returns the larger of the strategy's MinSize and the
// builder's global MinFontSize floor.
func (s LabelFitStrategy) effectiveMinSize(b *SVGBuilder) float64 {
	floor := b.MinFontSize()
	return math.Max(s.MinSize, floor)
}
