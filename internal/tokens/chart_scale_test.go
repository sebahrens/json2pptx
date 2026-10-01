package tokens_test

import (
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/svggen"
)

// TestChartTextRolesOnSlideScale pins svggen's chart text roles to the slide
// type scale (go-slide-creator-vmdfm): a chart title is the slide lead step at
// the reference canvas and never smaller than the subhead (card title) step,
// chart body text is the slide body step, labels and captions the caption
// step. svggen is a separate module and cannot import the tokens, so this
// test is what keeps the two scales one.
func TestChartTextRolesOnSlideScale(t *testing.T) {
	for _, c := range []struct {
		role      string
		chart, pt float64
	}{
		{"title", svggen.ChartTitlePt, tokens.TypeScaleLeadPt},
		{"subtitle", svggen.ChartSubtitlePt, tokens.TypeScaleSubheadPt},
		{"heading", svggen.ChartHeadingPt, tokens.TypeScaleBodyPt},
		{"body", svggen.ChartBodyPt, tokens.TypeScaleBodyPt},
		{"label", svggen.ChartLabelPt, tokens.TypeScaleCaptionPt},
		{"caption", svggen.ChartCaptionPt, tokens.TypeScaleCaptionPt},
		{"title floor", svggen.ChartTitleMinPt, tokens.TypeScaleSubheadPt},
		{"text floor", svggen.ChartTextMinPt, tokens.BodyTextMinPt},
		{"label floor", svggen.ChartLabelMinPt, tokens.TypeScaleCaptionPt},
	} {
		if c.chart != c.pt {
			t.Errorf("chart %s = %gpt, want the slide step %gpt", c.role, c.chart, c.pt)
		}
		if !tokens.OnTypeScaleHPt(int(math.Round(c.chart * 100))) {
			t.Errorf("chart %s %gpt is off the type scale", c.role, c.chart)
		}
	}

	ty := svggen.DefaultTypography()
	for _, c := range []struct {
		role      string
		got, want float64
	}{
		{"SizeTitle", ty.SizeTitle, svggen.ChartTitlePt},
		{"SizeSubtitle", ty.SizeSubtitle, svggen.ChartSubtitlePt},
		{"SizeHeading", ty.SizeHeading, svggen.ChartHeadingPt},
		{"SizeBody", ty.SizeBody, svggen.ChartBodyPt},
		{"SizeSmall", ty.SizeSmall, svggen.ChartLabelPt},
		{"SizeCaption", ty.SizeCaption, svggen.ChartCaptionPt},
	} {
		if c.got != c.want {
			t.Errorf("DefaultTypography().%s = %g, want the chart role %g", c.role, c.got, c.want)
		}
	}

	// A small canvas clamps the title to the subhead step, not below it: the
	// chart title beside a 14pt card title is never 13pt.
	small := svggen.DefaultTypography().ScaleForDimensions(400, 300)
	if small.SizeTitle < tokens.TypeScaleSubheadPt {
		t.Errorf("scaled chart title = %gpt on a small canvas, below the %gpt subhead step", small.SizeTitle, tokens.TypeScaleSubheadPt)
	}
}

// TestChartTypeScaleMatchesSlideScale pins svggen's mirrored scale steps to
// the slide's, one for one, and every chart typography table svggen ships —
// the defaults, the compact tier, every layout preset and the large-canvas
// caps — onto those steps (go-slide-creator-vmdfm). A step added to or moved
// on either side fails here until the other follows.
func TestChartTypeScaleMatchesSlideScale(t *testing.T) {
	slide := tokens.TypeScaleStepsHPt()
	chart := svggen.ChartTypeScaleStepsPt
	if len(slide) != len(chart) {
		t.Fatalf("svggen mirrors %d scale steps %v, the slide scale has %d %v", len(chart), chart, len(slide), slide)
	}
	for i := range slide {
		if int(math.Round(chart[i]*100)) != slide[i] {
			t.Errorf("scale step %d: svggen %gpt, slide %gpt", i, chart[i], float64(slide[i])/100)
		}
	}

	onScale := func(where string, pts ...float64) {
		t.Helper()
		for _, pt := range pts {
			if !tokens.OnTypeScaleHPt(int(math.Round(pt * 100))) {
				t.Errorf("%s: %gpt is off the slide type scale", where, pt)
			}
		}
	}
	sizes := func(ty *svggen.Typography) []float64 {
		return []float64{ty.SizeTitle, ty.SizeSubtitle, ty.SizeHeading, ty.SizeBody, ty.SizeSmall, ty.SizeCaption}
	}
	onScale("DefaultTypography", sizes(svggen.DefaultTypography())...)
	onScale("CompactTypography", sizes(svggen.CompactTypography())...)
	for name, ty := range svggen.PresetTypography {
		onScale("preset "+name, sizes(ty)...)
	}
	onScale("ScaleForDimensions caps", svggen.ChartTitleMaxPt, svggen.ChartSubtitleMaxPt, svggen.ChartHeadingMaxPt,
		svggen.ChartBodyMaxPt, svggen.ChartLabelMaxPt, svggen.ChartCaptionMaxPt)
	onScale("ScaleForDimensions at the caps", sizes(svggen.DefaultTypography().ScaleForDimensions(4000, 3000))...)
}
