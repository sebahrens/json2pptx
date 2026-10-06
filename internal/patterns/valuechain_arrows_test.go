package patterns

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

// go-slide-creator-gm4q9: the value chain drew grey rectangles with a 4pt
// arrowhead in the gap. The default is a row of interlocking arrows.

func valueChainLabelSizePt(t *testing.T, text json.RawMessage) float64 {
	t.Helper()
	var obj struct {
		Paragraphs []struct {
			Size float64 `json:"size"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(text, &obj); err != nil || len(obj.Paragraphs) == 0 {
		t.Fatalf("label text %s: %v", text, err)
	}
	return obj.Paragraphs[0].Size
}

func TestValueChainArrowsInterlock(t *testing.T) {
	p, _ := Default().Get("value-chain")
	ctx := testThemeCtx()
	for _, n := range []int{4, 6, 10} {
		vals := validValueChainValues(n)
		vals.Steps[2].Highlight = true
		grid, err := p.Expand(ctx, vals, nil, nil)
		if err != nil {
			t.Fatalf("n=%d: Expand: %v", n, err)
		}
		row := grid.Rows[0]
		if row.Connector != nil {
			t.Errorf("n=%d: interlocking arrows need no connector", n)
		}
		if row.MaxHeight <= 0 || row.MinHeight != row.MaxHeight {
			t.Errorf("n=%d: the arrow row must have a fixed height in points, got min=%.0f max=%.0f", n, row.MinHeight, row.MaxHeight)
		}
		fit := fitValueChainArrows(ctx, vals.Steps, scaleBodyPt)
		if fit.notchPt < valueChainMinNotchPt {
			t.Errorf("n=%d: point depth %.1fpt is below the %.0fpt minimum", n, fit.notchPt, valueChainMinNotchPt)
		}
		for i, c := range row.Cells {
			want := "chevron"
			if i == 0 {
				want = "homePlate"
			}
			if c.Shape.Geometry != want {
				t.Errorf("n=%d step %d: geometry %q, want %q", n, i, c.Shape.Geometry, want)
			}
			// The notch sits one column gap off the point before it: the
			// shape reaches back over the gap by exactly the point depth.
			wantBleed := fit.notchPt
			if i == 0 {
				wantBleed = 0
			}
			if c.BleedLeft != wantBleed {
				t.Errorf("n=%d step %d: bleed_left %.1f, want %.1f", n, i, c.BleedLeft, wantBleed)
			}
			// The written adjustment reproduces the fitted point depth, so
			// the preset's text rectangle is the one the labels were fitted
			// to.
			boxW := fit.colWPt + c.BleedLeft
			bounds := pptx.RectEmu{CX: int64(boxW * 12700), CY: int64(row.MaxHeight * 12700)}
			rectW, _ := pptx.PresetTextRectSize(c.Shape.Geometry, c.Shape.Adjustments["adj"], bounds)
			if got, wantW := float64(rectW)/12700, fit.textRectPt(i); got < wantW-0.5 || got > wantW+0.5 {
				t.Errorf("n=%d step %d: written text rectangle %.1fpt, fitted against %.1fpt", n, i, got, wantW)
			}
			if size := valueChainLabelSizePt(t, c.Shape.Text); size < valueChainMinLabelPt {
				t.Errorf("n=%d step %d: label written at %.1fpt, below the %.0fpt floor", n, i, size, valueChainMinLabelPt)
			}
		}
		if got := string(row.Cells[2].Shape.Fill); !strings.Contains(got, "accent") {
			t.Errorf("n=%d: the highlighted arrow lost its accent fill: %s", n, got)
		}
		// A plain arrow is the accent's Lighter 80% content swatch, with the
		// ink measured on it.
		step := valueChainStepTone(ctx, ctx.DefaultAccent())
		if got := string(row.Cells[1].Shape.Fill); got != string(step.fillJSON()) || got != `{"color":"accent1","lumMod":20000,"lumOff":80000}` {
			t.Errorf("n=%d: a plain arrow is filled %s, want accent1's Lighter 80%% swatch", n, got)
		}
		if ink := tonalInk(ctx, step); !strings.Contains(string(row.Cells[1].Shape.Text), `"color":"`+ink+`"`) {
			t.Errorf("n=%d: a plain arrow's label is not in the measured ink %q: %s", n, ink, row.Cells[1].Shape.Text)
		}
	}
}

// A word no arrow can hold is reported and written at the floor, never
// smaller; a two-word label wraps at its space instead of being reported.
func TestValueChainArrowLabelFit(t *testing.T) {
	p, _ := Default().Get("value-chain")
	warner := p.(PostExpandWarner)
	ctx := testThemeCtx()

	vals := validValueChainValues(10)
	for i, label := range []string{"Source", "Refine", "Make", "Move", "Sell", "Serve", "Reuse", "Decommissioning", "Renew", "Close"} {
		vals.Steps[i].Label = label
	}
	warnings := warner.PostExpandWarnings(ctx, vals, nil)
	found := false
	for _, w := range warnings {
		if strings.HasPrefix(w, ErrCodeTextExceedsShape+": ") && strings.Contains(w, "Decommissioning") {
			found = true
		}
	}
	if !found {
		t.Errorf("a word wider than a ten-step arrow was not reported: %v", warnings)
	}
	grid, err := p.Expand(ctx, vals, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	for i, c := range grid.Rows[0].Cells {
		if size := valueChainLabelSizePt(t, c.Shape.Text); size < valueChainMinLabelPt {
			t.Errorf("step %d: label shrunk to %.1fpt, below the %.0fpt floor", i, size, valueChainMinLabelPt)
		}
	}

	wrapped := validValueChainValues(6)
	wrapped.Steps[0].Label = "Inbound logistics"
	wrapped.Steps[3].Label = "Marketing and sales"
	if w := warner.PostExpandWarnings(ctx, wrapped, nil); len(w) != 0 {
		t.Errorf("labels that wrap at a space must not be reported: %v", w)
	}
	short, err := p.Expand(ctx, validValueChainValues(6), nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	tall, err := p.Expand(ctx, wrapped, nil, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if tall.Rows[0].MaxHeight < short.Rows[0].MaxHeight {
		t.Errorf("a wrapped label must not shrink the arrow row: %.0f < %.0f", tall.Rows[0].MaxHeight, short.Rows[0].MaxHeight)
	}
}

// The earlier look stays available as overrides.style "boxes".
func TestValueChainBoxesStyle(t *testing.T) {
	p, _ := Default().Get("value-chain")
	vals := validValueChainValues(5)
	grid, err := p.Expand(testThemeCtx(), vals, &ValueChainOverrides{Style: "boxes"}, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if grid.Rows[0].Connector == nil {
		t.Error("the boxes style joins its steps with connector arrows")
	}
	for i, c := range grid.Rows[0].Cells {
		if c.Shape.Geometry != "rect" || c.BleedLeft != 0 {
			t.Errorf("step %d: geometry %q bleed %.1f, want a plain rect", i, c.Shape.Geometry, c.BleedLeft)
		}
	}
	if err := p.Validate(vals, &ValueChainOverrides{Style: "ribbons"}, nil); err == nil {
		t.Error("an unknown overrides.style must be rejected")
	}
}
