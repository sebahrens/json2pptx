package main

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/diagnostics"
	"github.com/sebahrens/json2pptx/internal/types"
)

func TestContentVocabularyValidationParity(t *testing.T) {
	for _, tt := range []struct {
		name, kind, heading string
		valid               bool
	}{
		{"unicode heading at limit", "text", strings.Repeat("é", 60), true},
		{"unicode heading over limit", "text", strings.Repeat("é", 61), false},
		{"unknown content kind", "txet", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			text := "A concise body"
			item := ContentInput{PlaceholderID: "body", Type: tt.kind, Heading: tt.heading, TextValue: &text}
			_, err := convertPresentationContent([]ContentInput{item}, 1, types.SlideTypeContent)
			if (err == nil) != tt.valid {
				t.Fatalf("convert error=%v; want valid=%t", err, tt.valid)
			}
			out := dryRunOutput{Valid: true, verdictOnly: true}
			validateSlidesAgainstTemplate(&out, []SlideInput{{LayoutID: "body-layout", Content: []ContentInput{item}}}, &types.TemplateAnalysis{Layouts: []types.LayoutMetadata{{ID: "body-layout", Placeholders: []types.PlaceholderInfo{{ID: "body", Type: types.PlaceholderBody}}}}})
			if out.Valid != tt.valid {
				t.Fatalf("validate valid=%t; findings=%+v", out.Valid, out.Diagnostics)
			}
			if !tt.valid {
				for _, d := range out.Diagnostics {
					if d.Severity == diagnostics.SeverityError && !strings.Contains(err.Error(), d.Message) {
						t.Errorf("converter %q disagrees with validator %q", err, d.Message)
					}
				}
			}
		})
	}
}
