package patterns

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/types"
)

// gridGaps collects every non-zero Gap / ColGap / RowGap of a grid and its
// nested cell grids, in a stable walk order.
func gridGaps(g *jsonschema.ShapeGridInput) []float64 {
	if g == nil {
		return nil
	}
	var out []float64
	for _, v := range []float64{g.Gap, g.ColGap, g.RowGap} {
		if v != 0 {
			out = append(out, v)
		}
	}
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			if cell != nil {
				out = append(out, gridGaps(cell.Grid)...)
			}
		}
	}
	return out
}

func TestExpandContextGapIdentityWithoutTemplateGrid(t *testing.T) {
	for _, ctx := range []ExpandContext{
		{},
		{Metadata: &types.TemplateMetadata{}},
		{ContentZone: &shapegrid.ContentZone{}},
		{Metadata: &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: types.DefaultGridGutterPt}}},
	} {
		for _, pt := range []float64{0.01, 1, 2, 4, 6, 7, 8, 12, 24} {
			if got := ctx.Gap(pt); got != pt {
				t.Errorf("Gap(%g) = %g with no / default gutter, want unchanged", pt, got)
			}
		}
		if ctx.GutterPt() != types.DefaultGridGutterPt {
			t.Errorf("GutterPt() = %g, want default", ctx.GutterPt())
		}
	}
	scaled := ExpandContext{Metadata: &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: 12}}}
	if got := scaled.Gap(8); got != 12 {
		t.Errorf("Gap(8) under a 12pt gutter = %g, want 12", got)
	}
	if got := scaled.Gap(4); got != 6 {
		t.Errorf("Gap(4) under a 12pt gutter = %g, want 6", got)
	}
	if got := scaled.Gap(0.01); got != 0.01 {
		t.Errorf("hairline gap scaled to %g", got)
	}
	// The content zone's gutter (generation / preflight geometry) counts too.
	zoned := ExpandContext{ContentZone: &shapegrid.ContentZone{GutterPt: 16}}
	if got := zoned.Gap(6); got != 12 {
		t.Errorf("Gap(6) under a zone gutter of 16 = %g, want 12", got)
	}
}

// Every registered pattern routes its internal gaps through the template
// grid: with no declared gutter the expansion is byte-identical to an
// explicit 8pt gutter, and a 16pt gutter doubles every authored gap above the
// hairline floor (go-slide-creator-5ms8c).
func TestPatternGapsFollowTemplateGutter(t *testing.T) {
	base := ExpandContext{SlideWidth: 12192000, SlideHeight: 6858000}
	explicit8 := base
	explicit8.Metadata = &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: 8}}
	wide := base
	wide.Metadata = &types.TemplateMetadata{Grid: &types.TemplateGrid{GutterPt: 16}}

	// swimlane's gaps are connector channels sized against its step-text
	// budget (TestSchemaMaximaStayReadable), not gutters: they stay fixed.
	// matrix-2x2's open quadrants touch their crossing axis lines: its only
	// authored gaps are the fixed 4pt inside the two axis strips.
	fixedGaps := map[string]bool{"swimlane": true, "matrix-2x2": true}

	scaledPatterns := 0
	for _, p := range Default().List() {
		ex, ok := p.(interface{ ExemplarValues() any })
		if !ok {
			continue
		}
		expand := func(ctx ExpandContext) *jsonschema.ShapeGridInput {
			g, err := p.Expand(ctx, ex.ExemplarValues(), nil, nil)
			if err != nil {
				t.Fatalf("%s: expand: %v", p.Name(), err)
			}
			return g
		}
		plain := expand(base)
		plainJSON, _ := json.Marshal(plain)
		eightJSON, _ := json.Marshal(expand(explicit8))
		if string(plainJSON) != string(eightJSON) {
			t.Errorf("%s: an explicit 8pt gutter changed the expansion", p.Name())
		}

		var authored []float64
		for _, g := range gridGaps(plain) {
			if g > gapScaleFloorPt {
				authored = append(authored, g)
			}
		}
		if len(authored) == 0 || fixedGaps[p.Name()] {
			continue
		}
		var widened []float64
		for _, g := range gridGaps(expand(wide)) {
			if g > gapScaleFloorPt {
				widened = append(widened, g)
			}
		}
		// Row gaps can be content-derived (shrunk to fit), so require the
		// largest authored gap to double rather than every gap.
		maxOf := func(v []float64) float64 {
			m := 0.0
			for _, x := range v {
				m = math.Max(m, x)
			}
			return m
		}
		if got, want := maxOf(widened), 2*maxOf(authored); math.Abs(got-want) > 0.01 {
			t.Errorf("%s: largest gap %g under a 16pt gutter, want %g (2 x %g)", p.Name(), got, want, maxOf(authored))
			continue
		}
		scaledPatterns++
	}
	if scaledPatterns < 30 {
		t.Errorf("only %d patterns scaled their gaps with the template gutter", scaledPatterns)
	}
}
