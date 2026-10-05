package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

// go-slide-creator-ptazs: the semantic kinds hid fields their own patterns
// draw, so agents fell back to raw_json2pptx for a roadmap with parallel
// workstreams, a today → target comparison with connectors, or a wide
// screenshot beside text.

func roadmapTracksBody(tracks any) map[string]any {
	return map[string]any{
		"phases": []any{
			map[string]any{"name": "Pilot", "date_label": "Q1", "description": "Prove value"},
			map[string]any{"name": "Expand", "date_label": "Q2", "description": "Scale"},
			map[string]any{"name": "GA", "date_label": "Q3", "milestone": "Launch"},
		},
		"parallel_tracks": tracks,
	}
}

func TestCompileRoadmap_ParallelTracksReachThePattern(t *testing.T) {
	body := roadmapTracksBody([]any{"Data and reporting platform", map[string]any{"label": "Training 1,200 control owners"}, " ", 7})
	body["parallel_label"] = "Workstreams"
	slide, links, err := CompileRoadmap(Input{Title: "Roadmap", Body: body})
	if err != nil {
		t.Fatalf("CompileRoadmap: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "phase-roadmap" {
		t.Fatalf("expected phase-roadmap pattern, got %+v", slide.Pattern)
	}
	var vals phaseRoadmapValues
	decodePattern(t, slide.Pattern.Values, &vals)
	if got, want := strings.Join(vals.ParallelTracks, "|"), "Data and reporting platform|Training 1,200 control owners"; got != want {
		t.Errorf("parallel_tracks = %q, want %q", got, want)
	}
	if vals.ParallelLabel != "Workstreams" {
		t.Errorf("parallel_label = %q, want Workstreams", vals.ParallelLabel)
	}
	linked := false
	for _, l := range links {
		if strings.HasSuffix(l.RawPath, ".pattern.values.parallel_tracks") && strings.HasSuffix(l.SemanticPath, ".parallel_tracks") {
			linked = true
		}
	}
	if !linked {
		t.Errorf("no source link from pattern.values.parallel_tracks back to the authored field: %+v", links)
	}

	// The alias an author who says "parallel workstreams" reaches for.
	alias := roadmapTracksBody(nil)
	delete(alias, "parallel_tracks")
	alias["workstreams"] = []any{"Governance"}
	slide, _, err = CompileRoadmap(Input{Body: alias})
	if err != nil {
		t.Fatal(err)
	}
	decodePattern(t, slide.Pattern.Values, &vals)
	if len(vals.ParallelTracks) != 1 || vals.ParallelTracks[0] != "Governance" {
		t.Errorf("workstreams alias: parallel_tracks = %v", vals.ParallelTracks)
	}

	plain, _, err := CompileRoadmap(Input{Body: roadmapTracksBody(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain.Pattern.Values), "parallel") {
		t.Errorf("no tracks asked for, but values carry them: %s", plain.Pattern.Values)
	}
}

func TestRoadmapBudgetItemsAndFallback(t *testing.T) {
	five := roadmapTracksBody([]any{"A", "B", "C", "D", "E"})
	if over := RoadmapOverBudget(five); !strings.Contains(over, "5") || !strings.Contains(over, "4") {
		t.Errorf("five tracks: RoadmapOverBudget = %q, want the count and the cap", over)
	}
	slide, _, err := CompileRoadmap(Input{Title: "Roadmap", Body: five})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern != nil {
		t.Fatalf("five tracks must degrade (the pattern refuses them), got %+v", slide.Pattern)
	}
	raw, _ := json.Marshal(slide)
	for _, want := range []string{"In parallel", "A · B · C · D · E", "Launch"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("fallback dropped %q: %s", want, raw)
		}
	}
	assertNoGoMapLeak(t, slide)

	long := roadmapTracksBody([]any{strings.Repeat("x", 91)})
	items := RoadmapBudgetItems(long)
	if len(items) != 1 || items[0].Field != "parallel_tracks[0]" || items[0].Allowed != 90 || items[0].Measured != 91 {
		t.Fatalf("long track: items = %+v", items)
	}

	milestone := roadmapTracksBody(nil)
	milestone["phases"].([]any)[2].(map[string]any)["milestone"] = strings.Repeat("m", 61)
	items = RoadmapBudgetItems(milestone)
	if len(items) != 1 || items[0].Field != "phases[2].milestone" || items[0].Allowed != 60 {
		t.Fatalf("long milestone: items = %+v", items)
	}
	if RoadmapOverBudget(roadmapTracksBody([]any{"Change management"})) != "" {
		t.Error("a roadmap within its budgets reports an overrun")
	}
}

func comparisonShiftBody(extra map[string]any) map[string]any {
	body := map[string]any{
		"columns": []any{
			map[string]any{"header": "Today", "items": []any{"Manual reconciliation", "Monthly close in 9 days"}},
			map[string]any{"header": "Target", "items": []any{"Automated matching", "Close in 3 days"}},
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

func TestCompileComparison_ConnectorsAndHighlightsReachThePattern(t *testing.T) {
	body := comparisonShiftBody(map[string]any{"connectors": true, "highlight_column": "target"})
	slide, _, err := CompileComparison(Input{Title: "From today to target", Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "comparison-2col" {
		t.Fatalf("pattern = %+v, want comparison-2col", slide.Pattern)
	}
	var ovr comparison2colOverrides
	if err := json.Unmarshal(slide.Pattern.Overrides, &ovr); err != nil {
		t.Fatalf("overrides %s: %v", slide.Pattern.Overrides, err)
	}
	if !ovr.Connectors || ovr.HighlightColumn != "right" {
		t.Errorf("overrides = %+v, want connectors and the right column (matched by header)", ovr)
	}

	// A row by index or by its text.
	for _, ref := range []any{1.0, "close in 3 days"} {
		slide, _, err = CompileComparison(Input{Body: comparisonShiftBody(map[string]any{"highlight_row": ref})})
		if err != nil {
			t.Fatal(err)
		}
		var vals comparison2colValues
		decodePattern(t, slide.Pattern.Values, &vals)
		if len(vals.Rows) != 2 || vals.Rows[0].Highlight || !vals.Rows[1].Highlight {
			t.Errorf("highlight_row %v: rows = %+v", ref, vals.Rows)
		}
		if len(slide.Pattern.Overrides) != 0 {
			t.Errorf("highlight_row %v: overrides emitted with nothing to say: %s", ref, slide.Pattern.Overrides)
		}
	}

	plain, _, err := CompileComparison(Input{Body: comparisonShiftBody(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Pattern.Overrides) != 0 || strings.Contains(string(plain.Pattern.Values), "highlight") {
		t.Errorf("plain comparison carries overrides: %s %s", plain.Pattern.Overrides, plain.Pattern.Values)
	}
}

func TestComparisonHighlightResolution(t *testing.T) {
	body := comparisonShiftBody(nil)
	for ref, want := range map[string]string{"left": "left", "Right": "right", "TODAY": "left", "Target": "right"} {
		body["highlight_column"] = ref
		if got, ok := ComparisonHighlightColumn(body); !ok || got != want {
			t.Errorf("highlight_column %q = %q, %v; want %q", ref, got, ok, want)
		}
	}
	body["highlight_column"] = "Middle"
	if _, ok := ComparisonHighlightColumn(body); ok {
		t.Error("highlight_column Middle resolved")
	}
	delete(body, "highlight_column")
	body["highlight_row"] = 5.0
	if _, ok := ComparisonHighlightRow(body); ok {
		t.Error("highlight_row 5 resolved on a two-row comparison")
	}
	body["highlight_row"] = "Nothing like this"
	if _, ok := ComparisonHighlightRow(body); ok {
		t.Error("highlight_row with no matching text resolved")
	}
	body["highlight_row"] = "Manual reconciliation"
	if idx, ok := ComparisonHighlightRow(body); !ok || idx != 0 {
		t.Errorf("highlight_row by left text = %d, %v", idx, ok)
	}
}

func TestCompileComparison_OverridesStayOnTheTwoColumnVisual(t *testing.T) {
	body := map[string]any{
		"connectors": true,
		"columns": []any{
			map[string]any{"header": "A", "items": []any{"one"}},
			map[string]any{"header": "B", "items": []any{"two"}},
			map[string]any{"header": "C", "items": []any{"three"}},
		},
	}
	slide, _, err := CompileComparison(Input{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "stylish-panels" {
		t.Fatalf("pattern = %+v, want stylish-panels", slide.Pattern)
	}
	if len(slide.Pattern.Overrides) != 0 {
		t.Errorf("connectors leaked onto stylish-panels: %s", slide.Pattern.Overrides)
	}
}

func TestImageCaseWidthReachesThePattern(t *testing.T) {
	body := imageCaseBody(map[string]any{
		"image":           map[string]any{"path": "/assets/console.png", "fit": "contain"},
		"image_side":      "right",
		"image_width_pct": 60.0,
	})
	slide, _, err := CompileImageCase(Input{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	var ovr imageCaseOverrides
	if err := json.Unmarshal(slide.Pattern.Overrides, &ovr); err != nil {
		t.Fatalf("overrides %s: %v", slide.Pattern.Overrides, err)
	}
	if ovr.ImageWidthPct != 60 || ovr.ImageSide != "right" {
		t.Errorf("overrides = %+v, want width 60 on the right", ovr)
	}
	if pct, ok := ImageCaseImageWidthPct(body); !ok || pct != 60 {
		t.Errorf("ImageCaseImageWidthPct = %v, %v", pct, ok)
	}
	// Out of the pattern's 30–60: validation reports it; the compiler does not
	// pass a value the pattern would refuse.
	body["image_width_pct"] = 75.0
	slide, _, err = CompileImageCase(Input{Body: body})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(slide.Pattern.Overrides), "image_width_pct") {
		t.Errorf("out-of-range width reached the pattern: %s", slide.Pattern.Overrides)
	}
	if _, ok := ImageCaseImageWidthPct(body); ok {
		t.Error("75 reported as a usable width")
	}
	plain, _, err := CompileImageCase(Input{Body: imageCaseBody(map[string]any{"image": "/assets/photo.jpg"})})
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Pattern.Overrides) != 0 {
		t.Errorf("no width or side asked for, but overrides were emitted: %s", plain.Pattern.Overrides)
	}
}
