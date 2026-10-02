package semantic

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/semantic/slides"
)

// TestSalesPitchExample_CredibleAndActionable pins the authoring conventions of
// the shipped examples/semantic/sales_pitch.yaml (go-slide-creator-3i61q). The
// example once rendered a market chart in $B under the deck's customer
// benchmark source with no illustrative disclosure and no forecast marker on the
// current year, let a 66-character option label drop its decision slide to
// plain bullets, repeated the pilot in the recommendation, and closed on "ready
// to start when you are". It still scored 99, so nothing else caught it. Agents
// copy this file, so it must stay a credible, actionable exemplar.
//
// This is a fixture-content test, not a prose truth detector: it checks the
// structured fields (sources, chart categories and title, option labels, the
// recommendation, the closing actions).
func TestSalesPitchExample_CredibleAndActionable(t *testing.T) {
	data, err := os.ReadFile(salesPitchExamplePath)
	if err != nil {
		t.Fatalf("read %s: %v", salesPitchExamplePath, err)
	}
	spec, diags := Parse(salesPitchExamplePath, data)
	if diags.HasErrors() {
		t.Fatalf("parse: %v", diags)
	}

	// No slide may degrade from its visual pattern.
	for _, d := range Check("sales_pitch.yaml", data, StrictnessWarn) {
		if d.Code == diagnostics.CodeSemanticPatternDegraded {
			t.Errorf("%s at %s: %s", d.Code, d.Path, d.Message)
		}
	}

	// The as-of year comes from meta.date; a chart year at or after it is not
	// an actual and must be marked as a forecast.
	asOfYear := lastYear(spec.Meta.Date)
	if asOfYear == 0 {
		t.Errorf("meta.date %q must name the as-of year so forecasts can be told from actuals", spec.Meta.Date)
	}
	if !mentionsIllustrative(spec.Meta.Source) {
		t.Errorf("meta.source %q must disclose that the example data is illustrative", spec.Meta.Source)
	}

	var sawChart, sawKPI, sawDecision bool
	for i, s := range spec.Slides {
		src := s.String("source")
		switch s.Kind {
		case "chart_insight":
			sawChart = true
			// Market sizing is a model: its own source, disclosed as
			// illustrative, never the customer benchmark.
			if !mentionsIllustrative(src) {
				t.Errorf("slide %d (chart_insight): source %q must say the market figures are illustrative", i, src)
			}
			if strings.Contains(strings.ToLower(src), "customer benchmark") {
				t.Errorf("slide %d (chart_insight): market chart cites the customer benchmark %q", i, src)
			}
			chart, _ := s.Body["chart"].(map[string]any)
			title, _ := chart["title"].(string)
			if !mentionsIllustrative(title) {
				t.Errorf("slide %d: chart title %q must say the figures are illustrative", i, title)
			}
			chartData, _ := chart["data"].(map[string]any)
			cats, _ := chartData["categories"].([]any)
			forecast := false
			for _, c := range cats {
				cs, _ := c.(string)
				year := lastYear(cs)
				if year == 0 {
					t.Fatalf("slide %d: chart category %q is not a year", i, cs)
				}
				marked := strings.HasSuffix(strings.TrimSpace(cs), "F") || strings.Contains(strings.ToLower(cs), "forecast")
				if asOfYear != 0 && year >= asOfYear && !marked {
					t.Errorf("slide %d: chart category %q is not an actual as of %s; mark it as a forecast (\"%dF\")", i, cs, spec.Meta.Date, year)
				}
				forecast = forecast || marked
			}
			if forecast {
				for field, v := range map[string]string{"source": src, "chart title": title} {
					if !strings.Contains(strings.ToLower(v), "forecast") {
						t.Errorf("slide %d: the chart shows a forecast but its %s %q does not say so", i, field, v)
					}
				}
			}
		case "kpi_snapshot":
			sawKPI = true
			// Customer KPIs cite the customer benchmark, disclosed as
			// illustrative.
			if !mentionsIllustrative(src) || !strings.Contains(strings.ToLower(src), "customer benchmark") {
				t.Errorf("slide %d (kpi_snapshot): source %q must cite the customer benchmark and disclose it as illustrative", i, src)
			}
		case "decision":
			sawDecision = true
			if slides.DecisionPattern(s.Body) == "" {
				t.Errorf("slide %d (decision): options do not fit a visual decision pattern: %s", i, slides.DecisionOverBudget(s.Body))
			}
			rec := normalizeText(s.String("recommendation"))
			if rec == "" {
				t.Errorf("slide %d (decision): no recommendation", i)
			}
			for j, o := range slides.DecisionOptions(s.Body) {
				// The visual needs both halves: a short label and a detail.
				if o.Detail == "" {
					t.Errorf("slide %d option %d %q has no detail", i, j, o.Label)
				}
				// The band says why the choice wins; restating the
				// option is the repetition this example once had.
				if l := normalizeText(o.Label); l != "" && strings.Contains(rec, l) {
					t.Errorf("slide %d: recommendation repeats option %d label %q", i, j, o.Label)
				}
			}
		}
	}
	if !sawChart || !sawKPI || !sawDecision {
		t.Fatalf("sales_pitch example lost a slide this test pins: chart=%v kpi=%v decision=%v", sawChart, sawKPI, sawDecision)
	}

	// The deck closes on a concrete next step for the pilot: actions with an
	// accountable role and timing, rendered as the next-steps visual.
	last := spec.Slides[len(spec.Slides)-1]
	if last.Kind != "next_steps" {
		t.Fatalf("last slide is %q; the example must close on next_steps (action, owner, timing), not a thank-you", last.Kind)
	}
	if slides.NextStepsPattern(last.Body) == "" {
		t.Errorf("closing next_steps degrades to bullets: %s", slides.NextStepsDegradeReason(last.Body))
	}
	actions, _ := last.Body["actions"].([]any)
	pilot := false
	for j, a := range actions {
		m, _ := a.(map[string]any)
		action, _ := m["action"].(string)
		owner, _ := m["owner"].(string)
		date, _ := m["date"].(string)
		if strings.TrimSpace(owner) == "" || strings.TrimSpace(date) == "" {
			t.Errorf("closing action %d %q needs an accountable owner and a timing", j, action)
		}
		pilot = pilot || strings.Contains(strings.ToLower(action), "pilot")
	}
	for _, d := range collectStrings(last.Body["decisions"]) {
		pilot = pilot || strings.Contains(strings.ToLower(d), "pilot")
	}
	if !pilot {
		t.Error("closing next steps never mention the pilot they are meant to start")
	}
}

var yearRE = regexp.MustCompile(`\b(20\d{2})`)

// lastYear returns the last four-digit year in s, or 0.
func lastYear(s string) int {
	m := yearRE.FindAllStringSubmatch(s, -1)
	if len(m) == 0 {
		return 0
	}
	y, _ := strconv.Atoi(m[len(m)-1][1])
	return y
}

func mentionsIllustrative(s string) bool {
	return strings.Contains(strings.ToLower(s), "illustrative")
}

// normalizeText lowercases s and collapses everything but letters and digits
// to single spaces, so containment ignores punctuation and case.
func normalizeText(s string) string {
	f := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	return strings.Join(f, " ")
}
