package patterns

import (
	"encoding/json"
	"strings"
	"testing"
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

// go-slide-creator-6h1fy: a tier with components is a band holding one block
// per component; seven or more wrap to two rows; the rails take an accent tint
// so they do not read as one more neutral tier.
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
	// Two rows per tier: its content, then the band that bleeds up under it.
	if got := len(grid.Rows); got != 8 {
		t.Fatalf("rows = %d, want 2 per tier", got)
	}
	blocks := func(tier int) [][]string {
		var out [][]string
		for _, c := range grid.Rows[2*tier].Cells {
			if c == nil || c.Grid == nil {
				continue
			}
			for _, r := range c.Grid.Rows {
				var row []string
				for _, b := range r.Cells {
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
				out = append(out, row)
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

	// The band is the last cell of the row under the content, bled up over it,
	// with the tier name held to the label column.
	bandRow := grid.Rows[1]
	band := bandRow.Cells[len(bandRow.Cells)-1]
	if band.BleedTop < grid.Rows[0].MaxHeight || !strings.Contains(string(band.Shape.Text), `"inset_right"`) || !strings.Contains(string(band.Shape.Text), "Channels") {
		t.Errorf("band = bleed_top %.1f text %s; want it bled over the %.0fpt content row with the label inset from the blocks", band.BleedTop, band.Shape.Text, grid.Rows[0].MaxHeight)
	}
	var rail, tier string
	for _, c := range grid.Rows[0].Cells {
		if c != nil && c.Shape != nil && strings.Contains(string(c.Shape.Text), "Security") {
			rail = string(c.Shape.Fill)
		}
	}
	tier = string(band.Shape.Fill)
	if rail == "" || rail == tier || !strings.Contains(rail, "accent1") {
		t.Errorf("rail fill %s, tier fill %s; want the rail in an accent tint, distinct from the neutral tier", rail, tier)
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
