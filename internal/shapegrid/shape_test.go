package shapegrid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestGenerateShapeXML_MinimalRect(t *testing.T) {
	spec := &ShapeSpec{Geometry: "rect"}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}

	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `prst="rect"`) {
		t.Error("missing rect geometry")
	}
	if !strings.Contains(s, `<p:sp>`) || !strings.Contains(s, `</p:sp>`) {
		t.Error("missing p:sp element")
	}
}

func TestGenerateShapeXML_WithRotation(t *testing.T) {
	spec := &ShapeSpec{Geometry: "rect", Rotation: 45}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}

	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xml), `rot="2700000"`) {
		t.Error("expected rot=2700000 (45*60000)")
	}
}

func TestResolveFillString_Hex(t *testing.T) {
	fill := ResolveFillString("#4472C4")
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
}

func TestResolveFillString_Scheme(t *testing.T) {
	fill := ResolveFillString("accent1")
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
}

func TestResolveFillString_None(t *testing.T) {
	fill := ResolveFillString("none")
	if fill.IsZero() {
		t.Error("expected non-zero fill (noFill is still set)")
	}
}

func TestResolveFillInput_ObjectWithAlpha(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","alpha":20}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
	// Verify XML output: alpha 20 → 20000 (20% in OOXML thousandths of a percent)
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	xml := buf.String()
	if !strings.Contains(xml, `<a:alpha val="20000"/>`) {
		t.Errorf("expected alpha val=20000, got: %s", xml)
	}
	if !strings.Contains(xml, `schemeClr val="accent1"`) {
		t.Errorf("expected scheme color accent1, got: %s", xml)
	}
}

func TestResolveLineInput_String(t *testing.T) {
	raw := json.RawMessage(`"accent1"`)
	line, err := ResolveLineInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if line.Width != 12700 {
		t.Errorf("expected default width 12700, got %d", line.Width)
	}
}

func TestResolveLineInput_Object(t *testing.T) {
	raw := json.RawMessage(`{"color":"#FF0000","width":2.5,"dash":"dash"}`)
	line, err := ResolveLineInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	expected := int64(2.5 * 12700)
	if line.Width != expected {
		t.Errorf("expected width %d, got %d", expected, line.Width)
	}
	if line.Dash != "dash" {
		t.Errorf("expected dash 'dash', got %q", line.Dash)
	}
}

func TestResolveTextInput_String(t *testing.T) {
	raw := json.RawMessage(`"Hello"`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 1 || tb.Paragraphs[0].Runs[0].Text != "Hello" {
		t.Error("unexpected text body content")
	}
}

func TestResolveTextInput_Multiline(t *testing.T) {
	raw := json.RawMessage(`"Line1\nLine2"`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(tb.Paragraphs))
	}
}

func TestResolveTextInput_Object(t *testing.T) {
	raw := json.RawMessage(`{"content":"Bold Title","size":16,"bold":true,"align":"ctr","color":"#FFFFFF"}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	run := tb.Paragraphs[0].Runs[0]
	if run.Text != "Bold Title" || !run.Bold || run.FontSize != 1600 {
		t.Errorf("unexpected run: %+v", run)
	}
}

func TestGenerateShapeXML_AllPhase1Geometries(t *testing.T) {
	geometries := []string{
		"rect", "roundRect", "ellipse", "diamond", "triangle",
		"hexagon", "chevron", "homePlate", "rightArrow", "star5",
		"pentagon", "octagon", "trapezoid", "parallelogram",
		"cloud", "heart", "plus", "donut", "flowChartProcess", "flowChartDecision",
	}
	bounds := pptx.RectEmu{X: 100000, Y: 100000, CX: 2000000, CY: 1000000}
	for _, geom := range geometries {
		t.Run(geom, func(t *testing.T) {
			spec := &ShapeSpec{Geometry: geom}
			xml, err := GenerateShapeXML(spec, 1, bounds)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(xml), fmt.Sprintf(`prst="%s"`, geom)) {
				t.Errorf("expected prst=%q in XML", geom)
			}
		})
	}
}

func TestGenerateShapeXML_ZeroBounds(t *testing.T) {
	spec := &ShapeSpec{Geometry: "rect"}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 0, CY: 0}
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xml), `<p:sp>`) {
		t.Error("should generate valid shape even with zero bounds")
	}
}

func TestGenerateShapeXML_LargeBounds(t *testing.T) {
	spec := &ShapeSpec{Geometry: "rect"}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 12192000, CY: 6858000} // full slide
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xml), `cx="12192000"`) {
		t.Error("expected full slide width in bounds")
	}
}

func TestGenerateShapeXML_WithAdjustments(t *testing.T) {
	spec := &ShapeSpec{
		Geometry:    "roundRect",
		Adjustments: map[string]int64{"adj": 25000},
	}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `name="adj"`) {
		t.Error("expected adjustment name in XML")
	}
}

func TestGenerateShapeXML_RoundRectDefaultSharpCorners(t *testing.T) {
	spec := &ShapeSpec{
		Geometry: "roundRect",
		// No explicit adjustments — should default adj=0 (sharp corners)
	}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `name="adj" fmla="val 0"`) {
		t.Error("expected roundRect to default adj=0 for sharp corners")
	}
}

func TestGenerateShapeXML_RoundRectExplicitRadiusPreserved(t *testing.T) {
	spec := &ShapeSpec{
		Geometry:    "roundRect",
		Adjustments: map[string]int64{"adj": 16667}, // explicit rounded corners
	}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `name="adj" fmla="val 16667"`) {
		t.Error("expected explicit adj=16667 to be preserved")
	}
	if strings.Contains(s, `fmla="val 0"`) {
		t.Error("should not inject adj=0 when explicit adj is set")
	}
}

func TestGenerateShapeXML_RectNoAdjInjected(t *testing.T) {
	spec := &ShapeSpec{
		Geometry: "rect",
	}
	bounds := pptx.RectEmu{X: 0, Y: 0, CX: 1000000, CY: 500000}
	xml, err := GenerateShapeXML(spec, 1, bounds)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if strings.Contains(s, `name="adj"`) {
		t.Error("rect geometry should not get adj injection")
	}
}

func TestResolveFillInput_HexWithAlpha(t *testing.T) {
	raw := json.RawMessage(`{"color":"#4472C4","alpha":50}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fill.IsZero() {
		t.Error("expected non-zero fill")
	}
	// Verify XML output: alpha 50 → 50000 (50% in OOXML thousandths of a percent)
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	xml := buf.String()
	expected := `<a:solidFill><a:srgbClr val="4472C4"><a:alpha val="50000"/></a:srgbClr></a:solidFill>`
	if xml != expected {
		t.Errorf("HexWithAlpha XML:\ngot:  %s\nwant: %s", xml, expected)
	}
}

func TestResolveFillInput_SchemeAlpha_XML(t *testing.T) {
	raw := json.RawMessage(`{"color":"dk1","alpha":65}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="dk1"><a:alpha val="65000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("SchemeAlpha XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_AlphaZero_NoMod(t *testing.T) {
	// alpha: 0 should be treated as "not set" (no alpha modifier)
	raw := json.RawMessage(`{"color":"accent1","alpha":0}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="accent1"/></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("AlphaZero XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_FractionalAlpha(t *testing.T) {
	// Fractional alpha (0-1) as used in JSON inputs: 0.3 = 30% opacity → val="30000"
	raw := json.RawMessage(`{"color":"#000000","alpha":0.3}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:srgbClr val="000000"><a:alpha val="30000"/></a:srgbClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("FractionalAlpha XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_FractionalAlpha_Scheme(t *testing.T) {
	// Fractional alpha on scheme color: 0.6 = 60% opacity → val="60000"
	raw := json.RawMessage(`{"color":"accent1","alpha":0.6}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="accent1"><a:alpha val="60000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("FractionalAlpha Scheme XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	_, err := ResolveFillInput(raw)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestResolveFillString_HexWithoutHash(t *testing.T) {
	fill := ResolveFillString("4472C4")
	if fill.IsZero() {
		t.Error("expected non-zero fill for hex without #")
	}
}

func TestResolveFillString_EmptyString(t *testing.T) {
	fill := ResolveFillString("")
	// Empty string treated as "none"
	if fill.IsZero() {
		t.Error("expected non-zero fill (noFill is still set)")
	}
}

func TestResolveFillString_AllSchemeColors(t *testing.T) {
	colors := []string{
		"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
		"dk1", "dk2", "lt1", "lt2", "tx1", "tx2", "bg1", "bg2",
		"hlink", "folHlink",
	}
	for _, c := range colors {
		t.Run(c, func(t *testing.T) {
			fill := ResolveFillString(c)
			if fill.IsZero() {
				t.Errorf("expected non-zero fill for scheme color %s", c)
			}
		})
	}
}

func TestResolveFillInput_LumModLumOff(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent3","lumMod":20000,"lumOff":80000}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="accent3"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("LumModLumOff XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_Tint(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","tint":50000}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="accent1"><a:tint val="50000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("Tint XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_Shade(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent2","shade":75000}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="accent2"><a:shade val="75000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("Shade XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_AlphaWithLumMod(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","alpha":50,"lumMod":80000}`)
	fill, err := ResolveFillInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="accent1"><a:alpha val="50000"/><a:lumMod val="80000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("AlphaWithLumMod XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveFillInput_LumModOutOfRange(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","lumMod":200000}`)
	_, err := ResolveFillInput(raw)
	if err == nil {
		t.Error("expected error for lumMod > 100000")
	}
}

func TestResolveFillInput_ShadeNegative(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","shade":-1}`)
	_, err := ResolveFillInput(raw)
	if err == nil {
		t.Error("expected error for shade < 0")
	}
}

func TestResolveLineInput_WithLumMod(t *testing.T) {
	raw := json.RawMessage(`{"color":"dk1","lumMod":50000,"width":2}`)
	line, err := ResolveLineInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	line.Fill.WriteTo(&buf)
	expected := `<a:solidFill><a:schemeClr val="dk1"><a:lumMod val="50000"/></a:schemeClr></a:solidFill>`
	if buf.String() != expected {
		t.Errorf("Line LumMod XML:\ngot:  %s\nwant: %s", buf.String(), expected)
	}
}

func TestResolveLineInput_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	_, err := ResolveLineInput(raw)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestResolveLineInput_ZeroWidth(t *testing.T) {
	raw := json.RawMessage(`{"color":"accent1","width":0}`)
	line, err := ResolveLineInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	// width=0 should use default
	if line.Width != 12700 {
		t.Errorf("expected default width 12700 for width=0, got %d", line.Width)
	}
}

func TestResolveTextInput_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`{invalid}`)
	_, err := ResolveTextInput(raw)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestResolveTextInput_WithFont(t *testing.T) {
	raw := json.RawMessage(`{"content":"Hello","font":"Arial"}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Paragraphs[0].Runs[0].FontFamily != "Arial" {
		t.Errorf("expected font Arial, got %q", tb.Paragraphs[0].Runs[0].FontFamily)
	}
}

func TestResolveTextInput_DefaultFont(t *testing.T) {
	raw := json.RawMessage(`"Hello"`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Paragraphs[0].Runs[0].FontFamily != "+mn-lt" {
		t.Errorf("expected default font +mn-lt, got %q", tb.Paragraphs[0].Runs[0].FontFamily)
	}
}

func TestResolveTextInput_WithInsets(t *testing.T) {
	raw := json.RawMessage(`{"content":"Test","inset_left":5,"inset_right":5,"inset_top":3,"inset_bottom":3}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	expectedL := int64(5 * 12700)
	if tb.Insets[0] != expectedL {
		t.Errorf("expected inset_left=%d, got %d", expectedL, tb.Insets[0])
	}
}

func TestResolveTextInput_VerticalAlignTop(t *testing.T) {
	raw := json.RawMessage(`{"content":"Top","vertical_align":"t"}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Anchor != "t" {
		t.Errorf("expected anchor=t, got %q", tb.Anchor)
	}
}

func TestResolveTextInput_EmptyContent(t *testing.T) {
	raw := json.RawMessage(`""`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 1 || tb.Paragraphs[0].Runs[0].Text != "" {
		t.Error("expected single paragraph with empty text")
	}
}

func TestResolveTextInput_ParagraphsArray(t *testing.T) {
	raw := json.RawMessage(`{
		"paragraphs": [
			{"content": "$8.4M", "size": 28, "bold": true, "color": "#FFFFFF"},
			{"content": "Total Investment", "size": 12, "color": "#CCCCCC"}
		]
	}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.Paragraphs) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(tb.Paragraphs))
	}

	// First paragraph: large bold headline
	p0 := tb.Paragraphs[0]
	if p0.Runs[0].Text != "$8.4M" {
		t.Errorf("expected text '$8.4M', got %q", p0.Runs[0].Text)
	}
	if p0.Runs[0].FontSize != 2800 {
		t.Errorf("expected fontSize 2800, got %d", p0.Runs[0].FontSize)
	}
	if !p0.Runs[0].Bold {
		t.Error("expected bold=true")
	}

	// Second paragraph: smaller label
	p1 := tb.Paragraphs[1]
	if p1.Runs[0].Text != "Total Investment" {
		t.Errorf("expected text 'Total Investment', got %q", p1.Runs[0].Text)
	}
	if p1.Runs[0].FontSize != 1200 {
		t.Errorf("expected fontSize 1200, got %d", p1.Runs[0].FontSize)
	}
	if p1.Runs[0].Bold {
		t.Error("expected bold=false")
	}
}

func TestResolveTextInput_ParagraphsWithDefaults(t *testing.T) {
	raw := json.RawMessage(`{
		"paragraphs": [
			{"content": "Hello"}
		],
		"align": "l",
		"vertical_align": "t"
	}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Anchor != "t" {
		t.Errorf("expected anchor=t, got %q", tb.Anchor)
	}
	if tb.Paragraphs[0].Align != "l" {
		t.Errorf("expected align=l, got %q", tb.Paragraphs[0].Align)
	}
	if tb.Paragraphs[0].Runs[0].FontFamily != "+mn-lt" {
		t.Errorf("expected default font +mn-lt, got %q", tb.Paragraphs[0].Runs[0].FontFamily)
	}
}

func TestResolveTextInput_ParagraphsPerParaAlign(t *testing.T) {
	raw := json.RawMessage(`{
		"paragraphs": [
			{"content": "Centered", "align": "ctr"},
			{"content": "Left", "align": "l"}
		],
		"align": "r"
	}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Paragraphs[0].Align != "ctr" {
		t.Errorf("expected first paragraph align=ctr, got %q", tb.Paragraphs[0].Align)
	}
	if tb.Paragraphs[1].Align != "l" {
		t.Errorf("expected second paragraph align=l, got %q", tb.Paragraphs[1].Align)
	}
}

func TestResolveTextInput_ParagraphsWithFont(t *testing.T) {
	raw := json.RawMessage(`{
		"paragraphs": [
			{"content": "Custom", "font": "Arial"},
			{"content": "Default"}
		],
		"font": "Helvetica"
	}`)
	tb, err := ResolveTextInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tb.Paragraphs[0].Runs[0].FontFamily != "Arial" {
		t.Errorf("expected Arial, got %q", tb.Paragraphs[0].Runs[0].FontFamily)
	}
	if tb.Paragraphs[1].Runs[0].FontFamily != "Helvetica" {
		t.Errorf("expected Helvetica fallback, got %q", tb.Paragraphs[1].Runs[0].FontFamily)
	}
}

func TestGenerateAccentBarXML(t *testing.T) {
	bar := &ResolvedAccentBar{
		Bounds: pptx.RectEmu{X: 10000, Y: 20000, CX: 50800, CY: 300000},
		ID:     42,
		Spec:   &AccentBarSpec{Position: "left", Color: "accent1", Width: 4},
	}

	xml, err := GenerateAccentBarXML(bar)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	if !strings.Contains(s, `<p:sp>`) {
		t.Error("missing p:sp element")
	}
	if !strings.Contains(s, `prst="rect"`) {
		t.Error("expected rect geometry for accent bar")
	}
	// Should have a scheme fill for accent1
	if !strings.Contains(s, `accent1`) {
		t.Error("expected accent1 scheme color")
	}
	// Should have no line (noFill on line)
	if !strings.Contains(s, `<a:noFill/>`) {
		t.Error("expected noFill on accent bar line")
	}
}

func TestGenerateAccentBarXML_DefaultColor(t *testing.T) {
	bar := &ResolvedAccentBar{
		Bounds: pptx.RectEmu{X: 10000, Y: 20000, CX: 50800, CY: 300000},
		ID:     1,
		Spec:   &AccentBarSpec{}, // all defaults
	}

	xml, err := GenerateAccentBarXML(bar)
	if err != nil {
		t.Fatal(err)
	}
	s := string(xml)
	// Default color is accent1
	if !strings.Contains(s, `accent1`) {
		t.Error("expected default accent1 color")
	}
}

func TestGenerateImageOverlayXML(t *testing.T) {
	bounds := pptx.RectEmu{X: 100000, Y: 200000, CX: 3000000, CY: 2000000}

	t.Run("default_values", func(t *testing.T) {
		spec := &OverlaySpec{}
		xml, err := GenerateImageOverlayXML(spec, 42, bounds)
		if err != nil {
			t.Fatal(err)
		}
		s := string(xml)
		if !strings.Contains(s, `<p:sp>`) {
			t.Error("expected p:sp element")
		}
		if !strings.Contains(s, `prst="rect"`) {
			t.Error("expected rect geometry")
		}
		// Default color 000000, default alpha 0.4 = 40000
		if !strings.Contains(s, `000000`) {
			t.Error("expected default black color")
		}
		if !strings.Contains(s, `<a:alpha`) {
			t.Error("expected alpha transparency")
		}
	})

	t.Run("custom_color_and_alpha", func(t *testing.T) {
		spec := &OverlaySpec{Color: "#003366", Alpha: 0.6}
		xml, err := GenerateImageOverlayXML(spec, 43, bounds)
		if err != nil {
			t.Fatal(err)
		}
		s := string(xml)
		if !strings.Contains(s, `003366`) {
			t.Error("expected custom color 003366")
		}
	})

	t.Run("scheme_color", func(t *testing.T) {
		spec := &OverlaySpec{Color: "dk1", Alpha: 0.5}
		xml, err := GenerateImageOverlayXML(spec, 44, bounds)
		if err != nil {
			t.Fatal(err)
		}
		s := string(xml)
		if !strings.Contains(s, `dk1`) {
			t.Error("expected scheme color dk1")
		}
	})
}

func TestGenerateImageTextXML(t *testing.T) {
	bounds := pptx.RectEmu{X: 100000, Y: 200000, CX: 3000000, CY: 2000000}

	t.Run("default_values", func(t *testing.T) {
		spec := &ImageText{Content: "Hello World"}
		xml, err := GenerateImageTextXML(spec, 50, bounds)
		if err != nil {
			t.Fatal(err)
		}
		s := string(xml)
		if !strings.Contains(s, "Hello World") {
			t.Error("expected text content")
		}
		if !strings.Contains(s, `<p:sp>`) {
			t.Error("expected p:sp element")
		}
		// Default white text
		if !strings.Contains(s, `FFFFFF`) {
			t.Error("expected default white color")
		}
	})

	t.Run("custom_styling", func(t *testing.T) {
		spec := &ImageText{
			Content: "Custom Label",
			Size:    18,
			Bold:    true,
			Color:   "00FF00",
			Align:   "l",
		}
		xml, err := GenerateImageTextXML(spec, 51, bounds)
		if err != nil {
			t.Fatal(err)
		}
		s := string(xml)
		if !strings.Contains(s, "Custom Label") {
			t.Error("expected text content")
		}
		if !strings.Contains(s, `00FF00`) {
			t.Error("expected custom green color")
		}
		if !strings.Contains(s, `b="1"`) {
			t.Error("expected bold attribute")
		}
	})
}

func TestBuildTextBody_BulletDetection(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantBullet bool
		wantText   string
	}{
		{"plain line", "Hello world", false, "Hello world"},
		{"dash bullet", "- First item", true, "First item"},
		{"numbered item", "1. Step one", true, "Step one"},
		{"multi-digit numbered", "12. Step twelve", true, "Step twelve"},
		{"not a bullet", "- ", true, ""},
		{"no space after dash", "-noSpace", false, "-noSpace"},
		{"no space after number", "1.noSpace", false, "1.noSpace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := buildTextBody(tt.content, 0, false, false, "l", "t", "", "", 0, 0, 0, 0)
			if len(tb.Paragraphs) != 1 {
				t.Fatalf("expected 1 paragraph, got %d", len(tb.Paragraphs))
			}
			p := tb.Paragraphs[0]
			hasBullet := p.Bullet != nil
			if hasBullet != tt.wantBullet {
				t.Errorf("bullet: got %v, want %v", hasBullet, tt.wantBullet)
			}
			if len(p.Runs) != 1 {
				t.Fatalf("expected 1 run, got %d", len(p.Runs))
			}
			if p.Runs[0].Text != tt.wantText {
				t.Errorf("text: got %q, want %q", p.Runs[0].Text, tt.wantText)
			}
			if hasBullet {
				if p.MarginL != bulletMarginLeft {
					t.Errorf("MarginL: got %d, want %d", p.MarginL, bulletMarginLeft)
				}
				if p.Indent != bulletIndent {
					t.Errorf("Indent: got %d, want %d", p.Indent, bulletIndent)
				}
			}
		})
	}
}

func TestParseNumberedPrefix(t *testing.T) {
	tests := []struct {
		line    string
		wantOK  bool
		wantNum int
		wantRem string
	}{
		{"1. First", true, 1, "First"},
		{"12. Twelfth", true, 12, "Twelfth"},
		{"0. Zero", true, 0, "Zero"},
		{"abc. Not", false, 0, ""},
		{"1.NoSpace", false, 0, ""},
		{". Leading dot", false, 0, ""},
		{"", false, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			num, rest, ok := pptx.ParseNumberedPrefix(tt.line)
			if ok != tt.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if ok {
				if num != tt.wantNum {
					t.Errorf("num: got %d, want %d", num, tt.wantNum)
				}
				if rest != tt.wantRem {
					t.Errorf("rest: got %q, want %q", rest, tt.wantRem)
				}
			}
		})
	}
}
