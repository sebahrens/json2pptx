package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestEvaluateStrictFitSectionSequencePolicy(t *testing.T) {
	layouts := []types.LayoutMetadata{{
		ID:            "section-layout",
		CanonicalType: types.CanonicalLayoutSectionDivider,
		Placeholders: []types.PlaceholderInfo{{
			ID: "Section Number", Role: types.PlaceholderRoleSectionNumber,
		}},
	}}
	newInput := func() *PresentationInput {
		label := "02"
		return &PresentationInput{Slides: []SlideInput{{
			LayoutID: "section-layout", SlideType: "section",
			Content: []ContentInput{{PlaceholderID: "Section Number", Type: "text", TextValue: &label}},
		}}}
	}

	t.Run("strict refuses without changing authored input", func(t *testing.T) {
		input := newInput()
		findings, err := evaluateStrictFit(input, "strict", layouts, 12192000, 6858000, nil)
		if err == nil {
			t.Fatal("strict mode accepted a first divider labelled 02")
		}
		if *input.Slides[0].Content[0].TextValue != "02" {
			t.Fatal("strict evaluation changed the authored label")
		}
		if f := firstFindingCode(findings, patterns.ErrCodeSectionNumberSequenceMismatch); f == nil || f.Action != "refuse" {
			t.Fatalf("missing section-number refusal: %+v", findings)
		}
	})

	t.Run("warn corrects label and reports the correction", func(t *testing.T) {
		input := newInput()
		findings, err := evaluateStrictFit(input, "warn", layouts, 12192000, 6858000, nil)
		if err != nil {
			t.Fatalf("warn mode: %v", err)
		}
		if *input.Slides[0].Content[0].TextValue != "01" {
			t.Fatalf("section label = %q, want 01", *input.Slides[0].Content[0].TextValue)
		}
		if f := firstFindingCode(findings, patterns.ErrCodeSectionNumberRenumbered); f == nil || f.Action != "info" {
			t.Fatalf("missing informational renumbering finding: %+v", findings)
		}
		if f := firstFindingCode(findings, patterns.ErrCodeSectionNumberSequenceMismatch); f != nil {
			t.Fatalf("corrected label still has a mismatch finding: %+v", *f)
		}
	})

	t.Run("warn keeps remaining refusals ahead of correction info", func(t *testing.T) {
		input := newInput()
		findings, _ := evaluateStrictFit(input, "strict", layouts, 12192000, 6858000, nil)
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{Code: "OTHER_REFUSAL", Path: "/slides/0"},
			Action:          "refuse",
		})
		corrected, err := renumberWarnModeSections(input, findings)
		if err != nil {
			t.Fatal(err)
		}
		if corrected[0].Code != "OTHER_REFUSAL" || corrected[len(corrected)-1].Code != patterns.ErrCodeSectionNumberRenumbered {
			t.Fatalf("findings not sorted by post-correction severity: %+v", corrected)
		}
	})
}

func firstFindingCode(findings []patterns.FitFinding, code string) *patterns.FitFinding {
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}

func TestSectionSequenceRefusalMatchesValidateAndGenerate(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	deck := mustParseJSON(`{"template":"midnight-blue","slides":[{"layout_id":"section","slide_type":"section","content":[{"placeholder_id":"title","type":"text","text_value":"Market context"},{"placeholder_id":"Section Number","type":"text","text_value":"02"}]}]}`)
	validate, err := mc.handleValidate(context.Background(), makeRequest(map[string]any{"presentation": deck}))
	if err != nil {
		t.Fatal(err)
	}
	requireStructuredError(t, validate, "SECTION_NUMBER_SEQUENCE_MISMATCH")
	generate, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": deck,
		"strict_fit":   "strict",
	}))
	if err != nil {
		t.Fatal(err)
	}
	requireStructuredError(t, generate, "SECTION_NUMBER_SEQUENCE_MISMATCH")
}

func TestWarnModeRenumbersSectionInGeneratedDeck(t *testing.T) {
	mc := &mcpConfig{
		templatesDir: "../../templates",
		outputDir:    t.TempDir(),
		cache:        template.NewMemoryCache(24 * time.Hour),
	}
	deck := mustParseJSON(`{"template":"midnight-blue","slides":[{"layout_id":"section","slide_type":"section","content":[{"placeholder_id":"title","type":"text","text_value":"Market context"},{"placeholder_id":"Section Number","type":"text","text_value":"02"}]}]}`)
	result, err := mc.handleGenerate(context.Background(), makeRequest(map[string]any{
		"presentation": deck,
		"strict_fit":   "warn",
	}))
	if err != nil || result.IsError {
		t.Fatalf("warn generation failed: err=%v, result=%v", err, result)
	}
	var output JSONOutput
	if err := json.Unmarshal([]byte(textContent(result)), &output); err != nil {
		t.Fatalf("decode generation output: %v", err)
	}
	if !output.Success {
		t.Fatal("warn generation did not succeed")
	}
	if finding := firstFindingCode(output.FitFindings, patterns.ErrCodeSectionNumberRenumbered); finding == nil || finding.Action != "info" {
		t.Fatalf("missing informational correction in generation output: %+v", output.FitFindings)
	}
	reader, err := zip.OpenReader(output.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != "ppt/slides/slide1.xml" {
			continue
		}
		body, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		xml, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(xml), ">01</a:t>") || strings.Contains(string(xml), ">02</a:t>") {
			t.Fatalf("generated section number not corrected to 01: %s", xml)
		}
		return
	}
	t.Fatal("generated deck has no first slide XML")
}

func TestStrictFitRefusesEveryCorpusValidationRefusal(t *testing.T) {
	paths, err := filepath.Glob("../../examples/*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("find example corpus: %v (%d files)", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel() // Corpus files and input instances are independent.
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var input PresentationInput
			if err := strictUnmarshalJSON(data, &input); err != nil {
				t.Fatal(err)
			}
			applyDefaults(&input)
			layouts, theme, width, height := fitReportGeometry(input.Template, "../../templates")
			if layouts == nil {
				t.Fatalf("could not analyze template %q", input.Template)
			}
			validationFindings := collectFitFindings(&input, layouts, width, height, theme)
			refuses := false
			for _, finding := range validationFindings {
				if finding.Action == "refuse" {
					refuses = true
					break
				}
			}
			_, strictErr := evaluateStrictFit(&input, "strict", layouts, width, height, theme)
			if (strictErr != nil) != refuses {
				t.Fatalf("strict generation refusal=%t, validation refusal=%t", strictErr != nil, refuses)
			}
		})
	}
}
