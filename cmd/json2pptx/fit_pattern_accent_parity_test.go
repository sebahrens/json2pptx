package main

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

var sectionBadgeRunFillRE = regexp.MustCompile(`<a:rPr\b[^>]*><a:solidFill>`) // run fill, not inherited lstStyle fill

func TestSectionNumberTargetDoesNotIncludeSameTextTitle(t *testing.T) {
	withBadge := &types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{
		{ID: "7", Role: types.PlaceholderRoleTitle},
		{ID: "13", Role: types.PlaceholderRoleSectionNumber},
	}}
	withoutBadge := &types.LayoutMetadata{Placeholders: []types.PlaceholderInfo{
		{ID: "7", Role: types.PlaceholderRoleTitle},
		{ID: "body", Role: types.PlaceholderRoleBody},
	}}
	for _, tc := range []struct {
		name   string
		id     string
		layout *types.LayoutMetadata
		want   bool
	}{
		{"virtual badge", "section_number", withBadge, true},
		{"physical badge", "13", withBadge, true},
		{"same-text title", "7", withBadge, false},
		{"body with badge", "body", withBadge, false},
		{"body fallback", "body", withoutBadge, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSectionNumberContentTarget(tc.id, tc.layout); got != tc.want {
				t.Errorf("target(%q) = %t, want %t", tc.id, got, tc.want)
			}
		})
	}
}

// A section-keyed pattern after a divider uses accent2 in both validation and
// rendering. The light override makes a real contrast decision observable,
// rather than merely checking that both paths can expand a pattern.
func TestSectionKeyedPatternContrastParityAcrossLocalTemplates(t *testing.T) {
	for _, templateName := range testutil.AllTestTemplateNames() {
		t.Run(templateName, func(t *testing.T) {
			layouts, theme, width, height := fitReportGeometry(templateName, testutil.TemplatesDir())
			if theme == nil {
				t.Fatal("template theme unavailable")
			}
			input := &PresentationInput{
				Template: templateName, OutputFilename: "section-accent-parity.pptx",
				AccentStrategy: "section-keyed",
				ThemeOverride:  &ThemeInput{Colors: map[string]string{"accent2": "#EEEEEE"}},
				Slides: []SlideInput{
					{SlideType: "title"},
					{SlideType: "section"},
					{SlideType: "content", Pattern: &PatternInput{Name: "kpi-inline", Values: json.RawMessage(`[{"big":"42%","small":"Growth"},{"big":"1.2M","small":"ARR"}]`)}},
				},
			}
			applyDefaults(input)
			effectiveTheme, _ := theme.ApplyOverride(input.ThemeOverride.ToThemeOverride())
			expanded, _ := expandPatternsForFit(input, width, height, &effectiveTheme, layouts...)
			grid := expanded.Slides[2].ShapeGrid
			if grid == nil || len(grid.Rows) == 0 || len(grid.Rows[0].Cells) == 0 || grid.Rows[0].Cells[0].Shape == nil {
				t.Fatalf("preflight did not expand KPI pattern: %+v", grid)
			}
			if got := string(grid.Rows[0].Cells[0].Shape.Fill); got != `"accent2"` {
				t.Fatalf("preflight KPI fill = %s, want section-keyed accent2", got)
			}
			predicted := contrastPredictions(collectFitFindings(input, layouts, width, height, &effectiveTheme))
			found := false
			for _, finding := range predicted {
				if strings.HasPrefix(finding.Path, "/slides/2/") && strings.Contains(finding.Message, "(on #EEEEEE") {
					found = true
				}
			}
			if !found {
				t.Fatalf("no accent2 contrast prediction on slide 3: %+v", predicted)
			}
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
				AccentStrategy: patterns.AccentStrategy(input.AccentStrategy),
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, swap := range result.GenResult.ContrastSwaps {
				if swap.SlideIndex == 2 && swap.BackgroundColor == "#EEEEEE" {
					return
				}
			}
			t.Fatalf("render did not repair accent2 KPI text: %+v", result.GenResult.ContrastSwaps)
		})
	}
}

func TestSectionKeyedComposePreflightUsesSectionAccent(t *testing.T) {
	kpi := PatternInput{Name: "kpi-inline", Values: json.RawMessage(`[{"big":"42%","small":"Growth"},{"big":"1.2M","small":"ARR"}]`)}
	input := &PresentationInput{AccentStrategy: "section-keyed", Slides: []SlideInput{
		{SlideType: "title"}, {SlideType: "section"},
		{SlideType: "content", Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
			{Pattern: kpi, SizePct: 50}, {Pattern: kpi, SizePct: 50},
		}}},
	}}
	expanded := expandComposeForPreflight(input, 12192000, 6858000)
	grid := expanded.Slides[2].ShapeGrid
	if grid == nil {
		t.Fatal("compose preflight did not expand")
	}
	encoded, err := json.Marshal(grid)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"accent2"`) || strings.Contains(string(encoded), `"accent1"`) {
		t.Fatalf("section-keyed compose colors = %s, want accent2 only", encoded)
	}
}

// The divider number is part of the section's visual identity. A template's
// default accent1 must not remain on the badge while its KPI cards rotate to
// accent2/3 under section-keyed mode.
func TestSectionKeyedDividerNumberMatchesCardAccent(t *testing.T) {
	for _, templateName := range testutil.AllTestTemplateNames() {
		t.Run(templateName, func(t *testing.T) {
			input := &PresentationInput{
				Template: templateName, AccentStrategy: "section-keyed",
				Slides: []SlideInput{
					{SlideType: "title"},
					{SlideType: "section"},
					{SlideType: "content", Pattern: &PatternInput{Name: "kpi-2up", Values: json.RawMessage(`[{"big":"42%","small":"Growth"},{"big":"1.2M","small":"ARR"}]`)}},
					{SlideType: "section"},
					{SlideType: "content", Pattern: &PatternInput{Name: "kpi-2up", Values: json.RawMessage(`[{"big":"43%","small":"Growth"},{"big":"1.3M","small":"ARR"}]`)}},
				},
			}
			result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
				OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(), StrictFit: "off", OutputValidation: "off",
				AccentStrategy: patterns.AccentStrategySectionKeyed,
			})
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				slide       int
				num, accent string
			}{
				{2, "01", "accent2"}, {4, "02", "accent3"},
			} {
				colored := false
				for _, item := range result.SlideSpecs[tc.slide-1].Content {
					if item.Value == tc.num && item.TextColor == tc.accent {
						colored = true
					}
				}
				if !colored {
					t.Errorf("section %s badge was not assigned %s: %+v", tc.num, tc.accent, result.SlideSpecs[tc.slide-1].Content)
				}
				var badge string
				for _, shape := range matrixRenderedShapeRE.FindAllString(readSlideXML(t, result.OutputPath, "ppt/slides/slide"+strconv.Itoa(tc.slide)+".xml"), -1) {
					if strings.Contains(shape, "<a:t>"+tc.num+"</a:t>") {
						badge = shape
						break
					}
				}
				if badge == "" {
					t.Fatalf("section %s number missing from rendered slide %d", tc.num, tc.slide)
				}
				// Some template accents need a contrast-safe shade on the divider's
				// background. In that case the run is repaired to an sRGB fill;
				// otherwise it retains the assigned scheme color.
				if !sectionBadgeRunFillRE.MatchString(badge) {
					t.Errorf("section %s badge lacks an explicit accent-derived run fill", tc.num)
				}
			}
		})
	}
}
