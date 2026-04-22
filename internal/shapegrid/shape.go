package shapegrid

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// defaultTextSizeHPt is the default font size for shape_grid text cells in
// hundredths of a point. Without an explicit sz attribute, OOXML viewers fall
// back to the theme body default which is often 13pt — too small for
// comfortable reading at presentation distance.
const defaultTextSizeHPt = 1400 // 14pt

// minTextSizeHPt is the minimum font size floor for shape_grid text cells.
// JSON authors sometimes set conservative sizes (9-11pt) to avoid overflow;
// with normAutofit enabled, PowerPoint can shrink from this floor as needed,
// so we enforce a readable minimum.
const minTextSizeHPt = 1200 // 12pt

// Bullet list constants — aliases for the shared pptx values, kept for
// backward compatibility in tests and local references.
const (
	bulletMarginLeft = pptx.BulletMarginLeft
	bulletIndent     = pptx.BulletIndent
)

// schemeColorNames aliases the canonical set from the pptx package.
var schemeColorNames = pptx.SchemeColorNames

// GenerateShapeXML creates a <p:sp> XML element from a ShapeSpec and resolved cell.
// Optional extraInsets are added to the text body insets (e.g., to make room for
// an icon overlay positioned beside or above the text).
func GenerateShapeXML(spec *ShapeSpec, id uint32, bounds pptx.RectEmu, extraInsets ...[4]int64) ([]byte, error) {
	opts := pptx.ShapeOptions{
		ID:       id,
		Bounds:   bounds,
		Geometry: pptx.PresetGeometry(spec.Geometry),
		Rotation: int64(spec.Rotation * 60000), // degrees to 60000ths
	}

	// Adjustments
	for name, val := range spec.Adjustments {
		opts.Adjustments = append(opts.Adjustments, pptx.AdjustValue{Name: name, Value: int64(val)})
	}

	// Default roundRect to sharp corners (adj=0) when no explicit adj is set.
	if spec.Geometry == "roundRect" && !hasAdjustment(opts.Adjustments, "adj") {
		opts.Adjustments = append(opts.Adjustments, pptx.AdjustValue{Name: "adj", Value: 0})
	}

	// Fill
	if len(spec.Fill) > 0 {
		fill, err := ResolveFillInput(spec.Fill)
		if err != nil {
			return nil, fmt.Errorf("fill: %w", err)
		}
		opts.Fill = fill
	}

	// Line
	if len(spec.Line) > 0 {
		line, err := ResolveLineInput(spec.Line)
		if err != nil {
			return nil, fmt.Errorf("line: %w", err)
		}
		opts.Line = line
	}

	// Text
	if len(spec.Text) > 0 {
		tb, err := ResolveTextInput(spec.Text)
		if err != nil {
			return nil, fmt.Errorf("text: %w", err)
		}
		// Apply extra insets (e.g., for icon overlay offset)
		if len(extraInsets) > 0 {
			ei := extraInsets[0]
			for i := 0; i < 4; i++ {
				tb.Insets[i] += ei[i]
			}
		}
		opts.Text = tb
	}

	return pptx.GenerateShape(opts)
}

// ResolveFillInput parses fill from string shorthand or object form.
func ResolveFillInput(raw json.RawMessage) (pptx.Fill, error) {
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return ResolveFillString(s), nil
	}

	// Object form
	var obj struct {
		Color  string  `json:"color"`
		Alpha  float64 `json:"alpha,omitempty"`
		LumMod int     `json:"lumMod,omitempty"`
		LumOff int     `json:"lumOff,omitempty"`
		Tint   int     `json:"tint,omitempty"`
		Shade  int     `json:"shade,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return pptx.Fill{}, fmt.Errorf("fill must be a color string (e.g. \"#FF0000\", \"accent1\", \"none\") or object {\"color\": \"...\", \"alpha\": 50}: %w", err)
	}

	// Validate color modifier ranges [0, 100000]
	for _, pair := range []struct {
		name string
		val  int
	}{
		{"lumMod", obj.LumMod},
		{"lumOff", obj.LumOff},
		{"tint", obj.Tint},
		{"shade", obj.Shade},
	} {
		if pair.val < 0 || pair.val > 100000 {
			return pptx.Fill{}, fmt.Errorf("fill %s must be between 0 and 100000, got %d", pair.name, pair.val)
		}
	}

	// Build color modifiers
	var mods []pptx.ColorMod
	if obj.Alpha > 0 {
		// Normalize alpha to OOXML thousandths-of-percent (100000 = fully opaque).
		// Accept both fractional (0-1) and percentage (1-100) conventions.
		alphaVal := obj.Alpha
		if alphaVal <= 1 {
			alphaVal *= 100000 // 0.3 → 30000
		} else {
			alphaVal *= 1000 // 50 → 50000
		}
		mods = append(mods, pptx.Alpha(int(alphaVal)))
	}
	if obj.LumMod > 0 {
		mods = append(mods, pptx.LumMod(obj.LumMod))
	}
	if obj.LumOff > 0 {
		mods = append(mods, pptx.LumOff(obj.LumOff))
	}
	if obj.Tint > 0 {
		mods = append(mods, pptx.Tint(obj.Tint))
	}
	if obj.Shade > 0 {
		mods = append(mods, pptx.Shade(obj.Shade))
	}

	if len(mods) > 0 {
		if schemeColorNames[obj.Color] {
			return pptx.SchemeFill(obj.Color, mods...), nil
		}
		// For hex colors, only alpha is supported via SolidFillWithAlpha
		if obj.Alpha > 0 {
			alphaVal := obj.Alpha
			if alphaVal <= 1 {
				alphaVal *= 100000
			} else {
				alphaVal *= 1000
			}
			hex := strings.TrimPrefix(obj.Color, "#")
			return pptx.SolidFillWithAlpha(hex, int(alphaVal)), nil
		}
	}
	return ResolveFillString(obj.Color), nil
}

// ResolveFillString resolves a single fill string: "#hex", "accent1", or "none".
func ResolveFillString(s string) pptx.Fill {
	if s == "none" || s == "" {
		return pptx.NoFill()
	}
	if strings.HasPrefix(s, "#") {
		return pptx.SolidFill(strings.TrimPrefix(s, "#"))
	}
	if schemeColorNames[s] {
		return pptx.SchemeFill(s)
	}
	// Treat as hex without #
	return pptx.SolidFill(s)
}

// ResolveLineInput parses line from string shorthand or object form.
func ResolveLineInput(raw json.RawMessage) (pptx.Line, error) {
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		fill := ResolveFillString(s)
		return pptx.Line{
			Width: 12700, // 1pt default
			Fill:  fill,
		}, nil
	}

	// Object form
	var obj struct {
		Color  string  `json:"color"`
		Width  float64 `json:"width,omitempty"`
		Dash   string  `json:"dash,omitempty"`
		LumMod int     `json:"lumMod,omitempty"`
		LumOff int     `json:"lumOff,omitempty"`
		Tint   int     `json:"tint,omitempty"`
		Shade  int     `json:"shade,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return pptx.Line{}, fmt.Errorf("line must be a color string (e.g. \"#000000\") or object {\"color\": \"...\", \"width\": 2, \"dash\": \"dot\"}: %w", err)
	}

	// Validate color modifier ranges [0, 100000]
	for _, pair := range []struct {
		name string
		val  int
	}{
		{"lumMod", obj.LumMod},
		{"lumOff", obj.LumOff},
		{"tint", obj.Tint},
		{"shade", obj.Shade},
	} {
		if pair.val < 0 || pair.val > 100000 {
			return pptx.Line{}, fmt.Errorf("line %s must be between 0 and 100000, got %d", pair.name, pair.val)
		}
	}

	width := int64(12700) // 1pt default
	if obj.Width > 0 {
		width = int64(obj.Width * 12700) // points to EMU
	}

	var mods []pptx.ColorMod
	if obj.LumMod > 0 {
		mods = append(mods, pptx.LumMod(obj.LumMod))
	}
	if obj.LumOff > 0 {
		mods = append(mods, pptx.LumOff(obj.LumOff))
	}
	if obj.Tint > 0 {
		mods = append(mods, pptx.Tint(obj.Tint))
	}
	if obj.Shade > 0 {
		mods = append(mods, pptx.Shade(obj.Shade))
	}

	var fill pptx.Fill
	if len(mods) > 0 && schemeColorNames[obj.Color] {
		fill = pptx.SchemeFill(obj.Color, mods...)
	} else {
		fill = ResolveFillString(obj.Color)
	}

	return pptx.Line{
		Width: width,
		Fill:  fill,
		Dash:  obj.Dash,
	}, nil
}

// paragraphDef defines a single paragraph with individual styling in the paragraphs array form.
type paragraphDef struct {
	Content string  `json:"content"`
	Size    float64 `json:"size,omitempty"`
	Bold    bool    `json:"bold,omitempty"`
	Italic  bool    `json:"italic,omitempty"`
	Align   string  `json:"align,omitempty"`
	Color   string  `json:"color,omitempty"`
	Font    string  `json:"font,omitempty"`
}

// ResolveTextInput parses text from string shorthand, object form, or paragraphs array form.
func ResolveTextInput(raw json.RawMessage) (*pptx.TextBody, error) {
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return buildTextBody(s, 0, false, false, "ctr", "ctr", "", "", 0, 0, 0, 0), nil
	}

	// Object form — try to detect paragraphs array variant
	var obj struct {
		Content       string          `json:"content"`
		Paragraphs    []paragraphDef  `json:"paragraphs,omitempty"`
		Size          float64         `json:"size,omitempty"`
		Bold          bool            `json:"bold,omitempty"`
		Italic        bool            `json:"italic,omitempty"`
		Align         string          `json:"align,omitempty"`
		VerticalAlign string          `json:"vertical_align,omitempty"`
		Color         string          `json:"color,omitempty"`
		Font          string          `json:"font,omitempty"`
		InsetLeft     float64         `json:"inset_left,omitempty"`
		InsetRight    float64         `json:"inset_right,omitempty"`
		InsetTop      float64         `json:"inset_top,omitempty"`
		InsetBottom   float64         `json:"inset_bottom,omitempty"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("text must be a string, object with \"content\", or object with \"paragraphs\" array: %w", err)
	}

	// Paragraphs array form: individually styled paragraphs
	if len(obj.Paragraphs) > 0 {
		return buildParagraphsTextBody(obj.Paragraphs, obj.Align, obj.VerticalAlign, obj.Font,
			obj.InsetLeft, obj.InsetTop, obj.InsetRight, obj.InsetBottom), nil
	}

	return buildTextBody(obj.Content, obj.Size, obj.Bold, obj.Italic, obj.Align, obj.VerticalAlign, obj.Color, obj.Font,
		obj.InsetLeft, obj.InsetTop, obj.InsetRight, obj.InsetBottom), nil
}

// buildTextBody creates a TextBody with wrapping text.
func buildTextBody(content string, sizePt float64, bold, italic bool, align, vAlign, color, font string,
	insetL, insetT, insetR, insetB float64) *pptx.TextBody {
	if align == "" {
		align = "ctr"
	}
	if vAlign == "" {
		vAlign = "ctr"
	}

	// Default to OOXML theme minor (body) font when no explicit font set.
	// "+mn-lt" resolves at render time from the template's theme, so shapes
	// automatically match any template's typography.
	if font == "" {
		font = "+mn-lt"
	}

	fontSize := defaultTextSizeHPt
	if sizePt > 0 {
		fontSize = int(sizePt * 100) // points to hundredths of a point
	}
	if fontSize < minTextSizeHPt {
		fontSize = minTextSizeHPt
	}

	var colorFill pptx.Fill
	if color != "" {
		colorFill = ResolveFillString(color)
	}

	// Use the shared bullet text parser for consistent bullet detection
	// across all text rendering pipelines.
	paragraphs := pptx.ParseBulletText(content, pptx.BulletTextOptions{
		FontSize:       fontSize,
		Bold:           bold,
		Italic:         italic,
		Align:          align,
		Color:          colorFill,
		FontFamily:     font,
		DetectNumbered: true,
	})

	// Convert point insets to EMU (1pt = 12700 EMU)
	var insets [4]int64
	if insetL > 0 || insetT > 0 || insetR > 0 || insetB > 0 {
		insets = [4]int64{
			int64(insetL * 12700),
			int64(insetT * 12700),
			int64(insetR * 12700),
			int64(insetB * 12700),
		}
	}

	return &pptx.TextBody{
		Wrap:       "square",
		Anchor:     vAlign,
		AnchorCtr:  false,
		Insets:     insets,
		AutoFit:    "normAutofit",
		Paragraphs: paragraphs,
	}
}


// buildParagraphsTextBody creates a TextBody from individually styled paragraphs.
func buildParagraphsTextBody(defs []paragraphDef, defaultAlign, vAlign, defaultFont string,
	insetL, insetT, insetR, insetB float64) *pptx.TextBody {
	if defaultAlign == "" {
		defaultAlign = "ctr"
	}
	if vAlign == "" {
		vAlign = "ctr"
	}
	if defaultFont == "" {
		defaultFont = "+mn-lt"
	}

	paragraphs := make([]pptx.Paragraph, len(defs))
	for i, d := range defs {
		fontSize := defaultTextSizeHPt
		if d.Size > 0 {
			fontSize = int(d.Size * 100)
		}
		if fontSize < minTextSizeHPt {
			fontSize = minTextSizeHPt
		}

		var colorFill pptx.Fill
		if d.Color != "" {
			colorFill = ResolveFillString(d.Color)
		}

		font := d.Font
		if font == "" {
			font = defaultFont
		}

		align := d.Align
		if align == "" {
			align = defaultAlign
		}

		paragraphs[i] = pptx.Paragraph{
			Align: align,
			Runs: []pptx.Run{{
				Text:       d.Content,
				FontSize:   fontSize,
				Bold:       d.Bold,
				Italic:     d.Italic,
				Color:      colorFill,
				FontFamily: font,
			}},
		}
	}

	var insets [4]int64
	if insetL > 0 || insetT > 0 || insetR > 0 || insetB > 0 {
		insets = [4]int64{
			int64(insetL * 12700),
			int64(insetT * 12700),
			int64(insetR * 12700),
			int64(insetB * 12700),
		}
	}

	return &pptx.TextBody{
		Wrap:       "square",
		Anchor:     vAlign,
		AnchorCtr:  false,
		Insets:     insets,
		AutoFit:    "normAutofit",
		Paragraphs: paragraphs,
	}
}

// GenerateAccentBarXML creates a <p:sp> XML element for a decorative accent bar.
// The bar is a simple filled rectangle with no outline and no text.
func GenerateAccentBarXML(bar *ResolvedAccentBar) ([]byte, error) {
	color := bar.Spec.Color
	if color == "" {
		color = "accent1"
	}

	opts := pptx.ShapeOptions{
		ID:       bar.ID,
		Bounds:   bar.Bounds,
		Geometry: pptx.PresetGeometry("rect"),
		Fill:     ResolveFillString(color),
		Line:     pptx.NoLine(),
	}

	return pptx.GenerateShape(opts)
}

// GenerateImageOverlayXML creates a semi-transparent rectangle overlay for an image cell.
func GenerateImageOverlayXML(spec *OverlaySpec, id uint32, bounds pptx.RectEmu) ([]byte, error) {
	color := spec.Color
	if color == "" {
		color = "000000"
	}
	color = strings.TrimPrefix(color, "#")

	alpha := spec.Alpha
	if alpha <= 0 {
		alpha = 0.4
	}
	if alpha > 1 {
		alpha = 1
	}
	// Convert 0-1 opacity to OOXML thousandths-of-percent (100000 = fully opaque)
	alphaVal := int(alpha * 100000)

	var fill pptx.Fill
	if schemeColorNames[color] {
		fill = pptx.SchemeFill(color, pptx.Alpha(alphaVal))
	} else {
		fill = pptx.SolidFillWithAlpha(color, alphaVal)
	}

	opts := pptx.ShapeOptions{
		ID:       id,
		Bounds:   bounds,
		Geometry: pptx.PresetGeometry("rect"),
		Fill:     fill,
		Line:     pptx.NoLine(),
	}

	return pptx.GenerateShape(opts)
}

// GenerateImageTextXML creates a text box shape for an image text label.
func GenerateImageTextXML(spec *ImageText, id uint32, bounds pptx.RectEmu) ([]byte, error) {
	content := spec.Content
	size := spec.Size
	if size <= 0 {
		size = 14
	}
	color := spec.Color
	if color == "" {
		color = "FFFFFF"
	}
	align := spec.Align
	if align == "" {
		align = "ctr"
	}
	vAlign := spec.VerticalAlign
	if vAlign == "" {
		vAlign = "b"
	}
	font := spec.Font
	if font == "" {
		font = "+mn-lt"
	}

	tb := buildTextBody(content, size, spec.Bold, false, align, vAlign, color, font, 4, 4, 4, 4)

	opts := pptx.ShapeOptions{
		ID:       id,
		Bounds:   bounds,
		Geometry: pptx.PresetGeometry("rect"),
		Fill:     pptx.NoFill(),
		Line:     pptx.NoLine(),
		Text:     tb,
	}

	return pptx.GenerateShape(opts)
}

// hasAdjustment returns true if the slice contains an adjustment with the given name.
func hasAdjustment(adjs []pptx.AdjustValue, name string) bool {
	for _, a := range adjs {
		if a.Name == name {
			return true
		}
	}
	return false
}
