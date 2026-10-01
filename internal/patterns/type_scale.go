package patterns

import (
	"math"

	"github.com/sebahrens/json2pptx/internal/tokens"
)

// Pattern default text sizes (go-slide-creator-vmdfm).
//
// Every default font size a pattern resolves (the ResolveSize fallback, a
// fixed paragraph size, a measuring size) names a step of the one deck type
// scale in internal/tokens/typography.go instead of a numeric literal.
// TestPatternDefaultSizesOnTypeScale walks the package source and fails on a
// literal default or on a named default that is neither a scale step nor
// listed in offScaleDefaultReasons.
//
// Shrink ladders that a pattern walks while measuring a fit (agendaScales,
// execSummarySteps, metricListScales, …) name scale steps too: a ladder
// measures text at the size it renders, so it steps from one scale step to
// the next instead of through intermediate sizes the renderer would settle
// onto a lower step anyway (shapegrid.snapShapeTextToScale).
// TestPatternFitLaddersOnTypeScale holds every ladder to the scale; a step
// off it must be listed in offScaleLadderReasons.
const (
	scaleDisplayPt   = tokens.TypeScaleDisplayPt // 28pt display / title
	scaleLeadPt      = tokens.TypeScaleLeadPt    // 18pt lead / banner, step numerals
	scaleSubheadPt   = tokens.TypeScaleSubheadPt // 14pt subhead / card title
	scaleBodyPt      = tokens.TypeScaleBodyPt    // 12pt body
	scaleDenseBodyPt = tokens.BodyTextMinPt      // 11pt dense body (body minimum)
	scaleCaptionPt   = tokens.TypeScaleCaptionPt // 10pt caption
	scaleKPIPt       = tokens.TypeScaleKPIMinPt  // 40pt KPI display figure
	scaleKPIMaxPt    = tokens.TypeScaleKPIMaxPt  // 48pt top of the KPI display step
)

// Off-scale defaults. Each keeps the size a pattern has always measured and
// rendered at, so rendered output does not move; the reason is recorded in
// offScaleDefaultReasons (test-enforced). Sizes below the 12pt body step are
// raised to 12pt by the shape_grid renderer (shapegrid.EffectiveTextSizePt),
// and sizes between steps are settled onto the step below them unless they
// are display figures (shapegrid.snapShapeTextToScale).
const (
	// sizeHeaderPt is a bold card / panel header measured one step above the
	// 14pt subhead: it renders on the subhead step after the render snap, and
	// measuring it larger keeps wrap headroom in the header band.
	sizeHeaderPt = 16.0
	// sizeLabelPt is a bold step / band label measured above the 12pt body
	// step; it renders at 12pt after the render snap.
	sizeLabelPt = 13.0
	// sizeLeadPlusPt is a step numeral or keyword label measured above the
	// 18pt lead step: numerals render at 20pt as display figures, words at
	// 18pt after the render snap.
	sizeLeadPlusPt = 20.0
	// sizeFigurePt is a secondary display figure (an inline KPI value, a
	// result metric) kept as measured: display figures are not snapped.
	sizeFigurePt = 24.0
	// sizeHeadlineFigurePt is the chart-insights headline number, a display
	// figure under the KPI step so it leads the insights column without
	// competing with the chart; sizeHeadlineFigureCompactPt is its step in a
	// compact or crowded column.
	sizeHeadlineFigurePt        = 32.0
	sizeHeadlineFigureCompactPt = 26.0
	// sizeQuotePt is the pull-quote text: display type set below the KPI
	// figure, stepping down to fit.
	sizeQuotePt = 36.0
	// sizeBadgeNumeralMaxPt caps a numbered card's badge numeral (1.5x its
	// header) — a display figure kept as measured.
	sizeBadgeNumeralMaxPt = 36.0
	// sizeHeroFigurePt is the stat-hero figure: one oversized statistic
	// measured to fit, deliberately off the scale.
	sizeHeroFigurePt = 120.0
	// sizeDenseCaptionPt is a dense-diagram detail line (timeline dates,
	// canvas bullets, leaf and description text, quote attributions)
	// measured compact; the renderer raises it to the 12pt floor.
	sizeDenseCaptionPt = 9.0
	// sizeBadgePt is the short ALL-CAPS "RECOMMENDED" badge label on card-grid
	// and numbered-step cards (once 9pt on one, 8pt on the other) measured
	// compact; the renderer raises it to the 12pt floor.
	sizeBadgePt = 9.0
)

// offScaleDefaultReasons allow-lists the off-scale default sizes above. The
// source scan in TestPatternDefaultSizesOnTypeScale accepts a named default
// whose value is off the scale only when its constant is listed here.
var offScaleDefaultReasons = map[string]string{
	"sizeHeaderPt":                "bold header measured with headroom; renders on the 14pt subhead step",
	"sizeLabelPt":                 "bold label measured above body; renders on the 12pt body step",
	"sizeLeadPlusPt":              "20pt numeral display figure; words render on the 18pt lead step",
	"sizeFigurePt":                "secondary display figure, not snapped",
	"sizeHeadlineFigurePt":        "chart-insights headline display figure",
	"sizeHeadlineFigureCompactPt": "chart-insights headline display figure, compact step",
	"sizeQuotePt":                 "pull-quote display text, steps down to fit",
	"sizeBadgeNumeralMaxPt":       "numbered-card badge numeral cap, display figure",
	"sizeHeroFigurePt":            "stat-hero oversized statistic, measured to fit",
	"sizeDenseCaptionPt":          "dense detail line measured compact; rendered at the 12pt floor",
	"sizeBadgePt":                 "ALL-CAPS badge measured compact; rendered at the 12pt floor",
	"SourceNoteSizePt":            "engine-rendered 9pt source line (chrome, not an authoring size)",
}

// Off-scale ladder steps. A fit ladder may only leave the scale for a display
// figure above the 28pt display step (measured to fit, never snapped) or for
// a half-step whose removal would lower a TestSchemaMaximaStayReadable pin;
// each is listed in offScaleLadderReasons with the reason.
const (
	// sizeHeroDetailFigurePt is hero-detail's largest hero figure: one
	// dominant metric, an oversized display figure like stat-hero's.
	sizeHeroDetailFigurePt = 80.0
)

// offScaleLadderReasons allow-lists off-scale ladder steps
// (TestPatternFitLaddersOnTypeScale). Allow-listed defaults
// (offScaleDefaultReasons) may also appear in a ladder as its first step.
var offScaleLadderReasons = map[string]string{
	"sizeHeroDetailFigurePt": "hero-detail's dominant figure, an oversized display figure measured to fit",
}

// snapPt settles a derived size (a ladder step plus a bump) onto the type
// scale step at or below it — what the renderer would draw it at.
func snapPt(pt float64) float64 {
	return float64(tokens.SnapTextHPt(int(math.Round(pt*100)))) / 100
}
