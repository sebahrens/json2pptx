package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestFooterConfigForInputMergesChromeAndLegacyText(t *testing.T) {
	for _, tc := range []struct {
		name       string
		footer     *deckinput.JSONFooter
		chrome     *ChromeInput
		wantLine   string
		wantConfig bool
	}{
		{name: "no footer", wantConfig: false},
		{name: "legacy only", footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Legacy"}, wantLine: "Legacy", wantConfig: true},
		{name: "page numbers preserve legacy", footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Legacy"}, chrome: &ChromeInput{PageNumbers: &PageNumbersInput{Format: "{current}/{total}"}}, wantLine: "Legacy", wantConfig: true},
		{name: "structured text and legacy both survive", footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Legacy"}, chrome: &ChromeInput{ClientName: "Client B"}, wantLine: "Client B | Legacy", wantConfig: true},
		{name: "same text is not duplicated", footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Acme"}, chrome: &ChromeInput{ClientName: "Acme"}, wantLine: "Acme", wantConfig: true},
		{name: "chrome segment is not duplicated", footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Acme"}, chrome: &ChromeInput{ClientName: "Acme", FooterDate: "May"}, wantLine: "Acme | May", wantConfig: true},
		{name: "disabled legacy is omitted", footer: &deckinput.JSONFooter{LeftText: "Disabled"}, chrome: &ChromeInput{ClientName: "Acme"}, wantLine: "Acme", wantConfig: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := &PresentationInput{Footer: tc.footer, Chrome: tc.chrome}
			cfg := footerConfigForInput(input, 3)
			if (cfg != nil) != tc.wantConfig {
				t.Fatalf("footer config = %+v, want config=%t", cfg, tc.wantConfig)
			}
			if cfg != nil && cfg.LeftText != tc.wantLine {
				t.Errorf("merged line = %q, want %q", cfg.LeftText, tc.wantLine)
			}
		})
	}
}

func TestFooterConfigForInputKeepsSectionCrumbOnMergedLine(t *testing.T) {
	input := &PresentationInput{
		Footer: &deckinput.JSONFooter{Enabled: true, LeftText: "Legacy"},
		Chrome: &ChromeInput{ClientName: "Acme", SectionCrumb: true},
		Slides: []SlideInput{{SectionTitle: "Market"}},
	}
	cfg := footerConfigForInput(input, 1)
	if got := cfg.LeftTextFor(0); got != "Acme | Legacy | Market" {
		t.Errorf("merged section line = %q", got)
	}
}

func TestStructuralFooterUsesResolvedRegionAndChrome(t *testing.T) {
	const slideW, slideH int64 = 12192000, 6858000
	layout := types.LayoutMetadata{
		ID: "slideLayout1", Name: "One Content",
		Placeholders: []types.PlaceholderInfo{
			{ID: "body", Type: types.PlaceholderBody, Bounds: types.BoundingBox{X: 500000, Y: 1000000, Width: 11000000, Height: 5000000}},
			{ID: "sldNum", Type: types.PlaceholderOther, Bounds: types.BoundingBox{X: 11000000, Y: 6450000, Width: 500000, Height: 200000}},
			{ID: "ftr", Type: types.PlaceholderOther, Bounds: types.BoundingBox{X: 500000, Y: 6000000, Width: 5000000, Height: 250000}},
		},
		FooterRegions: []types.ChromeRegion{
			{Type: "sldNum", X: 11000000, Y: 6450000, Width: 500000, Height: 200000},
			{Type: "ftr", X: 500000, Y: 6000000, Width: 5000000, Height: 250000},
		},
	}
	ctx := resolveGridContext(&ShapeGridInput{}, &layout, slideW, slideH, true)
	if !ctx.layoutDeclaresFooter || ctx.footerY != 6000000 {
		t.Fatalf("resolved footer = %+v, want top 6000000", ctx)
	}
	grid := &ShapeGridInput{
		BoundsRelativeToContentArea: true,
		Bounds:                      &jsonschema.GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 120},
		Rows:                        []GridRowInput{{Cells: []*GridCellInput{{Shape: &jsonschema.ShapeSpecInput{Geometry: "rect"}}}}},
	}
	input := &PresentationInput{Chrome: &ChromeInput{PageNumbers: &PageNumbersInput{}}, Slides: []SlideInput{{LayoutID: layout.ID, ShapeGrid: grid}}}
	var collision bool
	for _, finding := range collectStructuralFindings(input, []types.LayoutMetadata{layout}, slideW, slideH) {
		if finding.Code == patterns.ErrCodeFooterCollision && strings.Contains(finding.Path, "/shape_grid/") {
			collision = true
		}
	}
	if !collision {
		t.Fatal("chrome-only footer collision was not reported")
	}
	layout.CanonicalType = types.CanonicalLayoutTitleSlide
	for _, finding := range collectStructuralFindings(input, []types.LayoutMetadata{layout}, slideW, slideH) {
		if finding.Code == patterns.ErrCodeFooterCollision {
			t.Errorf("title-layout chrome is skipped at render and must not report a footer collision: %+v", finding)
		}
	}
	input.Chrome.PageNumbers.Skip = []string{} // Explicitly show chrome on every layout.
	var titleCollision bool
	for _, finding := range collectStructuralFindings(input, []types.LayoutMetadata{layout}, slideW, slideH) {
		titleCollision = titleCollision || finding.Code == patterns.ErrCodeFooterCollision
	}
	if !titleCollision {
		t.Fatal("explicitly enabled title-layout chrome must report the collision")
	}
}
