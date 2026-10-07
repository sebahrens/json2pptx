package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

func archComponentValues() *ArchStackValues {
	return &ArchStackValues{
		Tiers: []ArchStackTier{
			{Label: "Channels", Components: []string{"Web", "Mobile app", "Partner API"}},
			{Label: "Services", Description: "Orders, pricing, fulfilment"},
			{Label: "Platform"},
			{Label: "Data", Components: []string{"A", "B", "C", "D", "E", "F", "G", "H", "I"}},
		},
		SideRails: []string{"Security"},
	}
}

// go-slide-creator-6h1fy, go-slide-creator-tfxns: a tier with components is a
// lane — a pentagon tab before one block per component, no band behind them;
// seven or more wrap to two rows; the rails are dark bars, so they read as
// neither one more lane nor a second accent.
func TestArchStack_ComponentsRenderAsBlocks(t *testing.T) {
	p, _ := Default().Get("arch-stack")
	vals := archComponentValues()
	if err := p.Validate(vals, nil, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if got := len(grid.Rows); got != 4 {
		t.Fatalf("rows = %d, want one per tier", got)
	}
	blocks := func(tier int) [][]string {
		var out [][]string
		for _, c := range grid.Rows[tier].Cells {
			if c == nil || c.Grid == nil {
				continue
			}
			for _, r := range c.Grid.Rows {
				var row []string
				for _, b := range r.Cells {
					if len(b.Shape.Text) == 0 {
						continue
					}
					var text struct {
						Paragraphs []struct {
							Content string `json:"content"`
						} `json:"paragraphs"`
					}
					if err := json.Unmarshal(b.Shape.Text, &text); err != nil || len(text.Paragraphs) != 1 {
						t.Fatalf("tier %d block text %s: %v", tier, b.Shape.Text, err)
					}
					row = append(row, text.Paragraphs[0].Content)
				}
				if len(row) > 0 {
					out = append(out, row)
				}
			}
		}
		return out
	}
	if got := blocks(0); len(got) != 1 || strings.Join(got[0], "|") != "Web|Mobile app|Partner API" {
		t.Errorf("tier 0 blocks = %v, want one block per component in one row", got)
	}
	if got := blocks(1); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "Orders, pricing, fulfilment" {
		t.Errorf("tier 1 = %v, want its description as one line of text", got)
	}
	if got := blocks(2); len(got) != 0 {
		t.Errorf("label-only tier 2 carries %v", got)
	}
	if got := blocks(3); len(got) != 2 || len(got[0]) != 5 || len(got[1]) != 4 {
		t.Errorf("tier 3 blocks = %v, want 9 components wrapped to rows of 5 and 4", got)
	}

	// Every tier leads with a pentagon tab carrying its name in bold, pulled
	// in to the blocks' edges; nothing lies behind the blocks.
	var tabFill, blockFill string
	for i, row := range grid.Rows {
		tab := row.Cells[0]
		if tab.Shape == nil || tab.Shape.Geometry != "homePlate" || tab.InsetTop != archStackSubGridInsetPt || tab.InsetBottom != archStackSubGridInsetPt ||
			!strings.Contains(string(tab.Shape.Text), vals.Tiers[i].Label) || !strings.Contains(string(tab.Shape.Text), `"bold":true`) {
			t.Errorf("tier %d label = %+v, want a bold pentagon tab inset to the blocks' edges", i, tab)
		}
		tabFill = string(tab.Shape.Fill)
		for _, c := range row.Cells {
			if c != nil && c.Shape != nil && c.Shape.Geometry == "rect" && c.RowSpan == 0 {
				t.Errorf("tier %d draws a band behind its blocks: %+v", i, c.Shape)
			}
		}
	}
	blockFill = string(grid.Rows[0].Cells[1].Grid.Rows[0].Cells[0].Shape.Fill)
	if tabFill == blockFill || !strings.Contains(tabFill, "accent1") || !strings.Contains(blockFill, "accent1") {
		t.Errorf("tab fill %s, block fill %s; want two rungs of the accent ladder", tabFill, blockFill)
	}
	var rail *jsonschema.GridCellInput
	for _, c := range grid.Rows[0].Cells {
		if c != nil && c.Shape != nil && strings.Contains(string(c.Shape.Text), "Security") {
			rail = c
		}
	}
	if rail == nil || rail.RowSpan != 4 || strings.Contains(string(rail.Shape.Fill), "accent") || !strings.Contains(string(rail.Shape.Text), `"bold":true`) {
		t.Errorf("rail = %+v; want one dark neutral bar down every tier with a bold name", rail)
	}
}

// The stack is set one type step up when no name wraps for it and it keeps
// its air, and grows its lanes towards the lower third.
func TestArchStack_ComponentScale(t *testing.T) {
	pat := &archStack{}
	ctx := fullThemeCtx()
	_, contentH := contentAreaPt(ctx)
	lay := layoutArchStackComponents(ctx, pat.ExemplarValues().(*ArchStackValues), &ArchStackOverrides{})
	if lay.headerSize != archStackLeadScale[0] || lay.bodySize != archStackLeadScale[1] {
		t.Errorf("exemplar set at %.0f / %.0fpt, want the larger step %v", lay.headerSize, lay.bodySize, archStackLeadScale)
	}
	sum := 0.0
	for _, h := range lay.tierH {
		sum += h
	}
	if sum < lay.needPt || sum > contentH {
		t.Errorf("lanes take %.0fpt: want at least the %.0fpt they need and at most the %.0fpt area", sum, lay.needPt, contentH)
	}
	if kept := layoutArchStackComponents(ctx, pat.ExemplarValues().(*ArchStackValues), &ArchStackOverrides{BodySize: 12}); kept.bodySize != 12 || kept.headerSize != scaleSubheadPt {
		t.Errorf("authored body_size: set at %.0f / %.0fpt, want 14 / 12", kept.headerSize, kept.bodySize)
	}
	// Twelve components in six tiers have no room for the larger step.
	dense := &ArchStackValues{}
	for i := 0; i < 6; i++ {
		tier := ArchStackTier{Label: "Tier"}
		for j := 0; j < 12; j++ {
			tier.Components = append(tier.Components, "Service name")
		}
		dense.Tiers = append(dense.Tiers, tier)
	}
	if lay := layoutArchStackComponents(ctx, dense, &ArchStackOverrides{}); lay.bodySize != scaleDenseBodyPt {
		t.Errorf("dense stack set at %.0fpt, want the standard %.0fpt", lay.bodySize, scaleDenseBodyPt)
	}
}

// A stack without components is drawn as before: one centred cell per tier.
func TestArchStack_DescriptionOnlyUnchanged(t *testing.T) {
	p, _ := Default().Get("arch-stack")
	vals := &ArchStackValues{
		Tiers:     []ArchStackTier{{Label: "A", Description: "x, y"}, {Label: "B", Description: "z"}, {Label: "C"}},
		SideRails: []string{"Security"},
	}
	grid, err := p.Expand(fullThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 3 || len(grid.Rows[0].Cells) != 2 || grid.Rows[0].Cells[0].AccentBar == nil {
		t.Fatalf("description-only stack: %d rows, %d cells in row 0; want the one-cell-per-tier layout", len(grid.Rows), len(grid.Rows[0].Cells))
	}
	if text := string(grid.Rows[0].Cells[0].Shape.Text); !strings.Contains(text, "x, y") || !strings.Contains(text, `"align":"ctr"`) {
		t.Errorf("tier text = %s, want label over description, centred", text)
	}
}

func TestArchStack_ComponentValidation(t *testing.T) {
	p, _ := Default().Get("arch-stack")
	for name, tc := range map[string]struct {
		edit func(*ArchStackValues)
		want string
	}{
		"too many":       {func(v *ArchStackValues) { v.Tiers[0].Components = make([]string, 13) }, "tiers[0].components"},
		"empty name":     {func(v *ArchStackValues) { v.Tiers[0].Components[1] = " " }, "tiers[0].components[1]"},
		"long name":      {func(v *ArchStackValues) { v.Tiers[0].Components[1] = strings.Repeat("x", 41) }, "tiers[0].components[1]"},
		"with a summary": {func(v *ArchStackValues) { v.Tiers[0].Description = "also text" }, "both components and description"},
	} {
		vals := archComponentValues()
		tc.edit(vals)
		if err := p.Validate(vals, nil, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want one naming %q", name, err, tc.want)
		}
	}
}

// A component whose name cannot be written readably in its block is reported.
func TestArchStack_ComponentWarnings(t *testing.T) {
	pat := &archStack{}
	ctx := fullThemeCtx()
	if got := pat.PostExpandWarnings(ctx, archComponentValues(), nil); len(got) != 0 {
		t.Fatalf("fitting stack warned: %v", got)
	}
	// Six tiers of two block rows are more than the slide holds: the bands
	// are squeezed and a name that wraps shrinks in its block.
	vals := &ArchStackValues{}
	for i := 0; i < 6; i++ {
		tier := ArchStackTier{Label: "Tier"}
		for j := 0; j < 12; j++ {
			tier.Components = append(tier.Components, "Svc")
		}
		vals.Tiers = append(vals.Tiers, tier)
	}
	vals.Tiers[3].Components[2] = "Customer identity and access service"
	got := pat.PostExpandWarnings(ctx, vals, nil)
	if len(got) == 0 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong) || !strings.Contains(strings.Join(got, "\n"), "tiers[3].components[2]") {
		t.Fatalf("warnings = %v, want BODY_TOO_LONG naming tiers[3].components[2]", got)
	}
}
