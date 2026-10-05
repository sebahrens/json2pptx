package generator

import (
	"archive/zip"
	"context"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// The footer line of persona f's proposal (go-slide-creator-m2tlt): on a 3in
// footer slot it rendered "Confidential — Project Falcon | Meridian Capital…"
// on every slide with no finding.
const (
	falconFull      = "Confidential — Project Falcon | Meridian Capital Partners | October 2026"
	falconNoProject = "Confidential — Meridian Capital Partners | October 2026"
	falconNoDate    = "Confidential — Meridian Capital Partners"
)

func falconFooter() *FooterConfig {
	return &FooterConfig{
		Enabled:      true,
		LeftText:     falconFull,
		LeftTextPath: "/chrome",
		LeftFallbacks: []FooterFallback{
			{Dropped: []string{"project_code"}, LeftText: falconNoProject},
			{Dropped: []string{"project_code", "footer_date"}, LeftText: falconNoDate},
		},
	}
}

// footerBoxFor returns the narrowest box width (EMU, insets included) that
// holds text on one line at the footer floor, plus slack.
func footerBoxFor(t *testing.T, text string, slackEMU int64) int64 {
	t.Helper()
	w := footerTextWidthEMU(text, footerMinFontSize, "Arial")
	if w <= 0 {
		t.Skip("no font metrics available")
	}
	return w + 2*91440 + slackEMU
}

func TestResolveFooterLineDropsByPriority(t *testing.T) {
	cfg := falconFooter()
	wide := footerBoxFor(t, falconFull, 50000)
	fitsNoProject := footerBoxFor(t, falconNoProject, 20000)
	fitsNoDate := footerBoxFor(t, falconNoDate, 20000)

	for _, tc := range []struct {
		name          string
		width         int64
		wantLevel     int
		wantLine      string
		wantDropped   []string
		wantTruncated bool
		wantFinding   bool
	}{
		{name: "full line fits", width: wide, wantLine: falconFull},
		{name: "project code goes first", width: fitsNoProject, wantLevel: 1, wantLine: falconNoProject, wantDropped: []string{"project_code"}, wantFinding: true},
		{name: "then the date", width: fitsNoDate, wantLevel: 2, wantLine: falconNoDate, wantDropped: []string{"project_code", "footer_date"}, wantFinding: true},
		{name: "then an ellipsis, reported", width: fitsNoDate / 2, wantLevel: 2, wantLine: falconNoDate, wantDropped: []string{"project_code", "footer_date"}, wantTruncated: true, wantFinding: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fit := resolveFooterLine(cfg, []footerLineBox{{slideIndex: 1, widthEMU: tc.width}}, "Arial")
			if fit.levels[1] != tc.wantLevel {
				t.Fatalf("level = %d, want %d", fit.levels[1], tc.wantLevel)
			}
			if got := cfg.withLeftLevels(fit.levels).LeftTextFor(1); got != tc.wantLine {
				t.Errorf("line = %q, want %q", got, tc.wantLine)
			}
			findings := fit.findings(cfg)
			if (len(findings) == 1) != tc.wantFinding || len(findings) > 1 {
				t.Fatalf("findings = %+v, want finding=%t", findings, tc.wantFinding)
			}
			if len(findings) == 0 {
				return
			}
			f := findings[0]
			if f.Code != patterns.ErrCodeChromeTruncated || f.Action != "review" || f.Path != "/chrome" {
				t.Errorf("finding = %s %s at %s, want CHROME_TRUNCATED review at /chrome", f.Code, f.Action, f.Path)
			}
			params := f.Fix.Params
			if got, _ := params["dropped"].([]string); strings.Join(got, ",") != strings.Join(tc.wantDropped, ",") {
				t.Errorf("dropped = %v, want %v", params["dropped"], tc.wantDropped)
			}
			if params["truncated"] != tc.wantTruncated {
				t.Errorf("truncated = %v, want %t", params["truncated"], tc.wantTruncated)
			}
			if got, _ := params["slides"].([]int); len(got) != 1 || got[0] != 2 {
				t.Errorf("slides = %v, want [2] (1-based)", params["slides"])
			}
			maxChars, _ := params["max_chars"].(int)
			lineChars, _ := params["line_chars"].(int)
			if lineChars != len([]rune(falconFull)) || maxChars <= 0 || maxChars >= lineChars {
				t.Errorf("max_chars = %d of line_chars = %d, want 0 < max_chars < %d", maxChars, lineChars, len([]rune(falconFull)))
			}
			// The reported budget is the real one: that many characters fit
			// the slot, one more does not.
			fits, _ := footerLineMeasure("Arial")
			runes := []rune(falconFull)
			if !fits(string(runes[:maxChars]), tc.width, footerMinFontSize) || fits(string(runes[:maxChars+1]), tc.width, footerMinFontSize) {
				t.Errorf("max_chars = %d is not the slot's capacity", maxChars)
			}
			for _, want := range tc.wantDropped {
				if !strings.Contains(f.Message, want) {
					t.Errorf("message does not name dropped field %q: %s", want, f.Message)
				}
			}
			if got := strings.Contains(f.Message, "ellipsis"); got != tc.wantTruncated {
				t.Errorf("message mentions an ellipsis = %t, want %t: %s", got, tc.wantTruncated, f.Message)
			}
		})
	}
}

// The level is chosen per footer slot. Slides that share a slot carry the same
// fields even when only one of them (a long section crumb) overflows; a slide
// on a narrower slot does not cost the others their project code.
func TestResolveFooterLineIsPerSlot(t *testing.T) {
	cfg := falconFooter()
	cfg.LeftTextBySlide = []string{"", "", falconFull + " | Market", ""}
	cfg.LeftFallbacks[0].LeftTextBySlide = []string{"", "", falconNoProject + " | Market", ""}
	cfg.LeftFallbacks[1].LeftTextBySlide = []string{"", "", falconNoDate + " | Market", ""}

	content := footerBoxFor(t, falconFull, 20000) // holds the full line, not the crumb
	divider := footerBoxFor(t, falconNoDate, 20000)
	wideOnly := footerBoxFor(t, falconFull+" | Market", 900000)
	fit := resolveFooterLine(cfg, []footerLineBox{
		{slideIndex: 0, widthEMU: wideOnly},
		{slideIndex: 1, widthEMU: content},
		{slideIndex: 2, widthEMU: content},
		{slideIndex: 3, widthEMU: divider},
	}, "Arial")

	deck := cfg.withLeftLevels(fit.levels)
	for i, want := range []string{falconFull, falconNoProject, falconNoProject + " | Market", falconNoDate} {
		if got := deck.LeftTextFor(i); got != want {
			t.Errorf("slide %d footer = %q, want %q", i+1, got, want)
		}
	}
	if cfg.LeftText != falconFull || len(cfg.LeftFallbacks) != 2 || cfg.LeftTextBySlide[1] != "" {
		t.Error("withLeftLevels must not mutate the caller's config")
	}

	findings := fit.findings(cfg)
	if len(findings) != 2 {
		t.Fatalf("findings = %+v, want one per slot that fell short", findings)
	}
	if got, _ := findings[0].Fix.Params["slides"].([]int); len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("first finding slides = %v, want [2 3]", findings[0].Fix.Params["slides"])
	}
	// The budget is the overflowing slide's, not its slot-mate's.
	if !strings.Contains(findings[0].Message, "| Market") {
		t.Errorf("first finding does not quote the line that overflowed: %s", findings[0].Message)
	}
	if got, _ := findings[1].Fix.Params["slides"].([]int); len(got) != 1 || got[0] != 4 {
		t.Errorf("second finding slides = %v, want [4]", findings[1].Fix.Params["slides"])
	}
}

// A legacy footer with nothing to drop is still reported when it is cut.
func TestResolveFooterLineLegacyTextIsReported(t *testing.T) {
	legacy := &FooterConfig{Enabled: true, LeftText: falconFull}
	fit := resolveFooterLine(legacy, []footerLineBox{{slideIndex: 0, widthEMU: footerBoxFor(t, falconNoDate, 0)}}, "Arial")
	if len(fit.levels) != 0 {
		t.Errorf("levels = %v, want none: a legacy line has no fallback", fit.levels)
	}
	findings := fit.findings(legacy)
	if len(findings) != 1 || findings[0].Path != "/footer/left_text" || findings[0].Fix.Params["truncated"] != true {
		t.Fatalf("legacy findings = %+v, want one truncated CHROME_TRUNCATED at /footer/left_text", findings)
	}
	if _, has := findings[0].Fix.Params["dropped"]; has {
		t.Errorf("a legacy line has no field to drop: %+v", findings[0].Fix.Params)
	}
}

var footerLeftTextRE = regexp.MustCompile(`(?s)name="Footer Left".*?<a:t>(.*?)</a:t>`)

// longFooter overflows every shipped template's footer slot (the widest is
// about 12in) until the project code goes.
func longFooter(clientName string) *FooterConfig {
	project := "Project Falcon" + strings.Repeat(" Commercial Due Diligence", 8)
	return &FooterConfig{
		Enabled:      true,
		LeftText:     "Strictly confidential — " + project + " | " + clientName + " | October 2026",
		LeftTextPath: "/chrome",
		LeftFallbacks: []FooterFallback{
			{Dropped: []string{"project_code"}, LeftText: "Strictly confidential — " + clientName + " | October 2026"},
			{Dropped: []string{"project_code", "footer_date"}, LeftText: "Strictly confidential — " + clientName},
		},
	}
}

// TestGenerateDropsFooterSegmentInsteadOfEllipsis renders a deck whose footer
// line is too wide for the template and checks the slide XML: the project code
// is gone, the client name is whole wherever the finding does not say the line
// is still cut, and the result carries CHROME_TRUNCATED.
func TestGenerateDropsFooterSegmentInsteadOfEllipsis(t *testing.T) {
	const client = "Meridian Capital Partners Infrastructure Fund Management Limited"
	for _, name := range []string{"midnight-blue", "modern-template", "forest-green", "warm-coral", "p-style"} {
		t.Run(name, func(t *testing.T) {
			templatePath := "../../templates/" + name + ".pptx"
			reader, err := template.OpenTemplate(templatePath)
			if err != nil {
				if name == "p-style" {
					t.Skip("local p-style template not present")
				}
				t.Fatalf("open template: %v", err)
			}
			layouts, err := template.ParseLayouts(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatalf("parse layouts: %v", err)
			}
			var content *types.LayoutMetadata
			for i := range layouts {
				if layouts[i].CanonicalType == types.CanonicalLayoutOneContent || layouts[i].Name == "One Content" {
					content = &layouts[i]
					break
				}
			}
			if content == nil {
				t.Fatal("template has no One Content layout")
			}

			cfg := longFooter(client)
			output := filepath.Join(t.TempDir(), name+".pptx")
			result, err := Generate(context.Background(), GenerationRequest{
				TemplatePath:          templatePath,
				OutputPath:            output,
				ExcludeTemplateSlides: true,
				Slides: []SlideSpec{
					{LayoutID: content.ID, Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "First"}}},
					{LayoutID: content.ID, Content: []ContentItem{{PlaceholderID: "title", Type: ContentText, Value: "Second"}}},
				},
				Footer: cfg,
			})
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if want := longFooter(client); cfg.LeftText != want.LeftText || len(cfg.LeftFallbacks) != 2 || len(cfg.LeftTextBySlide) != 0 {
				t.Error("Generate mutated the caller's footer config")
			}

			var rendered *patterns.FitFinding
			for i := range result.FitFindings {
				if result.FitFindings[i].Code == patterns.ErrCodeChromeTruncated {
					if rendered != nil {
						t.Fatalf("CHROME_TRUNCATED reported twice for one footer slot: %+v", result.FitFindings)
					}
					rendered = &result.FitFindings[i]
				}
			}
			if rendered == nil {
				t.Fatalf("no CHROME_TRUNCATED finding: %+v", result.FitFindings)
			}
			dropped, _ := rendered.Fix.Params["dropped"].([]string)
			if len(dropped) == 0 || dropped[0] != "project_code" {
				t.Fatalf("dropped = %v, want project_code first: %s", dropped, rendered.Message)
			}

			zr, err := zip.OpenReader(output)
			if err != nil {
				t.Fatalf("open output: %v", err)
			}
			defer zr.Close()
			truncated := rendered.Fix.Params["truncated"] == true
			var lines []string
			for _, f := range zr.File {
				if !strings.HasPrefix(f.Name, "ppt/slides/slide") || !strings.HasSuffix(f.Name, ".xml") {
					continue
				}
				r, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(r)
				_ = r.Close()
				if err != nil {
					t.Fatal(err)
				}
				m := footerLeftTextRE.FindSubmatch(data)
				if m == nil {
					t.Fatalf("%s has no left footer", f.Name)
				}
				lines = append(lines, string(m[1]))
			}
			if len(lines) != 2 || lines[0] != lines[1] {
				t.Fatalf("footer lines differ between slides: %q", lines)
			}
			line := lines[0]
			if strings.Contains(line, "Project Falcon") {
				t.Errorf("project code still in the footer: %q", line)
			}
			if got := strings.Contains(line, "…"); got != truncated {
				t.Errorf("footer %q ellipsized = %t, but the finding says truncated = %t", line, got, truncated)
			}
			if !truncated && !strings.Contains(line, client) {
				t.Errorf("client name is not whole in %q", line)
			}
		})
	}
}
