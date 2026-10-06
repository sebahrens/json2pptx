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
)

var rhTestLabels = []string{"Marketing", "Sales", "Service", "Finance", "Risk and compliance", "Product", "Partners", "Data science"}

func rhValues(n int, descriptions bool) *RadialHubValues {
	v := &RadialHubValues{Center: RadialHubCenter{Label: "Data platform", Sublabel: "One source"}}
	for i := 0; i < n; i++ {
		s := RadialHubSpoke{Label: rhTestLabels[i%len(rhTestLabels)]}
		if descriptions {
			s.Description = "Shared customer record for the team"
		}
		v.Spokes = append(v.Spokes, s)
	}
	return v
}

// rhCtx is the themed test context with a content area of w x h points.
func rhCtx(w, h float64) ExpandContext {
	ctx := fullThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: int64(w * sizingEMUPerPt), Height: int64(h * sizingEMUPerPt)}
	return ctx
}

// The content areas the layout is checked on: the widest shipped body
// (p-style), the smallest (abstract), a 50% compose segment and a square
// nested cell.
var rhAreas = []struct {
	name string
	w, h float64
}{
	{"p-style", 899, 360},
	{"abstract", 687, 294},
	{"half", 400, 330},
	{"square", 300, 300},
}

func rhExpand(t *testing.T, ctx ExpandContext, v *RadialHubValues, ovr *RadialHubOverrides) *jsonschema.ShapeGridInput {
	t.Helper()
	var overrides any
	if ovr != nil {
		overrides = ovr
	}
	p := &radialHub{}
	if err := p.Validate(v, overrides, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ctx, v, overrides, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return grid
}

// rhRingCell is the one cell that carries layers.
func rhRingCell(t *testing.T, grid *jsonschema.ShapeGridInput) *jsonschema.GridCellInput {
	t.Helper()
	var ring *jsonschema.GridCellInput
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			if c != nil && len(c.Layers) > 0 {
				if ring != nil {
					t.Fatal("more than one cell carries layers")
				}
				ring = c
			}
		}
	}
	if ring == nil {
		t.Fatal("no ring cell")
	}
	return ring
}

func rhLayersNamed(ring *jsonschema.GridCellInput, prefix string) []jsonschema.LayerInput {
	var out []jsonschema.LayerInput
	for _, l := range ring.Layers {
		if strings.HasPrefix(l.Name, prefix) {
			out = append(out, l)
		}
	}
	return out
}

// rhTextCells are the lattice cells that hold text, in grid order.
func rhTextCells(grid *jsonschema.ShapeGridInput) []*jsonschema.GridCellInput {
	var out []*jsonschema.GridCellInput
	for _, row := range grid.Rows {
		for _, c := range row.Cells {
			if c != nil && c.Shape != nil && len(c.Shape.Text) > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func rhErrCodes(err error) map[string]string {
	codes := map[string]string{}
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			for _, sub := range joined.Unwrap() {
				walk(sub)
			}
			return
		}
		var ve *ValidationError
		if errors.As(e, &ve) {
			codes[ve.Path] = ve.Code
		}
	}
	walk(err)
	return codes
}

func TestRadialHub_Metadata(t *testing.T) {
	p, ok := Default().Get("radial-hub")
	if !ok {
		t.Fatal("radial-hub not registered")
	}
	if p.Version() != 1 || p.CellsHint() == "" || p.Description() == "" {
		t.Errorf("metadata incomplete: version %d, cells %q", p.Version(), p.CellsHint())
	}
	for _, sibling := range []string{"cycle-ring", "cycle-nodes", "state-shift-hub", "driver-tree", "card-grid", "icon-row", "concentric-rings"} {
		if !strings.Contains(p.NotWhen(), sibling) {
			t.Errorf("NotWhen does not name %s", sibling)
		}
	}
	if !strings.Contains(p.UseWhen(), "prefer") {
		t.Error("UseWhen is not contrastive")
	}
	if tax := p.Taxonomy(); tax.Category != "structural" || tax.DataVisual {
		t.Errorf("taxonomy = %+v", tax)
	}
	if MotifFor("radial-hub", nil, nil) != MotifDiagram {
		t.Errorf("motif = %q, want diagram", MotifFor("radial-hub", nil, nil))
	}
	ex, ok := p.(Exemplar)
	if !ok {
		t.Fatal("no exemplar")
	}
	if err := p.Validate(ex.ExemplarValues(), nil, nil); err != nil {
		t.Errorf("exemplar does not validate: %v", err)
	}
}

func TestRadialHub_SchemaIsValidJSON(t *testing.T) {
	data, err := json.Marshal((&radialHub{}).Schema())
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > 6000 {
		t.Errorf("schema is %d bytes, over 6000", len(data))
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	defs, _ := m["$defs"].(map[string]any)
	if _, ok := defs["spoke"]; !ok {
		t.Error("schema has no $defs.spoke")
	}
	for _, want := range []string{`"maxItems":8`, `"minItems":4`, `"labels"`, `"legend"`, `"cell_accent_mode"`, `"highlight"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("schema lacks %s", want)
		}
	}
}

func TestRadialHub_Validate(t *testing.T) {
	p := &radialHub{}
	for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
		if err := p.Validate(rhValues(n, true), nil, nil); err != nil {
			t.Errorf("%d spokes: %v", n, err)
		}
	}

	t.Run("count", func(t *testing.T) {
		err := p.Validate(rhValues(3, false), nil, nil)
		if err == nil || !strings.Contains(err.Error(), "icon-row") {
			t.Errorf("3 spokes: %v, want a min-items error with the sibling hint", err)
		}
		err = p.Validate(rhValues(9, false), nil, nil)
		if err == nil || !strings.Contains(err.Error(), "card-grid") {
			t.Errorf("9 spokes: %v, want a max-items error with the sibling hint", err)
		}
	})

	t.Run("required", func(t *testing.T) {
		v := rhValues(4, false)
		v.Center.Label = " "
		v.Spokes[2].Label = ""
		codes := rhErrCodes(p.Validate(v, nil, nil))
		if codes["center.label"] != ErrCodeRequired || codes["spokes[2].label"] != ErrCodeRequired {
			t.Errorf("codes = %v, want required on center.label and spokes[2].label", codes)
		}
	})

	t.Run("max length counts runes", func(t *testing.T) {
		v := rhValues(4, false)
		v.Center.Label = strings.Repeat("é", rhHubLabelMax)             // 24 runes, 48 bytes
		v.Center.Sublabel = strings.Repeat("ü", rhHubSublabelMax)       // 32 runes
		v.Spokes[0].Label = strings.Repeat("ß", rhLabelMax)             // 26 runes
		v.Spokes[0].Description = strings.Repeat("ø", rhDescriptionMax) // 60 runes
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("multi-byte values inside the budgets were refused: %v", err)
		}
		v.Center.Label += "x"
		v.Center.Sublabel += "x"
		v.Spokes[0].Label += "x"
		v.Spokes[0].Description += "x"
		codes := rhErrCodes(p.Validate(v, nil, nil))
		for _, path := range []string{"center.label", "center.sublabel", "spokes[0].label", "spokes[0].description"} {
			if codes[path] != ErrCodeMaxLength {
				t.Errorf("%s: code %q, want %s", path, codes[path], ErrCodeMaxLength)
			}
		}
	})

	t.Run("overrides", func(t *testing.T) {
		for _, ovr := range []*RadialHubOverrides{
			{Labels: "around"}, {Spokes: "arrows"}, {TextOverrides: TextOverrides{CellAccentMode: "rainbow"}},
		} {
			if err := p.Validate(rhValues(4, false), ovr, nil); err == nil {
				t.Errorf("%+v was accepted", ovr)
			}
		}
		for _, ovr := range []*RadialHubOverrides{
			{Labels: "outside"}, {Labels: "legend"}, {Spokes: "none"}, {Spokes: "lines"},
			{TextOverrides: TextOverrides{CellAccentMode: "progressive", HeaderSize: 16, BodySize: 12, Accent: "accent3"}},
		} {
			if err := p.Validate(rhValues(5, true), ovr, nil); err != nil {
				t.Errorf("%+v: %v", ovr, err)
			}
		}
		if err := p.Validate(rhValues(4, false), &StateShiftHubOverrides{}, nil); err == nil {
			t.Error("a foreign overrides type was accepted")
		}
	})

	t.Run("labels inside", func(t *testing.T) {
		inside := &RadialHubOverrides{Labels: rhLabelsInside}
		v := rhValues(4, false) // "Marketing" .. "Finance": all within 14
		if err := p.Validate(v, inside, nil); err != nil {
			t.Errorf("short labels inside: %v", err)
		}
		codes := rhErrCodes(p.Validate(rhValues(5, false), inside, nil)) // "Risk and compliance" is 19
		if codes["spokes[4].label"] != ErrCodeMaxLength {
			t.Errorf("19-character label inside: codes %v", codes)
		}
		codes = rhErrCodes(p.Validate(rhValues(4, true), inside, nil))
		if codes["spokes[0].description"] != ErrCodeUnknownKey {
			t.Errorf("description with labels inside: codes %v", codes)
		}
		seven := rhValues(7, false)
		for i := range seven.Spokes {
			seven.Spokes[i].Label = "Team"
		}
		if err := p.Validate(seven, inside, nil); err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("7 spokes inside: %v, want a max-items error pointing at labels outside", err)
		}
	})

	t.Run("highlight and cell overrides", func(t *testing.T) {
		v := rhValues(5, false)
		v.Highlight = 5
		if err := p.Validate(v, nil, nil); err != nil {
			t.Errorf("highlight 5 of 5: %v", err)
		}
		v.Highlight = 6
		if err := p.Validate(v, nil, nil); err == nil {
			t.Error("highlight 6 of 5 was accepted")
		}
		v.Highlight = -1
		if err := p.Validate(v, nil, nil); err == nil {
			t.Error("highlight -1 was accepted")
		}
		codes := rhErrCodes(p.Validate(rhValues(4, false), nil, map[int]any{0: &struct{}{}}))
		if codes["cell_overrides"] != ErrCodeUnknownKey {
			t.Errorf("cell_overrides: codes %v", codes)
		}
	})
}

// Geometry of the layers for every count: one hub, one spoke and one
// satellite per item, satellites at the middles of a ring of N that starts at
// 12 o'clock, every frame inside the cell.
func TestRadialHub_ExpandLayout(t *testing.T) {
	for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			grid := rhExpand(t, rhCtx(899, 360), rhValues(n, true), nil)
			ring := rhRingCell(t, grid)
			if ring.Fit != "contain" || ring.Shape != nil {
				t.Errorf("ring cell: fit %q, shape %v; want a contain canvas with layers only", ring.Fit, ring.Shape)
			}
			spokes, sats, hubs := rhLayersNamed(ring, "spoke-"), rhLayersNamed(ring, "satellite-"), rhLayersNamed(ring, "hub")
			if len(spokes) != n || len(sats) != n || len(hubs) != 1 || len(ring.Layers) != 2*n+1 {
				t.Fatalf("layers: %d spokes, %d satellites, %d hubs of %d", len(spokes), len(sats), len(hubs), len(ring.Layers))
			}
			// Spokes are drawn first, so the hub and the satellites cover their ends.
			if !strings.HasPrefix(ring.Layers[0].Name, "spoke-") || ring.Layers[n].Name != "hub" {
				t.Errorf("z-order: first %q, layer %d %q", ring.Layers[0].Name, n, ring.Layers[n].Name)
			}
			for _, l := range ring.Layers {
				f := l.Frame
				if f.W <= 0 || f.H <= 0 || f.X < -1e-4 || f.Y < -1e-4 || f.X+f.W > 1+1e-4 || f.Y+f.H > 1+1e-4 {
					t.Errorf("%s: frame %+v leaves the cell", l.Name, f)
				}
				if l.Shape == nil || string(l.Shape.Line) != `"none"` {
					t.Errorf("%s: a filled layer must carry no outline", l.Name)
				}
			}
			hub := hubs[0].Frame
			if math.Abs(hub.W-hub.H) > 1e-9 || math.Abs(hub.X+hub.W/2-0.5) > 1e-6 || math.Abs(hub.Y+hub.H/2-0.5) > 1e-6 {
				t.Errorf("hub %+v is not a centred circle", hub)
			}
			step := 360.0 / float64(n)
			for i, s := range sats {
				f := s.Frame
				if math.Abs(f.W-rhSatDia) > 1e-6 || math.Abs(f.H-rhSatDia) > 1e-6 {
					t.Errorf("satellite %d: %+v, want a circle of %.2f", i+1, f, rhSatDia)
				}
				wantX, wantY := pointOnCircle(0.5, 0.5, 0.5-rhSatDia/2, -90+step*(float64(i)+0.5))
				if math.Abs(f.X+f.W/2-wantX) > 1e-5 || math.Abs(f.Y+f.H/2-wantY) > 1e-5 {
					t.Errorf("satellite %d centre (%.4f, %.4f), want (%.4f, %.4f)", i+1, f.X+f.W/2, f.Y+f.H/2, wantX, wantY)
				}
				if len(s.Shape.Text) != 0 {
					t.Errorf("satellite %d carries text with labels outside: %s", i+1, s.Shape.Text)
				}
			}
			// One label cell per item, none numbered.
			texts := rhTextCells(grid)
			if len(texts) != n {
				t.Fatalf("%d label cells, want %d", len(texts), n)
			}
			for _, c := range texts {
				if strings.ContainsAny(string(c.Shape.Text), "0123456789") && !strings.Contains(string(c.Shape.Text), `"size"`) {
					t.Errorf("label carries a number: %s", c.Shape.Text)
				}
			}
			if grid.VerticalAlign != "center" {
				t.Errorf("vertical_align = %q", grid.VerticalAlign)
			}
		})
	}
}

// Label sides: an even count has as many labels left as right and none at a
// pole; an odd count labels its 6 o'clock item below the ring.
func TestRadialHub_LabelSides(t *testing.T) {
	for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
		lay, err := rhMeasure(rhCtx(899, 360), rhValues(n, true), &RadialHubOverrides{})
		if err != nil || lay.mode != rhLabelsOutside {
			t.Fatalf("n=%d: mode %q, err %v", n, lay.mode, err)
		}
		count := map[string]int{}
		for _, it := range lay.items {
			count[it.side]++
			switch it.side {
			case ringSideLeft:
				if it.text.x1 > lay.ring.x0 {
					t.Errorf("n=%d: left label %+v enters the ring %+v", n, it.text, lay.ring)
				}
			case ringSideRight:
				if it.text.x0 < lay.ring.x1 {
					t.Errorf("n=%d: right label %+v enters the ring %+v", n, it.text, lay.ring)
				}
			case ringSideBottom:
				if it.text.y0 < lay.ring.y1 {
					t.Errorf("n=%d: bottom label %+v enters the ring %+v", n, it.text, lay.ring)
				}
			}
		}
		wantBottom := n % 2
		if count[ringSideTop] != 0 || count[ringSideBottom] != wantBottom || count[ringSideLeft] != count[ringSideRight] {
			t.Errorf("n=%d: sides %v, want %d below and the rest split evenly", n, count, wantBottom)
		}
		if side := lay.side(); math.Abs(lay.ring.h()-side) > 1e-6 || side < ringMinSidePt {
			t.Errorf("n=%d: ring %+v is not a square of at least %.0fpt", n, lay.ring, ringMinSidePt)
		}
	}
}

// Each spoke's two ends lie on the hub's and its satellite's circumference.
func TestRadialHubSpokesTouchBothEnds(t *testing.T) {
	for _, mode := range []string{rhLabelsOutside, rhLabelsInside, rhLabelsLegend} {
		for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
			if mode == rhLabelsInside && n > rhMaxInsideSpokes {
				continue
			}
			ctx, v, ovr := rhCtx(687, 294), rhValues(n, false), &RadialHubOverrides{Labels: mode}
			for i := range v.Spokes {
				v.Spokes[i].Label = rhTestLabels[i][:4]
			}
			lay, err := rhMeasure(ctx, v, ovr)
			if err != nil {
				t.Fatal(err)
			}
			ring := rhRingCell(t, rhExpand(t, ctx, v, ovr))
			hub := rhLayersNamed(ring, "hub")[0].Frame
			sats := rhLayersNamed(ring, "satellite-")
			onePt := 1 / lay.side()
			for i, sp := range rhLayersNamed(ring, "spoke-") {
				if sp.Shape.Geometry != "rect" {
					t.Fatalf("spoke %d is a %s", i+1, sp.Shape.Geometry)
				}
				cx, cy := sp.Frame.X+sp.Frame.W/2, sp.Frame.Y+sp.Frame.H/2
				sin, cos := math.Sincos(degToRad(sp.Shape.Rotation))
				inner := math.Hypot(cx-cos*sp.Frame.W/2-0.5, cy-sin*sp.Frame.W/2-0.5)
				outerX, outerY := cx+cos*sp.Frame.W/2, cy+sin*sp.Frame.W/2
				sat := sats[i].Frame
				toSat := math.Hypot(outerX-(sat.X+sat.W/2), outerY-(sat.Y+sat.H/2))
				if math.Abs(inner-hub.W/2) > onePt {
					t.Errorf("%s n=%d spoke %d starts %.4f from the centre, hub radius %.4f", mode, n, i+1, inner, hub.W/2)
				}
				if math.Abs(toSat-sat.W/2) > onePt {
					t.Errorf("%s n=%d spoke %d ends %.4f from its satellite's centre, radius %.4f", mode, n, i+1, toSat, sat.W/2)
				}
				if stroke := sp.Frame.H * lay.side(); math.Abs(stroke-rhSpokePt) > 0.01 {
					t.Errorf("%s n=%d spoke %d is %.2fpt thick, want %.1f", mode, n, i+1, stroke, rhSpokePt)
				}
			}
		}
	}
}

func rhSolidAccentLayers(ring *jsonschema.GridCellInput, minFrac float64) []string {
	var out []string
	for _, l := range ring.Layers {
		var s string
		if json.Unmarshal(l.Shape.Fill, &s) == nil && strings.HasPrefix(s, "accent") && math.Min(l.Frame.W, l.Frame.H) >= minFrac {
			out = append(out, l.Name)
		}
	}
	return out
}

// By default the hub is the one solid accent block: the satellites are
// neutral tints, the highlighted one a tint of the accent, and the spokes are
// too thin to be blocks.
func TestRadialHubHubIsTheOnlySolidAccent(t *testing.T) {
	for _, hl := range []int{0, 3} {
		for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
			v := rhValues(n, true)
			v.Highlight = hl
			ring := rhRingCell(t, rhExpand(t, rhCtx(899, 360), v, nil))
			// 0.1 of a 360pt square is 0.5in, the accent-restraint block size.
			if got := rhSolidAccentLayers(ring, 0.1); len(got) != 1 || got[0] != "hub" {
				t.Errorf("highlight %d, n=%d: solid accent blocks %v, want the hub only", hl, n, got)
			}
			for i, s := range rhLayersNamed(ring, "satellite-") {
				tone, ok := opaqueFillTone(s.Shape.Fill)
				if !ok {
					t.Fatalf("satellite %d fill %s", i+1, s.Shape.Fill)
				}
				if hl == i+1 {
					if tone.Color != "accent1" || tone.Tint == 0 {
						t.Errorf("highlighted satellite fill %s, want a tint of the accent", s.Shape.Fill)
					}
				} else if tone.Color != "dk1" || tone.LumMod == 0 {
					t.Errorf("satellite %d fill %s, want a neutral tint", i+1, s.Shape.Fill)
				}
			}
		}
	}
}

func TestRadialHub_ExpandStyling(t *testing.T) {
	ctx := rhCtx(899, 360)

	t.Run("default accent and override", func(t *testing.T) {
		ring := rhRingCell(t, rhExpand(t, ctx, rhValues(6, true), nil))
		if got := string(rhLayersNamed(ring, "hub")[0].Shape.Fill); got != fmt.Sprintf("%q", ctx.DefaultAccent()) {
			t.Errorf("hub fill %s, want the default accent %s", got, ctx.DefaultAccent())
		}
		ring = rhRingCell(t, rhExpand(t, ctx, rhValues(6, true), &RadialHubOverrides{TextOverrides: TextOverrides{Accent: "accent3"}}))
		hub := rhLayersNamed(ring, "hub")[0]
		if string(hub.Shape.Fill) != `"accent3"` || string(rhLayersNamed(ring, "spoke-")[0].Shape.Fill) != `"accent3"` {
			t.Errorf("accent override: hub %s, spoke %s", hub.Shape.Fill, rhLayersNamed(ring, "spoke-")[0].Shape.Fill)
		}
		// accent3 is a light amber in the test theme: white fails on it, so the
		// hub's ink is chosen by measurement.
		want := readableTextOn(ctx, fillTone{Color: "accent3"}, "lt1")
		if want == "lt1" || !strings.Contains(string(hub.Shape.Text), fmt.Sprintf(`"color":%q`, want)) {
			t.Errorf("hub ink on accent3: want measured %q in %s", want, hub.Shape.Text)
		}
	})

	t.Run("cell accent mode", func(t *testing.T) {
		for _, base := range []string{"accent1", "accent3"} {
			for _, mode := range []string{CellAccentUniform, CellAccentAlternate, CellAccentProgressive} {
				ovr := &RadialHubOverrides{TextOverrides: TextOverrides{Accent: base, CellAccentMode: mode}}
				ring := rhRingCell(t, rhExpand(t, ctx, rhValues(6, false), ovr))
				for i, s := range rhLayersNamed(ring, "satellite-") {
					want := string(neutralFillJSON(NeutralTint16))
					if mode != CellAccentUniform {
						want = fmt.Sprintf("%q", ResolveCellAccent(base, i, mode))
					}
					if string(s.Shape.Fill) != want {
						t.Errorf("%s %s satellite %d: fill %s, want %s", base, mode, i+1, s.Shape.Fill, want)
					}
				}
			}
		}
	})

	t.Run("highlight", func(t *testing.T) {
		v := rhValues(6, true)
		v.Highlight = 2
		grid := rhExpand(t, ctx, v, nil)
		ring := rhRingCell(t, grid)
		lay, _ := rhMeasure(ctx, v, &RadialHubOverrides{})
		for i, sp := range rhLayersNamed(ring, "spoke-") {
			stroke := sp.Frame.H * lay.side()
			if i == 1 {
				if string(sp.Shape.Fill) != `"accent1"` || math.Abs(stroke-rhSpokeHiPt) > 0.01 {
					t.Errorf("highlighted spoke: fill %s, %.2fpt", sp.Shape.Fill, stroke)
				}
			} else if string(sp.Shape.Fill) != string(neutralFillJSON(rhSpokeTone)) || math.Abs(stroke-rhSpokePt) > 0.01 {
				t.Errorf("spoke %d beside a highlight: fill %s, %.2fpt; want neutral", i+1, sp.Shape.Fill, stroke)
			}
		}
		accentLabels := 0
		for _, c := range rhTextCells(grid) {
			if strings.Contains(string(c.Shape.Text), `"color":"accent1"`) {
				accentLabels++
				if !strings.Contains(string(c.Shape.Text), "Sales") {
					t.Errorf("accent label is not the highlighted item: %s", c.Shape.Text)
				}
			}
		}
		if accentLabels != 1 {
			t.Errorf("%d accent labels, want the highlighted one", accentLabels)
		}
	})

	t.Run("spokes none", func(t *testing.T) {
		ring := rhRingCell(t, rhExpand(t, ctx, rhValues(5, true), &RadialHubOverrides{Spokes: rhSpokesNone}))
		if n := len(rhLayersNamed(ring, "spoke-")); n != 0 || len(ring.Layers) != 6 {
			t.Errorf("%d spokes of %d layers, want none of 6", n, len(ring.Layers))
		}
	})

	t.Run("sizes", func(t *testing.T) {
		grid := rhExpand(t, ctx, rhValues(4, true), &RadialHubOverrides{TextOverrides: TextOverrides{HeaderSize: 18, BodySize: 14}})
		for _, c := range rhTextCells(grid) {
			if !strings.Contains(string(c.Shape.Text), `"size":18`) || !strings.Contains(string(c.Shape.Text), `"size":14`) {
				t.Errorf("label sizes not applied: %s", c.Shape.Text)
			}
		}
		// Every size the pattern writes is at the 12pt floor or above.
		for _, n := range []int{4, 8} {
			data, _ := json.Marshal(rhExpand(t, rhCtx(687, 294), rhValues(n, true), nil))
			for _, part := range strings.Split(string(data), `"size":`)[1:] {
				var size float64
				if _, err := fmt.Sscanf(part, "%f", &size); err != nil || size < 12 {
					t.Errorf("n=%d: text size %v (%v) under the 12pt floor", n, size, err)
				}
			}
		}
	})
}

// labels inside and legend, and the default's fall-back to the legend.
func TestRadialHub_LabelModes(t *testing.T) {
	t.Run("inside", func(t *testing.T) {
		v := rhValues(6, false)
		v.Spokes[4].Label = "Risk"
		ctx := rhCtx(687, 294)
		grid := rhExpand(t, ctx, v, &RadialHubOverrides{Labels: rhLabelsInside})
		if n := len(rhTextCells(grid)); n != 0 {
			t.Errorf("%d label cells with labels inside", n)
		}
		ring := rhRingCell(t, grid)
		for i, s := range rhLayersNamed(ring, "satellite-") {
			if math.Abs(s.Frame.W-rhInsideSatDia) > 1e-6 || !strings.Contains(string(s.Shape.Text), v.Spokes[i].Label) {
				t.Errorf("satellite %d: %+v %s", i+1, s.Frame, s.Shape.Text)
			}
		}
		lay, _ := rhMeasure(ctx, v, &RadialHubOverrides{Labels: rhLabelsInside})
		if lay.side() != 294 {
			t.Errorf("ring side %.1f, want the full height", lay.side())
		}
	})

	t.Run("legend", func(t *testing.T) {
		ctx, v, ovr := rhCtx(687, 294), rhValues(5, true), &RadialHubOverrides{Labels: rhLabelsLegend}
		grid := rhExpand(t, ctx, v, ovr)
		ring := rhRingCell(t, grid)
		for i, s := range rhLayersNamed(ring, "satellite-") {
			if !strings.Contains(string(s.Shape.Text), fmt.Sprintf(`"content":%q`, rhKey(i))) {
				t.Errorf("satellite %d is not keyed %s: %s", i+1, rhKey(i), s.Shape.Text)
			}
		}
		if n := len(rhTextCells(grid)); n != 10 {
			t.Errorf("%d legend cells, want a key and a label per item", n)
		}
		lay, _ := rhMeasure(ctx, v, ovr)
		for i, it := range lay.items {
			if it.text.x0 < lay.ring.x1 || (i > 0 && it.text.y0 < lay.items[i-1].text.y1) {
				t.Errorf("legend row %d %+v is not stacked right of the ring %+v", i, it.text, lay.ring)
			}
		}
	})

	t.Run("legend wraps into two columns", func(t *testing.T) {
		ctx, v, ovr := rhCtx(687, 294), rhValues(8, true), &RadialHubOverrides{Labels: rhLabelsLegend}
		lay, err := rhMeasure(ctx, v, ovr)
		if err != nil {
			t.Fatal(err)
		}
		if lay.items[0].text.x0 >= lay.items[4].text.x0 || lay.items[0].text.y0 != lay.items[4].text.y0 {
			t.Errorf("8 described rows did not wrap: row 1 %+v, row 5 %+v", lay.items[0].text, lay.items[4].text)
		}
		if w := (&radialHub{}).PostExpandWarnings(ctx, v, ovr); len(w) != 0 {
			t.Errorf("two legend columns still overflow: %v", w)
		}
	})

	t.Run("default falls back to the legend in a narrow area", func(t *testing.T) {
		for _, area := range rhAreas {
			lay, err := rhMeasure(rhCtx(area.w, area.h), rhValues(6, false), &RadialHubOverrides{})
			if err != nil {
				t.Fatal(err)
			}
			want := rhLabelsOutside
			if area.w < 420 {
				want = rhLabelsLegend
			}
			if lay.mode != want {
				t.Errorf("%s (%.0fx%.0f): mode %s, want %s", area.name, area.w, area.h, lay.mode, want)
			}
			// A 150pt ring (the square cell) has no room for a two-line hub.
			if !lay.hubFits && area.w >= 400 {
				t.Errorf("%s: the hub does not hold its label", area.name)
			}
		}
	})
}

// The hub grows towards the satellites before its label is left to shrink.
func TestRadialHub_HubGrowsToHoldItsLabel(t *testing.T) {
	ctx := rhCtx(687, 294)
	short := rhValues(6, true)
	short.Center = RadialHubCenter{Label: "Core"}
	lay, _ := rhMeasure(ctx, short, &RadialHubOverrides{})
	if lay.hubDia != rhHubDia || lay.hubSize != rhHubPt || !lay.hubFits {
		t.Errorf("short label: hub %.2f at %.0fpt (fits %v), want the default hub at the ceiling", lay.hubDia, lay.hubSize, lay.hubFits)
	}
	// A longer label in a shorter area: somewhere between the area that holds
	// it in the default hub and the one that cannot hold it at all, the hub
	// grows and the label stays at 12pt or above.
	long := rhValues(6, true)
	long.Center = RadialHubCenter{Label: "Customer data platform", Sublabel: "One governed source"}
	grown := 0
	for h := 300.0; h >= 150; h -= 10 {
		lay, _ = rhMeasure(rhCtx(687, h), long, &RadialHubOverrides{Labels: rhLabelsOutside})
		if lay.mode != rhLabelsOutside || !lay.hubFits || lay.hubDia <= rhHubDia {
			continue
		}
		grown++
		if lay.hubSize < 12 {
			t.Errorf("height %.0f: grown hub writes its label at %.0fpt", h, lay.hubSize)
		}
		if spoke := lay.spec.Radius - lay.satDia/2 - lay.hubDia/2; spoke < rhMinSpoke-1e-9 {
			t.Errorf("height %.0f: grown hub leaves a %.3f spoke, under %.2f", h, spoke, rhMinSpoke)
		}
	}
	if grown == 0 {
		t.Error("the hub never grew to hold a long label")
	}
}

// No two lattice rectangles overlap, and every rectangle stays in the block,
// for every count, label mode and content area.
func TestRadialHubNoOverlapForEveryCount(t *testing.T) {
	for _, area := range rhAreas {
		for _, mode := range []string{"", rhLabelsInside, rhLabelsLegend} {
			for n := rhMinSpokes; n <= rhMaxSpokes; n++ {
				if mode == rhLabelsInside && n > rhMaxInsideSpokes {
					continue
				}
				for _, described := range []bool{false, true} {
					if mode == rhLabelsInside && described {
						continue
					}
					name := fmt.Sprintf("%s/%s/n=%d/desc=%v", area.name, mode, n, described)
					ctx, v, ovr := rhCtx(area.w, area.h), rhValues(n, described), &RadialHubOverrides{Labels: mode}
					if mode == rhLabelsInside {
						for i := range v.Spokes {
							v.Spokes[i].Label = rhTestLabels[i][:4]
						}
					}
					lay, err := rhMeasure(ctx, v, ovr)
					if err != nil {
						t.Fatalf("%s: %v", name, err)
					}
					rects := []rhRect{lay.ring}
					for _, it := range lay.items {
						if it.text.w() > 0 {
							rects = append(rects, it.text)
						}
					}
					for i, a := range rects {
						if a.x0 < -0.01 || a.y0 < -0.01 || a.x1 > lay.width+0.01 || a.y1 > lay.height+0.01 || a.w() <= 0 || a.h() <= 0 {
							t.Errorf("%s: rectangle %d %+v leaves the %.0fx%.0f block", name, i, a, lay.width, lay.height)
						}
						for j := i + 1; j < len(rects); j++ {
							b := rects[j]
							if a.x0 < b.x1-0.01 && b.x0 < a.x1-0.01 && a.y0 < b.y1-0.01 && b.y0 < a.y1-0.01 {
								t.Errorf("%s: rectangles %d %+v and %d %+v overlap", name, i, a, j, b)
							}
						}
					}
					if math.Abs(lay.ring.w()-lay.ring.h()) > 1e-6 {
						t.Errorf("%s: ring %+v is not square", name, lay.ring)
					}
					// The lattice refuses overlapping placements too.
					if _, err := (&radialHub{}).Expand(ctx, v, ovr, nil); err != nil {
						t.Errorf("%s: Expand: %v", name, err)
					}
				}
			}
		}
	}
}

func TestRadialHub_PostExpandWarnings(t *testing.T) {
	p := &radialHub{}
	for _, area := range rhAreas[:2] {
		for _, n := range []int{4, 6, 8} {
			if w := p.PostExpandWarnings(rhCtx(area.w, area.h), rhValues(n, true), nil); len(w) != 0 {
				t.Errorf("%s n=%d: short copy warns: %v", area.name, n, w)
			}
		}
	}
	if w := p.PostExpandWarnings(fullThemeCtx(), p.ExemplarValues(), nil); len(w) != 0 {
		t.Errorf("exemplar warns: %v", w)
	}

	t.Run("label rows", func(t *testing.T) {
		v := rhValues(8, true)
		for i := range v.Spokes {
			v.Spokes[i].Label = strings.Repeat("Wide label ", 3)[:rhLabelMax]
			v.Spokes[i].Description = strings.Repeat("a long description of the item ", 2)[:rhDescriptionMax]
		}
		w := p.PostExpandWarnings(rhCtx(500, 200), v, nil)
		if len(w) == 0 {
			t.Fatal("eight maximum-length rows in a 500x200 area do not warn")
		}
		for _, msg := range w {
			if !strings.HasPrefix(msg, ErrCodeBodyTooLong+": radial-hub ") {
				t.Errorf("warning shape: %q", msg)
			}
		}
		if !strings.Contains(strings.Join(w, "\n"), "spokes[0].description needs") || !strings.Contains(w[0], "8 spokes") {
			t.Errorf("warnings do not name the item and the count: %v", w)
		}
	})

	t.Run("hub", func(t *testing.T) {
		v := rhValues(6, false)
		v.Center = RadialHubCenter{Label: "Interoperability", Sublabel: "Standardisation everywhere"}
		w := p.PostExpandWarnings(rhCtx(300, 200), v, nil)
		if len(w) == 0 || !strings.Contains(strings.Join(w, "\n"), "center.label does not fit") {
			t.Errorf("a hub label that cannot fit does not warn: %v", w)
		}
	})

	t.Run("inside", func(t *testing.T) {
		v := rhValues(6, false)
		for i := range v.Spokes {
			v.Spokes[i].Label = "Sales"
		}
		v.Spokes[1].Label = "Infrastructure"
		v.Center = RadialHubCenter{Label: "Core"}
		w := p.PostExpandWarnings(rhCtx(300, 200), v, &RadialHubOverrides{Labels: rhLabelsInside})
		if len(w) != 1 || !strings.HasPrefix(w[0], ErrCodeNodeLabelTooLong+": radial-hub spokes[1].label does not fit") || !strings.Contains(w[0], `"outside"`) {
			t.Errorf("warnings = %v, want one NODE_LABEL_TOO_LONG for the label that does not fit its satellite", w)
		}
	})

	if w := p.PostExpandWarnings(fullThemeCtx(), rhValues(2, false), nil); w != nil {
		t.Errorf("an invalid count warns instead of leaving it to Validate: %v", w)
	}
}

func TestRadialHub_ExpandRejectsBadInput(t *testing.T) {
	p := &radialHub{}
	if _, err := p.Expand(fullThemeCtx(), rhValues(3, false), nil, nil); err == nil {
		t.Error("3 spokes expanded")
	}
	if _, err := p.Expand(fullThemeCtx(), &StateShiftHubValues{}, nil, nil); err == nil {
		t.Error("foreign values expanded")
	}
	if _, err := p.Expand(fullThemeCtx(), rhValues(4, false), &StateShiftHubOverrides{}, nil); err == nil {
		t.Error("foreign overrides expanded")
	}
}

func TestRadialHub_Golden(t *testing.T) {
	p := &radialHub{}
	grid, err := p.Expand(ExpandContext{}, p.ExemplarValues(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPatternGolden(t, grid, filepath.Join("testdata", "radial-hub", "default.golden.json"))
}

func TestRadialHub_RecommendIntents(t *testing.T) {
	cases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"hub and spoke view of the operating model", &ContentHints{ItemCount: 6}, "radial-hub"},
		{"ecosystem around the platform", &ContentHints{ItemCount: 5}, "radial-hub"},
		{"capabilities around the core", &ContentHints{ItemCount: 6}, "radial-hub"},
		{"stakeholder map for the programme", &ContentHints{ItemCount: 7}, "radial-hub"},
		{"hub and spoke", nil, "radial-hub"},
		// The today/future hub keeps its own intents.
		{"today vs future state across the workflow", &ContentHints{ItemCount: 4}, "state-shift-hub"},
		{"driver tree decomposing margin into its drivers", nil, "driver-tree"},
	}
	for _, tc := range cases {
		res := Recommend(Default(), tc.intent, tc.hints, 5)
		if len(res.Candidates) == 0 || res.Candidates[0].PatternName != tc.want {
			t.Errorf("intent %q: top = %v, want %s", tc.intent, res.Candidates, tc.want)
		}
	}
}
