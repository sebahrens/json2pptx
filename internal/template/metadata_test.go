package template

import (
	"errors"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestValidateMetadataVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		wantErr     bool
		errType     interface{} // Expected error type (nil, *MetadataVersionError, *MetadataParseError)
		errContains string      // Substring expected in error message
	}{
		{
			name:    "valid version 1.0",
			version: "1.0",
			wantErr: false,
		},
		{
			name:    "empty version defaults to minimum",
			version: "",
			wantErr: false,
		},
		{
			name:        "version too old",
			version:     "0.9",
			wantErr:     true,
			errType:     &MetadataVersionError{},
			errContains: "unsupported metadata version",
		},
		{
			name:        "version too new",
			version:     "2.0",
			wantErr:     true,
			errType:     &MetadataVersionError{},
			errContains: "unsupported metadata version",
		},
		{
			name:        "invalid format - no minor",
			version:     "1",
			wantErr:     true,
			errType:     &MetadataParseError{},
			errContains: "invalid version format",
		},
		{
			name:        "invalid format - too many parts",
			version:     "1.0.0",
			wantErr:     true,
			errType:     &MetadataParseError{},
			errContains: "invalid version format",
		},
		{
			name:        "invalid format - non-numeric major",
			version:     "a.0",
			wantErr:     true,
			errType:     &MetadataParseError{},
			errContains: "invalid version format",
		},
		{
			name:        "invalid format - non-numeric minor",
			version:     "1.b",
			wantErr:     true,
			errType:     &MetadataParseError{},
			errContains: "invalid version format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetadataVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMetadataVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil {
				switch tt.errType.(type) {
				case *MetadataVersionError:
					var versionErr *MetadataVersionError
					if !errors.As(err, &versionErr) {
						t.Errorf("expected MetadataVersionError, got %T", err)
					}
				case *MetadataParseError:
					var parseErr *MetadataParseError
					if !errors.As(err, &parseErr) {
						t.Errorf("expected MetadataParseError, got %T", err)
					}
				}
			}
			if tt.wantErr && tt.errContains != "" {
				if err != nil {
					errStr := err.Error()
					if !stringContains(errStr, tt.errContains) {
						t.Errorf("error %q should contain %q", errStr, tt.errContains)
					}
				}
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    versionParts
		b    versionParts
		want int
	}{
		{
			name: "equal versions",
			a:    versionParts{major: 1, minor: 0},
			b:    versionParts{major: 1, minor: 0},
			want: 0,
		},
		{
			name: "a major less than b",
			a:    versionParts{major: 1, minor: 0},
			b:    versionParts{major: 2, minor: 0},
			want: -1,
		},
		{
			name: "a major greater than b",
			a:    versionParts{major: 2, minor: 0},
			b:    versionParts{major: 1, minor: 0},
			want: 1,
		},
		{
			name: "a minor less than b (same major)",
			a:    versionParts{major: 1, minor: 0},
			b:    versionParts{major: 1, minor: 1},
			want: -1,
		},
		{
			name: "a minor greater than b (same major)",
			a:    versionParts{major: 1, minor: 2},
			b:    versionParts{major: 1, minor: 1},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{
			name:      "valid version 1.0",
			version:   "1.0",
			wantMajor: 1,
			wantMinor: 0,
			wantErr:   false,
		},
		{
			name:      "valid version 2.5",
			version:   "2.5",
			wantMajor: 2,
			wantMinor: 5,
			wantErr:   false,
		},
		{
			name:    "invalid - no dot",
			version: "10",
			wantErr: true,
		},
		{
			name:    "invalid - too many dots",
			version: "1.0.0",
			wantErr: true,
		},
		{
			name:    "invalid - empty",
			version: "",
			wantErr: true,
		},
		{
			name:    "invalid - non-numeric",
			version: "a.b",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersion(%q) error = %v, wantErr %v", tt.version, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.major != tt.wantMajor || got.minor != tt.wantMinor {
					t.Errorf("parseVersion(%q) = {%d, %d}, want {%d, %d}",
						tt.version, got.major, got.minor, tt.wantMajor, tt.wantMinor)
				}
			}
		})
	}
}

func TestNormalizeMetadataVersion(t *testing.T) {
	tests := []struct {
		name     string
		metadata *types.TemplateMetadata
		want     string
	}{
		{
			name:     "nil metadata returns minimum",
			metadata: nil,
			want:     types.MetadataVersionMin,
		},
		{
			name:     "empty version returns minimum",
			metadata: &types.TemplateMetadata{Version: ""},
			want:     types.MetadataVersionMin,
		},
		{
			name:     "explicit version preserved",
			metadata: &types.TemplateMetadata{Version: "1.0"},
			want:     "1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMetadataVersion(tt.metadata)
			if got != tt.want {
				t.Errorf("NormalizeMetadataVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyMetadataHints(t *testing.T) {
	tests := []struct {
		name           string
		layouts        []types.LayoutMetadata
		metadata       *types.TemplateMetadata
		checkLayout    int      // Index of layout to check
		wantMaxBullets int      // Expected MaxBullets after applying hints
		wantTags       []string // Expected tags after applying hints
	}{
		{
			name: "nil metadata does nothing",
			layouts: []types.LayoutMetadata{
				{ID: "layout1", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{}},
			},
			metadata:       nil,
			checkLayout:    0,
			wantMaxBullets: 5,
			wantTags:       []string{},
		},
		{
			name: "empty hints does nothing",
			layouts: []types.LayoutMetadata{
				{ID: "layout1", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{}},
			},
			metadata:       &types.TemplateMetadata{LayoutHints: nil},
			checkLayout:    0,
			wantMaxBullets: 5,
			wantTags:       []string{},
		},
		{
			name: "applies MaxBullets override by ID",
			layouts: []types.LayoutMetadata{
				{ID: "layout1", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{}},
			},
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"layout1": {MaxBullets: 10},
				},
			},
			checkLayout:    0,
			wantMaxBullets: 10,
			wantTags:       []string{},
		},
		{
			name: "applies MaxBullets override by Name",
			layouts: []types.LayoutMetadata{
				{ID: "slideLayout1", Name: "Title Slide", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{}},
			},
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"Title Slide": {MaxBullets: 3},
				},
			},
			checkLayout:    0,
			wantMaxBullets: 3,
			wantTags:       []string{},
		},
		{
			name: "adds deprecated tag",
			layouts: []types.LayoutMetadata{
				{ID: "layout1", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{"content"}},
			},
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"layout1": {Deprecated: true},
				},
			},
			checkLayout:    0,
			wantMaxBullets: 5,
			wantTags:       []string{"content", "deprecated"},
		},
		{
			name: "adds preferred tags",
			layouts: []types.LayoutMetadata{
				{ID: "layout1", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{}},
			},
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"layout1": {PreferredFor: []string{"charts", "data"}},
				},
			},
			checkLayout:    0,
			wantMaxBullets: 5,
			wantTags:       []string{"charts", "data"},
		},
		{
			name: "does not duplicate tags",
			layouts: []types.LayoutMetadata{
				{ID: "layout1", Capacity: types.CapacityEstimate{MaxBullets: 5}, Tags: []string{"content"}},
			},
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"layout1": {PreferredFor: []string{"content", "new-tag"}},
				},
			},
			checkLayout:    0,
			wantMaxBullets: 5,
			wantTags:       []string{"content", "new-tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			layouts := make([]types.LayoutMetadata, len(tt.layouts))
			copy(layouts, tt.layouts)
			for i := range layouts {
				layouts[i].Tags = make([]string, len(tt.layouts[i].Tags))
				copy(layouts[i].Tags, tt.layouts[i].Tags)
			}

			ApplyMetadataHints(layouts, tt.metadata)

			if layouts[tt.checkLayout].Capacity.MaxBullets != tt.wantMaxBullets {
				t.Errorf("MaxBullets = %d, want %d",
					layouts[tt.checkLayout].Capacity.MaxBullets, tt.wantMaxBullets)
			}

			if len(layouts[tt.checkLayout].Tags) != len(tt.wantTags) {
				t.Errorf("Tags = %v, want %v", layouts[tt.checkLayout].Tags, tt.wantTags)
				return
			}

			for _, wantTag := range tt.wantTags {
				found := false
				for _, gotTag := range layouts[tt.checkLayout].Tags {
					if gotTag == wantTag {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Tags %v missing expected tag %q", layouts[tt.checkLayout].Tags, wantTag)
				}
			}
		})
	}
}

func TestMetadataParseError(t *testing.T) {
	t.Run("error with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := &MetadataParseError{
			Path:    "ppt/metadata.json",
			Message: "failed to parse",
			Cause:   cause,
		}

		errMsg := err.Error()
		if errMsg != "metadata parse error in ppt/metadata.json: failed to parse: underlying error" {
			t.Errorf("unexpected error message: %s", errMsg)
		}

		if errors.Unwrap(err) != cause {
			t.Error("Unwrap should return the cause")
		}
	})

	t.Run("error without cause", func(t *testing.T) {
		err := &MetadataParseError{
			Path:    "ppt/metadata.json",
			Message: "file not found",
			Cause:   nil,
		}

		errMsg := err.Error()
		if errMsg != "metadata parse error in ppt/metadata.json: file not found" {
			t.Errorf("unexpected error message: %s", errMsg)
		}

		if errors.Unwrap(err) != nil {
			t.Error("Unwrap should return nil when no cause")
		}
	})
}

func TestMetadataVersionError(t *testing.T) {
	err := &MetadataVersionError{
		Version:    "0.5",
		MinVersion: "1.0",
		MaxVersion: "1.0",
	}

	errMsg := err.Error()
	expected := "unsupported metadata version 0.5: supported range is 1.0 to 1.0"
	if errMsg != expected {
		t.Errorf("error message = %q, want %q", errMsg, expected)
	}
}

// stringContains checks if a string contains a substring.
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestValidationResult_HasIssues(t *testing.T) {
	tests := []struct {
		name   string
		result *ValidationResult
		want   bool
	}{
		{
			name: "no issues",
			result: &ValidationResult{
				Valid:    true,
				Errors:   []string{},
				Warnings: []string{},
			},
			want: false,
		},
		{
			name: "has errors",
			result: &ValidationResult{
				Valid:    false,
				Errors:   []string{"error1"},
				Warnings: []string{},
			},
			want: true,
		},
		{
			name: "has warnings",
			result: &ValidationResult{
				Valid:    true,
				Errors:   []string{},
				Warnings: []string{"warning1"},
			},
			want: true,
		},
		{
			name: "has both",
			result: &ValidationResult{
				Valid:    false,
				Errors:   []string{"error1"},
				Warnings: []string{"warning1"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasIssues(); got != tt.want {
				t.Errorf("HasIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidationResult_AllIssues(t *testing.T) {
	result := &ValidationResult{
		Errors:   []string{"error1", "error2"},
		Warnings: []string{"warning1"},
	}

	issues := result.AllIssues()
	if len(issues) != 3 {
		t.Errorf("AllIssues() len = %d, want 3", len(issues))
	}

	// Errors should come first
	if issues[0] != "error1" || issues[1] != "error2" || issues[2] != "warning1" {
		t.Errorf("AllIssues() = %v, want [error1, error2, warning1]", issues)
	}
}

func TestIsValidAspectRatio(t *testing.T) {
	tests := []struct {
		ratio string
		want  bool
	}{
		{"16:9", true},
		{"4:3", true},
		{"1:1", true},
		{"21:9", true},
		{"", false},
		{"16", false},
		{"16:9:3", false},
		{"16:", false},
		{":9", false},
		{"a:b", false},
		{"16:a", false},
	}

	for _, tt := range tests {
		t.Run(tt.ratio, func(t *testing.T) {
			if got := isValidAspectRatio(tt.ratio); got != tt.want {
				t.Errorf("isValidAspectRatio(%q) = %v, want %v", tt.ratio, got, tt.want)
			}
		})
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name             string
		metadata         *types.TemplateMetadata
		wantWarningCount int
		wantWarnings     []string // Substrings expected in warnings
	}{
		{
			name:             "valid metadata",
			metadata:         &types.TemplateMetadata{AspectRatio: "16:9"},
			wantWarningCount: 0,
		},
		{
			name:             "invalid aspect ratio",
			metadata:         &types.TemplateMetadata{AspectRatio: "invalid"},
			wantWarningCount: 1,
			wantWarnings:     []string{"invalid aspect ratio format"},
		},
		{
			name: "negative max_bullets",
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"layout1": {MaxBullets: -1},
				},
			},
			wantWarningCount: 1,
			wantWarnings:     []string{"invalid max_bullets"},
		},
		{
			name: "negative max_chars",
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"layout1": {MaxChars: -5},
				},
			},
			wantWarningCount: 1,
			wantWarnings:     []string{"invalid max_chars"},
		},
		{
			name: "empty layout hint key",
			metadata: &types.TemplateMetadata{
				LayoutHints: map[string]types.LayoutHint{
					"": {MaxBullets: 5},
				},
			},
			wantWarningCount: 1,
			wantWarnings:     []string{"empty key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{
				Valid:    true,
				Errors:   make([]string, 0),
				Warnings: make([]string, 0),
			}

			validateRequiredFields(tt.metadata, result)

			if len(result.Warnings) != tt.wantWarningCount {
				t.Errorf("warning count = %d, want %d; warnings: %v",
					len(result.Warnings), tt.wantWarningCount, result.Warnings)
			}

			for _, wantWarning := range tt.wantWarnings {
				found := false
				for _, w := range result.Warnings {
					if stringContains(w, wantWarning) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("warnings %v missing expected warning containing %q", result.Warnings, wantWarning)
				}
			}
		})
	}
}
