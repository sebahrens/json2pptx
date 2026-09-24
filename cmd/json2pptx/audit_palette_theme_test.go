package main

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
)

const auditThemeFixture = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:clrScheme name="Test"><a:accent1><a:srgbClr val="2E5090"/></a:accent1><a:accent2><a:srgbClr val="C43F3F"/></a:accent2><a:accent3><a:srgbClr val="439C79"/></a:accent3><a:accent4><a:srgbClr val="9A69B1"/></a:accent4><a:accent5><a:srgbClr val="D9A441"/></a:accent5><a:accent6><a:sysClr val="windowText" lastClr="245A6A"/></a:accent6></a:clrScheme></a:themeElements></a:theme>`

func TestAuditThemeColorsAndNearest(t *testing.T) {
	colors, err := parseAuditThemeColors([]byte(auditThemeFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(colors) != 30 {
		t.Fatalf("got %d accent/tint colors, want 30", len(colors))
	}
	if colors[0].Scheme != "accent1" || colors[0].Color.Hex() != "#2E5090" {
		t.Fatalf("accent1 = %+v", colors[0])
	}
	if colors[25].Scheme != "accent6" || colors[25].Color.Hex() != "#245A6A" {
		t.Fatalf("sysClr accent6 = %+v", colors[25])
	}
	pic := auditRegion{Name: "Chart"}
	base := nearestAuditThemeColor(1, pic, auditChroma{R: 0x2E, G: 0x50, B: 0x90, Pixels: 20}, colors, 15)
	if !base.Pass || base.DeltaE != 0 || base.NearestSchemeColor != "accent1" || base.NearestTint != 0 {
		t.Fatalf("base match = %+v", base)
	}
	tinted := colors[2]
	match := nearestAuditThemeColor(1, pic, auditChroma{R: tinted.Color.R, G: tinted.Color.G, B: tinted.Color.B, Pixels: 20}, colors, 15)
	if !match.Pass || match.NearestSchemeColor != "accent1" || match.NearestTint != 40000 || match.DeltaE != 0 {
		t.Fatalf("tint match = %+v", match)
	}
	off := nearestAuditThemeColor(1, pic, auditChroma{R: 0xF2, G: 0x8E, B: 0x2B, Pixels: 20}, colors, 15)
	if off.Pass || off.DeltaE <= 15 || off.NearestSchemeColor == "" {
		t.Fatalf("off-brand match = %+v", off)
	}
}

func TestParseAuditThemeColorsRejectsMissingAndMalformedAccents(t *testing.T) {
	for _, src := range []string{`<a:theme`, `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:clrScheme/></a:themeElements></a:theme>`} {
		if _, err := parseAuditThemeColors([]byte(src)); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestDominantChromasPxDetectsMinorityOffBrandSeriesWithoutShapes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}
	for y := 20; y < 70; y++ {
		for x := 10; x < 60; x++ {
			img.Set(x, y, color.RGBA{46, 80, 144, 255})
		}
	}
	for y := 25; y < 40; y++ {
		for x := 70; x < 80; x++ {
			img.Set(x, y, color.RGBA{242, 142, 43, 255})
		}
	}
	colors, err := parseAuditThemeColors([]byte(auditThemeFixture))
	if err != nil {
		t.Fatal(err)
	}
	chromas := dominantChromasPx(img, [4]int{0, 0, 100, 100}, 25)
	if len(chromas) != 2 {
		t.Fatalf("got %d chromas, want both series: %+v", len(chromas), chromas)
	}
	var failed int
	for _, c := range chromas {
		if !nearestAuditThemeColor(1, auditRegion{Name: "Chart"}, c, colors, 15).Pass {
			failed++
		}
	}
	if failed != 1 {
		t.Fatalf("got %d off-brand series, want 1: %+v", failed, chromas)
	}
	pic := auditRegion{Kind: "pic", Name: "Chart", BoundsPx: [4]int{0, 0, 100, 100}, PixelCount: 2650}
	slide, pairFailures, themeFailures := scoreAuditSlide(img, 1, []auditRegion{pic}, nil, colors, auditOptions{Mode: "theme", MaxThemeDeltaE: 15, ChromaMin: 25})
	if slide.PicCount != 1 || slide.ShapeCount != 0 || slide.PairCount != 0 || slide.ThemeMatchCount != 2 || pairFailures != 0 || themeFailures != 1 {
		t.Fatalf("shape-free chart was not scored against theme: slide=%+v pairFailures=%d themeFailures=%d", slide, pairFailures, themeFailures)
	}
	pairOnly, pairFailures, themeFailures := scoreAuditSlide(img, 1, []auditRegion{pic}, nil, nil, auditOptions{Mode: "pair", MaxDeltaE: 5})
	if pairOnly.ThemeMatchCount != 0 || pairOnly.PairCount != 0 || pairFailures != 0 || themeFailures != 0 {
		t.Fatalf("pair mode should keep theme check opt-in: %+v, %d, %d", pairOnly, pairFailures, themeFailures)
	}
}

func TestThemeMatchDiagnosticsWithNoPairs(t *testing.T) {
	report := &auditReport{
		MaxThemeDeltaEAllowed: 15,
		Violations:            1,
		Slides: []auditSlide{{Index: 1, PairCount: 0, ThemeMatchCount: 1, ThemeMatches: []auditThemeMatch{{
			Slide: 1, Pic: auditRegion{Name: "Chart"}, Hex: "f28e2b", PixelCount: 150,
			NearestSchemeColor: "accent5", NearestHex: "d9a441", DeltaE: 22, Pass: false,
		}}}},
	}
	ds := diagnosticsFromAuditReport(report)
	if len(ds) != 1 || ds[0].Code != "RENDER.palette_drift" || ds[0].Details["nearest_scheme_color"] != "accent5" {
		t.Fatalf("diagnostics = %+v", ds)
	}
	if diagnostics.BuildEnvelope(diagnostics.EnvelopeOptions{Subcommand: "audit_palette"}, ds).OK {
		t.Fatal("off-theme pic with no shape pairs must fail the MCP envelope")
	}
	if text := formatAuditText(report); !strings.Contains(text, "nearest=accent5") || !strings.Contains(text, "Result: FAIL") {
		t.Fatalf("theme match missing from text output: %s", text)
	}
}

func TestAuditThemeCatchesNeonGroupedNativeShape(t *testing.T) {
	theme := strings.Replace(auditThemeFixture, `val="9A69B1"`, `val="0097A7"`, 1)
	// An unrelated accent matching the neon result must not make the shape
	// appear on-brand: the fill explicitly refers to accent4.
	theme = strings.Replace(theme, `val="D9A441"`, `val="76F2FF"`, 1)
	colors, err := parseAuditThemeColors([]byte(theme))
	if err != nil {
		t.Fatal(err)
	}
	slideXML := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:grpSp><p:sp><p:nvSpPr><p:cNvPr name="Neon panel"/></p:nvSpPr><p:spPr><a:solidFill><a:schemeClr val="accent4"><a:lumMod val="20000"/><a:lumOff val="80000"/></a:schemeClr></a:solidFill></p:spPr></p:sp><p:sp><p:nvSpPr><p:cNvPr name="Tinted panel"/></p:nvSpPr><p:spPr><a:solidFill><a:schemeClr val="accent4"><a:tint val="20000"/></a:schemeClr></a:solidFill></p:spPr></p:sp></p:grpSp></p:spTree></p:cSld></p:sld>`
	mods, err := extractAuditAccentMods([]byte(slideXML))
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 1 || mods[0].Name != "Neon panel" {
		t.Fatalf("grouped HSL accent detection = %+v", mods)
	}
	slide := &auditSlide{Index: 1}
	if violations := scoreAuditAccentMods(slide, 1, mods, colors, 15); violations != 1 {
		t.Fatalf("violations = %d, want 1; match = %+v", violations, slide.ThemeMatches)
	}
	if slide.ThemeMatchCount != 1 || slide.ThemeMatches[0].Pic.Kind != "shape" || slide.ThemeMatches[0].DeltaE <= 15 {
		t.Fatalf("neon accent should be reported as off-theme native shape: %+v", slide)
	}
	report := &auditReport{Slides: []auditSlide{*slide}, Violations: 1}
	if !strings.Contains(formatAuditText(report), "shape[Neon panel]") {
		t.Fatalf("text report omits the native shape identity: %s", formatAuditText(report))
	}
}

func TestAuditAccentModsIgnoresUnmodifiedAndMalformedShapes(t *testing.T) {
	if _, err := extractAuditAccentMods([]byte(`<p:sld`)); err == nil {
		t.Fatal("malformed slide XML should fail")
	}
	slideXML := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></p:spPr></p:sp><p:sp><p:spPr><a:solidFill><a:srgbClr val="0097A7"/></a:solidFill></p:spPr></p:sp><p:sp><p:spPr><a:solidFill><a:schemeClr val="accent1"><a:lumMod val="75000"/></a:schemeClr></a:solidFill></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`
	mods, err := extractAuditAccentMods([]byte(slideXML))
	if err != nil || len(mods) != 0 {
		t.Fatalf("unmodified, authored, or darkened fills should not trigger: mods=%+v err=%v", mods, err)
	}
	unknown := []auditAccentMod{{Name: "Unknown", Scheme: "accent7", Mod: 20000, Off: 80000}}
	if got := scoreAuditAccentMods(&auditSlide{}, 1, unknown, nil, 15); got != 0 {
		t.Fatalf("unknown theme accent should not be scored without a reference, got %d", got)
	}
}
