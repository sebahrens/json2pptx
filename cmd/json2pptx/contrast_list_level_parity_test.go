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
	return midnightBlueWithPatchedPart(t, sectionNumberLayoutPart, shippedSectionNumberLvl, replacement)
}

// midnightBlueWithPatchedPart writes a copy of midnight-blue in which the one
// occurrence of shipped in the named part reads replacement, and returns the
// directory holding it.
func midnightBlueWithPatchedPart(t *testing.T, part, shipped, replacement string) string {
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
		if entry.Name != part {
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
		if bytes.Count(body, []byte(shipped)) != 1 {
			t.Fatalf("%s no longer carries the markup this fixture patches: %s", entry.Name, shipped)
		}
		body = bytes.Replace(body, []byte(shipped), []byte(replacement), 1)
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
		t.Fatalf("midnight-blue has no %s", part)
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
	return deckContrastDecisions(t, templatesDir, SlideInput{
		// Named outright: a deck that opens on a section is laid out as a title.
		SlideType: "section", LayoutID: "slideLayout4",
		Content: []ContentInput{{PlaceholderID: "title", Type: "text", TextValue: strPtr("Market analysis")}},
	})
}

// deckContrastDecisions returns what validate predicts and what generate swaps
// for a one-slide deck on the midnight-blue copy in templatesDir.
func deckContrastDecisions(t *testing.T, templatesDir string, slide SlideInput) (predicted, actual map[contrastDecision]bool) {
	t.Helper()
	layouts, theme, width, height := fitReportGeometry("midnight-blue", templatesDir)
	if layouts == nil || theme == nil {
		t.Fatal("cannot analyze the fixture template")
	}
	input := &PresentationInput{Template: "midnight-blue", OutputFilename: "list-level-parity.pptx", Slides: []SlideInput{slide}}
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

// The body placeholder of midnight-blue's One Content layout and the second
// body level of its master, as shipped: the layout leaves every level to the
// master, whose levels are all dk2.
const (
	contentLayoutPart      = "ppt/slideLayouts/slideLayout2.xml"
	shippedContentBodyList = `<p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>`
	masterPart             = "ppt/slideMasters/slideMaster1.xml"
	shippedMasterBodyLvl2  = `<a:defRPr sz="1800" kern="1200"><a:solidFill><a:schemeClr val="dk2"/>`
)

// bulletContentSlide is a One Content slide whose body carries the given bullets; a
// bullet with leading whitespace is a sub-bullet, one list level down.
func bulletContentSlide(bullets ...string) SlideInput {
	return SlideInput{
		SlideType: "content", LayoutID: "slideLayout2",
		Content: []ContentInput{
			{PlaceholderID: "title", Type: "text", TextValue: strPtr("Renewals carried the quarter")},
			{PlaceholderID: "body", Type: "bullets", BulletsValue: &bullets},
		},
	}
}

func decisionsSwap(decisions map[contrastDecision]bool, original string) bool {
	for decision := range decisions {
		if decision.Original == original {
			return true
		}
	}
	return false
}

// TestContrastParityOnUsedDeeperListLevel is go-slide-creator-50xzk: a
// low-contrast colour on the SECOND list level of a body placeholder, which a
// sub-bullet does use. Generate swaps it; validate used to model the first
// level only and said nothing. The colour is stated once by the layout's own
// list style (the lstStyle pass fixes it) and once by the master's body style
// (the inherited pass fixes it), and in both the prediction now names the
// swap. The same templates with no sub-bullet draw neither.
func TestContrastParityOnUsedDeeperListLevel(t *testing.T) {
	nested := bulletContentSlide("Renewals grew", "  Enterprise led", "Churn fell")
	flat := bulletContentSlide("Renewals grew", "Enterprise led", "Churn fell")
	const lt2 = "#E8ECF1"

	predicted, actual := deckContrastDecisions(t, testutil.TemplatesDir(), nested)
	if !reflect.DeepEqual(predicted, actual) {
		t.Fatalf("shipped template is out of parity before the fixture is applied: predicted=%v actual=%v", predicted, actual)
	}
	if decisionsSwap(actual, lt2) {
		t.Fatalf("shipped template already swaps %s: %v", lt2, actual)
	}

	fixtures := []struct {
		name, part, shipped, replacement string
	}{
		{"layout list style", contentLayoutPart, shippedContentBodyList,
			strings.Replace(shippedContentBodyList, `<a:lstStyle/>`, `<a:lstStyle>`+lowContrastLvl2+`</a:lstStyle>`, 1)},
		{"master body style", masterPart, shippedMasterBodyLvl2,
			strings.Replace(shippedMasterBodyLvl2, `val="dk2"`, `val="lt2"`, 1)},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			dir := midnightBlueWithPatchedPart(t, fixture.part, fixture.shipped, fixture.replacement)

			predicted, actual := deckContrastDecisions(t, dir, nested)
			if !reflect.DeepEqual(predicted, actual) {
				t.Errorf("sub-bullet on the low-contrast level: predicted=%v actual=%v", predicted, actual)
			}
			if !decisionsSwap(actual, lt2) {
				t.Errorf("lt2 on the level the sub-bullet uses was not swapped: %v", actual)
			}

			predicted, actual = deckContrastDecisions(t, dir, flat)
			if !reflect.DeepEqual(predicted, actual) {
				t.Errorf("no sub-bullet: predicted=%v actual=%v", predicted, actual)
			}
			if decisionsSwap(actual, lt2) || decisionsSwap(predicted, lt2) {
				t.Errorf("a level no bullet uses drew a decision: predicted=%v actual=%v", predicted, actual)
			}
		})
	}
}
