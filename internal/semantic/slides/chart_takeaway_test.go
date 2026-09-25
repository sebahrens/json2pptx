package slides

import (
	"encoding/json"
	"testing"
)

// A lone insight that IS the takeaway printed the same sentence twice on one
// slide: once as the only Key Insight bullet and once verbatim in the takeaway
// bar (go-slide-creator-pyxn).
func TestDropDuplicateTakeaway(t *testing.T) {
	const line = "R&D and S&M are 69% of spend"
	cases := []struct {
		name     string
		takeaway string
		insights []string
		want     string
	}{
		{"lone insight is the takeaway", line, []string{line}, ""},
		{"ignores case and padding", line, []string{"  r&d AND s&m are 69% OF spend "}, ""},
		{"a different takeaway is kept", "Shift spend to R&D", []string{line}, "Shift spend to R&D"},
		{"a takeaway over several insights is kept", line, []string{line, "G&A is flat"}, line},
		{"no takeaway stays empty", "", []string{line}, ""},
		{"no insights leaves it alone", line, nil, line},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dropDuplicateTakeaway(c.takeaway, c.insights); got != c.want {
				t.Errorf("dropDuplicateTakeaway(%q, %v) = %q, want %q", c.takeaway, c.insights, got, c.want)
			}
		})
	}
}

// End to end through the compiler: the band is dropped for the duplicate and
// kept when it says something the bullets do not.
func TestCompileChartInsightDropsTheDuplicateBand(t *testing.T) {
	const line = "R&D and S&M are 69% of spend"
	chart := map[string]any{
		"type": "pie",
		"data": map[string]any{"categories": []any{"R&D", "S&M"}, "values": []any{69.0, 31.0}},
	}

	dup := Input{
		Takeaway: line,
		Body:     map[string]any{"title": "Where the money goes", "chart": chart, "insights": []any{line}},
	}
	slide, _, err := CompileChartInsight(dup)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Takeaway != "" {
		t.Errorf("takeaway = %q; the same sentence is already the only insight", slide.Takeaway)
	}

	distinct := Input{
		Takeaway: "Shift spend to R&D",
		Body:     map[string]any{"title": "Where the money goes", "chart": chart, "insights": []any{line}},
	}
	slide, _, err = CompileChartInsight(distinct)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Takeaway != "Shift spend to R&D" {
		t.Errorf("takeaway = %q; a takeaway that is not the insight must survive", slide.Takeaway)
	}
}

func TestCompileChartInsightUsesDistinctInsightAsCallout(t *testing.T) {
	chart := map[string]any{"type": "bar", "data": map[string]any{
		"categories": []any{"Q1"}, "series": []any{map[string]any{"name": "Revenue", "values": []any{42.0}}},
	}}
	for _, tc := range []struct {
		name, insight, takeaway, wantCallout, wantBand string
	}{
		{"distinct implication", "Expand the sales team", "", "Expand the sales team", ""},
		{"duplicate bullet", "Revenue rose 18%", "", "", ""},
		{"explicit separate takeaway", "Expand the sales team", "Cap spend", "Expand the sales team", "Cap spend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"chart": chart, "insights": []any{"Revenue rose 18%"}, "insight": tc.insight}
			if tc.takeaway != "" {
				body["takeaway"] = tc.takeaway
			}
			in := Input{Body: body, Takeaway: tc.insight}
			if tc.takeaway != "" {
				in.Takeaway = tc.takeaway
			}
			slide, links, err := CompileChartInsight(in)
			if err != nil {
				t.Fatal(err)
			}
			if slide.Takeaway != tc.wantBand {
				t.Errorf("takeaway = %q, want %q", slide.Takeaway, tc.wantBand)
			}
			var vals chartInsightsValues
			if err := json.Unmarshal(slide.Pattern.Values, &vals); err != nil {
				t.Fatal(err)
			}
			if vals.SoWhat != tc.wantCallout {
				t.Errorf("so_what = %q, want %q", vals.SoWhat, tc.wantCallout)
			}
			if len(vals.Insights) != 1 || vals.Insights[0] != "Revenue rose 18%" {
				t.Errorf("insights = %v, want the one evidence bullet", vals.Insights)
			}
			var ovr struct {
				TitleSize  float64 `json:"title_size"`
				BulletSize float64 `json:"bullet_size"`
			}
			if err := json.Unmarshal(slide.Pattern.Overrides, &ovr); err != nil {
				t.Fatal(err)
			}
			if ovr.TitleSize != 14 || ovr.BulletSize != 14 {
				t.Errorf("compiled chart insight sizes = %.0f/%.0fpt, want 14/14pt", ovr.TitleSize, ovr.BulletSize)
			}
			linkedCallout := false
			for _, link := range links {
				if link.RawPath == "slides[0].pattern.values.so_what" && link.SemanticPath == "slides[0].insight" {
					linkedCallout = true
				}
			}
			if linkedCallout != (tc.wantCallout != "") {
				t.Errorf("callout source link present = %t, want %t", linkedCallout, tc.wantCallout != "")
			}
		})
	}
}

func TestCompileChartInsightDoesNotRepeatScalarAliasInMultiBulletFooter(t *testing.T) {
	line := "Revenue rose 18%"
	in := Input{
		Body: map[string]any{
			"insights": []any{line, "Margin rose 3 points"},
			"insight":  line,
		},
		Takeaway: line,
	}
	slide, _, err := CompileChartInsight(in)
	if err != nil {
		t.Fatal(err)
	}
	if slide.Takeaway != "" {
		t.Errorf("repeated scalar alias in footer: %q", slide.Takeaway)
	}
	var vals chartInsightsValues
	if err := json.Unmarshal(slide.Pattern.Values, &vals); err != nil {
		t.Fatal(err)
	}
	if vals.SoWhat != "" || len(vals.Insights) != 2 {
		t.Errorf("expected two evidence bullets and no duplicate callout, got %+v", vals)
	}
}
