package semantic

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestQBRExample_ChronologyAndMeasures pins the period and measure labels of
// the shipped examples/semantic/qbr.yaml (go-slide-creator-z99un). The example
// once presented completed Q1–Q4 FY26 revenue in a Q2 FY26 review and labelled
// the same 48 as both ARR and quarterly revenue; it rendered cleanly and scored
// 99, so nothing else caught it. Agents copy this file, so it must stay
// internally coherent.
//
// This is a fixture-content test, not a prose truth detector: it checks the
// structured labels (deck period, chart categories and title, KPI labels,
// sources) and the two numbers the summary and KPI slide derive from the chart.
func TestQBRExample_ChronologyAndMeasures(t *testing.T) {
	data, err := os.ReadFile(qbrExamplePath)
	if err != nil {
		t.Fatalf("read %s: %v", qbrExamplePath, err)
	}
	spec, diags := Parse(qbrExamplePath, data)
	if diags.HasErrors() {
		t.Fatalf("parse: %v", diags)
	}

	// The as-of period is the review quarter named in the deck title.
	asOf, ok := firstQuarter(spec.Meta.Title, 0)
	if !ok || !asOf.hasFY {
		t.Fatalf("meta.title %q must name the review quarter as \"Qn FYyy\"", spec.Meta.Title)
	}

	var chart map[string]any
	var arrValue string
	for i, s := range spec.Slides {
		switch s.Kind {
		case "kpi_snapshot", "chart_insight":
			// Evidence slides report actuals: no field may cite a quarter
			// after the as-of quarter unless it says it is a forecast.
			for _, str := range collectStrings(s.Body) {
				for _, q := range allQuarters(str, asOf.fy) {
					if q.after(asOf) && !strings.Contains(strings.ToLower(str), "forecast") {
						t.Errorf("slide %d (%s): %q cites %s, after the as-of quarter %s, without marking it forecast",
							i, s.Kind, str, q, asOf)
					}
				}
			}
			if src := s.String("source"); src != "" {
				if q, ok := firstQuarter(src, asOf.fy); !ok || !q.same(asOf) {
					t.Errorf("slide %d (%s): source %q must name the as-of quarter %s", i, s.Kind, src, asOf)
				}
			}
		}
		if s.Kind == "kpi_snapshot" {
			kpis, _ := s.Body["kpis"].([]any)
			for _, k := range kpis {
				m, _ := k.(map[string]any)
				label, _ := m["label"].(string)
				if isARRLabel(label) {
					arrValue, _ = m["value"].(string)
					if strings.Contains(strings.ToLower(label), "revenue") && !strings.Contains(strings.ToLower(label), "recurring") {
						t.Errorf("ARR KPI label %q reads as period revenue", label)
					}
				}
			}
		}
		if s.Kind == "chart_insight" {
			chart, _ = s.Body["chart"].(map[string]any)
		}
	}
	if chart == nil {
		t.Fatal("qbr example has no chart_insight chart")
	}
	if arrValue == "" {
		t.Fatal("qbr example has no ARR KPI")
	}

	// Chart: period (quarterly) revenue actuals, never ARR.
	title, _ := chart["title"].(string)
	lt := strings.ToLower(title)
	if !strings.Contains(lt, "quarterly revenue") || !strings.Contains(lt, "actual") {
		t.Errorf("chart title %q must say it shows quarterly revenue actuals", title)
	}
	if isARRLabel(title) {
		t.Errorf("chart title %q conflates ARR with period revenue", title)
	}

	data2, _ := chart["data"].(map[string]any)
	rawCats, _ := data2["categories"].([]any)
	var cats []quarter
	for _, c := range rawCats {
		cs, _ := c.(string)
		q, ok := firstQuarter(cs, 0)
		if !ok || !q.hasFY {
			t.Fatalf("chart category %q must be a fiscal quarter \"Qn FYyy\"", cs)
		}
		cats = append(cats, q)
	}
	for i := 1; i < len(cats); i++ {
		if !cats[i].same(cats[i-1].next()) {
			t.Errorf("chart categories are not consecutive quarters: %s then %s", cats[i-1], cats[i])
		}
	}
	if len(cats) == 0 || !cats[len(cats)-1].same(asOf) {
		t.Errorf("chart must end at the as-of quarter %s, got %v", asOf, cats)
	}

	series, _ := data2["series"].([]any)
	if len(series) != 1 {
		t.Fatalf("expected one revenue series, got %d", len(series))
	}
	s0, _ := series[0].(map[string]any)
	if name, _ := s0["name"].(string); isARRLabel(name) {
		t.Errorf("chart series %q conflates ARR with period revenue", name)
	}
	rawVals, _ := s0["values"].([]any)
	vals := make([]float64, 0, len(rawVals))
	for _, v := range rawVals {
		vals = append(vals, toFloat(t, v))
	}
	if len(vals) != len(cats) || len(vals) < 2 {
		t.Fatalf("values %v do not match categories %v", vals, cats)
	}
	last, prev := vals[len(vals)-1], vals[len(vals)-2]

	// ARR is a distinct measure: never the same figure as a quarterly
	// revenue bar, and (as the example's header comment states) the
	// annualized run-rate of the as-of quarter.
	arr := parseMillions(t, arrValue)
	for i, v := range vals {
		if v == arr {
			t.Errorf("ARR %s equals the %s quarterly revenue bar; the two measures must differ", arrValue, cats[i])
		}
	}
	if math.Abs(arr-4*last) > 0.01*arr {
		t.Errorf("ARR %s is not the annualized run-rate of %s revenue %.1f (4x = %.1f)", arrValue, asOf, last, 4*last)
	}

	// The summary's QoQ claim must match the chart's last two quarters.
	qoq := regexp.MustCompile(`(\d+)% QoQ`)
	found := false
	for _, s := range spec.Slides {
		if s.Kind != "executive_summary" {
			continue
		}
		for _, str := range collectStrings(s.Body) {
			m := qoq.FindStringSubmatch(str)
			if m == nil {
				continue
			}
			found = true
			claimed, _ := strconv.Atoi(m[1])
			if got := int(math.Round((last/prev - 1) * 100)); got != claimed {
				t.Errorf("summary claims %d%% QoQ, chart %s→%s is %d%%", claimed, cats[len(cats)-2], asOf, got)
			}
		}
	}
	if !found {
		t.Error("executive summary no longer states a QoQ growth figure; update this test")
	}
}

type quarter struct {
	q, fy int
	hasFY bool
}

func (a quarter) after(b quarter) bool {
	return a.fy > b.fy || (a.fy == b.fy && a.q > b.q)
}

func (a quarter) same(b quarter) bool { return a.q == b.q && a.fy == b.fy }

func (a quarter) next() quarter {
	if a.q == 4 {
		return quarter{q: 1, fy: a.fy + 1, hasFY: a.hasFY}
	}
	return quarter{q: a.q + 1, fy: a.fy, hasFY: a.hasFY}
}

func (a quarter) String() string { return fmt.Sprintf("Q%d FY%02d", a.q, a.fy) }

var quarterRE = regexp.MustCompile(`\bQ([1-4])(?:\s+FY(\d{2}))?\b`)

// allQuarters returns every "Qn" / "Qn FYyy" token in s; a bare "Qn" is
// read as a quarter of defaultFY.
func allQuarters(s string, defaultFY int) []quarter {
	var out []quarter
	for _, m := range quarterRE.FindAllStringSubmatch(s, -1) {
		q, _ := strconv.Atoi(m[1])
		qt := quarter{q: q, fy: defaultFY}
		if m[2] != "" {
			qt.fy, _ = strconv.Atoi(m[2])
			qt.hasFY = true
		}
		out = append(out, qt)
	}
	return out
}

func firstQuarter(s string, defaultFY int) (quarter, bool) {
	qs := allQuarters(s, defaultFY)
	if len(qs) == 0 {
		return quarter{}, false
	}
	return qs[0], true
}

var arrRE = regexp.MustCompile(`\bARR\b`)

func isARRLabel(s string) bool {
	return arrRE.MatchString(s) || strings.Contains(strings.ToLower(s), "recurring")
}

func collectStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, collectStrings(e)...)
		}
		return out
	case map[string]any:
		var out []string
		for _, e := range x {
			out = append(out, collectStrings(e)...)
		}
		return out
	}
	return nil
}

func toFloat(t *testing.T, v any) float64 {
	t.Helper()
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	case float64:
		return n
	}
	t.Fatalf("chart value %v (%T) is not numeric", v, v)
	return 0
}

// parseMillions reads a "$48M" style KPI value as millions.
func parseMillions(t *testing.T, s string) float64 {
	t.Helper()
	m := regexp.MustCompile(`^\$(\d+(?:\.\d+)?)M$`).FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("ARR value %q is not a \"$NM\" figure", s)
	}
	f, _ := strconv.ParseFloat(m[1], 64)
	return f
}
