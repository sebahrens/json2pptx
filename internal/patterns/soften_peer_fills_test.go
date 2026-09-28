package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

func lightAccentCtx() ExpandContext {
	ctx := fullThemeCtx()
	ctx.Theme = types.ThemeInfo{Colors: append([]types.ThemeColor(nil), ctx.Theme.Colors...)}
	for i := range ctx.Theme.Colors {
		if ctx.Theme.Colors[i].Name == "accent1" {
			ctx.Theme.Colors[i].RGB = "#FD5108" // p-style orange: lt1 text fails AA
		}
	}
	return ctx
}

func peerCell(fill string) *jsonschema.GridCellInput {
	return &jsonschema.GridCellInput{Shape: &jsonschema.ShapeSpecInput{
		Geometry: "rect",
		Fill:     json.RawMessage(fill),
		Text:     json.RawMessage(`{"content":"x","color":"lt1"}`),
	}}
}

func peerGrid(cells ...*jsonschema.GridCellInput) *jsonschema.ShapeGridInput {
	return &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: cells}}}
}

// Peer cards become the neutral 4% surface plus a top accent rule on every
// template: light accents (p-style orange) and dark ones (navy) alike, so
// accent1 at 100% is left for the slide's one emphasised element
// (go-slide-creator-8xsj3).
func TestSoftenPeerFills_PeersBecomeNeutralWithRule(t *testing.T) {
	for name, ctx := range map[string]ExpandContext{"light accent": lightAccentCtx(), "dark accent": fullThemeCtx()} {
		grid := peerGrid(peerCell(`"accent1"`), peerCell(`"accent1"`), peerCell(`"accent1"`))
		SoftenPeerFills(ctx, "kpi-3up", grid)
		for i, c := range grid.Rows[0].Cells {
			if got := string(c.Shape.Fill); got != neutral4JSON {
				t.Errorf("%s: cell %d fill = %s, want neutral 4%%", name, i, got)
			}
			if c.AccentBar == nil || c.AccentBar.Color != "accent1" || c.AccentBar.Position != "top" {
				t.Errorf("%s: cell %d accent bar = %+v, want top accent1 rule", name, i, c.AccentBar)
			}
		}
	}
}

func TestSoftenPeerFills_KeepsSolidFills(t *testing.T) {
	cases := []struct {
		name    string
		ctx     ExpandContext
		pattern string
		grid    *jsonschema.ShapeGridInput
	}{
		{"not a peer-card pattern", lightAccentCtx(), "capability-heatmap", peerGrid(peerCell(`"accent1"`), peerCell(`"accent1"`))},
		{"lone solid card is emphasis", lightAccentCtx(), "card-grid", peerGrid(peerCell(`"accent1"`), peerCell(`{"color":"accent1","lumMod":20000,"lumOff":80000}`))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			SoftenPeerFills(tc.ctx, tc.pattern, tc.grid)
			if got := string(tc.grid.Rows[0].Cells[0].Shape.Fill); got != `"accent1"` {
				t.Errorf("fill = %s, want solid accent1 kept", got)
			}
			if tc.grid.Rows[0].Cells[0].AccentBar != nil {
				t.Error("unexpected accent bar")
			}
		})
	}
}

func TestSharedRotationKeyGroupsKPIFamily(t *testing.T) {
	a, okA := SharedRotationKey("kpi-3up")
	b, okB := SharedRotationKey("kpi-4up")
	if !okA || !okB || a != b {
		t.Errorf("kpi-3up / kpi-4up keys = %q,%v / %q,%v; want one shared key", a, okA, b, okB)
	}
	if _, ok := SharedRotationKey("card-grid"); ok {
		t.Error("card-grid should rotate on its own content")
	}
}

func TestSoftenPeerFills_CardWithOwnAccentBarStaysSolid(t *testing.T) {
	marked := peerCell(`"accent1"`)
	marked.AccentBar = &jsonschema.AccentBarInput{Position: "top", Color: "dk1"}
	grid := peerGrid(peerCell(`"accent1"`), marked, peerCell(`"accent1"`))
	SoftenPeerFills(lightAccentCtx(), "card-grid", grid)
	if got := string(marked.Shape.Fill); got != `"accent1"` {
		t.Errorf("emphasised card fill = %s, want solid accent1", got)
	}
	if got := string(grid.Rows[0].Cells[0].Shape.Fill); got == `"accent1"` {
		t.Error("peer card was not softened")
	}
}
