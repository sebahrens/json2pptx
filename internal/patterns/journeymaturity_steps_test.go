package patterns

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/svggen"
)

// go-slide-creator-an4ao: the default was ten grey boxes (a header over a body
// per stage) with an outlined "We are here" box under an outlined arrow. It is
// now one solid step per stage in a tonal ladder of the accent.

// journeyStepLayer returns the named layer of a staircase cell.
func journeyStepLayer(t *testing.T, cell *jsonschema.GridCellInput, prefix string) *jsonschema.LayerInput {
	t.Helper()
	for i := range cell.Layers {
		if strings.HasPrefix(cell.Layers[i].Name, prefix) {
			return &cell.Layers[i]
		}
	}
	return nil
}

// journeyParagraphs decodes a text payload's paragraphs.
func journeyParagraphs(t *testing.T, text json.RawMessage) []journeyMaturityParagraph {
	t.Helper()
	var obj journeyMaturityTextObj
	if err := json.Unmarshal(text, &obj); err != nil {
		t.Fatalf("text payload: %v: %s", err, text)
	}
	return obj.Paragraphs
}

func TestJourneyMaturityStepsRiseOnOneFloor(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	ctx := testThemeCtx()
	for n := 3; n <= 6; n++ {
		for _, current := range []int{-1, 0, n / 2, n - 1} {
			vals := validJourneyMaturityValues(n)
			if current >= 0 {
				vals.Stages[current].Current = true
			}
			name := fmt.Sprintf("n=%d current=%d", n, current)
			grid, err := p.Expand(ctx, vals, nil, nil)
			if err != nil {
				t.Fatalf("%s: Expand: %v", name, err)
			}
			if len(grid.Rows) != 2 {
				t.Fatalf("%s: %d rows, want the staircase and the descriptions", name, len(grid.Rows))
			}
			steps, descs := grid.Rows[0], grid.Rows[1]
			if len(steps.Cells) != n || len(descs.Cells) != n {
				t.Fatalf("%s: rows hold %d / %d cells, want %d each", name, len(steps.Cells), len(descs.Cells), n)
			}
			if steps.Connector != nil || descs.Connector != nil {
				t.Errorf("%s: the staircase draws no connectors", name)
			}
			if steps.MaxHeight <= 0 || steps.MinHeight != steps.MaxHeight {
				t.Fatalf("%s: the staircase row must be a fixed height: min=%.0f max=%.0f", name, steps.MinHeight, steps.MaxHeight)
			}
			_, contentH := contentAreaPt(ctx)
			if total := steps.MaxHeight + descs.MaxHeight; total > contentH+0.5 {
				t.Errorf("%s: staircase %.0fpt + descriptions %.0fpt exceed the %.0fpt content area", name, steps.MaxHeight, descs.MaxHeight, contentH)
			}

			prevTop := math.Inf(1)
			for i, cell := range steps.Cells {
				if cell.Shape != nil {
					t.Errorf("%s stage %d: the step cell must hold layers only", name, i+1)
				}
				step := journeyStepLayer(t, cell, "step-")
				if step == nil {
					t.Fatalf("%s stage %d: no step layer", name, i+1)
				}
				// Every step stands on the row's floor and starts one rise
				// above the stage before it.
				if bottom := step.Frame.Y + step.Frame.H; math.Abs(bottom-1) > 1e-9 {
					t.Errorf("%s stage %d: step ends at %.3f of the row, want the floor", name, i+1, bottom)
				}
				top := step.Frame.Y * steps.MaxHeight
				if rise := prevTop - top; i > 0 && rise < journeyMaturityStepMinRisePt-1e-6 {
					t.Errorf("%s: stage %d starts %.1fpt above stage %d, want at least %.0fpt", name, i+1, rise, i, journeyMaturityStepMinRisePt)
				}
				prevTop = top
				if string(step.Shape.Line) != string(noLine) {
					t.Errorf("%s stage %d: a filled step carries no outline: %s", name, i+1, step.Shape.Line)
				}
				paras := journeyParagraphs(t, step.Shape.Text)
				if len(paras) != 2 || paras[0].Content != fmt.Sprintf("%02d", i+1) || paras[1].Content != vals.Stages[i].Label || !paras[1].Bold {
					t.Errorf("%s stage %d: step text = %+v, want the numeral over the bold name", name, i+1, paras)
				}

				label, pointer := journeyStepLayer(t, cell, "marker-label"), journeyStepLayer(t, cell, "marker-pointer")
				if (label != nil) != (i == current) || (pointer != nil) != (i == current) {
					t.Errorf("%s stage %d: marker label=%t pointer=%t, want them on the current stage only", name, i+1, label != nil, pointer != nil)
				}
				if label == nil || pointer == nil {
					continue
				}
				// The marker stands in the air above its own step: label over
				// pointer over tread, nothing overlapping, nothing above the row.
				if label.Frame.Y < 0 || label.Frame.Y+label.Frame.H > pointer.Frame.Y+1e-9 || pointer.Frame.Y+pointer.Frame.H > step.Frame.Y+1e-9 {
					t.Errorf("%s: marker label %+v, pointer %+v and step top %.3f overlap or leave the row", name, label.Frame, pointer.Frame, step.Frame.Y)
				}
				if pointer.Shape.Geometry != journeyMaturityPointerGeometry || string(pointer.Shape.Fill) != string(step.Shape.Fill) {
					t.Errorf("%s: the pointer must be a solid triangle in the current step's tone: %s %s vs %s", name, pointer.Shape.Geometry, pointer.Shape.Fill, step.Shape.Fill)
				}
				if string(label.Shape.Fill) != string(noLine) || string(label.Shape.Line) != string(noLine) {
					t.Errorf("%s: the marker label is unboxed: fill %s line %s", name, label.Shape.Fill, label.Shape.Line)
				}
			}

			for i, cell := range descs.Cells {
				if cell.Shape == nil || string(cell.Shape.Fill) != string(noLine) || string(cell.Shape.Line) != string(noLine) {
					t.Fatalf("%s stage %d: the description is unboxed text", name, i+1)
				}
				paras := journeyParagraphs(t, cell.Shape.Text)
				if len(paras) != 1 || paras[0].Content != vals.Stages[i].Description {
					t.Errorf("%s stage %d: description = %+v", name, i+1, paras)
				}
			}
		}
	}
}

// Same-role text is one size across every stage, whatever the stage's copy
// (go-slide-creator-riyh7): numerals, names and descriptions each resolve one
// size, and the descriptions share one box height so no renderer shrinks one
// column's text and not its neighbour's.
func TestJourneyMaturityStepsOneSizePerRole(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	vals := validJourneyMaturityValues(6)
	vals.Stages[0].Description = "Short."
	vals.Stages[3].Description = strings.Repeat("Quantitatively measured outcomes. ", 3)
	vals.Stages[4].Label = "Continuously optimising everywhere"
	vals.Stages[2].Current = true
	grid, err := p.Expand(testThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var numeral, name, desc float64
	for i := range vals.Stages {
		step := journeyParagraphs(t, journeyStepLayer(t, grid.Rows[0].Cells[i], "step-").Shape.Text)
		d := journeyParagraphs(t, grid.Rows[1].Cells[i].Shape.Text)
		if i == 0 {
			numeral, name, desc = step[0].Size, step[1].Size, d[0].Size
			continue
		}
		if step[0].Size != numeral || step[1].Size != name || d[0].Size != desc {
			t.Errorf("stage %d: numeral %.0f / name %.0f / description %.0f, stage 1 has %.0f / %.0f / %.0f", i+1, step[0].Size, step[1].Size, d[0].Size, numeral, name, desc)
		}
	}
	if numeral < scaleDisplayPt || name < scaleSubheadPt || desc < scaleBodyPt {
		t.Errorf("sizes %.0f / %.0f / %.0f fall under the 28 / 14 / 12pt roles", numeral, name, desc)
	}
	if row := grid.Rows[1]; row.MinHeight != row.MaxHeight || row.MaxHeight <= 0 {
		t.Errorf("the description row must be one fixed height: min=%.0f max=%.0f", row.MinHeight, row.MaxHeight)
	}
}

// The fills are a tonal ladder: lighter swatches of the accent that deepen up
// to the one solid step, the palest swatch on the stages still ahead, and one
// ink that reads on every tinted step.
func TestJourneyMaturityStepsTonalLadder(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	ctx := testThemeCtx()
	for n := 3; n <= 6; n++ {
		for _, current := range []int{-1, 0, 1, n - 2, n - 1} {
			vals := validJourneyMaturityValues(n)
			target := n - 1
			if current >= 0 {
				vals.Stages[current].Current = true
				target = current
			}
			name := fmt.Sprintf("n=%d current=%d", n, current)
			grid, err := p.Expand(ctx, vals, nil, nil)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			prevLum := math.Inf(1)
			tintInk := ""
			for i, cell := range grid.Rows[0].Cells {
				step := journeyStepLayer(t, cell, "step-")
				tone, ok := parseFillTone(step.Shape.Fill)
				if !ok {
					t.Fatalf("%s stage %d: fill %s", name, i+1, step.Shape.Fill)
				}
				if tone.Color != "accent1" {
					t.Errorf("%s stage %d: fill colour %q, want the accent in every step", name, i+1, tone.Color)
				}
				solid := tone.LumMod == 0 && tone.LumOff == 0 && tone.Tint == 0 && tone.Alpha == 0
				if solid != (i == target) {
					t.Errorf("%s stage %d: solid=%t, want the solid accent on stage %d only: %s", name, i+1, solid, target+1, step.Shape.Fill)
				}
				fill, _ := effectiveFillColor(ctx, tone)
				lum := fill.Luminance()
				switch {
				case i <= target && lum >= prevLum:
					t.Errorf("%s stage %d: the ladder must deepen up to the solid step (luminance %.3f after %.3f)", name, i+1, lum, prevLum)
				case i > target && tone.LumMod != journeyMaturityAheadMod:
					t.Errorf("%s stage %d: a stage ahead keeps the palest swatch: %s", name, i+1, step.Shape.Fill)
				}
				prevLum = lum

				ink := journeyParagraphs(t, step.Shape.Text)[0].Color
				inkColor, _ := resolveThemeColor(ctx, ink)
				if ratio := inkColor.ContrastWith(fill); ratio < svggen.WCAGAANormal {
					t.Errorf("%s stage %d: ink %s on %s is %.2f:1", name, i+1, ink, step.Shape.Fill, ratio)
				}
				if i == target {
					continue
				}
				if tintInk == "" {
					tintInk = ink
				} else if ink != tintInk {
					t.Errorf("%s stage %d: ink %s, the other tinted steps use %s", name, i+1, ink, tintInk)
				}
			}
		}
	}
}

// A hex accent has no theme link to carry lumMod / lumOff, so its rungs are
// written as lightened hex colours rather than a neutral fallback.
func TestJourneyMaturityStepsHexAccent(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	vals := validJourneyMaturityValues(4)
	vals.Stages[2].Current = true
	grid, err := p.Expand(testThemeCtx(), vals, &JourneyMaturityOverrides{TextOverrides: TextOverrides{Accent: "#0B7A75"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i, cell := range grid.Rows[0].Cells {
		fill := string(journeyStepLayer(t, cell, "step-").Shape.Fill)
		var hex string
		if json.Unmarshal([]byte(fill), &hex) != nil || !isHexColor(hex) {
			t.Fatalf("stage %d: fill %s, want a plain hex", i+1, fill)
		}
		seen[strings.ToUpper(hex)] = true
	}
	if !seen["#0B7A75"] || len(seen) != 4 {
		t.Errorf("want the accent on the current stage and three distinct lighter rungs, got %v", seen)
	}
}

// The overrides the earlier default honoured still act on the steps.
func TestJourneyMaturityStepsOverrides(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	vals := validJourneyMaturityValues(5)
	vals.Stages[2].Current = true
	vals.Stages[0].Number = 7

	ovr := &JourneyMaturityOverrides{TextOverrides: TextOverrides{Accent: "accent2", HeaderSize: 16, BodySize: 13}}
	cellOvr := map[int]any{1: &JourneyMaturityCellOverride{AccentBar: true, Emphasis: "italic"}}
	grid, err := p.Expand(testThemeCtx(), vals, ovr, cellOvr)
	if err != nil {
		t.Fatal(err)
	}
	first := journeyParagraphs(t, journeyStepLayer(t, grid.Rows[0].Cells[0], "step-").Shape.Text)
	if first[0].Content != "07" || first[1].Size != 16 {
		t.Errorf("explicit number and header_size: %+v", first)
	}
	if d := journeyParagraphs(t, grid.Rows[1].Cells[0].Shape.Text); d[0].Size != 13 {
		t.Errorf("body_size: %+v", d)
	}
	for i, cell := range grid.Rows[0].Cells {
		if fill := string(journeyStepLayer(t, cell, "step-").Shape.Fill); !strings.Contains(fill, "accent2") {
			t.Errorf("stage %d: fill %s does not follow overrides.accent", i+1, fill)
		}
		if bar := journeyStepLayer(t, cell, "accent-bar-"); (bar != nil) != (i == 1) {
			t.Errorf("stage %d: accent bar = %t, want it on the overridden stage only", i+1, bar != nil)
		}
	}
	if text := string(journeyStepLayer(t, grid.Rows[0].Cells[1], "step-").Shape.Text); !strings.Contains(text, `"italic":true`) {
		t.Errorf("cell_overrides emphasis must reach the step text: %s", text)
	}

	// cell_accent_mode progressive paints the other stages in their own
	// accent slots, as it did on the header boxes.
	grid, err = p.Expand(testThemeCtx(), vals, &JourneyMaturityOverrides{TextOverrides: TextOverrides{CellAccentMode: CellAccentProgressive}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, cell := range grid.Rows[0].Cells {
		tone, _ := parseFillTone(journeyStepLayer(t, cell, "step-").Shape.Fill)
		if tone.LumMod != 0 || tone.LumOff != 0 {
			t.Errorf("stage %d: progressive mode must not paint ladder tints: %+v", i+1, tone)
		}
	}

	// Stages without copy leave the description row out.
	for i := range vals.Stages {
		vals.Stages[i].Description = ""
	}
	grid, err = p.Expand(testThemeCtx(), vals, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 1 {
		t.Errorf("%d rows, want the staircase alone when no stage has a description", len(grid.Rows))
	}
}

// The rise gives way to the copy and to the marker's room, never under the
// minimum; the marker costs height only above the last stages.
func TestJourneyMaturityStepRise(t *testing.T) {
	const need, marker = 80.0, 36.0
	roomy, _ := journeyMaturityStepRisePt(400, need, marker, 5, -1)
	if roomy != journeyMaturityStepMaxRisePt {
		t.Errorf("a roomy area takes the tallest rise: %.0f", roomy)
	}
	tight, reserve := journeyMaturityStepRisePt(200, need, marker, 5, 1)
	if tight >= roomy || tight < journeyMaturityStepMinRisePt {
		t.Errorf("a tight area lowers the rise within the minimum: %.0f", tight)
	}
	if reserve != 0 {
		t.Errorf("a marker over an early stage stands in free air, reserve %.0f", reserve)
	}
	if need+4*tight > 200 {
		t.Errorf("rise %.0f overruns the 200pt area", tight)
	}
	last, reserveLast := journeyMaturityStepRisePt(200, need, marker, 5, 4)
	if reserveLast != marker || last >= tight {
		t.Errorf("a marker over the last stage reserves its height above the staircase: rise %.0f reserve %.0f", last, reserveLast)
	}
	if floor, _ := journeyMaturityStepRisePt(60, need, marker, 6, 5); floor != journeyMaturityStepMinRisePt {
		t.Errorf("the rise never falls under the minimum: %.0f", floor)
	}
}
