package shapegrid

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// defaultTextSizeHPt is the default font size for shape_grid text cells in
// hundredths of a point. Without an explicit sz attribute, OOXML viewers fall
// back to the theme body default which is often 13pt — too small for
// comfortable reading at presentation distance.
const defaultTextSizeHPt = 1400 // 14pt

// DefaultTextSizePt is the point mirror of defaultTextSizeHPt: the size
// shape_grid text with no authored size renders at. Height-measuring estimators
// must use it, not a smaller legacy budget default, or they under-measure every
// unsized cell (go-slide-creator-lmpu).
const DefaultTextSizePt = float64(defaultTextSizeHPt) / 100

// minTextSizeHPt is the minimum font size floor for shape_grid text cells.
// JSON authors sometimes set conservative sizes (9-11pt) to avoid overflow;
// with normAutofit enabled, PowerPoint can shrink from this floor as needed,
// so we enforce a readable minimum.
const minTextSizeHPt = 1200 // 12pt

// MinTextSizePt is the shape_grid renderer's minimum effective font size, in
// points (the float mirror of minTextSizeHPt). Any explicitly authored size
// below this is raised to it at render time; text-capacity estimators must
// apply the same floor so budgets reflect the size text is actually rendered at.
const MinTextSizePt = float64(minTextSizeHPt) / 100 // 12.0pt

// EffectiveTextSizePt returns the point size shape_grid text with the given
// authored size is rendered at, after applying the minimum-size floor. This is
// the single source of truth for that floor, shared by the renderer
// (buildTextBody / buildParagraphsTextBody) and capacity estimators.
//
// An authored size of 0 (unspecified) is passed through unchanged so callers can
// substitute their own default — the renderer applies defaultTextSizeHPt for
// that case, but capacity estimators may use a different documented default.
func EffectiveTextSizePt(authoredPt float64) float64 {
	if authoredPt <= 0 {
		return authoredPt
	}
	if authoredPt < MinTextSizePt {
		return MinTextSizePt
	}
	return authoredPt
}

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
	return generateShapeXML(spec, id, bounds, 0, extraInsets...)
}

// generateShapeXML is GenerateShapeXML with an optional pre-computed autofit
// shrink (0 = let the shape measure its own).
func generateShapeXML(spec *ShapeSpec, id uint32, bounds pptx.RectEmu, autofitScale float64, extraInsets ...[4]int64) ([]byte, error) {
	opts := pptx.ShapeOptions{
		ID:       id,
		Bounds:   bounds,
		Geometry: pptx.PresetGeometry(spec.Geometry),
		Rotation: int64(spec.Rotation * 60000), // degrees to 60000ths
		FlipH:    spec.FlipH,
	}
	if spec.Link != nil {
		opts.HyperlinkRelID = fmt.Sprintf("json2pptx_shape_link_%d", id)
		if spec.Link.Slide > 0 {
			opts.HyperlinkAction = "ppaction://hlinksldjump"
		}
	}

	// Adjustments, in name order: ranging over the map directly made a
	// two-handle shape (an upArrow's adj1/adj2) emit its <a:gd> elements in
	// a random order, so identical input could produce different bytes.
	for _, name := range slices.Sorted(maps.Keys(spec.Adjustments)) {
		opts.Adjustments = append(opts.Adjustments, pptx.AdjustValue{Name: name, Value: int64(spec.Adjustments[name])})
	}

	// Default roundRect to sharp corners (adj=0) when no explicit adj is set.
	if spec.Geometry == "roundRect" && !hasAdjustment(opts.Adjustments, "adj") {
		opts.Adjustments = append(opts.Adjustments, pptx.AdjustValue{Name: "adj", Value: 0})
	}
	if adj, ok := DefaultChevronAdj(spec.Geometry, bounds.CX, bounds.CY); ok && adj < 50000 && !hasAdjustment(opts.Adjustments, "adj") {
		opts.Adjustments = append(opts.Adjustments, pptx.AdjustValue{Name: "adj", Value: adj})
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
		tb.ThemeFonts = spec.ThemeFonts
		// Apply extra insets (e.g., for icon overlay offset)
		if len(extraInsets) > 0 {
			ei := extraInsets[0]
			for i := 0; i < 4; i++ {
				tb.Insets[i] += ei[i]
			}
		}
		// Typographic finishing (go-slide-creator-58dhw): one alignment per
		// card, tracked caps labels, no one-word last line on bold headings.
		// Both measure the lines the text is set in: the preset's own text
		// rectangle, which an ellipse, a chevron or a diamond pulls well
		// inside the shape (go-slide-creator-69ums).
		unifyCardAlignment(tb)
		textRect := finishingTextRect(spec.Geometry, opts.Adjustments, bounds)
		if !spec.NoCapsTracking {
			trackCapsLabels(tb, textRect)
		}
		balanceHeadingLines(tb, textRect)
		pptx.SetAutofitScale(tb, autofitScale)
		opts.Text = tb
	}

	return pptx.GenerateShape(opts)
}

// finishingTextRect is the rectangle a shape's text is set in, before its
// insets: the preset geometry's text rectangle for the adjustments the shape
// is written with.
func finishingTextRect(geometry string, adjustments []pptx.AdjustValue, bounds pptx.RectEmu) pptx.RectEmu {
	adj := make(map[string]int64, len(adjustments))
	for _, av := range adjustments {
		adj[av.Name] = av.Value
	}
	w, h := pptx.PresetTextRect(geometry, adj, bounds)
	return pptx.RectEmu{X: bounds.X, Y: bounds.Y, CX: w, CY: h}
}

// FillObjectInput is the object form of a shape's fill, as ResolveFillInput
// decodes it. The input unknown-key check reads its keys from this type, so a
// key added here is accepted there without a second list.
type FillObjectInput struct {
	Color  string   `json:"color"`
	Alpha  *float64 `json:"alpha,omitempty"`
	LumMod int      `json:"lumMod,omitempty"`
	LumOff int      `json:"lumOff,omitempty"`
	Tint   int      `json:"tint,omitempty"`
	Shade  int      `json:"shade,omitempty"`
}

// LineObjectInput is the object form of a shape's line, as ResolveLineInput
// decodes it (see FillObjectInput).
type LineObjectInput struct {
	Color  string  `json:"color"`
	Width  float64 `json:"width,omitempty"`
	Dash   string  `json:"dash,omitempty"`
	LumMod int     `json:"lumMod,omitempty"`
	LumOff int     `json:"lumOff,omitempty"`
	Tint   int     `json:"tint,omitempty"`
	Shade  int     `json:"shade,omitempty"`
}

// TextObjectInput is the object form of a shape's text, as ResolveTextInput
// decodes it (see FillObjectInput): one run of content, or paragraphs.
type TextObjectInput struct {
	Content       string           `json:"content"`
	Paragraphs    []ParagraphInput `json:"paragraphs,omitempty"`
	Size          float64          `json:"size,omitempty"`
	Bold          bool             `json:"bold,omitempty"`
	Italic        bool             `json:"italic,omitempty"`
	Align         string           `json:"align,omitempty"`
	VerticalAlign string           `json:"vertical_align,omitempty"`
	Vert          string           `json:"vert,omitempty"`
	Color         string           `json:"color,omitempty"`
	Font          string           `json:"font,omitempty"`
	InsetLeft     *float64         `json:"inset_left,omitempty"`
	InsetRight    *float64         `json:"inset_right,omitempty"`
	InsetTop      *float64         `json:"inset_top,omitempty"`
	InsetBottom   *float64         `json:"inset_bottom,omitempty"`
}

// ParagraphInput is one entry of a text object's paragraphs array.
type ParagraphInput = paragraphDef

// ResolveFillInput parses fill from string shorthand or object form.
func ResolveFillInput(raw json.RawMessage) (pptx.Fill, error) {
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return ResolveFillString(s), nil
	}

	// Object form
	var obj FillObjectInput
	if err := json.Unmarshal(raw, &obj); err != nil {
		return pptx.Fill{}, fmt.Errorf("fill must be a color string (e.g. \"#FF0000\", \"accent1\", \"none\") or object {\"color\": \"...\", \"alpha\": 50}: %w", err)
	}
	if obj.Alpha != nil && (*obj.Alpha < 0 || *obj.Alpha > 100) {
		return pptx.Fill{}, fmt.Errorf("fill alpha must be between 0 and 100, got %g", *obj.Alpha)
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
	if obj.Alpha != nil {
		// Normalize alpha to OOXML thousandths-of-percent (100000 = fully opaque).
		// Accept both fractional (0-1) and percentage (1-100) conventions.
		alphaVal := *obj.Alpha
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
		if obj.Alpha != nil {
			alphaVal := *obj.Alpha
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
	var obj LineObjectInput
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
	Content    string  `json:"content"`
	Size       float64 `json:"size,omitempty"`
	Bold       bool    `json:"bold,omitempty"`
	Italic     bool    `json:"italic,omitempty"`
	Align      string  `json:"align,omitempty"`
	Color      string  `json:"color,omitempty"`
	Font       string  `json:"font,omitempty"`
	SpaceAfter float64 `json:"space_after,omitempty"` // Space below this paragraph in points (e.g. 6 = 6pt)
	// Alpha is the text opacity in percent (1-99); 0 or 100 = opaque. It needs
	// an explicit color. The agenda uses it to set the sections the deck is
	// not at in dk1 at 50% (go-slide-creator-r3gsw).
	Alpha float64 `json:"alpha,omitempty"`
	// Suffix is a trailing run in the same paragraph at SuffixSize (points),
	// sharing the paragraph's colour and weight and so its baseline: a unit set
	// smaller than the figure it qualifies ("$2.4B" + " TAM") rather than at
	// the same display size (go-slide-creator-yn2pw).
	Suffix     string  `json:"suffix,omitempty"`
	SuffixSize float64 `json:"suffix_size,omitempty"`
	// Figure marks a display figure its pattern measured to fit one line (a
	// KPI value): from FigureMinSizePt up it is written at that size instead
	// of settling onto the type scale, whose steps under the 18pt lead would
	// set a value fitted at 16pt in 14pt — the size of a subhead, two points
	// over its 12pt caption (go-slide-creator-a5ogo). Unlike the digit rule
	// for unmarked text (isDisplayFigure) it holds for a value with no digit
	// ("Unlimited"), so one row of values is one size.
	Figure bool `json:"figure,omitempty"`
	// Bullet makes the paragraph a real bulleted list item: true for the
	// default "•", or a string marker such as "–". It emits <a:buChar> with a
	// hanging indent so wrapped lines align with the text, not the marker
	// (go-slide-creator-zieyk).
	Bullet ParagraphBullet `json:"bullet,omitempty"`
}

// BulletHangPt is the hanging indent (points) of a bulleted grid paragraph
// at sizePt: 0.65em — the "•" marker (~0.35em) plus a gap, about the advance
// of the typed "• " prefix it replaces — capped at the 14pt
// (pptx.BulletMarginLeft) used by placeholder bullets, so small grid text does
// not lose a disproportionate share of a narrow cell.
func BulletHangPt(sizePt float64) float64 {
	maxPt := float64(pptx.BulletMarginLeft) / 12700
	return math.Min(0.65*sizePt, maxPt)
}

// ParagraphBullet is a paragraph's bullet marker: "" for none. It unmarshals
// from true (the default "•"), false (none) or a marker string.
type ParagraphBullet string

// UnmarshalJSON accepts a boolean or a marker string.
func (b *ParagraphBullet) UnmarshalJSON(data []byte) error {
	var on bool
	if err := json.Unmarshal(data, &on); err == nil {
		if on {
			*b = ParagraphBullet(pptx.DefaultBulletChar)
		} else {
			*b = ""
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("bullet must be true/false or a marker string: %w", err)
	}
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 2 {
		return fmt.Errorf("bullet marker %q must be at most 2 characters", s)
	}
	*b = ParagraphBullet(s)
	return nil
}

// paragraphColorFill resolves a paragraph's color, applying its opacity.
func paragraphColorFill(color string, alpha float64) pptx.Fill {
	if color == "" {
		return pptx.Fill{}
	}
	if alpha <= 0 || alpha >= 100 || color == "none" {
		return ResolveFillString(color)
	}
	val := int(alpha * 1000)
	if schemeColorNames[color] {
		return pptx.SchemeFill(color, pptx.Alpha(val))
	}
	return pptx.SolidFillWithAlpha(strings.TrimPrefix(color, "#"), val)
}

// ResolveTextInput parses text from string shorthand, object form, or paragraphs array form.
func ResolveTextInput(raw json.RawMessage) (*pptx.TextBody, error) {
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return buildTextBody(s, 0, false, false, "ctr", "ctr", "", "", pptx.ShapeTextInsets()), nil
	}

	// Object form — try to detect paragraphs array variant
	var obj TextObjectInput
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("text must be a string, object with \"content\", or object with \"paragraphs\" array: %w", err)
	}

	var tb *pptx.TextBody
	// Paragraphs array form: individually styled paragraphs
	if len(obj.Paragraphs) > 0 {
		tb = buildParagraphsTextBody(obj.Paragraphs, obj.Align, obj.VerticalAlign, obj.Font,
			shapeTextInsets(obj.InsetLeft, obj.InsetTop, obj.InsetRight, obj.InsetBottom))
	} else {
		tb = buildTextBody(obj.Content, obj.Size, obj.Bold, obj.Italic, obj.Align, obj.VerticalAlign, obj.Color, obj.Font,
			shapeTextInsets(obj.InsetLeft, obj.InsetTop, obj.InsetRight, obj.InsetBottom))
	}

	// Optional OOXML text-direction (e.g. "vert270") rotates only the text
	// within an otherwise-unrotated shape, so a vertical axis label can sit in a
	// colored band without flipping the band's fill geometry.
	if obj.Vert != "" {
		tb.Vert = obj.Vert
	}

	return tb, nil
}

// shapeTextInsets resolves a text object's inset_* fields (points) to EMU.
// Every side defaults to the uniform shape text margin (pptx.ShapeTextInsets,
// 0.5 cm); an inset the deck author writes explicitly — including 0 — replaces
// that side only. Named patterns write inset_* only to align baselines and to
// tighten the rows of a list that would not otherwise fit (docs/PATTERNS.md),
// so the margin is uniform on every other pattern shape.
func shapeTextInsets(l, t, r, b *float64) [4]int64 {
	insets := pptx.ShapeTextInsets()
	for i, v := range [4]*float64{l, t, r, b} {
		if v != nil && *v >= 0 {
			insets[i] = int64(*v * 12700)
		}
	}
	return insets
}

// normalizeVerticalAlign maps verbose anchor names ("top","center","middle",
// "bottom") to canonical OOXML attribute values ("t","ctr","b"). PowerPoint
// silently falls back to top alignment for unknown anchor strings, which
// produces conspicuous empty space under text in colored bars/boxes. Empty
// input is preserved so callers can apply their own default.
func normalizeVerticalAlign(v string) string {
	switch strings.ToLower(v) {
	case "top":
		return "t"
	case "center", "middle":
		return "ctr"
	case "bottom":
		return "b"
	default:
		return v
	}
}

// buildTextBody creates a TextBody with wrapping text.
func buildTextBody(content string, sizePt float64, bold, italic bool, align, vAlign, color, font string,
	insets [4]int64) *pptx.TextBody {
	if align == "" {
		align = "ctr"
	}
	vAlign = normalizeVerticalAlign(vAlign)
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
		fontSize = int(EffectiveTextSizePt(sizePt) * 100) // points to hundredths of a point
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
		InlineTags:     true,
	})

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
	insets [4]int64) *pptx.TextBody {
	if defaultAlign == "" {
		defaultAlign = "ctr"
	}
	vAlign = normalizeVerticalAlign(vAlign)
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
			fontSize = int(EffectiveTextSizePt(d.Size) * 100)
		}

		colorFill := paragraphColorFill(d.Color, d.Alpha)

		font := d.Font
		if font == "" {
			font = defaultFont
		}

		align := d.Align
		if align == "" {
			align = defaultAlign
		}

		baseRun := pptx.Run{
			Text:       d.Content,
			FontSize:   fontSize,
			Bold:       d.Bold,
			Italic:     d.Italic,
			Color:      colorFill,
			FontFamily: font,
		}

		var runs []pptx.Run
		if strings.Contains(d.Content, "<") {
			runs = pptx.SplitInlineTags(baseRun)
		} else {
			runs = []pptx.Run{baseRun}
		}
		if d.Suffix != "" {
			suffix := baseRun
			suffix.Text = d.Suffix
			if d.SuffixSize > 0 {
				suffix.FontSize = int(EffectiveTextSizePt(d.SuffixSize) * 100)
			}
			runs = append(runs, suffix)
		}

		paragraphs[i] = pptx.Paragraph{
			Align: align,
			Runs:  runs,
		}
		if d.SpaceAfter > 0 {
			paragraphs[i].SpaceAfter = int(d.SpaceAfter * 100)
		}
		if d.Bullet != "" {
			hang := int64(BulletHangPt(float64(fontSize)/100) * 12700)
			paragraphs[i].Bullet = &pptx.BulletDef{Char: string(d.Bullet), Font: pptx.DefaultBulletFont}
			paragraphs[i].MarginL = hang
			paragraphs[i].Indent = -hang
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
	if len(bar.Spec.Fill) > 0 {
		fill, err := ResolveFillInput(bar.Spec.Fill)
		if err != nil {
			return nil, err
		}
		opts.Fill = fill
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
	vAlign := normalizeVerticalAlign(spec.VerticalAlign)
	if vAlign == "" {
		vAlign = "b"
	}
	font := spec.Font
	if font == "" {
		font = "+mn-lt"
	}

	// An image caption is a label laid over a picture, not a shape: it keeps
	// its tight 4pt inset.
	const captionInsetEMU = 4 * 12700
	tb := buildTextBody(content, size, spec.Bold, false, align, vAlign, color, font,
		[4]int64{captionInsetEMU, captionInsetEMU, captionInsetEMU, captionInsetEMU})

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
