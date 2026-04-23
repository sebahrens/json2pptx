package patterns

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestResolveAccent(t *testing.T) {
	meta := &types.TemplateMetadata{
		SemanticAccents: map[string]string{
			"positive": "accent3",
			"negative": "accent2",
			"neutral":  "accent4",
		},
	}

	tests := []struct {
		name           string
		accent         string
		semanticAccent string
		metadata       *types.TemplateMetadata
		want           string
	}{
		{"default fallback", "", "", nil, "accent1"},
		{"explicit accent wins", "accent5", "", nil, "accent5"},
		{"explicit accent beats semantic", "accent5", "positive", meta, "accent5"},
		{"semantic resolves via metadata", "", "positive", meta, "accent3"},
		{"semantic negative", "", "negative", meta, "accent2"},
		{"semantic neutral", "", "neutral", meta, "accent4"},
		{"semantic with nil metadata falls back", "", "positive", nil, "accent1"},
		{"semantic with empty map falls back", "", "positive", &types.TemplateMetadata{}, "accent1"},
		{"unknown semantic role falls back", "", "danger", meta, "accent1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAccent(tt.accent, tt.semanticAccent, tt.metadata)
			if got != tt.want {
				t.Errorf("ResolveAccent(%q, %q, ...) = %q, want %q", tt.accent, tt.semanticAccent, got, tt.want)
			}
		})
	}
}
