package svggen

import (
	"fmt"
	"github.com/sebahrens/json2pptx/svggen/core"
	"math"
	"strings"
)

// ConsultingChartDiagram shares bounded text primitives, not chart semantics:
// each type validates and draws its own explicit data contract.
type ConsultingChartDiagram struct{ BaseDiagram }

func (d *ConsultingChartDiagram) Render(req *RequestEnvelope) (*SVGDocument, error) {
	return RenderFromBuilder(d.RenderWithBuilder, req)
}
func (d *ConsultingChartDiagram) RenderWithBuilder(req *RequestEnvelope) (*SVGBuilder, *SVGDocument, error) {
	if err := d.Validate(req); err != nil {
		return nil, nil, err
	}
	return RenderWithHelper(req, func(b *SVGBuilder, r *RequestEnvelope) error {
		var err error
		switch d.Type() {
		case "butterfly_chart":
			err = drawConsultingButterfly(b, r)
		case "mekko_chart":
			err = drawConsultingMekko(b, r)
		case "combo_chart":
			err = drawConsultingCombo(b, r)
		default:
			return fmt.Errorf("unknown consulting chart %s", d.Type())
		}
		if err != nil {
			return err
		}
		for i, a := range b.textBoxes {
			for _, other := range b.textBoxes[i+1:] {
				if a.rect.Intersects(other.rect) {
					return consultingChartError(fmt.Sprintf("labels %q and %q overlap", a.text, other.text))
				}
			}
		}
		return nil
	})
}
func (d *ConsultingChartDiagram) DataSchema() *DataSchema {
	numeric := ArrayDataSchema("Finite values", NumberDataSchema("Value"), 1)
	series := ObjectDataSchema("Named series", map[string]*DataSchema{"name": StringDataSchema("Series name"), "values": numeric}, []string{"name", "values"})
	switch d.Type() {
	case "butterfly_chart":
		return ObjectDataSchema("Mirrored nonnegative bars with a common scale", map[string]*DataSchema{"categories": ArrayDataSchema("2–12 center labels", StringDataSchema("Category"), 2), "left": series, "right": series}, []string{"categories", "left", "right"})
	case "mekko_chart":
		part := ObjectDataSchema("Named composition part", map[string]*DataSchema{"name": StringDataSchema("Part name"), "value": NumberDataSchema("Nonnegative weight")}, []string{"name", "value"})
		segment := ObjectDataSchema("Variable-width segment", map[string]*DataSchema{"name": StringDataSchema("Segment name"), "width": NumberDataSchema("Positive width weight"), "parts": ArrayDataSchema("2–6 matching part names", part, 2)}, []string{"name", "width", "parts"})
		return ObjectDataSchema("Variable-width, 100-percent stacked composition", map[string]*DataSchema{"segments": ArrayDataSchema("2–8 segments", segment, 2)}, []string{"segments"})
	case "combo_chart":
		return ObjectDataSchema("Bars and lines with explicit axis units", map[string]*DataSchema{"categories": ArrayDataSchema("2–12 shared categories", StringDataSchema("Category"), 2), "bars": ArrayDataSchema("1–3 bar series", series, 1), "lines": ArrayDataSchema("1–2 line series", series, 1), "secondary_axis": BooleanDataSchema("Place lines on the labeled right scale"), "bar_unit": StringDataSchema("Left-axis unit"), "line_unit": StringDataSchema("Line-axis unit"), "highlight": StringDataSchema("Bar series to emphasize")}, []string{"categories", "bars", "lines", "bar_unit", "line_unit"})
	}
	return nil
}
func (d *ConsultingChartDiagram) Validate(req *RequestEnvelope) error {
	if req == nil {
		return fmt.Errorf("chart request is required")
	}
	if req.Style.Scale != "" && req.Style.Scale != "linear" {
		return fmt.Errorf("%s supports only linear scale", d.Type())
	}
	if err := core.ValidateUnknownFields(req.Data, d.DataSchema(), d.Type()); err != nil {
		return err
	}
	switch d.Type() {
	case "butterfly_chart":
		return validateConsultingButterfly(req.Data)
	case "mekko_chart":
		return validateConsultingMekko(req.Data)
	case "combo_chart":
		return validateConsultingCombo(req.Data)
	}
	return nil
}
func consultingChartError(message string) error {
	return fmt.Errorf("chart geometry: %s; use fewer categories/series or a larger frame", message)
}
func consultingChartText(b *SVGBuilder, text string, x, y, maxW float64, align TextAlign) error {
	b.SetFontSize(max(12.0, b.MinFontSize())).SetFontWeight(400).SetTextColor(b.style.Palette.TextPrimary)
	w, h := b.MeasureText(text)
	left := x
	if align == TextAlignCenter {
		left -= w / 2
	}
	if align == TextAlignRight {
		left -= w
	}
	if w > maxW || left < 0 || left+w > b.Width() || y-h/2 < 0 || y+h/2 > b.Height() {
		return consultingChartError(fmt.Sprintf("label %q does not fit at 12pt", text))
	}
	b.DrawText(text, x, y, align, TextBaselineMiddle)
	return nil
}
func consultingChartTitle(b *SVGBuilder, req *RequestEnvelope) error {
	if req.Title == "" {
		return nil
	}
	return consultingChartText(b, req.Title, b.Width()/2, 20, b.Width()-30, TextAlignCenter)
}
func consultingFinite(v any) (float64, bool) {
	n, ok := v.(float64)
	if !ok {
		if i, yes := v.(int); yes {
			n = float64(i)
			ok = true
		}
	}
	return n, ok && !math.IsNaN(n) && !math.IsInf(n, 0)
}
func consultingNamedSeries(v any, n int, nonnegative bool) (string, []float64, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("series must be an object")
	}
	name, ok := m["name"].(string)
	if !ok || name == "" {
		return "", nil, fmt.Errorf("series name is required")
	}
	raw, ok := toAnySlice(m["values"])
	if !ok || len(raw) != n {
		return "", nil, fmt.Errorf("series %s needs one value per category", name)
	}
	values := make([]float64, n)
	for i, v := range raw {
		number, valid := consultingFinite(v)
		if !valid || (nonnegative && number < 0) {
			return "", nil, fmt.Errorf("series %s values must be finite and nonnegative", name)
		}
		values[i] = number
	}
	return name, values, nil
}
func validateConsultingButterfly(data map[string]any) error {
	cats, ok := toStringSlice(data["categories"])
	if !ok || len(cats) < 2 || len(cats) > 12 {
		return fmt.Errorf("butterfly_chart needs 2–12 categories")
	}
	for _, key := range []string{"left", "right"} {
		if _, _, err := consultingNamedSeries(data[key], len(cats), true); err != nil {
			return err
		}
	}
	return nil
}
func drawConsultingButterfly(b *SVGBuilder, req *RequestEnvelope) error {
	if err := consultingChartTitle(b, req); err != nil {
		return err
	}
	cats, _ := toStringSlice(req.Data["categories"])
	ln, left, _ := consultingNamedSeries(req.Data["left"], len(cats), true)
	rn, right, _ := consultingNamedSeries(req.Data["right"], len(cats), true)
	top := 65.0
	bottom := 25.0
	rowH := (b.Height() - top - bottom) / float64(len(cats))
	if rowH < 30 {
		return consultingChartError("mirrored rows need 30pt per category")
	}
	centerW := 0.0
	b.SetFontSize(max(12.0, b.MinFontSize()))
	for _, cat := range cats {
		w, _ := b.MeasureText(cat)
		centerW = max(centerW, w+24)
	}
	half := (b.Width() - centerW - 130) / 2
	if half < 60 {
		return consultingChartError("center labels leave no bar area")
	}
	lx := (b.Width() - centerW) / 2
	rx := lx + centerW
	if err := consultingChartText(b, ln, lx-half/2, 45, half, TextAlignCenter); err != nil {
		return err
	}
	if err := consultingChartText(b, rn, rx+half/2, 45, half, TextAlignCenter); err != nil {
		return err
	}
	maxV := 0.0
	for i := range cats {
		maxV = max(maxV, max(left[i], right[i]))
	}
	if maxV == 0 {
		maxV = 1
	}
	colors := consultingChartColors(req, b)
	for i, cat := range cats {
		y := top + (float64(i)+.5)*rowH
		h := min(18.0, rowH*.55)
		lw, rw := half*left[i]/maxV, half*right[i]/maxV
		b.SetFillColor(colors[0]).DrawRect(Rect{X: lx - lw, Y: y - h/2, W: lw, H: h})
		b.SetFillColor(colors[min(1, len(colors)-1)]).DrawRect(Rect{X: rx, Y: y - h/2, W: rw, H: h})
		if err := consultingChartText(b, cat, b.Width()/2, y, centerW-12, TextAlignCenter); err != nil {
			return err
		}
		if err := consultingChartText(b, fmt.Sprintf("%g", left[i]), lx-lw-8, y, 55, TextAlignRight); err != nil {
			return err
		}
		if err := consultingChartText(b, fmt.Sprintf("%g", right[i]), rx+rw+8, y, 55, TextAlignLeft); err != nil {
			return err
		}
	}
	return nil
}

func consultingMekkoParts(segment map[string]any) ([]map[string]any, float64, error) {
	raw, ok := toAnySlice(segment["parts"])
	if !ok || len(raw) < 2 || len(raw) > 6 {
		return nil, 0, fmt.Errorf("mekko segments require 2–6 parts")
	}
	out := make([]map[string]any, len(raw))
	total := 0.0
	seen := map[string]bool{}
	for i, r := range raw {
		p, ok := r.(map[string]any)
		if !ok {
			return nil, 0, fmt.Errorf("mekko part must be an object")
		}
		name, ok := p["name"].(string)
		if !ok || name == "" || seen[name] {
			return nil, 0, fmt.Errorf("mekko part names must be nonempty and unique")
		}
		seen[name] = true
		n, ok := consultingFinite(p["value"])
		if !ok || n < 0 {
			return nil, 0, fmt.Errorf("mekko part values must be finite and nonnegative")
		}
		total += n
		out[i] = p
	}
	if total <= 0 || math.IsInf(total, 0) {
		return nil, 0, fmt.Errorf("mekko segment composition must have positive finite total")
	}
	return out, total, nil
}
func validateConsultingMekko(data map[string]any) error {
	segments, ok := toAnySlice(data["segments"])
	if !ok || len(segments) < 2 || len(segments) > 8 {
		return fmt.Errorf("mekko_chart requires 2–8 segments")
	}
	var names []string
	totalWidth := 0.0
	for _, raw := range segments {
		s, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("mekko segment must be an object")
		}
		name, ok := s["name"].(string)
		if !ok || name == "" {
			return fmt.Errorf("mekko segment name is required")
		}
		w, ok := consultingFinite(s["width"])
		if !ok || w <= 0 {
			return fmt.Errorf("mekko width must be finite and positive")
		}
		totalWidth += w
		parts, _, err := consultingMekkoParts(s)
		if err != nil {
			return err
		}
		if names == nil {
			for _, p := range parts {
				partName, _ := p["name"].(string)
				names = append(names, partName)
			}
		} else {
			if len(parts) != len(names) {
				return fmt.Errorf("mekko segments must use the same ordered part names")
			}
			for i, p := range parts {
				if p["name"] != names[i] {
					return fmt.Errorf("mekko segments must use the same ordered part names")
				}
			}
		}
	}
	if math.IsInf(totalWidth, 0) {
		return fmt.Errorf("mekko total width must be finite")
	}
	return nil
}
func drawConsultingMekko(b *SVGBuilder, req *RequestEnvelope) error {
	if err := consultingChartTitle(b, req); err != nil {
		return err
	}
	segments, _ := toAnySlice(req.Data["segments"])
	totalWidth := 0.0
	for _, raw := range segments {
		s, _ := raw.(map[string]any)
		w, _ := consultingFinite(s["width"])
		totalWidth += w
	}
	x, top, plotW, plotH := 25.0, 55.0, b.Width()-50, b.Height()-110
	var zeroParts []string
	if plotW < 120 || plotH < 80 {
		return consultingChartError("mekko plot is too small")
	}
	colors := consultingChartColors(req, b)
	for _, raw := range segments {
		s, _ := raw.(map[string]any)
		weight, _ := consultingFinite(s["width"])
		w := plotW * (weight / totalWidth)
		parts, total, _ := consultingMekkoParts(s)
		y := top + plotH
		for j, p := range parts {
			value, _ := consultingFinite(p["value"])
			h := plotH * (value / total)
			y -= h
			if value == 0 {
				zeroParts = append(zeroParts, fmt.Sprintf("%v/%v:0%%", s["name"], p["name"]))
				continue
			}
			fill := colors[j%len(colors)]
			paper := b.style.Palette.Background
			fill = Color{R: uint8((int(fill.R) + 3*int(paper.R)) / 4), G: uint8((int(fill.G) + 3*int(paper.G)) / 4), B: uint8((int(fill.B) + 3*int(paper.B)) / 4), A: 1}
			b.SetFillColor(fill).DrawRect(Rect{X: x, Y: y, W: w, H: h})
			label := fmt.Sprintf("%s %.0f%%", p["name"], value/total*100)
			b.SetFontSize(max(12.0, b.MinFontSize()))
			_, textH := b.MeasureText(label)
			if h < textH+8 {
				return consultingChartError("mekko part height cannot hold its label")
			}
			if err := consultingChartText(b, label, x+w/2, y+h/2, w-12, TextAlignCenter); err != nil {
				return err
			}
		}
		label := fmt.Sprintf("%s (width %g)", s["name"], weight)
		if err := consultingChartText(b, label, x+w/2, top+plotH+23, w-8, TextAlignCenter); err != nil {
			return err
		}
		x += w
	}
	if len(zeroParts) > 0 {
		return consultingChartText(b, strings.Join(zeroParts, "; "), b.Width()/2, b.Height()-10, b.Width()-20, TextAlignCenter)
	}
	return nil
}

func consultingComboSeries(data map[string]any, key string, n int) ([]string, [][]float64, error) {
	raw, ok := toAnySlice(data[key])
	maxN := 3
	if key == "lines" {
		maxN = 2
	}
	if !ok || len(raw) < 1 || len(raw) > maxN {
		return nil, nil, fmt.Errorf("combo_chart %s requires 1–%d series", key, maxN)
	}
	names := make([]string, len(raw))
	values := make([][]float64, len(raw))
	for i, s := range raw {
		var err error
		names[i], values[i], err = consultingNamedSeries(s, n, true)
		if err != nil {
			return nil, nil, err
		}
	}
	return names, values, nil
}
func validateConsultingCombo(data map[string]any) error {
	cats, ok := toStringSlice(data["categories"])
	if !ok || len(cats) < 2 || len(cats) > 12 {
		return fmt.Errorf("combo_chart needs 2–12 categories")
	}
	bars, _, err := consultingComboSeries(data, "bars", len(cats))
	if err != nil {
		return err
	}
	_, _, err = consultingComboSeries(data, "lines", len(cats))
	if err != nil {
		return err
	}
	for _, key := range []string{"bar_unit", "line_unit"} {
		if _, ok := data[key].(string); !ok {
			return fmt.Errorf("combo_chart requires %s (empty string means no unit)", key)
		}
	}
	secondary, ok := data["secondary_axis"].(bool)
	if _, set := data["secondary_axis"]; set && !ok {
		return fmt.Errorf("secondary_axis must be boolean")
	}
	if !secondary && data["bar_unit"] != data["line_unit"] {
		return fmt.Errorf("different units require secondary_axis:true")
	}
	if h, set := data["highlight"]; set {
		found := false
		for _, name := range bars {
			found = found || h == name
		}
		if !found {
			return fmt.Errorf("highlight must name a bar series")
		}
	}
	return nil
}
func drawConsultingCombo(b *SVGBuilder, req *RequestEnvelope) error {
	if err := consultingChartTitle(b, req); err != nil {
		return err
	}
	cats, _ := toStringSlice(req.Data["categories"])
	barNames, bars, _ := consultingComboSeries(req.Data, "bars", len(cats))
	lineNames, lines, _ := consultingComboSeries(req.Data, "lines", len(cats))
	secondary, _ := req.Data["secondary_axis"].(bool)
	left, right, top, bottom := 65.0, b.Width()-65, 85.0, b.Height()-45
	w, h := right-left, bottom-top
	if w < 160 || h < 100 {
		return consultingChartError("combo plot is too small")
	}
	maxBar, maxLine := 1.0, 1.0
	for _, s := range bars {
		for _, v := range s {
			maxBar = max(maxBar, v)
		}
	}
	for _, s := range lines {
		for _, v := range s {
			maxLine = max(maxLine, v)
		}
	}
	if !secondary {
		maxBar = max(maxBar, maxLine)
		maxLine = maxBar
	}
	if err := consultingComboAxes(b, req, left, right, top, bottom, maxBar, maxLine, secondary); err != nil {
		return err
	}
	colors := consultingChartColors(req, b)
	legend := "Bars left→right: "
	for _, name := range barNames {
		legend += name + "   "
	}
	for _, name := range lineNames {
		legend += "Line: " + name + "   "
	}
	if err := consultingChartText(b, legend, b.Width()/2, 50, b.Width()-30, TextAlignCenter); err != nil {
		return err
	}
	band := w / float64(len(cats))
	barW := band * .65 / float64(len(bars))
	if barW < 6 {
		return consultingChartError("combo bars are too narrow")
	}
	neutral := b.style.Palette.TextMuted
	highlight, _ := req.Data["highlight"].(string)
	for i, cat := range cats {
		x := left + (float64(i)+.5)*band
		if err := consultingChartText(b, cat, x, bottom+23, band-4, TextAlignCenter); err != nil {
			return err
		}
		for j, s := range bars {
			color := neutral
			if highlight == barNames[j] {
				color = colors[0]
			}
			bh := h * (s[i] / maxBar)
			b.SetFillColor(color).DrawRect(Rect{X: x - band*.325 + float64(j)*barW, Y: bottom - bh, W: barW - 1, H: bh})
		}
	}
	for j, s := range lines {
		color := colors[j%len(colors)]
		b.SetStrokeColor(color).SetStrokeWidth(float64(2 + j))
		for i := 1; i < len(s); i++ {
			x1 := left + (float64(i)-.5)*band
			x2 := left + (float64(i)+.5)*band
			b.DrawLine(x1, bottom-h*(s[i-1]/maxLine), x2, bottom-h*(s[i]/maxLine))
		}
		x := left + (float64(len(cats))-.5)*band
		y := bottom - h*(s[len(s)-1]/maxLine)
		if err := consultingChartText(b, lineNames[j], x, y-13, band*2, TextAlignCenter); err != nil {
			return err
		}
	}
	return nil
}
func consultingComboAxes(b *SVGBuilder, req *RequestEnvelope, left, right, top, bottom, maxBar, maxLine float64, secondary bool) error {
	ink := b.style.Palette.TextPrimary
	b.SetStrokeColor(ink).SetStrokeWidth(.6).DrawLine(left, bottom, right, bottom)
	b.DrawLine(left, top, left, bottom)
	for i := 0; i <= 4; i++ {
		fraction := float64(i) / 4
		y := bottom - (bottom-top)*fraction
		if err := consultingChartText(b, fmt.Sprintf("%.3g", maxBar*fraction), left-8, y, 50, TextAlignRight); err != nil {
			return err
		}
		if secondary {
			if err := consultingChartText(b, fmt.Sprintf("%.3g", maxLine*fraction), right+8, y, 50, TextAlignLeft); err != nil {
				return err
			}
		}
	}
	barUnit, _ := req.Data["bar_unit"].(string)
	lineUnit, _ := req.Data["line_unit"].(string)
	if err := consultingChartText(b, barUnit, left, top-20, 120, TextAlignLeft); err != nil {
		return err
	}
	if secondary {
		return consultingChartText(b, lineUnit, right, top-20, 120, TextAlignRight)
	}
	return nil
}

func consultingChartColors(req *RequestEnvelope, b *SVGBuilder) []Color {
	return resolveColors(extractChartColorsForRequest(req, b), b.style, 6)
}
