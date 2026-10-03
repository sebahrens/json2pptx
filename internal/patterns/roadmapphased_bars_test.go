package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func roadmapBarsValues() *RoadmapPhasedValues {
	return &RoadmapPhasedValues{
		Phases:       []string{"Q1", "Q2", "Q3", "Q4"},
		CurrentPhase: "Q2",
		Workstreams: []RoadmapWorkstream{
			{Name: "Platform", Bars: []RoadmapBar{
				{Label: "Auth rewrite", Start: "Q1", End: "Q2"},
				{Label: "API v2", Start: "Q2", End: "Q4"},
				{Label: "GA", Start: "Q4", Milestone: true},
			}},
			{Name: "Data", Bars: []RoadmapBar{
				{Label: "Pipeline v2", Start: "Q1"},
				{Label: "ML models", Start: "q2 ", Span: 2},
			}},
		},
	}
}

// roadmapBarCell finds the cell whose text carries label, and the grid column
// it starts in.
func roadmapBarCell(t *testing.T, grid *jsonschema.ShapeGridInput, label string) (cell *jsonschema.GridCellInput, row, col int) {
	t.Helper()
	occupied := map[[2]int]bool{}
	for ri, r := range grid.Rows {
		c := 0
		for _, gc := range r.Cells {
			for occupied[[2]int{ri, c}] {
				c++
			}
			span := 1
			if gc != nil && gc.ColSpan > 1 {
				span = gc.ColSpan
			}
			if gc != nil && gc.RowSpan > 1 {
				for dr := 1; dr < gc.RowSpan; dr++ {
					occupied[[2]int{ri + dr, c}] = true
				}
			}
			if gc != nil && gc.Shape != nil && strings.Contains(string(gc.Shape.Text), label) {
				return gc, ri, c
			}
			c += span
		}
	}
	t.Fatalf("no cell carries %q", label)
	return nil, 0, 0
}

// go-slide-creator-4a0sm: an item with a start and an end period is one bar
// across those periods; overlapping bars of a workstream stack in lanes; a
// milestone is a marker; the current period's header is filled.
func TestRoadmapPhased_BarsSpanPeriods(t *testing.T) {
	p, _ := Default().Get("roadmap-phased")
	vals := roadmapBarsValues()
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	// Header, Platform's two lanes, a spacer, Data's one lane.
	if got := len(grid.Rows); got != 5 {
		t.Fatalf("rows = %d, want header + 2 lanes + spacer + 1 lane", got)
	}
	auth, authRow, authCol := roadmapBarCell(t, grid, "Auth rewrite")
	api, apiRow, apiCol := roadmapBarCell(t, grid, "API v2")
	if auth.ColSpan != 2 || authCol != 1 {
		t.Errorf("Auth rewrite: col %d span %d, want Q1-Q2 (col 1, span 2)", authCol, auth.ColSpan)
	}
	if api.ColSpan != 3 || apiCol != 2 {
		t.Errorf("API v2: col %d span %d, want Q2-Q4 (col 2, span 3)", apiCol, api.ColSpan)
	}
	if authRow == apiRow {
		t.Errorf("Auth rewrite and API v2 share Q2 and row %d; overlapping bars must stack", authRow)
	}
	// The milestone fits beside Auth rewrite in the first lane: no third lane.
	ga, gaRow, gaCol := roadmapBarCell(t, grid, "GA")
	if gaRow != authRow || gaCol != 4 || string(ga.Shape.Fill) != `"none"` || !strings.Contains(string(ga.Shape.Text), roadmapMilestoneGlyph) {
		t.Errorf("milestone: row %d col %d fill %s text %s, want an unfilled marker in Q4 of the first lane", gaRow, gaCol, ga.Shape.Fill, ga.Shape.Text)
	}
	label, _, _ := roadmapBarCell(t, grid, "Platform")
	if label.RowSpan != 2 {
		t.Errorf("Platform label row_span = %d, want its 2 lanes", label.RowSpan)
	}
	// span and a case-insensitive start resolve like end.
	ml, _, mlCol := roadmapBarCell(t, grid, "ML models")
	if ml.ColSpan != 2 || mlCol != 2 {
		t.Errorf("ML models: col %d span %d, want Q2-Q3", mlCol, ml.ColSpan)
	}

	// Time axis: every period header carries the axis segment except the
	// current one, which is the filled header.
	for i, c := range grid.Rows[0].Cells[1:] {
		filled := string(c.Shape.Fill) == `"accent1"`
		if filled != (i == 1) || (c.AccentBar == nil) != (i == 1) {
			t.Errorf("header %d: fill %s accent bar %+v; want only Q2 filled, the others over an axis segment", i, c.Shape.Fill, c.AccentBar)
		}
	}
	// Lanes are content-sized, not stretched over the slide.
	if grid.VerticalAlign != GridVerticalAlignDefault || grid.Rows[1].MaxHeight <= 0 {
		t.Errorf("vertical_align %q, lane max_height %.0f; want content-sized lanes", grid.VerticalAlign, grid.Rows[1].MaxHeight)
	}
}

// go-slide-creator-4a0sm: one-item-per-period input still renders, as
// one-period bars; layout "grid" keeps the legacy table.
func TestRoadmapPhased_ItemsRenderAsOnePeriodBars(t *testing.T) {
	p, _ := Default().Get("roadmap-phased")
	vals := &RoadmapPhasedValues{
		Phases: []string{"Q1", "Q2", "Q3"},
		Workstreams: []RoadmapWorkstream{
			{Name: "Platform", Items: []string{"Auth", "", "Scale"}},
			{Name: "Frontend", Items: []string{"Design", "Build", "Polish"}},
		},
	}
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got := len(grid.Rows); got != 4 {
		t.Fatalf("rows = %d, want header + lane + spacer + lane", got)
	}
	scale, row, col := roadmapBarCell(t, grid, "Scale")
	if row != 1 || col != 3 || scale.ColSpan > 1 {
		t.Errorf("Scale: row %d col %d span %d, want a one-period bar in Q3 of the first lane", row, col, scale.ColSpan)
	}
	// cell_overrides indices keep the one-slot-per-period scheme.
	co := map[int]any{3 + 1 + 2: &RoadmapPhasedCellOverride{AccentBar: true}}
	marked, err := p.Expand(fullThemeCtx(), vals, nil, co)
	if err != nil {
		t.Fatal(err)
	}
	if c, _, _ := roadmapBarCell(t, marked, "Scale"); c.AccentBar == nil {
		t.Error("cell_overrides[6] did not reach Platform's third item")
	}

	legacy, err := p.Expand(fullThemeCtx(), vals, &RoadmapPhasedOverrides{Layout: "grid"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy.Rows) != 3 || len(legacy.Rows[1].Cells) != 4 || legacy.Rows[1].MaxHeight != 0 {
		t.Errorf("layout grid: %d rows, %d cells in row 1; want the 3-row table of full-height tiles", len(legacy.Rows), len(legacy.Rows[1].Cells))
	}
}

func TestRoadmapPhased_BarValidation(t *testing.T) {
	p, _ := Default().Get("roadmap-phased")
	for name, tc := range map[string]struct {
		edit func(*RoadmapPhasedValues)
		ovr  *RoadmapPhasedOverrides
		want string
	}{
		"unknown start":       {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[0].Start = "Q9" }, nil, "bars[0].start"},
		"missing start":       {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[0].Start = "" }, nil, "bars[0].start"},
		"unknown end":         {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[0].End = "Q9" }, nil, "bars[0].end"},
		"end before start":    {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[1].End = "Q1" }, nil, "before it starts"},
		"end and span":        {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[0].Span = 2 }, nil, "more than once"},
		"span past the axis":  {func(v *RoadmapPhasedValues) { v.Workstreams[1].Bars[1].Span = 4 }, nil, "past the last"},
		"milestone with end":  {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[2].End = "Q4" }, nil, "milestone neither"},
		"bars and items":      {func(v *RoadmapPhasedValues) { v.Workstreams[0].Items = []string{"a", "b", "c", "d"} }, nil, "both bars and items"},
		"unknown current":     {func(v *RoadmapPhasedValues) { v.CurrentPhase = "Q7" }, nil, "current_phase"},
		"grid layout of bars": {func(*RoadmapPhasedValues) {}, &RoadmapPhasedOverrides{Layout: "grid"}, `layout "grid"`},
		"unknown layout":      {func(*RoadmapPhasedValues) {}, &RoadmapPhasedOverrides{Layout: "gantt"}, "overrides.layout"},
		"empty label":         {func(v *RoadmapPhasedValues) { v.Workstreams[0].Bars[0].Label = " " }, nil, "bars[0].label"},
	} {
		vals := roadmapBarsValues()
		tc.edit(vals)
		var ovr any
		if tc.ovr != nil {
			ovr = tc.ovr
		}
		if err := p.Validate(vals, ovr, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one naming %q", name, err, tc.want)
		}
	}
}

// A bar whose label cannot be written readably in its extent is reported by
// name; a roadmap that fits reports nothing.
func TestRoadmapPhased_BarWarnings(t *testing.T) {
	pat := &roadmapPhased{}
	ctx := fullThemeCtx()
	if got := pat.PostExpandWarnings(ctx, roadmapBarsValues(), nil); len(got) != 0 {
		t.Fatalf("fitting roadmap warned: %v", got)
	}
	stacked := func(perStream int) *RoadmapPhasedValues {
		vals := roadmapBarsValues()
		vals.Workstreams = nil
		for i := 0; i < 6; i++ {
			ws := RoadmapWorkstream{Name: "Stream"}
			for j := 0; j < perStream; j++ {
				ws.Bars = append(ws.Bars, RoadmapBar{Label: "Short", Start: "Q1"})
			}
			vals.Workstreams = append(vals.Workstreams, ws)
		}
		return vals
	}
	// Twelve lanes are squeezed to about a line each: short labels still
	// read, a label that wraps does not.
	vals := stacked(2)
	if got := pat.PostExpandWarnings(ctx, vals, nil); len(got) != 0 {
		t.Fatalf("12 one-line lanes warned: %v", got)
	}
	vals.Workstreams[1].Bars[0].Label = "A long activity name that wraps over several lines in one period"
	got := pat.PostExpandWarnings(ctx, vals, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong) || !strings.Contains(got[0], "workstreams[1].bars[0].label") {
		t.Fatalf("warnings = %v, want BODY_TOO_LONG naming workstreams[1].bars[0].label", got)
	}
	// Thirty lanes hold no readable bar at all: one warning, not thirty.
	got = pat.PostExpandWarnings(ctx, stacked(5), nil)
	if len(got) != 1 || !strings.Contains(got[0], "stacks 30 lanes") {
		t.Fatalf("warnings = %v, want one BODY_TOO_LONG for the lane count", got)
	}
}

// The schema advertises bars, current_phase and the layout override, and the
// exemplar round-trips through it.
func TestRoadmapPhased_SchemaCarriesBars(t *testing.T) {
	pat := &roadmapPhased{}
	data, err := json.Marshal(pat.Schema())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"bars"`, `"current_phase"`, `"milestone"`, `"span"`, `"layout"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema omits %s", want)
		}
	}
	ex, _ := json.Marshal(pat.ExemplarValues())
	vals := pat.NewValues()
	if err := json.Unmarshal(ex, vals); err != nil {
		t.Fatal(err)
	}
	if err := pat.Validate(vals, nil, nil); err != nil {
		t.Errorf("exemplar does not validate: %v", err)
	}
}
