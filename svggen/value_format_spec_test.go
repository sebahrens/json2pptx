package svggen

import (
	"regexp"
	"strings"
	"testing"
)

// intPtrVF is a local helper so the table below can set Decimals inline.
func intPtrVF(v int) *int { return &v }

func boolPtrVF(v bool) *bool { return &v }

// TestValueFormatterStyles pins the agent-facing value_format vocabulary
// (go-slide-creator-e2ck9).
func TestValueFormatterStyles(t *testing.T) {
	values := []float64{1240000, 865000, 412000}
	cases := []struct {
		name string
		spec *ValueFormatSpec
		in   float64
		want string
	}{
		{"nil spec on large values compacts", nil, 1240000, "1.2M"},
		{"compact", &ValueFormatSpec{Style: "compact"}, 1240000, "1.2M"},
		{"compact with a prefix", &ValueFormatSpec{Style: "compact", Prefix: "€"}, 1240000, "€1.2M"},
		{"compact with fixed decimals", &ValueFormatSpec{Style: "compact", Decimals: intPtrVF(2)}, 1240000, "1.24M"},
		{"compact with zero decimals", &ValueFormatSpec{Style: "compact", Decimals: intPtrVF(0)}, 1240000, "1M"},
		{"plain groups digits", &ValueFormatSpec{Style: "plain"}, 1240000, "1,240,000"},
		{"plain without grouping", &ValueFormatSpec{Style: "plain", ThousandsSep: boolPtrVF(false)}, 1240000, "1240000"},
		{"currency", &ValueFormatSpec{Style: "currency", Prefix: "$"}, 1240000, "$1,240,000"},
		{"currency without prefix is visibly currency", &ValueFormatSpec{Style: "currency"}, 1240000, "¤1,240,000"},
		{"suffix", &ValueFormatSpec{Style: "compact", Suffix: " ARR"}, 1240000, "1.2M ARR"},
		{"an unknown style is plain", &ValueFormatSpec{Style: "klingon"}, 1240000, "1,240,000"},
		{"style casing does not matter", &ValueFormatSpec{Style: "COMPACT"}, 1240000, "1.2M"},
		{"negatives keep their sign", &ValueFormatSpec{Style: "compact"}, -1240000, "-1.2M"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewValueFormatter(tc.spec, values, defaultValueFormat, true)
			if got := f.Format(tc.in); got != tc.want {
				t.Errorf("Format(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValueFormatterPercentScale(t *testing.T) {
	tests := []struct {
		name   string
		spec   *ValueFormatSpec
		values []float64
		input  float64
		want   string
	}{
		{name: "fractional values scale before auto precision", spec: &ValueFormatSpec{Style: "percent"}, values: []float64{0.412, 0.408, 0.401}, input: 0.412, want: "41.2%"},
		{name: "already-scaled values are preserved", spec: &ValueFormatSpec{Style: "percent", Decimals: intPtrVF(1)}, values: []float64{42.1, 40.8, 40.1}, input: 42.1, want: "42.1%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewValueFormatter(tt.spec, tt.values, defaultValueFormat, true)
			if got := f.Format(tt.input); got != tt.want {
				t.Fatalf("Format(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValueFormatFindings(t *testing.T) {
	tests := []struct {
		name     string
		spec     *ValueFormatSpec
		values   []any
		wantCode string
	}{
		{name: "fractional percent is unambiguous", spec: &ValueFormatSpec{Style: "percent"}, values: []any{0.412, 0.408, 0.401}},
		{name: "already-scaled percent warns", spec: &ValueFormatSpec{Style: "percent"}, values: []any{41.2, 40.8, 40.1}, wantCode: FindingPercentScaleAmbiguous},
		{name: "currency symbol is explicit", spec: &ValueFormatSpec{Style: "currency", Prefix: "€"}, values: []any{12.0}},
		{name: "currency symbol defaults visibly", spec: &ValueFormatSpec{Style: "currency"}, values: []any{12.0}, wantCode: FindingCurrencyPrefixDefaulted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{Style: StyleSpec{ValueFormat: tt.spec}, Data: map[string]any{"series": []any{map[string]any{"values": tt.values}}}}
			findings := valueFormatFindings(req)
			if tt.wantCode == "" {
				if len(findings) != 0 {
					t.Fatalf("findings = %+v, want none", findings)
				}
				return
			}
			if len(findings) != 1 || findings[0].Code != tt.wantCode {
				t.Fatalf("findings = %+v, want one %q finding", findings, tt.wantCode)
			}
		})
	}
}

func TestValueFormatFindingsProgrammaticTypedData(t *testing.T) {
	req := &RequestEnvelope{
		Style: StyleSpec{ValueFormat: &ValueFormatSpec{Style: "percent"}},
		Data: map[string]any{
			"series": []map[string]any{{"values": []float64{41.2, 40.8, 40.1}}},
		},
	}
	findings := valueFormatFindings(req)
	if len(findings) != 1 || findings[0].Code != FindingPercentScaleAmbiguous {
		t.Fatalf("findings = %+v, want one %q finding", findings, FindingPercentScaleAmbiguous)
	}
}

// TestValueFormatterDefaults pins what the renderer picks when the caller says
// nothing, which is the behaviour every existing deck depends on.
func TestValueFormatterDefaults(t *testing.T) {
	cases := []struct {
		name         string
		values       []float64
		legacy       string
		allowCompact bool
		in           float64
		want         string
	}{
		{"small integers are grouped", []float64{1240, 865, 413}, defaultValueFormat, true, 1240, "1,240"},
		{"fractions keep enough decimals to differ", []float64{4.6, 4.9, 5.2}, defaultValueFormat, true, 4.6, "4.6"},
		{"large values compact on a value axis", []float64{1240000, 865000}, defaultValueFormat, true, 1240000, "1.2M"},
		{"large values stay grouped with no axis to match", []float64{12400, 8650}, defaultValueFormat, false, 12400, "12,400"},
		{"an explicit printf verb is honoured", []float64{1240, 865}, "€%.1fM", true, 1240, "€1,240.0M"},
		{"an explicit verb is never rewritten as compact", []float64{1240000}, "$%.0f", true, 1240000, "$1,240,000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewValueFormatter(nil, tc.values, tc.legacy, tc.allowCompact)
			if got := f.Format(tc.in); got != tc.want {
				t.Errorf("Format(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestValueFormatterNilIsLegacy pins that a renderer that has not been handed a
// formatter keeps its old output.
func TestValueFormatterNilIsLegacy(t *testing.T) {
	var f *ValueFormatter
	if got, want := f.FormatOr(12400, "%.0f"), "12,400"; got != want {
		t.Errorf("nil formatter = %q, want the legacy %q", got, want)
	}
	if got, want := f.FormatOr(12400.5, "€%.1f"), "€12,400.5"; got != want {
		t.Errorf("nil formatter with a caller format = %q, want %q", got, want)
	}
}

// TestAxisAndLabelsAgree is the bead's headline: within one chart the axis ticks
// and the data labels must be formatted the same way. It compares what
// LinearAxisLabels renders against what the label sites render, for the exact
// series the bead reported.
func TestAxisAndLabelsAgree(t *testing.T) {
	cases := []struct {
		name      string
		values    []float64
		domainMax float64
		spec      *ValueFormatSpec
		wantTick  string
		wantLabel string
	}{
		{
			name: "the reported bar series", values: []float64{1240, 865, 413}, domainMax: 1400,
			wantTick: "1,200", wantLabel: "1,240",
		},
		{
			name: "millions", values: []float64{1240000, 865000, 412000}, domainMax: 1400000,
			wantTick: "1.2M", wantLabel: "1.2M",
		},
		{
			name: "an explicit currency format reaches both", values: []float64{1240000, 865000}, domainMax: 1400000,
			spec: &ValueFormatSpec{Style: "compact", Prefix: "€"}, wantTick: "€1.2M", wantLabel: "€1.2M",
		},
	}
	grouped := regexp.MustCompile(`^[^0-9]*[0-9]{4,}`)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewValueFormatter(tc.spec, tc.values, defaultValueFormat, true)
			scale := NewLinearScale(0, tc.domainMax)
			ticks := LinearAxisLabels(scale, 5, "", f)
			if len(ticks) == 0 {
				t.Fatal("no tick labels")
			}
			joined := strings.Join(ticks, " ")
			if !strings.Contains(joined, tc.wantTick) {
				t.Errorf("axis labels %v do not contain %q", ticks, tc.wantTick)
			}
			if got := f.Format(tc.values[0]); got != tc.wantLabel {
				t.Errorf("data label = %q, want %q", got, tc.wantLabel)
			}
			// Neither side may print a bare four-digit run while the other groups.
			for _, tick := range ticks {
				if grouped.MatchString(tick) {
					t.Errorf("tick %q prints ungrouped digits while the labels group them", tick)
				}
			}
		})
	}
}

// TestAxisKeepsItsTickPrecision pins the one thing the axis does NOT take from
// the data: its decimal places. A whole-numbered domain must print "1", not
// "1.0", just because one data point is fractional — and a domain that ticks in
// tenths must still print them.
func TestAxisKeepsItsTickPrecision(t *testing.T) {
	f := NewValueFormatter(nil, []float64{0.5, 2.0, 3.5}, defaultValueFormat, true)

	whole := LinearAxisLabels(NewLinearScale(0, 4), 5, "", f)
	for _, l := range whole {
		if strings.Contains(l, ".") {
			t.Errorf("whole-numbered ticks should not carry decimals, got %v", whole)
			break
		}
	}

	tenths := LinearAxisLabels(NewLinearScale(0, 1), 5, "", f)
	var sawDecimal bool
	for _, l := range tenths {
		if strings.Contains(l, ".") {
			sawDecimal = true
		}
	}
	if !sawDecimal {
		t.Errorf("ticks every 0.2 must show a decimal, got %v", tenths)
	}

	// An explicit decimals request is not second-guessed by the axis.
	fixed := NewValueFormatter(&ValueFormatSpec{Decimals: intPtrVF(0)}, []float64{0.5}, defaultValueFormat, true)
	for _, l := range LinearAxisLabels(NewLinearScale(0, 1), 5, "", fixed) {
		if strings.Contains(l, ".") {
			t.Errorf("decimals:0 must be honoured on the axis, got %q", l)
		}
	}
}

// TestLinearAxisLabelsWithoutAFormatter pins the historical rule, which still
// applies to axes no chart formatter reaches (an x value axis, a log axis's
// neighbours) and to a caller-supplied axis format.
func TestLinearAxisLabelsWithoutAFormatter(t *testing.T) {
	plain := LinearAxisLabels(NewLinearScale(0, 6000), 5, "", nil)
	if got := strings.Join(plain, " "); !strings.Contains(got, "1000") || strings.Contains(got, "1,000") {
		t.Errorf("with no formatter the axis keeps its ungrouped labels, got %v", plain)
	}
	compact := LinearAxisLabels(NewLinearScale(0, 1200000), 5, "", nil)
	if got := strings.Join(compact, " "); !strings.Contains(got, "M") {
		t.Errorf("with no formatter a large domain still compacts, got %v", compact)
	}
	explicit := LinearAxisLabels(NewLinearScale(0, 100), 5, "%.2f", NewValueFormatter(&ValueFormatSpec{Style: "compact"}, nil, defaultValueFormat, true))
	if explicit[0] != "0.00" {
		t.Errorf("an explicit axis format wins over the chart formatter, got %v", explicit)
	}
}

// TestRenderedChartUsesOneNumberFormat is the go-slide-creator-e2ck9 acceptance
// test, run through the real render path: a bar chart whose value_format asks for
// compact euros must print "€1.2M" on its axis AND on its bars. The bead's
// measured defect was the opposite — the axis compacted while the bars grouped
// digits, and no argument reached both.
func TestRenderedChartUsesOneNumberFormat(t *testing.T) {
	barReq := func(style StyleSpec) *RequestEnvelope {
		return &RequestEnvelope{
			Type:  "bar_chart",
			Title: "ARR by segment",
			Data: map[string]any{
				"categories": []any{"Enterprise", "Mid-market", "SMB"},
				"series": []any{
					map[string]any{"name": "ARR FY2025", "values": []any{1240000.0, 865000.0, 412000.0}},
				},
			},
			Output: OutputSpec{Width: 800, Height: 600},
			Style:  style,
		}
	}
	extract := func(t *testing.T, req *RequestEnvelope) []string {
		t.Helper()
		doc, err := Render(req)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		var out []string
		for _, m := range regexp.MustCompile(`>([^<>]+)</tspan>`).FindAllStringSubmatch(string(doc.Content), -1) {
			out = append(out, m[1])
		}
		return out
	}

	// With value_format: the axis ticks and the bar labels both carry it.
	euros := extract(t, barReq(StyleSpec{
		ShowValues:  true,
		ValueFormat: &ValueFormatSpec{Style: "compact", Prefix: "€"},
	}))
	joined := strings.Join(euros, " ")
	for _, want := range []string{"€1.2M", "€800K", "€865K"} {
		if !strings.Contains(joined, want) {
			t.Errorf("rendered chart is missing %q; text was %v", want, euros)
		}
	}
	for _, s := range euros {
		if regexp.MustCompile(`^[0-9]`).MatchString(s) {
			t.Errorf("value text %q lost the € prefix — one of the two sides is not using value_format", s)
		}
	}

	// Without it, both sides still agree: compact on the axis means compact on
	// the bars, which is the defect the bead reported.
	plain := extract(t, barReq(StyleSpec{ShowValues: true}))
	joinedPlain := strings.Join(plain, " ")
	if !strings.Contains(joinedPlain, "1.2M") {
		t.Errorf("default labels should read like the axis; text was %v", plain)
	}
	if strings.Contains(joinedPlain, "1,240,000") {
		t.Errorf("default labels still group digits while the axis compacts; text was %v", plain)
	}
}
