package patterns

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

func sshValues(n int) *StateShiftHubValues {
	pool := []StateShiftPair{
		{Title: "Intake", Before: "Requests triaged by hand", After: "Agents route every request"},
		{Title: "Research", Before: "Days assembling context", After: "Sourced brief in minutes"},
		{Title: "Decision", Before: "Weekly committee review", After: "Same-day approvals"},
		{Title: "Follow-up", Before: "Status chased manually", After: "Agents escalate exceptions"},
		{Title: "Reporting", Before: "Static packs a week late", After: "Live dashboards"},
		{Title: "Controls", Before: "Sample testing", After: "Continuous monitoring"},
		{Title: "Extra", Before: "One too many", After: "One too many"},
	}
	return &StateShiftHubValues{
		HubLabel:    "Today's state vs. agentic state",
		LeftHeader:  "TODAY'S STATE",
		RightHeader: "AGENTIC STATE",
		Pairs:       append([]StateShiftPair(nil), pool[:n]...),
	}
}

func TestStateShiftHub_Metadata(t *testing.T) {
	p, ok := Default().Get("state-shift-hub")
	if !ok {
		t.Fatal("state-shift-hub not registered")
	}
	if p.Name() != "state-shift-hub" || p.Version() != 1 {
		t.Errorf("name/version = %q/%d", p.Name(), p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" || p.Description() == "" || p.CellsHint() == "" {
		t.Error("UseWhen/NotWhen/Description/CellsHint must be non-empty (D6)")
	}
	for _, sibling := range []string{"before-after", "comparison-2col", "journey-maturity-model"} {
		if !strings.Contains(p.UseWhen(), sibling) || !strings.Contains(p.NotWhen(), sibling) {
			t.Errorf("UseWhen/NotWhen should contrast with %s", sibling)
		}
	}
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" || len(tx.NarrativeRole) == 0 || len(tx.PairsWith) == 0 {
		t.Errorf("taxonomy incomplete: %+v", tx)
	}
	for _, sib := range tx.PairsWith {
		if _, ok := Default().Get(sib); !ok {
			t.Errorf("PairsWith names unregistered pattern %q", sib)
		}
	}
}

func TestStateShiftHub_SchemaIsValidJSON(t *testing.T) {
	p := &stateShiftHub{}
	data, err := json.Marshal(p.Schema())
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if m["$schema"] == nil {
		t.Error("schema must be a root schema")
	}
	for _, key := range []string{"hub_label", "left_header", "right_header", "pairs", "before_title", "after_title", "title_size", "body_size", "hub_size", "semantic_accent"} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("schema missing %q", key)
		}
	}
}

func TestStateShiftHub_Validate(t *testing.T) {
	p := &stateShiftHub{}
	for n := sshMinPairs; n <= sshMaxPairs; n++ {
		if err := p.Validate(sshValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected error: %v", n, err)
		}
	}

	if err := p.Validate(sshValues(2), nil, nil); err == nil || !strings.Contains(err.Error(), "before-after") {
		t.Errorf("2 pairs: want min-items error with before-after hint, got %v", err)
	}
	if err := p.Validate(sshValues(7), nil, nil); err == nil {
		t.Error("7 pairs: want max-items error")
	}

	v := sshValues(3)
	v.HubLabel = " "
	v.Pairs[1].After = ""
	err := p.Validate(v, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "hub_label") || !strings.Contains(err.Error(), "pairs[1].after") {
		t.Errorf("want hub_label and pairs[1].after required errors (aggregated), got %v", err)
	}

	// Budgets count characters, not bytes: 140 euro signs are 420 bytes.
	v = sshValues(3)
	v.Pairs[0].Before = strings.Repeat("€", sshBodyMax)
	v.Pairs[0].BeforeTitle = strings.Repeat("ü", sshTitleMax)
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("multi-byte text inside the budget rejected: %v", err)
	}
	v.Pairs[0].Before += "€"
	var ve *ValidationError
	if err := p.Validate(v, nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxLength {
		t.Errorf("want max_length error, got %v", err)
	}

	v = sshValues(3)
	v.LeftHeader = strings.Repeat("H", sshHeaderMax+1)
	if err := p.Validate(v, nil, nil); err == nil {
		t.Error("over-long header accepted")
	}

	if err := p.Validate(sshValues(3), nil, map[int]any{0: &CellOverride{}}); err == nil {
		t.Error("cell_overrides must be rejected")
	}
	if err := p.Validate(sshValues(3), &TextOverrides{}, nil); err == nil {
		t.Error("wrong overrides type must be rejected")
	}
}

// sshResolve expands and resolves the grid at the default bounds.
func sshResolve(t *testing.T, v *StateShiftHubValues, ovr *StateShiftHubOverrides) (*jsonschema.ShapeGridInput, *shapegrid.ResolveResult) {
	t.Helper()
	p := &stateShiftHub{}
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	grid, err := p.Expand(ExpandContext{}, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	ApplyGridDefaults(grid)
	res := resolvePatternGrid(t, grid)
	return grid, res
}

func TestStateShiftHub_ExpandLayout(t *testing.T) {
	for n := sshMinPairs; n <= sshMaxPairs; n++ {
		for _, headers := range []bool{true, false} {
			v := sshValues(n)
			if !headers {
				v.LeftHeader, v.RightHeader = "", ""
			}
			grid, res := sshResolve(t, v, nil)
			wantRows := n
			if headers {
				wantRows++
			}
			if len(grid.Rows) != wantRows {
				t.Fatalf("n=%d headers=%v: rows = %d, want %d", n, headers, len(grid.Rows), wantRows)
			}

			var hub *shapegrid.ResolvedCell
			var nodes []shapegrid.ResolvedCell
			shapes := 0
			for i := range res.Cells {
				c := res.Cells[i]
				if c.ShapeSpec == nil {
					continue
				}
				shapes++
				if c.ShapeSpec.Geometry == "ellipse" {
					if c.Bounds.CX > 80*12700 {
						hub = &res.Cells[i]
					} else {
						nodes = append(nodes, c)
					}
				}
			}
			wantShapes := 1 + 4*n
			if headers {
				wantShapes += 2
			}
			if shapes != wantShapes {
				t.Errorf("n=%d headers=%v: %d shapes, want %d", n, headers, shapes, wantShapes)
			}
			if hub == nil {
				t.Fatalf("n=%d: no hub circle", n)
			}
			if hub.Bounds.CX != hub.Bounds.CY {
				t.Errorf("n=%d: hub is not a circle: %dx%d", n, hub.Bounds.CX, hub.Bounds.CY)
			}
			if len(nodes) != 2*n {
				t.Fatalf("n=%d: %d nodes, want %d", n, len(nodes), 2*n)
			}
			// Nodes are circles, sit clear of the hub, and pair up symmetrically.
			hubCX := hub.Bounds.X + hub.Bounds.CX/2
			for _, nd := range nodes {
				if nd.Bounds.CX != nd.Bounds.CY {
					t.Errorf("n=%d: node is not a circle: %dx%d", n, nd.Bounds.CX, nd.Bounds.CY)
				}
				if nd.Bounds.X < hub.Bounds.X+hub.Bounds.CX && nd.Bounds.X+nd.Bounds.CX > hub.Bounds.X {
					t.Errorf("n=%d: node at x=%d overlaps the hub column", n, nd.Bounds.X)
				}
			}
			assertNoOverlap(t, res)

			// The arc: with 3+ pairs the middle row's nodes sit further from
			// the hub than the top row's.
			left := map[int64]int64{}
			for _, nd := range nodes {
				if nd.Bounds.X < hubCX {
					left[nd.Bounds.Y] = hubCX - (nd.Bounds.X + nd.Bounds.CX/2)
				}
			}
			var top, mid int64
			var ys []int64
			for y := range left {
				ys = append(ys, y)
			}
			sortInt64(ys)
			top, mid = left[ys[0]], left[ys[len(ys)/2]]
			if mid <= top {
				t.Errorf("n=%d: middle node (%d) should sit further out than the top node (%d)", n, mid, top)
			}
		}
	}
}

// resolvePatternGrid resolves an expanded grid at the default 16:9 bounds,
// carrying the row/cell fields this pattern relies on (row spans, fit modes,
// row height caps).
func resolvePatternGrid(t *testing.T, in *jsonschema.ShapeGridInput) *shapegrid.ResolveResult {
	t.Helper()
	var cols []float64
	if err := json.Unmarshal(in.Columns, &cols); err != nil {
		t.Fatalf("columns: %v", err)
	}
	rows := make([]shapegrid.Row, len(in.Rows))
	for i, r := range in.Rows {
		cells := make([]shapegrid.Cell, len(r.Cells))
		for j, c := range r.Cells {
			if c == nil || c.Shape == nil {
				continue
			}
			cells[j] = shapegrid.Cell{
				ColSpan: c.ColSpan, RowSpan: c.RowSpan, Fit: shapegrid.FitMode(c.Fit),
				Shape: &shapegrid.ShapeSpec{Geometry: c.Shape.Geometry, Fill: c.Shape.Fill, Line: c.Shape.Line, Text: c.Shape.Text},
			}
		}
		rows[i] = shapegrid.Row{Cells: cells, Height: r.Height, MinHeight: r.MinHeight, MaxHeight: r.MaxHeight}
	}
	vAlign, _ := shapegrid.ParseVerticalAlign(in.VerticalAlign)
	g := &shapegrid.Grid{
		Bounds:  shapegrid.DefaultBounds(shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU),
		Columns: cols, Rows: rows, ColGap: in.ColGap, RowGap: in.RowGap, VAlign: vAlign,
	}
	if err := shapegrid.Validate(g); err != nil {
		t.Fatalf("validate: %v", err)
	}
	res, err := shapegrid.Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return res
}

func sortInt64(s []int64) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func assertNoOverlap(t *testing.T, res *shapegrid.ResolveResult) {
	t.Helper()
	var boxes []shapegrid.ResolvedCell
	for _, c := range res.Cells {
		if c.ShapeSpec != nil {
			boxes = append(boxes, c)
		}
	}
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i].CellBounds, boxes[j].CellBounds
			if a.X < b.X+b.CX && b.X < a.X+a.CX && a.Y < b.Y+b.CY && b.Y < a.Y+a.CY {
				t.Errorf("cells overlap: %+v and %+v", a, b)
			}
		}
	}
}

func TestStateShiftHub_ExpandStyling(t *testing.T) {
	p := &stateShiftHub{}
	v := sshValues(3)
	v.Pairs[2].BeforeTitle, v.Pairs[2].AfterTitle = "Legacy", "Modern"
	grid, err := p.Expand(ExpandContext{}, v, &StateShiftHubOverrides{Accent: "accent3", TitleSize: 16, BodySize: 13}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(grid)
	out := string(raw)
	for _, want := range []string{`"accent3"`, `"01"`, `"03"`, `"Legacy"`, `"Modern"`, `"size":16`, `"size":13`, "Today's state vs. agentic state"} {
		if !strings.Contains(out, want) {
			t.Errorf("expansion missing %s", want)
		}
	}
	if strings.Contains(out, `"accent1"`) {
		t.Error("accent override not applied everywhere")
	}

	// semantic_accent resolves through theme semantic accents.
	ctx := ExpandContext{}
	ctx.Theme.SemanticAccents = map[string]string{"positive": "accent6"}
	grid, err = p.Expand(ctx, sshValues(3), &StateShiftHubOverrides{SemanticAccent: "positive"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(grid)
	if !strings.Contains(string(raw), `"accent6"`) {
		t.Error("semantic_accent not resolved")
	}
}

func TestStateShiftHub_HubLabelShrinks(t *testing.T) {
	v := sshValues(4)
	short := sshMeasure(ExpandContext{}, v, &StateShiftHubOverrides{})
	v.HubLabel = "Transformation of the operating model end to end"
	long := sshMeasure(ExpandContext{}, v, &StateShiftHubOverrides{})
	if long.hubSize >= short.hubSize {
		t.Errorf("long hub label should shrink: %v vs %v", long.hubSize, short.hubSize)
	}
	if long.hubSize < shapegrid.MinTextSizePt {
		t.Errorf("hub label below the renderer floor: %v", long.hubSize)
	}
}

func TestStateShiftHub_PostExpandWarnings(t *testing.T) {
	p := &stateShiftHub{}
	if got := p.PostExpandWarnings(ExpandContext{}, sshValues(6), nil); len(got) != 0 {
		t.Errorf("short copy should not warn: %v", got)
	}
	v := sshValues(6)
	v.Pairs[3].After = strings.Repeat("word ", 28)
	got := p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") || !strings.Contains(got[0], "pairs[3].after") {
		t.Errorf("want one BODY_TOO_LONG for pairs[3].after, got %v", got)
	}
	v = sshValues(3)
	v.HubLabel = strings.Repeat("W", 22) + " " + strings.Repeat("W", 22)
	got = p.PostExpandWarnings(ExpandContext{}, v, nil)
	if len(got) != 1 || !strings.Contains(got[0], "hub_label") {
		t.Errorf("want hub_label warning, got %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Errorf("nil values: %v", got)
	}
}

func TestStateShiftHub_Golden(t *testing.T) {
	p := &stateShiftHub{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "state-shift-hub", "default.golden.json"))
}

func TestStateShiftHub_RecommendIntents(t *testing.T) {
	cases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"today vs future state across the workflow", &ContentHints{ItemCount: 4}, "state-shift-hub"},
		{"current vs target state for the finance close", &ContentHints{ItemCount: 5}, "state-shift-hub"},
		{"state shift from manual to agentic operations", nil, "state-shift-hub"},
		{"from-to shifts in how we work", &ContentHints{ItemCount: 4}, "state-shift-hub"},
		// A single before/after contrast still belongs to before-after.
		{"current state assessment", nil, "before-after"},
	}
	for _, tc := range cases {
		res := Recommend(Default(), tc.intent, tc.hints, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != tc.want {
			t.Errorf("intent %q: top = %v, want %s", tc.intent, res.Candidates, tc.want)
		}
	}
}
