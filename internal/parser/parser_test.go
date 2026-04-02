package parser

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestValidatePresentation(t *testing.T) {
	tests := []struct {
		name           string
		presentation   *types.PresentationDefinition
		wantErrorCount int
		wantFields     []string
	}{
		{
			name: "missing title produces error",
			presentation: &types.PresentationDefinition{
				Metadata: types.Metadata{
					Template: "default.pptx",
				},
				Slides: []types.SlideDefinition{{Index: 0}},
			},
			wantErrorCount: 1,
			wantFields:     []string{"title"},
		},
		{
			name: "missing template produces error",
			presentation: &types.PresentationDefinition{
				Metadata: types.Metadata{
					Title: "My Deck",
				},
				Slides: []types.SlideDefinition{{Index: 0}},
			},
			wantErrorCount: 1,
			wantFields:     []string{"template"},
		},
		{
			name: "no slides produces error",
			presentation: &types.PresentationDefinition{
				Metadata: types.Metadata{
					Title:    "My Deck",
					Template: "default.pptx",
				},
				Slides: []types.SlideDefinition{},
			},
			wantErrorCount: 1,
			wantFields:     []string{"slides"},
		},
		{
			name: "all missing produces three errors",
			presentation: &types.PresentationDefinition{
				Metadata: types.Metadata{},
				Slides:   []types.SlideDefinition{},
			},
			wantErrorCount: 3,
			wantFields:     []string{"title", "template", "slides"},
		},
		{
			name: "valid presentation no errors",
			presentation: &types.PresentationDefinition{
				Metadata: types.Metadata{
					Title:    "My Deck",
					Template: "default.pptx",
				},
				Slides: []types.SlideDefinition{{Index: 0}},
			},
			wantErrorCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidatePresentation(tt.presentation)

			if len(errs) != tt.wantErrorCount {
				t.Fatalf("ValidatePresentation() returned %d errors, want %d; errors: %v",
					len(errs), tt.wantErrorCount, errs)
			}

			for i, wantField := range tt.wantFields {
				if i >= len(errs) {
					break
				}
				if errs[i].Field != wantField {
					t.Errorf("error[%d].Field = %q, want %q", i, errs[i].Field, wantField)
				}
			}

			// All validation errors should be ErrorLevelError
			for i, e := range errs {
				if e.Level != types.ErrorLevelError {
					t.Errorf("error[%d].Level = %q, want %q", i, e.Level, types.ErrorLevelError)
				}
			}
		})
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name         string
		presentation *types.PresentationDefinition
		want         bool
	}{
		{
			name: "no errors returns false",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{},
			},
			want: false,
		},
		{
			name: "only warnings returns false",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{
					{Message: "minor issue", Level: types.ErrorLevelWarning},
					{Message: "another warning", Level: types.ErrorLevelWarning},
				},
			},
			want: false,
		},
		{
			name: "has error level error returns true",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{
					{Message: "a warning", Level: types.ErrorLevelWarning},
					{Message: "a fatal error", Level: types.ErrorLevelError},
				},
			},
			want: true,
		},
		{
			name: "nil errors returns false",
			presentation: &types.PresentationDefinition{
				Errors: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasErrors(tt.presentation)
			if got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetErrorsByLevel(t *testing.T) {
	tests := []struct {
		name         string
		presentation *types.PresentationDefinition
		level        types.ErrorLevel
		wantCount    int
	}{
		{
			name: "filter warnings from mixed errors",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{
					{Message: "warn1", Level: types.ErrorLevelWarning},
					{Message: "err1", Level: types.ErrorLevelError},
					{Message: "warn2", Level: types.ErrorLevelWarning},
					{Message: "err2", Level: types.ErrorLevelError},
				},
			},
			level:     types.ErrorLevelWarning,
			wantCount: 2,
		},
		{
			name: "filter errors from mixed errors",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{
					{Message: "warn1", Level: types.ErrorLevelWarning},
					{Message: "err1", Level: types.ErrorLevelError},
					{Message: "warn2", Level: types.ErrorLevelWarning},
				},
			},
			level:     types.ErrorLevelError,
			wantCount: 1,
		},
		{
			name: "no matching level returns empty",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{
					{Message: "warn1", Level: types.ErrorLevelWarning},
				},
			},
			level:     types.ErrorLevelError,
			wantCount: 0,
		},
		{
			name: "empty errors returns empty",
			presentation: &types.PresentationDefinition{
				Errors: []types.ParseError{},
			},
			level:     types.ErrorLevelError,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetErrorsByLevel(tt.presentation, tt.level)

			if len(got) != tt.wantCount {
				t.Fatalf("GetErrorsByLevel(%q) returned %d errors, want %d", tt.level, len(got), tt.wantCount)
			}

			for i, e := range got {
				if e.Level != tt.level {
					t.Errorf("filtered error[%d].Level = %q, want %q", i, e.Level, tt.level)
				}
			}
		})
	}
}
