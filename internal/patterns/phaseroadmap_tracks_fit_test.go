package patterns

import (
	"regexp"
	"strings"
	"testing"
)

// A roadmap with parallel tracks pinned only its date, description and track
// rows; the phase-box row was a percentage. On a content area a takeaway band
// had shortened, the pinned rows (two 47pt track bars among them) used the
// whole grid, the percentage rows were scaled down with the over-commitment,
// and the phase names were written at 8.1pt on modern-template — the roadmap
// kind refused a three-phase plan with two tracks (go-slide-creator-x1124).

// phaseRoadmapTakeawayAreas are content areas of the shipped templates and of
// p-style once a one-line takeaway band is reserved, as the roadmap kind
// expands the pattern.
var phaseRoadmapTakeawayAreas = []struct {
	name, font string
	w, h       float64
}{
	{"modern", "Calibri", 851, 215},
	{"modern-template", "Calibri", 800, 239},
	{"midnight-blue", "Calibri", 797, 253},
	{"p-style", "Arial", 899, 264},
	{"business-template", "Calibri", 850, 305},
}

func x1124Roadmap(milestones bool) *PhaseRoadmapValues {
	v := &PhaseRoadmapValues{
		Phases: []PhaseRoadmapPhase{
			{Name: "Foundation", DateLabel: "Q1-Q2 2027", Description: "Landing zone, governance, FinOps guardrails; 3 pilot sources live.", Active: true},
			{Name: "Migrate", DateLabel: "Q3 2027-Q2 2028", Description: "14 sources on CDC; 1,900 ETL jobs become 400 pipelines."},
			{Name: "Optimise", DateLabel: "Q3-Q4 2028", Description: "Warehouse decommissioned; compute tuned; hand-over to run."},
		},
		ParallelLabel:  "In parallel",
		ParallelTracks: []string{"Data literacy for 300 analysts", "FinOps guardrails: budgets, policies, showback"},
	}
	if milestones {
		for i, m := range []string{"3 pilots live", "400 pipelines", "Warehouse off"} {
			v.Phases[i].Milestone = m
		}
	}
	return v
}

func TestPhaseRoadmapTracksFitTakeawayAreas(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	for _, a := range phaseRoadmapTakeawayAreas {
		for _, ms := range []bool{false, true} {
			name := a.name
			if ms {
				name += "/milestones"
			}
			t.Run(name, func(t *testing.T) {
				warnings, shrinks := writtenFitShrinks(t, p, a.font, a.w, a.h, x1124Roadmap(ms), nil)
				if len(warnings) > 0 || len(shrinks) > 0 {
					t.Errorf("three phases with two tracks must fit a %.0fpt area unshrunk; warnings %v, shrinks:\n  %s", a.h, warnings, strings.Join(shrinks, "\n  "))
				}
			})
		}
	}
}

// Every row of a tracked roadmap is pinned in points and the rows fit the
// area, so no row can be scaled below the height its text was measured at.
func TestPhaseRoadmapTrackedRowsArePinnedInsideTheArea(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	for _, a := range phaseRoadmapTakeawayAreas {
		ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(a.w * 12700), Height: int64(a.h * 12700)}}
		ctx.Theme.BodyFont = a.font
		grid, err := p.Expand(ctx, x1124Roadmap(true), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		total := float64(len(grid.Rows)-1) * grid.RowGap
		for i, r := range grid.Rows {
			if r.Height != 0 || r.MinHeight <= 0 || r.MinHeight != r.MaxHeight {
				t.Errorf("%s: row %d is not pinned in points: %+v", a.name, i, r)
			}
			total += r.MinHeight
		}
		if total > a.h {
			t.Errorf("%s: rows and gaps need %.1fpt of a %.0fpt area", a.name, total, a.h)
		}
	}
}

// A track is a bar as tall as its own line of text, so two tracks stay lighter
// than the phase boxes they run under.
func TestPhaseRoadmapTrackBarsAreOneLineBars(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 899 * 12700, Height: 360 * 12700}}
	ctx.Theme.BodyFont = "Arial"
	grid, err := p.Expand(ctx, x1124Roadmap(false), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	header := grid.Rows[0].MinHeight
	block := grid.Rows[len(grid.Rows)-1].Cells[0].Grid
	for i, r := range block.Rows {
		if r.MinHeight >= header {
			t.Errorf("track %d bar is %.0fpt, not below the %.0fpt phase boxes", i, r.MinHeight, header)
		}
		if lineH := scaleBodyPt * contentLineHeight; r.MinHeight > lineH+2*phaseRoadmapTrackBarPadPt+2 {
			t.Errorf("track %d bar is %.0fpt for one %.1fpt line", i, r.MinHeight, lineH)
		}
	}
}

// Air gives way before type, and the phase name stops at the body floor.
func TestPhaseRoadmapFitStepsKeepThePhaseNameReadable(t *testing.T) {
	steps := phaseRoadmapFitSteps(scaleSubheadPt, false)
	if first := steps[0]; first.rowPad != 0 || first.headerPad != 0 || first.headerSize != 0 {
		t.Errorf("first step must keep the uniform margins and the header size: %+v", first)
	}
	sawSize := false
	for i, s := range steps {
		if s.headerSize != 0 {
			sawSize = true
			if s.headerSize < scaleBodyPt {
				t.Errorf("step %d sets the phase name at %.0fpt, below the %.0fpt floor", i, s.headerSize, scaleBodyPt)
			}
			if s.rowPad != rowPadStepsPt[len(rowPadStepsPt)-1] {
				t.Errorf("step %d steps the type down before the row padding is at its tightest: %+v", i, s)
			}
		}
	}
	if !sawSize {
		t.Error("no step takes the phase name to the body floor")
	}
	for i, s := range phaseRoadmapFitSteps(18, true) {
		if s.headerSize != 0 {
			t.Errorf("step %d changes an authored header size: %+v", i, s)
		}
	}
}

// What cannot fit is reported with a budget per field rather than left to the
// writer's refusal: an over-long description is told how many characters the
// area holds beside the tracks; when no description is the cause, the last
// track is told how many tracks there is room for.
func TestPhaseRoadmapTracksOverflowReportsFieldBudgets(t *testing.T) {
	p, _ := Default().Get("phase-roadmap")
	pw := p.(PostExpandWarner)
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 800 * 12700, Height: 239 * 12700}}
	ctx.Theme.BodyFont = "Calibri"

	long := x1124Roadmap(true)
	long.Phases = append(long.Phases, long.Phases...)
	for i := range long.Phases {
		long.Phases[i].Description = proseOfLen(107)
	}
	warnings := pw.PostExpandWarnings(ctx, long, nil)
	if len(warnings) != len(long.Phases) {
		t.Fatalf("want one budget warning per over-long description, got %d: %v", len(warnings), warnings)
	}
	budgetRE := regexp.MustCompile(`^BODY_TOO_LONG: phase-roadmap phases\[\d\]\.description is 107 characters; beside 2 parallel tracks and the milestone row this content area \(238pt high\) holds about (\d+) description characters per phase \((\d+) lines?\)`)
	for _, w := range warnings {
		m := budgetRE.FindStringSubmatch(w)
		if m == nil {
			t.Fatalf("warning does not state a description budget: %s", w)
		}
		if m[1] == "0" || len(m[1]) > 2 {
			t.Errorf("description budget %s is not below the 107 characters written: %s", m[1], w)
		}
	}
	// The budget is honest: a description cut to it fits.
	budget := budgetRE.FindStringSubmatch(warnings[0])[1]
	n := 0
	for _, c := range budget {
		n = n*10 + int(c-'0')
	}
	for i := range long.Phases {
		long.Phases[i].Description = proseOfLen(n)
	}
	if w, shrinks := writtenFitShrinks(t, p, "Calibri", 800, 239, long, nil); len(w) > 0 || len(shrinks) > 0 {
		t.Errorf("descriptions cut to the reported %d characters must fit; warnings %v, shrinks %v", n, w, shrinks)
	}

	tracks := x1124Roadmap(true)
	for i := range tracks.Phases {
		tracks.Phases[i].Description = "Short."
	}
	tracks.ParallelTracks = []string{"Track one", "Track two", "Track three", "Track four"}
	ctx.LayoutBounds.Height = 190 * 12700
	warnings = pw.PostExpandWarnings(ctx, tracks, nil)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], "BODY_TOO_LONG: phase-roadmap parallel_tracks[3]: ") || !strings.Contains(warnings[0], "has room for ") {
		t.Fatalf("want one warning at the last track saying how many tracks fit, got %v", warnings)
	}
	// No "holds about N" phrase: that reads as a character budget downstream.
	if strings.Contains(warnings[0], "holds about") {
		t.Errorf("a track-count warning must not read as a character budget: %s", warnings[0])
	}
}
