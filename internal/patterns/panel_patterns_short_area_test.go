package patterns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// The n1muf start set — stylish-panels, card-grid, contact-directory,
// framework-grid, team-bios — audited on rendered maxima across the shipped
// content areas (go-slide-creator-n1muf). Sized by the theme-font model and
// character budgets measured on the 828×349pt blank-title area alone,
// stylish-panels bullets were written at 44-84% autofit (6.2-11.8pt) on the
// abstract / modern / modern-template content layouts, and two-row team-bios
// cards at 68-96%, with no finding to say so. The contract every payload must
// meet: either every run at or above the 12pt floor is written unshrunk, or
// the pattern reports BODY_TOO_LONG against the area it was given.

// panelAuditAreas are the content areas of the shipped templates the audit
// renders against: the shortest (abstract, modern, modern-template content
// layouts), the common ~828×345pt and the local p-style geometry.
var panelAuditAreas = []struct {
	name string
	w, h float64
}{
	{"abstract", 687, 294},
	{"modern", 851, 311},
	{"modern-template", 824, 325},
	{"blue-corporate", 825, 345},
	{"warm-coral", 828, 349},
	{"p-style", 899, 360},
}

func panelAuditCtx(w, h float64) ExpandContext {
	return ExpandContext{LayoutBounds: LayoutBounds{Width: int64(w * 12700), Height: int64(h * 12700)}}
}

// auditText returns realistic copy of about n characters: words of mixed
// length, cut at a word boundary.
func auditText(n int) string {
	const src = "Consolidate regional supply planning into one integrated operating model with shared data, clear decision rights and measurable quarterly targets for every business unit and function across the network "
	var b strings.Builder
	for b.Len() < n+len(src) {
		b.WriteString(src)
	}
	s := b.String()[:n]
	if i := strings.LastIndexByte(s, ' '); i > n*3/4 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

type panelAuditCase struct {
	name      string
	values    any
	overrides any
}

// panelAuditFloorViolations expands values at the area and returns every text
// shape whose runs at or above the 12pt floor would be written below it —
// or, when unshrunk is set, written with any autofit shrink at all.
func panelAuditFloorViolations(t *testing.T, p Pattern, ctx ExpandContext, values, overrides any, unshrunk ...bool) []string {
	t.Helper()
	grid, err := p.Expand(ctx, values, overrides, nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyGridDefaults(grid)
	res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
	var out []string
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
		if smallest < 12 {
			continue
		}
		if scale := writtenScaleIn(ctx, tb, c.Bounds); smallest*scale < 12 || (len(unshrunk) > 0 && unshrunk[0] && scale < 1) {
			out = append(out, fmt.Sprintf("%q written at %.0fpt × %.0f%% = %.1fpt in a %.0f×%.0fpt shape",
				firstText(tb), smallest, scale*100, smallest*scale, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700))
		}
	}
	return out
}

func panelAuditWarnings(p Pattern, ctx ExpandContext, values, overrides any) []string {
	if w, ok := p.(PostExpandWarner); ok {
		return w.PostExpandWarnings(ctx, values, overrides)
	}
	return nil
}

// panelAuditCases builds the payloads per pattern: the exemplar, then legal
// payloads at the documented budgets. The full sweep runs outside -short.
func panelAuditCases(full bool) map[string][]panelAuditCase {
	cases := map[string][]panelAuditCase{}
	for _, name := range []string{"stylish-panels", "card-grid", "contact-directory", "framework-grid", "team-bios"} {
		p, _ := Default().Get(name)
		cases[name] = append(cases[name], panelAuditCase{"exemplar", p.(Exemplar).ExemplarValues(), nil})
	}
	pick := func(all []int, short []int) []int {
		if full {
			return all
		}
		return short
	}
	// stylish-panels at its documented average budgets.
	for _, panels := range []int{3, 4, 5} {
		for bi, band := range stylishPanelBodyBudgets[panels] {
			if !full && bi != 0 && bi != len(stylishPanelBodyBudgets[panels])-1 {
				continue
			}
			for _, bullets := range pick([]int{1, 2, 3, 4, 5, 6, 7, 8}, []int{2, 4, 7}) {
				v := StylishPanelsValues{}
				for i := 0; i < panels; i++ {
					item := StylishPanelsItem{Title: auditText(band.maxTitle)}
					for j := 0; j < bullets; j++ {
						item.Body = append(item.Body, auditText(band.perBullet[bullets-1]))
					}
					v = append(v, item)
				}
				cases["stylish-panels"] = append(cases["stylish-panels"], panelAuditCase{fmt.Sprintf("%dp-%dt-%db", panels, band.maxTitle, bullets), &v, nil})
			}
		}
	}
	// team-bios: 1-4 members with full bios; 5-8 at the two-row budgets.
	for _, n := range pick([]int{1, 2, 3, 4, 5, 6, 8}, []int{3, 4, 5, 8}) {
		v := TeamBiosValues{}
		for i := 0; i < n; i++ {
			m := TeamBiosMember{Name: auditText(24), Role: auditText(40), Bio: auditText(teamBiosReadableBioBudget(n))}
			if n > 4 {
				m.Name, m.Role = auditText(teamBiosTwoRowNameBudget), auditText(teamBiosTwoRowRoleBudget)
			}
			v.Members = append(v.Members, m)
		}
		cases["team-bios"] = append(cases["team-bios"], panelAuditCase{fmt.Sprintf("%dm", n), &v, nil})
	}
	// contact-directory: group / people / title-length sweep.
	for _, groups := range pick([]int{1, 2, 3, 4}, []int{1, 3}) {
		for _, people := range pick([]int{3, 4, 6, 8, 10, 12, 16, 20, 24}, []int{6, 12, 24}) {
			for _, title := range pick([]int{12, 24, 36, 48, 60}, []int{24, 60}) {
				per := people / groups
				v := ContactDirectoryValues{}
				for g := 0; g < groups; g++ {
					grp := ContactDirectoryGroup{Name: auditText(20)}
					for i := 0; i < per; i++ {
						grp.People = append(grp.People, ContactDirectoryPerson{Name: "Alexandra Montgomery", Title: auditText(title)})
					}
					v.Groups = append(v.Groups, grp)
				}
				cases["contact-directory"] = append(cases["contact-directory"], panelAuditCase{fmt.Sprintf("%dg%dp%dt", groups, people, title), &v, nil})
			}
		}
	}
	// framework-grid: realistic labels and titles, and the field maxima.
	for _, rows := range pick([]int{2, 3, 4, 5, 6}, []int{3, 5}) {
		for _, cards := range pick([]int{2, 3, 4}, []int{2, 4}) {
			for _, body := range pick([]int{0, 26, 42, 60, 80, 160}, []int{26, 80}) {
				for _, long := range []bool{false, true} {
					label, title := 12, 16
					if long {
						label, title = fgLabelMax, fgTitleMax
					}
					v := FrameworkGridValues{}
					for r := 0; r < rows; r++ {
						row := FrameworkGridRow{Label: auditText(label)}
						for c := 0; c < cards; c++ {
							row.Cards = append(row.Cards, FrameworkGridCard{Title: auditText(title), Body: auditText(body)})
						}
						v.Rows = append(v.Rows, row)
					}
					cases["framework-grid"] = append(cases["framework-grid"], panelAuditCase{fmt.Sprintf("%dr%dc-%d-long=%t", rows, cards, body, long), &v, nil})
				}
			}
		}
	}
	return cases
}

// cardGridAuditCases fills every card to the body budget card-grid reports
// for this area, across styles, header lengths and grid shapes.
func cardGridAuditCases(ctx ExpandContext, full bool) []panelAuditCase {
	styles := []string{"filled", "numbered-badge", "soft-card"}
	headers := []int{10, 60}
	dims := [][2]int{{2, 2}, {3, 3}, {5, 2}, {4, 4}}
	if full {
		styles = []string{"filled", "accent-stripe", "numbered-badge", "tinted", "soft-card"}
		headers = []int{10, 30, 60, 80}
		dims = [][2]int{{1, 1}, {2, 1}, {2, 2}, {3, 2}, {2, 3}, {3, 3}, {4, 2}, {5, 2}, {4, 3}, {5, 3}, {4, 4}, {5, 5}}
	}
	var out []panelAuditCase
	for _, style := range styles {
		for _, header := range headers {
			for _, d := range dims {
				v := CardGridValues{Columns: d[0], Rows: d[1]}
				for i := 0; i < d[0]*d[1]; i++ {
					v.Cells = append(v.Cells, CardGridCell{Header: auditText(header), Body: "x"})
				}
				o := &CardGridOverrides{Style: style}
				budgets := CardGridBodyBudgets(ctx, &v, o)
				for i := range v.Cells {
					v.Cells[i].Body = auditText(max(budgets[i], 5))
				}
				out = append(out, panelAuditCase{fmt.Sprintf("%s-h%d-%dx%d", style, header, d[0], d[1]), &v, o})
			}
		}
	}
	return out
}

// TestPanelPatternsReadableOrReportedOnShortAreas: no legal payload of the
// n1muf start set is written below the 12pt floor without a BODY_TOO_LONG
// measured against the area, and every exemplar is written unshrunk with no
// finding on every shipped content area.
func TestPanelPatternsReadableOrReportedOnShortAreas(t *testing.T) {
	t.Parallel()
	full := !testing.Short()
	cases := panelAuditCases(full)
	for _, name := range []string{"stylish-panels", "card-grid", "contact-directory", "framework-grid", "team-bios"} {
		p, _ := Default().Get(name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, a := range panelAuditAreas {
				ctx := panelAuditCtx(a.w, a.h)
				local := cases[name]
				if name == "card-grid" {
					local = append(append([]panelAuditCase(nil), local...), cardGridAuditCases(ctx, full)...)
				}
				for _, c := range local {
					warns := panelAuditWarnings(p, ctx, c.values, c.overrides)
					below := panelAuditFloorViolations(t, p, ctx, c.values, c.overrides)
					if c.name == "exemplar" && len(warns) > 0 {
						t.Errorf("%s/%s exemplar reports %v", a.name, c.name, warns)
					}
					if len(below) > 0 && len(warns) == 0 {
						t.Errorf("%s/%s: %d text shapes below the floor with no finding, e.g. %s", a.name, c.name, len(below), below[0])
					}
				}
			}
		})
	}
}

// TestPanelPatternsGiveWayBeforeReporting pins payloads that fit their area
// only once air, geometry or a 12pt type step gives way: each is written
// unshrunk with no finding. Before n1muf each was written with an autofit
// shrink (stylish-panels bullets at 14pt × 86-88%), below the floor
// (team-bios, framework-grid), or reported although it fits (the
// contact-directory and framework-grid exemplars on abstract).
func TestPanelPatternsGiveWayBeforeReporting(t *testing.T) {
	t.Parallel()
	stylish := func(panels, bullets, chars int) *StylishPanelsValues {
		v := StylishPanelsValues{}
		for i := 0; i < panels; i++ {
			item := StylishPanelsItem{Title: auditText(20)}
			for j := 0; j < bullets; j++ {
				item.Body = append(item.Body, auditText(chars))
			}
			v = append(v, item)
		}
		return &v
	}
	team := func(n, bio int) *TeamBiosValues {
		v := TeamBiosValues{}
		for i := 0; i < n; i++ {
			v.Members = append(v.Members, TeamBiosMember{Name: auditText(20), Role: auditText(24), Bio: auditText(bio)})
		}
		return &v
	}
	framework := func(rows, cards, body int) *FrameworkGridValues {
		v := FrameworkGridValues{}
		for r := 0; r < rows; r++ {
			row := FrameworkGridRow{Label: auditText(12)}
			for c := 0; c < cards; c++ {
				row.Cards = append(row.Cards, FrameworkGridCard{Title: auditText(16), Body: auditText(body)})
			}
			v.Rows = append(v.Rows, row)
		}
		return &v
	}
	for _, tc := range []struct {
		name, pattern string
		w, h          float64
		values        any
	}{
		// 90-character bullets: 14pt holds about 55 on abstract, 12pt 95.
		{"stylish-panels 4×3 bullets on abstract", "stylish-panels", 687, 294, stylish(4, 3, 90)},
		{"stylish-panels 3×3 bullets on modern", "stylish-panels", 851, 311, stylish(3, 3, 160)},
		// The headshots give way to a full bio.
		{"team-bios 4 members on abstract", "team-bios", 687, 294, team(4, 180)},
		// Card padding and row gaps tighten.
		{"framework-grid 4×2 on abstract", "framework-grid", 687, 294, framework(4, 2, 26)},
		{"framework-grid exemplar on abstract", "framework-grid", 687, 294, (&frameworkGrid{}).ExemplarValues()},
		// Row gaps and the group gap tighten.
		{"contact-directory exemplar on abstract", "contact-directory", 687, 294, (&contactDirectory{}).ExemplarValues()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := Default().Get(tc.pattern)
			ctx := panelAuditCtx(tc.w, tc.h)
			if warns := panelAuditWarnings(p, ctx, tc.values, nil); len(warns) > 0 {
				t.Errorf("reports %v", warns)
			}
			if below := panelAuditFloorViolations(t, p, ctx, tc.values, nil, true); len(below) > 0 {
				t.Errorf("%d text shapes written shrunk, e.g. %s", len(below), below[0])
			}
		})
	}
}
