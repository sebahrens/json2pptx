package svggen

import (
	"strings"
	"testing"
)

// go-slide-creator-b7qqg.22: an explicit input_scale fixes the units of
// percent data on the ticks, the data labels and any in-mark label alike.
func TestValueFormatter_InputScale(t *testing.T) {
	cases := []struct {
		name   string
		scale  string
		values []float64
		in     float64
		want   string
	}{
		{"percentage points keep low values", InputScalePercentagePoints, []float64{0.25, 0.5}, 0.25, "0.25%"},
		{"fraction scales above one", InputScaleFraction, []float64{1.2, 0.8}, 1.2, "120%"},
		{"auto keeps fractions", InputScaleAuto, []float64{0.25, 0.5}, 0.25, "25%"},
		{"auto keeps percentage points above one", "", []float64{41.2, 12}, 41.2, "41.2%"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewValueFormatter(&ValueFormatSpec{Style: "percent", InputScale: tc.scale}, tc.values, defaultValueFormat, true)
			if got := f.Format(tc.in); got != tc.want {
				t.Errorf("Format(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValueFormat_InputScaleRendersLabelsAndTicks(t *testing.T) {
	render := func(scale string, values []any) (string, []Finding) {
		out, err := RenderMultiFormatWithFindings(&RequestEnvelope{
			Type: "bar_chart",
			Data: map[string]any{"categories": []any{"Savings", "Checking"}, "series": []any{
				map[string]any{"name": "Deposit rate", "values": values},
			}},
			Style: StyleSpec{ShowValues: true, ValueFormat: &ValueFormatSpec{Style: "percent", InputScale: scale}},
		}, "svg")
		if err != nil {
			t.Fatalf("render %s: %v", scale, err)
		}
		return string(out.SVG.Content), out.Findings
	}

	svg, findings := render(InputScalePercentagePoints, []any{0.25, 0.5})
	texts := strings.Join(svgTexts(svg), " | ")
	if !strings.Contains(texts, "0.25%") || !(strings.Contains(texts, "0.5%") || strings.Contains(texts, "0.50%")) {
		t.Errorf("percentage_points labels missing 0.25%% / 0.5%%: %s", texts)
	}
	for _, s := range svgTexts(svg) {
		if s == "25%" || s == "50%" {
			t.Errorf("percentage_points data was multiplied by 100: %s", texts)
		}
	}
	if findFindingByCode(findings, FindingPercentScaleAmbiguous) != nil {
		t.Errorf("explicit input_scale still reported ambiguity: %+v", findings)
	}

	svg, _ = render(InputScaleFraction, []any{1.2, 0.8})
	texts = strings.Join(svgTexts(svg), " | ")
	if !strings.Contains(texts, "120%") || !strings.Contains(texts, "80%") {
		t.Errorf("fraction labels missing 120%% / 80%%: %s", texts)
	}

	_, findings = render("", []any{0.25, 0.5})
	f := findFindingByCode(findings, FindingPercentScaleAmbiguous)
	if f == nil || f.Severity != "info" || f.Fix == nil || f.Fix.Params["field"] != "style.value_format.input_scale" {
		t.Errorf("auto low-range percent should report an info ambiguity pointing at input_scale, got %+v", findings)
	}
}

func TestValueFormat_InputScaleValidation(t *testing.T) {
	cases := []struct {
		name string
		spec ValueFormatSpec
	}{
		{"unknown scale", ValueFormatSpec{Style: "percent", InputScale: "basis_points"}},
		{"scale on a non-percent style", ValueFormatSpec{Style: "plain", InputScale: "fraction"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.spec
			_, err := RenderMultiFormatWithFindings(&RequestEnvelope{
				Type:  "bar_chart",
				Data:  map[string]any{"categories": []any{"A"}, "series": []any{map[string]any{"name": "S", "values": []any{0.5}}}},
				Style: StyleSpec{ValueFormat: &spec},
			}, "svg")
			if err == nil || !strings.Contains(err.Error(), "input_scale") {
				t.Fatalf("err = %v, want a validation error naming input_scale", err)
			}
		})
	}
}
