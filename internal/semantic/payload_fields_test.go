package semantic

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

// go-slide-creator-dg8f: malformed chart data must point at chart.data (the
// path the author edits) and show the expected {categories, series[]} shape.
func TestValidateChartData_MapDataPointsAtChartData(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title":    "Revenue",
			"chart":    map[string]any{"type": "bar_chart", "data": map[string]any{"Q1": 1}},
			"insights": []any{"Up."},
			"takeaway": "Up.",
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticPatternDegraded, "slides[0].chart.data")
	if !ok {
		t.Fatalf("expected finding at slides[0].chart.data, got %v", ds)
	}
	if !strings.Contains(d.Message, "{categories:[…], series:[{name, values:[…]}]}") {
		t.Errorf("message should show the expected shape, got %q", d.Message)
	}
	if d.Fix == nil || d.Fix.Params["path"] != "slides[0].chart.data" || d.Fix.Params["example"] == nil {
		t.Errorf("fix.params must carry path + example, got %+v", d.Fix)
	}
	for _, x := range ds {
		if strings.HasSuffix(x.Path, "chart.series") {
			t.Errorf("no finding may point at the non-existent chart.series path: %+v", x)
		}
	}
}

func TestValidateChartData_PieValuesAccepted(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindChartInsight, Body: map[string]any{
			"title": "Mix", "takeaway": "Enterprise leads.", "insights": []any{"Enterprise leads."},
			"chart": map[string]any{"type": "pie_chart", "data": map[string]any{"categories": []any{"A", "B"}, "values": []any{1, 2}}},
		}}},
	}
	for _, d := range Validate(spec, StrictnessWarn) {
		if strings.HasPrefix(d.Path, "slides[0].chart") {
			t.Errorf("valid pie data flagged: %+v", d)
		}
	}
}

// go-slide-creator-h8o7: an unknown payload field is reported (not silently
// dropped) with its full path and a did-you-mean rename.
func TestValidateUnknownFields_KPIPayload(t *testing.T) {
	spec := &DeckSpec{
		Meta: DeckMeta{Title: "Deck"},
		Slides: []SlideSpec{{Kind: KindKPISnapshot, Body: map[string]any{
			"title":    "KPIs",
			"takeaway": "Strong quarter.",
			"takeawy":  "typo",
			"kpis": []any{
				map[string]any{"value": "$4M", "label": "ARR"},
				map[string]any{"valeu": "118%", "label": "NRR"},
			},
		}}},
	}
	ds := Validate(spec, StrictnessWarn)
	top, ok := findAt(ds, diagnostics.CodeSemanticUnknownField, "slides[0].takeawy")
	if !ok {
		t.Fatalf("expected SEMANTIC_UNKNOWN_FIELD at slides[0].takeawy, got %v", ds)
	}
	if top.Severity != diagnostics.SeverityError || top.Fix == nil || top.Fix.Params["did_you_mean"] != "takeaway" {
		t.Errorf("unexpected top-level finding: %+v", top)
	}
	item, ok := findAt(ds, diagnostics.CodeSemanticUnknownField, "slides[0].kpis[1].valeu")
	if !ok {
		t.Fatalf("expected SEMANTIC_UNKNOWN_FIELD at slides[0].kpis[1].valeu, got %v", ds)
	}
	if item.Fix == nil || item.Fix.Params["did_you_mean"] != "value" {
		t.Errorf("expected did_you_mean value, got %+v", item.Fix)
	}
	// A dropped field is structural, not advisory: every strictness blocks it.
	for _, mode := range []Strictness{StrictnessOff, StrictnessWarn, StrictnessStrict} {
		d, ok := findAt(Validate(spec, mode), diagnostics.CodeSemanticUnknownField, "slides[0].takeawy")
		if !ok || d.Severity != diagnostics.SeverityError {
			t.Errorf("%s must refuse the dropped field: %+v", mode, d)
		}
	}
}

func TestSectionSubtitleIsDiagnosedInsteadOfDropped(t *testing.T) {
	spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{
		Kind: KindSection, Body: map[string]any{"title": "Next chapter", "subtitle": "Supporting copy"},
	}}}
	ds := Validate(spec, StrictnessWarn)
	d, ok := findAt(ds, diagnostics.CodeSemanticUnknownField, "slides[0].subtitle")
	if !ok || d.Severity != diagnostics.SeverityError {
		t.Fatalf("unsupported section subtitle silently dropped: %+v", ds)
	}
	if d.Fix != nil || !strings.Contains(d.Message, "decorative numbers") || !strings.Contains(d.Message, "shape_grid") {
		t.Fatalf("section subtitle guidance is misleading: %+v", d)
	}
	off, ok := findAt(Validate(spec, StrictnessOff), diagnostics.CodeSemanticUnknownField, "slides[0].subtitle")
	if !ok || off.Severity != diagnostics.SeverityError {
		t.Fatalf("off mode did not reject dropped section.subtitle: %+v", off)
	}
	info, ok := LookupKind(KindSection)
	if !ok {
		t.Fatal("section kind not registered")
	}
	for _, field := range info.TypicalFields {
		if field == "subtitle" {
			t.Fatalf("section still advertises unsupported subtitle: %+v", info)
		}
	}
	for _, field := range PayloadFieldNames(KindSection) {
		if field == "subtitle" {
			t.Fatalf("section schema still accepts unsupported subtitle: %v", PayloadFieldNames(KindSection))
		}
	}
}

// Every kind's example must validate with zero findings — list_slide_kinds
// publishes these as copy-ready payloads.
func TestKindExamplesValidateClean(t *testing.T) {
	for _, k := range AllSlideKinds() {
		ex := KindExample(k)
		if ex == nil {
			t.Errorf("kind %s has no example", k)
			continue
		}
		if ex["kind"] != string(k) {
			t.Errorf("example for %s has kind %v", k, ex["kind"])
		}
		body := map[string]any{}
		for key, v := range ex {
			if key != "kind" {
				body[key] = v
			}
		}
		spec := &DeckSpec{Meta: DeckMeta{Title: "Deck"}, Slides: []SlideSpec{{Kind: k, Body: body}}}
		if ds := Validate(spec, StrictnessStrict); len(ds) != 0 {
			t.Errorf("example for %s is not clean under strict: %v", k, ds)
		}
	}
}

// The payload contract must cover every documented required/typical field so
// the closed schema never rejects a documented key.
func TestPayloadContractCoversKindRegistry(t *testing.T) {
	for _, k := range AllSlideKinds() {
		info, _ := LookupKind(k)
		fields := PayloadFieldNames(k)
		check := append(append([]string{}, info.RequiredFields...), info.TypicalFields...)
		for _, aliases := range info.RequiredAliases {
			check = append(check, aliases...)
		}
		for _, f := range check {
			if !containsKey(fields, f) {
				t.Errorf("kind %s documents %q but the payload contract omits it", k, f)
			}
		}
	}
}
