package patterns

import (
	"strings"
	"testing"
)

// go-slide-creator-j8t7o: every stage was the same box at the same height, so
// the direction of maturity was only readable from the text. The default is a
// staircase; the tests that pin the old look ask for overrides.style "flat".

var journeyFlat = &JourneyMaturityOverrides{Style: "flat"}

func TestJourneyMaturityStaircaseAscends(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	ctx := testThemeCtx()
	for n := 3; n <= 6; n++ {
		vals := validJourneyMaturityValues(n)
		vals.Stages[n/2].Current = true
		grid, err := p.Expand(ctx, vals, nil, nil)
		if err != nil {
			t.Fatalf("n=%d: Expand: %v", n, err)
		}
		// The same three rows as the flat style, so (row, column) addresses
		// the same cell in both: headers, descriptions, markers.
		if len(grid.Rows) != 3 {
			t.Fatalf("n=%d: %d rows, want headers, descriptions and markers", n, len(grid.Rows))
		}
		headers, bodies, markers := grid.Rows[0], grid.Rows[1], grid.Rows[2]
		if len(headers.Cells) != n || len(bodies.Cells) != n || len(markers.Cells) != n {
			t.Fatalf("n=%d: rows hold %d / %d / %d cells, want %d each", n, len(headers.Cells), len(bodies.Cells), len(markers.Cells), n)
		}
		if headers.Connector != nil {
			t.Errorf("n=%d: the staircase draws no connectors", n)
		}
		if headers.MaxHeight <= 0 || headers.MinHeight != headers.MaxHeight {
			t.Fatalf("n=%d: the header row must be as tall as the staircase, in points: min=%.0f max=%.0f", n, headers.MinHeight, headers.MaxHeight)
		}

		headerH := 0.0
		for i := 0; i < n; i++ {
			h, b := headers.Cells[i], bodies.Cells[i]
			if !strings.Contains(string(h.Shape.Text), vals.Stages[i].Label) || !strings.Contains(string(b.Shape.Text), vals.Stages[i].Description) {
				t.Fatalf("n=%d: column %d does not hold stage %d", n, i, i+1)
			}
			// The header's top is InsetTop below the row top: it must step
			// up by one rise per stage.
			if i > 0 {
				rise := headers.Cells[i-1].InsetTop - h.InsetTop
				if rise < journeyMaturityMinRisePt {
					t.Errorf("n=%d: stage %d starts %.1fpt above stage %d, want at least %.0fpt", n, i+1, rise, i, journeyMaturityMinRisePt)
				}
			}
			// Every header is the same height.
			got := headers.MaxHeight - h.InsetTop - h.InsetBottom
			if i == 0 {
				headerH = got
			} else if got != headerH {
				t.Errorf("n=%d stage %d: header is %.1fpt tall, stage 1's is %.1fpt", n, i+1, got, headerH)
			}
			// The description reaches up to the underside of its header: as
			// far above its row as the header's bottom is above the row's.
			if b.BleedTop != h.InsetBottom {
				t.Errorf("n=%d stage %d: description reaches %.1fpt up, its header ends %.1fpt above the row", n, i+1, b.BleedTop, h.InsetBottom)
			}
			if !strings.Contains(string(b.Shape.Text), `"vertical_align":"t"`) {
				t.Errorf("n=%d stage %d: the description must hang from its header: %s", n, i+1, b.Shape.Text)
			}
		}
		if headers.Cells[n-1].InsetTop != 0 || headers.Cells[0].InsetBottom != 0 {
			t.Errorf("n=%d: the last stage must top the staircase and the first sit on its floor", n)
		}
		for i, m := range markers.Cells {
			if isMarker := m.Shape.Geometry == journeyMaturityMarkerGeometry; isMarker != (i == n/2) {
				t.Errorf("n=%d: marker under column %d = %t, want it under the current stage %d only", n, i, isMarker, n/2)
			}
		}
	}
}

// Without a current stage there is no marker row.
func TestJourneyMaturityStaircaseWithoutMarker(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	grid, err := p.Expand(testThemeCtx(), validJourneyMaturityValues(4), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 2 {
		t.Errorf("%d rows, want headers and descriptions only", len(grid.Rows))
	}
}

// The rise gives way to the copy: a first stage with a long description gets
// a shallower staircase, never less than the minimum rise.
func TestJourneyMaturityRiseYieldsToDescriptions(t *testing.T) {
	const firstNeed = 150.0
	short := []float64{60, 60, 60, 60, 60, 60}
	long := []float64{firstNeed, 60, 60, 60, 60, 60}
	const area, fixed = 300.0, 90.0
	a := journeyMaturityRisePt(area, fixed, short)
	b := journeyMaturityRisePt(area, fixed, long)
	if b >= a {
		t.Errorf("a long first description should lower the rise: short=%.0f long=%.0f", a, b)
	}
	if b < journeyMaturityMinRisePt {
		t.Errorf("rise %.0f fell below the %.0fpt minimum", b, journeyMaturityMinRisePt)
	}
	if base := area - fixed - 5*b; base < firstNeed && b > journeyMaturityMinRisePt {
		t.Errorf("rise %.0f leaves the first stage %.0fpt for a %.0fpt description", b, base, firstNeed)
	}
	// A late stage's long description costs nothing: its column is taller.
	late := []float64{60, 60, 60, 60, 60, 150}
	if c := journeyMaturityRisePt(area, fixed, late); c != a {
		t.Errorf("a long last description should not lower the rise: %.0f vs %.0f", c, a)
	}
}

func TestJourneyMaturityStyleValidation(t *testing.T) {
	p, _ := Default().Get("journey-maturity-model")
	vals := validJourneyMaturityValues(4)
	for _, style := range []string{"", "staircase", "flat"} {
		if err := p.Validate(vals, &JourneyMaturityOverrides{Style: style}, nil); err != nil {
			t.Errorf("style %q rejected: %v", style, err)
		}
	}
	if err := p.Validate(vals, &JourneyMaturityOverrides{Style: "ramp"}, nil); err == nil {
		t.Error("an unknown overrides.style must be rejected")
	}
}
