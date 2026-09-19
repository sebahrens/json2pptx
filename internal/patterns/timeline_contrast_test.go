package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// timelineTextMinContrast is the WCAG AA floor for normal-size text.
const timelineTextMinContrast = 4.5

// timeline-horizontal's gantt and chevron styles tint each bar across the
// chain, from shade 70000 at the first stop to tint 40000 at the last, but the
// text inside every bar was hardcoded "lt1". On the lightest bar white on tint
// measured 1.54:1 in a real render — the date label was effectively invisible
// (go-slide-creator-5qotm).
func TestTimelineGradientTextStaysReadable(t *testing.T) {
	p, ok := Default().Get("timeline-horizontal")
	if !ok {
		t.Fatal("timeline-horizontal not registered")
	}

	// Seven stops is the pattern's own maximum, so this walks every position a
	// gradient can put a bar in.
	stops := TimelineHorizontalValues{
		{Label: "Discovery", Date: "Jan 2025", EndDate: "Mar 2025", Body: "Scoping"},
		{Label: "Design", Date: "Apr 2025", EndDate: "May 2025", Body: "Blueprint"},
		{Label: "Build", Date: "Jun 2025", EndDate: "Aug 2025", Body: "Waves 1-2"},
		{Label: "Test", Date: "Sep 2025", EndDate: "Sep 2025", Body: "Rehearsal"},
		{Label: "Pilot", Date: "Oct 2025", EndDate: "Oct 2025", Body: "One clearer"},
		{Label: "Cutover", Date: "Nov 2025", EndDate: "Nov 2025", Body: "The weekend"},
		{Label: "Close", Date: "Dec 2025", EndDate: "Dec 2025", Body: "Handover"},
	}

	for themeName, colors := range iconColorThemes {
		ctx := ExpandContext{Theme: types.ThemeInfo{Colors: colors}}
		for _, style := range []string{"gantt", "chevron"} {
			grid, err := p.Expand(ctx, &stops, &TimelineHorizontalOverrides{Style: style}, nil)
			if err != nil {
				t.Fatalf("%s/%s: Expand: %v", themeName, style, err)
			}
			checked := 0
			for _, row := range grid.Rows {
				for _, cell := range row.Cells {
					if cell == nil || cell.Shape == nil {
						continue
					}
					if assertGradientTextReadable(t, ctx, themeName+"/"+style, cell.Shape) {
						checked++
					}
				}
			}
			if checked < len(stops) {
				t.Errorf("%s/%s: only %d of %d bars carried checkable text", themeName, style, checked, len(stops))
			}
		}
	}
}

// assertGradientTextReadable checks a shape's text against its own tinted fill,
// reporting whether it had both to check.
func assertGradientTextReadable(t *testing.T, ctx ExpandContext, label string, shape *jsonschema.ShapeSpecInput) bool {
	t.Helper()
	tone, ok := parseFillTone(shape.Fill)
	if !ok {
		return false
	}
	fill, ok := effectiveFillColor(ctx, tone)
	if !ok {
		return false
	}
	var text struct {
		Paragraphs []struct {
			Content string `json:"content"`
			Color   string `json:"color"`
		} `json:"paragraphs"`
	}
	if len(shape.Text) == 0 || json.Unmarshal(shape.Text, &text) != nil || len(text.Paragraphs) == 0 {
		return false
	}
	for _, para := range text.Paragraphs {
		if para.Content == "" || para.Color == "" {
			continue
		}
		c, resolved := resolveThemeColor(ctx, para.Color)
		if !resolved {
			t.Errorf("%s: cannot resolve text colour %q", label, para.Color)
			continue
		}
		if cr := c.ContrastWith(fill); cr < timelineTextMinContrast {
			t.Errorf("%s: %q in %q on fill %+v contrast %.2f < %.1f",
				label, para.Content, para.Color, tone, cr, timelineTextMinContrast)
		}
	}
	return true
}

// The tint and shade transforms are linear-light mixes, not the HSL lightness
// lumMod/lumOff use. Both were verified against rendered pixels: midnight-blue
// accent1 #2E5090 renders rgb(38,67,122) under shade 70000 and rgb(205,208,219)
// under tint 40000.
func TestEffectiveColorModsMatchesRenderedTintAndShade(t *testing.T) {
	ctx := ExpandContext{Theme: types.ThemeInfo{Colors: iconColorThemes["midnight-blue"]}}
	for _, tc := range []struct {
		name string
		tone fillTone
		want string
	}{
		{name: "shade 70000", tone: fillTone{Color: "accent1", Shade: 70000}, want: "#26437A"},
		{name: "tint 40000", tone: fillTone{Color: "accent1", Tint: 40000}, want: "#CDD0DB"},
		{name: "unmodified", tone: fillTone{Color: "accent1"}, want: "#2E5090"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := effectiveFillColor(ctx, tc.tone)
			if !ok {
				t.Fatal("cannot resolve accent1")
			}
			if got.Hex() != tc.want {
				t.Errorf("effective colour = %s, want %s (measured from a real render)", got.Hex(), tc.want)
			}
		})
	}
}
