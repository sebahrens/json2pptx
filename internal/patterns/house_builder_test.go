package patterns

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func housePillars(n int) []StrategyHousePillar {
	all := []StrategyHousePillar{
		{Title: "Workflow redesign", Body: []string{"Map work end to end", "Set human checkpoints", "Measure cycle time"}},
		{Title: "Agent platform", Body: []string{"Shared tools and data access", "Evaluation and guardrails"}},
		{Title: "People and skills", Body: []string{"Train process owners"}},
		{Title: "Risk and controls", Body: []string{"Audit every production agent"}},
		{Title: "Partner reach", Body: []string{"Marketplace live in 12 markets", "Joint roadmaps"}},
	}
	return all[:n]
}

func expandHouse(t *testing.T, ctx ExpandContext, v *StrategyHouseValues, cos map[int]any) *jsonschema.ShapeGridInput {
	t.Helper()
	p, _ := Default().Get("strategy-house")
	if err := p.Validate(v, nil, cos); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ctx, v, nil, cos)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}

var houseSlideCtx = ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}

// TestStrategyHouseRoofIsAGable pins go-slide-creator-vlef3: for 3, 4 and 5
// pillars, with and without badges, the top of the house is one gable
// pentagon spanning the pillars — not a strip — whose rise is at least the
// minimum pitch, and the badges sit inside it with the objective.
func TestStrategyHouseRoofIsAGable(t *testing.T) {
	fullW, _ := contentAreaPt(houseSlideCtx)
	for n := 3; n <= 5; n++ {
		for _, badges := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d pillars badges=%t", n, badges), func(t *testing.T) {
				v := &StrategyHouseValues{Objective: "Turn AI from pilots into a repeatable operating capability", Pillars: housePillars(n), Foundation: "Governance for every production agent"}
				if badges {
					v.RoofBadges = []string{"Vision 2027", "Mission"}
				}
				grid := expandHouse(t, houseSlideCtx, v, nil)
				if got := len(grid.Rows); got != 3 {
					t.Fatalf("rows = %d, want 3 (roof, pillars, foundation): badges live in the roof, not in a strip", got)
				}
				roof := grid.Rows[0].Cells[0]
				if roof.Shape.Geometry != "upArrow" || roof.Shape.Adjustments["adj1"] != 100000 {
					t.Fatalf("roof geometry = %s %v, want the full-width upArrow gable", roof.Shape.Geometry, roof.Shape.Adjustments)
				}
				if roof.ColSpan != n {
					t.Errorf("roof spans %d columns, want %d", roof.ColSpan, n)
				}
				roofH := grid.Rows[0].MinHeight
				rise := float64(roof.Shape.Adjustments["adj2"]) / 100000 * roofH
				if rise < fullW*houseRoofMinPitch-1 {
					t.Errorf("gable rises %.0fpt over %.0fpt, flatter than the minimum pitch", rise, fullW)
				}
				// The eaves band under the gable holds the text at its
				// written size.
				need := writtenFitHeightPt(houseSlideCtx.themeFonts(), roof.Shape.Text, fullW, 0)
				if band := roofH - rise; band < need-1 {
					t.Errorf("eaves band is %.0fpt but the roof text needs %.0fpt", band, need)
				}
				text := string(roof.Shape.Text)
				if !strings.Contains(text, "Turn AI from pilots") {
					t.Errorf("objective missing from the roof: %s", text)
				}
				if badges != strings.Contains(text, "Vision 2027") {
					t.Errorf("badges in roof = %t, want %t: %s", !badges, badges, text)
				}
			})
		}
	}
}

// TestStrategyHouseLevelsFollowContent pins go-slide-creator-bjxb9: a beam,
// four pillars and two foundation levels (one split into three cells) each
// get their own row, sized from their text.
func TestStrategyHouseLevelsFollowContent(t *testing.T) {
	var v StrategyHouseValues
	raw := `{"objective": "Become the trusted platform", "beam": "One operating model in every market",
		"pillars": [{"title": "A", "body": ["One", "Two", "Three"]}, {"title": "B", "body": ["One"]}, {"title": "C"}, {"title": "D", "body": ["One", "Two"]}],
		"foundation": ["Governance and cost tracking", ["People", "Data platform", "Controls"]]}`
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.FoundationLayers) != 2 || len(v.FoundationLayers[1]) != 3 {
		t.Fatalf("foundation levels = %v", v.FoundationLayers)
	}
	// The values round-trip in the shape they were authored in.
	back, err := json.Marshal(&v)
	if err != nil || !strings.Contains(string(back), `"foundation":["Governance and cost tracking",["People","Data platform","Controls"]]`) {
		t.Fatalf("round trip = %s (%v)", back, err)
	}

	grid := expandHouse(t, houseSlideCtx, &v, nil)
	if string(grid.Columns) != "12" {
		t.Errorf("columns = %s, want 12 (4 pillars and 3 cells share a grid)", grid.Columns)
	}
	wantCells := []int{1, 1, 4, 1, 3} // roof, beam, pillars, band, cells
	if len(grid.Rows) != len(wantCells) {
		t.Fatalf("rows = %d, want %d", len(grid.Rows), len(wantCells))
	}
	_, areaH := contentAreaPt(houseSlideCtx)
	total := float64(len(grid.Rows)-1) * grid.RowGap
	for i, row := range grid.Rows {
		if len(row.Cells) != wantCells[i] {
			t.Errorf("row %d has %d cells, want %d", i, len(row.Cells), wantCells[i])
		}
		if row.MinHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Errorf("row %d is not pinned to its content: min %.0f max %.0f", i, row.MinHeight, row.MaxHeight)
		}
		total += row.MinHeight
	}
	if total > areaH+1 {
		t.Errorf("house is %.0fpt tall in a %.0fpt region", total, areaH)
	}
	if !strings.Contains(string(grid.Rows[1].Cells[0].Shape.Text), "One operating model") {
		t.Errorf("row 1 is not the beam: %s", grid.Rows[1].Cells[0].Shape.Text)
	}
	if grid.Rows[4].Cells[1].ColSpan != 4 || !strings.Contains(string(grid.Rows[4].Cells[1].Shape.Text), "Data platform") {
		t.Errorf("foundation cell 1 = span %d %s", grid.Rows[4].Cells[1].ColSpan, grid.Rows[4].Cells[1].Shape.Text)
	}

	// Heights come from the content: more bullets, taller pillars.
	short := v
	short.Pillars = []StrategyHousePillar{{Title: "A", Body: []string{"One"}}, {Title: "B"}, {Title: "C"}, {Title: "D"}}
	if s := expandHouse(t, houseSlideCtx, &short, nil); s.Rows[2].MinHeight >= grid.Rows[2].MinHeight {
		t.Errorf("one-bullet pillars are %.0fpt, three-bullet pillars %.0fpt: the row is not content-sized", s.Rows[2].MinHeight, grid.Rows[2].MinHeight)
	}

	// A pillar without bullets keeps its title on its neighbours' line.
	if text := string(grid.Rows[2].Cells[2].Shape.Text); !strings.Contains(text, `"vertical_align":"t"`) {
		t.Errorf("empty pillar is not top-aligned beside bulleted ones: %s", text)
	}
}

// TestStrategyHouseRejectsUnreadableShapes: more levels or cells than stay
// readable are refused with the counts.
func TestStrategyHouseRejectsUnreadableShapes(t *testing.T) {
	p, _ := Default().Get("strategy-house")
	base := func() *StrategyHouseValues {
		return &StrategyHouseValues{Objective: "Grow", Pillars: housePillars(5)}
	}
	cases := []struct {
		name   string
		layers []StrategyHouseLayer
		want   string
	}{
		{"four levels", []StrategyHouseLayer{{"A"}, {"B"}, {"C"}, {"D"}}, "at most 3 items, got 4"},
		{"six cells", []StrategyHouseLayer{{"A", "B", "C", "D", "E", "F"}}, "at most 5 items, got 6"},
		{"no common grid", []StrategyHouseLayer{{"A", "B", "C"}, {"A", "B", "C", "D"}}, "5 pillars over foundation levels of 3 and 4 cells"},
		{"empty cell", []StrategyHouseLayer{{"A", " "}}, "foundation[0][1] is required"},
		{"long cell", []StrategyHouseLayer{{"A", strings.Repeat("x", 41)}}, "foundation[0][1] exceeds maxLength 40"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := base()
			v.FoundationLayers = tc.layers
			err := p.Validate(v, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want %q", err, tc.want)
			}
		})
	}
	v := base()
	if err := json.Unmarshal([]byte(`{"objective": "Grow", "pillars": [], "foundation": [{"label": "x"}]}`), v); err == nil {
		t.Error("an object foundation level decoded; want a shape error")
	}
}

// TestStrategyHouseCellOverrideIndices: the classic house keeps its indices
// (N+1 foundation, N+2 badges); extra foundation cells and the beam follow.
func TestStrategyHouseCellOverrideIndices(t *testing.T) {
	classic := &StrategyHouseValues{Objective: "Grow", Pillars: housePillars(3), Foundation: "Base", RoofBadges: []string{"Vision"}}
	if c := classic.cells(); c.foundation[0] != 4 || c.badges != 5 || c.beam != -1 || c.total != 6 {
		t.Fatalf("classic cells = %+v", c)
	}
	grid := expandHouse(t, houseSlideCtx, classic, map[int]any{
		4: &StrategyHouseCellOverride{Color: "accent2"},
		5: &StrategyHouseCellOverride{FontSize: 13},
	})
	if !strings.Contains(string(grid.Rows[2].Cells[0].Shape.Text), "accent2") {
		t.Errorf("override 4 did not reach the foundation: %s", grid.Rows[2].Cells[0].Shape.Text)
	}
	var roof struct {
		Paragraphs []struct {
			Content string  `json:"content"`
			Size    float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &roof); err != nil || len(roof.Paragraphs) != 2 {
		t.Fatalf("roof text = %s (%v)", grid.Rows[0].Cells[0].Shape.Text, err)
	}
	if roof.Paragraphs[0].Size != 13 || roof.Paragraphs[1].Size == 13 {
		t.Errorf("override 5 must size the badge line only: %+v", roof.Paragraphs)
	}

	layered := &StrategyHouseValues{Objective: "Grow", Beam: "Shared", Pillars: housePillars(4),
		FoundationLayers: []StrategyHouseLayer{{"Band"}, {"A", "B", "C"}}}
	if c := layered.cells(); c.foundation[0] != 5 || c.foundation[1] != 6 || c.beam != 9 || c.badges != -1 || c.total != 10 {
		t.Fatalf("layered cells = %+v", c)
	}
	p, _ := Default().Get("strategy-house")
	if err := p.Validate(layered, nil, map[int]any{10: &StrategyHouseCellOverride{Color: "accent2"}}); err == nil {
		t.Error("cell_overrides[10] accepted on a 10-cell house")
	}
}

// TestStrategyHouseShapeWarnings pins go-slide-creator-qad87's advisory.
func TestStrategyHouseShapeWarnings(t *testing.T) {
	p := &strategyHouse{}
	warn := func(v *StrategyHouseValues) string {
		return strings.Join(p.PostExpandWarnings(houseSlideCtx, v, nil), "\n")
	}
	joined := &StrategyHouseValues{Objective: "Grow", Pillars: housePillars(3), Foundation: "People · Technology · Data"}
	if got := warn(joined); !strings.Contains(got, ErrCodeHouseShapeForced+": strategy-house foundation joins 3 separate items") ||
		!strings.Contains(got, `["People","Technology","Data"]`) {
		t.Errorf("joined foundation: %q", got)
	}
	for _, ok := range []string{"Governance, security and cost tracking for every production agent", "People and data", "People, technology"} {
		if got := warn(&StrategyHouseValues{Objective: "Grow", Pillars: housePillars(3), Foundation: ok}); strings.Contains(got, ErrCodeHouseShapeForced) {
			t.Errorf("%q is one statement, got %q", ok, got)
		}
	}
	split := &StrategyHouseValues{Objective: "Grow", Pillars: housePillars(3), FoundationLayers: []StrategyHouseLayer{{"People", "Technology", "Data"}}}
	if got := warn(split); got != "" {
		t.Errorf("split foundation: %q", got)
	}

	empty := &StrategyHouseValues{Objective: "Grow", Foundation: "Base", Pillars: []StrategyHousePillar{
		{Title: "Trust", Body: []string{"A", "B", "C"}}, {Title: "Reach"}, {Title: "Speed", Body: []string{"A"}},
	}}
	if got := warn(empty); !strings.Contains(got, `pillars[1] ("Reach") has no body while other pillars carry up to 3 bullets`) {
		t.Errorf("empty pillar: %q", got)
	}
	empty.Pillars[0].Body = empty.Pillars[0].Body[:2]
	if got := warn(empty); strings.Contains(got, ErrCodeHouseShapeForced) {
		t.Errorf("two-bullet neighbours: %q", got)
	}
}

// TestStrategyHouseSmallRegionIsReported: in a region too small for its
// levels the roof flattens before any text shrinks, and the placement is
// reported rather than clipped.
func TestStrategyHouseSmallRegionIsReported(t *testing.T) {
	p := &strategyHouse{}
	v := &StrategyHouseValues{Objective: "Turn AI from pilots into a repeatable operating capability", Pillars: housePillars(4),
		FoundationLayers: []StrategyHouseLayer{{"Governance and cost tracking"}, {"People", "Data", "Controls"}}}
	const emu = 12700
	narrow := ExpandContext{LayoutBounds: LayoutBounds{Width: 400 * emu, Height: 170 * emu}}
	got := strings.Join(p.PostExpandWarnings(narrow, v, nil), "\n")
	if !strings.Contains(got, ErrCodeBodyTooLong+": strategy-house needs about") {
		t.Errorf("small region not reported: %q", got)
	}
	layout, err := BuildHouse(v.model(), v.style(narrow, &StrategyHouseOverrides{}, nil), 400, 170)
	if err != nil || !layout.Tight || !layout.RoofFlattened {
		t.Fatalf("layout = %+v (%v)", layout, err)
	}
	// Half a slide wide but full height: the house fits and keeps its roof.
	half := ExpandContext{LayoutBounds: LayoutBounds{Width: 400 * emu, Height: 340 * emu}}
	light := &StrategyHouseValues{Objective: "Make AI repeatable", Foundation: "Governance", Pillars: []StrategyHousePillar{
		{Title: "Workflow", Body: []string{"Map end to end"}}, {Title: "Platform", Body: []string{"Shared tools"}}, {Title: "People", Body: []string{"Train owners"}},
	}}
	if got := p.PostExpandWarnings(half, light, nil); len(got) != 0 {
		t.Errorf("half-width house reported: %v", got)
	}
	layout, err = BuildHouse(light.model(), light.style(half, &StrategyHouseOverrides{}, nil), 400, 340)
	if err != nil || layout.RoofFlattened || layout.RoofRisePt < 400*houseRoofMinPitch {
		t.Fatalf("half-width roof = %+v (%v)", layout, err)
	}
}

func TestHouseColumnsFit(t *testing.T) {
	for _, tc := range []struct {
		counts []int
		want   bool
	}{{[]int{4, 1, 3}, true}, {[]int{5, 4}, true}, {[]int{5, 3, 4}, false}, {[]int{7, 5, 3}, false}, {[]int{12, 1, 6}, true}} {
		if got := HouseColumnsFit(tc.counts...); got != tc.want {
			t.Errorf("HouseColumnsFit(%v) = %t, want %t", tc.counts, got, tc.want)
		}
	}
}
