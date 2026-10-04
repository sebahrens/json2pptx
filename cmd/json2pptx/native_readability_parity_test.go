package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/testutil"
)

// nativeReadabilityDecks are the shipped native-diagram examples plus a
// fixture dense enough that generation refuses it on the short templates.
var nativeReadabilityDecks = []string{
	filepath.Join("..", "..", "examples", "diagrams", "business_model_canvas.json"),
	filepath.Join("..", "..", "examples", "diagrams", "nine_box_talent.json"),
	filepath.Join("..", "..", "examples", "diagrams", "porters_five_forces.json"),
	filepath.Join("..", "..", "examples", "diagrams", "swot.json"),
	filepath.Join("..", "..", "examples", "diagrams", "pyramid.json"),
	filepath.Join("testdata", "native_readability", "bmc_dense.json"),
}

// validateRefusesReadability reports whether validate --fit-report, for the
// template, raises the error-severity TEXT_BELOW_READABLE_MIN generation
// refuses on.
func validateRefusesReadability(t *testing.T, path, template string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	applyDefaults(&input)
	for _, f := range fitFindingsForInput(&input, template, testTemplatesDir, false) {
		if f.Code == patterns.ErrCodeTextBelowReadableMin && f.Action == "refuse" && f.Severity == "error" {
			return true
		}
	}
	return false
}

// TestNativeDiagramReadabilityValidateGenerateParity pins validate and
// generate to one answer for native diagrams: every example/template pair
// generate refuses as unreadable carries an error-severity
// TEXT_BELOW_READABLE_MIN from validate --fit-report, and validate raises no
// such error for a pair generate publishes (go-slide-creator-y72c3). The five
// shipped examples must publish on every template (go-slide-creator-zbo58).
func TestNativeDiagramReadabilityValidateGenerateParity(t *testing.T) {
	templates := testutil.AllTestTemplateNames()
	if testing.Short() {
		// One of the two shortest diagram placeholders (where refusals
		// happen) and one full-coverage template (where they do not): the
		// two directions the test needs. Every template runs without -short
		// and in the integration corpus job's TestShortReducedMatricesCorpus
		// (go-slide-creator-q7cpq).
		templates = []string{"modern", "midnight-blue"}
	}
	refusedAnywhere, publishedDense := false, false
	for _, deck := range nativeReadabilityDecks {
		shipped := strings.Contains(deck, "examples")
		for _, template := range templates {
			t.Run(filepath.Base(deck)+"/"+template, func(t *testing.T) {
				validateRefuses := validateRefusesReadability(t, deck, template)
				err := runJSONMode(deck, "", testTemplatesDir, t.TempDir(), "", false, false, template, "warn", false, "strict", "", false)
				generateRefuses := err != nil && strings.Contains(err.Error(), "unreadable generated text")
				if err != nil && !generateRefuses {
					t.Fatalf("generate failed for another reason: %v", err)
				}
				if validateRefuses != generateRefuses {
					t.Fatalf("validate refuses=%v but generate refuses=%v (%v)", validateRefuses, generateRefuses, err)
				}
				if shipped && generateRefuses {
					t.Fatalf("shipped example refused: %v", err)
				}
				if generateRefuses {
					refusedAnywhere = true
				} else if !shipped {
					publishedDense = true
				}
			})
		}
	}
	if !refusedAnywhere || !publishedDense {
		t.Fatalf("the dense fixture must refuse on a short template and publish on a tall one (refused=%v published=%v); the parity test has lost a direction", refusedAnywhere, publishedDense)
	}
}

// TestReadabilityRefusalNamesSlideAndPartialSkipsIt covers
// go-slide-creator-ygaln: the refusal names the authored slide, layout,
// diagram and JSON path, and --partial writes the rest of the deck with a
// CONTENT_DROPPED finding for the refused slide.
func TestReadabilityRefusalNamesSlideAndPartialSkipsIt(t *testing.T) {
	deck := filepath.Join("testdata", "native_readability", "bmc_dense.json")
	dir := t.TempDir()

	report := filepath.Join(dir, "refused.json")
	err := runJSONMode(deck, report, testTemplatesDir, filepath.Join(dir, "refused"), "", false, false, "modern", "warn", false, "strict", "", false)
	if err == nil {
		t.Fatal("dense canvas generated on modern")
	}
	for _, want := range []string{"slide 2 (layout ", `diagram "business_model_canvas"`, "at /slides/1/content/1/diagram_value", "unreadable generated text"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not contain %q", err, want)
		}
	}
	var refused JSONOutput
	readJSONFile(t, report, &refused)
	if refused.Success || len(refused.FitFindings) != 1 || refused.FitFindings[0].Path != "/slides/1/content/1/diagram_value" ||
		refused.FitFindings[0].Pattern != "business_model_canvas" || refused.FitFindings[0].Code != patterns.ErrCodeTextBelowReadableMin {
		t.Fatalf("refusal envelope = %+v", refused)
	}

	report = filepath.Join(dir, "partial.json")
	if err := runJSONMode(deck, report, testTemplatesDir, filepath.Join(dir, "partial"), "", false, false, "modern", "warn", true, "strict", "", false); err != nil {
		t.Fatalf("partial generation failed: %v", err)
	}
	var partial JSONOutput
	readJSONFile(t, report, &partial)
	if !partial.Success || partial.SlideCount != 1 {
		t.Fatalf("partial deck: success=%v slides=%d", partial.Success, partial.SlideCount)
	}
	if _, statErr := os.Stat(partial.OutputPath); statErr != nil {
		t.Fatalf("partial deck not written: %v", statErr)
	}
	found := false
	for _, f := range partial.FitFindings {
		if f.Code == patterns.ErrCodeContentDropped && f.Path == "/slides/1" && f.Fix != nil &&
			f.Fix.Params["cause"] == patterns.ErrCodeTextBelowReadableMin && f.Fix.Params["refused_path"] == "/slides/1/content/1/diagram_value" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no CONTENT_DROPPED finding for the skipped slide: %+v", partial.FitFindings)
	}
}

func readJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatal(err)
	}
}
