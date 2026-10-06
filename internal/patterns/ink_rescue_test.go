package patterns

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// softDarkOrangeTheme is a template whose darks are a soft charcoal and whose
// primary is a mid-tone orange: on the plain accent neither lt1 (3.3:1) nor
// dk2 (3.7:1) reads at body size (go-slide-creator-pr5bx).
func softDarkOrangeTheme() []types.ThemeColor {
	return []types.ThemeColor{
		{Name: "dk1", RGB: "2E353A"}, {Name: "lt1", RGB: "FFFFFF"},
		{Name: "dk2", RGB: "2E353A"}, {Name: "lt2", RGB: "FFE8D4"},
		{Name: "accent1", RGB: "FD5108"}, {Name: "accent2", RGB: "FE8A4F"},
		{Name: "accent3", RGB: "FFB38A"}, {Name: "accent4", RGB: "A9AFB8"},
		{Name: "accent5", RGB: "BCC2C9"}, {Name: "accent6", RGB: "D0D5DA"},
	}
}

// The middle link of an odd chain is the plain accent. readableTextOn answers
// the best theme ink even when none reads, so on soft darks the link carried
// dk2 at 3.77:1; the link is now deepened until lt1 reads, straight out of
// Expand (no ApplyReadableInk pass needed). accent4 is a mid-grey whose
// shaded links fall between the two inks as well.
func TestTimelineGradientRescuesLinkNoThemeInkReadsOn(t *testing.T) {
	p, _ := Default().Get("timeline-horizontal")
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: softDarkOrangeTheme()}}
	for _, accent := range []string{"", "accent4"} {
		for _, style := range []string{"chevron", "gantt"} {
			stops := make(TimelineHorizontalValues, 7)
			for i := range stops {
				stops[i] = TimelineStop{Label: "Wave close", Body: "Milestone detail", Date: fmt.Sprintf("Q%d", i+1)}
				if style == "gantt" {
					stops[i] = TimelineStop{Label: "Wave close", Date: fmt.Sprintf("2025-%02d", i+1), EndDate: fmt.Sprintf("2025-%02d", i+6)}
				}
			}
			ovr := &TimelineHorizontalOverrides{Style: style}
			ovr.Accent = accent
			grid, err := p.Expand(ctx, &stops, ovr, nil)
			if err != nil {
				t.Fatalf("%s/%q: Expand: %v", style, accent, err)
			}
			label := fmt.Sprintf("soft-darks/%s/accent=%q", style, accent)
			checked := 0
			for _, row := range grid.Rows {
				for _, cell := range row.Cells {
					if cell != nil && cell.Shape != nil && assertGradientTextReadable(t, ctx, label, cell.Shape) {
						checked++
					}
				}
			}
			if checked < len(stops) {
				t.Errorf("%s: only %d of %d links carried checkable text", label, checked, len(stops))
			}
		}
	}
}

// On a theme where an ink reads on every tone the chain is exactly the nominal
// ramp, so templates the rescue is not for render the documented gradient.
func TestTimelineGradientChainKeepsNominalRampWhereInkReads(t *testing.T) {
	for themeName, colors := range iconColorThemes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		for n := 2; n <= 7; n++ {
			for i, link := range timelineGradientChain(ctx, "accent1", n) {
				tone := chevronGradientTone("accent1", i, n)
				if !timelineGradientInkReads(ctx, tone) {
					continue // a soft-darks theme in the table: covered below
				}
				if link.tone != tone || link.ink != timelineGradientTextColor(ctx, tone) {
					t.Errorf("%s %d/%d: chain changed a readable link: %+v -> %+v/%s", themeName, i, n, tone, link.tone, link.ink)
				}
			}
		}
	}
	// No theme to measure against: the nominal ramp, light ink on the shaded
	// half and dark ink on the tinted half.
	for i, link := range timelineGradientChain(ExpandContext{}, "accent1", 7) {
		wantInk := "lt1"
		if i > 3 {
			wantInk = "dk2"
		}
		if tone := chevronGradientTone("accent1", i, 7); link.tone != tone || link.ink != wantInk {
			t.Errorf("no theme, link %d: got %+v/%s, want %+v/%s", i, link.tone, link.ink, tone, wantInk)
		}
	}
}

// The nominal ramp is the documented one: shade 70000 at the first stop, the
// plain accent at the midpoint, tint 40000 at the last, and every link keeps
// more of the accent (shaded half) or less of it (tinted half) than the one
// before. The modifiers used to be interpolated toward zero at the midpoint,
// which put the darkest link third and a near-white link fifth
// (go-slide-creator-c22bn).
func TestChevronGradientToneRampIsMonotonic(t *testing.T) {
	want7 := []fillTone{
		{Color: "accent1", Shade: 70000}, {Color: "accent1", Shade: 80000}, {Color: "accent1", Shade: 90000},
		{Color: "accent1"},
		{Color: "accent1", Tint: 80000}, {Color: "accent1", Tint: 60000}, {Color: "accent1", Tint: 40000},
	}
	for i, want := range want7 {
		if got := chevronGradientTone("accent1", i, 7); got != want {
			t.Errorf("link %d of 7: got %+v, want %+v", i, got, want)
		}
	}
	if got := chevronGradientTone("accent1", 0, 1); got != (fillTone{Color: "accent1"}) {
		t.Errorf("a single link is the plain accent, got %+v", got)
	}
	themes := map[string][]types.ThemeColor{"soft-darks": softDarkOrangeTheme()}
	for name, colors := range iconColorThemes {
		themes[name] = colors
	}
	for themeName, colors := range themes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		for _, accent := range []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"} {
			if _, ok := resolveThemeColor(ctx, accent); !ok {
				continue
			}
			for n := 2; n <= 7; n++ {
				nominal, chain := -1.0, -1.0
				for i, link := range timelineGradientChain(ctx, accent, n) {
					c, _ := effectiveFillColor(ctx, chevronGradientTone(accent, i, n))
					if lum := c.Luminance(); lum < nominal {
						t.Errorf("%s/%s nominal link %d of %d is darker than the link before it (%.4f < %.4f)", themeName, accent, i, n, lum, nominal)
					} else {
						nominal = lum
					}
					c, _ = effectiveFillColor(ctx, link.tone)
					if lum := c.Luminance(); lum < chain {
						t.Errorf("%s/%s chain link %d of %d is darker than the link before it (%.4f < %.4f): %+v", themeName, accent, i, n, lum, chain, link.tone)
					} else {
						chain = lum
					}
				}
			}
		}
	}
}

// On soft darks the plain orange midpoint holds no ink: it is deepened until
// lt1 reads and the links before it keep their proportion, so the dark half
// is still a ramp. A mid-grey accent's first link falls between the inks one
// step below a link dark ink reads on: it is lightened to it instead. Every
// link's ink clears the body-text bar.
func TestTimelineGradientChainLeavesNoLinkBetweenInks(t *testing.T) {
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: softDarkOrangeTheme()}}
	for _, accent := range []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"} {
		for n := 1; n <= 7; n++ {
			for i, link := range timelineGradientChain(ctx, accent, n) {
				fill, _ := effectiveFillColor(ctx, link.tone)
				ink, _ := resolveThemeColor(ctx, link.ink)
				if ratio := ink.ContrastWith(fill); ratio < timelineInkMinContrast {
					t.Errorf("%s link %d of %d: %s on %+v is %.2f:1", accent, i, n, link.ink, link.tone, ratio)
				}
			}
		}
	}
	chain := timelineGradientChain(ctx, "accent1", 7)
	if chain[3].tone.Shade == 0 || chain[3].ink != "lt1" {
		t.Errorf("plain orange midpoint was not deepened for lt1: %+v/%s", chain[3].tone, chain[3].ink)
	}
	for i := 1; i <= 3; i++ {
		if chain[i-1].tone.Shade >= chain[i].tone.Shade {
			t.Errorf("dark half is not a ramp: link %d shade %d, link %d shade %d", i-1, chain[i-1].tone.Shade, i, chain[i].tone.Shade)
		}
	}
	if floor := int(shadeMinKeep * gradientFullKeep); chain[0].tone.Shade < floor {
		t.Errorf("first link shaded to %d, below the %d floor", chain[0].tone.Shade, floor)
	}
}

// ApplyReadableInk applies the same rescue to any pattern shape whose dark
// ink fails on a fill no theme ink reads on, and leaves a shape alone when a
// theme ink does read (the pattern chose its ink on purpose).
func TestApplyReadableInkRescuesFailingDarkInk(t *testing.T) {
	block := func(fill string) *jsonschema.ShapeSpecInput {
		return &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"` + fill + `"`),
			Text:     json.RawMessage(`{"paragraphs":[{"content":"Intake","size":12,"bold":true,"color":"dk2"},{"content":"Detail","size":12,"color":"dk1"}]}`),
		}
	}
	gridOf := func(s *jsonschema.ShapeSpecInput) *jsonschema.ShapeGridInput {
		return &jsonschema.ShapeGridInput{Rows: []jsonschema.GridRowInput{{Cells: []*jsonschema.GridCellInput{{Shape: s}}}}}
	}

	soft := ExpandContext{Theme: types.ThemeInfo{Colors: softDarkOrangeTheme()}}
	rescued := block("accent1")
	ApplyReadableInk(soft, gridOf(rescued))
	tone, ok := parseFillTone(rescued.Fill)
	if !ok || tone.Color != "accent1" || tone.Shade == 0 {
		t.Fatalf("fill = %s, want accent1 deepened by a shade", rescued.Fill)
	}
	if !assertGradientTextReadable(t, soft, "rescued block", rescued) {
		t.Fatal("rescued block has no checkable text")
	}
	var text struct {
		Paragraphs []struct {
			Color string `json:"color"`
		} `json:"paragraphs"`
	}
	if err := json.Unmarshal(rescued.Text, &text); err != nil {
		t.Fatal(err)
	}
	for i, para := range text.Paragraphs {
		if para.Color != "lt1" {
			t.Errorf("paragraph %d ink = %s, want lt1 on the deepened fill", i, para.Color)
		}
	}

	// A pale accent on the same theme takes the charcoal at 5:1 or better: the
	// dark ink reads, so the block is not touched.
	pale := block("accent3")
	before := string(pale.Fill) + string(pale.Text)
	ApplyReadableInk(soft, gridOf(pale))
	if got := string(pale.Fill) + string(pale.Text); got != before {
		t.Errorf("readable dark-ink block was rewritten: %s -> %s", before, got)
	}

	// With a black dk1 some theme ink reads on every fill, so a dark ink is
	// never a failing fallback and nothing is shaded.
	for themeName, colors := range iconColorThemes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		for _, accent := range []string{"accent1", "accent2", "accent3", "accent4", "accent6"} {
			kept := block(accent)
			ApplyReadableInk(ctx, gridOf(kept))
			if tone, _ := parseFillTone(kept.Fill); tone.Shade != 0 {
				t.Errorf("%s/%s: dark-ink block was shaded although a theme ink reads: %s", themeName, accent, kept.Fill)
			}
		}
	}
}
