package svggen

import (
	"math"
	"strings"
	"testing"
)

// smRequest builds a small_multiples_chart request with one panel per values
// list (go-slide-creator-sxpvy).
func smRequest(w, h int, names []string, values [][]any, extra map[string]any) *RequestEnvelope {
	series := make([]any, len(names))
	for i, n := range names {
		series[i] = map[string]any{"name": n, "values": values[i]}
	}
	data := map[string]any{
		"categories": []any{"Q1", "Q2", "Q3", "Q4"},
		"series":     series,
	}
	for k, v := range extra {
		data[k] = v
	}
	return &RequestEnvelope{
		Type:   "small_multiples",
		Title:  "Regional revenue ($M)",
		Data:   data,
		Output: OutputSpec{Width: w, Height: h},
	}
}

// smLayout reproduces the renderer's layout for a request so tests can
// inspect panel geometry.
func smLayout(t *testing.T, req *RequestEnvelope) (*smallMultiples, smMetrics, smGrid, bool) {
	t.Helper()
	var sm *smallMultiples
	var m smMetrics
	var g smGrid
	var ok bool
	_, _, err := RenderWithHelper(req, func(b *SVGBuilder, req *RequestEnvelope) error {
		data, err := extractChartData(req)
		if err != nil {
			return err
		}
		mode, err := resolveYScaleMode(req.Data)
		if err != nil {
			return err
		}
		sm = &smallMultiples{b: b, data: data, mode: mode, config: DefaultChartConfig(b.Width(), b.Height()), showTitle: req.Title != ""}
		m = sm.measure()
		g, ok = chooseGrid(sm.grids(m, len(data.Series), m.panelTitleWidths))
		return nil
	})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	return sm, m, g, ok
}

// relative returns points relative to their plot's origin.
func relative(pts []Point, plot Rect) []Point {
	out := make([]Point, len(pts))
	for i, p := range pts {
		out[i] = Point{X: p.X - plot.X, Y: p.Y - plot.Y}
	}
	return out
}

func TestSmallMultiples_SharedScaleShowsMagnitude(t *testing.T) {
	req := smRequest(900, 500, []string{"Low", "High"}, [][]any{{10, 20, 15, 25}, {100, 200, 150, 250}}, nil)
	sm, m, g, ok := smLayout(t, req)
	if !ok {
		t.Fatalf("a 900x500 canvas fits two panels; grid failures: %v", g.failures)
	}
	if m.domains[0] != m.domains[1] {
		t.Fatalf("shared mode must use one domain, got %v and %v", m.domains[0], m.domains[1])
	}
	low := relative(panelPoints(panelRect(m, g, 0), m.domains[0], sm.data.Categories, sm.data.Series[0].Values), panelRect(m, g, 0))
	high := relative(panelPoints(panelRect(m, g, 1), m.domains[1], sm.data.Categories, sm.data.Series[1].Values), panelRect(m, g, 1))
	identical := true
	for i := range low {
		if math.Abs(low[i].Y-high[i].Y) > 0.01 {
			identical = false
		}
	}
	if identical {
		t.Fatal("10x data drew identical geometry: the shared scale is not applied")
	}
	// The high panel's Q4 must sit far higher (smaller y) than the low panel's.
	if hl, ll := panelRect(m, g, 0).H-low[3].Y, panelRect(m, g, 1).H-high[3].Y; ll < 5*hl {
		t.Errorf("Q4 heights above the baseline: low %.1f, high %.1f — want the 10x panel ~10x taller", hl, ll)
	}
}

func TestSmallMultiples_EqualValuesEqualPositions(t *testing.T) {
	cases := map[string][][]any{
		"positive": {{10, 20, 15, 25}, {10, 20, 15, 25}, {100, 200, 150, 250}},
		"negative": {{-30, -10, 5, -20}, {-30, -10, 5, -20}, {4, 8, 2, 6}},
		"constant": {{7, 7, 7, 7}, {7, 7, 7, 7}, {7, 7, 7, 7}},
	}
	for name, values := range cases {
		t.Run(name, func(t *testing.T) {
			req := smRequest(900, 500, []string{"A", "B — a much longer panel title", "C"}, values, nil)
			sm, m, g, _ := smLayout(t, req)
			var plots []Rect
			for i := range values {
				plots = append(plots, panelRect(m, g, i))
			}
			for i := 1; i < len(plots); i++ {
				if math.Abs(plots[i].W-plots[0].W) > 1e-9 || math.Abs(plots[i].H-plots[0].H) > 1e-9 {
					t.Fatalf("panel %d plot %vx%v differs from panel 0 %vx%v", i, plots[i].W, plots[i].H, plots[0].W, plots[0].H)
				}
			}
			a := relative(panelPoints(plots[0], m.domains[0], sm.data.Categories, sm.data.Series[0].Values), plots[0])
			b := relative(panelPoints(plots[1], m.domains[1], sm.data.Categories, sm.data.Series[1].Values), plots[1])
			for i := range a {
				if math.Abs(a[i].X-b[i].X) > 1e-9 || math.Abs(a[i].Y-b[i].Y) > 1e-9 {
					t.Errorf("point %d: %v vs %v — equal values must land at identical positions", i, a[i], b[i])
				}
				if a[i].Y < -1e-9 || a[i].Y > plots[0].H+1e-9 {
					t.Errorf("point %d at y=%.2f is outside its plot (h=%.2f)", i, a[i].Y, plots[0].H)
				}
			}
		})
	}
}

func TestSmallMultiples_TitleLengthDoesNotMisalign(t *testing.T) {
	short := smRequest(900, 500, []string{"A", "B", "C", "D"}, [][]any{{1, 2, 3, 4}, {1, 2, 3, 4}, {1, 2, 3, 4}, {1, 2, 3, 4}}, nil)
	long := smRequest(900, 500, []string{"A", "Enterprise and public sector", "C", "D"}, [][]any{{1, 2, 3, 4}, {1, 2, 3, 4}, {1, 2, 3, 4}, {1, 2, 3, 4}}, nil)
	_, ms, gs, _ := smLayout(t, short)
	_, ml, gl, _ := smLayout(t, long)
	if gs.cols != gl.cols {
		t.Skipf("grids differ (%d vs %d cols); alignment compares one grid", gs.cols, gl.cols)
	}
	for i := 0; i < 4; i++ {
		if panelRect(ms, gs, i) != panelRect(ml, gl, i) {
			t.Errorf("panel %d moved with a longer title: %v vs %v", i, panelRect(ms, gs, i), panelRect(ml, gl, i))
		}
	}
	// Panels in one row share their top and height; panels in one column share left and width.
	for i := 1; i < 4; i++ {
		p0, pi := panelRect(ml, gl, 0), panelRect(ml, gl, i)
		if i/gl.cols == 0 && (pi.Y != p0.Y || pi.H != p0.H) {
			t.Errorf("panel %d not aligned with panel 0 in its row: %v vs %v", i, pi, p0)
		}
		if i%gl.cols == 0 && (pi.X != p0.X || pi.W != p0.W) {
			t.Errorf("panel %d not aligned with panel 0 in its column: %v vs %v", i, pi, p0)
		}
	}
}

func TestSmallMultiples_CommonTickPrecision(t *testing.T) {
	// Independent domains with different tick steps (0.2 vs 50) must print one precision.
	req := smRequest(900, 500, []string{"Rate", "Volume"}, [][]any{{0.2, 0.5, 0.9, 1.1}, {100, 200, 150, 250}}, map[string]any{"y_scale": "independent"})
	_, m, _, _ := smLayout(t, req)
	if m.tickDecimals < 1 {
		t.Fatalf("common precision should cover the 0.x panel, got %d decimals", m.tickDecimals)
	}
	doc, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	svg := string(doc.Content)
	if !strings.Contains(svg, "100.0") && !strings.Contains(svg, "150.0") && !strings.Contains(svg, "200.0") {
		t.Errorf("the Volume panel ticks should carry the shared decimal precision")
	}
}

func TestSmallMultiples_IndependentIsExplicitAndLabelled(t *testing.T) {
	values := [][]any{{10, 20, 15, 25}, {100, 200, 150, 250}}
	shared, err := Render(smRequest(900, 500, []string{"Low", "High"}, values, nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(shared.Content), "Independent y-axes") {
		t.Error("the default (shared) chart must not carry the independent-scale label")
	}
	req := smRequest(900, 500, []string{"Low", "High"}, values, map[string]any{"y_scale": "independent"})
	_, m, _, _ := smLayout(t, req)
	if m.domains[0] == m.domains[1] {
		t.Error("independent mode should give each panel its own domain")
	}
	ind, err := Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ind.Content), "Independent y-axes") {
		t.Error("an independent-scale chart must be labelled so on the canvas")
	}
}

func TestSmallMultiples_Validation(t *testing.T) {
	one := smRequest(900, 500, []string{"Solo"}, [][]any{{1, 2, 3, 4}}, nil)
	if _, err := Render(one); err == nil || !strings.Contains(err.Error(), "line_chart") {
		t.Errorf("one panel should be rejected towards line_chart, got %v", err)
	}
	names := []string{"A", "B", "C", "D", "E", "F", "G"}
	vals := make([][]any, 7)
	for i := range vals {
		vals[i] = []any{1, 2, 3, 4}
	}
	if _, err := Render(smRequest(900, 500, names, vals, nil)); err == nil || !strings.Contains(err.Error(), "split") {
		t.Errorf("seven panels should be rejected with split advice, got %v", err)
	}
	if _, err := Render(smRequest(900, 500, []string{"A", "B"}, [][]any{{1, 2, 3, 4}, {1, 2, 3, 4}}, map[string]any{"y_scale": "log"})); err == nil {
		t.Error("an unknown y_scale must be rejected")
	}
	if _, err := Render(smRequest(900, 500, []string{"A", "B"}, [][]any{{1, 2, 3}, {1, 2, 3, 4}}, nil)); err == nil {
		t.Error("a panel with the wrong number of values must be rejected")
	}
	if _, err := Render(smRequest(900, 500, []string{"A", ""}, [][]any{{1, 2, 3, 4}, {1, 2, 3, 4}}, nil)); err == nil {
		t.Error("an unnamed panel must be rejected: its name is the panel title")
	}
	if _, err := Render(smRequest(900, 500, []string{"A", "B"}, [][]any{{1, 2, 3, 4}, {1, 2, 3, 4}}, map[string]any{"highlight": []any{"Z"}})); err == nil {
		t.Error("a highlight naming no panel must be rejected")
	}
	if _, err := Render(smRequest(900, 500, []string{"A", "B"}, [][]any{{1, 2, 3, 4}, {1, 2, 3, 4}}, map[string]any{"y_min": 0})); err == nil {
		t.Error("unknown data fields must be rejected by the schema")
	}
}

func TestSmallMultiples_CapabilityAdvertisesCanonicalName(t *testing.T) {
	for _, c := range ChartCapabilities() {
		if c.Type != "small_multiples" {
			continue
		}
		if len(c.Aliases) != 1 || c.Aliases[0] != SmallMultiplesChartType {
			t.Fatalf("aliases = %v, want [%s]", c.Aliases, SmallMultiplesChartType)
		}
		if c.MaxSeries == nil || *c.MaxSeries != SmallMultiplesMaxPanels {
			t.Errorf("max_series should be the %d-panel cap", SmallMultiplesMaxPanels)
		}
		return
	}
	t.Fatal("small_multiples missing from ChartCapabilities")
}

func TestSmallMultiples_UnreadableCanvasIsReported(t *testing.T) {
	names := []string{"North", "South", "East", "West", "Central", "Overseas"}
	vals := make([][]any, len(names))
	for i := range vals {
		vals[i] = []any{1, 2, 3, 4}
	}
	builder, _, err := (&SmallMultiplesDiagram{NewBaseDiagram(SmallMultiplesChartType)}).RenderWithBuilder(smRequest(260, 160, names, vals, nil))
	if err != nil {
		t.Fatal(err)
	}
	var found *Finding
	for _, f := range builder.Findings() {
		if f.Code == FindingPlotAreaCollapsed {
			f := f
			found = &f
		}
	}
	if found == nil {
		t.Fatalf("six panels on a 260x160 canvas must report %s, got %+v", FindingPlotAreaCollapsed, builder.Findings())
	}
	if found.Severity != "shrink_or_split" || found.Fix == nil || found.Fix.Kind != FixKindTruncateOrSplit {
		t.Errorf("finding should ask to shrink or split: %+v", found)
	}

	roomy, _, err := (&SmallMultiplesDiagram{NewBaseDiagram(SmallMultiplesChartType)}).RenderWithBuilder(smRequest(1100, 600, names, vals, nil))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range roomy.Findings() {
		if f.Code == FindingPlotAreaCollapsed {
			t.Errorf("six panels on 1100x600 are readable, got %+v", f)
		}
	}
}
