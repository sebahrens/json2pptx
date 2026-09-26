package template

import (
	"path/filepath"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

func TestReviewedTemplateSubtitlesHaveNormalTextContrast(t *testing.T) {
	for _, tc := range []struct {
		template, layout string
		gradient         bool
	}{
		{"business-template", "slideLayout3", false},
		{"modern", "slideLayout2", true},
		{"modern", "slideLayout4", true},
	} {
		t.Run(tc.template+"/"+tc.layout, func(t *testing.T) {
			r, err := OpenTemplate(filepath.Join("..", "..", "templates", tc.template+".pptx"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close() }()
			theme := ParseTheme(r)
			layouts, err := ParseLayouts(r)
			if err != nil {
				t.Fatal(err)
			}
			var subtitle *types.PlaceholderInfo
			for _, layout := range layouts {
				if layout.ID != tc.layout {
					continue
				}
				for i := range layout.Placeholders {
					if layout.Placeholders[i].Type == types.PlaceholderSubtitle {
						subtitle = &layout.Placeholders[i]
					}
				}
			}
			if subtitle == nil {
				t.Fatal("subtitle missing")
			}
			foregroundHex := ResolveBackgroundRefHexWithMods(subtitle.FontColor, subtitle.FontColorMods, theme.Colors)
			foreground, err := svggen.ParseColor(foregroundHex)
			if err != nil {
				t.Fatal("unresolved subtitle foreground:", foregroundHex, err)
			}
			background := svggen.MustParseColor("#FFFFFF")
			if tc.gradient {
				if !subtitle.FillGradient || len(subtitle.FillStops) != 3 {
					t.Fatal("original three-stop gradient lost")
				}
				if foreground.Hex() != "#FFFFFF" {
					t.Fatal("original white subtitle was changed")
				}
				// The brightest per-channel envelope is at least as luminous as
				// every stop and every RGB interpolation between them. White text
				// passing against it therefore also passes across the whole band.
				background = svggen.Color{A: 1}
				for _, stop := range subtitle.FillStops {
					hex := ResolveBackgroundRefHexWithMods(stop.Ref, stop.Mods, theme.Colors)
					color, err := svggen.ParseColor(hex)
					if err != nil {
						t.Fatal("unresolved gradient stop:", hex, err)
					}
					background.R = max(background.R, color.R)
					background.G = max(background.G, color.G)
					background.B = max(background.B, color.B)
				}
			}
			ratio := foreground.ContrastWith(background)
			t.Logf("foreground=%s worst background=%s contrast=%.2f:1", foreground.Hex(), background.Hex(), ratio)
			if ratio < 4.5 {
				t.Fatalf("subtitle contrast %.2f:1 is below4.5:1", ratio)
			}
		})
	}
}
