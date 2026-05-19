package main

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// TestValidateTakeawayMissing exercises the takeaway_missing warning that
// fires on chart and matrix slides whose `takeaway` field is empty, and
// verifies that the warning is suppressed when a takeaway is provided.
func TestValidateTakeawayMissing(t *testing.T) {
	analysis := &types.TemplateAnalysis{
		Layouts: []types.LayoutMetadata{
			{
				ID:   "content-slide",
				Name: "Content Slide",
				Placeholders: []types.PlaceholderInfo{
					{ID: "content", Type: types.PlaceholderContent, MaxChars: 0},
				},
			},
			{
				ID:           "blank",
				Name:         "Blank",
				Placeholders: []types.PlaceholderInfo{},
			},
		},
	}

	chartContent := []ContentInput{{
		PlaceholderID: "content",
		Type:          "chart",
		Value:         json.RawMessage(`{"type":"bar","data":{"Q1":10,"Q2":20}}`),
	}}

	matrixPattern := &PatternInput{Name: "matrix-2x2"}

	cases := []struct {
		name        string
		slide       SlideInput
		wantWarning bool
	}{
		{
			name: "chart slide without takeaway → warning",
			slide: SlideInput{
				LayoutID: "content-slide",
				Content:  chartContent,
			},
			wantWarning: true,
		},
		{
			name: "chart slide with takeaway → no warning",
			slide: SlideInput{
				LayoutID: "content-slide",
				Content:  chartContent,
				Takeaway: "Revenue doubled quarter over quarter.",
			},
			wantWarning: false,
		},
		{
			name: "matrix-2x2 slide without takeaway → warning",
			slide: SlideInput{
				LayoutID: "blank",
				Content:  []ContentInput{},
				Pattern:  matrixPattern,
			},
			wantWarning: true,
		},
		{
			name: "matrix-2x2 slide with takeaway → no warning",
			slide: SlideInput{
				LayoutID: "blank",
				Content:  []ContentInput{},
				Pattern:  matrixPattern,
				Takeaway: "Customers cluster in the high-value / high-effort quadrant.",
			},
			wantWarning: false,
		},
		{
			name: "plain content slide without takeaway → no warning",
			slide: SlideInput{
				LayoutID: "content-slide",
				Content: []ContentInput{{
					PlaceholderID: "content",
					Type:          "text",
					Value:         json.RawMessage(`"hello"`),
				}},
			},
			wantWarning: false,
		},
		{
			name: "whitespace-only takeaway counts as missing",
			slide: SlideInput{
				LayoutID: "content-slide",
				Content:  chartContent,
				Takeaway: "   \t  ",
			},
			wantWarning: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := dryRunOutput{Valid: true, Warnings: []string{}, Slides: []dryRunSlide{}}
			validateSlidesAgainstTemplate(&output, []SlideInput{tc.slide}, analysis)

			var found bool
			for _, vw := range output.ValidationWarnings {
				if vw.Code == patterns.ErrCodeTakeawayMissing {
					found = true
					break
				}
			}
			if found != tc.wantWarning {
				t.Errorf("takeaway_missing warning: got=%v want=%v\nwarnings=%v", found, tc.wantWarning, output.Warnings)
			}
		})
	}
}
