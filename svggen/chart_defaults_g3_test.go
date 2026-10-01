package svggen

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// svgTextRun is one rendered text element: its content, size and anchor.
type svgTextRun struct {
	text   string
	sizePx float64
	anchor string
}

var (
	g3TextRe      = regexp.MustCompile(`(?s)<text([^>]*)>(.*?)</text>`)
	svgFontSizeRe = regexp.MustCompile(`font-size:([0-9.]+)px`)
	svgAnchorRe   = regexp.MustCompile(`text-anchor="([a-z]+)"`)
	g3TagRe       = regexp.MustCompile(`<[^>]+>`)
)

func g3TextRuns(svg string) []svgTextRun {
	var out []svgTextRun
	for _, m := range g3TextRe.FindAllStringSubmatch(svg, -1) {
		run := svgTextRun{text: strings.TrimSpace(g3TagRe.ReplaceAllString(m[2], ""))}
		if s := svgFontSizeRe.FindStringSubmatch(m[1]); s != nil {
			run.sizePx, _ = strconv.ParseFloat(s[1], 64)
		}
		if a := svgAnchorRe.FindStringSubmatch(m[1]); a != nil {
			run.anchor = a[1]
		}
		out = append(out, run)
	}
	return out
}

func g3Texts(svg string) []string {
	var out []string
	for _, r := range g3TextRuns(svg) {
		out = append(out, r.text)
	}
	return out
}

func renderG3(t *testing.T, req *RequestEnvelope) string {
	t.Helper()
	if req.Output.Width == 0 {
		req.Output = OutputSpec{Width: 800, Height: 500}
	}
	doc, err := Render(req)
	if err != nil {
		t.Fatalf("render %s: %v", req.Type, err)
	}
	return string(doc.Content)
}

func indexOfText(texts []string, want string) int {
	for i, s := range texts {
		if s == want {
			return i
		}
	}
	return -1
}

func singleBar(cats []any, vals []any, extra map[string]any) map[string]any {
	d := map[string]any{"categories": cats, "series": []any{map[string]any{"name": "Units", "values": vals}}}
	for k, v := range extra {
		d[k] = v
	}
	return d
}

// go-slide-creator-oocqj (1, 3): a raw bar chart with few bars renders
// labelled — a value on each bar, no gridlines — and an explicit
// show_values:false (decoded from JSON) or data_labels:false opts out.
func TestBarChart_LabelledByDefaultWithOptOut(t *testing.T) {
	data := singleBar([]any{"Q1 2025", "Q2 2025", "Q3 2025"}, []any{12.0, 14.5, 15.2}, nil)
	svg := renderG3(t, &RequestEnvelope{Type: "bar_chart", Data: data})
	texts := g3Texts(svg)
	for _, want := range []string{"12.0", "14.5", "15.2"} {
		if indexOfText(texts, want) < 0 {
			t.Errorf("default bar chart lacks value label %q: %v", want, texts)
		}
	}
	if strings.Contains(svg, "stroke:#e0e0e0") {
		t.Error("labelled bar chart still draws gridlines")
	}

	var style StyleSpec
	if err := json.Unmarshal([]byte(`{"show_values": false}`), &style); err != nil {
		t.Fatal(err)
	}
	if !style.ShowValuesSet {
		t.Fatal("show_values:false was not recorded as explicit")
	}
	off := renderG3(t, &RequestEnvelope{Type: "bar_chart", Data: data, Style: style})
	if indexOfText(g3Texts(off), "14.5") >= 0 || !strings.Contains(off, "stroke:#e0e0e0") {
		t.Error("show_values:false should keep the axis + gridlines and drop the labels")
	}
	dl := singleBar([]any{"Q1 2025", "Q2 2025", "Q3 2025"}, []any{12.0, 14.5, 15.2}, map[string]any{"data_labels": false})
	if indexOfText(g3Texts(renderG3(t, &RequestEnvelope{Type: "bar_chart", Data: dl})), "14.5") >= 0 {
		t.Error("data.data_labels:false should drop the labels")
	}

	// The StyleSpec round-trips the explicit false, so the render cache
	// cannot confuse it with an omitted switch.
	raw, _ := json.Marshal(style)
	if !strings.Contains(string(raw), `"show_values":false`) {
		t.Errorf("explicit false lost on marshal: %s", raw)
	}
}

// go-slide-creator-oocqj (2): single-series line charts up to 12 points are
// labelled; multi-series lines are not.
func TestLineChart_LabelledDefaultOnlyForShortSingleSeries(t *testing.T) {
	if !DefaultDataLabels("line_chart", 1, 12) || DefaultDataLabels("line_chart", 1, 13) || DefaultDataLabels("line_chart", 2, 4) {
		t.Error("line default: want labels for 1 series <= 12 points only")
	}
	if !DefaultDataLabels("bar_chart", 2, 8) || DefaultDataLabels("bar_chart", 3, 6) {
		t.Error("bar default: want labels up to 16 bars")
	}
	if DefaultDataLabels("pie_chart", 1, 3) {
		t.Error("pie has its own labels; the bar/line default must not apply")
	}
}

// go-slide-creator-oocqj: data.sort ranks a single-series non-time chart
// descending by default, keeps periods and ordinal scales in order, and the
// highlight follows its bar.
func TestBarChart_SortDefaults(t *testing.T) {
	order := func(data map[string]any) []string {
		svg := renderG3(t, &RequestEnvelope{Type: "bar_chart", Data: data})
		var got []string
		for _, s := range g3Texts(svg) {
			switch s {
			case "Austria", "Belgium", "France", "Germany", "2023", "2024", "2025", "Low", "Medium", "High":
				got = append(got, s)
			}
		}
		return got
	}
	countries := []any{"Austria", "Belgium", "France", "Germany"}
	vals := []any{12.0, 8.0, 31.0, 42.0}
	if got := strings.Join(order(singleBar(countries, vals, nil)), ","); got != "Germany,France,Austria,Belgium" {
		t.Errorf("default sort = %s, want descending", got)
	}
	if got := strings.Join(order(singleBar(countries, vals, map[string]any{"sort": "asc"})), ","); got != "Belgium,Austria,France,Germany" {
		t.Errorf("asc sort = %s", got)
	}
	if got := strings.Join(order(singleBar(countries, vals, map[string]any{"sort": "none"})), ","); got != "Austria,Belgium,France,Germany" {
		t.Errorf("sort none = %s", got)
	}
	if got := strings.Join(order(singleBar([]any{"2023", "2024", "2025"}, []any{5.0, 9.0, 7.0}, nil)), ","); got != "2023,2024,2025" {
		t.Errorf("time categories must keep their order, got %s", got)
	}
	if got := strings.Join(order(singleBar([]any{"Low", "Medium", "High"}, []any{5.0, 9.0, 7.0}, nil)), ","); got != "Low,Medium,High" {
		t.Errorf("ordinal categories must keep their order, got %s", got)
	}
	if _, err := Render(&RequestEnvelope{Type: "bar_chart", Data: singleBar(countries, vals, map[string]any{"sort": "biggest"}), Output: OutputSpec{Width: 800, Height: 500}}); err == nil {
		t.Error("an invalid sort should be rejected")
	}

	// Highlight by name survives the reorder.
	chart := ChartData{Categories: []string{"A", "B", "C"}, Series: []ChartSeries{{Values: []float64{1, 3, 2}}}, Highlight: []int{0}, HighlightSet: true}
	applyCategorySort(&chart, SortDesc)
	if strings.Join(chart.Categories, "") != "BCA" || chart.Highlight[0] != 2 {
		t.Errorf("sorted %v, highlight %v; want BCA with A (now index 2) still highlighted", chart.Categories, chart.Highlight)
	}
}

// go-slide-creator-oocqj: data.orientation "horizontal" draws category names
// right-aligned in a left column with the values past the bar ends, and
// rejects an unknown orientation.
func TestBarChart_Horizontal(t *testing.T) {
	cats := []any{"Engineering & Product Development", "Sales and Marketing", "Legal & Compliance"}
	for _, typ := range []string{"bar_chart", "grouped_bar_chart", "stacked_bar_chart"} {
		data := map[string]any{"orientation": "horizontal", "categories": cats, "series": []any{
			map[string]any{"name": "FY24", "values": []any{12.4, 8.3, 0.9}},
			map[string]any{"name": "FY25", "values": []any{13.1, 7.9, 1.1}},
		}}
		if typ == "bar_chart" {
			data = singleBar(cats, []any{12.4, 8.3, 0.9}, map[string]any{"orientation": "horizontal"})
		}
		svg := renderG3(t, &RequestEnvelope{Type: typ, Data: data})
		runs := g3TextRuns(svg)
		var names int
		for _, r := range runs {
			if r.text == "Sales and Marketing" {
				names++
				if r.anchor != "end" {
					t.Errorf("%s: category name anchored %q, want right-aligned (end)", typ, r.anchor)
				}
			}
		}
		if names != 1 {
			t.Errorf("%s: category name drawn %d times: %v", typ, names, g3Texts(svg))
		}
	}
	if _, err := Render(&RequestEnvelope{Type: "bar_chart", Data: singleBar(cats, []any{1.0, 2.0, 3.0}, map[string]any{"orientation": "sideways"}), Output: OutputSpec{Width: 800, Height: 500}}); err == nil {
		t.Error("an unknown orientation should be rejected")
	}
}

// go-slide-creator-uz89d: stacked segment labels use the chart's one value
// format (10.4 next to 8.1, never 10), sit at or above the 10pt floor, and
// each stack carries its total.
func TestStackedBar_SegmentLabelsUniformAndTotalled(t *testing.T) {
	data := map[string]any{"categories": []any{"FY22", "FY23", "FY24", "FY25"}, "series": []any{
		map[string]any{"name": "Subscription", "values": []any{8.1, 9.6, 10.4, 12.2}},
		map[string]any{"name": "Services", "values": []any{2.4, 2.0, 3.1, 2.6}},
	}}
	svg := renderG3(t, &RequestEnvelope{Type: "stacked_bar_chart", Data: data, Output: OutputSpec{Width: 1000, Height: 600}})
	runs := g3TextRuns(svg)
	texts := g3Texts(svg)
	for _, want := range []string{"8.1", "9.6", "10.4", "12.2", "10.5", "11.6", "13.5", "14.8"} {
		if indexOfText(texts, want) < 0 {
			t.Errorf("missing label %q (segments and totals share one format): %v", want, texts)
		}
	}
	// The old rule printed values >= 10 without decimals ("12" for 12.2).
	if indexOfText(texts, "12") >= 0 {
		t.Errorf("12.2 printed as \"12\": %v", texts)
	}
	for _, r := range runs {
		if indexOfText([]string{"8.1", "9.6", "10.4", "12.2", "2.4", "2.0", "3.1", "2.6"}, r.text) >= 0 && r.sizePx < 13.3 {
			t.Errorf("segment label %q at %.2fpx is below the 10pt (13.33px) floor", r.text, r.sizePx)
		}
	}
	// data_labels:false drops segment labels and totals together.
	data["data_labels"] = false
	if indexOfText(g3Texts(renderG3(t, &RequestEnvelope{Type: "stacked_bar_chart", Data: data})), "14.8") >= 0 {
		t.Error("data_labels:false should drop the stack totals")
	}
}

// go-slide-creator-ihlsr: a pie sorts its slices largest-first, labels up to
// six slices "Name NN%" without a legend, folds the 2%/1% tail into Other,
// and paints a highlighted slice accent1 with the rest grey.
func TestPieChart_Defaults(t *testing.T) {
	data := map[string]any{"categories": []any{"B", "A", "C"}, "values": []any{25.0, 60.0, 15.0}}
	texts := g3Texts(renderG3(t, &RequestEnvelope{Type: "pie_chart", Data: data}))
	if indexOfText(texts, "A  60%") < 0 || indexOfText(texts, "C  15%") < 0 {
		t.Errorf("pie should label directly as \"Name  NN%%\": %v", texts)
	}
	if indexOfText(texts, "A") >= 0 {
		t.Errorf("a directly labelled pie should draw no legend: %v", texts)
	}

	chart := ChartData{Categories: []string{"a", "b", "c", "d", "e", "f", "g"}, Series: []ChartSeries{{Values: []float64{40, 25, 15, 12, 5, 2, 1}}}}
	if err := preparePieData(&RequestEnvelope{Data: map[string]any{}}, &chart); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(chart.Categories, ","); got != "a,b,c,d,e,Other" || chart.Series[0].Values[5] != 3 {
		t.Errorf("small-slice grouping = %s %v", got, chart.Series[0].Values)
	}
	chart = ChartData{Categories: []string{"x", "us", "y"}, Series: []ChartSeries{{Values: []float64{30, 50, 20}}}}
	if err := preparePieData(&RequestEnvelope{Data: map[string]any{"highlight": []any{"us"}}}, &chart); err != nil {
		t.Fatal(err)
	}
	if chart.Categories[0] != "us" || len(chart.Highlight) != 1 || chart.Highlight[0] != 0 {
		t.Errorf("sorted highlight: cats %v highlight %v", chart.Categories, chart.Highlight)
	}
	p := DefaultPalette()
	colors := pieHighlightColors(p, p.Accent1, 3, chart.Highlight)
	if colors[0] != p.Accent1 || colors[1] == p.Accent1 || colors[1] == colors[2] {
		t.Errorf("highlight colours: %v", colors)
	}
}

// go-slide-creator-ihlsr (2): outside labels of a run of thin slices
// (40/25/15/12/5/2/1) are spread so no two overlap.
func TestPieChart_OutsideLabelsDoNotOverlap(t *testing.T) {
	data := map[string]any{"categories": []any{"a", "b", "c", "d", "e", "f", "g"}, "values": []any{40.0, 25.0, 15.0, 12.0, 5.0, 2.0, 1.0}, "group_small_below_pct": 0.0, "sort": "none"}
	for _, typ := range []string{"pie_chart", "donut_chart"} {
		svg := renderG3(t, &RequestEnvelope{Type: typ, Data: data, Output: OutputSpec{Width: 800, Height: 500}})
		type box struct{ x, y float64 }
		re := regexp.MustCompile(`<text x="([0-9.]+)"[^>]*y="([0-9.]+)"[^>]*>(?:<tspan[^>]*>)?([^<]*%)(?:</tspan>)?</text>`)
		var labels []box
		for _, m := range re.FindAllStringSubmatch(svg, -1) {
			x, _ := strconv.ParseFloat(m[1], 64)
			y, _ := strconv.ParseFloat(m[2], 64)
			labels = append(labels, box{x, y})
		}
		if len(labels) < 7 {
			t.Fatalf("%s: found %d pct labels, want 7", typ, len(labels))
		}
		for i := range labels {
			for j := i + 1; j < len(labels); j++ {
				dx, dy := labels[i].x-labels[j].x, labels[i].y-labels[j].y
				if dx*dx < 40*40 && dy*dy < 12*12 {
					t.Errorf("%s: labels %d and %d overlap at (%.0f,%.0f) / (%.0f,%.0f)", typ, i, j, labels[i].x, labels[i].y, labels[j].x, labels[j].y)
				}
			}
		}
	}
}

// go-slide-creator-0fc8y: a treemap prints each cell's value (with its share
// when the values are not shares of 100) by default; show_values:false
// drops it.
func TestTreemap_ValueLabelsByDefault(t *testing.T) {
	data := map[string]any{"nodes": []any{
		map[string]any{"label": "Platform", "value": 54.0},
		map[string]any{"label": "Suite", "value": 31.0},
		map[string]any{"label": "API", "value": 35.0},
	}}
	req := &RequestEnvelope{Type: "treemap_chart", Data: data, Output: OutputSpec{Width: 900, Height: 600}}
	texts := g3Texts(renderG3(t, req))
	if indexOfText(texts, "54 (45%)") < 0 {
		t.Errorf("treemap should print value and share by default: %v", texts)
	}
	req.Style = StyleSpec{ShowValuesSet: true}
	if indexOfText(g3Texts(renderG3(t, req)), "54 (45%)") >= 0 {
		t.Error("show_values:false should drop the treemap value labels")
	}
}

// go-slide-creator-kbzu2: data.highlight on a multi-series line keeps the
// named series in colour at full width and turns the rest into thin grey
// context without markers; dense charts drop markers.
func TestLineChart_SeriesHighlight(t *testing.T) {
	series := []any{
		map[string]any{"name": "EMEA", "values": []any{10.0, 12.0, 13.0, 15.0, 18.0, 21.0}},
		map[string]any{"name": "NA", "values": []any{20.0, 21.0, 22.0, 22.0, 21.0, 20.0}},
		map[string]any{"name": "APAC", "values": []any{8.0, 9.0, 9.0, 10.0, 10.0, 9.0}},
	}
	data := map[string]any{"categories": []any{"Q1", "Q2", "Q3", "Q4", "Q5", "Q6"}, "series": series, "highlight": []any{"emea"}}
	req := &RequestEnvelope{Type: "line_chart", Data: data}
	chart, err := extractChartData(req)
	if err != nil {
		t.Fatal(err)
	}
	if !chart.SeriesHighlightSet || len(chart.SeriesHighlight) != 1 || chart.SeriesHighlight[0] != 0 {
		t.Fatalf("highlight should resolve to series 0, got %v %v", chart.SeriesHighlight, chart.SeriesHighlightSet)
	}
	p := DefaultPalette()
	colors := seriesHighlightColors(p, []Color{p.Accent1, p.Accent2, p.Accent3}, chart)
	neutral := NeutralInk(p, BarNeutralInk)
	if colors[0] != p.Accent1 || colors[1] != neutral || colors[2] != neutral {
		t.Errorf("series colours %v, want accent1 then neutral context", colors)
	}
	svg := renderG3(t, req)
	if !strings.Contains(svg, "stroke:"+strings.ToLower(neutral.Hex())) {
		t.Error("context lines are not drawn in the neutral ink")
	}
	if !lineMarkersByDefault(ChartData{Series: []ChartSeries{{Values: make([]float64, 6)}, {Values: make([]float64, 6)}}}) {
		t.Error("12 points should still carry markers")
	}
	if lineMarkersByDefault(chart) {
		t.Error("18 points should drop markers")
	}
	if got := DefaultSeriesHighlight("EMEA is the only region still growing", []string{"EMEA", "NA", "APAC"}); got != 0 {
		t.Errorf("title default highlight = %d, want 0", got)
	}
	if got := DefaultSeriesHighlight("EMEA overtook NA", []string{"EMEA", "NA"}); got != -1 {
		t.Errorf("two named series must not pick a default, got %d", got)
	}
	if got := DefaultSeriesHighlight("Banana growth", []string{"NA"}); got != -1 {
		t.Errorf("a name inside another word must not match, got %d", got)
	}
}
