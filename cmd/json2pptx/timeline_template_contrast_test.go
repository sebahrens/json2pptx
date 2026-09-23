package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
)

// Expand through the template-aware CLI path. Unit tests with hand-written
// palettes can pass while a shipped template still selects unreadable ink.
func TestTimelineChevronTextContrastOnLocalTemplateCorpus(t *testing.T) {
	stops := make([]patterns.TimelineStop, 7)
	for i := range stops {
		stops[i] = patterns.TimelineStop{Label: "Wave close", Body: "Milestone detail", Date: "Q1"}
	}
	values, err := json.Marshal(stops)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range testutil.TestTemplatePaths() {
		name := strings.TrimSuffix(filepath.Base(path), ".pptx")
		t.Run(name, func(t *testing.T) {
			ctx, _, err := resolveExpandContext(name, testutil.TemplatesDir())
			if err != nil {
				t.Fatal(err)
			}
			grid, _, err := expandPattern(&PatternInput{
				Name: "timeline-horizontal", Values: values,
				Overrides: json.RawMessage(`{"style":"chevron"}`),
			}, ctx, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			if len(grid.Rows) == 0 {
				t.Fatal("expanded timeline has no rows")
			}
			if len(grid.Rows[0].Cells) != len(stops) {
				t.Fatalf("chevron row has %d stops, want %d", len(grid.Rows[0].Cells), len(stops))
			}
			for i, cell := range grid.Rows[0].Cells {
				var fill struct {
					Color string `json:"color"`
					Tint  int    `json:"tint"`
					Shade int    `json:"shade"`
				}
				if err := json.Unmarshal(cell.Shape.Fill, &fill); err != nil {
					// The unmodified midpoint uses a bare scheme string.
					if err := json.Unmarshal(cell.Shape.Fill, &fill.Color); err != nil {
						t.Fatal(err)
					}
				}
				base := timelineThemeColor(t, ctx.Theme, fill.Color)
				bg := timelineThemeColor(t, ctx.Theme, "lt1")
				paint := patterns.EffectiveColorMods(base, patterns.ColorMods{Tint: fill.Tint, Shade: fill.Shade, Alpha: 1}, bg)
				var content struct {
					Paragraphs []struct {
						Color string `json:"color"`
					} `json:"paragraphs"`
				}
				if err := json.Unmarshal(cell.Shape.Text, &content); err != nil {
					t.Fatal(err)
				}
				if len(content.Paragraphs) != 2 {
					t.Fatalf("stop %d has %d paragraphs, want label and body", i, len(content.Paragraphs))
				}
				for _, para := range content.Paragraphs {
					ink := timelineThemeColor(t, ctx.Theme, para.Color)
					if ratio := ink.ContrastWith(paint); ratio < 4.5 {
						t.Errorf("stop %d text %s on %s: contrast %.2f < 4.5", i, para.Color, paint.Hex(), ratio)
					}
				}
			}
		})
	}
}

func timelineThemeColor(t *testing.T, theme types.ThemeInfo, name string) svggen.Color {
	t.Helper()
	for _, c := range theme.Colors {
		if c.Name == name {
			parsed, err := svggen.ParseColor(c.RGB)
			if err != nil {
				t.Fatal(err)
			}
			return parsed
		}
	}
	t.Fatalf("theme has no color %q", name)
	return svggen.Color{}
}

func TestTimelineChevronWarnsBeforeDenseBodyClips(t *testing.T) {
	ctx, _, err := resolveExpandContext("midnight-blue", testutil.TemplatesDir())
	if err != nil {
		t.Fatal(err)
	}
	stops := make([]patterns.TimelineStop, 7)
	for i := range stops {
		stops[i] = patterns.TimelineStop{Label: "Wave close", Body: "Fleet migrated", Date: "Q1"}
	}
	stops[4].Body = "Sheffield, Hull and Grenoble migrated; fleet stabilisation underway by June"
	values, err := json.Marshal(stops)
	if err != nil {
		t.Fatal(err)
	}
	pattern := &PatternInput{
		Name: "timeline-horizontal", Values: values,
		Overrides: json.RawMessage(`{"style":"chevron"}`),
	}
	_, warnings, err := expandPattern(pattern, ctx, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	warned := false
	bodyWarnings := 0
	for _, warning := range warnings {
		if strings.Contains(warning, "BODY_TOO_LONG") && strings.Contains(warning, ".body") {
			bodyWarnings++
			if strings.Contains(warning, "values[4].body") {
				warned = true
			}
		}
	}
	if !warned {
		t.Fatalf("75-character body fits the old character budget but clips in the chevron; warnings: %v", warnings)
	}
	if bodyWarnings != 1 {
		t.Errorf("short control bodies should fit; got %d body warnings: %v", bodyWarnings, warnings)
	}
	a := loadTemplateAnalysis(t, "midnight-blue")
	input := &PresentationInput{Slides: []SlideInput{{SlideType: "content", Pattern: pattern}}}
	found := false
	for _, finding := range collectFitFindings(input, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
		if finding.Code == patterns.ErrCodeBodyTooLong && finding.Path == "/slides/0/pattern" &&
			strings.Contains(finding.Message, "values[4].body") {
			found = true
		}
	}
	if !found {
		t.Error("measured chevron overflow did not reach the slide fit report")
	}
}

func TestTimelineChevronBodySizeOverrideChangesFitFinding(t *testing.T) {
	a := loadTemplateAnalysis(t, "midnight-blue")
	stops := make([]patterns.TimelineStop, 7)
	for i := range stops {
		stops[i] = patterns.TimelineStop{Label: "Wave close"}
	}
	stops[4].Body = "Fleet migrated"
	values, err := json.Marshal(stops)
	if err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{Slides: []SlideInput{{
		SlideType: "content",
		Pattern: &PatternInput{Name: "timeline-horizontal", Values: values,
			Overrides: json.RawMessage(`{"style":"chevron"}`)},
	}}}
	countBodyFindings := func() int {
		count := 0
		for _, finding := range collectFitFindings(input, a.Layouts, a.SlideWidth, a.SlideHeight, &a.Theme) {
			if finding.Code == patterns.ErrCodeBodyTooLong && strings.Contains(finding.Message, "values[4].body") {
				count++
			}
		}
		return count
	}
	if got := countBodyFindings(); got != 0 {
		t.Fatalf("default 12pt body should fit, got %d warnings", got)
	}
	input.Slides[0].Pattern.Overrides = json.RawMessage(`{"style":"chevron","body_size":30}`)
	if got := countBodyFindings(); got != 1 {
		t.Errorf("explicit 30pt body should warn exactly once, got %d", got)
	}
}

func TestTimelineChevronKeepsDarkInkInGeneratedDeck(t *testing.T) {
	stops := make([]patterns.TimelineStop, 7)
	for i := range stops {
		stops[i] = patterns.TimelineStop{Label: fmt.Sprintf("Stop %d", i+1), Body: "Milestone detail", Date: "Q1"}
	}
	values, err := json.Marshal(stops)
	if err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{
		Template: "midnight-blue", OutputFilename: "timeline-contrast.pptx",
		Slides: []SlideInput{{
			SlideType: "content",
			Pattern: &PatternInput{
				Name: "timeline-horizontal", Values: values,
				Overrides: json.RawMessage(`{"style":"chevron"}`),
			},
		}},
	}
	applyDefaults(input)
	result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: testutil.TemplatesDir(),
		StrictFit: "warn", OutputValidation: "strict",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(result.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var slideXML []byte
	for _, file := range z.File {
		if file.Name != "ppt/slides/slide1.xml" {
			continue
		}
		r, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		slideXML, err = io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if len(slideXML) == 0 {
		t.Fatal("generated deck has no slide1.xml")
	}
	decoder := xml.NewDecoder(strings.NewReader(string(slideXML)))
	seen := make(map[string]bool)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "r" {
			continue
		}
		var run struct {
			Text  string `xml:"t"`
			Props struct {
				Fill struct {
					Scheme struct {
						Value string `xml:"val,attr"`
					} `xml:"schemeClr"`
					SRGB struct {
						Value string `xml:"val,attr"`
					} `xml:"srgbClr"`
				} `xml:"solidFill"`
			} `xml:"rPr"`
		}
		if err := decoder.DecodeElement(&run, &start); err != nil {
			t.Fatal(err)
		}
		if run.Text == "Stop 5" || run.Text == "Stop 6" || run.Text == "Stop 7" {
			seen[run.Text] = true
			var ink svggen.Color
			if name := run.Props.Fill.Scheme.Value; name != "" {
				ink = timelineThemeColor(t, result.TemplateTheme, name)
			} else if hex := run.Props.Fill.SRGB.Value; hex != "" {
				ink, err = svggen.ParseColor("#" + hex)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				t.Errorf("%s has no explicit ink in generated OOXML", run.Text)
				continue
			}
			if ratio := ink.ContrastWith(svggen.MustParseColor("#FFFFFF")); ratio < 4.5 {
				t.Errorf("%s has light %s ink in generated OOXML (contrast %.2f against white)", run.Text, ink.Hex(), ratio)
			}
		}
	}
	if len(seen) != 3 {
		t.Errorf("found dark-color evidence for %d of 3 tinted stops", len(seen))
	}
}
