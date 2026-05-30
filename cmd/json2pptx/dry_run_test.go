package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// diagMessages returns the messages of the diagnostics matching sev. Used by
// tests that previously read the now-removed output.Warnings / output.Errors
// string slices; output.Diagnostics is the complete superset.
func diagMessages(ds []diagnostics.Diagnostic, sev diagnostics.Severity) []string {
	var out []string
	for _, d := range ds {
		if d.Severity == sev {
			out = append(out, d.Message)
		}
	}
	return out
}

// findDiagByCode returns the first diagnostic with the given code, or nil.
func findDiagByCode(ds []diagnostics.Diagnostic, code string) *diagnostics.Diagnostic {
	for i := range ds {
		if ds[i].Code == code {
			return &ds[i]
		}
	}
	return nil
}

// findingMessages returns the messages of the envelope's findings matching sev.
// Used by tests that parse the serialized dryRunOutput wire (where diagnostics
// live in the Findings envelope, not the json:"-" Diagnostics accumulator).
func findingMessages(env diagnostics.FindingEnvelope, sev diagnostics.Severity) []string {
	var out []string
	for _, f := range env.Findings {
		if f.Severity == sev {
			out = append(out, f.Message)
		}
	}
	return out
}

// TestValidateSlidesAgainstTemplate_ChartDiagramSvggen verifies that
// validateSlidesAgainstTemplate dispatches chart/diagram content items to
// svggen Validate() and surfaces structural validation warnings.
func TestValidateSlidesAgainstTemplate_ChartDiagramSvggen(t *testing.T) {
	// Minimal template analysis with one layout containing a content placeholder.
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "content-slide",
				Name: "Content Slide",
				Placeholders: []types.PlaceholderInfo{
					{ID: "content", Type: types.PlaceholderContent, MaxChars: 0},
				},
			},
		},
	}

	t.Run("diagram with invalid waterfall data produces svggen warning", func(t *testing.T) {
		// Waterfall diagram with flat map (no "points" array) should fail
		// svggen validation since diagram type passes data directly without
		// auto-conversion.
		output := dryRunOutput{
			Valid:  true,
			Slides: []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "diagram",
						Value:         json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		found := false
		for _, w := range diagMessages(output.Diagnostics, diagnostics.SeverityWarning) {
			if strings.Contains(w, "waterfall") && strings.Contains(w, "data validation") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected svggen validation warning for waterfall diagram with flat map data, got warnings: %v", diagMessages(output.Diagnostics, diagnostics.SeverityWarning))
		}
	})

	t.Run("chart with flat waterfall data emits conversion warning", func(t *testing.T) {
		// Chart type auto-converts flat maps via buildChartData, which should
		// produce a flat-map conversion warning but not a validation error.
		output := dryRunOutput{
			Valid:  true,
			Slides: []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"waterfall","data":{"Revenue":100,"Costs":-40}}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		foundConversion := false
		for _, w := range diagMessages(output.Diagnostics, diagnostics.SeverityWarning) {
			if strings.Contains(w, "flat data") {
				foundConversion = true
				break
			}
		}
		if !foundConversion {
			t.Errorf("expected flat-map conversion warning for waterfall chart, got warnings: %v", diagMessages(output.Diagnostics, diagnostics.SeverityWarning))
		}
	})

	t.Run("chart with valid bar data produces no svggen warning", func(t *testing.T) {
		output := dryRunOutput{
			Valid:  true,
			Slides: []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "chart",
						Value:         json.RawMessage(`{"type":"bar","data":{"Q1":10,"Q2":20,"Q3":30}}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, w := range diagMessages(output.Diagnostics, diagnostics.SeverityWarning) {
			if strings.Contains(w, "data validation") {
				t.Errorf("unexpected svggen validation warning for valid bar chart: %s", w)
			}
		}
	})

	t.Run("diagram with valid waterfall data produces no warning", func(t *testing.T) {
		output := dryRunOutput{
			Valid:  true,
			Slides: []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "diagram",
						Value: json.RawMessage(`{
							"type":"waterfall",
							"data":{
								"points":[
									{"label":"Revenue","value":100,"type":"increase"},
									{"label":"Costs","value":-40,"type":"decrease"}
								]
							}
						}`),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, w := range diagMessages(output.Diagnostics, diagnostics.SeverityWarning) {
			if strings.Contains(w, "data validation") {
				t.Errorf("unexpected svggen validation warning for valid waterfall diagram: %s", w)
			}
		}
	})

	t.Run("aggregate counts populated for mixed content", func(t *testing.T) {
		output := dryRunOutput{
			Valid:  true,
			Slides: []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{PlaceholderID: "content", Type: "chart", Value: json.RawMessage(`{"type":"bar","data":{"Q1":10}}`)},
					{PlaceholderID: "content", Type: "diagram", Value: json.RawMessage(`{"type":"timeline","data":{"events":[{"label":"A","date":"2026"}]}}`)},
					{PlaceholderID: "content", Type: "table", Value: json.RawMessage(`{"headers":["A"],"rows":[["1"]]}`)},
					{PlaceholderID: "content", Type: "text", TextValue: strPtr("hello")},
				},
				ShapeGrid: &ShapeGridInput{
					Rows: []GridRowInput{
						{Cells: []*GridCellInput{
							{Shape: &ShapeSpecInput{Geometry: "rect"}},
							{Shape: &ShapeSpecInput{Geometry: "ellipse"}},
						}},
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.SlideCount != 1 {
			t.Errorf("SlideCount: got %d, want 1", output.SlideCount)
		}
		if output.ChartCount != 1 {
			t.Errorf("ChartCount: got %d, want 1", output.ChartCount)
		}
		if output.DiagramCount != 1 {
			t.Errorf("DiagramCount: got %d, want 1", output.DiagramCount)
		}
		if output.TableCount != 1 {
			t.Errorf("TableCount: got %d, want 1", output.TableCount)
		}
		if output.ShapeCount != 2 {
			t.Errorf("ShapeCount: got %d, want 2", output.ShapeCount)
		}
	})

	t.Run("text content is not chart-validated", func(t *testing.T) {
		output := dryRunOutput{
			Valid:  true,
			Slides: []dryRunSlide{},
		}
		slides := []SlideInput{
			{
				LayoutID: "content-slide",
				Content: []ContentInput{
					{
						PlaceholderID: "content",
						Type:          "text",
						TextValue:     strPtr("Hello world"),
					},
				},
			},
		}

		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, w := range diagMessages(output.Diagnostics, diagnostics.SeverityWarning) {
			if strings.Contains(w, "data validation") {
				t.Errorf("unexpected chart/diagram validation warning for text content: %s", w)
			}
		}
	})
}

// TestValidateSlidesAgainstTemplate_UnknownLayoutID verifies that an unknown
// layout_id produces an error (not a warning), sets Valid=false, and includes
// a structured ValidationError with code "unknown_layout_id" and a did_you_mean
// suggestion when the typo is close.
func TestValidateSlidesAgainstTemplate_UnknownLayoutID(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{ID: "content-slide", Name: "Content Slide"},
			{ID: "title-slide", Name: "Title Slide"},
			{ID: "section-header", Name: "Section Header"},
		},
	}

	t.Run("typo layout_id is error with did_you_mean", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{LayoutID: "conten-slide"}} // typo
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.Valid {
			t.Error("expected Valid=false for unknown layout_id, got true")
		}
		errMsgs := diagMessages(output.Diagnostics, diagnostics.SeverityError)
		if len(errMsgs) == 0 {
			t.Fatal("expected at least one error for unknown layout_id")
		}
		if !strings.Contains(errMsgs[0], "not found") {
			t.Errorf("error should mention 'not found': %s", errMsgs[0])
		}
		// Check structured diagnostic
		ve := findDiagByCode(output.Diagnostics, patterns.ErrCodeUnknownLayoutID)
		if ve == nil {
			t.Fatal("expected a diagnostic for unknown layout_id")
		}
		if ve.Path != "/slides/0/layout_id" {
			t.Errorf("expected path /slides/0/layout_id, got %q", ve.Path)
		}
		if ve.Fix == nil {
			t.Fatal("expected fix suggestion")
		}
		if ve.Fix.Kind != "use_one_of" {
			t.Errorf("expected fix kind 'use_one_of', got %q", ve.Fix.Kind)
		}
		dym, ok := ve.Fix.Params["did_you_mean"].(string)
		if !ok || dym != "content-slide" {
			t.Errorf("expected did_you_mean='content-slide', got %v", ve.Fix.Params["did_you_mean"])
		}
	})

	t.Run("completely wrong layout_id is error without did_you_mean", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{LayoutID: "zzz-nonexistent-zzz"}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.Valid {
			t.Error("expected Valid=false for unknown layout_id")
		}
		ve := findDiagByCode(output.Diagnostics, patterns.ErrCodeUnknownLayoutID)
		if ve == nil {
			t.Fatal("expected a diagnostic for unknown layout_id")
		}
		if ve.Fix == nil {
			t.Fatal("expected fix suggestion")
		}
		if _, ok := ve.Fix.Params["did_you_mean"]; ok {
			t.Error("did not expect did_you_mean for completely wrong layout_id")
		}
	})

	t.Run("valid layout_id produces no error", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{LayoutID: "content-slide"}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if !output.Valid {
			t.Error("expected Valid=true for valid layout_id")
		}
		if errs := diagMessages(output.Diagnostics, diagnostics.SeverityError); len(errs) > 0 {
			t.Errorf("unexpected errors: %v", errs)
		}
	})
}

func TestValidateTableStyleID(t *testing.T) {
	knownGUID := "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "content-slide",
				Name: "Content Slide",
				Placeholders: []types.PlaceholderInfo{
					{ID: "body", Type: "body"},
				},
			},
		},
		TableStyles: []types.TableStyleInfo{
			{ID: knownGUID, Name: "Medium Style 2 - Accent 1"},
			{ID: "{21E4AEA4-8DFA-4A89-87EB-49C32662AFE0}", Name: "Medium Style 2 - Accent 2"},
		},
	}

	t.Run("unknown style_id produces validation warning", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{
			LayoutID: "content-slide",
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "table",
				TableValue: &TableInput{
					Headers: []string{"A", "B"},
					Rows:    [][]TableCellInput{{{Content: "1"}, {Content: "2"}}},
					Style:   &TableStyleInput{StyleID: "{00000000-0000-0000-0000-000000000000}"},
				},
			}},
		}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		found := findDiagByCode(output.Diagnostics, patterns.ErrCodeUnknownTableStyleID)
		if found == nil {
			t.Fatal("expected a diagnostic with code unknown_table_style_id")
		}
		if found.Severity != diagnostics.SeverityWarning {
			t.Errorf("expected warning severity, got %q", found.Severity)
		}
		if found.Fix == nil || found.Fix.Kind != "use_one_of" {
			t.Errorf("expected fix kind 'use_one_of', got %v", found.Fix)
		}
	})

	t.Run("known style_id produces no warning", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{
			LayoutID: "content-slide",
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "table",
				TableValue: &TableInput{
					Headers: []string{"A", "B"},
					Rows:    [][]TableCellInput{{{Content: "1"}, {Content: "2"}}},
					Style:   &TableStyleInput{StyleID: knownGUID},
				},
			}},
		}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if findDiagByCode(output.Diagnostics, patterns.ErrCodeUnknownTableStyleID) != nil {
			t.Errorf("unexpected unknown_table_style_id warning for known GUID")
		}
	})

	t.Run("@template-default is always valid", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{
			LayoutID: "content-slide",
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "table",
				TableValue: &TableInput{
					Headers: []string{"A", "B"},
					Rows:    [][]TableCellInput{{{Content: "1"}, {Content: "2"}}},
					Style:   &TableStyleInput{StyleID: "@template-default"},
				},
			}},
		}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if findDiagByCode(output.Diagnostics, patterns.ErrCodeUnknownTableStyleID) != nil {
			t.Errorf("unexpected unknown_table_style_id warning for @template-default")
		}
	})

	t.Run("no style_id produces no warning", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{
			LayoutID: "content-slide",
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "table",
				TableValue: &TableInput{
					Headers: []string{"A", "B"},
					Rows:    [][]TableCellInput{{{Content: "1"}, {Content: "2"}}},
				},
			}},
		}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if findDiagByCode(output.Diagnostics, patterns.ErrCodeUnknownTableStyleID) != nil {
			t.Errorf("unexpected unknown_table_style_id warning when no style_id set")
		}
	})

	t.Run("malicious non-GUID style_id is rejected as an error", func(t *testing.T) {
		output := dryRunOutput{Valid: true, Slides: []dryRunSlide{}}
		slides := []SlideInput{{
			LayoutID: "content-slide",
			Content: []ContentInput{{
				PlaceholderID: "body",
				Type:          "table",
				TableValue: &TableInput{
					Headers: []string{"A", "B"},
					Rows:    [][]TableCellInput{{{Content: "1"}, {Content: "2"}}},
					Style:   &TableStyleInput{StyleID: `bad"&<>`},
				},
			}},
		}}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.Valid {
			t.Error("expected deck to be marked invalid for a malformed style_id")
		}
		found := findDiagByCode(output.Diagnostics, string(diagnostics.CodeInvalidParameter))
		if found == nil {
			t.Fatal("expected an INVALID_PARAMETER diagnostic for malformed style_id")
		}
		if found.Severity != diagnostics.SeverityError {
			t.Errorf("expected error severity, got %q", found.Severity)
		}
	})
}

func TestValidateShapeFillColor_HexWarning(t *testing.T) {
	tests := []struct {
		name        string
		color       string
		wantWarning bool
	}{
		{"scheme name accent1 — no warning", "accent1", false},
		{"scheme name dk1 — no warning", "dk1", false},
		{"allowlisted black — no warning", "#000000", false},
		{"allowlisted white — no warning", "#FFFFFF", false},
		{"allowlisted white lowercase — no warning", "#ffffff", false},
		{"allowlisted short black — no warning", "#000", false},
		{"allowlisted short white — no warning", "#fff", false},
		{"non-allowlisted hex — warning", "#65686B", true},
		{"non-allowlisted hex lowercase — warning", "#65686b", true},
		{"non-allowlisted short hex — warning", "#abc", true},
		{"empty — no warning", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.color)
			var warnings []string
			valWarnings := validateShapeFillColor(json.RawMessage(raw), 1, 1, 1, &warnings)
			got := len(valWarnings) > 0
			if got != tt.wantWarning {
				t.Errorf("color %q: got warning=%v, want %v (valWarnings=%v)", tt.color, got, tt.wantWarning, valWarnings)
			}
			if tt.wantWarning && len(valWarnings) > 0 {
				if valWarnings[0].Code != patterns.ErrCodeHexFillNonBrand {
					t.Errorf("expected code %q, got %q", patterns.ErrCodeHexFillNonBrand, valWarnings[0].Code)
				}
			}
		})
	}
}

// TestResolveCanonicalLayoutIDs_SharedHelper verifies that the shared
// resolveCanonicalLayoutIDs helper resolves canonical names ("title",
// "content", "blank") to concrete layout IDs and passes through
// already-concrete IDs unchanged.
func TestResolveCanonicalLayoutIDs_SharedHelper(t *testing.T) {
	layouts := []types.LayoutMetadata{
		{ID: "slideLayout1", Name: "Title Slide", Tags: []string{"title-slide"}},
		{ID: "slideLayout2", Name: "Content", Tags: []string{"content"}},
		{ID: "slideLayout3", Name: "Blank + Title", Tags: []string{"blank-title"}},
	}

	tests := []struct {
		name     string
		layoutID string
		wantID   string
	}{
		{"canonical title resolves", "title", "slideLayout1"},
		{"canonical content resolves", "content", "slideLayout2"},
		{"canonical blank resolves", "blank", "slideLayout3"},
		{"concrete ID passes through", "slideLayout2", "slideLayout2"},
		{"unknown name passes through", "nonexistent", "nonexistent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slides := []SlideInput{{LayoutID: tt.layoutID}}
			resolveCanonicalLayoutIDs(slides, layouts)
			if slides[0].LayoutID != tt.wantID {
				t.Errorf("resolveCanonicalLayoutIDs(%q) = %q, want %q", tt.layoutID, slides[0].LayoutID, tt.wantID)
			}
		})
	}
}

// TestResolveCanonicalLayoutIDs_ValidationConsistency verifies that
// canonical layout IDs are resolved before validation, so that
// validateSlidesAgainstTemplate does not report them as unknown.
func TestResolveCanonicalLayoutIDs_ValidationConsistency(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "slideLayout1",
				Name: "Title Slide",
				Tags: []string{"title-slide"},
				Placeholders: []types.PlaceholderInfo{
					{ID: "title", Type: types.PlaceholderTitle},
				},
			},
			{
				ID:   "slideLayout2",
				Name: "Content",
				Tags: []string{"content"},
				Placeholders: []types.PlaceholderInfo{
					{ID: "title", Type: types.PlaceholderTitle},
					{ID: "body", Type: types.PlaceholderContent},
				},
			},
		},
	}

	slides := []SlideInput{
		{
			LayoutID: "title",
			Content: []ContentInput{
				{PlaceholderID: "title", Type: "text", TextValue: strPtr("Hello")},
			},
		},
		{
			LayoutID: "content",
			Content: []ContentInput{
				{PlaceholderID: "body", Type: "text", TextValue: strPtr("World")},
			},
		},
	}

	// Resolve canonical names first (as all entry points now do).
	resolveCanonicalLayoutIDs(slides, analysis.Layouts)

	output := dryRunOutput{
		Valid:  true,
		Slides: []dryRunSlide{},
	}
	validateSlidesAgainstTemplate(&output, slides, analysis)

	errMsgs := diagMessages(output.Diagnostics, diagnostics.SeverityError)
	if !output.Valid {
		t.Errorf("expected valid=true after canonical resolution, got errors: %v", errMsgs)
	}
	if len(errMsgs) > 0 {
		t.Errorf("expected no errors, got: %v", errMsgs)
	}
}

// TestValidateSlidesAgainstTemplate_SlideTypeAlternativeToLayoutID is a
// regression test for go-slide-creator-p13e. validateSlidesAgainstTemplate
// previously errored when layout_id was empty, even if slide_type was
// provided as an auto-selection hint. The generator accepts slide_type and
// auto-selects a layout, so the validator must do the same.
func TestValidateSlidesAgainstTemplate_SlideTypeAlternativeToLayoutID(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "content-slide",
				Name: "Content Slide",
				Placeholders: []types.PlaceholderInfo{
					{ID: "title", Type: types.PlaceholderTitle},
				},
			},
		},
	}

	t.Run("slide_type only is accepted", func(t *testing.T) {
		output := dryRunOutput{Valid: true}
		slides := []SlideInput{
			{
				SlideType: "title",
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Hello"`)},
				},
			},
		}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		for _, e := range diagMessages(output.Diagnostics, diagnostics.SeverityError) {
			if strings.Contains(e, "layout_id is required") || strings.Contains(e, "layout_id or slide_type is required") {
				t.Errorf("unexpected layout/slide_type error: %s", e)
			}
		}
	})

	t.Run("neither layout_id nor slide_type errors", func(t *testing.T) {
		output := dryRunOutput{Valid: true}
		slides := []SlideInput{
			{
				Content: []ContentInput{
					{PlaceholderID: "title", Type: "text", Value: json.RawMessage(`"Hello"`)},
				},
			},
		}
		validateSlidesAgainstTemplate(&output, slides, analysis)

		if output.Valid {
			t.Error("expected invalid when both layout_id and slide_type are missing")
		}
		errMsgs := diagMessages(output.Diagnostics, diagnostics.SeverityError)
		found := false
		for _, e := range errMsgs {
			if strings.Contains(e, "layout_id or slide_type is required") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected 'layout_id or slide_type is required' error, got: %v", errMsgs)
		}
	})
}
