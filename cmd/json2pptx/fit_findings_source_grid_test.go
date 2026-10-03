package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/semantic"
)

// sourceKindsSpec compiles a DeckSpec whose data slides become shape grids and
// table-highlight patterns rather than content items: a table of currency,
// percentage and period figures, a table of words, a text-scale option matrix
// of figures and a Harvey matrix (go-slide-creator-q2emv,
// go-slide-creator-1jtn7).
func sourceKindsSpec(t *testing.T, metaSource, slide0Source string) (*PresentationInput, *semantic.CompileResult) {
	t.Helper()
	numeric := map[string]any{"title": "Spend rose in every period while margin held",
		"headers": []any{"Period", "Spend", "Margin"},
		"rows":    []any{[]any{"FY24", "$1.2m", "18%"}, []any{"FY25", "$1.6m", "19%"}}}
	if slide0Source != "" {
		numeric["source"] = slide0Source
	}
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Sources", Source: metaSource},
		Slides: []semantic.SlideSpec{
			{Kind: semantic.KindTable, Body: numeric},
			{Kind: semantic.KindTable, Body: map[string]any{"title": "Every workstream has a named owner",
				"headers": []any{"Workstream", "Owner"},
				"rows":    []any{[]any{"Pricing", "Ana"}, []any{"Supply", "Ben"}}}},
			{Kind: semantic.KindOptionMatrix, Body: map[string]any{"title": "Partnering adds capacity at the lowest spend", "scale": "text",
				"criteria": []any{"Spend", "Capacity"},
				"options": []any{
					map[string]any{"name": "Status quo", "scores": []any{"$0", "0k"}},
					map[string]any{"name": "Partner", "scores": []any{"$0.4m", "25k"}},
					map[string]any{"name": "Build", "scores": []any{"$1.2m", "25k"}},
				}}},
			{Kind: semantic.KindOptionMatrix, Body: map[string]any{"title": "Partnering scores best on speed and risk",
				"criteria": []any{"Speed", "Risk"},
				"options": []any{
					map[string]any{"name": "Build", "scores": []any{1, 2}},
					map[string]any{"name": "Partner", "scores": []any{4, 3}},
				}}},
		},
	}
	input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, result.Diagnostics)
	}
	return input, result
}

// TestDeckSourceReachesGridTablesAndOptionMatrices pins one rule for every
// data-bearing kind: meta.source lands once on the numeric table and the
// text-scale matrix of figures, an explicit slide source wins, and the word
// table and Harvey matrix stay unsourced.
func TestDeckSourceReachesGridTablesAndOptionMatrices(t *testing.T) {
	input, _ := sourceKindsSpec(t, "Illustrative planning case", "")
	applyDefaults(input)
	want := []string{"Illustrative planning case", "", "Illustrative planning case", ""}
	for i, w := range want {
		if got := input.Slides[i].Source; got != w {
			t.Errorf("slide %d (%q) source = %q, want %q", i, slideDataKind(input.Slides[i]), got, w)
		}
	}
	if got := dataWithoutSourceCodes(collectDataWithoutSourceFindings(input)); len(got) != 0 {
		t.Errorf("a defaulted deck should raise no DATA_WITHOUT_SOURCE, got %v", got)
	}

	input, _ = sourceKindsSpec(t, "Illustrative planning case", "Company filings")
	applyDefaults(input)
	if got := input.Slides[0].Source; got != "Company filings" {
		t.Errorf("explicit slide source should win over meta.source, got %q", got)
	}
}

// TestDataWithoutSourceOnGridTablesAndOptionMatrices: with neither a default
// nor an explicit source, the numeric table and the matrix of figures are
// flagged at a path the source map resolves to the DeckSpec field.
func TestDataWithoutSourceOnGridTablesAndOptionMatrices(t *testing.T) {
	input, result := sourceKindsSpec(t, "", "")
	applyDefaults(input)
	got := dataWithoutSourceCodes(collectDataWithoutSourceFindings(input))
	want := map[string]string{"/slides/0/source": "slides[0].source", "/slides/2/source": "slides[2].source"}
	if len(got) != len(want) {
		t.Errorf("DATA_WITHOUT_SOURCE at %v, want exactly %v", got, want)
	}
	for raw, sem := range want {
		if !got[raw] {
			t.Errorf("DATA_WITHOUT_SOURCE missing at %s", raw)
		}
		if path, _, ok := result.SourceMap.ResolveSemantic(raw); !ok || path != sem {
			t.Errorf("ResolveSemantic(%s) = %q, %v; want %q", raw, path, ok, sem)
		}
	}
}

// TestDeckSourceReachesRegionsSlides keeps go-slide-creator-fn2ka on the shared
// rule: a regions slide with a chart region takes meta.source.
func TestDeckSourceReachesRegionsSlides(t *testing.T) {
	spec := &semantic.DeckSpec{
		Meta: semantic.DeckMeta{Title: "Regions", Source: "Finance ledger"},
		Slides: []semantic.SlideSpec{{Kind: semantic.KindRegions, Body: map[string]any{
			"title":       "Growth funds the launch",
			"arrangement": "columns",
			"regions": []any{
				map[string]any{"kind": "chart", "chart": map[string]any{"type": "line_chart", "data": map[string]any{
					"categories": []any{"Q1", "Q2", "Q3"},
					"series":     []any{map[string]any{"name": "Revenue", "values": []any{12, 14, 17}}},
				}}},
				map[string]any{"kind": "text", "body": "Enterprise doubled."},
			},
		}}},
	}
	input, result, err := semantic.Compile(spec, semantic.CompileOptions{Strict: semantic.StrictnessWarn})
	if err != nil {
		t.Fatalf("compile: %v (%+v)", err, result.Diagnostics)
	}
	applyDefaults(input)
	if got := input.Slides[0].Source; got != "Finance ledger" {
		t.Errorf("regions slide source = %q, want meta.source", got)
	}
}

// TestSlideDataKindReadsRawGridCells: a raw shape_grid is classified by its
// cells — a table of figures, a chart diagram, a cell-hosted KPI pattern in a
// nested grid count; a word table and a Harvey table-highlight do not.
func TestSlideDataKindReadsRawGridCells(t *testing.T) {
	cases := []struct{ grid, want string }{
		{`{"rows":[{"cells":[{"table":{"headers":["Q","Rev"],"rows":[["Q1","12"]]}}]}]}`, "a table of figures"},
		{`{"rows":[{"cells":[{"table":{"headers":["Role","Owner"],"rows":[["Lead","Ana"]]}}]}]}`, ""},
		{`{"rows":[{"cells":[{"diagram":{"type":"bar_chart","data":{}}}]}]}`, "a chart"},
		{`{"rows":[{"cells":[{"grid":{"rows":[{"cells":[{"pattern":{"name":"kpi-2up","values":[]}}]}]}}]}]}`, "the kpi-2up pattern"},
		{`{"rows":[{"cells":[{"pattern":{"name":"table-highlight","values":{"criteria":["A"],"options":[{"name":"x","scores":[3]}]}}}]}]}`, ""},
		{`{"rows":[{"cells":[{"pattern":{"name":"table-highlight","values":{"scale":"rag","criteria":["A"],"options":[{"name":"x","scores":["green"]}]}}}]}]}`, ""},
		{`{"rows":[{"cells":[{"pattern":{"name":"table-highlight","values":{"scale":"text","criteria":["Cost"],"options":[{"name":"x","scores":["$3m"]}]}}}]}]}`, "a matrix of figures"},
		{`{"rows":[{"cells":[{"pattern":{"name":"table-highlight","values":{"scale":"text","criteria":["Fit"],"options":[{"name":"x","scores":["Strong"]}]}}}]}]}`, ""},
	}
	for _, tc := range cases {
		var g ShapeGridInput
		if err := json.Unmarshal([]byte(tc.grid), &g); err != nil {
			t.Fatalf("%s: %v", tc.grid, err)
		}
		if got := slideDataKind(SlideInput{ShapeGrid: &g}); got != tc.want {
			t.Errorf("%s: slideDataKind = %q, want %q", tc.grid, got, tc.want)
		}
	}
}
