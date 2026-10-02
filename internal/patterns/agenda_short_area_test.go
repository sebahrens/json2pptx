package patterns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// agenda, before-after, before-after-compact, comparison-2col and hero-detail
// pinned their rows from the theme-font model alone, so extreme but legal
// payloads (within every documented character budget) were handed rows
// shorter than the written fit of their text on the short content areas of
// the shipped templates and stored with normAutofit shrinks — 12–14pt runs
// written at 6.5–11.8pt, which generation refuses (go-slide-creator-n1muf).
// Each pattern must now lay the content out at its written fit (air, then
// geometry, then a type step to the floor give way) or report BODY_TOO_LONG
// measured against the area.

// writtenFitAreas are the shortest and narrowest content areas of the shipped
// templates (content layout) and of the local p-style template, each with its
// theme body font, which pattern sizing measures with.
var writtenFitAreas = []struct {
	name, font string
	w, h       float64
}{
	{"abstract", "Tenorite", 687, 294},
	{"modern", "Calibri", 851, 311},
	{"midnight-blue", "Calibri", 797, 349},
	{"warm-coral", "Calibri", 828, 349},
	{"p-style", "Arial", 899, 360},
}

// writtenFitShrinks expands p at a w×h content area and returns the pattern's
// own warnings and every text cell the writer would store with a normAutofit
// shrink, i.e. whose row is below the written fit of its text.
func writtenFitShrinks(t *testing.T, p Pattern, font string, w, h float64, values, overrides any) (warnings, shrinks []string) {
	t.Helper()
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(w * 12700), Height: int64(h * 12700)}}
	ctx.Theme.BodyFont = font
	if pw, ok := p.(PostExpandWarner); ok {
		warnings = pw.PostExpandWarnings(ctx, values, overrides)
	}
	grid, err := p.Expand(ctx, values, overrides, nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyGridDefaults(grid)
	bounds := pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}
	if b := grid.Bounds; b != nil && b.Width > 0 && b.Height > 0 {
		// A height-capped pattern (before-after-compact) resolves in its
		// percentage share of the content area.
		bounds.CX = int64(float64(bounds.CX) * b.Width / 100)
		bounds.CY = int64(float64(bounds.CY) * b.Height / 100)
	}
	res := resolveGridAt(t, grid, bounds)
	for _, c := range res.Cells {
		if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb == nil {
			continue
		}
		for j := range tb.Insets {
			tb.Insets[j] += c.TextInsets[j]
		}
		smallest := smallestRunPt(tb)
		if smallest == 0 {
			continue
		}
		if scale := writtenScaleIn(ctx, tb, c.Bounds); scale < 1 {
			shrinks = append(shrinks, fmt.Sprintf("%q written at %.0fpt × %.0f%% = %.1fpt in a %.0f×%.0fpt shape",
				firstText(tb), smallest, scale*100, smallest*scale, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700))
		}
	}
	return warnings, shrinks
}

// writtenScaleIn is the autofit shrink the writer stores for tb in bounds when
// the slide's theme is ctx's: the shape grid hands the writer the theme fonts,
// so a Calibri body is measured in Carlito as pattern sizing measured it
// (go-slide-creator-ohhb2).
func writtenScaleIn(ctx ExpandContext, tb *pptx.TextBody, bounds pptx.RectEmu) float64 {
	tb.ThemeFonts = ctx.themeFonts()
	return pptx.AutofitScaleFor(tb, bounds)
}

func hasBodyTooLong(warnings []string) bool {
	for _, w := range warnings {
		if strings.Contains(w, ErrCodeBodyTooLong) {
			return true
		}
	}
	return false
}

// proseOfLen is realistic prose of n characters (no unbroken runs).
func proseOfLen(n int) string {
	const src = "Operating model redesign across regional delivery teams with shared service centres and clearer decision rights for every business unit leader "
	return strings.TrimSpace(strings.Repeat(src, n/len(src)+1)[:n])
}

func proseItems(count, n int) []string {
	items := make([]string, count)
	for i := range items {
		items[i] = proseOfLen(n)
	}
	return items
}

type writtenFitCase struct {
	values, overrides any
	// fits: the pattern must lay the payload out unshrunk on every area
	// (no BODY_TOO_LONG either); otherwise a shrink must come with one.
	fits bool
}

// assertWrittenFitOnShortAreas fails when a payload is written below the
// written fit of its text on a short content area without the pattern
// reporting BODY_TOO_LONG for that area, and when a payload that must fit
// shrinks or warns.
func assertWrittenFitOnShortAreas(t *testing.T, name string, cases map[string]writtenFitCase) {
	t.Helper()
	p, ok := Default().Get(name)
	if !ok {
		t.Fatalf("pattern %q not registered", name)
	}
	if ex, ok := p.(Exemplar); ok {
		cases["exemplar"] = writtenFitCase{values: ex.ExemplarValues(), fits: true}
	}
	for caseName, c := range cases {
		for _, a := range writtenFitAreas {
			t.Run(caseName+"/"+a.name, func(t *testing.T) {
				warnings, shrinks := writtenFitShrinks(t, p, a.font, a.w, a.h, c.values, c.overrides)
				switch {
				case c.fits && (len(shrinks) > 0 || len(warnings) > 0):
					t.Errorf("must fit unshrunk; warnings %v, shrinks:\n  %s", warnings, strings.Join(shrinks, "\n  "))
				case len(shrinks) > 0 && !hasBodyTooLong(warnings):
					t.Errorf("no BODY_TOO_LONG, but:\n  %s", strings.Join(shrinks, "\n  "))
				}
			})
		}
	}
}

func TestAgendaWrittenFitOnShortAreas(t *testing.T) {
	cases := map[string]writtenFitCase{}
	for _, n := range []int{2, 4, 5, 6, 8, 10} {
		for _, l := range []int{15, 50, 100} {
			cases[fmt.Sprintf("%dx%d", n, l)] = writtenFitCase{values: &AgendaValues{Items: proseItems(n, l)}, fits: n <= 4 && l <= 50}
		}
	}
	cases["6x100/highlight"] = writtenFitCase{values: &AgendaValues{Items: proseItems(6, 100)}, overrides: &AgendaOverrides{Highlight: 3}}
	assertWrittenFitOnShortAreas(t, "agenda", cases)
}

func TestBeforeAfterWrittenFitOnShortAreas(t *testing.T) {
	cases := map[string]writtenFitCase{}
	for _, n := range []int{1, 3, 4, 6, 8} {
		for _, l := range []int{40, 100, 200} {
			for _, hl := range []int{15, 60} {
				col := BeforeAfterColumn{Header: proseOfLen(hl), Items: proseItems(n, l)}
				cases[fmt.Sprintf("%dx%d/header%d", n, l, hl)] = writtenFitCase{values: &BeforeAfterValues{Before: col, After: col}, fits: n <= 3 && l <= 40}
			}
		}
	}
	assertWrittenFitOnShortAreas(t, "before-after", cases)
}

func TestBeforeAfterCompactWrittenFitOnShortAreas(t *testing.T) {
	cases := map[string]writtenFitCase{}
	// Every combination inside the documented line budget.
	for _, lens := range [][]int{{52, 52, 52, 52}, {103, 103, 103}, {133, 20, 20, 20}, {133, 133}, {40}} {
		for _, hl := range []int{10, 46} {
			items := make([]string, len(lens))
			for i, l := range lens {
				items[i] = proseOfLen(l)
			}
			col := BeforeAfterColumn{Header: proseOfLen(hl), Items: items}
			cases[fmt.Sprintf("%v/header%d", lens, hl)] = writtenFitCase{values: &BeforeAfterValues{Before: col, After: col}, fits: len(lens) == 1}
		}
	}
	assertWrittenFitOnShortAreas(t, "before-after-compact", cases)
}

func TestComparison2colWrittenFitOnShortAreas(t *testing.T) {
	cases := map[string]writtenFitCase{}
	for _, n := range []int{1, 2, 3, 4, 5, 6, 8, 10} {
		for _, headers := range []bool{false, true} {
			for _, connectors := range []bool{false, true} {
				// Every cell at the documented per-cell budget.
				l := comparisonBodyBudget(n, headers)
				if connectors {
					l = l * comparisonConnectorBudgetPct / 100
				}
				rows := make([]Comparison2colRow, n)
				for i := range rows {
					rows[i] = Comparison2colRow{Left: proseOfLen(l), Right: proseOfLen(l)}
				}
				v := &Comparison2colValues{Rows: rows}
				if headers {
					v.HeaderLeft, v.HeaderRight = proseOfLen(60), proseOfLen(60)
				}
				cases[fmt.Sprintf("%d/headers=%t/connectors=%t", n, headers, connectors)] = writtenFitCase{values: v, overrides: &Comparison2colOverrides{Connectors: connectors}, fits: n <= 2 && !headers}
			}
		}
	}
	// One long row among short ones takes height from the others.
	uneven := &Comparison2colValues{Rows: []Comparison2colRow{{Left: proseOfLen(190), Right: proseOfLen(190)}, {Left: "Fast", Right: "Slow"}, {Left: "Cheap", Right: "Costly"}}}
	cases["uneven"] = writtenFitCase{values: uneven, fits: true}
	assertWrittenFitOnShortAreas(t, "comparison-2col", cases)
}

func TestHeroDetailWrittenFitOnShortAreas(t *testing.T) {
	cases := map[string]writtenFitCase{}
	for _, n := range []int{2, 3, 4} {
		for _, tl := range []int{15, 60} {
			for _, context := range []int{0, 40} {
				for _, style := range []string{"cards", "minimal"} {
					for _, icon := range []bool{false, true} {
						d := make([]HeroDetailItem, n)
						for i := range d {
							d[i] = HeroDetailItem{Title: proseOfLen(tl)}
							if icon {
								d[i].Icon = &IconRef{Name: "chart-bar"}
							}
							if b := heroDetailBodyBudget(n, tl, icon); b > 0 {
								d[i].Body = proseOfLen(b)
							}
						}
						v := &HeroDetailValues{Hero: HeroDetailHero{Value: "$12,345.6M", Label: proseOfLen(80), Context: proseOfLen(context)}, Details: d}
						cases[fmt.Sprintf("%d/title%d/context%d/%s/icon=%t", n, tl, context, style, icon)] = writtenFitCase{values: v, overrides: &HeroDetailOverrides{Style: style}}
					}
				}
			}
		}
	}
	assertWrittenFitOnShortAreas(t, "hero-detail", cases)
}
