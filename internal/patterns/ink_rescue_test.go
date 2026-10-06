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

// An ink that already reads keeps its link exactly as the gradient made it,
// so templates the rescue is not for render as before.
func TestRescueChevronInkLeavesReadableLinksAlone(t *testing.T) {
	for themeName, colors := range iconColorThemes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		for i := range 7 {
			tone := chevronGradientTone("accent1", i, 7)
			ink := timelineGradientTextColor(ctx, tone)
			gotTone, gotInk := rescueChevronInk(ctx, tone, ink)
			if gotTone != tone || gotInk != ink {
				t.Errorf("%s link %d: rescue changed a readable link: %+v/%s -> %+v/%s", themeName, i, tone, ink, gotTone, gotInk)
			}
		}
	}
	// No theme to measure against: nothing is changed.
	tone := chevronGradientTone("accent1", 3, 7)
	if gotTone, gotInk := rescueChevronInk(ExpandContext{}, tone, "lt1"); gotTone != tone || gotInk != "lt1" {
		t.Errorf("rescue without a theme changed the link: %+v/%s", gotTone, gotInk)
	}
	// A tinted link is a light surface: it is never shaded dark.
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: softDarkOrangeTheme()}}
	tint := fillTone{Color: "accent1", Tint: 5000}
	if gotTone, _ := rescueChevronInk(ctx, tint, "dk2"); gotTone != tint {
		t.Errorf("rescue shaded a tinted link: %+v", gotTone)
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
