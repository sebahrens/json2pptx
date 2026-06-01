package slides

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCompileTitle(t *testing.T) {
	in := Input{
		Title: "Big Idea",
		Body: map[string]any{
			"eyebrow":  "STRATEGY",
			"subtitle": "A subtitle",
		},
	}
	slide, links, err := CompileTitle(in)
	if err != nil {
		t.Fatalf("CompileTitle: %v", err)
	}
	if slide.SlideType != "title" {
		t.Errorf("SlideType = %q, want title", slide.SlideType)
	}
	if slide.Eyebrow != "STRATEGY" {
		t.Errorf("Eyebrow = %q, want STRATEGY", slide.Eyebrow)
	}
	// title placeholder + subtitle placeholder.
	if len(slide.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(slide.Content))
	}
	if slide.Content[0].PlaceholderID != "title" || slide.Content[0].TextValue == nil || *slide.Content[0].TextValue != "Big Idea" {
		t.Errorf("title content wrong: %+v", slide.Content[0])
	}
	if slide.Content[1].PlaceholderID != "subtitle" {
		t.Errorf("Content[1] placeholder = %q, want subtitle", slide.Content[1].PlaceholderID)
	}
	if len(links) == 0 {
		t.Error("expected source links, got none")
	}
}

func TestCompileTitle_TitleOnly(t *testing.T) {
	slide, _, err := CompileTitle(Input{Title: "Only"})
	if err != nil {
		t.Fatalf("CompileTitle: %v", err)
	}
	if len(slide.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(slide.Content))
	}
}

func TestCompileSection(t *testing.T) {
	slide, _, err := CompileSection(Input{Title: "Part One", Body: map[string]any{"subtitle": "sub"}})
	if err != nil {
		t.Fatalf("CompileSection: %v", err)
	}
	if slide.SlideType != "section" {
		t.Errorf("SlideType = %q, want section", slide.SlideType)
	}
	if len(slide.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(slide.Content))
	}
	if slide.Content[0].PlaceholderID != "title" {
		t.Errorf("Content[0] placeholder = %q, want title", slide.Content[0].PlaceholderID)
	}
}

func TestCompileClosing_BulletsBranch(t *testing.T) {
	in := Input{
		Title: "Wrap Up",
		Body: map[string]any{
			"bullets": []any{"first", "second"},
		},
	}
	slide, _, err := CompileClosing(in)
	if err != nil {
		t.Fatalf("CompileClosing: %v", err)
	}
	if slide.SlideType != "content" {
		t.Errorf("SlideType = %q, want content", slide.SlideType)
	}
	// title + bullets body.
	if len(slide.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(slide.Content))
	}
	body := slide.Content[1]
	if body.Type != "bullets" || body.BulletsValue == nil {
		t.Fatalf("body not bullets: %+v", body)
	}
	if !reflect.DeepEqual(*body.BulletsValue, []string{"first", "second"}) {
		t.Errorf("bullets = %v, want [first second]", *body.BulletsValue)
	}
}

func TestCompileClosing_TitleBranch(t *testing.T) {
	in := Input{Title: "Thanks", Body: map[string]any{"subtitle": "see you"}}
	slide, _, err := CompileClosing(in)
	if err != nil {
		t.Fatalf("CompileClosing: %v", err)
	}
	if slide.SlideType != "title" {
		t.Errorf("SlideType = %q, want title", slide.SlideType)
	}
	if len(slide.Content) != 2 {
		t.Fatalf("Content len = %d, want 2 (title+subtitle)", len(slide.Content))
	}
}

func TestCompileExecutiveSummary(t *testing.T) {
	in := Input{
		Title:    "Summary",
		Takeaway: "We should ship",
		Body: map[string]any{
			"points": []any{"point a", "point b"},
		},
	}
	slide, links, err := CompileExecutiveSummary(in)
	if err != nil {
		t.Fatalf("CompileExecutiveSummary: %v", err)
	}
	if slide.SlideType != "content" {
		t.Errorf("SlideType = %q, want content", slide.SlideType)
	}
	if slide.Takeaway != "We should ship" {
		t.Errorf("Takeaway = %q", slide.Takeaway)
	}
	if len(slide.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(slide.Content))
	}
	if slide.Content[1].BulletsValue == nil || len(*slide.Content[1].BulletsValue) != 2 {
		t.Errorf("points not rendered as bullets: %+v", slide.Content[1])
	}
	// Expect a takeaway link among the emitted links.
	if !hasRawPath(links, in.rawSlide()+".takeaway") {
		t.Error("missing takeaway source link")
	}
}

func TestCompileExecutiveSummaryTakeaways(t *testing.T) {
	// Regression: the plural "takeaways" array is the body content and must be
	// compiled into a bullets block, not silently dropped (go-slide-creator-weyf).
	in := Input{
		Title:    "Summary",
		Takeaway: "We should ship",
		Body: map[string]any{
			"takeaways": []any{"finding a", "finding b", "finding c", "finding d"},
		},
	}
	slide, links, err := CompileExecutiveSummary(in)
	if err != nil {
		t.Fatalf("CompileExecutiveSummary: %v", err)
	}
	if len(slide.Content) != 2 {
		t.Fatalf("Content len = %d, want 2 (title + takeaways bullets)", len(slide.Content))
	}
	if slide.Content[1].BulletsValue == nil || len(*slide.Content[1].BulletsValue) != 4 {
		t.Errorf("takeaways not rendered as bullets: %+v", slide.Content[1])
	}
	assertHasLink(t, links, in.semSlide()+".takeaways")
}

func TestCompileDecision_RecommendationAndOptions(t *testing.T) {
	in := Input{
		Title: "Pick One",
		Body: map[string]any{
			"recommendation": "Go with B",
			"options":        []any{"A", "B"},
		},
	}
	slide, _, err := CompileDecision(in)
	if err != nil {
		t.Fatalf("CompileDecision: %v", err)
	}
	if len(slide.Content) != 2 {
		t.Fatalf("Content len = %d, want 2", len(slide.Content))
	}
	body := slide.Content[1]
	if body.Type != "body_and_bullets" || body.BodyAndBulletsValue == nil {
		t.Fatalf("body not body_and_bullets: %+v", body)
	}
	if body.BodyAndBulletsValue.Body != "Go with B" {
		t.Errorf("body text = %q", body.BodyAndBulletsValue.Body)
	}
	if !reflect.DeepEqual(body.BodyAndBulletsValue.Bullets, []string{"A", "B"}) {
		t.Errorf("bullets = %v", body.BodyAndBulletsValue.Bullets)
	}
}

func TestCompileDecision_OptionsOnly(t *testing.T) {
	in := Input{Title: "T", Body: map[string]any{"options": []any{"X"}}}
	slide, _, err := CompileDecision(in)
	if err != nil {
		t.Fatalf("CompileDecision: %v", err)
	}
	body := slide.Content[len(slide.Content)-1]
	if body.Type != "bullets" {
		t.Errorf("body type = %q, want bullets", body.Type)
	}
}

func TestCompileDecision_RecommendationOnly(t *testing.T) {
	in := Input{Title: "T", Body: map[string]any{"recommendation": "Just do it"}}
	slide, _, err := CompileDecision(in)
	if err != nil {
		t.Fatalf("CompileDecision: %v", err)
	}
	body := slide.Content[len(slide.Content)-1]
	if body.Type != "text" || body.TextValue == nil || *body.TextValue != "Just do it" {
		t.Errorf("body = %+v, want text 'Just do it'", body)
	}
}

func TestCompileFallback(t *testing.T) {
	in := Input{
		Title: "Generic",
		Body: map[string]any{
			"subtitle": "ignored",
			"items":    []any{"one", "two"},
		},
	}
	slide, _, err := CompileFallback(in)
	if err != nil {
		t.Fatalf("CompileFallback: %v", err)
	}
	if slide.SlideType != "content" {
		t.Errorf("SlideType = %q, want content", slide.SlideType)
	}
	body := slide.Content[len(slide.Content)-1]
	if body.BulletsValue == nil || !reflect.DeepEqual(*body.BulletsValue, []string{"one", "two"}) {
		t.Errorf("fallback bullets = %+v, want [one two]", body.BulletsValue)
	}
}

func TestCompileKPISnapshot_PatternPath(t *testing.T) {
	in := Input{
		Title: "Numbers",
		Body: map[string]any{
			"kpis": []any{
				map[string]any{"value": "10", "label": "ARR"},
				map[string]any{"big": "20", "small": "Users"},
				map[string]any{"value": "30", "caption": "NPS"},
			},
		},
	}
	slide, _, err := CompileKPISnapshot(in)
	if err != nil {
		t.Fatalf("CompileKPISnapshot: %v", err)
	}
	if slide.Pattern == nil {
		t.Fatal("expected Pattern, got nil")
	}
	if slide.Pattern.Name != "kpi-3up" {
		t.Errorf("Pattern.Name = %q, want kpi-3up", slide.Pattern.Name)
	}
	var cells []map[string]string
	if err := json.Unmarshal(slide.Pattern.Values, &cells); err != nil {
		t.Fatalf("unmarshal pattern values: %v", err)
	}
	if len(cells) != 3 {
		t.Fatalf("cells len = %d, want 3", len(cells))
	}
	if cells[0]["big"] != "10" || cells[0]["small"] != "ARR" {
		t.Errorf("cell[0] = %v, want big=10 small=ARR", cells[0])
	}
}

func TestCompileKPISnapshot_Fallback(t *testing.T) {
	// Only one metric -> outside the 2..6 kpi-Nup range -> content fallback.
	in := Input{
		Title: "One",
		Body: map[string]any{
			"metrics": []any{
				map[string]any{"value": "99", "label": "Solo"},
			},
		},
	}
	slide, _, err := CompileKPISnapshot(in)
	if err != nil {
		t.Fatalf("CompileKPISnapshot: %v", err)
	}
	if slide.Pattern != nil {
		t.Errorf("expected no pattern in fallback, got %+v", slide.Pattern)
	}
	body := slide.Content[len(slide.Content)-1]
	if body.BulletsValue == nil || (*body.BulletsValue)[0] != "99 — Solo" {
		t.Errorf("fallback bullet = %+v, want '99 — Solo'", body.BulletsValue)
	}
}

func TestCompileChartInsight_PatternWithChart(t *testing.T) {
	in := Input{
		Title: "Trend",
		Body: map[string]any{
			"insights": []any{"up and to the right"},
			"source":   "internal data",
			"chart": map[string]any{
				"type": "bar",
				"data": map[string]any{"series": []any{1, 2, 3}},
			},
		},
	}
	slide, _, err := CompileChartInsight(in)
	if err != nil {
		t.Fatalf("CompileChartInsight: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "chart-insights-split" {
		t.Fatalf("Pattern = %+v, want chart-insights-split", slide.Pattern)
	}
	var vals chartInsightsValues
	if err := json.Unmarshal(slide.Pattern.Values, &vals); err != nil {
		t.Fatalf("unmarshal values: %v", err)
	}
	if vals.Chart == nil || vals.Chart.Type != "bar" {
		t.Errorf("chart not embedded: %+v", vals.Chart)
	}
	if vals.Source != "internal data" {
		t.Errorf("source = %q", vals.Source)
	}
	if len(vals.Insights) != 1 {
		t.Errorf("insights = %v", vals.Insights)
	}
}

func TestCompileChartInsight_PatternNoChart(t *testing.T) {
	// Chart with no data -> chartSpec returns nil -> pattern with insights only.
	in := Input{
		Title: "Trend",
		Body: map[string]any{
			"insight": "single insight string",
			"chart":   map[string]any{"type": "bar"},
		},
	}
	slide, _, err := CompileChartInsight(in)
	if err != nil {
		t.Fatalf("CompileChartInsight: %v", err)
	}
	if slide.Pattern == nil {
		t.Fatal("expected pattern, got nil")
	}
	var vals chartInsightsValues
	if err := json.Unmarshal(slide.Pattern.Values, &vals); err != nil {
		t.Fatalf("unmarshal values: %v", err)
	}
	if vals.Chart != nil {
		t.Errorf("chart should be nil without data, got %+v", vals.Chart)
	}
	if len(vals.Insights) != 1 || vals.Insights[0] != "single insight string" {
		t.Errorf("insights = %v", vals.Insights)
	}
}

func TestCompileChartInsight_TakeawayPreservesChart(t *testing.T) {
	// A usable chart + takeaway but no explicit insights must still emit the
	// chart-insights-split pattern with the chart — the takeaway becomes the
	// single insight. Regression for the silent chart drop (go-slide-creator-q7hf).
	in := Input{
		Title:    "Trend",
		Takeaway: "Revenue is accelerating",
		Body: map[string]any{
			"takeaway": "Revenue is accelerating",
			"chart": map[string]any{
				"type": "bar",
				"data": map[string]any{"series": []any{1, 2, 3}},
			},
		},
	}
	slide, _, err := CompileChartInsight(in)
	if err != nil {
		t.Fatalf("CompileChartInsight: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "chart-insights-split" {
		t.Fatalf("Pattern = %+v, want chart-insights-split (chart must not be dropped)", slide.Pattern)
	}
	var vals chartInsightsValues
	if err := json.Unmarshal(slide.Pattern.Values, &vals); err != nil {
		t.Fatalf("unmarshal values: %v", err)
	}
	if vals.Chart == nil || vals.Chart.Type != "bar" {
		t.Errorf("chart not embedded: %+v", vals.Chart)
	}
	if len(vals.Insights) != 1 || vals.Insights[0] != "Revenue is accelerating" {
		t.Errorf("insights = %v, want [takeaway]", vals.Insights)
	}
}

func TestCompileChartInsight_Fallback(t *testing.T) {
	// No insights at all -> fallback content slide.
	in := Input{Title: "Empty", Body: map[string]any{}}
	slide, _, err := CompileChartInsight(in)
	if err != nil {
		t.Fatalf("CompileChartInsight: %v", err)
	}
	if slide.Pattern != nil {
		t.Errorf("expected no pattern, got %+v", slide.Pattern)
	}
	if slide.SlideType != "content" {
		t.Errorf("SlideType = %q, want content", slide.SlideType)
	}
}

func TestCompileRaw(t *testing.T) {
	in := Input{
		Body: map[string]any{
			"slide": map[string]any{
				"slide_type": "blank",
				"eyebrow":    "RAW",
			},
		},
	}
	slide, links, err := CompileRaw(in)
	if err != nil {
		t.Fatalf("CompileRaw: %v", err)
	}
	if slide.SlideType != "blank" {
		t.Errorf("SlideType = %q, want blank", slide.SlideType)
	}
	if slide.Eyebrow != "RAW" {
		t.Errorf("Eyebrow = %q, want RAW", slide.Eyebrow)
	}
	if len(links) != 1 {
		t.Errorf("links len = %d, want 1", len(links))
	}
}

func TestCompileRaw_MissingPayload(t *testing.T) {
	_, _, err := CompileRaw(Input{Body: map[string]any{}})
	if err == nil {
		t.Fatal("expected error for missing slide payload, got nil")
	}
}

// TestCompileRaw_NonObjectPayload is a regression for go-slide-creator-p9rp: a
// non-object "slide" payload must fail without leaking the internal Go target
// type (deckinput.SlideInput) from the json.Unmarshal error.
func TestCompileRaw_NonObjectPayload(t *testing.T) {
	_, _, err := CompileRaw(Input{Body: map[string]any{"slide": "i am a string not an object"}})
	if err == nil {
		t.Fatal("expected error for non-object slide payload, got nil")
	}
	if strings.Contains(err.Error(), "deckinput.SlideInput") {
		t.Errorf("CompileRaw error leaks internal type name: %v", err)
	}
}

// hasRawPath reports whether any link carries the given RawPath.
func hasRawPath(links []SourceLink, raw string) bool {
	for _, l := range links {
		if l.RawPath == raw {
			return true
		}
	}
	return false
}

// TestStringListScalarFormatting is a regression for go-slide-creator-ndbl:
// JSON numbers decode to float64 and must render as plain decimals, never in
// scientific notation, and booleans must stringify deliberately rather than
// leaking through fmt's default formatting.
func TestStringListScalarFormatting(t *testing.T) {
	body := map[string]any{
		// Mirrors the bead's reproduction: values as they decode from JSON.
		"points": []any{float64(1000000), float64(0.0000001), true, float64(3.14159265358979)},
	}
	list, ok := stringList(body, "points")
	if !ok {
		t.Fatal("stringList ok = false, want true")
	}
	want := []string{"1000000", "0.0000001", "true", "3.14159265358979"}
	if !reflect.DeepEqual(list, want) {
		t.Errorf("stringList = %#v, want %#v", list, want)
	}
	for _, s := range list {
		if strings.Contains(s, "e+") || strings.Contains(s, "e-") ||
			strings.Contains(s, "E+") || strings.Contains(s, "E-") {
			t.Errorf("bullet %q contains exponent notation", s)
		}
	}
}

func TestHelpers(t *testing.T) {
	body := map[string]any{
		"name":    "  trimmed  ",
		"nums":    []any{"a", 2, nil, "  ", "b"},
		"notlist": "scalar",
	}

	if got := strField(body, "name"); got != "trimmed" {
		t.Errorf("strField = %q, want trimmed", got)
	}
	if got := strField(body, "missing"); got != "" {
		t.Errorf("strField missing = %q, want empty", got)
	}
	if got := strField(nil, "name"); got != "" {
		t.Errorf("strField nil body = %q, want empty", got)
	}

	list, ok := stringList(body, "nums")
	if !ok {
		t.Fatal("stringList ok = false, want true")
	}
	// "a", fmt(2)="2", "b" — nil and blank skipped.
	if !reflect.DeepEqual(list, []string{"a", "2", "b"}) {
		t.Errorf("stringList = %v, want [a 2 b]", list)
	}
	if _, ok := stringList(body, "notlist"); ok {
		t.Error("stringList on scalar ok = true, want false")
	}

	if got := firstNonEmpty("", "  ", "x"); got != "x" {
		t.Errorf("firstNonEmpty = %q, want x", got)
	}
	if got := firstNonEmpty("", "  "); got != "" {
		t.Errorf("firstNonEmpty all empty = %q, want empty", got)
	}

	keys := sortedKeys(map[string]any{"c": 1, "a": 1, "b": 1})
	if !reflect.DeepEqual(keys, []string{"a", "b", "c"}) {
		t.Errorf("sortedKeys = %v, want [a b c]", keys)
	}
}
