package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/generator"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestGridDiagramConverterPreflight(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{
		ShapeGrid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{
			{Diagram: crowdedNominalDiagram()},
			{Composite: &jsonschema.CompositeInput{SubDiagram: crowdedNominalDiagram()}},
			{Grid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{
				{Diagram: crowdedNominalDiagram()},
			}}}}},
			{Diagram: &types.DiagramSpec{Type: "process_flow"}},
		}}},
		}}}}
	wantPaths := map[string]bool{
		"/slides/0/shape_grid/rows/0/cells/0/diagram":                     false,
		"/slides/0/shape_grid/rows/0/cells/1/composite/sub_diagram":       false,
		"/slides/0/shape_grid/rows/0/cells/2/grid/rows/0/cells/0/diagram": false,
	}
	for _, available := range []bool{false, true} {
		t.Run(fmt.Sprintf("converter_available=%t", available), func(t *testing.T) {
			seen := make(map[string]bool)
			for _, finding := range collectChartDryRenderFindingsWithConverter(input, nil, "", "warn", available) {
				if finding.Message != generator.GridDiagramConverterMissingMessage {
					continue
				}
				if finding.Code != patterns.ErrCodeDiagramRenderFailed || finding.Action != "refuse" {
					t.Errorf("dependency finding is not blocking: %+v", finding)
				}
				seen[finding.Path] = true
			}
			if available && len(seen) != 0 {
				t.Fatalf("converter present: unexpected dependency findings %v", seen)
			}
			if !available {
				for path := range wantPaths {
					if !seen[path] {
						t.Errorf("converter absent: missing finding at %s; got %v", path, seen)
					}
				}
				if len(seen) != len(wantPaths) {
					t.Errorf("converter absent: got dependency findings at %v", seen)
				}
			}
		})
	}
}

func TestGridDiagramConverterCLIDryRun(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{
		Rows: []GridRowInput{{Cells: []*GridCellInput{{Diagram: crowdedNominalDiagram()}}}},
	}}}}
	for _, available := range []bool{false, true} {
		output := &dryRunOutput{Valid: true}
		appendGridConverterPreflight(output, input, available)
		if output.Valid != available {
			t.Errorf("converter available=%t: dry-run valid=%t", available, output.Valid)
		}
		want := 1
		if available {
			want = 0
		}
		if len(output.FitFindings) != want {
			t.Errorf("converter available=%t: dependency findings=%d", available, len(output.FitFindings))
		}
	}
}

func TestGridDiagramReadabilityPreflight(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{
		Bounds: &GridBoundsInput{X: 10, Y: 10, Width: 25, Height: 40},
		Rows:   []GridRowInput{{Cells: []*GridCellInput{{Diagram: crowdedOrgDiagram()}}}},
	}}}}
	for _, tc := range []struct{ mode, floor string }{{"present", "12pt"}, {"read", "10pt"}} {
		t.Run(tc.mode, func(t *testing.T) {
			input.ViewingMode = tc.mode
			findings := collectChartDryRenderFindingsResolved(input, nil, "", "warn", true,
				nil, 12_000_000, 7_000_000)
			for _, finding := range findings {
				if finding.Code != "TEXT_BELOW_READABLE_MIN" {
					continue
				}
				if finding.Path != "/slides/0/shape_grid/rows/0/cells/0/diagram" || finding.Action != "review" || !strings.Contains(finding.Message, tc.floor) {
					t.Errorf("readability preflight has wrong path, action, or floor: %+v", finding)
				}
				return
			}
			t.Errorf("expected physical-size readability preflight finding; got %+v", findings)
		})
	}
}

// crowdedNominalDiagram builds a 40-category bar chart at a render width
// where every named category cannot fit even vertically. Shared by the
// dry-render shape_grid tests below.
func crowdedNominalDiagram() *types.DiagramSpec {
	const n = 40
	cats := make([]any, n)
	vals := make([]any, n)
	for i := 0; i < n; i++ {
		cats[i] = "Category " + string(rune('A'+i%26)) + string(rune('0'+i/26))
		vals[i] = float64(i + 1)
	}
	return &types.DiagramSpec{
		Type:   "bar_chart",
		Width:  400, // narrow to force tick label collisions
		Height: 300,
		Data: map[string]any{
			"categories": cats,
			"series": []any{
				map[string]any{"name": "Series A", "values": vals},
			},
		},
	}
}

func TestDryRenderChartStyleUnresolvedColorFinding(t *testing.T) {
	spec := &types.DiagramSpec{
		Type: "bar_chart", Style: &types.DiagramStyle{
			Colors: []string{"accent9"}, Background: "lt2",
		}, Data: map[string]any{
			"categories": []string{"A"}, "values": []float64{1},
		},
	}
	theme := []types.ThemeColor{{Name: "accent1", RGB: "#123456"}, {Name: "lt1", RGB: "#FFFFFF"}}
	findings := dryRenderSpecToFindings(spec, theme, "", "warn", "/slides/0/content/0/diagram_value")
	var dropped []string
	for _, finding := range findings {
		if finding.Code == "CUSTOM_COLOR_DROPPED" {
			if finding.Action != "info" {
				t.Errorf("dropped color action = %q, want info", finding.Action)
			}
			dropped = append(dropped, finding.Path)
		}
	}
	if len(dropped) != 2 || dropped[0] != "/slides/0/content/0/diagram_value/style/colors/0" || dropped[1] != "/slides/0/content/0/diagram_value/style/background" {
		t.Errorf("dropped color paths = %v", dropped)
	}

	spec.Style.Colors = []string{"accent1"}
	spec.Style.Background = "lt1"
	for _, finding := range dryRenderSpecToFindings(spec, theme, "", "warn", "/valid") {
		if finding.Code == "CUSTOM_COLOR_DROPPED" {
			t.Errorf("valid template color reported dropped: %+v", finding)
		}
	}
	spec.Style.Colors = []string{"accent1", "accent1", "accent1", "accent1", "accent1", "accent1", "accent9"}
	var seventhFound bool
	for _, finding := range dryRenderSpecToFindings(spec, theme, "", "warn", "/seven") {
		if finding.Code == "CUSTOM_COLOR_DROPPED" {
			if finding.Path != "/seven/style/colors/6" {
				t.Errorf("unexpected seventh-color finding path: %+v", finding)
			}
			seventhFound = true
		}
	}
	if !seventhFound {
		t.Error("missing unresolved seventh-color finding")
	}
}

// TestCollectChartDryRenderFindings_CrowdedNominal verifies that a 40-category
// bar chart at a narrow render width surfaces actionable crowding via the
// dry-render path, closing the validate → preview → generate feedback loop
// (acceptance criterion 6 of bead go-slide-creator-0ywh).
func TestCollectChartDryRenderFindings_CrowdedNominal(t *testing.T) {
	const n = 40
	cats := make([]any, n)
	vals := make([]any, n)
	for i := 0; i < n; i++ {
		cats[i] = "Category " + string(rune('A'+i%26)) + string(rune('0'+i/26))
		vals[i] = float64(i + 1)
	}

	input := &PresentationInput{
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{
						Type:          "diagram",
						PlaceholderID: "body",
						DiagramValue: &types.DiagramSpec{
							Type:   "bar_chart",
							Width:  400, // narrow to force tick label collisions
							Height: 300,
							Data: map[string]any{
								"categories": cats,
								"series": []any{
									map[string]any{"name": "Series A", "values": vals},
								},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding for 40-cat narrow bar chart, got none")
	}

	var sawCrowding bool
	for _, f := range findings {
		if f.Code == "chart.tick_thinned" && strings.Contains(f.Path, "/x_axis/labels") {
			t.Errorf("nominal categories must not be thinned: %+v", f)
		}
		if f.Code == "chart.capacity_exceeded" {
			sawCrowding = true
			if !strings.HasPrefix(f.Path, "/slides/0/content/0/diagram_value") {
				t.Errorf("capacity_exceeded path = %q; want prefix /slides/0/content/0/diagram_value", f.Path)
			}
			if f.Action != "shrink_or_split" {
				t.Errorf("capacity_exceeded action = %q, want shrink_or_split", f.Action)
			}
		}
	}
	if !sawCrowding {
		// Helpful diagnostics when the geometry changes upstream.
		codes := make([]string, 0, len(findings))
		for _, f := range findings {
			codes = append(codes, f.Code)
		}
		t.Errorf("expected chart.capacity_exceeded in dry-render findings; got codes=%v", codes)
	}
}

func TestCollectChartDryRenderFindings_EllipsizedCategoryAction(t *testing.T) {
	categories := make([]any, 14)
	values := make([]any, 14)
	for i := range categories {
		categories[i] = fmt.Sprintf("Category number %d with a long name", i+1)
		values[i] = float64(i + 1)
	}
	input := &PresentationInput{Slides: []SlideInput{{Content: []ContentInput{{
		Type: "diagram", DiagramValue: &types.DiagramSpec{
			Type: "bar_chart", Width: 800, Height: 360,
			Data: map[string]any{"categories": categories, "series": []any{map[string]any{"name": "S", "values": values}}},
		},
	}}}}}
	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	for _, f := range findings {
		if f.Code == "chart.label_ellipsized" && f.Action == "shrink_or_split" {
			return
		}
	}
	t.Errorf("majority-ellipsized category names must reach fit report as shrink_or_split; got %+v", findings)
}

// TestCollectChartDryRenderFindings_EmptyInput is a regression guard: passing
// no slides (or no chart/diagram content) must return nil without panicking.
func TestCollectChartDryRenderFindings_EmptyInput(t *testing.T) {
	if got := collectChartDryRenderFindings(nil, nil, "", "warn"); got != nil {
		t.Errorf("nil input: got %v, want nil", got)
	}
	empty := &PresentationInput{}
	if got := collectChartDryRenderFindings(empty, nil, "", "warn"); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
	textOnly := &PresentationInput{
		Slides: []SlideInput{
			{
				Content: []ContentInput{
					{Type: "text", PlaceholderID: "title", TextValue: ptr("Hello")},
				},
			},
		},
	}
	if got := collectChartDryRenderFindings(textOnly, nil, "", "warn"); got != nil {
		t.Errorf("text-only input: got %d findings, want 0", len(got))
	}
}

func ptr[T any](v T) *T { return &v }

func vennFrameFixture() *types.DiagramSpec {
	circles := []any{
		map[string]any{"label": "Customer needs", "items": []any{"Faster onboarding", "Transparent pricing"}},
		map[string]any{"label": "Our capabilities", "items": []any{"Platform APIs", "Global support"}},
		map[string]any{"label": "Competitor gaps", "items": []any{"No self-service", "Slow releases"}},
	}
	return &types.DiagramSpec{Type: "venn", Data: map[string]any{"circles": circles}}
}

func TestChartDryRenderUsesResolvedPlaceholderFrame(t *testing.T) {
	spec := vennFrameFixture()
	input := &PresentationInput{Slides: []SlideInput{{
		LayoutID: "slideLayout1",
		Content:  []ContentInput{{Type: "diagram", PlaceholderID: "body", DiagramValue: spec}},
	}}}
	layout := types.LayoutMetadata{ID: "slideLayout1", Name: "One Content", Placeholders: []types.PlaceholderInfo{{
		ID: "body", Type: types.PlaceholderBody,
		Bounds: types.BoundingBox{Width: 747 * int64(types.EMUPerPoint), Height: 220 * int64(types.EMUPerPoint)},
	}}}
	narrow := collectChartDryRenderFindingsResolved(input, nil, "", "warn", true, []types.LayoutMetadata{layout}, 0, 0)
	var dropped bool
	for _, f := range narrow {
		if f.Code == "diagram.items_dropped" {
			dropped = true
			if f.Action != "refuse" {
				t.Errorf("lost Venn items action = %q, want refuse", f.Action)
			}
			if !strings.HasPrefix(f.Path, "/slides/0/content/0/diagram_value") {
				t.Errorf("dropped-items path = %q", f.Path)
			}
		}
	}
	if !dropped {
		t.Fatalf("narrow template frame must reveal dropped Venn items: %+v", narrow)
	}
	layout.Placeholders[0].Bounds = types.BoundingBox{Width: 828 * int64(types.EMUPerPoint), Height: 342 * int64(types.EMUPerPoint)}
	wide := collectChartDryRenderFindingsResolved(input, nil, "", "warn", true, []types.LayoutMetadata{layout}, 0, 0)
	for _, f := range wide {
		if f.Code == "diagram.items_dropped" {
			t.Errorf("wider frame unexpectedly drops Venn items: %+v", f)
		}
	}
}

func TestChartDryRenderUsesResolvedGridCellFrame(t *testing.T) {
	spec := vennFrameFixture()
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 75, Height: 40},
		Rows:   []GridRowInput{{Cells: []*GridCellInput{{Diagram: spec}}}},
	}}}}
	findings := collectChartDryRenderFindingsResolved(input, nil, "", "warn", true, nil,
		960*int64(types.EMUPerPoint), 540*int64(types.EMUPerPoint))
	for _, f := range findings {
		if f.Code == "diagram.items_dropped" && strings.Contains(f.Path, "/shape_grid/rows/0/cells/0/diagram") {
			return
		}
	}
	t.Fatalf("narrow resolved grid cell must reveal dropped Venn items: %+v", findings)
}

func TestVennDroppedItemsDependsOnBundledTemplateFrame(t *testing.T) {
	for _, tc := range []struct {
		name        string
		layoutID    string
		wantDropped bool
	}{
		{name: "modern-yellow", layoutID: "slideLayout4", wantDropped: true},
		{name: "midnight-blue", layoutID: "slideLayout2", wantDropped: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layouts, theme, slideWidth, slideHeight := fitReportGeometry(tc.name, "../../templates")
			if len(layouts) == 0 || theme == nil {
				t.Fatalf("could not resolve %s template geometry", tc.name)
			}
			input := &PresentationInput{Slides: []SlideInput{{
				LayoutID: tc.layoutID,
				Content:  []ContentInput{{Type: "diagram", PlaceholderID: "body", DiagramValue: vennFrameFixture()}},
			}}}
			var dropped bool
			for _, f := range collectFitFindings(input, layouts, slideWidth, slideHeight, theme) {
				if f.Code == "diagram.items_dropped" {
					dropped = true
					if f.Action != "refuse" {
						t.Errorf("lost Venn items action = %q, want refuse", f.Action)
					}
				}
			}
			if dropped != tc.wantDropped {
				t.Errorf("%s dropped items = %t, want %t", tc.name, dropped, tc.wantDropped)
			}
		})
	}
}

func TestChartDryRenderUsesCompositeAndNestedGridFrames(t *testing.T) {
	input := &PresentationInput{Slides: []SlideInput{{ShapeGrid: &ShapeGridInput{
		Bounds: &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 75, Height: 40},
		Rows: []GridRowInput{{Cells: []*GridCellInput{
			{Composite: &jsonschema.CompositeInput{
				Text: &jsonschema.ShapeSpecInput{Geometry: "rect"}, SubDiagram: vennFrameFixture(),
			}},
			{Grid: &ShapeGridInput{Rows: []GridRowInput{{Cells: []*GridCellInput{{Diagram: vennFrameFixture()}}}}}},
		}}},
	}}}}
	findings := collectChartDryRenderFindingsResolved(input, nil, "", "warn", true, nil,
		960*int64(types.EMUPerPoint), 540*int64(types.EMUPerPoint))
	wantPaths := map[string]bool{
		"/shape_grid/rows/0/cells/0/composite/sub_diagram":       false,
		"/shape_grid/rows/0/cells/1/grid/rows/0/cells/0/diagram": false,
	}
	for _, f := range findings {
		if f.Code != "diagram.items_dropped" {
			continue
		}
		for path := range wantPaths {
			if strings.Contains(f.Path, path) {
				wantPaths[path] = true
			}
		}
	}
	for path, found := range wantPaths {
		if !found {
			t.Errorf("missing dropped-item finding at %s: %+v", path, findings)
		}
	}
}

// TestCollectChartDryRenderFindings_ShapeGridDiagram verifies that a diagram
// embedded directly in a top-level shape_grid cell is dry-rendered and that the
// finding path identifies the owning cell using the slidepath convention
// (go-slide-creator-kzzl).
func TestCollectChartDryRenderFindings_ShapeGridDiagram(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Diagram: crowdedNominalDiagram()},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/diagram"
	assertCrowdedNominalAt(t, findings, wantPath)
}

// TestCollectChartDryRenderFindings_CompositeSubDiagram verifies that the
// sub_diagram of a composite cell is traversed and that the finding path points
// at the composite/sub_diagram subfield.
func TestCollectChartDryRenderFindings_CompositeSubDiagram(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Composite: &jsonschema.CompositeInput{
									SubDiagram: crowdedNominalDiagram(),
								}},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/composite/sub_diagram"
	assertCrowdedNominalAt(t, findings, wantPath)
}

// TestCollectChartDryRenderFindings_NestedGridDiagram verifies that a diagram
// inside a recursively nested sub-grid is traversed and that the finding path
// threads through the nesting (.../cells/{c}/grid/rows/{r}/cells/{c}/diagram).
func TestCollectChartDryRenderFindings_NestedGridDiagram(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{
			{
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{
							Cells: []*GridCellInput{
								{Grid: &ShapeGridInput{
									Rows: []GridRowInput{
										{
											Cells: []*GridCellInput{
												nil, // exercise the nil-cell skip
												{Diagram: crowdedNominalDiagram()},
											},
										},
									},
								}},
							},
						},
					},
				},
			},
		},
	}

	findings := collectChartDryRenderFindings(input, nil, "", "warn")
	const wantPath = "/slides/0/shape_grid/rows/0/cells/0/grid/rows/0/cells/1/diagram"
	assertCrowdedNominalAt(t, findings, wantPath)
}

// assertCrowdedNominalAt verifies both the traversal path and action contract.
func assertCrowdedNominalAt(t *testing.T, findings []patterns.FitFinding, wantPath string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == "chart.capacity_exceeded" && f.Action == "shrink_or_split" && strings.Contains(f.Path, wantPath) {
			return
		}
	}
	codes := make([]string, 0, len(findings))
	for _, f := range findings {
		codes = append(codes, f.Code+"@"+f.Path)
	}
	t.Errorf("expected actionable chart.capacity_exceeded at path containing %q; got %v", wantPath, codes)
}

// go-slide-creator-r87g. The preflight dry-render asked svggen about every
// diagram type, including the half of the catalogue internal/generator draws as
// OOXML shapes. svggen answered "unknown diagram type" and validate reported a
// REFUSE saying the deck would render a grey "Data unavailable" placeholder —
// for diagrams that render perfectly. validate is the precondition gate
// SKILL.md tells agents to trust, so a false refuse there is worse than silence.
func TestDryRender_NativeDiagramsAreNotSvggenBusiness(t *testing.T) {
	for _, typ := range []string{"swot", "business_model_canvas", "value_chain", "heatmap", "panel_layout", "pyramid", "kpi_dashboard"} {
		spec := &types.DiagramSpec{Type: typ, Data: map[string]any{"whatever": "the native builder reads"}}
		if got := dryRenderSpecToFindings(spec, nil, "", "warn", "/slides/0/content/0/diagram_value"); len(got) != 0 {
			t.Errorf("%s: native diagram reported %d finding(s): %+v", typ, len(got), got)
		}
	}
	// A type NEITHER renderer owns still refuses: that one really does become a
	// placeholder.
	spec := &types.DiagramSpec{Type: "sankey_of_the_mind", Data: map[string]any{"a": 1}}
	got := dryRenderSpecToFindings(spec, nil, "", "warn", "/slides/0/content/0/diagram_value")
	if len(got) != 1 || got[0].Action != "refuse" {
		t.Fatalf("an unknown type must still refuse, got %+v", got)
	}
}

func TestDryRender_NativeDiagramGeometryFindings(t *testing.T) {
	items := make([]any, 9)
	for i := range items {
		items[i] = strings.Repeat("Long architecture capability ", 3)
	}
	spec := &types.DiagramSpec{Type: "business_model_canvas", Data: map[string]any{
		"key_partners": items, "key_activities": items, "value_propositions": items,
	}}
	got := dryRenderSpecToFindings(spec, nil, "Arial", "warn", "/slides/0/content/0/diagram_value")
	if len(got) == 0 || got[0].Code != "diagram.text_overlap" || !strings.Contains(got[0].Path, "/data/key_partners") {
		t.Fatalf("native BMC findings = %+v", got)
	}
}

// A dry run has to dry-run what actually happens: the render path normalizes an
// authored chart shorthand before svggen sees it, and the dry run used to skip
// that step and validate the shorthand instead. Four shipped example decks
// carried refuse-class findings for charts that render correctly.
func TestDryRender_NormalizesAuthoredChartShorthand(t *testing.T) {
	tests := []struct {
		name  string
		chart *types.ChartSpec //nolint:staticcheck // the authored chart shape
	}{
		{
			name: "waterfall authored as a flat map",
			chart: &types.ChartSpec{ //nolint:staticcheck
				Type:      "waterfall",
				Data:      map[string]any{"Revenue": 100.0, "Costs": -40.0, "Profit": 60.0},
				DataOrder: []string{"Revenue", "Costs", "Profit"},
			},
		},
		{
			name: "radar authored as a flat map",
			chart: &types.ChartSpec{ //nolint:staticcheck
				Type:      "radar",
				Data:      map[string]any{"Architecture": 4.0, "Security": 3.0, "Cost": 5.0},
				DataOrder: []string{"Architecture", "Security", "Cost"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := chartValueToDiagramSpec(tt.chart)
			if spec == nil {
				t.Fatal("no spec")
			}
			for _, f := range dryRenderSpecToFindings(spec, nil, "", "warn", "/slides/0/content/0/chart_value") {
				if f.Action == "refuse" {
					t.Errorf("a chart the renderer draws was refused: %s", f.Message)
				}
			}
		})
	}
}
