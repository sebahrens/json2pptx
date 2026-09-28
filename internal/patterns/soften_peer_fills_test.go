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

func TestSoftenPeerFills_LightAccentPeersBecomeTintWithRule(t *testing.T) {
	grid := peerGrid(peerCell(`"accent1"`), peerCell(`"accent1"`), peerCell(`"accent1"`))
	SoftenPeerFills(lightAccentCtx(), "kpi-3up", grid)
	for i, c := range grid.Rows[0].Cells {
		tone, _ := parseFillTone(c.Shape.Fill)
		if tone.Color != "accent1" || tone.LumMod != tintLumMod || tone.LumOff != tintLumOff {
			t.Errorf("cell %d fill = %s, want accent1 light tint", i, c.Shape.Fill)
		}
		if c.AccentBar == nil || c.AccentBar.Color != "accent1" || c.AccentBar.Position != "top" {
			t.Errorf("cell %d accent bar = %+v, want top accent1 rule", i, c.AccentBar)
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
		{"dark accent", fullThemeCtx(), "kpi-3up", peerGrid(peerCell(`"accent1"`), peerCell(`"accent1"`))},
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
