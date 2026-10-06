package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

var predictedContrastColors = regexp.MustCompile(`— (#[0-9A-Fa-f]{6}) → (#[0-9A-Fa-f]{6}) \(on (#[0-9A-Fa-f]{6}),`)

// contrastDecision is one colour replacement as both validate (a
// contrast_predicted finding) and generate (a ContrastSwap) describe it.
type contrastDecision struct {
	Slide                 int
	Original, Replacement string
	Background            string
}

func contrastDecisionsFromFindings(findings []patterns.FitFinding) (map[contrastDecision]bool, error) {
	decisions := make(map[contrastDecision]bool)
	for _, finding := range findings {
		colors := predictedContrastColors.FindStringSubmatch(finding.Message)
		if len(colors) != 4 {
			return nil, fmt.Errorf("cannot read predicted contrast colors from %q", finding.Message)
		}
		slide := slidepath.SlideIndex(finding.Path)
		if slide < 0 {
			return nil, fmt.Errorf("invalid contrast finding path %q", finding.Path)
		}
		decision := contrastDecision{Slide: slide, Original: strings.ToUpper(colors[1]),
			Replacement: strings.ToUpper(colors[2]), Background: strings.ToUpper(colors[3])}
		decisions[decision] = true
	}
	return decisions, nil
}

// The section-number placeholder of midnight-blue's Section Divider layout, as
// shipped: one styled level, in the accent.
const (
	sectionNumberLayoutPart = "ppt/slideLayouts/slideLayout4.xml"
	shippedSectionNumberLvl = `<a:defRPr sz="13600"><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>`
	// lt2 on the white section divider is about 1.2:1 at any size.
	lowContrastLvl2 = `<a:lvl2pPr marL="457200" indent="0"><a:buNone/><a:defRPr sz="1600"><a:solidFill><a:schemeClr val="lt2"/></a:solidFill></a:defRPr></a:lvl2pPr>`
)

// midnightBlueWithSectionNumberStyle writes a copy of midnight-blue whose
// section-number list style ends in replacement instead of the shipped level.
// The private template that exposed go-slide-creator-6s2w4 is not in the
// repository, so the fixture reproduces its shape on a shipped one.
func midnightBlueWithSectionNumberStyle(t *testing.T, replacement string) string {
	t.Helper()
	r, err := zip.OpenReader(filepath.Join(testutil.TemplatesDir(), "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "midnight-blue.pptx"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	patched := false
	for _, entry := range r.File {
		if entry.Name != sectionNumberLayoutPart {
			if err := w.Copy(entry); err != nil {
				t.Fatal(err)
			}
			continue
		}
		src, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(body, []byte(shippedSectionNumberLvl)) != 1 {
			t.Fatalf("%s no longer carries the section-number list style this fixture patches", entry.Name)
		}
		body = bytes.Replace(body, []byte(shippedSectionNumberLvl), []byte(replacement), 1)
		header := entry.FileHeader
		header.Method = zip.Deflate
		dst, err := w.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(body); err != nil {
			t.Fatal(err)
		}
		patched = true
	}
	if !patched {
		t.Fatalf("midnight-blue has no %s", sectionNumberLayoutPart)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sectionDeckContrastDecisions returns what validate predicts and what
// generate swaps for a one-section deck on the template in templatesDir.
func sectionDeckContrastDecisions(t *testing.T, templatesDir string) (predicted, actual map[contrastDecision]bool) {
	t.Helper()
	layouts, theme, width, height := fitReportGeometry("midnight-blue", templatesDir)
	if layouts == nil || theme == nil {
		t.Fatal("cannot analyze the fixture template")
	}
	input := &PresentationInput{Template: "midnight-blue", OutputFilename: "list-level-parity.pptx", Slides: []SlideInput{{
		// Named outright: a deck that opens on a section is laid out as a title.
		SlideType: "section", LayoutID: "slideLayout4",
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Market analysis")}},
	}}}
	applyDefaults(input)
	resolveCanonicalLayoutIDs(input.Slides, layouts)
	predicted, err := contrastDecisionsFromFindings(contrastPredictions(collectFitFindings(input, layouts, width, height, theme)))
	if err != nil {
		t.Fatal(err)
	}
	result, cleanup, err := RunPresentation(context.Background(), input, RenderOptions{
		OutputDir: t.TempDir(), TemplatesDir: templatesDir, StrictFit: "off", OutputValidation: "off",
	})
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		t.Fatal(err)
	}
	actual = make(map[contrastDecision]bool)
	for _, swap := range result.GenResult.ContrastSwaps {
		actual[contrastDecision{Slide: swap.SlideIndex,
			Original: strings.ToUpper(swap.OriginalColor), Replacement: strings.ToUpper(swap.ReplacedColor),
			Background: strings.ToUpper(swap.BackgroundColor)}] = true
	}
	return predicted, actual
}

// TestContrastParityIgnoresUnusedListLevels is go-slide-creator-6s2w4 without
// the private template: a low-contrast colour on a list level the section
// number never uses draws no swap at generate time, so validate (which models
// the text actually placed) and generate agree. The same colour on the level
// the number does use is predicted and swapped, which proves the fixture's
// colour is one the pass would act on.
func TestContrastParityIgnoresUnusedListLevels(t *testing.T) {
	baselinePredicted, baselineActual := sectionDeckContrastDecisions(t, testutil.TemplatesDir())
	if !reflect.DeepEqual(baselinePredicted, baselineActual) {
		t.Fatalf("shipped template is out of parity before the fixture is applied: predicted=%v actual=%v", baselinePredicted, baselineActual)
	}

	t.Run("low contrast on an unused level", func(t *testing.T) {
		dir := midnightBlueWithSectionNumberStyle(t, strings.Replace(shippedSectionNumberLvl, `</a:lvl1pPr>`, `</a:lvl1pPr>`+lowContrastLvl2, 1))
		predicted, actual := sectionDeckContrastDecisions(t, dir)
		if !reflect.DeepEqual(predicted, actual) {
			t.Errorf("contrast decision-set mismatch: predicted=%v actual=%v", predicted, actual)
		}
		if !reflect.DeepEqual(actual, baselineActual) {
			t.Errorf("an unused level changed what generate swaps: got %v, shipped template gives %v", actual, baselineActual)
		}
	})

	t.Run("low contrast on the used level", func(t *testing.T) {
		dir := midnightBlueWithSectionNumberStyle(t, strings.Replace(shippedSectionNumberLvl, `val="accent1"`, `val="lt2"`, 1))
		predicted, actual := sectionDeckContrastDecisions(t, dir)
		if !reflect.DeepEqual(predicted, actual) {
			t.Errorf("contrast decision-set mismatch: predicted=%v actual=%v", predicted, actual)
		}
		swapped := false
		for decision := range actual {
			if decision.Original == "#E8ECF1" {
				swapped = true
			}
		}
		if !swapped {
			t.Errorf("lt2 on the section number's own level was not swapped: %v", actual)
		}
	})
}
