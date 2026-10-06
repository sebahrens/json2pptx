package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// unknownKeyPaths returns the sorted paths checkInputUnknownKeys reports for a
// deck whose only slide is the given object.
func unknownKeyPaths(t *testing.T, slide string) []string {
	t.Helper()
	raw := json.RawMessage(`{"template":"t","slides":[` + slide + `]}`)
	if !json.Valid(raw) {
		t.Fatalf("test deck is not valid JSON: %s", raw)
	}
	var got []string
	for _, w := range checkInputUnknownKeys(raw) {
		if w.Code != "unknown_key" {
			t.Errorf("finding %s has code %q, want unknown_key", w.Path, w.Code)
		}
		if !w.Authored {
			t.Errorf("finding %s is not marked Authored", w.Path)
		}
		got = append(got, w.Path)
	}
	sort.Strings(got)
	return got
}

// gridWithCell wraps one cell in a single-row shape_grid slide.
func gridWithCell(cell string) string {
	return `{"shape_grid":{"rows":[{"cells":[` + cell + `]}]}}`
}

// A misspelled key is dropped by the decoder wherever it sits, so the check
// has to reach every container a slide can nest: each case holds the typos of
// one container beside working keys of the same object, and expects exactly
// the typos, at their JSON pointers (go-slide-creator-q1m8c).
func TestUnknownKeys_NestedContainers(t *testing.T) {
	const cell = "/slides/0/shape_grid/rows/0/cells/0"
	tests := []struct {
		name  string
		slide string
		want  []string
	}{
		{
			name: "nested grid",
			slide: gridWithCell(`{"grid":{"gap":4,"gapp":4,"bounds":{"x":0,"y":0,"width":100,"hieght":100},"rows":[
				{"height":50,"hight":50,"connector":{"style":"arrow","colour":"accent1"},"cells":[
					{"col_span":1,"colspan":1,"shape":{"geometry":"rect","fil":"accent1"}},
					{"icon":{"name":"chart-pie","colour":"accent1"}},
					{"accent_bar":{"position":"left","thickness":4}},
					{"image":{"path":"a.png","overlay":{"color":"dk1","opacity":0.4},"text":{"content":"x","colour":"lt1"}}},
					{"table":{"headers":["a"],"rows":[[{"content":"x","colspan":2}]],"style":{"borders":"none","border":"none"}}}
				]}
			]}}`),
			want: []string{
				cell + "/grid/bounds/hieght",
				cell + "/grid/gapp",
				cell + "/grid/rows/0/cells/0/colspan",
				cell + "/grid/rows/0/cells/0/shape/fil",
				cell + "/grid/rows/0/cells/1/icon/colour",
				cell + "/grid/rows/0/cells/2/accent_bar/thickness",
				cell + "/grid/rows/0/cells/3/image/overlay/opacity",
				cell + "/grid/rows/0/cells/3/image/text/colour",
				cell + "/grid/rows/0/cells/4/table/rows/0/0/colspan",
				cell + "/grid/rows/0/cells/4/table/style/border",
				cell + "/grid/rows/0/connector/colour",
				cell + "/grid/rows/0/hight",
			},
		},
		{
			name: "grid in a grid in a grid",
			slide: gridWithCell(`{"grid":{"rows":[{"cells":[{"grid":{"rows":[{"cells":[
				{"shape":{"geometry":"rect","txt":"deep"}}
			]}]}}]}]}}`),
			want: []string{cell + "/grid/rows/0/cells/0/grid/rows/0/cells/0/shape/txt"},
		},
		{
			name: "composite",
			slide: gridWithCell(`{"composite":{"split":"top","ratio":0.4,"ration":0.4,
				"text":{"geometry":"rect","fil":"accent1","text":"42%","icon":{"name":"star","sacle":0.5}},
				"sub_diagram":{"type":"line_chart","data":{},"tpye":"x","style":{"show_legend":false,"palette":["accent1"]}}}}`),
			want: []string{
				cell + "/composite/ration",
				cell + "/composite/sub_diagram/style/palette",
				cell + "/composite/sub_diagram/tpye",
				cell + "/composite/text/fil",
				cell + "/composite/text/icon/sacle",
			},
		},
		{
			name: "composite inside a nested grid",
			slide: gridWithCell(`{"grid":{"rows":[{"cells":[
				{"composite":{"txt":{},"text":{"geometry":"rect","fil":"accent1"},"sub_diagram":{"type":"line_chart","data":{}}}}
			]}]}}`),
			want: []string{
				cell + "/grid/rows/0/cells/0/composite/text/fil",
				cell + "/grid/rows/0/cells/0/composite/txt",
			},
		},
		{
			name: "shape icon and link",
			slide: gridWithCell(`{"shape":{"geometry":"rect","flip_h":true,
				"icon":{"name":"star","scale":0.5,"sacle":0.5},
				"link":{"url":"https://example.com","href":"https://example.com"}}}`),
			want: []string{
				cell + "/shape/icon/sacle",
				cell + "/shape/link/href",
			},
		},
		{
			name: "layer shape, its icon and link, in a nested grid",
			slide: gridWithCell(`{"grid":{"rows":[{"cells":[{"layers":[
				{"name":"ring","frame":{"x":0,"y":0,"w":1,"h":1,"width":1},
				 "shape":{"geometry":"ellipse","fil":"accent1","icon":{"name":"star","sacle":0.5},"link":{"slide":2,"page":2}}}
			]}]}]}}`),
			want: []string{
				cell + "/grid/rows/0/cells/0/layers/0/frame/width",
				cell + "/grid/rows/0/cells/0/layers/0/shape/fil",
				cell + "/grid/rows/0/cells/0/layers/0/shape/icon/sacle",
				cell + "/grid/rows/0/cells/0/layers/0/shape/link/page",
			},
		},
		{
			name: "cell diagram style",
			slide: gridWithCell(`{"diagram":{"type":"bar_chart","data":{},"titel":"x",
				"style":{"show_values":true,"show_grid":true,"value_format":{"decimals":1,"decimal":1}},
				"chart_style":{"bar_gap":2}}}`),
			want: []string{
				cell + "/diagram/chart_style/bar_gap",
				cell + "/diagram/style/show_grid",
				cell + "/diagram/style/value_format/decimal",
				cell + "/diagram/titel",
			},
		},
		{
			name: "cell pattern",
			slide: gridWithCell(`{"pattern":{"name":"kpi-3up","values":[],"value":[],
				"callout":{"text":"x","emphasis":"bold","emphasize":"bold"},
				"bounds":{"x":0,"y":0,"width":100,"height":100,"w":100}}}`),
			want: []string{
				cell + "/pattern/bounds/w",
				cell + "/pattern/callout/emphasize",
				cell + "/pattern/value",
			},
		},
		{
			name: "cell pattern inside a nested grid",
			slide: gridWithCell(`{"grid":{"rows":[{"cells":[
				{"pattern":{"name":"kpi-3up","values":[],"overides":{}}}
			]}]}}`),
			want: []string{cell + "/grid/rows/0/cells/0/pattern/overides"},
		},
		{
			name: "grid links",
			slide: `{"shape_grid":{"rows":[{"cells":[{"shape":{"geometry":"rect"}}]}],
				"links":[{"from":[0,0],"to":[0,0],"connector":{"style":"arrow","head":"lg","haed":"lg"},"form":[0,0]}]}}`,
			want: []string{
				"/slides/0/shape_grid/links/0/connector/haed",
				"/slides/0/shape_grid/links/0/form",
			},
		},
		{
			name: "slide pattern",
			slide: `{"pattern":{"name":"kpi-3up","values":[],"vertical_align":"top","valign":"top",
				"callout":{"text":"x","accent":"accent1","acent":"accent1"},
				"bounds":{"x":0,"y":0,"width":100,"height":100,"heigth":100}}}`,
			want: []string{
				"/slides/0/pattern/bounds/heigth",
				"/slides/0/pattern/callout/acent",
				"/slides/0/pattern/valign",
			},
		},
		{
			name: "compose segments",
			slide: `{"compose":{"direction":"vertical","gap":8,"gapp":8,
				"banner":{"text":"x","emphasis":"bold","emphasise":"bold"},
				"callout":{"text":"x","txt":"x"},
				"segments":[
					{"size_pct":40,"size":40,"pattern":{"name":"kpi-3up","values":[],"overides":{},
						"callout":{"text":"x","accent":"accent1","acent":"accent1"},
						"bounds":{"x":0,"y":0,"width":100,"height":100,"hieght":100}}},
					{"diagram":{"type":"bar_chart","data":{},"titel":"x","style":{"show_values":true,"palette":["accent1"]}}},
					{"compose":{"direction":"horizontal","segments":[
						{"pattern":{"name":"kpi-2up","values":[],"cell_override":{}}},
						{"diagram":{"type":"line_chart","data":{},"chart_style":{"bar_gap":2}}}
					]}}
				]}}`,
			want: []string{
				"/slides/0/compose/banner/emphasise",
				"/slides/0/compose/callout/txt",
				"/slides/0/compose/gapp",
				"/slides/0/compose/segments/0/pattern/bounds/hieght",
				"/slides/0/compose/segments/0/pattern/callout/acent",
				"/slides/0/compose/segments/0/pattern/overides",
				"/slides/0/compose/segments/0/size",
				"/slides/0/compose/segments/1/diagram/style/palette",
				"/slides/0/compose/segments/1/diagram/titel",
				"/slides/0/compose/segments/2/compose/segments/0/pattern/cell_override",
				"/slides/0/compose/segments/2/compose/segments/1/diagram/chart_style/bar_gap",
			},
		},
		{
			name: "split_slide base",
			slide: `{"type":"split_slide","split":{},"base":` + gridWithCell(`{"grid":{"rows":[{"cells":[
				{"composite":{"text":{"geometry":"rect","fil":"accent1"}}}
			]}]}}`) + `}`,
			want: []string{"/slides/0/base/shape_grid/rows/0/cells/0/grid/rows/0/cells/0/composite/text/fil"},
		},
		{
			name: "shape fill, line and text objects",
			slide: gridWithCell(`{"shape":{"geometry":"rect",
				"fill":{"color":"accent1","alpha":50,"lumMod":75000,"opacity":50},
				"line":{"color":"dk1","width":1,"dash":"dot","weight":1},
				"text":{"content":"x","size":14,"bold":true,"vertical_align":"t","inset_left":4,"sise":14,"valign":"t"}}}`),
			want: []string{
				cell + "/shape/fill/opacity",
				cell + "/shape/line/weight",
				cell + "/shape/text/sise",
				cell + "/shape/text/valign",
			},
		},
		{
			name: "shape text paragraphs",
			slide: gridWithCell(`{"shape":{"geometry":"rect","text":{"align":"l","paragraph":[],"paragraphs":[
				{"content":"a","size":14,"bold":true,"space_after":6,"bullet":true,"suffix":" m","suffix_size":12},
				{"content":"b","colour":"accent1","spacing_after":6}
			]}}}`),
			want: []string{
				cell + "/shape/text/paragraph",
				cell + "/shape/text/paragraphs/1/colour",
				cell + "/shape/text/paragraphs/1/spacing_after",
			},
		},
		{
			name:  "string shorthands for fill, line and text report nothing",
			slide: gridWithCell(`{"shape":{"geometry":"rect","fill":"accent1","line":"none","text":"plain"}}`),
			want:  nil,
		},
		{
			name: "fill, line and text objects in a layer, a composite and a nested grid",
			slide: gridWithCell(`{"grid":{"rows":[{"cells":[
				{"layers":[{"frame":{"x":0,"y":0,"w":1,"h":1},"shape":{"geometry":"ellipse","fill":{"color":"accent1","alfa":50}}}]},
				{"composite":{"text":{"geometry":"rect","text":{"content":"42%","blod":true}},"sub_diagram":{"type":"line_chart","data":{}}}},
				{"shape":{"geometry":"rect","line":{"color":"dk1","dashes":"dot"}}}
			]}]}}`),
			want: []string{
				cell + "/grid/rows/0/cells/0/layers/0/shape/fill/alfa",
				cell + "/grid/rows/0/cells/1/composite/text/text/blod",
				cell + "/grid/rows/0/cells/2/shape/line/dashes",
			},
		},
		{
			name: "table cell conditional",
			slide: `{"content":[{"placeholder_id":"body","type":"table","table_value":{"headers":["a","b"],"rows":[
				["plain",{"content":"7","conditional":{"rule":"gt","threshold":5,"fill":"accent1","colour":"accent1"}}]
			]}}],"shape_grid":{"rows":[{"cells":[{"table":{"headers":["a"],"rows":[
				[{"content":"7","conditional":{"rule":"gt","threshold":5,"treshold":5}}]
			]}}]}]}}`,
			want: []string{
				"/slides/0/content/0/table_value/rows/0/1/conditional/colour",
				"/slides/0/shape_grid/rows/0/cells/0/table/rows/0/0/conditional/treshold",
			},
		},
		{
			name: "slide overlays",
			slide: `{"overlays":[
				{"kind":"arrow","color":"accent1","width":1.5,"dash":"dot","colour":"accent1",
				 "from":{"x":10,"y":10,"z":1,"anchor_cell":{"row":0,"col":0,"at":"center","column":0}},
				 "to":{"anchor_image":{"row":0,"col":0,"x":0.5,"y":0.5,"units":"px","unit":"px"},"anchor":{}}},
				{"kind":"badge","text":"New","height":5,"from":{"x":1,"y":1},"link":{"url":"https://example.com","href":"x"},"label":"New"}
			]}`,
			want: []string{
				"/slides/0/overlays/0/colour",
				"/slides/0/overlays/0/from/anchor_cell/column",
				"/slides/0/overlays/0/from/z",
				"/slides/0/overlays/0/to/anchor",
				"/slides/0/overlays/0/to/anchor_image/unit",
				"/slides/0/overlays/1/label",
				"/slides/0/overlays/1/link/href",
			},
		},
		{
			name:  "slide source_link",
			slide: `{"source":"Annual report","source_link":{"url":"https://example.com","slide":2,"href":"https://example.com"}}`,
			want:  []string{"/slides/0/source_link/href"},
		},
		{
			name: "working nested containers report nothing",
			slide: gridWithCell(`{"col_span":2,"grid":{"gap":4,"col_gap":2,"columns":[1,2],"vertical_align":"top","rows":[
				{"flex":1,"rule":"below","cells":[
					{"composite":{"split":"bottom","ratio":0.4,
						"text":{"geometry":"rect","fill":{"color":"accent1","alpha":50},"text":{"content":"42%","size":24},"icon":{"name":"star","scale":0.5}},
						"sub_diagram":{"type":"line_chart","data":{"series":[]},"style":{"show_legend":false}}}},
					{"pattern":{"name":"kpi-3up","values":[{"big":"1","small":"a"}],"overrides":{"accent":"accent2"},"cell_overrides":{"0":{"emphasis":"bold"}},"callout":{"text":"x"}}},
					{"layers":[{"name":"ring","frame":{"x":0,"y":0,"w":1,"h":1},"shape":{"geometry":"blockArc","adjustments":{"adj1":1},"link":{"slide":2}}}]},
					{"grid":{"rows":[{"cells":[{"shape":{"geometry":"rect","rotation":90}}]}]}}
				]}
			]}}`),
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unknownKeyPaths(t, tc.slide)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("unknown keys\n got: %s\nwant: %s", strings.Join(got, "\n      "), strings.Join(tc.want, "\n      "))
			}
		})
	}
}

// A nested typo carries the same did-you-mean fix as one at the top of a cell.
func TestUnknownKeys_NestedContainerFix(t *testing.T) {
	raw := json.RawMessage(`{"template":"t","slides":[` + gridWithCell(`{"grid":{"rows":[{"cells":[{"shape":{"geometry":"rect","fil":"accent1"}}]}]}}`) + `]}`)
	warnings := checkInputUnknownKeys(raw)
	if len(warnings) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(warnings), warnings)
	}
	fix := warnings[0].Fix
	if fix == nil || fix.Kind != "rename_field" || fix.Params["from"] != "fil" || fix.Params["to"] != "fill" {
		t.Errorf("fix = %+v, want rename_field fil -> fill", fix)
	}
}

// expand_pattern output is fed straight back into generate, so every key a
// pattern writes into a shape's fill, line or text object must be one the
// writer decodes: an expanded grid reports no unknown key
// (go-slide-creator-hi3qe).
func TestUnknownKeys_ExpandedPatternGridsAreClean(t *testing.T) {
	reg := patterns.Default()
	n := 0
	for _, p := range reg.List() {
		ex, ok := p.(patterns.Exemplar)
		if !ok {
			continue
		}
		values, err := json.Marshal(ex.ExemplarValues())
		if err != nil {
			t.Fatal(err)
		}
		ctx := patterns.ExpandContext{
			SlideWidth:   12192000,
			SlideHeight:  6858000,
			LayoutBounds: patterns.LayoutBounds{X: contentRect.X, Y: contentRect.Y, Width: contentRect.CX, Height: contentRect.CY},
			Theme:        pStyleTheme(),
		}
		grid, _, err := expandPattern(&PatternInput{Name: p.Name(), Values: values}, ctx, reg)
		if err != nil {
			t.Errorf("%s: expand: %v", p.Name(), err)
			continue
		}
		raw, err := json.Marshal(grid)
		if err != nil {
			t.Fatal(err)
		}
		n++
		for _, w := range checkShapeGridUnknownKeys(raw, "/shape_grid") {
			t.Errorf("%s: expanded grid has unknown key %s", p.Name(), w.Path)
		}
	}
	if n == 0 {
		t.Fatal("no pattern exemplar was expanded")
	}
}
