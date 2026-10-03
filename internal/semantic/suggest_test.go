package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

func TestSuggestKind(t *testing.T) {
	cases := []struct {
		kind, want, hostedType, hostedAs string
	}{
		{"funnel", "chart_insight", "funnel", "chart"},
		{"funnel_chart", "chart_insight", "funnel", "chart"},
		{"pie", "chart_insight", "pie", "chart"},
		{"waterfall", "bridge", "waterfall", "chart"},
		{"swot", "framework", "swot", "diagram"},
		{"org_chart", "org", "org_chart", "diagram"},
		{"venn", "raw_json2pptx", "venn", "diagram"},
		{"metric_hero", "stat", "", ""},
		{"kpi", "kpi_snapshot", "", ""},
		{"proces", "process", "", ""},
		{"exec-summary", "executive_summary", "", ""},
		{"snapshot", "kpi_snapshot", "", ""},
		{"Timeline ", "timeline", "", ""},
		{"zzz", "", "", ""},
		{"", "", "", ""},
	}
	for _, c := range cases {
		got := SuggestKind(c.kind)
		if string(got.DidYouMean) != c.want || got.HostedType != c.hostedType || got.HostedAs != c.hostedAs {
			t.Errorf("SuggestKind(%q) = %+v, want kind %q hosted %q as %q", c.kind, got, c.want, c.hostedType, c.hostedAs)
		}
		if (got.Hint == "") != (c.want == "") {
			t.Errorf("SuggestKind(%q): hint %q for kind %q", c.kind, got.Hint, c.want)
		}
		if c.want != "" && !SlideKind(c.want).Valid() {
			t.Errorf("SuggestKind(%q) suggests the unregistered kind %q", c.kind, c.want)
		}
	}
}

// The parser and the validator word an unknown kind the same way, so Check
// reports it once, and the message leads with the kind to use.
func TestUnknownKindIsReportedOnceWithItsSuggestion(t *testing.T) {
	ds := Check("deck.json", []byte(`{"meta":{"title":"T"},"slides":[{"kind":"funnel","title":"Savings funnel"}]}`), StrictnessWarn)
	n := 0
	for _, d := range ds {
		if d.Code != diagnostics.CodeSemanticUnknownKind {
			continue
		}
		n++
		if d.Path != "slides[0].kind" || !strings.Contains(d.Message, `use kind "chart_insight" with chart {type: "funnel"`) {
			t.Errorf("finding = %+v", d)
		}
		if strings.Contains(d.Message, "expected one of") {
			t.Errorf("a suggestion is given, yet the message still lists every kind: %s", d.Message)
		}
	}
	if n != 1 {
		t.Errorf("unknown kind reported %d times: %+v", n, ds)
	}
}

func TestSuggestKey(t *testing.T) {
	cases := []struct {
		key   string
		known []string
		want  string
	}{
		{"takeway", []string{"kpis", "takeaway", "title"}, "takeaway"},
		{"bulets", []string{"cons", "header", "items", "label", "name", "pros", "title"}, "items"},
		{"bullets", []string{"header", "items"}, "items"},
		{"stages", []string{"steps", "title", "takeaway"}, "steps"},
		{"headline", []string{"title", "kpis"}, "title"},
		{"metrics", []string{"kpis", "title"}, "kpis"},
		{"value", []string{"label", "description", "type"}, ""},
		{"zzqq", []string{"header", "items"}, ""},
	}
	for _, c := range cases {
		if got := suggestKey(c.key, c.known); got != c.want {
			t.Errorf("suggestKey(%q, %v) = %q, want %q", c.key, c.known, got, c.want)
		}
	}
}

// Every unknown key with a close match carries did_you_mean, whichever pass
// found it: a slide field, a list entry's key, a meta field or a top-level
// field.
func TestEveryUnknownKeyWithACloseMatchHasDidYouMean(t *testing.T) {
	spec := `{"meta":{"title":"T","subtitel":"S"},"sldes":[],"slides":[
	 {"kind":"kpi_snapshot","title":"Two numbers carry the quarter","kpis":[{"value":"1","lable":"A"},{"value":"2","label":"B"}],"takeway":"x"},
	 {"kind":"process","title":"Three steps deliver the plan","stages":[{"label":"a"},{"label":"b"},{"label":"c"}]}]}`
	want := map[string]string{
		"meta.subtitel":           "subtitle",
		"sldes":                   "slides",
		"slides[0].kpis[0].lable": "label",
		"slides[0].takeway":       "takeaway",
		"slides[1].stages":        "steps",
	}
	for _, d := range Check("deck.json", []byte(spec), StrictnessWarn) {
		if d.Code != diagnostics.CodeSemanticUnknownField {
			continue
		}
		expect, ok := want[d.Path]
		if !ok {
			t.Errorf("unexpected unknown-field finding at %s", d.Path)
			continue
		}
		delete(want, d.Path)
		if d.Fix == nil || d.Fix.Params["did_you_mean"] != expect || d.Fix.Kind != "rename_field" {
			t.Errorf("%s: fix = %+v, want rename_field with did_you_mean %q", d.Path, d.Fix, expect)
		}
	}
	for path := range want {
		t.Errorf("no unknown-field finding at %s", path)
	}
}

// Product placeholders are reported whatever the strictness, with the marker
// that identifies them; authored filler stays an advisory.
func TestProductPlaceholderFinding(t *testing.T) {
	spec := `{"meta":{"title":"Churn review","date":"__FILL__"},"slides":[
	 {"kind":"title","title":"Replace with the action title this bar chart supports","subtitle":"TBD"}]}`
	for _, strict := range []Strictness{StrictnessOff, StrictnessWarn, StrictnessStrict} {
		found := map[string]diagnostics.Diagnostic{}
		for _, d := range Check("deck.json", []byte(spec), strict) {
			if d.Code == diagnostics.CodeSemanticWeakContent {
				found[d.Path] = d
			}
		}
		for path, marker := range map[string]string{"meta.date": "fill_token", "slides[0].title": "recipe_action_title"} {
			d, ok := found[path]
			if !ok || d.Details[PlaceholderDetail] != marker {
				t.Errorf("strict=%s: %s: finding %+v, want placeholder %s", strict, path, d, marker)
			}
		}
		_, filler := found["slides[0].subtitle"]
		if filler != (strict != StrictnessOff) {
			t.Errorf("strict=%s: authored filler reported = %v", strict, filler)
		}
		if d, ok := found["slides[0].subtitle"]; ok && d.Details[PlaceholderDetail] != nil {
			t.Errorf("authored filler is marked as a product placeholder: %+v", d)
		}
	}
}

// A count finding names the range it wants, and each over-budget item is its
// own finding with what was measured and what fits.
func TestDegradeFindingsCarryTheirLimits(t *testing.T) {
	long := strings.Repeat("word ", 16) // 80 characters
	spec := `{"meta":{"title":"T"},"slides":[
	 {"kind":"decision","title":"We ask for a decision on the budget today","recommendation":"Approve B","options":[
	  {"label":"` + long + `A","detail":"x"},{"label":"Option B","detail":"y","recommended":true},{"label":"` + long + `C","detail":"z"}]},
	 {"kind":"kpi_snapshot","title":"Seven numbers carry the quarter for the board","takeaway":"t","kpis":[
	  {"value":"1","label":"a"},{"value":"2","label":"b"},{"value":"3","label":"c"},{"value":"4","label":"d"},{"value":"5","label":"e"},{"value":"6","label":"f"},{"value":"7","label":"g"}]}]}`
	byPath := map[string]diagnostics.Diagnostic{}
	for _, d := range Check("deck.json", []byte(spec), StrictnessWarn) {
		if d.Code == diagnostics.CodeSemanticPatternDegraded {
			byPath[d.Path] = d
		}
	}
	for _, path := range []string{"slides[0].options[0].label", "slides[0].options[2].label"} {
		d, ok := byPath[path]
		if !ok {
			t.Errorf("no finding at %s: %+v", path, byPath)
			continue
		}
		if d.Details["measured"] != 81 || d.Details["allowed"] != 60 || d.Fix == nil || d.Fix.Params["max_chars"] != 60 {
			t.Errorf("%s: details %+v fix %+v", path, d.Details, d.Fix)
		}
	}
	if _, listLevel := byPath["slides[0].options"]; listLevel {
		t.Error("the list-level finding is still emitted beside the per-item ones")
	}
	count, ok := byPath["slides[1].kpis"]
	if !ok || count.Fix == nil || count.Fix.Params["max_items"] != 6 || count.Fix.Params["min_items"] != 2 {
		t.Errorf("count finding = %+v", count)
	}
}
