package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

func crValues(n int) *ConcentricRingsValues {
	pool := []ConcentricRingsLayer{
		{Label: "Sponsor", Description: "Owns the budget and the outcome"},
		{Label: "Leadership team", Description: "Sets priorities across the functions"},
		{Label: "Function heads", Description: "Commit people and change the process"},
		{Label: "Front-line teams", Description: "Adopt the new ways of working"},
		{Label: "Customers and partners", Description: "Feel the change and judge it"},
		{Label: "Regulators", Description: "One too many"},
	}
	return &ConcentricRingsValues{Layers: append([]ConcentricRingsLayer(nil), pool[:n]...)}
}

// crThemeCtx is a midnight-blue-like theme with the accents the styling
// tests address, in the given content area (points; 0 = the default area).
func crThemeCtx(w, h float64) ExpandContext {
	ctx := panelAuditCtx(w, h)
	ctx.Theme = types.ThemeInfo{Colors: []types.ThemeColor{
		{Name: "dk1", RGB: "#000000"},
		{Name: "lt1", RGB: "#FFFFFF"},
		{Name: "dk2", RGB: "#1F2A44"},
		{Name: "lt2", RGB: "#E7EBF1"},
		{Name: "accent1", RGB: "#2E5090"},
		{Name: "accent2", RGB: "#F5C518"},
		{Name: "accent3", RGB: "#1B7F5C"},
		{Name: "accent4", RGB: "#8E44AD"},
		{Name: "accent5", RGB: "#C0392B"},
		{Name: "accent6", RGB: "#16A085"},
	}}
	return ctx
}

// crParts splits an expansion into the ring cell and the ladder cells in
// input order (top row first, i.e. the outermost layer first).
func crParts(t *testing.T, grid *jsonschema.ShapeGridInput) (ring *jsonschema.GridCellInput, ladder []*jsonschema.GridCellInput) {
	t.Helper()
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			switch {
			case c == nil:
			case len(c.Layers) > 0:
				if ring != nil {
					t.Fatal("more than one cell carries layers")
				}
				ring = c
			case c.Shape != nil:
				ladder = append(ladder, c)
			}
		}
	}
	if ring == nil {
		t.Fatal("no ring cell")
	}
	return ring, ladder
}

func crLayersNamed(ring *jsonschema.GridCellInput, prefix string) []jsonschema.LayerInput {
	var out []jsonschema.LayerInput
	for _, l := range ring.Layers {
		if strings.HasPrefix(l.Name, prefix) {
			out = append(out, l)
		}
	}
	return out
}

func crExpand(t *testing.T, ctx ExpandContext, v *ConcentricRingsValues, ovr *ConcentricRingsOverrides) *jsonschema.ShapeGridInput {
	t.Helper()
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	grid, err := (&concentricRings{}).Expand(ctx, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}

func TestConcentricRings_Metadata(t *testing.T) {
	p, ok := Default().Get("concentric-rings")
	if !ok {
		t.Fatal("concentric-rings not registered")
	}
	if p.Name() != "concentric-rings" || p.Version() != 1 {
		t.Errorf("name/version = %q/%d", p.Name(), p.Version())
	}
	if p.UseWhen() == "" || p.NotWhen() == "" || p.Description() == "" || p.CellsHint() == "" {
		t.Error("UseWhen/NotWhen/Description/CellsHint must be non-empty (D6)")
	}
	for _, sibling := range []string{"pyramid", "radial-hub", "cycle-ring"} {
		if !strings.Contains(p.UseWhen(), sibling) || !strings.Contains(p.NotWhen(), sibling) {
			t.Errorf("UseWhen/NotWhen should contrast with %s", sibling)
		}
	}
	tx := p.Taxonomy()
	if tx.Category == "" || tx.DensityClass == "" || tx.AccentWeight == "" || len(tx.NarrativeRole) == 0 || len(tx.PairsWith) == 0 || tx.DataVisual {
		t.Errorf("taxonomy: %+v", tx)
	}
	for _, sib := range tx.PairsWith {
		if _, ok := Default().Get(sib); !ok {
			t.Errorf("PairsWith names unregistered pattern %q", sib)
		}
	}
	if got := PatternMotif("concentric-rings"); got != MotifDiagram {
		t.Errorf("motif = %v, want diagram", got)
	}
}

func TestConcentricRings_SchemaIsValidJSON(t *testing.T) {
	data, err := json.Marshal((&concentricRings{}).Schema())
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if m["$schema"] == nil || m["$defs"] == nil {
		t.Error("schema must be a root schema with $defs")
	}
	if len(data) > 6000 {
		t.Errorf("schema is %d bytes, over the 6000-byte budget", len(data))
	}
	for _, key := range []string{"layers", "label", "description", "highlight", "header_size", "body_size", "cell_accent_mode", "semantic_accent", "cell_overrides"} {
		if !strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("schema missing %q", key)
		}
	}
}

func TestConcentricRings_Validate(t *testing.T) {
	p := &concentricRings{}
	for n := crMinLayers; n <= crMaxLayers; n++ {
		if err := p.Validate(crValues(n), nil, nil); err != nil {
			t.Errorf("n=%d: unexpected error: %v", n, err)
		}
	}
	var ve *ValidationError
	if err := p.Validate(crValues(2), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMinItems || !strings.Contains(err.Error(), "comparison-2col") {
		t.Errorf("2 layers: want min_items with a sibling hint, got %v", err)
	}
	if err := p.Validate(crValues(6), nil, nil); err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxItems || !strings.Contains(err.Error(), "labeled-rows") {
		t.Errorf("6 layers: want max_items with a sibling hint, got %v", err)
	}

	v := crValues(3)
	v.Layers[1].Label = " "
	if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "layers[1].label") {
		t.Errorf("want layers[1].label required, got %v", err)
	}

	// Budgets count characters, not bytes: 24 euro signs are 72 bytes.
	v = crValues(3)
	v.Layers[0].Label = strings.Repeat("€", crLabelMax)
	v.Layers[0].Description = strings.Repeat("ü", crDescMax)
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("multi-byte text inside the budget rejected: %v", err)
	}
	v.Layers[0].Label += "€"
	v.Layers[2].Description = strings.Repeat("d", crDescMax+1)
	err := p.Validate(v, nil, nil)
	if err == nil || !errors.As(err, &ve) || ve.Code != ErrCodeMaxLength ||
		!strings.Contains(err.Error(), "layers[0].label") || !strings.Contains(err.Error(), "layers[2].description") {
		t.Errorf("want aggregated max_length errors, got %v", err)
	}

	for _, hl := range []int{-1, 4} {
		v = crValues(3)
		v.Highlight = hl
		if err := p.Validate(v, nil, nil); err == nil || !strings.Contains(err.Error(), "highlight") {
			t.Errorf("highlight %d on 3 layers accepted: %v", hl, err)
		}
	}
	v = crValues(3)
	v.Highlight = 3
	if err := p.Validate(v, nil, nil); err != nil {
		t.Errorf("highlight 3 of 3 rejected: %v", err)
	}

	if err := p.Validate(crValues(3), &ConcentricRingsOverrides{CellAccentMode: "rainbow"}, nil); err == nil {
		t.Error("invalid cell_accent_mode accepted")
	}
	if err := p.Validate(crValues(3), &StateShiftHubOverrides{}, nil); err == nil {
		t.Error("wrong overrides type accepted")
	}
	if err := p.Validate(crValues(3), nil, map[int]any{3: &CellOverride{Emphasis: "bold"}}); err == nil {
		t.Error("cell_overrides index past the last layer accepted")
	}
	if err := p.Validate(crValues(3), nil, map[int]any{0: map[string]any{"fill": "accent2"}}); err == nil {
		t.Error("unknown cell_overrides key accepted")
	}
	if err := p.Validate(crValues(3), nil, map[int]any{2: &CellOverride{Emphasis: "italic"}}); err != nil {
		t.Errorf("valid cell override rejected: %v", err)
	}
}

// The geometry of every count on every shipped content area: rings nested
// and base-aligned, each ladder row exactly as tall as its ring's crest and
// at its height, every frame inside the square, no two lattice rectangles
// overlapping.
func TestConcentricRings_ExpandLayout(t *testing.T) {
	for _, area := range panelAuditAreas {
		for m := crMinLayers; m <= crMaxLayers; m++ {
			t.Run(fmt.Sprintf("%s/%d", area.name, m), func(t *testing.T) {
				ctx := panelAuditCtx(area.w, area.h)
				v := crValues(m)
				lay := crMeasure(ctx, v, &ConcentricRingsOverrides{})
				if !lay.fits() {
					t.Fatalf("exemplar-length copy overflows its row by %.1fpt", lay.overflowPt())
				}
				if lay.side > area.h+1e-6 || lay.squareX < 0 || lay.ladderX+lay.ladderW > area.w+1e-6 {
					t.Fatalf("block leaves the area: side %.1f squareX %.1f ladder %.1f+%.1f", lay.side, lay.squareX, lay.ladderX, lay.ladderW)
				}
				if lay.ladderX < lay.squareX+lay.side {
					t.Errorf("ladder starts at %.1f inside the square (ends %.1f)", lay.ladderX, lay.squareX+lay.side)
				}
				if lay.labelSize < 12 || lay.descSize < 12 {
					t.Errorf("text under the 12pt floor: %v / %v", lay.labelSize, lay.descSize)
				}
				if lay.base < 0 || lay.crest > 1/float64(m)+1e-9 {
					t.Errorf("crest %v base %v", lay.crest, lay.base)
				}

				grid := crExpand(t, ctx, v, nil)
				ring, ladder := crParts(t, grid)
				if ring.Fit != "contain" || ring.Shape != nil {
					t.Errorf("ring cell must be a fit:contain canvas of layers only: fit %q shape %v", ring.Fit, ring.Shape)
				}
				if len(ladder) != m {
					t.Fatalf("%d ladder cells, want %d", len(ladder), m)
				}
				rings := crLayersNamed(ring, "ring-")
				if len(rings) != m {
					t.Fatalf("%d ring layers, want %d", len(rings), m)
				}
				for _, l := range ring.Layers {
					f := l.Frame
					if f.W <= 0 || f.H <= 0 || f.X < 0 || f.Y < 0 || f.X+f.W > 1+1e-9 || f.Y+f.H > 1+1e-9 {
						t.Errorf("layer %s leaves the square: %+v", l.Name, f)
					}
				}
				// rings[0] is the outermost: it fills the square; every next
				// ring is inside the previous one, centred on the same axis.
				if o := rings[0].Frame; math.Abs(o.X) > 1e-4 || math.Abs(o.Y) > 1e-4 || math.Abs(o.W-1) > 1e-4 {
					t.Errorf("outermost ring does not fill the square: %+v", o)
				}
				for i := 1; i < m; i++ {
					out, in := rings[i-1].Frame, rings[i].Frame
					if math.Abs(in.W-in.H) > 1e-9 {
						t.Errorf("ring %d is not a circle: %+v", i, in)
					}
					if math.Abs((in.X+in.W/2)-0.5) > 1e-4 {
						t.Errorf("ring %d is off the vertical axis: %+v", i, in)
					}
					if in.W >= out.W || in.Y <= out.Y || in.Y+in.H > out.Y+out.H+1e-4 {
						t.Errorf("ring %d is not nested in ring %d: %+v in %+v", i, i-1, in, out)
					}
					// One crest between consecutive ring tops.
					if got := in.Y - out.Y; math.Abs(got-lay.crest) > 1e-4 {
						t.Errorf("crest of ring %d = %v, want %v", i-1, got, lay.crest)
					}
				}
				// The core is one crest across, so its row is as tall as the others.
				if core := rings[m-1].Frame; math.Abs(core.W-lay.crest) > 1e-4 {
					t.Errorf("core diameter %v, want one crest %v", core.W, lay.crest)
				}
				// Every dot sits at the middle of its row and inside its own ring.
				for k := 1; k <= m; k++ {
					dots := crLayersNamed(ring, fmt.Sprintf("dot-%d", k))
					if len(dots) != 1 {
						t.Fatalf("ring %d: %d dots", k, len(dots))
					}
					d := dots[0].Frame
					cx, cy := d.X+d.W/2, d.Y+d.H/2
					if want := lay.rowCentre(k, m); math.Abs(cy-want) > 1e-4 {
						t.Errorf("dot %d at y %v, want the row centre %v", k, cy, want)
					}
					f := lay.ringFrame(k, m)
					if r := f.W / 2; math.Hypot(cx-0.5, cy-(f.Y+r)) > r-d.W/2 {
						t.Errorf("dot %d is not inside ring %d", k, k)
					}
					if k > 1 {
						in := lay.ringFrame(k-1, m)
						if r := in.W / 2; math.Hypot(cx-0.5, cy-(in.Y+r)) < r {
							t.Errorf("dot %d sits on the ring inside it", k)
						}
					}
				}
				crAssertLadderAligned(t, grid, lay, m)
			})
		}
	}
}

// crAssertLadderAligned checks the lattice: fixed rows that add up to the
// square's side, and the k-th ladder row covering exactly ring k's crest.
func crAssertLadderAligned(t *testing.T, grid *jsonschema.ShapeGridInput, lay crLayout, m int) {
	t.Helper()
	total := 0.0
	tops := make([]float64, len(grid.Rows))
	for r, row := range grid.Rows {
		if row.MinHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Errorf("row %d is not a fixed height: min %v max %v", r, row.MinHeight, row.MaxHeight)
		}
		tops[r] = total
		total += row.MinHeight
	}
	if math.Abs(total-lay.side) > 0.01 {
		t.Errorf("rows add up to %.2fpt, want the square's side %.2fpt", total, lay.side)
	}
	seen := 0
	for r, row := range grid.Rows {
		for _, c := range row.Cells {
			if c == nil || c.Shape == nil || len(c.Layers) > 0 {
				continue
			}
			// Ladder cells come top-down: the outermost ring first.
			k := m - seen
			seen++
			if want := float64(m-k) * lay.rowPt(); math.Abs(tops[r]-want) > 0.01 {
				t.Errorf("ladder row of ring %d starts at %.2fpt, want %.2fpt", k, tops[r], want)
			}
			if span := max(c.RowSpan, 1); span != 1 {
				t.Errorf("ladder row of ring %d spans %d lattice rows", k, span)
			}
			if math.Abs(row.MinHeight-lay.rowPt()) > 0.01 {
				t.Errorf("ladder row of ring %d is %.2fpt tall, want one crest %.2fpt", k, row.MinHeight, lay.rowPt())
			}
		}
	}
	if seen != m {
		t.Errorf("%d ladder cells on the lattice, want %d", seen, m)
	}
}

// Layers draw in input order, so the rings must go outermost first and every
// leader and dot after the last ring.
func TestConcentricRingsInnerOnTop(t *testing.T) {
	for m := crMinLayers; m <= crMaxLayers; m++ {
		ring, _ := crParts(t, crExpand(t, ExpandContext{}, crValues(m), nil))
		for i := 0; i < m; i++ {
			l := ring.Layers[i]
			if want := fmt.Sprintf("ring-%d", m-i); l.Name != want || l.Shape.Geometry != "ellipse" {
				t.Errorf("m=%d layer %d = %s (%s), want %s", m, i, l.Name, l.Shape.Geometry, want)
			}
			if i > 0 && l.Frame.W >= ring.Layers[i-1].Frame.W {
				t.Errorf("m=%d layer %d is not smaller than the one under it", m, i)
			}
		}
		for _, l := range ring.Layers[m:] {
			if strings.HasPrefix(l.Name, "ring-") {
				t.Errorf("m=%d: %s drawn over a leader", m, l.Name)
			}
		}
		for _, l := range ring.Layers {
			if string(l.Shape.Line) != `"none"` {
				t.Errorf("layer %s carries an outline: %s", l.Name, l.Shape.Line)
			}
			if len(l.Shape.Text) != 0 {
				t.Errorf("layer %s carries text; labels belong on the ladder", l.Name)
			}
		}
	}
}

// Five rings on the shortest shipped content area: a band is 29pt wide, so
// no label is set in it. Every label is a ladder row beside the square, and
// the base step between the rings gives way so each row holds a label and a
// one-line description unshrunk.
func TestConcentricRingsLadderEngagesWhenBandTooThin(t *testing.T) {
	ctx := panelAuditCtx(687, 294)
	v := crValues(5)
	lay := crMeasure(ctx, v, &ConcentricRingsOverrides{})
	if !lay.fits() {
		t.Fatalf("five layers with one-line descriptions overflow by %.1fpt on abstract", lay.overflowPt())
	}
	designed := 1 / (5 + 4*crBaseStepFrac)
	if lay.crest <= designed {
		t.Errorf("crest %v did not grow past the designed %v to hold the rows", lay.crest, designed)
	}
	if lay.base >= designed*crBaseStepFrac {
		t.Errorf("base step %v did not give way", lay.base)
	}
	_, ladder := crParts(t, crExpand(t, ctx, v, nil))
	if len(ladder) != 5 {
		t.Fatalf("%d ladder rows", len(ladder))
	}
	for i, c := range ladder {
		want := v.Layers[4-i].Label
		if !strings.Contains(string(c.Shape.Text), want) {
			t.Errorf("ladder row %d = %s, want the label %q (outermost first)", i, c.Shape.Text, want)
		}
	}
	p := &concentricRings{}
	for _, area := range panelAuditAreas {
		for m := crMinLayers; m <= crMaxLayers; m++ {
			if bad := panelAuditFloorViolations(t, p, panelAuditCtx(area.w, area.h), crValues(m), nil, true); len(bad) > 0 {
				t.Errorf("%s m=%d: text written shrunk: %v", area.name, m, bad)
			}
		}
	}
	// Labels alone leave the designed base step in place.
	bare := crValues(5)
	for i := range bare.Layers {
		bare.Layers[i].Description = ""
	}
	if got := crMeasure(ctx, bare, &ConcentricRingsOverrides{}); math.Abs(got.crest-designed) > 1e-9 {
		t.Errorf("label-only crest %v, want the designed %v", got.crest, designed)
	}
}

func crFillOf(t *testing.T, l jsonschema.LayerInput) fillTone {
	t.Helper()
	tone, ok := parseFillTone(l.Shape.Fill)
	if !ok {
		t.Fatalf("layer %s: unreadable fill %s", l.Name, l.Shape.Fill)
	}
	return tone
}

// crIsTint reports whether tone is a rung of accent's "Lighter N%" ladder: the
// accent lightened (lumOff > 0, lumMod + lumOff = 100%), which is a tint and
// not a solid.
func crIsTint(tone fillTone, accent string) bool {
	return tone.Color == accent && tone.LumOff > 0 && tone.LumMod+tone.LumOff == 100000 && tone.Alpha == 0 && tone.Tint == 0 && tone.Shade == 0
}

func TestConcentricRings_ExpandStyling(t *testing.T) {
	// Default: the core is the one solid accent, every other ring a rung of
	// the accent's Lighter ladder (crTintLadder) that gets lighter outwards.
	if crTintLadder != [crMaxLayers]int{90, 80, 68, 56, 44} {
		t.Errorf("tint ladder = %v, want Lighter 90 / 80 / 68 / 56 / 44%%", crTintLadder)
	}
	for m := crMinLayers; m <= crMaxLayers; m++ {
		ring, _ := crParts(t, crExpand(t, crThemeCtx(0, 0), crValues(m), nil))
		rings := crLayersNamed(ring, "ring-")
		solid, prev := 0, 0
		for i, l := range rings { // outermost first
			tone := crFillOf(t, l)
			if !crIsTint(tone, "accent1") {
				solid++
				if i != m-1 || tone != (fillTone{Color: "accent1"}) {
					t.Errorf("m=%d: solid fill %+v on ring %s, want accent1 on the core only", m, tone, l.Name)
				}
				continue
			}
			// Rank i from the outside is the ladder's rung i.
			if want := (fillTone{Color: "accent1", LumMod: (100 - crTintLadder[i]) * 1000, LumOff: crTintLadder[i] * 1000}); tone != want {
				t.Errorf("m=%d: %s fill %+v, want Lighter %d%% %+v", m, l.Name, tone, crTintLadder[i], want)
			}
			pct := tone.LumMod / 1000
			if pct <= prev {
				t.Errorf("m=%d: tint ladder not darker inwards at %s: %d%% after %d%%", m, l.Name, pct, prev)
			}
			prev = pct
		}
		if solid != 1 {
			t.Errorf("m=%d: %d solid rings, want exactly one", m, solid)
		}
	}

	// highlight moves the solid accent; the ladder text of that layer takes
	// the accent ink and every other label stays dk1.
	v := crValues(4)
	v.Highlight = 3
	grid := crExpand(t, crThemeCtx(0, 0), v, &ConcentricRingsOverrides{Accent: "accent3"})
	ring, ladder := crParts(t, grid)
	for _, l := range crLayersNamed(ring, "ring-") {
		tone := crFillOf(t, l)
		if want := l.Name == "ring-3"; crIsTint(tone, "accent3") == want {
			t.Errorf("%s: fill %+v (highlight is ring 3: the only solid, the others tints of accent3)", l.Name, tone)
		} else if want && tone != (fillTone{Color: "accent3"}) {
			t.Errorf("accent override not applied: %+v, want the solid accent3", tone)
		}
	}
	raw, _ := json.Marshal(grid)
	if strings.Contains(string(raw), `"accent1"`) {
		t.Error("accent override not applied everywhere")
	}
	for i, c := range ladder { // ladder[1] is layer index 2 of 4
		hasAccent := strings.Contains(string(c.Shape.Text), `"color":"accent3"`)
		if hasAccent != (i == 1) {
			t.Errorf("ladder row %d accent label = %v: %s", i, hasAccent, c.Shape.Text)
		}
	}

	// cell_accent_mode against two bases: uniform keeps the tint ladder,
	// alternate and progressive paint every ring the accent of its index.
	for _, base := range []string{"accent1", "accent3"} {
		for _, mode := range []string{CellAccentUniform, CellAccentAlternate, CellAccentProgressive} {
			ring, _ := crParts(t, crExpand(t, crThemeCtx(0, 0), crValues(4), &ConcentricRingsOverrides{Accent: base, CellAccentMode: mode}))
			for _, l := range crLayersNamed(ring, "ring-") {
				var k int
				if _, err := fmt.Sscanf(l.Name, "ring-%d", &k); err != nil {
					t.Fatal(err)
				}
				tone := crFillOf(t, l)
				switch {
				case mode == CellAccentUniform && k == 1:
					if tone != (fillTone{Color: base}) {
						t.Errorf("%s/%s core = %+v, want the solid %s", base, mode, tone, base)
					}
				case mode == CellAccentUniform:
					// Layer k of 4 is rank 4-k from the outside.
					pct := crTintLadder[4-k]
					if want := (fillTone{Color: base, LumMod: (100 - pct) * 1000, LumOff: pct * 1000}); !crIsTint(tone, base) || tone != want {
						t.Errorf("%s/%s %s = %+v, want the accent's Lighter %d%% %+v", base, mode, l.Name, tone, pct, want)
					}
				default:
					if want := ResolveCellAccent(base, k-1, mode); tone != (fillTone{Color: want}) {
						t.Errorf("%s/%s %s = %+v, want %s", base, mode, l.Name, tone, want)
					}
				}
			}
		}
	}

	// Without an override the accent is the context's default; sizes follow
	// header_size / body_size; semantic_accent resolves through the theme.
	ctx := crThemeCtx(0, 0)
	ctx.Theme.SemanticAccents = map[string]string{"positive": "accent6"}
	grid = crExpand(t, ctx, crValues(3), &ConcentricRingsOverrides{SemanticAccent: "positive", HeaderSize: 16, BodySize: 13})
	raw, _ = json.Marshal(grid)
	for _, want := range []string{`"accent6"`, `"size":16`, `"size":13`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("expansion missing %s", want)
		}
	}
	ring, _ = crParts(t, crExpand(t, ExpandContext{}, crValues(3), nil))
	if tone := crFillOf(t, crLayersNamed(ring, "ring-1")[0]); tone.Color != (ExpandContext{}).DefaultAccent() {
		t.Errorf("core fill %+v, want the default accent", tone)
	}

	// A cell override restyles the ladder text of its layer.
	p := &concentricRings{}
	grid, err := p.Expand(ExpandContext{}, crValues(3), nil, map[int]any{0: &CellOverride{FontSize: 18, AccentBar: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, ladder = crParts(t, grid)
	if core := ladder[2]; !strings.Contains(string(core.Shape.Text), `"size":18`) || core.AccentBar == nil {
		t.Errorf("cell override on layers[0] not applied: %s bar %v", core.Shape.Text, core.AccentBar)
	}
	if ladder[0].AccentBar != nil || strings.Contains(string(ladder[0].Shape.Text), `"size":18`) {
		t.Error("cell override leaked to another row")
	}
}

// A leader crosses every ring outside its own; each piece and each dot takes
// the ink that reads on the fill under it, on any template.
func TestConcentricRingsInkMeasuredPerTint(t *testing.T) {
	themes := map[string]ExpandContext{"navy accent": crThemeCtx(0, 0)}
	pale := crThemeCtx(0, 0)
	pale.Theme.Colors = append([]types.ThemeColor(nil), pale.Theme.Colors...)
	for i, c := range pale.Theme.Colors {
		if c.Name == "accent1" {
			pale.Theme.Colors[i].RGB = "#F5C518" // a yellow primary: light ink fails on it
		}
	}
	themes["yellow accent"] = pale

	for name, ctx := range themes {
		for _, hl := range []int{1, 3} {
			v := crValues(5)
			v.Highlight = hl
			lay := crMeasure(ctx, v, &ConcentricRingsOverrides{})
			ring, _ := crParts(t, crExpand(t, ctx, v, &ConcentricRingsOverrides{Accent: "accent1"}))
			tones := make([]fillTone, 6) // by ring number
			for _, l := range crLayersNamed(ring, "ring-") {
				var k int
				fmt.Sscanf(l.Name, "ring-%d", &k)
				tones[k] = crFillOf(t, l)
			}
			under := func(x, y float64) fillTone {
				for k := 1; k <= 5; k++ { // innermost ring that contains the point
					f := lay.ringFrame(k, 5)
					if r := f.W / 2; math.Hypot(x-0.5, y-(f.Y+r)) < r {
						return tones[k]
					}
				}
				return fillTone{Color: "lt1"}
			}
			checked := 0
			for _, l := range ring.Layers {
				if strings.HasPrefix(l.Name, "ring-") {
					continue
				}
				var ink string
				if err := json.Unmarshal(l.Shape.Fill, &ink); err != nil {
					t.Fatalf("%s: fill %s is not a scheme name", l.Name, l.Shape.Fill)
				}
				f := l.Frame
				// Sample just inside both ends: a piece never straddles a ring edge.
				for _, x := range []float64{f.X + f.W*0.02, f.X + f.W*0.98} {
					tone := under(x, f.Y+f.H/2)
					ratio, ok := schemeContrastOnTone(ctx, ink, tone)
					if !ok {
						t.Fatalf("%s: cannot measure %s on %+v", l.Name, ink, tone)
					}
					if ratio < 4.5 {
						t.Errorf("%s hl=%d: %s at x=%.3f is %s on %+v = %.2f:1", name, hl, l.Name, x, ink, tone, ratio)
					}
					checked++
				}
			}
			if checked < 20 {
				t.Errorf("%s hl=%d: only %d ink samples checked", name, hl, checked)
			}
			// The navy highlight takes light ink, its tinted neighbours dark:
			// the choice is per fill, not one colour for the drawing.
			if name == "navy accent" {
				dot := crLayersNamed(ring, fmt.Sprintf("dot-%d", hl))[0]
				other := crLayersNamed(ring, "dot-5")[0]
				if string(dot.Shape.Fill) != `"lt1"` || string(other.Shape.Fill) == `"lt1"` {
					t.Errorf("hl=%d: dot on the accent %s, dot on the outer tint %s", hl, dot.Shape.Fill, other.Shape.Fill)
				}
			}
		}
	}
}

// A half-width cell keeps the rings round (the square shrinks, the ladder
// keeps its minimum) and holds labels; descriptions that cannot fit there
// are reported, never shrunk silently.
func TestConcentricRings_NarrowCell(t *testing.T) {
	p := &concentricRings{}
	for _, area := range []struct{ w, h float64 }{{336, 286}, {440, 352}, {330, 140}} {
		ctx := panelAuditCtx(area.w, area.h)
		for m := crMinLayers; m <= crMaxLayers; m++ {
			v := crValues(m)
			for i := range v.Layers {
				// A half-width ladder holds about 16 characters on a line.
				v.Layers[i] = ConcentricRingsLayer{Label: auditText(16)}
			}
			lay := crMeasure(ctx, v, &ConcentricRingsOverrides{})
			if lay.side > area.h || lay.squareX+lay.side+lay.ladderW > area.w+1e-6 || lay.ladderW < crLadderMinPt-1e-6 {
				t.Errorf("%vx%v m=%d: side %.1f ladder %.1f", area.w, area.h, m, lay.side, lay.ladderW)
			}
			warnings := p.PostExpandWarnings(ctx, v, nil)
			bad := panelAuditFloorViolations(t, p, ctx, v, nil, true)
			if len(bad) > 0 && len(warnings) == 0 {
				t.Errorf("%vx%v m=%d: shrunk without a finding: %v", area.w, area.h, m, bad)
			}
			if area.h >= 286 && (len(bad) > 0 || len(warnings) > 0) {
				t.Errorf("%vx%v m=%d: labels alone must fit a half-width cell: %v %v", area.w, area.h, m, bad, warnings)
			}
		}
	}
}

func TestConcentricRings_PostExpandWarnings(t *testing.T) {
	p := &concentricRings{}
	for _, area := range panelAuditAreas {
		for m := crMinLayers; m <= crMaxLayers; m++ {
			if got := p.PostExpandWarnings(panelAuditCtx(area.w, area.h), crValues(m), nil); len(got) != 0 {
				t.Errorf("%s m=%d: short copy should not warn: %v", area.name, m, got)
			}
		}
	}
	ctx := panelAuditCtx(687, 294)
	v := crValues(5)
	v.Layers[3].Description = auditText(crDescMax) + " and then some more words to run on"
	got := p.PostExpandWarnings(ctx, v, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+":") || !strings.Contains(got[0], "layers[3].description") || !strings.Contains(got[0], "5 layers") {
		t.Errorf("want one BODY_TOO_LONG for layers[3].description, got %v", got)
	}
	// A label that cannot fit names the label.
	narrow := panelAuditCtx(300, 130)
	v = crValues(5)
	for i := range v.Layers {
		v.Layers[i].Description = ""
	}
	got = p.PostExpandWarnings(narrow, v, nil)
	if len(got) == 0 || !strings.Contains(got[0], ".label") || strings.Contains(got[0], ".description") {
		t.Errorf("want label warnings in a 130pt-tall cell, got %v", got)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Errorf("nil values: %v", got)
	}
}

// The copy budget the schema states, measured on every shipped content area
// including p-style: a full-length label with a full-length description of
// realistic copy is written unshrunk and unreported at every layer count.
func TestConcentricRings_CopyBudgets(t *testing.T) {
	p := &concentricRings{}
	for _, area := range panelAuditAreas {
		for m := crMinLayers; m <= crMaxLayers; m++ {
			const descLen = crDescMax
			v := &ConcentricRingsValues{}
			for i := 0; i < m; i++ {
				v.Layers = append(v.Layers, ConcentricRingsLayer{Label: auditText(crLabelMax), Description: auditText(descLen)})
			}
			if err := p.Validate(v, nil, nil); err != nil {
				t.Fatalf("m=%d: %v", m, err)
			}
			ctx := panelAuditCtx(area.w, area.h)
			if got := p.PostExpandWarnings(ctx, v, nil); len(got) != 0 {
				t.Errorf("%s m=%d: a %d-character description at the documented budget warns: %v", area.name, m, descLen, got)
			}
			if bad := panelAuditFloorViolations(t, p, ctx, v, nil, true); len(bad) > 0 {
				t.Errorf("%s m=%d: budget copy written shrunk: %v", area.name, m, bad)
			}
		}
	}
}

func TestConcentricRings_Golden(t *testing.T) {
	p := &concentricRings{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "concentric-rings", "default.golden.json"))
}

func TestConcentricRings_RecommendIntents(t *testing.T) {
	cases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"onion model of the stakeholders", &ContentHints{ItemCount: 4}, "concentric-rings"},
		{"layers of influence around the sponsor", nil, "concentric-rings"},
		{"core adjacent ecosystem view of the portfolio", &ContentHints{ItemCount: 3}, "concentric-rings"},
		{"nested layers of the operating model", &ContentHints{ItemCount: 5}, "concentric-rings"},
		// A ranked hierarchy still belongs to pyramid.
		{"pyramid of needs", &ContentHints{ItemCount: 4}, "pyramid"},
	}
	for _, tc := range cases {
		res := Recommend(Default(), tc.intent, tc.hints, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != tc.want {
			t.Errorf("intent %q: top = %v, want %s", tc.intent, res.Candidates, tc.want)
		}
	}
}
